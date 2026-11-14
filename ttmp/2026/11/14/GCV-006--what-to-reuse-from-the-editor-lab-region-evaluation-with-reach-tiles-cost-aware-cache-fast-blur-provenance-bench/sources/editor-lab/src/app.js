// ---------------------------------------------------------------------------------------------
// app.js: the app object, settings, the display renderer (coarse first, then tiles in slices),
// the header, theme, toasts, lab: links and keyboard shortcuts.
// ---------------------------------------------------------------------------------------------
'use strict';

const app = {
  ws: null, scene: null,
  settings: Object.assign({ tile: 128, budgetMB: 96, evict: 'lru', preview: 'auto', overlays: { grid: true, dims: true, tiles: false, mask: false, furniture: true } }, store.get('settings', {})),
  tool: store.get('tool', 'brush'),
  brush: Object.assign({ size: 40, hardness: 0.6, flow: 0.8, color: '#f2c27c' }, store.get('brush', {})),
  bg: '#000000', selMode: 'new', maskOp: 'add', selFeather: 0, wand: { tol: 32, contiguous: true, sample: 'composite' }, marquee: 'rect',
  saveSettings() { store.set('settings', this.settings); store.set('brush', this.brush); store.set('tool', this.tool); },
  listeners: {},
  on(ev, fn) { (this.listeners[ev] = this.listeners[ev] || []).push(fn); },
  emit(ev, d) { for (const fn of this.listeners[ev] || []) fn(d); },
};

// ---- loading documents ----------------------------------------------------------------------------
Workspace.prototype.load = function (doc) {
  this.doc = doc; this.engine.doc = doc; this.engine.cache.clear(); this.engine.maskCache.clear(); this.engine.layerMs.clear(); this.engine.invalidate();
  this.hist = { stack: [], ptr: 0 }; this.active = doc.layers.length ? doc.layers[doc.layers.length - 1].id : null; this.target = 'pixels';
  this.emit('load'); this.changed('load'); this.emit('hist');
};
function sceneDoc(sc) { return { code: sc.code, title: sc.title, about: sc.about, w: sc.w, h: sc.h, sel: null, layers: sc.build(sc.w, sc.h) }; }
function loadScene(code) {
  const sc = SCENES.find(s => s.code === code) || SCENES[0];
  const doc = sceneDoc(sc);
  if (!app.ws) initWorkspace(doc); else app.ws.load(doc);
  app.scene = sc.code; store.set('scene', sc.code);
  app.emit('scene');
  const p = app.ws.doc.layers.find(l => l.kind === 'paint'); if (p) app.ws.setActive(p.id);
  Stage.fit && Stage.fit();
}
function loadImageDoc(img, name) {
  const k = Math.min(1, 2048 / Math.max(img.width, img.height)), w = Math.round(img.width * k), h = Math.round(img.height * k);
  const photo = makeLayer('image', { name: name || 'Image' }); photo.raster = canvasRaster(w, h, g => g.drawImage(img, 0, 0, w, h));
  const paint = makeLayer('paint', { name: 'Paint' }); paint.raster = new Raster(w, h, 4);
  app.ws.load({ code: 'D-00', title: name || 'Opened image', about: 'an image you opened', w, h, sel: null, layers: [photo, paint] });
  app.scene = 'D-00'; app.emit('scene'); Stage.fit && Stage.fit();
}
function initWorkspace(doc) {
  const ws = app.ws = new Workspace(doc, { tile: app.settings.tile, budgetMB: app.settings.budgetMB, evict: app.settings.evict });
  ws.on('doc', e => { Render.request(e.interactive); app.emit('doc', e); });
  ws.on('hist', () => app.emit('hist'));
  ws.on('active', l => app.emit('active', l));
  ws.on('open', g => app.emit('open', g));
  ws.on('sel', () => { Stage.selOutline = null; app.emit('sel'); });
  ws.on('load', () => { Render.resize(); app.emit('load'); });
}
async function readImageBlob(blob) {
  const url = URL.createObjectURL(blob);
  try { const img = new Image(); img.src = url; await img.decode(); return img; } finally { setTimeout(() => URL.revokeObjectURL(url), 1000); }
}

