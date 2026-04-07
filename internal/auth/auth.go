// Package auth provides credential storage with keyring + file fallback.
//
// Credentials are stored per account profile:
//   - Primary: OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service)
//   - Fallback: ~/.deployhq/credentials.json (mode 0600) when keyring is unavailable
//
// Profile resolution:
//   - Named profile: keyring user = account name (e.g. "sg", "sg.staging")
//   - Default profile: keyring user = "default" (used when no account is specified)
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
	defaultProfile = "default"
)

// Credentials holds the authentication data for an account.
type Credentials struct {
	Account string `json:"account"`
	Email   string `json:"email"`
	APIKey  string `json:"api_key"`
}

// Store saves credentials to the OS keyring, falling back to file.
// If creds.Account is set, it is stored under both the named profile
// and the default profile so that commands without --account find
// the most recently logged-in credentials.
func Store(creds *Credentials) error {
	profile := profileName(creds.Account)

	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("auth: marshal credentials: %w", err)
	}

	if err := keyring.Set(keyringService, profile, string(data)); err != nil {
		// Keyring unavailable, fall back to file
		if err := storeToFile(profile, creds); err != nil {
			return err
		}
	}

	// Also store as the default profile so bare "dhq auth status" works.
	if profile != defaultProfile {
		if err := keyring.Set(keyringService, defaultProfile, string(data)); err != nil {
			return storeToFile(defaultProfile, creds)
		}
	}
	return nil
}

// Load retrieves the default profile credentials.
func Load() (*Credentials, error) {
	return LoadByAccount("")
}

// LoadByAccount retrieves credentials for a specific account profile.
// If account is empty, loads the default profile.
// If no named profile is found, falls back to the default profile.
func LoadByAccount(account string) (*Credentials, error) {
	profile := profileName(account)

	// Try the named profile first
	creds, err := loadProfile(profile)
	if err == nil {
		return creds, nil
	}

	// If we asked for a named profile and it wasn't found,
	// try the default profile (it may match the requested account)
	if profile != defaultProfile {
		creds, err = loadProfile(defaultProfile)
		if err == nil && (account == "" || creds.Account == account) {
			return creds, nil
		}
	}

	return nil, fmt.Errorf("auth: not logged in (run 'dhq auth login')")
}

// Delete removes stored credentials for the default profile.
func Delete() error {
	return DeleteByAccount("")
}

// DeleteByAccount removes stored credentials for a specific account profile.
// When deleting the default profile, also removes the named profile it points
// to (since Store writes both). When deleting a named profile, also cleans up
// the default profile if it points to the same account.
func DeleteByAccount(account string) error {
	profile := profileName(account)

	if profile == defaultProfile {
		// Bare "logout" — read the default profile first to find the named profile
		if creds, err := loadProfile(defaultProfile); err == nil && creds.Account != "" {
			named := profileName(creds.Account)
			if named != defaultProfile {
				_ = keyring.Delete(keyringService, named)
				_ = deleteFileProfile(named)
			}
		}
	}

	_ = keyring.Delete(keyringService, profile)
	_ = deleteFileProfile(profile)

	// Named logout — also clean up default if it points to the same account
	if profile != defaultProfile && account != "" {
		if creds, err := loadProfile(defaultProfile); err == nil && creds.Account == account {
			_ = keyring.Delete(keyringService, defaultProfile)
			_ = deleteFileProfile(defaultProfile)
		}
	}
	return nil
}

// HasCredentials returns true if default credentials are stored.
func HasCredentials() bool {
	creds, err := Load()
	return err == nil && creds != nil && creds.APIKey != ""
}

func profileName(account string) string {
	if account == "" {
		return defaultProfile
	}
	return account
}

func loadProfile(profile string) (*Credentials, error) {
	// Try keyring
	data, err := keyring.Get(keyringService, profile)
	if err == nil {
		var creds Credentials
		if err := json.Unmarshal([]byte(data), &creds); err != nil {
			return nil, fmt.Errorf("auth: unmarshal keyring data: %w", err)
		}
		return &creds, nil
	}

	// Fall back to file
	return loadFromFile(profile)
}

func credentialsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".deployhq")
}

func credentialsPath(profile string) string {
	dir := credentialsDir()
	if dir == "" {
		return ""
	}
	if profile == defaultProfile {
		return filepath.Join(dir, "credentials.json")
	}
	return filepath.Join(dir, fmt.Sprintf("credentials-%s.json", profile))
}

func storeToFile(profile string, creds *Credentials) error {
	path := credentialsPath(profile)
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

func loadFromFile(profile string) (*Credentials, error) {
	path := credentialsPath(profile)
	if path == "" {
		return nil, fmt.Errorf("auth: not logged in")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("auth: no credentials for profile %q", profile)
		}
		return nil, fmt.Errorf("auth: read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("auth: parse credentials: %w", err)
	}
	return &creds, nil
}

func deleteFileProfile(profile string) error {
	path := credentialsPath(profile)
	if path == "" {
		return nil
	}
	return os.Remove(path)
}
