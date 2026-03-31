package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func newConfigureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Interactive setup wizard",
		Long:  "Set up your DeployHQ CLI configuration interactively. Creates a .deployhq.toml in the current directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			if !env.IsTTY {
				return &output.UserError{
					Message: "Interactive setup requires a terminal",
					Hint:    "Use 'dhq config set' for non-interactive configuration",
				}
			}

			env.Status("DeployHQ CLI Setup Wizard")
			env.Status("")

			// Step 1: Check if logged in, try to get projects
			client, err := cliCtx.Client()
			if err != nil {
				env.Status("Not logged in. Run 'dhq auth login' or 'dhq signup' first.")
				return nil
			}

			// Step 2: Pick a default project
			projects, err := client.ListProjects(cliCtx.Background())
			if err != nil || len(projects) == 0 {
				env.Status("No projects found. Create one with 'dhq projects create --name my-app'")
				return nil
			}

			items := make([]string, len(projects))
			for i, p := range projects {
				items[i] = fmt.Sprintf("%s (%s)", p.Name, p.Permalink)
			}

			prompt := promptui.Select{
				Label: "Default project for this directory",
				Items: items,
			}

			idx, _, err := prompt.Run()
			if err != nil {
				return &output.UserError{Message: "Setup cancelled"}
			}

			project := projects[idx]
			path := config.ProjectConfigPath()

			if err := config.Set(path, "project", project.Permalink); err != nil {
				return &output.InternalError{Message: "save config", Cause: err}
			}

			env.Status("")
			output.ColorGreen.Fprintf(env.Stderr, "Saved to %s\n", path) //nolint:errcheck
			env.Status("")
			env.Status("Default project: %s (%s)", project.Name, project.Permalink)
			env.Status("")
			env.Status("You can now run commands without -p:")
			env.Status("  dhq servers list")
			env.Status("  dhq deploy")
			env.Status("  dhq deployments list")
			return nil
		},
	}
}
