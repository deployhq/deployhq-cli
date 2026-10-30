package commands

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/deployhq/deployhq-cli/internal/output"
)

func TestMergeEnv(t *testing.T) {
	tests := []struct {
		name       string
		env        []string
		overrides  map[string]string
		wantHas    []string
		wantAbsent []string
	}{
		{
			name:      "adds missing keys",
			env:       []string{"PATH=/usr/bin", "HOME=/root"},
			overrides: map[string]string{"DEPLOYHQ_ACCOUNT": "acme", "DEPLOYHQ_EMAIL": "a@b"},
			wantHas:   []string{"PATH=/usr/bin", "HOME=/root", "DEPLOYHQ_ACCOUNT=acme", "DEPLOYHQ_EMAIL=a@b"},
		},
		{
			name:       "preserves user-set keys",
			env:        []string{"DEPLOYHQ_ACCOUNT=other", "PATH=/usr/bin"},
			overrides:  map[string]string{"DEPLOYHQ_ACCOUNT": "acme", "DEPLOYHQ_EMAIL": "a@b"},
			wantHas:    []string{"DEPLOYHQ_ACCOUNT=other", "DEPLOYHQ_EMAIL=a@b", "PATH=/usr/bin"},
			wantAbsent: []string{"DEPLOYHQ_ACCOUNT=acme"},
		},
		{
			name:       "skips empty override values",
			env:        []string{"PATH=/usr/bin"},
			overrides:  map[string]string{"DEPLOYHQ_API_KEY": ""},
			wantHas:    []string{"PATH=/usr/bin"},
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

func TestMCPConfigSnippet_Hosted(t *testing.T) {
	got := MCPConfigSnippet()
	if !strings.Contains(got, `"type": "http"`) || !strings.Contains(got, hostedMCPURL) {
		t.Fatalf("snippet should target hosted server, got:\n%s", got)
	}
	if strings.Contains(got, `"command"`) {
		t.Fatalf("snippet should not use a stdio command, got:\n%s", got)
	}
}

func TestMCPCmd_HasLocalFlag(t *testing.T) {
	if newMCPCmd().Flags().Lookup("local") == nil {
		t.Fatal("dhq mcp should keep a --local flag for the stdio server")
	}
}

func TestDecideMCPMode(t *testing.T) {
	tests := []struct {
		name                       string
		local, piped, explicitJSON bool
		want                       mcpMode
	}{
		{"terminal", false, false, false, mcpModeSetup},
		{"--local wins", true, false, false, mcpModeLocal},
		{"--local wins over pipe", true, true, false, mcpModeLocal},
		{"--local wins over json", true, false, true, mcpModeLocal},
		{"legacy client config", false, true, false, mcpModeLegacyStdio},
		{"explicit --json beats pipe", false, true, true, mcpModeSetup},
		{"explicit --json in terminal", false, false, true, mcpModeSetup},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideMCPMode(tt.local, tt.piped, tt.explicitJSON); got != tt.want {
				t.Fatalf("decideMCPMode(%v,%v,%v) = %v, want %v", tt.local, tt.piped, tt.explicitJSON, got, tt.want)
			}
		})
	}
}

func TestPrintHostedMCPSetup_Text(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr, IsTTY: true}
	if err := printHostedMCPSetup(env); err != nil {
		t.Fatal(err)
	}
	// stdout is data only: the config snippet, no guidance prose.
	if got := strings.TrimSpace(stdout.String()); got != MCPConfigSnippet() {
		t.Fatalf("stdout should be only the config snippet, got:\n%s", got)
	}
	for _, want := range []string{hostedMCPURL, "claude mcp add --transport http", "Connectors", "dhq mcp --local"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestPrintHostedMCPSetup_JSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &output.Envelope{Stdout: &stdout, Stderr: &stderr, JSONMode: true}
	if err := printHostedMCPSetup(env); err != nil {
		t.Fatal(err)
	}
	var got struct {
		URL        string `json:"url"`
		ClaudeCode string `json:"claude_code"`
		Config     struct {
			MCPServers map[string]map[string]string `json:"mcpServers"`
		} `json:"config"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got.URL != hostedMCPURL || got.ClaudeCode != claudeCodeMCPCommand() {
		t.Errorf("unexpected url/claude_code: %+v", got)
	}
	if got.Config.MCPServers["deployhq"]["url"] != hostedMCPURL {
		t.Errorf("config should be a nested object targeting the hosted URL: %+v", got.Config)
	}
	if stderr.Len() != 0 {
		t.Errorf("JSON mode should not write guidance to stderr, got: %s", stderr.String())
	}
}
