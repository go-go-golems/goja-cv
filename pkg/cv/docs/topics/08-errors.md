# Errors and diagnostics

Every cv error has the same YAML shape: a stable code, a message, where (your line), the graph path, a hint and usually a literal fix you can paste. Warnings travel with results instead of failing them.

## Reading an error

```js error=E_NOT_AWAITED
const specks = img.detect.specks()
return specks.heal()
```

```yaml
error:
  code: E_NOT_AWAITED
  message: 'detect.specks() is a lazy Regions node and has no method "heal": regions are arguments of image methods.'
  where: 'line 2: return specks.heal()'
  path: open(planted-256x192-16bit.tif) → toGray → convert → detect.specks → ✗ heal
  hint: Pass the regions to the image method.
  fix: img.heal(regions, …)
```

- `where` is the line of your code; `path` is the chain of nodes that led to the failing call (`✗` marks it).
- `hint` explains; `fix` is literal code. Apply the fix, keeping your variable names: here `img.heal(specks)`.
- `valid` (when present) lists the allowed options, methods or values.

## Common codes

| code | meaning | usual fix |
|---|---|---|
| E_UNKNOWN_METHOD | no such method on this kind | the `fix`; synonyms are mapped (`blur` → `gaussian`, `brightness` → `exposure`) |
| E_NAMESPACE | the method lives in a namespace | `img.mask.luminance`, `img.detect.specks` |
| E_UNKNOWN_OPTION | an option key is misspelt or named differently (`min` → `above`, `radius` → `sigma` on a blur, `opacity` → `alpha`) | the `fix` / `valid` list |
| E_ARG_TYPE, E_ARG_RANGE | wrong type or outside the hard limits | read `cv.help(op)` |
| E_WRONG_KIND | the receiver or an input has the wrong kind or channel count | a mask is one channel: `.toGray()`; convert: `img.mask.regions(items)`, `.components()` |
| E_NOT_AWAITED | a property of, or a number from, a lazy Value or Regions node (`img.stats().p99`, `img.stats() > 0.5`) | `(await img.stats()).p99` |
| E_ARG_TYPE (a node as an option) | options are literal values: `levels({ white: d.stats() })` | `levels({ white: (await d.stats()).p99 })` |
| E_NO_IMAGE | no image is open | `img = cv.open("scan.tif")` |
| E_UNKNOWN_TOPIC | no such docs topic or section | `cv.docs.list()` |
| E_TIMEOUT | the eval ran longer than 60 s | crop before measuring, evaluate less per eval |

```js error=E_UNKNOWN_OPTION
return img.levels({ blak: 0.02 })
```

```js error=E_UNKNOWN_OPTION
return await img.mask.luminance({ min: 0.99 }).coverage()   // hint: Did you mean "above"?
```

## Warnings

- `RANGE line N: …` an argument is outside its usual range (it still runs). The same warning from a loop is listed once with `(×N)`.
- `UNUSED_NODE line N: …` a node was built and discarded (see `cv.docs.read("nodes-and-evaluation")`).
- `LOOKS: …` / `DATA: …` more than 4 looks (or 4 data items) in one eval: the extra ones, named by label, were not delivered. Split the work over several calls, or return fewer (one `cv.sheet` of many crops, one table with every row).

```js warns=RANGE
const loud = img.exposure(4)
return (await loud.stats()).meanL
```

## Help

`cv.help(name)` never throws, so you can ask for several names in one eval. It takes an op (`"levels"`, `"detect.specks"`: arguments with units, defaults, usual ranges and limits), a kind (`"Regions"`: its methods with one line each), `Kind.method` (`"Regions.filter"`) or a cv function (`"evaluate"`, `"compare"`, `"docs"`, `"show"`). An unknown name returns `{ found: false, name, didYouMean, hint }`.

```js
const names = ["detect.specks", "Regions", "Regions.filter", "evaluate", "mask.stats"]
return names.map(n => {
  const h = cv.help(n)
  return h.found === false ? { n, didYouMean: h.didYouMean, hint: h.hint } : { n, keys: Object.keys(h) }
})
```
