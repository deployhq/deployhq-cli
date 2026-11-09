package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	domainsServersJSON = `[{"identifier":"srv-static","name":"My Site","protocol_type":"static_hosting"},` +
		`{"identifier":"srv-ssh","name":"Production","protocol_type":"ssh"}]`
	domainJSON = `{"identifier":"dom-1","hostname":"www.example.com","primary":true,"status":"pending",` +
		`"ssl_status":null,"verification_errors":[],"activated_at":null,"created_at":"2026-11-05T10:00:00Z",` +
		`"dns_records":[{"type":"CNAME","name":"www.example.com","value":"customers.deployhq-sites.com"},` +
		`{"type":"TXT","name":"_cf-custom-hostname.www.example.com","value":"abc123"}]}`
)

type domainsCall struct {
	method, path string
	body         map[string]any
}

type cannedResponse struct {
	status int
	body   string
}

// domainsAPI answers DeployHQ API requests from a method+path table and
// records each call. Unlisted API paths fail the test.
func domainsAPI(t *testing.T, routes map[string]cannedResponse) *[]domainsCall {
	t.Helper()
	sequences := make(map[string][]cannedResponse, len(routes))
	for route, resp := range routes {
		sequences[route] = []cannedResponse{resp}
	}
	return domainsAPISequence(t, sequences)
}

// domainsAPISequence is domainsAPI with a list of responses per route, served
// in order; the last one repeats.
func domainsAPISequence(t *testing.T, routes map[string][]cannedResponse) *[]domainsCall {
	t.Helper()
	var (
		mu    sync.Mutex
		calls []domainsCall
		seen  = map[string]int{}
	)
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		respond := func(status int, body string) *http.Response {
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Request:    r,
			}
		}
		// The update check against GitHub is unrelated to these tests.
		if !strings.Contains(r.URL.Path, "/projects/") && !strings.Contains(r.URL.Path, "/global_servers") {
			return respond(http.StatusOK, `{}`), nil
		}
		call := domainsCall{method: r.Method, path: r.URL.Path}
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &call.body)
		}
		route := r.Method + " " + r.URL.Path
		mu.Lock()
		calls = append(calls, call)
		n := seen[route]
		seen[route]++
		mu.Unlock()
		responses, ok := routes[route]
		if !ok {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return respond(http.StatusInternalServerError, `{"error":"unexpected"}`), nil
		}
		if n >= len(responses) {
			n = len(responses) - 1
		}
		return respond(responses[n].status, responses[n].body), nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
	return &calls
}

const (
	domainsBase       = "/projects/my-app/servers/srv-static/custom_domains"
	globalDomainsBase = "/global_servers/srv-global/custom_domains"

	globalServersJSON = `[{"identifier":"srv-global","name":"Shared Site","protocol_type":"static_hosting"}]`
)

func TestDomainsCmd_Structure(t *testing.T) {
	cmd := newDomainsCmd()
	assert.Equal(t, "domains", cmd.Name())
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
		assert.NotNil(t, sub.Flags().Lookup("server"), "%s needs --server", sub.Name())
		assert.NotNil(t, sub.Flags().Lookup("global"), "%s needs --global", sub.Name())
	}
	for _, want := range []string{"list", "show", "add", "remove", "check"} {
		assert.True(t, names[want], "missing subcommand %q", want)
	}
}

func TestDomainsHelp_ExplainsProductRules(t *testing.T) {
	cmd := newDomainsCmd()
	for _, want := range []string{"at most 2", "primary", "TXT", "ALIAS", "every few minutes", "7 days", "paid plan"} {
		assert.Contains(t, cmd.Long, want)
	}
}

func TestDomainsList_ResolvesServerNameAndLists(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"GET " + domainsBase:           {200, "[" + domainJSON + "]"},
	})

	require.NoError(t, runServersCmd(t, "domains", "list", "-s", "My Site", "--json"))
	require.Len(t, *calls, 2)
	assert.Equal(t, domainsBase, (*calls)[1].path)
}

func TestDomainsList_UUIDSkipsServerListing(t *testing.T) {
	withResolvableContext(t)
	uuid := "0b9f3c2e-1d4a-4c8b-9e2f-123456789abc"
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers/" + uuid + "/custom_domains": {200, "[]"},
	})

	require.NoError(t, runServersCmd(t, "domains", "list", "-s", uuid, "--json"))
	require.Len(t, *calls, 1)
}

