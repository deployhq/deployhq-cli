package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	launchDomainProjectID = "my-app"
	launchDomainServerID  = "srv-1"
	launchDomainsPath     = "/projects/my-app/servers/srv-1/custom_domains"
	launchDomainLiveURL   = "https://my-app.deployhq-sites.com"
)

// launchActiveServerJSON is a create response for an already-active site, with
// extra fields spliced in (e.g. the custom domain outcome).
func launchActiveServerJSON(extra string) string {
	return `{"identifier":"srv-1","name":"my-app","protocol_type":"static_hosting",` +
		`"static_hosting":{"url":"https://my-app.deployhq-sites.com","subdomain":"my-app","status":"active"}` + extra + `}`
}

func launchStaticServer() *sdk.Server {
	return &sdk.Server{
		Identifier:    launchDomainServerID,
		Name:          "my-app",
		ProtocolType:  "static_hosting",
		StaticHosting: &sdk.StaticHostingInfo{URL: launchDomainLiveURL, Status: "active"},
	}
}

func launchDomainCfg(hostname string) launchConfig {
	return launchConfig{projectID: launchDomainProjectID, targetProtocol: "static_hosting", subdomain: "my-app", customDomain: hostname}
}

// launchDomainServer answers one 422/POST response for server create and
// records the request body. Any other request fails the test.
func launchDomainServer(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/servers") {
			raw, _ := io.ReadAll(r.Body)
			received = string(raw)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &received
}

