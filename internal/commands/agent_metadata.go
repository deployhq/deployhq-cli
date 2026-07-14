package commands

// AgentMetadata describes machine-readable safety and automation properties
// for a command, consumed by AI agents via `dhq commands --json`.
type AgentMetadata struct {
	Interactive          bool     `json:"interactive"`                     // may prompt the user
	Destructive          bool     `json:"destructive"`                     // deletes or overwrites data
	Idempotent           bool     `json:"idempotent"`                      // safe to retry
	RequiresConfirmation bool     `json:"requires_confirmation,omitempty"` // agent should confirm before running
	SupportsJSON         bool     `json:"supports_json"`                   // honours --json
	SafeForAutomation    bool     `json:"safe_for_automation"`             // deterministic in non-interactive mode
	ResourceTypes        []string `json:"resource_types,omitempty"`        // e.g. ["deployment", "server"]
}

// commandMetadataTable maps full command paths (e.g. "dhq deploy", "dhq projects delete")
// to their agent metadata. Commands not in this table get sensible defaults
// from defaultMetadata().
var commandMetadataTable = map[string]AgentMetadata{
	// Shortcuts
	"dhq deploy": {
		Interactive: true, Destructive: false, Idempotent: false,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
	},
	"dhq rollback": {
		Destructive: false, Idempotent: false,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
	},
	"dhq retry": {
		Idempotent:   false,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
	},
	"dhq launch": {
		// Provisions a project/server (Managed VPS or Static Hosting) and deploys.
		// Re-runs resolve the existing project/server from .deployhq.toml rather than
		// double-provisioning (idempotent). A Managed VPS is a managed resource (see
		// managedVPSAcknowledgePhrase in metered.go for the current cost framing), so
		// agents should confirm — and the command itself requires --accept-cost in
		// non-interactive mode before provisioning a Managed VPS.
		Interactive: true, Destructive: false, Idempotent: true,
		RequiresConfirmation: true,
		SupportsJSON:         true, SafeForAutomation: true,
		ResourceTypes: []string{"project", "server", "deployment"},
	},

	// Projects
	"dhq projects list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"project"},
	},
	"dhq projects show": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"project"},
	},
	"dhq projects create": {
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"project"},
	},
	"dhq projects update": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"project"},
	},
	"dhq projects delete": {
		Destructive: true, RequiresConfirmation: true,
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"project"},
	},

	// Servers
	"dhq servers list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"server"},
	},
	"dhq servers show": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"server"},
	},
	"dhq servers create": {
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"server"},
	},
	"dhq servers delete": {
		Destructive: true, RequiresConfirmation: true,
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"server"},
	},

	// Deployments
	"dhq deployments list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
	},
	"dhq deployments show": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
	},
	"dhq deployments create": {
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
	},
	"dhq deployments abort": {
		Destructive: true, Idempotent: true,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
	},
	"dhq deployments logs": {
		Idempotent: true, SupportsJSON: false, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
	},
	"dhq deployments watch": {
		Interactive: true, Idempotent: true,
		SupportsJSON: false, SafeForAutomation: false,
		ResourceTypes: []string{"deployment"},
	},

	// Environment variables
	"dhq env-vars list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"env_var"},
	},
	"dhq env-vars create": {
		Interactive: true, // prompts for value if --value omitted
		Idempotent:  false, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"env_var"},
	},
	"dhq env-vars delete": {
		Destructive: true, RequiresConfirmation: true,
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"env_var"},
	},

	// Operations
	"dhq doctor": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
	},
	"dhq activity list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
	},
	"dhq status": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
	},
	"dhq test-access": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"server"},
	},
	"dhq api": {
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
	},

	// Auth & Setup
	"dhq auth login": {
		Interactive: true, Idempotent: true,
		SupportsJSON: false, SafeForAutomation: true,
	},
	"dhq auth logout": {
		Interactive: true, Idempotent: true,
		SupportsJSON: false, SafeForAutomation: true,
	},
	"dhq auth status": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
	},
	"dhq auth token": {
		Idempotent: true, SupportsJSON: false, SafeForAutomation: true,
	},
	"dhq init": {
		Interactive:  true,
		SupportsJSON: false, SafeForAutomation: false,
	},
	"dhq hello": {
		Interactive:  true,
		SupportsJSON: false, SafeForAutomation: false,
	},
	"dhq configure": {
		Interactive:  true,
		SupportsJSON: false, SafeForAutomation: false,
	},
	"dhq signup": {
		Interactive:  true,
		SupportsJSON: true, SafeForAutomation: true,
	},
	"dhq mcp": {
		Interactive:  true,
		SupportsJSON: false, SafeForAutomation: false,
	},

	// Configuration resources (all non-interactive CRUD)
	"dhq config-files list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"config_file"},
	},
	"dhq config-files create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"config_file"},
	},
	"dhq config-files delete": {
		Destructive: true, RequiresConfirmation: true,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"config_file"},
	},
	"dhq build-commands list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"build_command"},
	},
	"dhq build-commands create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"build_command"},
	},
	"dhq excluded-files list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"excluded_file"},
	},
	"dhq excluded-files create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"excluded_file"},
	},
	"dhq ssh-commands list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"ssh_command"},
	},
	"dhq ssh-commands create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"ssh_command"},
	},
	"dhq deployment-checks list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment_check"},
	},
	"dhq deployment-checks show": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment_check"},
	},
	"dhq deployment-checks create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment_check"},
	},
	"dhq deployment-checks update": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment_check"},
	},
	"dhq deployment-checks delete": {
		Destructive: true, RequiresConfirmation: true,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment_check"},
	},

	// Repos
	"dhq repos show": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"repository"},
	},
	"dhq repos create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"repository"},
	},
	"dhq repos branches": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"repository"},
	},
	"dhq repos commits": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"repository"},
	},

	// Global resources
	"dhq global-servers list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"server"},
	},
	"dhq global-env-vars list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"env_var"},
	},
	"dhq ssh-keys list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"ssh_key"},
	},
	"dhq ssh-keys download": {
		// Emits raw private key material. Idempotent, but sensitive enough that
		// an agent should confirm before running and not treat it as safe to run
		// unattended (the key would land in logs/transcripts). Requires an admin
		// on a paid account (else 403).
		Idempotent: true, RequiresConfirmation: true,
		SupportsJSON: true, SafeForAutomation: false,
		ResourceTypes: []string{"ssh_key"},
	},
	// Users
	"dhq users list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"user"},
	},
	"dhq users show": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"user"},
	},
	"dhq users create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"user"},
	},
	"dhq users update": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"user"},
	},
	"dhq users delete": {
		Destructive: true, RequiresConfirmation: true,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"user"},
	},
	"dhq users resend-invitation": {
		// Not idempotent: each call sends a fresh invitation email, so agents
		// should not treat it as blindly retry-safe.
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"user"},
	},

	// Account
	"dhq account get": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"account"},
	},
	"dhq account update": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"account"},
	},
	"dhq account billing": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"account"},
	},

	// Profile
	"dhq profile get": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"profile"},
	},
	"dhq profile update": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"profile"},
	},

	// API keys
	"dhq api-keys create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"api_key"},
	},
	"dhq api-keys delete": {
		Destructive: true, RequiresConfirmation: true,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"api_key"},
	},

	"dhq folders list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"folder"},
	},
	"dhq folders create": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"folder"},
	},
	"dhq folders update": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"folder"},
	},
	"dhq folders delete": {
		Destructive: true, RequiresConfirmation: true,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"folder"},
	},
	"dhq templates list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"template"},
	},

	// Hosted resources (Managed VPS + Static Hosting lifecycle)
	"dhq hosted-resources list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"hosted_resource"},
	},
	"dhq hosted-resources show": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"hosted_resource"},
	},
	"dhq hosted-resources sync": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"hosted_resource"},
	},
	"dhq hosted-resources retry-provision": {
		// Only valid when the resource is in the error state; not blindly retry-safe.
		Idempotent: false, SupportsJSON: true, SafeForAutomation: false,
		ResourceTypes: []string{"hosted_resource"},
	},

	// Managed hosting catalog (read-only)
	"dhq managed-hosting regions": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"managed_hosting_region"},
	},
	"dhq managed-hosting sizes": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"managed_hosting_size"},
	},

	// Template sub-resources (all take -t <template>)
	"dhq templates config-files list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates config-files show":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates config-files create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates config-files update": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates config-files delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates excluded-files list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates excluded-files create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates excluded-files update": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates excluded-files delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates integrations list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates integrations create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates integrations update": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates integrations delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates commands list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates commands create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates commands update": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates commands delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates build-commands list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates build-commands create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates build-commands update": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates build-commands delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates build-cache-files list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates build-cache-files create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates build-cache-files update": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates build-cache-files delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates build-known-hosts list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates build-known-hosts create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates build-known-hosts delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates build-languages set": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates build-configuration": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	"dhq templates servers list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template", "server"}},
	"dhq templates servers show":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template", "server"}},
	"dhq templates servers create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template", "server"}},
	"dhq templates servers update": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template", "server"}},
	"dhq templates servers delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template", "server"}},

	"dhq templates server-groups list":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates server-groups create": {SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates server-groups update": {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},
	"dhq templates server-groups delete": {Destructive: true, RequiresConfirmation: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"template"}},

	// Phase 4 — project & server actions
	"dhq projects regenerate-key": {
		// Invalidates the existing deploy key — servers must be updated with the new one.
		Destructive: true, RequiresConfirmation: true,
		SupportsJSON: true, SafeForAutomation: false,
		ResourceTypes: []string{"project"},
	},
	"dhq projects undeployed-changes": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"project"},
	},
	"dhq projects ai-overview": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"project"},
	},
	"dhq servers from-global": {
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"server"},
	},
	"dhq servers metrics": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"server"},
	},
	"dhq config-files link-global":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"config_file"}},
	"dhq config-files unlink-global": {Destructive: true, Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"config_file"}},
	"dhq ssh-commands link-global":   {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"ssh_command"}},
	"dhq ssh-commands unlink-global": {Destructive: true, Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"ssh_command"}},

	// Phase 7 — long-tail resources
	"dhq ip-ranges":         {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"ip_range"}},
	"dhq plans":             {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"package"}},
	"dhq invoices list":     {Idempotent: true, SupportsJSON: true, SafeForAutomation: true, ResourceTypes: []string{"invoice"}},
	"dhq invoices download": {Idempotent: true, SupportsJSON: false, SafeForAutomation: true, ResourceTypes: []string{"invoice"}},
	"dhq detect":            {Idempotent: true, SupportsJSON: true, SafeForAutomation: true},
	"dhq beta enroll": {
		// Admin-gated (403 for non-admins); re-enrolling is a no-op.
		Idempotent: true, RequiresConfirmation: true,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"beta_enrollment"},
	},

	"dhq zones list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"zone"},
	},

	// Meta
	"dhq commands": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
	},
	"dhq version": {
		Idempotent: true, SupportsJSON: false, SafeForAutomation: true,
	},
	"dhq update": {
		Idempotent: true, SupportsJSON: false, SafeForAutomation: true,
	},
	"dhq config show": {
		Idempotent: true, SupportsJSON: false, SafeForAutomation: true,
	},
	"dhq config set": {
		Idempotent: true, SupportsJSON: false, SafeForAutomation: true,
	},
}

// lookupAgentMetadata returns agent metadata for a command path,
// falling back to sensible defaults.
func lookupAgentMetadata(commandPath string) AgentMetadata {
	if m, ok := commandMetadataTable[commandPath]; ok {
		return m
	}
	return defaultMetadata()
}

// defaultMetadata returns conservative defaults for unaudited commands:
// not safe for automation, not idempotent, no JSON support assumed.
// Commands must be explicitly added to commandMetadataTable to be marked safe.
func defaultMetadata() AgentMetadata {
	return AgentMetadata{
		Interactive:       false,
		Destructive:       false,
		Idempotent:        false,
		SupportsJSON:      false,
		SafeForAutomation: false,
	}
}
