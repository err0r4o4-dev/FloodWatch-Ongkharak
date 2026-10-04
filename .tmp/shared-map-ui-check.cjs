const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/shared-map-ui');
fs.mkdirSync(output, { recursive: true });
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.jpg': 'image/jpeg', '.png': 'image/png', '.woff2': 'font/woff2' };
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  const target = path.resolve(root, '.' + decodeURIComponent(pathname));
  if (!target.startsWith(root + path.sep)) { response.writeHead(403); response.end(); return; }
  const file = fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  fs.createReadStream(file).pipe(response);
});

async function run() {
  await new Promise(resolve => server.listen(19195, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const findings = [];
  try {
    for (const [width, height] of [[1440, 1000], [390, 844], [320, 740]]) {
      const context = await browser.newContext({ viewport: { width, height } });
      await context.addInitScript(() => {
        window.__gpsRequests = 0;
        window.__locationFailure = false;
        Object.defineProperty(navigator, 'geolocation', { configurable: true, value: {
          getCurrentPosition(success, failure) {
            window.__gpsRequests += 1;
            setTimeout(() => window.__locationFailure
              ? failure({ code: 1, message: 'Verification permission denied' })
              : success({ coords: { latitude: 14.1, longitude: 101, accuracy: 18 }, timestamp: Date.now() }), 20);
          }
        } });
      });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.route('**/api/v1/**', async route => {
        const pathname = new URL(route.request().url()).pathname;
        const reply = (status, body) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
        if (pathname.endsWith('/auth/google/config')) return reply(200, { enabled: false });
        if (pathname.includes('/auth/')) return reply(401, { error: { code: 'UNAUTHORIZED', message: 'Verification guest' } });
        if (pathname.endsWith('/events/stream')) return route.abort();
        if (pathname.endsWith('/weather/current')) return reply(503, { error: { code: 'UNAVAILABLE', message: 'Verification weather unavailable' } });
        return reply(200, { items: [], limit: 100, offset: 0 });
      });
      await page.goto('http://127.0.0.1:19195/map');
      const mainLocate = page.getByRole('button', { name: 'ตำแหน่งปัจจุบัน', exact: true });
      await mainLocate.waitFor();
      assert.equal(await page.evaluate(() => window.__gpsRequests), 0);
      if (width < 1200) await page.getByRole('button', { name: 'ย่อการ์ดสภาพอากาศ', exact: true }).click();
      await page.screenshot({ path: path.join(output, `main-initial-${width}.png`) });
      await mainLocate.click({ timeout: 5000 });
      await page.waitForFunction(() => document.querySelector('.map-location-control')?.getAttribute('aria-pressed') === 'true');
      await page.getByRole('button', { name: 'ซูมเข้า', exact: true }).click();
      await page.waitForFunction(() => document.querySelector('.leaflet-control-scale-line')?.textContent === '100 m');
      await page.getByRole('button', { name: 'ซูมเข้า', exact: true }).click();
      await page.waitForFunction(() => document.querySelector('.leaflet-control-scale-line')?.textContent === '50 m');
      assert.equal(await page.getByRole('button', { name: 'ซูมเข้า', exact: true }).isDisabled(), true);
      const originalColor = await mainLocate.evaluate(element => getComputedStyle(element).color);
      const map = page.locator('.situation-map .leaflet-container');
      const bounds = await map.boundingBox();
      await page.mouse.move(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2);
      await page.mouse.down();
      await page.mouse.move(bounds.x + bounds.width / 2 + 65, bounds.y + bounds.height / 2 + 30, { steps: 12 });
      await page.mouse.up();
      await page.waitForFunction(() => document.querySelector('.map-location-control')?.getAttribute('aria-pressed') === 'false');
      assert.notEqual(await mainLocate.evaluate(element => getComputedStyle(element).color), originalColor);
      await mainLocate.click();
      await page.waitForFunction(() => document.querySelector('.map-location-control')?.getAttribute('aria-pressed') === 'true');
      assert.equal(await page.evaluate(() => window.__gpsRequests), 1);
      await page.screenshot({ path: path.join(output, `main-${width}.png`) });
      const expandMain = page.locator('.map-tool-button--square');
      await expandMain.click();
      assert.equal(await page.locator('.map-page[role="dialog"]').count(), 1);
      await page.keyboard.press('Escape');
      assert.equal(await page.locator('.map-page[role="dialog"]').count(), 0);
      await page.locator('a[href="/reports"]:visible').first().click();
      await page.locator('a[href="/reports/new"]:visible').first().click();
      const reportLocate = page.getByRole('button', { name: 'ไปยังตำแหน่งของฉัน', exact: true });
      await reportLocate.scrollIntoViewIfNeeded();
      assert.equal(await page.locator('.leaflet-control-scale-line').textContent(), '50 m');
      assert.equal(await page.locator('.report-incident-pin').count(), 0);
      await reportLocate.click();
      assert.equal(await page.evaluate(() => window.__gpsRequests), 1);
      assert.equal(await page.locator('.report-incident-pin').count(), 0);
      await page.locator('.report-place-fields input').first().fill('UI verification draft');
      await page.getByRole('button', { name: 'เลือกจุดกึ่งกลางแผนที่', exact: true }).click();
      assert.equal(await page.locator('.report-incident-pin').count(), 1);
      const pinBefore = await page.locator('.report-incident-pin').getAttribute('style');
      await page.getByRole('button', { name: 'ขยายแผนที่', exact: true }).click();
      const dialog = page.getByRole('dialog', { name: 'ตำแหน่งเหตุการณ์', exact: true });
      await dialog.waitFor();
      await page.waitForTimeout(120);
      assert.equal(await page.evaluate(() => document.body.style.overflow), 'hidden');
      assert.equal(await page.locator('.leaflet-control-scale-line').textContent(), '50 m');
      const dialogBox = await dialog.boundingBox();
      assert.equal(Math.round(dialogBox.width), width);
      assert.equal(Math.round(dialogBox.height), height);
      const mapBox = await page.locator('.report-location-map').boundingBox();
      const leafletBox = await page.locator('.report-location-map .leaflet-container').boundingBox();
      console.log(JSON.stringify({ width, mapBox, leafletBox }));
      assert.ok(leafletBox.height >= mapBox.height - 2, 'Expanded Leaflet canvas fills its map surface');
      assert.equal(await reportLocate.getAttribute('aria-pressed'), 'true', 'Expanding keeps the user-centered camera');
      await page.getByRole('button', { name: 'เลือกจุดกึ่งกลางแผนที่', exact: true }).focus();
      await page.keyboard.press('Tab');
      await page.keyboard.press('Tab');
      assert.equal(await dialog.evaluate(element => element.contains(document.activeElement)), true);
      await page.screenshot({ path: path.join(output, `report-expanded-${width}.png`) });
      await page.keyboard.press('Escape');
      assert.equal(await page.getByRole('dialog').count(), 0);
      await page.waitForTimeout(120);
      assert.equal(await page.locator('.report-incident-pin').count(), 1);
      assert.equal(await page.locator('.report-place-fields input').first().inputValue(), 'UI verification draft');
      assert.equal(await page.locator('.report-incident-pin').getAttribute('style'), pinBefore);
      await page.getByRole('button', { name: 'ขยายแผนที่', exact: true }).click();
      await page.getByRole('button', { name: 'ย่อแผนที่', exact: true }).click();
      assert.equal(await page.getByRole('dialog').count(), 0);
      await page.locator('.report-location-map').screenshot({ path: path.join(output, `report-${width}.png`) });
      const scaleBox = await page.locator('.leaflet-control-scale-line').boundingBox();
      const attributionBox = await page.locator('.leaflet-control-attribution').boundingBox();
      console.log(JSON.stringify({ width, scaleBox, attributionBox }));
      assert.ok(scaleBox.y + scaleBox.height <= attributionBox.y, 'Scale does not overlap attribution');
      await page.getByRole('button', { name: 'ซูมออก', exact: true }).click();
      await page.waitForFunction(() => document.querySelector('.leaflet-control-scale-line')?.textContent === '100 m');
      await page.locator('a[href="/map"]:visible').first().click();
      await mainLocate.waitFor();
      assert.equal(await page.locator('.leaflet-control-scale-line').textContent(), '100 m', 'Report camera is restored on the main map');
      assert.equal(await page.evaluate(() => window.__gpsRequests), 1);
      await page.evaluate(() => { const originalNow = Date.now; Date.now = () => originalNow() + 61000; window.__locationFailure = true; });
      await mainLocate.click();
      await page.locator('.shared-map-error').waitFor();
      assert.equal(await mainLocate.getAttribute('aria-pressed'), 'false');
      assert.match(await page.locator('.shared-map-error').innerText(), /ไม่ได้รับอนุญาต/);
      await page.evaluate(() => { window.__locationFailure = false; });
      await mainLocate.click();
      await page.waitForFunction(() => document.querySelector('.map-location-control')?.getAttribute('aria-pressed') === 'true');
      assert.equal(await page.locator('.shared-map-error').count(), 0);
      assert.equal(await page.evaluate(() => window.__gpsRequests), 3);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
      assert.deepEqual(errors, []);
      findings.push({ width, height, scale: '50 m', bidirectionalCamera: true, cachedGPS: true, permissionErrorAndRetry: true, pinAndDraftPreserved: true, expandedKeyboardContained: true, pageErrors: errors.length });
      await context.close();
    }
    console.log(JSON.stringify({ findings, screenshotDirectory: output }, null, 2));
  } finally {
    await browser.close();
    await new Promise(resolve => server.close(resolve));
  }
}

run().catch(error => { console.error(error); process.exitCode = 1; server.close(); });
