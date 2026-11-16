// 041 (Nutzer 2026-11-14): Browserprüfungen für UX-S34…UX-S41 und die geänderten Entscheidungen.
// Loopback/Mock only. Heavy checks (new pages, dialogs, downloads) run in German only; layout and copy in every language.
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';

const box = (page, selector) => page.locator(selector).first().evaluate(n => { const r = n.getBoundingClientRect(); return {x: r.x, y: r.y, w: r.width, h: r.height, right: r.right, bottom: r.bottom}; });
const lang = page => page.evaluate(() => StromeaI18n.language());
const t = (page, s) => page.evaluate(s => StromeaI18n.t(s), s);
const visible = (page, selector) => page.locator(selector).first().isVisible().catch(() => false);
const delay = ms => new Promise(r => setTimeout(r, ms));

// UX-S34: no mode switch and no layout jump while the page loads; search button not glued to the adapter list.
export async function startStable(page) {
  const adapter = await visible(page, '#interface') ? '#interface' : '#interface-single';
  const a = await box(page, adapter), m = await box(page, '#step-1 .pair-methods');
  assert(m.y - a.bottom >= 10, 'gap between adapter and actions: ' + (m.y - a.bottom));
  if (await lang(page) !== 'de' || !await visible(page, '#discover')) return;
  const p2 = await page.context().newPage();
  try {
    for (const path of ['**/api/state', '**/api/interfaces']) await p2.route(path, async r => { await delay(700); await r.fallback(); });
    await p2.goto(new URL('/?step=1', page.url()).href, {waitUntil: 'domcontentloaded'});
    await p2.waitForTimeout(150);
    const read = () => p2.evaluate(() => { const d = document.getElementById('discover').getBoundingClientRect(), ws = document.querySelector('.workspace'); return {booting: document.documentElement.classList.contains('booting'), hidden: ws ? getComputedStyle(ws).visibility : '', sim: document.getElementById('sim-controls').hidden, lan: document.getElementById('lan-controls').hidden, onsite: document.getElementById('mode-onsite').checked, discover: Math.round(d.top), methods: Math.round(document.querySelector('#step-1 .pair-methods').getBoundingClientRect().top)}; });
    const early = await read();
    await p2.waitForFunction(() => !document.documentElement.classList.contains('booting') && typeof state !== 'undefined' && !!state, null, {timeout: 8000});
    await p2.waitForTimeout(200);
    const late = await read();
    assert(early.booting && early.hidden === 'hidden', 'content stays invisible until the first state: ' + JSON.stringify(early));
    assert.deepEqual([early.sim, early.lan, early.onsite], [late.sim, late.lan, late.onsite], 'no visible mode switch after loading');
    assert(Math.abs(early.methods - late.methods) <= 2, 'actions keep their place after loading: ' + early.methods + ' → ' + late.methods);
  } finally { await p2.close(); }
}

// UX-S35: copy icons right of the simulator ID / QR, QR left-aligned, fixed status slot.
export async function copyIcons(page) {
  const label = await t(page, 'Simulator-Kennung kopieren');
  if (await page.locator('#step-2').isVisible()) {
    let opened = false;
    if (!await visible(page, '#inspector-copy-ski')) { await page.locator('#details-toggle').click(); opened = true; await page.locator('#inspector-copy-ski').waitFor(); }
    try {
      const ski = await box(page, '#inspector-ski'), button = await box(page, '#inspector-copy-ski');
      assert(button.x >= ski.right - 1 && button.y < ski.bottom && button.bottom > ski.y, 'copy icon right of the simulator ID');
      assert.equal((await page.locator('#inspector-copy-ski').innerText()).trim(), '⧉', 'icon, no text button');
      assert.equal(await page.locator('#inspector-copy-ski').getAttribute('aria-label'), label);
    } finally { if (opened) await page.keyboard.press('Escape'); }
    return;
  }
  if (!await page.locator('#qr-toggle').isEnabled()) return;
  await page.locator('#qr-toggle').click();
  await page.locator('#qr-simulator-tab').click();
  try {
    const qr = await box(page, '#panel-local-qr svg'), section = await box(page, '#qr-simulator-section'), copyQR = await box(page, '#panel-copy-qr');
    assert(qr.x - section.x <= 24, 'simulator QR left-aligned: ' + (qr.x - section.x));
    assert(copyQR.x >= qr.right - 1 && copyQR.y < qr.bottom, 'QR copy icon right of the QR code');
    const ski = await box(page, '#panel-local-ski'), copySki = await box(page, '#panel-copy-ski');
    assert(copySki.x >= ski.right - 1 && copySki.y < ski.bottom && copySki.bottom > ski.y, 'ID copy icon right of the ID');
    for (const id of ['#panel-copy-qr', '#panel-copy-ski']) assert.equal((await page.locator(id).innerText()).trim(), '⧉', id + ' is an icon');
    const before = [await box(page, '#panel-copy-qr'), await box(page, '#panel-copy-ski')];
    await page.locator('#panel-copy-ski').click();
    await page.waitForFunction(() => document.getElementById('panel-copy-status').textContent.trim() !== '');
    const after = [await box(page, '#panel-copy-qr'), await box(page, '#panel-copy-ski')];
    assert.deepEqual(after.map(b => [Math.round(b.x), Math.round(b.y)]), before.map(b => [Math.round(b.x), Math.round(b.y)]), 'copy feedback moves nothing');
  } finally { await page.locator('#qr-device-tab').click(); await page.keyboard.press('Escape'); }
}

