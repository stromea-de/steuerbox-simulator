// 043 (Nutzer 2026-11-14, Echtgerät): Browserprüfungen für UX-G16…UX-G18 und UX-S43…UX-S48 im UX-Katalog.
// Der vollständige Nutzerweg mit echtem Verbindungsabbruch steht in scripts/test_ux043.mjs. Loopback/Mock only.
import assert from 'node:assert/strict';

const t = (page, s) => page.evaluate(s => StromeaI18n.t(s), s);

// UX-G16: status rows have reserved space; actions and content never move when a status appears.
export async function statusNoJump(page) {
  const v = await page.evaluate(() => {
    if (!document.getElementById('lan-controls').checkVisibility()) return null;
    const ids = ['discover', 'qr-toggle', 'manual-toggle', 'device-results'];
    const box = () => ids.map(id => { const r = document.getElementById(id).getBoundingClientRect(); return [Math.round(r.x), Math.round(r.y + scrollY)]; });
    const before = box(), was = searching; searching = true; controls();
    const during = box(), row = document.getElementById('search-motion'), shown = row.checkVisibility(), rb = row.getBoundingClientRect(), mb = document.querySelector('#step-1 .pair-methods').getBoundingClientRect();
    searching = was; controls();
    return {before, during, after: box(), shown, below: rb.top >= mb.bottom - 1, label: row.textContent.trim()};
  });
  if (!v) return;
  assert(v.shown, 'search status visible while searching');
  assert(v.below, 'search status in its own row below the actions');
  assert.deepEqual(v.during, v.before, 'nothing moves when the search status appears');
  assert.deepEqual(v.after, v.before, 'nothing moves when it disappears');
}

// UX-S44: every search result names the adapter it was found on (harness mock: mock-wifi).
export async function searchAdapter(page) {
  const v = await page.evaluate(() => ({text: document.querySelector('.device-choice .device-adapter')?.textContent || '', expected: StromeaI18n.t('über {adapter}').replace('{adapter}', StromeaI18n.t('WLAN') + ' (mock-wifi)'), details: [...document.querySelectorAll('#step-1 details')].filter(d => d.checkVisibility() && /Technische Details/.test(StromeaI18n.sourceText(d.querySelector('summary')))).length}));
  assert.equal(v.text, v.expected, 'result names its adapter');
  assert.equal(v.details, 0, 'no collapsible technical caption in step 1');
}

// UX-S43: inline remaining time, no grey second line, no false „waiting for approval“, confirmation text present.
export async function pairingInline(page) {
  const v = await page.evaluate(() => {
    const previous = state;
    state = {...state, reconnection: {active: true, attempt: 1, deadline: new Date(Date.now() + 54500).toISOString()}}; setConnection('connecting'); controls();
    const label = document.getElementById('connection-label').textContent, grey = document.getElementById('reconnect-progress').checkVisibility();
    state = previous; setConnection('idle'); controls();
    return {label, expected: StromeaI18n.t('Verbindung wird aufgebaut ({time})'), grey, trust: document.getElementById('simulator-trust').textContent.trim(), waiting: StromeaI18n.t('Wartet auf Freigabe am Gerät…'),
      success: document.querySelector('#step-1 .pair-primary #pair-success span')?.textContent || '', successExpected: StromeaI18n.t('Gekoppelt. Weiter zur Prüfung …'),
      adapter: [...document.querySelectorAll('#connection-context dd')].map(n => n.textContent).join(' | '), wlan: StromeaI18n.t('WLAN') + ' (mock-wifi)', column: document.getElementById('detail-inspector').checkVisibility(), wide: innerWidth > 1050};
  });
  const re = new RegExp('^' + v.expected.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace('\\{time\\}', '0:5\\d') + '$');
  assert.match(v.label, re, 'remaining time inline in the green line');
  assert(!v.grey, 'no second grey timer line');
  assert.notEqual(v.trust, v.waiting, '„Wartet auf Freigabe“ only while connecting');
  assert.equal(v.success, v.successExpected, 'confirmation shares the slot of the button');
  assert(v.adapter.includes(v.wlan), 'detail column names the adapter of the chosen result: ' + v.adapter);
  if (v.wide) assert(v.column, 'detail column visible with an opened device');
}

