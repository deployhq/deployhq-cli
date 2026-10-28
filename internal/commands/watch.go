package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// pollInterval is how often the watchers re-fetch the deployment. A variable
// so tests can shorten it.
var pollInterval = 3 * time.Second

// fetchRetryDelays are the waits before each retry of a status check that
// failed for a reason that may pass. A variable so tests can shorten them.
var fetchRetryDelays = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

// getDeploymentRetrying fetches a deployment, retrying a check that failed for
// a reason that may pass (see retryableCheckError) after each of
// fetchRetryDelays, or after a 429's Retry-After when that is longer. It stops
// waiting as soon as ctx ends.
func getDeploymentRetrying(ctx context.Context, client *sdk.Client, projectID, deploymentID string) (*sdk.Deployment, error) {
	for attempt := 0; ; attempt++ {
		dep, err := client.GetDeployment(ctx, projectID, deploymentID)
		if err == nil || ctx.Err() != nil || attempt >= len(fetchRetryDelays) || !retryableCheckError(err) {
			return dep, err
		}
		delay := fetchRetryDelays[attempt]
		var apiErr *sdk.APIError
		if errors.As(err, &apiErr) && time.Duration(apiErr.RetryAfter)*time.Second > delay {
			delay = time.Duration(apiErr.RetryAfter) * time.Second
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(delay):
		}
	}
}

// retryableCheckError reports whether a failed status check may pass if
// repeated: a network error, a server error (5xx) or rate limiting (429).
// Any other API error (wrong key, suspended account, deleted deployment) is
// final.
func retryableCheckError(err error) bool {
	var apiErr *sdk.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500
	}
	return output.IsNetworkErr(err)
}

// deploymentCheckError reports that the deployment's status could no longer
// be checked after retrying. The deployment itself may be fine and still
// running, so the error says so and how to check it later.
func deploymentCheckError(projectID, deploymentID, lastStatus string, err error) error {
	if !retryableCheckError(err) {
		return err
	}
	if lastStatus == "" {
		lastStatus = "unknown"
	}
	return &output.UserError{
		Message: fmt.Sprintf("Could not check deployment %s (last status: %s): %v. The deployment may still be running.", deploymentID, lastStatus, err),
		Hint:    fmt.Sprintf("dhq deployments show %s -p %s --json=status", deploymentID, projectID),
	}
}

// watchDeployment uses the TUI in interactive terminals, falls back to append-only otherwise.
func watchDeployment(ctx context.Context, client *sdk.Client, env *output.Envelope, projectID, deploymentID string) error {
	if env.IsTTY && !env.WantsJSON() {
		return watchDeploymentTUI(ctx, client, env, projectID, deploymentID)
	}
	return watchDeploymentPlain(ctx, client, env, projectID, deploymentID)
}

// watchDeploymentPlain is the append-only fallback for non-TTY/JSON mode.
// It returns nil only when the deployment completed; any other terminal
// status (failed, cancelled) is an error so the exit code is non-zero.
func watchDeploymentPlain(ctx context.Context, client *sdk.Client, env *output.Envelope, projectID, deploymentID string) error {
	dep, err := followDeploymentPlain(ctx, client, env, projectID, deploymentID)
	if err != nil {
		return err
	}
	return deploymentOutcomeError(dep)
}

// deploymentOutcomeError maps a deployment in a terminal state to the command
// result: nil for "completed", a UserError naming the final status otherwise.
func deploymentOutcomeError(dep *sdk.Deployment) error {
	if dep.Status == "completed" {
		return nil
	}
	return &output.UserError{Message: deploymentNotCompletedMessage(dep)}
}

func deploymentNotCompletedMessage(dep *sdk.Deployment) string {
	return fmt.Sprintf("Deployment %s finished with status %q (not completed)", dep.Identifier, dep.Status)
}

// followDeploymentPlain polls a deployment, printing append-only progress to
// stderr, until it reaches a terminal status. It returns the final deployment.
// If ctx ends first it returns the last deployment fetched (possibly nil)
// together with ctx.Err().
func followDeploymentPlain(ctx context.Context, client *sdk.Client, env *output.Envelope, projectID, deploymentID string) (*sdk.Deployment, error) {
	var last *sdk.Deployment
	printed := make(map[string]bool) // step identifier → printed as terminal
	stageShown := make(map[string]bool)
	shownRunning := "" // identifier of step currently shown as "running"

	for {
		dep, err := getDeploymentRetrying(ctx, client, projectID, deploymentID)
		if err != nil {
			if ctx.Err() != nil {
				return last, ctx.Err()
			}
			lastStatus := ""
			if last != nil {
				lastStatus = last.Status
			}
			return last, deploymentCheckError(projectID, deploymentID, lastStatus, err)
		}
		last = dep

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
			return dep, nil
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
						// De-duplicate identical lines: a static deploy can record
						// the same advisory/log line more than once, which would
						// otherwise print twice here (the web UI shows each once).
						seen := make(map[string]bool)
						for _, l := range logs[start:] {
							if seen[l.Message] {
								continue
							}
							seen[l.Message] = true
							env.Status("   %s", l.Message)
						}
					}
				}
			}

			env.Status("")
			env.Status("Next commands:")
			env.Status("  dhq deployments logs %s -p %s", deploymentID, projectID)
			env.Status("  dhq rollback %s -p %s", deploymentID, projectID)
			return dep, nil
		case "cancelled":
			env.Status("")
			env.Status("⚠️  Deployment cancelled")
			return dep, nil
		}

		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
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
