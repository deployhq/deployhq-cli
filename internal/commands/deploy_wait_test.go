package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastPoll shortens the watcher's poll interval for the duration of a test.
func fastPoll(t *testing.T) {
	t.Helper()
	orig := pollInterval
	pollInterval = 5 * time.Millisecond
	t.Cleanup(func() { pollInterval = orig })
}

// fastRetry shortens the waits between retries of a failed status check.
func fastRetry(t *testing.T) {
	t.Helper()
	orig := fetchRetryDelays
	fetchRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { fetchRetryDelays = orig })
}

// deploymentSequence answers GET /projects/p/deployments/dep-1 with the given
// statuses in order, repeating the last one forever. An entry such as "!503"
// answers with that HTTP status instead.
type deploymentSequence struct {
	mu       sync.Mutex
	statuses []string
	calls    int
}

func (s *deploymentSequence) next() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.calls
	if i >= len(s.statuses) {
		i = len(s.statuses) - 1
	}
	s.calls++
	return s.statuses[i]
}

func (s *deploymentSequence) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *deploymentSequence) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/projects/p/deployments/dep-1" {
			status := s.next()
			if code, ok := strings.CutPrefix(status, "!"); ok {
				n, err := strconv.Atoi(code)
				require.NoError(t, err)
				w.WriteHeader(n)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": http.StatusText(n)})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"identifier": "dep-1",
				"status":     status,
				"steps":      []any{},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}
}

func jsonEnvelope() (*output.Envelope, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &output.Envelope{Stdout: &stdout, Stderr: &stderr, JSONMode: true}, &stdout, &stderr
}

