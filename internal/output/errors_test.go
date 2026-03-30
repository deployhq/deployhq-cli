package output

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserError(t *testing.T) {
	err := &UserError{Message: "bad input", Hint: "use --flag"}
	assert.Contains(t, err.Error(), "bad input")
	assert.Contains(t, err.Error(), "use --flag")
	assert.Equal(t, ExitUserError, ClassifyError(err))
}

func TestUserError_NoHint(t *testing.T) {
	err := &UserError{Message: "bad input"}
	assert.Equal(t, "bad input", err.Error())
	assert.NotContains(t, err.Error(), "Hint")
}

func TestInternalError(t *testing.T) {
	cause := errors.New("disk full")
	err := &InternalError{Message: "write failed", Cause: cause}
	assert.Contains(t, err.Error(), "write failed")
	assert.Contains(t, err.Error(), "disk full")
	assert.Equal(t, ExitInternalError, ClassifyError(err))
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestAuthError(t *testing.T) {
	err := &AuthError{Message: "not logged in", Hint: "run deployhq auth login"}
	assert.Contains(t, err.Error(), "not logged in")
	assert.Equal(t, ExitAuthError, ClassifyError(err))
}

func TestClassifyError_Nil(t *testing.T) {
	assert.Equal(t, ExitOK, ClassifyError(nil))
}

func TestClassifyError_GenericError(t *testing.T) {
	assert.Equal(t, ExitInternalError, ClassifyError(errors.New("unknown")))
}
