package harness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// clearAllAgentEnvs unsets every env var that Detect() checks so tests
// don't leak state between each other.
func clearAllAgentEnvs(t *testing.T) {
	t.Helper()
	for _, env := range []string{
		"DEPLOYHQ_AGENT",
		// agent probes
		"CLAUDECODE", "CLAUDE_CODE", "CLAUDE",
		"CODEX",
		"CURSOR_TRACE_ID",
		"WINDSURF", "CODEIUM_ENV",
		"CLINE",
		"CONTINUE_GLOBAL_DIR",
		"AIDER",
		"WARP_IS_LOCAL_SHELL_SESSION",
		"GITHUB_COPILOT_CLI",
		// TERM_PROGRAM heuristic
		"TERM_PROGRAM",
		// CI vars
		"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "JENKINS_URL", "BUILDKITE",
	} {
		t.Setenv(env, "")
	}
}

func TestDetect_DeployHQAgent(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("DEPLOYHQ_AGENT", "my-bot")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "my-bot", info.Name)
	assert.Equal(t, "DEPLOYHQ_AGENT", info.Source)
}

func TestDetect_ClaudeCode(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("CLAUDECODE", "1")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "claude-code", info.Name)
}

func TestDetect_ClaudeCode_LegacyEnv(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("CLAUDE_CODE", "1")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "claude-code", info.Name)
}

func TestDetect_Cursor(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("CURSOR_TRACE_ID", "abc123")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "cursor", info.Name)
	assert.Equal(t, "CURSOR_TRACE_ID", info.Source)
}

func TestDetect_Cursor_TermProgram(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("TERM_PROGRAM", "cursor")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "cursor", info.Name)
	assert.Equal(t, "TERM_PROGRAM", info.Source)
}

func TestDetect_Windsurf(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("WINDSURF", "1")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "windsurf", info.Name)
	assert.Equal(t, "WINDSURF", info.Source)
}

func TestDetect_Windsurf_TermProgram(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("TERM_PROGRAM", "windsurf")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "windsurf", info.Name)
	assert.Equal(t, "TERM_PROGRAM", info.Source)
}

func TestDetect_Cline(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("CLINE", "1")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "cline", info.Name)
}

func TestDetect_Continue(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("CONTINUE_GLOBAL_DIR", "/home/user/.continue")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "continue", info.Name)
}

func TestDetect_Aider(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("AIDER", "1")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "aider", info.Name)
}

func TestDetect_Warp(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("WARP_IS_LOCAL_SHELL_SESSION", "1")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "warp", info.Name)
}

func TestDetect_CopilotCLI(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("GITHUB_COPILOT_CLI", "1")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "copilot-cli", info.Name)
}

func TestDetect_CI(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("CI", "true")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "ci", info.Name)
}

func TestDetect_None(t *testing.T) {
	clearAllAgentEnvs(t)

	info := Detect()
	assert.False(t, info.Detected)
	assert.Empty(t, info.Name)
}

func TestDetect_Priority_ExplicitOverridesProbe(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("DEPLOYHQ_AGENT", "my-bot")
	t.Setenv("CLAUDECODE", "1")

	info := Detect()
	assert.Equal(t, "my-bot", info.Name)
	assert.Equal(t, "DEPLOYHQ_AGENT", info.Source)
}

func TestDetect_Priority_ProbeOverridesTermProgram(t *testing.T) {
	clearAllAgentEnvs(t)
	t.Setenv("CURSOR_TRACE_ID", "abc")
	t.Setenv("TERM_PROGRAM", "windsurf")

	info := Detect()
	assert.Equal(t, "cursor", info.Name)
	assert.Equal(t, "CURSOR_TRACE_ID", info.Source)
}

func TestUserAgent(t *testing.T) {
	assert.Equal(t, "DeployHQ-CLI/1.0.0", UserAgent("1.0.0", AgentInfo{}))
	assert.Equal(t, "DeployHQ-CLI/1.0.0 (agent:claude-code)", UserAgent("1.0.0", AgentInfo{Detected: true, Name: "claude-code"}))
}
