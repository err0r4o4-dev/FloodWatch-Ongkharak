const fs = require('node:fs/promises');
const path = require('node:path');
const assert = require('node:assert/strict');

async function main() {
  const target = await (await fetch('http://127.0.0.1:9333/json/new?about:blank', { method: 'PUT' })).json();
  const socket = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, { once: true });
    socket.addEventListener('error', reject, { once: true });
  });
  let sequence = 0;
  const pending = new Map();
  const checks = [];
  const errors = [];
  let canceledRequests = 0;
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++sequence;
    const timeout = setTimeout(() => { pending.delete(id); reject(new Error('Timeout: ' + method)); }, 15000);
    pending.set(id, { resolve, reject, timeout });
    socket.send(JSON.stringify({ id, method, params }));
  });
  socket.addEventListener('message', ({ data }) => {
    const message = JSON.parse(data);
    if (message.id) {
      const entry = pending.get(message.id);
      if (!entry) return;
      clearTimeout(entry.timeout);
      pending.delete(message.id);
      if (message.error) entry.reject(new Error(JSON.stringify(message.error))); else entry.resolve(message.result);
    } else if (message.method === 'Runtime.exceptionThrown') errors.push(message.params.exceptionDetails.text);
    else if (message.method === 'Fetch.requestPaused') {
      const isCollection = /\/api\/v1\/alerts(\?|$)/.test(message.params.request.url);
      send('Fetch.fulfillRequest', {
        requestId: message.params.requestId, responseCode: isCollection ? 200 : 503,
        responseHeaders: [{ name: 'Content-Type', value: 'application/json' }],
        body: Buffer.from(JSON.stringify(isCollection ? { items: [], limit: 20, offset: 0 } : { error: 'Unavailable in isolated navbar UI test' })).toString('base64')
      }).catch((error) => {
        if (error.message.includes('Invalid InterceptionId')) canceledRequests++;
        else errors.push(error.message);
      });
    }
  });
  const evaluate = async (expression) => {
    const result = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
    if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails));
    return result.result.value;
  };
  const wait = async (expression) => {
    for (let attempt = 0; attempt < 60; attempt++) {
      if (await evaluate(`Boolean(${expression})`)) return;
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    await screenshot('navbar-ui-failure');
    throw new Error('Not ready: ' + expression + '\n' + await evaluate(`JSON.stringify({ path: location.pathname, content: document.querySelector('#main-content')?.innerText, classes: [...document.querySelectorAll('[class*="map-data"]')].map(element => element.className) })`));
  };
  const screenshot = async (name) => {
    const result = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
    await fs.writeFile(path.join(__dirname, name + '.png'), Buffer.from(result.data, 'base64'));
  };
  const metrics = () => evaluate(`(() => {
    const nav = document.querySelector('.bottom-navigation');
    const visible = element => getComputedStyle(element).display !== 'none';
    const bounds = element => { const rect = element.getBoundingClientRect(); return { left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, width: rect.width, height: rect.height }; };
    return {
      width: innerWidth, height: innerHeight, rootFontSize: getComputedStyle(document.documentElement).fontSize, overflow: document.documentElement.scrollWidth > innerWidth,
      navVisible: visible(nav), sidebarVisible: visible(document.querySelector('.desktop-sidebar')), nav: bounds(nav),
      padding: parseFloat(getComputedStyle(document.querySelector('.shell-main')).paddingBottom),
      links: [...nav.querySelectorAll('a')].map(link => ({ text: link.textContent, href: link.getAttribute('href'), current: link.getAttribute('aria-current'), ...bounds(link), label: bounds(link.lastElementChild) }))
    };
  })()`);
  try {
    await send('Page.enable');
    await send('Runtime.enable');
    await send('Fetch.enable', { patterns: [{ urlPattern: '*/api/*' }] });
    for (const [width, height, mobile] of [[1024, 1366, true], [320, 740, true], [390, 844, true], [768, 1024, true], [820, 1180, true], [1180, 820, true], [1366, 1024, true], [1280, 800, false], [1440, 900, false]]) {
      await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile });
      await send('Emulation.setTouchEmulationEnabled', { enabled: mobile, maxTouchPoints: mobile ? 5 : 1 });
      await send('Page.navigate', { url: 'http://127.0.0.1:5173/announcements' });
      await wait(`document.querySelector('.bottom-navigation') && document.fonts.status === 'loaded' && document.body.innerText.includes('ยังไม่มีประกาศที่เผยแพร่')`);
      const result = await metrics();
      checks.push({ viewport: `${width}x${height}`, mobile, ...result });
      await screenshot(`navbar-${width}-${mobile ? 'touch' : 'desktop'}`);
      assert.equal(result.overflow, false, `${width}: horizontal overflow`);
      assert.equal(result.navVisible, mobile, `${width}: bottom navigation visibility`);
      assert.equal(result.sidebarVisible, !mobile, `${width}: sidebar visibility`);
      assert.deepEqual(result.links.map(link => link.text), ['ภาพรวม', 'ประกาศ', 'พื้นที่เสี่ยง', 'แผนที่', 'รายงาน']);
      if (!mobile) continue;
      assert.ok(result.nav.width <= 770, `${width}: tablet navigation must be width-limited`);
      assert.ok(Math.abs((result.nav.left + result.nav.right) / 2 - width / 2) < 1, `${width}: centered navigation`);
      assert.ok(result.padding >= height - result.nav.top + 12, `${width}: content clearance includes floating nav and gap`);
      for (const link of result.links) {
        assert.ok(link.width >= 44 && link.height >= 44, `${width}: touch target ${link.text}`);
        assert.ok(link.label.left >= link.left - 1 && link.label.right <= link.right + 1 && link.label.bottom <= link.bottom, `${width}: label fits ${link.text}`);
      }
      assert.equal(result.links.filter(link => link.current === 'page').length, 1);
      assert.equal(result.links.find(link => link.current === 'page').href, '/announcements');
    }
    await send('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: true });
    await send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 5 });
    await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] });
    await send('Page.navigate', { url: 'http://127.0.0.1:5173/announcements' });
    await wait(`document.querySelector('.bottom-navigation') && document.fonts.status === 'loaded'`);
    await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 });
    await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 });
    await evaluate(`document.querySelector('.bottom-navigation a').focus()`);
    for (const href of ['/', '/announcements', '/risk', '/map', '/reports']) {
      const focus = await evaluate(`(() => { const link = document.activeElement; return { href: link.getAttribute('href'), visible: link.matches(':focus-visible'), outline: getComputedStyle(link).outlineStyle, duration: parseFloat(getComputedStyle(link).transitionDuration) }; })()`);
      assert.equal(focus.href, href);
      assert.equal(focus.visible, true);
      assert.notEqual(focus.outline, 'none');
      assert.ok(focus.duration <= 0.001, 'Reduced motion disables navigation transitions');
      await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 });
      await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 });
    }
    await evaluate(`document.querySelector('.bottom-navigation a[href="/map"]').focus()`);
    await screenshot('navbar-keyboard-focus');
    await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
    await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
    await wait(`location.pathname === '/map' && document.querySelector('.bottom-navigation a[aria-current="page"]').getAttribute('href') === '/map'`);
    await wait(`document.querySelector('.map-data-status [role="alert"]')`);
    await screenshot('navbar-map-unavailable');
    await evaluate(`document.querySelector('.map-tool-button[aria-controls="map-information-panel"]').click()`);
    await wait(`document.querySelector('#map-information-panel')`);
    const panel = await evaluate(`(() => { const panel = document.querySelector('#map-information-panel').getBoundingClientRect(); const nav = document.querySelector('.bottom-navigation').getBoundingClientRect(); return { top: panel.top, bottom: panel.bottom, navTop: nav.top }; })()`);
    assert.ok(panel.bottom <= panel.navTop - 12, 'Map panel clears floating navigation');
    await screenshot('navbar-map-panel');
    await send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 });
    await send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 });
    await wait(`!document.querySelector('#map-information-panel')`);
    for (const href of ['/', '/announcements', '/risk', '/map', '/reports']) {
      const point = await evaluate(`(() => { const rect = document.querySelector('.bottom-navigation a[href="${href}"]').getBoundingClientRect(); return { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 }; })()`);
      await send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [point] });
      await send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
      await wait(`location.pathname === '${href}' && document.querySelector('.bottom-navigation a[aria-current="page"]').getAttribute('href') === '${href}'`);
    }
    await send('Page.navigate', { url: 'http://127.0.0.1:5173/announcements' });
    await wait(`document.querySelector('.bottom-navigation') && document.fonts.status === 'loaded'`);
    await evaluate(`document.documentElement.style.fontSize = '200%'`);
    await evaluate(`new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))`);
    const zoom = await metrics();
    assert.ok(zoom.nav.left >= 0 && zoom.nav.right <= 390, 'Navigation remains inside viewport with 200% text size');
    assert.ok(zoom.nav.top >= 0 && zoom.nav.bottom <= 844, 'Navigation stays vertically visible with 200% text size');
    assert.ok(zoom.padding >= 844 - zoom.nav.top + 12, '200% text size retains content clearance');
    for (const link of zoom.links) assert.ok(link.label.right <= link.right + 1 && link.label.bottom <= link.bottom, '200% text label fits: ' + link.text);
    await screenshot('navbar-text-200-percent');
    checks.push({ textSize: '200%', ...zoom });
    checks.push({ keyboard: 'all five links, visible focus, Enter route change', reducedMotion: 'passed', mapUnavailable: 'rendered' });
    assert.deepEqual(errors, [], 'No browser runtime errors');
    console.log(JSON.stringify({ checks: checks.map(check => ({ viewport: check.viewport, mobile: check.mobile, nav: check.nav, padding: check.padding, keyboard: check.keyboard, textSize: check.textSize })), errors, canceledRequests }, null, 2));
  } finally {
    await fs.writeFile(path.join(__dirname, 'navbar-ui-results.json'), JSON.stringify({ checks, errors, canceledRequests }, null, 2));
    socket.close();
    await fetch('http://127.0.0.1:9333/json/close/' + target.id);
  }
}

main().catch((error) => { console.error(error); process.exitCode = 1; });
