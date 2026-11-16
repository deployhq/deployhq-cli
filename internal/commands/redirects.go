package commands

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

// redirectRulesHelp is the product behaviour every redirects command explains.
const redirectRulesHelp = `Redirect rules come from two places:
  - a _redirects file in the folder your deployment publishes, one rule per
    line ("/old /new 301"). It is versioned with your code and takes effect
    when you deploy.
  - rules managed with these commands (or in the dashboard). They take effect
    straight away, without a deploy.
Rules from the _redirects file are checked first: when a rule of each kind
matches a URL, the file wins. "list" shows both, and which rules here a file
rule overrides. Lines of the file that cannot be honoured (200 rewrites, 404 lines,
Country/Language/Role/Cookie conditions) are skipped and listed as warnings.

A source is a path such as /old-page, /blog/:slug or /docs/*. A destination is
a path or a full URL, and can use the source's :name captures and :splat.
Matching ignores a trailing slash. Status is 301 (the default), 302, 307 or 308.

--query matches query parameters as well as the path, for URLs such as
WordPress's /?p=123: "p=:id" requires the p parameter and captures its value
as :id for the destination ("p=123" requires that exact value). List several
as "key=value key2=:name". Parameters not listed (utm_source...) do not stop a
match.`

// redirectsServerHelp explains how every redirects command names its server.
const redirectsServerHelp = `Every command takes the project (-p) and the Static Hosting server (-s, name or
identifier). Without -s, the server from DEPLOYHQ_SERVER or .deployhq.toml is
used; dhq launch saves it there. For a global Static Hosting server, pass --global
instead of the project, with -s. Other server types do not support redirects.`

func newServersRedirectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "redirects",
		Aliases: []string{"redirect", "redirect-rules"},
		Short:   "Manage redirect rules for a Static Hosting site",
		Long: `Redirect old URLs of a Static Hosting site to new ones, for example after moving
a WordPress site.

` + redirectRulesHelp + `

` + redirectsServerHelp,
		Example: `  dhq servers redirects list -p my-site -s "My Site"
  dhq servers redirects add /old-page /new-page -p my-site -s "My Site"
  dhq servers redirects add / /posts/:id --query "p=:id" -p my-site -s "My Site"
  dhq servers redirects update <identifier> --status 302 -p my-site -s "My Site"
  dhq servers redirects reorder <identifier> <identifier> -p my-site -s "My Site"
  dhq servers redirects remove <identifier> -p my-site -s "My Site" --yes
  dhq servers redirects list --global -s "Shared Site"`,
	}
	cmd.AddCommand(
		newRedirectsListCmd(),
		newRedirectsAddCmd(),
		newRedirectsUpdateCmd(),
		newRedirectsRemoveCmd(),
		newRedirectsReorderCmd(),
	)
	return cmd
}

// redirectScope is the redirect rules scope of a resolved server.
func redirectScope(t domainsTarget) sdk.RedirectRuleScope {
	if t.global {
		return sdk.GlobalServerRedirects(t.serverID)
	}
	return sdk.ProjectServerRedirects(t.projectID, t.serverID)
}

func redirectsListCmd(t domainsTarget) string {
	return "dhq servers redirects list " + t.flags()
}

// validateRedirectStatus refuses a status the API does not accept, before any
// request is made.
func validateRedirectStatus(status int) error {
	for _, s := range sdk.RedirectStatuses {
		if s == status {
			return nil
		}
	}
	return &output.UserError{
		Message: fmt.Sprintf("--status %d is not a redirect status", status),
		Hint:    "Use 301 (permanent, the default), 302 (temporary), 307 or 308.",
	}
}

func validateRedirectPosition(position int) error {
	if position < 1 {
		return &output.UserError{
			Message: fmt.Sprintf("--position %d is not a position", position),
			Hint:    "Positions start at 1 (the first rule checked).",
		}
	}
	return nil
}

