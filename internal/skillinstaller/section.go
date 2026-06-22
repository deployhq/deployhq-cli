package skillinstaller

import (
	"regexp"
	"strings"
)

// Several agents (Windsurf, GitHub Copilot, Codex CLI) all want to share a
// single instructions file with the user's own content. We solve that by
// owning only a sentinel-bounded section in the file and preserving
// everything outside it.
//
// The markers, regex, and merge logic are all identical across those
// targets, so they live here. Per-target files just supply the body of
// the section (intros, ref paths, etc.) and call mergeSection.
const (
	sectionBeginPrefix = "<!-- BEGIN deployhq-skill v"
	sectionBeginSuffix = " -->"
	sectionEndMarker   = "<!-- END deployhq-skill -->"
)

// sectionRE matches our owned block including both markers, plus any
// surrounding blank lines so repeated installs don't leave growing gaps in
// the file. (?s) makes . match newlines.
var sectionRE = regexp.MustCompile(
	`(?s)\n*<!-- BEGIN deployhq-skill v[^ ]* -->.*?<!-- END deployhq-skill -->\n*`,
)

// parseSectionVersion returns the version string embedded in the BEGIN
// marker (the bare token between "v" and " -->"), or "" if no section is
// present in the body.
func parseSectionVersion(body string) string {
	idx := strings.Index(body, sectionBeginPrefix)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(sectionBeginPrefix):]
	end := strings.Index(rest, sectionBeginSuffix)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// mergeSection returns existing with the DeployHQ section replaced (if
// present) or appended (if not). The user's own content — anything outside
// the sentinel markers — is preserved byte-for-byte.
//
// Re-runs converge: mergeSection(mergeSection(x, s), s) == mergeSection(x, s).
// That's what makes 'dhq skills install' safely idempotent.
func mergeSection(existing, section string) string {
	if loc := sectionRE.FindStringIndex(existing); loc != nil {
		pre := strings.TrimRight(existing[:loc[0]], "\n")
		post := strings.TrimLeft(existing[loc[1]:], "\n")
		return joinAround(pre, section, post)
	}
	return joinAround(strings.TrimRight(existing, "\n"), section, "")
}
