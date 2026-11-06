package commands

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
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
		Long: `A project's repository configuration: the SCM type (git, mercurial, etc.), URL, and default branch DeployHQ pulls from. There is exactly one repository per project.

Use these commands to inspect or update the repo, browse branches and commits, and look up the latest revision before deploying.`,
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
			if env.WantsJSON() {
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
		Example: `  # Connect a Git repository over SSH
  dhq repos create -p my-app --scm-type git --url git@github.com:myco/my-app.git --branch main

  # Connect over HTTPS
  dhq repos create -p my-app --scm-type git --url https://github.com/myco/my-app.git`,
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
				return repoConnectError(cliCtx.Background(), client, projectID, scmType, url, branch, err)
			}

			// Fetch project to get deploy key
			project, projErr := client.GetProject(cliCtx.Background(), projectID)

			env := cliCtx.Envelope
			branchErr := branchNotKeptError(cliCtx.Background(), client, projectID, "connected", branch, repo)
			if branchErr == nil {
				branchErr = httpBranchMissingError(cliCtx.Background(), client, projectID, "connected", branch, repo)
			}
			if branchErr != nil {
				// The repository is connected, so the deploy key is still
				// needed once the branch is fixed.
				if projErr == nil && project.PublicKey != "" && !isHTTPRepoURL(url) {
					if env.WantsJSON() {
						branchErr.Recovery = append(branchErr.Recovery, deployKeyBreadcrumbs(url, project.PublicKey, projectID)...)
					} else {
						env.Status("Deploy key (add to your repo for DeployHQ to pull):")
						env.Status("  %s", project.PublicKey)
						printDeployKeyHint(env, url, project.PublicKey, projectID)
						env.Status("")
					}
				}
				return branchErr
			}
			if env.WantsJSON() {
				breadcrumbs := []output.Breadcrumb{}
				if projErr == nil && project.PublicKey != "" && !isHTTPRepoURL(url) {
					breadcrumbs = append(breadcrumbs, deployKeyBreadcrumbs(url, project.PublicKey, projectID)...)
				}
				return env.WriteJSON(output.NewResponse(repo, fmt.Sprintf("Repository created: %s", repo.URL), breadcrumbs...))
			}

			env.Status("Repository created: %s (%s)", repo.URL, repo.ScmType)
			if projErr == nil && project.PublicKey != "" && !isHTTPRepoURL(url) {
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
				return repoUpdateError(cliCtx.Background(), client, projectID, err)
			}
			if branchErr := branchNotKeptError(cliCtx.Background(), client, projectID, "updated", branch, repo); branchErr != nil {
				return branchErr
			}
			if branchErr := httpBranchMissingError(cliCtx.Background(), client, projectID, "updated", branch, repo); branchErr != nil {
				return branchErr
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
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

			branches, err := client.ListBranches(cliCtx.Background(), projectID, nil)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
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

			result, err := client.ListRecentCommits(cliCtx.Background(), projectID, nil)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
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
			if env.WantsJSON() {
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
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"ref": rev}, rev,
					output.Breadcrumb{Action: "deploy", Cmd: fmt.Sprintf("dhq deploy -p %s --revision %s", projectID, rev)},
				))
			}

			fmt.Fprintln(env.Stdout, rev) //nolint:errcheck
			return nil
		},
	}
}

// repoConnectError adds deploy-key instructions to a failed repository
// connection for an SSH URL. DeployHQ checks it can read an SSH repository with
// the project's deploy key when it is connected, so when it refuses access the
// cause is that the key has not been added to the git host yet. Other errors
// pass through as-is.
func repoConnectError(ctx context.Context, client *sdk.Client, projectID, scmType, url, branch string, err error) error {
	if !isSSHRepoURL(url) || !isRepoAccessRejected(err) {
		return err
	}
	project, projErr := client.GetProject(ctx, projectID)
	if projErr != nil || project == nil || project.PublicKey == "" {
		return err
	}
	rerun := fmt.Sprintf("dhq repos create -p %s --scm-type %s --url %s",
		quoteRerunArg(projectID), quoteRerunArg(scmType), quoteRerunArg(url))
	if branch != "" {
		rerun += " --branch " + quoteRerunArg(branch)
	}
	return &output.UserError{
		Message: err.Error(),
		Hint:    deployKeyInstructions(url, project.PublicKey, rerun),
	}
}

// repoUpdateError explains a failed repository update on a project that has no
// repository yet: DeployHQ answers that update with a server error (or, on
// newer servers, a 404), and reading the repository returns a placeholder with
// no URL. Other errors pass through as-is.
func repoUpdateError(ctx context.Context, client *sdk.Client, projectID string, err error) error {
	var apiErr *sdk.APIError
	if !errors.As(err, &apiErr) || (apiErr.StatusCode < 500 && apiErr.StatusCode != 404) {
		return err
	}
	repo, getErr := client.GetRepository(ctx, projectID)
	if getErr != nil || repo == nil || repo.URL != "" {
		return err
	}
	create := fmt.Sprintf("dhq repos create -p %s --scm-type git --url <url> [--branch <branch>]", quoteRerunArg(projectID))
	return &output.UserError{
		Message:  "This project has no repository yet",
		Hint:     create,
		Recovery: []output.Breadcrumb{{Action: "create_repository", Cmd: create}},
	}
}

// maxBranchesShown caps the branch names listed when a branch wasn't kept.
const maxBranchesShown = 20

