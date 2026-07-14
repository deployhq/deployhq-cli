package sdk

import "context"

// Profile represents the currently authenticated user's own profile. It carries
// the same user fields as User plus a nested account summary. PATCH /profile
// returns only {"status":"ok"}, so UpdateProfile does not return the object.
type Profile struct {
	ID                   int            `json:"id"`
	Identifier           string         `json:"identifier"`
	FirstName            string         `json:"first_name"`
	LastName             string         `json:"last_name"`
	EmailAddress         string         `json:"email_address"`
	TimeZone             string         `json:"time_zone"`
	AccountAdministrator bool           `json:"account_administrator"`
	Activated            bool           `json:"activated"`
	CanManageUsers       bool           `json:"can_manage_users"`
	CanManageBilling     bool           `json:"can_manage_billing"`
	CanCreateProjects    bool           `json:"can_create_projects"`
	CanManageAgents      bool           `json:"can_manage_agents"`
	AllProjectsAllowed   bool           `json:"all_projects_allowed"`
	Account              ProfileAccount `json:"account"`
}

// ProfileAccount is the account summary nested in the profile response. It
// carries the base account fields plus eligibility flags not present on the
// plain /account resource.
type ProfileAccount struct {
	Name                  string `json:"name"`
	Permalink             string `json:"permalink"`
	TimeZone              string `json:"time_zone"`
	Package               string `json:"package,omitempty"` // nullable
	Trialling             bool   `json:"trialling"`
	Suspended             bool   `json:"suspended"`
	ProjectCount          int    `json:"project_count"`
	ProjectLimit          *int   `json:"project_limit"` // nullable
	UsersAllowed          bool   `json:"users_allowed"`
	AIFeaturesDisabled    bool   `json:"ai_features_disabled"`
	BetaFeatures          bool   `json:"beta_features"`
	StaticHostingEligible bool   `json:"static_hosting_eligible"`
	ManagedVPSEligible    bool   `json:"managed_vps_eligible"`
}

// ProfileUpdateRequest is the payload for updating the current user's profile.
// Pointer fields are only serialised when set.
type ProfileUpdateRequest struct {
	FirstName         *string `json:"first_name,omitempty"`
	LastName          *string `json:"last_name,omitempty"`
	EmailAddress      *string `json:"email_address,omitempty"`
	TimeZone          *string `json:"time_zone,omitempty"`
	AlphabeticSort    *bool   `json:"alphabetic_sort,omitempty"`
	PromotionsEnabled *bool   `json:"promotions_enabled,omitempty"`
}

func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	var profile Profile
	if err := c.get(ctx, "/profile", &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// UpdateProfile updates the current user's profile. The endpoint returns only a
// status acknowledgement, not the updated object, so there is no return value.
func (c *Client) UpdateProfile(ctx context.Context, req ProfileUpdateRequest) error {
	body := struct {
		User ProfileUpdateRequest `json:"user"`
	}{User: req}
	return c.patch(ctx, "/profile", body, nil)
}
