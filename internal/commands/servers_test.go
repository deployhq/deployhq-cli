package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// blockNetwork installs a tripwire transport for the duration of the test. The
// SDK builds its http.Client with a nil Transport, so every request it makes
// goes through http.DefaultTransport — swapping that turns "did this command
// touch the network?" into an assertable fact rather than an assumption.
func blockNetwork(t *testing.T) *bool {
	t.Helper()
	called := false
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		t.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
		return nil, fmt.Errorf("network blocked in test")
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
	return &called
}

// withResolvableContext supplies credentials and a project through the env-var
// config layer so RequireProject() and Client() would both SUCCEED. That is
// what makes the "no HTTP call" assertions meaningful: the only thing standing
// between the command and the wire is the local flag validation.
func withResolvableContext(t *testing.T) {
	t.Helper()
	t.Setenv("DEPLOYHQ_ACCOUNT", "testco")
	t.Setenv("DEPLOYHQ_EMAIL", "user@example.com")
	t.Setenv("DEPLOYHQ_API_KEY", "test-key")
	t.Setenv("DEPLOYHQ_PROJECT", "my-app")
	t.Setenv("DEPLOYHQ_NO_TELEMETRY", "1")

	origCtx := cliCtx
	t.Cleanup(func() { cliCtx = origCtx })
}

// capturedRequest records what a command actually put on the wire.
type capturedRequest struct {
	mu     sync.Mutex
	count  int
	method string
	path   string
	body   map[string]any
}

// server returns the decoded `server` object from the captured body.
func (c *capturedRequest) server(t *testing.T) map[string]any {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	require.Equal(t, 1, c.count, "expected exactly one DeployHQ API request")
	srv, ok := c.body["server"].(map[string]any)
	require.True(t, ok, "request body must wrap a server object, got: %v", c.body)
	return srv
}

