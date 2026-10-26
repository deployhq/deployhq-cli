package commands

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sshAuthFailureBody is the 422 the API returns when DeployHQ cannot log in to
// an SSH server while validating it on create.
const sshAuthFailureBody = `{"protocol":["SSH authentication failed. If using SSH keys, verify the public key is installed on the server and the private key matches. If using a password, check it is correct and the user is allowed SSH access."]}`

const projectPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIProjectKey DeployHQ.com Key for my-app"
const globalPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGlobalKey"

// fakeServerAPI answers server creation with a scripted sequence of responses
// and serves the project and account SSH keys.
type fakeServerAPI struct {
	mu          sync.Mutex
	createCalls int
	// createFailures is how many leading create calls fail with an SSH auth error.
	createFailures int
	lastServer     map[string]any
	// sshKeysStatus, when non-zero, is the status listing account SSH keys returns.
	sshKeysStatus int
}

func installFakeServerAPI(t *testing.T, api *fakeServerAPI) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		api.mu.Lock()
		defer api.mu.Unlock()
		respond := func(status int, body string) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Request:    r,
			}, nil
		}
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/projects/my-app/servers"):
			api.createCalls++
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			api.lastServer, _ = body["server"].(map[string]any)
			if api.createCalls <= api.createFailures {
				return respond(http.StatusUnprocessableEntity, sshAuthFailureBody)
			}
			return respond(http.StatusCreated, `{"identifier":"srv-1","name":"prod","protocol_type":"ssh"}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/my-app"):
			return respond(http.StatusOK, `{"identifier":"p-1","name":"My App","permalink":"my-app","public_key":"`+projectPublicKey+`"}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/ssh_keys") && api.sshKeysStatus != 0:
			return respond(api.sshKeysStatus, `{"error":"You do not have permission to view SSH keys"}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/ssh_keys"):
			return respond(http.StatusOK, `[{"identifier":"key-abc123","title":"global","public_key":"`+globalPublicKey+`"}]`)
		}
		// Anything else (e.g. the background update check) is irrelevant here.
		return respond(http.StatusOK, `{}`)
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
}

type installCall struct {
	host, user, key string
	port            int
}

func stubInstallSSHKey(t *testing.T, result error) *[]installCall {
	t.Helper()
	calls := &[]installCall{}
	orig := installSSHKeyFn
	installSSHKeyFn = func(_ *output.Envelope, host string, port int, user, publicKey string) error {
		*calls = append(*calls, installCall{host: host, user: user, key: publicKey, port: port})
		return result
	}
	t.Cleanup(func() { installSSHKeyFn = orig })
	return calls
}

var sshCreateArgs = []string{
	"servers", "create", "--name", "prod", "--protocol-type", "ssh",
	"--hostname", "prod.example.com", "--username", "deploy", "--use-ssh-keys",
	"--path", "/var/www/app",
}

func TestServersCreate_InstallKeyUsesProjectKeyByDefault(t *testing.T) {
	withResolvableContext(t)
	api := &fakeServerAPI{createFailures: 1}
	installFakeServerAPI(t, api)
	calls := stubInstallSSHKey(t, nil)

	err := runServersCmd(t, append(sshCreateArgs, "--install-key")...)
	require.NoError(t, err)

	require.Len(t, *calls, 1, "--install-key must install the project's key when no global key is given")
	assert.Equal(t, projectPublicKey, (*calls)[0].key)
	assert.Equal(t, "prod.example.com", (*calls)[0].host)
	assert.Equal(t, "deploy", (*calls)[0].user)
	assert.Equal(t, 22, (*calls)[0].port)
	assert.Equal(t, 2, api.createCalls, "creation is retried after the key is installed")
}

func TestServersCreate_InstallKeyUsesGlobalKeyWhenGiven(t *testing.T) {
	withResolvableContext(t)
	api := &fakeServerAPI{createFailures: 1}
	installFakeServerAPI(t, api)
	calls := stubInstallSSHKey(t, nil)

	err := runServersCmd(t, append(sshCreateArgs, "--install-key", "--global-key-pair-id", "key-abc123", "--port", "2222")...)
	require.NoError(t, err)

	require.Len(t, *calls, 1)
	assert.Equal(t, globalPublicKey, (*calls)[0].key)
	assert.Equal(t, 2222, (*calls)[0].port)
}

func TestServersCreate_InstallKeyFailureIsReportedNotLooped(t *testing.T) {
	withResolvableContext(t)
	api := &fakeServerAPI{createFailures: 100}
	installFakeServerAPI(t, api)
	calls := stubInstallSSHKey(t, errors.New("ssh exited 255"))

	err := runServersCmd(t, append(sshCreateArgs, "--install-key")...)
	require.Error(t, err)

	assert.Len(t, *calls, 1, "the install is attempted once")
	assert.Equal(t, 1, api.createCalls)
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr))
	assert.Contains(t, uerr.Hint, projectPublicKey)
}

func TestServersCreate_InstallKeyRetriesOnlyOnce(t *testing.T) {
	withResolvableContext(t)
	// Key installs "successfully" but the server still rejects the login
	// (e.g. sshd ignores authorized_keys): must not loop forever.
	api := &fakeServerAPI{createFailures: 100}
	installFakeServerAPI(t, api)
	calls := stubInstallSSHKey(t, nil)

	err := runServersCmd(t, append(sshCreateArgs, "--install-key")...)
	require.Error(t, err)
	assert.Len(t, *calls, 1)
	assert.Equal(t, 2, api.createCalls)
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr))
	assert.NotContains(t, uerr.Hint, "re-run with --install-key", "do not suggest what was just tried")
}

func TestServersCreate_AuthFailureShowsKeyAndInstallCommand(t *testing.T) {
	withResolvableContext(t)
	api := &fakeServerAPI{createFailures: 1}
	installFakeServerAPI(t, api)
	calls := stubInstallSSHKey(t, nil)

	err := runServersCmd(t, sshCreateArgs...)
	require.Error(t, err)

	assert.Empty(t, *calls, "without --install-key the server is never modified")
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Contains(t, uerr.Message, "SSH authentication failed")
	assert.Contains(t, uerr.Hint, projectPublicKey)
	assert.Contains(t, uerr.Hint, `| ssh deploy@prod.example.com 'mkdir -p ~/.ssh`)
	assert.Contains(t, uerr.Hint, "ask the person before modifying their server")
	assert.Contains(t, uerr.Hint, "re-run with --install-key")
}

func TestServersCreate_InstallKeyRequiresKeyAuth_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "servers", "create", "--name", "prod", "--protocol-type", "ssh",
		"--hostname", "h", "--username", "u", "--install-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--install-key requires SSH key authentication")
	assert.False(t, *called)
}

