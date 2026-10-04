const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('.tmp/weather-campus-dist');
const output = path.resolve('.tmp/weather-campus-ui');
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
          location: { name: 'Ongkharak', latitude: 14.106458, longitude: 100.983765 },
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
    for (const [width, height] of [[1440, 900], [390, 844]]) {
      await page.setViewportSize({ width, height });
      const count = weatherRequests;
      await page.goto('http://127.0.0.1:19190/overview');
      await card.getByText('30.5°C', { exact: true }).waitFor();
      assert.equal(await card.count(), 1);
      assert(await card.getByRole('heading', { name: 'สภาพอากาศ มศว องครักษ์', exact: true }).isVisible());
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
      assert(await details.getByText('มหาวิทยาลัยศรีนครินทรวิโรฒ องครักษ์ จังหวัดนครนายก', { exact: true }).isVisible());
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
    await page.evaluate(() => localStorage.setItem('floodwatch.locale', 'en'));
    await page.reload();
    await card.getByRole('heading', { name: 'SWU Ongkharak weather', exact: true }).waitFor();
    await card.getByRole('button', { name: 'Show all' }).click();
    await details.getByRole('heading', { name: 'SWU Ongkharak weather details', exact: true }).waitFor();
    assert(await details.getByText('Srinakharinwirot University, Ongkharak Campus, Nakhon Nayok', { exact: true }).isVisible());
    assert(await details.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
    await details.screenshot({ path: path.join(output, 'campus-details-en-390.png') });
    checks.push('English campus title and location in mobile details');
    assert.deepEqual(errors, []);
    const result = { checks, errors, evidence: 'Synthetic intercepted weather; not live local conditions or physical device verification' };
    fs.writeFileSync(path.join(output, 'checks.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result, null, 2));
    async function navigationCount() { return page.locator('.bottom-navigation:visible').count(); }
  } finally { await browser.close(); await new Promise(resolve => server.close(resolve)); }
})().catch(error => { console.error(error); process.exitCode = 1; });

