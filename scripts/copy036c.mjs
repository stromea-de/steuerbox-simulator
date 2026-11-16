// The same classification runs on rendered DOM and on its captured inventory.
export function visibleCopy036c(snapshot){
 const chunks=[],add=(text,kind='text')=>chunks.push({text:String(text??''),kind});
 if(snapshot){
  add(snapshot.text);for(const label of snapshot.labels||[])add(label,'label');
  for(const field of snapshot.fields||[]){add(field.label,'label');for(const value of [field.value,field.placeholder,field.description,...(field.options||[])])add(value);}
 }else{
  const visible=n=>n.getClientRects().length&&n.checkVisibility({checkOpacity:true,checkVisibilityCSS:true})&&!n.closest('script,style,[hidden]');
  const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);let node;
  while(node=walker.nextNode())if(node.textContent.trim()&&visible(node.parentElement)&&!node.parentElement.closest('option,script,style'))add(node.textContent);
  for(const n of document.querySelectorAll('input:not([type=hidden]):not([type=password]),textarea,select'))if(visible(n))add(n.tagName==='SELECT'?[...n.options].map(o=>o.textContent).join('\n'):n.value);
  for(const n of document.querySelectorAll('[placeholder],[title],img[alt]'))if(visible(n))for(const attr of ['placeholder','title','alt'])if(n.hasAttribute(attr))add(n.getAttribute(attr));
  for(const n of document.querySelectorAll('dt,label,th'))if(visible(n))add(n.innerText,'label');
 }
 const bad=[];
 for(const {text,kind} of chunks){
  bad.push(...(text.match(/\b(?:undefined|NaN|TODO)\b|\[object Object\]|\{\{|\$\{/gu)||[]));
  // A natural-language zero is legitimate; an isolated/labelled null value is not.
  if(/(?:^|\n)\s*null\s*(?:$|\n)|:\s*null\s*(?:$|\n|[,;}\]])/u.test(text))bad.push('null');
  if(kind==='label'&&/^\s*Fachbetrieb(?:\s*\/\s*Unternehmen)?\s*:?\s*$/u.test(text))bad.push('Fachbetrieb');
 }
 return {passed:bad.length===0,bad:[...new Set(bad)]};
}
