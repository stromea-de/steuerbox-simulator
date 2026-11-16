// 042 (UX-S42, Nutzer 2026-11-14): the result title names the checked scope, the PDF title too, and the
// failsafe hint says what to do. Same rule in Go (inspection.ResultTitle) and in the browser (StromeaResult).
const fs=require('fs'),vm=require('vm'),assert=require('assert');
const dicts=Object.fromEntries(['de','en','pl','ru','uk','tr','ro'].map(l=>[l,JSON.parse(fs.readFileSync('internal/webui/assets/i18n/'+l+'.json','utf8'))]));
// R042-03: the real i18n.js (bundled dictionaries, phrase replacement) per language, no DOM needed for t().
const data=fs.readFileSync('internal/webui/assets/i18n-data.js','utf8'),i18n=fs.readFileSync('internal/webui/assets/i18n.js','utf8'),resultSource=fs.readFileSync('internal/webui/assets/result.js','utf8');
const contexts={};
function contextFor(l){
 if(contexts[l])return contexts[l];
 const c={navigator:{language:l},localStorage:{getItem:k=>k==='stromea-language'?l:null,setItem(){}},document:{getElementById:()=>({}),addEventListener(){}}};c.window=c;
 vm.createContext(c);vm.runInContext(data,c);vm.runInContext(i18n,c);
 vm.runInContext(resultSource,c);assert.equal(c.StromeaI18n.language(),l);return contexts[l]=c;
}
let lang='de';
const context={get window(){return contextFor(lang).window;}};
const title=(config,outcome,abortReason)=>context.window.StromeaResult.outcomeTitle({config,abortReason},outcome);
const cases=[
 [{scope:'consumption',testProfile:'quick'},'Bestanden','','LPC-Funktionsprüfung bestanden'],
 [{scope:'production',testProfile:'quick'},'Bestanden','','LPP-Funktionsprüfung bestanden'],
 [{scope:'both',testProfile:'quick'},'Bestanden','','LPC/LPP-Funktionsprüfung bestanden'],
 [{scope:'both',testProfile:'full'},'Bestanden','','LPC/LPP-Langzeitprüfung bestanden'],
 [{scope:'consumption',longTest:true},'Bestanden','','LPC-Langzeitprüfung bestanden'],
 [{scope:'consumption',testProfile:'quick'},'Nicht bestanden','','LPC-Funktionsprüfung nicht bestanden'],
 [{scope:'production',testProfile:'full'},'Unvollständig','','LPP-Langzeitprüfung unvollständig'],
 [{scope:'consumption',testProfile:'quick'},'Unvollständig','connection_lost','LPC-Funktionsprüfung unvollständig'],
 [{scope:'both',testProfile:'quick'},'Unvollständig','user_cancel','LPC/LPP-Funktionsprüfung abgebrochen'],
 [{scope:'consumption',testProfile:'quick'},'Nicht bestanden','user_cancel','LPC-Funktionsprüfung nicht bestanden'],
 [{testProfile:'quick'},'Bestanden','','Funktionsprüfung bestanden'],
 [{},'Unvollständig','user_cancel','Unvollständig'],
];
for(const [config,outcome,abort,want] of cases)assert.equal(title(config,outcome,abort),want,JSON.stringify([config,outcome,abort]));
// Go uses the identical table (internal/inspection/title042_test.go); keep both in step.
const go=fs.readFileSync('internal/inspection/title042_test.go','utf8');
for(const [,,,want] of cases.slice(0,4))assert(go.includes('"'+want+'"'),'Go test covers '+want);
// All 7 languages: every title is translated, names the scope and never glues a German word to it.
for(lang of Object.keys(dicts)){
 for(const [config,outcome,abort,want] of cases.slice(0,10)){
  const text=title(config,outcome,abort),scope={consumption:'LPC',production:'LPP',both:'LPC/LPP'}[config.scope];
  assert(text.includes(scope)&&!text.includes('{umfang}'),lang+': '+text);
  if(lang!=='de')assert(!/prüfung|bestanden|unvollständig|abgebrochen/i.test(text),lang+' untranslated: '+text);
  // The DOM observer translates the text node again: the title must stay as it is (no glued phrase replacement).
  assert.equal(contextFor(lang).StromeaI18n.t(text),text,lang+': re-translation keeps the title');
  // Same test-type term as the rest of the UI (Auswahl „Funktionsprüfung“/„Langzeitprüfung“), compared by word stem.
  const full=config.testProfile==='full'||(!config.testProfile&&config.longTest),term=contextFor(lang).StromeaI18n.t(full?'Langzeitprüfung':'Funktionsprüfung');
  for(const word of term.toLocaleLowerCase(lang).split(/\s+/).filter(w=>w.length>3))assert(text.toLocaleLowerCase(lang).includes(word.slice(0,Math.min(word.length-2,7))),lang+': '+text+' uses the UI term '+term);
 }
}
lang='de';
// The web result hint for an unconfirmed restore equals the Go constant and is translated.
const hint='Wiederherstellung der Notbetriebswerte nicht bestätigt. Stelle am Gerät Leistungsgrenze und Mindestdauer für Kommunikationsausfall wieder wie vor der Prüfung ein.';
assert(fs.readFileSync('internal/webui/assets/result.js','utf8').includes("'"+hint+"'"),'result.js uses the new hint');
assert(fs.readFileSync('internal/inspection/presentation.go','utf8').includes('"'+hint+'"'),'Go FailsafeNote uses the same hint');
for(const l of Object.keys(dicts))assert(dicts[l][hint]&&(l==='de'||dicts[l][hint]!==hint),l+': hint translated');
for(const l of Object.keys(dicts))assert(!Object.keys(dicts[l]).some(k=>k.startsWith('Rückstellung der Notbetriebs-Werte')),l+': old hint removed');
console.log('PASS 042 Ergebnistitel mit Prüfumfang (LPC/LPP/LPC+LPP, 4 Zustände, 7 Sprachen) und Notbetriebs-Hinweis');
