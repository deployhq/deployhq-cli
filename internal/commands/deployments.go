package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newDeploymentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deployments",
		Aliases: []string{"deployment", "dep"},
		Short:   "Manage deployments",
	}

	cmd.AddCommand(
		newDeploymentsListCmd(),
		newDeploymentsShowCmd(),
		newDeploymentsCreateCmd(),
		newDeploymentsAbortCmd(),
		newDeploymentsRollbackCmd(),
		newDeploymentsLogsCmd(),
	)

	return cmd
}

func newDeploymentsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List deployments",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			result, err := client.ListDeployments(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(result,
					fmt.Sprintf("%d deployments (page %d/%d)", len(result.Records), result.Pagination.CurrentPage, result.Pagination.TotalPages),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("deployhq deployments show <id> -p %s", projectID)},
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("deployhq deploy -p %s", projectID)},
				))
			}

			columns := []string{"Identifier", "Status", "Branch", "Deployer", "Queued"}
			rows := make([][]string, len(result.Records))
			for i, d := range result.Records {
				deployer := "-"
				if d.Deployer != nil {
					deployer = *d.Deployer
				}
				queued := "-"
				if d.Timestamps != nil {
					queued = d.Timestamps.QueuedAt
				}
				rows[i] = []string{d.Identifier, d.Status, d.Branch, deployer, queued}
			}
			env.WriteTable(columns, rows)
			return nil
		},
	}
}

func newDeploymentsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <identifier>",
		Short: "Show deployment details",
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

			dep, err := client.GetDeployment(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				crumbs := []output.Breadcrumb{
					{Action: "logs", Cmd: fmt.Sprintf("deployhq deployments logs %s -p %s", dep.Identifier, projectID)},
				}
				if dep.Status == "completed" {
					crumbs = append(crumbs, output.Breadcrumb{Action: "rollback", Cmd: fmt.Sprintf("deployhq rollback %s -p %s", dep.Identifier, projectID)})
				}
				if dep.Status == "running" {
					crumbs = append(crumbs, output.Breadcrumb{Action: "abort", Cmd: fmt.Sprintf("deployhq deployments abort %s -p %s", dep.Identifier, projectID)})
				}
				return env.WriteJSON(output.NewResponse(dep,
					fmt.Sprintf("Deployment %s: %s", dep.Identifier, dep.Status),
					crumbs...))
			}

			deployer := "-"
			if dep.Deployer != nil {
				deployer = *dep.Deployer
			}

			rows := [][]string{
				{"Identifier", dep.Identifier},
				{"Status", dep.Status},
				{"Branch", dep.Branch},
				{"Deployer", deployer},
			}

			if dep.StartRevision != nil {
				rows = append(rows, []string{"Start Revision", dep.StartRevision.Ref})
			}
			if dep.EndRevision != nil {
				rows = append(rows, []string{"End Revision", dep.EndRevision.Ref})
			}
			if dep.Timestamps != nil {
				rows = append(rows, []string{"Queued At", dep.Timestamps.QueuedAt})
				if dep.Timestamps.Duration != nil {
					rows = append(rows, []string{"Duration", dep.Timestamps.Duration.String() + "s"})
				}
			}
			rows = append(rows, []string{"Servers", fmt.Sprintf("%d", len(dep.Servers))})

			env.WriteTable([]string{"Field", "Value"}, rows)

			if len(dep.Steps) > 0 {
				env.Status("\nSteps:")
				stepCols := []string{"Step", "Stage", "Status", "Description"}
				stepRows := make([][]string, len(dep.Steps))
				for i, s := range dep.Steps {
					stepRows[i] = []string{s.Step, s.Stage, s.Status, s.Description}
				}
				env.WriteTable(stepCols, stepRows)
			}
			return nil
		},
	}
}

