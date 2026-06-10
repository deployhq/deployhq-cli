// Package detect provides local project framework detection for the dhq launch flow.
//
// Detection reads files in the project root directory and returns a best-guess
// recommendation for the deployment target (static_hosting, managed_vps, or none)
// along with pre-filled build configuration defaults.
//
// The mapping mirrors the one used in the DeployHQ web onboarding wizard (PR #915):
//   - Node apps that produce a static build artifact → static_hosting
//   - Any app with a server-runtime signal (Gemfile, requirements.txt, go.mod, etc.) → managed_vps
//   - No signal detected → empty (own-server / user's choice)
//
// Detection is intentionally heuristic and conservative. False negatives (returning
// empty when a protocol could be inferred) are preferred over false positives
// (suggesting a managed offering the user did not intend).
package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Protocol constants match the API protocol_type values.
const (
	ProtocolStaticHosting = "static_hosting"
	ProtocolManagedVPS    = "managed_vps"
	// ProtocolNone means no signal was detected — the user should choose manually.
	ProtocolNone = ""
)

// Framework identifies the detected frontend/backend framework.
type Framework string

const (
	FrameworkUnknown    Framework = ""
	FrameworkNextJS     Framework = "nextjs"
	FrameworkReact      Framework = "react"
	FrameworkVite       Framework = "vite"
	FrameworkNuxt       Framework = "nuxt"
	FrameworkGatsby     Framework = "gatsby"
	FrameworkAstro      Framework = "astro"
	FrameworkHugo       Framework = "hugo"
	FrameworkJekyll     Framework = "jekyll"
	FrameworkEleventy   Framework = "eleventy"
	FrameworkAngular    Framework = "angular"
	FrameworkSvelte     Framework = "svelte"
	FrameworkVueJS      Framework = "vuejs"
	FrameworkRails      Framework = "rails"
	FrameworkDjango     Framework = "django"
	FrameworkLaravel    Framework = "laravel"
	FrameworkGo         Framework = "go"
	FrameworkNode       Framework = "node"
)

// Result holds the output of a detection pass.
type Result struct {
	// Framework is the detected framework, or FrameworkUnknown when not identified.
	Framework Framework

	// SuggestedProtocol is "static_hosting", "managed_vps", or "" (no suggestion).
	// "" means the caller should ask the user to choose a target manually.
	SuggestedProtocol string

	// BuildCommand is a best-guess build command for the detected framework.
	// Empty string when no build command can be inferred.
	BuildCommand string

	// OutputDir is the build output directory for static hosting frameworks.
	// E.g. "dist", "build", "_site", "out", "public".
	// Empty string for server-runtime frameworks.
	OutputDir string

	// SPA is true when the detected framework is a single-page application
	// that needs all URL paths rewritten to index.html.
	SPA bool
}

