---
Title: Design and implementation guide
Ticket: GCV-003
Status: active
Topics:
    - measurement
    - regions
    - export
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/cv/graph/regions.go
      Note: Region.Field (x, y, w, h, cx, cy) vs Region.Data (box, centroid); RegionFields, KnownRegionFields; SummaryLimit 100
    - Path: pkg/cv/ops/regions.go
      Note: where/sortBy validate against the global KnownRegionFields; components adds meanMask
    - Path: pkg/cv/ops/detect.go
      Note: speckFields list (box, centroid) used for help
    - Path: pkg/cv/ops/measure.go
      Note: stats with mask/box; the measurement family the new ops join
    - Path: pkg/cv/kernels/measure_weighted.go
      Note: StatsWeighted is an exported wrapper of statsWeighted
    - Path: pkg/cv/graph/args.go
      Note: number[] Len 4 boxes and Len 2 points; new box and point types with object coercion
    - Path: pkg/cv/look/chart.go
      Note: histogram-only chart; generalized to declared chart shapes (profile)
    - Path: pkg/cvsession/yaml.go
      Note: result caps (48 items, 2000 chars, 9000 total) stay as presentation policy
    - Path: pkg/cvsession/session.go
      Note: Options gains OutRoot and Preludes; MaxLooks enforced per kind
    - Path: pkg/cvsession/mcp.go
      Note: replmcp limits count every content item as an image
    - Path: cmd/goja-cv/eval.go
      Note: writeLooks duplicates resultcontent.WriteFiles; gains --prelude and --out-root
    - Path: pkg/gojamod/cv/binding.go
      Note: resultToJS uses SummaryLimit; data-lambda hint lists box/centroid
    - Path: pkg/cv/graph/suggest.go
      Note: synonym measure -> stats conflicts with the new measure op
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/84-cv-helpers.js
    - /Users/manuel/go/pkg/mod/github.com/go-go-golems/go-go-goja@v0.10.8-0.20261109062017-5e1ba117462e/pkg/resultcontent/resultcontent.go
    - /Users/manuel/go/pkg/mod/github.com/go-go-golems/go-go-goja@v0.10.8-0.20261109062017-5e1ba117462e/pkg/replmcp/tools.go
Summary: "Intern guide for GCV-003: point sampling, line profiles, core-versus-ring measures, measuring regions on another image, one region field schema, no item cap, JSON/CSV result content, saving images under an out-root, and CLI preludes."
LastUpdated: 2026-11-09T16:00:00-05:00
WhatFor: "Implementing GCV-003 (after GCV-001 and GCV-002)."
WhenToUse: "Read before touching measurement ops, Regions fields, awaited summaries, result content or the CLI."
---

# GCV-003 — Measurement and data out: sampling, profiles, region measures, schema, export

This guide is for an intern who will implement GCV-003 in goja-cv. It is the third of three tickets. It uses
GCV-001's image model (working spaces, the `Level` unit, frames, encodings when writing files) and GCV-002's
render pipeline and text font (profile plots, labels). The CVJS-001 vocabulary (node, kind, Regions, Value,
evaluation, doc px, look) and its distinction between masks (soft fields) and regions (discrete, identified
things) are assumed.

**Dependencies.** From GCV-001: the `space` option and its resolution (measurements default to `linear`),
`Level` units (ring thresholds, sample values), `Frame` (coordinates on windows), `Encoding` (written into saved
files), and `chroma`'s kernel. From GCV-002: `pkg/cv/text` and the look renderer (charts), and the sheet and
annotate ops, which accept region items once this ticket's schema exists.

## 1. Executive summary

goja-cv measures well inside a mask or box (`stats`), but an agent cannot ask "what is the value here", "what
does the line from A to B look like", or "how much does this spot stand out from its surroundings" without
building masks and calling `stats` three times per point. The colleague wrote a JavaScript helper for the last
question (`mem.probe` in `scripts/84-cv-helpers.js`), and it became the main number their agents use. Regions
cannot be measured on another image, so their where-to-zoom list is probed point by point. Regions also expose
their geometry under two sets of names (`cx` in `where()`, `centroid` in items), the awaited list stops at 100
items, and the only way to get data out is a YAML text capped at 9000 characters.

This ticket adds three measurement ops (`sample`, `profile`, `measure`), a `regions.measure(image)` op, one
region field schema shared by filters, items, help and errors, removes the item cap, adds JSON and CSV result
content (`cv.json`, `cv.csv`), saving of images and data under a separate `--out-root`, and a CLI `--prelude`.
It removes the duplicate field lists, the 100-item summary limit, the `meanMask` special case, the box/point
arrays that only some ops accept as objects, and the CLI's hand-written file writer.

## 2. Problem statement and scope

### 2.1 The workflow

The colleague's agents decide by eye on magnified crops (GCV-002) and back the decision with numbers. The
playbook's decision table (DUST-002 `reference/04-…`, section 4) uses two numbers per spot:

| | real dust (median) | non-dust (middle half) |
|---|---|---|
| snr = step / ring spread | 1.7 – 4.0 | 0.3 – 1.8 |
| chroma (colour film) | 0.2 – 0.6 | 0.4 – 0.9 |

Both come from `mem.probe(x, y, r)`: the mean of a disk of radius r, the mean and the 10–90 % spread of a ring
from r + 3 to r + 9 px, and the per-channel steps. Their where-to-zoom list is `mem.hot(k)`: the connected blobs
of the DoG view above k × its 99th percentile, each probed. Calibration against human labels
(`scripts/83-cv-specks-vs-gold.py`) runs goja-cv from Python and needs every region of a tile.

### 2.2 The issues

| issue | title | priority |
|---|---|---|
| 9 | Point sampling, line profiles and ring measures | P2 |
| 10 | Measure regions against another image | P2 |
| 16 | Bulk data out | P2 |
| 17 | Consistent field names between `where()` and items | P2 |
| 21 | Save derived images | P3 |
| 23b | CLI prelude | P3 |

Out of scope: changing `detect.specks` scoring and area (issues 11 and 12; they are detector work and belong to a
separate ticket once `measure` exists to evaluate them), the document layer (`doc.apply` etc.).

## 3. Concepts

### 3.1 Sampling a point

