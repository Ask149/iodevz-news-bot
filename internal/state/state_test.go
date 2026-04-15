// internal/state/state_test.go
package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewState(t *testing.T) {
	s := New()
	if s.PostedItems == nil {
		t.Fatal("PostedItems should be initialized")
	}
	if s.DailyStats == nil {
		t.Fatal("DailyStats should be initialized")
	}
	if s.SeenIDs == nil {
		t.Fatal("SeenIDs should be initialized")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	s := New()
	s.RunCount = 5
	s.LastRun = time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	s.SeenIDs["abc123"] = true
	s.PostedItems["abc123"] = PostedItem{
		Source:  "hackernews",
		Title:   "Test Article",
		URL:     "https://example.com",
		TweetID: "tweet123",
		Score:   8.5,
	}

	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.RunCount != 5 {
		t.Errorf("RunCount: got %d, want 5", loaded.RunCount)
	}
	if !loaded.SeenIDs["abc123"] {
		t.Error("SeenIDs missing abc123")
	}
	if loaded.PostedItems["abc123"].TweetID != "tweet123" {
		t.Error("PostedItems missing abc123")
	}
}

func TestLoadNonExistentReturnsNew(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("Load nonexistent should not error: %v", err)
	}
	if s.PostedItems == nil {
		t.Fatal("should return initialized state")
	}
}

func TestHasSeen(t *testing.T) {
	s := New()
	if s.HasSeen("abc") {
		t.Error("should not have seen 'abc'")
	}
	s.MarkSeen("abc")
	if !s.HasSeen("abc") {
		t.Error("should have seen 'abc' after MarkSeen")
	}
}

func TestRecordPost(t *testing.T) {
	s := New()
	s.RecordPost("abc123", PostedItem{
		Source:  "hackernews",
		Title:   "Test",
		URL:     "https://example.com",
		TweetID: "tweet456",
		Score:   7.0,
	})

	if _, ok := s.PostedItems["abc123"]; !ok {
		t.Error("post not recorded")
	}

	date := time.Now().UTC().Format("2006-01-02")
	stats, ok := s.DailyStats[date]
	if !ok {
		t.Fatal("daily stats not recorded")
	}
	if stats.ItemsPosted != 1 {
		t.Errorf("ItemsPosted: got %d, want 1", stats.ItemsPosted)
	}
}

func TestRecordRanking(t *testing.T) {
	s := New()
	s.RecordRanking(5)

	date := time.Now().UTC().Format("2006-01-02")
	stats, ok := s.DailyStats[date]
	if !ok {
		t.Fatal("daily stats not created")
	}
	if stats.ItemsRanked != 5 {
		t.Errorf("ItemsRanked: got %d, want 5", stats.ItemsRanked)
	}
}
