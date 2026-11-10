package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CustomDomainDNSRecord is a DNS record the customer must create at their DNS
// provider for a custom domain to go live. Static Hosting returns a CNAME
// pointing the hostname at DeployHQ and a TXT record proving ownership. While
// the domain is Registering only the CNAME is returned; the TXT record's Name
// and Value can also be null until DeployHQ has issued them.
type CustomDomainDNSRecord struct {
	Type  string  `json:"type"`
	Name  *string `json:"name"`
	Value *string `json:"value"`
}

// CustomDomain is a custom hostname attached to a Static Hosting server.
type CustomDomain struct {
	Identifier string `json:"identifier"`
	Hostname   string `json:"hostname"`
	// Primary is true for the domain visitors land on; the other domain of an
	// apex + www pair redirects to it.
	Primary bool `json:"primary"`
	// Status is one of pending, active, failed or expired.
	Status string `json:"status"`
	// Registering is true while DeployHQ is still registering the domain, which
	// happens in the background after it is added: DNSRecords then holds only
	// the CNAME record. Servers that predate background registration omit the
	// field, which reads as false.
	Registering bool `json:"registering"`
	// SSLStatus is the certificate status (nullable → pointer).
	SSLStatus          *string                 `json:"ssl_status"`
	VerificationErrors []string                `json:"verification_errors"`
	ActivatedAt        *string                 `json:"activated_at"`
	CreatedAt          string                  `json:"created_at"`
	DNSRecords         []CustomDomainDNSRecord `json:"dns_records"`
}

// CustomDomainCreateRequest is a request to add a custom domain.
type CustomDomainCreateRequest struct {
	Hostname string `json:"hostname"`
	// WithCounterpart also adds the other half of the apex and www pair. The
	// outcome is in CustomDomainCreation.Counterpart and CounterpartErrors.
	WithCounterpart bool `json:"with_counterpart,omitempty"`
}

// CustomDomainCreation is the response to adding a custom domain: the domain
// itself plus, when requested, the other half of its apex and www pair.
type CustomDomainCreation struct {
	CustomDomain
	// Counterpart is nil when it was not requested, was not eligible or could
	// not be added; CounterpartErrors then says why, if it was refused.
	Counterpart       *CustomDomain `json:"counterpart"`
	CounterpartErrors []string      `json:"counterpart_errors"`
}

// CustomDomainRemoval is the response to a custom-domain removal request. The
// domain is removed in the background, so Status is "removal_requested".
type CustomDomainRemoval struct {
	Status     string `json:"status"`
	Identifier string `json:"identifier"`
}

// CustomDomainCheck is the response to asking for a pending custom domain to
// be checked now. The check runs in the background, so Status is
// "check_requested"; read the outcome from ListCustomDomains.
type CustomDomainCheck struct {
	Status     string `json:"status"`
	Identifier string `json:"identifier"`
}

// CustomDomainScope names the Static Hosting server whose custom domains a call
// acts on: a project's server or a global server. Build one with
// ProjectServerDomains or GlobalServerDomains.
type CustomDomainScope struct {
	path string
}

// ProjectServerDomains is the scope of a Static Hosting server in a project.
func ProjectServerDomains(projectID, serverID string) CustomDomainScope {
	return CustomDomainScope{path: fmt.Sprintf("/projects/%s/servers/%s/custom_domains", url.PathEscape(projectID), url.PathEscape(serverID))}
}

// GlobalServerDomains is the scope of a global Static Hosting server.
func GlobalServerDomains(serverID string) CustomDomainScope {
	return CustomDomainScope{path: fmt.Sprintf("/global_servers/%s/custom_domains", url.PathEscape(serverID))}
}

func (s CustomDomainScope) domainPath(identifier string) string {
	return s.path + "/" + url.PathEscape(identifier)
}

// ListCustomDomains returns the custom domains of a Static Hosting server.
// Servers of any other protocol return a 404.
func (c *Client) ListCustomDomains(ctx context.Context, scope CustomDomainScope) ([]CustomDomain, error) {
	var domains []CustomDomain
	if err := c.get(ctx, scope.path, &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// CreateCustomDomain adds a custom hostname to a Static Hosting server. The
// returned domain is pending until DeployHQ detects its DNS records, and
// Registering until its TXT record has been issued. Custom domains need a paid
// plan: other accounts get a 403 with error_code "paid_plan_required" (see
// APIError.IsPaidPlanRequired).
func (c *Client) CreateCustomDomain(ctx context.Context, scope CustomDomainScope, req CustomDomainCreateRequest) (*CustomDomainCreation, error) {
	body := map[string]interface{}{"custom_domain": req}
	var creation CustomDomainCreation
	if err := c.post(ctx, scope.path, body, &creation); err != nil {
		return nil, err
	}
	return &creation, nil
}

// DeleteCustomDomain requests removal of a custom domain. The API answers 202
// and removes the domain in the background.
func (c *Client) DeleteCustomDomain(ctx context.Context, scope CustomDomainScope, identifier string) (*CustomDomainRemoval, error) {
	var removal CustomDomainRemoval
	if err := c.do(ctx, http.MethodDelete, scope.domainPath(identifier), nil, &removal); err != nil {
		return nil, err
	}
	return &removal, nil
}

// RefreshCustomDomain asks for a pending custom domain to be checked now
// instead of on the next scheduled check. The API answers 202 and checks in the
// background. A domain that is not pending is a 422, and one checked within the
// last minute is a 429 (see APIError.RetryAfter).
func (c *Client) RefreshCustomDomain(ctx context.Context, scope CustomDomainScope, identifier string) (*CustomDomainCheck, error) {
	var check CustomDomainCheck
	if err := c.post(ctx, scope.domainPath(identifier)+"/refresh", nil, &check); err != nil {
		return nil, err
	}
	return &check, nil
}