// captureRequest is blockNetwork's inverse: instead of failing the test, it
// records the outgoing request and answers with a canned server so the command
// completes its success path. This is what exercises the command → request
// seam end to end — the helper-level tests below construct requests directly
// and so cannot catch a command that never calls applyTo{Create,Update}.
func captureRequest(t *testing.T) *capturedRequest {
	t.Helper()
	cap := &capturedRequest{}
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		cap.mu.Lock()
		defer cap.mu.Unlock()
		// Only the DeployHQ API counts. The root command also fires an update
		// check against api.github.com after the command completes; that is
		// pre-existing behaviour unrelated to what these tests assert.
		if !strings.Contains(r.URL.Path, "/projects/") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Request:    r,
			}, nil
		}
		cap.count++
		cap.method = r.Method
		cap.path = r.URL.Path
		if r.Body != nil {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			// Decode leniently: a malformed body should surface as a failed
			// assertion on the contents, not as a transport error.
			_ = json.Unmarshal(raw, &cap.body)
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(`{"identifier":"srv-1","name":"staging","protocol_type":"ssh"}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    r,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
	return cap
}

// runServersCmd executes the real root command with args, discarding output.
func runServersCmd(t *testing.T, args ...string) error {
	t.Helper()
	cmd := NewRootCmd("test")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd.Execute()
}

// parseDeploymentFlags registers the shared deployment flags on a throwaway
// command and parses args into them — exercising the helper in isolation.
func parseDeploymentFlags(t *testing.T, args ...string) *serverDeploymentFlags {
	t.Helper()
	f := &serverDeploymentFlags{}
	cmd := &cobra.Command{Use: "fake", RunE: func(*cobra.Command, []string) error { return nil }}
	f.register(cmd)
	require.NoError(t, cmd.Flags().Parse(args))
	return f
}

var deploymentFlagNames = []string{
	"branch", "auto-deploy", "atomic", "atomic-strategy", "atomic-retention",
}

// ── flag registration ─────────────────────────────────────────────────────────

func TestServersCreate_RegistersDeploymentFlags(t *testing.T) {
	root := NewRootCmd("test")
	createCmd, _, err := root.Find([]string{"servers", "create"})
	require.NoError(t, err)
	require.Equal(t, "create", createCmd.Name())

	for _, name := range deploymentFlagNames {
		assert.NotNil(t, createCmd.Flags().Lookup(name),
			"--%s must be registered on servers create", name)
	}
}

func TestServersUpdate_RegistersDeploymentFlags(t *testing.T) {
	root := NewRootCmd("test")
	updateCmd, _, err := root.Find([]string{"servers", "update"})
	require.NoError(t, err)
	require.Equal(t, "update", updateCmd.Name())

	for _, name := range deploymentFlagNames {
		assert.NotNil(t, updateCmd.Flags().Lookup(name),
			"--%s must be registered on servers update", name)
	}
}

// The booleans must be usable as bare switches (--atomic) as well as with an
// explicit value (--atomic=false); pflag expresses that via NoOptDefVal.
func TestServersDeploymentFlags_BooleansAcceptBareForm(t *testing.T) {
	root := NewRootCmd("test")
	createCmd, _, err := root.Find([]string{"servers", "create"})
	require.NoError(t, err)

	for _, name := range []string{"auto-deploy", "atomic"} {
		f := createCmd.Flags().Lookup(name)
		require.NotNil(t, f)
		assert.Equal(t, "true", f.NoOptDefVal, "--%s must work as a bare switch", name)
	}
}

// The help text has to carry the constraints an operator cannot discover from
// the flag name alone.
func TestServersCreate_DeploymentFlagHelpMentionsConstraints(t *testing.T) {
	root := NewRootCmd("test")
	createCmd, _, err := root.Find([]string{"servers", "create"})
	require.NoError(t, err)

	atomic := createCmd.Flags().Lookup("atomic").Usage
	assert.Contains(t, atomic, "first deployment",
		"--atomic help must warn that atomic is locked after the first deployment")
	for _, proto := range []string{"ssh", "rsync", "digitalocean", "hetzner_cloud", "managed_vps"} {
		assert.Contains(t, atomic, proto, "--atomic help must list the %s protocol", proto)
	}
	assert.Contains(t, atomic, "atomic deployments enabled",
		"--atomic help must mention the account-level requirement")

	strategy := createCmd.Flags().Lookup("atomic-strategy").Usage
	assert.Contains(t, strategy, "copy_release")
	assert.Contains(t, strategy, "copy_cache")

	retention := createCmd.Flags().Lookup("atomic-retention").Usage
	assert.Contains(t, retention, "1")
}

func TestServersCreate_ExampleShowsAtomicManagedVPS(t *testing.T) {
	root := NewRootCmd("test")
	createCmd, _, err := root.Find([]string{"servers", "create"})
	require.NoError(t, err)

	assert.Contains(t, createCmd.Example, "--branch")
	assert.Contains(t, createCmd.Example, "--atomic")
	assert.Contains(t, createCmd.Example, "managed_vps")
}

// ── local validation: fails before any network access ─────────────────────────

func TestServersCreate_RejectsUnknownAtomicStrategy_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "servers", "create",
		"--name", "staging", "--protocol-type", "ssh",
		"--atomic", "--atomic-strategy", "rsync_release")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid --atomic-strategy")
	assert.Contains(t, err.Error(), "copy_release")
	assert.Contains(t, err.Error(), "copy_cache")
	assert.False(t, *called, "validation must fail before any HTTP request")
}

func TestServersUpdate_RejectsUnknownAtomicStrategy_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "servers", "update", "srv-1",
		"--atomic-strategy", "nope")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid --atomic-strategy")
	assert.False(t, *called, "validation must fail before any HTTP request")
}

func TestServersCreate_RejectsZeroAtomicRetention_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "servers", "create",
		"--name", "staging", "--protocol-type", "ssh",
		"--atomic-retention", "0")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid --atomic-retention")
	assert.False(t, *called, "validation must fail before any HTTP request")
}

func TestServersCreate_RejectsNegativeAtomicRetention_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "servers", "create",
		"--name", "staging", "--protocol-type", "ssh",
		"--atomic-retention", "-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid --atomic-retention")
	assert.False(t, *called, "validation must fail before any HTTP request")
}

func TestServersUpdate_RejectsZeroAtomicRetention_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "servers", "update", "srv-1", "--atomic-retention", "0")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid --atomic-retention")
	assert.False(t, *called, "validation must fail before any HTTP request")
}

func TestServerDeploymentFlags_ValidateAcceptsSupportedStrategies(t *testing.T) {
	for _, s := range []string{"copy_release", "copy_cache"} {
		f := parseDeploymentFlags(t, "--atomic-strategy", s)
		assert.NoError(t, f.validate(), "%s must be accepted", s)
	}
}

func TestServerDeploymentFlags_ValidateAcceptsRetentionOfOne(t *testing.T) {
	f := parseDeploymentFlags(t, "--atomic-retention", "1")
	assert.NoError(t, f.validate())
}

// A zero-valued flag that was never supplied must not trip the >= 1 check —
// otherwise every `servers update --name x` would fail.
func TestServerDeploymentFlags_ValidateIgnoresUnsuppliedFlags(t *testing.T) {
	f := parseDeploymentFlags(t)
	assert.NoError(t, f.validate())
}

// ── request application ───────────────────────────────────────────────────────

func TestServerDeploymentFlags_OmittedFlagsLeaveCreateRequestZero(t *testing.T) {
	f := parseDeploymentFlags(t)
	var req sdk.ServerCreateRequest
	f.applyToCreate(&req)

	assert.Nil(t, req.Branch)
	assert.Nil(t, req.AutoDeploy)
	assert.Nil(t, req.Atomic)
	assert.Empty(t, req.AtomicStrategy)
	assert.Nil(t, req.AtomicRetention)
}

func TestServerDeploymentFlags_OmittedFlagsLeaveUpdateRequestZero(t *testing.T) {
	f := parseDeploymentFlags(t)
	var req sdk.ServerUpdateRequest
	f.applyToUpdate(&req)

	assert.Nil(t, req.Branch)
	assert.Nil(t, req.AutoDeploy)
	assert.Nil(t, req.Atomic)
	assert.Empty(t, req.AtomicStrategy)
	assert.Nil(t, req.AtomicRetention)
}

func TestServerDeploymentFlags_ExplicitFalseSetsNonNilPointer(t *testing.T) {
	f := parseDeploymentFlags(t, "--auto-deploy=false", "--atomic=false")

	var create sdk.ServerCreateRequest
	f.applyToCreate(&create)
	require.NotNil(t, create.AutoDeploy, "--auto-deploy=false must be sent, not dropped")
	assert.False(t, *create.AutoDeploy)
	require.NotNil(t, create.Atomic, "--atomic=false must be sent, not dropped")
	assert.False(t, *create.Atomic)
	// atomic=false must NOT pull in the copy_release default.
	assert.Empty(t, create.AtomicStrategy)

	var update sdk.ServerUpdateRequest
	f.applyToUpdate(&update)
	require.NotNil(t, update.AutoDeploy)
	assert.False(t, *update.AutoDeploy)
	require.NotNil(t, update.Atomic)
	assert.False(t, *update.Atomic)
	assert.Empty(t, update.AtomicStrategy)
}

// --atomic on create with no explicit strategy pins copy_release so the payload
// is deterministic. On update the same input must leave the strategy alone —
// writing one would clobber an existing copy_cache setting.
func TestServerDeploymentFlags_AtomicDefaultsStrategyOnCreateOnly(t *testing.T) {
	f := parseDeploymentFlags(t, "--atomic")

	var create sdk.ServerCreateRequest
	f.applyToCreate(&create)
	require.NotNil(t, create.Atomic)
	assert.True(t, *create.Atomic)
	assert.Equal(t, "copy_release", create.AtomicStrategy)

	var update sdk.ServerUpdateRequest
	f.applyToUpdate(&update)
	require.NotNil(t, update.Atomic)
	assert.True(t, *update.Atomic)
	assert.Empty(t, update.AtomicStrategy,
		"update must never write a strategy the operator did not supply")
}

func TestServerDeploymentFlags_ExplicitStrategyWinsOverCreateDefault(t *testing.T) {
	f := parseDeploymentFlags(t, "--atomic", "--atomic-strategy", "copy_cache")

	var create sdk.ServerCreateRequest
	f.applyToCreate(&create)
	assert.Equal(t, "copy_cache", create.AtomicStrategy)
}

func TestServerDeploymentFlags_AppliesEveryFlag(t *testing.T) {
	f := parseDeploymentFlags(t,
		"--branch", "staging",
		"--auto-deploy",
		"--atomic",
		"--atomic-strategy", "copy_cache",
		"--atomic-retention", "5",
	)

	var create sdk.ServerCreateRequest
	f.applyToCreate(&create)
	require.NotNil(t, create.Branch)
	assert.Equal(t, "staging", *create.Branch)
	require.NotNil(t, create.AutoDeploy)
	assert.True(t, *create.AutoDeploy)
	require.NotNil(t, create.Atomic)
	assert.True(t, *create.Atomic)
	assert.Equal(t, "copy_cache", create.AtomicStrategy)
	require.NotNil(t, create.AtomicRetention)
	assert.Equal(t, 5, *create.AtomicRetention)

	var update sdk.ServerUpdateRequest
	f.applyToUpdate(&update)
	require.NotNil(t, update.Branch)
	assert.Equal(t, "staging", *update.Branch)
	require.NotNil(t, update.AutoDeploy)
	assert.True(t, *update.AutoDeploy)
	require.NotNil(t, update.Atomic)
	assert.True(t, *update.Atomic)
	assert.Equal(t, "copy_cache", update.AtomicStrategy)
	require.NotNil(t, update.AtomicRetention)
	assert.Equal(t, 5, *update.AtomicRetention)
}

// Branch alone must not drag any atomic setting onto the wire.
func TestServerDeploymentFlags_BranchOnlyTouchesBranch(t *testing.T) {
	f := parseDeploymentFlags(t, "--branch", "main")

	var update sdk.ServerUpdateRequest
	f.applyToUpdate(&update)
	require.NotNil(t, update.Branch)
	assert.Equal(t, "main", *update.Branch)
	assert.Nil(t, update.AutoDeploy)
	assert.Nil(t, update.Atomic)
	assert.Empty(t, update.AtomicStrategy)
	assert.Nil(t, update.AtomicRetention)
}

// ── command → request wiring (end to end) ─────────────────────────────────────
//
// These are the only tests that fail if `deployFlags.applyToCreate(&req)` or
// `deployFlags.applyToUpdate(&req)` is deleted from the command bodies. Every
// other test in this file constructs the request itself, so the two lines that
// actually make the feature work were previously uncovered.

func TestServersCreate_SendsDeploymentSettingsOnTheWire(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	err := runServersCmd(t, "servers", "create",
		"--name", "staging", "--protocol-type", "ssh",
		"--hostname", "h", "--username", "u",
		"--branch", "staging",
		"--auto-deploy=false",
		"--atomic",
		"--atomic-retention", "5",
	)
	require.NoError(t, err)

	srv := cap.server(t)
	assert.Equal(t, http.MethodPost, cap.method)
	assert.Equal(t, "staging", srv["branch"])
	assert.Equal(t, false, srv["auto_deploy"], "explicit --auto-deploy=false must reach the wire")
	assert.Equal(t, true, srv["atomic"])
	assert.Equal(t, "copy_release", srv["atomic_strategy"], "create pins the default strategy")
	assert.Equal(t, float64(5), srv["atomic_retention"])
}

func TestServersUpdate_SendsOnlySuppliedSettingsOnTheWire(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	err := runServersCmd(t, "servers", "update", "srv-1", "--branch", "main")
	require.NoError(t, err)

	srv := cap.server(t)
	assert.Equal(t, http.MethodPut, cap.method)
	assert.Equal(t, "main", srv["branch"])
	// Requirement: an update never disturbs a setting the operator did not name.
	for _, k := range []string{"auto_deploy", "atomic", "atomic_strategy", "atomic_retention"} {
		assert.NotContains(t, srv, k, "%s must not be sent when its flag was omitted", k)
	}
}

// ── #1: an explicitly-cleared branch must reach the wire ──────────────────────
//
// The DeployHQ backend accepts and persists `branch: ""` — IGNORE_PARAMS_ON_BLANK
// covers only credential params, and every consumer resolves the branch with
// .presence, so an empty string reads as "unpinned, fall back to the repository
// default". A `string` field with omitempty silently swallowed that intent.

func TestServersUpdate_ExplicitEmptyBranchReachesTheWire(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	err := runServersCmd(t, "servers", "update", "srv-1", "--branch", "")
	require.NoError(t, err)

	srv := cap.server(t)
	require.Contains(t, srv, "branch", "an explicitly supplied empty --branch must be sent, not dropped")
	assert.Equal(t, "", srv["branch"])
}

func TestServersUpdate_OmittedBranchStillAbsent(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	err := runServersCmd(t, "servers", "update", "srv-1", "--name", "renamed")
	require.NoError(t, err)

	srv := cap.server(t)
	assert.Equal(t, "renamed", srv["name"])
	assert.NotContains(t, srv, "branch", "an omitted --branch must stay off the wire")
}

// ── #2: a branch set on a grouped server is dormant ───────────────────────────

func TestBranchIsDormant(t *testing.T) {
	grouped := "grp-1"
	empty := ""

	cases := []struct {
		name      string
		supplied  bool
		server    *sdk.Server
		wantDorma bool
	}{
		{"supplied on grouped server", true, &sdk.Server{ServerGroupIdentifier: &grouped}, true},
		{"supplied on ungrouped server", true, &sdk.Server{}, false},
		{"supplied, group identifier empty", true, &sdk.Server{ServerGroupIdentifier: &empty}, false},
		{"not supplied, grouped", false, &sdk.Server{ServerGroupIdentifier: &grouped}, false},
		{"nil server", true, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantDorma, branchIsDormant(tc.supplied, tc.server))
		})
	}
}

// ── #4: atomic requested but not applied (silent account-level strip) ─────────

func TestAtomicNotApplied(t *testing.T) {
	yes, no := true, false

	cases := []struct {
		name      string
		requested bool
		server    *sdk.Server
		want      bool
	}{
		{"requested, came back false", true, &sdk.Server{Atomic: &no}, true},
		{"requested, came back absent", true, &sdk.Server{}, true},
		{"requested, came back true", true, &sdk.Server{Atomic: &yes}, false},
		{"not requested, came back false", false, &sdk.Server{Atomic: &no}, false},
		{"nil server", true, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, atomicNotApplied(tc.requested, tc.server))
		})
	}
}

// ── #5: the Managed VPS billing guardrail must precede the API call ──────────
//
// The gate is the only thing between a non-interactive invocation and a
// billable provisioning call, and this change inserted statements on both
// sides of it. blockNetwork turns "no request was made" into a fact.

func TestServersCreate_ManagedVPSRequiresAcceptCost_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "servers", "create",
		"--name", "vps", "--protocol-type", "managed_vps",
		"--region", "lon1", "--size", "s-1vcpu-1gb",
		// deployment flags on the same command must not let the gate be skipped
		"--branch", "staging", "--atomic",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accept-cost")
	assert.False(t, *called, "the cost gate must fire before any API request")
}

func TestServersCreate_ManagedVPSWithAcceptCost_Proceeds(t *testing.T) {
	withResolvableContext(t)
	cap := captureRequest(t)

	err := runServersCmd(t, "servers", "create",
		"--name", "vps", "--protocol-type", "managed_vps",
		"--region", "lon1", "--size", "s-1vcpu-1gb", "--accept-cost",
		"--branch", "staging",
	)
	require.NoError(t, err)

	srv := cap.server(t)
	assert.Equal(t, "staging", srv["branch"])
	// provisioning params stay top-level siblings of `server`
	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, "lon1", cap.body["region"])
	assert.Equal(t, "s-1vcpu-1gb", cap.body["size"])
}
