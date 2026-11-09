# Masks and regions

A mask is a one-channel image of weights in [0, 1] (selections, feathered edges): every image op works on it, and it is read as stored. Regions are discrete areas (specks, blobs): awaited, they are a plain array of items you filter, sort and slice with JavaScript, and pass back wherever regions go. Each converts into the other.

## Making masks

A mask is an Image with one channel: `await m` reports `channels: 1`, and `m.stats()`, `m.crop(box)` and `m.look()` work as on any image. The mask makers, all on an Image: `mask.luminance({ above, below, softness })` (luminance levels in the image's space: for an opened file, the scale a look shows and `stats()` reports), `mask.color({ hue, tol, minSat })`, `mask.box([x, y, w, h])`, `mask.ellipse([x, y, w, h])`, `mask.gradient({ from: "top", to: 0.45 })` (or two points), `mask.flood([x, y], { tol })` (magic wand). Coordinates in doc px. `img.toGray()`, `chroma()` and `img.mask.regions(items)` are one-channel images too, so they are masks wherever one is expected.

```js
const bright = img.mask.luminance({ above: 0.6 })
const centre = img.mask.ellipse([64, 48, 128, 96])
return { bright: await bright.coverage(), centre: await centre.coverage() }
```

## Combining and refining

Set algebra: `a.and(b)`, `a.or(b)`, `a.minus(b)`, `a.not()`. Morphology in doc px: `m.dilate(r)` grows the selection by r, `m.erode(r)` shrinks it (a disk: pixels within r; see `cv.help("tophat")`), `feather(radius)` softens the edge; `refine(img)` snaps soft edges to image edges. `coverage()` is the selected fraction (the mask's mean); `img.stats({ mask })` measures the image inside the mask. These ops need a one-channel receiver, and an argument that takes a mask (`within`, `blend`, `stats`, `heal`, `inpaint`, `overlay`) takes any one-channel image: a colour image there is `E_WRONG_KIND` with the fix `img.toGray()`. A mask is a weight, never converted between spaces: `img.as("linear").within(m, …)` uses the same weights as `img.within(m, …)`.

```js
const sel = img.mask.luminance({ above: 0.45 })
  .and(img.mask.gradient({ from: "top", to: 0.6 }))
  .minus(img.mask.box([0, 0, 40, 40]))
  .feather(6)
return { coverage: await sel.coverage(), look: img.overlay(sel, { color: "lime" }).look("selection") }
```

## Regions are arrays

`detect.specks()`, `mask.components()` and `measure(items)` make Regions nodes. A Regions node has no methods: regions are arguments of image methods (`overlay`, `heal`, `measure`, `mask.regions`, `stats({ mask })`, `cv.sheet`). Awaited, it is a plain array of flat items `{ id, x, y, w, h, cx, cy, area, aspect, … }` in absolute doc px: `x, y, w, h` is the bounding box (corner and size), `cx, cy` the centre. `cv.help("detect.specks")` lists each op's fields. JavaScript does the querying:

```js
const sp = await img.detect.specks()
const keep = sp.filter(r => r.score >= 0.5).sort((a, b) => b.area - a.area).slice(0, 3)
const first = keep[0]
const { meanL, area } = await img.stats({ box: first })
return { n: sp.length, rows: keep.map(x => [x.id, x.cx, x.cy, x.area, x.score].join(" ")), around: { meanL, area } }
```

Pass the array (or the node) back wherever regions go. Each item stays linked to its region's exact pixels through `filter`, `sort`, `slice` and spread (`{ ...r, note: "x" }`); a `map` that builds new objects field by field drops the link (they become plain boxes; `ITEMS` warns). Editing `x, y, w, h` of a linked item is refused ("item 3 was moved": pass a plain box). Ids are data: edit or repeat them freely. A `label` string is printed instead of the id; a text `id` is `E_ARG_TYPE`.

```js looks=1
const sp = await img.detect.specks()
const top = sp.sort((a, b) => b.contrast - a.contrast).slice(0, 5).map((r, i) => ({ ...r, label: "c" + (i + 1) }))
return img.overlay(top, { labels: "id", color: "lime" }).look("top 5")
```

## Literal items

An array may also hold boxes `{ x, y, w, h }` or `[x, y, w, h]` and circles `{ cx, cy, r }`, next to the items of one region set (two sets: `img.overlay(a).overlay(b)`). They cover like `mask.box` and `mask.ellipse`. `measure`, `sample` and `cv.sheet` also take points `[x, y]` or `{ cx, cy }`. Literal items without an `id` are numbered 1, 2, … by position.

```js
const sp = (await img.detect.specks()).filter(r => r.score >= 0.5).slice(0, 4)
const gold = [{ cx: 200.5, cy: 46.5, r: 4, label: "star" }]
const m = await img.as("linear").measure([...sp, ...gold])
return m.map(x => [x.label ?? x.id, x.snr.toFixed(2)].join(" "))
```

## Converting

```js
const dust = (await img.detect.specks()).filter(r => r.score >= 0.5)
const m = img.mask.regions(dust).dilate(2)                                     // regions → mask
const blobs = await img.mask.luminance({ above: 0.7 }).components({ minArea: 4 })   // mask → regions
return { masked: await m.coverage(), blobs: blobs.length }
```

## Looking at selections

`img.overlay(itemsOrMask, { color, alpha, labels: "id" })` draws them over the image (colours: any CSS name such as `"lime"`, `"#ff8800"`, `"#f80"` or display `[r, g, b]` in [0, 1]). A mask's own look shows its weights as grey; a Regions node has no look: return `img.overlay(regions).look()`.

An overlay is drawn at the look's scale: in a zoomed crop its outlines stay 1 image px wide and the labels readable, and regions found on the whole image can be drawn on a crop, before or after the crop.

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

