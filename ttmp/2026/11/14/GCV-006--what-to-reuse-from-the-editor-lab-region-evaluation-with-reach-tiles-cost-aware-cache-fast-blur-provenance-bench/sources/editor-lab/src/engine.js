// ---------------------------------------------------------------------------------------------
// engine.js: rasters, layers, blend modes, adjustments, the tile cache and the tiled renderer.
// Colour is straight RGB in 0..1 inside an operation and premultiplied RGBA between operations.
// ---------------------------------------------------------------------------------------------
'use strict';

// ---- sRGB transfer, as two lookup tables (4,096 steps) -----------------------------------------
const LUTN = 4096, S2L = new Float32Array(LUTN + 1), L2S = new Float32Array(LUTN + 1);
for (let i = 0; i <= LUTN; i++) {
  const c = i / LUTN;
  S2L[i] = c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  L2S[i] = c <= 0.0031308 ? 12.92 * c : 1.055 * Math.pow(c, 1 / 2.4) - 0.055;
}
const toLin = c => c <= 0 ? 0 : c >= 1 ? 1 : S2L[(c * LUTN + 0.5) | 0];
const toSrgb = c => c <= 0 ? 0 : c >= 1 ? 1 : L2S[(c * LUTN + 0.5) | 0];
const lum = (r, g, b) => 0.2126 * r + 0.7152 * g + 0.0722 * b;
const smooth = (a, b, x) => { const t = clamp((x - a) / (b - a || 1e-6), 0, 1); return t * t * (3 - 2 * t); };

// ---- Raster: pixels at full resolution, a revision per 64 px tile, and a lazy pyramid ----------
let RASTER_ID = 0;
class Raster {
  constructor(w, h, ch, data) {
    this.id = ++RASTER_ID; this.w = w; this.h = h; this.ch = ch;
    this.data = data || new Uint8ClampedArray(w * h * ch);
    this.T = 64; this.ntx = Math.ceil(w / 64); this.nty = Math.ceil(h / 64);
    this.revs = new Uint32Array(this.ntx * this.nty); this.rev = 1; this.pyr = [];
  }
  // A key for the content inside a level-0 rectangle: the revisions of the 64 px tiles it touches.
  contentKey(x0, y0, x1, y1) {
    const tx0 = clamp(Math.floor(x0 / 64), 0, this.ntx - 1), tx1 = clamp(Math.floor((x1 - 1) / 64), 0, this.ntx - 1);
    const ty0 = clamp(Math.floor(y0 / 64), 0, this.nty - 1), ty1 = clamp(Math.floor((y1 - 1) / 64), 0, this.nty - 1);
    if (x1 <= 0 || y1 <= 0 || x0 >= this.w || y0 >= this.h) return 7;
    let k = this.id;
    for (let ty = ty0; ty <= ty1; ty++) for (let tx = tx0; tx <= tx1; tx++) k = hmix(k, this.revs[ty * this.ntx + tx]);
    return k;
  }
  bump(x0, y0, x1, y1) {
    const tx0 = clamp(Math.floor(x0 / 64), 0, this.ntx - 1), tx1 = clamp(Math.floor((x1 - 1) / 64), 0, this.ntx - 1);
    const ty0 = clamp(Math.floor(y0 / 64), 0, this.nty - 1), ty1 = clamp(Math.floor((y1 - 1) / 64), 0, this.nty - 1);
    this.rev++;
    for (let ty = ty0; ty <= ty1; ty++) for (let tx = tx0; tx <= tx1; tx++) this.revs[ty * this.ntx + tx] = this.rev;
  }
  bumpAll() { this.bump(0, 0, this.w, this.h); }
  // Level L of the pyramid: each level halves the last with a 2×2 box, weighted by alpha for colour.
  level(L) {
    if (L === 0) return this;
    const c = this.pyr[L];
    if (c && c.rev === this.rev) return c;
    const s = this.level(L - 1), w = Math.max(1, Math.ceil(s.w / 2)), h = Math.max(1, Math.ceil(s.h / 2)), ch = this.ch;
    const d = new Uint8ClampedArray(w * h * ch), sd = s.data;
    for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
      const xs = [Math.min(2 * x, s.w - 1), Math.min(2 * x + 1, s.w - 1)], ys = [Math.min(2 * y, s.h - 1), Math.min(2 * y + 1, s.h - 1)];
      const o = (y * w + x) * ch;
      if (ch === 1) { let t = 0; for (const yy of ys) for (const xx of xs) t += sd[yy * s.w + xx]; d[o] = t / 4; continue; }
      let r = 0, g = 0, b = 0, a = 0;
      for (const yy of ys) for (const xx of xs) { const i = (yy * s.w + xx) * 4, al = sd[i + 3]; r += sd[i] * al; g += sd[i + 1] * al; b += sd[i + 2] * al; a += al; }
      if (a > 0) { d[o] = r / a; d[o + 1] = g / a; d[o + 2] = b / a; d[o + 3] = a / 4; }
    }
    return (this.pyr[L] = { w, h, ch, data: d, rev: this.rev });
  }
  copyRect(x0, y0, w, h) {
    const out = new Uint8ClampedArray(w * h * this.ch);
    for (let y = 0; y < h; y++) out.set(this.data.subarray(((y0 + y) * this.w + x0) * this.ch, ((y0 + y) * this.w + x0 + w) * this.ch), y * w * this.ch);
    return out;
  }
  putRect(x0, y0, w, h, src) {
    for (let y = 0; y < h; y++) this.data.set(src.subarray(y * w * this.ch, (y + 1) * w * this.ch), ((y0 + y) * this.w + x0) * this.ch);
    this.bump(x0, y0, x0 + w, y0 + h);
  }
  clone() { const r = new Raster(this.w, this.h, this.ch, new Uint8ClampedArray(this.data)); return r; }
}

// ---- Gaussian blur as three box blurs (the sizes come from σ), on interleaved channels ------------
function boxSizes(sigma, n = 3) {
  const wIdeal = Math.sqrt(12 * sigma * sigma / n + 1);
  let wl = Math.floor(wIdeal); if (wl % 2 === 0) wl--;
  const wu = wl + 2, m = Math.round((12 * sigma * sigma - n * wl * wl - 4 * n * wl - 3 * n) / (-4 * wl - 4));
  return Array.from({ length: n }, (_, i) => i < m ? wl : wu);
}
// How far a blur of σ reaches: the sum of the three box radii. A tile needs this many extra pixels.
function blurReach(sigma) { return sigma < 0.3 ? 0 : boxSizes(sigma).reduce((s, b) => s + (b - 1) / 2, 0); }
function boxH(src, dst, w, h, ch, r) {
  if (ch === 4) return boxH4(src, dst, w, h, r);
  const norm = 1 / (2 * r + 1), rr = Math.min(r, w - 1);
  for (let y = 0; y < h; y++) {
    const o = y * w, fv = src[o], lv = src[o + w - 1];
    let acc = (r + 1) * fv;
    for (let j = 0; j < r; j++) acc += src[o + Math.min(j, w - 1)];
    let x = 0;
    for (; x <= r && x < w; x++) { acc += (x + r < w ? src[o + x + r] : lv) - fv; dst[o + x] = acc * norm; }
    for (; x < w - rr; x++) { acc += src[o + x + r] - src[o + x - r - 1]; dst[o + x] = acc * norm; }
    for (; x < w; x++) { acc += lv - src[o + x - r - 1]; dst[o + x] = acc * norm; }
  }
}
// Four channels at once: one pass over the row with four running sums.
function boxH4(src, dst, w, h, r) {
  const norm = 1 / (2 * r + 1), rr = Math.min(r, w - 1), R4 = r * 4, R14 = (r + 1) * 4;
  for (let y = 0; y < h; y++) {
    const o = y * w * 4, e = o + (w - 1) * 4;
    const f0 = src[o], f1 = src[o + 1], f2 = src[o + 2], f3 = src[o + 3], l0 = src[e], l1 = src[e + 1], l2 = src[e + 2], l3 = src[e + 3];
    let a0 = (r + 1) * f0, a1 = (r + 1) * f1, a2 = (r + 1) * f2, a3 = (r + 1) * f3;
    for (let j = 0; j < r; j++) { const k = o + Math.min(j, w - 1) * 4; a0 += src[k]; a1 += src[k + 1]; a2 += src[k + 2]; a3 += src[k + 3]; }
    let x = 0, i = o;
    for (; x <= r && x < w; x++, i += 4) {
      if (x + r < w) { const k = i + R4; a0 += src[k] - f0; a1 += src[k + 1] - f1; a2 += src[k + 2] - f2; a3 += src[k + 3] - f3; }
      else { a0 += l0 - f0; a1 += l1 - f1; a2 += l2 - f2; a3 += l3 - f3; }
      dst[i] = a0 * norm; dst[i + 1] = a1 * norm; dst[i + 2] = a2 * norm; dst[i + 3] = a3 * norm;
    }
    for (; x < w - rr; x++, i += 4) {
      const k = i + R4, q = i - R14;
      a0 += src[k] - src[q]; a1 += src[k + 1] - src[q + 1]; a2 += src[k + 2] - src[q + 2]; a3 += src[k + 3] - src[q + 3];
      dst[i] = a0 * norm; dst[i + 1] = a1 * norm; dst[i + 2] = a2 * norm; dst[i + 3] = a3 * norm;
    }
    for (; x < w; x++, i += 4) {
      const q = i - R14;
      a0 += l0 - src[q]; a1 += l1 - src[q + 1]; a2 += l2 - src[q + 2]; a3 += l3 - src[q + 3];
      dst[i] = a0 * norm; dst[i + 1] = a1 * norm; dst[i + 2] = a2 * norm; dst[i + 3] = a3 * norm;
    }
  }
}
function transpose(src, dst, w, h, ch) {
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) { const i = (y * w + x) * ch, j = (x * h + y) * ch; for (let c = 0; c < ch; c++) dst[j + c] = src[i + c]; }
}
const SCRATCH = [];
function scratch(slot, n) { let b = SCRATCH[slot]; if (!b || b.length < n) b = SCRATCH[slot] = new Float32Array(Math.max(n, 1 << 16)); return b.length === n ? b : b.subarray(0, n); }
function gaussBlur(buf, w, h, ch, sigma) {
  if (sigma < 0.3) return buf;
  const radii = boxSizes(sigma).map(s => (s - 1) / 2).filter(r => r > 0);
  const n = w * h * ch, a = scratch(8, n), t = scratch(9, n);
  let src = buf, dst = a;
  for (const r of radii) { boxH(src, dst, w, h, ch, r); [src, dst] = [dst, src === buf ? t : src]; }
  transpose(src, dst === buf ? t : dst, w, h, ch); src = dst === buf ? t : dst; dst = src === a ? t : a;
  for (const r of radii) { boxH(src, dst, h, w, ch, r); const nx = src; src = dst; dst = nx; }
  transpose(src, buf, h, w, ch);
  return buf;
}

