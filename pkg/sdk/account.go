package sdk

import "context"

// Account represents the current DeployHQ account. It is a singular resource —
// there is no id in the path and no list endpoint.
type Account struct {
	Name               string `json:"name"`
	Permalink          string `json:"permalink"`
	TimeZone           string `json:"time_zone"`
	Package            string `json:"package,omitempty"` // nullable
	Trialling          bool   `json:"trialling"`
	Suspended          bool   `json:"suspended"`
	ProjectCount       int    `json:"project_count"`
	ProjectLimit       *int   `json:"project_limit"` // nullable — no limit when null
	UsersAllowed       bool   `json:"users_allowed"`
	AIFeaturesDisabled bool   `json:"ai_features_disabled"`
}

// AccountUpdateRequest is the payload for updating the account. Pointer fields
// are only serialised when set so a single attribute can change in isolation.
type AccountUpdateRequest struct {
	Name                   *string `json:"name,omitempty"`
	Permalink              *string `json:"permalink,omitempty"`
	TimeZone               *string `json:"time_zone,omitempty"`
	IPRestricted           *bool   `json:"ip_restricted,omitempty"`
	AIFeaturesDisabled     *bool   `json:"ai_features_disabled,omitempty"`
	TwoFactorAuthRequired  *bool   `json:"two_factor_auth_required,omitempty"`
	StrongPasswordRequired *bool   `json:"strong_password_required,omitempty"`
	CName                  *string `json:"cname,omitempty"`
}

// BillingStatus is the subscription/billing snapshot for the account.
type BillingStatus struct {
	Package         string                  `json:"package,omitempty"` // nullable
	Frequency       int                     `json:"frequency"`
	Trialling       bool                    `json:"trialling"`
	Suspended       bool                    `json:"suspended"`
	ScheduledChange *ScheduledBillingChange `json:"scheduled_change,omitempty"`
}

// ScheduledBillingChange describes a pending package/frequency change. It is
// omitted from the billing status entirely when no change is scheduled.
type ScheduledBillingChange struct {
	TargetPackage   string `json:"target_package"`
	TargetFrequency int    `json:"target_frequency"`
	EffectiveAt     string `json:"effective_at"`
}

func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	var account Account
	if err := c.get(ctx, "/account", &account); err != nil {
		return nil, err
	}
	return &account, nil
}

func (c *Client) UpdateAccount(ctx context.Context, req AccountUpdateRequest) (*Account, error) {
	body := struct {
		Account AccountUpdateRequest `json:"account"`
	}{Account: req}
	var account Account
	if err := c.patch(ctx, "/account", body, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

func (c *Client) GetBillingStatus(ctx context.Context) (*BillingStatus, error) {
	var status BillingStatus
	if err := c.get(ctx, "/account/billing_status", &status); err != nil {
		return nil, err
	}
	return &status, nil
}
