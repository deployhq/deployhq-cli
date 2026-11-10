package assist

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The rule about -s must not contradict the commands the prompt lists.
func TestSystemPrompt_ServerFlagRuleCoversDomains(t *testing.T) {
	var rule string
	for _, line := range strings.Split(systemPrompt, "\n") {
		if strings.HasPrefix(line, "- The -s flag") {
			rule = line
		}
	}
	assert.NotEmpty(t, rule)
	assert.NotContains(t, rule, "deploy only")
	assert.Contains(t, rule, "dhq domains")
}
