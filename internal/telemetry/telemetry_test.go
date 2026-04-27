package telemetry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- identity tests ---

func TestDistinctID_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()

	id1 := DistinctID(dir)
	assert.NotEmpty(t, id1)
	assert.True(t, isValidUUID(id1), "expected UUID format, got %q", id1)

	// Second call returns the same ID
	id2 := DistinctID(dir)
	assert.Equal(t, id1, id2)
}

func TestDistinctID_ReadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, telemetryIDFile)
	require.NoError(t, os.WriteFile(path, []byte("existing-id-123\n"), 0600))

	id := DistinctID(dir)
	assert.Equal(t, "existing-id-123", id)
}

func TestHasIdentity(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, HasIdentity(dir))

	_ = DistinctID(dir) // creates the file
	assert.True(t, HasIdentity(dir))
}

func isValidUUID(s string) bool {
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}
	lengths := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != lengths[i] {
			return false
		}
	}
	return true
}

// --- consent tests ---

func TestIsEnabled_DefaultTrue(t *testing.T) {
	t.Setenv("DEPLOYHQ_NO_TELEMETRY", "")
	assert.True(t, IsEnabled())
}

func TestIsEnabled_EnvVarDisables(t *testing.T) {
	t.Setenv("DEPLOYHQ_NO_TELEMETRY", "1")
	assert.False(t, IsEnabled())
}

func TestIsEnabled_EnvVarTrueDisables(t *testing.T) {
	t.Setenv("DEPLOYHQ_NO_TELEMETRY", "true")
	assert.False(t, IsEnabled())
}

func TestEnabledSource_Default(t *testing.T) {
	t.Setenv("DEPLOYHQ_NO_TELEMETRY", "")
	assert.Equal(t, "default", EnabledSource())
}

func TestEnabledSource_Env(t *testing.T) {
	t.Setenv("DEPLOYHQ_NO_TELEMETRY", "1")
	assert.Equal(t, "env", EnabledSource())
}

func TestIsFirstRun(t *testing.T) {
	// Point HOME to a temp dir so there's no ~/.deployhq/telemetry_id
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	assert.True(t, IsFirstRun())

	// Create identity file
	deployhqDir := filepath.Join(dir, ".deployhq")
	require.NoError(t, os.MkdirAll(deployhqDir, 0700))
	_ = DistinctID(deployhqDir)

	assert.False(t, IsFirstRun())
}

// --- error class mapping tests ---

func TestErrorClassFromExitCode(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "ok"},
		{1, "user"},
		{2, "internal"},
		{3, "auth"},
		{4, "network"},
		{5, "not_found"},
		{6, "conflict"},
		{130, "interrupt"},
		{99, "unknown"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, ErrorClassFromExitCode(tt.code), "exit code %d", tt.code)
	}
}

// --- tracker tests ---

func TestDefaultTracker_NoToken_ReturnsNop(t *testing.T) {
	// mixpanelToken is empty in test builds (no ldflags)
	tracker := DefaultTracker()
	_, isNop := tracker.(nopTracker)
	assert.True(t, isNop, "expected nopTracker when token is empty")
}

func TestEnvironment_Default(t *testing.T) {
	// environment var is empty in test builds
	assert.Equal(t, "dev", Environment())
}

// mockTracker records Track calls for testing.
type mockTracker struct {
	mu     sync.Mutex
	events []Event
	ids    []string
}

func (m *mockTracker) Track(distinctID string, event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ids = append(m.ids, distinctID)
	m.events = append(m.events, event)
}

func TestMockTracker(t *testing.T) {
	m := &mockTracker{}
	m.Track("test-id", Event{
		Command:    "deploy",
		ExitCode:   0,
		ErrorClass: "ok",
		DurationMs: 500,
		CLIVersion: "1.0.0",
		IsAgent:    false,
	})

	assert.Len(t, m.events, 1)
	assert.Equal(t, "deploy", m.events[0].Command)
	assert.Equal(t, "test-id", m.ids[0])
}

// --- SanitizeErrorMessage tests ---

func TestSanitizeErrorMessage_Nil(t *testing.T) {
	assert.Equal(t, "", SanitizeErrorMessage(nil))
}

func TestSanitizeErrorMessage_FirstLineOnly(t *testing.T) {
	err := errors.New("primary cause\nstack: foo\nstack: bar")
	got := SanitizeErrorMessage(err)
	assert.Equal(t, "primary cause", got)
}

func TestSanitizeErrorMessage_RedactsEmail(t *testing.T) {
	err := errors.New("user alice@example.com not found")
	got := SanitizeErrorMessage(err)
	assert.Equal(t, "user <email> not found", got)
	assert.NotContains(t, got, "alice@example.com")
}

func TestSanitizeErrorMessage_RedactsUUID(t *testing.T) {
	err := errors.New("project 550e8400-e29b-41d4-a716-446655440000 not found")
	got := SanitizeErrorMessage(err)
	assert.Equal(t, "project <uuid> not found", got)
}

func TestSanitizeErrorMessage_RedactsBearer(t *testing.T) {
	err := errors.New("invalid Bearer abc123def456ghi token")
	got := SanitizeErrorMessage(err)
	assert.NotContains(t, got, "abc123def456ghi")
	assert.Contains(t, got, "<redacted>")
}

func TestSanitizeErrorMessage_RedactsKVSecrets(t *testing.T) {
	cases := []string{
		"failed: api_key=sk_live_abc123 not accepted",
		"failed: token=eyJhbGciOiJIUzI1NiJ9 expired",
		"failed: password=hunter2 invalid",
		"failed: secret=topsecret123",
	}
	for _, msg := range cases {
		got := SanitizeErrorMessage(errors.New(msg))
		assert.Contains(t, got, "<redacted>", "input: %q", msg)
		assert.NotContains(t, got, "sk_live_abc123", "input: %q", msg)
		assert.NotContains(t, got, "eyJhbGciOiJIUzI1NiJ9", "input: %q", msg)
		assert.NotContains(t, got, "hunter2", "input: %q", msg)
		assert.NotContains(t, got, "topsecret123", "input: %q", msg)
	}
}

func TestSanitizeErrorMessage_ReplacesHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	if home == "" || home == "/" {
		t.Skip("no usable home dir")
	}
	in := errors.New("read " + filepath.Join(home, ".deployhq", "config.toml") + ": permission denied")
	got := SanitizeErrorMessage(in)
	assert.NotContains(t, got, home)
	assert.Contains(t, got, "~")
}

func TestSanitizeErrorMessage_Truncates(t *testing.T) {
	long := strings.Repeat("x", 500)
	got := SanitizeErrorMessage(errors.New(long))
	// 200 chars + the truncation marker rune
	assert.True(t, len(got) <= errorMessageMaxLen+len("…"), "got %d chars: %q", len(got), got)
	assert.True(t, strings.HasSuffix(got, "…"), "expected ellipsis suffix, got %q", got)
}

func TestSanitizeErrorMessage_EmptyError(t *testing.T) {
	assert.Equal(t, "", SanitizeErrorMessage(errors.New("")))
}
