package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newDeployCmd() *cobra.Command {
	var branch, server, revision string
	var useLatest bool

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

			// Auto-select server if not specified and project has exactly one
			if server == "" {
				servers, err := client.ListServers(cliCtx.Background(), projectID)
				if err == nil && len(servers) == 1 {
					server = servers[0].Identifier
					env.Status("Auto-selected server: %s", servers[0].Name)
				} else if err == nil && len(servers) > 1 {
					env.Status("Available servers:")
					for _, s := range servers {
						env.Status("  %s (%s)", s.Name, s.Identifier)
					}
					return &output.UserError{
						Message: "Multiple servers found — specify which one",
						Hint:    fmt.Sprintf("Use --server <identifier>, e.g. dhq deploy -p %s -s %s", projectID, servers[0].Identifier),
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

			env.Status("Deployment %s queued (status: %s)", dep.Identifier, dep.Status)
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
	return cmd
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