goja-cv coordinates are continuous doc px: pixel i covers [i, i + 1), and a region's centroid of 1312.5 is the
centre of pixel 1312 (`pkg/cv/graph/regions.go:97`). **Sampling** a point means reading the image near a
coordinate. With radius r = 0 it reads the pixel that contains the point. With r > 0 it averages a disk,
weighting each pixel by the fraction of its area inside the disk, the same exact-coverage rule `mask.ellipse`
uses (`pkg/cv/kernels/mask.go:175`). A disk average is less noisy than one pixel and does not depend on whether
the point falls on a pixel centre or an edge.

### 3.2 Profiles

A **profile** is the sequence of values along a straight line from A to B, sampled at a fixed step (1 doc px by
default). With a width w > 1 each sample averages w values across the line, which suppresses grain without
blurring along it. The shape of a profile across an edge separates a soft transition (dust blurred by the
scanner optics, edge σ ≈ 1.6 px in the colleague's data) from a sharp one (a grain boundary, a scene edge).

### 3.3 Core and ring

How much a spot **stands out** is a comparison between the spot and its surroundings. The **core** is a disk
of radius r around the spot; the **ring** is the annulus between radii r1 and r2 around it. Four numbers follow:

| name | definition | meaning |
|---|---|---|
| `step` | mean(core) − mean(ring) of the gray plane | how much brighter (positive) or darker the spot is |
| `spread` | p90(ring) − p10(ring) of the gray plane | how grainy the surroundings are |
| `snr` | step / max(spread, 1e-3) | the step in units of local graininess |
| `chroma` | (max_c step_c − min_c step_c) / max(\|step\|, 1e-3) | how unequal the step is between channels; 0 for a neutral spot |

The **gray plane** is a weighted mean of the channels: Rec. 709 weights by default, or equal weights with
`gray: "average"`. The per-channel steps `stepR`, `stepG`, `stepB` use the channels themselves.

The defaults follow the colleague's probe: ring from r + 3 to r + 9. A worked example with numbers from a
synthetic speck: core mean 0.61, ring mean 0.52, ring p10 0.49 and p90 0.55. Then step = 0.09, spread = 0.06,
snr = 1.5, which the playbook's table puts in the "zoom and look" range. If the per-channel steps are 0.10, 0.09,
0.08, chroma = (0.10 − 0.08) / 0.09 = 0.22: neutral, as dust is.

### 3.4 Measuring regions on another image

A region set made from one image can be measured on another. The where-to-zoom list is made from a mask of the
DoG view; ranking it needs the DoG's peak inside each blob, and judging neutrality needs the original colour
image. **Measuring regions on an image** means computing statistics over each region's pixels (or its core and
ring) in that image and adding them to the region as fields.

### 3.5 Schema

A **schema** is the list of fields a value has, with their types and meanings. Regions are a table: one row per
region, one column per field. The field names a filter accepts, the names the items carry, the names help
prints and the names errors suggest must be the same list, or agents learn one set and are refused by another.

### 3.6 Data out

An eval's result is text, rendered as YAML and capped for the model's context. **Data out** is a different
channel for data meant for programs: JSON or CSV delivered as result content (go-go-goja's
`resultcontent.Provider`, the same mechanism looks use) or written to a file. **Saving** writes an image or data
to a file under an **out-root**, a directory the session may write to, separate from the directories it may read.

The key points:

- A measurement reads values in a working space, linear by default like `stats`, and reports which.
- Core/ring measures turn "does this stand out" into numbers that are comparable across a tile.
- One schema per Regions node is the only source of field names.
- Text results are for the model; data content and files are for programs.

## 4. Current-state analysis

### 4.1 Measuring a point takes masks and three stats calls

The only measurement op on images is `stats` (`pkg/cv/ops/measure.go:126-140`), with optional `mask`/`box`
selections (`measure.go:53-98`), plus `histogram`. The colleague's probe builds two ellipse masks per point and
calls `stats` twice (`84-cv-helpers.js`, `mem.probe`, lines 35-47):

```js
const core = img.mask.ellipse([x - r, y - r, 2 * r + 1, 2 * r + 1])
const ring = img.mask.ellipse([x - r - 9, y - r - 9, 2 * r + 19, 2 * r + 19]).minus(img.mask.ellipse([x - r - 3, y - r - 3, 2 * r + 7, 2 * r + 7]))
const c = await img.stats({ mask: core })
const g = await img.stats({ mask: ring, percentiles: [0.1, 0.9] })
```

Each call evaluates a full-image weighted stats kernel (`pkg/cv/kernels/measure_weighted.go:27-114`), so probing
28 spots evaluates 56 whole-image statistics. `mem.hot` probes each blob sequentially with `await` in a loop
(`84-cv-helpers.js:52-61`).

### 4.2 Regions carry geometry and one mask measure

`mask.components` returns regions with geometry and `meanMask`, the mean of the mask over the region
(`pkg/cv/ops/regions.go:223-263`, line 254). There is no op to add measures from another image.

### 4.3 Two sets of field names

`Region.Field` (`pkg/cv/graph/regions.go:111-138`) answers `id, area, x, y, w, h, cx, cy` and measures and tags;
`Region.Data` (`regions.go:158-175`), which JavaScript receives, has `id, box, area, centroid` and measures and
tags. So `where({ cx: { gt: 100 } })` works and `item.cx` is `undefined`; `item.centroid` works and
`where({ centroid })` is refused. The field list exists four times:

| list | file:line | contents |
|---|---|---|
| `RegionFields` | `pkg/cv/graph/regions.go:105` | `id, area, x, y, w, h, cx, cy, aspect` |
| `KnownRegionFields` | `regions.go:108-109` | the above plus every measure any op might add, hand-maintained |
| `FieldNames()` | `regions.go:141-151` | geometry plus the region's actual measures |
| `speckFields` | `pkg/cv/ops/detect.go:10` | `id, box, area, centroid, aspect, …` (the item names) |

`where` and `sortBy` validate against the global `KnownRegionFields` (`pkg/cv/ops/regions.go:47-56`,
`:295-304`), so `components().where({ score })` passes validation and fails at evaluation. The data-lambda hint
lists `{ id, box, area, centroid, score, … }` (`pkg/gojamod/cv/binding.go:74`).

### 4.4 Awaited Regions stop at 100 items

