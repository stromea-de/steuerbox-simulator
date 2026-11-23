// 236 (UX-S51): restoring the device's original values is shown without dialogs:
// a matrix row "Originalwerte wiederhergestellt" in device results and one quiet
// status line after an automatic restore on restart. 7 languages.
const fs=require('fs'),assert=require('assert'),vm=require('vm');
const app=fs.readFileSync('internal/webui/assets/app.js','utf8'),result=fs.readFileSync('internal/webui/assets/result.js','utf8'),html=fs.readFileSync('internal/webui/assets/index.html','utf8');
const keys=['Originalwerte wiederhergestellt','Originalwerte der unterbrochenen Prüfung werden wiederhergestellt …','Originalwerte von {device} wiederhergestellt und zurückgelesen.','Originalwerte von {device} noch nicht wiederhergestellt. Der Simulator versucht es vor der nächsten Prüfung an diesem Gerät erneut.'];
for(const lang of ['de','en','pl','ru','uk','tr','ro']){const d=JSON.parse(fs.readFileSync('internal/webui/assets/i18n/'+lang+'.json'));for(const k of keys){assert(d[k],lang+': '+k);if(k.includes('{device}'))assert(d[k].includes('{device}'),lang+': placeholder kept');}}
assert(/<p id="restore-status" class="help restore-status" role="status" hidden><\/p>/.test(html),'quiet status line, no dialog');
assert(!/<dialog[^>]*(restore|haftung|elektrofachkraft)/i.test(html),'no confirmation dialog');
assert(/r\.mode==='onsite' && touched\)\{[^]*?'Originalwerte wiederhergestellt'/.test(result),'row only for device results that changed values');
assert(result.indexOf("'Originalwerte wiederhergestellt'")<result.indexOf("title.textContent='Testgrenzen aufgehoben'"),'row stands before the release row (UX-S16 keeps release last)');
assert(/title\.textContent='Originalwerte wiederhergestellt'[^]*?r\.releaseVerified===false[^]*?r\.releaseVerified!==true\|\|!released/.test(result),'R3-04: row needs the read-back release like the PDF');
const fn=app.match(/function restoreNames\(pending,result\)\{.*\}/)[0]+'\n'+app.match(/function renderRestoreStatus\(s\)\{[^]*?\n\}/)[0];
const line={hidden:true,textContent:'',dataset:{},classList:{on:false,toggle(_,v){this.on=v;}}};
const ctx={$:()=>line,StromeaI18n:{t:s=>s}};vm.createContext(ctx);vm.runInContext(fn,ctx);
ctx.renderRestoreStatus({});assert(line.hidden,'nothing pending: hidden');
ctx.renderRestoreStatus({busy:true,kind:'restore',pendingRestores:[{device:'WP'}]});assert(!line.hidden&&/werden wiederhergestellt/.test(line.textContent)&&!line.classList.on);
ctx.renderRestoreStatus({pendingRestores:[{device:'WP'}],restoreResult:{ok:false,device:'WP'}});assert(/WP noch nicht wiederhergestellt/.test(line.textContent)&&line.classList.on,'failure stays visible');
ctx.renderRestoreStatus({pendingRestores:[],restoreResult:{ok:true,device:'WP'}});assert(/WP wiederhergestellt und zurückgelesen/.test(line.textContent)&&!line.classList.on);
ctx.renderRestoreStatus({busy:true,kind:'run',pendingRestores:[{device:'WP'}]});assert(line.hidden,'record of the running test is not an interrupted one');
ctx.renderRestoreStatus({pendingRestores:[],restoreResult:{ok:false,device:'kaputt.json'}});assert(!line.hidden&&/kaputt.json noch nicht/.test(line.textContent)&&line.classList.on,'unreadable record stays visible');
ctx.renderRestoreStatus({pendingRestores:[{device:'WP'},{device:'WR'},{device:'kaputt.json'}],restoreResult:{ok:false,device:'WP, kaputt.json'}});assert(/WP, WR, kaputt.json noch nicht/.test(line.textContent),'R4-05: every open record is named once');
assert(line.dataset.i18nRendered==='true'&&/addEventListener\('stromea-language',\(\)=>\{if\(restoreStatusState\)renderRestoreStatus/.test(app),'line is re-rendered on language change');
console.log('PASS 236 restore status (no dialog, matrix row, 7 languages)');
