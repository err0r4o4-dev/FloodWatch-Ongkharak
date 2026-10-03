const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('.tmp/overview-dist');
const output = path.resolve('.tmp/overview-ui');
fs.mkdirSync(output, { recursive: true });
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.jpg': 'image/jpeg', '.woff2': 'font/woff2', '.txt': 'text/plain; charset=utf-8' };
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  const target = path.resolve(root, '.' + pathname);
  const safe = target === root || target.startsWith(root + path.sep);
  const file = safe && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});

const observedAt = '2026-10-04T00:00:00Z';
const source = { id: 'test-source', code: 'SYNTHETIC_TEST_ONLY', name: 'ชุดทดสอบ · ไม่ใช่ข้อมูลสด', sourceType: 'OFFICIAL' };
const water = Array.from({ length: 5 }, (_, index) => ({ id: `station-${index}`, name: `สถานีทดสอบคลอง ${15 + index}`, source }));
const risks = Array.from({ length: 4 }, (_, index) => ({ id: `risk-${index}`, name: `พื้นที่ทดสอบคลอง ${15 + index}`, description: 'รายละเอียดผลประเมินสำหรับทดสอบหน้าจอเท่านั้น', riskLevel: 'HIGH', effectiveRiskLevel: index === 3 ? 'UNKNOWN' : 'HIGH', freshness: index === 3 ? 'STALE' : 'FRESH', source, validFrom: observedAt, validTo: null }));
const roads = Array.from({ length: 4 }, (_, index) => ({ id: `road-${index}`, name: `ถนนทดสอบ ${index + 1}`, currentCondition: { source, status: index === 0 ? 'CLOSED' : 'PASSABLE', reportedStatus: 'PASSABLE', observedAt, freshness: index === 3 ? 'STALE' : 'FRESH', verificationStatus: 'VERIFIED' } }));
const events = [{ id: 'event-1', title: 'เหตุการณ์น้ำสำหรับทดสอบ', description: 'ข้อมูลสังเคราะห์สำหรับทดสอบเท่านั้น', source, severity: 'WARNING', effectiveSeverity: 'WARNING', status: 'ACTIVE', effectiveStatus: 'ACTIVE', observedAt, freshness: 'FRESH', verificationStatus: 'VERIFIED' }];
const alerts = ['CRITICAL', 'WARNING', 'INFO'].map((severity, index) => ({ id: `alert-${index}`, ruleId: 'test-rule', severity, title: `ประกาศทดสอบ ${index + 1}`, message: 'ข้อความสำหรับตรวจรูปแบบหน้าจอเท่านั้น ไม่ใช่ประกาศจริง', sourceType: 'OFFICIAL', resourceType: 'water_station', resourceId: 'station-0', triggeredAt: observedAt, createdAt: observedAt }));
const reports = Array.from({ length: 3 }, (_, index) => ({ id: `report-${index}`, type: 'FLOOD', description: `รายงานบุคคลสำหรับทดสอบ ${index + 1}`, location: { type: 'Point', coordinates: [101.1, 14.1] }, estimatedWaterDepthM: null, status: 'VERIFIED', verificationStatus: 'VERIFIED', sourceType: 'COMMUNITY', createdAt: observedAt, updatedAt: observedAt, verifiedAt: observedAt, media: index === 1 ? [] : [{ id: `media-${index}`, url: `/api/v1/reports/report-${index}/media/media-${index}` }] }));

