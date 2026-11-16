#!/usr/bin/env node
// An actual E2E must reject the visible historical checkbox at its original place.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {readFile,writeFile,mkdir,open} from 'node:fs/promises';
import {resolve,join,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..'),out=resolve(root,'artifacts/036d/e2e-identity-confirm');await mkdir(out,{recursive:true});
assert(process.env.E2E_BINARY,'036d pairing mutation requires the freshly built E2E binary');
const log=await open(join(out,'run.log'),'w');let exit;
try{const child=spawn(process.execPath,['scripts/e2e_034.mjs','--browser=chrome','--case=hell-1440','--mutation=identity-confirm','--output='+out],{cwd:root,env:process.env,stdio:['ignore',log.fd,log.fd]});[exit]=await once(child,'exit');}finally{await log.close();}
const result=JSON.parse(await readFile(join(out,'e2e-chrome-hell-1440.json'),'utf8'));
const activated=result.actions.includes('036d identity-confirm visibly injected in compare/en');
const observed=exit===1&&!result.passed&&activated&&/compare en: forbidden-checkboxes/.test(result.failure||'')&&/identity-confirm/.test(result.failure||'');
await writeFile(join(out,'probe.json'),JSON.stringify({expected:'red',actualExitCode:exit,mutationVisible:activated,observedRejection:observed,failure:result.failure},null,2)+'\n');
assert(observed,'036d pairing mutation escaped or failed for an unrelated reason; see run.log and probe.json');
console.log('036d actual E2E mutation: identity-confirm visibly inserted at compare/en; rejected by the view audit (exit '+exit+')');
