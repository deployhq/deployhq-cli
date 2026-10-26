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
// identifier — one the listings show, or one that looks like an identifier
// (isUUID), since a long name with a dash can look like one too. The listings
// are fetched once, on the first value. It never prompts: an unknown or
// ambiguous name fails with the available choices so agents can retry.
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
		if isListedIdentifier(in, servers, groups) || isUUID(in) {
			ids = append(ids, in)
			continue
		}
		switch {
		case serversErr != nil:
			return nil, fmt.Errorf("--server %q: listing servers failed: %w", in, serversErr)
		case groupsErr != nil:
			return nil, fmt.Errorf("--server %q matches no server, and listing server groups failed: %w", in, groupsErr)
		default:
			return nil, unresolvedServerError(in, candidates, groups)
		}
	}
	if len(ids) == 0 {
		return nil, &output.UserError{
			Message: "--server needs at least one server name or identifier",
			Hint:    "Omit --server to apply to all servers",
		}
	}
	return ids, nil
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

func unresolvedServerError(input string, candidates []sdk.Server, groups []sdk.ServerGroup) error {
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
// case-insensitive comparison resolveServerName uses for an exact match. That
// helper returns the first such server, so more than one means the name is
// ambiguous and the caller must pick by identifier.
func sameNamedServers(input string, servers []sdk.Server) []sdk.Server {
	var matches []sdk.Server
	for _, s := range servers {
		if strings.EqualFold(s.Name, input) {
			matches = append(matches, s)
		}
	}
	return matches
}

// sameNamedGroups is sameNamedServers for server groups (resolveGroupName).
func sameNamedGroups(input string, groups []sdk.ServerGroup) []sdk.ServerGroup {
	var matches []sdk.ServerGroup
	for _, g := range groups {
		if strings.EqualFold(g.Name, input) {
			matches = append(matches, g)
		}
	}
	return matches
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
