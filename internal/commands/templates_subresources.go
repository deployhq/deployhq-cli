package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

// Template sub-resource commands mirror the project-level resources but are
// nested under a template identified by its permalink. Every command in this
// tree takes a --template / -t flag (analogous to -p <project> on project-level
// commands) that resolves to the template permalink used in the API path.
//
// These are exposed as standalone constructors so the orchestrator can attach
// them onto the existing `templates` parent command in root.go without this
// file editing that shared file.

// addTemplateFlag registers the shared --template / -t flag and returns a
// pointer to the bound value. Call requireTemplate(cmd, ptr) inside RunE to
// enforce presence with a consistent error.
func addTemplateFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVarP(target, "template", "t", "", "Template permalink (required)")
}

// requireUpdateFlags guards update handlers against sending a zero-value PATCH.
// If none of the named field flags were set, it returns a UserError instead of
// silently overwriting the resource with empty values.
func requireUpdateFlags(cmd *cobra.Command, flags ...string) error {
	for _, f := range flags {
		if cmd.Flags().Changed(f) {
			return nil
		}
	}
	return &output.UserError{Message: "no fields to update; pass at least one flag"}
}

// requireTemplate validates that a template permalink was supplied.
func requireTemplate(permalink string) (string, error) {
	if permalink == "" {
		return "", &output.UserError{
			Message: "No template specified",
			Hint: "Identify the template with its permalink:\n" +
				"  --template <permalink>   (or -t)\n" +
				"Find permalinks with 'dhq templates list'.",
		}
	}
	return permalink, nil
}

// newTemplatesSubresourceCmds returns every template sub-resource parent
// command. The orchestrator attaches these onto the `templates` command.
func newTemplatesSubresourceCmds() []*cobra.Command {
	return []*cobra.Command{
		newTemplateConfigFilesCmd(),
		newTemplateExcludedFilesCmd(),
		newTemplateIntegrationsCmd(),
		newTemplateCommandsCmd(),
		newTemplateBuildCommandsCmd(),
		newTemplateBuildCacheFilesCmd(),
		newTemplateBuildKnownHostsCmd(),
		newTemplateBuildLanguagesCmd(),
		newTemplateBuildConfigurationCmd(),
		newTemplateServersCmd(),
		newTemplateServerGroupsCmd(),
	}
}

// --- 1. config_files (full CRUD) ---

func newTemplateConfigFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-files",
		Short: "Manage config files on a template",
		Long:  "Config file overrides baked into a template. Copied into new projects created from the template. Requires --template <permalink>.",
	}
	cmd.AddCommand(
		tcfListCmd(), tcfShowCmd(), tcfCreateCmd(), tcfUpdateCmd(), tcfDeleteCmd(),
	)
	return cmd
}

func tcfListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template config files",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			files, err := client.ListTemplateConfigFiles(cliCtx.Background(), permalink, nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(files, fmt.Sprintf("%d config files", len(files))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(files))
				for i, f := range files {
					identifiers[i] = f.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(files))
			for i, f := range files {
				rows[i] = []string{f.Identifier, f.Path, f.Description}
			}
			env.WriteTable([]string{"Identifier", "Path", "Description"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tcfShowCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "show <id>", Short: "Show a template config file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.GetTemplateConfigFile(cliCtx.Background(), permalink, args[0])
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, f.Path))
			}
			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Path", f.Path},
				{"Description", f.Description},
				{"Identifier", f.Identifier},
			})
			env.Status("\nContent:\n%s", f.Body)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tcfCreateCmd() *cobra.Command {
	var tmpl, path, body, description string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" || body == "" {
				return &output.UserError{Message: "Both --path and --body are required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.CreateTemplateConfigFile(cliCtx.Background(), permalink, sdk.ConfigFileCreateRequest{
				Path: path, Body: body, Description: description,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Created: %s", f.Path)))
			}
			env.Status("Created config file: %s", f.Path)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&body, "body", "", "File content (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	return cmd
}

func tcfUpdateCmd() *cobra.Command {
	var tmpl, path, body, description string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a template config file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := requireUpdateFlags(cmd, "path", "body", "description"); err != nil {
				return err
			}
			var req sdk.ConfigFileUpdateRequest
			if cmd.Flags().Changed("path") {
				req.Path = &path
			}
			if cmd.Flags().Changed("body") {
				req.Body = &body
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			f, err := client.UpdateTemplateConfigFile(cliCtx.Background(), permalink, args[0], req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Updated: %s", f.Path)))
			}
			env.Status("Updated config file: %s", f.Path)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&path, "path", "", "File path")
	cmd.Flags().StringVar(&body, "body", "", "File content")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	return cmd
}

func tcfDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template config file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateConfigFile(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted config file: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 2. excluded_files (list/create/update/delete — NO show) ---

func newTemplateExcludedFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "excluded-files",
		Short: "Manage excluded files on a template",
		Long:  "File patterns excluded from deploys, baked into a template. Requires --template <permalink>.",
	}
	cmd.AddCommand(tefListCmd(), tefCreateCmd(), tefUpdateCmd(), tefDeleteCmd())
	return cmd
}

func tefListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template excluded files",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			files, err := client.ListTemplateExcludedFiles(cliCtx.Background(), permalink, nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(files, fmt.Sprintf("%d excluded files", len(files))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(files))
				for i, f := range files {
					identifiers[i] = f.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(files))
			for i, f := range files {
				rows[i] = []string{f.Identifier, f.Path}
			}
			env.WriteTable([]string{"Identifier", "Path"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tefCreateCmd() *cobra.Command {
	var tmpl, path string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template excluded file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return &output.UserError{Message: "--path is required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.CreateTemplateExcludedFile(cliCtx.Background(), permalink, sdk.ExcludedFileCreateRequest{Path: path})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Created: %s", f.Path)))
			}
			env.Status("Created excluded file: %s", f.Path)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&path, "path", "", "File pattern (required)")
	return cmd
}

func tefUpdateCmd() *cobra.Command {
	var tmpl, path string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a template excluded file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := requireUpdateFlags(cmd, "path"); err != nil {
				return err
			}
			f, err := client.UpdateTemplateExcludedFile(cliCtx.Background(), permalink, args[0], sdk.ExcludedFileCreateRequest{Path: path})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Updated: %s", f.Path)))
			}
			env.Status("Updated excluded file: %s", f.Path)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&path, "path", "", "File pattern")
	return cmd
}

func tefDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template excluded file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateExcludedFile(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted excluded file: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 3. integrations (list/create/update/delete) ---

func newTemplateIntegrationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "Manage integrations on a template",
		Long:  "Webhook/notification integrations baked into a template. Create is limited to API-creatable hook types; hook_type is immutable on update. Requires --template <permalink>.",
	}
	cmd.AddCommand(tintListCmd(), tintCreateCmd(), tintUpdateCmd(), tintDeleteCmd())
	return cmd
}

func tintListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template integrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			ints, err := client.ListTemplateIntegrations(cliCtx.Background(), permalink, nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(ints, fmt.Sprintf("%d integrations", len(ints))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(ints))
				for i, in := range ints {
					identifiers[i] = in.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(ints))
			for i, in := range ints {
				rows[i] = []string{in.Identifier, in.HookType, in.Name}
			}
			env.WriteTable([]string{"Identifier", "HookType", "Name"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tintCreateCmd() *cobra.Command {
	var tmpl, hookType, name string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hookType == "" {
				return &output.UserError{Message: "--hook-type is required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			in, err := client.CreateTemplateIntegration(cliCtx.Background(), permalink, sdk.IntegrationCreateRequest{
				HookType: hookType, Name: name,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if in.AuthRequired != nil && *in.AuthRequired {
				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(in, "Authorization required"))
				}
				env.Status("Authorization required — visit: %s", in.AuthURL)
				return nil
			}
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(in, fmt.Sprintf("Created: %s", in.Identifier)))
			}
			env.Status("Created integration: %s (%s)", in.HookType, in.Identifier)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&hookType, "hook-type", "", "Hook type (required)")
	cmd.Flags().StringVar(&name, "name", "", "Integration name")
	return cmd
}

func tintUpdateCmd() *cobra.Command {
	var tmpl, name string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a template integration (hook_type is immutable)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := requireUpdateFlags(cmd, "name"); err != nil {
				return err
			}
			in, err := client.UpdateTemplateIntegration(cliCtx.Background(), permalink, args[0], sdk.IntegrationCreateRequest{Name: name})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if in.AuthRequired != nil && *in.AuthRequired {
				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(in, "Authorization required"))
				}
				env.Status("Authorization required — visit: %s", in.AuthURL)
				return nil
			}
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(in, fmt.Sprintf("Updated: %s", in.Identifier)))
			}
			env.Status("Updated integration: %s", in.Identifier)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&name, "name", "", "Integration name")
	return cmd
}

func tintDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template integration", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateIntegration(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted integration: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 4. commands (list/create/update/delete) ---

func newTemplateCommandsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "commands",
		Aliases: []string{"ssh-commands"},
		Short:   "Manage SSH commands on a template",
		Long:    "SSH commands run on target servers during deploys, baked into a template. Requires --template <permalink>. (Aliased as 'ssh-commands', which hits the same backend route.)",
	}
	cmd.AddCommand(tcmdListCmd(), tcmdCreateCmd(), tcmdUpdateCmd(), tcmdDeleteCmd())
	return cmd
}

func tcmdListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template SSH commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			cmds, err := client.ListTemplateCommands(cliCtx.Background(), permalink, nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(cmds, fmt.Sprintf("%d commands", len(cmds))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(cmds))
				for i, cm := range cmds {
					identifiers[i] = cm.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(cmds))
			for i, cm := range cmds {
				rows[i] = []string{cm.Identifier, cm.Command, cm.Timing}
			}
			env.WriteTable([]string{"Identifier", "Command", "Timing"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tcmdCreateCmd() *cobra.Command {
	var tmpl, command, description, timing string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template SSH command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if command == "" {
				return &output.UserError{Message: "--command is required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			cm, err := client.CreateTemplateCommand(cliCtx.Background(), permalink, sdk.SSHCommandCreateRequest{
				Command: command, Description: description, Timing: timing,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(cm, fmt.Sprintf("Created: %s", cm.Identifier)))
			}
			env.Status("Created SSH command: %s", cm.Identifier)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&command, "command", "", "Command to run (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&timing, "timing", "", "Timing (before/after)")
	return cmd
}

func tcmdUpdateCmd() *cobra.Command {
	var tmpl, command, description, timing string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a template SSH command", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := requireUpdateFlags(cmd, "command", "description", "timing"); err != nil {
				return err
			}
			cm, err := client.UpdateTemplateCommand(cliCtx.Background(), permalink, args[0], sdk.SSHCommandCreateRequest{
				Command: command, Description: description, Timing: timing,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(cm, fmt.Sprintf("Updated: %s", cm.Identifier)))
			}
			env.Status("Updated SSH command: %s", cm.Identifier)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&command, "command", "", "Command to run")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&timing, "timing", "", "Timing (before/after)")
	return cmd
}

func tcmdDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template SSH command", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateCommand(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted SSH command: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 5. build_commands (list/create/update/delete) ---

func newTemplateBuildCommandsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-commands",
		Short: "Manage build commands on a template",
		Long:  "Build commands run on the build server, baked into a template. Requires --template <permalink>.",
	}
	cmd.AddCommand(tbcListCmd(), tbcCreateCmd(), tbcUpdateCmd(), tbcDeleteCmd())
	return cmd
}

func tbcListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template build commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			cmds, err := client.ListTemplateBuildCommands(cliCtx.Background(), permalink, nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(cmds, fmt.Sprintf("%d build commands", len(cmds))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(cmds))
				for i, cm := range cmds {
					identifiers[i] = cm.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(cmds))
			for i, cm := range cmds {
				rows[i] = []string{cm.Identifier, cm.Command, cm.Description}
			}
			env.WriteTable([]string{"Identifier", "Command", "Description"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tbcCreateCmd() *cobra.Command {
	var tmpl, command, description string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template build command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if command == "" {
				return &output.UserError{Message: "--command is required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			cm, err := client.CreateTemplateBuildCommand(cliCtx.Background(), permalink, sdk.BuildCommandCreateRequest{
				Command: command, Description: description,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(cm, fmt.Sprintf("Created: %s", cm.Identifier)))
			}
			env.Status("Created build command: %s", cm.Identifier)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&command, "command", "", "Command to run (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	return cmd
}

func tbcUpdateCmd() *cobra.Command {
	var tmpl, command, description string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a template build command", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := requireUpdateFlags(cmd, "command", "description"); err != nil {
				return err
			}
			cm, err := client.UpdateTemplateBuildCommand(cliCtx.Background(), permalink, args[0], sdk.BuildCommandCreateRequest{
				Command: command, Description: description,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(cm, fmt.Sprintf("Updated: %s", cm.Identifier)))
			}
			env.Status("Updated build command: %s", cm.Identifier)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&command, "command", "", "Command to run")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	return cmd
}

func tbcDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template build command", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateBuildCommand(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted build command: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 6. build_cache_files (list/create/update/delete) ---

func newTemplateBuildCacheFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-cache-files",
		Short: "Manage build cache files on a template",
		Long:  "Paths cached between builds, baked into a template. Requires --template <permalink>.",
	}
	cmd.AddCommand(tbcfListCmd(), tbcfCreateCmd(), tbcfUpdateCmd(), tbcfDeleteCmd())
	return cmd
}

func tbcfListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template build cache files",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			files, err := client.ListTemplateBuildCacheFiles(cliCtx.Background(), permalink)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(files, fmt.Sprintf("%d build cache files", len(files))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(files))
				for i, f := range files {
					identifiers[i] = f.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(files))
			for i, f := range files {
				rows[i] = []string{f.Identifier, f.Path}
			}
			env.WriteTable([]string{"Identifier", "Path"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tbcfCreateCmd() *cobra.Command {
	var tmpl, path string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template build cache file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return &output.UserError{Message: "--path is required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			f, err := client.CreateTemplateBuildCacheFile(cliCtx.Background(), permalink, sdk.BuildCacheFileCreateRequest{Path: path})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Created: %s", f.Path)))
			}
			env.Status("Created build cache file: %s", f.Path)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&path, "path", "", "Cache path (required)")
	return cmd
}

func tbcfUpdateCmd() *cobra.Command {
	var tmpl, path string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a template build cache file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := requireUpdateFlags(cmd, "path"); err != nil {
				return err
			}
			f, err := client.UpdateTemplateBuildCacheFile(cliCtx.Background(), permalink, args[0], sdk.BuildCacheFileCreateRequest{Path: path})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(f, fmt.Sprintf("Updated: %s", f.Path)))
			}
			env.Status("Updated build cache file: %s", f.Path)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&path, "path", "", "Cache path")
	return cmd
}

func tbcfDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template build cache file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateBuildCacheFile(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted build cache file: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 7. build_known_hosts (list/create/delete — NO update) ---

func newTemplateBuildKnownHostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-known-hosts",
		Short: "Manage build known hosts on a template",
		Long:  "SSH known-host entries for the build server, baked into a template. No update — recreate to change. Requires --template <permalink>.",
	}
	cmd.AddCommand(tbkhListCmd(), tbkhCreateCmd(), tbkhDeleteCmd())
	return cmd
}

func tbkhListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template build known hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			hosts, err := client.ListTemplateBuildKnownHosts(cliCtx.Background(), permalink)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(hosts, fmt.Sprintf("%d known hosts", len(hosts))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(hosts))
				for i, h := range hosts {
					identifiers[i] = h.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(hosts))
			for i, h := range hosts {
				rows[i] = []string{h.Identifier, h.Hostname}
			}
			env.WriteTable([]string{"Identifier", "Hostname"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tbkhCreateCmd() *cobra.Command {
	var tmpl, hostname, publicKey string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template build known host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hostname == "" || publicKey == "" {
				return &output.UserError{Message: "Both --hostname and --public-key are required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			h, err := client.CreateTemplateBuildKnownHost(cliCtx.Background(), permalink, sdk.BuildKnownHostCreateRequest{
				Hostname: hostname, PublicKey: publicKey,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(h, fmt.Sprintf("Created: %s", h.Hostname)))
			}
			env.Status("Created build known host: %s", h.Hostname)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&hostname, "hostname", "", "Hostname (required)")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "SSH public key (required)")
	return cmd
}

func tbkhDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template build known host", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateBuildKnownHost(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted build known host: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 8. build_languages (UPDATE/set ONLY; :id = package name) ---

func newTemplateBuildLanguagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-languages",
		Short: "Manage build language versions on a template",
		Long:  "Set language versions on a template's default build environment. The <package-name> is the package (e.g. ruby, nodejs). Requires --template <permalink>.",
	}
	cmd.AddCommand(tblSetCmd())
	return cmd
}

