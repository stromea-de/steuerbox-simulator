'use strict';
// Server embeds recorded data as escaped JSON; no API call, polling or mutation here.
window.StromeaSavedReport={mount(doc=document){
 const data=doc.getElementById('saved-report'),root=doc.getElementById('saved-result');if(!data || !root || root.dataset.mounted)return;
 const report=JSON.parse(data.textContent);
 const view=StromeaResult.create(root,{saved:true,isReal:()=>report.mode==='onsite',svgNode:StromeaResult.svgNode,drawnCheck:()=>StromeaResult.drawnCheck()});
 const state={report,counts:{passed:Number(data.dataset.passed),failed:Number(data.dataset.failed),notChecked:Number(data.dataset.notChecked)},outcome:data.dataset.outcome,busy:false};
 root.hidden=false;root.dataset.mounted='true';doc.getElementById('saved-fallback').hidden=true;
 root.querySelector('#live').hidden=true;root.querySelector('#title-3').textContent='Prüfergebnis';
 view.renderPower(state);view.renderResult(state);view.renderSteps([...(report.steps || []),...(report.events || [])].sort((a,b)=>Date.parse(a.at)-Date.parse(b.at)),'');
 for(const a of root.querySelectorAll('a[download]')){const original=doc.querySelector('#saved-fallback a[download$="'+(a.getAttribute('download').endsWith('.pdf')?'.pdf':'.json')+'"]');if(original)a.href=original.getAttribute('href');}

 const grid=doc.createElement('div');grid.className='workspace saved-workspace';root.before(grid);
 const main=doc.createElement('section');main.className='card results';grid.append(main);main.append(root);
 // 041: der Rücklink gehört in die Hauptspalte; die Detailspalte reicht bis zum Footer.
 const backLink=[...doc.querySelectorAll('main>p')].find(p=>p.querySelector('a[href="/protokolle"]'));if(backLink){backLink.classList.add('saved-back');main.append(backLink);}
 const side=doc.createElement('aside');side.className='saved-inspector';side.setAttribute('aria-label','Details');grid.append(side);side.append(root.querySelector('#all-steps'));
 const trigger=doc.createElement('button');trigger.id='saved-details';trigger.className='text-button saved-details';trigger.textContent='Details';trigger.setAttribute('aria-expanded','false');trigger.setAttribute('aria-controls','saved-inspector');grid.before(trigger);side.id='saved-inspector';side.prepend(root.querySelector('#power-technical'));side.prepend(root.querySelector('#power-table'));
 const close=doc.createElement('button');close.className='text-button saved-close';close.textContent='Schließen';side.prepend(close);
 const shade=doc.createElement('div');shade.className='saved-backdrop';shade.hidden=true;grid.before(shade);
 const small=doc.defaultView.matchMedia('(max-width:1050px)');let open=false;
 const setOpen=value=>{open=value && small.matches;side.classList.toggle('panel-open',open);shade.hidden=!open;trigger.setAttribute('aria-expanded',String(open));side.setAttribute('role',open?'dialog':'complementary');if(open)side.setAttribute('aria-modal','true');else side.removeAttribute('aria-modal');main.inert=open;for(const node of grid.parentElement.children){if(![grid,shade].includes(node))node.inert=open;}doc.body.classList.toggle('panel-open',open);};
 const hide=()=>{setOpen(false);trigger.focus();};trigger.addEventListener('click',()=>{setOpen(true);close.focus();});close.addEventListener('click',hide);shade.addEventListener('click',hide);
 doc.addEventListener('keydown',e=>{if(!open)return;if(e.key==='Escape'){e.preventDefault();hide();}if(e.key==='Tab'){const nodes=[...side.querySelectorAll('button,a[href],input,select,[tabindex="0"]')].filter(n=>!n.disabled && n.getClientRects().length);if(e.shiftKey && doc.activeElement===nodes[0]){e.preventDefault();nodes.at(-1).focus();}else if(!e.shiftKey && doc.activeElement===nodes.at(-1)){e.preventDefault();nodes[0].focus();}}});small.addEventListener('change',()=>setOpen(false));setOpen(false);
 const details=doc.createElement('dl');for(const [title,text] of [['Gerät',report.device?.name || '–'],['Gerätekennung',report.device?.ski?.toUpperCase().match(/.{1,4}/g)?.join(' ') || '–'],['Firma',report.business || '–'],['Prüfer / Prüferin',report.inspector || '–']]){const dt=doc.createElement('dt'),dd=doc.createElement('dd');dt.textContent=title;dd.textContent=text;dd.dataset.userContent='true';details.append(dt,dd);}side.insertBefore(details,root.ownerDocument.getElementById('all-steps'));
 const content=doc.createElement('div');content.className='inspector-content';while(side.firstChild)content.append(side.firstChild);side.append(content);
 // A saved report uses exactly the same language-dependent rendering as the live result.
 if(doc!==document)StromeaSavedReport.cleanup?.();
 const languageUpdate=()=>{view.invalidatePower();view.invalidateSteps();view.renderPower(state);view.renderResult(state);view.renderSteps([...(report.steps || []),...(report.events || [])].sort((a,b)=>Date.parse(a.at)-Date.parse(b.at)),'');StromeaI18n.walk(doc);};
 window.addEventListener('stromea-language',languageUpdate);
 if(doc!==document)StromeaSavedReport.cleanup=()=>{window.removeEventListener('stromea-language',languageUpdate);view.dispose();};
 }};
// 041: Dialoge „Über“ und „Simulator beenden“ wie in der Hauptansicht.
if(document.getElementById('saved-fallback') && window.StromeaDialogs){StromeaDialogs.about();StromeaDialogs.quit();}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',()=>StromeaSavedReport.mount());else StromeaSavedReport.mount();
