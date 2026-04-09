package commands

import (
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func addPaginationFlags(cmd *cobra.Command, page *int, perPage *int) {
	cmd.Flags().IntVar(page, "page", 0, "Page number (0 = API default)")
	cmd.Flags().IntVar(perPage, "per-page", 0, "Results per page (0 = API default)")
}

func listOptsFromFlags(page, perPage int) *sdk.ListOptions {
	if page == 0 && perPage == 0 {
		return nil
	}
	return &sdk.ListOptions{Page: page, PerPage: perPage}
}
