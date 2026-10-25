/* List and form editor. All rendering uses text nodes, never user-supplied HTML. */
(async function () {
  'use strict';
  const t = window.FlareEditorI18n?.t || ((text,...args)=>text.replace(/\{(\d+)\}/g,(_,i)=>String(args[i]??'')));
  const stateErrorText=error=>error.code==='entry_not_found'?t('条目不存在'):error.code==='draft_too_large'?t('草稿文件不能超过 64 MiB'):error.message;
  const $ = id => document.getElementById(id);
  let filterTimer;
  let store, saving=false, editingKey=null, formStart='', lastLocation='', undo=null, migrationID=null, conflict=false, recoveryRequired=false, focusBeforeDialog=null, batch=null;
  const entryDialog=$('entry-dialog'), categoryDialog=$('categories-dialog');
  const fields={Name:'entry-name',URL:'entry-url',Category:'entry-location'};
  // Error text can wrap to several lines at large font sizes; reserve its actual
  // height instead of assuming the fixed save bar always fits in 150/210 pixels.
  const savebar=document.querySelector('.savebar');
  function measureSavebar() {document.documentElement.style.setProperty('--savebar-height',savebar.getBoundingClientRect().height+'px');}
  if(window.ResizeObserver)new ResizeObserver(measureSavebar).observe(savebar);
  window.addEventListener('resize',measureSavebar);measureSavebar();
  // Mobile keyboards shrink the visual viewport without always changing dvh.
  function measureViewport() {document.documentElement.style.setProperty('--editor-viewport-height',(window.visualViewport?.height || window.innerHeight)+'px');}
  window.visualViewport?.addEventListener('resize',measureViewport);measureViewport();
  function focusEntry(key) {const row=[...$('entry-list').children].find(item=>item.dataset.key===key);(row?.querySelector('.entry-name') || $('new-entry')).focus();}

  function el(tag, text, className) { const node=document.createElement(tag); if(text!==undefined)node.textContent=text;if(className)node.className=className;return node; }
  function button(text, action, className) {const node=el('button',text,className);node.type='button';node.addEventListener('click',action);return node;}
  function entryButton(text,action,className){const node=el('button',text,className);node.type='button';node.dataset.entryAction=action;return node;}
  function categoryName(id) {
    if(id==='apps')return t('应用区');
    if(id==='category:')return store.working.categories.length ? t('默认分类 · ')+store.working.categories[0].Name : t('未分类书签');
    return store.working.categories.find(c=>'category:'+c.ID===id)?.Name || t('分类缺失：')+id;
  }

  function locations(select, value, includeAll=false, exclude=null) {
    select.replaceChildren();
    function option(v,t){const o=el('option',t);o.value=v;select.append(o);}
    if(includeAll)option('all',t('全部位置'));else option('',t('请选择位置'));
    option('apps',t('应用区'));if(exclude===null)option('category:',categoryName('category:'));
    for(const c of store.working.categories)if(c.ID!==exclude)option('category:'+c.ID,c.Name);
    if(value && ![...select.options].some(o=>o.value===value))option(value,t('分类缺失：')+value);
    select.value=value || (includeAll?'all':'');
  }
  function setStatus(message, error=false) {
    $('save-status').textContent=message;
    document.querySelector('.savebar').dataset.error=String(error);
    if(store){const dirty=store.dirty || !!batch?.dirty;$('save-changes').disabled=saving || recoveryRequired || !dirty;$('discard-changes').disabled=saving || !dirty;}
  }
  function syncStatus() {if(recoveryRequired){setStatus(t('保存恢复未完成，请先导出草稿并检查服务端恢复副本，再重新载入。'),true);return;}setStatus(conflict ? t('服务器数据已变化，请保留修改后重新载入。') : (store.dirty || batch?.dirty)?t('有未保存的更改'):t('所有更改已保存'), conflict);}
  function render() {
    clearTimeout(filterTimer);
    const filter=$('location-filter').value, query=$('entry-search').value.trim().toLocaleLowerCase();
    locations($('location-filter'),filter,true);
    const entries=store.viewEntries();
    const shown=entries.filter(({item,location})=>(filter==='all'||filter===location||(location==='category:'&&filter==='category:'+store.working.categories[0]?.ID)) && [item.Name,item.URL,item.Desc].some(value=>String(value||'').toLocaleLowerCase().includes(query)));
    $('total-count').textContent=entries.length+t(' 条');
    $('result-count').textContent=(query||filter!=='all')?t('找到 {0} 条，共 {1} 条',shown.length,entries.length):t('共 {0} 条',entries.length);
    const list=$('entry-list'),rowCache=new Map();
    for(let row=list.firstElementChild;row;row=row.nextElementSibling)if(row.dataset.key)rowCache.set(row.dataset.key,row);
    const rows=[],categoryLabels=new Map(store.working.categories.map(category=>['category:'+category.ID,category.Name]));categoryLabels.set('apps',t('应用区'));categoryLabels.set('category:',categoryName('category:'));
    if(!shown.length){const empty=el('div',undefined,'empty');empty.append(el('p',entries.length?t('没有找到匹配条目'):t('还没有条目，从新增或导入开始。')));empty.append(button(entries.length?t('清除筛选'):t('新增条目'),()=>{if(entries.length){$('entry-search').value='';$('location-filter').value='all';render();}else openEntry();}));rows.push(empty);}
    for(const {item,location} of shown){
      let row=rowCache.get(item._key);
      if(!row){
        const key=item._key;row=el('article',undefined,'entry');row.setAttribute('role','listitem');row.dataset.key=key;
        const main=el('div',undefined,'entry-main'),name=entryButton('','edit','entry-name'),address=el('span',undefined,'entry-address');main.append(name,address);
        const place=el('span',undefined,'entry-location'),actions=el('div',undefined,'entry-actions');actions.append(entryButton(t('编辑'),'edit'),entryButton(t('删除'),'delete','danger'));
        row.append(main,place,actions);
      }
      const {Name,URL}=item,name=row.firstElementChild.firstElementChild,address=name.nextElementSibling,place=row.firstElementChild.nextElementSibling;
      if(name.textContent!==Name)name.textContent=Name;
      if(address.textContent!==URL){address.textContent=URL;address.title=URL;}
      const label=categoryLabels.get(location) || t('分类缺失：')+location;if(place.textContent!==label)place.textContent=label;
      rows.push(row);
    }
    // Keep unchanged rows attached (including focus). Bulk replacement handles
    // filtering without repeatedly mutating a live HTMLCollection.
    let cursor=list.firstElementChild, unchanged=true;
    for(const row of rows){if(row!==cursor){unchanged=false;break;}cursor=cursor.nextElementSibling;}
    if(cursor)unchanged=false;
    if(!unchanged){const fragment=document.createDocumentFragment();for(const row of rows)fragment.append(row);list.replaceChildren(fragment);}
    $('undo-area').hidden=!undo;syncStatus();
  }
  function open(dialog) {focusBeforeDialog=document.activeElement;dialog.showModal();}
  function close(dialog) {dialog.close();if(focusBeforeDialog?.isConnected)focusBeforeDialog.focus();else $('new-entry').focus();}
  function formValue() {return JSON.stringify(Object.fromEntries(new FormData($('entry-form'))));}
  function hasUnsavedChanges() {return store?.dirty || batch?.dirty || (entryDialog.open && formValue()!==formStart);}
  function clearErrors() {for(const name of ['name','url','location']){$('error-'+name).textContent='';$(fields[name==='name'?'Name':name==='url'?'URL':'Category']).removeAttribute('aria-invalid');}}
  function fieldError(name,message) {const id=name==='Name'?'name':name==='URL'?'url':'location';$('error-'+id).textContent=message;$(fields[name]).setAttribute('aria-invalid','true');$(fields[name]).focus();}
  function openEntry(key=null) {
    if(saving)return;editingKey=key;const entry=key?store.find(key):null;
    $('entry-form').reset();clearErrors();$('entry-options').open=false;
    $('entry-dialog-title').textContent=entry?t('编辑条目'):t('新增条目');
    for(const name of ['Name','URL','Icon','Desc'])$('entry-form').elements[name].value=entry?.item[name]||'';
    const selected=entry?.location || ($('location-filter').value!=='all'?$('location-filter').value:lastLocation);
    locations($('entry-location'),selected);formStart=formValue();iconPreview();open(entryDialog);
  }
  function cancelEntry() {if(formValue()!==formStart && !confirm(t('放弃本次表单修改？')))return;close(entryDialog);}
  function iconPreview() {
    const value=$('entry-icon').value.trim(), preview=$('icon-preview');preview.replaceChildren();
    if(!value){preview.textContent='◇';return;}
    let src;
    if(/^https?:\/\//i.test(value))src=value;
    else if(/^[a-zA-Z0-9-]+$/.test(value))src='/editor/icon?name='+encodeURIComponent(value);
    if(!src){preview.textContent='◇';return;}
    const img=el('img');img.alt='';img.referrerPolicy='no-referrer';img.src=src;img.onerror=()=>{if(img.parentNode===preview)preview.textContent='◇';};preview.append(img);
  }
  $('entry-icon').addEventListener('input',iconPreview);
  $('entry-form').addEventListener('submit',event=>{
    event.preventDefault();clearErrors();const values=Object.fromEntries(new FormData(event.target));
    if(!values.Name.trim())return fieldError('Name',t('请填写名称'));if(!values.URL.trim())return fieldError('URL',t('请填写地址'));
    if(values.Location!=='apps' && values.Location!=='category:' && !store.working.categories.some(c=>'category:'+c.ID===values.Location))return fieldError('Category',t('请选择有效位置'));
    const existed=editingKey && store.find(editingKey);
    const item={Name:values.Name.trim(),URL:values.URL.trim(),Icon:values.Icon,Desc:values.Desc};
    try{editingKey=store.put(item,values.Location,editingKey);}catch(error){setStatus(stateErrorText(error),true);return;}lastLocation=values.Location;undo=null;close(entryDialog);render();
    focusEntry(editingKey);
    if(existed && $('location-filter').value!=='all' && $('location-filter').value!==values.Location)setStatus(t('条目已移至“')+categoryName(values.Location)+t('”，保存后生效。'));
  });
  function removeEntry(key) {
    if(saving)return;undo=store.remove(key);render();$('undo-delete').focus();setStatus(t('条目已移除，保存后生效；可撤销本次删除。'));
  }
  $('undo-delete').onclick=()=>{const key=undo?.item._key;store.restore(undo);undo=null;render();focusEntry(key);};
  $('new-entry').onclick=()=>openEntry();$('cancel-entry').onclick=cancelEntry;
  entryDialog.addEventListener('cancel',event=>{event.preventDefault();cancelEntry();});
  function validCategoryName(name,id) {return name.trim() && name.trim()!=='[Flare 应用]' && !store.working.categories.some(c=>c.ID!==id && c.Name.trim()===name.trim());}
  function removeCategory(id,destination) {
    const moved=store.working.bookmarks.filter(item=>item.Category===id);
    store.working.bookmarks=store.working.bookmarks.filter(item=>item.Category!==id);
    if(destination==='apps')store.working.apps.push(...moved.map(item=>({...item,Category:''})));
    else store.working.bookmarks.push(...moved.map(item=>({...item,Category:destination.slice('category:'.length)})));
    store.working.categories=store.working.categories.filter(c=>c.ID!==id);migrationID=null;undo=null;
    if($('location-filter').value==='category:'+id)$('location-filter').value='all';renderCategories();render();
  }
  function renderCategories() {
    const list=$('category-list');list.replaceChildren();
    if(!store.working.categories.length)list.append(el('p',t('还没有书签分类。'),'muted'));
    store.working.categories.forEach((category,index)=>{
      const row=el('div',undefined,'category'),top=el('div',undefined,'category-top');
      const input=el('input');input.value=category.Name;input.setAttribute('aria-label',t('分类名称 ')+(index+1));input.dataset.category=category.ID;
      const error=el('p',undefined,'field-error');error.setAttribute('role','status');
      input.onchange=()=>{if(!validCategoryName(input.value,category.ID)){error.textContent=t('名称不能为空、重复或使用保留名称');input.setAttribute('aria-invalid','true');return;}category.Name=input.value.trim();error.textContent='';input.removeAttribute('aria-invalid');render();};top.append(input);row.append(top,error);
      const tools=el('div',undefined,'category-tools');const count=store.working.bookmarks.filter(b=>b.Category===category.ID).length;tools.append(el('span',count+t(' 条书签')));
      for(const [label,offset] of [[t('上移'),-1],[t('下移'),1]]){const move=button(label,()=>{const target=index+offset;[store.working.categories[index],store.working.categories[target]]=[store.working.categories[target],store.working.categories[index]];renderCategories();render();categoryDialog.querySelectorAll('[data-category]')[target]?.focus();});move.disabled=index+offset<0||index+offset>=store.working.categories.length;tools.append(move);}
      tools.append(button(t('删除'),()=>{if(count){migrationID=category.ID;renderCategories();}else if(confirm(t('删除空分类“')+category.Name+'”？'))removeCategory(category.ID,'apps');},'danger'));row.append(tools);
      if(migrationID===category.ID){const migration=el('div',undefined,'migration');migration.append(el('p',t('包含 ')+count+t(' 条书签，请选择迁入位置。')));
        const select=el('select');select.setAttribute('aria-label',t('迁入位置'));locations(select,'',false,category.ID);
        const actions=el('div',undefined,'dialog-actions');actions.append(button(t('取消'),()=>{migrationID=null;renderCategories();}),button(t('迁移并删除'),()=>{if(!select.value)return select.focus();removeCategory(category.ID,select.value);},'danger'));migration.append(select,actions);row.append(migration);}
      list.append(row);
    });
  }
  $('manage-categories').onclick=()=>{if(saving)return;migrationID=null;$('category-error').textContent='';renderCategories();open(categoryDialog);};
  $('category-add-form').onsubmit=event=>{event.preventDefault();const name=$('new-category-name').value.trim();if(!validCategoryName(name)){$('category-error').textContent=t('名称不能为空、重复或使用保留名称');$('new-category-name').focus();return;}store.working.categories.push({ID:store.categoryID(),Name:name});$('new-category-name').value='';$('category-error').textContent='';renderCategories();render();};
  function closeCategories() {if(categoryDialog.querySelector('[aria-invalid="true"]')){categoryDialog.querySelector('[aria-invalid="true"]').focus();return;}if($('new-category-name').value.trim()&&!confirm(t('尚未添加的新分类名称将被放弃，继续？')))return;$('new-category-name').value='';close(categoryDialog);batch?.refresh();}
  $('close-categories').onclick=closeCategories;categoryDialog.addEventListener('cancel',event=>{event.preventDefault();closeCategories();});
  $('entry-list').addEventListener('click',event=>{const button=event.target.closest('[data-entry-action]'),row=button?.closest('[data-key]');if(!row)return;if(button.dataset.entryAction==='delete')removeEntry(row.dataset.key);else openEntry(row.dataset.key);});
  $('entry-search').oninput=()=>{if(store){clearTimeout(filterTimer);filterTimer=setTimeout(render,100);}};$('location-filter').onchange=()=>{if(store)render();};
  function busy(value) {saving=value;$('workspace').inert=value;$('workspace').setAttribute('aria-busy',String(value));document.querySelectorAll('.save-actions button').forEach(b=>b.disabled=value);if(!value)syncStatus();}
  function validateDocument() {
    const names=new Set(),ids=new Set();
    for(let i=0;i<store.working.categories.length;i++){const c=store.working.categories[i];if(!c.ID||ids.has(c.ID)||!c.Name.trim()||names.has(c.Name.trim())||c.Name.trim()==='[Flare 应用]')return {field:'categories.'+i+'.Name',error:t('分类名称或 ID 无效')};names.add(c.Name.trim());ids.add(c.ID);}
    for(const collection of ['apps','bookmarks'])for(let i=0;i<store.working[collection].length;i++){const item=store.working[collection][i];for(const name of ['Name','URL'])if(!String(item[name]||'').trim())return{field:collection+'.'+i+'.'+name,error:name==='Name'?t('请填写名称'):t('请填写地址')};if(collection==='bookmarks'&&item.Category&&!ids.has(item.Category))return{field:collection+'.'+i+'.Category',error:t('请选择有效位置')};}
  }
  function locateError(result) {
    if(batch?.active && !result.field.startsWith('categories.')) { batch.locateError(result); return; }
    const [collection,index,field]=result.field.split('.');
    if(collection==='categories'){renderCategories();open(categoryDialog);const input=categoryDialog.querySelectorAll('[data-category]')[Number(index)];if(input){input.setAttribute('aria-invalid','true');input.focus();}}
    else {const item=store.working[collection]?.[Number(index)];if(item){openEntry(item._key);fieldError(field,result.error);}}
  }
  async function save() {
    if(!store||saving||recoveryRequired||entryDialog.open||categoryDialog.open)return;
    if(batch?.active && !batch.flush())return;
    if(!store.dirty){syncStatus();return;}
    const invalid=validateDocument();if(invalid){locateError(invalid);setStatus(invalid.error,true);return;}
    busy(true);setStatus(t('正在保存…'));let message=t('所有更改已保存'),error=false, responseParsed=false, serverInvalid=null;
    try {
      const response=await fetch('/editor/document',{method:'POST',headers:{'Content-Type':'application/json','X-Flare-Editor':'1'},body:JSON.stringify(store.toJSON()),signal:AbortSignal.timeout(30000)});
      if(response.redirected||!(response.headers.get('content-type')||'').includes('application/json'))throw Error(t('登录可能已过期，请在新窗口登录后重试。'));
      const result=await response.json();responseParsed=true;
      if(!response.ok){if(result.recoveryRequired){recoveryRequired=true;$('export-draft').hidden=false;$('reload-data').hidden=false;}if(response.status===409){conflict=true;$('export-draft').hidden=false;$('reload-data').hidden=false;}if(result.field)serverInvalid=result;throw Error(result.error||t('保存失败，请重试。'));}
      if(!result.saved||!result.document)throw Error(t('保存响应不完整，请检查数据后重试。'));
      store.accept(result.document);batch?.refresh();undo=null;conflict=false;$('export-draft').hidden=true;$('reload-data').hidden=true;render();
    } catch(err){
      error=true;
      if(!responseParsed && (err.name==='TimeoutError'||err.name==='AbortError'||err instanceof TypeError||err instanceof SyntaxError)){
        message=t('保存结果待确认：请求中断或响应无法读取，服务端可能已经保存。请先导出草稿，再检查服务端数据；重新保存仍会校验版本。 当前编辑内容已保留。');
        $('export-draft').hidden=false;$('reload-data').hidden=false;
      }else message=err.message+t(' 当前编辑内容已保留。');
    }
    finally{busy(false);if(serverInvalid)locateError(serverInvalid);setStatus(message,error);}
  }
  $('save-changes').onclick=save;
  $('discard-changes').onclick=()=>{if(!saving&&confirm(t('放弃全部未保存的更改？'))){store.discard();batch?.refresh();undo=null;$('location-filter').value='all';render();if(conflict)setStatus(t('已恢复加载时的数据；服务器已有更新，请重新载入。'),true);}};
  $('export-draft').onclick=()=>{let blob;try{blob=FlareEditorState.draftBlob(batch?.active ? batch.exportDraft() : store.toJSON());}catch(error){setStatus(t('无法导出草稿：')+stateErrorText(error),true);return;}const url=URL.createObjectURL(blob);const a=el('a');a.href=url;a.download='flare-editor-draft.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);};
  $('reload-data').onclick=()=>{if(!saving&&(!(store.dirty||batch?.dirty)||confirm(t('重新载入会放弃当前修改，请先导出需要保留的内容。继续？'))))location.reload();};
  $('restore-draft').onclick=()=>{if(!saving&&!entryDialog.open&&!categoryDialog.open)$('draft-file').click();};
  $('draft-file').onchange=async event=>{
    const file=event.target.files[0];if(!file)return;
    try {
      if(file.size>FlareEditorState.maxDraftBytes)throw Error(t('草稿文件不能超过 64 MiB'));
      const draft=JSON.parse(await file.text());
      if(!draft || !Array.isArray(draft.categories) || !Array.isArray(draft.apps) || !Array.isArray(draft.bookmarks))throw Error(t('不是有效的编辑器草稿'));
      const categories=draft.categories.map(c=>{if(!c||typeof c.ID!=='string'||typeof c.Name!=='string')throw Error(t('分类格式无效'));return{ID:c.ID,Name:c.Name};});
      // Handsontable exports cleared text cells as null; keep them editable as empty strings.
      const items=rows=>rows.map(item=>{if(!item||!['Name','URL'].every(name=>item[name]===null||typeof item[name]==='string'))throw Error(t('条目格式无效'));const out={Name:item.Name??'',URL:item.URL??'',Category:typeof item.Category==='string'?item.Category:''};for(const name of ['Icon','Desc'])if(typeof item[name]==='string')out[name]=item[name];if(typeof item.Private==='boolean')out.Private=item.Private;return out;});
      const next={version:store.working.version,categories,apps:items(draft.apps),bookmarks:items(draft.bookmarks)};
      if(!confirm(t('以草稿替换当前待保存内容？保存时仍会检查本页加载的服务器版本。')))return;
      store.working=FlareEditorState.normalize(next);batch?.refresh();undo=null;$('location-filter').value='all';render();setStatus(t('草稿已恢复到待保存内容，请检查后保存。'));
    }catch(error){setStatus(t('无法恢复草稿：')+error.message,true);}finally{event.target.value='';}
  };
  document.querySelectorAll('[data-leave]').forEach(link=>link.onclick=event=>{if(saving||(hasUnsavedChanges()&&!confirm(t('更改尚未保存，确定离开？'))))event.preventDefault();});
  window.addEventListener('beforeunload',event=>{if(saving||hasUnsavedChanges()){event.preventDefault();event.returnValue='';}});
  // The lazy batch adapter attaches to this single working store.
  window.FlareEditorUI={get store(){return store;},get saving(){return saving;},render,setStatus,syncStatus,validateDocument,locateError,registerBatch(adapter){batch=adapter;},get hasOpenDialog(){return entryDialog.open||categoryDialog.open;}};
  try {const response=await fetch('/editor/document',{cache:'no-store'});if(!response.ok||response.redirected||!(response.headers.get('content-type')||'').includes('application/json'))throw Error(t('无法读取导航数据，请重新登录或检查配置。'));const serverDoc=await response.json();store=new FlareEditorState.Store(serverDoc);if(serverDoc.recoveryPaths?.length){const warning=$('recovery-warning');warning.hidden=false;warning.textContent=t('发现服务端恢复副本，请先检查恢复目录及数据一致性，再继续编辑：')+serverDoc.recoveryPaths.join(', ');}$('workspace').setAttribute('aria-busy','false');render();document.dispatchEvent(new Event('flare-editor-ready'));}
  catch(error){$('result-count').textContent=t('加载失败');setStatus(error.message,true);$('workspace').inert=true;}
})();
