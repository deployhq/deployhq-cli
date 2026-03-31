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

// Detect checks the environment for known AI agent signals.
// Detection hierarchy:
//  1. DEPLOYHQ_AGENT env var (explicit, highest priority)
//  2. CLAUDE_CODE env var (Claude Code)
//  3. CODEX env var (OpenAI Codex)
//  4. CI env var patterns (GitHub Actions, etc.)
func Detect() AgentInfo {
	// Explicit agent declaration
	if agent := os.Getenv("DEPLOYHQ_AGENT"); agent != "" {
		return AgentInfo{Detected: true, Name: agent, Source: "DEPLOYHQ_AGENT"}
	}

	// Claude Code
	if os.Getenv("CLAUDE_CODE") != "" || os.Getenv("CLAUDE") != "" {
		return AgentInfo{Detected: true, Name: "claude-code", Source: "CLAUDE_CODE"}
	}

	// OpenAI Codex
	if os.Getenv("CODEX") != "" {
		return AgentInfo{Detected: true, Name: "codex", Source: "CODEX"}
	}

	// Generic CI/agent detection
	if isCI() {
		return AgentInfo{Detected: true, Name: "ci", Source: "CI"}
	}

	return AgentInfo{}
}

// UserAgent returns a User-Agent string incorporating agent info.
func UserAgent(version string, agent AgentInfo) string {
	ua := "dhq/" + version
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
