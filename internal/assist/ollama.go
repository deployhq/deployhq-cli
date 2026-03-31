package assist

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultOllamaHost = "http://localhost:11434"
	defaultModel      = "qwen2.5:3b"
)

// Message represents a chat message for the Ollama API.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the payload for POST /api/chat.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// chatResponse is a single response chunk from Ollama.
type chatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// ollamaTagsResponse is the response from GET /api/tags.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	} `json:"models"`
}

// DefaultModelName returns the default model name for display.
func DefaultModelName() string { return defaultModel }

// OllamaClient communicates with a local Ollama instance.
type OllamaClient struct {
	BaseURL string
	Model   string
	client  *http.Client
}

// NewOllamaClient creates a client with env var overrides.
func NewOllamaClient() *OllamaClient {
	host := defaultOllamaHost
	if h := os.Getenv("OLLAMA_HOST"); h != "" {
		host = h
	}
	model := defaultModel
	if m := os.Getenv("DHQ_ASSIST_MODEL"); m != "" {
		model = m
	}
	return &OllamaClient{
		BaseURL: host,
		Model:   model,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// IsAvailable checks if Ollama is running.
func (c *OllamaClient) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	c.client.Timeout = 3 * time.Second
	resp, err := c.client.Do(req)
	c.client.Timeout = 5 * time.Minute
	if err != nil {
		return false
	}
	resp.Body.Close() //nolint:errcheck
	return resp.StatusCode == http.StatusOK
}

// ListModels returns installed model names.
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to Ollama: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	names := make([]string, len(tags.Models))
	for i, m := range tags.Models {
		names[i] = m.Name
	}
	return names, nil
}

// Chat sends a non-streaming chat request and returns the full response.
func (c *OllamaClient) Chat(ctx context.Context, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(b))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Message.Content, nil
}

// ChatStream sends a streaming chat request and writes tokens to the writer as they arrive.
func (c *OllamaClient) ChatStream(ctx context.Context, messages []Message, w io.Writer) error {
	body, err := json.Marshal(chatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk chatResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			fmt.Fprint(w, chunk.Message.Content) //nolint:errcheck
		}
		if chunk.Done {
			break
		}
	}
	fmt.Fprintln(w) //nolint:errcheck
	return scanner.Err()
}
