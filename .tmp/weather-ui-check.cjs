const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('.tmp/weather-dist');
const output = path.resolve('.tmp/weather-ui');
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
  await new Promise(resolve => server.listen(19187, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const errors = [];
  const checks = [];
  let mode = 'fresh';
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
    const page = await context.newPage();
    page.on('pageerror', error => errors.push(error.message));
    await page.route('https://**', route => route.abort());
    await page.route('**/api/v1/**', async route => {
      const pathname = new URL(route.request().url()).pathname;
      const reply = (status, body) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
      if (pathname.endsWith('/weather/ongkharak')) {
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
    const panel = page.locator('.weather-panel');
    for (const width of [1440, 768, 390, 320]) {
      await page.setViewportSize({ width, height: 1100 });
      await page.goto('http://127.0.0.1:19187/overview');
      await panel.getByText('30.5°C', { exact: true }).waitFor();
      assert.equal(await panel.getByRole('link', { name: 'Open-Meteo' }).getAttribute('href'), 'https://open-meteo.com/');
      assert.match(await panel.innerText(), /15.*นาที/);
      assert.match(await panel.innerText(), /แบบจำลอง/);
      assert(!/OFFICIAL|ปลอดภัยแล้ว|24 ชั่วโมง/.test(await panel.innerText()));
      assert(await panel.evaluate(element => element.scrollWidth <= element.clientWidth + 1));
      await panel.screenshot({ path: path.join(output, 'weather-' + width + '.png') });
      const attribution = panel.getByRole('link', { name: 'Open-Meteo' });
      await attribution.evaluate(element => element.scrollIntoView({ block: 'center' }));
      await attribution.focus();
      assert(await attribution.evaluate(element => {
        const rect = element.getBoundingClientRect();
        const navigation = document.querySelector('.bottom-navigation');
        const boundary = navigation && getComputedStyle(navigation).display !== 'none' ? navigation.getBoundingClientRect().top : window.innerHeight;
        return rect.top >= 0 && rect.bottom <= boundary;
      }));
      if (width <= 390) await panel.locator('.weather-panel__provider').screenshot({ path: path.join(output, 'weather-attribution-' + width + '.png') });
      checks.push('fresh TH no overflow ' + width);
    }
    mode = 'stale'; await page.reload(); await panel.getByText('ข้อมูลเก่า', { exact: true }).waitFor();
    assert.match(await panel.innerText(), /30.5°C/);
    await panel.screenshot({ path: path.join(output, 'weather-stale.png') }); checks.push('stale labeled');
    mode = 'unknown'; await page.reload(); await panel.getByRole('button').waitFor();
    assert(!/30.5°C/.test(await panel.innerText()));
    const retry = panel.getByRole('button'); await retry.focus(); assert(await retry.evaluate(element => element === document.activeElement));
    mode = 'fresh'; await retry.press('Enter'); await panel.getByText('30.5°C', { exact: true }).waitFor(); checks.push('unknown, keyboard retry recovery');
    mode = 'error'; await page.reload(); await panel.getByRole('button').waitFor();
    assert(!/30.5°C/.test(await panel.innerText())); await panel.screenshot({ path: path.join(output, 'weather-error.png') }); checks.push('transport failure no measurements');
    mode = 'loading'; await page.reload(); await panel.getByRole('status').waitFor();
    await panel.screenshot({ path: path.join(output, 'weather-loading.png') }); await panel.getByText('30.5°C', { exact: true }).waitFor(); checks.push('loading');
    mode = 'fresh'; await page.evaluate(() => localStorage.setItem('floodwatch.locale', 'en')); await page.reload();
    await panel.getByText('30.5°C', { exact: true }).waitFor(); assert.match(await panel.innerText(), /model/i); assert.match(await panel.innerText(), /15.*min/);
    await panel.screenshot({ path: path.join(output, 'weather-en.png') }); checks.push('EN model, interval, attribution');
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    await panel.screenshot({ path: path.join(output, 'weather-320-200-percent.png') });
    console.log('200 percent overflow diagnostics', await panel.evaluate(element => ({width:element.clientWidth,scroll:element.scrollWidth,children:[...element.querySelectorAll('*')].filter(child=>child.scrollWidth>child.clientWidth+1).map(child=>({tag:child.tagName,class:child.className,width:child.clientWidth,scroll:child.scrollWidth,text:child.innerText})).slice(0,8)})));
    assert(await panel.evaluate(element => element.scrollWidth <= element.clientWidth + 1)); checks.push('320px 200% text no panel overflow');
    assert.deepEqual(errors, []);
    const result = { checks, errors, evidence: 'Synthetic weather responses, not live flood or local station evidence' };
    fs.writeFileSync(path.join(output, 'checks.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result, null, 2));
  } finally { await browser.close(); await new Promise(resolve => server.close(resolve)); }
})().catch(error => { console.error(error); process.exitCode = 1; });