`resultToJS` returns `x.Summary(graph.SummaryLimit)` (`pkg/gojamod/cv/binding.go:339`) with `SummaryLimit = 100`
(`regions.go:177-178`), and the summary sets `truncated` (`regions.go:182-192`). JavaScript code that iterates
items (the colleague's `mem.hot`) silently sees the first 100. The YAML renderer then caps arrays at 48 items,
strings at 2000 characters and the whole result at 9000 characters (`pkg/cvsession/yaml.go:20-27`, `:279-283`).
To get ~200 specks the colleague pages with `where({ id: { gte, lt } })` and parses YAML flow sequences.

### 4.5 Boxes and points are bare arrays

Box arguments are `number[]` with `Len: 4` (`stats.box`, `pkg/cv/ops/measure.go:134`; `mask.box` and
`mask.ellipse`, `pkg/cv/ops/mask.go:149`); points are `number[]` with `Len: 2` (`mask.flood`, `mask.go:266`). A
region item cannot be passed where a box is expected, so agents write `[r.box[0], r.box[1], …]` or
`r.centroid.map(Math.round)`.

### 4.6 Data and files out

Results come back only as YAML text. Looks are the only result content. The CLI writes looks to files with its
own writer, which names non-image content `.bin` (`cmd/goja-cv/eval.go:175-224`), although go-go-goja's
`resultcontent.WriteFiles` and `ExtensionForMIME` do the same job with proper extensions
(`resultcontent/files.go:48-137`). Images can be written only by the CLI as looks (8-bit JPEG/PNG). The session
has read roots (`pkg/cvsession/session.go:56`, enforced by `checkPath`, `native.go:62-88`) and no write roots.

go-go-goja's MCP transport maps `application/json` and `text/*` content to MCP `TextContent` (`replmcp/tools.go`,
`toMCPContent`), but counts every content item against `MaxImages` (`tools.go`, the `attached >= maxImages`
check), and goja-cv sets that limit to `MaxLooks` = 4 (`pkg/cvsession/mcp.go:61`).

### 4.7 CLI sessions start empty

`goja-cv eval` and `run` create a fresh session per process (`cmd/goja-cv/eval.go:88-137`). `mem` helpers
defined in one CLI call are gone in the next, so the colleague re-pastes `84-cv-helpers.js` into every call.

## 5. Gap analysis

| need | today | gap |
|---|---|---|
| value at a point, many points (9) | masks + stats per point | `sample` |
| line profile, as numbers and a plot (9) | none | `profile`, line chart |
| core vs ring for many points (9) | 2 stats per point, JS helper | `measure` |
| region strength on another image, neutrality (10) | probe per centroid | `regions.measure(image)` |
| one set of field names (17) | four lists, two spellings | per-node schema |
| all items (16) | 100 | no cap |
| data for programs (16) | YAML text | `cv.json`, `cv.csv` content |
| derived images to files (21) | CLI looks, 8-bit | `node.save` under `--out-root` |
| helpers in every CLI run (23b) | re-paste | `--prelude` |

## 6. Legacy to remove or refactor

1. **The second set of region names.** Items lose `box` and `centroid`; they carry `x, y, w, h, cx, cy` like
   the filters. No alias in either direction.
2. **Hand-maintained field lists**: `RegionFields`, `KnownRegionFields`, `speckFields`, `Region.FieldNames`.
   Replaced by `FieldSpec` schemas declared by the ops that produce fields (§7.5).
3. **`meanMask` in `components`.** A special measure for one op; `regions.measure(mask)` gives it (`mean`).
4. **`SummaryLimit` and `truncated`.** Awaited Regions contain every item.
5. **`number[]` boxes and points.** Replaced by `box` and `point` argument types that also accept objects with
   the schema's field names (§7.6). Every op with a box or point argument switches.
6. **`kernels.StatsWeighted` wrapping `statsWeighted`** (`measure_weighted.go:27-29`): one exported function.
7. **The CLI's `writeLooks`**: replaced by `resultcontent.WriteFiles`.
8. **The `measure → stats` op synonym** (`pkg/cv/graph/suggest.go:29`): `measure` becomes an op.
9. **The data-lambda hint string** listing `box, centroid` (`binding.go:74`): generated from the schema.
10. **The hand-written `cvFuncs` help table** (`pkg/gojamod/cv/help.go:24-43`): module functions are registered
    with ArgSpecs (`graph.FuncDef`, §7.9); GCV-002 already moved `compare` and `sheet` to registry ops.
11. **Histogram-only charts** (`pkg/cv/look/chart.go:11-40`, `look.go:87-92`): Value ops declare a chart shape.

## 7. Proposed architecture

### 7.1 Overview

```
 measurement ops (Value)            region ops (Regions)                data out
 ───────────────────────            ────────────────────                ────────
 img.sample(points, {r})            mask.components()   ─┐              cv.json(x) ─┐  resultcontent.Provider
 img.profile(a, b, {width})         img.detect.specks()  ├─ schema      cv.csv(x)  ─┘  → MCP text / CLI files
 img.measure(points, {core, ring})  regions.measure(img) ┘  per node     node.save(path), data.save(path) → out-root
        │                                     │
        └──── kernels/probe.go: disk/annulus coverage sums, weighted quantiles (shared) ────┘
```

All measurement ops share one kernel file, `pkg/cv/kernels/probe.go`, which computes coverage-weighted sums and
quantiles over disks and annuli in a small window around each point. Nothing evaluates a whole-image statistic
per point.

### 7.2 `sample`

```js
await img.sample([[1337, 131], [1683, 123]], { r: 2 })
```

```yaml
- { x: 1337, y: 131, l: 0.612, r: 0.640, g: 0.605, b: 0.590, n: 13.1 }
- { x: 1683, y: 123, l: 0.548, r: 0.561, g: 0.547, b: 0.532, n: 13.1 }
```

| argument | type | default | meaning |
|---|---|---|---|
| points (positional) | `point[]` or Regions | — | points in root doc px; Regions use their centroids |
| `r` | doc px | 0 | 0: the pixel containing the point; > 0: coverage-weighted disk mean |
| `space` | space | linear | GCV-001 working space |
| `analysisScale` | (0, 1] | 1 | scale the image is read at (static, like Regions ops) |

Rows carry `l` (Rec. 709-weighted mean of the channels in the working space), the channel means (`r, g, b`, or
`v` for single-channel images) and `n`, the covered area in doc px². Points outside the frame produce a row with
`outside: true` and no values rather than an error, so one bad point does not lose a batch; a `RANGE` warning
counts them. The node's static `Space` is the resolved space, so lifting `sample(...)[0].l` into `levels` gets
GCV-001's `SPACE` check.

### 7.3 `profile`

