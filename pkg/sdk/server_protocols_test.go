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

// createServerBody mirrors the wire shape CreateServer sends: the generic server
// object plus the managed-resource provisioning params as TOP-LEVEL siblings of
// `server` (the backend reads params[:region], params[:hosted_website_attributes],
// etc. — not params[:server][:region]).
type createServerBody struct {
	Server                  ServerCreateRequest      `json:"server"`
	HostedWebsiteAttributes *HostedWebsiteAttributes `json:"hosted_website_attributes"`
	Region                  string                   `json:"region"`
	Size                    string                   `json:"size"`
	OSImage                 string                   `json:"os_image"`
}

// TestCreateServer_StaticHosting verifies the static_hosting request shape:
// hosted_website_attributes is a top-level sibling of `server` (subdomain,
// spa_mode, subdirectory), and no VPS params are present.
func TestCreateServer_StaticHosting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/servers", r.URL.Path)

		var body createServerBody
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		assert.Equal(t, "static_hosting", body.Server.ProtocolType)
		require.NotNil(t, body.HostedWebsiteAttributes, "hosted_website_attributes must be a top-level sibling of server")
		assert.Equal(t, "my-site", body.HostedWebsiteAttributes.Subdomain)
		assert.Equal(t, "dist", body.HostedWebsiteAttributes.Subdirectory)
		assert.True(t, body.HostedWebsiteAttributes.SPAMode)

		// VPS params must be absent
		assert.Empty(t, body.Region)
		assert.Empty(t, body.Size)
		assert.Empty(t, body.OSImage)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{
			Identifier:   "srv-static",
			Name:         "My Site",
			ProtocolType: "static_hosting",
			StaticHosting: &StaticHostingInfo{
				URL:       "https://my-site.deployhq-sites.com",
				Subdomain: "my-site",
				Status:    "provisioning",
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name:         "My Site",
		ProtocolType: "static_hosting",
		HostedWebsiteAttributes: &HostedWebsiteAttributes{
			Subdomain:    "my-site",
			Subdirectory: "dist",
			SPAMode:      true,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "srv-static", s.Identifier)
	require.NotNil(t, s.StaticHosting)
	assert.Equal(t, "https://my-site.deployhq-sites.com", s.StaticHosting.URL)
}

// TestCreateServer_ManagedVPS verifies the managed_vps request shape:
// region, size, and os_image are top-level siblings of `server` (not nested).
func TestCreateServer_ManagedVPS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body createServerBody
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		assert.Equal(t, "managed_vps", body.Server.ProtocolType)
		assert.Equal(t, "lon1", body.Region)
		assert.Equal(t, "s-1vcpu-1gb", body.Size)
		assert.Equal(t, "ubuntu-24-04-x64", body.OSImage)

		// Static hosting attributes must be absent
		assert.Nil(t, body.HostedWebsiteAttributes)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{
			Identifier:   "srv-vps",
			Name:         "My VPS",
			ProtocolType: "managed_vps",
			ManagedVPS:   &ManagedVPSInfo{Status: "provisioning"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name:         "My VPS",
		ProtocolType: "managed_vps",
		Region:       "lon1",
		Size:         "s-1vcpu-1gb",
		OSImage:      "ubuntu-24-04-x64",
	})
	require.NoError(t, err)
	assert.Equal(t, "srv-vps", s.Identifier)
	assert.Equal(t, "provisioning", ProvisioningStatus(s))
}

// TestCreateServer_ManagedVPS_DefaultOSImage verifies that os_image is omitted
// from the request entirely when not set — the backend defaults it to
// ubuntu-24-04-x64 (OQ3 hardcoded default), the CLI does not force-fill it.
func TestCreateServer_ManagedVPS_DefaultOSImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body createServerBody
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		// OSImage omitted → not hoisted → absent from the top-level body.
		assert.Empty(t, body.OSImage)
		assert.Equal(t, "lon1", body.Region)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{Identifier: "srv-vps", ProtocolType: "managed_vps"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name:         "My VPS",
		ProtocolType: "managed_vps",
		Region:       "lon1",
		Size:         "s-1vcpu-1gb",
		// OSImage intentionally omitted — backend defaults to ubuntu-24-04-x64
	})
	require.NoError(t, err)
	assert.Equal(t, "srv-vps", s.Identifier)
}

// TestServerProvisioningFields_Unmarshal verifies the Server type deserialises
// the nested managed_vps provisioning block from the project-scoped server show.
func TestServerProvisioningFields_Unmarshal(t *testing.T) {
	raw := `{
		"identifier": "srv-vps",
		"name": "My VPS",
		"protocol_type": "managed_vps",
		"enabled": true,
		"managed_vps": {
			"hosted_resource_identifier": "hr-1",
			"status": "active",
			"ip_address": "203.0.113.10",
			"region": "lon1",
			"size": "s-1vcpu-1gb"
		}
	}`
	var s Server
	require.NoError(t, json.Unmarshal([]byte(raw), &s))
	assert.Equal(t, "active", ProvisioningStatus(&s))
	require.NotNil(t, s.ManagedVPS)
	assert.Equal(t, "203.0.113.10", s.ManagedVPS.IPAddress)
	assert.Nil(t, s.StaticHosting)
	assert.Equal(t, "http://203.0.113.10", LiveURL(&s))
}

func TestStaticHostingInfo_Unmarshal(t *testing.T) {
	raw := `{
		"identifier": "srv-sh",
		"name": "My Site",
		"protocol_type": "static_hosting",
		"static_hosting": {
			"url": "https://my-app.deployhq-sites.com",
			"subdomain": "my-app",
			"status": "active"
		}
	}`
	var s Server
	require.NoError(t, json.Unmarshal([]byte(raw), &s))
	assert.Equal(t, "active", ProvisioningStatus(&s))
	require.NotNil(t, s.StaticHosting)
	assert.Equal(t, "https://my-app.deployhq-sites.com", s.StaticHosting.URL)
	assert.Equal(t, "my-app", s.StaticHosting.Subdomain)
	assert.Nil(t, s.ManagedVPS)
}
