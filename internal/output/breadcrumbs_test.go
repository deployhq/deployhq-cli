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
		Breadcrumb{Action: "logs", Cmd: "dhq deployments logs 123"},
		Breadcrumb{Action: "abort", Cmd: "dhq deployments abort 123"},
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

func TestNewPaginatedResponse(t *testing.T) {
	resp := NewPaginatedResponse(
		[]string{"a", "b"},
		Pagination{CurrentPage: 2, TotalPages: 5, TotalRecords: 50, Offset: 10},
		"test summary",
	)
	assert.True(t, resp.OK)
	assert.NotNil(t, resp.Pagination)
	assert.Equal(t, 2, resp.Pagination.CurrentPage)
	assert.Equal(t, 5, resp.Pagination.TotalPages)
	assert.Equal(t, 50, resp.Pagination.TotalRecords)
	assert.Equal(t, 10, resp.Pagination.Offset)
	assert.Equal(t, "test summary", resp.Summary)
}

func TestNewResponse_NoPagination(t *testing.T) {
	resp := NewResponse([]string{"a"}, "test")
	assert.Nil(t, resp.Pagination)

	b, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.NotContains(t, string(b), "pagination")
}

func TestNewPaginatedResponse_IncludesPaginationInJSON(t *testing.T) {
	resp := NewPaginatedResponse(
		[]string{"a"},
		Pagination{CurrentPage: 1, TotalPages: 3, TotalRecords: 30, Offset: 0},
		"test",
	)
	b, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.Contains(t, string(b), `"pagination"`)
	assert.Contains(t, string(b), `"current_page":1`)
	assert.Contains(t, string(b), `"total_pages":3`)
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse("timeout", "Deploy timed out", "Increase --wait-timeout", "https://docs.example.com")

	assert.False(t, resp.OK)
	data := resp.Data.(map[string]string)
	assert.Equal(t, "timeout", data["code"])
	assert.Equal(t, "Deploy timed out", data["error"])
}
