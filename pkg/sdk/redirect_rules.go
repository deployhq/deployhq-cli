package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// RedirectStatuses are the HTTP status codes a Static Hosting redirect rule can
// answer with.
var RedirectStatuses = []int{301, 302, 307, 308}

// RedirectRuleShadow names the rule that takes over requests a redirect rule
// matches (the first match wins). Either a rule of the site's _redirects file,
// which is checked first and may take some or all of them (Line set,
// RuleIdentifier nil), or an earlier rule managed in DeployHQ that takes all
// of them, so the rule never applies (RuleIdentifier set, Line nil).
type RedirectRuleShadow struct {
	Source         string  `json:"source"`
	Line           *int    `json:"line"`
	RuleIdentifier *string `json:"rule_identifier"`
}

// RedirectRule is a redirect rule managed in DeployHQ (dashboard, API or CLI)
// for a Static Hosting site. Rules are evaluated in Position order, after the
// rules of the site's _redirects file.
type RedirectRule struct {
	Identifier  string `json:"identifier"`
	Position    int    `json:"position"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Status      int    `json:"status"`
	// Query is the query-string matcher, e.g. "p=:id": every listed key must be
	// present, and a ":name" value captures it for the destination. Nil when the
	// rule matches on the path alone.
	Query *string `json:"query"`
	// ShadowedBy is set when a rule of the _redirects file matches some or all
	// of the same requests first, or an earlier DeployHQ rule matches all of
	// them first.
	ShadowedBy *RedirectRuleShadow `json:"shadowed_by"`
}

// RedirectFileRule is a rule read from the _redirects file of the site's
// active deployment. Line is its line number in that file.
type RedirectFileRule struct {
	Source      string  `json:"source"`
	Destination string  `json:"destination"`
	Status      int     `json:"status"`
	Query       *string `json:"query"`
	Line        int     `json:"line"`
}

// RedirectFileWarning is a line of the _redirects file that was skipped, and
// why, or a note about a line that was applied in a normalised form (see
// IsNote). Line is nil for a problem with the file as a whole.
type RedirectFileWarning struct {
	Line   *int   `json:"line"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// redirectFileNoteCodes are the codes of warnings about _redirects lines that
// WERE applied, in a normalised form, rather than skipped.
var redirectFileNoteCodes = map[string]bool{"source_normalised": true, "slash_only_redirect": true}

// IsNote reports whether the warning is a note about a line that was applied,
// not a line that was skipped.
func (w RedirectFileWarning) IsNote() bool {
	return redirectFileNoteCodes[w.Code]
}

// UnmarshalJSON accepts a warning either as an object or as plain text.
func (w *RedirectFileWarning) UnmarshalJSON(data []byte) error {
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '"' {
		var reason string
		if err := json.Unmarshal(trimmed, &reason); err != nil {
			return err
		}
		*w = RedirectFileWarning{Reason: reason}
		return nil
	}
	type plain RedirectFileWarning
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*w = RedirectFileWarning(p)
	return nil
}

// RedirectRules is the redirect configuration of a Static Hosting site: the
// rules managed in DeployHQ, and the rules and warnings from the _redirects
// file of the active deployment (DeploymentUUID; nil before the first deploy).
type RedirectRules struct {
	Rules          []RedirectRule        `json:"rules"`
	FileRules      []RedirectFileRule    `json:"file_rules"`
	FileWarnings   []RedirectFileWarning `json:"file_warnings"`
	DeploymentUUID *string               `json:"deployment_uuid"`
}

// RedirectRuleCreateRequest is a request to add a redirect rule. A zero Status
// leaves the API's default (301); a nil Position appends the rule at the end.
type RedirectRuleCreateRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Status      int    `json:"status,omitempty"`
	Query       string `json:"query,omitempty"`
	Position    *int   `json:"position,omitempty"`
}

// RedirectRuleUpdateRequest changes a redirect rule. Only the non-nil fields
// are sent, so the others are left as they are. An empty Query removes the
// query matcher. Position (1 = checked first) moves the rule; a position past
// the end moves it last.
type RedirectRuleUpdateRequest struct {
	Source      *string `json:"source,omitempty"`
	Destination *string `json:"destination,omitempty"`
	Status      *int    `json:"status,omitempty"`
	Query       *string `json:"query,omitempty"`
	Position    *int    `json:"position,omitempty"`
}

// RedirectRuleScope names the Static Hosting server whose redirect rules a call
// acts on. Build one with ProjectServerRedirects: Static Hosting sites are
// created inside a project; they can't be global servers.
type RedirectRuleScope struct {
	path string
}

// ProjectServerRedirects is the scope of a Static Hosting server in a project.
func ProjectServerRedirects(projectID, serverID string) RedirectRuleScope {
	return RedirectRuleScope{path: fmt.Sprintf("/projects/%s/servers/%s/redirect_rules", url.PathEscape(projectID), url.PathEscape(serverID))}
}

func (s RedirectRuleScope) rulePath(identifier string) string {
	return s.path + "/" + url.PathEscape(identifier)
}

// ListRedirectRules returns the redirect configuration of a Static Hosting
// server. Servers of any other protocol return a 404.
func (c *Client) ListRedirectRules(ctx context.Context, scope RedirectRuleScope) (*RedirectRules, error) {
	var rules RedirectRules
	if err := c.get(ctx, scope.path, &rules); err != nil {
		return nil, err
	}
	return &rules, nil
}

// CreateRedirectRule adds a redirect rule. The site's configuration is
// republished straight away; an invalid rule, or a site being deleted, is a
// 422, and a failed publish is a 503 that changed nothing.
func (c *Client) CreateRedirectRule(ctx context.Context, scope RedirectRuleScope, req RedirectRuleCreateRequest) (*RedirectRule, error) {
	body := map[string]any{"redirect_rule": req}
	var rule RedirectRule
	if err := c.post(ctx, scope.path, body, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// UpdateRedirectRule changes the fields set in req and leaves the others.
func (c *Client) UpdateRedirectRule(ctx context.Context, scope RedirectRuleScope, identifier string, req RedirectRuleUpdateRequest) (*RedirectRule, error) {
	body := map[string]any{"redirect_rule": req}
	var rule RedirectRule
	if err := c.patch(ctx, scope.rulePath(identifier), body, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// DeleteRedirectRule removes a redirect rule.
func (c *Client) DeleteRedirectRule(ctx context.Context, scope RedirectRuleScope, identifier string) error {
	return c.delete(ctx, scope.rulePath(identifier))
}

// ReorderRedirectRules puts the rules in the order of identifiers and returns
// them in their new order. identifiers must name every rule of the site exactly
// once; any other list is a 422 and nothing is moved.
func (c *Client) ReorderRedirectRules(ctx context.Context, scope RedirectRuleScope, identifiers []string) ([]RedirectRule, error) {
	body := map[string]any{"identifiers": identifiers}
	var resp struct {
		Rules []RedirectRule `json:"rules"`
	}
	if err := c.put(ctx, scope.path+"/reorder", body, &resp); err != nil {
		return nil, err
	}
	return resp.Rules, nil
}
