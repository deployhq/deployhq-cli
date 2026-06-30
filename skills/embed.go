// Package skills exposes the bundled DeployHQ skill files as an embed.FS so
// that other packages (notably internal/skillinstaller) can write them into
// each AI agent's local config directory.
//
// The on-disk layout under skills/deployhq/ is the canonical Claude Code
// skill format: SKILL.md at the root, supporting docs under references/.
// Other agents reshape this into their own format at install time.
package skills

import "embed"

// FS contains the deployhq/ skill directory tree, rooted at "deployhq".
// Iterate with fs.WalkDir or read individual files with FS.ReadFile.
//
//go:embed deployhq
var FS embed.FS

// Version is the schema version of the embedded skill. Installers write this
// alongside the installed files so a future `dhq` can detect stale installs
// and update without re-prompting. Bump when the skill content changes
// meaningfully (new commands, restructured references, etc.).
const Version = "1"
