package commands

import (
	"encoding/json"
	"fmt"
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

// hostedMCPURL is the hosted, read-only DeployHQ MCP server. Nothing to
// install; the user signs in to DeployHQ from their assistant.
const hostedMCPURL = "https://mcp.deployhq.com/mcp"

func newMCPCmd() *cobra.Command {
	var local bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Set up the hosted DeployHQ MCP server",
		Long: `Show how to connect an AI assistant to the hosted DeployHQ MCP server at
` + hostedMCPURL + `

Nothing to install: you sign in to DeployHQ from your assistant. The hosted
server is read-only (accounts, projects, servers and deployments); use the
CLI for anything that changes a deployment.

The previous local stdio server (the deployhq-mcp-server npm package) is still
available with --local. It is also used when stdin is piped, so existing client
configs of the form {"command": "dhq", "args": ["mcp"]} keep working; that
fallback is deprecated.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			switch decideMCPMode(local, stdinIsPiped(), env.JSONMode) {
			case mcpModeLocal:
				return runLocalMCP(args)
			case mcpModeLegacyStdio:
				env.Warn("`dhq mcp` over stdio is deprecated; point your client at the hosted server %s (run `dhq mcp` in a terminal for setup). Use `dhq mcp --local` to keep the local server.", hostedMCPURL)
				return runLocalMCP(args)
			}
			return printHostedMCPSetup(env)
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "Run the local stdio server (deployhq-mcp-server) instead of showing hosted setup")
	return cmd
}

type mcpMode int

const (
	mcpModeSetup       mcpMode = iota // print hosted-server setup
	mcpModeLocal                      // --local: run the stdio server
	mcpModeLegacyStdio                // piped stdin: an old client config launched us
)

// decideMCPMode picks what `dhq mcp` does. --local and an explicit --json are
// authoritative; piped stdin alone is only a heuristic for "an MCP client
// launched me", so it must not override an explicit request for setup output
// (agents and CI routinely give subprocesses a pipe for stdin).
func decideMCPMode(local, stdinPiped, explicitJSON bool) mcpMode {
	switch {
	case local:
		return mcpModeLocal
	case explicitJSON:
		return mcpModeSetup
	case stdinPiped:
		return mcpModeLegacyStdio
	}
	return mcpModeSetup
}

// printHostedMCPSetup writes the hosted setup. Stdout carries only data (the
// JSON config snippet, or the structured --json object); the guidance is
// human text and goes to stderr.
func printHostedMCPSetup(env *output.Envelope) error {
	// Only an explicit --json gets the wrapper object. A redirected stdout
	// (`dhq mcp > mcp.json`) must stay the bare mcpServers config, which is
	// what a client reads; WantsJSON() alone would auto-switch on any pipe.
	if env.JSONMode && env.WantsJSON() {
		return env.WriteJSON(map[string]interface{}{
			"url":         hostedMCPURL,
			"claude_code": claudeCodeMCPCommand(),
			"config":      json.RawMessage(MCPConfigSnippet()),
		})
	}
	env.Status("Hosted DeployHQ MCP server: %s", hostedMCPURL)
	env.Status("Read-only for now: look at accounts, projects, servers and deployments.")
	env.Status("Sign in to DeployHQ from your assistant; use the CLI to deploy or roll back.")
	env.Status("")
	env.Status("Claude Code:  %s", claudeCodeMCPCommand())
	env.Status("Claude Desktop: Settings > Connectors > Add custom connector, and enter the URL above.")
	env.Status("Cursor and other clients that read an mcpServers config: add this to your MCP config (printed on stdout):")
	env.Status("")
	_, _ = fmt.Fprintln(env.Stdout, MCPConfigSnippet())
	env.Status("")
	env.Status("Local stdio server (deprecated): dhq mcp --local")
	return nil
}

// stdinIsPiped reports whether stdin is not a terminal, i.e. an MCP client has
// launched us and expects a stdio server.
func stdinIsPiped() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}

func claudeCodeMCPCommand() string {
	return "claude mcp add --transport http deployhq " + hostedMCPURL
}

// runLocalMCP starts the local deployhq-mcp-server over stdio.
func runLocalMCP(args []string) error {
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
}

// mergeEnv returns env with each key in overrides set to its value, only when
// the key is not already present with a non-empty value. An exported-but-empty
// variable (e.g. `DEPLOYHQ_API_KEY=`) is treated as missing and dropped from
// the result — otherwise the MCP server would still crash at startup because
// empty required vars fail its validation just like absent ones, and a stale
// empty entry could shadow the override on platforms that take the first
// occurrence.
//
// User-set non-empty values are preserved, so power users can still point the
// server at a different account for one session.
func mergeEnv(env []string, overrides map[string]string) []string {
	out := make([]string, 0, len(env)+len(overrides))
	occupied := make(map[string]struct{}, len(env))
	for _, kv := range env {
		i := strings.Index(kv, "=")
		if i <= 0 {
			out = append(out, kv)
			continue
		}
		key, val := kv[:i], kv[i+1:]
		if val == "" {
			if _, willOverride := overrides[key]; willOverride {
				continue
			}
		}
		out = append(out, kv)
		occupied[key] = struct{}{}
	}
	for k, v := range overrides {
		if _, ok := occupied[k]; ok {
			continue
		}
		if v == "" {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
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

// MCPConfigSnippet returns an mcpServers JSON snippet for the hosted MCP server,
// for clients that read that config format (e.g. Cursor). Claude Desktop adds
// remote servers as account-level custom connectors instead.
func MCPConfigSnippet() string {
	return `{
  "mcpServers": {
    "deployhq": {
      "type": "http",
      "url": "` + hostedMCPURL + `"
    }
  }
}`
}