```js
const p = img.profile([1320, 131], [1354, 131], { width: 3 })
return { values: (await p).l, plot: p.look("across the speck") }
```

```yaml
from: [1320, 131]
to: [1354, 131]
step: 1
width: 3
t: [0, 1, 2, …, 34]          # doc px along the line
x: [1320, 1321, …]
y: [131, 131, …]
l: [0.52, 0.52, 0.53, …]
r: […]
g: […]
b: […]
```

Samples are bilinear at `t` = 0, step, 2·step, … ≤ length, averaged over `width` positions spaced 1 doc px
across the line. At most 4096 samples (`E_ARG_RANGE` with a larger step as the fix). The op declares a chart
shape (§7.8): `look()` of a profile renders a line chart, one line per channel plus `l`, with `t` on the x axis
in doc px and value ticks in the working space, labelled with `pkg/cv/text`.

### 7.4 `measure` and `regions.measure`

Points:

```js
const rows = await img.measure([[1337, 131], [1683, 123]], { core: 3 })   // ring defaults to [core + 3, core + 9]
```

```yaml
- { x: 1337, y: 131, core: 0.612, ring: 0.521, step: 0.091, spread: 0.061, snr: 1.49, stepR: 0.10, stepG: 0.09, stepB: 0.08, chroma: 0.22 }
```

| option | type | default | meaning |
|---|---|---|---|
| `core` | doc px | 3 | core disk radius |
| `ring` | `[r1, r2]` doc px | `[core + 3, core + 9]` | annulus radii |
| `channels` | bool | true for colour images | add `stepR/G/B` and `chroma` |
| `gray` | `"rec709" \| "average"` | `rec709` | weights of the gray plane used for `core`, `ring`, `step`, `spread` |
| `space` | space | linear | GCV-001 working space |
| `analysisScale` | (0, 1] | 1 | as for `sample` |

Regions (issue 10):

```js
const hot = dog.levels({ black: 0, white: dog.stats().p99.mul(2) }).mask.luminance({ above: 0.99 }).components({ minArea: 4 })
const ranked = hot.measure(dog, { stats: ["max"], prefix: "dog" })     // adds dogMax
                  .measure(img, { ring: true })                         // adds core, ring, step, spread, snr, stepR/G/B, chroma
                  .sortBy("snr").top(30)
return { n: await hot.count(), top: (await ranked).items.map(r => [r.id, r.cx, r.cy, r.snr, r.chroma].join(" ")) }
```

`regions.measure(image, opts)` returns Regions with the same items plus new fields:

| option | type | meaning |
|---|---|---|
| `stats` | list of `mean`, `max`, `min`, `std`, `median`, `p10`, `p90` | statistics over each region's own pixels |
| `channels` | bool | per-channel versions (`meanR`, `maxG`, …) of the requested stats |
| `ring` | `true` or `[r1, r2]` | core/ring measure at each centroid; core radius = clamp(√(area / π), 2, 8) doc px unless `core` is given; ring default `[core + 3, core + 9]` |
| `core` | doc px | core radius for `ring` |
| `prefix` | identifier | prefix for the new field names (`dog` → `dogMax`, `dogMeanR`) |
| `space` | space | working space (linear) |

Field names are camelCase: `prefix + Stat + Channel` (`max`, `dogMax`, `meanR`). A field that already exists
(measuring twice without a prefix) is `E_ARG_TYPE` with a fix that adds a prefix. Regions and image must share a
frame (GCV-001 frames); the region pixels are rasterized at the image's analysis scale with the area-exact
`Rasterize` (`regions.go:194-240`). `chroma` exists only with `ring`, so it always means "neutrality of the
spot's contrast", the cue in the playbook.

`components` stops adding `meanMask`; `components().measure(mask, { stats: ["mean"] })` gives `mean`.

### 7.5 One schema

```go
// pkg/cv/graph/schema.go
type FieldSpec struct {
    Name string
    Type string // "number" | "string"
    Unit Unit   // DocPx (x, y, w, h, cx, cy), DocPx2 (area), Level (core, step, max …), Unitless (snr, chroma, aspect, score)
    Doc  string
}

// GeometryFields are on every region, in this order.
var GeometryFields = []FieldSpec{
    {Name: "id", Type: "number", Doc: "stable id within this region set"},
    {Name: "x", Type: "number", Unit: DocPx, Doc: "left of the bounding box"},
    {Name: "y", Type: "number", Unit: DocPx, Doc: "top of the bounding box"},
    {Name: "w", Type: "number", Unit: DocPx, Doc: "width of the bounding box"},
    {Name: "h", Type: "number", Unit: DocPx, Doc: "height of the bounding box"},
    {Name: "cx", Type: "number", Unit: DocPx, Doc: "centroid x"},
    {Name: "cy", Type: "number", Unit: DocPx, Doc: "centroid y"},
    {Name: "area", Type: "number", Unit: DocPx2, Doc: "area"},
    {Name: "aspect", Type: "number", Doc: "ratio of the principal axes (≥ 1)"},
}

// OpDef gains, for ops returning Regions:
//   RegionFields func(args Args, inputs []*Node) []FieldSpec  // the result's schema; nil = the receiver's
```

- `Node.Fields []FieldSpec` is computed at intern time: `components` → geometry; `detect.specks` → geometry +
  `contrast, localNoise, sharpness, score, meanL` (number) + `polarity` (string); `regions.measure` → receiver's +
  the requested fields; `where`, `sortBy`, `top`, `without`, `only`, `filter` → receiver's.
- `RegionsValue` carries the same schema; `Region.Data()` emits fields in schema order, flat:
  `{ id, x, y, w, h, cx, cy, area, aspect, contrast, …, polarity }` (geometry rounded to 0.01 doc px, measures to
  4 decimals, as today).
- `where` and `sortBy` validate against the receiver node's schema at build time, so
  `components().where({ score: … })` fails immediately with "these regions have id, x, y, w, h, cx, cy, area,
  aspect" and, when another op produces `score`, a hint naming it ("score comes from detect.specks").
- `cv.help("Regions")`, `cv.help("detect.specks")` and the data-lambda hint print fields from schemas. The
  global `KnownRegionFields` is deleted; the "which op produces this field" index for hints is built from the
  registry by evaluating each op's `RegionFields` with default arguments.

The names `x, y, w, h, cx, cy` win over `box` and `centroid` because filters need scalars:
`where({ cx: { gt: 100 } })` has no array equivalent.

