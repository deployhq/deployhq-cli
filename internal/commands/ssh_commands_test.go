package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSSHTiming(t *testing.T) {
	t.Run("empty is allowed (omitempty on update)", func(t *testing.T) {
		require.NoError(t, validateSSHTiming(""))
	})

	for _, v := range []string{"all", "first", "after_first"} {
		v := v
		t.Run("valid: "+v, func(t *testing.T) {
			require.NoError(t, validateSSHTiming(v))
		})
	}

	for _, v := range []string{"before", "after", "pre", "post", "ALL", "After_First"} {
		v := v
		t.Run("invalid: "+v, func(t *testing.T) {
			err := validateSSHTiming(v)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Invalid --timing")
		})
	}
}
