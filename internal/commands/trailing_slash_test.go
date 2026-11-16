package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTrailingSlash(t *testing.T) {
	for _, in := range []string{"strip", "force", "preserve", " Force "} {
		got, err := parseTrailingSlash(in)
		require.NoError(t, err, in)
		assert.Contains(t, sdk.TrailingSlashModes, got)
	}
	_, err := parseTrailingSlash("add")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "force")
}

func TestServersCreate_TrailingSlashIsSentWithTheSite(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "create", "--name", "site", "--protocol-type", "static_hosting",
		"--subdomain", "my-site", "--trailing-slash", "FORCE", "--json"))
	attrs, ok := cap.body["hosted_website_attributes"].(map[string]any)
	require.True(t, ok, "body: %v", cap.body)
	assert.Equal(t, "force", attrs["trailing_slash"])
}

func TestServersCreate_TrailingSlashOmittedWhenNotGiven(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "create", "--name", "site", "--protocol-type", "static_hosting",
		"--subdomain", "my-site", "--json"))
	attrs := cap.body["hosted_website_attributes"].(map[string]any)
	assert.NotContains(t, attrs, "trailing_slash")
}

func TestServersCreate_TrailingSlashRefusals_NoHTTP(t *testing.T) {
	cases := map[string][]string{
		"not a mode":      {"--protocol-type", "static_hosting", "--subdomain", "my-site", "--trailing-slash", "yes"},
		"not static":      {"--protocol-type", "ssh", "--hostname", "h", "--username", "u", "--trailing-slash", "force"},
		"needs subdomain": {"--protocol-type", "static_hosting", "--trailing-slash", "force"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			withResolvableContext(t)
			blockNetwork(t)
			err := runServersCmd(t, append([]string{"servers", "create", "--name", "site"}, args...)...)
			var userErr *output.UserError
			require.True(t, errors.As(err, &userErr), "got %v", err)
			assert.Contains(t, userErr.Message, "--trailing-slash")
		})
	}
}

func TestServersUpdate_TrailingSlashIsHoisted(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "update", "srv-1", "--trailing-slash", "preserve", "--json"))
	assert.Equal(t, map[string]any{"trailing_slash": "preserve"}, cap.body["hosted_website_attributes"])
	server := cap.server(t)
	assert.NotContains(t, server, "trailing_slash")
}

func TestServersUpdate_NoTrailingSlashSendsNoSiteSettings(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "update", "srv-1", "--name", "renamed", "--json"))
	assert.NotContains(t, cap.body, "hosted_website_attributes")
}

func TestServersUpdate_BadTrailingSlash_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "servers", "update", "srv-1", "--trailing-slash", "sometimes")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
}

func TestWarnIfTrailingSlashNotApplied(t *testing.T) {
	force, strip := "force", "strip"
	cases := []struct {
		name   string
		server *sdk.Server
		warn   string
	}{
		{"applied", &sdk.Server{ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{TrailingSlash: &force}}, ""},
		{"no site block", &sdk.Server{ProtocolType: "static_hosting"}, ""},
		{"other server type", &sdk.Server{Name: "prod", ProtocolType: "ssh"}, "only applies to Static Hosting"},
		{"different mode", &sdk.Server{Identifier: "srv-1", ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{TrailingSlash: &strip}}, "did not confirm --trailing-slash force. Check the site's settings with: dhq servers show srv-1 -p my-app"},
		{"not reported", &sdk.Server{ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{}}, "did not confirm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
			warnIfTrailingSlashNotApplied(env, "force", tc.server, "dhq servers show srv-1 -p my-app")
			if tc.warn == "" {
				assert.Empty(t, stderr.String())
			} else {
				assert.Contains(t, stderr.String(), tc.warn)
			}
		})
	}
}

func TestLaunchStatic_SendsTrailingSlash(t *testing.T) {
	srv, body := launchDomainServer(t, http.StatusCreated, launchActiveServerJSON(""))
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("")
	cfg.trailingSlash = "force"

	_, err := launchProvisionStatic(t.Context(), env, &cfg, newTestClient(t, srv))
	require.NoError(t, err)
	var payload struct {
		Attrs map[string]any `json:"hosted_website_attributes"`
	}
	require.NoError(t, json.Unmarshal([]byte(*body), &payload))
	assert.Equal(t, "force", payload.Attrs["trailing_slash"])
}

