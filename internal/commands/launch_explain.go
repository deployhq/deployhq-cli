package commands

import (
	"context"

	"github.com/deployhq/deployhq-cli/internal/assist"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
)

// explainLaunchFailure offers a local-AI diagnosis of a just-failed deployment,
// reusing the same Ollama-backed assistant as `dhq assist` (which already
// gathers the failed step's logs as context). It closes the failure loop without
// a round-trip to the DeployHQ API — nothing leaves the machine.
//
// It is INTERACTIVE-ONLY and best-effort:
//   - In non-interactive / --json mode it does nothing; the structured
//     launchError is the machine-readable output and must not be cluttered.
//   - When Ollama isn't running it prints a one-line tip pointing at `dhq assist`.
//   - Any error (context gathering, streaming) is swallowed — the diagnosis is
//     an aid, never a gate.
func explainLaunchFailure(ctx context.Context, env *output.Envelope, client *sdk.Client, projectID string) {
	if projectID == "" || env == nil || env.NonInteractive || env.WantsJSON() || !env.IsTTY {
		return
	}

	ollama := assist.NewOllamaClient()
	if !ollama.IsAvailable(ctx) {
		env.Status("")
		env.Status("Tip: run 'dhq assist -p %s' for a local AI diagnosis of this failure (requires Ollama).", projectID)
		return
	}

	ac, err := assist.GatherContext(ctx, client, projectID)
	if err != nil {
		return
	}

	const question = "The most recent deployment just failed. Using the failed step's logs, " +
		"explain the most likely root cause in 2-3 sentences, then give the exact fix " +
		"(the commands to run or the DeployHQ setting to change). Be concise and specific."
	messages := assist.BuildMessages(ac.FormatContext(), question)

	env.Status("")
	output.ColorCyan.Fprint(env.Stderr, "AI diagnosis (local Ollama):\n") //nolint:errcheck
	if streamErr := ollama.ChatStream(ctx, messages, env.Stderr); streamErr != nil {
		env.Logger.Write("ollama diagnosis failed: %v", streamErr)
	}
	env.Status("")
}
