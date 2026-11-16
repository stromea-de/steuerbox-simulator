// Offline runtime initialization; no DOM service or translation provider.
'use strict';
const fs=require('fs'),vm=require('vm'),assert=require('assert/strict');
function load(browser,stored,blocked=false){const ctx=vm.createContext({navigator:{language:browser},localStorage:{getItem:key=>{if(blocked)throw Error('blocked');return key==='stromea-language'?stored:null;}},document:{getElementById:()=>({})}});ctx.window=ctx;for(const name of ['i18n-data.js','i18n.js'])vm.runInContext(fs.readFileSync('internal/webui/assets/'+name,'utf8'),ctx);return ctx.StromeaI18n;}
assert.equal(load('pl-PL',null).language(),'pl');assert.equal(load('fr-FR',null).language(),'de');assert.equal(load('en-US','ru').language(),'ru');assert.equal(load('tr-TR',null,true).language(),'tr');
const en=load('en-GB',null);assert.equal(en.t('Nicht verbunden'),'Not connected');assert.equal(en.t('Prüfung läuft… Schritt 1 von 16'),'Test running… step 1 of 16');assert.equal(en.t('Einspeisung: Grenze setzen · Nicht bestanden'),'Feed-in: Set limit · Failed');assert.equal(en.t('error-code-0815'),'error-code-0815');
for(const code of ['de','en','pl','ru','uk','tr','ro'])assert.equal(load(code,null).language(),code);
console.log('016: browser locale, storage, blocked storage, fallback and dynamic dictionary phrases passed');
// Unicode letters must protect words from accidental unit substitutions.
assert.equal(load('tr',null).t('dahili simülasyon daha hızlı'),'dahili simülasyon daha hızlı');