func TestLaunchStatic_OmitsTrailingSlashWhenNotGiven(t *testing.T) {
	srv, body := launchDomainServer(t, http.StatusCreated, launchActiveServerJSON(""))
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("")

	_, err := launchProvisionStatic(t.Context(), env, &cfg, newTestClient(t, srv))
	require.NoError(t, err)
	assert.NotContains(t, *body, "trailing_slash")
}

func TestValidateLaunchTrailingSlash(t *testing.T) {
	cfg := launchConfig{trailingSlash: "Preserve"}
	require.NoError(t, validateLaunchTrailingSlash(&cfg))
	assert.Equal(t, "preserve", cfg.trailingSlash)

	bad := launchConfig{trailingSlash: "always"}
	require.Error(t, validateLaunchTrailingSlash(&bad))

	vps := launchConfig{trailingSlash: "force", targetProtocol: "managed_vps"}
	err := validateLaunchTrailingSlash(&vps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only applies to Static Hosting")
}

func TestResolveLaunchTrailingSlashTarget(t *testing.T) {
	undecided := launchConfig{trailingSlash: "force"}
	require.NoError(t, resolveLaunchTrailingSlashTarget(&undecided))
	assert.Equal(t, "static_hosting", undecided.targetProtocol)

	vps := launchConfig{trailingSlash: "force", targetProtocol: "managed_vps"}
	require.Error(t, resolveLaunchTrailingSlashTarget(&vps))

	none := launchConfig{targetProtocol: "managed_vps"}
	require.NoError(t, resolveLaunchTrailingSlashTarget(&none))
}

func TestLaunchCommand_VPSWithTrailingSlashRefusedBeforeAnyRequest(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{})
	_, err := runCmdCapturingStdout(t, "launch", "--vps", "--trailing-slash", "force", "--accept-cost", "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only applies to Static Hosting")
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
	assert.Empty(t, *calls, "refused before any API call")
}

func TestLaunchCommand_BadTrailingSlashRefusedBeforeAnyRequest(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{})
	_, err := runCmdCapturingStdout(t, "launch", "--static", "--trailing-slash", "on", "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a trailing-slash mode")
	assert.Empty(t, *calls, "refused before any API call")
}

func TestLaunchDryRun_TrailingSlash(t *testing.T) {
	env, stdout, _ := testLaunchEnvelopeJSON()
	caps := &sdk.AccountCapabilities{BetaFeatures: true, StaticHostingEligible: true}
	cfg := launchDomainCfg("")
	cfg.dryRun = true
	cfg.trailingSlash = "force"

	require.NoError(t, launchDryRun(t.Context(), env, cfg, nil, caps))
	var resp struct{ Data map[string]any }
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp))
	assert.Equal(t, "force", resp.Data["would"].(map[string]any)["trailing_slash"])
}

