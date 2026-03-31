package commands

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/deployhq/deployhq-cli/internal/output"
	versionpkg "github.com/deployhq/deployhq-cli/internal/version"
	"github.com/spf13/cobra"
)

func newUpdateCmd(currentVersion string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update deployhq-cli to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			env.Status("Current version: %s", currentVersion)
			env.Status("Checking for updates...")

			info := versionpkg.Check(currentVersion)

			if !info.UpdateAvailable || info.Latest == "" {
				env.Status("Already up to date.")
				return nil
			}

			env.Status("Update available: %s -> %s", currentVersion, info.Latest)

			// Try Homebrew first
			if brewPath, err := exec.LookPath("brew"); err == nil {
				env.Status("Updating via Homebrew...")
				c := exec.Command(brewPath, "upgrade", "deployhq")
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				if err := c.Run(); err == nil {
					env.Status("Updated to %s via Homebrew.", info.Latest)
					return nil
				}
				// Homebrew failed (maybe not installed via brew), fall through
			}

			// Fall back to install script
			env.Status("Downloading v%s...", info.Latest)
			script := fmt.Sprintf(
				"curl -fsSL https://raw.githubusercontent.com/deployhq/deployhq-cli/main/install.sh | sh",
			)

			c := exec.Command("sh", "-c", script)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return &output.UserError{
					Message: fmt.Sprintf("Auto-update failed: %v", err),
					Hint: fmt.Sprintf("Download manually from %s\n\n  Or run: curl -fsSL https://raw.githubusercontent.com/deployhq/deployhq-cli/main/install.sh | sh",
						info.URL),
				}
			}

			env.Status("Updated to %s.", info.Latest)

			if runtime.GOOS == "windows" {
				env.Status("\nNote: restart your terminal to use the new version.")
			}
			return nil
		},
	}
}
