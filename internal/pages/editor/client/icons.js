/* Built-in silhouettes follow the editor palette; external images keep their colors. */
(function () {
 'use strict';
 window.FlareEditorIcons = {
  create(name, onerror) {
   const icon=document.createElement('span');icon.className='editor-builtin-icon';icon.setAttribute('aria-hidden','true');
   const image=document.createElement('img');image.alt='';image.hidden=true;
   // Reuse the downloaded SVG for decoding and masking. The endpoint is no-store,
   // so using its URL twice would make a second authenticated HTTP request.
   image.onload=()=>{icon.style.setProperty('--editor-icon-url',`url("${image.src}")`);icon.dataset.loaded='true';};
   image.onerror=()=>onerror(icon);
   icon.append(image);
   fetch('/editor/icon?name='+encodeURIComponent(name),{cache:'no-store'})
    .then(response=>{
     if(!response.ok || response.redirected || !(response.headers.get('content-type')||'').includes('image/svg+xml'))throw Error('Invalid icon response');
     return response.text();
    })
    .then(svg=>{image.src='data:image/svg+xml;charset=utf-8,'+encodeURIComponent(svg);})
    .catch(()=>onerror(icon));
   return icon;
  }
 };
})();
