package sdk

import (
	"context"
	"fmt"
	"net/url"
)

// HostedResourceSSHKey is the SSH key pair attached to a Managed VPS hosted
// resource. It mirrors the account-level SSHKey shape.
type HostedResourceSSHKey struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	PublicKey   string `json:"public_key"`
	KeyType     string `json:"key_type"`
	Fingerprint string `json:"fingerprint"`
	Account     string `json:"account"`
}

// HostedResource is a managed hosting resource. The API returns two kinds of
// object in a single collection, discriminated by the Kind field:
//
//   - kind == "hosted_resource": a Managed VPS droplet (Region, Size, IPAddress,
//     SSHKey are populated).
//   - kind == "hosted_website":  a Static Hosting website (Subdomain, SPAMode,
//     StorageBytesTotal, ActiveDeploymentUUID, DeploymentCount are populated).
//
// Both shapes are modelled by this single struct; fields absent for a given
// kind are omitted from the JSON and decode to their zero values.
type HostedResource struct {
	// Kind discriminates the resource: "hosted_resource" or "hosted_website".
	Kind string `json:"kind"`
	// Identifier is the opaque string identifier used in the show/action routes.
	Identifier string `json:"identifier"`
	// Name is the human-readable resource name.
	Name string `json:"name"`
	// Status is the provisioning lifecycle status:
	// provisioning|active|suspended|error|destroying|disabled.
	Status string `json:"status"`
	// MonthlyCost is the monthly cost in the account's billing currency.
	MonthlyCost float64 `json:"monthly_cost"`

	// --- hosted_resource (Managed VPS) fields ---

	// Region is the DigitalOcean region slug (nullable → pointer).
	Region *string `json:"region,omitempty"`
	// Size is the droplet size slug (nullable → pointer).
	Size *string `json:"size,omitempty"`
	// IPAddress is the public IP once provisioned (nullable → pointer).
	IPAddress *string `json:"ip_address,omitempty"`
	// SSHKey is the attached SSH key pair (nil when none).
	SSHKey *HostedResourceSSHKey `json:"ssh_key,omitempty"`

	// --- hosted_website (Static Hosting) fields ---

	// Subdomain is the deployhq-sites subdomain for a hosted_website.
	Subdomain string `json:"subdomain,omitempty"`
	// SPAMode indicates single-page-app fallback routing for a hosted_website.
	SPAMode bool `json:"spa_mode,omitempty"`
	// StorageBytesTotal is the total stored bytes for a hosted_website.
	StorageBytesTotal int `json:"storage_bytes_total,omitempty"`
	// ActiveDeploymentUUID is the UUID of the live deployment (nullable → pointer).
	ActiveDeploymentUUID *string `json:"active_deployment_uuid,omitempty"`
	// DeploymentCount is the number of deployments for a hosted_website.
	DeploymentCount int `json:"deployment_count,omitempty"`

	// --- common timestamps ---

	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ListHostedResources returns all managed hosting resources for the account —
// both Managed VPS resources (kind "hosted_resource") and Static Hosting
// websites (kind "hosted_website"). The endpoint is gated by admin +
// beta_features; callers without both receive a 403.
func (c *Client) ListHostedResources(ctx context.Context, opts *ListOptions) ([]HostedResource, error) {
	var resources []HostedResource
	path := appendListParams("/hosted_resources", opts)
	if err := c.get(ctx, path, &resources); err != nil {
		return nil, err
	}
	return resources, nil
}

// GetHostedResource returns a single hosted resource by its string identifier.
func (c *Client) GetHostedResource(ctx context.Context, id string) (*HostedResource, error) {
	var resource HostedResource
	if err := c.get(ctx, fmt.Sprintf("/hosted_resources/%s", url.PathEscape(id)), &resource); err != nil {
		return nil, err
	}
	return &resource, nil
}

// SyncHostedResource requests a re-sync of the hosted resource's provisioning
// state from the upstream provider. POSTs with no request body; the 200 response
// carries {"status":"sync_requested"} which is not surfaced.
func (c *Client) SyncHostedResource(ctx context.Context, id string) error {
	return c.post(ctx, fmt.Sprintf("/hosted_resources/%s/sync", url.PathEscape(id)), nil, nil)
}

// RetryProvisionHostedResource retries provisioning of a resource that is in
// the "error" state. POSTs with no request body; the 200 response carries
// {"status":"provisioning"}. A resource not in the error state returns a 422.
func (c *Client) RetryProvisionHostedResource(ctx context.Context, id string) error {
	return c.post(ctx, fmt.Sprintf("/hosted_resources/%s/retry_provision", url.PathEscape(id)), nil, nil)
}