func TestDomainsList_RequiresServer(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "domains", "list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server")
}

func TestDomainsList_UnknownServerListsStaticHostingChoices(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
	})
	err := runServersCmd(t, "domains", "list", "-s", "nope")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "My Site (srv-static)")
	assert.NotContains(t, userErr.Hint, "Production", "only Static Hosting servers are offered")
}

func TestDomainsAdd_SendsWrappedHostname(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase:          {201, domainJSON},
	})

	require.NoError(t, runServersCmd(t, "domains", "add", "WWW.Example.com.", "-s", "srv-static", "--json"))
	require.Len(t, *calls, 2)
	post := (*calls)[1]
	assert.Equal(t, http.MethodPost, post.method)
	assert.Equal(t, map[string]any{"custom_domain": map[string]any{"hostname": "www.example.com"}}, post.body)
}

func TestDomainsAdd_RejectsURL_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "domains", "add", "https://www.example.com/", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "not a hostname")
}

func TestDomainsAdd_PaidPlanRequiredShowsHint(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase: {403, `{"error":"Custom domains are available on paid plans",` +
			`"error_code":"paid_plan_required","hint":"Upgrade your plan to add a custom domain"}`},
	})

	err := runServersCmd(t, "domains", "add", "www.example.com", "-s", "srv-static")
	require.Error(t, err)
	var acct *output.AccountError
	require.True(t, errors.As(output.ExplainAccountError(err), &acct), "got %v", err)
	assert.Equal(t, output.CodePaidPlanRequired, acct.Code)
	assert.Equal(t, "Upgrade your plan to add a custom domain", acct.Hint)
}

func TestDomainsAdd_NotStaticHostingIsExplained(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST /projects/my-app/servers/srv-ssh/custom_domains": {404,
			`{"status":"not_found","error_code":"record_not_found","hint":"Check the identifier"}`},
	})

	err := runServersCmd(t, "domains", "add", "www.example.com", "-s", "Production")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "not a Static Hosting server")
	require.NotEmpty(t, userErr.Recovery)
}

func TestDomainsAdd_ValidationErrorsPassThrough(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase:          {422, `{"errors":["A site can have at most 2 custom domains"]}`},
	})
	err := runServersCmd(t, "domains", "add", "blog.example.com", "-s", "srv-static")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 2 custom domains")
}

func TestDomainsAdd_ServiceUnavailableIsRetryAdvice(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase:          {503, `{"error":"Cloudflare is unavailable"}`},
	})
	err := runServersCmd(t, "domains", "add", "www.example.com", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Equal(t, "Cloudflare is unavailable", userErr.Message)
	assert.Contains(t, userErr.Hint, "retry")
}

func TestDomainsRemove_NonInteractiveNeedsYes_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "domains", "remove", "www.example.com", "-s", "srv-static", "--non-interactive")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "--yes")
}

func TestDomainsRemove_ByHostnameWithYes(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":     {200, domainsServersJSON},
		"GET " + domainsBase:               {200, "[" + domainJSON + "]"},
		"DELETE " + domainsBase + "/dom-1": {202, `{"status":"removal_requested","identifier":"dom-1"}`},
	})

	require.NoError(t, runServersCmd(t, "domains", "remove", "www.example.com", "-s", "My Site", "--yes", "--json"))
	require.Len(t, *calls, 3)
	assert.Equal(t, http.MethodDelete, (*calls)[2].method)
	assert.Equal(t, domainsBase+"/dom-1", (*calls)[2].path)
}

func TestDomainsRemove_UnknownDomainDoesNotDelete(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"GET " + domainsBase:           {200, "[" + domainJSON + "]"},
	})

	err := runServersCmd(t, "domains", "remove", "shop.example.com", "-s", "srv-static", "--yes")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "www.example.com (dom-1)")
	for _, c := range *calls {
		assert.NotEqual(t, http.MethodDelete, c.method)
	}
}

func TestDomainsGlobal_ListResolvesNameAgainstGlobalServers(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers":      {200, globalServersJSON},
		"GET " + globalDomainsBase: {200, "[" + domainJSON + "]"},
	})

	require.NoError(t, runServersCmd(t, "domains", "list", "--global", "-s", "Shared Site", "--json"))
	require.Len(t, *calls, 2)
	assert.Equal(t, "/global_servers", (*calls)[0].path)
	assert.Equal(t, globalDomainsBase, (*calls)[1].path)
}

