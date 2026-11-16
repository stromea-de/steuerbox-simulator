#!/usr/bin/env node
// 043 (Nutzer 2026-11-14, Echtgerät): der Nutzerweg als Browserprüfung gegen den isolierten Loopback-Testpartner.
// Verbinden → Profil → zurück → Verbindung bricht ab → Neu verbinden → Start ohne Prüfer → Dialog → läuft →
// zu wenig Leistung (eine Karte) → Verbindung bricht ab → Ursache oben → Neu verbinden. Kein LAN, keine Fremdgeräte.
// Maßstab: kein Layoutsprung, kein Raten, kein Datenverlust (UX-G16, UX-G17, UX-G18, UX-S43…UX-S48).
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {mkdir,mkdtemp,rm,writeFile} from 'node:fs/promises';
import {resolve,dirname,join} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {auditDOM} from './audit034.mjs';
import {visibleCopy036c} from './copy036c.mjs';

const root=resolve(dirname(fileURLToPath(import.meta.url)),'..');
const output=resolve(root,process.env.UX043_OUTPUT||'artifacts/043');await mkdir(join(output,'screens'),{recursive:true});
const binary=resolve(root,process.env.E2E_BINARY||'artifacts/036/check-server');
const langs=['de','en','pl','ru','uk','tr','ro'],results=[],wait=ms=>new Promise(r=>setTimeout(r,ms));
async function until(fn,label,ms=15000){const end=Date.now()+ms;for(;;){const v=await fn();if(v)return v;if(Date.now()>end)throw Error('Timeout: '+label);await wait(80);}}
async function freePort(){const s=createServer();s.listen(0,'127.0.0.1');await once(s,'listening');const p=s.address().port;await new Promise(r=>s.close(r));return p;}
const pw=process.env.E2E_PLAYWRIGHT||join(process.env.HOME,'.cache/stromea-pw/node_modules/playwright-core/index.mjs');
const {chromium}=await import(pathToFileURL(pw));
const browser=await chromium.launch({headless:true,args:['--disable-background-networking']});