// ---- the display renderer ----------------------------------------------------------------------
const Render = {
  disp: document.createElement('canvas'), small: document.createElement('canvas'),
  pending: false, interactiveUntil: 0, needFull: true, queue: [], reqT: 0, sliceMs: 8,
  coarseL: 2, estFullMs: 30, shownL: 0, frame: 0, recent: new Map(), stats: { fullMs: 0, coarseMs: 0, tiles: 0, computed: 0, hitRate: 0, coarseL: 0 }, passComputed: 0, passMs: 0,
  resize() { const d = app.ws.doc; this.disp.width = d.w; this.disp.height = d.h; this.dctx = this.disp.getContext('2d'); this.needFull = true; this.request(false); },
  request(interactive) {
    if (!this.pending && !this.queue.length) this.reqT = performance.now();
    this.pending = true; this.needFull = true;
    if (interactive) this.interactiveUntil = performance.now() + 220;
  },
  tick(now) {
    this.frame++;
    const ws = app.ws, eng = ws.engine, interactive = now < this.interactiveUntil, pol = app.settings.preview;
    if (this.pending) {
      this.pending = false;
      const coarse = pol !== 'off' && (pol !== 'auto' || this.estFullMs > (interactive ? 10 : 40));
      if (coarse) {
        const L = pol === 'auto' ? this.coarseL : +pol, D = eng.dims(L);
        const t0 = performance.now(); const buf = eng.rect(eng.topIndex(), L, 0, 0, D.w, D.h); const dt = performance.now() - t0;
        this.small.width = D.w; this.small.height = D.h; this.small.getContext('2d').putImageData(toImageData(buf, D.w, D.h), 0, 0);
        this.dctx.clearRect(0, 0, this.disp.width, this.disp.height); this.dctx.imageSmoothingEnabled = true; this.dctx.drawImage(this.small, 0, 0, this.disp.width, this.disp.height);
        this.shownL = L; this.stats.coarseMs = performance.now() - this.reqT; this.stats.coarseL = L;
        if (pol === 'auto') { if (dt > 12 && this.coarseL < 3) this.coarseL++; else if (dt < 2.5 && this.coarseL > 1) this.coarseL--; }
        this.queue = []; Stage.dirty = true;
      }
    }
    if (this.needFull && !interactive) { this.needFull = false; this.queue = this.order(); this.passComputed = 0; this.passMs = 0; this.passHits = eng.cache.hits; this.passMiss = eng.cache.misses; }
    if (this.queue.length && !(interactive && this.shownL > 0)) {
      const t0 = performance.now(), all = pol === 'off' || this.estFullMs < 12;
      while (this.queue.length && (all || performance.now() - t0 < this.sliceMs)) {
        const [tx, ty] = this.queue.shift(), c0 = eng.computed, s0 = performance.now();
        const t = eng.tile(eng.topIndex(), 0, tx, ty);
        this.passMs += performance.now() - s0;
        this.dctx.putImageData(toImageData(t.buf, t.w, t.h), tx * eng.TS, ty * eng.TS);
        if (eng.computed > c0) { this.passComputed++; this.recent.set(tx + ',' + ty, this.frame); }
      }
      Stage.dirty = true;
      if (!this.queue.length) {
        this.shownL = 0;
        const g = eng.grid(0), hits = eng.cache.hits - this.passHits, miss = eng.cache.misses - this.passMiss;
        Object.assign(this.stats, { fullMs: performance.now() - this.reqT, tiles: g.nx * g.ny, computed: this.passComputed, hitRate: hits + miss ? hits / (hits + miss) : 1, passMs: this.passMs });
        this.estFullMs = this.estFullMs * 0.4 + this.passMs * 0.6;
        this.layerMs = new Map(eng.layerMs); eng.layerMs.clear();
        app.emit('frame', this.stats);
      }
    }
    if (Stage.dirty) Stage.draw();
    requestAnimationFrame(t => this.tick(t));
  },
  // Visible tiles first, nearest the middle of the view first.
  order() {
    const eng = app.ws.engine, g = eng.grid(0), T = eng.TS, v = Stage.viewRect ? Stage.viewRect() : { x: 0, y: 0, w: app.ws.doc.w, h: app.ws.doc.h };
    const cx = v.x + v.w / 2, cy = v.y + v.h / 2, out = [];
    for (let ty = 0; ty < g.ny; ty++) for (let tx = 0; tx < g.nx; tx++) {
      const x0 = tx * T, y0 = ty * T, vis = x0 < v.x + v.w && x0 + T > v.x && y0 < v.y + v.h && y0 + T > v.y;
      out.push({ tx, ty, k: (vis ? 0 : 1e9) + Math.hypot(x0 + T / 2 - cx, y0 + T / 2 - cy) });
    }
    return out.sort((a, b) => a.k - b.k).map(o => [o.tx, o.ty]);
  },
};
function applyEngineSettings() {
  const eng = app.ws.engine;
  if (eng.TS !== app.settings.tile) { eng.TS = app.settings.tile; eng.cache.clear(); }
  eng.cache.policy = app.settings.evict; eng.cache.setBudget(app.settings.budgetMB); eng.invalidate();
  app.saveSettings(); Render.request(false); app.emit('settings');
}

