'use strict';
const assert=require('node:assert/strict');
const g=require('../internal/webui/assets/guide.js');
assert.equal(g.minimum({mode:'direct',power:11000}),4200);
assert.equal(g.minimum({mode:'direct',power:15000}),6000);
assert.equal(g.minimum({mode:'ems',n:2}),7560);
for(const [n,gzf] of [[2,.8],[3,.75],[4,.7],[5,.65],[6,.6],[7,.55],[8,.5],[9,.45],[12,.45]])assert.equal(g.minimum({mode:'ems',n}),Math.round(4200+(n-1)*gzf*4200));
assert.equal(g.minimum({mode:'ems',n:3,wp:15000,ac:20000}),14300);
assert.equal(g.minimum({mode:'ems',n:1,wp:15000}),6000);
for(const input of [{mode:'ems',n:0},{mode:'ems',n:1.5},{mode:'direct',power:-1},{mode:'ems',n:2,wp:NaN}])assert.equal(g.minimum(input),null);
assert.equal(g.production(null),null);assert.equal(g.production(4000),null);assert.equal(g.production(-4000),2000);assert.equal(g.production(-1234.567),617.283);assert.equal(g.production(NaN),null);
assert.equal(g.budget({scope:'consumption',effect:120,outage:120,step:60,cleanup:30,minimum:7200,profile:'quick'}),1200);
assert.equal(g.budget({scope:'production',effect:120,outage:120,step:60,cleanup:30,minimum:7200,profile:'full'}),8400);
assert.equal(g.budget({scope:'both',effect:120,outage:120,step:60,cleanup:30,minimum:7200,profile:'full'}),16800);
for(const [seconds,text] of [[60,'1 Min.'],[89*60,'89 Min.'],[90*60,'1 h 30 min'],[120*60,'2 h'],[140*60,'2 h 20 min'],[280*60,'4 h 40 min']])assert.equal(g.duration(seconds),text);
console.log('Guide 020: calculator, production suggestion and profile budgets passed');

// Copy is authored in seven local dictionaries, including singular imperatives.
const fs=require('node:fs');
for(const lang of ['de','en','pl','ru','uk','tr','ro']){
 const dict=JSON.parse(fs.readFileSync('internal/webui/assets/i18n/'+lang+'.json','utf8'));
 for(const key of ['Vorschlag aus Messung übernommen','Die Mindestdauer von','h wird abgewartet.','h','min'])assert.ok(dict[key],lang+': '+key);
 const guideKey=Object.keys(dict).find(k=>k.startsWith('Wähle eine Grenze deutlich'));
 assert.ok(guideKey,lang+': informal source');
 assert.ok(!Object.values(dict).some(v=>/Wählen Sie|Выберите|Виберіть|seçin/.test(v)),lang+': no formal guide imperative');
 const heartbeatKey=Object.keys(dict).find(k=>k.startsWith('Beobachtung: 120 s ab lokalem Stopp'));
 assert.ok(dict[heartbeatKey],lang+': new stop reference');
}
assert.equal(g.budget({scope:'both',effect:120,outage:120,step:60,cleanup:30,minimum:7200,profile:'full',measurable:false}),1680);
