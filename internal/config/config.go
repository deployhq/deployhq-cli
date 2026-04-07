// Package config provides 4-layer configuration loading.
//
// Layer precedence (highest to lowest):
//  1. CLI flags (--account, --project, etc.)
//  2. Environment variables (DEPLOYHQ_ACCOUNT, DEPLOYHQ_EMAIL, etc.)
//  3. Project config (.deployhq.toml in current dir or parents)
//  4. Global config (~/.deployhq/config.toml)
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the resolved configuration from all layers.
type Config struct {
	Account   string `mapstructure:"account"   json:"account,omitempty"`
	Email     string `mapstructure:"email"     json:"email,omitempty"`
	APIKey    string `mapstructure:"api_key"   json:"api_key,omitempty"`
	Project   string `mapstructure:"project"   json:"project,omitempty"`
	OutputFmt string `mapstructure:"format"    json:"format,omitempty"`
	Host      string `mapstructure:"host"      json:"host,omitempty"`

	// Resolved metadata (not persisted)
	Sources map[string]string `json:"sources,omitempty"` // field -> source layer
}

// Keys is the list of all config keys.
var Keys = []string{"account", "email", "api_key", "project", "format", "host"}

// Load reads config from all 4 layers and returns the resolved Config.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("format", "table")

	// Layer 4: Global config (~/.deployhq/config.toml)
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".deployhq", "config.toml")
		v.SetConfigFile(globalPath)
		v.SetConfigType("toml")
		_ = v.ReadInConfig() // ignore if not found
	}

	// Layer 3: Project config (.deployhq.toml)
	if projectPath := findProjectConfig(); projectPath != "" {
		pv := viper.New()
		pv.SetConfigFile(projectPath)
		pv.SetConfigType("toml")
		if err := pv.ReadInConfig(); err == nil {
			_ = v.MergeConfigMap(pv.AllSettings())
		}
	}

	// Layer 2: Environment variables
	v.SetEnvPrefix("DEPLOYHQ")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit bindings for keys that need them
	for _, key := range Keys {
		_ = v.BindEnv(key)
	}

	cfg := &Config{Sources: make(map[string]string)}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	// Track sources for --resolved display
	cfg.resolveSources(v)

	return cfg, nil
}

// ApplyFlags applies CLI flag overrides to the config.
func (cfg *Config) ApplyFlags(account, email, apiKey, project, format, host string) {
	if account != "" {
		cfg.Account = account
		cfg.Sources["account"] = "flag"
	}
	if email != "" {
		cfg.Email = email
		cfg.Sources["email"] = "flag"
	}
	if apiKey != "" {
		cfg.APIKey = apiKey
		cfg.Sources["api_key"] = "flag"
	}
	if project != "" {
		cfg.Project = project
		cfg.Sources["project"] = "flag"
	}
	if format != "" {
		cfg.OutputFmt = format
		cfg.Sources["format"] = "flag"
	}
	if host != "" {
		cfg.Host = host
		cfg.Sources["host"] = "flag"
	}
}

func (cfg *Config) resolveSources(v *viper.Viper) {
	for _, key := range Keys {
		if os.Getenv("DEPLOYHQ_"+strings.ToUpper(key)) != "" {
			cfg.Sources[key] = "env"
		} else if v.InConfig(key) {
			cfg.Sources[key] = "file"
		} else {
			cfg.Sources[key] = "default"
		}
	}
}

// BaseURL returns the API base URL for the given account.
// When Host is set (e.g. "deployhq.dev"), the URL becomes https://{account}.{host}.
// Otherwise it returns empty string (SDK will use its default).
func (cfg *Config) BaseURL(account string) string {
	if cfg.Host == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.%s", account, cfg.Host)
}

// SignupURL returns the signup endpoint URL.
// When Host is set, the URL becomes https://api.{host}/api/v1/signup.
// Otherwise it returns empty string (SDK will use its default).
func (cfg *Config) SignupURL() string {
	if cfg.Host == "" {
		return ""
	}
	return fmt.Sprintf("https://api.%s/api/v1/signup", cfg.Host)
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".deployhq", "config.toml")
}

// ProjectConfigPath returns the path to the project config file (in cwd).
func ProjectConfigPath() string {
	return ".deployhq.toml"
}

// findProjectConfig walks up from cwd to find .deployhq.toml.
func findProjectConfig() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		path := filepath.Join(dir, ".deployhq.toml")
		if _, err := os.Stat(path); err == nil {
			return path
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// Set writes a key-value pair to the specified config file.
func Set(path, key, value string) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	_ = v.ReadInConfig() // read existing

	v.Set(key, value)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}

	return v.WriteConfigAs(path)
}

// Unset removes a key from the specified config file.
func Unset(path, key string) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil // nothing to unset
	}

	// Viper doesn't support unsetting, so rebuild without the key
	settings := v.AllSettings()
	delete(settings, key)

	v2 := viper.New()
	v2.SetConfigFile(path)
	v2.SetConfigType("toml")
	for k, val := range settings {
		v2.Set(k, val)
	}

	return v2.WriteConfigAs(path)
}
