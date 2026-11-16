// 039: browser checks for the QA findings and the product owner's binding findings.
// Shared by scripts/test_qa039.mjs (dedicated matrix) and scripts/test_ux_entscheidungen.mjs (UX-S25 ff.).
// Every check leaves the page in the state it found it (panel closed, language unchanged).
import assert from 'node:assert/strict';

const box = r => r && {x: r.x, y: r.y, w: r.width, h: r.height};
export async function boxes(page, selector) {
  return page.evaluate(sel => [...document.querySelectorAll(sel)].filter(n => n.checkVisibility()).map(n => {
    const r = n.getBoundingClientRect();
    return {id: n.id || n.textContent.trim().slice(0, 24), x: r.x, y: r.y + scrollY, w: r.width, h: r.height};
  }), selector);
}
const t = (page, s) => page.evaluate(x => StromeaI18n.t(x), s);

export async function openPanel(page, which) {
  await page.evaluate(() => { if (document.querySelector('#mode-onsite') && !document.querySelector('#mode-onsite').checked) document.querySelector('#mode-onsite').click(); });
  if (!await page.locator('#' + which).isVisible() && await page.locator('#change-device').isVisible()) await page.locator('#change-device').click();
  await page.locator('#' + which).click();
  await page.locator(which === 'qr-toggle' ? '#qr-panel' : '#manual-device').waitFor({state: 'visible'});
}
export async function closePanel(page) {
  await page.evaluate(() => { const b = document.querySelector('#inspector-close'); if (b && b.checkVisibility()) b.click(); });
  await page.waitForFunction(() => document.querySelector('#qr-panel').hidden && document.querySelector('#manual-device').hidden);
}

// UX-S25 / Nutzer 1+2 / QA-01, QA-02, QA-13: equal-width QR actions, no jump on errors, centred small print, no Ctrl+V hint.
export async function qrPanel(page) {
  await openPanel(page, 'qr-toggle');
  try {
    const actions = '#qr-import button:not([hidden])';
    const before = await boxes(page, actions);
    assert(before.length >= 4, 'QR actions visible: ' + JSON.stringify(before.map(b => b.id)));
    const widths = before.map(b => Math.round(b.w));
    assert(Math.max(...widths) - Math.min(...widths) <= 1, 'all QR actions share one width: ' + JSON.stringify(before.map(b => [b.id, Math.round(b.w)])));
    const lefts = before.map(b => Math.round(b.x));
    assert(Math.max(...lefts) - Math.min(...lefts) <= 1, 'QR actions share one left edge');
    const small = async () => page.evaluate(() => [...document.querySelectorAll('#qr-panel .help, #qr-panel small, #qr-panel #qr-status')].filter(n => n.checkVisibility() && n.textContent.trim()).map(n => {
      const cs = getComputedStyle(n), r = n.getBoundingClientRect(), range = document.createRange(); range.selectNodeContents(n);
      const text = range.getBoundingClientRect(), parent = n.parentElement.getBoundingClientRect();
      // A ::before icon (e.g. ⚠) belongs to the centred line, but is not part of the text range.
      const before = getComputedStyle(n, '::before'), icon = before.content && !['none', 'normal'].includes(before.content) ? (() => { const c = document.createElement('canvas').getContext('2d'); c.font = before.font || cs.font; return c.measureText(before.content.replace(/^"|"$/g, '')).width; })() : 0;
      return {text: n.textContent.trim(), align: cs.textAlign, left: text.left - icon - parent.left, right: parent.right - text.right, width: r.width};
    }));
    for (const s of await small()) {
      assert.equal(s.align, 'center', 'small print centred: ' + s.text);
      assert(Math.abs(s.left - s.right) <= 3, 'small print horizontally centred (' + Math.round(s.left) + '/' + Math.round(s.right) + '): ' + s.text);
    }
    const panelText = await page.locator('#qr-panel').innerText();
    assert(!/Strg\s*\+\s*V|Ctrl\s*\+\s*V|⌘\s*V/i.test(panelText), 'no keyboard paste hint in the QR panel');
    // Invalid QR text: message appears, no action moves.
    const watched = actions + ', #qr-import label, #qr-import textarea';
    const stable = await boxes(page, watched);
    await page.locator('#qr-text').fill('falscher QR-Text');
    await page.locator('#qr-apply').click();
    await page.locator('#qr-status.qr-error').waitFor();
    const after = await boxes(page, watched);
    assert.deepEqual(after.map(b => [b.id, Math.round(b.x), Math.round(b.y), Math.round(b.w)]), stable.map(b => [b.id, Math.round(b.x), Math.round(b.y), Math.round(b.w)]), 'QR actions keep position and width when an error appears');
    for (const s of await small()) assert.equal(s.align, 'center', 'status centred: ' + s.text);
    // Camera without device (headless): localized message, no browser English, no jump.
    await page.locator('#qr-camera').click();
    await page.waitForFunction(() => /\S/.test(document.querySelector('#qr-status').textContent) && !document.querySelector('#qr-status').textContent.includes('…'));
    const camera = await page.locator('#qr-status').innerText();
    assert(!/Requested device|NotFoundError|NotAllowedError|Permission denied|not found/i.test(camera), 'QA-08 camera message without raw browser text: ' + camera);
    const afterCamera = await boxes(page, watched);
    assert.deepEqual(afterCamera.map(b => [b.id, Math.round(b.y)]), stable.map(b => [b.id, Math.round(b.y)]), 'no jump after camera error');
    await page.locator('#qr-text').fill('');
  } finally { await closePanel(page); }
}

