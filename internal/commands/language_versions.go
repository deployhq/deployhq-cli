package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newLanguageVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "language-versions",
		Aliases: []string{"lv"},
		Short:   "Manage available language versions",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List available language versions for the build server",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				versions, err := client.ListLanguageVersions(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(versions, fmt.Sprintf("%d languages available", len(versions))))
				}

				// Sort language names for stable output.
				langs := make([]string, 0, len(versions))
				for lang := range versions {
					langs = append(langs, lang)
				}
				sort.Strings(langs)

				rows := make([][]string, len(langs))
				for i, lang := range langs {
					rows[i] = []string{lang, strings.Join(versions[lang], ", ")}
				}
				env.WriteTable([]string{"Language", "Versions"}, rows)
				env.Status("Tip: dhq build-configs create -p <project> --packages '{\"node\":\"20\"}'")
				return nil
			},
		},
	)
	return cmd
}
