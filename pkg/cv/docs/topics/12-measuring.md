# Measuring

Numbers at points and in regions: the value at a point (sample), the values along a line (profile), and how much each item stands out from its surroundings or what its area holds (measure). All work in the receiver's space at scale 1.

## Points and the value there

Coordinates are doc px and continuous: pixel (i, j) covers [i, i + 1), so its centre is `[i + 0.5, j + 0.5]`. A point is `[x, y]` or an object with `cx, cy`; items (a Regions node, region items, boxes, circles) are sampled at their centres. `img.sample(points, { radius })` gives one row per point: `{ cx, cy, meanL, meanR, meanG, meanB, area }`. Radius 0 reads the pixel that contains the point; a radius r > 0 averages a disk, weighting each pixel by the fraction of it inside (4 × 4 supersampling, the rule of `mask.ellipse`).

```js
const rows = await img.sample([[200.5, 46.5], [40.5, 150.5]], { radius: 2 })
return rows   // a table: one line per point
```

A point outside the frame gives a row with `outside: true` and null values, and an `OUTSIDE` warning.

## Profiles

`img.profile(a, b, { step, width })` samples the line from a to b every `step` doc px (bilinear), averaging `width` samples across the line: `{ from, to, step, width, rows: [{ t, cx, cy, meanL, meanR, meanG, meanB }] }`. Its look is a line chart (t on x, one line per channel, meanL in black). A soft rise (dust blurred by the scanner, edge σ ≈ 1.6 px) and a sharp one (a grain boundary) look different.

```js looks=1
const p = img.profile([185.5, 46.5], [215.5, 46.5], { width: 3 })
const v = (await p).rows.map(r => r.meanL)
return { peak: Math.max(...v).toFixed(3), plot: p.look("across the star") }
```

## Core and ring

`img.measure(items, opts)` returns the items (a Regions node: awaited, an array) with new fields. By default it compares a disk (the core) at each item's centre with the ring around it, on the gray plane (the luminance; `gray: "average"` for equal weights). The radii come from each item: the core radius is its `r` for a circle `{ cx, cy, r }`, else √(area / π) of a region or box (at least 1 doc px), and the ring is `[1.5 r, 2.5 r]`. `coreRadius` and `ringRadii` override them for every item; a point `[x, y]` or `{ cx, cy }` has no radius and needs `coreRadius`.

| field | meaning |
|---|---|
| `coreMean`, `ringMean` | coverage-weighted means of the core and the ring |
| `step` | coreMean − ringMean: > 0 when the spot is brighter |
| `spread` | p90 − p10 of the ring: how grainy the surroundings are |
| `snr` | step / spread: the step in units of local graininess |
| `stepR/G/B`, `stepChroma` | the step per channel (colour images); (max − min) / \|step\|: ≈ 0 for a neutral (grey) spot such as dust; `null` when \|snr\| < 1 |
| `ringCoverage` | the part of the ring inside the frame (below about 0.7 next to an edge) |

Example: core 0.61, ring 0.52 with p10 0.49 and p90 0.55 give step 0.09, spread 0.06, snr 1.5; channel steps 0.10, 0.09, 0.08 give stepChroma 0.22 (neutral).

`stepChroma` is `null` when \|snr\| < 1: the step is smaller than the ring's spread, so its colour cannot be attributed (dividing by a step near 0 gave values like 3.25 at snr 0.12). Test for `null` before formatting it.

```js
const [r] = await img.measure([[200.5, 46.5]], { coreRadius: 3.5 })
const [c] = await img.measure([{ cx: 200.5, cy: 46.5, r: 3.5 }])   // core 3.5, ring [5.25, 8.75]
return [r.step, r.spread, r.snr, r.stepChroma, c.snr].map(x => x.toFixed(3))
```

Gold points labelled with dustview carry the fit radius, about twice the speck's half-level radius: measure them as circles of half that radius, `gold.map(p => ({ cx: p.x - ox, cy: p.y - oy, r: p.r / 2 }))`.

The old helper `mem.probe(x, y, r)` is `img.measure([[x + 0.5, y + 0.5]], { coreRadius: r + 0.5, ringRadii: [r + 3.5, r + 9.5], gray: "average" })`.

## Measuring regions

Regions keep their ids, labels and order. `{ stats: ["mean", "max"] }` measures each item's own area instead of the ring (`meanL`, `maxL`; `channels: true` adds R, G, B); `{ stats: […], ring: true }` gives both; `prefix: "dog"` names the new fields `dogMaxL`, `dogStep` (a field the items already have is `E_FIELD_EXISTS`). The image may be a mask or any derived image of the same frame. Rank the awaited array with JavaScript.

```js
const dog = img.toGray("average").dog(1, 4, { polarity: "bright" })
const white = 2 * (await dog.stats()).p99     // a measurement used as an option is awaited first
const hot = dog.levels({ black: 0, white }).mask.luminance({ above: 0.99 }).components({ minArea: 4 })
const m = await img.measure(dog.measure(hot, { stats: ["max"], prefix: "dog" }))
const ranked = m.sort((a, b) => b.snr - a.snr).slice(0, 5)
return ranked.map(({ id, cx, cy, dogMaxL, snr, stepChroma }) => ({ id, cx, cy, dogMaxL, snr, stepChroma }))   // stepChroma: null when |snr| < 1
```

The unstretched DoG is measured (not `view("speck")`, which stretches per tile), so `dogMaxL` compares across tiles. On density tiles (opened with `encoding: "linear"`, or the `as("linear")` of an sRGB-encoded export) build the hot list on the density, as the speck view does: on 197 labelled points of 15 tiles the hot list has about 37 spots per tile, ranking by snr puts 60 % of the points within 4 px of one of the first 30 spots (43 % by area), and labelled dust has a median snr of 2.2 (other spots 0.8).

## Names and spaces

`x, y, w, h` is always a box (corner and size), `cx, cy` a point or centre, `meanL` the gray mean; radii are options only. Measurements are in the receiver's space (on `img`, the values a look shows; measure `img.as("linear")` for physical units: light, density) and read the image at scale 1 (`analysisScale`), so they do not depend on a look's scale. To hand all items to a program, save them: `await cv.save("items.csv", regions)` (see `cv.docs.read("data-out")`).
