// Browser-side text audit. No network or API access.
export function auditDOM(strictTable=false){
 const german=[],clipped=[];let nodes=0;
 const visuallyHidden=el=>{for(let n=el;n && n!==document.body;n=n.parentElement){const s=getComputedStyle(n),r=n.getBoundingClientRect();if(s.clipPath==='inset(50%)' || (r.width<=1 && r.height<=1 && ['hidden','clip'].includes(s.overflow)))return true;}return false;};
 const re=/Prüf|Gerät|Einspeis|Bezug|Übung|Zurück|Messwert|Freigabe|Notbetrieb|bestanden|ungeprüft|gewählten|Ausstehend|Zeitlimit|Leistungs|Rückkehr|Wiederkehr|Unterschrift|aufgehoben|Einstellungen|geprüft|Sonderfälle|Drosselung|Quellen|Fundstellen|Mindestdauer|Grenzwerte|Höchst|Zeitplanung/;
 // In addition to stems, match complete untranslated source phrases/words.
 const language=document.documentElement.lang,dicts=window.StromeaDictionaries;
 const escape=s=>s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');
 const phrases=dicts?Object.entries(dicts.de).filter(([k,v])=>dicts[language]?.[k]!==v && /\p{L}{5}/u.test(v)).map(([,v])=>v):[];
 const sourceRE=phrases.length?new RegExp('(^|[^\\p{L}\\p{N}])(?:'+[...new Set(phrases)].sort((a,b)=>b.length-a.length).map(escape).join('|')+')(?=$|[^\\p{L}\\p{N}])','u'):null;
 const isGerman=text=>{const copy=text.replaceAll('Stromea Steuerbox-Simulator','');return language!=='de' && (re.test(copy)||sourceRE?.test(copy));};
 // Native option elements have no useful text rectangles. Audit their parent select.
 for(const select of document.querySelectorAll('select:not(#language)')){
  if(!select.getClientRects().length || (select.checkVisibility && !select.checkVisibility()) || select.closest('[hidden],[data-user-content]') || getComputedStyle(select).visibility!=='visible')continue;
  for(const option of select.options)if(isGerman(option.textContent.trim()))german.push({element:select.id||'SELECT',kind:'option',selected:option.selected,text:option.textContent.trim()});
 }
 // Layout boxes matter even when overflow is visible and the page still fits.
 const boundsCache=new WeakMap();
 function wordBounds(el){
  if(boundsCache.has(el))return boundsCache.get(el);
  const boxes=[];
  for(let parent=el;parent && parent!==document.body;parent=parent.parentElement){
   const style=getComputedStyle(parent),box=parent.getBoundingClientRect();
   if(!['inline','contents'].includes(style.display) && box.width>1){
    boxes.push({element:parent.id||parent.tagName,left:box.left+parseFloat(style.borderLeftWidth||0)+parseFloat(style.paddingLeft||0),right:box.right-parseFloat(style.borderRightWidth||0)-parseFloat(style.paddingRight||0)});
   }
   // Popovers have their own containing box; horizontal scroll areas own their extent.
   if(['absolute','fixed'].includes(style.position) || ['auto','scroll'].includes(style.overflowX))break;
  }
  boundsCache.set(el,boxes);return boxes;
 }
 const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);let node;
 while(node=walker.nextNode()){
  const el=node.parentElement;if(!el || visuallyHidden(el) || el.closest('script,style,option,[hidden],.raw-technical,[data-user-content]') || !el.getClientRects().length || (el.checkVisibility && !el.checkVisibility()) || getComputedStyle(el).visibility!=='visible' || !node.textContent.trim())continue;
  nodes++;const text=node.textContent.trim();
  // Text ranges catch clipping at fixed-size ancestors, beyond element scrollWidth.
  if(el.namespaceURI==='http://www.w3.org/1999/xhtml'){
   const range=document.createRange();range.selectNodeContents(node);
   for(const box of range.getClientRects())for(let parent=el;parent && parent!==document.body;parent=parent.parentElement){
    const ps=getComputedStyle(parent),pr=parent.getBoundingClientRect();
    if(['hidden','clip'].includes(ps.overflowX) && (box.left<pr.left-2 || box.right>pr.right+2)){clipped.push({element:parent.id||parent.tagName,kind:'text-range',text:text.slice(0,120)});break;}
   }
  }
  // A word may use native hyphenation, but arbitrary CSS breaks must not split it.
  if(el.namespaceURI==='http://www.w3.org/1999/xhtml' && !el.closest('pre,code,.mono,.raw-technical,[data-user-content],select,textarea')){
   const style=getComputedStyle(el);
   {
    for(const match of node.textContent.matchAll(/[\p{L}\p{M}]{4,}/gu)){
     const range=document.createRange();range.setStart(node,match.index);range.setEnd(node,match.index+match[0].length);
     const boxes=[...range.getClientRects()].filter(r=>r.width>0),lines=new Set(boxes.map(r=>Math.round(r.top)));
     const over=wordBounds(el).find(parent=>boxes.some(box=>box.left<parent.left-2 || box.right>parent.right+2));
     if(over)clipped.push({element:over.element,kind:'word-overflow',text:match[0],context:text.slice(0,120)});
     if(lines.size<2 || (style.overflowWrap==='normal' && !['break-all','break-word'].includes(style.wordBreak)))continue;
     let native=false;
     if(style.hyphens==='auto'){
      // Compare the first line with hyphenation alone, without emergency word wrapping.
      const probe=document.createElement('span');probe.lang=el.closest('[lang]')?.lang||language;probe.textContent=match[0];
      Object.assign(probe.style,{position:'fixed',left:'-10000px',top:'0',display:'block',font:style.font,letterSpacing:style.letterSpacing,width:Math.max(...boxes.filter(r=>Math.round(r.top)===Math.round(boxes[0].top)).map(r=>r.width))+2+'px',whiteSpace:'normal',overflowWrap:'normal',wordBreak:'normal',hyphens:'auto'});
      document.body.append(probe);const r=document.createRange();r.selectNodeContents(probe);native=new Set([...r.getClientRects()].map(b=>Math.round(b.top))).size>1;probe.remove();
     }
     if(!native)clipped.push({element:el.id||el.tagName,kind:'word-break',text:match[0],context:text.slice(0,120)});
    }
   }
  }
  if(isGerman(text))german.push({element:el.id||el.tagName,text:text.slice(0,180)});
 }
 for(const el of document.querySelectorAll('button,a,label,h1,h2,h3,h4,p,summary,th,td,dt,dd,span')){
  if(visuallyHidden(el) || el.closest('[hidden],script,style,.raw-technical,[data-user-content]') || !el.getClientRects().length || (el.checkVisibility && !el.checkVisibility()))continue;
  const s=getComputedStyle(el),r=el.getBoundingClientRect();if(r.width<1 || r.height<1 || s.visibility!=='visible')continue;
  if(strictTable && ['TH','TD'].includes(el.tagName)){
   const range=document.createRange();range.selectNodeContents(el);for(const box of range.getClientRects())if(box.right>r.right+2 || box.left<r.left-2)clipped.push({element:el.tagName,kind:'table-text-range',text:el.textContent.trim().slice(0,120)});
  }
  if(el.scrollWidth>el.clientWidth+2 && ['hidden','clip'].includes(s.overflowX))clipped.push({element:el.id||el.tagName,text:el.textContent.trim().slice(0,120),width:r.width,scroll:el.scrollWidth});
  if(el.scrollHeight>el.clientHeight+2 && ['hidden','clip'].includes(s.overflowY) && s.whiteSpace!=='nowrap')clipped.push({element:el.id||el.tagName,kind:'vertical',text:el.textContent.trim().slice(0,120)});
 }
 return {nodes,german,clipped,horizontal:document.documentElement.scrollWidth>innerWidth+1};
}
