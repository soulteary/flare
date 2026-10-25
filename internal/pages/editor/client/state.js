/* Shared editor state. Display filters never change persisted data. */
(function (root) {
  'use strict';
  let sequence = 0;
  const clone = value => JSON.parse(JSON.stringify(value));
  const key = () => 'entry-' + (++sequence);
  const id = () => 'category-' + (globalThis.crypto && globalThis.crypto.randomUUID ? globalThis.crypto.randomUUID() : Date.now().toString(36) + '-' + (++sequence));
  function normalize(doc) {
    return {version: doc.version || '', categories: clone(doc.categories || []), apps: (doc.apps || []).map(item => ({...clone(item), _key: item._key || key()})), bookmarks: (doc.bookmarks || []).map(item => ({...clone(item), _key: item._key || key()}))};
  }
  function payload(doc) {
    const strip = items => items.map(({_key, ...item}) => item);
    return {version: doc.version, categories: clone(doc.categories), apps: strip(doc.apps), bookmarks: strip(doc.bookmarks)};
  }
  function canonical(value) {
    if (Array.isArray(value)) return value.map(canonical);
    if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map(k => [k, canonical(value[k])]));
    return value;
  }
  class Store {
    constructor(doc) { this.accept(doc); }
    accept(doc) { this.working = normalize(doc); this.saved = clone(this.working); }
    get dirty() { const a=payload(this.working),b=payload(this.saved); delete a.version;delete b.version;return JSON.stringify(canonical(a))!==JSON.stringify(canonical(b)); }
    discard() { this.working = clone(this.saved); }
    toJSON() { return payload(this.working); }
    categoryID() { return id(); }
    entries() { return [...this.working.apps.map(item => ({item, location:'apps'})), ...this.working.bookmarks.map(item => ({item,location:item.Category}))]; }
    find(entryKey) { return this.entries().find(({item}) => item._key===entryKey); }
    put(item, location, entryKey) {
      const current=entryKey && this.find(entryKey);
      if (entryKey && !current) throw Error('条目不存在');
      const next={...(current ? current.item : {}), ...item, _key:entryKey || key(), Category:location==='apps' ? '' : location};
      if (current && current.location===location) { Object.assign(current.item,next); return next._key; }
      if (current) this.remove(entryKey);
      (location==='apps' ? this.working.apps : this.working.bookmarks).push(next);
      return next._key;
    }
    remove(entryKey) {
      for(const collection of ['apps','bookmarks']) {const index=this.working[collection].findIndex(item=>item._key===entryKey);if(index>=0){return {collection,index,item:this.working[collection].splice(index,1)[0]};}}
    }
    restore(removed) { if(removed)this.working[removed.collection].splice(removed.index,0,removed.item); }
  }
  const api={Store,normalize,payload};
  if (typeof module==='object' && module.exports) module.exports=api; else root.FlareEditorState=api;
})(typeof window==='undefined' ? globalThis : window);
