# Nodes and evaluation

Calls build lazy, immutable, hash-consed nodes; nothing runs until you await a node, return a look, or use the node in something that is evaluated. explain() shows the graph; UNUSED_NODE and E_LAZY_COMPARE catch the usual mistakes.

## Kinds

Every node has one kind, and every op says which kind it takes and returns.

| kind | evaluated value | made by |
|---|---|---|
| Image | linear float32 pixels (awaits to a summary) | `img`, `exposure`, `levels`, `within`, `heal`, `overlay`, `compare` |
| Mask | one channel in [0, 1] | `mask.luminance`, `mask.box`, `regions.toMask()`, `a.and(b)` |
| Regions | a list of areas with id, box, area, centroid and measures | `detect.specks`, `mask.components()`, `regions.where()` |
| Value | plain data | `stats`, `histogram`, `coverage`, `count`, `stats().meanL` |
| Param | a value the person can move | `P.slider`, `P.toggle`, `P.choice` |

## Lazy and hash-consed

`img.exposure(0.3)` computes nothing; it returns a node. The same call returns the same node (`===`), so shared sub-graphs are computed once and cached by node, scale and param values.

```js
const a = img.exposure(0.3)
const same = a === img.exposure(0.3)
const b = a.gaussian(2)
return { same, graph: b.explain() }
```

## Evaluating

Evaluation happens when you:

- `await` a node: a Value gives its data, an Image a summary `{ kind, w, h, scale, … }`, Regions `{ count, items, … }`;
- return a look (`node.look()`) or `show(node)`;
- `cv.evaluate(node, { scale })` for an explicit scale.

Fields of a Value node are Value nodes too, so they can feed other ops before anything runs:

```js
const s = img.stats()
const fixed = img.levels({ black: s.p01, white: s.p99 })    // s.p01 is a lazy Value node
const after = await fixed.stats().pick("meanL", "p01", "p99")
return { after, look: fixed.look("levels") }
```

## UNUSED_NODE

A node built as a statement and never returned, awaited, looked at, stored in `mem` or used by another node did nothing. The result then carries a warning with your line:

```js warns=UNUSED_NODE
img.exposure(0.3)
return "the line above was a no-op"
```

```yaml
warnings:
  - 'UNUSED_NODE line 1: img.exposure(0.3) builds a node and discards it. Nodes are lazy: nothing ran. Return it, await it, show() it, or keep it in mem.'
```

## E_LAZY_COMPARE

Comparing or adding a node with `<`, `>`, `+` is an error: the node has no value yet. Await first.

```js error=E_LAZY_COMPARE
if (img.stats().meanL > 0.5) return "bright"
return "dark"
```

```js
const meanL = (await img.stats()).meanL
return meanL > 0.5 ? "bright" : "dark"
```

Nodes are also immutable: assigning a property (`node.x = 1`) throws `E_IMMUTABLE`.
