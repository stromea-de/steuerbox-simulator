// Flow regressions: actual WebKit DOM; isolated synthetic fetch, clipboard and speech.
window.uiTest={done:false,error:'',snapshots:{},checks:0,contrasts:{}};
window.addEventListener('error',e=>{window.uiTest.error=e.message;});
(async()=>{
 const t=window.uiTest,el=id=>document.getElementById(id),m=window.mockAPI;
 const assert=(ok,msg)=>{if(!ok)throw new Error(msg);t.checks++;};
 const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));
 const settle=()=>pause(100);
 const click=async id=>{assert(!el(id).disabled,id+' enabled');el(id).click();await settle();};
 const input=(id,value)=>{el(id).value=value;el(id).dispatchEvent(new Event('input',{bubbles:true}));};
 const shot=name=>{
  assert(document.documentElement.scrollWidth<=innerWidth,name+' fits viewport');
  const clone=document.documentElement.cloneNode(true);
  clone.querySelectorAll('script').forEach(node=>node.remove());
  const inputs=document.querySelectorAll('input');clone.querySelectorAll('input').forEach((n,i)=>{n.setAttribute('value',inputs[i].value);if(inputs[i].checked)n.setAttribute('checked','');else n.removeAttribute('checked');});
  const options=document.querySelectorAll('option');clone.querySelectorAll('option').forEach((n,i)=>{if(options[i].selected)n.setAttribute('selected','');else n.removeAttribute('selected');});
  const style=document.createElement('style');style.textContent='*,*::before,*::after{animation:none!important;transition:none!important}';clone.querySelector('head').append(style);
  t.snapshots[name]=clone.outerHTML;
 };
 function ratio(a,b){const luminance=c=>c.match(/[\d.]+/g).slice(0,3).map(Number).map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4;}).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);const x=luminance(a),y=luminance(b);return(Math.max(x,y)+.05)/(Math.min(x,y)+.05);}
 const device={name:'Wallbox',address:'192.0.2.50:4711',ski:'0123456789abcdef0123456789abcdef01234567'};
 try{
  await settle();shot('uebung');
  if(el('mode-onsite').checked)await click('mode-simulator');await click('pair-simulator');assert(!el('business').required && !el('inspector').required,'practice has optional report data');
  el('form').requestSubmit();await settle();assert(m.lastConfig.business.includes('Übung'),'practice sends clear examples');
  m.state={...m.state,busy:false,report:null,kind:''};await refresh();goStep(1);
  await click('mode-onsite');assert([...el('interface').options].every(o=>o.value!=='lo0'),'loopback hidden onsite');
  assert(el('interface').textContent.includes('WLAN · 192.0.2.10'),'readable network');
  m.devices=[];await click('discover');assert(!el('search-status').hidden,'empty discovery');
  // P0: no search result, only manual address and grouped pasted identity.
  el('manual-device').open=true;input('address',device.address);input('ski','0123-4567:89ab-cdef 0123 4567 89ab cdef 0123 4567');
  assert(el('ski-count').textContent==='40 / 40' && !el('device').hidden,'manual identity confirmation reachable');
  assert(el('device-ski').children.length===10,'ten four-character blocks');
  assert(!el('ski-confirmed'),'confirmation is the Connect action');m.connectBusy=true;await click('pair');
  assert(el('pair-link').dataset.state==='waiting' && el('pair-status').textContent==='','one connection status');
  assert(!el('copy-ski').disabled && !el('copy-qr').disabled,'copy remains available during connection');shot('verbindet');
  m.state.busy=false;m.state.connected=true;m.state.capabilities={lpc:true,lpp:false,mpc:true};await refresh();
  assert(!el('step-1').hidden,'manual connection waits for next');await click('pair');assert(!el('step-2').hidden,'next advances to step two');assert(el('scope').value==='consumption' && el('lpp-field').hidden && !el('lpp').required,'consumer scope follows capabilities');
  goStep(1);await refresh();assert(!el('step-1').hidden,'refresh respects back navigation');
  await click('discover');await refresh();assert(!el('step-1').hidden,'discovery never reopens step two');shot('kopplung');
  // QR without discovery then manual address uses the same confirmation path.
  await click('qr-toggle');assert(!el('qr-panel').hidden && el('qr-toggle').getAttribute('aria-expanded')==='true','QR workspace opens');
  input('qr-text',StromeaInput.ownShip(device.ski,'wallbox'));await click('qr-apply');
  input('address',device.address);assert(!el('device').hidden && !el('ski-confirmed'),'QR confirmation remains reachable after address');shot('qr-panel');
  await click('inspector-close');assert(el('qr-panel').hidden && document.activeElement.id==='change-device','QR returns to coupling');
  assert(!el('ski-confirmed'),'confirmation is the Connect action');m.connectBusy=false;m.connectError='Verbindung abgewiesen: Vertrauen fehlt';await click('pair');
  assert(!el('pair-error').hidden && el('connection-motion').hidden && !el('error-copy-ski').hidden,'refusal has one status and direct copy');shot('abgelehnt');
  m.connectError='';await click('retry');await click('pair');assert(!el('step-2').hidden,'retry and explicit next after refusal');
  const tip=document.querySelector('#lpc-field .tip');
  assert(tip.hidden,'redundant info icon removed');el('lpc').focus();assert(el('field-help').textContent.includes('LPC'),'focus puts help in inspector');
  document.querySelector('label[for="lpc"]').click();assert(el('field-help').textContent.includes('LPC'),'label opens context');
  if(innerWidth<=1050){document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));assert(document.activeElement===el('lpc'),'Escape returns field focus');}else assert(document.activeElement===el('lpc'),'label keeps field focused');
  const check=el('sign-operator');check.focus();assert(el('field-help').textContent.includes('Werkabnahme'),'checkbox help without toggling');assert(!check.checked,'focusing help does not toggle checkbox');
  for(const tip of document.querySelectorAll('.tip'))assert(tip.getAttribute('aria-describedby') && el(tip.getAttribute('aria-describedby')),'help metadata has accessible target');
  input('business','Elektro Beispiel');input('inspector','Erika Muster');input('lpc','4.200');el('form').requestSubmit();await settle();
  assert(!el('lpc-error').hidden && el('lpc').getAttribute('aria-invalid')==='true' && !el('step-2').hidden,'ambiguous number has field error');shot('feldfehler');
  input('lpc','4200');el('scope').value='consumption';el('scope').dispatchEvent(new Event('change'));shot('einstellungen');
  el('form').requestSubmit();await settle();assert(m.lastConfig.lpp===0 && m.lastConfig.scope==='consumption','only selected scope sent');
  m.state.progress={steps:[],current:'Ausgangsmessung MPC',lowLoad:{watts:800,required:4250,direction:'LPC'},samples:[]};await refresh();
  assert(!el('low-load').hidden && el('low-load-title').textContent.includes('800') && el('low-load-text').textContent.includes('4250'),'early load warning numeric');shot('zu-wenig-last');
  // Low load action is a protected explicit POST.
  await click('remeasure');assert(m.calls.some(c=>c.path==='/api/low-load' && c.body.retry),'remeasure requested');
  const at=Date.now()-20000,samples=[];
  for(let i=0;i<=40;i++){const phase=i<8?'Ausgangsmessung MPC':i<18?'LPC gemessene Wirkung':i<29?'LPC Failsafe gemessene Wirkung':'Grenzen aufheben';let target=i<8?11000:i<18?4200:i<29?500:11000;const previous=samples.at(-1)?.watts ?? target;const watts=Math.round(previous+(target-previous)*.45+Math.sin(i)*3);samples.push({at:new Date(at+i*500).toISOString(),watts,step:phase});}
  m.state.progress={current:'LPC Failsafe gemessene Wirkung',steps:[],samples};await refresh();
  assert(el('heartbeat-label').textContent.includes('Reaktion wird geprüft'),'phase alone never claims notbetrieb confirmed');
  const steps=[{name:'LPC Failsafe gemessene Wirkung',passed:true},{name:'Grenzen aufheben',passed:true},{name:'LPC Failsafe-Leistung zurückstellen',passed:true},{name:'LPC Failsafe-Mindestdauer zurückstellen',passed:true},{name:'LPC Wiederkehr',passed:false,status:'not_checked',detail:'Langtest nicht durchgeführt'}];
  m.state={...m.state,busy:false,outcome:'Bestanden',report:{mode:'onsite',device,config:{scope:'consumption',lpc:4200,lpp:0,failsafe:500},steps,samples,started:new Date(at).toISOString(),finished:new Date().toISOString()}};await refresh();
  assert(el('outcome-title').textContent==='LPC-Funktionsprüfung bestanden','consumer passes green');
  assert(!el('release-status').hidden && el('release-limits').textContent.includes('Ja') && el('release-failsafe').textContent.includes('Ja'),'release and restore distinct');
  const ids=['outcome','release-status','downloads','open-evidence','power-monitor'];for(let i=1;i<ids.length;i++)assert(el(ids[i-1]).compareDocumentPosition(el(ids[i]))&Node.DOCUMENT_POSITION_FOLLOWING,'result order '+ids[i]);
  assert(el('detail-inspector').contains(el('all-steps')) && !el('repeat').hidden && !el('other-device').hidden,'protocol in inspector and next actions distinct');
  assert(el('power-chart').querySelectorAll('.power-point').length===41,'dense points from backend');
  assert(!el('power-chart').querySelector('.reference-line.lpp'),'irrelevant feed-in line hidden');assert(el('power-chart').textContent.includes('4200 W §14a') && el('power-chart').textContent.includes('500 W Notbetrieb'),'reference labels direct');
  assert(el('power-value').textContent===wattsText(samples.at(-1).watts),'value changes without tween');assert(el('power-age').textContent.includes('vor '),'measurement age');assert(el('power-table').rows.length===5,'mobile has value table');shot('ergebnis');
  const bg=getComputedStyle(el('power-monitor')).backgroundColor;
  for(const id of ['power-status','power-reference','power-age']){const value=ratio(getComputedStyle(el(id)).color,bg);assert(value>=4.5,id+' contrast');t.contrasts[id]=value;}
  const buttonStyle=getComputedStyle(el('run'));t.contrasts.button=ratio(buttonStyle.color,buttonStyle.backgroundColor);assert(t.contrasts.button>=4.5,'primary contrast');
  // Null and long gaps must split the path; signs remain distinct.
  const signed=[{at:new Date(at).toISOString(),watts:4200,step:'LPC gemessene Wirkung'},{at:new Date(at+1000).toISOString(),watts:null,step:'LPC gemessene Wirkung'},{at:new Date(at+2000).toISOString(),watts:-3000,step:'LPP gemessene Wirkung'},{at:new Date(at+10000).toISOString(),watts:-500,step:'LPP Failsafe gemessene Wirkung'}];
  renderPower({report:{samples:signed,steps:[],config:{scope:'both',lpc:4200,lpp:3000,failsafe:500}},busy:false});
  assert((el('power-chart').querySelector('.power-trace').getAttribute('d').match(/M/g)||[]).length===3,'failed reads and long gaps unconnected');assert(el('power-direction').textContent==='Einspeisung','negative direction');shot('messluecken');
  // Decode a real local QR bitmap through the file-import and paste handlers.
  goStep(1);toggleQR(true);const raw=StromeaQR.encode(StromeaInput.ownShip(device.ski,'wallbox'),'raw',{ecc:'medium',border:4,scale:5});const bitmap=new StromeaQR.Bitmap({width:raw[0].length,height:raw.length});bitmap.data=raw;
  const blob=new Blob([bitmap.toGIF()],{type:'image/gif'});await readQRImage(blob);assert(el('qr-status').textContent.includes('übernommen'),'local QR photo decoded');
  toggleQR(true);const paste=new Event('paste',{bubbles:true,cancelable:true});Object.defineProperty(paste,'clipboardData',{value:{items:[{type:'image/png',getAsFile:()=>blob}]}});document.dispatchEvent(paste);for(let i=0;i<100 && !el('qr-status').textContent.includes('übernommen');i++)await pause(20);assert(el('qr-status').textContent.includes('übernommen'),'image paste finished');assert(paste.defaultPrevented,'image paste captured only in QR workspace');
  toggleQR(false);
  // Optional browser dictation: per-session consent and review before adoption.
  let consents=0,recognizer;window.confirm=()=>{consents++;return true;};
  try{sessionStorage.removeItem('stromea-speech-consent');}catch{}
  globalThis.SpeechRecognition=class {constructor(){recognizer=this;}start(){}abort(){this.onend?.();}};
  setupDictation();input('ski','');await click('dictate');assert(consents===1,'speech consent before recording');
  recognizer.onresult({results:[[{transcript:'Anton Apfel Ze De'}]]});assert(el('dictation-transcript').querySelector('mark') && el('dictation-apply').disabled,'unknown speech words retained and marked');
  recognizer.onend();await click('dictation-repeat');assert(consents===1,'speech consent once per session');
  recognizer.onresult({results:[[{transcript:'Anton Berta Cäsar Dora'}]]});recognizer.onend();await click('dictation-apply');assert(el('ski').value==='ABCD' && !el('ski-confirmed'),'reviewed speech block adopted without trust: '+el('ski').value+' / '+el('dictation-error').textContent);
  await click('dictation-repeat');recognizer.onresult({results:[[{transcript:'null eins zwei drei'}]]});recognizer.onend();await click('dictation-apply');assert(el('ski').value==='0123','repeat replaces last block');
  t.done=true;
 }catch(error){t.error=String(error)+'\n'+(error.stack||'');t.done=true;}
})();
