package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

var (
	styleStage    = lipgloss.NewStyle().Bold(true)
	styleDim      = lipgloss.NewStyle().Faint(true)
	styleSuccess  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	styleRunning  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
)

// watchDeploymentTUI runs a bubbletea TUI that updates steps in-place.
func watchDeploymentTUI(ctx context.Context, client *sdk.Client, env *output.Envelope, projectID, deploymentID string) error {
	m := &watchModel{
		ctx:          ctx,
		client:       client,
		env:          env,
		projectID:    projectID,
		deploymentID: deploymentID,
	}

	p := tea.NewProgram(m, tea.WithOutput(env.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	fm := finalModel.(*watchModel)

	// Print failed logs after TUI exits
	if fm.status == "failed" {
		fmt.Fprintln(env.Stderr) //nolint:errcheck
		for _, s := range fm.steps {
			if s.Status == "failed" && s.Logs {
				logs, err := client.GetDeploymentStepLogs(ctx, projectID, deploymentID, s.Identifier)
				if err == nil && len(logs) > 0 {
					fmt.Fprintf(env.Stderr, "📋 Logs for %s:\n", s.Description) //nolint:errcheck
					start := 0
					if len(logs) > 15 {
						start = len(logs) - 15
					}
					for _, l := range logs[start:] {
						fmt.Fprintf(env.Stderr, "   %s\n", l.Message) //nolint:errcheck
					}
					fmt.Fprintln(env.Stderr) //nolint:errcheck
				}
			}
		}
		fmt.Fprintf(env.Stderr, "Next commands:\n") //nolint:errcheck
		fmt.Fprintf(env.Stderr, "  dhq deployments logs %s -p %s\n", deploymentID, projectID) //nolint:errcheck
		fmt.Fprintf(env.Stderr, "  dhq rollback %s -p %s\n", deploymentID, projectID) //nolint:errcheck
		return &output.UserError{Message: "Deployment failed"}
	}

	return nil
}

// bubbletea messages
type tickMsg time.Time
type spinnerTickMsg time.Time
type deploymentMsg struct {
	dep *sdk.Deployment
	err error
}
type abortMsg struct{ err error }

// Spinner frames (Braille pattern — matches Claude Code and most modern CLIs).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 80 * time.Millisecond

type watchModel struct {
	ctx          context.Context
	client       *sdk.Client
	env          *output.Envelope
	projectID    string
	deploymentID string
	steps        []sdk.DeploymentStep
	status       string
	duration     string
	done         bool
	confirmAbort bool
	aborted      bool
	spinnerFrame int
}

func (m *watchModel) Init() tea.Cmd {
	return tea.Batch(m.fetchDeployment, m.spinnerTick())
}

func (m *watchModel) spinnerTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

func (m *watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmAbort {
			switch msg.String() {
			case "y", "Y":
				m.aborted = true
				m.done = true
				return m, m.abortDeployment
			default:
				m.confirmAbort = false
				return m, nil
			}
		}
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.confirmAbort = true
			return m, nil
		}
	case abortMsg:
		m.done = true
		if msg.err != nil {
			m.status = "abort_failed"
		} else {
			m.status = "cancelled"
		}
		return m, tea.Quit
	case deploymentMsg:
		if msg.err != nil {
			m.done = true
			return m, tea.Quit
		}
		dep := msg.dep
		m.steps = dep.Steps
		m.status = dep.Status

		if dep.Timestamps != nil && dep.Timestamps.Duration != nil {
			m.duration = dep.Timestamps.Duration.String() + "s"
		}

		switch dep.Status {
		case "completed", "failed", "cancelled":
			m.done = true
			return m, tea.Quit
		}

		return m, tea.Tick(pollInterval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case tickMsg:
		return m, m.fetchDeployment
	case spinnerTickMsg:
		if m.done {
			return m, nil
		}
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, m.spinnerTick()
	}
	return m, nil
}

func (m *watchModel) View() string {
	var b strings.Builder

	lastStage := ""
	for _, s := range m.steps {
		if s.Stage != lastStage {
			if lastStage != "" {
				b.WriteString("\n")
			}
			lastStage = s.Stage
			b.WriteString(styleStage.Render(capitalize(s.Stage) + ":"))
			b.WriteString("\n")
		}

		desc := s.Description

		var line string
		switch s.Status {
		case "completed":
			line = fmt.Sprintf("  %s %s", styleSuccess.Render("✓"), styleDim.Render(desc))
		case "running":
			spinner := spinnerFrames[m.spinnerFrame]
			line = fmt.Sprintf("  %s %s", styleRunning.Render(spinner), styleRunning.Render(desc))
		case "failed":
			line = fmt.Sprintf("  %s %s", styleError.Render("✗"), styleError.Render(desc))
		case "skipped":
			line = fmt.Sprintf("  %s %s", styleDim.Render("–"), styleDim.Render(desc))
		default:
			line = fmt.Sprintf("  %s %s", styleDim.Render("·"), styleDim.Render(desc))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")

	switch m.status {
	case "completed":
		msg := "✅ Deployment completed"
		if m.duration != "" {
			msg += " in " + m.duration
		}
		b.WriteString(styleSuccess.Render(msg))
		b.WriteString("\n")
	case "failed":
		b.WriteString(styleError.Render("❌ Deployment failed"))
		b.WriteString("\n")
	case "cancelled":
		b.WriteString(styleRunning.Render("⚠️  Deployment cancelled"))
		b.WriteString("\n")
	case "abort_failed":
		b.WriteString(styleError.Render("⚠️  Could not abort deployment"))
		b.WriteString("\n")
	default:
		if m.confirmAbort {
			b.WriteString(styleRunning.Render("Abort this deployment? (y/n)"))
		} else {
			b.WriteString(styleDim.Render("Waiting for updates... (press q to abort)"))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *watchModel) abortDeployment() tea.Msg {
	err := m.client.AbortDeployment(m.ctx, m.projectID, m.deploymentID)
	return abortMsg{err: err}
}

func (m *watchModel) fetchDeployment() tea.Msg {
	dep, err := m.client.GetDeployment(m.ctx, m.projectID, m.deploymentID)
	return deploymentMsg{dep: dep, err: err}
}
