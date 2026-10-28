package commands

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Links to account pages point at the account's own host, where those pages
// live, and never at app.deployhq.com.

func TestBetaEnroll_NonAdminLinksToAccountBetaPage(t *testing.T) {
	withResolvableContext(t)
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(`{"error":"admin_required"}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })

	_, err := runBuildConfigsCmd(t, "beta", "enroll")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https://testco.deployhq.com/beta_features")
	assert.NotContains(t, err.Error(), "app.deployhq.com")
}

func TestLaunchBetaEnroll_NonAdminLinksToAccountBetaPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"admin_required"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	env, _, _ := testLaunchEnvelope()
	err := launchEnsureBetaEnrolled(t.Context(), env, launchConfig{targetProtocol: "static_hosting"}, newTestClient(t, srv), "test")
	var le *launchError
	require.True(t, isLaunchErr(err, &le))
	assert.Equal(t, srv.URL+"/beta_features", le.Details["beta_url"])
	assert.Contains(t, le.NextStep, srv.URL+"/beta_features")
	assert.NotContains(t, le.NextStep, "app.deployhq.com")
}

func TestLaunchBetaEnroll_EndpointMissingLinksToAccountBetaPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	env, _, stderr := testLaunchEnvelope()
	err := launchEnsureBetaEnrolled(t.Context(), env, launchConfig{targetProtocol: "static_hosting"}, newTestClient(t, srv), "test")
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), srv.URL+"/beta_features")
	assert.NotContains(t, stderr.String(), "app.deployhq.com")
}

func TestLaunchGetCaps_UnavailableLinksToAccountBetaPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	env, _, stderr := testLaunchEnvelope()
	_, capsKnown, err := launchGetCaps(t.Context(), env, newTestClient(t, srv))
	require.NoError(t, err)
	assert.False(t, capsKnown)
	assert.Contains(t, stderr.String(), srv.URL+"/beta_features")
	assert.NotContains(t, stderr.String(), "app.deployhq.com")
}

func TestLaunchCheckPlanLimits_LinksToAccountPages(t *testing.T) {
	env, _, _ := testLaunchEnvelope()
	cases := []struct {
		protocol string
		caps     sdk.AccountCapabilities
		wantURL  string
	}{
		{"static_hosting", sdk.AccountCapabilities{ManagedVPSEligible: true}, "https://acme.deployhq.com/account/packages"},
		{"managed_vps", sdk.AccountCapabilities{StaticHostingEligible: true}, "https://acme.deployhq.com/account/payment_details"},
	}
	for _, tc := range cases {
		t.Run(tc.protocol, func(t *testing.T) {
			err := launchCheckPlanLimits(env, launchConfig{targetProtocol: tc.protocol}, &tc.caps, "acme")
			var le *launchError
			require.True(t, isLaunchErr(err, &le))
			assert.Contains(t, le.NextStep, tc.wantURL)
			assert.NotContains(t, le.NextStep, "app.deployhq.com")
		})
	}
}
