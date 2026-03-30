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

func TestListProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		json.NewEncoder(w).Encode([]Project{
			{Name: "My App", Permalink: "my-app", Identifier: "abc123", Zone: "us-east"},
			{Name: "Other App", Permalink: "other-app", Identifier: "def456", Zone: "eu-west"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	projects, err := c.ListProjects(context.Background())
	require.NoError(t, err)
	assert.Len(t, projects, 2)
	assert.Equal(t, "My App", projects[0].Name)
	assert.Equal(t, "abc123", projects[0].Identifier)
}

func TestGetProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app", r.URL.Path)
		json.NewEncoder(w).Encode(Project{
			Name: "My App", Permalink: "my-app", Identifier: "abc123",
			AutoDeployURL: "https://deployhq.com/deploy/abc123",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	project, err := c.GetProject(context.Background(), "my-app")
	require.NoError(t, err)
	assert.Equal(t, "My App", project.Name)
	assert.Equal(t, "abc123", project.Identifier)
}

func TestCreateProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects", r.URL.Path)

		var body struct {
			Project ProjectCreateRequest `json:"project"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "New Project", body.Project.Name)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Project{Name: "New Project", Permalink: "new-project", Identifier: "new123"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	project, err := c.CreateProject(context.Background(), ProjectCreateRequest{Name: "New Project"})
	require.NoError(t, err)
	assert.Equal(t, "New Project", project.Name)
}

func TestUpdateProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/projects/my-app", r.URL.Path)
		json.NewEncoder(w).Encode(Project{Name: "Updated App", Permalink: "my-app"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	project, err := c.UpdateProject(context.Background(), "my-app", ProjectUpdateRequest{Name: "Updated App"})
	require.NoError(t, err)
	assert.Equal(t, "Updated App", project.Name)
}

func TestDeleteProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/projects/my-app", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteProject(context.Background(), "my-app")
	require.NoError(t, err)
}

func TestStarProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/star", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.StarProject(context.Background(), "my-app")
	require.NoError(t, err)
}
