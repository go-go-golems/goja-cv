// ---------------------------------------------------------------------------------------------
// views/agent.js: the agent (Form Skate Agent's design). One tool, eval, runs JavaScript with the
// lab API in scope; each step shows its code, time and YAML result, and can be reverted.
// ---------------------------------------------------------------------------------------------
'use strict';

// ---- the lab API: what the agent's code can reach -------------------------------------------------
const compInfo = c => ({ id: c.id, type: c.type, name: c.name, op: c.op, invert: c.invert, on: c.on, amount: c.amount, params: c.params });
function labScope(readOnly, source) {
  const ws = app.ws;
  // Layers made through this scope remember who made them: the agent's step, or a script's file and line.
  const stamp = (o = {}) => { if (!source) return o; const src = Object.assign({}, source); if (source.kind === 'script') { const ln = labLine(new Error()); if (ln) src.line = ln; } return Object.assign({}, o, { meta: Object.assign({}, o.meta || {}, { source: Provenance.stamp(src.kind, src) }) }); };
  const L = id => { const l = ws.layer(id); if (!l) throw new Error(`no layer '${id}'; layers are ${ws.doc.layers.map(l => l.id + ' ' + l.name).join(', ')}`); return l; };
  const info = l => { const o = { id: l.id, name: l.name, kind: l.kind, visible: l.visible, opacity: +l.opacity.toFixed(3), blend: l.blend, params: l.params };
    if (l.parent) o.parent = l.parent; if (KINDS[l.kind].isGroup) { o.children = ws.kids(l.id).map(k => k.id); o.open = l.open !== false; delete o.params; }
    if (l.meta) o.meta = l.meta; const src = Provenance.of(l); if (src) o.source = Provenance.label(src);
    if (l.raster) o.offset = [l.ox, l.oy]; o.mask = l.mask ? { density: l.mask.density, invert: l.mask.invert, on: l.mask.on, components: l.mask.comps.map(compInfo) } : null;
    const ms = (Render.layerMs || new Map()).get(l.id); if (ms != null) o.ms_last_update = +ms.toFixed(2); return o; };
  const timeFull = () => { const eng = ws.engine, c0 = eng.computed, h0 = eng.cache.hits, m0 = eng.cache.misses, t0 = performance.now(); ws.composite(0); return { ms: +(performance.now() - t0).toFixed(2), tiles_computed: eng.computed - c0, hits: eng.cache.hits - h0, misses: eng.cache.misses - m0, cache_MB: +(eng.cache.bytes / 1048576).toFixed(1) }; };
  const scope = {
    doc: () => ({ code: ws.doc.code, title: ws.doc.title, width: ws.doc.w, height: ws.doc.h, layers: ws.doc.layers.length, active: ws.active, target: ws.target, selection: ws.selectionInfo() }),
    layers: () => ws.doc.layers.map((l, i) => Object.assign({ index: i, depth: ws.depth(l) }, info(l))),
    layer: id => info(L(id)),
    kinds: () => Object.fromEntries(Object.entries(KINDS).map(([k, K]) => [k, { group: K.group, params: Object.fromEntries(Object.entries(K.params).map(([p, s]) => [p, s.type === 'color' ? `colour #rrggbb, default ${s.def}` : s.type === 'choice' ? `one of ${s.options.join(' | ')}, default ${s.def}` : `${s.min} to ${s.max}${s.unit ? ' ' + s.unit : ''}${s.pct ? ' (a fraction; shown as %)' : ''}, default ${s.def}`])) }])),
    blendModes: () => BLEND_MODES,
    add: (kind, opts = {}) => { const { at, parent, ...o } = opts; if (parent != null) o.parent = L(parent).id; return info(ws.addLayer(kind, stamp(o), at)); },
    group: (ids, opts = {}) => info(ws.group([].concat(ids).map(x => L(x).id), stamp(opts))),
    ungroup: id => ws.ungroup(L(id).id).map(l => l.id),
    moveTo: (id, target, where = 'above') => { ws.moveTo(L(id).id, L(target).id, where); return scope.layers().map(l => '  '.repeat(l.depth) + l.index + ' ' + l.name); },
    meta: (id, patch) => patch === undefined ? (L(id).meta || {}) : ws.setMeta(L(id).id, patch),
    source: id => { const l = L(id), s = Provenance.of(l); return s ? Object.assign({ label: Provenance.label(s) }, s) : null; },
    reveal: (id, where) => Provenance.reveal(L(id), where) ? `showing where ${L(id).name} came from` : `${L(id).name} has no source to show`,
    madeBy: q => Provenance.madeBy(s => typeof q === 'object' && q ? (q.kind == null || s.kind === q.kind) && (q.preset == null || s.id === q.preset || s.name === q.preset || s.preset === q.preset) && (q.file == null || s.file === q.file || s.fileId === q.file)
      : s.id === q || s.file === q || s.fileId === q || s.preset === q || s.name === q).map(l => l.id),
    set: (id, patch) => info(ws.setLayer(L(id).id, patch)),
    remove: id => { const l = L(id); ws.removeLayer(l.id); return `removed ${l.id} ${l.name}`; },
    move: (id, index) => { ws.moveLayer(L(id).id, index); return scope.layers().map(l => l.index + ' ' + l.name); },
    activate: (id, target = 'pixels') => { ws.setActive(L(id).id, target); return { active: ws.active, target: ws.target }; },
    select: {
      rect: (x, y, w, h, mode = 'new', o = {}) => (ws.selectRect(x, y, w, h, mode, false, o.feather || 0), ws.selectionInfo()),
      ellipse: (x, y, w, h, mode = 'new', o = {}) => (ws.selectRect(x, y, w, h, mode, true, o.feather || 0), ws.selectionInfo()),
      polygon: (points, mode = 'new', o = {}) => (ws.selectPolygon(points, mode, o.feather || 0), ws.selectionInfo()),
      wand: (x, y, o = {}) => (ws.selectWand(x, y, o.tolerance ?? 32, o.contiguous ?? true, o.mode || 'new', o.sample || 'composite', o.feather || 0), ws.selectionInfo()),
      all: () => (ws.selectAll(), ws.selectionInfo()), none: () => (ws.selectNone(), null), invert: () => (ws.selectInvert(), ws.selectionInfo()),
      grow: px => (ws.selectGrow(px), ws.selectionInfo()), feather: px => (ws.selectFeather(px), ws.selectionInfo()), info: () => ws.selectionInfo(),
    },
    mask: {
      add: (id, from = 'reveal', o = {}) => (ws.addMask(L(id).id, from, o), info(L(id))),
      component: (id, spec = {}) => { const { type, ...o } = spec; return compInfo(ws.addMaskComp(L(id).id, type, o)); },
      components: id => (L(id).mask ? L(id).mask.comps.map(compInfo) : []),
      edit: (id, compId, patch) => compInfo(ws.editComp(L(id).id, compId, patch)),
      removeComponent: (id, compId) => (ws.removeComp(L(id).id, compId), scope.mask.components(id)),
      move: (id, compId, index) => (ws.moveComp(L(id).id, compId, index), scope.mask.components(id)),
      gradient: (id, from, to, op = 'add') => compInfo(ws.maskGradient(L(id).id, from, to, 255, 0, op)),
      radial: (id, o = {}) => { const { op, invert, name, amount, ...params } = o; return compInfo(ws.addMaskComp(L(id).id, 'radial', { op, invert, name, amount, params })); },
      luminance: (id, o = {}) => compInfo(ws.maskLuminance(L(id).id, o.min ?? 0.5, o.max ?? 1, o.soft ?? 0.1, o.op || 'add')),
      color: (id, o = {}) => compInfo(ws.addMaskComp(L(id).id, 'color', { op: o.op, invert: o.invert, params: { color: o.color || '#808080', tol: o.tol ?? 0.15, soft: o.soft ?? 0.1 } })),
      set: (id, m) => info(ws.setLayer(L(id).id, { mask: m })),
      remove: id => (ws.deleteMask(L(id).id), info(L(id))),
    },
    paint: {
      stroke: (id, points, b = {}) => { const l = L(id), target = b.target || 'pixels'; ws.setActive(l.id, target); const br = Object.assign({ size: 30, hardness: 0.6, flow: 0.8, color: '#ffffff', erase: false }, b); ws.strokeBegin(target); for (const [x, y] of points) ws.dab(x, y, br); ws.strokeEnd(null, br); return { layer: l.id, target, points: points.length }; },
      fill: color => (ws.fillSelection(color), 'filled'), clear: () => (ws.clearSelection(), 'cleared'),
      layerFromSelection: name => info(ws.layerFromSelection(name)),
    },
    stats: rect => ws.stats(rect), sample: (x, y) => ws.sample(x, y), render: timeFull,
    presets: {
      list: () => Presets.all().map(p => ({ id: p.id, name: p.name, about: p.about, builtin: !!p.builtin, layers: p.layers.map(l => l.kind + (l.blend !== 'normal' ? ' ' + l.blend : '')).join(', ') })),
      get: id => { const p = Presets.get(id); if (!p) throw new Error(`no preset '${id}'`); return p; },
      save: p => { const s = Presets.save(p); return { id: s.id, name: s.name, layers: s.layers.length }; },
      fromLayers: (ids, name) => { const p = Presets.fromLayers(ids.map(i => L(i).id), name); return p ? { id: p.id, name: p.name } : null; },
      apply: id => Presets.apply(id, source ? stamp().meta.source : undefined), remove: id => Presets.remove(id),
      put: (name, p = {}) => { const ex = Presets.user.find(x => x.name === name); const s = Presets.save({ id: ex && ex.id, name, about: p.about, layers: p.layers }); return { id: s.id, name: s.name, layers: s.layers.length, updated: !!ex }; },
      select: id => { Presets.select(id); return Presets.selectedId; },
    },
    M, lin, rad, lumR, colR, sleep,
    code: { open: (name, text) => (Code.openScratch(String(name || 'untitled.js'), String(text || '')), `opened ${name} in the Code view`), files: () => Code.files.map(f => ({ id: f.id, name: f.name, lines: f.code.split('\n').length })) },
    bench: {
      scenarios: () => Bench.scenarios().map(s => ({ id: s.id, name: s.name, mode: s.mode, about: s.about })),
      profiles: () => PROFILES.map(p => ({ id: p.id, name: p.name, about: p.about })),
      run: async (o = {}) => { const r = await Bench.run(o); Dock.focus('Bench'); return { run: r.n, rows: Bench.summary(r) }; },
      runs: () => Bench.runs.map(r => ({ run: r.n, at: r.at, doc: r.doc, rows: r.rows.length })),
      summary: n => Bench.summary(Exports.run(n)),
      csv: n => Exports.benchCSV(Exports.run(n)),
    },
    exports: {
      benchCSV: n => { const r = Exports.run(n); return Exports.save(`bench-run-${r.n}-${Exports.stamp()}.csv`, Exports.benchCSV(r), 'the CSV'); },
      benchJSON: n => { const r = Exports.run(n); return Exports.save(`bench-run-${r.n}-${Exports.stamp()}.json`, Exports.benchJSON(r), 'the JSON'); },
      benchReport: n => { const r = Exports.run(n); return Exports.save(`bench-run-${r.n}-${Exports.stamp()}.md`, Exports.benchReport(r), 'the report'); },
      picture: (format = 'png', level = 0) => Exports.picture(format, level),
      presets: (ids = 'mine') => { const list = ids === 'all' ? Presets.all() : ids === 'mine' ? Presets.user : ids.map(id => scope.presets.get(id)); return Exports.save(`presets-${Exports.stamp()}.json`, Exports.presetsJSON(list), 'the presets'); },
      probes: () => Exports.save(`probes-${app.ws.doc.code}-${Exports.stamp()}.json`, Exports.probesJSON(), 'the probe reports'),
      text: (filename, text) => { if (!/\.(md|txt|csv|json|html|svg)$/i.test(filename)) throw new Error('text exports end in .md, .txt, .csv, .json, .html or .svg'); return Exports.save(filename, String(text), filename); },
    },
    gpu: { probe: async (reps = 7) => GPU.probe(reps), plan: () => { const p = GPU.plan(app.ws.doc); return { base: p.base && p.base.name, fused: p.fused.map(l => l.name), skipped: p.skipped }; }, shader: () => GPU.shader(GPU.plan(app.ws.doc)), blur: async (r = 8, reps = 7) => GPU.blurProbe(r, reps), blurShader: () => BLUR_WGSL },
    wasm: { env: () => WASM.env(), probe: async (r = 8, reps = 9) => WASM.probe(r, reps) },
    engine: () => ({ tile: app.settings.tile, cache_budget_MB: app.settings.budgetMB, preview: app.settings.preview, last_update: Render.stats }),
    history: { list: () => ws.hist.stack.map((e, i) => (i < ws.hist.ptr ? '' : '(undone) ') + e.label), undo: () => ws.undo(), redo: () => ws.redo() },
    view: name => (Dock.focus(name), `showing ${name}`), tool: name => (setTool(name), app.tool),
    overlay: (name, on = true) => { app.settings.overlays[name] = on; app.saveSettings(); Stage.dirty = true; return app.settings.overlays; },
    book: { contents: () => BOOK.map(c => ({ n: c.n, title: c.title })), chapter: n => (BOOK.find(c => c.n === +n) || {}).md || null, open: n => (Book.open(n), `opened chapter ${n}`) },
  };
  if (readOnly) {
    const off = name => () => { throw new Error(`read-only mode: ${name} is off`); };
    for (const k of ['add', 'set', 'remove', 'move', 'group', 'ungroup', 'moveTo']) scope[k] = off(k + '()');
    { const m = scope.meta; scope.meta = (id, patch) => { if (patch !== undefined) throw new Error('read-only mode: meta(id, patch) is off'); return m(id); }; }
    for (const k of Object.keys(scope.select)) if (k !== 'info') scope.select[k] = off('select.' + k);
    for (const k of Object.keys(scope.mask)) if (k !== 'components') scope.mask[k] = off('mask.' + k);
    for (const k of Object.keys(scope.paint)) scope.paint[k] = off('paint.' + k);
    for (const k of ['save', 'put', 'fromLayers', 'apply', 'remove']) scope.presets[k] = off('presets.' + k);
    scope.history.undo = off('history.undo'); scope.history.redo = off('history.redo');
  }
  return scope;
}
const LAB_REFERENCE = `## The lab API (in scope in eval)
Coordinates are picture pixels, origin top left. Layer ids look like L4; a layer name works too. Index 0 is the bottom layer.
- doc() · layers() · layer(id) · kinds() (every layer kind with its params, ranges and units) · blendModes()
- add(kind, { name, params, opacity, blend, visible, at, parent }) → the new layer (parent: a group's id puts it at the top of that group) · set(id, { name, visible, opacity, blend, params: {...}, mask: { density, feather, invert, on }, ox, oy }) · remove(id) · move(id, index) · activate(id, 'pixels' | 'mask')
- Groups: a group is a layer of kind 'group' whose children list it as parent; layers() gives each layer's depth and parent, and a group's children. group(ids, { name, blend, opacity }) wraps layers in a new group · ungroup(id) · moveTo(id, targetId, 'above' | 'below' | 'inside'). A group's blend 'pass' (pass through, the default) applies its children straight onto what is below; any other blend mode isolates it: the children composite on transparent, and the result is blended in. Removing, duplicating or hiding a group does the same to everything inside it; a group can have opacity and a mask, but no params.
- Where a layer came from: every layer you add is stamped with meta.source (this eval: thread, turn and step); a preset applied with more than one layer arrives as a group named after it. meta(id) reads meta, meta(id, { note }) merges into it (null deletes a key) · source(id) → { kind: 'preset' | 'program' | 'script' | 'agent' | 'codeLayer', ... , label } · reveal(id, 'preset' | 'code') opens what made it (the preset, or the file at the line) · madeBy(q) → ids of the layers on the stage from that source; q is a preset id or name, a file name, or { preset, file, kind }
- select.rect(x, y, w, h, mode, { feather }) · select.ellipse(...) · select.polygon([[x, y], ...], mode, { feather }) · select.wand(x, y, { tolerance 0–255, contiguous, mode, sample: 'composite' | 'layer', feather }) · select.all() · select.none() · select.invert() · select.grow(px) · select.feather(px) · select.info(); mode is new, add, subtract or intersect
- A mask is an ordered list of components, combined in order: the first is the base; then add (union, max), subtract (v·(1 − c)) or intersect (v·c). Each component has op, invert, on, amount (0–1) and params:
  linear { x0, y0, x1, y1 } (fractions of the sheet: full effect at the start line, none past the end line) · radial { cx, cy, rx, ry (fractions of width and height), angle (°), feather (0–1) } · brush { feather (px) } (painted pixels) · luminance { lo, hi, soft } and color { color, tol, soft } (read what is below the layer)
- mask.add(id, 'reveal' | 'hide' | 'selection' | 'linear' | 'radial' | 'luminance' | 'color', { op, invert, params }) · mask.component(id, { type, op, invert, amount, name, params }) · mask.components(id) · mask.edit(id, componentId, { op, invert, on, amount, name, params }) · mask.removeComponent(id, componentId) · mask.move(id, componentId, index)
- shorthands: mask.gradient(id, [fx, fy], [fx, fy], op) · mask.radial(id, { cx, cy, rx, ry, angle, feather, op, invert }) · mask.luminance(id, { min, max, soft, op }) · mask.color(id, { color, tol, soft, op }) · mask.set(id, { density, invert, on }) · mask.remove(id)
- paint.stroke(id, [[x, y], ...], { size, hardness, flow, color, erase, target: 'pixels' | 'mask' }) · paint.fill(color) and paint.clear() act on the selection · paint.layerFromSelection(name)
- stats(rect?) → mean colour, luminance percentiles, a 16-bin histogram, clipped fractions · sample(x, y) · render() → a timed full render: ms, tiles computed, hits
- presets.list() · presets.get(id) · presets.save({ id?, name, about, layers: [{ kind, name, params, opacity, blend, mask }] }) (no id makes a new one; built-in presets cannot be changed) · presets.put(name, { about, layers }) (saves, or updates the one of yours with that name) · presets.select(id) · presets.fromLayers(ids, name) · presets.apply(id) · presets.remove(id)
- A preset layer's mask is M(...components), built with lin(x0, y0, x1, y1, o?) · rad(cx, cy, rx, ry, feather, o?) · lumR(lo, hi, soft?, o?) · colR('#rrggbb', tol, soft?, o?); positions and radii are fractions of the width and height; o may hold op ('add' | 'subtract' | 'intersect'), invert, amount, name, and for rad an angle in degrees
- code.open(name, text) (puts a script or a program in the Code view for the person to read, edit and run) · code.files() · sleep(ms)
- Code layers: a layer of kind 'code' runs JavaScript. Its program is a file of preset(name, { about }) and layer(name, { params: { key: [min, max, value, label?, unit?] }, fn(b, n, p, ctx) | roiFn(buf, w, h, p, L) with reach(p, L) | paint(src, n, p, ctx), mask?, blend?, opacity? }) calls, the same signatures as the built-in kinds. To write one for the person, put it in the Code view with code.open; running it there saves the preset and puts its layers on the stage. Every built-in preset can be opened as such a program from Presets › Edit as code.
- bench.scenarios() · bench.profiles() · await bench.run({ scenarios, profiles, tiles: [64, 128, 256], previews: ['on', 'off'], evictions: ['lru', 'cost'], repeats }) → rows with update p50/p95, settle, tiles computed, hit rate, peak MB, gate
- bench.runs() · bench.summary(n) · bench.csv(n) (the CSV as text)
- exports.benchCSV(n?) · exports.benchJSON(n?) · exports.benchReport(n?) (Markdown) · exports.picture('png' | 'jpg', level) · exports.presets('mine' | 'all' | [ids]) · exports.probes() (the last GPU, GPU blur and WASM probe reports as JSON) · exports.text(filename, text): each asks the person to confirm the save, and resolves { status: 'saved' | 'declined' | 'copied' | 'unavailable' }; n defaults to the run shown in the Bench
- gpu.plan() (which layers fuse into one WebGPU shader, and why others do not) · await gpu.probe(reps) (GPU against CPU on those layers: compile, upload, kernel, readback times, largest difference) · gpu.shader() (the generated WGSL) · await gpu.blur(r, reps) (one box pass in workgroup memory against plain reads and the CPU) · gpu.blurShader()
- wasm.env() (SIMD, threads, cross-origin isolation, cores) · await wasm.probe(r, reps) (box pass in JS, scalar WASM and SIMD WASM; a gain in JS and SIMD; copy costs)
- engine() · history.list() · history.undo() · history.redo() · view(name) · tool(name) · overlay(name, on) (grid, dims, tiles, mask, furniture) · book.contents() · book.chapter(n) · book.open(n)
Every change is one step in History, so the person can undo it.`;
const LAB_LINKS = `lab: links the person can click: [Scene](lab:scene?id=D-02) · [view](lab:view?name=Bench) · [tool](lab:tool?name=wand) · [overlay](lab:overlay?name=tiles) · [add a layer](lab:add?kind=blur&radius=6) · [select a layer](lab:select?layer=L3) · [group layers](lab:group?layers=L5,L6&blend=normal) · [preset](lab:preset?id=P-01) · [bench](lab:bench?scenario=scrub-bottom&profile=tablet&run=1) · [chapter](lab:book?ch=4) · [preview level](lab:preview?level=off) · [tile size](lab:tile?size=256)`;
function agentInstructions() {
  return `You are the agent in Editor lab, a layer-based image editor in the person's browser: a stage with tools, a layer stack with masks and blend modes, presets, a benchmark bench, and a short textbook. You work through one tool, eval, which runs the body of an async JavaScript function with the lab API below in scope and returns the value as YAML. Use return to see a value.

How to work:
- Look before changing: layers(), layer(id), stats(), doc(). When an image is attached, the first one is the current composite at reduced size; any others were pasted by the person. Describe only what you can see in them or measure with stats().
- Change with add, set, mask.* and select.*; prefer adjustment layers and masks to painting pixels, so every change stays editable. Group a look into a preset with presets.save when the person asks for one.
- After a change, measure: render() for cost, stats() for what it did to the picture, and report those numbers. Say which numbers are measured and which are modelled (bench profiles other than This browser scale measured time).
- Keep each eval small and return only the fields you need.
- In Ask mode the person approves each eval; in Read-only mode edits are refused: say what you would change instead.

How to answer: a few short paragraphs of Markdown. Name layers by name and id. Numbers with units and a space ("12.4 ms", "40 px"). British spelling, no exclamation marks, no emoji. Offer lab: links where the person should look or click.

${LAB_REFERENCE}

${LAB_LINKS}`;
}
function agentContext(withImage) {
  const ws = app.ws, s = Render.stats, eng = ws.engine, sel = ws.selectionInfo(), l = ws.activeLayer();
  const lines = ws.doc.layers.map((l, i) => `  ${i} ${l.id} ${l.name}: ${l.kind}${l.visible ? '' : ' (hidden)'}${l.blend !== 'normal' ? ' · ' + l.blend : ''}${l.opacity < 1 ? ' · ' + Math.round(l.opacity * 100) + ' %' : ''}${l.mask ? ' · masked' : ''}`);
  return `[context]
document: ${ws.doc.code} ${ws.doc.title}, ${ws.doc.w} × ${ws.doc.h} px, ${ws.doc.layers.length} layers (bottom first):
${lines.join('\n')}
active: ${l ? `${l.id} ${l.name} (${ws.target})` : 'none'}
selection: ${sel ? `${sel.w} × ${sel.h} px at ${sel.x}, ${sel.y}` : 'none'}
last update: ${fmtMs(s.fullMs)} ms to full resolution, ${s.computed} of ${s.tiles} tiles computed; cache ${fmtMB(eng.cache.bytes)} of ${app.settings.budgetMB} MB; tile ${eng.TS} px; preview ${app.settings.preview}
mode: ${{ auto: 'Auto', ask: 'Ask', read: 'Read-only' }[Agent.S.settings.mode]}
stage image: ${withImage ? 'attached as the first image' : 'not attached'}
[/context]`;
}

