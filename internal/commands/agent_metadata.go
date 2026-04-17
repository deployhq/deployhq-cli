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
	"dhq auth status": {
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
		SupportsJSON: false, SafeForAutomation: true,
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

// defaultMetadata returns conservative defaults: non-interactive, non-destructive,
// idempotent, supports JSON, safe for automation.
func defaultMetadata() AgentMetadata {
	return AgentMetadata{
		Idempotent:    true,
		SupportsJSON:  true,
		SafeForAutomation: true,
	}
}
