package detect

import (
	"os"
	"path/filepath"
)

// manifestFiles are the files whose CONTENTS the backend's StackDetector
// pipeline parses (package.json deps, Gemfile gems, composer.json, framework
// config files, …). Their contents are uploaded so remote detection reaches
// full precision; everything else is sent as a name-only listing.
var manifestFiles = []string{
	// Node / JS
	"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lockb",
	"vite.config.js", "vite.config.ts", "vite.config.mjs",
	"next.config.js", "next.config.mjs", "next.config.ts",
	"nuxt.config.js", "nuxt.config.ts",
	"astro.config.mjs", "astro.config.ts",
	"svelte.config.js", "remix.config.js",
	"angular.json", ".eleventy.js", "eleventy.config.js",
	// PHP
	"composer.json", "composer.lock",
	// Ruby
	"Gemfile", "Gemfile.lock",
	// Python
	"requirements.txt", "Pipfile", "pyproject.toml",
	// Go
	"go.mod",
	// Static site generators
	"_config.yml", "_config.yaml", "config.toml", "hugo.toml", "hugo.yaml",
}

// maxManifestBytes caps each uploaded manifest. Vendored monorepo manifests
// (lockfiles especially) can be enormous; detection only needs the head, and
// the server rejects anything larger than its own cap.
const maxManifestBytes = 64 * 1024

// maxManifestFiles bounds how many manifest files we upload (the server caps at 32).
const maxManifestFiles = 32

// CollectManifest gathers the input for remote framework detection from dir:
// the root-directory filename listing (for existence checks) plus the contents
// of any present manifest files (capped per file and in count). Returns plain
// types so this package stays independent of the SDK wire format. Never fails —
// an unreadable directory yields an empty listing.
func CollectManifest(dir string) (filenames []string, files map[string]string) {
	if dir == "" {
		dir = "."
	}
	// Non-nil so the JSON payload is always `"filenames": []`, never `null`
	// (which violates the documented array schema).
	filenames = []string{}
	files = map[string]string{}

	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			filenames = append(filenames, e.Name())
		}
	}

	for _, name := range manifestFiles {
		if len(files) >= maxManifestFiles {
			break
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if len(data) > maxManifestBytes {
			data = data[:maxManifestBytes]
		}
		files[name] = string(data)
	}
	return filenames, files
}
