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

Status: early development (milestones M0/M1 of the CVJS-001 intern guide).

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
```

go-go-goja is consumed through a local `replace ../go-go-goja`.
