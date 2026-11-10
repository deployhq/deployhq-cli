package commands

// launch_domain.go adds a Static Hosting custom domain to `dhq launch`.
//
// The domain is requested with the server create. A hostname the API refuses
// fails the launch before anything is provisioned; every other problem with the
// domain is reported in the successful result, because by then the site is live
// and the launch has done its job.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/detect"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/manifoldco/promptui"
)

// The values of launchCustomDomainResult.Status.
const (
	launchDomainDNSRequired = "dns_records_required"
	launchDomainRegistering = "registering"
	launchDomainActive      = "active"
	launchDomainFailed      = "failed"
	launchDomainExpired     = "expired"
	launchDomainNotAdded    = "not_added"
)

// launchCustomDomainResult is the custom domain part of launch's JSON result.
type launchCustomDomainResult struct {
	Hostname string `json:"hostname"`
	// WithCounterpart reports whether the other half of the apex and www pair
	// was asked for: true unless --no-pair.
	WithCounterpart bool `json:"with_counterpart"`
	// Added reports whether the domain is on the site at all. Status says
	// where it has got to.
	Added  bool   `json:"added"`
	Status string `json:"status"`
	// Domains holds the domain and, when asked for, the other half of its pair,
	// shaped as in dhq domains show; dns_records are what the person publishes.
	Domains []sdk.CustomDomain `json:"domains"`
	// Errors says why something could not be done. An entry does not mean the
	// launch failed: the site is live whatever Status says.
	Errors    []string `json:"errors,omitempty"`
	Retryable bool     `json:"retryable"`
	NextStep  string   `json:"next_step"`
}

// validateLaunchCustomDomain checks the custom domain flags before anything
// else happens, so a bad combination costs no API call.
func validateLaunchCustomDomain(cfg *launchConfig) error {
	if cfg.customDomain == "" {
		if cfg.noPair {
			return &output.UserError{
				Message: "--no-pair needs --custom-domain",
				Hint:    "Name the domain too, e.g. dhq launch --custom-domain example.com --no-pair",
			}
		}
		return nil
	}
	hostname, err := parseDomainHostname(cfg.customDomain)
	if err != nil {
		return err
	}
	cfg.customDomain = hostname
	return refuseCustomDomainTarget(cfg.targetProtocol)
}

// resolveLaunchCustomDomainTarget runs once detection has had its say: a custom
// domain settles an undecided target (only Static Hosting has them), and a
// Managed VPS picked by detection or a previous launch is refused like --vps.
func resolveLaunchCustomDomainTarget(cfg *launchConfig) error {
	if cfg.customDomain == "" {
		return nil
	}
	if cfg.targetProtocol == "" {
		cfg.targetProtocol = detect.ProtocolStaticHosting
		return nil
	}
	return refuseCustomDomainTarget(cfg.targetProtocol)
}

func refuseCustomDomainTarget(target string) error {
	if target == "" || target == detect.ProtocolStaticHosting {
		return nil
	}
	return &output.UserError{
		Message: "--custom-domain only applies to Static Hosting, not " + hostingName(target),
		Hint:    "Launch without --custom-domain, or choose Static Hosting with --static.",
	}
}

// customDomainInvalidError is the structured refusal of a hostname the API
// rejected. reasons are the API's own words for why.
func customDomainInvalidError(hostname string, reasons []string) *launchError {
	return &launchError{
		Reason:   reasonCustomDomainInvalid,
		Message:  fmt.Sprintf("DeployHQ refused the custom domain %q: %s. No server was created.", hostname, strings.Join(reasons, "; ")),
		NextStep: "Pass a different hostname with --custom-domain, or launch without it and add the domain later with dhq domains add.",
		Details:  map[string]string{"custom_domain": hostname},
	}
}

// customDomainRefusalReasons picks the API's messages about the custom domain
// out of a 422, without the "base" field name its validation errors carry. It
// returns nil when none of them is about the domain, and is checked before the
// subdomain match: a hostname message can itself say "subdomains".
func customDomainRefusalReasons(apiErr *sdk.APIError) []string {
	var reasons []string
	for _, msg := range apiErr.Errors {
		msg = strings.TrimPrefix(msg, "base ")
		if strings.Contains(strings.ToLower(msg), "custom domain") {
			reasons = append(reasons, msg)
		}
	}
	return reasons
}

