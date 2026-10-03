const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const output = path.resolve('.tmp/auth-flow-ui');
const bundle = path.resolve('.tmp/auth-flow-dist');
const calls = [];
const account = { id: '00000000-0000-4000-8000-000000000001', email: 'fixture@example.test', username: 'fixture_user', role: 'USER', createdAt: '2026-01-01T00:00:00Z' };
const server = http.createServer(async (request, response) => {
  const url = new URL(request.url, 'http://localhost');
  response.setHeader('Cache-Control', 'no-store');
  if (url.pathname.startsWith('/api/')) {
    response.setHeader('Content-Type', 'application/json');
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const input = chunks.length ? JSON.parse(Buffer.concat(chunks).toString()) : {};
    calls.push({ path: url.pathname, input });
    const scenario = new URL(request.headers.referer || 'http://localhost').searchParams.get('scenario');
    const error = (status, code) => { response.writeHead(status); response.end(JSON.stringify({ error: { code, message: code } })); };
    const send = (status, value) => { response.writeHead(status); response.end(value === undefined ? undefined : JSON.stringify(value)); };
    if (url.pathname.endsWith('/google/config')) return send(200, { enabled: false });
    if (url.pathname.endsWith('/me') && request.headers.authorization === 'Bearer fixture-access-token') return send(200, { user: account });
    if (url.pathname.endsWith('/refresh') || url.pathname.endsWith('/me')) return error(401, 'UNAUTHENTICATED');
    if (url.pathname.endsWith('/register')) { assert.deepEqual(Object.keys(input).sort(), ['email', 'password', 'username']); return send(201, { user: account }); }
    if (url.pathname.endsWith('/login')) {
      if (scenario === 'register-login-error') return error(503, 'SERVICE_UNAVAILABLE');
      if (input.password === 'wrong long passphrase') return error(401, 'INVALID_CREDENTIALS');
      return send(200, { user: account, accessToken: 'fixture-access-token', expiresIn: 900 });
    }
    if (url.pathname.endsWith('/request-otp')) {
      if (scenario === 'service-error') return error(503, 'RECOVERY_UNAVAILABLE');
      await new Promise((resolve) => setTimeout(resolve, 150));
      return send(202, { challengeId: '00000000-0000-4000-8000-000000000002', expiresIn: 600, resendAfter: 60, message: 'หากอีเมลนี้มีบัญชีที่รองรับการตั้งรหัสผ่านใหม่ คุณสามารถใช้รหัส OTP ที่ส่งไปเพื่อดำเนินการต่อได้' });
    }
    if (url.pathname.endsWith('/verify-otp')) {
      if (input.otp !== '123456') return error(400, 'INVALID_OTP');
      return send(200, { resetToken: 'fixture-reset-only-token', expiresIn: 300 });
    }
    if (url.pathname.endsWith('/password/reset')) {
      assert.deepEqual(Object.keys(input).sort(), ['password', 'resetToken']);
      if (scenario === 'expired') return error(400, 'INVALID_RESET_TOKEN');
      return send(204);
    }
    return error(503, 'ISOLATED_UI_FIXTURE');
  }
  const pathname = decodeURIComponent(url.pathname);
  const file = pathname.startsWith('/assets/') || pathname === '/flood-watch-mark.svg' ? path.join(bundle, pathname) : path.join(bundle, 'index.html');
  if (!file.startsWith(bundle + path.sep)) { response.writeHead(404); response.end(); return; }
  try {
    response.setHeader('Content-Type', file.endsWith('.js') ? 'application/javascript' : file.endsWith('.css') ? 'text/css' : file.endsWith('.svg') ? 'image/svg+xml' : 'text/html');
    response.end(fs.readFileSync(file));
  } catch { response.writeHead(404); response.end(); }
});