// ---- the panel ------------------------------------------------------------------------------------
// ---- Running code: the agent's eval and the Code view share this. new Function when the page's
// content security policy allows it, else an inline script element, else nothing.
const EXEC = { mode: null, offset: null, seq: 0 };
function execMode() {
  if (EXEC.mode) return EXEC.mode;
  try { if (new Function('return 1')() === 1) return (EXEC.mode = 'function'); } catch (e) { /* blocked */ }
  try { const s = document.createElement('script'); s.textContent = 'window.__labProbe = 1;'; document.head.append(s); s.remove(); if (window.__labProbe === 1) { delete window.__labProbe; return (EXEC.mode = 'script'); } } catch (e) { /* blocked */ }
  return (EXEC.mode = 'none');
}
function compileLab(names, code) {
  const body = '"use strict";\nreturn (async () => {\n' + code + '\n})();';
  const mode = execMode();
  if (mode === 'function') return new Function(...names, body + '\n//# sourceURL=lab-code.js');
  if (mode === 'script') {
    const id = '__labFn' + (++EXEC.seq); let err = null;
    const onErr = e => { err = e.error || new SyntaxError(e.message); e.preventDefault(); };
    addEventListener('error', onErr);
    const s = document.createElement('script'); s.textContent = `window.${id} = function (${names.join(', ')}) {\n${body}\n};\n//# sourceURL=lab-code.js`;
    try { document.head.append(s); } catch (x) { err = err || x; } s.remove(); removeEventListener('error', onErr);
    const fn = window[id]; delete window[id];
    if (err) { const x = new SyntaxError(String(err.message || err).replace(/^Failed to execute '\w+' on '\w+': /, '').replace(/^Uncaught SyntaxError: /, '')); throw x; }
    if (fn) return fn;
  }
  throw new Error('this page cannot run code: its content security policy blocks both eval and inline scripts');
}
// A code layer's program: compiled synchronously, run once with preset() and layer() to collect.
function compileModule(src) {
  const names = ['preset', 'layer', 'M', 'lin', 'rad', 'lumR', 'colR'], body = '"use strict";\n' + src;
  const mode = execMode();
  if (mode === 'function') return new Function(...names, body + '\n//# sourceURL=lab-layer.js');
  if (mode === 'script') {
    const id = '__labMod' + (++EXEC.seq); let err = null;
    const onErr = e => { err = e.error || new SyntaxError(e.message); e.preventDefault(); };
    addEventListener('error', onErr);
    const s = document.createElement('script'); s.textContent = `window.${id} = function (${names.join(', ')}) {\n${body}\n};\n//# sourceURL=lab-layer.js`;
    try { document.head.append(s); } catch (x) { err = err || x; } s.remove(); removeEventListener('error', onErr);
    const fn = window[id]; delete window[id];
    if (err) throw new SyntaxError(String(err.message || err).replace(/^Failed to execute '\w+' on '\w+': /, '').replace(/^Uncaught SyntaxError: /, ''));
    if (fn) return fn;
  }
  throw new Error('this page cannot run code: its content security policy blocks both eval and inline scripts');
}
// The line of a program a stack frame points at: one line fewer of wrapper than a script has.
function moduleLine(e) {
  const m = /lab-layer\.js:(\d+)/.exec((e && e.stack) || ''); if (!m) return null;
  const off = EXEC.mode === 'script' ? 2 : EXEC.offset != null ? EXEC.offset - 1 : null; return off != null ? +m[1] - off : null;
}
// Which line of the user's code a stack frame points at, calibrated once.
function labLine(e) {
  if (EXEC.offset == null) { try { compileLab([], 'throw new Error("probe")')().catch(x => { const m = /lab-code\.js:(\d+)/.exec(x.stack || ''); EXEC.offset = m ? +m[1] - 1 : 0; }); } catch (x) { EXEC.offset = 0; } }
  const off = EXEC.mode === 'script' ? 3 : EXEC.offset;   // our script element puts the code on its 4th line
  const m = /lab-code\.js:(\d+)/.exec((e && e.stack) || ''); return m && off != null ? +m[1] - off : null;
}
// Run code against the lab API. Returns { text, error, ms, line, value }.
async function runLab(code, { readOnly = false, timeout = 25000, maxText = 9000, source = null } = {}) {
  const scope = labScope(readOnly, source), logs = [];
  const say = (p, ...a) => logs.push(p + a.map(x => typeof x === 'string' ? x : yaml(x).replace(/\n\s*/g, ' ')).join(' '));
  scope.console = { log: (...a) => say('', ...a), warn: (...a) => say('warn: ', ...a), error: (...a) => say('error: ', ...a) }; scope.log = scope.console.log; scope.lab = Object.assign({}, scope);
  const names = Object.keys(scope).filter(k => /^[A-Za-z_$][\w$]*$/.test(k));
  const t0 = performance.now(); let value, error = null;
  try {
    const fn = compileLab(names, code);
    value = await Promise.race([fn(...names.map(k => scope[k])), new Promise((_, rej) => setTimeout(() => rej(new Error(`stopped waiting after ${Math.round(timeout / 1000)} s`)), timeout))]);
  } catch (e) { error = e; }
  const ms = performance.now() - t0, line = error ? labLine(error) : null;
  let text;
  if (error) text = `Error: ${error.message}${line ? ` (line ${line})` : ''}`;
  else text = yaml(value === undefined ? null : value);
  if (logs.length) text = 'logs:\n' + logs.join('\n') + '\n---\n' + text;
  if (text.length > maxText) text = text.slice(0, maxText) + `\n… cut (${text.length} chars). Return less: pick fields, slice arrays.`;
  return { text, error: !!error, ms, line, value };
}
labLine(null);
let CAN_EVAL = execMode() !== 'none';
const OFF = { load: 'Claude is not available in this view, so the agent cannot run. Everything else in the workbench works.', send: 'Claude is not available here, so the agent is off. Everything else in the workbench works.',
  not_granted: 'Claude is not allowed for this page. You can allow it from the artifact’s permissions.', sampling_disabled: 'Claude is not available for this account.', not_declared: 'This page no longer asks for Claude.',
  capability_disabled: 'Claude is unavailable in this view.', tools_unavailable: 'This view cannot run the agent’s tool.', capability_removed: 'This app version cannot run the agent.' };
