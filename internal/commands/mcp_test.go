package commands

import (
	"sort"
	"strings"
	"testing"
)

func TestMergeEnv(t *testing.T) {
	tests := []struct {
		name      string
		env       []string
		overrides map[string]string
		wantHas   []string
		wantAbsent []string
	}{
		{
			name:      "adds missing keys",
			env:       []string{"PATH=/usr/bin", "HOME=/root"},
			overrides: map[string]string{"DEPLOYHQ_ACCOUNT": "acme", "DEPLOYHQ_EMAIL": "a@b"},
			wantHas:   []string{"PATH=/usr/bin", "HOME=/root", "DEPLOYHQ_ACCOUNT=acme", "DEPLOYHQ_EMAIL=a@b"},
		},
		{
			name:      "preserves user-set keys",
			env:       []string{"DEPLOYHQ_ACCOUNT=other", "PATH=/usr/bin"},
			overrides: map[string]string{"DEPLOYHQ_ACCOUNT": "acme", "DEPLOYHQ_EMAIL": "a@b"},
			wantHas:   []string{"DEPLOYHQ_ACCOUNT=other", "DEPLOYHQ_EMAIL=a@b", "PATH=/usr/bin"},
			wantAbsent: []string{"DEPLOYHQ_ACCOUNT=acme"},
		},
		{
			name:      "skips empty override values",
			env:       []string{"PATH=/usr/bin"},
			overrides: map[string]string{"DEPLOYHQ_API_KEY": ""},
			wantHas:   []string{"PATH=/usr/bin"},
			wantAbsent: []string{"DEPLOYHQ_API_KEY="},
		},
		{
			name:      "empty exported key is treated as missing and dropped",
			env:       []string{"DEPLOYHQ_API_KEY=", "PATH=/usr/bin"},
			overrides: map[string]string{"DEPLOYHQ_API_KEY": "realkey"},
			wantHas:   []string{"DEPLOYHQ_API_KEY=realkey", "PATH=/usr/bin"},
		},
		{
			name:      "empty exported key for unrelated var is preserved",
			env:       []string{"OTHER_THING=", "PATH=/usr/bin"},
			overrides: map[string]string{"DEPLOYHQ_API_KEY": "realkey"},
			wantHas:   []string{"OTHER_THING=", "DEPLOYHQ_API_KEY=realkey", "PATH=/usr/bin"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeEnv(tt.env, tt.overrides)
			sort.Strings(got)
			joined := strings.Join(got, "\n")
			for _, want := range tt.wantHas {
				if !strings.Contains(joined, want) {
					t.Errorf("expected env to contain %q, got %v", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(joined, absent+"\n") || strings.HasSuffix(joined, absent) {
					t.Errorf("expected env NOT to contain %q, got %v", absent, got)
				}
			}
		})
	}
}
