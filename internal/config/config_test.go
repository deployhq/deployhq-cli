package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear env vars to test defaults
	for _, key := range []string{"DEPLOYHQ_ACCOUNT", "DEPLOYHQ_EMAIL", "DEPLOYHQ_API_KEY", "DEPLOYHQ_PROJECT", "DEPLOYHQ_FORMAT"} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "table", cfg.OutputFmt)
	assert.Empty(t, cfg.Account)
}

func TestLoad_EnvVars(t *testing.T) {
	t.Setenv("DEPLOYHQ_ACCOUNT", "myco")
	t.Setenv("DEPLOYHQ_EMAIL", "user@test.com")
	t.Setenv("DEPLOYHQ_PROJECT", "my-app")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "myco", cfg.Account)
	assert.Equal(t, "user@test.com", cfg.Email)
	assert.Equal(t, "my-app", cfg.Project)
}

func TestApplyFlags(t *testing.T) {
	cfg := &Config{Sources: make(map[string]string)}
	cfg.Account = "from-env"
	cfg.Sources["account"] = "env"

	cfg.ApplyFlags("from-flag", "", "", "proj", "")

	assert.Equal(t, "from-flag", cfg.Account)
	assert.Equal(t, "flag", cfg.Sources["account"])
	assert.Equal(t, "proj", cfg.Project)
	assert.Equal(t, "flag", cfg.Sources["project"])
}

func TestSetAndUnset(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.toml")

	// Set a value
	err := Set(path, "account", "myco")
	require.NoError(t, err)

	// Read it back
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "myco")

	// Unset it
	err = Unset(path, "account")
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "myco")
}

func TestUnset_NonexistentFile(t *testing.T) {
	err := Unset("/tmp/nonexistent-deployhq-test.toml", "account")
	assert.NoError(t, err) // should not error
}

func TestFindProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	configPath := filepath.Join(tmpDir, ".deployhq.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("project = \"test\""), 0644))

	// Create a subdirectory and search from there
	subDir := filepath.Join(tmpDir, "sub", "deep")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(subDir)

	found := findProjectConfig()
	assert.Equal(t, configPath, found)
}
