const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/i18n-ui');
fs.mkdirSync(output, { recursive: true });
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2', '.jpg': 'image/jpeg', '.png': 'image/png' };
const server = http.createServer((request, response) => {
  const target = path.resolve(root, '.' + new URL(request.url, 'http://localhost').pathname);
  const file = target.startsWith(root + path.sep) && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});
const account = { id: '00000000-0000-4000-8000-000000000001', email: 'synthetic-ui@example.test', username: 'test_user', role: 'ADMIN', createdAt: '2026-10-04T00:00:00Z' };
const original = 'ต้นฉบับทดสอบเท่านั้น';
const alert = { id: 'synthetic-alert', ruleId: 'synthetic-rule', severity: 'WARNING', title: original, message: original, sourceType: 'OFFICIAL', resourceType: 'ROAD', resourceId: 'synthetic-road', triggeredAt: '2026-10-04T00:00:00Z', createdAt: '2026-10-04T00:00:00Z' };
const checks = [];
const failures = [];

(async () => {
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const origin = 'http://127.0.0.1:' + server.address().port;
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    for (const width of [320, 390, 1440]) {
      const context = await browser.newContext({ viewport: { width, height: 950 }, reducedMotion: 'reduce', serviceWorkers: 'block' });
      let failData = false;
      let guest = false;
      const requests = [];
      await context.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== origin) return route.abort();
        if (!url.pathname.startsWith('/api/v1/')) return route.continue();
        requests.push(url.pathname);
        if (url.pathname.includes('/events')) return route.abort();
        if (url.pathname === '/api/v1/auth/refresh') return guest ? route.fulfill({ status: 401, json: { error: { code: 'UNAUTHENTICATED', message: 'Synthetic guest' } } }) : route.fulfill({ json: { user: account, accessToken: 'synthetic-test-token', expiresIn: 900 } });
        if (url.pathname === '/api/v1/auth/me') return route.fulfill({ json: { user: account } });
        if (url.pathname.startsWith('/api/v1/auth/google/')) return route.fulfill({ status: 503, json: { error: { code: 'GOOGLE_UNAVAILABLE', message: 'Synthetic unavailable state' } } });
        if (failData) return route.fulfill({ status: 503, json: { error: { code: 'UNAVAILABLE', message: 'Synthetic unavailable data' } } });
        if (url.pathname === '/api/v1/admin/dashboard') return route.fulfill({ json: { totalUsers: 2, activeUsers: 1, totalReports: 0, pendingReports: 0, recentActivity: 0 } });
        if (url.pathname === '/api/v1/admin/users') return route.fulfill({ json: { users: [], pagination: { total: 0, limit: 50, offset: 0 } } });
        if (url.pathname.startsWith('/api/v1/admin/users/')) return route.fulfill({ json: { user: { ...account, updatedAt: account.createdAt, disabledAt: null }, stats: { reportCount: 0, lastActivity: null, verifiedReports: 0, rejectedReports: 0, auditLogCount: 0, refreshSessionCount: 0 } } });
        if (url.pathname === '/api/v1/admin/settings') return route.fulfill({ json: { settings: [] } });
        if (url.pathname === '/api/v1/admin/audit-logs') return route.fulfill({ json: { logs: [], pagination: { total: 0, limit: 50, offset: 0 } } });
        if (url.pathname === '/api/v1/notifications/unread-count') return route.fulfill({ json: { count: 1 } });
        if (url.pathname === '/api/v1/notifications/public') return route.fulfill({ json: { items: [{ id: 'alert:synthetic-alert', title: original, message: original, severity: 'WARNING', sourceType: 'OFFICIAL', createdAt: alert.createdAt, url: '/announcements', location: null }], pagination: { total: 1, limit: 10, offset: 0 } } });
        if (url.pathname === '/api/v1/notifications') return route.fulfill({ json: { items: [{ id: 'synthetic-notification', alertId: alert.id, title: original, message: original, severity: 'WARNING', sourceType: 'OFFICIAL', createdAt: alert.createdAt, readAt: null }], pagination: { total: 1, limit: 10, offset: 0 } } });
        if (url.pathname === '/api/v1/alerts') return route.fulfill({ json: { items: [alert], limit: 20, offset: 0 } });
        return route.fulfill({ json: { items: [], limit: 100, offset: 0, pagination: { total: 0, limit: 20, offset: 0 } } });
      });
      const page = await context.newPage();
      page.on('pageerror', (error) => failures.push(error.message));
      const routes = ['/', '/overview', '/map', '/announcements', '/reports', '/notifications', '/login', '/register', '/forgot-password', '/auth/google/link', '/system-status', '/admin', '/admin/login', '/admin/reports', '/admin/users', '/admin/users/' + account.id, '/admin/settings', '/admin/audit-logs', '/admin/ai-evidence'];
      for (const route of routes) {
        await page.goto(origin + route);
        await page.waitForTimeout(180);
        await page.getByRole('button', { name: 'EN', exact: true }).click();
        await page.waitForTimeout(100);
        assert.equal(await page.locator('html').getAttribute('lang'), 'en', route);
        assert.equal(await page.evaluate(() => localStorage.getItem('floodwatch.locale')), 'en');
        const text = (await page.locator('body').innerText()).split(original).join('');
        assert(!/[ก-๙]/.test(text), 'Untranslated UI on ' + route + ': ' + text.match(/[^\n]*[ก-๙][^\n]*/g));
        const accessibleText = await page.locator('[aria-label], [placeholder], [title]').evaluateAll((elements) => elements.map((element) => ['aria-label', 'placeholder', 'title'].map((attribute) => element.getAttribute(attribute) || '').join(' ')).join(' '));
        assert(!/[ก-๙]/.test(accessibleText.split(original).join('')), 'Untranslated accessible labels on ' + route);
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1);
        assert(!overflow, 'Horizontal overflow: ' + width + ' ' + route);
        if (['/', '/map', '/register', '/notifications', '/admin/settings', '/admin/users/' + account.id].includes(route)) await page.screenshot({ path: path.join(output, width + '-en-' + (route === '/' ? 'home' : route.slice(1).replaceAll('/', '-')) + '.png'), fullPage: true });
        await page.reload();
        assert.equal(await page.locator('html').getAttribute('lang'), 'en', 'Persisted ' + route);
        await page.getByRole('button', { name: 'TH', exact: true }).click();
        assert.equal(await page.locator('html').getAttribute('lang'), 'th');
        checks.push(width + ' ' + route + ' bilingual, persisted, no overflow');
      }
      guest = true;
      await page.goto(origin + '/admin/login');
      await page.getByLabel('อีเมล หรือ ชื่อผู้ใช้งาน', { exact: true }).fill('draft@example.test');
      await page.getByLabel('รหัสผ่าน', { exact: true }).fill('short');
      await page.getByRole('button', { name: 'เข้าสู่ระบบผู้ดูแลระบบ', exact: true }).click();
      await page.getByRole('alert').waitFor();
      await page.getByRole('button', { name: 'EN', exact: true }).click();
      assert.equal(await page.getByLabel('Email or username', { exact: true }).inputValue(), 'draft@example.test');
      assert.equal(await page.getByLabel('Password', { exact: true }).inputValue(), 'short');
      assert.match(await page.getByRole('alert').innerText(), /too short/i);
      assert(!(await page.locator('body').innerText()).includes('ASCII'));
      await page.getByRole('button', { name: 'TH', exact: true }).focus();
      await page.keyboard.press('Enter');
      assert.equal(await page.locator('html').getAttribute('lang'), 'th');
      checks.push(width + ' auth draft + translated error + keyboard switch');
      await page.goto(origin + '/notifications');
      await page.getByRole('button', { name: 'EN', exact: true }).click();
      await page.getByText(original, { exact: true }).first().waitFor();
      assert((await page.locator('body').innerText()).includes('Original announcement'));
      assert(!(await page.locator('body').innerText()).split(original).join('').match(/[ก-๙]/));
      checks.push(width + ' guest public feed preserves original content and translates severity');
      failData = true;
      await page.goto(origin + '/map');
      await page.getByRole('button', { name: 'EN', exact: true }).click();
      await page.getByRole('alert').waitFor();
      assert.match(await page.getByRole('alert').innerText(), /unknown/i);
      assert(!/area is safe/i.test(await page.getByRole('alert').innerText()));
      await page.screenshot({ path: path.join(output, width + '-en-map-error.png'), fullPage: true });
      checks.push(width + ' map failure remains unknown');
      if (width === 390) {
        await page.goto(origin + '/admin/login');
        await page.getByRole('button', { name: 'EN', exact: true }).click();
        await page.addStyleTag({ content: 'html { font-size: 200% !important; }' });
        const overflowing = await page.evaluate(() => [...document.querySelectorAll('body *')].map((element) => ({ tag: element.tagName, className: element.className, text: element.textContent.slice(0, 80), right: element.getBoundingClientRect().right, width: element.getBoundingClientRect().width })).filter((item) => item.right > innerWidth + 1));
        await page.screenshot({ path: path.join(output, '390-en-auth-200pct.png'), fullPage: true });
        assert(!(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)), '200% text overflow: ' + JSON.stringify(overflowing));
        await page.screenshot({ path: path.join(output, '390-en-auth-200pct.png'), fullPage: true });
        checks.push('390 English auth at 200% text');
      }
      await context.close();
    }
    assert.deepEqual(failures, []);
    fs.writeFileSync(path.join(output, 'checks.json'), JSON.stringify({ checks, failures }, null, 2));
    console.log(JSON.stringify({ checks: checks.length, failures, output }, null, 2));
  } finally { await browser.close(); await new Promise((resolve) => server.close(resolve)); }
})().catch((error) => { console.error(error); process.exitCode = 1; server.close(); });
