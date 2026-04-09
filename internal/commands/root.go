// Package commands provides all CLI commands.
package commands

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deployhq/deployhq-cli/internal/cli"
	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/harness"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/internal/telemetry"
	versionpkg "github.com/deployhq/deployhq-cli/internal/version"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	flagAccount string
	flagEmail   string
	flagAPIKey  string
	flagProject string
	flagJSON    string
	flagCwd     string
	flagHost    string

	// Shared context
	cliCtx *cli.Context

	// cliVersion is the build version, set by NewRootCmd.
	cliVersion string

	// Telemetry state captured during command execution.
	commandStartTime time.Time
	commandPath      string
	telemetryID      string
	telemetryTracker telemetry.Tracker
)

// NewRootCmd creates the root command with all subcommands.
func NewRootCmd(version string) *cobra.Command {
	cliVersion = version
	root := &cobra.Command{
		Use:     "dhq",
		Short:   "DeployHQ CLI — deploy from your terminal",
		Long: `The official DeployHQ command-line interface for managing projects, servers, and deployments.

Feedback & feature requests: https://changelog.deployhq.com
Support: support@deployhq.com`,
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Handle --cwd
			if flagCwd != "" {
				if err := os.Chdir(flagCwd); err != nil {
					return &output.UserError{
						Message: "Cannot change to directory: " + flagCwd,
						Hint:    "Check that the directory exists",
					}
				}
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Apply flag overrides (Layer 1)
			cfg.ApplyFlags(flagAccount, flagEmail, flagAPIKey, flagProject, "", flagHost)

			// Setup output
			logger := output.NewLogger()
			env := output.NewEnvelope(logger)

			// Handle --json flag
			if flagJSON != "" {
				env.JSONMode = true
				if flagJSON != "true" && flagJSON != "1" {
					env.JSONFields = strings.Split(flagJSON, ",")
				}
			}

			// Detect agent mode
			agent := harness.Detect()

			cliCtx = cli.NewContext(cfg, env, logger)
			cliCtx.IsAgent = agent.Detected
			cliCtx.Version = version

			// Telemetry setup
			commandStartTime = time.Now()
			commandPath = strings.TrimPrefix(cmd.CommandPath(), "dhq ")
			telemetryTracker = telemetry.DefaultTracker()

			home, _ := os.UserHomeDir()
			if home != "" {
				dir := filepath.Join(home, ".deployhq")

				// First-run notice
				if telemetry.IsFirstRun() && telemetry.IsEnabled() {
					env.Status(telemetry.FirstRunNotice())
				}

				telemetryID = telemetry.DistinctID(dir)
			}

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if cliCtx == nil {
				return
			}

			// Version update check (Wrangler pattern: show on exit, non-blocking)
			// Skip after "dhq update" — the running binary still has the old
			// version baked in, so the check would show a stale notice.
			if version != "dev" && !cliCtx.IsAgent && cmd.Name() != "update" {
				info := versionpkg.Check(version)
				if msg := versionpkg.FormatUpdateMessage(info); msg != "" {
					cliCtx.Envelope.Status(msg)
				}
			}

			cliCtx.Envelope.Close()
			cliCtx.Logger.Close()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global persistent flags
	pf := root.PersistentFlags()
	pf.StringVar(&flagAccount, "account", "", "DeployHQ account subdomain")
	pf.StringVar(&flagEmail, "email", "", "Authentication email")
	pf.StringVar(&flagAPIKey, "api-key", "", "API key")
	pf.StringVarP(&flagProject, "project", "p", "", "Project permalink or identifier")
	pf.StringVar(&flagJSON, "json", "", "Output as JSON (optionally specify fields: --json name,status)")
	pf.Lookup("json").NoOptDefVal = "true"
	pf.StringVarP(&flagCwd, "cwd", "C", "", "Change working directory before running")
	pf.StringVar(&flagHost, "host", "", "API host override (e.g. deployhq.dev for staging)")
	_ = pf.MarkHidden("host")

	// Register subcommands
	root.AddCommand(
		// Core resource commands
		newProjectsCmd(),
		newServersCmd(),
		newServerGroupsCmd(),
		newDeploymentsCmd(),
		newReposCmd(),

		// Extended resource commands
		newEnvVarsCmd(),
		newConfigFilesCmd(),
		newBuildCommandsCmd(),
		newBuildConfigsCmd(),
		newLanguageVersionsCmd(),
		newSSHCommandsCmd(),
		newExcludedFilesCmd(),
		newIntegrationsCmd(),
		newAgentsCmd(),
		newSSHKeysCmd(),
		newGlobalServersCmd(),
		newGlobalEnvVarsCmd(),
		newAutoDeploysCmd(),
		newScheduledDeploysCmd(),
		newTemplatesCmd(),
		newZonesCmd(),

		// Operations
		newTestAccessCmd(),

		// Shortcuts
		newDeployCmd(),
		newRetryCmd(),
		newRollbackCmd(),
		newOpenCmd(),
		newInitCmd(),
		newHelloCmd(),

		// Escape hatch
		newAPICmd(),

		// Auth & Config
		newAuthCmd(),
		newSignupCmd(),
		newConfigCmd(),
		newConfigureCmd(),

		// Dashboard
		newActivityCmd(),
		newStatusCmd(),

		// AI Assistant
		newAssistCmd(),

		// Agent & Meta
		newCommandsCatalogCmd(),
		newShowCmd(),
		newURLCmd(),
		newSetupCmd(),
		newMCPCmd(),
		newCompletionCmd(),
		newTelemetryCmd(),
		newFeedbackCmd(),
		newDoctorCmd(),
		newUpdateCmd(version),
		newVersionCmd(version),
	)

	// Register dynamic completions for --project flag
	root.RegisterFlagCompletionFunc("project", completeProjectNames) //nolint:errcheck

	// Install --help --agent JSON help on all commands
	installAgentHelp(root)

	return root
}

// IsJSONMode returns true if --json was passed or output is piped (non-TTY).
func IsJSONMode() bool {
	if cliCtx != nil {
		return cliCtx.Envelope.JSONMode || !cliCtx.Envelope.IsTTY
	}
	return flagJSON != ""
}

// cliUserAgent returns a User-Agent string like "DeployHQ-CLI/1.2.3"
// or "DeployHQ-CLI/1.2.3 (agent:claude-code)" when an agent is detected.
func cliUserAgent() string {
	v := cliVersion
	if v == "" {
		v = "dev"
	}
	return harness.UserAgent(v, harness.Detect())
}

// SendTelemetry fires a telemetry event for the just-completed command.
// It is called from main.go after cmd.Execute() returns so that the
// exit code is available. The call is non-blocking (fire-and-forget).
func SendTelemetry(err error) {
	if telemetryTracker == nil || telemetryID == "" {
		return
	}
	if !telemetry.IsEnabled() {
		return
	}
	// Don't track the telemetry command itself.
	if strings.HasPrefix(commandPath, "telemetry") {
		return
	}

	exitCode := output.ClassifyError(err)
	if err != nil && exitCode == 0 {
		exitCode = 1
	}

	agentName := ""
	isAgent := false
	if cliCtx != nil {
		isAgent = cliCtx.IsAgent
	}
	if isAgent {
		agentName = harness.Detect().Name
	}

	evt := telemetry.Event{
		Command:    commandPath,
		ExitCode:   exitCode,
		ErrorClass: telemetry.ErrorClassFromExitCode(exitCode),
		DurationMs: time.Since(commandStartTime).Milliseconds(),
		CLIVersion: cliVersion,
		IsAgent:    isAgent,
		AgentName:  agentName,
	}

	// Run in a goroutine but wait up to 2s so the HTTP request
	// completes before main() exits or calls os.Exit.
	done := make(chan struct{})
	go func() {
		telemetryTracker.Track(telemetryID, evt)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}
