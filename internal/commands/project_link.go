package commands

import (
	"context"
	"fmt"
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

// linkProjectToDirectory records permalink as the project in the current
// directory's .deployhq.toml, so later commands (and new agent sessions) in
// this folder find and reuse it instead of creating another. It never replaces
// a different project already recorded there — it warns instead. Best-effort:
// a write failure is a warning, not an error.
func linkProjectToDirectory(env *output.Envelope, permalink string) {
	path := config.ProjectConfigPath()
	current := config.Get(path, "project")
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
