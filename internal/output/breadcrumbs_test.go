package output

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResponse(t *testing.T) {
	resp := NewResponse(
		map[string]string{"id": "123"},
		"Deployment created",
		Breadcrumb{Action: "logs", Cmd: "deployhq deployments logs 123"},
		Breadcrumb{Action: "abort", Cmd: "deployhq deployments abort 123"},
	)

	assert.True(t, resp.OK)
	assert.Equal(t, "Deployment created", resp.Summary)
	assert.Len(t, resp.Breadcrumbs, 2)
	assert.Equal(t, "logs", resp.Breadcrumbs[0].Action)
}

func TestResponse_JSONShape(t *testing.T) {
	resp := NewResponse([]string{"a", "b"}, "2 items")

	b, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &raw))

	assert.True(t, raw["ok"].(bool))
	assert.Equal(t, "2 items", raw["summary"])
	assert.NotNil(t, raw["data"])
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse("timeout", "Deploy timed out", "Increase --wait-timeout", "https://docs.example.com")

	assert.False(t, resp.OK)
	data := resp.Data.(map[string]string)
	assert.Equal(t, "timeout", data["code"])
	assert.Equal(t, "Deploy timed out", data["error"])
}
