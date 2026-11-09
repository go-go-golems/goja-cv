# Recipes

Worked evals for the common jobs: check and fix exposure, find and remove dust, measure inside a box or region, edit inside a mask, compare before and after, inspect candidates in a sheet and zoomed crops, mark points. Each block runs as written.

## Check exposure

```js
const s = await img.stats()
return {
  meanL: s.meanL,          // mean linear luminance (mid-grey is about 0.18)
  black: s.clipLow,        // fraction of pixels clipped to black
  white: s.clipHigh,       // fraction clipped to white
  p01: s.p01, p99: s.p99,
  hist: img.histogram().look("histogram"),
}
```

## Fix exposure

Stretch the 1st–99th percentiles to black and white, then lift the midtones, and compare.

```js
const s = img.stats()
const fixed = img.levels({ black: s.p01, white: s.p99 }).exposure(0.25)
show(cv.compare([img, fixed], { mode: "side" }), { label: "before / after" })
mem.fixed = fixed
return await fixed.stats().pick("meanL", "clipLow", "clipHigh")
```

## Remove dust

Detect, look, heal, check. Specks with `score ≥ 0.5` are dust; soft round blobs (stars, highlights) score low.

```js
const specks = img.detect.specks()
const dust = specks.where({ score: { gte: 0.5 } })
mem.dust = dust
return {
  verdict: await img.check.dust(),
  biggest: (await dust.sortBy("area").top(3)).items.map(r => ({ id: r.id, x: r.x, y: r.y, w: r.w, h: r.h, score: r.score })),
  look: img.overlay(dust, { labels: "id" }).look("dust"),
}
```

```js
const healed = img.heal(mem.dust, { radius: 3 })
show(cv.compare([img, healed], { mode: "split" }), { label: "before | healed" })
return { after: await healed.check.dust() }
```

To keep a speck that is real detail, drop it by id before healing: `img.heal(mem.dust.without([31]))`.

## Measure a spot, a box or regions

`img.measure(points)` says how much each point stands out from a ring around it (step, spread, snr, stepChroma); `img.sample(points)` reads values; `img.stats({ box })` or `{ mask }` measures inside an area (plus `area`, `coverage`). Region items are points and boxes. Details: `cv.docs.read("measuring")`.

```js
const top = (await img.detect.specks().sortBy("area").top(2)).items
return {
  measured: (await img.measure(top)).map(r => [r.cx, r.cy, r.snr.toFixed(2)].join(" ")),
  inside: await img.stats({ box: top[0] }).pick("meanL", "area"),
}
```

## Rank candidates

Blobs of the DoG, measured on the scan and ranked by snr: `hot.measure(img, { ring: true }).sortBy("snr").top(20)`; return `cv.csv(ranked)` for every row. The full recipe is in `cv.docs.read("measuring", { section: "Measuring regions" })`.

## Edit inside a mask

Darken the top of the frame with a soft gradient mask:

```js
const top = img.mask.gradient({ from: "top", to: 0.5 }).feather(10)
const out = img.within(top, i => i.exposure(-0.6))
return { mask: top.look("mask"), out: out.look("darker top") }
```

## Compare

`cv.compare([a, b, …], { mode })` lays images out: `"side"` by side or `"split"` into strips. A difference is an image of its own: `a.sub(b)` keeps the sign (positive where `a` is brighter); `.mul(4).add(0.5)` shows zero as mid-grey.

```js
const blurred = img.gaussian(3)
return { side: cv.compare([img, blurred]).look("side"), diff: img.sub(blurred).mul(8).add(0.5).look("img − blurred, ×8, zero = grey") }
```

## Inspect candidates

Make small bright things stand out, find them, look at all of them in one sheet, then zoom on the doubtful ones with coordinates.

```js looks=2
const speck = img.view("speck")
const spots = speck.mask.luminance({ above: 0.5 }).components({ minArea: 4 })
const top = spots.sortBy("area").top(12)
const first = (await top).items[0]
return {
  sheet: cv.sheet(top, { views: [img, speck], size: 32, zoom: 4 }).look("candidates"),
  zoom: speck.crop([Math.round(first.cx) - 16, Math.round(first.cy) - 16, 32, 32]).look({ zoom: 8, ruler: 4 }),
}
```

## Compare two versions

`a.sub(b)` is signed (positive where `a` is brighter); show it grey with zero at 0.5, or in the diverging colormap on one channel.

```js looks=2
const healed = img.heal(img.detect.specks().where({ score: { gte: 0.5 } }))
const d = img.sub(healed)
return { grey: d.mul(4).add(0.5).look("img − healed, ×4"), map: d.toGray("average").colormap("diverging").look("blue darker, red brighter") }
```

## Mark points

Draw your own marks (absolute doc px) to show what you mean, then zoom.

```js looks=1
const marked = img.annotate([
  { circle: [200.5, 46.5, 6], color: "lime", label: "star" },
  { point: [160, 30] },
  { box: [170, 60, 12, 10], color: "#ff8800" },
  { text: "hair?", at: [185, 25] },
])
return marked.crop([150, 20, 64, 64]).look({ zoom: 4, ruler: 8 })
```

## Not yet

The document layer (history, `doc.apply`, `doc.propose`, undo, export, recipes over many files) is not available. Describe the edit you would apply and show it as looks.

## Open scientific data

Linear data (float TIFF, `.npy`, a DNG raw scan, density stored as plain codes) opens with `encoding: "linear"` (implied for float TIFF, `.npy` and DNG); then linear and encoded spaces coincide and every number is in file units. A window of a large scan keeps the scan's coordinates.

```js
const v = cv.open("planted-linear.tif", { encoding: "linear", as: "v" })
const s = await v.stats()
return { encoding: (await v).encoding, p01: s.p01, p99: s.p99, look: v.levels({ black: s.p01, white: s.p99 }).look("stretched") }
```

## Neutral or coloured?

`img.chroma({ relative: true })` is 0 where the channels agree. Measure it inside a spot to tell neutral dust from coloured grain, and look at it in false colour.

```js
const spots = img.detect.specks().where({ score: { gte: 0.5 } }).top(5)
const c = img.chroma({ relative: true })
return { spotChroma: await c.stats({ mask: spots }).pick("meanL"), look: c.colormap("viridis").look("chroma") }
```

