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
    doc = rawTargets.get(doc) || doc;
    const strip = items => items.map(({_key, ...item}) => item);
    return {version: doc.version, categories: clone(doc.categories), apps: strip(doc.apps), bookmarks: strip(doc.bookmarks)};
  }
  function canonical(value) {
    if (Array.isArray(value)) return value.map(canonical);
    if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map(k => [k, canonical(value[k])]));
    return value;
  }
  // Track mutations without serializing the whole document on display-only work.
  // Weak maps keep replaced snapshots collectible and avoid wrapping proxies again.
  const rawTargets = new WeakMap();
  function observe(source, changed) {
    const proxies = new WeakMap();
    function wrap(value) {
      if (!value || typeof value !== 'object') return value;
      value = rawTargets.get(value) || value;
      if (proxies.has(value)) return proxies.get(value);
      const proxy = new Proxy(value, {
        get(target, key) { return wrap(Reflect.get(target, key)); },
        set(target, key, value) {
          value = value && typeof value === 'object' ? rawTargets.get(value) || value : value;
          const differs = !Object.hasOwn(target, key) || !Object.is(target[key], value);
          const result = Reflect.set(target, key, value);
          if (result && differs) changed();
          return result;
        },
        deleteProperty(target, key) {
          const existed = Object.hasOwn(target, key), result = Reflect.deleteProperty(target, key);
          if (result && existed) changed();
          return result;
        },
        defineProperty(target, key, descriptor) {
          const result = Reflect.defineProperty(target, key, descriptor);
          if (result) changed();
          return result;
        }
      });
      proxies.set(value, proxy); rawTargets.set(proxy, value); return proxy;
    }
    return wrap(source);
  }
  class Store {
    constructor(doc) { this.accept(doc); }
    set working(doc) { this._revision = (this._revision || 0) + 1; this._working = observe(doc, () => { this._revision++; }); }
    get working() { return this._working; }
    accept(doc) { this.working = normalize(doc); this.saved=clone(rawTargets.get(this.working));this._dirtyRevision=this._revision;this._dirty=false; }
    get dirty() {
      if(this._dirtyRevision!==this._revision){const current=payload(this.working),saved=payload(this.saved);delete current.version;delete saved.version;this._dirty=JSON.stringify(canonical(current))!==JSON.stringify(canonical(saved));this._dirtyRevision=this._revision;}
      return this._dirty;
    }
    discard() { this.working = clone(this.saved); }
    toJSON() { return payload(this.working); }
    categoryID() { return id(); }
    entries() { return [...this.working.apps.map(item => ({item, location:'apps'})), ...this.working.bookmarks.map(item => ({item,location:'category:'+item.Category}))]; }
    // Read-only display views avoid creating mutation-observing proxies for every
    // row merely to paint/filter a large list. Mutations must use working/put/remove.
    viewEntries() {
      const doc=rawTargets.get(this.working);return [...doc.apps.map(item=>({item:Object.freeze({...item}),location:'apps'})),...doc.bookmarks.map(item=>({item:Object.freeze({...item}),location:'category:'+item.Category}))];
    }
    find(entryKey) {
      const doc=rawTargets.get(this.working);
      for(const collection of ['apps','bookmarks']){const index=doc[collection].findIndex(item=>item._key===entryKey);if(index>=0){const item=this.working[collection][index];return{item,location:collection==='apps'?'apps':'category:'+item.Category};}}
    }
    put(item, location, entryKey, position) {
      const current=entryKey && this.find(entryKey);
      if (entryKey && !current) throw Object.assign(Error('entry_not_found'),{code:'entry_not_found'});
      const next={...(current ? current.item : {}), ...item, _key:entryKey || key(), Category:location==='apps' ? '' : location.slice('category:'.length)};
      if (current && current.location===location && (!position || position==='keep')) { Object.assign(current.item,next); return next._key; }
      if (current) this.remove(entryKey);
      const collection=location==='apps' ? 'apps' : 'bookmarks';
      this.working[collection].splice(this.placeAt(collection,next.Category,position),0,next);
      return next._key;
    }
    // placeAt is where an entry goes among those of its place: after one of
    // them, before the first, or, without a position, after the last. The
    // applications are one place and a category of the bookmarks is one. An
    // entry that is the first of its place goes to the end of the list.
    placeAt(collection, category, position) {
      const list=rawTargets.get(this.working)[collection], same=item=>collection==='apps' || item.Category===category;
      if (typeof position==='string' && position.startsWith('after:')) {
        const index=list.findIndex(item=>item._key===position.slice('after:'.length) && same(item));
        if (index>=0) return index+1;
      }
      if (position==='start') { const index=list.findIndex(same); if (index>=0) return index; }
      for (let index=list.length-1; index>=0; index--) if (same(list[index])) return index+1;
      return list.length;
    }
    // siblings are the other entries of a place, in their order.
    siblings(location, entryKey) {
      const doc=rawTargets.get(this.working), category=location.slice('category:'.length);
      return (location==='apps' ? doc.apps : doc.bookmarks.filter(item=>item.Category===category)).filter(item=>item._key!==entryKey).map(item=>({key:item._key,name:item.Name}));
    }
    remove(entryKey) {
      const doc=rawTargets.get(this.working);for(const collection of ['apps','bookmarks']) {const index=doc[collection].findIndex(item=>item._key===entryKey);if(index>=0){const item=doc[collection].splice(index,1)[0];this._revision++;return {collection,index,item};}}
    }
    restore(removed) { if(removed){rawTargets.get(this.working)[removed.collection].splice(removed.index,0,removed.item);this._revision++;} }
  }
  const maxDraftBytes=64*1024*1024;
  function draftBlob(doc) {const blob=new Blob([JSON.stringify(doc)],{type:'application/json'});if(blob.size>maxDraftBytes)throw Object.assign(Error('draft_too_large'),{code:'draft_too_large'});return blob;}
  const api={Store,normalize,payload,maxDraftBytes,draftBlob};
  if (typeof module==='object' && module.exports) module.exports=api; else root.FlareEditorState=api;
})(typeof window==='undefined' ? globalThis : window);
