package telemetry

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
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
	Command      string
	ExitCode     int
	ErrorClass   string
	ErrorMessage string // sanitized first line of the error; empty on success
	DurationMs   int64
	CLIVersion   string
	IsAgent      bool
	AgentName    string
}

// Tracker is the interface used for sending telemetry.
// Track sends the event synchronously (with a timeout). The caller
// is responsible for running it in a goroutine if non-blocking
// behaviour is desired.
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
		mp: mixpanel.NewApiClient(mixpanelToken, mixpanel.HttpClient(client), mixpanel.EuResidency()),
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
		"command":       event.Command,
		"exit_code":     event.ExitCode,
		"error_class":   event.ErrorClass,
		"error_message": event.ErrorMessage,
		"duration_ms":   event.DurationMs,
		"cli_version":   event.CLIVersion,
		"environment":   env,
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"is_agent":      event.IsAgent,
		"agent_name":    event.AgentName,
	})

	// Synchronous send with timeout. The caller (SendTelemetry) runs
	// this in a goroutine and waits with a channel, so the request
	// actually completes before main() exits.
	ctx, cancel := context.WithTimeout(context.Background(), trackTimeout)
	defer cancel()
	_ = t.mp.Track(ctx, []*mixpanel.Event{e})
}

// nopTracker silently discards events (used when no token is configured).
type nopTracker struct{}

func (nopTracker) Track(string, Event) {}

const errorMessageMaxLen = 200

//nolint:gochecknoglobals // compiled once, used by SanitizeErrorMessage
var (
	emailRE = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	uuidRE  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	// bearerRE catches "Bearer <token>" and "Authorization: Bearer <token>" patterns.
	bearerRE = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._\-]{8,}`)
	// kvSecretRE catches key=value / token=value style leaks (api_key=..., secret=..., token=...).
	kvSecretRE = regexp.MustCompile(`(?i)\b(api[_-]?key|api[_-]?token|secret|token|password|passwd)\s*[=:]\s*\S+`)
)

// SanitizeErrorMessage produces a privacy-safe, single-line summary of an
// error suitable for telemetry. It:
//   - returns "" if err is nil
//   - keeps only the first line
//   - replaces the user's home directory with "~"
//   - redacts emails, UUIDs, bearer tokens, and key=value secrets
//   - truncates to 200 characters
func SanitizeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if home, herr := os.UserHomeDir(); herr == nil && home != "" && home != "/" {
		s = strings.ReplaceAll(s, home, "~")
	}
	s = kvSecretRE.ReplaceAllString(s, "$1=<redacted>")
	s = bearerRE.ReplaceAllString(s, "${1}<redacted>")
	s = emailRE.ReplaceAllString(s, "<email>")
	s = uuidRE.ReplaceAllString(s, "<uuid>")
	if len(s) > errorMessageMaxLen {
		s = s[:errorMessageMaxLen] + "…"
	}
	return s
}