// branchNotKeptError returns an error when --branch was given and the saved
// repository is on a different branch. DeployHQ does not refuse a branch that
// isn't in the repository: it saves the first branch it finds instead, so a
// mistyped branch would otherwise be reported as a success. done says what
// happened to the repository ("connected", "updated").
func branchNotKeptError(ctx context.Context, client *sdk.Client, projectID, done, requested string, repo *sdk.Repository) *output.UserError {
	if requested == "" || repo == nil || repo.Branch == requested {
		return nil
	}
	project := quoteRerunArg(projectID)

	// A repository with no commits has no branches, so none could be saved.
	if repo.Branch == "" {
		retry := fmt.Sprintf("dhq repos update -p %s --branch %s", project, quoteRerunArg(requested))
		return &output.UserError{
			Message:  fmt.Sprintf("The repository was %s, but it has no branches yet, so %q couldn't be set", done, requested),
			Hint:     "Push a commit to the repository, then run: " + retry + "\nNothing has been deployed.",
			Recovery: []output.Breadcrumb{{Action: "set_branch", Cmd: retry}},
		}
	}

	var branches map[string]string
	if b, err := client.ListBranches(ctx, projectID, nil); err == nil {
		branches = b
	}
	return pickBranchError(projectID,
		fmt.Sprintf("The repository was %s, but on branch %q: %q isn't in it", done, repo.Branch, requested),
		branches)
}

// httpBranchMissingError reports a requested branch that is not in a git
// repository connected over HTTP(S). DeployHQ checks the branch of an SSH
// repository when it saves it (see branchNotKeptError), but for a git URL
// starting with http it skips that check and saves the branch as given, so
// the first deployment would fail. dhq checks it against the repository's
// branch list instead. When the list can't be read (a private repository
// whose credentials don't work yet, a network error) or is empty, nothing is
// reported: the branch can't be shown to be wrong.
func httpBranchMissingError(ctx context.Context, client *sdk.Client, projectID, done, requested string, repo *sdk.Repository) *output.UserError {
	if requested == "" || repo == nil || repo.Branch != requested ||
		repo.ScmType != "git" || !strings.HasPrefix(repo.URL, "http") {
		return nil
	}
	branches, err := client.ListBranches(ctx, projectID, nil)
	if err != nil || len(branches) == 0 {
		return nil
	}
	if _, ok := branches[requested]; ok {
		return nil
	}
	return pickBranchError(projectID,
		fmt.Sprintf("The repository was %s on branch %q, but %q isn't in it", done, requested, requested),
		branches)
}

// pickBranchError is the error for a saved branch that isn't in the
// repository: it lists the branches (at most maxBranchesShown) and gives the
// command to pick one. branches may be nil when the list couldn't be read.
func pickBranchError(projectID, message string, branches map[string]string) *output.UserError {
	project := quoteRerunArg(projectID)
	setBranch := fmt.Sprintf("dhq repos update -p %s --branch <name>", project)
	listBranches := fmt.Sprintf("dhq repos branches -p %s", project)

	hint := "Pick one with: " + setBranch
	if len(branches) > 0 {
		names := make([]string, 0, len(branches))
		for name := range branches {
			names = append(names, name)
		}
		sort.Strings(names)
		list := strings.Join(names, ", ")
		if len(names) > maxBranchesShown {
			list = strings.Join(names[:maxBranchesShown], ", ") +
				fmt.Sprintf(" … and %d more (see: %s)", len(names)-maxBranchesShown, listBranches)
		}
		hint = "Branches: " + list + ". " + hint
	} else {
		hint = "See the branches with: " + listBranches + ". " + hint
	}

	return &output.UserError{
		Message: message,
		Hint:    hint + "\nNothing has been deployed.",
		Recovery: []output.Breadcrumb{
			{Action: "set_branch", Cmd: setBranch},
			{Action: "list_branches", Cmd: listBranches},
		},
	}
}

// isHTTPRepoURL reports whether repoURL is an HTTP(S) URL. Such a repository
// is read with a username and password (or none, when public), never with
// the project's deploy key.
func isHTTPRepoURL(repoURL string) bool {
	u := strings.ToLower(repoURL)
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

// rerunArgSafe matches values that can be pasted into a shell unquoted.
var rerunArgSafe = regexp.MustCompile(`^[A-Za-z0-9@%+=:,./_-]+$`)

// quoteRerunArg makes a value safe to paste into a suggested shell command:
// left bare when it has no shell syntax, otherwise single-quoted with any
// embedded ' written as '\''. A valid git branch such as release&hotfix would
// otherwise run shell syntax when the command is copied.
func quoteRerunArg(s string) string {
	if rerunArgSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// deployKeyBreadcrumbs returns breadcrumbs with CLI commands for adding a deploy key.
func deployKeyBreadcrumbs(repoURL, publicKey, projectID string) []output.Breadcrumb {
	crumbs := []output.Breadcrumb{}

	if isGitHubURL(repoURL) {
		repo := extractGitHubRepo(repoURL)
		if repo != "" {
			crumbs = append(crumbs, output.Breadcrumb{
				Action: "add deploy key via GitHub CLI",
				Cmd:    fmt.Sprintf("echo %s | gh repo deploy-key add -R %s --title DeployHQ -", quoteRerunArg(publicKey), repo),
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
			env.Status("  echo %s | gh repo deploy-key add -R %s --title DeployHQ -", quoteRerunArg(publicKey), repo)
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
