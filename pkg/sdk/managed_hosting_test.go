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

// Capability flags are served from the account sub-object of GET /profile,
// which (unlike GET /account) is readable by any authenticated account member.

func TestGetAccountCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/profile", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"email_address": "dev@example.com",
			"account": map[string]any{
				"name":                    "Example",
				"beta_features":           true,
				"static_hosting_eligible": true,
				"managed_vps_eligible":    true,
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	caps, err := c.GetAccountCapabilities(context.Background())
	require.NoError(t, err)
	assert.True(t, caps.BetaFeatures)
	assert.True(t, caps.StaticHostingEligible)
	assert.True(t, caps.ManagedVPSEligible)
}

func TestGetAccountCapabilities_NotBetaEnrolled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/profile", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": map[string]any{
				"beta_features":           false,
				"static_hosting_eligible": false,
				"managed_vps_eligible":    false,
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	caps, err := c.GetAccountCapabilities(context.Background())
	require.NoError(t, err)
	assert.False(t, caps.BetaFeatures)
	assert.False(t, caps.StaticHostingEligible)
}

func TestGetAccountCapabilities_MissingFieldsDefaultFalse(t *testing.T) {
	// An older backend without the capability fields → they decode to false,
	// and the caller falls back to directing the user to /beta_features.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": map[string]any{"name": "Example"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	caps, err := c.GetAccountCapabilities(context.Background())
	require.NoError(t, err)
	assert.False(t, caps.BetaFeatures)
	assert.False(t, caps.StaticHostingEligible)
	assert.False(t, caps.ManagedVPSEligible)
}

func TestEnrollBeta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/beta/enrollments", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var body BetaEnrollmentRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "static_hosting", body.Protocol)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BetaEnrollmentResponse{
			Enrolled:     true,
			BetaFeatures: true,
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	resp, err := c.EnrollBeta(context.Background(), "static_hosting")
	require.NoError(t, err)
	assert.True(t, resp.Enrolled)
	assert.True(t, resp.BetaFeatures)
}

func TestEnrollBeta_AlreadyEnrolled_Idempotent(t *testing.T) {
	// Non-admin user who is already enrolled — passes idempotently (D8).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BetaEnrollmentResponse{
			Enrolled:     true,
			BetaFeatures: true,
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	resp, err := c.EnrollBeta(context.Background(), "managed_vps")
	require.NoError(t, err)
	assert.True(t, resp.Enrolled)
}

func TestEnrollBeta_AdminRequired(t *testing.T) {
	// Non-admin user who is NOT enrolled — backend returns 403 with structured error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":    "admin_required",
			"beta_url": "https://example.deployhq.com/beta_features",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.EnrollBeta(context.Background(), "static_hosting")
	require.Error(t, err)
	assert.True(t, IsForbidden(err))

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 403, apiErr.StatusCode)
}

func TestEnrollBeta_AllProtocols(t *testing.T) {
	// Empty protocol → enroll in all managed protocols.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body BetaEnrollmentRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Empty(t, body.Protocol, "empty protocol enrolls in all managed protocols")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(BetaEnrollmentResponse{Enrolled: true, BetaFeatures: true})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	resp, err := c.EnrollBeta(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, resp.Enrolled)
}

// The real GET /managed_hosting/regions payload is grouped by continent.
const managedRegionsPayload = `{
	"grouped_regions": {
		"Europe": {
			"default": "lon1",
			"regions": [
				{"slug": "lon1", "name": "London, United Kingdom", "flag": "🇬🇧", "country": "United Kingdom"},
				{"slug": "ams3", "name": "Amsterdam, Netherlands", "flag": "🇳🇱", "country": "Netherlands"}
			]
		},
		"North America": {
			"default": "nyc3",
			"regions": [
				{"slug": "nyc3", "name": "New York City, United States", "flag": "🇺🇸", "country": "United States"}
			]
		}
	}
}`

