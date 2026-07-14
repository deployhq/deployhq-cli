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

func TestDownloadSSHKeyPrivateKey(t *testing.T) {
	const pem = "-----BEGIN OPENSSH PRIVATE KEY-----\nabc123\n-----END OPENSSH PRIVATE KEY-----"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/ssh_keys/key-1/download_private_key", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]string{"private_key": pem})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	key, err := c.DownloadSSHKeyPrivateKey(context.Background(), "key-1")
	require.NoError(t, err)
	assert.Equal(t, pem, key)
}

// The endpoint is gated on admin + paid account; a free or non-admin account
// receives a 403 with an explanatory error.
func TestDownloadSSHKeyPrivateKey_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "This feature is only available on paid accounts."})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.DownloadSSHKeyPrivateKey(context.Background(), "key-1")
	require.Error(t, err)
	assert.True(t, IsForbidden(err))
}

// A 200 with an empty private_key yields an empty string; the command layer
// turns this into an explicit error rather than writing an empty file.
func TestDownloadSSHKeyPrivateKey_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"private_key": ""})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	key, err := c.DownloadSSHKeyPrivateKey(context.Background(), "key-1")
	require.NoError(t, err)
	assert.Empty(t, key)
}

func TestDownloadSSHKeyPrivateKey_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.DownloadSSHKeyPrivateKey(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
}
