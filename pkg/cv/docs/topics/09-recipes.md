# Recipes

Worked evals for the common jobs: check and fix exposure, find and remove dust, measure inside a box or region, edit inside a mask, compare before and after. Each block runs as written.

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
  biggest: (await dust.sortBy("area").top(3)).items.map(r => ({ id: r.id, box: r.box, score: r.score })),
  look: img.overlay(dust, { labels: "id" }).look("dust"),
}
```

```js
const healed = img.heal(mem.dust, { radius: 3 })
show(cv.compare([img, healed], { mode: "split" }), { label: "before | healed" })
return { after: await healed.check.dust() }
```

To keep a speck that is real detail, drop it by id before healing: `img.heal(mem.dust.without([31]))`.

## Measure inside a box or region

`img.stats({ box: [x, y, w, h] })` (doc px) and `img.stats({ mask })` measure only those pixels (a mask weights them; Regions count as their pixels). The result has the usual fields plus `area` (doc px²) and `coverage`. Use it to compare a speck with its surroundings or to probe a highlight; there is no need to darken everything else first.

```js
const specks = img.detect.specks()
const top = (await specks.sortBy("area").top(1)).items[0]
const [x, y, w, h] = top.box
const ring = img.mask.box([x - 6, y - 6, w + 12, h + 12]).minus(img.mask.box(top.box))
return {
  id: top.id,
  inside: await img.stats({ box: top.box }).pick("meanL", "p99", "area"),
  around: await img.stats({ mask: ring }).pick("meanL", "area"),
  dust: await img.stats({ mask: specks.where({ score: { gte: 0.5 } }) }).pick("meanL", "area"),
}
```

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

