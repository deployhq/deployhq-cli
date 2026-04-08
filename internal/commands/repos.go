package commands

import (
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newReposCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repos",
		Aliases: []string{"repo", "repository"},
		Short:   "Manage repository",
	}

	cmd.AddCommand(
		newReposShowCmd(),
		newReposCreateCmd(),
		newReposUpdateCmd(),
		newReposBranchesCmd(),
		newReposCommitsCmd(),
		newReposCommitInfoCmd(),
		newReposLatestRevisionCmd(),
	)

	return cmd
}

func newReposShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show repository configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			repo, err := client.GetRepository(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(repo,
					fmt.Sprintf("Repository: %s (%s)", repo.URL, repo.ScmType),
					output.Breadcrumb{Action: "branches", Cmd: fmt.Sprintf("dhq repos branches -p %s", projectID)},
					output.Breadcrumb{Action: "commits", Cmd: fmt.Sprintf("dhq repos commits -p %s", projectID)},
				))
			}

			cached := "no"
			if repo.Cached {
				cached = "yes"
			}
			rows := [][]string{
				{"SCM Type", repo.ScmType},
				{"URL", repo.URL},
				{"Branch", repo.Branch},
				{"Cached", cached},
			}
			if repo.HostingService != nil {
				rows = append(rows, []string{"Hosting", repo.HostingService.Name})
			}
			env.WriteTable([]string{"Field", "Value"}, rows)
			env.Status("\nTip: dhq repos branches -p %s", projectID)
			return nil
		},
	}
}

func newReposCreateCmd() *cobra.Command {
	var scmType, url, branch string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create repository configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if scmType == "" || url == "" {
				return &output.UserError{Message: "Both --scm-type and --url are required"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.RepositoryCreateRequest{ScmType: scmType, URL: url, Branch: branch}
			repo, err := client.CreateRepository(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}

			// Fetch project to get deploy key
			project, projErr := client.GetProject(cliCtx.Background(), projectID)

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				breadcrumbs := []output.Breadcrumb{}
				if projErr == nil && project.PublicKey != "" {
					breadcrumbs = append(breadcrumbs, deployKeyBreadcrumbs(url, project.PublicKey, projectID)...)
				}
				return env.WriteJSON(output.NewResponse(repo, fmt.Sprintf("Repository created: %s", repo.URL), breadcrumbs...))
			}

			env.Status("Repository created: %s (%s)", repo.URL, repo.ScmType)
			if projErr == nil && project.PublicKey != "" {
				env.Status("")
				env.Status("Deploy key (add to your repo for DeployHQ to pull):")
				env.Status("  %s", project.PublicKey)
				printDeployKeyHint(env, url, project.PublicKey, projectID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&scmType, "scm-type", "", "SCM type: git, mercurial, subversion (required)")
	cmd.Flags().StringVar(&url, "url", "", "Repository URL (required)")
	cmd.Flags().StringVar(&branch, "branch", "", "Default branch")
	return cmd
}

func newReposUpdateCmd() *cobra.Command {
	var scmType, url, branch string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update repository configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			req := sdk.RepositoryCreateRequest{ScmType: scmType, URL: url, Branch: branch}
			repo, err := client.UpdateRepository(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(repo, "Repository updated",
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq repos show -p %s", projectID)},
					output.Breadcrumb{Action: "branches", Cmd: fmt.Sprintf("dhq repos branches -p %s", projectID)},
				))
			}
			env.Status("Repository updated: %s", repo.URL)
			return nil
		},
	}

	cmd.Flags().StringVar(&scmType, "scm-type", "", "SCM type (git, mercurial, subversion)")
	cmd.Flags().StringVar(&url, "url", "", "Repository URL")
	cmd.Flags().StringVar(&branch, "branch", "", "Default branch")
	return cmd
}

func newReposBranchesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "branches",
		Short: "List repository branches",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			branches, err := client.ListBranches(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(branches, fmt.Sprintf("%d branches", len(branches))))
			}

			for name, sha := range branches {
				short := sha
				if len(short) > 8 {
					short = short[:8]
				}
				fmt.Fprintf(env.Stdout, "%s\t%s\n", name, short) //nolint:errcheck
			}
			return nil
		},
	}
}

func newReposCommitsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commits",
		Short: "List recent commits",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			result, err := client.ListRecentCommits(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(result,
					fmt.Sprintf("%d commits, %d tags", len(result.Commits), len(result.Tags))))
			}

			columns := []string{"Ref", "Author", "Message"}
			rows := make([][]string, len(result.Commits))
			for i, c := range result.Commits {
				ref := c.Ref
				if len(ref) > 8 {
					ref = ref[:8]
				}
				msg := c.ShortMessage
				if msg == "" {
					msg = c.Message
				}
				if len(msg) > 60 {
					msg = msg[:57] + "..."
				}
				rows[i] = []string{ref, c.Author, msg}
			}
			env.WriteTable(columns, rows)
			return nil
		},
	}
}

func newReposCommitInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit-info <ref>",
		Short: "Show details for a specific commit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			commit, err := client.GetCommitInfo(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(commit,
					fmt.Sprintf("Commit: %s", commit.Ref),
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("dhq deploy -p %s --revision %s", projectID, commit.Ref)},
				))
			}

			ref := commit.Ref
			if len(ref) > 8 {
				ref = ref[:8]
			}
			rows := [][]string{
				{"Ref", ref},
				{"Author", commit.Author},
				{"Message", commit.Message},
			}
			if !commit.Timestamp.IsZero() {
				rows = append(rows, []string{"Date", commit.Timestamp.Format("2006-01-02 15:04:05")})
			}
			env.WriteTable([]string{"Field", "Value"}, rows)
			return nil
		},
	}
}

func newReposLatestRevisionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "latest-revision",
		Short: "Show latest revision",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}

			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			rev, err := client.GetLatestRevision(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(map[string]string{"ref": rev}, rev,
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("dhq deploy -p %s --revision %s", projectID, rev)},
				))
			}

			fmt.Fprintln(env.Stdout, rev) //nolint:errcheck
			return nil
		},
	}
}

// deployKeyBreadcrumbs returns breadcrumbs with CLI commands for adding a deploy key.
func deployKeyBreadcrumbs(repoURL, publicKey, projectID string) []output.Breadcrumb {
	crumbs := []output.Breadcrumb{}

	if isGitHubURL(repoURL) {
		repo := extractGitHubRepo(repoURL)
		if repo != "" {
			crumbs = append(crumbs, output.Breadcrumb{
				Action: "add deploy key via GitHub CLI",
				Cmd:    fmt.Sprintf("echo '%s' | gh repo deploy-key add -R %s --title DeployHQ -", publicKey, repo),
			})
		}
	} else if isGitLabURL(repoURL) {
		crumbs = append(crumbs, output.Breadcrumb{
			Action: "add deploy key via GitLab CLI",
			Cmd:    fmt.Sprintf("glab ssh-key add --title DeployHQ --type deploy_key # paste key: %s", publicKey),
		})
	} else if isBitbucketURL(repoURL) {
		crumbs = append(crumbs, output.Breadcrumb{
			Action: "add deploy key",
			Cmd:    "Add at Bitbucket repo > Settings > Access keys > Add key",
		})
	}

	crumbs = append(crumbs, output.Breadcrumb{
		Action: "view deploy key",
		Cmd:    fmt.Sprintf("dhq repos show -p %s", projectID),
	})

	return crumbs
}

// printDeployKeyHint prints human-readable instructions for adding the deploy key.
func printDeployKeyHint(env *output.Envelope, repoURL, publicKey, projectID string) {
	if isGitHubURL(repoURL) {
		repo := extractGitHubRepo(repoURL)
		if repo != "" {
			env.Status("")
			env.Status("Add it with the GitHub CLI:")
			env.Status("  echo '%s' | gh repo deploy-key add -R %s --title DeployHQ -", publicKey, repo)
			return
		}
	}
	if isGitLabURL(repoURL) {
		env.Status("")
		env.Status("Add it with the GitLab CLI:")
		env.Status("  glab ssh-key add --title DeployHQ --type deploy_key")
		return
	}
	if isBitbucketURL(repoURL) {
		env.Status("")
		env.Status("Add it at Bitbucket:")
		env.Status("  repo > Settings > Access keys > Add key")
		return
	}
	env.Status("")
	env.Status("Add it at your repo's Settings > Deploy keys")
}

func isGitHubURL(url string) bool {
	return strings.Contains(url, "github.com")
}

func isGitLabURL(url string) bool {
	return strings.Contains(url, "gitlab.com")
}

func isBitbucketURL(url string) bool {
	return strings.Contains(url, "bitbucket.org")
}