func TestDomainsGlobal_UUIDSkipsServerListing(t *testing.T) {
	withResolvableContext(t)
	uuid := "0b9f3c2e-1d4a-4c8b-9e2f-123456789abc"
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers/" + uuid + "/custom_domains": {200, "[]"},
	})

	require.NoError(t, runServersCmd(t, "domains", "list", "--global", "-s", uuid, "--json"))
	require.Len(t, *calls, 1)
}

func TestDomainsGlobal_DoesNotNeedAProject(t *testing.T) {
	withResolvableContext(t)
	t.Setenv("DEPLOYHQ_PROJECT", "")
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers":      {200, globalServersJSON},
		"GET " + globalDomainsBase: {200, "[]"},
	})

	require.NoError(t, runServersCmd(t, "domains", "list", "--global", "-s", "srv-global", "--json"))
	require.Len(t, *calls, 2)
}

func TestDomainsGlobal_Show(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers":      {200, globalServersJSON},
		"GET " + globalDomainsBase: {200, "[" + domainJSON + "]"},
	})

	require.NoError(t, runServersCmd(t, "domains", "show", "www.example.com", "--global", "-s", "srv-global", "--json"))
	assert.Equal(t, globalDomainsBase, (*calls)[1].path)
}

func TestDomainsGlobal_Add(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers":       {200, globalServersJSON},
		"POST " + globalDomainsBase: {201, domainJSON},
	})

	require.NoError(t, runServersCmd(t, "domains", "add", "www.example.com", "--global", "-s", "Shared Site", "--json"))
	require.Len(t, *calls, 2)
	assert.Equal(t, http.MethodPost, (*calls)[1].method)
	assert.Equal(t, globalDomainsBase, (*calls)[1].path)
}

func TestDomainsGlobal_Remove(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers":                    {200, globalServersJSON},
		"GET " + globalDomainsBase:               {200, "[" + domainJSON + "]"},
		"DELETE " + globalDomainsBase + "/dom-1": {202, `{"status":"removal_requested","identifier":"dom-1"}`},
	})

	require.NoError(t, runServersCmd(t, "domains", "remove", "www.example.com", "--global", "-s", "srv-global", "--yes", "--json"))
	require.Len(t, *calls, 3)
	assert.Equal(t, globalDomainsBase+"/dom-1", (*calls)[2].path)
}

func TestDomainsGlobal_ErrorsSuggestGlobalCommands(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers":      {200, globalServersJSON},
		"GET " + globalDomainsBase: {404, `{"status":"not_found","error_code":"record_not_found"}`},
	})

	err := runServersCmd(t, "domains", "list", "--global", "-s", "Shared Site")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "dhq global-servers list")
	assert.Contains(t, userErr.Hint, "--global -s 'Shared Site'")
	assert.NotContains(t, userErr.Hint, "-p ")
}

func TestDomainsGlobal_UnknownServerListsChoices(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers": {200, globalServersJSON},
	})

	err := runServersCmd(t, "domains", "list", "--global", "-s", "nope")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "Shared Site (srv-global)")
}

func TestDomainsAdd_WithWWWSendsCounterpart(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase: {201, `{"identifier":"dom-1","hostname":"example.com","primary":true,"status":"pending",` +
			`"counterpart":{"identifier":"dom-2","hostname":"www.example.com","status":"pending"},"counterpart_errors":[]}`},
	})

	require.NoError(t, runServersCmd(t, "domains", "add", "example.com", "-s", "srv-static", "--with-www", "--json"))
	assert.Equal(t, map[string]any{"custom_domain": map[string]any{"hostname": "example.com", "with_counterpart": true}}, (*calls)[1].body)
}

func TestDomainsAdd_CounterpartErrorsDoNotFail(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase: {201, `{"identifier":"dom-1","hostname":"example.com","primary":true,"status":"pending",` +
			`"counterpart":null,"counterpart_errors":["Hostname has already been taken"]}`},
	})

	require.NoError(t, runServersCmd(t, "domains", "add", "example.com", "-s", "srv-static", "--with-www"))
}

func TestDomainsAdd_RateLimitedSaysWhenToRetry(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase:          {429, `{"error":"Too many custom domain changes","retry_after":3600}`},
	})

	err := runServersCmd(t, "domains", "add", "www.example.com", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Equal(t, "Too many custom domain changes", userErr.Message)
	assert.Contains(t, userErr.Hint, "3600 seconds")
}

