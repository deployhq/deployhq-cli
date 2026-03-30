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

func TestListServers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/servers", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		json.NewEncoder(w).Encode([]Server{
			{ID: 1, Identifier: "srv1", Name: "Production", ProtocolType: "ssh", Enabled: true},
			{ID: 2, Identifier: "srv2", Name: "Staging", ProtocolType: "ftp", Enabled: true},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	servers, err := c.ListServers(context.Background(), "my-app")
	require.NoError(t, err)
	assert.Len(t, servers, 2)
	assert.Equal(t, "Production", servers[0].Name)
	assert.Equal(t, "ssh", servers[0].ProtocolType)
}

func TestGetServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/servers/srv1", r.URL.Path)
		json.NewEncoder(w).Encode(Server{
			ID: 1, Identifier: "srv1", Name: "Production",
			ProtocolType: "ssh", ServerPath: "/var/www", Enabled: true,
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.GetServer(context.Background(), "my-app", "srv1")
	require.NoError(t, err)
	assert.Equal(t, "Production", s.Name)
	assert.Equal(t, "/var/www", s.ServerPath)
}

func TestCreateServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body struct {
			Server ServerCreateRequest `json:"server"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "New Server", body.Server.Name)
		assert.Equal(t, "ssh", body.Server.ProtocolType)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Server{Identifier: "new-srv", Name: "New Server", ProtocolType: "ssh"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.CreateServer(context.Background(), "my-app", ServerCreateRequest{
		Name: "New Server", ProtocolType: "ssh",
	})
	require.NoError(t, err)
	assert.Equal(t, "New Server", s.Name)
}

func TestDeleteServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteServer(context.Background(), "my-app", "srv1")
	require.NoError(t, err)
}

func TestResetServerHostKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/reset_host_key", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.ResetServerHostKey(context.Background(), "my-app", "srv1")
	require.NoError(t, err)
}
