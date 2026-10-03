const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

const output = path.resolve('.tmp/google-login-ui');
fs.mkdirSync(output, { recursive: true });
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname;
  if (pathname.startsWith('/api/')) {
    response.setHeader('Content-Type', 'application/json');
    response.setHeader('Cache-Control', 'no-store');
    response.setHeader('X-Validation-Fixture', 'google-ui-only');
    if (pathname.endsWith('/google/config')) {
      if ((request.headers.referer || '').includes('scenario=error')) {
        response.writeHead(503); response.end(JSON.stringify({ error: { code: 'UNAVAILABLE' } })); return;
      }
      response.end(JSON.stringify({ enabled: !(request.headers.referer || '').includes('scenario=disabled') })); return;
    }
    if (pathname.endsWith('/google/link')) {
      let body = '';
      request.on('data', (chunk) => { body += chunk; });
      request.on('end', () => {
        assert.deepEqual(Object.keys(JSON.parse(body)), ['password']);
        response.writeHead(429); response.end(JSON.stringify({ error: { code: 'GOOGLE_LINK_LIMIT' } }));
      });
      return;
    }
    response.writeHead(503); response.end(JSON.stringify({ error: { code: 'FIXTURE_ANONYMOUS' } })); return;
  }
  const upstream = http.request({ hostname: '127.0.0.1', port: 5173, path: request.url, method: request.method, headers: { ...request.headers, host: 'localhost:5173' } }, (incoming) => { response.writeHead(incoming.statusCode, incoming.headers); incoming.pipe(response); });
  upstream.on('error', () => { response.writeHead(502); response.end('Development frontend unavailable'); });
  request.pipe(upstream);
});

(async () => {
  await new Promise((resolve) => server.listen(19173, '127.0.0.1', resolve));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await context.newPage();
    await page.goto('http://127.0.0.1:19173/login?scenario=enabled', { waitUntil: 'networkidle' });
    const google = page.getByRole('button', { name: 'Continue with Google' });
    await google.waitFor();
    assert.equal(await google.isEnabled(), true);
    await google.focus();
    assert.equal(await google.evaluate((element) => element === document.activeElement), true);
    const target = await google.boundingBox();
    assert.ok(target.height >= 44, 'Google action must have a 44px touch target');
    await page.screenshot({ path: path.join(output, 'login-desktop-enabled.png'), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('http://127.0.0.1:19173/login?scenario=disabled', { waitUntil: 'networkidle' });
    await page.getByText('Google Login ยังไม่ได้ตั้งค่า').waitFor();
    assert.equal(await google.isEnabled(), false);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.screenshot({ path: path.join(output, 'login-mobile-disabled.png'), fullPage: true });
    await page.goto('http://127.0.0.1:19173/login?scenario=error', { waitUntil: 'networkidle' });
    await page.getByText('ตรวจสอบ Google Login ไม่สำเร็จ ใช้อีเมลและรหัสผ่านได้ตามปกติ').waitFor();
    await page.screenshot({ path: path.join(output, 'login-mobile-unavailable.png'), fullPage: true });
    await page.goto('http://127.0.0.1:19173/auth/google/link', { waitUntil: 'networkidle' });
    assert.equal(await page.getByRole('main').count(), 1);
    await page.getByLabel('รหัสผ่านบัญชีเดิม').fill('fixture-wrong-password');
    await page.getByLabel('รหัสผ่านบัญชีเดิม').focus();
    await page.keyboard.press('Tab');
    const confirm = page.getByRole('button', { name: 'ยืนยันและเชื่อม Google' });
    assert.equal(await confirm.evaluate((element) => element === document.activeElement), true);
    await page.keyboard.press('Enter');
    await page.getByRole('alert').waitFor();
    assert.equal(await confirm.isEnabled(), false);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.screenshot({ path: path.join(output, 'link-mobile-attempt-error.png'), fullPage: true });
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto('http://127.0.0.1:19173/auth/complete?returnTo=/reports', { waitUntil: 'networkidle' });
    await page.getByRole('alert').waitFor();
    await page.screenshot({ path: path.join(output, 'completion-desktop-error.png'), fullPage: true });
    console.log('PASS: isolated mock-only UI, desktop/mobile layouts, enabled/disabled/unavailable Google, linking keyboard and attempt failure, completion bootstrap error, no mobile horizontal overflow.');
  } finally { await browser.close(); server.close(); }
})().catch((error) => { console.error(error.message); server.close(); process.exitCode = 1; });
