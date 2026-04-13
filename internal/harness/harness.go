// Package harness detects AI agent contexts and threads agent identity
// into the CLI's User-Agent header and output behavior.
package harness

import (
	"os"
	"strings"
)

// AgentInfo describes the detected agent environment.
type AgentInfo struct {
	Detected bool   `json:"detected"`
	Name     string `json:"name,omitempty"`
	Source   string `json:"source,omitempty"` // how we detected it
}

// agentProbe defines a single environment-based agent detection rule.
type agentProbe struct {
	name   string   // canonical agent name
	source string   // detection source label
	envs   []string // env vars to check (any non-empty match wins)
}

// agentProbes is the ordered list of agent detection rules.
// First match wins after the explicit DEPLOYHQ_AGENT override.
var agentProbes = []agentProbe{
	// Claude Code (CLAUDECODE=1 is set in subprocesses)
	{name: "claude-code", source: "CLAUDECODE", envs: []string{"CLAUDECODE", "CLAUDE_CODE", "CLAUDE"}},
	// OpenAI Codex CLI
	{name: "codex", source: "CODEX", envs: []string{"CODEX"}},
	// Cursor (sets CURSOR_TRACE_ID in its integrated terminal)
	{name: "cursor", source: "CURSOR_TRACE_ID", envs: []string{"CURSOR_TRACE_ID"}},
	// Windsurf / Codeium (sets WINDSURF or CODEIUM_* vars)
	{name: "windsurf", source: "WINDSURF", envs: []string{"WINDSURF", "CODEIUM_ENV"}},
	// Cline (VS Code extension, sets CLINE=1)
	{name: "cline", source: "CLINE", envs: []string{"CLINE"}},
	// Continue (VS Code/JetBrains extension)
	{name: "continue", source: "CONTINUE", envs: []string{"CONTINUE_GLOBAL_DIR"}},
	// Aider
	{name: "aider", source: "AIDER", envs: []string{"AIDER"}},
	// Warp terminal AI
	{name: "warp", source: "WARP_IS_LOCAL_SHELL_SESSION", envs: []string{"WARP_IS_LOCAL_SHELL_SESSION"}},
	// GitHub Copilot CLI
	{name: "copilot-cli", source: "GITHUB_COPILOT", envs: []string{"GITHUB_COPILOT_CLI"}},
}

// Detect checks the environment for known AI agent signals.
// Detection hierarchy:
//  1. DEPLOYHQ_AGENT env var (explicit, highest priority)
//  2. Known agent env vars (Claude Code, Codex, Cursor, Windsurf, etc.)
//  3. TERM_PROGRAM heuristic (Cursor, Windsurf)
//  4. CI env var patterns (GitHub Actions, etc.)
func Detect() AgentInfo {
	// Explicit agent declaration (highest priority)
	if agent := os.Getenv("DEPLOYHQ_AGENT"); agent != "" {
		return AgentInfo{Detected: true, Name: agent, Source: "DEPLOYHQ_AGENT"}
	}

	// Check each known agent probe
	for _, p := range agentProbes {
		for _, env := range p.envs {
			if os.Getenv(env) != "" {
				return AgentInfo{Detected: true, Name: p.name, Source: p.source}
			}
		}
	}

	// TERM_PROGRAM heuristic — some editors set this in their integrated terminal
	if tp := strings.ToLower(os.Getenv("TERM_PROGRAM")); tp != "" {
		switch {
		case strings.Contains(tp, "cursor"):
			return AgentInfo{Detected: true, Name: "cursor", Source: "TERM_PROGRAM"}
		case strings.Contains(tp, "windsurf"):
			return AgentInfo{Detected: true, Name: "windsurf", Source: "TERM_PROGRAM"}
		}
	}

	// Generic CI/agent detection
	if isCI() {
		return AgentInfo{Detected: true, Name: "ci", Source: "CI"}
	}

	return AgentInfo{}
}

// UserAgent returns a User-Agent string incorporating agent info.
func UserAgent(version string, agent AgentInfo) string {
	ua := "DeployHQ-CLI/" + version
	if agent.Detected {
		ua += " (agent:" + agent.Name + ")"
	}
	return ua
}

func isCI() bool {
	ciVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "JENKINS_URL", "BUILDKITE"}
	for _, v := range ciVars {
		if val := os.Getenv(v); val != "" && !strings.EqualFold(val, "false") {
			return true
		}
	}
	return false
}
