package telemetry

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/spf13/viper"
)

const (
	envNoTelemetry = "DEPLOYHQ_NO_TELEMETRY"
	configKey      = "telemetry"

	firstRunNotice = `DeployHQ CLI collects anonymous usage data to improve the tool.
Run 'dhq telemetry disable' to opt out. Learn more: dhq telemetry status`
)

// FirstRunNotice returns the text shown on the user's first CLI invocation.
func FirstRunNotice() string { return firstRunNotice }

// IsEnabled returns true when telemetry should be sent.
// Check order: env var > global config > default (enabled).
func IsEnabled() bool {
	// 1. Environment variable override
	if v := os.Getenv(envNoTelemetry); v == "1" || strings.EqualFold(v, "true") {
		return false
	}

	// 2. Global config explicit setting
	if val, ok := readGlobalConfigBool(configKey); ok {
		return val
	}

	// 3. Default: enabled
	return true
}

// EnabledSource returns a human-readable string describing why telemetry
// is enabled or disabled ("env", "config", or "default").
func EnabledSource() string {
	if v := os.Getenv(envNoTelemetry); v == "1" || strings.EqualFold(v, "true") {
		return "env"
	}
	if _, ok := readGlobalConfigBool(configKey); ok {
		return "config"
	}
	return "default"
}

// SetEnabled writes the telemetry preference to the global config file.
func SetEnabled(enabled bool) error {
	path := config.GlobalConfigPath()
	val := "false"
	if enabled {
		val = "true"
	}
	return config.Set(path, configKey, val)
}

// IsFirstRun returns true if the user has never run the CLI before
// (no telemetry_id file exists).
func IsFirstRun() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return !HasIdentity(filepath.Join(home, ".deployhq"))
}

// readGlobalConfigBool reads a boolean key from the global config file.
func readGlobalConfigBool(key string) (bool, bool) {
	path := config.GlobalConfigPath()
	if path == "" {
		return false, false
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return false, false
	}

	if !v.IsSet(key) {
		return false, false
	}
	return v.GetBool(key), true
}
