#!/usr/bin/env node
// Binary acceptance: real loopback HTTP/DNS-SD/TLS/SHIP/SPINE; only the adapter label is mocked.
import assert from 'node:assert/strict';
import {forbiddenConfirmationLabels,splitWords} from './ux036-audit.mjs';
import {visibleCopy036c} from './copy036c.mjs';
import {auditDOM} from './audit034.mjs';
import {captureDOM035} from './dom035.mjs';
import copy035 from './copy035.cjs';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {createHash} from 'node:crypto';
import {readFile,writeFile,mkdir,mkdtemp,rm} from 'node:fs/promises';
import {resolve,join,dirname} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..');
assert(process.env.E2E_BINARY,'E2E_BINARY must identify the finished release binary');
const binary=resolve(root,process.env.E2E_BINARY),hash=createHash('sha256').update(await readFile(binary)).digest('hex');
const arg=k=>process.argv.find(v=>v.startsWith('--'+k+'='))?.split('=')[1];
const browserName=arg('browser')||'chrome',only=arg('case');assert(['chrome','chromium'].includes(browserName));
const scopesOnly=arg('scopes-only')==='1',libraryOnly=arg('library-only')==='1';
const textMode=arg('texts')==='035';
const identityMutation=arg('mutation')==='identity-confirm';if(identityMutation)assert(only==='hell-1440'&&!scopesOnly&&!libraryOnly,'identity mutation uses the full pairing case');
const langs=textMode?['de']:['de','en','pl','ru','uk','tr','ro'];
const expected=libraryOnly?['reports-list','reports-nojs']:scopesOnly?['scope-both-result','scope-production-result']:['start','reports-empty','report-empty','manual','manual-address-error','manual-id-error','search-running','search-empty','device-list','device-details','pair-help','connection-diagnostics','about','qr-panel','qr-invalid','qr-valid','own-qr','compare','connecting','connect-cancelled','connected','preparation','preparation-consumption','preparation-production','preparation-long','calculator-ems','calculator-invalid','calculator-applied','customer','signatures','help-sign-operator','help-sign-inspector','preflight','advanced',...['lpc','lpp','failsafe','tolerance','profile','duration'].map(v=>'help-'+v),'config-error','practice-preparation','practice-running','practice-result','practice-saved','practice-export','onsite-running','onsite-result','onsite-export','pdf-export-error','onsite-saved','report-site-edit','fallback','result-details','reports-list','reports-nojs','reports-search-empty','profile','profile-validation','profile-logo-preview','profile-saved','help-all','low-load','low-load-explanation','low-load-opposite-direction','aborted-result','failed-result','wrong-id','device-gone','identity-reset-confirm','identity-reset-result'];
const output=resolve(root,arg('output')||(textMode?'artifacts/035-dom':'docs/tests/034')),work=join(root,'artifacts/034-e2e',browserName);await mkdir(work,{recursive:true});await mkdir(output,{recursive:true});
await writeFile(join(work,'go.mod'),'module local.invalid/e2e-artifacts\n\ngo 1.26.0\n');
const pw=process.env.E2E_PLAYWRIGHT||join(process.env.HOME,'.cache/stromea-pw/node_modules/playwright-core/index.mjs');const {chromium}=await import(pathToFileURL(pw));
const wait=ms=>new Promise(r=>setTimeout(r,ms));
async function until(fn,label,ms=20000){const end=Date.now()+ms;while(Date.now()<end){const v=await fn();if(v)return v;await wait(100);}throw Error('Timeout: '+label);}
async function freePort(){const s=createServer();s.listen(0,'127.0.0.1');await once(s,'listening');const p=s.address().port;await new Promise(r=>s.close(r));return p;}
const results=[];
async function run(width,theme){
 const label=theme+'-'+width;if(only && only!==label)return;
 console.log(browserName+' '+label+' starting');
 await rm(join(output,scopesOnly?'scopes':'',browserName,theme,String(width)),{recursive:true,force:true});
 const stateDir=await mkdtemp(join(work,'state-')),profile=await mkdtemp(join(work,'profile-'));const base='http://127.0.0.1:'+await freePort();
 const server=spawn(binary,['serve','--state',stateDir,'--listen',base.slice(7),'--loopback-only','--loopback-partner'],{cwd:root,stdio:['pipe','pipe','pipe']});let serverOutput='';server.stdout.on('data',b=>serverOutput+=b.toString());server.stderr.resume();
 const result={browser:browserName,width,theme,binarySHA256:hash,releaseBinary:true,views:[],actions:[],errors:[],external:[],expected,passed:false};results.push(result);let context,stall,networkFault=false;const sockets=new Set();result.expectedNetworkErrors=[];
 try{
  await until(async()=>{try{return (await fetch(base+'/api/instance')).ok;}catch{return false;}},'serve');
  context=await chromium.launchPersistentContext(profile,{...(browserName==='chrome'?{channel:'chrome'}:{}),headless:true,viewport:{width,height:width===390?844:1000},deviceScaleFactor:1,colorScheme:theme==='hell'?'light':'dark',locale:'de-DE',downloadsPath:join(work,'downloads'),args:['--disable-background-networking','--disable-component-update','--disable-sync','--disable-breakpad','--no-first-run','--disable-features=MediaRouter']});
  await context.route('**/*',route=>{const u=new URL(route.request().url());if(u.origin===base){if(u.pathname==='/api/interfaces')return route.fulfill({json:[{name:'e2e-adapter',kind:'ethernet',address:'192.0.2.1'}]});if(u.pathname==='/api/discover-lan')return route.continue({postData:JSON.stringify({interface:'loopback'})});return route.continue();}if(['data:','blob:','about:'].includes(u.protocol))return route.continue();result.external.push(u.origin);return route.abort('blockedbyclient');});
  const page=context.pages()[0];result.version=context.browser().version();page.setDefaultTimeout(10000);
  page.on('pageerror',e=>result.errors.push(e.message));page.on('console',m=>{if(m.type()==='error'){if(networkFault && /ERR_INTERNET_DISCONNECTED|Failed to fetch/.test(m.text()))result.expectedNetworkErrors.push(m.text());else result.errors.push(m.text());}});
  const loc=id=>page.locator('#'+id),state=async()=>{const r=await page.request.get(base+'/api/state');assert.equal(r.status(),200);return r.json();};
  let sequence=0;
  async function view(name,{all=true}={}){
   for(const lang of all?langs:['de']){
    if(name==='practice-running'){await until(async()=>!(await state()).busy,'previous fast practice');await loc('repeat').click();await loc('run').click();await loc('live').waitFor();}
    const transient=({'search-running':'search-motion','connecting':'connection-operation','onsite-running':'live','practice-running':'live','low-load':'low-load','low-load-explanation':'low-load'})[name];
    if(name==='search-running' && !await loc('search-motion').isVisible()){await loc('discover').click();await loc('search-motion').waitFor();}
    if(transient && !await loc(transient).isVisible()){result.actions.push('missed transient '+name+' '+lang);continue;}
    // This calls the production language selector handler without disturbing modal focus.
    await page.evaluate(code=>StromeaI18n.setLanguage(code),lang);await wait(70);await page.evaluate(()=>document.fonts.ready);
    const copyChecks=[];let dialogText='';
    if(identityMutation && name==='compare' && lang==='en'){await page.evaluate(()=>{const label=document.createElement('label');label.className='identity-check';label.innerHTML='<input id="identity-confirm" type="checkbox"><span>Ich habe die Gerätekennung verglichen.</span>';document.querySelector('#copy-device-ski').after(label);});assert(await loc('identity-confirm').isVisible(),'mutation must actually be visible');result.actions.push('036d identity-confirm visibly injected in compare/en');}
    if(name==='identity-reset-confirm'){
     const expectedDialog=await page.evaluate(()=>StromeaI18n.t('Simulator-Kennung erneuern? Die bisherige Kopplung wird ungültig. Gib die neue Simulator-Kennung am Gerät frei.'));
     const waitingDialog=page.waitForEvent('dialog'),clicking=loc('reset-identity').click();const dialog=await waitingDialog;dialogText=dialog.message();await dialog.dismiss();await clicking;assert.equal(dialogText,expectedDialog,'specific reset confirmation '+lang);copyChecks.push({element:'browser-confirm',expected:expectedDialog});
    }
    if(name==='identity-reset-result'){assert(!await loc('pair-detail').isVisible(),'old selection cleared on identity renewal');assert(await loc('mode-simulator').isChecked());assert.equal(await loc('identity-status').innerText(),await page.evaluate(()=>StromeaI18n.t('Simulator-Kennung erneuert. Verbinde das Gerät erneut und gib die neue Kennung dort frei.')),'localized renewal result '+lang);}

    const errorCopy={
     'wrong-id':['pair-error-text','Unter dieser Adresse antwortet ein anderes Gerät. Prüfe die IP-Adresse und vergleiche die Gerätekennung erneut.'],
     'device-gone':['pair-error-text','Das Gerät ist nicht erreichbar. Prüfe, ob es eingeschaltet und mit dem Netzwerk verbunden ist und ob die IP-Adresse stimmt.'],
     'connect-cancelled':['pair-error-text','Verbindung abgebrochen. Versuche es erneut oder wähle ein anderes Gerät.'],
     'manual-address-error':['address-error','IP-Adresse mit Port eingeben'], // 039 A-01: precise short message
     'manual-id-error':['ski-error','40 Zeichen nötig'], // 039 QA-09/10
     'qr-invalid':['qr-status','Füge den vollständigen Text aus dem QR-Code des Geräts ein.'],
    }[name];
    if(errorCopy){const [id,key]=errorCopy,expectedText=await page.evaluate(k=>StromeaI18n.t(k),key);assert.equal(await loc(id).innerText(),expectedText,name+' specific error text '+lang);copyChecks.push({element:id,expected:expectedText});}
    if(['compare','compared','connected','connecting','wrong-id','device-gone'].includes(name)){
     assert.equal(await loc('device-name').innerText(),await page.evaluate(()=>StromeaI18n.t('Gerätekennung (SKI)')),'fixed device row label '+lang);
    }
    if(name==='connected'){assert.match(await loc('title-1').innerText(),/Stromea/,'connected heading retains device identity');}
    if(name==='onsite-result' || name.startsWith('scope-')){assert(!await loc('result-counts').isVisible(),'counts collapsed in details');assert(!await loc('result-counting').isVisible(),'counting explanation collapsed');}
    if(name==='search-empty')assert(!await loc('device-results-title').isVisible(),'no heading for an empty device list '+lang);
    if(['low-load','low-load-explanation','low-load-opposite-direction'].includes(name)){
     /* 043 (UX-S47): one card, table „Prüfrichtung · Gemessen · Nötig“ */assert.deepEqual(await loc('low-load-text').locator('tbody tr').first().locator('th,td').allInnerTexts(),await page.evaluate(()=>{const v=(state.progress.lowLoad.directions||[state.progress.lowLoad])[0],w=n=>StromeaI18n.t('{watts} W').replace('{watts}',n.toLocaleString(StromeaI18n.locale()));return [StromeaI18n.t(v.direction==='LPP'?'Einspeisung (LPP)':'Bezug (LPC)'),w(Math.max(0,v.direction==='LPP'?-v.watts:v.watts)),StromeaI18n.t('über {watts}').replace('{watts}',w(v.required))];}),'low-load reason in current language '+lang);
     assert.equal(await loc('low-load-text').locator('details').count(),0,'short explanation is directly visible '+lang);
    }
    if(name==='low-load'){
     assert.equal(await loc('title-3').innerText(),await page.evaluate(()=>StromeaI18n.t('Leistung für die Prüfung zu niedrig')));
     /* 043 (UX-S47): the card asks the question; it never repeats the step title */assert.equal(await loc('low-load').locator('h2,h3').count(),1,'one card heading '+lang);assert.notEqual(await loc('low-load').locator('h3').innerText(),await loc('title-3').innerText(),'no repeated low-load heading '+lang);
    }
    if(name==='help-tolerance')assert(!await loc('guide-sources').isVisible(),'50 W tolerance has no BNetzA citation '+lang);
    if(name==='help-lpp' && await loc('production-apply').isHidden())assert.equal(await loc('production-copy').innerText(),await page.evaluate(()=>StromeaI18n.t('Noch kein Messwert. Trage die Einspeisegrenze selbst ein.')),'no reading, no half-power promise '+lang);
    if(['practice-result','onsite-result','scope-both-result','scope-production-result'].includes(name)){
     if(lang==='ru' || name==='practice-result')await wait(650);
     assert(await loc('power-reference').locator('.chart-reference').isVisible(),'035c legend survives polling/language change '+name+' '+lang);
     const zero=await page.evaluate(()=>new Intl.NumberFormat(StromeaI18n.locale(),{style:'unit',unit:'second',unitDisplay:'narrow',maximumFractionDigits:0}).format(0));
     assert((await loc('power-chart').locator('text').allTextContents()).includes(zero),'localized chart seconds '+lang);
     if(lang==='ru')assert(!(await loc('power-chart').locator('text').allTextContents()).some(t=>/\d\s*s$/.test(t)),'no Latin s on Russian axes');
    }
    if(['help-lpc','calculator-ems','calculator-invalid','calculator-applied'].includes(name)){
     const minimum=await page.evaluate(()=>new Intl.NumberFormat(StromeaI18n.locale(),{useGrouping:true}).format(4200));assert((await loc('calc-formula').innerText()).includes(minimum),'localized 4200-W formula '+lang);
     assert(/08\.11\.2026|8 Nov 2026/.test(await loc('guide-sources').innerText()),'BNetzA checked date in every language '+lang);
    }
    if(name==='preparation'){
     assert(await page.evaluate(()=>!!(document.getElementById('capability-detail').compareDocumentPosition(document.getElementById('connection-timing-details')) & Node.DOCUMENT_POSITION_FOLLOWING)),'technical details follow device functions '+lang);
     if(width>=600)assert.equal(await loc('lpc-field').locator('p.help').first().evaluate(n=>getComputedStyle(n).hyphens),'none','desktop prose is not hyphenated '+lang);
    }
    const forbiddenCheckboxes=await page.evaluate(forbiddenConfirmationLabels);assert.deepEqual(forbiddenCheckboxes,[],name+' '+lang+': forbidden-checkboxes');
    const checkboxAudit={violations:forbiddenCheckboxes};
    if(name==='compare'){const a=await loc('copy-ski').boundingBox(),b=await loc('copy-device-ski').boundingBox();assert(Math.abs(a.x-b.x)<=2,'pairing copy symbols align '+lang);copyChecks.push({element:'pairing-copy-alignment',ownX:a.x,deviceX:b.x});}
    assert.deepEqual((await page.evaluate(visibleCopy036c)).bad,[],name+' '+lang+': visible placeholders/legacy labels');
    for(const heading of await page.locator('#pair-detail .pairing-cards>section>h3:visible').all())assert.deepEqual(await heading.evaluate(splitWords),[],name+': pairing label must not hyphenate');
    const copyDOM=await page.evaluate(captureDOM035);
    for(const text of [...copyDOM.text.split('\n'),...copyDOM.fields.flatMap(f=>[f.description,...(f.options||[])])]){assert.deepEqual(copy035.duplicates035(text),[],name+' '+lang+': duplicated wording '+text);assert(!copy035.forbiddenCopy035(text,lang),name+' '+lang+': forbidden wording '+text);}
    for(const f of copyDOM.fields.filter(f=>['lpc','lpp','failsafe','tolerance','duration-help'].includes(f.id))){assert(f.description && f.descriptionIds.includes('field-description-'+f.id) && !f.descriptionIds.includes('field-help'),name+' '+lang+': individual description '+f.id);}
    const audit=await page.evaluate(auditDOM,scopesOnly),dir=join(output,scopesOnly?'scopes':'',browserName,theme,String(width),lang);await mkdir(dir,{recursive:true});
    const files=[];const height=page.viewportSize().height,total=await page.evaluate(()=>document.documentElement.scrollHeight);
    for(let y=0,part=1;!textMode && y<total;y+=height,part++){
     await page.evaluate(top=>scrollTo(0,top),y);const file=join(dir,`${String(sequence).padStart(3,'0')}-${name}${part===1?'':'-p'+part}.png`);await page.screenshot({path:file,animations:'disabled'});files.push(file.slice(output.length+1));
    }
    await page.evaluate(()=>scrollTo(0,0));
    const panes=page.locator('#detail-inspector,.inspector-content,.saved-inspector');
    for(let i=0;i<await panes.count();i++){
     const pane=panes.nth(i);if(!await pane.isVisible())continue;
     const size=await pane.evaluate(n=>({height:n.clientHeight,total:n.scrollHeight,overflow:getComputedStyle(n).overflowY}));
     if(size.height<200 || !['auto','scroll'].includes(size.overflow))continue;
     for(let y=size.height,part=2;!textMode && y<size.total;y+=size.height,part++){
      await pane.evaluate((n,top)=>n.scrollTop=top,y);const file=join(dir,`${String(sequence).padStart(3,'0')}-${name}-side${i}-${part}.png`);await page.screenshot({path:file,animations:'disabled'});files.push(file.slice(output.length+1));
     }
     await pane.evaluate(n=>n.scrollTop=0);
    }
    result.views.push({name,language:lang,files,audit,checkboxAudit,copyChecks,...(lang==='de'?{dom:{...copyDOM,...(dialogText?{dialog:dialogText}:{})}}:{})});
   }
   await page.evaluate(()=>StromeaI18n.setLanguage('de'));sequence++;console.log(browserName+' '+label+' '+name);
   await writeFile(join(output,`e2e-${scopesOnly?'scopes-':''}${browserName}-${label}.json`),JSON.stringify(result,null,2)+'\n');
  }
  async function closePanel(){if(await loc('inspector-close').isVisible())await loc('inspector-close').click();}
  async function goto(path){await page.goto(base+path,{waitUntil:'load'});await page.waitForFunction(()=>!window.StromeaRoute || (typeof state!=='undefined' && state && !StromeaRoute.restoring));}
  async function list(){await goto('/?new=1');if(await loc('change-device').isVisible())await loc('change-device').click();}
  async function connect(peer){const was=(await state()).connected;await list();await loc('manual-toggle').click();await loc('address').fill(peer.address);await loc('ski').fill(peer.ski);await loc('manual-connect').click();await until(async()=>{const s=await state();return s.connected && !s.busy;},'connected',70000);/* 043 (UX-S43): a new connection opens step 2 by itself; an already connected device keeps its one action */try{await loc('step-2').waitFor({timeout:4000});}catch{assert(was,'a new connection opens step 2 by itself (UX-S43)');assert.equal(await loc('pair').innerText(),await page.evaluate(()=>StromeaI18n.t('Prüfung vorbereiten →')),'already connected: one action');await loc('pair').click();await loc('step-2').waitFor();}await until(async()=>!(await state()).busy,'preparation');}
  async function setScope(value){await loc('scope-lpc').setChecked(['both','consumption'].includes(value));await loc('scope-lpp').setChecked(['both','production'].includes(value));assert.equal(await loc('scope').inputValue(),value);}
  async function business(company='E2E',inspector='Tester'){await goto('/profil');await loc('profile-business').fill(company);await loc('profile-inspector').fill(inspector);/* 043 (UX-S8): saving only after a change */if(await page.locator('.profile-form button[type=submit]').isEnabled())await page.locator('.profile-form button[type=submit]').click();await goto('/?step=2');await loc('step-2').waitFor();}
  async function openSaved(id){await goto('/protokolle');await loc('report-practice').check();/* 041: practice reports are hidden by default */await page.locator('.report-row[data-id="'+id+'"] a[href^="/protokoll"]').click();await loc('saved-result').waitFor();}
  async function timing(effect='5',limit='4200'){
   await setScope('consumption');await business();
   await loc('lpc').fill(limit);await loc('failsafe').fill('500');
   await loc('real-config').evaluate(n=>n.open=true);
   for(const [id,v] of Object.entries({'step-timeout':'8','effect-timeout':effect,'failsafe-timeout':'4','recovery-timeout':'10','cleanup-timeout':'8'}))await loc(id).fill(v);
   await loc('real-config').evaluate(n=>n.open=false);
  }
  async function finish(label){let decided=false,last='';return until(async()=>{const s=await state();result.lastProgress={busy:s.busy,current:s.progress?.current,error:s.error,lowLoad:!!s.progress?.lowLoad};if(s.progress?.current!==last){last=s.progress?.current;console.log(browserName+' '+label+' '+(last||'finished'));}if(label!=='aborted' && s.progress?.lowLoad && !decided){await loc('continue-low').click();decided=true;result.actions.push('low-load choice: '+label);}return !s.busy && s.report?s:false;},label,180000);}
  async function rendered(s){
   await loc('downloads').waitFor();await until(async()=>{const value=await loc('site-editor').getAttribute('data-report');return !!value && JSON.parse(value)[0]===s.report.id;},'rendered current report');
  }
  async function exports(name){
   const reports=[];
   for(const ext of ['pdf','json']){const downloading=page.waitForEvent('download');await page.locator(`#downloads a[download$=".${ext}"]`).click();const d=await downloading;const file=join(work,`${label}-${name}.${ext}`);await d.saveAs(file);const data=await readFile(file);if(ext==='pdf')assert(data.subarray(0,5).equals(Buffer.from('%PDF-')));else reports.push(JSON.parse(data));}
   assert(reports[0]?.id);result.actions.push(name+'-pdf-json-downloads');await view(name+'-export');
  }
  async function checkPracticeLibrary(practice){
   const response=await page.request.get(base+'/api/reports');assert.equal(response.status(),200);const entries=await response.json(),entry=entries.find(e=>e.id===practice.report.id);
   assert(entry,'the actual completed practice report is listed');assert.equal(entry.report.mode,'simulator','035c: saved practice mode must survive the /api/reports summary');
   await goto('/protokolle');await loc('report-practice').check();await page.locator('.report-row[data-id="'+practice.report.id+'"]').waitFor();
   for(const language of langs){await page.evaluate(l=>StromeaI18n.setLanguage(l),language);await wait(70);const expected=await page.evaluate(()=>StromeaI18n.t('Bestanden')+' '+StromeaI18n.t('(Übung)'));assert.equal(await page.locator('.report-row[data-id="'+practice.report.id+'"] .report-status').innerText(),expected,'practice list badge '+language);}
   await view('reports-list');
   const nojs=await context.browser().newContext({javaScriptEnabled:false,viewport:{width,height:844},colorScheme:theme==='hell'?'light':'dark'});
   await nojs.route('**/*',route=>{const u=new URL(route.request().url());if(u.origin===base || ['data:','blob:','about:'].includes(u.protocol))return route.continue();result.external.push(u.origin);return route.abort('blockedbyclient');});
   try{const fallback=await nojs.newPage();fallback.on('console',m=>{if(m.type()==='error')result.errors.push('No-JS list: '+m.text());});fallback.on('pageerror',e=>result.errors.push('No-JS list: '+e.message));await fallback.goto(base+'/protokolle');assert(!await fallback.locator('header .preferences').isVisible(),'No-JS list has no empty language picker');assert(!await fallback.locator('#practice-badge').isVisible(),'No-JS library has no global practice badge');assert(!await fallback.locator('#step-1').isVisible(),'No-JS library hides the device wizard');assert(await fallback.locator('.brand-heading h1').isVisible(),'No-JS library retains its header');const row=fallback.locator('#report-list-fallback tr[data-id="'+practice.report.id+'"]');await row.waitFor();assert.equal(await row.locator('[data-column="result"]').innerText(),'Bestanden (Übung)','No-JS list identifies practice');assert.equal(await row.locator('[data-column="mode"]').innerText(),'Übungsmodus');
    const files=[],dir=join(output,browserName,theme,String(width),'de');await mkdir(dir,{recursive:true});const total=await fallback.evaluate(()=>document.documentElement.scrollHeight);
    for(let y=0,part=1;!textMode&&y<total;y+=844,part++){await fallback.evaluate(top=>scrollTo(0,top),y);const name='reports-nojs'+(part===1?'':'-p'+part)+'.png';await fallback.screenshot({path:join(dir,name)});files.push(`${browserName}/${theme}/${width}/de/${name}`);}
    const forbiddenCheckboxes=await fallback.evaluate(forbiddenConfirmationLabels);assert.deepEqual(forbiddenCheckboxes,[],'reports-nojs forbidden-checkboxes');assert.deepEqual((await fallback.evaluate(visibleCopy036c)).bad,[]);result.views.push({checkboxAudit:{violations:forbiddenCheckboxes},name:'reports-nojs',language:'de',files,audit:await fallback.evaluate(auditDOM),dom:await fallback.evaluate(captureDOM035)});
   }finally{await nojs.close();}
   result.actions.push('035c: real completed practice -> summary mode -> localized list badge in all languages -> German No-JS list');
  }
  if(libraryOnly){await goto('/');await loc('mode-simulator').check();await loc('pair-simulator').click();await loc('step-2').waitFor();await setScope('consumption');await loc('run').click();await loc('live').waitFor();const practice=await finish('library practice');assert.equal(practice.report.mode,'simulator');assert(practice.report.passed);await rendered(practice);await checkPracticeLibrary(practice);assert(!result.views.some(v=>v.audit.horizontal||v.audit.german.length||v.audit.clipped.length));assert.deepEqual(result.errors,[]);assert.deepEqual(result.external,[]);result.passed=true;return;}
  if(scopesOnly){
   await goto('/');await loc('mode-simulator').check();await loc('pair-simulator').click();await loc('step-2').waitFor();
   for(const scope of ['both','production']){
    if(scope==='production'){await loc('repeat').click();await loc('step-2').waitFor();}
    await setScope(scope);await loc('run').click();await loc('live').waitFor();const completed=await finish('practice '+scope);assert.equal(completed.outcome,'Bestanden');await rendered(completed);const pdf=await page.request.get(base+'/api/report.pdf');assert.equal(pdf.status(),200);const pdfBytes=await pdf.body();assert(pdfBytes.subarray(0,5).equals(Buffer.from('%PDF-')));await writeFile(join(work,label+'-scope-'+scope+'.pdf'),pdfBytes);result.actions.push('real '+scope+' PDF generated');
    assert.equal(await page.locator('#result-matrix thead th:visible').count(),scope==='both'?3:2);await view('scope-'+scope+'-result');
   }
   result.missing=expected.flatMap(name=>langs.filter(language=>!result.views.some(v=>v.name===name&&v.language===language)).map(language=>({name,language})));
   result.auditFailures=result.views.filter(v=>v.audit.horizontal||v.audit.german.length||v.audit.clipped.length);
   assert.deepEqual(result.missing,[]);assert.deepEqual(result.errors,[]);assert.deepEqual(result.external,[]);assert.equal(result.auditFailures.length,0,'Scope table layout findings');result.passed=true;return;
  }
  await goto('/');await loc('interface-single').waitFor();assert.equal(await loc('interface option').count(),1,'single mock adapter');assert(!await loc('interface').isVisible(),'one adapter is plain text');
  for(const lang of langs){await loc('language').selectOption(lang);assert.equal(await page.locator('html').getAttribute('lang'),lang);}await loc('language').selectOption('de');result.actions.push('language-selector-all-seven');
  await view('start');await loc('about-open').click();await loc('about-dialog').waitFor();await view('about');await loc('about-close').click(); /* 041: Dialog statt Aufklapper */const peer=(await state()).device;assert(peer.address.startsWith('127.0.0.1:'));
  await goto('/protokolle');await view('reports-empty');await goto('/protokoll');await view('report-empty');await goto('/');
  await loc('manual-toggle').focus();await page.keyboard.press('Enter');await view('manual');
  if(width===390){await loc('inspector-close').focus();await page.keyboard.press('Shift+Tab');assert(await page.evaluate(()=>document.activeElement.closest('#detail-inspector')!==null));result.actions.push('keyboard-modal-focus-trap');}
  await loc('address').fill('invalid');await loc('ski').fill(peer.ski);await loc('manual-connect').click();await loc('address-error').waitFor();await view('manual-address-error');
  await loc('address').fill(peer.address);await loc('ski').fill('1234');await loc('manual-connect').click();await loc('ski-error').waitFor();await view('manual-id-error');await page.keyboard.press('Escape');assert(!await loc('manual-device').isVisible());
  server.stdin.write('hide-device\n');await until(()=>serverOutput.includes('Ankündigung verborgen'),'hide DNS');
  const searching=page.waitForResponse(r=>r.url()===base+'/api/discover-lan');await loc('discover').click();await view('search-running');await searching;await until(async()=>!(await state()).search?.active,'empty discovery complete',10000);await loc('search-motion').waitFor({state:'hidden'});await view('search-empty');
  server.stdin.write('show-device\n');await until(()=>serverOutput.includes('Ankündigung sichtbar'),'show DNS');await loc('discover').click();await page.locator('.device-choice').first().waitFor();await view('device-list');
  await loc('qr-toggle').click();await view('qr-panel');await loc('qr-text').fill('invalid');await loc('qr-apply').click();await view('qr-invalid');
  await loc('qr-text').fill('SHIP;SKI:'+peer.ski+';ID:e2e034;ENDSHIP;');await loc('qr-apply').click();await view('qr-valid');await loc('qr-simulator-tab').click();assert(await loc('panel-local-qr').locator('svg').count());await view('own-qr');await closePanel();
  await list();await loc('discover').click();await page.locator('.device-choice').first().click();await view('device-details');await loc('pair-help').locator('summary').click();await view('pair-help');await loc('pair-help').evaluate(n=>n.open=false);await view('compare');
  // A real TCP endpoint that never completes TLS exercises connection/cancellation.
  await list();stall=createServer(s=>{sockets.add(s);s.on('close',()=>sockets.delete(s));});stall.listen(0,'127.0.0.1');await once(stall,'listening');
  await loc('manual-toggle').click();await loc('address').fill('127.0.0.1:'+stall.address().port);await loc('ski').fill(peer.ski);await loc('manual-connect').click();await loc('connection-operation').waitFor();await view('connecting');await loc('connect-cancel').click();await until(async()=>!(await state()).busy,'connect cancelled');await view('connect-cancelled');for(const s of sockets)s.destroy();await new Promise(r=>stall.close(r));stall=null;
  await connect(peer);await loc('nav-1').click();await page.locator('.device-choice[data-state=connected]').click();await view('connected');/* 043 (UX-S44): connection details in the detail column, no caption */if(await loc('details-toggle').isVisible())await loc('details-toggle').click();await loc('connection-timings').waitFor({state:width>1050?'visible':'attached'});assert(await loc('connection-timings').evaluate(n=>!!n.closest('#detail-inspector')),'timings live in the detail column');await view('connection-diagnostics');if(width<=1050)await closePanel();await loc('pair').click();await loc('step-2').waitFor();await view('preparation');
  for(const [scope,name] of [['consumption','preparation-consumption'],['production','preparation-production']]){await setScope(scope);await loc('scope-lpc').focus();await view(name);}await setScope('both');
  await loc('profile-full').check();await view('preparation-long');await loc('profile-quick').check();
  await loc('site-customer').fill('E2E Customer');await loc('site-meter').fill('M034');await view('customer');
  for(const [id,name] of [['pdf-notes','signatures'],['preflight-notes','preflight'],['real-config','advanced']]){if(id==='real-config')await loc(id).locator('summary').click();if(id==='pdf-notes'){for(const sign of ['sign-operator','sign-inspector']){await loc(sign).check();await view('help-'+sign);await closePanel();}}await view(name);if(id==='real-config')await loc(id).evaluate(n=>n.open=false);}
  for(const id of ['lpc','lpp','failsafe','tolerance']){await page.locator(`label[for="${id}"]`).click();await view('help-'+id);if(id==='lpc'){
   assert.equal(await loc('calc-power').getAttribute('placeholder'),null,'no numeric placeholder');assert(await loc('calc-apply').isDisabled(),'empty direct calculator has no result to adopt');await loc('calc-power').fill('15000');assert((await loc('calc-result').innerText()).includes('6.000'),'15 kW direct WP/climate equals 6 kW');await loc('calc-power').fill('');assert(await loc('calc-apply').isDisabled());
   await loc('calc-mode').selectOption('ems');await view('calculator-ems');await loc('calc-n').fill('0');assert(await loc('calc-apply').isDisabled());await view('calculator-invalid');await loc('calc-n').fill('2');await loc('calc-wp').fill('15000');await loc('calc-apply').click();await view('calculator-applied');
  }if(width===390)await closePanel();}
  await loc('profile-full').check();await page.locator('label:has(#profile-full)').click();if(await loc('details-toggle').isVisible())await loc('details-toggle').click();await view('help-profile');if(width===390)await closePanel();await loc('profile-quick').check();
  if(await loc('details-toggle').isVisible())await loc('details-toggle').click();await loc('duration-help').click();await view('help-duration');await closePanel();
  /* 043 (UX-S46): only the inspector is required; a missing direction is named with a field link */await business();await setScope('');await loc('run').click();await loc('config-error').waitFor();await view('config-error');await setScope('both');
  // Low load is produced by a real test limit above the simulator's actual power.
  await timing();await setScope('both');await loc('run').click();await loc('low-load').waitFor({timeout:30000});const opposite=(await state()).progress.lowLoad;assert.equal(opposite.directions.length,1);assert.equal(opposite.directions[0].direction,'LPP');assert(opposite.directions[0].watts>opposite.directions[0].required);await view('low-load-opposite-direction');if(browserName==='chrome' && width===1440 && theme==='hell'){
   await page.evaluate(()=>StromeaI18n.setLanguage('de'));const dir=join(output,'..','screens');await mkdir(dir,{recursive:true});for(const targetWidth of [1440,1280,820]){await page.setViewportSize({width:targetWidth,height:1000});assert(await loc('low-load').isVisible(),'capture the actual pending decision');assert(!await loc('outcome').isVisible(),'no stale result in the low-load screenshot');await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:join(dir,'low-load-direction-'+targetWidth+'.png'),fullPage:true});}await page.setViewportSize({width,height:1000});
  }await loc('cancel').click();await finish('aborted');await connect(peer);
  await timing('5','20000');await loc('run').click();await loc('low-load').waitFor({timeout:30000});await view('low-load');await view('low-load-explanation');await loc('remeasure').click();await loc('low-load').waitFor();await loc('cancel').click();const aborted=await finish('aborted');assert(aborted.report.abortReason==='user_cancel');await rendered(aborted);await view('aborted-result');
  await connect(peer);await timing();await loc('run').click();await loc('live').waitFor();
  await until(async()=>{const s=await state();if(s.progress?.lowLoad && await loc('continue-low').isVisible()){await loc('continue-low').click();return false;}return s.busy && !s.progress?.lowLoad && s.progress?.steps?.some(step=>step.name==='Heartbeat starten');},'active measurement phase',30000);
  await view('onsite-running');const passed=await finish('onsite result');assert.equal(passed.outcome,'Bestanden');assert(passed.report.steps.length>0);await rendered(passed);await view('onsite-result');
  if(await loc('open-evidence').isVisible())await loc('open-evidence').evaluate(n=>n.open=true);if(await loc('details-toggle').isVisible())await loc('details-toggle').click();await view('result-details');await loc('open-evidence').evaluate(n=>n.open=false);await closePanel();await exports('onsite');
  networkFault=true;await context.setOffline(true);await page.locator('#downloads a[download$=".pdf"]').click();await page.locator('#downloads .download-error').waitFor();await view('pdf-export-error');await context.setOffline(false);await wait(400);networkFault=false;
  const recoveryDownload=page.waitForEvent('download');await page.locator('#downloads a[download$=".pdf"]').click();await (await recoveryDownload).saveAs(join(work,label+'-recovered.pdf'));result.actions.push('real network failure and PDF retry');
  await openSaved(passed.report.id);await view('onsite-saved');await loc('site-edit-toggle').click();await loc('report-site-customer').fill('E2E Updated');const correction=page.waitForResponse(r=>r.url().includes('/api/report/site') && r.request().method()==='POST');await page.locator('#site-editor button[type=submit]').click();const revised=await (await correction).json();assert.notEqual(revised.id,passed.report.id);assert.equal(revised.revision.number,2);assert.equal(revised.revision.previousId,passed.report.id);await until(async()=>await loc('report-revision').locator('a').count()>0,'revision rendered');assert(!await loc('site-editor').isVisible(),'revision editor closes after saving');assert.deepEqual(await page.evaluate(()=>[document.activeElement?.id,document.activeElement?.tagName,document.activeElement?.getAttribute('aria-expanded')]),['site-edit-toggle','A','false'],'focus returns to the correction link after saving (UX-S17)');assert.equal(await loc('report-save-status').count(),0,'041: no save notice; the customer line shows the data');assert((await loc('result-customer-data').innerText()).includes('E2E Updated'),'041: corrected customer in the customer line');assert.equal(await loc('report-revision').locator('a').getAttribute('href'),'/protokoll?id='+encodeURIComponent(passed.report.id),'earlier-version link after correction');await view('report-site-edit');
  const revisedPDF=await page.request.get(base+'/api/report.pdf?id='+encodeURIComponent(revised.id));assert.equal(revisedPDF.status(),200);await writeFile(join(work,label+'-revision.pdf'),await revisedPDF.body());
  const originalAgain=await page.request.get(base+'/api/report.pdf?id='+encodeURIComponent(passed.report.id));assert((await originalAgain.body()).equals(await readFile(join(work,label+'-onsite.pdf'))),'old PDF remains byte-identical');
  assert.deepEqual(revised.samples,passed.report.samples);assert.deepEqual(revised.steps,passed.report.steps);assert.equal(revised.started,passed.report.started);assert.equal(revised.finished,passed.report.finished);
  const archive=await (await page.request.get(base+'/api/reports')).json();assert(!archive.some(e=>e.id===passed.report.id),'041: one row per inspection (newest version)');assert.equal(archive.find(e=>e.id===revised.id).report.revision.number,2);assert.equal(archive.find(e=>e.id===revised.id).versions,2);result.actions.push('036 immutable correction: original PDF bytes and measurements preserved; new PDF, ID, revision and archive entry');
  assert.equal(new URL(page.url()).searchParams.get('id'),revised.id,'saved-view URL follows the corrected version');await page.reload({waitUntil:'load'});await loc('saved-result').waitFor();await until(async()=>await loc('report-revision').locator('a').count()===1 && await loc('report-revision').locator('a').getAttribute('href')==='/protokoll?id='+encodeURIComponent(passed.report.id),'corrected version survives reload');assert(!/Berichtsversion|Version\s*\d/.test(await loc('saved-result').innerText()),'no version number in the saved view (UX-S18)');result.actions.push('036 corrected saved view survives reload at its new ID');
  if(await loc('site-editor').isVisible())await loc('site-edit-toggle').click();
  // Reload without JS proves server fallback count/table agreement independently.
  const nojs=await context.browser().newContext({javaScriptEnabled:false,viewport:{width,height:844}});const fallback=await nojs.newPage();await fallback.goto(base+'/protokoll');const forbiddenCheckboxes=await fallback.evaluate(forbiddenConfirmationLabels);assert.deepEqual(forbiddenCheckboxes,[],'fallback forbidden-checkboxes');assert.deepEqual((await fallback.evaluate(visibleCopy036c)).bad,[]);assert.equal(await fallback.locator('#saved-fallback dt').filter({hasText:/^Firma$/}).count(),1,'No-JS company label is Firma');const text=await fallback.locator('#saved-fallback').textContent();assert(!await fallback.locator('#fallback-counts').isVisible(),'fallback count is available in details');assert(text.includes('Funktionen:'));assert.equal(await fallback.locator('#saved-fallback table').first().locator('tbody tr').count(),5,'four functions plus the release row (UX-S16, 038)');assert.equal((await fallback.locator('#saved-fallback table').first().locator('tbody tr').last().locator('th').innerText()).trim(),'Testgrenzen aufgehoben');const fdir=join(output,browserName,theme,String(width),'de');const fallbackAudit=await fallback.evaluate(auditDOM,true),fallbackFiles=[];const fallbackHeight=await fallback.evaluate(()=>document.documentElement.scrollHeight);for(let y=0,part=1;!textMode && y<fallbackHeight;y+=844,part++){await fallback.evaluate(top=>scrollTo(0,top),y);const name='fallback'+(part===1?'':'-p'+part)+'.png';await fallback.screenshot({path:join(fdir,name)});fallbackFiles.push(`${browserName}/${theme}/${width}/de/${name}`);}result.views.push({checkboxAudit:{violations:forbiddenCheckboxes},name:'fallback',language:'de',files:fallbackFiles,audit:fallbackAudit,dom:await fallback.evaluate(captureDOM035),note:'Server fallback is German when JavaScript is disabled; localization is client-side.'});await nojs.close();
  // Tight electrical deadline produces a real negative measurement result.
  await goto('/');await connect(peer);await timing('1');await loc('tolerance').fill('0');await loc('run').click();await loc('live').waitFor();const failed=await finish('failed result');assert.equal(failed.outcome,'Nicht bestanden');await rendered(failed);await view('failed-result');
  await list();await loc('mode-simulator').check();await loc('pair-simulator').click();await loc('step-2').waitFor();await setScope('production');await page.reload();await loc('step-2').waitFor();assert.equal(await loc('scope').inputValue(),'production','practice scope survives real reload');assert.deepEqual((await state()).capabilities,{lpc:true,lpp:true,mpc:true});for(const language of langs){await page.evaluate(l=>StromeaI18n.setLanguage(l),language);await wait(60);assert.equal(await loc('scope').inputValue(),'production','scope survives language change '+language);assert.equal(await loc('capability-detail').locator('dd').filter({hasText:await page.evaluate(()=>StromeaI18n.t('Unterstützt'))}).count(),3);}result.actions.push('035b: real reload in practice step 2, three supported functions and selected scope preserved through all languages');await view('practice-preparation');await setScope('consumption');await loc('run').click();await loc('live').waitFor();await view('practice-running');const practice=await finish('practice result');assert.equal(practice.report.mode,'simulator');assert(practice.report.passed);await rendered(practice);await view('practice-result');await exports('practice');await openSaved(practice.report.id);await view('practice-saved');
  await checkPracticeLibrary(practice);await loc('report-search').fill('no-such-customer');await loc('report-empty').waitFor();await view('reports-search-empty');await loc('report-search').fill('');
  const dl=page.waitForEvent('download');await page.locator('.report-row a[download]').first().click();await (await dl).saveAs(join(work,label+'-library.pdf'));result.actions.push('library-pdf-download');
  await goto('/profil');await view('profile');await loc('profile-logo').setInputFiles({name:'invalid.txt',mimeType:'text/plain',buffer:Buffer.from('invalid')});await until(async()=>!!await loc('profile-status').innerText(),'profile validation');await view('profile-validation');const logo=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=c.height=32;const ctx=c.getContext('2d');ctx.fillStyle='#fff';ctx.fillRect(0,0,32,32);ctx.fillStyle='#000';ctx.fillRect(12,12,8,8);return c.toDataURL('image/png').split(',')[1];});await loc('profile-logo').setInputFiles({name:'e2e-logo.png',mimeType:'image/png',buffer:Buffer.from(logo,'base64')});await loc('profile-preview').waitFor();await view('profile-logo-preview');await loc('profile-business').fill('E2E');await loc('profile-inspector').fill('Tester');await page.locator('.profile-form button[type="submit"]').click();await view('profile-saved');
  await goto('/hilfe');assert.equal(await page.locator('.local-page details').count(),0,'help has no accordions');await view('help-all');
  // Entering practice mode replaced the local CS listener. Never reuse its old address.
  const currentPeer=(await state()).device;assert(currentPeer.address.startsWith('127.0.0.1:'));
  await list();await loc('mode-onsite').check();await loc('manual-toggle').click();await loc('address').fill(currentPeer.address);await loc('ski').fill((currentPeer.ski[0]==='0'?'1':'0')+currentPeer.ski.slice(1));await loc('manual-connect').click();await until(async()=>{const s=await state();return !s.busy && !!s.error;},'wrong ID',70000);await loc('pair-error').waitFor();const wrong=(await state()).error;assert.match(wrong,/Anderes Gerät antwortet/,'real TLS identity mismatch required');result.failureEvidence=[{name:'wrong-id',address:currentPeer.address,rawError:wrong}];await view('wrong-id');
  server.stdin.write('stop-device\n');await until(()=>serverOutput.includes('Loopback-Gerät beendet'),'stop peer');await list();await loc('manual-toggle').click();await loc('address').fill(currentPeer.address);await loc('ski').fill(currentPeer.ski);await loc('manual-connect').click();await until(async()=>{const s=await state();return !s.busy && !!s.error;},'device gone',70000);await loc('pair-error').waitFor();const gone=(await state()).error;assert.match(gone,/connection refused|unreachable|no route/i);assert(!/Anderes Gerät antwortet/.test(gone));result.failureEvidence.push({name:'device-gone',address:currentPeer.address,rawError:gone});await view('device-gone');
  await goto('/profil');await loc('reset-identity').waitFor();await view('identity-reset-confirm'); /* 041: im Profil */const oldIdentity=(await state()).localSki;
  const waitingReset=page.waitForEvent('dialog'),resetting=loc('reset-identity').click();await (await waitingReset).accept();await resetting;
  await until(async()=>{const s=await state();return !s.busy && s.localSki!==oldIdentity;},'new loopback identity');await until(async()=>!!await loc('identity-status').innerText(),'reset status');await view('identity-reset-result');result.actions.push('real loopback identity renewal; confirmation dismissed in every language first');
  result.missing=expected.flatMap(name=>(['fallback','reports-nojs'].includes(name)?['de']:langs).filter(language=>!result.views.some(v=>v.name===name && v.language===language)).map(language=>({name,language})));result.auditFailures=result.views.filter(v=>v.audit.horizontal || v.audit.german.length || v.audit.clipped.length).map(v=>({name:v.name,language:v.language,...v.audit}));
  assert(result.views.every(v=>v.checkboxAudit&&v.checkboxAudit.violations.length===0),'every recorded E2E view must have an observed checkbox audit');assert.deepEqual(result.missing,[],'Missing views');assert.deepEqual(result.errors,[],'Browser/console errors');assert.deepEqual(result.external,[],'External requests');assert.equal(result.auditFailures.length,0,'DOM translation/layout findings (see JSON)');result.passed=true;
 }catch(e){result.failure=String(e.message);console.error(browserName+' '+label+': '+e.message);process.exitCode=1;}
 finally{for(const socket of sockets)socket.destroy();if(stall)stall.close();if(context)await context.close();server.stdin.end();if(server.exitCode===null){server.kill('SIGTERM');await Promise.race([once(server,'exit'),wait(7000)]);if(server.exitCode===null)server.kill('SIGKILL');}await rm(stateDir,{recursive:true,force:true});await rm(profile,{recursive:true,force:true});await writeFile(join(output,`e2e-${scopesOnly?'scopes-':''}${browserName}-${label}.json`),JSON.stringify(result,null,2)+'\n');}
}
for(const width of [1440,390])for(const theme of ['hell','dunkel'])await run(width,theme);
console.log(browserName+': '+results.filter(r=>r.passed).length+'/'+results.length+' cases passed');