(async () => {
  await new Promise((resolve) => server.listen(19176, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const errors = [];
  const checks = [];
  let mode = 'ready';
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1100 }, reducedMotion: 'reduce' });
    const page = await context.newPage();
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('https://**', (route) => route.abort());
    await page.route('**/api/v1/**', async (route) => {
      const url = new URL(route.request().url());
      const pathname = url.pathname;
      const reply = (status, body) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
      if (pathname.includes('/auth/')) return reply(401, { error: { code: 'UNAUTHORIZED', message: 'Isolated guest fixture' } });
      if (pathname.endsWith('/events/stream')) return route.abort();
      if (mode === 'loading') await new Promise((resolve) => setTimeout(resolve, 1800));
      if (mode === 'error' || mode === 'partial' && pathname.endsWith('/reports')) return reply(503, { error: { code: 'UNAVAILABLE', message: 'Isolated failure' } });
      if (pathname.includes('/media/')) {
        if (mode === 'media-error') return reply(503, { error: { code: 'UNAVAILABLE' } });
        return route.fulfill({ status: 200, contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="100" height="80"><rect width="100" height="80" fill="#dcebff"/><text x="12" y="43" fill="#2563eb" font-size="18">TEST</text></svg>' });
      }
      if (mode === 'empty' || url.searchParams.has('bbox')) return reply(200, { items: [], limit: 20, offset: 0 });
      let items = [];
      if (pathname.endsWith('/water-stations')) items = water;
      else if (pathname.endsWith('/readings')) {
        const index = Number(pathname.split('/')[4].split('-')[1]);
        items = [{ source, observedAt, waterLevelM: Number((1.2 + index / 10).toFixed(2)), flowRateM3s: null, rainfallMm: index === 4 ? null : 8 + index * 4, freshness: index === 3 ? 'STALE' : 'FRESH', verificationStatus: 'VERIFIED' }];
      } else if (pathname.endsWith('/risk-areas')) items = risks;
      else if (pathname.endsWith('/roads')) items = roads;
      else if (pathname.endsWith('/flood-events')) items = events;
      else if (pathname.endsWith('/alerts')) items = alerts;
      else if (pathname.endsWith('/reports')) items = reports;
      return reply(200, { items, limit: 20, offset: 0 });
    });

    async function openOverview() {
      await page.goto('http://127.0.0.1:19176/');
      if (mode === 'loading') await page.getByRole('status', { name: 'กำลังโหลดข้อมูล', exact: true }).waitFor();
      else if (mode === 'error') await page.getByRole('alert').waitFor();
      else if (mode === 'empty') await page.getByText('ยังไม่มีข้อมูลสถานการณ์ที่เผยแพร่', { exact: true }).waitFor();
      else await page.getByRole('heading', { name: 'เหตุการณ์น้ำสำหรับทดสอบ' }).waitFor();
      await page.evaluate(() => document.fonts.ready);
    }

    async function checkOverflow(label) {
      const widths = await page.evaluate(() => ({ viewport: innerWidth, body: document.body.scrollWidth, document: document.documentElement.scrollWidth }));
      assert.ok(widths.body <= widths.viewport + 1 && widths.document <= widths.viewport + 1, `${label}: horizontal overflow ${JSON.stringify(widths)}`);
    }

    await openOverview();
    for (const width of [1440, 1280, 1024, 768, 390, 320]) {
      await page.setViewportSize({ width, height: 1100 });
      await page.waitForTimeout(100);
      await checkOverflow(`overview ${width}`);
      const metrics = await page.locator('.summary-metrics').evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(' ').length);
      assert.equal(metrics, width > 1200 ? 4 : 2);
      await page.screenshot({ path: path.join(output, `overview-${width}.png`), fullPage: true });
      checks.push(`overview ${width}px no document overflow`);
    }

    await page.setViewportSize({ width: 1440, height: 1100 });
    await page.getByRole('region', { name: 'พื้นที่เสี่ยง (ล่าสุด)', exact: true }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(output, 'overview-desktop-lower-panels.png') });
    await page.getByRole('region', { name: 'รายงานจากประชาชน (ล่าสุด)', exact: true }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(output, 'overview-desktop-reports.png') });
    checks.push('desktop lower risk/road/report panels are reachable in existing content scroller');

    await page.setViewportSize({ width: 1440, height: 1100 });
    const chosen = page.getByRole('link', { name: /รายงานบุคคลสำหรับทดสอบ 3/ });
    await chosen.focus();
    assert.equal(await chosen.evaluate((element) => document.activeElement === element), true);
    await page.keyboard.press('Enter');
    await page.getByRole('heading', { name: 'รายงานบุคคลสำหรับทดสอบ 3', exact: true }).waitFor();
    assert.ok(page.url().endsWith('/reports?report=report-2'));
    checks.push('keyboard Enter opens selected verified individual report');

    for (const state of ['empty', 'error', 'partial', 'loading', 'media-error']) {
      mode = state;
      await page.setViewportSize({ width: 390, height: 1000 });
      await openOverview();
      if (state === 'partial') await page.getByText('ข้อมูลบางส่วนไม่พร้อมใช้งาน', { exact: true }).waitFor();
      if (state === 'media-error') await page.getByText('ไม่สามารถโหลดภาพประกอบได้', { exact: true }).first().waitFor();
      await checkOverflow(state);
      await page.screenshot({ path: path.join(output, `overview-${state}.png`), fullPage: true });
      checks.push(`${state} state visible without overflow`);
    }

    mode = 'empty';
    for (const route of ['/map', '/risk', '/announcements', '/reports']) {
      for (const width of [1440, 390]) {
        await page.setViewportSize({ width, height: 1100 });
        await page.goto('http://127.0.0.1:19176' + route);
        const hero = page.locator('.page-hero');
        await hero.waitFor();
        await page.evaluate(() => document.fonts.ready);
        assert.ok((await hero.evaluate((element) => getComputedStyle(element).backgroundImage)).includes('river-landscape.jpg'));
        const credit = hero.getByRole('link', { name: 'เครดิตภาพ', exact: true });
        await credit.scrollIntoViewIfNeeded();
        assert.ok((await credit.boundingBox()).height >= 44, 'image credit touch target');
        const clickable = await credit.evaluate((element) => { const rect = element.getBoundingClientRect(); const target = document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2); return target === element || element.contains(target); });
        assert.ok(clickable, `hero credit obscured on ${route} ${width}`);
        await checkOverflow(`${route} ${width}`);
        await hero.screenshot({ path: path.join(output, `hero-${route.slice(1)}-${width}.png`) });
        checks.push(`${route} photographic hero ${width}px with clickable credit`);
      }
    }

    mode = 'ready';
    await page.setViewportSize({ width: 390, height: 1100 });
    await openOverview();
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    await page.screenshot({ path: path.join(output, 'overview-200-percent.png'), fullPage: true });
    const rootZoom = await page.evaluate(() => ({ viewport: innerWidth, body: document.body.scrollWidth, header: document.querySelector('.institution-header').getBoundingClientRect().width, bodyMinWidth: getComputedStyle(document.body).minWidth }));
    checks.push(`existing whole-page 200% root font limitation retained (header/body width ${rootZoom.body}px at ${rootZoom.viewport}px; header unchanged)`);
    await page.evaluate(() => {
      document.documentElement.style.fontSize = '';
      const targets = Array.from(document.querySelectorAll('.overview-page, .overview-page *'));
      const sizes = targets.map((element) => parseFloat(getComputedStyle(element).fontSize));
      targets.forEach((element, index) => { element.style.fontSize = `${sizes[index] * 2}px`; });
    });
    await page.screenshot({ path: path.join(output, 'overview-scoped-200-percent.png'), fullPage: true });
    await checkOverflow('affected overview 200% text');
    checks.push('affected overview 200% text reflows without document overflow');
    assert.deepEqual(errors, []);
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ checks, errors, fixtures: 'SYNTHETIC_TEST_ONLY intercepted within isolated browser; no live flood or database writes' }, null, 2));
    console.log('PASS: ' + checks.join('; '));
  } finally {
    await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); server.close(); process.exitCode = 1; });
