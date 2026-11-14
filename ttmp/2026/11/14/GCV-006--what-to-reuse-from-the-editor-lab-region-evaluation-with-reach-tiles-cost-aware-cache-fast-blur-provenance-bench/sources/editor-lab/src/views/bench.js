// ---------------------------------------------------------------------------------------------
// views/bench.js: workloads run against a copy of the document, timed in this browser.
// "Measured" is this machine. Other profiles are modelled: measured time × a stated factor,
// with their cache budget applied for real, so hit rates and evictions are not modelled.
// ---------------------------------------------------------------------------------------------
'use strict';

const PROFILES = [
  { id: 'here', name: 'This browser', about: 'measured, × 1', cpu: 1, budgetMB: null },
  { id: 'laptop', name: 'Older laptop', about: 'modelled: measured × 1.8, 64 MB cache', cpu: 1.8, budgetMB: 64 },
  { id: 'tablet', name: 'Tablet', about: 'modelled: measured × 3.5, 32 MB cache', cpu: 3.5, budgetMB: 32 },
  { id: 'phone', name: 'Phone', about: 'modelled: measured × 5, 16 MB cache', cpu: 5, budgetMB: 16 },
];
const BUDGET_MS = 1000 / 60;

// Each scenario returns its updates: functions that change the copy. "continuous" means one gesture
// (a drag): the full-resolution render waits until it ends. "discrete" means separate actions.
function adjustments(ws) { return ws.doc.layers.filter(l => KINDS[l.kind].main && l.visible); }
function sweep(n, lo, hi) { return Array.from({ length: n }, (_, i) => lo + (hi - lo) * (0.5 - 0.5 * Math.cos(Math.PI * 2 * i / (n - 1)))); }
const SCENARIOS = [
  { id: 'scrub-top', name: 'Scrub the top adjustment', about: '40 slider steps on the highest adjustment layer', mode: 'continuous',
    make(ws) { const a = adjustments(ws); const l = a[a.length - 1] || ws.addLayer('exposure'); const k = KINDS[l.kind].main, s = KINDS[l.kind].params[k];
      return sweep(40, s.min + (s.max - s.min) * 0.3, s.min + (s.max - s.min) * 0.7).map(v => () => ws.live(l.id, { params: { [k]: v } })); } },
  { id: 'scrub-bottom', name: 'Scrub the lowest adjustment', about: '40 slider steps on the lowest adjustment: everything above it recomputes', mode: 'continuous',
    make(ws) { const a = adjustments(ws); let l = a[0]; if (!l) { l = ws.addLayer('exposure', {}, 1); } const k = KINDS[l.kind].main, s = KINDS[l.kind].params[k];
      return sweep(40, s.min + (s.max - s.min) * 0.3, s.min + (s.max - s.min) * 0.7).map(v => () => ws.live(l.id, { params: { [k]: v } })); } },
  { id: 'stroke-top', name: 'Brush stroke on top', about: '60 dabs of a 60 px brush on a paint layer at the top', mode: 'continuous',
    make(ws) { const l = ws.addLayer('paint', { name: 'Bench paint' }); ws.setActive(l.id, 'pixels'); return strokeUpdates(ws, 'pixels'); } },
  { id: 'stroke-low', name: 'Brush stroke under the adjustments', about: 'the same stroke on a paint layer second from the bottom: its tiles dirty everything above', mode: 'continuous',
    make(ws) { const l = ws.addLayer('paint', { name: 'Bench paint' }, 1); ws.setActive(l.id, 'pixels'); return strokeUpdates(ws, 'pixels'); } },
  { id: 'toggle', name: 'Toggle each layer', about: 'every layer off, then on again: each a separate action', mode: 'discrete',
    make(ws) { const out = []; for (const l of ws.doc.layers.slice(1)) { out.push(() => ws.setLayer(l.id, { visible: !l.visible })); out.push(() => ws.setLayer(l.id, { visible: !l.visible })); } return out; } },
  { id: 'blur-sweep', name: 'Blur radius sweep', about: 'a blur on top, its radius swept from 1 to 40 px in 20 steps', mode: 'continuous',
    make(ws) { const l = ws.addLayer('blur', { params: { radius: 1 } }); return sweep(20, 1, 40).map(v => () => ws.live(l.id, { params: { radius: v } })); } },
  { id: 'undo', name: 'Undo and redo', about: 'ten edits, then ten undos and ten redos: undo should hit the cache', mode: 'discrete',
    make(ws) { const a = adjustments(ws), out = []; const l = a[a.length - 1] || ws.addLayer('exposure'); const k = KINDS[l.kind].main, s = KINDS[l.kind].params[k];
      for (let i = 0; i < 10; i++) out.push(() => ws.setLayer(l.id, { params: { [k]: s.min + (s.max - s.min) * (0.35 + 0.03 * i) } }));
      for (let i = 0; i < 10; i++) out.push(() => ws.undo()); for (let i = 0; i < 10; i++) out.push(() => ws.redo()); return out; } },
  { id: 'radial-drag', name: 'Drag a radial mask', about: 'an exposure layer on top whose radial mask is dragged across the picture in 30 steps', mode: 'continuous',
    make(ws) { const l = ws.addLayer('exposure', { name: 'Bench spot', params: { ev: 0.6 } }); const c = ws.addMaskComp(l.id, 'radial', { params: { cx: 0.2, cy: 0.5, rx: 0.15, ry: 0.22, feather: 0.6 } });
      return sweep(30, 0.2, 0.8).map(v => () => ws.liveMask(l.id, () => { c.params.cx = v; c.params.cy = 0.5 + 0.15 * Math.sin(v * 9); })); } },
  { id: 'cold', name: 'Cold render', about: 'the cache cleared, then one full render: opening a document', mode: 'discrete',
    make(ws) { return [() => { ws.engine.cache.clear(); ws.engine.maskCache.clear(); ws.changed('cold'); }]; } },
];
function strokeUpdates(ws, target) {
  const W = ws.doc.w, H = ws.doc.h, b = { size: 60, hardness: 0.5, flow: 0.7, color: '#ffffff', erase: false }, out = [];
  out.push(() => ws.strokeBegin(target));
  for (let i = 0; i < 60; i++) { const t = i / 59, x = W * (0.15 + 0.7 * t), y = H * (0.5 + 0.25 * Math.sin(t * Math.PI * 2)); out.push(() => ws.dab(x, y, b)); }
  out.push(() => ws.strokeEnd('Bench stroke', b));
  return out;
}
// Recorded mask patches name brush rasters by id; on the copy, reuse each component's own raster.
function relinkPatch(l, patch) {
  if (!patch.mask || !patch.mask.comps) return patch;
  const comps = patch.mask.comps.map(c => Object.assign({}, c, { raster: c.raster ? ((l.mask && l.mask.comps.find(x => x.id === c.id)) || {}).raster || null : null }));
  return Object.assign({}, patch, { mask: Object.assign({}, patch.mask, { comps }) });
}
// A recorded session replays its operations; a stroke becomes one update per few dabs.
function sessionScenario(s) {
  return { id: s.id, name: s.name, about: `${s.ops.length} operations recorded on ${s.doc} · ${fmt(s.seconds, 1)} s`, mode: 'discrete', session: true,
    make(ws) {
      const out = [], L = i => ws.doc.layers[i];
      for (const op of s.ops) {
        if (op.op === 'set') out.push(() => L(op.i) && ws.setLayer(L(op.i).id, relinkPatch(L(op.i), op.patch)));
        else if (op.op === 'stroke' && L(op.i)) { out.push(() => { ws.setActive(L(op.i).id, op.target); ws.strokeBegin(op.target); }); for (let k = 0; k < op.pts.length; k += 3) { const chunk = op.pts.slice(k, k + 3); out.push(() => chunk.forEach(([x, y]) => ws.dab(x, y, op.brush))); } out.push(() => ws.strokeEnd('Replayed stroke', op.brush)); }
        else if (op.op === 'add') out.push(() => ws.addLayer(op.kind, op.opts, op.i));
        else if (op.op === 'addMany') out.push(() => ws.addLayers(op.specs, 'Replayed preset'));
        else if (op.op === 'remove') out.push(() => L(op.i) && ws.removeLayer(L(op.i).id));
        else if (op.op === 'move') out.push(() => L(op.from) && ws.moveLayer(L(op.from).id, op.to));
        else if (op.op === 'maskGradient') out.push(() => L(op.i) && ws.maskGradient(L(op.i).id, op.p0, op.p1, op.a, op.b));
        else if (op.op === 'mask') out.push(() => L(op.i) && !L(op.i).mask && ws.addMask(L(op.i).id, op.from === 'selection' && !ws.doc.sel ? 'reveal' : op.from));
        else if (op.op === 'undo') out.push(() => ws.undo());
        else if (op.op === 'redo') out.push(() => ws.redo());
      }
      return out;
    } };
}

