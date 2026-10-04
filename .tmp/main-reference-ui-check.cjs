const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('.tmp/main-reference-dist');
const output = path.resolve('.tmp/main-reference-ui');
const origin = 'http://127.0.0.1:19181';
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2', '.jpg': 'image/jpeg' };
fs.mkdirSync(output, { recursive: true });
const server = http.createServer((request, response) => {
  const target = path.resolve(root, '.' + new URL(request.url, origin).pathname);
  const safe = target === root || target.startsWith(root + path.sep);
  const file = safe && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});

const source = { id: 'SYNTHETIC_TEST_ONLY', code: 'SYNTHETIC_TEST_ONLY', name: 'ชุดทดสอบหน้าจอ · ไม่ใช่ข้อมูลจริง', sourceType: 'OFFICIAL' };
const observedAt = '2026-10-04T00:00:00Z';
const collections = {
  '/api/v1/water-stations': [{ id: 'test-water', name: 'สถานีสังเคราะห์สำหรับตรวจ UI', location: { type: 'Point', coordinates: [100.97, 14.07] }, source, active: true }],
  '/api/v1/flood-events': [{ id: 'test-flood', title: 'เหตุการณ์สังเคราะห์สำหรับตรวจ UI', description: 'ข้อมูลทดสอบเท่านั้น ไม่ใช่เหตุการณ์จริง', geometry: { type: 'Point', coordinates: [100.99, 14.11] }, source, severity: 'WARNING', effectiveSeverity: 'WARNING', status: 'ACTIVE', effectiveStatus: 'ACTIVE', observedAt, freshness: 'FRESH', verificationStatus: 'VERIFIED', confidence: null }],
  '/api/v1/risk-areas': [{ id: 'test-risk', name: 'พื้นที่สังเคราะห์สำหรับตรวจ UI', description: 'ข้อมูลทดสอบเท่านั้น', geometry: { type: 'MultiPolygon', coordinates: [[[[101.025, 14.09], [101.04, 14.09], [101.04, 14.105], [101.025, 14.105], [101.025, 14.09]]]] }, source, riskLevel: 'MODERATE', effectiveRiskLevel: 'MODERATE', freshness: 'FRESH', validFrom: observedAt, validTo: null }],
  '/api/v1/reports': [{ id: 'test-report', type: 'FLOOD', description: 'รายงานชุมชนสังเคราะห์สำหรับตรวจ UI', location: { type: 'Point', coordinates: [101.045, 14.06] }, estimatedWaterDepthM: null, status: 'VERIFIED', verificationStatus: 'VERIFIED', sourceType: 'COMMUNITY', createdAt: observedAt, updatedAt: observedAt, verifiedAt: observedAt, verifiedBy: null, media: [] }],
  '/api/v1/roads': []
};