// Detect reads the directory at dir and returns a detection Result.
// dir should be the root of the project (the directory containing package.json,
// Gemfile, etc.). If dir is empty it defaults to ".".
//
// Detection never fails: when no signal is recognised, it returns a zero-value
// Result with SuggestedProtocol = "".
func Detect(dir string) Result {
	if dir == "" {
		dir = "."
	}

	// Presence tests — cheap file-exists checks
	has := func(names ...string) bool {
		for _, name := range names {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
		return false
	}

	readFile := func(name string) []byte {
		data, _ := os.ReadFile(filepath.Join(dir, name))
		return data
	}

	// --- Server-runtime signals (checked first; override static signals) ---

	// Ruby / Rails
	if has("Gemfile") {
		framework := FrameworkRails
		if !fileContains(readFile("Gemfile"), "rails") {
			framework = FrameworkUnknown
		}
		return Result{
			Framework:         framework,
			SuggestedProtocol: ProtocolManagedVPS,
		}
	}

	// Python / Django
	if has("requirements.txt", "Pipfile", "pyproject.toml") {
		framework := FrameworkUnknown
		if has("manage.py") && fileContains(readFile("requirements.txt"), "django") {
			framework = FrameworkDjango
		}
		return Result{
			Framework:         framework,
			SuggestedProtocol: ProtocolManagedVPS,
		}
	}

	// PHP / Laravel
	if has("composer.json") {
		framework := FrameworkUnknown
		if fileContains(readFile("composer.json"), "laravel") {
			framework = FrameworkLaravel
		}
		return Result{
			Framework:         framework,
			SuggestedProtocol: ProtocolManagedVPS,
		}
	}

	// Go binary / module (non-static, i.e. no static site generator config)
	if has("go.mod") && !has("config.toml", "config.yaml", "hugo.toml", "hugo.yaml") {
		return Result{
			Framework:         FrameworkGo,
			SuggestedProtocol: ProtocolManagedVPS,
		}
	}

	// --- Static site signals ---

	// Hugo
	if has("hugo.toml", "hugo.yaml", "hugo.json") || (has("config.toml") && fileContains(readFile("config.toml"), "baseURL")) {
		return Result{
			Framework:         FrameworkHugo,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      "hugo",
			OutputDir:         "public",
			SPA:               false,
		}
	}

	// Jekyll
	if has("_config.yml", "_config.yaml") {
		return Result{
			Framework:         FrameworkJekyll,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      "bundle exec jekyll build",
			OutputDir:         "_site",
			SPA:               false,
		}
	}

	// Eleventy
	if has(".eleventy.js", "eleventy.config.js", "eleventy.config.mjs") {
		return Result{
			Framework:         FrameworkEleventy,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      "npx @11ty/eleventy",
			OutputDir:         "_site",
			SPA:               false,
		}
	}

	// --- Node/JS — framework detection from package.json ---
	if has("package.json") {
		return detectNode(readFile("package.json"))
	}

	// Nothing detected.
	return Result{}
}

// detectNode analyses package.json content and returns a Result for Node/JS projects.
func detectNode(pkgJSON []byte) Result {
	if len(pkgJSON) == 0 {
		return Result{}
	}

	var pkg struct {
		Scripts      map[string]string      `json:"scripts"`
		Dependencies map[string]interface{} `json:"dependencies"`
		DevDeps      map[string]interface{} `json:"devDependencies"`
	}
	if err := json.Unmarshal(pkgJSON, &pkg); err != nil {
		return Result{}
	}

	allDeps := make(map[string]struct{})
	for k := range pkg.Dependencies {
		allDeps[k] = struct{}{}
	}
	for k := range pkg.DevDeps {
		allDeps[k] = struct{}{}
	}

	hasDep := func(names ...string) bool {
		for _, n := range names {
			if _, ok := allDeps[n]; ok {
				return true
			}
		}
		return false
	}

	// Build script heuristic — prefer explicit "build" script
	buildScript := ""
	if s, ok := pkg.Scripts["build"]; ok && s != "" {
		buildScript = "npm run build"
	}

	// Next.js
	if hasDep("next") {
		out := "out"
		if buildScript == "" {
			buildScript = "npm run build"
		}
		return Result{
			Framework:         FrameworkNextJS,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         out,
			SPA:               false, // Next.js export is not strictly SPA
		}
	}

	// Astro
	if hasDep("astro") {
		return Result{
			Framework:         FrameworkAstro,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         "dist",
			SPA:               false,
		}
	}

	// Gatsby
	if hasDep("gatsby") {
		return Result{
			Framework:         FrameworkGatsby,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         "public",
			SPA:               false,
		}
	}

	// Nuxt
	if hasDep("nuxt", "nuxt3") {
		return Result{
			Framework:         FrameworkNuxt,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         ".output/public",
			SPA:               false,
		}
	}

	// Vite (generic — covers Vue, React+Vite, Svelte+Vite, etc.)
	if hasDep("vite") {
		fw := FrameworkVite
		spa := true
		if hasDep("vue", "@vue/core") {
			fw = FrameworkVueJS
		} else if hasDep("svelte") {
			fw = FrameworkSvelte
		} else if hasDep("react", "react-dom") {
			fw = FrameworkReact
		}
		return Result{
			Framework:         fw,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         "dist",
			SPA:               spa,
		}
	}

	// Angular CLI
	if hasDep("@angular/core") {
		return Result{
			Framework:         FrameworkAngular,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      "npm run build",
			OutputDir:         "dist",
			SPA:               true,
		}
	}

	// React (CRA / other)
	if hasDep("react", "react-dom") {
		out := "build"
		if hasDep("vite") {
			out = "dist"
		}
		return Result{
			Framework:         FrameworkReact,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         out,
			SPA:               true,
		}
	}

	// Svelte (without Vite)
	if hasDep("svelte") {
		return Result{
			Framework:         FrameworkSvelte,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         "public",
			SPA:               true,
		}
	}

	// Vue CLI (without Vite)
	if hasDep("vue", "@vue/cli-service") {
		return Result{
			Framework:         FrameworkVueJS,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         "dist",
			SPA:               true,
		}
	}

	// Express / Fastify / NestJS / Koa → server runtime → VPS
	if hasDep("express", "fastify", "@nestjs/core", "koa", "hapi") {
		return Result{
			Framework:         FrameworkNode,
			SuggestedProtocol: ProtocolManagedVPS,
		}
	}

	// Generic Node app with a build script → optimistically suggest static
	if buildScript != "" {
		return Result{
			Framework:         FrameworkNode,
			SuggestedProtocol: ProtocolStaticHosting,
			BuildCommand:      buildScript,
			OutputDir:         "dist",
			SPA:               false,
		}
	}

	// Node app with no build → server runtime assumption
	return Result{
		Framework:         FrameworkNode,
		SuggestedProtocol: ProtocolManagedVPS,
	}
}

// fileContains is a simple substring search across a byte slice.
// It is used for quick presence checks in config/manifest files.
func fileContains(data []byte, substr string) bool {
	if len(data) == 0 {
		return false
	}
	return contains(string(data), substr)
}

// contains is strings.Contains without importing strings (keeps the package lean).
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
