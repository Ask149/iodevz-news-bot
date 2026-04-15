// internal/collector/reddit.go
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	redditBaseURL = "https://old.reddit.com"
)

// RedditCollector fetches hot posts from monitored subreddits.
type RedditCollector struct {
	baseURL string
	client  *http.Client
}

// NewRedditCollector creates a new Reddit collector.
func NewRedditCollector() *RedditCollector {
	return &RedditCollector{
		baseURL: redditBaseURL,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *RedditCollector) Name() Source {
	return SourceReddit
}

// Collect fetches hot posts from the given subreddits.
// The topics parameter here is actually the subreddit list.
func (c *RedditCollector) Collect(ctx context.Context, subreddits []string) ([]Item, error) {
	var items []Item

	for _, sub := range subreddits {
		posts, err := c.fetchSubreddit(ctx, sub)
		if err != nil {
			log.Printf("[reddit] error fetching r/%s: %v", sub, err)
			continue
		}
		items = append(items, posts...)

		// Rate limit: Reddit is strict about rapid requests.
		time.Sleep(1 * time.Second)
	}

	log.Printf("[reddit] collected %d items from %d subreddits", len(items), len(subreddits))
	return items, nil
}

type redditListing struct {
	Data struct {
		Children []struct {
			Data redditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type redditPost struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Author      string  `json:"author"`
	Subreddit   string  `json:"subreddit"`
	Ups         int     `json:"ups"`
	NumComments int     `json:"num_comments"`
	Selftext    string  `json:"selftext"`
	Permalink   string  `json:"permalink"`
	CreatedUTC  float64 `json:"created_utc"`
}

func (c *RedditCollector) fetchSubreddit(ctx context.Context, subreddit string) ([]Item, error) {
	u := fmt.Sprintf("%s/r/%s/hot.json?limit=25", c.baseURL, subreddit)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	// Use a browser-like User-Agent to avoid 403 blocks from datacenter IPs.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; IODevzBot/0.1; +https://github.com/Ask149/iodevz-news-bot)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("reddit rate limited (429)")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reddit HTTP %d: %s", resp.StatusCode, body)
	}

	var listing redditListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("parse reddit response: %w", err)
	}

	var items []Item
	for _, child := range listing.Data.Children {
		post := child.Data

		// Use permalink as URL for self-posts (url points to self).
		postURL := post.URL
		if post.Selftext != "" && postURL == fmt.Sprintf("https://www.reddit.com%s", post.Permalink) {
			postURL = fmt.Sprintf("https://www.reddit.com%s", post.Permalink)
		}

		item := Item{
			Source:       SourceReddit,
			SourceAuthor: fmt.Sprintf("u/%s (r/%s)", post.Author, post.Subreddit),
			URL:          postURL,
			Title:        post.Title,
			Body:         truncateString(post.Selftext, 500),
			CollectedAt:  time.Now().UTC(),
			Engagement: Engagement{
				Upvotes:  post.Ups,
				Comments: post.NumComments,
			},
		}
		if item.IsValid() {
			items = append(items, item)
		}
	}

	return items, nil
}

// truncateString shortens a string to max length.
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
