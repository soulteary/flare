/* Lazy, table-only adapter over the same working document as the list. */
(function () {
  'use strict';
  const t = window.FlareEditorI18n?.t || ((text,...args)=>text.replace(/\{(\d+)\}/g,(_,i)=>String(args[i]??'')));
  const $=id=>document.getElementById(id);
  let ui,table=null,active=false,loading=false,baseline='',selected=[],column=0,assets;
  const clone=value=>JSON.parse(JSON.stringify(value));
  function snapshotRows(){return ui.store.viewEntries().map(({item,location})=>({...clone(item),Location:location}));}
  function rows(){return Array.from({length:table.countRows()},(_,r)=>({...table.getSourceDataAtRow(table.toPhysicalRow(r))}));}
  function serial(data){return JSON.stringify(data.map(r=>[r._key||'',r.Name||'',r.URL||'',r.Location||'',r.Icon||'',r.Desc||'',!!r.Private]));}
  function notify(){if(!loading&&ui)ui.syncStatus();}
  function finish(){const editor=table?.getActiveEditor();if(editor?.isOpened())editor.finishEditing(false);}
  function labels(){return Object.fromEntries([['apps',t('editor.applications')],['category:',ui.store.working.categories.length?t('editor.default_category',{name:ui.store.working.categories[0].Name}):t('editor.uncategorized_bookmarks')],...ui.store.working.categories.map(c=>['category:'+c.ID,c.Name])]);}
  function columns(){return [{data:'Name'},{data:'URL'},{data:'Location',editor:'select',selectOptions:labels(),renderer(instance,td,row,col,prop,value,properties){Handsontable.renderers.TextRenderer(instance,td,row,col,prop,labels()[value]||t('editor.missing_category',{location:value}),properties);}},{data:'Icon'},{data:'Desc'}];}
  function select(r1,c1,r2){selected=Array.from({length:Math.abs(r2-r1)+1},(_,i)=>Math.min(r1,r2)+i).filter(r=>r>=0&&r<table.countRows());column=Math.max(0,c1);$('batch-selection').textContent=t('editor.row_selection_count',{total:table.countRows(),selected:selected.length});$('batch-delete').disabled=!selected.length;$('batch-edit').disabled=!selected.length;$('batch-up').disabled=selected.length!==1||selected[0]===0;$('batch-down').disabled=selected.length!==1||selected[0]===table.countRows()-1;}
  function buildDocument(data){const doc=ui.store.toJSON();doc.apps=[];doc.bookmarks=[];for(const row of data){const {Location,...item}=row;item.Category=Location==='apps'?'':String(Location||'').startsWith('category:')?Location.slice(9):String(Location||'');(Location==='apps'?doc.apps:doc.bookmarks).push(item);}return doc;}
  function validate(data){const known=labels();for(let r=0;r<data.length;r++){const row=data[r];if(!String(row.Name||'').trim())return{row:r,col:0,error:t('editor.enter_a_name_or_delete_the_empty_row')};if(!String(row.URL||'').trim())return{row:r,col:1,error:t('editor.enter_an_address_or_delete_the_empty_row')};if(!Object.hasOwn(known,row.Location))return{row:r,col:2,error:t('editor.choose_a_valid_location')};}}
  function focusError(error){table.selectCell(error.row,error.col);table.scrollViewportTo(error.row,error.col);ui.setStatus(t('editor.row_error',{row:error.row+1,error:error.error}),true);}
  const adapter={
    get active(){return active;},
    get dirty(){if(!active||!table)return false;const editor=table.getActiveEditor();return !!(editor?.isOpened()&&String(editor.getValue())!==String(editor.originalValue))||serial(rows())!==baseline;},
    flush(){if(!active||!table)return true;finish();const data=rows(),invalid=validate(data);if(invalid){focusError(invalid);return false;}ui.store.working=FlareEditorState.normalize(buildDocument(data));baseline=serial(data);ui.syncStatus();return true;},
    refresh(){if(!active||!table)return;loading=true;table.updateSettings({columns:columns()});table.loadData(snapshotRows());baseline=serial(rows());selected=[];$('batch-selection').textContent=t('editor.row_count',{count:table.countRows()});for(const id of ['batch-edit','batch-delete','batch-up','batch-down'])$(id).disabled=true;loading=false;notify();},
    exportDraft(){finish();return active?FlareEditorState.payload(buildDocument(rows())):ui.store.toJSON();},
    locateError(result){const [collection,index,field]=result.field.split('.');const item=ui.store.working[collection]?.[Number(index)];const row=rows().findIndex(r=>r._key===item?._key);focusError({row:Math.max(0,row),col:{Name:0,URL:1,Category:2}[field]||0,error:result.error});}
  };
  function loadAssets(){
    const version=document.documentElement.dataset.editorAssetVersion || '';
    const assetURL=name=>'/assets/editor/'+name+'?v='+encodeURIComponent(version);
    if(assets)return assets;
    if(!document.querySelector('link[data-flare-batch-style]')){const css=document.createElement('link');css.rel='stylesheet';css.dataset.flareBatchStyle='';css.href=assetURL('handsontable.full.min.css');document.head.append(css);}
    const load=src=>new Promise((resolve,reject)=>{const script=document.createElement('script');script.src=src;script.onload=resolve;script.onerror=()=>{script.remove();reject(Error(t('editor.cannot_load_batch_editing_please_retry')));};document.head.append(script);});
    assets=load(assetURL('handsontable.full.min.js')).then(()=>load(assetURL('zh-CN.min.js'))).catch(error=>{assets=null;throw error;});return assets;

  }

  async function enter(){if(!ui?.store||ui.saving||ui.hasOpenDialog||active)return;try{ui.setStatus(t('editor.loading_batch_editing'));await loadAssets();if(ui.saving||ui.hasOpenDialog)return;active=true;$('list-view').hidden=true;$('batch-view').hidden=false;document.querySelector('.menu').open=false;if(table){adapter.refresh();table.render();$('leave-batch').focus();return;}loading=true;
    table=new Handsontable($('batch-table'),{data:snapshotRows(),dataSchema:{Name:'',URL:'',Location:'apps',Icon:'',Desc:'',Private:document.querySelector('meta[name=trusted-mode]')?.content!=='true'},columns:columns(),language:document.documentElement.lang==='en'?'en-US':'zh-CN',colHeaders:[t('editor.name'),t('editor.address'),t('editor.location'),t('editor.icon'),t('editor.description')],colWidths:[180,300,180,160,260],rowHeaders:true,height:'auto',manualRowMove:true,outsideClickDeselects:false,licenseKey:'non-commercial-and-evaluation',contextMenu:['row_above','row_below','---------','undo','redo','---------','cut','copy'],afterSelectionEnd:select,afterChange:notify,afterCreateRow:notify,afterRemoveRow:notify,afterRowMove:notify});
    baseline=serial(rows());loading=false;$('batch-selection').textContent=t('editor.row_count',{count:table.countRows()});$('leave-batch').focus();notify();
  }catch(error){active=false;loading=false;$('list-view').hidden=false;$('batch-view').hidden=true;ui.setStatus(error.message,true);}}
  function leave(){if(ui.saving||!adapter.flush())return;active=false;table.deselectCell();table.unlisten();$('batch-view').hidden=true;$('list-view').hidden=false;ui.render();document.querySelector('.menu summary').focus();}
  function init(){ui=window.FlareEditorUI;ui.registerBatch(adapter);window.FlareEditorBatch=adapter;
    $('open-batch').onclick=enter;$('leave-batch').onclick=leave;
    $('batch-add').onclick=()=>{finish();const row=table.countRows();table.alter('insert_row',row,1);table.selectCell(row,0);const editor=table.getActiveEditor();if(editor){editor.beginEditing();editor.focus();}notify();};
    $('batch-edit').onclick=()=>{finish();if(!selected.length)return;table.selectCell(selected[0],column);const editor=table.getActiveEditor();if(editor){editor.beginEditing();editor.focus();}notify();};
    $('batch-delete').onclick=()=>{finish();if(!selected.length||!confirm(t('editor.confirm_delete_selected_rows',{count:selected.length})))return;[...selected].sort((a,b)=>b-a).forEach(r=>table.alter('remove_row',r,1));selected=[];for(const id of ['batch-edit','batch-delete','batch-up','batch-down'])$(id).disabled=true;notify();};
    for(const [id,offset]of[['batch-up',-1],['batch-down',1]])$(id).onclick=()=>{finish();if(selected.length!==1)return;const target=selected[0]+offset;table.getPlugin('manualRowMove').moveRow(selected[0],target);table.render();table.selectCell(target,column);notify();};
    $('batch-categories').onclick=()=>{if(adapter.flush())$('manage-categories').click();};
    $('batch-reset').onclick=()=>{if(!adapter.dirty||confirm(t('editor.discard_table_edits_not_yet_synchronized_changes_already_in')))adapter.refresh();};
    $('batch-table').addEventListener('input',notify);
    if(location.pathname==='/editor/batch'||new URLSearchParams(location.search).get('view')==='batch')enter();
  }
  document.addEventListener('flare-editor-ready',init,{once:true});if(window.FlareEditorUI?.store)init();
})();
