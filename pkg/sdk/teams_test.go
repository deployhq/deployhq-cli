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

func TestListTeams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/teams", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]Team{
			{Identifier: "t1", Name: "Admins", IsAdmin: true, AllProjectsAllowed: true},
			{Identifier: "t2", Name: "Deployers"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	teams, err := c.ListTeams(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, teams, 2)
	assert.Equal(t, "Admins", teams[0].Name)
	assert.True(t, teams[0].IsAdmin)
	assert.Equal(t, "Deployers", teams[1].Name)
}

func TestGetTeam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/teams/t1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Team{
			Identifier:         "t1",
			Name:               "Admins",
			IsAdmin:            true,
			AllProjectsAllowed: true,
			Members: []TeamMember{
				{ID: 5, Identifier: "u5", FirstName: "Ada", LastName: "Lovelace", EmailAddress: "ada@example.com"},
				{ID: 6, Identifier: "u6", FirstName: "Alan", LastName: "Turing", EmailAddress: "alan@example.com"},
			},
			ProjectAssignments: []TeamProjectAssignment{
				{Name: "Web", Identifier: "web", CanDeployAll: true, CanUpdateConfig: true},
			},
			ProjectExclusions: []TeamProjectExclusion{
				{Name: "Secret", Identifier: "secret"},
			},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	team, err := c.GetTeam(context.Background(), "t1")
	require.NoError(t, err)
	assert.Equal(t, "Admins", team.Name)
	assert.Len(t, team.Members, 2)
	assert.Equal(t, "ada@example.com", team.Members[0].EmailAddress)
	assert.Equal(t, 5, team.Members[0].ID)
	assert.Len(t, team.ProjectAssignments, 1)
	assert.True(t, team.ProjectAssignments[0].CanDeployAll)
	require.Len(t, team.ProjectExclusions, 1)
	assert.Equal(t, "secret", team.ProjectExclusions[0].Identifier)
}

func TestGetTeam_NoExclusionsWhenNotAllProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// project_exclusions absent entirely when all_projects_allowed is false.
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"identifier":           "t2",
			"name":                 "Deployers",
			"all_projects_allowed": false,
			"project_assignments":  []interface{}{},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	team, err := c.GetTeam(context.Background(), "t2")
	require.NoError(t, err)
	assert.False(t, team.AllProjectsAllowed)
	assert.Empty(t, team.ProjectExclusions)
}

func TestCreateTeam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/teams", r.URL.Path)

		var body struct {
			Team TeamCreateRequest `json:"team"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Deployers", body.Team.Name)
		assert.True(t, body.Team.CanCreateProjects)
		assert.Equal(t, []int{5, 6}, body.Team.UserIDs)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Team{Identifier: "t-new", Name: "Deployers", CanCreateProjects: true})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	team, err := c.CreateTeam(context.Background(), TeamCreateRequest{
		Name:              "Deployers",
		CanCreateProjects: true,
		UserIDs:           []int{5, 6},
	})
	require.NoError(t, err)
	assert.Equal(t, "Deployers", team.Name)
	assert.Equal(t, "t-new", team.Identifier)
}

func TestUpdateTeam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Codebase convention: users/teams update via PATCH.
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/teams/t1", r.URL.Path)

		var raw map[string]map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		teamBody, ok := raw["team"]
		require.True(t, ok, "body must be wrapped under 'team'")
		assert.Equal(t, "Renamed", teamBody["name"])
		// Only 'name' was set; unset pointer fields must be omitted so a partial
		// update doesn't clobber the other flags.
		_, hasAdmin := teamBody["is_admin"]
		assert.False(t, hasAdmin, "unset is_admin must be omitted")

		_ = json.NewEncoder(w).Encode(Team{Identifier: "t1", Name: "Renamed"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	name := "Renamed"
	team, err := c.UpdateTeam(context.Background(), "t1", TeamUpdateRequest{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", team.Name)
}

func TestUpdateTeam_ClearMembers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		teamBody, ok := raw["team"]
		require.True(t, ok, "body must be wrapped under 'team'")
		// An explicit empty list must be sent as [] (clear all members), not
		// dropped by omitempty — otherwise "remove everyone" silently no-ops.
		ids, ok := teamBody["user_ids"]
		require.True(t, ok, "explicit empty user_ids must be present")
		assert.Equal(t, []interface{}{}, ids)

		_ = json.NewEncoder(w).Encode(Team{Identifier: "t1", Name: "Admins"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	empty := []int{}
	_, err := c.UpdateTeam(context.Background(), "t1", TeamUpdateRequest{UserIDs: &empty})
	require.NoError(t, err)
}

func TestUpdateTeam_OmittedMembersUntouched(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
		teamBody := raw["team"]
		// A nil UserIDs pointer (flag omitted) must NOT send user_ids at all.
		_, hasUserIDs := teamBody["user_ids"]
		assert.False(t, hasUserIDs, "omitted user_ids must be absent so membership is untouched")

		_ = json.NewEncoder(w).Encode(Team{Identifier: "t1", Name: "Admins"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	admin := true
	_, err := c.UpdateTeam(context.Background(), "t1", TeamUpdateRequest{IsAdmin: &admin})
	require.NoError(t, err)
}

func TestDeleteTeam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/teams/t1", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteTeam(context.Background(), "t1")
	require.NoError(t, err)
}

func TestCreateTeam_DuplicateName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string][]string{"name": {"can't be blank"}})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.CreateTeam(context.Background(), TeamCreateRequest{Name: ""})
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
}

func TestGetTeam_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":      "You do not have access...",
			"error_code": "access_denied",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.GetTeam(context.Background(), "t1")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestGetTeam_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.GetTeam(context.Background(), "nope")
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}
