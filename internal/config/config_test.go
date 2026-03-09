// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAccounts(t *testing.T) {
	dir := t.TempDir()
	data := `{"accounts": [
		{"handle": "karpathy", "category": "AI Lab Leaders"},
		{"handle": "AnthropicAI", "category": "Anthropic"}
	]}`
	os.WriteFile(filepath.Join(dir, "accounts.json"), []byte(data), 0644)

	accounts, err := LoadAccounts(filepath.Join(dir, "accounts.json"))
	if err != nil {
		t.Fatalf("LoadAccounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
	if accounts[0].Handle != "karpathy" {
		t.Errorf("expected handle 'karpathy', got %q", accounts[0].Handle)
	}
}

func TestLoadTopics(t *testing.T) {
	dir := t.TempDir()
	data := `{"topics": ["claude", "openai", "agentic"]}`
	os.WriteFile(filepath.Join(dir, "topics.json"), []byte(data), 0644)

	topics, err := LoadTopics(filepath.Join(dir, "topics.json"))
	if err != nil {
		t.Fatalf("LoadTopics: %v", err)
	}
	if len(topics) != 3 {
		t.Fatalf("expected 3 topics, got %d", len(topics))
	}
}

func TestLoadSubreddits(t *testing.T) {
	dir := t.TempDir()
	data := `{"subreddits": ["MachineLearning", "LocalLLaMA"]}`
	os.WriteFile(filepath.Join(dir, "subreddits.json"), []byte(data), 0644)

	subs, err := LoadSubreddits(filepath.Join(dir, "subreddits.json"))
	if err != nil {
		t.Fatalf("LoadSubreddits: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subreddits, got %d", len(subs))
	}
}
