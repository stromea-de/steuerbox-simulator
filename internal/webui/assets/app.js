'use strict';
const $ = id => document.getElementById(id);
const token = document.querySelector('meta[name="session-token"]').content;
let real = true, paired = false, connected = false, busy = false, pending = false;
let step = 1, localSki = '', devices = [], selected = null, state = null, completed = 0;
let capabilityState=null,deviceInfo=null,fromQR=false;
let stopDictation=()=>{};
let autoPair=false,autoAdvanceTimer=0,searching=false,searchSignature="",searchGeneration=0;
let connectionPending=false,pairReady=false,pairReadyTimer=0;
let refreshPromise=null,selectionOverview=false;
let pairingView=false,scopeTouched=false;
// Session-local presentation state; trust and transport remain backend-owned.
const deviceErrors=new Map(),savedDevices=new Set();
let deviceListSignature='';
const deviceKey=d=>(d?.ski || '').replace(/[\s:-]/g,'').toLowerCase()+'|'+(d?.address || '');
const currentDevice=()=>selected || {ski:$('ski').value,address:$('address').value};
let operationEpoch=0,cancelInProgress=false,startRequest=null,requestAbort=null,requestKind='';
const canCancel=()=>busy || searching || connectionPending || !!startRequest || !!requestAbort && !requestAbort.signal.aborted;
let refreshing = false, retryAction = null, lastError = '', renderedSteps = '', quitting = false;
window.addEventListener('pagehide',()=>{try{sessionStorage.setItem('stromea-device-choice-036',JSON.stringify({real,device:currentDevice(),devices,pairingView}));}catch{}});
const timeFields = [['step-timeout', 'stepTimeout'], ['effect-timeout', 'effectTimeout'], ['failsafe-timeout', 'failsafeTimeout'], ['failsafe-minimum', 'failsafeMinimum'], ['recovery-timeout', 'recoveryTimeout'], ['cleanup-timeout', 'cleanupTimeout']];
// Motion is presentation only: state, controls and focus always update synchronously.
const motionQuery = matchMedia('(prefers-reduced-motion: reduce)');
const motionEffects = new Map();
const resultView=StromeaResult.create($('step-3'),{isReal:()=>real,motion,motionQuery,svgNode,drawnCheck,motionEffects,enter,actions:{reconnect:()=>reconnectFromResult(),repeat:()=>$('repeat').click()}});
const {renderPower,renderResult,resetPower,stepName,evidenceReason,renderSteps}=resultView;
function motion(el, frames, duration = 300, delay = 0) {
  motionEffects.get(el)?.cancel();
  if (motionQuery.matches || document.hidden || !el.animate) return;
  const effect = el.animate(frames, {duration, delay, fill:'backwards', easing: 'cubic-bezier(.2,.75,.25,1)'});
  motionEffects.set(el, effect);
  const clear = () => { if (motionEffects.get(el) === effect) motionEffects.delete(el); };
  effect.onfinish = clear; effect.oncancel = clear;
}
function enter(el, direction = 1, delay = 0) {
  motion(el, [{opacity: 0, transform: 'translateY(' + (direction * 12) + 'px)'}, {opacity: 1, transform: 'translateY(0)'}], 300, delay);
}
function applyMotionPreference() {
  document.documentElement.classList.toggle('reduce-motion', motionQuery.matches);
  if (!motionQuery.matches) return;
  for (const effect of motionEffects.values()) effect.cancel();
  motionEffects.clear();
  if(connected){clearTimeout(pairReadyTimer);pairReady=true;controls();}
  document.querySelectorAll('.confetti-piece').forEach(node => node.remove());
}
motionQuery.addEventListener('change', applyMotionPreference);
function svgNode(name, attrs = {}) {
  const node = document.createElementNS('http://www.w3.org/2000/svg', name);
  for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, value);
  return node;
}
function drawnCheck() {
  const svg = svgNode('svg', {viewBox: '0 0 32 32', class: 'draw-check', 'aria-hidden': 'true'});
  // Each straight stroke grows along its own axis; no animated paint properties.
  for (const [transform, length, name] of [['translate(7 16) rotate(45)', 8, 'short'], ['translate(12.65 21.65) rotate(-45)', 17, 'long']]) {
    const group = svgNode('g', {transform});
    group.append(svgNode('path', {d: 'M0 0H' + length, class: 'check-' + name})); svg.append(group);
  }
  return svg;
}
function setConnection(phase) {
  const box = $('connection-motion');
  if (box.dataset.phase === phase) return;
  box.dataset.phase = phase; box.hidden = phase !== 'connecting';
  clearTimeout(pairReadyTimer);pairReady=false;
  if(phase==='connected')pairReady=true;
  $('connection-label').textContent = {connecting:'Verbindung wird aufgebaut', connected:'Verbindung bestätigt', failed:'Verbindung nicht möglich', idle:''}[phase];
  if (phase === 'failed') motion(box, [{transform:'translateX(0)'},{transform:'translateX(-5px)'},{transform:'translateX(4px)'},{transform:'translateX(-2px)'},{transform:'translateX(0)'}], 280);
}
function copyFeedback() {
  const box = $('copy-status'); box.classList.add('copy-feedback'); enter(box);
}
function setupMotion() {
  document.documentElement.classList.add('motion-ready');
  for (const card of document.querySelectorAll('.card')) {
    const surface = document.createElement('span'); surface.className = 'card-surface'; surface.setAttribute('aria-hidden','true'); card.prepend(surface);
  }
  const search = document.createElement('div'); search.id = 'search-motion'; search.className = 'search-motion'; search.hidden = true;
  search.setAttribute('role','status');
  const radar = document.createElement('span'); radar.className = 'radar'; radar.setAttribute('aria-hidden','true');
  for (let i = 0; i < 3; i++) radar.append(document.createElement('i'));
  search.append(radar, document.createTextNode('Suche läuft…'));
  // 043 (Nutzer 2026-11-14, Bild 7): „Suche läuft…“ hat eine eigene, immer reservierte Zeile unter den Aktionen;
  // kein Knopf rückt in eine neue Zeile.
  search.classList.add('search-row');document.querySelector('#step-1 .pair-methods').after(search);
  const connection = document.createElement('div'); connection.id = 'connection-motion'; connection.className = 'operation-spinner'; connection.hidden = true;
  connection.setAttribute('role','status');
  const label = document.createElement('span'); label.id = 'connection-label';
  // 043: the remaining time stands inline behind the green status („Verbindung wird aufgebaut (0:54)“).
  const timer = document.createElement('span'); timer.id = 'connection-timer'; timer.className = 'inline-timer'; timer.setAttribute('aria-hidden','true');
  connection.append(label, timer); $('reconnect-progress').before(connection);
  // 043: connecting, the success confirmation and the action share one slot where the button was (no layout jump).
  const slot=document.querySelector('#step-1 .pair-primary');slot.append($('connection-anchor'));
  const success=document.createElement('p');success.id='pair-success';success.className='pair-success';success.setAttribute('role','status');success.hidden=true;
  success.append(drawnCheck(),Object.assign(document.createElement('span'),{textContent:'Gekoppelt. Weiter zur Prüfung …'}));slot.append(success);
  $('paired-status').replaceChildren(drawnCheck(),document.createTextNode($('paired-status').textContent.replace(/^✓\s*/,'')));
  const progress = document.createElement('div'); progress.className = 'motion-progress'; progress.setAttribute('aria-hidden','true');
  const fill = document.createElement('span'); fill.id = 'progress-fill'; fill.style.transform = 'scaleX(0)'; progress.append(fill); $('progress').after(progress);
  const video = $('qr-video'), scanner = document.createElement('div'), frame = document.createElement('span');
  scanner.className = 'qr-scanner'; video.before(scanner); scanner.append(video,frame); frame.id = 'qr-frame'; frame.hidden = true; frame.setAttribute('aria-hidden','true');
  new MutationObserver(() => { frame.hidden = video.hidden; }).observe(video, {attributes:true,attributeFilter:['hidden']});
  // Native details semantics, keyboard activation and immediate state remain intact.
  document.addEventListener('click', event => {
    const summary = event.target.closest('summary');
    if (!summary || event.defaultPrevented || event.button !== 0 || event.target.closest('a,button,.tip')) return;
    const details = summary.parentElement;
    if (details.id === 'prototype-menu') return;
    event.preventDefault();
    const moving = [...document.querySelectorAll('.card:not([hidden]) > :not(.card-surface), main > #about, main > footer')];
    const tops = moving.map(node => node.getBoundingClientRect().top);
    details.open = !details.open;
    moving.forEach((node,i) => {
      const delta = tops[i]-node.getBoundingClientRect().top;
      if (delta && node.getClientRects().length) motion(node,[{transform:'translateY('+delta+'px)'},{transform:'translateY(0)'}]);
    });
    if (details.open) for (const child of details.children) if (child !== summary) enter(child);
  });
  applyMotionPreference();
}
async function api(path, body) {
  const abortable=['/api/discover-lan','/api/prepare'].includes(path);
  const controller=abortable?new AbortController():null;
  if(controller){requestAbort=controller;requestKind=path;controls();}
  const request = fetch(path, {signal:controller?.signal,method: body === undefined ? 'GET' : 'POST', headers: body === undefined ? {} : {'Content-Type': 'application/json', 'X-Stromea-Token': token}, body: body === undefined ? undefined : JSON.stringify(body)});
  const starting=['/api/select','/api/connect','/api/reconnect','/api/run'].includes(path);
  if(starting)startRequest=request;
  let response;
  try {response=await request;} finally {if(startRequest===request)startRequest=null;if(requestAbort===controller){requestAbort=null;requestKind='';}}
  if (!response.ok) throw new Error((await response.text()).trim());
  return response.status === 202 ? null : response.json();
}
function formatSki(value) { return (value || '').replace(/[\s:-]/g, '').toUpperCase().match(/.{1,4}/g)?.join(' ') || ''; }
function controls() {
  const overview=selectionOverview && !!state?.connected && backendSelectionMatches(state);
  $('step-1').classList.toggle('selection-overview',overview);
  renderDeviceOverview();
  $('connected-choice').hidden=true;
  $('connected-name').textContent=[state?.device?.name,state?.device?.address].filter(Boolean).join(' · ');
  $('connected-continue').disabled=pending;

  const locked = busy || pending || searching;
  for (const el of document.querySelectorAll('.workspace input, .workspace select, .workspace textarea')) el.disabled = locked;
  for (const [id] of timeFields) $(id).disabled = locked || !real;
  $('pair-simulator').disabled = locked;
 for (const id of ['qr-apply','qr-camera','qr-image','dictate','reset-identity']) if($(id))$(id).disabled = locked;
  const browsing=step===1 && !pending && !cancelInProgress && (!busy || selectionOverview);
 if(browsing){$('interface').disabled=searching;for(const el of $('manual-device').querySelectorAll('input,textarea,select'))el.disabled=searching;}
 $('discover').disabled = cancelInProgress || (!searching && ((locked && !browsing) || !$('interface').value));
 // 039 (Nutzer 7, QA-05): ohne Netzwerk keine Kopplungsaktion; der Übungsmodus bleibt verfügbar.
 const offline=offlineNow();
 $('network-warning').hidden=!offline;
 for(const id of ['qr-toggle','manual-toggle'])$(id).disabled=offline || cancelInProgress;
 if(offline){for(const id of ['manual-connect','qr-apply','qr-camera','qr-image','qr-image-select','qr-paste'])$(id).disabled=true;if(!$('qr-panel').hidden || !$('manual-device').hidden)workspace.closeQR();}
  // 043: translated directly, so the width never flashes with the German source text.
  const discoverLabel=StromeaI18n.t(searching?'Suche stoppen':'Im Netzwerk suchen');if($('discover').textContent!==discoverLabel)$('discover').textContent=discoverLabel;sizeDiscover();
  $('discover').setAttribute('aria-busy',String(searching));
  $('search-motion').hidden=!searching;
  $('lpp').disabled=locked || ['consumption',''].includes($('scope').value);$('lpc').disabled=locked || ['production',''].includes($('scope').value);
  for(const direction of ['lpc','lpp'])$('scope-'+direction).disabled=locked || !capabilityState?.[direction];
  for(const radio of document.querySelectorAll('input[name="device-choice"]')){
   radio.disabled=pending || cancelInProgress || (busy && state?.kind!=='connect' && !selectionOverview);
   radio.closest('label').hidden=false;
  }
  $('device-choices').hidden=!devices.length;
  $('pair').disabled = busy || pending || cancelInProgress || !skiComplete() || !$('address').value.trim();
  const connecting = busy && state?.kind === 'connect';
  if(paired && connected && !pairReady)$('pair').disabled=true;
  if(offline)$('pair').disabled=true;
  $('pair').textContent = state?.controlUnresponsive && !locked ? 'Verbindung neu aufbauen' : connecting || (pending && connectionPending) ? 'Verbinde …' : paired && connected ? (pairReady?'Prüfung vorbereiten →':'Verbindung bestätigt') : state?.paired && backendSelectionMatches(state) ? 'Neu verbinden' : 'Verbinden';
  $('pair').hidden=connecting || connectionPending || !skiComplete() || (!$('pair-error').hidden && !$('retry').hidden) || !$('pair-success').hidden;
  $('pair').setAttribute('aria-busy', String(connecting || (pending && connectionPending)));
  $('qr-paste').disabled=locked || offline;$('qr-image-select').disabled=locked || offline;
  // 043: an empty scope or a missing inspector name is explained on click (field link, inspector dialog); a lost
  // connection turns the primary action into „Neu verbinden“ (renderStartGate).
  $('run').disabled = locked || !!state?.controlUnresponsive || !paired || (real && (!connected || !capabilityState || (!capabilityState.lpc && !capabilityState.lpp)));
  if(overview){$('pair').textContent=busy?'Weiter':'Prüfung vorbereiten →';$('pair').disabled=pending;}
 $('language').disabled=false;
  $('change-device').disabled=cancelInProgress || (locked && !canCancel());
  workspace.update({adapter:pairingView && skiComplete()?adapterLabel(currentDevice().interface || (currentDevice().manual?$('manual-interface').value:'')):null,localSki,connectionState:state?.connectionState || (connected?'connected':'disconnected'),connectionAt:state?.connectionAt,address:$('address').value,name:deviceTitle(pairingDevice())});
  $('discover').classList.toggle('secondary',searching || overview || !$('pair').hidden || !$('pair-error').hidden);
  $('back').disabled = locked;
  $('retry').disabled = locked || offline;
  $('repeat').disabled = locked;
  $('cancel').disabled = !busy || pending;
 $('other-device').disabled=locked;
 for(const id of ['remeasure','continue-low']) $(id).disabled=pending || !state?.progress?.lowLoad;
  const reading=state?.kind==='prepare' && busy || requestKind==='/api/prepare';
  const connectingNow=busy && state?.kind==='connect' || connectionPending;
  const reconnectingRun=busy && state?.kind==='run' && !!state?.reconnection?.active;
  const connectionRow=$('connection-operation');
  const connectionHome=reconnectingRun?$('cancel'):$('connection-anchor');
  if(reconnectingRun){if(connectionRow.nextElementSibling!==connectionHome)connectionHome.before(connectionRow);setConnection('connecting');}
  else if(connectionHome.nextElementSibling!==connectionRow)connectionHome.after(connectionRow);
  connectionRow.hidden=!(connectingNow || reconnectingRun);
  $('connect-cancel').hidden=reconnectingRun;
  $('connect-cancel').disabled=cancelInProgress;
  $('preparation-operation').hidden=!reading;
  $('preparation-cancel').disabled=cancelInProgress;
  $('manual-connect').disabled=(locked && !browsing) || offline;
  const expired=!busy && !connected && !!state?.reconnection?.timedOut;
  // Other menu pages have one return action; cancellation stays at the operation.
  $('operation-pill').hidden=!document.body.classList.contains('local-page-open') || !(canCancel() || expired);
  $('operation-label').textContent=expired?'Verbindung nicht möglich':cancelInProgress || /Abbruch/.test(state?.operation || '')?'Wird abgebrochen …':searching || state?.search?.active?'Suche läuft':reading?'Gerätewerte werden gelesen …':state?.kind==='run'?'Prüfung läuft':'Verbindung wird aufgebaut';
  const rp=state?.reconnection;
  // 043 (Nutzer 2026-11-14): no second grey line; the remaining time stands inline as „(m:ss)“.
  $('reconnect-progress').hidden=true;
  const left=rp?.active && Number.isFinite(Date.parse(rp.deadline))?Math.max(0,Math.ceil((Date.parse(rp.deadline)-Date.now())/1000)):null;
  // One translatable sentence „Verbindung wird aufgebaut (0:54)“ (no trailing ellipsis before the time).
  const clock=left===null?'':Math.floor(left/60)+':'+String(left%60).padStart(2,'0'),label=$('connection-label');
  if(clock && $('connection-motion').dataset.phase==='connecting'){label.dataset.i18nRendered='';label.textContent=StromeaI18n.t('Verbindung wird aufgebaut ({time})').replace('{time}',clock);}
  else if(label.hasAttribute('data-i18n-rendered')){label.removeAttribute('data-i18n-rendered');label.textContent={connecting:'Verbindung wird aufgebaut', connected:'Verbindung bestätigt', failed:'Verbindung nicht möglich', idle:''}[$('connection-motion').dataset.phase] || '';}
  $('connection-timer').textContent='';
  $('connection-motion').title=left===null?'':'Verbleibende Zeit für Verbindung und Freigabe am Gerät';
  renderStartGate();
  for (let i = 1; i <= 3; i++) {
    // 043: step 2 stays reachable after a dropped connection (values kept, one-click reconnect there).
    $('nav-' + i).disabled = (i!==1 && locked) || (i === 2 && !prepared()) || (i === 3 && !state?.report);
  }
}
// 043 (Nutzer 2026-11-14): a device that was connected once in this selection keeps step 2 reachable, even if the
// connection dropped in between; the backend keeps selection, trust and the read capabilities.
function prepared(){return !real?paired:paired && backendSelectionMatches(state) && !!state?.capabilities;}
// 043 (Bild 7): „Im Netzwerk suchen“ and „Suche stoppen“ share one width, so QR-Code/Manuell never move.
let discoverWidthKey='';
function sizeDiscover(){
 const b=$('discover');if(!b.getClientRects().length)return;
 const key=StromeaI18n.language()+'|'+innerWidth;if(key===discoverWidthKey)return;
 const current=b.textContent;b.style.minWidth='';let width=0;
 for(const label of ['Im Netzwerk suchen','Suche stoppen']){b.textContent=StromeaI18n.t(label);width=Math.max(width,b.getBoundingClientRect().width);}
 b.textContent=current;b.style.minWidth=Math.ceil(width)+'px';discoverWidthKey=key;
}
function goStep(next, focus = true) {
 if(next!==1){stopDictation();stopCamera();}
 if(next===1 && step!==1){pairingView=false;}
 if(next===1 && state?.connected){selectionOverview=true;autoPair=false;clearTimeout(autoAdvanceTimer);autoAdvanceTimer=0;}
  const previous = step, oldHeight = $('step-' + step).offsetHeight;
  step = next;
  for (let i = 1; i <= 3; i++) {
    $('step-' + i).hidden = i !== next;
    if (i === next) $('nav-' + i).setAttribute('aria-current', 'step');
    else $('nav-' + i).removeAttribute('aria-current');
    $('nav-' + i).classList.toggle('done', i < next);
  }
  if (previous !== next) {
    const card = $('step-' + next);
    enter(card, next > previous ? 1 : -1);

  }
  workspace?.step(next);
  controls();
  if (focus) $('title-' + next).focus();
 if(window.StromeaRoute && !StromeaRoute.restoring)StromeaRoute.step(next);
}
function clearErrors() {configFields=[];for(const id of ['pair-error-detail','config-error-detail','run-error-detail'])$(id).textContent=''; setConnection('idle'); for (const id of ['pair-error', 'config-error', 'run-error']) $(id).hidden = true; lastError = ''; }
function showError(where, error, action = null) {
  const source=error.message || String(error);
  const raw=where==='pair'?source:source.replace(/context canceled/g,'Prüfung abgebrochen').replace(/context deadline exceeded/g,'Zeitlimit erreicht');
  $(where + '-error').hidden = false;
  $(where + '-error-detail').textContent = raw;
  if(where==='config'){$('config-error-text').textContent=raw;configFields=error.fields || [];renderLimitSummary();}
  if (where === 'pair') {
    setConnection('idle');
    $('pair-error-title').textContent = 'Verbindung nicht möglich';
    let text = 'Verbindung fehlgeschlagen. Prüfe Adresse und Freigabe am Gerät.';
    if (/EOF|reset by peer|abgewiesen|trust|Vertrauen/i.test(raw)) text = 'Das Gerät hat die Verbindung abgelehnt. Öffne dort das EEBUS-Menü und gib die Simulator-Kennung frei.';
    if (/timeout|deadline|Zeitlimit/i.test(raw)) text = 'Das Gerät antwortet nicht rechtzeitig. Bitte versuche es erneut.';
    const phase=raw.match(/(TCP-Verbindung|TLS-Handshake|WebSocket-Upgrade|SHIP-Handshake).*nicht abgeschlossen/);
    if(phase)text=({'TCP-Verbindung':'Netzwerkverbindung fehlgeschlagen. Bitte prüfe Adresse und Netzwerk und versuche es erneut.','TLS-Handshake':'Sichere Verbindung fehlgeschlagen. Bitte versuche es erneut.','WebSocket-Upgrade':'Datenverbindung fehlgeschlagen. Bitte versuche es erneut.','SHIP-Handshake':'Gerätefreigabe nicht abgeschlossen. Prüfe die Freigabe am Gerät und versuche es erneut.'})[phase[1]];
    if (/unreachable|no route|network is down|connection refused|ECONNREFUSED/i.test(raw)) text = 'Das Gerät ist nicht erreichbar. Prüfe, ob es eingeschaltet und mit dem Netzwerk verbunden ist und ob die IP-Adresse stimmt.';
    const mismatch=/Anderes Gerät|Zertifikat.*SKI|SKI.*(Ziel|öffentlichen)/i.test(raw);
    if (mismatch) {text='Unter dieser Adresse antwortet ein anderes Gerät. Prüfe die IP-Adresse und vergleiche die Gerätekennung erneut.';connected=false;paired=false;}
    if (!real) text = 'Der Simulator ist nicht erreichbar. Versuche die Kopplung erneut.';
    if (/context canceled|abgebrochen|^Du hast/i.test(error.message || String(error))) text='Verbindung abgebrochen. Versuche es erneut oder wähle ein anderes Gerät.';
    if($('ski').value)deviceErrors.set(deviceKey(currentDevice()),error.message || String(error));
    $('pair-error-text').textContent = text;
    renderPairTrust();
    $('pair-error-text').removeAttribute('title');
    $('pair-next').replaceChildren(...(mismatch?['Adresse prüfen.','Vergleiche die Gerätekennung mit der Anzeige am Gerät.','Erneut suchen.']:['Öffne die EEBUS-Einstellungen deines Geräts.','Gib dort die Simulator-Kennung frei.','Prüfe Adresse und Netzwerkverbindung.']).map(text=>{const li=document.createElement('li');li.textContent=text;return li;}));
    $('pair-next').hidden = true; $('error-ski').textContent=formatSki(localSki);$('error-ski').hidden=!real || mismatch; $('error-copy-ski').hidden=true;
    retryAction = action;
    $('retry').hidden = !action;
  }
  if(where==='pair')renderDeviceOverview();
  if(where!=='pair' || step===1)$(where + '-error').scrollIntoView({block: 'nearest'});
}
async function perform(where, action) {
  if (pending || busy) return;
  const epoch=operationEpoch;
  pending = true; clearErrors(); controls();
  try { await action(); } catch (error) { if(epoch===operationEpoch)showError(where, error, () => perform(where, action)); }
  finally { if(epoch===operationEpoch){connectionPending=false;pending = false; controls();} }
}
function modeView() {
  $('mode-simulator').checked = !real; $('mode-onsite').checked = real;
  $('sim-controls').hidden = real; $('lan-controls').hidden = !real;
  $('profile-option').hidden = !real;$('profile-consequence').hidden=!real;
 $('real-config').hidden = !real; $('sim-profile').hidden = real;
  $('practice-badge').hidden=real;$('run').textContent=real?'Prüfung starten':'Prüfung starten · Übungsmodus'; $('practice-details').hidden=real; StromeaProfile.prefill();
  updateDuration(); controls();
}
function applyScope() {
  const scope=$('scope').value, consumption=scope==='consumption',production=scope==='production';
  // UX-S22: the off direction keeps its place, disabled with its reason (no layout jump).
  for(const [id,off] of [['lpp',consumption||!scope],['lpc',production||!scope]]){const field=$(id+'-field');field.hidden=false;field.classList.toggle('field-off',off);$(id).disabled=off;$(id).required=!off;field.querySelector('.field-off-note').hidden=!off;field.querySelectorAll('.text-button').forEach(n=>n.disabled=off);}
  updateDuration();controls();
}
function typeLabel(type) {
 return ({HeatPump:'Wärmepumpe',HeatGenerationSystem:'Wärmeerzeuger',Inverter:'Wechselrichter',ChargingStation:'Wallbox',EnergyManagementSystem:'Energiemanagementsystem'})[type] || 'Gerätetyp nicht angegeben';
}
// 043 (Nutzer 2026-11-14): „Erkanntes Gerät: [Wallbox]“ below „Prüfumfang“, or „unbekannt“.
function renderDeviceType(){
 const chip=$('device-type-chip');if(!chip)return;
 const type=real?deviceInfo?.type:'',known=!!type && typeLabel(type)!=='Gerätetyp nicht angegeben';
 const text=!real?StromeaI18n.t('Simuliertes Gerät'):known?StromeaI18n.t(typeLabel(type)):StromeaI18n.t('unbekannt');
 if(chip.textContent!==text)chip.textContent=text;chip.title=type || '';chip.classList.toggle('chip-unknown',real && !known);
}
function setCapabilities(caps) {
 capabilityState=caps;
 const typ=deviceInfo?.type;
 const detail=document.createElement('dl');detail.id='capability-detail';
 for(const [title,value] of [['Gerätetyp',typeLabel(typ)],...['lpc','lpp','mpc'].map((key,i)=>[['Bezug begrenzen','Einspeisung begrenzen','Leistung messen'][i],(!caps?'Wird abgefragt …':caps[key]?'Unterstützt':'nicht gemeldet')])]){const dt=document.createElement('dt'),dd=document.createElement('dd');if(title==='Gerätetyp' && typeLabel(typ)==='Gerätetyp nicht angegeben')continue;dt.textContent=title;dd.textContent=value;if(title==='Gerätetyp' && typ)dd.title=typ;else{const key=({'Bezug begrenzen':'LPC','Einspeisung begrenzen':'LPP','Leistung messen':'MPC'})[title];if(key)dd.title=key+(caps?.scenarios?' · '+(caps.scenarios[key]?.join(', ') || '–'):'');}detail.append(dt,dd);}
 $('capability-detail')?.remove();$('diagnostics').append(detail);if($('connection-timing-details'))$('diagnostics').append($('connection-timing-details'));
 $('scope-control').hidden=false;
 const wanted=scopeTouched?$('scope').value:caps?.lpc?(caps.lpp?'both':'consumption'):caps?.lpp?'production':'';
 for(const direction of ['lpc','lpp']){const supported=!!caps?.[direction],checkbox=$('scope-'+direction);checkbox.checked=supported && (wanted==='both' || wanted===(direction==='lpc'?'consumption':'production'));checkbox.disabled=!supported;$('scope-'+direction+'-reason').hidden=supported;$('scope-'+direction+'-reason').textContent=caps?'Vom Gerät nicht unterstützt.':'Gerätefunktionen werden gelesen …';}
 syncScope();
 $('measurement-warning').hidden=!real || !caps || !!caps.mpc;
 $('capability-error').hidden=!caps || caps.lpc || caps.lpp;

   $('device-icon').textContent='◇';renderDeviceType();
}
function rememberedMode(){try{return localStorage.getItem('stromea-mode');}catch{return null;}}
function renderPairTrust() {
 const valid=skiComplete();
 $('qr-origin').hidden=!fromQR;$('ski-compare').hidden=fromQR;
 const active=state?.busy && state.kind==='connect', trust=state?.trust || {};
 const error=state?.kind==='connect' && state?.device?.ski?.replace(/\s/g,'').toLowerCase()===$('ski').value.replace(/\s/g,'').toLowerCase()?state.error:'';
 const same=backendSelectionMatches(state);
 if(same && deviceInfo){const instance=selected?.name || deviceInfo.name;const model=deviceInfo.model && deviceInfo.model!==instance?deviceInfo.model:selected?.model;renderDeviceTitle({...selected,...deviceInfo,brand:deviceInfo.brand || selected?.brand,model:model || '',name:instance});}
 const deviceOK=same && state?.connected && trust.device===true, simulatorOK=same && state?.connected && trust.simulator===true;
 $('title-1').toggleAttribute('data-user-content',pairingView && valid);$('title-1').textContent=pairingView && valid?deviceTitle(pairingDevice()):'Gerät verbinden';
 const mismatch=/Anderes Gerät|Zertifikat.*SKI|SKI.*(Ziel|öffentlichen)/i.test(error || '');
 const refused=!simulatorOK && /abgewiesen|abgelehnt|trust|Vertrauen|EOF|timeout|deadline|Zeitlimit/i.test(error || '');
 $('device-trust').textContent=deviceOK?'✓ Gerätekennung geprüft':mismatch?'Gerätekennung stimmt nicht überein':'';
 // 039 (QA-12): one consistent state – no "waiting for approval" next to a connection error.
 // 043 (Nutzer 2026-11-14, Bild 4/5): „Wartet auf Freigabe“ only while a connection is being set up; a connection
 // that was confirmed and then dropped (e.g. WLAN) says so instead of falling back to the approval state.
 const dropped=!active && same && !!state?.paired && !!state?.capabilities && !state?.connected;
 $('simulator-trust').textContent=simulatorOK?'✓ Am Gerät freigegeben':error || !$('pair-error').hidden?(refused?'Nicht freigegeben':''):active?'Wartet auf Freigabe am Gerät…':dropped?'Verbindung getrennt':'';
 $('device').classList.toggle('trusted',deviceOK);$('own-identity').classList.toggle('trusted',simulatorOK);
 $('device').classList.toggle('trust-error',mismatch);$('own-identity').classList.toggle('trust-error',refused);
 $('pair-link').dataset.errorSide=mismatch?'device':refused?'simulator':'';
 $('pair-link').dataset.state=deviceOK && simulatorOK?'trusted':error?'failed':active?'waiting':'idle';
 $('pair-link-label').textContent=state?.controlUnresponsive?'Gerät antwortet nicht auf Steuerbefehle':deviceOK && simulatorOK?'Gekoppelt':error?(mismatch?'Gerätekennung stimmt nicht überein':'Verbindung fehlgeschlagen'):active?'Wartet auf Freigabe am Gerät…':dropped?'Verbindung getrennt':'Nicht verbunden';
 $('own-identity').classList.toggle('trust-dropped',dropped);
 if(mismatch){connected=false;paired=false;}
 $('device').hidden=!valid;
}
$('manual-toggle').addEventListener('click',()=>{workspace.show('manual',$('manual-toggle'));$('address').focus();});
// 041 (Nutzer 2026-11-14): der Hinweis steht ohne Umbruch in derselben Zeile wie die Prüfdauer.
const keepOpen='Lass das Browserfenster geöffnet. Prüfe am Ende, ob die Testgrenzen aufgehoben wurden.';
function updateDuration() {
 $('advanced-summary').textContent=timeFields.every(([id])=>$(id).value===$(id).defaultValue)?'(Standardwerte)':'(angepasst)';

  const n=$('scope').value==='both'?2:1;
  const seconds=id=>Number($(id).value)||0;
  const full=$('profile-full').checked,measurable=!capabilityState || !!capabilityState.mpc;
  const estimate=StromeaGuide.budget({scope:$('scope').value,effect:seconds('effect-timeout'),outage:seconds('failsafe-timeout'),step:seconds('step-timeout'),cleanup:seconds('cleanup-timeout'),minimum:seconds('failsafe-minimum'),profile:full?'full':'quick',measurable});
  if(!real){if($('protocol-budget'))$('protocol-budget').hidden=true;$('long-test-minimum').hidden=true;const practice=new Intl.NumberFormat(StromeaI18n.locale(),{style:'unit',unit:'second',unitDisplay:'short'}).format(2);$('duration').textContent=StromeaI18n.t('Echte Prüfung: bis {real}. Übung: ca. {practice}.').replace('{real}',StromeaGuide.duration(estimate,StromeaI18n.locale()).replace(/\.$/,'' )).replace('{practice}',practice.replace(/\.$/,''))+' '+StromeaI18n.t(keepOpen);$('duration-help').dataset.tip='Die Übung verkürzt reale Wartezeiten. Die angegebene echte Prüfdauer ist das Zeitbudget für die gewählten Einstellungen; im Ergebnis steht die tatsächlich verstrichene Zeit.';return;}
  $('profile-consequence').textContent=!measurable?'Ohne Leistungsmessung werden nur die Geräteantworten geprüft. Leistungsänderung und Mindestdauer bleiben ungeprüft.':full?'Die Langzeitprüfung wartet zusätzlich die Mindestdauer des Notbetriebs ab.':'Die Funktionsprüfung prüft Drosselung und Notbetrieb. Die Mindestdauer des Notbetriebs bleibt ungeprüft.';
  let protocol=$('protocol-budget');if(!protocol){protocol=document.createElement('p');protocol.id='protocol-budget';protocol.className='help';$('preflight-notes').append(protocol);}protocol.hidden=measurable;protocol.textContent='Ohne Leistungsmessung werden nur die Geräteantworten geprüft. Leistungsänderung und Mindestdauer bleiben ungeprüft.';
  $('duration').textContent=StromeaI18n.t('Prüfdauer: höchstens etwa {duration}.').replace('{duration}',StromeaI18n.t(StromeaGuide.duration(estimate,StromeaI18n.locale())).replace(/\.$/,''))+' '+StromeaI18n.t(keepOpen);
  $('long-test-minimum').hidden=!full || !measurable;
  $('long-test-minimum').textContent='Die Mindestdauer von '+(n*seconds('failsafe-minimum')/3600).toLocaleString(StromeaI18n.locale())+' h wird abgewartet.';
  const parts={effect:measurable?seconds('effect-timeout'):0,outage:seconds('failsafe-timeout'),step:seconds('step-timeout'),cleanup:seconds('cleanup-timeout')};
  $('duration-help').dataset.tip=StromeaI18n.t('Je Prüfrichtung: Drosselung (3 × {effect} s) + Ausfallbeobachtung ({outage} s + 30 s Reserve) + Steuerbefehle (10 × {step} s) + Rückstellung (3 × {cleanup} s).').replace(/\{(effect|outage|step|cleanup)\}/g,(_,key)=>String(parts[key]))+' '+(!measurable?StromeaI18n.t('Nur Geräteantworten: keine Zeit für elektrische Wirkungsnachweise eingeplant.'):full?$('long-test-minimum').textContent:StromeaI18n.t('Funktionsprüfung: Rückkehr nach Mindestdauer bleibt ungeprüft.'));

}
function syncScope(){const lpc=$('scope-lpc').checked,lpp=$('scope-lpp').checked;$('scope').value=lpc?(lpp?'both':'consumption'):lpp?'production':'';applyScope();}
for(const direction of ['lpc','lpp'])$('scope-'+direction).addEventListener('change',()=>{scopeTouched=true;syncScope();try{sessionStorage.setItem('stromea-scope-'+(real?'onsite-'+(state?.device?.ski||''):'simulator'),$('scope').value);}catch{}});
async function loadInterfaces() {
  const interfaces = await api('/api/interfaces');
  const signature=JSON.stringify(interfaces);if(signature===loadInterfaces.signature && $('interface').options.length){controls();return;}loadInterfaces.signature=signature; // unchanged list: keep the open select untouched
  const previous=$('interface').value,manualPrevious=$('manual-interface').value;
  $('interface').replaceChildren();$('manual-interface').replaceChildren();const auto=document.createElement('option');auto.value='';auto.textContent='Automatisch wählen';$('manual-interface').append(auto);
  for (const i of (interfaces || []).filter(i=>!i.loopbackOnly && !/^lo(?:\d*|opback)$/.test(i.name) && !/^127\./.test(i.address) && i.address!=='::1')) {
    interfaceKinds.set(i.name,i.kind || '');
    const option = document.createElement('option'); option.value = i.name; option.textContent = (({wifi:'WLAN',ethernet:'Ethernet',vpn:'VPN'})[i.kind] || 'Netzwerk') + ' · ' + i.address; option.dataset.technical=i.name; $('manual-interface').append(option.cloneNode(true));$('interface').append(option);
  }
  if (!$('interface').options.length) {
    const option = document.createElement('option'); option.value = ''; option.textContent = 'Keine Netzwerkverbindung'; $('interface').append(option);
  }
 if([...$('interface').options].some(o=>o.value===previous))$('interface').value=previous;
 if([...$('manual-interface').options].some(o=>o.value===manualPrevious))$('manual-interface').value=manualPrevious;
 const single=$('interface').options.length===1;$('interface-tip').hidden=single;$('interface').hidden=single;$('interface-single').hidden=!single;$('interface-single').textContent=$('interface').selectedOptions[0]?.textContent || '';$('manual-interface').hidden=single;$('manual-interface-single').hidden=!single;$('manual-interface-single').textContent=$('interface-single').textContent;controls();
}
// 043 (Nutzer 2026-11-14): a search result belongs to the adapter it was found on; the row names it and the
// connection uses exactly this adapter (also after the adapter selection changed).
const interfaceKinds=new Map();
function adapterLabel(name){if(!name)return '';const kind=interfaceKinds.get(name);return StromeaI18n.t(({wifi:'WLAN',ethernet:'Ethernet',vpn:'VPN'})[kind] || 'Netzwerk')+' ('+name+')';}
for (const id of ['mode-simulator', 'mode-onsite']) $(id).addEventListener('change', () => perform('pair', async () => {
  stopCamera();
 stopDictation();const nextReal = $('mode-onsite').checked;
  if (!nextReal) await api('/api/simulator', {});selectionOverview=false;
  try{localStorage.setItem('stromea-mode',nextReal?'onsite':'simulator');}catch{}
  $('step-1').classList.remove('has-selection');capabilityState=null;deviceInfo=null;real = nextReal; paired = false; connected = false; selected = null;
  $('device').hidden = true; $('device-choices').hidden = true; $('search-status').hidden = true;
  $('ski').value = ''; $('address').value = ''; $('pair-status').textContent = '';
  state = null;devices=[];deviceErrors.clear();savedDevices.clear();pairingView=false;modeView(); goStep(1, false);
  if (real) await loadInterfaces();
}));
$('interface').addEventListener('change', () => { const single=$('interface').options.length===1;$('interface-tip').hidden=single;$('interface').hidden=single;$('interface-single').hidden=!single;$('interface-single').textContent=$('interface').selectedOptions[0]?.textContent || '';$('manual-interface').hidden=single;$('manual-interface-single').hidden=!single;$('manual-interface-single').textContent=$('interface-single').textContent;controls(); $('search-status').hidden = true; });
// 043 (Nutzer 2026-11-14, Bild 8): connection details stand in the right detail column like all other details,
// never as a collapsible caption below the step.
function timingSection(){
 let section=$('connection-timing-details');
 if(!section){section=document.createElement('section');section.id='connection-timing-details';section.className='connection-technical';const heading=document.createElement('h4');heading.textContent='Verbindungsaufbau';section.append(heading,$('device-model'));$('diagnostics').append(section);}
 return section;
}
function renderConnectionTimings(rows){
 let box=$('connection-timings');
 if(!box){box=document.createElement('pre');box.id='connection-timings';box.className='raw-technical';timingSection().append(box);}
 box.hidden=!rows.length;$('pair-technical').hidden=true;
 box.textContent=rows.map(row=>row.phase+': '+row.durationMs+' ms · '+StromeaI18n.t(({complete:'Abgeschlossen',active:'In Prüfung',failed:'Fehlgeschlagen',canceled:'Abgebrochen'})[row.status] || row.status)+(row.detail?' · '+row.detail:'')).join('\n');
}
function mergeDevices(found){
 for(const d of found){if(!d?.ski || !d.address)continue;const i=devices.findIndex(item=>deviceKey(item)===deviceKey(d));if(i<0)devices.push({...d});else devices[i]={...devices[i],...d};}
}
function renderDiscovered(found){mergeDevices(found.map(d=>({...d,discovered:true})));renderDeviceOverview();}
function renderDeviceCount(){
 // 039 (QA-11): manually entered or QR devices are not "found"; they are counted as manual.
 const count=devices.filter(d=>d.discovered).length,saved=devices.filter(d=>!d.discovered && !d.manual && savedDevices.has(deviceKey(d))).length,manual=devices.length-count-saved,lang=StromeaI18n.language(),category=new Intl.PluralRules(lang).select(count);
 const phrase=category==='one'?'Gerät gefunden':['pl','ru','uk'].includes(lang) && category==='many'?'Geräte gefunden (viele)':'Geräte gefunden';
 const copy=[count?count+' '+StromeaI18n.t(phrase):'',manual?manual+' '+StromeaI18n.t('manuell hinzugefügt'):'',saved?saved+' '+StromeaI18n.t('gespeichert'):''].filter(Boolean).join(' · ');if($('device-count').textContent!==copy)$('device-count').textContent=copy;
}
function renderDeviceOverview(){
 if(state?.deviceMode && state?.paired){mergeDevices([state.device]);savedDevices.add(deviceKey(state.device));}
 if(selected)mergeDevices([selected]);
 const key=deviceKey(currentDevice()),same=backendSelectionMatches(state || {}),valid=skiComplete();
 const status=d=>state?.deviceMode && deviceKey(state.device)===deviceKey(d) && state.connected?'connected':deviceErrors.has(deviceKey(d))?'error':savedDevices.has(deviceKey(d))?'saved':d.discovered?'found':'manual';
 const labels={connected:'Verbunden',saved:'Gespeichert',found:'Gefunden',manual:'Manuell',error:'Fehler'};
 const signature=JSON.stringify([devices,devices.map(status),StromeaI18n.language(),[...interfaceKinds]]);
 if(signature!==deviceListSignature){
  const focused=document.activeElement?.closest('.device-choice')?.dataset.key;
  deviceListSignature=signature;$('device-list').replaceChildren();
  devices.forEach((d,i)=>{
   const label=document.createElement('label'),radio=document.createElement('input'),content=document.createElement('span');
   label.className='device-choice';label.dataset.key=deviceKey(d);label.dataset.state=status(d);
   radio.type='radio';radio.name='device-choice';radio.value=String(i);
   const title=document.createElement('strong'),address=document.createElement('span'),badge=document.createElement('span'),chevron=document.createElement('span');
   title.textContent=deviceTitle(d);title.dataset.userContent='true';title.title=d.name || '';
   address.textContent=StromeaI18n.t('Gerätekennung (SKI)')+': '+formatSki(d.ski)+' · IP: '+d.address;address.title=[d.address,d.interface,d.localAddress].filter(Boolean).join(' · ');
   content.className='device-copy';badge.className='device-state';badge.textContent=labels[status(d)];chevron.className='device-chevron';chevron.textContent='›';chevron.setAttribute('aria-hidden','true');
       const adapter=d.interface?adapterLabel(d.interface):'';
    if(adapter){const via=document.createElement('span');via.className='device-adapter';via.dataset.i18nRendered='';via.textContent=StromeaI18n.t('über {adapter}').replace('{adapter}',adapter);via.title=StromeaI18n.t('Gefunden über diesen Netzwerkadapter; die Verbindung nutzt ihn ebenfalls.');address.append(document.createTextNode(' · '),via);}
    content.append(title,address);label.append(radio,content);if(status(d)!=='found')label.append(badge);label.append(chevron);$('device-list').append(label);
   radio.addEventListener('change',async()=>{await chooseDevice(d);controls();if(pairingView)$('title-1').focus();});
   radio.addEventListener('click',()=>{if(deviceKey(currentDevice())===deviceKey(d) && !pairingView){pairingView=true;controls();$('title-1').focus();}});
   if(focused===deviceKey(d))radio.focus({preventScroll:true});
  });
 }
 for(const radio of $('device-list').querySelectorAll('input'))radio.checked=deviceKey(devices[Number(radio.value)])===key;
 $('device-choices').hidden=!devices.length;$('device-list-empty').hidden=!!devices.length || pairingView; /* 043: stays during the search (no jump) */renderDeviceCount();
 renderPairingView(valid);$('pair-empty').hidden=valid;$('pair-approval').hidden=!valid || !!(same && state?.connected) && $('pair-success').hidden; // 043: no jump while confirming
 const explanation=$('device-explanation');
 explanation.hidden=!valid || !savedDevices.has(key) || !!(state?.busy && state.kind==='connect') || !$('pair-error').hidden;
 const retained=same && state?.connected;
 explanation.textContent=retained?'Dieses Gerät ist verbunden. Du kannst mit der Prüfung fortfahren.':'Dieses Gerät ist gespeichert. Verbinde es erneut für die Prüfung.';
 explanation.title=retained?'Die bestehende Verbindung bleibt beim Wechsel der Schritte erhalten. Erst ein Gerätewechsel trennt sie.':'Die Kennung ist in dieser Sitzung vorgemerkt. Eine aktive Verbindung und die Freigabe am Gerät werden erneut geprüft.';
 const timings=$('connection-timings'),failed=!$('pair-error').hidden;
 const technical=!!state?.connectionTimings?.length && (step!==1 || same || failed);
 timingSection().hidden=!technical;if(timings)timings.hidden=!technical;
 $('pair-technical').hidden=true;
 const rows=(same?state?.connectionTimings:[]) || [],phases=['TCP-Verbindung','TLS-Handshake','WebSocket-Upgrade','SHIP-Handshake'];
 const phaseSignature=JSON.stringify(rows.map(r=>[r.phase,r.status]));
 if($('pair-phases').dataset.signature!==phaseSignature){
  $('pair-phases').dataset.signature=phaseSignature;$('pair-phases').replaceChildren();
  for(const phase of phases){const row=rows.find(r=>r.phase===phase),li=document.createElement('li');li.textContent=({complete:'✓ ',active:'● ',failed:'× ',canceled:'× '})[row?.status] || '– ';li.append(document.createTextNode(({'TCP-Verbindung':'Netzwerk','TLS-Handshake':'Sicherheit','WebSocket-Upgrade':'Datenkanal','SHIP-Handshake':'Gerätefreigabe'})[phase]));li.title=phase;li.dataset.status=row?.status || 'waiting';if(row?.status==='active')li.setAttribute('aria-current','step');$('pair-phases').append(li);}
 }
}
async function discover() {
 $('step-1').classList.remove('has-selection');
  const epoch=operationEpoch,generation=++searchGeneration;
 clearErrors();$('search-technical')?.remove();$('discovery-detail')?.remove();searching=true;searchSignature='';controls();
 $('search-motion').hidden = false;
  $('discover').setAttribute('aria-busy', 'true');
  $('search-status').hidden = true;
  try {
    const found = await api('/api/discover-lan', {interface: $('interface').value}) || [];
    if(epoch!==operationEpoch || generation!==searchGeneration)return;
    if (!found.length) {
      $('search-status').setAttribute('role', 'status');
      $('search-status').className = 'notice warning';
      $('search-status').textContent = 'Kein Gerät gefunden. Prüfe, ob das Gerät eingeschaltet, im selben Netzwerk und für EEBUS freigegeben ist.';
      $('search-status').hidden = false; return;
    }
    renderDiscovered(found);
  } catch (error) {
    if(epoch!==operationEpoch || generation!==searchGeneration)return;
    const box = $('search-status'), raw = document.createElement('pre');
    box.setAttribute('role', 'alert'); box.className = 'error';
    box.textContent = 'Die Gerätesuche konnte nicht starten. Prüfe die Netzwerkverbindung oder gib die Geräteadresse manuell ein.';
    // 043: technical details in the right detail column, not as a collapsible caption below.
    raw.textContent=error.message;raw.className='raw-technical';$('discovery-detail')?.remove();raw.id='discovery-detail';$('diagnostics').append(raw);box.hidden=false;
  } finally { if(epoch===operationEpoch && generation===searchGeneration){searching=false;searchSignature='';$('search-motion').hidden = true; $('discover').textContent = 'Im Netzwerk suchen'; $('discover').setAttribute('aria-busy', 'false');controls();} }
}
$('discover').addEventListener('click', () => {if(searching){stopBrowsingSearch();return;}if(!pending && (!busy || selectionOverview) && !cancelInProgress)discover();});
async function stopBrowsingSearch(){
 ++searchGeneration;searching=false;searchSignature='';if(requestKind==='/api/discover-lan')requestAbort?.abort();
 try{await api('/api/discover-stop',{});}catch(error){showError('pair',error);}controls();
}
function focusDeviceList(){const choice=$('device-list').querySelector('input:checked:not(:disabled)') || $('device-list').querySelector('input:not(:disabled)');(choice || $('discover')).focus();}
async function changeDevice(){
 if(state?.connected){selectionOverview=true;autoPair=false;clearTimeout(autoAdvanceTimer);goStep(1);renderPairTrust();controls();focusDeviceList();return;}
 await cancel();
 if(busy || cancelInProgress)return;
 clearTimeout(autoAdvanceTimer);autoAdvanceTimer=0;autoPair=false;
 goStep(1);pairingView=false;
 $('step-1').classList.remove('has-selection');selected=null;paired=false;connected=false;fromQR=false;
 $('ski').value='';$('address').value='';$('device').hidden=true;$('pair-status').textContent='';clearErrors();renderPairTrust();controls();focusDeviceList();
}
$('change-device').addEventListener('click',showDeviceList);
function showDeviceList(){pairingView=false;controls();focusDeviceList();}
$('pair-help').addEventListener('keydown',e=>{if(e.key==='Escape' && $('pair-help').open){$('pair-help').open=false;e.preventDefault();e.stopPropagation();$('pair-help').querySelector('summary').focus();}});
document.addEventListener('click',e=>{if(!$('pair-help').contains(e.target))$('pair-help').open=false;});
$('other-pair-device').addEventListener('click',changeDevice);
async function chooseDevice(d){
 const same=state?.deviceMode && state?.connected && deviceKey(state.device)===deviceKey(d);
 if(same){pairingView=true;selected=devices.find(item=>deviceKey(item)===deviceKey(d)) || d;selectionOverview=true;renderPairTrust();controls();return;}
 if(state?.connected || busy){
  if(busy && state?.kind==='run' && !window.confirm(StromeaI18n.t('Prüfung abbrechen und anderes Gerät wählen?')))return;
  await cancel();if(busy || cancelInProgress)return;
 }
 showDevice(d);
}
$('connected-continue').addEventListener('click',()=>goStep(busy && state?.kind==='run'?3:2));
function showDevice(d,restoring=false) {
 pairingView=true;
 $('device-copy-status').textContent='';
 selectionOverview=false;
 $('step-1').classList.add('has-selection');
  fromQR=false;if(!restoring)scopeTouched=false;selected = d; paired = false; connected = false; fromQR=false;
  $('address').value = d.address || ''; $('ski').value = StromeaInput.ski(d.ski || ''); skiCount();
  renderDeviceTitle(d);if(!restoring)workspace.resetProduction();
  renderDeviceSKI(d.ski); $('device').hidden = !d.ski;
  clearErrors();renderPairTrust();
  const previousError=deviceErrors.get(deviceKey(d));if(previousError)showError('pair',previousError,()=>perform('pair',pairDevice));
  controls();
}
for (const id of ['ski', 'address']) $(id).addEventListener('input', () => {
  $(id+'-error').hidden=true;$(id).removeAttribute('aria-invalid');
  selected=null;fromQR=false;paired=false;connected=false;
  for (const radio of document.querySelectorAll('input[name="device-choice"]')) radio.checked = false;
  if(id==='ski'){$('device-name').removeAttribute('data-user-content');$('device-name').textContent = 'Gerätekennung (SKI)'; $('device-model').textContent = '';}
  const field=$('ski'),pos=field.selectionStart || 0,hexBefore=field.value.slice(0,pos).replace(/[\s:-]/g,'').length;
 field.value=StromeaInput.ski(field.value); if(id==='ski'){const cursor=hexBefore+Math.floor(Math.max(0,hexBefore-1)/4);field.setSelectionRange(cursor,cursor);}
 skiCount();renderDeviceSKI(field.value);
 if(id==='ski'){const problem=StromeaInput.skiProblem(field.value);if(problem && problem.kind!=='short')skiError(problem);}
 $('device').hidden = !skiComplete();
  $('step-1').classList.toggle('has-selection',skiComplete());
  clearErrors();renderPairTrust(); controls();
});
function deviceTitle(d){
 const brand=(d.brand || '').trim(),model=(d.model && !/[0-9a-f]{16,}/i.test(d.model)?d.model:'').trim();
 const title=(brand && (model.toLowerCase()===brand.toLowerCase() || model.toLowerCase().startsWith(brand.toLowerCase()+' '))?model:[brand,model].filter(Boolean).join(' '));
 return title || (d.name && !/[0-9a-f]{16,}/i.test(d.name)?d.name:d.type?StromeaI18n.t(typeLabel(d.type)):StromeaI18n.t('Gerät'));
}
function pairingDevice(){
 const info=backendSelectionMatches(state)?deviceInfo:null;
 const combined={...info};for(const [key,value] of Object.entries(selected || {}))if(value)combined[key]=value;return combined;
}
function renderDeviceTitle(d){
 $('device-name').textContent='Gerätekennung (SKI)';$('device-name').removeAttribute('data-user-content');$('device-name').title=deviceTitle(d);
 $('device-model').replaceChildren();const name=document.createElement('span');name.dataset.userContent='true';name.textContent=d.name || '';name.title=d.name || '';$('device-model').append(name);
}
function renderPairingView(valid){
 const open=pairingView && valid;
 $('step-1').classList.toggle('pairing-open',open);$('step-1').classList.toggle('pairing-confirmed',!!connected);
 for(const id of ['device-browser','device-catalog'])$(id).hidden=open;
 $('device-results-title').hidden=open || !devices.length;
 $('device-count').hidden=open || !devices.length;
 $('pair-detail').hidden=!open;$('change-device').hidden=!open;
 $('title-1').toggleAttribute('data-user-content',open);$('title-1').textContent=open?deviceTitle(pairingDevice()):'Gerät verbinden';
}

