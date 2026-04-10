package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

const pollInterval = 3 * time.Second

// watchDeployment uses the TUI in interactive terminals, falls back to append-only otherwise.
func watchDeployment(ctx context.Context, client *sdk.Client, env *output.Envelope, projectID, deploymentID string) error {
	if env.IsTTY && !env.JSONMode {
		return watchDeploymentTUI(ctx, client, env, projectID, deploymentID)
	}
	return watchDeploymentPlain(ctx, client, env, projectID, deploymentID)
}

// watchDeploymentPlain is the append-only fallback for non-TTY/JSON mode.
func watchDeploymentPlain(ctx context.Context, client *sdk.Client, env *output.Envelope, projectID, deploymentID string) error {
	printed := make(map[string]bool) // step identifier → printed as terminal
	stageShown := make(map[string]bool)
	shownRunning := "" // identifier of step currently shown as "running"

	for {
		dep, err := client.GetDeployment(ctx, projectID, deploymentID)
		if err != nil {
			return err
		}

		for _, s := range dep.Steps {
			isTerminal := s.Status == "completed" || s.Status == "failed" || s.Status == "skipped"

			// Print terminal steps once
			if isTerminal && !printed[s.Identifier] {
				if !stageShown[s.Stage] {
					stageShown[s.Stage] = true
					env.Status("\n%s:", capitalize(s.Stage))
				}
				env.Status("  %s %s", stepEmoji(s.Status), s.Description)
				printed[s.Identifier] = true

				// Clear running indicator if this was the running step
				if shownRunning == s.Identifier {
					shownRunning = ""
				}
			}

			// Show the first running step (only one at a time, only once)
			if s.Status == "running" && shownRunning != s.Identifier && !printed[s.Identifier] {
				if !stageShown[s.Stage] {
					stageShown[s.Stage] = true
					env.Status("\n%s:", capitalize(s.Stage))
				}
				env.Status("  %s %s...", stepEmoji("running"), s.Description)
				shownRunning = s.Identifier
			}
		}

		// Check terminal states
		switch dep.Status {
		case "completed":
			duration := ""
			if dep.Timestamps != nil && dep.Timestamps.Duration != nil {
				duration = fmt.Sprintf(" in %ss", dep.Timestamps.Duration.String())
			}
			env.Status("")
			env.Status("✅ Deployment completed%s", duration)
			return nil
		case "failed":
			env.Status("")
			env.Status("❌ Deployment failed")

			// Auto-fetch logs for failed steps
			for _, s := range dep.Steps {
				if s.Status == "failed" && s.Logs {
					logs, err := client.GetDeploymentStepLogs(ctx, projectID, deploymentID, s.Identifier)
					if err == nil && len(logs) > 0 {
						env.Status("")
						env.Status("📋 Logs for %s:", s.Description)
						start := 0
						if len(logs) > 15 {
							start = len(logs) - 15
						}
						for _, l := range logs[start:] {
							env.Status("   %s", l.Message)
						}
					}
				}
			}

			env.Status("")
			env.Status("Next commands:")
			env.Status("  dhq deployments logs %s -p %s", deploymentID, projectID)
			env.Status("  dhq rollback %s -p %s", deploymentID, projectID)
			return &output.UserError{Message: "Deployment failed"}
		case "cancelled":
			env.Status("")
			env.Status("⚠️  Deployment cancelled")
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// stageEmoji is retained for backward-compat with the non-TTY fallback output,
// but the TUI now renders stage headers without an icon prefix.
func stageEmoji(stage string) string {
	return ""
}

func stepEmoji(status string) string {
	switch status {
	case "completed":
		return "✓"
	case "failed":
		return "✗"
	case "running":
		return "⋮"
	case "pending":
		return "·"
	case "skipped":
		return "–"
	default:
		return "·"
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
