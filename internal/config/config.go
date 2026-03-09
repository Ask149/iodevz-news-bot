// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Account is a curated Twitter account to monitor.
type Account struct {
	Handle   string `json:"handle"`
	Category string `json:"category"`
}

// LoadAccounts reads the curated account list from a JSON file.
func LoadAccounts(path string) ([]Account, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read accounts: %w", err)
	}
	var result struct {
		Accounts []Account `json:"accounts"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse accounts: %w", err)
	}
	return result.Accounts, nil
}

// LoadTopics reads the search topic list from a JSON file.
func LoadTopics(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read topics: %w", err)
	}
	var result struct {
		Topics []string `json:"topics"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse topics: %w", err)
	}
	return result.Topics, nil
}

// LoadSubreddits reads the subreddit list from a JSON file.
func LoadSubreddits(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read subreddits: %w", err)
	}
	var result struct {
		Subreddits []string `json:"subreddits"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse subreddits: %w", err)
	}
	return result.Subreddits, nil
}
