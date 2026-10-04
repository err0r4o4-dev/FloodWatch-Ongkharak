const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');
const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/account-menu-ui');
const origin = 'http://127.0.0.1:19183';
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
const server = http.createServer((request, response) => {
  const target = path.join(root, decodeURIComponent(new URL(request.url, origin).pathname));
  const file = target.startsWith(root) && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});
(async () => {
  fs.mkdirSync(output, { recursive: true });
  await new Promise((resolve) => server.listen(19183, '127.0.0.1', resolve));
  let browser;
  let guest = false;
  let failSave = false;
  let saves = 0;
  let account = { id: 'synthetic-ui-account', email: 'verylongunbrokenemailaddress'.repeat(4) + '@example.test', username: 'long_username_for_layout_check_32', role: 'ADMIN', createdAt: '2026-10-04T00:00:00Z' };
  try {
    browser = await chromium.launch({ channel: 'msedge', headless: true });
    const context = await browser.newContext({ reducedMotion: 'reduce' });
    await context.route('**/api/v1/**', async (route) => {
      const url = new URL(route.request().url());
      if (guest) return route.fulfill({ status: 401, json: { error: { code: 'UNAUTHENTICATED', message: 'Isolated guest UI verification' } } });
      if (url.pathname.endsWith('/auth/refresh')) return route.fulfill({ json: { user: account, accessToken: 'synthetic-test-token', expiresIn: 900 } });
      if (url.pathname.endsWith('/auth/me')) {
        if (route.request().method() === 'PATCH') {
          saves++;
          if (failSave) return route.fulfill({ status: 409, json: { error: { code: 'USERNAME_EXISTS', message: 'Synthetic duplicate' } } });
          await new Promise((resolve) => setTimeout(resolve, 300));
          account = { ...account, username: route.request().postDataJSON().username };
        }
        return route.fulfill({ json: { user: account } });
      }
      return route.fulfill({ json: { items: [], limit: 100, offset: 0 } });
    });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    for (const width of [320, 390, 768, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      await page.goto(origin + '/system-status');
      await page.locator('.is-logged-in').waitFor();
      await page.getByLabel('เมนูบัญชี').click();
      await page.getByRole('button', { name: 'แก้ไข username' }).waitFor();
      const layout = await page.locator('.account-menu__content').evaluate((element) => {
        const box = element.getBoundingClientRect();
        return { left: box.left, right: box.right, scroll: element.scrollWidth, client: element.clientWidth, window: innerWidth, page: document.documentElement.scrollWidth, logoutColor: getComputedStyle(element.querySelector('.account-menu__logout')).color, targets: [...element.querySelectorAll('button,a')].map((action) => action.getBoundingClientRect().height) };
      });
      assert(layout.left >= 0 && layout.right <= width && layout.page <= width && layout.scroll <= layout.client, JSON.stringify(layout));
      assert(layout.targets.every((height) => height >= 44), JSON.stringify(layout));
      assert.equal(layout.logoutColor, 'rgb(128, 35, 63)');
      await page.screenshot({ path: path.join(output, `account-${width}.png`) });
      await page.getByRole('button', { name: 'แก้ไข username' }).click();
      await page.getByRole('textbox', { name: 'Username' }).fill(' NEW_USER ');
      assert(await page.getByRole('textbox', { name: 'Username' }).evaluate((element) => element === document.activeElement));
      await page.screenshot({ path: path.join(output, `edit-${width}.png`) });
      await page.getByRole('textbox', { name: 'Username' }).press('Enter');
      await page.getByRole('button', { name: 'กำลังบันทึก…' }).waitFor();
      assert(await page.getByRole('button', { name: 'ออกจากระบบ' }).isDisabled());
      await page.getByText('บันทึก username แล้ว', { exact: true }).waitFor();
      assert.equal(account.username, 'new_user');
      assert(await page.getByRole('button', { name: 'แก้ไข username' }).evaluate((element) => element === document.activeElement));
      await page.getByRole('button', { name: 'แก้ไข username' }).press('Escape');
      assert(await page.getByLabel('เมนูบัญชี').evaluate((element) => element === document.activeElement && !element.parentElement.open));
    }
    assert.equal(saves, 4);
    await page.setViewportSize({ width: 390, height: 900 });
    await page.evaluate(() => document.documentElement.style.fontSize = '200%');
    await page.getByLabel('เมนูบัญชี').click();
    const zoomLayout = await page.locator('.account-menu__content').evaluate((element) => {
      const box = element.getBoundingClientRect();
      return { left: box.left, right: box.right, scroll: element.scrollWidth, client: element.clientWidth };
    });
    assert(zoomLayout.left >= 0 && zoomLayout.right <= 390 && zoomLayout.scroll <= zoomLayout.client, JSON.stringify(zoomLayout));
    await page.screenshot({ path: path.join(output, 'zoom-mobile.png') });
    await page.getByLabel('เมนูบัญชี').press('Escape');
    await page.evaluate(() => document.documentElement.style.fontSize = '');
    await page.setViewportSize({ width: 390, height: 900 });
    await page.getByLabel('เมนูบัญชี').click();
    await page.getByRole('button', { name: 'แก้ไข username' }).click();
    await page.getByRole('textbox', { name: 'Username' }).fill('taken_name');
    failSave = true;
    await page.getByRole('button', { name: 'บันทึก', exact: true }).click();
    await page.getByRole('alert').filter({ hasText: 'username นี้ถูกใช้งานแล้ว' }).waitFor();
    assert.equal(await page.getByRole('textbox', { name: 'Username' }).inputValue(), 'taken_name');
    await page.screenshot({ path: path.join(output, 'duplicate-mobile.png') });
    await page.getByRole('button', { name: 'ยกเลิก' }).click();
    await page.getByRole('link', { name: 'รายงาน', exact: true }).click();
    assert.equal(await page.locator('.account-menu').getAttribute('open'), null);
    guest = true;
    await page.reload();
    await page.locator('.account-menu summary:not([aria-disabled])').waitFor();
    await page.getByLabel('เมนูบัญชี').focus();
    await page.keyboard.press('Enter');
    await page.locator('.account-menu__content').getByRole('link', { name: 'เข้าสู่ระบบ' }).waitFor();
    assert.equal(await page.locator('.account-menu__content a').count(), 1);
    await page.keyboard.press('Tab');
    assert(await page.locator('.account-menu__content a').evaluate((element) => element === document.activeElement));
    await page.screenshot({ path: path.join(output, 'guest-mobile.png') });
    assert.deepEqual(errors, []);
    console.log('PASS: 320/390/768/1440px, full long email, 44px targets, red logout, inline save, busy state, duplicate error, outside dismissal, guest menu and keyboard/Escape. Synthetic intercepted account only; no DB mutation.');
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
