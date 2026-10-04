const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/weather-main-ui');
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
  await new Promise(resolve => server.listen(19191, '127.0.0.1', resolve));
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
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      const count = weatherRequests;
      await page.goto('http://127.0.0.1:19191/');
      await page.getByRole('heading', { level: 1 }).waitFor();
      assert.equal(await page.locator('.weather-floating').count(), 0);
      assert.equal(weatherRequests, count);
      await page.screenshot({ path: path.join(output, 'main-' + width + '.png') });
      await page.goto('http://127.0.0.1:19191/overview');
      const card = page.locator('.weather-widget:visible');
      await card.getByText('30.5°C', { exact: true }).waitFor();
      assert.equal(await card.count(), 1);
      await card.getByRole('button', { name: 'แสดงทั้งหมด' }).click();
      const details = page.locator('.weather-dialog[open]');
      await details.getByRole('link', { name: 'Open-Meteo' }).waitFor();
      assert(!(await details.innerText()).includes('แบบจำลองสภาพอากาศ ไม่ใช่สถานีตรวจวัด'));
      await details.screenshot({ path: path.join(output, 'details-' + width + '.png') });
      await page.keyboard.press('Escape');
      await page.locator('.weather-floating').waitFor({ state: 'visible' });
      checks.push(width + 'px: no main popup/request; one Overview popup; removed text; attribution and close preserved');
    }
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({ checks, errors, evidence: 'Synthetic intercepted weather, not live local conditions' }, null, 2));
  } finally { await browser.close(); await new Promise(resolve => server.close(resolve)); }
})().catch(error => { console.error(error); process.exitCode = 1; });
