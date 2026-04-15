// internal/state/state.go
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// State holds the persistent pipeline state between runs.
type State struct {
	LastRun     time.Time              `json:"last_run"`
	RunCount    int                    `json:"run_count"`
	SeenIDs     map[string]bool        `json:"seen_ids"`
	PostedItems map[string]PostedItem  `json:"posted_items"`
	DailyStats  map[string]*DailyStats `json:"daily_stats"`
}

// PostedItem is a record of a successfully posted tweet.
type PostedItem struct {
	Source   string    `json:"source"`
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	TweetID  string    `json:"tweet_id"`
	Score    float64   `json:"score"`
	PostedAt time.Time `json:"posted_at"`
}

// DailyStats tracks collection/posting counts for a single day.
type DailyStats struct {
	ItemsCollected int            `json:"items_collected"`
	ItemsRanked    int            `json:"items_ranked"`
	ItemsPosted    int            `json:"items_posted"`
	BySource       map[string]int `json:"by_source"`
}

// New creates an empty, initialized State.
func New() *State {
	return &State{
		SeenIDs:     make(map[string]bool),
		PostedItems: make(map[string]PostedItem),
		DailyStats:  make(map[string]*DailyStats),
	}
}

// Load reads state from a JSON file. Returns a new empty state if the file doesn't exist.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}

	// Ensure maps are initialized even if JSON had nulls.
	if s.SeenIDs == nil {
		s.SeenIDs = make(map[string]bool)
	}
	if s.PostedItems == nil {
		s.PostedItems = make(map[string]PostedItem)
	}
	if s.DailyStats == nil {
		s.DailyStats = make(map[string]*DailyStats)
	}

	return &s, nil
}

// Save writes the state to a JSON file.
func (s *State) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// HasSeen returns true if the item ID has been seen in a previous run.
func (s *State) HasSeen(id string) bool {
	return s.SeenIDs[id]
}

// MarkSeen records an item ID as seen.
func (s *State) MarkSeen(id string) {
	s.SeenIDs[id] = true
}

// RecordPost saves a posted item and increments daily stats.
func (s *State) RecordPost(id string, item PostedItem) {
	item.PostedAt = time.Now().UTC()
	s.PostedItems[id] = item

	date := time.Now().UTC().Format("2006-01-02")
	stats := s.getOrCreateDailyStats(date)
	stats.ItemsPosted++
}

// RecordCollection increments collection stats for today.
func (s *State) RecordCollection(source string, count int) {
	date := time.Now().UTC().Format("2006-01-02")
	stats := s.getOrCreateDailyStats(date)
	stats.ItemsCollected += count
	stats.BySource[source] += count
}

// RecordRanking increments the items ranked counter for today.
func (s *State) RecordRanking(count int) {
	date := time.Now().UTC().Format("2006-01-02")
	stats := s.getOrCreateDailyStats(date)
	stats.ItemsRanked += count
}

// StartRun updates run metadata.
func (s *State) StartRun() {
	s.LastRun = time.Now().UTC()
	s.RunCount++
}

func (s *State) getOrCreateDailyStats(date string) *DailyStats {
	if stats, ok := s.DailyStats[date]; ok {
		return stats
	}
	stats := &DailyStats{
		BySource: make(map[string]int),
	}
	s.DailyStats[date] = stats
	return stats
}

// PruneOlderThan removes seen IDs and posted items older than the given duration.
// Keeps state.json from growing unbounded.
func (s *State) PruneOlderThan(d time.Duration) {
	cutoff := time.Now().UTC().Add(-d)

	for id, item := range s.PostedItems {
		if item.PostedAt.Before(cutoff) {
			delete(s.PostedItems, id)
			delete(s.SeenIDs, id)
		}
	}

	// Prune daily stats older than cutoff.
	cutoffDate := cutoff.Format("2006-01-02")
	for date := range s.DailyStats {
		if date < cutoffDate {
			delete(s.DailyStats, date)
		}
	}
}
