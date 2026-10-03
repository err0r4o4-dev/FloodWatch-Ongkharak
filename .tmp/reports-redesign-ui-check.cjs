const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');
const root = path.resolve('.tmp/reports-redesign-dist');
const output = path.resolve('.tmp/reports-redesign-ui');
fs.mkdirSync(output, { recursive: true });
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.jpg': 'image/jpeg', '.woff2': 'font/woff2' };
const server = http.createServer((request, response) => {
  const target = path.resolve(root, '.' + new URL(request.url, 'http://localhost').pathname);
  const safe = target.startsWith(root + path.sep);
  const file = safe && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});
const timestamp = '2026-10-04T00:00:00Z';
const reports = ['FLOOD', 'ROAD', 'OTHER', 'FLOOD'].map((type, index) => ({
  id: `test-${index}`, type, description: ['น้ำท่วมขังบริเวณหน้าตลาด · ข้อมูลทดสอบเท่านั้น', 'ถนนมีน้ำขัง โปรดตรวจสอบสภาพก่อนเดินทาง · ข้อมูลทดสอบ', 'แจ้งข้อมูลจากชุมชน · ไม่ใช่สถานการณ์จริง', 'รายงานรายละเอียดที่ยาวสำหรับตรวจสอบการตัดข้อความและการแสดงผลบนหน้าจอขนาดเล็กโดยยังอ่านรายละเอียดฉบับเต็มได้ในแผงที่เลือก'][index],
  location: { type: 'Point', coordinates: [101.1, 14.1] }, estimatedWaterDepthM: type === 'FLOOD' ? 0.25 : null,
  status: 'VERIFIED', verificationStatus: 'VERIFIED', sourceType: 'COMMUNITY', createdAt: timestamp, updatedAt: timestamp, verifiedAt: timestamp,
  media: index % 2 === 0 ? [{ id: `media-${index}`, url: `/api/v1/reports/test-${index}/media/media-${index}` }] : []
}));
(async () => {
  await new Promise((resolve) => server.listen(19186, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const errors = [];
  const checks = [];
  let mode = 'ready';
  let authenticated = false;
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1100 }, reducedMotion: 'reduce' });
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('https://**', (route) => route.abort());
    await page.route('**/api/v1/**', async (route) => {
      const pathname = new URL(route.request().url()).pathname;
      const reply = (status, body) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
      if (pathname.includes('/auth/')) {
        if (!authenticated) return reply(401, { error: { code: 'UNAUTHORIZED', message: 'Guest fixture' } });
        const user = { id: 'test-user', email: 'test@example.com', role: 'USER', createdAt: timestamp };
        return reply(200, pathname.endsWith('/me') ? { user } : { user, accessToken: 'synthetic-only-token', expiresIn: 900 });
      }
      if (pathname.endsWith('/events/stream')) return route.abort();
      if (pathname.includes('/media/')) {
        if (mode === 'media-error') return reply(503, { error: { code: 'UNAVAILABLE' } });
        return route.fulfill({ status: 200, contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="500" height="300"><rect width="500" height="300" fill="#dcebff"/><path d="M0 190Q80 140 160 190T320 190T500 190V300H0Z" fill="#8cc6f5"/><text x="120" y="130" fill="#174382" font-size="26">TEST IMAGE ONLY</text></svg>' });
      }
      if (pathname.endsWith('/reports')) {
        if (mode === 'loading') await new Promise((resolve) => setTimeout(resolve, 1600));
        if (mode === 'error') return reply(503, { error: { code: 'UNAVAILABLE', message: 'Test failure' } });
        return reply(200, { items: mode === 'empty' ? [] : reports, limit: 20, offset: 0 });
      }
      return reply(200, { items: [], limit: 20, offset: 0 });
    });
    const goto = async () => { await page.goto('http://127.0.0.1:19186/reports'); await page.getByRole('heading', { name: 'รายงานสถานการณ์', exact: true }).waitFor(); };
    for (const width of [1440, 1024, 768, 390, 320]) {
      await page.setViewportSize({ width, height: 1100 });
      await goto();
      await page.getByRole('button', { name: `ดูรายละเอียด: น้ำท่วม · ${reports[0].description}`, exact: true }).waitFor();
      await page.evaluate(() => document.fonts.ready);
      const metrics = await page.evaluate(() => ({ width: innerWidth, scrollWidth: document.documentElement.scrollWidth, targets: [...document.querySelectorAll('.reports-filters button, .report-card__detail-button, .reports-create-button')].map((element) => element.getBoundingClientRect().height) }));
      assert(metrics.scrollWidth <= width, `overflow at ${width}: ${metrics.scrollWidth}`);
      assert(metrics.targets.every((height) => height >= 44), `small target at ${width}`);
      await page.screenshot({ path: path.join(output, `reports-${width}.png`), fullPage: true });
      checks.push({ width, ...metrics });
    }
    await page.setViewportSize({ width: 1440, height: 1100 });
    await goto();
    const roadAction = page.getByRole('button', { name: `ดูรายละเอียด: สภาพถนน · ${reports[1].description}` });
    await roadAction.focus();
    await roadAction.press('Enter');
    assert.equal(await roadAction.getAttribute('aria-pressed'), 'true');
    await page.getByRole('heading', { name: reports[1].description, exact: true }).waitFor();
    await page.getByRole('button', { name: 'สภาพถนน', exact: true }).click();
    assert.equal(await page.locator('.reports-list .report-card').count(), 1);
    await page.getByRole('searchbox', { name: 'ค้นหารายงาน' }).fill('ไม่มีคำนี้');
    await page.getByRole('heading', { name: 'ไม่พบรายงานที่ตรงกับตัวกรอง' }).waitFor();
    await page.getByRole('button', { name: 'ล้างตัวกรอง' }).click();
    assert.equal(await page.locator('.reports-list .report-card').count(), 4);
    checks.push({ keyboardSelection: true, filtering: true, clearFilters: true });
    await page.setViewportSize({ width: 390, height: 1100 });
    const mobileAction = page.getByRole('button', { name: `ดูรายละเอียด: สภาพถนน · ${reports[1].description}` });
    await mobileAction.click();
    const mobileHeading = page.getByRole('heading', { name: reports[1].description, exact: true });
    await mobileHeading.waitFor();
    await page.waitForFunction(() => document.activeElement?.id === 'report-detail-heading');
    const detailBounds = await mobileHeading.boundingBox();
    assert(detailBounds.y >= 0 && detailBounds.y < 1100, 'mobile detail not brought into view');
    await page.screenshot({ path: path.join(output, 'reports-mobile-selected.png') });
    await goto();
    await page.getByRole('button', { name: /ดูรายละเอียด: น้ำท่วม/ }).first().waitFor();
    await page.evaluate(() => {
      const elements = [...document.querySelectorAll('.reports-page, .reports-page *')];
      const sizes = elements.map((element) => parseFloat(getComputedStyle(element).fontSize));
      elements.forEach((element, index) => { element.style.fontSize = `${sizes[index] * 2}px`; });
    });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'reports 200% text overflow');
    await page.screenshot({ path: path.join(output, 'reports-200-percent-text.png'), fullPage: true });
    checks.push({ mobileDetailFocus: true, scoped200PercentText: true });
    await page.setViewportSize({ width: 1440, height: 1100 });
    for (const state of ['loading', 'empty', 'error', 'media-error']) {
      mode = state;
      await goto();
      if (state === 'loading') await page.getByText('กำลังโหลดจำนวนรายงานที่ยืนยันแล้ว').waitFor();
      if (state === 'empty') await page.getByRole('heading', { name: 'ยังไม่มีรายงานที่เผยแพร่' }).waitFor();
      if (state === 'error') await page.getByRole('alert').filter({ hasText: 'ไม่สามารถโหลดรายงานได้' }).waitFor();
      if (state === 'media-error') await page.getByRole('button', { name: /ดูรายละเอียด: น้ำท่วม/ }).first().waitFor();
      await page.screenshot({ path: path.join(output, `reports-${state}.png`), fullPage: true });
      if (state === 'error') {
        assert.equal(await page.locator('.reports-summary dd').first().innerText(), '— รายงาน');
        mode = 'ready';
        await page.getByRole('button', { name: 'โหลดรายงานอีกครั้ง' }).click();
        await page.getByRole('heading', { name: reports[0].description, exact: true }).waitFor();
      }
      checks.push({ state, checked: true });
    }
    mode = 'ready';
    await page.goto('http://127.0.0.1:19186/reports?report=missing');
    await page.getByRole('heading', { name: 'รายงานที่เลือกไม่พร้อมใช้งาน' }).waitFor();
    assert.equal(await page.locator('.reports-list [aria-pressed="true"]').count(), 0);
    authenticated = true;
    await goto();
    await page.getByRole('heading', { name: 'ส่งรายงานชุมชน', exact: true }).waitFor();
    await page.getByRole('link', { name: 'ส่งรายงานสถานการณ์', exact: true }).click();
    await page.waitForFunction(() => document.activeElement?.id === 'submit-report');
    await page.screenshot({ path: path.join(output, 'reports-authenticated-1440.png'), fullPage: true });
    await page.setViewportSize({ width: 390, height: 1100 });
    await page.screenshot({ path: path.join(output, 'reports-authenticated-390.png'), fullPage: true });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'authenticated overflow');
    checks.push({ authenticated: true, createAnchor: true, missingDeepLink: true });
    assert.deepEqual(errors, []);
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ checks, errors }, null, 2));
    console.log(JSON.stringify({ checks, errors }, null, 2));
  } finally { await browser.close(); await new Promise((resolve) => server.close(resolve)); }
})().catch((error) => { console.error(error); process.exitCode = 1; });
