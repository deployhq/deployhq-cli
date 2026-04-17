package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeployDryRunFlag(t *testing.T) {
	cmd := NewRootCmd("test")
	deployCmd, _, _ := cmd.Find([]string{"deploy"})
	assert.NotNil(t, deployCmd)
	assert.NotNil(t, deployCmd.Flags().Lookup("dry-run"))
}

func TestDeployDryRunMutuallyExclusiveWithWait(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"deploy", "--dry-run", "--wait", "-p", "test-project"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}