// UX-S36: quit is the last menu item; badge right of the menu (menu never moves); persistent dialog, Enter = Schließen.
export async function quitMenu(page) {
  const v = await page.evaluate(() => { const menu = document.querySelector('header .main-menu'), badge = document.getElementById('practice-badge'); const last = [...menu.children].filter(n => n.checkVisibility()).at(-1); const was = badge.hidden; badge.hidden = true; const a = menu.getBoundingClientRect().x; badge.hidden = false; const b = menu.getBoundingClientRect().x; badge.hidden = was; return {last: last?.id, after: !!(menu.compareDocumentPosition(badge) & Node.DOCUMENT_POSITION_FOLLOWING), moved: Math.abs(a - b), text: last?.textContent.trim()}; });
  assert.equal(v.last, 'quit-open', 'quit is the last menu item');
  assert.equal(v.text, await t(page, 'Simulator beenden'));
  assert(v.after && v.moved < 1, 'practice badge right of the menu, menu does not move: ' + JSON.stringify(v));
  if (await lang(page) !== 'de') return;
  const p2 = await page.context().newPage();
  try {
    let shutdowns = 0;
    await p2.route('**/api/shutdown', r => { shutdowns++; return r.fulfill({status: 202, contentType: 'application/json', body: '{"shuttingDown":true,"aborting":false}'}); });
    await p2.route('**/api/instance', r => r.abort());
    await p2.goto(page.url());
    await p2.waitForFunction(() => typeof state !== 'undefined' && !!state && !document.documentElement.classList.contains('booting'));
    await p2.locator('#quit-open').click();
    await p2.locator('#quit-dialog[open]').waitFor();
    assert.equal(await p2.evaluate(() => document.activeElement?.id), 'quit-confirm', 'Enter = Schließen (default focus)');
    assert(!await p2.locator('#quit-busy').isVisible(), 'no running-test hint when idle');
    await p2.mouse.click(4, 4);
    assert(await p2.locator('#quit-dialog[open]').count(), 'click beside does not close');
    await p2.keyboard.press('Escape');
    assert.equal(await p2.locator('#quit-dialog[open]').count(), 0, 'Escape cancels');
    await p2.evaluate(() => { state = {...state, busy: true, kind: 'run'}; });
    await p2.locator('#quit-open').click();
    assert(await p2.locator('#quit-busy').isVisible(), 'running test: hint on abort and lifting the limits');
    assert.equal(await p2.evaluate(() => document.activeElement?.id), 'quit-confirm');
    await p2.keyboard.press('Enter');
    await p2.locator('.quit-screen').waitFor({timeout: 6000});
    await p2.waitForFunction(() => document.querySelector('.quit-screen h2')?.textContent === 'Simulator beendet', null, {timeout: 6000});
    assert.equal(shutdowns, 1, 'exactly one shutdown request');
    assert(!await p2.locator('.workspace').isVisible(), 'calm end state without the workspace');
  } finally { await p2.close(); }
}

