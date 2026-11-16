// Motion regressions in the real DOM; all API replies are synthetic and local.
window.uiTest = {done: false, error: '', snapshots: {}, checks: 0};
window.addEventListener('error', e => { window.uiTest.error = e.message; });
(async () => {
  const t = window.uiTest, el = id => document.getElementById(id), mock = window.mockAPI;
  const assert = (ok, message) => { if (!ok) throw new Error(message); t.checks++; };
  const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
  const settle = () => pause(360);
  const click = async id => { assert(!el(id).disabled, id + ' enabled'); el(id).click(); await pause(30); };
  const shot = (name, time) => {
    assert(document.documentElement.scrollWidth <= innerWidth, name + ': no horizontal overflow');
    // Sample actual WebKit keyframes and freeze their computed transforms/opacity.
    const animations = document.getAnimations(), times = animations.map(a => a.currentTime);
    if (time !== undefined) animations.forEach(a => { a.currentTime=time; });
    const clone = document.documentElement.cloneNode(true), source = [...document.documentElement.querySelectorAll('*')], targets = [...clone.querySelectorAll('*')];
    let frozen = '*,*::before,*::after{animation:none!important;transition:none!important}';
    source.forEach((node,i) => {
      const style = getComputedStyle(node); targets[i].style.transform=style.transform; targets[i].style.opacity=style.opacity;
      for (const pseudo of ['::before','::after']) {
        const p = getComputedStyle(node,pseudo);
        if (p.content && p.content !== 'none') {
          targets[i].setAttribute('data-motion-capture',String(i));
          frozen += '[data-motion-capture="'+i+'"]'+pseudo+'{transform:'+p.transform+'!important;opacity:'+p.opacity+'!important}';
        }
      }
    });
    const liveInputs = document.querySelectorAll('input');
    clone.querySelectorAll('input').forEach((node,i) => {
      node.setAttribute('value',liveInputs[i].value);
      if (liveInputs[i].checked) node.setAttribute('checked',''); else node.removeAttribute('checked');
    });
    const liveOptions = document.querySelectorAll('option');
    clone.querySelectorAll('option').forEach((node,i) => {
      if (liveOptions[i].selected) node.setAttribute('selected',''); else node.removeAttribute('selected');
    });
    const style = document.createElement('style'); style.textContent=frozen; clone.querySelector('head').append(style);
    clone.querySelectorAll('script').forEach(node => node.remove());
    t.snapshots[name] = '<!doctype html>' + clone.outerHTML;
    if (time !== undefined) animations.forEach((a,i) => { a.currentTime=times[i]; });
  };
  t.contrasts = {};
  const contrast = (foreground, background) => {
    const rgb = color => color.match(/[\d.]+/g).slice(0,3).map(Number);
    const luminance = color => rgb(color).map(x=>{x/=255;return x<=.04045?x/12.92:Math.pow((x+.055)/1.055,2.4);}).reduce((sum,x,i)=>sum+x*[.2126,.7152,.0722][i],0);
    const a=luminance(foreground),b=luminance(background); return (Math.max(a,b)+.05)/(Math.min(a,b)+.05);
  };
  const checkContrast = name => {
    const background=getComputedStyle(el('power-monitor')).backgroundColor;
    for (const id of ['power-status','power-reference','heartbeat-status']) {
      const ratio=contrast(getComputedStyle(el(id)).color,background);
      assert(ratio>=4.5,id+' AA text contrast'); t.contrasts[name+'-'+id]=Math.round(ratio*100)/100;
    }
    const trace=el('power-chart').querySelector('.power-trace');
    if(trace) { const ratio=contrast(getComputedStyle(trace).stroke,background); assert(ratio>=3,'power curve contrast'); t.contrasts[name+'-curve']=Math.round(ratio*100)/100; }
  };
  const checkAnimations = () => {
    for (const a of document.getAnimations()) {
      const timing = a.effect.getTiming();
      assert(timing.iterations===Infinity ? timing.duration >= 1800 : timing.duration <= 600, 'short transitions and quiet loops');
      for (const frame of a.effect.getKeyframes()) {
        assert(Object.keys(frame).every(key => ['offset','computedOffset','easing','composite','transform','opacity'].includes(key)), 'only compositor properties animate');
      }
    }
  };
  try {
  el('lpp').value='3000';
    await pause(80);
    const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
    assert(document.documentElement.classList.contains('reduce-motion') === reduced, 'media preference applied');
    if(el('mode-onsite').checked)await click('mode-simulator');await click('pair-simulator');
    assert(!el('step-2').hidden && document.activeElement.id === 'title-2', 'navigation and focus are immediate');
    checkAnimations();
    shot('assistent-030ms',30); shot('assistent-150ms',150); shot('assistent-299ms',299);
    if (reduced) assert(document.getAnimations().length === 0, 'reduced navigation has no animations');
    // Rapid forward/backward actions leave no stale cards or blocked controls.
    goStep(1); goStep(2); goStep(1); goStep(2);
    await settle();
    assert(!el('step-2').hidden && el('step-1').hidden, 'rapid navigation settles correctly');
    el('business').value = 'Elektro Muster'; el('inspector').value = 'Fachkraft';
    el('form').requestSubmit(); await pause(50);
    assert(el('power-value').textContent === '–', 'no invented live values');
    const steps = [{name:'Ausgangsmessung MPC', passed:true, detail:'MPC', watts:6200, at:'2026-10-28T10:00:00Z'}];
    mock.state.progress = {current:'LPC gemessene Wirkung', steps}; await refresh();
    const firstRow = el('steps').firstElementChild;
    assert(firstRow.querySelector('.raw-technical').textContent==='MPC', 'diagnosis shown directly in inspector');
    await refresh();
    assert(el('steps').firstElementChild === firstRow && firstRow.querySelector('.raw-technical').textContent==='MPC', 'polling preserves rows and expanded details');
    await settle();
    assert(el('power-value').textContent.includes('6200'), 'tween reaches exact reported watts');
    assert(el('power-chart').querySelectorAll('.power-point').length === 1, 'one actual sample');
    checkContrast('normal'); shot('leistung-bezug');
    mock.state.progress.current = 'LPC Failsafe gemessene Wirkung'; await refresh();
    assert(el('power-monitor').classList.contains('failsafe-phase') && el('heartbeat-status').textContent.includes('Reaktion wird geprüft'), 'failsafe is explicit');
    assert(!el('heartbeat-symbol'),'status text replaces heart');
    checkContrast('notbetrieb'); shot('notbetrieb');
    steps.push({name:'LPC Failsafe gemessene Wirkung', passed:true, detail:'MPC', watts:500, at:'2026-10-28T10:00:03Z'});
    mock.state.progress = {current:'LPP gemessene Wirkung', steps:[...steps, {name:'LPP Ausgangsmessung MPC', passed:true, watts:-4000, at:'2026-10-28T10:00:05Z'}]};
    await refresh(); await settle();
    assert(el('power-direction').textContent.includes('Einspeisung'), 'negative power keeps direction');
    assert(el('steps').firstElementChild === firstRow && firstRow.querySelector('.raw-technical').textContent==='MPC', 'new progress preserves prior rows and details');
    assert(el('power-chart').querySelectorAll('.power-point').length === 3, 'missing polls do not fabricate samples');
    assert(el('power-chart').textContent.includes('4200') && el('power-chart').textContent.includes('500'), 'reference lines have text labels');
    const pass = {mode:'simulator', steps, config:{lpc:4200,lpp:3000,failsafe:500}, started:'2026-10-28T10:00:00Z',finished:'2026-10-28T10:00:10Z'};
    mock.state = {...mock.state,busy:false,report:pass,outcome:'Bestanden'}; await refresh();
    assert(el('result-badge').dataset.outcome === 'passed', 'passed badge');
    const confetti = el('outcome').querySelectorAll('.confetti-piece');
    assert(reduced ? confetti.length === 0 : confetti.length > 0, 'confetti only with full motion');
    await refresh();
    assert(el('outcome').querySelectorAll('.confetti-piece').length === confetti.length, 'polling does not restart confetti');
    checkAnimations(); await settle(); shot('bestanden');
    for (const [outcome, variant] of [['Unvollständig','partial'],['Nicht bestanden','failed']]) {
      mock.state.outcome=outcome; await refresh();
      assert(el('result-badge').dataset.outcome===variant && !el('outcome').querySelector('.confetti-piece'), 'honest '+outcome+' badge without confetti');
      await settle(); shot(variant);
    }
    await click('repeat'); await click('back'); await click('mode-onsite');
    // Hold just the discovery reply; the indicator must follow the actual request.
    const originalFetch = window.fetch; let release;
    window.fetch = async (path, options) => { if (path === '/api/discover-lan') await new Promise(resolve => { release=resolve; }); return originalFetch(path,options); };
    mock.devices = [{name:'Loopback Wallbox',ski:'0123456789abcdef0123456789abcdef01234567',address:'127.0.0.1:4711'}];
    await click('discover');
    assert(!el('search-motion').hidden && !el('discover').disabled && el('discover').textContent==='Suche stoppen', 'radar follows pending search');
    checkAnimations(); shot('suche'); release(); await pause(60);
    assert(el('search-motion').hidden && el('device-list').children.length===1, 'radar ends with reply');
    el('device-list').querySelector('input').click(); assert(!el('ski-confirmed'),'confirmation is the Connect action');
    mock.connectBusy=true; await click('pair');
    assert(el('connection-motion').dataset.phase==='connecting', 'connection data line active');
    checkAnimations(); await settle(); shot('kopplung',240);
    mock.state.busy=false; mock.state.trust={device:true,simulator:false};mock.state.error='Verbindung abgewiesen'; await refresh();
    assert(el('connection-motion').hidden && !el('pair-error').hidden, 'rejection text and red state');
    await settle(); shot('abgelehnt');
    mock.connectBusy=false; mock.connectError=''; await click('retry'); await settle();
    assert(!el('step-1').hidden && el('paired-status').querySelector('.draw-check'), 'drawn success check before next');await pause(700);await click('pair');assert(!el('step-2').hidden,'explicit next');
    goStep(1); el('qr-import').open=true;
    el('qr-video').hidden=false; el('qr-stop').hidden=false;
    await pause(20); assert(!el('qr-frame').hidden, 'scanner frame only while camera visible');
    stopCamera(); await pause(20); assert(el('qr-frame').hidden, 'scanner frame stops with camera');
    // A restored run has no config in the state API: don't invent default reference lines.
    await refresh(true);
    renderPower({busy:true,progress:{steps:[{name:'MPC',watts:0,at:'2026-10-28T10:00:00Z'}],current:''}});
    assert(el('power-chart').querySelectorAll('.reference-line').length===0 && el('power-chart').querySelectorAll('.power-point').length===1, 'unknown restored limits stay unknown');
    // Clipboard confirmation and tooltip focus are visible without motion.
    Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async()=>{}}});
    goStep(1); el('own-identity').open=true; await click('copy-ski');
    assert(el('copy-status').textContent.includes('✓ Schlüssel kopiert'), 'copy has explicit confirmation');
    goStep(2); const tip = el('lpc'); tip.focus();
    assert(document.activeElement===tip && tip.getAttribute('aria-describedby').includes('field-help') && el('field-help').textContent.includes('LPC'), 'tooltip focus and description reference');
    // Change the preference during a tween: exact value and all effects settle now.
    goStep(3); resetPower(); renderPower({busy:true,progress:{steps:[{name:'MPC',watts:1234.125,at:'2026-10-28T10:00:00Z'}],current:'Heartbeat starten'}});
    window.setTestReducedMotion(true);
    assert(el('power-value').textContent==='1234,125', 'runtime reduction finishes number tween exactly');
    assert(document.getAnimations().length===0, 'runtime reduction cancels every animation');
    assert(!el('outcome').querySelector('.confetti-piece'), 'runtime reduction removes confetti');
    // New run / restored initial state must not retain a previous curve.
    mock.state = {...mock.state,kind:'',busy:false,report:null,deviceMode:false,paired:false,connected:false}; await refresh(true);
    assert(el('power-value').textContent==='–' && el('power-chart').querySelectorAll('.power-point').length===0, 'reset clears old measurement');
    t.done=true;
  } catch(error) { t.error=String(error)+'\n'+(error.stack||''); t.done=true; }
})();
