// Recorded-data projection regression; no browser/network required.
const fs=require('fs'),vm=require('vm'),assert=require('assert');
const sandbox={window:{}};vm.createContext(sandbox);vm.runInContext(fs.readFileSync('internal/webui/assets/result.js','utf8'),sandbox);
const view=sandbox.window.StromeaResult;
const report={mode:'onsite',schemaVersion:1,config:{scope:'consumption',testProfile:'quick',lpc:4200,tolerance:50},steps:[{name:'Verbindung',passed:true},{name:'Ausgangsmessung MPC',passed:true,watts:0},{name:'LPC Grenze + Quittung',passed:false},{name:'LPC Grenze + Quittung',passed:true},{name:'LPC gemessene Wirkung',passed:true},{name:'Grenzen aufheben',passed:true}]};
const before=JSON.stringify(report),rows=view.formMatrix(report);
assert.equal(rows.length,4);assert.equal(rows[0].title,'Grenze angenommen');assert.equal(rows[0].lpc,'failed');assert.equal(rows[1].lpc,'not_checked');assert(rows.every(r=>r.lpp==='excluded'));
assert.equal(JSON.stringify(report),before,'source evidence remains unchanged');
report.config.testProfile='full';assert.equal(view.formMatrix(report).length,5);
report.config.scope='both';report.config.testProfile='quick';assert.equal(view.formMatrix(report).length,4);assert.equal(view.formMatrix(report)[0].lpp,'open');
assert.equal(view.workflow(report).length,2);assert(view.workflow(report).every(r=>!r.name.startsWith('LPC')));
console.log('PASS 024c result grouping, low-load migration, retry failure, scope, long test, workflow, source preservation');
