// dust/helpers.js: helpers for annotator agents on DUST-002 gold tiles (goja-cv v2)
// Load: await cv.load("dust/helpers.js"), or goja-cv mcp … --read DIR --prelude DIR/dust/helpers.js
// The comment that opens each function's body is its doc line in the context block.

mem.img = img
mem.lin = img.as("linear")

mem.tile = (id, ox, oy) => { // open a tile at its origin: every coordinate is an absolute scan coordinate
  mem.img = cv.open("cv_tiles/" + id + ".tif", { encoding: "linear", origin: [ox, oy], name: "img" })
  mem.lin = mem.img.as("linear")
  return mem.img
}

mem.spots = async ({ snr = 1, radius = [2, 4, 8] } = {}) => { // peaks with the speck view's step: sp = spStep
  const pk = await mem.lin.peaks({ radius, snr })
  return await mem.lin.view("speck").measure(pk, { prefix: "sp" })
}

mem.sheet = (items, size = 40) => // contact sheet of up to 15 items: tile | speck view, captioned with their ids
  cv.sheet(items.slice(0, 15), { views: [mem.img, mem.img.view("speck")], size, zoom: 4, stretch: "cell" })

return "dust helpers ready"
