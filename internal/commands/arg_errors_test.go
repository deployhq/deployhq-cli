package commands

import (
	"io"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeRoot runs the real root command with args and returns its error.
// Argument and flag errors surface before any command code runs, so no
// config, credentials or network are involved.
func executeRoot(t *testing.T, args ...string) error {
	t.Helper()
	t.Cleanup(func() { flagJSON = "" })
	root := NewRootCmd("test")
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	return root.Execute()
}

func TestArgErrors_ExitUserError(t *testing.T) {
	cases := []struct {
		name string
		args []string
		msg  string
	}{
		{"too many args", []string{"deployments", "show", "a", "b"}, "accepts 1 arg(s), received 2"},
		{"missing arg", []string{"deployments", "show"}, "accepts 1 arg(s), received 0"},
		{"unknown flag", []string{"deployments", "list", "--bogus"}, "unknown flag: --bogus"},
		{"flag without value", []string{"deployments", "show", "x", "--project"}, "flag needs an argument"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := executeRoot(t, tc.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.msg, "cobra's message is kept")
			assert.Equal(t, output.ExitUserError, output.ClassifyError(err))

			data := output.ErrorResponseFromErr(err).Data.(output.ErrorData)
			assert.Equal(t, output.ExitUserError, data.ExitCode)
			assert.Equal(t, "user_error", data.Code)
		})
	}
}

// `--json status,steps` (without '=') turns the fields into an extra argument;
// the structured error still suggests the '=' form.
func TestArgErrors_JSONFieldsSuggestionKept(t *testing.T) {
	err := executeRoot(t, "deployments", "show", "abc", "--json", "status,steps")
	require.Error(t, err)
	assert.Equal(t, output.ExitUserError, output.ClassifyError(err))
	data := output.ErrorResponseFromErr(err).Data.(output.ErrorData)
	assert.Contains(t, data.Suggestion, "use --json=field1,field2")
}

// An Args validator that already returns a typed error keeps it unchanged.
func TestArgErrors_TypedErrorKept(t *testing.T) {
	authErr := &output.AuthError{Message: "sign in first", Hint: "dhq auth login"}
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{
		Use:  "child",
		Args: func(*cobra.Command, []string) error { return authErr },
		RunE: func(*cobra.Command, []string) error { return nil },
	}
	root.AddCommand(child)
	argErrorsAsUserErrors(root)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"child"})

	err := root.Execute()
	assert.Same(t, authErr, err)
}