func TestWaitForDeployment_JSONBlocksUntilCompleted(t *testing.T) {
	fastPoll(t)
	seq := &deploymentSequence{statuses: []string{"pending", "running", "running", "completed"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, stdout, _ := jsonEnvelope()

	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
	require.NoError(t, err)
	assert.Equal(t, 4, seq.count(), "must keep polling until a terminal status")

	var resp struct {
		OK      bool           `json:"ok"`
		Data    map[string]any `json:"data"`
		Summary string         `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &resp), "stdout: %s", stdout.String())
	assert.True(t, resp.OK)
	assert.Equal(t, "completed", resp.Data["status"], "JSON must carry the FINAL status")
	assert.Equal(t, "Deployment dep-1 completed", resp.Summary)
}

func TestWaitForDeployment_JSONFailedIsNonZero(t *testing.T) {
	fastPoll(t)
	seq := &deploymentSequence{statuses: []string{"running", "failed"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, stdout, _ := jsonEnvelope()

	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
	require.Error(t, err)
	var ue *output.UserError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
	assert.Contains(t, ue.Message, `"failed"`)
	assert.Empty(t, stdout.String(), "the error envelope is the only stdout document on failure")
}

func TestWaitForDeployment_JSONCancelledIsNonZero(t *testing.T) {
	fastPoll(t)
	seq := &deploymentSequence{statuses: []string{"cancelled"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, _, _ := jsonEnvelope()

	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"cancelled"`)
}

func TestWaitForDeployment_TimeoutReportsLastStatus(t *testing.T) {
	fastPoll(t)
	seq := &deploymentSequence{statuses: []string{"running"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, stdout, _ := jsonEnvelope()

	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Timed out after 1s")
	assert.Contains(t, err.Error(), "last status: running")
	assert.Empty(t, stdout.String())
}

func TestWatchDeploymentPlain_CancelledIsNonZero(t *testing.T) {
	fastPoll(t)
	seq := &deploymentSequence{statuses: []string{"cancelled"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, _, _ := jsonEnvelope()

	err := watchDeploymentPlain(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1")
	require.Error(t, err)
}

// TestDeployWaitCommand_JSONWaitsForFinalStatus drives the real `dhq deploy
// --wait` command with JSON output (what agents and pipes get) and asserts it
// does not return the queued deployment: it must poll to the end and print
// the completed one.
func TestDeployWaitCommand_JSONWaitsForFinalStatus(t *testing.T) {
	fastPoll(t)
	withResolvableContext(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DEPLOYHQ_PROJECT", "p")

	seq := &deploymentSequence{statuses: []string{"running", "completed"}}
	var mu sync.Mutex
	created := 0
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		respond := func(code int, body string) (*http.Response, error) {
			return &http.Response{
				StatusCode: code,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Request:    r,
			}, nil
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects/p/deployments":
			mu.Lock()
			created++
			mu.Unlock()
			return respond(http.StatusCreated, `{"identifier":"dep-1","status":"pending"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/projects/p/deployments/dep-1":
			return respond(http.StatusOK, `{"identifier":"dep-1","status":"`+seq.next()+`","steps":[]}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/projects/p/servers/"):
			return respond(http.StatusOK, `{"identifier":"11111111-1111-1111-1111-111111111111","name":"web","protocol_type":"ssh"}`)
		default:
			return respond(http.StatusOK, `{}`)
		}
	})
	t.Cleanup(func() { http.DefaultTransport = orig })

	// Capture the real stdout: the envelope binds os.Stdout at startup.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"deploy", "--json", "--wait",
		"-s", "11111111-1111-1111-1111-111111111111",
		"-b", "main", "-r", "abc123", "--start-revision", "def456"})
	execErr := cmd.Execute()
	_ = w.Close()
	os.Stdout = origStdout
	out, _ := io.ReadAll(r)

	require.NoError(t, execErr, "stdout: %s", out)
	assert.Equal(t, 1, created)
	assert.GreaterOrEqual(t, seq.count(), 2, "--wait must poll past the queued state")

	var resp struct {
		Data    map[string]any `json:"data"`
		Summary string         `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(out, &resp), "stdout: %s", out)
	assert.Equal(t, "completed", resp.Data["status"])
	assert.NotContains(t, resp.Summary, "queued")
}

// ttyEnvelope is an interactive terminal without --json: --wait shows the
// live view there.
func ttyEnvelope(t *testing.T) (*output.Envelope, *bytes.Buffer) {
	t.Helper()
	orig := watchTUIOptions
	watchTUIOptions = []tea.ProgramOption{tea.WithInput(nil)}
	t.Cleanup(func() { watchTUIOptions = orig })
	var stdout, stderr bytes.Buffer
	return &output.Envelope{Stdout: &stdout, Stderr: &stderr, IsTTY: true}, &stderr
}

func TestWaitForDeployment_TerminalCancelledIsNonZero(t *testing.T) {
	fastPoll(t)
	seq := &deploymentSequence{statuses: []string{"running", "cancelled"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, _ := ttyEnvelope(t)

	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"cancelled"`)
}

func TestWaitForDeployment_TerminalTimeoutReportsLastStatus(t *testing.T) {
	fastPoll(t)
	seq := &deploymentSequence{statuses: []string{"running"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, _ := ttyEnvelope(t)

	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Timed out after 1s")
	assert.Contains(t, err.Error(), "last status: running")
}

// The live view must stop at the deadline, not at its next poll: with the
// normal poll interval, a 1s timeout still returns in about a second.
func TestWaitForDeployment_TerminalTimeoutIsPrompt(t *testing.T) {
	seq := &deploymentSequence{statuses: []string{"running"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, _ := ttyEnvelope(t)

	start := time.Now()
	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 1)
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Timed out after 1s")
	assert.Contains(t, err.Error(), "last status: running")
	assert.Less(t, elapsed, 2*time.Second, "timeout should not wait for the next poll")
}

// --table (or -q) turns JSON off even without a terminal, e.g. in CI. --wait
// must still wait there, with plain progress, and never open the live view.
func TestWaitForDeployment_TableWithoutTerminalWaits(t *testing.T) {
	for name, tc := range map[string]struct {
		statuses []string
		wantErr  bool
	}{
		"completed": {statuses: []string{"running", "completed"}},
		"cancelled": {statuses: []string{"running", "cancelled"}, wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			fastPoll(t)
			seq := &deploymentSequence{statuses: tc.statuses}
			srv := httptest.NewServer(seq.handler(t))
			defer srv.Close()
			var stdout, stderr bytes.Buffer
			env := &output.Envelope{Stdout: &stdout, Stderr: &stderr, TableMode: true}

			err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `"cancelled"`)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, 2, seq.count(), "it polled until the deployment finished")
			assert.Empty(t, stdout.String(), "no JSON is written when JSON is off")
		})
	}
}

// With stdout on a terminal but stdin piped, the CLI runs non-interactive.
// The live view needs keyboard input, so --wait uses plain progress there.
func TestWaitForDeployment_NonInteractiveTerminalUsesPlainProgress(t *testing.T) {
	fastPoll(t)
	seq := &deploymentSequence{statuses: []string{"running", "completed"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr, IsTTY: true, NonInteractive: true}

	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
	require.NoError(t, err)
	assert.Equal(t, 2, seq.count(), "it polled until the deployment finished")
	assert.Contains(t, stderr.String(), "Deployment completed")
}

// A status check that fails for a reason that may pass (a 5xx, a 429, a
// network error) is retried, so one blip does not end the wait while the
// deployment keeps running.
func TestWaitForDeployment_RetriesATransientFailedCheck(t *testing.T) {
	for _, blip := range []string{"!503", "!502", "!429"} {
		t.Run(blip, func(t *testing.T) {
			fastPoll(t)
			fastRetry(t)
			seq := &deploymentSequence{statuses: []string{"running", blip, "running", "completed"}}
			srv := httptest.NewServer(seq.handler(t))
			defer srv.Close()
			env, stdout, _ := jsonEnvelope()

			err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
			require.NoError(t, err)
			assert.Equal(t, 4, seq.count())
			assert.Contains(t, stdout.String(), `"status": "completed"`)
		})
	}
}

func TestWaitForDeployment_GivesUpAfterRetriesAndSaysItMayStillRun(t *testing.T) {
	fastPoll(t)
	fastRetry(t)
	seq := &deploymentSequence{statuses: []string{"running", "!503"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, stdout, _ := jsonEnvelope()

	err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
	require.Error(t, err)
	var ue *output.UserError
	require.ErrorAs(t, err, &ue)
	assert.Contains(t, ue.Message, "Could not check deployment dep-1")
	assert.Contains(t, ue.Message, "last status: running")
	assert.Contains(t, ue.Message, "may still be running")
	assert.Contains(t, ue.Hint, "dhq deployments show dep-1 -p p")
	assert.Equal(t, 1+1+len(fetchRetryDelays), seq.count(), "one success, then the failed check and its retries")
	assert.Empty(t, stdout.String())
}

// A refusal that retrying cannot fix (wrong key, suspended account, deleted
// deployment) is returned at once and keeps its own exit code.
func TestWaitForDeployment_DoesNotRetryAPermanentFailure(t *testing.T) {
	for _, tc := range []struct {
		status string
		exit   int
	}{
		{"!401", output.ExitAuthError},
		{"!404", output.ExitNotFoundError},
	} {
		t.Run(tc.status, func(t *testing.T) {
			fastPoll(t)
			fastRetry(t)
			seq := &deploymentSequence{statuses: []string{"running", tc.status}}
			srv := httptest.NewServer(seq.handler(t))
			defer srv.Close()
			env, _, _ := jsonEnvelope()

			err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
			require.Error(t, err)
			assert.Equal(t, 2, seq.count(), "must not retry")
			assert.Equal(t, tc.exit, output.ClassifyError(err))
		})
	}
}

// In the live view a check that keeps failing is reported as such, not as
// "finished with status ..." with whatever status was last seen.
func TestWaitForDeployment_TerminalFailedCheckIsReported(t *testing.T) {
	for _, statuses := range [][]string{{"running", "!500"}, {"!500"}} {
		t.Run(strings.Join(statuses, ","), func(t *testing.T) {
			fastPoll(t)
			fastRetry(t)
			seq := &deploymentSequence{statuses: statuses}
			srv := httptest.NewServer(seq.handler(t))
			defer srv.Close()
			env, _ := ttyEnvelope(t)

			err := waitForDeployment(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1", 0)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "may still be running")
			assert.NotContains(t, err.Error(), "finished with status")
		})
	}
}

// deployments watch shares the live view: a check that keeps failing is an
// error there too, not a silent exit 0.
func TestWatchDeploymentTUI_FailedCheckIsAnError(t *testing.T) {
	fastPoll(t)
	fastRetry(t)
	seq := &deploymentSequence{statuses: []string{"running", "!500"}}
	srv := httptest.NewServer(seq.handler(t))
	defer srv.Close()
	env, _ := ttyEnvelope(t)

	err := watchDeploymentTUI(context.Background(), newTestSDKClient(t, srv), env, "p", "dep-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "may still be running")
}
