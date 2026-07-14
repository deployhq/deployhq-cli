package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateSSHCommand_OmitsEmptyTiming guards the regression that hit v0.16.3:
// when `--timing` was not supplied, the CLI sent `"timing":""` in the PATCH body
// and the API rejected it with "timing is not included in the list".
// `omitempty` on the Timing field must drop it from the JSON when unset.
func TestUpdateSSHCommand_OmitsEmptyTiming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/projects/my-app/commands/cmd1", r.URL.Path)

		raw, err := readAllJSON(r)
		require.NoError(t, err)
		_, hasTiming := raw["command"].(map[string]interface{})["timing"]
		assert.False(t, hasTiming, "timing must be omitted when empty, got body: %v", raw)

		_ = json.NewEncoder(w).Encode(SSHCommand{Identifier: "cmd1", Timing: "all"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.UpdateSSHCommand(context.Background(), "my-app", "cmd1", SSHCommandCreateRequest{
		Description: "Restart workers",
	})
	require.NoError(t, err)
}

// TestUpdateSSHCommand_SendsTiming verifies that an explicit timing value is sent through.
func TestUpdateSSHCommand_SendsTiming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := readAllJSON(r)
		require.NoError(t, err)
		assert.Equal(t, "after_first", raw["command"].(map[string]interface{})["timing"])

		_ = json.NewEncoder(w).Encode(SSHCommand{Identifier: "cmd1", Timing: "after_first"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.UpdateSSHCommand(context.Background(), "my-app", "cmd1", SSHCommandCreateRequest{
		Timing: "after_first",
	})
	require.NoError(t, err)
}

func readAllJSON(r *http.Request) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func TestLinkGlobalCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/projects/my-app/commands/link_global", r.URL.Path)

		// Body must be FLAT (top-level command_id).
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "cmd-1", body["command_id"])
		_, hasWrapper := body["command"]
		assert.False(t, hasWrapper, "body should be flat, not wrapped in command")

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SSHCommand{Identifier: "cmd-1", Command: "echo hi", Timing: "all"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	cmd, err := c.LinkGlobalCommand(context.Background(), "my-app", "cmd-1")
	require.NoError(t, err)
	assert.Equal(t, "cmd-1", cmd.Identifier)
	assert.Equal(t, "echo hi", cmd.Command)
}

func TestUnlinkGlobalCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/projects/my-app/commands/unlink_global", r.URL.Path)

		// DELETE carries a FLAT body.
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "cmd-1", body["command_id"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.UnlinkGlobalCommand(context.Background(), "my-app", "cmd-1")
	require.NoError(t, err)
}
