package commands

import "strings"

// meteredResourcesInBeta is the single switch that gates the "free during beta"
// messaging for DeployHQ's metered managed resources (Managed VPS and Static
// Hosting). While true, the CLI presents these resources as free for early
// customers during the beta and frames the listed monthly price as the rate
// that applies once the beta ends.
//
// Flip this to false when managed resources leave beta — every piece of runtime
// CLI copy keyed off it (via the helpers below) updates automatically.
//
// NOTE: the markdown docs (README.md, skills/deployhq/SKILL.md,
// skills/deployhq/references/launch.md) repeat this wording and cannot read this
// variable — update them in the same change when you flip it.
var meteredResourcesInBeta = true

// managedVPSAcknowledgePhrase describes — without a specific rate — what a user
// is acknowledging when they provision a Managed VPS. Used in flag help and the
// non-interactive cost gate.
func managedVPSAcknowledgePhrase() string {
	if meteredResourcesInBeta {
		return "free for early customers during beta, billed monthly afterwards"
	}
	return "billed monthly"
}

// managedVPSCostDescription renders a Managed VPS's cost given its monthly rate
// (e.g. "$6.00/month"). A rate that does not start with "$" is treated as an
// unknown/fallback phrase. During the beta the description leads with "free
// during beta".
func managedVPSCostDescription(rate string) string {
	hasPrice := strings.HasPrefix(rate, "$")
	switch {
	case meteredResourcesInBeta && hasPrice:
		return "free during beta, then " + rate
	case meteredResourcesInBeta:
		return "free for early customers during beta"
	default:
		return rate
	}
}

// managedRunningCostTail is appended when warning that a provisioned (but
// not-yet-deployed) Managed VPS is still running and should be cleaned up.
func managedRunningCostTail() string {
	if meteredResourcesInBeta {
		return " (free during beta)"
	}
	return " and billable"
}

// betaFreeSuffix is a light-touch "(free during beta)" qualifier for managed-
// resource copy (Static Hosting or Managed VPS), or "" once metered resources
// are generally available.
func betaFreeSuffix() string {
	if meteredResourcesInBeta {
		return " (free during beta)"
	}
	return ""
}
