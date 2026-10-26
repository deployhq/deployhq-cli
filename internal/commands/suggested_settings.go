package commands

import (
	"context"
	"fmt"

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

// planSuggestedBuildCommands diffs the suggested build steps against the
// project. Build commands run as an ordered pipeline, so they are all-or-nothing:
// when the project already has any build command, every suggestion is skipped
// rather than merged into (or duplicated in) a pipeline someone set up already.
// A failed lookup does not block — the steps are planned as additions.
func planSuggestedBuildCommands(ctx context.Context, client *sdk.Client, projectID string, steps []detect.BuildCommandStep) []suggestedItem {
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
		return nil
	}
	if existing, err := client.ListBuildCommands(ctx, projectID, nil); err == nil && len(existing) > 0 {
		for i := range items {
			items[i].Status = suggestionSkipped
		}
	}
	return items
}

// planSuggestedPaths diffs suggested path patterns against the ones the project
// already has (as returned by list). A failed lookup plans every path.
func planSuggestedPaths(paths []string, list func() ([]string, error)) []suggestedItem {
	if len(paths) == 0 {
		return nil
	}
	existing := map[string]bool{}
	if have, err := list(); err == nil {
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
	return items
}

func planSuggestedExcludedFiles(ctx context.Context, client *sdk.Client, projectID string, paths []string) []suggestedItem {
	return planSuggestedPaths(paths, func() ([]string, error) {
		list, err := client.ListExcludedFiles(ctx, projectID, nil)
		out := make([]string, 0, len(list))
		for _, e := range list {
			out = append(out, e.Path)
		}
		return out, err
	})
}

func planSuggestedBuildCacheFiles(ctx context.Context, client *sdk.Client, projectID string, paths []string) []suggestedItem {
	return planSuggestedPaths(paths, func() ([]string, error) {
		list, err := client.ListBuildCacheFiles(ctx, projectID)
		out := make([]string, 0, len(list))
		for _, b := range list {
			out = append(out, b.Path)
		}
		return out, err
	})
}

// planSuggestedSettings builds the full plan for projectID from a detection result.
func planSuggestedSettings(ctx context.Context, client *sdk.Client, projectID string, detection detect.Result) *suggestedSettingsPlan {
	return &suggestedSettingsPlan{
		Project:         projectID,
		Stack:           string(detection.Framework),
		BuildCommands:   planSuggestedBuildCommands(ctx, client, projectID, detection.BuildCommands),
		ExcludedFiles:   planSuggestedExcludedFiles(ctx, client, projectID, detection.ExcludedFiles),
		BuildCacheFiles: planSuggestedBuildCacheFiles(ctx, client, projectID, detection.BuildCacheFiles),
	}
}

// applySuggestedBuildCommands creates every planned build command in order.
// Failures are non-fatal and leave a manual hint on stderr.
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
			env.Warn("Could not set the suggested build command (%q): %v", it.Command, err)
			env.Warn("Add it manually: dhq build-commands create -p %s --command %q", projectID, it.Command)
			continue
		}
		it.Status = suggestionAdded
	}
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

			plan := planSuggestedSettings(ctx, client, projectID, detectionResultFromAPI(resp))
			plan.DryRun = dryRun
			pending := plan.pending()

			printSuggestedSettingsPlan(env, plan)
			if pending > 0 && !dryRun {
				applySuggestedSettings(ctx, env, client, plan)
			}

			summary := fmt.Sprintf("Applied suggested settings to %s", projectID)
			switch {
			case pending == 0:
				summary = fmt.Sprintf("Nothing to apply — %s already has the suggested settings", projectID)
			case dryRun:
				summary = fmt.Sprintf("Dry run: %d suggested setting(s) would be added to %s", pending, projectID)
			}

			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(plan, summary,
					output.Breadcrumb{Action: "build-commands", Cmd: fmt.Sprintf("dhq build-commands list -p %s", projectID)},
				))
			}
			env.Status("%s", summary)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be applied without changing anything")
	return cmd
}
