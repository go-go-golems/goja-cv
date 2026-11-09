---
Title: goja-cv recipes
Slug: recipes
Short: Check and fix exposure, find and heal dust, inspect defects up close, measure, edit inside a mask, task libraries, thread widths, the playbook pattern
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
  rows: ranked.map(({ id, cx, cy, step, snr }) => ({ id, cx, cy, step, snr })),   // a table
  star: star.snr.toFixed(2),
  table: (await cv.save("ranked.csv", lin.measure(specks))).rows,
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

## Write a task library

A task library is a file of helpers for one job, loaded instead of pasted: `await cv.load("dust/helpers.js")` in an
eval, or `goja-cv mcp --read DIR --prelude DIR/dust/helpers.js` for every session. The file is an eval body (top-level
`await` and `return` work, `const` stays private), so this block also runs as an eval. Start the file with a comment
line (`cv.scripts()` shows it), keep what must survive on `mem`, and open each function's body with a comment: it is
the function's doc line in the context block.

```js
// dust/helpers.js: helpers for annotator agents (a model task library)
mem.lin = img.as("linear")

mem.probe = async (x, y, r = 3) => { // core/ring measure at (x, y): { cx, cy, step, spread, snr }
  const [m] = await mem.lin.measure([{ cx: x, cy: y, r }])
  return m
}

mem.sheet = (items, size = 40) => // contact sheet of up to 15 items: image | speck view, captioned with their ids
  cv.sheet(items.slice(0, 15), { views: [img, img.view("speck")], size, zoom: 4, stretch: "cell" })

return "dust helpers ready"
```

## Measure the width of a thread

`profile` samples across a line; a pipeline script turns the profile into the full width at half height above the
baseline of its two ends. Pick the two points either side of the thread from data, and run the script at two or
three places along it:

```js fragment
// dust/thread-width.js: the width of a bright thread across the segment a → b (doc px), full width at half height
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
  if (j < 0 || j >= n) return null // the segment is too short
  return t[i] + (t[j] - t[i]) * (v[i] - half) / (v[i] - v[j])
}
const lo = cross(-1), hi = cross(1)
return { width: lo === null || hi === null ? null : hi - lo, height: v[k] - base, at: [p.rows[k].cx, p.rows[k].cy] }
```

```js
const w = await cv.load("dust/thread-width.js", { a: [192, 46.5], b: [209, 46.5] })
return { width: w.width > 3 && w.width < 8, at: w.at }
```

## The playbook pattern

For people who write playbooks for annotator agents:

1. Register the server with the task library as a prelude, so agents never paste it.
2. Name the library's functions and what they return in the playbook; agents use what the playbook names.
3. Give agents coordinates as data: candidates as rows from a library function, sheets of those items (their
   captions list the ids), and the instruction "never read a coordinate off a ruler; take it from the rows".
4. For threads: pick two points either side of the thread from a zoom's data and run
   `await cv.load("dust/thread-width.js", { a, b })` at two or three places along it.
5. Return rows, not strings: the result renders them as a table.
