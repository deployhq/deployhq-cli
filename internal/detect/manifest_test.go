package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectManifest_ListsFilesAndReadsManifests(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"dependencies":{"react":"^18"}}`)
	write(t, dir, "index.html", "<html></html>")
	write(t, dir, "README.md", "# hi")

	filenames, files := CollectManifest(dir)

	// Every root entry is listed.
	assertContains(t, filenames, "package.json")
	assertContains(t, filenames, "index.html")
	assertContains(t, filenames, "README.md")

	// Only manifest files have their contents uploaded.
	if _, ok := files["package.json"]; !ok {
		t.Fatalf("package.json contents must be collected, got keys %v", keys(files))
	}
	if _, ok := files["README.md"]; ok {
		t.Fatalf("README.md is not a manifest and must not be uploaded")
	}
	if !strings.Contains(files["package.json"], "react") {
		t.Fatalf("manifest contents must be the file body, got %q", files["package.json"])
	}
}

func TestCollectManifest_CapsLargeManifest(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package-lock.json", strings.Repeat("a", maxManifestBytes+5000))

	_, files := CollectManifest(dir)
	if got := len(files["package-lock.json"]); got != maxManifestBytes {
		t.Fatalf("manifest must be capped at %d bytes, got %d", maxManifestBytes, got)
	}
}

func TestCollectManifest_MissingDirIsEmpty(t *testing.T) {
	filenames, files := CollectManifest(filepath.Join(t.TempDir(), "does-not-exist"))
	if len(filenames) != 0 || len(files) != 0 {
		t.Fatalf("missing dir must yield empty manifest, got %v / %v", filenames, files)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func assertContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, h := range haystack {
		if h == needle {
			return
		}
	}
	t.Fatalf("expected %v to contain %q", haystack, needle)
}