// launchDomainsTarget names server the way the dhq domains commands do.
func launchDomainsTarget(cfg launchConfig, server *sdk.Server) domainsTarget {
	return domainsTarget{
		scope:     sdk.ProjectServerDomains(cfg.projectID, server.Identifier),
		projectID: cfg.projectID,
		server:    server.Identifier,
	}
}

func launchDomainAddCmd(cfg launchConfig, target domainsTarget) string {
	return domainsAddCmd(cfg.customDomain, target, cfg.noPair)
}

// domainCounterpart is the other half of a hostname's apex and www pair, or ""
// if it has none we can tell (see isApexHostname).
func domainCounterpart(hostname string) string {
	if strings.HasPrefix(hostname, "www.") && isApexHostname(strings.TrimPrefix(hostname, "www.")) {
		return strings.TrimPrefix(hostname, "www.")
	}
	if isApexHostname(hostname) {
		return "www." + hostname
	}
	return ""
}

// otherWWWForm flips hostname between its www and non-www forms. Matching that
// against the domains a site really has finds its pair without having to guess
// where the registrable domain starts (example.co.uk).
func otherWWWForm(hostname string) string {
	if strings.HasPrefix(hostname, "www.") {
		return strings.TrimPrefix(hostname, "www.")
	}
	return "www." + hostname
}

// launchPromptCustomDomain offers a custom domain, once, to a person at a
// terminal who did not pass --custom-domain. A blank answer is no domain. It is
// never called in non-interactive mode, so it cannot hold up an agent.
func launchPromptCustomDomain(env *output.Envelope, cfg *launchConfig) {
	if env.NonInteractive || cfg.customDomain != "" || cfg.customDomainPrompted {
		return
	}
	cfg.customDomainPrompted = true
	for {
		answer, err := (&promptui.Prompt{Label: "Custom domain (optional, press Enter to skip)"}).Run()
		answer = strings.TrimSpace(answer)
		if err != nil || answer == "" {
			return
		}
		hostname, parseErr := parseDomainHostname(answer)
		if parseErr != nil {
			env.Warn("%v", parseErr)
			continue
		}
		cfg.customDomain = hostname
		break
	}
	counterpart := domainCounterpart(cfg.customDomain)
	if counterpart == "" {
		return
	}
	prompt := promptui.Select{
		Label: fmt.Sprintf("Also add %s, which redirects to %s?", counterpart, cfg.customDomain),
		Items: []string{"Yes", "No"},
	}
	if idx, _, err := prompt.Run(); err == nil {
		cfg.noPair = idx != 0
	}
}

// launchEnsureCustomDomainOnServer adds the requested domain to a server that
// already existed (a re-run of launch), recording the outcome on server the way
// a create response would. A domain already there is left alone, so re-running
// launch is safe. Only a refused hostname is an error.
func launchEnsureCustomDomainOnServer(ctx context.Context, env *output.Envelope, cfg *launchConfig, client *sdk.Client, server *sdk.Server) error {
	if cfg.customDomain == "" {
		return nil
	}
	server.CustomDomains = []sdk.CustomDomain{}
	server.CustomDomainErrors = []string{}
	target := launchDomainsTarget(*cfg, server)

	listed, err := client.ListCustomDomains(ctx, target.scope)
	if err != nil {
		server.CustomDomainErrors = []string{explainDomainsError(err, target).Error()}
		return nil
	}
	if d := findDomain(listed, cfg.customDomain); d != nil {
		env.Status("Custom domain %s is already on this site.", d.Hostname)
		server.CustomDomains = []sdk.CustomDomain{*d}
		if o := findDomain(listed, otherWWWForm(d.Hostname)); o != nil {
			server.CustomDomains = append(server.CustomDomains, *o)
		}
		return nil
	}

	created, err := client.CreateCustomDomain(ctx, target.scope, sdk.CustomDomainCreateRequest{Hostname: cfg.customDomain, WithCounterpart: !cfg.noPair})
	if err != nil {
		var apiErr *sdk.APIError
		if errors.As(err, &apiErr) && apiErr.IsValidationError() {
			reasons := make([]string, 0, len(apiErr.Errors))
			for _, msg := range apiErr.Errors {
				reasons = append(reasons, strings.TrimPrefix(msg, "base "))
			}
			if len(reasons) == 0 {
				reasons = []string{apiErr.Error()}
			}
			return customDomainInvalidError(cfg.customDomain, reasons)
		}
		server.CustomDomainErrors = []string{explainDomainsError(err, target).Error()}
		return nil
	}
	server.CustomDomains = []sdk.CustomDomain{created.CustomDomain}
	if created.Counterpart != nil {
		server.CustomDomains = append(server.CustomDomains, *created.Counterpart)
	}
	server.CustomDomainErrors = append(server.CustomDomainErrors, created.CounterpartErrors...)
	return nil
}

