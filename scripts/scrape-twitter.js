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

async function scrapeAccount(page, handle) {
  const url = `https://x.com/${handle}`;
  try {
    await page.goto(url, { waitUntil: 'networkidle', timeout: 15000 });
    await page.waitForTimeout(2000);

    const tweets = await page.evaluate((handle) => {
      const articles = document.querySelectorAll('article[data-testid="tweet"]');
      const results = [];

      articles.forEach((article, i) => {
        if (i >= 5) return; // Max 5 tweets per account.

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
    }, handle);

    return tweets.filter(t => t.text.length > 0);
  } catch (err) {
    console.error(`[scrape-twitter] Error scraping @${handle}: ${err.message}`);
    return [];
  }
}

async function main() {
  console.log(`[scrape-twitter] Scraping ${handles.length} accounts...`);

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
  });
  const page = await context.newPage();

  const allTweets = [];

  for (const handle of handles) {
    console.log(`[scrape-twitter] Scraping @${handle}...`);
    const tweets = await scrapeAccount(page, handle);
    allTweets.push(...tweets);
    // Rate limit: 2 seconds between accounts.
    await page.waitForTimeout(2000);
  }

  await browser.close();

  fs.writeFileSync(outputFile, JSON.stringify(allTweets, null, 2));
  console.log(`[scrape-twitter] Done! ${allTweets.length} tweets from ${handles.length} accounts`);
}

main().catch(err => {
  console.error(`[scrape-twitter] Fatal: ${err.message}`);
  // Write empty array on failure so Go wrapper doesn't crash.
  fs.writeFileSync(outputFile, '[]');
  process.exit(1);
});
