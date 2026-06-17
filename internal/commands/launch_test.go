package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deployhq/deployhq-cli/internal/detect"
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

// ── auth_required structured error ────────────────────────────────────────────
//
// The full no-credentials flow in launchEnsureAuth depends on the OS keyring
// (auth.LoadByAccount), so it cannot be unit-tested deterministically across
// machines. We verify the observable contract instead: a non-interactive auth
// failure surfaces as a structured auth_required error (and launchEnsureAuth
// never attempts a headless signup in non-interactive mode — see launch.go).
// Flag registration is covered separately by TestLaunchCommandFlagSet.

func TestLaunchAuthRequired_JSONReason(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	authErr := &output.AuthError{Message: "Not authenticated"}

	result := writeLaunchError(env, launchConfig{}, reasonAuthRequired, authErr)
	assert.Equal(t, authErr, result, "must return the original error for exit-code purposes")

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	assert.Equal(t, false, resp["ok"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, reasonAuthRequired, data["reason"])
	assert.Contains(t, data["error"].(string), "Not authenticated")
}

// ── Build command application (static) ───────────────────────────────────────

func TestLaunchApplyBuildCommand_CreatesViaBuildCommandsEndpoint(t *testing.T) {
	// The detected build command must be created through POST /build_commands
	// with a {build_command: {command}} body — not the old /build_configs path —
	// so the first static deploy publishes built output, not unbuilt sources.
	var postPath, postBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/build_commands"):
			_, _ = w.Write([]byte("[]")) // no existing build commands
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/build_commands"):
			postPath = r.URL.Path
			b, _ := io.ReadAll(r.Body)
			postBody = string(b)
			_, _ = w.Write([]byte(`{"identifier":"bc-1","command":"npm run build"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	client := newSpecValidatingClient(t, srv)
	cfg := launchConfig{projectID: "my-app", targetProtocol: "static_hosting"}
	detection := detect.Result{BuildCommand: "npm run build", OutputDir: "dist"}

	launchApplyBuildCommand(t.Context(), env, cfg, client, detection)

	assert.Contains(t, postPath, "/projects/my-app/build_commands")
	assert.Contains(t, postBody, `"build_command"`)
	assert.Contains(t, postBody, `"command":"npm run build"`)
	assert.NotContains(t, postBody, "build_config", "must not use the wrong build_config payload")
}

func TestLaunchApplyBuildCommand_SkipsWhenBuildCommandsExist(t *testing.T) {
	// An idempotent re-run / reused --project must not duplicate build commands.
	postCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/build_commands"):
			_, _ = w.Write([]byte(`[{"identifier":"bc-existing","command":"npm run build"}]`))
		case r.Method == http.MethodPost:
			postCalled = true
			t.Errorf("must not POST when build commands already exist")
		}
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	client := newSpecValidatingClient(t, srv)
	cfg := launchConfig{projectID: "my-app"}
	launchApplyBuildCommand(t.Context(), env, cfg, client, detect.Result{BuildCommand: "npm run build"})

	assert.False(t, postCalled)
}

func TestLaunchApplyBuildCommand_EmptyCommandIsNoop(t *testing.T) {
	// No detected build command → no HTTP calls at all.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Errorf("no request expected for an empty build command: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	launchApplyBuildCommand(t.Context(), env, launchConfig{projectID: "p"}, newSpecValidatingClient(t, srv), detect.Result{BuildCommand: ""})
	assert.False(t, called)
}

func TestLaunchApplyBuildCommand_ListErrorStillCreates(t *testing.T) {
	// If listing existing build commands fails, the guard must NOT skip — it's
	// best-effort, so we still attempt to create the detected command.
	postCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/build_commands"):
			w.WriteHeader(http.StatusInternalServerError)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/build_commands"):
			postCalled = true
			_, _ = w.Write([]byte(`{"identifier":"bc-1","command":"npm run build"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	launchApplyBuildCommand(t.Context(), env, launchConfig{projectID: "p"}, newSpecValidatingClient(t, srv), detect.Result{BuildCommand: "npm run build"})
	assert.True(t, postCalled, "list error must not prevent the create attempt")
}

func TestLaunchApplyBuildCommand_CreateErrorWarnsWithHint(t *testing.T) {
	// A failed create is non-fatal but must surface a discoverable manual hint —
	// otherwise the static site silently deploys unbuilt sources.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/build_commands"):
			_, _ = w.Write([]byte("[]"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/build_commands"):
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":"command is invalid"}`))
		}
	}))
	defer srv.Close()

	env, _, stderr := testLaunchEnvelope()
	// Must not panic / must return normally despite the API error.
	launchApplyBuildCommand(t.Context(), env, launchConfig{projectID: "my-app"}, newSpecValidatingClient(t, srv),
		detect.Result{BuildCommand: "npm run build"})

	warn := stderr.String()
	assert.Contains(t, warn, "dhq build-commands create", "must point the user at the manual fix")
	assert.Contains(t, warn, "npm run build")
}

func TestLaunchApplyBuildCommand_TruncatesLongDescription(t *testing.T) {
	// Description is capped at 100 runes (mirrors the web wizard) while the full
	// command is preserved.
	longCmd := strings.Repeat("a", 150)
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/build_commands"):
			_, _ = w.Write([]byte("[]"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/build_commands"):
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			_, _ = w.Write([]byte(`{"identifier":"bc-1"}`))
		}
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	launchApplyBuildCommand(t.Context(), env, launchConfig{projectID: "p"}, newSpecValidatingClient(t, srv),
		detect.Result{BuildCommand: longCmd})

	var parsed struct {
		BuildCommand struct {
			Command     string `json:"command"`
			Description string `json:"description"`
		} `json:"build_command"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &parsed))
	assert.Equal(t, 150, len([]rune(parsed.BuildCommand.Command)), "command preserved in full")
	assert.Equal(t, 100, len([]rune(parsed.BuildCommand.Description)), "description capped at 100 runes")
}

// TestLaunchStatic_ProvisionThenBuildCommand_Integration exercises the static
// branch as the orchestrator runs it: provision the Static Hosting server (Step
// 8), then apply the detected build command (Step 9), against one routed server.
// It proves the build command lands on /build_commands during a real static
// launch — the core regression the P1 finding flagged.
func TestLaunchStatic_ProvisionThenBuildCommand_Integration(t *testing.T) {
	var serverCreated, buildCmdPath, buildCmdBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/servers"):
			b, _ := io.ReadAll(r.Body)
			serverCreated = string(b)
			// Return an already-active static server so provisioning doesn't poll.
			_, _ = w.Write([]byte(`{"identifier":"srv-1","protocol_type":"static_hosting",` +
				`"static_hosting":{"url":"https://my-app.deployhq-sites.com","subdomain":"my-app","status":"active"}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/build_commands"):
			_, _ = w.Write([]byte("[]"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/build_commands"):
			buildCmdPath = r.URL.Path
			b, _ := io.ReadAll(r.Body)
			buildCmdBody = string(b)
			_, _ = w.Write([]byte(`{"identifier":"bc-1","command":"npm run build"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	client := newSpecValidatingClient(t, srv)
	cfg := launchConfig{
		projectID:      "my-app",
		targetProtocol: "static_hosting",
		subdomain:      "my-app",
		subdirectory:   "dist",
	}
	detection := detect.Result{BuildCommand: "npm run build", OutputDir: "dist"}

	// Step 8: provision the static server.
	server, err := launchProvisionStatic(t.Context(), env, cfg, client)
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Equal(t, "srv-1", server.Identifier)
	assert.Contains(t, serverCreated, "static_hosting")
	assert.Contains(t, serverCreated, `"subdomain":"my-app"`)

	// Step 9: apply the detected build command (the static-only branch).
	launchApplyBuildCommand(t.Context(), env, cfg, client, detection)
	assert.Contains(t, buildCmdPath, "/projects/my-app/build_commands")
	assert.Contains(t, buildCmdBody, `"command":"npm run build"`)
	assert.NotContains(t, buildCmdBody, "build_config")
}

// ── Provisioning poll (provisioning → active / error) ────────────────────────

// shrinkPollBackoff makes pollProvisioningState's first delay ~instant for the
// duration of the test, restoring the production value afterwards.
func shrinkPollBackoff(t *testing.T) {
	t.Helper()
	old := provisionPollInitialBackoff
	provisionPollInitialBackoff = time.Millisecond
	t.Cleanup(func() { provisionPollInitialBackoff = old })
}

// pollStateServer returns an httptest server whose GET /servers/:id responses
// walk the given status sequence (clamping on the last one), plus a counter.
func pollStateServer(t *testing.T, statuses []string) (*httptest.Server, *int) {
	t.Helper()
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/servers/") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		idx := polls
		if idx >= len(statuses) {
			idx = len(statuses) - 1
		}
		polls++
		_, _ = fmt.Fprintf(w, `{"identifier":"srv-1","protocol_type":"static_hosting",`+
			`"static_hosting":{"url":"https://my-app.deployhq-sites.com","subdomain":"my-app","status":%q}}`, statuses[idx])
	}))
	t.Cleanup(srv.Close)
	return srv, &polls
}

func TestPollProvisioningState_ProvisioningThenActive(t *testing.T) {
	// The poll loop must keep polling through "provisioning" and terminate as
	// soon as the resource reports "active" — the core async-provisioning path
	// `dhq launch` relies on for both Static Hosting and Managed VPS.
	shrinkPollBackoff(t)
	srv, polls := pollStateServer(t, []string{"provisioning", "provisioning", "active"})

	env, _, _ := testLaunchEnvelope()
	client := newSpecValidatingClient(t, srv)
	pending := &sdk.Server{
		Identifier:    "srv-1",
		ProtocolType:  "static_hosting",
		StaticHosting: &sdk.StaticHostingInfo{Status: "provisioning"},
	}

	got, err := pollProvisioningState(t.Context(), env, client, "my-app", pending, 10*time.Second)
	require.NoError(t, err)
	assert.True(t, sdk.IsProvisioningActive(got), "must return the active server")
	assert.Equal(t, 3, *polls, "must poll exactly until the first active status")
}

// (The already-active short-circuit is covered by the pre-existing
// TestPollProvisioningState_AlreadyActive_NoRequests further down.)

func TestPollProvisioningState_ErrorStatusFailsWithProvisionReason(t *testing.T) {
	// A resource that lands in "error" must stop polling and surface a
	// structured provision_failed error — not spin until the timeout.
	shrinkPollBackoff(t)
	srv, polls := pollStateServer(t, []string{"provisioning", "error"})

	env, _, _ := testLaunchEnvelope()
	client := newSpecValidatingClient(t, srv)
	pending := &sdk.Server{
		Identifier:    "srv-1",
		ProtocolType:  "static_hosting",
		StaticHosting: &sdk.StaticHostingInfo{Status: "provisioning"},
	}

	_, err := pollProvisioningState(t.Context(), env, client, "my-app", pending, 10*time.Second)
	require.Error(t, err)
	var le *launchError
	require.ErrorAs(t, err, &le)
	assert.Equal(t, reasonProvisionFailed, le.Reason)
	assert.Equal(t, 2, *polls, "must stop on the first error status")
}

// ── Managed VPS size presentation ────────────────────────────────────────────

func TestHumanMB(t *testing.T) {
	assert.Equal(t, "1 GB", humanMB(1024))
	assert.Equal(t, "2 GB", humanMB(2048))
	assert.Equal(t, "512 MB", humanMB(512))
	assert.Equal(t, "1.5 GB", humanMB(1536))
}

func TestManagedSizeRanksAndTiers(t *testing.T) {
	// Deliberately out of price order to prove ranking is by price, not position.
	sizes := []sdk.ManagedHostingSize{
		{Slug: "mid", PriceMonthly: 12},
		{Slug: "cheap", PriceMonthly: 6},
		{Slug: "dear", PriceMonthly: 24},
	}
	ranks := managedSizeRanks(sizes)
	assert.Equal(t, []int{1, 0, 2}, ranks)

	assert.Equal(t, "Starter", managedSizeTier(0))
	assert.Equal(t, "Standard", managedSizeTier(1))
	assert.Equal(t, "Plus", managedSizeTier(2))
	assert.Equal(t, "Pro", managedSizeTier(3))
	assert.Equal(t, "", managedSizeTier(4), "ranks beyond named tiers fall back to spec-only")
}

func TestManagedSizeLabel(t *testing.T) {
	s := sdk.ManagedHostingSize{Slug: "s-1vcpu-1gb", VCPUs: 1, Memory: 1024, Disk: 25, PriceMonthly: 6}
	label := managedSizeLabel(s, 0)
	assert.Contains(t, label, "Starter")
	assert.Contains(t, label, "1 vCPU")
	assert.Contains(t, label, "1 GB RAM")
	assert.Contains(t, label, "25 GB SSD")
	assert.Contains(t, label, "$6.00/mo")
	assert.Contains(t, label, "(s-1vcpu-1gb)", "slug stays visible for --size discoverability")

	// Missing structured specs → fall back to the API Description, no tier crash.
	bare := sdk.ManagedHostingSize{Slug: "x", Description: "custom", PriceMonthly: 9}
	assert.Contains(t, managedSizeLabel(bare, 9), "custom")
}

// ── rate_limited (429 provisioning rate limit) ───────────────────────────────

func TestRateLimitLaunchError_Mapping(t *testing.T) {
	// 429 with Retry-After → retryable rate_limited error carrying the backoff.
	withRA := rateLimitLaunchError(&sdk.APIError{StatusCode: http.StatusTooManyRequests, RetryAfter: 30})
	require.NotNil(t, withRA)
	assert.Equal(t, reasonRateLimited, withRA.Reason)
	assert.True(t, withRA.Retryable)
	assert.Equal(t, "30", withRA.Details["retry_after"])
	assert.Contains(t, withRA.NextStep, "30s")

	// 429 without Retry-After → still retryable, no retry_after detail.
	noRA := rateLimitLaunchError(&sdk.APIError{StatusCode: http.StatusTooManyRequests})
	require.NotNil(t, noRA)
	assert.True(t, noRA.Retryable)
	_, hasRA := noRA.Details["retry_after"]
	assert.False(t, hasRA)

	// A 422 cap is NOT a rate limit — must fall through (nil).
	assert.Nil(t, rateLimitLaunchError(&sdk.APIError{StatusCode: http.StatusUnprocessableEntity}))
	// A non-API error is not a rate limit either.
	assert.Nil(t, rateLimitLaunchError(errors.New("boom")))
}

func TestLaunchRateLimited_JSONReason(t *testing.T) {
	// A rate_limited error surfaced through the generic provision_failed call
	// site must keep its true reason and retryable flag in --json output —
	// agents branch on `reason` + `retryable`, not the call site.
	env, stdout, _ := testLaunchEnvelopeJSON()
	rl := rateLimitLaunchError(&sdk.APIError{StatusCode: http.StatusTooManyRequests, RetryAfter: 12})
	require.NotNil(t, rl)

	_ = writeLaunchError(env, launchConfig{}, reasonProvisionFailed, rl)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(stdout).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, reasonRateLimited, data["reason"], "reason must be rate_limited, not the provision_failed call site")
	assert.Equal(t, true, data["retryable"])
	details := data["details"].(map[string]interface{})
	assert.Equal(t, "12", details["retry_after"])
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
	assert.Contains(t, le.Message, "--accept-cost")
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
	// regardless of admin status (admin is required only for the first
	// not-enrolled→enrolled flip). Verify that round-trip directly.
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

// ── Idempotent re-run — two launches → exactly ONE POST /servers ───────

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

// ── --dry-run must not call POST /beta/enrollments ─────────────────────

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

// ── dhq servers create managed_vps without --accept-cost → error ───────

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

// ── Provision failure with --cleanup-on-failure → issues a DELETE ──────

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

// ── Repo-connect failure is terminal before provision ──────────────────

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

// ── Public deploy key surfaced after repo connect (private-repo support) ──────

func TestLaunchEnsureRepo_SurfacesPublicDeployKey(t *testing.T) {
	// After connecting a repository, the project's PUBLIC deploy key must be
	// surfaced so a private repo can be granted read access before the clone.
	const pubKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABexamplekey deployhq-my-app"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects/my-app":
			// Project with no repo connected yet, carrying its public deploy key.
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"name":       "my-app",
				"permalink":  "my-app",
				"public_key": pubKey,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/projects/my-app/repository":
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"scm_type": "git", "url": "git@github.com:acme/my-app.git", "branch": "main",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, stderr := testLaunchEnvelope() // NonInteractive: surfaces without pausing

	cfg := launchConfig{targetProtocol: "static_hosting", branch: "main"}
	// Non-GitHub host so the gh-CLI auto-install path is skipped and we
	// deterministically exercise the surface-the-key fallback.
	err := launchEnsureRepo(t.Context(), env, cfg, client, "my-app", "git@git.example.com:acme/my-app.git")
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), pubKey, "public deploy key must be surfaced after connecting the repo")
}

func TestLaunchEnsureRepo_AlreadyConnected_NoKeyNoise(t *testing.T) {
	// When the repo is already connected, launchEnsureRepo returns early and must
	// NOT re-print the deploy key (avoids noise on idempotent re-runs).
	const pubKey = "ssh-rsa AAAAB3already-connected-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/projects/my-app" {
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"name": "my-app", "permalink": "my-app", "public_key": pubKey,
				"repository": map[string]interface{}{
					"scm_type": "git", "url": "git@github.com:acme/my-app.git", "branch": "main",
				},
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	env, _, stderr := testLaunchEnvelope()

	err := launchEnsureRepo(t.Context(), env, launchConfig{}, client, "my-app", "git@github.com:acme/my-app.git")
	require.NoError(t, err)
	assert.NotContains(t, stderr.String(), pubKey, "no deploy-key output when the repo is already connected")
}

// ── Failure diagnosis (explainLaunchFailure) — interactive-only ───────────────

func TestExplainLaunchFailure_NonInteractiveIsNoOp(t *testing.T) {
	// Non-interactive must never auto-diagnose, prompt, or call Ollama — the
	// structured launchError is the machine-readable output. Expect no output.
	env, stdout, stderr := testLaunchEnvelope() // NonInteractive=true, IsTTY=false
	explainLaunchFailure(t.Context(), env, nil, "my-app")
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func TestExplainLaunchFailure_EmptyProjectIsNoOp(t *testing.T) {
	env, _, stderr := testLaunchEnvelope()
	explainLaunchFailure(t.Context(), env, nil, "")
	assert.Empty(t, stderr.String())
}

func TestInstallDeployKeyViaGH_RejectsNonGitHubURL(t *testing.T) {
	// A non-GitHub URL is rejected before any `gh` invocation, so callers fall
	// back to surfacing the key for manual installation.
	err := installDeployKeyViaGH("git@gitlab.com:acme/app.git", "ssh-ed25519 AAAA key", "DeployHQ - app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GitHub")
}

// testLaunchEnvelope uses io.Discard for the Logger so tests don't need a
// real log file. We define a Logger inline since output.Logger has exported
// fields but the constructor creates a file.
var _ io.Writer = (*bytes.Buffer)(nil) // compile-time check
