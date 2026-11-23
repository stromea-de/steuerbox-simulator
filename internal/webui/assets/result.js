'use strict';
// Shared recorded-data view; never infers backend test decisions.
window.StromeaResult = {
 async downloadPDF(a){
  const region=a.closest('#downloads,.report-downloads')?.querySelector('.download-error');
  if(region){region.hidden=true;region.textContent='';}
  a.setAttribute('aria-busy','true');
  try{
   const response=await fetch(a.href);if(!response.ok || !response.headers.get('Content-Type')?.includes('application/pdf'))throw Error('pdf');
   const url=URL.createObjectURL(await response.blob()),link=document.createElement('a');link.href=url;link.download='EEBUS-Protokoll.pdf';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
  }catch{if(region){region.textContent='PDF konnte nicht erstellt werden. Versuche es erneut oder exportiere die Messdaten.';region.hidden=false;}}
  finally{a.removeAttribute('aria-busy');}
 },
 siteEditor(container,report,onSaved=()=>{}){
  const doc=container.ownerDocument,form=doc.createElement('form');form.className='site-editor';
  for(const [key,label] of [['customer','Kunde (Name)'],['customerAddress','Adresse des Anlagenbetreibers'],['address','Anlagenadresse (falls abweichend)'],['meter','Zähler- oder Anlagennummer'],['note','Notiz']]){
   const l=doc.createElement('label'),input=doc.createElement(key==='note'?'textarea':'input');l.textContent=label;input.id='report-site-'+key;input.name=key;input.maxLength=1000;input.value=report.config?.site?.[key] || '';input.autocomplete='off';l.append(input);form.append(l);
  }
  // 041 (Nutzer 2026-11-14): Felder in einer Karte voller Breite; am Desktop die Schaltfläche rechts daneben, der Hinweis darunter.
  const fields=doc.createElement('div');fields.className='site-editor-fields';fields.append(...form.children);
  const privacy=doc.createElement('p');privacy.className='help site-editor-intro';privacy.textContent='Eine Korrektur erzeugt eine neue Berichtsversion mit neuem PDF und neuer Prüfsumme. Die frühere Version bleibt abrufbar. Messdaten und Prüfergebnis bleiben unverändert.';
  const save=doc.createElement('button');save.type='submit';save.textContent='Neue Berichtsversion speichern';
  const side=doc.createElement('div');side.className='site-editor-actions';side.append(save,privacy);
  const status=doc.createElement('p');status.className='site-editor-status';status.setAttribute('role','status');form.append(fields,side,status);
  form.addEventListener('submit',async e=>{
   e.preventDefault();save.disabled=true;
   const site=Object.fromEntries(new FormData(form)),token=doc.querySelector('meta[name="session-token"]')?.content;
   try{
    const response=await fetch('/api/report/site'+(report.id?'?id='+encodeURIComponent(report.id):''),{method:'POST',headers:{'Content-Type':'application/json','X-Stromea-Token':token},body:JSON.stringify(site)});
    if(!response.ok)throw Error('save');const updated=await response.json();Object.assign(report,updated);
    status.textContent='';onSaved(report);container.hidden=true;doc.getElementById('site-edit')?.classList.remove('site-edit-open');const toggle=doc.getElementById('site-edit-toggle');toggle?.setAttribute('aria-expanded','false');toggle?.focus(); /* 041: kein „Kundendaten gespeichert.“ – die Kundenzeile zeigt die neuen Angaben */
   }catch{status.textContent='Angaben konnten nicht gespeichert werden. Erneut versuchen.';}
   finally{save.disabled=false;}
  });container.replaceChildren(form);
 },
 counts(report){
  const states=new Map();
  for(const step of report.steps || []){const checkpoint=step.category?step.category==='checkpoint':/Ausgangsmessung MPC|Grenze \+ Quittung|gemessene Wirkung|(?:LPC|LPP) Wiederkehr$|Wiederkehr Protokoll|Vorzeitige Freigabe/.test(step.name);if(!checkpoint)continue;const rank=step.status==='not_checked'?2:step.passed?1:3;states.set(step.name,Math.max(states.get(step.name)||0,rank));}
  const c={passed:0,failed:0,notChecked:0};for(const rank of states.values())c[rank===1?'passed':rank===2?'notChecked':'failed']++;return c;
 },
 // Fold recorded steps conservatively: a retry cannot hide a prior failure.
 matrix(report){
  const rows=[],scope=report.config?.scope || 'both',rank={passed:0,not_checked:1,open:2,failed:3};
  for(const item of report.steps || []){
   const match=item.name==='Ausgangsmessung MPC'?['','LPC',item.name]:/^(LPC|LPP) (.*)$/.exec(item.name),key=match?match[2]:item.name;
   let row=rows.find(r=>r.key===key);if(!row){row={key,directed:!!match,lpc:scope==='production'?'excluded':'open',lpp:scope==='consumption'?'excluded':'open'};rows.push(row);}
   const status=item.status==='not_checked'?'not_checked':item.passed?'passed':'failed';
   const directions=match?[match[1].toLowerCase()]:['lpc','lpp'];
   for(const direction of directions){if(row[direction]==='excluded')continue;const seen=direction+'Seen';if(!row[seen] || rank[status]>rank[row[direction]])row[direction]=status;row[seen]=true;}
  }
  return rows;
 },
 // 042 (UX-S42, Nutzer 2026-11-14): the result title names the checked scope ("LPC-Funktionsprüfung bestanden"),
 // same rule as inspection.ResultTitle; legacy reports without a recorded scope keep the previous title.
 outcomeTitle(report,outcome){
  const c=report?.config || {},profile=c.testProfile==='full' || (!c.testProfile && c.longTest)?'Langzeitprüfung':'Funktionsprüfung',scope={consumption:'LPC',production:'LPP',both:'LPC/LPP'}[c.scope];
  if(!scope)return StromeaI18n.t(outcome==='Bestanden'?profile+' bestanden':outcome);
  const state=outcome==='Bestanden'?'bestanden':outcome==='Nicht bestanden'?'nicht bestanden':report.abortReason==='user_cancel'?'abgebrochen':'unvollständig';
  return StromeaI18n.t('{umfang}-'+profile+' '+state).replace('{umfang}',scope);
 },
 isCheckpoint(item){return item.category?item.category==='checkpoint':/Ausgangsmessung MPC|Grenze \+ Quittung|gemessene Wirkung|(?:LPC|LPP) Wiederkehr$|Wiederkehr Protokoll|Vorzeitige Freigabe/.test(item.name);},
 workflow(report){return (report.steps || []).filter(item=>!this.isCheckpoint(item));},
 formMatrix(report){
  const scope=report.config?.scope || 'both',full=report.config?.testProfile==='full' || !report.config?.testProfile && report.config?.longTest;
  const rows=['Grenze angenommen','Grenze wirkt','Notbetrieb bei Ausfall','Rückkehr zum Normalbetrieb'];if(full)rows.push('Notbetrieb hält Mindestdauer');
  const output=rows.map(title=>({title,lpc:'open',lpp:'open'}));
  for(const kind of ['LPC','LPP']){
   const dir=kind.toLowerCase();if(kind==='LPC' && scope==='production' || kind==='LPP' && scope==='consumption'){output.forEach(r=>r[dir]='excluded');continue;}
   const baseline=kind==='LPC'?'Ausgangsmessung MPC':'LPP Ausgangsmessung MPC';
   let effect=[kind+' gemessene Wirkung'];if(kind==='LPC' || scope==='production' || report.schemaVersion>=1 && report.mode==='onsite')effect.unshift(baseline);
   let recovery=[kind+' Wiederkehr'];if(report.mode==='onsite' && report.config?.scope)recovery=full?[kind+' Wiederkehr',kind+' Wiederkehr gemessene Wirkung']:[kind+' Vorzeitige Freigabe',kind+' Vorzeitige Rückkehr gemessene Wirkung'];
   const groups=[[kind+' Grenze + Quittung'],effect,[kind+' Heartbeat-Ausfall',kind+' Failsafe gemessene Wirkung'],recovery];if(full)groups.push([kind+' Wiederkehr']);
   for(const [index,names] of groups.entries()){
    let missing=false,failed=false;
    for(const name of names){const attempts=(report.steps || []).filter(item=>item.name===name);if(!attempts.length)missing=true;
     for(const item of attempts){let unchecked=item.status==='not_checked';if(name===baseline){const match=/^MPC (-?[0-9]+(?:\.[0-9]+)?) W(?:;.*)?$/.exec(item.detail || ''),value=typeof item.watts==='number'?item.watts:match?Number(match[1]):null;const power=kind==='LPP'?-value:value;const limit=kind==='LPP'?report.config?.lpp:report.config?.lpc;if(value!==null && power<=limit+(report.config?.tolerance || 0))unchecked=true;}
      if(unchecked)missing=true;else if(!item.passed)failed=true;
     }
    }
    output[index][dir]=failed?'failed':missing?'not_checked':'passed';
    if(names.some(name=>!(report.steps || []).some(item=>item.name===name)) && !failed)output[index][dir]='open';
   }
  }
  return output;
 },
 functionCounts(report){
  const counts={passed:0,failed:0,notChecked:0,open:0};
  const rank={passed:0,not_checked:1,open:2,failed:3};
  for(const row of this.formMatrix(report)){
   const states=[row.lpc,row.lpp].filter(status=>status!=='excluded');
   if(!states.length)continue;
   const status=states.reduce((worst,value)=>rank[value]>rank[worst]?value:worst,'passed');
   counts[status==='not_checked'?'notChecked':status]++;
  }
  return counts;
 },
 evidenceGroups(steps,reason){const groups=new Map();for(const item of steps){const key=reason(item);if(!groups.has(key))groups.set(key,[]);groups.get(key).push(item);}return groups;},
 svgNode(name,attrs={}) {const n=document.createElementNS('http://www.w3.org/2000/svg',name);for(const [k,v] of Object.entries(attrs))n.setAttribute(k,v);return n;},
 drawnCheck(){const svg=this.svgNode('svg',{viewBox:'0 0 32 32',class:'draw-check','aria-hidden':'true'});for(const [transform,length] of [['translate(7 16) rotate(45)',8],['translate(12.65 21.65) rotate(-45)',17]]){const group=this.svgNode('g',{transform});group.append(this.svgNode('path',{d:'M0 0H'+length}));svg.append(group);}return svg;},
   create(root, {isReal=()=>true, motion=()=>{}, motionQuery={matches:true}, svgNode, drawnCheck, saved=false,enter=()=>{},motionEffects=new Map(),actions={}}) {
 const $=id=>root.querySelector('#'+id) || root.ownerDocument.getElementById(id);
 const wattsText=value=>value.toLocaleString(StromeaI18n.locale(),{maximumFractionDigits:3,useGrouping:true});
 let observedRange={};
 let powerAt=null,powerFrame=0,powerShown=null,powerTarget=null,powerSignature='',resultSignature='',powerConfig=null,renderedSteps='',resultContentSignature='';
 root.innerHTML=`<p class="eyebrow">SCHRITT 3 VON 3</p>
<h2 id="title-3" tabindex="-1">Prüfergebnis</h2>
<div id="live">
<strong id="current-step">
</strong>
<p id="run-status" role="status" aria-live="polite">Prüfung läuft</p>
<progress id="progress" max="16" value="0" aria-label="Abgeschlossene Prüfpunkte">
</progress>
<section id="low-load" class="decision-card" hidden aria-labelledby="low-load-title">
<div class="decision-head"><h3 id="low-load-title">Wie soll die Prüfung weitergehen?</h3><span id="low-load-timer" class="decision-timer" role="timer" title="Ohne Entscheidung endet die Prüfung, wenn die Zeit abgelaufen ist."></span></div>
<p class="help" id="low-load-intro">Die Leistung reicht für den Nachweis nicht aus. Am Gerät wurde noch nichts verändert.</p>
<div id="low-load-text"></div>
<div class="decision-options">
<div class="decision-option"><button id="remeasure">Erneut messen</button><p class="option-copy">Last einschalten, z. B. Fahrzeug laden oder Heizung anfordern, dann erneut messen.</p></div>
<div class="decision-option"><button id="continue-low" class="secondary">Nur Geräteantworten prüfen</button><p class="option-copy">Prüft, ob das Gerät die Steuerbefehle bestätigt. Leistungsänderung und Notbetrieb stehen im Bericht als nicht geprüft.</p></div>
</div>
</section>
<button id="cancel" class="secondary">Prüfung abbrechen</button>
<p class="help">Am Ende versucht der Simulator, die Testgrenzen aufzuheben.</p>
</div>
<p id="control-warning" class="notice warning" role="alert" hidden>Gerät antwortet nicht auf Steuerbefehle</p><p id="wait-clock" role="timer" hidden></p><p id="wait-banner" class="notice warning" role="status" hidden></p><button id="reconnect" class="secondary" hidden>Verbindung neu aufbauen</button>
<div id="outcome" class="notice" role="status" hidden>
<h3 id="outcome-title">
</h3>
<div id="result-abort" class="result-cause" role="status" hidden><strong id="result-cause-title"></strong><p id="result-cause-text"></p><button type="button" id="result-cause-action" hidden></button></div>
<p id="outcome-text">
</p>
<p id="result-scope">
</p>
<p id="result-meta" class="help">
</p>
<p id="result-customer" class="help customer-line"><span id="result-customer-data" data-i18n-rendered></span><span id="customer-sep"> · </span><a id="site-edit-toggle" class="text-link" href="#site-edit" aria-expanded="false" aria-controls="site-editor">Kundendaten korrigieren</a></p>
</div>
<section id="site-edit" hidden><div id="site-editor" hidden></div></section>
<div id="downloads" hidden><p class="download-error error" role="alert" hidden></p>
<a class="button" href="/api/report.pdf" download="EEBUS-Protokoll.pdf">PDF-Prüfbericht laden</a>
<a class="text-link" href="/api/report.json" title="JSON" download="EEBUS-Protokoll.json">Messdaten exportieren</a>
<p id="report-revision" class="report-versions" hidden></p>
<p class="help" id="unpair-reminder">Entferne nach der Prüfung die Kopplung am Gerät. Nach dem Trennen kann das Gerät bis zur Mindestdauer im Notbetrieb bleiben.</p>
<p class="notice warning" id="failsafe-warning" hidden>Notbetriebs-Werte am Gerät prüfen</p>
<p class="help" id="report-storage" data-i18n-rendered></p>
</div>
<section id="open-evidence" hidden><h3>Offene Nachweise</h3><table><thead><tr><th>Prüfschritt</th><th>Ergebnis</th><th>Grund</th></tr></thead><tbody id="evidence-list"></tbody></table></section>
<section id="result-matrix" hidden><h3>Geprüfte Funktionen</h3><table><thead><tr><th>Funktion</th><th>Bezug (LPC)</th><th>Einspeisung (LPP)</th></tr></thead><tbody></tbody></table><p id="release-failsafe" class="help" hidden></p><p class="help">✓ <span>Bestanden</span> · ○ <span>Ausstehend</span> · – <span>Nicht geprüft</span> · × <span>Fehlgeschlagen</span></p></section>
<div id="result-curve">
</div>
<div id="run-error" class="error" role="alert" hidden>
<h3>Prüfung unterbrochen</h3>
<p>Verbindung prüfen. Grenzaufhebung abwarten.</p>
<details>
<summary>Technische Details</summary>
<pre id="run-error-detail">
</pre>
</details>
</div>
<section id="all-steps">
<h3>Live-Protokoll</h3>
<ol id="steps" class="steps" aria-label="Prüfschritte">
</ol>
</section>
<div class="next-actions">
<button id="repeat" class="secondary" hidden>Erneut prüfen</button>
<button id="other-device" class="secondary" hidden>Anderes Gerät</button>
</div>`;
 // UX-S17: a real link to the form; click/Enter toggles it in place (no fragment jump). aria-disabled links have no function.
 $('site-edit-toggle').addEventListener('click',e=>{e.preventDefault();if(e.currentTarget.getAttribute('aria-disabled')==='true')return;const editor=$('site-editor');editor.hidden=!editor.hidden;$('site-edit').classList.toggle('site-edit-open',!editor.hidden);$('site-edit-toggle').setAttribute('aria-expanded',String(!editor.hidden));});
 const resultHeader=document.createElement('div');resultHeader.className='result-header';$('outcome').before(resultHeader);resultHeader.append($('outcome'),$('downloads'));resultHeader.after($('result-matrix'));resultHeader.after($('site-edit'));
 $('current-step').after($('wait-clock'),$('wait-banner'));$('result-matrix').after($('open-evidence'));
 const monitor=document.createElement('section');monitor.id='power-monitor';monitor.className='power-monitor';monitor.setAttribute('aria-label','Leistungsverlauf');monitor.innerHTML=`<h3>Messverlauf</h3>
<div class="power-heading">
<span class="eyebrow">AKTUELLE LEISTUNG</span>
<span id="heartbeat-status">
<span id="heartbeat-label">
</span>
</span>
</div>
<p class="power-reading">
<span id="power-value" aria-hidden="true">–</span> <span class="subtle">W</span> <span id="power-direction" class="subtle">
</span>
</p>
<p id="power-status" class="help" role="status">Noch kein Messwert.</p>
<p id="power-age" class="help">
</p>
<p id="power-technical" class="measurement-details">
</p>
<p class="chart-caption" id="power-chart-caption" hidden></p>
<svg id="power-chart" class="power-chart" data-direction="LPC" viewBox="0 0 720 260" role="img" aria-labelledby="power-chart-title power-chart-description">
<title id="power-chart-title">Gemessener Leistungsverlauf</title>
<desc id="power-chart-description">Noch keine Messpunkte.</desc>
</svg>
<p class="chart-caption" id="power-chart-lpp-caption" hidden></p>
<svg id="power-chart-lpp" class="power-chart" data-direction="LPP" viewBox="0 0 720 260" role="img" aria-labelledby="power-chart-lpp-title" hidden>
<title id="power-chart-lpp-title">Gemessener Leistungsverlauf</title>
</svg>
<p id="power-reference" class="help">Positive Werte zeigen den Bezug, negative die Einspeisung. Lücken zeigen fehlende Messwerte.</p>
<table id="power-table">
<caption>Letzte Messwerte</caption>
<thead>
<tr>
<th>Zeit</th>
<th>Prüfschritt</th>
<th>Leistung</th>
</tr>
</thead>
<tbody>
</tbody>
</table>`;
 $('result-curve').append(monitor);
 $('downloads').querySelector('a[download$=".pdf"]').addEventListener('click',e=>{e.preventDefault();StromeaResult.downloadPDF(e.currentTarget);});
 const badge=document.createElement('span');badge.id='result-badge';badge.className='result-badge';badge.setAttribute('aria-hidden','true');const heading=document.createElement('div');heading.className='outcome-heading';$('outcome').prepend(heading);heading.append(badge,$('outcome-title'));
function resetPower(config = null,resetRun=true) {if(resetRun){observedRange={};}
 powerConfig=config;if(resetRun){resultSignature='';renderedSteps='';}
 powerAt=null;
  cancelAnimationFrame(powerFrame); powerShown = null; powerTarget = null; powerSignature = '';
  $('power-value').textContent = '–'; $('power-direction').textContent = '';
  $('power-status').textContent = 'Noch kein Messpunkt verfügbar.';
  for(const id of ['power-chart','power-chart-lpp'])$(id).querySelectorAll('g').forEach(node => node.remove());
  $('power-chart-description').textContent = 'Noch keine Messpunkte.'; $('power-reference').textContent = '';
  $('power-monitor').classList.remove('failsafe-phase','heartbeat-running'); $('heartbeat-label').textContent = 'Lebenszeichen bereit'; $('power-table').querySelector('tbody').replaceChildren(); $('power-age').textContent='';
}
function tweenPower(value) {
  const changed=value!==powerTarget;
  powerTarget=value;powerShown=Math.abs(value);$('power-value').textContent=wattsText(Math.abs(value));
  if(changed)motion($('power-value'),[{opacity:.45},{opacity:1}],180);
}
function powerPhase(name) {
  // Preparation readings precede any limit; they are baseline, not "Steuerung aktiv".
  if (/Ausgangsmessung|Last vor|Lastentscheidung|^Verbindung$|Use Cases|Ursprungswerte|Failsafe-Werte setzen|^Heartbeat starten$/.test(name)) return 'Ausgang';
  if (/Failsafe gemessene|Heartbeat-Ausfall/.test(name)) return 'Notbetrieb';
  // "Wiederkehr" returns to the still active limit; only release steps end control (Review 037c).
  if (/Vorzeitige Rückkehr|Freigabe|freigeben|aufheben|zurückstellen/.test(name)) return 'Freigabe';
  return 'Grenze';
}
// UX-S21: phase at t from the recorded limit timeline, the same time base as the limit lines.
function limitPhase(limits,t){let failsafe=false,active=false,ever=false;
  for(const direction of ['LPC','LPP']){let current=null;for(const c of limits){if(c.direction!==direction||Date.parse(c.at)>t)continue;current=c;if(typeof c.watts==='number')ever=true;}
   if(current&&typeof current.watts==='number'){if(current.failsafe)failsafe=true;else active=true;}}
  return failsafe?'Notbetrieb':active?'Grenze':ever?'Freigabe':'Ausgang';}
function updatePowerAge(){if(powerAt)$('power-age').textContent=StromeaI18n.t('Letzter Messwert · vor {seconds} s').replace('{seconds}',Math.max(0,Math.round((Date.now()-Date.parse(powerAt))/1000)));}

function renderPower(s) {
  const steps=s.report?.steps || s.progress?.steps || [],current=s.busy?s.progress?.current || '':'';
  const interrupted=/Heartbeat-Ausfall|Failsafe gemessene Wirkung/.test(current);
  const direction=current.startsWith('LPP')?'LPP':'LPC';
  const measured=steps.some(item=>item.passed && item.name===direction+' Failsafe gemessene Wirkung');
  $('power-monitor').classList.toggle('failsafe-phase',interrupted);
  $('heartbeat-label').textContent=interrupted?(measured?'Notbetrieb bestätigt':'Lebenszeichen unterbrochen. Notbetrieb wird geprüft.'):!s.busy?'Prüflauf beendet':'Lebenszeichen aktiv';
  let samples=(s.report?.samples || s.progress?.samples || steps.filter(item=>typeof item.watts==='number').map(item=>({...item,step:item.name}))).slice(-600);
  const allValid=samples.filter(item=>typeof item.watts==='number' && Number.isFinite(item.watts) && Number.isFinite(Date.parse(item.at)));
  if(s.busy && samples.length){const since=Date.parse(samples[samples.length-1].at)-180000;samples=samples.filter(item=>Date.parse(item.at)>=since);}
  const valid=samples.filter(item=>typeof item.watts==='number' && Number.isFinite(item.watts) && Number.isFinite(Date.parse(item.at)));
  if (!allValid.length) {resetPower(powerConfig,false);for(const id of ['power-chart','power-chart-lpp']){$(id).toggleAttribute('hidden',true);$(id+'-caption').hidden=true;}$('power-table').hidden=true;$('power-reference').hidden=true;$('power-heading')?.setAttribute('hidden','');$('power-status').textContent=s.busy?'Warte auf ersten Messwert …':'Keine auswertbaren Messwerte: Das Gerät meldet keine Messwerte oder die Messung war nicht verfügbar.';return;}
 $('power-table').hidden=false;$('power-reference').hidden=false;
  const latest=allValid[allValid.length-1],config=s.report?.config || s.config || powerConfig;
  $('power-technical').textContent=(isReal()?'Leistungsmessung des Geräts · Zeit: ':'Simulierter Messwert · Zeit: ')+new Date(latest.at).toLocaleString(StromeaI18n.locale())+' · Leistung: '+latest.watts.toLocaleString(StromeaI18n.locale())+' W · Prüfschritt: '+stepName(latest.step);
  powerAt=latest.at;updatePowerAge();
  const signature=JSON.stringify([samples,config,(s.report?.samples?s.report.limits:s.progress?.limits)||[],!!s.busy,innerWidth<600,innerWidth<850,StromeaI18n.language()]);if(signature===powerSignature)return;powerSignature=signature;
  $('power-reference').textContent=(samples.length>=600?'Letzte 600 Messpunkte · ':'')+'Positive Werte zeigen den Bezug, negative die Einspeisung. Lücken zeigen fehlende Messwerte.';

  tweenPower(latest.watts);$('power-direction').textContent=latest.watts<0?'Einspeisung':'Bezug';
  $('power-status').textContent=(isReal()?'Leistungsmessung des Geräts':'Simulierter Messwert')+(s.busy?' · '+stepName(latest.step):'');
  const charts=[$('power-chart'),$('power-chart-lpp')];
  if(!valid.length){charts.forEach(c=>c.querySelector('g')?.remove());$('power-chart-description').textContent='Noch keine Messpunkte.';$('power-table').querySelector('tbody').replaceChildren();return;}
  const compact=innerWidth<600,left=compact?68:80,right=compact?345:700;
  const limits=((s.report?.samples?s.report.limits:s.progress?.limits)||[]).filter(c=>Number.isFinite(Date.parse(c.at)));
  const isValid=item=>typeof item.watts==='number'&&Number.isFinite(item.watts)&&Number.isFinite(Date.parse(item.at));
  // UX-S23: one chart per direction; a reading belongs to the section of the step it was recorded in.
  const lpcOn=config?.scope!=='production',lppOn=config?.scope!=='consumption',sections=[];
  const addSection=(direction,list)=>{if(list.some(isValid))sections.push({direction,list});};
  if(!lppOn)addSection('LPC',samples);else if(!lpcOn)addSection('LPP',samples);else{const at=samples.findIndex(item=>/^LPP /.test(item.step||''));const split=at<0?samples.length:at;addSection('LPC',samples.slice(0,split));addSection('LPP',samples.slice(split));}
  function drawChart(svg,direction,list){
   svg.setAttribute('viewBox',compact?'0 0 360 260':'0 0 720 260');const group=svgNode('g'),own=limits.filter(c=>c.direction===direction),points=list.filter(isValid);
   const refs=(direction==='LPC'?[[config?.lpc,'Bezug '+wattsText(config?.lpc??0)+' W §14a','lpc'],[config?.failsafe,'Bezug '+wattsText(config?.failsafe??0)+' W Notbetrieb','failsafe']]:[[-config?.lpp,'Einspeisung '+wattsText(config?.lpp??0)+' W','lpp'],[-config?.failsafe,'Einspeisung '+wattsText(config?.failsafe??0)+' W Notbetrieb','failsafe']]).filter(r=>Number.isFinite(r[0]));
   const values=[0,...points.map(v=>v.watts),...refs.map(r=>r[0]),...own.filter(c=>typeof c.watts==='number').map(c=>c.watts)];
   const range=observedRange[direction]||(observedRange[direction]={min:0,max:0});range.max=Math.max(range.max,...values);range.min=Math.min(range.min,...values);
   const max=range.max,min=range.min,raw=Math.max(1,(max-min)/4),base=10**Math.floor(Math.log10(raw));
   const tick=[1,2,2.5,5,10].find(n=>n*base>=raw)*base;
   const lo=Math.floor(min/tick)*tick,hi=Math.max(lo+tick,Math.ceil(max/tick)*tick);
   const y=v=>220-(v-lo)/(hi-lo)*180;
   const times=list.map(item=>Date.parse(item.at)).filter(Number.isFinite),start=Math.min(...times),observed=Math.max(...times),end=Math.max(start+1000,observed);
   const x=t=>left+(right-left)*(t-start)/(end-start);
   function text(x,y,value,cls='chart-label') {const node=svgNode('text',{x,y,class:cls});node.textContent=value;node.style.fontSize=(compact?(cls==='phase-label'?13:11):(cls==='phase-label'?14:12))+'px';group.append(node);return node;}
   // UX-S19: step-after. A reading holds until the next one (horizontal), changes are vertical;
   // an explicit failed read ends the hold at its timestamp and breaks the trace.
   let previous=null,path='',phaseLabelRight=-Infinity,contiguous=false;
   const bands=[];
   for(const item of list) {
    const t=Date.parse(item.at);
    if(!Number.isFinite(item.watts) || !Number.isFinite(t)) {if(Number.isFinite(t)){const gap=svgNode('path',{d:'M'+x(t)+' 40V220',class:'chart-gap'}),label=svgNode('title');label.textContent=StromeaI18n.t('Messausfall');gap.append(label);group.append(gap);if(previous && t>=previous.t)path+=' H'+x(t);}previous=null;contiguous=false;continue;}
    const px=x(t),py=y(item.watts),joined=previous && t>=previous.t;
    // Older reports without a limit timeline: phases from the recorded steps, contiguous readings only.
    if(!own.length){const phase=powerPhase(item.step),last=bands[bands.length-1];if(contiguous&&last){last.end=px;if(last.phase!==phase)bands.push({start:px,end:px,phase});}else bands.push({start:px,end:px,phase});contiguous=true;}
    path+=(joined?' H'+px+' V'+py:' M'+px+' '+py);
    previous={t,step:item.step};
   }
   // Recorded timeline (UX-S21): phases cut at every change of this direction within the observed time.
   if(own.length){const cuts=[start,...own.map(c=>Date.parse(c.at)).filter(t=>t>start&&t<observed),observed].sort((a,b)=>a-b);
    for(let i=0;i+1<cuts.length;i++){if(cuts[i+1]<=cuts[i])continue;const phase=limitPhase(own,cuts[i]),last=bands[bands.length-1];if(last&&last.phase===phase){last.end=x(cuts[i+1]);continue;}bands.push({start:x(cuts[i]),end:x(cuts[i+1]),phase});}}
   // UX-S23: phases as measure arrows above the plot, no coloured areas.
   const labelRows=[],placed=[];
   for(const b of bands){if(b.end-b.start<.5)continue;const span=svgNode('g',{class:'phase-span phase-'+b.phase,'data-from':b.start,'data-to':b.end}),name=svgNode('title');name.textContent=b.phase;
    span.append(name,svgNode('path',{d:'M'+b.start+' 30H'+b.end,class:'phase-arrow'}),svgNode('path',{d:'M'+b.start+' 30l6 -3.5v7z',class:'phase-arrow-head'}),svgNode('path',{d:'M'+b.end+' 30l-6 -3.5v7z',class:'phase-arrow-head'}));group.append(span);
    // Review 037d/038: every arrow keeps a visible name; a name that does not fit moves one row up.
    const label=svgNode('text',{x:0,y:20,class:'phase-label','data-i18n-rendered':''});label.textContent=StromeaI18n.t(b.phase);label.style.fontSize=(compact?13:14)+'px';svg.append(label);
    const width=Math.max(label.getComputedTextLength?.()||0,label.textContent.length*(compact?7.6:8.2)*.8),labelX=Math.max(left,Math.min((b.start+b.end)/2-width/2,(compact?360:720)-width));
    let row=0;while(row<labelRows.length&&labelX<labelRows[row]+8)row++;labelRows[row]=labelX+width;placed.push({label,row,labelX});}
   const rowGap=(compact?13:14)+9;/* 039 (QA-07): stacked names keep a clear gap */for(const {label,row,labelX} of placed){const ly=20-rowGap*row;label.setAttribute('x',labelX);label.setAttribute('y',ly);group.append(label);}
   if(labelRows.length>1){const extra=rowGap*(labelRows.length-1);svg.setAttribute('viewBox','0 '+(-extra)+' '+(compact?360:720)+' '+(260+extra));}
   for(let value=lo;value<=hi+tick/100;value+=tick){const py=y(value);group.append(svgNode('path',{d:'M'+left+' '+py+'H'+right,class:'chart-grid'}));text(0,py+4,value.toLocaleString(StromeaI18n.locale(),{maximumFractionDigits:3}));}
   const seconds=new Intl.NumberFormat(StromeaI18n.locale(),{style:'unit',unit:'second',unitDisplay:'narrow',maximumFractionDigits:0});
   text(0,22,'W');text(left,248,seconds.format(0));text(right-45,248,seconds.format(Math.round((end-start)/1000)));
   // UX-S21: recorded limit timeline as dashed steps (failsafe dotted), only while a limit applied.
   {const changes=own;let previous=null;
    for(const [i,c] of changes.entries()){const value=c.watts;let from=Date.parse(c.at),to=i+1<changes.length?Date.parse(changes[i+1].at):observed;
     if(typeof value!=='number'||!Number.isFinite(value)){previous=null;continue;}
     const level=y(value),clipped=from<start;if(clipped)from=start;if(to>observed)to=observed;if(to<=from){previous=level;continue;}
     const d=(previous!==null&&!clipped?'M'+x(from)+' '+previous+' V'+level:'M'+x(from)+' '+level)+' H'+x(to);
     const line=svgNode('path',{d,class:'limit-line '+direction.toLowerCase()+(c.failsafe?' failsafe':'')+(c.unconfirmed?' unconfirmed':'')}),title=svgNode('title');title.textContent=(value<0?'Einspeisung ':'Bezug ')+wattsText(Math.abs(value))+' W'+(c.failsafe?' Notbetrieb':direction==='LPC'?' §14a':'');line.append(title);group.append(line);previous=level;}}
   group.append(svgNode('path',{d:path,class:'power-trace'}));
   for(const item of points){const dot=svgNode('circle',{cx:x(Date.parse(item.at)),cy:y(item.watts),r:1.5,class:'power-point'}),title=svgNode('title');title.textContent=stepName(item.step)+': '+wattsText(item.watts)+' W';dot.append(title);group.append(dot);}
   svg.append(group);return refs;
  }
  const refs=[];
  charts.forEach((svg,i)=>{const section=sections[i],caption=$(svg.id+'-caption');svg.querySelector('g')?.remove();svg.toggleAttribute('hidden',!section);caption.hidden=!section;if(!section)return;svg.dataset.direction=section.direction;caption.textContent=section.direction==='LPC'?'Bezug (LPC)':'Einspeisung (LPP)';refs.push(...drawChart(svg,section.direction,section.list));});
  // Reference labels belong below the plot so they cannot obscure measurements.
  const reference=document.createElement('span');reference.className='chart-reference';reference.textContent=refs.map(([,label])=>label).join(' · ');$('power-reference').append(reference);if(samples.some(item=>!Number.isFinite(item.watts))){const gap=document.createElement('span');gap.className='chart-gap-label';gap.textContent='Messausfall';$('power-reference').append(gap);}
  {const note=document.createElement('span');note.className='chart-limits';note.textContent=limits.length?'Gestrichelt: Grenze ab Quittung; gepunktet: Notbetrieb ab Ablauf der Ausfallfrist.':!s.busy?'Kein Grenzverlauf aufgezeichnet.':'';if(note.textContent)$('power-reference').append(note);
   if(limits.some(c=>c.unconfirmed)){const extra=document.createElement('span');extra.className='chart-limits';extra.textContent='Hell gepunktet: Notbetrieb oder Rückkehr daraus nicht nachgewiesen.';$('power-reference').append(extra);}}
  $('power-chart-description').textContent=valid.length+' Messpunkte. Bezug positiv, Einspeisung negativ. Markierte Lücken zeigen Messausfälle.';
  const tbody=$('power-table').querySelector('tbody');tbody.replaceChildren();
  for(const item of valid.slice(-4)){const row=document.createElement('tr');for(const value of [new Date(item.at).toLocaleTimeString(StromeaI18n.locale()),limits.length?limitPhase(limits,Date.parse(item.at)):powerPhase(item.step),wattsText(item.watts)+' W']){const td=document.createElement('td');td.textContent=value;row.append(td);}tbody.append(row);}
}
function resultMotion(s) {
  const signature = JSON.stringify([s.report?.started,s.report?.finished,s.outcome]);
  if (signature === resultSignature) return; resultSignature = signature;
  $('outcome').querySelectorAll('.confetti-piece').forEach(node=>node.remove());
  const badge = $('result-badge'), passed = s.outcome === 'Bestanden';
  badge.dataset.outcome = passed?'passed':s.outcome==='Nicht bestanden'?'failed':'partial'; badge.replaceChildren();
  const ring = svgNode('svg',{viewBox:'0 0 48 48',class:'result-ring'}); ring.append(svgNode('circle',{cx:24,cy:24,r:21})); badge.append(ring);
  if (passed) badge.append(drawnCheck());
  else { const mark = document.createElement('span'); mark.textContent = s.outcome==='Nicht bestanden'?'×':'–'; badge.append(mark); }
  motion(badge,[{opacity:0,transform:'scale(.7) rotate(-12deg)'},{opacity:1,transform:'scale(1) rotate(0)'}],340);
  if (!passed || motionQuery.matches || document.hidden) return;
  for (let i=0;i<6;i++) {
    const piece=document.createElement('i'); piece.className='confetti-piece'; piece.setAttribute('aria-hidden','true');
    piece.style.left=(20+i*12)+'%'; $('outcome').append(piece);
    motion(piece,[{opacity:0,transform:'translateY(-8px) rotate(0)'},{opacity:.8,offset:.2,transform:'translateY(0) rotate(25deg)'},{opacity:0,transform:'translateY(64px) rotate('+(i%2?100:-100)+'deg)'}],520);
    const effect=motionEffects.get(piece);
    if (effect) { const cleanup=()=>{ motionEffects.delete(piece); piece.remove(); }; effect.onfinish=cleanup; effect.oncancel=cleanup; }
    else piece.remove();
  }
}


// UX-S20: the whole translated sentence carries the link word as {link}…{/link}; nodes are built in that order
// and excluded from the text walker. The offline prototype has no server for a new tab and stays in its window.
function renderStorage() {
  const note=$('report-storage'),doc=note.ownerDocument,proto=!!window.StromeaOfflinePrototype,text=StromeaI18n.t('Alle {link}Protokolle{/link} werden lokal gespeichert.');
  const m=/^([\s\S]*)\{link\}([\s\S]+)\{\/link\}([\s\S]*)$/.exec(text) || ['','',text.replace(/\{\/?link\}/g,''),''];
  const a=doc.createElement('a');a.href='/protokolle';a.textContent=m[2];
  if(!proto){a.target='_blank';a.rel='noopener noreferrer';a.className='external-link';const hint=doc.createElement('span');hint.className='sr-only';hint.textContent=' '+StromeaI18n.t('(öffnet in neuem Tab)');a.append(hint);}
  note.replaceChildren(...[m[1],a,m[3]].filter(part=>part!=='').map(part=>typeof part==='string'?doc.createTextNode(part):part));
}
function stepName(name) {
 if(!isReal() && /^(LPC|LPP) Wiederkehr$/.test(name))return (name.startsWith('LPP')?'Einspeisung':'Bezug')+': Mindestdauer verkürzt simulieren';
  if(!name)return '';
  const names={'Lastentscheidung vor Start':'Leistung für die Prüfung zu niedrig','Last vor Prüfbeginn prüfen':'Ausgangsleistung prüfen','Verbindung':'Datenverbindung prüfen','Unterstützte Use Cases':'Gerätefunktionen prüfen','Failsafe-Werte setzen':'Notbetrieb konfigurieren','Heartbeat starten':'Lebenszeichen senden','Ausgangsmessung MPC':'Ausgangsleistung messen','LPP Ausgangsmessung MPC':'Einspeisung messen','Grenzen aufheben':'Testgrenzen aufheben'};
  if(names[name])return names[name];
  const direction=name.startsWith('LPP')?'Einspeisung':'Bezug',suffix=name.replace(/^(LPC|LPP) /,'');
  const actions={'Ausgangsmessung MPC':'Ausgangsleistung messen','Grenze + Quittung':'Grenze setzen','gemessene Wirkung':'Leistungsanpassung messen','Heartbeat-Ausfall':'Lebenszeichen unterbrechen','Failsafe gemessene Wirkung':'Notbetrieb messen','Wiederkehr':'Mindestdauer abwarten','Wiederkehr gemessene Wirkung':'Rückkehr nach Mindestdauer messen','Heartbeat wieder starten':'Lebenszeichen senden','Grenze freigeben':'Grenze freigeben','Grenzen aufheben':'Testgrenzen aufheben','Vorzeitige Freigabe':'Freigabe versuchen','Vorzeitige Rückkehr gemessene Wirkung':'Rückkehr messen','Ausfallfrist abwarten':'Ausfallfrist abwarten','Wiederkehr Protokoll':'Grenze nach neuem Lebenszeichen quittieren','Failsafe Ursprungswerte':'Notbetriebs-Werte sichern','Failsafe-Leistung zurückstellen':'Notbetriebs-Leistung zurückstellen','Failsafe-Mindestdauer zurückstellen':'Notbetriebs-Dauer zurückstellen'};
  return actions[suffix]?direction+': '+actions[suffix]:name;
}

function evidenceReason(item) {
 const detail=item.detail || '';
 if(detail.includes('Langtest'))return 'Mindestdauer nicht geprüft: In der Funktionsprüfung wird sie nicht abgewartet';
 if(detail.includes('MPC nicht') || detail.includes('MPC fehlt'))return 'Das Gerät meldet keine Leistungsmessung.';
 if(detail.includes('nicht vom Gerät gemeldet'))return 'vom Gerät nicht gemeldet';
 if(/zu wenig Leistung/i.test(detail))return 'Die Leistung reicht für diesen Nachweis nicht aus.';
 if(detail.includes('meldet keinen Notbetrieb'))return 'Das Gerät meldet keinen Notbetrieb.';
 if(detail.includes('Notbetrieb nicht bewertbar'))return 'Notbetrieb nicht bewertbar: Zeitbezug der Lebenszeichen unklar.';
 if(detail.includes('nicht unterscheidbar'))return 'Notbetrieb nicht unterscheidbar: Kein Leistungsabfall, der erst durch den Notbetrieb erklärbar ist.';
 if(detail.includes('Notbetrieb nicht nachgewiesen'))return 'Rückkehr nicht bewertbar: Der Notbetrieb wurde nicht nachgewiesen.';
 if(detail.includes('Vorzeitige Rückkehr nicht nachgewiesen'))return 'Die Rückkehr wurde nicht bestätigt. Notbetrieb am Gerät prüfen.';
 if(detail.includes('Keine Änderung ohne lesbare Ursprungswerte'))return 'Die bisherigen Notbetriebs-Werte konnten nicht gelesen werden. Es wurden keine Werte verändert.';
 return item.status==='not_checked'?'Dieser Nachweis fehlt. Prüfschritt öffnen.':!item.passed?'Der Prüfschritt ist fehlgeschlagen. Einstellungen und Gerät prüfen.':'Bestätigt';
}
let versionsFor='',versionsRequest=0;
function renderVersions(r){
 const box=$('report-revision'),toggle=$('site-edit-toggle'),id=r.id || '';
 const apply=list=>{
  box.replaceChildren();box.hidden=list.length<2;
  list.forEach((v,i)=>{if(i)box.append(document.createTextNode(' · '));const label='v'+v.number;if(v.id===id){const b=document.createElement('strong');b.textContent=label;b.setAttribute('aria-current','page');box.append(b);}else{const a=document.createElement('a');a.className='text-link';a.href='/protokoll?id='+encodeURIComponent(v.id);a.textContent=label;a.title=StromeaI18n.t('Version {number} öffnen').replace('{number}',v.number);box.append(a);}});
  const newest=!list.length || list[0].id===id;toggle.hidden=!newest;$('customer-sep').hidden=!newest;if(!newest){$('site-editor').hidden=true;$('site-edit').classList.remove('site-edit-open');toggle.setAttribute('aria-expanded','false');}
 };
 if(versionsFor===id && box.dataset.versions){apply(JSON.parse(box.dataset.versions));return;}
 versionsFor=id;box.dataset.versions='';apply([]);if(!id || window.StromeaOfflinePrototype)return;
 const request=++versionsRequest;
 fetch('/api/report/versions?id='+encodeURIComponent(id)).then(response=>response.ok?response.json():[]).then(list=>{if(request!==versionsRequest || !Array.isArray(list))return;box.dataset.versions=JSON.stringify(list);apply(list);}).catch(()=>{});
}
function renderCustomer(r){
 const box=$('result-customer-data'),site=r.config?.site || {};box.replaceChildren();
 const parts=[['Kunde',site.customer],['Adresse Anlagenbetreiber',site.customerAddress],['Anlagenadresse',site.address && site.address!==site.customerAddress?site.address:''],['Zähler / Anlage',site.meter],['Notiz',site.note]].filter(([,v])=>v && v.trim());
 if(!parts.length){const n=document.createElement('span');n.textContent=StromeaI18n.t('Kunde: nicht eingetragen');box.append(n);}
 parts.forEach(([label,value],i)=>{const k=document.createElement('span'),v=document.createElement('span');k.textContent=StromeaI18n.t(label)+': ';v.textContent=value.replace(/\s*\n\s*/g,', ');v.dataset.userContent='true';if(i)box.append(document.createTextNode(' · '));box.append(k,v);});
}
// 043 (Nutzer 2026-11-14): why a run ended early stands directly below the result title – what happened, in
// which step, after how long – with exactly one action. No second warning card further down.
function renderCause(r){
 const box=$('result-abort'),reason=r.abortReason,t=StromeaI18n.t;
 const shown=['connection_lost','timeout','user_cancel'].includes(reason);box.hidden=!shown;
 if(!shown){$('result-cause-action').hidden=true;return false;}
 // R043-03: the report field is „reason“; the first step with the abort reason is where it happened (later cleanup
 // failures – resetting values, releasing limits – are consequences, not the cause).
 const steps=r.steps || [],cleanup=item=>/zurückstellen|Grenzen aufheben/.test(item.name || '');
 const at=steps.find(item=>item.reason===reason && !cleanup(item)) || steps.find(item=>!item.passed && item.status!=='not_checked' && !cleanup(item)) || steps.find(item=>item.reason===reason) || steps.at(-1);
 const seconds=at?Math.max(0,Math.round((Date.parse(at.at)-Date.parse(r.started))/1000)):NaN,time=Number.isFinite(seconds)?t('{time} min').replace('{time}',Math.floor(seconds/60)+':'+String(seconds%60).padStart(2,'0')):'';
 const name=at?t(stepName(at.name)):'',decision=reason==='timeout' && at?.name==='Lastentscheidung vor Start'; // R043-04: the backend names an expired decision explicitly
 const fill=text=>t(text).replace('{step}',name).replace('{time}',time);
 const copy=reason==='connection_lost'?['Verbindung zum Gerät unterbrochen','Im Schritt „{step}“ nach {time}. Das Gerät war nicht mehr erreichbar, zum Beispiel wegen schwachem WLAN. Prüfe die Netzwerkverbindung des Geräts und verbinde neu.','Neu verbinden','reconnect']
  :decision?['Keine Entscheidung zur Leistung','Die Leistung war zu niedrig und es wurde nicht rechtzeitig entschieden. Am Gerät wurde nichts verändert.','Erneut prüfen','repeat']
  :reason==='timeout'?['Zeitlimit erreicht','Im Schritt „{step}“ nach {time}: Das Gerät hat nicht rechtzeitig geantwortet.','Erneut prüfen','repeat']
  :['Prüfung abgebrochen','Du hast die Prüfung im Schritt „{step}“ nach {time} abgebrochen.','Erneut prüfen','repeat'];
 $('result-cause-title').textContent=reason==='user_cancel'?'':t(copy[0]);$('result-cause-title').hidden=reason==='user_cancel'; // the result title already says „abgebrochen“
 $('result-cause-text').textContent=fill(copy[1]);
 $('result-cause-title').dataset.i18nRendered='';$('result-cause-text').dataset.i18nRendered='';
 box.className='result-cause '+(reason==='user_cancel'?'cause-neutral':'cause-warning');
 const action=$('result-cause-action'),handler=actions[copy[3]];action.hidden=saved || !handler;action.textContent=t(copy[2]);action.dataset.i18nRendered='';action.onclick=handler || null;
 return true;
}
function renderResult(s) {
  const signature=JSON.stringify([s.report,s.outcome,StromeaI18n.language()]);if(signature===resultContentSignature)return;resultContentSignature=signature;
  const r=s.report,outcome=s.outcome;resultMotion(s);
   const cause=renderCause(r);

 $('site-edit').hidden=false;if($('site-editor').dataset.report!==JSON.stringify([r.id,r.started])){StromeaResult.siteEditor($('site-editor'),r,()=>{const win=root.ownerDocument.defaultView;if(saved && root.isConnected && ['http:','https:'].includes(win.location.protocol) && win.location.pathname==='/protokoll'){const url=new URL(win.location.href);url.searchParams.set('id',r.id);win.history.replaceState(win.history.state,'',url);}resultContentSignature='';renderResult({...s,report:r});});$('site-editor').dataset.report=JSON.stringify([r.id,r.started]);}
 for(const [suffix,path] of [['pdf','/api/report.pdf'],['json','/api/report.json']])$('downloads').querySelector('a[download$=".'+suffix+'"]').href=path+'?id='+encodeURIComponent(r.id || '');
 // 041 (UX-S18): Versionen als „v2 · v1“, angezeigte Version fett, die anderen als Link. Ältere Versionen
 // lassen sich nicht mehr korrigieren, sobald es eine neuere gibt.
 renderVersions(r);
 $('reconnect').hidden=saved || !r.controlUnresponsive;$('control-warning').hidden=!r.controlUnresponsive;
     $('result-scope').textContent='Prüfumfang: '+({consumption:'Nur Bezug (§14a)',production:'Nur Einspeisung',both:'Bezug & Einspeisung'}[r.config?.scope] || 'Nicht angegeben');
  $('result-curve').append($('power-monitor'));const doc=root.ownerDocument,context=doc.getElementById('protocol-context') || doc.querySelector('#saved-inspector .inspector-content') || doc.getElementById('saved-inspector');if(context){context.prepend($('power-technical'));context.prepend($('power-table'));}
  $('result-meta').classList.toggle('practice-result',r.mode==='simulator');$('result-customer').classList.toggle('practice-result',r.mode==='simulator');
  $('outcome').hidden=false;$('outcome').className='notice '+(outcome==='Bestanden'?'success':outcome==='Nicht bestanden'?'error':'warning');
  $('outcome-title').textContent=StromeaResult.outcomeTitle(r,outcome);
  // The sidebar preserves all setup/transport steps; the main table shows required missing evidence.
  const missing=(r.steps || []).filter(item=>item.status==='not_checked' && item.required!==false && !(r.mode==='onsite' && r.config?.scope && r.config?.testProfile!=='full' && !r.config?.longTest && /^(LPC|LPP) Wiederkehr(?: gemessene Wirkung)?$/.test(item.name)) && StromeaResult.isCheckpoint(item));
  // UX-S16: the release statement appears once, as the sentence below the matrix. The banner only recommends an action.
  const releases=(r.steps||[]).filter(item=>item.name==='Grenzen aufheben'),released=releases.length>0&&releases.every(item=>item.passed&&item.status!=='not_checked');
  const failed=r.steps?.find(item=>!item.passed && item.status!=='not_checked' && item.name!=='Grenzen aufheben');
  $('outcome-text').textContent=r.abortReason==='user_cancel'?'Prüfung abgebrochen. Starte sie erneut, wenn du bereit bist.':failed?stepName(failed.name)+' fehlgeschlagen. Einstellungen prüfen und erneut testen.':missing.length?'Offene Nachweise stehen unten mit konkreten Gründen.':!released?'Testgrenzen am Gerät aufheben und Freigabe prüfen.':outcome==='Bestanden'?'Alle gewählten Prüfpunkte bestanden.':'Pflichtnachweise fehlen. Prüfschritte kontrollieren.';
  // 043: the cause block replaces the generic sentence; the old bottom card („Prüfung unterbrochen“) stays hidden.
  $('outcome-text').hidden=cause;if(cause && $('run-error'))$('run-error').hidden=true;
  // UX-S16 (038): releasing the test limits is the last table row, same symbols as the other rows.
  const restores=r.steps?.filter(item=>/zurückstellen/.test(item.name)) || [];
  const expected=Object.keys(r.failsafeOriginals || {}).length*2;
  const restored=restores.length>0 && restores.length>=expected && restores.every(item=>item.passed && item.status!=='not_checked');
  // R042-01: no hint when the test never wrote fallback values (cancelled early, originals unreadable).
  const touched=restores.length>0 || (r.steps || []).some(item=>item.name==='Failsafe-Werte setzen' && item.status!=='not_checked');
  // Neutral line without heading, only when a real device did not confirm the restore.
  $('release-failsafe').hidden=r.mode!=='onsite' || restored || !touched;$('release-failsafe').textContent=$('release-failsafe').hidden?'':'Wiederherstellung der Notbetriebswerte nicht bestätigt. Stelle am Gerät Leistungsgrenze und Mindestdauer für Kommunikationsausfall wieder wie vor der Prüfung ein.';
  $('evidence-list').replaceChildren();
  for(const [reason,items] of StromeaResult.evidenceGroups(missing,evidenceReason))for(const [index,item] of items.entries()){const tr=document.createElement('tr'),title=document.createElement('th'),status=document.createElement('td');title.scope='row';title.textContent=stepName(item.name);status.textContent='Nicht geprüft';tr.append(title,status);if(index===0){const cell=document.createElement('td');cell.rowSpan=items.length;cell.textContent=reason;tr.append(cell);}$('evidence-list').append(tr);}
  const body=$('result-matrix').querySelector('tbody');body.replaceChildren();
  const labels={passed:'Bestanden',open:'Ausstehend',not_checked:'Nicht geprüft',failed:'Fehlgeschlagen',excluded:'Nicht im Prüfumfang'},symbols={passed:'✓',open:'○',not_checked:'–',failed:'×',excluded:'·'};
  for(const row of StromeaResult.formMatrix(r)){
   const tr=document.createElement('tr'),title=document.createElement('th');title.scope='row';title.textContent=row.title;tr.append(title);
   for(const direction of ['lpc','lpp'].filter(d=>d==='lpc'?r.config?.scope!=='production':r.config?.scope!=='consumption')){const cell=document.createElement('td'),status=row[direction];cell.className='matrix-'+status;cell.textContent=symbols[status];cell.title=labels[status];cell.setAttribute('aria-label',labels[status]);tr.append(cell);}body.append(tr);
  }
  // 236: own row for restoring the device's original values (read back), only when the test changed them.
  // R3-04: like the PDF, ✓ needs the failsafe readback and the read-back release of every limit.
  if(r.mode==='onsite' && touched){
   const tr=document.createElement('tr'),title=document.createElement('th');title.scope='row';title.textContent='Originalwerte wiederhergestellt';tr.append(title);
   for(const direction of ['lpc','lpp'].filter(d=>d==='lpc'?r.config?.scope!=='production':r.config?.scope!=='consumption')){const k=direction.toUpperCase(),own=restores.filter(item=>item.name.startsWith(k+' ')),status=!(k in (r.failsafeOriginals||{}))?'excluded':!own.length?'open':own.some(item=>!item.passed&&item.status!=='not_checked')||r.releaseVerified===false||releases.some(item=>!item.passed&&item.status!=='not_checked')?'failed':own.length<2||own.some(item=>item.status==='not_checked')||r.releaseVerified!==true||!released?'not_checked':'passed';const cell=document.createElement('td');cell.className='matrix-'+status;cell.textContent=symbols[status];cell.title=labels[status];cell.setAttribute('aria-label',labels[status]);tr.append(cell);}body.append(tr);}
  {const status=!releases.length?'open':releases.some(item=>!item.passed&&item.status!=='not_checked')?'failed':releases.some(item=>item.status==='not_checked')?'not_checked':'passed';
   const tr=document.createElement('tr'),title=document.createElement('th');title.scope='row';title.textContent='Testgrenzen aufgehoben';tr.append(title);
   for(const direction of ['lpc','lpp'].filter(d=>d==='lpc'?r.config?.scope!=='production':r.config?.scope!=='consumption')){const cell=document.createElement('td');cell.className='matrix-'+status;cell.textContent=symbols[status];cell.title=labels[status];cell.setAttribute('aria-label',labels[status]);tr.append(cell);}body.append(tr);}
  $('result-matrix').querySelectorAll('thead th')[1].hidden=r.config?.scope==='production';$('result-matrix').querySelectorAll('thead th')[2].hidden=r.config?.scope==='consumption';
  $('result-matrix').hidden=false;
  $('open-evidence').hidden=!missing.length;
  $('unpair-reminder').hidden=r.mode!=='onsite';$('failsafe-warning').hidden=true;
  const seconds=Math.max(0,Math.round((Date.parse(r.finished)-Date.parse(r.started))/1000));
  $('result-meta').replaceChildren();for(const [index,value] of [r.mode==='onsite'?'Gerät vor Ort':'Übungsmodus',[r.device?.brand,r.device?.model].filter(Boolean).join(' ') || r.device?.name,r.config?.testProfile==='full' || (!r.config?.testProfile && r.config?.longTest)?'Langzeitprüfung':'Funktionsprüfung',Number.isFinite(seconds)?(seconds<120?StromeaI18n.t('Dauer: {seconds} s').replace('{seconds}',seconds):StromeaI18n.t('Dauer:')+' '+Math.round(seconds/60).toLocaleString(StromeaI18n.locale())+' '+StromeaI18n.t('Min.')):'' /* 041: lange Prüfungen in Minuten */].entries()){if(!value)continue;if($('result-meta').childNodes.length)$('result-meta').append(document.createTextNode(' · '));const span=document.createElement('span');if(index===1)span.dataset.userContent='true';span.textContent=value;$('result-meta').append(span);}
  // 041: eigene Kundenzeile unter der Prüfzeile, nur eingetragene Felder; ohne Angaben „Kunde: nicht eingetragen“.
  renderCustomer(r);
 $('downloads').querySelector('a[download$=".pdf"]').textContent=r.mode==='simulator'?'PDF-Prüfbericht laden (Übungsmodus)':'PDF-Prüfbericht laden';
  renderStorage();$('downloads').hidden=false;$('repeat').hidden=saved;$('other-device').hidden=saved;
}

function renderSteps(steps,current,progress){
 waiting(progress,!!current);
 const signature=JSON.stringify([steps,current,StromeaI18n.language()]);if(signature===renderedSteps)return;renderedSteps=signature;
 const list=$('steps');list.replaceChildren();
 const groupFor=name=>/zurückstellen|Grenzen aufheben/.test(name)?'Abschluss':/Ursprungswerte/.test(name)?'Vorbereitung':name.startsWith('LPC ')?'Bezug':name.startsWith('LPP ')?'Einspeisung':'Vorbereitung';
 const transport=steps.filter(item=>/^(TCP-Verbindung|TLS-Handshake|WebSocket-Upgrade|SHIP-Handshake)/.test(item.name));steps=steps.filter(item=>!transport.includes(item));
 if(transport.length){const li=document.createElement('li'),details=document.createElement('details'),summary=document.createElement('summary'),raw=document.createElement('pre');summary.textContent='Technische Verbindungsdetails';raw.className='raw-technical';raw.textContent=transport.map(item=>item.name+': '+(item.detail||'')).join('\n');details.append(summary,raw);li.append(details);list.append(li);}
 const grouped=new Map(['Vorbereitung','Bezug','Einspeisung','Abschluss'].map(name=>[name,[]]));
 for(const item of steps)grouped.get(groupFor(item.name)).push(item);
 if(current)grouped.get(groupFor(current)).push({name:current,current:true});
 for(const [name,items] of grouped){
  if(!items.length)continue;
  const section=document.createElement('li');section.className='step-group';const heading=document.createElement('h4');heading.textContent=name;section.append(heading);
  const group=document.createElement('ol');section.append(group);list.append(section);
  for(const item of items){
   const li=document.createElement('li'),icon=document.createElement('span'),title=document.createElement('span'),at=document.createElement('time');
   li.dataset.name=item.name;li.className='step-row'+(item.current?' active-step':'');icon.textContent=item.current?'⟳':item.status==='not_checked'?'–':item.passed?'✓':'×';icon.className=item.current?'spinner':item.status==='not_checked'?'subtle':item.passed?'pass':'fail';icon.setAttribute('aria-hidden','true');
   title.textContent=['Bezug','Einspeisung'].includes(name)?stepName(item.name).replace(/^(Bezug|Einspeisung): /,''):stepName(item.name);title.className='step-title';title.title=stepName(item.name)+' · '+(item.current?'In Prüfung':item.status==='not_checked'?'Nicht geprüft':item.passed?(item.category==='workflow'?'Erledigt':'Bestanden'):'Fehler');at.dateTime=item.at || '';at.textContent=item.at?new Date(item.at).toLocaleTimeString(StromeaI18n.locale()):item.current?'…':'–';li.append(icon,title,at);if(item.current){at.classList.add('step-clock');at.hidden=!!progress?.lowLoad;at.textContent=progress?.lowLoad?'':countdown();}
   if(item.detail){const raw=document.createElement('p');raw.className='step-detail raw-technical';raw.textContent=item.detail.includes('nicht geprüft (Langtest)')?StromeaI18n.t('Mindestdauer nicht geprüft: In der Funktionsprüfung wird sie nicht abgewartet'):item.detail.replace(/context canceled/g,StromeaI18n.t('Prüfung abgebrochen')).replace(/context deadline exceeded/g,StromeaI18n.t('Zeitlimit erreicht')).replace(/Wiederkehr/g,StromeaI18n.t('Rückkehr')).replace(/Langtest/g,StromeaI18n.t('Langzeitprüfung'));li.classList.add('detail-visible');li.append(raw);}
   group.append(li);
  }
 }
 if(current){const active=list.querySelector('.active-step');if(active && active.getClientRects().length){const content=active.closest('.inspector-content');if(content && (active.getBoundingClientRect().bottom>content.getBoundingClientRect().bottom || active.getBoundingClientRect().top<content.getBoundingClientRect().top))active.scrollIntoView({block:'nearest'});}}
}

 let waitProgress=null,waitBusy=false;
 function countdown(){const deadline=Date.parse(waitProgress?.deadline);if(!waitBusy || !Number.isFinite(deadline))return '…';const s=Math.max(0,Math.ceil((deadline-Date.now())/1000));return Math.floor(s/60)+':'+String(s%60).padStart(2,'0');}
 function updateWaiting(){
  const low=waitBusy && !!waitProgress?.lowLoad,has=waitBusy && Number.isFinite(Date.parse(waitProgress?.deadline));
  $('wait-clock').hidden=!has || low;$('wait-clock').textContent=has && !low?StromeaI18n.t('Verbleibend: ')+countdown():'';
  const clock=document.querySelector('.active-step .step-clock');if(clock){clock.hidden=low;clock.textContent=low?'':countdown();}
     // 043 (Nutzer 2026-11-14): one decision card; its timer stands in the card header (no second banner).
   $('wait-banner').hidden=true;$('wait-banner').textContent='';const timer=$('low-load-timer');if(timer)timer.textContent=low?countdown():'';
  if(!saved)document.title=low?StromeaI18n.t('Entscheidung erforderlich')+' · '+countdown()+' · Stromea':waitBusy?StromeaI18n.t('Prüfung läuft')+' · '+countdown()+' · Stromea':'Stromea Steuerbox-Simulator';
 }
 function waiting(progress,busy){waitProgress=progress;waitBusy=busy;updateWaiting();}
 const ageTimer=setInterval(()=>{updatePowerAge();updateWaiting();},1000);
 return {waiting,dispose:()=>clearInterval(ageTimer),renderPower,renderResult,resetPower,stepName,evidenceReason,renderSteps,invalidateSteps:()=>{renderedSteps='';resultContentSignature='';},invalidatePower:()=>{powerSignature='';},placeLive:()=>{$('live').insertBefore(monitor,$('low-load'));const context=document.getElementById('protocol-context');if(context){context.prepend($('power-technical'));context.prepend($('power-table'));}}};
 }
};