async function run(width){
 const work=await mkdtemp(join(output,'runtime-')),port=await freePort(),base='http://127.0.0.1:'+port;
 const server=spawn(binary,['serve','--state',join(work,'state'),'--listen','127.0.0.1:'+port,'--loopback-only','--loopback-partner'],{cwd:root,stdio:['pipe','ignore','ignore']});
 const fixture=command=>server.stdin.write(command+'\n');
 const context=await browser.newContext({viewport:{width,height:900},locale:'de-DE',reducedMotion:'reduce'}),page=await context.newPage();
 const errors=[];page.on('pageerror',e=>errors.push(e.message));page.setDefaultTimeout(8000);
 const step=async(name,fn)=>{try{await fn();results.push({name,width,passed:true});}catch(e){results.push({name,width,passed:false,error:e.message.split('\n')[0]+' @'+(e.stack.match(/test_ux043\.mjs:(\d+)/)||[])[1]});throw e;}};
 try{
  await until(async()=>{try{return (await fetch(base+'/api/state')).ok;}catch{return false;}},'server');
  const peer=(await (await fetch(base+'/api/state')).json()).device;
  await context.route('**/*',async route=>{const u=new URL(route.request().url());
   if(u.origin!==base && !['data:','blob:','about:'].includes(u.protocol))return route.abort('blockedbyclient');
   if(u.pathname==='/api/interfaces')return route.fulfill({json:[{name:'mock-ethernet',kind:'ethernet',address:'192.0.2.1'},{name:'mock-wifi',kind:'wifi',address:'192.0.2.2'},{name:'loopback',address:'127.0.0.1',loopbackOnly:true}]});
   // The loopback-only server accepts only its own adapter; the result names the adapter it was found on.
   if(u.pathname==='/api/discover-lan'){await wait(700);return route.fulfill({json:[{...peer,interface:'loopback'}]});}
   return route.continue();});
  const t=s=>page.evaluate(s=>StromeaI18n.t(s),s),visible=sel=>page.locator(sel).isVisible();
  const boxes=sels=>page.evaluate(sels=>sels.map(s=>{const r=document.querySelector(s)?.getBoundingClientRect();return r?[Math.round(r.x),Math.round(r.y+scrollY),Math.round(r.width),Math.round(r.height)]:null;}),sels);
  const shot=name=>page.screenshot({path:join(output,'screens',name+'-'+width+'.png'),fullPage:true});
  const audit=async name=>{for(const lang of langs){await page.evaluate(l=>StromeaI18n.setLanguage(l),lang);await wait(60);const a=await page.evaluate(auditDOM);assert.deepEqual(a.german,[],name+' '+lang+': untranslated copy');const c=await page.evaluate(visibleCopy036c);assert(c.passed,name+' '+lang+': '+JSON.stringify(c.bad));}await page.evaluate(()=>StromeaI18n.setLanguage('de'));};
  await page.goto(base+'/?step=1');await page.waitForFunction(()=>typeof state!=='undefined' && !!state);await page.locator('#mode-onsite').check();

  // UX-G16 / UX-S44: „Suche läuft…“ has its reserved row below the actions; nothing moves; results name the adapter.
  await step('search-row-no-jump',async()=>{
   const sels=['#discover','#qr-toggle','#manual-toggle','#device-results'],before=await boxes(sels);
   await page.locator('#discover').click();await page.locator('#search-motion').waitFor({state:'visible'});
   const during=await boxes([...sels,'#search-motion']);await shot('01-search');
   for(let i=0;i<3;i++)assert.deepEqual([during[i][0],during[i][1]],[before[i][0],before[i][1]],sels[i]+' keeps its place while searching');
   assert.equal(during[3][1],before[3][1],'content below the actions does not move');
   const [m]=await boxes(['#step-1 .pair-methods']);assert(during[4][1]>=m[1]+m[3]-1,'search status below the actions, not beside them');
   await page.locator('.device-choice').first().waitFor();const after=await boxes(sels);for(let i=0;i<3;i++)assert.deepEqual(after[i].slice(0,2),before[i].slice(0,2),sels[i]+' after the search');
  });
  await step('result-names-adapter',async()=>{
   const expected=(await t('über {adapter}')).replace('{adapter}',(await t('Netzwerk'))+' (loopback)');
   assert((await page.locator('.device-choice .device-adapter').first().innerText()).includes(expected),'search result names its adapter');
   assert.equal(await page.locator('#device-count').innerText(),'1 '+await t('Gerät gefunden'),'found count names the result');
  });
  await page.locator('.device-choice').first().click();await page.locator('#compare-block').waitFor();
  // UX-S43: no second click; visible confirmation where the button was; step 2 opens by itself.
  await step('pairing-without-second-click',async()=>{
   const [slot]=await boxes(['#step-1 .pair-primary']);
   const seen=page.evaluate(()=>new Promise(resolve=>{const lines=[];const timer=setInterval(()=>{const r=document.getElementById('reconnect-progress');if(r && r.checkVisibility() && r.textContent.trim())lines.push(r.textContent);const s=document.getElementById('pair-success');if(s && !s.hidden){clearInterval(timer);resolve({lines,text:s.textContent});}},20);setTimeout(()=>{clearInterval(timer);resolve({lines,text:null});},15000);}));
   await page.locator('#pair').click();const v=await seen;
   assert(v.text && v.text.includes(await t('Gekoppelt. Weiter zur Prüfung …')),'visible pairing confirmation');assert.deepEqual(v.lines,[],'no second grey timer line');
   const [during]=await boxes(['#step-1 .pair-primary']);assert(Math.abs(during[1]-slot[1])<=1,'confirmation in the place of the button');
   await page.locator('#step-2:not([hidden])').waitFor({timeout:4000});
   assert(await page.locator('#paired-status').isVisible(),'step 2 confirms the pairing');
   assert.equal(await page.locator('#run').isEnabled(),true,'start possible');
  });
  await shot('02-step2');
  // UX-S45: detected device type below „Prüfumfang“; one calculator link; bold W.
  await step('detected-device',async()=>{
   const v=await page.evaluate(()=>({legend:document.querySelector('#scope-control legend').nextElementSibling?.id,chip:document.getElementById('device-type-chip').textContent.trim(),calc:[...document.querySelectorAll('#step-2 button,#step-2 a')].filter(n=>n.checkVisibility() && n.textContent.trim()===StromeaI18n.t('Rechner öffnen')).length,w:getComputedStyle(document.getElementById('unit-lpc')).fontWeight}));
   assert.equal(v.legend,'detected-device','chip row directly below „Prüfumfang“');assert(v.chip.length>2,'chip names the device type or „unbekannt“');assert.equal(v.calc,1,'„Rechner öffnen“ exactly once');assert(Number(v.w)>=600,'W bold');
  });
  await page.locator('#site-customer').fill('Kunde 043');await page.locator('#lpc').fill('5000');await page.locator('#site-meter').fill('Z-043');
  const kept=async label=>{assert.equal(await page.locator('#site-customer').inputValue(),'Kunde 043',label+': customer');assert.equal(await page.locator('#site-meter').inputValue(),'Z-043',label+': meter');assert.equal(await page.locator('#lpc').inputValue(),'5.000',label+': limit');};
  // UX-G17 / UX-S8: profile and back – same step, same values, connection kept.
  await step('profile-return-keeps-state',async()=>{
   await page.locator('.main-menu a[href="/profil"]').click();await page.locator('#profile-inspector').waitFor();await shot('03-profile');
   const v=await page.evaluate(()=>({inActions:!!document.querySelector('.profile-actions #profile-return'),cls:document.getElementById('profile-return').className,save:document.querySelector('.profile-actions button[type=submit]').disabled,required:document.querySelector('label[for=profile-inspector] .field-required')?.textContent,optional:document.querySelectorAll('.profile-form .field-optional').length,logoHint:document.body.innerText.includes('Logo verkleinert')}));
   assert(v.inActions && v.cls.includes('secondary'),'return is a secondary button next to „Profil speichern“');assert(v.save,'nothing changed: save disabled');assert.equal(v.required,await t('Pflichtfeld'));assert(v.optional>=6,'every other field says optional');assert(!v.logoHint);
   await page.locator('#profile-business').fill('Firma 043');assert(await page.locator('.profile-actions button[type=submit]').isEnabled(),'save enabled after a change');
   await page.locator('#profile-business').fill('');assert(await page.locator('.profile-actions button[type=submit]').isDisabled(),'back to the stored value: disabled again');
   await page.reload();await page.locator('#profile-inspector').waitFor(); // R043-02: return target survives a reload of the profile
   await page.locator('#profile-return').click();await page.locator('#step-2:not([hidden])').waitFor();await kept('profile');
   assert(await page.locator('#run').isVisible(),'still connected after the profile');
  });
  // UX-G18 / UX-S46 / UX-G17: a lost connection (like the WLAN drop of the Opta) is named; one click reconnects; values stay.
  await step('lost-connection-step-2',async()=>{
   const [row]=await boxes(['.start-action']);fixture('drop-connection');
   await page.locator('#run-reconnect:not([hidden])').waitFor({timeout:8000});await shot('04-step2-lost');
   const v=await page.evaluate(()=>({run:document.getElementById('run').checkVisibility(),reason:document.getElementById('start-reason').textContent,shown:document.getElementById('start-reason').checkVisibility(),status:document.getElementById('paired-status').textContent,reconnect:document.getElementById('run-reconnect').disabled}));
   assert(!v.run && !v.reconnect,'primary action is „Neu verbinden“');
   if(width>600){const [b,r]=await boxes(['#run-reconnect','#start-reason']);assert(b[2]<400 && r[0]>=b[0]+b[2]-1 && Math.abs(r[1]-b[1])<30,'G043-03: button keeps its size, reason beside it: '+JSON.stringify([b,r]));}assert(v.shown && v.reason===await t('Verbindung zum Gerät getrennt. Deine Eingaben bleiben erhalten.'),'reason next to the button: '+v.reason);
   assert(v.status.includes(await t('Verbindung zum Gerät getrennt')),'status line names the loss');const [after]=await boxes(['.start-action']);assert.equal(after[1],row[1],'start row keeps its place');await kept('lost');
   await page.locator('.main-menu a[href="/hilfe"]').click();await page.locator('.local-page').waitFor();await page.goBack();await page.locator('#step-2:not([hidden])').waitFor();await kept('help-back');
   await page.reload();await page.locator('#step-2:not([hidden])').waitFor();await kept('reload');assert(await visible('#run-reconnect'),'reload keeps the reconnect offer');
   await page.locator('#nav-1').click();await page.locator('.device-choice').first().click();await page.locator('#compare-block').waitFor();
   assert.equal(await page.locator('#simulator-trust').innerText(),await t('Verbindung getrennt'),'no false „Wartet auf Freigabe“ after a drop');
   await page.goBack();await page.locator('#step-2:not([hidden])').waitFor();await kept('history back to step 2');
   await page.locator('#run-reconnect').click();await page.locator('#run:not([hidden])').waitFor({timeout:10000});await until(()=>page.locator('#run').isEnabled(),'reconnected');
   assert(await page.locator('#step-2').isVisible(),'reconnect stays in step 2');await kept('reconnected');
   // R043-06: a reconnect started in step 2 belongs to step 2, also after a reload while it runs.
   assert.equal(await page.evaluate(()=>sessionStorage.getItem('stromea-connect-origin-043')),'2','origin of the reconnect stored');
   await page.reload();await page.waitForFunction(()=>typeof state!=='undefined' && !!state);
   assert.equal(await page.evaluate(()=>{const saved=state;state={...state,kind:'connect',busy:true};const v=operationStep();state=saved;return v;}),2,'operation step after reload');
  });
  // UX-S46: start without inspector name → persistent dialog → save & continue → running.
  await step('inspector-dialog',async()=>{
   await page.locator('#run').click();await page.locator('#inspector-dialog[open]').waitFor();await shot('05-inspector-dialog');
   await page.mouse.click(5,5);assert(await page.locator('#inspector-dialog[open]').count(),'click beside the dialog keeps it open');
   await page.locator('#inspector-dialog-cancel').click();assert.equal(await page.locator('#inspector-dialog[open]').count(),0,'Abbrechen closes');assert(await visible('#step-2'),'back in step 2');
   await page.locator('#run').click();await page.locator('#inspector-dialog-save').click();assert.equal(await page.locator('#inspector-dialog-error').innerText(),await t('Bitte gib deinen Namen ein.'),'empty name explained');
   await page.locator('#inspector-dialog-name').fill('Prüferin 043');await page.locator('#inspector-dialog-save').click();
   await page.locator('#live').waitFor({timeout:10000});assert.equal(await page.evaluate(()=>StromeaProfile.snapshot().inspector),'Prüferin 043','name stored in the profile');
  });
  // UX-S47: one decision card with the timer in its header.
  await step('low-load-card',async()=>{
   await page.locator('#low-load:not([hidden])').waitFor({timeout:20000});await shot('06-low-load');
   const v=await page.evaluate(()=>({banner:document.getElementById('wait-banner').checkVisibility(),timer:document.getElementById('low-load-timer').textContent,rows:document.querySelectorAll('#low-load .decision-table tbody tr').length,options:[...document.querySelectorAll('#low-load .decision-option')].map(o=>[o.querySelector('button').checkVisibility(),o.querySelector('.option-copy').textContent.trim().length]),warnings:[...document.querySelectorAll('#live .notice.warning')].filter(n=>n.checkVisibility()).length}));
   assert(!v.banner,'no second banner');assert.match(v.timer,/^\d+:\d\d$/,'timer in the card header');assert(v.rows>=1,'measured vs. required per direction');
   assert.equal(v.options.length,2);for(const [b,l] of v.options)assert(b && l>20,'each option has its half-sentence');assert.equal(v.warnings,0,'no extra warning card');
   await audit('low-load');
  });
  // UX-S48: the cause stands directly below the title, with one action; no bottom card.
  await step('abort-cause-on-top',async()=>{
   fixture('drop-connection');await page.locator('#result-abort:not([hidden])').waitFor({timeout:30000}).catch(async e=>{await shot('dbg-abort');console.log(JSON.stringify(await page.evaluate(()=>({busy:state.busy,kind:state.kind,progress:state.progress?.current,low:!!state.progress?.lowLoad,report:!!state.report,abort:state.report?.abortReason,events:state.progress?.events,connected:state.connected,error:state.error}))));throw e;});await wait(300);await shot('07-abort');
   const v=await page.evaluate(()=>{const r=id=>document.getElementById(id).getBoundingClientRect(),title=r('outcome-title'),cause=r('result-abort');
    const before=['outcome-text','result-scope','result-meta','result-customer'].filter(id=>{const n=document.getElementById(id);return n.checkVisibility() && n.getBoundingClientRect().top<cause.top;});
    return {next:cause.top-title.bottom<=24 && !before.length?'result-abort':JSON.stringify([cause.top-title.bottom,before]),cause:document.getElementById('result-cause-title').textContent,text:document.getElementById('result-cause-text').textContent,action:document.getElementById('result-cause-action').checkVisibility()?document.getElementById('result-cause-action').textContent:null,runError:document.getElementById('run-error').checkVisibility(),scope:document.getElementById('result-scope').textContent,generic:document.getElementById('outcome-text').checkVisibility()};});
   assert.equal(v.next,'result-abort','cause directly below the result title');assert.equal(v.cause,await t('Verbindung zum Gerät unterbrochen'));
   assert(/\d+:\d\d/.test(v.text),'names when: '+v.text);assert.equal(v.action,await t('Neu verbinden'),'exactly one action');assert(!v.runError,'no „Prüfung unterbrochen“ card below');assert(!v.generic);
   assert(v.scope.startsWith(await t('Prüfumfang:')),'„Prüfumfang“, not „Geprüfter Umfang“');
   await audit('abort-result');
  });
  // R043-06: after a finished run, „Neu verbinden“ in step 2 with a slow request, profile and back: stays in step 2.
  await step('reconnect-race-keeps-step-2',async()=>{
   await page.route('**/api/connect',async route=>{await wait(2500);await route.continue();},{times:1});
   if(await visible('#nav-2'))await page.locator('#nav-2').click();else await page.evaluate(()=>goStep(2));
   await page.locator('#run-reconnect:not([hidden])').waitFor();
   // A poll started before the click answers late with the old run state (R043-06, Re-Review 3).
   let slow=true;await page.route('**/api/state',async route=>{if(slow){slow=false;await wait(1500);}await route.continue();});
   await page.waitForRequest(r=>r.url().endsWith('/api/state'));const stale=page.waitForResponse(r=>r.url().endsWith('/api/state'));await page.locator('#run-reconnect').click();await stale;await wait(100);
   await page.locator('.main-menu a[href="/profil"]').click();await page.locator('#profile-return').click();
   await page.locator('#step-2:not([hidden])').waitFor();await page.locator('#run:not([hidden])').waitFor({timeout:15000});
   assert(await visible('#step-2'),'still step 2 after the reconnect finished');await kept('reconnect race');
   await page.unroute('**/api/state');
  });
  await step('reconnect-from-result',async()=>{
   fixture('drop-connection');await until(async()=>!(await (await fetch(base+'/api/state')).json()).connected,'dropped');
   if(await visible('#nav-3'))await page.locator('#nav-3').click();else await page.evaluate(()=>goStep(3));
   await page.locator('#result-cause-action').waitFor();await page.locator('#result-cause-action').click();await page.locator('#step-2:not([hidden])').waitFor({timeout:12000});await kept('after reconnect from the result');
  });
  // R043-01: a failed reconnect keeps step 2, the values and the reconnect offer (device gone for good here).
  await step('failed-reconnect-keeps-step-2',async()=>{
   fixture('stop-device');await page.locator('#run-reconnect:not([hidden])').waitFor({timeout:10000});
   await page.locator('#run-reconnect').click();await page.locator('#config-error:not([hidden])').waitFor({timeout:20000});
   await wait(1500);assert(await visible('#step-2'),'still step 2');assert(await visible('#run-reconnect'),'reconnect offered again');assert(await page.locator('#run-reconnect').isEnabled(),'retry possible');await kept('failed reconnect');
  });
  assert.deepEqual(errors,[],'no page errors');
 }finally{await context.close();server.stdin.end('quit\n');if(server.exitCode===null){server.kill('SIGTERM');await Promise.race([once(server,'exit'),wait(5000)]);}await rm(work,{recursive:true,force:true});}
}
let failed=false;
for(const width of (process.env.UX043_WIDTHS||'1440,390').split(',').map(Number)){try{await run(width);}catch(e){failed=true;console.error('043 '+width+': '+e.message);}}
await browser.close();
await writeFile(join(output,'ux043.json'),JSON.stringify(results,null,2)+'\n');
for(const r of results.filter(r=>!r.passed))console.error(r.name+' '+r.width+': '+r.error);
console.log('043 Nutzerweg: '+results.filter(r=>r.passed).length+'/'+results.length+' Schritte bestanden');
process.exitCode=failed || results.some(r=>!r.passed)?1:0;
