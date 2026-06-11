package commands

// launch.go implements `dhq launch` — the one-command deploy flow for
// Static Hosting and Managed VPS. It is designed non-interactive-first:
// the core logic takes a fully-resolved launchConfig and executes with zero
// prompts; interactive prompts only fill *missing* values when a TTY is present.
//
// Phase 2 + Phase 3 of the one-command deploy plan.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/deployhq/deployhq-cli/internal/auth"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/detect"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// launchConfig holds the fully-resolved inputs for the launch flow.
// It is populated from flags → env → .deployhq.toml → detection defaults
// before any prompts or API calls happen.
type launchConfig struct {
	// Target selection
	targetProtocol string // "static_hosting", "managed_vps", or ""

	// Project
	projectID string // permalink / identifier

	// Persisted server identifier from a previous launch (enables idempotency).
	// When non-empty, re-runs skip provisioning and go straight to deploy.
	serverID string

	// Static Hosting params
	subdomain    string
	subdirectory string
	spaMode      bool

	// VPS params
	region  string
	size    string
	osImage string

	// Deploy params
	branch string

	// Behavioural flags
	acceptCost       bool
	cleanupOnFailure bool
	dryRun           bool
}

// launchResult is the machine-readable output for --json mode.
type launchResult struct {
	Status     string `json:"status"`
	Target     string `json:"target"`
	URL        string `json:"url,omitempty"`
	Project    string `json:"project"`
	Server     string `json:"server"`
	Deployment string `json:"deployment,omitempty"`
}

// dryRunResult is the --dry-run output (no side effects).
type dryRunResult struct {
	Would    dryRunWould `json:"would"`
	Requires []string    `json:"requires,omitempty"`
	Warning  string      `json:"warning,omitempty"`
}

type dryRunWould struct {
	Provision   string  `json:"provision,omitempty"`
	Region      string  `json:"region,omitempty"`
	Size        string  `json:"size,omitempty"`
	MonthlyCost string  `json:"monthly_cost,omitempty"`
	Subdomain   string  `json:"subdomain,omitempty"`
	Project     string  `json:"project,omitempty"`
	Branch      string  `json:"branch,omitempty"`
}

// launchErrorReason is the machine-readable error code taxonomy.
const (
	reasonAuthRequired       = "auth_required"
	reasonBetaEnrollRequired = "beta_enroll_required"
	reasonAcceptCostRequired = "accept_cost_required"
	reasonRepoUnreachable    = "repo_unreachable"
	reasonPlanLimitReached   = "plan_limit_reached"
	reasonSubdomainTaken     = "subdomain_taken"
	reasonRateLimited        = "rate_limited"
	reasonProvisionFailed    = "provision_failed"
	reasonDeployFailed       = "deploy_failed"
)

// launchError is a structured error that carries machine-readable next-step
// info for --json mode alongside the human-readable message + hint.
type launchError struct {
	Reason   string
	Message  string
	NextStep string
	Details  map[string]string
	// Retryable marks an error an agent/CI can safely re-attempt after backing
	// off (e.g. a 429 provisioning rate limit), as opposed to a hard wall like
	// plan_limit_reached. Surfaced as `retryable` in --json output.
	Retryable bool
}

func (e *launchError) Error() string {
	msg := e.Message
	if e.NextStep != "" {
		msg += "\n\nNext step: " + e.NextStep
	}
	return msg
}

// rateLimitLaunchError converts a 429 provisioning-rate-limit API error into a
// structured, retryable launchError carrying the Retry-After backoff hint. It
// returns nil when err is not a 429, so callers can fall through to their
// existing error handling.
func rateLimitLaunchError(err error) *launchError {
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) || !apiErr.IsRateLimited() {
		return nil
	}
	details := map[string]string{}
	nextStep := "Wait a moment, then re-run the same command — this is a temporary provisioning rate limit, not a hard cap."
	if apiErr.RetryAfter > 0 {
		details["retry_after"] = strconv.Itoa(apiErr.RetryAfter)
		nextStep = fmt.Sprintf("Wait %ds, then re-run the same command (provisioning rate limit; Retry-After: %ds).", apiErr.RetryAfter, apiErr.RetryAfter)
	}
	return &launchError{
		Reason:    reasonRateLimited,
		Message:   "Provisioning rate limit reached for this account",
		NextStep:  nextStep,
		Details:   details,
		Retryable: true,
	}
}

// newLaunchCmd builds the `dhq launch` Cobra command.
func newLaunchCmd() *cobra.Command {
	var (
		flagStatic          bool
		flagVPS             bool
		flagAcceptCost      bool
		flagSubdomain       string
		flagRegion          string
		flagSize            string
		flagBranch          string
		flagProject         string
		flagCleanupOnFail   bool
		flagNonInteract     bool // local --non-interactive (mirrors global but scoped)
		flagInteractive     bool
		flagDryRun          bool
	)

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "One-command deploy to Static Hosting or Managed VPS",
		Long: `Provision and deploy your project to DeployHQ's managed infrastructure in one step.

dhq launch detects your framework, provisions the right target (Static Hosting or
Managed VPS), connects your repository, and deploys — printing a live URL when done.

The command is fully non-interactive: pass flags or environment variables to drive
it from CI / AI agents. Interactive prompts only fill in missing values when a TTY
is present.

Equivalent of 'netlify deploy' / 'vercel' / 'fly launch' for DeployHQ's own infra.`,
		Example: `  # Interactive: auto-detect framework and prompt for anything missing
  dhq launch

  # CI — static is safe under --yes; a Managed VPS REQUIRES --accept-cost
  dhq launch --static --subdomain my-app
  dhq launch --vps --accept-cost --region lon1

  # Agent — structured output (no side effects; inspect before running)
  dhq launch --vps --dry-run --json
  dhq launch --json --static --subdomain my-app

  # Opt out to own-server setup
  # At the target prompt, choose "Use my own server" → branches into dhq init`,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			// Local --non-interactive flag merges with the global one.
			if flagNonInteract {
				env.NonInteractive = true
			}
			// --interactive explicitly re-enables prompts even in piped mode.
			if flagInteractive {
				env.NonInteractive = false
			}

			// Resolve launchConfig from flags → env → .deployhq.toml → detection
			cfg := resolveLaunchConfig(flagStatic, flagVPS, flagSubdomain, flagRegion, flagSize, flagBranch, flagProject, flagAcceptCost, flagCleanupOnFail, flagDryRun)

			return runLaunch(env, cfg)
		},
	}

	cmd.Flags().BoolVar(&flagStatic, "static", false, "Force Static Hosting target")
	cmd.Flags().BoolVar(&flagVPS, "vps", false, "Force Managed VPS target")
	cmd.Flags().BoolVar(&flagAcceptCost, "accept-cost", false, "Acknowledge Managed VPS provisioning — "+managedVPSAcknowledgePhrase()+" (required to provision a VPS non-interactively)")
	cmd.Flags().StringVar(&flagSubdomain, "subdomain", "", "Static Hosting subdomain (default: repo / project name)")
	cmd.Flags().StringVar(&flagRegion, "region", "", "Managed VPS region slug (e.g. lon1, nyc3)")
	cmd.Flags().StringVar(&flagSize, "size", "", "Managed VPS size slug (e.g. s-1vcpu-1gb)")
	cmd.Flags().StringVar(&flagBranch, "branch", "", "Branch to deploy (default: repo default)")
	cmd.Flags().StringVar(&flagProject, "project", "", "Existing project permalink to reuse (skips project creation)")
	cmd.Flags().BoolVar(&flagCleanupOnFail, "cleanup-on-failure", false, "Delete the provisioned server when the deploy fails (prevents orphaned managed resources)")
	cmd.Flags().BoolVar(&flagNonInteract, "non-interactive", false, "Never prompt; fail fast with structured errors on ambiguity (alias: --yes)")
	cmd.Flags().BoolVar(&flagNonInteract, "yes", false, "Alias for --non-interactive")
	cmd.Flags().BoolVar(&flagInteractive, "interactive", false, "Force interactive mode even in piped / agent contexts")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print intended actions and cost without executing (no side effects)")

	return cmd
}