// explainRedirectsError rewrites API refusals into messages about redirect
// rules.
func explainRedirectsError(err error, target domainsTarget) error {
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	listCmd := redirectsListCmd(target)
	if deleting := siteBeingDeletedError(apiErr, target); deleting != nil {
		return deleting
	}
	switch {
	case apiErr.IsNotFound():
		return &output.UserError{
			Message: fmt.Sprintf("Not found: server %q is not a Static Hosting server, or the redirect rule does not exist", target.server),
			Hint: fmt.Sprintf("Redirect rules are only available on Static Hosting servers. Check with: %s, and %s",
				target.listServersCmd(), listCmd),
			Recovery: []output.Breadcrumb{
				{Action: "list_servers", Cmd: target.listServersCmd() + " --json"},
				{Action: "list_redirects", Cmd: listCmd + " --json"},
			},
		}
	case apiErr.StatusCode == 403 && apiErr.ErrorCode == "access_denied":
		msg := apiErr.Message
		if msg == "" || msg == apiErr.ErrorCode {
			msg = "You do not have permission to manage redirect rules for this server"
		}
		hint := apiErr.Hint
		if hint == "" {
			hint = "Ask an account administrator for access to this project's servers."
			if target.global {
				hint = "Changing the redirect rules of a global server needs an account administrator."
			}
		}
		return &output.AuthError{Message: msg, Hint: hint}
	case apiErr.IsRateLimited():
		msg := apiErr.Message
		if msg == "" {
			msg = "Too many redirect rule requests"
		}
		hint := "Wait a while, then run the command again."
		if apiErr.RetryAfter > 0 {
			hint = fmt.Sprintf("Try again in %d seconds.", apiErr.RetryAfter)
		}
		return &output.UserError{Message: msg, Hint: hint, Retryable: true, RetryAfter: apiErr.RetryAfter}
	case apiErr.StatusCode == 409:
		msg := apiErr.Message
		if msg == "" {
			msg = "The site is busy publishing a deployment or another change"
		}
		return &output.UserError{
			Message:   msg,
			Hint:      "Nothing was changed. Run the command again in a moment, then check the rules with: " + listCmd,
			Retryable: true,
			Recovery:  []output.Breadcrumb{{Action: "list_redirects", Cmd: listCmd + " --json"}},
		}
	case apiErr.StatusCode == 503:
		msg := apiErr.Message
		if msg == "" {
			msg = "Redirect rules cannot be published right now"
		}
		return &output.UserError{
			Message:   msg,
			Hint:      "Nothing was changed: the rules could not be published to the live site. Retry in a few minutes, then check the rules with: " + listCmd,
			Retryable: true,
		}
	case apiErr.IsValidationError():
		msg := strings.Join(apiErr.Errors, "; ")
		if msg == "" {
			msg = apiErr.Message
		}
		return &output.UserError{
			Message:  "Redirect rule refused: " + msg,
			Hint:     "A source is a path starting with /, a destination a path or an http(s) URL, and the status 301, 302, 307 or 308. See this site's rules with: " + listCmd,
			Recovery: []output.Breadcrumb{{Action: "list_redirects", Cmd: listCmd + " --json"}},
		}
	}
	return err
}

// explainRedirectsReorderError explains a refused reorder: a 422 for anything
// but a site being deleted means the identifiers were not every rule exactly
// once.
func explainRedirectsReorderError(err error, target domainsTarget) error {
	var apiErr *sdk.APIError
	if errors.As(err, &apiErr) && apiErr.IsValidationError() && siteBeingDeletedError(apiErr, target) == nil {
		msg := strings.Join(apiErr.Errors, "; ")
		if msg == "" {
			msg = apiErr.Message
		}
		listCmd := redirectsListCmd(target)
		return &output.UserError{
			Message:  "Redirect rules not reordered: " + msg,
			Hint:     "Name every rule of the site exactly once, in the new order, or move one rule with \"update <identifier> --position N\". See the rules with: " + listCmd,
			Recovery: []output.Breadcrumb{{Action: "list_redirects", Cmd: listCmd + " --json"}},
		}
	}
	return explainRedirectsError(err, target)
}

// queryText renders a nullable query matcher.
func queryText(q *string) string {
	if q == nil {
		return ""
	}
	return *q
}

// redirectRuleSummary is a rule on one line: "/ ?p=:id -> /posts/:id (301)".
func redirectRuleSummary(source string, query *string, destination string, status int) string {
	from := source
	if q := queryText(query); q != "" {
		from += " ?" + q
	}
	return fmt.Sprintf("%s -> %s (%d)", from, destination, status)
}

func writeRedirectRulesTable(env *output.Envelope, rules []sdk.RedirectRule) {
	rows := make([][]string, len(rules))
	for i, r := range rules {
		rows[i] = []string{strconv.Itoa(r.Position), r.Source, queryText(r.Query), r.Destination, strconv.Itoa(r.Status), r.Identifier}
	}
	env.WriteTable([]string{"Position", "Source", "Query", "Destination", "Status", "Identifier"}, rows)
}