// launchFinishCustomDomain turns the outcome recorded on server into the result
// to report, waiting (bounded) for a domain still being registered. It never
// fails the launch.
func launchFinishCustomDomain(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, server *sdk.Server, account, liveURL string) *launchCustomDomainResult {
	if cfg.customDomain == "" {
		return nil
	}
	target := launchDomainsTarget(cfg, server)
	result := &launchCustomDomainResult{
		Hostname:        cfg.customDomain,
		WithCounterpart: !cfg.noPair,
		Domains:         []sdk.CustomDomain{},
		Errors:          append([]string{}, server.CustomDomainErrors...),
	}

	if server.CustomDomains == nil && server.CustomDomainErrors == nil {
		result.Errors = []string{"This DeployHQ server did not report the custom domain, so it was not added."}
	}
	if len(server.CustomDomains) == 0 {
		result.Status = launchDomainNotAdded
		result.Retryable, result.NextStep = launchDomainRefusalNextStep(cfg, target, result.Errors, account, liveURL)
		return result
	}

	created := &sdk.CustomDomainCreation{CustomDomain: server.CustomDomains[0]}
	if len(server.CustomDomains) > 1 {
		created.Counterpart = &server.CustomDomains[1]
	}
	domains := addedDomains(created)
	timedOut, waitErr := waitForDomainRegistration(ctx, client, target, env, domains)
	if waitErr != nil {
		result.Errors = append(result.Errors, waitErr.Error())
	}
	for _, d := range domains {
		result.Domains = append(result.Domains, *d)
	}
	result.Added = true

	showCmd := domainsShowCmd(cfg.customDomain, target)
	failed := failedDomains(created)
	expired := expiredDomains(domains)
	// Starting both halves over: re-adding one must not recreate the other.
	retryNoPair := cfg.noPair || len(failed)+len(expired) > 1
	switch {
	case len(failed) > 0:
		failure := domainsFailedError(failed, created, target, retryNoPair)
		var ue *output.UserError
		if errors.As(failure, &ue) {
			result.Errors = append(result.Errors, ue.Message)
			result.NextStep = ue.Hint + ". The site itself is live at " + liveURL + "."
		}
		result.Status = launchDomainFailed
		if len(expired) > 0 {
			problem, fix := expiredDomainsAdvice(expired, target, retryNoPair)
			result.Errors = append(result.Errors, problem)
			result.NextStep += " " + fix
		}
	case len(expired) > 0:
		problem, fix := expiredDomainsAdvice(expired, target, retryNoPair)
		result.Status = launchDomainExpired
		result.Errors = append(result.Errors, problem)
		result.NextStep = "The site is live at " + liveURL + ". " + fix
	case timedOut || waitErr != nil || anyRegistering(domains):
		result.Status = launchDomainRegistering
		result.NextStep = fmt.Sprintf("The site is live at %s. The domain is still being set up; get its DNS records, including the TXT record, with: %s", liveURL, showCmd)
	case allDomainsActive(domains):
		result.Status = launchDomainActive
		result.NextStep = fmt.Sprintf("The domain is live. Check it with: %s", showCmd)
	default:
		result.Status = launchDomainDNSRequired
		result.NextStep = fmt.Sprintf("The site is live at %s. Publish both the CNAME and TXT records in dns_records at the DNS provider for each domain; "+
			"the domain goes live automatically once both are detected, usually within minutes, and expires if it is not verified within 7 days. Check progress with: %s",
			liveURL, showCmd)
	}

	return result
}

