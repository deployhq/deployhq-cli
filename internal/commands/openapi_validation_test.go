package commands

// openapi_validation_test.go — a request-validating http.RoundTripper for tests.
//
// newSpecValidatingClient wraps the usual httptest-backed SDK client with a
// transport that validates every outgoing request against the backend's
// OpenAPI document (testdata/openapi.json, a committed snapshot of the
// backend's /docs.json — refresh with script/update-openapi-fixture.sh).
//
// A request to an undocumented path, or with a body that violates the
// documented schema, fails the SDK call with an "openapi spec violation"
// error — so tests using this client prove the CLI speaks the backend's
// actual contract, not just whatever the hand-written test fake accepts.
// This is exactly the bug class behind the launch build-command P1 (the CLI
// POSTed to /build_configs, a path that does not exist in the API).

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

var (
	specRouterOnce sync.Once
	specRouter     routers.Router
	specRouterErr  error
)

// loadSpecRouter loads the committed OpenAPI fixture once per test binary and
// builds a route matcher from it.
func loadSpecRouter() (routers.Router, error) {
	specRouterOnce.Do(func() {
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromFile(filepath.Join("testdata", "openapi.json"))
		if err != nil {
			specRouterErr = fmt.Errorf("load OpenAPI fixture: %w", err)
			return
		}
		// Tests run against httptest hosts, not the documented server URLs —
		// match on path only.
		doc.Servers = nil
		specRouter, specRouterErr = gorillamux.NewRouter(doc)
	})
	return specRouter, specRouterErr
}

// specValidatingTransport validates each request against the OpenAPI document
// before forwarding it to the wrapped transport. Violations surface as
// transport errors, failing the SDK call loudly.
type specValidatingTransport struct {
	wrapped http.RoundTripper
	router  routers.Router
}

func (s *specValidatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Buffer the body: validation consumes it, and the real request still
	// needs to send it.
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		body = b
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	route, pathParams, err := s.router.FindRoute(req)
	if err != nil {
		return nil, fmt.Errorf("openapi spec violation: %s %s is not a documented endpoint: %w", req.Method, req.URL.Path, err)
	}
	if err := openapi3filter.ValidateRequest(req.Context(), &openapi3filter.RequestValidationInput{
		Request:    req,
		Route:      route,
		PathParams: pathParams,
		Options: &openapi3filter.Options{
			// Auth is enforced by the backend, not the schema check.
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}); err != nil {
		return nil, fmt.Errorf("openapi spec violation: %s %s: %w", req.Method, req.URL.Path, err)
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	return s.wrapped.RoundTrip(req)
}

// newSpecValidatingClient returns an SDK client wired to the given httptest
// server whose every request is validated against the OpenAPI fixture.
// Prefer this over newTestClient for tests that exercise real API flows.
func newSpecValidatingClient(t *testing.T, srv *httptest.Server) *sdk.Client {
	t.Helper()
	router, err := loadSpecRouter()
	require.NoError(t, err, "OpenAPI fixture must load (refresh with script/update-openapi-fixture.sh)")

	hc := srv.Client()
	base := hc.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	hc.Transport = &specValidatingTransport{wrapped: base, router: router}

	c, err := sdk.New("test", "u@e.com", "k",
		sdk.WithBaseURL(srv.URL),
		sdk.WithHTTPClient(hc),
	)
	require.NoError(t, err)
	return c
}

// ── Harness self-tests ────────────────────────────────────────────────────────

func TestSpecValidatingClient_RejectsUndocumentedEndpoint(t *testing.T) {
	// Recreates the original launch build-command P1: POSTing to
	// /build_configs (a path that does not exist in the API) must fail at the
	// validation layer — the backend is never reached.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request must be rejected before reaching the server: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newSpecValidatingClient(t, srv)
	err := client.Do(t.Context(), "POST", "/projects/my-app/build_configs",
		map[string]any{"build_config": map[string]any{"build_commands": "npm run build"}}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openapi spec violation")
	assert.Contains(t, err.Error(), "not a documented endpoint")
}

func TestSpecValidatingClient_RejectsSchemaViolation(t *testing.T) {
	// A documented path with a body missing its required key must also fail:
	// POST /projects/:id/servers requires a top-level `server` object.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request must be rejected before reaching the server: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newSpecValidatingClient(t, srv)
	err := client.Do(t.Context(), "POST", "/projects/my-app/servers",
		map[string]any{"name": "missing-server-wrapper"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openapi spec violation")
}

func TestSpecValidatingClient_AcceptsDocumentedRequest(t *testing.T) {
	// Sanity: a well-formed request passes validation and reaches the server.
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		_, _ = w.Write([]byte(`{"identifier":"bc-1","command":"npm run build"}`))
	}))
	defer srv.Close()

	client := newSpecValidatingClient(t, srv)
	_, err := client.CreateBuildCommand(t.Context(), "my-app", sdk.BuildCommandCreateRequest{
		Command:     "npm run build",
		Description: "npm run build",
	})
	require.NoError(t, err)
	assert.True(t, reached)
}
