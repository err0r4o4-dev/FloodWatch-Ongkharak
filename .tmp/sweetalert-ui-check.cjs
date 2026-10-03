const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const root = path.resolve('.tmp/sweetalert-dist');
const output = path.resolve('.tmp/sweetalert-ui');
fs.mkdirSync(output, { recursive: true });
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.woff2': 'font/woff2' };
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  const target = path.resolve(root, '.' + pathname);
  const safe = target === root || target.startsWith(root + path.sep);
  const file = safe && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html');
  response.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
  response.setHeader('Cache-Control', 'no-store');
  response.end(fs.readFileSync(file));
});

(async () => {
  await new Promise((resolve) => server.listen(19174, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  let signedIn = true;
  let logoutMode = 'success';
  let completeLogout = null;
  let logoutCount = 0;
  const checks = [];
  const errors = [];
  const account = { id: 'ui-fixture', email: 'member@example.test', role: 'USER', createdAt: '2026-01-01T00:00:00Z' };
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, reducedMotion: 'reduce' });
    const page = await context.newPage();
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('dialog', async (dialog) => { errors.push('Unexpected native dialog: ' + dialog.message()); await dialog.dismiss(); });
    await page.route('**/api/**', async (route) => {
      const pathname = new URL(route.request().url()).pathname;
      const reply = (status, data) => route.fulfill({ status, contentType: 'application/json', headers: { 'X-Validation-Fixture': 'alerts-ui-only' }, ...(status === 204 ? {} : { body: JSON.stringify(data) }) });
      if (pathname.endsWith('/auth/logout')) {
        logoutCount++;
        completeLogout = async () => {
          signedIn = false;
          if (logoutMode === 'success') await reply(204);
          else await reply(503, { error: { code: 'UNAVAILABLE', message: 'Fixture logout failure' } });
          completeLogout = null;
        };
        return;
      }
      if (pathname.endsWith('/auth/refresh')) return signedIn ? reply(200, { accessToken: 'isolated-ui-fixture', expiresIn: 300, user: account }) : reply(401, { error: { code: 'UNAUTHORIZED' } });
      if (pathname.endsWith('/auth/me')) return reply(200, { user: account });
      if (pathname.endsWith('/auth/google/config')) return reply(200, { enabled: false });
      if (pathname.endsWith('/alerts')) return reply(200, { items: [], limit: 20, offset: 0 });
      return reply(503, { error: { code: 'UNAVAILABLE', message: 'Unavailable in isolated alert test' } });
    });
    await page.route('https://**', (route) => route.abort());

    async function openLogout() {
      if (!await page.locator('.account-menu').evaluate((element) => element.open)) await page.getByLabel('เมนูบัญชี').click();
      await page.getByRole('button', { name: 'ออกจากระบบ', exact: true }).click();
      await page.getByRole('dialog', { name: 'ออกจากระบบหรือไม่?' }).waitFor();
    }

    await page.goto('http://127.0.0.1:19174/system-status');
    await openLogout();
    const cancel = page.getByRole('button', { name: 'อยู่ในระบบต่อ' });
    const confirm = page.getByRole('button', { name: 'ออกจากระบบ', exact: true });
    assert.equal(await cancel.evaluate((element) => element === document.activeElement), true);
    const cancelBounds = await cancel.boundingBox();
    const confirmBounds = await confirm.boundingBox();
    assert.ok(cancelBounds.x < confirmBounds.x);
    assert.ok(cancelBounds.height >= 44 && confirmBounds.height >= 44);
    assert.equal(await confirm.evaluate((element) => getComputedStyle(element).backgroundColor), 'rgb(128, 35, 63)');
    await page.screenshot({ path: path.join(output, 'confirm-desktop.png') });
    await page.keyboard.press('Escape');
    await page.locator('.fw-alert').waitFor({ state: 'hidden' });
    assert.equal(logoutCount, 0);
    await page.waitForFunction(() => document.activeElement?.textContent === 'ออกจากระบบ', { timeout: 2000 });
    assert.equal(await page.getByRole('button', { name: 'ออกจากระบบ', exact: true }).evaluate((element) => element === document.activeElement), true);
    checks.push('desktop reference style, cancel-first focus, cancel-left, 44px targets, Escape cancellation, restored trigger focus, zero API calls on cancel');

    for (const width of [320, 390, 768]) {
      await page.setViewportSize({ width, height: 844 });
      await openLogout();
      const bounds = await page.locator('.fw-alert').boundingBox();
      assert.ok(bounds.x >= 0 && bounds.x + bounds.width <= width);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
      await page.screenshot({ path: path.join(output, `confirm-${width}.png`) });
      await cancel.click();
      await page.locator('.fw-alert').waitFor({ state: 'hidden' });
    }
    assert.equal(logoutCount, 0);
    checks.push('320px, 390px, 768px responsive dialogs without horizontal overflow');

    await page.setViewportSize({ width: 1280, height: 900 });
    await openLogout();
    await page.keyboard.press('Tab');
    assert.equal(await confirm.evaluate((element) => element === document.activeElement), true);
    await page.keyboard.press('Enter');
    await page.getByRole('dialog', { name: 'กำลังดำเนินการ…' }).waitFor();
    await page.locator('.fw-alert__loader').waitFor();
    assert.equal(await cancel.isEnabled(), false);
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('.fw-alert').count(), 1);
    await page.screenshot({ path: path.join(output, 'loading-desktop.png') });
    assert.equal(logoutCount, 1);
    assert.ok(completeLogout);
    await completeLogout();
    await page.getByRole('dialog', { name: 'ออกจากระบบแล้ว' }).waitFor();
    await page.waitForFunction(() => getComputedStyle(document.querySelector('.fw-alert')).opacity === '1');
    await page.screenshot({ path: path.join(output, 'success-desktop.png') });
    await page.getByRole('button', { name: 'รับทราบ' }).click();
    await page.waitForURL('http://127.0.0.1:19174/');
    checks.push('keyboard confirm, blocked cancellation while loading, one API request, success acknowledgement and overview navigation');

    signedIn = true;
    logoutMode = 'failure';
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('http://127.0.0.1:19174/system-status');
    await openLogout();
    await confirm.click();
    await page.getByRole('dialog', { name: 'กำลังดำเนินการ…' }).waitFor();
    assert.ok(completeLogout);
    await completeLogout();
    await page.getByRole('dialog', { name: 'ยืนยันการออกจากระบบไม่สำเร็จ' }).waitFor();
    await page.waitForFunction(() => getComputedStyle(document.querySelector('.fw-alert')).opacity === '1');
    assert.equal(await page.getByText('ออกจากระบบแล้ว', { exact: true }).count(), 0);
    await page.screenshot({ path: path.join(output, 'error-mobile.png') });
    await page.getByRole('button', { name: 'รับทราบ' }).click();
    checks.push('mobile API failure message without false success');

    await page.goto('http://127.0.0.1:19174/forgot-password');
    await page.getByLabel('อีเมล', { exact: true }).fill('fixture@example.test');
    await page.getByRole('button', { name: 'ส่งรหัส OTP', exact: true }).click();
    await page.getByLabel('รหัส OTP', { exact: true }).fill('000000');
    await page.getByRole('button', { name: 'ยืนยันรหัส OTP', exact: true }).click();
    await page.getByLabel('รหัสผ่านใหม่', { exact: true }).fill('fixture-only-password');
    await page.getByLabel('ยืนยันรหัสผ่านใหม่', { exact: true }).fill('fixture-only-password');
    await page.getByRole('button', { name: 'เปลี่ยนรหัสผ่าน', exact: true }).click();
    await page.getByRole('dialog', { name: 'โหมดสาธิตเท่านั้น' }).waitFor();
    assert.equal(await page.getByText('ขั้นตอนนี้เป็นเพียงการสาธิต และไม่มีการเปลี่ยนรหัสผ่านจริง', { exact: true }).count(), 1);
    await page.screenshot({ path: path.join(output, 'info-mobile.png') });
    await page.getByRole('button', { name: 'รับทราบ' }).click();
    checks.push('informational dialog honestly identifies demo-only password recovery');

    signedIn = true;
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('http://127.0.0.1:19174/system-status');
    await page.evaluate(() => { document.documentElement.style.fontSize = '200%'; });
    await openLogout();
    assert.equal(await page.locator('.fw-alert').evaluate((element) => element.scrollWidth <= element.clientWidth), true);
    await cancel.scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(output, 'confirm-mobile-200-percent.png') });
    assert.equal(await page.locator('.fw-alert').evaluate((element) => getComputedStyle(element).animationName), 'none');
    await cancel.click();
    checks.push('200% root text reflow in dialog and reduced motion');

    assert.deepEqual(errors, []);
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ checks, errors, logoutCount, fixtures: 'isolated, no live flood or production services' }, null, 2));
    console.log('PASS: ' + checks.join('; '));
  } finally {
    await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); server.close(); process.exitCode = 1; });
