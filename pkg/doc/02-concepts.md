---
Title: Concepts tour
Slug: concepts
Short: Values and kinds, lazy nodes, doc px and scales, masks vs regions, lambdas, looks, params and errors
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

## Five kinds of value

Every node has one kind, and every op says which kinds it takes and returns.

| kind | what it evaluates to | made by |
|---|---|---|
| Image | float32 pixels with an encoding (srgb or linear) and a frame (await gives a summary, never pixels) | `img`, `exposure`, `levels`, `within`, `heal`, `overlay` |
| Mask | one channel of weights in [0, 1] | `img.mask.luminance`, `img.mask.box`, `regions.toMask()` |
| Regions | discrete areas with id, box, area, centroid and measures | `img.detect.specks()`, `mask.components()` |
| Value | plain data: numbers, objects, arrays | `stats`, `histogram`, `coverage`, `count` |
| Param | a value a person can move | `P.slider`, `P.toggle`, `P.choice` |

## Lazy, immutable, shared nodes

`img.exposure(0.3)` computes nothing: it returns a node. The same call returns the same node, so shared work
is cached by node, scale and param values. Evaluation happens when you `await` a node, return a look, or
`show()` one.

```js
const a = img.exposure(0.3)
const b = a.gaussian(2)
return { same: a === img.exposure(0.3), graph: b.explain(), summary: await b }
```

Awaited data still remembers its node, so `const r = await img.detect.specks()` followed by
`r.where(...)` works; data fields win where names collide (`r.count` is the number). A node that is built and
thrown away produces an `UNUSED_NODE` warning, and comparing a node with `<` is `E_LAZY_COMPARE` — await it first.

## Doc px and scales

Lengths, boxes and points are in **doc px** (pixels of the original image) at every scale; areas in doc px².
`await` evaluates at a preview scale (long side about 1600 px), looks at a look scale (at most 1568 px) or, with
`look({ zoom })`, magnify a crop above full size (filters are computed at full size, then upsampled); the
engine rescales radii and boxes so a preview and a full-size result agree. Region detection runs at a fixed
`analysisScale`, so region ids and boxes are the same everywhere. Coordinates are absolute in the image's
**frame** (a window read with `cv.open(path, { box })` keeps the scan's coordinates).

## Encodings and working spaces

Every image has an **encoding**: `srgb` (photographs: the sRGB curve undone gives light) or `linear` (float
TIFF, DNG raw scans, `.npy`, density maps: the stored value is the quantity). `cv.open(path, { encoding })`
overrides what the file implies. Every pixel op has a **working space**: `linear` (exposure, blur, blend, stats,
levels: physical steps count equally) or `encoded` (thresholds, contrast, histogram, specks: visible steps count
equally), changed per call with `{ space }`. The graph inserts cached conversion nodes (`explain()` shows them),
luminance is always computed from linear light, and a look shows encoded values; masks and derived images
(`chroma`, `toLab`) show their values directly. Several images can be open under names:
`cv.open(path, { as: "ref" })`.

## Masks and regions

A **mask** is a soft field (a feathered sky); **regions** are identified things you count, rank and exclude
(143 specks, drop id 31). Each converts into the other: `regions.toMask()`, `mask.components()`.

```js
const sel = img.mask.luminance({ above: 0.45 }).minus(img.mask.box([0, 0, 40, 40])).feather(6)
const specks = img.detect.specks().where({ score: { gte: 0.5 } }).sortBy("area").top(3)
return {
  coverage: await sel.coverage(),
  inside: await img.stats({ mask: sel }).pick("meanL", "area"),
  biggest: (await specks).items.map(r => ({ id: r.id, box: r.box })),
}
```

## Lambdas

Two kinds of function can be passed in. **Builder** lambdas run once while the graph is built and return
nodes: `img.within(mask, i => i.exposure(-0.5))`. **Data** lambdas run at evaluation time on small data:
`regions.filter(r => r.aspect > 2)`. There are no per-pixel JavaScript functions; prefer `where({ … })`, which
runs in Go and follows params.

## Looks

A look is an image the agent receives: `node.look("label")` inside the returned value, or `show(node)`, which
also survives a later `throw`. Looks are numbered (the number is burned into the corner) and the caption gives
what the image is and its scale, `look 2: after — open(scan.tif) → levels (1568×1045, 1 image px = 4.0 doc px)`,
so coordinates read off a look convert to doc px. To judge small things, crop and zoom with coordinate rulers,
`img.crop([x, y, 64, 64]).look({ zoom: 8, ruler: 8 })`, use a view that makes them stand out
(`img.view("speck")`), and put many candidates in one contact sheet (`cv.sheet(regions, { views })`). At most
four images arrive per eval.

## Params

`P.slider("ev", { min: -2, max: 2, value: 0.3 })` is a node that goes wherever a number goes. When it moves
(`P.set` stands in for a person moving a slider; there is no slider UI yet), only nodes downstream recompute,
and the context block reports the move.

## Errors and warnings

Every error has the same shape: `code`, `message`, `where` (the agent's line), `path` (the graph path),
`hint` and usually a literal `fix`. Raw JavaScript errors that involve cv objects are translated into the same
shape. Warnings (`RANGE`, `UNUSED_NODE`) travel with results instead of failing them. `cv.help(name)` never
throws: unknown names return `{ found: false, didYouMean }`.

```js error=E_UNKNOWN_METHOD
const specks = img.detect.specks()
return specks.heal()   // fix: img.heal(regions, …)
```
