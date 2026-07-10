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

func TestGetProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/profile", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Profile{
			ID: 1, Identifier: "u-1", FirstName: "Ada", LastName: "Lovelace",
			EmailAddress: "ada@example.com", TimeZone: "UTC", AccountAdministrator: true,
			Account: ProfileAccount{
				Name: "Acme", Permalink: "acme", TimeZone: "UTC", Package: "standard",
				BetaFeatures: true, StaticHostingEligible: true, ManagedVPSEligible: false,
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	profile, err := c.GetProfile(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "u-1", profile.Identifier)
	assert.Equal(t, "Ada", profile.FirstName)
	// Nested account with the eligibility extras.
	assert.Equal(t, "Acme", profile.Account.Name)
	assert.True(t, profile.Account.BetaFeatures)
	assert.True(t, profile.Account.StaticHostingEligible)
	assert.False(t, profile.Account.ManagedVPSEligible)
}

func TestUpdateProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/profile", r.URL.Path)

		// Request body must be wrapped: {"user":{...}}.
		var body struct {
			User ProfileUpdateRequest `json:"user"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.NotNil(t, body.User.FirstName)
		assert.Equal(t, "Augusta", *body.User.FirstName)
		require.NotNil(t, body.User.AlphabeticSort)
		assert.True(t, *body.User.AlphabeticSort)
		// Unset fields must not be serialised.
		assert.Nil(t, body.User.LastName)

		// PATCH /profile returns only a status acknowledgement.
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.UpdateProfile(context.Background(), ProfileUpdateRequest{
		FirstName:      strPtr("Augusta"),
		AlphabeticSort: boolPtr(true),
	})
	require.NoError(t, err)
}

func TestUpdateProfile_ValidationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string][]string{"email_address": {"is invalid"}})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.UpdateProfile(context.Background(), ProfileUpdateRequest{EmailAddress: strPtr("not-an-email")})
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, 422, apiErr.StatusCode)
	assert.True(t, apiErr.IsValidationError())
}
