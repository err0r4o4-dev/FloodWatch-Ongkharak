const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');

(async () => {
  const output = path.resolve('.tmp/road-segments-live-ui');
  fs.mkdirSync(output, { recursive: true });
  const response = await fetch('http://localhost:5173/api/v1/report-areas?limit=100');
  assert.equal(response.status, 200);
  const payload = await response.json();
  const area = payload.items.find(item => item.areaType === 'ROAD_SEGMENT' && item.segmentActive);
  assert.ok(area, 'Real imported segment summary must be available');
  const [longitude, latitude] = area.location.coordinates;
  const bbox = [longitude - 0.002, latitude - 0.002, longitude + 0.002, latitude + 0.002].join(',');
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    for (const [width, height] of [[1440, 1000], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' });
      const page = await context.newPage();
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.goto('http://localhost:5173/map?bbox=' + encodeURIComponent(bbox));
      await page.locator('.map-marker--reports').first().waitFor({ timeout: 30000 });
      const closeWeather = page.getByRole('button', { name: 'ย่อการ์ดสภาพอากาศ' });
      if (await closeWeather.count()) await closeWeather.first().click();
      await page.locator('.map-page__workspace').scrollIntoViewIfNeeded();
      const marker = page.locator('.map-marker--reports').first();
      if (width < 600) await marker.press('Enter');
      else await marker.click({ force: true });
      const panel = page.locator('.map-side-panel');
      await panel.waitFor();
      await page.getByText('ประวัติสถานการณ์น้ำในช่วงถนนนี้').waitFor();
      await page.locator('.report-area-timeline li').first().waitFor({ timeout: 30000 });
      assert.equal(await page.locator('.report-area-timeline li').count(), 24);
      assert.equal(await page.getByRole('button', { name: 'รายการในขอบเขต' }).count(), 0);
      assert.ok(await page.locator('path.leaflet-interactive').count() > 0);
      const box = await panel.boundingBox();
      assert.ok(box && box.x >= 0 && box.x + box.width <= width + 1);
      await page.screenshot({ path: path.join(output, `map-${width}.png`) });
      await page.locator('.report-area-detail__update').click();
      await page.locator('input[name="report-road-segment"]').first().waitFor();
      await page.waitForFunction(() => document.querySelectorAll('input[name="report-road-segment"]').length > 1);
      const selected = page.locator('input[name="report-road-segment"][value]:not([value=""])').first();
      assert.ok(await selected.count());
      await selected.check();
      assert.equal(await selected.isChecked(), true);
      await page.screenshot({ path: path.join(output, `report-${width}.png`), fullPage: true });
      assert.deepEqual(errors, []);
      console.log(`PASS live map details, 24 timeline windows, explicit report selection ${width}x${height}; no report submitted`);
      await context.close();
    }
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