function renderSKIField(box,value){box.replaceChildren();const blocks=formatSki(value).split(' ');for(let i=0;i<blocks.length;i+=5){const row=document.createElement('div');row.className='ski-row';for(let j=i;j<Math.min(i+5,blocks.length);j++){const span=document.createElement('span');if(j===blocks.length-1){const strong=document.createElement('strong');strong.textContent=blocks[j];span.append(strong);}else span.textContent=blocks[j];row.append(span);}box.append(row);}}

function renderDeviceSKI(value) {
 $('device-ending').textContent=formatSki(value).split(' ').pop() || '–';
 renderSKIField($('device-ski'),value);
}
async function pairSimulator() {
 stopCamera();
  const s = await api('/api/state'); localSki = s.localSki;
  await api('/api/pair', {ski: s.device.ski});
  paired = true; connected = true; $('paired-status').textContent = '✓ Simulator erfolgreich gekoppelt.';
  setConnection('connected');
  $('paired-status').replaceChildren(drawnCheck(), document.createTextNode('Simulator erfolgreich gekoppelt.'));
  capabilityState={lpc:true,lpp:true,mpc:true};setCapabilities(capabilityState);goStep(2);
}
$('pair-simulator').addEventListener('click', () => perform('pair', pairSimulator));
async function pairDevice() {
 if(offlineNow())throw new Error('Keine Netzwerkverbindung'); // 039 R039-03: retries obey the same lock
 const epoch=operationEpoch;
 deviceErrors.delete(deviceKey(currentDevice()));
 state={...state,connectionTimings:[]};renderConnectionTimings([]);
 // A choice stays interactive while mDNS runs. Connecting joins the backend
 // search barrier; invalidate its pending response before starting a new job.
 if(searching){++searchGeneration;searching=false;if(requestKind==='/api/discover-lan')requestAbort?.abort();}
 stopCamera(); autoPair=true;setConnectOrigin(1);clearTimeout(autoAdvanceTimer);autoAdvanceTimer=0;connectionPending=true;controls();setConnection('connecting');
 $('connection-operation').scrollIntoView({block:'nearest'});
  const {manual:_manual,discovered:_discovered,...device}=selected || {}; // 039: UI-only origin flags never reach the API
  await api('/api/select', {device: {...device, name: selected?.name || '', ski: $('ski').value.replace(/\s/g,''), address: $('address').value,interface: selected?.interface || $('manual-interface').value,localAddress:selected?.localAddress || ''}, confirmed: true});
  if(epoch!==operationEpoch)return;
  await api('/api/connect', {timeout: Number($('step-timeout').value) * 1e9});
  if(epoch!==operationEpoch)return;
  busy = true; state = {...state,kind: 'connect',busy:true};
  $('pair-status').textContent = '';
  await refresh();
 if(epoch===operationEpoch && step===1 && !document.body.classList.contains('local-page-open'))$('connect-cancel').scrollIntoView({block:'nearest'});
}

