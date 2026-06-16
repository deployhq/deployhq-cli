// Package detect provides a minimal local target heuristic for the dhq launch flow.
//
// This is the OFFLINE FALLBACK. The primary path is the backend's POST /detection
// endpoint (see launchDetect), which runs the same StackDetector pipeline as the
// web onboarding wizard — including rule-based detection and, when the account
// permits, AI-assisted framework + static-hosting assessment — over an uploaded
// manifest. That keeps the CLI's recommendation in lockstep with the server and
// is the single source of truth for framework identity and build configuration
// (output directory, build command, SPA routing).
//
// This local heuristic only runs when that endpoint is unavailable (older
// backend, offline, transient error). It deliberately does NOT try to identify
// the framework or infer build configuration — duplicating the backend's
// per-framework logic here only drifts out of sync. It answers one coarse
// question so the launch flow can pre-seed the target prompt:
//
//   - a server-runtime manifest/entrypoint (Gemfile [Ruby], requirements.txt /
//     Pipfile / pyproject.toml [Python], composer.json / index.php [PHP],
//     go.mod [Go], …) → managed_vps
//   - a package.json declaring a known static-site framework/bundler → static_hosting
//   - no confident signal → "" (let the user choose)
//
// Detection is intentionally heuristic and conservative. False negatives
// (returning "" when a protocol could be inferred) are preferred over false
// positives. CollectManifest (manifest.go) gathers the upload for the primary path.
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

// Framework identifies a detected framework. Local detection no longer populates
// it (the backend's /detection response is the authority); the type is retained
// because detectionResultFromAPI maps the backend stack into it and callers read
// it for display.
type Framework string

// FrameworkUnknown is the zero value — local detection always leaves Framework
// unset, and the backend reports the concrete stack when available.
const FrameworkUnknown Framework = ""

// Result holds the output of a detection pass.
//
// Local Detect() only sets SuggestedProtocol. The other fields are populated by
// detectionResultFromAPI from the backend /detection response (the authority for
// framework identity and build configuration) and are read by the launch flow.
type Result struct {
	// Framework is the detected framework (set from the backend response), or
	// FrameworkUnknown when not identified / from local detection.
	Framework Framework

	// SuggestedProtocol is "static_hosting", "managed_vps", or "" (no suggestion).
	// "" means the caller should ask the user to choose a target manually.
	SuggestedProtocol string

	// BuildCommand is the build command from the backend response, or "" when
	// none is known (local detection never sets it).
	BuildCommand string

	// OutputDir is the build output directory from the backend response, or ""
	// (local detection never sets it).
	OutputDir string

	// SPA is true when the backend reports the site needs single-page-application
	// routing (all paths rewritten to index.html). Local detection never sets it.
	SPA bool
}

// Detect reads the directory at dir and returns a coarse target Result.
// dir should be the root of the project (the directory containing package.json,
// Gemfile, etc.). If dir is empty it defaults to ".".
//
// Detection never fails: when no signal is recognised, it returns a zero-value
// Result with SuggestedProtocol = "". Only SuggestedProtocol is populated.
func Detect(dir string) Result {
	if dir == "" {
		dir = "."
	}

	has := func(names ...string) bool {
		for _, name := range names {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
		return false
	}

	// 1. Server-runtime manifests/entrypoints → Managed VPS. Checked first:
	//    full-stack frameworks (Rails, Django, Laravel) commonly ship a
	//    package.json for asset bundling, so a server manifest is the stronger
	//    signal. PHP is covered both by composer.json (Composer projects: Laravel,
	//    Symfony, …) and a root index.php entrypoint (Composer-less / legacy PHP,
	//    WordPress).
	if has("Gemfile", "requirements.txt", "Pipfile", "pyproject.toml", "composer.json", "go.mod", "index.php") {
		return Result{SuggestedProtocol: ProtocolManagedVPS}
	}

	// 2. A package.json declaring a known static-site framework or bundler →
	//    Static Hosting.
	if has("package.json") {
		if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil && packageDeclaresStaticBuild(data) {
			return Result{SuggestedProtocol: ProtocolStaticHosting}
		}
	}

	// 3. No confident signal — let the user choose.
	return Result{}
}

// staticBuildDeps are package.json dependencies that indicate a static-site
// build (the output is a directory of static assets suitable for CDN hosting).
// The list is intentionally coarse — it only seeds the offline target suggestion,
// which the user can override. The backend's /detection endpoint is the
// authoritative source for framework identity and build configuration.
var staticBuildDeps = map[string]struct{}{
	"next":             {},
	"nuxt":             {},
	"nuxt3":            {},
	"react":            {},
	"react-dom":        {},
	"vue":              {},
	"@vue/cli-service": {},
	"@angular/core":    {},
	"svelte":           {},
	"@sveltejs/kit":    {},
	"astro":            {},
	"gatsby":           {},
	"vite":             {},
	"preact":           {},
	"solid-js":         {},
	"gridsome":         {},
	"@11ty/eleventy":   {},
	"vitepress":        {},
	"vuepress":         {},
	"@docusaurus/core": {},
}

// packageDeclaresStaticBuild reports whether a package.json's dependencies or
// devDependencies include a known static-site framework or bundler.
func packageDeclaresStaticBuild(pkgJSON []byte) bool {
	if len(pkgJSON) == 0 {
		return false
	}
	var pkg struct {
		Dependencies    map[string]json.RawMessage `json:"dependencies"`
		DevDependencies map[string]json.RawMessage `json:"devDependencies"`
	}
	if err := json.Unmarshal(pkgJSON, &pkg); err != nil {
		return false
	}
	for name := range pkg.Dependencies {
		if _, ok := staticBuildDeps[name]; ok {
			return true
		}
	}
	for name := range pkg.DevDependencies {
		if _, ok := staticBuildDeps[name]; ok {
			return true
		}
	}
	return false
}
