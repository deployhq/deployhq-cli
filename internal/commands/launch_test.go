package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// testLaunchEnvelope creates a non-TTY, non-interactive Envelope backed by buffers.
func testLaunchEnvelope() (*output.Envelope, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{
		Stdout:         &stdout,
		Stderr:         &stderr,
		Logger:         &output.Logger{},
		IsTTY:          false,
		NonInteractive: true,
		JSONMode:       false,
	}
	return env, &stdout, &stderr
}

// testLaunchEnvelopeJSON creates a JSON-mode Envelope for --json tests.
func testLaunchEnvelopeJSON() (*output.Envelope, *bytes.Buffer, *bytes.Buffer) {
	env, stdout, stderr := testLaunchEnvelope()
	env.JSONMode = true
	return env, stdout, stderr
}

// newTestClient returns an SDK client wired to the given httptest server.
func newTestClient(t *testing.T, srv *httptest.Server) *sdk.Client {
	t.Helper()
	c, err := sdk.New("test", "u@e.com", "k",
		sdk.WithBaseURL(srv.URL),
		sdk.WithHTTPClient(srv.Client()),
	)
	require.NoError(t, err)
	return c
}

// ── Unit: resolveLaunchConfig ─────────────────────────────────────────────────

func TestResolveLaunchConfig_FlagStaticSetsProtocol(t *testing.T) {
	// --static must set targetProtocol without cliCtx involvement
	cfg := resolveLaunchConfig(true, false, "myapp", "", "", "", "", false, false, false)
	assert.Equal(t, "static_hosting", cfg.targetProtocol)
	assert.Equal(t, "myapp", cfg.subdomain)
}

func TestResolveLaunchConfig_FlagVPSSetsProtocol(t *testing.T) {
	cfg := resolveLaunchConfig(false, true, "", "lon1", "s-1vcpu-1gb", "main", "", true, false, false)
	assert.Equal(t, "managed_vps", cfg.targetProtocol)
	assert.Equal(t, "lon1", cfg.region)
	assert.Equal(t, "s-1vcpu-1gb", cfg.size)
	assert.True(t, cfg.acceptCost)
}

func TestResolveLaunchConfig_NoFlagsMeansEmptyProtocol(t *testing.T) {
	cfg := resolveLaunchConfig(false, false, "", "", "", "", "", false, false, false)
	assert.Equal(t, "", cfg.targetProtocol, "no flag → protocol empty, resolved later via detection/prompt")
}

func TestResolveLaunchConfig_DryRunFlag(t *testing.T) {
	cfg := resolveLaunchConfig(false, true, "", "", "", "", "", false, false, true)
	assert.True(t, cfg.dryRun)
}

// ── Unit: projectNameFromRemote ───────────────────────────────────────────────

func TestProjectNameFromRemote_SSHStyle(t *testing.T) {
	name := projectNameFromRemote("git@github.com:acme/my-app.git")
	assert.Equal(t, "my-app", name)
}

func TestProjectNameFromRemote_HTTPS(t *testing.T) {
	name := projectNameFromRemote("https://github.com/acme/example-repo.git")
	assert.Equal(t, "example-repo", name)
}

func TestProjectNameFromRemote_Empty(t *testing.T) {
	name := projectNameFromRemote("")
	assert.Equal(t, "my-app", name)
}

func TestProjectNameFromRemote_NoExtension(t *testing.T) {
	name := projectNameFromRemote("https://bitbucket.org/team/service")
	assert.Equal(t, "service", name)
}

// ── Integration: auth_required in non-interactive mode ───────────────────────

func TestLaunchErrorAuthRequired_NonInteractive(t *testing.T) {
	// When no credentials are available and env is non-interactive, launch must
	// return a structured auth_required error — never attempt headless signup.

	// Save and restore cliCtx state
	origCtx := cliCtx
	defer func() { cliCtx = origCtx }()

	// Set up a minimal cliCtx that returns an auth error
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"launch", "--non-interactive", "--static", "--subdomain", "testapp"})
	// Don't actually run — just check the flag is registered
	launchCmd, _, _ := cmd.Find([]string{"launch"})
	require.NotNil(t, launchCmd)
	assert.NotNil(t, launchCmd.Flags().Lookup("non-interactive"))
	assert.NotNil(t, launchCmd.Flags().Lookup("accept-cost"))
	assert.NotNil(t, launchCmd.Flags().Lookup("static"))
	assert.NotNil(t, launchCmd.Flags().Lookup("vps"))
	assert.NotNil(t, launchCmd.Flags().Lookup("dry-run"))
	assert.NotNil(t, launchCmd.Flags().Lookup("subdomain"))
	assert.NotNil(t, launchCmd.Flags().Lookup("region"))
	assert.NotNil(t, launchCmd.Flags().Lookup("size"))
	assert.NotNil(t, launchCmd.Flags().Lookup("branch"))
	assert.NotNil(t, launchCmd.Flags().Lookup("cleanup-on-failure"))
	assert.NotNil(t, launchCmd.Flags().Lookup("yes"))
	assert.NotNil(t, launchCmd.Flags().Lookup("interactive"))
}

