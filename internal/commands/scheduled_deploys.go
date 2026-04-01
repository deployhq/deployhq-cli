package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newScheduledDeploysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduled-deploys",
		Short: "Manage scheduled deployments",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List scheduled deployments",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				result, err := client.ListScheduledDeployments(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(result, fmt.Sprintf("%d scheduled deployments", len(result))))
				}
				rows := make([][]string, len(result))
				for i, s := range result {
					rows[i] = []string{s.Identifier, s.Server, s.Frequency, s.NextDeploymentAt}
				}
				env.WriteTable([]string{"Identifier", "Server", "Frequency", "Next At"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show a scheduled deployment", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				s, err := client.GetScheduledDeployment(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(s, fmt.Sprintf("Scheduled: %s (%s)", s.Identifier, s.Frequency)))
			},
		},
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a scheduled deployment", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteScheduledDeployment(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted scheduled deployment: %s", args[0])
				return nil
			},
		},
		newScheduledDeploysCreateCmd(),
		newScheduledDeploysUpdateCmd(),
	)
	return cmd
}

func newScheduledDeploysUpdateCmd() *cobra.Command {
	var (
		server        string
		frequency     string
		at            string
		startRevision string
		endRevision   string
		copyConfig    bool
		runBuild      bool
		useCache      bool
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a scheduled deployment",
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
			req := sdk.ScheduledDeploymentUpdateRequest{}
			if cmd.Flags().Changed("server") {
				req.ServerIdentifier = server
			}
			if cmd.Flags().Changed("frequency") {
				req.Frequency = frequency
			}
			if cmd.Flags().Changed("at") {
				req.At = at
			}
			if cmd.Flags().Changed("start-revision") {
				req.StartRevision = startRevision
			}
			if cmd.Flags().Changed("end-revision") {
				req.EndRevision = endRevision
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
			s, err := client.UpdateScheduledDeployment(cliCtx.Background(), projectID, args[0], req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(s, fmt.Sprintf("Updated scheduled deployment: %s", s.Identifier)))
			}
			env.Status("Updated scheduled deployment: %s (%s at %s)", s.Identifier, s.Frequency, s.At)
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "Server identifier")
	cmd.Flags().StringVar(&frequency, "frequency", "", "Deployment frequency, e.g. daily, weekly")
	cmd.Flags().StringVar(&at, "at", "", "Time to deploy, e.g. 14:30")
	cmd.Flags().StringVar(&startRevision, "start-revision", "", "Start revision")
	cmd.Flags().StringVar(&endRevision, "end-revision", "", "End revision")
	cmd.Flags().BoolVar(&copyConfig, "copy-config", false, "Copy config files")
	cmd.Flags().BoolVar(&runBuild, "run-build", false, "Run build commands")
	cmd.Flags().BoolVar(&useCache, "use-cache", false, "Use build cache")
	return cmd
}

func newScheduledDeploysCreateCmd() *cobra.Command {
	var (
		server        string
		frequency     string
		at            string
		startRevision string
		endRevision   string
		copyConfig    bool
		runBuild      bool
		useCache      bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a scheduled deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if server == "" {
				return &output.UserError{Message: "Server is required", Hint: "Use --server flag"}
			}
			if frequency == "" {
				return &output.UserError{Message: "Frequency is required", Hint: "Use --frequency flag"}
			}
			if at == "" {
				return &output.UserError{Message: "At is required", Hint: "Use --at flag"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			s, err := client.CreateScheduledDeployment(cliCtx.Background(), projectID, sdk.ScheduledDeploymentCreateRequest{
				ServerIdentifier: server,
				StartRevision:    startRevision,
				EndRevision:      endRevision,
				Frequency:        frequency,
				At:               at,
				CopyConfigFiles:  copyConfig,
				RunBuildCommands: runBuild,
				UseBuildCache:    useCache,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(s, fmt.Sprintf("Created scheduled deployment: %s", s.Identifier)))
			}
			env.Status("Created scheduled deployment: %s (%s at %s)", s.Identifier, s.Frequency, s.At)
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "Server identifier (required)")
	cmd.Flags().StringVar(&frequency, "frequency", "", "Deployment frequency, e.g. daily, weekly (required)")
	cmd.Flags().StringVar(&at, "at", "", "Time to deploy, e.g. 14:30 (required)")
	cmd.Flags().StringVar(&startRevision, "start-revision", "", "Start revision")
	cmd.Flags().StringVar(&endRevision, "end-revision", "", "End revision")
	cmd.Flags().BoolVar(&copyConfig, "copy-config", false, "Copy config files")
	cmd.Flags().BoolVar(&runBuild, "run-build", false, "Run build commands")
	cmd.Flags().BoolVar(&useCache, "use-cache", false, "Use build cache")
	return cmd
}