// warnShadowed says which rules another rule takes over: a _redirects rule
// (checked first) for some or all of the URLs they match, or an earlier rule
// managed here for all of them, so they never apply.
func warnShadowed(env *output.Envelope, rules []sdk.RedirectRule) {
	for _, r := range rules {
		s := r.ShadowedBy
		switch {
		case s == nil:
		case s.RuleIdentifier != nil:
			env.Warn("Rule %d (%s) never applies: earlier rule %s (%s) matches first.",
				r.Position, r.Source, *s.RuleIdentifier, s.Source)
		case s.Line != nil:
			env.Warn("Rule %d (%s) is overridden, for some or all of the URLs it matches, by line %d of the _redirects file (%s), which is checked first.",
				r.Position, r.Source, *s.Line, s.Source)
		default:
			env.Warn("Rule %d (%s) is overridden, for some or all of the URLs it matches, by %s in the _redirects file, which is checked first.",
				r.Position, r.Source, s.Source)
		}
	}
}

func writeRedirectFile(env *output.Envelope, rules *sdk.RedirectRules) {
	if rules.DeploymentUUID == nil {
		env.Status("\nRules from a _redirects file appear here once the site has been deployed.")
		return
	}
	if len(rules.FileRules) == 0 && len(rules.FileWarnings) == 0 {
		env.Status("\nThe active deployment has no _redirects file rules.")
		return
	}
	if len(rules.FileRules) > 0 {
		env.Status("\nFrom the _redirects file of the active deployment (checked first):")
		rows := make([][]string, len(rules.FileRules))
		for i, r := range rules.FileRules {
			rows[i] = []string{strconv.Itoa(r.Line), r.Source, queryText(r.Query), r.Destination, strconv.Itoa(r.Status)}
		}
		env.WriteTable([]string{"Line", "Source", "Query", "Destination", "Status"}, rows)
	}
	var skipped, notes []sdk.RedirectFileWarning
	for _, w := range rules.FileWarnings {
		if w.IsNote() {
			notes = append(notes, w)
		} else {
			skipped = append(skipped, w)
		}
	}
	writeFileWarnings(env, "\nLines of the _redirects file that were skipped:", skipped)
	writeFileWarnings(env, "\nLines of the _redirects file that were applied in a normalised form:", notes)
}

func writeFileWarnings(env *output.Envelope, heading string, warnings []sdk.RedirectFileWarning) {
	if len(warnings) == 0 {
		return
	}
	env.Status("%s", heading)
	for _, w := range warnings {
		if w.Line != nil {
			env.Status("  line %d: %s", *w.Line, w.Reason)
		} else {
			env.Status("  %s", w.Reason)
		}
	}
}

