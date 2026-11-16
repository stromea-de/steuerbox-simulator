// Focused copy regression in the existing synthetic browser harness; no device/network API.
(async()=>{
 const t=window.uiTest={done:false,error:'',checks:0,snapshots:{}};
 const el=id=>document.getElementById(id),pause=()=>new Promise(r=>setTimeout(r,100));
 const assert=(ok,message)=>{t.checks++;if(!ok)throw Error(message);};
 try{
  await pause();StromeaI18n.setLanguage('de');
  assert(!el('device-count').getClientRects().length,'no failed-search count before the first search');
  assert(el('device-results-title').hidden,'no empty device heading');
  for(const id of ['lpc','lpp','failsafe','tolerance','duration-help']){const ids=(el(id).getAttribute('aria-describedby')||'').split(' ');assert(!ids.includes('field-help') && ids.some(id=>id.startsWith('field-description-')),'035: own description '+id);}
  for(const lang of ['de','en','pl','ru','uk','tr','ro']){
   StromeaI18n.setLanguage(lang);await pause();
   for(const id of ['lpc','lpp','failsafe','tolerance']){const button=el(id+'-field').querySelector('[data-tip]');const expected=StromeaI18n.t(StromeaI18n.sourceAttribute(button,'data-tip'))+' '+StromeaI18n.t('Eingabe in Watt.');assert(el('field-description-'+id).textContent===expected,'035: translated individual description and unit '+id+' '+lang);}
   assert(el('field-description-duration-help').textContent===StromeaI18n.t('Die Prüfdauer ergibt sich aus den gewählten Wartezeiten. Eine erfolgreiche Prüfung kann früher enden.'),'035: translated duration description '+lang);
  }
  StromeaI18n.setLanguage('de');await pause();
  assert(!el('capabilities') && !el('lpp-explanation'),'redundant copy removed from the DOM');
  assert(el('title-1').textContent==='Gerät verbinden' && el('nav-1').textContent.includes('Gerät verbinden'),'step 1 title matches navigation');
  assert(el('simulator-trust').textContent==='Wartet auf Freigabe am Gerät…','pending approval is plain status');
  assert(stepName('LPC Heartbeat-Ausfall')==='Bezug: Lebenszeichen unterbrechen','wire names still map to everyday text');
  assert(stepName('Heartbeat starten')==='Lebenszeichen senden','heartbeat is a keep-alive signal');
  const d={name:'Beispielgerät',ski:'0123456789abcdef0123456789abcdef01234567',address:'127.0.0.1:4711'};
  showDevice(d);mockAPI.state={...mockAPI.state,device:d,deviceMode:true,deviceInfo:{...d,type:'HeatPump'},connected:true,paired:true,kind:'connect',trust:{device:true,simulator:true},capabilities:{lpc:true,lpp:true,mpc:false}};
  await refresh();goStep(1);await pause();assert(el('title-1').textContent==='Beispielgerät','connected heading preserves device name');assert(el('device-name').textContent===StromeaI18n.t('Gerätekennung (SKI)'),'fixed device row label');goStep(2);await pause();
  assert(el('capability-detail').compareDocumentPosition(el('connection-timing-details')) & Node.DOCUMENT_POSITION_FOLLOWING,'technical details follow capability list');
  if(innerWidth>=600)assert(getComputedStyle(el('lpc-field').querySelector('p.help')).hyphens==='none','no desktop prose hyphenation');
  assert(!el('measurement-warning').hidden && el('measurement-warning').textContent.includes('Geräteantworten'),'missing measurements explained before test');
  assert(el('capability-detail').querySelector('dd').textContent==='Wärmepumpe','device type is readable');
  assert(el('capability-detail').querySelector('dd').title==='HeatPump','original type remains in tooltip');
  const expectedBudget=StromeaGuide.duration(StromeaGuide.budget({scope:'both',effect:120,outage:120,step:60,cleanup:30,minimum:7200,profile:'quick',measurable:false}),StromeaI18n.locale());
  assert(el('duration').textContent.includes(expectedBudget),'duration keeps the calculated upper bound');
  renderLowLoad({busy:true,progress:{lowLoad:{directions:[{direction:'LPC'},{direction:'LPP'}]}}});assert(!el('low-load-title'),'one low-load heading');assert(el('low-load-text').querySelector('details') && !el('low-load-text').querySelector('details').open,'low-load consequences are available on demand');
  workspace.show('context',el('details-toggle'));el('duration-help').click();StromeaI18n.setLanguage('en');await pause();
  assert(!/Je Prüfrichtung|Ausfallbeobachtung|Steuerbefehle/.test(el('field-help').textContent),'open duration explanation refreshes its formatted translation');
  StromeaI18n.setLanguage('de');workspace.closeQR();
  goStep(1);workspace.show('manual',el('manual-toggle'));const connectCalls=mockAPI.calls.filter(c=>c.path==='/api/connect').length;
  await manualConnect();await pause();assert(el('manual-device').hidden,'manual dialog closes when reusing the already connected device');
  assert(mockAPI.calls.filter(c=>c.path==='/api/connect').length===connectCalls,'reusing the connected device preserves the connection');goStep(2);
  const at=new Date().toISOString(),report={mode:'onsite',schemaVersion:1,config:{scope:'consumption',testProfile:'quick'},started:at,finished:at,device:d,steps:[{name:'LPC Grenze + Quittung',passed:true,at},{name:'Grenzen aufheben',passed:false,at}]};
  mockAPI.state={...mockAPI.state,kind:'run',busy:false,report,outcome:'Nicht bestanden'};await refresh();goStep(3);await pause();
  assert(el('title-3').textContent==='Prüfergebnis','result heading is distinct from PDF');
  assert(!el('result-counts').closest('details')?.open,'result count hidden in collapsed details');assert(el('result-counts').closest('details')?.id==='result-workflow','count remains available in workflow details');
  assert(el('result-counts').textContent==='Funktionen: 1 bestanden, 0 fehlgeschlagen, 0 nicht geprüft, 3 ausstehend.','summary counts the visible table rows');
  assert(el('result-matrix').querySelectorAll('tbody tr').length===4,'four function rows');
  assert(el('release-limits').textContent.startsWith('⚠ ') && el('release-limits').textContent.includes('nicht bestätigt'),'failed cleanup never claims success');
  assert(el('release-failsafe').textContent.startsWith('⚠ '),'missing restoration is not claimed');
  const copy=JSON.stringify(report);renderResult({...mockAPI.state});assert(JSON.stringify(report)===copy,'presentation preserves evidence');
  report.steps.push({name:'LPC Failsafe-Leistung zurückstellen',passed:true,at},{name:'LPC Failsafe-Mindestdauer zurückstellen',passed:true,at});report.steps.find(s=>s.name==='Grenzen aufheben').passed=true;await refresh();await pause();
  assert(el('release-limits').textContent==='✓ Testgrenzen erfolgreich aufgehoben.','confirmed cleanup is concise');
  assert(el('release-failsafe').textContent==='✓ Bisherige Notbetriebs-Werte am Gerät wiederhergestellt.','confirmed restoration is concise');
  for(const lang of ['en','pl','ru','uk','tr','ro']){
   StromeaI18n.setLanguage(lang);await pause();
   assert(el('result-counts').textContent===StromeaI18n.t('Funktionen: {passed} bestanden, {failed} fehlgeschlagen, {notChecked} nicht geprüft, {open} ausstehend.').replace('{passed}','1').replace('{failed}','0').replace('{notChecked}','0').replace('{open}','3'),'localized result summary '+lang);
   assert(el('title-3').textContent===StromeaI18n.t('Prüfergebnis'),'localized outcome heading '+lang);
   assert(el('duration').textContent===StromeaI18n.t('Prüfdauer: höchstens etwa {duration}.').replace('{duration}',StromeaI18n.t(StromeaGuide.duration(1680,StromeaI18n.locale())).replace(/\.$/,'')) && !/\.\.$/.test(el('duration').textContent),'duration refreshes on language change without double period '+lang);
   assert(!/Leistungsmessung|Geräteantworten/.test(el('measurement-warning').textContent),'warning translated '+lang);
  }
  mockAPI.state={...mockAPI.state,busy:true,progress:{lowLoad:{directions:[{direction:'LPC'}]},deadline:new Date(Date.now()+60000).toISOString()}};await refresh();StromeaI18n.setLanguage('en');await pause();
  for(const id of ['outcome','downloads','result-matrix','result-workflow','open-evidence','release-status','site-edit'])assert(el(id).hidden,'no stale result during a new run or language switch: '+id);
  for(const lang of ['de','en','pl','ru','uk','tr','ro']){StromeaI18n.setLanguage(lang);const prefix=StromeaI18n.t('Wähle, wie die Prüfung weitergehen soll. Ohne Entscheidung endet sie in {time}.').split('{time}')[0];assert(el('wait-banner').textContent.startsWith(prefix),'decision countdown translates synchronously: '+lang);}
  el('low-load-text').querySelector('details').open=true;
  for(const lang of ['de','en','pl','ru','uk','tr','ro']){StromeaI18n.setLanguage(lang);assert(el('low-load-text').querySelector('span').textContent===StromeaI18n.t('Für {direction} ist die Leistung zu niedrig.').replace('{direction}',StromeaI18n.t('Bezug')),'035: low-load reason updates synchronously '+lang);assert(el('low-load-text').querySelector('details').open,'035: low-load details survive language switch '+lang);}
  mockAPI.state={...mockAPI.state,busy:false};await refresh();StromeaI18n.setLanguage('de');
  mockAPI.state={...mockAPI.state,busy:false,kind:'pair',deviceMode:false,paired:true,connected:true,capabilities:null,report:null,progress:null};
  real=false;el('scope').value='both';await refresh(true);
  assert(el('scope').value==='both','035b reload: practice scope must not jump when backend has no capability declaration');
  assert([...el('capability-detail').querySelectorAll('dd')].filter(n=>n.textContent===StromeaI18n.t('Unterstützt')).length===3,'035b reload: built-in simulator still supports all three functions');
  goStep(2);mockAPI.state.preparation={};state.preparation={};workspace.refreshGuide();
  el('scope').dispatchEvent(new Event('focus'));assert(el('guide-copy').textContent.includes('LPC') && el('guide-copy').textContent.includes('LPP'),'035b scope help explains power directions');const scopeHelp=el('guide-copy').textContent;el('business').dispatchEvent(new Event('focus'));assert(el('guide-copy').textContent===scopeHelp,'035b company field does not open an empty guide');
  el('lpp').dispatchEvent(new Event('focus'));workspace.refreshGuide();assert(el('production-apply').hidden,'035b no measurement: do not offer a half-power limit');
  el('failsafe').dispatchEvent(new Event('focus'));workspace.refreshGuide();assert(el('original-values').hidden,'035b practice does not claim unread device originals');
  el('tolerance').dispatchEvent(new Event('focus'));workspace.refreshGuide();assert(el('guide-sources').hidden,'035b 50 W tolerance does not cite a BNetzA limit');
  el('lpc').dispatchEvent(new Event('focus'));el('calc-mode').value='direct';el('calc-power').value='';el('calc-power').dispatchEvent(new Event('input'));assert(el('calc-apply').disabled && !el('calc-result').textContent.includes('4.200'),'035b empty calculator cannot imply a result for a placeholder');
  el('calc-power').value='15000';el('calc-power').dispatchEvent(new Event('input'));assert(el('calc-result').textContent.includes('6.000') && !el('calc-apply').disabled,'035b 15 kW WP direct produces 6 kW');
  real=false;modeView();assert(el('run').textContent==='Prüfung starten · Übungsmodus','practice mode naming');
  assert(!el('business').required,'practice fields remain optional');
  state=null;goStep(3);const chart={busy:false,report:{config:{scope:'consumption',lpc:4200,failsafe:500},steps:[],samples:[{watts:4200,at:'2026-11-08T10:00:00Z',step:'LPC gemessene Wirkung'}]}};
  for(const lang of ['de','en','pl','ru','uk','tr','ro']){StromeaI18n.setLanguage(lang);renderPower(chart);assert(!!el('power-reference').querySelector('.chart-reference'),'035c legend created '+lang);renderPower(chart);assert(!!el('power-reference').querySelector('.chart-reference'),'035c identical measurement refresh preserves legend '+lang);const zero=new Intl.NumberFormat(StromeaI18n.locale(),{style:'unit',unit:'second',unitDisplay:'narrow',maximumFractionDigits:0}).format(0);assert([...el('power-chart').querySelectorAll('text')].some(n=>n.textContent===zero),'035c localized axis seconds '+lang);assert(el('power-value').textContent===new Intl.NumberFormat(StromeaI18n.locale(),{useGrouping:true,maximumFractionDigits:3}).format(4200),'035c current-language number format '+lang);}
  t.done=true;
 }catch(e){t.error=e.message+' '+e.stack;t.done=true;}
})();
