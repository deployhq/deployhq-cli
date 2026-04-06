package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// promptSecretValue prompts for a value via masked input when running interactively.
func promptSecretValue(env *output.Envelope) (string, error) {
	if !env.IsTTY {
		return "", &output.UserError{Message: "--value is required in non-interactive mode"}
	}
	fmt.Fprint(env.Stderr, "Value: ") //nolint:errcheck
	val, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(env.Stderr) //nolint:errcheck
	if err != nil {
		return "", &output.InternalError{Message: "read value", Cause: err}
	}
	return strings.TrimSpace(string(val)), nil
}

func newEnvVarsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "env-vars",
		Aliases: []string{"env"},
		Short:   "Manage environment variables",
	}
	cmd.AddCommand(
		newEnvVarsListCmd(),
		newEnvVarsShowCmd(),
		newEnvVarsCreateCmd(),
		newEnvVarsUpdateCmd(),
		newEnvVarsDeleteCmd(),
	)
	return cmd
}

func newEnvVarsListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			vars, err := client.ListEnvVars(cliCtx.Background(), projectID)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(vars, fmt.Sprintf("%d environment variables", len(vars))))
			}
			rows := make([][]string, len(vars))
			for i, v := range vars {
				locked := "no"
				if v.Locked {
					locked = "yes"
				}
				rows[i] = []string{v.Name, v.MaskedValue, locked}
			}
			env.WriteTable([]string{"Name", "Value", "Locked"}, rows)

			env.Status("\nTip: dhq env-vars create --name KEY --value VALUE -p %s", projectID)
			return nil
		},
	}
}

func newEnvVarsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use: "show <id>", Short: "Show an environment variable", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			v, err := client.GetEnvVar(cliCtx.Background(), projectID, args[0])
			if err != nil {
				return err
			}
			return cliCtx.Envelope.WriteJSON(output.NewResponse(v, v.Name,
				output.Breadcrumb{Action: "list", Cmd: fmt.Sprintf("dhq env-vars list -p %s", projectID)},
				output.Breadcrumb{Action: "update", Cmd: fmt.Sprintf("dhq env-vars update %s -p %s --value <value>", args[0], projectID)},
				output.Breadcrumb{Action: "delete", Cmd: fmt.Sprintf("dhq env-vars delete %s -p %s", args[0], projectID)},
			))
		},
	}
}

func newEnvVarsCreateCmd() *cobra.Command {
	var name, value string
	var locked bool
	cmd := &cobra.Command{
		Use: "create", Short: "Create an environment variable",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "--name is required"}
			}
			env := cliCtx.Envelope
			if value == "" {
				v, err := promptSecretValue(env)
				if err != nil {
					return err
				}
				value = v
			}
			if value == "" {
				return &output.UserError{Message: "Value cannot be empty"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := sdk.EnvVarCreateRequest{Name: name, Value: value}
			if locked {
				req.Locked = &locked
			}
			v, err := client.CreateEnvVar(cliCtx.Background(), projectID, req)
			if err != nil {
				return err
			}
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(v, fmt.Sprintf("Created: %s", v.Name)))
			}
			env.Status("Created environment variable: %s", v.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Variable name (required)")
	cmd.Flags().StringVar(&value, "value", "", "Variable value (prompts securely if omitted)")
	cmd.Flags().BoolVar(&locked, "locked", false, "Lock the variable (value hidden after creation)")
	return cmd
}

func newEnvVarsUpdateCmd() *cobra.Command {
	var name, value string
	var locked bool
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update an environment variable", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := sdk.EnvVarCreateRequest{Name: name, Value: value}
			if cmd.Flags().Changed("locked") {
				req.Locked = &locked
			}
			v, err := client.UpdateEnvVar(cliCtx.Background(), projectID, args[0], req)
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated: %s", v.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Variable name")
	cmd.Flags().StringVar(&value, "value", "", "Variable value")
	cmd.Flags().BoolVar(&locked, "locked", false, "Lock the variable")
	return cmd
}

func newEnvVarsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use: "delete <id>", Short: "Delete an environment variable", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteEnvVar(cliCtx.Background(), projectID, args[0]); err != nil {
				return err
			}
			cliCtx.Envelope.Status("Deleted environment variable: %s", args[0])
			return nil
		},
	}
}

// Global env vars

func newGlobalEnvVarsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "global-env-vars",
		Short: "Manage global environment variables",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List global environment variables",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				vars, err := client.ListGlobalEnvVars(cliCtx.Background())
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.JSONMode || !env.IsTTY {
					return env.WriteJSON(output.NewResponse(vars, fmt.Sprintf("%d global env vars", len(vars))))
				}
				rows := make([][]string, len(vars))
				for i, v := range vars {
					rows[i] = []string{v.Name, v.MaskedValue}
				}
				env.WriteTable([]string{"Name", "Value"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show a global environment variable", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				v, err := client.GetGlobalEnvVar(cliCtx.Background(), args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(v, v.Name,
					output.Breadcrumb{Action: "list", Cmd: "dhq global-env-vars list"},
					output.Breadcrumb{Action: "update", Cmd: fmt.Sprintf("dhq global-env-vars update %s --value <value>", args[0])},
					output.Breadcrumb{Action: "delete", Cmd: fmt.Sprintf("dhq global-env-vars delete %s", args[0])},
				))
			},
		},
		newGlobalEnvVarsCreateCmd(),
		newGlobalEnvVarsUpdateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a global environment variable", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteGlobalEnvVar(cliCtx.Background(), args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted global env var: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newGlobalEnvVarsCreateCmd() *cobra.Command {
	var name, value string
	var locked bool
	cmd := &cobra.Command{
		Use: "create", Short: "Create a global environment variable",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "--name is required"}
			}
			env := cliCtx.Envelope
			if value == "" {
				v, err := promptSecretValue(env)
				if err != nil {
					return err
				}
				value = v
			}
			if value == "" {
				return &output.UserError{Message: "Value cannot be empty"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := sdk.EnvVarCreateRequest{Name: name, Value: value}
			if locked {
				req.Locked = &locked
			}
			v, err := client.CreateGlobalEnvVar(cliCtx.Background(), req)
			if err != nil {
				return err
			}
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(v, fmt.Sprintf("Created: %s", v.Name)))
			}
			env.Status("Created global env var: %s", v.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Variable name (required)")
	cmd.Flags().StringVar(&value, "value", "", "Variable value (prompts securely if omitted)")
	cmd.Flags().BoolVar(&locked, "locked", false, "Lock the variable (value hidden after creation)")
	return cmd
}

func newGlobalEnvVarsUpdateCmd() *cobra.Command {
	var name, value string
	var locked bool
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a global environment variable", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := sdk.EnvVarCreateRequest{Name: name, Value: value}
			if cmd.Flags().Changed("locked") {
				req.Locked = &locked
			}
			v, err := client.UpdateGlobalEnvVar(cliCtx.Background(), args[0], req)
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated global env var: %s", v.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Variable name")
	cmd.Flags().StringVar(&value, "value", "", "Variable value")
	cmd.Flags().BoolVar(&locked, "locked", false, "Lock the variable")
	return cmd
}
