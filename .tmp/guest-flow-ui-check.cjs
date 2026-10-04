const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');
const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/guest-flow-ui');
const origin = 'http://127.0.0.1:19189';
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
const server = http.createServer((request, response) => {
  const target = path.resolve(root, '.' + decodeURIComponent(new URL(request.url, origin).pathname));
  const file = target.startsWith(root + path.sep) && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', types[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});
const report = { id: '00000000-0000-4000-8000-000000000001', displayName: 'Synthetic reporter', type: 'FLOOD', description: 'Synthetic verified evidence for isolated UI testing only', location: { type: 'Point', coordinates: [101.1,14.2] }, estimatedWaterDepthM: null, status: 'VERIFIED', verificationStatus: 'VERIFIED', sourceType: 'COMMUNITY', createdAt: '2026-10-04T00:00:00Z', updatedAt: '2026-10-04T01:00:00Z', verifiedAt: '2026-10-04T01:00:00Z', media: [] };
const notice = { id: 'report:' + report.id, severity: 'INFO', sourceType: 'COMMUNITY', createdAt: report.verifiedAt, title: 'Synthetic notification', message: report.description, url: '/reports?report=' + report.id, location: report.location };
const account = { id: '00000000-0000-4000-8000-000000000002', email: 'synthetic-admin@example.test', username: 'test_admin', role: 'ADMIN', createdAt: report.createdAt };
(async () => {
  fs.mkdirSync(output, { recursive: true });
  await new Promise((resolve) => server.listen(19189, '127.0.0.1', resolve));
  let browser;
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const context = await browser.newContext({ reducedMotion: 'reduce' });
    const errors = [];
    let session = false;
    let submitted = null;
    let noticesFail = false;
    const page = await context.newPage();
    page.on('pageerror', (error) => errors.push(error.message));
    await context.route('**/api/v1/**', async (route) => {
      const url = new URL(route.request().url());
      const method = route.request().method();
      if (url.pathname.endsWith('/auth/refresh')) return route.fulfill({ status: session ? 200 : 401, json: session ? { user: account, accessToken: 'synthetic-admin-session', expiresIn: 900 } : { error: { code: 'UNAUTHENTICATED' } } });
      if (url.pathname.endsWith('/auth/login')) { session = true; return route.fulfill({ json: { user: account, accessToken: 'synthetic-admin-session', expiresIn: 900 } }); }
      if (url.pathname.endsWith('/auth/me')) return route.fulfill({ json: { user: account } });
      if (url.pathname.endsWith('/reports') && method === 'POST') { submitted = route.request().postDataJSON(); return route.fulfill({ status: 201, json: { report: { ...report, ...submitted, status: 'PENDING', verificationStatus: 'PENDING', verifiedAt: null }, uploadToken: 'synthetic-private-capability' } }); }
      if (url.pathname.endsWith('/admin/reports')) return route.fulfill({ json: { items: [], limit: 20, offset: 0 } });
      if (url.pathname.endsWith('/reports')) return route.fulfill({ json: { items: [report], limit: 20, offset: 0 } });
      if (url.pathname.endsWith('/notifications/public')) return route.fulfill({ status: noticesFail ? 503 : 200, json: noticesFail ? { error: { code: 'UNAVAILABLE' } } : { items: url.searchParams.has('bbox') ? [] : [notice], pagination: { total: url.searchParams.has('bbox') ? 0 : 1, limit: 10, offset: 0 } } });
      return route.fulfill({ json: { items: [], limit: 100, offset: 0 } });
    });
    await context.route('**/api/v1/events/stream', (route) => route.fulfill({ contentType: 'text/event-stream', body: 'event: connected\ndata: {}\n\n' }));
    for (const width of [320,390,768,1440]) {
      await page.setViewportSize({ width, height: 980 });
      await page.goto(origin + '/reports');
      await page.getByRole('heading', { name: 'ส่งรายงานชุมชน', exact: true }).waitFor();
      await page.locator('.account-menu summary').click();
      const name = page.locator('.account-menu__content input');
      await name.fill('ผู้ทดสอบ Guest');
      await page.locator('.account-menu__content').getByRole('button', { name: 'บันทึก', exact: true }).click();
      await name.press('Escape');
      assert(await page.locator('.account-menu summary').evaluate((element) => element === document.activeElement));
      assert.equal(await page.locator('a[href*="login"]').count(), 0);
      assert.equal(await page.getByLabel('ชื่อที่แสดง (ไม่ใช่บัญชีผู้ใช้งาน)').inputValue(), 'ผู้ทดสอบ Guest');
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
      await page.locator('.account-menu summary').click();
      await page.screenshot({ path: path.join(output, `reports-menu-${width}.png`), fullPage: true });
      await page.keyboard.press('Escape');
      await page.goto(origin + '/notifications');
      await page.getByText('Synthetic notification', { exact: true }).waitFor();
      assert.equal(await page.locator('a[href*="login"]').count(), 0);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
      await page.screenshot({ path: path.join(output, `notifications-${width}.png`), fullPage: true });
    }
    await page.goto(origin + '/reports');
    await page.getByLabel('รายละเอียด', { exact: true }).fill('Synthetic pending report, never live evidence');
    await page.getByLabel('ลองจิจูด', { exact: true }).fill('101.1');
    await page.getByLabel('ละติจูด', { exact: true }).fill('14.2');
    await page.getByRole('button', { name: 'ส่งรายงาน', exact: true }).click();
    await page.locator('.form-success').getByText('รายงานยังรอการตรวจสอบ และยังไม่เผยแพร่ต่อสาธารณะ', { exact: true }).waitFor();
    assert.equal(submitted.displayName, 'ผู้ทดสอบ Guest');
    assert.equal(await page.locator('button').filter({ hasText: 'แก้ไขรายงานที่รอตรวจสอบ' }).count(), 0);
    await page.screenshot({ path: path.join(output, 'submitted-pending.png'), fullPage: true });
    await page.goto(origin + '/admin/reports');
    await page.waitForURL('**/admin/login?returnTo=*');
    assert.equal(await page.locator('a[href*="register"]').count(), 0);
    await page.getByLabel('อีเมล หรือ ชื่อผู้ใช้งาน', { exact: true }).fill('synthetic-admin@example.test');
    await page.getByLabel('รหัสผ่าน', { exact: true }).fill('synthetic-test-only-passphrase');
    await page.locator('.auth-form button[type="submit"]').click();
    await page.waitForURL('**/admin/reports');
    await page.getByRole('button', { name: 'รับทราบ', exact: true }).click();
    await page.locator('.swal2-container').waitFor({ state: 'hidden' });
    await page.screenshot({ path: path.join(output, 'admin-reports.png'), fullPage: true });
    await page.goto(origin + '/notifications');
    await page.getByText('Synthetic notification', { exact: true }).waitFor();
    await page.locator('.account-menu summary').click();
    assert.equal(await page.locator('.account-menu__content input').inputValue(), 'ผู้ทดสอบ Guest');
    assert.equal(await page.locator('.account-menu__content').getByText(account.email, { exact: true }).count(), 0);
    await page.keyboard.press('Escape');
    noticesFail = true;
    await page.reload();
    await page.locator('[role="alert"]').waitFor();
    await page.screenshot({ path: path.join(output, 'notifications-error.png'), fullPage: true });
    noticesFail = false;
    await page.getByRole('button', { name: 'ลองอีกครั้ง', exact: true }).click();
    await page.getByText('Synthetic notification', { exact: true }).waitFor();
    await page.setViewportSize({ width: 390, height: 980 });
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
    await page.screenshot({ path: path.join(output, 'notifications-zoom.png'), fullPage: true });
    assert.deepEqual(errors, []);
    console.log('PASS: Guest name/save/Escape, report submission/PENDING, no public login, admin return route, public feed during admin session, error retry; 320/390/768/1440px and mobile 200% text. All API evidence intercepted synthetic fixtures, no running database mutations.');
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
