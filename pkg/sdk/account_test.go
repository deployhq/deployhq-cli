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

func TestGetAccount(t *testing.T) {
	limit := 25
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/account", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Account{
			Name: "Acme", Permalink: "acme", TimeZone: "UTC", Package: "standard",
			Trialling: false, Suspended: false, ProjectCount: 4, ProjectLimit: &limit,
			UsersAllowed: true, AIFeaturesDisabled: false,
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	account, err := c.GetAccount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Acme", account.Name)
	assert.Equal(t, "standard", account.Package)
	require.NotNil(t, account.ProjectLimit)
	assert.Equal(t, 25, *account.ProjectLimit)
}

// project_limit and package are nullable; they must round-trip as null/empty.
func TestGetAccount_NullableFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"Acme","permalink":"acme","time_zone":"UTC","package":null,"trialling":true,"suspended":false,"project_count":0,"project_limit":null,"users_allowed":true,"ai_features_disabled":false}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	account, err := c.GetAccount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "", account.Package)
	assert.Nil(t, account.ProjectLimit)
	assert.True(t, account.Trialling)
}

func TestUpdateAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/account", r.URL.Path)

		// Request body must be wrapped: {"account":{...}}.
		var body struct {
			Account AccountUpdateRequest `json:"account"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.NotNil(t, body.Account.Name)
		assert.Equal(t, "Renamed", *body.Account.Name)
		require.NotNil(t, body.Account.IPRestricted)
		assert.True(t, *body.Account.IPRestricted)
		// Unset fields must not be serialised.
		assert.Nil(t, body.Account.Permalink)

		_ = json.NewEncoder(w).Encode(Account{Name: "Renamed", Permalink: "acme", TimeZone: "UTC"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	account, err := c.UpdateAccount(context.Background(), AccountUpdateRequest{
		Name:         strPtr("Renamed"),
		IPRestricted: boolPtr(true),
	})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", account.Name)
}

// Non-administrators get a 403 on show/update.
func TestUpdateAccount_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.UpdateAccount(context.Background(), AccountUpdateRequest{Name: strPtr("X")})
	require.Error(t, err)
	assert.True(t, IsForbidden(err))
}

func TestGetBillingStatus_WithScheduledChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/account/billing_status", r.URL.Path)
		_ = json.NewEncoder(w).Encode(BillingStatus{
			Package: "standard", Frequency: 1, Trialling: false, Suspended: false,
			ScheduledChange: &ScheduledBillingChange{TargetPackage: "premium", TargetFrequency: 12, EffectiveAt: "2026-08-01"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	status, err := c.GetBillingStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "standard", status.Package)
	assert.Equal(t, 1, status.Frequency)
	require.NotNil(t, status.ScheduledChange)
	assert.Equal(t, "premium", status.ScheduledChange.TargetPackage)
	assert.Equal(t, 12, status.ScheduledChange.TargetFrequency)
}

// scheduled_change is omitted when no change is pending.
func TestGetBillingStatus_NoScheduledChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"package":"standard","frequency":1,"trialling":false,"suspended":false}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	status, err := c.GetBillingStatus(context.Background())
	require.NoError(t, err)
	assert.Nil(t, status.ScheduledChange)
}
