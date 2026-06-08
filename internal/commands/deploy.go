package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// isUUID checks if a string looks like a UUID (contains dashes and hex chars).
func isUUID(s string) bool {
	return len(s) >= 32 && strings.ContainsRune(s, '-')
}

// resolveBranchAndRevision determines the branch and end revision for a deployment.
//
// Branch resolution: explicit --branch → server's PreferredBranch (or Branch) → group's
// PreferredBranch → "" (use repo default).
// Revision resolution: explicit --revision → tip SHA of the resolved branch → repo default tip.
//
// Sending end_revision without ensuring it matches the chosen branch was the source of two
// user-reported bugs: server.preferred_branch was ignored, and --branch foo deployed main's tip
// because /repository/latest_revision is branch-agnostic and end_revision overrides branch on the API.
func resolveBranchAndRevision(
	ctx context.Context,
	client *sdk.Client,
	projectID, serverIdentifier, flagBranch, flagRevision string,
	knownServer *sdk.Server,
	knownGroup *sdk.ServerGroup,
) (branch, revision string, err error) {
	branch = flagBranch
	revision = flagRevision

	if branch == "" && serverIdentifier != "" {
		srv := knownServer
		grp := knownGroup
		if srv == nil && grp == nil {
			// GetServer 404s for server-group identifiers; fall through to GetServerGroup.
			if s, gerr := client.GetServer(ctx, projectID, serverIdentifier); gerr == nil {
				srv = s
			} else if g, gerr := client.GetServerGroup(ctx, projectID, serverIdentifier); gerr == nil {
				grp = g
			}
		}
		switch {
		case srv != nil && srv.PreferredBranch != "":
			branch = srv.PreferredBranch
		case srv != nil && srv.Branch != "":
			branch = srv.Branch
		case grp != nil && grp.PreferredBranch != "":
			branch = grp.PreferredBranch
		}
	}

	if revision != "" {
		return branch, revision, nil
	}

	if branch != "" {
		branches, lerr := client.ListBranches(ctx, projectID, nil)
		if lerr == nil {
			sha, ok := branches[branch]
			if !ok || sha == "" {
				return "", "", &output.UserError{
					Message: fmt.Sprintf("Branch %q not found in repository", branch),
					Hint:    "Run 'dhq repos branches' to list available branches",
				}
			}
			return branch, sha, nil
		}
		// ListBranches failed — fall through to repo-default revision.
	}

	rev, ferr := resolveLatestRevision(ctx, client, projectID)
	if ferr != nil {
		return "", "", ferr
	}
	return branch, rev, nil
}

// resolveStartRevision determines the start_revision for a deployment.
//
// Resolution order: --full → "" (deploy entire branch) → explicit --start-revision →
// resolved server's LastRevision → resolved group's LastRevision → "".
//
// Sending an empty start_revision to the API means "deploy entire branch from the
// first commit". That was the source of issue #5 (server case) and the DHQ-586
// follow-up (group case): without populating this field every deploy looked like
// an initial one. ServerGroup.LastRevision is the end_revision of the most recent
// deployment that targeted the group (Rails: ServerGroup#last_revision), which is
// the correct incremental baseline. Newly-created groups have an empty value, so
// the first group deploy still falls through to a full deploy.
func resolveStartRevision(server *sdk.Server, group *sdk.ServerGroup, flagStart string, full bool) string {
	if full {
		return ""
	}
	if flagStart != "" {
		return flagStart
	}
	if server != nil && server.LastRevision != "" {
		return server.LastRevision
	}
	if group != nil && group.LastRevision != "" {
		return group.LastRevision
	}
	return ""
}

