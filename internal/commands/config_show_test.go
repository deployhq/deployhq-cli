package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/cli"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigShow_JSON_MasksAPIKey(t *testing.T) {
	const rawKey = "sekret-XYZ-1234"
	masked := maskAPIKey(rawKey)

	for name, args := range map[string][]string{
		"plain":    nil,
		"resolved": {"--resolved"},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout bytes.Buffer
			orig := cliCtx
			t.Cleanup(func() { cliCtx = orig })
			cliCtx = &cli.Context{
				Config: &config.Config{
					Account: "acme",
					APIKey:  rawKey,
					Sources: map[string]string{"account": "env", "api_key": "env"},
				},
				Envelope: &output.Envelope{
					Stdout: &stdout, Stderr: io.Discard,
					Logger: &output.Logger{}, NonInteractive: true, JSONMode: true,
				},
			}

			cmd := newConfigShowCmd()
			cmd.SetArgs(args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			require.NoError(t, cmd.Execute())

			out := stdout.String()
			assert.NotContains(t, out, rawKey)
			assert.Contains(t, out, masked)
			assert.Equal(t, rawKey, cliCtx.Config.APIKey, "must not mutate the loaded config")

			var doc map[string]any
			require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc), "got %q", out)
			if name == "resolved" {
				assert.Contains(t, out, `"sources"`)
			}
		})
	}
}

func TestConfigShow_JSON_EmptyAPIKeyStaysOmitted(t *testing.T) {
	var stdout bytes.Buffer
	orig := cliCtx
	t.Cleanup(func() { cliCtx = orig })
	cliCtx = &cli.Context{
		Config: &config.Config{Account: "acme"},
		Envelope: &output.Envelope{
			Stdout: &stdout, Stderr: io.Discard,
			Logger: &output.Logger{}, NonInteractive: true, JSONMode: true,
		},
	}

	cmd := newConfigShowCmd()
	cmd.SetArgs(nil)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stdout.String(), "api_key")
}