// UX-S26 / Nutzer 4 / QA-03: "Im Netzwerk suchen", "QR-Code", "Manuell" in one line.
export async function pairMethods(page) {
  const labels = await page.evaluate(() => ['#qr-toggle', '#manual-toggle'].map(s => document.querySelector(s).textContent.trim()));
  assert.deepEqual(labels, [await t(page, 'QR-Code'), await t(page, 'Manuell')], 'short labels');
  const titles = await page.evaluate(() => ['#qr-toggle', '#manual-toggle'].map(s => document.querySelector(s).title));
  assert(titles.every(Boolean), 'details stay in the tooltip');
  const b = await boxes(page, '#discover, #qr-toggle, #manual-toggle');
  assert.equal(b.length, 3, 'three actions visible');
  const tops = b.map(x => Math.round(x.y + x.h / 2));
  assert(Math.max(...tops) - Math.min(...tops) <= 2, 'one line: ' + JSON.stringify(b.map(x => [x.id, Math.round(x.y), Math.round(x.h)])));
  for (const x of b) assert(x.h <= 48, 'label does not wrap: ' + x.id + ' ' + x.h);
  const overflow = await page.evaluate(() => ['#discover', '#qr-toggle', '#manual-toggle'].map(s => { const n = document.querySelector(s); return n.scrollWidth - n.clientWidth; }));
  assert(overflow.every(v => v <= 1), 'no clipped label');
  const right = await page.evaluate(() => document.querySelector('#step-1').getBoundingClientRect().right);
  assert(b.every(x => x.x + x.w <= right + 1), 'inside the card');
}

