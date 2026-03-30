package commands

import (
	"fmt"
	"os"

	"github.com/deployhq/deployhq-cli/internal/config"
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long:  "View, set, and unset configuration values across global and project scopes.",
	}

	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigInitCmd(),
		newConfigSetCmd(),
		newConfigUnsetCmd(),
	)

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	var resolved bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  "Display the resolved configuration. Use --resolved to see which layer each value comes from.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := cliCtx.Envelope
			cfg := cliCtx.Config

			if env.JSONMode {
				if resolved {
					return env.WriteJSON(map[string]interface{}{
						"values":  cfg,
						"sources": cfg.Sources,
					})
				}
				return env.WriteJSON(cfg)
			}

			// Table output
			if resolved {
				env.WriteTable(
					[]string{"Key", "Value", "Source"},
					[][]string{
						{"account", cfg.Account, cfg.Sources["account"]},
						{"email", cfg.Email, cfg.Sources["email"]},
						{"api_key", maskAPIKey(cfg.APIKey), cfg.Sources["api_key"]},
						{"project", cfg.Project, cfg.Sources["project"]},
						{"format", cfg.OutputFmt, cfg.Sources["format"]},
					},
				)
			} else {
				env.WriteTable(
					[]string{"Key", "Value"},
					[][]string{
						{"account", cfg.Account},
						{"email", cfg.Email},
						{"api_key", maskAPIKey(cfg.APIKey)},
						{"project", cfg.Project},
						{"format", cfg.OutputFmt},
					},
				)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&resolved, "resolved", false, "Show which layer each value comes from")
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a config file",
		Long:  "Create a .deployhq.toml in the current directory (or --global for ~/.deployhq/config.toml).",
		RunE: func(cmd *cobra.Command, args []string) error {
			var path string
			if global {
				path = config.GlobalConfigPath()
			} else {
				path = config.ProjectConfigPath()
			}

			// Check if file already exists
			if fileExists(path) {
				return &output.UserError{
					Message: fmt.Sprintf("Config file already exists: %s", path),
					Hint:    "Use 'deployhq config set' to modify existing config",
				}
			}

			if err := config.Set(path, "project", ""); err != nil {
				return &output.InternalError{Message: "create config file", Cause: err}
			}

			cliCtx.Envelope.Status("Created %s", path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Create global config instead of project config")
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long:  "Set a configuration value in the project config (.deployhq.toml) or global config (--global).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			if !isValidKey(key) {
				return &output.UserError{
					Message: fmt.Sprintf("Unknown config key: %s", key),
					Hint:    fmt.Sprintf("Valid keys: %v", config.Keys),
				}
			}

			var path string
			if global {
				path = config.GlobalConfigPath()
			} else {
				path = config.ProjectConfigPath()
			}

			if err := config.Set(path, key, value); err != nil {
				return &output.InternalError{Message: "set config", Cause: err}
			}

			cliCtx.Envelope.Status("Set %s = %s in %s", key, value, path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Set in global config")
	return cmd
}

func newConfigUnsetCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			if !isValidKey(key) {
				return &output.UserError{
					Message: fmt.Sprintf("Unknown config key: %s", key),
					Hint:    fmt.Sprintf("Valid keys: %v", config.Keys),
				}
			}

			var path string
			if global {
				path = config.GlobalConfigPath()
			} else {
				path = config.ProjectConfigPath()
			}

			if err := config.Unset(path, key); err != nil {
				return &output.InternalError{Message: "unset config", Cause: err}
			}

			cliCtx.Envelope.Status("Removed %s from %s", key, path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Unset in global config")
	return cmd
}

func isValidKey(key string) bool {
	for _, k := range config.Keys {
		if k == key {
			return true
		}
	}
	return false
}

func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
