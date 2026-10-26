package commands

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/detect"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

// Suggested-settings item statuses. A plan starts with planned/exists/skipped;
// applying turns "add" into "added" or "failed".
const (
	suggestionAdd     = "add"     // will be created
	suggestionExists  = "exists"  // already on the project; left alone
	suggestionSkipped = "skipped" // not applied (e.g. project already has build commands)
	suggestionAdded   = "added"   // created by this run
	suggestionFailed  = "failed"  // create attempt failed
	// suggestionNotAttempted marks a build command left out because an earlier
	// step of the pipeline failed: adding it alone would build out of order.
	suggestionNotAttempted = "not_attempted"
)

// suggestedItem is one suggested build command, excluded-file pattern or
// build-cache path, with what happens (or happened) to it.
type suggestedItem struct {
	Command      string `json:"command,omitempty"`
	Description  string `json:"description,omitempty"`
	Path         string `json:"path,omitempty"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
	templateName string
	haltOnError  *bool
}

// suggestedSettingsPlan is the set of DeployHQ-suggested settings for a project,
// diffed against what the project already has. Planning makes no changes.
type suggestedSettingsPlan struct {
	Project         string          `json:"project"`
	Stack           string          `json:"stack,omitempty"`
	BuildCommands   []suggestedItem `json:"build_commands"`
	ExcludedFiles   []suggestedItem `json:"excluded_files"`
	BuildCacheFiles []suggestedItem `json:"build_cache_files"`
	DryRun          bool            `json:"dry_run"`
}

// pending reports how many items the plan would create.
func (p *suggestedSettingsPlan) pending() int {
	n := 0
	for _, list := range [][]suggestedItem{p.BuildCommands, p.ExcludedFiles, p.BuildCacheFiles} {
		for _, it := range list {
			if it.Status == suggestionAdd {
				n++
			}
		}
	}
	return n
}

// failed names every item that was not created: those whose create failed,
// e.g. `excluded file "node_modules/"`, and build commands not attempted after
// an earlier one failed, marked "(not attempted)".
func (p *suggestedSettingsPlan) failed() []string {
	var names []string
	add := func(kind string, items []suggestedItem, label func(suggestedItem) string) {
		for _, it := range items {
			switch it.Status {
			case suggestionFailed:
				names = append(names, fmt.Sprintf("%s %q", kind, label(it)))
			case suggestionNotAttempted:
				names = append(names, fmt.Sprintf("%s %q (not attempted)", kind, label(it)))
			}
		}
	}
	add("build command", p.BuildCommands, func(it suggestedItem) string { return it.Command })
	add("excluded file", p.ExcludedFiles, func(it suggestedItem) string { return it.Path })
	add("build cache file", p.BuildCacheFiles, func(it suggestedItem) string { return it.Path })
	return names
}

// planSuggestedBuildCommands diffs the suggested build steps against the
// project. Build commands run as an ordered pipeline, so they are all-or-nothing:
// when the project already has any build command, every suggestion is skipped
// rather than merged into (or duplicated in) a pipeline someone set up already.
// A failed lookup is returned as an error alongside the steps planned as
// additions: without the lookup there is no knowing whether a pipeline exists,
// so the caller decides whether to stop or add them anyway.
func planSuggestedBuildCommands(ctx context.Context, client *sdk.Client, projectID string, steps []detect.BuildCommandStep) ([]suggestedItem, error) {
	var items []suggestedItem
	for _, step := range steps {
		if step.Command == "" {
			continue
		}
		desc := step.Description
		if desc == "" {
			desc = step.Command
		}
		if r := []rune(desc); len(r) > 100 {
			desc = string(r[:100])
		}
		items = append(items, suggestedItem{
			Command: step.Command, Description: desc, Status: suggestionAdd,
			templateName: step.TemplateName, haltOnError: step.HaltOnError,
		})
	}
	if len(items) == 0 {
		return nil, nil
	}
	existing, err := client.ListBuildCommands(ctx, projectID, nil)
	if err != nil {
		return items, fmt.Errorf("could not list the project's build commands: %w", err)
	}
	if len(existing) > 0 {
		for i := range items {
			items[i].Status = suggestionSkipped
		}
	}
	return items, nil
}

// planSuggestedPaths diffs suggested path patterns against the ones the project
// already has (as returned by list). A failed lookup is returned as an error,
// wrapped with what (e.g. "excluded files"), alongside every path planned as an
// addition: without the lookup there is no knowing which already exist, so the
// caller decides whether to stop or add them anyway.
func planSuggestedPaths(paths []string, what string, list func() ([]string, error)) ([]suggestedItem, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	existing := map[string]bool{}
	have, listErr := list()
	if listErr != nil {
		listErr = fmt.Errorf("could not list the project's %s: %w", what, listErr)
	} else {
		for _, p := range have {
			existing[p] = true
		}
	}
	var items []suggestedItem
	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		status := suggestionAdd
		if existing[p] {
			status = suggestionExists
		}
		items = append(items, suggestedItem{Path: p, Status: status})
	}
	return items, listErr
}

func planSuggestedExcludedFiles(ctx context.Context, client *sdk.Client, projectID string, paths []string) ([]suggestedItem, error) {
	return planSuggestedPaths(paths, "excluded files", func() ([]string, error) {
		list, err := client.ListExcludedFiles(ctx, projectID, nil)
		out := make([]string, 0, len(list))
		for _, e := range list {
			out = append(out, e.Path)
		}
		return out, err
	})
}

func planSuggestedBuildCacheFiles(ctx context.Context, client *sdk.Client, projectID string, paths []string) ([]suggestedItem, error) {
	return planSuggestedPaths(paths, "build cache files", func() ([]string, error) {
		list, err := client.ListBuildCacheFiles(ctx, projectID)
		out := make([]string, 0, len(list))
		for _, b := range list {
			out = append(out, b.Path)
		}
		return out, err
	})
}

// planSuggestedSettings builds the full plan for projectID from a detection
// result. It stops on the first settings list that can't be read, since the
// plan would otherwise add settings the project may already have.
func planSuggestedSettings(ctx context.Context, client *sdk.Client, projectID string, detection detect.Result) (*suggestedSettingsPlan, error) {
	buildCommands, err := planSuggestedBuildCommands(ctx, client, projectID, detection.BuildCommands)
	if err != nil {
		return nil, err
	}
	excludedFiles, err := planSuggestedExcludedFiles(ctx, client, projectID, detection.ExcludedFiles)
	if err != nil {
		return nil, err
	}
	buildCacheFiles, err := planSuggestedBuildCacheFiles(ctx, client, projectID, detection.BuildCacheFiles)
	if err != nil {
		return nil, err
	}
	return &suggestedSettingsPlan{
		Project:         projectID,
		Stack:           string(detection.Framework),
		BuildCommands:   buildCommands,
		ExcludedFiles:   excludedFiles,
		BuildCacheFiles: buildCacheFiles,
	}, nil
}

// applySuggestedBuildCommands creates the planned build commands in order.
// They run as a pipeline, so it stops at the first that fails: adding the later
// steps alone would leave e.g. "npm run build" without "npm ci", and a re-run
// can't repair that because build commands are skipped once the project has
// any. The failed step and the ones after it are left for the user to add by
// hand, with the commands to do so on stderr. Failures are not fatal here.
func applySuggestedBuildCommands(ctx context.Context, env *output.Envelope, client *sdk.Client, projectID string, items []suggestedItem) {
	for i := range items {
		it := &items[i]
		if it.Status != suggestionAdd {
			continue
		}
		env.Status("Adding build command: %s", it.Command)
		if _, err := client.CreateBuildCommand(ctx, projectID, sdk.BuildCommandCreateRequest{
			Command:      it.Command,
			Description:  it.Description,
			TemplateName: it.templateName,
			HaltOnError:  it.haltOnError,
		}); err != nil {
			it.Status, it.Error = suggestionFailed, err.Error()
			for j := i + 1; j < len(items); j++ {
				if items[j].Status == suggestionAdd {
					items[j].Status = suggestionNotAttempted
				}
			}
			env.Warn("Could not set the suggested build command (%q): %v", it.Command, err)
			env.Warn("Stopped adding build commands. Add the remaining ones manually, in order:")
			for _, c := range manualBuildCommands(projectID, items) {
				env.Warn("  %s", c)
			}
			if manualBuildCommandsLoseSettings(items) {
				env.Warn("Halt-on-error and template settings for these steps can be adjusted in the DeployHQ dashboard.")
			}
			return
		}
		it.Status = suggestionAdded
	}
}

// manualBuildCommands returns, in order, the 'dhq build-commands create'
// command for every build command that failed or was not attempted. Values are
// shell-quoted so $VAR, $(...) and backticks in a command survive being pasted.
func manualBuildCommands(projectID string, items []suggestedItem) []string {
	var cmds []string
	for _, it := range items {
		if it.Status == suggestionFailed || it.Status == suggestionNotAttempted {
			c := fmt.Sprintf("dhq build-commands create -p %s --command %s", quoteRerunArg(projectID), quoteRerunArg(it.Command))
			if it.Description != "" {
				c += " --description " + quoteRerunArg(it.Description)
			}
			cmds = append(cmds, c)
		}
	}
	return cmds
}

// manualBuildCommandsLoseSettings reports whether any build command left for the
// user to add has a template name or halt-on-error setting, which 'dhq
// build-commands create' has no flags for.
func manualBuildCommandsLoseSettings(items []suggestedItem) bool {
	for _, it := range items {
		if (it.Status == suggestionFailed || it.Status == suggestionNotAttempted) &&
			(it.templateName != "" || it.haltOnError != nil) {
			return true
		}
	}
	return false
}

// applySuggestedPaths creates every planned path with create. Failures are
// recorded on the item and logged (they are an optimisation, not a blocker).
func applySuggestedPaths(env *output.Envelope, kind string, items []suggestedItem, create func(path string) error) {
	for i := range items {
		it := &items[i]
		if it.Status != suggestionAdd {
			continue
		}
		if err := create(it.Path); err != nil {
			it.Status, it.Error = suggestionFailed, err.Error()
			env.Logger.Write("could not add %s %q: %v", kind, it.Path, err)
			continue
		}
		it.Status = suggestionAdded
	}
}

func applySuggestedExcludedFiles(ctx context.Context, env *output.Envelope, client *sdk.Client, projectID string, items []suggestedItem) {
	applySuggestedPaths(env, "excluded file", items, func(path string) error {
		_, err := client.CreateExcludedFile(ctx, projectID, sdk.ExcludedFileCreateRequest{Path: path, AllServers: true})
		return err
	})
}

func applySuggestedBuildCacheFiles(ctx context.Context, env *output.Envelope, client *sdk.Client, projectID string, items []suggestedItem) {
	applySuggestedPaths(env, "build cache file", items, func(path string) error {
		_, err := client.CreateBuildCacheFile(ctx, projectID, sdk.BuildCacheFileCreateRequest{Path: path})
		return err
	})
}

// applySuggestedSettings applies every pending item of plan, in place.
func applySuggestedSettings(ctx context.Context, env *output.Envelope, client *sdk.Client, plan *suggestedSettingsPlan) {
	applySuggestedBuildCommands(ctx, env, client, plan.Project, plan.BuildCommands)
	applySuggestedExcludedFiles(ctx, env, client, plan.Project, plan.ExcludedFiles)
	applySuggestedBuildCacheFiles(ctx, env, client, plan.Project, plan.BuildCacheFiles)
}

// printSuggestedSettingsPlan writes a human summary of the plan to stderr.
func printSuggestedSettingsPlan(env *output.Envelope, plan *suggestedSettingsPlan) {
	stack := plan.Stack
	if stack == "" {
		stack = "no framework detected"
	}
	env.Status("Suggested settings for project %s (%s):", plan.Project, stack)
	section := func(title string, items []suggestedItem, label func(suggestedItem) string) {
		if len(items) == 0 {
			return
		}
		env.Status("  %s:", title)
		for _, it := range items {
			env.Status("    [%s] %s", it.Status, label(it))
		}
	}
	section("Build commands", plan.BuildCommands, func(it suggestedItem) string { return it.Command })
	section("Excluded files", plan.ExcludedFiles, func(it suggestedItem) string { return it.Path })
	section("Build cache files", plan.BuildCacheFiles, func(it suggestedItem) string { return it.Path })
	for _, it := range plan.BuildCommands {
		if it.Status == suggestionSkipped {
			env.Status("  Build commands are skipped because the project already has some (see 'dhq build-commands list -p %s').", plan.Project)
			break
		}
	}
}

// requireReadableDir rejects a path that isn't a readable directory. Detection
// on such a path sees no files, so a typo would otherwise run on nothing.
func requireReadableDir(dir string) error {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &output.UserError{Message: fmt.Sprintf("No such directory: %s", dir)}
	case err != nil:
		return &output.UserError{Message: fmt.Sprintf("Cannot read %s: %v", dir, err)}
	case !info.IsDir():
		return &output.UserError{Message: fmt.Sprintf("%s is not a directory", dir)}
	}
	if _, err := os.ReadDir(dir); err != nil {
		return &output.UserError{Message: fmt.Sprintf("Cannot read %s: %v", dir, err)}
	}
	return nil
}

// applySuggestionsFailureHint says how to finish a partly applied plan. Failed
// build commands must be added by hand, with the commands applySuggestedBuildCommands
// printed (a re-run skips build commands once the project has any); failed
// excluded or build cache files are added by a re-run.
func applySuggestionsFailureHint(plan *suggestedSettingsPlan) string {
	var hints []string
	if len(manualBuildCommands(plan.Project, plan.BuildCommands)) > 0 {
		hints = append(hints, "Add the remaining build commands manually, in order, with the 'dhq build-commands create' commands printed above; a re-run skips build commands once the project has some.")
	}
	pathFailed := false
	for _, it := range append(append([]suggestedItem{}, plan.ExcludedFiles...), plan.BuildCacheFiles...) {
		if it.Status == suggestionFailed {
			pathFailed = true
			break
		}
	}
	if pathFailed {
		what := "settings"
		if len(hints) > 0 {
			what = "excluded and build cache files"
		}
		hints = append(hints, fmt.Sprintf("Re-run 'dhq projects apply-suggestions -p %s' to retry the %s; only missing ones are added.", plan.Project, what))
	}
	return strings.Join(hints, "\n")
}

func newProjectsApplySuggestionsCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "apply-suggestions [path]",
		Short: "Apply DeployHQ's suggested build settings to a project",
		Long: `Detect the framework of a local directory (the same detection as 'dhq detect')
and add DeployHQ's suggested build settings to a project: build commands,
excluded files (e.g. dependency folders) and build cache files.

Works for any project, whatever servers it deploys to. Only missing settings are
added, so it is safe to re-run. Build commands are only added when the project
has none yet, so an existing build pipeline is never changed. Use --dry-run to
see what would be applied without changing anything.`,
		Example: `  # Preview the suggested settings for the current directory
  dhq projects apply-suggestions -p my-app --dry-run

  # Apply them
  dhq projects apply-suggestions -p my-app`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			if err := requireReadableDir(dir); err != nil {
				return err
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			ctx := cliCtx.Background()
			env := cliCtx.Envelope

			if _, err := client.GetProject(ctx, projectID); err != nil {
				return err
			}

			filenames, files := detect.CollectManifest(dir)
			resp, err := client.DetectFramework(ctx, sdk.DetectionPayload{Filenames: filenames, Files: files})
			if err != nil {
				return err
			}

			plan, err := planSuggestedSettings(ctx, client, projectID, detectionResultFromAPI(resp))
			if err != nil {
				return err
			}
			plan.DryRun = dryRun
			pending := plan.pending()

			printSuggestedSettingsPlan(env, plan)
			if pending > 0 && !dryRun {
				applySuggestedSettings(ctx, env, client, plan)
			}

			failed := plan.failed()
			summary := fmt.Sprintf("Applied suggested settings to %s", projectID)
			switch {
			case pending == 0:
				summary = fmt.Sprintf("Nothing to apply — %s already has the suggested settings", projectID)
			case dryRun:
				summary = fmt.Sprintf("Dry run: %d suggested setting(s) would be added to %s", pending, projectID)
			case len(failed) > 0:
				summary = fmt.Sprintf("%d of %d suggested setting(s) could not be added to %s", len(failed), pending, projectID)
			}
			var failErr error
			if len(failed) > 0 {
				env.Warn("Could not add: %s", strings.Join(failed, ", "))
				failErr = &output.UserError{
					Message: fmt.Sprintf("%s: %s", summary, strings.Join(failed, ", ")),
					Hint:    applySuggestionsFailureHint(plan),
				}
			}

			if env.WantsJSON() {
				resp := output.NewResponse(plan, summary,
					output.Breadcrumb{Action: "build-commands", Cmd: fmt.Sprintf("dhq build-commands list -p %s", projectID)},
				)
				resp.OK = failErr == nil
				if err := env.WriteJSON(resp); err != nil {
					return err
				}
				if failErr != nil {
					// The plan response above is the error report; main must
					// not write a second JSON object.
					return &output.ReportedError{Err: failErr}
				}
				return nil
			}
			if failErr != nil {
				return failErr
			}
			env.Status("%s", summary)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be applied without changing anything")
	return cmd
}