// ---- Blend modes (W3C Compositing and Blending), on straight colour ------------------------------
const SEP = {
  normal: (b, s) => s,
  multiply: (b, s) => b * s,
  screen: (b, s) => b + s - b * s,
  overlay: (b, s) => b <= 0.5 ? 2 * s * b : 1 - 2 * (1 - s) * (1 - b),
  darken: (b, s) => Math.min(b, s),
  lighten: (b, s) => Math.max(b, s),
  colorDodge: (b, s) => b <= 0 ? 0 : s >= 1 ? 1 : Math.min(1, b / (1 - s)),
  colorBurn: (b, s) => b >= 1 ? 1 : s <= 0 ? 0 : 1 - Math.min(1, (1 - b) / s),
  hardLight: (b, s) => s <= 0.5 ? 2 * s * b : 1 - 2 * (1 - s) * (1 - b),
  softLight: (b, s) => {
    if (s <= 0.5) return b - (1 - 2 * s) * b * (1 - b);
    const d = b <= 0.25 ? ((16 * b - 12) * b + 4) * b : Math.sqrt(b);
    return b + (2 * s - 1) * (d - b);
  },
  difference: (b, s) => Math.abs(b - s),
  exclusion: (b, s) => b + s - 2 * b * s,
  add: (b, s) => Math.min(1, b + s),
  subtract: (b, s) => Math.max(0, b - s),
};
const NONSEP = new Set(['hue', 'saturation', 'color', 'luminosity']);
const BLEND_MODES = ['normal', 'multiply', 'screen', 'overlay', 'softLight', 'hardLight', 'darken', 'lighten', 'colorDodge', 'colorBurn', 'add', 'subtract', 'difference', 'exclusion', 'hue', 'saturation', 'color', 'luminosity'];
const BLEND_LABEL = { pass: 'pass through', normal: 'normal', multiply: 'multiply', screen: 'screen', overlay: 'overlay', softLight: 'soft light', hardLight: 'hard light', darken: 'darken', lighten: 'lighten', colorDodge: 'colour dodge', colorBurn: 'colour burn', add: 'add', subtract: 'subtract', difference: 'difference', exclusion: 'exclusion', hue: 'hue', saturation: 'saturation', color: 'colour', luminosity: 'luminosity' };
function clipColor(o) {
  const l = lum(o[0], o[1], o[2]), n = Math.min(o[0], o[1], o[2]), x = Math.max(o[0], o[1], o[2]);
  for (let i = 0; i < 3; i++) {
    if (n < 0) o[i] = l + (o[i] - l) * l / (l - n || 1e-6);
    if (x > 1) o[i] = l + (o[i] - l) * (1 - l) / (x - l || 1e-6);
  }
}
function setLum(r, g, b, l, o) { const d = l - lum(r, g, b); o[0] = r + d; o[1] = g + d; o[2] = b + d; clipColor(o); }
function setSat(r, g, b, s, o) {
  const mx = Math.max(r, g, b), mn = Math.min(r, g, b), c = [r, g, b];
  if (mx > mn) for (let i = 0; i < 3; i++) o[i] = (c[i] - mn) * s / (mx - mn); else o[0] = o[1] = o[2] = 0;
}
const sat3 = (r, g, b) => Math.max(r, g, b) - Math.min(r, g, b);
// B(Cb, Cs) for one pixel; writes the result into o.
function blend3(mode, br, bg, bb, sr, sg, sb, o) {
  const f = SEP[mode];
  if (f) { o[0] = f(br, sr); o[1] = f(bg, sg); o[2] = f(bb, sb); return; }
  const t = [0, 0, 0];
  if (mode === 'hue') { setSat(sr, sg, sb, sat3(br, bg, bb), t); setLum(t[0], t[1], t[2], lum(br, bg, bb), o); }
  else if (mode === 'saturation') { setSat(br, bg, bb, sat3(sr, sg, sb), t); setLum(t[0], t[1], t[2], lum(br, bg, bb), o); }
  else if (mode === 'color') setLum(sr, sg, sb, lum(br, bg, bb), o);
  else setLum(br, bg, bb, lum(sr, sg, sb), o);
}

// ---- Layer kinds: their params (with units), what they compute, and how far they reach ------------
const P = (min, max, def, unit = '', label = '', step) => ({ min, max, def, unit, label, step: step ?? (max - min > 20 ? 1 : max - min > 2 ? 0.01 : 0.001) });
const PCT = (min, max, def, label) => ({ min, max, def, unit: '%', pct: true, label, step: 0.005 });
const COLOR = (def, label) => ({ type: 'color', def, label });
const CHOICE = (options, def, label) => ({ type: 'choice', options, def, label });
const hexToRgb = hx => { const n = parseInt(hx.replace('#', '').padEnd(6, '0').slice(0, 6), 16); return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255]; };
const rgbToHex = (r, g, b) => '#' + [r, g, b].map(v => Math.round(clamp(v, 0, 1) * 255).toString(16).padStart(2, '0')).join('');

function hash2(x, y, s) { let h = Math.imul(x | 0, 374761393) ^ Math.imul(y | 0, 668265263) ^ Math.imul(s | 0, 2147483647); h = Math.imul(h ^ (h >>> 13), 1274126177); return ((h ^ (h >>> 16)) >>> 0) / 4294967296; }

function hue2rgb(p, q, t) { t = (t + 1) % 1; return t < 1 / 6 ? p + (q - p) * 6 * t : t < 0.5 ? q : t < 2 / 3 ? p + (q - p) * (2 / 3 - t) * 6 : p; }
function curveLut(p) {
  const N = 1024, lut = new Float32Array(N);
  const bump = (x, c) => Math.exp(-(((x - c) / 0.2) ** 2)) * 4 * x * (1 - x);
  let prev = 0;
  for (let i = 0; i < N; i++) {
    const x = i / (N - 1), S = x * x * (3 - 2 * x);
    let y = p.contrast >= 0 ? x + p.contrast * (S - x) * 1.2 : x - p.contrast * (x - S) * -1.2;
    y += 0.35 * (p.shadows * bump(x, 0.22) + p.midtones * bump(x, 0.5) + p.highlights * bump(x, 0.78));
    y = clamp(y, 0, 1); if (y < prev) y = prev; prev = y; lut[i] = y;
  }
  return lut;
}
const lutAt = (lut, v) => lut[clamp((v * 1023 + 0.5) | 0, 0, 1023)];

