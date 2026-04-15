// internal/pipeline/pipeline.go
package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Ask149/iodevz-news-bot/internal/api"
	"github.com/Ask149/iodevz-news-bot/internal/auth"
	"github.com/Ask149/iodevz-news-bot/internal/collector"
	"github.com/Ask149/iodevz-news-bot/internal/config"
	"github.com/Ask149/iodevz-news-bot/internal/generator"
	"github.com/Ask149/iodevz-news-bot/internal/llm"
	"github.com/Ask149/iodevz-news-bot/internal/poster"
	"github.com/Ask149/iodevz-news-bot/internal/ranker"
	"github.com/Ask149/iodevz-news-bot/internal/state"
)

// Config holds pipeline configuration.
type Config struct {
	StateFile      string
	DigestsDir     string
	APIDir         string
	AccountsFile   string
	TopicsFile     string
	SubredditsFile string
	ScriptDir      string
	MinScore       float64
	MaxTweets      int
	MaxDigestItems int
	DryRun         bool
}

// Validate checks that required config fields are set.
func (c *Config) Validate() error {
	if c.StateFile == "" {
		return fmt.Errorf("StateFile is required")
	}
	if c.DigestsDir == "" {
		return fmt.Errorf("DigestsDir is required")
	}
	if c.APIDir == "" {
		return fmt.Errorf("APIDir is required")
	}
	if c.AccountsFile == "" {
		return fmt.Errorf("AccountsFile is required")
	}
	if c.TopicsFile == "" {
		return fmt.Errorf("TopicsFile is required")
	}
	if c.SubredditsFile == "" {
		return fmt.Errorf("SubredditsFile is required")
	}
	return nil
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		StateFile:      "state.json",
		DigestsDir:     "digests",
		APIDir:         "api",
		AccountsFile:   "config/accounts.json",
		TopicsFile:     "config/topics.json",
		SubredditsFile: "config/subreddits.json",
		ScriptDir:      "scripts",
		MinScore:       6.0,
		MaxTweets:      5,
		MaxDigestItems: 15,
		DryRun:         false,
	}
}

