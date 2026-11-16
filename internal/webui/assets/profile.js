'use strict';
window.StromeaProfile=(()=>{
 const key='stromea-profile-v1',fields=['business','street','postalCode','city','country','inspector'],defaultCountry='Deutschland';
 // 041: one rule for stored and imported profiles (text up to 200 characters, logo PNG/JPEG up to 1,000,000 characters).
 const clean=v=>{const out={};for(const k of [...fields,'logo'])if(typeof v?.[k]==='string'){if(k!=='logo')out[k]=v[k].slice(0,200);else if(/^data:image\/(png|jpeg);base64,/.test(v[k]) && v[k].length<=1000000)out[k]=v[k];}return out;};
 let value={};try{value=clean(JSON.parse(localStorage.getItem(key)||'{}'));}catch{}
 if(typeof value.country!=='string')value.country=defaultCountry;
 try{localStorage.removeItem('stromea-remembered-site');}catch{}
 const save=(next=value)=>{try{localStorage.setItem(key,JSON.stringify(next));return true;}catch{return false;}};
 function prefill(){
  const box=document.getElementById('business-summary');if(!box)return;box.replaceChildren();
  // 041 (Nutzer 2026-11-14): „– (im Profil ändern)“ in der Zeile „Firma“, gleiche Schriftgröße, kein Hinweis darunter.
  const dl=document.createElement('dl');
  for(const [index,[title,text]] of [['Firma',value.business],['Adresse',[value.street,[value.postalCode,value.city].filter(Boolean).join(' '),value.country && value.country!==defaultCountry?value.country:''].filter(Boolean).join(', ')],['Prüfer / Prüferin',value.inspector]].entries()){const dt=document.createElement('dt'),dd=document.createElement('dd'),v=document.createElement('span');dt.textContent=title;v.textContent=text || '–';v.dataset.userContent='true';dd.append(v);
   if(index===0){const link=document.createElement('a');link.id='edit-profile';link.href='/profil';link.textContent='im Profil ändern';dd.append(document.createTextNode(' ('),link,document.createTextNode(')'));}dl.append(dt,dd);}
  box.append(dl);if(value.logo){const img=document.createElement('img');img.src=value.logo;img.alt='Firmenlogo';box.append(img);}window.StromeaI18n?.walk(box);
 }
 function snapshot(){return {...value};}
 // Small PNG encoder for hosts whose canvas image encoder is unavailable.
 // PNG chunks + uncompressed DEFLATE, capped at 320×320 RGBA (< 420 KB).
 function pngPixels(pixels,width,height){
  const raw=new Uint8Array((width*4+1)*height);for(let y=0;y<height;y++)raw.set(pixels.subarray(y*width*4,(y+1)*width*4),y*(width*4+1)+1);
  const z=[0x78,0x01];for(let at=0;at<raw.length;){const n=Math.min(65535,raw.length-at);z.push(at+n===raw.length?1:0,n&255,n>>8,(~n)&255,((~n)>>8)&255);for(let i=0;i<n;i++)z.push(raw[at+i]);at+=n;}
  let a=1,b=0;for(const v of raw){a=(a+v)%65521;b=(b+a)%65521;}const u32=n=>[(n>>>24)&255,(n>>>16)&255,(n>>>8)&255,n&255];z.push(...u32((b<<16)|a));
  const out=[137,80,78,71,13,10,26,10];
  const chunk=(name,data)=>{const bytes=[...name].map(c=>c.charCodeAt(0));for(const v of data)bytes.push(v);let crc=0xffffffff;for(const v of bytes){crc^=v;for(let j=0;j<8;j++)crc=(crc>>>1)^((crc&1)?0xedb88320:0);}out.push(...u32(data.length));for(const v of bytes)out.push(v);out.push(...u32((crc^0xffffffff)>>>0));};
  chunk('IHDR',[...u32(width),...u32(height),8,6,0,0,0]);chunk('IDAT',z);chunk('IEND',[]);
  let binary='';for(let at=0;at<out.length;at+=8192)binary+=String.fromCharCode(...out.slice(at,at+8192));return 'data:image/png;base64,'+btoa(binary);
 }

 async function logo(file){
  if(!['image/png','image/jpeg'].includes(file.type) || file.size>5*1024*1024)throw Error('Bitte PNG oder JPG bis 5 MB auswählen.');
  const url=await new Promise((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(reader.result);reader.onerror=()=>reject(Error('Logo konnte nicht gelesen werden.'));reader.readAsDataURL(file);});
  try{
   const img=new Image();await new Promise((resolve,reject)=>{img.onload=resolve;img.onerror=()=>reject(Error('Logo konnte nicht gelesen werden.'));img.src=url;});
   if(!img.naturalWidth || !img.naturalHeight || img.naturalWidth*img.naturalHeight>25000000)throw Error('Logo zu groß: höchstens 25 Megapixel.');
   const ratio=Math.min(1,320/Math.max(img.naturalWidth,img.naturalHeight)),c=document.createElement('canvas');c.width=Math.max(1,Math.round(img.naturalWidth*ratio));c.height=Math.max(1,Math.round(img.naturalHeight*ratio));
   const ctx=c.getContext('2d');ctx.drawImage(img,0,0,c.width,c.height);
   // Inspect visible border pixels; transparent and near-white backgrounds are accepted.
   const d=ctx.getImageData(0,0,c.width,c.height).data;let dark=0,total=0;
   const edge=(x,y)=>{const i=(y*c.width+x)*4;total++;if(d[i+3]>30 && Math.min(d[i],d[i+1],d[i+2])<235)dark++;};
   for(let x=0;x<c.width;x++){edge(x,0);edge(x,c.height-1);}for(let y=0;y<c.height;y++){edge(0,y);edge(c.width-1,y);}
   if(dark>total*.1)throw Error('Bitte ein Logo mit weißem oder transparentem Hintergrund wählen.');
   let data=c.toDataURL('image/png');if(!data.startsWith('data:image/png;base64,'))data=pngPixels(d,c.width,c.height);if(data.length>1000000)throw Error('Logo nach Verkleinerung zu groß. Bitte einfacheres Bild wählen.');return data;
  }catch(error){if(error instanceof DOMException)throw Error('Logo konnte nicht gelesen werden.');throw error;}
 }
 function mount(section){
  const help=document.createElement('p');help.textContent='Hinterlege deine Firmendaten für den Prüfbericht. Sie bleiben auf diesem Computer; Kundendaten erfasst du je Prüfung.';
  const heading=document.createElement('div');heading.className='local-heading';section.querySelector('h2').before(heading);heading.append(section.querySelector('h2'));section.append(help);
  // 043 (Nutzer 2026-11-14): „Zurück zur Prüfung“ is a secondary button next to „Profil speichern“ and returns to
  // exactly the step that was left (connection and entered values stay).
  const back=document.createElement('a');back.id='profile-return';back.className='button secondary';let leftStep=step;try{const stored=Number(sessionStorage.getItem('stromea-return-step-043'));if([1,2,3].includes(stored))leftStep=stored;}catch{}const result=!busy && leftStep===3 && !!state?.report;back.href='/?step='+(busy?operationStep():leftStep); /* operationStep() keeps a reconnect from step 2 in step 2 (R043-06) */ /* R043-02: also after reloading the profile */back.textContent=result?'Zurück zum Ergebnis':'Zurück zur Prüfung';
  let draft;try{draft=JSON.parse(sessionStorage.getItem('stromea-profile-draft')||'null');}catch{}const initial=draft || value;
  const form=document.createElement('form');form.className='profile-form';form.noValidate=true;section.append(form);
  // 041: Prüfer/in in voller Breite zuerst; Firma und Logo teilen sich eine Zeile; PLZ, Ort und Land eine Zeile.
  // 043: only the inspector's name is required (marked); every other field says „optional“.
  for(const [k,label] of [['inspector','Prüfer / Prüferin (Vor- und Nachname)'],['business','Firma'],['street','Straße'],['postalCode','PLZ'],['city','Ort'],['country','Land']]){
   const l=document.createElement('label'),input=document.createElement('input');l.htmlFor=input.id='profile-'+k;l.append(document.createTextNode(label));const mark=document.createElement('span');mark.className=k==='inspector'?'field-mark field-required':'field-mark field-optional';mark.textContent=k==='inspector'?'Pflichtfeld':'optional';l.append(document.createTextNode(' · '),mark); /* G043-09: separated like „Kunde / Anlage · optional“ */
   input.maxLength=200;input.value=initial[k] ?? (k==='country'?defaultCountry:'');input.autocomplete='off';if(k==='inspector'){input.required=true;input.setAttribute('aria-required','true');}const field=document.createElement('div');field.className='profile-field profile-'+k;field.append(l,input);form.append(field);
  }
  const label=document.createElement('label');label.htmlFor='profile-logo';label.append(document.createTextNode('Firmenlogo für den Prüfbericht'),document.createTextNode(' · '),Object.assign(document.createElement('span'),{className:'field-mark field-optional',textContent:'optional'}));label.title='PNG oder JPG mit weißem oder transparentem Hintergrund, höchstens 5 MB.';
  const file=document.createElement('input');file.id='profile-logo';file.type='file';file.accept='image/png,image/jpeg';file.hidden=true;const choose=document.createElement('button');choose.type='button';choose.className='secondary';choose.textContent='Logo auswählen';choose.addEventListener('click',()=>file.click());
  const preview=document.createElement('img');preview.id='profile-preview';preview.alt='Logo-Vorschau';preview.hidden=!initial.logo;if(initial.logo)preview.src=initial.logo;
  const remove=document.createElement('button');remove.type='button';remove.className='text-button';remove.textContent='Logo entfernen';remove.hidden=!initial.logo;
  let draftLogo=initial.logo || '',decoding=false;
  const current=()=>({...Object.fromEntries(fields.map(k=>[k,document.getElementById('profile-'+k).value.trim()])),logo:draftLogo});
  const dirty=()=>{const now=current();return fields.some(k=>(now[k] || '')!==(value[k] ?? (k==='country'?defaultCountry:''))) || (now.logo || '')!==(value.logo || '');};
  const status=document.createElement('p');status.id='profile-status';status.className='help';status.setAttribute('role','status');
  const submit=document.createElement('button');submit.type='submit';submit.textContent='Profil speichern';
  // 043: „Profil speichern“ is enabled only while something differs from the stored profile.
  const sync=()=>{submit.disabled=decoding || !dirty();};
  const stash=()=>{try{sessionStorage.setItem('stromea-profile-draft',JSON.stringify({...Object.fromEntries(fields.map(k=>[k,document.getElementById('profile-'+k).value])),logo:draftLogo}));}catch{}};form.addEventListener('input',()=>{stash();status.textContent='';status.classList.remove('profile-error');sync();});
  const problem=text=>{status.textContent=text;status.classList.add('profile-error');};
  file.addEventListener('change',async()=>{if(!file.files[0])return;decoding=true;sync();try{draftLogo=await logo(file.files[0]);preview.src=draftLogo;preview.hidden=false;remove.hidden=false;stash();status.textContent='';status.classList.remove('profile-error');}catch(e){problem(e.message);}finally{decoding=false;sync();file.value='';}});
  remove.addEventListener('click',()=>{draftLogo='';preview.removeAttribute('src');preview.hidden=true;remove.hidden=true;stash();status.textContent='';sync();});
  const artwork=document.createElement('div');artwork.className='profile-artwork';const pick=document.createElement('div');pick.className='profile-logo-row';pick.append(choose,preview,remove);artwork.append(label,file,pick);form.querySelector('.profile-business').after(artwork);const actions=document.createElement('div');actions.className='profile-actions';actions.append(submit);if(paired || busy)actions.append(back);actions.append(status);form.append(actions);
  // 041: „Simulator-Kennung erneuern“ gehört ins Profil, unter Speichern nach einer Trennlinie.
  const identity=document.createElement('section');identity.className='profile-identity';identity.innerHTML='<hr><div class="profile-identity-row"><button type="button" id="reset-identity" class="secondary">Simulator-Kennung erneuern</button><p class="help">Die Identität bleibt lokal gespeichert. Bei einer neuen Identität müssen bisherige Geräte neu gekoppelt werden.</p></div><p id="identity-status" role="status"></p>';section.append(identity);
  const resetButton=identity.querySelector('#reset-identity');resetButton.disabled=!!(busy || pending || searching);resetButton.addEventListener('click',()=>window.StromeaIdentityReset?.());
  form.addEventListener('submit',e=>{e.preventDefault();if(decoding || !dirty())return;const next=current();
   if(!next.inspector){problem('Bitte gib den Namen der prüfenden Person ein.');document.getElementById('profile-inspector').setAttribute('aria-invalid','true');document.getElementById('profile-inspector').focus();return;}
   document.getElementById('profile-inspector').removeAttribute('aria-invalid');const previous=value;value=next;if(!value.logo)delete value.logo;
   // R043-05: the stored profile is the comparison state only after a successful write.
   if(save()){prefill();try{sessionStorage.removeItem('stromea-profile-draft');}catch{}status.classList.remove('profile-error');status.textContent='Profil gespeichert';}else{value=previous;problem('Profil konnte nicht lokal gespeichert werden.');}sync();
  });sync();StromeaI18n.walk(section);
 }
 function update(patch){const previous=value;value={...value,...clean(patch)};const ok=save();if(!ok){value=previous;return false;}prefill();try{const draft=JSON.parse(sessionStorage.getItem('stromea-profile-draft')||'null');if(draft){Object.assign(draft,clean(patch));sessionStorage.setItem('stromea-profile-draft',JSON.stringify(draft));}}catch{}return ok;}
 function newInspection(){document.getElementById('form').reset();window.StromeaDraft?.reset();prefill();}
 // 041: Import replaces the whole profile (validated with the same rules as the stored one).
 function replace(next){const cleaned=clean(next);if(typeof cleaned.country!=='string')cleaned.country=defaultCountry;if(!save(cleaned))return false;value=cleaned;try{sessionStorage.removeItem('stromea-profile-draft');}catch{}prefill();return true;}
 prefill();return {mount,snapshot,logo,prefill,newInspection,replace,update};
})();
// Tab-local drafts survive reload/history, without remembering customer data across sessions.
window.StromeaDraft=(()=>{
 const key='stromea-inspection-draft-v036',form=document.getElementById('form');
 const fields=()=>[...form.querySelectorAll('input[id],textarea[id],select[id]'),...document.querySelectorAll('#manual-device input[id],#manual-device textarea[id],#manual-device select[id],#qr-text')];
 try{const saved=JSON.parse(sessionStorage.getItem(key)||'{}');for(const field of fields()){const v=saved[field.id];if(v){if(['checkbox','radio'].includes(field.type))field.checked=!!v.checked;else if(typeof v.value==='string')field.value=v.value;}}}catch{}
 const save=()=>{try{sessionStorage.setItem(key,JSON.stringify(Object.fromEntries(fields().map(n=>[n.id,{value:n.value,checked:n.checked}]))));}catch{}};
 const tracked=n=>form.contains(n) || !!n.closest?.('#manual-device') || n.id==='qr-text';document.addEventListener('input',e=>{if(tracked(e.target))save();});document.addEventListener('change',e=>{if(tracked(e.target))save();});window.addEventListener('pagehide',save);
 return {save,reset(){try{sessionStorage.removeItem(key);}catch{}}};
})();
// URLs describe views. Every restoration is resolved against the current backend,
// never against a previously saved busy/connection/report object.
window.StromeaRoute=(()=>{
 let restoring=true,restoreEpoch=0;
 const local=['/profil','/protokolle','/hilfe'],fileMode=location.protocol==='file:';
 const current=()=>fileMode?new URL(location.hash.slice(1) || '/?step=1','http://prototype.invalid'):new URL(location.href);
 const write=(url,value={},replace=false)=>history[replace?'replaceState':'pushState'](value,'',fileMode?'#'+url:url);
 function mark(path){for(const a of document.querySelectorAll('.main-menu a')){const active=new URL(a.href,location.href).pathname===path;a.toggleAttribute('aria-current',active);if(active)a.setAttribute('aria-current','page');}}
 function stepURL(next,replace=false){if(restoring || local.includes(current().pathname))return;const url='/?step='+next;if(current().pathname+current().search===url)return;write(url,{step:next,report:state?.report?.id || null},replace);mark('/');}
 async function restore(){
  const epoch=++restoreEpoch;restoring=true;stopDictation();stopCamera();
  const path=current().pathname;
  if(local.includes(path)){workspace.closeQR();await StromeaNavigation(path);if(epoch!==restoreEpoch)return;mark(path);}
  else{
   document.body.classList.remove('local-page-open');document.querySelector('.local-page')?.remove();mark('/');
   let next=Number(new URLSearchParams(current().search).get('step')) || 1;
   if(canCancel())next=operationStep();
   // 043 (Nutzer 2026-11-14): returning (profile, help, reports, back/forward) keeps the step even after the
   // connection dropped; step 2 then offers a one-click reconnect and keeps every entered value.
   else if(next===3 && !state?.report)next=prepared()?2:1;
   else if(next===2 && !prepared())next=1;
   if(![1,2,3].includes(next))next=1;
   goStep(next,false);write('/?step='+next,{step:next,report:state?.report?.id || null},true);
  }
  restoring=false;controls();if(!local.includes(path))workspace.resume();
 }
 document.addEventListener('click',e=>{
  const a=e.target.closest('a[href]');if(!a || e.defaultPrevented || e.button!==0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || a.target==='_blank')return; // UX-S20: explicit new-tab links are left to the browser
  const url=new URL(a.href,location.href);if(url.origin!==location.origin || !(local.includes(url.pathname) || (url.pathname==='/' && (a.closest('.main-menu') || a.id==='profile-return'))))return;
  e.preventDefault();StromeaDraft.save();if(!local.includes(current().pathname) && local.includes(url.pathname)){try{sessionStorage.setItem('stromea-return-step-043',String(step));}catch{}}workspace.closeQR();if(url.pathname==='/' && url.searchParams.has('new') && !busy && !pending)StromeaProfile.newInspection();write(url.pathname+url.search);restore();
 });
 window.addEventListener('popstate',async()=>{await refresh();await restore();});
 return {get restoring(){return restoring;},restore,reload:async()=>{restoring=true;await refresh(true);await restore();},step:stepURL};
})();
