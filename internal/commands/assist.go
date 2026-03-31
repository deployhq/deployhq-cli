package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/assist"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAssistCmd() *cobra.Command {
	var model string
	var setup, status, noStream bool

	cmd := &cobra.Command{
		Use:   "assist [question]",
		Short: "AI-powered deployment assistant (requires Ollama)",
		Long: `Ask questions about your deployments using a local AI model.

Requires Ollama running locally. Run 'dhq assist --setup' to get started.
All data stays on your machine — nothing is sent to external services.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			ollama := assist.NewOllamaClient()
			if model != "" {
				ollama.Model = model
			}

			// --setup: guide Ollama installation
			if setup {
				return runAssistSetup(env, ollama)
			}

			// --status: check Ollama health
			if status {
				return runAssistStatus(env, ollama)
			}

			// Need a question
			question := strings.Join(args, " ")
			if question == "" {
				return &output.UserError{
					Message: "Please ask a question",
					Hint:    `Example: dhq assist "why did my deploy fail?" -p <project>`,
				}
			}

			// Check Ollama is available
			if !ollama.IsAvailable(cliCtx.Background()) {
				return &output.UserError{
					Message: "Ollama is not running",
					Hint:    "Run 'dhq assist --setup' to install and configure Ollama",
				}
			}

			// Gather deployment context
			var contextStr string
			projectID := cliCtx.Config.Project
			if projectID != "" {
				client, err := cliCtx.Client()
				if err == nil {
					env.Status("Gathering deployment context...")
					ac, err := assist.GatherContext(cliCtx.Background(), client, projectID)
					if err == nil {
						contextStr = ac.FormatContext()
					}
				}
			}

			if contextStr == "" {
				contextStr = "No project context available. Answer based on general DeployHQ knowledge."
			}

			messages := assist.BuildMessages(contextStr, question)

			// Stream to TTY, or return complete response for JSON/pipe
			if env.IsTTY && !env.JSONMode && !noStream {
				env.Status("")
				fmt.Fprint(env.Stderr, "✨ ") //nolint:errcheck
				return ollama.ChatStream(cliCtx.Background(), messages, env.Stderr)
			}

			response, err := ollama.Chat(cliCtx.Background(), messages)
			if err != nil {
				return err
			}

			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"answer": response, "model": ollama.Model},
					"assist response",
				))
			}

			env.Status("\n%s", response)
			return nil
		},
	}

	cmd.Flags().StringVar(&model, "model", "", fmt.Sprintf("Ollama model (default: %s)", assist.DefaultModelName()))
	cmd.Flags().BoolVar(&setup, "setup", false, "Set up Ollama and download the model")
	cmd.Flags().BoolVar(&status, "status", false, "Check Ollama status")
	cmd.Flags().BoolVar(&noStream, "no-stream", false, "Disable streaming output")
	return cmd
}

func runAssistSetup(env *output.Envelope, ollama *assist.OllamaClient) error {
	env.Status("DeployHQ AI Assistant Setup")
	env.Status("")

	// Check if Ollama is installed
	ollamaPath, err := exec.LookPath("ollama")
	if err != nil {
		env.Status("Ollama is not installed.")
		env.Status("")
		env.Status("Install it with:")
		env.Status("  brew install ollama     # macOS")
		env.Status("  curl -fsSL https://ollama.com/install.sh | sh   # Linux")
		env.Status("")
		env.Status("Then run 'dhq assist --setup' again.")
		return nil
	}
	output.ColorGreen.Fprintf(env.Stderr, "Ollama found: %s\n", ollamaPath) //nolint:errcheck

	// Check if Ollama is running
	if !ollama.IsAvailable(context.TODO()) {
		env.Status("")
		env.Status("Ollama is installed but not running.")
		env.Status("Start it with: ollama serve")
		env.Status("")
		env.Status("Then run 'dhq assist --setup' again.")
		return nil
	}
	output.ColorGreen.Fprintln(env.Stderr, "Ollama is running") //nolint:errcheck

	// Check if model is available
	models, err := ollama.ListModels(context.TODO())
	if err != nil {
		return err
	}

	modelInstalled := false
	for _, m := range models {
		if strings.HasPrefix(m, ollama.Model) {
			modelInstalled = true
			break
		}
	}

	if !modelInstalled {
		env.Status("")
		env.Status("Downloading model %s (this may take a few minutes)...", ollama.Model)
		c := exec.Command("ollama", "pull", ollama.Model)
		c.Stdout = env.Stderr
		c.Stderr = env.Stderr
		if err := c.Run(); err != nil {
			return &output.UserError{
				Message: fmt.Sprintf("Failed to pull model: %v", err),
				Hint:    fmt.Sprintf("Try manually: ollama pull %s", ollama.Model),
			}
		}
	}
	output.ColorGreen.Fprintf(env.Stderr, "Model ready: %s\n", ollama.Model) //nolint:errcheck

	env.Status("")
	output.ColorGreen.Fprintln(env.Stderr, "Setup complete!") //nolint:errcheck
	env.Status("")
	env.Status("Try it out:")
	env.Status("  dhq assist \"why did my deploy fail?\" -p <project>")
	env.Status("  dhq assist \"what should I do?\" -p <project>")
	return nil
}

func runAssistStatus(env *output.Envelope, ollama *assist.OllamaClient) error {
	env.Status("DeployHQ AI Assistant Status")
	env.Status("")

	// Ollama running?
	if !ollama.IsAvailable(context.TODO()) {
		output.ColorRed.Fprintln(env.Stderr, "Ollama: not running") //nolint:errcheck
		env.Status("")
		env.Status("Run 'dhq assist --setup' to get started.")
		return nil
	}
	output.ColorGreen.Fprintf(env.Stderr, "Ollama: running (%s)\n", ollama.BaseURL) //nolint:errcheck

	// List models
	models, err := ollama.ListModels(context.TODO())
	if err != nil {
		output.ColorRed.Fprintf(env.Stderr, "Models: error (%v)\n", err) //nolint:errcheck
		return nil
	}

	modelFound := false
	for _, m := range models {
		if strings.HasPrefix(m, ollama.Model) {
			modelFound = true
			break
		}
	}

	if modelFound {
		output.ColorGreen.Fprintf(env.Stderr, "Model: %s (installed)\n", ollama.Model) //nolint:errcheck
	} else {
		output.ColorYellow.Fprintf(env.Stderr, "Model: %s (not installed)\n", ollama.Model) //nolint:errcheck
		env.Status("")
		env.Status("Pull it with: ollama pull %s", ollama.Model)
	}

	if len(models) > 0 {
		env.Status("")
		env.Status("Available models:")
		for _, m := range models {
			env.Status("  %s", m)
		}
	}

	return nil
}