func tblSetCmd() *cobra.Command {
	var tmpl, version string
	cmd := &cobra.Command{
		Use: "set <package-name>", Short: "Set the version for a build language", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if version == "" {
				return &output.UserError{Message: "--version is required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			lang, err := client.UpdateTemplateBuildLanguage(cliCtx.Background(), permalink, args[0], sdk.BuildLanguageUpdateRequest{Version: version})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(lang, lang.Name+" "+lang.Version))
			}
			env.Status("Set %s to version %s", lang.Name, lang.Version)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&version, "version", "", "Language version (required)")
	return cmd
}

// --- 9. build_configuration (SHOW ONLY; singular) ---

func newTemplateBuildConfigurationCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use:   "build-configuration",
		Short: "Show a template's default build environment",
		Long:  "Show the default build environment (packages) for a template. Requires --template <permalink>.",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			cfg, err := client.GetTemplateBuildConfiguration(cliCtx.Background(), permalink)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(cfg, "Build configuration"))
			}
			rows := make([][]string, 0, len(cfg.Packages))
			for pkg, ver := range cfg.Packages {
				rows = append(rows, []string{pkg, ver})
			}
			env.WriteTable([]string{"Package", "Version"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 10. servers (full CRUD) ---

func newTemplateServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "servers",
		Short: "Manage servers on a template",
		Long:  "Server definitions baked into a template. Project-only protocols are rejected. Requires --template <permalink>.",
	}
	cmd.AddCommand(tsrvListCmd(), tsrvShowCmd(), tsrvCreateCmd(), tsrvUpdateCmd(), tsrvDeleteCmd())
	return cmd
}

func tsrvListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			servers, err := client.ListTemplateServers(cliCtx.Background(), permalink, nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(servers, fmt.Sprintf("%d servers", len(servers))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(servers))
				for i, s := range servers {
					identifiers[i] = s.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(servers))
			for i, s := range servers {
				rows[i] = []string{s.Identifier, s.Name, s.ProtocolType, s.Environment}
			}
			env.WriteTable([]string{"Identifier", "Name", "Protocol", "Environment"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tsrvShowCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "show <id>", Short: "Show a template server", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			s, err := client.GetTemplateServer(cliCtx.Background(), permalink, args[0])
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(s, fmt.Sprintf("Server: %s", s.Name)))
			}
			env.WriteTable([]string{"Field", "Value"}, [][]string{
				{"Name", s.Name},
				{"Identifier", s.Identifier},
				{"Protocol", s.ProtocolType},
				{"Environment", s.Environment},
				{"Server Path", s.ServerPath},
			})
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tsrvCreateCmd() *cobra.Command {
	var tmpl, name, protocolType, serverPath, environment string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || protocolType == "" {
				return &output.UserError{Message: "Both --name and --protocol-type are required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			s, err := client.CreateTemplateServer(cliCtx.Background(), permalink, sdk.ServerCreateRequest{
				Name: name, ProtocolType: protocolType, ServerPath: serverPath, Environment: environment,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(s, fmt.Sprintf("Created: %s", s.Name)))
			}
			env.Status("Created server: %s (%s)", s.Name, s.Identifier)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&name, "name", "", "Server name (required)")
	cmd.Flags().StringVar(&protocolType, "protocol-type", "", "Protocol type (required)")
	cmd.Flags().StringVar(&serverPath, "server-path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment")
	return cmd
}

func tsrvUpdateCmd() *cobra.Command {
	var tmpl, name, serverPath, environment string
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a template server", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := requireUpdateFlags(cmd, "name", "server-path", "environment"); err != nil {
				return err
			}
			s, err := client.UpdateTemplateServer(cliCtx.Background(), permalink, args[0], sdk.ServerUpdateRequest{
				Name: name, ServerPath: serverPath, Environment: environment,
			})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(s, fmt.Sprintf("Updated: %s", s.Name)))
			}
			env.Status("Updated server: %s", s.Name)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&name, "name", "", "Server name")
	cmd.Flags().StringVar(&serverPath, "server-path", "", "Server path")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment")
	return cmd
}

func tsrvDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template server", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateServer(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted server: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

// --- 11. server_groups (list/create/update/delete — NO show) ---

func newTemplateServerGroupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "server-groups",
		Aliases: []string{"sg"},
		Short:   "Manage server groups on a template",
		Long:    "Server groups baked into a template. Requires --template <permalink>.",
	}
	cmd.AddCommand(tsgListCmd(), tsgCreateCmd(), tsgUpdateCmd(), tsgDeleteCmd())
	return cmd
}

func tsgListCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "list", Short: "List template server groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			groups, err := client.ListTemplateServerGroups(cliCtx.Background(), permalink, nil)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(groups, fmt.Sprintf("%d server groups", len(groups))))
			}
			if env.QuietMode {
				identifiers := make([]string, len(groups))
				for i, g := range groups {
					identifiers[i] = g.Identifier
				}
				env.WriteQuiet(identifiers)
				return nil
			}
			rows := make([][]string, len(groups))
			for i, g := range groups {
				rows[i] = []string{g.Identifier, g.Name, g.Environment}
			}
			env.WriteTable([]string{"Identifier", "Name", "Environment"}, rows)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}

func tsgCreateCmd() *cobra.Command {
	var tmpl, name, environment, transferOrder, emailNotifyOn, notificationEmail string
	var autoDeploy bool
	cmd := &cobra.Command{
		Use: "create", Short: "Create a template server group",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return &output.UserError{Message: "--name is required"}
			}
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			req := sdk.TemplateServerGroupCreateRequest{
				Name: name, Environment: environment, TransferOrder: transferOrder,
				EmailNotifyOn: emailNotifyOn, NotificationEmail: notificationEmail,
			}
			if cmd.Flags().Changed("auto-deploy") {
				req.AutoDeploy = &autoDeploy
			}
			g, err := client.CreateTemplateServerGroup(cliCtx.Background(), permalink, req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(g, fmt.Sprintf("Created: %s", g.Name)))
			}
			env.Status("Created server group: %s (%s)", g.Name, g.Identifier)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&name, "name", "", "Server group name (required)")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment")
	cmd.Flags().StringVar(&transferOrder, "transfer-order", "", "Transfer order")
	cmd.Flags().StringVar(&emailNotifyOn, "email-notify-on", "", "Email notify on (event)")
	cmd.Flags().StringVar(&notificationEmail, "notification-email", "", "Notification email")
	cmd.Flags().BoolVar(&autoDeploy, "auto-deploy", false, "Enable auto-deploy")
	return cmd
}

func tsgUpdateCmd() *cobra.Command {
	var tmpl, name, environment, transferOrder, emailNotifyOn, notificationEmail string
	var autoDeploy bool
	cmd := &cobra.Command{
		Use: "update <id>", Short: "Update a template server group", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := requireUpdateFlags(cmd, "name", "environment", "transfer-order", "email-notify-on", "notification-email", "auto-deploy"); err != nil {
				return err
			}
			req := sdk.TemplateServerGroupUpdateRequest{
				Name: name, Environment: environment, TransferOrder: transferOrder,
				EmailNotifyOn: emailNotifyOn, NotificationEmail: notificationEmail,
			}
			if cmd.Flags().Changed("auto-deploy") {
				req.AutoDeploy = &autoDeploy
			}
			g, err := client.UpdateTemplateServerGroup(cliCtx.Background(), permalink, args[0], req)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(g, fmt.Sprintf("Updated: %s", g.Name)))
			}
			env.Status("Updated server group: %s", g.Name)
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	cmd.Flags().StringVar(&name, "name", "", "Server group name")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment")
	cmd.Flags().StringVar(&transferOrder, "transfer-order", "", "Transfer order")
	cmd.Flags().StringVar(&emailNotifyOn, "email-notify-on", "", "Email notify on (event)")
	cmd.Flags().StringVar(&notificationEmail, "notification-email", "", "Notification email")
	cmd.Flags().BoolVar(&autoDeploy, "auto-deploy", false, "Enable auto-deploy")
	return cmd
}

func tsgDeleteCmd() *cobra.Command {
	var tmpl string
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template server group", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			permalink, err := requireTemplate(tmpl)
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			if err := client.DeleteTemplateServerGroup(cliCtx.Background(), permalink, args[0]); err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(map[string]string{"identifier": args[0], "status": "deleted"}, fmt.Sprintf("Deleted: %s", args[0])))
			}
			env.Status("Deleted server group: %s", args[0])
			return nil
		},
	}
	addTemplateFlag(cmd, &tmpl)
	return cmd
}
