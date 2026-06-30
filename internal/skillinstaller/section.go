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

// sectionRE matches our owned block — a complete BEGIN…END pair — including
// both markers and any surrounding blank lines, so repeated installs don't
// leave growing gaps in the file. The capture group holds the version token
// from the BEGIN marker. (?s) makes . match newlines.
//
// Requiring a full pair (rather than just spotting the BEGIN prefix) is what
// keeps Detect and Install in agreement: a lone/truncated BEGIN, or user
// prose that merely mentions the marker text, is NOT a section. Both
// parseSectionVersion and mergeSection key off this single regex, so they can
// never disagree about whether a section exists.
var sectionRE = regexp.MustCompile(
	`(?s)\n*<!-- BEGIN deployhq-skill v([^ ]*) -->.*?<!-- END deployhq-skill -->\n*`,
)

// parseSectionVersion returns the version string from a complete DeployHQ
// section, or "" if no complete BEGIN…END section is present in the body.
func parseSectionVersion(body string) string {
	m := sectionRE.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// mergeSection returns existing with the DeployHQ section refreshed: every
// complete BEGIN…END block we own is stripped and a single fresh section is
// appended. The user's own content — anything outside the sentinel markers —
// is preserved. Stripping all matches (rather than replacing the first) also
// collapses any accidental duplicate sections left by an earlier bug.
//
// Re-runs converge: mergeSection(mergeSection(x, s), s) == mergeSection(x, s).
// That's what makes 'dhq skills install' safely idempotent.
func mergeSection(existing, section string) string {
	stripped := strings.TrimRight(sectionRE.ReplaceAllString(existing, "\n"), "\n")
	return joinAround(stripped, section, "")
}
