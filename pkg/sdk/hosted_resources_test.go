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

func TestListHostedResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/hosted_resources", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		// Top-level JSON array unioning both kinds.
		_, _ = w.Write([]byte(`[
			{
				"kind": "hosted_resource",
				"identifier": "hr-1",
				"name": "my-vps",
				"region": "lon1",
				"size": "s-1vcpu-1gb",
				"status": "active",
				"ip_address": "203.0.113.10",
				"monthly_cost": 6.0,
				"ssh_key": {
					"identifier": "key-1",
					"title": "vps key",
					"public_key": "ssh-ed25519 AAAA",
					"key_type": "ed25519",
					"fingerprint": "aa:bb:cc",
					"account": "acme"
				},
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-01-02T00:00:00Z"
			},
			{
				"kind": "hosted_website",
				"identifier": "hw-1",
				"name": "my-site",
				"subdomain": "my-site",
				"status": "provisioning",
				"monthly_cost": 0.0,
				"spa_mode": true,
				"storage_bytes_total": 1048576,
				"active_deployment_uuid": null,
				"deployment_count": 3,
				"created_at": "2026-01-03T00:00:00Z",
				"updated_at": "2026-01-04T00:00:00Z"
			},
			{
				"kind": "hosted_resource",
				"identifier": "hr-2",
				"name": "pending-vps",
				"region": null,
				"size": null,
				"status": "provisioning",
				"ip_address": null,
				"monthly_cost": 12.0,
				"ssh_key": null,
				"created_at": "2026-01-05T00:00:00Z",
				"updated_at": "2026-01-05T00:00:00Z"
			}
		]`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	resources, err := c.ListHostedResources(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, resources, 3)

	// hosted_resource with populated nullable fields + ssh_key.
	vps := resources[0]
	assert.Equal(t, "hosted_resource", vps.Kind)
	assert.Equal(t, "hr-1", vps.Identifier)
	assert.Equal(t, "active", vps.Status)
	assert.Equal(t, 6.0, vps.MonthlyCost)
	require.NotNil(t, vps.Region)
	assert.Equal(t, "lon1", *vps.Region)
	require.NotNil(t, vps.IPAddress)
	assert.Equal(t, "203.0.113.10", *vps.IPAddress)
	require.NotNil(t, vps.SSHKey)
	assert.Equal(t, "key-1", vps.SSHKey.Identifier)
	assert.Equal(t, "ed25519", vps.SSHKey.KeyType)

	// hosted_website discriminated by kind.
	site := resources[1]
	assert.Equal(t, "hosted_website", site.Kind)
	assert.Equal(t, "my-site", site.Subdomain)
	assert.True(t, site.SPAMode)
	assert.Equal(t, 1048576, site.StorageBytesTotal)
	assert.Equal(t, 3, site.DeploymentCount)
	assert.Nil(t, site.ActiveDeploymentUUID)
	assert.Empty(t, site.Region) // region pointer is nil for websites

	// hosted_resource with null nullable fields.
	pending := resources[2]
	assert.Nil(t, pending.Region)
	assert.Nil(t, pending.Size)
	assert.Nil(t, pending.IPAddress)
	assert.Nil(t, pending.SSHKey)
}

func TestGetHostedResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/hosted_resources/hr-1", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_ = json.NewEncoder(w).Encode(HostedResource{
			Kind:       "hosted_resource",
			Identifier: "hr-1",
			Name:       "my-vps",
			Status:     "active",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	res, err := c.GetHostedResource(context.Background(), "hr-1")
	require.NoError(t, err)
	assert.Equal(t, "hr-1", res.Identifier)
	assert.Equal(t, "hosted_resource", res.Kind)
}

func TestSyncHostedResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/hosted_resources/hr-1/sync", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "sync_requested"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.SyncHostedResource(context.Background(), "hr-1")
	require.NoError(t, err)
}

func TestRetryProvisionHostedResource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/hosted_resources/hr-1/retry_provision", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "provisioning"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.RetryProvisionHostedResource(context.Background(), "hr-1")
	require.NoError(t, err)
}

func TestRetryProvisionHostedResource_NotInErrorState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Only resources in error state can be retried.",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.RetryProvisionHostedResource(context.Background(), "hr-1")
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	assert.Equal(t, "Only resources in error state can be retried.", apiErr.Message)
}

func TestListHostedResources_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.ListHostedResources(context.Background(), nil)
	require.Error(t, err)
	assert.True(t, IsForbidden(err))
}

func TestGetHostedResource_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.GetHostedResource(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
}
