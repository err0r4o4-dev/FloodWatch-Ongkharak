const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const output = path.resolve('.tmp/fm91-ui');
fs.mkdirSync(output, { recursive: true });
const now = new Date().toISOString();
const headline = 'ข้อมูลทดสอบเท่านั้น · องครักษ์ น้ำท่วมคลอง 14';
const source = { code: 'FM91_NEWS', name: 'FM91', sourceType: 'MEDIA', url: 'https://fm91bkk.com/news', enabled: true, status: 'AVAILABLE', freshness: 'FRESH', lastSuccessAt: now, errorCode: null };
const article = { id: '00000000-0000-5000-8000-000000000123', articleId: '123', title: headline, description: 'SYNTHETIC_TEST_ONLY: ข่าวจำลองสำหรับตรวจหน้าจอ ไม่ใช่ข้อมูลน้ำท่วมจริง', url: 'https://fm91bkk.com/newsarticle/123', category: 'ข่าวจราจร', publishedAt: now, observedAt: null, retrievedAt: now, updatedAt: now, historyTotal: 2, provenance: 'MEDIA', status: 'PUBLISHED', freshness: 'FRESH', reason: null, imageURLs: [], locations: ['องครักษ์', 'คลอง 14'], roads: ['รังสิต-นครนายก', 'ทล.305'], eventTimes: ['เวลา 16.00 น.'], waterLevels: ['ข่าวจำลอง: ประมาณ 30 ซม.'], travelAdvice: ['ข่าวจำลอง: รถเล็กโปรดเลี่ยงเส้นทาง'], matchedKeywords: ['องครักษ์', 'น้ำท่วม'], latitude: null, longitude: null };

async function main() {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const checks = [];
  const exceptions = [];
  try {
    for (const scenario of [
      ['announcements', 1440, 'fresh'], ['announcements', 390, 'fresh'], ['announcements', 320, 'fresh'],
      ['overview', 1440, 'fresh'], ['overview', 390, 'fresh'],
      ['map', 1440, 'fresh'], ['map', 390, 'fresh'],
      ['announcements', 390, 'stale'], ['announcements', 390, 'error'], ['announcements', 390, 'empty'], ['announcements', 390, 'loading'],
    ]) {
      const [pathname, width, initialMode] = scenario;
      let mode = initialMode;
      const context = await browser.newContext({ viewport: { width, height: 950 }, reducedMotion: 'reduce' });
      const page = await context.newPage();
      page.on('pageerror', (error) => exceptions.push(`${pathname}/${width}: ${error.message}`));
      await page.route('https://**', (route) => route.abort());
      await page.route('**/api/v1/**', async (route) => {
        const routePath = new URL(route.request().url()).pathname.replace('/api/v1', '');
        const send = (data, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(data) });
        if (routePath === '/events/stream') return route.abort();
        if (routePath === '/auth/refresh' || routePath === '/auth/me') return send({ error: { code: 'UNAUTHENTICATED', message: 'Synthetic guest' } }, 401);
        if (routePath === '/news-announcements') {
          if (mode === 'loading') return new Promise(() => {});
          if (mode === 'error') return send({ error: { code: 'NEWS_ANNOUNCEMENTS_UNAVAILABLE', message: 'Synthetic outage' } }, 503);
          return send({ retrievedAt: now, source, items: mode === 'empty' ? [] : [{ ...article, ...(mode === 'stale' ? { freshness: 'STALE', publishedAt: '2020-01-01T00:00:00Z', reason: 'NEWS_STALE' } : {}) }], limit: 20, offset: 0, total: mode === 'empty' ? 0 : 1 });
        }
        if (routePath === '/official-announcements') return send({ retrievedAt: now, source: { code: 'TMD_CAP', name: 'Thai Meteorological Department', sourceType: 'OFFICIAL', url: 'https://www.tmd.go.th', enabled: false, status: 'NOT_CONFIGURED', freshness: 'UNKNOWN', lastSuccessAt: null }, items: [], limit: 20, offset: 0, total: 0 });
        if (routePath === '/external-data/ongkharak') return send({ retrievedAt: now, sources: [] });
        if (routePath === '/weather/ongkharak' || routePath === '/notifications/public-key') return send({ error: { code: 'UNAVAILABLE', message: 'Synthetic unavailable' } }, 503);
        return send({ items: [], pagination: { total: 0, limit: 20, offset: 0 }, limit: 20, offset: 0, total: 0 });
      });
      await page.goto(`http://127.0.0.1:15173/${pathname}`, { waitUntil: 'domcontentloaded' });
      const panel = page.locator('.fm91-news');
      await panel.waitFor({ state: 'visible' });
      const minimizeWeather = page.locator('.weather-widget__header button');
      if (await minimizeWeather.count()) await minimizeWeather.click();
      if (mode === 'error') {
        await panel.getByText('ข่าว FM91 ไม่พร้อมใช้งาน', { exact: true }).waitFor();
        assert.equal(await panel.getByText(headline, { exact: true }).count(), 0);
        await panel.screenshot({ path: path.join(output, `${pathname}-${width}-outage.png`) });
        mode = 'fresh';
        await panel.getByRole('button', { name: 'ลองโหลดข่าว FM91 อีกครั้ง' }).click();
        await panel.getByText(headline, { exact: true }).waitFor();
      } else if (mode === 'loading') {
        await panel.getByText('กำลังโหลดข่าว FM91', { exact: true }).waitFor();
        assert.equal(await panel.getByText(headline, { exact: true }).count(), 0);
      } else if (mode === 'empty') {
        await panel.getByText('ยังไม่มีข่าว FM91', { exact: true }).waitFor();
      } else {
        await panel.getByText(headline, { exact: true }).waitFor();
        assert.equal(await panel.getByText('เผยแพร่อัตโนมัติ', { exact: true }).count(), 1);
        const link = panel.locator('a[href="https://fm91bkk.com/newsarticle/123"]').first();
        assert.equal(await link.count(), 1);
        await link.focus();
        assert.equal(await link.evaluate((element) => element === document.activeElement), true);
      }
      const geometry = await page.evaluate(() => ({ width: window.innerWidth, content: document.documentElement.scrollWidth }));
      assert.ok(geometry.content <= geometry.width + 1, `horizontal overflow ${JSON.stringify(geometry)} at ${pathname}/${width}`);
      await panel.screenshot({ path: path.join(output, `${pathname}-${width}-${initialMode}.png`) });
      checks.push({ pathname, width, mode: initialMode, horizontalOverflow: false, rendered: true });
      await context.close();
    }
    assert.deepEqual(exceptions, []);
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ checks, exceptions }, null, 2));
    console.log(JSON.stringify({ passed: checks.length, exceptions }, null, 2));
  } finally {
    await browser.close();
  }
}

main().catch((error) => { console.error(error); process.exitCode = 1; });
