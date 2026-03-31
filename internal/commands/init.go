package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

// Styles
var (
	initTitle     = lipgloss.NewStyle().Bold(true)
	initSuccess   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	initDim       = lipgloss.NewStyle().Faint(true)
	initHighlight = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	initError     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// Steps
const (
	stepProjectName = iota
	stepSCMType
	stepRepoURL
	stepBranch
	stepDeployKeyAuto
	stepDeployKeyManual
	stepProtocol
	stepServerName
	stepHostname
	stepUsername
	stepPassword
	stepServerPath
	stepDeployConfirm
	stepDone
)

type initModel struct {
	step    int
	cursor  int // for select lists
	input   string
	err     string
	client  *sdk.Client
	env     *output.Envelope

	// Collected values
	projectName string
	scmType     string
	repoURL     string
	branch      string
	protocol    string
	serverName  string
	hostname    string
	username    string
	password    string
	serverPath  string
	deployNow   bool

	// Created resources
	project *sdk.Project
	server  *sdk.Server
	repoOK  bool

	// Detected
	detectedRemote string

	// Status
	creating         bool
	quitting         bool
	showAllProtocols bool
	ghAvailable      bool
	deployKeyAdded   bool
}

var scmTypes = []string{"git", "mercurial", "subversion"}
var protocolsCommon = []string{
	"SSH/SFTP",
	"FTP",
	"FTPS (SSL/TLS)",
	"More protocols...",
}

var protocolsAll = []string{
	"SSH/SFTP",
	"FTP",
	"FTPS (SSL/TLS)",
	"Rsync",
	"Amazon S3",
	"S3-Compatible Storage",
	"DigitalOcean",
	"Hetzner Cloud",
	"Heroku",
	"Netlify",
	"Shopify",
}

// protocolAPIType maps display names to API protocol_type values.
var protocolAPIType = map[string]string{
	"SSH/SFTP":               "ssh",
	"FTP":                    "ftp",
	"FTPS (SSL/TLS)":        "ftps",
	"Rsync":                  "rsync",
	"Amazon S3":              "s3",
	"S3-Compatible Storage":  "s3_compatible",
	"DigitalOcean":           "digitalocean",
	"Hetzner Cloud":          "hetzner_cloud",
	"Heroku":                 "heroku",
	"Netlify":                "netlify",
	"Shopify":                "shopify",
}

// protocolNeedsHost returns true if the protocol requires hostname/username/password.
func protocolNeedsHost(proto string) bool {
	switch proto {
	case "ssh", "ftp", "ftps", "rsync":
		return true
	}
	return false
}

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

			_, ghErr := exec.LookPath("gh")
			m := &initModel{
				step:           stepProjectName,
				client:         client,
				env:            env,
				branch:         "main",
				serverName:     "production",
				detectedRemote: detectGitRemote(),
				ghAvailable:    ghErr == nil,
			}

			p := tea.NewProgram(m, tea.WithOutput(env.Stderr))
			finalModel, err := p.Run()
			if err != nil {
				return err
			}

			fm := finalModel.(*initModel)
			if fm.quitting && fm.project == nil {
				return nil // cancelled before creating anything
			}

			// Post-TUI actions (these need real I/O)
			if fm.project != nil {
				// Save config
				path := config.ProjectConfigPath()
				_ = config.Set(path, "project", fm.project.Permalink)

				// Deploy if requested
				if fm.deployNow && fm.server != nil {
					fmt.Fprintln(env.Stderr) //nolint:errcheck
					ctx := cliCtx.Background()

					// Fetch latest revision
					env.Status("Fetching latest revision...")
					rev, revErr := client.GetLatestRevision(ctx, fm.project.Permalink)
					if revErr != nil {
						env.Warn("Could not fetch latest revision: %v", revErr)
					}

					dep, err := client.CreateDeployment(ctx, fm.project.Permalink, sdk.DeploymentCreateRequest{
						ParentIdentifier: fm.server.Identifier,
						EndRevision:      rev,
					})
					if err != nil {
						env.Warn("Deploy failed: %v", err)
					} else {
						env.Status("🚀 Deployment %s queued", dep.Identifier)
						env.Status("")
						_ = watchDeployment(cliCtx.Background(), client, env, fm.project.Permalink, dep.Identifier)
					}
				}

				fmt.Fprintln(env.Stderr) //nolint:errcheck
				fmt.Fprintln(env.Stderr, initSuccess.Render("Saved to .deployhq.toml")) //nolint:errcheck
				fmt.Fprintln(env.Stderr) //nolint:errcheck
				env.Status("Next commands:")
				if fm.server != nil {
					env.Status("  dhq deploy -p %s --wait", fm.project.Permalink)
				} else {
					env.Status("  dhq servers create -p %s --name production --protocol ssh", fm.project.Permalink)
				}
				env.Status("  dhq open %s", fm.project.Permalink)
			}

			return nil
		},
	}
}

