package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	feedbackURL  = "https://changelog.deployhq.com"
	supportEmail = "support@deployhq.com"
)

func newFeedbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "feedback",
		Short: "Send feedback or request a feature",
		Long: fmt.Sprintf(`Open the DeployHQ feedback board to submit ideas, vote on features, or report issues.

Feedback board: %s
Email support:  %s`, feedbackURL, supportEmail),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			env.Status("Opening feedback board...")
			env.Status("")
			env.Status("  Feedback & features: %s", feedbackURL)
			env.Status("  Email support:       %s", supportEmail)

			if err := openBrowser(feedbackURL); err != nil {
				env.Status("\nCouldn't open browser automatically. Visit the URL above.")
			}
			return nil
		},
	}
}