// resolveLaunchConfig builds a launchConfig from flag values, falling through
// to .deployhq.toml (already loaded in cliCtx.Config) and detection defaults.
func resolveLaunchConfig(
	flagStatic, flagVPS bool,
	flagSubdomain, flagRegion, flagSize, flagBranch, flagProject string,
	flagAcceptCost, flagCleanupOnFail, flagDryRun bool,
) launchConfig {
	cfg := launchConfig{
		acceptCost:       flagAcceptCost,
		cleanupOnFailure: flagCleanupOnFail,
		dryRun:           flagDryRun,
	}

	// Target protocol: flag wins, then nothing (resolved later via detection/prompt)
	if flagStatic {
		cfg.targetProtocol = detect.ProtocolStaticHosting
	} else if flagVPS {
		cfg.targetProtocol = detect.ProtocolManagedVPS
	}

	// Project: flag > env > .deployhq.toml (cliCtx.Config already merged those layers)
	cfg.projectID = flagProject
	if cfg.projectID == "" && cliCtx != nil {
		cfg.projectID = cliCtx.Config.Project
	}

	// Server / target: read from .deployhq.toml so re-runs can skip provisioning.
	if cliCtx != nil {
		if cfg.serverID == "" {
			cfg.serverID = cliCtx.Config.Server
		}
		// Only fall back to the persisted target when no flag was given.
		if cfg.targetProtocol == "" {
			cfg.targetProtocol = cliCtx.Config.Target
		}
	}

	cfg.subdomain = flagSubdomain
	cfg.region = flagRegion
	cfg.size = flagSize
	cfg.branch = flagBranch

	return cfg
}

// runLaunch executes the full launch flow. It is broken out from the cobra
// RunE so tests can call it directly with a pre-built Envelope.
func runLaunch(env *output.Envelope, cfg launchConfig) error {
	ctx := context.Background()

	env.Status("")
	env.Status("DeployHQ • one-command deploy")
	env.Status("")

	// ── Step 1: Auth / bootstrap ────────────────────────────────────────────
	client, accountSubdomain, err := launchEnsureAuth(ctx, env, cfg)
	if err != nil {
		return writeLaunchError(env, cfg, reasonAuthRequired, err)
	}

	// ── Step 2: Detect ──────────────────────────────────────────────────────
	// Prefer backend detection (same StackDetector pipeline as the web
	// onboarding wizard) so the CLI's recommendation stays in lockstep with the
	// server. Fall back to the local heuristic when the endpoint is
	// unavailable (older backend, offline, transient error).
	cwd, _ := os.Getwd()
	detection := launchDetect(ctx, env, client, cwd)
	if detection.Framework != detect.FrameworkUnknown && detection.Framework != "" {
		env.Status("Detected: %s", string(detection.Framework))
	} else {
		env.Status("No framework detected — will prompt for target.")
	}
	if detection.SuggestedProtocol != "" && cfg.targetProtocol == "" {
		cfg.targetProtocol = detection.SuggestedProtocol
	}

	// ── Dry-run exit: before any mutation ─────────────────────────────
	// Read-only: caps and region/size listing for cost estimates are allowed,
	// but beta enrollment, project creation, and provisioning must NOT happen.
	if cfg.dryRun {
		// Fetch caps for the dry-run cost/warning display (read-only).
		caps, capsErr := client.GetAccountCapabilities(ctx)
		if capsErr != nil {
			var apiErr *sdk.APIError
			if errors.As(capsErr, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				caps = &sdk.AccountCapabilities{}
			} else {
				caps = &sdk.AccountCapabilities{}
			}
		}
		return launchDryRun(ctx, env, cfg, client, caps)
	}

	// ── Step 3: Capability pre-flight ───────────────────────────────────────
	// capsKnown tracks whether the backend returned capability data. When false
	// (404 = older backend), the plan-limit gate is skipped and CreateServer
	// acts as the authority.
	caps, capsKnown, err := launchGetCaps(ctx, env, client)
	if err != nil {
		return err
	}

	// ── Step 4: Repo deployability pre-flight ───────────────────────────────
	gitRemote := detectGitRemote()
	if gitRemote == "" {
		return writeLaunchError(env, cfg, reasonRepoUnreachable, &output.UserError{
			Message: "No git remote found in this directory",
			Hint:    "Static Hosting and Managed VPS deployments require a connected git repository.\nRun: git remote add origin <url>\nThen re-run dhq launch.",
		})
	}
	env.Status("Repository: %s", gitRemote)

	// ── Step 5: Target selection ─────────────────────────────────────────────
	if cfg.targetProtocol == "" {
		selected, err := launchPromptTarget(env)
		if err != nil {
			return err
		}
		if selected == "own_server" {
			env.Status("")
			env.Status("Redirecting to guided setup for your own server...")
			env.Status("Run: dhq init")
			return nil
		}
		cfg.targetProtocol = selected
	}
	env.Status("Target: %s", cfg.targetProtocol)

	// ── Step 6: Beta enrollment (after target is known) ─────────────
	// isManagedTarget is now evaluated with the final resolved target so that
	// interactive target selection is accounted for before we attempt enrollment.
	isManagedTarget := cfg.targetProtocol == detect.ProtocolStaticHosting || cfg.targetProtocol == detect.ProtocolManagedVPS
	if isManagedTarget && capsKnown && !caps.BetaFeatures {
		if err := launchEnsureBetaEnrolled(ctx, env, cfg, client, accountSubdomain); err != nil {
			return err
		}
		// Re-read caps after enrollment
		if caps, _, err = launchGetCaps(ctx, env, client); err != nil {
			caps = &sdk.AccountCapabilities{}
		}
	}

	// ── Step 7: Project + repo ───────────────────────────────────────────────
	projectID, err := launchEnsureProject(ctx, env, cfg, client, gitRemote)
	if err != nil {
		return err
	}
	cfg.projectID = projectID

	// ── Apply detection defaults for static hosting ──────────────────
	// Seed subdirectory and SPA mode from detection when flags were not set.
	if cfg.targetProtocol == detect.ProtocolStaticHosting {
		if cfg.subdirectory == "" && detection.OutputDir != "" {
			cfg.subdirectory = detection.OutputDir
		}
		// Only override spaMode from detection when it hasn't been set by a flag.
		// Since spaMode is a bool it defaults to false; we apply detection.SPA
		// when detection actually had an opinion (SPA==true) and the user didn't
		// explicitly prompt otherwise. False detection.SPA leaves cfg.spaMode alone.
		if detection.SPA && !cfg.spaMode {
			cfg.spaMode = true
		}
	}

	// ── Step 8: Plan / limit pre-flight ─────────────────────────────────────
	// Only apply eligibility gates when we have real capability data.
	if capsKnown {
		if err := launchCheckPlanLimits(env, cfg, caps); err != nil {
			return err
		}
	}

	// ── Step 9: Provision server — idempotency check ─────────────────
	// If a server identifier was persisted from a previous run, verify it still
	// exists. If it does, skip provisioning and go straight to deploy.
	var server *sdk.Server
	if cfg.serverID != "" {
		existing, pollErr := client.GetServerProvisioningState(ctx, cfg.projectID, cfg.serverID)
		if pollErr == nil {
			env.Status("Found existing server %s — skipping provisioning.", cfg.serverID)
			server = existing
		} else {
			// Server no longer exists (404) or there's an error — fall through to provision.
			env.Status("Persisted server %s not found (%v) — provisioning new server.", cfg.serverID, pollErr)
		}
	}

	if server == nil {
		var provErr error
		server, provErr = launchProvision(ctx, env, cfg, client)
		if provErr != nil {
			// Provision failure cleanup: run the same cleanup path as deploy failures
			// so --cleanup-on-failure and resource naming apply.
			if server != nil {
				launchDeployFailureCleanup(ctx, env, cfg, client, server)
			}
			return writeLaunchError(env, cfg, reasonProvisionFailed, provErr)
		}
		// Persist the new server so re-runs are idempotent.
		launchPersistConfig(env, cfg, server)
		cfg.serverID = server.Identifier
	}

	// ── Step 10: Build command (static only) ──────────────────────────────────
	if cfg.targetProtocol == detect.ProtocolStaticHosting && detection.BuildCommand != "" {
		launchApplyBuildCommand(ctx, env, cfg, client, detection)
	}

	// ── Step 11: Deploy ───────────────────────────────────────────────────────
	dep, liveURL, err := launchDeploy(ctx, env, cfg, client, server)
	if err != nil {
		// Provision succeeded but deploy failed — name the resource
		if server != nil {
			launchDeployFailureCleanup(ctx, env, cfg, client, server)
		}
		return writeLaunchError(env, cfg, reasonDeployFailed, err)
	}

	// ── Step 12: Persist to .deployhq.toml ───────────────────────────────────
	launchPersistConfig(env, cfg, server)

	// ── Final output ──────────────────────────────────────────────────────────
	if env.WantsJSON() {
		result := launchResult{
			Status:     "live",
			Target:     cfg.targetProtocol,
			URL:        liveURL,
			Project:    cfg.projectID,
			Server:     server.Identifier,
			Deployment: "",
		}
		if dep != nil {
			result.Deployment = dep.Identifier
		}
		return env.WriteJSON(output.NewResponse(result, "Deployment live: "+liveURL))
	}

	env.Status("")
	output.ColorGreen.Fprintf(env.Stderr, "Live: %s\n", liveURL) //nolint:errcheck
	env.Status("Saved settings to .deployhq.toml — redeploy with 'dhq deploy -s %s'.", server.Identifier)
	env.Status("Roll back anytime with 'dhq rollback <deployment>' — redeploys the previous revision ('dhq deployments list' shows history).")
	// Final stdout line = live URL (Vercel pattern, scriptable)
	fmt.Fprintln(os.Stdout, liveURL) //nolint:errcheck

	return nil
}

