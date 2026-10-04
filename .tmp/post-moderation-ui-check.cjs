const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');
const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/post-moderation-ui');
const origin = 'http://127.0.0.1:19321';
const baseTime = Date.parse('2026-10-04T12:00:00Z');
const reportId = '00000000-0000-4000-8000-000000000101';
const administrator = { id: '00000000-0000-4000-8000-000000000102', email: 'synthetic-admin@example.test', role: 'ADMIN', createdAt: new Date(baseTime).toISOString() };
const server = http.createServer((request, response) => {
  const target = path.resolve(root, '.' + decodeURIComponent(new URL(request.url, origin).pathname));
  const file = target.startsWith(root + path.sep) && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  const contentTypes = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
  response.setHeader('Content-Type', contentTypes[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});
function newReport() {
  return { id: reportId, displayName: 'Synthetic reporter', type: 'FLOOD', description: 'Synthetic post-moderation evidence only', locationName: 'Synthetic flood point', locationNote: 'Fixture, not live data', location: { type: 'Point', coordinates: [101, 14] }, estimatedWaterDepthM: 0.3, status: 'PENDING', verificationStatus: 'PENDING', publicationStatus: 'PUBLISHED', sourceType: 'COMMUNITY', createdAt: new Date(baseTime).toISOString(), updatedAt: new Date(baseTime).toISOString(), verifiedAt: null, media: [], reviewHistory: [] };
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  await new Promise((resolve) => server.listen(19321, '127.0.0.1', resolve));
  let browser;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    for (const width of [390, 1440]) {
      const context = await browser.newContext({ viewport: { width, height: 980 }, reducedMotion: 'reduce' });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      let report = newReport();
      let version = 0;
      const submittedActions = [];
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await context.route('**/api/v1/**', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        const method = request.method();
        if (url.pathname.endsWith('/auth/refresh')) return route.fulfill({ json: { user: administrator, accessToken: 'synthetic-session-only', expiresIn: 900 } });
        if (url.pathname.endsWith('/auth/me')) return route.fulfill({ json: { user: administrator } });
        if (url.pathname.endsWith('/events/stream')) return route.fulfill({ contentType: 'text/event-stream', body: 'event: connected\ndata: {}\n\n' });
        if (url.pathname === '/api/v1/admin/reports') return route.fulfill({ json: { items: url.searchParams.get('status') === report.status ? [report] : [], limit: 20, offset: 0 } });
        if (url.pathname === '/api/v1/admin/reports/' + reportId) return route.fulfill({ json: { report } });
        if (url.pathname.startsWith('/api/v1/admin/reports/' + reportId + '/') && method === 'POST') {
          const action = url.pathname.split('/').at(-1);
          const input = request.postDataJSON();
          assert.equal(input.expectedUpdatedAt, report.updatedAt);
          if (action !== 'verify') assert(input.reason.trim());
          submittedActions.push(action);
          const previousStatus = report.status;
          const status = action === 'verify' ? 'VERIFIED' : action === 'reject' ? 'REJECTED' : 'PENDING';
          const now = new Date(baseTime + (++version) * 1000).toISOString();
          report = { ...report, status, verificationStatus: status, publicationStatus: status === 'REJECTED' ? 'REMOVED' : 'PUBLISHED', updatedAt: now, verifiedAt: status === 'VERIFIED' ? now : null, rejectionReason: status === 'REJECTED' ? input.reason : null, reviewHistory: [{ id: 'synthetic-history-' + version, action: action === 'verify' ? 'VERIFY' : action === 'reject' ? 'REMOVE' : 'RESTORE', previousStatus, status, actorId: administrator.id, reason: input.reason || '', createdAt: now }, ...report.reviewHistory] };
          return route.fulfill({ json: { report } });
        }
        if (url.pathname === '/api/v1/reports') {
          const { reviewHistory, rejectionReason, ...publicReport } = report;
          return route.fulfill({ json: { items: report.status === 'REJECTED' ? [] : [publicReport], limit: 20, offset: 0 } });
        }
        if (url.pathname.includes('/weather/')) return route.fulfill({ status: 503, json: { error: { code: 'UNAVAILABLE' } } });
        return route.fulfill({ json: { items: [], limit: 20, offset: 0 } });
      });
      await page.goto(origin + '/admin/reports');
      const weatherToggle = page.getByRole('button', { name: 'ย่อการ์ดสภาพอากาศ' });
      if (await weatherToggle.isVisible()) await weatherToggle.click();
      await page.getByRole('button', { name: 'เปิดรายงาน FLOOD', exact: true }).click();
      await page.getByRole('dialog', { name: 'เปิดรายงาน FLOOD' }).waitFor();
      await page.getByRole('button', { name: 'นำรายงานออก', exact: true }).click();
      await page.getByRole('alert').filter({ hasText: 'กรุณาระบุเหตุผล' }).waitFor();
      assert.deepEqual(submittedActions, []);
      await page.keyboard.press('Escape');
      await page.getByRole('button', { name: 'เปิดรายงาน FLOOD', exact: true }).click();
      const closeButton = page.getByRole('button', { name: 'ปิดรายละเอียดรายงาน', exact: true });
      await closeButton.waitFor();
      assert(await closeButton.evaluate((element) => element === document.activeElement));
      await page.getByRole('button', { name: 'ยืนยันรายงาน', exact: true }).click();
      await page.locator('.swal2-confirm').click();
      await page.locator('.swal2-title').filter({ hasText: 'ยืนยันรายงานแล้ว' }).waitFor();
      await page.locator('.swal2-confirm').click();
      await page.getByRole('button', { name: 'ตรวจสอบแล้ว', exact: true }).click();
      await page.getByRole('button', { name: 'เปิดรายงาน FLOOD', exact: true }).click();
      await page.getByLabel('เหตุผลการนำออก', { exact: true }).fill('Synthetic incorrect evidence');
      await page.screenshot({ path: path.join(output, 'admin-reviewed-' + width + '.png'), fullPage: true });
      await page.getByRole('button', { name: 'นำรายงานออก', exact: true }).click();
      await page.locator('.swal2-cancel').click();
      assert.deepEqual(submittedActions, ['verify']);
      await page.getByRole('button', { name: 'นำรายงานออก', exact: true }).click();
      await page.locator('.swal2-confirm').click();
      await page.locator('.swal2-title').filter({ hasText: 'นำรายงานออกแล้ว' }).waitFor();
      await page.locator('.swal2-confirm').click();
      await page.goto(origin + '/reports');
      await page.locator('.reports-state-panel').waitFor();
      assert.equal(await page.getByText('Synthetic post-moderation evidence only', { exact: true }).count(), 0);
      await page.goto(origin + '/admin/reports');
      await page.getByRole('button', { name: 'นำออกแล้ว', exact: true }).click();
      await page.getByRole('button', { name: 'เปิดรายงาน FLOOD', exact: true }).click();
      await page.getByLabel('เหตุผลการนำออกหรือกู้คืน', { exact: true }).fill('Synthetic mistaken removal');
      await page.screenshot({ path: path.join(output, 'admin-removed-' + width + '.png'), fullPage: true });
      await page.getByRole('button', { name: 'กู้คืนรายงาน', exact: true }).click();
      await page.locator('.swal2-confirm').click();
      await page.locator('.swal2-title').filter({ hasText: 'กู้คืนรายงานแล้ว' }).waitFor();
      await page.locator('.swal2-confirm').click();
      await page.getByRole('button', { name: 'ยังไม่ตรวจสอบ', exact: true }).click();
      await page.getByRole('button', { name: 'เปิดรายงาน FLOOD', exact: true }).click();
      assert.equal(await page.locator('.admin-review__history li').count(), 3);
      assert.equal(report.status, 'PENDING');
      assert.equal(report.verifiedAt, null);
      await page.screenshot({ path: path.join(output, 'admin-restored-' + width + '.png'), fullPage: true });
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      const targets = await page.locator('.admin-review button').evaluateAll((buttons) => buttons.map((button) => button.getBoundingClientRect().height));
      assert(targets.every((height) => height >= 44));
      await page.goto(origin + '/reports');
      await page.locator('.report-card').waitFor();
      await page.getByText('เผยแพร่แล้ว · ยังไม่ตรวจสอบ', { exact: true }).first().waitFor();
      assert(await page.getByText('เผยแพร่แล้ว · ยังไม่ตรวจสอบ', { exact: true }).count() >= 1);
      assert.equal(await page.getByText('Synthetic incorrect evidence', { exact: false }).count(), 0);
      await page.screenshot({ path: path.join(output, 'public-restored-' + width + '.png'), fullPage: true });
      assert.deepEqual(submittedActions, ['verify', 'reject', 'restore']);
      assert.deepEqual(errors, []);
      await context.close();
      console.log('PASS ' + width + ': review, mandatory reason, cancel, remove, public absence, restore-unreviewed, history, Escape/focus, 44px targets and no overflow; synthetic fixtures only.');
    }
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