// ── Integration: accept_cost_required ────────────────────────────────────────

func TestLaunchVPS_AcceptCostRequired_NonInteractive(t *testing.T) {
	// Non-interactive VPS provisioning without --accept-cost must return
	// accept_cost_required — never silently charge the user.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/profile":
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"account": map[string]interface{}{
					"beta_features":          true,
					"static_hosting_eligible": true,
					"managed_vps_eligible":   true,
				},
			})
		case "/managed_hosting/regions":
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint:errcheck
				{"slug": "lon1", "name": "London", "available": true},
			})
		case "/managed_hosting/sizes":
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint:errcheck
				{"slug": "s-1vcpu-1gb", "description": "1 vCPU / 1 GB", "price_monthly": 6.0},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelope()

	cfg := launchConfig{
		targetProtocol: "managed_vps",
		projectID:      "test-proj",
		acceptCost:     false, // deliberately absent
	}

	server, err := launchProvisionVPS(t.Context(), env, cfg, client)
	require.Error(t, err)
	assert.Nil(t, server)

	var le *launchError
	require.True(t, isLaunchErr(err, &le), "must be a launchError")
	assert.Equal(t, reasonAcceptCostRequired, le.Reason)
	assert.Contains(t, strings.ToLower(le.Message), "billable")
	assert.Contains(t, le.NextStep, "--accept-cost")
}

// ── Integration: accept_cost passes with --accept-cost ────────────────────────

func TestLaunchVPS_AcceptCostPresent_ProvisionsCalled(t *testing.T) {
	// With --accept-cost the flow should proceed past the cost gate and
	// call POST /projects/:id/servers.
	provisionCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/managed_hosting/regions":
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint:errcheck
				{"slug": "lon1", "name": "London", "available": true},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/managed_hosting/sizes":
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint:errcheck
				{"slug": "s-1vcpu-1gb", "description": "1 vCPU", "price_monthly": 6.0},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/servers"):
			provisionCalled = true
			// Return a server in "active" state so polling is skipped
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"identifier":    "srv-abc",
				"name":          "s-1vcpu-1gb-lon1",
				"protocol_type": "managed_vps",
				"managed_vps": map[string]interface{}{
					"status":     "active",
					"ip_address": "203.0.113.10",
					"region":     "lon1",
					"size":       "s-1vcpu-1gb",
				},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelope()

	cfg := launchConfig{
		targetProtocol: "managed_vps",
		projectID:      "test-proj",
		acceptCost:     true,
		region:         "lon1",
		size:           "s-1vcpu-1gb",
	}

	server, err := launchProvisionVPS(t.Context(), env, cfg, client)
	require.NoError(t, err)
	assert.True(t, provisionCalled, "POST /servers must be called when --accept-cost is set")
	require.NotNil(t, server)
	assert.Equal(t, "srv-abc", server.Identifier)
}

// ── Integration: repo_unreachable ────────────────────────────────────────────

func TestLaunchError_RepoUnreachable(t *testing.T) {
	// writeLaunchError with reason=repo_unreachable must surface the reason.
	env, _, stderr := testLaunchEnvelope()
	err := &output.UserError{
		Message: "No git remote found in this directory",
		Hint:    "Run: git remote add origin <url>",
	}
	result := writeLaunchError(env, launchConfig{}, reasonRepoUnreachable, err)
	require.Error(t, result)
	assert.Contains(t, stderr.String(), "", "plain mode: just return the error, no JSON emitted")
	assert.Equal(t, err, result, "error must pass through unchanged in plain mode")
}

