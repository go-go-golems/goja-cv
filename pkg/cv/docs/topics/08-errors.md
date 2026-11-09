# Errors and diagnostics

Every cv error has the same YAML shape: a stable code, a message, where (your line), the graph path, a hint and usually a literal fix you can paste. Warnings travel with results instead of failing them.

## Reading an error

```js error=E_UNKNOWN_METHOD
const specks = img.detect.specks()
return specks.heal()
```

```yaml
error:
  code: E_UNKNOWN_METHOD
  message: Regions has no method "heal".
  where: 'line 2: return specks.heal()'
  path: open(planted-256x192-16bit.tif) → detect.specks → ✗ heal
  hint: '"heal" is an Image method that takes Regions: call it on the image and pass this node.'
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
| E_UNKNOWN_OPTION | an option key is misspelt | the `fix` / `valid` list |
| E_ARG_TYPE, E_ARG_RANGE | wrong type or outside the hard limits | read `cv.help(op)` |
| E_WRONG_KIND | the receiver or an input has the wrong kind | convert: `.toMask()`, `.components()` |
| E_LAZY_COMPARE | a node used as a number | `(await node) > x` |
| E_NO_FIELD | a derived field does not exist | `valid` lists the fields |
| E_NO_IMAGE | no image is open | `img = cv.open("scan.tif")` |
| E_NO_DOCUMENT | `doc` is not available yet | return looks instead |
| E_UNKNOWN_TOPIC | no such docs topic or section | `cv.docs.list()` |
| E_TIMEOUT | the eval ran longer than 60 s | evaluate less, at preview scale |

```js error=E_UNKNOWN_OPTION
return img.levels({ blak: 0.02 })
```

## Warnings

- `RANGE line N: …` an argument is outside its usual range (it still runs).
- `UNUSED_NODE line N: …` a node was built and discarded (see `cv.docs.read("nodes-and-evaluation")`).

```js warns=RANGE
const loud = img.exposure(4)
return await loud.stats().meanL
```

## Help

`cv.help(name)` never throws, so you can ask for several names in one eval. It takes an op (`"levels"`, `"detect.specks"`: arguments with units, defaults, usual ranges and limits), a kind (`"Mask"`: its methods with one line each), `Kind.method` (`"Regions.filter"`) or a cv function (`"evaluate"`, `"compare"`, `"docs"`, `"show"`). An unknown name returns `{ found: false, name, didYouMean, hint }`.

```js
const names = ["detect.specks", "Mask", "Regions.filter", "evaluate", "mask.stats"]
return names.map(n => {
  const h = cv.help(n)
  return h.found === false ? { n, didYouMean: h.didYouMean, hint: h.hint } : { n, keys: Object.keys(h) }
})
```
