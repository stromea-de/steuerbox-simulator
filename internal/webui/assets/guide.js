'use strict';
// Offline field guidance. Formulas: research 018 Q1, BK6-22-300 Annex 1,
// 4.5.1/4.5.2, 27 Nov 2023; checked 30 Oct 2026. No runtime source fetches.
(function(root){
 const g={
 minimum({mode,n=1,power=0,wp=0,ac=0}){
  // 039 (B-01): only the inputs of the chosen control type count; a hidden field never blocks the result.
  const used=mode==='direct'?[power]:[wp,ac];
  if(![...used,n].every(Number.isFinite) || used.some(v=>v<0 || v>1000000))return null;
  if(mode==='direct')return Math.round((power>11000?.4*power:4200)*1000)/1000;
  if(mode!=='ems' || !Number.isInteger(n) || n<1 || n>1000)return null;
  const gzf=n===1?0:[0,0,.8,.75,.7,.65,.6,.55,.5][n] ?? .45;
  const base=wp>11000 || ac>11000?Math.max(wp>11000?.4*wp:0,ac>11000?.4*ac:0):4200;
  return Math.round((base+(n-1)*gzf*4200)*1000)/1000;
 },
 // Own test suggestion, ADR 027: half of a fresh, negative MPC total reading.
 // Round down to milliwatts, leave headroom for variation. No EEG default.
 production(watts){return typeof watts==='number' && Number.isFinite(watts) && watts<0?Math.floor(-watts*500)/1000:null;},
 budget({scope,effect,outage,step,cleanup,minimum,profile,measurable=true}){return (scope==='both'?2:1)*((measurable?3*effect:0)+outage+30+10*step+3*cleanup+(profile==='full' && measurable?minimum:0));},
 duration(seconds,locale='de-DE'){
  const minutes=Math.max(1,Math.ceil(seconds/60)),number=n=>n.toLocaleString(locale);
  return minutes<90?number(minutes)+' Min.':number(Math.floor(minutes/60))+' h'+(minutes%60?' '+number(minutes%60)+' min':'');
 },
 mount(container,{apply,refresh,getData}){
  const $=id=>document.getElementById(id);
  const panel=document.createElement('section');panel.id='guide-panel';
  panel.innerHTML=`<h3 id="guide-title">Hilfe zu den Eingaben</h3><p id="guide-copy"></p><p id="guide-extra" class="help"></p>
<section id="minimum-calculator" hidden><h4>Berechnungshilfe nach § 14a EnWG</h4>
<label for="calc-mode">Steuerungsart</label><select id="calc-mode"><option value="direct">Direktsteuerung: Wärmepumpe / Klimaanlage</option><option value="ems">Energiemanagementsystem</option></select>
<div id="calc-direct"><label for="calc-power">Netzanschlussleistung WP / Klima (W)</label><input id="calc-power" type="text" inputmode="decimal" ></div>
<div id="calc-ems" hidden><label for="calc-n">Anzahl gesteuerter Einrichtungen</label><input id="calc-n" type="number" min="1" max="1000" value="2"><label for="calc-wp">Summe Netzanschlussleistung WP-Gruppe (W)</label><input id="calc-wp" type="text" inputmode="decimal" placeholder="0"><label for="calc-ac">Summe Netzanschlussleistung Klima-Gruppe (W)</label><input id="calc-ac" type="text" inputmode="decimal" placeholder="0"><p class="help">Nur passende Gruppen über 11.000 W skalieren. Gruppendefinition und Netzbetreiber-Sollwert prüfen.</p></div>
<p id="calc-formula" class="help"></p><p id="calc-result" role="status"></p><button id="calc-apply" class="secondary" type="button">Bezugsgrenze übernehmen</button></section>
<section id="production-suggestion" hidden><p id="production-copy" role="status"></p><button id="production-apply" class="secondary" type="button" disabled>Einspeisevorschlag übernehmen</button></section>
<section id="original-values" hidden><h4>Ausgelesene Geräteeinstellungen</h4><dl id="original-list"></dl></section>
<button id="guide-refresh" class="text-button" type="button" hidden>Werte vom Gerät neu abfragen</button><p id="guide-status" role="status"></p><dl id="preparation-values"></dl>
<section id="guide-basis" hidden><h4 id="guide-basis-title"></h4><p id="guide-basis-copy"></p><ul id="guide-sources" class="guide-sources"></ul></section>`;
  container.append(panel);
  let reading=false,readDeadline=0;
 let active='',calculated=null,suggested=null,appliedProduction=null,manualProduction=false;
  function prefillProduction(){
   if(manualProduction || getData().busy)return;const sample=getData().preparation?.sample;const age=Date.now()-Date.parse(sample?.at);const proposal=age>=0 && age<=30000?g.production(sample.watts):null;
   const valid=proposal!==null && proposal>0;const value=valid?proposal:1000;
   $('lpp').value=value.toLocaleString(StromeaI18n.locale(),{maximumFractionDigits:3,useGrouping:true}); /* 041: Tausendertrennzeichen wie im Feld */appliedProduction=valid?$('lpp').value:null;productionHint();
  }
  function productionHint(){
   const hint=$('lpp-hint');hint.hidden=false;
   hint.textContent=manualProduction?'Eigene Einspeisegrenze':appliedProduction?'Vorschlag aus Messung übernommen':'Für den Test muss die Einspeisung über dieser Grenze liegen.';
  }
  $('lpp').addEventListener('input',()=>{manualProduction=true;appliedProduction=null;productionHint();});

  // 041 (Nutzer 2026-11-14): Quellen direkt kleingedruckt, jede mit Link in neuem Tab (statt „Vorgaben nachlesen“).
  const sourceURL={
   law:'https://www.bundesnetzagentur.de/DE/Beschlusskammern/1_GZ/BK6-GZ/2022/BK6-22-300/Beschluss/BK6-22-300_Beschluss_Anlage1.pdf?__blob=publicationFile&v=1',
   eeg:'https://www.gesetze-im-internet.de/eeg_2014/__9.html',
   lpc:'https://www.eebus.org/wp-content/uploads/2023/04/EEBus_UC_TS_LimitationOfPowerConsumption_V1.0.0_public.pdf'
  };
  const source={
   law:'BNetzA · BK6-22-300 Anlage 1 · Nr. 2.4, 4.4–4.5.2 · 27.11.2023 · Recherche 30.10.2026',
   eeg:'EEG 2023 · §9 und §100 · Onlinefassung geprüft 30.10.2026 ',
   lpc:'EEBUS · LPC Technical Specification 1.0.0 · 14.10.2022 · §2.6.2.1, LPC-022/1–5 · geprüft 31.10.2026',
   own:''
  };
  const guides={
   scope:['Prüfumfang','Wähle die Leistungsrichtungen passend zur Anlage und zu den gemeldeten Funktionen. Ladepunkt oder Wärmepumpe: Bezug (LPC). PV-Wechselrichter: Einspeisung (LPP). Ein Speicher kann beide Richtungen unterstützen.',[]],
   lpc:['Bezugsgrenze §14a','Bei Direktsteuerung gelten meist 4.200 W. Für Wärmepumpen und Klimaanlagen über 11.000 W sowie für ein Energiemanagementsystem (EMS): Mindestleistung mit dem Rechner bestimmen. Die gemessene Leistung muss vor dem Test über der Grenze liegen.',['law']],
   lpp:['Einspeisegrenze','Wähle einen Testwert unter der aktuellen Einspeisung. Nur dann lässt sich die Drosselung nachweisen. ',['eeg','own']],
   failsafe:['Notbetrieb','Bleiben die Lebenszeichen (Heartbeat) aus, begrenzt das Gerät seine Leistung nach der Ausfallfrist selbst auf höchstens diesen Wert. Bei LPC beträgt die Ausfallfrist 120 s. Die am Gerät eingestellte Mindestdauer dieses Notbetriebs liegt bei LPC zwischen 2 und 24 Stunden. Die Steuerbox kann versuchen, den Notbetrieb per Befehl vorzeitig zu beenden.',['lpc','own']],
   tolerance:['Messtoleranz','Die gemessene Bezugs- oder Einspeiseleistung darf die Testgrenze um höchstens diesen Wert überschreiten. Wähle eine Toleranz passend zur Messgenauigkeit und zu Lastschwankungen.',[]],
   'failsafe-minimum':['Mindestdauer des Notbetriebs','EEBUS LPC 1.0.0 (14.10.2022), LPC-022/3: 2–24 Std. Kürzere Werte sind für LPC nicht vorgesehen. Die Funktionsprüfung prüft den Eintritt und die vorzeitige Freigabe. Nur die Langzeitprüfung beobachtet den Ablauf dieser Dauer.',['lpc','own']],
   'failsafe-timeout':['Ausfall der Lebenszeichen','Der Simulator stoppt die Lebenszeichen. Er beobachtet 120 s plus 30 s Reserve. Eine spätere positive Quittung verlängert die Frist. Bewertet wird die gemessene Reaktion im Notbetrieb.',['lpc','own']],
   profile:['Prüfart','Die Funktionsprüfung prüft den Eintritt in den Notbetrieb und versucht, ihn auf Befehl der Steuerbox vorzeitig zu beenden. Die Mindestdauer wird nicht abgewartet. Die Langzeitprüfung beobachtet zusätzlich deren Ablauf.',['lpc','own']]
  };
  function recalculate(){
   const num=id=>{try{return StromeaInput.number($(id).value || '0',0);}catch{return NaN;}};
   const ems=$('calc-mode').value==='ems';$('calc-direct').hidden=ems;$('calc-ems').hidden=!ems;
   calculated=g.minimum({mode:$('calc-mode').value,n:Number($('calc-n').value),power:num('calc-power'),wp:num('calc-wp'),ac:num('calc-ac')});
   if(!ems && (!String($('calc-power').value).trim() || num('calc-power')<=0))calculated=null;
   $('calc-formula').textContent=StromeaI18n.t(ems?'Pmin = Basis + (n − 1) × GZF × 4200 W. Basis: 4200 W oder max(0,4 × WP; 0,4 × Klima). GZF: n = 2/3/4/5/6/7/8/≥9 → 0,8/0,75/0,7/0,65/0,6/0,55/0,5/0,45.':'Bis 11.000 W: 4200 W; über 11.000 W: 0,4 × Netzanschlussleistung.');
   const visible=(ems?['calc-n','calc-wp','calc-ac']:['calc-power']),bad=visible.filter(id=>id==='calc-n'?!(Number.isInteger(Number($(id).value)) && +$(id).value>=1 && +$(id).value<=1000):!Number.isFinite(num(id)) || num(id)>1000000);
   for(const id of ['calc-power','calc-n','calc-wp','calc-ac']){if(bad.includes(id))$(id).setAttribute('aria-invalid','true');else $(id).removeAttribute('aria-invalid');}
   $('calc-result').textContent=calculated===null?(!ems && !$('calc-power').value.trim()?'Trage die Anschlussleistung ein.':bad.length?StromeaI18n.t('Eingabe prüfen: {field}').replace('{field}',StromeaI18n.t(StromeaI18n.sourceText(document.querySelector('label[for="'+bad[0]+'"]')))):'Eingaben prüfen.'):'Berechnete Mindestleistung: '+calculated.toLocaleString(StromeaI18n.locale(),{maximumFractionDigits:3})+' W';$('calc-apply').disabled=calculated===null || getData().busy;
  }
  function render(id=active){
   prefillProduction();
   if(active!==id)$('guide-status').textContent='';active=id;panel.hidden=false;$('field-help').hidden=!!guides[id];$('context-copy').hidden=!!guides[id];
   const [title,copy,refs]=guides[id] || ['','',[]];$('guide-title').textContent=title;$('guide-title').hidden=!guides[id];$('guide-copy').textContent=copy;$('guide-copy').hidden=!copy;$('guide-extra').hidden=true;$('guide-extra').textContent=['lpc','lpp','failsafe'].includes(id)?StromeaI18n.t(id+'_info'):'';$('guide-sources').replaceChildren(...refs.filter(key=>key!=='own').map(key=>{const li=document.createElement('li'),a=document.createElement('a');a.href=sourceURL[key];a.target='_blank';a.rel='noopener noreferrer';a.className='external-link';a.textContent=StromeaI18n.t(source[key]).trim();const hint=document.createElement('span');hint.className='sr-only';hint.textContent=' '+StromeaI18n.t('(öffnet in neuem Tab)');a.append(hint);li.append(a);return li;}));
   const basis={
    lpc:['Woher kommt die Bezugsgrenze?','4.200 W entsprechen der Mindestleistung bei Direktsteuerung nach BNetzA BK6-22-300, Anlage 1, Ziffer 4.5.1. Für Wärmepumpen und Klimaanlagen über 11.000 W sowie EMS die Mindestleistung mit dem Rechner bestimmen.'],
    lpp:['Warum 1.000 W?','1.000 W sind ein von Stromea gewählter Prüfwert, frei änderbar. Für den Betrieb gilt die Vorgabe des Netzbetreibers. Hier geht es nur um den Test.'],
    failsafe:['Warum 500 W?','500 W sind ein von Stromea gewählter Prüfwert, frei änderbar. Bei ausbleibenden Lebenszeichen (Heartbeat) soll das Gerät höchstens diese Leistung beziehen oder einspeisen.'],
    tolerance:['Warum 50 W?','50 W sind eine von Stromea gewählte Messtoleranz, frei änderbar. Die passende Toleranz hängt von der Messgenauigkeit des Geräts und vom Lastverlauf ab.']
   }[id];$('guide-basis').hidden=!basis;$('guide-basis-title').textContent=basis?.[0] || '';$('guide-basis-copy').textContent=basis?.[1] || '';
   $('guide-sources').hidden=!refs.some(key=>key!=='own');
   $('minimum-calculator').hidden=id!=='lpc';$('production-suggestion').hidden=id!=='lpp';$('original-values').hidden=!getData().real || !['failsafe','failsafe-minimum'].includes(id);
   productionHint();
   const data=getData(),prep=data.preparation || {},sample=prep.sample;
   suggested=sample && Date.now()-Date.parse(sample.at)>=0 && Date.now()-Date.parse(sample.at)<=30000?g.production(sample.watts):null;
   $('production-copy').textContent=suggested===null?'Noch kein Messwert. Trage die Einspeisegrenze selbst ein.':'Gemessene Einspeisung: '+Math.abs(sample.watts).toLocaleString(StromeaI18n.locale())+' W · '+new Date(sample.at).toLocaleTimeString(StromeaI18n.locale())+' · Vorschlag: '+suggested.toLocaleString(StromeaI18n.locale())+' W';
   $('production-apply').hidden=suggested===null;
   $('production-apply').disabled=suggested===null || data.busy;
   $('original-list').replaceChildren();
   for(const kind of ['LPC','LPP']) {const v=prep.originals?.[kind];const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=StromeaI18n.t(kind==='LPC'?'Bezug':'Einspeisung');dd.textContent=v?'Leistung: '+v.power.toLocaleString(StromeaI18n.locale())+' W · Mindestdauer: '+(v.minimum/3.6e12).toLocaleString(StromeaI18n.locale())+' h':'nicht vom Gerät gelesen';$('original-list').append(dt,dd);}
   $('guide-refresh').hidden=!data.real;$('guide-refresh').disabled=data.busy || reading;
   $('guide-refresh').setAttribute('aria-busy',String(reading));
   if(reading){$('guide-status').textContent=StromeaI18n.t('Gerätewerte werden gelesen …')+' '+StromeaI18n.t('Zeitlimit:')+' '+Math.max(0,Math.ceil((readDeadline-Date.now())/1000))+' s';}
   $('preparation-values').replaceChildren();
   for(const item of prep.values || []){const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=item.key==='MPC'?StromeaI18n.t('Leistung'):(item.key.startsWith('LPC')?StromeaI18n.t('Bezug'):StromeaI18n.t('Einspeisung'))+' · '+StromeaI18n.t(item.key.endsWith('.minimum')?'Mindestdauer':'Notbetriebs-Leistung');dd.textContent=item.reason?StromeaI18n.t(item.reason):StromeaI18n.t('Gelesen')+': '+Number(item.value).toLocaleString(StromeaI18n.locale())+(item.key.endsWith('.minimum')?' s':' W');$('preparation-values').append(dt,dd);}

   recalculate();
  }
  for(const id of ['calc-mode','calc-power','calc-n','calc-wp','calc-ac'])$(id).addEventListener('input',recalculate);
  $('calc-apply').addEventListener('click',()=>{if(calculated!==null)apply('lpc',calculated);});
  $('production-apply').addEventListener('click',()=>{render();if(suggested!==null){apply('lpp',suggested);appliedProduction=$('lpp').value.trim();manualProduction=false;productionHint();}});
  $('guide-refresh').addEventListener('click',async()=>{
   if(reading)return;reading=true;readDeadline=Date.now()+15000;$('guide-status').classList.add('loading');render();
   const ticker=setInterval(()=>render(),250);
   try{await refresh();$('guide-status').textContent='Gerätewerte gelesen.';}catch(e){$('guide-status').textContent='Gerätewerte nicht lesbar.';}
   finally{reading=false;clearInterval(ticker);$('guide-status').classList.remove('loading');render();}
  });
  window.addEventListener('stromea-language',()=>render());
  setInterval(()=>{if(active==='lpp' && !panel.hidden)render();},1000);
  render();return {show:render,update:()=>render(),resetProduction:()=>{manualProduction=false;prefillProduction();}};
 }
 };
 if(typeof module!=='undefined' && module.exports)module.exports=g;else root.StromeaGuide=g;
})(typeof window==='undefined'?globalThis:window);
