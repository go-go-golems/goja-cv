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

Status: milestones M0/M1 (engine core), M2 (eval tool: looks, YAML results, lints, context block,
`goja-cv` CLI and MCP server) and M3 (selection and retouch) of the CVJS-001 intern guide. The
document layer (M4) comes next.

## The eval tool (M2)

```
goja-cv eval -e 'return { s: await img.stats(), after: img.exposure(0.5).look("after") }' --image scan.tif --out looks/
goja-cv run script.js --image scan.tif [--out DIR] [--json]
goja-cv ops [--kind Image] [--json]
goja-cv help levels
goja-cv mcp --image scan.tif --root .          # MCP over stdio; --http 127.0.0.1:3092 for streamable HTTP
claude mcp add cv -- goja-cv mcp --image scan.tif --root .
```

Code is the body of an async function (`await`, `return`). Pre-bound globals: `cv`, `P`, `img`,
`mem` (persists between evals), `show(node, opts)`; `doc` arrives with the document layer (M4).
Results are YAML capped at 9000 characters with `warnings:` (RANGE, UNUSED_NODE with your line)
and a `context:` block; errors are YAML with `code, message, where, path, hint, fix`.
`node.look({ label, maxSide })` returned in the value (or `show()`) delivers a numbered image
whose caption gives the scale (`look 1: after (1568×1045, 1 image px = 4.0 doc px)`).
The guides are readable from inside eval, so an agent with only the eval tool can find them:
`cv.docs.list()`, `cv.docs.search("heal dust", { limit: 3 })` → `[{ id, title, section, snippet, score }]`,
`cv.docs.read("recipes", { section: "Remove dust" })` → Markdown (topics: getting-started, units-and-scales,
nodes-and-evaluation, lambdas, masks-and-regions, looks, params, errors, recipes; sources in
`pkg/cv/docs/topics`, every `js` block is run by `TestDocsCodeBlocksRun`). The MCP server sends short
instructions (eval contract, most used calls) and serves the topics as `cv://docs/<id>` next to the full op
reference `cv://reference`.
Exit codes: 0 ok, 1 cv error (still printed), 2 usage error. Go API: `pkg/cvsession`
(`cvsession.New`, `App.Evaluate`, `NewMCPServer`); xgoja: `pkg/xgoja/providers/cv` and
`examples/xgoja/cv-agent/xgoja.yaml`.

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
cv.help(); cv.help("levels"); cv.warnings(); cv.docs.search("mask feather")
```

Ops: `exposure`, `levels`, `contrast`, `toGray`, `gaussian`, `stats`, `histogram`, `explain`;
Value ops `field` (also `value.someField`), `pick`, `percentile`, `add/sub/mul/div/min/max/distance/abs`.
Errors are JS `Error`s named by code (`E_UNKNOWN_OPTION`, `E_UNKNOWN_METHOD`, `E_LAZY_COMPARE`, …)
with `code`, `path`, `hint`, `fix`, `valid`.

### Selection and retouch (M3)

```js
// masks: soft [0, 1] fields; lengths and coordinates in doc px
const sky = img.mask.luminance({ above: 0.62 })            // perceptual luminance, soft ramp
  .and(img.mask.gradient({ from: "top", to: 0.45 }))
  .minus(img.mask.color({ hue: 30, tol: 15 }))
  .feather(20)                                             // also grow(r), shrink(r), not(), or(m), refine(img, { r, eps })
await sky                                                  // { kind: "Mask", w, h, coverage, scale, docW, docH }
await sky.coverage()
img.mask.box([x, y, w, h]); img.mask.ellipse([x, y, w, h]); img.mask.flood([x, y], { tol })

// edits inside a mask: the function runs once at build time on a placeholder
const ev = P.slider("skyEv", { min: -2, max: 1, value: -0.4 })
const out = img.within(sky, i => i.exposure(ev))          // moving skyEv re-runs exposure + within only
img.blend(other, { mode: "multiply", opacity: 0.5, mask: sky })

// regions: discrete, identified; ids and doc-px boxes are the same at every scale
const specks = img.detect.specks({ maxArea: 120, minContrast: 0.04 })
await specks                                               // { count, items: [{ id, box, area, centroid, contrast, localNoise, sharpness, score, polarity, … }], truncated }
const keep = specks.where({ score: { gte: P.slider("minScore", { min: 0, max: 1, value: 0.5 }) } })
  .without([41]).sortBy("contrast").top(200)               // also only(ids), count(), toMask({ grow }), filter(r => …)
const healed = img.heal(keep, { radius: 3, method: "patch" })   // or "diffuse"; img.inpaint(mask)
await img.check.dust()                                     // { verdict, count, candidates, maxArea, coverage, noise }
img.overlay(specks, { labels: "id", color: "red" })        // looking: also sky.overlay(img)
cv.compare([img, healed], { mode: "side" })                // "side" | "split" | "diff"
mask.components({ minArea: 4 })                            // Mask → Regions
```

## Layout

- `pkg/cv/buffer` – planar float32 linear-light buffers, sRGB LUTs, IO, pyramid
- `pkg/cv/parallel` – band-parallel helpers
- `pkg/cv/kernels` – pure kernels (tone, filter, measures, masks, EDT, guided filter,
  morphology, labelling, speck detection, heal, drawing)
- `pkg/cv/graph` – op registry, nodes and hash-consing, params, evaluator, cache
- `pkg/cv/ops` – op registrations
- `pkg/gojamod/cv` – go-go-goja native module `cv` and its embedded `prelude.js`
- `pkg/cv/look` – looks (lazy image results with burned-in numbers)
- `pkg/cv/docs` – the agent guides (`cv.docs`), embedded Markdown topics
- `pkg/cvsession` – the eval session on replapi, result processor, MCP server
- `cmd/goja-cv` – the CLI
- `pkg/xgoja/providers/cv` – xgoja provider for the cv module

## Development

```
make test     # go test ./... -count=1
make vet
make lint     # golangci-lint
make staticcheck
make testdata # regenerate the synthetic test images
go run ./internal/testimage/gen -out /tmp -planted   # the planted-fault dust scan (tests generate it)
CV_LOOK_DIR=/tmp/look go test ./pkg/cv/ops -run OverlayAndCompare   # write overlay/side/diff PNGs
```

go-go-goja is pinned to the commit of go-go-golems/go-go-goja#108 (result content, replmcp, xgoja mcp provider)
until that PR is merged and released. To work against a local checkout, add
`replace github.com/go-go-golems/go-go-goja => ../go-go-goja` (do not commit it).
