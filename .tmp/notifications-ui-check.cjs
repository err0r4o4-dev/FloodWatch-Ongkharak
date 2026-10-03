const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('.tmp/notifications-dist');
const output = path.resolve('.tmp/notifications-ui');
const checks = [];
const errors = [];
const account = { id: 'ui-fixture', email: 'ui-fixture@example.test', role: 'USER', createdAt: '2026-01-01T00:00:00Z' };
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
fs.mkdirSync(output, { recursive: true });
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  const candidate = path.resolve(root, '.' + pathname);
  const safe = candidate.startsWith(root + path.sep);
  const file = safe && fs.existsSync(candidate) && fs.statSync(candidate).isFile() ? candidate : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.setHeader('Cache-Control', 'no-store');
  response.end(fs.readFileSync(file));
});

async function main() {
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const origin = 'http://127.0.0.1:' + server.address().port;
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  let currentPage;

  async function scenario(mode, width = 1440) {
    const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce' });
    const page = await context.newPage();
    currentPage = page;
    page.on('pageerror', (error) => errors.push(error.message));
    let subscriptionAttempts = 0;
    const requests = [];
    await page.route('**/api/**', async (route) => {
      const request = route.request();
      const pathname = new URL(request.url()).pathname;
      requests.push({ path: pathname, method: request.method() });
      const reply = (status, data) => route.fulfill({ status, contentType: 'application/json', headers: { 'X-Validation-Fixture': 'notifications-ui-only' }, ...(status === 204 ? {} : { body: JSON.stringify(data) }) });
      if (pathname.endsWith('/events/stream')) return route.abort();
      if (pathname.endsWith('/auth/refresh')) return mode === 'guest' ? reply(401, { error: { code: 'UNAUTHORIZED' } }) : reply(200, { accessToken: 'synthetic-ui-token', expiresIn: 300, user: account });
      if (pathname.endsWith('/auth/me')) return reply(200, { user: account });
      if (pathname.endsWith('/notifications/public-key')) return reply(200, { publicKey: 'dGVzdA==' });
      if (pathname.endsWith('/notifications/subscribe')) {
        assert.equal(request.headers().authorization, 'Bearer synthetic-ui-token');
        subscriptionAttempts++;
        return mode === 'action-error' && subscriptionAttempts === 1 ? reply(503, { error: { code: 'UNAVAILABLE' } }) : reply(204);
      }
      if (pathname.endsWith('/notifications/unsubscribe')) {
        assert.equal(request.headers().authorization, 'Bearer synthetic-ui-token');
        return reply(204);
      }
      return reply(503, { error: { code: 'UNAVAILABLE', message: 'Not available in isolated UI validation' } });
    });
    await page.route('https://**', (route) => route.abort());
    await page.addInitScript(({ mode }) => {
      const fixture = { subscription: null, subscribeCalls: 0, permissionRequests: 0, checks: 0 };
      window.__pushFixture = fixture;
      const registration = {
        active: {},
        pushManager: {
          getSubscription: async () => fixture.subscription,
          subscribe: async () => {
            fixture.subscribeCalls++;
            fixture.subscription = {
              endpoint: 'https://push.example.test/ui-only',
              toJSON: () => ({ endpoint: 'https://push.example.test/ui-only', keys: { p256dh: 'fixture-key', auth: 'fixture-auth' } }),
              unsubscribe: async () => { fixture.subscription = null; return true; }
            };
            return fixture.subscription;
          }
        }
      };
      Object.defineProperty(window, 'PushManager', { configurable: true, value: mode === 'unsupported' ? undefined : class {} });
      Object.defineProperty(window, 'Notification', { configurable: true, value: {
        permission: mode === 'denied' ? 'denied' : 'default',
        requestPermission: async () => { fixture.permissionRequests++; return 'granted'; }
      } });
      Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: {
        register: async () => registration,
        getRegistration: async () => {
          fixture.checks++;
          if (mode === 'no-worker') return undefined;
          if (mode === 'check-error' && fixture.checks === 1) throw new Error('Isolated browser-check fixture');
          if (mode === 'checking' && fixture.checks === 1) return new Promise((resolve) => { window.__resolveRegistration = () => resolve(registration); });
          return registration;
        }
      } });
    }, { mode });
    await page.goto(origin + '/notifications');
    await page.getByRole('heading', { level: 1, name: 'การแจ้งเตือน', exact: true }).waitFor();
    return { page, context, requests };
  }

  try {
    const guest = await scenario('guest');
    const page = guest.page;
    await page.getByRole('link', { name: 'เข้าสู่ระบบเพื่อเปิดแจ้งเตือน' }).waitFor();
    assert.equal(await page.getByRole('button', { name: 'อ่านทั้งหมดแล้ว' }).isDisabled(), true);
    assert.equal(await page.locator('main').count(), 1);
    assert.equal(await page.getByRole('link', { name: 'การแจ้งเตือน', exact: true }).getAttribute('aria-current'), 'page');
    for (const width of [1440, 1280, 1024, 820, 768, 390, 320]) {
      await page.setViewportSize({ width, height: width < 600 ? 844 : 900 });
      await page.locator('.page-content').evaluate((element) => { element.scrollTop = 0; });
      await page.evaluate(() => window.scrollTo(0, 0));
      const metrics = await page.evaluate(() => {
        const layout = document.querySelector('.notifications-page__layout');
        const history = document.querySelector('.notifications-history').getBoundingClientRect();
        const sidebar = document.querySelector('.notifications-page__sidebar').getBoundingClientRect();
        const bell = document.querySelector('a.header-icon').getBoundingClientRect();
        return { width: innerWidth, overflow: document.documentElement.scrollWidth > innerWidth, columns: getComputedStyle(layout).gridTemplateColumns.split(' ').length, history: { right: history.right, bottom: history.bottom }, sidebar: { left: sidebar.left, top: sidebar.top }, bell: { width: bell.width, height: bell.height } };
      });
      assert.equal(metrics.overflow, false, width + ': horizontal overflow');
      assert.equal(metrics.columns, width > 880 ? 2 : 1, width + ': layout columns');
      assert.ok(metrics.bell.width >= 44 && metrics.bell.height >= 44, width + ': header bell touch target');
      if (width > 880) assert.ok(metrics.sidebar.left > metrics.history.right);
      else assert.ok(metrics.sidebar.top >= metrics.history.bottom);
      await page.screenshot({ path: path.join(output, 'guest-' + width + '.png') });
      await page.locator('.page-content').evaluate((element) => { element.scrollTop = element.scrollHeight; });
      await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
      const navigation = page.locator('.bottom-navigation');
      if (await navigation.isVisible()) {
        const link = await page.getByRole('link', { name: 'ดูแผนที่สถานการณ์น้ำ' }).boundingBox();
        const nav = await navigation.boundingBox();
        assert.ok(link.y + link.height < nav.y, width + ': navigation clearance');
      }
      checks.push(width + 'px responsive layout, 44px header bell, no overflow, navigation clearance');
    }
    assert.equal(await page.evaluate(() => window.__pushFixture.permissionRequests + window.__pushFixture.subscribeCalls), 0);
    assert.equal(guest.requests.some((request) => request.path.includes('/notifications/')), false);
    await page.goto(origin + '/system-status');
    const bell = page.getByRole('link', { name: 'การแจ้งเตือน', exact: true });
    await bell.focus();
    assert.equal(await bell.evaluate((element) => getComputedStyle(element).outlineStyle), 'solid');
    await page.keyboard.press('Enter');
    await page.getByRole('heading', { level: 1, name: 'การแจ้งเตือน', exact: true }).waitFor();
    assert.equal(await page.getByRole('heading', { level: 1, name: 'การแจ้งเตือน', exact: true }).evaluate((element) => element === document.activeElement), true);
    await page.reload();
    await page.getByRole('heading', { name: 'ประวัติการแจ้งเตือนยังไม่พร้อมใช้งาน' }).waitFor();
    checks.push('bookmark/reload, keyboard Enter from header, heading focus, disabled mark-all-read, guest page makes no push/history calls');
    await page.evaluate(() => { document.documentElement.style.fontSize = '32px'; });
    assert.equal(await page.locator('.notifications-page').evaluate((element) => element.scrollWidth <= element.clientWidth), true);
    await page.locator('.notifications-card').first().scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(output, 'guest-320-text-200.png') });
    checks.push('notification content fits at 200% root text size on 320px');
    await guest.context.close();

    for (const mode of ['no-worker', 'unsupported', 'denied', 'checking', 'check-error']) {
      const state = await scenario(mode, 390);
      const expected = {
        'no-worker': 'ระบบแจ้งเตือนบนเบราว์เซอร์ยังไม่พร้อมใช้งาน',
        unsupported: 'เบราว์เซอร์นี้ไม่รองรับการแจ้งเตือนแบบ Push',
        denied: 'เบราว์เซอร์ปิดกั้นการแจ้งเตือนอยู่',
        checking: 'กำลังตรวจสอบการแจ้งเตือน…',
        'check-error': 'ตรวจสอบการแจ้งเตือนไม่สำเร็จ กรุณาลองอีกครั้ง'
      };
      await state.page.getByText(expected[mode], { exact: true }).waitFor();
      await state.page.locator('.notifications-card').first().scrollIntoViewIfNeeded();
      if (mode !== 'unsupported') assert.equal(await state.page.getByRole('checkbox').isDisabled(), true);
      await state.page.screenshot({ path: path.join(output, mode + '-390.png') });
      if (mode === 'checking') {
        await state.page.evaluate(() => window.__resolveRegistration());
        await state.page.waitForFunction(() => !document.querySelector('input[type=checkbox]').disabled);
      }
      if (mode === 'check-error') {
        await state.page.getByRole('button', { name: 'ตรวจสอบอีกครั้ง' }).click();
        await state.page.waitForFunction(() => !document.querySelector('input[type=checkbox]').disabled);
        assert.equal(await state.page.getByRole('alert').count(), 0);
      }
      assert.equal(await state.page.evaluate(() => window.__pushFixture.permissionRequests + window.__pushFixture.subscribeCalls), 0);
      checks.push(mode + ': rendered mobile state, no automatic permission request, retry where supported');
      await state.context.close();
    }

    const action = await scenario('action-error', 1440);
    await action.page.waitForFunction(() => !document.querySelector('input[type=checkbox]').disabled);
    const toggle = action.page.getByRole('checkbox', { name: 'รับการแจ้งเตือนผ่านเบราว์เซอร์' });
    await toggle.focus();
    assert.equal(await action.page.locator('.notification-preference__control').evaluate((element) => getComputedStyle(element).outlineStyle), 'solid');
    assert.ok(await action.page.locator('.notification-preference__switch').evaluate((element) => parseFloat(getComputedStyle(element, '::after').transitionDuration) <= .00001));
    await action.page.keyboard.press('Space');
    await action.page.getByRole('dialog', { name: 'เปลี่ยนการตั้งค่าไม่สำเร็จ' }).waitFor();
    await action.page.getByRole('button', { name: 'รับทราบ' }).click();
    assert.equal(await toggle.isChecked(), false);
    await action.page.getByRole('alert').waitFor();
    await action.page.screenshot({ path: path.join(output, 'action-error-desktop.png') });
    await toggle.focus();
    await action.page.keyboard.press('Space');
    await action.page.getByRole('dialog', { name: 'เปิดการแจ้งเตือนแล้ว' }).waitFor();
    await action.page.getByRole('button', { name: 'รับทราบ' }).click();
    assert.equal(await toggle.isChecked(), true);
    await action.page.screenshot({ path: path.join(output, 'enabled-desktop.png') });
    await toggle.focus();
    await action.page.keyboard.press('Space');
    await action.page.getByRole('dialog', { name: 'ปิดการแจ้งเตือนแล้ว' }).waitFor();
    await action.page.getByRole('button', { name: 'รับทราบ' }).click();
    assert.equal(await toggle.isChecked(), false);
    checks.push('keyboard Space toggle, visible focus, reduced motion, authenticated server requests, failure/rollback/retry, success and unsubscribe dialogs');
    await action.context.close();
    assert.deepEqual(errors, []);
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ checks, runtimeErrors: errors, isolatedFixturesOnly: true }, null, 2));
    console.log(JSON.stringify({ checks, runtimeErrors: errors, screenshots: output }, null, 2));
  } catch (error) {
    if (currentPage && !currentPage.isClosed()) await currentPage.screenshot({ path: path.join(output, 'failure.png') }).catch(() => undefined);
    throw error;
  } finally {
    await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
}

main().catch((error) => { console.error(error); server.close(); process.exitCode = 1; });
