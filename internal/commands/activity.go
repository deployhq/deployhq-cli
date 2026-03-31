package commands

import (
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newActivityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Show account activity and deploy stats",
		Long:  "Display recent deployment events and stats (deployments/week, success rate, avg duration, active servers).",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List recent activity events",
			RunE: func(cmd *cobra.Command, args []string) error {
				return &output.UserError{
					Message: "This command is not yet available",
					Hint:    "Account activity API (GET /activity) is coming soon. Use 'dhq deployments list -p <project>' in the meantime.",
				}
			},
		},
		&cobra.Command{
			Use: "stats", Short: "Show deploy stats (deployments/week, success rate, avg duration)",
			RunE: func(cmd *cobra.Command, args []string) error {
				return &output.UserError{
					Message: "This command is not yet available",
					Hint:    "Account activity API (GET /activity?include=stats) is coming soon.",
				}
			},
		},
	)
	return cmd
}
