---
Title: goja-cv recipes
Slug: recipes
Short: Check and fix exposure, find and heal dust, inspect defects up close, measure inside a box, edit inside a mask, compare
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
  file: info.file, bitDepth: info.bitDepth, frame: info.frame,
  meanL: s.meanL, p01: s.p01, p99: s.p99, crushed: s.clipLow, blown: s.clipHigh,
  histogram: img.histogram().look("histogram"),
}
```

`meanL` is luminance in file units, the scale a look shows (mid-grey is about 0.46; `img.as("linear").stats()` for linear light, where it is 0.18); `clipLow`/`clipHigh` are the fractions of clipped pixels.

## Fix exposure and compare

```js
const s = await img.stats()     // options are numbers: await the measurement first
const fixed = img.levels({ black: s.p01, white: s.p99 }).exposure(0.25)
cv.compare([img, fixed], { mode: "side" }).look("before / after")
const { meanL, clipLow, clipHigh } = await fixed.stats()
return { meanL, clipLow, clipHigh }
```

## Find dust, look at it, heal it

```js
const specks = await img.detect.specks()
const dust = specks.filter(r => r.score >= 0.5)
const healed = img.heal(dust, { radius: 3 })
mem.healed = healed
return {
  verdict: (await img.check.dust()).verdict,
  found: dust.length,
  notDust: specks.filter(r => r.score < 0.5).map(r => ({ id: r.id, cx: r.cx, cy: r.cy, sharpness: r.sharpness })),
  after: (await healed.check.dust()).verdict,
  look: cv.compare([img.overlay(dust, { labels: "id", color: "lime" }), healed], { mode: "side" }).look("dust | healed"),
}
```

Specks scoring below 0.5 are soft, round features (stars, highlights) and are left alone. To keep one real
detail, drop it: `img.heal(dust.filter(r => r.id !== 31))`.

## Inspect small defects up close

A view that keeps only what differs from its surroundings, a contact sheet of the candidates, and a zoomed crop
with coordinate rulers (labels are absolute doc px):

```js
const speck = img.view("speck")
const spots = (await speck.mask.luminance({ above: 0.5 }).components({ minArea: 4 })).sort((a, b) => b.area - a.area).slice(0, 12)
return {
  sheet: cv.sheet(spots, { views: [img, speck], size: 32, zoom: 4 }).look("candidates"),
  zoom: img.crop([176, 22, 48, 48]).levels({ auto: true }).look({ zoom: 8, ruler: 8, label: "star 8×" }),
}
```

`cv.docs.read("views")` lists the presets; `cv.docs.read("looks")` the look options (zoom, ruler, grid, format).

## Measure spots, boxes and regions

`img.measure(items)` compares a disk at each item's centre with a ring around it, both sized from the item (step, spread, snr, stepChroma: null when |snr| < 1), and returns the items with these fields and their ids; points need `{ coreRadius }`. Sort the awaited array with JavaScript.

```js
const specks = (await img.detect.specks()).sort((a, b) => b.area - a.area).slice(0, 3)
const lin = img.as("linear")
const ranked = (await lin.measure(specks)).sort((a, b) => b.snr - a.snr)
const [star] = await lin.measure([[200.5, 46.5]], { coreRadius: 3 })
return {
  rows: ranked.map(r => [r.id, r.cx, r.cy, r.step.toFixed(3), r.snr.toFixed(2)].join(" ")),
  star: star.snr.toFixed(2),
  table: cv.csv(lin.measure(specks), { name: "ranked" }),
}
```

## Edit inside a mask

```js
const top = img.mask.gradient({ from: "top", to: 0.5 }).feather(10)
const darker = img.within(top, i => i.exposure(-0.6))
return { mask: top.look("mask"), result: darker.look("darker top"), meanTop: (await darker.stats({ mask: top })).meanL }
```

## Explore a value

Options are literal values: to try another exposure, run the eval again with another number, or compare
several in one look.

```js
const evs = [-0.5, 0, 0.5]
return cv.compare(evs.map(ev => img.exposure(ev)), { mode: "side" }).look("ev -0.5 / 0 / +0.5")
```
