// ---------------------------------------------------------------------------------------------
// views/code.js: the code editor. Scripts run against the open document through the same lab API
// as the agent, inside an async function; what a script returns is shown as YAML. Presets open
// here as JavaScript, and a set of examples shows the API at work. Your scripts are saved to your
// account when the page can, else in this browser.
// ---------------------------------------------------------------------------------------------
'use strict';

// ---- Presets as JavaScript --------------------------------------------------------------------
function jsLit(v) {
  if (v === null || v === undefined) return 'null';
  if (typeof v === 'number') return String(+v.toFixed(4));
  if (typeof v === 'boolean') return String(v);
  if (typeof v === 'string') return "'" + v.replace(/\\/g, '\\\\').replace(/'/g, "\\'") + "'";
  if (Array.isArray(v)) return '[' + v.map(jsLit).join(', ') + ']';
  const ents = Object.entries(v).filter(([, x]) => x !== undefined);
  return ents.length ? '{ ' + ents.map(([k, x]) => (/^[A-Za-z_$][\w$]*$/.test(k) ? k : jsLit(k)) + ': ' + jsLit(x)).join(', ') + ' }' : '{}';
}
function compJS(c) {
  const p = c.params || {}, o = {};
  if (c.type === 'radial' && p.angle) o.angle = p.angle;
  if (c.op && c.op !== 'add') o.op = c.op;
  if (c.invert) o.invert = true;
  if (c.amount != null && c.amount !== 1) o.amount = c.amount;
  if (c.name && COMP_KINDS[c.type] && c.name !== COMP_KINDS[c.type].label) o.name = c.name;
  const tail = Object.keys(o).length ? ', ' + jsLit(o) : '', args = a => a.map(jsLit).join(', ');
  if (c.type === 'linear') return `lin(${args([p.x0, p.y0, p.x1, p.y1])}${tail})`;
  if (c.type === 'radial') return `rad(${args([p.cx, p.cy, p.rx, p.ry, p.feather])}${tail})`;
  if (c.type === 'luminance') return `lumR(${args([p.lo, p.hi, p.soft])}${tail})`;
  if (c.type === 'color') return `colR(${args([p.color, p.tol, p.soft])}${tail})`;
  return jsLit(Object.assign({ type: c.type, params: p }, o));
}
function presetSource(p) {
  const mine = !p.builtin, d = KINDS;
  const layer = s => {
    const o = [`kind: ${jsLit(s.kind)}`], def = defaults(s.kind), pr = {};
    if (s.name && s.name !== d[s.kind].label) o.push(`name: ${jsLit(s.name)}`);
    for (const [k, v] of Object.entries(s.params || {})) if (JSON.stringify(v) !== JSON.stringify(def[k])) pr[k] = v;
    if (Object.keys(pr).length) o.push(`params: ${jsLit(pr)}`);
    if (s.opacity != null && s.opacity !== 1) o.push(`opacity: ${jsLit(s.opacity)}`);
    if (s.blend && s.blend !== 'normal') o.push(`blend: ${jsLit(s.blend)}`);
    let line = '    { ' + o.join(', ');
    if (s.mask && s.mask.comps && s.mask.comps.length) {
      const comps = s.mask.comps.map(compJS), plain = (s.mask.density ?? 1) === 1 && !s.mask.invert;
      line += ',\n      mask: ' + (plain ? `M(${comps.join(', ')})` : `{ density: ${jsLit(s.mask.density ?? 1)}, invert: ${!!s.mask.invert}, comps: [${comps.join(', ')}] }`);
    }
    return line + ' },';
  };
  const head = mine
    ? `// ${p.id} ${p.name}, one of your presets. Change it here and run it (⌘↵ or Ctrl+Enter) to update it.\n`
    : `// ${p.id} ${p.name}, a built-in preset. Running this (⌘↵ or Ctrl+Enter) saves it as one of\n// your presets, called '${p.name} (mine)'; run it again after a change to update that copy.\n`;
  const call = mine ? `presets.save({ id: ${jsLit(p.id)}, name: ${jsLit(p.name)},` : `presets.put(${jsLit(p.name + ' (mine)')}, {`;
  return head + `// A preset is a list of layers: a kind, its settings, and a mask made with M(...) from lin, rad,
// lumR and colR components. The API button lists every kind, parameter and helper.
const p = ${call}
  about: ${jsLit(p.about || '')},
  layers: [
${p.layers.map(layer).join('\n')}
  ],
});
presets.select(p.id);
// presets.apply(p.id);   // uncomment to put it on the document too
return p;
`;
}

// ---- Presets as programs: the code that runs ------------------------------------------------------
// presetProgram writes a preset out as code layers holding the engine's own code for each kind,
// followed by the engine helpers that code calls, so the file is the whole of what the preset does.
const FORK_HELPERS = () => ({ toLin, toSrgb, lum, smooth, s01, hexToRgb, hash2, hue2rgb, sat3, clipColor, setLum, setSat, curveLut, lutAt, blurReach, boxSizes, boxH, boxH4, transpose, gaussBlur, unsharp });
const CODE_PROGRAM_HELP = `## Programs: layers written as code
A file whose lines start with preset(...) or layer(name, {...}) is a program. Running it compiles the
layers, saves the preset, and puts the layers on the stage (running again updates them in place).

preset(name, { about, apply = true })
layer(name, {
  params: { key: [min, max, value, label?, unit?, step?] | '#rrggbb' | { color, label } | { choose: [...], value, label } },
  // one of:
  fn(b, n, p, ctx) { },          // pointwise: b holds n pixels of straight RGBA floats, 0 to 1; change them in place
  roiFn(buf, w, h, p, L) { },    // reads neighbours: buf is a premultiplied w × h tile including an apron of
  reach: (p, L) => pixels,       //   reach(p, L) pixels on each side; change it in place
  paint(src, n, p, ctx) { },     // a source: write straight RGBA with alpha; it is composited by blend and opacity
  prep: p => value,              // optional: computed once per change of settings, passed as ctx.prep
  mask: M(lin(...), rad(...)), blend: 'normal', opacity: 1,   // optional
})
p holds the layer's settings by key. ctx: x0, y0 (the tile's corner at this level), w (and h for paint),
s (2^L, the level's scale), W, H (the picture's size at full resolution), L, prep.
Every engine function is in scope: toLin, toSrgb, lum, clamp, gaussBlur, unsharp, curveLut, hexToRgb and
the rest. A function of the same name declared in the file is used instead, for that file's layers only.
A layer that throws passes its backdrop through, and the error is shown here and in Params.`;
function fnText(f) { return dedent(f.toString()); }
function indentRest(t, pad) { return t.split('\n').map((l, i) => i ? pad + l : l).join('\n'); }
function paramLit(k, spec, v) {
  if (spec.type === 'text') return null;
  if (spec.type === 'color') return spec.label && spec.label !== k ? `{ color: ${jsLit(v)}, label: ${jsLit(spec.label)} }` : jsLit(v);
  if (spec.type === 'choice') return `{ choose: ${jsLit(spec.options)}, value: ${jsLit(v)}${spec.label && spec.label !== k ? ', label: ' + jsLit(spec.label) : ''} }`;
  const a = [spec.min, spec.max, v, spec.label || k], defStep = spec.max - spec.min > 20 ? 1 : spec.max - spec.min > 2 ? 0.01 : 0.001;
  if (spec.pct) a.push('%'); else if (spec.unit || (spec.step && spec.step !== defStep)) a.push(spec.unit || '');
  if (!spec.pct && spec.step && spec.step !== defStep) a.push(spec.step);
  return jsLit(a);
}
function presetProgram(p) {
  const layers = p.layers || [];
  if (layers.some(s => s.kind === 'code')) {
    const srcs = [...new Set(layers.filter(s => s.kind === 'code').map(s => s.params.src))];
    if (srcs.length === 1 && layers.every(s => s.kind === 'code')) return srcs[0];
    throw new Error(`${p.name} mixes code layers with built-in ones; open each code layer from Params instead`);
  }
  const helpers = FORK_HELPERS(), used = [], seen = new Set();
  const scan = text => { for (const m of text.matchAll(/\b([A-Za-z_$][\w$]*)\b/g)) { const n = m[1]; if (helpers[n] && !seen.has(n)) { seen.add(n); used.push(n); scan(helpers[n].toString()); } } };
  const name = p.builtin ? p.name + ' (code)' : p.name;
  const body = layers.map(s => {
    const K = KINDS[s.kind], vals = Object.assign(defaults(s.kind), s.params || {}), out = [];
    const ps = Object.entries(K.params).map(([k, spec]) => { const v = paramLit(k, spec, vals[k]); return v == null ? null : `${/^[A-Za-z_$][\w$]*$/.test(k) ? k : jsLit(k)}: ${v}`; }).filter(Boolean);
    out.push(`  params: { ${ps.join(', ')} },`);
    if (s.mask && s.mask.comps && s.mask.comps.length) { const comps = s.mask.comps.map(compJS); out.push(`  mask: ${(s.mask.density ?? 1) === 1 && !s.mask.invert ? `M(${comps.join(', ')})` : `{ density: ${jsLit(s.mask.density ?? 1)}, invert: ${!!s.mask.invert}, comps: [${comps.join(', ')}] }`},`); }
    if (s.blend && s.blend !== 'normal') out.push(`  blend: ${jsLit(s.blend)},`);
    if (s.opacity != null && s.opacity !== 1) out.push(`  opacity: ${jsLit(s.opacity)},`);
    const fns = [];
    if (K.paint) fns.push(fnText(K.paint));
    else if (K.roi) { fns.push('reach: ' + fnText(K.reach)); fns.push(fnText(K.roiFn)); }
    else {
      if (K.prep) fns.push(helpers[K.prep.name] === K.prep ? `prep: p => ${K.prep.name}(p)` : fnText(K.prep));
      fns.push(fnText(K.fn));
    }
    fns.forEach(t => { scan(t); out.push('  ' + indentRest(t, '  ') + ','); });
    const what = K.paint ? 'a source: paint() writes its colour and alpha' : K.roi ? 'reads neighbours: roiFn() works on the tile and its apron' : 'pointwise: fn() changes each pixel on its own';
    return `// ${s.name} · the engine's ${K.label.toLowerCase()} kind (${s.kind}), ${what}\nlayer(${jsLit(s.name || K.label)}, {\n${out.join('\n')}\n});`;
  }).join('\n\n');
  const help = used.map(n => { const t = fnText(helpers[n]); return t.startsWith('function') ? t : `const ${n} = ${t};`; }).join('\n');
  return `// ${p.id} ${p.name}${p.builtin ? ', a built-in preset' : ''}, as the code the engine runs for it.
// Each layer() is one layer of the preset: its settings ([min, max, value, label]) and the engine's
// own function for its kind, copied here. Below the layers are the engine helpers those functions call.
// Change anything and run it (⌘↵ or Ctrl+Enter): it is saved as the preset '${name}' and put on
// the stage; run again and those layers update in place. The API button explains the signatures.

preset(${jsLit(name)}, { about: ${jsLit(p.about || '')} });

${body}
${used.length ? `
// ---- Engine helpers the layers above call, copied from the engine --------------------------------
// They belong to this file now: change one and only this preset changes. Delete one to use the
// engine's own again. (Others they call, such as clamp and scratch, come from the engine.)
${help}
` : ''}`;
}
const CODE_LAYER_STARTER = `// A code layer: your own JavaScript, run by the engine on every tile of the picture.
// Run it (⌘↵ or Ctrl+Enter) to update the layer; its settings appear in Params as sliders.
layer('My layer', {
  params: { amount: [0, 1, 0.6, 'amount', '%'], warmth: [-1, 1, 0.2, 'warmth'] },
  fn(b, n, p) {                  // n pixels of straight RGBA floats from 0 to 1, changed in place
    for (let i = 0; i < n * 4; i += 4) {
      const g = lum(b[i], b[i + 1], b[i + 2]);                    // grey of the same brightness
      b[i] += (g - b[i]) * p.amount + 0.1 * p.warmth;
      b[i + 1] += (g - b[i + 1]) * p.amount;
      b[i + 2] += (g - b[i + 2]) * p.amount - 0.1 * p.warmth;
    }
  },
});
`;

// ---- Examples ---------------------------------------------------------------------------------
const CODE_EXAMPLES = [
  { id: 'E-01', name: 'tour.js', about: 'read the document: layers, a full render, a pixel', code: `// A tour of the lab API. Run it with ⌘↵ (Ctrl+Enter). Whatever you return is shown below as YAML,
// and log(...) lines appear above it. The agent works through exactly this API.
const d = doc();
log('document', d.code, d.title, d.width + ' × ' + d.height + ' px');
for (const l of layers()) log(l.index, l.name, '·', l.kind, l.blend, l.mask ? '· ' + l.mask.components.length + ' mask components' : '');
return {
  full_render: render(),                       // every tile at full size, timed
  middle_pixel: sample(d.width / 2, d.height / 2),
  cache: engine().cache_budget_MB + ' MB budget',
};
` },
  { id: 'E-02', name: 'preset-in-code.js', about: 'write a preset with masks, save it, apply it', code: `// A preset is data: a list of layers, each a kind, settings and an optional mask.
// M(...) builds a mask from components: lin (linear), rad (radial), lumR (luminance range) and
// colR (colour range). Positions and radii are fractions of the picture's width and height.
// presets.put saves it as one of yours, or updates the one with that name: run it twice, still one.
const p = presets.put('Evening light', {
  about: 'warmer, a glow on the right, and a darker sky that spares the sun',
  layers: [
    { kind: 'temperature', name: 'Warm', params: { temp: 18 } },
    { kind: 'exposure', name: 'Glow', params: { ev: 0.4 },
      mask: M(rad(0.66, 0.55, 0.25, 0.3, 0.9)) },
    { kind: 'exposure', name: 'Sky', params: { ev: -0.5 },
      mask: M(lin(0.5, 0.1, 0.5, 0.55), rad(0.66, 0.55, 0.12, 0.18, 0.8, { op: 'subtract' })) },
  ],
});
presets.select(p.id);
presets.apply(p.id);      // one undo step: Revert below takes it off again
return p;
` },
  { id: 'E-03', name: 'masks.js', about: 'build a mask from components on a new layer', code: `// Masks from code: an exposure through a radial component, minus a smaller one,
// intersected with a linear ramp. Every call is one undo step.
const l = add('exposure', { name: 'Lift the hills', params: { ev: 0.6 } });
mask.radial(l.id, { cx: 0.3, cy: 0.62, rx: 0.3, ry: 0.16, feather: 0.7 });
mask.radial(l.id, { cx: 0.3, cy: 0.62, rx: 0.08, ry: 0.05, feather: 0.5, op: 'subtract' });
mask.gradient(l.id, [0.5, 0.78], [0.5, 0.5], 'intersect');
activate(l.id, 'mask');   // shows the component handles on the stage
return mask.components(l.id);
` },
  { id: 'E-04', name: 'try-every-preset.js', about: 'apply each built-in preset, time it, take it off', code: `// Apply each built-in preset, time a full render with it on, and take it off again.
// The history ends where it started.
const out = [];
for (const p of presets.list().filter(p => p.builtin)) {
  presets.apply(p.id);
  const r = render();
  out.push({ preset: p.id + ' ' + p.name, ms: r.ms, tiles: r.tiles_computed });
  history.undo();
  await sleep(0);         // let the page draw between presets
}
out.sort((a, b) => b.ms - a.ms);
log('slowest first; layers below the preset come from the cache');
return out.slice(0, 12);
` },
  { id: 'E-05', name: 'select-and-mask.js', about: 'select with the wand, turn it into a mask', code: `// Select the sun's reflection with the wand, feather it, and make it the mask of a new layer.
// Coordinates are in picture pixels.
const d = doc();
const sel = select.wand(Math.round(d.width * 0.66), Math.round(d.height * 0.75), { tolerance: 48, contiguous: false, feather: 3 });
const l = add('exposure', { name: 'Reflection', params: { ev: 0.5 } });
mask.add(l.id, 'selection');
select.none();
return { selected: sel, layer: layer(l.id).mask };
` },
  { id: 'E-06', name: 'bench-from-code.js', about: 'run bench scenarios and keep the numbers', code: `// Run bench scenarios from code. This takes a minute; the Bench shows progress.
// exports.benchCSV(r.run) would then offer the table as a file.
const r = await bench.run({
  scenarios: ['scrub-top', 'stroke-low'],
  profiles: ['here'], tiles: [64, 128, 256], previews: ['off'], evictions: ['lru'],
});
return r.rows.map(x => ({ scenario: x.scenario, tile: x.tile, p50_ms: x.update_p50_ms, p95_ms: x.update_p95_ms }));
` },
  { id: 'E-07', name: 'probes.js', about: 'WebAssembly and WebGPU probes', code: `// The WebAssembly probe runs anywhere; the GPU probes need WebGPU.
const out = { wasm: await wasm.probe(8, 5) };
try { out.gpu_blur = await gpu.blur(8, 5); }
catch (e) { out.gpu_blur = 'not here: ' + e.message; }
return out;
` },
  { id: 'E-08', name: 'code-layers.js', about: 'a program: two layers written as code', code: `// A program: layers written as code. Each layer() is one layer; Run puts them on the stage, and
// running again after a change updates them in place. Their params appear in Params as sliders.
preset('Glow and dots', { about: 'two code layers: a glow that reads neighbours, and a dot screen' });

layer('Glow', {
  params: { radius: [1, 40, 12, 'radius', 'px'], amount: [0, 1, 0.5, 'amount', '%'] },
  reach: (p, L) => blurReach(p.radius / 2 ** L) + 1,   // how far beyond the tile roiFn reads
  roiFn(buf, w, h, p, L) {                              // premultiplied RGBA: the tile plus its apron
    const soft = buf.slice(); gaussBlur(soft, w, h, 4, p.radius / 2 ** L);
    for (let i = 0; i < w * h * 4; i += 4) for (let c = 0; c < 3; c++) {
      const a = buf[i + c], s = soft[i + c];
      buf[i + c] = a + (1 - (1 - a) * (1 - s) - a) * p.amount;   // screen the blur over the original
    }
  },
});

layer('Dots', {
  params: { size: [3, 40, 10, 'cell size', 'px'], angle: [0, 90, 45, 'angle', '°'] },
  blend: 'multiply', opacity: 0.5,
  fn(b, n, p, ctx) {                                     // straight RGBA, n pixels of one tile
    const s = p.size / ctx.s, a = p.angle * Math.PI / 180, co = Math.cos(a), si = Math.sin(a);
    for (let j = 0, i = 0; j < n; j++, i += 4) {
      const x = ctx.x0 + (j % ctx.w), y = ctx.y0 + Math.floor(j / ctx.w);
      const u = (x * co + y * si) / s, v = (-x * si + y * co) / s;
      const du = u - Math.round(u), dv = v - Math.round(v);
      const r = 0.62 * Math.sqrt(1 - lum(b[i], b[i + 1], b[i + 2]));   // darker: a bigger dot
      b[i] = b[i + 1] = b[i + 2] = du * du + dv * dv < r * r ? 0 : 1;
    }
  },
});
` },
].map(e => Object.assign(e, { kind: 'example' }));

// ---- The editor: a highlighted <pre> under a transparent <textarea> ------------------------------
function codeEditor({ onChange, onRun, onSave, onCursor } = {}) {
  const ta = h('textarea', { spellcheck: 'false', autocapitalize: 'off', autocomplete: 'off', autocorrect: 'off', wrap: 'off', 'aria-label': 'Script' });
  const pre = h('pre', { 'aria-hidden': 'true' }), gin = h('div', { class: 'gutter-in', 'aria-hidden': 'true' });
  const el = h('div', { class: 'ed' }, h('div', { class: 'egutter' }, gin), h('div', { class: 'code' }, pre, ta));
  let err = 0, hl = 0, nLines = -1;
  const sync = () => { pre.style.transform = `translate(${-ta.scrollLeft}px, ${-ta.scrollTop}px)`; gin.style.transform = `translateY(${-ta.scrollTop}px)`; };
  const paint = () => {
    const lines = ta.value.split('\n');
    pre.innerHTML = lines.map((l, i) => `<div${i + 1 === err ? ' class="ln-err"' : i + 1 === hl ? ' class="ln-hl"' : ''}>${highlight(l)}</div>`).join('');
    if (lines.length !== nLines || err) { nLines = lines.length; gin.innerHTML = lines.map((_, i) => `<div${i + 1 === err ? ' class="err"' : ''}>${i + 1}</div>`).join(''); }
    sync();
  };
  const cursor = () => { if (!onCursor) return; const v = ta.value, s = ta.selectionStart, before = v.slice(0, s), ln = before.split('\n').length, col = s - before.lastIndexOf('\n'); onCursor(ln, col, v.split('\n').length, ta.selectionEnd - s); };
  const insert = text => { ta.focus(); if (!document.execCommand('insertText', false, text)) { ta.setRangeText(text, ta.selectionStart, ta.selectionEnd, 'end'); ta.dispatchEvent(new Event('input')); } };
  const indent = out => {
    const v = ta.value, s = ta.selectionStart, e = ta.selectionEnd;
    if (s === e && !out) { insert('  '); return; }
    const a = v.lastIndexOf('\n', s - 1) + 1, bEnd = v.indexOf('\n', e - (e > s && v[e - 1] === '\n' ? 1 : 0)), b = bEnd < 0 ? v.length : bEnd;
    const block = v.slice(a, b), next = block.split('\n').map(l => out ? l.replace(/^ {1,2}/, '') : '  ' + l).join('\n');
    ta.setSelectionRange(a, b); insert(next); ta.setSelectionRange(a, a + next.length);
  };
  ta.addEventListener('input', () => { if (err) { err = 0; nLines = -1; } paint(); cursor(); onChange && onChange(ta.value); });
  ta.addEventListener('scroll', sync);
  ['keyup', 'click', 'select', 'focus'].forEach(ev => ta.addEventListener(ev, cursor));
  ta.addEventListener('keydown', e => {
    const mod = e.metaKey || e.ctrlKey;
    if (mod && e.key === 'Enter') { e.preventDefault(); onRun && onRun(); return; }
    if (mod && e.key.toLowerCase() === 's') { e.preventDefault(); onSave && onSave(); return; }
    if (e.key === 'Tab' && !mod && !e.altKey) { e.preventDefault(); indent(e.shiftKey); return; }
    if (e.key === 'Enter' && !mod && !e.altKey && !e.shiftKey) {
      e.preventDefault(); const v = ta.value, s = ta.selectionStart, cur = v.slice(v.lastIndexOf('\n', s - 1) + 1, s);
      insert('\n' + cur.match(/^\s*/)[0] + (/[{[(]\s*$/.test(cur) ? '  ' : '')); return;
    }
    if ('}])'.includes(e.key) && !mod && !e.altKey) {   // a closing bracket at the start of a line steps back one level
      const v = ta.value, s = ta.selectionStart, a = v.lastIndexOf('\n', s - 1) + 1, before = v.slice(a, s);
      if (s === ta.selectionEnd && /^ {2,}$/.test(before)) { e.preventDefault(); ta.setSelectionRange(a, s); insert(before.slice(2) + e.key); return; }
    }
    if (e.key === 'Escape') ta.blur();   // so that Tab can leave the editor
  });
  return {
    el, ta,
    get value() { return ta.value; },
    set value(v) { ta.value = v; err = 0; hl = 0; nLines = -1; ta.scrollTop = 0; ta.scrollLeft = 0; paint(); },
    // Put the caret at the start of a line, mark the line, and scroll it into view.
    goto(line) { const ls = ta.value.split('\n'); line = clamp(line | 0, 1, ls.length); const at = ls.slice(0, line - 1).reduce((n, x) => n + x.length + 1, 0); hl = line; ta.focus(); ta.setSelectionRange(at, at); ta.scrollTop = Math.max(0, (line - 1) * 18 - ta.clientHeight / 3); paint(); sync(); if (onCursor) onCursor(line, 1, ls.length, 0); },
    setError(line) { err = line || 0; nLines = -1; paint(); if (line) { const top = (line - 1) * 18 - ta.clientHeight / 3; ta.scrollTop = Math.max(0, top); sync(); } },
    focus() { ta.focus(); }, paint,
  };
}

// ---- The view -----------------------------------------------------------------------------------
const Code = {
  files: [], scratch: [], edits: new Map(), tabs: ['E-01'], cur: 'E-01', ref: null, where: 'saved in this browser',
  busy: false, readOnly: false, last: null, showRef: false, renaming: null, trash: null,
  bound: new Map(), lastSrc: new Map(),
  get(id) {
    const f = this.files.find(x => x.id === id), e = CODE_EXAMPLES.find(x => x.id === id), t = this.scratch.find(x => x.id === id);
    if (f) return { id, name: f.name, kind: 'file', code: f.code };
    if (e) return { id, name: e.name, kind: 'example', code: this.edits.has(id) ? this.edits.get(id) : e.code, edited: this.edits.has(id) && this.edits.get(id) !== e.code, about: e.about };
    if (t) return { id, name: t.name, kind: 'scratch', code: t.code, edited: true };
    return null;
  },
  // ---- persistence: the same pattern as presets ----
  async load() {
    this.files = store.get('scripts', []); const st = store.get('code.tabs', null);
    if (st) { this.tabs = st.tabs.filter(id => this.get(id)); this.cur = this.get(st.cur) ? st.cur : this.tabs[0] || 'E-01'; if (!this.tabs.length) this.tabs = ['E-01']; }
    this.render();
    this.ref = await userDoc('scripts'); if (!this.ref) return;
    try {
      const snap = await Promise.race([this.ref.get(), sleep(10000).then(() => null)]);
      if (snap && snap.exists && Array.isArray(snap.data().list)) { this.files = JSON.parse(JSON.stringify(snap.data().list)); store.set('scripts', this.files); }
      this.where = 'saved to your account';
    } catch (e) { this.ref = null; }
    this.tabs = this.tabs.filter(id => this.get(id)); if (!this.tabs.includes(this.cur)) this.cur = this.tabs[0] || 'E-01'; if (!this.tabs.length) this.tabs = ['E-01'];
    this.render();
  },
  persist: debounce(async function () {
    store.set('scripts', Code.files);
    if (!Code.ref) return;
    if (Code.writing) { Code.again = true; return; }
    Code.writing = true;
    try { await Code.ref.set({ list: JSON.parse(JSON.stringify(Code.files)), updated: Date.now() }); }
    catch (e) { toast('Scripts are kept in this browser only: ' + (e.message || e.code), true); Code.ref = null; Code.where = 'saved in this browser'; Code.renderFoot(); }
    Code.writing = false; if (Code.again) { Code.again = false; Code.persist(); }
  }, 700),
  saveTabs() { store.set('code.tabs', { tabs: this.tabs.filter(id => !id.startsWith('T-')), cur: this.cur }); },
  nextId(prefix, list) { let n = 1; while (list.some(x => x.id === prefix + String(n).padStart(2, '0'))) n++; return prefix + String(n).padStart(2, '0'); },
  uniqueName(name) { if (!this.files.some(f => f.name === name)) return name; const m = /^(.*?)(\.[\w.]+)?$/.exec(name); let n = 2; while (this.files.some(f => f.name === `${m[1]}-${n}${m[2] || ''}`)) n++; return `${m[1]}-${n}${m[2] || ''}`; },
  // ---- files and tabs ----
  open(id) { if (!this.get(id)) return; if (!this.tabs.includes(id)) this.tabs.push(id); if (this.cur !== id) this.pos = null; this.cur = id; this.last = null; this.saveTabs(); this.render(); Dock.focus('Code'); },
  close(id) {
    const i = this.tabs.indexOf(id); if (i < 0) return;
    this.tabs.splice(i, 1); if (id.startsWith('T-')) this.scratch = this.scratch.filter(t => t.id !== id); else if (this.edits.has(id)) this.edits.delete(id);
    if (!this.tabs.length) this.tabs = ['E-01'];
    if (this.cur === id) this.cur = this.tabs[Math.min(i, this.tabs.length - 1)];
    this.saveTabs(); this.render();
  },
  newFile(name, code) {
    const f = { id: this.nextId('S-', this.files), name: this.uniqueName(name || 'untitled.js'), code: code ?? '// A new script. ⌘↵ (Ctrl+Enter) runs it; the API button lists what you can call.\n\nreturn layers().map(l => l.name);\n', updated: Date.now() };
    this.files.push(f); this.persist(); this.open(f.id); return f;
  },
  openScratch(name, code) {
    let t = this.scratch.find(x => x.name === name);
    if (t) t.code = code; else { t = { id: this.nextId('T-', this.scratch), name, code }; this.scratch.push(t); }
    if (this.ed && this.cur === t.id) this.ed.value = code;
    this.open(t.id); return t;
  },
  // A preset as the program that runs it: its own code if it was written as one, else the engine's.
  openPreset(id) { const p = Presets.get(id); if (!p) throw new Error(`no preset '${id}'`); return this.openScratch(`${p.id}.preset.js`, p.code || presetProgram(p)); },
  // A code layer's program, bound to every layer in the document that runs it.
  openLayer(id) {
    const l = app.ws.layer(id); if (!l || l.kind !== 'code') throw new Error('that is not a code layer');
    const src = l.params.src, t = this.openScratch(`${(l.params.preset || l.params.part || 'layer').replace(/[^\w.-]+/g, '-').toLowerCase()}.layer.js`, src);
    this.bound.set(t.id, app.ws.doc.layers.filter(x => x.kind === 'code' && x.params.src === src).map(x => x.id)); this.lastSrc.set(t.id, src);
    return t;
  },
  // + Layer › Code: a new code layer from the starter program, opened here.
  newCodeLayer() {
    const src = CODE_LAYER_STARTER, part = codeModule(src).parts[0];
    const l = app.ws.addLayer('code', { name: part.name, params: Object.assign(defaultsOf('code', { src, part: part.name }), { src, part: part.name }) });
    app.ws.setActive(l.id); this.openLayer(l.id); return l;
  },
  // Show a line of the current file: the way back from a layer to the code that made it.
  goto(line) { requestAnimationFrame(() => this.ed && this.ed.goto(line)); },
  isProgram(code) { return /^\s*preset\s*\(/m.test(code) || /^\s*layer\s*\(\s*(['"`])[^\n]*?\1\s*,\s*\{/m.test(code); },
  // Run a program: compile it, save its preset, and add or update its layers in the document.
  runProgram(d) {
    const t0 = performance.now(); CODE_MODULES.delete(d.code); const m = codeModule(d.code);
    if (m.error) return { error: true, text: `Error: ${m.error.message}${m.error.line ? ` (line ${m.error.line})` : ''}`, line: m.error.line, ms: performance.now() - t0 };
    const ws = app.ws, meta = m.meta, out = { compiled: m.parts.map(pt => `${pt.name}: ${pt.src ? 'paint' : pt.roi ? 'roiFn' : 'fn'}${Object.keys(pt.params).length ? ' · ' + Object.keys(pt.params).join(', ') : ''}`) };
    for (const pt of m.parts) if (pt.blend && !BLEND_MODES.includes(pt.blend)) return { error: true, text: `Error: layer '${pt.name}': no blend mode '${pt.blend}'`, ms: performance.now() - t0 };
    const seen = new Map(), srcOf = pt => { const n = seen.get(pt.name) || 0; seen.set(pt.name, n + 1); return Provenance.stamp('program', { file: d.name, fileId: d.id, line: Provenance.programLine(d.code, pt.name, n), preset: meta ? meta.name : undefined }); };
    const specs = m.parts.map(pt => ({ kind: 'code', name: pt.name, params: Object.assign(defaultsOf('code', { src: d.code, part: pt.name }), { src: d.code, part: pt.name, preset: meta ? meta.name : '' }), opacity: pt.opacity ?? 1, blend: pt.blend || 'normal', mask: pt.mask, meta: { source: srcOf(pt) } }));
    if (meta) {
      const ex = Presets.user.find(x => x.name === meta.name);
      const saved = Presets.save({ id: ex && ex.id, name: meta.name, about: meta.about || '', layers: specs.map(s => Object.assign({}, s, { mask: s.mask || undefined })), code: d.code });
      out.preset = `${saved.id} ${saved.name} (${ex ? 'updated' : 'saved'})`;
    }
    const bound = this.bound.get(d.id) || [], last = this.lastSrc.get(d.id);
    const targets = ws.doc.layers.filter(l => l.kind === 'code' && (bound.includes(l.id) || (meta && l.params.preset === meta.name) || (last && l.params.src === last)));
    if (meta && meta.apply === false && !targets.length) out.stage = 'not applied (apply: false)';
    else if (targets.length === specs.length) { targets.forEach((l, i) => { ws.replaceLayer(l.id, Object.assign({}, specs[i], { mask: specs[i].mask === undefined ? undefined : specs[i].mask }), `Code: ${specs[i].name}`); l.meta = Object.assign({}, l.meta || {}, specs[i].meta); }); out.stage = `updated ${targets.length} layer${targets.length === 1 ? '' : 's'}`; this.bound.set(d.id, targets.map(l => l.id)); }
    else {
      // Replacing: new layers go where the old ones were, inside the program's group if it made one.
      const home = targets.length ? targets[targets.length - 1].parent : null, homeG = home && ws.layer(home);
      targets.forEach(l => ws.removeLayer(l.id));
      const layerSpecs = specs.map(s => [s.kind, { name: s.name, params: s.params, opacity: s.opacity, blend: s.blend, mask: s.mask || null, meta: s.meta }]);
      const made = ws.addLayers(layerSpecs, `Code: ${meta ? meta.name : d.name}`, homeG ? { parent: homeG.id } : specs.length > 1 ? { group: { name: meta ? meta.name : d.name.replace(/\.js$/, ''), meta: { source: Provenance.stamp('program', { file: d.name, fileId: d.id, line: 1, preset: meta ? meta.name : undefined }) } } } : {});
      this.bound.set(d.id, made.map(l => l.id)); out.stage = `${targets.length ? 'replaced ' + targets.length + ' with' : 'added'} ${made.length} layer${made.length === 1 ? '' : 's'}`;
    }
    this.lastSrc.set(d.id, d.code);
    // A quick render at level 2, so that an error in fn shows now rather than on the next frame.
    m.runtime = null; const eng = ws.engine, D = eng.dims(2), r0 = performance.now(); eng.invalidate(); eng.rect(eng.topIndex(), 2, 0, 0, D.w, D.h); out.preview_ms = +(performance.now() - r0).toFixed(1);
    if (m.runtime) return { error: true, text: `Error in layer '${m.runtime.part}': ${m.runtime.message}${m.runtime.line ? ` (line ${m.runtime.line})` : ''}\nThe layer passes its backdrop through until this is fixed.\n---\n` + yaml(out), line: m.runtime.line, ms: performance.now() - t0 };
    if (meta) Presets.select(Presets.user.find(x => x.name === meta.name).id);
    return { error: false, text: yaml(out), ms: performance.now() - t0, line: null };
  },
  saveCurrent() {
    const d = this.get(this.cur); if (!d) return;
    if (d.kind === 'file') { this.persist(); toast(`${d.name} is ${this.where}`); return; }
    const f = this.newFile(d.name, d.code);
    if (d.kind === 'scratch') { this.scratch = this.scratch.filter(t => t.id !== d.id); } else this.edits.delete(d.id);
    this.tabs = this.tabs.filter(x => x !== d.id); this.cur = f.id; this.saveTabs(); this.render();
    toast(`Saved ${f.name} as one of your scripts`);
  },
  edit(code) {
    const d = this.get(this.cur); if (!d) return;
    if (d.kind === 'file') { const f = this.files.find(x => x.id === d.id); f.code = code; f.updated = Date.now(); this.persist(); }
    else if (d.kind === 'scratch') this.scratch.find(x => x.id === d.id).code = code;
    else this.edits.set(d.id, code);
    this.renderTabs(); this.renderFoot();
  },
  rename(id, name) {
    const f = this.files.find(x => x.id === id), t = this.scratch.find(x => x.id === id); name = String(name || '').trim(); if (!name) return;
    if (f && name !== f.name) { f.name = this.uniqueName(name); this.persist(); } else if (t) t.name = name;
    this.renaming = null; this.render();
  },
  remove(id) {
    const i = this.files.findIndex(x => x.id === id); if (i < 0) return;
    this.trash = this.files.splice(i, 1)[0]; this.persist(); this.close(id);
    toast(`Deleted ${this.trash.name}. Files ▾ › Restore brings it back.`);
  },
  restore() { if (!this.trash) return; this.files.push(this.trash); const id = this.trash.id; this.trash = null; this.persist(); this.open(id); },
  importFile() {
    const inp = h('input', { type: 'file', accept: '.json,.js,.txt,application/json,text/plain', style: { display: 'none' } });
    inp.onchange = async () => {
      const f = inp.files[0]; inp.remove(); if (!f) return;
      try {
        const text = await f.text();
        if (/\.json$/i.test(f.name)) { const d = JSON.parse(text), list = d.scripts || (Array.isArray(d) ? d : [d]); let n = 0; for (const s of list) if (s && typeof s.code === 'string') { this.newFile(s.name || 'imported.js', s.code); n++; } toast(`Imported ${n} script${n === 1 ? '' : 's'}`); }
        else this.newFile(f.name.replace(/\.txt$/i, ''), text);
      } catch (e) { toast('Could not import: ' + e.message, true); }
    };
    document.body.append(inp); inp.click();
  },
  // ---- running ----
  async run() {
    if (this.busy) return;
    const d = this.get(this.cur); if (!d) return;
    const ws = app.ws, h0 = ws.hist.ptr;
    this.busy = true; this.showRef = false; this.last = { pending: true, name: d.name }; this.renderRun(); this.renderOut();
    let r;
    if (this.isProgram(d.code)) { if (this.readOnly) r = { error: true, text: 'Error: read-only is on, and running a program changes the document', ms: 0 }; else { try { r = this.runProgram(d); } catch (e) { r = { error: true, text: 'Error: ' + e.message, ms: 0 }; } } r.program = true; }
    else r = await runLab(d.code, { readOnly: this.readOnly, timeout: 300000, maxText: 60000, source: { kind: 'script', file: d.name, fileId: d.id } });
    const steps = app.ws === ws ? ws.hist.ptr - h0 : 0;
    this.last = Object.assign(r, { name: d.name, id: d.id, at: new Date(), ws, h0, steps, ptr: ws.hist.ptr, mode: EXEC.mode });
    this.busy = false; if (this.cur === d.id) this.ed.setError(r.line || 0);
    this.renderRun(); this.renderOut(); this.renderFoot();
  },
  revert() {
    const r = this.last; if (!r || !r.steps) return;
    if (app.ws !== r.ws) { toast('The document has changed since that run', true); return; }
    if (app.ws.hist.ptr !== r.ptr) { toast('Other edits came after that run: use History to go back', true); return; }
    app.ws.jump(r.h0); toast(`Reverted ${r.name}: ${r.steps} step${r.steps === 1 ? '' : 's'}`); r.steps = 0; r.reverted = true; this.renderOut();
  },
  // ---- view ----
  create() {
    this.filesBtn = h('button', { class: 'btn sm', 'aria-haspopup': 'menu', onclick: e => openMenu(e.currentTarget, this.menuItems(), { cls: 'files' }) }, 'Files ▾');
    this.tabsEl = h('div', { class: 'filetabs', role: 'tablist', 'aria-label': 'Open scripts' });
    this.roBox = h('input', { type: 'checkbox', onchange: e => { this.readOnly = e.target.checked; } });
    this.runBtn = h('button', { class: 'btn sm primary', title: '⌘↵ or Ctrl+Enter', onclick: () => this.run() }, 'Run');
    const api = h('button', { class: 'btn sm', title: 'The lab API: every call a script can make', onclick: () => { this.showRef = !this.showRef; this.renderOut(); } }, 'API');
    const bar = h('div', { class: 'toolbar filebar' }, this.filesBtn, this.tabsEl, h('span', { class: 'sp' }), h('label', { class: 'ronly', title: 'Calls that change the document refuse to run' }, this.roBox, 'read-only'), api, this.runBtn);
    this.ed = codeEditor({ onChange: v => this.edit(v), onRun: () => this.run(), onSave: () => this.saveCurrent(), onCursor: (ln, col, n, sel) => { this.pos = { ln, col, n, sel }; this.renderFoot(); } });
    this.out = h('section', { class: 'cout', 'aria-live': 'polite' });
    this.foot = h('div', { class: 'ed-foot' });
    this.el = h('div', { class: 'tabbody col codeview' }, bar, this.ed.el, this.out, this.foot);
    this.render(); return this.el;
  },
  menuItems() {
    const d = this.get(this.cur), run = f => () => { try { f(); } catch (e) { toast(e.message, true); } };
    return [
      { group: 'Examples' }, ...CODE_EXAMPLES.map(e => ({ label: e.name, small: e.about, on: e.id === this.cur, run: () => this.open(e.id) })),
      { label: 'New code layer', small: 'a layer whose operation is your JavaScript', run: run(() => this.newCodeLayer()) },
      { group: `Yours · ${this.where}` },
      ...(this.files.length ? this.files.map(f => ({ label: f.name, small: f.code.split('\n').length + ' lines', on: f.id === this.cur, run: () => this.open(f.id) })) : [{ label: 'None yet', disabled: true }]),
      { sep: true },
      { label: 'New script', run: run(() => this.newFile()) },
      { label: d && d.kind === 'file' ? 'Save' : 'Save as one of yours', key: '⌘S', run: run(() => this.saveCurrent()) },
      { label: 'Rename', disabled: !d || d.kind === 'example', run: () => { this.renaming = this.cur; this.renderTabs(); } },
      { label: 'Copy to the clipboard', run: run(async () => { await navigator.clipboard.writeText(d.code); toast('Copied ' + d.name); }) },
      { label: 'Save this script as a file', small: '.txt: the downloads allowlist has no .js', run: run(() => Exports.save(d.name.replace(/\.js$/i, '') + '.js.txt', d.code, d.name)) },
      { label: 'Your scripts as JSON', disabled: !this.files.length, run: run(() => Exports.save(`scripts-${Exports.stamp()}.json`, JSON.stringify({ format: 'editor-lab scripts', version: 1, scripts: this.files }, null, 2), 'your scripts')) },
      { label: 'Import scripts…', small: 'a .json exported here, or a .js or .txt file', run: () => this.importFile() },
      { sep: true },
      { label: 'Open a preset as code…', small: 'from the Presets view: Edit as code', run: () => Dock.focus('Presets') },
      this.trash ? { label: 'Restore ' + this.trash.name, run: () => this.restore() } : null,
      d && d.kind === 'file' ? { label: 'Delete ' + d.name, danger: true, run: () => this.remove(d.id) } : null,
    ].filter(Boolean);
  },
  render() {
    if (!this.el) return;
    const d = this.get(this.cur) || this.get('E-01'); this.cur = d.id;
    if (this.ed.shown !== d.id || this.ed.value !== d.code) { this.ed.value = d.code; this.ed.shown = d.id; }
    if (this.last && this.last.id === d.id && this.last.line) this.ed.setError(this.last.line);
    this.renderTabs(); this.renderRun(); this.renderOut(); this.renderFoot();
  },
  renderTabs() {
    if (!this.tabsEl) return;
    this.tabsEl.replaceChildren(...this.tabs.map(id => {
      const d = this.get(id); if (!d) return null;
      if (this.renaming === id) {
        const inp = h('input', { value: d.name, 'aria-label': 'File name', onkeydown: e => { if (e.key === 'Enter') this.rename(id, inp.value); if (e.key === 'Escape') { this.renaming = null; this.renderTabs(); } }, onblur: () => this.rename(id, inp.value) });
        setTimeout(() => { inp.focus(); inp.select(); }, 0); return h('span', { class: 'ftab on' }, inp);
      }
      const mark = d.kind === 'example' ? (d.edited ? ' · edited' : '') : d.kind === 'scratch' ? ' · unsaved' : '';
      return h('span', { class: 'ftab' + (id === this.cur ? ' on' : '') },
        h('button', { role: 'tab', 'aria-selected': String(id === this.cur), title: d.kind === 'example' ? 'An example: edits are not kept unless you save a copy' : d.kind === 'scratch' ? 'Not saved: ⌘S keeps it as one of your scripts' : 'Double-click to rename', onclick: () => this.open(id), ondblclick: () => { if (d.kind !== 'example') { this.renaming = id; this.renderTabs(); } } }, d.name, mark ? h('span', { class: 'dim' }, mark) : null),
        h('button', { class: 'x', 'aria-label': 'Close ' + d.name, onclick: () => this.close(id) }, '×'));
    }).filter(Boolean));
  },
  renderRun() { if (this.runBtn) { this.runBtn.disabled = this.busy; this.runBtn.textContent = this.busy ? 'Running…' : 'Run'; } },
  renderOut() {
    if (!this.out) return;
    const o = this.out; o.textContent = ''; o.classList.toggle('idle', !this.last && !this.showRef && execMode() !== 'none');
    if (this.showRef) {
      o.append(h('header', null, h('b', null, 'The lab API'), h('span', null, 'the same calls the agent makes'), h('span', { class: 'sp' }), h('button', { class: 'link', onclick: () => { this.showRef = false; this.renderOut(); } }, 'close')),
        h('pre', { class: 'src' }, CODE_PROGRAM_HELP + '\n\n' + LAB_REFERENCE));
      return;
    }
    const r = this.last;
    if (!r) { o.append(h('header', null, h('b', null, 'Output'), h('span', null, '⌘↵ or Ctrl+Enter runs the script; it is an async function, so await works and return gives the result')), h('pre', { class: 'src dim' }, execMode() === 'none' ? 'This page cannot run scripts: its content security policy blocks them.' : '')); return; }
    if (r.pending) { o.append(h('header', null, h('b', null, 'Running ' + r.name + '…')), h('pre', { class: 'src dim' }, '')); return; }
    const head = h('header', null, h('b', { class: r.error ? 'bad' : '' }, r.error ? 'Error' : 'Result'), h('span', null, `${r.name} · ${fmtMs(r.ms)} ms · ${r.at.toLocaleTimeString()}`),
      r.steps > 0 ? h('span', null, `${r.steps} step${r.steps === 1 ? '' : 's'} in History`) : r.reverted ? h('span', null, 'reverted') : null,
      h('span', { class: 'sp' }), r.steps > 0 ? h('button', { class: 'btn sm', onclick: () => this.revert() }, 'Revert this run') : null,
      h('button', { class: 'link', onclick: async () => { try { await navigator.clipboard.writeText(r.text); toast('Copied the output'); } catch (e) { toast('Copying was refused', true); } } }, 'copy'),
      h('button', { class: 'link', onclick: () => { this.last = null; this.ed.setError(0); this.renderOut(); } }, 'clear'));
    o.append(head, h('pre', { class: 'src' + (r.error ? ' bad' : ''), html: r.error ? esc(r.text) : highlightYaml(r.text) }));
  },
  // The way back from a file: the layers on the stage whose source names it.
  madeHere(d) {
    const mine = Provenance.madeBy(s => (s.kind === 'program' || s.kind === 'script') && (s.fileId ? s.fileId === d.id : s.file === d.name));
    if (!mine.length) return null;
    const top = mine.find(l => !mine.some(o => o.id === l.parent)) || mine[0];
    return h('span', null, h('button', { class: 'link', title: 'Select it in Layers', onclick: () => { app.ws.setActive(top.id); Dock.focus('Layers'); } }, mine.length === 1 ? `made ${top.name}` : `made ${mine.length} layers`));
  },
  renderFoot() {
    if (!this.foot) return; const d = this.get(this.cur); if (!d) return; const p = this.pos || { ln: 1, col: 1, n: d.code.split('\n').length, sel: 0 };
    const kind = d.kind === 'file' ? `yours, ${this.where}` : d.kind === 'example' ? (d.edited ? 'example, edited: ⌘S saves a copy as yours' : 'example') : 'not saved: ⌘S keeps it as yours';
    const runs = { function: 'runs with Function()', script: 'runs as an inline script', none: 'cannot run code here' }[execMode()];
    this.foot.replaceChildren(...[h('b', null, `Ln ${p.ln}, Col ${p.col}`), h('span', null, `${p.n} lines${p.sel ? ' · ' + p.sel + ' selected' : ''}`), h('span', null, kind), h('span', null, this.isProgram(d.code) ? 'a program: Run saves and updates its layers' : 'a script: Run calls the lab API'), this.madeHere(d), h('span', { class: 'sp' }),
      h('span', null, this.readOnly ? 'read-only: edits refused' : 'edits go to History'), h('span', { class: execMode() === 'none' ? 'bad' : '' }, runs)].filter(Boolean));
  },
};
// YAML output: keys bold, comments and dashes dim.
function highlightYaml(t) { return esc(t).split('\n').map(l => l.replace(/^(\s*(?:- )?)([\w.$ -]+?):(\s|$)/, '$1<span class="tk-k">$2</span>:$3').replace(/^(\s*)(---|…[^\n]*)$/, '$1<span class="tk-c">$2</span>')).join('\n'); }
app.on('codeError', ({ module, error }) => {
  const d = Code.get(Code.cur); if (!d || d.code !== module.src || !Code.el) return;
  Code.last = { error: true, text: `Error in layer '${error.part}': ${error.message}${error.line ? ` (line ${error.line})` : ''}\nThe layer passes its backdrop through until this is fixed.`, name: d.name, id: d.id, at: new Date(), ms: 0, steps: 0, line: error.line };
  Code.ed.setError(error.line || 0); Code.renderOut();
});
registerView('Code', { title: 'Code', about: 'scripts against the lab API, presets as JavaScript, examples', create: () => Code.create(), badge: () => Code.busy ? { text: '●' } : null });
