// ---------------------------------------------------------------------------------------------
// workspace.js: a document with its engine and history, and every action that changes it.
// The app is one Workspace; the bench makes others from copies of the document.
// ---------------------------------------------------------------------------------------------
'use strict';

class Workspace {
  constructor(doc, opts = {}) {
    this.doc = doc;
    this.engine = new Engine(doc, { tile: opts.tile || 128, budgetMB: opts.budgetMB || 96, evict: opts.evict || 'lru' });
    this.hist = { stack: [], ptr: 0 };
    this.active = doc.layers.length ? doc.layers[doc.layers.length - 1].id : null;
    this.target = 'pixels';
    this.rec = null;
    this.listeners = {};
  }
  on(ev, fn) { (this.listeners[ev] = this.listeners[ev] || []).push(fn); }
  emit(ev, data) { for (const fn of this.listeners[ev] || []) fn(data); }
  changed(what = 'doc', interactive = false) { this.engine.invalidate(); this.emit('doc', { what, interactive }); }

  // ---- lookups ----
  layer(id) { return typeof id === 'object' ? id : this.doc.layers.find(l => l.id === id || l.name === id) || null; }
  index(id) { const l = this.layer(id); return l ? this.doc.layers.indexOf(l) : -1; }
  activeLayer() { return this.layer(this.active) || this.doc.layers[this.doc.layers.length - 1] || null; }
  setActive(id, target) {
    const l = this.layer(id); if (!l) return;
    this.active = l.id;
    this.target = target || ((this.target === 'mask' || !l.raster) && l.mask ? 'mask' : 'pixels');
    if (this.target === 'mask' && l.mask && !l.mask.comps.find(c => c.id === this.compId)) this.compId = (l.mask.comps[l.mask.comps.length - 1] || {}).id;
    if (this.target === 'mask' && !l.mask) this.target = 'pixels';
    this.emit('active', l);
  }

  // ---- history ----
  push(entry) {
    this.hist.stack.length = this.hist.ptr;
    entry.t = Date.now(); entry.bytes = entry.bytes || 0;
    this.hist.stack.push(entry); this.hist.ptr++;
    if (this.hist.stack.length > 200) { this.hist.stack.shift(); this.hist.ptr--; }
    this.emit('hist');
  }
  undo() { if (!this.hist.ptr) return false; const e = this.hist.stack[--this.hist.ptr]; e.undo(); this.log({ op: 'undo' }); this.changed('undo'); this.emit('hist'); return e.label; }
  redo() { if (this.hist.ptr >= this.hist.stack.length) return false; const e = this.hist.stack[this.hist.ptr++]; e.redo(); this.log({ op: 'redo' }); this.changed('redo'); this.emit('hist'); return e.label; }
  jump(n) { while (this.hist.ptr > n && this.undo()); while (this.hist.ptr < n && this.redo()); }
  log(op) { if (this.rec) this.rec.ops.push(Object.assign({ t: (performance.now() - this.rec.t0) / 1000 }, JSON.parse(propsJSON(op)))); }

