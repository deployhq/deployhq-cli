package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// AgentHelpSchema is the structured JSON help for a command (Basecamp pattern).
// Emitted when --help and --agent are both present.
type AgentHelpSchema struct {
	Name        string            `json:"name"`
	FullCommand string            `json:"full_command"`
	Description string            `json:"description"`
	Usage       string            `json:"usage"`
	Aliases     []string          `json:"aliases,omitempty"`
	Flags       []AgentFlagSchema `json:"flags,omitempty"`
	Subcommands []AgentHelpSchema `json:"subcommands,omitempty"`
	Examples    []string          `json:"examples,omitempty"`
}

// AgentFlagSchema describes a flag for agent consumption.
type AgentFlagSchema struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Global      bool   `json:"global,omitempty"`
}

// installAgentHelp adds a --agent flag to every command and overrides
// the help function to emit JSON when both --help and --agent are set.
func installAgentHelp(root *cobra.Command) {
	// Add --agent as a persistent flag on root
	root.PersistentFlags().Bool("agent", false, "Output help as structured JSON for agent discovery")

	// Walk all commands and wrap their help
	walkCommands(root, func(cmd *cobra.Command) {
		original := cmd.HelpFunc()
		cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
			agentFlag, _ := c.Flags().GetBool("agent")
			if agentFlag {
				schema := buildHelpSchema(c)
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(schema); err != nil {
					fmt.Fprintf(c.ErrOrStderr(), "Error encoding agent help: %v\n", err) //nolint:errcheck // best-effort stderr
				}
				return
			}
			original(c, args)
		})
	})
}

func walkCommands(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, child := range cmd.Commands() {
		walkCommands(child, fn)
	}
}

func buildHelpSchema(cmd *cobra.Command) AgentHelpSchema {
	schema := AgentHelpSchema{
		Name:        cmd.Name(),
		FullCommand: cmd.CommandPath(),
		Description: cmd.Short,
		Usage:       cmd.UseLine(),
		Aliases:     cmd.Aliases,
	}

	if cmd.Long != "" {
		schema.Description = cmd.Long
	}

	// Collect examples
	if cmd.Example != "" {
		schema.Examples = []string{cmd.Example}
	}

	// Collect local flags
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" || f.Name == "agent" {
			return
		}
		schema.Flags = append(schema.Flags, AgentFlagSchema{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		})
	})

	// Collect inherited flags
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" || f.Name == "agent" {
			return
		}
		schema.Flags = append(schema.Flags, AgentFlagSchema{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
			Global:      true,
		})
	})

	// Recurse into subcommands
	for _, child := range cmd.Commands() {
		if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
			continue
		}
		schema.Subcommands = append(schema.Subcommands, buildHelpSchema(child))
	}

	return schema
}

