package commands

import (
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			info := map[string]string{
				"version": version,
				"go":      runtime.Version(),
				"os":      runtime.GOOS,
				"arch":    runtime.GOARCH,
			}

			if env.JSONMode {
				return env.WriteJSON(info)
			}

			env.Status("deployhq-cli %s (%s/%s, %s)", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
			return nil
		},
	}
}
