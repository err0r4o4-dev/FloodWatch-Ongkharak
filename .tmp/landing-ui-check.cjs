const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('.tmp/landing-dist');
const output = path.resolve('.tmp/landing-ui');
const origin = 'http://127.0.0.1:19179';
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2', '.jpg': 'image/jpeg' };
fs.mkdirSync(output, { recursive: true });
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, origin).pathname;
  const target = path.resolve(root, '.' + pathname);
  const safe = target === root || target.startsWith(root + path.sep);
  const file = safe && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});

(async () => {
  await new Promise((resolve) => server.listen(19179, '127.0.0.1', resolve));
  let browser;
  const checks = [];
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
    const feeds = [];
    await context.route('**/*', async (route) => {
      const url = new URL(route.request().url());
      if (url.origin !== origin) return route.abort();
      if (url.pathname.startsWith('/api/v1/')) {
        if (url.pathname.startsWith('/api/v1/auth/')) return route.fulfill({ status: 401, json: { error: { code: 'UNAUTHORIZED', message: 'Isolated guest UI verification' } } });
        if (url.pathname.includes('/events')) return route.abort();
        feeds.push(url.pathname);
        return route.fulfill({ json: { items: [], limit: 20, offset: 0 } });
      }
      return route.continue();
    });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    for (const width of [320, 390, 768, 1024, 1440]) {
      await page.setViewportSize({ width, height: 1000 });
      await page.goto(origin);
      await page.getByRole('heading', { level: 1, name: 'ติดตามสถานการณ์น้ำ ก่อนออกเดินทาง' }).waitFor();
      await page.evaluate(() => document.fonts.ready);
      const dimensions = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, document: document.documentElement.scrollWidth }));
      assert(dimensions.document <= dimensions.viewport, `Landing overflow at ${width}: ${JSON.stringify(dimensions)}`);
      assert.equal(await page.getByRole('navigation', { name: 'การนำทางมือถือ' }).count(), 0);
      const shortTargets = await page.locator('.landing-page a:not(.skip-link)').evaluateAll((links) => links.filter((link) => link.getBoundingClientRect().height < 43.9).map((link) => link.textContent));
      assert.deepEqual(shortTargets, [], `Touch targets at ${width}`);
      await page.screenshot({ path: path.join(output, `landing-${width}.png`), fullPage: true });
      checks.push({ width, ...dimensions, touchTargets: 'pass' });
    }
    assert.equal(feeds.length, 0, 'Introductory landing must not request public flood feeds');
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(origin);
    await page.keyboard.press('Tab');
    assert.equal(await page.locator(':focus').textContent(), 'ข้ามไปยังเนื้อหาหลัก');
    await page.keyboard.press('Enter');
    assert.equal(await page.locator(':focus').getAttribute('id'), 'main-content');
    await page.keyboard.press('Tab');
    assert.equal((await page.locator(':focus').textContent()).trim(), 'เข้าสู่เมนู →');
    await page.keyboard.press('Enter');
    await page.getByRole('heading', { level: 1, name: 'ภาพรวมสถานการณ์น้ำ' }).waitFor();
    assert.equal(new URL(page.url()).pathname, '/overview');
    assert.equal(await page.getByRole('navigation', { name: 'การนำทางมือถือ' }).getByRole('link', { name: 'ภาพรวม' }).getAttribute('aria-current'), 'page');
    assert.equal(await page.getByRole('navigation', { name: 'การนำทางมือถือ' }).getByRole('link').count(), 4);
    await page.screenshot({ path: path.join(output, 'overview-from-main-390.png'), fullPage: true });
    await page.goto(`${origin}/overview`);
    await page.getByRole('heading', { level: 1, name: 'ภาพรวมสถานการณ์น้ำ' }).waitFor();
    await page.getByRole('link', { name: 'Flood Watch Ongkharak หน้าหลัก' }).filter({ visible: true }).first().click();
    await page.getByRole('heading', { level: 1, name: 'ติดตามสถานการณ์น้ำ ก่อนออกเดินทาง' }).waitFor();
    await page.getByRole('link', { name: 'รู้จักระบบ' }).click();
    assert.equal(new URL(page.url()).hash, '#how-it-works');
    await page.getByRole('heading', { name: 'เริ่มติดตามสถานการณ์ได้ใน 3 ขั้นตอน' }).waitFor();
    await page.goto(origin);
    await page.getByRole('link', { name: 'ดูแผนที่', exact: true }).click();
    await page.getByRole('heading', { level: 1, name: 'แผนที่สถานการณ์น้ำ' }).waitFor();
    assert.equal(new URL(page.url()).pathname, '/map');
    await page.goto(origin);
    await page.getByRole('link', { name: 'เข้าสู่ระบบ', exact: true }).click();
    await page.getByRole('heading', { level: 1, name: 'เข้าสู่ระบบ' }).waitFor();
    assert.equal(new URL(page.url()).searchParams.get('returnTo'), '/overview');
    await page.goto(origin);
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    await page.evaluate(() => document.fonts.ready);
    const zoom = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, document: document.documentElement.scrollWidth }));
    assert(zoom.document <= zoom.viewport, `200% text overflow: ${JSON.stringify(zoom)}`);
    await page.screenshot({ path: path.join(output, 'landing-390-200-percent.png'), fullPage: true });
    assert.deepEqual(errors, [], 'Uncaught browser errors');
    checks.push({ keyboard: 'pass', routes: 'pass', overviewBookmark: 'pass', loginReturnTo: '/overview', textZoom: zoom, pageErrors: errors });
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify(checks, null, 2));
    console.log(JSON.stringify(checks, null, 2));
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