// launchDomainListServer answers the custom domains list with the given
// bodies in order (the last repeats) and counts the calls.
func launchDomainListServer(t *testing.T, bodies ...string) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == launchDomainsPath {
			idx := calls
			if idx >= len(bodies) {
				idx = len(bodies) - 1
			}
			calls++
			_, _ = w.Write([]byte(bodies[idx]))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func launchDomainFromJSON(t *testing.T, raw string) sdk.CustomDomain {
	t.Helper()
	var d sdk.CustomDomain
	require.NoError(t, json.Unmarshal([]byte(raw), &d))
	return d
}

// ── Request ───────────────────────────────────────────────────────────────────

func TestLaunchStatic_SendsCustomDomainAttributes(t *testing.T) {
	srv, body := launchDomainServer(t, http.StatusCreated, launchActiveServerJSON(`,"custom_domains":[],"custom_domain_errors":[]`))
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("example.com")
	cfg.withWWW = true

	_, err := launchProvisionStatic(t.Context(), env, &cfg, newTestClient(t, srv))
	require.NoError(t, err)

	var payload struct {
		Attrs map[string]any `json:"hosted_website_attributes"`
	}
	require.NoError(t, json.Unmarshal([]byte(*body), &payload))
	assert.Equal(t, "example.com", payload.Attrs["custom_domain_hostname"])
	assert.Equal(t, true, payload.Attrs["custom_domain_with_counterpart"])
}

func TestLaunchStatic_OmitsCustomDomainAttributesWhenNotRequested(t *testing.T) {
	srv, body := launchDomainServer(t, http.StatusCreated, launchActiveServerJSON(""))
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("")

	_, err := launchProvisionStatic(t.Context(), env, &cfg, newTestClient(t, srv))
	require.NoError(t, err)
	assert.NotContains(t, *body, "custom_domain")
}

// The poll replaces the server with GET /servers/:id, which has no domains: the
// create response's outcome must survive it.
func TestLaunchStatic_KeepsCreateResponseDomainsThroughProvisioningPoll(t *testing.T) {
	shrinkPollBackoff(t)
	created := `{"identifier":"srv-1","protocol_type":"static_hosting","static_hosting":{"url":"https://my-app.deployhq-sites.com","status":"provisioning"},` +
		`"custom_domains":[` + registeringDomainJSON + `],"custom_domain_errors":["counterpart problem"]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/servers"):
			_, _ = w.Write([]byte(created))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/servers/srv-1"):
			_, _ = w.Write([]byte(launchActiveServerJSON("")))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("www.example.com")
	server, err := launchProvisionStatic(t.Context(), env, &cfg, newTestClient(t, srv))
	require.NoError(t, err)
	require.Len(t, server.CustomDomains, 1)
	assert.Equal(t, "www.example.com", server.CustomDomains[0].Hostname)
	assert.Equal(t, []string{"counterpart problem"}, server.CustomDomainErrors)
}

// ── Invalid hostname (422) ────────────────────────────────────────────────────

func TestLaunchStatic_InvalidCustomDomain_NonInteractive(t *testing.T) {
	cases := []struct{ name, apiError string }{
		{"format", "Custom domain is not a valid domain name"},
		// Contains the word "subdomain": it must not be filed as subdomain_taken.
		{"blocked domain", "Custom domain may not be deployhq-sites.com or one of its subdomains"},
		{"taken", "Custom domain is already in use"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := launchDomainServer(t, http.StatusUnprocessableEntity, fmt.Sprintf(`{"base":[%q]}`, tc.apiError))
			env, _, _ := testLaunchEnvelope()
			cfg := launchDomainCfg("bad.example")

			server, err := launchProvisionStatic(t.Context(), env, &cfg, newTestClient(t, srv))
			require.Error(t, err)
			assert.Nil(t, server, "a 422 created no server")

			var le *launchError
			require.ErrorAs(t, err, &le)
			assert.Equal(t, reasonCustomDomainInvalid, le.Reason)
			assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
			assert.False(t, le.Retryable)
			assert.Contains(t, le.Message, tc.apiError, "the API's reason reaches the caller")
			assert.Contains(t, le.Message, "bad.example")
			assert.NotContains(t, le.Message, "base ", "the field name is not part of the message")
			assert.Contains(t, le.NextStep, "--custom-domain")
			assert.Equal(t, "bad.example", le.Details["custom_domain"])
		})
	}
}

// A taken subdomain on a launch that also asked for a domain is still reported
// as the subdomain, not the domain.
func TestLaunchStatic_SubdomainTakenStillReportedWithCustomDomain(t *testing.T) {
	srv, _ := launchDomainServer(t, http.StatusUnprocessableEntity, `{"base":["Subdomain has already been taken"]}`)
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("example.com")

	_, err := launchProvisionStatic(t.Context(), env, &cfg, newTestClient(t, srv))
	var le *launchError
	require.ErrorAs(t, err, &le)
	assert.Equal(t, reasonSubdomainTaken, le.Reason)
}

// ── Static-only refusal and flag validation ──────────────────────────────────

func TestValidateLaunchCustomDomain(t *testing.T) {
	cases := []struct {
		name    string
		cfg     launchConfig
		wantErr string
		want    launchConfig
	}{
		{"no domain, no check", launchConfig{targetProtocol: "managed_vps"}, "", launchConfig{targetProtocol: "managed_vps"}},
		{"vps refused", launchConfig{targetProtocol: "managed_vps", customDomain: "example.com"}, "only applies to Static Hosting", launchConfig{}},
		{"with-www alone refused", launchConfig{withWWW: true}, "--with-www", launchConfig{}},
		{"url refused", launchConfig{customDomain: "https://example.com"}, "is not a hostname", launchConfig{}},
		{"path refused", launchConfig{customDomain: "example.com/about"}, "is not a hostname", launchConfig{}},
		{"normalised", launchConfig{customDomain: " Example.COM. ", targetProtocol: "static_hosting"}, "", launchConfig{customDomain: "example.com", targetProtocol: "static_hosting"}},
		{"static ok", launchConfig{customDomain: "example.com", targetProtocol: "static_hosting"}, "", launchConfig{customDomain: "example.com", targetProtocol: "static_hosting"}},
		{"undecided ok", launchConfig{customDomain: "example.com"}, "", launchConfig{customDomain: "example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			err := validateLaunchCustomDomain(&cfg)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want.customDomain, cfg.customDomain)
		})
	}
}

// A VPS chosen by detection (or a previous launch's config) is refused once the
// target is known, and an undecided target becomes Static Hosting.
func TestResolveLaunchCustomDomainTarget(t *testing.T) {
	vps := launchConfig{customDomain: "example.com", targetProtocol: "managed_vps"}
	err := resolveLaunchCustomDomainTarget(&vps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only applies to Static Hosting")
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
	var ue *output.UserError
	require.ErrorAs(t, err, &ue)
	assert.Contains(t, ue.Hint, "--static")

	undecided := launchConfig{customDomain: "example.com"}
	require.NoError(t, resolveLaunchCustomDomainTarget(&undecided))
	assert.Equal(t, "static_hosting", undecided.targetProtocol)

	none := launchConfig{targetProtocol: "managed_vps"}
	require.NoError(t, resolveLaunchCustomDomainTarget(&none))
	assert.Equal(t, "managed_vps", none.targetProtocol)
}

func TestLaunchCommand_VPSWithCustomDomainRefusedBeforeAnyRequest(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{})
	_, err := runCmdCapturingStdout(t, "launch", "--vps", "--custom-domain", "example.com", "--accept-cost", "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only applies to Static Hosting")
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
	assert.Empty(t, *calls, "refused before any API call")
}

func TestLaunchCommand_FlagSet(t *testing.T) {
	cmd := newLaunchCmd()
	assert.NotNil(t, cmd.Flags().Lookup("custom-domain"))
	assert.NotNil(t, cmd.Flags().Lookup("with-www"))
}

// ── Non-fatal outcomes ────────────────────────────────────────────────────────

func TestLaunchFinishCustomDomain_RefusalsAreNonFatal(t *testing.T) {
	cases := []struct {
		name      string
		errors    []string
		retryable bool
		next      string
	}{
		{"paid plan", []string{"Custom domains are only available on paid plans"}, false, "/account/packages"},
		{"rate limit", []string{"Too many requests to add or remove custom domains (at most 10 per hour). Try again later."}, true, "dhq domains add"},
		{"other", []string{"We couldn't add example.com right now. Please try again in a few minutes."}, false, "dhq domains add"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No HTTP is expected: there is no domain to wait for.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()
			env, _, _ := testLaunchEnvelope()
			server := launchStaticServer()
			server.CustomDomains = []sdk.CustomDomain{}
			server.CustomDomainErrors = tc.errors

			got := launchFinishCustomDomain(t.Context(), env, launchDomainCfg("example.com"), newTestClient(t, srv), server, "acme", launchDomainLiveURL)
			require.NotNil(t, got)
			assert.Equal(t, "not_added", got.Status)
			assert.False(t, got.Added)
			assert.Equal(t, tc.errors, got.Errors)
			assert.Equal(t, tc.retryable, got.Retryable)
			assert.Contains(t, got.NextStep, tc.next)
			assert.Contains(t, got.NextStep, launchDomainLiveURL, "says the site is live where it is")
			assert.Contains(t, got.NextStep, "dhq domains add example.com")
		})
	}
}

// An older server ignores the attributes and reports neither field.
func TestLaunchFinishCustomDomain_OlderServerReportsNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	env, _, _ := testLaunchEnvelope()

	got := launchFinishCustomDomain(t.Context(), env, launchDomainCfg("example.com"), newTestClient(t, srv), launchStaticServer(), "acme", launchDomainLiveURL)
	require.NotNil(t, got)
	assert.Equal(t, "not_added", got.Status)
	assert.Contains(t, got.NextStep, "dhq domains add example.com")
	require.Len(t, got.Errors, 1)
	assert.Contains(t, got.Errors[0], "did not report")
}

func TestLaunchFinishCustomDomain_NoDomainRequested(t *testing.T) {
	env, _, _ := testLaunchEnvelope()
	assert.Nil(t, launchFinishCustomDomain(t.Context(), env, launchDomainCfg(""), nil, launchStaticServer(), "acme", launchDomainLiveURL))
}

// ── Polling for the TXT record ────────────────────────────────────────────────

func TestLaunchFinishCustomDomain_PollsUntilRegistered(t *testing.T) {
	fastDomainsPolling(t, 5*time.Second)
	srv, calls := launchDomainListServer(t, "["+registeringDomainJSON+"]", "["+registeredDomainJSON+"]")
	env, _, _ := testLaunchEnvelope()
	server := launchStaticServer()
	server.CustomDomains = []sdk.CustomDomain{launchDomainFromJSON(t, registeringDomainJSON)}
	server.CustomDomainErrors = []string{}

	got := launchFinishCustomDomain(t.Context(), env, launchDomainCfg("www.example.com"), newTestClient(t, srv), server, "acme", launchDomainLiveURL)
	require.NotNil(t, got)
	assert.Equal(t, 2, *calls, "polls until registering is false")
	assert.Equal(t, "dns_records_required", got.Status)
	assert.True(t, got.Added)
	require.Len(t, got.Domains, 1)
	assert.False(t, got.Domains[0].Registering)
	types := []string{}
	for _, r := range got.Domains[0].DNSRecords {
		types = append(types, r.Type)
	}
	assert.Equal(t, []string{"CNAME", "TXT"}, types)
	assert.Contains(t, got.NextStep, "dhq domains show www.example.com -p my-app -s srv-1")
	assert.Contains(t, got.NextStep, "TXT")
}

func TestLaunchFinishCustomDomain_StopsWaitingAfterTimeout(t *testing.T) {
	fastDomainsPolling(t, 20*time.Millisecond)
	srv, calls := launchDomainListServer(t, "["+registeringDomainJSON+"]")
	env, _, _ := testLaunchEnvelope()
	server := launchStaticServer()
	server.CustomDomains = []sdk.CustomDomain{launchDomainFromJSON(t, registeringDomainJSON)}
	server.CustomDomainErrors = []string{}

	got := launchFinishCustomDomain(t.Context(), env, launchDomainCfg("www.example.com"), newTestClient(t, srv), server, "acme", launchDomainLiveURL)
	require.NotNil(t, got)
	assert.Positive(t, *calls)
	assert.Equal(t, "registering", got.Status)
	assert.True(t, got.Domains[0].Registering)
	assert.Contains(t, got.NextStep, "dhq domains show")
}

// A domain with no `registering` field comes from an older server: there is
// nothing to wait for.
func TestLaunchFinishCustomDomain_OlderServerDomainIsNotPolled(t *testing.T) {
	fastDomainsPolling(t, 5*time.Second)
	srv, calls := launchDomainListServer(t, "["+domainJSON+"]")
	env, _, _ := testLaunchEnvelope()
	server := launchStaticServer()
	server.CustomDomains = []sdk.CustomDomain{launchDomainFromJSON(t, domainJSON)}
	server.CustomDomainErrors = []string{}

	got := launchFinishCustomDomain(t.Context(), env, launchDomainCfg("www.example.com"), newTestClient(t, srv), server, "acme", launchDomainLiveURL)
	require.NotNil(t, got)
	assert.Zero(t, *calls)
	assert.Equal(t, "dns_records_required", got.Status)
}

func TestLaunchFinishCustomDomain_FailedRegistrationIsNonFatal(t *testing.T) {
	fastDomainsPolling(t, 5*time.Second)
	srv, _ := launchDomainListServer(t, "["+failedDomainJSON+"]")
	env, _, _ := testLaunchEnvelope()
	server := launchStaticServer()
	server.CustomDomains = []sdk.CustomDomain{launchDomainFromJSON(t, registeringDomainJSON)}
	server.CustomDomainErrors = []string{}

	got := launchFinishCustomDomain(t.Context(), env, launchDomainCfg("www.example.com"), newTestClient(t, srv), server, "acme", launchDomainLiveURL)
	require.NotNil(t, got)
	assert.Equal(t, "failed", got.Status)
	assert.Contains(t, strings.Join(got.Errors, " "), "could not be registered")
	assert.Contains(t, got.NextStep, "dhq domains remove www.example.com")
	assert.Contains(t, got.NextStep, "dhq domains add www.example.com")
}

// Losing the ability to read the domain back is reported, never a failure.
func TestLaunchFinishCustomDomain_ReadBackFailureIsNonFatal(t *testing.T) {
	fastDomainsPolling(t, 5*time.Second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()
	env, _, _ := testLaunchEnvelope()
	server := launchStaticServer()
	server.CustomDomains = []sdk.CustomDomain{launchDomainFromJSON(t, registeringDomainJSON)}
	server.CustomDomainErrors = []string{}

	got := launchFinishCustomDomain(t.Context(), env, launchDomainCfg("www.example.com"), newTestClient(t, srv), server, "acme", launchDomainLiveURL)
	require.NotNil(t, got)
	assert.Equal(t, "registering", got.Status)
	assert.Contains(t, strings.Join(got.Errors, " "), "could not be read")
}

func TestLaunchFinishCustomDomain_ActiveDomain(t *testing.T) {
	active := strings.Replace(registeredDomainJSON, `"status":"pending"`, `"status":"active"`, 1)
	srv, _ := launchDomainListServer(t, "["+active+"]")
	env, _, _ := testLaunchEnvelope()
	server := launchStaticServer()
	server.CustomDomains = []sdk.CustomDomain{launchDomainFromJSON(t, active)}
	server.CustomDomainErrors = []string{}

	got := launchFinishCustomDomain(t.Context(), env, launchDomainCfg("www.example.com"), newTestClient(t, srv), server, "acme", launchDomainLiveURL)
	assert.Equal(t, "active", got.Status)
}

// --with-www on a hostname with no www form asks for something that cannot
// happen, so the result says it added nothing extra.
func TestLaunchFinishCustomDomain_WithWWWAddedNothing(t *testing.T) {
	env, _, _ := testLaunchEnvelope()
	srv, _ := launchDomainListServer(t, "["+registeredDomainJSON+"]")
	server := launchStaticServer()
	server.CustomDomains = []sdk.CustomDomain{launchDomainFromJSON(t, registeredDomainJSON)}
	server.CustomDomainErrors = []string{}
	cfg := launchDomainCfg("blog.example.com")
	cfg.withWWW = true

	got := launchFinishCustomDomain(t.Context(), env, cfg, newTestClient(t, srv), server, "acme", launchDomainLiveURL)
	assert.Contains(t, strings.Join(got.Errors, " "), "--with-www added nothing")
}

// ── Re-running launch on a server that already exists ─────────────────────────

func launchExistingServerAPI(t *testing.T, listBody string, createStatus int, createBody string) (*httptest.Server, *[]string) {
	t.Helper()
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == launchDomainsPath:
			_, _ = w.Write([]byte(listBody))
		case r.Method == http.MethodPost && r.URL.Path == launchDomainsPath:
			w.WriteHeader(createStatus)
			_, _ = w.Write([]byte(createBody))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func TestLaunchEnsureCustomDomain_AddsToExistingServer(t *testing.T) {
	srv, requests := launchExistingServerAPI(t, "[]", http.StatusCreated, registeredDomainJSON)
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("www.example.com")
	server := launchStaticServer()

	require.NoError(t, launchEnsureCustomDomainOnServer(t.Context(), env, &cfg, newTestClient(t, srv), server))
	assert.Equal(t, []string{"GET " + launchDomainsPath, "POST " + launchDomainsPath}, *requests)
	require.Len(t, server.CustomDomains, 1)
	assert.Equal(t, "www.example.com", server.CustomDomains[0].Hostname)
}

// Re-running the same launch must not add the domain a second time.
func TestLaunchEnsureCustomDomain_KeepsDomainAlreadyOnServer(t *testing.T) {
	srv, requests := launchExistingServerAPI(t, "["+registeredDomainJSON+"]", http.StatusCreated, registeredDomainJSON)
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("www.example.com")
	server := launchStaticServer()

	require.NoError(t, launchEnsureCustomDomainOnServer(t.Context(), env, &cfg, newTestClient(t, srv), server))
	assert.Equal(t, []string{"GET " + launchDomainsPath}, *requests, "already there: nothing added")
	require.Len(t, server.CustomDomains, 1)
}

func TestLaunchEnsureCustomDomain_InvalidHostnameIsFatal(t *testing.T) {
	srv, _ := launchExistingServerAPI(t, "[]", http.StatusUnprocessableEntity, `{"errors":["Hostname is already in use"]}`)
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("www.example.com")

	err := launchEnsureCustomDomainOnServer(t.Context(), env, &cfg, newTestClient(t, srv), launchStaticServer())
	var le *launchError
	require.ErrorAs(t, err, &le)
	assert.Equal(t, reasonCustomDomainInvalid, le.Reason)
}

func TestLaunchEnsureCustomDomain_RefusalsAreNonFatal(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantInErrs string
	}{
		{"paid plan", http.StatusForbidden, `{"error":"paid_plan_required","error_code":"paid_plan_required","message":"Custom domains are only available on paid plans"}`, "paid plans"},
		{"rate limit", http.StatusTooManyRequests, `{"error":"Too many requests to add or remove custom domains (at most 10 per hour). Try again later."}`, "Too many requests"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := launchExistingServerAPI(t, "[]", tc.status, tc.body)
			env, _, _ := testLaunchEnvelope()
			cfg := launchDomainCfg("www.example.com")
			server := launchStaticServer()

			require.NoError(t, launchEnsureCustomDomainOnServer(t.Context(), env, &cfg, newTestClient(t, srv), server))
			assert.NotNil(t, server.CustomDomains, "the server did report on the domain")
			assert.Empty(t, server.CustomDomains)
			assert.Contains(t, strings.Join(server.CustomDomainErrors, " "), tc.wantInErrs)
		})
	}
}

// ── Final output ──────────────────────────────────────────────────────────────

func launchDomainResultFixture() *launchCustomDomainResult {
	var d sdk.CustomDomain
	_ = json.Unmarshal([]byte(registeredDomainJSON), &d)
	return &launchCustomDomainResult{
		Hostname: "www.example.com", Added: true, Status: "dns_records_required",
		Domains: []sdk.CustomDomain{d}, NextStep: "Publish them",
	}
}

func TestWriteLaunchResult_JSONShape(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	err := writeLaunchResult(env, launchDomainCfg("www.example.com"), launchStaticServer(),
		&sdk.Deployment{Identifier: "dep-1"}, launchDomainLiveURL, launchDomainResultFixture())
	require.NoError(t, err)

	var resp struct {
		OK          bool
		Data        map[string]any
		Breadcrumbs []output.Breadcrumb
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp))
	assert.True(t, resp.OK)
	assert.Equal(t, "live", resp.Data["status"])
	assert.Equal(t, launchDomainLiveURL, resp.Data["url"], "the deployhq-sites.com URL stays the reported URL until the domain is live")
	domain := resp.Data["custom_domain"].(map[string]any)
	assert.Equal(t, "www.example.com", domain["hostname"])
	assert.Equal(t, "dns_records_required", domain["status"])
	assert.Contains(t, domain, "next_step")
	assert.Contains(t, domain, "domains")
	assert.Contains(t, domain, "retryable")
	var cmds []string
	for _, b := range resp.Breadcrumbs {
		cmds = append(cmds, b.Cmd)
	}
	assert.Contains(t, strings.Join(cmds, "\n"), "dhq domains show www.example.com")
	assert.Contains(t, strings.Join(cmds, "\n"), "dhq domains check www.example.com")
}

func TestWriteLaunchResult_JSONOmitsDomainWhenNotRequested(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	require.NoError(t, writeLaunchResult(env, launchDomainCfg(""), launchStaticServer(), &sdk.Deployment{Identifier: "d"}, launchDomainLiveURL, nil))
	var resp struct{ Data map[string]any }
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp))
	assert.NotContains(t, resp.Data, "custom_domain")
}

// Text mode: the scripting contract is that the only thing on stdout is the live
// URL, so the DNS records go to stderr.
func TestWriteLaunchResult_TextPrintsRecordsOnStderr(t *testing.T) {
	env, stdout, stderr := testLaunchEnvelope()
	env.IsTTY = true
	require.NoError(t, writeLaunchResult(env, launchDomainCfg("www.example.com"), launchStaticServer(),
		&sdk.Deployment{Identifier: "dep-1"}, launchDomainLiveURL, launchDomainResultFixture()))

	assert.Equal(t, launchDomainLiveURL+"\n", stdout.String(), "stdout stays the live URL only")
	errOut := stderr.String()
	assert.Contains(t, errOut, "www.example.com")
	assert.Contains(t, errOut, "customers.deployhq-sites.com", "the CNAME target is printed")
	assert.Contains(t, errOut, "abc123", "the TXT value is printed")
}

func TestWriteLaunchResult_TextReportsRefusalButSucceeds(t *testing.T) {
	env, stdout, stderr := testLaunchEnvelope()
	env.IsTTY = true
	domain := &launchCustomDomainResult{
		Hostname: "example.com", Status: "not_added", Domains: []sdk.CustomDomain{},
		Errors: []string{"Custom domains are only available on paid plans"}, NextStep: "Upgrade, then run: dhq domains add example.com",
	}
	require.NoError(t, writeLaunchResult(env, launchDomainCfg("example.com"), launchStaticServer(),
		&sdk.Deployment{Identifier: "dep-1"}, launchDomainLiveURL, domain), "the site is live: exit 0")

	assert.Equal(t, launchDomainLiveURL+"\n", stdout.String())
	assert.Contains(t, stderr.String(), "paid plans")
	assert.Contains(t, stderr.String(), "dhq domains add example.com")
}

// ── Dry run ───────────────────────────────────────────────────────────────────

func TestLaunchDryRun_CustomDomain_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a static dry run makes no requests here: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	env, stdout, _ := testLaunchEnvelopeJSON()
	caps := &sdk.AccountCapabilities{BetaFeatures: true, StaticHostingEligible: true}
	cfg := launchDomainCfg("example.com")
	cfg.withWWW = true
	cfg.dryRun = true

	require.NoError(t, launchDryRun(t.Context(), env, cfg, newTestClient(t, srv), caps))

	var resp struct{ Data map[string]any }
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp))
	would := resp.Data["would"].(map[string]any)
	assert.Equal(t, "example.com", would["custom_domain"])
	assert.Equal(t, true, would["with_www"])
	assert.Contains(t, resp.Data["warning"], "paid plan")
}

func TestLaunchDryRun_NoCustomDomainHasNoDomainFields(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	caps := &sdk.AccountCapabilities{BetaFeatures: true, StaticHostingEligible: true}
	cfg := launchDomainCfg("")
	cfg.dryRun = true

	require.NoError(t, launchDryRun(t.Context(), env, cfg, nil, caps))
	assert.NotContains(t, stdout.String(), "custom_domain")
	assert.NotContains(t, stdout.String(), "paid plan")
}

func TestLaunchDryRun_CustomDomain_Text(t *testing.T) {
	env, _, stderr := testLaunchEnvelope()
	caps := &sdk.AccountCapabilities{BetaFeatures: true, StaticHostingEligible: true}
	cfg := launchDomainCfg("example.com")
	cfg.withWWW = true
	cfg.dryRun = true
	env.IsTTY = true

	require.NoError(t, launchDryRun(t.Context(), env, cfg, nil, caps))
	assert.Contains(t, stderr.String(), "example.com")
	assert.Contains(t, stderr.String(), "www.example.com")
}

// ── Pair helpers ──────────────────────────────────────────────────────────────

func TestDomainCounterpart(t *testing.T) {
	assert.Equal(t, "www.example.com", domainCounterpart("example.com"))
	assert.Equal(t, "example.com", domainCounterpart("www.example.com"))
	assert.Equal(t, "", domainCounterpart("blog.example.com"))
}

// ── End to end, through the command ───────────────────────────────────────────

// launchFakeAPI serves the requests a full static launch makes, with the given
// outcome for the server create, and records what was asked.
func launchFakeAPI(t *testing.T, createBody string, createStatus int, domains []string) *[]string {
	t.Helper()
	var requests []string
	listIdx := 0
	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	respond := func(r *http.Request, status int, body string) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	}
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		key := r.Method + " " + r.URL.Path
		if strings.Contains(r.URL.Host, "github") {
			return respond(r, 200, `{}`)
		}
		requests = append(requests, key)
		switch {
		case key == "GET /profile":
			return respond(r, 200, `{"account":{"beta_features":true,"static_hosting_eligible":true,"managed_vps_eligible":true}}`)
		case key == "POST /detection":
			return respond(r, 200, `{"stack":"vite","suggested_protocol":"static_hosting","static_hosting":{"root_path":"dist","spa_mode":false}}`)
		case key == "GET /projects/my-app":
			return respond(r, 200, `{"permalink":"my-app","name":"my-app","repository":{"url":"git@github.com:acme/site.git"}}`)
		case key == "POST /projects/my-app/servers":
			return respond(r, createStatus, createBody)
		case key == "POST /projects/my-app/deployments":
			return respond(r, 201, `{"identifier":"dep-1","status":"running"}`)
		case key == "GET /projects/my-app/deployments/dep-1":
			return respond(r, 200, `{"identifier":"dep-1","status":"completed"}`)
		case key == "GET "+launchDomainsPath:
			body := domains[min(listIdx, len(domains)-1)]
			listIdx++
			return respond(r, 200, body)
		}
		return respond(r, 404, `{"error":"not_found"}`)
	})
	return &requests
}

func launchDomainTestRepo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	runGit := func(args ...string) {
		t.Helper()
		_, err := runGitCommand(args...)
		_ = err
	}
	runGit("init", "-q")
	runGit("remote", "add", "origin", "git@github.com:acme/site.git")
}

func TestLaunchCommand_CustomDomain_EndToEnd(t *testing.T) {
	withResolvableContext(t)
	launchDomainTestRepo(t)
	shrinkPollBackoff(t)
	fastDomainsPolling(t, 5*time.Second)
	created := launchActiveServerJSON(`,"custom_domains":[` + registeringDomainJSON + `],"custom_domain_errors":[]`)
	requests := launchFakeAPI(t, created, 201, []string{"[" + registeredDomainJSON + "]"})

	stdout, err := runCmdCapturingStdout(t, "launch", "--static", "--subdomain", "my-app", "--custom-domain", "www.example.com", "--json")
	require.NoError(t, err, "the site is live: exit 0")

	var resp struct {
		OK          bool
		Summary     string
		Breadcrumbs []output.Breadcrumb
		Data        struct {
			Status       string
			URL          string
			Server       string
			CustomDomain struct {
				Hostname string
				Added    bool
				Status   string
				Domains  []struct {
					DNSRecords  []struct{ Type string } `json:"dns_records"`
					Registering bool
				}
				NextStep string `json:"next_step"`
			} `json:"custom_domain"`
		}
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &resp), "stdout: %s", stdout)
	assert.True(t, resp.OK)
	assert.Equal(t, "live", resp.Data.Status)
	assert.Equal(t, launchDomainLiveURL, resp.Data.URL)
	assert.Equal(t, "dns_records_required", resp.Data.CustomDomain.Status)
	assert.True(t, resp.Data.CustomDomain.Added)
	require.Len(t, resp.Data.CustomDomain.Domains, 1)
	assert.False(t, resp.Data.CustomDomain.Domains[0].Registering, "waited for the TXT record")
	assert.Len(t, resp.Data.CustomDomain.Domains[0].DNSRecords, 2)
	assert.Contains(t, resp.Summary, "www.example.com")
	assert.Contains(t, strings.Join(*requests, "\n"), "GET "+launchDomainsPath, "polled for registration")
}

func TestLaunchCommand_InvalidCustomDomainExitsOne(t *testing.T) {
	withResolvableContext(t)
	launchDomainTestRepo(t)
	requests := launchFakeAPI(t, `{"base":["Custom domain must be one of the test domains in this environment (dropsite.dev)"]}`, 422, nil)

	stdout, err := runCmdCapturingStdout(t, "launch", "--static", "--subdomain", "my-app", "--custom-domain", "other.example", "--json")
	require.Error(t, err)
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))

	var resp struct {
		OK   bool
		Data struct {
			Reason    string
			Retryable bool
			ExitCode  int    `json:"exit_code"`
			NextStep  string `json:"next_step"`
		}
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &resp), "stdout: %s", stdout)
	assert.False(t, resp.OK)
	assert.Equal(t, reasonCustomDomainInvalid, resp.Data.Reason)
	assert.Equal(t, 1, resp.Data.ExitCode)
	assert.Contains(t, resp.Data.NextStep, "--custom-domain")
	assert.NotContains(t, strings.Join(*requests, "\n"), "POST /projects/my-app/deployments", "nothing deployed")
}

func TestLaunchCommand_DomainRefusalStillExitsZero(t *testing.T) {
	withResolvableContext(t)
	launchDomainTestRepo(t)
	created := launchActiveServerJSON(`,"custom_domains":[],"custom_domain_errors":["Custom domains are only available on paid plans"]`)
	launchFakeAPI(t, created, 201, nil)

	stdout, err := runCmdCapturingStdout(t, "launch", "--static", "--subdomain", "my-app", "--custom-domain", "example.com", "--json")
	require.NoError(t, err, "a plan refusal must not fail a live launch")

	var resp struct {
		Summary string
		Data    struct {
			Status       string
			CustomDomain struct {
				Status   string
				Errors   []string
				NextStep string `json:"next_step"`
			} `json:"custom_domain"`
		}
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &resp), "stdout: %s", stdout)
	assert.Equal(t, "live", resp.Data.Status)
	assert.Equal(t, "not_added", resp.Data.CustomDomain.Status)
	assert.Contains(t, resp.Data.CustomDomain.Errors[0], "paid plans")
	assert.Contains(t, resp.Data.CustomDomain.NextStep, "/account/packages")
	assert.Contains(t, resp.Summary, "not added")
}

func TestLaunchCommand_DryRunWithCustomDomainMakesNoChanges(t *testing.T) {
	withResolvableContext(t)
	launchDomainTestRepo(t)
	requests := launchFakeAPI(t, "{}", 500, nil)

	stdout, err := runCmdCapturingStdout(t, "launch", "--static", "--subdomain", "my-app", "--custom-domain", "example.com", "--with-www", "--dry-run", "--json")
	require.NoError(t, err)
	var resp struct {
		Data struct {
			Would map[string]any
		}
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &resp), "stdout: %s", stdout)
	assert.Equal(t, "example.com", resp.Data.Would["custom_domain"])
	for _, req := range *requests {
		assert.False(t, strings.HasPrefix(req, "POST /projects/"), "dry run must not change anything: %s", req)
	}
}
