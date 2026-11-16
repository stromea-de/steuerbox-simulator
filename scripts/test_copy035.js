'use strict';
const fs=require('fs'),assert=require('node:assert/strict');
const {duplicates035,forbidden035,forbiddenCopy035}=require('./copy035.cjs');
for(const word of ['Übungsmodussmodus','(EMS) (EMS)','mode mode','trybtryb','режимрежим','modumodu','modulmodul'])assert(duplicates035(word).length,word);
for(const word of ['Mindestdauer','simulated readings','уровень мощности','Sygnał podtrzymania','mesurements','feed-in in the test scope'])assert.equal(duplicates035(word).length,0,word);
for(const bad of ['frisch','quittiert','complete','research 018 Q4/Q5','Langtest','Wiederkehr'])for(const lang of ['de','en','pl','ru','uk','tr','ro'])assert(forbiddenCopy035(bad,lang),lang+': detector counterexample '+bad);
for(const [lang,bad] of [['en','fresh reading'],['pl','świeży pomiar'],['ru','свежее измерение'],['uk','свіжий вимір'],['tr','taze ölçüm'],['ro','măsurare proaspătă']])assert(forbiddenCopy035(bad,lang),'localized counterexample '+lang);
for(const lang of ['de','en','pl','ru','uk','tr','ro']){
 const d=JSON.parse(fs.readFileSync('internal/webui/assets/i18n/'+lang+'.json'));
 for(const [key,value] of Object.entries(d)){
  assert.deepEqual(duplicates035(value),[],lang+': duplicate '+key+': '+value);
  assert(!forbiddenCopy035(value,lang),lang+': forbidden display wording: '+value);
 }
 for(const [key,acronym] of [['Gerätekennung (SKI)','SKI'],['Bezug begrenzen (§14a, LPC)','LPC'],['Einspeisung begrenzen (LPP)','LPP'],['Leistung messen (MPC)','MPC'],['Notbetrieb (Failsafe)','Failsafe'],['Lebenszeichen (Heartbeat)','Heartbeat']])assert(d[key]?.includes(acronym) && /^[^()]*\p{L}[^()]*\([^()]+\)$/u.test(d[key]) && d[key].slice(d[key].indexOf('(')).includes(acronym),lang+': everyday term followed by parenthesized '+key);
}
console.log('035: no meta phrases/duplicated words, familiar terms with technical abbreviations in all seven languages');
