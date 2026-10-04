const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/weather-floating-ui');
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
  await new Promise(resolve => server.listen(19190, '127.0.0.1', resolve));
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
    const handleApi = async route => {
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
    };
    await page.route('**/api/v1/**', handleApi);
    const card = page.locator('.weather-widget:visible');
    const floating = page.locator('.weather-floating');
    const details = page.locator('.weather-dialog[open]');
    for (const [width, height] of [[1440, 900], [1280, 800], [1366, 1024], [1024, 768], [768, 1024], [390, 844], [320, 700], [844, 390]]) {
      await page.setViewportSize({ width, height });
      const count = weatherRequests;
      await page.goto('http://127.0.0.1:19190/overview');
      await card.getByText('30.5°C', { exact: true }).waitFor();
      assert.equal(await card.count(), 1);
      assert.equal(weatherRequests - count, 1);
      assert.equal(await floating.evaluate(element => getComputedStyle(element).position), 'fixed');
      assert.equal(await floating.evaluate(element => element.closest('.desktop-sidebar,main')), null);
      let bounds = await floating.boundingBox();
      assert(bounds.x >= 12 && bounds.x <= 32 && bounds.x + bounds.width <= width);
      assert(bounds.y >= 0 && bounds.y + bounds.height <= height);
      const navigation = page.locator('.bottom-navigation');
      if (await navigation.isVisible()) assert(bounds.y + bounds.height <= (await navigation.boundingBox()).y);
      assert(await floating.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
      await page.evaluate(() => { window.scrollTo(0, 500); const main = document.querySelector('.page-content'); if (main) main.scrollTop = 400; });
      const afterScroll = await floating.boundingBox();
      assert(Math.abs(afterScroll.x - bounds.x) < 1 && Math.abs(afterScroll.y - bounds.y) < 1);
      await page.screenshot({ path: path.join(output, 'floating-' + width + '-' + height + '.png') });
      const opener = card.getByRole('button', { name: 'แสดงทั้งหมด', exact: true });
      await opener.focus(); await opener.press('Enter');
      await details.getByRole('link', { name: 'Open-Meteo' }).waitFor();
      assert.equal(await floating.isVisible(), false);
      assert.equal(weatherRequests - count, 1);
      const dialogBounds = await details.boundingBox();
      assert(Math.abs(dialogBounds.x + dialogBounds.width / 2 - width / 2) <= 2);
      assert(Math.abs(dialogBounds.y + dialogBounds.height / 2 - height / 2) <= 2);
      assert(dialogBounds.y >= 0 && dialogBounds.y + dialogBounds.height <= height);
      assert(await details.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
      for (let index = 0; index < 5; index++) {
        await page.keyboard.press('Tab');
        assert(await details.evaluate(element => element.contains(document.activeElement) || document.activeElement === document.body));
      }
      await details.screenshot({ path: path.join(output, 'centered-' + width + '-' + height + '.png') });
      await page.keyboard.press('Escape'); await details.waitFor({ state: 'hidden' });
      await floating.waitFor({ state: 'visible' });
      assert(await floating.isVisible());
      assert(await opener.evaluate(element => element === document.activeElement));
      await opener.click(); await details.getByRole('button', { name: 'ปิดรายละเอียดสภาพอากาศ' }).click();
      await details.waitFor({ state: 'hidden' }); await floating.waitFor({ state: 'visible' }); assert(await floating.isVisible());
      await card.getByRole('button', { name: 'ย่อการ์ดสภาพอากาศ' }).click();
      const reopen = page.getByRole('button', { name: 'เปิดการ์ดสภาพอากาศ' });
      await reopen.waitFor(); assert.equal(await reopen.innerText(), '');
      assert.equal(await reopen.locator('svg').count(), 1);
      bounds = await floating.boundingBox(); assert(bounds.width >= 44 && bounds.width <= 64);
      await page.reload(); await reopen.waitFor(); assert.equal(await card.count(), 0);
      await reopen.focus(); await reopen.press('Enter'); await card.getByText('30.5°C', { exact: true }).waitFor();
      for (const button of await card.getByRole('button').all()) assert((await button.boundingBox()).height >= 44);
      checks.push(width + 'x' + height + ': floating left, stable scroll, centered details, Escape/close, cloud-only persisted minimize, 44px controls');
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('http://127.0.0.1:19190/overview'); await card.getByText('30.5°C', { exact: true }).waitFor();
    const beforeNavigation = weatherRequests;
    await page.locator('.bottom-navigation').getByRole('link', { name: 'ประกาศ', exact: true }).click();
    await page.waitForURL('**/announcements'); assert.equal(weatherRequests, beforeNavigation);
    await card.getByRole('button', { name: 'ย่อการ์ดสภาพอากาศ' }).click();
    await page.locator('.bottom-navigation').getByRole('link', { name: 'แผนที่และพื้นที่เสี่ยง', exact: true }).click();
    await page.waitForURL('**/map'); await page.getByRole('button', { name: 'เปิดการ์ดสภาพอากาศ' }).waitFor();
    assert.equal(weatherRequests, beforeNavigation);
    await page.screenshot({ path: path.join(output, 'map-cloud.png') });
    await page.getByRole('button', { name: 'เปิดการ์ดสภาพอากาศ' }).click();
    await page.screenshot({ path: path.join(output, 'map-summary.png') });
    checks.push('public route navigation, map, single shared request and minimize state');
    for (const state of ['stale', 'unknown', 'error', 'loading']) {
      mode = state; await page.goto('http://127.0.0.1:19190/overview');
      if (state === 'stale') await card.getByText('ข้อมูลเก่า', { exact: true }).waitFor();
      if (state === 'unknown') { await card.getByText('ไม่ทราบสถานะ', { exact: true }).waitFor(); assert(!/30.5°C/.test(await card.innerText())); }
      if (state === 'error') await card.getByText('ไม่สามารถโหลดข้อมูลสภาพอากาศได้', { exact: true }).waitFor();
      if (state === 'loading') await card.getByRole('status').waitFor();
      await card.screenshot({ path: path.join(output, 'state-' + state + '.png') });
      if (state === 'unknown' || state === 'error') {
        mode = 'fresh'; const retry = card.getByRole('button', { name: 'ลองโหลดสภาพอากาศอีกครั้ง' });
        await retry.focus(); await retry.press('Enter'); await card.getByText('30.5°C', { exact: true }).waitFor();
      }
      checks.push(state + ' state and applicable keyboard retry');
    }
    mode = 'fresh'; await page.goto('http://127.0.0.1:19190/');
    await card.getByText('30.5°C', { exact: true }).waitFor();
    assert.equal(await navigationCount(), 0);
    let landingBounds = await floating.boundingBox(); assert(844 - landingBounds.y - landingBounds.height <= 32);
    await page.screenshot({ path: path.join(output, 'landing-summary.png') });
    checks.push('landing page popup without bottom-navigation offset');
    await page.goto('http://127.0.0.1:19190/admin/login'); assert.equal(await floating.count(), 0);
    checks.push('authentication route has no weather popup');
    await page.setViewportSize({ width: 320, height: 1000 });
    await page.goto('http://127.0.0.1:19190/overview'); await card.getByText('30.5°C', { exact: true }).waitFor();
    await page.evaluate(() => localStorage.setItem('floodwatch.locale', 'en')); await page.reload();
    await card.getByRole('button', { name: 'Show all' }).waitFor();
    await card.screenshot({ path: path.join(output, 'floating-en.png') });
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    assert(await floating.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
    await card.getByRole('button', { name: 'Show all' }).click();
    assert(await details.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
    await details.screenshot({ path: path.join(output, 'centered-320-200-percent.png') });
    checks.push('English, 320px/200% text popup and dialog fit');
    const tabletContext = await browser.newContext({ hasTouch: true, viewport: { width: 1366, height: 1024 }, reducedMotion: 'reduce' });
    const tabletPage = await tabletContext.newPage();
    tabletPage.on('pageerror', error => errors.push(error.message));
    await tabletPage.route('https://**', route => route.abort());
    await tabletPage.route('**/api/v1/**', handleApi);
    await tabletPage.goto('http://127.0.0.1:19190/overview');
    await tabletPage.locator('.weather-widget').getByText('30.5°C', { exact: true }).waitFor();
    assert(await tabletPage.locator('.bottom-navigation').isVisible());
    const tabletBounds = await tabletPage.locator('.weather-floating').boundingBox();
    assert(tabletBounds.y + tabletBounds.height <= (await tabletPage.locator('.bottom-navigation').boundingBox()).y);
    await tabletPage.screenshot({ path: path.join(output, 'ipad-touch-landscape.png') });
    await tabletPage.getByRole('button', { name: 'แสดงทั้งหมด' }).tap();
    await tabletPage.locator('.weather-dialog[open]').waitFor();
    const tabletDialogBounds = await tabletPage.locator('.weather-dialog[open]').boundingBox();
    assert(Math.abs(tabletDialogBounds.x + tabletDialogBounds.width / 2 - 683) <= 2);
    assert(Math.abs(tabletDialogBounds.y + tabletDialogBounds.height / 2 - 512) <= 2);
    await tabletPage.getByRole('button', { name: 'ปิดรายละเอียดสภาพอากาศ' }).tap();
    await tabletPage.getByRole('button', { name: 'ย่อการ์ดสภาพอากาศ' }).tap();
    await tabletPage.getByRole('button', { name: 'เปิดการ์ดสภาพอากาศ' }).tap();
    await tabletContext.close();
    checks.push('1366x1024 touch emulation: bottom-navigation clearance, centered modal, touch close/minimize/reopen');
    assert.deepEqual(errors, []);
    const result = { checks, errors, evidence: 'Synthetic intercepted weather; not live local conditions or physical device verification' };
    fs.writeFileSync(path.join(output, 'checks.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result, null, 2));
    async function navigationCount() { return page.locator('.bottom-navigation:visible').count(); }
  } finally { await browser.close(); await new Promise(resolve => server.close(resolve)); }
})().catch(error => { console.error(error); process.exitCode = 1; });
