// internal/pipeline/pipeline.go
package pipeline

import (
	"context"
	"fmt"
	"log"
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

	// 3. Initialize auth + LLM.
	tm, err := auth.NewTokenManager()
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	llmClient := llm.NewClient(tm)

	// 4. Collect from all sources.
	var allItems []collector.Item

	// HackerNews
	hn := collector.NewHackerNewsCollector()
	if items, err := hn.Collect(ctx, topics); err != nil {
		log.Printf("WARNING: HN collection failed: %v", err)
	} else {
		st.RecordCollection(string(collector.SourceHackerNews), len(items))
		allItems = append(allItems, items...)
	}

	// Reddit
	reddit := collector.NewRedditCollector()
	if items, err := reddit.Collect(ctx, subreddits); err != nil {
		log.Printf("WARNING: Reddit collection failed: %v", err)
	} else {
		st.RecordCollection(string(collector.SourceReddit), len(items))
		allItems = append(allItems, items...)
	}

	// GitHub
	gh := collector.NewGitHubCollector()
	if items, err := gh.Collect(ctx, topics); err != nil {
		log.Printf("WARNING: GitHub collection failed: %v", err)
	} else {
		st.RecordCollection(string(collector.SourceGitHub), len(items))
		allItems = append(allItems, items...)
	}

	// Twitter (if script exists)
	var handles []string
	for _, a := range accounts {
		handles = append(handles, a.Handle)
	}
	twitter := collector.NewTwitterCollector(
		filepath.Join(cfg.ScriptDir, "scrape-twitter.js"),
		handles,
	)
	if items, err := twitter.Collect(ctx, nil); err != nil {
		log.Printf("WARNING: Twitter collection failed: %v", err)
	} else {
		st.RecordCollection(string(collector.SourceTwitter), len(items))
		allItems = append(allItems, items...)
	}

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
	r := ranker.New(llmClient)
	ranked, err := r.Rank(ctx, newItems)
	if err != nil {
		return fmt.Errorf("rank: %w", err)
	}

	// 7. Generate tweets for top items.
	tweetItems := ranker.FilterByMinScore(ranked, cfg.MinScore)
	tweetItems = ranker.TopN(tweetItems, cfg.MaxTweets)

	tweetGen := generator.NewTweetGenerator(llmClient)
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
	digestGen := generator.NewDigestGenerator(llmClient)
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
