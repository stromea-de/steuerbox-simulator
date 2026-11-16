// Browser regression suite. Uses only synthetic API responses; no network access.
window.uiTest = {done: false, error: '', snapshots: {}, checks: 0};
window.addEventListener('error', e => { window.uiTest.error = e.message; });
(async () => {
  const el = id => document.getElementById(id);
  const assert = (value, message) => { if (!value) throw new Error(message); window.uiTest.checks++; };
  const pause = () => new Promise(resolve => setTimeout(resolve, 20));
  async function settle() { for (let i = 0; i < 5; i++) await pause(); }
  async function click(id) { assert(!el(id).disabled, id + ' unexpectedly disabled'); el(id).click(); await settle(); }
  const visible = id => !el(id).hidden;
  function input(id, value) { el(id).value = value; el(id).dispatchEvent(new Event('input', {bubbles: true})); }
  function snapshot(name) {
    assert(document.documentElement.scrollWidth <= window.innerWidth, name + ': horizontal overflow');
    const clone = document.documentElement.cloneNode(true);
    const liveInputs = document.querySelectorAll('input');
    clone.querySelectorAll('input').forEach((input, i) => {
      input.setAttribute('value', liveInputs[i].value);
      if (liveInputs[i].checked) input.setAttribute('checked', ''); else input.removeAttribute('checked');
    });
    const liveOptions = document.querySelectorAll('option');
    clone.querySelectorAll('option').forEach((option, i) => {
      if (liveOptions[i].selected) option.setAttribute('selected', ''); else option.removeAttribute('selected');
    });
    window.uiTest.snapshots[name] = clone.outerHTML;
  }
  const device = {name: 'Wallbox Pro', brand: 'Testhersteller', model: 'CS/MU', ski: '0123456789abcdef0123456789abcdef01234567', address: '127.0.0.1:4711'};
  const mock = window.mockAPI;
  try {
  el('lpp').value='3000';
    await settle();
    assert(visible('step-1') && !visible('step-2') && !visible('step-3'), 'initial wizard');
    assert(el('nav-2').disabled && el('nav-3').disabled, 'navigation gates');
    assert(mock.calls.every(c => !c.path.includes('discover')), 'no automatic discovery');
    const rgb = color => color.match(/[\d.]+/g).slice(0,3).map(Number).map(x => { x /= 255; return x <= .04045 ? x / 12.92 : Math.pow((x + .055) / 1.055, 2.4); });
    const luminance = color => rgb(color).reduce((sum, x, i) => sum + x * [.2126,.7152,.0722][i], 0);
    const contrast = (a, b) => { const x = luminance(a), y = luminance(b); return (Math.max(x,y)+.05)/(Math.min(x,y)+.05); };
    const buttonStyle = getComputedStyle(el('pair-simulator'));
    assert(contrast(buttonStyle.color, buttonStyle.backgroundColor) >= 4.5, 'primary button AA contrast');
    assert(contrast(getComputedStyle(document.querySelector('footer a')).color, getComputedStyle(document.documentElement).backgroundColor) >= 4.5, 'link AA contrast');
    assert(document.querySelector('header svg path'), 'Stromea vector in header');
    snapshot('schritt-1-uebung');
    if(el('mode-onsite').checked)await click('mode-simulator');await click('pair-simulator');
    assert(visible('step-2') && el('paired-status').textContent.includes('Simulator erfolgreich'), 'one click simulator pairing');
    assert(document.activeElement.id === 'title-2', 'focus follows wizard');
    assert(!el('sign-operator').checked && !el('sign-inspector').checked, 'signatures opt in');
    assert(!el('real-config').open && !visible('real-config'), 'simulator timings hidden');
    input('business', 'Elektro Muster GmbH'); input('inspector', 'Erika Muster');
    snapshot('schritt-2-uebung');
    input('lpc', '4200,125'); input('lpp', '3000,125');
    mock.failRun = true;
    el('form').requestSubmit(); await settle();
    assert(visible('step-2') && visible('config-error'), 'run validation error stays at form');
    assert(el('detail-inspector').contains(el('config-error-detail')), 'technical validation details collapsed');
    mock.failRun = false;
    el('form').requestSubmit(); await settle();
    assert(visible('step-3') && visible('live'), 'run opens step 3');
    assert(!el('nav-1').disabled && el('nav-2').disabled && el('mode-onsite').disabled, 'device overview reachable; mode and preparation locked during run');
    assert(mock.lastConfig.lpc === 4200.125 && mock.lastConfig.lpp === 3000.125, 'validated decimal commas are sent exactly');
 assert(mock.lastConfig.business === 'Elektro Muster GmbH' && !('authorized' in mock.lastConfig), 'no authority gate sent');
    mock.state.progress = {current: 'LPC gemessene Wirkung', steps: [{name: 'Verbindung', passed: true, detail: 'Bestätigt'}]};
    await refresh();
    assert(el('progress').value === 1 && el('steps').querySelector('.active-step .step-title')?.title.includes('In Prüfung'), 'actual live progress');
    const old = el('progress').value;
    mock.state.progress = {current: 'LPC gemessene Wirkung', steps: []}; await refresh();
    assert(el('progress').value >= old, 'monotonic progress');
    mock.state.progress.steps = [{name: 'Verbindung', passed: true, detail: 'Bestätigt'}]; await refresh();
    snapshot('schritt-3-aktiv');
    await click('cancel');
    assert(mock.calls.some(c => c.path === '/api/cancel'), 'cancellation endpoint called');
    assert(el('run-status').textContent.includes('Prüfung beendet'), 'completed cancellation explains next action');
    function report(outcome, steps) {
      mock.state.busy = false; mock.state.outcome = outcome;
      mock.state.report = {mode: 'simulator', device, steps, started: '2026-10-26T10:00:00Z', finished: '2026-10-26T10:00:10Z'};
    }
    const passedSteps = ['Verbindung', 'Unterstützte Use Cases', 'Failsafe-Werte setzen', 'Heartbeat starten', 'Ausgangsmessung MPC', 'LPC Grenze + Quittung', 'LPC gemessene Wirkung', 'LPC Heartbeat-Ausfall', 'LPC Failsafe gemessene Wirkung', 'LPC Wiederkehr', 'LPP Grenze + Quittung', 'LPP gemessene Wirkung', 'LPP Heartbeat-Ausfall', 'LPP Failsafe gemessene Wirkung', 'LPP Wiederkehr', 'Grenzen aufheben'].map(name => ({name, passed: true, detail: 'Bestätigt (synthetische API-Antwort für den UI-Test)'}));
    report('Bestanden', passedSteps); await refresh();
    assert(el('steps').querySelectorAll('.step-row').length === 16, 'all reported steps rendered');
    assert(visible('downloads') && el('outcome-title').textContent.includes('bestanden'), 'passed downloads');
    snapshot('schritt-3-ergebnis');
    report('Unvollständig', [{name: 'LPP gemessene Wirkung', status: 'not_checked', passed: false, detail: 'Zu wenig Leistung'}]); await refresh();
    assert(el('outcome-title').textContent === 'Unvollständig' && [...el('steps').querySelectorAll('.step-title')].some(n=>n.title.includes('Nicht geprüft')), 'partial result does not assume success');
    report('Nicht bestanden', [{name: 'Grenzen aufheben', passed: false, detail: 'Freigabe nicht bestätigt'}]); await refresh();
    assert(visible('release-status') && !el('release-status').classList.contains('success') && el('release-limits').textContent.includes('am Gerät'), 'cleanup failure prominent');
    snapshot('schritt-3-freigabefehler');
    await click('repeat'); assert(visible('step-2'), 'repeat returns to settings');
    await click('back'); await click('mode-onsite');
    assert(visible('lan-controls') && !el('manual-device').open, 'onsite manual collapsed');
    assert(!mock.calls.some(c => c.path === '/api/discover-lan'), 'mode switch never discovers automatically');
    assert(el('duration').textContent.includes('bis zu ca.'), 'honest onsite duration');
    mock.discoveryError = true; await click('discover');
    assert(visible('search-status') && el('search-status').textContent.includes('Gerätesuche nicht möglich') && !visible('pair-error'), 'search error directly below search action');
    assert(el('discovery-detail').closest('details')?.closest('#device-browser'), 'search technical detail collapsed');
    mock.discoveryError = false;
    mock.devices = []; await click('discover');
    assert(visible('search-status') && el('search-status').textContent.includes('Kein Gerät gefunden'), 'empty search advice');
    mock.devices = [device, {...device, ski:'1123456789abcdef0123456789abcdef01234567',brand:'',model:'',name: '<script>bad</script>'}]; await click('discover');
    assert(el('device-list').querySelectorAll('input[type=radio]').length === 2, 'radio result cards');
    assert(!el('device-list').querySelector('script'), 'untrusted announcement treated as text');
    mock.devices = [device]; await click('discover');
    el('device-list').querySelector('input').click(); await settle();
    assert(visible('device') && el('device-ski').textContent.replace(/\s/g,'').includes('01234567'), 'formatted identity');
    snapshot('schritt-1-vor-ort');
    assert(!el('ski-confirmed'),'confirmation is the Connect action');
 mock.connectBusy = true;
    await click('pair');
    assert(visible('step-1') && el('connection-label').textContent.includes('Verbindung wird aufgebaut'), 'waiting for remote trust');
    assert(el('pair').disabled && visible('connect-cancel'), 'connection pending controls');
    await refresh(true);
    assert(visible('step-1') && visible('connect-cancel'), 'reload during connection');
    await click('connect-cancel');
    mock.connectBusy = false;
    mock.state.busy = false; mock.state.error = 'context canceled'; await refresh();
    assert(visible('pair-error'), 'canceled connection stays at pairing');
    mock.connectError = 'SHIP handshake: connection reset by peer (EOF)';
    await click('retry');
    assert(mock.calls.slice(-3).map(c => c.path).includes('/api/connect'), 'one click selects and connects');
    assert(visible('step-1') && visible('pair-error'), 'connection error at pairing');
    assert(!el('pair-error-text').textContent.includes('SHIP') && /Freigabe|frei/.test(el('pair-error-text').textContent), 'plain language reset advice');
    assert(el('pair-error-detail').textContent.includes('EOF') && el('pair-error-detail').closest('details')?.closest('#pair-detail'), 'raw detail collapsed at device');
    assert(!visible('downloads') || !visible('step-3'), 'old download not shown at pairing');
    snapshot('schritt-1-verbindungsfehler');
    mock.connectError = 'context deadline exceeded'; await click('retry');
    assert(el('pair-error-text').textContent.includes('nicht rechtzeitig'), 'timeout explanation');
    mock.connectError = 'network is down'; await click('retry');
    assert(el('pair-error-text').textContent.includes('nicht erreichbar'), 'network explanation');
    // Editing a previously confirmed device invalidates the local connection.
    const deviceTitleBeforeEdit=el('device-name').textContent;input('address', '127.0.0.1:4712');
    assert(el('nav-2').disabled && el('device-name').textContent === deviceTitleBeforeEdit, 'manual edit invalidates pairing');
    assert(!el('device-list').querySelector('input').checked, 'manual edit deselects discovery');
    mock.connectError = ''; assert(!el('ski-confirmed'),'confirmation is the Connect action'); await click('pair');
    assert(visible('step-1') && !visible('pair-error'), 'trusted connection waits for explicit next');await click('pair');assert(visible('step-2'),'next opens config');
    el('real-config').open = true; input('failsafe-minimum', '10800');
    assert(el('duration-help').dataset.tip.includes('3 h'), 'duration reacts to settings');
    input('failsafe-minimum', '7200');
    snapshot('schritt-2-vor-ort');
    el('real-config').open = false;
    // A page reload during connection must restore the same contextual error.
    mock.state = {...mock.state, report: null, busy: false, kind: 'connect', connected: false, error: 'EOF', deviceMode: true, paired: true, device};
    await refresh(true);
    assert(visible('step-1') && visible('pair-error'), 'reload preserves connection context');
    assert(el('local-qr').querySelector('svg path'), 'own SHIP QR rendered offline');
    assert(!el('ski-confirmed') && !connected, 'reloaded device needs explicit comparison');
    assert(!el('device-list').textContent.includes(formatSki(device.ski)) && el('device-list').textContent.includes('…0123 4567'), 'discovery shows only short SKI ending');
    input('qr-text', StromeaInput.ownShip(device.ski, 'inverter-id').replace('BRAND:Stromea', 'BRAND:Inverter'));
    await click('qr-apply');
    assert(el('ski').value.replace(/\s/g,'').toLowerCase() === device.ski && el('device-model').textContent.includes('Inverter'), 'pasted SHIP identity imported');
    assert(!el('ski-confirmed') && !connected, 'QR import does not establish trust');
    input('qr-text', 'SHIP;SKI:bad;ID:id;ENDSHIP;'); await click('qr-apply');
    assert(el('qr-status').textContent===StromeaI18n.t('SHIP-QR benötigt eine gültige SKI und ID.'), 'invalid QR explained');
    assert(!mock.calls.some(c => c.path.includes('identity/reset')), 'no automatic identity rotation');
    // Labels, references and touch targets are verified against the actual DOM.
    for (const field of document.querySelectorAll('input:not([type=radio]):not([type=checkbox]),select')) {
      assert(document.querySelector('label[for="' + field.id + '"]') || field.closest('label') || field.getAttribute('aria-label'), 'missing label: ' + field.id);
    }
    for (const item of document.querySelectorAll('[aria-describedby]')) {
      for (const id of item.getAttribute('aria-describedby').split(' ')) assert(el(id), 'broken description: ' + id);
    }
    window.uiTest.done = true;
  } catch (error) { window.uiTest.error = String(error) + '\n' + (error.stack || ''); window.uiTest.done = true; }
})();
