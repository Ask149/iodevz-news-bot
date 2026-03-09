// internal/collector/collector.go
package collector

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

// Source identifies where an item was collected from.
type Source string

const (
	SourceTwitter    Source = "twitter"
	SourceHackerNews Source = "hackernews"
	SourceReddit     Source = "reddit"
	SourceGitHub     Source = "github"
)

// Item is a single piece of collected news.
type Item struct {
	Source       Source     `json:"source"`
	SourceAuthor string     `json:"source_author"`
	URL          string     `json:"url"`
	Title        string     `json:"title"`
	Body         string     `json:"body,omitempty"`
	CollectedAt  time.Time  `json:"collected_at"`
	Engagement   Engagement `json:"engagement"`
	Topics       []string   `json:"topics,omitempty"`
}

// Engagement holds source-specific engagement metrics.
type Engagement struct {
	Likes    int `json:"likes,omitempty"`
	Retweets int `json:"retweets,omitempty"`
	Replies  int `json:"replies,omitempty"`
	Upvotes  int `json:"upvotes,omitempty"`
	Comments int `json:"comments,omitempty"`
	Stars    int `json:"stars,omitempty"`
}

// ID returns a deterministic SHA-256 hash of source+URL+title.
// Used for deduplication across runs.
func (item *Item) ID() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s", item.Source, item.URL, item.Title)))
	return fmt.Sprintf("%x", h[:16]) // 32 hex chars, enough for dedup
}

// IsValid checks that the item has minimum required fields.
func (item *Item) IsValid() bool {
	return item.Source != "" && item.URL != "" && item.Title != ""
}

// Collector is the interface for all news source collectors.
type Collector interface {
	// Name returns the source name (e.g., "hackernews").
	Name() Source

	// Collect fetches items from the source.
	// topics is the list of search terms to use.
	Collect(ctx context.Context, topics []string) ([]Item, error)
}
