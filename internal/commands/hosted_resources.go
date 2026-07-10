package commands

import (
	"fmt"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/deployhq/deployhq-cli/pkg/sdk"
	"github.com/spf13/cobra"
)

func newHostedResourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "hosted-resources",
		Aliases: []string{"hosted-resource"},
		Short:   "Manage hosted resources (Managed VPS + Static Hosting)",
		Long: `Managed hosting resources for the account: Managed VPS droplets (kind "hosted_resource") and Static Hosting websites (kind "hosted_website").

This is an admin + beta-features area; without both you will get a permission error. Inspect a resource with "dhq hosted-resources show <id>", re-sync provider state with "sync", and retry a failed provision with "retry-provision".`,
	}

	cmd.AddCommand(
		newHostedResourcesListCmd(),
		newHostedResourcesShowCmd(),
		newHostedResourcesSyncCmd(),
		newHostedResourcesRetryProvisionCmd(),
	)

	return cmd
}

// hostedResourceLocation returns the value for the "Region / Subdomain" column:
// the region slug for a Managed VPS, or the subdomain for a Static Hosting site.
func hostedResourceLocation(r sdk.HostedResource) string {
	if r.Kind == "hosted_website" {
		return r.Subdomain
	}
	if r.Region != nil {
		return *r.Region
	}
	return ""
}

func newHostedResourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List hosted resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			resources, err := client.ListHostedResources(cliCtx.Background(), nil)
			if err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(resources,
					fmt.Sprintf("%d hosted resources", len(resources)),
					output.Breadcrumb{Action: "show", Cmd: "dhq hosted-resources show <id>"},
				))
			}

			if env.QuietMode {
				ids := make([]string, len(resources))
				for i, r := range resources {
					ids[i] = r.Identifier
				}
				env.WriteQuiet(ids)
				return nil
			}

			columns := []string{"Name", "Identifier", "Kind", "Status", "Region / Subdomain", "Monthly Cost"}
			rows := make([][]string, len(resources))
			for i, r := range resources {
				rows[i] = []string{
					r.Name,
					r.Identifier,
					r.Kind,
					r.Status,
					hostedResourceLocation(r),
					fmt.Sprintf("$%.2f", r.MonthlyCost),
				}
			}
			env.WriteTable(columns, rows)
			return nil
		},
	}
}

func newHostedResourcesShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show hosted resource details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			resource, err := client.GetHostedResource(cliCtx.Background(), args[0])
			if err != nil {
				return err
			}

			return cliCtx.Envelope.WriteJSON(output.NewResponse(resource,
				fmt.Sprintf("Hosted resource: %s", resource.Name),
				output.Breadcrumb{Action: "sync", Cmd: fmt.Sprintf("dhq hosted-resources sync %s", resource.Identifier)},
				output.Breadcrumb{Action: "retry-provision", Cmd: fmt.Sprintf("dhq hosted-resources retry-provision %s", resource.Identifier)},
			))
		},
	}
}

func newHostedResourcesSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <id>",
		Short: "Request a re-sync of a hosted resource's provisioning state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.SyncHostedResource(cliCtx.Background(), args[0]); err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"identifier": args[0], "status": "sync_requested"},
					fmt.Sprintf("Sync requested for %s", args[0]),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq hosted-resources show %s", args[0])},
				))
			}
			env.Status("Sync requested for %s", args[0])
			return nil
		},
	}
}

func newHostedResourcesRetryProvisionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retry-provision <id>",
		Short: "Retry provisioning a hosted resource that is in the error state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.Client()
			if err != nil {
				return err
			}

			if err := client.RetryProvisionHostedResource(cliCtx.Background(), args[0]); err != nil {
				return err
			}

			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(
					map[string]string{"identifier": args[0], "status": "provisioning"},
					fmt.Sprintf("Provisioning retry started for %s", args[0]),
					output.Breadcrumb{Action: "show", Cmd: fmt.Sprintf("dhq hosted-resources show %s", args[0])},
				))
			}
			env.Status("Provisioning retry started for %s", args[0])
			return nil
		},
	}
}
