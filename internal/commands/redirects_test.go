package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	redirectsBase = "/projects/my-app/servers/srv-static/redirect_rules"

	redirectRuleJSON = `{"identifier":"rr-1","position":1,"source":"/","destination":"/posts/:id",` +
		`"status":301,"query":"p=:id","shadowed_by":null}`
	redirectRulesJSON = `{"rules":[` + redirectRuleJSON + `,` +
		`{"identifier":"rr-2","position":2,"source":"/old","destination":"/new","status":302,"query":null,` +
		`"shadowed_by":{"source":"/old","line":4}}],` +
		`"file_rules":[{"source":"/blog/*","destination":"/news/:splat","status":301,"query":null,"line":1}],` +
		`"file_warnings":[{"line":3,"code":"rewrite_unsupported","reason":"200 rewrites are not supported"}],` +
		`"deployment_uuid":"dep-uuid"}`
)

func TestRedirectsCmd_Structure(t *testing.T) {
	cmd := newServersRedirectsCmd()
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
		assert.NotNil(t, sub.Flags().Lookup("server"), "%s needs --server", sub.Name())
		// Static Hosting sites can't be global servers, so there is no --global.
		assert.Nil(t, sub.Flags().Lookup("global"), "%s must not offer --global", sub.Name())
	}
	for _, want := range []string{"list", "add", "update", "remove", "reorder"} {
		assert.True(t, names[want], "missing subcommand %q", want)
	}
}

func TestRedirectsHelp_ExplainsFileVersusCommands(t *testing.T) {
	cmd := newServersRedirectsCmd()
	for _, want := range []string{"_redirects", "file wins", "301", "308", "--query", "p=:id", "can't be global servers"} {
		assert.Contains(t, cmd.Long, want)
	}
}