// launchSettingsAPI answers a server update and records each request.
func launchSettingsAPI(t *testing.T, status int, body string) (*httptest.Server, *[]capturedBody) {
	t.Helper()
	var requests []capturedBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		requests = append(requests, capturedBody{method: r.Method, path: r.URL.Path, body: b})
		if r.Method == http.MethodPut && r.URL.Path == "/projects/my-app/servers/srv-1" {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

type capturedBody struct {
	method, path string
	body         map[string]any
}

func TestLaunchEnsureTrailingSlash_AppliesToExistingSite(t *testing.T) {
	srv, requests := launchSettingsAPI(t, http.StatusOK,
		`{"identifier":"srv-1","protocol_type":"static_hosting","static_hosting":{"trailing_slash":"force"}}`)
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("")
	cfg.trailingSlash = "force"
	server := launchStaticServer()

	launchEnsureTrailingSlashOnServer(t.Context(), env, cfg, newTestClient(t, srv), server)
	require.Len(t, *requests, 1)
	assert.Equal(t, map[string]any{"trailing_slash": "force"}, (*requests)[0].body["hosted_website_attributes"])
	require.NotNil(t, server.StaticHosting.TrailingSlash)
	assert.Equal(t, "force", *server.StaticHosting.TrailingSlash)
}

func TestLaunchEnsureTrailingSlash_AlreadySetMakesNoRequest(t *testing.T) {
	srv, requests := launchSettingsAPI(t, http.StatusOK, `{}`)
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("")
	cfg.trailingSlash = "force"
	server := launchStaticServer()
	force := "force"
	server.StaticHosting.TrailingSlash = &force

	launchEnsureTrailingSlashOnServer(t.Context(), env, cfg, newTestClient(t, srv), server)
	assert.Empty(t, *requests)
}

func TestLaunchEnsureTrailingSlash_NotRequestedMakesNoRequest(t *testing.T) {
	srv, requests := launchSettingsAPI(t, http.StatusOK, `{}`)
	env, _, _ := testLaunchEnvelope()

	launchEnsureTrailingSlashOnServer(t.Context(), env, launchDomainCfg(""), newTestClient(t, srv), launchStaticServer())
	assert.Empty(t, *requests)
}

func TestLaunchEnsureTrailingSlash_FailureIsAWarning(t *testing.T) {
	srv, _ := launchSettingsAPI(t, http.StatusUnprocessableEntity, `{"error":"Trailing slash is not included in the list"}`)
	env, _, stderr := testLaunchEnvelope()
	cfg := launchDomainCfg("")
	cfg.trailingSlash = "preserve"

	launchEnsureTrailingSlashOnServer(t.Context(), env, cfg, newTestClient(t, srv), launchStaticServer())
	assert.Contains(t, stderr.String(), "dhq servers update srv-1 -p my-app --trailing-slash preserve")
}

func TestGlobalServersUpdate_TrailingSlashIsHoisted(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"PUT /global_servers/srv-global": {200, `{"identifier":"srv-global","name":"Shared Site","protocol_type":"static_hosting"}`},
	})

	require.NoError(t, runServersCmd(t, "global-servers", "update", "srv-global", "--trailing-slash", "force"))
	require.Len(t, *calls, 1)
	body := (*calls)[0].body
	assert.Equal(t, map[string]any{"trailing_slash": "force"}, body["hosted_website_attributes"])
	assert.NotContains(t, body["server"], "trailing_slash")
}

func TestGlobalServersUpdate_NoTrailingSlashSendsNoSiteSettings(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"PUT /global_servers/srv-global": {200, `{"identifier":"srv-global","name":"Renamed"}`},
	})

	require.NoError(t, runServersCmd(t, "global-servers", "update", "srv-global", "--name", "Renamed"))
	assert.NotContains(t, (*calls)[0].body, "hosted_website_attributes")
}

func TestGlobalServersUpdate_BadTrailingSlash_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "global-servers", "update", "srv-global", "--trailing-slash", "maybe")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "--trailing-slash")
}

// A site that never chose a mode reports null, which behaves as strip; the
// API leaves it null when strip is requested, so that is not a refusal.
func TestWarnIfTrailingSlashNotApplied_StripOnUnsetSite(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	warnIfTrailingSlashNotApplied(env, "strip", &sdk.Server{ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{}}, "dhq servers show srv-1 -p my-app")
	assert.Empty(t, stderr.String())
}

func TestLaunchEnsureTrailingSlash_StripOnUnsetSiteMakesNoRequest(t *testing.T) {
	srv, requests := launchSettingsAPI(t, http.StatusOK, `{}`)
	env, _, _ := testLaunchEnvelope()
	cfg := launchDomainCfg("")
	cfg.trailingSlash = "strip"

	launchEnsureTrailingSlashOnServer(t.Context(), env, cfg, newTestClient(t, srv), launchStaticServer())
	assert.Empty(t, *requests)
}

// --trailing-slash alone sends an empty server object beside the site
// settings, which the API accepts for a Static Hosting server.
func TestServersUpdate_TrailingSlashOnlySendsEmptyServer(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "update", "srv-1", "--trailing-slash", "force", "--json"))
	assert.Equal(t, map[string]any{}, cap.server(t))
	assert.Equal(t, map[string]any{"trailing_slash": "force"}, cap.body["hosted_website_attributes"])
}

// Any other server type refuses that body as a missing `server`; say what
// actually went wrong.
func TestServersUpdate_TrailingSlashOnlyOnOtherServerIsExplained(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"PUT /projects/my-app/servers/srv-ssh": {400, `{"error":"There was a problem in the JSON you submitted: param is missing or the value is empty: server","error_code":"invalid_json"}`},
	})

	err := runServersCmd(t, "servers", "update", "srv-ssh", "--trailing-slash", "force")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "only applies to Static Hosting")
}

