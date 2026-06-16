package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureDir creates a temporary directory populated with the given files and returns
// the path. t.TempDir handles cleanup.
func fixtureDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return dir
}

func TestDetect_Empty(t *testing.T) {
	dir := t.TempDir()
	result := Detect(dir)
	assert.Equal(t, FrameworkUnknown, result.Framework)
	assert.Equal(t, ProtocolNone, result.SuggestedProtocol)
	assert.Empty(t, result.BuildCommand)
	assert.Empty(t, result.OutputDir)
	assert.False(t, result.SPA)
}

func TestDetect_DefaultsToCurrentDir(t *testing.T) {
	// Detect("") should not panic.
	result := Detect("")
	// We can't assert the exact output since it depends on the test runner's cwd,
	// but we assert it returns a valid (non-panicking) result.
	_ = result
}

// --- Server-runtime manifests → Managed VPS ---

func TestDetect_Gemfile(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"Gemfile": `source "https://rubygems.org"` + "\n" + `gem "rails", "~> 7.1"`,
	})
	result := Detect(dir)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
	// Local detection no longer identifies the framework or build config.
	assert.Equal(t, FrameworkUnknown, result.Framework)
	assert.Empty(t, result.BuildCommand)
	assert.Empty(t, result.OutputDir)
}

func TestDetect_RequirementsTxt(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"requirements.txt": "django==4.2\ngunicorn==21.2",
	})
	assert.Equal(t, ProtocolManagedVPS, Detect(dir).SuggestedProtocol)
}

func TestDetect_Pipfile(t *testing.T) {
	dir := fixtureDir(t, map[string]string{"Pipfile": "[packages]\nflask = \"*\""})
	assert.Equal(t, ProtocolManagedVPS, Detect(dir).SuggestedProtocol)
}

func TestDetect_PyprojectToml(t *testing.T) {
	dir := fixtureDir(t, map[string]string{"pyproject.toml": "[project]\nname = \"app\""})
	assert.Equal(t, ProtocolManagedVPS, Detect(dir).SuggestedProtocol)
}

func TestDetect_ComposerJSON(t *testing.T) {
	// PHP via Composer (Laravel, Symfony, …).
	dir := fixtureDir(t, map[string]string{
		"composer.json": `{"require": {"laravel/framework": "^10.0"}}`,
	})
	assert.Equal(t, ProtocolManagedVPS, Detect(dir).SuggestedProtocol)
}

func TestDetect_IndexPHP(t *testing.T) {
	// Composer-less / legacy PHP (plain PHP, WordPress) has no composer.json but a
	// root index.php entrypoint — still a server runtime.
	dir := fixtureDir(t, map[string]string{
		"index.php": "<?php echo 'hello'; ?>",
	})
	assert.Equal(t, ProtocolManagedVPS, Detect(dir).SuggestedProtocol)
}

func TestDetect_GoMod(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"go.mod": "module example.com/myapp\n\ngo 1.21",
	})
	assert.Equal(t, ProtocolManagedVPS, Detect(dir).SuggestedProtocol)
}

// --- package.json with a static framework/bundler → Static Hosting ---

func TestDetect_NextJS(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "my-app",
			"dependencies": { "next": "14.0.0", "react": "18.0.0", "react-dom": "18.0.0" },
			"scripts": { "build": "next build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	// Build config comes from the backend, not local detection.
	assert.Empty(t, result.BuildCommand)
	assert.Empty(t, result.OutputDir)
	assert.False(t, result.SPA)
}

func TestDetect_Vite_DevDependency(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "react-app",
			"dependencies": { "react": "18.0.0", "react-dom": "18.0.0" },
			"devDependencies": { "vite": "5.0.0" },
			"scripts": { "build": "vite build" }
		}`,
	})
	assert.Equal(t, ProtocolStaticHosting, Detect(dir).SuggestedProtocol)
}

func TestDetect_Angular(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "ng-app",
			"dependencies": { "@angular/core": "17.0.0" },
			"scripts": { "build": "ng build" }
		}`,
	})
	assert.Equal(t, ProtocolStaticHosting, Detect(dir).SuggestedProtocol)
}

func TestDetect_Astro(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{"name": "site", "devDependencies": { "astro": "4.0.0" }}`,
	})
	assert.Equal(t, ProtocolStaticHosting, Detect(dir).SuggestedProtocol)
}

// --- Precedence: server manifest wins over a static package.json ---

func TestDetect_ServerManifestWinsOverStaticPackageJSON(t *testing.T) {
	// A Rails app that ships a package.json with Vite for asset bundling must be
	// classified as managed_vps — the Gemfile is the stronger signal. This is the
	// core reason server manifests are checked first.
	dir := fixtureDir(t, map[string]string{
		"Gemfile": `gem "rails"`,
		"package.json": `{
			"name": "rails-app",
			"devDependencies": { "vite": "5.0.0", "@vitejs/plugin-react": "4.0.0" }
		}`,
	})
	assert.Equal(t, ProtocolManagedVPS, Detect(dir).SuggestedProtocol,
		"a server manifest must outrank a static package.json")
}

// --- No confident signal → no suggestion ---

func TestDetect_NodeServerWithoutStaticDep(t *testing.T) {
	// An Express API (package.json, no static framework, no server manifest file)
	// yields no confident signal — the user is prompted to choose.
	dir := fixtureDir(t, map[string]string{
		"package.json": `{"name": "api", "dependencies": { "express": "4.18.0" }}`,
	})
	assert.Equal(t, ProtocolNone, Detect(dir).SuggestedProtocol)
}

func TestDetect_PackageJSONNoDeps(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{"name": "my-tool", "scripts": { "build": "tsc" }, "dependencies": {}}`,
	})
	assert.Equal(t, ProtocolNone, Detect(dir).SuggestedProtocol)
}

func TestDetect_MalformedPackageJSON(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `this is not valid json`,
	})
	// Must not panic; returns no suggestion.
	assert.Equal(t, ProtocolNone, Detect(dir).SuggestedProtocol)
}

// --- Helper: packageDeclaresStaticBuild ---

func TestPackageDeclaresStaticBuild(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		expected bool
	}{
		{"empty", "", false},
		{"dependency match", `{"dependencies":{"next":"14"}}`, true},
		{"devDependency match", `{"devDependencies":{"vite":"5"}}`, true},
		{"scoped match", `{"dependencies":{"@angular/core":"17"}}`, true},
		{"no static dep", `{"dependencies":{"express":"4"}}`, false},
		{"malformed", `not json`, false},
		{"empty deps", `{"dependencies":{}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, packageDeclaresStaticBuild([]byte(tt.pkg)))
		})
	}
}
