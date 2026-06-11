package sdk

import (
	"context"
	"fmt"
	"strings"
)

// GetAccountCapabilities returns the beta/eligibility status for the current account.
// The capability flags live on the account sub-object of GET /profile, which —
// unlike GET /account — is readable by ANY authenticated account member (not admin-gated).
//
// Returns AccountCapabilities with beta_features, static_hosting_eligible, and
// managed_vps_eligible populated; the other profile/account fields are ignored.
//
// On an older backend without the capability fields they default to false, and the
// caller should direct the user to /beta_features — handling staging/version mismatches.
func (c *Client) GetAccountCapabilities(ctx context.Context) (*AccountCapabilities, error) {
	// Decode only the account capability fields from the profile payload.
	var profile struct {
		Account AccountCapabilities `json:"account"`
	}
	if err := c.get(ctx, "/profile", &profile); err != nil {
		return nil, err
	}
	return &profile.Account, nil
}

// EnrollBeta enrolls the current account in the managed-resources beta.
// This calls POST /beta/enrollments with the given protocol ("static_hosting",
// "managed_vps", or "" to enroll in all managed-resources protocols).
//
// Authorization:
//   - Admin users: can flip beta from false to true.
//   - Non-admin users who are already enrolled: pass idempotently.
//   - Non-admin users who are not yet enrolled: receive a 403 with a structured
//     error body. The caller should surface an actionable message with the
//     /beta_features deep-link rather than a raw 403.
//
// Returns ErrBetaEnrollAdminRequired (a *APIError with StatusCode 403) when
// the account is not yet enrolled and the current user is not an admin.
func (c *Client) EnrollBeta(ctx context.Context, protocol string) (*BetaEnrollmentResponse, error) {
	req := BetaEnrollmentRequest{Protocol: protocol}
	var resp BetaEnrollmentResponse
	if err := c.post(ctx, "/beta/enrollments", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListManagedHostingRegions returns the DigitalOcean regions available for
// Managed VPS provisioning. This endpoint requires beta_features to be enabled
// on the account (require_beta_features gate on the backend).
//
// Use the returned Region.Slug as ServerCreateRequest.Region.
func (c *Client) ListManagedHostingRegions(ctx context.Context) ([]ManagedHostingRegion, error) {
	var regions []ManagedHostingRegion
	if err := c.get(ctx, "/managed_hosting/regions", &regions); err != nil {
		return nil, err
	}
	return regions, nil
}

// ListManagedHostingSizes returns the DigitalOcean droplet sizes available for
// Managed VPS provisioning. Prices are denominated in the account's billing
// currency. This endpoint requires beta_features to be enabled on the account.
//
// Use the returned Size.Slug as ServerCreateRequest.Size.
func (c *Client) ListManagedHostingSizes(ctx context.Context) ([]ManagedHostingSize, error) {
	var sizes []ManagedHostingSize
	if err := c.get(ctx, "/managed_hosting/sizes", &sizes); err != nil {
		return nil, err
	}
	return sizes, nil
}

// GetServerProvisioningState returns the current server record from the
// project-scoped server show endpoint (GET /projects/:id/servers/:id).
//
// This is the correct polling target for Managed VPS and Static Hosting servers
// during provisioning. The endpoint is gated by project config permission (not
// admin), which is the same access level required to create the server.
//
// When the server is a managed_vps or static_hosting, the returned Server carries
// a protocol-specific block (ManagedVPS or StaticHosting) with the provisioning
// status and, once active, the IP / live URL. Use ProvisioningStatus(server),
// IsProvisioning(server), IsProvisioningActive(server) and LiveURL(server) to read them.
//
// For non-managed servers those blocks are nil.
func (c *Client) GetServerProvisioningState(ctx context.Context, projectID, serverID string) (*Server, error) {
	var server Server
	if err := c.get(ctx, fmt.Sprintf("/projects/%s/servers/%s", projectID, serverID), &server); err != nil {
		return nil, err
	}
	return &server, nil
}

// LiveURL extracts the publicly accessible URL from a provisioned server.
// Returns "" when the server is not yet active or is not a managed type.
//
// For static_hosting: returns server.StaticHosting.URL when non-empty.
// For managed_vps: returns "http://<ip_address>" when IPAddress is set.
// For all other protocol types: returns "".
func LiveURL(server *Server) string {
	if server == nil {
		return ""
	}
	switch server.ProtocolType {
	case "static_hosting":
		if server.StaticHosting != nil && server.StaticHosting.URL != "" {
			return server.StaticHosting.URL
		}
	case "managed_vps":
		if server.ManagedVPS != nil && server.ManagedVPS.IPAddress != "" {
			return "http://" + server.ManagedVPS.IPAddress
		}
	}
	return ""
}

// ProvisioningStatus returns the managed-resource provisioning lifecycle status
// ("provisioning" / "active" / "error") for a managed_vps or static_hosting server,
// reading whichever protocol-specific block the backend populated. Returns "" for
// non-managed servers or when the block is absent.
func ProvisioningStatus(server *Server) string {
	if server == nil {
		return ""
	}
	switch server.ProtocolType {
	case "static_hosting":
		if server.StaticHosting != nil {
			return server.StaticHosting.Status
		}
	case "managed_vps":
		if server.ManagedVPS != nil {
			return server.ManagedVPS.Status
		}
	}
	return ""
}

// IsProvisioning returns true when the server is a managed type whose
// backend resource has not yet finished provisioning.
func IsProvisioning(server *Server) bool {
	return strings.EqualFold(ProvisioningStatus(server), "provisioning")
}

// IsProvisioningActive returns true when the managed server has successfully
// completed provisioning and is ready for deployments.
func IsProvisioningActive(server *Server) bool {
	return strings.EqualFold(ProvisioningStatus(server), "active")
}
