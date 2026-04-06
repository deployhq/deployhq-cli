package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newActivityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Show account activity and deploy stats",
		Long:  "Display recent deployment events and stats (deployments/week, success rate, avg duration, active servers).",
	}
	cmd.AddCommand(
		newActivityListCmd(),
		newActivityStatsCmd(),
	)
	return cmd
}

func newActivityListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recent activity events",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			events, err := client.ListActivity(cliCtx.Background())
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(events, fmt.Sprintf("%d events", len(events)),
					output.Breadcrumb{Action: "stats", Cmd: "dhq activity stats"},
				))
			}
			if len(events) == 0 {
				env.Status("No recent activity")
				return nil
			}
			rows := make([][]string, len(events))
			for i, e := range events {
				rows[i] = []string{
					formatEventTime(e.CreatedAt),
					formatEventType(e.Event),
					e.Project.Name,
					formatEventDetail(e),
					e.User,
				}
			}
			env.WriteTable([]string{"Time", "Event", "Project", "Details", "User"}, rows)
			return nil
		},
	}
}

func newActivityStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show deploy stats (deployments/week, success rate, avg duration)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			result, err := client.ListActivityWithStats(cliCtx.Background())
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(result, "Account activity with stats",
					output.Breadcrumb{Action: "events", Cmd: "dhq activity list"},
				))
			}
			s := result.Stats
			env.WriteTable([]string{"Metric", "Value"}, [][]string{
				{"Deployments this week", fmt.Sprintf("%d (%+d)", s.DeploymentsThisWeek, s.DeploymentsDelta)},
				{"Success rate", fmt.Sprintf("%.0f%% (%+.1f%%)", s.SuccessRate, s.SuccessRateDelta)},
				{"Avg duration", fmt.Sprintf("%ds", s.AvgDurationSeconds)},
				{"Active servers", fmt.Sprintf("%d", s.ActiveServers)},
			})
			if len(result.Events) > 0 {
				env.Status("")
				rows := make([][]string, len(result.Events))
				for i, e := range result.Events {
					rows[i] = []string{
						formatEventTime(e.CreatedAt),
						formatEventType(e.Event),
						e.Project.Name,
						formatEventDetail(e),
					}
				}
				env.WriteTable([]string{"Time", "Event", "Project", "Details"}, rows)
			}
			return nil
		},
	}
}

func formatEventTime(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Local().Format("Jan 02 15:04")
}

func formatEventType(event string) string {
	switch event {
	case "deploy_completed":
		return "deployed"
	case "deploy_failed":
		return "failed"
	case "deploy_started":
		return "started"
	case "deploy_aborted":
		return "aborted"
	case "project_created":
		return "project created"
	case "server_created":
		return "server created"
	default:
		return strings.ReplaceAll(event, "_", " ")
	}
}

func formatEventDetail(e sdk.ActivityEvent) string {
	props := e.Properties
	parts := []string{}
	if s, ok := props["servers"].(string); ok && s != "" {
		parts = append(parts, s)
	}
	if ref, ok := props["end_ref"].(string); ok && ref != "" {
		if len(ref) > 8 {
			ref = ref[:8]
		}
		parts = append(parts, ref)
	}
	if name, ok := props["name"].(string); ok && name != "" {
		parts = append(parts, name)
	}
	return strings.Join(parts, " ")
}
