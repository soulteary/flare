/* List and form editor. All rendering uses text nodes, never user-supplied HTML. */
(async function () {
  'use strict';
  const $ = id => document.getElementById(id);
  let store, saving=false, editingKey=null, formStart='', lastLocation='', undo=null, migrationID=null, conflict=false, focusBeforeDialog=null;
  const entryDialog=$('entry-dialog'), categoryDialog=$('categories-dialog');
  const fields={Name:'entry-name',URL:'entry-url',Category:'entry-location'};
  function el(tag, text, className) { const node=document.createElement(tag); if(text!==undefined)node.textContent=text;if(className)node.className=className;return node; }
  function button(text, action, className) {const node=el('button',text,className);node.type='button';node.addEventListener('click',action);return node;}
  function categoryName(id) {return id==='apps' ? '应用区' : (store.working.categories.find(c=>c.ID===id)?.Name || '分类缺失：'+id);}
  function locations(select, value, includeAll=false, exclude=null) {
    select.replaceChildren();
    function option(v,t){const o=el('option',t);o.value=v;select.append(o);}
    if(includeAll)option('all','全部位置');else option('','请选择位置');
    option('apps','应用区');
    for(const c of store.working.categories)if(c.ID!==exclude)option(c.ID,c.Name);
    if(value && ![...select.options].some(o=>o.value===value))option(value,'分类缺失：'+value);
    select.value=value || (includeAll?'all':'');
  }
  function setStatus(message, error=false) {
    $('save-status').textContent=message;
    document.querySelector('.savebar').dataset.error=String(error);
    if(store){$('save-changes').disabled=saving || !store.dirty;$('discard-changes').disabled=saving || !store.dirty;}
  }
  function syncStatus() {setStatus(conflict ? '服务器数据已变化，请保留修改后重新载入。' : store.dirty?'有未保存的更改':'所有更改已保存', conflict);}
  function render() {
    const filter=$('location-filter').value, query=$('entry-search').value.trim().toLocaleLowerCase();
    locations($('location-filter'),filter,true);
    const entries=store.entries();
    const shown=entries.filter(({item,location})=>(filter==='all'||filter===location) && [item.Name,item.URL,item.Desc].some(value=>String(value||'').toLocaleLowerCase().includes(query)));
    $('total-count').textContent=entries.length+' 条';
    $('result-count').textContent=(query||filter!=='all')?`找到 ${shown.length} 条，共 ${entries.length} 条`:`共 ${entries.length} 条`;
    const list=$('entry-list');list.replaceChildren();
    if(!shown.length){const empty=el('div',undefined,'empty');empty.append(el('p',entries.length?'没有找到匹配条目':'还没有条目，从新增或导入开始。'));empty.append(button(entries.length?'清除筛选':'新增条目',()=>{if(entries.length){$('entry-search').value='';$('location-filter').value='all';render();}else openEntry();}));list.append(empty);}
    for(const {item,location} of shown){
      const row=el('article',undefined,'entry');row.setAttribute('role','listitem');row.dataset.key=item._key;
      const main=el('div',undefined,'entry-main');main.append(button(item.Name,()=>openEntry(item._key),'entry-name'));
      const address=el('span',item.URL,'entry-address');address.title=item.URL;main.append(address);
      const place=el('span',categoryName(location),'entry-location');
      const actions=el('div',undefined,'entry-actions');actions.append(button('编辑',()=>openEntry(item._key)),button('删除',()=>removeEntry(item._key),'danger'));
      row.append(main,place,actions);list.append(row);
    }
    $('undo-area').hidden=!undo;syncStatus();
  }
  function open(dialog) {focusBeforeDialog=document.activeElement;dialog.showModal();}
  function close(dialog) {dialog.close();if(focusBeforeDialog?.isConnected)focusBeforeDialog.focus();}
  function formValue() {return JSON.stringify(Object.fromEntries(new FormData($('entry-form'))));}
  function clearErrors() {for(const name of ['name','url','location']){$('error-'+name).textContent='';$(fields[name==='name'?'Name':name==='url'?'URL':'Category']).removeAttribute('aria-invalid');}}
  function fieldError(name,message) {const id=name==='Name'?'name':name==='URL'?'url':'location';$('error-'+id).textContent=message;$(fields[name]).setAttribute('aria-invalid','true');$(fields[name]).focus();}
  function openEntry(key=null) {
    if(saving)return;editingKey=key;const entry=key?store.find(key):null;
    $('entry-form').reset();clearErrors();$('entry-options').open=false;
    $('entry-dialog-title').textContent=entry?'编辑条目':'新增条目';
    for(const name of ['Name','URL','Icon','Desc'])$('entry-form').elements[name].value=entry?.item[name]||'';
    const selected=entry?.location || ($('location-filter').value!=='all'?$('location-filter').value:lastLocation);
    locations($('entry-location'),selected);formStart=formValue();iconPreview();open(entryDialog);
  }
  function cancelEntry() {if(formValue()!==formStart && !confirm('放弃本次表单修改？'))return;close(entryDialog);}
  function iconPreview() {
    const value=$('entry-icon').value.trim(), preview=$('icon-preview');preview.replaceChildren();
    if(!value){preview.textContent='◇';return;}
    let src;
    if(/^https?:\/\//i.test(value))src=value;
    else if(/^[a-zA-Z0-9-]+$/.test(value))src='/assets/mdi/blackboard-'+value.toLowerCase()+'.svg';
    if(!src){preview.textContent='◇';return;}
    const img=el('img');img.alt='';img.referrerPolicy='no-referrer';img.src=src;img.onerror=()=>{preview.textContent='◇';};preview.append(img);
  }
  $('entry-icon').addEventListener('input',iconPreview);
  $('entry-form').addEventListener('submit',event=>{
    event.preventDefault();clearErrors();const values=Object.fromEntries(new FormData(event.target));
    if(!values.Name.trim())return fieldError('Name','请填写名称');if(!values.URL.trim())return fieldError('URL','请填写地址');
    if(values.Location!=='apps' && !store.working.categories.some(c=>c.ID===values.Location))return fieldError('Category','请选择有效位置');
    const existed=editingKey && store.find(editingKey);
    const item={Name:values.Name.trim(),URL:values.URL.trim(),Icon:values.Icon,Desc:values.Desc};
    store.put(item,values.Location,editingKey);lastLocation=values.Location;undo=null;close(entryDialog);render();
    const updated=[...$('entry-list').children].find(row=>row.dataset.key===editingKey);
    (updated?.querySelector('.entry-name') || $('new-entry')).focus();
    if(existed && $('location-filter').value!=='all' && $('location-filter').value!==values.Location)setStatus('条目已移至“'+categoryName(values.Location)+'”，保存后生效。');
  });
  function removeEntry(key) {
    if(saving)return;undo=store.remove(key);render();setStatus('条目已移除，保存后生效；可撤销本次删除。');
  }
  $('undo-delete').onclick=()=>{store.restore(undo);undo=null;render();};
  $('new-entry').onclick=()=>openEntry();$('cancel-entry').onclick=cancelEntry;
  entryDialog.addEventListener('cancel',event=>{event.preventDefault();cancelEntry();});
  function validCategoryName(name,id) {return name.trim() && name.trim()!=='[Flare 应用]' && !store.working.categories.some(c=>c.ID!==id && c.Name.trim()===name.trim());}
  function removeCategory(id,destination) {
    const moved=store.working.bookmarks.filter(item=>item.Category===id);
    store.working.bookmarks=store.working.bookmarks.filter(item=>item.Category!==id);
    if(destination==='apps')store.working.apps.push(...moved.map(item=>({...item,Category:''})));
    else store.working.bookmarks.push(...moved.map(item=>({...item,Category:destination})));
    store.working.categories=store.working.categories.filter(c=>c.ID!==id);migrationID=null;undo=null;
    if($('location-filter').value===id)$('location-filter').value='all';renderCategories();render();
  }
  function renderCategories() {
    const list=$('category-list');list.replaceChildren();
    if(!store.working.categories.length)list.append(el('p','还没有书签分类。','muted'));
    store.working.categories.forEach((category,index)=>{
      const row=el('div',undefined,'category'),top=el('div',undefined,'category-top');
      const input=el('input');input.value=category.Name;input.setAttribute('aria-label','分类名称 '+(index+1));input.dataset.category=category.ID;
      const error=el('p',undefined,'field-error');error.setAttribute('role','status');
      input.onchange=()=>{if(!validCategoryName(input.value,category.ID)){error.textContent='名称不能为空、重复或使用保留名称';input.setAttribute('aria-invalid','true');return;}category.Name=input.value.trim();error.textContent='';input.removeAttribute('aria-invalid');render();};top.append(input);row.append(top,error);
      const tools=el('div',undefined,'category-tools');const count=store.working.bookmarks.filter(b=>b.Category===category.ID).length;tools.append(el('span',count+' 条书签'));
      for(const [label,offset] of [['上移',-1],['下移',1]]){const move=button(label,()=>{const target=index+offset;[store.working.categories[index],store.working.categories[target]]=[store.working.categories[target],store.working.categories[index]];renderCategories();render();});move.disabled=index+offset<0||index+offset>=store.working.categories.length;tools.append(move);}
      tools.append(button('删除',()=>{if(count){migrationID=category.ID;renderCategories();}else if(confirm('删除空分类“'+category.Name+'”？'))removeCategory(category.ID,'apps');},'danger'));row.append(tools);
      if(migrationID===category.ID){const migration=el('div',undefined,'migration');migration.append(el('p','包含 '+count+' 条书签，请选择迁入位置。'));
        const select=el('select');select.setAttribute('aria-label','迁入位置');locations(select,'',false,category.ID);
        const actions=el('div',undefined,'dialog-actions');actions.append(button('取消',()=>{migrationID=null;renderCategories();}),button('迁移并删除',()=>{if(!select.value)return select.focus();removeCategory(category.ID,select.value);},'danger'));migration.append(select,actions);row.append(migration);}
      list.append(row);
    });
  }
  $('manage-categories').onclick=()=>{if(saving)return;migrationID=null;$('category-error').textContent='';renderCategories();open(categoryDialog);};
  $('category-add-form').onsubmit=event=>{event.preventDefault();const name=$('new-category-name').value.trim();if(!validCategoryName(name)){$('category-error').textContent='名称不能为空、重复或使用保留名称';$('new-category-name').focus();return;}store.working.categories.push({ID:store.categoryID(),Name:name});$('new-category-name').value='';$('category-error').textContent='';renderCategories();render();};
  function closeCategories() {if(categoryDialog.querySelector('[aria-invalid="true"]')){categoryDialog.querySelector('[aria-invalid="true"]').focus();return;}if($('new-category-name').value.trim()&&!confirm('尚未添加的新分类名称将被放弃，继续？'))return;$('new-category-name').value='';close(categoryDialog);}
  $('close-categories').onclick=closeCategories;categoryDialog.addEventListener('cancel',event=>{event.preventDefault();closeCategories();});
  $('entry-search').oninput=()=>{if(store)render();};$('location-filter').onchange=()=>{if(store)render();};
  function busy(value) {saving=value;$('workspace').inert=value;$('workspace').setAttribute('aria-busy',String(value));document.querySelectorAll('.save-actions button').forEach(b=>b.disabled=value);if(!value)syncStatus();}
  function validateDocument() {
    const names=new Set(),ids=new Set();
    for(let i=0;i<store.working.categories.length;i++){const c=store.working.categories[i];if(!c.ID||ids.has(c.ID)||!c.Name.trim()||names.has(c.Name.trim())||c.Name.trim()==='[Flare 应用]')return {field:'categories.'+i+'.Name',error:'分类名称或 ID 无效'};names.add(c.Name.trim());ids.add(c.ID);}
    for(const collection of ['apps','bookmarks'])for(let i=0;i<store.working[collection].length;i++){const item=store.working[collection][i];for(const name of ['Name','URL'])if(!String(item[name]||'').trim())return{field:collection+'.'+i+'.'+name,error:name==='Name'?'请填写名称':'请填写地址'};if(collection==='bookmarks'&&!ids.has(item.Category))return{field:collection+'.'+i+'.Category',error:'请选择有效位置'};}
  }
  function locateError(result) {
    const [collection,index,field]=result.field.split('.');
    if(collection==='categories'){renderCategories();open(categoryDialog);const input=categoryDialog.querySelectorAll('[data-category]')[Number(index)];if(input){input.setAttribute('aria-invalid','true');input.focus();}}
    else {const item=store.working[collection]?.[Number(index)];if(item){openEntry(item._key);fieldError(field,result.error);}}
  }
  async function save() {
    if(!store||saving||!store.dirty||entryDialog.open||categoryDialog.open)return;
    const invalid=validateDocument();if(invalid){locateError(invalid);setStatus(invalid.error,true);return;}
    busy(true);setStatus('正在保存…');let message='所有更改已保存',error=false;
    try {
      const response=await fetch('/editor/document',{method:'POST',headers:{'Content-Type':'application/json','X-Flare-Editor':'1'},body:JSON.stringify(store.toJSON()),signal:AbortSignal.timeout(30000)});
      if(response.redirected||!(response.headers.get('content-type')||'').includes('application/json'))throw Error('登录可能已过期，请在新窗口登录后重试。');
      const result=await response.json();
      if(!response.ok){if(response.status===409){conflict=true;$('export-draft').hidden=false;$('reload-data').hidden=false;}if(result.field)locateError(result);throw Error(result.error||'保存失败，请重试。');}
      if(!result.saved||!result.document)throw Error('保存响应不完整，请检查数据后重试。');
      store.accept(result.document);undo=null;conflict=false;$('export-draft').hidden=true;$('reload-data').hidden=true;render();
    } catch(err){error=true;message=err.message+' 当前编辑内容已保留。';}
    finally{busy(false);setStatus(message,error);}
  }
  $('save-changes').onclick=save;
  $('discard-changes').onclick=()=>{if(!saving&&confirm('放弃全部未保存的更改？')){store.discard();undo=null;$('location-filter').value='all';render();if(conflict)setStatus('已恢复加载时的数据；服务器已有更新，请重新载入。',true);}};
  $('export-draft').onclick=()=>{const url=URL.createObjectURL(new Blob([JSON.stringify(store.toJSON(),null,2)],{type:'application/json'}));const a=el('a');a.href=url;a.download='flare-editor-draft.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);};
  $('reload-data').onclick=()=>{if(!saving&&(!store.dirty||confirm('重新载入会放弃当前修改，请先导出需要保留的内容。继续？')))location.reload();};
  document.querySelectorAll('[data-leave]').forEach(link=>link.onclick=event=>{if(saving||(store?.dirty&&!confirm('更改尚未保存，确定离开？')))event.preventDefault();});
  window.addEventListener('beforeunload',event=>{if(saving||store?.dirty){event.preventDefault();event.returnValue='';}});
  // The batch adapter in the next PR attaches to this single working store.
  window.FlareEditorUI={get store(){return store;},get saving(){return saving;},render,setStatus,validateDocument,locateError};
  try {const response=await fetch('/editor/document',{cache:'no-store'});if(!response.ok||response.redirected||!(response.headers.get('content-type')||'').includes('application/json'))throw Error('无法读取导航数据，请重新登录或检查配置。');store=new FlareEditorState.Store(await response.json());$('workspace').setAttribute('aria-busy','false');render();document.dispatchEvent(new Event('flare-editor-ready'));}
  catch(error){$('result-count').textContent='加载失败';setStatus(error.message,true);$('workspace').inert=true;}
})();