func TestServersCreate_PasswordStdinReadsPipedPassword(t *testing.T) {
	withResolvableContext(t)
	api := &fakeServerAPI{}
	installFakeServerAPI(t, api)

	cmd := NewRootCmd("test")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader("s3cret pass\n"))
	cmd.SetArgs([]string{"servers", "create", "--name", "ftp", "--protocol-type", "ftp",
		"--hostname", "ftp.example.com", "--username", "deploy", "--password-stdin"})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, "s3cret pass", api.lastServer["password"], "trailing newline is stripped")
}

func TestServersCreate_PasswordStdinRejectsEmptyInput_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	cmd := NewRootCmd("test")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader("\n"))
	cmd.SetArgs([]string{"servers", "create", "--name", "ftp", "--protocol-type", "ftp",
		"--hostname", "h", "--username", "u", "--password-stdin"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty password")
	assert.False(t, *called)
}

func TestServersCreate_PasswordAndPasswordStdinConflict_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)

	err := runServersCmd(t, "servers", "create", "--name", "ftp", "--protocol-type", "ftp",
		"--hostname", "h", "--username", "u", "--password", "x", "--password-stdin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
	assert.False(t, *called)
}

func TestServersCreate_MissingPasswordNonInteractiveSendsPersonToSafePath_NoHTTP(t *testing.T) {
	for _, proto := range []string{"ftp", "ftps", "ssh"} {
		t.Run(proto, func(t *testing.T) {
			withResolvableContext(t)
			called := blockNetwork(t)

			err := runServersCmd(t, "servers", "create", "--name", "s", "--protocol-type", proto,
				"--hostname", "h", "--username", "u")
			require.Error(t, err)
			var uerr *output.UserError
			require.True(t, errors.As(err, &uerr))
			assert.Contains(t, uerr.Message, "password is required")
			assert.Contains(t, uerr.Hint, "--password-stdin")
			assert.Contains(t, uerr.Hint, "DeployHQ web app")
			assert.Contains(t, uerr.Hint, "rather than give the password to an AI agent")
			assert.False(t, *called)
		})
	}
}

