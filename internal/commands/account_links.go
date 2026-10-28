package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// accountPageURL links to a page of the DeployHQ web app, which lives on the
// account's own host (for example https://acme.deployhq.com/account/packages).
// It prefers the host the API error came from, then the configured host
// (--host or DEPLOYHQ_HOST), and otherwise builds the default host from the
// account subdomain. apiErr may be nil.
func accountPageURL(account string, apiErr *sdk.APIError, path string) string {
	if apiErr != nil && apiErr.AccountURL != "" {
		return apiErr.AccountURL + path
	}
	if cliCtx != nil && cliCtx.Config != nil {
		if base := cliCtx.Config.BaseURL(account); base != "" {
			return base + path
		}
	}
	return fmt.Sprintf("https://%s.deployhq.com%s", account, path)
}
