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

func TestListPackages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/packages", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(`[
			{
				"permalink": "small", "name": "Small", "currency": "USD",
				"price": 25, "price_billed_annually": 20,
				"features": ["10 projects", "Unlimited deployments"]
			},
			{"permalink": "medium", "name": "Medium", "currency": "USD", "price": 50, "price_billed_annually": 42}
		]`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	pkgs, err := c.ListPackages(context.Background())
	require.NoError(t, err)
	assert.Len(t, pkgs, 2)
	assert.Equal(t, "Small", pkgs[0].Name)
	assert.Equal(t, 25.0, pkgs[0].Price)
	assert.Equal(t, 20.0, pkgs[0].PriceBilledAnnually)
	assert.Equal(t, "USD", pkgs[0].Currency)
	assert.Len(t, pkgs[0].Features, 2)
}

func TestListPackagesUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "pricing unavailable"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.ListPackages(context.Background())
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
}
