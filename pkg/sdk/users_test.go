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

func boolPtr(b bool) *bool { return &b }

func TestListUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/users", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]User{
			{ID: 1, Identifier: "u-1", FirstName: "Ada", LastName: "Lovelace", EmailAddress: "ada@example.com", AccountAdministrator: true, Activated: true},
			{ID: 2, Identifier: "u-2", FirstName: "Alan", LastName: "Turing", EmailAddress: "alan@example.com", Activated: false},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	users, err := c.ListUsers(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "Ada", users[0].FirstName)
	assert.True(t, users[0].AccountAdministrator)
	assert.True(t, users[0].Activated)
	assert.False(t, users[1].Activated)
}

func TestGetUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/users/u-1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(User{ID: 1, Identifier: "u-1", FirstName: "Ada", EmailAddress: "ada@example.com", TimeZone: "UTC"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	user, err := c.GetUser(context.Background(), "u-1")
	require.NoError(t, err)
	assert.Equal(t, "u-1", user.Identifier)
	assert.Equal(t, "UTC", user.TimeZone)
}

func TestCreateUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/users", r.URL.Path)

		// Request body must be wrapped: {"user":{...}}.
		var body struct {
			User UserRequest `json:"user"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.NotNil(t, body.User.EmailAddress)
		assert.Equal(t, "new@example.com", *body.User.EmailAddress)
		require.NotNil(t, body.User.FirstName)
		assert.Equal(t, "New", *body.User.FirstName)
		require.NotNil(t, body.User.AccountAdministrator)
		assert.True(t, *body.User.AccountAdministrator)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(User{ID: 3, Identifier: "u-new", FirstName: "New", EmailAddress: "new@example.com", AccountAdministrator: true})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	user, err := c.CreateUser(context.Background(), UserRequest{
		FirstName:            strPtr("New"),
		EmailAddress:         strPtr("new@example.com"),
		AccountAdministrator: boolPtr(true),
	})
	require.NoError(t, err)
	assert.Equal(t, "u-new", user.Identifier)
	assert.True(t, user.AccountAdministrator)
}

func TestCreateUser_ValidationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string][]string{"email_address": {"has already been taken"}})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateUser(context.Background(), UserRequest{EmailAddress: strPtr("dup@example.com")})
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 422, apiErr.StatusCode)
	assert.True(t, apiErr.IsValidationError())
}

func TestUpdateUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/users/u-1", r.URL.Path)

		var body struct {
			User UserRequest `json:"user"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.NotNil(t, body.User.LastName)
		assert.Equal(t, "Byron", *body.User.LastName)
		// Unset fields must not be serialised.
		assert.Nil(t, body.User.EmailAddress)

		_ = json.NewEncoder(w).Encode(User{ID: 1, Identifier: "u-1", FirstName: "Ada", LastName: "Byron"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	user, err := c.UpdateUser(context.Background(), "u-1", UserRequest{LastName: strPtr("Byron")})
	require.NoError(t, err)
	assert.Equal(t, "Byron", user.LastName)
}

func TestDeleteUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/users/u-1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteUser(context.Background(), "u-1")
	require.NoError(t, err)
}

// Deleting the last account administrator is forbidden (403).
func TestDeleteUser_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "You cannot remove the last account administrator"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteUser(context.Background(), "u-admin")
	require.Error(t, err)
	assert.True(t, IsForbidden(err))
}

func TestGetUser_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.GetUser(context.Background(), "does-not-exist")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
}

func TestResendUserInvitation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/users/u-2/resend_invitation", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.ResendUserInvitation(context.Background(), "u-2")
	require.NoError(t, err)
}

// Resending to an already-activated user returns 422.
func TestResendUserInvitation_AlreadyActivated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "User already activated"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.ResendUserInvitation(context.Background(), "u-1")
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 422, apiErr.StatusCode)
}
