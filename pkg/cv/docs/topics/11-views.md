# Views

A view transforms the image so that what you look for stands out: high-pass and difference-of-Gaussians views show what differs from its surroundings, local contrast evens out shadows and highlights, and img.view(name) gives named presets with their stretch.

## Why a view

Small defects are hard to see in a stretched photograph: a 3 px speck sits among thousands of grain bumps and scene edges. In a view that keeps only what is smaller than its surroundings, a speck is among the first things the eye meets. On 142 hand-labelled specks of 19 real tiles, a labelled speck was the 715th most visible bump of the stretched tile (median), but the 18th of the `speck` view: about 40 times earlier; 78 % of the specks were among the 50 most visible bumps of the view.

## High-pass and difference of Gaussians

- `img.highpass(sigma)` is `img − img.gaussian(sigma)`: it shows only what differs from its surroundings at size σ (doc px). Positive where the image is brighter than its neighbourhood.
- `img.dog(sigma1, sigma2)` (sigma1 < sigma2) is `gaussian(sigma1) − gaussian(sigma2)`: the first blur also removes the finest grain, so small specks rank higher. `dog(1, 4)` suits specks of 1–3 px.
- `{ polarity: "bright" }` keeps only the positive part, `"dark"` only the negative part (as positive values), `"both"` (the default) keeps the sign.

Both work in their receiver's space: on `img`, the values a look shows; on `img.as("linear")`, light. Their result is a signed, derived image: its stored value is its value.

```js
const hp = img.highpass(3)
const s = await hp.stats()
return { p01: s.p01, p99: s.p99, look: hp.mul(4).add(0.5).look("high-pass ×4, zero = grey") }
```

## Showing signed images

A look clamps to [0, 1], so a signed image needs a mapping: add 0.5 to make zero mid-grey (scale it first), or reduce it to one channel and use the diverging colormap (blue = darker, white = zero, red = brighter).

```js
const g = img.toGray("average")
const d = g.sub(g.gaussian(3))
return {
  grey: d.mul(4).add(0.5).look("signed, grey"),
  colour: d.colormap("diverging").look("signed, diverging"),
}
```

## Local contrast

`img.localContrast(sigma)` divides the high-pass by the local spread: 3 means "three local standard deviations above the local mean", in a dark corner and in a bright one alike. Use it to sweep a whole tile; values are about −4 … 4.

## Stretch

`levels({ auto: true })` maps the image's own 1st and 99th percentiles to black and white (`auto: [0.005, 0.995]` for other percentiles). After a crop it stretches by the crop's pixels: `img.crop(box).levels({ auto: true })` is the per-window stretch. A fixed stretch is `levels({ black, white })`, and `white` may come from a measurement awaited first: `d.levels({ black: 0, white: 5 * (await d.stats()).p99 })`.

```js
const box = [150, 20, 64, 64]
return { plain: img.crop(box).levels({ auto: true }).look({ zoom: 4, label: "stretched crop" }) }
```

## Presets

`img.view(name, { size })` builds a named view; `size` (doc px, default 3) is the typical size of what you look for, and the presets derive their σ from it. The caption says `view(speck)`; `cv.explain` shows the expansion. `speck` and `speckColor` compute in linear light (`img.as("linear")`; on density tiles a labelled speck ranks higher there: 18th against 20th), the others in the image's space.

| name | what it shows | use it for |
|---|---|---|
| `scene` | the image stretched 1–99 % | judging what belongs to the scene |
| `speck` | small bright things that differ from their surroundings: grey `dog(size/3, 4·size/3, { polarity: "bright" })`, white = 5 × p99 | dust on density tiles, stars, sensor defects |
| `speckDark` | small dark things, shown bright | dust on positives and prints |
| `speckColor` | the speck view on all channels: neutral specks stay white | neutral defects versus colour grain |
| `signed` | brighter (red) and darker (blue) than the surroundings at size s | edges, scratches, both polarities |
| `detail` | local contrast (σ 4·size), diverging ±4 | sweeping a whole tile: detail equally visible in shadows and highlights |
| `neutral` | where the channels disagree (relative chroma, viridis 0–1) | grey dust versus coloured detail |

```js looks=2
return { speck: img.view("speck").look("speck view"), detail: img.view("detail").look() }
```

The speck view's white point is 5 × the 99th percentile of its high-pass over the image it is computed on. On a tile with strong scene edges (windows, cables, lattices) the edges set it, and dust can map to 3–10 % of white. `img.view("speck").crop(b)` keeps the whole image's scale; `img.crop(b).view("speck")` sets it from the crop alone (a local stretch). To find faint dust, do not lower thresholds on the view: use `img.as("linear").peaks({ snr })`, whose snr compares each spot with its own surroundings. A large saturated speck has a flat top, which the view turns into a ring: use a top-hat (`L.sub(L.erode(r).dilate(r))`) or peaks with several radii (`cv.docs.read("spots")`).

A view is a node like any other: crop it, zoom it, find components in it, put it in a sheet.

```js looks=1
const box = [176, 22, 48, 48]
return img.view("speck").crop(box).look({ zoom: 8, ruler: 8, label: "speck 8×" })
```