// UX-S37: bold footer link right of the GitHub link opens a non-persistent dialog.
export async function aboutDialog(page) {
  const v = await page.evaluate(() => { const b = document.getElementById('about-open'); return {inFooter: !!b?.closest('footer'), previous: b?.previousElementSibling?.getAttribute('href') || '', weight: b ? Number(getComputedStyle(b).fontWeight) : 0, details: !!document.querySelector('details#about')}; });
  assert(v.inFooter && v.previous.includes('github.com'), 'about link right of the GitHub link in the footer');
  assert(v.weight >= 600, 'about link bold');
  assert(!v.details, 'no about details block above the footer');
  if (await lang(page) !== 'de') return;
  await page.locator('#about-open').click();
  await page.locator('#about-dialog[open]').waitFor();
  await page.mouse.click(4, 4);
  assert.equal(await page.locator('#about-dialog[open]').count(), 0, 'click beside closes the about dialog');
  await page.locator('#about-open').click();
  await page.locator('#about-close').click();
  assert.equal(await page.locator('#about-dialog[open]').count(), 0, 'Schließen closes');
}

// UX-S38: export/import from the help page; persistent import dialog, Enter = Abbrechen.
export async function transfer(page) {
  const q = await t(page, 'Wie übertrage ich alle Daten auf einen anderen Computer?');
  assert.equal((await page.locator('#transfer-help h3').innerText()).trim(), q);
  for (const [id, text] of [['#transfer-export', 'Exportdatei erzeugen'], ['#transfer-import', 'Exportdatei importieren']]) assert.equal((await page.locator(id).innerText()).trim(), await t(page, text));
  assert((await page.locator('#transfer-help').innerText()).includes(await t(page, 'Hinweis: Behandle die Exportdatei vertraulich. Sie enthält personenbezogene Daten und die Simulator-Kennung mit ihrem privaten Schlüssel.')), 'confidentiality note');
  if (await lang(page) !== 'de') return;
  const [download] = await Promise.all([page.waitForEvent('download'), page.locator('#transfer-export').click()]);
  assert(/\.zip$/.test(download.suggestedFilename()), 'ZIP download: ' + download.suggestedFilename());
  const file = await download.path(), bytes = await readFile(file);
  assert.equal(bytes.subarray(0, 2).toString(), 'PK', 'export is a ZIP file');
  let imports = 0;
  const spy = r => { if (new URL(r.url()).pathname === '/api/import') imports++; };
  page.on('request', spy);
  try {
    const [chooser] = await Promise.all([page.waitForEvent('filechooser'), page.locator('#transfer-import').click()]);
    await chooser.setFiles(file);
    await page.locator('#import-dialog[open]').waitFor();
    for (const id of ['#import-reports', '#import-profile', '#import-identity']) assert(await page.locator(id).isChecked(), id + ' checked by default');
    assert((await page.locator('#import-warning').innerText()).includes('unwiderruflich'), 'overwrite warning');
    assert.equal(await page.evaluate(() => document.activeElement?.id), 'import-cancel', 'Abbrechen is the default');
    await page.mouse.click(4, 4);
    assert(await page.locator('#import-dialog[open]').count(), 'click beside does not close');
    await page.locator('#import-reports').focus();
    await page.keyboard.press('Enter');
    assert.equal(await page.locator('#import-dialog[open]').count(), 0, 'Enter = Abbrechen');
    assert.equal(imports, 0, 'Enter does not import');
    const [chooser2] = await Promise.all([page.waitForEvent('filechooser'), page.locator('#transfer-import').click()]);
    await chooser2.setFiles(file);
    await page.locator('#import-dialog[open]').waitFor();
    await page.locator('#import-profile').uncheck();
    await page.locator('#import-identity').uncheck();
    await page.locator('#import-confirm').click();
    await page.waitForFunction(() => /Protokolle: \d+ hinzugefügt, \d+ bereits vorhanden\./.test(document.getElementById('transfer-status').textContent), null, {timeout: 8000});
    assert.equal(imports, 1, 'one import request');
  } finally { page.off('request', spy); }
}

