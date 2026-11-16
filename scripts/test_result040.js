// 040 F-09 / R040-04: the device-mode result tells the tester to remove the pairing (heartbeat keeps running
// until the simulator ends; afterwards the device may stay in fallback mode). Practice results hide it.
const fs=require('fs'),assert=require('assert');
const src=fs.readFileSync('internal/webui/assets/result.js','utf8');
const text='Entferne nach der Prüfung die Kopplung am Gerät. Nach dem Trennen kann das Gerät bis zur Mindestdauer im Notbetrieb bleiben.';
const m=src.match(/<p class="help" id="unpair-reminder">([^<]*)<\/p>/);
assert(m,'unpair reminder paragraph exists');assert.equal(m[1],text,'unpair reminder has its full text');
assert(/\$\('unpair-reminder'\)\.hidden=r\.mode!=='onsite'/.test(src),'shown only for device reports');
for(const lang of ['de','en','pl','ru','uk','tr','ro']){const d=JSON.parse(fs.readFileSync('internal/webui/assets/i18n/'+lang+'.json'));assert(d[text]&&d[text].length>40,lang+': unpair reminder translated');}
console.log('PASS 040 unpair reminder (device only, 7 languages)');
