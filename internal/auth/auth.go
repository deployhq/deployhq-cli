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
	"strings"

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

	// Track this account in the profile index for enumeration
	addToProfileIndex(creds.Account)

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
// Since Store writes both a named profile and the default profile, logout
// must clean up both. We read before deleting to discover the paired profile,
// then aggressively clear all storage backends (keyring + file) for both.
func DeleteByAccount(account string) error {
	profile := profileName(account)

	// Discover the paired profile before we delete anything.
	// Bare logout (default profile) → find the named profile.
	// Named logout → find the default profile if it matches.
	var pairedProfiles []string

	if profile == defaultProfile {
		// Read the default to discover which named account it points to
		if creds, err := loadProfile(defaultProfile); err == nil && creds.Account != "" {
			named := profileName(creds.Account)
			if named != defaultProfile {
				pairedProfiles = append(pairedProfiles, named)
			}
		}
	} else {
		// Named logout — also remove default if it points to same account
		if creds, err := loadProfile(defaultProfile); err == nil && creds.Account == account {
			pairedProfiles = append(pairedProfiles, defaultProfile)
		}
	}

	// Delete the primary profile from ALL backends
	deleteAllBackends(profile)

	// Delete paired profiles from ALL backends
	for _, p := range pairedProfiles {
		deleteAllBackends(p)
	}

	// Clean up the profile index
	if account != "" {
		removeFromProfileIndex(account)
	} else {
		// Bare logout — remove whichever account the default pointed to
		for _, p := range pairedProfiles {
			removeFromProfileIndex(p)
		}
	}

	return nil
}

// deleteAllBackends removes a profile from both keyring and file storage.
// Errors are intentionally ignored — we want best-effort cleanup.
// On macOS Keychain, Delete can silently fail, so we overwrite with a
// tombstone value first, then delete.
func deleteAllBackends(profile string) {
	_ = keyring.Set(keyringService, profile, "{}")
	_ = keyring.Delete(keyringService, profile)
	_ = deleteFileProfile(profile)
}

// HasCredentials returns true if default credentials are stored.
func HasCredentials() bool {
	creds, err := Load()
	return err == nil && creds != nil && creds.APIKey != ""
}

// ListProfiles returns all stored credential profiles that are still valid.
// It uses a local index file (~/.deployhq/profiles) to track known accounts
// since the keyring API doesn't support enumeration.
func ListProfiles() []*Credentials {
	accounts := readProfileIndex()
	// Always check the default profile too
	accounts = append(accounts, "")

	seen := make(map[string]bool)
	var result []*Credentials
	for _, account := range accounts {
		creds, err := LoadByAccount(account)
		if err != nil || creds.APIKey == "" {
			continue
		}
		if seen[creds.Account] {
			continue
		}
		seen[creds.Account] = true
		result = append(result, creds)
	}
	return result
}

// DeleteAll removes all known credential profiles.
func DeleteAll() {
	for _, account := range readProfileIndex() {
		deleteAllBackends(profileName(account))
	}
	deleteAllBackends(defaultProfile)
	_ = os.Remove(profileIndexPath())
}

// profileIndexPath returns the path to the profile index file.
func profileIndexPath() string {
	dir := credentialsDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "profiles")
}

// addToProfileIndex records an account name in the index.
func addToProfileIndex(account string) {
	if account == "" {
		return
	}
	path := profileIndexPath()
	if path == "" {
		return
	}
	accounts := readProfileIndex()
	for _, a := range accounts {
		if a == account {
			return // already tracked
		}
	}
	accounts = append(accounts, account)

	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0700)
	_ = os.WriteFile(path, []byte(strings.Join(accounts, "\n")+"\n"), 0600)
}

// removeFromProfileIndex removes an account from the index.
func removeFromProfileIndex(account string) {
	if account == "" {
		return
	}
	path := profileIndexPath()
	if path == "" {
		return
	}
	accounts := readProfileIndex()
	var remaining []string
	for _, a := range accounts {
		if a != account {
			remaining = append(remaining, a)
		}
	}
	if len(remaining) == 0 {
		_ = os.Remove(path)
		return
	}
	_ = os.WriteFile(path, []byte(strings.Join(remaining, "\n")+"\n"), 0600)
}

// readProfileIndex reads known account names from the index file.
func readProfileIndex() []string {
	path := profileIndexPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var accounts []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			accounts = append(accounts, line)
		}
	}
	return accounts
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
		// Reject empty/tombstone entries (left over from failed deletes)
		if creds.APIKey == "" {
			return nil, fmt.Errorf("auth: empty credentials for profile %q", profile)
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
