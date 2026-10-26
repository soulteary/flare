/* The picker changes only the icon input. Entry form data stays untouched. */
(function () {
 'use strict';
  const t = window.FlareEditorI18n?.t || ((text,...args)=>text.replace(/\{(\d+)\}/g,(_,i)=>String(args[i]??'')));
 const input=document.getElementById('entry-icon');
 if(!input)return;
 const make=(tag,text)=>{const n=document.createElement(tag);if(text)n.textContent=text;return n;};
 const choose=make('button',t('editor.choose_icon'));choose.type='button';choose.id='choose-icon';input.parentNode.append(choose);
 const dialog=make('dialog');dialog.id='icon-picker';dialog.setAttribute('aria-labelledby','icon-picker-title');
 const title=make('h2',t('editor.choose_a_built_in_icon'));title.id='icon-picker-title';
 const label=make('label',t('editor.search_icon_names_english_keywords'));label.htmlFor='icon-picker-search';
 const search=make('input');search.id='icon-picker-search';search.type='search';search.autocomplete='off';search.maxLength=100;
 const status=make('p');status.id='icon-picker-status';status.setAttribute('role','status');status.setAttribute('aria-live','polite');
 const grid=make('div');grid.className='icon-picker-grid';
 const pages=make('div');pages.className='icon-picker-pages';
 const prev=make('button',t('editor.previous')),next=make('button',t('editor.next')),close=make('button',t('editor.cancel'));
 for(const b of [prev,next,close])b.type='button';pages.append(prev,next,close);
 dialog.append(title,label,search,status,grid,pages);document.body.append(dialog);
 let offset=0,request=0,timer,controller;
 function dismiss(){request++;controller?.abort();clearTimeout(timer);dialog.close();choose.focus();}
 async function load(){
  const current=++request;controller?.abort();controller=new AbortController();grid.replaceChildren();prev.disabled=true;next.disabled=true;status.textContent=t('editor.loading_icons');
  try{
   const response=await fetch('/editor/icons?'+new URLSearchParams({q:search.value,offset:String(offset)}),{signal:controller.signal,cache:'no-store'});
   if(!response.ok||response.redirected||!(response.headers.get('content-type')||'').includes('application/json'))throw Error(t('editor.cannot_load_icons_check_your_session_and_retry'));
   const result=await response.json();if(current!==request)return;
   status.textContent=result.total ? t('editor.icon_result_count',{total:result.total,start:offset+1,end:offset+result.names.length}):t('editor.no_icons_found_try_another_english_keyword');
   for(const name of result.names){
    const button=make('button');button.type='button';button.className='icon-choice';button.setAttribute('aria-label',t('editor.choose_named_icon',{name}));button.setAttribute('aria-pressed',String(input.value.toLowerCase()===name));
    const image=make('img');image.alt='';image.loading='lazy';image.src='/editor/icon?name='+encodeURIComponent(name);
    image.onerror=()=>{image.remove();button.append(make('span',t('editor.preview_unavailable')));};
    button.append(image,make('span',name));button.onclick=()=>{input.value=name;input.dispatchEvent(new Event('input',{bubbles:true}));dismiss();input.focus();};grid.append(button);
   }
   prev.disabled=offset===0;next.disabled=offset+result.names.length>=result.total;
  }catch(error){if(current!==request||error.name==='AbortError')return;status.textContent=error.message;const retry=make('button',t('editor.retry'));retry.type='button';retry.onclick=load;grid.append(retry);}
 }
 choose.onclick=()=>{offset=0;search.value='';dialog.showModal();search.focus();load();};
 search.oninput=()=>{offset=0;clearTimeout(timer);request++;controller?.abort();grid.replaceChildren();prev.disabled=true;next.disabled=true;timer=setTimeout(load,180);};
 prev.onclick=()=>{offset=Math.max(0,offset-48);load();search.focus();};next.onclick=()=>{offset+=48;load();search.focus();};
 close.onclick=dismiss;dialog.addEventListener('cancel',event=>{event.preventDefault();dismiss();});
})();
