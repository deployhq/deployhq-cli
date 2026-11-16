package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const redirectRuleJSON = `{"identifier":"rr-1","position":1,"source":"/","destination":"/posts/:id",` +
	`"status":301,"query":"p=:id","shadowed_by":null}`

const redirectRulesJSON = `{
  "rules": [
    ` + redirectRuleJSON + `,
    {"identifier":"rr-2","position":2,"source":"/old","destination":"/new","status":302,"query":null,
     "shadowed_by":{"source":"/old","line":4}}
  ],
  "file_rules": [{"source":"/blog/*","destination":"/news/:splat","status":301,"query":null,"line":1}],
  "file_warnings": [{"line":3,"code":"rewrite_unsupported","reason":"200 rewrites are not supported"}, "file too large"],
  "deployment_uuid": "dep-uuid"
}`

func TestListRedirectRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/redirect_rules", r.URL.Path)
		_, _ = w.Write([]byte(redirectRulesJSON))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	rules, err := c.ListRedirectRules(context.Background(), ProjectServerRedirects("my-app", "srv1"))
	require.NoError(t, err)

	require.Len(t, rules.Rules, 2)
	first := rules.Rules[0]
	assert.Equal(t, "rr-1", first.Identifier)
	assert.Equal(t, 301, first.Status)
	require.NotNil(t, first.Query)
	assert.Equal(t, "p=:id", *first.Query)
	assert.Nil(t, first.ShadowedBy)
	assert.Nil(t, rules.Rules[1].Query)
	require.NotNil(t, rules.Rules[1].ShadowedBy)
	require.NotNil(t, rules.Rules[1].ShadowedBy.Line)
	assert.Equal(t, 4, *rules.Rules[1].ShadowedBy.Line)
	assert.Nil(t, rules.Rules[1].ShadowedBy.RuleIdentifier)

	require.Len(t, rules.FileRules, 1)
	assert.Equal(t, 1, rules.FileRules[0].Line)

	require.Len(t, rules.FileWarnings, 2)
	require.NotNil(t, rules.FileWarnings[0].Line)
	assert.Equal(t, 3, *rules.FileWarnings[0].Line)
	assert.Equal(t, "rewrite_unsupported", rules.FileWarnings[0].Code)
	assert.Equal(t, "file too large", rules.FileWarnings[1].Reason, "a plain-text warning is accepted")
	assert.Nil(t, rules.FileWarnings[1].Line)

	require.NotNil(t, rules.DeploymentUUID)
	assert.Equal(t, "dep-uuid", *rules.DeploymentUUID)
}

func TestListRedirectRules_BeforeFirstDeploy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/global_servers/srv1/redirect_rules", r.URL.Path)
		_, _ = w.Write([]byte(`{"rules":[],"file_rules":[],"file_warnings":[],"deployment_uuid":null}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	rules, err := c.ListRedirectRules(context.Background(), GlobalServerRedirects("srv1"))
	require.NoError(t, err)
	assert.Empty(t, rules.Rules)
	assert.Nil(t, rules.DeploymentUUID)
}

func TestCreateRedirectRule_WrapsRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/redirect_rules", r.URL.Path)
		var body map[string]map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]map[string]any{"redirect_rule": {
			"source": "/", "destination": "/posts/:id", "status": float64(301), "query": "p=:id", "position": float64(1),
		}}, body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(redirectRuleJSON))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	pos := 1
	rule, err := c.CreateRedirectRule(context.Background(), ProjectServerRedirects("my-app", "srv1"), RedirectRuleCreateRequest{
		Source: "/", Destination: "/posts/:id", Status: 301, Query: "p=:id", Position: &pos,
	})
	require.NoError(t, err)
	assert.Equal(t, "rr-1", rule.Identifier)
}

func TestCreateRedirectRule_OmitsUnsetOptionals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]any{"source": "/old", "destination": "/new"}, body["redirect_rule"])
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(redirectRuleJSON))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateRedirectRule(context.Background(), ProjectServerRedirects("my-app", "srv1"), RedirectRuleCreateRequest{Source: "/old", Destination: "/new"})
	require.NoError(t, err)
}

func TestUpdateRedirectRule_SendsOnlySetFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/redirect_rules/rr-1", r.URL.Path)
		var body map[string]map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]any{"status": float64(308), "query": ""}, body["redirect_rule"],
			"an empty query is sent, to remove the matcher")
		_, _ = w.Write([]byte(redirectRuleJSON))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	status, query := 308, ""
	_, err := c.UpdateRedirectRule(context.Background(), ProjectServerRedirects("my-app", "srv1"), "rr-1",
		RedirectRuleUpdateRequest{Status: &status, Query: &query})
	require.NoError(t, err)
}

func TestDeleteRedirectRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/global_servers/srv1/redirect_rules/rr-1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	require.NoError(t, c.DeleteRedirectRule(context.Background(), GlobalServerRedirects("srv1"), "rr-1"))
}

func TestReorderRedirectRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/redirect_rules/reorder", r.URL.Path)
		var body map[string][]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, []string{"rr-2", "rr-1"}, body["identifiers"])
		_, _ = w.Write([]byte(`{"rules":[{"identifier":"rr-2","position":1},{"identifier":"rr-1","position":2}]}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	rules, err := c.ReorderRedirectRules(context.Background(), ProjectServerRedirects("my-app", "srv1"), []string{"rr-2", "rr-1"})
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "rr-2", rules[0].Identifier)
}

func TestUpdateRedirectRule_SendsPosition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]any{"position": float64(1)}, body["redirect_rule"])
		_, _ = w.Write([]byte(redirectRuleJSON))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	position := 1
	_, err := c.UpdateRedirectRule(context.Background(), ProjectServerRedirects("my-app", "srv1"), "rr-1",
		RedirectRuleUpdateRequest{Position: &position})
	require.NoError(t, err)
}

// shadowed_by names either a _redirects line or an earlier DeployHQ rule that
// takes the rule over completely (line null, rule_identifier set).
func TestRedirectRuleShadow_EarlierRule(t *testing.T) {
	var rule RedirectRule
	require.NoError(t, json.Unmarshal([]byte(`{"identifier":"rr-2","position":2,"source":"/","destination":"/x","status":301,`+
		`"query":"p=:id","shadowed_by":{"source":"/","line":null,"rule_identifier":"rr-1"}}`), &rule))
	require.NotNil(t, rule.ShadowedBy)
	assert.Nil(t, rule.ShadowedBy.Line)
	require.NotNil(t, rule.ShadowedBy.RuleIdentifier)
	assert.Equal(t, "rr-1", *rule.ShadowedBy.RuleIdentifier)
}
