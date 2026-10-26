package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// deploymentSequence answers GET /projects/p/deployments/dep-1 with the given
// statuses in order, repeating the last one forever.
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
			_ = json.NewEncoder(w).Encode(map[string]any{
				"identifier": "dep-1",
				"status":     s.next(),
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
