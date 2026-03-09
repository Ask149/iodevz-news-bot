// internal/collector/collector_test.go
package collector

import (
	"testing"
	"time"
)

func TestItemID(t *testing.T) {
	item := Item{
		Source: "hackernews",
		URL:    "https://example.com/article",
		Title:  "Test Article",
	}

	id := item.ID()
	if id == "" {
		t.Fatal("ID() returned empty string")
	}

	// Same inputs should produce the same ID.
	id2 := item.ID()
	if id != id2 {
		t.Errorf("ID() not deterministic: %q != %q", id, id2)
	}

	// Different inputs should produce different IDs.
	item2 := Item{
		Source: "reddit",
		URL:    "https://example.com/article",
		Title:  "Test Article",
	}
	id3 := item2.ID()
	if id == id3 {
		t.Errorf("different sources should produce different IDs")
	}
}

func TestItemIsValid(t *testing.T) {
	valid := Item{
		Source:      "hackernews",
		URL:         "https://example.com",
		Title:       "Test",
		CollectedAt: time.Now(),
	}
	if !valid.IsValid() {
		t.Error("expected valid item")
	}

	invalid := Item{Source: "hackernews"}
	if invalid.IsValid() {
		t.Error("expected invalid item (no URL or title)")
	}
}