  // ---- layers ----
  addLayer(kind, opts = {}, at) {
    if (!KINDS[kind]) throw new Error(`no layer kind '${kind}'; kinds are ${Object.keys(KINDS).join(', ')}`);
    if (opts.parent) { const g = this.layer(opts.parent); if (!g || !KINDS[g.kind].isGroup) throw new Error(`'${opts.parent}' is not a group`); opts = Object.assign({}, opts, { parent: g.id }); const gi = this.doc.layers.indexOf(g); at = at == null ? gi : clamp(at, this.doc.layers.indexOf(this.block(g)[0]), gi); }
    else if (at != null && this.hasGroups()) opts = Object.assign({}, opts, { parent: this.parentAt(clamp(at, 0, this.doc.layers.length)) });
    const l = makeLayer(kind, opts);
    if (KINDS[kind].raster) l.raster = opts.raster || new Raster(this.doc.w, this.doc.h, 4);
    if (opts.mask) l.mask = maskFromSpec(opts.mask, this.doc.w, this.doc.h);
    const i = at == null ? this.doc.layers.length : clamp(at, 0, this.doc.layers.length);
    const prevActive = this.active;
    const doIt = () => { this.doc.layers.splice(i, 0, l); this.active = l.id; this.target = 'pixels'; };
    doIt();
    this.push({ label: 'Add ' + l.name, detail: KINDS[kind].label.toLowerCase(), bytes: l.raster ? l.raster.data.length : 0,
      undo: () => { this.doc.layers.splice(this.doc.layers.indexOf(l), 1); this.active = prevActive; }, redo: doIt });
    this.log({ op: 'add', i, kind, opts: { name: l.name, params: l.params, opacity: l.opacity, blend: l.blend } });
    this.changed('layers');
    return l;
  }
  removeLayer(id) {
    const l = this.layer(id); if (!l) throw new Error(`no layer '${id}'`);
    if (KINDS[l.kind].isGroup) {
      const blk = new Set(this.block(l)), i0 = this.doc.layers.indexOf(this.block(l)[0]), rest = this.doc.layers.filter(x => !blk.has(x));
      this.restructure(rest, new Map(), `Delete ${l.name}`, { detail: `group and ${blk.size - 1} layer${blk.size === 2 ? '' : 's'}`, active: (rest[Math.max(0, i0 - 1)] || rest[0] || {}).id || null });
      return;
    }
    const i = this.doc.layers.indexOf(l);
    const doIt = () => { this.doc.layers.splice(i, 1); if (this.active === l.id) this.active = (this.doc.layers[Math.min(i, this.doc.layers.length - 1)] || {}).id || null; };
    doIt();
    this.push({ label: 'Delete ' + l.name, undo: () => { this.doc.layers.splice(i, 0, l); this.active = l.id; }, redo: doIt });
    this.log({ op: 'remove', i });
    this.changed('layers');
  }
  moveLayer(id, to) {
    const l = this.layer(id); if (!l) throw new Error(`no layer '${id}'`);
    const from = this.doc.layers.indexOf(l); to = clamp(to, 0, this.doc.layers.length - 1);
    if (from === to) return;
    if (this.hasGroups()) {
      // With groups, a layer moves with its descendants, and its new parent is read from the layer above it:
      // directly under a group is inside that group, under any other layer is beside it.
      const blk = this.block(l), set = new Set(blk), rest = this.doc.layers.filter(x => !set.has(x));
      const at = clamp(to - (blk.length - 1) + (to > from ? 0 : 0), 0, rest.length), order = [...rest.slice(0, at), ...blk, ...rest.slice(at)];
      const above = order[at + blk.length], parent = !above ? null : KINDS[above.kind].isGroup ? above.id : above.parent || null;
      this.restructure(order, new Map([[l, parent]]), 'Move ' + l.name, { detail: `${from + 1} → ${order.indexOf(l) + 1}` });
      return;
    }
    const mv = (a, b) => { this.doc.layers.splice(a, 1); this.doc.layers.splice(b, 0, l); };
    mv(from, to);
    this.push({ label: 'Move ' + l.name, detail: `${from + 1} → ${to + 1}`, undo: () => mv(to, from), redo: () => mv(from, to) });
    this.log({ op: 'move', from, to });
    this.changed('layers');
  }
  duplicateLayer(id) {
    const l = this.layer(id); if (!l) return;
    if (KINDS[l.kind].isGroup) {
      const blk = this.block(l), map = new Map(), copies = blk.map(x => { const c = this.copyOf(x); map.set(x.id, c.id); return c; });
      for (const c of copies) if (map.has(c.parent)) c.parent = map.get(c.parent);
      const top = copies[copies.length - 1]; top.name = l.name + ' copy';
      const ls = this.doc.layers, i = ls.indexOf(l), order = [...ls.slice(0, i + 1), ...copies, ...ls.slice(i + 1)];
      this.restructure(order, new Map(), 'Duplicate ' + l.name, { active: top.id });
      return top;
    }
    const opts = { name: l.name + ' copy', params: JSON.parse(JSON.stringify(l.params)), opacity: l.opacity, blend: l.blend, visible: l.visible, ox: l.ox, oy: l.oy };
    if (l.raster) opts.raster = l.raster.clone();
    if (l.mask) opts.mask = cloneMask(l.mask);
    return this.addLayer(l.kind, opts, this.doc.layers.indexOf(l) + 1);
  }
  // ---- groups: the order of doc.layers plus each layer's parent. A group's descendants sit directly
  // below it, so a group and its contents always form one contiguous block. ----
  hasGroups() { return this.doc.layers.some(l => l.parent || KINDS[l.kind].isGroup); }
  isGroup(id) { const l = this.layer(id); return !!l && !!KINDS[l.kind].isGroup; }
  kids(id) { const g = this.layer(id); return g ? this.doc.layers.filter(l => l.parent === g.id) : []; }
  ancestors(id) { const out = []; let l = this.layer(id); while (l && l.parent) { l = this.layer(l.parent); if (l) out.push(l); } return out; }
  depth(id) { return this.ancestors(id).length; }
  // A layer and everything inside it, bottom to top.
  block(id, list = this.doc.layers) {
    const l = this.layer(id), i = list.indexOf(l), ids = new Set([l.id]), out = [l];
    for (let j = i - 1; j >= 0 && ids.has(list[j].parent); j--) { out.unshift(list[j]); if (KINDS[list[j].kind].isGroup) ids.add(list[j].id); }
    return out;
  }
  // The parent a layer inserted at index `at` would have: inside a group when it lands right under the group.
  parentAt(at) { const above = this.doc.layers[at]; return !above ? null : KINDS[above.kind].isGroup ? above.id : above.parent || null; }
  copyOf(x) {
    const c = makeLayer(x.kind, { name: x.name, params: JSON.parse(JSON.stringify(x.params)), opacity: x.opacity, blend: x.blend, visible: x.visible, ox: x.ox, oy: x.oy, parent: x.parent, meta: x.meta, open: x.open });
    if (x.raster) c.raster = x.raster.clone(); if (x.mask) c.mask = cloneMask(x.mask); return c;
  }
  // One undo step that replaces the order of the stack and some parents.
  restructure(order, parents, label, opts = {}) {
    const ls = this.doc.layers, before = ls.slice(), beforeP = new Map(before.map(l => [l, l.parent])), prev = this.active;
    const after = order.slice(), afterP = new Map(after.map(l => [l, parents.has(l) ? parents.get(l) : l.parent]));
    const put = (ord, par) => { ls.splice(0, ls.length, ...ord); for (const [l, p] of par) { if (p) l.parent = p; else delete l.parent; } };
    const doIt = () => { put(after, afterP); if (opts.active !== undefined) this.active = opts.active; };
    doIt();
    this.push({ label, detail: opts.detail, undo: () => { put(before, beforeP); this.active = prev; }, redo: doIt });
    this.log({ op: 'restructure', label });
    this.changed('layers');
  }
  // Put layers in a new group, at the place of the topmost of them, inside that one's parent.
  group(ids, opts = {}) {
    const ls = this.doc.layers, picked = [...new Set((Array.isArray(ids) ? ids : [ids]).map(id => this.layer(id)).filter(Boolean))];
    if (!picked.length) throw new Error('nothing to group: name one or more layers');
    const set = new Set(picked), roots = picked.filter(l => !this.ancestors(l).some(a => set.has(a)));
    const moving = new Set(); for (const r of roots) for (const x of this.block(r)) moving.add(x);
    const topRoot = roots.reduce((a, b) => ls.indexOf(a) > ls.indexOf(b) ? a : b), ti = ls.indexOf(topRoot);
    const n = ls.filter(l => KINDS[l.kind].isGroup).length + 1;
    const g = makeLayer('group', { name: opts.name || 'Group ' + n, blend: opts.blend, opacity: opts.opacity, meta: opts.meta, parent: topRoot.parent });
    if (opts.blend && opts.blend !== 'pass' && !BLEND_MODES.includes(opts.blend)) throw new Error(`no blend mode '${opts.blend}'; a group takes pass or ${BLEND_MODES.join(', ')}`);
    const order = [];
    ls.forEach((l, i) => { if (!moving.has(l)) order.push(l); else if (i === ti) { for (const m of ls) if (moving.has(m)) order.push(m); order.push(g); } });
    const par = new Map(roots.map(r => [r, g.id])); par.set(g, topRoot.parent || null);
    this.restructure(order, par, `Group ${moving.size} layer${moving.size === 1 ? '' : 's'}`, { detail: g.name, active: g.id });
    return g;
  }
  // Take a group's children out to its own parent and remove the group.
  ungroup(id) {
    const g = this.layer(id); if (!g || !KINDS[g.kind].isGroup) throw new Error(`'${id}' is not a group`);
    const kids = this.kids(g.id), order = this.doc.layers.filter(l => l !== g);
    this.restructure(order, new Map(kids.map(k => [k, g.parent || null])), 'Ungroup ' + g.name, { detail: `${kids.length} layer${kids.length === 1 ? '' : 's'} out`, active: (kids[kids.length - 1] || {}).id || this.active });
    return kids;
  }
  // Move a layer (and its contents) above or below another, or into a group as its top child.
  moveTo(id, targetId, where = 'above') {
    const l = this.layer(id), t = this.layer(targetId);
    if (!l || !t) throw new Error(`no layer '${l ? targetId : id}'`);
    if (!['above', 'below', 'inside'].includes(where)) throw new Error("where is 'above', 'below' or 'inside'");
    if (l === t) return;
    const blk = this.block(l), set = new Set(blk);
    if (set.has(t)) throw new Error('a group cannot go inside itself');
    if (where === 'inside' && !KINDS[t.kind].isGroup) throw new Error(`${t.name} is not a group`);
    const rest = this.doc.layers.filter(x => !set.has(x)), ti = rest.indexOf(t);
    const at = where === 'above' ? ti + 1 : where === 'inside' ? ti : rest.indexOf(this.block(t, rest)[0]);
    const order = [...rest.slice(0, at), ...blk, ...rest.slice(at)], parent = where === 'inside' ? t.id : t.parent || null;
    if (order.every((x, i) => x === this.doc.layers[i]) && (l.parent || null) === parent) return;
    this.restructure(order, new Map([[l, parent]]), 'Move ' + l.name, { detail: `${where} ${t.name}` });
  }
  // Fold or unfold a group in the Layers panel: not a change to the picture, so not in the history.
  setOpen(id, open) { const g = this.layer(id); if (!g || !KINDS[g.kind].isGroup) return; g.open = !!open; this.emit('open', g); }
  // Metadata: where a layer came from (meta.source) and anything else worth keeping with it.
  setMeta(id, patch, label) {
    const l = this.layer(id); if (!l) throw new Error(`no layer '${id}'`);
    const before = l.meta ? JSON.parse(JSON.stringify(l.meta)) : null, after = Object.assign({}, before || {});
    for (const [k, v] of Object.entries(patch || {})) { if (v === null || v === undefined) delete after[k]; else after[k] = JSON.parse(JSON.stringify(v)); }
    const put = m => { if (m && Object.keys(m).length) l.meta = JSON.parse(JSON.stringify(m)); else delete l.meta; };
    put(after);
    this.push({ label: label || `${l.name}: metadata`, undo: () => put(before), redo: () => put(after) });
    this.emit('doc', { what: 'meta' });
    return l.meta || {};
  }
  // Settings: a patch of { name, visible, opacity, blend, ox, oy, params: {...}, mask: {...} }.
  checkPatch(l, patch) {
    const K = KINDS[l.kind];
    for (const k of Object.keys(patch)) if (!['name', 'visible', 'opacity', 'blend', 'ox', 'oy', 'params', 'mask'].includes(k)) throw new Error(`unknown setting '${k}' (settings are name, visible, opacity, blend, ox, oy, params, mask)`);
    if (patch.blend && !(BLEND_MODES.includes(patch.blend) || (K.isGroup && patch.blend === 'pass'))) throw new Error(`no blend mode '${patch.blend}'; modes are ${K.isGroup ? 'pass, ' : ''}${BLEND_MODES.join(', ')}`);
    if (patch.params && K.isGroup && Object.keys(patch.params).length) throw new Error('a group has no params: set its blend, opacity or mask');
    const specs = patch.params ? specsOf(l.kind, Object.assign({}, l.params, patch.params)) : null;
    if (patch.params) for (const [k, v] of Object.entries(patch.params)) {
      const s = specs[k]; if (!s) throw new Error(`${l.kind === 'code' ? 'this code layer' : l.kind} has no param '${k}'; its params are ${Object.keys(specs).filter(x => !(l.kind === 'code' && KINDS.code.params[x])).join(', ') || 'none'}`);
      if (s.type === 'text' && typeof v !== 'string') throw new Error(`${k} takes text`);
      if (s.type === 'color' && !/^#[0-9a-f]{6}$/i.test(v)) throw new Error(`${k} takes a colour like '#ff8800'`);
      if (s.type === 'choice' && !s.options.includes(v)) throw new Error(`${k} is one of ${s.options.join(', ')}`);
      if (!s.type && (typeof v !== 'number' || !Number.isFinite(v))) throw new Error(`${k} takes a number from ${s.min} to ${s.max}`);
    }
    if (patch.mask && !l.mask) throw new Error(`${l.name} has no mask; add one first`);
    if (patch.mask) for (const k of Object.keys(patch.mask)) if (!['density', 'invert', 'on', 'comps'].includes(k)) throw new Error(`mask settings are density, invert and on; '${k}' belongs to a component (mask.edit)`);
  }
  applyPatch(l, patch) {
    for (const k of ['name', 'visible', 'blend', 'ox', 'oy']) if (k in patch) l[k] = patch[k];
    if ('opacity' in patch) l.opacity = clamp(+patch.opacity, 0, 1);
    if (patch.params) { const specs = specsOf(l.kind, Object.assign({}, l.params, patch.params)); for (const [k, v] of Object.entries(patch.params)) { const s = specs[k]; l.params[k] = !s || s.type ? v : clamp(v, s.min, s.max); } }
    if (patch.mask && l.mask) { for (const k of ['density', 'invert', 'on']) if (k in patch.mask) l.mask[k] = patch.mask[k]; if (patch.mask.comps) l.mask.comps = patch.mask.comps.map(compJSON); }
  }
  // Replace a layer's name, opacity, blend, params and (when given) mask as one undo step.
  replaceLayer(id, spec, label) {
    const l = this.layer(id); if (!l) throw new Error(`no layer '${id}'`);
    const before = { props: layerProps(l), mask: l.mask };
    const mask = spec.mask !== undefined ? (spec.mask ? maskFromSpec(JSON.parse(JSON.stringify(spec.mask)), this.doc.w, this.doc.h) : null) : l.mask;
    const after = { name: spec.name ?? l.name, opacity: spec.opacity ?? l.opacity, blend: spec.blend || l.blend, params: JSON.parse(JSON.stringify(spec.params || l.params)), mask };
    const doIt = () => { l.name = after.name; l.opacity = after.opacity; l.blend = after.blend; l.params = JSON.parse(JSON.stringify(after.params)); l.mask = after.mask; };
    doIt();
    this.push({ label: label || 'Update ' + l.name, undo: () => { l.mask = before.mask; applyProps(l, before.props); }, redo: doIt });
    this.changed('props');
    return l;
  }
  setLayer(id, patch, label) {
    const l = this.layer(id); if (!l) throw new Error(`no layer '${id}'`);
    this.checkPatch(l, patch);
    const before = layerProps(l); this.applyPatch(l, patch); const after = layerProps(l);
    if (propsJSON(before) === propsJSON(after)) return l;
    this.push({ label: label || this.patchLabel(l, patch), undo: () => applyProps(l, before), redo: () => applyProps(l, after) });
    this.log({ op: 'set', i: this.doc.layers.indexOf(l), patch });
    this.changed('props');
    return l;
  }
  patchLabel(l, p) {
    if ('visible' in p) return (p.visible ? 'Show ' : 'Hide ') + l.name;
    if (p.params) { const k = Object.keys(p.params)[0]; return `${l.name}: ${k === 'src' ? 'code' : (specsOf(l.kind, l.params)[k] || {}).label || k}`; }
    if ('opacity' in p) return `${l.name}: opacity`;
    if ('blend' in p) return `${l.name}: ${BLEND_LABEL[p.blend]}`;
    if (p.mask) return `${l.name}: mask`;
    if ('name' in p) return 'Rename ' + l.name;
    return 'Change ' + l.name;
  }
  // Live edits (a slider being dragged) skip the history until commit.
  live(id, patch) {
    const l = this.layer(id); if (!l) return;
    if (!l._before) l._before = layerProps(l);
    this.applyPatch(l, patch); this.changed('props', true);
  }
  commit(id, label) {
    const l = this.layer(id); if (!l || !l._before) return;
    const before = l._before, after = layerProps(l); delete l._before;
    if (propsJSON(before) === propsJSON(after)) return;
    const patch = {}; for (const k of ['visible', 'opacity', 'blend', 'ox', 'oy']) if (before[k] !== after[k]) patch[k] = after[k];
    const dp = {}; for (const k in after.params) if (before.params[k] !== after.params[k]) dp[k] = after.params[k]; if (Object.keys(dp).length) patch.params = dp;
    if (after.mask && propsJSON(after.mask) !== propsJSON(before.mask)) patch.mask = after.mask;
    this.push({ label: label || this.patchLabel(l, patch), undo: () => applyProps(l, before), redo: () => applyProps(l, after) });
    this.log({ op: 'set', i: this.doc.layers.indexOf(l), patch });
    this.changed('props');
  }

  // ---- masks: a list of components ----
  activeComp(l) { l = this.layer(l); if (!l || !l.mask) return null; return l.mask.comps.find(c => c.id === this.compId) || l.mask.comps[l.mask.comps.length - 1] || null; }
  setComp(id, cid) { const l = this.layer(id); if (!l || !l.mask) return; this.active = l.id; this.target = 'mask'; this.compId = cid; this.emit('active', l); }
  addMask(id, from = 'reveal', opts = {}) {
    const l = this.layer(id); if (!l) throw new Error(`no layer '${id}'`);
    const W = this.doc.w, H = this.doc.h;
    let c;
    if (from === 'selection') { if (!this.doc.sel) throw new Error('no selection'); const r = this.doc.sel.clone(); c = makeComp('brush', Object.assign({ name: 'Selection', raster: r }, opts), W, H); }
    else if (from === 'reveal' || from === 'hide') c = makeComp('brush', Object.assign({ name: 'Brush', fill: from === 'hide' ? 0 : 255 }, opts), W, H);
    else c = makeComp(from, opts, W, H);
    if (l.mask) { this.addComp(l.id, c); return l.mask; }
    const old = l.mask, m = makeMask(W, H, 255, [c]);
    const doIt = () => { l.mask = m; this.active = l.id; this.target = 'mask'; this.compId = c.id; };
    doIt();
    this.push({ label: `${l.name}: mask from ${from === 'reveal' || from === 'hide' ? 'brush' : from}`, bytes: c.raster ? c.raster.data.length : 0, undo: () => { l.mask = old; this.target = 'pixels'; }, redo: doIt });
    this.log({ op: 'mask', i: this.doc.layers.indexOf(l), from, opts });
    this.changed('mask');
    return m;
  }
  // Add one component to a layer's mask (making the mask if there is none), as one undo step.
  addComp(id, c, label) {
    const l = this.layer(id); if (!l) throw new Error(`no layer '${id}'`);
    if (!l.mask) { const m = this.addMask(l.id, c.type, c); return m.comps[0]; }
    this.editMask(l.id, mk => mk.comps.push(c), label || `${l.name}: ${c.op === 'add' || !l.mask.comps.length ? 'add' : c.op} ${c.name.toLowerCase()}`);
    this.compId = c.id; this.target = 'mask'; this.active = l.id; this.emit('active', l);
    return c;
  }
  addMaskComp(id, type, o = {}) { return this.addComp(id, makeComp(type, o, this.doc.w, this.doc.h)); }
  // Change a mask's components or settings as one undo step; liveMask skips history until commit.
  editMask(id, fn, label) {
    const l = this.layer(id); if (!l || !l.mask) throw new Error(`${l ? l.name : id} has no mask`);
    const before = layerProps(l); fn(l.mask, l); const after = layerProps(l);
    if (propsJSON(before) === propsJSON(after)) return;
    this.push({ label: label || `${l.name}: mask`, undo: () => applyProps(l, before), redo: () => applyProps(l, after) });
    this.log({ op: 'set', i: this.doc.layers.indexOf(l), patch: { mask: after.mask } });
    this.changed('mask');
  }
  liveMask(id, fn) { const l = this.layer(id); if (!l || !l.mask) return; if (!l._before) l._before = layerProps(l); fn(l.mask, l); this.changed('mask', true); }
  comp(id, cid) { const l = this.layer(id); const c = l && l.mask && l.mask.comps.find(c => c.id === cid || c.name === cid); if (!c) throw new Error(`no mask component '${cid}' on ${l ? l.name : id}`); return c; }
  editComp(id, cid, patch, label) {
    const c = this.comp(id, cid), K = COMP_KINDS[c.type];
    if (patch.op && !COMP_OPS.includes(patch.op)) throw new Error(`op is one of ${COMP_OPS.join(', ')}`);
    if (patch.params) for (const [k, v] of Object.entries(patch.params)) { const sp = K.params[k]; if (!sp) throw new Error(`${c.type} has no param '${k}'; its params are ${Object.keys(K.params).join(', ')}`); if (!sp.type && !Number.isFinite(v)) throw new Error(`${k} takes a number`); }
    this.editMask(id, () => { for (const k of ['name', 'op', 'invert', 'on', 'amount']) if (k in patch) c[k] = k === 'amount' ? clamp(+patch[k], 0, 1) : patch[k]; if (patch.params) for (const [k, v] of Object.entries(patch.params)) { const sp = K.params[k]; c.params[k] = sp.type ? v : clamp(v, sp.min, sp.max); } }, label || `${this.layer(id).name}: ${c.name.toLowerCase()}`);
    return c;
  }
  removeComp(id, cid) { const c = this.comp(id, cid); this.editMask(id, mk => mk.comps.splice(mk.comps.indexOf(c), 1), `${this.layer(id).name}: remove ${c.name.toLowerCase()}`); }
  moveComp(id, cid, to) { const c = this.comp(id, cid); this.editMask(id, mk => { const i = mk.comps.indexOf(c); mk.comps.splice(i, 1); mk.comps.splice(clamp(to, 0, mk.comps.length), 0, c); }, `${this.layer(id).name}: reorder mask`); }
  deleteMask(id) {
    const l = this.layer(id); if (!l || !l.mask) return;
    const old = l.mask; const doIt = () => { l.mask = null; this.target = 'pixels'; };
    doIt(); this.push({ label: `${l.name}: delete mask`, undo: () => { l.mask = old; }, redo: doIt }); this.changed('mask');
  }
  // Rewrite a whole one-channel raster (a brush component or the selection) as one undoable step.
  rewrite(r, label, fn) {
    const before = new Uint8ClampedArray(r.data); fn(r.data); const after = new Uint8ClampedArray(r.data); r.bumpAll();
    this.push({ label, bytes: before.length * 2, undo: () => { r.data.set(before); r.bumpAll(); }, redo: () => { r.data.set(after); r.bumpAll(); } });
    this.changed('mask');
  }
  maskGradient(id, p0, p1, a = 255, b = 0, op = 'add') { return this.addMaskComp(id, 'linear', { op, invert: a < b, params: { x0: p0[0], y0: p0[1], x1: p1[0], y1: p1[1] } }); }
  maskLuminance(id, lo = 0.5, hi = 1, soft = 0.1, op = 'add') { return this.addMaskComp(id, 'luminance', { op, params: { lo, hi, soft } }); }
  // The brush component painting goes into: the active one, or a new one (subtracting, for the eraser).
  brushComp(l, erase) {
    let c = this.activeComp(l);
    if (c && c.raster) return { c, fresh: false };
    c = makeComp('brush', { name: erase ? 'Brush (erase)' : 'Brush', op: erase ? 'subtract' : 'add' }, this.doc.w, this.doc.h);
    this.addComp(l.id, c); return { c, fresh: true };
  }

  // ---- selection ----
  setSelection(data, label, mode = 'new', feather = 0) {
    const w = this.doc.w, h = this.doc.h, old = this.doc.sel;
    if (data && feather > 0) { const f = new Float32Array(data.length); for (let i = 0; i < f.length; i++) f[i] = data[i] / 255; gaussBlur(f, w, h, 1, feather / 2); data = new Uint8ClampedArray(f.length); for (let i = 0; i < f.length; i++) data[i] = f[i] * 255; label += ` · feather ${feather} px`; }
    let next = null;
    if (data) {
      next = new Raster(w, h, 1);
      const cur = old ? old.data : null, d = next.data;
      for (let i = 0; i < d.length; i++) {
        const a = cur ? cur[i] : 0, b = data[i];
        d[i] = mode === 'add' ? Math.max(a, b) : mode === 'subtract' ? Math.min(a, 255 - b) : mode === 'intersect' ? Math.min(a, b) : b;
      }
      let any = false; for (let i = 0; i < d.length; i += 7) if (d[i]) { any = true; break; }
      if (!any) next = null;
    }
    const doIt = () => { this.doc.sel = next; this.emit('sel'); };
    doIt();
    this.push({ label, bytes: next ? next.data.length : 0, undo: () => { this.doc.sel = old; this.emit('sel'); }, redo: doIt });
    this.log({ op: 'select', label });
    this.emit('doc', { what: 'sel' });
  }
  selectRect(x, y, w, h, mode, ellipse = false, feather = 0) {
    const W = this.doc.w, H = this.doc.h, d = new Uint8ClampedArray(W * H);
    const x0 = clamp(Math.round(Math.min(x, x + w)), 0, W), x1 = clamp(Math.round(Math.max(x, x + w)), 0, W), y0 = clamp(Math.round(Math.min(y, y + h)), 0, H), y1 = clamp(Math.round(Math.max(y, y + h)), 0, H);
    const cx = (x0 + x1) / 2, cy = (y0 + y1) / 2, rx = (x1 - x0) / 2 || 1, ry = (y1 - y0) / 2 || 1;
    for (let yy = y0; yy < y1; yy++) for (let xx = x0; xx < x1; xx++) {
      if (ellipse) { const u = (xx + 0.5 - cx) / rx, v = (yy + 0.5 - cy) / ry; if (u * u + v * v > 1) continue; }
      d[yy * W + xx] = 255;
    }
    this.setSelection(d, ellipse ? 'Ellipse selection' : 'Rectangle selection', mode, feather);
  }
  selectPolygon(pts, mode, feather = 0) { this.setSelection(polyFill(this.doc.w, this.doc.h, pts), 'Lasso selection', mode, feather); }
  selectWand(x, y, tol = 32, contiguous = true, mode, sample = 'composite', feather = 0) {
    x = clamp(Math.round(x), 0, this.doc.w - 1); y = clamp(Math.round(y), 0, this.doc.h - 1);
    const img = sample === 'layer' && this.activeLayer() && this.activeLayer().raster ? this.activeLayer().raster.data : this.compositeU8();
    this.setSelection(floodSelect(img, this.doc.w, this.doc.h, x, y, tol, contiguous), 'Magic wand', mode, feather);
  }
  selectAll() { this.setSelection(new Uint8ClampedArray(this.doc.w * this.doc.h).fill(255), 'Select all'); }
  selectNone() { if (this.doc.sel) this.setSelection(null, 'Deselect'); }
  selectInvert() { const s = this.doc.sel; const d = new Uint8ClampedArray(this.doc.w * this.doc.h).fill(255); if (s) for (let i = 0; i < d.length; i++) d[i] = 255 - s.data[i]; this.setSelection(d, 'Invert selection'); }
  selectGrow(px) {
    const s = this.doc.sel; if (!s) return;
    const f = new Float32Array(s.data.length); for (let i = 0; i < f.length; i++) f[i] = s.data[i] / 255;
    gaussBlur(f, s.w, s.h, 1, Math.abs(px) / 2);
    const d = new Uint8ClampedArray(f.length), t = px > 0 ? 0.08 : 0.92; for (let i = 0; i < f.length; i++) d[i] = f[i] > t ? 255 : 0;
    this.setSelection(d, px > 0 ? `Expand selection ${px} px` : `Contract selection ${-px} px`);
  }
  selectFeather(px) {
    const s = this.doc.sel; if (!s || px <= 0) return;
    const f = new Float32Array(s.data.length); for (let i = 0; i < f.length; i++) f[i] = s.data[i] / 255;
    gaussBlur(f, s.w, s.h, 1, px / 2);
    const d = new Uint8ClampedArray(f.length); for (let i = 0; i < f.length; i++) d[i] = f[i] * 255;
    this.setSelection(d, `Feather selection ${px} px`);
  }
  selectionInfo() {
    const s = this.doc.sel; if (!s) return null;
    let x0 = s.w, y0 = s.h, x1 = -1, y1 = -1, n = 0;
    for (let y = 0; y < s.h; y++) for (let x = 0; x < s.w; x++) if (s.data[y * s.w + x] >= 128) { n++; if (x < x0) x0 = x; if (x > x1) x1 = x; if (y < y0) y0 = y; if (y > y1) y1 = y; }
    return n ? { x: x0, y: y0, w: x1 - x0 + 1, h: y1 - y0 + 1, pixels: n } : null;
  }

  // ---- painting ----
  strokeBegin(target, erase = false) {
    const l = this.activeLayer(); if (!l) throw new Error('no layer to paint on');
    let r = l.raster, eraseInvert = false;
    if (target === 'mask') {
      if (!l.mask) throw new Error(`${l.name} has no mask; add one from Layers ▸ Mask`);
      const { c, fresh } = this.brushComp(l, erase); r = c.raster; eraseInvert = fresh && erase;
    }
    if (!r) throw new Error(`${l.name} has no pixels to paint (paint on a paint or image layer, or on a mask)`);
    this.stroke = { l, r, target, before: new Map(), pts: [], last: null, eraseInvert };
    return this.stroke;
  }
  dab(x, y, b) {
    const st = this.stroke; if (!st) return;
    const r = st.r, R = b.size / 2, ox = st.target === 'mask' ? 0 : st.l.ox, oy = st.target === 'mask' ? 0 : st.l.oy;
    const pts = [];
    if (st.last) { const dx = x - st.last[0], dy = y - st.last[1], d = Math.hypot(dx, dy), step = Math.max(1, b.size * 0.15); if (d < step) return; const n = Math.floor(d / step); for (let i = 1; i <= n; i++) pts.push([st.last[0] + dx * i / n, st.last[1] + dy * i / n]); }
    else pts.push([x, y]);
    st.last = pts[pts.length - 1];
    const sel = this.doc.sel, col = hexToRgb(b.color).map(v => v * 255);
    for (const [px, py] of pts) {
      st.pts.push([Math.round(px), Math.round(py)]);
      const cx = px - ox, cy = py - oy, x0 = clamp(Math.floor(cx - R), 0, r.w), x1 = clamp(Math.ceil(cx + R), 0, r.w), y0 = clamp(Math.floor(cy - R), 0, r.h), y1 = clamp(Math.ceil(cy + R), 0, r.h);
      if (x0 >= x1 || y0 >= y1) continue;
      for (let ty = Math.floor(y0 / 64); ty * 64 < y1; ty++) for (let tx = Math.floor(x0 / 64); tx * 64 < x1; tx++) {
        const k = ty * r.ntx + tx; if (st.before.has(k)) continue;
        const tw = Math.min(64, r.w - tx * 64), th = Math.min(64, r.h - ty * 64); st.before.set(k, r.copyRect(tx * 64, ty * 64, tw, th));
      }
      const d = r.data, hard = clamp(b.hardness, 0, 0.99);
      for (let yy = y0; yy < y1; yy++) for (let xx = x0; xx < x1; xx++) {
        const dist = Math.hypot(xx + 0.5 - cx, yy + 0.5 - cy) / R; if (dist > 1) continue;
        let a = (dist < hard ? 1 : 1 - smooth(hard, 1, dist)) * b.flow;
        if (sel) a *= sel.data[(yy + oy) * sel.w + xx + ox] / 255 || 0;
        if (a <= 0) continue;
        if (r.ch === 1) { const i = yy * r.w + xx; d[i] = b.erase && !st.eraseInvert ? d[i] * (1 - a) : d[i] + (255 - d[i]) * a; continue; }
        const i = (yy * r.w + xx) * 4, da = d[i + 3] / 255;
        if (b.erase) { d[i + 3] = d[i + 3] * (1 - a); continue; }
        const oa = a + da * (1 - a);
        d[i] = (col[0] * a + d[i] * da * (1 - a)) / oa; d[i + 1] = (col[1] * a + d[i + 1] * da * (1 - a)) / oa; d[i + 2] = (col[2] * a + d[i + 2] * da * (1 - a)) / oa; d[i + 3] = oa * 255;
      }
      r.bump(x0, y0, x1, y1);
    }
    this.changed('pixels', true);
  }
  strokeEnd(label, brush) {
    const st = this.stroke; this.stroke = null; if (!st || !st.before.size) return;
    const r = st.r, after = new Map();
    for (const k of st.before.keys()) { const tx = k % r.ntx, ty = (k / r.ntx) | 0; after.set(k, r.copyRect(tx * 64, ty * 64, Math.min(64, r.w - tx * 64), Math.min(64, r.h - ty * 64))); }
    const put = m => { for (const [k, buf] of m) { const tx = k % r.ntx, ty = (k / r.ntx) | 0; r.putRect(tx * 64, ty * 64, Math.min(64, r.w - tx * 64), Math.min(64, r.h - ty * 64), buf); } };
    let bytes = 0; for (const b of st.before.values()) bytes += b.length * 2;
    this.push({ label: label || (st.target === 'mask' ? `${st.l.name}: paint mask` : `Paint on ${st.l.name}`), detail: `${st.before.size} tiles`, bytes, undo: () => put(st.before), redo: () => put(after) });
    this.log({ op: 'stroke', i: this.doc.layers.indexOf(st.l), target: st.target, brush, pts: st.pts });
    this.changed('pixels');
  }
  fillSelection(color) {
    const l = this.activeLayer(), s = this.doc.sel;
    if (!s) throw new Error('make a selection first');
    if (this.target === 'mask' && l.mask) { const { c } = this.brushComp(l, false); this.rewrite(c.raster, `${l.name}: fill ${c.name.toLowerCase()}`, d => { for (let i = 0; i < d.length; i++) d[i] += (255 - d[i]) * s.data[i] / 255; }); return; }
    if (!l || !l.raster) throw new Error('fill needs a paint or image layer');
    const c = hexToRgb(color).map(v => v * 255);
    this.rewriteRGBA(l.raster, `Fill selection on ${l.name}`, d => { for (let p = 0, i = 0; p < s.data.length; p++, i += 4) { const a = s.data[p] / 255; if (!a) continue; const da = d[i + 3] / 255, oa = a + da * (1 - a); for (let k = 0; k < 3; k++) d[i + k] = (c[k] * a + d[i + k] * da * (1 - a)) / oa; d[i + 3] = oa * 255; } });
  }
  clearSelection() {
    const l = this.activeLayer(), s = this.doc.sel; if (!s || !l) return;
    if (this.target === 'mask' && l.mask) { const c = this.activeComp(l); if (c && c.raster) return this.rewrite(c.raster, `${l.name}: clear in ${c.name.toLowerCase()}`, d => { for (let i = 0; i < d.length; i++) d[i] *= 1 - s.data[i] / 255; }); return this.addMask(l.id, 'selection', { op: 'subtract', name: 'Cleared selection' }); }
    if (!l.raster) throw new Error('nothing to clear on an adjustment layer; add a mask and clear that');
    this.rewriteRGBA(l.raster, `Clear in ${l.name}`, d => { for (let p = 0; p < s.data.length; p++) d[p * 4 + 3] *= 1 - s.data[p] / 255; });
  }
  rewriteRGBA(r, label, fn) {
    const before = new Uint8ClampedArray(r.data); fn(r.data); const after = new Uint8ClampedArray(r.data); r.bumpAll();
    this.push({ label, bytes: before.length * 2, undo: () => { r.data.set(before); r.bumpAll(); }, redo: () => { r.data.set(after); r.bumpAll(); } });
    this.changed('pixels');
  }
  layerFromSelection(name = 'From selection') {
    const s = this.doc.sel; if (!s) throw new Error('make a selection first');
    const img = this.compositeU8(), r = new Raster(this.doc.w, this.doc.h, 4);
    for (let p = 0, i = 0; p < s.data.length; p++, i += 4) { if (!s.data[p]) continue; r.data[i] = img[i]; r.data[i + 1] = img[i + 1]; r.data[i + 2] = img[i + 2]; r.data[i + 3] = img[i + 3] * s.data[p] / 255; }
    r.bumpAll();
    return this.addLayer('image', { name, raster: r });
  }
  imageLayer(img, name = 'Pasted image') {
    const W = this.doc.w, H = this.doc.h, k = Math.min(1, W / img.width, H / img.height), w = Math.round(img.width * k), h = Math.round(img.height * k);
    const r = canvasRaster(W, H, g => g.drawImage(img, Math.round((W - w) / 2), Math.round((H - h) / 2), w, h));
    return this.addLayer('image', { name, raster: r });
  }

  // ---- reading the picture ----
  composite(L = 0) { const { w, h } = this.engine.dims(L); return this.engine.rect(this.engine.topIndex(), L, 0, 0, w, h); }
  compositeU8(L = 0) { const f = this.composite(L), { w, h } = this.engine.dims(L); return toImageData(f, w, h).data; }
  stats(rect) {
    const f = this.composite(0), W = this.doc.w; const x0 = rect ? rect.x : 0, y0 = rect ? rect.y : 0, w = rect ? rect.w : W, h = rect ? rect.h : this.doc.h;
    const hist = new Array(16).fill(0); let n = 0, sr = 0, sg = 0, sb = 0; const ls = [];
    for (let y = y0; y < y0 + h; y += 2) for (let x = x0; x < x0 + w; x += 2) {
      const i = (y * W + x) * 4, a = f[i + 3]; if (a <= 0) continue;
      const r = f[i] / a, g = f[i + 1] / a, b = f[i + 2] / a, l = lum(r, g, b); sr += r; sg += g; sb += b; n++; hist[Math.min(15, (l * 16) | 0)]++; ls.push(l);
    }
    ls.sort((a, b) => a - b); const q = p => ls.length ? +ls[Math.floor(p * (ls.length - 1))].toFixed(3) : 0;
    return { pixels: n * 4, mean: n ? rgbToHex(sr / n, sg / n, sb / n) : null, luminance: { p05: q(0.05), p50: q(0.5), p95: q(0.95) }, histogram16: hist.map(v => +(v / (n || 1)).toFixed(3)), clipped: { black: +(ls.filter(v => v < 0.01).length / (n || 1)).toFixed(4), white: +(ls.filter(v => v > 0.99).length / (n || 1)).toFixed(4) } };
  }
  sample(x, y) {
    x = clamp(Math.round(x), 0, this.doc.w - 1); y = clamp(Math.round(y), 0, this.doc.h - 1);
    const T = this.engine.TS, t = this.engine.tile(this.engine.topIndex(), 0, Math.floor(x / T), Math.floor(y / T)), i = ((y % T) * t.w + (x % T)) * 4, a = t.buf[i + 3];
    return a > 0 ? { hex: rgbToHex(t.buf[i] / a, t.buf[i + 1] / a, t.buf[i + 2] / a), alpha: +a.toFixed(3) } : { hex: null, alpha: 0 };
  }
}

// A deep copy of a document (pixels included), for the bench and for preset previews.
function cloneDoc(doc) {
  return { code: doc.code, title: doc.title, w: doc.w, h: doc.h, sel: doc.sel ? doc.sel.clone() : null,
    layers: doc.layers.map(l => { const c = Object.assign({}, l, { params: JSON.parse(JSON.stringify(l.params)), raster: l.raster ? l.raster.clone() : null, mask: l.mask ? cloneMask(l.mask) : null }); if (l.meta) c.meta = JSON.parse(JSON.stringify(l.meta)); return c; }) };
}
function cloneMask(m) {
  return { density: m.density, invert: m.invert, on: m.on, comps: m.comps.map(c => Object.assign(compJSON(c), { raster: c.raster ? c.raster.clone() : null })) };
}