$('pair').addEventListener('click', () => selectionOverview && state?.connected && backendSelectionMatches(state) ? goStep(busy && state?.kind==='run'?3:2) : state?.controlUnresponsive ? perform('pair',reconnectDevice) : paired && connected ? goStep(busy && state?.kind==='run'?3:2) : perform('pair', pairDevice));
$('retry').addEventListener('click', () => retryAction?.());
async function copySki() {
  try { await navigator.clipboard.writeText(localSki); $('copy-status').textContent = '✓ Schlüssel kopiert.'; copyFeedback(); }
  catch { $('copy-status').textContent = 'Markiere den Schlüssel und kopiere ihn mit der Tastatur.'; }
}
$('copy-ski').addEventListener('click',copySki);$('error-copy-ski').addEventListener('click',copySki);
for (let i = 1; i <= 3; i++) $('nav-' + i).addEventListener('click', () => goStep(i));
$('back').addEventListener('click', () => goStep(1));
$('other-device').addEventListener('click',changeDevice);
$('repeat').addEventListener('click', async () => { await refresh();clearErrors(); goStep(prepared()?2:1); });
// 043: „Neu verbinden“ from the result: connect the same device again, then step 2 opens with all values.
function reconnectFromResult(){
 pairingView=true;goStep(1);
 perform('pair',async()=>{const epoch=operationEpoch;autoPair=true;setConnectOrigin(1);connectionPending=true;controls();setConnection('connecting');await api('/api/connect',{timeout:Number($('step-timeout').value)*1e9});if(epoch!==operationEpoch)return;busy=true;state={...state,kind:'connect',busy:true};await refresh();});
}
for (const [id] of timeFields) $(id).addEventListener('input', updateDuration);
// 039 (QA-14/15, B-02): one validation for submit and live correction; the summary names and links
// the fields, focus moves to the first one (after perform() re-enables the form).
const limitKeys=['lpc','lpp','failsafe','tolerance'];
function limitValue(key){return StromeaInput.watts($(key).value,StromeaI18n.locale(),['lpc','lpp'].includes(key)?0.001:0);}
// 041 (Nutzer 2026-11-14): Watt-Felder mit automatischem Tausendertrennzeichen der UI-Sprache.
let wattLocale=StromeaI18n.locale();
function formatWatts(key,keepCaret=false){const el=$(key),caret=el.selectionStart ?? el.value.length,f=StromeaInput.wattsFormat(el.value,StromeaI18n.locale(),caret);if(f.text===el.value)return;el.value=f.text;if(keepCaret && document.activeElement===el)el.setSelectionRange(f.caret,f.caret);}
for(const key of limitKeys){$(key).addEventListener('input',e=>{if(!e.isComposing)formatWatts(key,true);});formatWatts(key);}
window.addEventListener('stromea-language',()=>{const next=StromeaI18n.locale();if(next===wattLocale)return;for(const key of limitKeys)$(key).value=StromeaInput.wattsConvert($(key).value,wattLocale,next);wattLocale=next;});
function limitError(key,message){$(key+'-error').textContent=message;$(key+'-error').hidden=!message;if(message){$(key).setAttribute('aria-invalid','true');$(key).setAttribute('aria-describedby',key+'-error '+($(key).dataset.description || ''));}else{$(key).removeAttribute('aria-invalid');$(key).setAttribute('aria-describedby',$(key).dataset.description || '');}}
function invalidLimits(){return limitKeys.filter(key=>$(key).getAttribute('aria-invalid')==='true');}
// 043 (Nutzer 2026-11-14): „Angaben prüfen“ names the concrete fields as links; the detail column shows the same.
let configFields=[];
function fieldError(message,fields){const error=new Error(message);error.fields=fields;return error;}
function fieldLabel(key){const label=key==='scope-lpc'?$('scope-control').querySelector('legend'):document.querySelector('label[for="'+key+'"]');return label?StromeaI18n.sourceText(label).trim():key;}
function renderLimitSummary(){
 const list=$('config-error-fields'),keys=[...new Set([...configFields.filter(key=>!$(key)?.disabled || key==='scope-lpc'),...invalidLimits()])];list.replaceChildren();list.hidden=!keys.length;
 for(const key of keys){const li=document.createElement('li'),a=document.createElement('a');a.href='#'+key;a.textContent=fieldLabel(key);a.addEventListener('click',e=>{e.preventDefault();$(key).focus();$(key).scrollIntoView({block:'center'});});li.append(a);list.append(li);}
 if(!keys.length && /Markierte Felder|markierte Felder/.test($('config-error-detail').textContent))$('config-error').hidden=true;
}
for(const key of limitKeys)$(key).addEventListener('input',()=>{if($(key).getAttribute('aria-invalid')!=='true')return;try{limitValue(key);limitError(key,'');}catch(error){limitError(key,error.message);}renderLimitSummary();});
$('form').addEventListener('submit', e => {
  e.preventDefault();
  // 043 (Nutzer 2026-11-14): only the inspector's name is required. Missing → persistent dialog, then start.
  if(real && !pending && !busy && !(StromeaProfile.snapshot().inspector || '').trim()){openInspectorDialog();return;}
  perform('config', async () => {
    const epoch=operationEpoch;
    const config = {profile:StromeaProfile.snapshot(),site:Object.fromEntries(['customer','customerAddress','address','meter','note'].map(key=>[key,$('site-'+key).value.trim()])),business:StromeaProfile.snapshot().business || (!real?'Beispielbetrieb (Übungsmodus)':''), inspector:StromeaProfile.snapshot().inspector || (!real?'Beispielperson (Übungsmodus)':''), scope:$('scope').value, pauseLowLoad:real, signOperator: $('sign-operator').checked, signInspector: $('sign-inspector').checked};
    if(!config.scope)throw fieldError('Wähle mindestens eine Prüfrichtung.',['scope-lpc']);
    let invalid=false;
    for (const key of ['lpc','lpp','failsafe','tolerance']) {
      limitError(key,'');
      if(key==='lpp' && config.scope==='consumption' || key==='lpc' && config.scope==='production'){config[key]=0;continue;}
      try{config[key]=limitValue(key);}
      catch(error){limitError(key,error.message);invalid=true;}
    }
    if(invalid)throw fieldError('Markierte Felder korrigieren.',[]);
    if(config.tolerance*2>=(config.scope==='consumption'?config.lpc:config.scope==='production'?config.lpp:Math.min(config.lpc,config.lpp))-config.failsafe)throw fieldError('Testgrenzen müssen ausreichend über dem Notbetrieb liegen.',['failsafe','tolerance']);
    config.testProfile=real && $('profile-full').checked?'full':'quick';
    if(real)for(const [id,key] of timeFields)config[key]=StromeaInput.number($(id).value,0.001,id==='failsafe-timeout'?120:id==='effect-timeout'?3600:id==='failsafe-minimum'?86400:id==='step-timeout'||id==='cleanup-timeout'?600:90000,'s')*1e9;
    stopCamera();
 await api('/api/run', config);
    if(epoch!==operationEpoch)return;
    resetPower({...config});
    completed = 0; renderedSteps = ''; $('steps').replaceChildren(); $('downloads').hidden = true; $('outcome').hidden = true;
    $('result-matrix').hidden=true;$('open-evidence').hidden=true;$('other-device').hidden=true;$('all-steps').open=false;
    busy = true; state = {kind: 'run',busy:true}; goStep(3); await refresh();
  }).then(()=>{renderLimitSummary();const first=invalidLimits()[0];if(first && !$('config-error').hidden)$(first).focus();});
});
async function cancel() {
 if(cancelInProgress)return;
 const cancelingRead=state?.kind==='prepare' || requestKind==='/api/prepare';
 const cancelingRun=state?.kind==='run' && busy;
 const cancelingConnection=connectionPending || state?.kind==='connect' && busy;
 cancelInProgress=true;operationEpoch++;pending=true;
 clearTimeout(autoAdvanceTimer);autoAdvanceTimer=0;autoPair=false;
 controls();
 const feedback=$(step===1?'pair-status':'run-status');feedback.textContent='Wird abgebrochen …';feedback.classList.add('cancelling');
 $('cancel').textContent='Wird abgebrochen …';$('cancel').setAttribute('aria-busy','true');
 try{
  // Finish an already submitted start before canceling its backend operation.
  // An earlier local epoch prevents any subsequent start in that action.
  requestAbort?.abort();
  if(startRequest)await startRequest.catch(()=>{});
  // 040 F-04: canceling the read ends only the read; the device stays connected.
  await api('/api/cancel',cancelingRead?{read:true}:{});
  if(!cancelingRead)connected=false;
  searching=false;searchSignature='';connectionPending=false;
  $('search-motion').hidden=true;$('discover').setAttribute('aria-busy','false');
  clearErrors();await refresh();
  feedback.textContent=cancelingRead?'Lesen abgebrochen. Lade die Gerätewerte bei Bedarf erneut.':cancelingRun || step!==1?'Prüfung beendet. Starte sie bei Bedarf erneut.':'Suche beendet. Wähle ein Gerät oder suche erneut.';
  if(cancelingConnection){feedback.textContent='';showError('pair',new Error('Verbindung abgebrochen. Versuche es erneut oder wähle ein anderes Gerät.'),()=>perform('pair',pairDevice));}
 }catch(error){showError(step===1?'pair':'run',error);}
 finally{feedback.classList.remove('cancelling');$('cancel').textContent='Prüfung abbrechen';$('cancel').removeAttribute('aria-busy');cancelInProgress=false;pending=false;controls();}
}
$('cancel').addEventListener('click',cancel);$('connect-cancel').addEventListener('click',cancel);
$('preparation-cancel').addEventListener('click',cancel);
// R043-06: a reconnect started in step 2 belongs to step 2 (navigation during it returns there).
// It survives a reload while the reconnect still runs.
let connectOrigin=(()=>{try{return Number(sessionStorage.getItem('stromea-connect-origin-043'))===2?2:1;}catch{return 1;}})();
function setConnectOrigin(value){connectOrigin=value;try{sessionStorage.setItem('stromea-connect-origin-043',String(value));}catch{}}
function operationStep(){if(connectionPending || busy && state?.kind==='connect')return connectOrigin; // R043-06: a pending connect request counts before a stale backend kind
 return state?.kind==='prepare' || requestKind==='/api/prepare'?2:state?.kind==='run'?3:state?.kind==='connect' && connectOrigin===2?2:1;}
