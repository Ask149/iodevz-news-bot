// internal/api/builder.go
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Ask149/iodevz-news-bot/internal/ranker"
	"github.com/Ask149/iodevz-news-bot/internal/state"
)

// APIItem is the public schema for a news item in the API.
type APIItem struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	URL          string         `json:"url"`
	Source       string         `json:"source"`
	SourceAuthor string         `json:"source_author"`
	Topics       []string       `json:"topics"`
	Score        float64        `json:"score"`
	ScoreReason  string         `json:"score_reason"`
	TweetText    string         `json:"tweet_text,omitempty"`
	TweetID      string         `json:"tweet_id,omitempty"`
	Posted       bool           `json:"posted"`
	CollectedAt  string         `json:"collected_at"`
	PostedAt     string         `json:"posted_at,omitempty"`
	Engagement   map[string]int `json:"engagement"`
}

// Builder generates static JSON API files.
type Builder struct {
	outputDir string
}

// New creates an API builder.
func New(outputDir string) *Builder {
	return &Builder{outputDir: outputDir}
}

// BuildAll generates all API JSON files.
func (b *Builder) BuildAll(items []ranker.RankedItem, st *state.State) error {
	// Ensure directories exist.
	dirs := []string{
		b.outputDir,
		filepath.Join(b.outputDir, "topics"),
		filepath.Join(b.outputDir, "sources"),
		filepath.Join(b.outputDir, "daily"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	// Convert to API items.
	apiItems := toAPIItems(items, st)

	// Build each file.
	if err := b.writeJSON("index.json", b.buildIndex()); err != nil {
		return err
	}
	if err := b.writeJSON("latest.json", b.buildLatest(apiItems)); err != nil {
		return err
	}
	if err := b.buildTopicFiles(apiItems); err != nil {
		return err
	}
	if err := b.buildSourceFiles(apiItems); err != nil {
		return err
	}
	if err := b.buildDailyFile(apiItems); err != nil {
		return err
	}
	if err := b.buildStats(st); err != nil {
		return err
	}

	log.Printf("[api] generated API files with %d items", len(apiItems))
	return nil
}

func (b *Builder) buildIndex() map[string]interface{} {
	return map[string]interface{}{
		"name":        "IODevz AI News API",
		"description": "Curated AI news feed from Twitter, HackerNews, Reddit, and GitHub",
		"updated_at":  time.Now().UTC().Format(time.RFC3339),
		"endpoints": map[string]string{
			"latest":  "latest.json",
			"feed":    "feed.json",
			"stats":   "stats.json",
			"topics":  "topics/{topic}.json",
			"sources": "sources/{source}.json",
			"daily":   "daily/{date}.json",
		},
	}
}

func (b *Builder) buildLatest(items []APIItem) map[string]interface{} {
	return map[string]interface{}{
		"updated_at": time.Now().UTC().Format(time.RFC3339),
		"count":      len(items),
		"items":      items,
	}
}

func (b *Builder) buildTopicFiles(items []APIItem) error {
	byTopic := make(map[string][]APIItem)
	for _, item := range items {
		for _, topic := range item.Topics {
			byTopic[topic] = append(byTopic[topic], item)
		}
	}

	for topic, topicItems := range byTopic {
		filename := filepath.Join("topics", topic+".json")
		data := map[string]interface{}{
			"topic":      topic,
			"updated_at": time.Now().UTC().Format(time.RFC3339),
			"count":      len(topicItems),
			"items":      topicItems,
		}
		if err := b.writeJSON(filename, data); err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) buildSourceFiles(items []APIItem) error {
	bySource := make(map[string][]APIItem)
	for _, item := range items {
		bySource[item.Source] = append(bySource[item.Source], item)
	}

	for source, sourceItems := range bySource {
		filename := filepath.Join("sources", source+".json")
		data := map[string]interface{}{
			"source":     source,
			"updated_at": time.Now().UTC().Format(time.RFC3339),
			"count":      len(sourceItems),
			"items":      sourceItems,
		}
		if err := b.writeJSON(filename, data); err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) buildDailyFile(items []APIItem) error {
	date := time.Now().UTC().Format("2006-01-02")
	filename := filepath.Join("daily", date+".json")
	data := map[string]interface{}{
		"date":       date,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
		"count":      len(items),
		"items":      items,
	}
	return b.writeJSON(filename, data)
}

func (b *Builder) buildStats(st *state.State) error {
	stats := map[string]interface{}{
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if st != nil {
		stats["last_run"] = st.LastRun
		stats["run_count"] = st.RunCount
		stats["total_posted"] = len(st.PostedItems)
		stats["daily_stats"] = st.DailyStats
	}
	return b.writeJSON("stats.json", stats)
}

func (b *Builder) writeJSON(filename string, data interface{}) error {
	path := filepath.Join(b.outputDir, filename)
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filename, err)
	}
	return os.WriteFile(path, bytes, 0644)
}

func toAPIItems(items []ranker.RankedItem, st *state.State) []APIItem {
	var apiItems []APIItem
	for i := range items {
		apiItem := APIItem{
			ID:           items[i].ID(),
			Title:        items[i].Title,
			URL:          items[i].URL,
			Source:       string(items[i].Source),
			SourceAuthor: items[i].SourceAuthor,
			Topics:       items[i].Topics,
			Score:        items[i].Score,
			ScoreReason:  items[i].Reason,
			CollectedAt:  items[i].CollectedAt.Format(time.RFC3339),
			Engagement: map[string]int{
				"likes":    items[i].Engagement.Likes,
				"retweets": items[i].Engagement.Retweets,
				"replies":  items[i].Engagement.Replies,
				"upvotes":  items[i].Engagement.Upvotes,
				"comments": items[i].Engagement.Comments,
				"stars":    items[i].Engagement.Stars,
			},
		}

		// Check if this item was posted.
		if st != nil {
			if posted, ok := st.PostedItems[items[i].ID()]; ok {
				apiItem.TweetID = posted.TweetID
				apiItem.Posted = true
				apiItem.PostedAt = posted.PostedAt.Format(time.RFC3339)
			}
		}

		if apiItem.Topics == nil {
			apiItem.Topics = []string{}
		}

		apiItems = append(apiItems, apiItem)
	}
	return apiItems
}
