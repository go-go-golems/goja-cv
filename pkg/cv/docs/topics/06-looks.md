# Looks and images

A look is an image you return: node.look(options) in the result, or show(node). It can fit the whole image, magnify a crop with zoom, carry coordinate rulers, and its caption says what the image is (the op chain) and its scale. Looks are rendered after the eval; at most four arrive per eval.

## Returning looks

`node.look("label")` or `node.look({ label, zoom, interp, maxSide, ruler, grid, format })` returns a look object. Put it anywhere in the returned value; the YAML shows a placeholder with the caption and the image arrives next to the text.

```js looks=2
const s = img.stats()
const fixed = img.levels({ black: s.p01, white: s.p99 })
return { meanL: await fixed.stats().meanL, before: img.look("before"), after: fixed.look({ label: "after", maxSide: 800 }) }
```

```yaml
meanL: 0.39417
before: '[look 1: before — open(planted-256x192-16bit.tif) (256×192, 1 image px = 1.0 doc px)]'
after: '[look 2: after — open(planted-256x192-16bit.tif) → levels (256×192, 1 image px = 1.0 doc px)]'
```

| option | meaning |
|---|---|
| `label` | caption text (default: none; the caption always has the op chain) |
| `zoom` | render scale in [1, 16]: each doc px becomes zoom × zoom image px; exclusive with `maxSide` |
| `interp` | `"nearest"` (default: pixel edges stay visible, values exact) or `"linear"` when zoomed |
| `maxSide` | long side in image px when fitting (16 … 1568, default 1568) |
| `ruler` | coordinate rulers in margins: tick spacing in doc px, or `true` to pick one; labels are absolute doc px |
| `grid` | faint lines every `grid` doc px over the image |
| `format` | `"auto"` (default), `"png"` or `"jpeg"` |

## Zoom into a window

A look shows at most 1568 px on its long side, so a whole tile is shown shrunk. To judge a speck, crop a window and zoom it: `img.crop([x, y, w, h])` is a node whose coordinates stay absolute (doc px of the file), and `look({ zoom: 8 })` magnifies it with nearest-neighbour pixels. A 64 px window at zoom 8 is a 512 × 512 look.

```js looks=1
return img.crop([150, 20, 64, 64]).look({ zoom: 8, ruler: 8, label: "star 8×" })
```

Filters are computed at full resolution, then magnified. A look over 1568 px is an error whose fix gives a smaller crop or a lower zoom. Crop **last**: a blur after a crop sees only the window (`CROP_FIRST`). Stretch a crop by its own pixels: `img.crop(box).levels({ auto: true })`.

## Rulers and grid

`ruler: 8` draws ticks every 8 doc px in margins around the image, labelled with absolute coordinates: on a crop at (150, 20) the labels read 150, 158, … and 20, 28, … The tick at x = 158 is at the left edge of doc pixel 158. `ruler: true` picks a spacing (1, 2, 4, 8, …) so ticks are at least 40 image px apart. `grid: 8` draws faint lines across the image every 8 doc px. Read coordinates off the rulers and pass them to `crop`, `mask.box`, `stats({ box })` or `annotate` unchanged.

## Captions

```
look 1: speck 8× — open(planted-256x192-16bit.tif) → view(speck) → crop[176,22,48,48] (384×384 + ruler, 1 image px = 0.125 doc px, doc x 176–224, y 22–70)
```

The number (also burned into the corner), your label, the **op chain** from the source (views appear by name; `explain()` shows their expansion), the size (`+ ruler` with margins), the scale, and for crops, zooms and rulers the doc-px range shown. On a fitted look, multiply coordinates by the doc-px factor.

## show()

`show(node, opts)` creates the same look, queues it and returns it. Queued looks are listed under `shown:` and delivered even if the eval throws afterwards.

```js looks=1
show(img, "before")
const s = await img.stats()
return { meanL: s.meanL }
```

## Formats

`format: "auto"` sends PNG for gray images, masks, zoomed looks, sheets and looks up to 512 × 512 px, JPEG (q85) for larger colour looks (four whole tiles fit the size budget). JPEG ringing is the size of small specks: use `format: "png"` to judge fine detail in a large colour look.

## Contact sheets

`cv.sheet(items, { views, size, zoom })` is one image of many magnified crops: a cell per region (centred on its centroid), point or box, labelled with the id and the centre in doc px. Give several views to see each item side by side in each. At the defaults (64 px windows at zoom 4) 36 cells fit one look; with two views, 18 items. A sheet is a picture: its pixels are not doc px, so use the labels' coordinates.

```js looks=1
const spots = img.view("speck").mask.luminance({ above: 0.5 }).components({ minArea: 4 })
return cv.sheet(spots.sortBy("area").top(6), { views: [img, img.view("speck")], size: 32, zoom: 4 }).look("candidates")
```

More items than cells are cut (`SHEET_TRUNCATED`): sort and `top(n)` first. `stretch: "cell"` stretches each cell alone.

## What a look shows

A look shows **encoded** values: an sRGB photo looks as usual, a linear-encoded image (`cv.open(path, { encoding: "linear" })`, `.npy`, DNG) shows its stored values directly, a mask shows its weight (0.5 is mid-grey), and a derived image (`chroma`, `toLab`, `sub`, `highpass`) shows its value as grey level. Signed images need `.mul(k).add(0.5)` or a diverging colormap (see `cv.docs.read("views")`). `img.encode("srgb")` makes a linear photograph look normal. See `cv.docs.read("encodings-and-spaces")`.

## What can be looked at

- Image and Mask nodes; one-channel maps in false colour with `colormap("viridis")`;
- `histogram()` Value nodes render as a bar chart (no zoom or rulers);
- Regions: draw them with `img.overlay(regions, { labels: "id" })`, marks with `img.annotate([...])`;
- several images side by side: `cv.compare([a, b], { mode: "side" | "split" })`; their difference: `a.sub(b)`.

```js looks=2
const fixed = img.exposure(0.4)
show(cv.compare([img, fixed], { mode: "side" }), { label: "before / after" })
show(fixed.histogram())
return await fixed.stats().pick("meanL", "clipHigh")
```

Other Value nodes have no look (`E_WRONG_KIND`): await them instead.
