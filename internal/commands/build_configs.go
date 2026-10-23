package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newBuildConfigsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-configs",
		Short: "Manage build configurations",
		Long: `The build environment for a project — which language versions, OS image, and runtime tooling are available to build commands. Pair with "dhq language-versions" to discover what's pinnable.

Each project has one default build config (read-only) and may have overrides that customize the environment for specific servers. Only overrides can be updated or deleted. To customize the environment, create an override with "dhq build-configs create --server <server>" and set its language versions with "dhq build-languages set --build-config". An override applies only to the servers it targets.`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list", Short: "List build configurations",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				configs, err := client.ListBuildConfigs(cliCtx.Background(), projectID, nil)
				if err != nil {
					return err
				}
				env := cliCtx.Envelope
				if env.WantsJSON() {
					return env.WriteJSON(output.NewResponse(configs, fmt.Sprintf("%d build configs", len(configs))))
				}
				rows := make([][]string, len(configs))
				for i, c := range configs {
					kind := "(override)"
					if c.Default {
						kind = "(default)"
					}
					rows[i] = []string{c.Identifier, kind}
				}
				env.WriteTable([]string{"Identifier", "Type"}, rows)
				return nil
			},
		},
		&cobra.Command{
			Use: "show <id>", Short: "Show a build configuration", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				config, err := client.GetBuildConfig(cliCtx.Background(), projectID, args[0])
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(config, config.Identifier))
			},
		},
		&cobra.Command{
			Use: "default", Short: "Show default build configuration",
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				config, err := client.GetDefaultBuildConfig(cliCtx.Background(), projectID)
				if err != nil {
					return err
				}
				return cliCtx.Envelope.WriteJSON(output.NewResponse(config, "Default build config"))
			},
		},
		newBuildConfigsCreateCmd(),
		newBuildConfigsUpdateCmd(),
		&cobra.Command{
			Use: "delete <id>", Short: "Delete a build configuration", Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				projectID, err := cliCtx.RequireProject()
				if err != nil {
					return err
				}
				client, err := cliCtx.Client()
				if err != nil {
					return err
				}
				if err := client.DeleteBuildConfig(cliCtx.Background(), projectID, args[0]); err != nil {
					return err
				}
				cliCtx.Envelope.Status("Deleted build config: %s", args[0])
				return nil
			},
		},
	)
	return cmd
}

// packagesNotSupportedError explains why --packages is rejected: DeployHQ
// ignores language versions sent with a build configuration create or update,
// so accepting the flag would silently do nothing.
func packagesNotSupportedError() error {
	return &output.UserError{
		Message: "--packages is not applied by DeployHQ when creating or updating a build configuration",
		Hint:    "Set language versions on an override with: dhq build-languages set <language> --version <version> --build-config <override-id>",
	}
}

const buildConfigServerFlagUsage = "Server this override applies to (name or identifier; repeatable or comma-separated). Replaces the override's current servers"

