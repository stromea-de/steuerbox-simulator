const fs=require('fs'),assert=require('assert/strict');
const read=n=>fs.readFileSync('internal/webui/assets/'+n,'utf8');
assert(!read('app.js').includes('SKI am Gerät vergleichen.'));
assert(!read('result.js').includes('<th>Phase</th>'));
assert(!read('index.html').includes('keine Norm'));
assert(read('protokoll.html').includes('.Report.FunctionCounts'));
for(const lang of ['de','en','pl','ru','uk','tr','ro']){
 const d=JSON.parse(read('i18n/'+lang+'.json'));
 const k='PDF konnte nicht erstellt werden. Erneut versuchen oder Messdaten exportieren.';
 assert(d[k] && !d[k].includes('JSON'),lang+': JSON copy regression');
}
console.log('034 re-review: regression messages and fallback projection passed');

const vm=require('vm'),ctx={window:{}};vm.createContext(ctx);vm.runInContext(read('result.js'),ctx);
for(const f of JSON.parse(fs.readFileSync('scripts/fixtures/functions034.json','utf8'))){assert.deepEqual(JSON.parse(JSON.stringify(ctx.window.StromeaResult.functionCounts(f.report))),f.counts,f.name);assert.deepEqual(JSON.parse(JSON.stringify(ctx.window.StromeaResult.formMatrix(f.report))),f.matrix,f.name);}
console.log('034: 24 shared browser/server projection fixtures passed');

// Follow-up review: retain evidence but keep the main result concise.
assert(!/<p id="result-counts"><\/p><p id="result-abort"/.test(read('result.js')), '034 follow-up: counts belong in details');
assert(!read('index.html').includes('Verbindungsaufbau dauert bis zu 15 s.'), '034 follow-up: no conflicting setup deadline');
assert(!read('app.js').includes("shortModel.length<=12?shortModel"), '034 follow-up: device row is never a model label');
