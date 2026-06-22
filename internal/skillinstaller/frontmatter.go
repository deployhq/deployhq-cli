package skillinstaller

import "strings"

// splitSkillFrontmatter strips the leading YAML frontmatter from SKILL.md
// and pulls out the `description:` value. The frontmatter uses the literal
// block style (`description: |`) followed by an indented block, so we
// can't lean on a regex match against the same line.
//
// Returns (description, body). If no frontmatter is found, returns ("", input).
func splitSkillFrontmatter(input []byte) (description string, body []byte) {
	s := string(input)
	if !strings.HasPrefix(s, "---\n") {
		return "", input
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return "", input
	}
	frontmatter := s[4 : 4+end]
	rest := s[4+end+len("\n---"):]
	rest = strings.TrimPrefix(rest, "\n")

	description = extractDescription(frontmatter)
	return description, []byte(rest)
}

// extractDescription reads the YAML `description:` field, handling both the
// inline form (`description: foo`) and the literal block form
// (`description: |` followed by indented lines). Returns an empty string if
// the field is absent or unparseable.
func extractDescription(frontmatter string) string {
	lines := strings.Split(frontmatter, "\n")
	for i, line := range lines {
		trim := strings.TrimRight(line, "\r")
		if !strings.HasPrefix(trim, "description:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trim, "description:"))
		if rest != "|" && rest != ">" && rest != "" {
			return rest
		}
		var out []string
		for j := i + 1; j < len(lines); j++ {
			l := lines[j]
			if l == "" {
				out = append(out, "")
				continue
			}
			if !strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "\t") {
				break
			}
			out = append(out, strings.TrimSpace(l))
		}
		return strings.TrimSpace(strings.Join(out, " "))
	}
	return ""
}

// oneLine collapses internal whitespace into single spaces — useful when
// a multi-line YAML block description has to fit on a single-line field
// (e.g. Cursor's .mdc frontmatter).
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
