'use strict';
// Context workspace: edits retain their DOM nodes and values when the panel closes.
window.StromeaWorkspace={create(){
 const $=id=>document.getElementById(id),small=matchMedia('(max-width:1050px)');
 let mode='context',opener=null,opened=false,fieldCopy='',step=1,activeTip=null;
 const grid=document.createElement('div');grid.className='workspace';
 const main=document.createElement('div');main.className='workspace-main';$('step-1').before(grid);grid.append(main);
 for(const id of ['step-1','step-2','step-3'])main.append($(id));
 main.prepend(document.querySelector('.wizard').closest('nav'));
 const inspector=document.createElement('aside');inspector.id='detail-inspector';inspector.setAttribute('aria-labelledby','inspector-title');
 inspector.innerHTML='<div class="inspector-content"><div class="inspector-heading"><h2 id="inspector-title">Details</h2><button type="button" id="inspector-close" class="text-button">Schließen</button></div><section id="context-panel"><h3 id="context-title">Gerät verbinden</h3><p id="context-copy"></p><section id="field-context"><p id="field-help"></p></section><section id="diagnostics"><h3>Verbindungsdetails</h3><dl id="connection-context"></dl></section></section><div id="editor-tools" hidden></div><section id="protocol-context" hidden></section></div>';
 grid.append(inspector);
 const identity=document.createElement('section');identity.id='inspector-identity';identity.innerHTML='<h3>Simulator-Kennung</h3><div class="copy-row"><div id="inspector-ski" class="ski-field mono" data-user-content="true"></div><button id="inspector-copy-ski" type="button" class="copy-icon" aria-label="Simulator-Kennung kopieren" title="Simulator-Kennung kopieren">⧉</button></div><p id="inspector-copy-status" class="copy-status" role="status"></p>'; /* 041: Kopiersymbol rechts wie beim Koppeln */$('diagnostics').before(identity);
 $('inspector-copy-ski').addEventListener('click',async()=>{await copySki();$('inspector-copy-status').textContent=$('copy-status').textContent;});


 const backdrop=document.createElement('div');backdrop.id='inspector-backdrop';backdrop.hidden=true;grid.before(backdrop);
 const trigger=document.createElement('button');trigger.id='details-toggle';trigger.className='text-button';trigger.textContent='Details';trigger.setAttribute('aria-controls','detail-inspector');trigger.setAttribute('aria-expanded','false');document.querySelector('.wizard').closest('nav').append(trigger);
 $('pair-error-anchor').after($('pair-error'));
 const tools=$('editor-tools');tools.append($('manual-device'),$('qr-panel'));
 // One diagnosis location, with critical messages and correction actions remaining in the active step.
 for(const id of ['pair-error-detail','config-error-detail','run-error-detail']){
  const node=$(id),old=node.closest('details');$('diagnostics').append(node);if(old)old.remove();
 }
 const own=$('own-qr'),ownSection=document.createElement('section');ownSection.id='simulator-qr';
 own.querySelector('summary').remove();while(own.firstChild)ownSection.append(own.firstChild);own.remove();
 // The panel already renders the same simulator identity; keep the source QR offscreen for generation.
 ownSection.hidden=true;tools.append(ownSection);
 const tabs=document.createElement('div');tabs.className='qr-tabs';tabs.setAttribute('aria-label','QR-Auswahl');
 tabs.innerHTML='<button type="button" id="qr-device-tab" class="text-button" aria-pressed="true">Geräte-QR</button><button type="button" id="qr-simulator-tab" class="text-button" aria-pressed="false">Simulator-QR</button>';
 $('qr-close').hidden=true;
 const qr=$('qr-panel'),simQR=$('panel-local-qr').parentElement;simQR.id='qr-simulator-section';simQR.hidden=true; /* 041: QR-Text über das Kopiersymbol neben dem QR-Code (#panel-copy-qr) */qr.insertBefore(tabs,$('qr-import'));
 for(const [id,sim] of [['qr-device-tab',false],['qr-simulator-tab',true]])$(id).addEventListener('click',()=>{ $('qr-import').hidden=sim;simQR.hidden=!sim;$('qr-device-tab').setAttribute('aria-pressed',String(!sim));$('qr-simulator-tab').setAttribute('aria-pressed',String(sim));if(sim)stopCamera(); });
 const minimum=document.createElement('p');minimum.id='long-test-minimum';minimum.className='notice warning';minimum.hidden=true;$('profile-consequence').after(minimum);
 const guide=StromeaGuide.mount($('field-context'),{apply(id,value){$(id).value=Number(value).toLocaleString(StromeaI18n.locale(),{maximumFractionDigits:3,useGrouping:true});$(id).dispatchEvent(new Event('input',{bubbles:true}));},refresh:async()=>{let read=false;const epoch=operationEpoch;await perform('config',async()=>{const p=await api('/api/prepare',{});if(epoch!==operationEpoch)return;state={...state,preparation:p};read=true;});if(!read)throw new Error('read failed');},getData:()=>({real,busy:busy||pending,preparation:state?.preparation})});
 let activeField='';
 function guideFor(field){if(['business','inspector'].includes(field?.id))return;activeField=field?.id || '';guide.show(activeField.startsWith('profile-')?'profile':activeField);}
 const duration=$('duration');duration.classList.remove('notice');const action=document.createElement('div');action.className='start-action';$('run').before(action);action.append($('run'),$('run-reconnect'),duration,$('start-reason')); // 043: reason and duration share one slot
 $('open-calculator').addEventListener('click',()=>{guideFor($('lpc'));fieldCopy='';show('context',$('open-calculator'));});
 const durationHelp=$('duration-help');$('field-context').append(durationHelp);durationHelp.classList.add('text-button');durationHelp.hidden=true;durationHelp.textContent='Wie setzt sich die Prüfdauer zusammen?';
 // Logs and raw measurements have a stable place beside the running task.
 $('protocol-context').append($('all-steps'));
 function context(){
 $('inspector-identity').hidden=step!==2;
  $('duration-help').hidden=step!==2;
  $('connection-context').hidden=step!==1;
  const hasError=['pair-error-detail','config-error-detail','run-error-detail'].some(id=>$(id).textContent.trim());
  $('diagnostics').hidden=mode!=='context' || (step===3 && !hasError);
  if($('capability-detail'))$('capability-detail').hidden=step===3;
  $('diagnostics').querySelector('h3').textContent=step===1?'Verbindungsdetails':step===2?'Gerätefunktionen':'Technische Details';
  $('context-title').textContent=['','Gerät verbinden','Prüfung vorbereiten','Prüfergebnis'][step];
  $('context-copy').textContent=step===1 && $('step-1').classList.contains('selection-overview')?'Wähle ein Gerät oder fahre mit dem verbundenen Gerät fort.':step===1?'Vergleiche die vollständigen Gerätekennungen. Vertrauen wird für beide Seiten getrennt bestätigt.':step===2?'Prüfe die Leistungsgrenzen und starte die Prüfung.':'Messwerte stammen aus dem aktuellen Prüflauf. Fehlende Werte und offene Nachweise bleiben sichtbar.';
  $('field-context').hidden=step!==2;if(step===2)$('context-copy').hidden=true;
  $('field-help').textContent=fieldCopy;if(!fieldCopy)$('field-help').hidden=true;
 }
 function modal(on){
  opened=on && small.matches;inspector.classList.toggle('panel-open',opened);backdrop.hidden=!opened;
  inspector.setAttribute('role',opened?'dialog':'complementary');
  if(opened)inspector.setAttribute('aria-modal','true');else inspector.removeAttribute('aria-modal');
  // Prevent keyboard focus moving behind a modal panel, including the application header.
  for(const node of document.querySelector('main').children){if(node!==grid && node!==backdrop)node.inert=opened;}
  main.inert=opened;document.body.classList.toggle('panel-open',opened);trigger.setAttribute('aria-expanded',String(opened));
 }
 function display(){
  $('context-panel').hidden=mode!=='context';$('editor-tools').hidden=mode==='context';
  $('manual-device').hidden=mode!=='manual';$('qr-panel').hidden=mode!=='qr';
  $('protocol-context').hidden=mode!=='context' || step!==3;
  $('inspector-close').style.visibility=mode!=='context' || small.matches?'visible':'hidden';
  $('inspector-close').textContent=mode==='manual'?'Abbrechen':'Schließen';
  $('inspector-title').textContent=mode==='manual'?'Manuell eingeben':mode==='qr'?'QR-Kopplung':step===3?'Messdetails':'Details';context();
 }
 // 039 (Nutzer 6): Ein offenes Werkzeug (QR/Manuell) überlebt native Browserdialoge, die die Seite
 // verdecken, aus dem bfcache zurückkehren oder neu laden lassen (iOS/Android-Dateiauswahl, Kamera).
 const panelKey='stromea-panel-039',remember=value=>{try{if(value)sessionStorage.setItem(panelKey,value);else sessionStorage.removeItem(panelKey);}catch{}};
 let refocused=0;window.addEventListener('focus',()=>{refocused=performance.now();});
 function show(next='context',source=trigger){if(next!==mode){stopCamera();stopDictation();}mode=next;remember(['qr','manual'].includes(next)?next:'');$('qr-toggle').setAttribute('aria-expanded',String(next==='qr'));opener=source;display();modal(true);if(small.matches)$('inspector-close').focus();}
 function close(){remember('');stopCamera();stopDictation();modal(false);mode='context';$('qr-toggle').setAttribute('aria-expanded','false');display();if(opener?.isConnected && opener.getClientRects().length)opener.focus();else if($('change-device').getClientRects().length)$('change-device').focus();else if(trigger.getClientRects().length)trigger.focus();else $('title-'+step).focus();}
 $('inspector-close').addEventListener('click',close);trigger.addEventListener('click',()=>show('context',trigger));backdrop.addEventListener('click',()=>{if(performance.now()-refocused<600)return;close();}); // no ghost tap from a closing native dialog
 document.addEventListener('keydown',e=>{
  if(e.key==='Escape' && (opened || mode!=='context' || (opener && opener!==trigger))){e.preventDefault();close();return;}
  if(e.key==='Tab' && opened){const all=[...inspector.querySelectorAll('button,input,textarea,select,a[href],[tabindex="0"]')].filter(n=>!n.disabled && n.getClientRects().length);const first=all[0],last=all.at(-1);if(e.shiftKey && document.activeElement===first){e.preventDefault();last.focus();}else if(!e.shiftKey && document.activeElement===last){e.preventDefault();first.focus();}}
 });
 small.addEventListener('change',()=>{const wasOpen=opened;modal(false);if(wasOpen || mode!=='context')modal(true);});
   function update({localSki,address,name,connectionState,connectionAt,adapter}){
 if(localSki && $('inspector-ski').dataset.ski!==localSki){renderSKIField($('inspector-ski'),localSki);$('inspector-ski').dataset.ski=localSki;}
  guide.update();
  const dl=$('connection-context');dl.replaceChildren();
  const option=$('interface').selectedOptions[0];
     // 043: with a device selected, the adapter of that device (search result) is shown, not the dropdown value.
   for(const [title,value,user] of (adapter?[['Netzwerkadapter',adapter,true]]:[['Netzwerk',option?.textContent,true],['Netzwerkadapter',$('interface').value,true]]).concat([['IP-Adresse und Port des Geräts',address,true],['Verbindung',StromeaI18n.t(({connected:'Verbunden',reconnecting:'Verbinde neu',disconnected:'Getrennt'})[connectionState] || 'Getrennt')+(connectionAt?' · '+new Date(connectionAt).toLocaleTimeString(StromeaI18n.locale()):''),false]])){
   if(!value)continue;const dt=document.createElement('dt'),dd=document.createElement('dd');dt.textContent=title;dd.textContent=value;if(user)dd.dataset.userContent='true';if(title.includes('SKI'))dd.className='context-ski';dl.append(dt,dd);
  }
  context();
  const selected=$('step-1').classList.contains('has-selection');$('selected-device-summary').hidden=!selected;
  $('selected-device-copy').toggleAttribute('data-user-content',!!name);
  $('selected-device-copy').textContent=name || 'Gerät';
 }
 function bindHelp(){
  for(const label of document.querySelectorAll('#step-2 label'))label.addEventListener('click',()=>{const f=label.querySelector('input') || $(label.htmlFor);if(f)guideFor(f);});
  for(const button of document.querySelectorAll('[data-tip]')){
   button.removeAttribute('aria-describedby');if(button.closest('#step-2') && button.classList.contains('tip'))button.hidden=true;
   const help=()=>{activeTip=button;const heading=button.closest('.field-heading');const label=heading?.querySelector('label');guideFor(label?$(label.htmlFor) || label.querySelector('input'):button);fieldCopy=(label?StromeaI18n.sourceText(label)+' · ':'')+StromeaI18n.sourceAttribute(button,'data-tip');context();};
   const label=button.closest('.field-heading')?.querySelector('label');if(label)label.addEventListener('click',()=>{help();if(small.matches)show('context',label.querySelector('input') || document.getElementById(label.htmlFor) || trigger);});
   button.addEventListener('focus',help);button.addEventListener('click',()=>{help();show('context',button);});
   const heading=button.closest('.field-heading');const field=heading?.querySelector('input,select') || (heading?.nextElementSibling?.matches('input,select')?heading.nextElementSibling:heading?.nextElementSibling?.querySelector('input,select'));const desc=document.createElement('span');desc.id='field-description-'+(field?.id || button.id || document.querySelectorAll('.field-description').length);desc.className='field-description';
   desc.textContent=button.id==='duration-help'?'Die Prüfdauer ergibt sich aus den gewählten Wartezeiten. Eine erfolgreiche Prüfung kann früher enden.':StromeaI18n.sourceAttribute(button,'data-tip');button.after(desc);button.setAttribute('aria-describedby',desc.id);
   if(field){const unit=$('unit-'+field.id);if(unit?.textContent.trim()==='W')desc.textContent+=' Eingabe in Watt.';field.dataset.description=desc.id;field.setAttribute('aria-describedby',field.dataset.description);field.addEventListener('focus',()=>{help();guideFor(field);});}
  }
  for(const label of document.querySelectorAll('#step-2 label[for]')){const field=$(label.htmlFor);if(field && !label.closest('.field-heading'))label.addEventListener('click',()=>{field.focus();if(small.matches)show('context',field);});}
  for(const field of document.querySelectorAll('input,textarea,select'))field.addEventListener('focus',()=>{
   if(['language','business','inspector'].includes(field.id) || field.closest('#guide-panel'))return;
   guideFor(field);
   const tip=field.closest('.field-heading')?.querySelector('[data-tip]') || document.querySelector('label[for="'+field.id+'"]')?.closest('.field-heading')?.querySelector('[data-tip]');
   if(tip){const label=document.querySelector('label[for="'+field.id+'"]') || tip.closest('.field-heading')?.querySelector('label');fieldCopy=(label?StromeaI18n.sourceText(label)+' · ':'')+StromeaI18n.sourceAttribute(tip,'data-tip');}else{const label=document.querySelector('label[for="'+field.id+'"]');const help=field.getAttribute('aria-describedby')?.split(' ').map(id=>StromeaI18n.sourceText($(id))).join(' ');fieldCopy=StromeaI18n.sourceText(label)+(help?' · '+help:'');}
   context();
  });
 }
 window.addEventListener('stromea-language',()=>queueMicrotask(()=>{
  if(step!==2 || !activeTip)return;
  const label=activeTip.closest('.field-heading')?.querySelector('label');
  fieldCopy=(label?StromeaI18n.sourceText(label)+' · ':'')+activeTip.getAttribute('data-tip');context();
 }));
 display();modal(false);
 function resume(){let saved='';try{saved=sessionStorage.getItem(panelKey)||'';}catch{}const toggle=$(saved==='qr'?'qr-toggle':'manual-toggle');if(step===1 && mode==='context' && ['qr','manual'].includes(saved) && !$('lan-controls').hidden && !toggle.disabled)show(saved,toggle);}
 return {refreshGuide:()=>guide.update(),resetProduction:()=>guide.resetProduction(),update,show,resume,closeQR:close,step(next){step=next;fieldCopy='';activeTip=null;guide.show('');if(next!==1)remember('');if(mode!=='context'){stopCamera();stopDictation();modal(false);mode='context';}display();},bindHelp};
}};