// ── Detection (remote-first, local fallback) ──────────────────────────────────

// launchDetect runs framework detection for dir. It prefers the backend's
// /detection endpoint (the same StackDetector pipeline the web onboarding
// wizard uses, so the CLI's recommendation matches the server), and falls back
// to the local heuristic when the endpoint is unavailable — an older backend
// that 404s, an offline run, or any transient error. The fallback is silent at
// normal verbosity: detection is advisory, and the local result is good.
func launchDetect(ctx context.Context, env *output.Envelope, client *sdk.Client, dir string) detect.Result {
	filenames, files := detect.CollectManifest(dir)
	resp, err := client.DetectFramework(ctx, sdk.DetectionPayload{Filenames: filenames, Files: files})
	if err != nil {
		// Silent to the user (detection is advisory and the local result is
		// good); recorded for debugging.
		env.Logger.Write("remote detection unavailable (%v) — using local detection", err)
		return detect.Detect(dir)
	}
	return detectionResultFromAPI(resp)
}

// detectionResultFromAPI maps a backend /detection response into the Result
// shape the launch flow consumes. Multiple suggested build commands are joined
// into a single shell command (the backend runs build commands as shell steps,
// so "install && build" is equivalent to two ordered commands).
func detectionResultFromAPI(resp *sdk.DetectionResponse) detect.Result {
	cmds := make([]string, 0, len(resp.BuildCommands))
	for _, bc := range resp.BuildCommands {
		if bc.Command != "" {
			cmds = append(cmds, bc.Command)
		}
	}
	return detect.Result{
		Framework:         detect.Framework(resp.Stack),
		SuggestedProtocol: resp.SuggestedProtocol,
		BuildCommand:      strings.Join(cmds, " && "),
		OutputDir:         resp.StaticHosting.RootPath,
		SPA:               resp.StaticHosting.SPAMode,
	}
}

// launchGetCaps fetches account capabilities and reports whether the data is
// authoritative. When the backend 404s (older/staging), capsKnown is false
// and callers must not gate on the returned caps.
func launchGetCaps(ctx context.Context, env *output.Envelope, client *sdk.Client) (caps *sdk.AccountCapabilities, capsKnown bool, err error) {
	caps, err = client.GetAccountCapabilities(ctx)
	if err != nil {
		var apiErr *sdk.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			env.Warn("Capability check unavailable — beta status unknown. Visit https://app.deployhq.com/beta_features to enable.")
			return &sdk.AccountCapabilities{}, false, nil
		}
		return nil, false, err
	}
	return caps, true, nil
}

// ── Auth / bootstrap ────────────────────────────────────────────────────────

// launchEnsureAuth returns an authenticated SDK client and the account subdomain.
// In non-interactive mode it requires env-var creds; it never attempts headless signup.
func launchEnsureAuth(ctx context.Context, env *output.Envelope, cfg launchConfig) (*sdk.Client, string, error) {
	// Fast path: try to build a client from existing creds
	client, err := cliCtx.Client()
	if err == nil {
		account, _, _, _ := cliCtx.Credentials()
		return client, account, nil
	}

	// Non-interactive: fail fast with structured error
	if env.NonInteractive {
		return nil, "", &output.AuthError{
			Message: "Not authenticated",
			Hint: "Set credentials via environment variables:\n" +
				"  export DEPLOYHQ_ACCOUNT=<subdomain> DEPLOYHQ_EMAIL=<email> DEPLOYHQ_API_KEY=<key>\n" +
				"Or log in interactively: dhq auth login",
		}
	}

	// Interactive: offer create-account or log in
	env.Status("No DeployHQ account found on this machine.")
	env.Status("")

	prompt := promptui.Select{
		Label: "Get started",
		Items: []string{"Create a new account (recommended)", "Log in to an existing account"},
	}
	idx, _, promptErr := prompt.Run()
	if promptErr != nil {
		return nil, "", &output.UserError{Message: "Auth cancelled"}
	}

	reader := bufio.NewReader(os.Stdin)
	if idx == 0 {
		return launchSignup(ctx, env, reader)
	}
	return launchLogin(ctx, env, reader)
}

