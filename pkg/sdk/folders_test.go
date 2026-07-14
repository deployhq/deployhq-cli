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

func TestListFolders(t *testing.T) {
	pos := 2
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/folders", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]Folder{
			{Identifier: "uuid-1", Name: "Internal Tools", Position: nil, ProjectsCount: 0},
			{Identifier: "uuid-2", Name: "Client Sites", Position: &pos, ProjectsCount: 3},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	folders, err := c.ListFolders(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, folders, 2)

	// position round-trips as *int, including null.
	assert.Equal(t, "Internal Tools", folders[0].Name)
	assert.Nil(t, folders[0].Position)
	assert.Equal(t, 0, folders[0].ProjectsCount)

	assert.Equal(t, "Client Sites", folders[1].Name)
	require.NotNil(t, folders[1].Position)
	assert.Equal(t, 2, *folders[1].Position)
	assert.Equal(t, 3, folders[1].ProjectsCount)
}

func TestCreateFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/folders", r.URL.Path)

		// Request body must be wrapped: {"folder":{"name":"..."}}.
		var body struct {
			Folder FolderCreateRequest `json:"folder"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Internal Tools", body.Folder.Name)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Folder{Identifier: "uuid-new", Name: "Internal Tools", Position: nil, ProjectsCount: 0})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	folder, err := c.CreateFolder(context.Background(), FolderCreateRequest{Name: "Internal Tools"})
	require.NoError(t, err)
	assert.Equal(t, "uuid-new", folder.Identifier)
	assert.Equal(t, "Internal Tools", folder.Name)
	assert.Nil(t, folder.Position)
}

func TestCreateFolder_ValidationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string][]string{"name": {"has already been taken"}})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateFolder(context.Background(), FolderCreateRequest{Name: "Internal Tools"})
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 422, apiErr.StatusCode)
	assert.True(t, apiErr.IsValidationError())
}

func TestUpdateFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/folders/uuid-1", r.URL.Path)

		var body struct {
			Folder FolderCreateRequest `json:"folder"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Renamed Folder", body.Folder.Name)

		_ = json.NewEncoder(w).Encode(Folder{Identifier: "uuid-1", Name: "Renamed Folder", Position: nil, ProjectsCount: 1})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	folder, err := c.UpdateFolder(context.Background(), "uuid-1", FolderCreateRequest{Name: "Renamed Folder"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed Folder", folder.Name)
	assert.Equal(t, 1, folder.ProjectsCount)
}

func TestDeleteFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/folders/uuid-1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteFolder(context.Background(), "uuid-1")
	require.NoError(t, err)
}

// The backend requires unrestricted-global (or team-global with no exclusions)
// access to rename a folder; an under-privileged API user gets a 403.
func TestUpdateFolder_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.UpdateFolder(context.Background(), "uuid-1", FolderCreateRequest{Name: "Renamed"})
	require.Error(t, err)
	assert.True(t, IsForbidden(err))

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 403, apiErr.StatusCode)
}

// An unknown identifier — or one belonging to another account (cross-account
// isolation) — is indistinguishable to the caller: the backend returns 404.
func TestUpdateFolder_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.UpdateFolder(context.Background(), "not-a-real-identifier", FolderCreateRequest{Name: "X"})
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
}

func TestDeleteFolder_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteFolder(context.Background(), "other-account-folder")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
}