// Run executes the full pipeline: collect → dedupe → rank → generate → post → API.
func Run(ctx context.Context, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	log.Println("=== IODevz News Bot Pipeline ===")

	// 1. Load state.
	st, err := state.Load(cfg.StateFile)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	st.StartRun()
	log.Printf("Run #%d (last run: %s)", st.RunCount, st.LastRun.Format(time.RFC3339))

	// 2. Load config files.
	accounts, err := config.LoadAccounts(cfg.AccountsFile)
	if err != nil {
		log.Printf("WARNING: could not load accounts: %v", err)
	}
	topics, err := config.LoadTopics(cfg.TopicsFile)
	if err != nil {
		return fmt.Errorf("load topics: %w", err)
	}
	subreddits, err := config.LoadSubreddits(cfg.SubredditsFile)
	if err != nil {
		return fmt.Errorf("load subreddits: %w", err)
	}

	// 3. Initialize LLM client.
	// Prefer GitHub Models API (works with regular PAT) over Copilot API (needs OAuth token).
	// Use interface types so we can assign Client, NoopClient, etc.
	var jsonLLM ranker.LLMClient
	var textLLM generator.TextLLMClient
	if pat := os.Getenv("GITHUB_PAT"); pat != "" {
		log.Println("Using GitHub Models API for LLM (GITHUB_PAT)")
		c := llm.NewGitHubModelsClient(pat)
		jsonLLM = c
		textLLM = c
	} else {
		log.Println("Trying Copilot API for LLM (GITHUB_TOKEN)")
		tm, err := auth.NewTokenManager()
		if err != nil {
			log.Printf("WARNING: auth setup failed: %v — LLM features will use fallback scoring", err)
			noop := llm.NewNoopClient()
			jsonLLM = noop
			textLLM = noop
		} else {
			c := llm.NewClient(tm)
			jsonLLM = c
			textLLM = c
		}
	}

	// 4. Collect from all sources (each with its own timeout).
	var allItems []collector.Item

	// collectWithTimeout runs a collector with a per-source deadline.
	collectWithTimeout := func(name string, timeout time.Duration, fn func(ctx context.Context) ([]collector.Item, error)) {
		cctx, ccancel := context.WithTimeout(ctx, timeout)
		defer ccancel()
		items, err := fn(cctx)
		if err != nil {
			log.Printf("WARNING: %s collection failed: %v", name, err)
			return
		}
		st.RecordCollection(name, len(items))
		allItems = append(allItems, items...)
	}

	// HackerNews (2 min timeout)
	hn := collector.NewHackerNewsCollector()
	collectWithTimeout(string(collector.SourceHackerNews), 2*time.Minute, func(ctx context.Context) ([]collector.Item, error) {
		return hn.Collect(ctx, topics)
	})

	// Reddit (2 min timeout)
	reddit := collector.NewRedditCollector()
	collectWithTimeout(string(collector.SourceReddit), 2*time.Minute, func(ctx context.Context) ([]collector.Item, error) {
		return reddit.Collect(ctx, subreddits)
	})

	// GitHub (2 min timeout)
	gh := collector.NewGitHubCollector()
	collectWithTimeout(string(collector.SourceGitHub), 2*time.Minute, func(ctx context.Context) ([]collector.Item, error) {
		return gh.Collect(ctx, topics)
	})

	// Twitter (5 min timeout — Playwright scraping is slow)
	var handles []string
	for _, a := range accounts {
		handles = append(handles, a.Handle)
	}
	twitter := collector.NewTwitterCollector(
		filepath.Join(cfg.ScriptDir, "scrape-twitter.js"),
		handles,
	)
	collectWithTimeout(string(collector.SourceTwitter), 5*time.Minute, func(ctx context.Context) ([]collector.Item, error) {
		return twitter.Collect(ctx, nil)
	})

	log.Printf("Collected %d total items", len(allItems))

	// 5. Deduplicate against state.
	var newItems []collector.Item
	for _, item := range allItems {
		if !st.HasSeen(item.ID()) {
			st.MarkSeen(item.ID())
			newItems = append(newItems, item)
		}
	}
	log.Printf("After dedup: %d new items (filtered %d seen)", len(newItems), len(allItems)-len(newItems))

	if len(newItems) == 0 {
		log.Println("No new items — saving state and exiting")
		if err := st.Save(cfg.StateFile); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
		// Still build API files (updates timestamps).
		apiBuilder := api.New(cfg.APIDir)
		return apiBuilder.BuildAll(nil, st)
	}

	// 6. Rank items.
	r := ranker.New(jsonLLM)
	ranked, err := r.Rank(ctx, newItems)
	if err != nil {
		return fmt.Errorf("rank: %w", err)
	}
	st.RecordRanking(len(ranked))

	// 7. Generate tweets for top items.
	tweetItems := ranker.FilterByMinScore(ranked, cfg.MinScore)
	tweetItems = ranker.TopN(tweetItems, cfg.MaxTweets)

	tweetGen := generator.NewTweetGenerator(textLLM)
	tweets, err := tweetGen.Generate(ctx, tweetItems)
	if err != nil {
		log.Printf("WARNING: tweet generation failed: %v", err)
	}

	// 8. Post tweets.
	tp := poster.NewTwitterPoster()
	if tp.IsConfigured() && !cfg.DryRun {
		for _, tweet := range tweets {
			tweetID, err := tp.Post(ctx, tweet.Text)
			if err != nil {
				log.Printf("WARNING: posting failed: %v", err)
				continue
			}
			st.RecordPost(tweet.ItemID, state.PostedItem{
				Source:  string(tweet.Item.Source),
				Title:   tweet.Item.Title,
				URL:     tweet.Item.URL,
				TweetID: tweetID,
				Score:   tweet.Item.Score,
			})
		}
	} else if cfg.DryRun {
		log.Println("[DRY RUN] Would post tweets:")
		for _, tweet := range tweets {
			log.Printf("  - %s", tweet.Text)
		}
	} else {
		log.Println("Twitter not configured — skipping posting")
	}

	// 9. Generate digest.
	digestItems := ranker.TopN(ranked, cfg.MaxDigestItems)
	digestGen := generator.NewDigestGenerator(textLLM)
	digest, err := digestGen.Generate(ctx, digestItems)
	if err != nil {
		log.Printf("WARNING: digest generation failed: %v", err)
	} else {
		date := time.Now().UTC().Format("2006-01-02")
		digestPath := filepath.Join(cfg.DigestsDir, date+".md")
		if err := generator.SaveDigest(digestPath, digest); err != nil {
			log.Printf("WARNING: save digest failed: %v", err)
		}
	}

	// 10. Build API files.
	apiBuilder := api.New(cfg.APIDir)
	if err := apiBuilder.BuildAll(ranked, st); err != nil {
		log.Printf("WARNING: API build failed: %v", err)
	}

	// 11. Prune old state (keep 30 days).
	st.PruneOlderThan(30 * 24 * time.Hour)

	// 12. Save state.
	if err := st.Save(cfg.StateFile); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	log.Println("=== Pipeline complete ===")
	return nil
}