func TestGlobalServersUpdate_TrailingSlashOnlyOnOtherServerIsExplained(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"PUT /global_servers/srv-ssh": {400, `{"error":"There was a problem in the JSON you submitted: param is missing or the value is empty: server","error_code":"invalid_json"}`},
	})

	err := runServersCmd(t, "global-servers", "update", "srv-ssh", "--trailing-slash", "force")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "only applies to Static Hosting")
}

// A launch re-run whose saved server is not Static Hosting cannot apply
// --trailing-slash: say so instead of skipping it silently.
func TestLaunchEnsureTrailingSlash_OtherServerTypeWarns(t *testing.T) {
	srv, requests := launchSettingsAPI(t, http.StatusOK, `{}`)
	env, _, stderr := testLaunchEnvelope()
	cfg := launchDomainCfg("")
	cfg.trailingSlash = "force"
	server := launchStaticServer()
	server.ProtocolType = "managed_vps"

	launchEnsureTrailingSlashOnServer(t.Context(), env, cfg, newTestClient(t, srv), server)
	assert.Empty(t, *requests)
	assert.Contains(t, stderr.String(), "--trailing-slash force was not applied")
	assert.Contains(t, stderr.String(), "not a Static Hosting server")
}

// The API's own reason stays visible next to the explanation.
func TestServersUpdate_TrailingSlashOnlyKeepsTheAPIReason(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"PUT /projects/my-app/servers/srv-ssh": {400, `{"error":"There was a problem in the JSON you submitted: param is missing or the value is empty: server","error_code":"invalid_json"}`},
	})

	err := runServersCmd(t, "servers", "update", "srv-ssh", "--trailing-slash", "force")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "param is missing")
}

// The API saves the rest of the server and answers 409, 429 or 503 when only
// the trailing slash could not be published: report that, retryably.
func TestServersUpdate_TrailingSlashNotPublishedIsAPartialSave(t *testing.T) {
	cases := []struct {
		status     int
		body       string
		retryAfter int
	}{
		{409, `{"error":"The server was saved, but the trailing slash setting was not changed: the site is busy publishing a deployment or another change. Try again in a moment."}`, 0},
		{429, `{"error":"The server was saved, but the trailing slash setting was not changed: Too many redirect changes. Try again later.","retry_after":3600}`, 3600},
		{503, `{"error":"The server was saved, but the trailing slash setting was not changed: it could not be published to the site. Try again in a moment."}`, 0},
	}
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			withResolvableContext(t)
			domainsAPI(t, map[string]cannedResponse{
				"PUT /projects/my-app/servers/srv-1": {tc.status, tc.body},
			})

			err := runServersCmd(t, "servers", "update", "srv-1", "--name", "renamed", "--trailing-slash", "force")
			var userErr *output.UserError
			require.True(t, errors.As(err, &userErr), "got %v", err)
			assert.Contains(t, userErr.Message, "The server was saved")
			assert.True(t, userErr.Retryable)
			assert.Equal(t, tc.retryAfter, userErr.RetryAfter)
			assert.Contains(t, userErr.Hint, "dhq servers update srv-1 -p my-app --trailing-slash force")
		})
	}
}

func TestGlobalServersUpdate_TrailingSlashNotPublishedIsAPartialSave(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"PUT /global_servers/srv-global": {409, `{"error":"The server was saved, but the trailing slash setting was not changed: the site is busy publishing a deployment or another change. Try again in a moment."}`},
	})

	err := runServersCmd(t, "global-servers", "update", "srv-global", "--trailing-slash", "force")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.True(t, userErr.Retryable)
	assert.Contains(t, userErr.Hint, "dhq global-servers update srv-global --trailing-slash force")
}

// A launch re-run keeps deploying; the warning says how long to wait.
func TestLaunchEnsureTrailingSlash_RateLimitedWarnsWithTheWait(t *testing.T) {
	srv, _ := launchSettingsAPI(t, http.StatusTooManyRequests,
		`{"error":"The server was saved, but the trailing slash setting was not changed: Too many redirect changes. Try again later.","retry_after":3600}`)
	env, _, stderr := testLaunchEnvelope()
	cfg := launchDomainCfg("")
	cfg.trailingSlash = "force"

	launchEnsureTrailingSlashOnServer(t.Context(), env, cfg, newTestClient(t, srv), launchStaticServer())
	assert.Contains(t, stderr.String(), "the trailing slash setting was not changed")
	assert.Contains(t, stderr.String(), "3600 seconds")
	assert.Contains(t, stderr.String(), "dhq servers update srv-1 -p my-app --trailing-slash force")
}
