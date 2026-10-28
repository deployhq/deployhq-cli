package output

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// A reported error keeps the exit code of the error it wraps.
func TestReportedError_KeepsExitCode(t *testing.T) {
	cases := []error{
		&UserError{Message: "bad"},
		&AuthError{Message: "no"},
		&sdk.APIError{StatusCode: http.StatusNotFound},
		&sdk.APIError{StatusCode: http.StatusUnprocessableEntity},
		errors.New("plain"),
	}
	for _, err := range cases {
		reported := &ReportedError{Err: err}
		assert.Equal(t, ClassifyError(err), ClassifyError(reported), "%T", err)
		assert.Equal(t, ClassifyError(err), ClassifyError(fmt.Errorf("wrapped: %w", reported)), "%T wrapped", err)
		assert.Equal(t, err.Error(), reported.Error())
		assert.ErrorIs(t, reported, err)
	}
}

func TestIsReported(t *testing.T) {
	assert.True(t, IsReported(&ReportedError{Err: errors.New("x")}))
	assert.True(t, IsReported(fmt.Errorf("ctx: %w", &ReportedError{Err: errors.New("x")})))
	assert.False(t, IsReported(&UserError{Message: "x"}))
	assert.False(t, IsReported(nil))
}
