package sdk

import "context"

// IPRanges is the set of DeployHQ IP ranges and ports customers need to
// allowlist so DeployHQ can reach their servers (SSH/FTP) and so their firewall
// admits DeployHQ's build and network-agent traffic. Served by GET /ip_ranges,
// which is public (no authentication required).
type IPRanges struct {
	// Shared holds the IPs shared across all zones.
	Shared IPRangeSet `json:"shared"`
	// Zones maps a zone identifier to its zone-specific ranges.
	Zones map[string]IPRangeZone `json:"zones"`
	// NetworkAgent holds the IPs used by the DeployHQ network agent.
	NetworkAgent IPRangeSet `json:"network_agent"`
	// AllIPv4 is the flattened union of every IPv4 range (shared + zones + agent).
	AllIPv4 []string `json:"all_ipv4"`
	// AllIPv6 is the flattened union of every IPv6 range.
	AllIPv6 []string `json:"all_ipv6"`
	// Ports lists the protocol/port descriptions DeployHQ connects on.
	Ports []IPRangePort `json:"ports"`
}

// IPRangeSet is a pair of IPv4 and IPv6 CIDR ranges.
type IPRangeSet struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// IPRangeZone is a named zone's IP ranges.
type IPRangeZone struct {
	Name string   `json:"name"`
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// IPRangePort describes a protocol/port DeployHQ uses when connecting to a
// customer's servers.
type IPRangePort struct {
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
}

// GetIPRanges returns DeployHQ's published IP ranges and ports for firewall
// allowlisting. The endpoint is public and requires no authentication.
func (c *Client) GetIPRanges(ctx context.Context) (*IPRanges, error) {
	var resp IPRanges
	if err := c.get(ctx, "/ip_ranges", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
