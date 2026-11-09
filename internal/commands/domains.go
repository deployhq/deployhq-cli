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
identifier). Other server types do not support custom domains.`,
		Example: `  dhq domains add www.example.com -p my-site -s "My Site"
  dhq domains list -p my-site -s "My Site"
  dhq domains show www.example.com -p my-site -s "My Site"
  dhq domains remove www.example.com -p my-site -s "My Site" --yes`,
	}

	cmd.AddCommand(
		newDomainsListCmd(),
		newDomainsShowCmd(),
		newDomainsAddCmd(),
		newDomainsRemoveCmd(),
	)
	return cmd
}

// addDomainsServerFlag registers the required -s/--server flag.
func addDomainsServerFlag(cmd *cobra.Command, server *string) {
	cmd.Flags().StringVarP(server, "server", "s", "", "Static Hosting server (name or identifier, required)")
	_ = cmd.MarkFlagRequired("server")
}

// domainsTarget resolves the project and the -s value to a server identifier.
func domainsTarget(ctx context.Context, client *sdk.Client, projectID, server string) (string, error) {
	in := strings.TrimSpace(server)
	if in == "" {
		return "", &output.UserError{
			Message: "--server is required",
			Hint:    fmt.Sprintf("Find the Static Hosting server with: dhq servers list -p %s", projectID),
		}
	}
	// A canonical UUID is taken as an identifier without a listing.
	if isCanonicalUUID(in) {
		return in, nil
	}
	servers, err := client.ListServers(ctx, projectID, nil)
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
	hint := fmt.Sprintf("This project has no Static Hosting server. Create one with: dhq servers create -p %s --protocol-type static_hosting ...", projectID)
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
func explainDomainsError(err error, projectID, server string) error {
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch {
	case apiErr.IsNotFound():
		return &output.UserError{
			Message: fmt.Sprintf("Not found: server %q is not a Static Hosting server, or the custom domain does not exist", server),
			Hint: fmt.Sprintf("Custom domains are only available on Static Hosting servers. Check with: dhq servers list -p %s, and dhq domains list -p %s -s %s",
				projectID, projectID, server),
			Recovery: []output.Breadcrumb{
				{Action: "list_servers", Cmd: fmt.Sprintf("dhq servers list -p %s --json", projectID)},
				{Action: "list_domains", Cmd: fmt.Sprintf("dhq domains list -p %s -s %s --json", projectID, server)},
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
		}
		return &output.AuthError{Message: msg, Hint: hint}
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
func domainsShowCmd(hostname, projectID, server string) string {
	return fmt.Sprintf("dhq domains show %s -p %s -s %q", hostname, projectID, server)
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

func domainNotFoundError(arg, projectID, server string, domains []sdk.CustomDomain) error {
	var names []string
	for _, d := range domains {
		names = append(names, fmt.Sprintf("%s (%s)", d.Hostname, d.Identifier))
	}
	hint := "This server has no custom domains. Add one with: dhq domains add <hostname> -p " + projectID + " -s " + server
	if len(names) > 0 {
		hint = "Custom domains on this server: " + strings.Join(names, ", ")
	}
	return &output.UserError{
		Message: fmt.Sprintf("No custom domain %q on this server", arg),
		Hint:    hint,
	}
}

func newDomainsListCmd() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the custom domains of a Static Hosting server",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			ctx := cliCtx.Background()
			serverID, err := domainsTarget(ctx, client, projectID, server)
			if err != nil {
				return err
			}

			domains, err := client.ListCustomDomains(ctx, sdk.ProjectServerDomains(projectID, serverID))
			if err != nil {
				return explainDomainsError(err, projectID, server)
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(domains,
					fmt.Sprintf("%d custom domains", len(domains)),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq domains show <hostname> -p %s -s %s", projectID, server)},
					output.Breadcrumb{Action: "add", Cmd: fmt.Sprintf("dhq domains add <hostname> -p %s -s %s", projectID, server)},
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
				env.Status("No custom domains. Add one with: dhq domains add <hostname> -p %s -s %s", projectID, server)
				return nil
			}
			writeDomainsTable(env, domains)
			for _, d := range domains {
				if d.Status == "pending" {
					env.Status("\nPending domains go live once both DNS records are detected. See them with:")
					env.Status("  dhq domains show <hostname> -p %s -s %s", projectID, server)
					break
				}
			}
			return nil
		},
	}
	addDomainsServerFlag(cmd, &server)
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
	var server string
	cmd := &cobra.Command{
		Use:   "show <hostname|identifier>",
		Short: "Show a custom domain and the DNS records it needs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			ctx := cliCtx.Background()
			serverID, err := domainsTarget(ctx, client, projectID, server)
			if err != nil {
				return err
			}

			// There is no single-domain endpoint; a server has at most two.
			domains, err := client.ListCustomDomains(ctx, sdk.ProjectServerDomains(projectID, serverID))
			if err != nil {
				return explainDomainsError(err, projectID, server)
			}
			d := findDomain(domains, args[0])
			if d == nil {
				return domainNotFoundError(args[0], projectID, server, domains)
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(d,
					fmt.Sprintf("Custom domain %s: %s", d.Hostname, d.Status),
					output.Breadcrumb{Action: "remove", Cmd: fmt.Sprintf("dhq domains remove %s -p %s -s %s", d.Hostname, projectID, server)},
				))
			}

			writeDomainDetails(env, d)
			if d.Status != "active" {
				writeDNSRecords(env, d, domainsShowCmd(d.Hostname, projectID, server))
			}
			return nil
		},
	}
	addDomainsServerFlag(cmd, &server)
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
	var server string
	cmd := &cobra.Command{
		Use:   "add <hostname>",
		Short: "Add a custom domain to a Static Hosting server",
		Long: `Add a custom domain to a Static Hosting server and print the DNS records to create.