func launchSignup(ctx context.Context, env *output.Envelope, reader *bufio.Reader) (*sdk.Client, string, error) {
	env.Status("")
	fmt.Fprint(env.Stderr, "Email: ") //nolint:errcheck
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, "", &output.UserError{Message: "Email is required"}
	}

	fmt.Fprint(env.Stderr, "Password: ") //nolint:errcheck
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(env.Stderr) //nolint:errcheck
	if err != nil {
		return nil, "", &output.InternalError{Message: "read password", Cause: err}
	}
	password := strings.TrimSpace(string(pw))
	if password == "" {
		return nil, "", &output.UserError{Message: "Password is required"}
	}

	fmt.Fprint(env.Stderr, "Accept terms of service? [Y/n]: ") //nolint:errcheck
	terms, _ := reader.ReadString('\n')
	terms = strings.TrimSpace(strings.ToLower(terms))
	if terms != "" && terms != "y" && terms != "yes" {
		return nil, "", &output.UserError{
			Message: "Terms of service must be accepted to create an account",
			Hint:    "Visit https://www.deployhq.com/terms to review.",
		}
	}

	env.Status("Creating account...")
	ua := cliUserAgent()
	result, err := sdk.Signup(sdk.SignupRequest{
		Email:         email,
		Password:      password,
		Client:        "dhq-cli",
		TermsAccepted: true,
	}, ua, cliCtx.Config.SignupURL())
	if err != nil {
		var twoFA *sdk.TwoFactorError
		if errors.As(err, &twoFA) {
			return nil, "", &output.UserError{
				Message: "Two-factor authentication required",
				Hint:    "This email is linked to an existing account with 2FA. Please sign up or log in at https://www.deployhq.com then run: dhq auth login",
			}
		}
		return nil, "", err
	}

	creds := &auth.Credentials{
		Account: result.Account.Subdomain,
		Email:   email,
		APIKey:  result.APIKey,
	}
	if storeErr := auth.Store(creds); storeErr != nil {
		env.Warn("Could not save credentials: %v", storeErr)
	}
	_ = config.Set(config.GlobalConfigPath(), "account", result.Account.Subdomain)

	if !result.EmailVerified {
		env.Warn("Email not yet verified — account is usable but please verify when you can.")
	}

	output.ColorGreen.Fprintf(env.Stderr, "Account %q created. API key stored.\n", result.Account.Subdomain) //nolint:errcheck

	var sdkOpts []sdk.Option
	if baseURL := cliCtx.Config.BaseURL(result.Account.Subdomain); baseURL != "" {
		sdkOpts = append(sdkOpts, sdk.WithBaseURL(baseURL))
	}
	sdkOpts = append(sdkOpts, sdk.WithUserAgent(cliUserAgent()))
	client, clientErr := sdk.New(result.Account.Subdomain, email, result.APIKey, sdkOpts...)
	if clientErr != nil {
		return nil, "", &output.InternalError{Message: "create api client after signup", Cause: clientErr}
	}
	return client, result.Account.Subdomain, nil
}

func launchLogin(ctx context.Context, env *output.Envelope, reader *bufio.Reader) (*sdk.Client, string, error) {
	creds, err := helloLogin(env, reader)
	if err != nil {
		return nil, "", err
	}

	var sdkOpts []sdk.Option
	if baseURL := cliCtx.Config.BaseURL(creds.Account); baseURL != "" {
		sdkOpts = append(sdkOpts, sdk.WithBaseURL(baseURL))
	}
	sdkOpts = append(sdkOpts, sdk.WithUserAgent(cliUserAgent()))
	client, clientErr := sdk.New(creds.Account, creds.Email, creds.APIKey, sdkOpts...)
	if clientErr != nil {
		return nil, "", &output.InternalError{Message: "create api client after login", Cause: clientErr}
	}
	return client, creds.Account, nil
}

// ── Beta enrollment ──────────────────────────────────────────────────────────

func launchEnsureBetaEnrolled(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, accountSubdomain string) error {
	betaURL := fmt.Sprintf("https://app.deployhq.com/%s/beta_features", accountSubdomain)

	env.Status("")
	env.Status("Managed-resources beta is not enabled on this account.")

	if env.NonInteractive {
		// Non-interactive (CI/agent): attempt enrollment directly. EnrollBeta is
		// idempotent and admin-gated server-side, so an admin (or an already-enrolled
		// account) proceeds automatically, while a non-admin gets the structured
		// beta_enroll_required 403 below.
		env.Status("Enabling managed-resources beta...")
	} else {
		prompt := promptui.Select{
			Label: "Enable managed-resources beta now?",
			Items: []string{"Yes, enable beta", "No, use my own server instead"},
		}
		idx, _, err := prompt.Run()
		if err != nil || idx == 1 {
			env.Status("")
			env.Status("To set up with your own server, run: dhq init")
			return &output.UserError{Message: "Beta enrollment skipped — use 'dhq init' for own-server setup"}
		}
		env.Status("Enrolling in managed-resources beta...")
	}

	_, enrollErr := client.EnrollBeta(ctx, cfg.targetProtocol)
	if enrollErr != nil {
		var apiErr *sdk.APIError
		if errors.As(enrollErr, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
			return &launchError{
				Reason:   reasonBetaEnrollRequired,
				Message:  "Beta enrollment requires an account admin",
				NextStep: "Ask an account admin to enable the beta at: " + betaURL + "\nOr use your own server: dhq init",
				Details:  map[string]string{"beta_url": betaURL, "admin_required": "true"},
			}
		}
		// 404 = endpoint doesn't exist yet (older backend / staging mismatch)
		if errors.As(enrollErr, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			env.Warn("Beta enrollment endpoint not available on this server. Visit %s to enable manually.", betaURL)
			return nil
		}
		return enrollErr
	}

	output.ColorGreen.Fprintf(env.Stderr, "Beta enabled.\n") //nolint:errcheck
	return nil
}

// ── Target selection ─────────────────────────────────────────────────────────

func launchPromptTarget(env *output.Envelope) (string, error) {
	if env.NonInteractive {
		return "", &output.UserError{
			Message: "Target not specified",
			Hint:    "Pass --static (Static Hosting) or --vps (Managed VPS) or choose your own server with dhq init",
		}
	}

	prompt := promptui.Select{
		Label: "Deployment target",
		Items: []string{
			"Static Hosting (beta) — global CDN via Cloudflare, from $2/site" + betaFreeSuffix(),
			"Managed VPS (beta) — DeployHQ provisions and manages a VPS for you" + betaFreeSuffix(),
			"Use my own server (SSH/FTP/…)",
		},
	}
	idx, _, err := prompt.Run()
	if err != nil {
		return "", &output.UserError{Message: "Target selection cancelled"}
	}
	switch idx {
	case 0:
		return detect.ProtocolStaticHosting, nil
	case 1:
		return detect.ProtocolManagedVPS, nil
	default:
		return "own_server", nil
	}
}

// ── Dry-run ───────────────────────────────────────────────────────────────────

func launchDryRun(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, caps *sdk.AccountCapabilities) error {

	dr := dryRunResult{
		Would: dryRunWould{
			Provision: cfg.targetProtocol,
			Project:   cfg.projectID,
			Branch:    cfg.branch,
		},
	}

	switch cfg.targetProtocol {
	case detect.ProtocolStaticHosting:
		dr.Would.Subdomain = cfg.subdomain
		if dr.Would.Subdomain == "" {
			dr.Would.Subdomain = "<repo-name>"
		}
	case detect.ProtocolManagedVPS:
		region := cfg.region
		if region == "" {
			region = "lon1"
		}
		size := cfg.size
		monthlyCost := ""
		if size == "" {
			// Try to fetch first available size for cost estimate
			sizes, sErr := client.ListManagedHostingSizes(ctx)
			if sErr == nil && len(sizes) > 0 {
				size = sizes[0].Slug
				monthlyCost = fmt.Sprintf("$%.2f", sizes[0].PriceMonthly)
			} else {
				size = "s-1vcpu-1gb"
			}
		}
		dr.Would.Region = region
		dr.Would.Size = size
		dr.Would.MonthlyCost = monthlyCost

		if !cfg.acceptCost {
			dr.Requires = append(dr.Requires, "--accept-cost")
		}
		if !caps.BetaFeatures {
			dr.Requires = append(dr.Requires, "beta enrollment")
		}
	}

	if !caps.BetaFeatures && cfg.targetProtocol != "" {
		dr.Warning = "Managed-resources beta not enabled. Run dhq launch without --dry-run to enroll."
	}

	if env.WantsJSON() {
		return env.WriteJSON(output.NewResponse(dr, "Dry run — no changes made"))
	}

	env.Status("DRY RUN — no side effects")
	env.Status("")
	env.Status("Would provision: %s", dr.Would.Provision)
	if dr.Would.Subdomain != "" {
		env.Status("  Subdomain:     %s.deployhq-sites.com", dr.Would.Subdomain)
	}
	if dr.Would.Region != "" {
		env.Status("  Region:        %s", dr.Would.Region)
		env.Status("  Size:          %s", dr.Would.Size)
		env.Status("  Cost:          %s", managedVPSCostDescription(dr.Would.MonthlyCost))
	}
	if len(dr.Requires) > 0 {
		env.Status("")
		env.Status("Required before non-interactive run:")
		for _, r := range dr.Requires {
			env.Status("  %s", r)
		}
	}
	if dr.Warning != "" {
		env.Warn("%s", dr.Warning)
	}
	return nil
}

