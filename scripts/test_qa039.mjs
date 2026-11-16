#!/usr/bin/env node
// 039: browser regression tests for the QA 039 findings. Own loopback server, own state, no LAN.
// QA039_ONLY=<comma separated check names> limits the run; QA039_SCREENS=<dir> stores screenshots.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {mkdir, mkdtemp, rm, writeFile} from 'node:fs/promises';
import {resolve, dirname, join} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
import * as K from './qa039-checks.mjs';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const output = resolve(root, process.env.QA039_OUTPUT || 'artifacts/039');
await mkdir(output, {recursive: true});
const only = (process.env.QA039_ONLY || '').split(',').filter(Boolean);
const screens = process.env.QA039_SCREENS ? resolve(root, process.env.QA039_SCREENS) : '';
if (screens) await mkdir(screens, {recursive: true});
const languages = ['de', 'en', 'pl', 'ru', 'uk', 'tr', 'ro'];
const results = [];

const work = await mkdtemp(join(output, 'runtime-'));
const listener = createServer(); listener.listen(0, '127.0.0.1'); await once(listener, 'listening');
const port = listener.address().port; await new Promise(r => listener.close(r));
const base = 'http://127.0.0.1:' + port;
const binary = resolve(root, process.env.E2E_BINARY || 'artifacts/039/server');
const server = spawn(binary, ['serve', '--state', join(work, 'state'), '--listen', '127.0.0.1:' + port, '--loopback-only','--loopback-partner'], {cwd: root, stdio: ['pipe', 'ignore', 'ignore']});
for (let i = 0; ; i++) { try { if ((await fetch(base + '/api/state')).ok) break; } catch {} if (i > 200) throw Error('server did not start'); await new Promise(r => setTimeout(r, 100)); }

const pw = process.env.E2E_PLAYWRIGHT || join(process.env.HOME, '.cache/stromea-pw/node_modules/playwright-core/index.mjs');
const {chromium} = await import(pathToFileURL(pw));
const browser = await chromium.launch({headless: true, args: ['--disable-background-networking']});