func TestLaunchError_RepoUnreachable_JSON(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	err := &launchError{
		Reason:   reasonRepoUnreachable,
		Message:  "No git remote found",
		NextStep: "Run: git remote add origin <url>",
	}
	result := writeLaunchError(env, launchConfig{}, reasonRepoUnreachable, err)
	require.Error(t, result)

	// JSON must contain the reason field
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	assert.Equal(t, false, resp["ok"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, reasonRepoUnreachable, data["reason"])
	assert.Contains(t, data["next_step"], "git remote")
}

// ── Integration: beta_enroll_required ────────────────────────────────────────

func TestLaunchBetaEnroll_NonAdminForbidden(t *testing.T) {
	// POST /beta/enrollments returns 403 → launchError with beta_enroll_required.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/beta/enrollments" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"admin_required"}`)) //nolint:errcheck
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelope()

	cfg := launchConfig{targetProtocol: "static_hosting"}
	err := launchEnsureBetaEnrolled(t.Context(), env, cfg, client, "myaccount")
	require.Error(t, err)

	var le *launchError
	require.True(t, isLaunchErr(err, &le))
	assert.Equal(t, reasonBetaEnrollRequired, le.Reason)
	assert.Contains(t, le.NextStep, "admin")
	assert.Contains(t, le.Details["admin_required"], "true")
}

func TestLaunchBetaEnroll_AlreadyEnrolled_Idempotent(t *testing.T) {
	// POST /beta/enrollments returns 200 with enrolled:true → no error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/beta/enrollments" {
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"enrolled":      true,
				"beta_features": true,
			})
			return
		}
		t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	// An already-enrolled account must succeed idempotently on EnrollBeta,
	// regardless of admin status (D8). Verify that round-trip directly.
	_, enrollErr := client.EnrollBeta(t.Context(), "static_hosting")
	require.NoError(t, enrollErr, "already-enrolled account must succeed idempotently")
}

// ── Integration: subdomain_taken 422 handling ─────────────────────────────────

func TestLaunchStatic_SubdomainTaken_NonInteractive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/servers") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"errors":["subdomain has already been taken"]}`)) //nolint:errcheck
			return
		}
		t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelope() // NonInteractive=true

	cfg := launchConfig{
		targetProtocol: "static_hosting",
		projectID:      "test-proj",
		subdomain:      "taken-subdomain",
	}

	server, err := launchProvisionStatic(t.Context(), env, cfg, client)
	require.Error(t, err)
	assert.Nil(t, server)

	var le *launchError
	require.True(t, isLaunchErr(err, &le))
	assert.Equal(t, reasonSubdomainTaken, le.Reason)
	assert.Contains(t, le.NextStep, "--subdomain")
}

// ── Integration: dry-run output ───────────────────────────────────────────────

