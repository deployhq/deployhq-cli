package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIPRanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ip_ranges", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(`{
			"shared": {"ipv4": ["1.2.3.0/24"], "ipv6": ["2001:db8::/32"]},
			"zones": {
				"eu": {"name": "Europe", "ipv4": ["10.0.0.0/24"], "ipv6": ["2001:db9::/32"]}
			},
			"network_agent": {"ipv4": ["5.6.7.0/24"]},
			"all_ipv4": ["1.2.3.0/24", "10.0.0.0/24", "5.6.7.0/24"],
			"all_ipv6": ["2001:db8::/32", "2001:db9::/32"],
			"ports": [{"protocol": "ssh", "description": "SSH deployments"}]
		}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	ranges, err := c.GetIPRanges(context.Background())
	require.NoError(t, err)
	assert.Len(t, ranges.AllIPv4, 3)
	assert.Len(t, ranges.AllIPv6, 2)
	assert.Equal(t, "Europe", ranges.Zones["eu"].Name)
	assert.Equal(t, "ssh", ranges.Ports[0].Protocol)
	assert.Equal(t, []string{"5.6.7.0/24"}, ranges.NetworkAgent.IPv4)
}
