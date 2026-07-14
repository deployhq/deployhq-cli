package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newManagedHostingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "managed-hosting",
		Aliases: []string{"managed-vps"},
		Short:   "Browse Managed VPS regions and sizes",
		Long: `Read-only catalogue of the regions and droplet sizes available for Managed VPS provisioning. Requires beta_features on the account.

Use these slugs with "dhq launch" or "dhq servers create --region <slug> --size <slug>".`,
	}

	cmd.AddCommand(
		newManagedHostingRegionsCmd(),
		newManagedHostingSizesCmd(),
	)

	return cmd
}

func newManagedHostingRegionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "regions",
		Short: "List Managed VPS regions",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			rows, err := client.ListManagedHostingRegionRows(cliCtx.Background())
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(rows,
					fmt.Sprintf("%d regions", len(rows)),
				))
			}

			if env.QuietMode {
				slugs := make([]string, len(rows))
				for i, r := range rows {
					slugs[i] = r.Slug
				}
				env.WriteQuiet(slugs)
				return nil
			}

			columns := []string{"Group", "Slug", "Name", "Country", "Default?"}
			tableRows := make([][]string, len(rows))
			for i, r := range rows {
				def := ""
				if r.IsDefault {
					def = "yes"
				}
				tableRows[i] = []string{r.Group, r.Slug, r.Name, r.Country, def}
			}
			env.WriteTable(columns, tableRows)
			return nil
		},
	}
}

func newManagedHostingSizesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sizes",
		Short: "List Managed VPS sizes",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			sizes, err := client.ListManagedHostingSizes(cliCtx.Background())
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(sizes,
					fmt.Sprintf("%d sizes", len(sizes)),
				))
			}

			if env.QuietMode {
				slugs := make([]string, len(sizes))
				for i, s := range sizes {
					slugs[i] = s.Slug
				}
				env.WriteQuiet(slugs)
				return nil
			}

			columns := []string{"Slug", "Label", "Monthly Cost", "Currency"}
			tableRows := make([][]string, len(sizes))
			for i, s := range sizes {
				tableRows[i] = []string{
					s.Slug,
					s.Label,
					fmt.Sprintf("%.2f", s.MonthlyCost),
					s.Currency,
				}
			}
			env.WriteTable(columns, tableRows)
			return nil
		},
	}
}
