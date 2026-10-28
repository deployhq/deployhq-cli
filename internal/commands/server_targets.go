package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

// Several per-project resources (excluded files, config files, SSH commands,
// integrations) can apply to every server in the project or only to some of
// them. DeployHQ treats a create request that names neither "all servers" nor
// a server list as "no servers", which silently produces a resource that never
// takes effect. These helpers make "all servers" the explicit default on create
// and let callers target specific servers or server groups by name or
// identifier.

const serverFlagUsage = "Apply only to this server or server group (name or identifier; repeatable or comma-separated). Default on create: all servers"

// addServerFlag registers the repeatable --server flag.
func addServerFlag(cmd *cobra.Command, servers *[]string) {
	cmd.Flags().StringSliceVar(servers, "server", nil, serverFlagUsage)
}

// addAllServersFlag registers --all-servers on update commands, where omitting
// both flags must leave the existing targeting untouched.
func addAllServersFlag(cmd *cobra.Command, all *bool) {
	cmd.Flags().BoolVar(all, "all-servers", false, "Apply to all servers in the project, including servers added later")
	cmd.MarkFlagsMutuallyExclusive("server", "all-servers")
}

// resolveServerTargets turns --server values into server or server-group
// identifiers. Names are matched against the project's servers first and then
// its server groups; only a value that matches no name is taken as an
// identifier, and only one the listings show, since a long name with a dash can
// look like one too. Both listings are complete, so a value missing from both is
// not in the project and fails: the API would drop it and save a resource that
// targets no server. If a listing failed, a canonical UUID is looked up by
// identifier in place of each failed listing: it is used if found and rejected
// if every lookup is a 404. Any other value, or a lookup that fails for another
// reason, reports the listing error, since the failed listing might have
// resolved it. The listings are fetched once, on the first value. It never
// prompts: an unknown or ambiguous value fails with the available choices so
// agents can retry.
func resolveServerTargets(ctx context.Context, client *sdk.Client, projectID string, inputs []string) ([]string, error) {
	var (
		ids        []string
		servers    []sdk.Server
		groups     []sdk.ServerGroup
		serversErr error
		groupsErr  error
		loaded     bool
	)
	for _, raw := range inputs {
		in := strings.TrimSpace(raw)
		if in == "" {
			continue
		}
		if !loaded {
			// A failed listing must not block values the other listing (or the
			// identifier fallback) resolves, so errors are only reported once a
			// value resolves no other way.
			servers, serversErr = client.ListServers(ctx, projectID, nil)
			groups, groupsErr = client.ListServerGroups(ctx, projectID, nil)
			loaded = true
		}
		id, candidates := resolveServerName(in, servers)
		if id != "" {
			if dups := sameNamedServers(in, servers); len(dups) > 1 {
				return nil, ambiguousServerError(in, dups, nil)
			}
			ids = append(ids, id)
			continue
		}
		if groupID, _ := resolveGroupName(in, groups); groupID != "" {
			if dups := sameNamedGroups(in, groups); len(dups) > 1 {
				return nil, ambiguousServerError(in, nil, dups)
			}
			ids = append(ids, groupID)
			continue
		}
		if isListedIdentifier(in, servers, groups) {
			ids = append(ids, in)
			continue
		}
		// With both listings in hand, an unlisted value is not in the project.
		// If either failed, the value may be a name or identifier the failed
		// listing would have shown, so only a canonical UUID is looked up then.
		listingFailed := serversErr != nil || groupsErr != nil
		if listingFailed && isCanonicalUUID(in) {
			found, notFound := lookUpIdentifier(ctx, client, projectID, in, serversErr, groupsErr)
			if found {
				ids = append(ids, in)
				continue
			}
			if notFound {
				return nil, unresolvedServerError(in, nil, nil, false)
			}
		}
		switch {
		case serversErr != nil:
			return nil, fmt.Errorf("--server %q: listing servers failed: %w", in, serversErr)
		case groupsErr != nil:
			return nil, fmt.Errorf("--server %q matches no server, and listing server groups failed: %w", in, groupsErr)
		default:
			return nil, unresolvedServerError(in, candidates, groups, true)
		}
	}
	if len(ids) == 0 {
		return nil, &output.UserError{
			Message: "--server needs at least one server name or identifier",
			Hint:    "To target every server, omit --server when creating, or use --all-servers when updating.",
		}
	}
	return ids, nil
}

