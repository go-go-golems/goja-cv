// helpers-fixture.js: the shapes of the DUST-002 annotator helpers (v1), for the doc-line parser
mem.T = "/tiles/"
const r3 = { coreRadius: 3, ringRadii: [6, 12] }
mem.tile = (id, ox, oy) => { // open a tile at its origin: every coordinate is an absolute scan coordinate
  mem.img = cv.open(mem.T + id + ".tif", { encoding: "linear", origin: [ox, oy], name: "img" })
  return mem.img
}
mem.best = async (node, x, y, opts) => {
  const pts = [] // best by its snr; we report its step
  for (let dy = -2; dy <= 2; dy++) for (let dx = -2; dx <= 2; dx++) if (dx * dx + dy * dy <= 4) pts.push([x + dx, y + dy])
  const rows = await node.measure(pts, opts)
  return rows.reduce((a, b) => (b.snr > a.snr ? b : a))
}
mem.probe = async (x, y) => {
  const t = await mem.best(mem.img, x, y, r3) // a mid-body comment is not a doc line
  return { x: t.cx, y: t.cy, snr: t.snr }
}
mem.hot = async (level = 0.4, max = 80) => { /* components of the speck view above level,
   each probed and sorted by sp */
  return []
}
mem.spots = (level = 0.4) => // blobs of the speck view above level, as regions
  mem.img.view("speck").mask.luminance({ above: level }).components({ minArea: 4 })
mem.sweep = async function sweep({ q = 0, zoom = 2 } = {}) { // one quarter of the speck view, scaled on its own
  return q + zoom
}
mem.one = x => /* the identity */ x
mem.first = async () => {
  // a comment on the line after the brace is still the first token
  return 1
}
mem.lib = {
  sheet(items, size = 40) { // contact sheet of up to 15 items
    return items.slice(0, 15).length + size
  },
  get count() { return 1 },
  set count(v) {},
  *gen(a) { // a generator method
    yield a
  },
  get(k) { // a method named get
    return k
  },
}
mem.bound = mem.one.bind(null)
return "helpers ready (v1)"