func TestGetManagedHostingRegions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/managed_hosting/regions", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(managedRegionsPayload))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	resp, err := c.GetManagedHostingRegions(context.Background())
	require.NoError(t, err)
	require.Contains(t, resp.GroupedRegions, "Europe")
	eu := resp.GroupedRegions["Europe"]
	assert.Equal(t, "lon1", eu.Default)
	require.Len(t, eu.Regions, 2)
	assert.Equal(t, "lon1", eu.Regions[0].Slug)
	assert.Equal(t, "United Kingdom", eu.Regions[0].Country)
	assert.Equal(t, "🇬🇧", eu.Regions[0].Flag)
}

func TestListManagedHostingRegionRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(managedRegionsPayload))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	rows, err := c.ListManagedHostingRegionRows(context.Background())
	require.NoError(t, err)
	assert.Len(t, rows, 3)

	// Ordering must be deterministic: groups sorted by name, regions by slug.
	// Europe(ams3, lon1) then North America(nyc3).
	gotOrder := []string{rows[0].Slug, rows[1].Slug, rows[2].Slug}
	assert.Equal(t, []string{"ams3", "lon1", "nyc3"}, gotOrder)

	// Find the lon1 row and assert it is flagged default within its group.
	var lon1 *ManagedHostingRegionRow
	for i := range rows {
		if rows[i].Slug == "lon1" {
			lon1 = &rows[i]
		}
	}
	require.NotNil(t, lon1)
	assert.Equal(t, "Europe", lon1.Group)
	assert.True(t, lon1.IsDefault)
	assert.Equal(t, "United Kingdom", lon1.Country)
}

func TestListManagedHostingRegions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/managed_hosting/regions", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(managedRegionsPayload))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	regions, err := c.ListManagedHostingRegions(context.Background())
	require.NoError(t, err)
	assert.Len(t, regions, 3)
	slugs := make(map[string]bool)
	for _, r := range regions {
		slugs[r.Slug] = true
	}
	assert.True(t, slugs["lon1"])
	assert.True(t, slugs["nyc3"])
	assert.True(t, slugs["ams3"])
}

func TestListManagedHostingSizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/managed_hosting/sizes", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_ = json.NewEncoder(w).Encode(ManagedHostingSizesResponse{
			Sizes: []ManagedHostingSize{
				{Slug: "s-1vcpu-1gb", Label: "1 vCPU / 1 GB RAM", MonthlyCost: 6.0, Currency: "USD"},
				{Slug: "s-2vcpu-2gb", Label: "2 vCPU / 2 GB RAM", MonthlyCost: 12.0, Currency: "USD"},
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	sizes, err := c.ListManagedHostingSizes(context.Background())
	require.NoError(t, err)
	assert.Len(t, sizes, 2)
	assert.Equal(t, "s-1vcpu-1gb", sizes[0].Slug)
	assert.Equal(t, 6.0, sizes[0].MonthlyCost)
	assert.Equal(t, "USD", sizes[0].Currency)
	assert.Equal(t, "1 vCPU / 1 GB RAM", sizes[0].Label)
}

func TestListManagedHostingSizes_PricingUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Pricing temporarily unavailable."})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.ListManagedHostingSizes(context.Background())
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
}

func TestGetServerProvisioningState_ManagedVPS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/servers/srv-vps", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Server{
			Identifier:   "srv-vps",
			Name:         "My VPS",
			ProtocolType: "managed_vps",
			ManagedVPS: &ManagedVPSInfo{
				HostedResourceIdentifier: "hr-1",
				Status:                   "active",
				IPAddress:                "203.0.113.10",
				Region:                   "lon1",
				Size:                     "s-1vcpu-1gb",
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.GetServerProvisioningState(context.Background(), "my-app", "srv-vps")
	require.NoError(t, err)
	assert.Equal(t, "managed_vps", s.ProtocolType)
	assert.Equal(t, "active", ProvisioningStatus(s))
	assert.True(t, IsProvisioningActive(s))
	require.NotNil(t, s.ManagedVPS)
	assert.Equal(t, "203.0.113.10", s.ManagedVPS.IPAddress)
	assert.Equal(t, "http://203.0.113.10", LiveURL(s))
}

