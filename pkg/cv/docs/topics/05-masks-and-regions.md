# Masks and regions

A mask is a one-channel image of weights in [0, 1] (selections, feathered edges): every image op works on it, and it is read as stored. Regions are discrete, identified areas you can count, rank and exclude by id (specks). Each converts into the other.

## Making masks

A mask is an Image with one channel: `await m` reports `channels: 1`, and `m.stats()`, `m.crop(box)` and `m.look()` work as on any image. The mask makers, all on an Image: `mask.luminance({ above, below, softness })` (luminance levels in the image's space: for an opened file, the scale a look shows and `stats()` reports), `mask.color({ hue, tol, minSat })`, `mask.box([x, y, w, h])`, `mask.ellipse([x, y, w, h])`, `mask.gradient({ from: "top", to: 0.45 })` (or two points), `mask.flood([x, y], { tol })` (magic wand). Coordinates in doc px. `img.toGray()`, `chroma()` and `regions.toMask()` are one-channel images too, so they are masks wherever one is expected.

```js
const bright = img.mask.luminance({ above: 0.6 })
const centre = img.mask.ellipse([64, 48, 128, 96])
return { bright: await bright.coverage(), centre: await centre.coverage() }
```

## Combining and refining

Set algebra: `a.and(b)`, `a.or(b)`, `a.minus(b)`, `a.not()`. Morphology in doc px: `m.dilate(r)` grows the selection by r, `m.erode(r)` shrinks it (both take a disk: the pixels within r; see below), `feather(radius)` softens the edge; `refine(img)` snaps soft edges to image edges. `coverage()` is the selected fraction (the mask's mean); `img.stats({ mask })` measures the image inside the mask. These ops need a one-channel receiver, and an argument that takes a mask (`within`, `blend`, `stats`, `heal`, `inpaint`, `overlay`) takes any one-channel image: a colour image there is `E_WRONG_KIND` with the fix `img.toGray()`. A mask is a weight, never converted between spaces: `img.as("linear").within(m, …)` uses the same weights as `img.within(m, …)`.

```js
const sel = img.mask.luminance({ above: 0.45 })
  .and(img.mask.gradient({ from: "top", to: 0.6 }))
  .minus(img.mask.box([0, 0, 40, 40]))
  .feather(6)
return { coverage: await sel.coverage(), look: img.overlay(sel, { color: "lime" }).look("selection") }
```

## Erosion and dilation

`x.erode(r)` and `x.dilate(r)` work on any one-channel image (a mask, a grey image, a view): the minimum or maximum over a disk of radius r doc px (the offsets at distance ≤ r; r ≤ 64), or with `{ line: angle }` over a line of radius r at that angle (degrees; any length). Opening is `x.erode(r).dilate(r)`, closing `x.dilate(r).erode(r)`, a white top-hat `x.sub(x.erode(r).dilate(r))`: bright structures smaller than the disk at full height, a saturated speck filled (`cv.help("tophat")`). `grow(r)` reached r + 0.5: `m.dilate(r + 0.5)` gives its set.

```js
const L = img.as("linear").toGray()
const top = L.sub(L.erode(12).dilate(12))
return { top: (await top.stats()).p99, grown: await img.mask.luminance({ above: 0.6 }).dilate(3).coverage() }
```

## Regions

`detect.specks()` and `mask.components()` make Regions. Awaiting gives `{ count, items: [{ id, x, y, w, h, cx, cy, area, aspect, … }], frame, … }`: flat fields, absolute doc px in the image's frame. `x, y, w, h` is the bounding box (corner and size), `cx, cy` the centroid; the same names work in `where` and `sortBy`, and an item can be passed wherever a box or a point is expected (`img.crop(item)`, `img.mask.flood(item)`, `cv.sheet(items)`, `only(item)`). `cv.help("detect.specks")` lists each op's fields. Narrow them with:

- `where({ field: { lt, lte, gt, gte, eq, ne, in } })` or `{ field: value }` (data, runs in Go);
- `without([ids])`, `only([ids])` (ids or items), `sortBy("contrast", { desc })`, `top(n)`, `count()`;
- `filter(r => …)` for tests that cannot be written as data (see `cv.docs.read("lambdas")`).

```js
const specks = img.detect.specks()
const keep = specks.where({ score: { gte: 0.5 } }).sortBy("area").top(3)
const r = await keep
const first = r.items[0]
const { meanL, area } = await img.stats({ box: first })
return { rows: r.items.map(x => [x.id, x.cx, x.cy, x.area, x.score].join(" ")), around: { meanL, area } }
```

Region ids are stable for the same image and options (detection runs at `analysisScale`), so you can drop one by id in a later eval: `specks.without([31])`.

## Converting

```js
const specks = img.detect.specks().where({ score: { gte: 0.5 } })
const m = specks.toMask().dilate(2)                                    // Regions → mask, grown by 2 px
const blobs = img.mask.luminance({ above: 0.7 }).components({ minArea: 4 })   // mask → Regions
return { masked: await m.coverage(), blobs: await blobs.count() }
```

## Looking at selections

`img.overlay(regions | mask, { color, alpha, labels: "id" })` draws them over the image (colours: any CSS name such as `"lime"`, `"#ff8800"`, `"#f80"` or display `[r, g, b]` in [0, 1]). A mask's own look shows its weights as grey; a Regions node has no look: return `img.overlay(regions).look()`.

An overlay is drawn at the look's scale: in a zoomed crop its outlines stay 1 image px wide and the ids readable, and regions found on the whole image can be drawn on a crop, before or after the crop.

```js looks=1
const specks = img.detect.specks()
return img.overlay(specks, { labels: "id", color: "lime" }).crop([150, 20, 64, 64]).look({ zoom: 8, ruler: 8 })
```

## Neutral or coloured?

Dust on colour film is neutral: it raises all three channels alike, while colour grain and scene detail do not. `chroma()` is the per-pixel channel spread (0 for grey); a mask of low chroma keeps neutral areas.

```js
const c = img.chroma({ relative: true })
const neutral = c.mask.luminance({ below: 0.1 })
return { neutralShare: await neutral.coverage(), look: c.colormap("magma").look("chroma") }
```

