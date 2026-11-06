package cli

import (
	"testing"

	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The "No project specified" error is one of the first an agent meets in a new
// setup: its hint and JSON recovery point at DeployHQ's skill.
func TestRequireProject_PointsAgentsAtSkill(t *testing.T) {
	t.Setenv("DEPLOYHQ_AGENT", "claude-code")
	c := &Context{Config: &config.Config{}}
	_, err := c.RequireProject()
	require.Error(t, err)

	var uerr *output.UserError
	require.ErrorAs(t, err, &uerr)
	assert.Contains(t, uerr.Hint, "read DeployHQ's setup steps with 'dhq skills show'")
	assert.Contains(t, uerr.Hint, "dhq skills install --agent claude-code", "install is scoped to the running agent")
	assert.Contains(t, output.ErrorResponseFromErr(err).Data.(output.ErrorData).Recovery,
		output.Breadcrumb{Action: "read_setup_skill", Cmd: "dhq skills show"})
}