func TestGetServerProvisioningState_StaticHosting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/servers/srv-static", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Server{
			Identifier:   "srv-static",
			Name:         "My Site",
			ProtocolType: "static_hosting",
			StaticHosting: &StaticHostingInfo{
				URL:       "https://my-app.deployhq-sites.com",
				Subdomain: "my-app",
				Status:    "active",
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.GetServerProvisioningState(context.Background(), "my-app", "srv-static")
	require.NoError(t, err)
	assert.Equal(t, "static_hosting", s.ProtocolType)
	assert.Equal(t, "active", ProvisioningStatus(s))
	require.NotNil(t, s.StaticHosting)
	assert.Equal(t, "https://my-app.deployhq-sites.com", LiveURL(s))
}

func TestGetServerProvisioningState_Provisioning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Server{
			Identifier:   "srv-vps",
			ProtocolType: "managed_vps",
			ManagedVPS:   &ManagedVPSInfo{Status: "provisioning"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.GetServerProvisioningState(context.Background(), "my-app", "srv-vps")
	require.NoError(t, err)
	assert.True(t, IsProvisioning(s))
	assert.False(t, IsProvisioningActive(s))
}

// --- LiveURL / status helpers ---

func TestLiveURL(t *testing.T) {
	tests := []struct {
		name     string
		server   *Server
		expected string
	}{
		{
			name:     "nil server",
			server:   nil,
			expected: "",
		},
		{
			name:     "static_hosting with url",
			server:   &Server{ProtocolType: "static_hosting", StaticHosting: &StaticHostingInfo{URL: "https://my-app.deployhq-sites.com"}},
			expected: "https://my-app.deployhq-sites.com",
		},
		{
			name:     "static_hosting no StaticHosting block",
			server:   &Server{ProtocolType: "static_hosting"},
			expected: "",
		},
		{
			name:     "managed_vps with ip",
			server:   &Server{ProtocolType: "managed_vps", ManagedVPS: &ManagedVPSInfo{IPAddress: "203.0.113.10"}},
			expected: "http://203.0.113.10",
		},
		{
			name:     "managed_vps no ip yet",
			server:   &Server{ProtocolType: "managed_vps", ManagedVPS: &ManagedVPSInfo{}},
			expected: "",
		},
		{
			name:     "ssh server",
			server:   &Server{ProtocolType: "ssh", Hostname: "prod.example.com"},
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, LiveURL(tt.server))
		})
	}
}

func TestIsProvisioning(t *testing.T) {
	tests := []struct {
		name     string
		server   *Server
		expected bool
	}{
		{"nil", nil, false},
		{"managed_vps provisioning", &Server{ProtocolType: "managed_vps", ManagedVPS: &ManagedVPSInfo{Status: "provisioning"}}, true},
		{"managed_vps active", &Server{ProtocolType: "managed_vps", ManagedVPS: &ManagedVPSInfo{Status: "active"}}, false},
		{"static_hosting provisioning", &Server{ProtocolType: "static_hosting", StaticHosting: &StaticHostingInfo{Status: "provisioning"}}, true},
		{"static_hosting active", &Server{ProtocolType: "static_hosting", StaticHosting: &StaticHostingInfo{Status: "active"}}, false},
		{"ssh server", &Server{ProtocolType: "ssh"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsProvisioning(tt.server))
		})
	}
}

func TestIsProvisioningActive(t *testing.T) {
	tests := []struct {
		name     string
		server   *Server
		expected bool
	}{
		{"nil", nil, false},
		{"managed_vps active", &Server{ProtocolType: "managed_vps", ManagedVPS: &ManagedVPSInfo{Status: "active"}}, true},
		{"managed_vps provisioning", &Server{ProtocolType: "managed_vps", ManagedVPS: &ManagedVPSInfo{Status: "provisioning"}}, false},
		{"static_hosting active", &Server{ProtocolType: "static_hosting", StaticHosting: &StaticHostingInfo{Status: "active"}}, true},
		{"ssh server", &Server{ProtocolType: "ssh"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsProvisioningActive(tt.server))
		})
	}
}
