package commands

import (
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newActivityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activity",
		Short: "Show account activity and deploy stats",
		Long:  "Display recent deployment activity, success rate, average duration, and active servers across your account.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return &output.UserError{
				Message: "This command is not yet available",
				Hint:    "Account activity API is coming soon. Use 'dhq deployments list -p <project>' in the meantime.",
			}
		},
	}
}
