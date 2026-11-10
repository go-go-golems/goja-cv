---
Title: Concepts tour
Slug: concepts
Short: Values and kinds, lazy nodes, literal options, doc px and scales, masks vs regions, lambdas, looks and errors
Topics:
- goja-cv
- concepts
Commands:
- goja-cv
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

This page is the language in one sitting, for the person who sets goja-cv up or reads an agent's transcript.
The agent reads the same ideas in shorter form from inside eval (`cv.docs.read("getting-started")` and friends).
All examples run against an open image `img`.

## Kinds of value

Every node has one kind, and every op says which kinds it takes and returns.

| kind | what it evaluates to | made by |
|---|---|---|
| Image | float32 pixels with an encoding (srgb or linear), a space, a channel count (1 or 3) and a frame (await gives a summary, never pixels); a mask is a one-channel image of weights in [0, 1] | `img`, `exposure`, `levels`, `within`, `heal`, `overlay`; masks: `img.mask.luminance`, `img.mask.box`, `img.mask.regions(items)` |
| Regions | discrete areas with flat fields id, x, y, w, h (box), cx, cy (centre), area, aspect and measures; no methods; awaited, an array of items | `img.detect.specks()`, `mask.components()`, `img.measure(items)` |
| Value | plain data: numbers, objects, arrays | `stats`, `histogram`, `coverage`, `profile` |

## Lazy, immutable, shared nodes

`img.exposure(0.3)` computes nothing: it returns a node. The same call returns the same node, so shared work
is cached by node and scale. `await` is the only evaluation: a measurement (a Value) is computed at full
resolution, an Image gives its summary without computing anything. Looks render after the eval.

```js
const a = img.exposure(0.3)
const b = a.gaussian(2)
return { same: a === img.exposure(0.3), graph: cv.explain(b).graph, summary: await b }
```

Awaited regions are a plain array: `const r = await img.detect.specks()`, then `r.filter(...)`, `r.sort(...)`,
`r.slice(...)` are JavaScript, and the array goes back wherever regions go. A node that is built and
thrown away produces an `UNUSED_NODE` warning, and reading a field of an unawaited measurement
(`img.stats().p99`) or comparing a node with `<` is `E_NOT_AWAITED` — await it first.

Options are literal values. A measurement that feeds an option is awaited first:
`img.levels({ black: s.p01, white: s.p99 })` with `const s = await img.stats()`. A node passed where an option is
expected is `E_ARG_TYPE` with that fix.

## Doc px and scales

Lengths, boxes and points are in **doc px** (pixels of the original image) at every scale; areas in doc px².
`await` measures at full resolution; looks render at a look scale (at most 1568 px) or, with `look({ zoom })`,
magnify a crop above full size (filters are computed at full size, then upsampled); the engine rescales radii and
boxes so a look and a measurement agree. Region detection runs at a fixed
`analysisScale`, so region ids and boxes are the same everywhere. Coordinates are absolute in the image's
**frame** (a window read with `cv.open(path, { box })` keeps the scan's coordinates).

## Encodings and spaces

Every image has an **encoding**: `srgb` (photographs: the sRGB curve undone gives light) or `linear` (float
TIFF, DNG raw scans, `.npy`, density maps: the stored value is the quantity). `cv.open(path, { encoding })`
overrides what the file implies. Every image also has a **space**: images open in `encoded` space (file units,
what looks show), and `img.as("linear")` gives light (or density). An op works in its receiver's space and returns
it, so a level measured on an image fits ops on that image; physical ops (`exposure`, `heal`, `inpaint`, `toLab`,
`encode`, `toGray`) compute in linear light and return the receiver's space, and pictures (`overlay`, `compare`,
`sheet`) are encoded. The graph inserts cached conversion nodes (`cv.explain` shows them), luminance is always
computed from linear light, and a look shows encoded values; masks and derived images
(`chroma`, `toLab`) show their values directly. Several images can be open under names:
`cv.open(path, { name: "ref" })`.

## Masks and regions

A **mask** is a soft field (a feathered sky): a one-channel image of weights, read as stored (never converted
between spaces), on which every image op works; **regions** are things you count, rank and exclude
(143 specks, drop id 31). Awaited regions are an array of **items**; each stays linked to its region's exact pixels
through `filter`, `sort`, `slice` and `{ ...r }`, and literal boxes `{ x, y, w, h }` and circles `{ cx, cy, r }`
mix in. Ids are data (they may repeat); a `label` is printed instead of the id. Each converts into the other:
`img.mask.regions(items)`, `mask.components()`.

```js
const sel = img.mask.luminance({ above: 0.45 }).minus(img.mask.box([0, 0, 40, 40])).feather(6)
const specks = (await img.detect.specks()).filter(r => r.score >= 0.5).sort((a, b) => b.area - a.area).slice(0, 3)
return {
  coverage: await sel.coverage(),
  inside: (({ meanL, area }) => ({ meanL, area }))(await img.stats({ mask: sel })),
  biggest: specks.map(r => ({ id: r.id, x: r.x, y: r.y, w: r.w, h: r.h })),
}
```

## Lambdas

The one function cv takes is a **builder** lambda: it runs once while the graph is built and returns nodes:
`img.within(mask, i => i.exposure(-0.5))`. Data is selected with JavaScript on awaited arrays
(`(await regions).filter(r => r.aspect > 2)`). There are no per-pixel JavaScript functions.

## Looks

A look is an image the agent receives: `node.look("label")`. Every look created in an eval is delivered, whether
it is returned or not, also when the eval throws later; at most four per eval (a `LOOKS` warning names the others
by line). Looks are numbered (the number is burned into the corner) and the caption gives
what the image is and its scale, `look 2: after — open(scan.tif) → levels (1568×1045, 1 image px = 4.0 doc px)`,
so coordinates read off a look convert to doc px. To judge small things, crop and zoom with coordinate rulers,
`img.crop([x, y, 64, 64]).look({ zoom: 8, ruler: 8 })`, use a view that makes them stand out
(`img.view("speck")`), and put many candidates in one contact sheet (`cv.sheet(regions, { views })`). At most
four images arrive per eval.

## Errors and warnings

Every error has the same shape: `code`, `message`, `where` (the agent's line), `path` (the graph path),
`hint` and usually a literal `fix`. Raw JavaScript errors that involve cv objects are translated into the same
shape. Warnings (`RANGE`, `UNUSED_NODE`) travel with results instead of failing them. `cv.help(name)` never
throws: unknown names return `{ found: false, didYouMean }`.

```js error=E_NOT_AWAITED
const specks = img.detect.specks()
return specks.heal()   // fix: img.heal(regions, …)
```
