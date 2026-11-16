#!/usr/bin/env node
// Actual production markup, styles and translations; isolated browser component probes.
import assert from 'node:assert/strict';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {resolve,join,dirname} from 'node:path';
import {pathToFileURL,fileURLToPath} from 'node:url';
import {readableContent as currentReadable,forbiddenConfirmationLabels as currentCheckboxes} from './ux036-audit.mjs';
import {visibleCopy036c as currentCopy} from './copy036c.mjs';
import {execFileSync} from 'node:child_process';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..'),out=resolve(process.env.REVIEW036D_OUTPUT||'artifacts/036d');await mkdir(out,{recursive:true});
const {chromium}=await import(pathToFileURL(process.env.E2E_PLAYWRIGHT||join(process.env.HOME,'.cache/stromea-pw/node_modules/playwright-core/index.mjs')));
const browser=await chromium.launch({headless:true,args:['--disable-background-networking']}),results=[];
const html=(await readFile(join(root,'internal/webui/assets/index.html'),'utf8')).replace(/<script\b[^>]*>[\s\S]*?<\/script>/g,'').replace(/<link\b[^>]*>/g,'').replaceAll('{{.Version}}','Testversion 036d').replaceAll('{{.Token}}','local-fixture');
let css=await readFile(join(root,'internal/webui/assets/style.css'),'utf8'),readableContent=currentReadable,forbiddenConfirmationLabels=currentCheckboxes,visibleCopy036c=currentCopy;
const revision=process.argv.find(v=>v.startsWith('--baseline='))?.slice('--baseline='.length);
if(revision){assert(/^[0-9a-f]{7,40}$/.test(revision));const source=path=>execFileSync('git',['show',revision+':'+path],{cwd:root,encoding:'utf8'});css=source('internal/webui/assets/style.css');for(const name of ['ux036-audit.mjs','copy036c.mjs'])await writeFile(join(out,name),source('scripts/'+name));({readableContent,forbiddenConfirmationLabels}=await import(pathToFileURL(join(out,'ux036-audit.mjs'))));({visibleCopy036c}=await import(pathToFileURL(join(out,'copy036c.mjs'))));}

