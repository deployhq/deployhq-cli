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

func TestListDeploymentChecks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/deployment_checks", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]DeploymentCheck{
			{Identifier: "chk1", Name: "Lint", Stage: "pre_build", CheckType: "ssh", Position: 1},
			{Identifier: "chk2", Name: "Smoke", Stage: "post_deploy", CheckType: "http", Position: 1},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	checks, err := c.ListDeploymentChecks(context.Background(), "my-app", nil)
	require.NoError(t, err)
	assert.Len(t, checks, 2)
	assert.Equal(t, "Lint", checks[0].Name)
	assert.Equal(t, "http", checks[1].CheckType)
}

func TestGetDeploymentCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/deployment_checks/chk1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(DeploymentCheck{
			Identifier: "chk1", Name: "Smoke test", Stage: "post_deploy",
			CheckType: "http", HTTPMethod: "GET", HTTPURL: "https://example.com/health",
			HTTPExpectedStatus: 200,
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	check, err := c.GetDeploymentCheck(context.Background(), "my-app", "chk1")
	require.NoError(t, err)
	assert.Equal(t, "Smoke test", check.Name)
	assert.Equal(t, "https://example.com/health", check.HTTPURL)
	assert.Equal(t, 200, check.HTTPExpectedStatus)
}

func TestCreateDeploymentCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body struct {
			DeploymentCheck DeploymentCheckCreateRequest `json:"deployment_check"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Lint", body.DeploymentCheck.Name)
		assert.Equal(t, "pre_build", body.DeploymentCheck.Stage)
		assert.Equal(t, "ssh", body.DeploymentCheck.CheckType)
		assert.Equal(t, "bundle exec rubocop", body.DeploymentCheck.Command)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(DeploymentCheck{
			Identifier: "chk-new", Name: "Lint", Stage: "pre_build", CheckType: "ssh",
			Command: "bundle exec rubocop",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	check, err := c.CreateDeploymentCheck(context.Background(), "my-app", DeploymentCheckCreateRequest{
		Name:      "Lint",
		Stage:     "pre_build",
		CheckType: "ssh",
		Command:   "bundle exec rubocop",
	})
	require.NoError(t, err)
	assert.Equal(t, "chk-new", check.Identifier)
}

func TestUpdateDeploymentCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/projects/my-app/deployment_checks/chk1", r.URL.Path)

		var body struct {
			DeploymentCheck DeploymentCheckCreateRequest `json:"deployment_check"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.NotNil(t, body.DeploymentCheck.Enabled)
		assert.False(t, *body.DeploymentCheck.Enabled)

		_ = json.NewEncoder(w).Encode(DeploymentCheck{
			Identifier: "chk1", Name: "Lint", Enabled: false,
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	disabled := false
	check, err := c.UpdateDeploymentCheck(context.Background(), "my-app", "chk1", DeploymentCheckCreateRequest{
		Enabled: &disabled,
	})
	require.NoError(t, err)
	assert.False(t, check.Enabled)
}

func TestDeleteDeploymentCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/projects/my-app/deployment_checks/chk1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteDeploymentCheck(context.Background(), "my-app", "chk1")
	require.NoError(t, err)
}
