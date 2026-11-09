# Recipes

Worked evals for the common jobs: check and fix exposure, find and remove dust, edit inside a mask, compare before and after. Each block runs as written.

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

## Edit inside a mask

Darken the top of the frame with a soft gradient mask:

```js
const top = img.mask.gradient({ from: "top", to: 0.5 }).feather(10)
const out = img.within(top, i => i.exposure(-0.6))
return { mask: top.look("mask"), out: out.look("darker top") }
```

## Compare

`cv.compare([a, b, …], { mode })`: `"side"` by side, `"split"` into strips, `"diff"` the amplified difference of two.

```js
const blurred = img.gaussian(3)
return { side: cv.compare([img, blurred]).look("side"), diff: cv.compare([img, blurred], { mode: "diff", gain: 8 }).look("diff") }
```

## Not yet

The document layer (history, `doc.apply`, `doc.propose`, undo, export, recipes over many files) is not available. Describe the edit you would apply and show it as looks.
