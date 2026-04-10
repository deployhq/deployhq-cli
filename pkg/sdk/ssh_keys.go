package sdk

import (
	"context"
	"fmt"
)

// SSHKey represents a global SSH key pair.
type SSHKey struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	PublicKey   string `json:"public_key"`
	KeyType     string `json:"key_type"`
	Fingerprint string `json:"fingerprint"`
	Account     string `json:"account"`
}

// SSHKeyCreateRequest is the payload for creating an SSH key.
type SSHKeyCreateRequest struct {
	Title   string `json:"title"`
	KeyType string `json:"key_type,omitempty"`
}

func (c *Client) ListSSHKeys(ctx context.Context, opts *ListOptions) ([]SSHKey, error) {
	var keys []SSHKey
	path := appendListParams("/ssh_keys", opts)
	if err := c.get(ctx, path, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func (c *Client) CreateSSHKey(ctx context.Context, req SSHKeyCreateRequest) (*SSHKey, error) {
	body := struct {
		KeyPair SSHKeyCreateRequest `json:"key_pair"`
	}{KeyPair: req}
	var key SSHKey
	if err := c.post(ctx, "/ssh_keys", body, &key); err != nil {
		return nil, err
	}
	return &key, nil
}

func (c *Client) DeleteSSHKey(ctx context.Context, keyID string) error {
	return c.delete(ctx, fmt.Sprintf("/ssh_keys/%s", keyID))
}