// ── Project + repo ───────────────────────────────────────────────────────────

func launchEnsureProject(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, gitRemote string) (string, error) {
	// Re-use an existing project if specified or already in .deployhq.toml
	if cfg.projectID != "" {
		env.Status("Using project: %s", cfg.projectID)
		// Ensure a repo is connected — treat hard failures as terminal.
		if err := launchEnsureRepo(ctx, env, cfg, client, cfg.projectID, gitRemote); err != nil {
			return "", &launchError{
				Reason:   reasonRepoUnreachable,
				Message:  "Could not connect repository to project: " + err.Error(),
				NextStep: "Verify the repository URL is accessible and retry, or connect it manually in the DeployHQ dashboard.",
			}
		}
		return cfg.projectID, nil
	}

	// Auto-pick the sole project
	projects, err := client.ListProjects(ctx, nil)
	if err != nil {
		return "", err
	}
	if len(projects) == 1 {
		env.Status("Auto-selected project: %s", projects[0].Name)
		// Treat hard repo-connection failure as terminal.
		if err := launchEnsureRepo(ctx, env, cfg, client, projects[0].Permalink, gitRemote); err != nil {
			return "", &launchError{
				Reason:   reasonRepoUnreachable,
				Message:  "Could not connect repository to project: " + err.Error(),
				NextStep: "Verify the repository URL is accessible and retry, or connect it manually in the DeployHQ dashboard.",
			}
		}
		return projects[0].Permalink, nil
	}

	// Interactive: offer to pick or create
	if !env.NonInteractive {
		return launchPromptOrCreateProject(ctx, env, cfg, client, projects, gitRemote)
	}

	// Non-interactive, no project configured: fail fast
	return "", &output.UserError{
		Message: "No project specified",
		Hint:    "Pass --project <permalink> or set DEPLOYHQ_PROJECT. Run 'dhq projects list' to see available projects.",
	}
}

func launchPromptOrCreateProject(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, existing []sdk.Project, gitRemote string) (string, error) {
	items := []string{"Create a new project"}
	for _, p := range existing {
		items = append(items, fmt.Sprintf("%s (%s)", p.Name, p.Permalink))
	}

	prompt := promptui.Select{
		Label: "Project",
		Items: items,
	}
	idx, _, err := prompt.Run()
	if err != nil {
		return "", &output.UserError{Message: "Project selection cancelled"}
	}

	if idx > 0 {
		p := existing[idx-1]
		env.Status("Using project: %s", p.Name)
		// Treat hard repo-connection failure as terminal.
		if err := launchEnsureRepo(ctx, env, cfg, client, p.Permalink, gitRemote); err != nil {
			return "", &launchError{
				Reason:   reasonRepoUnreachable,
				Message:  "Could not connect repository to project: " + err.Error(),
				NextStep: "Verify the repository URL is accessible and retry, or connect it manually in the DeployHQ dashboard.",
			}
		}
		return p.Permalink, nil
	}

	// Create a new project
	return launchCreateProject(ctx, env, cfg, client, gitRemote)
}

