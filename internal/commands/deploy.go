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

			// Auto-select server if not specified
			if server == "" {
				servers, err := client.ListServers(cliCtx.Background(), projectID, nil)
				if err == nil && len(servers) == 1 {
					server = servers[0].Identifier
					env.Status("Auto-selected server: %s", servers[0].Name)
				} else if err == nil && len(servers) > 1 {
					if env.IsTTY {
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
						env.Status("Selected server: %s", servers[idx].Name)
					} else {
						return &output.UserError{
							Message: "Multiple servers found — specify which one",
							Hint:    fmt.Sprintf("Use --server <identifier>, e.g. dhq deploy -p %s -s %s", projectID, servers[0].Identifier),
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
					} else if len(candidates) > 0 && env.IsTTY {
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
					} else if len(candidates) > 0 {
						return &output.UserError{
							Message: fmt.Sprintf("Multiple servers match %q — specify which one", server),
							Hint:    fmt.Sprintf("Use the full identifier, e.g. dhq deploy -p %s -s %s", projectID, candidates[0].Identifier),
						}
					}
				}
			}

			// Auto-fetch latest revision if none specified
			if revision == "" {
				env.Status("Fetching latest revision...")
				rev, err := resolveLatestRevision(cliCtx.Background(), client, projectID)
				if err != nil {
					return err
				}
				revision = rev
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
					output.Breadcrumb{Action: "watch", Cmd: fmt.Sprintf("dhq deployments watch %s -p %s", dep.Identifier, projectID)},
					output.Breadcrumb{Action: "status", Cmd: fmt.Sprintf("dhq deployments show %s -p %s", dep.Identifier, projectID)},
					output.Breadcrumb{Action: "logs", Cmd: fmt.Sprintf("dhq deployments logs %s -p %s", dep.Identifier, projectID)},
					output.Breadcrumb{Action: "abort", Cmd: fmt.Sprintf("dhq deployments abort %s -p %s", dep.Identifier, projectID)},
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.ExactArgs(1),
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
