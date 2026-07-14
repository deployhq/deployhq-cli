package sdk

import (
	"context"
	"fmt"
)

// APIKey represents an API key. The plaintext key (Key) is only ever returned
// once, at creation time — it is not retrievable afterwards. The API keys the
// resource by its string identifier, which is what DeleteAPIKey expects.
type APIKey struct {
	Key         string `json:"api_key"` // plaintext, shown ONCE at creation
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	UserID      int    `json:"user_id"`
	Device      string `json:"device,omitempty"` // nullable
	ReadOnly    bool   `json:"read_only"`
}

// APIKeyCreateRequest is the payload for creating an API key. Description is
// required; ReadOnly is optional.
type APIKeyCreateRequest struct {
	Description string `json:"description"`
	ReadOnly    *bool  `json:"read_only,omitempty"`
}

// CreateAPIKey creates a new API key. The response includes the plaintext key
// in APIKey.Key — the only time it is ever exposed. Note the endpoint returns
// 200 (not 201) on success.
func (c *Client) CreateAPIKey(ctx context.Context, req APIKeyCreateRequest) (*APIKey, error) {
	body := struct {
		APIKey APIKeyCreateRequest `json:"api_key"`
	}{APIKey: req}
	var key APIKey
	if err := c.post(ctx, "/security/api_keys", body, &key); err != nil {
		return nil, err
	}
	return &key, nil
}

// DeleteAPIKey deletes an API key by its string identifier.
func (c *Client) DeleteAPIKey(ctx context.Context, identifier string) error {
	return c.delete(ctx, fmt.Sprintf("/security/api_keys/%s", identifier))
}
