package sdk

import (
	"context"
	"fmt"
)

// Team is an account-level permission/role group. Members are users who
// inherit the team's permission flags. Teams are distinct from folders
// (which organise projects for display).
type Team struct {
	Identifier         string `json:"identifier"`
	Name               string `json:"name"`
	IsAdmin            bool   `json:"is_admin"`
	CanManageUsers     bool   `json:"can_manage_users"`
	CanManageBilling   bool   `json:"can_manage_billing"`
	CanManageAgents    bool   `json:"can_manage_agents"`
	CanCreateProjects  bool   `json:"can_create_projects"`
	AllProjectsAllowed bool   `json:"all_projects_allowed"`

	Members            []TeamMember            `json:"members,omitempty"`
	ProjectAssignments []TeamProjectAssignment `json:"project_assignments,omitempty"`
	// ProjectExclusions is only present when AllProjectsAllowed is true.
	ProjectExclusions []TeamProjectExclusion `json:"project_exclusions,omitempty"`
}

// TeamMember is a user belonging to a team. Unknown fields are ignored.
type TeamMember struct {
	ID                   int    `json:"id"`
	Identifier           string `json:"identifier"`
	FirstName            string `json:"first_name"`
	LastName             string `json:"last_name"`
	EmailAddress         string `json:"email_address"`
	TimeZone             string `json:"time_zone"`
	AccountAdministrator bool   `json:"account_administrator"`
	Activated            bool   `json:"activated"`
	IsAdmin              bool   `json:"is_admin"`
	CanManageUsers       bool   `json:"can_manage_users"`
	CanManageBilling     bool   `json:"can_manage_billing"`
	CanManageAgents      bool   `json:"can_manage_agents"`
	CanCreateProjects    bool   `json:"can_create_projects"`
	AllProjectsAllowed   bool   `json:"all_projects_allowed"`
}

// TeamProjectAssignment describes a project the team can access and the
// per-project capabilities granted.
type TeamProjectAssignment struct {
	Name                 string `json:"name"`
	Identifier           string `json:"identifier"`
	CanDeployAll         bool   `json:"can_deploy_all"`
	CanUpdateConfig      bool   `json:"can_update_config"`
	CanManageConfigFiles bool   `json:"can_manage_config_files"`
}

// TeamProjectExclusion describes a project explicitly excluded from an
// otherwise all-projects team.
type TeamProjectExclusion struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

// TeamCreateRequest is the payload for creating a team. Name is required.
//
// When IsAdmin is true the server force-sets the other permission flags and
// AllProjectsAllowed to true, regardless of what is sent.
//
// UserIDs syncs team membership. Omitting it leaves membership untouched.
type TeamCreateRequest struct {
	Name               string `json:"name"`
	IsAdmin            bool   `json:"is_admin"`
	CanManageUsers     bool   `json:"can_manage_users"`
	CanManageBilling   bool   `json:"can_manage_billing"`
	CanManageAgents    bool   `json:"can_manage_agents"`
	CanCreateProjects  bool   `json:"can_create_projects"`
	AllProjectsAllowed bool   `json:"all_projects_allowed"`
	UserIDs            []int  `json:"user_ids,omitempty"`
}

// TeamUpdateRequest is the payload for updating a team. All fields are pointers
// with omitempty so partial updates don't clobber unset fields.
//
// UserIDs syncs team membership. It is a pointer so the two intents stay
// distinct: a nil pointer leaves membership untouched, while a non-nil pointer
// to an empty slice clears all members — a plain []int with omitempty would
// drop the empty case and silently no-op. (The CLI reaches the empty case via
// --clear-members, since --user-ids "" cannot be parsed as an int slice.)
type TeamUpdateRequest struct {
	Name               *string `json:"name,omitempty"`
	IsAdmin            *bool   `json:"is_admin,omitempty"`
	CanManageUsers     *bool   `json:"can_manage_users,omitempty"`
	CanManageBilling   *bool   `json:"can_manage_billing,omitempty"`
	CanManageAgents    *bool   `json:"can_manage_agents,omitempty"`
	CanCreateProjects  *bool   `json:"can_create_projects,omitempty"`
	AllProjectsAllowed *bool   `json:"all_projects_allowed,omitempty"`
	UserIDs            *[]int  `json:"user_ids,omitempty"`
}

// ListTeams returns all teams on the account.
func (c *Client) ListTeams(ctx context.Context, opts *ListOptions) ([]Team, error) {
	var teams []Team
	path := appendListParams("/teams", opts)
	if err := c.get(ctx, path, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// GetTeam returns a single team by identifier.
func (c *Client) GetTeam(ctx context.Context, id string) (*Team, error) {
	var team Team
	if err := c.get(ctx, fmt.Sprintf("/teams/%s", id), &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// CreateTeam creates a new team.
func (c *Client) CreateTeam(ctx context.Context, req TeamCreateRequest) (*Team, error) {
	body := struct {
		Team TeamCreateRequest `json:"team"`
	}{Team: req}
	var team Team
	if err := c.post(ctx, "/teams", body, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// UpdateTeam updates an existing team.
func (c *Client) UpdateTeam(ctx context.Context, id string, req TeamUpdateRequest) (*Team, error) {
	body := struct {
		Team TeamUpdateRequest `json:"team"`
	}{Team: req}
	var team Team
	if err := c.patch(ctx, fmt.Sprintf("/teams/%s", id), body, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// DeleteTeam deletes a team by identifier.
func (c *Client) DeleteTeam(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/teams/%s", id))
}
