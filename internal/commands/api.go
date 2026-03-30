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
		Long: `Make a raw API request to any DeployHQ endpoint.
Covers all 144+ endpoints including those without dedicated commands.

Examples:
  deployhq api GET /projects
  deployhq api POST /projects --body '{"project":{"name":"New"}}'
  deployhq api GET /projects/my-app/deployments
  deployhq api DELETE /projects/my-app/servers/srv-123`,
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
