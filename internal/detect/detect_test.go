package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureDir creates a temporary directory populated with the given files and returns
// the path. The caller must defer os.RemoveAll on the returned path.
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
}

func TestDetect_DefaultsToCurrentDir(t *testing.T) {
	// Detect("") should not panic
	result := Detect("")
	// We can't assert the exact output since it depends on the test runner's cwd,
	// but we assert it returns a valid (non-panicking) result.
	_ = result
}

// --- Static hosting: Node/JS frameworks ---

func TestDetect_NextJS(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "my-app",
			"dependencies": { "next": "14.0.0", "react": "18.0.0", "react-dom": "18.0.0" },
			"scripts": { "build": "next build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkNextJS, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "npm run build", result.BuildCommand)
	assert.Equal(t, "out", result.OutputDir)
	assert.False(t, result.SPA)
}

func TestDetect_Vite_React(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "react-app",
			"dependencies": { "react": "18.0.0", "react-dom": "18.0.0" },
			"devDependencies": { "vite": "5.0.0" },
			"scripts": { "build": "vite build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkReact, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "npm run build", result.BuildCommand)
	assert.Equal(t, "dist", result.OutputDir)
	assert.True(t, result.SPA, "React+Vite apps need SPA routing")
}

func TestDetect_Astro(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "my-astro-site",
			"devDependencies": { "astro": "4.0.0" },
			"scripts": { "build": "astro build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkAstro, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "dist", result.OutputDir)
	assert.False(t, result.SPA)
}

func TestDetect_Gatsby(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "my-gatsby-site",
			"dependencies": { "gatsby": "5.0.0" },
			"scripts": { "build": "gatsby build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkGatsby, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "public", result.OutputDir)
}

func TestDetect_Nuxt(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "my-nuxt-app",
			"devDependencies": { "nuxt": "3.0.0" },
			"scripts": { "build": "nuxt build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkNuxt, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, ".output/public", result.OutputDir)
}

func TestDetect_Vue_Vite(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "vue-app",
			"dependencies": { "vue": "3.0.0" },
			"devDependencies": { "vite": "5.0.0" },
			"scripts": { "build": "vite build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkVueJS, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "dist", result.OutputDir)
}

func TestDetect_Angular(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "ng-app",
			"dependencies": { "@angular/core": "17.0.0" },
			"scripts": { "build": "ng build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkAngular, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "dist", result.OutputDir)
	assert.True(t, result.SPA, "Angular apps are SPAs")
}

func TestDetect_Svelte(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "svelte-app",
			"devDependencies": { "svelte": "4.0.0" },
			"scripts": { "build": "npm run build" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkSvelte, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
}

// --- Static hosting: traditional generators ---

func TestDetect_Hugo(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"hugo.toml": `baseURL = "https://example.com"`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkHugo, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "hugo", result.BuildCommand)
	assert.Equal(t, "public", result.OutputDir)
	assert.False(t, result.SPA)
}

func TestDetect_Jekyll(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"_config.yml": `title: My Blog`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkJekyll, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "bundle exec jekyll build", result.BuildCommand)
	assert.Equal(t, "_site", result.OutputDir)
}

func TestDetect_Eleventy(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		".eleventy.js": `module.exports = function(cfg) {};`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkEleventy, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "_site", result.OutputDir)
}

// --- Managed VPS: server runtimes ---

func TestDetect_Rails(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"Gemfile": `source "https://rubygems.org"\ngem "rails", "~> 7.1"`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkRails, result.Framework)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
	assert.Empty(t, result.BuildCommand, "VPS apps have no static build command")
	assert.Empty(t, result.OutputDir)
}

func TestDetect_Ruby_NonRails(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"Gemfile": `source "https://rubygems.org"\ngem "sinatra"`,
	})
	result := Detect(dir)
	// Non-Rails Ruby still suggests VPS
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
}

func TestDetect_Django(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"requirements.txt": "django==4.2\ngunicorn==21.2",
		"manage.py":        "#!/usr/bin/env python",
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkDjango, result.Framework)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
}

func TestDetect_Python_NonDjango(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"requirements.txt": "flask==3.0\ngunicorn==21.2",
	})
	result := Detect(dir)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
}

func TestDetect_Laravel(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"composer.json": `{"require": {"laravel/framework": "^10.0"}}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkLaravel, result.Framework)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
}

func TestDetect_Go(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"go.mod": "module example.com/myapp\n\ngo 1.21",
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkGo, result.Framework)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
}

func TestDetect_Go_SkipsHugoProject(t *testing.T) {
	// Hugo projects have both go.mod (for Hugo modules) and hugo.toml/config.toml
	// — they should be detected as Hugo (static), not Go (VPS).
	dir := fixtureDir(t, map[string]string{
		"go.mod":   "module example.com/mysite\n\ngo 1.21",
		"hugo.toml": `baseURL = "https://example.com"`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkHugo, result.Framework)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
}

func TestDetect_Express(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "api",
			"dependencies": { "express": "4.18.0" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkNode, result.Framework)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
}

func TestDetect_NestJS(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "nest-api",
			"dependencies": { "@nestjs/core": "10.0.0" }
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
}

// --- Edge cases ---

func TestDetect_NodeNoBuildScript(t *testing.T) {
	// A Node project with no build script and no known deps → VPS assumption
	dir := fixtureDir(t, map[string]string{
		"package.json": `{"name": "my-server", "dependencies": {}}`,
	})
	result := Detect(dir)
	assert.Equal(t, FrameworkNode, result.Framework)
	assert.Equal(t, ProtocolManagedVPS, result.SuggestedProtocol)
}

func TestDetect_NodeGenericWithBuild(t *testing.T) {
	// A Node project with a build script but no recognisable framework
	dir := fixtureDir(t, map[string]string{
		"package.json": `{
			"name": "my-tool",
			"scripts": { "build": "tsc" },
			"dependencies": {}
		}`,
	})
	result := Detect(dir)
	assert.Equal(t, ProtocolStaticHosting, result.SuggestedProtocol)
	assert.Equal(t, "npm run build", result.BuildCommand)
	assert.Equal(t, "dist", result.OutputDir)
}

func TestDetect_MalformedPackageJSON(t *testing.T) {
	dir := fixtureDir(t, map[string]string{
		"package.json": `this is not valid json`,
	})
	// Should not panic; returns empty result
	result := Detect(dir)
	assert.Equal(t, ProtocolNone, result.SuggestedProtocol)
}

// --- Helper: fileContains ---

func TestFileContains(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		substr   string
		expected bool
	}{
		{"empty data", nil, "rails", false},
		{"empty substr", []byte("some content"), "", true},
		{"present", []byte("gem 'rails', '~> 7.1'"), "rails", true},
		{"absent", []byte("gem 'sinatra'"), "rails", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, fileContains(tt.data, tt.substr))
		})
	}
}
