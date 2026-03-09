// internal/ranker/ranker.go
package ranker

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/Ask149/iodevz-news-bot/internal/collector"
)

const rankingPrompt = `You are an AI news curator for @iodevz_ai, a technical AI/developer audience.

Score each item 1-10 based on:
- Novelty (is this genuinely new, not a rehash?)
- Impact (does this change how developers work?)
- Relevance (agentic AI, GenAI, developer tools, frontier research)
- Engagement signal (high engagement = likely interesting)

Respond ONLY as a JSON array (no markdown, no explanation):
[{"id": "item_id", "score": 8.5, "reason": "One sentence why"}]`

// ScoredItem is the LLM's response for a single item.
type ScoredItem struct {
	ID     string  `json:"id"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// RankedItem is a collector.Item with an LLM-assigned score.
type RankedItem struct {
	collector.Item
	Score  float64 `json:"score"`
	Reason string  `json:"score_reason"`
}

// LLMClient is the interface the ranker needs from the LLM package.
type LLMClient interface {
	ChatJSON(ctx context.Context, systemPrompt, userMessage string, v interface{}) error
}

// Ranker scores and sorts collected items using an LLM.
type Ranker struct {
	llm LLMClient
}

// New creates a new Ranker.
func New(client LLMClient) *Ranker {
	return &Ranker{llm: client}
}

// Rank scores all items via the LLM and returns them sorted by score descending.
// Items are batched (max 20 per LLM call) to avoid context length issues.
func (r *Ranker) Rank(ctx context.Context, items []collector.Item) ([]RankedItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	// Build ID -> item map.
	idMap := make(map[string]collector.Item)
	for i := range items {
		idMap[items[i].ID()] = items[i]
	}

	// Batch items for LLM ranking.
	const batchSize = 20
	var allScored []ScoredItem

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]

		// Build user message with item summaries.
		userMsg := "Items to rank:\n"
		for j := range batch {
			userMsg += fmt.Sprintf("- id: %s | source: %s | title: %s | url: %s | engagement: likes=%d upvotes=%d stars=%d comments=%d\n",
				batch[j].ID(), batch[j].Source, batch[j].Title, batch[j].URL,
				batch[j].Engagement.Likes, batch[j].Engagement.Upvotes,
				batch[j].Engagement.Stars, batch[j].Engagement.Comments)
		}

		var scored []ScoredItem
		if err := r.llm.ChatJSON(ctx, rankingPrompt, userMsg, &scored); err != nil {
			log.Printf("[ranker] LLM ranking error for batch %d: %v", i/batchSize, err)
			// On LLM failure, assign default scores based on engagement.
			for j := range batch {
				score := defaultScore(batch[j])
				allScored = append(allScored, ScoredItem{
					ID:     batch[j].ID(),
					Score:  score,
					Reason: "auto-scored from engagement (LLM unavailable)",
				})
			}
			continue
		}

		allScored = append(allScored, scored...)
	}

	// Merge scores back to items.
	scoreMap := make(map[string]ScoredItem)
	for _, s := range allScored {
		scoreMap[s.ID] = s
	}

	var ranked []RankedItem
	for i := range items {
		score := ScoredItem{Score: 5.0, Reason: "unscored"}
		if s, ok := scoreMap[items[i].ID()]; ok {
			score = s
		}
		ranked = append(ranked, RankedItem{
			Item:   items[i],
			Score:  score.Score,
			Reason: score.Reason,
		})
	}

	// Sort by score descending.
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	log.Printf("[ranker] ranked %d items", len(ranked))
	return ranked, nil
}

// FilterByMinScore returns items with score >= minScore.
func FilterByMinScore(items []RankedItem, minScore float64) []RankedItem {
	var filtered []RankedItem
	for _, item := range items {
		if item.Score >= minScore {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// TopN returns the top N items (already sorted by Rank).
func TopN(items []RankedItem, n int) []RankedItem {
	// Sort first in case items aren't sorted.
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	if n > len(items) {
		n = len(items)
	}
	return items[:n]
}

// defaultScore assigns a score based on engagement signals when LLM is unavailable.
func defaultScore(item collector.Item) float64 {
	e := item.Engagement
	total := e.Likes + e.Retweets*2 + e.Upvotes + e.Stars*3 + e.Comments
	switch {
	case total > 1000:
		return 8.0
	case total > 500:
		return 7.0
	case total > 100:
		return 6.0
	case total > 50:
		return 5.0
	default:
		return 4.0
	}
}
