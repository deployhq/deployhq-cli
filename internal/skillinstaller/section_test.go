package skillinstaller

import (
	"strings"
	"testing"
)

func TestParseSectionVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no marker", "# Just user rules\n", ""},
		{"v1", "prefix\n<!-- BEGIN deployhq-skill v1 -->\nbody\n<!-- END deployhq-skill -->\nsuffix\n", "1"},
		{"v42", "<!-- BEGIN deployhq-skill v42 -->\nx\n<!-- END deployhq-skill -->\n", "42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSectionVersion(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMergeSection_AppendsToEmpty(t *testing.T) {
	got := mergeSection("", "<!-- BEGIN deployhq-skill v1 -->\nbody\n<!-- END deployhq-skill -->")
	want := "<!-- BEGIN deployhq-skill v1 -->\nbody\n<!-- END deployhq-skill -->\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMergeSection_PreservesPreAndPost(t *testing.T) {
	pre := "# top\n"
	post := "# bottom\n"
	existing := pre + "\n<!-- BEGIN deployhq-skill v0 -->\nold\n<!-- END deployhq-skill -->\n\n" + post
	got := mergeSection(existing, "<!-- BEGIN deployhq-skill v1 -->\nnew\n<!-- END deployhq-skill -->")

	for _, must := range []string{"# top", "# bottom", "v1 -->", "new"} {
		if !strings.Contains(got, must) {
			t.Errorf("missing %q in result:\n%s", must, got)
		}
	}
	if strings.Contains(got, "old") || strings.Contains(got, "v0 -->") {
		t.Errorf("stale content survived:\n%s", got)
	}
}

func TestMergeSection_Idempotent(t *testing.T) {
	section := "<!-- BEGIN deployhq-skill v1 -->\nbody\n<!-- END deployhq-skill -->"
	first := mergeSection("# user\n", section)
	second := mergeSection(first, section)
	if first != second {
		t.Errorf("merge not idempotent\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

