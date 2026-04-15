// internal/generator/tweet_test.go
package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ask149/iodevz-news-bot/internal/collector"
	"github.com/Ask149/iodevz-news-bot/internal/ranker"
)

func TestGenerateTweets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tweet := "Anthropic just dropped Claude 4 — first model to beat GPT-5 on coding. Open-weight too. anthropic.com/claude-4"
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": tweet}, "finish_reason": "stop"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	gen := &TweetGenerator{llm: newTestTextLLMClient(server.URL)}

	items := []ranker.RankedItem{
		{
			Item:   collector.Item{Source: "hackernews", Title: "Claude 4 Released", URL: "https://anthropic.com/claude-4"},
			Score:  9.0,
			Reason: "Major release",
		},
	}

	tweets, err := gen.Generate(context.Background(), items)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}

	if len(tweets[0].Text) == 0 {
		t.Error("empty tweet text")
	}
	if len(tweets[0].Text) > 280 {
		t.Errorf("tweet too long: %d chars", len(tweets[0].Text))
	}
}

func TestTruncateTweet(t *testing.T) {
	long := ""
	for i := 0; i < 300; i++ {
		long += "x"
	}
	result := truncateTweet(long)
	if len(result) > 280 {
		t.Errorf("truncated tweet is %d chars, want <= 280", len(result))
	}
}

func TestGenerateTweets_FallbackOnLLMError(t *testing.T) {
	gen := &TweetGenerator{llm: &errorTextLLMClient{}}

	items := []ranker.RankedItem{
		{
			Item:   collector.Item{Source: "hackernews", Title: "Claude 4 Released", URL: "https://anthropic.com/claude-4"},
			Score:  9.0,
			Reason: "Major release",
		},
		{
			Item:   collector.Item{Source: "reddit", Title: "Go 1.24 brings range-over-func", URL: "https://go.dev/blog/go1.24"},
			Score:  8.0,
			Reason: "Language update",
		},
	}

	tweets, err := gen.Generate(context.Background(), items)
	if err != nil {
		t.Fatalf("Generate should not error on LLM failure: %v", err)
	}

	if len(tweets) != len(items) {
		t.Fatalf("expected %d tweets (one per item), got %d", len(items), len(tweets))
	}

	for i, tw := range tweets {
		if len(tw.Text) == 0 {
			t.Errorf("tweet %d: empty text", i)
		}
		if len(tw.Text) > 280 {
			t.Errorf("tweet %d: too long (%d chars), want <= 280", i, len(tw.Text))
		}
		if !strings.Contains(tw.Text, items[i].URL) {
			t.Errorf("tweet %d: should contain URL %q, got %q", i, items[i].URL, tw.Text)
		}
	}
}
