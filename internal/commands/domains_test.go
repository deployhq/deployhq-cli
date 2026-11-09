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
	var (
		mu    sync.Mutex
		calls []domainsCall
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
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
		resp, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return respond(http.StatusInternalServerError, `{"error":"unexpected"}`), nil
		}
		return respond(resp.status, resp.body), nil
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
