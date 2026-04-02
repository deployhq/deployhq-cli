package sdk

import (
	"context"
	"fmt"
)

// Template represents a deployment template.
type Template struct {
	Name        string `json:"name"`
	Permalink   string `json:"permalink"`
	Description string `json:"description"`
}

// TemplateCreateRequest is the payload for creating a template.
type TemplateCreateRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	NotificationEmail string `json:"notification_email,omitempty"`
	EmailNotify       *bool  `json:"email_notify,omitempty"`
	ProjectID         string `json:"project_id,omitempty"`
}

// TemplateUpdateRequest is the payload for updating a template.
type TemplateUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ListTemplates returns all private templates for the account.
func (c *Client) ListTemplates(ctx context.Context) ([]Template, error) {
	var templates []Template
	if err := c.get(ctx, "/templates", &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

// ListPublicTemplates returns publicly available templates, optionally filtered by framework type.
func (c *Client) ListPublicTemplates(ctx context.Context, frameworkType string) ([]Template, error) {
	path := "/templates/public_templates"
	if frameworkType != "" {
		path += "?framework_type=" + frameworkType
	}
	var templates []Template
	if err := c.get(ctx, path, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

// GetPublicTemplate returns a single public template by permalink.
func (c *Client) GetPublicTemplate(ctx context.Context, templateID, publicID string) (*Template, error) {
	var tmpl Template
	if err := c.get(ctx, fmt.Sprintf("/templates/%s/public/%s", templateID, publicID), &tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// CreateTemplate creates a new template.
func (c *Client) CreateTemplate(ctx context.Context, req TemplateCreateRequest) (*Template, error) {
	body := struct {
		Template TemplateCreateRequest `json:"template"`
	}{Template: req}
	var tmpl Template
	if err := c.post(ctx, "/templates", body, &tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// UpdateTemplate updates a template by permalink.
func (c *Client) UpdateTemplate(ctx context.Context, id string, req TemplateUpdateRequest) (*Template, error) {
	body := struct {
		Template TemplateUpdateRequest `json:"template"`
	}{Template: req}
	var tmpl Template
	if err := c.put(ctx, fmt.Sprintf("/templates/%s", id), body, &tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// DeleteTemplate deletes a template by permalink.
func (c *Client) DeleteTemplate(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/templates/%s", id))
}
