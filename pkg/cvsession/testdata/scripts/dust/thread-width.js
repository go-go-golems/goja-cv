// dust/thread-width.js: the width of a bright thread across the segment a → b (doc px), full width at half height
// args: { a: [x, y], b: [x, y], along: 3 }   (a and b on either side of the thread; along: px averaged along it)
const p = await img.as("linear").profile(args.a, args.b, { step: 0.25, width: args.along ?? 3 })
const v = p.rows.map(r => r.meanL), t = p.rows.map(r => r.t), n = v.length
const base = (v[0] + v[n - 1]) / 2
let k = 0
for (let i = 1; i < n; i++) if (v[i] > v[k]) k = i
const half = base + (v[k] - base) / 2
const cross = d => { // where the profile falls below half, walking from the peak
  let i = k
  while (i + d >= 0 && i + d < n && v[i + d] > half) i += d
  const j = i + d
  if (j < 0 || j >= n) return null // it never does: the segment is too short
  return t[i] + (t[j] - t[i]) * (v[i] - half) / (v[i] - v[j])
}
const lo = cross(-1), hi = cross(1)
return { width: lo === null || hi === null ? null : hi - lo, height: v[k] - base, at: [p.rows[k].cx, p.rows[k].cy] }
