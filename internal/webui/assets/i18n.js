/* Local, offline presentation. Backend values and the German PDF are untouched. */
(() => {
 'use strict';
 const dictionaries=window.StromeaDictionaries;
 const names={de:'Deutsch',en:'English',pl:'Polski',ru:'Русский',uk:'Українська',tr:'Türkçe',ro:'Română'};
 let lang='de',technical=false,observer=null;
 try {lang=localStorage.getItem('stromea-language') || (navigator.language || 'de').split('-')[0];technical=localStorage.getItem('stromea-technical')==='true';}catch {lang=(navigator.language || 'de').split('-')[0];}
 if(!names[lang])lang='de';
 let linkedDocuments=[];
 const texts=new WeakMap(),attributes=new WeakMap(),cache=new Map();
 const escape=s=>s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');
 const phrases=Object.keys(dictionaries.de).sort((a,b)=>b.length-a.length);
 const pattern=new RegExp('(^|[^\\p{L}\\p{N}])('+phrases.map(escape).join('|')+')(?![\\p{L}\\p{N}])','gu');
 function t(value) {
  if(!value)return value;
  const key=lang+'\0'+value;if(cache.has(key))return cache.get(key);
  const dict=dictionaries[lang];
  const result=dict[value] || value.replace(pattern,(_,prefix,phrase)=>prefix+dict[phrase]);cache.set(key,result);return result;
 }
 function translateText(node) {
  if(node.parentElement?.closest('script,style,code,pre,[data-user-content],.raw-technical,[data-i18n-rendered]'))return;
  let record=texts.get(node);
  if(!record || node.nodeValue!==record.output)record={source:node.nodeValue,output:node.nodeValue};
  record.output=t(record.source);if(node.nodeValue!==record.output)node.nodeValue=record.output;texts.set(node,record);
 }
 function walk(root=document) {
  observer?.disconnect();
  const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);while(walker.nextNode())translateText(walker.currentNode);
  const all=root.querySelectorAll?.('[aria-label],[data-tip],[title],[placeholder],img[alt]') || []; // 039 (C-02): alt text too
  for(const el of all) {
   if(el.closest('[data-user-content]'))continue;
   const records=attributes.get(el) || {};
   for(const attr of ['aria-label','data-tip','title','placeholder','alt']) {
    if(!el.hasAttribute(attr))continue;const value=el.getAttribute(attr);let record=records[attr];
    if(!record || record.output!==value)record={source:value};record.output=t(record.source);el.setAttribute(attr,record.output);records[attr]=record;
   }
   attributes.set(el,records);
  }
  observer?.observe(document.documentElement,{childList:true,characterData:true,subtree:true,attributes:true,attributeFilter:['aria-label','data-tip','title','placeholder','alt']});
 }
 function preferences(doc) {
  doc.documentElement.lang=lang;doc.documentElement.classList.toggle('show-technical',technical);
  doc.querySelectorAll('.technical').forEach(el=>el.dataset.technicalLabel=t('Technik'));doc.querySelectorAll('details[data-technical]').forEach(el=>el.open=technical);
  const select=doc.getElementById('language');if(select)select.value=lang;
  const toggle=doc.getElementById('technical-toggle');if(toggle)toggle.checked=technical;
  doc.querySelectorAll('[data-number]').forEach(el=>{const value=Number(el.dataset.number);if(Number.isFinite(value))el.textContent=value.toLocaleString(locale(),{maximumFractionDigits:3,useGrouping:true})/* 040 F-12: same grouping as the result view */;});
 }
 function apply() {
  preferences(document);for(const doc of linkedDocuments){preferences(doc);walk(doc);}
  document.documentElement.lang=lang;document.documentElement.classList.toggle('show-technical',technical);
  document.querySelectorAll('details[data-technical]').forEach(el=>el.open=technical);
  const label=t('Technik');document.querySelectorAll('.technical').forEach(el=>el.dataset.technicalLabel=label);
  walk();
 }
 function setLanguage(code) {
  if(!names[code])return;lang=code;try{localStorage.setItem('stromea-language',lang);}catch{}
  const select=document.getElementById('language');if(select)select.value=lang;apply();window.dispatchEvent(new Event('stromea-language'));walk();
 }
 function start() {
  const select=document.getElementById('language');
  if(select){select.replaceChildren();for(const [value,label] of Object.entries(names)){const option=document.createElement('option');option.value=value;option.textContent=label;select.append(option);}select.value=lang;select.addEventListener('change',()=>setLanguage(select.value));}
  const toggle=document.getElementById('technical-toggle');if(toggle){toggle.checked=technical;toggle.addEventListener('change',()=>{technical=toggle.checked;try{localStorage.setItem('stromea-technical',String(technical));}catch{}apply();});}
  // Raw diagnostics remain explicitly labelled. Essential result reasons stay visible.
  for(const id of ['interface-detail','pair-error-detail','config-error-detail','run-error-detail']){
   const el=document.getElementById(id);if(el)el.classList.add('technical','raw-technical');
  }
  observer=new MutationObserver(()=>{document.querySelectorAll('.technical').forEach(el=>el.dataset.technicalLabel=t('Technik'));walk();});
  apply();
 }
 const locale=()=>({de:'de-DE',en:'en-GB',pl:'pl-PL',ru:'ru-RU',uk:'uk-UA',tr:'tr-TR',ro:'ro-RO'})[lang];
 function attachDocument(doc) {
  linkedDocuments=[doc];const select=doc.getElementById('language');
  if(select){select.replaceChildren();for(const [value,label] of Object.entries(names)){const option=doc.createElement('option');option.value=value;option.textContent=label;select.append(option);}select.addEventListener('change',()=>setLanguage(select.value));}
  const toggle=doc.getElementById('technical-toggle');if(toggle)toggle.addEventListener('change',()=>{technical=toggle.checked;try{localStorage.setItem('stromea-technical',String(technical));}catch{}apply();});
  apply();
 }
 function sourceText(el){if(!el)return '';const walker=document.createTreeWalker(el,NodeFilter.SHOW_TEXT);let n,value='';while(n=walker.nextNode())value+=(texts.get(n)?.source || n.nodeValue)+' ';return value.replace(/\s+/g,' ').trim();}
 window.StromeaI18n={sourceText,t,start,setLanguage,walk,attachDocument,sourceAttribute:(el,name)=>attributes.get(el)?.[name]?.source || el.getAttribute(name),language:()=>lang,locale};
 if(!document.getElementById('form'))document.addEventListener('DOMContentLoaded',start);
})();
// 039 (Nutzer 3): Mobile Seitenleisten richten sich nach dem sichtbaren Bereich (Tastatur, Werkzeugleisten).
(() => {
 'use strict';
 const vv=window.visualViewport;if(!vv)return;
 const sync=()=>{const root=document.documentElement.style;root.setProperty('--vv-height',Math.round(vv.height)+'px');root.setProperty('--vv-top',Math.round(vv.offsetTop)+'px');};
 vv.addEventListener('resize',sync);vv.addEventListener('scroll',sync);sync();
})();