func newRedirectsListCmd() *cobra.Command {
	var flags domainsServerFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the redirect rules of a Static Hosting server, and those of its _redirects file",
		Long: `List the redirect rules managed in DeployHQ, in the order they are checked, then
the rules and skipped lines of the _redirects file of the active deployment.

With --json, "rules" holds the rules managed in DeployHQ. "shadowed_by" names the
rule that takes one over: a _redirects line ("line") for some or all of the URLs
it matches, or an earlier rule here ("rule_identifier") that takes all of them,
so the rule never applies. "file_rules" and "file_warnings" hold the _redirects
file's rules and its skipped lines (codes source_normalised and
slash_only_redirect are notes on lines that were applied), and
"deployment_uuid" the deployment they come from (null before the first
deploy).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}
			rules, err := client.ListRedirectRules(ctx, redirectScope(target))
			if err != nil {
				return explainRedirectsError(err, target)
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(rules,
					fmt.Sprintf("%d redirect rules, %d from the _redirects file", len(rules.Rules), len(rules.FileRules)),
					output.Breadcrumb{Action: "add", Cmd: fmt.Sprintf("dhq servers redirects add <source> <destination> %s", target.flags())},
				))
			}
			if env.QuietMode {
				ids := make([]string, len(rules.Rules))
				for i, r := range rules.Rules {
					ids[i] = r.Identifier
				}
				env.WriteQuiet(ids)
				return nil
			}

			if len(rules.Rules) == 0 {
				env.Status("No redirect rules managed in DeployHQ. Add one with: dhq servers redirects add <source> <destination> %s", target.flags())
			} else {
				writeRedirectRulesTable(env, rules.Rules)
				warnShadowed(env, rules.Rules)
			}
			writeRedirectFile(env, rules)
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	return cmd
}

func newRedirectsAddCmd() *cobra.Command {
	var flags domainsServerFlags
	var status, position int
	var query string
	cmd := &cobra.Command{
		Use:   "add <source> <destination>",
		Short: "Add a redirect rule to a Static Hosting server",
		Long: `Add a redirect rule. It takes effect straight away, without a deploy. New rules
go last unless --position is given.

` + redirectRulesHelp,
		Example: `  dhq servers redirects add /old-page /new-page -p my-site -s "My Site"
  dhq servers redirects add /blog/:slug /news/:slug --status 302 -p my-site -s "My Site"
  dhq servers redirects add / /posts/:id --query "p=:id" -p my-site -s "My Site"
  dhq servers redirects add /docs/* https://docs.example.com/:splat --position 1 -p my-site -s "My Site"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := sdk.RedirectRuleCreateRequest{
				Source:      strings.TrimSpace(args[0]),
				Destination: strings.TrimSpace(args[1]),
				Query:       strings.TrimSpace(query),
			}
			if cmd.Flags().Changed("status") {
				if err := validateRedirectStatus(status); err != nil {
					return err
				}
				req.Status = status
			}
			if cmd.Flags().Changed("position") {
				if err := validateRedirectPosition(position); err != nil {
					return err
				}
				req.Position = &position
			}

			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}
			rule, err := client.CreateRedirectRule(ctx, redirectScope(target), req)
			if err != nil {
				return explainRedirectsError(err, target)
			}

			env := cliCtx.Envelope
			summary := redirectRuleSummary(rule.Source, rule.Query, rule.Destination, rule.Status)
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(rule,
					fmt.Sprintf("Added redirect %s at position %d", summary, rule.Position),
					output.Breadcrumb{Action: "list", Cmd: redirectsListCmd(target)},
				))
			}
			env.Status("Added redirect %s at position %d (%s)", summary, rule.Position, rule.Identifier)
			warnShadowed(env, []sdk.RedirectRule{*rule})
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	cmd.Flags().IntVar(&status, "status", 301, "Redirect status: 301, 302, 307 or 308")
	cmd.Flags().StringVar(&query, "query", "", `Query parameters to match as well as the path, e.g. "p=:id"`)
	cmd.Flags().IntVar(&position, "position", 0, "Where to insert the rule (1 = checked first; default: last)")
	return cmd
}

func newRedirectsUpdateCmd() *cobra.Command {
	var flags domainsServerFlags
	var source, destination, query string
	var status, position int
	cmd := &cobra.Command{
		Use:   "update <identifier>",
		Short: "Change a redirect rule",
		Long: `Change a redirect rule. Only the flags given are changed. --query "" removes
the query matcher, so the rule matches on the path alone. --position moves the
rule (1 = checked first; past the end moves it last); "reorder" sets the order
of every rule at once. Find identifiers with "dhq servers redirects list".`,
		Example: `  dhq servers redirects update <identifier> --destination /new-home -p my-site -s "My Site"
  dhq servers redirects update <identifier> --status 302 -p my-site -s "My Site"
  dhq servers redirects update <identifier> --query "" -p my-site -s "My Site"
  dhq servers redirects update <identifier> --position 1 -p my-site -s "My Site"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var req sdk.RedirectRuleUpdateRequest
			changed := cmd.Flags().Changed
			if changed("source") {
				v := strings.TrimSpace(source)
				req.Source = &v
			}
			if changed("destination") {
				v := strings.TrimSpace(destination)
				req.Destination = &v
			}
			if changed("status") {
				if err := validateRedirectStatus(status); err != nil {
					return err
				}
				req.Status = &status
			}
			if changed("query") {
				v := strings.TrimSpace(query)
				req.Query = &v
			}
			if changed("position") {
				if err := validateRedirectPosition(position); err != nil {
					return err
				}
				req.Position = &position
			}
			if req == (sdk.RedirectRuleUpdateRequest{}) {
				return &output.UserError{
					Message: "Nothing to change",
					Hint:    "Pass at least one of --source, --destination, --status, --query or --position.",
				}
			}

			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}
			rule, err := client.UpdateRedirectRule(ctx, redirectScope(target), args[0], req)
			if err != nil {
				return explainRedirectsError(err, target)
			}

			env := cliCtx.Envelope
			summary := redirectRuleSummary(rule.Source, rule.Query, rule.Destination, rule.Status)
			if req.Position != nil {
				summary = fmt.Sprintf("%s, now at position %d", summary, rule.Position)
			}
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(rule,
					"Updated redirect "+summary,
					output.Breadcrumb{Action: "list", Cmd: redirectsListCmd(target)},
				))
			}
			env.Status("Updated redirect %s", summary)
			warnShadowed(env, []sdk.RedirectRule{*rule})
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	cmd.Flags().StringVar(&source, "source", "", "New source path")
	cmd.Flags().StringVar(&destination, "destination", "", "New destination path or URL")
	cmd.Flags().IntVar(&status, "status", 0, "New status: 301, 302, 307 or 308")
	cmd.Flags().StringVar(&query, "query", "", `New query matcher, e.g. "p=:id" ("" removes it)`)
	cmd.Flags().IntVar(&position, "position", 0, "Move the rule to this position (1 = checked first)")
	return cmd
}

// findRedirectRule matches identifier against the rules.
func findRedirectRule(rules []sdk.RedirectRule, identifier string) *sdk.RedirectRule {
	for i := range rules {
		if rules[i].Identifier == identifier {
			return &rules[i]
		}
	}
	return nil
}

func newRedirectsRemoveCmd() *cobra.Command {
	var flags domainsServerFlags
	var yes bool
	cmd := &cobra.Command{
		Use:     "remove <identifier>",
		Aliases: []string{"delete", "rm"},
		Short:   "Remove a redirect rule",
		Long: `Remove a redirect rule managed in DeployHQ; it stops applying straight away.
Rules from a _redirects file are removed by editing the file and deploying.
Asks for confirmation unless --yes is given, which is required in
non-interactive mode (agents, CI, piped output, --json).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			if !yes && (!env.IsTTY || env.NonInteractive || env.JSONMode) {
				return &output.UserError{
					Message: "Removing a redirect rule needs confirmation",
					Hint:    "Re-run with --yes to remove it without a prompt.",
				}
			}

			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}
			scope := redirectScope(target)

			// Confirm the rule exists, and name it in the prompt and output.
			rules, err := client.ListRedirectRules(ctx, scope)
			if err != nil {
				return explainRedirectsError(err, target)
			}
			rule := findRedirectRule(rules.Rules, args[0])
			if rule == nil {
				return redirectRuleNotFoundError(args[0], target, rules.Rules)
			}
			summary := redirectRuleSummary(rule.Source, rule.Query, rule.Destination, rule.Status)

			if !yes {
				fmt.Fprintf(env.Stderr, "Remove redirect %s? [y/N]: ", summary) //nolint:errcheck
				answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					return &output.UserError{Message: "Redirect rule removal cancelled"}
				}
			}

			if err := client.DeleteRedirectRule(ctx, scope, rule.Identifier); err != nil {
				return explainRedirectsError(err, target)
			}

			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"identifier": rule.Identifier, "status": "removed"},
					"Removed redirect "+summary,
					output.Breadcrumb{Action: "list", Cmd: redirectsListCmd(target)},
				))
			}
			env.Status("Removed redirect %s", summary)
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Remove without asking for confirmation (required in non-interactive mode)")
	return cmd
}

func redirectRuleNotFoundError(identifier string, target domainsTarget, rules []sdk.RedirectRule) error {
	hint := "This server has no redirect rules managed in DeployHQ. See them with: " + redirectsListCmd(target)
	if len(rules) > 0 {
		var names []string
		for _, r := range rules {
			names = append(names, fmt.Sprintf("%s (%s)", r.Identifier, r.Source))
		}
		hint = "Redirect rules on this server: " + strings.Join(names, ", ")
	}
	return &output.UserError{
		Message:  fmt.Sprintf("No redirect rule %q on this server", identifier),
		Hint:     hint,
		Recovery: []output.Breadcrumb{{Action: "list_redirects", Cmd: redirectsListCmd(target) + " --json"}},
	}
}

func newRedirectsReorderCmd() *cobra.Command {
	var flags domainsServerFlags
	cmd := &cobra.Command{
		Use:   "reorder <identifier>...",
		Short: "Change the order redirect rules are checked in",
		Long: `Put the redirect rules in the order given: the first identifier is checked
first. List every rule of the site exactly once; any other list is refused and
nothing moves. To move a single rule, use "update <identifier> --position N".
The first rule that matches a URL applies, so put specific rules before general
ones (e.g. /blog/archive before /blog/*). Find identifiers with
"dhq servers redirects list".`,
		Example: `  dhq servers redirects reorder rr-3 rr-1 rr-2 -p my-site -s "My Site"`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cliCtx.Background()
			client, target, err := openDomainsTarget(ctx, flags)
			if err != nil {
				return err
			}
			rules, err := client.ReorderRedirectRules(ctx, redirectScope(target), args)
			if err != nil {
				return explainRedirectsReorderError(err, target)
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(rules,
					fmt.Sprintf("Reordered %d redirect rules", len(rules)),
					output.Breadcrumb{Action: "list", Cmd: redirectsListCmd(target)},
				))
			}
			env.Status("Redirect rules are now checked in this order:")
			writeRedirectRulesTable(env, rules)
			warnShadowed(env, rules)
			return nil
		},
	}
	addDomainsServerFlags(cmd, &flags)
	return cmd
}