async function context({width, height = 900, theme = 'light', network = true, mobile = false}) {
  const ctx = await browser.newContext({viewport: {width, height}, locale: 'de-DE', colorScheme: theme, reducedMotion: 'reduce', isMobile: mobile, hasTouch: mobile});
  const net = {on: network};
  await ctx.route('**/*', route => {
    const u = new URL(route.request().url());
    if (u.origin !== base && !['data:', 'blob:', 'about:'].includes(u.protocol)) return route.abort('blockedbyclient');
    if (!net.on && network && u.pathname === '/api/interfaces') return route.fulfill({json: [{name: 'loopback', address: '127.0.0.1', loopbackOnly: true}]});
    if (net.on && u.pathname === '/api/interfaces') return route.fulfill({json: [{name: 'mock-ethernet', kind: 'ethernet', address: '192.0.2.1'}, {name: 'mock-wifi', kind: 'wifi', address: '192.0.2.2'}, {name: 'loopback', address: '127.0.0.1', loopbackOnly: true}]});
    return route.continue();
  });
  const page = await ctx.newPage();
  page.setDefaultTimeout(4000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  page.on('console', m => { if (m.type() === 'error') errors.push(m.text()); });
  page.on('response', r => { if (r.status() >= 400) errors.push(r.status() + ' ' + r.url()); });
  return {ctx, page, errors, setNetwork: on => { net.on = on; }};
}
async function start(page, step = '/?step=1') {
  await page.goto(base + step);
  await page.waitForFunction(() => typeof state !== 'undefined' && !!state);
  if (!await page.locator('#mode-onsite').isChecked()) { await page.locator('#mode-onsite').check(); }
  await page.locator('#lan-controls').waitFor({state: 'visible'});
  await page.waitForFunction(() => document.querySelector('#interface').options.length > 0);
}
async function language(page, lang) { await page.evaluate(l => StromeaI18n.setLanguage(l), lang); await page.waitForTimeout(40); }
async function run(name, meta, fn) {
  if (only.length && !only.includes(name)) return;
  try { await fn(); results.push({name, ...meta, passed: true}); }
  catch (e) { results.push({name, ...meta, passed: false, error: e.message.split('\n')[0]}); }
}
async function shot(page, file) { if (screens) await page.screenshot({path: join(screens, file)}); }

try {
  for (const theme of ['light', 'dark']) for (const [width, height] of [[1440, 900], [1024, 768], [390, 844]]) {
    const {ctx, page, errors} = await context({width, height, theme});
    const vp = width + '-' + theme;
    await start(page);
    await shot(page, 'start-netzwerk-' + vp + '.png');
    for (const lang of languages) {
      await language(page, lang);
      await run('pair-methods', {width, theme, lang}, () => K.pairMethods(page));
      if (theme === 'light') await run('qr-panel', {width, theme, lang}, () => K.qrPanel(page));
    }
    await language(page, 'de');
    if (screens) {
      await K.openPanel(page, 'qr-toggle'); await page.locator('#qr-text').fill('falscher QR-Text'); await page.locator('#qr-apply').click(); await shot(page, 'qr-fehler-' + vp + '.png');
      await page.locator('#qr-text').fill(''); await K.closePanel(page);
      await K.openPanel(page, 'manual-toggle'); await shot(page, 'manuell-' + vp + '.png'); await K.closePanel(page);
    }
    await run('mic-icon', {width, theme}, () => K.micIcon(page));
    if (theme === 'light') for (const lang of width === 390 ? languages : ['de']) { await language(page, lang); await run('manual-entry', {width, lang}, () => K.manualEntry(page)); }
    await language(page, 'de');
    if (theme === 'light') {
      await run('sidebar-end', {width, height}, () => K.sidebarEnd(page));
      await run('native-dialogs', {width}, () => K.nativeDialogs(page));
    }
    await run('console-clean', {width, theme}, async () => assert.deepEqual(errors.filter(e => !/Permission denied|getUserMedia|Requested device/.test(e)), [], 'QA-06 no failed request or console error'));
    await ctx.close();
  }
  // Additional sidebar heights, incl. small laptop, tablet and a phone with the software keyboard open.
  for (const [width, height, keyboard, mobile] of [[1280, 720, 0, false], [1100, 650, 0, false], [820, 1180, 0, true], [390, 560, 0, true], [390, 844, 320, true]]) {
    const {ctx, page} = await context({width, height, mobile});
    await start(page);
    await run('sidebar-end', {width, height, keyboard, mobile}, () => K.sidebarEnd(page, {keyboard}));
    await ctx.close();
  }
  {
    const {ctx, page} = await context({width: 1024});
    await start(page);
    await run('failed-manual', {width: 1024}, () => K.failedManual(page));
    await ctx.close();
  }
  {
    const {ctx, page, setNetwork} = await context({width: 390, height: 844});
    await start(page);
    await run('network-change', {width: 390}, () => K.networkChange(page, setNetwork));
    await ctx.close();
  }
  // Practice run: result view (charts, outcome).
  if (!only.length || only.some(n => ['phase-labels'].includes(n))) {
    const {ctx, page} = await context({width: 390, height: 844, theme: 'dark'});
    await run('practice-run', {width: 390}, () => K.practiceResult(page, base));
    for (const [width, height] of [[390, 844], [1024, 768], [1440, 900]]) {
      await page.setViewportSize({width, height}); await page.waitForTimeout(200);
      for (const lang of languages) { await language(page, lang); await page.waitForTimeout(80); await run('phase-labels', {width, lang}, () => K.phaseLabels(page)); }
      await language(page, 'de');
      if (screens) { await page.locator('#power-chart').scrollIntoViewIfNeeded(); await shot(page, 'ergebnis-messverlauf-' + width + '-dark.png'); }
    }
    await ctx.close();
  }
  // Preparation, abort, profile and report list (practice mode only).
  {
    const {ctx, page} = await context({width: 390, height: 844});
    await K.practicePrepared(page, base);
    for (const lang of languages) { await language(page, lang); await run('limit-errors', {lang}, () => K.limitErrors(page)); await run('prep-translated', {lang}, () => K.preparationTranslated(page)); }
    await language(page, 'de');
    await run('calculator', {width: 390}, () => K.calculatorHiddenField(page));
    await page.setViewportSize({width: 1440, height: 900});
    await run('calculator', {width: 1440}, () => K.calculatorHiddenField(page));
    await run('aborted-run', {width: 1440}, () => K.abortedRun(page, base));
    if (screens) await shot(page, 'abbruch-1440-light.png');
    await run('practice-run-2', {width: 1440}, () => K.practiceResult(page, base));
    await run('profile-return', {width: 1440}, () => K.profileReturn(page));
    await run('logo-alt', {width: 1440}, () => K.logoAlt(page, base));
    await run('report-list', {width: 1440}, () => K.reportList(page, base));
    await ctx.close();
  }
  // Re-QA U3: calculator, measurement details and saved report details scroll to their end (all sizes, both schemes).
  for (const theme of ['light', 'dark']) for (const [width, height] of [[1440, 900], [1280, 720], [1024, 768], [390, 844], [390, 560]]) {
    const {ctx, page} = await context({width, height, theme});
    const meta = {width, height, theme};
    await run('calculator-end', meta, async () => { await K.practicePrepared(page, base); await K.contextSidebarEnd(page, '#open-calculator', '#detail-inspector'); });
    await run('details-end', meta, async () => { await K.practiceResult(page, base); if (width <= 1050) await K.contextSidebarEnd(page, '#details-toggle', '#detail-inspector'); else await K.contextSidebarEnd(page, null, '#detail-inspector'); });
    await run('saved-details-end', meta, async () => {
      const id = await page.evaluate(() => state.report.id);
      await page.goto(base + '/protokoll?id=' + encodeURIComponent(id));
      await page.locator('#saved-result').waitFor();
      if (width <= 1050) await K.contextSidebarEnd(page, '#saved-details', '#saved-inspector'); else await K.contextSidebarEnd(page, null, '#saved-inspector');
    });
    if (screens && width !== 1280 && height !== 560) { await page.goto(base + '/?step=3'); }
    await ctx.close();
  }
  // R039-08: offline prototype (private checkout only).
  {
    const file = join(root, 'docs/prototyp/steuerbox-simulator-prototyp.html');
    let exists = true; try { await (await import('node:fs/promises')).access(file); } catch { exists = false; }
    if (exists) {
      const ctx = await browser.newContext({viewport: {width: 1440, height: 900}, locale: 'de-DE'});
      await ctx.route('**/*', route => { const u = new URL(route.request().url()); return route.request().url() === pathToFileURL(file).href || ['data:', 'blob:', 'about:'].includes(u.protocol) ? route.continue() : route.abort('blockedbyclient'); });
      const page = await ctx.newPage(); page.setDefaultTimeout(4000);
      await run('prototype-delete', {width: 1440}, () => K.prototypeDelete(page, pathToFileURL(file).href));
      await ctx.close();
    }
  }
  // No network: loopback-only server without mocked adapters.
  for (const theme of ['light', 'dark']) for (const width of [1440, 1024, 390]) {
    const {ctx, page} = await context({width, theme, network: false});
    await start(page);
    await shot(page, 'ohne-netzwerk-' + width + '-' + theme + '.png');
    for (const lang of theme === 'light' ? languages : ['de']) { await language(page, lang); await run('no-network', {width, theme, lang}, () => K.noNetwork(page)); }
    await language(page, 'de');
    await run('no-network-practice', {width, theme}, async () => {
      await page.locator('#mode-simulator').check();
      await page.locator('#pair-simulator').click();
      await page.locator('#step-2').waitFor();
      await page.locator('#mode-onsite').evaluate(() => goStep(1));
    });
    await ctx.close();
  }
} catch (e) { results.push({name: 'runner', passed: false, error: e.stack}); }
finally {
  await browser.close();
  server.stdin.end('quit\n'); server.kill('SIGTERM');
  await rm(work, {recursive: true, force: true});
}
await writeFile(join(output, 'qa039.json'), JSON.stringify(results, null, 1) + '\n');
const failed = results.filter(r => !r.passed);
const byName = {};
for (const r of results) { byName[r.name] ??= [0, 0]; byName[r.name][r.passed ? 0 : 1]++; }
for (const [n, [p, f]] of Object.entries(byName)) console.log((f ? 'FAIL ' : 'ok   ') + n + ': ' + p + ' bestanden, ' + f + ' fehlgeschlagen');
const seen = new Set();
for (const r of failed) { const key = r.name + r.error; if (seen.has(key)) continue; seen.add(key); console.error('  ' + r.name + ' ' + JSON.stringify({w: r.width, h: r.height, lang: r.lang, theme: r.theme}) + ': ' + r.error); }
console.log('039 QA: ' + (results.length - failed.length) + '/' + results.length + ' Prüfungen bestanden');
process.exitCode = failed.length ? 1 : 0;
