// internal/collector/github_test.go
package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubCollector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"full_name":        "anthropics/claude-code",
					"html_url":         "https://github.com/anthropics/claude-code",
					"description":      "Claude Code CLI agent",
					"stargazers_count": 5000,
					"language":         "TypeScript",
					"owner": map[string]string{
						"login": "anthropics",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	gc := &GitHubCollector{
		apiURL: server.URL,
		client: http.DefaultClient,
	}

	items, err := gc.Collect(context.Background(), []string{"claude"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("expected items")
	}

	item := items[0]
	if item.Source != SourceGitHub {
		t.Errorf("source: got %q", item.Source)
	}
	if item.Title != "anthropics/claude-code" {
		t.Errorf("title: got %q", item.Title)
	}
	if item.Engagement.Stars != 5000 {
		t.Errorf("stars: got %d, want 5000", item.Engagement.Stars)
	}
}
