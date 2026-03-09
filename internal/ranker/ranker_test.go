// internal/ranker/ranker_test.go
package ranker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ask149/iodevz-news-bot/internal/collector"
	"github.com/Ask149/iodevz-news-bot/internal/llm"
)

func TestRankItems(t *testing.T) {
	// Mock LLM server that returns ranking scores.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rankings := []ScoredItem{
			{ID: "item1", Score: 9.0, Reason: "Major model release"},
			{ID: "item2", Score: 4.0, Reason: "Minor update"},
		}
		rankJSON, _ := json.Marshal(rankings)
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": string(rankJSON)}, "finish_reason": "stop"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &llm.Client{}
	// Use reflection or exported test helper — for now, create test client directly.
	ranker := &Ranker{llm: newTestLLMClient(server.URL)}

	items := []collector.Item{
		{Source: "hackernews", Title: "Claude 5 Released", URL: "https://example.com/1"},
		{Source: "reddit", Title: "Minor bug fix", URL: "https://example.com/2"},
	}

	scored, err := ranker.Rank(context.Background(), items)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}

	_ = client // silence unused
	if len(scored) != 2 {
		t.Fatalf("expected 2 scored items, got %d", len(scored))
	}
}

func TestFilterByMinScore(t *testing.T) {
	items := []RankedItem{
		{Item: collector.Item{Title: "High"}, Score: 8.5, Reason: "important"},
		{Item: collector.Item{Title: "Low"}, Score: 3.0, Reason: "meh"},
		{Item: collector.Item{Title: "Medium"}, Score: 6.0, Reason: "okay"},
	}

	filtered := FilterByMinScore(items, 6.0)
	if len(filtered) != 2 {
		t.Errorf("expected 2 items with score >= 6.0, got %d", len(filtered))
	}
}

func TestTopN(t *testing.T) {
	items := []RankedItem{
		{Item: collector.Item{Title: "A"}, Score: 5.0},
		{Item: collector.Item{Title: "B"}, Score: 9.0},
		{Item: collector.Item{Title: "C"}, Score: 7.0},
	}

	top := TopN(items, 2)
	if len(top) != 2 {
		t.Fatalf("expected 2 items, got %d", len(top))
	}
	// Should be sorted descending.
	if top[0].Score < top[1].Score {
		t.Errorf("not sorted: %v > %v", top[0].Score, top[1].Score)
	}
}