const NOTES = { cancelled: 'Stopped.', rate_limited: 'Too many requests just now; try again in a little while.', prompt_too_large: 'The conversation is too long. Start a new thread.', refused: 'Claude declined this request.' };
const SUGGEST = ['Describe this picture and suggest three edits', 'Make the sky moodier without touching the water', 'Build a preset called Golden hour from what you see', 'Which layer costs the most to render, and why?', 'Benchmark scrubbing the lowest adjustment on the tablet profile and explain the result', 'Select the sun with the wand and give it a soft glow'];

const Agent = {
  S: null, busy: false, off: null, attachments: [], withImage: store.get('agentImage', true),
  init() {
    const t = store.get('threads', null);
    this.S = { threads: t && t.length ? t : [this.newThreadObj()], active: store.get('thread', null), settings: Object.assign({ mode: 'auto', tier: 'default', effort: '' }, store.get('agentSettings', {})) };
    if (!this.S.threads.find(x => x.id === this.S.active)) this.S.active = this.S.threads[0].id;
    this.getSample().then(s => { if (!s) this.off = OFF.load; this.render(); });
  },
  // The way back from a layer the agent made: its thread, opened at the step.
  reveal(src) {
    const th = this.S.threads.find(t => t.id === src.thread);
    if (!th) { toast('That agent thread is no longer kept here', true); Dock.focus('Agent'); return false; }
    this.S.active = th.id; const t = th.turns[src.turn], st = t && t.steps && t.steps[src.step]; if (st) st.open = true;
    Dock.focus('Agent'); this.render();
    requestAnimationFrame(() => { const el = this.log && this.log.querySelector(`[data-step="${src.turn}:${src.step}"]`); if (el) { el.scrollIntoView({ block: 'center' }); el.classList.add('flash'); setTimeout(() => el.classList.remove('flash'), 1200); } });
    return true;
  },
  newThreadObj() { return { id: 'a' + Date.now().toString(36), title: 'New thread', turns: [] }; },
  thread() { return this.S.threads.find(t => t.id === this.S.active); },
  persist() {
    const trim = th => ({ id: th.id, title: th.title, turns: th.turns.slice(-30).map(t => ({ role: t.role, text: t.text, note: t.note, imgs: t.imgs ? t.imgs.length : 0, steps: (t.steps || []).map(s => ({ code: s.code, result: s.result && s.result.length > 3000 ? s.result.slice(0, 3000) + '\n…' : s.result, ms: s.ms, error: s.error, changed: s.changed, reverted: s.reverted })) })) });
    store.set('threads', this.S.threads.map(trim)); store.set('thread', this.S.active); store.set('agentSettings', this.S.settings);
  },
  getSample() { if (!this.samp) this.samp = cap('sample'); return this.samp; },
  prefill(q) { if (this.ta) { this.ta.value = q; this.ta.focus(); } },
  attach(blob) {
    if (this.attachments.length >= 4) { toast('Up to four images per message', true); return; }
    this.attachments.push({ blob, url: URL.createObjectURL(blob) }); this.renderAttach(); Dock.focus('Agent');
  },

  create() {
    this.threadsEl = h('div', { class: 'threads' }); this.setEl = h('div', { class: 'agent-set' }); this.offEl = h('div'); this.log = h('div', { class: 'agent-log', 'aria-live': 'polite' });
    this.ta = h('textarea', { placeholder: 'Ask about the picture, change it, build a preset or run the bench. Paste images here to show them.', 'aria-label': 'Message to the agent',
      onkeydown: e => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); this.send(this.ta.value); } } });
    this.attEl = h('div', { class: 'attach' }); this.meta = h('span', { class: 'meta' }); this.sendBtn = h('button', { class: 'btn primary', onclick: () => this.busy ? this.stop() : this.send(this.ta.value) }, 'Send');
    this.imgTog = toggle('Stage image', this.withImage, v => { this.withImage = v; store.set('agentImage', v); }, 'Send the current picture with each message');
    this.composer = h('div', { class: 'composer' }, this.attEl, this.ta, h('div', { class: 'row' }, this.meta, this.imgTog, this.sendBtn));
    this.composer.addEventListener('dragover', e => { if ([...e.dataTransfer.items].some(i => i.type.startsWith('image/'))) { e.preventDefault(); this.composer.classList.add('drop'); } });
    this.composer.addEventListener('dragleave', () => this.composer.classList.remove('drop'));
    this.composer.addEventListener('drop', e => { e.preventDefault(); this.composer.classList.remove('drop'); for (const f of e.dataTransfer.files) if (f.type.startsWith('image/')) this.attach(f); });
    this.el = h('div', { class: 'tabbody col' }, h('div', { class: 'agent-panel' }, this.threadsEl, this.setEl, this.offEl, this.log, this.composer));
    this.render(); return this.el;
  },
  onShow() { this.log && (this.log.scrollTop = this.log.scrollHeight); },
  render() {
    if (!this.el) return;
    const S = this.S, th = this.thread();
    this.threadsEl.replaceChildren(...S.threads.map(t => h('span', { class: 'th' + (t.id === S.active ? ' on' : '') }, h('button', { onclick: () => { S.active = t.id; this.persist(); this.render(); }, disabled: this.busy && t.id !== S.active }, t.title),
      h('button', { class: 'x', 'aria-label': 'Close thread', disabled: this.busy, onclick: () => this.closeThread(t.id) }, '×'))), h('button', { class: 'new', disabled: this.busy, onclick: () => { const n = this.newThreadObj(); S.threads.push(n); S.active = n.id; this.persist(); this.render(); } }, '+ New'));
    const set = (k, v) => { S.settings[k] = v; this.persist(); this.renderMeta(); };
    this.setEl.replaceChildren(h('span', { class: 'ctl-l' }, 'Mode'), segmented(['auto', 'ask', 'read'], S.settings.mode, v => set('mode', v), { sm: true, labels: { auto: 'Auto', ask: 'Ask', read: 'Read-only' } }),
      h('span', { class: 'ctl-l' }, 'Model'), segmented(['quick', 'default', 'complex'], S.settings.tier, v => set('tier', v), { sm: true, labels: { quick: 'Fast', default: 'Balanced', complex: 'Deep' } }),
      h('span', { class: 'ctl-l' }, 'Thinking'), segmented(['', 'low', 'medium', 'high'], S.settings.effort, v => set('effort', v), { sm: true, labels: { '': 'Auto', low: 'Low', medium: 'Med', high: 'High' } }));
    this.offEl.replaceChildren(this.off ? h('div', { class: 'agent-off' }, h('b', null, 'The agent is off here.'), this.off) : '');
    this.renderLog(); this.renderMeta(); this.renderAttach();
  },
  renderMeta() {
    if (!this.meta) return;
    this.meta.textContent = { auto: 'steps run as they come', ask: 'you approve each step', read: 'read-only' }[this.S.settings.mode];
    this.sendBtn.textContent = this.busy ? 'Stop' : 'Send'; this.sendBtn.className = 'btn ' + (this.busy ? 'danger' : 'primary'); this.sendBtn.disabled = !!this.off && !this.busy;
  },
  renderAttach() {
    if (!this.attEl) return;
    this.attEl.replaceChildren(...this.attachments.map((a, i) => h('span', { class: 'att' }, h('img', { src: a.url, alt: 'pasted image ' + (i + 1) }), 'image ' + (i + 1),
      h('button', { class: 'link', title: 'Add it to the picture as a layer', onclick: async () => { const img = await readImageBlob(a.blob); app.ws.imageLayer(img, 'Pasted image'); } }, 'as layer'),
      h('button', { 'aria-label': 'Remove image', onclick: () => { URL.revokeObjectURL(a.url); this.attachments.splice(i, 1); this.renderAttach(); } }, '×'))));
  },
  renderLog() {
    if (!this.log) return;
    const th = this.thread(), log = this.log; log.textContent = '';
    if (!th.turns.length) {
      log.append(h('div', { class: 'agent-empty' }, h('p', null, 'The agent reads the layer stack and the picture, changes them through the same API as the panels, builds presets and runs the bench. Every change lands in History. Paste an image into the box below to show it.'),
        h('div', { class: 'sugg' }, SUGGEST.map(s => h('button', { disabled: !!this.off, onclick: () => this.send(s) }, s)))));
      log.scrollTop = 0; return;
    }
    for (const t of th.turns) {
      if (t.role === 'user') { log.append(h('div', { class: 'msg user' }, h('span', { class: 'who' }, 'You'), t.imgUrls && t.imgUrls.length ? h('div', { class: 'imgs' }, t.imgUrls.map(u => h('img', { src: u, alt: '' }))) : (t.imgs ? h('span', { class: 'who' }, `${t.imgs} image${t.imgs > 1 ? 's' : ''} sent`) : null), h('div', { class: 'body' }, t.text))); continue; }
      const m = h('div', { class: 'msg agent' + (t.note && !t.text ? ' error' : '') }, h('span', { class: 'who' }, 'Agent'));
      (t.steps || []).forEach((s, i) => m.append(this.stepEl(t, s, i)));
      if (t.pending && !t.text) m.append(h('span', { class: 'thinking' }, (t.steps || []).some(s => s.running) ? 'Working' : 'Thinking'));
      if (t.text) m.append(h('div', { class: 'md doc-inline', html: markdown(t.text) }));
      if (t.note) m.append(h('span', { class: 'note' }, t.note));
      log.append(m);
    }
    log.scrollTop = log.scrollHeight;
  },
  stepEl(turn, s, i) {
    const cls = 'tool' + (s.asking ? ' ask' : s.running ? ' run' : '') + (s.error ? ' err' : '') + (s.reverted ? ' reverted' : '');
    const first = (s.code || '').split('\n').find(l => l.trim()) || '';
    const d = h('details', { class: cls, 'data-step': `${(this.thread().turns || []).indexOf(turn)}:${i}`, open: !!s.asking || !!s.open, ontoggle: e => { s.open = e.target.open; } },
      h('summary', null, h('i', { class: 'dot' }), h('span', { class: 'nm' }, 'eval'), h('span', { class: 'first' }, first.trim()), h('span', { class: 'ms' }, s.asking ? 'waiting for you' : s.running ? 'running…' : fmtMs(s.ms || 0) + ' ms')));
    const tb = h('div', { class: 'tb' }, h('pre', { class: 'src', html: highlight(s.code || '') }));
    if (s.result != null) tb.append(h('pre', { class: 'res' + (s.error ? ' bad' : '') }, s.result));
    const row = h('div', { class: 'row' });
    if (s.asking) row.append(h('button', { class: 'btn sm primary', onclick: () => s.resolve(true) }, 'Run this step'), h('button', { class: 'btn sm', onclick: () => s.resolve(false) }, 'Decline'));
    if (s.changed && !s.reverted && s.histAt != null) row.append(h('button', { class: 'link', onclick: () => this.revert(turn, i) }, 'revert the document to before this step'));
    if (s.reverted) row.append(h('span', { class: 'dim' }, 'reverted'));
    row.append(h('button', { class: 'link', onclick: async () => { try { await navigator.clipboard.writeText(s.code); toast('Copied'); } catch (e) { toast('Copy failed', true); } } }, 'copy code'));
    tb.append(row); d.append(tb);
    return d;
  },
  closeThread(id) {
    const S = this.S; S.threads = S.threads.filter(t => t.id !== id); if (!S.threads.length) S.threads.push(this.newThreadObj());
    if (!S.threads.find(t => t.id === S.active)) S.active = S.threads[S.threads.length - 1].id; this.persist(); this.render();
  },
  revert(turn, i) {
    const s = turn.steps[i], ws = app.ws;
    if (s.histEntry && ws.hist.stack[s.histAt] !== s.histEntry) { toast('That step’s history is gone (a new document, or edits replaced it)', true); return; }
    ws.jump(s.histAt);
    for (let j = i; j < turn.steps.length; j++) if (turn.steps[j].changed) turn.steps[j].reverted = true;
    toast('Reverted the document to before that step'); this.persist(); this.render();
  },
  stop() { if (this.ctl) this.ctl.abort(); for (const t of this.thread().turns) for (const s of t.steps || []) if (s.asking) s.resolve(false); },

  // The thread as the turn list sample() takes: instructions on the first user turn, context on the last.
  toTurns(th, withImage) {
    const out = [];
    for (const t of th.turns) {
      if (t.role === 'user') out.push({ role: 'user', content: t.text + (t.imgs || (t.imgUrls && t.imgUrls.length) ? `\n(${t.imgs || t.imgUrls.length} pasted image${(t.imgs || t.imgUrls.length) > 1 ? 's' : ''})` : '') });
      else if (!t.pending) {
        const steps = (t.steps || []).map(s => `[eval${s.reverted ? ', later reverted' : ''}] ${(s.code || '').split('\n').find(l => l.trim()) ? (s.code || '').split('\n').find(l => l.trim()).trim().slice(0, 100) : ''} → ${String(s.result || '').replace(/\s+/g, ' ').slice(0, 160)}`);
        out.push({ role: 'assistant', content: [...steps, t.text || '(no answer)'].join('\n') });
      }
    }
    while (out.length && out[out.length - 1].role !== 'user') out.pop();
    const firstU = out.find(m => m.role === 'user'), lastU = [...out].reverse().find(m => m.role === 'user');
    if (firstU) firstU.content = agentInstructions() + '\n---\n' + firstU.content;
    if (lastU) lastU.content = agentContext(withImage) + '\n' + lastU.content;
    const merged = []; for (const m of out) { const p = merged[merged.length - 1]; if (p && p.role === m.role) p.content += '\n\n' + m.content; else merged.push(Object.assign({}, m)); }
    return merged;
  },
  async snapshotBlob() {
    const ws = app.ws, L = Math.max(0, Math.ceil(Math.log2(Math.max(ws.doc.w, ws.doc.h) / 1024))), D = ws.engine.dims(L);
    const c = document.createElement('canvas'); c.width = D.w; c.height = D.h; c.getContext('2d').putImageData(toImageData(ws.composite(L), D.w, D.h), 0, 0);
    return new Promise(r => c.toBlob(r, 'image/jpeg', 0.86));
  },
  async runEval(code, readOnly, source) { return runLab(code, { readOnly, source }); },
  async runCall(path, args, readOnly, source) {
    const scope = labScope(readOnly, source), t0 = performance.now(); let fn = scope, owner = null, value, error = null;
    try {
      for (const k of String(path).split('.')) { owner = fn; fn = fn == null ? undefined : fn[k]; }
      if (typeof fn !== 'function') throw new Error(`no function '${path}' in the lab API`);
      value = await fn.apply(owner, Array.isArray(args) ? args : []);
    } catch (e) { error = e; }
    let text = error ? 'Error: ' + error.message : yaml(value === undefined ? null : value);
    if (text.length > 9000) text = text.slice(0, 9000) + `\n… cut (${text.length} chars). Ask for less.`;
    return { text, error: !!error, ms: performance.now() - t0 };
  },
  async send(text) {
    text = (text || '').trim(); if (!text || this.busy) return;
    const sample = await this.getSample();
    if (!sample) { this.off = OFF.send; this.render(); return; }
    const S = this.S, th = this.thread(), ws = app.ws;
    let images = [];
    try { const lim = sample.limits ? await sample.limits() : null; if (lim && lim.images) { if (this.withImage) images.push(await this.snapshotBlob()); images.push(...this.attachments.map(a => a.blob)); } } catch (e) { images = []; }
    const user = { role: 'user', text, imgUrls: this.attachments.map(a => a.url) };
    th.turns.push(user); if (th.title === 'New thread') th.title = text.slice(0, 32);
    this.attachments = [];
    const turn = { role: 'agent', text: '', steps: [], pending: true }; th.turns.push(turn);
    if (this.ta) this.ta.value = ''; this.busy = true; this.persist(); this.render();
    this.ctl = new AbortController();
    const re = () => this.renderLog();
    const evalTool = {
      name: 'eval',
      description: 'Run an async JavaScript function body in Editor lab with the lab API in scope (layers, add, set, select, mask, paint, stats, render, presets, bench, …). Returns the value as YAML plus console.log output. Use `return` to return a value.',
      inputSchema: { type: 'object', properties: { code: { type: 'string', description: 'the body of an async function; return what you want to see' } }, required: ['code'] },
      execute: async ({ code, direct }) => {
        code = String(code || '');
        const step = { code, result: null, running: true, ms: 0, histAt: ws.hist.ptr };
        turn.steps.push(step); re();
        if (S.settings.mode === 'ask') {
          step.asking = true; re();
          const ok = await new Promise(r => { step.resolve = r; });
          step.asking = false; step.resolve = null;
          if (!ok) { step.running = false; step.error = true; step.result = 'The person declined this step.'; re(); return 'Declined by the person. Ask what they want instead, or explain the change without running it.'; }
        }
        const before = ws.hist.ptr, beforeEntry = ws.hist.stack[before];
        const source = { kind: 'agent', thread: th.id, turn: th.turns.indexOf(turn), step: turn.steps.indexOf(step) };
        const r = direct ? await this.runCall(direct.path, direct.args, S.settings.mode === 'read', source) : await this.runEval(code, S.settings.mode === 'read', source);
        Object.assign(step, { running: false, result: r.text, error: r.error, ms: r.ms, changed: ws.hist.ptr !== before || ws.hist.stack[before] !== beforeEntry });
        step.histAt = before; step.histEntry = ws.hist.stack[before] || null;
        re(); return r.text;
      },
    };
    // Where the page may not run code from strings, the agent gets one structured tool instead: call(path, args).
    const callTool = {
      name: 'call',
      description: 'Call one function of the Editor lab API by its path, with JSON arguments, and get the result as YAML. Examples: { "path": "layers", "args": [] }, { "path": "set", "args": ["L3", { "params": { "ev": -1 } }] }, { "path": "select.wand", "args": [640, 330, { "tolerance": 40 }] }.',
      inputSchema: { type: 'object', properties: { path: { type: 'string', description: 'a function in the lab API, e.g. set or mask.gradient' }, args: { type: 'array', description: 'its arguments, as JSON' } }, required: ['path'] },
      execute: async ({ path, args }) => evalTool.execute({ code: `return ${String(path).replace(/[^\w.]/g, '')}(...${JSON.stringify(Array.isArray(args) ? args : [])});`, direct: { path, args } }),
    };
    try {
      const turns = this.toTurns(th, images.length > 0 && this.withImage);
      const opts = { signal: this.ctl.signal, modelTier: S.settings.tier, tools: [CAN_EVAL ? evalTool : callTool], cache: false, onText: ({ text }) => { turn.text = text; re(); } };
      if (S.settings.effort) opts.effort = S.settings.effort;
      if (images.length) opts.images = images;
      const res = await sample(turns, opts);
      turn.text = res.text; if (res.truncated) turn.note = 'The answer hit the length limit.';
    } catch (e) {
      const code = e && e.code;
      if (OFF[code]) { this.off = OFF[code]; turn.note = OFF[code]; }
      else turn.note = NOTES[code] || ('Interrupted: ' + (e && (e.message || e.code) || e));
      if (code === 'refused') turn.text = '';
      else if (e && e.text && !turn.text) turn.text = e.text;
    }
    turn.pending = false; this.busy = false; this.ctl = null;
    this.persist(); this.render();
  },
};
registerView('Agent', { title: 'Agent', about: 'Claude, working through the same API as the panels', create: () => Agent.create(), onShow: () => Agent.onShow(), badge: () => Agent.busy ? { text: '●' } : null });
