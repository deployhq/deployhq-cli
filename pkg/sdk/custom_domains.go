package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CustomDomainDNSRecord is a DNS record the customer must create at their DNS
// provider for a custom domain to go live. Static Hosting returns two: a CNAME
// pointing the hostname at DeployHQ, and a TXT record proving ownership. The
// TXT record's Name and Value are null until DeployHQ has issued them.
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
	// SSLStatus is the certificate status (nullable → pointer).
	SSLStatus          *string                 `json:"ssl_status"`
	VerificationErrors []string                `json:"verification_errors"`
	ActivatedAt        *string                 `json:"activated_at"`
	CreatedAt          string                  `json:"created_at"`
	DNSRecords         []CustomDomainDNSRecord `json:"dns_records"`
}

// CustomDomainRemoval is the response to a custom-domain removal request. The
// domain is removed in the background, so Status is "removal_requested".
type CustomDomainRemoval struct {
	Status     string `json:"status"`
	Identifier string `json:"identifier"`
}

func customDomainsPath(projectID, serverID string) string {
	return fmt.Sprintf("/projects/%s/servers/%s/custom_domains", url.PathEscape(projectID), url.PathEscape(serverID))
}

// ListCustomDomains returns the custom domains of a Static Hosting server.
// Servers of any other protocol return a 404.
func (c *Client) ListCustomDomains(ctx context.Context, projectID, serverID string) ([]CustomDomain, error) {
	var domains []CustomDomain
	if err := c.get(ctx, customDomainsPath(projectID, serverID), &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// CreateCustomDomain adds a custom hostname to a Static Hosting server. The
// returned domain is pending until DeployHQ detects its DNS records. Custom
// domains need a paid plan: other accounts get a 403 with error_code
// "paid_plan_required" (see APIError.IsPaidPlanRequired).
func (c *Client) CreateCustomDomain(ctx context.Context, projectID, serverID, hostname string) (*CustomDomain, error) {
	body := map[string]interface{}{
		"custom_domain": map[string]string{"hostname": hostname},
	}
	var domain CustomDomain
	if err := c.post(ctx, customDomainsPath(projectID, serverID), body, &domain); err != nil {
		return nil, err
	}
	return &domain, nil
}

// DeleteCustomDomain requests removal of a custom domain. The API answers 202
// and removes the domain in the background.
func (c *Client) DeleteCustomDomain(ctx context.Context, projectID, serverID, identifier string) (*CustomDomainRemoval, error) {
	var removal CustomDomainRemoval
	path := customDomainsPath(projectID, serverID) + "/" + url.PathEscape(identifier)
	if err := c.do(ctx, http.MethodDelete, path, nil, &removal); err != nil {
		return nil, err
	}
	return &removal, nil
}