func expiredDomains(domains []*sdk.CustomDomain) []*sdk.CustomDomain {
	var expired []*sdk.CustomDomain
	for _, d := range domains {
		if d.Status == launchDomainExpired {
			expired = append(expired, d)
		}
	}
	return expired
}

// expiredDomainsAdvice says what is wrong with domains that expired and the
// commands that start them over. Publishing DNS records cannot revive one.
func expiredDomainsAdvice(expired []*sdk.CustomDomain, target domainsTarget, noPair bool) (problem, fix string) {
	names := make([]string, 0, len(expired))
	retry := make([]string, 0, len(expired))
	for _, d := range expired {
		names = append(names, d.Hostname)
		retry = append(retry, domainsRetryCmds(d.Hostname, target, noPair))
	}
	problem = strings.Join(names, " and ") + " expired before it was verified."
	fix = "Remove it and add it again to get new DNS records: " + strings.Join(retry, "; ") + "."
	return problem, fix
}

func allDomainsActive(domains []*sdk.CustomDomain) bool {
	for _, d := range domains {
		if d.Status != "active" {
			return false
		}
	}
	return true
}

// launchDomainRefusalNextStep picks what to do about a domain that was not
// added, from the API's reasons. It always says the site is live, and always
// names the command that adds the domain once the blocker is gone.
func launchDomainRefusalNextStep(cfg launchConfig, target domainsTarget, reasons []string, account, liveURL string) (retryable bool, nextStep string) {
	joined := strings.ToLower(strings.Join(reasons, " "))
	addCmd := launchDomainAddCmd(cfg, target)
	live := "The site is live at " + liveURL + ". "
	switch {
	case strings.Contains(joined, "paid plan"):
		return false, live + "Custom domains need a paid plan: upgrade at " + accountPageURL(account, nil, "/account/packages") + ", then run: " + addCmd
	case strings.Contains(joined, "too many requests"):
		return true, live + "Wait a while, then run: " + addCmd
	default:
		return false, live + "Add the domain with: " + addCmd
	}
}

// writeLaunchResult prints the outcome of a successful launch.
func writeLaunchResult(env *output.Envelope, cfg launchConfig, server *sdk.Server, dep *sdk.Deployment, liveURL string, domain *launchCustomDomainResult) error {
	target := launchDomainsTarget(cfg, server)
	if env.WantsJSON() {
		result := launchResult{
			Status:       "live",
			Target:       cfg.targetProtocol,
			URL:          liveURL,
			Project:      cfg.projectID,
			Server:       server.Identifier,
			CustomDomain: domain,
		}
		if dep != nil {
			result.Deployment = dep.Identifier
		}
		summary := "Deployment live: " + liveURL
		var crumbs []output.Breadcrumb
		if domain != nil {
			summary = launchDomainSummary(summary, domain)
			crumbs = launchDomainBreadcrumbs(cfg, target, domain)
		}
		return env.WriteJSON(output.NewResponse(result, summary, crumbs...))
	}

	env.Status("")
	output.ColorGreen.Fprintf(env.Stderr, "Live: %s\n", liveURL) //nolint:errcheck
	env.Status("Saved settings to .deployhq.toml — redeploy with 'dhq deploy -s %s'.", server.Identifier)
	env.Status("Roll back anytime with 'dhq rollback <deployment>' — redeploys the previous revision ('dhq deployments list' shows history).")
	if domain != nil {
		writeLaunchDomainReport(env, target, domain)
	}
	// Final stdout line = live URL (Vercel pattern, scriptable)
	fmt.Fprintln(env.Stdout, liveURL) //nolint:errcheck
	return nil
}

