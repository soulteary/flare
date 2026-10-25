/* The picker changes only the icon input. Entry form data stays untouched. */
(function () {
 'use strict';
 const input=document.getElementById('entry-icon');
 if(!input)return;
 const make=(tag,text)=>{const n=document.createElement(tag);if(text)n.textContent=text;return n;};
 const choose=make('button','选择图标');choose.type='button';choose.id='choose-icon';input.parentNode.append(choose);
 const dialog=make('dialog');dialog.id='icon-picker';dialog.setAttribute('aria-labelledby','icon-picker-title');
 const title=make('h2','选择内置图标');title.id='icon-picker-title';
 const label=make('label','搜索图标名称（英文关键词）');label.htmlFor='icon-picker-search';
 const search=make('input');search.id='icon-picker-search';search.type='search';search.autocomplete='off';search.maxLength=100;
 const status=make('p');status.id='icon-picker-status';status.setAttribute('role','status');status.setAttribute('aria-live','polite');
 const grid=make('div');grid.className='icon-picker-grid';
 const pages=make('div');pages.className='icon-picker-pages';
 const prev=make('button','上一页'),next=make('button','下一页'),close=make('button','取消');
 for(const b of [prev,next,close])b.type='button';pages.append(prev,next,close);
 dialog.append(title,label,search,status,grid,pages);document.body.append(dialog);
 let offset=0,request=0,timer,controller;
 function dismiss(){request++;controller?.abort();clearTimeout(timer);dialog.close();choose.focus();}
 async function load(){
  const current=++request;controller?.abort();controller=new AbortController();grid.replaceChildren();prev.disabled=true;next.disabled=true;status.textContent='正在加载图标…';
  try{
   const response=await fetch('/editor/icons?'+new URLSearchParams({q:search.value,offset:String(offset)}),{signal:controller.signal,cache:'no-store'});
   if(!response.ok||response.redirected||!(response.headers.get('content-type')||'').includes('application/json'))throw Error('无法加载图标，请确认登录状态后重试。');
   const result=await response.json();if(current!==request)return;
   status.textContent=result.total ? `共 ${result.total} 个图标，显示 ${offset+1}—${offset+result.names.length}`:'没有找到图标，请尝试其他英文关键词。';
   for(const name of result.names){
    const button=make('button');button.type='button';button.className='icon-choice';button.setAttribute('aria-label','选择图标 '+name);button.setAttribute('aria-pressed',String(input.value.toLowerCase()===name));
    const image=make('img');image.alt='';image.loading='lazy';image.src='/editor/icon?name='+encodeURIComponent(name);
    image.onerror=()=>{image.remove();button.append(make('span','预览不可用'));};
    button.append(image,make('span',name));button.onclick=()=>{input.value=name;input.dispatchEvent(new Event('input',{bubbles:true}));dismiss();input.focus();};grid.append(button);
   }
   prev.disabled=offset===0;next.disabled=offset+result.names.length>=result.total;
  }catch(error){if(current!==request||error.name==='AbortError')return;status.textContent=error.message;const retry=make('button','重试');retry.type='button';retry.onclick=load;grid.append(retry);}
 }
 choose.onclick=()=>{offset=0;search.value='';dialog.showModal();search.focus();load();};
 search.oninput=()=>{offset=0;clearTimeout(timer);request++;controller?.abort();grid.replaceChildren();prev.disabled=true;next.disabled=true;timer=setTimeout(load,180);};
 prev.onclick=()=>{offset=Math.max(0,offset-48);load();search.focus();};next.onclick=()=>{offset+=48;load();search.focus();};
 close.onclick=dismiss;dialog.addEventListener('cancel',event=>{event.preventDefault();dismiss();});
})();