**Agents that write `item.box`.** Agents trained on the old docs will read `r.box` and get `undefined`. The
prelude's `link()` (`pkg/gojamod/cv/prelude.js:54-79`) already attaches non-enumerable helpers to awaited data;
for Regions items it adds non-enumerable accessors `box` and `centroid` that throw `E_NO_FIELD` with the literal
fix (`[r.x, r.y, r.w, r.h]` or `[r.cx, r.cy]`, or "pass the item itself: mask.box(r)"). These accessors return no
data, so they are a diagnostic, not an alias.

### 7.6 `box` and `point` argument types

The binder (`pkg/cv/graph/args.go`) gains two types:

| type | accepts | canonical form |
|---|---|---|
| `box` | `[x, y, w, h]`; an object with numeric `x, y, w, h` (a region item) | `[]any{x, y, w, h}` |
| `point` | `[x, y]`; an object with numeric `cx, cy` (a region item) | `[]any{x, y}` |
| `point[]` | a list of points; a Regions node or awaited Regions (centroids) | list, or a lifted Regions input |

Objects are converted at bind time, so the hash and `explain()` see the numbers. An object with `x, y` but no
`cx, cy` is refused as a point (`E_ARG_TYPE`, "a region's x, y is its box corner; points use cx, cy"), because
reading a region's box corner as its centre is the mistake to prevent. Every op with a box or point switches:
`stats.box`, `mask.box`, `mask.ellipse`, `mask.flood`, `mask.gradient` (points), `crop`, `annotate`, `sheet`.

### 7.7 No item cap

`RegionsValue.Summary` returns every item (`{ kind, count, items, scale, frame }`, no `truncated`). A hard
safety limit, `MaxRegionItems = 100000`, turns pathological masks (every pixel a region) into
`E_TOO_MANY: 412 337 regions; await at most 100 000. fix: regions.sortBy("area").top(1000)` instead of a
multi-gigabyte JavaScript array. The YAML renderer's caps stay: they protect the model's context, and their
truncation notes gain a pointer to data out (`… 152 more (return cv.json(x) for all)`; the CUT note says the
same).

### 7.8 Charts

`look()` of a Value node renders a chart when its op declares one:

```go
// OpDef gains:
//   Chart ChartKind // ChartNone, ChartBars (histogram: counts), ChartLines (profile: t → l, r, g, b)
```

`look.New` (`look.go:87-92`) checks `op.Chart` instead of the op name. `chart.go` gains a line renderer; both
use `pkg/cv/text` for axis labels. Charts stay PNG.

### 7.9 JSON and CSV content

```js
return {
  summary: { spots: await hot.count() },
  all: cv.csv(ranked, { name: "spots" }),            // a Regions node: every item, schema order
  raw: cv.json(await img.measure(pts), { name: "probe" }),
}
```

```yaml
summary:
  spots: 213
all: "[content 1: csv spots.csv (213 rows × 21 columns)]"
raw: "[content 2: json probe.json (40 items)]"
```

```go
// pkg/cv/dataout
type Content struct {
    name string
    mime string            // "application/json" | "text/csv"
    node graph.NodeID      // lazy: evaluated at render time, off the owner goroutine
    data any               // eager: plain data converted on the owner goroutine at the call
    eng  *graph.Engine
}

func (c *Content) ResultPlaceholder() string                                  // "json probe.json (40 items)"
func (c *Content) ResultContent(ctx context.Context) ([]resultcontent.Content, error)
```

- `cv.json(x, { name })`: `x` is plain data, awaited data (its link to the node is used, so it renders lazily), or a
  Value/Regions node. Image nodes are `E_WRONG_KIND` with the fix `node.save("x.tif")`.
- `cv.csv(x, { name, columns })`: `x` is a Regions node (rows = items, columns = schema order) or a list of
  objects (columns = keys in first-seen order). `columns` selects and orders. Nested arrays and objects are
  JSON-encoded in their cell.
- Both return Go objects, not nodes, so they cannot be ops (an op returns a node of a kind). They are registered
  as **module functions**: `Registry.RegisterFunc(graph.FuncDef{Name, Args []ArgSpec, Doc, See})`, a small
  table next to the ops. The binding implements them; the registry supplies help, option checking, synonyms and
  did-you-mean through `graph.BindOptions` (GCV-002 §6, item 4). The remaining hand-written `cvFuncs` entries
  (`evaluate`, `describe`, `docs`, `help`, `isNode`, `warnings`) move into the same table, which ends the
  hand-maintained help table (`pkg/gojamod/cv/help.go:24-43`).
- Delivery: MCP sends `application/json` and `text/csv` as text content (`toMCPContent`); the CLI writes them as
  `name.json` / `name.csv` in its `--out` directory through `resultcontent.WriteFiles`.

**Limits.** Looks and data are different budgets. The session enforces `MaxLooks` (4) itself in
`state.collect` (`pkg/cvsession/processor.go:116-140`), dropping later looks with a note, and sets the transport's
item limit to `MaxLooks + MaxData` (4 + 4). Data items larger than `MaxDataBytes` (256 KiB) are dropped from MCP
delivery with the note "too large for the conversation: save it with .save(\"probe.json\")"; the CLI writes them
regardless. go-go-goja's `replmcp` limits are configured from goja-cv (`mcp.go:61`); no upstream change is needed.

### 7.10 Saving under an out-root

```js
const r = await img.view("speck").save("views/tile-17-speck.tif", { depth: 16 })
// { path: "/abs/out/views/tile-17-speck.tif", format: "tiff", depth: 16, encoding: "linear", w: 1024, h: 1024, bytes: 6291912 }
const d = await cv.csv(ranked).save("spots/tile-17.csv")
```

| method | on | options |
|---|---|---|
| `node.save(path, opts)` | Image and Mask nodes | `depth` (8, 16, 32), `format` (from the extension: `.png`, `.tif`/`.tiff`, `.npy`, `.jpg`) |
| `content.save(path)` | the objects `cv.json` / `cv.csv` return | — |

Both return a promise of `{ path, format, bytes, … }`; an unawaited save renders as
`[Promise] (not awaited: add await)` like any promise (`pkg/cvsession/yaml.go:109-110`).

- Paths resolve against the out-root (never the working directory). After symlink resolution the target must be
  inside the out-root (the same containment check as `checkPath`, `native.go:72-84`); parent directories are
  created; existing files are replaced and the result says `replaced: true`.