(async () => {
  await new Promise((resolve) => server.listen(19181, '127.0.0.1', resolve));
  let browser;
  const checks = [];
  let mode = 'ready';
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
    await context.route('**/*', async (route) => {
      const url = new URL(route.request().url());
      if (url.hostname === 'tile.openstreetmap.org') return route.continue();
      if (url.origin !== origin) return route.abort();
      if (url.pathname.startsWith('/api/v1/')) {
        if (url.pathname.startsWith('/api/v1/auth/')) return route.fulfill({ status: 401, json: { error: { code: 'UNAUTHORIZED', message: 'Isolated guest verification' } } });
        if (url.pathname.includes('/events')) return route.abort();
        if (mode === 'loading') await new Promise((resolve) => setTimeout(resolve, 1000));
        if (mode === 'error' || (mode === 'partial' && url.pathname.includes('/risk-areas'))) return route.fulfill({ status: 503, json: { error: { code: 'UNAVAILABLE', message: 'Isolated UI failure test' } } });
        return route.fulfill({ json: { items: mode === 'empty' ? [] : collections[url.pathname] || [], limit: 100, offset: 0 } });
      }
      return route.continue();
    });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    async function openMain() {
      await page.goto(origin);
      await page.getByRole('heading', { level: 1, name: 'ติดตามสถานการณ์น้ำ เพื่อความปลอดภัยของทุกคน' }).waitFor();
      await page.evaluate(() => document.fonts.ready);
      if (mode !== 'loading') await page.getByText('กำลังโหลดข้อมูลแผนที่', { exact: true }).waitFor({ state: 'hidden' });
    }
    async function noOverflow(label) {
      const dimensions = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, document: document.documentElement.scrollWidth }));
      assert(dimensions.document <= dimensions.viewport, `${label}: ${JSON.stringify(dimensions)}`);
      return dimensions;
    }
    for (const width of [320, 390, 768, 1024, 1440, 1672]) {
      await page.setViewportSize({ width, height: 1000 });
      await openMain();
      await page.waitForTimeout(500);
      const dimensions = await noOverflow(`Main ${width}`);
      assert.equal(await page.getByRole('navigation', { name: 'การนำทางมือถือ' }).count(), 0);
      assert.equal(await page.getByRole('link', { name: /เลื่อนลง|รู้จักระบบ/ }).count(), 0);
      assert.equal(await page.locator('.landing-goal').count(), 4);
      if (width >= 1440) {
        const title = await page.locator('#landing-title span').evaluate((span) => ({ height: span.getBoundingClientRect().height, lineHeight: parseFloat(getComputedStyle(span).lineHeight) }));
        assert(title.height <= title.lineHeight + 1, 'Desktop blue title must fit on one line');
      }
      assert(await page.getByRole('button', { name: 'EN', exact: true }).isDisabled());
      const short = await page.locator('.landing-page a:not(.skip-link):not(.leaflet-control-attribution a), .landing-page button, .landing-page summary').evaluateAll((controls) => controls.filter((control) => control.getBoundingClientRect().height > 0 && control.getBoundingClientRect().height < 43.9).map((control) => control.textContent));
      assert.deepEqual(short, [], `Touch targets ${width}`);
      await page.screenshot({ path: path.join(output, `main-${width}.png`), fullPage: true });
      checks.push({ width, ...dimensions, touchTargets: 'pass' });
    }
    const loadedTiles = await page.locator('.leaflet-tile').evaluateAll((tiles) => tiles.filter((tile) => tile.complete && tile.naturalWidth > 0).length);
    checks.push({ loadedOpenStreetMapTiles: loadedTiles });
    await page.setViewportSize({ width: 1440, height: 1000 });
    for (const state of ['loading', 'empty', 'error', 'partial']) {
      mode = state;
      await openMain();
      if (state === 'loading') await page.getByText('กำลังโหลดข้อมูลแผนที่', { exact: true }).waitFor();
      if (state === 'empty') await page.getByText('ยังไม่มีข้อมูลสถานการณ์ในขอบเขตนี้ · ไม่ทราบสถานะ', { exact: true }).waitFor();
      if (state === 'error') await page.getByRole('alert').waitFor();
      if (state === 'partial') await page.getByText('ข้อมูลบางชั้นไม่พร้อมใช้งาน · ไม่สามารถสรุปว่าปลอดภัย', { exact: true }).waitFor();
      const obscured = await page.locator('.landing-map .map-zoom-controls button').evaluateAll((buttons) => buttons.filter((button) => {
        const rect = button.getBoundingClientRect();
        if (!rect.height) return false;
        const top = document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2);
        return !top || !button.contains(top);
      }).map((button) => button.getAttribute('aria-label')));
      assert.deepEqual(obscured, [], `Map controls obscured in ${state}`);
      await noOverflow(state);
      await page.screenshot({ path: path.join(output, `main-${state}.png`), fullPage: true });
    }
    mode = 'ready';
    await page.getByRole('button', { name: 'ขยายแผนที่', exact: true }).click();
    await page.getByRole('button', { name: 'ย่อแผนที่', exact: true }).click();
    await page.getByRole('button', { name: 'กลับมุมมองเริ่มต้น', exact: true }).click();
    await openMain();
    const marker = page.getByTitle('เหตุการณ์สังเคราะห์สำหรับตรวจ UI', { exact: true });
    await marker.focus();
    await page.keyboard.press('Enter');
    const markerDetails = page.getByRole('region', { name: 'ข้อมูลตำแหน่งที่เลือก' });
    await markerDetails.waitFor();
    await page.waitForTimeout(700);
    assert(await markerDetails.isVisible(), 'Marker details must persist without an auto-pan/refetch loop');
    await page.screenshot({ path: path.join(output, 'main-marker-details.png'), fullPage: true });
    await markerDetails.getByRole('button', { name: 'ปิดข้อมูลตำแหน่ง' }).click();
    await page.locator('summary').click();
    await page.locator('.landing-map__evidence').getByRole('button', { name: 'เหตุการณ์สังเคราะห์สำหรับตรวจ UI', exact: true }).click();
    const details = page.getByRole('region', { name: 'ข้อมูลตำแหน่งที่เลือก' });
    await details.waitFor();
    assert(await details.getByRole('button', { name: 'ปิดข้อมูลตำแหน่ง' }).evaluate((button) => document.activeElement === button));
    await page.screenshot({ path: path.join(output, 'main-record-details.png'), fullPage: true });
    await page.keyboard.press('Escape');
    await details.waitFor({ state: 'hidden' });
    await openMain();
    await page.getByRole('link', { name: 'เข้าสู่ระบบ', exact: true }).first().click();
    await page.getByRole('heading', { level: 1, name: 'เข้าสู่ระบบ' }).waitFor();
    assert.equal(new URL(page.url()).searchParams.get('returnTo'), '/overview');
    await page.setViewportSize({ width: 390, height: 844 });
    await openMain();
    await page.keyboard.press('Tab');
    assert.equal(await page.locator(':focus').textContent(), 'ข้ามไปยังเนื้อหาหลัก');
    await page.keyboard.press('Enter');
    assert.equal(await page.locator(':focus').getAttribute('id'), 'main-content');
    await page.keyboard.press('Tab');
    assert.equal((await page.locator(':focus').textContent()).trim(), 'เข้าสู่ระบบ →');
    await page.getByRole('link', { name: 'ดูภาพรวมโดยไม่เข้าสู่ระบบ' }).click();
    await page.getByRole('heading', { level: 1, name: 'ภาพรวมสถานการณ์น้ำ' }).waitFor();
    assert.equal(new URL(page.url()).pathname, '/overview');
    await page.getByRole('link', { name: 'Flood Watch Ongkharak หน้าหลัก' }).filter({ visible: true }).first().click();
    await page.getByRole('link', { name: 'ดูแผนที่', exact: true }).click();
    await page.getByRole('heading', { level: 1, name: 'แผนที่สถานการณ์น้ำ' }).waitFor();
    assert.equal(new URL(page.url()).pathname, '/map');
    await openMain();
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    const zoom = await noOverflow('Root text 200%');
    await page.screenshot({ path: path.join(output, 'main-390-200-percent.png'), fullPage: true });
    assert.deepEqual(errors, []);
    checks.push({ states: ['loading', 'empty', 'error', 'partial'], keyboard: 'pass', recordDetails: 'pass', routes: 'pass', rootText200Percent: zoom, pageErrors: errors });
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify(checks, null, 2));
    console.log(JSON.stringify(checks, null, 2));
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
