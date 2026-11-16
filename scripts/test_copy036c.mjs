#!/usr/bin/env node
// Mandatory coverage check of actual browser captures, never dictionary contents.
import assert from 'node:assert/strict';
import {visibleCopy036c} from './copy036c.mjs';
import {readFile,access} from 'node:fs/promises';
const data=JSON.parse(await readFile('artifacts/036/dom036.json','utf8'));
const langs=['de','en','pl','ru','uk','tr','ro'];
let prototype=false;try{await access('scripts/build-prototyp.py');prototype=true;}catch(e){if(e.code!=='ENOENT')throw e;}
for(const view of data)assert.deepEqual(visibleCopy036c(view.dom).bad,[],JSON.stringify({name:view.name,language:view.language,width:view.width,theme:view.theme}));
for(const language of langs)for(const width of [1440,1280,820,390])for(const theme of ['light','dark'])for(const name of ['start','network-single-adapter','network-no-adapter','devices','pairing','reports-empty','reports-full','report-empty','help','profile','preparation','test-types','running','result','result-open-evidence','saved-result','saved-sidebar'])assert(data.some(v=>v.name===name&&v.language===language&&v.width===width&&v.theme===theme),'missing copy view '+[name,language,width,theme]);
if(prototype)for(const language of langs)for(const width of [1440,1280,820,1920,390])for(const theme of ['light','dark'])for(const name of ['start','pairing','reports-full','reports-empty','saved-result','help','profile','preparation','running','result','result-open-evidence'])assert(data.some(v=>v.name==='prototype-'+name&&v.language===language&&v.width===width&&v.theme===theme),'missing prototype copy view '+[name,language,width,theme]);
console.log('036c visible-copy coverage: '+data.length+' actual captures; all languages, widths, themes, app'+(prototype?' and private prototype':''));