// translateParentMustExistError detects the API's 422 "parent must exist"
// validation failure and rewrites it as a UserError that explains the
// situation and what to do.
//
// The API rejects a deployment when its start_revision doesn't trace to a
// prior deployment's end_revision on the same target. Two real-world causes:
//   - the SHA was force-pushed/rebased out of the repo while the server's
//     LastRevision still points at it
//   - the user passed --start-revision <sha> with a SHA that was never the
//     end of a prior deploy on this target
//
// Without translation the user sees "deployhq api: 422 parent must exist",
// which is opaque. The replacement names the SHA, explains the cause, and
// offers --full as an escape hatch.
func translateParentMustExistError(err error, startRevision string) error {
	if err == nil {
		return nil
	}
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		return err
	}
	if !mentionsParentMustExist(apiErr) {
		return err
	}
	sha := startRevision
	if sha == "" {
		sha = "(none)"
	}
	return &output.UserError{
		Message: fmt.Sprintf("No prior deployment matches start_revision %s on this target", sha),
		Hint: "Incremental deploys need a prior deployment whose end_revision equals this SHA. Likely the SHA was force-pushed away or was never deployed here.\n" +
			"  --full                          deploy the entire branch from the first commit\n" +
			"  --start-revision <sha>          use a SHA that ended a prior deploy (see 'dhq deployments list')",
	}
}

func mentionsParentMustExist(e *sdk.APIError) bool {
	if strings.Contains(e.Message, "parent must exist") {
		return true
	}
	for _, msg := range e.Errors {
		if strings.Contains(msg, "parent must exist") {
			return true
		}
	}
	return false
}

// resolveLatestRevision tries to find the latest revision for a project,
// falling back to the most recent deployment's end revision if the
// repository endpoint fails (e.g. empty repo, missing default branch).
func resolveLatestRevision(ctx context.Context, client *sdk.Client, projectID string) (string, error) {
	// Primary: /repository/latest_revision
	rev, primaryErr := client.GetLatestRevision(ctx, projectID)
	if primaryErr == nil && rev != "" {
		return rev, nil
	}

	// Fallback: most recent deployment's end revision
	deps, depsErr := client.ListDeployments(ctx, projectID, nil)
	if depsErr == nil && deps != nil {
		for _, d := range deps.Records {
			if d.EndRevision != nil && d.EndRevision.Ref != "" {
				return d.EndRevision.Ref, nil
			}
		}
	}

	// Both failed — surface the primary error with an actionable hint.
	hint := "Specify a revision with --revision <sha>"
	if primaryErr != nil {
		hint = fmt.Sprintf("API error: %v\nSpecify a revision with --revision <sha>", primaryErr)
	}
	return "", &output.UserError{
		Message: "Could not fetch latest revision",
		Hint:    hint,
	}
}

// resolveServerName matches a user-provided server name to a server identifier.
// Returns the identifier if a single match is found, or empty string + candidates for picker.
func resolveServerName(input string, servers []sdk.Server) (string, []sdk.Server) {
	normalized := normalize(input)

	// Exact case-insensitive match
	for _, s := range servers {
		if strings.EqualFold(s.Name, input) {
			return s.Identifier, nil
		}
	}

	// Normalized match (e.g. "DO-FEDORA" matches "DO - FEDORA")
	var normalizedMatches []sdk.Server
	for _, s := range servers {
		if normalize(s.Name) == normalized {
			normalizedMatches = append(normalizedMatches, s)
		}
	}
	if len(normalizedMatches) == 1 {
		return normalizedMatches[0].Identifier, nil
	}

	// Contains match (e.g. "fedora" matches "DO - FEDORA")
	lower := strings.ToLower(input)
	var containsMatches []sdk.Server
	for _, s := range servers {
		if strings.Contains(strings.ToLower(s.Name), lower) {
			containsMatches = append(containsMatches, s)
		}
	}
	if len(containsMatches) == 1 {
		return containsMatches[0].Identifier, nil
	}
	if len(containsMatches) > 1 {
		return "", containsMatches
	}

	// No matches — return all servers as candidates
	return "", servers
}

