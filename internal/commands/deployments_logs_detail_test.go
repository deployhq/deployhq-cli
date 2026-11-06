package commands

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each log's detail is printed under its message, as the DeployHQ web app
// shows it: without it, "Excluded files for server ..." showed no files.
func TestPrintStepLogs_PrintsDetail(t *testing.T) {
	detail := "  - deploy.sh (1 file)\n"
	blank := "  \n"
	var out bytes.Buffer
	printStepLogs(&out, []sdk.DeploymentStepLog{
		{Message: "Excluded files for server 'site'", Detail: &detail},
		{Message: "Skipped due to project/account settings"},
		{Message: "Done", Detail: &blank},
	})
	assert.Equal(t, "Excluded files for server 'site'\n  - deploy.sh (1 file)\nSkipped due to project/account settings\nDone\n", out.String())
}

// fakeDeploymentLogsAPI answers deployment dep1 with the given steps body and
// serves step s1's logs (or a 500 when logsStatus is set).
func fakeDeploymentLogsAPI(t *testing.T, steps string, logsStatus int) {
	t.Helper()
	orig := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = orig })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := http.StatusOK, `{}`
		switch {
		case strings.HasSuffix(r.URL.Path, "/projects/my-app/deployments/dep1"):
			body = `{"identifier":"dep1","status":"completed","steps":` + steps + `}`
		case strings.HasSuffix(r.URL.Path, "/deployments/dep1/steps/s1/logs"):
			body = `[{"id":1,"step":"s1","message":"Excluded files for server 'site'","detail":"  - deploy.sh (1 file)"}]`
			if logsStatus != 0 {
				status, body = logsStatus, `{"error":"boom"}`
			}
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)),
			Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}, nil
	})
}

const twoSteps = `[
	{"identifier":"s1","description":"Generating manifest","status":"completed","logs":true},
	{"identifier":"s2","description":"Transferring files","status":"completed","logs":false}]`

// runLogsCapture runs 'dhq deployments logs dep1 <args>' and returns the JSON
// written (captured through DEPLOYHQ_OUTPUT_FILE).
func runLogsCapture(t *testing.T, args ...string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.jsonl")
	t.Setenv("DEPLOYHQ_OUTPUT_FILE", out)
	require.NoError(t, runServersCmd(t, append([]string{"deployments", "logs", "dep1"}, args...)...))
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	return string(b)
}

// Without --step, --json returns every step's logs, with their detail, as JSON.
func TestDeploymentsLogs_AllStepsJSON(t *testing.T) {
	withResolvableContext(t)
	fakeDeploymentLogsAPI(t, twoSteps, 0)
	got := runLogsCapture(t, "--json")
	assert.Contains(t, got, `"description":"Generating manifest"`)
	assert.Contains(t, got, `deploy.sh (1 file)`)
	assert.NotContains(t, got, "Transferring files", "steps without logs are left out")
}

// Piped without --json (the test's stdout is not a terminal), it still prints
// text, as it always has, so scripts that grep it keep working; the detail is
// now part of that text.
func TestDeploymentsLogs_PipedStaysText(t *testing.T) {
	withResolvableContext(t)
	fakeDeploymentLogsAPI(t, twoSteps, 0)

	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStdout := os.Stdout
	os.Stdout = w
	runErr := runServersCmd(t, "deployments", "logs", "dep1")
	os.Stdout = origStdout
	require.NoError(t, w.Close())
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, runErr)

	assert.Equal(t, "Excluded files for server 'site'\n  - deploy.sh (1 file)\n", string(b))
}

// A deployment with no steps yet (queued) still gives JSON: an empty array.
func TestDeploymentsLogs_NoStepsJSON(t *testing.T) {
	withResolvableContext(t)
	fakeDeploymentLogsAPI(t, `[]`, 0)
	assert.Contains(t, runLogsCapture(t, "--json"), `"data":[]`)
}

// A step whose logs can't be read is reported, not silently left out.
func TestDeploymentsLogs_StepLogsErrorReported(t *testing.T) {
	withResolvableContext(t)
	fakeDeploymentLogsAPI(t, twoSteps, http.StatusInternalServerError)
	got := runLogsCapture(t, "--json")
	assert.Contains(t, got, `"step":"s1"`)
	assert.Contains(t, got, `"error":`)
}