- Without `--out-root` every save is `E_ACCESS: saving is disabled in this session` with the hint "start
  goja-cv with --out-root DIR".
- Images are evaluated at scale 1 and written as **encoded** values with the image's encoding (GCV-001). The
  TIFF writer records `goja-cv encoding=linear origin=1536,1536` in `ImageDescription` (270), and GCV-001's
  TIFF decoder reads that description back, so a saved derived view reopens with the same encoding and frame.
  PNG records `gAMA` = 1.0 for linear images and an `sRGB` chunk for sRGB ones; the PNG decoder honours both.
- Values outside [0, 1] (signed derived images) are clamped by integer formats; the save then warns `CLAMP`
  with the clipped fraction and the fix `.save("x.tif", { depth: 32 })` (float TIFF) or `.npy`.
- `--out-root` is a flag of `eval`, `run` and `mcp`, distinct from `--root` (reads), and `cvsession.Options`
  gains `OutRoot string`.

### 7.11 Preludes

```bash
goja-cv eval --image tile.tif --prelude scripts/84-cv-helpers.js -e 'return await mem.probe(1337, 131, 3)'
goja-cv mcp --image tile.tif --root . --prelude helpers.js
```

- `--prelude FILE` (repeatable) on `eval`, `run` and `mcp`. `cvsession.Options.Preludes []string`.
- `App.CreateSession` evaluates each file, in order, with the same async-body semantics as agent code (so a
  prelude may `await` and `return`), before the session is handed out. MCP sessions created later run them too.
- A prelude that fails aborts session creation with its file name and line (`prelude helpers.js:12: …`), using the
  same structured error rendering as agent code. Prelude results are not printed; `mem` keeps what they define.
- Preludes count toward the context block's `mem` listing like anything else.

## 8. Decision records

### Decision: flat scalar fields, one schema per node

- **Context:** Items say `centroid`, filters say `cx` (issue 17). The colleague proposed accepting both.
- **Options considered:** (a) aliases both ways; (b) arrays (`box`, `centroid`) everywhere with array-aware
  filters; (c) flat scalars everywhere, with box/point arguments accepting region objects.
- **Decision:** (c).
- **Rationale:** Aliases double every list and every error message and are excluded by the project's rules.
  Filters and sorting need scalars. Accepting a region item as a box or point removes the main reason arrays were
  convenient (`mask.box(r)`, `crop(r)`, `sample(regions)`).
- **Consequences:** Every example that reads `box`/`centroid` changes (§11); a throwing accessor turns old habits
  into literal fixes.
- **Status:** proposed

### Decision: no item cap; caps belong to presentation

- **Context:** The 100-item cap truncates data inside JavaScript (issue 16).
- **Options considered:** (a) `items({ all: true })`; (b) raise the cap; (c) no cap in data, a safety limit, and
  presentation caps in the YAML renderer.
- **Decision:** (c).
- **Rationale:** A cap on data is a policy disguised as an API: code that iterates items cannot know it saw a
  prefix. The model's context is protected where text is produced. (a) adds a method whose only purpose is to undo
  a limit.
- **Consequences:** Memory per awaited Regions grows with the count; the safety limit bounds it.
- **Status:** proposed

### Decision: measurements are Value ops with a shared probe kernel

- **Context:** `sample`, `profile`, `measure` and `regions.measure` all read small neighbourhoods.
- **Options considered:** (a) JavaScript helpers over `stats` (status quo); (b) each op its own kernel; (c) one
  probe kernel with coverage-weighted disk/annulus sums and weighted quantiles.
- **Decision:** (c).
- **Rationale:** (a) costs a whole-image pass per call. (c) gives identical numbers across the ops (the same
  disk means the same thing in `sample` and `measure`) and lets the test suite check them against `stats` with
  masks once.
- **Consequences:** `probe.go` must match `StatsWeighted` exactly for binary weights; tests enforce it.
- **Status:** proposed

### Decision: data out as result content; files only under an out-root

- **Context:** Issue 16 suggests `--json-out FILE` or `cv.export(value)`; issue 21 asks for `node.save(path)`.
- **Options considered:** (a) a CLI flag that writes the whole result as JSON; (b) content providers for data plus
  explicit saves under a write root; (c) unrestricted file writes.
- **Decision:** (b).
- **Rationale:** Content providers reuse the mechanism looks already use; the CLI gets files and MCP gets text
  without new transport code. Separate read and write roots keep an agent that may read scans from writing next
  to them. (a) cannot mix data with text and looks in one result.
- **Consequences:** `--out-root` is required for saving; data larger than 256 KiB reaches MCP clients only as a
  saved file.
- **Status:** proposed

### Decision: saved files carry their encoding and frame

- **Context:** A saved linear derived view reopened as sRGB would be decoded with the wrong curve.
- **Options considered:** (a) always write sRGB; (b) write encoded values and record the encoding in standard
  places (TIFF description, PNG gAMA/sRGB), read back by GCV-001's decoders; (c) sidecar JSON files.
- **Decision:** (b).
- **Rationale:** (b) round-trips inside goja-cv and is visible to other tools (gAMA is standard; the TIFF
  description is readable). (c) separates data from its meaning.
- **Consequences:** GCV-001's decoders gain two small checks (TIFF description, PNG gAMA/sRGB chunks).
- **Status:** proposed

### Decision: preludes run at session creation through the eval path

- **Context:** Issue 23b wants helpers in every CLI run.
- **Options considered:** (a) run prelude files in the runtime initializer (synchronous); (b) evaluate them as
  agent code after the session is created.
- **Decision:** (b).
- **Rationale:** Helper files like `84-cv-helpers.js` are async-function bodies (`await`, `return "helpers
  ready"`). (b) gives them the same semantics and error rendering as typed code.
- **Consequences:** Session creation can fail with a script error; MCP reports it in the tool result.
- **Status:** proposed

## 9. Pseudocode and key flows

### 9.1 The probe kernel

```
probe(buf, frame, scale, centre (doc px), core r, ring [r1, r2], percentiles):
  map centre to buffer px: (cx − frame.X) · scale, (cy − frame.Y) · scale
  window := square of side 2·r2·scale + 2 around it, clipped to the buffer
  for each pixel in window:
      d := distance from the pixel's centre to the centre (buffer px)
      wc := coverage of the pixel by the core disk (exact for ≤ 1 px from the edge, 0/1 otherwise)
      wr := coverage by the annulus = coverage(r2) − coverage(r1)
      accumulate weighted sums per channel and luminance; keep (value, wr) pairs for ring quantiles
  core mean, ring mean, ring p10/p90 (weighted quantile, the rule of kernels.weightedQuantile)
```

