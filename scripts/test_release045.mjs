#!/usr/bin/env node
// Synthetic browser acceptance: isolated loopback servers, no discovery outside loopback.
import assert from 'node:assert/strict';
import {spawn,execFileSync} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {createHash} from 'node:crypto';
import {mkdtemp,mkdir,readFile,writeFile,rm,readdir} from 'node:fs/promises';
import {join,resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
const out=resolve(process.env.RELEASE045_OUTPUT||'artifacts/045');await mkdir(out,{recursive:true});
const temp=await mkdtemp(join(out,'runtime-')),evidence={synthetic:true,layout:[],failures:[],matrix:[],invalid:[]};
const {chromium}=await import(pathToFileURL(process.env.E2E_PLAYWRIGHT||join(process.env.HOME,'.cache/stromea-pw/node_modules/playwright-core/index.mjs')));
const browser=await chromium.launch({headless:true,args:['--disable-background-networking']});
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
async function until(fn,label){for(let i=0;i<1200;i++){const v=await fn();if(v)return v;await sleep(100);}throw Error(label);}
const digest=b=>createHash('sha256').update(b).digest('hex');
async function instance(name){
 const socket=createServer();socket.listen(0,'127.0.0.1');await once(socket,'listening');const port=socket.address().port;await new Promise(r=>socket.close(r));
 const stateDir=join(temp,name),base='http://127.0.0.1:'+port;
 const server=spawn(resolve(process.env.E2E_BINARY||'artifacts/036/check-server'),['serve','--state',stateDir,'--listen','127.0.0.1:'+port,'--loopback-only','--loopback-partner'],{stdio:['pipe','ignore','ignore']});
 const context=await browser.newContext({viewport:{width:1280,height:1000},locale:'de-DE',reducedMotion:'reduce'});
 await context.route('**/*',r=>(new URL(r.request().url()).origin===base||/^(data|blob|about):/.test(r.request().url()))?r.continue():r.abort());
 const page=await context.newPage();page.setDefaultTimeout(12000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
 const state=async()=>{const r=await context.request.get(base+'/api/state');return r.json();};
 await until(async()=>{try{return (await fetch(base+'/api/state')).ok;}catch{return false;}},'server');
 const visit=async path=>{await page.goto(base+path);await page.waitForFunction(()=>typeof state!=='undefined' && state!==null);};
 const stop=async()=>{await context.close();server.stdin.end('quit\n');if(server.exitCode===null){server.kill('SIGTERM');await once(server,'exit');}assert.deepEqual(errors,[]);};
 return {page,base,state,stateDir,visit,stop,context};
}
async function layout(a,name){
 for(const width of [1280,1790]){
  await a.page.setViewportSize({width,height:1000});
  const boxes=await a.page.evaluate(()=>{const box=s=>{const n=document.querySelector(s),r=n.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top+scrollY,visible:n.checkVisibility()};};return {main:box('.workspace-main'),aside:box('#detail-inspector')};});
  evidence.layout.push({name,width,...boxes});
  try{assert(boxes.aside.visible,`${name} ${width}: details visible`);
  assert(boxes.aside.left>=boxes.main.right-2,`${name} ${width}: details right: ${JSON.stringify(boxes)}`);
  assert(Math.abs(boxes.aside.top-boxes.main.top)<=2,`${name} ${width}: same top`);}catch(e){evidence.failures.push(e.message);}
 }
}
async function openImport(a,path){
 const chooser=a.page.waitForEvent('filechooser');await a.page.locator('#transfer-import').click();await (await chooser).setFiles(path);await a.page.locator('#import-dialog[open]').waitFor();
}
async function snapshot(a){
 const s=await a.state();return {profile:await a.page.evaluate(()=>StromeaProfile.snapshot()),ski:s.localSki,ship:s.localShipID,reports:(await (await a.context.request.get(a.base+'/api/reports')).json()).map(r=>r.id).sort(),files:await disk(a.stateDir)};
}
async function disk(dir){const entries=[];async function walk(d){for(const e of await readdir(d,{withFileTypes:true})){const p=join(d,e.name);if(e.isDirectory())await walk(p);else entries.push([p.slice(dir.length),digest(await readFile(p))]);}}await walk(dir);return entries.sort((a,b)=>a[0].localeCompare(b[0]));}
let a;
try{
 a=await instance('A');await a.page.route('**/api/interfaces',r=>r.fulfill({json:[{name:'mock-ethernet',kind:'ethernet',address:'192.0.2.1'},{name:'loopback',address:'127.0.0.1',loopbackOnly:true}]}));await a.visit('/?step=1');await a.page.locator('#mode-onsite').check();
 // Discovery is mocked; the peer itself uses real loopback SHIP.
 const peer=(await a.state()).device;
 await a.page.route('**/api/discover-lan',async r=>{await sleep(500);await r.fulfill({json:[{...peer,interface:'loopback'}]});});
 await layout(a,'search-idle');await a.page.locator('#manual-toggle').click();await layout(a,'manual');await a.page.evaluate(()=>workspace.closeQR());await a.page.locator('#qr-toggle').click();await layout(a,'qr');await a.page.evaluate(()=>workspace.closeQR());await a.page.locator('#discover').click();await a.page.locator('#search-motion').waitFor();await layout(a,'search-running');
 await a.page.locator('.device-choice').first().waitFor();await layout(a,'search-results');
 await a.page.route('**/api/discover-lan',r=>r.fulfill({json:[]}),{times:1});await a.page.locator('#discover').click();await a.page.locator('#search-status').waitFor();await layout(a,'search-empty');
 await a.page.route('**/api/discover-lan',r=>r.fulfill({status:503,body:'synthetic discovery failure'}),{times:1});await a.page.locator('#discover').click();await a.page.waitForFunction(()=>document.getElementById('search-status').classList.contains('error'));await layout(a,'search-error');
 await a.page.locator('#discover').click();await a.page.locator('#search-motion').waitFor({state:'hidden'});await a.page.locator('.device-choice').first().click();await a.page.locator('#compare-block').waitFor();await layout(a,'compare-connect');
 // Deterministic rendering fixture for transient connection/approval states (not a hardware claim).
 await a.page.evaluate(()=>{window.saved045={state,busy};busy=true;state={...state,busy:true,kind:'connect',connectionState:'connecting',reconnection:{active:true,attempt:1,deadline:new Date(Date.now()+60000).toISOString()}};setConnection('connecting');controls();});
 await layout(a,'connecting-approval-fixture');
 for(const width of [1280,1790,390])for(const language of ['de','en','pl','ru','uk','tr','ro']){
  await a.page.setViewportSize({width,height:1000});await a.page.evaluate(l=>StromeaI18n.setLanguage(l),language);
  const {gap,before,after}=await a.page.evaluate(()=>{state={...state,kind:'connect'};const slot=document.querySelector('#step-1 .pair-primary'),box=()=>{const r=slot.getBoundingClientRect();return [r.top+scrollY,r.height];};busy=false;setConnection('idle');controls();const before=box();busy=true;controls();setConnection('connecting');StromeaI18n.walk(slot);const after=box();if(!document.getElementById('connection-motion').checkVisibility())throw Error('connection fixture must be visible');return {before,after,gap:document.getElementById('connection-motion').getBoundingClientRect().top-document.getElementById('pair-phases').getBoundingClientRect().bottom};});
  assert.deepEqual(after,before,'connection slot reserves space at '+width+' '+language);
  if(gap<12)evidence.failures.push(`connection status gap ${width}: ${gap}`);evidence.layout.push({name:'status-gap',width,language,gap});
 }
 await a.page.evaluate(()=>StromeaI18n.setLanguage('de'));
 await a.page.evaluate(()=>{({state,busy}=window.saved045);delete window.saved045;setConnection('idle');controls();});
 for(const [name,message] of [['connection-cancelled-fixture','context canceled'],['approval-refused-fixture','trust rejected'],['connection-timeout-fixture','deadline exceeded']]){
  await a.page.evaluate(message=>showError('pair',new Error(message)),message);await layout(a,name);
  await a.page.evaluate(()=>{clearErrors();deviceErrors.clear();controls();});
 }
 if(process.env.RELEASE045_LAYOUT_ONLY==='1')console.log('layout-only measurements complete');
 else{
 await a.page.route('**/api/connect',async r=>{await sleep(1500);await r.continue();},{times:1});
 await a.page.locator('#pair').click();await a.page.locator('#connection-operation').waitFor();await layout(a,'connecting-real-loopback');
 await a.page.locator('#step-2').waitFor({timeout:20000});await layout(a,'connected-step-2');
 await a.page.locator('#nav-1').click();await layout(a,'connected-overview');
 await a.visit('/profil');await a.page.locator('#profile-business').fill('Synthetischer Betrieb 045');await a.page.locator('#profile-inspector').fill('Prüferin 045');for(const [id,value] of Object.entries({street:'Musterweg 45',postalCode:'12345',city:'Musterstadt',country:'Deutschland'}))await a.page.locator('#profile-'+id).fill(value);
 const logo=await a.page.evaluate(()=>{const c=document.createElement('canvas');c.width=c.height=32;const g=c.getContext('2d');g.fillStyle='#4C7A1E';g.fillRect(8,8,16,16);return c.toDataURL('image/png').split(',')[1];});
 await a.page.locator('#profile-logo').setInputFiles({name:'synthetic.png',mimeType:'image/png',buffer:Buffer.from(logo,'base64')});await a.page.locator('#profile-preview').waitFor();await a.page.locator('.profile-form button[type=submit]').click();
 await a.visit('/?step=1');if(await a.page.locator('#change-device').isVisible())await a.page.locator('#change-device').click();await a.page.locator('#mode-simulator').check();await a.page.locator('#pair-simulator').click();await a.page.locator('#step-2').waitFor();await layout(a,'step-2');
 const pdfs={};
 for(let run=0;run<2;run++){
  if(run){await a.page.locator('#repeat').click();await a.page.locator('#step-2').waitFor();}
  await a.page.locator('#run').click();await a.page.locator('#live').waitFor();await layout(a,'running-'+run);
  const s=await until(async()=>{const s=await a.state();return !s.busy && s.report?s:false;},'practice report');assert(s.report.passed);assert.equal(s.report.mode,'simulator');
  await a.page.locator('#downloads').waitFor();await layout(a,'result-'+run);
  const response=await a.context.request.get(a.base+'/api/report.pdf?id='+s.report.id);assert.equal(response.status(),200);pdfs[s.report.id]=digest(await response.body());
 }
 const source=await snapshot(a);assert(source.profile.logo);await a.visit('/hilfe');
 const downloading=a.page.waitForEvent('download');await a.page.locator('#transfer-export').click();const zip=join(temp,'export.zip');await (await downloading).saveAs(zip);
 evidence.pdfSHA256=pdfs;
 for(let mask=0;mask<8;mask++){
  const b=await instance('B-'+mask);
  try{
   await b.visit('/hilfe');const before=await snapshot(b);assert.notEqual(before.ski,source.ski);
   await openImport(b,zip);for(const [bit,id] of [[1,'reports'],[2,'profile'],[4,'identity']])await b.page.locator('#import-'+id).setChecked(!!(mask&bit));
   if(!mask){assert(await b.page.locator('#import-confirm').isDisabled());await b.page.locator('#import-cancel').click();}
   else{await b.page.locator('#import-confirm').click();await b.page.locator('#import-dialog').waitFor({state:'hidden'});}
   const after=await snapshot(b);
   await b.page.reload();await b.page.waitForFunction(()=>typeof state!=='undefined' && !!state);
   assert.deepEqual(await snapshot(b),after,'selection persists after reload: '+mask);
   assert.deepEqual(after.reports,mask&1?source.reports:before.reports);
   assert.deepEqual(after.profile,mask&2?source.profile:before.profile);
   assert.equal(after.ski,mask&4?source.ski:before.ski);assert.equal(after.ship,mask&4?source.ship:before.ship);
   const compared={};if(mask&1)for(const [id,sha] of Object.entries(pdfs)){const r=await b.context.request.get(b.base+'/api/report.pdf?id='+id);assert.equal(r.status(),200);compared[id]=digest(await r.body());assert.equal(compared[id],sha,'PDF byte identity');}
   if(mask===0)assert.deepEqual(after,before,'empty selection changes nothing');
   // Verify the stored identity independently of the current HTTP state.
   const identity=await readFile(join(b.stateDir,'energyguard','identity.pem'));const sourceIdentity=await readFile(join(a.stateDir,'energyguard','identity.pem'));assert.equal(digest(identity)===digest(sourceIdentity),!!(mask&4));
   evidence.matrix.push({reports:!!(mask&1),profile:!!(mask&2),identity:!!(mask&4),passed:true,reloaded:true,pdfs:compared});
  }finally{await b.stop();}
 }
 // Review R045-01: real server import + a selectively failing browser storage write.
 const quota=await instance('B-quota');
 try{
  await quota.visit('/profil');await quota.page.locator('#profile-business').fill('Bisheriger Betrieb');await quota.page.locator('#profile-inspector').fill('Bisherige Prüferin');await quota.page.locator('.profile-form button[type=submit]').click();
  await quota.page.locator('#profile-business').fill('Ungespeicherter Entwurf');await quota.page.locator('.main-menu a[href="/hilfe"]').click();await quota.page.locator('#transfer-import').waitFor();
  const before=await snapshot(quota),draft=await quota.page.evaluate(()=>sessionStorage.getItem('stromea-profile-draft'));
  assert(draft && draft.includes('Ungespeicherter Entwurf'));
  await quota.page.evaluate(()=>{window.restoreStorage045=Storage.prototype.setItem;Storage.prototype.setItem=function(key,value){if(this===localStorage && key==='stromea-profile-v1')throw new DOMException('Synthetic quota failure','QuotaExceededError');return window.restoreStorage045.call(this,key,value);};});
  await openImport(quota,zip);await quota.page.locator('#import-confirm').click();
  await quota.page.waitForFunction(()=>!document.getElementById('import-confirm').hasAttribute('aria-busy'));
  const after=await snapshot(quota),ui=await quota.page.evaluate(()=>({open:document.getElementById('import-dialog').open,message:document.getElementById('import-status').textContent,helpStatus:document.getElementById('transfer-status').textContent,draft:sessionStorage.getItem('stromea-profile-draft'),reports:document.getElementById('import-reports').checked,identity:document.getElementById('import-identity').checked,profile:document.getElementById('import-profile').checked}));
  evidence.quota={serverReports:after.reports.length,identityImported:after.ski===source.ski,oldProfileRetained:JSON.stringify(after.profile)===JSON.stringify(before.profile),draftRetained:ui.draft===draft,dialogOpen:ui.open,message:ui.message,helpStatus:ui.helpStatus};
  assert.deepEqual(after.reports,source.reports,'confirmed reports really imported');assert.equal(after.ski,source.ski,'confirmed identity really imported');
  assert.deepEqual(after.profile,before.profile,'quota error must preserve in-memory profile');assert.equal(ui.draft,draft,'quota error must preserve draft');
  assert(ui.open && !ui.helpStatus.includes('Profil übernommen.'),'no false success or closed dialog');
  assert(ui.message.includes('Import nicht vollständig.') && ui.message.includes('Protokolle: 2 hinzugefügt') && ui.message.includes('Simulator-Kennung übernommen.') && ui.message.includes('Profil konnte nicht lokal gespeichert werden.'),'concrete partial result');
  assert(!ui.reports && !ui.identity && ui.profile,'retry selects only profile');
  assert.equal(await quota.page.evaluate(p=>StromeaProfile.replace(p),source.profile),false,'replace returns false');assert.deepEqual((await snapshot(quota)).profile,before.profile);assert.equal(await quota.page.evaluate(()=>sessionStorage.getItem('stromea-profile-draft')),draft);evidence.quota.replaceReturnedFalse=true;
  await quota.page.evaluate(()=>{Storage.prototype.setItem=window.restoreStorage045;delete window.restoreStorage045;});
  await quota.page.reload();await quota.page.waitForFunction(()=>typeof state!=='undefined' && !!state);assert.deepEqual(await snapshot(quota),after,'old profile and server changes survive reload');assert.equal(await quota.page.evaluate(()=>sessionStorage.getItem('stromea-profile-draft')),draft);
  // Reopen after reload and retry only the failed part. Server-imported files stay byte-identical.
  await openImport(quota,zip);await quota.page.locator('#import-reports').uncheck();await quota.page.locator('#import-identity').uncheck();await quota.page.locator('#import-confirm').click();await quota.page.locator('#import-dialog').waitFor({state:'hidden'});
  const recovered=await snapshot(quota);assert.deepEqual(recovered.profile,source.profile);assert.deepEqual(recovered.files,after.files);assert.equal(await quota.page.evaluate(()=>sessionStorage.getItem('stromea-profile-draft')),null);
  await quota.page.reload();await quota.page.waitForFunction(()=>typeof state!=='undefined' && !!state);assert.deepEqual(await snapshot(quota),recovered);evidence.quota.recoveredAfterReload=true;
 }finally{await quota.stop();}
 const b=await instance('B-invalid');
 try{
  await b.visit('/hilfe');const before=await snapshot(b);let imports=0;b.page.on('request',r=>{if(new URL(r.url()).pathname==='/api/import')imports++;});
  await openImport(b,zip);await b.page.locator('#import-cancel').click();assert.deepEqual(await snapshot(b),before);
  for(const id of ['import-cancel','import-reports','import-profile','import-identity','import-confirm']){await openImport(b,zip);await b.page.locator('#'+id).focus();await b.page.keyboard.press('Enter');assert.equal(await b.page.locator('#import-dialog[open]').count(),0,'Enter on '+id+' cancels');assert.deepEqual(await snapshot(b),before);}
  assert.equal(imports,0,'Enter and cancel must never import');evidence.cancelAndEnter=true;
  // Python standard library creates deliberately malformed archives; no extra dependency.
  execFileSync('python3',['-c',String.raw`import sys,zipfile,io,json,os,hashlib
src,dst=sys.argv[1:]
data=open(src,'rb').read()
open(os.path.join(dst,'truncated.zip'),'wb').write(data[:len(data)//2])
with zipfile.ZipFile(io.BytesIO(data)) as z: entries={n:z.read(n) for n in z.namelist()}
for label,name in [('parent','../profile.json'),('absolute','/profile.json'),('drive',r'C:\profile.json'),('backslash',r'identity\energyguard.pem'),('case','Profile.json'),('reserved','reports/CON.json'),('long','reports/'+'a'*260+'.json'),('unicode-nfc','reports/é.json'),('unicode-nfd','reports/e\u0301.json')]:
 copy=dict(entries);copy[name]=b'{}'
 manifest=json.loads(copy['manifest.json']);manifest['files'][name]=hashlib.sha256(copy[name]).hexdigest();copy['manifest.json']=json.dumps(manifest).encode()
 with zipfile.ZipFile(os.path.join(dst,label+'.zip'),'w',zipfile.ZIP_DEFLATED) as z:
  for n,v in copy.items():z.writestr(n,v)
for label in ['checksum','oversize']:
 copy=dict(entries);copy['profile.json']=b'{}' if label=='checksum' else b' '*((2<<20)+1)
 with zipfile.ZipFile(os.path.join(dst,label+'.zip'),'w',zipfile.ZIP_DEFLATED) as z:
  for n,v in copy.items():z.writestr(n,v)
`,zip,temp]);
  for(const name of (await readdir(temp)).filter(n=>n.endsWith('.zip')&&n!=='export.zip')){
   await openImport(b,join(temp,name));await b.page.locator('#import-confirm').click();await b.page.waitForFunction(()=>document.getElementById('import-status').textContent.startsWith('Import nicht möglich:'));
   const message=await b.page.locator('#import-status').innerText();assert(message.length>25);assert.deepEqual(await snapshot(b),before,name+': no partial changes');evidence.invalid.push({name,message,passed:true});await b.page.locator('#import-cancel').click();
  }
 }finally{await b.stop();}
 }
 assert.deepEqual(evidence.failures,[],'desktop geometry and status spacing');evidence.passed=true;
}catch(e){evidence.error=e.stack;process.exitCode=1;console.error(e.stack);}
finally{if(a)await a.stop();await browser.close();await writeFile(join(out,'release045.json'),JSON.stringify(evidence,null,2)+'\n');await rm(temp,{recursive:true,force:true});}
console.log(JSON.stringify({passed:evidence.passed,layout:evidence.layout.length,matrix:evidence.matrix.length,invalid:evidence.invalid.length}));
