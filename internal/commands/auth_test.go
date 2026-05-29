package commands

import "testing"

func TestNormalizeAccount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		host  string
		want  string
	}{
		{"bare subdomain", "mycompany", "", "mycompany"},
		{"full hostname", "mycompany.deployhq.com", "", "mycompany"},
		{"hostname with scheme", "https://mycompany.deployhq.com", "", "mycompany"},
		{"hostname with scheme and trailing slash", "https://mycompany.deployhq.com/", "", "mycompany"},
		{"hostname with path", "https://mycompany.deployhq.com/projects", "", "mycompany"},
		{"http scheme", "http://mycompany.deployhq.com", "", "mycompany"},
		{"surrounding whitespace", "  mycompany.deployhq.com  ", "", "mycompany"},
		{"custom host", "mycompany.deployhq.dev", "deployhq.dev", "mycompany"},
		{"custom host with default host suffix is not stripped", "mycompany.deployhq.com", "deployhq.dev", "mycompany.deployhq.com"},
		{"hyphenated subdomain", "my-company.deployhq.com", "", "my-company"},
		{"query string ignored", "mycompany.deployhq.com?foo=bar", "", "mycompany"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAccount(tt.input, tt.host)
			if got != tt.want {
				t.Errorf("normalizeAccount(%q, %q) = %q, want %q", tt.input, tt.host, got, tt.want)
			}
		})
	}
}