// UX-S30 / Nutzer 5 / QA-04: dictation button is a recognisable line icon in the common icon colour.
export async function micIcon(page) {
  await openPanel(page, 'manual-toggle');
  try {
    const v = await page.evaluate(() => {
      const b = document.querySelector('#dictate'), svg = b.querySelector('svg'), ref = document.querySelector('#change-device svg');
      return {visible: b.checkVisibility(), text: [...b.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join('').trim(), svg: !!svg,
        stroke: svg && [...svg.querySelectorAll('path,rect,line,circle')].every(n => (n.getAttribute('stroke') || '') === 'currentColor' && ['none', null].includes(n.getAttribute('fill'))),
        width: svg?.getAttribute('stroke-width') || svg?.querySelector('[stroke-width]')?.getAttribute('stroke-width'),
        color: getComputedStyle(b).color, ref: getComputedStyle(document.querySelector('#copy-ski')).color, name: b.getAttribute('aria-label'), tip: b.title,
        refStroke: ref?.querySelector('path')?.getAttribute('stroke')};
    });
    if (!v.visible && await page.evaluate(() => StromeaI18n.language() === 'de')) {
      // Review 039: never pass vacuously – emulate a browser with speech recognition and check again.
      await page.evaluate(() => { window.webkitSpeechRecognition = function () {}; window.dispatchEvent(new Event('stromea-language')); });
      assert(await page.locator('#dictate').isVisible(), 'dictation button appears with browser speech recognition');
      return micIcon(page);
    }
    if (!v.visible) return; // other languages: system dictation hint instead
    assert.equal(v.text, '', 'no emoji glyph as icon');
    assert(v.svg && v.stroke, 'SVG line icon in currentColor');
    assert.equal(v.color, v.ref, 'same colour as the other icon buttons');
    assert(v.name && v.tip, 'accessible name and tooltip');
  } finally { await closePanel(page); }
}

// Last visible element of the open sidebar after scrolling like a user (wheel over the sidebar).
async function lastVisible(page) {
  return page.evaluate(() => {
    const ins = document.querySelector('#detail-inspector');
    const all = [...ins.querySelectorAll('button,p,label,input,textarea,select,h3,a,video,li')].filter(n => n.checkVisibility() && n.getBoundingClientRect().height > 0 && (n.textContent.trim() || n.matches('input,textarea,select,video')));
    const last = all.reduce((a, n) => !a || n.getBoundingClientRect().bottom >= a.getBoundingClientRect().bottom ? n : a, null);
    const vv = window.visualViewport, bottom = vv ? Math.min(innerHeight, vv.offsetTop + vv.height) : innerHeight;
    const r = last.getBoundingClientRect();
    return {id: last.id || last.textContent.trim().slice(0, 30), top: r.top, bottom: r.bottom, viewport: bottom, drawer: ins.getBoundingClientRect().bottom};
  });
}
export async function scrollSidebar(page) {
  const b = await page.locator('#detail-inspector').boundingBox();
  const vp = page.viewportSize();
  await page.mouse.move(b.x + b.width / 2, Math.min(vp.height - 20, Math.max(b.y, 0) + 120));
  for (let i = 0; i < 40; i++) { await page.mouse.wheel(0, 240); }
  await page.waitForTimeout(250);
}

// UX-S27 / Nutzer 3: every sidebar scrolls to its last element (incl. mobile end spacing and keyboard via visualViewport).
export async function sidebarEnd(page, {keyboard = 0} = {}) {
  for (const which of ['qr-toggle', 'manual-toggle']) {
    await openPanel(page, which);
    try {
      if (which === 'qr-toggle') {
        await page.locator('#qr-text').fill('x');
        await page.locator('#qr-apply').click();
      }
      if (keyboard) {
        // Software keyboard: the visual viewport shrinks, the layout viewport does not (iOS Safari).
        await page.evaluate(h => {
          const vv = window.visualViewport, height = innerHeight - h;
          Object.defineProperty(vv, 'height', {configurable: true, get: () => height});
          vv.dispatchEvent(new Event('resize'));
        }, keyboard);
        await page.waitForTimeout(100);
      }
      await scrollSidebar(page);
      const v = await lastVisible(page);
      const mobile = page.viewportSize().width <= 1050;
      assert(v.bottom <= v.viewport + 0.5, which + ': last element ' + v.id + ' reachable (' + Math.round(v.bottom) + ' ≤ ' + Math.round(v.viewport) + ')');
      if (mobile) assert(v.viewport - v.bottom >= 56, which + ': mobile sheet keeps end spacing above browser toolbars/home indicator (' + Math.round(v.viewport - v.bottom) + ' px)');
      if (keyboard) assert(v.drawer <= v.viewport + 0.5, which + ': sheet ends above the keyboard');
    } finally {
      if (keyboard) await page.evaluate(() => { delete window.visualViewport.height; window.visualViewport.dispatchEvent(new Event('resize')); });
      if (which === 'qr-toggle') await page.locator('#qr-text').fill('');
      await closePanel(page);
    }
  }
}

// UX-S28 / Nutzer 6: native dialogs (file chooser, camera permission, confirm) never close the sidebar.
export async function nativeDialogs(page) {
  await openPanel(page, 'qr-toggle');
  try {
    const open = () => page.evaluate(() => !document.querySelector('#qr-panel').hidden && document.querySelector('#detail-inspector').checkVisibility());
    // File chooser: the browser opens a native window. The page loses focus, may be hidden and,
    // on mobile Safari, comes back through pageshow (bfcache) when the picker closes.
    const chooser = page.waitForEvent('filechooser');
    await page.locator('#qr-image-select').click();
    await chooser;
    await page.evaluate(async () => {
      window.dispatchEvent(new Event('blur'));
      Object.defineProperty(document, 'visibilityState', {configurable: true, get: () => 'hidden'});
      Object.defineProperty(document, 'hidden', {configurable: true, get: () => true});
      document.dispatchEvent(new Event('visibilitychange'));
      await new Promise(r => setTimeout(r, 50));
      delete document.visibilityState; delete document.hidden;
      document.dispatchEvent(new Event('visibilitychange'));
      window.dispatchEvent(new PageTransitionEvent('pageshow', {persisted: true}));
      window.dispatchEvent(new Event('focus'));
    });
    await page.waitForTimeout(900);
    assert(await open(), 'sidebar stays open after the file chooser');
    // Camera permission prompt: same focus sequence, then a denial.
    await page.evaluate(() => {
      navigator.mediaDevices.getUserMedia = () => new Promise((_, reject) => setTimeout(() => {
        window.dispatchEvent(new Event('blur'));
        window.dispatchEvent(new Event('focus'));
        reject(new DOMException('Permission denied', 'NotAllowedError'));
      }, 100));
    });
    await page.locator('#qr-camera').click();
    await page.waitForTimeout(600);
    assert(await open(), 'sidebar stays open after the camera permission prompt');
    // Reload while the sidebar is open (mobile browsers may discard a page behind a native picker).
    await page.locator('#qr-text').fill('SHIP;Entwurf');
    await page.reload();
    await page.waitForFunction(() => typeof state !== 'undefined' && !!state);
    await page.locator('#qr-panel').waitFor({state: 'visible'});
    assert.equal(await page.locator('#qr-text').inputValue(), 'SHIP;Entwurf', 'QR text survives');
    await page.locator('#qr-text').fill('');
  } finally { await closePanel(page); }
}

// UX-S29 / Nutzer 7 / QA-05: no network → warning card, every pairing action locked; practice mode works.
export async function noNetwork(page) {
  const v = await page.evaluate(() => {
    const card = document.querySelector('#network-warning');
    const ids = ['discover', 'qr-toggle', 'manual-toggle', 'manual-connect', 'pair', 'qr-apply', 'qr-camera', 'qr-image-select', 'qr-paste'];
    return {card: !!card && card.checkVisibility(), text: card?.innerText.trim() || '', cls: card?.className || '',
      disabled: Object.fromEntries(ids.map(id => [id, document.getElementById(id).disabled])),
      practice: !document.querySelector('#mode-simulator').disabled};
  });
  assert(v.card, 'warning card visible');
  assert(/warning/.test(v.cls), 'styled as warning');
  assert(v.text.length > 0 && v.text.length <= 140, 'short card text: ' + v.text);
  for (const [id, disabled] of Object.entries(v.disabled)) assert(disabled, id + ' locked without network');
  assert(v.practice, 'practice mode stays available');
}

// UX-S31 / QA-09, QA-10, A-01: precise SKI/port messages in a reserved line; nothing moves; nothing truncated.
export async function manualEntry(page) {
  await openPanel(page, 'manual-toggle');
  const connects = [];
  const spy = r => { if (new URL(r.url()).pathname === '/api/connect') connects.push(r.url()); };
  page.on('request', spy);
  try {
    const watched = '#manual-device label, #manual-device input, #manual-device textarea, #manual-connect';
    const before = await boxes(page, watched);
    const same = async what => assert.deepEqual((await boxes(page, watched)).map(b => [b.id, Math.round(b.y)]), before.map(b => [b.id, Math.round(b.y)]), what + ': no layout jump');
    await page.locator('#address').fill(''); await page.locator('#ski').fill('');
    await page.locator('#manual-connect').click();
    assert(await page.locator('#address-error').isVisible() && await page.locator('#ski-error').isVisible(), 'both fields report');
    await same('empty fields');
    const valid = '0123456789abcdef0123456789abcdef01234567';
    for (const port of ['65536', '0']) {
      await page.locator('#address').fill('127.0.0.1:' + port); await page.locator('#ski').fill(valid);
      await page.locator('#manual-connect').click();
      assert.equal(await page.locator('#address-error').innerText(), await t(page, 'Port 1–65535 eingeben'), 'A-01 port range named');
    }
    await same('port error');
    await page.locator('#address').fill('127.0.0.1:1');
    await page.locator('#ski').fill('A'.repeat(45));
    assert.equal((await page.locator('#ski').inputValue()).replace(/\s/g, '').length, 45, 'QA-09 overlong SKI is not cut');
    assert.equal(await page.locator('#ski-count').innerText(), '45 / 40');
    await page.locator('#manual-connect').click();
    assert.equal(await page.locator('#ski-error').innerText(), (await t(page, '{count} statt 40 Zeichen')).replace('{count}', '45'));
    await page.locator('#ski').fill('Z'.repeat(40));
    assert.equal((await page.locator('#ski').inputValue()).replace(/\s/g, ''), 'Z'.repeat(40), 'QA-10 invalid characters stay visible');
    assert((await page.locator('#ski-error').innerText()).includes('Z'), 'QA-10 invalid characters named');
    assert(!await page.locator('#device').isVisible(), 'no comparison view for an invalid identifier');
    // R039-04: many different invalid characters still give a one-line message.
    await page.locator('#ski').fill('GHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+=?');
    await page.locator('#manual-connect').click();
    assert((await page.locator('#ski-error').innerText()).includes('G'), 'first invalid character named');
    await same('SKI errors');
    assert.deepEqual(connects, [], 'no connection attempt with invalid input');
    for (const id of ['#address', '#ski']) await page.locator(id).fill('');
  } finally { page.off('request', spy); await closePanel(page); }
}

// QA-11, QA-12: a failed manual loopback attempt is not "found" and not "waiting for approval".
export async function failedManual(page) {
  await openPanel(page, 'manual-toggle');
  await page.locator('#address').fill('127.0.0.1:1');
  await page.locator('#ski').fill('0123456789abcdef0123456789abcdef01234567');
  const selected = page.waitForResponse(r => new URL(r.url()).pathname === '/api/select');
  await page.locator('#manual-connect').click();
  assert.equal((await selected).status(), 200, 'the API accepts the manual device (no UI-only fields)');
  await page.locator('#pair-error').waitFor({state: 'visible', timeout: 20000});
  const v = await page.evaluate(() => ({trust: document.querySelector('#simulator-trust').textContent.trim(), waiting: StromeaI18n.t('Wartet auf Freigabe am Gerät…'), link: document.querySelector('#pair-link-label').textContent.trim()}));
  assert.notEqual(v.trust, v.waiting, 'QA-12 no waiting state next to the connection error');
  await page.locator('#change-device').click();
  await page.locator('#device-count').waitFor({state: 'visible'});
  const count = await page.locator('#device-count').innerText();
  const found = [await t(page, 'Gerät gefunden'), await t(page, 'Geräte gefunden')];
  assert(!found.some(f => count.includes(f)), 'QA-11 not counted as found: ' + count);
  assert(count.includes(await t(page, 'manuell hinzugefügt')), 'QA-11 counted as manual: ' + count);
}

// QA-07 / UX-S23: phase names above both charts never overlap and keep a visible gap (390 px included).
export async function phaseLabels(page) {
  const v = await page.evaluate(() => [...document.querySelectorAll('svg.power-chart')].filter(s => s.checkVisibility()).map(svg => [...svg.querySelectorAll('text.phase-label')].map(n => { const r = n.getBoundingClientRect(); return {text: n.textContent, x: r.x, y: r.y, w: r.width, h: r.height}; })));
  assert(v.length > 0, 'charts visible');
  for (const labels of v) for (let i = 0; i < labels.length; i++) for (let j = i + 1; j < labels.length; j++) {
    const a = labels[i], b = labels[j];
    const overlapX = a.x < b.x + b.w && b.x < a.x + a.w;
    if (overlapX) { const vgap = a.y < b.y ? b.y - (a.y + a.h) : a.y - (b.y + b.h); assert(vgap >= 4, 'stacked phase names keep a vertical gap (' + Math.round(vgap) + ' px): ' + a.text + ' / ' + b.text); continue; }
    const sameRow = a.y < b.y + b.h && b.y < a.y + a.h;
    if (!sameRow) continue;
    const gap = a.x < b.x ? b.x - (a.x + a.w) : a.x - (b.x + b.w);
    assert(gap >= 6, 'phase names keep a gap (' + Math.round(gap) + ' px): ' + a.text + ' / ' + b.text);
  }
}
export async function practiceResult(page, base) {
  await practicePrepared(page, base);
  const previous = await page.evaluate(() => state?.report?.id || '');
  await page.locator('#run').click();
  await page.waitForFunction(p => state && !state.busy && state.report && state.report.id !== p && !document.querySelector('#step-3').hidden && document.querySelector('#outcome-title')?.textContent.trim(), previous, {timeout: 180000});
}

export async function practicePrepared(page, base) {
  await page.goto(base + '/?step=1');
  await page.waitForFunction(() => typeof state !== 'undefined' && !!state && !state.busy, null, {timeout: 60000});
  if (!await page.locator('#mode-simulator').isVisible() && await page.locator('#change-device').isVisible()) await page.locator('#change-device').click();
  await page.locator('#mode-simulator').check();
  await page.locator('#pair-simulator').click({timeout: 15000});
  await page.locator('#step-2').waitFor();
}

// QA-14, QA-15, B-02: localized limits, focus/summary to the first error, errors vanish when corrected.
export async function limitErrors(page) {
  const values = {lpc: '0', lpp: '-1', failsafe: '1000001', tolerance: '-1'};
  for (const [k, v] of Object.entries(values)) await page.locator('#' + k).fill(v);
  await page.locator('#run').click();
  await page.locator('#config-error').waitFor({state: 'visible'});
  await page.waitForFunction(() => document.activeElement?.id === 'lpc', null, {timeout: 2000}).catch(() => {});
  assert.equal(await page.evaluate(() => document.activeElement?.id), 'lpc', 'QA-15 focus on the first invalid field');
  const links = await page.locator('#config-error-fields a').allInnerTexts();
  assert.equal(links.length, 4, 'QA-15 summary names every marked field');
  if (await page.evaluate(() => StromeaI18n.language() === 'de')) {
    const lpc = await page.locator('#lpc-error').innerText(), fs = await page.locator('#failsafe-error').innerText();
    assert(lpc.includes('0,001 W') && !lpc.includes('0.001'), 'QA-14 German decimal and unit: ' + lpc);
    assert(fs.includes('1.000.000 W') && !fs.includes('1000000'), 'QA-14 German grouping and unit: ' + fs);
  }
  for (const [k, v] of Object.entries({lpc: '4200', lpp: '1000', failsafe: '500', tolerance: '50'})) await page.locator('#' + k).fill(v);
  const stale = await page.evaluate(() => ['lpc', 'lpp', 'failsafe', 'tolerance'].filter(k => document.getElementById(k).getAttribute('aria-invalid') === 'true' || document.getElementById(k + '-error').checkVisibility()));
  assert.deepEqual(stale, [], 'B-02 corrected fields lose their error');
  assert(!await page.locator('#config-error').isVisible(), 'B-02 summary disappears when everything is corrected');
}

// B-01: a hidden field of the other control type never blocks the calculator.
export async function calculatorHiddenField(page) {
  await page.locator('#open-calculator').click();
  await page.locator('#calc-mode').waitFor({state: 'visible'});
  try {
    await page.locator('#calc-power').fill('abc');
    await page.locator('#calc-mode').selectOption('ems');
    await page.locator('#calc-n').fill('2'); await page.locator('#calc-wp').fill('12000'); await page.locator('#calc-ac').fill('0');
    await page.locator('#calc-ac').dispatchEvent('input');
    const result = await page.locator('#calc-result').innerText();
    assert(/8[.,\s ]?160/.test(result), 'B-01 EMS result despite hidden invalid field: ' + result);
    await page.locator('#calc-mode').selectOption('direct');
    await page.locator('#calc-power').dispatchEvent('input');
    const direct = await page.locator('#calc-result').innerText();
    assert(direct.includes(await page.evaluate(() => StromeaI18n.sourceText(document.querySelector('label[for="calc-power"]')) ? StromeaI18n.t(StromeaI18n.sourceText(document.querySelector('label[for="calc-power"]'))) : '')), 'B-01 message names the visible field: ' + direct);
    await page.locator('#calc-power').fill('');
  } finally { await page.keyboard.press('Escape'); }
}

// QA-16: the business summary is fully translated.
export async function preparationTranslated(page) {
  const lang = await page.evaluate(() => StromeaI18n.language());
  if (lang === 'de') return;
  const labels = await page.locator('#business-summary dt').allInnerTexts();
  const expected = await page.evaluate(() => ['Firma', 'Adresse', 'Prüfer / Prüferin'].map(k => StromeaI18n.t(k)));
  assert.notEqual(expected[1], 'Adresse', 'QA-16 dictionary has "Adresse" for ' + lang);
  for (const l of labels) assert(expected.includes(l), 'QA-16 label translated in ' + lang + ': ' + l);
}

// B-03: an aborted practice run is not "failed"; unchecked points are not crossed.
export async function abortedRun(page, base) {
  await practicePrepared(page, base);
  await page.locator('#run').click();
  await page.locator('#cancel').waitFor({state: 'visible'});
  await page.locator('#cancel').click();
  await page.waitForFunction(() => state && !state.busy && state.report && document.querySelector('#outcome-title')?.textContent.trim(), null, {timeout: 60000});
  const v = await page.evaluate(() => ({title: document.querySelector('#outcome-title').textContent.trim(), failed: StromeaI18n.t('Nicht bestanden'), crosses: [...document.querySelectorAll('#result-matrix td')].filter(n => n.checkVisibility() && n.textContent.trim() === '×').length, outcome: state.outcome || state.report?.outcome}));
  assert(!v.title.includes(v.failed), 'B-03 aborted run is not "failed": ' + v.title);
  assert.equal(v.crosses, 0, 'B-03 no × for points that were not evaluated');
}

// C-03: from the result into the profile and back lands on the result.
export async function profileReturn(page) {
  await page.locator('.main-menu a[href="/profil"]').click();
  await page.locator('#profile-return').waitFor();
  assert.equal(await page.locator('#profile-return').getAttribute('href'), '/?step=3');
  await page.locator('#profile-return').click();
  await page.locator('#step-3').waitFor({state: 'visible'});
  assert(!await page.locator('#step-2').isVisible(), 'C-03 not the preparation');
}

// C-02: the logo preview alt text follows the language.
export async function logoAlt(page, base) {
  await page.goto(base + '/profil');
  await page.locator('#profile-preview').waitFor({state: 'attached'});
  await page.evaluate(() => StromeaI18n.setLanguage('ro'));
  await page.waitForTimeout(80);
  assert.equal(await page.locator('#profile-preview').getAttribute('alt'), await t(page, 'Logo-Vorschau'));
  await page.evaluate(() => StromeaI18n.setLanguage('de'));
}

// C-04, C-05, C-01: unique link names, version line follows the language, delete with confirmation.
export async function reportList(page, base) {
  await page.goto(base + '/protokolle');
  await page.locator('#report-practice').check(); // 041: practice reports are hidden by default
  await page.locator('.report-row').first().waitFor();
  // R039-05: a correction with unchanged customer/start/outcome still gets distinct action names.
  const first = await page.locator('.report-row').first().getAttribute('data-id');
  const revised = await page.evaluate(async id => (await fetch('/api/report/site?id=' + encodeURIComponent(id), {method: 'POST', headers: {'Content-Type': 'application/json', 'X-Stromea-Token': document.querySelector('meta[name="session-token"]').content}, body: JSON.stringify({customer: '', customerAddress: '', address: 'Korrigierte Anlage 039', meter: '', note: ''})})).status, first);
  assert.equal(revised, 200, 'correction stored');
  await page.reload();
  await page.locator('#report-practice').check();
  await page.locator('.report-row').nth(2).waitFor();
  const names = await page.evaluate(() => [...document.querySelectorAll('.report-downloads a, .report-downloads .report-delete')].map(a => a.textContent.trim()));
  assert.equal(new Set(names).size, names.length, 'C-04 every link has a unique accessible name: ' + names.join(' | '));
  await page.evaluate(() => StromeaI18n.setLanguage('en'));
  await page.waitForTimeout(80);
  // 041: no version line in the list (one row per inspection); the date follows the language immediately.
  assert.equal(await page.locator('.report-version').count(), 0, '041: no version line');
  const date = await page.locator('.report-date').first().innerText();
  assert(/\d{2}\/\d{2}\/\d{4}/.test(date), 'date follows the language immediately: ' + date);
  await page.evaluate(() => StromeaI18n.setLanguage('de'));
  await page.waitForTimeout(80);
  // R039-07: deleting the only search hit shows the empty-search row although other reports remain.
  const all = await page.locator('.report-row').count();
  await page.locator('#report-search').fill('Korrigierte Anlage 039');
  page.once('dialog', d => d.accept());
  await page.locator('.report-row:not([hidden]) .report-delete').first().click();
  await page.waitForFunction(n => document.querySelectorAll('.report-row').length === n - 1, all); // 041: the corrected series is one row
  assert(await page.locator('#report-empty').isVisible(), 'R039-07 empty-search row visible after deleting the last hit');
  await page.locator('#report-search').fill('');
  const before = await page.locator('.report-row').count();
  page.once('dialog', d => d.dismiss());
  await page.locator('.report-delete').first().click();
  assert.equal(await page.locator('.report-row').count(), before, 'C-01 nothing deleted without confirmation');
  page.once('dialog', d => d.accept());
  await page.locator('.report-delete').first().click();
  await page.waitForFunction(n => document.querySelectorAll('.report-row').length < n, before);
  const listed = (await (await page.request.get(base + '/api/reports')).json()).length;
  assert.equal(listed, await page.locator('.report-row').count(), 'C-01 list and storage agree');
  await page.reload();
  await page.locator('.report-library').waitFor();
  assert.equal(await page.locator('.report-row').count(), listed, 'C-01 deletion is persistent');
}

// UX-S32 on any report list (also the mocked long list): one delete action per row, unique names, no deletion without confirmation.
export async function reportActions(page) {
  const rows = await page.locator('.report-row').count();
  if (!rows) return;
  assert.equal(await page.locator('.report-row .report-delete').count(), rows, 'one delete action per report');
  const names = await page.evaluate(() => [...document.querySelectorAll('.report-downloads a, .report-downloads .report-delete')].map(a => a.textContent.trim()));
  assert.equal(new Set(names).size, names.length, 'unique accessible names');
  const colors = await page.locator('.report-delete').first().evaluate(n => { const probe = document.createElement('span'); probe.style.color = 'var(--error)'; n.parentElement.append(probe); const expected = getComputedStyle(probe).color; probe.remove(); return {actual: getComputedStyle(n).color, expected}; });
  assert.equal(colors.actual, colors.expected, 'R039-06 delete action uses the error colour');
  page.once('dialog', d => d.dismiss());
  await page.locator('.report-delete').first().click();
  assert.equal(await page.locator('.report-row').count(), rows, 'no deletion without confirmation');
}

// R039-02/03: losing and regaining the network is detected live; an open tool closes; a retry after a
// failed connection is locked while offline (real click, no request).
export async function networkChange(page, setNetwork) {
  await openPanel(page, 'manual-toggle');
  await page.locator('#address').fill('127.0.0.1:1');
  await page.locator('#ski').fill('0123456789abcdef0123456789abcdef01234567');
  await page.locator('#manual-connect').click();
  await page.locator('#pair-error').waitFor({state: 'visible', timeout: 20000});
  await page.waitForFunction(() => !state.busy && !document.querySelector('#retry').disabled, null, {timeout: 20000});
  setNetwork(false);
  await page.locator('#network-warning').waitFor({state: 'visible', timeout: 8000}).catch(() => {});
  // The comparison view hides the warning card; the lock itself must hold.
  await page.waitForFunction(() => document.querySelector('#retry').disabled, null, {timeout: 8000});
  const selects = [];
  const spy = r => { if (['/api/select', '/api/connect'].includes(new URL(r.url()).pathname)) selects.push(r.url()); };
  page.on('request', spy);
  await page.locator('#retry').click({force: true});
  await page.waitForTimeout(400);
  page.off('request', spy);
  assert.deepEqual(selects, [], 'R039-03 retry sends nothing while offline');
  await page.locator('#change-device').click();
  await page.locator('#network-warning').waitFor({state: 'visible', timeout: 8000});
  for (const id of ['#discover', '#qr-toggle', '#manual-toggle']) assert(await page.locator(id).isDisabled(), 'R039-02 ' + id + ' locked after losing the network');
  setNetwork(true);
  await page.waitForFunction(() => !document.querySelector('#manual-toggle').disabled, null, {timeout: 8000});
  assert(!await page.locator('#network-warning').isVisible(), 'R039-02 warning disappears when the network returns');
  // Loss while a tool is open closes it.
  await openPanel(page, 'qr-toggle');
  setNetwork(false);
  await page.waitForFunction(() => document.querySelector('#qr-panel').hidden, null, {timeout: 8000});
  setNetwork(true);
  await page.waitForFunction(() => !document.querySelector('#qr-toggle').disabled, null, {timeout: 8000});
}

// R039-08: deleting an example report works in the offline prototype too.
export async function prototypeDelete(page, file) {
  await page.goto(file);
  await page.waitForFunction(() => typeof state !== 'undefined' && !!state);
  await page.locator('.main-menu a[href="/protokolle"]').click();
  await page.locator('#report-practice').check(); // 041
  await page.locator('.report-row').first().waitFor();
  const before = await page.locator('.report-row').count();
  await reportActions(page);
  page.once('dialog', d => d.accept());
  await page.locator('.report-delete').first().click();
  await page.waitForFunction(n => document.querySelectorAll('.report-row').length < n, before, {timeout: 4000});
  assert(!await page.locator('.report-library .download-error').filter({visible: true}).count(), 'no error message');
}

// Re-QA U3: every other sidebar (calculator, measurement details, saved report details) also scrolls to its end.
async function scrollEnd(page, selector) {
  const b = await page.locator(selector).boundingBox();
  const vp = page.viewportSize();
  await page.mouse.move(b.x + b.width / 2, Math.min(vp.height - 20, Math.max(b.y, 0) + 120));
  for (let i = 0; i < 60; i++) await page.mouse.wheel(0, 300);
  await page.waitForTimeout(250);
  return page.evaluate(sel => {
    const root = document.querySelector(sel);
    const all = [...root.querySelectorAll('button,p,label,input,textarea,select,h2,h3,a,li,summary,td,dd,svg')].filter(n => n.checkVisibility() && n.getBoundingClientRect().height > 0 && !n.closest('.sr-only'));
    const last = all.reduce((a, n) => !a || n.getBoundingClientRect().bottom >= a.getBoundingClientRect().bottom ? n : a, null);
    const vv = window.visualViewport, bottom = vv ? Math.min(innerHeight, vv.offsetTop + vv.height) : innerHeight;
    return {id: last.id || last.tagName + ':' + last.textContent.trim().slice(0, 30), bottom: last.getBoundingClientRect().bottom, viewport: bottom};
  }, selector);
}
export async function contextSidebarEnd(page, opener, panel) {
  const mobile = page.viewportSize().width <= 1050;
  if (opener) await page.locator(opener).click();
  await page.locator(panel).waitFor({state: 'visible'});
  const v = await scrollEnd(page, panel);
  assert(v.bottom <= v.viewport + 0.5, panel + ': last element ' + v.id + ' reachable (' + Math.round(v.bottom) + ' ≤ ' + Math.round(v.viewport) + ')');
  if (mobile) assert(v.viewport - v.bottom >= 40, panel + ': end spacing on mobile (' + Math.round(v.viewport - v.bottom) + ' px)');
  await page.keyboard.press('Escape');
  await page.evaluate(() => scrollTo(0, 0));
}
