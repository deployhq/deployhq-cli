package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/skills"
)

func TestSafeWriteFile_RefusesSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(victim, []byte("important user content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Attacker pre-plants `target` as a symlink to victim — the simulated
	// "~/.codex/AGENTS.md → ~/.bashrc" pattern.
	target := filepath.Join(dir, "target.md")
	if err := os.Symlink(victim, target); err != nil {
		t.Fatal(err)
	}

	err := safeWriteFile(target, []byte("payload\n"), 0o644)
	if err == nil {
		t.Fatal("safeWriteFile via symlink should have errored")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink: %v", err)
	}

	// Critical assertion: the victim was NOT modified.
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "important user content\n" {
		t.Errorf("victim file was overwritten through symlink: %q", got)
	}
}

func TestSafeWriteFile_WritesToMissingPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "new.md")

	if err := safeWriteFile(target, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("safeWriteFile on missing path = %v, want nil", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello\n" {
		t.Errorf("file content = %q, want %q", got, "hello\n")
	}
}

func TestSafeWriteFile_OverwritesNonSymlinkFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "exists.md")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := safeWriteFile(target, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("safeWriteFile overwriting regular file = %v, want nil", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Errorf("file content = %q, want %q", got, "new\n")
	}
}

func TestEnsureNotSymlinkDir_RefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	victim := t.TempDir()
	link := filepath.Join(dir, "linked-refs")
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}

	err := ensureNotSymlinkDir(link)
	if err == nil {
		t.Fatal("ensureNotSymlinkDir on symlink should have errored")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink: %v", err)
	}
}

func TestEnsureNotSymlinkDir_AllowsMissingPath(t *testing.T) {
	dir := t.TempDir()
	if err := ensureNotSymlinkDir(filepath.Join(dir, "does-not-exist")); err != nil {
		t.Errorf("ensureNotSymlinkDir on missing path = %v, want nil", err)
	}
}

func TestEnsureNotSymlinkDir_AllowsRealDirectory(t *testing.T) {
	if err := ensureNotSymlinkDir(t.TempDir()); err != nil {
		t.Errorf("ensureNotSymlinkDir on real dir = %v, want nil", err)
	}
}

func TestWriteEmbeddedTree_RefusesSymlinkedRoot(t *testing.T) {
	// Regression for the security finding: planted symlink at the refs
	// root must not let writeEmbeddedTree silently redirect every file
	// write into the victim directory.
	dir := t.TempDir()
	victim := t.TempDir()
	refsRoot := filepath.Join(dir, "deployhq-references")
	if err := os.Symlink(victim, refsRoot); err != nil {
		t.Fatal(err)
	}

	// Use the real embedded skill FS to mirror the production call path.
	err := writeEmbeddedTree(skills.FS, "deployhq", refsRoot)
	if err == nil {
		t.Fatal("writeEmbeddedTree into symlinked root should have errored")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink: %v", err)
	}

	// Victim must be untouched.
	entries, err := os.ReadDir(victim)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("victim directory has %d entries after refused write: %v", len(entries), names)
	}
}
