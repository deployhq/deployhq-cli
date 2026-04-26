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

			return cliCtx.Envelope.WriteJSON(output.NewResponse(insights,
				fmt.Sprintf("Insights for project: %s", projectID),
			))
		},
	}
}
