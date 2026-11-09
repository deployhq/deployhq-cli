package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

// customDomainRules is the product behaviour every domains command explains:
// what a site may have, what the customer has to do, and what happens next.
const customDomainRules = `A Static Hosting site can have at most 2 custom domains: an apex + www pair
(e.g. example.com and www.example.com). The first domain added is the primary;
the other redirects to it. Custom domains need a paid plan.

For each domain, add BOTH DNS records that "dhq domains add" prints at your DNS
provider:
  - the CNAME record (for an apex domain such as example.com, use an ALIAS or
    ANAME record, or CNAME flattening, if your provider does not allow a CNAME
    at the apex), and
  - the TXT record, which proves you own the domain.

The domain goes live automatically once both records are detected (DeployHQ
checks every few minutes). A domain that is not verified within 7 days expires;
remove it and add it again to start over.`

func newDomainsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "domains",
		Aliases: []string{"domain", "custom-domains"},
		Short:   "Manage custom domains for a Static Hosting site",
		Long: `Attach your own domain (e.g. www.example.com) to a Static Hosting server.

` + customDomainRules + `

Every command takes the project (-p) and the Static Hosting server (-s, name or
identifier). For a global Static Hosting server, pass --global instead of the
project. Other server types do not support custom domains.`,
		Example: `  dhq domains add www.example.com -p my-site -s "My Site"
  dhq domains list -p my-site -s "My Site"
  dhq domains show www.example.com -p my-site -s "My Site"
  dhq domains remove www.example.com -p my-site -s "My Site" --yes
  dhq domains check www.example.com -p my-site -s "My Site"
  dhq domains list --global -s "Shared Site"`,
	}

	cmd.AddCommand(
		newDomainsListCmd(),
		newDomainsShowCmd(),
		newDomainsAddCmd(),
		newDomainsRemoveCmd(),
		newDomainsCheckCmd(),
	)
	return cmd
}

// domainsServerFlags are the flags every domains command names its server with.
type domainsServerFlags struct {
	server string
	global bool
}

// addDomainsServerFlags registers the required -s/--server flag and --global.
func addDomainsServerFlags(cmd *cobra.Command, f *domainsServerFlags) {
	cmd.Flags().StringVarP(&f.server, "server", "s", "", "Static Hosting server (name or identifier, required)")
	_ = cmd.MarkFlagRequired("server")
	cmd.Flags().BoolVar(&f.global, "global", false, "--server is a global server (no project needed)")
}

// domainsTarget is the Static Hosting server a domains command acts on.
type domainsTarget struct {
	scope     sdk.CustomDomainScope
	projectID string // empty for a global server
	server    string // as the user typed it, for the commands we suggest
	global    bool
}

// flags renders the flags that name this server in a suggested command.
func (t domainsTarget) flags() string {
	server := shellQuoteIfNeeded(t.server)
	if t.global {
		return "--global -s " + server
	}
	return fmt.Sprintf("-p %s -s %s", t.projectID, server)
}

// listServersCmd is the command that lists the servers this target is one of.
func (t domainsTarget) listServersCmd() string {
	if t.global {
		return "dhq global-servers list"
	}
	return "dhq servers list -p " + t.projectID
}

// openDomainsTarget resolves the flags to a server: the project (unless
// --global) and the -s value, as a name or an identifier.
func openDomainsTarget(ctx context.Context, f domainsServerFlags) (*sdk.Client, domainsTarget, error) {
	target := domainsTarget{server: strings.TrimSpace(f.server), global: f.global}
	if !f.global {
		projectID, err := cliCtx.RequireProject()
		if err != nil {
			return nil, target, err
		}
		target.projectID = projectID
	}
	client, err := cliCtx.Client()
	if err != nil {
		return nil, target, err
	}
	serverID, err := resolveDomainsServer(ctx, client, target)
	if err != nil {
		return nil, target, err
	}
	if f.global {
		target.scope = sdk.GlobalServerDomains(serverID)
	} else {
		target.scope = sdk.ProjectServerDomains(target.projectID, serverID)
	}
	return client, target, nil
}

