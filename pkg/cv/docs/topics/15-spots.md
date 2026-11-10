# Spots and shapes

Spots as data and shapes by size: `peaks` gives the coordinates of the spots that stand out from their own surroundings; `erode` and `dilate` keep or remove structures by size and direction (openings, top-hats, line removal). Take coordinates from these rows, never from a picture.

## Peaks

`x.peaks({ radius, snr, polarity })` finds the local maxima of measure's core/ring step: the mean of a disk of radius `radius` minus the mean of the ring `[1.5 r, 2.5 r]` (doc px, default 3). Each peak is a disk `{ id, cx, cy, r, step, spread, snr, ringCoverage, … }` with a sub-pixel centre; the items are sorted by |snr| and `id` is the rank. The snr compares each spot with its own ring, so a faint speck on a quiet, dark background stands out, and it is exactly what `measure` reports at that centre and radius. `snr` (default 1) is the smallest |snr| kept: 1 favours recall (on grainy tiles hundreds of peaks), so filter or slice the sorted list. `polarity` is `"bright"` (default), `"dark"` or `"both"` (dark peaks have a negative step and snr).

```js
const pk = await img.as("linear").peaks({ radius: [2, 4], polarity: "both" })
return pk.slice(0, 8)   // a table: id, cx, cy, r, step, spread, snr, …
```

`radius: [2, 4, 8]` searches several sizes: each spot is found at its own size, and of two overlapping peaks the one with the higher |snr| is kept, so a large saturated speck is one peak at a large radius instead of a ring of small ones along its rim.

Peaks are computed on the luminance at analysis scale 1. On `img.crop(b)` only the crop is searched (faster on a large scan); near the crop's border rings are cut, which `ringCoverage` shows.

## The speck view's step of each peak

Search the image, measure the view. On a view that clips at zero, such as the speck view, most rings are flat, the spread falls to its floor and the snr means nothing; its `step` ranks well, also for a speck next to a bright edge (whose snr on the image drops). Peaks already have `step` and `snr`, so measuring them again needs a `prefix`.

```js
const lin = img.as("linear")
const pk = await lin.peaks({ radius: [2, 4, 8], polarity: "both" })
const rows = await lin.view("speck").measure(pk, { prefix: "sp" })
return rows.sort((a, b) => b.spStep - a.spStep).slice(0, 5).map(({ id, cx, cy, r, snr, spStep }) => ({ id, cx, cy, r, snr, spStep }))
```

The strongest spot near a point you were given: `(await lin.crop([x - 16, y - 16, 32, 32]).peaks({ snr: 0 }))[0]`.

## Look at them with their ids

A sheet's caption lists its cells, so every cell's id is data too.

```js looks=1
const pk = await img.as("linear").peaks({ snr: 2, polarity: "both" })
return cv.sheet(pk.slice(0, 15), { views: [img, img.view("speck")], size: 24, zoom: 4 }).look("peaks")
```

## Openings and top-hats

`x.erode(r)` is the minimum over a disk of radius r (doc px, at most 64), `x.dilate(r)` the maximum; both take one-channel images. An opening `x.erode(r).dilate(r)` removes bright things the disk does not fit in; a top-hat `x.sub(x.erode(r).dilate(r))` keeps them, filled, on a flat background. A saturated speck has a flat top that the speck view turns into a ring; its top-hat is a filled blob.

```js
const L = img.as("linear").toGray()
const th = L.sub(L.erode(6).dilate(6))   // bright things narrower than about 12 px
const above = 3 * (await th.stats()).p99
const blobs = await th.mask.luminance({ above }).components({ minArea: 4 })
return blobs.slice(0, 5).map(({ id, cx, cy, area }) => ({ id, cx, cy, area }))
```

Dark things: the black top-hat `x.dilate(r).erode(r).sub(x)`. `cv.help("opening")`, `cv.help("tophat")` give the compositions.

## Removing lines

`{ line: angle }` makes the element a line of radius r at that angle (degrees, counter-clockwise; any length). A line opening keeps whatever contains such a segment: threads, cables, scene edges. The maximum of the openings at eight angles keeps every line; subtracting it leaves the compact spots.

```js
const S = img.view("speck")
let lines = null
for (let a = 0; a < 180; a += 22.5) {
  const o = S.erode(12, { line: a }).dilate(12, { line: a })
  lines = lines ? lines.max(o) : o
}
const spots = S.sub(lines).mask.luminance({ above: 0.3 }).components({ minArea: 2 })
return (await spots).filter(r => r.aspect < 3).length
```