func TestServersCreate_KeyAuthNeedsNoPassword(t *testing.T) {
	assert.False(t, serverPasswordRequired("ssh", true))
	assert.False(t, serverPasswordRequired("rsync", false))
	assert.False(t, serverPasswordRequired("s3", false))
	assert.True(t, serverPasswordRequired("ssh", false))
	assert.True(t, serverPasswordRequired("ftp", false))
}

func TestServersCreate_ExampleLeadsWithKeyAuth(t *testing.T) {
	ex := newServersCreateCmd().Example
	firstExample := strings.SplitN(ex, "\n\n", 2)[0]
	assert.Contains(t, firstExample, "--use-ssh-keys")
	assert.NotContains(t, firstExample, "--password")
	assert.NotContains(t, ex, "--password '", "no example puts a password in the command")
	assert.Contains(t, ex, "--password-stdin")
	assert.Contains(t, ex, "shell history")
}

func TestSSHKeyInstallCommand(t *testing.T) {
	assert.Equal(t,
		`printf '%s\n' 'ssh-ed25519 AAAA k' | ssh deploy@h 'mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'`,
		sshKeyInstallCommand("h", 22, "deploy", "ssh-ed25519 AAAA k"))
	assert.True(t, strings.Contains(sshKeyInstallCommand("h", 2222, "deploy", "k"), "| ssh -p 2222 deploy@h "))
}

// The printed command is pasted into a shell, by a person or an agent, so a
// key comment must reach the server as plain text, never run locally.
func TestSSHKeyInstallCommand_KeyIsNotExpandedByTheLocalShell(t *testing.T) {
	dir := t.TempDir()
	got := filepath.Join(dir, "stdin")
	// A stand-in for ssh that records the key it is sent on stdin.
	fake := "#!/bin/sh\ncat > " + got + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(fake), 0o755)) //nolint:gosec // test stub must be executable

	key := `ssh-ed25519 AAAA it's "$(touch ` + filepath.Join(dir, "pwned") + `)" ` + "`id` \\n $HOME"
	cmd := exec.Command("/bin/sh", "-c", sshKeyInstallCommand("h", 22, "deploy", key)) //nolint:gosec // the command under test
	cmd.Env = []string{"PATH=" + dir + ":/usr/bin:/bin", "HOME=/nonexistent"}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	sent, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, key+"\n", string(sent), "the key reaches the server byte for byte")
	assert.NoFileExists(t, filepath.Join(dir, "pwned"), "nothing in the key runs locally")
}

// If the account keys cannot be listed, dhq cannot know which key DeployHQ
// will use, so it must not install the project key in its place.
func TestServersCreate_InstallKeyDoesNotGuessWhenGlobalKeyLookupFails(t *testing.T) {
	withResolvableContext(t)
	api := &fakeServerAPI{createFailures: 1, sshKeysStatus: http.StatusForbidden}
	installFakeServerAPI(t, api)
	calls := stubInstallSSHKey(t, nil)

	err := runServersCmd(t, append(sshCreateArgs, "--install-key", "--global-key-pair-id", "key-abc123")...)
	require.Error(t, err)

	assert.Empty(t, *calls, "no key is installed when the global key cannot be looked up")
	assert.NotContains(t, err.Error(), projectPublicKey, "the hint must not offer the project key instead")
}

// With --non-interactive, --password-stdin never waits at a hidden prompt,
// even when stdin is a terminal (an agent running under a pseudo-terminal).
func TestServersCreate_PasswordStdinNonInteractiveDoesNotPrompt_NoHTTP(t *testing.T) {
	withResolvableContext(t)
	called := blockNetwork(t)
	orig := stdinTerminalFd
	stdinTerminalFd = func(io.Reader) (int, bool) { return -1, true }
	t.Cleanup(func() { stdinTerminalFd = orig })

	err := runServersCmd(t, "servers", "create", "--name", "ftp", "--protocol-type", "ftp",
		"--hostname", "h", "--username", "u", "--password-stdin", "--non-interactive")
	require.Error(t, err)
	var uerr *output.UserError
	require.True(t, errors.As(err, &uerr), "got %T: %v", err, err)
	assert.Contains(t, uerr.Message, "--password-stdin")
	assert.False(t, *called)
}
