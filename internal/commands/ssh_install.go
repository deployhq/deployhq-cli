package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/deployhq/deployhq-cli/internal/output"
)

// runInstallCmd runs one key-install command (ssh-copy-id or ssh). It is a
// variable so tests can capture the arguments without shelling out.
var runInstallCmd = func(name string, args []string) error {
	//nolint:gosec // host, port, user are from CLI flags, not untrusted input
	cmd := exec.Command(name, args...)
	// Passthrough stdin/stdout/stderr so the user can enter their password
	// (interactive mode only; batch mode never prompts).
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr // human output goes to stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// sshInstallOptions are the ssh -o options used for key installation. In
// batch (non-interactive) mode ssh never prompts for a password or passphrase
// and gives up quickly on an unreachable host, so the command fails instead of
// hanging with nobody there to answer.
func sshInstallOptions(batch bool) []string {
	opts := []string{"-o", "StrictHostKeyChecking=accept-new"}
	if batch {
		opts = append(opts, "-o", "BatchMode=yes", "-o", "ConnectTimeout=15")
	}
	return opts
}

// installSSHKey attempts to install a public key on a remote server's
// ~/.ssh/authorized_keys. It tries ssh-copy-id first, then falls back
// to a raw ssh command. Both use the person's own SSH access: in an
// interactive terminal they may be asked for a password; in non-interactive
// mode only non-prompting auth (e.g. an SSH agent or default key) is used.
func installSSHKey(env *output.Envelope, host string, port int, user, publicKey string) error {
	if publicKey == "" {
		return fmt.Errorf("no public key to install")
	}

	portStr := strconv.Itoa(port)
	batch := env.NonInteractive || !env.IsTTY

	// Try ssh-copy-id first (available on macOS/Linux)
	if path, err := exec.LookPath("ssh-copy-id"); err == nil {
		env.Status("Attempting to install key via ssh-copy-id...")
		if err := runSSHCopyID(path, host, portStr, user, publicKey, batch); err == nil {
			return nil
		}
		env.Status("ssh-copy-id failed, trying ssh fallback...")
	}

	// Fallback: raw ssh command to append key
	env.Status("Attempting to install key via ssh...")
	err := runInstallCmd("ssh", sshAppendArgs(host, portStr, user, publicKey, batch))
	if err != nil && batch {
		return fmt.Errorf("%w (non-interactive mode: ssh may not prompt for a password, so your SSH login must work without one)", err)
	}
	return err
}

// runSSHCopyID writes the public key to a temp file and runs ssh-copy-id.
func runSSHCopyID(sshCopyIDPath, host, port, user, publicKey string, batch bool) error {
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

	return runInstallCmd(sshCopyIDPath, sshCopyIDArgs(tmpFile.Name(), host, port, user, batch))
}

// sshCopyIDArgs builds the ssh-copy-id arguments; -o options are passed
// through to ssh. -f (force mode) is required because only the public key
// exists: without it ssh-copy-id refuses to run unless the matching private
// key sits next to the .pub file.
func sshCopyIDArgs(keyFile, host, port, user string, batch bool) []string {
	args := []string{"-f", "-i", keyFile, "-p", port}
	args = append(args, sshInstallOptions(batch)...)
	return append(args, fmt.Sprintf("%s@%s", user, host))
}

// sshAppendArgs builds the ssh arguments that append the key to authorized_keys.
// "--" ends ssh's options, so the destination is never read as one.
func sshAppendArgs(host, port, user, publicKey string, batch bool) []string {
	args := []string{"-p", port}
	args = append(args, sshInstallOptions(batch)...)
	return append(args, "--", fmt.Sprintf("%s@%s", user, host), authorizedKeysAppendScript(publicKey))
}
