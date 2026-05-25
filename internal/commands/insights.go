package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newInsightsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "insights [project]",
		Short: "Show deployment insights (shortcut for projects insights)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := resolveProjectArg(args)
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			insights, err := client.GetProjectInsights(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			return renderInsights(insights, projectID)
		},
	}
}

func renderInsights(insights map[string]interface{}, projectID string) error {
	env := cliCtx.Envelope
	if env.WantsJSON() {
		return env.WriteJSON(output.NewResponse(insights,
			fmt.Sprintf("Insights for project: %s", projectID),
		))
	}

	data, ok := insights["data"]
	if !ok {
		data = insights
	}
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return env.WriteJSON(insights)
	}

	if period, ok := dataMap["period"]; ok {
		env.Status(fmt.Sprintf("Insights for project: %s (last %v days)", projectID, period))
	}

	servers, ok := dataMap["servers"].([]interface{})
	if !ok || len(servers) == 0 {
		env.Status("No server data available")
		return nil
	}

	rows := make([][]string, 0, len(servers))
	for _, s := range servers {
		srv, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		name := fmt.Sprintf("%v", srv["name"])
		deploysPerWeek := fmt.Sprintf("%.1f", toFloat64(srv["deploys_per_week"]))
		latestStatus := fmt.Sprintf("%v", srv["latest_status"])
		avgDuration := avgDeployDuration(srv["recent_deploys"])
		rows = append(rows, []string{name, deploysPerWeek, latestStatus, avgDuration})
	}

	env.WriteTable([]string{"Server", "Deploys/Week", "Latest", "Avg Duration"}, rows)
	return nil
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}

func avgDeployDuration(v interface{}) string {
	deploys, ok := v.([]interface{})
	if !ok || len(deploys) == 0 {
		return "-"
	}
	var total float64
	var count int
	for _, d := range deploys {
		dm, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		dur := toFloat64(dm["duration"])
		if dur > 0 {
			total += dur
			count++
		}
	}
	if count == 0 {
		return "-"
	}
	avg := total / float64(count)
	if avg >= 60 {
		return fmt.Sprintf("%.0fm %02.0fs", float64(int(avg)/60), float64(int(avg)%60))
	}
	return fmt.Sprintf("%.0fs", avg)
}
