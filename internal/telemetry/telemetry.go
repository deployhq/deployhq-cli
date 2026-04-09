package telemetry

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/mixpanel/mixpanel-go"
)

// mixpanelToken is set at build time via ldflags:
//
//	-X github.com/deployhq/deployhq-cli/internal/telemetry.mixpanelToken=...
var mixpanelToken string //nolint:gochecknoglobals

// environment is set at build time via ldflags:
//
//	-X github.com/deployhq/deployhq-cli/internal/telemetry.environment=...
var environment string //nolint:gochecknoglobals

const (
	eventName    = "cli_command"
	trackTimeout = 2 * time.Second
)

// Event holds the properties sent with each telemetry event.
type Event struct {
	Command    string
	ExitCode   int
	ErrorClass string
	DurationMs int64
	CLIVersion string
	IsAgent    bool
	AgentName  string
}

// Tracker is the interface used for sending telemetry.
// It exists so tests can substitute a mock.
type Tracker interface {
	Track(distinctID string, event Event)
}

// DefaultTracker returns a Tracker backed by Mixpanel.
// If the build has no token, it returns a no-op tracker.
func DefaultTracker() Tracker {
	if mixpanelToken == "" {
		return nopTracker{}
	}
	client := &http.Client{Timeout: trackTimeout}
	return &mixpanelTracker{
		mp: mixpanel.NewApiClient(mixpanelToken, mixpanel.HttpClient(client)),
	}
}

// Environment returns the build environment label ("production", "staging",
// or "dev" for local builds).
func Environment() string {
	if environment == "" {
		return "dev"
	}
	return environment
}

// Token returns the Mixpanel project token (empty for local builds).
func Token() string {
	return mixpanelToken
}

// ErrorClassFromExitCode maps an exit code to a human-readable error class.
func ErrorClassFromExitCode(code int) string {
	switch code {
	case 0:
		return "ok"
	case 1:
		return "user"
	case 2:
		return "internal"
	case 3:
		return "auth"
	case 4:
		return "network"
	case 5:
		return "not_found"
	case 6:
		return "conflict"
	case 130:
		return "interrupt"
	default:
		return "unknown"
	}
}

// mixpanelTracker sends events to Mixpanel.
type mixpanelTracker struct {
	mp *mixpanel.ApiClient
}

func (t *mixpanelTracker) Track(distinctID string, event Event) {
	env := Environment()

	e := t.mp.NewEvent(eventName, distinctID, map[string]any{
		"command":     event.Command,
		"exit_code":   event.ExitCode,
		"error_class": event.ErrorClass,
		"duration_ms": event.DurationMs,
		"cli_version": event.CLIVersion,
		"environment": env,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"is_agent":    event.IsAgent,
		"agent_name":  event.AgentName,
	})

	// Fire-and-forget: don't block the CLI exit.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), trackTimeout)
		defer cancel()
		_ = t.mp.Track(ctx, []*mixpanel.Event{e})
	}()
}

// nopTracker silently discards events (used when no token is configured).
type nopTracker struct{}

func (nopTracker) Track(string, Event) {}
