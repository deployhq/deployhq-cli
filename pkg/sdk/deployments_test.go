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

func TestListDeployments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/deployments", r.URL.Path)
		_ = json.NewEncoder(w).Encode(PaginatedResponse[Deployment]{
			Pagination: Pagination{CurrentPage: 1, TotalPages: 1, TotalRecords: 2, Offset: 0},
			Records: []Deployment{
				{Identifier: "dep1", Status: "completed", Branch: "main"},
				{Identifier: "dep2", Status: "running", Branch: "develop"},
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	result, err := c.ListDeployments(context.Background(), "my-app", nil)
	require.NoError(t, err)
	assert.Len(t, result.Records, 2)
	assert.Equal(t, 1, result.Pagination.CurrentPage)
	assert.Equal(t, "completed", result.Records[0].Status)
}

func TestGetDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/deployments/dep1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Deployment{
			Identifier: "dep1",
			Status:     "completed",
			Branch:     "main",
			Timestamps: &Timestamps{
				QueuedAt:    "2026-01-01T00:00:00Z",
				CompletedAt: strPtr("2026-01-01T00:05:00Z"),
				Duration:    flexPtr("300"),
			},
			Steps: []DeploymentStep{
				{Step: "transfer", Stage: "deploy", Identifier: "step1", Status: "completed", Description: "Transferring files"},
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	dep, err := c.GetDeployment(context.Background(), "my-app", "dep1")
	require.NoError(t, err)
	assert.Equal(t, "completed", dep.Status)
	assert.NotNil(t, dep.Timestamps)
	assert.Len(t, dep.Steps, 1)
}

func TestCreateDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body struct {
			Deployment DeploymentCreateRequest `json:"deployment"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "main", body.Deployment.Branch)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Deployment{Identifier: "dep-new", Status: "queued", Branch: "main"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	dep, err := c.CreateDeployment(context.Background(), "my-app", DeploymentCreateRequest{
		Branch: "main",
	})
	require.NoError(t, err)
	assert.Equal(t, "queued", dep.Status)
}

func TestAbortDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/deployments/dep1/abort", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.AbortDeployment(context.Background(), "my-app", "dep1")
	require.NoError(t, err)
}

func TestRollbackDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/deployments/dep1/rollback", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Deployment{Identifier: "dep-rollback", Status: "queued"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	dep, err := c.RollbackDeployment(context.Background(), "my-app", "dep1")
	require.NoError(t, err)
	assert.Equal(t, "dep-rollback", dep.Identifier)
}

func TestGetDeploymentStepLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/deployments/dep1/steps/step1/logs", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]DeploymentStepLog{
			{ID: "log1", Step: "transfer", Message: "Uploading file.txt"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	logs, err := c.GetDeploymentStepLogs(context.Background(), "my-app", "dep1", "step1")
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "Uploading file.txt", logs[0].Message)
}

func strPtr(s string) *string         { return &s }
func flexPtr(s string) *FlexString { v := FlexString(s); return &v }