### 9.2 `regions.measure`

```
Run(regions, image, opts):
  check frames; img := image at analysisScale (1)
  out := regions.With(copy of items with copied measure maps)     // items are shared and immutable
  if opts.stats: m := Rasterize(regions at img size); per region: weighted stats over its pixels (one pass per region window)
  if opts.ring:  per region: probe(img, centroid, core(region), ring) → core, ring, step, spread, snr, steps, chroma
  name fields with prefix; append FieldSpecs to the schema
  return out
```

### 9.3 Delivering a CSV over MCP and the CLI

```
eval returns { all: cv.csv(ranked) }
  processor: walk value → providers [csv content]; YAML shows "[content 1: csv spots.csv (213 rows × 21 columns)]"
  render off the owner goroutine: evaluate ranked → RegionsValue → rows in schema order → CSV bytes
  MCP: text/csv → TextContent if ≤ 256 KiB, else dropped with the save hint
  CLI: WriteFiles(out, "spots.csv"); prints files: { "content 1 csv spots.csv": /abs/out/spots.csv }
```

## 10. Errors and diagnostics

| situation | code | message and fix |
|---|---|---|
| unknown field in `where`/`sortBy` | `E_UNKNOWN_OPTION` | lists this node's fields; names the op that produces the field when one does |
| reading `item.box`, `item.centroid` | `E_NO_FIELD` | `[r.x, r.y, r.w, r.h]`, `[r.cx, r.cy]`, or pass the item |
| point given as `{ x, y }` | `E_ARG_TYPE` | "x, y is a box corner; use [r.cx, r.cy] or the item" |
| measuring a field twice | `E_ARG_TYPE` | add `prefix` |
| profile too long | `E_ARG_RANGE` | larger `step` |
| ring radii wrong (`r1 ≤ core` or `r2 ≤ r1`) | `E_ARG_RANGE` | default ring |
| too many regions | `E_TOO_MANY` | `sortBy(…).top(n)` |
| save without out-root, or outside it | `E_ACCESS` | `--out-root DIR`; a path inside |
| save of a Value/Regions node | `E_WRONG_KIND` | `cv.json(x).save("x.json")` |
| `cv.json(imageNode)` | `E_WRONG_KIND` | `node.save("x.tif")` |
| integer format clamps signed values | `CLAMP` warning | `{ depth: 32 }` or `.npy` |
| prelude fails | session error | `prelude FILE:LINE: …` |

Synonyms: ops `probe`, `ring`, `contrastAt` → `measure`; `pick`, `valueAt`, `pixel`, `at` → `sample` (`pick` is a
Value op; the synonym applies on Image only); `lineProfile`, `line`, `trace` → `profile`; `export`, `write`,
`saveAs` → `save`; `toJSON`, `json` → `cv.json`; `toCSV`, `table` → `cv.csv`. Options: `radius` → `r` (sample),
`core` (measure); `annulus`, `background` → `ring`; `thickness` → `width`; `fields` → `columns`; `bits` →
`depth`.

## 11. Documentation and examples that change

- New topic `pkg/cv/docs/topics/12-measuring.md` (id `measuring`): sample, profile (with a plot), core/ring with
  the worked example of §3.3, `regions.measure` with the hot-spot recipe, and which space numbers are in.
- New topic `13-data-out.md` (id `data-out`): `cv.json`, `cv.csv`, `.save`, `--out-root`, `--prelude`, the
  difference between YAML (for you) and data content (for programs).
- `05-masks-and-regions.md`: items are `{ id, x, y, w, h, cx, cy, area, aspect, … }`; the example at lines 31-41
  changes from `box`/`centroid` to the flat fields; "pass the item" for boxes.
- `09-recipes.md`: "Measure inside a box or region" uses `measure`/`sample`; "Rank candidates" (the hot list).
- `03-nodes-and-evaluation.md`: awaited Regions have every item.
- Guidebook `pkg/doc/03-cli.md`: `--prelude`, `--out-root`, data files in `--out`; `08-recipes.md` likewise.
- MCP instructions: one line for `measure`/`sample`, one for data out; the Regions line lists the flat fields.
- Hand-over to the colleague: `mem.probe` → `img.measure`; `mem.hot` → `components().measure(...)`; script 83 can
  call `goja-cv eval --out DIR -e 'return cv.csv(img.detect.specks({ maxArea: 2000 }))'` and read the CSV.

Tests and examples that change: every test reading `box`/`centroid` from items (`pkg/cv/ops/regions_test.go`,
`detect_test.go`, `pkg/gojamod/cv/module_test.go`, `m3_test.go`, cvsession tests, README); `truncated`
assertions; `meanMask` tests; the CLI test that checks look file names (`cmd/goja-cv/main_test.go`).

## 12. Implementation phases

**Phase 1 — schema and argument types.**
1. Tests first: every Regions-producing op's items have exactly its schema's names (property over ops); `where`
   on a field from another op fails at build time with the producer hint; `box`/`point` accept arrays and region
   items and refuse `{ x, y }` as a point; the throwing `box`/`centroid` accessors.
2. `schema.go`, `Node.Fields`, `OpDef.RegionFields`; flat `Region.Data`; delete the four lists; `box`/`point`
   types; switch every op; data-lambda hint from the schema.

**Phase 2 — no cap.**
1. Tests: 5000 regions all awaited; `E_TOO_MANY` above the limit; YAML truncation note mentions `cv.json`.
2. Remove `SummaryLimit`/`truncated`; add the safety limit.

**Phase 3 — probe kernel and measurement ops.**
1. Tests first: `probe` disk means equal `StatsWeighted` with ellipse masks for binary and soft edges;
   `measure` on the planted specks matches the colleague's `mem.probe` (pasted into the test) under the
   coordinate and luminance mapping of §13.2; `sample` r = 0 equals the pixel; profile across a
   synthetic edge of known σ recovers it (fit within 10 %); spaces respected.
2. `kernels/probe.go`; `sample`, `profile`, `measure`; merge `StatsWeighted`.

**Phase 4 — regions.measure.**
1. Tests: stats fields equal `stats({ mask: regions.only([id]).toMask() })` per region; ring fields equal
   `measure` at centroids; prefixes; duplicate-field error; frames.