async function reset(page,language,screen='start'){
 const ready=await page.evaluate(()=>!!window.reviewFixtureBody);if(!ready){await page.setContent(html);await page.addStyleTag({content:css});
 await page.evaluate(screen=>{
  document.documentElement.classList.remove('booting'); // 041: the fixture is a finished view, not the loading state
  const source=document.querySelector('#step-1'),prep=document.querySelector('#step-2'),footer=document.querySelector('footer');
  const start=document.createElement('section');start.id='step-1';start.className='card';const pairing=document.createElement('section');pairing.id='pair-detail';pairing.append(source.querySelector('#compare-block'));start.append(source.querySelector('.mode-control'),source.querySelector('.list-tools'),pairing);
  for(const el of start.querySelectorAll('[hidden]'))if(['device','lan-controls'].includes(el.id))el.hidden=false;
  start.querySelector('#interface').innerHTML='<option>Ethernet · 192.0.2.1</option><option>Wi-Fi · 192.0.2.2</option>';
  for(const id of ['device-ski','local-ski'])start.querySelector('#'+id).textContent='0123 4567 89AB CDEF 0123 4567 89AB CDEF 0123 4567';
  const wrap=document.createElement('div');wrap.className='workspace';const main=document.createElement('div');main.className='workspace-main';main.append(start,prep);wrap.append(main);document.querySelector('main').replaceChildren(wrap,footer); /* 041: „Über“ ist ein Footer-Link mit Dialog */
  window.reviewFixtureBody=document.body.innerHTML;
 },screen);
 for(const name of ['i18n-data.js','i18n.js'])await page.addScriptTag({path:join(root,'internal/webui/assets',name)});
 await page.evaluate(()=>StromeaI18n.start());}
 await page.evaluate(({language,screen})=>{document.querySelectorAll('style[data-review-mutation]').forEach(n=>n.remove());document.body.innerHTML=window.reviewFixtureBody;document.querySelector('#step-2').hidden=screen!=='preparation';document.querySelector('#step-1').hidden=screen==='preparation';document.querySelector('#compare-block').hidden=screen!=='pairing';StromeaI18n.setLanguage(language);},{language,screen});
}
async function verdict(page,kind){
 if(kind==='label'||kind==='select')return page.locator(kind==='label'?'label[for=interface]':'#interface').evaluate(readableContent);
 if(kind==='checkbox')return {readable:(await page.evaluate(forbiddenConfirmationLabels)).length===0};
 if(kind==='copy')return {readable:(await page.evaluate(visibleCopy036c)).passed};
 if(kind==='footer')return page.evaluate(()=>{const a=document.querySelector('footer #about-open')?.getBoundingClientRect(),g=document.querySelector('footer a[href*="github.com"]')?.getBoundingClientRect();return {readable:!!a&&!!g&&(a.x>g.x||a.y>g.y)&&document.querySelector('footer #about-open').previousElementSibling?.matches('a[href*="github.com"]'),about:a?.x,github:g?.x};}); // 041: about link right of the GitHub link
 return page.evaluate(()=>{const a=document.querySelector('#copy-ski').getBoundingClientRect(),b=document.querySelector('#copy-device-ski').getBoundingClientRect();const grids=['#device','#own-identity'].map(s=>getComputedStyle(document.querySelector(s)).gridTemplateColumns.split(/\s+/).length);return {readable:Math.abs(a.x-b.x)<=2&&grids.every(n=>n===2),a:a.x,b:b.x,grids};});
}
const probes=[
 ['hidden-checkbox-visible-label','checkbox'],['switch-confirmation','checkbox'],['radio-confirmation','checkbox'],['allowed-mode-radios','checkbox','start',true],['allowed-profile-radios','checkbox','preparation',true],
 ['text-fill-transparent','label'],['mask-transparent','label'],['overlay-label-no-pointer','label'],['overlay-select-no-pointer','select'],['overlay-pseudo-no-pointer','label'],
 ['allowed-label-extended','checkbox','preparation'],['identity-confirm-pairing','checkbox','pairing'],
 ['aria-hidden-span','label','start',true],['legitimate-null','copy','start',true],['legitimate-fachbetriebe','copy','start',true],
 ['null-token','copy'],['null-field','copy'],['legacy-company-label','copy','preparation'],
 ['footer-old-edge','footer'],['copy-old-column','icons','pairing'],
];
try{
 for(const theme of ['light','dark'])for(const width of [1440,820,390]){
  const context=await browser.newContext({viewport:{width,height:1000},colorScheme:theme,reducedMotion:'reduce'}),page=await context.newPage();await page.route('**/*',r=>r.abort());
  for(const language of ['de','en','pl','ru','uk','tr','ro']){
   for(const [probe,kind,screen='start',positive=false] of probes){
    await reset(page,language,screen);const before=await verdict(page,kind);if(kind==='select')before.legacyIsVisible=await page.locator('#interface').isVisible();
    await page.evaluate(probe=>{
     const label=document.querySelector('label[for=interface]'),select=document.querySelector('#interface');
     if(['hidden-checkbox-visible-label','switch-confirmation','radio-confirmation'].includes(probe)){const n=document.createElement('div');n.innerHTML=probe==='hidden-checkbox-visible-label'?'<label><input type="checkbox" style="opacity:0">☐ Ich habe die Gerätekennung verglichen.</label>':probe==='switch-confirmation'?'<button role="switch" aria-checked="false">Gerätekennung verglichen</button>':'<fieldset><legend>Gerätekennung verglichen?</legend><label><input type="radio" name="confirm">Ja</label><label><input type="radio" name="confirm">Nein</label></fieldset>';document.querySelector('#step-1').append(n);}
     if(probe==='text-fill-transparent')label.style.setProperty('-webkit-text-fill-color','transparent','important');
     if(probe==='mask-transparent')for(const name of ['mask-image','-webkit-mask-image'])label.style.setProperty(name,'linear-gradient(transparent,transparent)','important');
     if(probe.startsWith('overlay-')){
      const target=probe==='overlay-select-no-pointer'?select:label;target.scrollIntoView({block:'center',behavior:'instant'});const b=target.getBoundingClientRect();
      if(probe==='overlay-pseudo-no-pointer'){const host=label.parentElement;host.id='mutation-overlay-host';const style=document.createElement('style');style.dataset.reviewMutation='true';style.textContent='#mutation-overlay-host::after{content:"";position:fixed;z-index:999999;left:'+b.left+'px;top:'+b.top+'px;width:'+b.width+'px;height:'+b.height+'px;background:#fff;pointer-events:none}';document.head.append(style);}
      else{const cover=document.createElement('div');cover.style.cssText='position:fixed;z-index:999999;left:'+b.left+'px;top:'+b.top+'px;width:'+b.width+'px;height:'+b.height+'px;background:#fff;pointer-events:none';document.body.append(cover);}
     }
     if(probe==='allowed-label-extended')document.querySelector('label:has(#sign-inspector)>span').prepend(document.createTextNode('Ich habe die Gerätekennung verglichen. '));
     if(probe==='identity-confirm-pairing'){const n=document.createElement('label');n.className='identity-check';n.innerHTML='<input id="identity-confirm" type="checkbox"><span>Ich habe die Gerätekennung verglichen.</span>';document.querySelector('#copy-device-ski').after(n);}
     if(probe==='aria-hidden-span'){const span=document.createElement('span');span.setAttribute('aria-hidden','true');while(label.firstChild)span.append(label.firstChild);label.append(span);}
     if(['legitimate-null','legitimate-fachbetriebe','null-token','null-field','legacy-company-label'].includes(probe)){const n=document.createElement(probe==='legacy-company-label'?'label':'p');n.dataset.userContent='true';n.textContent=({'legitimate-null':'Ohne Verbindung fällt die Leistung nicht auf null.','legitimate-fachbetriebe':'Für Elektro-Fachbetriebe.','null-token':'null','null-field':'Messwert: null','legacy-company-label':'Fachbetrieb'})[probe];document.body.append(n);}
     if(probe==='footer-old-edge')document.querySelector('footer').prepend(document.querySelector('#about-open'));
     if(probe==='copy-old-column')document.querySelector('#copy-device-ski').style.gridColumn='3';
    },probe);
    const during=await verdict(page,kind);if(kind==='select')during.legacyIsVisible=await page.locator('#interface').isVisible();await reset(page,language,screen);const after=await verdict(page,kind);
    const passed=before.readable&&after.readable&&(positive?during.readable:!during.readable);
    results.push({probe,kind,language,width,theme,expected:positive?'green':'red',actual:during.readable?'green':'red',passed,before,during,after});
   }
  }
  await context.close();
 }
}finally{await browser.close();await writeFile(join(out,'mutations.json'),JSON.stringify(results,null,2)+'\n');}
for(const r of results.filter(r=>!r.passed))console.error(JSON.stringify(r));
console.log('036d component probes: '+results.filter(r=>r.passed).length+'/'+results.length+' observed checks passed');
assert(results.length>0&&results.every(r=>r.passed),'036d mutation/counterexample findings; see mutations.json');
