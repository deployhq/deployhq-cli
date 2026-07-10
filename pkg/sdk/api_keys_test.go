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

func TestCreateAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/security/api_keys", r.URL.Path)

		// Request body must be wrapped: {"api_key":{...}}.
		var body struct {
			APIKey APIKeyCreateRequest `json:"api_key"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "CI token", body.APIKey.Description)
		require.NotNil(t, body.APIKey.ReadOnly)
		assert.True(t, *body.APIKey.ReadOnly)

		// The endpoint returns 200 (not 201). The server also returns an "html"
		// field which the SDK intentionally ignores.
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"api_key":     "plaintext-secret-key-value",
			"identifier":  "k-1",
			"description": "CI token",
			"user_id":     7,
			"device":      nil,
			"read_only":   true,
			"html":        "<div>ignored</div>",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	key, err := c.CreateAPIKey(context.Background(), APIKeyCreateRequest{
		Description: "CI token",
		ReadOnly:    boolPtr(true),
	})
	require.NoError(t, err)
	// The plaintext key is surfaced (shown once).
	assert.Equal(t, "plaintext-secret-key-value", key.Key)
	assert.Equal(t, "k-1", key.Identifier)
	assert.Equal(t, 7, key.UserID)
	assert.True(t, key.ReadOnly)
}

// ReadOnly is optional; when omitted it must not be serialised.
func TestCreateAPIKey_ReadOnlyOmitted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			APIKey APIKeyCreateRequest `json:"api_key"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Nil(t, body.APIKey.ReadOnly)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(APIKey{Key: "k", Identifier: "k-2", Description: "token"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateAPIKey(context.Background(), APIKeyCreateRequest{Description: "token"})
	require.NoError(t, err)
}

func TestCreateAPIKey_ValidationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string][]string{"description": {"can't be blank"}})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateAPIKey(context.Background(), APIKeyCreateRequest{})
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 422, apiErr.StatusCode)
	assert.True(t, apiErr.IsValidationError())
}

func TestDeleteAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/security/api_keys/k-1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteAPIKey(context.Background(), "k-1")
	require.NoError(t, err)
}

func TestDeleteAPIKey_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteAPIKey(context.Background(), "does-not-exist")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
}