func TestRedirectsList_ResolvesServerAndListsJSON(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"GET " + redirectsBase:         {200, redirectRulesJSON},
	})

	stdout, err := runCmdCapturingStdout(t, "servers", "redirects", "list", "-s", "My Site", "--json")
	require.NoError(t, err)
	require.Len(t, *calls, 2)
	assert.Equal(t, redirectsBase, (*calls)[1].path)

	var resp struct {
		Data sdk.RedirectRules `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &resp), "stdout: %s", stdout)
	require.Len(t, resp.Data.Rules, 2)
	require.Len(t, resp.Data.FileRules, 1)
	require.Len(t, resp.Data.FileWarnings, 1)
	require.NotNil(t, resp.Data.Rules[1].ShadowedBy)
}

func TestRedirectsList_UsesServerFromProjectConfig(t *testing.T) {
	withResolvableContext(t)
	inFolderLinkedTo(t, "my-app", "srv-static")
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"GET " + redirectsBase:         {200, redirectRulesJSON},
	})

	require.NoError(t, runServersCmd(t, "servers", "redirects", "list", "--json"))
	assert.Equal(t, redirectsBase, (*calls)[len(*calls)-1].path)
}

func TestRedirectsList_NotStaticHostingIsExplained(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":                        {200, domainsServersJSON},
		"GET /projects/my-app/servers/srv-ssh/redirect_rules": {404, `{"error":"Not found"}`},
	})

	err := runServersCmd(t, "servers", "redirects", "list", "-s", "Production")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "not a Static Hosting server")
	assert.Contains(t, userErr.Hint, "dhq servers list -p my-app")
}

func TestWriteRedirectFile_ShowsFileRulesAndWarnings(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	var rules sdk.RedirectRules
	require.NoError(t, json.Unmarshal([]byte(redirectRulesJSON), &rules))

	writeRedirectFile(env, &rules)
	assert.Contains(t, stdout.String(), "/news/:splat")
	assert.Contains(t, stderr.String(), "checked first")
	assert.Contains(t, stderr.String(), "line 3: 200 rewrites are not supported")
}

func TestWriteRedirectFile_BeforeFirstDeploy(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	writeRedirectFile(env, &sdk.RedirectRules{})
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "once the site has been deployed")
}

func TestWarnShadowed_NamesTheFileLine(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	var rules sdk.RedirectRules
	require.NoError(t, json.Unmarshal([]byte(redirectRulesJSON), &rules))

	warnShadowed(env, rules.Rules)
	assert.Contains(t, stderr.String(), "Rule 2 (/old)")
	assert.Contains(t, stderr.String(), "line 4 of the _redirects file (/old)")
	assert.NotContains(t, stderr.String(), "Rule 1")
}

func TestRedirectsAdd_SendsWrappedRule(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + redirectsBase:        {201, redirectRuleJSON},
	})

	require.NoError(t, runServersCmd(t, "servers", "redirects", "add", "/", "/posts/:id",
		"--query", "p=:id", "--status", "308", "--position", "1", "-s", "srv-static", "--json"))
	post := (*calls)[len(*calls)-1]
	assert.Equal(t, http.MethodPost, post.method)
	assert.Equal(t, map[string]any{"redirect_rule": map[string]any{
		"source": "/", "destination": "/posts/:id", "query": "p=:id", "status": float64(308), "position": float64(1),
	}}, post.body)
}

func TestRedirectsAdd_OmitsUnsetFlags(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + redirectsBase:        {201, redirectRuleJSON},
	})

	require.NoError(t, runServersCmd(t, "servers", "redirects", "add", "/old", "/new", "-s", "srv-static", "--json"))
	post := (*calls)[len(*calls)-1]
	assert.Equal(t, map[string]any{"redirect_rule": map[string]any{"source": "/old", "destination": "/new"}}, post.body,
		"status and position are left to the API's defaults")
}

func TestRedirectsAdd_RejectsBadStatus_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "servers", "redirects", "add", "/old", "/new", "--status", "200", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "--status 200")
	assert.Contains(t, userErr.Hint, "301")
}

func TestRedirectsAdd_RejectsBadPosition_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "servers", "redirects", "add", "/old", "/new", "--position", "0", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "--position 0")
}

func TestRedirectsAdd_ValidationErrorsPassThrough(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + redirectsBase:        {422, `{"errors":["Source must start with /"]}`},
	})

	err := runServersCmd(t, "servers", "redirects", "add", "old", "/new", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "Source must start with /")
	assert.Contains(t, userErr.Hint, "dhq servers redirects list -p my-app -s srv-static")
}

func TestRedirectsUpdate_SendsOnlyGivenFlags(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":     {200, domainsServersJSON},
		"PATCH " + redirectsBase + "/rr-1": {200, redirectRuleJSON},
	})

	require.NoError(t, runServersCmd(t, "servers", "redirects", "update", "rr-1",
		"--destination", "/articles/:id", "--query", "", "-s", "srv-static", "--json"))
	patch := (*calls)[len(*calls)-1]
	assert.Equal(t, http.MethodPatch, patch.method)
	assert.Equal(t, map[string]any{"redirect_rule": map[string]any{"destination": "/articles/:id", "query": ""}}, patch.body,
		`only the given flags are sent; --query "" removes the matcher`)
}

func TestRedirectsUpdate_StatusOnly(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":     {200, domainsServersJSON},
		"PATCH " + redirectsBase + "/rr-1": {200, redirectRuleJSON},
	})

	require.NoError(t, runServersCmd(t, "servers", "redirects", "update", "rr-1", "--status", "302", "-s", "srv-static", "--json"))
	assert.Equal(t, map[string]any{"redirect_rule": map[string]any{"status": float64(302)}}, (*calls)[len(*calls)-1].body)
}

func TestRedirectsUpdate_NothingToChange_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "servers", "redirects", "update", "rr-1", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "Nothing to change")
}

func TestRedirectsUpdate_RejectsBadStatus_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "servers", "redirects", "update", "rr-1", "--status", "404", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "--status 404")
}

func TestRedirectsRemove_NonInteractiveNeedsYes_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "servers", "redirects", "remove", "rr-1", "-s", "srv-static", "--json")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "--yes")
}

func TestRedirectsRemove_WithYes(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":      {200, domainsServersJSON},
		"GET " + redirectsBase:              {200, redirectRulesJSON},
		"DELETE " + redirectsBase + "/rr-2": {204, ``},
	})

	stdout, err := runCmdCapturingStdout(t, "servers", "redirects", "remove", "rr-2", "-s", "srv-static", "--yes", "--json")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, (*calls)[len(*calls)-1].method)
	assert.Contains(t, stdout, `"removed"`)
}

func TestRedirectsRemove_UnknownRuleDoesNotDelete(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"GET " + redirectsBase:         {200, redirectRulesJSON},
	})

	err := runServersCmd(t, "servers", "redirects", "remove", "rr-9", "-s", "srv-static", "--yes")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, `"rr-9"`)
	assert.Contains(t, userErr.Hint, "rr-1 (/)")
	for _, c := range *calls {
		assert.NotEqual(t, http.MethodDelete, c.method)
	}
}

func TestRedirectsReorder_SendsIdentifiersInOrder(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":      {200, domainsServersJSON},
		"PUT " + redirectsBase + "/reorder": {200, `{"rules":[{"identifier":"rr-2","position":1},{"identifier":"rr-1","position":2}]}`},
	})

	require.NoError(t, runServersCmd(t, "servers", "redirects", "reorder", "rr-2", "rr-1", "-s", "srv-static", "--json"))
	put := (*calls)[len(*calls)-1]
	assert.Equal(t, map[string]any{"identifiers": []any{"rr-2", "rr-1"}}, put.body)
}

func TestRedirectsReorder_RefusalPassesThrough(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":      {200, domainsServersJSON},
		"PUT " + redirectsBase + "/reorder": {422, `{"error":"Identifiers must list every rule once"}`},
	})

	err := runServersCmd(t, "servers", "redirects", "reorder", "rr-2", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "must list every rule once")
}

func TestRedirectsUpdate_PositionMovesTheRule(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":     {200, domainsServersJSON},
		"PATCH " + redirectsBase + "/rr-1": {200, redirectRuleJSON},
	})

	require.NoError(t, runServersCmd(t, "servers", "redirects", "update", "rr-1", "--position", "2", "-s", "srv-static", "--json"))
	assert.Equal(t, map[string]any{"redirect_rule": map[string]any{"position": float64(2)}}, (*calls)[len(*calls)-1].body,
		"--position alone moves the rule without changing it")
}

func TestRedirectsUpdate_RejectsBadPosition_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "servers", "redirects", "update", "rr-1", "--position", "0", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "--position 0")
}

// The API refuses a reorder that does not name every rule exactly once; the
// hint is about that, not about how to write a rule.
func TestRedirectsReorder_IncompleteListIsExplained(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":      {200, domainsServersJSON},
		"PUT " + redirectsBase + "/reorder": {422, `{"errors":["List every redirect rule of the site exactly once, in the new order."]}`},
	})

	err := runServersCmd(t, "servers", "redirects", "reorder", "rr-2", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "exactly once")
	assert.Contains(t, userErr.Hint, "every rule")
	assert.Contains(t, userErr.Hint, "dhq servers redirects list -p my-app -s srv-static")
	assert.NotContains(t, userErr.Hint, "A source is a path")
}

func TestRedirectsReorder_HelpSaysEveryRule(t *testing.T) {
	cmd := newRedirectsReorderCmd()
	assert.Contains(t, cmd.Long, "every rule")
}

// A site being deleted refuses every change with a 422 that has nothing to do
// with the rule itself.
func TestRedirects_SiteBeingDeletedIsNotARuleProblem(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + redirectsBase:        {422, `{"errors":["This site is being deleted"]}`},
	})

	err := runServersCmd(t, "servers", "redirects", "add", "/old", "/new", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "This site is being deleted")
	assert.NotContains(t, userErr.Message, "Redirect rule refused")
	assert.NotContains(t, userErr.Hint, "A source is a path")
}

func TestRedirectsAdd_PublishFailureIsRetryable(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + redirectsBase:        {503, `{"error":"The redirect rules could not be published, so nothing was changed. Try again in a moment."}`},
	})

	err := runServersCmd(t, "servers", "redirects", "add", "/old", "/new", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.True(t, userErr.Retryable)
	assert.Contains(t, userErr.Message, "nothing was changed")
	assert.Contains(t, userErr.Hint, "Nothing was changed")
}

// shadowed_by is also set when a file rule takes over only some of the URLs a
// rule matches, so the warning must not say the rule never applies.
func TestWarnShadowed_DoesNotOverstate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	var rules sdk.RedirectRules
	require.NoError(t, json.Unmarshal([]byte(redirectRulesJSON), &rules))

	warnShadowed(env, rules.Rules)
	assert.NotContains(t, stderr.String(), "never applies")
	assert.Contains(t, stderr.String(), "line 4")
}

// A deployment or another change holds the site while it publishes: every
// change is refused with 409 and nothing changes, so it is worth retrying.
func TestRedirectsAdd_SiteBusyIsRetryable(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + redirectsBase:        {409, `{"error":"The site is busy publishing a deployment or another change, so nothing was changed. Try again in a moment."}`},
	})

	err := runServersCmd(t, "servers", "redirects", "add", "/old", "/new", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.True(t, userErr.Retryable)
	assert.Contains(t, userErr.Message, "The site is busy")
	assert.Contains(t, userErr.Hint, "Nothing was changed")
}

func TestRedirectsReorder_SiteBusyIsRetryable(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":      {200, domainsServersJSON},
		"PUT " + redirectsBase + "/reorder": {409, `{"error":"The site is busy publishing a deployment or another change, so nothing was changed. Try again in a moment."}`},
	})

	err := runServersCmd(t, "servers", "redirects", "reorder", "rr-2", "rr-1", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.True(t, userErr.Retryable)
}

func TestRedirectsAdd_RateLimitIsRetryable(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers": {200, domainsServersJSON},
		"POST " + redirectsBase:        {429, `{"error":"Too many requests","retry_after":30}`},
	})

	err := runServersCmd(t, "servers", "redirects", "add", "/old", "/new", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.True(t, userErr.Retryable)
	assert.Equal(t, 30, userErr.RetryAfter)
}

func TestRedirectsUpdate_UnknownRuleIsExplained(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"GET /projects/my-app/servers":     {200, domainsServersJSON},
		"PATCH " + redirectsBase + "/rr-9": {404, `{"status":"not_found","error_code":"not_found"}`},
	})

	err := runServersCmd(t, "servers", "redirects", "update", "rr-9", "--status", "302", "-s", "srv-static")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Message, "the redirect rule does not exist")
	assert.Contains(t, userErr.Hint, "dhq servers redirects list -p my-app -s srv-static")
}

// An earlier DeployHQ rule that takes a rule over completely means it never
// applies; the warning names that rule rather than a _redirects line.
func TestWarnShadowed_EarlierRuleNeverApplies(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	earlier := "rr-1"
	warnShadowed(env, []sdk.RedirectRule{{Identifier: "rr-2", Position: 2, Source: "/",
		ShadowedBy: &sdk.RedirectRuleShadow{Source: "/", RuleIdentifier: &earlier}}})
	assert.Contains(t, stderr.String(), "Rule 2 (/) never applies: earlier rule rr-1 (/) matches first")
	assert.NotContains(t, stderr.String(), "_redirects")
}

// Notes about lines that were applied in a normalised form are not skipped
// lines.
func TestWriteRedirectFile_SeparatesNotesFromSkippedLines(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
	var rules sdk.RedirectRules
	require.NoError(t, json.Unmarshal([]byte(`{"rules":[],"file_rules":[{"source":"/a","destination":"/b","status":301,"query":null,"line":1}],`+
		`"file_warnings":[{"line":2,"code":"rewrite_unsupported","reason":"200 rewrites are not supported"},`+
		`{"line":1,"code":"source_normalised","reason":"/a/ was applied as /a, the address visitors see."}],"deployment_uuid":"dep"}`), &rules))

	writeRedirectFile(env, &rules)
	out := stderr.String()
	skipped := strings.Index(out, "were skipped")
	notes := strings.Index(out, "were applied")
	require.True(t, skipped >= 0 && notes >= 0, out)
	assert.Contains(t, out[skipped:notes], "line 2")
	assert.NotContains(t, out[skipped:notes], "line 1:")
	assert.Contains(t, out[notes:], "line 1: /a/ was applied as /a")
}
