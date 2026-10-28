package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCmdCapturingStdout runs the real root command and returns what it wrote
// to stdout. The output envelope writes to the process streams, so they are
// swapped for pipes for the duration of the command.
func runCmdCapturingStdout(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Cleanup(func() { flagJSON = "" })
	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout, os.Stderr = outW, errW

	var wg sync.WaitGroup
	var stdout bytes.Buffer
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&stdout, outR) }()
	go func() { defer wg.Done(); _, _ = io.Copy(io.Discard, errR) }()

	cmd := NewRootCmd("test")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	runErr := cmd.Execute()

	os.Stdout, os.Stderr = origOut, origErr
	_ = outW.Close()
	_ = errW.Close()
	wg.Wait()
	return stdout.String(), runErr
}

func TestExcludedFilesCreate_JSONWritesTheCreatedRule(t *testing.T) {
	withResolvableContext(t)
	installTargetingAPI(t, `{"identifier":"ef-1","path":"deploy.sh","servers":[{"identifier":"`+targetServerUUID+`","name":"Production"}]}`)

	stdout, err := runCmdCapturingStdout(t, "excluded-files", "create", "--path", "deploy.sh", "--json")
	require.NoError(t, err)

	dec := json.NewDecoder(bytes.NewBufferString(stdout))
	var resp struct {
		OK   bool `json:"ok"`
		Data struct {
			Identifier string           `json:"identifier"`
			Path       string           `json:"path"`
			Servers    []map[string]any `json:"servers"`
		} `json:"data"`
	}
	require.NoError(t, dec.Decode(&resp), "stdout: %q", stdout)
	assert.False(t, dec.More(), "stdout holds exactly one JSON document: %q", stdout)
	assert.True(t, resp.OK)
	assert.Equal(t, "ef-1", resp.Data.Identifier)
	assert.Equal(t, "deploy.sh", resp.Data.Path)
	require.Len(t, resp.Data.Servers, 1)
	assert.Equal(t, targetServerUUID, resp.Data.Servers[0]["identifier"])
}
