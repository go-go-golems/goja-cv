# goja-cv

A Go image-analysis and editing engine exposed to JavaScript as `require("cv")`
through [go-go-goja](https://github.com/go-go-golems/go-go-goja). An LLM agent with a
JavaScript `eval()` tool builds lazy image-processing graphs ("an automated
Photoshop"); the graph, the evaluator, the cache and every pixel stay in Go.

```js
const cv = require("cv")
const img = cv.open("scan.tif")
const s = img.stats()                                  // a lazy Value node
const fixed = img.levels({ black: s.p01, white: s.p99 }) // node-valued args are lifted
return await fixed.stats()                             // evaluated at preview scale
```

Status: milestones M0/M1 of the CVJS-001 intern guide (engine core). The eval tool,
looks and MCP come in M2.

## What works today

```js
const cv = require("cv"), P = cv.P
const img = cv.open("testdata/gradient-64x48-16bit.tif")   // Image node (header read, pixels lazy)
const a = img.exposure(0.3)                     // linear light, 2^ev
img.exposure(0.3) === a                         // true: nodes are hash-consed
await a.gaussian(2)                             // { kind: "Image", w, h, channels, scale, docW, docH }
await img.stats()                               // { meanL, medianL, p01, p99, clipLow, clipHigh, percentiles, perChannel, w, h, scale }
img.levels({ black: s.p01, white: s.p99 })      // node-valued args (also inside options) are lifted
await img.histogram({ bins: 64, channel: "L" })
await cv.evaluate(node, { scale: "full" })      // explicit scale: number, "preview" (default), "look", "full"
img.exposure(0.3).gaussian(2).explain()         // the graph as lines, with sizes and cache state
const ev = P.slider("ev", { min: -2, max: 2, value: 0.3 }); P.set("ev", 1)
cv.help(); cv.help("levels"); cv.warnings()
```

Ops: `exposure`, `levels`, `contrast`, `toGray`, `gaussian`, `stats`, `histogram`, `explain`;
Value ops `field` (also `value.someField`), `pick`, `percentile`, `add/sub/mul/div/min/max/distance/abs`.
Errors are JS `Error`s named by code (`E_UNKNOWN_OPTION`, `E_UNKNOWN_METHOD`, `E_LAZY_COMPARE`, …)
with `code`, `path`, `hint`, `fix`, `valid`.

## Layout

- `pkg/cv/buffer` – planar float32 linear-light buffers, sRGB LUTs, IO, pyramid
- `pkg/cv/parallel` – band-parallel helpers
- `pkg/cv/kernels` – pure kernels (tone, filter, measures)
- `pkg/cv/graph` – op registry, nodes and hash-consing, params, evaluator, cache
- `pkg/cv/ops` – op registrations
- `pkg/gojamod/cv` – go-go-goja native module `cv` and its embedded `prelude.js`

## Development

```
make test     # go test ./... -count=1
make vet
make lint     # golangci-lint
make staticcheck
make testdata # regenerate the synthetic test images
```

go-go-goja is consumed through a local `replace ../go-go-goja`.