func launchCreateProject(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, gitRemote string) (string, error) {
	// Derive a default project name from the git remote
	projectName := projectNameFromRemote(gitRemote)

	if !env.NonInteractive {
		prompt := promptui.Prompt{
			Label:   "Project name",
			Default: projectName,
		}
		name, err := prompt.Run()
		if err != nil {
			return "", &output.UserError{Message: "Project creation cancelled"}
		}
		if name != "" {
			projectName = name
		}
	}
	if projectName == "" {
		projectName = "my-app"
	}

	env.Status("Creating project %q...", projectName)
	proj, err := client.CreateProject(ctx, sdk.ProjectCreateRequest{Name: projectName})
	if err != nil {
		// 422 name conflict
		var apiErr *sdk.APIError
		if errors.As(err, &apiErr) && apiErr.IsValidationError() {
			// Try with a timestamp suffix
			projectName = projectName + "-" + fmt.Sprintf("%d", time.Now().Unix()%10000)
			env.Status("Name conflict — retrying as %q...", projectName)
			proj, err = client.CreateProject(ctx, sdk.ProjectCreateRequest{Name: projectName})
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	env.Status("Project %q created (%s)", proj.Name, proj.Permalink)

	// Connect the git repository
	if err := launchEnsureRepo(ctx, env, cfg, client, proj.Permalink, gitRemote); err != nil {
		env.Warn("Could not connect repository automatically: %v", err)
		env.Warn("You may need to connect it manually in the DeployHQ dashboard.")
	}

	return proj.Permalink, nil
}

func launchEnsureRepo(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, projectID, gitRemote string) error {
	if gitRemote == "" {
		return nil
	}
	// Check if repo already connected
	proj, err := client.GetProject(ctx, projectID)
	if err == nil && proj.Repository != nil && proj.Repository.URL != "" {
		// Already connected
		return nil
	}

	env.Status("Connecting repository %s...", gitRemote)
	branch := cfg.branch
	if branch == "" {
		// Detect the local default branch rather than hardcoding "main".
		// Try the local HEAD first, then the tracked remote HEAD. Fall back to
		// omitting the branch so the DeployHQ API resolves the repo default.
		branch = detectDefaultBranch()
	}
	_, err = client.CreateRepository(ctx, projectID, sdk.RepositoryCreateRequest{
		ScmType: "git",
		URL:     gitRemote,
		Branch:  branch,
	})
	if err != nil {
		return err
	}
	env.Status("Repository connected.")
	return nil
}

// detectDefaultBranch returns the local HEAD branch name for the repository in
// the current working directory. Returns "" when it cannot be determined so the
// API can use its own default resolution.
func detectDefaultBranch() string {
	// Try local HEAD (works for checked-out repos)
	if out, err := runGitCommand("symbolic-ref", "--short", "HEAD"); err == nil && out != "" {
		return out
	}
	// Try remote tracking HEAD (works in detached-HEAD / CI clones)
	if out, err := runGitCommand("rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil && out != "" {
		// Strip "origin/" prefix if present
		if len(out) > 7 && out[:7] == "origin/" {
			return out[7:]
		}
		return out
	}
	// Return empty — let the API decide
	return ""
}

// projectNameFromRemote derives a human-readable project name from a git remote URL.
// git@github.com:acme/my-app.git → "my-app"
// https://github.com/acme/my-app.git → "my-app"
func projectNameFromRemote(remote string) string {
	if remote == "" {
		return "my-app"
	}
	// Strip trailing .git
	remote = strings.TrimSuffix(remote, ".git")
	// Take the last path segment
	if i := strings.LastIndexAny(remote, "/:"); i >= 0 {
		remote = remote[i+1:]
	}
	if remote == "" {
		return "my-app"
	}
	return remote
}

// ── Plan / limit pre-flight ───────────────────────────────────────────────────

func launchCheckPlanLimits(env *output.Envelope, cfg launchConfig, caps *sdk.AccountCapabilities) error {
	if cfg.targetProtocol == detect.ProtocolStaticHosting && !caps.StaticHostingEligible {
		// Not eligible = plan limit or billing wall
		return &launchError{
			Reason:   reasonPlanLimitReached,
			Message:  "Your account cannot provision Static Hosting sites",
			NextStep: "Check your plan or billing at https://app.deployhq.com/account/plan. Free plans support 1 site.",
			Details:  map[string]string{"target": detect.ProtocolStaticHosting},
		}
	}
	if cfg.targetProtocol == detect.ProtocolManagedVPS && !caps.ManagedVPSEligible {
		return &launchError{
			Reason:   reasonPlanLimitReached,
			Message:  "Your account cannot provision Managed VPS servers",
			NextStep: "Ensure your billing details are set up at https://app.deployhq.com/account/billing",
			Details:  map[string]string{"target": detect.ProtocolManagedVPS},
		}
	}
	return nil
}

// ── Provision ─────────────────────────────────────────────────────────────────

func launchProvision(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client) (*sdk.Server, error) {
	switch cfg.targetProtocol {
	case detect.ProtocolStaticHosting:
		return launchProvisionStatic(ctx, env, cfg, client)
	case detect.ProtocolManagedVPS:
		return launchProvisionVPS(ctx, env, cfg, client)
	default:
		return nil, &output.UserError{Message: "Unknown target protocol: " + cfg.targetProtocol}
	}
}

func launchProvisionStatic(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client) (*sdk.Server, error) {
	subdomain := cfg.subdomain
	if subdomain == "" {
		subdomain = projectNameFromRemote(detectGitRemote())
	}

	if !env.NonInteractive {
		// Prompt for subdomain with default
		subPrompt := promptui.Prompt{Label: "Subdomain", Default: subdomain}
		if s, err := subPrompt.Run(); err == nil && s != "" {
			subdomain = s
		}

		// SPA routing prompt
		spaPrompt := promptui.Select{
			Label: "SPA routing (rewrite all paths to index.html)?",
			Items: []string{"No", "Yes"},
		}
		if idx, _, err := spaPrompt.Run(); err == nil {
			cfg.spaMode = idx == 1
		}
	} else if subdomain == "" {
		return nil, &output.UserError{
			Message: "Subdomain not specified",
			Hint:    "Pass --subdomain <name>",
		}
	}

	env.Status("Provisioning Static Hosting site: %s.deployhq-sites.com...", subdomain)

	req := sdk.ServerCreateRequest{
		Name:         subdomain,
		ProtocolType: detect.ProtocolStaticHosting,
		HostedWebsiteAttributes: &sdk.HostedWebsiteAttributes{
			Subdomain:    subdomain,
			SPAMode:      cfg.spaMode,
			Subdirectory: cfg.subdirectory,
		},
	}

	server, err := client.CreateServer(ctx, cfg.projectID, req)
	if err != nil {
		// 429 provisioning rate limit — retryable, distinct from the 422 cap.
		if rl := rateLimitLaunchError(err); rl != nil {
			return nil, rl
		}
		var apiErr *sdk.APIError
		if errors.As(err, &apiErr) && apiErr.IsValidationError() {
			msg := apiErr.Error()
			if strings.Contains(strings.ToLower(msg), "subdomain") {
				if env.NonInteractive {
					return nil, &launchError{
						Reason:   reasonSubdomainTaken,
						Message:  "Subdomain already taken: " + subdomain,
						NextStep: "Pass --subdomain <unique-name> to choose a different subdomain",
						Details:  map[string]string{"subdomain": subdomain},
					}
				}
				// Interactive: re-prompt for a different subdomain
				env.Warn("Subdomain %q is taken. Please choose another.", subdomain)
				subPrompt := promptui.Prompt{Label: "Subdomain", Default: subdomain + "-app"}
				newSub, promptErr := subPrompt.Run()
				if promptErr != nil || newSub == "" {
					return nil, &output.UserError{Message: "Subdomain selection cancelled"}
				}
				cfg.subdomain = newSub
				return launchProvisionStatic(ctx, env, cfg, client)
			}
		}
		return nil, err
	}

	// Poll until active
	server, err = pollProvisioningState(ctx, env, client, cfg.projectID, server, 10*time.Minute)
	if err != nil {
		return server, err
	}

	liveURL := sdk.LiveURL(server)
	output.ColorGreen.Fprintf(env.Stderr, "Static Hosting site provisioned: %s\n", liveURL) //nolint:errcheck
	return server, nil
}

// ── Managed VPS size presentation ─────────────────────────────────────────────

// humanMB renders a RAM figure (in MB) as a friendly "1 GB" / "512 MB" string.
func humanMB(mb int) string {
	switch {
	case mb >= 1024 && mb%1024 == 0:
		return fmt.Sprintf("%d GB", mb/1024)
	case mb >= 1024:
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	default:
		return fmt.Sprintf("%d MB", mb)
	}
}

// managedSizeTier returns a friendly tier name for a size given its zero-based
// rank among the offered sizes ordered by price (cheapest first), or "" when the
// rank is beyond the named tiers — those fall back to a spec-only label.
func managedSizeTier(rank int) string {
	switch rank {
	case 0:
		return "Starter"
	case 1:
		return "Standard"
	case 2:
		return "Plus"
	case 3:
		return "Pro"
	default:
		return ""
	}
}

// managedSizeRanks returns the price-rank of each size aligned to the input
// order (0 = cheapest). Ties are broken by original index for stability. O(n²)
// but n is a handful of sizes, so no sort import is warranted.
func managedSizeRanks(sizes []sdk.ManagedHostingSize) []int {
	ranks := make([]int, len(sizes))
	for i := range sizes {
		rank := 0
		for j := range sizes {
			if j == i {
				continue
			}
			if sizes[j].PriceMonthly < sizes[i].PriceMonthly ||
				(sizes[j].PriceMonthly == sizes[i].PriceMonthly && j < i) {
				rank++
			}
		}
		ranks[i] = rank
	}
	return ranks
}

// managedSizeSpecs renders the hardware line for a size, e.g.
// "1 vCPU · 1 GB RAM · 25 GB SSD". Falls back to the API's Description when the
// structured fields are absent.
func managedSizeSpecs(s sdk.ManagedHostingSize) string {
	if s.VCPUs > 0 {
		return fmt.Sprintf("%d vCPU · %s RAM · %d GB SSD", s.VCPUs, humanMB(s.Memory), s.Disk)
	}
	return s.Description
}

// managedSizeLabel renders a human-friendly one-line label for a size in the
// interactive picker, with an optional tier prefix derived from its price rank:
//
//	Starter · 1 vCPU · 1 GB RAM · 25 GB SSD · $6.00/mo  (s-1vcpu-1gb)
//
// The slug stays visible so the equivalent --size flag is discoverable.
func managedSizeLabel(s sdk.ManagedHostingSize, rank int) string {
	parts := make([]string, 0, 3)
	if tier := managedSizeTier(rank); tier != "" {
		parts = append(parts, tier)
	}
	if specs := managedSizeSpecs(s); specs != "" {
		parts = append(parts, specs)
	}
	parts = append(parts, fmt.Sprintf("$%.2f/mo", s.PriceMonthly))
	return fmt.Sprintf("%s  (%s)", strings.Join(parts, " · "), s.Slug)
}

func launchProvisionVPS(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client) (*sdk.Server, error) {
	// Resolve region and size defaults
	region := cfg.region
	size := cfg.size
	osImage := cfg.osImage
	if osImage == "" {
		osImage = "ubuntu-24-04-x64"
	}

	var selectedRegion sdk.ManagedHostingRegion
	var selectedSize sdk.ManagedHostingSize
	var monthlyCostStr string

	// Fetch available regions and sizes
	regions, err := client.ListManagedHostingRegions(ctx)
	if err != nil {
		// Fall back to hardcoded defaults if endpoint not available
		if region == "" {
			region = "lon1"
		}
		if size == "" {
			size = "s-1vcpu-1gb"
		}
		monthlyCostStr = "contact support for pricing"
	} else {
		// Pick region
		if region == "" {
			if len(regions) > 0 {
				// Pick first available region as default
				for _, r := range regions {
					if r.Available {
						selectedRegion = r
						region = r.Slug
						break
					}
				}
				if region == "" {
					region = regions[0].Slug
					selectedRegion = regions[0]
				}
			} else {
				region = "lon1"
			}
		} else {
			for _, r := range regions {
				if r.Slug == region {
					selectedRegion = r
					break
				}
			}
		}

		// Fetch sizes
		sizes, sErr := client.ListManagedHostingSizes(ctx)
		if sErr == nil && len(sizes) > 0 {
			if size == "" {
				selectedSize = sizes[0]
				size = sizes[0].Slug
			} else {
				for _, s := range sizes {
					if s.Slug == size {
						selectedSize = s
						break
					}
				}
				if selectedSize.Slug == "" {
					selectedSize = sizes[0]
				}
			}
			monthlyCostStr = fmt.Sprintf("$%.2f/month", selectedSize.PriceMonthly)
		} else {
			if size == "" {
				size = "s-1vcpu-1gb"
			}
			monthlyCostStr = "see dashboard for pricing"
		}

		// Interactive: allow region/size selection
		if !env.NonInteractive {
			if len(regions) > 1 {
				// Keep a parallel slice of the available regions: the prompt only
				// lists those, so the selected index must map back through this
				// filtered slice — indexing the full `regions` slice would pick the
				// wrong region whenever an unavailable one precedes an available one.
				availableRegions := make([]sdk.ManagedHostingRegion, 0, len(regions))
				regionItems := make([]string, 0, len(regions))
				for _, r := range regions {
					if r.Available {
						availableRegions = append(availableRegions, r)
						regionItems = append(regionItems, fmt.Sprintf("%s (%s)", r.Name, r.Slug))
					}
				}
				regionPrompt := promptui.Select{
					Label: fmt.Sprintf("Region [%s]", region),
					Items: regionItems,
				}
				if rIdx, _, rErr := regionPrompt.Run(); rErr == nil && rIdx < len(availableRegions) {
					region = availableRegions[rIdx].Slug
					selectedRegion = availableRegions[rIdx]
				}
			}

			sizes, sErr := client.ListManagedHostingSizes(ctx)
			if sErr == nil && len(sizes) > 1 {
				ranks := managedSizeRanks(sizes)
				sizeItems := make([]string, len(sizes))
				for i, s := range sizes {
					sizeItems[i] = managedSizeLabel(s, ranks[i])
				}
				sizePrompt := promptui.Select{
					Label: fmt.Sprintf("Size [%s]", size),
					Items: sizeItems,
				}
				if sIdx, _, sErr2 := sizePrompt.Run(); sErr2 == nil {
					selectedSize = sizes[sIdx]
					size = selectedSize.Slug
					monthlyCostStr = fmt.Sprintf("$%.2f/month", selectedSize.PriceMonthly)
				}
			}
		}
	}

	// Cost confirmation gate
	regionName := selectedRegion.Name
	if regionName == "" {
		regionName = region
	}
	sizeDisplay := size
	if selectedSize.Slug != "" {
		if specs := managedSizeSpecs(selectedSize); specs != "" {
			sizeDisplay = fmt.Sprintf("%s (%s)", specs, selectedSize.Slug)
		}
	}
	env.Status("")
	env.Status("Managed VPS configuration:")
	env.Status("  Region: %s", regionName)
	env.Status("  Size:   %s", sizeDisplay)
	env.Status("  OS:     %s", osImage)
	env.Status("  Cost:   %s", managedVPSCostDescription(monthlyCostStr))
	env.Status("")

	if !cfg.acceptCost {
		if env.NonInteractive {
			return nil, &launchError{
				Reason:   reasonAcceptCostRequired,
				Message:  "Provisioning a Managed VPS requires --accept-cost (" + managedVPSAcknowledgePhrase() + ")",
				NextStep: "Add --accept-cost to acknowledge that a Managed VPS is " + managedVPSAcknowledgePhrase(),
				Details: map[string]string{
					"monthly_cost": monthlyCostStr,
					"region":       region,
					"size":         size,
				},
			}
		}

		// Interactive cost confirm
		confirmPrompt := promptui.Select{
			Label: fmt.Sprintf("Provision a Managed VPS (%s)? Continue?", managedVPSCostDescription(monthlyCostStr)),
			Items: []string{"No", "Yes, provision VPS"},
		}
		idx, _, cErr := confirmPrompt.Run()
		if cErr != nil || idx == 0 {
			return nil, &output.UserError{Message: "VPS provisioning cancelled"}
		}
	}

	serverName := size + "-" + region
	if cfg.projectID != "" {
		serverName = cfg.projectID + "-vps"
	}

	env.Status("Provisioning Managed VPS...")
	req := sdk.ServerCreateRequest{
		Name:         serverName,
		ProtocolType: detect.ProtocolManagedVPS,
		Region:       region,
		Size:         size,
		OSImage:      osImage,
	}

	server, err := client.CreateServer(ctx, cfg.projectID, req)
	if err != nil {
		// 429 provisioning rate limit — retryable, distinct from the 422 cap.
		if rl := rateLimitLaunchError(err); rl != nil {
			return nil, rl
		}
		return nil, err
	}

	// Poll until active
	server, err = pollProvisioningState(ctx, env, client, cfg.projectID, server, 15*time.Minute)
	if err != nil {
		return server, err
	}

	ipAddr := ""
	if server.ManagedVPS != nil {
		ipAddr = server.ManagedVPS.IPAddress
	}
	output.ColorGreen.Fprintf(env.Stderr, "Managed VPS provisioned: IP %s\n", ipAddr) //nolint:errcheck
	return server, nil
}

// provisionPollInitialBackoff is the first poll delay in pollProvisioningState.
// It is a variable (not a constant) only as a test seam — tests shrink it to
// ~1ms so the provisioning→active sequence runs without real waits.
var provisionPollInitialBackoff = 5 * time.Second

// pollProvisioningState polls GET /projects/:id/servers/:id until the managed
// resource reaches "active" or "error". It prints a progress message on the
// first call and an "async, safe to Ctrl-C" hint.
func pollProvisioningState(ctx context.Context, env *output.Envelope, client *sdk.Client, projectID string, server *sdk.Server, maxWait time.Duration) (*sdk.Server, error) {
	if sdk.IsProvisioningActive(server) {
		return server, nil
	}

	env.Status("Provisioning is async — safe to Ctrl-C (resource will continue provisioning).")
	env.Status("Waiting for active status...")

	deadline := time.Now().Add(maxWait)
	backoff := provisionPollInitialBackoff
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return server, ctx.Err()
		case <-time.After(backoff):
		}

		updated, err := client.GetServerProvisioningState(ctx, projectID, server.Identifier)
		if err != nil {
			// Terminate immediately on non-retryable errors (401/403/404) to
			// avoid burning the full timeout on a permanent failure.
			var apiErr *sdk.APIError
			if errors.As(err, &apiErr) {
				switch apiErr.StatusCode {
				case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
					return server, &launchError{
						Reason:   reasonProvisionFailed,
						Message:  fmt.Sprintf("Provisioning poll failed (status %d): %s", apiErr.StatusCode, apiErr.Error()),
						NextStep: "Check your credentials and project access, then retry.",
					}
				}
			}
			// Transient error: keep polling
			env.Warn("Poll error (will retry): %v", err)
			continue
		}
		server = updated

		status := sdk.ProvisioningStatus(server)
		env.Status("  Provisioning status: %s", status)

		if sdk.IsProvisioningActive(server) {
			return server, nil
		}
		if strings.EqualFold(status, "error") {
			return server, &launchError{
				Reason:   reasonProvisionFailed,
				Message:  "Provisioning failed (status: error)",
				NextStep: "Check the DeployHQ dashboard for error details, or run 'dhq servers delete' to remove the failed resource",
			}
		}

		if time.Now().After(deadline) {
			return server, &launchError{
				Reason:   reasonProvisionFailed,
				Message:  fmt.Sprintf("Provisioning timed out after %s", maxWait),
				NextStep: "The resource may still be provisioning. Check status with: dhq servers show -p " + projectID + " " + server.Identifier,
			}
		}

		// Exponential backoff capped at maxBackoff
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// ── Build command ─────────────────────────────────────────────────────────────

// launchApplyBuildCommand sets the detected build command on the project so the
// first Static Hosting deploy publishes the built output (dist/public/…) rather
// than unbuilt sources. It mirrors the web onboarding wizard
// (Onboarding::ProjectCreator), which creates project build commands via
// POST /projects/:id/build_commands — no separate build environment is needed.
func launchApplyBuildCommand(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, detection detect.Result) {
	if detection.BuildCommand == "" {
		return
	}
	// Don't clobber or duplicate: if the project already has build commands
	// (an idempotent re-run, or a reused --project), leave them as-is.
	if existing, err := client.ListBuildCommands(ctx, cfg.projectID, nil); err == nil && len(existing) > 0 {
		return
	}

	env.Status("Setting build command from detection: %s", detection.BuildCommand)
	desc := detection.BuildCommand
	if r := []rune(desc); len(r) > 100 {
		desc = string(r[:100])
	}
	_, err := client.CreateBuildCommand(ctx, cfg.projectID, sdk.BuildCommandCreateRequest{
		Command:     detection.BuildCommand,
		Description: desc,
	})
	if err != nil {
		// Non-fatal: the site still provisions, but without a build step the
		// first deploy may publish unbuilt sources — make the fix discoverable.
		env.Warn("Could not set the detected build command (%q): %v", detection.BuildCommand, err)
		env.Warn("Add it manually: dhq build-commands create -p %s --command %q", cfg.projectID, detection.BuildCommand)
	}
}

// ── Deploy ────────────────────────────────────────────────────────────────────

// resolveLaunchRevision picks the end_revision for the launch deploy. With a
// branch set it resolves THAT branch's tip — resolveLatestRevision only knows the
// repository default, so pairing its SHA with a different Branch would deploy the
// wrong commit. Returns "" to let the backend resolve the branch/repo HEAD from
// the Branch field (e.g. when the branch tip can't be looked up).
func resolveLaunchRevision(ctx context.Context, env *output.Envelope, client *sdk.Client, cfg launchConfig) string {
	if cfg.branch != "" {
		if branches, err := client.ListBranches(ctx, cfg.projectID, nil); err == nil {
			return branches[cfg.branch]
		}
		return ""
	}
	rev, err := resolveLatestRevision(ctx, client, cfg.projectID)
	if err != nil {
		env.Warn("Could not fetch latest revision, deploying HEAD: %v", err)
		return ""
	}
	return rev
}

func launchDeploy(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, server *sdk.Server) (*sdk.Deployment, string, error) {
	env.Status("Deploying...")

	req := sdk.DeploymentCreateRequest{
		ParentIdentifier: server.Identifier,
		EndRevision:      resolveLaunchRevision(ctx, env, client, cfg),
		Branch:           cfg.branch,
	}

	dep, err := client.CreateDeployment(ctx, cfg.projectID, req)
	if err != nil {
		return nil, "", err
	}

	env.Status("Deployment %s queued", dep.Identifier)

	// Watch the deployment
	if err := watchDeployment(ctx, client, env, cfg.projectID, dep.Identifier); err != nil {
		return dep, "", err
	}

	liveURL := sdk.LiveURL(server)
	return dep, liveURL, nil
}

// ── Failure / cleanup ─────────────────────────────────────────────────────────

func launchDeployFailureCleanup(ctx context.Context, env *output.Envelope, cfg launchConfig, client *sdk.Client, server *sdk.Server) {
	env.Status("")
	env.Warn("Deploy failed. Your %s %q is still running%s.", cfg.targetProtocol, server.Name, managedRunningCostTail())
	env.Status("To remove it: dhq servers delete -p %s %s", cfg.projectID, server.Identifier)
	env.Status("To re-deploy: dhq deploy -p %s -s %s", cfg.projectID, server.Identifier)

	if cfg.cleanupOnFailure {
		env.Status("--cleanup-on-failure set: deleting server %s...", server.Identifier)
		if delErr := client.DeleteServer(ctx, cfg.projectID, server.Identifier); delErr != nil {
			env.Warn("Could not delete server: %v", delErr)
		} else {
			env.Status("Server %s deleted.", server.Identifier)
		}
	}
}

// ── Persist ───────────────────────────────────────────────────────────────────

func launchPersistConfig(env *output.Envelope, cfg launchConfig, server *sdk.Server) {
	path := config.ProjectConfigPath()
	_ = config.Set(path, "project", cfg.projectID)
	if server != nil {
		_ = config.Set(path, "server", server.Identifier)
	}
	_ = config.Set(path, "target", cfg.targetProtocol)
	env.Status("Settings saved to %s", path)
}

// ── Git helpers ───────────────────────────────────────────────────────────────

// runGitCommand runs a git sub-command and returns the trimmed stdout output.
// Returns an error when the command fails or produces no output.
func runGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "", fmt.Errorf("git %s: empty output", strings.Join(args, " "))
	}
	return result, nil
}

