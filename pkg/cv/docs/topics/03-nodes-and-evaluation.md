# Nodes and evaluation

Calls build lazy, immutable, hash-consed nodes; nothing runs until you await a node, return a look, or use the node in something that is evaluated. Options are literal values. cv.explain(node) shows the graph; UNUSED_NODE and E_NOT_AWAITED catch the usual mistakes.

## Kinds

Every node has one kind, and every op says which kind it takes and returns.

| kind | evaluated value | made by |
|---|---|---|
| Image | float32 pixels in a space, encoded (file units) or linear (awaits to a summary) | `img`, `exposure`, `levels`, `within`, `heal`, `overlay`, `compare` |
| Mask | one channel in [0, 1] | `mask.luminance`, `mask.box`, `regions.toMask()`, `a.and(b)` |
| Regions | a list of areas with flat fields id, x, y, w, h, cx, cy, area, aspect and measures | `detect.specks`, `mask.components()`, `regions.where()` |
| Value | plain data | `stats`, `histogram`, `coverage`, `count` |

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

## Awaited data remembers its node

What `await` returns is plain data (YAML, `Object.keys` and `JSON.stringify` see only the data), but it remembers the node it came from. You can keep calling node methods on it and pass it to ops, which use the node:

```js
const specks = await img.detect.specks()          // data: { count, items, … }
const dust = specks.where({ score: { gte: 0.5 } })  // a Regions node again
return { all: specks.count, dust: await dust.count(), look: img.overlay(dust).look("dust") }
```

Data wins where names collide: `specks.count` is the number; the method form is `img.detect.specks().count()`. Ops recompute the node: they see its value, not an edited copy of the awaited data.

Awaited Regions are an object `{ count, items: [...] }`, not an array: the list is `.items`, and it holds every region (up to 100000; more is `E_TOO_MANY`). The YAML result shows at most 48 items of a list; return `cv.json(x)` or `cv.csv(x)` to hand a program all of them.

```js error=TypeError
const specks = await img.detect.specks()
return await specks.count()        // hint: specks.count is a number (41), not a method; fix: specks.count
```

```js error=E_NOT_ARRAY
const rows = await img.detect.specks().top(5)
return rows.slice(0, 2)            // fix: rows.items.slice(…)
```

## UNUSED_NODE

A node built as a statement and never returned, awaited, looked at, stored in `mem` or used by another node did nothing. The result then carries a warning with your line:

```js warns=UNUSED_NODE
img.exposure(0.3)
return "the line above was a no-op"
```

```yaml
warnings:
  - 'UNUSED_NODE line 1: img.exposure(0.3) builds a node and discards it. Nodes are lazy: nothing ran. Return it, await it, look at it, or keep it in mem.'
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
