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

// A pending domain as the API returns it: the TXT record's name and value
// are null until DeployHQ has issued them.
const pendingDomainJSON = `{
  "identifier": "dom-1",
  "hostname": "www.example.com",
  "primary": true,
  "status": "pending",
  "ssl_status": null,
  "verification_errors": [],
  "activated_at": null,
  "created_at": "2026-11-05T10:00:00Z",
  "dns_records": [
    {"type": "CNAME", "name": "www.example.com", "value": "customers.deployhq-sites.com"},
    {"type": "TXT", "name": null, "value": null}
  ]
}`

func TestListCustomDomains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/custom_domains", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte("[" + pendingDomainJSON + "]"))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	domains, err := c.ListCustomDomains(context.Background(), ProjectServerDomains("my-app", "srv1"))
	require.NoError(t, err)
	require.Len(t, domains, 1)

	d := domains[0]
	assert.Equal(t, "dom-1", d.Identifier)
	assert.Equal(t, "www.example.com", d.Hostname)
	assert.True(t, d.Primary)
	assert.Equal(t, "pending", d.Status)
	assert.Nil(t, d.SSLStatus)
	assert.Nil(t, d.ActivatedAt)
	require.Len(t, d.DNSRecords, 2)
	assert.Equal(t, "CNAME", d.DNSRecords[0].Type)
	require.NotNil(t, d.DNSRecords[0].Value)
	assert.Equal(t, "customers.deployhq-sites.com", *d.DNSRecords[0].Value)
	assert.Equal(t, "TXT", d.DNSRecords[1].Type)
	assert.Nil(t, d.DNSRecords[1].Name)
	assert.Nil(t, d.DNSRecords[1].Value)
}

func TestCreateCustomDomain_WrapsHostname(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/custom_domains", r.URL.Path)

		var body map[string]map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]map[string]string{
			"custom_domain": {"hostname": "www.example.com"},
		}, body)

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(pendingDomainJSON))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	d, err := c.CreateCustomDomain(context.Background(), ProjectServerDomains("my-app", "srv1"), CustomDomainCreateRequest{Hostname: "www.example.com"})
	require.NoError(t, err)
	assert.Equal(t, "dom-1", d.Identifier)
	assert.Equal(t, "pending", d.Status)
}

func TestDeleteCustomDomain_ReadsAcceptedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/custom_domains/dom-1", r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"removal_requested","identifier":"dom-1"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	removal, err := c.DeleteCustomDomain(context.Background(), ProjectServerDomains("my-app", "srv1"), "dom-1")
	require.NoError(t, err)
	assert.Equal(t, "removal_requested", removal.Status)
	assert.Equal(t, "dom-1", removal.Identifier)
}

func TestCreateCustomDomain_PaidPlanRequired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Custom domains are available on paid plans",` +
			`"error_code":"paid_plan_required","hint":"Upgrade your plan to add a custom domain"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateCustomDomain(context.Background(), ProjectServerDomains("my-app", "srv1"), CustomDomainCreateRequest{Hostname: "www.example.com"})
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.True(t, apiErr.IsPaidPlanRequired())
	assert.Equal(t, "Upgrade your plan to add a custom domain", apiErr.Hint)
}

func TestCreateCustomDomain_ValidationErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["Hostname has already been taken"]}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateCustomDomain(context.Background(), ProjectServerDomains("my-app", "srv1"), CustomDomainCreateRequest{Hostname: "www.example.com"})
	require.Error(t, err)
	assert.Equal(t, "deployhq api: 422 Hostname has already been taken", err.Error())
}

func TestCustomDomains_GlobalServerPaths(t *testing.T) {
	const base = "/global_servers/srv1/custom_domains"
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte("[" + pendingDomainJSON + "]"))
		case r.Method == http.MethodPost && r.URL.Path == base:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(pendingDomainJSON))
		default:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted","identifier":"dom-1"}`))
		}
	}))
	defer server.Close()

	c := newTestClient(t, server)
	ctx := context.Background()
	scope := GlobalServerDomains("srv1")

	_, err := c.ListCustomDomains(ctx, scope)
	require.NoError(t, err)
	_, err = c.CreateCustomDomain(ctx, scope, CustomDomainCreateRequest{Hostname: "www.example.com"})
	require.NoError(t, err)
	_, err = c.RefreshCustomDomain(ctx, scope, "dom-1")
	require.NoError(t, err)
	_, err = c.DeleteCustomDomain(ctx, scope, "dom-1")
	require.NoError(t, err)

	assert.Equal(t, []string{
		"GET " + base,
		"POST " + base,
		"POST " + base + "/dom-1/refresh",
		"DELETE " + base + "/dom-1",
	}, calls)
}

func TestCreateCustomDomain_WithCounterpart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]map[string]interface{}{
			"custom_domain": {"hostname": "example.com", "with_counterpart": true},
		}, body)

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"identifier":"dom-1","hostname":"example.com","status":"pending",` +
			`"counterpart":{"identifier":"dom-2","hostname":"www.example.com","status":"pending"},"counterpart_errors":[]}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	created, err := c.CreateCustomDomain(context.Background(), ProjectServerDomains("my-app", "srv1"),
		CustomDomainCreateRequest{Hostname: "example.com", WithCounterpart: true})
	require.NoError(t, err)
	assert.Equal(t, "dom-1", created.Identifier)
	require.NotNil(t, created.Counterpart)
	assert.Equal(t, "dom-2", created.Counterpart.Identifier)
}

func TestCustomDomain_Registering(t *testing.T) {
	var absent, present CustomDomain
	require.NoError(t, json.Unmarshal([]byte(pendingDomainJSON), &absent))
	assert.False(t, absent.Registering, "an older server omits the field")
	require.NoError(t, json.Unmarshal([]byte(`{"registering":true}`), &present))
	assert.True(t, present.Registering)
}

func TestRefreshCustomDomain_RetryAfterFromBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/custom_domains/dom-1/refresh", r.URL.Path)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"This domain was checked moments ago. Try again shortly.","retry_after":42}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.RefreshCustomDomain(context.Background(), ProjectServerDomains("my-app", "srv1"), "dom-1")
	apiErr, ok := err.(*APIError)
	require.True(t, ok, "got %v", err)
	assert.True(t, apiErr.IsRateLimited())
	assert.Equal(t, 42, apiErr.RetryAfter)
}

func TestRefreshCustomDomain_HeaderWinsOverBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"slow down","retry_after":42}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.RefreshCustomDomain(context.Background(), ProjectServerDomains("my-app", "srv1"), "dom-1")
	apiErr, ok := err.(*APIError)
	require.True(t, ok, "got %v", err)
	assert.Equal(t, 7, apiErr.RetryAfter)
}
