// 042 (Nutzer 2026-11-14): Browserprüfung für UX-S42 (Ergebnistitel nennt den Prüfumfang; Notbetriebs-Hinweis
// mit Handlung). Unabhängig von StromeaResult.outcomeTitle formuliert. Loopback/Mock only.
import assert from 'node:assert/strict';

export async function resultTitle(page) {
  const v = await page.evaluate(() => {
    const source = document.querySelector('#saved-report'), report = source ? JSON.parse(source.textContent) : state.report, outcome = source ? source.dataset.outcome : state.outcome;
    const c = report?.config || {}, scope = {consumption: 'LPC', production: 'LPP', both: 'LPC/LPP'}[c.scope];
    const profile = c.testProfile === 'full' || (!c.testProfile && c.longTest) ? 'Langzeitprüfung' : 'Funktionsprüfung';
    const stateWord = outcome === 'Bestanden' ? 'bestanden' : outcome === 'Nicht bestanden' ? 'nicht bestanden' : report?.abortReason === 'user_cancel' ? 'abgebrochen' : 'unvollständig';
    const hint = 'Wiederherstellung der Notbetriebswerte nicht bestätigt. Stelle am Gerät Leistungsgrenze und Mindestdauer für Kommunikationsausfall wieder wie vor der Prüfung ein.';
    const scopeNode = document.querySelector('#saved-result:not([hidden])') || document.querySelector('#step-3');
    const release = scopeNode?.querySelector('#release-failsafe'), fallback = document.querySelector('#saved-fallback .pass strong,#saved-fallback .fail strong,#saved-fallback .subtle strong');
    // The server renders the fallback from the stored report; a test route that only rewrites the embedded JSON
    // (release-unconfirmed scenario) leaves it on the stored outcome. Compare only when both describe the same outcome.
    const fallbackClass = fallback?.parentElement.className, sameOutcome = fallbackClass === (outcome === 'Bestanden' ? 'pass' : outcome === 'Nicht bestanden' ? 'fail' : 'subtle');
    return {lang: StromeaI18n.language(), scope, profile, stateWord, title: document.querySelector('#outcome-title')?.textContent.trim() || '',
      expected: scope ? StromeaI18n.t('{umfang}-' + profile + ' ' + stateWord).replace('{umfang}', scope) : null,
      german: scope ? scope + '-' + profile + ' ' + stateWord : null, mode: report?.mode,
      release: release && release.checkVisibility() ? release.textContent.trim() : null, hint: StromeaI18n.t(hint),
      old: (scopeNode?.innerText || '').includes('Rückstellung der Notbetriebs-Werte'), fallback: fallback && sameOutcome ? fallback.textContent.trim() : null};
  });
  assert(v.scope, 'result report records its scope');
  assert.equal(v.title, v.expected, 'title names the checked scope');
  assert(v.title.includes(v.scope), 'scope visible in the title: ' + v.title);
  if (v.lang === 'de') {
    assert.equal(v.title, v.german, 'German title');
    if (v.fallback !== null) assert.equal(v.fallback, v.german, 'view without JavaScript uses the same title');
  }
  assert(!v.old, 'old failsafe wording gone');
  if (v.release !== null) assert.equal(v.release, v.hint, 'failsafe hint names what to do');
}
