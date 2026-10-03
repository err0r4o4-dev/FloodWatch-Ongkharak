const { chromium } = require('./google-login-ui-tools/node_modules/playwright');
const fs = require('node:fs/promises');
const assert = require('node:assert/strict');

async function main() {
  const browser = await chromium.connectOverCDP('http://127.0.0.1:9333');
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = [];
  const checks = [];
  page.on('pageerror', error => errors.push(error.message));
  let scenario = 'data';
  const source = { id: 'test-source', code: 'ui-fixture', name: 'ข้อมูลจำลองสำหรับตรวจ UI เท่านั้น', sourceType: 'OFFICIAL' };
  const risk = { id: 'risk-ui', name: 'พื้นที่จำลองสำหรับตรวจ UI', description: 'ไม่ใช่ข้อมูลสถานการณ์จริง', source, riskLevel: 'HIGH', effectiveRiskLevel: 'HIGH', freshness: 'FRESH', validFrom: '2026-10-03T00:00:00Z', validTo: '2026-10-04T00:00:00Z', geometry: { type: 'MultiPolygon', coordinates: [[[[100.97,14.09],[101.01,14.09],[101.01,14.13],[100.97,14.13],[100.97,14.09]]]] } };
  await context.route('**/api/**', async route => {
    const pathname = new URL(route.request().url()).pathname;
    if (!/\/api\/v1\/(risk-areas|water-stations|flood-events|roads|reports)$/.test(pathname)) return route.fulfill({ status: 503, json: { error: 'Isolated UI test only' } });
    if (scenario === 'error' && pathname.endsWith('/risk-areas')) return route.fulfill({ status: 503, json: { error: 'Unavailable in UI test' } });
    const items = scenario !== 'empty' && pathname.endsWith('/risk-areas') ? [risk, { ...risk, id: 'risk-stale', name: 'พื้นที่จำลองข้อมูลเก่า', freshness: 'STALE', source: { ...source, sourceType: 'COMMUNITY' }, geometry: { type: 'MultiPolygon', coordinates: [[[[101.02,14.1],[101.04,14.1],[101.04,14.12],[101.02,14.12],[101.02,14.1]]]] } }] : [];
    await route.fulfill({ status: 200, json: { items, limit: 100, offset: 0 } });
  });
  try {
    for (const [width, height] of [[1440,900],[820,1180],[390,844],[320,740]]) {
      await page.setViewportSize({ width, height });
      await page.goto('http://127.0.0.1:5173/map');
      await page.getByRole('button', { name: 'พื้นที่เสี่ยง', exact: true }).click();
      await page.getByRole('button', { name: /พื้นที่จำลองข้อมูลเก่า/ }).waitFor();
      await page.screenshot({ path: `.tmp/merged-risk-${width}.png` });
      const layout = await page.evaluate(() => {
        const box = selector => { const rect = document.querySelector(selector).getBoundingClientRect(); return { x: rect.x, y: rect.y, right: rect.right, bottom: rect.bottom, width: rect.width, height: rect.height }; };
        return { overflow: document.documentElement.scrollWidth > innerWidth, panel: box('.map-side-panel'), nav: box('.bottom-navigation'), tools: box('.map-page__tools'), controls: [...document.querySelectorAll('.map-page__tools button')].map(button => ({ text: button.textContent, width: button.getBoundingClientRect().width, height: button.getBoundingClientRect().height })), links: [...document.querySelectorAll('.bottom-navigation a')].map(link => ({ text: link.textContent, width: link.getBoundingClientRect().width, height: link.getBoundingClientRect().height })) };
      });
      checks.push({ width, layout });
      assert.equal(layout.overflow, false, `${width}: horizontal overflow`);
      assert.equal(layout.links.length, 4);
      assert.ok(layout.panel.x >= 0 && layout.panel.right <= width, `${width}: panel fits`);
      if (width <= 820) assert.ok(layout.panel.bottom <= layout.nav.y, `${width}: panel clears navigation`);
      for (const control of layout.controls) assert.ok(control.width >= 44 && control.height >= 44, `${width}: touch target ${control.text}`);
      await page.getByRole('button', { name: 'กรอง: ความเสี่ยงสูง', exact: true }).click();
      assert.equal(await page.getByRole('button', { name: /พื้นที่จำลองข้อมูลเก่า/ }).count(), 0);
      await page.getByRole('button', { name: 'กรอง: ไม่ทราบสถานะ', exact: true }).click();
      await page.getByRole('button', { name: /พื้นที่จำลองข้อมูลเก่า/ }).waitFor();
      await page.getByRole('button', { name: 'กรอง: ทั้งหมด', exact: true }).click();
      await page.getByRole('button', { name: /พื้นที่จำลองสำหรับตรวจ UI/ }).click();
      await page.getByRole('region', { name: 'ข้อมูลสรุปบนแผนที่' }).waitFor();
      await page.screenshot({ path: `.tmp/merged-risk-selected-${width}.png` });
      assert.equal(await page.getByRole('button', { name: /พื้นที่จำลองสำหรับตรวจ UI/ }).getAttribute('aria-pressed'), 'true');
      await page.getByRole('button', { name: 'ปิดแผงข้อมูล', exact: true }).focus();
      await page.keyboard.press('Escape');
      assert.equal(await page.getByRole('button', { name: 'พื้นที่เสี่ยง', exact: true }).evaluate(element => element === document.activeElement), true);
      checks.push({ width, filters: 'passed', popup: 'passed', escapeFocus: 'passed' });
    }
    await page.setViewportSize({ width: 390, height: 844 });
    for (const state of ['empty', 'error']) {
      scenario = state;
      await page.goto('http://127.0.0.1:5173/risk?risk=missing&bbox=100.97,14.09,101.04,14.13');
      await page.waitForURL('**/map?**');
      await page.getByText(state === 'empty' ? 'ไม่มีพื้นที่เสี่ยงในข้อมูลที่โหลด ไม่ได้หมายความว่าปลอดภัย' : 'ไม่สามารถโหลดพื้นที่เสี่ยงได้ · สถานะไม่ทราบ', { exact: true }).waitFor();
      assert.equal(new URL(page.url()).searchParams.get('risk'), 'missing');
      assert.equal(new URL(page.url()).searchParams.get('bbox'), '100.97,14.09,101.04,14.13');
      await page.screenshot({ path: `.tmp/merged-risk-${state}.png` });
      checks.push({ state, legacyRedirect: 'passed' });
    }
    assert.deepEqual(errors, []);
    console.log(JSON.stringify({ checks, errors }, null, 2));
  } finally {
    await fs.writeFile('.tmp/merged-risk-ui-results.json', JSON.stringify({ checks, errors }, null, 2));
    await context.close();
    await browser.close();
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
