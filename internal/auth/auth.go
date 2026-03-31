// Package auth provides credential storage with keyring + file fallback.
//
// Credentials are stored as:
//   - Primary: OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service)
//   - Fallback: ~/.deployhq/credentials.json (mode 0600) when keyring is unavailable
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "deployhq-cli"
	keyringUser    = "default"
)

// Credentials holds the authentication data for an account.
type Credentials struct {
	Account string `json:"account"`
	Email   string `json:"email"`
	APIKey  string `json:"api_key"`
}

// Store saves credentials to the OS keyring, falling back to file.
func Store(creds *Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("auth: marshal credentials: %w", err)
	}

	if err := keyring.Set(keyringService, keyringUser, string(data)); err != nil {
		// Keyring unavailable, fall back to file
		return storeToFile(creds)
	}
	return nil
}

// Load retrieves credentials from the OS keyring, falling back to file.
func Load() (*Credentials, error) {
	data, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		var creds Credentials
		if err := json.Unmarshal([]byte(data), &creds); err != nil {
			return nil, fmt.Errorf("auth: unmarshal keyring data: %w", err)
		}
		return &creds, nil
	}

	// Fall back to file
	return loadFromFile()
}

// Delete removes stored credentials from both keyring and file.
func Delete() error {
	_ = keyring.Delete(keyringService, keyringUser) // ignore error
	_ = deleteFile()                                 // ignore error
	return nil
}

// HasCredentials returns true if credentials are stored.
func HasCredentials() bool {
	creds, err := Load()
	return err == nil && creds != nil && creds.APIKey != ""
}

func credentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".deployhq", "credentials.json")
}

func storeToFile(creds *Credentials) error {
	path := credentialsPath()
	if path == "" {
		return fmt.Errorf("auth: cannot determine home directory")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("auth: create dir: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("auth: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("auth: write credentials: %w", err)
	}
	return nil
}

func loadFromFile() (*Credentials, error) {
	path := credentialsPath()
	if path == "" {
		return nil, fmt.Errorf("auth: not logged in")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("auth: not logged in (run 'dhq auth login')")
		}
		return nil, fmt.Errorf("auth: read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("auth: parse credentials: %w", err)
	}
	return &creds, nil
}

func deleteFile() error {
	path := credentialsPath()
	if path == "" {
		return nil
	}
	return os.Remove(path)
}
