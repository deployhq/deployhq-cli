package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestWriteOwnedFile_WritesToMissingPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "deployhq-skill.md")
	body := []byte(ownedFileVersionPrefix + skills.Version + ownedFileVersionSuffix + "\n\nbody\n")

	if err := writeOwnedFile(p, body, 0o644); err != nil {
		t.Fatalf("writeOwnedFile on missing path = %v, want nil", err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("content = %q, want %q", got, body)
	}
}

func TestWriteOwnedFile_RefusesUnmarkedExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "deployhq-skill.md")
	userContent := "# my own notes, not DeployHQ's\n"
	if err := os.WriteFile(p, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeOwnedFile(p, []byte("payload\n"), 0o644)
	if err == nil {
		t.Fatal("writeOwnedFile over an unmarked file should have errored")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("error should explain the refusal: %v", err)
	}
	// The user's file must be untouched.
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != userContent {
		t.Errorf("user file was modified: %q", got)
	}
}

func TestWriteOwnedFile_OverwritesOwnMarkedFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "deployhq-skill.md")
	// A previously-installed (marked) file is "ours" and may be replaced.
	old := ownedFileVersionPrefix + "0" + ownedFileVersionSuffix + "\n\nold body\n"
	if err := os.WriteFile(p, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	next := []byte(ownedFileVersionPrefix + skills.Version + ownedFileVersionSuffix + "\n\nnew body\n")
	if err := writeOwnedFile(p, next, 0o644); err != nil {
		t.Fatalf("writeOwnedFile over our own marked file = %v, want nil", err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(next) {
		t.Errorf("content = %q, want %q", got, next)
	}
}
