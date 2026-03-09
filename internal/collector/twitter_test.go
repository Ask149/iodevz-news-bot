// internal/collector/twitter_test.go
package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTwitterCollectorParseTweets(t *testing.T) {
	// Test the JSON parsing of scraped tweet data (not the actual scraping).
	raw := `[
		{
			"handle": "karpathy",
			"text": "Just released a new video on transformers",
			"url": "https://twitter.com/karpathy/status/123",
			"likes": 5000,
			"retweets": 800,
			"replies": 200,
			"timestamp": "2026-03-08T10:00:00Z"
		}
	]`

	items, err := parseTweetJSON([]byte(raw))
	if err != nil {
		t.Fatalf("parseTweetJSON: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Source != SourceTwitter {
		t.Errorf("source: got %q", item.Source)
	}
	if item.SourceAuthor != "karpathy" {
		t.Errorf("author: got %q", item.SourceAuthor)
	}
	if item.Engagement.Likes != 5000 {
		t.Errorf("likes: got %d, want 5000", item.Engagement.Likes)
	}
}

func TestTwitterCollectorSkipsOnMissingScript(t *testing.T) {
	tc := &TwitterCollector{
		scriptPath: "/nonexistent/scrape.js",
	}

	items, err := tc.Collect(context.Background(), []string{})
	if err != nil {
		t.Fatalf("should not error, just return empty: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestWriteAccountsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.txt")

	handles := []string{"karpathy", "AnthropicAI", "OpenAI"}
	err := writeAccountsFile(path, handles)
	if err != nil {
		t.Fatalf("writeAccountsFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "karpathy\nAnthropicAI\nOpenAI\n" {
		t.Errorf("unexpected content: %q", string(data))
	}
}