const q = (xs, p) => { if (!xs.length) return 0; const s = [...xs].sort((a, b) => a - b); return s[Math.min(s.length - 1, Math.floor(p * s.length))]; };
function fullRender(eng) { const g = eng.grid(0), t0 = performance.now(); for (let ty = 0; ty < g.ny; ty++) for (let tx = 0; tx < g.nx; tx++) eng.tile(eng.topIndex(), 0, tx, ty); return performance.now() - t0; }
function previewRender(eng, L) { const D = eng.dims(L), t0 = performance.now(); eng.rect(eng.topIndex(), L, 0, 0, D.w, D.h); return performance.now() - t0; }

const Bench = {
  sel: new Set(['scrub-top', 'scrub-bottom', 'stroke-low', 'toggle', 'undo']), profs: new Set(['here', 'tablet']), tiles: new Set([128]), previews: new Set(['on']), evicts: new Set(['lru']), repeats: 1,
  runs: [], sessions: [], shown: null, baseline: null, running: false, stopFlag: false,
  scenarios() { return [...SCENARIOS, ...this.sessions.map(sessionScenario)]; },
  async load() {
    const d = store.get('bench', null); if (d) { this.runs = d.runs || []; this.sessions = d.sessions || []; }
    this.ref = await userDoc('bench');
    if (this.ref) try { const s = await Promise.race([this.ref.get(), sleep(10000).then(() => null)]); if (s && s.exists) { const v = s.data(); this.runs = JSON.parse(JSON.stringify(v.runs || [])); this.sessions = JSON.parse(JSON.stringify(v.sessions || [])); } } catch (e) { this.ref = null; }
    this.shown = this.runs[this.runs.length - 1] || null; this.render();
  },
  persist: debounce(async function () {
    const B = Bench, data = { runs: B.runs.slice(-12), sessions: B.sessions.slice(-8) }; store.set('bench', data);
    if (B.ref) try { await B.ref.set(JSON.parse(JSON.stringify(data))); } catch (e) { B.ref = null; }
  }, 800),

  // Run every chosen scenario × profile × tile size × preview policy, `repeats` times.
  async run(opts = {}) {
    if (this.running) throw new Error('a bench run is already going');
    const scen = (opts.scenarios || [...this.sel]).map(id => this.scenarios().find(s => s.id === id)).filter(Boolean);
    const profs = (opts.profiles || [...this.profs]).map(id => PROFILES.find(p => p.id === id)).filter(Boolean);
    const tiles = opts.tiles || [...this.tiles], previews = opts.previews || [...this.previews], evicts = opts.evictions || [...this.evicts], reps = opts.repeats || this.repeats;
    if (!scen.length || !profs.length || !tiles.length || !previews.length) throw new Error('choose at least one scenario, profile, tile size and preview policy');
    const combos = []; for (const s of scen) for (const p of profs) for (const t of tiles) for (const pv of previews) for (const ev of evicts) combos.push([s, p, t, pv, ev]);
    this.running = true; this.stopFlag = false; const t0 = performance.now(); const rows = [];
    const run = { budgetMB: app.settings.budgetMB, id: 'R' + Date.now().toString(36), n: (this.runs.length ? this.runs[this.runs.length - 1].n : 0) + 1, at: new Date().toISOString(), doc: app.ws.doc.code + ' ' + app.ws.doc.title, size: `${app.ws.doc.w} × ${app.ws.doc.h}`, rows };
    try {
      for (let c = 0; c < combos.length; c++) {
        const [s, p, tile, pv, ev] = combos[c];
        app.emit('bench', { running: true, text: `${c + 1} of ${combos.length} · ${s.name}` }); this.progress((c) / combos.length, `${s.name} · ${p.name} · tile ${tile} px · preview ${pv} · ${ev === 'cost' ? 'cost-aware' : 'LRU'}`);
        const samples = [];
        for (let r = 0; r < reps; r++) { samples.push(await this.one(s, p, tile, pv, ev)); if (this.stopFlag) break; }
        if (this.stopFlag) break;
        const agg = k => samples.reduce((a, x) => a + x[k], 0) / samples.length;
        rows.push({ scenario: s.id, name: s.name, profile: p.id, tile, preview: pv, evict: ev, updates: samples[0].updates,
          p50: agg('p50'), p95: agg('p95'), settle: agg('settle'), over: agg('over'), cold: agg('cold'), tiles: Math.round(agg('tiles')), hit: agg('hit'), peakMB: agg('peakMB'), evictions: Math.round(agg('evictions')), lat: samples[0].lat });
        if (this.shown === run) this.renderResults(); else { this.shown = run; this.renderResults(); }
      }
    } finally {
      this.running = false; app.emit('bench', { running: false }); this.progress(null);
    }
    run.ms = performance.now() - t0; run.stopped = this.stopFlag;
    if (rows.length) { this.runs.push(run); this.persist(); }
    this.shown = run; this.render();
    return run;
  },
  async one(s, p, tile, pv, ev = 'lru') {
    const doc = cloneDoc(app.ws.doc), ws = new Workspace(doc, { tile, budgetMB: p.budgetMB || app.settings.budgetMB, evict: ev }), eng = ws.engine;
    const L = pv === 'on' ? 2 : null;
    const cold = fullRender(eng); eng.cache.resetCounters();
    const ups = s.make(ws); eng.invalidate();
    const lat = [], fulls = []; let comp0 = eng.computed;
    for (let i = 0; i < ups.length; i++) {
      ups[i](); eng.invalidate();
      if (s.mode === 'continuous') { lat.push(L != null ? previewRender(eng, L) : fullRender(eng)); }
      else { const a = L != null ? previewRender(eng, L) : 0; const f = fullRender(eng); lat.push(L != null ? a : f); fulls.push(f); }
      if (i % 3 === 2) { await sleep(0); if (this.stopFlag) break; }
    }
    const settle = s.mode === 'continuous' ? fullRender(eng) : q(fulls, 0.95);
    const k = p.cpu, c = eng.cache;
    return { lat: lat.map(x => +(x * k).toFixed(3)), updates: ups.length, p50: q(lat, 0.5) * k, p95: q(lat, 0.95) * k, settle: settle * k, over: lat.filter(x => x * k > BUDGET_MS).length / (lat.length || 1), cold: cold * k,
      tiles: eng.computed - comp0, hit: c.hits / ((c.hits + c.misses) || 1), peakMB: c.peak / 1048576, evictions: c.evictions };
  },
  stop() { this.stopFlag = true; },
  async runBlur() { this.gpuBusy = true; this.gpuErr = null; this.renderSide(); try { await GPU.blurProbe(); } catch (e) { this.gpuErr = e.message; } this.gpuBusy = false; this.renderSide(); },
  async runWasm() { this.wasmBusy = true; this.wasmErr = null; this.renderSide(); try { await WASM.probe(); } catch (e) { this.wasmErr = e.message; } this.wasmBusy = false; this.renderSide(); },
  async runGpu() { this.gpuBusy = true; this.gpuErr = null; this.renderSide(); try { await GPU.probe(); } catch (e) { this.gpuErr = e.message; } this.gpuBusy = false; this.renderSide(); },
  summary(run) { return (run || this.shown || {}).rows ? (run || this.shown).rows.map(r => ({ scenario: r.scenario, profile: r.profile, tile: r.tile, preview: r.preview, eviction: r.evict || 'lru', update_p50_ms: +r.p50.toFixed(2), update_p95_ms: +r.p95.toFixed(2), settle_ms: +r.settle.toFixed(1), over_budget: +(r.over * 100).toFixed(0) + ' %', tiles_computed: r.tiles, hit_rate: +(r.hit * 100).toFixed(0) + ' %', peak_MB: +r.peakMB.toFixed(1), gate: r.p95 <= BUDGET_MS ? 'pass' : 'fail' })) : []; },
  record() {
    const ws = app.ws;
    if (ws.rec) {
      const r = ws.rec; ws.rec = null;
      if (!r.ops.length) { toast('Nothing recorded: the session had no edits'); this.render(); Stage.updateChips(); return; }
      for (const op of r.ops) if (op.pts && op.pts.length > 400) op.pts = op.pts.filter((_, i) => i % Math.ceil(op.pts.length / 400) === 0);
      const n = this.sessions.length + 1, s = { id: 'S-' + Date.now().toString(36), name: `Session ${n}`, doc: ws.doc.code, seconds: (performance.now() - r.t0) / 1000, ops: r.ops };
      this.sessions.push(s); this.sel.add(s.id); this.persist(); toast(`Recorded ${s.name}: ${s.ops.length} operations`);
    } else { ws.rec = { t0: performance.now(), ops: [] }; toast('Recording: edit as usual, then press Stop recording'); }
    this.render(); Stage.updateChips();
  },
  show(qp) {
    if (qp.scenario) { this.sel = new Set(qp.scenario.split(',')); }
    if (qp.profile) this.profs = new Set(qp.profile.split(','));
    if (qp.tile) this.tiles = new Set(qp.tile.split(',').map(Number));
    if (qp.preview) this.previews = new Set(qp.preview.split(','));
    if (qp.evict) this.evicts = new Set(qp.evict.split(','));
    this.render();
    if (qp.run) this.run().catch(e => toast(e.message, true));
  },

  // ---- the view ----
  create() {
    this.top = h('div', { class: 'toolbar' }); this.body = h('div', { class: 'scroll' }); this.sideEl = h('aside', { class: 'side' });
    this.el = h('div', { class: 'tabbody bench' }, h('div', { class: 'page col', style: { flexDirection: 'column' } }, this.top, this.body), this.sideEl);
    app.on('frame', debounce(() => this.renderSide(), 200)); app.on('settings', () => this.renderSide()); app.on('scene', () => this.render());
    this.render(); return this.el;
  },
  progress(f, text) {
    if (!this.meter) return;
    this.meter.hidden = f == null; this.progLbl.textContent = f == null ? '' : text;
    if (f != null) this.meter.firstChild.style.width = Math.round(f * 100) + '%';
  },
  render() {
    if (!this.el) return;
    const t = this.top; t.textContent = '';
    this.meter = h('div', { class: 'meter expmeter', hidden: true }, h('i', { style: { width: '0%' } })); this.progLbl = h('span', { class: 'lbl' });
    const runBtn = this.running ? h('button', { class: 'btn sm danger', onclick: () => this.stop() }, 'Stop') : h('button', { class: 'btn sm primary', onclick: () => this.run().catch(e => toast(e.message, true)) }, 'Run');
    const reps = h('select', { class: 'selectlike', style: { width: 'auto' }, 'aria-label': 'Repeats', onchange: e => { this.repeats = +e.target.value; } }, [1, 3, 5].map(n => h('option', { value: n, selected: n === this.repeats }, n + '×')));
    const rec = h('button', { class: 'btn sm' + (app.ws && app.ws.rec ? ' danger' : ''), onclick: () => this.record() }, app.ws && app.ws.rec ? '■ Stop recording' : '● Record a session');
    const nCombos = this.sel.size * this.profs.size * this.tiles.size * this.previews.size * this.evicts.size;
    const exp = h('button', { class: 'btn sm', 'aria-haspopup': 'menu', onclick: e => openMenu(e.currentTarget, Exports.benchItems(), { right: true }) }, 'Export ▾');
    t.append(runBtn, reps, h('span', { class: 'lbl' }, `${nCombos} runs on a copy of ${app.ws.doc.code}`), this.meter, this.progLbl, h('span', { class: 'sp' }), exp, rec);
    const b = this.body; b.textContent = '';
    const chipRow = (label, items) => h('div', { class: 'block' }, h('header', null, h('h2', null, label)), h('div', { class: 'chips' }, items));
    const chip = (set, v, text, title) => h('button', { class: set.has(v) ? 'on' : '', title: title || null, 'aria-pressed': String(set.has(v)), onclick: () => { set.has(v) ? set.delete(v) : set.add(v); this.render(); } }, text);
    b.append(h('div', { class: 'explist' }, this.scenarios().map(s => h('div', { class: 'exprow' + (this.sel.has(s.id) ? ' on' : '') },
      h('button', { class: 'tog' + (this.sel.has(s.id) ? ' on' : ''), 'aria-pressed': String(this.sel.has(s.id)), onclick: () => { this.sel.has(s.id) ? this.sel.delete(s.id) : this.sel.add(s.id); this.render(); } }, ''),
      h('button', { class: 'linkhd', onclick: () => { this.sel = new Set([s.id]); this.render(); } }, h('b', null, s.name)), h('span', { class: 'sub' }, s.about + ' · ' + s.mode),
      h('span', { class: 'sp' }), s.session ? h('button', { class: 'link', onclick: () => { this.sessions = this.sessions.filter(x => x.id !== s.id); this.sel.delete(s.id); this.persist(); this.render(); } }, 'delete') : null,
      h('button', { class: 'btn sm', disabled: this.running, onclick: () => this.run({ scenarios: [s.id] }).catch(e => toast(e.message, true)) }, 'Run')))));
    b.append(chipRow('Profiles', PROFILES.map(p => chip(this.profs, p.id, p.name, p.about))),
      chipRow('Tile size', [64, 128, 256].map(n => chip(this.tiles, n, n + ' px'))),
      chipRow('Preview', [chip(this.previews, 'on', 'coarse first (¼)', 'show a quarter-resolution frame while the gesture lasts'), chip(this.previews, 'off', 'full only', 'render every update at full resolution')]),
      chipRow('Eviction', [chip(this.evicts, 'lru', 'least recently used'), chip(this.evicts, 'cost', 'cost-aware (GreedyDual-Size)', 'keeps tiles that were expensive to compute for their size')]));
    this.resultsEl = h('div'); b.append(this.resultsEl); this.renderResults();
    b.append(h('div', { class: 'block' }, h('header', null, h('h2', null, 'Runs'), h('span', { class: 'sub' }, this.ref ? 'saved to your account' : 'saved in this browser')),
      this.runs.length ? h('div', { class: 'runlist' }, [...this.runs].reverse().map(r => h('button', { class: r === this.shown ? 'on' : '', onclick: () => { this.shown = r; this.render(); } }, `Run ${r.n}`, h('small', null, `${new Date(r.at).toLocaleString('en-GB', { dateStyle: 'short', timeStyle: 'short' })} · ${r.doc} · ${r.rows.length} rows${this.baseline === r ? ' · baseline' : ''}`)))) : h('p', { class: 'note' }, 'No runs yet. Choose scenarios and profiles, then Run.')));
    this.renderSide();
  },
  renderResults() {
    const el = this.resultsEl; if (!el) return; el.textContent = '';
    const run = this.shown; if (!run || !run.rows.length) { el.append(h('p', { class: 'empty padded' }, 'Results appear here: each row is one scenario on one profile with one policy.')); return; }
    const base = this.baseline && this.baseline !== run ? this.baseline : null;
    el.append(h('div', { class: 'reshd' }, h('b', null, `Run ${run.n}`), h('span', { class: 'sub' }, `${run.doc} · ${run.size} px · ${run.rows.length} rows${run.ms ? ' · ' + fmt(run.ms / 1000, 1) + ' s' : ''}${run.stopped ? ' · stopped' : ''}`), h('span', { class: 'sp' }),
      h('button', { class: 'link', onclick: () => { this.baseline = this.baseline === run ? null : run; this.render(); } }, this.baseline === run ? 'baseline (click to clear)' : 'use as baseline')));
    const pass = run.rows.filter(r => r.p95 <= BUDGET_MS).length;
    el.append(h('p', { class: 'note padded', style: { paddingTop: '6px', paddingBottom: '0' } }, `Gate: update p95 at most 16.7 ms (one frame at 60 Hz). ${pass} of ${run.rows.length} pass. Modelled profiles scale measured time; their cache budgets are real.`));
    const th = s => h('th', { class: 'num' }, s);
    const tbl = h('table', { class: 'rt' }, h('thead', null, h('tr', null, h('th', null, 'scenario'), h('th', null, 'profile'), th('tile'), h('th', null, 'preview'), h('th', null, 'evict'), th('update p50'), th('update p95'), base ? th('Δ p95') : null, th('settle'), th('over budget'), th('tiles'), th('hit'), th('peak'), h('th', null, 'gate'))));
    const tb = h('tbody'); tbl.append(tb);
    for (const r of run.rows) {
      const b = base && base.rows.find(x => x.scenario === r.scenario && x.profile === r.profile && x.tile === r.tile && x.preview === r.preview && (x.evict || 'lru') === (r.evict || 'lru'));
      const d = b ? (r.p95 - b.p95) / (b.p95 || 1) : null;
      tb.append(h('tr', { class: r.p95 > BUDGET_MS ? '' : '' }, h('td', { class: 'nm' }, r.name), h('td', null, (PROFILES.find(p => p.id === r.profile) || {}).name || r.profile), h('td', { class: 'num' }, r.tile), h('td', null, r.preview === 'on' ? '¼ first' : 'full'), h('td', null, (r.evict || 'lru') === 'cost' ? 'cost' : 'LRU'),
        h('td', { class: 'num' }, fmtMs(r.p50) + ' ms'), h('td', { class: 'num' }, fmtMs(r.p95) + ' ms'), base ? h('td', { class: 'num delta' + (d > 0.1 ? ' worse' : '') }, d == null ? '–' : (d > 0 ? '+' : '') + Math.round(d * 100) + ' %') : null,
        h('td', { class: 'num' }, fmtMs(r.settle) + ' ms'), h('td', { class: 'num' }, Math.round(r.over * 100) + ' %'), h('td', { class: 'num' }, r.tiles), h('td', { class: 'num' }, Math.round(r.hit * 100) + ' %'), h('td', { class: 'num' }, fmt(r.peakMB, 1) + ' MB'),
        h('td', { class: 'gate' + (r.p95 > BUDGET_MS ? ' bad' : '') }, r.p95 > BUDGET_MS ? '✗ over' : '✓')));
    }
    el.append(h('div', { class: 'tablewrap' }, tbl), h('div', { class: 'expres' }, this.scatter(run)));
  },
  // Update p95 against peak cache memory; profiles by mark shape, never by colour.
  scatter(run) {
    const W = 520, H = 250, m = { l: 48, r: 16, t: 14, b: 34 }, rows = run.rows;
    const xs = rows.map(r => Math.max(0.1, r.p95)), ys = rows.map(r => r.peakMB);
    const x0 = Math.log10(Math.min(...xs, BUDGET_MS) / 1.5), x1 = Math.log10(Math.max(...xs, BUDGET_MS) * 1.5), y1 = Math.max(...ys) * 1.15 || 1;
    const X = v => m.l + (Math.log10(Math.max(0.1, v)) - x0) / (x1 - x0) * (W - m.l - m.r), Y = v => H - m.b - v / y1 * (H - m.t - m.b);
    const shapes = ['circle', 'square', 'diamond', 'triangle'], mark = (shape, x, y, cls) => shape === 'circle' ? `<circle class="${cls}" cx="${x}" cy="${y}" r="3.6"/>` : shape === 'square' ? `<rect class="${cls}" x="${x - 3.2}" y="${y - 3.2}" width="6.4" height="6.4"/>` : shape === 'diamond' ? `<path class="${cls}" d="M${x} ${y - 4.4}L${x + 4.4} ${y}L${x} ${y + 4.4}L${x - 4.4} ${y}Z"/>` : `<path class="${cls}" d="M${x} ${y - 4.4}L${x + 4.2} ${y + 3.4}L${x - 4.2} ${y + 3.4}Z"/>`;
    let s = `<svg class="roof scatter" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="Scatter: update p95 against peak cache memory, one mark per row">`;
    const ticks = [0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000].filter(v => Math.log10(v) >= x0 && Math.log10(v) <= x1);
    for (const v of ticks) s += `<line class="gl" x1="${X(v)}" x2="${X(v)}" y1="${m.t}" y2="${H - m.b}"/><text class="ax" x="${X(v)}" y="${H - m.b + 13}" text-anchor="middle">${v}</text>`;
    for (let i = 0; i <= 4; i++) { const v = y1 * i / 4; s += `<line class="gl" x1="${m.l}" x2="${W - m.r}" y1="${Y(v)}" y2="${Y(v)}"/><text class="ax" x="${m.l - 6}" y="${Y(v) + 3}" text-anchor="end">${fmt(v, v < 10 ? 1 : 0)}</text>`; }
    s += `<line class="rl" x1="${X(BUDGET_MS)}" x2="${X(BUDGET_MS)}" y1="${m.t}" y2="${H - m.b}" stroke-dasharray="4 3"/><text class="ax" x="${X(BUDGET_MS) + 4}" y="${m.t + 9}">16.7 ms</text>`;
    s += `<text class="ax" x="${(m.l + W - m.r) / 2}" y="${H - 4}" text-anchor="middle">update p95, ms (log scale)</text><text class="ax" transform="translate(12 ${(m.t + H - m.b) / 2}) rotate(-90)" text-anchor="middle">peak cache, MB</text>`;
    const profs = [...new Set(rows.map(r => r.profile))];
    profs.forEach((pid, i) => {
      s += `<g class="series s${Math.min(i, 3)}">`;
      for (const r of rows.filter(r => r.profile === pid)) s += `<g class="pt"><title>${esc(r.name)} · ${esc(pid)} · tile ${r.tile} · ${fmtMs(r.p95)} ms · ${fmt(r.peakMB, 1)} MB</title>${mark(shapes[i % 4], X(r.p95), Y(r.peakMB), '')}</g>`;
      s += '</g>';
    });
    s += '</svg>';
    const legend = h('div', { class: 'legend' }, profs.map((pid, i) => h('span', { html: `<svg width="12" height="12" viewBox="-6 -6 12 12" class="roof scatter"><g class="series s${Math.min(i, 3)}"><g class="pt">${mark(shapes[i % 4], 0, 0, '')}</g></g></svg>${esc((PROFILES.find(p => p.id === pid) || {}).name || pid)}` })));
    return h('div', null, legend, h('div', { class: 'scatterwrap', html: s }));
  },
  renderSide() {
    const sd = this.sideEl; if (!sd || !app.ws) return; sd.textContent = '';
    const eng = app.ws.engine, c = eng.cache, st = Render.stats;
    const budget = paramRow({ label: 'cache budget', spec: P(8, 512, 96, 'MB', '', 8), value: app.settings.budgetMB, onLive: () => {}, onCommit: v => { app.settings.budgetMB = v; applyEngineSettings(); } });
    sd.append(h('section', { class: 'block' }, h('header', null, h('h2', null, 'Engine'), h('span', { class: 'sub' }, 'for the stage')),
      h('div', { class: 'row' }, h('span', { class: 'note' }, 'tile'), segmented(['64', '128', '256'], String(app.settings.tile), v => { app.settings.tile = +v; applyEngineSettings(); }, { sm: true, labels: { 64: '64', 128: '128', 256: '256 px' } })),
      h('div', { class: 'row' }, h('span', { class: 'note' }, 'preview'), segmented(['auto', '1', '2', '3', 'off'], app.settings.preview, v => { app.settings.preview = v; applyEngineSettings(); }, { sm: true, labels: { auto: 'Auto', 1: '½', 2: '¼', 3: '⅛', off: 'Off' } })),
      h('div', { class: 'row' }, h('span', { class: 'note' }, 'evict'), segmented(['lru', 'cost'], app.settings.evict, v => { app.settings.evict = v; applyEngineSettings(); }, { sm: true, labels: { lru: 'LRU', cost: 'Cost-aware' } })),
      h('div', { class: 'paramlist', style: { margin: '0 -14px' } }, budget),
      h('div', { class: 'row' }, h('button', { class: 'btn sm', onclick: () => { eng.cache.clear(); eng.maskCache.clear(); eng.invalidate(); Render.request(false); toast('Cleared the tile cache'); this.renderSide(); } }, 'Clear cache'))));
    sd.append(h('section', { class: 'block' }, h('header', null, h('h2', null, 'Last update'), h('span', { class: 'sub' }, 'on the stage')),
      h('div', { class: 'bignum' }, h('b', null, fmtMs(st.fullMs) + ' ms'), h('span', null, 'from the change to the last tile on screen')),
      h('dl', { class: 'kv' }, h('dt', null, 'preview shown after'), h('dd', null, st.coarseL ? fmtMs(st.coarseMs) + ' ms' : '–'), h('dt', null, 'tiles computed'), h('dd', null, `${st.computed} of ${st.tiles}`), h('dt', null, 'tile hit rate'), h('dd', null, Math.round(st.hitRate * 100) + ' %'))));
    sd.append(h('section', { class: 'block' }, h('header', null, h('h2', null, 'Cache'), h('span', { class: 'sub' }, c.policy === 'cost' ? 'cost-aware' : 'least recently used')),
      h('dl', { class: 'kv' }, h('dt', null, 'held'), h('dd', null, `${fmtMB(c.bytes)} of ${app.settings.budgetMB} MB`), h('dt', null, 'tiles'), h('dd', null, c.map.size), h('dt', null, 'hits · misses'), h('dd', null, `${c.hits} · ${c.misses}`), h('dt', null, 'evictions'), h('dd', null, c.evictions)),
      h('div', { class: 'meter' + (c.bytes > c.budget * 0.95 ? ' bad' : '') }, h('i', { style: { width: Math.min(100, c.bytes / c.budget * 100) + '%' } }))));
    const g = GPU.last, gb = h('section', { class: 'block' }, h('header', null, h('h2', null, 'GPU probe'), h('span', { class: 'sub' }, 'WebGPU, fused layers')),
      h('p', { class: 'note' }, 'Compiles this document’s pointwise layers into one compute shader, runs it on the bottom layer, and times it against the CPU engine on the same layers.'),
      h('div', { class: 'row' }, h('button', { class: 'btn sm primary', disabled: this.gpuBusy, onclick: () => this.runGpu() }, this.gpuBusy ? 'Running…' : 'Run the probe'),
        GPU.lastCode ? h('button', { class: 'link', onclick: () => { this.showWgsl = !this.showWgsl; this.renderSide(); } }, this.showWgsl ? 'hide the shader' : 'show the shader') : null,
        GPU.lastCode ? h('button', { class: 'link', onclick: () => Exports.save(`fused-${app.ws.doc.code}.wgsl.txt`, GPU.lastCode, 'the shader') }, 'export it') : null));
    if (this.gpuErr) gb.append(h('p', { class: 'fail' }, h('b', null, 'Stopped: '), this.gpuErr));
    if (g) gb.append(h('div', { class: 'bignum' }, h('b', null, fmtMs(g.gpu_ms) + ' ms'), h('span', null, `on the GPU, against ${fmtMs(g.cpu_ms)} ms on the CPU`)),
      h('dl', { class: 'kv' }, ...[['fused', g.fused.length + ' layers'], ['compile', fmtMs(g.compile_ms) + ' ms'], ['upload', fmtMs(g.upload_ms) + ' ms'], ['readback', fmtMs(g.readback_ms) + ' ms'], ['kernel speed-up', g.speedup_kernel_only + ' ×'], ['with transfers', g.speedup_with_transfers + ' ×'], ['largest difference', g.max_error_8bit + ' of 255']].flatMap(([k, v]) => [h('dt', null, k), h('dd', null, v)])),
      h('p', { class: 'note' }, `${g.adapter}. Fused: ${g.fused.join(', ') || 'none'}.${g.skipped.length ? ' Left out: ' + g.skipped.join('; ') + '.' : ''}`));
    if (this.showWgsl && GPU.lastCode) gb.append(h('pre', { class: 'src', html: highlight(GPU.lastCode) }));
    const gbl = GPU.lastBlur;
    gb.append(h('div', { class: 'row' }, h('button', { class: 'btn sm', disabled: this.gpuBusy, onclick: () => this.runBlur() }, 'Time a blur pass'),
      h('span', { class: 'note' }, 'workgroup memory against plain buffer reads')));
    if (gbl) gb.append(h('dl', { class: 'kv' }, ...[['radius', gbl.radius + ' px'], ['workgroup', `${fmtMs(gbl.workgroup.gpu_ms)} ms · ${gbl.workgroup.buffer_reads_per_pixel} reads a pixel`], ['plain reads', `${fmtMs(gbl.naive.gpu_ms)} ms · ${gbl.naive.buffer_reads_per_pixel} reads a pixel`], ['CPU boxH4', fmtMs(gbl.cpu_boxH4_ms) + ' ms'], ['largest difference', Math.max(gbl.workgroup.max_error_8bit, gbl.naive.max_error_8bit) + ' of 255']].flatMap(([k, v]) => [h('dt', null, k), h('dd', null, v)])));
    sd.append(gb);
    const w = WASM.last, we = WASM.env(), wb = h('section', { class: 'block' }, h('header', null, h('h2', null, 'WASM probe'), h('span', { class: 'sub' }, 'WebAssembly kernels')),
      h('p', { class: 'note' }, 'Runs a hand-written box pass (scalar and SIMD) and a gain over the current composite, against the engine’s JavaScript.'),
      h('dl', { class: 'kv' }, ...[['SIMD', we.simd ? 'yes' : 'no'], ['threads', we.threads ? 'yes' : we.sharedArrayBuffer ? 'off: page not cross-origin isolated' : 'off: no SharedArrayBuffer here'], ['cores', we.cores ?? 'unknown']].flatMap(([k, v]) => [h('dt', null, k), h('dd', null, v)])),
      h('div', { class: 'row' }, h('button', { class: 'btn sm primary', disabled: this.wasmBusy, onclick: () => this.runWasm() }, this.wasmBusy ? 'Running…' : 'Run the probe')));
    if (this.wasmErr) wb.append(h('p', { class: 'fail' }, h('b', null, 'Stopped: '), this.wasmErr));
    if (w) wb.append(h('div', { class: 'bignum' }, h('b', null, w.box_pass.simd_over_js + ' ×'), h('span', null, `SIMD box pass against boxH4 (${fmtMs(w.box_pass.wasm_simd_ms)} against ${fmtMs(w.box_pass.js_boxH4_ms)} ms)`)),
      h('dl', { class: 'kv' }, ...[['scalar WASM', fmtMs(w.box_pass.wasm_scalar_ms) + ' ms'], ['SIMD over scalar', w.box_pass.simd_over_scalar + ' ×'], ['gain, JS · SIMD', `${fmtMs(w.gain.js_ms)} · ${fmtMs(w.gain.wasm_simd_ms)} ms`], ['copy in · out', `${fmtMs(w.copy_in_ms)} · ${fmtMs(w.copy_out_ms)} ms`], ['largest difference', w.box_pass.max_error_8bit.simd + ' of 255']].flatMap(([k, v]) => [h('dt', null, k), h('dd', null, v)])));
    sd.append(wb);
    sd.append(h('section', { class: 'block' }, h('header', null, h('h2', null, 'Profiles')), h('dl', { class: 'kv keys' }, PROFILES.flatMap(p => [h('dt', null, p.name), h('dd', null, p.about)]))));
  },
};
registerView('Bench', { title: 'Bench', about: 'workloads on a copy of the document: timed, compared, gated', create: () => Bench.create(), badge: () => Bench.running ? { text: '●' } : null });