// UX-S45: detected device chip below „Prüfumfang“, bold W at LPC/LPP, one calculator link, extra space.
export async function preparation043(page) {
  const v = await page.evaluate(() => {
    const legend = document.querySelector('#scope-control legend'), row = document.getElementById('detected-device'), chip = document.getElementById('device-type-chip');
    const calc = [...document.querySelectorAll('#step-2 button,#step-2 a')].filter(n => n.checkVisibility() && n.textContent.trim() === StromeaI18n.t('Rechner öffnen'));
    const mt = id => parseFloat(getComputedStyle(document.getElementById(id).closest('.field-heading')).marginTop);
    return {after: legend.nextElementSibling === row, rowVisible: row.checkVisibility(), chip: chip.textContent.trim(), chipVisible: chip.checkVisibility(), label: row.firstElementChild.textContent.trim(),
      expectedLabel: StromeaI18n.t('Erkanntes Gerät:'), allowed: ['Simuliertes Gerät', 'unbekannt', 'Wärmepumpe', 'Wärmeerzeuger', 'Wechselrichter', 'Wallbox', 'Energiemanagementsystem'].map(StromeaI18n.t),
      weights: ['unit-lpc', 'unit-lpp'].map(id => Number(getComputedStyle(document.getElementById(id)).fontWeight)), calc: calc.length, inHelp: calc.every(n => !!n.closest('.calculator-help')),
      space: mt('sign-inspector') - mt('sign-operator')};
  });
  assert(v.after && v.rowVisible, '„Erkanntes Gerät“ directly below „Prüfumfang“');
  assert.equal(v.label, v.expectedLabel);
  assert(v.chipVisible && v.allowed.includes(v.chip), 'device type chip: ' + v.chip);
  for (const w of v.weights) assert(w >= 600, 'W bold at LPC/LPP');
  assert.equal(v.calc, 1, '„Rechner öffnen“ exactly once');
  assert(v.inHelp, 'calculator link inside the help text');
  assert(v.space >= 7.5, '6 pt more space above the inspector signature field: ' + v.space);
}

// UX-G17: profile and help return to exactly this step; entered values stay.
export async function navigationKeepsState(page) {
  const before = await page.evaluate(() => ({meter: document.getElementById('site-meter').value, lpc: document.getElementById('lpc').value}));
  await page.locator('#site-meter').fill('G17 bleibt');
  const lpc = await page.locator('#lpc').inputValue();
  try {
    await page.locator('.main-menu a[href="/profil"]').click(); await page.locator('#profile-inspector').waitFor();
    assert(await page.locator('.profile-actions #profile-return').isVisible(), 'return button next to „Profil speichern“');
    await page.locator('#profile-return').click(); await page.locator('#step-2:not([hidden])').waitFor();
    assert.equal(await page.locator('#site-meter').inputValue(), 'G17 bleibt', 'value kept after the profile');
    await page.locator('.main-menu a[href="/hilfe"]').click(); await page.locator('.local-page').waitFor(); await page.goBack(); await page.locator('#step-2:not([hidden])').waitFor();
    assert.equal(await page.locator('#site-meter').inputValue(), 'G17 bleibt', 'value kept after help and back');
    assert.equal(await page.locator('#lpc').inputValue(), lpc, 'limit kept');
  } finally { await page.locator('#site-meter').fill(before.meter); }
}

// UX-G18: no disabled primary button without a visible reason next to it.
export async function noSilentDisabled(page) {
  const v = await page.evaluate(() => {
    const read = () => { const run = document.getElementById('run'), alt = document.getElementById('run-reconnect'), reason = document.getElementById('start-reason');
      const primary = run.checkVisibility() ? run : alt; return {disabled: primary.disabled, busy: primary.getAttribute('aria-busy') === 'true', reason: reason.checkVisibility() ? reason.textContent.trim() : '', sameRow: primary.closest('.start-action') === reason.closest('.start-action')}; };
    const saved = {real, capabilityState, connected}, out = {normal: read()};
    real = true; connected = true; capabilityState = {lpc: false, lpp: false, mpc: true}; controls(); out.noControl = read();
    capabilityState = null; controls(); out.reading = read();
    ({real, capabilityState, connected} = saved); controls(); out.restored = read();
    return out;
  });
  for (const [name, s] of Object.entries(v)) {
    assert(s.sameRow, name + ': reason shares the row of the button');
    if (s.disabled && !s.busy) assert(s.reason.length > 5, name + ': disabled primary button needs a visible reason');
  }
  assert(!v.normal.disabled, 'ready: start enabled');
}