func TestDomainsCheck_ResolvesHostnameAndAccepts(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":           {200, domainsServersJSON},
		"GET " + domainsBase:                     {200, "[" + domainJSON + "]"},
		"POST " + domainsBase + "/dom-1/refresh": {202, `{"status":"check_requested","identifier":"dom-1"}`},
	})

	require.NoError(t, runServersCmd(t, "domains", "check", "www.example.com", "-s", "My Site", "--json"))
	require.Len(t, *calls, 3)
	assert.Equal(t, http.MethodPost, (*calls)[2].method)
	assert.Equal(t, domainsBase+"/dom-1/refresh", (*calls)[2].path)
}

func TestDomainsCheck_GlobalServer(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /global_servers":                          {200, globalServersJSON},
		"GET " + globalDomainsBase:                     {200, "[" + domainJSON + "]"},
		"POST " + globalDomainsBase + "/dom-1/refresh": {202, `{"status":"check_requested","identifier":"dom-1"}`},
	})

	require.NoError(t, runServersCmd(t, "domains", "check", "dom-1", "--global", "-s", "srv-global", "--json"))
	assert.Equal(t, globalDomainsBase+"/dom-1/refresh", (*calls)[2].path)
}

func TestDomainsCheck_RateLimitedSaysWhenToRetry(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"GET " + domainsBase:           {200, "[" + domainJSON + "]"},
		"POST " + domainsBase + "/dom-1/refresh": {429,
			`{"error":"This domain was checked moments ago. Try again shortly.","retry_after":42}`},
	})

	err := runServersCmd(t, "domains", "check", "www.example.com", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "checked moments ago")
	assert.Contains(t, userErr.Hint, "42 seconds")
	data, ok := output.ErrorResponseFromErr(err).Data.(output.ErrorData)
	require.True(t, ok)
	assert.True(t, data.Retryable, "agents can repeat a rate-limited check")
}

func TestDomainsCheck_NotPendingIsExplained(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"GET " + domainsBase:           {200, "[" + domainJSON + "]"},
		"POST " + domainsBase + "/dom-1/refresh": {422,
			`{"error":"www.example.com is not waiting for verification, so there is nothing to check"}`},
	})

	err := runServersCmd(t, "domains", "check", "www.example.com", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "not waiting for verification")
	assert.Contains(t, userErr.Hint, "dhq domains show www.example.com")
	require.NotEmpty(t, userErr.Recovery)
}

func TestDomainsCheck_UnknownDomainDoesNotPost(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"GET " + domainsBase:           {200, "[" + domainJSON + "]"},
	})

	require.Error(t, runServersCmd(t, "domains", "check", "shop.example.com", "-s", "srv-static"))
	for _, c := range *calls {
		assert.NotEqual(t, http.MethodPost, c.method)
	}
}

// Registration fixtures: while registering the TXT record is absent.
const (
	registeringDomainJSON = `{"identifier":"dom-1","hostname":"www.example.com","primary":true,"status":"pending",` +
		`"registering":true,"ssl_status":null,"verification_errors":[],"activated_at":null,"created_at":"2026-11-05T10:00:00Z",` +
		`"dns_records":[{"type":"CNAME","name":"www.example.com","value":"customers.deployhq-sites.com"}]}`
	registeredDomainJSON = `{"identifier":"dom-1","hostname":"www.example.com","primary":true,"status":"pending",` +
		`"registering":false,"ssl_status":null,"verification_errors":[],"activated_at":null,"created_at":"2026-11-05T10:00:00Z",` +
		`"dns_records":[{"type":"CNAME","name":"www.example.com","value":"customers.deployhq-sites.com"},` +
		`{"type":"TXT","name":"_cf-custom-hostname.www.example.com","value":"abc123"}]}`
	failedDomainJSON = `{"identifier":"dom-1","hostname":"www.example.com","primary":true,"status":"failed",` +
		`"registering":false,"ssl_status":null,"verification_errors":["The hostname could not be registered"],"activated_at":null,` +
		`"created_at":"2026-11-05T10:00:00Z","dns_records":[{"type":"CNAME","name":"www.example.com","value":"customers.deployhq-sites.com"}]}`
)