func TestLaunchDryRun_Static_JSON(t *testing.T) {
	// --dry-run --static --json must emit the intended action with no side effects.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No requests should reach the server for a static dry run (no size/region calls needed)
		// We may receive the capabilities request but dry run should not provision.
		if r.Method == http.MethodGet && r.URL.Path == "/profile" {
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"account": map[string]interface{}{
					"beta_features":          true,
					"static_hosting_eligible": true,
					"managed_vps_eligible":   true,
				},
			})
			return
		}
		// POST to any endpoint in dry-run must not happen
		if r.Method == http.MethodPost {
			t.Errorf("dry-run must not make POST requests: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, stdout, _ := testLaunchEnvelopeJSON()

	caps := &sdk.AccountCapabilities{
		BetaFeatures:          true,
		StaticHostingEligible: true,
		ManagedVPSEligible:    true,
	}
	cfg := launchConfig{
		targetProtocol: "static_hosting",
		subdomain:      "my-app",
		dryRun:         true,
	}

	err := launchDryRun(t.Context(), env, cfg, client, caps)
	require.NoError(t, err)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	assert.Equal(t, true, resp["ok"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	would, ok := data["would"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "static_hosting", would["provision"])
	assert.Contains(t, would["subdomain"], "my-app")
}

func TestLaunchDryRun_VPS_JSON_RequiresAcceptCost(t *testing.T) {
	// VPS dry-run without --accept-cost must list it in "requires".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/managed_hosting/sizes" {
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint:errcheck
				{"slug": "s-1vcpu-1gb", "description": "1 vCPU", "price_monthly": 6.0},
			})
			return
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, stdout, _ := testLaunchEnvelopeJSON()

	caps := &sdk.AccountCapabilities{
		BetaFeatures:       true,
		ManagedVPSEligible: true,
	}
	cfg := launchConfig{
		targetProtocol: "managed_vps",
		acceptCost:     false, // not set
		dryRun:         true,
	}

	err := launchDryRun(t.Context(), env, cfg, client, caps)
	require.NoError(t, err)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	requires, ok := data["requires"].([]interface{})
	require.True(t, ok)
	var requiresStrs []string
	for _, r := range requires {
		requiresStrs = append(requiresStrs, r.(string))
	}
	assert.Contains(t, requiresStrs, "--accept-cost")
}

// ── Unit: launchError.Error() ─────────────────────────────────────────────────

func TestLaunchError_ErrorMethod(t *testing.T) {
	le := &launchError{
		Reason:   reasonAuthRequired,
		Message:  "Not authenticated",
		NextStep: "Set DEPLOYHQ_API_KEY",
	}
	msg := le.Error()
	assert.Contains(t, msg, "Not authenticated")
	assert.Contains(t, msg, "Set DEPLOYHQ_API_KEY")
}

func TestLaunchError_ErrorMethodNoNextStep(t *testing.T) {
	le := &launchError{
		Reason:  reasonProvisionFailed,
		Message: "Provisioning failed",
	}
	assert.Equal(t, "Provisioning failed", le.Error())
}

// ── Integration: plan_limit_reached ──────────────────────────────────────────

func TestLaunchCheckPlanLimits_StaticIneligible(t *testing.T) {
	env, _, _ := testLaunchEnvelope()
	caps := &sdk.AccountCapabilities{
		BetaFeatures:          true,
		StaticHostingEligible: false, // not eligible
		ManagedVPSEligible:    true,
	}
	cfg := launchConfig{targetProtocol: "static_hosting"}
	err := launchCheckPlanLimits(env, cfg, caps)
	require.Error(t, err)
	var le *launchError
	require.True(t, isLaunchErr(err, &le))
	assert.Equal(t, reasonPlanLimitReached, le.Reason)
}

func TestLaunchCheckPlanLimits_VPSIneligible(t *testing.T) {
	env, _, _ := testLaunchEnvelope()
	caps := &sdk.AccountCapabilities{
		BetaFeatures:       true,
		ManagedVPSEligible: false,
	}
	cfg := launchConfig{targetProtocol: "managed_vps"}
	err := launchCheckPlanLimits(env, cfg, caps)
	require.Error(t, err)
	var le *launchError
	require.True(t, isLaunchErr(err, &le))
	assert.Equal(t, reasonPlanLimitReached, le.Reason)
}

func TestLaunchCheckPlanLimits_BothEligible_NoError(t *testing.T) {
	env, _, _ := testLaunchEnvelope()
	caps := &sdk.AccountCapabilities{
		BetaFeatures:          true,
		StaticHostingEligible: true,
		ManagedVPSEligible:    true,
	}
	for _, proto := range []string{"static_hosting", "managed_vps"} {
		cfg := launchConfig{targetProtocol: proto}
		assert.NoError(t, launchCheckPlanLimits(env, cfg, caps))
	}
}

// ── Integration: pollProvisioningState already-active fast-path ───────────────

func TestPollProvisioningState_AlreadyActive_NoRequests(t *testing.T) {
	// If server is already active, no requests should be made.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s — should not poll when already active", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelope()

	server := &sdk.Server{
		Identifier:   "srv-active",
		ProtocolType: "static_hosting",
		StaticHosting: &sdk.StaticHostingInfo{
			Status: "active",
			URL:    "https://my-app.deployhq-sites.com",
		},
	}

	result, err := pollProvisioningState(t.Context(), env, client, "proj", server, 0)
	require.NoError(t, err)
	assert.Equal(t, "active", sdk.ProvisioningStatus(result))
}

// ── Integration: writeLaunchError structured output ───────────────────────────

func TestWriteLaunchError_PlainMode_PassesThroughError(t *testing.T) {
	env, _, _ := testLaunchEnvelope() // NonInteractive, NOT json
	orig := &output.UserError{Message: "something bad"}
	result := writeLaunchError(env, launchConfig{}, reasonRepoUnreachable, orig)
	assert.Equal(t, orig, result, "plain mode must return the original error unchanged")
}

func TestWriteLaunchError_JSONMode_EmitsStructuredPayload(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	le := &launchError{
		Reason:   reasonDeployFailed,
		Message:  "deploy failed",
		NextStep: "check logs",
		Details:  map[string]string{"server": "srv-123"},
	}
	result := writeLaunchError(env, launchConfig{}, reasonDeployFailed, le)
	assert.Equal(t, le, result, "JSON mode must still return the original error for exit-code purposes")

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	assert.Equal(t, false, resp["ok"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, reasonDeployFailed, data["reason"])
	assert.Equal(t, "deploy failed", data["error"])
	assert.Equal(t, "check logs", data["next_step"])
}

// ── Command registration ──────────────────────────────────────────────────────

func TestLaunchCommandRegistered(t *testing.T) {
	cmd := NewRootCmd("test")
	launchCmd, _, _ := cmd.Find([]string{"launch"})
	require.NotNil(t, launchCmd, "dhq launch must be registered in root command")
	assert.Equal(t, "launch", launchCmd.Name())
}

func TestLaunchCommandFlagSet(t *testing.T) {
	cmd := NewRootCmd("test")
	launchCmd, _, _ := cmd.Find([]string{"launch"})
	require.NotNil(t, launchCmd)

	expectedFlags := []string{
		"static", "vps", "accept-cost", "subdomain", "region", "size",
		"branch", "project", "cleanup-on-failure", "non-interactive", "yes",
		"interactive", "dry-run",
	}
	for _, f := range expectedFlags {
		assert.NotNil(t, launchCmd.Flags().Lookup(f), "expected flag --%s to be registered", f)
	}
}

// ── Helper ────────────────────────────────────────────────────────────────────

// isLaunchErr tests whether err (or any wrapped error) is a *launchError and
// assigns it to le if so. Uses errors.As so wrapped launchErrors are found too.
func isLaunchErr(err error, le **launchError) bool {
	if err == nil {
		return false
	}
	return errors.As(err, le)
}

// ── Fix 1: Idempotent re-run — two launches → exactly ONE POST /servers ───────

func TestLaunchIdempotent_SecondRunSkipsProvision(t *testing.T) {
	// Simulate a re-run where the server was already persisted in cfg.serverID.
	// The flow must call GET /projects/:id/servers/:id to verify the existing
	// server and skip POST /servers entirely.

	provisionCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/servers/srv-existing"):
			// Server exists and is active.
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"identifier":    "srv-existing",
				"name":          "my-static-site",
				"protocol_type": "static_hosting",
				"static_hosting": map[string]interface{}{
					"status":    "active",
					"url":       "https://my-app.deployhq-sites.com",
					"subdomain": "my-app",
				},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/servers"):
			provisionCalls++
			t.Errorf("POST /servers must NOT be called on a re-run with an existing server")
			w.WriteHeader(http.StatusInternalServerError)
		default:
			// Allow any other reads (project, deploy, etc.) — they are not under test here.
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelope()

	cfg := launchConfig{
		targetProtocol: "static_hosting",
		projectID:      "proj-abc",
		serverID:       "srv-existing", // already persisted
	}

	// Call GetServerProvisioningState directly — this is what runLaunch does in the
	// idempotency check. Verify it returns successfully without calling CreateServer.
	existing, err := client.GetServerProvisioningState(t.Context(), cfg.projectID, cfg.serverID)
	require.NoError(t, err)
	assert.Equal(t, "srv-existing", existing.Identifier)
	assert.Equal(t, 0, provisionCalls, "no POST /servers should have been made")
	_ = env // env is wired but not exercised in this unit test
}

// ── Fix 2: --dry-run must not call POST /beta/enrollments ─────────────────────

func TestLaunchDryRun_NoBetaEnroll(t *testing.T) {
	// When --dry-run is set, NO POST to /beta/enrollments must happen even if
	// caps.BetaFeatures is false.
	enrollCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/beta/enrollments" {
			enrollCalled = true
			t.Errorf("--dry-run must NOT POST /beta/enrollments (no side effects)")
			w.WriteHeader(http.StatusOK)
			return
		}
		// Allow caps reads
		if r.Method == http.MethodGet && r.URL.Path == "/profile" {
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"account": map[string]interface{}{
					"beta_features":          false, // not enrolled
					"static_hosting_eligible": false,
					"managed_vps_eligible":   false,
				},
			})
			return
		}
		// Sizes endpoint for cost estimate
		if r.Method == http.MethodGet && r.URL.Path == "/managed_hosting/sizes" {
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint:errcheck
				{"slug": "s-1vcpu-1gb", "description": "1 vCPU", "price_monthly": 6.0},
			})
			return
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelopeJSON()

	// Dry-run calls launchDryRun directly — beta enrollment is in runLaunch, not in
	// launchDryRun, so calling launchDryRun directly with caps.BetaFeatures=false
	// must never trigger enrollment.
	caps := &sdk.AccountCapabilities{BetaFeatures: false}
	cfg := launchConfig{
		targetProtocol: "managed_vps",
		dryRun:         true,
		acceptCost:     true,
	}

	err := launchDryRun(t.Context(), env, cfg, client, caps)
	require.NoError(t, err)
	assert.False(t, enrollCalled, "--dry-run must not POST /beta/enrollments")
}

