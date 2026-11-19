const {test}=require('node:test');const assert=require('node:assert/strict');
const {Store}=require('../../internal/pages/editor/client/state.js');
const fixture=()=>({version:'v1',categories:[{ID:'stable',Name:'Original'}],apps:[],bookmarks:[{Name:'One',URL:'custom://a?b=c#d',Category:'stable',Private:true}]});
test('dirty state compares actual data and discard restores saved data',()=>{const s=new Store(fixture());assert.equal(s.dirty,false);s.working.categories[0].Name='Other';assert.equal(s.dirty,true);s.working.categories[0].Name='Original';assert.equal(s.dirty,false);s.working.bookmarks=[];s.discard();assert.equal(s.dirty,false);assert.equal(s.working.bookmarks.length,1)});
test('session keys survive filtering, moves and undo but are not persisted',()=>{const s=new Store(fixture());const k=s.entries()[0].item._key;s.put({Name:'Edited'},'apps',k);assert.equal(s.find(k).item.Private,true);assert.equal(s.find(k).item.Category,'');assert.equal(s.working.bookmarks.length,0);assert.equal(s.toJSON().apps[0]._key,undefined);s.restore(s.remove(k));assert.equal(s.find(k).item.Name,'Edited')});
test('category IDs remain independent from labels and snapshots do not alias',()=>{const source=fixture(),s=new Store(source);s.working.categories[0].Name='Rename';assert.equal(source.categories[0].Name,'Original');assert.equal(s.working.bookmarks[0].Category,'stable');assert.notEqual(s.categoryID(),s.categoryID());s.accept({...s.toJSON(),version:'v2'});assert.equal(s.dirty,false);assert.equal(s.toJSON().version,'v2')});
test('application placement cannot collide with a legacy category ID named apps',()=>{const s=new Store({version:'v',categories:[{ID:'apps',Name:'Legacy'}],apps:[{Name:'App',URL:'http://app'}],bookmarks:[{Name:'Bookmark',URL:'http://bookmark',Category:'apps'}]});assert.deepEqual(s.entries().map(e=>e.location),['apps','category:apps']);const k=s.working.bookmarks[0]._key;s.put({Name:'Renamed'},'category:apps',k);assert.equal(s.toJSON().bookmarks[0].Category,'apps');assert.equal(s.toJSON().apps.length,1)});
test('implicit default group remains a bookmark with or without named categories',()=>{for(const categories of [[],[{ID:'first',Name:'First'}]]){const s=new Store({version:'v',categories,apps:[],bookmarks:[{Name:'Implicit',URL:'http://nas',Category:'',Private:true}]});const entry=s.entries()[0];assert.equal(entry.location,'category:');s.put({Name:'Edited'},entry.location,entry.item._key);assert.equal(s.toJSON().apps.length,0);assert.equal(s.toJSON().bookmarks.length,1);assert.equal(s.toJSON().bookmarks[0].Category,'');assert.equal(s.toJSON().bookmarks[0].Private,true)}});
test('large draft exports remain importable and preserve hidden fields',async()=>{const {draftBlob,maxDraftBytes}=require('../../internal/pages/editor/client/state.js');const doc=fixture();doc.bookmarks[0].Desc='x'.repeat(5*1024*1024);const blob=draftBlob(doc);assert.ok(blob.size>4*1024*1024);assert.ok(blob.size<=maxDraftBytes);const restored=JSON.parse(await blob.text());assert.equal(restored.bookmarks[0].Private,true);assert.equal(restored.bookmarks[0].URL,doc.bookmarks[0].URL);assert.equal(restored.bookmarks[0].Desc.length,5*1024*1024);assert.throws(()=>draftBlob({...doc,bookmarks:[{Name:'too large',URL:'http://nas',Desc:'x'.repeat(maxDraftBytes)}]}),{code:'draft_too_large'});});
test('dirty checks cache serialization until an actual mutation, and remain reversible',()=>{const s=new Store(fixture()),stringify=JSON.stringify;let calls=0;JSON.stringify=(...args)=>{calls++;return stringify(...args);};try{for(let i=0;i<100;i++)assert.equal(s.dirty,false);assert.equal(calls,0);s.working.bookmarks[0].Private=false;assert.equal(s.dirty,true);const changedCalls=calls;for(let i=0;i<100;i++)assert.equal(s.dirty,true);assert.equal(calls,changedCalls);s.working.bookmarks[0].Private=true;assert.equal(s.dirty,false);s.working.bookmarks.push({Name:'Temporary',URL:'http://temp'});assert.equal(s.dirty,true);s.working.bookmarks.pop();assert.equal(s.dirty,false);s.working=JSON.parse(stringify(s.working));assert.equal(s.dirty,false);delete s.working.bookmarks[0].Private;assert.equal(s.dirty,true);s.discard();assert.equal(s.dirty,false);}finally{JSON.stringify=stringify;}});
test('display entries cannot mutate stored data or dirty tracking',()=>{const s=new Store(fixture()),view=s.viewEntries();assert.equal(Object.isFrozen(view[0].item),true);assert.equal(Reflect.set(view[0].item,'Name','Bypassed'),false);view.pop();assert.equal(s.toJSON().bookmarks[0].Name,'One');assert.equal(s.dirty,false);s.put({Name:'Edited'},'category:stable',s.toJSON().bookmarks[0]?s.working.bookmarks[0]._key:null);assert.equal(s.dirty,true);s.discard();assert.equal(s.dirty,false);assert.equal(s.toJSON().bookmarks[0].Private,true);});
// The place of a new or moved entry among those of its category, where the
// home page shows it: the list of the editor and the table show the file in
// its order, so the entry has to stand there too.
const ordered=()=>new Store({version:'v',categories:[{ID:'a',Name:'A'},{ID:'b',Name:'B'},{ID:'c',Name:'C'}],apps:[{Name:'App 1',URL:'http://1'},{Name:'App 2',URL:'http://2'}],
  bookmarks:[{Name:'a1',URL:'http://a1',Category:'a'},{Name:'b1',URL:'http://b1',Category:'b'},{Name:'a2',URL:'http://a2',Category:'a'},{Name:'b2',URL:'http://b2',Category:'b'},{Name:'loose',URL:'http://l',Category:''}]});
