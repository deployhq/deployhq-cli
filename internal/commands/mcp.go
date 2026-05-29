package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// mcpBinary describes how to launch the MCP server.
type mcpBinary struct {
	path string   // executable path
	args []string // extra args prepended before user args
}

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server (stdio passthrough)",
		Long: `Start the DeployHQ MCP server in stdio mode.
This passes stdin/stdout through to the MCP server binary,
allowing AI agents to use DeployHQ via the Model Context Protocol.

The MCP server binary is searched in:
  1. deployhq-mcp-server in PATH
  2. ./node_modules/.bin/deployhq-mcp-server
  3. npx deployhq-mcp-server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			bin := findMCPBinary()
			if bin == nil {
				// Offer to install if running interactively
				if !env.NonInteractive {
					env.Status("MCP server not found.")
					prompt := promptui.Select{
						Label: "Install deployhq-mcp-server",
						Items: []string{
							"npm install -g deployhq-mcp-server",
							"Cancel",
						},
					}
					idx, _, err := prompt.Run()
					if err != nil || idx == 1 {
						return &output.UserError{Message: "MCP server not installed"}
					}

					env.Status("Installing deployhq-mcp-server...")
					c := exec.Command("npm", "install", "-g", "deployhq-mcp-server")
					c.Stdout = env.Stderr
					c.Stderr = env.Stderr
					if err := c.Run(); err != nil {
						return &output.InternalError{Message: "npm install failed", Cause: err}
					}

					// Re-detect after install
					bin = findMCPBinary()
					if bin == nil {
						return &output.InternalError{Message: "installed but binary not found in PATH"}
					}
				} else {
					return &output.UserError{
						Message: "MCP server binary not found",
						Hint:    "Install with: npm install -g deployhq-mcp-server",
					}
				}
			}

			// The MCP server reads credentials from DEPLOYHQ_ACCOUNT / DEPLOYHQ_EMAIL /
			// DEPLOYHQ_API_KEY and exits with status 1 if any is missing. Inject the
			// resolved CLI credentials so users who logged in via keyring (the common
			// case) don't have to re-export them just to start the server.
			account, email, apiKey, err := cliCtx.Credentials()
			if err != nil {
				return err
			}

			cmdArgs := append(bin.args, args...)
			cliCtx.Logger.Write("Starting MCP server: %s %v", bin.path, cmdArgs)

			c := exec.Command(bin.path, cmdArgs...)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Env = mergeEnv(os.Environ(), map[string]string{
				"DEPLOYHQ_ACCOUNT": account,
				"DEPLOYHQ_EMAIL":   email,
				"DEPLOYHQ_API_KEY": apiKey,
			})

			if err := c.Run(); err != nil {
				return &output.InternalError{Message: "MCP server exited", Cause: err}
			}
			return nil
		},
	}
}

// mergeEnv returns env with each key in overrides set to its value, only when
// the key is not already present. This preserves the user's explicit env-var
// overrides (e.g. pointing the MCP server at a different account) while still
// providing sensible defaults from the keyring.
func mergeEnv(env []string, overrides map[string]string) []string {
	present := make(map[string]struct{}, len(env))
	for _, kv := range env {
		if i := strings.Index(kv, "="); i > 0 {
			present[kv[:i]] = struct{}{}
		}
	}
	for k, v := range overrides {
		if _, ok := present[k]; ok {
			continue
		}
		if v == "" {
			continue
		}
		env = append(env, k+"="+v)
	}
	return env
}

func findMCPBinary() *mcpBinary {
	// 1. In PATH
	if p, err := exec.LookPath("deployhq-mcp-server"); err == nil {
		return &mcpBinary{path: p}
	}

	// 2. Local node_modules
	local := filepath.Join("node_modules", ".bin", "deployhq-mcp-server")
	if abs, err := filepath.Abs(local); err == nil {
		if _, err := os.Stat(abs); err == nil {
			return &mcpBinary{path: abs}
		}
	}

	// 3. npx fallback
	if p, err := exec.LookPath("npx"); err == nil {
		return &mcpBinary{path: p, args: []string{"deployhq-mcp-server"}}
	}

	return nil
}

// MCPConfigSnippet returns a JSON snippet for configuring the MCP server
// in agent config files (e.g. Claude Desktop, Cursor).
func MCPConfigSnippet() string {
	return `{
  "mcpServers": {
    "deployhq": {
      "command": "dhq",
      "args": ["mcp"]
    }
  }
}`
}
