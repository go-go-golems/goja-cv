# Nodes and evaluation

Calls build lazy, immutable, hash-consed nodes; nothing runs until you await a node, return a look, or use the node in something that is evaluated. Options are literal values. cv.explain(node) shows the graph; E_NOT_AWAITED catches reading data that was not awaited.

## Kinds

Every node has one kind, and every op says which kind it takes and returns.

| kind | evaluated value | made by |
|---|---|---|
| Image | float32 pixels in a space, encoded (file units) or linear (awaits to a summary) | `img`, `exposure`, `levels`, `within`, `heal`, `overlay`, `compare` |
| Image with one channel (a mask) | weights in [0, 1] | `mask.luminance`, `mask.box`, `mask.regions(items)`, `a.and(b)` |
| Regions | a list of areas with flat fields id, x, y, w, h, cx, cy, area, aspect and measures (awaits to an array; no methods) | `detect.specks`, `mask.components()`, `measure(items)` |
| Value | plain data | `stats`, `histogram`, `coverage`, `profile` |

## Lazy and hash-consed

`img.exposure(0.3)` computes nothing; it returns a node. The same call returns the same node (`===`), so shared sub-graphs are computed once and cached by node and scale.

```js
const a = img.exposure(0.3)
const same = a === img.exposure(0.3)
const b = a.gaussian(2)
return { same, graph: cv.explain(b).graph }
```

## Evaluating

Evaluation happens when you:

- `await` a node: a Value is evaluated at full resolution and gives its data, Regions give `{ count, items, … }`; an Image gives its summary `{ kind, w, h, encoding, space, frame, … }` (plus file metadata for an opened file) without computing anything;
- create a look (`node.look()`): every look of an eval is rendered after it.

`await` is the only way to evaluate. `cv.explain(node)` describes a node without evaluating it: id, kind, op, call, frame, encoding, space, path, and `graph`, one line per node (call, kind, size, cached).

## Options are literal values

Every option is a number, string, boolean, list or object: written in the script or computed by JavaScript. A measurement that feeds an option is a **data dependency**: await it first, then build the node. The cache makes the extra evaluation cheap (the `stats` node and its inputs are reused).

```js
const s = await img.stats()
const fixed = img.levels({ black: s.p01, white: s.p99 })    // numbers: s was awaited
const { meanL, p01, p99 } = await fixed.stats()
return { after: { meanL, p01, p99 }, look: fixed.look("levels") }
```

Passing a node where an option is expected is `E_ARG_TYPE` with the fix:

```js error=E_ARG_TYPE
const d = img.dog(1, 4)
return d.levels({ black: 0, white: d.stats() }).look()   // fix: { white: (await node.stats()).p99 }
```

Node inputs exist only where an op takes an image, a mask or regions: `a.sub(b)`, `img.within(mask, fn)`, `img.overlay(regions)`.

## Awaited regions are arrays

What `await` returns is plain data (YAML, `Object.keys` and `JSON.stringify` see only the data). Awaited Regions are a plain JavaScript array of items, every region (up to 100000; more is `E_TOO_MANY`): filter, sort and slice it, and pass it back wherever regions go. Each item keeps a hidden link to its region's pixels (`cv.docs.read("masks-and-regions")`). A Regions node itself has no methods; its properties are lazy data:

```js
const specks = await img.detect.specks()          // an array of items
const dust = specks.filter(r => r.score >= 0.5)   // still linked
return { all: specks.length, dust: dust.length, look: img.overlay(dust).look("dust") }
```

```js error=E_NOT_AWAITED
return img.detect.specks().filter(r => r.score >= 0.5)   // fix: (await img.detect.specks()).filter
```

Other awaited data is not linked to its node: `(await img.stats()).p99` is a number. The YAML result shows at most 48 items of a list; `await cv.save("x.csv", items)` hands a program all of them.

## A node you don't use does nothing

A node you don't await, look at or save does nothing: no pixels are computed, nothing is written, and nothing warns. A forgotten `await` shows up as a missing value in your result. What the eval does wait for are `cv.load` and `cv.save`: it ends only when every load and save it started has settled, awaited or not, and a failure nobody handled is an `UNHANDLED` warning.

```js
img.exposure(0.3)                       // built, never used: a no-op
cv.save("probe.json", { ok: true })     // not awaited: still written before the eval ends
return "two lines above: one no-op, one saved file"
```

## E_NOT_AWAITED

A Value or Regions node has no data until it is awaited. Reading a property that is not one of its methods (`img.stats().p99`), or using it as a number (`img.stats() > 0.5`), is `E_NOT_AWAITED` with the fix:

```js error=E_NOT_AWAITED
if (img.stats().meanL > 0.5) return "bright"    // fix: (await img.stats()).meanL
return "dark"
```

```js
const meanL = (await img.stats()).meanL
return meanL > 0.5 ? "bright" : "dark"
```

Nodes are also immutable: assigning a property (`node.x = 1`) throws `E_IMMUTABLE`.
