// internal/collector/hackernews_test.go
package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHackerNewsCollector(t *testing.T) {
	// Mock Algolia API.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			t.Error("missing query parameter")
		}

		resp := map[string]interface{}{
			"hits": []map[string]interface{}{
				{
					"objectID":     "12345",
					"title":        "Claude 4 Released",
					"url":          "https://anthropic.com/claude-4",
					"author":       "testuser",
					"points":       150,
					"num_comments": 42,
					"created_at_i": 1709900000,
				},
				{
					"objectID":     "12346",
					"title":        "Another AI Article",
					"url":          "https://example.com/ai",
					"author":       "user2",
					"points":       50,
					"num_comments": 10,
					"created_at_i": 1709800000,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	hn := &HackerNewsCollector{
		baseURL: server.URL,
		client:  http.DefaultClient,
	}

	items, err := hn.Collect(context.Background(), []string{"claude"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("expected items")
	}

	item := items[0]
	if item.Source != SourceHackerNews {
		t.Errorf("source: got %q, want %q", item.Source, SourceHackerNews)
	}
	if item.Title != "Claude 4 Released" {
		t.Errorf("title: got %q", item.Title)
	}
	if item.URL != "https://anthropic.com/claude-4" {
		t.Errorf("url: got %q", item.URL)
	}
	if item.Engagement.Upvotes != 150 {
		t.Errorf("upvotes: got %d, want 150", item.Engagement.Upvotes)
	}
	if item.Engagement.Comments != 42 {
		t.Errorf("comments: got %d, want 42", item.Engagement.Comments)
	}
}

func TestHackerNewsDeduplicatesAcrossTopics(t *testing.T) {
	// Same article returned for both topics.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"hits": []map[string]interface{}{
				{
					"objectID":     "12345",
					"title":        "Claude 4 Released",
					"url":          "https://anthropic.com/claude-4",
					"author":       "testuser",
					"points":       150,
					"num_comments": 42,
					"created_at_i": 1709900000,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	hn := &HackerNewsCollector{
		baseURL: server.URL,
		client:  http.DefaultClient,
	}

	items, err := hn.Collect(context.Background(), []string{"claude", "anthropic"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// Should deduplicate — same URL from two topic searches.
	if len(items) != 1 {
		t.Errorf("expected 1 item after dedup, got %d", len(items))
	}
}
