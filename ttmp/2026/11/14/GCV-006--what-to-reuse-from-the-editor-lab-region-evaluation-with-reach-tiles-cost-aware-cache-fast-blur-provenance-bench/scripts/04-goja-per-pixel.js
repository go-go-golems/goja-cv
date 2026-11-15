// 04-goja-per-pixel: a JavaScript function called once per pixel over 1 MP in goja, against two Go point ops.
// Run: goja-cv eval --root DIR_WITH_BIG_NPY -e "$(cat 04-goja-per-pixel.js)"
const n = 1024 * 1024, b = new Float32Array(n * 3)
for (let i = 0; i < b.length; i++) b[i] = (i % 97) / 97
const fn = (r, g, bl) => Math.min(1, 0.2126 * r + 0.7152 * g + 0.0722 * bl) * 1.1
let t0 = Date.now()
for (let i = 0; i < n; i++) { const v = fn(b[3*i], b[3*i+1], b[3*i+2]); b[3*i] = v }
const perPixel = Date.now() - t0
const img = cv.open("big.npy", { encoding: "linear" }).crop([0, 0, 1024, 1024])
await img.stats()
t0 = Date.now(); await img.mul(1.1).add(0.01).stats(); const goOps = Date.now() - t0
return { goja_per_pixel_fn_1MP_ms: perPixel, go_two_point_ops_1MP_ms: goOps }
