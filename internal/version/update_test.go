package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheck_DevVersion(t *testing.T) {
	info := Check("dev")
	assert.Equal(t, "dev", info.Current)
	assert.False(t, info.UpdateAvailable)
	assert.Empty(t, info.Latest)
}

func TestFormatUpdateMessage_NoUpdate(t *testing.T) {
	info := UpdateInfo{Current: "1.0.0", Latest: "1.0.0", UpdateAvailable: false}
	assert.Empty(t, FormatUpdateMessage(info))
}

func TestFormatUpdateMessage_UpdateAvailable(t *testing.T) {
	info := UpdateInfo{
		Current: "1.0.0", Latest: "1.1.0",
		UpdateAvailable: true,
		URL: "https://github.com/deployhq/deployhq-cli/releases/tag/v1.1.0",
	}
	msg := FormatUpdateMessage(info)
	assert.Contains(t, msg, "1.0.0")
	assert.Contains(t, msg, "1.1.0")
	assert.Contains(t, msg, "https://")
}

func TestFormatUpdateMessage_EmptyLatest(t *testing.T) {
	info := UpdateInfo{Current: "1.0.0", UpdateAvailable: true}
	assert.Empty(t, FormatUpdateMessage(info))
}