func fastDomainsPolling(t *testing.T, timeout time.Duration) {
	t.Helper()
	origInterval, origTimeout := pollInterval, domainsRegistrationTimeout
	pollInterval, domainsRegistrationTimeout = time.Millisecond, timeout
	t.Cleanup(func() { pollInterval, domainsRegistrationTimeout = origInterval, origTimeout })
}

// addDomainJSONOutput runs a dhq domains add --json and returns the data of the
// single JSON document it wrote to stdout.
func addDomainJSONOutput(t *testing.T, args ...string) map[string]any {
	t.Helper()
	stdout, err := runCmdCapturingStdout(t, append([]string{"domains", "add", "www.example.com", "-s", "srv-static", "--json"}, args...)...)
	require.NoError(t, err)
	var resp struct {
		Data    map[string]any `json:"data"`
		Summary string         `json:"summary"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &resp), "stdout: %s", stdout)
	resp.Data["__summary"] = resp.Summary
	return resp.Data
}

func dnsRecordTypes(t *testing.T, data map[string]any) []string {
	t.Helper()
	records, ok := data["dns_records"].([]any)
	require.True(t, ok, "dns_records: %v", data["dns_records"])
	types := make([]string, len(records))
	for i, r := range records {
		types[i] = r.(map[string]any)["type"].(string)
	}
	return types
}

func TestDomainsAdd_PollsUntilRegisteredThenPrintsTXT(t *testing.T) {
	withResolvableContext(t)
	fastDomainsPolling(t, 5*time.Second)
	calls := domainsAPISequence(t, map[string][]cannedResponse{
		"GET /projects/my-app/servers": {{200, domainsServersJSON}},
		"POST " + domainsBase:          {{201, registeringDomainJSON}},
		"GET " + domainsBase: {
			{200, "[" + registeringDomainJSON + "]"},
			{200, "[" + registeredDomainJSON + "]"},
		},
	})

	data := addDomainJSONOutput(t)
	assert.Equal(t, false, data["registering"])
	assert.Equal(t, []string{"CNAME", "TXT"}, dnsRecordTypes(t, data))

	polls := 0
	for _, c := range *calls {
		if c.method == http.MethodGet && c.path == domainsBase {
			polls++
		}
	}
	assert.Equal(t, 2, polls)
}

func TestDomainsAdd_PollsTheCounterpartToo(t *testing.T) {
	withResolvableContext(t)
	fastDomainsPolling(t, 5*time.Second)
	counterpart := func(registering string) string {
		return `{"identifier":"dom-2","hostname":"example.com","status":"pending","registering":` + registering + `,"dns_records":[]}`
	}
	domainsAPISequence(t, map[string][]cannedResponse{
		"GET /projects/my-app/servers": {{200, domainsServersJSON}},
		"POST " + domainsBase: {{201, strings.TrimSuffix(registeredDomainJSON, "}") +
			`,"counterpart":` + counterpart("true") + `,"counterpart_errors":[]}`}},
		"GET " + domainsBase: {
			{200, "[" + registeredDomainJSON + "," + counterpart("true") + "]"},
			{200, "[" + registeredDomainJSON + "," + counterpart("false") + "]"},
		},
	})

	data := addDomainJSONOutput(t, "--with-www")
	assert.Equal(t, false, data["counterpart"].(map[string]any)["registering"])
}

func TestDomainsAdd_ToleratesATransientPollFailure(t *testing.T) {
	withResolvableContext(t)
	fastDomainsPolling(t, 5*time.Second)
	domainsAPISequence(t, map[string][]cannedResponse{
		"GET /projects/my-app/servers": {{200, domainsServersJSON}},
		"POST " + domainsBase:          {{201, registeringDomainJSON}},
		"GET " + domainsBase: {
			{503, `{"error":"try later"}`},
			{200, "[" + registeredDomainJSON + "]"},
		},
	})

	data := addDomainJSONOutput(t)
	assert.Equal(t, false, data["registering"])
}

func TestDomainsAdd_FinalPollErrorSaysDomainWasAdded(t *testing.T) {
	withResolvableContext(t)
	fastDomainsPolling(t, 5*time.Second)
	domainsAPISequence(t, map[string][]cannedResponse{
		"GET /projects/my-app/servers": {{200, domainsServersJSON}},
		"POST " + domainsBase:          {{201, registeringDomainJSON}},
		"GET " + domainsBase:           {{403, `{"error":"Forbidden"}`}},
	})

	err := runServersCmd(t, "domains", "add", "www.example.com", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "was added")
	assert.Contains(t, userErr.Hint, "dhq domains show www.example.com")
}

func TestDomainsAdd_TimeoutSaysSoAndExitsZero(t *testing.T) {
	withResolvableContext(t)
	fastDomainsPolling(t, 20*time.Millisecond)
	domainsAPISequence(t, map[string][]cannedResponse{
		"GET /projects/my-app/servers": {{200, domainsServersJSON}},
		"POST " + domainsBase:          {{201, registeringDomainJSON}},
		"GET " + domainsBase:           {{200, "[" + registeringDomainJSON + "]"}},
	})

	data := addDomainJSONOutput(t)
	assert.Equal(t, true, data["registering"])
	assert.Equal(t, []string{"CNAME"}, dnsRecordTypes(t, data))
	assert.Contains(t, data["__summary"], "still running")
	assert.Contains(t, data["__summary"], "dhq domains show www.example.com")
}

func TestDomainsAdd_JSONFieldSelection(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase:          {201, domainJSON},
	})

	stdout, err := runCmdCapturingStdout(t, "domains", "add", "www.example.com", "-s", "srv-static", "--json=status,registering")
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"pending","registering":false}`, stdout)
}

