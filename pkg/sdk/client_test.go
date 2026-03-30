package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a Client that points at the given test server.
func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c, err := New("test", "user@example.com", "test-key")
	require.NoError(t, err)
	c.baseURL = mustParseURL(t, server.URL)
	c.httpClient = server.Client()
	return c
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestNew(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c, err := New("myco", "user@test.com", "key123")
		require.NoError(t, err)
		assert.Equal(t, "https://myco.deployhq.com", c.baseURL.String())
		assert.Equal(t, "user@test.com", c.email)
	})

	t.Run("missing account", func(t *testing.T) {
		_, err := New("", "user@test.com", "key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "account is required")
	})

	t.Run("missing email", func(t *testing.T) {
		_, err := New("myco", "", "key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email is required")
	})

	t.Run("missing api key", func(t *testing.T) {
		_, err := New("myco", "user@test.com", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api key is required")
	})
}

func TestClient_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "basic auth should be present")
		assert.Equal(t, "user@example.com", user)
		assert.Equal(t, "test-key", pass)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Project{})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.ListProjects(context.Background())
	require.NoError(t, err)
}

func TestClient_APIError_SingleError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Project not found"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.GetProject(context.Background(), "nonexistent")
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 404, apiErr.StatusCode)
	assert.Equal(t, "Project not found", apiErr.Message)
	assert.True(t, apiErr.IsNotFound())
	assert.True(t, IsNotFound(err))
}

func TestClient_APIError_MultipleErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string][]string{"errors": {"name is required", "permalink is taken"}})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateProject(context.Background(), ProjectCreateRequest{})
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 422, apiErr.StatusCode)
	assert.Len(t, apiErr.Errors, 2)
	assert.True(t, apiErr.IsValidationError())
}

func TestClient_APIError_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.ListProjects(context.Background())
	require.Error(t, err)
	assert.True(t, IsUnauthorized(err))
}

func TestClient_UserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("User-Agent"), "my-agent")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]Project{})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	c.userAgent = "my-agent"
	_, err := c.ListProjects(context.Background())
	require.NoError(t, err)
}

func TestClient_Do_EscapeHatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/custom/endpoint", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "ok"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	var result map[string]string
	err := c.Do(context.Background(), http.MethodPost, "/custom/endpoint", map[string]string{"key": "val"}, &result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result["result"])
}
