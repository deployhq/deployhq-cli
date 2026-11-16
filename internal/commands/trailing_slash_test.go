package commands

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTrailingSlash(t *testing.T) {
	for _, in := range []string{"strip", "force", "preserve", " Force "} {
		got, err := parseTrailingSlash(in)
		require.NoError(t, err, in)
		assert.Contains(t, sdk.TrailingSlashModes, got)
	}
	_, err := parseTrailingSlash("add")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint, "force")
}

func TestServersCreate_TrailingSlashIsSentWithTheSite(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "create", "--name", "site", "--protocol-type", "static_hosting",
		"--subdomain", "my-site", "--trailing-slash", "FORCE", "--json"))
	attrs, ok := cap.body["hosted_website_attributes"].(map[string]any)
	require.True(t, ok, "body: %v", cap.body)
	assert.Equal(t, "force", attrs["trailing_slash"])
}

func TestServersCreate_TrailingSlashOmittedWhenNotGiven(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "create", "--name", "site", "--protocol-type", "static_hosting",
		"--subdomain", "my-site", "--json"))
	attrs := cap.body["hosted_website_attributes"].(map[string]any)
	assert.NotContains(t, attrs, "trailing_slash")
}

func TestServersCreate_TrailingSlashRefusals_NoHTTP(t *testing.T) {
	cases := map[string][]string{
		"not a mode":      {"--protocol-type", "static_hosting", "--subdomain", "my-site", "--trailing-slash", "yes"},
		"not static":      {"--protocol-type", "ssh", "--hostname", "h", "--username", "u", "--trailing-slash", "force"},
		"needs subdomain": {"--protocol-type", "static_hosting", "--trailing-slash", "force"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			withResolvableContext(t)
			blockNetwork(t)
			err := runServersCmd(t, append([]string{"servers", "create", "--name", "site"}, args...)...)
			var userErr *output.UserError
			require.True(t, errors.As(err, &userErr), "got %v", err)
			assert.Contains(t, userErr.Message, "--trailing-slash")
		})
	}
}

func TestServersUpdate_TrailingSlashIsHoisted(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "update", "srv-1", "--trailing-slash", "preserve", "--json"))
	assert.Equal(t, map[string]any{"trailing_slash": "preserve"}, cap.body["hosted_website_attributes"])
	server := cap.server(t)
	assert.NotContains(t, server, "trailing_slash")
}

func TestServersUpdate_NoTrailingSlashSendsNoSiteSettings(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	require.NoError(t, runServersCmd(t, "servers", "update", "srv-1", "--name", "renamed", "--json"))
	assert.NotContains(t, cap.body, "hosted_website_attributes")
}

func TestServersUpdate_BadTrailingSlash_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	blockNetwork(t)
	err := runServersCmd(t, "servers", "update", "srv-1", "--trailing-slash", "sometimes")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
}

func TestWarnIfTrailingSlashNotApplied(t *testing.T) {
	force, strip := "force", "strip"
	cases := []struct {
		name   string
		server *sdk.Server
		warn   string
	}{
		{"applied", &sdk.Server{ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{TrailingSlash: &force}}, ""},
		{"no site block", &sdk.Server{ProtocolType: "static_hosting"}, ""},
		{"other server type", &sdk.Server{Name: "prod", ProtocolType: "ssh"}, "only applies to Static Hosting"},
		{"different mode", &sdk.Server{Identifier: "srv-1", ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{TrailingSlash: &strip}}, "did not confirm"},
		{"not reported", &sdk.Server{ProtocolType: "static_hosting", StaticHosting: &sdk.StaticHostingInfo{}}, "did not confirm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			env := &output.Envelope{Stdout: &stdout, Stderr: &stderr}
			warnIfTrailingSlashNotApplied(env, "force", tc.server)
			if tc.warn == "" {
				assert.Empty(t, stderr.String())
			} else {
				assert.Contains(t, stderr.String(), tc.warn)
			}
		})
	}
}