func newDeploymentsCreateCmd() *cobra.Command {
	var branch, endRevision, serverID, parentID string
	var copyConfig, runBuild, useCache, useLatest bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.DeploymentCreateRequest{
				Branch:           branch,
				EndRevision:      endRevision,
				ServerIdentifier: serverID,
				ParentIdentifier: parentID,
			}
			if cmd.Flags().Changed("copy-config") {
				req.CopyConfigFiles = &copyConfig
			}
			if cmd.Flags().Changed("run-build") {
				req.RunBuildCommands = &runBuild
			}
			if cmd.Flags().Changed("use-cache") {
				req.UseBuildCache = &useCache
			}
			if cmd.Flags().Changed("use-latest") {
				req.UseLatest = &useLatest
			}

			dep, err := client.CreateDeployment(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(dep,
					fmt.Sprintf("Deployment %s queued", dep.Identifier),
					output.Breadcrumb{Action: "status", Cmd: fmt.Sprintf("deployhq deployments show %s -p %s", dep.Identifier, projectID)},
					output.Breadcrumb{Action: "logs", Cmd: fmt.Sprintf("deployhq deployments logs %s -p %s", dep.Identifier, projectID)},
				))
			}

			env.Status("Deployment %s queued (status: %s)", dep.Identifier, dep.Status)
			env.Status("\nNext: deployhq deployments show %s -p %s", dep.Identifier, projectID)
			return nil
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "Branch to deploy")
	cmd.Flags().StringVar(&endRevision, "revision", "", "End revision")
	cmd.Flags().StringVar(&serverID, "server", "", "Server identifier")
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent identifier (server or group)")
	cmd.Flags().BoolVar(&copyConfig, "copy-config", false, "Copy config files")
	cmd.Flags().BoolVar(&runBuild, "run-build", true, "Run build commands")
	cmd.Flags().BoolVar(&useCache, "use-cache", true, "Use build cache")
	cmd.Flags().BoolVar(&useLatest, "use-latest", false, "Deploy latest revision")
	return cmd
}

func newDeploymentsAbortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "abort <identifier>",
		Short: "Abort a running deployment",
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

			if err := client.AbortDeployment(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Aborted deployment: %s", args[0])
			return nil
		},
	}
}

func newDeploymentsRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback <identifier>",
		Short: "Rollback a deployment",
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
					output.Breadcrumb{Action: "status", Cmd: fmt.Sprintf("deployhq deployments show %s -p %s", dep.Identifier, projectID)},
				))
			}

			env.Status("Rollback deployment %s queued (status: %s)", dep.Identifier, dep.Status)
			return nil
		},
	}
}

func newDeploymentsLogsCmd() *cobra.Command {
	var stepID string

	cmd := &cobra.Command{
		Use:   "logs <deployment-id>",
		Short: "Show deployment step logs",
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

			// If no step ID provided, first get the deployment to find steps
			if stepID == "" {
				dep, err := client.GetDeployment(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				if len(dep.Steps) == 0 {
					cliCtx.Envelope.Status("No steps found for deployment %s", args[0])
					return nil
				}
				// Show logs for all steps that have logs
				for _, step := range dep.Steps {
					if !step.Logs {
						continue
					}
					logs, err := client.GetDeploymentStepLogs(cliCtx.Background(), projectID, args[0], step.Identifier)
					if err != nil {
						continue
					}
					cliCtx.Envelope.Status("\n--- %s (%s) ---", step.Description, step.Status)
					for _, log := range logs {
						fmt.Fprintln(cliCtx.Envelope.Stdout, log.Message) //nolint:errcheck
					}
				}
				return nil
			}

			logs, err := client.GetDeploymentStepLogs(cliCtx.Background(), projectID, args[0], stepID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(logs, fmt.Sprintf("%d log entries", len(logs))))
			}

			for _, log := range logs {
				fmt.Fprintln(env.Stdout, log.Message) //nolint:errcheck
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&stepID, "step", "", "Specific step identifier")
	return cmd
}