$('operation-pill').addEventListener('click',async()=>{if(operationStep()===1 && !searching)pairingView=true;history.pushState({},'',(location.protocol==='file:'?'#':'')+'/?step='+operationStep());await StromeaRoute.restore();});
function skiComplete(){return /^[0-9a-f]{40}$/i.test($('ski').value.replace(/\s/g,''));}
function skiCount(){const n=$('ski').value.replace(/\s/g,'').length;$('ski-count').textContent=n+' / 40';$('ski-count').classList.toggle('over',n>40);}
// 039 (QA-09/10, A-01): precise, short messages in a reserved line; nothing moves when they appear.
function skiMessage(problem){return problem.kind==='invalid'?StromeaI18n.t('Ungültig: {chars} · nur 0–9, A–F').replace('{chars}',problem.chars.split(' ').length>3?problem.chars.split(' ').slice(0,3).join(' ')+' …':problem.chars):problem.kind==='long'?StromeaI18n.t('{count} statt 40 Zeichen').replace('{count}',String(problem.count)):StromeaI18n.t('40 Zeichen nötig');}
function skiError(problem){$('ski-error').hidden=false;$('ski-error').textContent=skiMessage(problem);$('ski').setAttribute('aria-invalid','true');}
function addressMessage(value){
 const port=value.match(/^(.+):(-?\d+)$/);
 if(port && (+port[2]<1 || +port[2]>65535))return 'Port 1–65535 eingeben';
 if(/^\d{1,3}(?:\.\d{1,3}){3}$|^\[[0-9a-f:]+(?:%[^\]]+)?\]$/i.test(value))return 'Port fehlt, z. B. :4711';
 return port?'Lokale IP-Adresse eingeben':'IP-Adresse mit Port eingeben';
}
function validateManual(){
 let valid=true;
 const address=$('address').value.trim(),problem=StromeaInput.skiProblem($('ski').value);
 for(const [id,ok,text] of [
  ['address',validManualAddress(address),()=>addressMessage(address)],
  ['ski',!problem,()=>skiMessage(problem)]
 ]){
  $(id+'-error').hidden=ok;$(id+'-error').textContent=ok?'':text();$(id).setAttribute('aria-invalid',String(!ok));
  if(!ok && valid){$(id).focus();valid=false;}
 }
 return valid;
}
function validManualAddress(value){
 const v4=value.match(/^(\d{1,3}(?:\.\d{1,3}){3}):(\d{1,5})$/);
 if(v4){const a=v4[1].split('.').map(Number);return +v4[2]>=1 && +v4[2]<=65535 && a.every(n=>n<=255) && (a[0]===10 || a[0]===127 || a[0]===172 && a[1]>=16 && a[1]<=31 || a[0]===192 && a[1]===168 || a[0]===169 && a[1]===254);}
 const v6=value.match(/^\[([0-9a-f:]+)(?:%([^%\]\s]+))?\]:(\d{1,5})$/i);
 if(!v6 || +v6[3]<1 || +v6[3]>65535)return false;
 try{new URL('http://'+value.replace(/%[^\]]+/,''));return v6[1]==='::1' || /^(f[cd]|fe[89ab])/i.test(v6[1]);}catch{return false;}
}
async function manualConnect(){if(pending || !validateManual())return;const d={name:'',address:$('address').value,ski:$('ski').value.replace(/\s/g,'').toLowerCase(),interface:$('manual-interface').value,manual:true};await chooseDevice(d);if(deviceKey(selected)!==deviceKey(d))return;workspace.closeQR();if(state?.connected && backendSelectionMatches(state)){controls();return;}perform('pair',pairDevice);}
$('manual-connect').addEventListener('click',manualConnect);
$('manual-device').addEventListener('keydown',e=>{if(e.key==='Enter' && !e.isComposing && e.target.matches('input,textarea')){e.preventDefault();manualConnect();}});
function refresh(initial=false){
 if(refreshPromise)return initial?refreshPromise.then(()=>refresh(true)):refreshPromise;
 refreshPromise=readState(initial).finally(()=>{refreshPromise=null;});return refreshPromise;
}
async function readState(initial = false) {
  refreshing = true;
  try {
    const s = await api('/api/state'); const previousState=state;state = s; busy = s.busy; deviceInfo=s.deviceInfo || s.report?.device || null;
 if(real && !initial){const same=backendSelectionMatches(s);connected=same && !!s.connected && !s.controlUnresponsive;paired=same && !!s.paired;}
 if(initial && s.search?.active)searching=true;
 if(searching && s.search?.active){renderDiscovered(s.search.devices || []);$('search-motion').setAttribute('aria-label',StromeaI18n.t('EEBUS-Geräte werden gesucht…')+' '+Math.max(0,Math.ceil((Date.parse(s.search.deadline)-Date.now())/1000))+' s');}
renderConnectionTimings(s.connectionTimings || []);renderDeviceType();
if(s.version)$('software-version').textContent=s.version;workspace.refreshGuide(); renderPairTrust();
    if (localSki !== s.localSki || !$('local-qr').firstChild) {
 localSki = s.localSki; renderSKIField($('local-ski'),localSki);$('local-ending').textContent=formatSki(localSki).split(' ').pop(); renderOwnQR();
 }
    if (initial) {
      resetPower();
      real = s.busy || s.report && /(?:[?&]step=3)(?:&|$)/.test(location.protocol==='file:'?location.hash:location.search) ? s.deviceMode : rememberedMode() ? rememberedMode()==='onsite' : s.deviceMode || !s.paired; paired = s.paired; connected = s.connected;
      modeView();if(!real && paired)$('paired-status').replaceChildren(drawnCheck(),document.createTextNode('Simulator erfolgreich gekoppelt.'));try{const saved=sessionStorage.getItem('stromea-scope-'+(real?'onsite-'+(s.device?.ski||''):'simulator'));if(['both','consumption','production',''].includes(saved)){$('scope').value=saved;scopeTouched=true;}}catch{}
      setCapabilities(!real && paired?{lpc:true,lpp:true,mpc:true}:s.capabilities);
      if (real) {
       let draft;try{draft=JSON.parse(sessionStorage.getItem('stromea-device-choice-036')||'null');}catch{}
       if(draft?.real && /^[0-9a-f]{40}$/i.test((draft.device?.ski||'').replace(/\s/g,''))){devices=Array.isArray(draft.devices)?draft.devices:[];showDevice(draft.device,true);pairingView=!!draft.pairingView;}
       else if(s.deviceMode && s.paired)showDevice(s.device,true);
       paired = s.deviceMode && s.paired && backendSelectionMatches(s); connected = paired && s.connected; await loadInterfaces();
      }
      if(s.busy && s.kind==='connect')autoPair=true;
 if (s.busy && s.kind === 'run') goStep(3, false);
      else if (!real && paired) goStep(2, false);
      else goStep(1, false);
    }
    if(real && s.capabilities && JSON.stringify(capabilityState)!==JSON.stringify(s.capabilities))setCapabilities(s.capabilities);
    if (s.kind === 'connect' && backendSelectionMatches(s)) {
      // R043-01: a selection that was connected before (capabilities kept by the backend) stays prepared while
      // reconnecting and after a failed attempt; only a first connection starts unpaired.
      const kept=!!s.paired && !!s.capabilities;
      if (busy) {connected=false;paired=kept;setConnection('connecting'); $('pair-status').textContent = ''; }
      else if (s.error) {connected=false;paired=kept;
        $('pair-status').textContent = '';
        // 043: a reconnect started in step 2 reports its failure there (field area), not in hidden step 1.
        if (lastError !== s.error) { if(step===2)showError('config', s.error);else showError('pair', s.error, () => perform('pair', pairDevice)); lastError = s.error; }
      } else if (s.connected) {
        $('pair-status').textContent='';
        paired = true; connected = true;
        const fresh=$('connection-motion').dataset.phase !== 'connected';
        if (fresh) $('paired-status').replaceChildren(drawnCheck(),document.createTextNode((real?'Gerät erfolgreich gekoppelt.':'Simulator erfolgreich gekoppelt.')));
        setConnection('connected'); // Capabilities are updated above only when changed; polling must preserve form choices.
 deviceErrors.delete(deviceKey(currentDevice()));
        // 043 (Nutzer 2026-11-14, Bild 5): no second click. A connection started here is confirmed visibly
        // („✓ Gekoppelt“ where the button was), then step 2 opens by itself.
        if(fresh && autoPair && real)pairedAdvance();
        autoPair=false;
      }
    }
    if((!s.connected || !backendSelectionMatches(s)) && real && !(busy && s.kind==='connect')){clearTimeout(autoAdvanceTimer);autoAdvanceTimer=0;connected=false;setConnection('idle');}
 if (s.kind !== 'connect' && !pending) {connectionPending=false; /* R043-06: a stale poll never resets a connect request still in flight */setConnection(connected?'connected':'idle');}
    if(searching && requestKind!=='/api/discover-lan' && !s.search?.active){searching=false;$('search-motion').hidden=true;}
    if (s.kind === 'run' || s.report) {
      // 043: only a running test shows the live view; a (re)connect keeps the previous result as it is.
      const runBusy=busy && s.kind==='run';
      if(runBusy)for(const id of ['outcome','downloads','result-matrix','open-evidence','site-edit'])$(id).hidden=true; // 043: a reconnect from step 2 keeps the previous result
      $('live').hidden = !runBusy; $('repeat').hidden = runBusy;
      const steps = s.report?.steps || s.progress?.steps || [];
      const current = runBusy ? s.progress?.current || '' : '';
      const total = s.report?.steps?.length || (real ? Math.max(32,steps.length+(runBusy?1:0)) : 16);
      completed = Math.max(completed, steps.length);
      if(runBusy)resultView.placeLive();
      $('title-3').textContent=runBusy?(s.progress?.lowLoad?'Leistung für die Prüfung zu niedrig':'Prüfung läuft'):'Prüfergebnis';
      renderPower(s);renderLowLoad(s);
      $('progress-fill').style.transform = 'scaleX('+Math.min(1,completed/total)+')';
      $('progress').max = total; $('progress').value = Math.min(total, completed);
      const status = s.progress?.lowLoad?'Leistung für die Prüfung zu niedrig':real ? 'Prüfung läuft…' : 'Prüfung läuft… Prüfschritt ' + Math.min(total, completed + 1) + ' von ' + total;
      // Avoid repeatedly announcing identical polling snapshots.
      const statusText = /Abbruch/.test(s.operation || '') ? 'Wird abgebrochen …' : status;
 $('run-status').classList.toggle('cancelling',/Abbruch/.test(s.operation || '') && runBusy);
 if(!runBusy){$('cancel').textContent='Prüfung abbrechen';$('cancel').removeAttribute('aria-busy');}
      if ($('run-status').textContent !== statusText) $('run-status').textContent = statusText;
      $('current-step').textContent = stepName(current);
 resultView.waiting(s.progress,runBusy);
 $('reconnect').hidden=runBusy || !(s.controlUnresponsive || s.report?.controlUnresponsive);$('control-warning').hidden=!(s.controlUnresponsive || s.report?.controlUnresponsive);
      renderSteps([...steps,...(s.progress?.events?.length?s.progress.events:s.report?.events || [])].sort((a,b)=>Date.parse(a.at)-Date.parse(b.at)), current, s.progress);
      if (s.report && !runBusy) renderResult(s);
      if (s.error && lastError !== s.error) { showError('run', s.error); lastError = s.error; }
    }
    controls();
  } finally { refreshing = false; }
}
$('profile-full').addEventListener('change', updateDuration);$('profile-quick').addEventListener('change', updateDuration);
const workspace=StromeaWorkspace.create();
setupMotion();setupTooltips();setupDictation();StromeaI18n.start();
window.addEventListener('stromea-language',()=>{renderDeviceCount();renderDeviceType();stopDictation();const speech=globalThis.SpeechRecognition || globalThis.webkitSpeechRecognition;$('dictate').hidden=!speech || StromeaI18n.language()!=='de';$('system-dictation').hidden=!!speech && StromeaI18n.language()==='de';resultView.invalidatePower();renderedSteps='';resultView.invalidateSteps();if(state){resultView.waiting(state.progress,!!state.busy);renderPower(state);renderLowLoad(state);if(state.report && !state.busy)renderResult(state);}renderPairTrust();updateDuration();controls(); });
// 041 (Nutzer 2026-11-14, Bild 1→2): the start view is decided synchronously from the remembered mode; the
// content stays invisible (space kept, no layout jump) until the backend state has been applied once.
real=rememberedMode()?rememberedMode()==='onsite':true;modeView();
const booted=()=>document.documentElement.classList.remove('booting');setTimeout(booted,4000);
refresh(true).then(()=>StromeaRoute.restore()).catch(error => showError('pair', error, () => refresh(true))).finally(booted);
setInterval(() => { if (quitting) return; if (!pending || searching) refresh().catch(error => showError(step === 1 ? 'pair' : 'run', error)); }, 500);
// 039: Sobald ein Netzwerk verfügbar ist, werden die Kopplungsaktionen ohne Neuladen freigegeben.
// 039 R039-02: also detect the loss of an existing network (live), not only its return.
setInterval(() => { if (!quitting && real && step === 1 && !busy && !pending && !document.hidden) loadInterfaces().catch(() => {}); }, 3000);
function offlineNow(){return real && !!$('interface').options.length && !$('interface').value && !$('interface').options[0].hasAttribute('data-loading');} // 041: no "Kein Netzwerk" before the first adapter list

