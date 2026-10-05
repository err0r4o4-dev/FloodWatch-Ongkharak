const { chromium } = require('../google-login-ui-tools/node_modules/playwright');
(async () => {
 const { createServer } = await import('../../apps/web/node_modules/vite/dist/node/index.js');
 const server = await createServer({root:require('path').resolve('apps/web'),server:{host:'127.0.0.1',port:5175,strictPort:true}});
 await server.listen();
 const browser = await chromium.launch({ headless: true, executablePath:'C:/Users/Taitunnn/AppData/Local/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-win64/chrome-headless-shell.exe' });
 const page = await browser.newPage();
 await page.route('**/api/**', async route => {
  const path = new URL(route.request().url()).pathname;
  const account = { id:'123e4567-e89b-12d3-a456-426614174000', email:'ui-test@example.com', role:'ADMIN', createdAt:'2026-10-04T00:00:00Z' };
  const payload = path.endsWith('/auth/refresh') ? {user:account,accessToken:'ui-test-only',expiresIn:900} : path.endsWith('/auth/me') ? {user:account} : path.includes('manual-announcements') ? {items:[],total:0,limit:20,offset:0} : {items:[],total:0,limit:20,offset:0};
  await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(payload)});
 });
 for (const [width,height] of [[1440,1000],[390,844]]) {
  await page.setViewportSize({width,height});
  await page.goto('http://127.0.0.1:5175/admin/announcements');
  await page.getByRole('button',{name:'สร้างประกาศ',exact:true}).click();
  await page.getByLabel('หัวข้อ',{exact:true}).fill('ข้อมูลตัวอย่างสำหรับตรวจหน้าจอเท่านั้น');
  await page.getByRole('button',{name:'เพิ่มแหล่งอ้างอิง',exact:true}).evaluate(element => element.scrollIntoView({block:'center'}));
  await page.getByRole('button',{name:'เพิ่มแหล่งอ้างอิง',exact:true}).click();
  await page.getByRole('button',{name:'ดูตัวอย่าง',exact:true}).evaluate(element => element.scrollIntoView({block:'center'}));
  await page.getByRole('button',{name:'ดูตัวอย่าง',exact:true}).click();
  await page.getByText('ตัวอย่างประกาศ — ยังไม่เผยแพร่การเปลี่ยนแปลงนี้',{exact:true}).waitFor();
  await page.screenshot({path:`.tmp/manual-announcements-ui/admin-${width}.png`,fullPage:true});
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
  if (overflow) throw new Error(`Horizontal overflow at ${width}`);
  await page.goto('http://127.0.0.1:5175/announcements');
  await page.getByText('ยังไม่มีประกาศจากผู้ดูแล',{exact:true}).waitFor();
  await page.screenshot({path:`.tmp/manual-announcements-ui/public-${width}.png`,fullPage:true});
  console.log(`Verified admin draft/preview/reference and public empty states: ${width}x${height}`);
 }
 await browser.close();
 await server.close();
})().catch(error => { console.error(error.message); process.exit(1); });
