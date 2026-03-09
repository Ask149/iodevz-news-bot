// internal/generator/tweet.go
package generator

import (
	"context"
	"fmt"
	"log"

	"github.com/Ask149/iodevz-news-bot/internal/ranker"
)

const tweetPrompt = `You are writing tweets for @iodevz_ai. Voice: technical, builder-focused,
concise. NO emoji spam (0-2 max). NO hashtag spam (0-2 max, only if natural).

For the given item, write a tweet (max 280 chars) that:
1. States the key insight (not just "X released Y")
2. Explains why it matters to developers/builders
3. Includes the source URL

IMPORTANT: Response must be ONLY the tweet text. No quotes, no explanation. Max 280 characters.`

// Tweet is a generated tweet ready for posting.
type Tweet struct {
	Text   string            `json:"text"`
	ItemID string            `json:"item_id"`
	Item   ranker.RankedItem `json:"item"`
}

// TextLLMClient is the interface for generating text (not JSON).
type TextLLMClient interface {
	Chat(ctx context.Context, systemPrompt, userMessage string) (string, error)
}

// TweetGenerator creates tweets from ranked items using an LLM.
type TweetGenerator struct {
	llm TextLLMClient
}

// NewTweetGenerator creates a TweetGenerator.
func NewTweetGenerator(client TextLLMClient) *TweetGenerator {
	return &TweetGenerator{llm: client}
}

// Generate creates tweet text for each ranked item.
func (g *TweetGenerator) Generate(ctx context.Context, items []ranker.RankedItem) ([]Tweet, error) {
	var tweets []Tweet

	for _, item := range items {
		userMsg := fmt.Sprintf("Source: %s\nTitle: %s\nURL: %s\nScore reason: %s\nBody: %s",
			item.Source, item.Title, item.URL, item.Reason, truncateString(item.Body, 300))

		text, err := g.llm.Chat(ctx, tweetPrompt, userMsg)
		if err != nil {
			log.Printf("[tweet] generation error for %q: %v", item.Title, err)
			continue
		}

		// Clean up: remove quotes if LLM wrapped in them.
		text = cleanTweetText(text)
		text = truncateTweet(text)

		if len(text) > 0 {
			tweets = append(tweets, Tweet{
				Text:   text,
				ItemID: item.ID(),
				Item:   item,
			})
		}
	}

	log.Printf("[tweet] generated %d tweets from %d items", len(tweets), len(items))
	return tweets, nil
}

// truncateTweet ensures tweet is max 280 characters.
func truncateTweet(text string) string {
	if len(text) <= 280 {
		return text
	}
	return text[:277] + "..."
}

// cleanTweetText removes common LLM artifacts.
func cleanTweetText(text string) string {
	// Remove wrapping quotes.
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	// Trim whitespace.
	for len(text) > 0 && (text[0] == '\n' || text[0] == '\r' || text[0] == ' ') {
		text = text[1:]
	}
	for len(text) > 0 && (text[len(text)-1] == '\n' || text[len(text)-1] == '\r' || text[len(text)-1] == ' ') {
		text = text[:len(text)-1]
	}
	return text
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
