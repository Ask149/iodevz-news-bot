// internal/collector/reddit_test.go
package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedditCollector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"children": []map[string]interface{}{
					{
						"data": map[string]interface{}{
							"id":           "abc123",
							"title":        "Claude 4 is amazing",
							"url":          "https://anthropic.com/claude-4",
							"author":       "redditor1",
							"subreddit":    "ClaudeAI",
							"ups":          500,
							"num_comments": 120,
							"selftext":     "Just tried Claude 4 and...",
							"permalink":    "/r/ClaudeAI/comments/abc123/",
							"created_utc":  1709900000.0,
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rc := &RedditCollector{
		baseURL: server.URL,
		client:  http.DefaultClient,
	}

	items, err := rc.Collect(context.Background(), []string{"ClaudeAI"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("expected items")
	}

	item := items[0]
	if item.Source != SourceReddit {
		t.Errorf("source: got %q", item.Source)
	}
	if item.Title != "Claude 4 is amazing" {
		t.Errorf("title: got %q", item.Title)
	}
	if item.Engagement.Upvotes != 500 {
		t.Errorf("upvotes: got %d, want 500", item.Engagement.Upvotes)
	}
}