// -- bubbletea lifecycle --

type createResultMsg struct {
	project *sdk.Project
	server  *sdk.Server
	repoOK  bool
	err     error
	step    int
}

func (m *initModel) Init() tea.Cmd {
	return nil
}

func (m *initModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case createResultMsg:
		m.creating = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		switch msg.step {
		case stepProjectName:
			m.project = msg.project
			m.step = stepSCMType
		case stepRepoURL:
			m.repoOK = msg.repoOK
			if m.isPrivateRepo() {
				if m.ghAvailable {
					m.step = stepDeployKeyAuto
					m.cursor = 0
				} else {
					m.step = stepDeployKeyManual
				}
			} else {
				m.step = stepProtocol
			}
		case stepDeployKeyAuto:
			m.deployKeyAdded = msg.err == nil
			m.step = stepProtocol
		case stepServerName:
			m.server = msg.server
			m.step = stepDeployConfirm
		}
		m.input = ""
		m.cursor = 0
		m.err = ""
		return m, nil

	case tea.KeyMsg:
		// Handle paste (bracketed paste comes as KeyMsg with Paste=true)
		if msg.Paste {
			m.input += strings.TrimSpace(msg.String())
			return m, nil
		}
		m.err = ""
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *initModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.step {
	case stepProjectName:
		return m.handleTextInput(key, "")
	case stepSCMType:
		return m.handleSelect(key, scmTypes, stepProjectName, func() {
			m.scmType = scmTypes[m.cursor]
			if m.detectedRemote != "" {
				m.input = m.detectedRemote
			}
			m.step = stepRepoURL
		})
	case stepRepoURL:
		return m.handleTextInput(key, "back")
	case stepBranch:
		return m.handleTextInput(key, "back")
	case stepDeployKeyAuto:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 1 {
				m.cursor++
			}
		case "enter":
			if m.cursor == 0 {
				m.creating = true
				return m, m.addDeployKeyViaGH
			}
			m.step = stepDeployKeyManual
		case "backspace", "left":
			m.step = stepBranch
			m.cursor = 0
		}
		return m, nil
	case stepDeployKeyManual:
		switch key {
		case "enter":
			m.step = stepProtocol
			m.cursor = 0
		case "backspace", "left":
			m.step = stepBranch
		}
		return m, nil
	case stepProtocol:
		backTo := stepBranch
		if m.repoURL == "" {
			backTo = stepRepoURL
		}
		items := protocolsCommon
		if m.showAllProtocols {
			items = protocolsAll
		}
		return m.handleSelect(key, items, backTo, func() {
			selected := items[m.cursor]
			if selected == "More protocols..." {
				m.showAllProtocols = true
				m.cursor = 0
				return
			}
			m.protocol = protocolAPIType[selected]
			m.input = m.serverName
			m.step = stepServerName
		})
	case stepServerName:
		return m.handleTextInput(key, "back-protocol")
	case stepHostname:
		return m.handleTextInput(key, "back")
	case stepUsername:
		return m.handleTextInput(key, "back")
	case stepPassword:
		return m.handleTextInput(key, "back")
	case stepServerPath:
		return m.handleTextInput(key, "back")
	case stepDeployConfirm:
		return m.handleSelect(key, []string{"Yes", "No"}, stepServerPath, func() {
			m.deployNow = m.cursor == 0
			m.step = stepDone
		})
	case stepDone:
		if key == "enter" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *initModel) handleTextInput(key string, backTarget string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Strip bracketed paste artifacts
		m.input = strings.TrimPrefix(m.input, "[")
		m.input = strings.TrimSuffix(m.input, "]")
		return m.submitTextStep()
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		} else if backTarget != "" {
			return m.goBack(backTarget)
		}
	case "left":
		if m.input == "" && backTarget != "" {
			return m.goBack(backTarget)
		}
	default:
		if len(key) == 1 {
			m.input += key
		}
	}
	return m, nil
}

func (m *initModel) handleSelect(key string, items []string, backStep int, onSelect func()) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(items)-1 {
			m.cursor++
		}
	case "enter":
		onSelect()
	case "backspace", "left":
		m.step = backStep
		m.cursor = 0
		m.input = ""
	}
	return m, nil
}