2. The op; remove `meanMask` from `components`.

**Phase 5 — charts.**
1. Tests: profile look is a PNG line chart of the right size; histogram unchanged.
2. `OpDef.Chart`, line renderer.

**Phase 6 — data out and saving.**
1. Tests: `cv.json`/`cv.csv` placeholders and bytes; lazy node rendering; MCP text delivery and the size note;
   CLI files with `.json`/`.csv` names through `WriteFiles`; look and data budgets; save containment (a symlink
   pointing outside the out-root is refused); save → open round trips for PNG 8/16, TIFF 16/32, NPY, including
   encoding and frame metadata; `CLAMP` warning.
2. `pkg/cv/dataout`; `cv.json`, `cv.csv`, `.save`; `OutRoot`; delete `writeLooks`.

**Phase 7 — preludes.**
1. Tests: a prelude defining `mem.f` is usable in the eval; a failing prelude reports `FILE:LINE`; MCP sessions
   created later run preludes.
2. `Options.Preludes`, flags on `eval`, `run`, `mcp`.

**Phase 8 — documentation and agent check.** Topics, guidebook, instructions, synonyms (§10, §11); the gated
and harness checks (§13).

## 13. Test strategy

### 13.1 Unit and property tests

- Probe correctness against `StatsWeighted` (exact for binary coverage, within 1e-6 for soft edges).
- Schema property: for every op that returns Regions, `Object.keys(item)` equals the schema names, and every
  schema field is accepted by `where` and `sortBy`.
- Box/point coercion table tests.
- CSV: quoting, nested values, column selection; JSON: round trip through `encoding/json`.
- Save: containment, formats, metadata round trip.

### 13.2 Equivalence with the colleague's helper

The strongest test for `measure` is that it returns what the agents already trust. Copy `mem.probe` from
`scripts/84-cv-helpers.js` into `pkg/cvsession/testdata/probe.js`, run it and `img.measure` on the same points
of the planted scan in one session, and compare. Two details of the helper must be mapped, and the guide's
`measure` Doc string should state the mapping so agents can translate the playbook's numbers:

- Coordinates. The helper's core is the ellipse inscribed in the box `[x − r, y − r, 2r + 1, 2r + 1]`, a disk
  of radius r + 0.5 centred on (x + 0.5, y + 0.5), i.e. on the centre of pixel (x, y); its ring radii are
  r + 3.5 and r + 9.5. So `mem.probe(x, y, r)` corresponds to
  `img.measure([[x + 0.5, y + 0.5]], { core: r + 0.5, ring: [r + 3.5, r + 9.5] })`.
- Luminance. The helper's `step` averages the three channel steps (equal weights) while its `spread` uses
  `stats()` luminance percentiles (Rec. 709 weights). `measure` uses one gray plane per call. With
  `gray: "average"`, `step`, `stepR/G/B` and `chroma` must equal the helper's within 1e-6; with the default
  `gray: "rec709"`, `spread` must.

Any other difference is a bug in one of the two.

### 13.3 Documentation tests

The `measuring` and `data-out` topics' ```js blocks run against the planted scan. Saving needs an out-root: the
docs test harness creates a temporary out-root and passes it in `Options`.

### 13.4 Real tiles (gated)

On `GOJACV_TILES` with the gold points file from GCV-002's export script: build the hot list with the speck view
and `components`, measure it with `ring: true`, and assert that labelled dust has median `snr` ≥ 1.5 and that the
fraction of gold points within 4 px of a top-30 spot is ≥ 70 % (the colleague's `mem.hot(2)` covered 77 %).
Separately, time the whole hot-list measurement on a 1024² tile: it must take under 200 ms (today's helper takes
56 whole-image stats).

### 13.5 Agent harness

`questions-gcv-003.md` for the pi harness on one tile:

1. Rank the 20 strongest candidate spots by how much they stand out, with snr and chroma.
2. Give the full candidate table as CSV.
3. Save the view you used as a 16-bit TIFF and say where it is.

Run before (expect the probe helper or many stats calls, and YAML paging) and after; record calls, errors, and
whether the CSV row count equals the region count.

## 14. Risks, alternatives and open questions

**Risks.**

- *Breaking every region example.* The flat schema touches docs, tests, README and the colleague's helpers. The
  throwing accessors and the docs tests catch what greps miss.
- *Large data in the conversation.* JSON delivered as MCP text enters the model's context. The 256 KiB data
  limit and the save hint keep large tables out; the docs say to return summaries to the model and data to files.
- *Write access.* An out-root is a new capability. Containment checks follow the read-root code and are tested
  with symlinks.

**Alternatives not taken.** A `Table` kind for tabular results (Regions already are a table, and Value rows serve
the rest); writing whole results as JSON from the CLI (`--json` already prints the result text in JSON form);
storing preludes in a library (the CVJS-001 `lib` design, not yet built).

**Open questions.**

1. Should `sample` accept `space: "all"` and return all three spaces? Useful for teaching, noisy for routine use.
2. Should `regions.measure` support `ring` on region shapes (dilated region minus region) instead of disks around
   the centroid? Better for elongated hairs; the disk version matches the playbook's numbers.
3. Should data content default to `save` when the session has an out-root and the item exceeds the MCP limit,
   instead of dropping with a hint?

## 15. References

- Handover issues 9, 10, 16, 17, 21, 23: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md`; feature requests items 4, 9, 10, 11: `…/reference/05-goja-cv-feature-requests.md`; generic list C1–C4b, D3, D4, E1, E4: `…/reference/06-goja-cv-generic-inspection-features.md`.
- Helpers and calibration: `…/DUST-002…/scripts/84-cv-helpers.js` (`mem.probe`, `mem.hot`), `…/scripts/83-cv-specks-vs-gold.py`; playbook section 4 (numbers).
- go-go-goja result content and MCP mapping: `resultcontent/resultcontent.go`, `resultcontent/files.go`, `replmcp/tools.go` (module cache, version in `go.mod`).
- CVJS-001 intern guide Part 2 §5 (masks vs regions, identity regions) and the integration-surfaces doc §3 (result content): `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/`.
- GCV-001 (spaces, Level unit, frames, encodings and decoders that read saved metadata) and GCV-002 (module ops, text font, charts renderer, sheet and annotate accepting region items): the `design-doc/01-design-and-implementation-guide.md` of each ticket.