// ---- toasts (the workbench's) and the stage toast ------------------------------------------------
function toast(text, bad) {
  let box = $('.toasts'); if (!box) { box = h('div', { class: 'toasts', 'aria-live': 'polite' }); document.body.append(box); }
  const t = h('div', { class: 'toast' + (bad ? ' bad' : '') }, text); box.append(t); setTimeout(() => t.remove(), bad ? 4200 : 2400);
}

// ---- theme: system → light → dark -----------------------------------------------------------------
const THEME_ICONS = {
  system: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="5.5"/><path d="M8 2.5v11A5.5 5.5 0 0 0 8 2.5z" fill="currentColor"/></svg>',
  light: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="3"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3 3l1.4 1.4M11.6 11.6L13 13M3 13l1.4-1.4M11.6 4.4L13 3"/></svg>',
  dark: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M13 9.5A5.5 5.5 0 1 1 6.5 3 4.5 4.5 0 0 0 13 9.5z"/></svg>',
};
function setTheme(t) {
  app.theme = t; store.set('theme', t);
  if (t === 'system') document.documentElement.removeAttribute('data-theme'); else document.documentElement.setAttribute('data-theme', t);
  const b = $('#themebtn'); if (b) { b.innerHTML = THEME_ICONS[t]; b.title = 'Theme: ' + t + ' (click for ' + { system: 'light', light: 'dark', dark: 'system' }[t] + ')'; }
  setTimeout(() => { Stage.readColors(); Stage.dirty = true; app.emit('theme'); }, 30);
}