func (m *initModel) goBack(target string) (tea.Model, tea.Cmd) {
	switch target {
	case "back":
		m.step--
	case "back-protocol":
		m.step = stepProtocol
	default:
		m.step--
	}
	m.input = ""
	m.cursor = 0
	return m, nil
}

func (m *initModel) submitTextStep() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepProjectName:
		m.projectName = m.input
		if m.projectName == "" {
			m.err = "Project name is required"
			return m, nil
		}
		m.creating = true
		return m, m.createProject
	case stepRepoURL:
		m.repoURL = m.input
		if m.repoURL == "" {
			// Skip repo, go to server
			m.step = stepProtocol
			m.cursor = 0
			return m, nil
		}
		m.input = m.branch
		m.step = stepBranch
	case stepBranch:
		if m.input != "" {
			m.branch = m.input
		}
		m.creating = true
		return m, m.createRepo
	case stepServerName:
		if m.input != "" {
			m.serverName = m.input
		}
		m.input = ""
		if protocolNeedsHost(m.protocol) {
			m.step = stepHostname
		} else {
			m.step = stepServerPath
		}
	case stepHostname:
		m.hostname = m.input
		if m.hostname == "" {
			m.err = "Hostname is required"
			return m, nil
		}
		m.input = ""
		m.step = stepUsername
	case stepUsername:
		m.username = m.input
		m.input = ""
		m.step = stepPassword
	case stepPassword:
		m.password = m.input
		m.input = ""
		m.step = stepServerPath
	case stepServerPath:
		m.serverPath = m.input
		m.creating = true
		return m, m.createServer
	}
	return m, nil
}

// -- API calls --

func (m *initModel) createProject() tea.Msg {
	ctx := context.Background()
	project, err := m.client.CreateProject(ctx, sdk.ProjectCreateRequest{Name: m.projectName})
	return createResultMsg{project: project, err: err, step: stepProjectName}
}

func (m *initModel) createRepo() tea.Msg {
	ctx := context.Background()
	_, err := m.client.CreateRepository(ctx, m.project.Permalink, sdk.RepositoryCreateRequest{
		ScmType: m.scmType, URL: m.repoURL, Branch: m.branch,
	})
	return createResultMsg{repoOK: err == nil, err: err, step: stepRepoURL}
}

func (m *initModel) createServer() tea.Msg {
	ctx := context.Background()
	server, err := m.client.CreateServer(ctx, m.project.Permalink, sdk.ServerCreateRequest{
		Name:         m.serverName,
		ProtocolType: m.protocol,
		Hostname:     m.hostname,
		Username:     m.username,
		Password:     m.password,
		ServerPath:   m.serverPath,
	})
	return createResultMsg{server: server, err: err, step: stepServerName}
}

func (m *initModel) addDeployKeyViaGH() tea.Msg {
	// Extract repo owner/name from URL (e.g. git@github.com:owner/repo.git)
	repo := extractGitHubRepo(m.repoURL)
	if repo == "" {
		return createResultMsg{err: fmt.Errorf("could not extract GitHub repo from URL: %s", m.repoURL), step: stepDeployKeyAuto}
	}

	// Write public key to temp file
	tmpFile, err := os.CreateTemp("", "dhq-deploy-key-*.pub")
	if err != nil {
		return createResultMsg{err: err, step: stepDeployKeyAuto}
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck

	if _, err := tmpFile.WriteString(m.project.PublicKey); err != nil {
		tmpFile.Close() //nolint:errcheck
		return createResultMsg{err: err, step: stepDeployKeyAuto}
	}
	tmpFile.Close() //nolint:errcheck

	// Run gh repo deploy-key add
	cmd := exec.Command("gh", "repo", "deploy-key", "add", tmpFile.Name(),
		"--repo", repo,
		"--title", fmt.Sprintf("DeployHQ - %s", m.project.Name))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return createResultMsg{err: fmt.Errorf("%s", strings.TrimSpace(string(output))), step: stepDeployKeyAuto}
	}

	return createResultMsg{err: nil, step: stepDeployKeyAuto}
}

func extractGitHubRepo(url string) string {
	// git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@github.com:") {
		repo := strings.TrimPrefix(url, "git@github.com:")
		repo = strings.TrimSuffix(repo, ".git")
		return repo
	}
	// https://github.com/owner/repo.git
	if strings.Contains(url, "github.com/") {
		parts := strings.SplitN(url, "github.com/", 2)
		if len(parts) == 2 {
			repo := strings.TrimSuffix(parts[1], ".git")
			return repo
		}
	}
	return ""
}

