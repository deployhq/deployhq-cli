package commands

import (
	"testing"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostedResourcesCmd_Structure(t *testing.T) {
	cmd := newHostedResourcesCmd()
	assert.Equal(t, "hosted-resources", cmd.Name())

	want := map[string]bool{
		"list":            false,
		"show":            false,
		"sync":            false,
		"retry-provision": false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for name, found := range want {
		assert.Truef(t, found, "missing subcommand %q", name)
	}
}

func TestHostedResourcesCmd_Help(t *testing.T) {
	cmd := newHostedResourcesCmd()
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())
}

func TestHostedResourceLocation(t *testing.T) {
	region := "lon1"
	vps := sdk.HostedResource{Kind: "hosted_resource", Region: &region}
	assert.Equal(t, "lon1", hostedResourceLocation(vps))

	website := sdk.HostedResource{Kind: "hosted_website", Subdomain: "my-site"}
	assert.Equal(t, "my-site", hostedResourceLocation(website))

	// Managed VPS with no region yet → empty, no nil-deref.
	pending := sdk.HostedResource{Kind: "hosted_resource"}
	assert.Equal(t, "", hostedResourceLocation(pending))
}
