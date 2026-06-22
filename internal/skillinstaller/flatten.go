package skillinstaller

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/deployhq/deployhq-cli/skills"
)

// Markers for targets that own their skill file entirely — there's no user
// content to coexist with, so we use a single top-of-file HTML comment for
// version tracking rather than the sentinel-section pattern used by
// shared-file targets (Windsurf/Copilot/Codex/Gemini/OpenCode).
//
// Used by Aider, Cline, Continue.
const (
	ownedFileVersionPrefix = "<!-- deployhq-skill v"
	ownedFileVersionSuffix = " -->"
)

// buildOwnedSkillFile flattens the embedded skill tree into a single
// markdown file, prepended with a version-tracking HTML comment.
//
// Output:
//
//	<!-- deployhq-skill v1 -->
//
//	<SKILL.md body (frontmatter stripped) + concatenated references>
func buildOwnedSkillFile(efs fs.FS, root string) ([]byte, error) {
	_, body, err := flattenSkill(efs, root)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "%s%s%s\n\n", ownedFileVersionPrefix, skills.Version, ownedFileVersionSuffix)
	buf.Write(body)
	return []byte(buf.String()), nil
}

// parseOwnedFileVersion returns the version embedded in our top-of-file
// HTML comment, or "" if no marker is present at the start of the body.
//
// The marker MUST be the first thing in the file. We write it that way
// via buildOwnedSkillFile, and if a future read finds content above the
// marker we treat the file as "not ours" — reporting StatusAvailable so
// the next install rewrites cleanly rather than appending to user edits.
func parseOwnedFileVersion(body string) string {
	if !strings.HasPrefix(body, ownedFileVersionPrefix) {
		return ""
	}
	rest := body[len(ownedFileVersionPrefix):]
	end := strings.Index(rest, ownedFileVersionSuffix)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// flattenSkill reads SKILL.md plus every references/*.md from the embedded
// skill tree and returns:
//
//   - description: the value of the YAML `description:` field in SKILL.md's
//     frontmatter (handles the literal-block `description: |` form). Empty
//     when no description is present.
//   - body: SKILL.md's body (frontmatter stripped) followed by each
//     reference file under a `## reference: references/<name>` header,
//     sorted alphabetically for deterministic output.
//
// Targets that ship the skill as a single document (Cursor's .mdc,
// Aider's read-file) build their output by wrapping this body in their
// own header/frontmatter. Targets that ship the tree as-is (Claude Code,
// Windsurf, Codex, Copilot) bypass this and call writeEmbeddedTree
// directly instead.
func flattenSkill(efs fs.FS, root string) (description string, body []byte, err error) {
	skill, err := fs.ReadFile(efs, path.Join(root, "SKILL.md"))
	if err != nil {
		return "", nil, fmt.Errorf("read SKILL.md: %w", err)
	}
	description, skillBody := splitSkillFrontmatter(skill)

	var buf strings.Builder
	buf.Write(skillBody)
	if !strings.HasSuffix(string(skillBody), "\n") {
		buf.WriteByte('\n')
	}

	refsDir := path.Join(root, "references")
	entries, err := fs.ReadDir(efs, refsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return description, []byte(buf.String()), nil
		}
		return "", nil, fmt.Errorf("read references/: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(efs, path.Join(refsDir, e.Name()))
		if err != nil {
			return "", nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		fmt.Fprintf(&buf, "\n\n## reference: references/%s\n\n", e.Name())
		buf.Write(data)
		if !strings.HasSuffix(string(data), "\n") {
			buf.WriteByte('\n')
		}
	}

	return description, []byte(buf.String()), nil
}
