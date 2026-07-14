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

func TestLinkGlobalConfigFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/config_files/link_global", r.URL.Path)

		// Body must be FLAT (top-level config_file_id).
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "cfg-1", body["config_file_id"])
		_, hasWrapper := body["config_file"]
		assert.False(t, hasWrapper, "body should be flat, not wrapped in config_file")

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ConfigFile{Identifier: "cfg-1", Path: "/etc/app.conf", Description: "Linked"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	f, err := c.LinkGlobalConfigFile(context.Background(), "my-app", "cfg-1")
	require.NoError(t, err)
	assert.Equal(t, "cfg-1", f.Identifier)
	assert.Equal(t, "/etc/app.conf", f.Path)
}

func TestUpdateConfigFile_DescriptionOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/projects/my-app/config_files/cf1", r.URL.Path)
		// A description-only update must NOT send empty path/body, which would
		// clear the existing values server-side.
		var raw map[string]map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		cf := raw["config_file"]
		assert.Equal(t, "new desc", cf["description"])
		_, hasPath := cf["path"]
		_, hasBody := cf["body"]
		assert.False(t, hasPath, "path must be omitted when unchanged")
		assert.False(t, hasBody, "body must be omitted when unchanged")
		_ = json.NewEncoder(w).Encode(ConfigFile{Identifier: "cf1", Path: "/etc/app.conf", Description: "new desc"})
	}))
	defer server.Close()

	desc := "new desc"
	c := newTestClient(t, server)
	f, err := c.UpdateConfigFile(context.Background(), "my-app", "cf1", ConfigFileUpdateRequest{Description: &desc})
	require.NoError(t, err)
	assert.Equal(t, "new desc", f.Description)
}

func TestUnlinkGlobalConfigFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/projects/my-app/config_files/unlink_global", r.URL.Path)

		// DELETE carries a FLAT body.
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "cfg-1", body["config_file_id"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.UnlinkGlobalConfigFile(context.Background(), "my-app", "cfg-1")
	require.NoError(t, err)
}