const TEXT = (def, label) => ({ type: 'text', def, label });
const KINDS = {
  image: { group: 'Sources', label: 'Image', abbr: 'IMG', src: true, raster: true, params: {} },
  paint: { group: 'Sources', label: 'Paint', abbr: 'PNT', src: true, raster: true, params: {} },
  fill: { group: 'Sources', label: 'Fill', abbr: 'FILL', src: true, params: { color: COLOR('#2f5d8a', 'colour') },
    paint(src, n, p) { const [r, g, b] = hexToRgb(p.color); for (let i = 0; i < n * 4; i += 4) { src[i] = r; src[i + 1] = g; src[i + 2] = b; src[i + 3] = 1; } } },
  gradient: {
    group: 'Sources', label: 'Gradient', abbr: 'GRAD', src: true, positional: true,
    params: { shape: CHOICE(['linear', 'radial'], 'linear', 'shape'), c0: COLOR('#000000', 'start colour'), a0: PCT(0, 1, 1, 'start opacity'), c1: COLOR('#000000', 'end colour'), a1: PCT(0, 1, 0, 'end opacity'),
      x0: P(0, 1, 0.5, '× width', 'start x'), y0: P(0, 1, 0, '× height', 'start y'), x1: P(0, 1, 0.5, '× width', 'end x'), y1: P(0, 1, 0.6, '× height', 'end y') },
    paint(src, n, p, ctx) {
      const c0 = hexToRgb(p.c0), c1 = hexToRgb(p.c1), ax = p.x0 * ctx.W, ay = p.y0 * ctx.H, dx = p.x1 * ctx.W - ax, dy = p.y1 * ctx.H - ay, len2 = dx * dx + dy * dy || 1, len = Math.sqrt(len2);
      for (let y = 0; y < ctx.h; y++) for (let x = 0; x < ctx.w; x++) {
        const X = (ctx.x0 + x + 0.5) * ctx.s - ax, Y = (ctx.y0 + y + 0.5) * ctx.s - ay;
        let t = p.shape === 'radial' ? Math.sqrt(X * X + Y * Y) / len : (X * dx + Y * dy) / len2; t = clamp(t, 0, 1);
        const i = (y * ctx.w + x) * 4;
        src[i] = c0[0] + (c1[0] - c0[0]) * t; src[i + 1] = c0[1] + (c1[1] - c0[1]) * t; src[i + 2] = c0[2] + (c1[2] - c0[2]) * t; src[i + 3] = p.a0 + (p.a1 - p.a0) * t;
      }
    },
  },
  exposure: { group: 'Tone', label: 'Exposure', abbr: 'EXP', main: 'ev', params: { ev: P(-4, 4, 0, 'EV', 'exposure', 0.01), offset: P(-0.2, 0.2, 0, '', 'black offset', 0.001), gamma: P(0.2, 3, 1, '', 'gamma', 0.01) },
    fn(b, n, p) { const m = Math.pow(2, p.ev), o = p.offset, g = 1 / p.gamma;
      for (let i = 0; i < n * 4; i += 4) for (let c = 0; c < 3; c++) { let v = toSrgb(toLin(b[i + c]) * m + o); if (g !== 1) v = Math.pow(v, g); b[i + c] = v; } } },
  levels: { group: 'Tone', label: 'Levels', abbr: 'LVL', main: 'gamma', params: { black: P(0, 1, 0, '', 'input black'), white: P(0, 1, 1, '', 'input white'), gamma: P(0.1, 4, 1, '', 'midtone gamma'), outBlack: P(0, 1, 0, '', 'output black'), outWhite: P(0, 1, 1, '', 'output white') },
    fn(b, n, p) { const span = Math.max(1e-3, p.white - p.black), g = 1 / p.gamma, ob = p.outBlack, os = p.outWhite - p.outBlack;
      for (let i = 0; i < n * 4; i += 4) for (let c = 0; c < 3; c++) { let v = clamp((b[i + c] - p.black) / span, 0, 1); if (g !== 1) v = Math.pow(v, g); b[i + c] = ob + v * os; } } },
  curves: { group: 'Tone', label: 'Curves', abbr: 'CRV', main: 'contrast', params: { contrast: P(-1, 1, 0.2, '', 'contrast (S-curve)'), shadows: P(-1, 1, 0, '', 'shadows'), midtones: P(-1, 1, 0, '', 'midtones'), highlights: P(-1, 1, 0, '', 'highlights') },
    prep: curveLut, fn(b, n, p, ctx) { const lut = ctx.prep; for (let i = 0; i < n * 4; i += 4) { b[i] = lutAt(lut, b[i]); b[i + 1] = lutAt(lut, b[i + 1]); b[i + 2] = lutAt(lut, b[i + 2]); } } },
  hueSat: { group: 'Colour', label: 'Hue and saturation', abbr: 'HSL', main: 'saturation', params: { hue: P(-180, 180, 0, '°', 'hue shift', 1), saturation: PCT(-1, 1, 0, 'saturation'), lightness: PCT(-1, 1, 0, 'lightness') },
    fn(b, n, p) {
      const dh = p.hue / 360, ds = p.saturation, dl = p.lightness;
      for (let i = 0; i < n * 4; i += 4) {
        const r = b[i], g = b[i + 1], bl = b[i + 2], mx = Math.max(r, g, bl), mn = Math.min(r, g, bl);
        let hh = 0, s = 0, l = (mx + mn) / 2; const d = mx - mn;
        if (d > 1e-6) { s = l > 0.5 ? d / (2 - mx - mn) : d / (mx + mn); hh = mx === r ? (g - bl) / d + (g < bl ? 6 : 0) : mx === g ? (bl - r) / d + 2 : (r - g) / d + 4; hh /= 6; }
        hh = (hh + dh + 1) % 1; s = clamp(ds >= 0 ? s + (1 - s) * ds * s : s * (1 + ds), 0, 1); l = clamp(dl >= 0 ? l + (1 - l) * dl : l * (1 + dl), 0, 1);
        const q = l < 0.5 ? l * (1 + s) : l + s - l * s, pp = 2 * l - q;
        b[i] = hue2rgb(pp, q, hh + 1 / 3); b[i + 1] = hue2rgb(pp, q, hh); b[i + 2] = hue2rgb(pp, q, hh - 1 / 3);
      } } },
  vibrance: { group: 'Colour', label: 'Vibrance', abbr: 'VIB', main: 'amount', params: { amount: PCT(-1, 1, 0.3, 'vibrance') },
    fn(b, n, p) { const a = p.amount; for (let i = 0; i < n * 4; i += 4) { const r = b[i], g = b[i + 1], bl = b[i + 2], avg = (r + g + bl) / 3, s = sat3(r, g, bl), k = 1 + a * (1 - s) * 1.4; b[i] = avg + (r - avg) * k; b[i + 1] = avg + (g - avg) * k; b[i + 2] = avg + (bl - avg) * k; } } },
  temperature: { group: 'Colour', label: 'Temperature', abbr: 'TMP', main: 'temp', params: { temp: P(-100, 100, 15, '', 'warmth', 1), tint: P(-100, 100, 0, '', 'tint (green to magenta)', 1) },
    fn(b, n, p) { const t = p.temp / 100, ti = p.tint / 100; let gr = 1 + 0.28 * t, gg = 1 - 0.12 * ti, gb = 1 - 0.28 * t; const k = 1 / lum(gr, gg, gb); gr *= k; gg *= k; gb *= k;
      for (let i = 0; i < n * 4; i += 4) { b[i] = toSrgb(toLin(b[i]) * gr); b[i + 1] = toSrgb(toLin(b[i + 1]) * gg); b[i + 2] = toSrgb(toLin(b[i + 2]) * gb); } } },
  bw: { group: 'Colour', label: 'Black and white', abbr: 'B&W', main: 'red', params: { red: PCT(-0.5, 2, 0.3, 'red weight'), green: PCT(-0.5, 2, 0.59, 'green weight'), blue: PCT(-0.5, 2, 0.11, 'blue weight') },
    fn(b, n, p) { for (let i = 0; i < n * 4; i += 4) { const v = clamp(b[i] * p.red + b[i + 1] * p.green + b[i + 2] * p.blue, 0, 1); b[i] = b[i + 1] = b[i + 2] = v; } } },
  invert: { group: 'Colour', label: 'Invert', abbr: 'INV', params: {}, fn(b, n) { for (let i = 0; i < n * 4; i += 4) { b[i] = 1 - b[i]; b[i + 1] = 1 - b[i + 1]; b[i + 2] = 1 - b[i + 2]; } } },
  threshold: { group: 'Colour', label: 'Threshold', abbr: 'THR', main: 'level', params: { level: P(0, 1, 0.5, '', 'level') },
    fn(b, n, p) { for (let i = 0; i < n * 4; i += 4) { const v = lum(b[i], b[i + 1], b[i + 2]) >= p.level ? 1 : 0; b[i] = b[i + 1] = b[i + 2] = v; } } },
  posterize: { group: 'Colour', label: 'Posterize', abbr: 'PST', main: 'levels', params: { levels: P(2, 16, 4, 'steps', 'levels', 1) },
    fn(b, n, p) { const k = Math.max(1, Math.round(p.levels) - 1); for (let i = 0; i < n * 4; i += 4) for (let c = 0; c < 3; c++) b[i + c] = Math.round(clamp(b[i + c], 0, 1) * k) / k; } },
  blur: { group: 'Detail', label: 'Gaussian blur', abbr: 'BLUR', main: 'radius', roi: true, params: { radius: P(0, 80, 6, 'px', 'radius (σ)', 0.1) },
    reach: (p, L) => Math.ceil(blurReach(p.radius / 2 ** L)),
    roiFn(buf, w, h, p, L) { gaussBlur(buf, w, h, 4, p.radius / 2 ** L); } },
  sharpen: { group: 'Detail', label: 'Sharpen', abbr: 'SHP', main: 'amount', roi: true, params: { amount: P(0, 3, 0.8, '×', 'amount'), radius: P(0.3, 10, 1.2, 'px', 'radius (σ)'), threshold: P(0, 0.2, 0, '', 'threshold') },
    reach: (p, L) => Math.ceil(blurReach(p.radius / 2 ** L)),
    roiFn(buf, w, h, p, L) { unsharp(buf, w, h, p.radius / 2 ** L, p.amount, p.threshold, false); } },
  clarity: { group: 'Detail', label: 'Clarity', abbr: 'CLR', main: 'amount', roi: true, params: { amount: P(-1, 1, 0.35, '×', 'amount'), radius: P(4, 60, 24, 'px', 'radius (σ)') },
    reach: (p, L) => Math.ceil(blurReach(p.radius / 2 ** L)),
    roiFn(buf, w, h, p, L) { unsharp(buf, w, h, p.radius / 2 ** L, p.amount, 0, true); } },
  vignette: { group: 'Effects', label: 'Vignette', abbr: 'VIG', main: 'amount', positional: true, params: { amount: P(-1, 1, -0.4, '', 'amount'), size: P(0, 1.5, 0.7, '', 'size'), feather: P(0.01, 1, 0.55, '', 'feather') },
    fn(b, n, p, ctx) { const { x0, y0, w, s, W, H } = ctx;
      for (let i = 0, j = 0; j < n; j++, i += 4) { const X = (x0 + (j % w) + 0.5) * s, Y = (y0 + ((j / w) | 0) + 0.5) * s, dx = (X - W / 2) / (W / 2), dy = (Y - H / 2) / (H / 2);
        const f = smooth(p.size, p.size + p.feather, Math.sqrt(dx * dx + dy * dy)) * p.amount;
        for (let c = 0; c < 3; c++) b[i + c] = f < 0 ? b[i + c] * (1 + f) : b[i + c] + (1 - b[i + c]) * f; } } },
  grain: { group: 'Effects', label: 'Grain', abbr: 'GRN', main: 'amount', positional: true, params: { amount: P(0, 0.5, 0.08, '', 'amount'), size: P(0.5, 4, 1, 'px', 'grain size'), seed: P(0, 99, 7, '', 'seed', 1) },
    fn(b, n, p, ctx) { const { x0, y0, w, s } = ctx;
      for (let i = 0, j = 0; j < n; j++, i += 4) { const X = Math.floor((x0 + (j % w) + 0.5) * s / p.size), Y = Math.floor((y0 + ((j / w) | 0) + 0.5) * s / p.size);
        const v = (hash2(X, Y, p.seed) + hash2(X + 7, Y + 3, p.seed + 1) - 1) * p.amount;
        for (let c = 0; c < 3; c++) { const x = b[i + c]; b[i + c] = x + v * (0.25 + 3 * x * (1 - x)); } } } },
  code: { group: 'Code', label: 'Code', abbr: 'JS', code: true, params: { src: TEXT('', 'source'), part: TEXT('', 'layer in the source'), preset: TEXT('', 'preset') }, fn() {} },
  // A group holds the layers listed directly below it whose parent is its id. Pass through: its children
  // apply to the backdrop and the group mixes the result by opacity and mask. Any blend mode: isolated, its
  // children start from transparent and the group blends their composite onto the backdrop as one source.
  group: { group: 'Layers', label: 'Group', abbr: 'GRP', isGroup: true, params: {} },
};
function unsharp(buf, w, h, sigma, amount, thresh, midtones) {
  const bl = scratch(3, buf.length); bl.set(buf); gaussBlur(bl, w, h, 4, sigma);
  for (let i = 0; i < w * h * 4; i += 4) {
    const a = buf[i + 3]; if (a <= 0) continue;
    const r = buf[i] / a, g = buf[i + 1] / a, b = buf[i + 2] / a, ba = bl[i + 3] || 1e-6;
    const dr = r - bl[i] / ba, dg = g - bl[i + 1] / ba, db = b - bl[i + 2] / ba;
    if (thresh && Math.abs(lum(dr, dg, db)) < thresh) continue;
    let k = amount; if (midtones) { const l = lum(r, g, b); k *= 1 - (2 * l - 1) ** 2; }
    buf[i] = clamp(r + k * dr, 0, 1) * a; buf[i + 1] = clamp(g + k * dg, 0, 1) * a; buf[i + 2] = clamp(b + k * db, 0, 1) * a;
  }
}
function defaults(kind) { const o = {}; for (const [k, s] of Object.entries(KINDS[kind].params)) o[k] = s.def; return o; }

