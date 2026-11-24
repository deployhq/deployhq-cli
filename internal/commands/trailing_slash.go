package commands

import (
	"context"
	"errors"
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

// effectiveTrailingSlash is the mode a site's reported setting gives: a site
// that never chose one reports null and behaves as strip. The API leaves the
// setting null when strip is requested for such a site.
func effectiveTrailingSlash(reported *string) string {
	if reported == nil || *reported == "" {
		return "strip"
	}
	return *reported
}

// onlyTrailingSlash reports whether an update request sends nothing but the
// Static Hosting site settings.
func onlyTrailingSlash(req sdk.ServerUpdateRequest) bool {
	if req.HostedWebsite == nil {
		return false
	}
	req.HostedWebsite = nil
	return req == sdk.ServerUpdateRequest{}
}

// explainTrailingSlashOnlyError explains the refusal of an update that sent
// only --trailing-slash: the API accepts that for a Static Hosting server, and
// any other server type answers 400 because nothing was sent for the server
// itself. Other errors are returned unchanged.
func explainTrailingSlashOnlyError(err error, onlyTrailingSlash bool, showCmd string) error {
	var apiErr *sdk.APIError
	if !onlyTrailingSlash || !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		return err
	}
	return &output.UserError{
		Message:  "--trailing-slash only applies to Static Hosting servers, and nothing else was given to update",
		Hint:     fmt.Sprintf("Check the server's type with: %s (the API said: %s)", showCmd, apiErr.Message),
		Recovery: []output.Breadcrumb{{Action: "show", Cmd: showCmd + " --json"}},
	}
}

// explainTrailingSlashNotChanged explains an update the API saved except for
// the trailing slash: it answers 409 (the site is busy publishing), 429 (the
// hourly limit on redirect changes) or 503 (it could not be published), with a
// message that says the server was saved. Each is worth retrying; retryCmd
// sets only the mode. Other errors are returned unchanged.
func explainTrailingSlashNotChanged(err error, requested bool, retryCmd string) error {
	var apiErr *sdk.APIError
	if !requested || !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.StatusCode {
	case 409, 429, 503:
	default:
		return err
	}
	msg := apiErr.Message
	if msg == "" {
		msg = "The server was saved, but the trailing slash setting was not changed"
	}
	hint := "Only the trailing slash was not changed. Set it again in a moment with: " + retryCmd
	if apiErr.RetryAfter > 0 {
		hint = fmt.Sprintf("Only the trailing slash was not changed. Wait up to %d seconds, then set it with: %s", apiErr.RetryAfter, retryCmd)
	}
	return &output.UserError{
		Message:    msg,
		Hint:       hint,
		Retryable:  true,
		RetryAfter: apiErr.RetryAfter,
		Recovery:   []output.Breadcrumb{{Action: "set_trailing_slash", Cmd: retryCmd}},
	}
}

// warnIfTrailingSlashNotApplied says when a server update did not take the
// requested trailing-slash mode: the server is not a Static Hosting site, or
// it reports a different mode. A response without the site block says nothing
// either way, so it is not warned about. showCmd is the command that shows the
// server.
func warnIfTrailingSlashNotApplied(env *output.Envelope, requested string, server *sdk.Server, showCmd string) {
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
	if effectiveTrailingSlash(server.StaticHosting.TrailingSlash) != requested {
		env.Warn("The server did not confirm --trailing-slash %s. Check the site's settings with: %s",
			requested, showCmd)
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
	if cfg.trailingSlash == "" || server == nil {
		return
	}
	// A re-run reuses the server saved by an earlier launch, whatever its
	// type; --static does not change it.
	if server.ProtocolType != detect.ProtocolStaticHosting {
		env.Warn("--trailing-slash %s was not applied: the saved server %s is not a Static Hosting server.",
			cfg.trailingSlash, server.Identifier)
		return
	}
	if server.StaticHosting != nil && effectiveTrailingSlash(server.StaticHosting.TrailingSlash) == cfg.trailingSlash {
		return
	}
	setCmd := fmt.Sprintf("dhq servers update %s -p %s --trailing-slash %s", server.Identifier, cfg.projectID, cfg.trailingSlash)
	updated, err := client.UpdateServer(ctx, cfg.projectID, server.Identifier, sdk.ServerUpdateRequest{
		HostedWebsite: &sdk.HostedWebsiteUpdate{TrailingSlash: cfg.trailingSlash},
	})
	if err != nil {
		var apiErr *sdk.APIError
		switch {
		case errors.As(err, &apiErr) && apiErr.RetryAfter > 0 && apiErr.Message != "":
			env.Warn("--trailing-slash %s was not applied: %s Wait up to %d seconds, then set it with: %s",
				cfg.trailingSlash, apiErr.Message, apiErr.RetryAfter, setCmd)
		case errors.As(err, &apiErr) && apiErr.Message != "":
			env.Warn("--trailing-slash %s was not applied: %s Set it later with: %s", cfg.trailingSlash, apiErr.Message, setCmd)
		default:
			env.Warn("Could not set --trailing-slash %s on the existing site: %v. Set it later with: %s", cfg.trailingSlash, err, setCmd)
		}
		return
	}
	if updated.StaticHosting != nil {
		server.StaticHosting = updated.StaticHosting
		// As warnIfTrailingSlashNotApplied: a response without the site block
		// says nothing either way, but a different mode is not a success.
		if effectiveTrailingSlash(updated.StaticHosting.TrailingSlash) != cfg.trailingSlash {
			env.Warn("The server did not confirm --trailing-slash %s. Set it later with: %s", cfg.trailingSlash, setCmd)
			return
		}
	}
	env.Status("Trailing slash set to %s.", cfg.trailingSlash)
}
