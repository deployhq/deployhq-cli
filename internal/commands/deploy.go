package commands

import (
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// isUUID checks if a string looks like a UUID (contains dashes and hex chars).
func isUUID(s string) bool {
	return len(s) >= 32 && strings.ContainsRune(s, '-')
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
	var useLatest, wait bool

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy to a server (shortcut for deployments create)",
		Long:  "Create a deployment. Shortcut for 'dhq deployments create'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			env := cliCtx.Envelope

			// Auto-select server if not specified
			if server == "" {
				servers, err := client.ListServers(cliCtx.Background(), projectID)
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
				servers, err := client.ListServers(cliCtx.Background(), projectID)
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
			if revision == "" && !useLatest {
				env.Status("Fetching latest revision...")
				rev, err := client.GetLatestRevision(cliCtx.Background(), projectID)
				if err != nil {
					return &output.UserError{
						Message: "Could not fetch latest revision",
						Hint:    "Specify --revision or --use-latest",
					}
				}
				revision = rev
			}

			req := sdk.DeploymentCreateRequest{
				Branch:           branch,
				EndRevision:      revision,
				ParentIdentifier: server,
			}
			if cmd.Flags().Changed("use-latest") {
				req.UseLatest = &useLatest
			}

			dep, err := client.CreateDeployment(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}

			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(dep,
					fmt.Sprintf("Deployment %s queued", dep.Identifier),
					output.Breadcrumb{Action: "status", Cmd: fmt.Sprintf("dhq deployments show %s -p %s", dep.Identifier, projectID)},
					output.Breadcrumb{Action: "logs", Cmd: fmt.Sprintf("dhq deployments logs %s -p %s", dep.Identifier, projectID)},
					output.Breadcrumb{Action: "abort", Cmd: fmt.Sprintf("dhq deployments abort %s -p %s", dep.Identifier, projectID)},
				))
			}

			if wait {
				env.Status("🚀 Deployment %s queued", dep.Identifier)
				env.Status("")
				return watchDeployment(cliCtx.Background(), client, env, projectID, dep.Identifier)
			}

			env.Status("Deployment %s queued (status: %s)", dep.Identifier, output.ColorStatus(dep.Status))

			env.Status("\nNext:")
			env.Status("  dhq deployments show %s -p %s", dep.Identifier, projectID)
			env.Status("  dhq deployments logs %s -p %s", dep.Identifier, projectID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to deploy")
	cmd.Flags().StringVarP(&server, "server", "s", "", "Server or group identifier")
	cmd.Flags().StringVarP(&revision, "revision", "r", "", "End revision")
	cmd.Flags().BoolVar(&useLatest, "use-latest", false, "Deploy latest revision")
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait for deployment to complete")
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
