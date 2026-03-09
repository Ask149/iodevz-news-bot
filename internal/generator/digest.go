// internal/generator/digest.go
package generator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Ask149/iodevz-news-bot/internal/ranker"
)

const digestPrompt = `Create a daily AI news digest in markdown. Group by category:
- Frontier Models & Research
- Agentic AI & Tools
- Open Source & Repos
- Video/Audio/Multimodal
- Industry & Business

For each item: one-line TL;DR + link. Keep it scannable.
Start with "# AI News Digest — {date}" header.
Skip empty categories.`

// DigestGenerator creates daily markdown digests from ranked items.
type DigestGenerator struct {
	llm TextLLMClient
}

// NewDigestGenerator creates a DigestGenerator.
func NewDigestGenerator(client TextLLMClient) *DigestGenerator {
	return &DigestGenerator{llm: client}
}

// Generate creates a markdown digest from ranked items.
func (g *DigestGenerator) Generate(ctx context.Context, items []ranker.RankedItem) (string, error) {
	if len(items) == 0 {
		return fmt.Sprintf("# AI News Digest — %s\n\nNo notable AI news today.\n",
			time.Now().UTC().Format("2006-01-02")), nil
	}

	userMsg := fmt.Sprintf("Date: %s\n\nItems:\n", time.Now().UTC().Format("2006-01-02"))
	for _, item := range items {
		userMsg += fmt.Sprintf("- [%.1f] %s — %s (source: %s)\n",
			item.Score, item.Title, item.URL, item.Source)
	}

	digest, err := g.llm.Chat(ctx, digestPrompt, userMsg)
	if err != nil {
		return "", fmt.Errorf("digest generation: %w", err)
	}

	log.Printf("[digest] generated %d byte digest from %d items", len(digest), len(items))
	return digest, nil
}

// SaveDigest writes a digest to a file, creating directories as needed.
func SaveDigest(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create digest dir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}
