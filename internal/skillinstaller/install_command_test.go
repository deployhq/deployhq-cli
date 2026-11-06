package skillinstaller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The command names one agent: a bare 'dhq skills install' would install for
// every AI tool found on the machine.
func TestInstallCommandFor(t *testing.T) {
	for name, want := range map[string]string{
		"claude-code": "dhq skills install --agent claude-code",
		"cursor":      "dhq skills install --agent cursor",
		"copilot":     "dhq skills install --agent <your agent>", // project scope: writes into the repo
		"":            "dhq skills install --agent <your agent>",
		"vim":         "dhq skills install --agent <your agent>",
	} {
		assert.Equal(t, want, InstallCommandFor(name), name)
	}
}
