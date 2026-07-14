package sdk

import (
	"context"
	"fmt"
)

// User represents an account user (team member). The API keys users by their
// string identifier, never an integer id.
type User struct {
	ID                   int    `json:"id"`
	Identifier           string `json:"identifier"`
	FirstName            string `json:"first_name"`
	LastName             string `json:"last_name"`
	EmailAddress         string `json:"email_address"`
	TimeZone             string `json:"time_zone"`
	AccountAdministrator bool   `json:"account_administrator"`
	Activated            bool   `json:"activated"`
	CanManageUsers       bool   `json:"can_manage_users"`
	CanManageBilling     bool   `json:"can_manage_billing"`
	CanCreateProjects    bool   `json:"can_create_projects"`
	CanManageAgents      bool   `json:"can_manage_agents"`
	AllProjectsAllowed   bool   `json:"all_projects_allowed"`
}

// UserRequest is the payload for creating or updating a user.
//
// Pointer fields are only serialised when set, so an update can change a single
// attribute without clobbering the rest. The backend strips the admin-only
// fields (account_administrator and the can_* capabilities) for callers who are
// not themselves account administrators.
type UserRequest struct {
	FirstName            *string `json:"first_name,omitempty"`
	LastName             *string `json:"last_name,omitempty"`
	EmailAddress         *string `json:"email_address,omitempty"`
	TimeZone             *string `json:"time_zone,omitempty"`
	AllProjectsAllowed   *bool   `json:"all_projects_allowed,omitempty"`
	AccountAdministrator *bool   `json:"account_administrator,omitempty"`
	CanManageUsers       *bool   `json:"can_manage_users,omitempty"`
	CanManageBilling     *bool   `json:"can_manage_billing,omitempty"`
	CanManageAgents      *bool   `json:"can_manage_agents,omitempty"`
	CanCreateProjects    *bool   `json:"can_create_projects,omitempty"`
}

func (c *Client) ListUsers(ctx context.Context, opts *ListOptions) ([]User, error) {
	var users []User
	path := appendListParams("/users", opts)
	if err := c.get(ctx, path, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) GetUser(ctx context.Context, identifier string) (*User, error) {
	var user User
	if err := c.get(ctx, fmt.Sprintf("/users/%s", identifier), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) CreateUser(ctx context.Context, req UserRequest) (*User, error) {
	body := struct {
		User UserRequest `json:"user"`
	}{User: req}
	var user User
	if err := c.post(ctx, "/users", body, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) UpdateUser(ctx context.Context, identifier string, req UserRequest) (*User, error) {
	body := struct {
		User UserRequest `json:"user"`
	}{User: req}
	var user User
	if err := c.patch(ctx, fmt.Sprintf("/users/%s", identifier), body, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) DeleteUser(ctx context.Context, identifier string) error {
	return c.delete(ctx, fmt.Sprintf("/users/%s", identifier))
}

// ResendUserInvitation re-sends the activation invitation email to a user who
// has not yet activated their account. The backend returns 422 with
// {"error":"User already activated"} if the user is already active.
func (c *Client) ResendUserInvitation(ctx context.Context, identifier string) error {
	return c.post(ctx, fmt.Sprintf("/users/%s/resend_invitation", identifier), nil, nil)
}