` + customDomainRules,
		Example: `  dhq domains add www.example.com -p my-site -s "My Site"
  dhq domains add example.com -p my-site -s "My Site"    # the apex of the pair`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostname := strings.TrimSuffix(strings.TrimSpace(strings.ToLower(args[0])), ".")
			if hostname == "" || strings.Contains(hostname, "://") || strings.ContainsAny(hostname, "/ ") {
				return &output.UserError{
					Message: fmt.Sprintf("%q is not a hostname", args[0]),
					Hint:    "Pass just the domain, e.g. www.example.com (no https:// or path).",
				}
			}

			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			ctx := cliCtx.Background()
			serverID, err := domainsTarget(ctx, client, projectID, server)
			if err != nil {
				return err
			}

			d, err := client.CreateCustomDomain(ctx, sdk.ProjectServerDomains(projectID, serverID), sdk.CustomDomainCreateRequest{Hostname: hostname})
			if err != nil {
				return explainDomainsError(err, projectID, server)
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(d,
					fmt.Sprintf("Custom domain %s added (%s). Create both dns_records at the DNS provider; it goes live automatically once they are detected, and expires if not verified within 7 days.", d.Hostname, d.Status),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq domains show %s -p %s -s %s", d.Hostname, projectID, server)},
					output.Breadcrumb{Action: "list", Cmd: fmt.Sprintf("dhq domains list -p %s -s %s", projectID, server)},
				))
			}

			primary := ""
			if d.Primary {
				primary = " as the primary domain"
			}
			env.Status("Added %s%s (status: %s)", d.Hostname, primary, d.Status)
			showCmd := domainsShowCmd(d.Hostname, projectID, server)
			writeDNSRecords(env, &d.CustomDomain, showCmd)
			env.Status("\nCheck progress with: %s", showCmd)
			return nil
		},
	}
	addDomainsServerFlag(cmd, &server)
	return cmd
}

func newDomainsRemoveCmd() *cobra.Command {
	var server string
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

			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			ctx := cliCtx.Background()
			serverID, err := domainsTarget(ctx, client, projectID, server)
			if err != nil {
				return err
			}

			// Resolve a hostname to its identifier (and confirm it exists).
			domains, err := client.ListCustomDomains(ctx, sdk.ProjectServerDomains(projectID, serverID))
			if err != nil {
				return explainDomainsError(err, projectID, server)
			}
			d := findDomain(domains, args[0])
			if d == nil {
				return domainNotFoundError(args[0], projectID, server, domains)
			}

			if !yes {
				fmt.Fprintf(env.Stderr, "Remove custom domain %s? The site stops answering on it. [y/N]: ", d.Hostname) //nolint:errcheck
				answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					return &output.UserError{Message: "Custom domain removal cancelled"}
				}
			}

			removal, err := client.DeleteCustomDomain(ctx, sdk.ProjectServerDomains(projectID, serverID), d.Identifier)
			if err != nil {
				return explainDomainsError(err, projectID, server)
			}

			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(removal,
					fmt.Sprintf("Removal of %s requested; it happens in the background", d.Hostname),
					output.Breadcrumb{Action: "list", Cmd: fmt.Sprintf("dhq domains list -p %s -s %s", projectID, server)},
				))
			}
			env.Status("Removal of %s requested; it happens in the background.", d.Hostname)
			env.Status("Remove its DNS records at your DNS provider once it is gone. Check with: dhq domains list -p %s -s %s", projectID, server)
			return nil
		},
	}
	addDomainsServerFlag(cmd, &server)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Remove without asking for confirmation (required in non-interactive mode)")
	return cmd
}
