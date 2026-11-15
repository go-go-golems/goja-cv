// ---------------------------------------------------------------------------------------------
// views/panels.js: Layers, Params and History.
// ---------------------------------------------------------------------------------------------
'use strict';

function debounce(fn, ms) { let t; return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); }; }
function thumbCanvas(w, h) { const c = document.createElement('canvas'), d = devicePixelRatio || 1; c.width = w * d; c.height = h * d; return c; }
function drawRasterThumb(c, r, docW, docH, ox = 0, oy = 0) {
  let L = 0; while (r.w / 2 ** L > c.width * 2) L++;
  const lv = r.level(L), g = c.getContext('2d'), k = Math.min(c.width / docW, c.height / docH);
  const tmp = document.createElement('canvas'); tmp.width = lv.w; tmp.height = lv.h;
  const im = new ImageData(lv.w, lv.h);
  if (lv.ch === 4) im.data.set(lv.data); else for (let i = 0; i < lv.data.length; i++) { im.data[i * 4] = im.data[i * 4 + 1] = im.data[i * 4 + 2] = lv.data[i]; im.data[i * 4 + 3] = 255; }
  tmp.getContext('2d').putImageData(im, 0, 0);
  g.clearRect(0, 0, c.width, c.height); g.imageSmoothingEnabled = true;
  g.drawImage(tmp, (c.width - docW * k) / 2 + ox * k, (c.height - docH * k) / 2 + oy * k, r.w * k, r.h * k);
}

