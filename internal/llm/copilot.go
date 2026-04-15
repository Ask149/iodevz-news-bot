// internal/llm/copilot.go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Ask149/iodevz-news-bot/internal/auth"
)

const (
	copilotCompletionsURL      = "https://api.githubcopilot.com/chat/completions"
	githubModelsCompletionsURL = "https://models.inference.ai.azure.com/chat/completions"
	copilotDefaultModel        = "gpt-4o"      // Free via Copilot
	githubModelsDefaultModel   = "gpt-4o-mini" // Free via GitHub Models
)

// Required headers that identify us as a Copilot integration.
var copilotHeaders = map[string]string{
	"Editor-Version":         "vscode/1.105.1",
	"Editor-Plugin-Version":  "copilot-chat/0.32.4",
	"Copilot-Integration-Id": "vscode-chat",
	"Openai-Intent":          "conversation-panel",
	"Content-Type":           "application/json",
}

// Client is a simplified LLM client for the news bot.
// Supports two backends: Copilot API (via TokenManager) and GitHub Models API (via PAT).
type Client struct {
	tokenManager      *auth.TokenManager
	token             string // static token for direct auth (GitHub Models or testing)
	completionsURL    string
	model             string
	httpClient        *http.Client
	useCopilotHeaders bool // only for Copilot API
}

// NewClient creates a Client backed by a TokenManager (Copilot API).
func NewClient(tm *auth.TokenManager) *Client {
	return &Client{
		tokenManager:      tm,
		completionsURL:    copilotCompletionsURL,
		model:             copilotDefaultModel,
		useCopilotHeaders: true,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// NewGitHubModelsClient creates a Client that uses the GitHub Models API directly.
// Accepts a regular GitHub PAT — no token exchange needed.
func NewGitHubModelsClient(pat string) *Client {
	return &Client{
		token:             pat,
		completionsURL:    githubModelsCompletionsURL,
		model:             githubModelsDefaultModel,
		useCopilotHeaders: false,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SetModel overrides the default model.
func (c *Client) SetModel(model string) {
	c.model = model
}

// Chat sends a system+user message pair and returns the text response.
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	payload := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"stream": false,
	}

	body, _ := json.Marshal(payload)

	token, err := c.getToken()
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}

	resp, err := c.doRequest(ctx, token, body)
	if err != nil {
		return "", err
	}

	// Retry once on 401 (token expired mid-request).
	if resp.StatusCode == 401 {
		resp.Body.Close()
		token, err = c.forceRefreshToken()
		if err != nil {
			return "", fmt.Errorf("token refresh after 401: %w", err)
		}
		resp, err = c.doRequest(ctx, token, body)
		if err != nil {
			return "", err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("copilot API error: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// ChatJSON sends a system+user message pair and unmarshals the JSON response into v.
func (c *Client) ChatJSON(ctx context.Context, systemPrompt, userMessage string, v interface{}) error {
	text, err := c.Chat(ctx, systemPrompt, userMessage)
	if err != nil {
		return err
	}

	// Strip markdown code fences if present.
	text = stripCodeFences(text)

	if err := json.Unmarshal([]byte(text), v); err != nil {
		return fmt.Errorf("parse LLM JSON: %w (raw: %.500s)", err, text)
	}
	return nil
}

func (c *Client) getToken() (string, error) {
	if c.token != "" {
		return c.token, nil // testing mode
	}
	tok, err := c.tokenManager.GetToken()
	if err != nil {
		return "", err
	}
	return tok.Token, nil
}

func (c *Client) forceRefreshToken() (string, error) {
	if c.token != "" {
		return c.token, nil
	}
	tok, err := c.tokenManager.ForceRefresh()
	if err != nil {
		return "", err
	}
	return tok.Token, nil
}

func (c *Client) doRequest(ctx context.Context, token string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.completionsURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if c.useCopilotHeaders {
		for k, v := range copilotHeaders {
			req.Header.Set(k, v)
		}
	}
	return c.httpClient.Do(req)
}

// stripCodeFences removes ```json ... ``` wrappers from LLM output.
func stripCodeFences(s string) string {
	// Simple approach: trim prefix/suffix.
	if len(s) > 7 && s[:7] == "```json" {
		s = s[7:]
	} else if len(s) > 3 && s[:3] == "```" {
		s = s[3:]
	}
	if len(s) > 3 && s[len(s)-3:] == "```" {
		s = s[:len(s)-3]
	}
	// Trim leading/trailing whitespace.
	for len(s) > 0 && (s[0] == '\n' || s[0] == '\r' || s[0] == ' ') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}

// NoopClient is an LLM client that always returns errors.
// Used when no LLM backend is available — triggers fallback scoring in ranker.
type NoopClient struct{}

func NewNoopClient() *NoopClient { return &NoopClient{} }

func (n *NoopClient) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return "", fmt.Errorf("LLM not available (no GITHUB_PAT or Copilot token configured)")
}

func (n *NoopClient) ChatJSON(ctx context.Context, systemPrompt, userMessage string, v interface{}) error {
	return fmt.Errorf("LLM not available (no GITHUB_PAT or Copilot token configured)")
}
