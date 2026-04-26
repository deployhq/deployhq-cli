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
		Idempotent: false,
		SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"deployment"},
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
		Idempotent: false, SupportsJSON: true, SafeForAutomation: true,
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
		Interactive: true,
		SupportsJSON: false, SafeForAutomation: false,
	},
	"dhq hello": {
		Interactive: true,
		SupportsJSON: false, SafeForAutomation: false,
	},
	"dhq configure": {
		Interactive: true,
		SupportsJSON: false, SafeForAutomation: false,
	},
	"dhq signup": {
		Interactive: true,
		SupportsJSON: true, SafeForAutomation: true,
	},
	"dhq mcp": {
		Interactive: true,
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
	"dhq templates list": {
		Idempotent: true, SupportsJSON: true, SafeForAutomation: true,
		ResourceTypes: []string{"template"},
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
