package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTemplates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]Template{
			{Name: "Rails", Permalink: "rails", Description: "Ruby on Rails template"},
			{Name: "Node", Permalink: "node", Description: "Node.js template"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	templates, err := c.ListTemplates(context.Background())
	require.NoError(t, err)
	assert.Len(t, templates, 2)
	assert.Equal(t, "Rails", templates[0].Name)
	assert.Equal(t, "rails", templates[0].Permalink)
}

func TestGetTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/rails", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Template{
			Name: "Rails", Permalink: "rails", Description: "Ruby on Rails template",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	tmpl, err := c.GetTemplate(context.Background(), "rails")
	require.NoError(t, err)
	assert.Equal(t, "Rails", tmpl.Name)
	assert.Equal(t, "rails", tmpl.Permalink)
}

func TestListPublicTemplates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/public_templates", r.URL.Path)
		_ = json.NewEncoder(w).Encode([]Template{
			{Name: "WordPress", Permalink: "wordpress", Description: "WordPress CMS"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	templates, err := c.ListPublicTemplates(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "WordPress", templates[0].Name)
}

func TestGetPublicTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/my-tmpl/public/pub-123", r.URL.Path)
		_ = json.NewEncoder(w).Encode(Template{
			Name: "Public Rails", Permalink: "public-rails", Description: "Public Rails template",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	tmpl, err := c.GetPublicTemplate(context.Background(), "my-tmpl", "pub-123")
	require.NoError(t, err)
	assert.Equal(t, "Public Rails", tmpl.Name)
	assert.Equal(t, "public-rails", tmpl.Permalink)
}

func TestListPublicTemplates_WithFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/templates/public_templates", r.URL.Path)
		assert.Equal(t, "cms", r.URL.Query().Get("framework_type"))
		_ = json.NewEncoder(w).Encode([]Template{
			{Name: "WordPress", Permalink: "wordpress", Description: "WordPress CMS"},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	templates, err := c.ListPublicTemplates(context.Background(), "cms")
	require.NoError(t, err)
	assert.Len(t, templates, 1)
}

func TestCreateTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/templates", r.URL.Path)

		var body struct {
			Template TemplateCreateRequest `json:"template"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "My Template", body.Template.Name)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Template{Name: "My Template", Permalink: "my-template"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	tmpl, err := c.CreateTemplate(context.Background(), TemplateCreateRequest{Name: "My Template"})
	require.NoError(t, err)
	assert.Equal(t, "My Template", tmpl.Name)
}

func TestUpdateTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/templates/rails", r.URL.Path)

		var body struct {
			Template TemplateUpdateRequest `json:"template"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Updated Rails", body.Template.Name)

		_ = json.NewEncoder(w).Encode(Template{Name: "Updated Rails", Permalink: "rails"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	tmpl, err := c.UpdateTemplate(context.Background(), "rails", TemplateUpdateRequest{Name: "Updated Rails"})
	require.NoError(t, err)
	assert.Equal(t, "Updated Rails", tmpl.Name)
}

func TestDeleteTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/templates/rails", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	err := c.DeleteTemplate(context.Background(), "rails")
	require.NoError(t, err)
}

func TestGetCommitInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/projects/my-app/repository/commit_info", r.URL.Path)
		assert.Equal(t, "abc123", r.URL.Query().Get("ref"))
		_ = json.NewEncoder(w).Encode(Commit{
			Ref: "abc123def456", Author: "Jane", Message: "Fix critical bug",
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	commit, err := c.GetCommitInfo(context.Background(), "my-app", "abc123")
	require.NoError(t, err)
	assert.Equal(t, "abc123def456", commit.Ref)
	assert.Equal(t, "Jane", commit.Author)
	assert.Equal(t, "Fix critical bug", commit.Message)
}
