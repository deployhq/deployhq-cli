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

func TestGetIPRanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ip_ranges", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_ = json.NewEncoder(w).Encode(IPRanges{
			Shared: IPRangeSet{IPv4: []string{"1.2.3.0/24"}, IPv6: []string{"2001:db8::/32"}},
			Zones: map[string]IPRangeZone{
				"eu": {Name: "Europe", IPv4: []string{"10.0.0.0/24"}, IPv6: []string{"2001:db9::/32"}},
			},
			NetworkAgent: IPRangeSet{IPv4: []string{"5.6.7.0/24"}},
			AllIPv4:      []string{"1.2.3.0/24", "10.0.0.0/24", "5.6.7.0/24"},
			AllIPv6:      []string{"2001:db8::/32", "2001:db9::/32"},
			Ports:        []IPRangePort{{Protocol: "ssh", Description: "SSH deployments"}},
		})
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
