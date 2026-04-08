package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
)

// installSSHKey attempts to install a public key on a remote server's
// ~/.ssh/authorized_keys. It tries ssh-copy-id first, then falls back
// to a raw ssh command. Both methods require the user to authenticate
// (password prompt or existing SSH agent) via TTY passthrough.
func installSSHKey(env *output.Envelope, host string, port int, user, publicKey string) error {
	if publicKey == "" {
		return fmt.Errorf("no public key to install")
	}

	portStr := strconv.Itoa(port)

	// Try ssh-copy-id first (available on macOS/Linux)
	if path, err := exec.LookPath("ssh-copy-id"); err == nil {
		env.Status("Attempting to install key via ssh-copy-id...")
		if err := runSSHCopyID(path, host, portStr, user, publicKey); err == nil {
			return nil
		}
		env.Status("ssh-copy-id failed, trying ssh fallback...")
	}

	// Fallback: raw ssh command to append key
	env.Status("Attempting to install key via ssh...")
	return runSSHAppend(host, portStr, user, publicKey)
}

// runSSHCopyID writes the public key to a temp file and runs ssh-copy-id.
func runSSHCopyID(sshCopyIDPath, host, port, user, publicKey string) error {
	// ssh-copy-id needs a file, write key to temp
	tmpFile, err := os.CreateTemp("", "dhq-ssh-key-*.pub")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck

	if _, err := tmpFile.WriteString(publicKey + "\n"); err != nil {
		tmpFile.Close() //nolint:errcheck
		return err
	}
	tmpFile.Close() //nolint:errcheck

	//nolint:gosec // host, port, user are from CLI flags, not untrusted input
	cmd := exec.Command(sshCopyIDPath,
		"-i", tmpFile.Name(),
		"-p", port,
		"-o", "StrictHostKeyChecking=accept-new",
		fmt.Sprintf("%s@%s", user, host),
	)
	// Passthrough stdin/stdout/stderr so the user can enter their password
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr // human output goes to stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runSSHAppend uses a raw ssh command to append the key to authorized_keys.
func runSSHAppend(host, port, user, publicKey string) error {
	// Escape single quotes in the public key for the shell command
	escapedKey := strings.ReplaceAll(publicKey, "'", "'\\''")

	script := fmt.Sprintf(
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys",
		escapedKey,
	)

	//nolint:gosec // host, port, user are from CLI flags, not untrusted input
	cmd := exec.Command("ssh",
		"-p", port,
		"-o", "StrictHostKeyChecking=accept-new",
		fmt.Sprintf("%s@%s", user, host),
		script,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
