package telemetry

import (
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
