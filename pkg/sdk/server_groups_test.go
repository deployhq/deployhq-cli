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

func TestListServerGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/server_groups", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]ServerGroup{
			{Identifier: "sg1", Name: "Production", Environment: "production"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	groups, err := c.ListServerGroups(context.Background(), "my-app")
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "Production", groups[0].Name)
}

func TestGetServerGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/server_groups/sg1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(ServerGroup{
			Identifier: "sg1", Name: "Production",
			Servers: []Server{
				{Identifier: "srv1", Name: "Server 1"},
				{Identifier: "srv2", Name: "Server 2"},
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	group, err := c.GetServerGroup(context.Background(), "my-app", "sg1")
	require.NoError(t, err)
	assert.Equal(t, "Production", group.Name)
	assert.Len(t, group.Servers, 2)
}

func TestCreateServerGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body struct {
			ServerGroup ServerGroupCreateRequest `json:"server_group"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Staging", body.ServerGroup.Name)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ServerGroup{Identifier: "sg-new", Name: "Staging"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	group, err := c.CreateServerGroup(context.Background(), "my-app", ServerGroupCreateRequest{Name: "Staging"})
	require.NoError(t, err)
	assert.Equal(t, "Staging", group.Name)
}

func TestDeleteServerGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/projects/my-app/server_groups/sg1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteServerGroup(context.Background(), "my-app", "sg1")
	require.NoError(t, err)
}
