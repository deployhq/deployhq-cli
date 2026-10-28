package sdk

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slowTestClient points a Client at a server that answers every request after
// delay, with the default and the extended per-request timeouts shortened so
// the test runs fast: default < delay < extended.
func slowTestClient(t *testing.T, delay time.Duration) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"identifier":"x","name":"x"}`))
	}))
	t.Cleanup(srv.Close)

	c, err := New("test", "user@example.com", "test-key", WithBaseURL(srv.URL))
	require.NoError(t, err)
	c.httpClient.Timeout = delay / 3
	c.slowHTTPClient.Timeout = delay * 4
	return c
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func TestNew_SlowCallsGetLongerTimeoutThanDefault(t *testing.T) {
	c, err := New("test", "user@example.com", "test-key")
	require.NoError(t, err)
	assert.Equal(t, defaultTimeout, c.httpClient.Timeout)
	assert.Equal(t, slowRequestTimeout, c.slowHTTPClient.Timeout)
	assert.Greater(t, slowRequestTimeout, defaultTimeout)
}

func TestSlowCalls_WaitPastDefaultTimeout(t *testing.T) {
	const delay = 150 * time.Millisecond
	ctx := context.Background()

	t.Run("CreateRepository", func(t *testing.T) {
		_, err := slowTestClient(t, delay).CreateRepository(ctx, "p", RepositoryCreateRequest{URL: "git@example.com:a/b.git"})
		assert.NoError(t, err)
	})
	t.Run("UpdateRepository", func(t *testing.T) {
		_, err := slowTestClient(t, delay).UpdateRepository(ctx, "p", RepositoryCreateRequest{Branch: "main"})
		assert.NoError(t, err)
	})
	t.Run("CreateServer", func(t *testing.T) {
		_, err := slowTestClient(t, delay).CreateServer(ctx, "p", ServerCreateRequest{Name: "web"})
		assert.NoError(t, err)
	})
}

func TestOtherCalls_KeepDefaultTimeout(t *testing.T) {
	_, err := slowTestClient(t, 150*time.Millisecond).GetRepository(context.Background(), "p")
	require.Error(t, err)
	assert.True(t, isTimeout(err), "expected a network timeout, got %v", err)
}

func TestSlowCalls_CallerDeadlineStillWins(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := slowTestClient(t, 150*time.Millisecond).CreateRepository(ctx, "p", RepositoryCreateRequest{URL: "git@example.com:a/b.git"})
	require.Error(t, err)
	assert.True(t, isTimeout(err), "expected a timeout, got %v", err)
}

func TestWithHTTPClient_CallerClientUsedForSlowCalls(t *testing.T) {
	hc := &http.Client{Timeout: 5 * time.Second}
	c, err := New("test", "user@example.com", "test-key", WithHTTPClient(hc))
	require.NoError(t, err)
	assert.Same(t, hc, c.httpClient)
	assert.Same(t, hc, c.slowHTTPClient, "a caller-supplied client stays in charge of every request")
}