const names=(s,collection='bookmarks')=>s.toJSON()[collection].map(item=>item.Name).join(' ');
const keyOf=(s,name)=>s.entries().find(entry=>entry.item.Name===name).item._key;
test('a new entry stands after the last entry of its place',()=>{
  const s=ordered();s.put({Name:'a3',URL:'http://a3'},'category:a');assert.equal(names(s),'a1 b1 a2 a3 b2 loose');
  s.put({Name:'b3',URL:'http://b3'},'category:b','', 'end');assert.equal(names(s),'a1 b1 a2 a3 b2 b3 loose');
  s.put({Name:'loose2',URL:'http://l2'},'category:');assert.equal(names(s),'a1 b1 a2 a3 b2 b3 loose loose2');
  // The first entry of a place has no other to stand by, and ends the list.
  s.put({Name:'c1',URL:'http://c1'},'category:c','', 'start');assert.equal(names(s),'a1 b1 a2 a3 b2 b3 loose loose2 c1');
  s.put({Name:'App 3',URL:'http://3'},'apps');assert.equal(names(s,'apps'),'App 1 App 2 App 3');
  assert.equal(s.toJSON().bookmarks.find(item=>item.Name==='a3').Category,'a');
});
test('a position puts an entry at the start of its place or after another entry of it',()=>{
  const s=ordered();s.put({Name:'b0',URL:'http://b0'},'category:b',null,'start');assert.equal(names(s),'a1 b0 b1 a2 b2 loose');
  s.put({Name:'a1.5',URL:'http://x'},'category:a',null,'after:'+keyOf(s,'a1'));assert.equal(names(s),'a1 a1.5 b0 b1 a2 b2 loose');
  s.put({Name:'App 0',URL:'http://0'},'apps',null,'start');s.put({Name:'App 1.5',URL:'http://15'},'apps',null,'after:'+keyOf(s,'App 1'));assert.equal(names(s,'apps'),'App 0 App 1 App 1.5 App 2');
  // An entry of another place, or one that is gone, is no position: the end of the place is.
  s.put({Name:'a9',URL:'http://a9'},'category:a',null,'after:'+keyOf(s,'b1'));s.put({Name:'b9',URL:'http://b9'},'category:b',null,'after:gone');
  assert.equal(names(s),'a1 a1.5 b0 b1 a2 a9 b2 b9 loose');
  s.put({Name:'a10',URL:'http://a10'},'category:a',null,'after:'+keyOf(s,'App 1'));assert.equal(names(s),'a1 a1.5 b0 b1 a2 a9 a10 b2 b9 loose');
});
test('an edited entry keeps its position unless another is given, and a moved one joins its new place',()=>{
  const s=ordered(),a1=keyOf(s,'a1'),b2=keyOf(s,'b2');
  s.put({Name:'a1 edited'},'category:a',a1);assert.equal(names(s),'a1 edited b1 a2 b2 loose');
  s.put({Name:'a1 kept'},'category:a',a1,'keep');assert.equal(names(s),'a1 kept b1 a2 b2 loose');
  s.put({Name:'a1'},'category:a',a1,'end');assert.equal(names(s),'b1 a2 a1 b2 loose');assert.equal(keyOf(s,'a1'),a1);
  s.put({},'category:a',a1,'start');assert.equal(names(s),'b1 a1 a2 b2 loose');
  s.put({},'category:a',b2);assert.equal(names(s),'b1 a1 a2 b2 loose');assert.equal(s.find(b2).location,'category:a');
  s.put({},'category:b',b2,'start');assert.equal(names(s),'b2 b1 a1 a2 loose');
  s.put({},'apps',b2,'after:'+keyOf(s,'App 1'));assert.equal(names(s,'apps'),'App 1 b2 App 2');assert.equal(names(s),'b1 a1 a2 loose');
  // The others of a place, without the entry itself, in their order.
  assert.deepEqual(s.siblings('category:a',keyOf(s,'a1')).map(other=>other.name),['a2']);assert.deepEqual(s.siblings('apps').map(other=>other.name),['App 1','b2','App 2']);
  assert.deepEqual(s.siblings('category:c'),[]);assert.equal(s.dirty,true);
});
