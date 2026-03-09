// scripts/scrape-twitter.js
// Twitter scraping via Playwright — reads account list, outputs JSON.
// Usage: node scrape-twitter.js --accounts accounts.txt --output tweets.json

const { chromium } = require('playwright');
const fs = require('fs');

const args = process.argv.slice(2);
const accountsIdx = args.indexOf('--accounts');
const outputIdx = args.indexOf('--output');

if (accountsIdx === -1 || outputIdx === -1) {
  console.error('Usage: node scrape-twitter.js --accounts <file> --output <file>');
  process.exit(1);
}

const accountsFile = args[accountsIdx + 1];
const outputFile = args[outputIdx + 1];

const handles = fs.readFileSync(accountsFile, 'utf-8')
  .split('\n')
  .map(h => h.trim())
  .filter(h => h.length > 0);

// Overall script timeout: 4 minutes.
const SCRIPT_TIMEOUT_MS = 4 * 60 * 1000;
const PAGE_TIMEOUT_MS = 10000; // 10s per page load (down from 15s)
const INTER_ACCOUNT_DELAY_MS = 1000; // 1s between accounts (down from 2s)
const POST_LOAD_WAIT_MS = 1500; // 1.5s wait after load (down from 2s)
const MAX_TWEETS_PER_ACCOUNT = 3; // Reduced from 5 to speed up
const startTime = Date.now();

function isTimedOut() {
  return Date.now() - startTime > SCRIPT_TIMEOUT_MS;
}

async function scrapeAccount(page, handle) {
  const url = `https://x.com/${handle}`;
  try {
    // Use 'domcontentloaded' instead of 'networkidle' to avoid hanging.
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: PAGE_TIMEOUT_MS });
    await page.waitForTimeout(POST_LOAD_WAIT_MS);

    const tweets = await page.evaluate((opts) => {
      const { handle, maxTweets } = opts;
      const articles = document.querySelectorAll('article[data-testid="tweet"]');
      const results = [];

      articles.forEach((article, i) => {
        if (i >= maxTweets) return;

        const textEl = article.querySelector('[data-testid="tweetText"]');
        const text = textEl ? textEl.innerText : '';

        // Extract tweet URL from time element's parent link.
        const timeEl = article.querySelector('time');
        const linkEl = timeEl ? timeEl.closest('a') : null;
        const tweetUrl = linkEl ? 'https://x.com' + linkEl.getAttribute('href') : '';

        // Extract engagement counts.
        const getCount = (testId) => {
          const el = article.querySelector(`[data-testid="${testId}"]`);
          if (!el) return 0;
          const text = el.getAttribute('aria-label') || el.innerText || '0';
          const match = text.match(/(\d[\d,]*)/);
          return match ? parseInt(match[1].replace(/,/g, '')) : 0;
        };

        const timestamp = timeEl ? timeEl.getAttribute('datetime') : new Date().toISOString();

        results.push({
          handle: handle,
          text: text,
          url: tweetUrl,
          likes: getCount('like'),
          retweets: getCount('retweet'),
          replies: getCount('reply'),
          timestamp: timestamp,
        });
      });

      return results;
    }, { handle, maxTweets: MAX_TWEETS_PER_ACCOUNT });

    return tweets.filter(t => t.text.length > 0);
  } catch (err) {
    console.error(`[scrape-twitter] Error scraping @${handle}: ${err.message}`);
    return [];
  }
}

async function main() {
  console.log(`[scrape-twitter] Scraping ${handles.length} accounts (timeout: ${SCRIPT_TIMEOUT_MS / 1000}s)...`);

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
  });
  const page = await context.newPage();

  const allTweets = [];
  let scraped = 0;
  let skipped = 0;

  for (const handle of handles) {
    // Check overall timeout before each account.
    if (isTimedOut()) {
      skipped = handles.length - scraped;
      console.log(`[scrape-twitter] Timeout reached after ${scraped} accounts, skipping ${skipped} remaining`);
      break;
    }

    console.log(`[scrape-twitter] Scraping @${handle}... (${scraped + 1}/${handles.length})`);
    const tweets = await scrapeAccount(page, handle);
    allTweets.push(...tweets);
    scraped++;

    // Brief delay between accounts to avoid rate limiting.
    if (!isTimedOut()) {
      await page.waitForTimeout(INTER_ACCOUNT_DELAY_MS);
    }
  }

  await browser.close();

  const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
  fs.writeFileSync(outputFile, JSON.stringify(allTweets, null, 2));
  console.log(`[scrape-twitter] Done! ${allTweets.length} tweets from ${scraped} accounts in ${elapsed}s (${skipped} skipped)`);
}

main().catch(err => {
  console.error(`[scrape-twitter] Fatal: ${err.message}`);
  // Write empty array on failure so Go wrapper doesn't crash.
  fs.writeFileSync(outputFile, '[]');
  process.exit(1);
});
