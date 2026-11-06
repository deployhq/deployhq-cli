package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/cli"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runSkillsShow runs 'dhq skills show' with stdout not a terminal, as when an
// agent runs it, and returns stdout.
func runSkillsShow(t *testing.T, jsonMode bool, args ...string) (string, error) {
	t.Helper()
	var stdout bytes.Buffer
	orig := cliCtx
	t.Cleanup(func() { cliCtx = orig })
	cliCtx = &cli.Context{
		Config: &config.Config{},
		Envelope: &output.Envelope{
			Stdout: &stdout, Stderr: io.Discard, Logger: &output.Logger{},
			NonInteractive: true, JSONMode: jsonMode, IsTTY: false,
		},
	}
	cmd := newSkillsShowCmd()
	err := cmd.RunE(cmd, args)
	return stdout.String(), err
}

// Piped (as an agent runs it), it prints the skill as Markdown, not JSON:
// it is a document to read.
func TestSkillsShow_PrintsSkillMarkdown(t *testing.T) {
	want, err := skills.FS.ReadFile("deployhq/SKILL.md")
	require.NoError(t, err)

	out, err := runSkillsShow(t, false)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, string(want)), "starts with SKILL.md verbatim")
	assert.Contains(t, out, "dhq skills show <name>")
	assert.Contains(t, out, "launch", "lists the references")
}

func TestSkillsShow_Reference(t *testing.T) {
	want, err := skills.FS.ReadFile("deployhq/references/launch.md")
	require.NoError(t, err)

	for _, arg := range []string{"launch", "launch.md"} {
		out, err := runSkillsShow(t, false, arg)
		require.NoError(t, err)
		assert.Equal(t, string(want), out, arg)
	}
}

func TestSkillsShow_UnknownReference(t *testing.T) {
	_, err := runSkillsShow(t, false, "nope")
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Contains(t, uerr.Message, `"nope"`)
	assert.Contains(t, uerr.Hint, "launch")

	_, err = runSkillsShow(t, false, "../SKILL")
	require.Error(t, err, "only reference documents can be printed")
}

func TestSkillsShow_JSON(t *testing.T) {
	out, err := runSkillsShow(t, true)
	require.NoError(t, err)
	var res struct {
		OK   bool            `json:"ok"`
		Data skillShowOutput `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.True(t, res.OK)
	assert.Equal(t, "SKILL", res.Data.Name)
	assert.Contains(t, res.Data.Content, "DeployHQ")
	assert.Contains(t, res.Data.References, "launch")
}
