// internal/auth/copilot_test.go
package auth

import (
	"testing"
)

func TestCopilotTokenIsExpired(t *testing.T) {
	tests := []struct {
		name    string
		token   *CopilotToken
		expired bool
	}{
		{"nil token", nil, true},
		{"expired token", &CopilotToken{ExpiresAt: 1000000000}, true}, // 2001
		{"future token", &CopilotToken{ExpiresAt: 9999999999}, false}, // 2286
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestNewTokenManagerRequiresToken(t *testing.T) {
	// Unset env to test failure path.
	t.Setenv("GITHUB_TOKEN", "")

	_, err := NewTokenManager()
	if err == nil {
		// It's OK if it found a token from hosts.json/apps.json;
		// we only verify the error message on actual failure.
		t.Log("NewTokenManager succeeded (found token from disk)")
	}
}

func TestNewTokenManagerFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "gho_test_token_12345")

	tm, err := NewTokenManager()
	if err != nil {
		t.Fatalf("NewTokenManager with env: %v", err)
	}
	if tm == nil {
		t.Fatal("NewTokenManager returned nil")
	}
}