function renderOwnQR() {
  const qr = $('local-qr'); qr.replaceChildren();
  const data = StromeaQR.encode(StromeaInput.ownShip(localSki, state?.localShipID), 'raw', {ecc: 'medium', border: 4, scale: 1});
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', '0 0 ' + data.length + ' ' + data.length);
  svg.setAttribute('width', '256'); svg.setAttribute('height', '256');
  const background = document.createElementNS(svg.namespaceURI, 'rect');
  background.setAttribute('width', '100%'); background.setAttribute('height', '100%'); background.setAttribute('fill', '#fff'); svg.append(background);
  const path = document.createElementNS(svg.namespaceURI, 'path');
  let d = '';
  data.forEach((row, y) => row.forEach((on, x) => { if (on) d += 'M' + x + ' ' + y + 'h1v1h-1z'; }));
  path.setAttribute('d', d); path.setAttribute('fill', '#000'); svg.append(path); qr.append(svg);$('panel-local-qr').replaceChildren(svg.cloneNode(true));renderSKIField($('panel-local-ski'),localSki);
}
$('panel-copy-ski').addEventListener('click',async()=>{await copySki();$('panel-copy-status').textContent=$('copy-status').textContent;});
$('panel-copy-qr').addEventListener('click',()=>$('copy-qr').click());
$('copy-qr').addEventListener('click', async () => {
  try { await navigator.clipboard.writeText(StromeaInput.ownShip(localSki, state?.localShipID)); $('copy-status').textContent = '✓ QR-Text kopiert.'; copyFeedback(); }
  catch { $('copy-status').textContent = 'Kopieren nicht möglich. Nutze den QR-Code oder markiere die Gerätekennung.'; }
  $('panel-copy-status').textContent=$('copy-status').textContent;
});
// 039 (QA-01/13): one fixed status slot below the actions; feedback never moves a button.
function qrFeedback(text,error=false){
 const status=$('qr-status');status.textContent=text;status.classList.toggle('qr-error',error);status.setAttribute('role',error?'alert':'status');
}
function applyQR(text) {
  const d = StromeaInput.ship(text);
  const match = devices.find(item => item.ski?.replace(/[\s:-]/g, '').toLowerCase() === d.ski);
  showDevice({...match,...d, address: match?.address || '', name: [d.brand, d.model].filter(Boolean).join(' · ') || '', manual: !match?.discovered});
  if(!match)workspace.show('manual',$('qr-toggle'));
  fromQR=true;renderPairTrust();
  qrFeedback('Gerätekennung übernommen. ' + (match ? 'Adresse aus der Gerätesuche ergänzt.' : 'Geräte-Adresse ergänzen.'));
}
$('qr-apply').addEventListener('click', () => {
  if (busy || pending) return;
  try { applyQR($('qr-text').value); } catch (error) { qrFeedback(error.message,true); }
});
let cameraStream = null, cameraEpoch = 0;
function stopCamera() {
  cameraEpoch++;
  cameraStream?.getTracks().forEach(track => track.stop()); cameraStream = null;
  $('qr-video').srcObject = null; $('qr-video').hidden = true; $('qr-stop').hidden = true; $('qr-camera').hidden = false;
}
// 039 (QA-08): own short message per browser error; the raw (often English) browser text stays out of the UI.
function cameraMessage(error){
  const name=error?.name || '';
  if(name==='NotAllowedError' || name==='SecurityError')return 'Kamerazugriff nicht erlaubt. Bild oder QR-Text verwenden.';
  if(name==='NotFoundError' || name==='OverconstrainedError' || name==='DevicesNotFoundError')return 'Keine Kamera gefunden. Bild oder QR-Text verwenden.';
  if(name==='NotReadableError' || name==='AbortError')return 'Kamera wird bereits verwendet. Bild oder QR-Text verwenden.';
  return 'Kamera nicht verfügbar. Bild oder QR-Text verwenden.';
}
$('qr-stop').addEventListener('click', stopCamera);
window.addEventListener('pagehide', stopCamera);
document.addEventListener('visibilitychange', () => { if (document.hidden) stopCamera(); });
$('qr-camera').addEventListener('click', async () => {
  if (busy || pending) return;
  stopCamera();
  const epoch = cameraEpoch;
  $('qr-camera').hidden=true;$('qr-stop').hidden=false;qrFeedback('Kamera wird angefordert…');
  try {
    if (!navigator.mediaDevices?.getUserMedia) throw new DOMException('', 'NotSupportedError');
    const stream = await navigator.mediaDevices.getUserMedia({video: {facingMode: 'environment'}, audio: false});
    if (epoch !== cameraEpoch) { stream.getTracks().forEach(track => track.stop()); return; }
    cameraStream = stream;
    const video = $('qr-video'); video.srcObject = cameraStream; video.hidden = false; $('qr-camera').hidden = true; $('qr-stop').hidden = false;
    await video.play();
    let detector = null;
    try {
      if (globalThis.BarcodeDetector && (await BarcodeDetector.getSupportedFormats()).includes('qr_code')) detector = new BarcodeDetector({formats: ['qr_code']});
    } catch { /* The bundled MIT decoder also works without the browser API. */ }
    const canvas = document.createElement('canvas'), ctx = canvas.getContext('2d', {willReadFrequently: true});
    qrFeedback('Halte den Geräte-QR-Code vor die Kamera.');
    const scan = async () => {
      if (epoch !== cameraEpoch || !cameraStream) return;
      let text = '';
      if (detector) { try { text = (await detector.detect(video))[0]?.rawValue || ''; } catch { detector = null; } }
      if (!text && video.videoWidth) {
        const ratio = Math.min(1, 960 / video.videoWidth);
        canvas.width = Math.round(video.videoWidth * ratio); canvas.height = Math.round(video.videoHeight * ratio);
        ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
        try { text = StromeaQR.decode(ctx.getImageData(0, 0, canvas.width, canvas.height)); } catch { /* No readable QR in this frame. */ }
      }
      if (epoch !== cameraEpoch) return;
      if (text) {
        try { applyQR(text); stopCamera(); return; }
        catch (error) { qrFeedback(error.message,true); }
      }
      setTimeout(() => { scan().catch(() => { stopCamera(); qrFeedback('Kamera nicht lesbar. Bild oder QR-Text verwenden.',true); }); }, 200);
    };
    await scan();
  } catch (error) { if(epoch!==cameraEpoch)return;stopCamera(); qrFeedback(cameraMessage(error),true); }
});
// 041: the action lives on the profile page; the status is shown there.
window.StromeaIdentityReset=() => {
  if (busy || pending || !window.confirm(StromeaI18n.t('Simulator-Kennung erneuern? Die bisherige Kopplung wird ungültig. Gib die neue Simulator-Kennung am Gerät frei.'))) return;
  perform('pair', async () => {
    stopCamera();
 await api('/api/identity/reset', {confirmed: true});
    real = false; paired = false; connected = false; selected = null;
    try{localStorage.setItem('stromea-mode','simulator');}catch{}
    state=null;deviceInfo=null;capabilityState=null;devices=[];deviceErrors.clear();savedDevices.clear();pairingView=false;selectionOverview=false;
    $('ski').value='';$('address').value='';
    fromQR=false; $('device').hidden = true;
    if($('identity-status'))$('identity-status').textContent = 'Simulator-Kennung erneuert. Verbinde das Gerät erneut und gib die neue Kennung dort frei.';
    modeView(); await refresh(true); if(!document.body.classList.contains('local-page-open'))goStep(1);
  });
};

