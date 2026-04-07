package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	jsonMediaType  = "application/json"
	userAgent      = "deployhq-cli"
)

// Client is the DeployHQ API client.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	email      string
	apiKey     string
	userAgent  string
}

// Account returns the account subdomain this client is configured for.
func (c *Client) Account() string {
	host := c.baseURL.Hostname()
	if i := strings.Index(host, "."); i > 0 {
		return host[:i]
	}
	return host
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) { cl.httpClient = c }
}

// WithUserAgent appends to the User-Agent header (e.g. agent name).
func WithUserAgent(ua string) Option {
	return func(cl *Client) { cl.userAgent = ua }
}

// WithBaseURL overrides the default base URL for all API requests.
// Use this to point the client at staging or dev environments.
// The URL must include the scheme (e.g. "https://myco.deployhq.dev").
func WithBaseURL(rawURL string) Option {
	return func(cl *Client) {
		if u, err := url.Parse(rawURL); err == nil {
			cl.baseURL = u
		}
	}
}

// New creates a new DeployHQ API client.
//
// account is the subdomain (e.g. "mycompany" for mycompany.deployhq.com).
// email and apiKey are used for HTTP Basic authentication.
func New(account, email, apiKey string, opts ...Option) (*Client, error) {
	if account == "" {
		return nil, fmt.Errorf("deployhq: account is required")
	}
	if email == "" {
		return nil, fmt.Errorf("deployhq: email is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("deployhq: api key is required")
	}

	base, err := url.Parse(fmt.Sprintf("https://%s.deployhq.com", account))
	if err != nil {
		return nil, fmt.Errorf("deployhq: invalid account name %q: %w", account, err)
	}

	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    base,
		email:      email,
		apiKey:     apiKey,
		userAgent:  userAgent,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// doRaw executes an HTTP request and returns the raw response body.
func (c *Client) doRaw(ctx context.Context, method, path string) ([]byte, error) {
	rel, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("deployhq: invalid path %q: %w", path, err)
	}
	u := c.baseURL.ResolveReference(rel)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("deployhq: create request: %w", err)
	}

	req.SetBasicAuth(c.email, c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deployhq: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return nil, parseAPIError(resp)
	}

	return io.ReadAll(resp.Body)
}

// do executes an HTTP request and decodes the JSON response into v.
// If v is nil, the response body is discarded.
func (c *Client) do(ctx context.Context, method, path string, body, v interface{}) error {
	rel, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("deployhq: invalid path %q: %w", path, err)
	}
	u := c.baseURL.ResolveReference(rel)

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("deployhq: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return fmt.Errorf("deployhq: create request: %w", err)
	}

	req.SetBasicAuth(c.email, c.apiKey)
	req.Header.Set("Accept", jsonMediaType)
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", jsonMediaType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deployhq: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}

	// 204 No Content or caller doesn't need the body
	if resp.StatusCode == http.StatusNoContent || v == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("deployhq: decode response: %w", err)
	}
	return nil
}

func parseAPIError(resp *http.Response) error {
	apiErr := &APIError{StatusCode: resp.StatusCode}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return apiErr
	}

	// Try {"error": "..."} shape first
	var singleErr struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &singleErr) == nil && singleErr.Error != "" {
		apiErr.Message = singleErr.Error
		return apiErr
	}

	// Try {"errors": ["..."]} shape
	var multiErr struct {
		Errors []string `json:"errors"`
	}
	if json.Unmarshal(body, &multiErr) == nil && len(multiErr.Errors) > 0 {
		apiErr.Errors = multiErr.Errors
		return apiErr
	}

	// Try validation errors: {"field": ["message", ...]} shape (422)
	var validationMap map[string][]string
	if json.Unmarshal(body, &validationMap) == nil && len(validationMap) > 0 {
		var msgs []string
		for field, errs := range validationMap {
			for _, msg := range errs {
				msgs = append(msgs, fmt.Sprintf("%s %s", field, msg))
			}
		}
		if len(msgs) > 0 {
			apiErr.Errors = msgs
			return apiErr
		}
	}

	// Fall back to raw body as message
	apiErr.Message = strings.TrimSpace(string(body))
	return apiErr
}

// get performs a GET request.
func (c *Client) get(ctx context.Context, path string, v interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, v)
}

// post performs a POST request.
func (c *Client) post(ctx context.Context, path string, body, v interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, v)
}

// put performs a PUT request.
func (c *Client) put(ctx context.Context, path string, body, v interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, v)
}

// patch performs a PATCH request.
func (c *Client) patch(ctx context.Context, path string, body, v interface{}) error {
	return c.do(ctx, http.MethodPatch, path, body, v)
}

// delete performs a DELETE request.
func (c *Client) delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// Do performs a raw HTTP request. This is the escape hatch for endpoints
// not covered by typed methods (equivalent to `deployhq api`).
func (c *Client) Do(ctx context.Context, method, path string, body, v interface{}) error {
	return c.do(ctx, method, path, body, v)
}
