const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/weather-sidebar-ui');
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.jpg': 'image/jpeg', '.png': 'image/png', '.woff2': 'font/woff2' };
fs.mkdirSync(output, { recursive: true });
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  const target = path.resolve(root, '.' + pathname);
  if (!target.startsWith(root + path.sep) && target !== root) { response.writeHead(403); response.end(); return; }
  const file = fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(response);
});

(async () => {
  await new Promise(resolve => server.listen(19189, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const errors = [];
  const checks = [];
  let mode = 'fresh';
  let weatherRequests = 0;
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
    const page = await context.newPage();
    page.on('pageerror', error => errors.push(error.message));
    await page.route('https://**', route => route.abort());
    await page.route('**/api/v1/**', async route => {
      const pathname = new URL(route.request().url()).pathname;
      const reply = (status, body) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
      if (pathname.endsWith('/weather/ongkharak')) {
        weatherRequests++;
        if (mode === 'loading') await new Promise(resolve => setTimeout(resolve, 1500));
        if (mode === 'error') return reply(502, { error: { code: 'TEST_ONLY', message: 'Synthetic failure' } });
        const absent = mode === 'unknown';
        return reply(absent ? 503 : 200, {
          location: { name: 'Ongkharak', latitude: 14.1, longitude: 101 },
          source: { name: 'Open-Meteo', url: 'https://open-meteo.com/', kind: 'MODEL' },
          freshness: absent ? 'UNKNOWN' : mode === 'stale' ? 'STALE' : 'FRESH',
          retrievedAt: absent ? null : new Date().toISOString(),
          reason: absent || mode === 'stale' ? 'UPSTREAM_UNAVAILABLE' : null,
          current: absent ? null : {
            observedAt: new Date(Date.now() - (mode === 'stale' ? 90 * 60000 : 5 * 60000)).toISOString(),
            intervalSeconds: 900, temperatureC: 30.5, apparentTemperatureC: 35.5,
            humidityPercent: 72, precipitationMm: 0.1, weatherCode: 3,
            windSpeedKmh: 12, windDirectionDegrees: 180, isDay: true
          }
        });
      }
      if (pathname.endsWith('/auth/google/config')) return reply(200, { enabled: false });
      if (pathname.includes('/auth/')) return reply(401, { error: { code: 'UNAUTHORIZED', message: 'Synthetic guest' } });
      if (pathname.endsWith('/events/stream')) return route.abort();
      return reply(200, { items: [], limit: 20, offset: 0 });
    });
    const card = page.locator('.weather-widget:visible');
    const details = page.locator('.weather-dialog[open]');
    for (const width of [1440, 1280, 1200, 768, 390, 320]) {
      await page.setViewportSize({ width, height: 900 });
      const count = weatherRequests;
      await page.goto('http://127.0.0.1:19189/overview');
      await card.getByText('30.5°C', { exact: true }).waitFor();
      assert.equal(await card.count(), 1);
      assert.equal(weatherRequests - count, 1);
      assert(await card.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
      if (width > 1200) {
        const bounds = await card.boundingBox();
        const sidebar = await page.locator('.desktop-sidebar').boundingBox();
        assert(bounds.x >= sidebar.x && bounds.x + bounds.width <= sidebar.x + sidebar.width + 1);
        assert(bounds.y + bounds.height <= 900);
        await page.screenshot({ path: path.join(output, 'desktop-' + width + '.png') });
      } else {
        assert.equal(await page.locator('.desktop-sidebar').isVisible(), false);
        await card.evaluate(element => element.scrollIntoView({ block: 'center' }));
        const cardBounds = await card.boundingBox();
        const navigationBounds = await page.locator('.bottom-navigation').boundingBox();
        assert(cardBounds.y + cardBounds.height <= navigationBounds.y);
        await card.screenshot({ path: path.join(output, 'mobile-' + width + '.png') });
      }
      const opener = card.getByRole('button', { name: 'ดูรายละเอียดสภาพอากาศ' });
      await opener.focus(); await opener.press('Enter');
      await details.getByRole('link', { name: 'Open-Meteo' }).waitFor();
      assert.equal(weatherRequests - count, 1);
      assert(await details.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
      for (let index = 0; index < 6; index++) {
        await page.keyboard.press('Tab');
        const focus = await details.evaluate(element => ({ inside: element.contains(document.activeElement), tag: document.activeElement.tagName, html: document.activeElement.outerHTML.slice(0, 180) }));
        assert(focus.inside || focus.tag === 'BODY', JSON.stringify(focus));
      }
      await details.screenshot({ path: path.join(output, 'details-' + width + '.png') });
      await page.keyboard.press('Escape');
      await details.waitFor({ state: 'hidden' });
      assert(await opener.evaluate(element => element === document.activeElement));
      await card.getByRole('button', { name: 'ย่อการ์ดสภาพอากาศ' }).click();
      const reopen = page.getByRole('button', { name: 'เปิดการ์ดสภาพอากาศ' }).filter({ visible: true });
      await reopen.waitFor();
      assert(await reopen.evaluate(element => element === document.activeElement));
      await reopen.press('Enter'); await card.getByText('30.5°C', { exact: true }).waitFor();
      if (width <= 390) {
        for (const button of await card.getByRole('button').all()) assert((await button.boundingBox()).height >= 44);
      }
      checks.push(width + 'px: single request, card fit, dialog, focus trap, Escape, minimize/reopen');
    }
    await page.setViewportSize({ width: 1440, height: 700 });
    await page.goto('http://127.0.0.1:19189/overview');
    await card.getByText('30.5°C', { exact: true }).waitFor();
    const beforeNavigation = weatherRequests;
    await page.locator('.desktop-sidebar').getByRole('link', { name: 'ประกาศ', exact: true }).click();
    await page.waitForURL('**/announcements');
    assert.equal(weatherRequests, beforeNavigation);
    assert(await card.isVisible());
    checks.push('public navigation preserves shared weather');
    for (const state of ['stale', 'unknown', 'error', 'loading']) {
      mode = state;
      await page.reload();
      if (state === 'stale') await card.getByText('ข้อมูลเก่า', { exact: true }).waitFor();
      if (state === 'unknown') { await card.getByText('ไม่ทราบสถานะ', { exact: true }).waitFor(); assert(!/30.5°C/.test(await card.innerText())); }
      if (state === 'error') await card.getByText('ไม่สามารถโหลดข้อมูลสภาพอากาศได้', { exact: true }).waitFor();
      if (state === 'loading') await card.getByRole('status').waitFor();
      await card.screenshot({ path: path.join(output, 'state-' + state + '.png') });
      if (state === 'unknown' || state === 'error') {
        mode = 'fresh'; const retry = card.getByRole('button', { name: 'ลองโหลดสภาพอากาศอีกครั้ง' });
        await retry.focus(); await retry.press('Enter'); await card.getByText('30.5°C', { exact: true }).waitFor();
      }
      checks.push(state + ' state');
    }
    mode = 'fresh';
    await page.setViewportSize({ width: 320, height: 1000 });
    await page.goto('http://127.0.0.1:19189/overview');
    await card.getByText('30.5°C', { exact: true }).waitFor();
    await page.evaluate(() => localStorage.setItem('floodwatch.locale', 'en')); await page.reload();
    await card.getByRole('button', { name: 'View weather details' }).waitFor();
    await card.screenshot({ path: path.join(output, 'mobile-en.png') });
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    assert(await card.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
    await card.getByRole('button', { name: 'View weather details' }).click();
    console.log('zoom diagnostics', await details.evaluate(element => ({ width: element.clientWidth, scroll: element.scrollWidth, children: [...element.querySelectorAll('*')].filter(child => child.scrollWidth > child.clientWidth + 1).map(child => ({ tag: child.tagName, class: child.className, width: child.clientWidth, scroll: child.scrollWidth, text: child.innerText })).slice(0, 8) })));
    await details.screenshot({ path: path.join(output, 'details-320-200-percent.png') });
    assert(await details.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
    await details.screenshot({ path: path.join(output, 'details-320-200-percent.png') });
    checks.push('English, 320px 200% text card/dialog fit');
    assert.deepEqual(errors, []);
    const result = { checks, errors, evidence: 'Synthetic intercepted weather, not live local conditions' };
    fs.writeFileSync(path.join(output, 'checks.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result, null, 2));
  } finally { await browser.close(); await new Promise(resolve => server.close(resolve)); }
})().catch(error => { console.error(error); process.exitCode = 1; });
