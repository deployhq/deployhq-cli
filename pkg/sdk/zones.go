package sdk

import "context"

// Zone represents an available deployment zone.
type Zone struct {
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
}

func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var zones []Zone
	if err := c.get(ctx, "/zones", &zones); err != nil {
		return nil, err
	}
	return zones, nil
}
