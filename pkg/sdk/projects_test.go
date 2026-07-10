package sdk

import (
	"context"
	"encoding/json"
	"io"
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
		_ = json.NewEncoder(w).Encode([]Project{
			{Name: "My App", Permalink: "my-app", Identifier: "abc123", Zone: "us-east"},
			{Name: "Other App", Permalink: "other-app", Identifier: "def456", Zone: "eu-west"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	projects, err := c.ListProjects(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, projects, 2)
	assert.Equal(t, "My App", projects[0].Name)
	assert.Equal(t, "abc123", projects[0].Identifier)
}

func TestGetProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Project{
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
		_ = json.NewEncoder(w).Encode(Project{Name: "New Project", Permalink: "new-project", Identifier: "new123"})
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
		_ = json.NewEncoder(w).Encode(Project{Name: "Updated App", Permalink: "my-app"})
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

func TestRegenerateProjectKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/projects/my-app/regenerate_key", r.URL.Path)

		var body struct {
			Project struct {
				KeyType string `json:"key_type"`
			} `json:"project"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "ED25519", body.Project.KeyType)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"public_key": "ssh-ed25519 AAAA..."})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	key, err := c.RegenerateProjectKey(context.Background(), "my-app", "ED25519")
	require.NoError(t, err)
	assert.Equal(t, "ssh-ed25519 AAAA...", key)
}

func TestRegenerateProjectKey_NoKeyType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// key_type is omitempty, so an empty value must not appear in the body.
		assert.NotContains(t, string(body), "key_type")
		_ = json.NewEncoder(w).Encode(map[string]string{"public_key": "ssh-rsa AAAA..."})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	key, err := c.RegenerateProjectKey(context.Background(), "my-app", "")
	require.NoError(t, err)
	assert.Equal(t, "ssh-rsa AAAA...", key)
}

func TestGetUndeployedChanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/projects/my-app/undeployed_changes", r.URL.Path)
		_, _ = w.Write([]byte(`{
			"count": 2,
			"truncated": false,
			"latest_deployed_revision": "abc123",
			"current_revision": "def456",
			"last_checked_at": "2026-01-01T00:00:00Z",
			"check_in_progress": false,
			"check_fail_message": null,
			"commits": [
				{"ref":"def456","author":"Alice","email":"a@x.com","timestamp":"t","message":"Fix bug","short_message":"Fix bug","url":"http://x"},
				{"ref":"aaa111","author":"Bob","email":"b@x.com","timestamp":"t","message":"Add feature","short_message":"Add feature","url":"http://y"}
			]
		}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	changes, err := c.GetUndeployedChanges(context.Background(), "my-app")
	require.NoError(t, err)
	assert.Equal(t, 2, changes.Count)
	assert.False(t, changes.Truncated)
	require.NotNil(t, changes.LatestDeployedRevision)
	assert.Equal(t, "abc123", *changes.LatestDeployedRevision)
	assert.Nil(t, changes.CheckFailMessage)
	require.Len(t, changes.Commits, 2)
	assert.Equal(t, "Alice", changes.Commits[0].Author)
	assert.Equal(t, "Fix bug", changes.Commits[0].Message)
}

func TestGenerateAIDeploymentOverview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/ai_deployment_overview", r.URL.Path)

		// Body must be FLAT (not wrapped in "project").
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "abc123", body["start_ref"])
		assert.Equal(t, "def456", body["end_ref"])
		_, hasProject := body["project"]
		assert.False(t, hasProject, "body should be flat, not wrapped in project")

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":           true,
			"overview":          "This deployment fixes bugs.",
			"ai_prompt_version": "v2",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	overview, err := c.GenerateAIDeploymentOverview(context.Background(), "my-app", "abc123", "def456")
	require.NoError(t, err)
	assert.Equal(t, "This deployment fixes bugs.", overview)
}

func TestGenerateAIDeploymentOverview_RequiresEndRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request should be made when end_ref is empty")
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.GenerateAIDeploymentOverview(context.Background(), "my-app", "abc123", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end_ref is required")
}
