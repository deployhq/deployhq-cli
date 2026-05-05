package commands

import (
	"context"
	"fmt"
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
// Branch resolution: explicit --branch → server's PreferredBranch (or Branch) → "" (use repo default).
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
) (branch, revision string, err error) {
	branch = flagBranch
	revision = flagRevision

	if branch == "" && serverIdentifier != "" {
		srv := knownServer
		if srv == nil {
			// GetServer 404s for server-group identifiers; that's expected — we fall through.
			if s, gerr := client.GetServer(ctx, projectID, serverIdentifier); gerr == nil {
				srv = s
			}
		}
		if srv != nil {
			switch {
			case srv.PreferredBranch != "":
				branch = srv.PreferredBranch
			case srv.Branch != "":
				branch = srv.Branch
			}
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
	var branch, server, revision string
	var dryRun, wait bool
	var timeout int

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy to a server (shortcut for deployments create)",
		Long:  "Create a deployment. Shortcut for 'dhq deployments create'.\n\nUse --wait (-w) to watch the deployment until it completes or fails.",
		Example: `  # Deploy the latest revision (auto-selects the only server, uses the server's preferred branch)
  dhq deploy

  # Deploy a specific branch and watch until it finishes
  dhq deploy --branch develop --wait

  # Deploy to a specific server, watching with a 10-minute timeout
  dhq deploy -s production -w --timeout 600

  # Preview what would be deployed without executing
  dhq deploy --dry-run

  # Deploy a specific commit
  dhq deploy --revision a1b2c3d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			if dryRun && wait {
				return &output.UserError{Message: "--dry-run and --wait are mutually exclusive"}
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			env := cliCtx.Envelope

			// Track the Server we resolved to, so branch/revision lookup can reuse it
			// without an extra GetServer round-trip.
			var resolvedServer *sdk.Server

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
					if resolved != "" {
						server = resolved
						for i := range servers {
							if servers[i].Identifier == resolved {
								resolvedServer = &servers[i]
								break
							}
						}
					} else if len(candidates) > 0 && !env.NonInteractive {
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

			if branch == "" || revision == "" {
				env.Status("Resolving branch and revision...")
				resolvedBranch, resolvedRev, err := resolveBranchAndRevision(
					cliCtx.Background(), client, projectID, server, branch, revision, resolvedServer,
				)
				if err != nil {
					return err
				}
				branch = resolvedBranch
				revision = resolvedRev
			}

			req := sdk.DeploymentCreateRequest{
				Branch:           branch,
				EndRevision:      revision,
				ParentIdentifier: server,
			}

			if dryRun {
				preview, err := client.PreviewDeployment(cliCtx.Background(), projectID, req)
				if err != nil {
					return err
				}

				if env.JSONMode || !env.IsTTY {
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
				return err
			}

			if env.JSONMode || !env.IsTTY {
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be deployed without executing")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait for deployment to complete")
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
			if env.JSONMode || !env.IsTTY {
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
			if env.JSONMode || !env.IsTTY {
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
