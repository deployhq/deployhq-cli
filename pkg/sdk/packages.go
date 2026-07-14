package sdk

import "context"

// Package is a DeployHQ subscription plan (pricing tier) as served by the public
// GET /packages endpoint. Pricing is localised to the caller's currency based on
// the request's originating IP, so Currency and the prices vary by location.
type Package struct {
	// Permalink is the stable plan identifier (e.g. "small", "medium").
	Permalink string `json:"permalink"`
	// Name is the human-readable plan name.
	Name string `json:"name"`
	// Currency is the ISO currency code the prices are quoted in.
	Currency string `json:"currency"`
	// Price is the monthly price.
	Price float64 `json:"price"`
	// PriceBilledAnnually is the effective monthly price when billed annually.
	PriceBilledAnnually float64 `json:"price_billed_annually"`
	// Features lists the plan's headline features.
	Features []string `json:"features"`
}

// ListPackages returns DeployHQ's available subscription plans with pricing
// localised to the caller's IP. The endpoint is public (no authentication).
// A 503 with an {"error": ...} body indicates pricing is temporarily
// unavailable; it is surfaced as an *APIError.
func (c *Client) ListPackages(ctx context.Context) ([]Package, error) {
	var resp []Package
	if err := c.get(ctx, "/packages", &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