// UX-S46: concrete field link; missing inspector name opens the persistent dialog (cancel returns to step 2).
export async function startWithoutGuessing(page) {
  const scope = await page.evaluate(() => [document.getElementById('scope-lpc').checked, document.getElementById('scope-lpp').checked]);
  try {
    await page.evaluate(() => { for (const id of ['scope-lpc', 'scope-lpp']) { const c = document.getElementById(id); c.checked = false; c.dispatchEvent(new Event('change', {bubbles: true})); } });
    assert(await page.locator('#run').isEnabled(), 'empty scope: start stays clickable and explains');
    await page.locator('#run').click(); await page.locator('#config-error:not([hidden])').waitFor();
    const e = await page.evaluate(() => ({text: document.getElementById('config-error-text').textContent, links: [...document.querySelectorAll('#config-error-fields a')].map(a => a.textContent.trim()), detail: document.getElementById('config-error-detail').textContent}));
    assert.equal(e.text, await t(page, 'Wähle mindestens eine Prüfrichtung.'), 'concrete message');
    assert.deepEqual(e.links, [await t(page, 'Prüfumfang')], 'field link to „Prüfumfang“');
    assert(!e.text.includes('Firma'), 'no profile hint');
  } finally {
    await page.evaluate(([a, b]) => { for (const [id, on] of [['scope-lpc', a], ['scope-lpp', b]]) { const c = document.getElementById(id); if (c.checked !== on) { c.checked = on; c.dispatchEvent(new Event('change', {bubbles: true})); } } clearErrors(); controls(); }, scope);
  }
  const profile = await page.evaluate(() => { const p = StromeaProfile.snapshot(); if (p.inspector) StromeaProfile.replace({...p, inspector: ''}); window.ux043real = real; real = true; return p; });
  try {
    await page.locator('#run').click(); await page.locator('#inspector-dialog[open]').waitFor();
    const d = await page.evaluate(() => ({title: document.getElementById('inspector-dialog-title').textContent, label: document.querySelector('label[for=inspector-dialog-name]').textContent, save: document.getElementById('inspector-dialog-save').textContent, cancel: document.getElementById('inspector-dialog-cancel').textContent, focus: document.activeElement?.id}));
    assert.equal(d.title, await t(page, 'Prüfer / Prüferin'));
    assert.equal(d.save, await t(page, 'Speichern & weiter'));
    assert.equal(d.cancel, await t(page, 'Abbrechen'));
    assert.equal(d.focus, 'inspector-dialog-name', 'focus in the name field');
    await page.locator('#inspector-dialog-cancel').click();
    assert.equal(await page.locator('#inspector-dialog[open]').count(), 0, 'Abbrechen closes the dialog');
    assert(await page.locator('#step-2').isVisible(), 'back in step 2');
  } finally { await page.evaluate(p => { real = window.ux043real; delete window.ux043real; StromeaProfile.replace(p); controls(); }, profile); }
}

// UX-S47: one decision card with the timer in its header, measured vs. required, two explained options.
export async function lowLoadCard(page) {
  const v = await page.evaluate(() => {
    const low = {direction: 'LPP', watts: 10996, required: 1050, directions: [{direction: 'LPP', watts: 10996, required: 1050}]};
    renderLowLoad({busy: true, progress: {lowLoad: low}});
    resultView.waiting({lowLoad: low, deadline: new Date(Date.now() + 276000).toISOString()}, true);
    const card = document.getElementById('low-load'), rows = [...card.querySelectorAll('.decision-table tbody tr')].map(tr => [...tr.children].map(c => c.textContent.trim()));
    const out = {timer: card.querySelector('.decision-head #low-load-timer')?.textContent || '', banner: !document.getElementById('wait-banner').hidden, rows,
      reading: card.querySelector('.low-load-reading')?.textContent || '', options: [...card.querySelectorAll('.decision-option')].map(o => [o.querySelector('button')?.id, (o.querySelector('.option-copy')?.textContent || '').trim().length]),
      advice: document.querySelector('#remeasure + .option-copy').textContent, adviceExpected: StromeaI18n.t('Einspeisung erhöhen, z. B. Erzeugung abwarten oder Verbraucher ausschalten, dann erneut messen.'),
      expected: {name: StromeaI18n.t('Einspeisung (LPP)'), measured: StromeaI18n.t('{watts} W').replace('{watts}', '0'), required: StromeaI18n.t('über {watts}').replace('{watts}', StromeaI18n.t('{watts} W').replace('{watts}', (1050).toLocaleString(StromeaI18n.locale()))), reading: (10996).toLocaleString(StromeaI18n.locale())}};
    renderLowLoad(state); resultView.waiting(state.progress, !!state.busy);
    return out;
  });
  assert.match(v.timer, /^4:3\d$/, 'remaining time in the card header');
  assert(!v.banner, 'no second banner');
  assert.deepEqual(v.rows, [[v.expected.name, v.expected.measured, v.expected.required]], 'measured vs. required per direction');
  assert(v.reading.includes(v.expected.reading), 'signed reading explains 0 W feed-in');
  assert.deepEqual(v.options.map(o => o[0]), ['remeasure', 'continue-low']);
  assert.equal(v.advice, v.adviceExpected, 'G043-01: feed-in needs generation, not load');
  for (const [, length] of v.options) assert(length > 20, 'each option is explained');
}