function setupTooltips() { workspace.bindHelp(); }
function toggleQR(open) {
  if(open){workspace.show('qr',$('qr-toggle'));}else{workspace.closeQR();}
}

$('qr-toggle').addEventListener('click',()=>toggleQR($('qr-panel').hidden));
$('qr-close').addEventListener('click',()=>toggleQR(false));
async function readQRImage(file) {
  if(busy || pending)return;
  qrFeedback('Bild wird lokal gelesen…');
  const url=URL.createObjectURL(file);
  try{
    const img=new Image();img.src=url;await img.decode();
    const ratio=Math.min(1,1600/Math.max(img.naturalWidth,img.naturalHeight));
    const canvas=document.createElement('canvas');canvas.width=Math.max(1,Math.round(img.naturalWidth*ratio));canvas.height=Math.max(1,Math.round(img.naturalHeight*ratio));
    const ctx=canvas.getContext('2d',{willReadFrequently:true});ctx.drawImage(img,0,0,canvas.width,canvas.height);
    const text=StromeaQR.decode(ctx.getImageData(0,0,canvas.width,canvas.height));
    if(!text)throw new Error('Kein lesbarer QR-Code. Anderes Bild oder QR-Text verwenden.');
    if(!busy && !pending)applyQR(text);
  }catch(error){qrFeedback('Bild nicht lesbar. Kein lesbarer QR-Code. Anderes Bild oder QR-Text verwenden.',true);}
  finally{URL.revokeObjectURL(url);$('qr-image').value='';}
}
$('qr-image-select').addEventListener('click',()=>{$('qr-image').click();});
$('qr-paste').addEventListener('click',async()=>{
 if(busy || pending)return;
 try{if(navigator.clipboard?.read){for(const item of await navigator.clipboard.read()){const type=item.types.find(t=>t==='image/png'||t==='image/jpeg');if(type){await readQRImage(await item.getType(type));return;}if(item.types.includes('text/plain')){applyQR(await (await item.getType('text/plain')).text());return;}}throw Error('Zwischenablage enthält kein Bild und keinen SHIP-Text.');}else{applyQR(await navigator.clipboard.readText());}}catch{ qrFeedback('Zwischenablage nicht lesbar. QR-Text einfügen oder Bild auswählen.',true);}
});
$('qr-image').addEventListener('change',e=>{if(e.target.files[0])readQRImage(e.target.files[0]);});
document.addEventListener('paste',e=>{
  if($('qr-panel').hidden || busy || pending)return;
  const text=e.clipboardData?.getData?.('text/plain');if(text?.trim().startsWith('SHIP;')){e.preventDefault();try{applyQR(text);}catch(error){qrFeedback(error.message,true);}return;}
  const image=[...(e.clipboardData?.items || [])].find(item=>item.type.startsWith('image/'));
  if(image){e.preventDefault();readQRImage(image.getAsFile());}
});
function renderLowLoad(s) {
 const low=s.progress?.lowLoad;$('low-load').hidden=!s.busy || !low;
 if(!low)return;
 // 043 (Nutzer 2026-11-14): one card – measured vs. required per direction, two options, timer in the header.
 const watt=n=>StromeaI18n.t('{watts} W').replace('{watts}',n.toLocaleString(StromeaI18n.locale()));
 const rows=(low.directions || [low]).map(v=>{const feed=v.direction==='LPP',known=Number.isFinite(v.watts) && Number.isFinite(v.required);return {name:StromeaI18n.t(feed?'Einspeisung (LPP)':'Bezug (LPC)'),measured:known?watt(Math.max(0,feed?-v.watts:v.watts)):'–',required:known?StromeaI18n.t('über {watts}').replace('{watts}',watt(v.required)):StromeaI18n.t('zu niedrig')};});
 const signature=JSON.stringify([rows,low.directions || low,StromeaI18n.language()]);
 if($('low-load-text').dataset.signature!==signature){
  $('low-load-text').dataset.signature=signature;
  const table=document.createElement('table');table.className='decision-table';table.dataset.i18nRendered='';
  const head=document.createElement('tr');for(const text of ['Prüfrichtung','Gemessen','Nötig']){const th=document.createElement('th');th.scope='col';th.textContent=StromeaI18n.t(text);head.append(th);}
  const thead=document.createElement('thead'),tbody=document.createElement('tbody');thead.append(head);table.append(thead,tbody);
  for(const row of rows){const tr=document.createElement('tr'),th=document.createElement('th'),measured=document.createElement('td'),required=document.createElement('td');th.scope='row';th.textContent=row.name;measured.className='decision-low';measured.textContent=row.measured;required.textContent=row.required;tr.append(th,measured,required);tbody.append(tr);}
  // The signed reading explains a 0 W row (e.g. feed-in required while the device draws power).
  const readings=[...new Set((low.directions || [low]).filter(v=>Number.isFinite(v.watts) && (v.direction==='LPP')!==(v.watts<0) && v.watts!==0).map(v=>StromeaI18n.t('Aktuell: {direction} {watts} W.').replace('{direction}',StromeaI18n.t(v.watts<0?'Einspeisung':'Bezug')).replace('{watts}',Math.abs(v.watts).toLocaleString(StromeaI18n.locale()))))];
  const reading=document.createElement('p');reading.className='help low-load-reading';reading.dataset.i18nRendered='';reading.textContent=readings.join(' ');reading.hidden=!readings.length;
  $('low-load-text').replaceChildren(table,reading);
  // G043-01 (UI-Review Gemini): the advice fits the direction – load for import, generation for feed-in.
  const kinds=new Set((low.directions || [low]).map(v=>v.direction==='LPP'?'LPP':'LPC'));
  const advice=kinds.size>1?'Leistung anpassen: für Bezug Last einschalten, für Einspeisung Erzeugung erhöhen oder Verbraucher ausschalten. Dann erneut messen.':kinds.has('LPP')?'Einspeisung erhöhen, z. B. Erzeugung abwarten oder Verbraucher ausschalten, dann erneut messen.':'Last einschalten, z. B. Fahrzeug laden oder Heizung anfordern, dann erneut messen.';
  const copy=document.querySelector('#remeasure + .option-copy');if(copy)copy.textContent=advice;
 }
 $('current-step').textContent='Leistung für die Prüfung zu niedrig';
}
let pairSuccessTimer=0;
function pairedAdvance(){
 clearTimeout(pairSuccessTimer);$('pair-success').hidden=false;enter($('pair-success'));controls();
 pairSuccessTimer=setTimeout(()=>{pairSuccessTimer=0;$('pair-success').hidden=true;if(step===1 && connected && !busy && !document.body.classList.contains('local-page-open')){goStep(2);motion($('paired-status'),[{opacity:0,transform:'scale(.96)'},{opacity:1,transform:'scale(1)'}],360);}else controls();},900);
}
async function reconnectDevice(){const epoch=operationEpoch;autoPair=true;connectionPending=true;controls();await api('/api/reconnect',{});if(epoch!==operationEpoch)return;connected=false;paired=false;goStep(1);await refresh();}
$('reconnect').addEventListener('click',()=>perform('run',reconnectDevice));
// 043 (Nutzer 2026-11-14): „Prüfung starten“ is never disabled without a visible reason. After a lost connection
// the primary action becomes „Neu verbinden“ in the same place; step, values and choices stay.
function renderStartGate(){
 const reason=$('start-reason');if(!reason)return;
 const connecting=real && busy && state?.kind==='connect';
 const unresponsive=real && !busy && !!state?.controlUnresponsive;
 const lost=real && !busy && !unresponsive && prepared() && !connected;
 const reconnect=lost || unresponsive || connecting && step===2;
 $('run').hidden=reconnect;$('run-reconnect').hidden=!reconnect;
 $('run-reconnect').disabled=connecting || pending || cancelInProgress;$('run-reconnect').setAttribute('aria-busy',String(connecting));
 $('run-reconnect').textContent=connecting?'Verbinde …':unresponsive?'Verbindung neu aufbauen':'Neu verbinden';
 let text='';
   if(connecting)text=$('connection-label').textContent || 'Verbindung wird aufgebaut';
 else if(unresponsive)text='Gerät antwortet nicht auf Steuerbefehle. Baue die Verbindung neu auf.';
 else if(lost)text='Verbindung zum Gerät getrennt. Deine Eingaben bleiben erhalten.';
 else if($('run').disabled){
  text=cancelInProgress?'Wird abgebrochen …':state?.kind==='prepare' && busy || requestKind==='/api/prepare'?'Gerätewerte werden gelesen …':pending?'':real && connected && !capabilityState?'Gerätefunktionen werden gelesen …':real && capabilityState && !capabilityState.lpc && !capabilityState.lpp?'Das Gerät meldet keine Steuerfunktion. Eine Prüfung ist nicht möglich.':busy?'Eine andere Aktion läuft.':'Wähle zuerst ein Gerät.';
 }
 reason.hidden=!text;$('duration').hidden=!!text;if(reason.dataset.source!==text){reason.dataset.source=text;reason.textContent=text;}
 reason.classList.toggle('warning',lost || unresponsive);$('run').setAttribute('aria-busy',String(pending && !text && $('run').disabled));
 // The status line at the top tells the same state once: confirmed connection or its loss.
 const status=$('paired-status');
 if(real && (lost || unresponsive || connecting && step===2)){if(status.dataset.state!=='lost'){status.dataset.state='lost';status.classList.remove('success');status.classList.add('warning');status.textContent=connecting?'Verbindung wird aufgebaut':'Verbindung zum Gerät getrennt';}}
 else if(status.dataset.state==='lost'){status.dataset.state='';status.classList.remove('warning');status.classList.add('success');status.replaceChildren(drawnCheck(),document.createTextNode(real?'Gerät erfolgreich gekoppelt.':'Simulator erfolgreich gekoppelt.'));}
}
async function reconnectInPlace(){
 const epoch=operationEpoch,unresponsive=!!state?.controlUnresponsive;setConnectOrigin(2);connectionPending=true;controls();
 await api(unresponsive?'/api/reconnect':'/api/connect',unresponsive?{}:{timeout:Number($('step-timeout').value)*1e9});
 if(epoch!==operationEpoch)return;busy=true;state={...state,kind:'connect',busy:true};await refresh();
}
$('run-reconnect').addEventListener('click',()=>perform('config',reconnectInPlace));
// 043: Prüfer/in fehlt → persistenter Dialog; „Speichern & weiter“ speichert im Profil und startet sofort.
let inspectorDialog=null;
function openInspectorDialog(){
 inspectorDialog=inspectorDialog || StromeaDialogs.create($('inspector-dialog'),{defaultButton:()=>$('inspector-dialog-save'),onCancel:closeInspectorDialog});
 $('inspector-dialog-error').textContent='';$('inspector-dialog-name').removeAttribute('aria-invalid');inspectorDialog.open();$('inspector-dialog-name').focus();
}
function closeInspectorDialog(){inspectorDialog?.close();$('run').focus();}
$('inspector-dialog-cancel').addEventListener('click',closeInspectorDialog);
$('inspector-dialog-save').addEventListener('click',()=>{
 const name=$('inspector-dialog-name').value.trim();
 if(!name){$('inspector-dialog-error').textContent=StromeaI18n.t('Bitte gib deinen Namen ein.');$('inspector-dialog-name').setAttribute('aria-invalid','true');$('inspector-dialog-name').focus();return;}
 if(!StromeaProfile.update({inspector:name})){$('inspector-dialog-error').textContent=StromeaI18n.t('Profil konnte nicht lokal gespeichert werden.');return;} // R043-05
 inspectorDialog.close();$('form').requestSubmit($('run'));
});
for(const [id,retry] of [['remeasure',true],['continue-low',false]]) $(id).addEventListener('click',async()=>{
  if(pending)return;pending=true;controls();
  // 043: if the run ended meanwhile (e.g. connection lost), its result explains why; no extra error card.
  try{await api('/api/low-load',{retry});await refresh();}catch(error){await refresh().catch(()=>{});if(state?.progress?.lowLoad)showError('run',error);}finally{pending=false;controls();}
});
function setupDictation() {
  const Speech=globalThis.SpeechRecognition || globalThis.webkitSpeechRecognition;
  $('dictate').hidden=!Speech || StromeaI18n.language()!=='de';$('system-dictation').hidden=!!Speech && StromeaI18n.language()==='de';
  if(!Speech)return;
  let recognition=null,consented=false,suggestion=null,blockStart=0;
  try{consented=sessionStorage.getItem('stromea-speech-consent')==='yes';}catch{}
  // 039 (Nutzer 5, QA-04): SF-Symbol-artige Linienicons in currentColor statt Emoji.
  const micIcon=$('dictate').innerHTML,stopIcon='<svg class="dictate-icon" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true" focusable="false"><rect x="7" y="7" width="10" height="10" rx="2" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>';
  const icon=recording=>{$('dictate').innerHTML=recording?stopIcon:micIcon;};
  const stop=()=>{recognition?.abort();recognition=null;icon(false);};
  stopDictation=stop;
  window.addEventListener('pagehide',stop);document.addEventListener('visibilitychange',()=>{if(document.hidden)stop();});
  for(const id of ['nav-1','nav-2','nav-3','pair','pair-simulator','mode-simulator','mode-onsite'])$(id).addEventListener('click',stop);
  const begin=repeat=>{
    if(busy || pending || StromeaI18n.language()!=='de')return;
    if(recognition){stop();return;}
    if(!consented){if(!window.confirm($('dictate').dataset.tip+' Aufnahme für diese Sitzung erlauben?'))return;consented=true;try{sessionStorage.setItem('stromea-speech-consent','yes');}catch{}}
    if(!repeat)blockStart=$('ski').value.replace(/\s/g,'').length;
    if(blockStart>36){$('dictation-error').textContent='Die Gerätekennung ist vollständig. Lösche zuerst einen Block.';$('dictation-review').hidden=false;return;}
    suggestion=null;$('dictation-apply').disabled=true;$('dictation-review').hidden=false;$('dictation-error').textContent='Vier Zeichen sprechen. Aufnahme mit Mikrofonknopf stoppen.';
    const rec=new Speech();recognition=rec;rec.lang='de-DE';rec.continuous=false;rec.interimResults=false;icon(true);
    rec.onresult=e=>{
      const transcript=e.results[0][0].transcript,parsed=StromeaInput.dictation(transcript);suggestion=parsed;
      $('dictation-transcript').replaceChildren();
      for(const token of parsed.tokens){const span=document.createElement(token.value===null?'mark':'span');span.textContent=token.raw+' ';$('dictation-transcript').append(span);}
      $('dictation-hex').textContent=parsed.hex || '–';
      $('dictation-error').textContent=parsed.unknown.length?'Unbekannte Wörter markiert. Block wiederholen.':parsed.hex.length!==4?'Bitte genau vier Zeichen sprechen.':'Vier Zeichen prüfen und übernehmen.';
      $('dictation-apply').disabled=parsed.unknown.length>0 || parsed.hex.length!==4;
    };
    rec.onerror=e=>{$('dictation-error').textContent='Spracherkennung: '+e.error+'. Eingabe oder Systemdiktat verwenden.';stop();};
    rec.onend=()=>{if(recognition===rec){recognition=null;icon(false);}};
    try{rec.start();}catch(error){$('dictation-error').textContent=error.message;stop();}
  };
  $('dictate').addEventListener('click',()=>begin(false));$('dictation-repeat').addEventListener('click',()=>begin(true));
  $('dictation-apply').addEventListener('click',()=>{
    if(!suggestion || suggestion.hex.length!==4 || suggestion.unknown.length || busy || pending)return;
    const compact=$('ski').value.replace(/\s/g,'');$('ski').value=compact.slice(0,blockStart)+suggestion.hex+compact.slice(blockStart+4);$('ski').dispatchEvent(new Event('input',{bubbles:true}));
    $('dictation-error').textContent='Der Block wurde übernommen. Vergleiche die vollständige Kennung mit dem Gerät.';$('dictation-apply').disabled=true;
  });
}

