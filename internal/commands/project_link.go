package commands

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/manifoldco/promptui"
)

// findProjectsNamed returns the account's projects whose name matches name
// (case-insensitive, surrounding whitespace ignored). DeployHQ allows several
// projects with the same name (it suffixes the permalink), so this is the
// only guard against creating the same project twice.
func findProjectsNamed(ctx context.Context, client *sdk.Client, name string) ([]sdk.Project, error) {
	projects, err := client.ListProjects(ctx, nil)
	if err != nil {
		return nil, err
	}
	want := strings.TrimSpace(name)
	var matches []sdk.Project
	for _, p := range projects {
		if strings.EqualFold(strings.TrimSpace(p.Name), want) {
			matches = append(matches, p)
		}
	}
	return matches, nil
}

// describeProjects formats projects as "Name (permalink)" for messages.
func describeProjects(projects []sdk.Project) string {
	parts := make([]string, 0, len(projects))
	for _, p := range projects {
		parts = append(parts, fmt.Sprintf("%s (%s)", p.Name, p.Permalink))
	}
	return strings.Join(parts, ", ")
}

// duplicateProjectError is returned when a project with the requested name
// already exists and the caller has not opted into creating another one.
func duplicateProjectError(name string, matches []sdk.Project) error {
	return &output.UserError{
		Message: fmt.Sprintf("A project named %q already exists: %s", name, describeProjects(matches)),
		Hint: fmt.Sprintf("Reuse it for this directory: dhq config set project %s\n"+
			"Or create another project with the same name anyway: add --allow-duplicate", matches[0].Permalink),
	}
}

// confirmDuplicateProjectFn asks whether to create a project although others
// with the same name exist. A variable so tests can stub the prompt.
var confirmDuplicateProjectFn = func(name string) bool {
	prompt := promptui.Select{
		Label: fmt.Sprintf("Create another project named %q anyway?", name),
		Items: []string{"No", "Yes, create another"},
	}
	idx, _, err := prompt.Run()
	return err == nil && idx == 1
}

// confirmNoDuplicateProject checks the account for projects already named name
// before one is created. With matches it warns and lists them; then it proceeds
// when allowDuplicate is set, asks in interactive mode, and refuses in
// non-interactive mode. A failed lookup never blocks creation.
func confirmNoDuplicateProject(ctx context.Context, env *output.Envelope, client *sdk.Client, name string, allowDuplicate bool) error {
	matches, err := findProjectsNamed(ctx, client, name)
	if err != nil {
		env.Logger.Write("could not check for existing projects named %q: %v", name, err)
		return nil
	}
	if len(matches) == 0 {
		return nil
	}
	if allowDuplicate {
		env.Warn("A project named %q already exists: %s — creating another (--allow-duplicate).", name, describeProjects(matches))
		return nil
	}
	if env.NonInteractive {
		return duplicateProjectError(name, matches)
	}
	env.Warn("A project named %q already exists: %s", name, describeProjects(matches))
	if !confirmDuplicateProjectFn(name) {
		return duplicateProjectError(name, matches)
	}
	return nil
}

// initDuplicateProjectCheck is the 'dhq init' wizard's duplicate-name check.
// The wizard can't show a separate prompt, so a taken name is reported as an
// error on the name step and the user confirms by submitting the same name
// again (the caller then skips this check) or edits it.
func initDuplicateProjectCheck(ctx context.Context, env *output.Envelope, client *sdk.Client, name string) error {
	quiet := *env
	quiet.NonInteractive = true // report, don't prompt: the wizard owns the terminal
	err := confirmNoDuplicateProject(ctx, &quiet, client, name, false)
	var userErr *output.UserError
	if errors.As(err, &userErr) {
		return fmt.Errorf("%s. Press Enter again to create another one anyway, or change the name", userErr.Message)
	}
	return err
}

// linkProjectToDirectory records permalink as the project in the current
// directory's .deployhq.toml, so later commands (and new agent sessions) in
// this folder find and reuse it instead of creating another. It never replaces
// a different project already recorded there, and never creates a file here
// when a parent folder's .deployhq.toml is the one commands here use (the
// loader reads only the nearest file, so a new one would shadow the parent's
// project and settings) — it warns instead — and never rewrites a file it
// can't parse. Best-effort: a failure is a warning, not an error.
func linkProjectToDirectory(env *output.Envelope, permalink string) {
	path := config.ProjectConfigPath()
	if inherited := parentProjectConfig(path); inherited != "" {
		linked, err := config.GetStrict(inherited, "project")
		if err != nil {
			env.Warn("Could not read %s, so the project was not saved: %v", inherited, err)
			env.Warn("Fix the file, then link the project: dhq config set project %s", permalink)
			return
		}
		switch {
		case linked == permalink:
			return
		case linked != "":
			env.Warn("%s already links project %q for this folder; leaving it unchanged.", inherited, linked)
			env.Warn("To use the new project in this folder only: dhq config set project %s, then copy any other settings from %s (account, email) into the new file, which replaces it here.", permalink, inherited)
			return
		}
		// The parent file has settings but no project. The loader reads only
		// the nearest file, so a new one here would hide those settings.
		env.Warn("%s holds this folder's settings, so the project was not saved to a new file here (it would hide them).", inherited)
		env.Warn("To link it, add it to that file: dhq config set project %s (run in %s)", permalink, filepath.Dir(inherited))
		return
	}
	current, err := config.GetStrict(path, "project")
	if err != nil {
		env.Warn("Could not read %s, so the project was not saved to it: %v", path, err)
		env.Warn("Fix the file, then link the project: dhq config set project %s", permalink)
		return
	}
	switch {
	case current == permalink:
		return
	case current != "":
		env.Warn("This directory's %s is linked to project %q; leaving it unchanged.", path, current)
		env.Warn("To use the new project here: dhq config set project %s", permalink)
		return
	}
	if err := config.Set(path, "project", permalink); err != nil {
		env.Warn("Could not save project to %s: %v", path, err)
		return
	}
	env.Status("Saved project %s to %s", permalink, path)
}

// parentProjectConfig returns the parent folder's .deployhq.toml the config
// loader would use for the current directory, or "" when it would use path
// (the current directory's own file) or none.
func parentProjectConfig(path string) string {
	active := config.FindProjectConfig()
	if active == "" {
		return ""
	}
	if own, err := filepath.Abs(path); err == nil && own == active {
		return ""
	}
	return active
}
