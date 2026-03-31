package harness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetect_DeployHQAgent(t *testing.T) {
	t.Setenv("DEPLOYHQ_AGENT", "my-bot")
	t.Setenv("CLAUDE_CODE", "")
	t.Setenv("CI", "")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "my-bot", info.Name)
	assert.Equal(t, "DEPLOYHQ_AGENT", info.Source)
}

func TestDetect_ClaudeCode(t *testing.T) {
	t.Setenv("DEPLOYHQ_AGENT", "")
	t.Setenv("CLAUDE_CODE", "1")
	t.Setenv("CI", "")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "claude-code", info.Name)
}

func TestDetect_CI(t *testing.T) {
	t.Setenv("DEPLOYHQ_AGENT", "")
	t.Setenv("CLAUDE_CODE", "")
	t.Setenv("CLAUDE", "")
	t.Setenv("CODEX", "")
	t.Setenv("CI", "true")

	info := Detect()
	assert.True(t, info.Detected)
	assert.Equal(t, "ci", info.Name)
}

func TestDetect_None(t *testing.T) {
	t.Setenv("DEPLOYHQ_AGENT", "")
	t.Setenv("CLAUDE_CODE", "")
	t.Setenv("CLAUDE", "")
	t.Setenv("CODEX", "")
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("CIRCLECI", "")
	t.Setenv("JENKINS_URL", "")
	t.Setenv("BUILDKITE", "")

	info := Detect()
	assert.False(t, info.Detected)
	assert.Empty(t, info.Name)
}

func TestUserAgent(t *testing.T) {
	assert.Equal(t, "dhq/1.0.0", UserAgent("1.0.0", AgentInfo{}))
	assert.Equal(t, "dhq/1.0.0 (agent:claude-code)", UserAgent("1.0.0", AgentInfo{Detected: true, Name: "claude-code"}))
}