window.addEventListener('resize',()=>{if(state && step===3)renderPower(state);});

$('copy-device-ski').addEventListener('click',async()=>{try{await navigator.clipboard.writeText($('ski').value.replace(/\s/g,''));$('device-copy-status').textContent='Kopiert';}catch{$('device-copy-status').textContent='Kopieren fehlgeschlagen';}});

function backendSelectionMatches(s){return !real || (s?.deviceMode && s?.device?.ski?.replace(/[\s:-]/g,'').toLowerCase()===$('ski').value.replace(/[\s:-]/g,'').toLowerCase() && s?.device?.address===$('address').value);}

window.addEventListener('pageshow',e=>{if(e.persisted)StromeaRoute.reload();});

// 041 (Nutzer 2026-11-14): Dialoge „Über“ und „Simulator beenden“ (dialogs.js, gemeinsam mit /protokoll).
const appDialog=StromeaDialogs.create;StromeaDialogs.about();
StromeaDialogs.quit({isBusy:()=>!!(state?.busy && state?.kind==='run'),onStart:()=>{quitting=true;stopCamera();stopDictation();}});

// Export und Import aller Daten (Protokolle, Profil inkl. Logo, Simulator-Kennung) als ZIP-Datei.
window.StromeaTransfer=(()=>{
 let file=null,onStatus=()=>{};
 const picker=document.createElement('input');picker.type='file';picker.accept='.zip,application/zip';picker.hidden=true;document.body.append(picker);
 const dialog=appDialog($('import-dialog'),{enterAlwaysDefault:true,defaultButton:()=>$('import-cancel'),onCancel:()=>cancel()});
 const cancel=()=>{if($('import-confirm').getAttribute('aria-busy')==='true')return;file=null;dialog.close();};
 $('import-cancel').addEventListener('click',cancel);
 picker.addEventListener('change',()=>{file=picker.files[0]||null;picker.value='';if(!file)return;$('import-file-name').textContent=file.name;$('import-status').textContent='';for(const id of ['import-reports','import-profile','import-identity'])$(id).checked=true;$('import-confirm').disabled=false;dialog.open();});
 for(const id of ['import-reports','import-profile','import-identity'])$(id).addEventListener('change',()=>{$('import-confirm').disabled=!['import-reports','import-profile','import-identity'].some(x=>$(x).checked);});
 $('import-confirm').addEventListener('click',async()=>{
  if(!file || busy || pending)return;const button=$('import-confirm');button.disabled=true;button.setAttribute('aria-busy','true');$('import-status').textContent=StromeaI18n.t('Import läuft …');onStatus('');
  const query=new URLSearchParams();for(const [id,key] of [['import-reports','reports'],['import-profile','profile'],['import-identity','identity']])if($(id).checked)query.set(key,'1');
  try{
   const response=await fetch('/api/import?'+query,{method:'POST',headers:{'Content-Type':'application/zip','X-Stromea-Token':token},body:file});
   if(!response.ok)throw Error((await response.text()).trim() || 'import');
   const res=await response.json(),parts=[];
   // Browser storage and server files have no shared transaction. Report acknowledged server changes honestly.
   const profileStored=!res.profile || StromeaProfile.replace(res.profile);
   const refreshIdentity=async()=>{if(!res.identity)return;paired=false;connected=false;try{await refresh(true);}catch{/* Confirmed import stays confirmed; polling/reload refreshes the view. */}};
   if(query.has('reports'))parts.push(StromeaI18n.t('Protokolle: {imported} hinzugefügt, {skipped} bereits vorhanden.').replace('{imported}',res.imported).replace('{skipped}',res.skipped));
   if(res.conflicts)parts.push(StromeaI18n.t('{conflicts} abweichende Berichtsversionen nicht übernommen.').replace('{conflicts}',res.conflicts));
   if(query.has('profile') && profileStored)parts.push(StromeaI18n.t(res.profile?'Profil übernommen.':'Kein Profil in der Exportdatei.'));
   if(query.has('identity'))parts.push(StromeaI18n.t(res.identity?'Simulator-Kennung übernommen. Koppele Geräte bei Bedarf neu.':'Keine Simulator-Kennung in der Exportdatei.'));
   if(!profileStored){
    const message=StromeaI18n.t('Import nicht vollständig. {details} Profil konnte nicht lokal gespeichert werden. Dein bisheriges Profil und dein Entwurf bleiben erhalten. Versuche den Import erneut; nur das Profil ist noch ausgewählt.').replace('{details}',parts.join(' ')).replace(/\s+/g,' ').trim();
    $('import-status').textContent=message;onStatus(message);
    $('import-reports').checked=false;$('import-identity').checked=false;$('import-profile').checked=true;button.disabled=false;
    await refreshIdentity();return;
   }
   file=null;button.removeAttribute('aria-busy');dialog.close();onStatus(parts.join(' '));
   await refreshIdentity();
  }catch(error){$('import-status').textContent=StromeaI18n.t('Import nicht möglich: {reason}').replace('{reason}',StromeaI18n.t(error.message));button.disabled=false;}
  finally{button.removeAttribute('aria-busy');}
 });
 async function exportFile(status=()=>{}){
  try{
   const response=await fetch('/api/export',{method:'POST',headers:{'Content-Type':'application/json','X-Stromea-Token':token},body:JSON.stringify({profile:StromeaProfile.snapshot()})});
   if(!response.ok)throw Error('export');
   const name=/filename="([^"]+)"/.exec(response.headers.get('Content-Disposition')||'')?.[1] || 'stromea-simulator-export.zip';
   const url=URL.createObjectURL(await response.blob()),link=document.createElement('a');link.href=url;link.download=name;document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
   status(StromeaI18n.t('Exportdatei erzeugt.'));
  }catch{status(StromeaI18n.t('Exportdatei konnte nicht erzeugt werden.'));}
 }
 return {exportFile,chooseImport(status=()=>{}){onStatus=status;if(busy || pending){status(StromeaI18n.t('Während einer Prüfung oder Verbindung ist kein Import möglich.'));return;}picker.click();}};
})();
