package commands

import (
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newBuildLanguagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-languages",
		Short: "Manage build language versions",
	}
	cmd.AddCommand(
		newBuildLanguagesSetCmd(),
	)
	return cmd
}

func newBuildLanguagesSetCmd() *cobra.Command {
	var version, buildConfig string
	cmd := &cobra.Command{
		Use:   "set <language-id>",
		Short: "Set the version for a build language",
		Long:  "Set the language version used in the build server (e.g. ruby, node, python).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == "" {
				return &output.UserError{
					Message: "--version is required",
					Hint:    "Use 'dhq language-versions list -p <project>' to see available versions",
				}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			var lang *sdk.BuildLanguage
			if buildConfig != "" {
				lang, err = client.UpdateBuildLanguageOverride(cliCtx.Background(), projectID, buildConfig, args[0], sdk.BuildLanguageUpdateRequest{
					Version: version,
				})
			} else {
				lang, err = client.UpdateBuildLanguage(cliCtx.Background(), projectID, args[0], sdk.BuildLanguageUpdateRequest{
					Version: version,
				})
			}
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(lang, lang.Name+" "+lang.Version))
			}
			env.Status("Set %s to version %s", lang.Name, lang.Version)
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Language version (required)")
	cmd.Flags().StringVar(&buildConfig, "build-config", "", "Build configuration override ID (optional)")
	return cmd
}