// resolveDomainsServer turns the -s value into a server identifier.
func resolveDomainsServer(ctx context.Context, client *sdk.Client, target domainsTarget) (string, error) {
	in := target.server
	if in == "" {
		return "", &output.UserError{
			Message: "--server is required",
			Hint:    "Find the Static Hosting server with: " + target.listServersCmd(),
		}
	}
	// A canonical UUID is taken as an identifier without a listing.
	if isCanonicalUUID(in) {
		return in, nil
	}
	var (
		servers []sdk.Server
		err     error
	)
	if target.global {
		servers, err = client.ListGlobalServers(ctx, nil)
	} else {
		servers, err = client.ListServers(ctx, target.projectID, nil)
	}
	if err != nil {
		return "", err
	}
	if id, _ := resolveServerName(in, servers); id != "" {
		if dups := sameNamedServers(in, servers); len(dups) > 1 {
			return "", ambiguousServerError(in, dups, nil)
		}
		return id, nil
	}
	if isListedIdentifier(in, servers, nil) {
		return in, nil
	}
	var names []string
	for _, s := range servers {
		if s.ProtocolType == "static_hosting" {
			names = append(names, fmt.Sprintf("%s (%s)", s.Name, s.Identifier))
		}
	}
	hint := fmt.Sprintf("This project has no Static Hosting server. Create one with: dhq servers create -p %s --protocol-type static_hosting ...", target.projectID)
	if target.global {
		hint = "This account has no global Static Hosting server. See its global servers with: dhq global-servers list"
	}
	if len(names) > 0 {
		hint = "Static Hosting servers: " + strings.Join(names, ", ")
	}
	return "", &output.UserError{
		Message: fmt.Sprintf("No unique server matches %q", in),
		Hint:    hint,
	}
}

// explainDomainsError rewrites API refusals into messages about custom
// domains. A paid_plan_required refusal is left as is: output.ExplainAccountError
// already turns it into a plan message carrying the API's hint.
func explainDomainsError(err error, target domainsTarget) error {
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch {
	case apiErr.IsNotFound():
		return &output.UserError{
			Message: fmt.Sprintf("Not found: server %q is not a Static Hosting server, or the custom domain does not exist", target.server),
			Hint: fmt.Sprintf("Custom domains are only available on Static Hosting servers. Check with: %s, and dhq domains list %s",
				target.listServersCmd(), target.flags()),
			Recovery: []output.Breadcrumb{
				{Action: "list_servers", Cmd: target.listServersCmd() + " --json"},
				{Action: "list_domains", Cmd: fmt.Sprintf("dhq domains list %s --json", target.flags())},
			},
		}
	case apiErr.StatusCode == 403 && apiErr.ErrorCode == "access_denied":
		msg := apiErr.Message
		if msg == "" || msg == apiErr.ErrorCode {
			msg = "You do not have permission to manage custom domains for this server"
		}
		hint := apiErr.Hint
		if hint == "" {
			hint = "Ask an account administrator for access to this project's servers."
			if target.global {
				hint = "Adding, removing or checking a domain on a global server needs an account administrator."
			}
		}
		return &output.AuthError{Message: msg, Hint: hint}
	case apiErr.IsRateLimited():
		msg := apiErr.Message
		if msg == "" {
			msg = "Too many custom domain requests"
		}
		hint := "Wait a while, then run the command again."
		if apiErr.RetryAfter > 0 {
			hint = fmt.Sprintf("Try again in %d seconds.", apiErr.RetryAfter)
		}
		return &output.UserError{Message: msg, Hint: hint}
	case apiErr.StatusCode == 503:
		msg := apiErr.Message
		if msg == "" {
			msg = "Custom domains are temporarily unavailable"
		}
		return &output.UserError{
			Message: msg,
			Hint:    "The DNS/CDN provider behind Static Hosting is unavailable right now. Nothing was changed; retry in a few minutes.",
		}
	}
	return err
}

