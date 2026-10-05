const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('./google-login-ui-tools/node_modules/playwright');
const root = path.resolve('apps/web/dist');
const output = path.resolve('.tmp/road-segments-ui');
const now = new Date().toISOString();
const id = '11111111-1111-4111-8111-111111111111';
const area = { id, name: 'ช่วงถนนทดสอบ — ข้อมูลจำลอง', location: { type: 'Point', coordinates: [101.035,14.075] }, radiusM: 0, areaType: 'ROAD_SEGMENT', roadSegmentId: id, segmentVersion: 1, segmentActive: true, startLabel: 'แยกทดสอบ A', endLabel: 'แยกทดสอบ B', geometry: { type: 'LineString', coordinates: [[101.034,14.075],[101.036,14.075]] }, sourceType: 'COMMUNITY', depthBand: 'HIGH', previousDepthBand: 'UNKNOWN', trend: 'UNKNOWN', reportedTrend: 'UNKNOWN', freshness: 'FRESH', summaryReason: 'PROVISIONAL', latestObservedAt: now, computedAt: now, windowStart: new Date(Date.now()-3600000).toISOString(), windowEnd: now, reportCount: 1, sampleCount: 1, totalReportCount: 1, reportIds: [] };
const server = http.createServer((request,response) => {
  const target = path.resolve(root, '.'+decodeURIComponent(new URL(request.url,'http://localhost').pathname));
  const file = target.startsWith(root+path.sep) && fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root,'index.html');
  response.setHeader('Content-Type', { '.html':'text/html','.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml' }[path.extname(file)] || 'application/octet-stream');
  response.end(fs.readFileSync(file));
});
(async () => {
  fs.mkdirSync(output,{recursive:true});
  await new Promise(resolve=>server.listen(19204,'127.0.0.1',resolve));
  const browser = await chromium.launch({channel:'msedge',headless:true});
  try {
    for (const [width,height] of [[1440,1000],[390,844]]) {
      const context = await browser.newContext({viewport:{width,height},reducedMotion:'reduce'});
      const page = await context.newPage();
      await page.route('**/api/v1/**', route => {
        const pathname = new URL(route.request().url()).pathname;
        const reply = body=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(body)});
        if (pathname.includes('/auth/')) return route.fulfill({status:401,contentType:'application/json',body:'{}'});
        if (pathname.endsWith('/events/stream')) return route.abort();
        if (pathname === '/api/v1/report-areas') return reply({items:[area],limit:100,offset:0});
        if (pathname === '/api/v1/report-areas/'+id) return reply({area,timeline:[]});
        return reply({items:[],limit:100,offset:0});
      });
      await page.route('**/tile.openstreetmap.org/**', route=>route.abort());
      await page.goto('http://127.0.0.1:19204/map');
      const closeWeather = page.getByRole('button', { name: /ปิด.*สภาพอากาศ/ });
      if (await closeWeather.count()) await closeWeather.first().click();
      const marker = page.locator('.leaflet-marker-icon').filter({has:page.locator('.map-marker')});
      const icons = page.locator('.leaflet-marker-icon');
      await icons.first().waitFor({timeout:15000});
      await page.locator('.map-page__workspace').scrollIntoViewIfNeeded();
      await page.waitForTimeout(300);
      if (width < 600) await icons.first().press('Enter');
      else await icons.first().click({force:true});
      await page.locator('.map-side-panel').waitFor();
      assert.equal(await page.getByRole('button',{name:'รายการในขอบเขต'}).count(),0);
      assert.ok(await page.getByText('ประวัติสถานการณ์น้ำในช่วงถนนนี้').count());
      const box = await page.locator('.map-side-panel').boundingBox();
      assert.ok(box && box.x>=0 && box.x+box.width<=width+1);
      await page.screenshot({path:path.join(output,`map-${width}.png`)});
      await page.keyboard.press('Escape');
      await context.close();
      console.log(`PASS rendered segment details ${width}x${height}`);
    }
  } finally { await browser.close(); server.close(); }
})().catch(error=>{console.error(error);server.close();process.exitCode=1;});
