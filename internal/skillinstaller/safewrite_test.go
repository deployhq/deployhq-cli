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

func TestSafeWriteFile_RefusesSymlinkedParent(t *testing.T) {
	dir := t.TempDir()
	// Attacker pre-plants the predictable *directory* we write into (e.g.
	// `~/.codex` or `.github`) as a symlink to a dir they control. The leaf
	// itself isn't a symlink, so only the parent check catches this.
	attacker := filepath.Join(dir, "attacker")
	if err := os.MkdirAll(attacker, 0o755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(dir, "config")
	if err := os.Symlink(attacker, parent); err != nil {
		t.Fatal(err)
	}

	err := safeWriteFile(filepath.Join(parent, "AGENTS.md"), []byte("payload\n"), 0o644)
	if err == nil {
		t.Fatal("safeWriteFile through symlinked parent should have errored")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink: %v", err)
	}
	// The write must not have landed in the attacker-controlled directory.
	if _, statErr := os.Stat(filepath.Join(attacker, "AGENTS.md")); !os.IsNotExist(statErr) {
		t.Errorf("write leaked through symlinked parent into attacker dir")
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

func TestWriteEmbeddedTree_ReplacesStaleFilesAndLeavesNoTemp(t *testing.T) {
	parent := t.TempDir()
	dst := filepath.Join(parent, "deployhq-references")

	// First install.
	if err := writeEmbeddedTree(skills.FS, "deployhq", dst); err != nil {
		t.Fatalf("first install = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md missing after install: %v", err)
	}

	// Plant a stale file that the new embedded tree does not contain.
	stale := filepath.Join(dst, "stale-leftover.md")
	if err := os.WriteFile(stale, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-install: the atomic swap replaces the whole tree, so the stale file
	// must be gone and SKILL.md must still be present.
	if err := writeEmbeddedTree(skills.FS, "deployhq", dst); err != nil {
		t.Fatalf("re-install = %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale file survived the atomic re-install (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md missing after re-install: %v", err)
	}

	// No staging directory should be left behind in the parent.
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".deployhq-references.tmp-") {
			t.Errorf("leftover staging dir not cleaned up: %s", e.Name())
		}
	}
}
