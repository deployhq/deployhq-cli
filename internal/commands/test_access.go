package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newTestAccessCmd() *cobra.Command {
	var server string
	var wait bool
	var timeout int

	cmd := &cobra.Command{
		Use:   "test-access",
		Short: "Test repository and server connectivity",
		Long:  "Run a connectivity test for all servers (or a specific server) in a project.",
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

			// Resolve server name to identifier if provided
			if server != "" && !isUUID(server) {
				servers, err := client.ListServers(cliCtx.Background(), projectID, nil)
				if err == nil {
					resolved, _ := resolveServerName(server, servers)
					if resolved != "" {
						server = resolved
					}
				}
			}

			// Trigger test
			var run *sdk.TestAccessRun
			if server != "" {
				env.Status("Testing server connectivity...")
				run, err = client.RunServerTestAccess(cliCtx.Background(), projectID, server)
			} else {
				env.Status("Testing repository and server connectivity...")
				run, err = client.RunTestAccess(cliCtx.Background(), projectID)
			}
			if err != nil {
				return err
			}

			if !wait {
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(run,
						fmt.Sprintf("Test access %s started", run.Identifier),
						output.Breadcrumb{Action: "results", Cmd: fmt.Sprintf("dhq test-access show %s -p %s", run.Identifier, projectID)},
					))
				}
				env.Status("Test access %s started (status: %s)", run.Identifier, run.Status)
				env.Status("\nCheck results:")
				env.Status("  dhq test-access show %s -p %s", run.Identifier, projectID)
				return nil
			}

			// Poll until complete
			ctx := cliCtx.Background()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
				defer cancel()
			}

			run, err = pollTestAccess(ctx, client, env, projectID, run.Identifier)
			if ctx.Err() == context.DeadlineExceeded {
				return &output.UserError{
					Message: fmt.Sprintf("Timed out after %ds waiting for test access to complete", timeout),
					Hint:    fmt.Sprintf("dhq test-access show %s -p %s", run.Identifier, projectID),
				}
			}
			if err != nil {
				return err
			}

			// Render results
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(run, formatTestSummary(run)))
			}

			printTestResults(env, run)
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "Test a specific server (name or identifier)")
	cmd.Flags().BoolVarP(&wait, "wait", "w", true, "Wait for results (default: true)")
	cmd.Flags().IntVar(&timeout, "timeout", 120, "Timeout in seconds when waiting (default: 120)")

	cmd.AddCommand(newTestAccessShowCmd())

	return cmd
}

func newTestAccessShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show test access results",
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

			run, err := client.GetTestAccess(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(run, formatTestSummary(run)))
			}

			printTestResults(env, run)
			return nil
		},
	}
}

func pollTestAccess(ctx context.Context, client *sdk.Client, env *output.Envelope, projectID, runID string) (*sdk.TestAccessRun, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	env.Status("Waiting for results...")

	for {
		select {
		case <-ctx.Done():
			// Return the last known state
			run, _ := client.GetTestAccess(context.Background(), projectID, runID)
			if run != nil {
				return run, ctx.Err()
			}
			return &sdk.TestAccessRun{Identifier: runID}, ctx.Err()
		case <-ticker.C:
			run, err := client.GetTestAccess(ctx, projectID, runID)
			if err != nil {
				return nil, err
			}
			if run.Status != "pending" && run.Status != "running" {
				return run, nil
			}
		}
	}
}

func printTestResults(env *output.Envelope, run *sdk.TestAccessRun) {
	env.Status("Test access %s — %s", run.Identifier, output.ColorStatus(run.Status))
	env.Status("")

	if run.Results == nil {
		env.Status("No results yet (status: %s)", run.Status)
		return
	}

	// Repository
	if run.Results.Repository != nil {
		r := run.Results.Repository
		icon := statusIcon(r.Status)
		env.Status("Repository: %s %s", icon, r.Status)
		if r.Message != "" {
			env.Status("  %s", r.Message)
		}
	}

	// Servers
	if len(run.Results.Servers) > 0 {
		env.Status("")
		columns := []string{"Server", "Status", "Message"}
		var rows [][]string
		for _, s := range run.Results.Servers {
			rows = append(rows, []string{s.Name, fmt.Sprintf("%s %s", statusIcon(s.Status), s.Status), s.Message})
		}
		env.WriteTable(columns, rows)
	}
}

func formatTestSummary(run *sdk.TestAccessRun) string {
	if run.Results == nil {
		return fmt.Sprintf("Test access %s: %s", run.Identifier, run.Status)
	}

	passed, failed := 0, 0
	if run.Results.Repository != nil && strings.EqualFold(run.Results.Repository.Status, "ok") {
		passed++
	} else if run.Results.Repository != nil {
		failed++
	}
	for _, s := range run.Results.Servers {
		if strings.EqualFold(s.Status, "ok") {
			passed++
		} else {
			failed++
		}
	}

	if failed == 0 {
		return fmt.Sprintf("All %d checks passed", passed)
	}
	return fmt.Sprintf("%d passed, %d failed", passed, failed)
}

func statusIcon(status string) string {
	switch strings.ToLower(status) {
	case "ok", "success", "passed":
		return "✓"
	case "error", "failed", "failure":
		return "✗"
	default:
		return "?"
	}
}
