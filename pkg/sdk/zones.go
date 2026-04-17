package sdk

import "context"

// Zone represents an available deployment zone.
type Zone struct {
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
}

func (c *Client) ListZones(ctx context.Context, opts *ListOptions) ([]Zone, error) {
	var zones []Zone
	path := appendListParams("/zones", opts)
	if err := c.get(ctx, path, &zones); err != nil {
		return nil, err
	}
	return zones, nil
}
