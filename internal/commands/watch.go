package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

const pollInterval = 3 * time.Second

// watchDeployment polls a deployment until it completes or fails, showing real-time step progress.
func watchDeployment(ctx context.Context, client *sdk.Client, env *output.Envelope, projectID, deploymentID string) error {
	seen := make(map[string]string) // step identifier → last seen status
	lastStage := ""

	for {
		dep, err := client.GetDeployment(ctx, projectID, deploymentID)
		if err != nil {
			return err
		}

		// Show step progress grouped by stage
		for _, s := range dep.Steps {
			prev, exists := seen[s.Identifier]
			if !exists || prev != s.Status {
				seen[s.Identifier] = s.Status

				// Print stage header when it changes
				if s.Stage != lastStage {
					lastStage = s.Stage
					env.Status("\n%s %s:", stageEmoji(s.Stage), capitalize(s.Stage))
				}

				env.Status("  %s %s", stepEmoji(s.Status), s.Description)
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

func stageEmoji(stage string) string {
	switch stage {
	case "preparing":
		return "📦"
	case "building":
		return "🌀"
	case "transferring":
		return "🌐"
	case "finishing":
		return "✨"
	default:
		return "▸"
	}
}

func stepEmoji(status string) string {
	switch status {
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "running":
		return "🔄"
	case "pending":
		return "⏳"
	case "skipped":
		return "⏭️"
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
