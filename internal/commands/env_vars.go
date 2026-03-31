package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

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
			return cliCtx.Envelope.WriteJSON(output.NewResponse(v, v.Name))
		},
	}
}

func newEnvVarsCreateCmd() *cobra.Command {
	var name, value string
	cmd := &cobra.Command{
		Use: "create", Short: "Create an environment variable",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || value == "" {
				return &output.UserError{Message: "Both --name and --value are required"}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			v, err := client.CreateEnvVar(cliCtx.Background(), projectID, sdk.EnvVarCreateRequest{Name: name, Value: value})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(v, fmt.Sprintf("Created: %s", v.Name)))
			}
			env.Status("Created environment variable: %s", v.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Variable name (required)")
	cmd.Flags().StringVar(&value, "value", "", "Variable value (required)")
	return cmd
}

func newEnvVarsUpdateCmd() *cobra.Command {
	var name, value string
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
			v, err := client.UpdateEnvVar(cliCtx.Background(), projectID, args[0], sdk.EnvVarCreateRequest{Name: name, Value: value})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated: %s", v.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Variable name")
	cmd.Flags().StringVar(&value, "value", "", "Variable value")
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
				return cliCtx.Envelope.WriteJSON(output.NewResponse(v, v.Name))
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
	cmd := &cobra.Command{
		Use: "create", Short: "Create a global environment variable",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || value == "" {
				return &output.UserError{Message: "Both --name and --value are required"}
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			v, err := client.CreateGlobalEnvVar(cliCtx.Background(), sdk.EnvVarCreateRequest{Name: name, Value: value})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.JSONMode || !env.IsTTY {
				return env.WriteJSON(output.NewResponse(v, fmt.Sprintf("Created: %s", v.Name)))
			}
			env.Status("Created global env var: %s", v.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Variable name (required)")
	cmd.Flags().StringVar(&value, "value", "", "Variable value (required)")
	return cmd
}

func newGlobalEnvVarsUpdateCmd() *cobra.Command {
	var name, value string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a global environment variable", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			v, err := client.UpdateGlobalEnvVar(cliCtx.Background(), args[0], sdk.EnvVarCreateRequest{Name: name, Value: value})
			if err != nil {
				return err
			}
			cliCtx.Envelope.Status("Updated global env var: %s", v.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Variable name")
	cmd.Flags().StringVar(&value, "value", "", "Variable value")
	return cmd
}
