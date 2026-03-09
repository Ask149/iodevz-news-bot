// internal/generator/digest_test.go
package generator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ask149/iodevz-news-bot/internal/collector"
	"github.com/Ask149/iodevz-news-bot/internal/ranker"
)

func TestGenerateDigest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		digest := "# AI News Digest\n\n## Frontier Models\n- Claude 4 released\n"
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": digest}, "finish_reason": "stop"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	gen := &DigestGenerator{llm: newTestTextLLMClient(server.URL)}

	items := []ranker.RankedItem{
		{Item: collector.Item{Title: "Claude 4", URL: "https://example.com"}, Score: 9.0},
	}

	digest, err := gen.Generate(context.Background(), items)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(digest) == 0 {
		t.Error("empty digest")
	}
	if digest[:2] != "# " {
		t.Errorf("digest should start with markdown heading")
	}
}

func TestSaveDigest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026-03-08.md")

	err := SaveDigest(path, "# Test Digest\n\nContent here")
	if err != nil {
		t.Fatalf("SaveDigest: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "# Test Digest\n\nContent here" {
		t.Errorf("unexpected content: %q", string(data))
	}
}