// ---- lab: links, from the book, the agent's answers and the presets ------------------------------
function labLink(href) {
  const u = new URL(href.replace(/^lab:/, 'lab://x/')), a = u.pathname.replace(/^\//, ''), q = Object.fromEntries(u.searchParams);
  try {
    if (a === 'scene') { loadScene(q.id); Dock.focus('Stage'); }
    else if (a === 'view') Dock.focus(q.name);
    else if (a === 'tool') { setTool(q.name); Dock.focus('Stage'); }
    else if (a === 'overlay') { app.settings.overlays[q.name] = q.on !== 'false'; app.saveSettings(); Stage.dirty = true; app.emit('settings'); Dock.focus('Stage'); }
    else if (a === 'add') { const { kind, ...rest } = q; const params = {}; for (const [k, v] of Object.entries(rest)) params[k] = isNaN(+v) ? v : +v; app.ws.addLayer(kind, { params }); Dock.focus('Params'); }
    else if (a === 'blend') { const l = app.ws.activeLayer(); if (l) app.ws.setLayer(l.id, { blend: q.mode }); Dock.focus('Layers'); }
    else if (a === 'select') { app.ws.setActive(q.layer); Dock.focus('Params'); }
    else if (a === 'group') { const ids = (q.layers || '').split(',').map(x => app.ws.layer(x.trim())).filter(Boolean).map(l => l.id); if (!ids.length) throw new Error('none of those layers are here: ' + q.layers); app.ws.group(ids, { name: q.name, blend: q.blend }); Dock.focus('Layers'); }
    else if (a === 'preview') { app.settings.preview = q.level; applyEngineSettings(); }
    else if (a === 'tile') { app.settings.tile = +q.size; applyEngineSettings(); }
    else if (a === 'budget') { app.settings.budgetMB = +q.mb; applyEngineSettings(); }
    else if (a === 'evict') { app.settings.evict = q.policy === 'cost' ? 'cost' : 'lru'; applyEngineSettings(); }
    else if (a === 'bench') { Dock.focus('Bench'); Bench.show(q); }
    else if (a === 'preset') { Dock.focus('Presets'); Presets.select(q.id); }
    else if (a === 'book') { Dock.focus('Book'); Book.open(q.ch, q.at); }
    else if (a === 'ask') { Dock.focus('Agent'); Agent.prefill(q.q || ''); }
    else if (a === 'history') Dock.focus('History');
    else if (a === 'code') { const ex = q.ex || q.run; if (!Code.get(ex)) throw new Error(`no example '${ex}'`); Code.open(ex); if (q.run) Code.run(); }
    else toast('That link does nothing here: ' + href, true);
  } catch (e) { toast(e.message, true); }
}
document.addEventListener('click', e => { const a = e.target.closest('a[data-lab]'); if (a) { e.preventDefault(); labLink(a.dataset.lab); } });
document.addEventListener('click', e => { const b = e.target.closest('button[data-run]'); if (!b) return; e.preventDefault(); try { Code.openScratch(b.dataset.runName, decodeURIComponent(b.dataset.run)); Code.run(); } catch (er) { toast(er.message, true); } });

// ---- tools and keyboard ------------------------------------------------------------------------
const TOOLS = [
  { id: 'move', label: 'Move', key: 'V', hint: 'drag to move the layer · arrows nudge 1 px, Shift 10 px' },
  { id: 'hand', label: 'Hand', key: 'H', hint: 'drag to pan · wheel to zoom · double-click to fit' },
  { id: 'marquee', label: 'Select', key: 'M', hint: 'drag a rectangle or ellipse · Shift adds · Alt subtracts' },
  { id: 'lasso', label: 'Lasso', key: 'L', hint: 'draw round the area · release to close · Shift adds · Alt subtracts' },
  { id: 'wand', label: 'Wand', key: 'W', hint: 'click a colour · tolerance sets how near · Shift adds · Alt subtracts' },
  { id: 'brush', label: 'Brush', key: 'B', hint: '[ ] size · on a mask it paints into a brush component, feathered by its edge feather' },
  { id: 'eraser', label: 'Erase', key: 'E', hint: '[ ] size · on a mask it erases the brush component, or adds one that subtracts' },
  { id: 'gradient', label: 'Linear', key: 'G', hint: 'drag from start to end: a linear mask component on an adjustment or its mask, else a gradient layer · Alt subtracts' },
  { id: 'radial', label: 'Radial', key: 'U', hint: 'drag from the centre out: a radial mask component on an adjustment or its mask, else a radial gradient layer · Shift for a circle · Alt subtracts' },
  { id: 'picker', label: 'Pick', key: 'I', hint: 'click to take the colour under the pointer' },
];
function setTool(id) { if (!TOOLS.find(t => t.id === id)) return; app.tool = id; app.saveSettings(); app.emit('tool'); Stage.dirty = true; }
document.addEventListener('keydown', e => {
  const tag = (e.target.tagName || '').toLowerCase(), typing = tag === 'input' || tag === 'textarea' || tag === 'select' || e.target.isContentEditable;
  const mod = e.metaKey || e.ctrlKey;
  if (mod && e.key.toLowerCase() === 'z' && !typing) { e.preventDefault(); const l = e.shiftKey ? app.ws.redo() : app.ws.undo(); if (l) Stage.stageToast((e.shiftKey ? 'Redo ' : 'Undo ') + l); return; }
  if (mod && e.key.toLowerCase() === 'y' && !typing) { e.preventDefault(); app.ws.redo(); return; }
  if (typing) return;
  if (mod && e.key.toLowerCase() === 'a') { e.preventDefault(); app.ws.selectAll(); return; }
  if (mod && e.key.toLowerCase() === 'd') { e.preventDefault(); app.ws.selectNone(); return; }
  if (mod && e.shiftKey && e.key.toLowerCase() === 'i') { e.preventDefault(); app.ws.selectInvert(); return; }
  if (mod) return;
  if (e.key === 'Delete' || e.key === 'Backspace') { if (app.ws.doc.sel) { e.preventDefault(); try { app.ws.clearSelection(); } catch (er) { toast(er.message, true); } } return; }
  if (e.key === '[' || e.key === ']') { app.brush.size = clamp(Math.round(app.brush.size * (e.key === '[' ? 0.85 : 1.18)), 1, 600); app.saveSettings(); app.emit('tool'); Stage.dirty = true; return; }
  if (e.key === 'x' || e.key === 'X') { [app.brush.color, app.bg] = [app.bg, app.brush.color]; app.emit('tool'); return; }
  if (e.key === 'd' || e.key === 'D') { app.brush.color = '#ffffff'; app.bg = '#000000'; app.emit('tool'); return; }
  const t = TOOLS.find(t => t.key === e.key.toUpperCase()); if (t) { setTool(t.id); return; }
  if (e.key === ' ' && Stage.hover) { e.preventDefault(); Stage.spacePan = true; Stage.setCursor(); }
  if (Stage.hover && app.tool === 'move' && e.key.startsWith('Arrow')) {
    e.preventDefault(); const l = app.ws.activeLayer(); if (!l || !l.raster) return; const d = e.shiftKey ? 10 : 1;
    const dx = { ArrowLeft: -d, ArrowRight: d }[e.key] || 0, dy = { ArrowUp: -d, ArrowDown: d }[e.key] || 0;
    app.ws.setLayer(l.id, { ox: l.ox + dx, oy: l.oy + dy }, 'Nudge ' + l.name);
  }
});
document.addEventListener('keyup', e => { if (e.key === ' ') { Stage.spacePan = false; Stage.setCursor(); } });

// ---- paste and drop images ----------------------------------------------------------------------
document.addEventListener('paste', async e => {
  const items = [...(e.clipboardData ? e.clipboardData.items : [])].filter(i => i.type.startsWith('image/'));
  if (!items.length) return;
  const inComposer = e.target.closest && e.target.closest('.composer');
  e.preventDefault();
  for (const it of items) {
    const blob = it.getAsFile(); if (!blob) continue;
    if (inComposer) { Agent.attach(blob); continue; }
    try { const img = await readImageBlob(blob); app.ws.imageLayer(img, 'Pasted image'); Stage.stageToast('Pasted image', `${img.width} × ${img.height} px · new layer`); }
    catch (er) { toast('Could not read that image: ' + er.message, true); }
  }
});

// ---- header -------------------------------------------------------------------------------------
function buildHeader() {
  const scenebtn = h('button', { class: 'projbtn', 'aria-haspopup': 'menu', onclick: e => openMenu(e.currentTarget, [
    { group: 'Scenes' },
    ...SCENES.map(s => ({ label: s.title, small: s.code + ' · ' + s.about, on: app.scene === s.code, run: () => { loadScene(s.code); toast(`Loaded ${s.code} ${s.title}`); } })),
    { sep: true },
    { label: 'Open an image…', small: 'a file from your computer, up to 2048 px across', run: () => openFile() },
    { sep: true },
    { label: 'Export the picture as PNG', small: 'full resolution, with transparency', run: () => Exports.picture('png') },
    { label: 'Export the picture as JPEG', small: 'full resolution, quality 92', run: () => Exports.picture('jpg') },
    { note: 'Paste an image anywhere to add it as a layer; paste into the agent to show it to Claude.' },
  ], { cls: 'presets' }) }, h('span', { class: 'pcode' }), h('span', { class: 'ptitle' }), h('span', { class: 'caret' }, '▾'));
  const undo = h('button', { class: 'btn sm', title: 'Undo (⌘Z)', onclick: () => { const l = app.ws.undo(); if (l) Stage.stageToast('Undo ' + l); } }, 'Undo', h('kbd', null, '⌘Z'));
  const redo = h('button', { class: 'btn sm', title: 'Redo (⇧⌘Z)', onclick: () => { const l = app.ws.redo(); if (l) Stage.stageToast('Redo ' + l); } }, 'Redo');
  const rpill = h('button', { class: 'pill', title: 'Last full render: from the change to the last tile on screen. Opens the bench.', onclick: () => Dock.focus('Bench') }, h('b', null, '…'), h('span', null, 'render'));
  const cpill = h('button', { class: 'pill', title: 'Tile cache: hit rate of the last full render, and memory held', onclick: () => Dock.focus('Bench') }, h('b', null, '…'), h('span', null, 'cache'));
  const bpill = h('button', { class: 'pill busy', hidden: true, onclick: () => Dock.focus('Bench') }, h('b', null, 'bench'), h('span', null, 'running'));
  const lay = h('button', { class: 'btn laymenu', 'aria-haspopup': 'menu', onclick: e => openMenu(e.currentTarget, [{ group: 'Layouts' }, ...LAYOUTS.map(L => ({ label: L.name, small: L.about, run: () => Dock.setLayout(L.id) }))], { right: true, cls: 'layouts' }) }, '▦ Layout');
  const theme = h('button', { class: 'ibtn', id: 'themebtn', onclick: () => setTheme({ system: 'light', light: 'dark', dark: 'system' }[app.theme]) });
  const bar = h('header', { class: 'bar' }, h('div', { class: 'brand' }, 'Editor lab ', h('i', null, 'workbench')), h('div', { class: 'proj' }, scenebtn), h('div', { class: 'hist' }, undo, redo), h('span', { class: 'sp' }), bpill, rpill, cpill, lay, theme);
  const upd = () => {
    const d = app.ws.doc; scenebtn.querySelector('.pcode').textContent = d.code; scenebtn.querySelector('.ptitle').textContent = d.title;
    undo.disabled = !app.ws.hist.ptr; redo.disabled = app.ws.hist.ptr >= app.ws.hist.stack.length;
  };
  const updStats = () => {
    const s = Render.stats, eng = app.ws.engine, busy = Render.queue.length > 0 || Render.shownL > 0;
    rpill.querySelector('b').textContent = busy ? (Render.shownL ? 'preview ' + ['', '½', '¼', '⅛'][Render.shownL] : 'refining') : fmtMs(s.fullMs) + ' ms';
    rpill.querySelector('span').textContent = busy ? `${Render.queue.length} tiles to go` : `full · ${s.tiles} tiles · ${s.computed} computed`;
    rpill.classList.toggle('busy', busy);
    cpill.querySelector('b').textContent = Math.round(s.hitRate * 100) + ' %';
    cpill.querySelector('span').textContent = `cache hit · ${fmtMB(eng.cache.bytes)} of ${app.settings.budgetMB} MB`;
  };
  app.on('hist', upd); app.on('scene', upd); app.on('frame', updStats);
  setInterval(updStats, 400);
  app.on('bench', s => { bpill.hidden = !s.running; if (s.running) bpill.querySelector('span').textContent = s.text || 'running'; });
  upd(); return bar;
}
function openFile() {
  const inp = h('input', { type: 'file', accept: 'image/*', style: { display: 'none' } });
  inp.onchange = async () => { const f = inp.files[0]; inp.remove(); if (!f) return; try { const img = await readImageBlob(f); loadImageDoc(img, f.name.replace(/\.[^.]+$/, '')); toast(`Opened ${f.name}`); } catch (e) { toast('Could not open that file: ' + e.message, true); } };
  document.body.append(inp); inp.click();
}

// ---- boot --------------------------------------------------------------------------------------
function boot() {
  setTheme(store.get('theme', 'system'));
  loadScene(store.get('scene', 'D-01'));
  const root = $('#app'); root.textContent = '';
  root.append(buildHeader());
  const dock = h('main', { class: 'dock' }); root.append(dock);
  Dock.init(dock);
  Render.resize();
  requestAnimationFrame(t => Render.tick(t));
  app.on('doc', () => Dock.refreshBadges()); app.on('hist', () => Dock.refreshBadges());
  Presets.load(); Code.load(); Agent.init();
}
