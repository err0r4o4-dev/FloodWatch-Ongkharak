const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/api-coverage-ui');
const checks = [];
fs.mkdirSync(output, { recursive: true });
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2', '.png': 'image/png', '.jpg': 'image/jpeg' };
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  const candidate = path.resolve(root, '.' + pathname);
  const file = candidate.startsWith(root + path.sep) && fs.existsSync(candidate) && fs.statSync(candidate).isFile() ? candidate : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});

async function main() {
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const origin = 'http://127.0.0.1:' + server.address().port;
  const exceptions = [];
  const id = '00000000-0000-4000-8000-000000000001';
  const account = { id, email: 'synthetic-ui@example.test', role: 'ADMIN', createdAt: '2020-01-01T00:00:00Z' };

  async function scenario(mode, width, pathname) {
    const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce' });
    const page = await context.newPage();
    page.on('pageerror', (error) => exceptions.push(String(error)));
    let read = false;
    let evidenceReviewed = false;
    let statsAttempts = 0;
    let unblock;
    const gate = new Promise((resolve) => { unblock = resolve; });
    const result = (route, data, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(data) });
    await page.route('**/api/v1/**', async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const routePath = url.pathname.slice('/api/v1'.length);
      if (routePath === '/auth/refresh') return mode === 'guest' ? result(route, { error: { code: 'UNAUTHENTICATED', message: 'Test guest' } }, 401) : result(route, { user: { ...account, role: pathname.startsWith('/admin') ? 'ADMIN' : 'USER' }, accessToken: 'synthetic-ui-only-token', expiresIn: 900 });
      if (routePath === '/auth/me') return result(route, { user: account });
      if (routePath === '/notifications/public-key') return result(route, { error: { code: 'UNAVAILABLE', message: 'Test Push unavailable' } }, 503);
      if (routePath === '/events/stream') return route.abort();
      if (routePath.startsWith('/reports/') && routePath.includes('/media/')) return result(route, { error: { code: 'UNAVAILABLE', message: 'Synthetic private media unavailable' } }, 503);
      if (routePath === '/notifications' || routePath === '/notifications/unread-count') {
        if (mode === 'loading') await gate;
        if (mode === 'error') return result(route, { error: { code: 'UNAVAILABLE', message: 'Synthetic dependency error' } }, 503);
        const items = mode === 'empty' ? [] : [{ id, alertId: id, title: 'Synthetic UI fixture — not live flood data', message: 'ข้อมูลทดสอบสำหรับตรวจ UI เท่านั้น', severity: 'WARNING', sourceType: 'COMMUNITY', createdAt: '2020-01-01T00:00:00Z', readAt: read ? '2020-01-01T00:01:00Z' : null }];
        return routePath === '/notifications/unread-count' ? result(route, { count: items.length && !read ? 1 : 0 }) : result(route, { items, pagination: { total: items.length, limit: 20, offset: 0 } });
      }
      if (routePath === '/notifications/read-all' || routePath.endsWith('/read')) { read = true; return route.fulfill({ status: 204 }); }
      if (routePath === '/admin/dashboard/stats') {
        if (mode === 'stats-error' && statsAttempts++ === 0) return result(route, { error: { code: 'UNAVAILABLE', message: 'Synthetic stats error' } }, 503);
        return result(route, { totalUsers: 12, activeUsers: 9, totalReports: 7, pendingReports: 3, recentActivity: 5 });
      }
      if (routePath === '/admin/ai-evidence' || routePath === '/admin/ai-evidence/' + id) {
        const items = evidenceReviewed ? [] : [{ id, reportId: id, mediaId: id, sourceType: 'AI', modelVersion: 'synthetic-ui-fixture-v1', flood: { severity: 'UNKNOWN', water_depth_m: null, confidence: 0, bounding_box: null }, road: null, observedAt: '2020-01-01T00:00:00Z', receivedAt: '2020-01-01T00:01:00Z', reportUpdatedAt: '2020-01-01T00:00:00Z', limitations: ['TEST ONLY — not live inference'], status: 'UNVERIFIED' }];
        return routePath.endsWith('/' + id) ? result(route, { evidence: items[0] }) : result(route, { items, pagination: { total: items.length, limit: 20, offset: 0 } });
      }
      if (routePath.endsWith('/verify')) { evidenceReviewed = true; return route.fulfill({ status: 204 }); }
      return result(route, { items: [], limit: 20, offset: 0 });
    });
    await page.goto(origin + pathname, { waitUntil: 'domcontentloaded' });
    if (mode === 'guest') await page.getByRole('heading', { name: 'ลงชื่อเข้าใช้เพื่อดูการแจ้งเตือน' }).waitFor();
    else if (mode === 'error') {
      await page.getByRole('heading', { name: 'ไม่สามารถโหลดการแจ้งเตือนได้' }).waitFor();
      assert.equal(await page.getByText('ยังไม่ได้อ่าน 0 รายการ', { exact: true }).count(), 0);
    } else if (mode === 'empty') await page.getByRole('heading', { name: 'ยังไม่มีการแจ้งเตือน' }).waitFor();
    else if (mode === 'loading') await page.getByText('กำลังโหลดการแจ้งเตือน…', { exact: true }).waitFor();
    else if (pathname === '/admin') {
      if (mode === 'stats-error') {
        await page.getByRole('alert').waitFor();
        await page.screenshot({ path: path.join(output, `stats-error-${width}.png`), fullPage: true });
        await page.getByRole('button', { name: 'ลองโหลดสถิติอีกครั้ง' }).focus();
        await page.keyboard.press('Enter');
      }
      await page.getByText('12', { exact: true }).waitFor();
    } else if (pathname === '/admin/ai-evidence') {
      await page.getByRole('button', { name: /เปิดรายละเอียด:/ }).waitFor();
      await page.getByRole('button', { name: /เปิดรายละเอียด:/ }).focus();
      await page.keyboard.press('Enter');
      await page.getByText('TEST ONLY — not live inference', { exact: true }).waitFor();
    }
    else await page.getByText('Synthetic UI fixture — not live flood data', { exact: true }).waitFor();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1);
    assert.equal(overflow, false, `${mode} ${width} horizontal overflow`);
    await page.screenshot({ path: path.join(output, `${mode}-${width}.png`), fullPage: true });
    if (mode === 'inbox') {
      const markAll = page.getByRole('button', { name: /อ่านทั้งหมดแล้ว/ });
      await markAll.focus();
      await page.keyboard.press('Enter');
      await page.getByText('ยังไม่ได้อ่าน 0 รายการ', { exact: true }).waitFor();
      assert.equal(await markAll.isDisabled(), true);
      await page.screenshot({ path: path.join(output, `read-${width}.png`), fullPage: true });
    }
    if (mode === 'ai') {
      const approve = page.getByRole('button', { name: /อนุมัติ/ }).first();
      await approve.focus();
      await page.keyboard.press('Enter');
      await page.locator('.swal2-confirm').waitFor();
      await page.locator('.swal2-confirm').focus();
      await page.keyboard.press('Enter');
      await page.getByRole('heading', { name: 'อนุมัติหลักฐานแล้ว' }).waitFor();
      await page.getByRole('button', { name: 'รับทราบ' }).focus();
      await page.keyboard.press('Enter');
      await page.locator('.swal2-container').waitFor({ state: 'detached' });
      await page.getByText(/ไม่มีหลักฐาน AI/).waitFor();
    }
    checks.push({ mode, width, pathname, overflow: false });
    unblock();
    await context.close();
  }

  try {
    for (const width of [1440, 390, 320]) await scenario('inbox', width, '/notifications');
    for (const mode of ['guest', 'empty', 'error', 'loading']) await scenario(mode, 390, '/notifications');
    await scenario('stats', 1440, '/admin');
    await scenario('stats-error', 390, '/admin');
    await scenario('ai', 1440, '/admin/ai-evidence');
    await scenario('ai', 390, '/admin/ai-evidence');
    assert.deepEqual(exceptions, []);
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ checks, exceptions }, null, 2));
    console.log(JSON.stringify({ checks: checks.length, exceptions: exceptions.length, output }));
  } finally {
    await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
}
main().catch((error) => { console.error(error); process.exitCode = 1; server.close(); });
