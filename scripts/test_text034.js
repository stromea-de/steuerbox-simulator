// Copy and evidence presentation, entirely offline. No requests or host data.
'use strict';
const fs=require('fs'),vm=require('vm'),assert=require('assert/strict');
const base='internal/webui/assets/',read=name=>fs.readFileSync(base+name,'utf8');
const app=read('app.js'),html=read('index.html');
assert(!html.includes('id="capabilities"'),'034: redundant capability row must be removed');
assert(!html.includes('id="lpp-explanation"'),'034: one feed-in hint, detailed explanation in sidebar');
assert(!/Kein MPC:/.test(html),'034: measurement warning must use everyday language');
assert(html.includes('Prüfergebnis'),'034: outcome and PDF have different labels');
assert(html.includes('Text aus dem QR-Code hier einfügen'),'034: QR placeholder explains the input');
const sandbox={window:{}};vm.createContext(sandbox);vm.runInContext(read('result.js'),sandbox);
const view=sandbox.window.StromeaResult;
const report={mode:'onsite',schemaVersion:1,config:{scope:'both',testProfile:'quick'},steps:[
 {name:'LPC Grenze + Quittung',passed:false},{name:'LPC Grenze + Quittung',passed:true},
 {name:'LPP Grenze + Quittung',passed:true},{name:'Grenzen aufheben',passed:true}
]};
const snapshot=JSON.stringify(report),counts=view.functionCounts(report);
assert.deepEqual(JSON.parse(JSON.stringify(counts)),{passed:0,failed:1,notChecked:0,open:3});
assert.equal(JSON.stringify(report),snapshot,'034: counting must preserve all recorded evidence');
report.config.scope='consumption';assert.equal(Object.values(view.functionCounts(report)).reduce((a,b)=>a+b),4);
report.config.testProfile='full';assert.equal(Object.values(view.functionCounts(report)).reduce((a,b)=>a+b),5);
function node(){return {children:[],dataset:{},textContent:'',append(...items){this.children.push(...items);},remove(){},setAttribute(){}};}
const fields=Object.fromEntries(['diagnostics','scope-control','scope','measurement-warning','capability-error','device-icon','scope-lpc','scope-lpp','scope-lpc-reason','scope-lpp-reason'].map(id=>[id,node()]));
fields.scope.value='both';
const ctx=vm.createContext({real:true,scopeTouched:false,syncScope(){fields.scope.value=fields['scope-lpc'].checked?(fields['scope-lpp'].checked?'both':'consumption'):'production';},deviceInfo:{type:'HeatPump'},document:{createElement:node},$:id=>fields[id],applyScope(){}});
vm.runInContext(app.slice(app.indexOf('function typeLabel('),app.indexOf('function rememberedMode(')),ctx);
ctx.setCapabilities({lpc:true,lpp:true,mpc:false});
const detail=fields.diagnostics.children.at(-1);
assert.equal(detail.children[1].textContent,'Wärmepumpe');
assert.equal(detail.children[1].dataset.userContent,undefined,'034: known device types must remain translatable');
assert.equal(fields['measurement-warning'].hidden,false);
ctx.deviceInfo={type:'Generic'};ctx.setCapabilities({lpc:true,mpc:true});
assert(!fields.diagnostics.children.at(-1).children.some(n=>n.textContent==='Gerätetyp'),'035: missing device type is omitted rather than repeated');
assert.equal(fields['measurement-warning'].hidden,true);
assert(!app.includes("'○ ')+'Am Gerät freigegeben'"),'034: pending approval is a status, not a radio circle');
const languages=['de','en','pl','ru','uk','tr','ro'];
for(const lang of languages){
 const dict=JSON.parse(read('i18n/'+lang+'.json'));
 for(const key of ['Gerät verbinden','Prüfergebnis','Wartet auf Freigabe am Gerät…','Wie setzt sich die Prüfdauer zusammen?','EEBUS · LPC Technical Specification 1.0.0 · 14.10.2022 · §2.6.2.1, LPC-022/1–5 · geprüft 31.10.2026']){
  assert(dict[key],lang+': '+key);if(lang!=='de')assert.notEqual(dict[key],JSON.parse(read('i18n/de.json'))[key],lang+': translated copy');
 }
}
console.log('034: concise copy, sidebar types, honest function counts and seven dictionaries passed');
// Protocol identifiers are stable; only their presentation changes.
const renderer=read('result.js'),names=vm.createContext({isReal:()=>true});
vm.runInContext(renderer.slice(renderer.indexOf('function stepName('),renderer.indexOf('function evidenceReason(')),names);
assert.equal(names.stepName('LPC Heartbeat-Ausfall'),'Bezug: Lebenszeichen unterbrechen');
assert.equal(names.stepName('Heartbeat starten'),'Lebenszeichen senden');
const exact={mode:'onsite',schemaVersion:1,config:{scope:'consumption',testProfile:'quick',lpc:4200,tolerance:50},steps:[
 {name:'LPC Grenze + Quittung',passed:true},{name:'Ausgangsmessung MPC',passed:true,watts:0},
 {name:'LPC gemessene Wirkung',passed:true},{name:'LPC Heartbeat-Ausfall',passed:true},
 {name:'LPC Failsafe gemessene Wirkung',passed:true},{name:'LPC Vorzeitige Freigabe',passed:true},
 {name:'LPC Vorzeitige Rückkehr gemessene Wirkung',passed:true}
]};
assert.deepEqual(JSON.parse(JSON.stringify(view.functionCounts(exact))),{passed:3,failed:0,notChecked:1,open:0},'low baseline cannot turn into a passed function');
exact.steps.push({name:'LPC gemessene Wirkung',passed:false});
assert.deepEqual(JSON.parse(JSON.stringify(view.functionCounts(exact))),{passed:3,failed:1,notChecked:0,open:0},'any failed attempt remains visible');
for(const lang of languages){
 const dict=JSON.parse(read('i18n/'+lang+'.json'));
 for(const key of ['Prüfdauer: höchstens etwa {duration}.','Funktionen: {passed} bestanden, {failed} fehlgeschlagen, {notChecked} nicht geprüft, {open} ausstehend.']){
  assert.deepEqual((dict[key].match(/\{[^}]+\}/g)||[]).sort(),(key.match(/\{[^}]+\}/g)||[]).sort(),lang+': copy placeholders');
 }
}
// Review 034: result legend means "failed", not "not performed"; address stays informal singular.
{
 const d=l=>JSON.parse(read('i18n/'+l+'.json'));
 assert.equal(d('de')['fehlgeschlagen. Einstellungen prüfen und erneut testen.'][0],'f','034 review: failed step summary continues the sentence in lower case');
 for(const [l,bad] of [['pl',/wykonano/i],['ru',/выполнено/i],['uk',/виконано/i]])assert(!bad.test(d(l)['Fehlgeschlagen']),l+': failed must not read as not performed');
 const formal={ru:/(?:^|[\s,.])(Повторите|Введите|Сравните|Переподключитесь|дождитесь|начните)(?=[\s,.])/,uk:/(?:^|[\s,.])(Введіть|Порівняйте|почніть)(?=[\s,.])/,tr:/başlayın\./};
 for(const [l,re] of Object.entries(formal))for(const v of Object.values(d(l)))assert(!re.test(v),l+': informal singular address: '+v);
 assert(/StromeaGuide\.duration\(estimate,StromeaI18n\.locale\(\)\)\)\.replace\(\/\\\.\$\/,''\)/.test(app),'034 review: duration drops its own final period before the sentence period');
}
console.log('034: stable protocol names, low-load evidence, failure precedence and copy placeholders passed');
