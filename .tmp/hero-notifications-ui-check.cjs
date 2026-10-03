const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('.tmp/hero-notifications-dist');
const output = path.resolve('.tmp/hero-notifications-ui');
fs.mkdirSync(output, { recursive: true });
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  const target = path.resolve(root, '.' + pathname);
  const safe = target === root || target.startsWith(root + path.sep);
  const file = safe && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});

(async () => {
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const checks = [];
  const errors = [];
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('https://**', (route) => route.abort());
    await page.route('**/api/v1/**', (route) => {
      const pathname = new URL(route.request().url()).pathname;
      if (pathname.includes('/auth/')) return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: { code: 'UNAUTHORIZED', message: 'Isolated guest fixture' } }) });
      if (pathname.endsWith('/events/stream')) return route.abort();
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ items: [], limit: 20, offset: 0 }) });
    });

    async function appearance(hero) {
      return hero.evaluate((element) => {
        const style = getComputedStyle(element);
        const heading = getComputedStyle(element.querySelector('h1'));
        const copy = element.querySelector('.page-hero__copy, .notifications-hero__copy');
        const paragraph = getComputedStyle(copy.querySelector('p:not(.eyebrow)'));
        const art = element.querySelector('.page-hero__art, .notifications-hero__art');
        const tile = getComputedStyle(art.querySelector('.page-hero__tile--large, .notifications-hero__tile--bell'));
        return {
          background: style.backgroundImage, border: style.border, radius: style.borderRadius, shadow: style.boxShadow,
          padding: style.padding, minHeight: style.minHeight, alignItems: style.alignItems, gap: style.gap,
          titleFont: heading.fontSize, titleWeight: heading.fontWeight, titleLineHeight: heading.lineHeight,
          descriptionFont: paragraph.fontSize, descriptionColor: paragraph.color,
          tileWidth: tile.width, tileHeight: tile.height, tileRadius: tile.borderRadius,
          artOpacity: getComputedStyle(art).opacity
        };
      });
    }

    for (const width of [1440, 768, 390, 320]) {
      await page.setViewportSize({ width, height: 1000 });
      await page.goto(origin + '/notifications');
      await page.evaluate(() => document.fonts.ready);
      const reference = await appearance(page.locator('.notifications-hero'));
      const referenceHeader = await page.locator('.institution-header').evaluate((element) => ({ height: element.getBoundingClientRect().height, background: getComputedStyle(element).backgroundColor }));
      await page.locator('.notifications-hero').screenshot({ path: path.join(output, `reference-notifications-${width}.png`) });
      for (const route of ['/', '/map', '/risk', '/announcements', '/reports']) {
        await page.goto(origin + route);
        const hero = page.locator('.page-hero');
        await hero.waitFor();
        await page.evaluate(() => document.fonts.ready);
        const actual = await appearance(hero);
        assert.deepEqual(actual, reference, `hero theme mismatch on ${route} at ${width}px`);
        assert.ok(!actual.background.includes('url('), 'no photographic hero background');
        assert.equal(await hero.locator('.page-hero__art').getAttribute('aria-hidden'), 'true');
        assert.equal(await hero.locator('.page-hero__tile').count(), 3);
        assert.equal(await hero.locator('.page-hero__credit').count(), 0);
        assert.equal(await hero.getByRole('heading', { level: 1 }).count(), 1);
        const widths = await page.evaluate(() => ({ viewport: innerWidth, body: document.body.scrollWidth, document: document.documentElement.scrollWidth }));
        assert.ok(widths.body <= width + 1 && widths.document <= width + 1, JSON.stringify(widths));
        const header = await page.locator('.institution-header').evaluate((element) => ({ height: element.getBoundingClientRect().height, background: getComputedStyle(element).backgroundColor }));
        assert.deepEqual(header, referenceHeader, 'header remains unchanged across routes');
        if (route === '/map' || route === '/') {
          const content = page.locator(route === '/map' ? '.map-page__workspace' : '.summary-metrics');
          const heroBounds = await hero.boundingBox();
          const contentBounds = await content.boundingBox();
          assert.ok(contentBounds.y >= heroBounds.y + heroBounds.height, 'content must not overlap compact hero');
        }
        await hero.screenshot({ path: path.join(output, `hero-${route === '/' ? 'overview' : route.slice(1)}-${width}.png`) });
        if (route === '/') await page.screenshot({ path: path.join(output, `overview-${width}.png`) });
        checks.push(`${route} matches notifications hero at ${width}px; header unchanged; no overflow`);
      }
    }
    assert.deepEqual(errors, []);
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ checks, errors, fixtures: 'Isolated empty guest APIs; no live flood data or service mutations' }, null, 2));
    console.log('PASS: ' + checks.join('; '));
  } finally {
    await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); server.close(); process.exitCode = 1; });
