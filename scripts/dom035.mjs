// Capture rendered copy and accessible field descriptions, never dictionary JSON.
export function captureDOM035(){
 const visible=n=>n.getClientRects().length && (!n.checkVisibility||n.checkVisibility()) && getComputedStyle(n).visibility==='visible' && !n.closest('[hidden],script,style');
 const fields=[...document.querySelectorAll('input:not([type=hidden]),textarea,select,button[aria-describedby]')].filter(visible).map(n=>({id:n.id,label:n.labels?.[0]?.innerText?.trim()||n.getAttribute('aria-label')||n.innerText?.trim()||'',value:n.type==='password'?'[nicht erfasst]':n.value||'',placeholder:n.getAttribute('placeholder')||'',descriptionIds:(n.getAttribute('aria-describedby')||'').split(/\s+/).filter(Boolean),description:(n.getAttribute('aria-describedby')||'').split(/\s+/).filter(Boolean).map(id=>document.getElementById(id)?.textContent?.trim()||'').join(' '),options:n.tagName==='SELECT'?[...n.options].map(o=>o.textContent.trim()):undefined}));
 const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT),lines=[];let node,last;
 while(node=walker.nextNode()){
  const el=node.parentElement;if(!el || !visible(el) || el.closest('script,style,option,.field-description,[hidden]') || !node.nodeValue.trim())continue;
  let clipped=false;for(let p=el;p && p!==document.body;p=p.parentElement)if(getComputedStyle(p).clipPath==='inset(50%)')clipped=true;if(clipped)continue;
  const host=el.closest('h1,h2,h3,h4,p,button,a,label,summary,legend,dt,dd,th,td,li,.ski-field,.eyebrow,.trust-state,.device-copy,.step-title,text,header,footer')||el;
  if(last?.host===host)last.parts.push(node.nodeValue);else{last={host,parts:[node.nodeValue]};lines.push(last);}
 }
 return {labels:[...document.querySelectorAll('dt,label,th')].filter(visible).map(n=>n.innerText.trim()),text:lines.map(l=>l.parts.join(' ').replace(/\s+/g,' ').trim()).filter(Boolean).join('\n'),fields};
}
