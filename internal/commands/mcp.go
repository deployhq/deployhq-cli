package commands

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

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
			binary := findMCPBinary()
			if binary == "" {
				return &output.UserError{
					Message: "MCP server binary not found",
					Hint:    "Install with: npm install -g deployhq-mcp-server",
				}
			}

			cliCtx.Logger.Write("Starting MCP server: %s", binary)

			// Exec the MCP server, replacing this process
			c := exec.Command(binary, args...)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			// Pass through DeployHQ env vars
			c.Env = os.Environ()

			if err := c.Run(); err != nil {
				return &output.InternalError{Message: "MCP server exited", Cause: err}
			}
			return nil
		},
	}
}

func findMCPBinary() string {
	// 1. In PATH
	if p, err := exec.LookPath("deployhq-mcp-server"); err == nil {
		return p
	}

	// 2. Local node_modules
	local := filepath.Join("node_modules", ".bin", "deployhq-mcp-server")
	if _, err := os.Stat(local); err == nil {
		return local
	}

	// 3. npx fallback
	if p, err := exec.LookPath("npx"); err == nil {
		return p + " deployhq-mcp-server"
	}

	return ""
}