function drawMaskThumb(c, l, ws) {
  let L = 0; while (ws.doc.w / 2 ** L > 120) L++;
  const m = maskImage(ws.engine, l, L), tmp = document.createElement('canvas'); tmp.width = m.w; tmp.height = m.h;
  const im = new ImageData(m.w, m.h); for (let i = 0; i < m.f.length; i++) { const v = m.f[i] * 255; im.data[i * 4] = im.data[i * 4 + 1] = im.data[i * 4 + 2] = v; im.data[i * 4 + 3] = 255; }
  tmp.getContext('2d').putImageData(im, 0, 0);
  const g = c.getContext('2d'), k = Math.min(c.width / m.w, c.height / m.h); g.clearRect(0, 0, c.width, c.height); g.drawImage(tmp, (c.width - m.w * k) / 2, (c.height - m.h * k) / 2, m.w * k, m.h * k);
}
// ---- Provenance: where a layer came from, and the way back to it --------------------------------------
// meta.source kinds: preset { id, name, index } · program { file, fileId, line, preset } · script { file, fileId,
// line } · agent { thread, turn, step } · codeLayer { file }. Each may carry via: the source that applied it.
const Provenance = {
  stamp(kind, o = {}) { return Object.assign({ kind }, o, { at: new Date().toISOString() }); },
  // The line of the nth layer('name', ...) call in a program, 1-based.
  programLine(code, name, nth = 0) {
    const lines = String(code || '').split('\n'); let seen = 0;
    for (let i = 0; i < lines.length; i++) { const m = /^\s*layer\(\s*(['"`])((?:\\.|(?!\1).)*)\1/.exec(lines[i]); if (m && m[2].replace(/\\(.)/g, '$1') === name && seen++ === nth) return i + 1; }
    return null;
  },
  of(l) { return l && l.meta && l.meta.source || null; },
  label(src) {
    if (!src) return '';
    if (src.kind === 'preset') return src.id;
    if (src.kind === 'program' || src.kind === 'script') return src.file + (src.line ? ':' + src.line : '');
    if (src.kind === 'agent') return `agent · step ${(src.step ?? 0) + 1}`;
    if (src.kind === 'codeLayer') return src.file || 'code layer';
    return src.kind;
  },
  title(src) {
    if (!src) return '';
    const when = src.at ? ' · ' + new Date(src.at).toLocaleString() : '';
    if (src.kind === 'preset') return `Made by preset ${src.id} ${src.name || ''}${src.via ? ', applied by ' + this.label(src.via) : ''}${when}. Click to open the preset.`;
    if (src.kind === 'program') return `Made by the program ${src.file}${src.line ? ', line ' + src.line : ''}${src.preset ? ' (preset ' + src.preset + ')' : ''}${when}. Click to open it in the Code view.`;
    if (src.kind === 'script') return `Added by the script ${src.file}${src.line ? ', line ' + src.line : ''}${when}. Click to open it in the Code view.`;
    if (src.kind === 'agent') return `Added by the agent, step ${(src.step ?? 0) + 1} of a reply${when}. Click to show the step.`;
    return src.kind + when;
  },
  // Open what made the layer: the preset, the program at the line of this layer, the script line, the agent's step.
  reveal(l, where) {
    const src = this.of(l); if (!src) return false;
    if (src.kind === 'preset') {
      if (where === 'code') { const t = Code.openPreset(src.id), p = Presets.get(src.id); Code.goto(this.programLine(t.code, p ? (p.layers[src.index] || {}).name || l.name : l.name, this.nthName(p, src.index)) || 1); return true; }
      if (!Presets.get(src.id)) { toast(`Preset ${src.id} is no longer there`, true); return false; }
      Presets.select(src.id); Dock.focus('Presets'); return true;
    }
    if (src.kind === 'program' || src.kind === 'script') {
      const f = src.fileId && Code.get(src.fileId) ? src.fileId : (Code.files.find(x => x.name === src.file) || CODE_EXAMPLES.find(x => x.name === src.file) || Code.scratch.find(x => x.name === src.file) || {}).id;
      if (!f) { toast(`${src.file} is no longer open or saved here`, true); return false; }
      Code.open(f); if (src.line) Code.goto(src.line); return true;
    }
    if (src.kind === 'agent') return Agent.reveal(src);
    if (src.kind === 'codeLayer' && l.kind === 'code') { Code.openLayer(l.id); return true; }
    return false;
  },
  nthName(p, index) { if (!p) return 0; const nm = (p.layers[index] || {}).name; let n = 0; for (let i = 0; i < index; i++) if (p.layers[i].name === nm) n++; return n; },
  // Layers in the document that a source made.
  madeBy(test) { return app.ws.doc.layers.filter(l => { const s = this.of(l); return s && test(s); }); },
};

// ---- Layers ------------------------------------------------------------------------------------
const Layers = {
  create() {
    this.list = h('div', { class: 'scroll layers', role: 'list' });
    this.foot = h('div', { class: 'lfoot' });
    this.count = h('span', { class: 'lbl' });
    const add = h('button', { class: 'btn', 'aria-haspopup': 'menu', onclick: e => openMenu(e.currentTarget, this.addItems(), { cls: 'presets' }) }, '+ Layer ▾');
    const mask = h('button', { class: 'btn', 'aria-haspopup': 'menu', onclick: e => openMenu(e.currentTarget, this.maskItems()) }, 'Mask ▾');
    const dup = h('button', { class: 'btn', onclick: () => app.ws.active && app.ws.duplicateLayer(app.ws.active) }, 'Duplicate');
    const del = h('button', { class: 'btn', onclick: () => app.ws.active && app.ws.removeLayer(app.ws.active) }, 'Delete');
    this.grpBtn = h('button', { class: 'btn', title: 'Group the selected layers (Shift- or ⌘-click rows to select several). On a group: take its layers out.', onclick: () => this.groupOrUngroup() }, 'Group');
    const el = h('div', { class: 'tabbody col' }, h('div', { class: 'toolbar' }, add, mask, this.grpBtn, dup, del, h('span', { class: 'sp' }), this.count), this.list, this.foot);
    this.sel = new Set();
    const re = debounce(() => this.render(), 120);
    for (const ev of ['doc', 'active', 'load', 'hist', 'theme', 'open']) app.on(ev, re);
    app.on('frame', () => this.renderFoot());
    this.render();
    return el;
  },
  // Where + Layer puts a new layer: at the top of the active group when a group is active, else on top.
  addTarget() { const a = app.ws.activeLayer(); return a && KINDS[a.kind].isGroup ? { parent: a.id } : a && a.parent ? { parent: a.parent } : {}; },
  addItems() {
    const groups = {}; for (const [k, K] of Object.entries(KINDS)) (groups[K.group] = groups[K.group] || []).push([k, K]);
    const items = [];
    for (const [g, ks] of Object.entries(groups)) { items.push({ group: g }); for (const [k, K] of ks) items.push({ label: K.isGroup ? 'Empty group' : K.label, small: ADD_ABOUT[k] || '', run: () => { if (k === 'code') { try { Code.newCodeLayer(); } catch (e) { toast(e.message, true); } return; } const l = app.ws.addLayer(k, this.addTarget()); if (k === 'image') openFileAsLayer(l); } }); }
    items.push({ note: 'Adjustments change everything under them. A mask limits where.' });
    return items;
  },
  maskItems() {
    const ws = app.ws, l = ws.activeLayer();
    if (!l) return [{ note: 'No layer selected.' }];
    return [...compMenu(l), { sep: true }, { label: 'Delete mask', danger: true, disabled: !l.mask, run: () => ws.deleteMask(l.id) }];
  },
  groupOrUngroup() {
    const ws = app.ws, a = ws.activeLayer();
    try {
      const ids = this.sel.size ? [...this.sel] : a ? [a.id] : [];
      if (!this.sel.size && a && KINDS[a.kind].isGroup) { ws.ungroup(a.id); return; }
      if (!ids.length) { toast('Pick layers to group: Shift- or ⌘-click rows', true); return; }
      ws.group(ids); this.sel.clear();
    } catch (e) { toast(e.message, true); }
  },
  pick(e, l) {
    const ws = app.ws;
    if (e.shiftKey || e.metaKey || e.ctrlKey) {
      if (!this.sel.size && ws.active && ws.active !== l.id) this.sel.add(ws.active);
      if (this.sel.has(l.id)) this.sel.delete(l.id); else this.sel.add(l.id);
      this.render(); return;
    }
    this.sel.clear(); ws.setActive(l.id);
  },
  render() {
    const ws = app.ws, layers = ws.doc.layers, lm = Render.layerMs || new Map();
    const nG = layers.filter(l => KINDS[l.kind].isGroup).length;
    this.count.textContent = `${layers.length - nG} layers${nG ? ` · ${nG} group${nG === 1 ? '' : 's'}` : ''}`;
    for (const id of [...this.sel]) if (!ws.layer(id)) this.sel.delete(id);
    const act = ws.activeLayer();
    this.grpBtn.textContent = !this.sel.size && act && KINDS[act.kind].isGroup ? 'Ungroup' : this.sel.size > 1 ? `Group ${this.sel.size}` : 'Group';
    this.list.textContent = '';
    if (!layers.length) { this.list.append(h('p', { class: 'empty padded' }, 'No layers. Add one with + Layer, or paste an image.')); return; }
    const hiddenBy = l => ws.ancestors(l).find(g => g.open === false);
    for (let i = layers.length - 1; i >= 0; i--) {
      const l = layers[i], K = KINDS[l.kind], on = l.id === ws.active;
      if (hiddenBy(l)) continue;
      const depth = ws.depth(l), grp = !!K.isGroup, dimmed = !l.visible || ws.ancestors(l).some(g => !g.visible);
      const eye = h('button', { class: 'eye' + (l.visible ? ' on' : ''), title: l.visible ? 'Hide' : 'Show', 'aria-pressed': String(l.visible), onclick: e => { e.stopPropagation(); ws.setLayer(l.id, { visible: !l.visible }); } });
      const th = h('button', { class: 'th' + (on && ws.target === 'pixels' && l.mask ? ' tgt' : ''), title: 'Layer', onclick: e => { e.stopPropagation(); ws.setActive(l.id, 'pixels'); } });
      if (l.raster) { const c = thumbCanvas(44, 30); drawRasterThumb(c, l.raster, ws.doc.w, ws.doc.h, l.ox, l.oy); th.append(c); }
      else if (l.kind === 'fill') th.style.background = l.params.color;
      else if (l.kind === 'gradient') th.style.background = `linear-gradient(${Math.round(Math.atan2(l.params.y1 - l.params.y0, l.params.x1 - l.params.x0) * 180 / Math.PI + 90)}deg, ${l.params.c0}${Math.round(l.params.a0 * 255).toString(16).padStart(2, '0')}, ${l.params.c1}${Math.round(l.params.a1 * 255).toString(16).padStart(2, '0')})`;
      else th.textContent = K.abbr;
      const ms = lm.get(l.id), src = Provenance.of(l);
      const sub = [grp ? `${ws.kids(l.id).length} inside` : null, BLEND_LABEL[l.blend], Math.round(l.opacity * 100) + ' %', ms != null ? fmtMs(ms) + ' ms' : null].filter(Boolean).join(' · ');
      const chip = src ? h('button', { class: 'srcchip', title: Provenance.title(src), onclick: e => { e.stopPropagation(); Provenance.reveal(l); } }, Provenance.label(src)) : null;
      const meta = h('div', { class: 'meta' }, h('span', { class: 'ln', ondblclick: e => this.rename(e.currentTarget, l) }, l.name), h('span', { class: 'ls' }, (grp ? 'group' : K.label.toLowerCase()) + ' · ' + sub, chip ? ' ' : null, chip));
      const mk = h('div', { class: 'mk' });
      if (l.mask) { const mt = h('button', { class: 'mth' + (on && ws.target === 'mask' ? ' tgt' : '') + (l.mask.on ? '' : ' off'), title: 'Mask: click to paint on it, Shift-click to switch it off', onclick: e => { e.stopPropagation(); if (e.shiftKey) ws.setLayer(l.id, { mask: { on: !l.mask.on } }); else ws.setActive(l.id, 'mask'); } }); const c = thumbCanvas(30, 30); drawMaskThumb(c, l, ws); mt.append(c); mk.append(mt); }
      const grip = h('span', { class: 'grip', title: 'Drag to reorder' }, '⋮⋮');
      grip.addEventListener('pointerdown', e => this.dragRow(e, l, i));
      mk.append(grip);
      const fold = grp ? h('button', { class: 'fold' + (l.open === false ? '' : ' open'), title: l.open === false ? 'Show the layers inside' : 'Fold the group', 'aria-expanded': String(l.open !== false), onclick: e => { e.stopPropagation(); ws.setOpen(l.id, l.open === false); } }, l.open === false ? '▸' : '▾') : h('span', { class: 'fold none' });
      const row = h('div', { class: 'lrow' + (on ? ' on' : '') + (this.sel.has(l.id) ? ' picked' : '') + (dimmed ? ' hiddenl' : '') + (grp ? ' grp' : ''), role: 'listitem', 'data-index': i, 'data-id': l.id, style: { paddingLeft: (10 + depth * 16) + 'px' }, onclick: e => this.pick(e, l) }, eye, fold, th, meta, mk);
      this.list.append(row);
    }
    this.renderFoot();
  },
  renderFoot() {
    if (!this.foot) return;
    const lm = Render.layerMs || new Map(); let worst = null;
    for (const l of app.ws.doc.layers) { const v = lm.get(l.id); if (v != null && (!worst || v > worst[1])) worst = [l.name, v]; }
    this.foot.replaceChildren(h('span', null, `${app.ws.doc.w} × ${app.ws.doc.h} px`), h('span', null, worst ? `costliest in the last update: ${worst[0]} ${fmtMs(worst[1])} ms` : 'no layer computed in the last update'));
  },
  rename(span, l) {
    const inp = h('input', { class: 'ren', value: l.name });
    const done = ok => { const v = inp.value.trim(); inp.replaceWith(span); if (ok && v && v !== l.name) app.ws.setLayer(l.id, { name: v }); };
    inp.addEventListener('keydown', e => { if (e.key === 'Enter') done(true); if (e.key === 'Escape') done(false); });
    inp.addEventListener('blur', () => done(true)); inp.addEventListener('click', e => e.stopPropagation());
    span.replaceWith(inp); inp.select();
  },
  // Drop on the top or bottom third of a row: above or below that layer, beside it. On the middle of a
  // group's row: into the group, at the top of its layers.
  dragRow(e, l, i) {
    e.preventDefault(); e.stopPropagation();
    const ws = app.ws, rows = [...this.list.querySelectorAll('.lrow')]; let drop = null;
    const clear = () => rows.forEach(r => r.classList.remove('dropbefore', 'dropafter', 'dropinto'));
    const mv = ev => {
      clear(); drop = null;
      const r = rows.find(r => { const b = r.getBoundingClientRect(); return ev.clientY >= b.top && ev.clientY < b.bottom; }); if (!r) return;
      const t = ws.layer(r.dataset.id); if (!t || t === l || ws.ancestors(t).includes(l)) return;
      const b = r.getBoundingClientRect(), f = (ev.clientY - b.top) / b.height, grp = KINDS[t.kind].isGroup;
      // the list shows the top layer first, so the upper part of a row means above that layer
      let where = grp && f > 0.3 && f < 0.7 ? 'inside' : f < 0.5 ? 'above' : 'below';
      if (where === 'below' && grp && t.open !== false && ws.kids(t.id).length) where = 'inside';   // under an open group's row is its first layer
      r.classList.add(where === 'inside' ? 'dropinto' : where === 'above' ? 'dropbefore' : 'dropafter');
      drop = { t, where };
    };
    const up = () => { removeEventListener('pointermove', mv); removeEventListener('pointerup', up); clear(); document.body.classList.remove('dragging-view'); if (drop) { try { ws.moveTo(l.id, drop.t.id, drop.where); } catch (err) { toast(err.message, true); } } };
    document.body.classList.add('dragging-view');
    addEventListener('pointermove', mv); addEventListener('pointerup', up);
  },
};
function compMenu(l) {
  const ws = app.ws, run = f => () => { try { f(); } catch (e) { toast(e.message, true); } }, op = l.mask ? app.maskOp : 'add';
  return [
    { group: l.mask ? `Add a component (${op}; change it after)` : 'Start the mask with' },
    { label: 'Linear gradient', small: 'full at the start line, nothing past the end line', run: run(() => ws.addMaskComp(l.id, 'linear', { op })) },
    { label: 'Radial gradient', small: 'an ellipse, feathered towards its edge', run: run(() => ws.addMaskComp(l.id, 'radial', { op })) },
    { label: 'Brush', small: 'starts empty: paint where the layer applies', run: run(() => { ws.addMask(l.id, 'hide', { op }); setTool('brush'); }) },
    { label: 'Luminance range', small: 'where what is below is between two levels', run: run(() => ws.addMaskComp(l.id, 'luminance', { op })) },
    { label: 'Colour range', small: 'where what is below is near a colour', run: run(() => ws.addMaskComp(l.id, 'color', { op, params: { color: app.brush.color } })) },
    { label: 'From the selection', disabled: !ws.doc.sel, run: run(() => ws.addMask(l.id, 'selection', { op })) },
  ];
}
const ADD_ABOUT = { group: 'a folder for layers: drag layers in, or select several and press Group', image: 'a picture file from your computer', paint: 'empty pixels for the brush', fill: 'one colour over everything', gradient: 'two colours and two opacities between two points', exposure: 'brightness in stops, in linear light', levels: 'black point, white point, midtone gamma', curves: 'contrast, shadows, midtones, highlights', hueSat: 'rotate hues, push saturation', vibrance: 'saturation that spares saturated colours', temperature: 'warm or cool, green or magenta', bw: 'grey from a weighted channel mix', invert: 'one minus each channel', threshold: 'black or white by luminance', posterize: 'a few levels per channel', blur: 'Gaussian, as three box blurs', sharpen: 'unsharp mask', clarity: 'large-radius local contrast in the midtones', vignette: 'darker or lighter towards the corners', grain: 'seeded noise, strongest in the midtones', code: 'your own JavaScript, run on every tile; written in the Code view' };
function openFileAsLayer(l) {
  const inp = h('input', { type: 'file', accept: 'image/*', style: { display: 'none' } });
  inp.onchange = async () => { const f = inp.files[0]; inp.remove(); if (!f) return; try { const img = await readImageBlob(f); const ws = app.ws, W = ws.doc.w, H = ws.doc.h, k = Math.min(1, W / img.width, H / img.height), w = Math.round(img.width * k), hh = Math.round(img.height * k);
    const r = canvasRaster(W, H, g => g.drawImage(img, Math.round((W - w) / 2), Math.round((H - hh) / 2), w, hh)); l.raster.data.set(r.data); l.raster.bumpAll(); l.name = f.name.replace(/\.[^.]+$/, ''); ws.changed('pixels'); } catch (e) { toast('Could not open that file: ' + e.message, true); } };
  document.body.append(inp); inp.click();
}
registerView('Layers', { title: 'Layers', about: 'the stack, top first: visibility, order, masks', create: () => Layers.create(), badge: () => app.ws ? app.ws.doc.layers.length : null });

// ---- Params ------------------------------------------------------------------------------------
const Params = {
  create() {
    this.body = h('div', { class: 'scroll paramlist' });
    this.lbl = h('span', { class: 'lbl' });
    const el = h('div', { class: 'tabbody col' }, h('div', { class: 'toolbar' }, this.lbl, h('span', { class: 'sp' }), h('button', { class: 'btn sm', title: 'Save this layer as a new preset', onclick: () => Presets.fromLayers([app.ws.active]) }, 'Save as preset')), this.body);
    const re = debounce(() => { if (!this.dragging) this.render(); }, 80);
    app.on('active', re); app.on('load', re); app.on('hist', re); app.on('doc', e => { if (!e.interactive) re(); });
    this.render();
    return el;
  },
  render() {
    const ws = app.ws, l = ws.activeLayer(); this.body.textContent = '';
    if (!l) { this.lbl.textContent = 'no layer'; this.body.append(h('p', { class: 'empty padded' }, 'Select a layer in Layers to see its settings.')); return; }
    const K = KINDS[l.kind];
    this.lbl.replaceChildren('Settings of ', h('b', null, l.name), ' · ' + K.label.toLowerCase());
    const live = (patch) => { this.dragging = true; ws.live(l.id, patch); };
    const commit = (patch, label) => { this.dragging = false; if (l._before) { ws.applyPatch(l, patch); ws.commit(l.id, label); } else ws.setLayer(l.id, patch, label); };
    const name = h('input', { class: 'ptitle-in', value: l.name, 'aria-label': 'Layer name', onchange: e => e.target.value.trim() && ws.setLayer(l.id, { name: e.target.value.trim() }) });
    this.body.append(h('div', { class: 'block' }, h('header', null, name, h('span', { class: 'sub' }, l.id)), h('p', { class: 'note' }, ADD_ABOUT[l.kind] || '')));
    this.body.append(h('div', { class: 'psub' }, 'layer'));
    const dflt = K.isGroup ? 'pass' : 'normal', modes = K.isGroup ? ['pass', ...BLEND_MODES] : BLEND_MODES;
    const blendSel = h('select', { class: 'selectlike', 'aria-label': 'Blend mode', onchange: e => ws.setLayer(l.id, { blend: e.target.value }) }, modes.map(m => h('option', { value: m, selected: m === l.blend }, BLEND_LABEL[m])));
    this.body.append(h('div', { class: 'prow' }, h('span', { class: 'pn' }, 'blend'), h('span', { class: 'pc' }, blendSel), h('span', { class: 'pv' }, ''), h('button', { class: 'tabact', disabled: l.blend === dflt, title: 'default ' + BLEND_LABEL[dflt], onclick: () => ws.setLayer(l.id, { blend: dflt }) }, l.blend === dflt ? '' : '↺')));
    if (K.isGroup) this.body.append(h('div', { class: 'block' }, h('p', { class: 'note' }, l.blend === 'pass'
      ? `Pass through: the ${ws.kids(l.id).length} layers inside apply to everything below, as if they were not grouped; opacity and mask fade their whole effect.`
      : `Isolated (${BLEND_LABEL[l.blend]}): the layers inside start from transparent, so adjustments in the group change only the group's own layers; the result is blended onto what is below.`)));
    this.body.append(paramRow({ label: 'opacity', spec: PCT(0, 1, 1, 'opacity'), value: l.opacity, onLive: v => live({ opacity: v }), onCommit: v => commit({ opacity: v }) }));
    if (l.raster) {
      this.body.append(paramRow({ label: 'offset x', spec: P(-ws.doc.w, ws.doc.w, 0, 'px', '', 1), value: l.ox, onLive: v => live({ ox: v }), onCommit: v => commit({ ox: v }, 'Move ' + l.name) }));
      this.body.append(paramRow({ label: 'offset y', spec: P(-ws.doc.h, ws.doc.h, 0, 'px', '', 1), value: l.oy, onLive: v => live({ oy: v }), onCommit: v => commit({ oy: v }, 'Move ' + l.name) }));
    }
    this.body.append(this.sourceBlock(l));
    const ks = Object.entries(specsOf(l.kind, l.params)).filter(([, sp]) => sp.type !== 'text');
    if (l.kind === 'code') this.body.append(this.codeBlock(l));
    if (ks.length) {
      this.body.append(h('div', { class: 'psub' }, l.kind));
      if (['curves', 'levels', 'exposure'].includes(l.kind)) this.body.append(h('div', { class: 'block' }, this.curvePlot(l)));
      for (const [k, spec] of ks) this.body.append(paramRow({ label: spec.label || k, id: k, spec, value: l.params[k], onLive: v => live({ params: { [k]: v } }), onCommit: v => commit({ params: { [k]: v } }) }));
    }
    this.body.append(h('div', { class: 'psub' }, 'mask', h('span', { class: 'sp' }), l.mask ? h('span', { class: 'dim' }, `${l.mask.comps.length} component${l.mask.comps.length === 1 ? '' : 's'}${l.mask.on ? '' : ' · off'}`) : null));
    if (!l.mask) {
      this.body.append(h('div', { class: 'block' }, h('p', { class: 'note' }, 'No mask: the layer applies everywhere. A mask is a list of components (linear and radial gradients, brush strokes, luminance and colour ranges) combined in order.'),
        h('div', { class: 'row', style: { display: 'flex', gap: '6px', flexWrap: 'wrap' } }, h('button', { class: 'btn sm', onclick: e => openMenu(e.currentTarget, compMenu(l)) }, '+ Add mask ▾'))));
      return;
    }
    const m = l.mask, act = ws.activeComp(l);
    this.body.append(paramRow({ label: 'density', spec: PCT(0, 1, 1, 'density'), value: m.density, onLive: v => live({ mask: { density: v } }), onCommit: v => commit({ mask: { density: v } }) }));
    this.body.append(h('div', { class: 'block', style: { paddingTop: '8px', paddingBottom: '8px' } }, h('div', { style: { display: 'flex', gap: '6px', flexWrap: 'wrap' } },
      toggle('invert all', m.invert, v => ws.setLayer(l.id, { mask: { invert: v } })), toggle('mask on', m.on, v => ws.setLayer(l.id, { mask: { on: v } })),
      h('button', { class: 'btn sm', onclick: e => openMenu(e.currentTarget, compMenu(l)) }, '+ Component ▾'), h('button', { class: 'link', onclick: () => ws.deleteMask(l.id) }, 'delete mask'))));
    const list = h('div', { class: 'comps', role: 'list' });
    m.comps.forEach((c, i) => {
      const K = COMP_KINDS[c.type], on = c === act && ws.target === 'mask';
      const op = i === 0 ? h('span', { class: 'cbase', title: 'the first component is the base; the rest combine with it in order' }, 'base')
        : segmented(COMP_OPS, c.op, v => ws.editComp(l.id, c.id, { op: v }), { sm: true, labels: { add: '+', subtract: '−', intersect: '∩' }, title: 'add (union) · subtract · intersect' });
      list.append(h('div', { class: 'crow' + (on ? ' on' : '') + (c.on ? '' : ' off'), role: 'listitem', onclick: () => ws.setComp(l.id, c.id) },
        h('button', { class: 'eye' + (c.on ? ' on' : ''), title: c.on ? 'Switch this component off' : 'Switch it on', onclick: e => { e.stopPropagation(); ws.editComp(l.id, c.id, { on: !c.on }); } }),
        op, h('span', { class: 'cn' }, h('b', null, c.name), h('small', null, ' ' + K.label.toLowerCase() + (c.invert ? ' · inverted' : '') + (c.amount < 1 ? ` · ${Math.round(c.amount * 100)} %` : ''))),
        h('span', { class: 'cact' }, h('button', { class: 'tabact', title: 'Earlier', disabled: i === 0, onclick: e => { e.stopPropagation(); ws.moveComp(l.id, c.id, i - 1); } }, '↑'), h('button', { class: 'tabact', title: 'Later', disabled: i === m.comps.length - 1, onclick: e => { e.stopPropagation(); ws.moveComp(l.id, c.id, i + 1); } }, '↓'),
          h('button', { class: 'tabact x', title: 'Remove', onclick: e => { e.stopPropagation(); ws.removeComp(l.id, c.id); } }, '×'))));
    });
    this.body.append(list);
    if (act) {
      const K = COMP_KINDS[act.type], liveC = f => { this.dragging = true; ws.liveMask(l.id, f); }, commitC = (f, label) => { this.dragging = false; if (!l._before) l._before = layerProps(l); f(); ws.commit(l.id, label || `${l.name}: ${act.name.toLowerCase()}`); };
      this.body.append(h('div', { class: 'psub' }, act.name, h('span', { class: 'sp' }), h('span', { class: 'dim' }, K.label.toLowerCase())));
      this.body.append(h('div', { class: 'block', style: { paddingTop: '8px', paddingBottom: '8px' } }, h('div', { style: { display: 'flex', gap: '6px', flexWrap: 'wrap', alignItems: 'center' } },
        h('input', { class: 'ptitle-in', value: act.name, 'aria-label': 'Component name', style: { fontWeight: 400, fontSize: 'var(--fs-s)', maxWidth: '160px' }, onchange: e => e.target.value.trim() && ws.editComp(l.id, act.id, { name: e.target.value.trim() }) }),
        toggle('invert', act.invert, v => ws.editComp(l.id, act.id, { invert: v })),
        act.raster ? h('button', { class: 'btn sm', onclick: () => { ws.setComp(l.id, act.id); setTool('brush'); Dock.focus('Stage'); } }, 'Paint it') : null,
        act.raster ? h('button', { class: 'btn sm', disabled: !ws.doc.sel, onclick: () => { ws.setComp(l.id, act.id); ws.fillSelection('#ffffff'); } }, 'Fill selection') : null,
        act.type === 'color' ? h('button', { class: 'btn sm', title: 'Use the brush colour (pick one with the Pick tool)', onclick: () => ws.editComp(l.id, act.id, { params: { color: app.brush.color } }) }, 'Use brush colour') : null)));
      this.body.append(paramRow({ label: 'amount', spec: PCT(0, 1, 1, 'amount'), value: act.amount, onLive: v => liveC(() => { act.amount = v; }), onCommit: v => commitC(() => { act.amount = v; }) }));
      for (const [k, spec] of Object.entries(K.params)) this.body.append(paramRow({ label: spec.label || k, id: k, spec, value: act.params[k], onLive: v => liveC(() => { act.params[k] = v; }), onCommit: v => commitC(() => { act.params[k] = v; }) }));
      if (act.type === 'linear' || act.type === 'radial') this.body.append(h('p', { class: 'note padded' }, 'Or drag its pin and handles on the stage: the square handles resize, the round one rotates, the inner one sets the feather.'));
    }
  },
  // Where the layer came from, with links back to it, and a note kept in its metadata.
  sourceBlock(l) {
    const ws = app.ws, src = Provenance.of(l), go = w => () => { try { Provenance.reveal(l, w); } catch (e) { toast(e.message, true); } };
    const links = [];
    if (src && src.kind === 'preset') links.push(h('button', { class: 'btn sm', onclick: go() }, `Preset ${src.id}`), h('button', { class: 'btn sm', onclick: go('code') }, 'Its code, at this layer'));
    else if (src && (src.kind === 'program' || src.kind === 'script')) links.push(h('button', { class: 'btn sm', onclick: go() }, `Open ${Provenance.label(src)}`));
    else if (src && src.kind === 'agent') links.push(h('button', { class: 'btn sm', onclick: go() }, 'Show the agent step'));
    if (src && src.via) links.push(h('span', { class: 'dim' }, ` applied by ${Provenance.label(src.via)}`));
    const note = h('input', { class: 'selectlike', value: (l.meta && l.meta.note) || '', placeholder: 'a note kept with the layer', 'aria-label': 'Note', onchange: e => ws.setMeta(l.id, { note: e.target.value.trim() || null }, `${l.name}: note`) });
    const what = !src ? 'Made here, by hand.' : src.kind === 'preset' ? `Made by preset ${src.id} ${src.name || ''}.` : src.kind === 'program' ? `Made by the program ${src.file}${src.line ? ', line ' + src.line : ''}.` : src.kind === 'script' ? `Added by the script ${src.file}${src.line ? ', line ' + src.line : ''}.` : src.kind === 'agent' ? `Added by the agent, step ${(src.step ?? 0) + 1}.` : `From ${src.kind}.`;
    return h('div', null, h('div', { class: 'psub' }, 'source'), h('div', { class: 'block' }, h('p', { class: 'note' }, what + (src && src.at ? ` ${new Date(src.at).toLocaleString()}.` : '')),
      links.length ? h('div', { class: 'row', style: { display: 'flex', gap: '6px', flexWrap: 'wrap', alignItems: 'center' } }, links) : null,
      h('div', { class: 'row', style: { marginTop: '6px' } }, note)));
  },
  codeBlock(l) {
    const m = codeModule(l.params.src), part = m.byName.get(l.params.part), err = m.error || (m.runtime && m.runtime.part === l.params.part ? m.runtime : null);
    const what = !part ? 'not found in its program' : part.src ? 'a source: paint()' : part.roi ? 'reads neighbours: roiFn()' : 'pointwise: fn()';
    return h('div', { class: 'block' },
      h('p', { class: 'note' }, `Code layer '${l.params.part}' · ${what} · ${m.parts.length} layer${m.parts.length === 1 ? '' : 's'} in a ${l.params.src.split('\n').length}-line program${l.params.preset ? ` (preset ${l.params.preset})` : ''}`),
      err ? h('p', { class: 'fail' }, h('b', null, 'Error: '), err.message + (err.line ? ` (line ${err.line})` : ''), '. The layer passes its backdrop through.') : null,
      h('div', null, h('button', { class: 'btn sm', onclick: () => { try { Code.openLayer(l.id); } catch (e) { toast(e.message, true); } } }, 'Edit as code')));
  },
  curvePlot(l) {
    const N = 64, pts = [], t = new Float32Array(N * 4);
    for (let i = 0; i < N; i++) { t[i * 4] = t[i * 4 + 1] = t[i * 4 + 2] = i / (N - 1); t[i * 4 + 3] = 1; }
    const K = kindOf(l); K.fn(t, N, l.params, { prep: K.prep ? K.prep(l.params) : null });
    for (let i = 0; i < N; i++) pts.push(`${(i / (N - 1) * 200).toFixed(1)},${(100 - clamp(t[i * 4], 0, 1) * 100).toFixed(1)}`);
    const svg = `<svg class="curve" viewBox="-2 -2 204 104" role="img" aria-label="Transfer curve: input left to right, output bottom to top"><path class="gl" d="M50 0V100M100 0V100M150 0V100M0 25H200M0 50H200M0 75H200"/><path class="id" d="M0 100L200 0"/><polyline class="cv" points="${pts.join(' ')}"/></svg>`;
    return h('div', { html: svg + '<p class="note">input left to right, output bottom to top; the dashed line changes nothing</p>' });
  },
};
registerView('Params', { title: 'Params', about: 'the active layer’s settings, as sliders', create: () => Params.create() });

// ---- History -----------------------------------------------------------------------------------
const HistoryView = {
  create() {
    this.list = h('div', { class: 'scroll' }); this.lbl = h('span', { class: 'lbl' });
    const el = h('div', { class: 'tabbody col' }, h('div', { class: 'toolbar' }, h('button', { class: 'btn sm', onclick: () => app.ws.undo() }, 'Undo'), h('button', { class: 'btn sm', onclick: () => app.ws.redo() }, 'Redo'), h('span', { class: 'sp' }), this.lbl), this.list);
    const re = debounce(() => this.render(), 60); app.on('hist', re); app.on('load', re);
    this.render(); return el;
  },
  render() {
    const H = app.ws.hist; this.list.textContent = '';
    let bytes = 0; for (const e of H.stack) bytes += e.bytes || 0;
    this.lbl.textContent = `${H.stack.length} steps · ${fmtMB(bytes)} of stored pixels`;
    const row = (n, label, detail, sub) => h('button', { class: 'hrow' + (n === H.ptr ? ' on' : '') + (n > H.ptr ? ' future' : ''), onclick: () => app.ws.jump(n) }, h('span', { class: 'n' }, n), h('span', null, label, detail ? h('span', { class: 'd' }, ' · ' + detail) : null), h('span', { class: 'd' }, sub || ''));
    this.list.append(row(0, 'Open ' + app.ws.doc.code + ' ' + app.ws.doc.title, null, ''));
    H.stack.forEach((e, i) => this.list.append(row(i + 1, e.label, e.detail, e.bytes ? fmtMB(e.bytes) : '')));
    if (!H.stack.length) this.list.append(h('p', { class: 'empty padded' }, 'Nothing changed yet. Every edit lands here; click a row to go back to it.'));
    const on = this.list.querySelector('.hrow.on'); if (on) on.scrollIntoView({ block: 'nearest' });
  },
};
registerView('History', { title: 'History', about: 'every edit, undoable; click a row to go back to it', create: () => HistoryView.create(), badge: () => app.ws && app.ws.hist.stack.length ? app.ws.hist.ptr : null });
