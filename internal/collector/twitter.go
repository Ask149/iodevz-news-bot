// internal/collector/twitter.go
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TwitterCollector scrapes tweets via a Playwright script.
type TwitterCollector struct {
	scriptPath string
	handles    []string
}

// NewTwitterCollector creates a Twitter collector.
// scriptPath is the path to the Playwright scraping script (scripts/scrape-twitter.js).
// handles is the list of Twitter handles to monitor.
func NewTwitterCollector(scriptPath string, handles []string) *TwitterCollector {
	return &TwitterCollector{
		scriptPath: scriptPath,
		handles:    handles,
	}
}

func (c *TwitterCollector) Name() Source {
	return SourceTwitter
}

func (c *TwitterCollector) Collect(ctx context.Context, _ []string) ([]Item, error) {
	// Check script exists.
	if _, err := os.Stat(c.scriptPath); os.IsNotExist(err) {
		log.Printf("[twitter] scrape script not found at %s, skipping", c.scriptPath)
		return nil, nil
	}

	// Write handles to a temp file for the script.
	tmpDir := os.TempDir()
	accountsFile := filepath.Join(tmpDir, "newsbot-accounts.txt")
	if err := writeAccountsFile(accountsFile, c.handles); err != nil {
		return nil, fmt.Errorf("write accounts file: %w", err)
	}
	defer os.Remove(accountsFile)

	// Output file for scraped tweets.
	outputFile := filepath.Join(tmpDir, "newsbot-tweets.json")
	defer os.Remove(outputFile)

	// Run the Playwright script.
	cmd := exec.CommandContext(ctx, "node", c.scriptPath,
		"--accounts", accountsFile,
		"--output", outputFile,
	)
	cmd.Stderr = os.Stderr

	log.Printf("[twitter] running scrape script for %d accounts...", len(c.handles))
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("playwright script: %w", err)
	}

	// Read output.
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("read tweet output: %w", err)
	}

	items, err := parseTweetJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse tweet output: %w", err)
	}

	log.Printf("[twitter] scraped %d tweets from %d accounts", len(items), len(c.handles))
	return items, nil
}

type scrapedTweet struct {
	Handle    string `json:"handle"`
	Text      string `json:"text"`
	URL       string `json:"url"`
	Likes     int    `json:"likes"`
	Retweets  int    `json:"retweets"`
	Replies   int    `json:"replies"`
	Timestamp string `json:"timestamp"`
}

func parseTweetJSON(data []byte) ([]Item, error) {
	var tweets []scrapedTweet
	if err := json.Unmarshal(data, &tweets); err != nil {
		return nil, err
	}

	var items []Item
	for _, tw := range tweets {
		// Use tweet text as title (first 100 chars).
		title := tw.Text
		if len(title) > 100 {
			title = title[:100] + "..."
		}

		item := Item{
			Source:       SourceTwitter,
			SourceAuthor: tw.Handle,
			URL:          tw.URL,
			Title:        title,
			Body:         tw.Text,
			CollectedAt:  time.Now().UTC(),
			Engagement: Engagement{
				Likes:    tw.Likes,
				Retweets: tw.Retweets,
				Replies:  tw.Replies,
			},
		}
		if item.IsValid() {
			items = append(items, item)
		}
	}

	return items, nil
}

func writeAccountsFile(path string, handles []string) error {
	content := strings.Join(handles, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}
