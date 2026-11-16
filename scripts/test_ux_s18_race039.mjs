#!/usr/bin/env node
// Review 039 R2 (R039-R2-01), 041: the UX-S18 version line (real correction) must survive a /api/state response that is still
// in flight when the harness freezes refresh, and the version-text mutation must then be detected every time.
// UXS18_RACE_RUNS sets the repetitions (default 10). Own loopback server, practice mode only.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {mkdtemp, rm, mkdir} from 'node:fs/promises';
import {resolve, dirname, join} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..'), runs = Number(process.env.UXS18_RACE_RUNS || 10);
await mkdir(join(root, 'artifacts/039'), {recursive: true});
const work = await mkdtemp(join(root, 'artifacts/039/s18-'));
const l = createServer(); l.listen(0, '127.0.0.1'); await once(l, 'listening'); const port = l.address().port; await new Promise(r => l.close(r));
const base = 'http://127.0.0.1:' + port;
const server = spawn(resolve(root, process.env.E2E_BINARY || 'artifacts/036/check-server'), ['serve', '--state', join(work, 's'), '--listen', '127.0.0.1:' + port, '--loopback-only'], {stdio: ['pipe', 'ignore', 'ignore']});
const {chromium} = await import(pathToFileURL(process.env.E2E_PLAYWRIGHT || join(process.env.HOME, '.cache/stromea-pw/node_modules/playwright-core/index.mjs')));
const browser = await chromium.launch({headless: true});
let failures = 0;
try {
  for (let i = 0; ; i++) { try { if ((await fetch(base + '/api/state')).ok) break; } catch {} if (i > 200) throw Error('server'); await new Promise(r => setTimeout(r, 100)); }
  const page = await (await browser.newContext({viewport: {width: 1280, height: 1000}, locale: 'de-DE'})).newPage();
  await page.goto(base + '/?step=1'); await page.waitForFunction(() => typeof state !== 'undefined' && !!state);
  await page.locator('#mode-simulator').check(); await page.locator('#pair-simulator').click(); await page.locator('#step-2').waitFor();
  await page.locator('#run').click();
  await page.waitForFunction(() => state && !state.busy && state.report && document.querySelector('#outcome-title')?.textContent.trim(), null, {timeout: 120000});
  const versionHits = () => page.evaluate(() => { const text = document.querySelector('#step-3').innerText; return text.match(/Berichtsversion|\bVersion\s*\d/gi) || []; });
  // 041: a real correction creates v2; the version line "v2 · v1" must survive in-flight state responses.
  const v1 = await page.evaluate(() => state.report.id);
  await page.locator('#site-edit-toggle').click(); await page.locator('#report-site-customer').fill('Race 041'); await page.locator('.site-editor button[type=submit]').click();
  await page.waitForFunction(id => state.report?.id !== id && document.querySelector('#step-3 #report-revision strong')?.textContent === 'v2', v1, {timeout: 10000});
  for (let n = 0; n < runs; n++) {
    // Hold one state response back, start it, then freeze exactly like the harness does.
    let release; const held = new Promise(r => release = r);
    await page.route('**/api/state', async route => { await held; await route.continue(); }, {times: 1});
    await page.evaluate(() => { refresh().catch(() => {}); });
    setTimeout(() => release(), 300 + (n % 5) * 150);
    await page.evaluate(async () => { window.uxR = refresh; refresh = async () => {}; while (refreshPromise) await refreshPromise.catch(() => {}); });
    await page.waitForTimeout(1100);
    const kept = await page.evaluate(() => { const b = document.querySelector('#step-3 #report-revision'); return b.checkVisibility() && b.textContent.replace(/\s+/g, ' ').trim() === 'v2 · v1'; });
    await page.evaluate(() => { const r = document.querySelector('#report-revision'); r.prepend(document.createTextNode('Version 2 – ')); });
    const detected = (await versionHits()).length > 0;
    await page.evaluate(() => { const r = document.querySelector('#report-revision'); r.firstChild.remove(); refresh = window.uxR; });
    if (!kept || !detected) { failures++; console.error('run ' + (n + 1) + ': version line kept=' + kept + ', mutation detected=' + detected); }
  }
} finally { await browser.close(); server.stdin.end('quit\n'); server.kill('SIGTERM'); await rm(work, {recursive: true, force: true}); }
console.log('UX-S18 race guard: ' + (runs - failures) + '/' + runs + ' runs kept the fixture and detected the mutation');
assert.equal(failures, 0);
