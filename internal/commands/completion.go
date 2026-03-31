package commands

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for dhq.

To load completions:

  Bash:
    $ source <(dhq completion bash)
    # To load completions for each session, add to your ~/.bashrc:
    # echo 'source <(dhq completion bash)' >> ~/.bashrc

  Zsh:
    $ source <(dhq completion zsh)
    # To load completions for each session, add to your ~/.zshrc:
    # echo 'source <(dhq completion zsh)' >> ~/.zshrc

  Fish:
    $ dhq completion fish | source
    # To load completions for each session:
    # dhq completion fish > ~/.config/fish/completions/dhq.fish

  PowerShell:
    PS> dhq completion powershell | Out-String | Invoke-Expression
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
	return cmd
}
