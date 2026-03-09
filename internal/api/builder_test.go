// internal/api/builder_test.go
package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ask149/iodevz-news-bot/internal/collector"
	"github.com/Ask149/iodevz-news-bot/internal/ranker"
)

func TestBuildLatest(t *testing.T) {
	dir := t.TempDir()
	builder := New(dir)

	items := []ranker.RankedItem{
		{
			Item:   collector.Item{Source: "hackernews", Title: "Test", URL: "https://example.com", CollectedAt: time.Now()},
			Score:  8.5,
			Reason: "important",
		},
	}

	err := builder.BuildAll(items, nil)
	if err != nil {
		t.Fatalf("BuildAll: %v", err)
	}

	// Check latest.json exists.
	data, err := os.ReadFile(filepath.Join(dir, "latest.json"))
	if err != nil {
		t.Fatalf("read latest.json: %v", err)
	}

	var latest struct {
		UpdatedAt string        `json:"updated_at"`
		Items     []interface{} `json:"items"`
		Count     int           `json:"count"`
	}
	if err := json.Unmarshal(data, &latest); err != nil {
		t.Fatalf("parse latest.json: %v", err)
	}

	if latest.Count != 1 {
		t.Errorf("count: got %d, want 1", latest.Count)
	}
}

func TestBuildCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "api")
	builder := New(apiDir)

	items := []ranker.RankedItem{
		{
			Item:   collector.Item{Source: "hackernews", Title: "Test", URL: "https://example.com", Topics: []string{"claude"}, CollectedAt: time.Now()},
			Score:  7.0,
			Reason: "test",
		},
	}

	err := builder.BuildAll(items, nil)
	if err != nil {
		t.Fatalf("BuildAll: %v", err)
	}

	// Check topics subdirectory was created.
	if _, err := os.Stat(filepath.Join(apiDir, "topics", "claude.json")); os.IsNotExist(err) {
		t.Error("topics/claude.json not created")
	}

	// Check sources subdirectory.
	if _, err := os.Stat(filepath.Join(apiDir, "sources", "hackernews.json")); os.IsNotExist(err) {
		t.Error("sources/hackernews.json not created")
	}
}

func TestBuildIndex(t *testing.T) {
	dir := t.TempDir()
	builder := New(dir)

	err := builder.BuildAll(nil, nil)
	if err != nil {
		t.Fatalf("BuildAll: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("read index.json: %v", err)
	}

	var index map[string]interface{}
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("parse index.json: %v", err)
	}

	if _, ok := index["endpoints"]; !ok {
		t.Error("index.json missing endpoints")
	}
}