// recordField renders a nullable DNS record field.
func recordField(s *string) string {
	if s == nil || *s == "" {
		return "(not issued yet)"
	}
	return *s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// isApexHostname reports whether hostname looks like an apex (bare) domain
// such as example.com. It is a heuristic for help text only: multi-part public
// suffixes (example.co.uk) read as subdomains.
func isApexHostname(hostname string) bool {
	return strings.Count(strings.Trim(hostname, "."), ".") == 1
}

// writeDNSRecords prints the DNS records a domain needs: a table on stdout and
// the instructions on stderr.
func writeDNSRecords(env *output.Envelope, d *sdk.CustomDomain, showCmd string) {
	rows := make([][]string, len(d.DNSRecords))
	for i, r := range d.DNSRecords {
		rows[i] = []string{r.Type, recordField(r.Name), recordField(r.Value)}
	}
	env.Status("\nAdd BOTH of these DNS records at your DNS provider for %s:\n", d.Hostname)
	env.WriteTable([]string{"Type", "Name", "Value"}, rows)
	if isApexHostname(d.Hostname) {
		env.Status("\n%s is an apex domain: most DNS providers do not allow a CNAME there.", d.Hostname)
		env.Status("Use an ALIAS or ANAME record, or CNAME flattening, with the same value instead.")
	}
	for _, r := range d.DNSRecords {
		if r.Type == "TXT" && (r.Name == nil || r.Value == nil) {
			env.Status("\nThe TXT record is not issued yet. Check again shortly with: %s", showCmd)
			break
		}
	}
	env.Status("\nThe TXT record proves you own the domain. Once both records are detected")
	env.Status("(DeployHQ checks every few minutes) the domain goes live automatically.")
	env.Status("Domains not verified within 7 days expire.")
}

// domainsShowCmd is the command that shows a domain's status and DNS records.
func domainsShowCmd(hostname string, target domainsTarget) string {
	return fmt.Sprintf("dhq domains show %s %s", hostname, target.flags())
}

// findDomain matches arg against the identifier or hostname of each domain.
func findDomain(domains []sdk.CustomDomain, arg string) *sdk.CustomDomain {
	for i := range domains {
		if domains[i].Identifier == arg || strings.EqualFold(domains[i].Hostname, strings.TrimSuffix(arg, ".")) {
			return &domains[i]
		}
	}
	return nil
}

func domainNotFoundError(arg string, target domainsTarget, domains []sdk.CustomDomain) error {
	var names []string
	for _, d := range domains {
		names = append(names, fmt.Sprintf("%s (%s)", d.Hostname, d.Identifier))
	}
	hint := "This server has no custom domains. Add one with: dhq domains add <hostname> " + target.flags()
	if len(names) > 0 {
		hint = "Custom domains on this server: " + strings.Join(names, ", ")
	}
	return &output.UserError{
		Message: fmt.Sprintf("No custom domain %q on this server", arg),
		Hint:    hint,
	}
}

func newDomainsListCmd() *cobra.Command {
	var flags domainsServerFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the custom domains of a Static Hosting server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}

			domains, err := client.ListCustomDomains(ctx, target.scope)
			if err != nil {
				return explainDomainsError(err, target)
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(domains,
					fmt.Sprintf("%d custom domains", len(domains)),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq domains show <hostname> %s", target.flags())},
					output.Breadcrumb{Action: "add", Cmd: fmt.Sprintf("dhq domains add <hostname> %s", target.flags())},
				))
			}

			if env.QuietMode {
				ids := make([]string, len(domains))
				for i, d := range domains {
					ids[i] = d.Identifier
				}
				env.WriteQuiet(ids)
				return nil
			}

			if len(domains) == 0 {
				env.Status("No custom domains. Add one with: dhq domains add <hostname> %s", target.flags())
				return nil
			}
			writeDomainsTable(env, domains)
			for _, d := range domains {
				if d.Status == "pending" {
					env.Status("\nPending domains go live once both DNS records are detected. See them with:")
					env.Status("  dhq domains show <hostname> %s", target.flags())
					break
				}
			}
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	return cmd
}

func writeDomainsTable(env *output.Envelope, domains []sdk.CustomDomain) {
	rows := make([][]string, len(domains))
	for i, d := range domains {
		rows[i] = []string{d.Hostname, output.ColorStatus(d.Status), yesNo(d.Primary), d.Identifier}
	}
	env.WriteTable([]string{"Hostname", "Status", "Primary", "Identifier"}, rows)
}

func newDomainsShowCmd() *cobra.Command {
	var flags domainsServerFlags
	cmd := &cobra.Command{
		Use:   "show <hostname|identifier>",
		Short: "Show a custom domain and the DNS records it needs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}

			// There is no single-domain endpoint; a server has at most two.
			domains, err := client.ListCustomDomains(ctx, target.scope)
			if err != nil {
				return explainDomainsError(err, target)
			}
			d := findDomain(domains, args[0])
			if d == nil {
				return domainNotFoundError(args[0], target, domains)
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(d,
					fmt.Sprintf("Custom domain %s: %s", d.Hostname, d.Status),
					output.Breadcrumb{Action: "remove", Cmd: fmt.Sprintf("dhq domains remove %s %s", d.Hostname, target.flags())},
				))
			}

			writeDomainDetails(env, d)
			if d.Status != "active" {
				writeDNSRecords(env, d, domainsShowCmd(d.Hostname, target))
			}
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	return cmd
}

