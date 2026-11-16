package commands

import (
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// trailingSlashFlagHelp describes the --trailing-slash flag.
const trailingSlashFlagHelp = "How the site treats a trailing slash (static_hosting): " +
	"strip (default; /about/ redirects to /about), " +
	"force (/about redirects to /about/; use when moving from WordPress or any site whose URLs end in a slash), " +
	"or preserve (both work, no redirect; add canonical tags)"

// parseTrailingSlash normalises a --trailing-slash value and refuses anything
// the API would not accept, before any request is made.
func parseTrailingSlash(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	for _, mode := range sdk.TrailingSlashModes {
		if value == mode {
			return value, nil
		}
	}
	return "", &output.UserError{
		Message: fmt.Sprintf("--trailing-slash %q is not a trailing-slash mode", raw),
		Hint: "Use strip (/about/ redirects to /about, the default), force (/about redirects to /about/, " +
			"for sites moved from WordPress) or preserve (both work, no redirect).",
	}
}

// warnIfTrailingSlashNotApplied says when a server update did not take the
// requested trailing-slash mode: the server is not a Static Hosting site, or
// it reports a different mode. A response without the site block says nothing
// either way, so it is not warned about.
func warnIfTrailingSlashNotApplied(env *output.Envelope, requested string, server *sdk.Server) {
	if requested == "" || server == nil {
		return
	}
	if server.ProtocolType != "" && server.ProtocolType != "static_hosting" {
		env.Warn("--trailing-slash was ignored: it only applies to Static Hosting servers, and %s is a %s server.",
			server.Name, server.ProtocolType)
		return
	}
	if server.StaticHosting == nil {
		return
	}
	if got := server.StaticHosting.TrailingSlash; got == nil || *got != requested {
		env.Warn("The server did not confirm --trailing-slash %s. Check the site's settings with: dhq servers show %s",
			requested, server.Identifier)
	}
}