// UX-S39: report list tools, two-digit dates, actions, practice filter; /protokoll without unstyled flash.
export async function reportList041(page) {
  if (await page.locator('.report-library').count()) {
    const search = await box(page, '#report-search'), practice = await box(page, '#report-practice');
    if (page.viewportSize().width > 600) assert(practice.x >= search.right - 1 && Math.abs((practice.y + practice.h / 2) - (search.y + search.h / 2)) <= 8, 'practice filter right of the search');
    assert.equal((await page.locator('label.report-practice').innerText()).trim(), await t(page, 'Übungs-Protokolle anzeigen'));
    const rows = page.locator('.report-row');
    if (!await rows.count()) return;
    const was = await page.locator('#report-practice').isChecked();
    if (was) await page.locator('#report-practice').uncheck();
    const hidden = await page.evaluate(() => [...document.querySelectorAll('.report-row')].filter(r => r.checkVisibility()).length);
    await page.locator('#report-practice').check();
    const shown = await page.evaluate(() => [...document.querySelectorAll('.report-row')].filter(r => r.checkVisibility()).length);
    assert(shown > hidden, 'practice reports appear only when the checkbox is on');
    const v = await page.evaluate(() => { const row = [...document.querySelectorAll('.report-row')].find(r => r.checkVisibility()); const date = row.querySelector('.report-date').lastElementChild.textContent; const names = [...row.querySelectorAll('.report-downloads a, .report-downloads .report-delete')].map(a => a.firstChild.textContent.trim()); const del = row.querySelector('.report-delete'), first = row.querySelector('.report-downloads a'); const r1 = del.getBoundingClientRect(), r2 = first.getBoundingClientRect(); return {date, names, deco: getComputedStyle(del).textDecorationLine, versions: document.querySelectorAll('.report-version').length, previous: [...document.querySelectorAll('.report-library a')].some(a => a.textContent.includes(StromeaI18n.t('Frühere Version'))), lineDiff: Math.abs((r1.top + r1.height / 2) - (r2.top + r2.height / 2)), sameRow: Math.abs(r1.top - r2.top) < 4}; });
    assert(/(^|\D)\d{2}\D\d{2}\D\d{4}/.test(v.date) || /\d{4}\D\d{2}\D\d{2}/.test(v.date), 'two-digit day and month: ' + v.date);
    assert.deepEqual(v.names, [await t(page, 'Ansehen'), await t(page, 'PDF laden'), await t(page, 'Messdaten (JSON)'), await t(page, 'Löschen')], 'actions');
    assert(v.deco.includes('underline'), 'Löschen underlined');
    assert.equal(v.versions, 0, 'no version line in the list');
    assert(!v.previous, 'no link to an earlier version in the list');
    if (!was) await page.locator('#report-practice').uncheck();
    // Review R041-05: without JavaScript the server-rendered list is visible (no loading state hides it).
    if (await lang(page) === 'de' && page.url().startsWith('http')) {
      const ctx = await page.context().browser().newContext({javaScriptEnabled: false});
      try { const nojs = await ctx.newPage(); await nojs.goto(new URL('/protokolle', page.url()).href); assert(await nojs.locator('#report-list-fallback .report-library').isVisible(), 'report list visible without JavaScript'); }
      finally { await ctx.close(); }
    }
    return;
  }
  const id = await page.evaluate(() => state?.report?.id || '');
  if (!id || await lang(page) !== 'de') return;
  const p2 = await page.context().newPage();
  try {
    // Review R041-06: hold the deferred report script back and inspect the page while it is still missing.
    let release; const held = new Promise(r => { release = r; });
    await p2.route('**/saved-report.js', async r => { await held; await r.fallback(); });
    await p2.goto(new URL('/protokoll?id=' + encodeURIComponent(id), page.url()).href, {waitUntil: 'commit'});
    await p2.locator('#saved-fallback').waitFor({state: 'attached'});
    await p2.waitForTimeout(200);
    const early = await p2.evaluate(() => ({fallback: document.getElementById('saved-fallback')?.checkVisibility(), mounted: !!document.querySelector('#saved-result:not([hidden])')}));
    release();
    assert(!early.mounted && !early.fallback, 'no unstyled server view while the report view loads: ' + JSON.stringify(early));
    await p2.locator('#saved-result:not([hidden])').waitFor({timeout: 8000});
  } finally { await p2.close(); }
}