func writeDomainDetails(env *output.Envelope, d *sdk.CustomDomain) {
	ssl := ""
	if d.SSLStatus != nil {
		ssl = *d.SSLStatus
	}
	activated := ""
	if d.ActivatedAt != nil {
		activated = *d.ActivatedAt
	}
	env.WriteTable([]string{"Field", "Value"}, [][]string{
		{"Hostname", d.Hostname},
		{"Identifier", d.Identifier},
		{"Status", output.ColorStatus(d.Status)},
		{"Primary", yesNo(d.Primary)},
		{"SSL", ssl},
		{"Activated", activated},
		{"Created", d.CreatedAt},
	})
	if len(d.VerificationErrors) > 0 {
		env.Status("\nVerification errors:")
		for _, e := range d.VerificationErrors {
			env.Status("  - %s", e)
		}
	}
	if d.Status == "expired" {
		env.Status("\nThis domain expired before it was verified. Remove it and add it again to start over.")
	}
}

func newDomainsAddCmd() *cobra.Command {
	var flags domainsServerFlags
	var withWWW bool
	cmd := &cobra.Command{
		Use:   "add <hostname>",
		Short: "Add a custom domain to a Static Hosting server",
		Long: `Add a custom domain to a Static Hosting server and print the DNS records to create.

` + customDomainRules,
		Example: `  dhq domains add www.example.com -p my-site -s "My Site"
  dhq domains add example.com -p my-site -s "My Site"    # the apex of the pair
  dhq domains add example.com -p my-site -s "My Site" --with-www    # both halves of the pair`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostname := strings.TrimSuffix(strings.TrimSpace(strings.ToLower(args[0])), ".")
			if hostname == "" || strings.Contains(hostname, "://") || strings.ContainsAny(hostname, "/ ") {
				return &output.UserError{
					Message: fmt.Sprintf("%q is not a hostname", args[0]),
					Hint:    "Pass just the domain, e.g. www.example.com (no https:// or path).",
				}
			}

			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}

			d, err := client.CreateCustomDomain(ctx, target.scope, sdk.CustomDomainCreateRequest{Hostname: hostname, WithCounterpart: withWWW})
			if err != nil {
				return explainDomainsError(err, target)
			}

			return reportDomainsAdded(cliCtx.Envelope, d, target, withWWW)
		},
	}
	addDomainsServerFlags(cmd, &flags)
	cmd.Flags().BoolVar(&withWWW, "with-www", false, "Also add the other half of the apex and www pair (only for an apex domain or its www form, on a site with no other custom domains)")
	return cmd
}

// reportDomainsAdded prints the domain just added and the DNS records it needs.
func reportDomainsAdded(env *output.Envelope, created *sdk.CustomDomainCreation, target domainsTarget, withWWW bool) error {
	showCmd := domainsShowCmd(created.Hostname, target)
	if env.WantsJSON() {
		return env.WriteJSON(output.NewResponse(created,
			fmt.Sprintf("Custom domain %s added (%s). Create both dns_records at the DNS provider; it goes live automatically once they are detected, and expires if not verified within 7 days.", created.Hostname, created.Status),
			output.Breadcrumb{Action: "show", Cmd: showCmd},
			output.Breadcrumb{Action: "list", Cmd: fmt.Sprintf("dhq domains list %s", target.flags())},
		))
	}

	primary := ""
	if created.Primary {
		primary = " as the primary domain"
	}
	env.Status("Added %s%s (status: %s)", created.Hostname, primary, created.Status)
	writeDNSRecords(env, &created.CustomDomain, showCmd)
	if c := created.Counterpart; c != nil {
		env.Status("\nAlso added %s (status: %s)", c.Hostname, c.Status)
		writeDNSRecords(env, c, domainsShowCmd(c.Hostname, target))
	}
	for _, e := range created.CounterpartErrors {
		env.Status("\nWarning: the other half of the pair could not be added: %s", e)
	}
	if withWWW && created.Counterpart == nil && len(created.CounterpartErrors) == 0 {
		env.Status("\nWarning: --with-www added nothing: it needs an apex domain or its www form, on a site with no other custom domains.")
	}
	env.Status("\nCheck progress with: %s", showCmd)
	return nil
}

