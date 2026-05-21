package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newDeploymentChecksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deployment-checks",
		Short: "Manage deployment checks",
		Long: `Deployment checks gate a deployment at one of two stages: pre_build (runs on the build server before the build) or post_deploy (runs after files have been uploaded).

Three check types are supported:
  ssh                 — runs a command over SSH on selected servers
  http                — sends an HTTP request from the deployment worker
  vulnerability_scan  — runs a security scanner (Snyk, Trivy, or a custom CLI emitting SARIF); pre_build only`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List deployment checks",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				checks, err := client.ListDeploymentChecks(cliCtx.Background(), projectID, nil)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(checks, fmt.Sprintf("%d deployment checks", len(checks))))
				}
				rows := make([][]string, len(checks))
				for i, c := range checks {
					rows[i] = []string{c.Identifier, c.Name, c.Stage, c.CheckType, enabledLabel(c.Enabled)}
				}
				env.WriteTable([]string{"Identifier", "Name", "Stage", "Type", "Enabled"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show deployment check details", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				c, err := client.GetDeploymentCheck(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(c, c.Name))
			},
		},
		newDeploymentChecksCreateCmd(),
		newDeploymentChecksUpdateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a deployment check", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteDeploymentCheck(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted deployment check: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

// checkFlags holds the shared flag set for create and update.
type checkFlags struct {
	name, description, stage, checkType, command            string
	servers                                                 []string
	httpMethod, httpURL, httpBodyMatch                      string
	httpExpectedStatus, timeoutSeconds                      int
	httpExpectedStatusSet, timeoutSecondsSet                bool
	scanner, scanTargetKind, scanTarget, severityThreshold  string
	sarifOutputPath                                         string
	enabled, failOnUnfixedOnly                              bool
	enabledSet, failOnUnfixedOnlySet                        bool
}

func (f *checkFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.name, "name", "", "Display name for the check")
	cmd.Flags().StringVar(&f.description, "description", "", "Description")
	cmd.Flags().StringVar(&f.stage, "stage", "", "Stage: pre_build or post_deploy")
	cmd.Flags().StringVar(&f.checkType, "check-type", "", "Check type: ssh, http, or vulnerability_scan")
	cmd.Flags().BoolVar(&f.enabled, "enabled", true, "Whether the check is enabled")
	cmd.Flags().IntVar(&f.timeoutSeconds, "timeout", 0, "Timeout in seconds")
	cmd.Flags().StringVar(&f.command, "command", "", "Command to run (ssh checks)")
	cmd.Flags().StringSliceVar(&f.servers, "servers", nil, "Server identifiers to target (ssh checks); repeat or comma-separate")
	cmd.Flags().StringVar(&f.httpMethod, "http-method", "", "HTTP method (http checks)")
	cmd.Flags().StringVar(&f.httpURL, "http-url", "", "URL to request (http checks)")
	cmd.Flags().IntVar(&f.httpExpectedStatus, "http-expected-status", 0, "Expected HTTP status code (http checks)")
	cmd.Flags().StringVar(&f.httpBodyMatch, "http-body-match", "", "Substring expected in HTTP response body (http checks)")
	cmd.Flags().StringVar(&f.scanner, "scanner", "", "Scanner: snyk, trivy, or custom (vulnerability_scan)")
	cmd.Flags().StringVar(&f.scanTargetKind, "scan-target-kind", "", "Scan target kind (vulnerability_scan)")
	cmd.Flags().StringVar(&f.scanTarget, "scan-target", "", "Scan target path or identifier (vulnerability_scan)")
	cmd.Flags().StringVar(&f.severityThreshold, "severity-threshold", "", "Minimum severity that fails the check (vulnerability_scan)")
	cmd.Flags().BoolVar(&f.failOnUnfixedOnly, "fail-on-unfixed-only", false, "Only fail on findings with no available fix (vulnerability_scan)")
	cmd.Flags().StringVar(&f.sarifOutputPath, "sarif-output-path", "", "Path where the scanner writes SARIF output (vulnerability_scan)")
}

// captureChanged inspects which flags the user actually set so omitted flags
// don't overwrite existing values on update.
func (f *checkFlags) captureChanged(cmd *cobra.Command) {
	f.enabledSet = cmd.Flags().Changed("enabled")
	f.timeoutSecondsSet = cmd.Flags().Changed("timeout")
	f.httpExpectedStatusSet = cmd.Flags().Changed("http-expected-status")
	f.failOnUnfixedOnlySet = cmd.Flags().Changed("fail-on-unfixed-only")
}

func (f *checkFlags) toRequest() sdk.DeploymentCheckCreateRequest {
	req := sdk.DeploymentCheckCreateRequest{
		Name:              f.name,
		Description:       f.description,
		Stage:             f.stage,
		CheckType:         f.checkType,
		Command:           f.command,
		Servers:           f.servers,
		HTTPMethod:        f.httpMethod,
		HTTPURL:           f.httpURL,
		HTTPBodyMatch:     f.httpBodyMatch,
		Scanner:           f.scanner,
		ScanTargetKind:    f.scanTargetKind,
		ScanTarget:        f.scanTarget,
		SeverityThreshold: f.severityThreshold,
		SARIFOutputPath:   f.sarifOutputPath,
	}
	if f.enabledSet {
		enabled := f.enabled
		req.Enabled = &enabled
	}
	if f.timeoutSecondsSet {
		t := f.timeoutSeconds
		req.TimeoutSeconds = &t
	}
	if f.httpExpectedStatusSet {
		s := f.httpExpectedStatus
		req.HTTPExpectedStatus = &s
	}
	if f.failOnUnfixedOnlySet {
		fou := f.failOnUnfixedOnly
		req.FailOnUnfixedOnly = &fou
	}
	return req
}

func newDeploymentChecksCreateCmd() *cobra.Command {
	f := &checkFlags{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a deployment check",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.name == "" {
				return &output.UserError{Message: "--name is required"}
			}
			if f.stage == "" {
				return &output.UserError{Message: "--stage is required (pre_build or post_deploy)"}
			}
			if f.checkType == "" {
				return &output.UserError{Message: "--check-type is required (ssh, http, or vulnerability_scan)"}
			}
			switch f.checkType {
			case "ssh":
				if f.command == "" {
					return &output.UserError{Message: "--command is required for ssh checks"}
				}
			case "http":
				if f.httpURL == "" {
					return &output.UserError{Message: "--http-url is required for http checks"}
				}
			case "vulnerability_scan":
				if f.stage != "pre_build" {
					return &output.UserError{Message: "vulnerability_scan checks must use --stage pre_build"}
				}
				if f.scanner == "" {
					return &output.UserError{Message: "--scanner is required for vulnerability_scan checks"}
				}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f.captureChanged(cmd)
			c, err := client.CreateDeploymentCheck(cliCtx.Background(), projectID, f.toRequest())
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Created deployment check: %s (%s)", c.Name, c.Identifier)
			return nil
		},
	}
	f.register(cmd)
	return cmd
}

func newDeploymentChecksUpdateCmd() *cobra.Command {
	f := &checkFlags{}
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a deployment check",
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
			f.captureChanged(cmd)
			c, err := client.UpdateDeploymentCheck(cliCtx.Background(), projectID, args[0], f.toRequest())
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated deployment check: %s", c.Identifier)
			return nil
		},
	}
	f.register(cmd)
	return cmd
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "yes"
	}
	return "no"
}
