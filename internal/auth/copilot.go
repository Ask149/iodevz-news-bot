// internal/auth/copilot.go
package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	copilotTokenURL = "https://api.github.com/copilot_internal/v2/token"
	refreshMargin   = 3 * time.Minute
)

// CopilotToken is a short-lived bearer token for the Copilot API.
type CopilotToken struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// IsExpired returns true if the token has expired or is about to expire.
func (t *CopilotToken) IsExpired() bool {
	if t == nil {
		return true
	}
	return time.Now().Unix() >= t.ExpiresAt-int64(refreshMargin.Seconds())
}

// ExpiresIn returns the duration until the token expires.
func (t *CopilotToken) ExpiresIn() time.Duration {
	if t == nil {
		return 0
	}
	return time.Until(time.Unix(t.ExpiresAt, 0))
}

// TokenManager manages the Copilot bearer token lifecycle.
type TokenManager struct {
	githubToken string
	token       atomic.Pointer[CopilotToken]
	mu          sync.Mutex
	client      *http.Client
}

// NewTokenManager creates a TokenManager by finding a GitHub OAuth token.
func NewTokenManager() (*TokenManager, error) {
	ghToken, err := findGitHubToken()
	if err != nil {
		return nil, fmt.Errorf("no GitHub token found: %w", err)
	}

	return &TokenManager{
		githubToken: ghToken,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// GetToken returns a valid Copilot token, refreshing if needed.
func (tm *TokenManager) GetToken() (*CopilotToken, error) {
	if tok := tm.token.Load(); tok != nil && !tok.IsExpired() {
		return tok, nil
	}
	return tm.doRefresh()
}

// ForceRefresh forces a token refresh regardless of expiry.
func (tm *TokenManager) ForceRefresh() (*CopilotToken, error) {
	return tm.doRefresh()
}

func (tm *TokenManager) doRefresh() (*CopilotToken, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring lock.
	if tok := tm.token.Load(); tok != nil && !tok.IsExpired() {
		return tok, nil
	}

	req, err := http.NewRequest("GET", copilotTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Authorization", "token "+tm.githubToken)
	req.Header.Set("User-Agent", "iodevz-news-bot/0.1")
	req.Header.Set("Accept", "application/json")

	resp, err := tm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tok CopilotToken
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	tm.token.Store(&tok)
	return &tok, nil
}

// findGitHubToken looks for a GitHub OAuth token from multiple sources.
// Priority: GITHUB_TOKEN env var > hosts.json > apps.json
func findGitHubToken() (string, error) {
	// 1. Environment variable (used in GitHub Actions).
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// 2. GitHub Copilot hosts.json (from VS Code extension).
	configDir, err := os.UserConfigDir()
	if err == nil {
		hostsPath := filepath.Join(configDir, "github-copilot", "hosts.json")
		if token, err := readTokenFromJSON(hostsPath); err == nil && token != "" {
			return token, nil
		}
	}

	// 3. GitHub Copilot apps.json (alternative).
	if configDir != "" {
		appsPath := filepath.Join(configDir, "github-copilot", "apps.json")
		if token, err := readTokenFromAppsJSON(appsPath); err == nil && token != "" {
			return token, nil
		}
	}

	return "", fmt.Errorf("no GitHub token found (set GITHUB_TOKEN or install GitHub Copilot extension)")
}

// readTokenFromJSON reads a token from hosts.json format:
// {"github.com": {"oauth_token": "gho_..."}}
func readTokenFromJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var hosts map[string]struct {
		OAuthToken string `json:"oauth_token"`
	}
	if err := json.Unmarshal(data, &hosts); err != nil {
		return "", err
	}

	if host, ok := hosts["github.com"]; ok && host.OAuthToken != "" {
		return host.OAuthToken, nil
	}

	return "", fmt.Errorf("no github.com token in %s", path)
}

// readTokenFromAppsJSON reads a token from apps.json format:
// {"github.com:app-id": {"oauth_token": "gho_..."}}
func readTokenFromAppsJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var apps map[string]struct {
		OAuthToken string `json:"oauth_token"`
	}
	if err := json.Unmarshal(data, &apps); err != nil {
		return "", err
	}

	// Find any entry that has an oauth_token.
	for _, app := range apps {
		if app.OAuthToken != "" {
			return app.OAuthToken, nil
		}
	}

	return "", fmt.Errorf("no token in %s", path)
}