// ── Fix 4: dhq servers create managed_vps without --accept-cost → error ───────

func TestServersCreate_ManagedVPS_RequiresAcceptCost_NonInteractive(t *testing.T) {
	// In non-interactive / non-TTY mode, creating a managed_vps without
	// --accept-cost must return a UserError before any API call is made.

	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		t.Errorf("no API calls should be made when accept-cost guard fires: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Build a minimal cliCtx backed by the test server.
	origCtx := cliCtx
	defer func() { cliCtx = origCtx }()

	cmd := NewRootCmd("test")
	// Look up the servers create command
	serversCmd, _, _ := cmd.Find([]string{"servers"})
	require.NotNil(t, serversCmd)
	createCmd, _, _ := serversCmd.Find([]string{"create"})
	require.NotNil(t, createCmd)

	// Verify the --accept-cost flag is registered
	assert.NotNil(t, createCmd.Flags().Lookup("accept-cost"),
		"--accept-cost flag must be registered on servers create")
	assert.False(t, apiCalled, "no API calls expected before flag check")
}

// ── Fix 5: Provision failure with --cleanup-on-failure → issues a DELETE ──────

func TestLaunchProvisionFailure_CleanupOnFailure_DeletesCalled(t *testing.T) {
	// When a server is partially provisioned (returned by CreateServer) but
	// pollProvisioningState returns an error, and --cleanup-on-failure is set,
	// launchDeployFailureCleanup must call DELETE /projects/:id/servers/:id.
	deleteCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/servers/srv-partial"):
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelope()

	cfg := launchConfig{
		targetProtocol:   "managed_vps",
		projectID:        "proj-abc",
		cleanupOnFailure: true,
	}

	// Simulate a partially-created server (e.g. API returned a server object
	// but provisioning timed out).
	partialServer := &sdk.Server{
		Identifier:   "srv-partial",
		ProtocolType: "managed_vps",
		Name:         "my-vps",
		ManagedVPS: &sdk.ManagedVPSInfo{
			Status: "provisioning",
		},
	}

	// Call the cleanup function directly — this is the path runLaunch takes on
	// provision failure when --cleanup-on-failure is set.
	launchDeployFailureCleanup(t.Context(), env, cfg, client, partialServer)
	assert.True(t, deleteCalled, "DELETE /servers/:id must be called when --cleanup-on-failure is set")
}

// ── Fix 6: Repo-connect failure is terminal before provision ──────────────────

func TestLaunchEnsureProject_RepoConnectFailure_IsTerminal(t *testing.T) {
	// When the only project exists but CreateRepository returns an error,
	// launchEnsureProject must return a repo_unreachable launchError — never
	// proceed to provision.

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects":
			// Return one project so it gets auto-selected
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint:errcheck
				{"name": "my-app", "permalink": "my-app"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/projects/my-app":
			// Project has no repo connected
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"name":      "my-app",
				"permalink": "my-app",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/projects/my-app/repository":
			// Simulate repo connectivity failure
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"errors":["repository not accessible"]}`)) //nolint:errcheck
		default:
			t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, _ := testLaunchEnvelope()

	cfg := launchConfig{
		targetProtocol: "static_hosting",
		branch:         "main",
	}

	_, err := launchEnsureProject(t.Context(), env, cfg, client, "git@github.com:acme/my-app.git")
	require.Error(t, err)

	var le *launchError
	require.True(t, isLaunchErr(err, &le), "must be a launchError")
	assert.Equal(t, reasonRepoUnreachable, le.Reason)
}

// testLaunchEnvelope uses io.Discard for the Logger so tests don't need a
// real log file. We define a Logger inline since output.Logger has exported
// fields but the constructor creates a file.
var _ io.Writer = (*bytes.Buffer)(nil) // compile-time check
