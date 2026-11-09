// mem.probe from the colleague's helpers (DUST-002 scripts/84-cv-helpers.js),
// copied for the equivalence test of img.measure (GCV-003 §13.2). The only
// change: the returned numbers are not rounded (the original applies
// toFixed(3) / toFixed(2)), so they can be compared within 1e-6.
mem.probe = async (x, y, r = 3) => {
  const core = img.mask.ellipse([x - r, y - r, 2 * r + 1, 2 * r + 1])
  const ring = img.mask.ellipse([x - r - 9, y - r - 9, 2 * r + 19, 2 * r + 19]).minus(img.mask.ellipse([x - r - 3, y - r - 3, 2 * r + 7, 2 * r + 7]))
  const c = await img.stats({ mask: core })
  const g = await img.stats({ mask: ring, percentiles: [0.1, 0.9] })
  const ch = ["r", "g", "b"].map(k => c.perChannel[k].mean - g.perChannel[k].mean)
  const step = (ch[0] + ch[1] + ch[2]) / 3
  const spread = g.percentiles["0.9"] - g.percentiles["0.1"]
  return { x, y, r, step, ch, ringSpread: spread, snr: step / Math.max(spread, 1e-3),
    chroma: (Math.max(...ch) - Math.min(...ch)) / Math.max(Math.abs(step), 1e-3) }
}
return "probe ready"
