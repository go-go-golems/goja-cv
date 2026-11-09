// dust/helpers.js: helpers for annotator agents (a model task library)
// Load: await cv.load("dust/helpers.js"), or goja-cv mcp … --read DIR --prelude DIR/dust/helpers.js
// The comment that opens each function's body is its doc line in the context block.

mem.lin = img.as("linear")

mem.probe = async (x, y, r = 3) => { // core/ring measure at (x, y): { cx, cy, step, spread, snr }
  const [m] = await mem.lin.measure([{ cx: x, cy: y, r }])
  return m
}

mem.spots = async (level = 0.3) => { // blobs of the speck view above level, measured on the linear image
  const blobs = await img.view("speck").mask.luminance({ above: level }).components({ minArea: 2 })
  return await mem.lin.measure(blobs)
}

mem.sheet = (items, size = 40) => // contact sheet of up to 15 items: image | speck view, captioned with their ids
  cv.sheet(items.slice(0, 15), { views: [img, img.view("speck")], size, zoom: 4, stretch: "cell" })

return "dust helpers ready"
