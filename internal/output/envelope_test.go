package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvelope_WriteJSON(t *testing.T) {
	var stdout bytes.Buffer
	env := &Envelope{Stdout: &stdout, Stderr: os.Stderr, Logger: &Logger{}}

	data := map[string]string{"name": "My App", "status": "active"}
	err := env.WriteJSON(data)
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	assert.Equal(t, "My App", result["name"])
	assert.Equal(t, "active", result["status"])
}

func TestEnvelope_WriteJSON_FieldSelection(t *testing.T) {
	var stdout bytes.Buffer
	env := &Envelope{
		Stdout:     &stdout,
		Stderr:     os.Stderr,
		Logger:     &Logger{},
		JSONMode:   true,
		JSONFields: []string{"name"},
	}

	data := map[string]interface{}{"name": "My App", "status": "active", "id": 123}
	err := env.WriteJSON(data)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	assert.Equal(t, "My App", result["name"])
	assert.Nil(t, result["status"])
	assert.Nil(t, result["id"])
}

func TestEnvelope_WriteJSON_ArrayFieldSelection(t *testing.T) {
	var stdout bytes.Buffer
	env := &Envelope{
		Stdout:     &stdout,
		Stderr:     os.Stderr,
		Logger:     &Logger{},
		JSONMode:   true,
		JSONFields: []string{"name", "status"},
	}

	data := []map[string]interface{}{
		{"name": "App 1", "status": "active", "id": 1},
		{"name": "App 2", "status": "paused", "id": 2},
	}
	err := env.WriteJSON(data)
	require.NoError(t, err)

	var result []map[string]interface{}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	assert.Len(t, result, 2)
	assert.Equal(t, "App 1", result[0]["name"])
	assert.Nil(t, result[0]["id"])
}

func TestEnvelope_WriteJSON_ResponseEnvelopeFieldSelection(t *testing.T) {
	var stdout bytes.Buffer
	env := &Envelope{
		Stdout:     &stdout,
		Stderr:     os.Stderr,
		Logger:     &Logger{},
		JSONMode:   true,
		JSONFields: []string{"name", "permalink"},
	}

	projects := []map[string]interface{}{
		{"name": "App 1", "permalink": "app-1", "id": 1, "status": "active"},
		{"name": "App 2", "permalink": "app-2", "id": 2, "status": "paused"},
	}
	resp := NewResponse(projects, "2 projects",
		Breadcrumb{Action: "show", Cmd: "dhq projects show <permalink>"},
	)
	err := env.WriteJSON(resp)
	require.NoError(t, err)

	var result []map[string]interface{}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	assert.Len(t, result, 2)
	assert.Equal(t, "App 1", result[0]["name"])
	assert.Equal(t, "app-1", result[0]["permalink"])
	assert.Nil(t, result[0]["id"], "id should be filtered out")
	assert.Nil(t, result[0]["status"], "status should be filtered out")
}

func TestEnvelope_WriteTable(t *testing.T) {
	var stdout bytes.Buffer
	env := &Envelope{Stdout: &stdout, Stderr: os.Stderr, Logger: &Logger{}}

	env.WriteTable(
		[]string{"Name", "Status"},
		[][]string{
			{"My App", "active"},
			{"Other App", "paused"},
		},
	)

	output := stdout.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "My App")
	assert.Contains(t, output, "active")
	assert.Contains(t, output, "Other App")
}

func TestEnvelope_WriteTable_Empty(t *testing.T) {
	var stdout bytes.Buffer
	env := &Envelope{Stdout: &stdout, Stderr: os.Stderr, Logger: &Logger{}}

	env.WriteTable([]string{"Name"}, [][]string{})
	assert.Empty(t, stdout.String())
}

func TestEnvelope_Status(t *testing.T) {
	var stderr bytes.Buffer
	env := &Envelope{Stdout: os.Stdout, Stderr: &stderr, Logger: &Logger{}}

	env.Status("Deploying %s to %s", "main", "production")
	assert.Contains(t, stderr.String(), "Deploying main to production")
}

func TestEnvelope_Error_UserError(t *testing.T) {
	var stderr bytes.Buffer
	env := &Envelope{Stdout: os.Stdout, Stderr: &stderr, Logger: &Logger{}}

	env.Error(&UserError{Message: "bad input", Hint: "try again"})
	assert.Contains(t, stderr.String(), "bad input")
	assert.Contains(t, stderr.String(), "try again")
}

func TestEnvelope_Error_InternalError(t *testing.T) {
	var stderr bytes.Buffer
	logger := &Logger{Path: "/tmp/test.log"}
	env := &Envelope{Stdout: os.Stdout, Stderr: &stderr, Logger: logger}

	env.Error(&InternalError{Message: "something broke"})
	assert.Contains(t, stderr.String(), "Internal error")
	assert.Contains(t, stderr.String(), "/tmp/test.log")
}

func TestEnvelope_JSONL_OutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.jsonl")

	f, err := os.Create(outputPath)
	require.NoError(t, err)

	var stdout bytes.Buffer
	env := &Envelope{
		Stdout:     &stdout,
		Stderr:     os.Stderr,
		Logger:     &Logger{},
		OutputFile: f,
	}

	require.NoError(t, env.WriteJSON(map[string]string{"id": "1", "name": "first"}))
	require.NoError(t, env.WriteJSON(map[string]string{"id": "2", "name": "second"}))
	env.Close()

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	assert.Len(t, lines, 2)

	var first map[string]string
	require.NoError(t, json.Unmarshal(lines[0], &first))
	assert.Equal(t, "1", first["id"])
}
