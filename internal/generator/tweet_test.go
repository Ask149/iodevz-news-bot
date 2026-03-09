// internal/generator/tweet_test.go
package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