// ── Error helpers ─────────────────────────────────────────────────────────────

// writeLaunchError emits a structured JSON error (in --json mode) or a human
// error (plain mode) and returns the underlying error for exit-code purposes.
func writeLaunchError(env *output.Envelope, cfg launchConfig, reason string, err error) error {
	if !env.WantsJSON() {
		return err
	}

	// Build structured error payload using errors.As so wrapped errors keep their
	// metadata (cheap correctness fix).
	var le *launchError
	isLaunchErr := errors.As(err, &le)

	code := reason
	message := err.Error()
	nextStep := ""
	details := map[string]string{}
	retryable := false

	if isLaunchErr {
		// The wrapped error's own Reason is authoritative — a rate_limited or
		// subdomain_taken error surfaced through a generic call site (e.g.
		// provision_failed) must keep its true reason and retryable flag.
		if le.Reason != "" {
			code = le.Reason
		}
		message = le.Message
		nextStep = le.NextStep
		retryable = le.Retryable
		for k, v := range le.Details {
			details[k] = v
		}
	}

	type structuredErr struct {
		Error     string            `json:"error"`
		Reason    string            `json:"reason"`
		Retryable bool              `json:"retryable"`
		NextStep  string            `json:"next_step,omitempty"`
		Details   map[string]string `json:"details,omitempty"`
	}

	resp := &output.Response{
		OK: false,
		Data: structuredErr{
			Error:     message,
			Reason:    code,
			Retryable: retryable,
			NextStep:  nextStep,
			Details:   details,
		},
	}
	_ = env.WriteJSON(resp)
	return err
}
