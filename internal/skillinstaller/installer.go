// Package skillinstaller detects locally-installed AI coding agents and writes
// the bundled DeployHQ skill into each agent's config directory.
//
// This is the Wrangler-style onboarding pattern: after `dhq hello` logs the
// user in, we check which agents are present on disk and offer to install the
// skill so the agent can drive the CLI competently.
//
// Each agent has a different idea of what a "skill" is (Claude Code has a
// formal skills/ directory; Cursor uses .cursor/rules/*.mdc; Copilot uses a
// single .github/copilot-instructions.md; etc.). Each one is implemented as a
// Target so the quirks stay isolated.
package skillinstaller

import (
	"fmt"
	"sort"
)

// Scope tells the post-login prompt how aggressive it should be about
// installing a target without explicit user consent.
//
// User-scope targets write only to the user's home directory and never
// touch project files — safe to install during 'dhq hello' after a Y/n
// prompt, or silently for the runtime agent.
//
// Project-scope targets modify the current repo (e.g. .github/). They are
// always opt-in via 'dhq skills install --agent <name>' so we never mutate
// a user's project as a side effect of logging in.
type Scope int

const (
	ScopeUser Scope = iota
	ScopeProject
)

// Status describes the install state of a Target on this machine.
type Status int

const (
	// StatusNotInstalled means the target agent is not installed on this
	// machine (or we can't tell — we treat absence of the config dir as
	// "not present" rather than fail-loud).
	StatusNotInstalled Status = iota
	// StatusAvailable means the agent is installed but the skill is not.
	StatusAvailable
	// StatusInstalled means the skill is already installed at the same or
	// newer version. Callers should skip re-installing.
	StatusInstalled
	// StatusOutdated means the skill is installed but at an older version.
	// Callers may upgrade without re-prompting.
	StatusOutdated
)

func (s Status) String() string {
	switch s {
	case StatusNotInstalled:
		return "not-installed"
	case StatusAvailable:
		return "available"
	case StatusInstalled:
		return "installed"
	case StatusOutdated:
		return "outdated"
	default:
		return "unknown"
	}
}

// Target is one AI agent's skill-install integration.
//
// Targets are expected to be cheap to construct and side-effect-free until
// Install is called. Detect should never error — return StatusNotInstalled
// when in doubt.
type Target interface {
	// Name is a stable identifier ("claude-code", "cursor", etc.) used in
	// CLI output, telemetry, and matching against harness.Detect().
	Name() string

	// DisplayName is a human-friendly label ("Claude Code") for prompts.
	DisplayName() string

	// Scope is User for targets that only touch ~/, Project for targets
	// that write into the current repo. The hello-flow prompt only
	// auto-offers User-scope targets.
	Scope() Scope

	// Detect reports whether this agent is installed locally and the
	// current install state of the skill.
	Detect() Status

	// Install writes the skill into the agent's config directory. It must
	// be idempotent — safe to re-run over an existing install. Returns a
	// short human-readable summary on success (e.g. the path written to)
	// for use in CLI status output.
	Install() (string, error)
}

// registry is the global list of known targets. Each target self-registers in
// an init() so adding a new agent is a single-file change.
var registry []Target

// Register adds a Target to the global registry. Intended for use from
// per-target init() functions.
func Register(t Target) {
	registry = append(registry, t)
}

// All returns every registered target, sorted by Name for stable output.
func All() []Target {
	out := make([]Target, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// DetectResult pairs a Target with its current Status.
type DetectResult struct {
	Target Target
	Status Status
}

// Noter is an optional Target capability. A target that implements it gets
// a chance to append a free-form note after install — useful when the agent
// needs the user to wire something up manually (e.g. add a `read:` entry to
// a config file we can't safely auto-edit).
//
// Callers (the hello hook, `dhq skills install`) detect this via a type
// assertion and print the note to stderr after the install line.
type Noter interface {
	PostInstallNote() string
}

// DetectInstalled probes every registered target and returns those whose
// agent appears to be installed locally (Status != StatusNotInstalled).
// The runtime agent — the one the user is currently running `dhq` from,
// as reported by harness.Detect — is included even if Status would
// otherwise be StatusInstalled, so the caller can decide whether to
// upgrade silently or skip.
func DetectInstalled() []DetectResult {
	var out []DetectResult
	for _, t := range All() {
		if s := t.Detect(); s != StatusNotInstalled {
			out = append(out, DetectResult{Target: t, Status: s})
		}
	}
	return out
}

// Find returns the Target with the given name, or nil if no such target is
// registered. Matching is case-insensitive on the canonical name.
func Find(name string) Target {
	for _, t := range All() {
		if t.Name() == name {
			return t
		}
	}
	return nil
}

// Needed reports whether the skill needs to be (re)installed for this target.
// Returns true for StatusAvailable and StatusOutdated; false for
// StatusInstalled and StatusNotInstalled.
func Needed(s Status) bool {
	return s == StatusAvailable || s == StatusOutdated
}

// InstallError wraps a target-specific failure with the target name so
// callers can surface partial failures without losing context.
type InstallError struct {
	Target string
	Cause  error
}

func (e *InstallError) Error() string {
	return fmt.Sprintf("install %s: %v", e.Target, e.Cause)
}

func (e *InstallError) Unwrap() error { return e.Cause }
