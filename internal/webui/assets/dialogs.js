'use strict';
// 041 (Nutzer 2026-11-14): Dialoge. Persistente Dialoge schließen nicht durch Klick daneben; Enter löst die
// sichere Standardaktion aus (Beenden: „Schließen“, Import: „Abbrechen“). Escape bricht ab.
window.StromeaDialogs=(()=>{
 const $=id=>document.getElementById(id),t=s=>window.StromeaI18n?StromeaI18n.t(s):s;
 function create(dialog,{persistent=true,defaultButton,enterAlwaysDefault=false,onCancel=()=>{}}){
  dialog.addEventListener('cancel',e=>{e.preventDefault();onCancel();});
  dialog.addEventListener('keydown',e=>{if(e.key!=='Enter' || e.isComposing)return;if(!enterAlwaysDefault && e.target.matches('button:not(:disabled)'))return;e.preventDefault();defaultButton().click();});
  if(!persistent)dialog.addEventListener('click',e=>{if(e.target!==dialog)return;const r=dialog.getBoundingClientRect();if(e.clientX<r.left || e.clientX>r.right || e.clientY<r.top || e.clientY>r.bottom)onCancel();});
  return {open(){if(!dialog.open)dialog.showModal();window.StromeaI18n?.walk(dialog);defaultButton().focus();},close(){if(dialog.open)dialog.close();}};
 }
 // „Über den Stromea Steuerbox-Simulator“: nicht persistent (Schließen, Escape, Klick daneben).
 function about(){
  if(!$('about-dialog') || !$('about-open'))return;
  const close=()=>{dialog.close();$('about-open').focus();},dialog=create($('about-dialog'),{persistent:false,defaultButton:()=>$('about-close'),onCancel:close});
  $('about-open').addEventListener('click',()=>dialog.open());$('about-close').addEventListener('click',close);
 }
 // „Simulator beenden“: beendet nur diesen Simulator-Dienst (nie einen vorgeschalteten Proxy). Eine laufende
 // Prüfung wird abgebrochen; der Dienst hebt die Testgrenzen vorher auf. Danach ein ruhiger Endzustand.
 function quit({isBusy=()=>false,onStart=()=>{}}={}){
  if(!$('quit-dialog') || !$('quit-open'))return;
  const token=document.querySelector('meta[name="session-token"]')?.content || '';
  const back=()=>{dialog.close();$('quit-open').focus();},dialog=create($('quit-dialog'),{defaultButton:()=>$('quit-confirm'),onCancel:back});
  const busyText=$('quit-busy').textContent;
  $('quit-open').addEventListener('click',()=>{$('quit-busy').textContent=busyText;$('quit-busy').hidden=!isBusy();$('quit-confirm').disabled=false;dialog.open();});
  $('quit-cancel').addEventListener('click',back);
  $('quit-confirm').addEventListener('click',async()=>{
   $('quit-confirm').disabled=true;let result;
   try{const response=await fetch('/api/shutdown',{method:'POST',headers:{'Content-Type':'application/json','X-Stromea-Token':token},body:'{}'});if(!response.ok)throw Error('shutdown');result=await response.json();}
   catch{$('quit-confirm').disabled=false;$('quit-busy').hidden=false;$('quit-busy').textContent=t('Beenden ist hier nicht möglich. Beende den Simulator über das Stromea-Symbol oder schließe das Terminal.');return;}
   onStart();dialog.close();
   const screen=document.createElement('section');screen.className='quit-screen card';screen.setAttribute('role','status');
   const title=document.createElement('h2'),copy=document.createElement('p');screen.append(title,copy);document.querySelector('header').after(screen);document.body.classList.add('quit-open');
   const show=(heading,text)=>{title.textContent=heading;copy.textContent=text;window.StromeaI18n?.walk(screen);};
   show('Simulator wird beendet …',result?.aborting?'Die laufende Prüfung wird abgebrochen und die Testgrenzen werden aufgehoben.':'Einen Moment bitte.');
   const done=()=>show('Simulator beendet','Du kannst dieses Fenster jetzt schließen. Zum erneuten Öffnen starte den Simulator über die Task- oder Menüleiste oder das Programmsymbol.');
   for(let i=0;i<180;i++){await new Promise(r=>setTimeout(r,1000));try{const r=await fetch('/api/instance',{cache:'no-store'});if(!r.ok){done();return;}}catch{done();return;}}
   done();
  });
 }
 return {create,about,quit};
})();