func TestDomainsAdd_NoWaitDoesNotPoll(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPISequence(t, map[string][]cannedResponse{
		"GET /projects/my-app/servers": {{200, domainsServersJSON}},
		"POST " + domainsBase:          {{201, registeringDomainJSON}},
	})

	data := addDomainJSONOutput(t, "--no-wait")
	assert.Equal(t, true, data["registering"])
	assert.Equal(t, []string{"CNAME"}, dnsRecordTypes(t, data))
	require.Len(t, *calls, 2, "no GET of the domains list")
}

func TestDomainsAdd_FieldAbsentDoesNotPoll(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase:          {201, domainJSON},
	})

	data := addDomainJSONOutput(t)
	assert.Equal(t, false, data["registering"])
	assert.Equal(t, []string{"CNAME", "TXT"}, dnsRecordTypes(t, data))
	require.Len(t, *calls, 2, "an older server is never polled")
}

func TestDomainsAdd_FailedDomainPrintsReasonAndFails(t *testing.T) {
	for _, mode := range []struct {
		name string
		args []string
	}{{"text", nil}, {"json", []string{"--json"}}} {
		t.Run(mode.name, func(t *testing.T) {
			withResolvableContext(t)
			fastDomainsPolling(t, 5*time.Second)
			domainsAPISequence(t, map[string][]cannedResponse{
				"GET /projects/my-app/servers": {{200, domainsServersJSON}},
				"POST " + domainsBase:          {{201, registeringDomainJSON}},
				"GET " + domainsBase:           {{200, "[" + failedDomainJSON + "]"}},
			})

			err := runServersCmd(t, append([]string{"domains", "add", "www.example.com", "-s", "srv-static"}, mode.args...)...)
			var userErr *output.UserError
			require.True(t, errors.As(err, &userErr), "got %v", err)
			assert.Contains(t, userErr.Message, "The hostname could not be registered")
			assert.Contains(t, userErr.Hint, "dhq domains remove www.example.com")
			require.NotEmpty(t, userErr.Recovery)
		})
	}
}

func TestDomainsAdd_FailedCounterpartFailsTheCommand(t *testing.T) {
	withResolvableContext(t)
	fastDomainsPolling(t, 5*time.Second)
	failedCounterpart := `{"identifier":"dom-2","hostname":"example.com","status":"failed","registering":false,` +
		`"verification_errors":["Counterpart refused"],"dns_records":[]}`
	registeringCounterpart := `{"identifier":"dom-2","hostname":"example.com","status":"pending","registering":true,"dns_records":[]}`
	domainsAPISequence(t, map[string][]cannedResponse{
		"GET /projects/my-app/servers": {{200, domainsServersJSON}},
		"POST " + domainsBase: {{201, strings.TrimSuffix(registeredDomainJSON, "}") +
			`,"counterpart":` + registeringCounterpart + `,"counterpart_errors":[]}`}},
		"GET " + domainsBase: {{200, "[" + registeredDomainJSON + "," + failedCounterpart + "]"}},
	})

	err := runServersCmd(t, "domains", "add", "www.example.com", "-s", "srv-static", "--with-www")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "example.com: Counterpart refused")
	assert.NotContains(t, userErr.Message, "www.example.com", "the typed domain is fine")
}

