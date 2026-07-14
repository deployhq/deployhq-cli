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
		_ = json.NewEncoder(w).Encode([]Server{
			{ID: 1, Identifier: "srv1", Name: "Production", ProtocolType: "ssh", Enabled: true},
			{ID: 2, Identifier: "srv2", Name: "Staging", ProtocolType: "ftp", Enabled: true},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	servers, err := c.ListServers(context.Background(), "my-app", nil)
	require.NoError(t, err)
	assert.Len(t, servers, 2)
	assert.Equal(t, "Production", servers[0].Name)
	assert.Equal(t, "ssh", servers[0].ProtocolType)
}

func TestGetServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/servers/srv1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Server{
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
		_ = json.NewEncoder(w).Encode(Server{Identifier: "new-srv", Name: "New Server", ProtocolType: "ssh"})
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

func TestCreateServerFromGlobal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/servers/from_global", r.URL.Path)

		// Body must be FLAT (top-level global_server_id).
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "glob-1", body["global_server_id"])
		_, hasServer := body["server"]
		assert.False(t, hasServer, "body should be flat, not wrapped in server")

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Server{Identifier: "srv-new", Name: "From Global", ProtocolType: "ssh"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	s, err := c.CreateServerFromGlobal(context.Background(), "my-app", "glob-1")
	require.NoError(t, err)
	assert.Equal(t, "srv-new", s.Identifier)
	assert.Equal(t, "From Global", s.Name)
}

func TestGetServerMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/projects/my-app/servers/srv1/server_metrics", r.URL.Path)
		// Mirrors the real (deep-camelized) server_metrics payload: uptime is a
		// nested {formatted, seconds} object, disk is an array of partition
		// objects — NOT a string / map.
		// Mirrors the full deep-camelized server_metrics payload: all nine keys,
		// with uptime an object, disk an array, and last_deploy/status objects.
		_, _ = w.Write([]byte(`{
			"status": {"online": true, "message": "reachable"},
			"cpu": {"usage": 12.5, "loadAverages": [0.1, 0.2, 0.3]},
			"memory": {"used": 1024, "total": 4096},
			"disk": [
				{"mount": "/", "used": 50, "total": 100, "percentage": 50, "formattedUsed": "50 GB"},
				{"mount": "/data", "used": 10, "total": 200, "percentage": 5}
			],
			"uptime": {"formatted": "5 days", "seconds": 432000},
			"lastDeploy": {"uuid": "dep-1", "status": "completed"},
			"profile": {"cores": 2},
			"sharedHosting": false,
			"hostname": "web-1.example.com"
		}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	m, err := c.GetServerMetrics(context.Background(), "my-app", "srv1")
	require.NoError(t, err)
	assert.Equal(t, "5 days", m.Uptime.Formatted)
	assert.Equal(t, int64(432000), m.Uptime.Seconds)
	assert.Len(t, m.Disk, 2)
	assert.Equal(t, "/", m.Disk[0]["mount"])
	assert.Equal(t, 12.5, m.CPU["usage"])
	assert.Equal(t, float64(2), m.Profile["cores"])
	// The four previously-dropped fields must now round-trip.
	assert.Equal(t, true, m.Status["online"])
	assert.Equal(t, "dep-1", m.LastDeploy["uuid"])
	assert.False(t, m.SharedHosting)
	assert.Equal(t, "web-1.example.com", m.Hostname)
}

func TestGetServerMetrics_NotAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Not available"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.GetServerMetrics(context.Background(), "my-app", "srv1")
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}