func launchDomainSummary(base string, domain *launchCustomDomainResult) string {
	switch domain.Status {
	case launchDomainDNSRequired:
		return fmt.Sprintf("%s. Custom domain %s is waiting for its DNS records: see custom_domain.domains[].dns_records", base, domain.Hostname)
	case launchDomainRegistering:
		return fmt.Sprintf("%s. Custom domain %s is still being set up", base, domain.Hostname)
	case launchDomainFailed:
		return fmt.Sprintf("%s. Custom domain %s could not be set up: see custom_domain.errors", base, domain.Hostname)
	case launchDomainExpired:
		return fmt.Sprintf("%s. Custom domain %s expired and must be removed and added again: see custom_domain.next_step", base, domain.Hostname)
	case launchDomainNotAdded:
		return fmt.Sprintf("%s. Custom domain %s was not added: see custom_domain.errors", base, domain.Hostname)
	}
	return base
}

func launchDomainBreadcrumbs(cfg launchConfig, target domainsTarget, domain *launchCustomDomainResult) []output.Breadcrumb {
	if domain.Status == launchDomainNotAdded {
		return []output.Breadcrumb{{Action: "add_custom_domain", Cmd: launchDomainAddCmd(cfg, target) + " --json"}}
	}
	if domain.Status == launchDomainExpired {
		var expired []*sdk.CustomDomain
		for i := range domain.Domains {
			if domain.Domains[i].Status == launchDomainExpired {
				expired = append(expired, &domain.Domains[i])
			}
		}
		var crumbs []output.Breadcrumb
		for _, d := range expired {
			crumbs = append(crumbs,
				output.Breadcrumb{Action: "remove_custom_domain", Cmd: domainsRemoveCmd(d.Hostname, target) + " --json"},
				output.Breadcrumb{Action: "add_custom_domain", Cmd: domainsAddCmd(d.Hostname, target, cfg.noPair || len(expired) > 1) + " --json"},
			)
		}
		return crumbs
	}
	showCmd := domainsShowCmd(domain.Hostname, target)
	crumbs := []output.Breadcrumb{{Action: "show_custom_domain", Cmd: showCmd + " --json"}}
	if domain.Status == launchDomainDNSRequired {
		crumbs = append(crumbs, output.Breadcrumb{Action: "check_custom_domain", Cmd: fmt.Sprintf("dhq domains check %s %s --json", domain.Hostname, target.flags())})
	}
	return crumbs
}

// writeLaunchDomainReport tells a person at a terminal what became of the
// domain. Everything goes to stderr: launch's stdout is only the live URL.
func writeLaunchDomainReport(env *output.Envelope, target domainsTarget, domain *launchCustomDomainResult) {
	env.Status("")
	switch domain.Status {
	case launchDomainActive:
		env.Status("Custom domain %s is live.", domain.Hostname)
	case launchDomainFailed:
		env.Warn("Custom domain %s could not be set up: %s", domain.Hostname, strings.Join(domain.Errors, "; "))
		env.Status("Next: %s", domain.NextStep)
	case launchDomainExpired:
		env.Warn("Custom domain %s cannot go live: %s", domain.Hostname, strings.Join(domain.Errors, "; "))
		env.Status("Next: %s", domain.NextStep)
	case launchDomainNotAdded:
		env.Warn("Custom domain %s was not added: %s", domain.Hostname, strings.Join(domain.Errors, "; "))
		env.Status("Next: %s", domain.NextStep)
	default:
		env.Status("Custom domain %s is not live yet: the URL above is the one to use until it is.", domain.Hostname)
		stderrEnv := *env
		stderrEnv.Stdout = env.Stderr
		for i := range domain.Domains {
			d := &domain.Domains[i]
			writeDNSRecords(&stderrEnv, d, domainsShowCmd(d.Hostname, target))
		}
		for _, e := range domain.Errors {
			env.Warn("%s", e)
		}
		env.Status("\nCheck progress with: %s", domainsShowCmd(domain.Hostname, target))
	}
}