// lookUpIdentifier asks the project for id directly, as a server if the server
// listing failed and as a server group if the group listing failed (the
// listing that succeeded has already shown it is not there). found means the
// project has it; notFound means every lookup was a 404, so it does not. Both
// false means a lookup failed for another reason and the answer is unknown.
func lookUpIdentifier(ctx context.Context, client *sdk.Client, projectID, id string, serversErr, groupsErr error) (found, notFound bool) {
	notFound = true
	if serversErr != nil {
		_, err := client.GetServer(ctx, projectID, id)
		if err == nil {
			return true, false
		}
		notFound = sdk.IsNotFound(err)
	}
	if groupsErr != nil {
		_, err := client.GetServerGroup(ctx, projectID, id)
		if err == nil {
			return true, false
		}
		notFound = notFound && sdk.IsNotFound(err)
	}
	return false, notFound
}

// isCanonicalUUID reports whether s is a UUID in the canonical 8-4-4-4-12
// hexadecimal form (either case). It is stricter than isUUID, which also
// accepts long dashed names.
func isCanonicalUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
			if !isHex {
				return false
			}
		}
	}
	return true
}

// isListedIdentifier reports whether input is exactly the identifier of one of
// the listed servers or server groups.
func isListedIdentifier(input string, servers []sdk.Server, groups []sdk.ServerGroup) bool {
	for _, s := range servers {
		if s.Identifier == input {
			return true
		}
	}
	for _, g := range groups {
		if g.Identifier == input {
			return true
		}
	}
	return false
}

// unresolvedServerError rejects input with the available choices. listed says
// whether candidates and groups came from complete listings; if not, the
// choices are unknown and the hint points at the list commands instead.
func unresolvedServerError(input string, candidates []sdk.Server, groups []sdk.ServerGroup, listed bool) error {
	if !listed {
		return &output.UserError{
			Message: fmt.Sprintf("No unique server or server group matches %q", input),
			Hint:    "Check the identifier with: dhq servers list, or dhq server-groups list",
		}
	}
	var names []string
	for _, s := range candidates {
		names = append(names, fmt.Sprintf("%s (%s)", s.Name, s.Identifier))
	}
	for _, g := range groups {
		names = append(names, fmt.Sprintf("%s (%s, group)", g.Name, g.Identifier))
	}
	hint := "No servers in this project — create one first with: dhq servers create"
	if len(names) > 0 {
		hint = "Available: " + strings.Join(names, ", ")
	}
	return &output.UserError{
		Message: fmt.Sprintf("No unique server or server group matches %q", input),
		Hint:    hint,
	}
}

// sameNamedServers returns every server whose name equals input under the
// case-insensitive comparison resolveServerName uses for an exact match or,
// when nothing matches exactly, under its punctuation-insensitive comparison.
// That helper returns the first exact match, and when several match ignoring
// punctuation it falls through to a looser "contains" match, so more than one
// here means the name is ambiguous and the caller must pick by identifier.
func sameNamedServers(input string, servers []sdk.Server) []sdk.Server {
	var exact, normalized []sdk.Server
	for _, s := range servers {
		if strings.EqualFold(s.Name, input) {
			exact = append(exact, s)
		}
		if normalize(s.Name) == normalize(input) {
			normalized = append(normalized, s)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return normalized
}

// sameNamedGroups is sameNamedServers for server groups (resolveGroupName).
func sameNamedGroups(input string, groups []sdk.ServerGroup) []sdk.ServerGroup {
	var exact, normalized []sdk.ServerGroup
	for _, g := range groups {
		if strings.EqualFold(g.Name, input) {
			exact = append(exact, g)
		}
		if normalize(g.Name) == normalize(input) {
			normalized = append(normalized, g)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return normalized
}

func ambiguousServerError(input string, servers []sdk.Server, groups []sdk.ServerGroup) error {
	kind := "server"
	var names []string
	for _, s := range servers {
		names = append(names, fmt.Sprintf("%s (%s)", s.Name, s.Identifier))
	}
	if len(groups) > 0 {
		kind = "server group"
	}
	for _, g := range groups {
		names = append(names, fmt.Sprintf("%s (%s, group)", g.Name, g.Identifier))
	}
	return &output.UserError{
		Message: fmt.Sprintf("--server %q matches more than one %s", input, kind),
		Hint:    "Use the identifier instead. Matches: " + strings.Join(names, ", "),
	}
}

// describeServerTargets is the human-readable target for status lines.
func describeServerTargets(ids []string) string {
	if len(ids) == 0 {
		return "all servers"
	}
	return "servers: " + strings.Join(ids, ", ")
}
