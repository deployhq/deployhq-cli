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

// The key identifier is a TOP-LEVEL provisioning param, a sibling of `server` —
// the backend reads params[:key_pair_identifier], not params[:server][...].
// Sending it nested is a silent no-op: Rails' permit list for Servers::ManagedVps
// drops unknown nested keys and still returns 2xx.
func TestCreateServer_ManagedVPS_KeyPairIdentifierIsTopLevel(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{Identifier: "srv-1", Name: "vps"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name:              "vps",
		ProtocolType:      "managed_vps",
		Region:            "lon1",
		Size:              "s-1vcpu-1gb",
		KeyPairIdentifier: "key-uuid-123",
	})
	require.NoError(t, err)

	assert.Equal(t, "key-uuid-123", body["key_pair_identifier"], "must be a top-level sibling of server")

	server := body["server"].(map[string]any)
	assert.NotContains(t, server, "key_pair_identifier", "must NOT be nested inside server")
	assert.NotContains(t, server, "key_pair_id", "the internal id is never sent")
}

func TestCreateServer_OmittedKeyPairIdentifierIsAbsent(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{Identifier: "srv-1"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name: "vps", ProtocolType: "managed_vps", Region: "lon1", Size: "s-1vcpu-1gb",
	})
	require.NoError(t, err)
	assert.NotContains(t, body, "key_pair_identifier")
}

// Read-back: the CLI must surface which key was applied, and never key material.
func TestServer_ManagedVPS_SSHKeyReadBack(t *testing.T) {
	raw := `{
	  "identifier": "srv-1",
	  "name": "vps",
	  "managed_vps": {
	    "hosted_resource_identifier": "hr-1",
	    "status": "active",
	    "region": "lon1",
	    "size": "s-1vcpu-1gb",
	    "ssh_key": {"identifier": "key-uuid-123", "title": "ops key", "fingerprint": "SHA256:abc"}
	  }
	}`
	var s Server
	require.NoError(t, json.Unmarshal([]byte(raw), &s))
	require.NotNil(t, s.ManagedVPS)
	require.NotNil(t, s.ManagedVPS.SSHKey)
	assert.Equal(t, "key-uuid-123", s.ManagedVPS.SSHKey.Identifier)
	assert.Equal(t, "ops key", s.ManagedVPS.SSHKey.Title)
	assert.Equal(t, "SHA256:abc", s.ManagedVPS.SSHKey.Fingerprint)
}

func TestServer_ManagedVPS_SSHKeyAbsent(t *testing.T) {
	var s Server
	require.NoError(t, json.Unmarshal([]byte(`{"identifier":"srv-1","managed_vps":{"status":"provisioning"}}`), &s))
	require.NotNil(t, s.ManagedVPS)
	assert.Nil(t, s.ManagedVPS.SSHKey, "absent ssh_key must stay nil, not a zero-valued struct")
}
