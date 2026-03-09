// internal/collector/hackernews.go
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	algoliaBaseURL = "https://hn.algolia.com/api/v1"
)

// HackerNewsCollector fetches AI-related stories from HackerNews via Algolia API.
type HackerNewsCollector struct {
	baseURL string
	client  *http.Client
}

// NewHackerNewsCollector creates a new HN collector.
func NewHackerNewsCollector() *HackerNewsCollector {
	return &HackerNewsCollector{
		baseURL: algoliaBaseURL,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *HackerNewsCollector) Name() Source {
	return SourceHackerNews
}

func (c *HackerNewsCollector) Collect(ctx context.Context, topics []string) ([]Item, error) {
	seen := make(map[string]bool)
	var items []Item

	for _, topic := range topics {
		hits, err := c.searchTopic(ctx, topic)
		if err != nil {
			log.Printf("[hackernews] error searching %q: %v", topic, err)
			continue
		}

		for _, hit := range hits {
			// Deduplicate by URL across topic searches.
			if seen[hit.URL] || hit.URL == "" {
				continue
			}
			seen[hit.URL] = true

			item := Item{
				Source:       SourceHackerNews,
				SourceAuthor: hit.Author,
				URL:          hit.URL,
				Title:        hit.Title,
				CollectedAt:  time.Now().UTC(),
				Engagement: Engagement{
					Upvotes:  hit.Points,
					Comments: hit.NumComments,
				},
			}
			if item.IsValid() {
				items = append(items, item)
			}
		}
	}

	log.Printf("[hackernews] collected %d items from %d topics", len(items), len(topics))
	return items, nil
}

type hnHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	CreatedAtI  int64  `json:"created_at_i"`
}

func (c *HackerNewsCollector) searchTopic(ctx context.Context, topic string) ([]hnHit, error) {
	// Search last 24 hours.
	since := time.Now().Add(-24 * time.Hour).Unix()

	params := url.Values{}
	params.Set("query", topic)
	params.Set("tags", "story")
	params.Set("numericFilters", fmt.Sprintf("created_at_i>%d", since))
	params.Set("hitsPerPage", "20")

	u := fmt.Sprintf("%s/search_by_date?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("algolia request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("algolia HTTP %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Hits []hnHit `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse algolia response: %w", err)
	}

	return result.Hits, nil
}
