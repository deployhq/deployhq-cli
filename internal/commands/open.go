package commands

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [project]",
		Short: "Open DeployHQ in the browser",
		Long:  "Open the DeployHQ dashboard in your default browser. Optionally specify a project to open directly.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			base := fmt.Sprintf("https://%s.deployhq.com", client.Account())

			url := base
			if len(args) > 0 {
				url = fmt.Sprintf("%s/projects/%s", base, args[0])
			} else if cliCtx.Config.Project != "" {
				url = fmt.Sprintf("%s/projects/%s", base, cliCtx.Config.Project)
			}

			env := cliCtx.Envelope
			if err := openBrowser(url); err != nil {
				env.Status("Open in browser: %s", url)
				return nil
			}
			env.Status("Opened %s", url)
			return nil
		},
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return &output.UserError{Message: fmt.Sprintf("unsupported platform: %s", runtime.GOOS)}
	}
	return cmd.Run()
}
