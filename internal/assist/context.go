package assist

import (
	"context"
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// AssistContext holds deployment data gathered for the LLM prompt.
type AssistContext struct {
	Project     string
	Deployments []deploymentSummary
	Servers     []serverSummary
	FailedLogs  []stepLog
}

type deploymentSummary struct {
	ID       string
	Status   string
	Branch   string
	Deployer string
	QueuedAt string
	Steps    []stepSummary
}

type stepSummary struct {
	Step        string
	Stage       string
	Status      string
	Description string
	HasLogs     bool
}

type serverSummary struct {
	Name       string
	Identifier string
	Protocol   string
	Branch     string
	Enabled    bool
}

type stepLog struct {
	DeploymentID string
	StepName     string
	Messages     []string
}

// GatherContext fetches recent deployment data for a project.
func GatherContext(ctx context.Context, client *sdk.Client, projectID string) (*AssistContext, error) {
	ac := &AssistContext{Project: projectID}

	// Fetch recent deployments
	result, err := client.ListDeployments(ctx, projectID)
	if err != nil {
		return ac, nil // non-fatal
	}

	// Summarize up to 5 recent deployments
	limit := 5
	if len(result.Records) < limit {
		limit = len(result.Records)
	}

	for i, d := range result.Records[:limit] {
		deployer := "-"
		if d.Deployer != nil {
			deployer = *d.Deployer
		}
		queued := "-"
		if d.Timestamps != nil {
			queued = d.Timestamps.QueuedAt
		}

		ds := deploymentSummary{
			ID:       d.Identifier,
			Status:   d.Status,
			Branch:   d.Branch,
			Deployer: deployer,
			QueuedAt: queued,
		}

		// Get full deployment with steps for the most recent ones
		if d.Status == "failed" || i == 0 {
			full, err := client.GetDeployment(ctx, projectID, d.Identifier)
			if err == nil {
				for _, s := range full.Steps {
					ds.Steps = append(ds.Steps, stepSummary{
						Step:        s.Step,
						Stage:       s.Stage,
						Status:      s.Status,
						Description: s.Description,
						HasLogs:     s.Logs,
					})
				}

				// Fetch logs for failed steps
				if d.Status == "failed" {
					for _, s := range full.Steps {
						if s.Status == "failed" && s.Logs {
							logs, err := client.GetDeploymentStepLogs(ctx, projectID, d.Identifier, s.Identifier)
							if err == nil {
								var msgs []string
								for _, l := range logs {
									msgs = append(msgs, l.Message)
								}
								ac.FailedLogs = append(ac.FailedLogs, stepLog{
									DeploymentID: d.Identifier,
									StepName:     s.Description,
									Messages:     msgs,
								})
							}
						}
					}
				}
			}
		}

		ac.Deployments = append(ac.Deployments, ds)
	}

	// Fetch servers
	servers, err := client.ListServers(ctx, projectID)
	if err == nil {
		for _, s := range servers {
			ac.Servers = append(ac.Servers, serverSummary{
				Name:       s.Name,
				Identifier: s.Identifier,
				Protocol:   s.ProtocolType,
				Branch:     s.PreferredBranch,
				Enabled:    s.Enabled,
			})
		}
	}

	return ac, nil
}

// FormatContext renders the context as a string for the LLM prompt.
// Kept concise to fit within small model context windows.
func (ac *AssistContext) FormatContext() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Project: %s\n\n", ac.Project)

	if len(ac.Servers) > 0 {
		b.WriteString("Servers:\n")
		for _, s := range ac.Servers {
			enabled := "enabled"
			if !s.Enabled {
				enabled = "disabled"
			}
			fmt.Fprintf(&b, "  - %s (identifier: %s, protocol: %s, branch: %s, %s)\n", s.Name, s.Identifier, s.Protocol, s.Branch, enabled)
		}
		b.WriteString("\n")
	}

	if len(ac.Deployments) > 0 {
		b.WriteString("Recent deployments:\n")
		for _, d := range ac.Deployments {
			fmt.Fprintf(&b, "  - id=%s status=%s branch=%s deployer=%s queued=%s\n",
				d.ID, d.Status, d.Branch, d.Deployer, d.QueuedAt)
			if len(d.Steps) > 0 {
				b.WriteString("    Steps:\n")
				for _, s := range d.Steps {
					fmt.Fprintf(&b, "      %s [%s] %s - %s\n", s.Step, s.Stage, s.Status, s.Description)
				}
			}
		}
		b.WriteString("\n")
	}

	if len(ac.FailedLogs) > 0 {
		b.WriteString("Failed step logs:\n")
		for _, fl := range ac.FailedLogs {
			fmt.Fprintf(&b, "  --- %s (deployment: %s) ---\n", fl.StepName, fl.DeploymentID)
			// Limit to last 20 log lines to fit context
			start := 0
			if len(fl.Messages) > 20 {
				start = len(fl.Messages) - 20
			}
			for _, msg := range fl.Messages[start:] {
				fmt.Fprintf(&b, "  %s\n", msg)
			}
		}
	}

	return b.String()
}
