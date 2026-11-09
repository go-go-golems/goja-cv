---
Title: goja-cv recipes
Slug: recipes
Short: Check and fix exposure, find and heal dust, measure inside a box, edit inside a mask, compare
Topics:
- goja-cv
- recipes
Commands:
- eval
- run
- mcp
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Example
---

Each block is an eval body: paste it into `goja-cv eval --image scan.tif -e '…'`, save it as a script for
`goja-cv run`, or let an agent send it. All of them run as written (they are tested on a synthetic scan). The
agent-facing versions are in `cv.docs.read("recipes")`.

## Check exposure

```js
const s = await img.stats()
const info = await img
return {
  file: info.file, bitDepth: info.bitDepth, size: [info.docW, info.docH],
  meanL: s.meanL, p01: s.p01, p99: s.p99, crushed: s.clipLow, blown: s.clipHigh,
  histogram: img.histogram().look("histogram"),
}
```

`meanL` is linear luminance (mid-grey is about 0.18); `clipLow`/`clipHigh` are the fractions of clipped pixels.

## Fix exposure and compare

```js
const s = img.stats()
const fixed = img.levels({ black: s.p01, white: s.p99 }).exposure(0.25)
show(cv.compare([img, fixed], { mode: "side" }), { label: "before / after" })
return await fixed.stats().pick("meanL", "clipLow", "clipHigh")
```

## Find dust, look at it, heal it

```js
const specks = img.detect.specks()
const dust = specks.where({ score: { gte: 0.5 } })
const healed = img.heal(dust, { radius: 3 })
mem.healed = healed
return {
  verdict: (await img.check.dust()).verdict,
  found: await dust.count(),
  notDust: (await specks.where({ score: { lt: 0.5 } })).items.map(r => ({ id: r.id, box: r.box, sharpness: r.sharpness })),
  after: (await healed.check.dust()).verdict,
  look: cv.compare([img.overlay(dust, { labels: "id", color: "lime" }), healed], { mode: "side" }).look("dust | healed"),
}
```

Specks scoring below 0.5 are soft, round features (stars, highlights) and are left alone. To keep one real
detail, drop it by id: `img.heal(dust.without([31]))`.

## Measure inside a box or region

```js
const speck = (await img.detect.specks().sortBy("area").top(1)).items[0]
const [x, y, w, h] = speck.box
const ring = img.mask.box([x - 6, y - 6, w + 12, h + 12]).minus(img.mask.box(speck.box))
return {
  id: speck.id,
  inside: await img.stats({ box: speck.box }).pick("meanL", "area"),
  around: await img.stats({ mask: ring }).pick("meanL", "area"),
}
```

## Edit inside a mask

```js
const top = img.mask.gradient({ from: "top", to: 0.5 }).feather(10)
const darker = img.within(top, i => i.exposure(-0.6))
return { mask: top.look("mask"), result: darker.look("darker top"), meanTop: await darker.stats({ mask: top }).meanL }
```

## Hand a person a slider

```js
const ev = P.slider("ev", { min: -2, max: 2, value: 0.3, label: "Exposure", unit: "EV" })
const out = img.exposure(ev)
return { meanL: await out.stats().meanL, look: out.look("ev") }
```

Moving the param (a host UI, or `P.set("ev", 0.8)` in a later eval) recomputes only what depends on it; the
context block reports the move.
