package commands

import (
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Quick dashboard across all projects",
		Long:  "Show recent deployments, running deployments, and overall health across all your projects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return &output.UserError{
				Message: "This command is not yet available",
				Hint:    "Account status API is coming soon. Use 'dhq projects list' and 'dhq deployments list -p <project>' in the meantime.",
			}
		},
	}
}
