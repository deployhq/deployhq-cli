package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/detect"
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

// validateLaunchTrailingSlash checks --trailing-slash before any request: the
// value, and that the target (when already known) is Static Hosting.
func validateLaunchTrailingSlash(cfg *launchConfig) error {
	if cfg.trailingSlash == "" {
		return nil
	}
	mode, err := parseTrailingSlash(cfg.trailingSlash)
	if err != nil {
		return err
	}
	cfg.trailingSlash = mode
	return refuseTrailingSlashTarget(cfg.targetProtocol)
}

// resolveLaunchTrailingSlashTarget makes --trailing-slash choose Static Hosting
// when nothing else chose a target, and refuses it for any other target.
func resolveLaunchTrailingSlashTarget(cfg *launchConfig) error {
	if cfg.trailingSlash == "" {
		return nil
	}
	if cfg.targetProtocol == "" {
		cfg.targetProtocol = detect.ProtocolStaticHosting
		return nil
	}
	return refuseTrailingSlashTarget(cfg.targetProtocol)
}

func refuseTrailingSlashTarget(target string) error {
	if target == "" || target == detect.ProtocolStaticHosting {
		return nil
	}
	return &output.UserError{
		Message: "--trailing-slash only applies to Static Hosting, not " + hostingName(target),
		Hint:    "Launch without --trailing-slash, or choose Static Hosting with --static.",
	}
}

// launchEnsureTrailingSlashOnServer applies --trailing-slash to the site a
// re-run found, when it has a different mode. A failure is a warning: the
// site still deploys, and the warning names the command that sets it.
func launchEnsureTrailingSlashOnServer(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, server *sdk.Server) {
	if cfg.trailingSlash == "" || server == nil || server.ProtocolType != detect.ProtocolStaticHosting {
		return
	}
	if server.StaticHosting != nil && server.StaticHosting.TrailingSlash != nil && *server.StaticHosting.TrailingSlash == cfg.trailingSlash {
		return
	}
	setCmd := fmt.Sprintf("dhq servers update %s -p %s --trailing-slash %s", server.Identifier, cfg.projectID, cfg.trailingSlash)
	updated, err := client.UpdateServer(ctx, cfg.projectID, server.Identifier, sdk.ServerUpdateRequest{
		HostedWebsite: &sdk.HostedWebsiteUpdate{TrailingSlash: cfg.trailingSlash},
	})
	if err != nil {
		env.Warn("Could not set --trailing-slash %s on the existing site: %v. Set it later with: %s", cfg.trailingSlash, err, setCmd)
		return
	}
	if updated.StaticHosting != nil {
		server.StaticHosting = updated.StaticHosting
	}
	env.Status("Trailing slash set to %s.", cfg.trailingSlash)
}
