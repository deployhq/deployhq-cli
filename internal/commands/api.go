package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAPICmd() *cobra.Command {
	var jsonBody string

	cmd := &cobra.Command{
		Use:   "api <method> <path>",
		Short: "Make a raw API request (escape hatch)",
		Long:  "Make a raw API request to any DeployHQ endpoint. Covers all 144+ endpoints including those without dedicated commands.",
		Example: `  # GET request
  dhq api GET /projects

  # POST with a JSON body
  dhq api POST /projects --body '{"project":{"name":"New"}}'

  # Nested resources
  dhq api GET /projects/my-app/deployments

  # DELETE request
  dhq api DELETE /projects/my-app/servers/srv-123`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]

			// Validate method
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE":
			default:
				return &output.UserError{
					Message: fmt.Sprintf("Invalid HTTP method: %s", method),
					Hint:    "Use GET, POST, PUT, PATCH, or DELETE",
				}
			}

			// Detect a path that a Git Bash / MSYS shell has rewritten into a
			// Windows filesystem path (POSIX path conversion turns a leading "/"
			// into "C:/Program Files/Git/..."). We refuse rather than guess the
			// intended endpoint — this is a raw escape hatch and reconstructing
			// the wrong path could fire a destructive call.
			if isShellMangledPath(path) {
				return &output.UserError{
					Message: "API path was rewritten by your shell (Git Bash/MSYS turned it into a Windows path)",
					Hint: "Your shell converted the leading '/' into a filesystem path. Re-run either way:\n" +
						fmt.Sprintf("  MSYS_NO_PATHCONV=1 dhq api %s /<path>   (disable path conversion)\n", method) +
						fmt.Sprintf("  dhq api %s <path>                       (omit the leading slash, e.g. projects/my-app)", method),
				}
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			// Parse body if provided
			var body interface{}
			if jsonBody != "" {
				if err := json.Unmarshal([]byte(jsonBody), &body); err != nil {
					return &output.UserError{
						Message: fmt.Sprintf("Invalid JSON body: %v", err),
						Hint:    "Provide valid JSON with --body",
					}
				}
			}

			var result interface{}
			if err := client.Do(cliCtx.Background(), method, path, body, &result); err != nil {
				return err
			}

			if result != nil {
				return cliCtx.Envelope.WriteJSON(result)
			}
			cliCtx.Envelope.Status("OK (%s %s)", method, path)
			return nil
		},
	}

	cmd.Flags().StringVar(&jsonBody, "body", "", "JSON request body")
	return cmd
}

// isShellMangledPath reports whether p looks like an API path that a Git Bash /
// MSYS shell has rewritten into a Windows filesystem path via POSIX path
// conversion. A real API path is relative ("projects/x") or root-absolute
// ("/projects/x") and never begins with a drive letter, so a leading "C:/" or
// "C:\" is a reliable signal that the argument was mangled before the CLI saw
// it.
func isShellMangledPath(p string) bool {
	if len(p) < 3 {
		return false
	}
	c := p[0]
	isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
	return isLetter && p[1] == ':' && (p[2] == '/' || p[2] == '\\')
}
