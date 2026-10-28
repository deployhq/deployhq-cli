package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deployhq/deployhq-cli/internal/output"
)

func TestWriteJSONError_WritesOneObject(t *testing.T) {
	var buf bytes.Buffer
	writeJSONError(&buf, &output.UserError{Message: "bad input"})

	var resp map[string]interface{}
	dec := json.NewDecoder(&buf)
	require.NoError(t, dec.Decode(&resp))
	assert.Equal(t, false, resp["ok"])
	assert.False(t, dec.More())
}

// An error whose JSON response the command already wrote adds nothing.
func TestWriteJSONError_SkipsReported(t *testing.T) {
	var buf bytes.Buffer
	writeJSONError(&buf, &output.ReportedError{Err: &output.UserError{Message: "bad input"}})
	assert.Empty(t, buf.String())
}