// UX-S40: watt fields half width, right-aligned, LPC/LPP bold, grouping; calculator link hidden without jump; one duration line.
export async function wattFields(page) {
  const v = await page.evaluate(() => { const out = {}; for (const id of ['lpc', 'lpp', 'failsafe', 'tolerance']) { const input = document.getElementById(id), field = document.getElementById(id + '-field'), cs = getComputedStyle(input); out[id] = {ratio: input.closest('.input-unit').getBoundingClientRect().width / field.getBoundingClientRect().width, align: cs.textAlign, weight: Number(cs.fontWeight), help: field.querySelector('.help')?.getBoundingClientRect().width / field.getBoundingClientRect().width}; } return out; });
  for (const [id, f] of Object.entries(v)) {
    if (page.viewportSize().width > 600) assert(f.ratio > .4 && f.ratio < .62, id + ' half width: ' + f.ratio);
    assert(['right', 'end'].includes(f.align), id + ' right-aligned');
    if (['lpc', 'lpp'].includes(id)) assert(f.weight >= 600, id + ' bold');
    if (f.help) assert(f.help > .9, id + ' help keeps the full width');
  }
  const grouped = await page.evaluate(() => { const input = document.getElementById('lpc'), old = input.value; input.value = '42000'; input.dispatchEvent(new Event('input', {bubbles: true})); const value = input.value; input.value = old; input.dispatchEvent(new Event('input', {bubbles: true})); return {value, expected: (42000).toLocaleString(StromeaI18n.locale())}; });
  assert.equal(grouped.value, grouped.expected, 'thousands separator of the UI language');
  const docTop = () => page.evaluate(() => document.getElementById('failsafe-field').getBoundingClientRect().top + scrollY);
  const before = {y: await docTop()};
  const setLpc = on => page.evaluate(on => { const c = document.getElementById('scope-lpc'); if (c.checked !== on) { c.checked = on; c.dispatchEvent(new Event('change', {bubbles: true})); } }, on);
  const touched = await page.evaluate(() => typeof scopeTouched === 'undefined' ? null : scopeTouched);
  await setLpc(false);
  try {
    const off = await page.evaluate(() => getComputedStyle(document.getElementById('open-calculator')).visibility);
    const after = {y: await docTop()};
    assert.equal(off, 'hidden', 'calculator link invisible without LPC');
    assert(Math.abs(after.y - before.y) < 1, 'no layout jump below');
  } finally { await setLpc(true); await page.evaluate(t => { if (t !== null) scopeTouched = t; }, touched); }
  const d = await page.evaluate(() => ({text: document.getElementById('duration').textContent, keep: StromeaI18n.t('Lass das Browserfenster geöffnet. Prüfe am Ende, ob die Testgrenzen aufgehoben wurden.'), trust: !!document.querySelector('#form .notice.trust')}));
  assert(d.text.endsWith(d.keep), 'duration line ends with the keep-open hint');
  assert(!d.trust, 'no separate keep-open notice');
  assert(!/10\s?[×x]|10 раз|10 kat|de 10 ori/i.test(d.text), 'no internal speed-up remark');
}

// UX-S41: calculator sources as small print with links (new tab), no "Vorgaben nachlesen".
export async function guideSources(page) {
  await page.locator('#open-calculator').click();
  try {
    await page.locator('#guide-sources a').first().waitFor();
    const v = await page.evaluate(() => [...document.querySelectorAll('#guide-sources a')].filter(a => a.checkVisibility()).map(a => ({href: a.href, target: a.target, rel: a.rel, cls: a.className, size: parseFloat(getComputedStyle(a).fontSize)})));
    assert(v.length >= 1 && v[0].href.startsWith('https://www.bundesnetzagentur.de/'), 'BNetzA source linked');
    for (const a of v) { assert.equal(a.target, '_blank'); assert(a.rel.includes('noopener')); assert(a.cls.includes('external-link')); assert(a.size <= 13, 'small print'); }
    assert.equal(await page.locator('#guide-reference, #guide-basis details').count(), 0, 'no "Vorgaben nachlesen" disclosure');
  } finally { if (page.viewportSize().width <= 1050) await page.keyboard.press('Escape'); }
}

// UX-G6 (041): footer sticky; the detail column reaches the footer.
export async function footerSticky(page) {
  const position = await page.locator('main>footer').evaluate(n => getComputedStyle(n).position);
  if (page.viewportSize().width > 600) assert.equal(position, 'sticky', 'footer floats like the header');
  if (page.viewportSize().width < 1051 || !await visible(page, '#detail-inspector')) return;
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  const aside = await box(page, '#detail-inspector'), footer = await box(page, 'main>footer');
  await page.evaluate(() => window.scrollTo(0, 0));
  assert(aside.bottom >= footer.y - 2, 'detail column reaches the footer: ' + aside.bottom + ' / ' + footer.y);
}