func newDomainsRemoveCmd() *cobra.Command {
	var flags domainsServerFlags
	var yes bool
	cmd := &cobra.Command{
		Use:     "remove <hostname|identifier>",
		Aliases: []string{"delete", "rm"},
		Short:   "Remove a custom domain from a Static Hosting server",
		Long: `Remove a custom domain. The site stops answering on that domain; removal
happens in the background. Asks for confirmation unless --yes is given, which
is required in non-interactive mode (agents, CI, piped output, --json).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			if !yes && (!env.IsTTY || env.NonInteractive || env.JSONMode) {
				return &output.UserError{
					Message: "Removing a custom domain needs confirmation",
					Hint:    "Re-run with --yes to remove it without a prompt.",
				}
			}

			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}

			// Resolve a hostname to its identifier (and confirm it exists).
			domains, err := client.ListCustomDomains(ctx, target.scope)
			if err != nil {
				return explainDomainsError(err, target)
			}
			d := findDomain(domains, args[0])
			if d == nil {
				return domainNotFoundError(args[0], target, domains)
			}

			if !yes {
				fmt.Fprintf(env.Stderr, "Remove custom domain %s? The site stops answering on it. [y/N]: ", d.Hostname) //nolint:errcheck
				answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					return &output.UserError{Message: "Custom domain removal cancelled"}
				}
			}

			removal, err := client.DeleteCustomDomain(ctx, target.scope, d.Identifier)
			if err != nil {
				return explainDomainsError(err, target)
			}

			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(removal,
					fmt.Sprintf("Removal of %s requested; it happens in the background", d.Hostname),
					output.Breadcrumb{Action: "list", Cmd: fmt.Sprintf("dhq domains list %s", target.flags())},
				))
			}
			env.Status("Removal of %s requested; it happens in the background.", d.Hostname)
			env.Status("Remove its DNS records at your DNS provider once it is gone. Check with: dhq domains list %s", target.flags())
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Remove without asking for confirmation (required in non-interactive mode)")
	return cmd
}

func newDomainsCheckCmd() *cobra.Command {
	var flags domainsServerFlags
	cmd := &cobra.Command{
		Use:   "check <hostname|identifier>",
		Short: "Check a pending custom domain now",
		Long: `Ask DeployHQ to check a pending domain's DNS records now, instead of on the next
scheduled check (every few minutes). The check runs in the background: read the
outcome a few seconds later with "dhq domains show". Only a pending domain can be
checked, and at most once a minute.`,
		Example: `  dhq domains check www.example.com -p my-site -s "My Site"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}

			// Resolve a hostname to its identifier (and confirm it exists).
			domains, err := client.ListCustomDomains(ctx, target.scope)
			if err != nil {
				return explainDomainsError(err, target)
			}
			d := findDomain(domains, args[0])
			if d == nil {
				return domainNotFoundError(args[0], target, domains)
			}

			check, err := client.RefreshCustomDomain(ctx, target.scope, d.Identifier)
			if err != nil {
				return explainDomainsCheckError(err, d, target)
			}

			showCmd := domainsShowCmd(d.Hostname, target)
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(check,
					fmt.Sprintf("Check of %s requested; read the outcome with dhq domains show in a few seconds", d.Hostname),
					output.Breadcrumb{Action: "show", Cmd: showCmd},
				))
			}
			env.Status("Checking %s now. See the outcome in a few seconds with: %s", d.Hostname, showCmd)
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	return cmd
}

// explainDomainsCheckError explains why a check was refused. A 422 means the
// domain is not waiting for verification (or its site is being deleted); the
// other refusals are shared with the rest of the domains commands.
func explainDomainsCheckError(err error, d *sdk.CustomDomain, target domainsTarget) error {
	var apiErr *sdk.APIError
	if errors.As(err, &apiErr) && apiErr.IsValidationError() {
		msg := apiErr.Message
		if msg == "" {
			msg = fmt.Sprintf("%s cannot be checked", d.Hostname)
		}
		showCmd := domainsShowCmd(d.Hostname, target)
		return &output.UserError{
			Message:  msg,
			Hint:     "Only a pending domain can be checked. See its status with: " + showCmd,
			Recovery: []output.Breadcrumb{{Action: "show", Cmd: showCmd + " --json"}},
		}
	}
	return explainDomainsError(err, target)
}
