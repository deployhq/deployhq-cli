package commands

import "testing"

func TestIsShellMangledPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		// Mangled by Git Bash / MSYS POSIX path conversion.
		{"drive lowercase forward slash", "c:/Program Files/Git/projects/x", true},
		{"drive uppercase forward slash", "C:/Program Files/Git/projects/x", true},
		{"drive backslash", `C:\Program Files\Git\projects\x`, true},
		{"other drive letter", "d:/msys/projects/x", true},

		// Legitimate API paths.
		{"root-absolute", "/projects/my-app", false},
		{"relative", "projects/my-app", false},
		{"nested relative", "projects/my-app/deployments", false},
		{"root only", "/", false},

		// Edge cases that must not false-positive.
		{"empty", "", false},
		{"single char", "c", false},
		{"two chars", "c:", false},
		{"colon but not drive", "ab:/x", false},
		{"digit prefix", "1:/x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isShellMangledPath(tc.path); got != tc.want {
				t.Errorf("isShellMangledPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
