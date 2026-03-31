package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive project setup wizard",
		Long:  "Create a new DeployHQ project with repository, server, and optional first deploy — all from your terminal.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope

			if !env.IsTTY {
				return &output.UserError{
					Message: "Interactive setup requires a terminal",
					Hint:    "Use dhq projects create, dhq repos create, dhq servers create for non-interactive setup",
				}
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			ctx := cliCtx.Background()
			reader := bufio.NewReader(os.Stdin)

			env.Status("🚀 DeployHQ Project Setup")
			env.Status("")

			// Step 1: Create project
			env.Status("Step 1/4 — Project")
			projectName := promptString(env, reader, "  Project name")
			if projectName == "" {
				return &output.UserError{Message: "Project name is required"}
			}

			project, err := client.CreateProject(ctx, sdk.ProjectCreateRequest{Name: projectName})
			if err != nil {
				return err
			}
			output.ColorGreen.Fprintf(env.Stderr, "  ✅ Created project: %s (permalink: %s)\n", project.Name, project.Permalink) //nolint:errcheck
			env.Status("")

			// Step 2: Repository
			env.Status("Step 2/4 — Repository")

			scmTypes := []string{"git", "mercurial", "subversion"}
			scmPrompt := promptui.Select{Label: "  SCM type", Items: scmTypes}
			_, scmType, err := scmPrompt.Run()
			if err != nil {
				env.Status("  Skipped repository setup. Add later with: dhq repos create -p %s", project.Permalink)
				return saveAndFinish(env, project, false)
			}

			// Try to detect git remote
			repoURL := detectGitRemote()
			if repoURL != "" {
				env.Status("  Detected: %s", repoURL)
				confirm := promptString(env, reader, "  Use this URL? (Y/n)")
				if confirm != "" && strings.ToLower(confirm) != "y" {
					repoURL = ""
				}
			}
			if repoURL == "" {
				repoURL = promptString(env, reader, "  Repository URL")
			}
			if repoURL == "" {
				env.Status("  Skipped repository setup.")
				return saveAndFinish(env, project, false)
			}

			branch := promptString(env, reader, "  Default branch [main]")
			if branch == "" {
				branch = "main"
			}

			// Show deploy key for private repos (SSH URLs)
			isPrivate := strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://")
			if isPrivate && project.PublicKey != "" {
				env.Status("")
				output.ColorYellow.Fprintln(env.Stderr, "  Add this deploy key to your repository:") //nolint:errcheck
				env.Status("")
				env.Status("  %s", project.PublicKey)
				env.Status("")
				env.Status("  GitHub:    repo → Settings → Deploy keys → Add deploy key")
				env.Status("  GitLab:    repo → Settings → Repository → Deploy keys")
				env.Status("  Bitbucket: repo → Settings → Access keys")
				env.Status("")
				promptString(env, reader, "  Press Enter once you've added the key...")
			}

			_, err = client.CreateRepository(ctx, project.Permalink, sdk.RepositoryCreateRequest{
				ScmType: scmType, URL: repoURL, Branch: branch,
			})
			if err != nil {
				env.Warn("Repository setup failed: %v", err)
				env.Status("  Add later with: dhq repos create -p %s --scm-type %s --url %s", project.Permalink, scmType, repoURL)
			} else {
				output.ColorGreen.Fprintln(env.Stderr, "  ✅ Repository connected") //nolint:errcheck
			}
			env.Status("")

			// Step 3: Server
			env.Status("Step 3/4 — Server")

			protocols := []string{"ssh", "sftp", "ftp", "managed_vps"}
			protoPrompt := promptui.Select{Label: "  Protocol", Items: protocols}
			_, protocol, err := protoPrompt.Run()
			if err != nil {
				env.Status("  Skipped server setup. Add later with: dhq servers create -p %s", project.Permalink)
				return saveAndFinish(env, project, false)
			}

			serverName := promptString(env, reader, "  Server name")
			if serverName == "" {
				serverName = "production"
			}

			serverPath := promptString(env, reader, "  Server path (e.g. /var/www/my-app)")

			server, err := client.CreateServer(ctx, project.Permalink, sdk.ServerCreateRequest{
				Name: serverName, ProtocolType: protocol, ServerPath: serverPath,
			})
			if err != nil {
				env.Warn("Server setup failed: %v", err)
				env.Status("  Add later with: dhq servers create -p %s", project.Permalink)
				return saveAndFinish(env, project, false)
			}
			output.ColorGreen.Fprintf(env.Stderr, "  ✅ Server added: %s (%s)\n", server.Name, server.Identifier) //nolint:errcheck
			env.Status("")

			// Step 4: Deploy?
			env.Status("Step 4/4 — Deploy")
			deployNow := promptString(env, reader, "  Deploy now? (y/N)")

			if strings.ToLower(deployNow) == "y" {
				env.Status("")
				dep, err := client.CreateDeployment(ctx, project.Permalink, sdk.DeploymentCreateRequest{
					ParentIdentifier: server.Identifier,
				})
				if err != nil {
					env.Warn("Deploy failed: %v", err)
					env.Status("  Deploy later with: dhq deploy -p %s", project.Permalink)
				} else {
					env.Status("🚀 Deployment %s queued", dep.Identifier)
					env.Status("")
					_ = watchDeployment(ctx, client, env, project.Permalink, dep.Identifier)
				}
			}

			return saveAndFinish(env, project, true)
		},
	}
}

func saveAndFinish(env *output.Envelope, project *sdk.Project, showDeploy bool) error {
	// Save .deployhq.toml
	path := config.ProjectConfigPath()
	if err := config.Set(path, "project", project.Permalink); err != nil {
		env.Warn("Could not save config: %v", err)
	} else {
		env.Status("")
		output.ColorGreen.Fprintf(env.Stderr, "Saved to %s\n", path) //nolint:errcheck
	}

	env.Status("")
	env.Status("Next commands:")
	env.Status("  dhq servers list -p %s", project.Permalink)
	if showDeploy {
		env.Status("  dhq deploy -p %s --wait", project.Permalink)
	}
	env.Status("  dhq open %s", project.Permalink)
	return nil
}

func promptString(env *output.Envelope, reader *bufio.Reader, label string) string {
	fmt.Fprintf(env.Stderr, "%s: ", label) //nolint:errcheck
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func detectGitRemote() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