// ---- Code layers: a layer whose operation is JavaScript ----------------------------------------
// The source is a small program: optional preset(name, { about }) and one or more
// layer(name, { params, fn | roiFn + reach | paint, prep?, mask?, blend?, opacity? }) calls. It is
// compiled once per distinct source; a code layer names its part. fn, roiFn and paint have the
// built-in kinds' signatures, so any kind's code can be copied into a code layer unchanged.
const CODE_MODULES = new Map();
function specFrom(k, v) {
  if (typeof v === 'number') return P(Math.min(0, v), Math.max(1, Math.abs(v) * 2), v, '', k);
  if (typeof v === 'string' && /^#[0-9a-f]{6}$/i.test(v)) return COLOR(v, k);
  if (Array.isArray(v) && typeof v[0] === 'number') { const [min, max, val = min, label = k, unit = ''] = v; if (!(max > min)) throw new Error(`${k}: [min, max, value] needs max above min`); return unit === '%' ? PCT(min, max, clamp(val, min, max), label) : P(min, max, clamp(val, min, max), unit, label); }
  if (v && typeof v === 'object' && v.color) return COLOR(v.color, v.label || k);
  if (v && typeof v === 'object' && Array.isArray(v.choose)) return CHOICE(v.choose, v.value ?? v.choose[0], v.label || k);
  throw new Error(`${k}: a setting is [min, max, value, label?, unit?], '#rrggbb', { color, label } or { choose: [...], value, label }`);
}
function codeModule(src) {
  src = String(src || ''); let m = CODE_MODULES.get(src); if (m) return m;
  m = { src, meta: null, parts: [], byName: new Map(), error: null, runtime: null };
  try {
    const run = compileModule(src);
    run((name, o = {}) => { m.meta = Object.assign({}, o, { name: String(name) }); }, (name, def = {}) => {
      name = String(name); if (m.byName.has(name)) throw new Error(`two layers are called '${name}'`);
      const has = ['fn', 'roiFn', 'paint'].filter(k => typeof def[k] === 'function');
      if (has.length !== 1) throw new Error(`layer '${name}': give one of fn(b, n, p, ctx) for a pointwise change, roiFn(buf, w, h, p, L) with reach(p, L) for one that reads neighbours, or paint(src, n, p, ctx) for a source`);
      if (def.roiFn && typeof def.reach !== 'function') throw new Error(`layer '${name}': roiFn needs reach(p, L), the pixels it reads beyond the tile`);
      const params = {}; for (const [k, v] of Object.entries(def.params || {})) { if (['src', 'part', 'preset'].includes(k)) throw new Error(`layer '${name}': '${k}' is a reserved setting name`); params[k] = specFrom(k, v); }
      const part = { name, label: name, abbr: 'JS', code: true, module: m, params, fn: def.fn, prep: typeof def.prep === 'function' ? def.prep : undefined, roi: !!def.roiFn, roiFn: def.roiFn, reach: def.reach, src: !!def.paint, paint: def.paint, mask: def.mask, blend: def.blend, opacity: def.opacity, about: def.about };
      m.parts.push(part); m.byName.set(name, part);
    }, M, lin, rad, lumR, colR);
    if (!m.parts.length) throw new Error('the code defines no layer: call layer(name, { params, fn })');
  } catch (e) { m.error = codeErr(e); }
  if (CODE_MODULES.size > 60) CODE_MODULES.delete(CODE_MODULES.keys().next().value);
  CODE_MODULES.set(src, m); return m;
}
function codeErr(e) { return { message: (e && e.message) || String(e), line: moduleLine(e) }; }
const CODE_BROKEN = { label: 'Code', abbr: 'JS', code: true, broken: true, params: {}, fn() {} };
function codePart(params) { const m = codeModule(params && params.src); return m.error ? null : m.byName.get(params.part) || null; }
function kindOf(l) { if (l.kind !== 'code') return KINDS[l.kind]; return codePart(l.params) || CODE_BROKEN; }
// The settings a layer offers: its kind's, or for a code layer its part's (and the hidden source fields).
function specsOf(kind, params) { if (kind !== 'code') return KINDS[kind].params; const part = codePart(params); return Object.assign({}, KINDS.code.params, part ? part.params : {}); }
function defaultsOf(kind, params) { const o = {}; for (const [k, s] of Object.entries(specsOf(kind, params))) o[k] = s.def; return o; }
// A code layer that throws passes its backdrop through; the first error per part is kept and announced.
function codeFail(K, e) { const m = K.module; if (!m) return; const err = Object.assign(codeErr(e), { part: K.name }); if (!m.runtime || m.runtime.message !== err.message) { m.runtime = err; if (typeof app !== 'undefined' && app.emit) setTimeout(() => app.emit('codeError', { module: m, error: err }), 0); } }

// ---- Layers and documents -----------------------------------------------------------------------
let LAYER_ID = 0;
function makeLayer(kind, opts = {}) {
  const K = KINDS[kind];
  const L = { id: opts.id || 'L' + (++LAYER_ID), kind, name: opts.name || K.label, visible: opts.visible ?? true, opacity: opts.opacity ?? 1,
    blend: opts.blend || (K.isGroup ? 'pass' : 'normal'), params: Object.assign(defaults(kind), opts.params || {}), ox: opts.ox || 0, oy: opts.oy || 0, raster: null, mask: null };
  if (opts.parent) L.parent = opts.parent;
  if (K.isGroup) L.open = opts.open ?? true;
  if (opts.meta) L.meta = JSON.parse(JSON.stringify(opts.meta));
  return L;
}
// ---- Masks: an ordered list of components, each giving a value from 0 to 1 at every pixel ----------
// linear and radial are geometry; brush holds pixels; luminance and colour read the backdrop.
const COMP_KINDS = {
  linear: { label: 'Linear gradient', abbr: 'LIN', params: { x0: P(-0.5, 1.5, 0.5, '× width', 'start x'), y0: P(-0.5, 1.5, 0.15, '× height', 'start y'), x1: P(-0.5, 1.5, 0.5, '× width', 'end x'), y1: P(-0.5, 1.5, 0.6, '× height', 'end y') } },
  radial: { label: 'Radial gradient', abbr: 'RAD', params: { cx: P(-0.5, 1.5, 0.5, '× width', 'centre x'), cy: P(-0.5, 1.5, 0.5, '× height', 'centre y'), rx: P(0.01, 1.5, 0.25, '× width', 'half width'), ry: P(0.01, 1.5, 0.3, '× height', 'half height'), angle: P(-180, 180, 0, '°', 'rotation', 1), feather: PCT(0, 1, 0.5, 'feather') } },
  brush: { label: 'Brush', abbr: 'BR', raster: true, params: { feather: P(0, 120, 0, 'px', 'edge feather', 0.5) } },
  luminance: { label: 'Luminance range', abbr: 'LUM', params: { lo: P(0, 1, 0.6, '', 'from'), hi: P(0, 1, 1, '', 'to'), soft: P(0, 0.5, 0.1, '', 'softness') } },
  color: { label: 'Colour range', abbr: 'COL', params: { color: COLOR('#e08a4a', 'colour'), tol: P(0, 1, 0.15, '', 'tolerance'), soft: P(0.001, 0.5, 0.1, '', 'softness') } },
};
const COMP_OPS = ['add', 'subtract', 'intersect'];
let COMP_ID = 0;
function makeComp(type, o = {}, w, h) {
  const K = COMP_KINDS[type]; if (!K) throw new Error(`no mask component '${type}'; types are ${Object.keys(COMP_KINDS).join(', ')}`);
  const params = {}; for (const [k, sp] of Object.entries(K.params)) params[k] = sp.def;
  const c = { id: 'c' + (++COMP_ID), type, name: o.name || K.label, op: o.op || 'add', invert: !!o.invert, on: o.on ?? true, amount: o.amount ?? 1, params: Object.assign(params, o.params || {}) };
  if (K.raster) { c.raster = o.raster || new Raster(w, h, 1); if (!o.raster && o.fill) { c.raster.data.fill(o.fill); c.raster.bumpAll(); } }
  return c;
}
function makeMask(w, h, fill = 255, comps) {
  return { density: 1, invert: false, on: true, comps: comps || [makeComp('brush', { name: fill ? 'Brush' : 'Brush (hides)', fill }, w, h)] };
}
const compJSON = c => ({ id: c.id, type: c.type, name: c.name, op: c.op, invert: c.invert, on: c.on, amount: c.amount, params: JSON.parse(JSON.stringify(c.params)), raster: c.raster || null });
function layerProps(l) { return { name: l.name, visible: l.visible, opacity: l.opacity, blend: l.blend, params: JSON.parse(JSON.stringify(l.params)), ox: l.ox, oy: l.oy, mask: l.mask ? { density: l.mask.density, invert: l.mask.invert, on: l.mask.on, comps: l.mask.comps.map(compJSON) } : null }; }
function applyProps(l, p) {
  l.name = p.name; l.visible = p.visible; l.opacity = p.opacity; l.blend = p.blend; l.params = JSON.parse(JSON.stringify(p.params)); l.ox = p.ox; l.oy = p.oy;
  if (l.mask && p.mask) { l.mask.density = p.mask.density; l.mask.invert = p.mask.invert; l.mask.on = p.mask.on; l.mask.comps = p.mask.comps.map(compJSON); }
}
// JSON for comparing and hashing settings: a raster stands in as its id.
const propsJSON = v => JSON.stringify(v, (k, x) => k === 'raster' ? (x ? x.id : 0) : x);
function layerHash(l) {
  const m = l.mask ? [l.mask.density, l.mask.invert, l.mask.on, l.mask.comps] : 0;
  return hstr(propsJSON([l.kind, l.opacity, l.blend, l.params, l.ox, l.oy, m]));
}
const s01 = t => t <= 0 ? 0 : t >= 1 ? 1 : t * t * (3 - 2 * t);
// The combined mask of a layer over the whole picture at level L (for overlays and thumbnails).
function maskImage(eng, l, L) {
  const D = eng.dims(L), i = eng.doc.layers.indexOf(l), R = { x: 0, y: 0, w: D.w, h: D.h };
  const range = l.mask.comps.some(c => c.on && (c.type === 'luminance' || c.type === 'color'));
  const below = range && i >= 0 ? eng.rect(eng.below(i), L, 0, 0, D.w, D.h) : new Float32Array(D.w * D.h * 4);
  return { w: D.w, h: D.h, f: eng.coverage(l, L, R, below, 0) || new Float32Array(D.w * D.h).fill(1) };
}
// Combining components in order: add is the union (max), subtract and intersect are products,
// so two feathered edges meet in a smooth falloff instead of a crease. The first one is the base.
function combine(op, v, c) { return op === 'subtract' ? v * (1 - c) : op === 'intersect' ? v * c : (v > c ? v : c); }

// ---- The tile cache: least recently used, under a byte budget -----------------------------------
class TileCache {
  constructor(budgetMB = 96, policy = 'lru') { this.map = new Map(); this.bytes = 0; this.budget = budgetMB * 1048576; this.policy = policy; this.clock = 0; this.hits = 0; this.misses = 0; this.evictions = 0; this.peak = 0; }
  get(k) {
    const e = this.map.get(k); if (!e) { this.misses++; return null; }
    if (this.policy === 'lru') { this.map.delete(k); this.map.set(k, e); } else e.H = this.clock + e.cost / e.buf.byteLength;
    this.hits++; return e;
  }
  put(k, e) {
    e.cost = Math.max(e.cost || 0, 0.01) * 1e6; e.H = this.clock + e.cost / e.buf.byteLength;
    this.map.set(k, e); this.bytes += e.buf.byteLength; this.evict(k);
    if (this.bytes > this.peak) this.peak = this.bytes;
  }
  // LRU drops the oldest use. Cost-aware (GreedyDual-Size) drops the entry with the lowest
  // H = clock + cost / size, then moves the clock up to that H, so old entries age out too.
  evict(keep) {
    while (this.bytes > this.budget && this.map.size > 1) {
      let victim = null, vk;
      if (this.policy === 'lru') { for (const [k, e] of this.map) if (k !== keep) { vk = k; victim = e; break; } }
      else { for (const [k, e] of this.map) if (k !== keep && (!victim || e.H < victim.H)) { vk = k; victim = e; } if (victim) this.clock = victim.H; }
      if (!victim) break;
      this.map.delete(vk); this.bytes -= victim.buf.byteLength; this.evictions++;
    }
  }
  setBudget(mb) { this.budget = mb * 1048576; this.evict(null); }
  clear() { this.map.clear(); this.bytes = 0; this.clock = 0; }
  resetCounters() { this.hits = this.misses = this.evictions = 0; this.peak = this.bytes; }
}

// ---- The renderer: tiles of the composite after layer i, at pyramid level L ---------------------
class Engine {
  constructor(doc, opts = {}) {
    this.doc = doc; this.TS = opts.tile || 128; this.cache = opts.cache || new TileCache(opts.budgetMB || 96, opts.evict || 'lru');
    this.memo = new Map(); this.hashes = new Map(); this.maskCache = new Map(); this.layerMs = new Map(); this.computed = 0; this.applyMs = 0;
    this.empty = new Map(); this.preps = new WeakMap();
  }
  invalidate() { this.memo.clear(); this.hashes.clear(); this.st = null; }
  // The stack's structure, rebuilt after every change: each layer's parent index, each group's lowest
  // descendant, whether a layer shows (it, and every group above it, visible and not at zero opacity),
  // and the floor below which a layer's backdrop may not be looked for (the start of the innermost
  // isolated group around it, or 0).
  struct() {
    if (this.st && this.st.n === this.doc.layers.length) return this.st;
    const ls = this.doc.layers, n = ls.length, at = new Map(ls.map((l, i) => [l.id, i]));
    const par = new Int32Array(n).fill(-1), start = new Int32Array(n), vis = new Uint8Array(n), floor = new Int32Array(n), group = new Uint8Array(n);
    for (let i = 0; i < n; i++) { const p = ls[i].parent, pi = p != null && at.has(p) ? at.get(p) : -1; par[i] = pi > i && KINDS[ls[pi].kind].isGroup ? pi : -1; start[i] = i; group[i] = KINDS[ls[i].kind].isGroup ? 1 : 0; }
    for (let i = 0; i < n; i++) for (let p = par[i]; p >= 0; p = par[p]) if (i < start[p]) start[p] = i;
    for (let i = n - 1; i >= 0; i--) vis[i] = ls[i].visible && !(ls[i].opacity <= 0) && (par[i] < 0 || vis[par[i]]) ? 1 : 0;
    for (let i = 0; i < n; i++) { let p = par[i]; while (p >= 0 && ls[p].blend === 'pass') p = par[p]; floor[i] = p >= 0 ? start[p] : 0; }
    return (this.st = { n, par, start, vis, floor, group });
  }
  // The composite a layer draws on: the nearest showing layer below it (below a group's descendants for a
  // group), never under the floor of its isolated group. -1 is transparent.
  below(i) {
    const st = this.struct(), f = st.floor[i]; let j = (st.group[i] ? st.start[i] : i) - 1;
    while (j >= f && !st.vis[j]) j--;
    return j >= f ? j : -1;
  }
  // The top of a group's own stack: what its children made. A pass-through group's children draw on the
  // real backdrop; an isolated group's start from transparent.
  top(g) {
    const st = this.struct(), f = this.doc.layers[g].blend === 'pass' ? st.floor[g] : st.start[g]; let j = g - 1;
    while (j >= f && !st.vis[j]) j--;
    return j >= f ? j : -1;
  }
  // The composite after layer i: itself when it shows, else what it would have been drawn on.
  resolve(i) { return i >= 0 && !this.live(i) ? this.below(i) : i; }
  // A pass-through group at full opacity without a mask changes nothing: it is its children's result.
  identity(l) { return KINDS[l.kind].isGroup && l.blend === 'pass' && !(l.opacity < 1) && !(l.mask && l.mask.on); }
  prep(l) { const h = this.lh(l); let p = this.preps.get(l); if (!p || p.h !== h) { const K = kindOf(l); let v = null; try { v = K.prep(l.params); } catch (e) { if (K.code) codeFail(K, e); else throw e; } p = { h, v }; this.preps.set(l, p); } return p.v; }
  dims(L) { return { w: Math.max(1, Math.ceil(this.doc.w / 2 ** L)), h: Math.max(1, Math.ceil(this.doc.h / 2 ** L)) }; }
  grid(L) { const { w, h } = this.dims(L); return { w, h, nx: Math.ceil(w / this.TS), ny: Math.ceil(h / this.TS) }; }
  tileRect(L, tx, ty) { const { w, h } = this.dims(L), T = this.TS, x = tx * T, y = ty * T; return { x, y, w: Math.min(T, w - x), h: Math.min(T, h - y) }; }
  reach(l, L) { const K = kindOf(l); if (!K.roi) return 0; if (!K.code) return K.reach(l.params, L); try { const m = K.reach(l.params, L); return Number.isFinite(m) ? clamp(Math.ceil(m), 0, 512) : 0; } catch (e) { codeFail(K, e); return 0; } }
  live(i) { const st = this.struct(); return i >= 0 && i < st.n && st.vis[i] === 1; }
  lh(l) { let v = this.hashes.get(l); if (v === undefined) { v = layerHash(l); this.hashes.set(l, v); } return v; }

  // The tile's key: this layer's settings, the content it reads, and the keys of the tiles below it.
  // Two 32-bit hash lanes over the same inputs make a 52-bit key (an exact double).
  key(i, L, tx, ty) {
    i = this.resolve(i);
    if (i < 0) return 0;
    const mk = i + ':' + L + ':' + tx + ':' + ty;
    let k = this.memo.get(mk); if (k !== undefined) return k;
    const l = this.doc.layers[i];
    if (this.identity(l)) { k = this.key(this.top(i), L, tx, ty); this.memo.set(mk, k); return k; }
    const s = 2 ** L, R = this.tileRect(L, tx, ty), parts = [this.lh(l), L, tx * 4099 + ty, this.TS], b0 = this.below(i);
    const X0 = R.x * s, Y0 = R.y * s, X1 = (R.x + R.w) * s, Y1 = (R.y + R.h) * s;
    if (l.raster) parts.push(l.raster.contentKey(X0 - l.ox, Y0 - l.oy, X1 - l.ox, Y1 - l.oy));
    if (l.mask && l.mask.on) for (const c of l.mask.comps) if (c.raster && c.on) { const f = Math.ceil(blurReach(c.params.feather)) + 1; parts.push(c.raster.contentKey(X0 - f, Y0 - f, X1 + f, Y1 + f)); }
    const below = b => { parts.push(b % 67108864, Math.floor(b / 67108864)); };
    if (KINDS[l.kind].isGroup) { below(this.key(b0, L, tx, ty)); below(this.key(this.top(i), L, tx, ty)); }   // the backdrop, and what the children made
    else if (!(KINDS[l.kind].src && l.blend === 'normal' && this.opaqueSource(l))) {
      const m = this.reach(l, L);
      if (m) {
        const g = this.grid(L), T = this.TS;
        const tx0 = Math.max(0, Math.floor((R.x - m) / T)), tx1 = Math.min(g.nx - 1, Math.floor((R.x + R.w - 1 + m) / T));
        const ty0 = Math.max(0, Math.floor((R.y - m) / T)), ty1 = Math.min(g.ny - 1, Math.floor((R.y + R.h - 1 + m) / T));
        for (let y = ty0; y <= ty1; y++) for (let x = tx0; x <= tx1; x++) below(this.key(b0, L, x, y));
      } else below(this.key(b0, L, tx, ty));
    }   // an opaque full-cover fill hides everything below, so its key ignores it
    let a = 0x811c9dc5, b = 0x2545f491;
    for (const v of parts) { a = hmix(a, v); b = hmix(b, v ^ 0x5bd1e995); }
    k = a * 1048576 + (b >>> 12) || 1;
    this.memo.set(mk, k);
    return k;
  }
  opaqueSource(l) { return l.kind === 'fill' && l.opacity >= 1 && !(l.mask && l.mask.on); }

  emptyTile(n) { let e = this.empty.get(n); if (!e) { e = new Float32Array(n * 4); this.empty.set(n, e); } return e; }
  // The composite after layer i for one tile. Returned buffers are shared with the cache: never write to them.
  tile(i, L, tx, ty) {
    i = this.resolve(i);
    const R = this.tileRect(L, tx, ty);
    if (i < 0) return { buf: this.emptyTile(R.w * R.h), w: R.w, h: R.h };
    const l = this.doc.layers[i];
    if (this.identity(l)) return this.tile(this.top(i), L, tx, ty);
    const k = this.key(i, L, tx, ty);
    const hit = this.cache.get(k);
    if (hit) return hit;
    const m = this.reach(l, L), b0 = this.below(i), grp = KINDS[l.kind].isGroup;
    let below, top = null;
    if (grp) { below = this.tile(b0, L, tx, ty).buf; const t = this.top(i); top = t >= 0 ? this.tile(t, L, tx, ty).buf : this.emptyTile(R.w * R.h); }
    else if (KINDS[l.kind].src && l.blend === 'normal' && this.opaqueSource(l)) below = this.emptyTile(R.w * R.h);
    else below = m ? this.rect(b0, L, R.x - m, R.y - m, R.w + 2 * m, R.h + 2 * m) : this.tile(b0, L, tx, ty).buf;
    const t0 = performance.now();
    const buf = grp ? applyGroup(this, l, L, R, below, top) : applyLayer(this, l, L, R, below, m);
    const dt = performance.now() - t0;
    this.layerMs.set(l.id, (this.layerMs.get(l.id) || 0) + dt); this.applyMs += dt; this.computed++;
    const e = { buf, w: R.w, h: R.h, x: R.x, y: R.y, cost: dt };
    this.cache.put(k, e);
    return e;
  }
  // Any rectangle of the composite after layer i, edges extended outside the image.
  rect(i, L, x, y, w, h) {
    const out = new Float32Array(w * h * 4), D = this.dims(L), T = this.TS;
    const cx0 = clamp(x, 0, D.w - 1), cy0 = clamp(y, 0, D.h - 1), cx1 = clamp(x + w, cx0 + 1, D.w), cy1 = clamp(y + h, cy0 + 1, D.h);
    for (let ty = Math.floor(cy0 / T); ty * T < cy1; ty++) for (let tx = Math.floor(cx0 / T); tx * T < cx1; tx++) {
      const t = this.tile(i, L, tx, ty), ox = tx * T, oy = ty * T;
      const ix0 = Math.max(cx0, ox), ix1 = Math.min(cx1, ox + t.w), iy0 = Math.max(cy0, oy), iy1 = Math.min(cy1, oy + t.h);
      for (let yy = iy0; yy < iy1; yy++) out.set(t.buf.subarray(((yy - oy) * t.w + ix0 - ox) * 4, ((yy - oy) * t.w + ix1 - ox) * 4), ((yy - y) * w + ix0 - x) * 4);
    }
    // extend edges: rows inside the image first, then the rows above and below
    const lx = cx0 - x, rx = cx1 - x;
    for (let yy = Math.max(cy0, y); yy < Math.min(cy1, y + h); yy++) {
      const row = (yy - y) * w * 4;
      for (let xx = 0; xx < lx; xx++) out.copyWithin(row + xx * 4, row + lx * 4, row + lx * 4 + 4);
      for (let xx = rx; xx < w; xx++) out.copyWithin(row + xx * 4, row + (rx - 1) * 4, row + rx * 4);
    }
    const top = (cy0 - y) * w * 4, bot = (cy1 - 1 - y) * w * 4;
    for (let yy = 0; yy < cy0 - y; yy++) out.copyWithin(yy * w * 4, top, top + w * 4);
    for (let yy = cy1 - y; yy < h; yy++) out.copyWithin(yy * w * 4, bot, bot + w * 4);
    return out;
  }
  topIndex() { return this.doc.layers.length - 1; }
  // Coverage at each pixel of R (opacity aside): the mask's components combined in order, then its
  // invert and density. `below` is the backdrop, m pixels wider on each side, for range components.
  coverage(l, L, R, below, m = 0) {
    const mk = l.mask; if (!(mk && mk.on)) return null;
    const n = R.w * R.h, out = new Float32Array(n), cv = new Float32Array(n), W = this.doc.w, H = this.doc.h, s = 2 ** L, BW = R.w + 2 * m;
    let first = true;
    for (const c of mk.comps) {
      if (!c.on) continue;
      this.compValues(c, L, R, below, m, BW, W, H, s, cv);
      const amt = c.amount, inv = c.invert, op = first ? 'add' : c.op;
      for (let j = 0; j < n; j++) { let v = cv[j]; if (inv) v = 1 - v; v *= amt; out[j] = first ? v : combine(op, out[j], v); }
      first = false;
    }
    const d = mk.density;
    for (let j = 0; j < n; j++) { let v = out[j]; if (mk.invert) v = 1 - v; out[j] = 1 - d * (1 - v); }
    return out;
  }
  compValues(c, L, R, below, m, BW, W, H, s, cv) {
    const p = c.params;
    if (c.type === 'linear') {
      const ax = p.x0 * W, ay = p.y0 * H, dx = (p.x1 - p.x0) * W, dy = (p.y1 - p.y0) * H, l2 = dx * dx + dy * dy || 1;
      for (let y = 0, j = 0; y < R.h; y++) { const Y = (R.y + y + 0.5) * s - ay; for (let x = 0; x < R.w; x++, j++) cv[j] = 1 - s01(((R.x + x + 0.5) * s - ax) * dx / l2 + Y * dy / l2); }
    } else if (c.type === 'radial') {
      const cx = p.cx * W, cy = p.cy * H, a = p.angle * Math.PI / 180, co = Math.cos(a), si = Math.sin(a), irx = 1 / Math.max(1e-3, p.rx * W), iry = 1 / Math.max(1e-3, p.ry * H), inner = 1 - p.feather, span = Math.max(1e-4, p.feather);
      for (let y = 0, j = 0; y < R.h; y++) { const v0 = (R.y + y + 0.5) * s - cy; for (let x = 0; x < R.w; x++, j++) {
        const u0 = (R.x + x + 0.5) * s - cx, u = (u0 * co + v0 * si) * irx, v = (-u0 * si + v0 * co) * iry, d = Math.sqrt(u * u + v * v);
        cv[j] = d <= inner ? 1 : 1 - s01((d - inner) / span); } }
    } else if (c.type === 'brush') {
      if (!c.raster) { cv.fill(0); return; }
      const key = c.raster.id + ':' + c.raster.rev + ':' + p.feather + ':' + L;
      let lv = this.maskCache.get(key);
      if (!lv) {
        const src = c.raster.level(L), f = new Float32Array(src.w * src.h); for (let i = 0; i < f.length; i++) f[i] = src.data[i] / 255;
        if (p.feather > 0) gaussBlur(f, src.w, src.h, 1, p.feather / s);
        lv = { w: src.w, h: src.h, f };
        for (const k of this.maskCache.keys()) if (k.startsWith(c.raster.id + ':') && k.endsWith(':' + L)) this.maskCache.delete(k);
        this.maskCache.set(key, lv);
      }
      for (let y = 0, j = 0; y < R.h; y++) { const row = Math.min(lv.h - 1, R.y + y) * lv.w; for (let x = 0; x < R.w; x++, j++) cv[j] = lv.f[row + Math.min(lv.w - 1, R.x + x)]; }
    } else {
      const lumR = c.type === 'luminance', tc = lumR ? null : hexToRgb(p.color), isq3 = 1 / Math.sqrt(3), sf = Math.max(1e-4, p.soft);
      for (let y = 0, j = 0; y < R.h; y++) for (let x = 0; x < R.w; x++, j++) {
        const i = ((y + m) * BW + x + m) * 4, a = below[i + 3];
        if (a <= 0) { cv[j] = 0; continue; }
        const r = below[i] / a, g = below[i + 1] / a, b = below[i + 2] / a;
        if (lumR) { const v = lum(r, g, b); cv[j] = (p.lo <= 0 ? 1 : smooth(p.lo - p.soft, p.lo + p.soft, v)) * (p.hi >= 1 ? 1 : 1 - smooth(p.hi - p.soft, p.hi + p.soft, v)); }
        else { const dist = Math.sqrt((r - tc[0]) ** 2 + (g - tc[1]) ** 2 + (b - tc[2]) ** 2) * isq3; cv[j] = 1 - s01((dist - p.tol) / sf); }
      }
    }
  }
}

// Apply one layer to the tile R, given the composite below (expanded by m pixels for reaching ops).
function applyLayer(eng, l, L, R, below, m) {
  const K = kindOf(l), n = R.w * R.h, out = new Float32Array(n * 4), op = l.opacity, mode = l.blend;
  const cov = eng.coverage(l, L, R, below, m);
  const D = eng.dims(L), s = 2 ** L;
  const o3 = [0, 0, 0];
  if (K.src) {
    const src = scratch(2, n * 4); src.fill(0);
    sourcePixels(l, L, R, src, eng);
    return blendSource(src, below, n, op, cov, mode, out);
  }
  if (K.roi) {
    const W = R.w + 2 * m, H = R.h + 2 * m, buf = scratch(1, below.length); buf.set(below);
    if (K.code) { try { K.roiFn(buf, W, H, l.params, L); } catch (e) { codeFail(K, e); buf.set(below); } } else K.roiFn(buf, W, H, l.params, L);
    for (let y = 0; y < R.h; y++) for (let x = 0; x < R.w; x++) {
      const j = y * R.w + x, i = j * 4, e = ((y + m) * W + x + m) * 4, k = op * (cov ? cov[j] : 1);
      const ab = below[e + 3], aa = buf[e + 3];
      if (k <= 0) { out[i] = below[e]; out[i + 1] = below[e + 1]; out[i + 2] = below[e + 2]; out[i + 3] = ab; continue; }
      if (mode === 'normal' || ab <= 0 || aa <= 0) { for (let c = 0; c < 4; c++) out[i + c] = below[e + c] + k * (buf[e + c] - below[e + c]); continue; }
      const br = below[e] / ab, bg = below[e + 1] / ab, bb = below[e + 2] / ab;
      blend3(mode, br, bg, bb, buf[e] / aa, buf[e + 1] / aa, buf[e + 2] / aa, o3);
      const a = ab + k * (aa - ab);
      out[i] = (br + k * (o3[0] - br)) * a; out[i + 1] = (bg + k * (o3[1] - bg)) * a; out[i + 2] = (bb + k * (o3[2] - bb)) * a; out[i + 3] = a;
    }
    return out;
  }
  // pointwise adjustment: unpremultiply, adjust, mix back by coverage through the blend mode
  const t = scratch(0, n * 4);
  for (let i = 0; i < n * 4; i += 4) { const a = below[i + 3]; if (a > 0) { t[i] = below[i] / a; t[i + 1] = below[i + 1] / a; t[i + 2] = below[i + 2] / a; } else t[i] = t[i + 1] = t[i + 2] = 0; t[i + 3] = a; }
  const ctx = { x0: R.x, y0: R.y, w: R.w, s, W: eng.doc.w, H: eng.doc.h, L, prep: K.prep ? eng.prep(l) : null };
  if (K.code) { try { K.fn(t, n, l.params, ctx); } catch (e) { codeFail(K, e); for (let i = 0; i < n * 4; i += 4) { const a = below[i + 3]; if (a > 0) { t[i] = below[i] / a; t[i + 1] = below[i + 1] / a; t[i + 2] = below[i + 2] / a; } else t[i] = t[i + 1] = t[i + 2] = 0; } } } else K.fn(t, n, l.params, ctx);
  const normal = mode === 'normal';
  for (let j = 0, i = 0; j < n; j++, i += 4) {
    const a = below[i + 3], k = cov ? op * cov[j] : op;
    if (a <= 0 || k <= 0) { out[i] = below[i]; out[i + 1] = below[i + 1]; out[i + 2] = below[i + 2]; out[i + 3] = a; continue; }
    const br = below[i] / a, bg = below[i + 1] / a, bb = below[i + 2] / a;
    let r = t[i], g = t[i + 1], b = t[i + 2];
    r = r < 0 ? 0 : r > 1 ? 1 : r; g = g < 0 ? 0 : g > 1 ? 1 : g; b = b < 0 ? 0 : b > 1 ? 1 : b;
    if (!normal) { blend3(mode, br, bg, bb, r, g, b, o3); r = o3[0]; g = o3[1]; b = o3[2]; }
    out[i] = (br + k * (r - br)) * a; out[i + 1] = (bg + k * (g - bg)) * a; out[i + 2] = (bb + k * (b - bb)) * a; out[i + 3] = a;
  }
  return out;
}

// Straight colour and alpha of a source layer over the tile R.
// A source over a backdrop: src is straight colour with alpha, below and out premultiplied.
function blendSource(src, below, n, op, cov, mode, out) {
  const o3 = [0, 0, 0];
  for (let j = 0, i = 0; j < n; j++, i += 4) {
    const as = src[i + 3] * op * (cov ? cov[j] : 1), ab = below[i + 3];
    if (as <= 0) { out[i] = below[i]; out[i + 1] = below[i + 1]; out[i + 2] = below[i + 2]; out[i + 3] = ab; continue; }
    const sr = src[i], sg = src[i + 1], sb = src[i + 2];
    if (mode === 'normal' || ab <= 0) {
      out[i] = sr * as + below[i] * (1 - as); out[i + 1] = sg * as + below[i + 1] * (1 - as); out[i + 2] = sb * as + below[i + 2] * (1 - as);
    } else {
      const br = below[i] / ab, bg = below[i + 1] / ab, bb = below[i + 2] / ab;
      blend3(mode, br, bg, bb, sr, sg, sb, o3);
      out[i] = as * (1 - ab) * sr + as * ab * o3[0] + (1 - as) * below[i];
      out[i + 1] = as * (1 - ab) * sg + as * ab * o3[1] + (1 - as) * below[i + 1];
      out[i + 2] = as * (1 - ab) * sb + as * ab * o3[2] + (1 - as) * below[i + 2];
    }
    out[i + 3] = as + ab * (1 - as);
  }
  return out;
}
// A group's tile. Pass through: mix from the backdrop towards what the children made, by opacity and mask.
// Isolated: the children's composite (premultiplied) becomes a straight-colour source over the backdrop.
function applyGroup(eng, g, L, R, back, top) {
  const n = R.w * R.h, out = new Float32Array(n * 4), op = g.opacity, cov = eng.coverage(g, L, R, back, 0);
  if (g.blend === 'pass') {
    for (let j = 0, i = 0; j < n; j++, i += 4) { const k = op * (cov ? cov[j] : 1); for (let c = 0; c < 4; c++) out[i + c] = back[i + c] + k * (top[i + c] - back[i + c]); }
    return out;
  }
  const src = scratch(2, n * 4);
  for (let i = 0; i < n * 4; i += 4) { const a = top[i + 3]; if (a > 0) { src[i] = top[i] / a; src[i + 1] = top[i + 1] / a; src[i + 2] = top[i + 2] / a; } else src[i] = src[i + 1] = src[i + 2] = 0; src[i + 3] = a; }
  return blendSource(src, back, n, op, cov, g.blend, out);
}
function sourcePixels(l, L, R, src, eng) {
  const n = R.w * R.h, s = 2 ** L;
  const K = kindOf(l);
  if (K.paint) { const ctx = { x0: R.x, y0: R.y, w: R.w, h: R.h, s, W: eng.doc.w, H: eng.doc.h, L }; if (K.code) { try { K.paint(src, n, l.params, ctx); } catch (e) { codeFail(K, e); src.fill(0); } } else K.paint(src, n, l.params, ctx); return; }
  const lv = l.raster.level(L), ox = Math.round(l.ox / s), oy = Math.round(l.oy / s), d = lv.data;
  for (let y = 0; y < R.h; y++) {
    const sy = R.y + y - oy; if (sy < 0 || sy >= lv.h) continue;
    for (let x = 0; x < R.w; x++) {
      const sx = R.x + x - ox; if (sx < 0 || sx >= lv.w) continue;
      const k = (sy * lv.w + sx) * 4, i = (y * R.w + x) * 4;
      src[i] = d[k] / 255; src[i + 1] = d[k + 1] / 255; src[i + 2] = d[k + 2] / 255; src[i + 3] = d[k + 3] / 255;
    }
  }
}

// Premultiplied float tile to 8-bit straight RGBA.
function toImageData(buf, w, h) {
  const im = new ImageData(w, h), d = im.data;
  for (let i = 0; i < w * h * 4; i += 4) { const a = buf[i + 3]; if (a > 0) { d[i] = buf[i] / a * 255; d[i + 1] = buf[i + 1] / a * 255; d[i + 2] = buf[i + 2] / a * 255; d[i + 3] = a * 255; } }
  return im;
}

// ---- Selections, lasso fill, flood fill -----------------------------------------------------------
function polyFill(w, h, pts) {
  const out = new Uint8ClampedArray(w * h); if (pts.length < 3) return out;
  const ys = pts.map(p => p[1]), y0 = clamp(Math.floor(Math.min(...ys)), 0, h - 1), y1 = clamp(Math.ceil(Math.max(...ys)), 0, h - 1);
  for (let y = y0; y <= y1; y++) {
    const yc = y + 0.5, xs = [];
    for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
      const [xi, yi] = pts[i], [xj, yj] = pts[j];
      if ((yi > yc) !== (yj > yc)) xs.push(xi + (yc - yi) * (xj - xi) / (yj - yi));
    }
    xs.sort((a, b) => a - b);
    for (let k = 0; k + 1 < xs.length; k += 2) { const a = clamp(Math.round(xs[k]), 0, w), b = clamp(Math.round(xs[k + 1]), 0, w); out.fill(255, y * w + a, y * w + b); }
  }
  return out;
}
// Magic wand: pixels within a tolerance of the seed's colour; contiguous uses a scanline fill.
function floodSelect(img, w, h, sx, sy, tol, contiguous) {
  const out = new Uint8ClampedArray(w * h), i0 = (sy * w + sx) * 4;
  const r0 = img[i0], g0 = img[i0 + 1], b0 = img[i0 + 2], a0 = img[i0 + 3];
  const ok = p => { const i = p * 4; return Math.max(Math.abs(img[i] - r0), Math.abs(img[i + 1] - g0), Math.abs(img[i + 2] - b0), Math.abs(img[i + 3] - a0)) <= tol; };
  if (!contiguous) { for (let p = 0; p < w * h; p++) if (ok(p)) out[p] = 255; return out; }
  const stack = [[sx, sy]];
  while (stack.length) {
    const [x, y] = stack.pop(); let xl = x, xr = x; const row = y * w;
    if (out[row + x] || !ok(row + x)) continue;
    while (xl > 0 && !out[row + xl - 1] && ok(row + xl - 1)) xl--;
    while (xr < w - 1 && !out[row + xr + 1] && ok(row + xr + 1)) xr++;
    out.fill(255, row + xl, row + xr + 1);
    for (const ny of [y - 1, y + 1]) {
      if (ny < 0 || ny >= h) continue;
      let inRun = false;
      for (let xx = xl; xx <= xr; xx++) { const p = ny * w + xx, can = !out[p] && ok(p); if (can && !inRun) { stack.push([xx, ny]); inRun = true; } else if (!can) inRun = false; }
    }
  }
  return out;
}
// Marching squares on a coarse copy of a mask: the outline as segments in document pixels.
function outline(r) {
  let L = 0; while (r.w / 2 ** L > 640) L++;
  const lv = r.level(L), w = lv.w, h = lv.h, d = lv.data, s = 2 ** L, segs = [];
  const v = (x, y) => x < 0 || y < 0 || x >= w || y >= h ? 0 : d[y * w + x] >= 128 ? 1 : 0;
  const P = (x, y) => [(x + 0.5) * s, (y + 0.5) * s];
  for (let y = -1; y < h; y++) for (let x = -1; x < w; x++) {
    const k = v(x, y) * 8 + v(x + 1, y) * 4 + v(x + 1, y + 1) * 2 + v(x, y + 1);
    if (k === 0 || k === 15) continue;
    const top = P(x + 0.5, y), right = P(x + 1, y + 0.5), bottom = P(x + 0.5, y + 1), left = P(x, y + 0.5);
    const add = (p, q) => segs.push(p[0], p[1], q[0], q[1]);
    switch (k) {
      case 1: case 14: add(left, bottom); break; case 2: case 13: add(bottom, right); break; case 3: case 12: add(left, right); break;
      case 4: case 11: add(top, right); break; case 6: case 9: add(top, bottom); break; case 7: case 8: add(left, top); break;
      case 5: add(left, top); add(bottom, right); break; case 10: add(top, right); add(left, bottom); break;
    }
  }
  return segs;
}