// Shared local navigation and offline help; assignment data is only rendered as text.
window.StromeaNavigation=async(path=location.pathname)=>{
 if(!['/protokolle','/hilfe','/profil'].includes(path))return;
 document.body.classList.add('local-page-open');document.querySelector('.local-page')?.remove();
 StromeaNavigation.cleanup?.();StromeaNavigation.cleanup=null;
 const section=document.createElement('section');section.className='local-page card';document.querySelector('header').after(section);
 const title=document.createElement('h2');title.textContent=path==='/profil'?'Profil':path==='/hilfe'?'Hilfe':'Protokolle';section.append(title);
 if(path==='/profil'){StromeaProfile.mount(section);return;}
 if(path==='/hilfe'){
  for(const [question,answer] of [
   ['Wie starte ich eine Prüfung?','Wähle ein Gerät, vergleiche die Gerätekennung und gib den Simulator am Gerät frei. Mit „Prüfung vorbereiten“ gehst du weiter.'],
   ['Warum muss das Gerät vor der Prüfung Strom beziehen oder einspeisen?','Gemeint ist die Leistung, die das Gerät gerade tatsächlich aus dem Netz bezieht (z. B. laufende Wärmepumpe, ladendes Auto) oder einspeist (z. B. PV-Wechselrichter). Sie muss vor dem Test über der Testgrenze liegen, sonst lässt sich keine Drosselung messen und es werden nur die Antworten des Geräts geprüft. Die übrigen Watt-Angaben sind Einstellungen der Prüfung: Bezugs- und Einspeisegrenze, Leistung im Notbetrieb und Messtoleranz.'],
   ['Was tun bei fehlender Quittung?','Gerät antwortet nicht auf Steuerbefehle. Verbindung neu aufbauen, Erkennung abwarten und Freigabe sowie Notbetriebs-Werte am Gerät prüfen.'],
   ['Wo sind meine Protokolle?','Alle abgeschlossenen Prüfungen werden lokal gespeichert. Im Menü Protokolle kannst du sie öffnen, Kundendaten ändern und PDFs laden.'],
   ['Beendet das Browserfenster den Simulator?','Nein. Der Simulator läuft im Hintergrund weiter. Beende ihn im Menü mit „Simulator beenden“ oder über das Stromea-Symbol in der Task- oder Menüleiste. Eine laufende Prüfung wird vorher abgebrochen und die Testgrenzen werden aufgehoben.'],
   ['Werden Kundendaten übertragen?','Kunden-/Anlagenangaben bleiben ausschließlich lokal auf diesem Rechner. Sie erscheinen im gespeicherten Protokoll und PDF, nicht in Diagnoseprotokollen.'],
   ['Ist der Prüfbericht ein Zertifikat?','Der Simulator prüft Funktionen. Sein Bericht ist kein Zertifikat und ersetzt weder eine elektrische Erstprüfung noch eine Konformitätsprüfung.']
  ]){const q=document.createElement('h3'),a=document.createElement('p');q.textContent=question;a.textContent=answer;section.append(q,a);}
   // 041 (Nutzer 2026-11-14): Datenübertragung auf einen anderen Computer (ZIP-Export/-Import).
   {const box=document.createElement('section');box.id='transfer-help';const q=document.createElement('h3');q.textContent='Wie übertrage ich alle Daten auf einen anderen Computer?';
    const row=(text,label,id)=>{const p=document.createElement('p');p.className='transfer-row';const span=document.createElement('span');span.textContent=text;const b=document.createElement('button');b.type='button';b.id=id;b.className='secondary';b.textContent=label;p.append(span,b);return p;};
    const status=document.createElement('p');status.id='transfer-status';status.className='copy-status';status.setAttribute('role','status');const say=text=>{status.textContent=text;};
    const note=document.createElement('p');note.className='help';note.textContent='Hinweis: Behandle die Exportdatei vertraulich. Sie enthält personenbezogene Daten und die Simulator-Kennung mit ihrem privaten Schlüssel.';
    const ex=row('Klicke hier, um eine Exportdatei zu erzeugen:','Exportdatei erzeugen','transfer-export'),im=row('Klicke hier, um eine erzeugte Exportdatei zu importieren:','Exportdatei importieren','transfer-import');
    box.append(q,ex,im,note,status);section.append(box);
    ex.querySelector('button').addEventListener('click',()=>window.StromeaTransfer?.exportFile(say));im.querySelector('button').addEventListener('click',()=>window.StromeaTransfer?.chooseImport(say));}
   const technical=document.createElement('section');technical.id='technical-help';const question=document.createElement('h3');question.textContent='Wo finde ich detaillierte technische Informationen zum Simulator?';const answer=document.createElement('p');answer.textContent='Im GitHub-Projekt findest du technische Dokumentation, Quellcode und Hinweise zur Entwicklung.';const link=document.createElement('a');link.href='https://github.com/stromea-de/steuerbox-simulator#readme';link.target='_blank';link.rel='noopener noreferrer';link.className='external-link';link.textContent='Technische Dokumentation auf GitHub';technical.append(question,answer,link);section.append(technical);StromeaI18n.walk(section);return;
 }
 const status=document.createElement('p');status.setAttribute('role','status');status.textContent='Protokolle werden geladen …';section.append(status);
 try{
  const response=await fetch('/api/reports');if(!response.ok)throw Error('list');const entries=await response.json();if(!section.isConnected)return;status.textContent='Hier findest du die auf diesem Computer gespeicherten Prüfungen.';const skipped=Number(response.headers?.get?.('X-Stromea-Skipped-Reports'))||0;if(skipped>0){const note=document.createElement('p');note.className='report-skipped';note.dataset.i18nRendered='true';const skippedText=()=>{note.textContent=StromeaI18n.t('Nicht lesbare Dateien im Protokollordner: {count}. Sie fehlen in dieser Liste.').replace('{count}',skipped);};skippedText();window.addEventListener('stromea-language',skippedText);status.after(note);} // 040 F-02
  section.classList.add('report-page');
  const label=document.createElement('label');label.htmlFor='report-search';label.textContent='Protokolle suchen';
  const search=document.createElement('input');search.type='search';search.id='report-search';search.placeholder='Nach Kunde, Anlagenadresse oder Zählernummer suchen';search.setAttribute('aria-controls','report-rows');
   // 041: rechts neben der Suche „Übungs-Protokolle anzeigen“ (standardmäßig aus).
   const tools=document.createElement('div');tools.className='report-tools';const practiceLabel=document.createElement('label');practiceLabel.className='check report-practice';const practice=document.createElement('input');practice.type='checkbox';practice.id='report-practice';practice.setAttribute('aria-controls','report-rows');const practiceText=document.createElement('span');practiceText.textContent='Übungs-Protokolle anzeigen';practiceLabel.append(practice,practiceText);tools.append(search,practiceLabel);section.append(label,tools);
  const table=document.createElement('table');table.className='report-library';
  const caption=document.createElement('caption');caption.className='sr-only';caption.textContent='Protokolle';table.append(caption);
  const head=document.createElement('thead'),headRow=document.createElement('tr'),columns=['Datum','Kunde','Zähler / Anlage','Gerät','Ergebnis','Aktionen'];head.append(headRow);table.append(head);
  for(const name of columns){const th=document.createElement('th');th.scope='col';th.textContent=name;headRow.append(th);}
  const list=document.createElement('tbody');list.id='report-rows';table.append(list);section.append(table);
  const dateCells=[],linkContexts=[];
   // 041: Datum zweistellig (08.11.2026), Uhrzeit ohne Sekunden, in der Sprache der Oberfläche.
   const dateText=at=>new Date(at).toLocaleString(StromeaI18n.locale(),{day:'2-digit',month:'2-digit',year:'numeric',hour:'2-digit',minute:'2-digit'});
  const rows=[],normalize=value=>String(value || '').normalize('NFKC').toLocaleLowerCase();
  entries.sort((a,b)=>(Date.parse(b.report.revision?.created || b.report.started)||0)-(Date.parse(a.report.revision?.created || a.report.started)||0));
  for(const entry of entries){
   const r=entry.report,site=r.config?.site || {},row=document.createElement('tr');row.className='report-row';row.dataset.id=entry.id;
   const cell=(label,value,user=false)=>{const td=document.createElement('td'),key=document.createElement('span'),text=document.createElement('span');key.className='report-cell-label';key.textContent=label;text.textContent=value || '–';if(user)text.dataset.userContent='true';td.append(key,text);row.append(td);return td;};
   const date=cell('Datum',dateText(r.started),true);date.className='report-date';dateCells.push({node:date.lastElementChild,at:r.started});
   const customer=cell('Kunde',site.customer,true);customer.className='report-customer';
   if(site.address){const address=document.createElement('small');address.dataset.userContent='true';address.textContent=site.address;customer.append(address);}
   const meter=cell('Zähler / Anlage',site.meter,true);meter.className='report-meter';const value=meter.lastElementChild;value.className='report-meter-value';value.title=site.meter || '';value.replaceChildren();const text=site.meter || '–',chars=Array.from(text),at=Math.ceil(chars.length/2);for(const part of chars.length>28?[chars.slice(0,at).join(''),chars.slice(at).join('')]:[text]){const line=document.createElement('span');line.textContent=part;line.dataset.userContent='true';value.append(line);}
   cell('Gerät',[r.device?.brand,r.device?.model].filter(Boolean).join(' ') || r.device?.name,true);
   const state=cell('Ergebnis',entry.outcome+(r.mode==='simulator'?' (Übung)':'')),badge=state.lastElementChild;badge.className='report-status';badge.classList.toggle('report-status-failed',entry.outcome==='Nicht bestanden');badge.classList.toggle('report-status-passed',entry.outcome==='Bestanden');
   /* 041: eine Zeile je Prüfung (neueste Version), kein Link auf frühere Versionen. */
   const actions=cell('Aktionen',''),links=document.createElement('div');actions.lastElementChild.remove();links.className='report-downloads';actions.append(links);
   for(const [name,url] of [['Ansehen','/protokoll'],['PDF laden','/api/report.pdf'],['Messdaten (JSON)','/api/report.json']]){const a=document.createElement('a');a.className='text-link';a.textContent=name;/* 039 (C-04): unique accessible name per row */const context=document.createElement('span');context.className='sr-only';context.dataset.userContent='true';a.append(context);linkContexts.push({node:context,at:r.started,customer:site.customer,outcome:entry.outcome,version:r.revision?.number || 1,id:entry.id});a.href=url+'?id='+encodeURIComponent(entry.id);if(name==='PDF laden'){a.download='EEBUS-Protokoll.pdf';a.addEventListener('click',e=>{e.preventDefault();StromeaResult.downloadPDF(a);});}if(url==='/api/report.json')a.download='EEBUS-Protokoll.json';links.append(a);}
   /* 039 (C-01): delete one report with all its versions after an explicit confirmation. */
   const remove=document.createElement('button');remove.type='button';remove.className='text-button report-delete';remove.textContent='Löschen';const removeContext=document.createElement('span');removeContext.className='sr-only';removeContext.dataset.userContent='true';remove.append(removeContext);linkContexts.push({node:removeContext,at:r.started,customer:site.customer,outcome:entry.outcome,version:r.revision?.number || 1,id:entry.id});links.append(remove);
   remove.addEventListener('click',async()=>{if(!window.confirm(StromeaI18n.t('Protokoll löschen? Alle Versionen dieses Berichts werden von diesem Computer entfernt.')))return;error.hidden=true;remove.disabled=true;
    try{const response=await fetch('/api/reports/delete',{method:'POST',headers:{'Content-Type':'application/json','X-Stromea-Token':document.querySelector('meta[name="session-token"]').content},body:JSON.stringify({id:entry.id})});if(!response.ok)throw Error('delete');const {removed}=await response.json();
     for(const item of rows.filter(item=>removed.includes(item.row.dataset.id)))item.row.remove();rows.splice(0,rows.length,...rows.filter(item=>item.row.isConnected));if(!rows.length)emptyCell.textContent=StromeaI18n.t('Noch keine gespeicherten Protokolle. Starte eine Prüfung, dann erscheint ihr Bericht hier.');search.dispatchEvent(new Event('input')); /* R039-07: re-apply the search filter and its empty row */status.textContent=StromeaI18n.t('Protokoll gelöscht.');}
    catch{remove.disabled=false;error.textContent=StromeaI18n.t('Protokoll konnte nicht gelöscht werden.');error.hidden=false;}});
   const error=document.createElement('p');error.className='download-error error';error.hidden=true;error.setAttribute('role','alert');links.append(error);list.append(row);
   rows.push({row,practice:r.mode==='simulator',text:normalize([site.customer,site.address,site.meter].filter(Boolean).join(' '))});
  }
  const localizeDates=()=>{for(const item of dateCells)item.node.textContent=dateText(item.at);const contextText=item=>[dateText(item.at),item.customer,StromeaI18n.t(item.outcome)].filter(Boolean).join(' · '),seen=new Map();for(const item of linkContexts){const k=item.node.parentElement.firstChild.textContent+contextText(item);seen.set(k,(seen.get(k)||0)+1);}for(const item of linkContexts){const text=contextText(item),k=item.node.parentElement.firstChild.textContent+text;item.node.textContent=' – '+text+(seen.get(k)>1?' · ID '+item.id:'');} /* R039-05: report ID only where everything else is equal */};localizeDates();window.addEventListener('stromea-language',localizeDates);StromeaNavigation.cleanup=()=>window.removeEventListener('stromea-language',localizeDates);
  const empty=document.createElement('tr'),emptyCell=document.createElement('td');empty.id='report-empty';empty.className='report-empty';emptyCell.colSpan=6;emptyCell.setAttribute('role','status');emptyCell.textContent=entries.length?'Keine passenden Protokolle. Ändere deine Suche.':'Noch keine gespeicherten Protokolle. Starte eine Prüfung, dann erscheint ihr Bericht hier.';empty.append(emptyCell);empty.hidden=entries.length>0;list.append(empty);
  const filter=()=>{const terms=normalize(search.value).trim().split(/\s+/).filter(Boolean);let found=0,practiceOnly=0;for(const item of rows){const match=terms.every(term=>item.text.includes(term));item.row.hidden=!match || item.practice && !practice.checked;if(!item.row.hidden)found++;else if(match && item.practice)practiceOnly++;}
    if(!rows.length){empty.hidden=false;return;}empty.hidden=found>0;if(!found)emptyCell.textContent=StromeaI18n.t(practiceOnly && !practice.checked?'Nur Übungs-Protokolle gefunden. Aktiviere „Übungs-Protokolle anzeigen“.':'Keine passenden Protokolle. Ändere deine Suche.');};
   search.addEventListener('input',filter);practice.addEventListener('change',filter);filter();
 }catch{status.textContent='Protokolle konnten nicht geladen werden. Erneut versuchen.';}
 StromeaI18n.walk(section);
};