// resolveBuildConfigServers turns --server values into server identifiers.
// Identifiers pass through without an API call; names are matched against the
// project's servers. Only servers are accepted: a build configuration override
// is matched against the individual servers in a deployment.
func resolveBuildConfigServers(ctx context.Context, client *sdk.Client, projectID string, inputs []string) ([]string, error) {
	var (
		ids     []string
		seen    = map[string]bool{}
		servers []sdk.Server
		loaded  bool
	)
	for _, raw := range inputs {
		in := strings.TrimSpace(raw)
		if in == "" {
			continue
		}
		id := in
		if !isUUID(in) {
			if !loaded {
				var err error
				if servers, err = client.ListServers(ctx, projectID, nil); err != nil {
					return nil, err
				}
				loaded = true
			}
			var candidates []sdk.Server
			id, candidates = resolveServerName(in, servers)
			if id == "" {
				names := make([]string, len(candidates))
				for i, c := range candidates {
					names[i] = fmt.Sprintf("%s (%s)", c.Name, c.Identifier)
				}
				hint := "No servers in this project — create one first with: dhq servers create"
				if len(names) > 0 {
					hint = "Available: " + strings.Join(names, ", ")
				}
				return nil, &output.UserError{Message: fmt.Sprintf("No unique server matches %q", in), Hint: hint}
			}
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, &output.UserError{
			Message: "--server needs at least one server name or identifier",
			Hint:    "List servers with: dhq servers list",
		}
	}
	return ids, nil
}

// targetBuildConfig points an override at servers and warns when DeployHQ
// kept fewer servers than were sent (unknown identifiers are dropped).
func targetBuildConfig(client *sdk.Client, projectID, configID string, serverIDs []string) (*sdk.BuildConfig, error) {
	config, err := client.UpdateBuildConfig(cliCtx.Background(), projectID, configID, sdk.BuildConfigCreateRequest{ServerIdentifiers: serverIDs})
	if err != nil {
		return nil, err
	}
	if len(config.Servers) < len(serverIDs) {
		cliCtx.Envelope.Warn("Build config %s targets %d of the %d servers given; unknown server identifiers were ignored. Check with: dhq build-configs show %s", config.Identifier, len(config.Servers), len(serverIDs), config.Identifier)
	}
	return config, nil
}

func newBuildConfigsCreateCmd() *cobra.Command {
	var (
		packages string
		servers  []string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a build configuration override",
		Long: `Create a build configuration override. An override only applies to the servers it targets, so pass --server to choose them; an override with no servers never applies to a deployment.

Set language versions on the new override with "dhq build-languages set <language> --version <version> --build-config <override-id>".`,
		Example: `  dhq build-configs create -p my-app --server production --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if packages != "" {
				return packagesNotSupportedError()
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			var serverIDs []string
			if cmd.Flags().Changed("server") {
				if serverIDs, err = resolveBuildConfigServers(cliCtx.Background(), client, projectID, servers); err != nil {
					return err
				}
			}
			config, err := client.CreateBuildConfig(cliCtx.Background(), projectID, sdk.BuildConfigCreateRequest{})
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if len(serverIDs) > 0 {
				targeted, err := targetBuildConfig(client, projectID, config.Identifier, serverIDs)
				if err != nil {
					return &output.UserError{
						Message: fmt.Sprintf("Created build config %s but could not target it at servers: %v", config.Identifier, err),
						Hint:    fmt.Sprintf("It will not apply until targeted. Retry with: dhq build-configs update %s --server <server>", config.Identifier),
					}
				}
				config = targeted
			} else {
				env.Warn("Build config %s targets no servers, so it will not apply to any deployment. Target it with: dhq build-configs update %s --server <server>", config.Identifier, config.Identifier)
			}
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(config, fmt.Sprintf("Created: %s", config.Identifier)))
			}
			env.Status("Created build config: %s", config.Identifier)
			return nil
		},
	}
	cmd.Flags().StringVar(&packages, "packages", "", "Not supported; use 'dhq build-languages set --build-config' instead")
	cmd.Flags().StringSliceVar(&servers, "server", nil, buildConfigServerFlagUsage)
	return cmd
}

func newBuildConfigsUpdateCmd() *cobra.Command {
	var (
		packages string
		servers  []string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Change which servers a build configuration override applies to",
		Long: `Change which servers a build configuration override applies to. --server replaces the override's current servers. A server can only be targeted by one override.

Set language versions on an override with "dhq build-languages set <language> --version <version> --build-config <id>".`,
		Example: `  dhq build-configs update <override-id> -p my-app --server production --server staging`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if packages != "" {
				return packagesNotSupportedError()
			}
			if !cmd.Flags().Changed("server") {
				return &output.UserError{
					Message: "Nothing to update: pass --server",
					Hint:    fmt.Sprintf("Example: dhq build-configs update %s --server <server>", args[0]),
				}
			}
			projectID, err := cliCtx.RequireProject()
			if err != nil {
				return err
			}
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}
			serverIDs, err := resolveBuildConfigServers(cliCtx.Background(), client, projectID, servers)
			if err != nil {
				return err
			}
			config, err := targetBuildConfig(client, projectID, args[0], serverIDs)
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(config, fmt.Sprintf("Updated: %s", config.Identifier)))
			}
			env.Status("Updated build config: %s (%d servers)", config.Identifier, len(config.Servers))
			return nil
		},
	}
	cmd.Flags().StringVar(&packages, "packages", "", "Not supported; use 'dhq build-languages set --build-config' instead")
	cmd.Flags().StringSliceVar(&servers, "server", nil, buildConfigServerFlagUsage)
	return cmd
}