// resolveGroupName matches a user-provided name to a server-group identifier.
// Mirrors resolveServerName's exact / normalized / contains tiers. Returns the
// identifier on a unique match, or "" when ambiguous or no match. The group's
// display name is also returned for user-facing status messages.
func resolveGroupName(input string, groups []sdk.ServerGroup) (identifier, name string) {
	normalized := normalize(input)

	for _, g := range groups {
		if strings.EqualFold(g.Name, input) {
			return g.Identifier, g.Name
		}
	}

	var normalizedMatches []sdk.ServerGroup
	for _, g := range groups {
		if normalize(g.Name) == normalized {
			normalizedMatches = append(normalizedMatches, g)
		}
	}
	if len(normalizedMatches) == 1 {
		return normalizedMatches[0].Identifier, normalizedMatches[0].Name
	}

	lower := strings.ToLower(input)
	var containsMatches []sdk.ServerGroup
	for _, g := range groups {
		if strings.Contains(strings.ToLower(g.Name), lower) {
			containsMatches = append(containsMatches, g)
		}
	}
	if len(containsMatches) == 1 {
		return containsMatches[0].Identifier, containsMatches[0].Name
	}

	return "", ""
}

// normalize lowercases and collapses all non-alphanumeric chars.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func newDeployCmd() *cobra.Command {
	var branch, server, revision, startRevision string
	var dryRun, wait, full bool
	var timeout int

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy to a server (shortcut for deployments create)",
		Long:  "Create a deployment. Shortcut for 'dhq deployments create'.\n\nBy default deploys are incremental: the start revision defaults to the server's last successfully deployed commit. Use --full to deploy the entire branch from the first commit.\n\nUse --wait (-w) to watch the deployment until it completes or fails.",
		Example: `  # Deploy the latest revision (auto-selects the only server, uses the server's preferred branch)
  dhq deploy

  # Deploy a specific branch and watch until it finishes
  dhq deploy --branch develop --wait

  # Deploy to a specific server, watching with a 10-minute timeout
  dhq deploy -s production -w --timeout 600

  # Preview what would be deployed without executing
  dhq deploy --dry-run

  # Deploy a specific commit
  dhq deploy --revision a1b2c3d

  # Deploy from a specific start commit (useful for hotfixes)
  dhq deploy --start-revision a1b2c3d --revision e4f5g6h

  # Deploy the entire branch from the first commit (overrides incremental default)
  dhq deploy --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			if dryRun && wait {
				return &output.UserError{
					Message: "--dry-run and --wait are mutually exclusive",
					Hint:    "--dry-run creates a preview that doesn't execute, so there's nothing to wait for. Drop --wait to preview, or drop --dry-run to deploy and watch.",
				}
			}

			if full && startRevision != "" {
				return &output.UserError{Message: "--full and --start-revision are mutually exclusive"}
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			env := cliCtx.Envelope

			// Track the Server or ServerGroup we resolved to, so branch/revision lookup
			// can reuse them without extra round-trips. Exactly one of these will be
			// non-nil once the target is locked in (or both nil for a project-wide deploy).
			var resolvedServer *sdk.Server
			var resolvedGroup *sdk.ServerGroup

			// Auto-select server if not specified
			if server == "" {
				servers, err := client.ListServers(cliCtx.Background(), projectID, nil)
				if err == nil && len(servers) == 1 {
					server = servers[0].Identifier
					resolvedServer = &servers[0]
					env.Status("Auto-selected server: %s", servers[0].Name)
				} else if err == nil && len(servers) > 1 {
					if !env.NonInteractive {
						// Interactive picker
						items := make([]string, len(servers))
						for i, s := range servers {
							items[i] = fmt.Sprintf("%s (%s)", s.Name, s.ProtocolType)
						}
						prompt := promptui.Select{
							Label: "Select server",
							Items: items,
						}
						idx, _, err := prompt.Run()
						if err != nil {
							return &output.UserError{Message: "Server selection cancelled"}
						}
						server = servers[idx].Identifier
						resolvedServer = &servers[idx]
						env.Status("Selected server: %s", servers[idx].Name)
					} else {
						names := make([]string, len(servers))
						for i, s := range servers {
							names[i] = fmt.Sprintf("%s (%s)", s.Identifier, s.Name)
						}
						return &output.UserError{
							Message: "Multiple servers found — specify which one",
							Hint:    fmt.Sprintf("Use --server <identifier>. Available: %s", strings.Join(names, ", ")),
						}
					}
				}
			}

			// Resolve server name to identifier if needed
			if server != "" && !isUUID(server) {
				servers, err := client.ListServers(cliCtx.Background(), projectID, nil)
				if err == nil {
					resolved, candidates := resolveServerName(server, servers)
					matched := false
					if resolved != "" {
						server = resolved
						for i := range servers {
							if servers[i].Identifier == resolved {
								resolvedServer = &servers[i]
								break
							}
						}
						matched = true
					}
					// Server groups are deployable targets too. Fall back to group-name
					// resolution before the per-server picker so `-s "My Group"` works
					// (documented at deployhq.com/support/cli/cli-deploying).
					if !matched {
						if groups, gerr := client.ListServerGroups(cliCtx.Background(), projectID, nil); gerr == nil {
							if groupID, _ := resolveGroupName(server, groups); groupID != "" {
								for i := range groups {
									if groups[i].Identifier == groupID {
										resolvedGroup = &groups[i]
										break
									}
								}
								server = groupID
								resolvedServer = nil // groups don't map to a single Server
								env.Status("Resolved to server group: %s", resolvedGroup.Name)
								matched = true
							}
						}
					}
					if !matched {
						if len(candidates) > 0 && !env.NonInteractive {
							items := make([]string, len(candidates))
							for i, s := range candidates {
								items[i] = fmt.Sprintf("%s (%s)", s.Name, s.ProtocolType)
							}
							prompt := promptui.Select{
								Label: fmt.Sprintf("Multiple servers match %q", server),
								Items: items,
							}
							idx, _, err := prompt.Run()
							if err != nil {
								return &output.UserError{Message: "Server selection cancelled"}
							}
							server = candidates[idx].Identifier
							resolvedServer = &candidates[idx]
						} else if len(candidates) > 0 {
							names := make([]string, len(candidates))
							for i, s := range candidates {
								names[i] = fmt.Sprintf("%s (%s)", s.Identifier, s.Name)
							}
							return &output.UserError{
								Message: fmt.Sprintf("Multiple servers match %q — specify which one", server),
								Hint:    fmt.Sprintf("Use the full identifier. Matches: %s", strings.Join(names, ", ")),
							}
						}
					}
				}
			}

			// Eagerly fetch the target when caller didn't already resolve it
			// (e.g. user passed a UUID directly). We need either Server.LastRevision or
			// ServerGroup.LastRevision for start_revision resolution. GetServer 404s for
			// server-group identifiers, so fall back to GetServerGroup before giving up.
			if resolvedServer == nil && resolvedGroup == nil && server != "" {
				if s, gerr := client.GetServer(cliCtx.Background(), projectID, server); gerr == nil {
					resolvedServer = s
				} else if g, gerr := client.GetServerGroup(cliCtx.Background(), projectID, server); gerr == nil {
					resolvedGroup = g
				}
			}

			if branch == "" || revision == "" {
				env.Status("Resolving branch and revision...")
				resolvedBranch, resolvedRev, err := resolveBranchAndRevision(
					cliCtx.Background(), client, projectID, server, branch, revision, resolvedServer, resolvedGroup,
				)
				if err != nil {
					return err
				}
				branch = resolvedBranch
				revision = resolvedRev
			}

			req := sdk.DeploymentCreateRequest{
				StartRevision:    resolveStartRevision(resolvedServer, resolvedGroup, startRevision, full),
				Branch:           branch,
				EndRevision:      revision,
				ParentIdentifier: server,
			}

			if dryRun {
				preview, err := client.PreviewDeployment(cliCtx.Background(), projectID, req)
				if err != nil {
					return translateParentMustExistError(err, req.StartRevision)
				}

				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(preview,
						fmt.Sprintf("Preview %s created (status: %s)", preview.Identifier, preview.Status),
						output.Breadcrumb{Action: "execute", Cmd: deployExecuteCmd(projectID, server, branch)},
					))
				}

				env.Status("DRY RUN — preview created, deployment will NOT execute\n")
				env.Status("  Identifier: %s", preview.Identifier)
				env.Status("  Status:     %s", output.ColorStatus(preview.Status))
				env.Status("\nThe preview will be processed asynchronously.")
				env.Status("Preview deployments are excluded from 'dhq deployments list'.")
				env.Status("\nUse 'dhq deploy' (without --dry-run) to execute.")
				return nil
			}

			dep, err := client.CreateDeployment(cliCtx.Background(), projectID, req)
			if err != nil {
				return translateParentMustExistError(err, req.StartRevision)
			}

			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(dep,
					fmt.Sprintf("Deployment %s queued", dep.Identifier),
					output.Breadcrumb{Action: "watch", Cmd: fmt.Sprintf("dhq deployments watch %s -p %s", dep.Identifier, projectID), Resource: "deployment", ID: dep.Identifier},
					output.Breadcrumb{Action: "status", Cmd: fmt.Sprintf("dhq deployments show %s -p %s", dep.Identifier, projectID), Resource: "deployment", ID: dep.Identifier},
					output.Breadcrumb{Action: "logs", Cmd: fmt.Sprintf("dhq deployments logs %s -p %s", dep.Identifier, projectID), Resource: "deployment", ID: dep.Identifier},
					output.Breadcrumb{Action: "abort", Cmd: fmt.Sprintf("dhq deployments abort %s -p %s", dep.Identifier, projectID), Resource: "deployment", ID: dep.Identifier},
				))
			}

			if wait {
				env.Status("🚀 Deployment %s queued", dep.Identifier)
				env.Status("")
				ctx := cliCtx.Background()
				if timeout > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
					defer cancel()
				}
				err := watchDeployment(ctx, client, env, projectID, dep.Identifier)
				if ctx.Err() == context.DeadlineExceeded {
					return &output.UserError{
						Message: fmt.Sprintf("Timed out after %ds waiting for deployment to complete", timeout),
						Hint:    fmt.Sprintf("dhq deployments show %s -p %s", dep.Identifier, projectID),
					}
				}
				return err
			}

			env.Status("Deployment %s queued (status: %s)", dep.Identifier, output.ColorStatus(dep.Status))

			env.Status("\nNext:")
			env.Status("  dhq deployments watch %s -p %s", dep.Identifier, projectID)
			env.Status("  dhq deployments logs %s -p %s", dep.Identifier, projectID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to deploy")
	cmd.Flags().StringVarP(&server, "server", "s", "", "Server or group identifier")
	cmd.Flags().StringVarP(&revision, "revision", "r", "", "End revision (default: latest)")
	cmd.Flags().StringVar(&startRevision, "start-revision", "", "Start revision (default: server's last deployed commit)")
	cmd.Flags().BoolVar(&full, "full", false, "Deploy entire branch from the first commit (overrides the incremental default)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be deployed without executing (cannot combine with --wait)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait for deployment to complete (cannot combine with --dry-run)")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Timeout in seconds when using --wait (0 = no timeout)")
	return cmd
}

func deployExecuteCmd(projectID, server, branch string) string {
	cmd := fmt.Sprintf("dhq deploy -p %s -s %s", projectID, server)
	if branch != "" {
		cmd += " -b " + branch
	}
	return cmd
}

func newRetryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retry <deployment-id>",
		Short: "Retry a deployment (shortcut for deployments retry)",
		Example: `  # Retry a failed deployment
  dhq retry dep-abc123

  # Retry and watch the result
  dhq retry dep-abc123 && dhq deployments watch dep-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			dep, err := client.RetryDeployment(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(dep,
					fmt.Sprintf("Retry deployment %s queued", dep.Identifier),
					output.Breadcrumb{Action: "status", Cmd: fmt.Sprintf("dhq deployments show %s -p %s", dep.Identifier, projectID)},
				))
			}
			env.Status("Retry deployment %s queued (status: %s)", dep.Identifier, dep.Status)
			return nil
		},
	}
}

func newRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback <deployment-id>",
		Short: "Rollback a deployment (shortcut for deployments rollback)",
		Example: `  # Roll back a deployment to the previous revision
  dhq rollback dep-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			dep, err := client.RollbackDeployment(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(dep,
					fmt.Sprintf("Rollback deployment %s queued", dep.Identifier),
					output.Breadcrumb{Action: "status", Cmd: fmt.Sprintf("dhq deployments show %s -p %s", dep.Identifier, projectID)},
				))
			}

			env.Status("Rollback deployment %s queued (status: %s)", dep.Identifier, dep.Status)
			return nil
		},
	}
}