// -- View --

func (m *initModel) View() string {
	var b strings.Builder

	b.WriteString(initTitle.Render("🚀 DeployHQ Project Setup"))
	b.WriteString("\n\n")

	switch m.step {
	case stepProjectName:
		m.viewStepHeader(&b, "1/4", "Project")
		m.viewTextPrompt(&b, "Project name", m.input)
	case stepSCMType:
		m.viewCompleted(&b, "1/4", "Project", fmt.Sprintf("✅ %s", m.project.Name))
		m.viewStepHeader(&b, "2/4", "Repository")
		m.viewSelect(&b, "SCM type", scmTypes)
	case stepRepoURL:
		m.viewCompleted(&b, "1/4", "Project", fmt.Sprintf("✅ %s", m.project.Name))
		m.viewStepHeader(&b, "2/4", "Repository")
		b.WriteString(initDim.Render(fmt.Sprintf("  SCM: %s", m.scmType)))
		b.WriteString("\n")
		if m.detectedRemote != "" && m.input == m.detectedRemote {
			b.WriteString(initDim.Render(fmt.Sprintf("  Detected: %s", m.detectedRemote)))
			b.WriteString("\n")
		}
		m.viewTextPrompt(&b, "Repository URL (empty to skip)", m.input)
	case stepBranch:
		m.viewCompleted(&b, "1/4", "Project", fmt.Sprintf("✅ %s", m.project.Name))
		m.viewStepHeader(&b, "2/4", "Repository")
		b.WriteString(initDim.Render(fmt.Sprintf("  SCM: %s | URL: %s", m.scmType, m.repoURL)))
		b.WriteString("\n")
		m.viewTextPrompt(&b, "Default branch", m.input)
	case stepDeployKeyAuto:
		m.viewCompleted(&b, "1/4", "Project", fmt.Sprintf("✅ %s", m.project.Name))
		m.viewCompleted(&b, "2/4", "Repository", "✅ Connected")
		b.WriteString("\n")
		b.WriteString(initHighlight.Render("  🔑 Deploy key required for private repo"))
		b.WriteString("\n\n")
		if m.deployKeyAdded {
			b.WriteString(initSuccess.Render("  ✅ Deploy key added via GitHub CLI"))
			b.WriteString("\n")
		} else {
			m.viewSelect(&b, "Add deploy key automatically?", []string{"Yes, add via GitHub CLI", "No, I'll add it manually"})
		}
	case stepDeployKeyManual:
		m.viewCompleted(&b, "1/4", "Project", fmt.Sprintf("✅ %s", m.project.Name))
		m.viewCompleted(&b, "2/4", "Repository", "✅ Connected")
		b.WriteString("\n")
		b.WriteString(initHighlight.Render("  Add this deploy key to your repository:"))
		b.WriteString("\n\n")
		fmt.Fprintf(&b, "  %s", m.project.PublicKey)
		b.WriteString("\n\n")
		b.WriteString(initDim.Render("  GitHub:    repo → Settings → Deploy keys → Add deploy key"))
		b.WriteString("\n")
		b.WriteString(initDim.Render("  GitLab:    repo → Settings → Repository → Deploy keys"))
		b.WriteString("\n")
		b.WriteString(initDim.Render("  Bitbucket: repo → Settings → Access keys"))
		b.WriteString("\n\n")
		b.WriteString("  Press Enter once you've added the key...")
		b.WriteString("\n")
	case stepProtocol:
		m.viewCompleted(&b, "1/4", "Project", fmt.Sprintf("✅ %s", m.project.Name))
		if m.repoOK {
			m.viewCompleted(&b, "2/4", "Repository", "✅ Connected")
		} else if m.repoURL != "" {
			m.viewCompleted(&b, "2/4", "Repository", "⚠️ Failed")
		} else {
			m.viewCompleted(&b, "2/4", "Repository", "⏭️ Skipped")
		}
		m.viewStepHeader(&b, "3/4", "Server")
		items := protocolsCommon
		if m.showAllProtocols {
			items = protocolsAll
		}
		m.viewSelect(&b, "Protocol", items)
	case stepServerName:
		m.viewCompletedSteps(&b)
		m.viewStepHeader(&b, "3/4", "Server")
		b.WriteString(initDim.Render(fmt.Sprintf("  Protocol: %s", m.protocol)))
		b.WriteString("\n")
		m.viewTextPrompt(&b, "Server name", m.input)
	case stepHostname:
		m.viewCompletedSteps(&b)
		m.viewStepHeader(&b, "3/4", "Server")
		b.WriteString(initDim.Render(fmt.Sprintf("  Protocol: %s | Name: %s", m.protocol, m.serverName)))
		b.WriteString("\n")
		m.viewTextPrompt(&b, "Hostname", m.input)
	case stepUsername:
		m.viewCompletedSteps(&b)
		m.viewStepHeader(&b, "3/4", "Server")
		b.WriteString(initDim.Render(fmt.Sprintf("  Protocol: %s | Name: %s | Host: %s", m.protocol, m.serverName, m.hostname)))
		b.WriteString("\n")
		m.viewTextPrompt(&b, "Username", m.input)
	case stepPassword:
		m.viewCompletedSteps(&b)
		m.viewStepHeader(&b, "3/4", "Server")
		b.WriteString(initDim.Render(fmt.Sprintf("  Protocol: %s | Name: %s | Host: %s | User: %s", m.protocol, m.serverName, m.hostname, m.username)))
		b.WriteString("\n")
		masked := strings.Repeat("*", len(m.input))
		m.viewTextPrompt(&b, "Password", masked)
	case stepServerPath:
		m.viewCompletedSteps(&b)
		m.viewStepHeader(&b, "3/4", "Server")
		summary := fmt.Sprintf("  Protocol: %s | Name: %s", m.protocol, m.serverName)
		if m.hostname != "" {
			summary += fmt.Sprintf(" | Host: %s", m.hostname)
		}
		b.WriteString(initDim.Render(summary))
		b.WriteString("\n")
		m.viewTextPrompt(&b, "Server path (e.g. /var/www/my-app)", m.input)
	case stepDeployConfirm:
		m.viewCompletedSteps(&b)
		m.viewCompleted(&b, "3/4", "Server", fmt.Sprintf("✅ %s (%s)", m.server.Name, m.protocol))
		m.viewStepHeader(&b, "4/4", "Deploy")
		m.viewSelect(&b, "Deploy now?", []string{"Yes", "No"})
	case stepDone:
		m.viewCompletedSteps(&b)
		if m.server != nil {
			m.viewCompleted(&b, "3/4", "Server", fmt.Sprintf("✅ %s", m.server.Name))
		}
		b.WriteString("\n")
		b.WriteString(initSuccess.Render("Setup complete!"))
		b.WriteString("\n")
	}

	if m.creating {
		b.WriteString("\n")
		b.WriteString(initDim.Render("  Creating..."))
		b.WriteString("\n")
	}

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(initError.Render(fmt.Sprintf("  Error: %s", m.err)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(initDim.Render("  ← backspace to go back • enter to continue • esc to quit"))
	b.WriteString("\n")

	return b.String()
}

func (m *initModel) viewStepHeader(b *strings.Builder, num, name string) {
	b.WriteString(initTitle.Render(fmt.Sprintf("Step %s — %s", num, name)))
	b.WriteString("\n")
}

func (m *initModel) viewCompleted(b *strings.Builder, num, name, status string) {
	b.WriteString(initDim.Render(fmt.Sprintf("Step %s — %s: %s", num, name, status)))
	b.WriteString("\n")
}

func (m *initModel) viewTextPrompt(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "  %s: %s", label, value)
	b.WriteString(initHighlight.Render("▊")) // cursor
	b.WriteString("\n")
}

func (m *initModel) viewSelect(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "  %s:\n", label)
	for i, item := range items {
		if i == m.cursor {
			b.WriteString(initHighlight.Render(fmt.Sprintf("  ▸ %s", item)))
		} else {
			b.WriteString(fmt.Sprintf("    %s", item))
		}
		b.WriteString("\n")
	}
}

func (m *initModel) viewCompletedSteps(b *strings.Builder) {
	if m.project != nil {
		m.viewCompleted(b, "1/4", "Project", fmt.Sprintf("✅ %s", m.project.Name))
	}
	if m.repoOK {
		m.viewCompleted(b, "2/4", "Repository", "✅ Connected")
	} else if m.repoURL != "" {
		m.viewCompleted(b, "2/4", "Repository", "⚠️ Failed")
	} else {
		m.viewCompleted(b, "2/4", "Repository", "⏭️ Skipped")
	}
}

func (m *initModel) isPrivateRepo() bool {
	return strings.HasPrefix(m.repoURL, "git@") || strings.HasPrefix(m.repoURL, "ssh://")
}

func detectGitRemote() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
