package commands

import (
	"strings"

	"github.com/deployhq/deployhq-cli/internal/detect"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newDetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect [path]",
		Short: "Detect a project's framework and suggested deploy configuration",
		Long: `Scan a project directory and ask DeployHQ to detect its framework, the
suggested deploy protocol, and a build pipeline — the same StackDetector the web
onboarding wizard uses, so the CLI's recommendation matches the server.

The scan uploads the directory's filename listing plus the contents of a bounded
set of manifest files (package.json, Gemfile, requirements.txt, go.mod, and the
like). .git and node_modules are skipped. Defaults to the current directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			filenames, files := detect.CollectManifest(dir)
			resp, err := client.DetectFramework(cliCtx.Background(), sdk.DetectionPayload{
				Filenames: filenames,
				Files:     files,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				summary := resp.Stack
				if summary == "" {
					summary = "no framework detected"
				}
				return env.WriteJSON(output.NewResponse(resp, summary))
			}

			stack := resp.Stack
			if stack == "" {
				stack = "(none detected)"
			}
			rows := [][]string{{"Stack", stack}}
			if resp.Version != "" {
				rows = append(rows, []string{"Version", resp.Version})
			}
			if resp.SuggestedProtocol != "" {
				rows = append(rows, []string{"Suggested protocol", resp.SuggestedProtocol})
			}
			if resp.StaticHosting.Eligibility != "" {
				rows = append(rows, []string{"Static hosting", resp.StaticHosting.Eligibility})
			}
			if resp.Description != "" {
				rows = append(rows, []string{"Description", resp.Description})
			}
			env.WriteTable([]string{"Field", "Value"}, rows)

			if len(resp.BuildCommands) > 0 {
				cmds := make([]string, 0, len(resp.BuildCommands))
				for _, bc := range resp.BuildCommands {
					if bc.Command != "" {
						cmds = append(cmds, bc.Command)
					}
				}
				if len(cmds) > 0 {
					env.Status("\nBuild commands:\n  %s", strings.Join(cmds, "\n  "))
				}
			}
			if resp.AIAssisted {
				env.Status("\n(AI-assisted detection)")
			}
			env.Status("\nTip: dhq launch to deploy this project, or dhq detect --json for the full result")
			return nil
		},
	}
	return cmd
}
