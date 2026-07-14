package commands

import (
	"fmt"
	"strings"

	"github.com/deployhq/deployhq-cli/internal/output"
	"github.com/spf13/cobra"
)

func newIPRangesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ip-ranges",
		Short: "List DeployHQ IP ranges for firewall allowlisting",
		Long: `The IP ranges and ports DeployHQ connects from when deploying to your servers.

Add these to your server firewall's allowlist so DeployHQ can reach it over SSH/FTP,
and so DeployHQ's build and network-agent traffic is admitted. This endpoint is public.

Use --json for the full structure (shared, per-zone, and network-agent ranges); the
default table shows the flattened all-IPv4/all-IPv6 union plus the ports used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cliCtx.PublicClient()
			if err != nil {
				return err
			}
			ranges, err := client.GetIPRanges(cliCtx.Background())
			if err != nil {
				return err
			}
			env := cliCtx.Envelope
			if env.WantsJSON() {
				return env.WriteJSON(output.NewResponse(ranges,
					fmt.Sprintf("%d IPv4 and %d IPv6 ranges", len(ranges.AllIPv4), len(ranges.AllIPv6))))
			}

			rows := make([][]string, 0, len(ranges.AllIPv4)+len(ranges.AllIPv6))
			for _, ip := range ranges.AllIPv4 {
				rows = append(rows, []string{"IPv4", ip})
			}
			for _, ip := range ranges.AllIPv6 {
				rows = append(rows, []string{"IPv6", ip})
			}
			env.WriteTable([]string{"Family", "Range"}, rows)

			if len(ranges.Ports) > 0 {
				parts := make([]string, len(ranges.Ports))
				for i, p := range ranges.Ports {
					parts[i] = fmt.Sprintf("%s (%s)", p.Protocol, p.Description)
				}
				env.Status("\nPorts: %s", strings.Join(parts, ", "))
			}
			env.Status("\nTip: dhq ip-ranges --json for shared, per-zone, and network-agent ranges")
			return nil
		},
	}
	return cmd
}
