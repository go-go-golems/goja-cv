# Masks and regions

A Mask is a soft field of weights in [0, 1] (selections, feathered edges); Regions are discrete, identified areas you can count, rank and exclude by id (specks). Each converts into the other.

## Making masks

All on an Image: `mask.luminance({ above, below, softness })` (luminance levels in the working space: encoded by default, the scale a look shows), `mask.color({ hue, tol, minSat })`, `mask.box([x, y, w, h])`, `mask.ellipse([x, y, w, h])`, `mask.gradient({ from: "top", to: 0.45 })` (or two points), `mask.flood([x, y], { tol })` (magic wand). Coordinates in doc px.

```js
const bright = img.mask.luminance({ above: 0.6 })
const centre = img.mask.ellipse([64, 48, 128, 96])
return { bright: await bright.coverage(), centre: await centre.coverage() }
```

## Combining and refining

Set algebra: `a.and(b)`, `a.or(b)`, `a.minus(b)`, `a.not()`. Morphology in doc px: `grow(radius)`, `shrink(radius)`, `feather(radius)`; `refine(img)` snaps soft edges to image edges. `coverage()` is the selected fraction; `img.stats({ mask })` measures the image inside the mask.

```js
const sel = img.mask.luminance({ above: 0.45 })
  .and(img.mask.gradient({ from: "top", to: 0.6 }))
  .minus(img.mask.box([0, 0, 40, 40]))
  .feather(6)
return { coverage: await sel.coverage(), look: sel.overlay(img, { color: "lime" }).look("selection") }
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
const m = specks.toMask({ grow: 2 })                                   // Regions → Mask
const blobs = img.mask.luminance({ above: 0.7 }).components({ minArea: 4 })   // Mask → Regions
return { masked: await m.coverage(), blobs: await blobs.count() }
```

## Looking at selections

`img.overlay(regions | mask, { color, alpha, labels: "id" })` draws them over the image (colours: any CSS name such as `"lime"`, `"#ff8800"`, `"#f80"` or display `[r, g, b]` in [0, 1]) (`regions.overlay(img)` and `mask.overlay(img)` work too). A Regions node itself has no look: return `img.overlay(regions).look()`.

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

