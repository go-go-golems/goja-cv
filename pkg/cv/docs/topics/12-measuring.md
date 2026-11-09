# Measuring

Numbers at points and in regions: the value at a point (sample), the values along a line (profile), how much a spot stands out from its surroundings (measure), and the same for every region of a set (regions.measure). All work in linear space at scale 1 by default.

## Points and the value there

Coordinates are doc px and continuous: pixel (i, j) covers [i, i + 1), so its centre is `[i + 0.5, j + 0.5]`. A point is `[x, y]` or an object with `cx, cy`: a region item or a measurement row works as it is. `img.sample(points, { radius })` gives one row per point: `{ cx, cy, meanL, meanR, meanG, meanB, area }`. Radius 0 reads the pixel that contains the point; a radius r > 0 averages a disk, weighting each pixel by the fraction of it inside (4 × 4 supersampling, the rule of `mask.ellipse`).

```js
const rows = await img.sample([[200.5, 46.5], [40.5, 150.5]], { radius: 2 })
return rows.map(r => [r.cx, r.cy, r.meanL.toFixed(3)].join(" "))
```

A point outside the frame gives a row with `outside: true` and null values, and an `OUTSIDE` warning.

## Profiles

`img.profile(a, b, { step, width })` samples the line from a to b every `step` doc px (bilinear), averaging `width` samples across the line: `{ t, cx, cy, meanL, meanR, meanG, meanB }`. Its look is a line chart (t on x, one line per channel, meanL in black). A soft rise (dust blurred by the scanner, edge σ ≈ 1.6 px) and a sharp one (a grain boundary) look different.

```js looks=1
const p = img.profile([185.5, 46.5], [215.5, 46.5], { width: 3 })
const v = (await p).meanL
return { peak: Math.max(...v).toFixed(3), plot: p.look("across the star") }
```

## Core and ring

`img.measure(points, { coreRadius, ringRadii })` compares a disk (the core, default radius 3) with the ring around it (default `[coreRadius + 3, coreRadius + 9]`) on the gray plane (Rec. 709; `gray: "average"` for equal weights):

| field | meaning |
|---|---|
| `coreMean`, `ringMean` | coverage-weighted means of the core and the ring |
| `step` | coreMean − ringMean: > 0 when the spot is brighter |
| `spread` | p90 − p10 of the ring: how grainy the surroundings are |
| `snr` | step / spread: the step in units of local graininess |
| `stepR/G/B`, `stepChroma` | the step per channel; (max − min) / \|step\|: ≈ 0 for a neutral (grey) spot such as dust; `null` when \|snr\| < 1 |
| `ringCoverage` | the part of the ring inside the frame (below about 0.7 next to an edge) |

Example: core 0.61, ring 0.52 with p10 0.49 and p90 0.55 give step 0.09, spread 0.06, snr 1.5; channel steps 0.10, 0.09, 0.08 give stepChroma 0.22 (neutral).

`stepChroma` is `null` when \|snr\| < 1: the step is smaller than the ring's spread, so its colour cannot be attributed (dividing by a step near 0 gave values like 3.25 at snr 0.12). Test for `null` before formatting it; `sortBy("stepChroma")` puts null rows last and `where` comparisons on null are false.

```js
const [r] = await img.measure([[200.5, 46.5]], { coreRadius: 3.5 })
return [r.step, r.spread, r.snr, r.stepChroma].map(x => x.toFixed(3))
```

The old helper `mem.probe(x, y, r)` is `img.measure([[x + 0.5, y + 0.5]], { coreRadius: r + 0.5, ringRadii: [r + 3.5, r + 9.5], gray: "average" })`.

## Measuring regions

`regions.measure(image, opts)` keeps the regions (ids, order) and adds fields measured on an image or mask of the same frame: `{ stats: ["mean", "max"] }` over each region's own pixels (`meanL`, `maxL`; `channels: true` adds R, G, B), `{ ring: true }` the core/ring fields at each centroid (core radius from the region's area unless `coreRadius`), `prefix: "dog"` to name them `dogMaxL`, `dogStep`. Rank with `sortBy`. Points ops take points: `img.measure(regions)` is an error with this call as the fix.

```js
const dog = img.toGray("average").dog(1, 4, { polarity: "bright" })
const hot = dog.levels({ black: 0, white: dog.stats().p99.mul(2) }).mask.luminance({ above: 0.99 }).components({ minArea: 4 })
const ranked = hot.measure(dog, { stats: ["max"], prefix: "dog" }).measure(img, { ring: true }).sortBy("snr").top(5)
const chroma = x => x === null ? "null" : x.toFixed(2)   // null when |snr| < 1
return (await ranked).items.map(r => [r.id, r.cx, r.cy, r.dogMaxL, r.snr.toFixed(2), chroma(r.stepChroma)].join(" "))
```

The unstretched DoG is measured (not `view("speck")`, which stretches per tile), so `dogMaxL` compares across tiles. On density tiles (linear = density) pass `{ polarity: "bright", space: "linear" }` to `dog`, as the speck view does: on 142 labelled points of 15 tiles the hot list then has about 37 spots per tile within 4 px of 72 % of the points, and labelled dust has a median snr of 2.2 (other spots 0.9).

## Names and spaces

`x, y, w, h` is always a box (corner and size), `cx, cy` a point or centre, `meanL` the gray mean; radii are options only. Measurements are in the working space (linear by default: `{ space: "encoded" }` for the values a look shows) and read the image at scale 1 (`analysisScale`), so they do not depend on the preview. To hand all rows to a program, return `cv.csv(regions)` (see `cv.docs.read("data-out")`).