// UX-S48: cause directly below the result title with one action; „Prüfumfang:“; no bottom card.
export async function abortCause(page) {
  const saved = await page.evaluate(() => !!document.querySelector('#saved-report'));
  const scope = await page.evaluate(() => {const s = (document.querySelector('#saved-result:not([hidden])') || document.getElementById('step-3')); return {line: s.querySelector('#result-scope').textContent, prefix: StromeaI18n.t('Prüfumfang:'), old: s.innerText.includes('Geprüfter Umfang')};});
  assert(scope.line.startsWith(scope.prefix), '„Prüfumfang:“ line: ' + scope.line);
  assert(!scope.old, 'no „Geprüfter Umfang“');
  if (saved) return;
  const v = await page.evaluate(() => {
    const report = state.report, started = Date.parse(report.started);
    const synthetic = {...state, outcome: 'Unvollständig', report: {...report, abortReason: 'connection_lost', steps: [...(report.steps || []), {name: 'Last vor Start prüfen', passed: false, status: 'not_checked', reason: 'connection_lost', at: new Date(started + 61000).toISOString()}, {name: 'Grenzen aufheben', passed: false, reason: 'connection_lost', at: new Date(started + 360000).toISOString()}]}};
    renderResult(synthetic);
    const r = id => document.getElementById(id).getBoundingClientRect(), title = r('outcome-title'), cause = r('result-abort');
    const between = ['outcome-text', 'result-scope', 'result-meta'].filter(id => { const n = document.getElementById(id); return n.checkVisibility() && n.getBoundingClientRect().top < cause.top; });
    const out = {visible: document.getElementById('result-abort').checkVisibility(), gap: cause.top - title.bottom, between, title: document.getElementById('result-cause-title').textContent, text: document.getElementById('result-cause-text').textContent,
      action: document.getElementById('result-cause-action').checkVisibility() ? document.getElementById('result-cause-action').textContent : null, runError: document.getElementById('run-error').checkVisibility(),
      expected: {title: StromeaI18n.t('Verbindung zum Gerät unterbrochen'), action: StromeaI18n.t('Neu verbinden')}};
    // R043-04: a timeout of the first measurement is no missed power decision; a timeout after a measurement is.
    const timeout = name => { renderResult({...state, outcome: 'Unvollständig', report: {...report, abortReason: 'timeout', steps: [{name: 'Last vor Start prüfen', passed: true, at: new Date(started + 1000).toISOString()}, {name, passed: false, reason: 'timeout', at: new Date(started + 300000).toISOString()}]}}); return document.getElementById('result-cause-title').textContent; };
    // After „Erneut messen“ a timeout of the second measurement is a measurement timeout, not a missed decision.
    out.measurementTimeout = timeout('Last vor Start prüfen'); out.decisionTimeout = timeout('Lastentscheidung vor Start');
    out.expected.measurement = StromeaI18n.t('Zeitlimit erreicht'); out.expected.decision = StromeaI18n.t('Keine Entscheidung zur Leistung');
    renderResult(state);
    out.restored = document.getElementById('result-abort').hidden === !state.report?.abortReason || !['connection_lost', 'timeout', 'user_cancel'].includes(state.report?.abortReason);
    return out;
  });
  assert(v.visible, 'cause visible');
  assert(v.gap <= 24 && !v.between.length, 'cause directly below the title: ' + JSON.stringify([v.gap, v.between]));
  assert.equal(v.title, v.expected.title);
  assert(v.text.includes('1:01'), 'names when it happened: ' + v.text);
  assert.equal(v.measurementTimeout, v.expected.measurement, 'measurement timeout is no missed decision');
  assert.equal(v.decisionTimeout, v.expected.decision, 'missed power decision');
  assert.equal(v.action, v.expected.action, 'exactly one action');
  assert(!v.runError, 'no warning card further down');
  assert(v.restored, 'restored');
}
