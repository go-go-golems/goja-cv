# Looks and images

A look is an image of a node: `node.look(options)`. Every look created in an eval is delivered after it, whether or not you return it; at most four arrive per eval. It can fit the whole image, magnify a crop with zoom, carry coordinate rulers, and its caption says what the image is (the op chain) and its scale.

## Looks

`node.look("label")` or `node.look({ label, zoom, interp, maxSide, ruler, grid, format })` creates a look and queues it. Put it in the returned value to see its placeholder (the caption) in the YAML; the image arrives next to the text either way.

```js looks=2
const s = await img.stats()
const fixed = img.levels({ black: s.p01, white: s.p99 })
return { meanL: (await fixed.stats()).meanL, before: img.look("before"), after: fixed.look({ label: "after", maxSide: 800 }) }
```

The YAML shows `before: '[look 1: before — open(planted-256x192-16bit.tif) (256×192, 1 image px = 1.0 doc px)]'`.

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

A look shows at most 1568 px on its long side. To judge a speck, crop a window and zoom it: `img.crop([x, y, w, h])` keeps absolute coordinates (doc px of the file), and `look({ zoom: 8 })` magnifies it with nearest-neighbour pixels (a 64 px window at zoom 8 is 512 × 512).

```js looks=1
return img.crop([150, 20, 64, 64]).look({ zoom: 8, ruler: 8, label: "star 8×" })
```

Filters are computed at full resolution, then magnified. A look over 1568 px is an error whose fix gives a smaller crop or a lower zoom. Crop **last**: a blur after a crop sees only the window (`CROP_FIRST`). Stretch a crop by its own pixels: `img.crop(box).levels({ auto: true })`.

## Rulers and grid

`ruler: 8` draws ticks every 8 doc px in margins, labelled with absolute coordinates (a crop at (150, 20) reads 150, 158, … and 20, 28, …; the tick at 158 is the left edge of doc pixel 158). `ruler: true` picks a spacing so ticks are at least 40 image px apart; `grid: 8` draws faint lines every 8 doc px. Pass coordinates read off the rulers to `crop`, `mask.box`, `stats({ box })` or `annotate` unchanged.

## Captions

```
look 1: speck 8× — open(planted-256x192-16bit.tif) → view(speck) → crop[176,22,48,48] (384×384 + ruler, 1 image px = 0.125 doc px, doc x 176–224, y 22–70)
```

The number (also burned into the corner), your label, the **op chain** (views by name; `cv.explain(node).graph` shows their expansion), the size (`+ ruler` with margins), the scale, and for crops, zooms and rulers the doc-px range. On a fitted look, multiply coordinates by the doc-px factor.

## Delivery

Looks in the returned value come first, then the others in creation order; a look created before an error is still delivered; equal looks (same node, same options) are one. At most four are delivered: the others are not rendered and a `LOOKS` warning names each by line (return one `cv.sheet` of the crops, or look in two evals).

```js looks=1
img.look("before")
const s = await img.stats()
return { meanL: s.meanL }
```

```js looks=4 warns=LOOKS
for (const x of [0, 40, 80, 120, 160, 200]) img.crop([x, 20, 32, 32]).look({ zoom: 4, label: "x " + x })
return "four delivered; LOOKS names two"
```

## Formats

`format: "auto"` sends PNG for gray images, masks, zoomed looks, sheets and looks up to 512 × 512 px, JPEG (q85) for larger colour looks. JPEG ringing is the size of small specks: use `format: "png"` for fine detail.

## Contact sheets

`cv.sheet(items, { views, size, zoom })` is one image of many magnified crops: a cell per item (regions, boxes, circles, points), labelled with its label (else id) and centre in doc px; several views show each item side by side. At the defaults (64 px windows at zoom 4) 36 cells fit one look (18 items with two views). A sheet is a picture: use the labels' coordinates, not its pixels.

```js looks=1
const spots = img.view("speck").mask.luminance({ above: 0.5 }).components({ minArea: 4 })
return cv.sheet((await spots).sort((a, b) => b.area - a.area).slice(0, 6), { views: [img, img.view("speck")], size: 32, zoom: 4 }).look("candidates")
```

More items than cells are cut (`SHEET_TRUNCATED`): sort and slice first. `stretch: "cell"` stretches each cell alone.

## What a look shows

A look shows **encoded** values: an sRGB photo looks as usual, a linear-encoded image (`cv.open(path, { encoding: "linear" })`, `.npy`, DNG) shows its stored values directly, a mask shows its weight (0.5 is mid-grey), and a derived image (`chroma`, `toLab`, `sub`, `highpass`) shows its value as grey level. Signed images need `.mul(k).add(0.5)` or a diverging colormap (see `cv.docs.read("views")`). `img.encode("srgb")` makes a linear photograph look normal. See `cv.docs.read("encodings-and-spaces")`.

## What can be looked at

- Image nodes, masks included (a mask shows its weights as grey); one-channel maps in false colour with `colormap("viridis")`;
- `histogram()` Value nodes render as a bar chart (no zoom or rulers);
- Regions: `img.overlay(regions, { labels: "id" })`; marks: `img.annotate([...])`;
- images side by side: `cv.compare([a, b], { mode: "side" | "split" })`; a difference: `a.sub(b)`.

```js looks=2
const fixed = img.exposure(0.4)
cv.compare([img, fixed], { mode: "side" }).look("before / after")
fixed.histogram().look()
return (await fixed.stats()).clipHigh
```

Other Value nodes have no look (`E_WRONG_KIND`): await them instead.
