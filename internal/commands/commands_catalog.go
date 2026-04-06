package commands

import (
	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandInfo describes a command for the agent discovery catalog.
type CommandInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Usage       string        `json:"usage"`
	Aliases     []string      `json:"aliases,omitempty"`
	Flags       []FlagInfo    `json:"flags,omitempty"`
	Subcommands []CommandInfo `json:"subcommands,omitempty"`
}

// FlagInfo describes a command flag.
type FlagInfo struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
}

func newCommandsCatalogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "List all commands (agent discovery)",
		Long:  "Output the full command catalog as JSON for AI agent discovery.",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			catalog := buildCatalog(root)

			return cliCtx.Envelope.WriteJSON(output.NewResponse(
				catalog,
				"Full command catalog for agent discovery",
			))
		},
	}
}

func buildCatalog(cmd *cobra.Command) []CommandInfo {
	var catalog []CommandInfo

	for _, child := range cmd.Commands() {
		if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
			continue
		}

		info := CommandInfo{
			Name:        child.Name(),
			Description: child.Short,
			Usage:       child.UseLine(),
			Aliases:     child.Aliases,
		}

		// Collect local flags
		child.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			fi := FlagInfo{
				Name:        f.Name,
				Shorthand:   f.Shorthand,
				Description: f.Usage,
				Default:     f.DefValue,
			}
			// Mark required flags
			if ann := child.Flags().Lookup(f.Name); ann != nil {
				if _, ok := ann.Annotations[cobra.BashCompOneRequiredFlag]; ok {
					fi.Required = true
				}
			}
			info.Flags = append(info.Flags, fi)
		})

		// Collect inherited flags (e.g. --project, --json)
		child.InheritedFlags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			fi := FlagInfo{
				Name:        f.Name,
				Shorthand:   f.Shorthand,
				Description: f.Usage + " (inherited)",
				Default:     f.DefValue,
			}
			info.Flags = append(info.Flags, fi)
		})

		// Recurse into subcommands
		if child.HasSubCommands() {
			info.Subcommands = buildCatalog(child)
		}

		catalog = append(catalog, info)
	}

	return catalog
}
