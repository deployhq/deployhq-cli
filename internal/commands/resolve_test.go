package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResource struct {
	id         int
	identifier string
}

func (f fakeResource) NumericID() int { return f.id }
func (f fakeResource) UUID() string   { return f.identifier }

func TestResolveID_UUID(t *testing.T) {
	list := []fakeResource{{id: 429, identifier: "abc-123"}}
	got, err := resolveID("abc-123", list)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", got)
}

func TestResolveID_NumericMatch(t *testing.T) {
	list := []fakeResource{
		{id: 100, identifier: "first-uuid"},
		{id: 429, identifier: "target-uuid"},
	}
	got, err := resolveID("429", list)
	require.NoError(t, err)
	assert.Equal(t, "target-uuid", got)
}

func TestResolveID_NumericNotFound(t *testing.T) {
	list := []fakeResource{{id: 100, identifier: "first-uuid"}}
	_, err := resolveID("999", list)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "999")
}