(async () => {
  fs.mkdirSync(output, { recursive: true });
  await new Promise((resolve) => server.listen(19176, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  const results = [];
  try {
    for (const width of [1280, 390, 320]) {
      const context = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: 'reduce' });
      const page = await context.newPage();
      await page.goto('http://127.0.0.1:19176/register?returnTo=%2Fmap', { waitUntil: 'networkidle' });
      await page.getByRole('heading', { name: 'สมัครสมาชิก', exact: true }).waitFor();
      assert.equal(await page.getByLabel('ชื่อผู้ใช้งาน', { exact: true }).count(), 1);
      await page.screenshot({ path: path.join(output, `register-${width}.png`), fullPage: true });
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
      await page.goto('http://127.0.0.1:19176/forgot-password?returnTo=%2Fmap', { waitUntil: 'networkidle' });
      await page.getByLabel('อีเมล', { exact: true }).fill(account.email);
      await page.getByRole('button', { name: 'ส่งรหัส OTP', exact: true }).focus();
      const before = calls.filter((entry) => entry.path.endsWith('/request-otp')).length;
      await page.keyboard.press('Enter');
      await page.getByRole('heading', { name: 'ยืนยันรหัส OTP', exact: true }).waitFor();
      assert.equal(calls.filter((entry) => entry.path.endsWith('/request-otp')).length, before + 1);
      assert.equal(await page.getByRole('heading', { name: 'ยืนยันรหัส OTP', exact: true }).evaluate((element) => element === document.activeElement), true);
      assert.ok((await page.getByRole('button', { name: 'ยืนยันรหัส OTP', exact: true }).boundingBox()).height >= 44);
      await page.screenshot({ path: path.join(output, `otp-${width}.png`), fullPage: true });
      await page.getByLabel('รหัส OTP', { exact: true }).fill('111111');
      await page.getByRole('button', { name: 'ยืนยันรหัส OTP', exact: true }).click();
      await page.getByRole('alert').filter({ hasText: 'รหัส OTP ไม่ถูกต้อง' }).waitFor();
      await page.screenshot({ path: path.join(output, `otp-error-${width}.png`), fullPage: true });
      await page.getByLabel('รหัส OTP', { exact: true }).fill('123456');
      await page.getByRole('button', { name: 'ยืนยันรหัส OTP', exact: true }).click();
      await page.getByRole('heading', { name: 'ตั้งรหัสผ่านใหม่', exact: true }).waitFor();
      assert.ok((await page.getByRole('button', { name: 'เปลี่ยนรหัสผ่าน', exact: true }).boundingBox()).height >= 44);
      await page.getByLabel('รหัสผ่านใหม่', { exact: true }).fill('replacement long passphrase');
      await page.getByLabel('ยืนยันรหัสผ่านใหม่', { exact: true }).fill('different long passphrase');
      const resets = calls.filter((entry) => entry.path.endsWith('/password/reset')).length;
      await page.getByRole('button', { name: 'เปลี่ยนรหัสผ่าน', exact: true }).click();
      await page.getByRole('alert').filter({ hasText: 'รหัสผ่านไม่ตรงกัน' }).waitFor();
      assert.equal(calls.filter((entry) => entry.path.endsWith('/password/reset')).length, resets);
      await page.screenshot({ path: path.join(output, `reset-mismatch-${width}.png`), fullPage: true });
      await page.getByLabel('ยืนยันรหัสผ่านใหม่', { exact: true }).fill('replacement long passphrase');
      await page.getByRole('button', { name: 'เปลี่ยนรหัสผ่าน', exact: true }).focus();
      await page.keyboard.press('Enter');
      await page.getByRole('heading', { name: 'เปลี่ยนรหัสผ่านสำเร็จ', exact: true }).waitFor();
      assert.ok((await page.getByRole('button', { name: 'กลับไปหน้าเข้าสู่ระบบ', exact: true }).boundingBox()).height >= 44);
      assert.ok(page.url().includes('/forgot-password'));
      assert.equal(await page.evaluate(() => localStorage.length + sessionStorage.length), 0);
      await page.screenshot({ path: path.join(output, `success-${width}.png`), fullPage: true });
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
      await page.getByRole('button', { name: 'กลับไปหน้าเข้าสู่ระบบ', exact: true }).click();
      await page.getByRole('heading', { name: 'เข้าสู่ระบบ', exact: true }).waitFor();
      assert.equal(new URL(page.url()).searchParams.get('returnTo'), '/map');
      await page.screenshot({ path: path.join(output, `login-${width}.png`), fullPage: true });
      results.push({ width, passed: ['register-fields', 'keyboard', 'step-focus', 'single-request', 'wrong-otp', 'password-mismatch-no-request', 'success-no-auto-redirect', 'no-grant-storage', 'returnTo', 'no-overflow'] });
      await context.close();
    }
    const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
    const page = await context.newPage();
    await page.goto('http://127.0.0.1:19176/forgot-password?scenario=service-error');
    await page.getByLabel('อีเมล', { exact: true }).fill(account.email);
    await page.getByRole('button', { name: 'ส่งรหัส OTP', exact: true }).click();
    await page.getByRole('alert').filter({ hasText: 'บริการกู้คืนรหัสผ่านไม่พร้อมใช้งาน' }).waitFor();
    assert.equal(await page.getByRole('heading', { name: 'ลืมรหัสผ่าน?', exact: true }).count(), 1);
    await page.screenshot({ path: path.join(output, 'smtp-unavailable-390.png'), fullPage: true });
    await page.goto('http://127.0.0.1:19176/forgot-password?scenario=expired');
    await page.getByLabel('อีเมล', { exact: true }).fill(account.email);
    await page.getByRole('button', { name: 'ส่งรหัส OTP', exact: true }).click();
    await page.getByRole('heading', { name: 'ยืนยันรหัส OTP', exact: true }).waitFor();
    await page.addStyleTag({ content: '.auth-page { font-size: 200%; } .auth-page input, .auth-page button, .auth-page p { font-size: inherit; }' });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.screenshot({ path: path.join(output, 'otp-enlarged-text-390.png'), fullPage: true });
    await page.getByLabel('รหัส OTP', { exact: true }).fill('123456');
    await page.getByRole('button', { name: 'ยืนยันรหัส OTP', exact: true }).click();
    await page.getByRole('heading', { name: 'ตั้งรหัสผ่านใหม่', exact: true }).waitFor();
    await page.getByLabel('รหัสผ่านใหม่', { exact: true }).fill('replacement long passphrase');
    await page.getByLabel('ยืนยันรหัสผ่านใหม่', { exact: true }).fill('replacement long passphrase');
    await page.getByRole('button', { name: 'เปลี่ยนรหัสผ่าน', exact: true }).click();
    await page.getByRole('heading', { name: 'ลืมรหัสผ่าน?', exact: true }).waitFor();
    await page.getByRole('alert').filter({ hasText: 'สิทธิ์ตั้งรหัสผ่านหมดอายุ' }).waitFor();
    assert.equal(await page.getByLabel('รหัสผ่านใหม่', { exact: true }).count(), 0);
    await page.screenshot({ path: path.join(output, 'expired-grant-390.png'), fullPage: true });
    await page.goto('http://127.0.0.1:19176/register?scenario=register-login-error');
    await page.getByLabel('อีเมล', { exact: true }).fill(account.email);
    await page.getByLabel('ชื่อผู้ใช้งาน', { exact: true }).fill('Fixture_User');
    await page.getByLabel('รหัสผ่าน', { exact: true }).fill('original long passphrase');
    await page.getByRole('button', { name: 'สมัครสมาชิก', exact: true }).click();
    await page.locator('.form-error').filter({ hasText: 'สมัครสมาชิกสำเร็จแล้ว แต่เข้าสู่ระบบอัตโนมัติไม่สำเร็จ' }).waitFor();
    await page.locator('.swal2-confirm').click();
    await page.locator('.swal2-title').filter({ hasText: 'เข้าสู่ระบบไม่สำเร็จ' }).waitFor();
    await page.locator('.swal2-confirm').click();
    await page.locator('.swal2-container').waitFor({ state: 'detached' });
    await page.screenshot({ path: path.join(output, 'register-login-failure-390.png'), fullPage: true });
    assert.equal(await page.getByRole('link', { name: 'เข้าสู่ระบบด้วยบัญชีที่สร้างไว้' }).count(), 1);
    await page.goto('http://127.0.0.1:19176/login?returnTo=%2Fnotifications');
    await page.getByLabel('อีเมล หรือ ชื่อผู้ใช้งาน', { exact: true }).fill(account.username);
    await page.getByLabel('รหัสผ่าน', { exact: true }).fill('wrong long passphrase');
    await page.getByRole('button', { name: /เข้าสู่ระบบ/, exact: false }).click();
    await page.locator('.form-error').filter({ hasText: 'อีเมล/ชื่อผู้ใช้งานหรือรหัสผ่านไม่ถูกต้อง' }).waitFor();
    await page.locator('.swal2-confirm').click();
    await page.locator('.swal2-container').waitFor({ state: 'detached' });
    await page.screenshot({ path: path.join(output, 'username-login-error-390.png'), fullPage: true });
    await page.getByLabel('รหัสผ่าน', { exact: true }).fill('original long passphrase');
    await page.getByRole('button', { name: /เข้าสู่ระบบ/, exact: false }).focus();
    await page.keyboard.press('Enter');
    await page.waitForURL('**/notifications');
    const latestLogin = calls.filter((entry) => entry.path.endsWith('/login')).at(-1);
    assert.equal(latestLogin.input.email, account.username);
    results.push({ isolatedErrors: ['SMTP-unavailable-stays-step1', 'expired-grant-restarts-without-password-inputs', 'register-success-login-failure-honest', 'username-login-error-then-safe-destination'], accessibility: ['44px-primary-targets', '200%-auth-text-no-overflow', 'reduced-motion'] });
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify(results, null, 2));
    console.log(JSON.stringify(results));
    await context.close();
  } finally { await browser.close(); await new Promise((resolve) => server.close(resolve)); }
})().catch((error) => { console.error(error); server.close(); process.exitCode = 1; });
