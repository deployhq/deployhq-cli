package commands

import (
	"errors"
	"net/http"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	serversListPath   = "GET /projects/my-app/servers"
	serversUpdateJSON = `{"identifier":"srv-static","name":"site","protocol_type":"static_hosting"}`
	serversNotFound   = `{"error":"Not found"}`
)

// A server name 404s as an identifier, resolves through the listing, and the
// update is sent again to the resolved identifier.
func TestServersUpdate_ResolvesAServerName(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"PUT /projects/my-app/servers/site":       {http.StatusNotFound, serversNotFound},
		serversListPath:                           {http.StatusOK, `[{"identifier":"srv-static","name":"site"},{"identifier":"srv-2","name":"prod"}]`},
		"PUT /projects/my-app/servers/srv-static": {http.StatusOK, serversUpdateJSON},
	})

	require.NoError(t, runServersCmd(t, "servers", "update", "site", "--name", "renamed"))

	require.Len(t, *calls, 3)
	last := (*calls)[2]
	assert.Equal(t, "/projects/my-app/servers/srv-static", last.path)
	srv, ok := last.body["server"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "renamed", srv["name"])
}

// An identifier that works is sent once, with no listing.
func TestServersUpdate_IdentifierNeedsNoListing(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"PUT /projects/my-app/servers/srv-static": {http.StatusOK, serversUpdateJSON},
	})

	require.NoError(t, runServersCmd(t, "servers", "update", "srv-static", "--name", "renamed"))
	assert.Len(t, *calls, 1)
}

// A canonical UUID that 404s is an unknown identifier, not a name.
func TestServersUpdate_UnknownUUIDIsNotLookedUpByName(t *testing.T) {
	withResolvableContext(t)
	id := "0b6a1e2c-3d4f-4a5b-8c6d-7e8f9a0b1c2d"
	calls := domainsAPI(t, map[string]cannedResponse{
		"PUT /projects/my-app/servers/" + id: {http.StatusNotFound, serversNotFound},
	})

	err := runServersCmd(t, "servers", "update", id, "--name", "renamed")
	require.Error(t, err)
	assert.Len(t, *calls, 1)
}

// Two servers with the same name are ambiguous: nothing is updated.
func TestServersUpdate_SharedNameIsAmbiguous(t *testing.T) {
	withResolvableContext(t)
	calls := domainsAPI(t, map[string]cannedResponse{
		"PUT /projects/my-app/servers/site": {http.StatusNotFound, serversNotFound},
		serversListPath:                     {http.StatusOK, `[{"identifier":"srv-a","name":"site"},{"identifier":"srv-b","name":"site"}]`},
	})

	err := runServersCmd(t, "servers", "update", "site", "--name", "renamed")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Contains(t, userErr.Hint+userErr.Message, "srv-a")
	assert.Contains(t, userErr.Hint+userErr.Message, "srv-b")
	assert.Len(t, *calls, 2, "no update may be sent for an ambiguous name")
}

// An unknown name fails with the project's servers.
func TestServersUpdate_UnknownNameListsTheServers(t *testing.T) {
	withResolvableContext(t)
	domainsAPI(t, map[string]cannedResponse{
		"PUT /projects/my-app/servers/nope": {http.StatusNotFound, serversNotFound},
		serversListPath:                     {http.StatusOK, `[{"identifier":"srv-static","name":"site"}]`},
	})

	err := runServersCmd(t, "servers", "update", "nope", "--name", "renamed")
	var userErr *output.UserError
	require.True(t, errors.As(err, &userErr), "got %v", err)
	assert.Equal(t, `No unique server matches "nope"`, userErr.Message)
	assert.Equal(t, "Servers: site (srv-static)", userErr.Hint)
}
