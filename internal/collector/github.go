// internal/collector/github.go
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	githubAPIURL = "https://api.github.com"
)

// GitHubCollector searches GitHub for trending/new AI repositories.
type GitHubCollector struct {
	apiURL string
	pat    string // personal access token (optional but recommended)
	client *http.Client
}

// NewGitHubCollector creates a new GitHub collector.
func NewGitHubCollector() *GitHubCollector {
	return &GitHubCollector{
		apiURL: githubAPIURL,
		pat:    os.Getenv("GITHUB_PAT"),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *GitHubCollector) Name() Source {
	return SourceGitHub
}

func (c *GitHubCollector) Collect(ctx context.Context, topics []string) ([]Item, error) {
	seen := make(map[string]bool)
	var items []Item

	for _, topic := range topics {
		repos, err := c.searchTopic(ctx, topic)
		if err != nil {
			log.Printf("[github] error searching %q: %v", topic, err)
			continue
		}

		for _, repo := range repos {
			if seen[repo.HTMLURL] {
				continue
			}
			seen[repo.HTMLURL] = true

			item := Item{
				Source:       SourceGitHub,
				SourceAuthor: repo.Owner.Login,
				URL:          repo.HTMLURL,
				Title:        repo.FullName,
				Body:         truncateString(repo.Description, 500),
				CollectedAt:  time.Now().UTC(),
				Engagement: Engagement{
					Stars: repo.Stars,
				},
			}
			if item.IsValid() {
				items = append(items, item)
			}
		}

		// Rate limit: avoid GitHub API throttling.
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("[github] collected %d repos from %d topics", len(items), len(topics))
	return items, nil
}

type ghRepo struct {
	FullName    string `json:"full_name"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Stars       int    `json:"stargazers_count"`
	Language    string `json:"language"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (c *GitHubCollector) searchTopic(ctx context.Context, topic string) ([]ghRepo, error) {
	// Search repos created in the last 7 days, sorted by stars.
	since := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02")
	query := fmt.Sprintf("%s created:>%s", topic, since)

	u := fmt.Sprintf("%s/search/repositories?q=%s&sort=stars&order=desc&per_page=10",
		c.apiURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "iodevz-news-bot/0.1")
	if c.pat != "" {
		req.Header.Set("Authorization", "Bearer "+c.pat)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("github rate limited (403)")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github HTTP %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Items []ghRepo `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse github response: %w", err)
	}

	return result.Items, nil
}
