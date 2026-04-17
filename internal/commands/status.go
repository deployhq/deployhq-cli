package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Quick dashboard across all projects",
		Long:  "Show deploy stats and recent activity across all your projects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			result, err := client.ListActivityWithStats(cliCtx.Background(), nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(result, "Account status",
					output.Breadcrumb{Action: "activity", Cmd: "dhq activity list"},
					output.Breadcrumb{Action: "projects", Cmd: "dhq projects list"},
				))
			}
			s := result.Stats
			env.Status("Deploy Stats (this week)")
			env.WriteTable([]string{"Metric", "Value"}, [][]string{
				{"Deployments", fmt.Sprintf("%d (%+d vs last week)", s.DeploymentsThisWeek, s.DeploymentsDelta)},
				{"Success rate", fmt.Sprintf("%.0f%% (%+.1f%%)", s.SuccessRate, s.SuccessRateDelta)},
				{"Avg duration", fmt.Sprintf("%ds", s.AvgDurationSeconds)},
				{"Active servers", fmt.Sprintf("%d", s.ActiveServers)},
			})
			if len(result.Events) > 0 {
				env.Status("\nRecent Activity")
				limit := len(result.Events)
				if limit > 5 {
					limit = 5
				}
				rows := make([][]string, limit)
				for i := 0; i < limit; i++ {
					e := result.Events[i]
					rows[i] = []string{
						formatEventTime(e.CreatedAt),
						formatEventType(e.Event),
						e.Project.Name,
						formatEventDetail(e),
					}
				}
				env.WriteTable([]string{"Time", "Event", "Project", "Details"}, rows)
				if len(result.Events) > 5 {
					env.Status("  ... and %d more (dhq activity list)", len(result.Events)-5)
				}
			}
			return nil
		},
	}
}
