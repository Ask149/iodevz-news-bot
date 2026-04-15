# iodevz-news-bot

Autonomous AI news intelligence pipeline for [@iodevz_ai](https://twitter.com/iodevz_ai). Collects AI news from 4 sources, ranks by importance using LLM, generates tweets, posts automatically, and serves a static JSON API.

**$0/month** — runs entirely on GitHub Actions + GitHub Copilot API.

## Architecture

```
┌─────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│   Twitter    │  │  HackerNews  │  │    Reddit    │  │    GitHub    │
│ (Playwright) │  │ (Algolia API)│  │ (JSON API)   │  │ (Search API) │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │                 │
       └─────────────────┴────────┬────────┴─────────────────┘
                                  │
                          ┌───────▼────────┐
                          │   Dedup + State │ ← state.json
                          └───────┬────────┘
                                  │
                          ┌───────▼────────┐
                          │  LLM Ranker    │ ← GitHub Copilot API
                          └───────┬────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    │             │             │
            ┌───────▼──────┐ ┌───▼────┐ ┌──────▼──────┐
            │ Tweet Gen    │ │ Digest │ │  API Build  │
            └───────┬──────┘ └───┬────┘ └──────┬──────┘
                    │            │              │
            ┌───────▼──────┐    │       ┌──────▼──────┐
            │ Twitter Post │    │       │ GitHub Pages│
            │ (API v2)     │    │       │ Static JSON │
            └──────────────┘    │       └─────────────┘
                                │
                        ┌───────▼──────┐
                        │ digests/*.md │
                        └──────────────┘
```

## Features

- **4 Sources**: Twitter (Playwright scraping of ~52 curated AI accounts), HackerNews (Algolia), Reddit (8 subreddits), GitHub (trending + search)
- **LLM Ranking**: Scores items 1-10 on novelty, impact, relevance, engagement using GitHub Copilot API
- **Auto-Tweeting**: Generates and posts 3-5 tweets/day with technical, builder-focused voice
- **Daily Digests**: Markdown digests grouped by category
- **Static JSON API**: Served via GitHub Pages — `latest.json`, `topics/{topic}.json`, `sources/{source}.json`, `daily/{date}.json`, `stats.json`
- **Dedup**: SHA-256 based deduplication across runs via `state.json`
- **Autonomous**: Runs 3x/day (6AM, 12PM, 6PM IST) on GitHub Actions cron

## Setup

### Prerequisites

- Go 1.25+
- Node.js 20+ (for Playwright Twitter scraping)
- GitHub account with Copilot access

### Local Development

```bash
# Clone
git clone https://github.com/Ask149/iodevz-news-bot.git
cd news-bot

# Build
make build

# Run tests
make test

# Dry run (no posting)
DRY_RUN=true GITHUB_PAT=<your-github-pat> ./bot
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GITHUB_PAT` | Yes | GitHub PAT for GitHub Models API (LLM ranking/generation) |
| `TWITTER_API_KEY` | For posting | Twitter OAuth 1.0a consumer key |
| `TWITTER_API_SECRET` | For posting | Twitter OAuth 1.0a consumer secret |
| `TWITTER_ACCESS_TOKEN` | For posting | Twitter OAuth 1.0a access token |
| `TWITTER_ACCESS_SECRET` | For posting | Twitter OAuth 1.0a access secret |
| `DRY_RUN` | No | Set to `true` to skip posting |

### GitHub Actions Secrets

Add these to your repo's Settings → Secrets:

- `GH_PAT` — GitHub personal access token (for GitHub Models API + pushing state)
- `TWITTER_API_KEY`, `TWITTER_API_SECRET`, `TWITTER_ACCESS_TOKEN`, `TWITTER_ACCESS_SECRET`

## Project Structure

```
iodevz-news-bot/
├── cmd/bot/main.go              # Entry point
├── internal/
│   ├── auth/copilot.go          # GitHub Copilot token management (fallback)
│   ├── llm/copilot.go           # LLM client (GitHub Models API + Copilot fallback)
│   ├── config/config.go         # Account/topic/subreddit loaders
│   ├── collector/
│   │   ├── collector.go         # Item type + Collector interface
│   │   ├── hackernews.go        # HackerNews via Algolia API
│   │   ├── reddit.go            # Reddit via public JSON API
│   │   ├── github.go            # GitHub via Search API
│   │   └── twitter.go           # Twitter via Playwright wrapper
│   ├── ranker/ranker.go         # LLM-based scoring + selection
│   ├── generator/
│   │   ├── tweet.go             # Tweet generation
│   │   └── digest.go            # Daily digest generation
│   ├── poster/twitter.go        # Twitter API v2 posting (OAuth 1.0a)
│   ├── api/builder.go           # Static JSON API builder
│   └── pipeline/pipeline.go     # Pipeline orchestration
├── config/
│   ├── accounts.json            # 52 curated Twitter accounts
│   ├── topics.json              # 19 AI search topics
│   └── subreddits.json          # 8 monitored subreddits
├── scripts/
│   ├── scrape-twitter.js        # Playwright Twitter scraper
│   └── package.json             # Node.js dependencies
├── .github/workflows/
│   └── news-bot.yml             # Cron 3x/day + manual trigger
├── state.json                   # Persistent pipeline state
├── digests/                     # Generated daily markdown digests
├── api/                         # Generated static JSON API
├── Makefile                     # Build, test, run commands
└── go.mod                       # Go module
```

## API Endpoints

Once deployed to GitHub Pages:

| Endpoint | Description |
|----------|-------------|
| `index.json` | API metadata and endpoint listing |
| `latest.json` | Latest ranked items |
| `topics/{topic}.json` | Items filtered by topic (e.g., `claude`, `openai`) |
| `sources/{source}.json` | Items filtered by source (e.g., `hackernews`, `reddit`) |
| `daily/{date}.json` | Items for a specific date |
| `stats.json` | Pipeline run statistics |

## Tweet Voice

- Technical, builder-focused, concise
- 0-2 emoji (not spam)
- 0-2 hashtags (only if natural)
- States the key insight, not just "X released Y"
- Explains why it matters to developers

## Cost

| Component | Cost |
|-----------|------|
| GitHub Actions | Free (2,000 min/month) |
| GitHub Copilot API | Free (included with Copilot) |
| HackerNews Algolia API | Free |
| Reddit JSON API | Free |
| GitHub Search API | Free |
| Twitter API v2 (posting) | Free tier (1,500 tweets/month) |
| GitHub Pages (API hosting) | Free |
| **Total** | **$0/month** |

## License

Private — IODevz (@iodevz_ai)
