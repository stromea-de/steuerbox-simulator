// Browser-side checks of readable content, independent of implementation IDs.
export function readableContent(el){
 // Scroll ordinary below-fold content into view without rescuing offscreen CSS.
 const oldX=scrollX;el.scrollIntoView({block:'center',inline:'nearest',behavior:'instant'});scrollTo({left:oldX,top:scrollY,behavior:'instant'});
 const r=el.getBoundingClientRect(),s=getComputedStyle(el),font=parseFloat(s.fontSize),reasons=[];
 const rgba=value=>{const v=value.match(/[\d.]+/g)?.map(Number)||[];return [v[0]||0,v[1]||0,v[2]||0,v.length>3?v[3]:1];};
 const over=(a,b)=>[...a.slice(0,3).map((v,i)=>v*a[3]+b[i]*(1-a[3])),1];
 let opacity=1,bg=[255,255,255,1];const chain=[];
 for(let n=el;n;n=n.parentElement){const st=getComputedStyle(n);chain.unshift(st);opacity*=Number(st.opacity);for(const f of st.filter.matchAll(/opacity\(\s*([\d.]+)(%)?\s*\)/g))opacity*=Number(f[1])/(f[2]?100:1);
  if(st.display==='none'||st.visibility!=='visible')reasons.push('hidden style');
  if(st.clipPath!=='none'||st.clip!=='auto')reasons.push('clipped content');
  if([st.maskImage,st.webkitMaskImage].some(v=>v&&v!=='none'))reasons.push('masked content');
 }
 for(const st of chain)bg=over(rgba(st.backgroundColor),bg);
 const fill=s.getPropertyValue('-webkit-text-fill-color'),fg=rgba(fill&&fill!=='currentcolor'?fill:s.color);fg[3]*=opacity;
 const ink=over(fg,bg),lum=c=>c.slice(0,3).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((v,n,i)=>v+n*[.2126,.7152,.0722][i],0),l=[lum(ink),lum(bg)].sort((a,b)=>b-a),contrast=(l[0]+.05)/(l[1]+.05),minimum=font>=24||(font>=18.666&&Number(s.fontWeight)>=700)?3:4.5;
 if(!Number.isFinite(font)||font<10)reasons.push('font below readable size');
 if(opacity<.9)reasons.push('effective opacity below 0.9');
 if(contrast+1e-6<minimum)reasons.push('contrast below '+minimum);
 if(!el.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))reasons.push('checkVisibility');
 if(r.width<Math.max(8,font*.7)||r.height<Math.max(10,font*.8))reasons.push('box below readable size');
 if(r.right<=0||r.left>=innerWidth||r.bottom<=0||r.top>=innerHeight)reasons.push('outside viewport');
 const paint=st=>Number(st.opacity)>=.9&&(rgba(st.backgroundColor)[3]>=.9||st.backgroundImage!=='none');
 const original=[],oldSheets=document.adoptedStyleSheets;
 let fragments=0,uncovered=0;
 try{
  // Hit-testing normally omits pointer-events:none, including pseudo-elements.
  // Temporarily enable only hit-testing, not visibility or paint, then restore.
  for(const n of document.querySelectorAll('body *'))if(getComputedStyle(n).pointerEvents==='none'){
   original.push([n,n.style.getPropertyValue('pointer-events'),n.style.getPropertyPriority('pointer-events')]);n.style.setProperty('pointer-events','auto','important');
  }
  const sheet=new CSSStyleSheet(),scope=':is(#ux-hit-a,*):is(#ux-hit-b,*):is(#ux-hit-c,*)';
  sheet.replaceSync(scope+'::before,'+scope+'::after{pointer-events:auto!important}');document.adoptedStyleSheets=[...oldSheets,sheet];
  const visiblePoint=(x,y,owner)=>{
   const stack=document.elementsFromPoint(x,y),at=stack.indexOf(owner);if(at<0)return false;
   return !stack.slice(0,at).some(n=>paint(getComputedStyle(n))||['::before','::after'].some(p=>{const st=getComputedStyle(n,p);return st.content!=='none'&&st.content!=='normal'&&paint(st);}));
  };
  const sample=(b,owner)=>{if(b.width<1||b.height<1||b.bottom<=0||b.top>=innerHeight||b.right<=0||b.left>=innerWidth)return;for(const fraction of [.15,.5,.85]){fragments++;const x=Math.max(.5,Math.min(innerWidth-.5,b.left+b.width*fraction)),y=Math.max(.5,Math.min(innerHeight-.5,b.top+b.height/2));if(visiblePoint(x,y,owner))uncovered++;}};
  if(el.matches('select,input,textarea'))sample(r,el);
  else{const walker=document.createTreeWalker(el,NodeFilter.SHOW_TEXT);let node;while(node=walker.nextNode()){
   if(!node.textContent.trim()||!node.parentElement.checkVisibility())continue;
   const range=document.createRange();range.selectNodeContents(node);for(const b of range.getClientRects())sample(b,node.parentElement);
  }}
 }finally{
  document.adoptedStyleSheets=oldSheets;for(const [n,value,priority] of original){if(value)n.style.setProperty('pointer-events',value,priority);else n.style.removeProperty('pointer-events');}
 }
 if(fragments&&uncovered/fragments<.9)reasons.push('text or control occluded');if(!fragments)reasons.push('text outside viewport or absent');
 return {readable:reasons.length===0,reasons,width:r.width,height:r.height,font,opacity,contrast,minimum,text:el.textContent.trim()};
}
export function forbiddenConfirmationLabels(){
 // Exact visible copy includes the explanation; aria-hidden does not hide pixels.
 const allowed=new Map([
  ['mode-onsite',{type:'radio',screen:'#step-1 .mode-control',parts:['Gerät vor Ort','Prüfe dein Gerät im lokalen Netzwerk.']}],
  ['mode-simulator',{type:'radio',screen:'#step-1 .mode-control',parts:['Übungsmodus','Lerne den Ablauf mit einem simulierten Gerät kennen.']}],
  ['profile-quick',{type:'radio',screen:'#step-2 #profile-option',parts:['Funktionsprüfung','Prüft Drosselung, Notbetrieb und vorzeitige Rückkehr.']}],
  ['profile-full',{type:'radio',screen:'#step-2 #profile-option',parts:['Langzeitprüfung','Beobachtet zusätzlich die eingestellte Mindestdauer des Notbetriebs.']}],
  ['scope-lpc',{screen:'#step-2 #scope-control',parts:['Bezug begrenzen (LPC)','Prüft, ob das Gerät seinen Strombezug begrenzt.']}],
  ['scope-lpp',{screen:'#step-2 #scope-control',parts:['Einspeisung begrenzen (LPP)','Prüft, ob das Gerät seine Einspeisung begrenzt.']}],
  ['sign-operator',{screen:'#step-2 #pdf-notes',parts:['Unterschriftsfeld für den Berichtsempfang','Bestätigt nur den Empfang des Berichts, keine Werkabnahme.']}],
  ['sign-inspector',{screen:'#step-2 #pdf-notes',parts:['Unterschriftsfeld für die prüfende Fachkraft','Mit der Unterschrift bestätigst du die Durchführung der aufgeführten Prüfung.']}],
  // 041 (Nutzer 2026-11-14): filter and import choices, no acknowledgements.
  ['report-practice',{screen:'.report-page .report-tools',parts:['Übungs-Protokolle anzeigen']}],
  ['import-reports',{screen:'#import-dialog',parts:['Protokolle importieren']}],
  ['import-profile',{screen:'#import-dialog',parts:['Profil importieren']}],
  ['import-identity',{screen:'#import-dialog',parts:['Simulator-Kennung importieren']}],
 ]),normalize=text=>text.replace(/\s+/gu,' ').trim(),t=text=>window.StromeaI18n?.t(text)||text;
 return [...document.querySelectorAll('input[type=checkbox],input[type=radio],[role=checkbox],[role=switch]')].filter(n=>n.checkVisibility({checkOpacity:true,checkVisibilityCSS:true})||[...(n.labels||[])].some(l=>l.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))).filter(n=>{
  const spec=allowed.get(n.id),label=n.labels?.[0];
  // Existing device selectors are choices, not acknowledgements. Match the
  // production data and the complete label, so appended confirmation text fails.
  if(n.matches('#step-1 #device-list>label.device-choice>input[type=radio][name=device-choice]')&&!n.hasAttribute('role')&&typeof devices!=='undefined'){
   const d=devices[Number(n.value)],copy=label?.querySelector('.device-copy'),badge=label?.querySelector('.device-state');
   if(!d||!copy||typeof deviceTitle!=='function'||typeof formatSki!=='function')return true;
   // 043: the result names the adapter it was found on („über WLAN (en1)“), nothing else.
   const via=d.interface&&typeof adapterLabel==='function'?' · '+t('über {adapter}').replace('{adapter}',adapterLabel(d.interface)):'';
   const expected=[deviceTitle(d),t('Gerätekennung (SKI)')+': '+formatSki(d.ski)+' · IP: '+d.address+via,...(badge?[badge.textContent]:[]),'›'];
   if(badge&&!['Verbunden','Gespeichert','Gefunden','Fehler'].map(t).includes(normalize(badge.innerText)))return true;
   return normalize(label.innerText)!==normalize(expected.join(' '));
  }
  if(n.tagName!=='INPUT'||n.hasAttribute('role')||!spec||n.type!==(spec.type||'checkbox')||!n.closest(spec.screen)||document.querySelectorAll('[id="'+n.id+'"]').length!==1||!label?.checkVisibility())return true;
  const expected=spec.parts.map(t),reason=document.getElementById(n.id+'-reason');
  if(reason?.checkVisibility()&&n.disabled){const text=normalize(reason.innerText);if(!['Vom Gerät nicht unterstützt.','Gerätefunktionen werden gelesen …'].map(t).map(normalize).includes(text))return true;expected.push(text);}
  return normalize(label.innerText)!==normalize(expected.join(' '));
 }).map(n=>({id:n.id,text:[...(n.labels||[])].map(l=>l.innerText).concat(n.getAttribute('aria-label')||'',n.parentElement?.innerText||'').join(' ').normalize('NFKC')}));
}
export function forbiddenNetworkDisclosures(){
 const names=['Netzwerkdetails','Network details','Szczegóły sieci','Сведения о сети','Сетевые данные','Відомості про мережу','Дані мережі','Ağ ayrıntıları','Ağ detayları','Detalii rețea','Detalii despre rețea',...Object.values(window.StromeaDictionaries||{}).map(d=>d['Netzwerkdetails'])].filter(Boolean).map(s=>s.toLocaleLowerCase().replace(/\s+/g,' ').trim());
 return [...document.querySelectorAll('#step-1 details,#step-1 summary,#step-1 button,#step-1 [role=button]')].filter(el=>names.some(n=>el.textContent.toLocaleLowerCase().replace(/\s+/g,' ').includes(n))).map(el=>({tag:el.tagName,text:el.textContent.trim()}));
}
// Includes user-supplied names: they must wrap between words, not within words.
export function splitWords(el){
 const bad=[],walker=document.createTreeWalker(el,NodeFilter.SHOW_TEXT);let n;
 while(n=walker.nextNode()){const p=n.parentElement;if(!p.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))continue;for(const m of n.textContent.matchAll(/[\p{L}\p{M}]{4,}/gu)){const r=document.createRange();r.setStart(n,m.index);r.setEnd(n,m.index+m[0].length);const boxes=[...r.getClientRects()].filter(b=>b.width>0);if(new Set(boxes.map(b=>Math.round(b.top))).size>1)bad.push(m[0]);}}
 return bad;
}
export function textLineCount(el){const walker=document.createTreeWalker(el,NodeFilter.SHOW_TEXT),tops=new Set();let n;while(n=walker.nextNode()){const r=document.createRange();r.selectNodeContents(n);for(const b of r.getClientRects())if(b.width>0&&b.height>0)tops.add(Math.round(b.top));}return tops.size;}
export function ancestorClipping(el){
 const bad=[],walker=document.createTreeWalker(el,NodeFilter.SHOW_TEXT);let n;
 while(n=walker.nextNode()){if(!n.textContent.trim()||!n.parentElement.checkVisibility())continue;const range=document.createRange();range.selectNodeContents(n);for(const r of range.getClientRects())for(let p=n.parentElement;p&&p!==document.body;p=p.parentElement){const s=getComputedStyle(p),b=p.getBoundingClientRect();if(['hidden','clip','auto','scroll'].includes(s.overflowY)&&(r.top<b.top-2||r.bottom>b.bottom+2))bad.push({text:n.textContent.trim(),ancestor:p.id||p.tagName});}}
 return bad;
}
export function horizontalTextOverflow(el){
 const bounds=el.getBoundingClientRect(),bad=[],walker=document.createTreeWalker(el,NodeFilter.SHOW_TEXT);let n;
 while(n=walker.nextNode()){if(!n.textContent.trim()||!n.parentElement.checkVisibility()||n.parentElement.closest('.sr-only'))continue; /* 039: visually hidden screen-reader context cannot overflow visibly */
 const range=document.createRange();range.selectNodeContents(n);if([...range.getClientRects()].some(r=>r.left<bounds.left-2||r.right>bounds.right+2))bad.push(n.textContent.trim());}
 return bad;
}
