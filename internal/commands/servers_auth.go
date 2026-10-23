package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// installSSHKeyFn is the hook `dhq servers create --install-key` uses to put
// DeployHQ's public key on the target server. It is a variable so tests can
// observe the call without shelling out to ssh.
var installSSHKeyFn = installSSHKey

// maxPasswordStdinBytes bounds what --password-stdin reads, so a mistaken pipe
// (e.g. a whole file) fails fast instead of being sent as a password.
const maxPasswordStdinBytes = 4096

// serverPasswordRequired reports whether a server of this protocol needs a
// login password: FTP/FTPS always do, and SSH does unless key auth is used.
// Rsync always connects with SSH keys.
func serverPasswordRequired(protocolType string, useSSHKeys bool) bool {
	switch protocolType {
	case "ftp", "ftps":
		return true
	case "ssh":
		return !useSSHKeys
	}
	return false
}

// serverUsesSSHKeys reports whether DeployHQ will connect to the server with
// its own SSH key (rather than a password).
func serverUsesSSHKeys(protocolType string, useSSHKeys bool) bool {
	return useSSHKeys || protocolType == "rsync"
}

// passwordAgentHint is the guidance shown when a password is needed but cannot
// be asked for privately. It steers people away from handing the password to an
// AI agent, where it would stay in the conversation history.
const passwordAgentHint = "Use key-based access instead (--use-ssh-keys) if the server supports it. " +
	"For password-only servers (FTP/FTPS, or SFTP with a password), the person who owns the password " +
	"should add the server themselves rather than give the password to an AI agent: either run this " +
	"command in their own terminal with --password-stdin (it asks for the password with hidden input), " +
	"or add the server in the DeployHQ web app."

// readServerPassword resolves the server password for `dhq servers create`.
// It returns the --password value when given, reads --password-stdin, or,
// when a password is required and none was supplied, prompts with hidden input
// in an interactive terminal. In non-interactive mode it returns an error that
// explains how to add a password-based server safely.
func readServerPassword(cmd *cobra.Command, env *output.Envelope, password string, fromStdin bool, protocolType string, useSSHKeys bool) (string, error) {
	if fromStdin {
		return readPasswordStdin(cmd, env)
	}
	if password != "" || !serverPasswordRequired(protocolType, useSSHKeys) {
		return password, nil
	}
	if env.NonInteractive || env.JSONMode || !env.IsTTY {
		return "", &output.UserError{
			Message: fmt.Sprintf("A password is required for %s servers that do not use SSH keys", protocolType),
			Hint:    passwordAgentHint,
		}
	}
	return promptHiddenPassword(env, int(os.Stdin.Fd()))
}

// readPasswordStdin implements --password-stdin: a hidden prompt when stdin is
// a terminal, otherwise the piped input with the trailing newline removed
// (the same convention as `docker login --password-stdin`).
func readPasswordStdin(cmd *cobra.Command, env *output.Envelope) (string, error) {
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return promptHiddenPassword(env, int(f.Fd()))
	}
	raw, err := io.ReadAll(io.LimitReader(in, maxPasswordStdinBytes+1))
	if err != nil {
		return "", &output.InternalError{Message: "read password from stdin", Cause: err}
	}
	if len(raw) > maxPasswordStdinBytes {
		return "", &output.UserError{Message: "--password-stdin input is too long to be a password"}
	}
	pw := strings.TrimRight(string(raw), "\r\n")
	if pw == "" {
		return "", &output.UserError{
			Message: "--password-stdin received an empty password",
			Hint:    "Run the command in a terminal to be asked for the password, or pipe it in from a file: dhq servers create ... --password-stdin < password.txt",
		}
	}
	return pw, nil
}

func promptHiddenPassword(env *output.Envelope, fd int) (string, error) {
	fmt.Fprint(env.Stderr, "Server password (input hidden): ") //nolint:errcheck
	pw, err := term.ReadPassword(fd)
	fmt.Fprintln(env.Stderr) //nolint:errcheck
	if err != nil {
		return "", &output.InternalError{Message: "read password", Cause: err}
	}
	if len(pw) == 0 {
		return "", &output.UserError{Message: "No password entered"}
	}
	return string(pw), nil
}

// isSSHAuthFailure reports whether a server-create error is DeployHQ failing
// to log in to the server (as opposed to e.g. a bad path or a timeout).
func isSSHAuthFailure(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "public key onto the server")
}

// resolveDeployPublicKey returns the public key DeployHQ will use to connect
// to the server: the account-wide key when --global-key-pair-id names one,
// otherwise the project's own key (which is also what DeployHQ falls back to
// when the global identifier matches no key on the account).
func resolveDeployPublicKey(ctx context.Context, client *sdk.Client, projectID, globalKeyPairID string) (string, error) {
	if globalKeyPairID != "" {
		keys, err := client.ListSSHKeys(ctx, nil)
		if err == nil {
			for _, k := range keys {
				if k.Identifier == globalKeyPairID && k.PublicKey != "" {
					return strings.TrimSpace(k.PublicKey), nil
				}
			}
		}
	}
	project, err := client.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	if project.PublicKey == "" {
		return "", fmt.Errorf("the project has no public key")
	}
	return strings.TrimSpace(project.PublicKey), nil
}

// authorizedKeysAppendScript is the remote shell snippet that appends a public
// key to ~/.ssh/authorized_keys, creating the directory with safe permissions.
func authorizedKeysAppendScript(publicKey string) string {
	escapedKey := strings.ReplaceAll(publicKey, "'", "'\\''")
	return fmt.Sprintf(
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys",
		escapedKey,
	)
}

// sshKeyInstallCommand is the one-line command a person can run, with their
// own SSH access, to install DeployHQ's public key on the server.
func sshKeyInstallCommand(host string, port int, user, publicKey string) string {
	portFlag := ""
	if port != 0 && port != 22 {
		portFlag = fmt.Sprintf("-p %d ", port)
	}
	return fmt.Sprintf(`ssh %s%s@%s "%s"`, portFlag, user, host, authorizedKeysAppendScript(publicKey))
}

// sshKeyMissingHint explains an SSH authentication failure during server
// creation: DeployHQ's key is not on the server yet. It includes the key and
// the command to install it when the key is known.
func sshKeyMissingHint(host string, port int, user, publicKey, keyErr string, installTried bool) string {
	installOffer := "Or re-run with --install-key to have dhq do this. "
	if installTried {
		installOffer = ""
	}
	if publicKey == "" {
		h := "DeployHQ connects with its own SSH key, which must be in ~/.ssh/authorized_keys on the server."
		if keyErr != "" {
			h += " dhq could not fetch the public key (" + keyErr + "); find it in the DeployHQ web app under the project's servers."
		}
		return h
	}
	return fmt.Sprintf(
		"DeployHQ connects with its own SSH key, which is not on the server yet. "+
			"Add this public key to ~/.ssh/authorized_keys for %s on %s:\n  %s\n\n"+
			"To install it with your own SSH access, run:\n  %s\n\n"+
			"%s"+
			"If you are an AI agent, ask the person before modifying their server.",
		user, host, publicKey, sshKeyInstallCommand(host, port, user, publicKey), installOffer,
	)
}
