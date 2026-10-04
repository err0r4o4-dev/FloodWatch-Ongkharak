const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');
const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/report-create-ui');
let origin = 'http://127.0.0.1';
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
const server = http.createServer((request, response) => {
  const target = path.resolve(root, '.' + decodeURIComponent(new URL(request.url, origin).pathname));
  const file = target.startsWith(root + path.sep) && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', types[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});
const fixture = { id: '00000000-0000-4000-8000-000000000001', displayName: 'Synthetic reporter', type: 'FLOOD', description: 'Synthetic report — UI testing only', locationName: 'อาคารทดสอบ (ข้อมูลจำลอง)', locationNote: 'หน้าอาคาร ฝั่งประตู', location: { type: 'Point', coordinates: [101.1, 14.2] }, estimatedWaterDepthM: 0.3, status: 'VERIFIED', verificationStatus: 'VERIFIED', sourceType: 'COMMUNITY', createdAt: '2026-10-04T00:00:00Z', updatedAt: '2026-10-04T01:00:00Z', verifiedAt: '2026-10-04T01:00:00Z', media: [] };
(async () => {
  fs.mkdirSync(output, { recursive: true });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  origin = 'http://127.0.0.1:' + server.address().port;
  let browser;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const context = await browser.newContext({ reducedMotion: 'reduce', permissions: ['geolocation'], geolocation: { latitude: 14.2, longitude: 101.1, accuracy: 25 } });
    let submitted = null;
    const errors = [];
    const page = await context.newPage();
    page.on('pageerror', (error) => errors.push(error.message));
    await context.route('https://tile.openstreetmap.org/**', (route) => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256"><rect width="256" height="256" fill="#edf2f4"/><path d="M0 128H256 M128 0V256" stroke="#b6cbd4" stroke-width="10"/><text x="16" y="30" font-size="12" fill="#536b78">Synthetic map tile — UI test</text></svg>' }));
    await context.route('**/api/v1/**', (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith('/auth/refresh')) return route.fulfill({ status: 401, json: { error: { code: 'UNAUTHENTICATED' } } });
      if (url.pathname.endsWith('/events/stream')) return route.fulfill({ contentType: 'text/event-stream', body: 'event: connected\ndata: {}\n\n' });
      if (url.pathname.endsWith('/reports') && route.request().method() === 'POST') {
        submitted = route.request().postDataJSON();
        return route.fulfill({ status: 201, json: { report: { ...fixture, ...submitted, status: 'PENDING', verificationStatus: 'PENDING', verifiedAt: null }, uploadToken: 'synthetic-private-capability' } });
      }
      if (url.pathname.endsWith('/reports')) return route.fulfill({ json: { items: [fixture], limit: 20, offset: 0 } });
      return route.fulfill({ json: { items: [], pagination: { total: 0, limit: 10, offset: 0 }, limit: 100, offset: 0 } });
    });
    for (const width of [320, 390, 768, 1440]) {
      await page.setViewportSize({ width, height: 980 });
      await page.goto(origin + '/reports');
      await page.getByRole('link', { name: 'ส่งรายงานสถานการณ์' }).click();
      await page.getByRole('heading', { name: 'เพิ่มรายงานสถานการณ์' }).waitFor();
      assert.equal(new URL(page.url()).pathname, '/reports/new');
      assert.equal(await page.getByLabel('ลองจิจูด').count(), 0);
      assert.equal(await page.getByLabel('ละติจูด').count(), 0);
      assert.equal(await page.locator('.report-location-map .leaflet-interactive').count(), 0);
      await page.screenshot({ path: path.join(output, `initial-${width}.png`), fullPage: true });
      const locate = page.getByRole('button', { name: 'ไปยังตำแหน่งของฉัน', exact: true });
      await locate.click();
      await page.getByText('ตำแหน่งของคุณอาจคลาดเคลื่อนประมาณ 25 เมตร', { exact: true }).waitFor();
      assert.match(await page.locator('.report-location-picker__status').innerText(), /ยังไม่ได้เลือก/);
      await page.getByRole('button', { name: 'ใช้ตำแหน่งของฉันเป็นจุดเกิดเหตุ', exact: true }).click();
      await page.getByText('เลือกจุดเกิดเหตุแล้ว', { exact: true }).waitFor();
      assert(await page.locator('.report-incident-pin svg').isVisible());
      assert.equal(await page.locator('.report-incident-pin').getAttribute('title'), 'จุดที่รายงาน');
      const map = page.locator('.report-location-map');
      const bounds = await map.boundingBox();
      await map.click({ position: { x: bounds.width * 0.7, y: bounds.height * 0.35 } });
      await page.getByText(/เปลี่ยนจุดเกิดเหตุแล้ว/).waitFor();
      await locate.click();
      await page.getByText('ตำแหน่งของคุณอาจคลาดเคลื่อนประมาณ 25 เมตร', { exact: true }).waitFor();
      await page.getByLabel('ชื่อสถานที่ / ชื่อซอย *').fill('  อาคารทดสอบ (ข้อมูลจำลอง)  ');
      await page.getByLabel('จุดเกิดเหตุ / จุดสังเกต (ไม่บังคับ)').fill('  หน้าอาคาร ฝั่งประตู  ');
      await page.getByRole('radio', { name: '30 ซม.', exact: true }).check();
      await page.getByLabel('รายละเอียด', { exact: true }).fill('Synthetic incident — no live flood data');
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
      assert((await locate.boundingBox()).height >= 44);
      for (const option of await page.locator('.report-depth-option').all()) assert((await option.boundingBox()).height >= 44);
      for (const zoomControl of await page.locator('.leaflet-control-zoom a').all()) {
        const control = await zoomControl.boundingBox();
        assert(control.height >= 44 && control.width >= 44);
      }
      await page.screenshot({ path: path.join(output, `form-${width}.png`), fullPage: true });
      await page.getByRole('heading', { name: 'เพิ่มรายงานสถานการณ์', exact: true }).scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(output, `selected-map-${width}.png`), fullPage: true });
      await page.getByRole('button', { name: 'ส่งรายงาน', exact: true }).click();
      await page.locator('.form-success').waitFor();
      assert.equal(submitted.locationName, 'อาคารทดสอบ (ข้อมูลจำลอง)');
      assert.equal(submitted.locationNote, 'หน้าอาคาร ฝั่งประตู');
      assert.equal(submitted.estimatedWaterDepthM, 0.3);
      assert.notDeepEqual(submitted.location.coordinates, [101.1, 14.2]);
      assert(!('userPosition' in submitted));
      await page.getByRole('button', { name: 'รับทราบ', exact: true }).click();
      await page.getByRole('link', { name: 'กลับไปหน้ารายงานสถานการณ์' }).click();
      await page.getByText('อาคารทดสอบ (ข้อมูลจำลอง)', { exact: true }).waitFor();
      assert.equal(await page.getByRole('button', { name: 'ส่งรายงาน', exact: true }).count(), 0);
    }
    await page.goto(origin + '/reports/new');
    await page.getByRole('button', { name: 'เลือกจุดกึ่งกลางแผนที่', exact: true }).focus();
    await page.keyboard.press('Enter');
    await page.getByText('เลือกจุดเกิดเหตุแล้ว', { exact: true }).waitFor();
    await page.getByRole('radio', { name: 'ไม่ระบุ', exact: true }).focus();
    await page.keyboard.press('ArrowRight');
    assert(await page.getByRole('radio', { name: '10 ซม.', exact: true }).isChecked());
    await page.getByRole('radio', { name: 'ระบุค่าอื่น', exact: true }).check();
    await page.getByLabel('ระดับน้ำ (เซนติเมตร)').fill('75');
    await page.getByLabel('ชื่อสถานที่ / ชื่อซอย *').fill('ข้อมูลจำลอง');
    await page.getByLabel('รายละเอียด', { exact: true }).fill('Synthetic custom depth');
    await page.getByRole('button', { name: 'ส่งรายงาน', exact: true }).click();
    await page.locator('.form-success').waitFor();
    assert.equal(submitted.estimatedWaterDepthM, 0.75);
    await page.getByRole('button', { name: 'รับทราบ', exact: true }).click();
    await page.goto(origin + '/reports/new');
    await page.getByLabel('ชื่อสถานที่ / ชื่อซอย *').fill('เก็บข้อความเมื่อค้นหาตำแหน่งไม่ได้');
    await page.evaluate(() => Object.defineProperty(navigator, 'geolocation', { configurable: true, value: { getCurrentPosition: (_success, fail) => setTimeout(() => fail({ code: 1 }), 100) } }));
    await page.getByRole('button', { name: 'ไปยังตำแหน่งของฉัน', exact: true }).click();
    await page.getByText(/ไม่ได้รับอนุญาตให้ใช้ตำแหน่ง/).waitFor();
    assert.equal(await page.getByLabel('ชื่อสถานที่ / ชื่อซอย *').inputValue(), 'เก็บข้อความเมื่อค้นหาตำแหน่งไม่ได้');
    await page.locator('.report-location-picker__error').scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(output, 'permission-error.png'), fullPage: true });
    await page.getByRole('button', { name: 'เลือกจุดกึ่งกลางแผนที่', exact: true }).click();
    await page.getByText('เลือกจุดเกิดเหตุแล้ว', { exact: true }).waitFor();
    await page.setViewportSize({ width: 390, height: 980 });
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    await page.screenshot({ path: path.join(output, 'manual-fallback-zoom.png'), fullPage: true });
    assert(await page.locator('.create-report-page').evaluate((element) => element.getBoundingClientRect().right <= window.innerWidth));
    const zoomLayout = await page.evaluate(() => ({ viewport: window.innerWidth, scroll: document.documentElement.scrollWidth, header: document.querySelector('.institution-header__actions').getBoundingClientRect().right }));
    if (zoomLayout.scroll > zoomLayout.viewport) {
      await page.goto(origin + '/reports');
      await page.getByRole('link', { name: 'ส่งรายงานสถานการณ์' }).waitFor();
      await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
      await page.screenshot({ path: path.join(output, 'existing-shell-zoom.png'), fullPage: true });
      const baseline = await page.evaluate(() => ({ viewport: window.innerWidth, scroll: document.documentElement.scrollWidth, header: document.querySelector('.institution-header__actions').getBoundingClientRect().right }));
      assert(baseline.scroll > baseline.viewport);
      console.warn('KNOWN GLOBAL GAP: unchanged institutional header overflows at 200% text on both creation and existing reports pages.', { creation: zoomLayout, existingReports: baseline });
      process.exitCode = 2;
    }
    assert.deepEqual(errors, []);
    console.log('PASS scoped UI: new route/back, ordered form, locate-only without moving incident, explicit current location, map pin/change warning, metadata persistence payload, preset/custom depth, PENDING notice, keyboard centre/radios, permission denial fallback, 320/390/768/1440 widths and creation form fitting 200% mobile text; no page errors. Global zoom gap is reported separately. All API data, map tiles and positions are synthetic fixtures; no live database writes.');
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
