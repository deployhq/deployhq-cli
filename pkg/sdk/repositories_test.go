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

func TestGetRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/repository", r.URL.Path)
		json.NewEncoder(w).Encode(Repository{
			ScmType: "git",
			URL:     "git@github.com:myco/myapp.git",
			Branch:  "main",
			Cached:  true,
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	repo, err := c.GetRepository(context.Background(), "my-app")
	require.NoError(t, err)
	assert.Equal(t, "git", repo.ScmType)
	assert.Equal(t, "main", repo.Branch)
	assert.True(t, repo.Cached)
}

func TestCreateRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/repository", r.URL.Path)

		var body struct {
			Repository RepositoryCreateRequest `json:"repository"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "git", body.Repository.ScmType)
		assert.Equal(t, "git@github.com:myco/myapp.git", body.Repository.URL)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Repository{ScmType: "git", URL: "git@github.com:myco/myapp.git", Branch: "main"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	repo, err := c.CreateRepository(context.Background(), "my-app", RepositoryCreateRequest{
		ScmType: "git", URL: "git@github.com:myco/myapp.git",
	})
	require.NoError(t, err)
	assert.Equal(t, "git", repo.ScmType)
}

func TestListBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/repository/branches", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]string{
			"main":           "abc123",
			"develop":        "def456",
			"feature/new-ui": "ghi789",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	branches, err := c.ListBranches(context.Background(), "my-app")
	require.NoError(t, err)
	assert.Len(t, branches, 3)
	assert.Contains(t, branches, "main")
	assert.Equal(t, "abc123", branches["main"])
}

func TestGetLatestRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/repository/latest_revision", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]string{"ref": "abc123def456"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	rev, err := c.GetLatestRevision(context.Background(), "my-app")
	require.NoError(t, err)
	assert.Equal(t, "abc123def456", rev)
}

func TestListRecentCommits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/repository/recent_commits", r.URL.Path)
		json.NewEncoder(w).Encode(CommitsTagsReleases{
			Commits: []Commit{
				{Ref: "abc123", Author: "Jane", Message: "Fix bug"},
			},
			Tags:     []string{"v1.0.0"},
			Releases: []string{},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	result, err := c.ListRecentCommits(context.Background(), "my-app")
	require.NoError(t, err)
	assert.Len(t, result.Commits, 1)
	assert.Equal(t, "abc123", result.Commits[0].Ref)
	assert.Len(t, result.Tags, 1)
}