func TestDomainsAdd_NoWaitStillReportsAFailedDomain(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + domainsBase:          {201, failedDomainJSON},
	})

	err := runServersCmd(t, "domains", "add", "www.example.com", "-s", "srv-static", "--no-wait")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "could not be registered")
}

func TestDomainsAdd_RemovedWhileWaitingIsAnError(t *testing.T) {
	withResolvableContext(t)
	fastDomainsPolling(t, 5*time.Second)
	domainsAPISequence(t, map[string][]cannedResponse{
		"GET /projects/my-app/servers": {{200, domainsServersJSON}},
		"POST " + domainsBase:          {{201, registeringDomainJSON}},
		"GET " + domainsBase:           {{200, "[]"}},
	})

	err := runServersCmd(t, "domains", "add", "www.example.com", "-s", "srv-static")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer on this server")
}

func TestDomainsAdd_GlobalServerPollsGlobalPath(t *testing.T) {
	withResolvableContext(t)
	fastDomainsPolling(t, 5*time.Second)
	calls := domainsAPISequence(t, map[string][]cannedResponse{
		"GET /global_servers":       {{200, globalServersJSON}},
		"POST " + globalDomainsBase: {{201, registeringDomainJSON}},
		"GET " + globalDomainsBase:  {{200, "[" + registeredDomainJSON + "]"}},
	})

	require.NoError(t, runServersCmd(t, "domains", "add", "www.example.com", "--global", "-s", "srv-global", "--json"))
	assert.Equal(t, globalDomainsBase, (*calls)[2].path)
}

func TestWriteDNSRecords_WhileRegisteringAsksForOnlyTheCNAME(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	name, value := "www.example.com", "customers.deployhq-sites.com"
	d := &sdk.CustomDomain{Hostname: "www.example.com", Registering: true, DNSRecords: []sdk.CustomDomainDNSRecord{
		{Type: "CNAME", Name: &name, Value: &value},
	}}
	writeDNSRecords(env, d, "dhq domains show www.example.com")

	assert.NotContains(t, stderr.String(), "BOTH")
	assert.Contains(t, stderr.String(), "being set up")
	assert.Contains(t, stderr.String(), "dhq domains show www.example.com")
}

func TestDomainsShow_FindsByIdentifierOrHostname(t *testing.T) {
	domains := []sdk.CustomDomain{{Identifier: "dom-1", Hostname: "www.example.com"}}
	assert.NotNil(t, findDomain(domains, "dom-1"))
	assert.NotNil(t, findDomain(domains, "WWW.EXAMPLE.COM."))
	assert.Nil(t, findDomain(domains, "example.com"))
}

func TestWriteDNSRecords_TableAndApexNote(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	cname, target := "example.com", "customers.deployhq-sites.com"
	d := &sdk.CustomDomain{
		Hostname: "example.com",
		DNSRecords: []sdk.CustomDomainDNSRecord{
			{Type: "CNAME", Name: &cname, Value: &target},
			{Type: "TXT"},
		},
	}
	writeDNSRecords(env, d, "dhq domains show example.com -p my-app -s site")

	assert.Contains(t, stdout.String(), "CNAME")
	assert.Contains(t, stdout.String(), "customers.deployhq-sites.com")
	assert.Contains(t, stdout.String(), "(not issued yet)", "a null TXT record renders as not issued")
	assert.Contains(t, stderr.String(), "BOTH")
	assert.Contains(t, stderr.String(), "ALIAS or ANAME")
	assert.Contains(t, stderr.String(), "7 days")
}

func TestWriteDNSRecords_NoApexNoteForSubdomain(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	name, value := "www.example.com", "customers.deployhq-sites.com"
	txtName, txtValue := "_cf.www.example.com", "abc"
	d := &sdk.CustomDomain{Hostname: "www.example.com", DNSRecords: []sdk.CustomDomainDNSRecord{
		{Type: "CNAME", Name: &name, Value: &value},
		{Type: "TXT", Name: &txtName, Value: &txtValue},
	}}
	writeDNSRecords(env, d, "dhq domains show www.example.com")
	assert.NotContains(t, stderr.String(), "apex domain")
	assert.NotContains(t, stderr.String(), "not issued yet")
}
