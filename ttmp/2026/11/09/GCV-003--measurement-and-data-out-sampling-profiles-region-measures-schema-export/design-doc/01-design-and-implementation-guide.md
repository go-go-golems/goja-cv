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
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md
      Note: shared foundations chapter (read first)
    - Path: ttmp/2026/11/09/GCV-003--measurement-and-data-out-sampling-profiles-region-measures-schema-export/reference/01-review-read-findings.md
      Note: review findings and their disposition (revision 2)
    - Path: pkg/cv/graph/regions.go
      Note: Region.Field vs Region.Data; RegionFields, KnownRegionFields; SummaryLimit 100; Rasterize
    - Path: pkg/cv/graph/registry.go
      Note: OpDef.Fields []string and Validate(args) become FieldSpec schemas and Validate(args, inputs)
    - Path: pkg/cv/ops/regions.go
      Note: where/sortBy validate against the global KnownRegionFields; components adds meanMask
    - Path: pkg/cv/ops/detect.go
      Note: speckFields list (box, centroid) used for help
    - Path: pkg/cv/kernels/mask.go
      Note: EllipseMask 4x4 supersampling; becomes EllipseCoverage shared with the probe kernel
    - Path: pkg/cv/kernels/measure_weighted.go
      Note: StatsWeighted wrapper
    - Path: pkg/cv/buffer/io.go
      Note: existing PNG/JPEG/TIFF16 writers, replaced by writers that record encoding and frame
    - Path: pkg/cvsession/session.go
      Note: ContentLimits, Options gains MaxData, MaxDataBytes, OutRoot, Preludes
    - Path: cmd/goja-cv/eval.go
      Note: writeLooks naming and --json output; gains --out-root and --prelude
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/84-cv-helpers.js
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/83-cv-specks-vs-gold.py
    - /Users/manuel/go/pkg/mod/github.com/go-go-golems/go-go-goja@v0.10.8-0.20261109062017-5e1ba117462e/pkg/resultcontent/files.go
    - /Users/manuel/go/pkg/mod/github.com/go-go-golems/go-go-goja@v0.10.8-0.20261109062017-5e1ba117462e/pkg/replmcp/tools.go
Summary: "Intern guide for GCV-003: point sampling, line profiles, core-versus-ring measures and regions.measure on one probe kernel; one field schema for Regions and Value results with one naming rule; no item cap; JSON/CSV result content with named files; saving with encoding and frame metadata under an out-root; CLI preludes."
LastUpdated: 2026-11-09T23:30:00-05:00
WhatFor: "Implementing GCV-003 (after GCV-001 and GCV-002)."
WhenToUse: "Read before touching measurement ops, Regions fields, awaited summaries, result content, file writers or the CLI."
---

# GCV-003 — Measurement and data out: sampling, profiles, region measures, schema, export

## Revision history

| date | revision | what changed |
|---|---|---|
| 2026-11-09 | 1 | First version (commit b30efec). |
| 2026-11-09 | 2 | Revised from the review read (`reference/01-review-read-findings.md`, disposition table at its end). Foundations in the shared chapter. One naming rule (`x, y` corner, `cx, cy` point, `meanL`, radius options that cannot be read as fields, `stepChroma`). Coverage is the 4 × 4 supersampled rule of `mask.ellipse`, shared through one function. One schema mechanism for Regions and Value results; `Validate` sees inputs. Point ops take points only; one way to ring-measure regions. Named data files written by goja-cv; `--json` lists files. Writers for TIFF (uint and float), PNG chunks and NPY with an honest round-trip table. Content budgets, evaluation-time warnings, edge rule, directories table, and new decision records. |
| 2026-11-09 | 3 | Revised from the re-check (rows P2-14, P2-15 and P3-18 to P3-23 in the disposition). Data names are identifiers with a default; `mem.hot`'s 0–1 px centring offset stated; the MCP data limit runs inside the Data handle; `EllipseCoverage` handles non-circular ellipses; "within 1e-9" instead of "exactly"; masks saved with the linear note; `internal/testimage` switches to the production writers; default depths; the prelude does not marshal `data` arguments; the CLI counts rows. |

## How to read this guide

This guide is for an intern who will implement GCV-003 in goja-cv. Read first:

- the shared foundations chapter,
  `ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md`,
  especially §F3 (registry, binder, lifting), §F4 (evaluation, `AnalysisScale`, awaiting), §F6 (masks, regions, region
  items), §F8 (session, `mem`, context block, roots, CLI) and §F9 (looks and the result-content pipeline);
- GCV-001 §3.3 (spaces and levels), §3.7 (frames and absolute coordinates), §7.4 (`box` and `point` argument types and
  the rule "`x, y` is a corner, `cx, cy` a centre"), §7.5 (space resolution) and §7.6 (the `Level` unit);
- GCV-002 §3.7 (the DoG view the colleague ranks spots with), §7.10 (ops versus registered functions, `FuncDef`),
  §7.12 (`pkg/cv/text`) and §7.13 (warnings at evaluation time).

**Dependencies.** GCV-003 is implemented after GCV-001 and GCV-002 and uses the pieces listed above.

## 1. Executive summary

goja-cv measures well inside a mask or box (`stats`), but an agent cannot ask "what is the value here", "what does
the line from A to B look like", or "how much does this spot stand out from its surroundings" without building masks
and calling `stats` twice per point. The colleague wrote a JavaScript helper for the last question (`mem.probe` in
`scripts/84-cv-helpers.js`), and it became the main number their agents use. Regions cannot be measured on another
image, so their where-to-zoom list is probed point by point. Regions also expose their geometry under two sets of
names (`cx` in `where()`, `centroid` in items), awaited region lists stop at 100 items, and data leaves an eval only
as YAML text capped at 9000 characters.

This ticket adds three measurement ops (`sample`, `profile`, `measure`) and `regions.measure(image)`, all on one
probe kernel; one field schema for Regions and Value results, with one naming rule; awaited Regions with every item;
JSON and CSV result content (`cv.json`, `cv.csv`) written to named files by the CLI; saving images and data under a
separate `--out-root` with their encoding and frame recorded; and a CLI `--prelude`. It removes the duplicate field
lists, the 100-item limit, the `meanMask` special case, `number[]` boxes and points, and the CLI's look-only file
writer.

## 2. Problem statement and scope

### 2.1 The workflow

The colleague's agents decide by eye on magnified crops (GCV-002) and back the decision with numbers. The playbook's
decision table (DUST-002 `reference/04-goja-cv-dust-playbook.md`, section 4) uses two numbers per spot:

| | real dust (median) | non-dust (middle half) |
|---|---|---|
| snr = step / ring spread | 1.7 – 4.0 | 0.3 – 1.8 |
| chroma (colour film) | 0.2 – 0.6 | 0.4 – 0.9 |

Both come from `mem.probe(x, y, r)`: the mean of a disk of radius r, the mean and the 10–90 % spread of a ring from r
+ 3 to r + 9 px, and the per-channel steps. Their where-to-zoom list is `mem.hot(k)`: the connected blobs of the DoG
view (GCV-002 §3.7) above k × its 99th percentile, each probed. Calibration against human labels
(`scripts/83-cv-specks-vs-gold.py`) runs `goja-cv eval --json` from Python and needs every region of a tile.

### 2.2 The issues

| issue | title | priority |
|---|---|---|
| 9 | Point sampling, line profiles and ring measures | P2 |
| 10 | Measure regions against another image | P2 |
| 16 | Bulk data out | P2 |
| 17 | Consistent field names between `where()` and items | P2 |
| 21 | Save derived images | P3 |
| 23b | CLI prelude | P3 |

Out of scope: `detect.specks` scoring and area (issues 11 and 12; detector work for a later ticket, which `measure` will
help evaluate), and the document layer.

## 3. Concepts

### 3.1 Sampling a point, and coverage

Coordinates are continuous doc px (Foundations §F5): pixel i covers [i, i + 1), so the centre of pixel (1337, 131) is
(1337.5, 131.5). **Sampling** a point reads the image near a coordinate. With radius 0 it reads the pixel that contains
the point. With radius r > 0 it averages a disk, weighting each pixel by its **coverage**: the fraction of the pixel
inside the disk.

goja-cv already computes disk coverage in `mask.ellipse`: it tests a 4 × 4 grid of sub-pixel points and counts those
inside, so weights are multiples of 1/16 (`EllipseMask`, `pkg/cv/kernels/mask.go:173-200`). This ticket extracts that
rule into one function, `kernels.EllipseCoverage(px, py, cx, cy, rx, ry float64) float64`, used by `EllipseMask`
(whose boxes may give rx ≠ ry, `mask.go:178`) and by every measurement, which passes rx = ry = r. Keeping the 4 × 4 rule, rather than switching to exact area, keeps measurements identical to what the
colleague's helper computed with `mask.ellipse` and what the playbook's thresholds were calibrated on. A disk of radius
2 then has a total weight close to π · 2² ≈ 12.6 doc px².

### 3.2 Profiles

A **profile** is the sequence of values along a straight line from A to B, sampled at a fixed step (1 doc px by
default). With a width w > 1, each sample averages w values across the line, which suppresses grain without blurring
along it. The shape of a profile across an edge separates a soft transition (dust blurred by the scanner optics, edge
σ ≈ 1.6 px in the colleague's data) from a sharp one (a grain boundary, a scene edge).

### 3.3 Core and ring

How much a spot **stands out** compares the spot with its surroundings. The **core** is a disk of radius r around the
spot; the **ring** is the annulus between radii r1 and r2 around it. Measures are taken on the **gray plane**, a
weighted mean of the channels (Rec. 709 weights by default, or equal weights with `gray: "average"`), and on each
channel:

| field | definition | meaning |
|---|---|---|
| `coreMean` | coverage-weighted mean of the gray plane in the core | the spot's level |
| `ringMean` | the same in the ring | the surroundings' level |
| `step` | `coreMean − ringMean` | how much brighter (positive) or darker the spot is |
| `spread` | p90 − p10 of the gray plane in the ring | how grainy the surroundings are |
| `snr` | `step / max(spread, 1e-3)` | the step in units of local graininess |
| `stepR`, `stepG`, `stepB` | the step of each channel | |
| `stepChroma` | (max of channel steps − min) / max(\|step\|, 1e-3) | how unequal the step is between channels; 0 for a neutral spot |

A worked example with a synthetic speck: core mean 0.61, ring mean 0.52, ring p10 0.49 and p90 0.55. Then step = 0.09,
spread = 0.06 and snr = 1.5, which lies inside the overlap of the playbook's dust and non-dust ranges (the helper's own
comment calls 1.5–2.5 "zoom", `84-cv-helpers.js:17`). With channel steps 0.10, 0.09 and 0.08, stepChroma = (0.10 −
0.08) / 0.09 = 0.22: neutral, as dust is. `stepChroma` is not GCV-001's per-pixel `chroma()` map (GCV-001 §3.8); it is
the neutrality of a spot's contrast, the colleague's cue.

Near the image edge part of the core or ring lies outside the frame. Weights then cover only pixels inside, the means
and percentiles are taken over those, and the row reports `ringCoverage`, the fraction of the ring's weight that was
inside. The playbook warns that specks within 3 px of the border are measured badly; `ringCoverage` below about 0.7
says so in numbers.

### 3.4 Measuring regions on another image

A region set made from one image can be measured on another. The where-to-zoom list is made from a mask of the DoG
view; ranking it needs the DoG's peak inside each blob, and judging neutrality needs the colour image. **Measuring
regions on an image** computes statistics over each region's pixels, or its core and ring, in that image and adds
them to the region as fields.

### 3.5 Schemas and the naming rule

A **schema** is the list of fields a result has, with types, units and meanings. Regions are a table: one row per
region, one column per field. Measurement results are tables too (one row per point). The names a filter accepts,
the names items carry, the names help prints and the names errors suggest must come from one schema, or agents learn
one set and are refused by another.

The names follow one rule across the three tickets:

| name | always means |
|---|---|
| `x, y, w, h` | a box: corner and size |
| `cx, cy` | a point or a centre |
| `meanL` | the mean of the gray plane (as in `stats().meanL` today) |
| `meanR`, `meanG`, `meanB` | the mean of one channel |
| `coreMean`, `ringMean`, `step`, `spread`, `snr`, `stepR/G/B`, `stepChroma`, `ringCoverage` | the core/ring measures of §3.3 |
| `radius`, `coreRadius`, `ringRadii` | radii, as options only; no field has these names |

### 3.6 Data out

An eval's result is text, rendered as YAML and capped for the model's context (Foundations §F8). **Data out** is a
channel for data meant for programs: JSON or CSV delivered as result content (Foundations §F9) or written to a file.
Here is the pipeline for one eval that returns a look and a CSV:

1. The eval returns `{ look: v.look("view"), all: cv.csv(regions, { name: "spots" }) }`.
2. Extract, on the owner goroutine, replaces the two Go objects by placeholders: `[content 1: look 1: view (…)]`,
   `[content 2: data spots.csv (20 columns)]`.
3. Render, off the owner goroutine, evaluates the view and the regions and produces a PNG and the CSV bytes.
4. The MCP server sends the PNG as image content and the CSV as text content; the CLI writes `look-1-view.png` and
   `spots.csv` into its `--out` directory and lists them.

**Saving** writes an image or data to a file under an **out-root**, a directory the session may write to, separate
from the directories it may read:

| flag | commands | purpose | default |
|---|---|---|---|
| `--root DIR` | `mcp` | directories `cv.open` may read | none: any path is readable |
| `--out DIR` | `eval`, `run` | where the CLI writes delivered content (looks, data) | a new temporary directory |
| `--out-root DIR` | `eval`, `run`, `mcp` | the only directory `save` may write under | none: saving is disabled |
| `--prelude FILE` | `eval`, `run`, `mcp` | scripts run in each new session before agent code | none |

`--prelude` paths are operator input: they are resolved against the working directory and are not subject to `--root`.

The key points:

- Measurements weight pixels by 4 × 4 supersampled disk coverage, the rule `mask.ellipse` already uses.
- A spot's standing-out is step, spread and snr on the gray plane, and stepChroma across channels.
- One schema per result; `x, y` is always a corner and `cx, cy` always a point.
- Text results are for the model; data content and saved files are for programs.

## 4. Current-state analysis

### 4.1 Measuring a point takes masks and two stats calls

The only measurement ops on images are `stats` (`pkg/cv/ops/measure.go:126-140`, with `mask`/`box` selections at
`:53-98`) and `histogram`. The colleague's probe builds two ellipse masks per point and calls `stats` twice
(`84-cv-helpers.js`, `mem.probe`, lines 35-47):

```js
const core = img.mask.ellipse([x - r, y - r, 2 * r + 1, 2 * r + 1])
const ring = img.mask.ellipse([x - r - 9, y - r - 9, 2 * r + 19, 2 * r + 19]).minus(img.mask.ellipse([x - r - 3, y - r - 3, 2 * r + 7, 2 * r + 7]))
const c = await img.stats({ mask: core })
const g = await img.stats({ mask: ring, percentiles: [0.1, 0.9] })
```

Each call evaluates a whole-image weighted statistic (`kernels/measure_weighted.go:27-114`), so probing 28 spots
evaluates 56 of them. `mem.hot` probes each blob sequentially with `await` in a loop (`84-cv-helpers.js:52-61`).

### 4.2 Regions carry geometry and one mask measure

`mask.components` returns regions with geometry and `meanMask`, the mean of the mask over the region
(`pkg/cv/ops/regions.go:223-263`, line 254). No op adds measures from another image.

### 4.3 Two sets of field names, five lists

`Region.Field` (`pkg/cv/graph/regions.go:111-138`) answers `id, area, x, y, w, h, cx, cy` plus measures and tags;
`Region.Data` (`regions.go:158-175`), which JavaScript receives, has `id, box, area, centroid` plus measures and tags.
So `where({ cx: { gt: 100 } })` works and `item.cx` is `undefined`; `item.centroid` works and `where({ centroid })` is
refused. The field list exists five times:

| list | file:line | contents |
|---|---|---|
| `RegionFields` | `pkg/cv/graph/regions.go:105` | `id, area, x, y, w, h, cx, cy, aspect` |
| `KnownRegionFields` | `regions.go:108-109` | the above plus every measure any op might add, hand-maintained |
| `FieldNames()` | `regions.go:141-151` | geometry plus the region's actual measures |
| `speckFields` | `pkg/cv/ops/detect.go:10` | `id, box, area, centroid, aspect, …` (the item names) |
| `OpDef.Fields []string` | `registry.go:113`, set to `RegionFields` by components (`ops/regions.go:261`) | documented fields of a result, read by the prelude through `__fieldNames` (`pkg/gojamod/cv/fields.go:9-25`) |

`where` and `sortBy` validate against the global `KnownRegionFields` (`ops/regions.go:47-56`, `:295-304`) because
`Validate` receives only the args (`registry.go:114-116`, called at `pkg/cv/graph/node.go:104-108`); so
`components().where({ score })` passes validation and fails at evaluation. The data-lambda hint lists `{ id, box,
area, centroid, score, … }` (`pkg/gojamod/cv/binding.go:74`).

### 4.4 Awaited Regions stop at 100 items

`resultToJS` returns `x.Summary(graph.SummaryLimit)` (`binding.go:339`) with `SummaryLimit = 100` (`regions.go:177-178`),
and the summary sets `truncated` (`regions.go:182-192`). JavaScript code that iterates items (`mem.hot`) silently sees
the first 100. The YAML renderer then caps arrays at 48 items and strings at 2000 characters
(`pkg/cvsession/yaml.go:20-27`), and the whole result at 9000 characters (`DefaultMaxResultChars`,
`pkg/cvsession/session.go:38-39`). To get ~200 specks the colleague pages with `where({ id: { gte, lt } })` and parses
YAML flow sequences.

### 4.5 Boxes and points are bare arrays

Box arguments are `number[]` with `Len: 4` (`stats.box`, `measure.go:134`; `mask.box`, `mask.ellipse`,
`pkg/cv/ops/mask.go:149`); points are `number[]` with `Len: 2` (`mask.flood`, `mask.go:266`). GCV-001 §7.4 introduces
`box` and `point` types; region items still cannot be passed as boxes because their fields are `box` and `centroid`.

### 4.6 Data and files out

Results come back only as YAML text; looks are the only result content. goja-cv can write images: `buffer.WriteFile`
(`pkg/cv/buffer/io.go:254-294`) writes 8/16-bit PNG, JPEG and 16-bit TIFF through the standard library and
`golang.org/x/image/tiff`, but nothing in JavaScript reaches it, the TIFF writer stores RGB as RGBA (`io.go:244-245`),
neither library writes custom TIFF tags, float samples or PNG metadata chunks, and there is no NPY writer. The CLI writes
looks with its own `writeLooks` (`cmd/goja-cv/eval.go:175-224`): looks are named `look-<n>-<slug>` from their caption,
dropped items are reported with a reason, and any other content would be named `content-<n>.bin`. Its `--json` output
lists them under `looks` (`eval.go:71-86`). go-go-goja's `resultcontent.WriteFiles` (`resultcontent/files.go:67-125`)
names files `<prefix>-<N><ext>` with extensions from `ExtensionForMIME`, but has no per-item names and skips dropped
items. The session has read roots (`session.go:56`, `checkPath` in `native.go:62-88`) and no write roots.

go-go-goja's MCP transport maps `application/json` and `text/*` content to MCP text content (`replmcp/tools.go:236-244`)
but counts every item against `MaxImages` (`tools.go:222`), which goja-cv sets to `MaxLooks` = 4
(`pkg/cvsession/mcp.go:61`); the render limit is `ContentLimits()` = 4 items and 8 MiB (`session.go:115-119`).

### 4.7 CLI sessions start empty

`goja-cv eval` and `run` create a fresh session per process (`eval.go:88-137`). `mem` helpers defined in one CLI call
are gone in the next, so the colleague re-pastes `84-cv-helpers.js` into every call.

## 5. Gap analysis

| need | today | gap |
|---|---|---|
| value at a point, many points (9) | masks + stats per point | `sample` |
| line profile, as numbers and a plot (9) | none | `profile`, line chart |
| core vs ring for many points (9) | 2 stats per point, JS helper | `measure` |
| region strength on another image, neutrality (10) | probe per centroid | `regions.measure(image)` |
| one set of field names (17) | five lists, two spellings | one schema mechanism, one naming rule |
| all items (16) | 100 | no cap |
| data for programs (16) | YAML text | `cv.json`, `cv.csv` content with named files |
| derived images to files (21) | CLI looks, 8-bit | `save` under `--out-root`, with encoding and frame |
| helpers in every CLI run (23b) | re-paste | `--prelude` |

## 6. Legacy to remove or refactor

1. Region items lose `box` and `centroid` and carry `x, y, w, h, cx, cy` like the filters; there is no alias in either
   direction.
2. `RegionFields`, `KnownRegionFields`, `speckFields`, `Region.FieldNames` and `OpDef.Fields []string` are replaced by
   one schema mechanism, `OpDef.Fields func(args, inputs) []FieldSpec`, for Regions and Value results (§7.5).
3. `meanMask` in `components` is removed; `regions.measure(mask, { stats: ["mean"] })` gives `meanL`.
4. `SummaryLimit` and the `truncated` flag are removed; awaited Regions contain every item.
5. `number[]` boxes and points are replaced everywhere by GCV-001's `box` and `point` types; this ticket switches the
   ops GCV-001 left (`mask.gradient`'s points and the measurement ops).
6. `kernels.StatsWeighted`, a wrapper around `statsWeighted` (`measure_weighted.go:27-29`), becomes one exported
   function.
7. `buffer.WriteFile`, `EncodeTIFF16` and the RGBA TIFF path are replaced by writers that record encoding and frame
   (§7.10).
8. The CLI's `writeLooks` becomes `writeContent`, which keeps its naming and dropped-item reporting and adds named data
   files (§7.9).
9. The `measure → stats` op synonym (`pkg/cv/graph/suggest.go:29`) is removed because `measure` becomes an op.
10. The data-lambda hint string listing `box, centroid` (`binding.go:74`) is generated from the schema.
11. Histogram-only charts (`pkg/cv/look/chart.go:11-40`, chosen by op name at `look.go:87-92`) become charts declared by
    the op (§7.8).
12. `EAccess` moves from `pkg/cvsession` to `pkg/cv/graph`, because `save` runs in the cv module.

## 7. Proposed architecture

### 7.1 Overview

```
 measurement ops (Value rows)          region ops (Regions)                    data out (functions)
 ────────────────────────────          ────────────────────                    ────────────────────
 img.sample(points, { radius })        mask.components()   ─┐                  cv.json(x), cv.csv(x) → Data handle
 img.profile(a, b, { width })          img.detect.specks()  ├─ schema per node  node.save(path), data.save(path)
 img.measure(points, { coreRadius })   regions.measure(img) ┘                  → out-root
        │                                       │
        └── kernels/probe.go: EllipseCoverage, coverage-weighted sums, weighted quantiles, ringCoverage ──┘
```

All measurement ops share `pkg/cv/kernels/probe.go`, which works in a small window around each point; nothing evaluates
a whole-image statistic per point. Measurements are Value or Regions ops with `Space: {Default: linear}` (GCV-001
§7.5) and `AnalysisScale` 1 (Foundations §F4), so their numbers do not depend on the preview scale.

### 7.2 `sample`

```js
await img.sample([[1337.5, 131.5], [1683.5, 123.5]], { radius: 2 })
```

```yaml
- { cx: 1337.5, cy: 131.5, meanL: 0.612, meanR: 0.640, meanG: 0.605, meanB: 0.590, area: 12.6 }
- { cx: 1683.5, cy: 123.5, meanL: 0.548, meanR: 0.561, meanG: 0.547, meanB: 0.532, area: 12.6 }
```

| argument | type | default | meaning |
|---|---|---|---|
| points (positional) | `point[]` | — | points in absolute doc px; `[x, y]` arrays or objects with `cx, cy` (region items, rows) |
| `radius` | doc px | 0 | 0: the pixel containing the point; > 0: coverage-weighted disk mean |
| `gray` | `"rec709" \| "average"` | `rec709` | weights of `meanL` |
| `space` | space | linear | GCV-001 working space |
| `analysisScale` | (0, 1] | 1 | scale the image is read at (static) |

Rows have the fields `cx, cy, meanL, meanR, meanG, meanB, area` (`meanL` only for single-channel images; `area` in doc
px² of covered weight). To centre on pixel (i, j), pass `[i + 0.5, j + 0.5]` or a region item. A point outside the
frame produces a row with `outside: true` and no values rather than an error, so one bad point does not lose a batch;
an evaluation-time warning (GCV-002 §7.13) counts such points. Passing Regions is `E_WRONG_KIND` with the fix
`regions.measure(img, { stats: ["mean"] })`: regions are measured by the Regions op, which keeps their ids (§7.4).

### 7.3 `profile`

```js
const p = img.profile([1320.5, 131.5], [1354.5, 131.5], { width: 3 })
return { values: (await p).meanL, plot: p.look("across the speck") }
```

```yaml
from: [1320.5, 131.5]
to: [1354.5, 131.5]
step: 1
width: 3
t: [0, 1, 2, …, 34]          # doc px along the line
cx: [1320.5, 1321.5, …]
cy: [131.5, 131.5, …]
meanL: [0.52, 0.52, 0.53, …]
meanR: […]
meanG: […]
meanB: […]
```

Samples are bilinear at t = 0, step, 2·step, … ≤ length, averaged over `width` positions 1 doc px apart across the
line. Samples outside the frame are `null`. At most 4096 samples (`E_ARG_RANGE` with a larger `step` as the fix). The op
declares a line chart (§7.8): `look()` of a profile renders one line per channel plus `meanL`, with `t` in doc px on
the x axis.

### 7.4 `measure` and `regions.measure`

Points:

```js
const rows = await img.measure([[1337.5, 131.5], [1683.5, 123.5]], { coreRadius: 3.5 })   // ring defaults to [coreRadius + 3, coreRadius + 9]
```

```yaml
- { cx: 1337.5, cy: 131.5, coreMean: 0.612, ringMean: 0.521, step: 0.091, spread: 0.061, snr: 1.49, stepR: 0.10, stepG: 0.09, stepB: 0.08, stepChroma: 0.22, ringCoverage: 1 }
```

| option | type | default | meaning |
|---|---|---|---|
| `coreRadius` | doc px | 3 | core disk radius |
| `ringRadii` | `[r1, r2]` doc px | `[coreRadius + 3, coreRadius + 9]` | annulus radii |
| `channels` | bool | true for colour images | add `stepR/G/B` and `stepChroma` |
| `gray` | `"rec709" \| "average"` | `rec709` | weights of the gray plane |
| `space` | space | linear | GCV-001 working space |
| `analysisScale` | (0, 1] | 1 | as for `sample` |

`img.measure` takes points only, like `sample`; Regions are `E_WRONG_KIND` with the fix `regions.measure(img, { ring:
true })`.

Regions (issue 10):

```js
const dog = img.toGray("average").dog(1, 4, { polarity: "bright" })    // the unstretched DoG: its values are comparable across spots
const hot = dog.levels({ black: 0, white: dog.stats().p99.mul(2) }).mask.luminance({ above: 0.99 }).components({ minArea: 4 })
const ranked = hot.measure(dog, { stats: ["max"], prefix: "dog" })     // adds dogMaxL
                  .measure(img, { ring: true })                         // adds coreMean … stepChroma, ringCoverage
                  .sortBy("snr").top(30)                                 // sortBy sorts largest first
return { n: await hot.count(), top: (await ranked).items.map(r => [r.id, r.cx, r.cy, r.snr, r.stepChroma].join(" ")) }
```

The unstretched DoG is measured rather than `img.view("speck")` because the preset includes a per-tile stretch, which
would make `dogMaxL` incomparable across tiles.

`regions.measure(image, opts)` returns Regions with the same items plus new fields:

| option | type | meaning |
|---|---|---|
| `stats` | list of `mean`, `max`, `min`, `std`, `median`, `p10`, `p90` | statistics over each region's own pixels, on the gray plane (suffix `L`) |
| `channels` | bool | the same statistics per channel (suffixes `R`, `G`, `B`) |
| `ring` | `true` or `[r1, r2]` doc px | core/ring measures at each centroid (§3.3) |
| `coreRadius` | doc px | core radius for `ring`; default clamp(round(√(area / π)), 2, 8) + 0.5 |
| `prefix` | identifier | prefix for the new field names (`dog` → `dogMaxL`) |
| `gray`, `space` | as for `measure` | |

Field names are `prefix + Stat + Channel` in camelCase (`meanL`, `maxR`, `dogMaxL`) and, for `ring`, the §3.3 names with
the prefix (`dogStep`). A field that already exists is `E_FIELD_EXISTS` with a fix that adds a prefix. Regions and image
must share a frame (`E_FRAME`, GCV-001 §7.10).

The default core radius follows `mem.hot` (`84-cv-helpers.js:56-58`): it probes with r = clamp(round(√(area / 3.14)),
2, 8), which by the mapping of §13.2 is a disk of radius r + 0.5. The guide's formula uses π; on integer areas in the
clamp range the two round differently only at area 95 (√(95/π) = 5.499, √(95/3.14) = 5.500).

The centre differs more. `mem.hot` passes `Math.round(centroid)` to `mem.probe`, which centres on (x + 0.5, y + 0.5),
so the helper's centre is round(cx) + 0.5: between 0 and 1 px right of and below the true centroid, 0.5 px on
average per axis. A one-pixel blob at pixel 1312 has cx = 1312.5 and is probed at 1313.5, a full pixel off with a
core radius of 2.5. `regions.measure` centres on the exact centroid, which is correct; the helper's numbers carry
that bias, which is one more reason for §13.4's re-measured ranges. `img.measure` reproduces the helper exactly only
through the mapping of §13.2, which takes the helper's integer arguments, not its rounding of centroids.

Per-region statistics need per-region weights. `RegionsValue.Rasterize` merges all regions into one mask
(`regions.go:194-240`), so it cannot be used. When the region raster and the image are the same size (both at
`AnalysisScale` 1, the usual case), each region's weights are its own pixels (`Region.Pix`, weight 1). Otherwise a helper
extracted from `Rasterize`, `Region.Coverage(target Frame, scale) (window image.Rectangle, weights []float32)`, gives each
region's area overlap in its own bounding window.

`components` stops adding `meanMask`; `components().measure(mask, { stats: ["mean"] })` gives `meanL`.

### 7.5 One schema mechanism

```go
// pkg/cv/graph/schema.go
type FieldSpec struct {
    Name string
    Type string // "number" | "string" | "number[]"
    Unit Unit   // DocPx (x, y, w, h, cx, cy), DocPx2 (area), Level (meanL, coreMean, step, …), Unitless (snr, aspect, score)
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

// RemovedFields maps names agents may still use to the literal fix.
var RemovedFields = map[string]string{
    "box":      "[r.x, r.y, r.w, r.h], or pass the item itself where a box is expected",
    "centroid": "[r.cx, r.cy], or pass the item itself where a point is expected",
    "meanMask": "regions.measure(mask, { stats: [\"mean\"] }).meanL",
}

// OpDef changes:
//   Fields   func(args Args, inputs []*Node) []FieldSpec // the result's fields (Regions items, Value rows or object); nil = the receiver's
//   Produces []string                                     // field names the op can add with some arguments (for hints)
//   Validate func(args Args, inputs []*Node) error        // now sees the input nodes
```

- `Node.Fields []FieldSpec` is computed in `internNode`, like GCV-001's other statics: `components` → geometry;
  `detect.specks` → geometry + `contrast, localNoise, sharpness, score, meanL` and `polarity` (string);
  `regions.measure` → the receiver's fields + the requested ones; `where`, `sortBy`, `top`, `without`, `only`, `filter`
  → the receiver's; `sample`, `measure`, `profile`, `stats`, `histogram` → their row or object fields.
- `RegionsValue` carries the same schema; `Region.Data()` emits fields in schema order, flat (geometry rounded to 0.01
  doc px, measures to 4 decimals, as today).
- `where` and `sortBy` validate against the receiver node's `Fields` at build time through the new `Validate` signature;
  `components().where({ score: … })` fails immediately with "these regions have id, x, y, w, h, cx, cy, area, aspect"
  and, from the `Produces` index, "score comes from detect.specks". `where({ snr })` on plain components hints
  "snr comes from regions.measure(img, { ring: true })".
- `__fieldNames` (`fields.go:9-25`) and the prelude's derived-field rule read the same specs; `cv.help("Regions")`,
  `cv.help("detect.specks")` and the data-lambda hint print fields from them.

**Old habits.** Agents trained on old docs will write `r.box`. Awaited Regions items get, on one shared prototype (not
per item), non-enumerable accessors for the `RemovedFields` names that throw `E_NO_FIELD` with the literal fix; they
return no data, so they are a diagnostic, not an alias. `where({ centroid })` and `sortBy("box")` use the same table.

### 7.6 Coordinates passed as objects

GCV-001 §7.4 defined the `box` and `point` types with the rule "`x, y` is a corner, `cx, cy` a centre". With flat items,
a region item is both a box (`x, y, w, h`) and a point (`cx, cy`), and measurement rows are points: `img.sample(rows)`,
`mask.flood(item)`, `crop(item)` and `cv.sheet(rows)` all work. This ticket switches the remaining ops: `mask.gradient`'s
`from`/`to` become `point|enum:top,bottom,left,right` (a point or an edge name), and the binder's union rule is stated
once: a union type tries its alternatives in order and reports all of them in the error. `cv.sheet`'s items are
`Regions|(point|box)[]`.

### 7.7 No item cap

`RegionsValue.Summary` returns every item (`{ kind, count, items, scale, frame }`, no `truncated`). A safety limit,
`MaxRegionItems = 100000`, turns pathological masks into `E_TOO_MANY: 412337 regions; await at most 100000. fix:
regions.sortBy("area").top(1000)` instead of a multi-gigabyte JavaScript array. The YAML renderer's caps stay, because
they protect the model's context; their truncation notes gain a pointer to data out ("… 152 more (return cv.json(x) for
all)"), and the CUT note says the same.

### 7.8 Charts

`look()` of a Value node renders a chart when its op declares one:

```go
// OpDef gains:
//   Chart ChartKind // ChartNone, ChartBars (histogram: counts), ChartLines (profile: t → meanL, meanR, meanG, meanB)
```

`look.New` checks `op.Chart` instead of the op name (`look.go:87-92`). `chart.go` gains a line renderer; both use
`pkg/cv/text` for axis labels. Charts are PNG and have no doc px (GCV-002 §7.7).

### 7.9 JSON and CSV content

```js
return {
  summary: { spots: await hot.count() },
  all: cv.csv(ranked, { name: "spots" }),            // a Regions node: every item, schema order
  raw: cv.json(rows, { name: "probe" }),             // plain data
}
```

```yaml
summary:
  spots: 213
all: "[content 1: data spots.csv (20 columns)]"
raw: "[content 2: data probe.json (2 items)]"
```

`cv.json` and `cv.csv` are module functions (`FuncDef`, GCV-002 §7.10) returning a **Data handle**, a Go object:

```go
// pkg/cv/dataout
type Data struct {
    name string
    mime string            // "application/json" | "text/csv"
    node graph.NodeID      // lazy: a node, evaluated at render time off the owner goroutine
    data any               // eager: plain data, converted on the owner goroutine at the call
    cols []string          // CSV columns (from the schema or the first rows)
    eng  *graph.Engine
}

func (d *Data) ResultPlaceholder() string                                  // "data spots.csv (20 columns)"
func (d *Data) ResultContent(ctx context.Context) ([]resultcontent.Content, error)
```

- `x` may be a Value or Regions node (lazy) or plain data (eager). The function's argument is declared with the type
  `data`. Awaited data is lifted back to its node by the prelude's `marshal` (`prelude.js:103`, Foundations §F4), so
  for `data` arguments the prelude passes the value without `marshal`: `cv.json(await r)` writes the data as the
  agent sees it, including edits the agent made to it. Image nodes are `E_WRONG_KIND` with the fix
  `node.save("x.tif")`.
- A lazy placeholder states what is known at build time: the name and, for CSV, the column count from the schema. Row
  counts are known only after rendering; the CLI file list and the MCP note report them.
- CSV columns default to the schema order for Regions and Value rows, and to first-seen key order for plain objects;
  `columns` selects and orders. Nested arrays and objects are JSON-encoded in their cell.
- `name` is an identifier matching `[A-Za-z0-9_-]{1,64}`; anything else (a path, `../../x`) is `E_ARG_TYPE` at bind
  time with a sanitized fix. Without `name`, the item is called `data-<N>`, N being its content number. Agent text
  therefore never becomes a path component.
- The content's caption is `data <name>.<ext>`; the extension comes from `resultcontent.ExtensionForMIME`.

**Files from the CLI.** `cmd/goja-cv`'s `writeLooks` becomes `writeContent`: looks keep their names
(`look-<n>-<slug>.<ext>`), data items are written as `<name>.<ext>` from their caption (a second item with the same name
gets `-2`), and dropped items are still reported with their reason. Because names are identifiers, every file lands
directly in `--out`. The CLI counts CSV records or JSON array items itself for the `rows` field. No upstream change to
go-go-goja is needed. The
`--json` output renames `looks` to `files`:

```json
{ "status": "ok", "result": "…", "files": [
  { "caption": "data spots.csv", "mimeType": "text/csv", "bytes": 18234, "rows": 213, "path": "/tmp/out/spots.csv" },
  { "caption": "look 1: view (…)", "mimeType": "image/png", "bytes": 40211, "path": "/tmp/out/look-1-view.png" } ] }
```

The colleague's calibration script then runs

```bash
goja-cv eval --json --out /tmp/specks --image TILE.tif \
  -e 'return cv.csv(img.detect.specks({ polarity: "bright", maxArea: 2000 }), { name: "specks" })'
```

and reads `files[i].path` for the item whose `mimeType` is `text/csv`.

**Budgets.** Looks and data are different budgets:

| setting | default | applied by |
|---|---|---|
| `Options.MaxLooks` | 4 | the session, in `state.collect` (`processor.go:116-140`): later looks are replaced by a note |
| `Options.MaxData` | 4 | the session, in `state.collect` |
| `Options.MaxDataBytes` | 0 = unlimited; the `mcp` command sets 256 KiB (counted on the rendered bytes) | the Data handle itself, in `ResultContent` (goja-cv has no hook after go-go-goja's Render; the handle receives the limit from the session that created it): a larger data item is replaced by the note "too large for the conversation (412 KB): save it with .save(\"spots.csv\")" |
| `ContentLimits().MaxItems` | `MaxLooks + MaxData` | go-go-goja's render limit; `mcp.go:61` mirrors it for the MCP transport |
| `ContentLimits().MaxTotalBytes` | 8 MiB, shared | go-go-goja |

The CLI leaves `MaxDataBytes` at 0, so it writes every data item.

### 7.10 Saving under an out-root

```js
const r = await img.view("speck").save("views/tile-17-speck.tif", { depth: 16 })
// { path: "/abs/out/views/tile-17-speck.tif", format: "tiff", depth: 16, encoding: "linear", frame: [1536, 1536, 1024, 1024], bytes: 6291912 }
const d = await cv.csv(ranked).save("spots/tile-17.csv")
```

`save` is a registered function (GCV-002 §7.10) on Image and Mask nodes and on the Data handle. It returns a promise of
`{ path, format, bytes, … }`; an unawaited save renders as `[Promise] (not awaited: add await)` like any promise
(`pkg/cvsession/yaml.go:109-110`).

- Paths resolve against the out-root, never the working directory. After symlink resolution the target must be inside
  the out-root (the containment check of `checkPath`, `native.go:72-84`, moved into a shared helper); parent directories
  are created; existing files are replaced and the result says `replaced: true`.
- Without an out-root, every save is `E_ACCESS: saving is disabled in this session` with the hint "start goja-cv with
  --out-root DIR".
- Images are evaluated at scale 1 and written as encoded values (GCV-001 §7.7) with the image's encoding.
- Values outside [0, 1] (signed derived images) are clamped by integer formats; the save warns `CLAMP` with the
  clipped fraction and the fix `{ depth: 32 }` (float TIFF) or `.npy`.

**Writers.** The current writers cannot record encoding and frame, so this ticket adds:

- a TIFF writer in `pkg/cv/buffer/tiffwrite.go`, paired with GCV-001's parser: uncompressed or Deflate strips, 1 or 3
  samples (no alpha), `uint8`, `uint16` or `float32` (`SampleFormat` 3), and `ImageDescription` (270) with
  `goja-cv encoding=<enc> origin=<x>,<y>`;
- PNG chunk insertion after `IHDR` in the bytes produced by `image/png`: `gAMA` (1.0 for linear) or `sRGB` (for sRGB),
  and `tEXt` with keyword `goja-cv` and the same string;
- an NPY writer for `<f4` and `<u2`.

| format | depths (default first) | encodings | what reopens (GCV-001 decoders) |
|---|---|---|---|
| `.tif`, `.tiff` | 16, 8, 32 (float) | srgb, linear | values, encoding, frame origin |
| `.png` | 16, 8 | srgb, linear | values, encoding (`gAMA`/`sRGB`), frame origin (`tEXt`) |
| `.npy` | 32 (`<f4`), 16 (`<u2`) | any | values only: opens as `linear` with origin (0, 0); a note in the result says so |
| `.jpg`, `.jpeg` | 8 | srgb only | values; a linear image is `E_ARG_TYPE` with the fix `.png` |

Masks are saved as single-channel images with the linear note (`encoding=linear` in TIFF, `gAMA` 1.0 in PNG), because
a mask's value is its displayed value (GCV-001 §3.6); they reopen with the same numbers. `buffer.WriteFile` and
`EncodeTIFF16` are deleted, and `internal/testimage` (`testimage.go:42,45`, `planted.go:127`), which calls
`buffer.WriteFile`, switches to the new writers in the same phase; GCV-001's fixture writers for TIFF and NPY are
deleted in favour of them, and only the minimal DNG writer (SubIFDs, LinearRaw) stays in `internal/testimage`.

### 7.11 Preludes

```bash
goja-cv eval --image tile.tif --prelude scripts/84-cv-helpers.js -e 'return await mem.probe(1337, 131, 3)'
goja-cv mcp --image tile.tif --root . --prelude helpers.js
```

- `--prelude FILE` (repeatable) on `eval`, `run` and `mcp`; `cvsession.Options.Preludes []string`.
- `App.CreateSession` evaluates each file in order with the async-body semantics of agent code (so a prelude may `await`
  and `return`) before the session is handed out. MCP sessions created later run them too.
- A prelude that fails aborts session creation with its file name and line (`prelude helpers.js:12: …`), rendered like an
  agent error. Prelude results are not printed; `mem` keeps what they define and the context block lists it.

The key points:

- One probe kernel and one coverage rule serve `sample`, `profile`, `measure` and `regions.measure`.
- One schema mechanism serves Regions and Value results, help, the prelude and build-time validation.
- Point ops take points; Regions are measured by `regions.measure`, which keeps their ids.
- Data leaves through Data handles: as content, as named CLI files, or saved under the out-root.
- Saved files record encoding and frame where the format can hold them, and the table says where it cannot.

## 8. Decision records

### Decision: flat scalar fields, one naming rule

- **Context:** Items say `centroid`, filters say `cx` (issue 17); the colleague proposed accepting both. Measurement rows
  also need names for points.
- **Options considered:** (a) aliases both ways; (b) arrays (`box`, `centroid`) everywhere with array-aware filters;
  (c) flat scalars with one meaning per name (`x, y` corner, `cx, cy` point) and object coercion for box and point
  arguments.
- **Decision:** (c).
- **Rationale:** Aliases double every list and error and are excluded by the project's rules. Filters and sorting need
  scalars. One meaning per name lets region items and measurement rows be passed wherever a box or point is expected.
- **Consequences:** Every example that reads `box`/`centroid` changes (§11); rows name their point `cx, cy`.
- **Status:** accepted

### Decision: one schema mechanism for Regions and Value results

- **Context:** `OpDef.Fields []string` already exists for Value results; revision 1 added a separate `RegionFields`.
- **Options considered:** (a) a second field for Regions; (b) replace `Fields []string` with `Fields func(args, inputs)
  []FieldSpec` for every result kind.
- **Decision:** (b), with `Validate` receiving the input nodes and a static `Produces` list for hints.
- **Rationale:** One mechanism serves items, rows, help, the prelude's derived-field rule, units for `SPACE` checks and
  build-time validation.
- **Consequences:** Every existing validator changes signature; `__fieldNames` reads specs.
- **Status:** accepted

### Decision: throwing accessors for removed field names

- **Context:** Agents and the colleague's helpers read `r.box` and `r.centroid`.
- **Options considered:** (a) nothing (silent `undefined`); (b) aliases; (c) non-enumerable accessors on a shared
  prototype that throw with the literal fix, driven by `RemovedFields`.
- **Decision:** (c), to be removed when the playbook, the helpers and the CVJS-001 docs no longer use the old names.
- **Rationale:** (a) turns an old habit into wrong numbers downstream; (b) is excluded. (c) returns no data and costs one
  prototype per Regions value.
- **Consequences:** `where`/`sortBy` errors use the same table.
- **Status:** accepted

### Decision: coverage is the 4 × 4 supersampled rule of `mask.ellipse`

- **Context:** The probe kernel needs a disk coverage rule; `mask.ellipse` uses 4 × 4 supersampling; the playbook's
  thresholds were calibrated through `mask.ellipse`.
- **Options considered:** (a) exact area for measurements only; (b) exact area for both; (c) 4 × 4 supersampling for
  both, through one function.
- **Decision:** (c).
- **Rationale:** (a) makes `measure` and `stats({ mask: ellipse })` disagree; (b) moves the calibrated numbers. (c)
  keeps them and removes the duplication.
- **Consequences:** Weights are multiples of 1/16 on edge pixels; tests compare with `StatsWeighted` within 1e-9
  (sums run in a different order).
- **Status:** accepted

### Decision: no item cap; caps belong to presentation

- **Context:** The 100-item cap truncates data inside JavaScript (issue 16).
- **Options considered:** (a) `items({ all: true })`; (b) raise the cap; (c) no cap in data, a safety limit, and caps in
  the YAML renderer.
- **Decision:** (c).
- **Rationale:** Code that iterates items cannot know it saw a prefix. The model's context is protected where text is
  produced. (a) adds a method whose only purpose is to undo a limit.
- **Consequences:** Memory per awaited Regions grows with the count; the safety limit bounds it.
- **Status:** accepted

### Decision: measurements are Value ops on a shared probe kernel; point ops take points only

- **Context:** `sample`, `profile`, `measure` and `regions.measure` read small neighbourhoods; revision 1 let `sample` and
  `measure` take Regions too.
- **Options considered:** (a) JavaScript helpers over `stats`; (b) a kernel per op; (c) one probe kernel, with Regions
  measured only by `regions.measure`.
- **Decision:** (c).
- **Rationale:** (a) costs a whole-image pass per call. One kernel gives identical numbers across ops. One entry point for
  regions avoids two `snr` values for one spot (different core radii, rows without ids).
- **Consequences:** `img.measure(regions)` is an error with the fix.
- **Status:** accepted

### Decision: data out as result content and named files; files written only under an out-root

- **Context:** Issue 16 suggests `--json-out FILE` or `cv.export(value)`; issue 21 asks for `node.save(path)`.
- **Options considered:** (a) a CLI flag that writes the whole result as JSON; (b) Data handles as result content, named
  files in the CLI, explicit saves under a write root; (c) unrestricted file writes.
- **Decision:** (b).
- **Rationale:** Content reuses the mechanism looks use; the CLI writes named files without an upstream change; separate
  read and write roots keep an agent that may read scans from writing next to them. (a) cannot mix data with text and
  looks.
- **Consequences:** `--out-root` is required for saving; data larger than 256 KiB reaches MCP clients only as a saved file.
- **Status:** accepted

### Decision: separate budgets for looks and data

- **Context:** go-go-goja counts every content item against one limit set to 4 looks.
- **Options considered:** (a) one shared limit; (b) an upstream change with per-class limits; (c) the session enforces
  look and data counts and the data byte limit, and the transport limit is their sum.
- **Decision:** (c).
- **Rationale:** No upstream change, and the session already numbers looks in `collect`.
- **Consequences:** `Options.MaxData`, `MaxDataBytes`; the MCP command sets the byte limit.
- **Status:** accepted

### Decision: saved files carry their encoding and frame where the format allows

- **Context:** A saved linear derived view reopened as sRGB would be decoded with the wrong curve.
- **Options considered:** (a) always write sRGB; (b) write encoded values and record encoding and origin in TIFF
  descriptions and PNG chunks, with own writers; (c) sidecar JSON files.
- **Decision:** (b), with the table of §7.10 stating what each format keeps.
- **Rationale:** (b) round-trips inside goja-cv and is visible to other tools; (c) separates data from its meaning. NPY and
  JPEG cannot hold the metadata, and the result says so.
- **Consequences:** Own TIFF, PNG-chunk and NPY writers; `buffer.WriteFile` is deleted.
- **Status:** accepted

### Decision: preludes run at session creation through the eval path

- **Context:** Issue 23b wants helpers in every CLI run.
- **Options considered:** (a) run prelude files in the runtime initializer (synchronous); (b) evaluate them as agent code
  after the session is created.
- **Decision:** (b).
- **Rationale:** Helper files like `84-cv-helpers.js` are async-function bodies (`await`, `return "helpers ready"`); (b)
  gives them the same semantics and error rendering as typed code.
- **Consequences:** Session creation can fail with a script error, which the MCP tool reports.
- **Status:** accepted

## 9. Pseudocode and key flows

### 9.1 The probe kernel

```
probe(buf, frame, scale, centre (doc px), coreRadius r, ringRadii [r1, r2], gray weights):
  c := ((cx − frame.X) · scale, (cy − frame.Y) · scale)          // buffer px
  window := square of side 2·r2·scale + 2 around c, clipped to the buffer
  for each pixel p in window:
      wc := EllipseCoverage(p, c, r·scale, r·scale)              // 4×4 supersampled
      wr := EllipseCoverage(p, c, r2·scale, r2·scale) − EllipseCoverage(p, c, r1·scale, r1·scale)
      accumulate wc- and wr-weighted sums of the gray plane and of each channel; keep (gray value, wr) pairs
  coreMean, ringMean, per-channel means; spread = weighted p90 − p10 (the rule of kernels.weightedQuantile)
  ringCoverage = Σ wr inside the frame / (full annulus weight)
```

### 9.2 `regions.measure`

```
Run(regions, image, opts):
  check frames; img := image at analysisScale 1
  items := copies of the items with copied measure maps            // items are shared and immutable
  for each region:
      weights := region.Pix (same raster) or region.Coverage(img frame, scale)
      if opts.stats: weighted stats of the gray plane and, with channels, each channel
      if opts.ring:  probe(img, region centroid, coreRadius(region), ringRadii)
  name fields with the prefix; the node's Fields already list them (computed at intern time)
  return regions.With(items)
```

### 9.3 Delivering a CSV over MCP and the CLI

```
eval returns { all: cv.csv(ranked, { name: "spots" }) }
  Extract: placeholder "[content 1: data spots.csv (20 columns)]"
  collect: 1 data item ≤ MaxData
  Render (off the owner goroutine): evaluate ranked → RegionsValue → rows in schema order → CSV bytes
  MCP: text/csv → text content if ≤ 256 KiB, else replaced by the save note
  CLI: writeContent → /out/spots.csv; files: [{ caption: "data spots.csv", rows: 30, path: … }]
```

## 10. Errors and diagnostics

| situation | code | where defined | message and fix |
|---|---|---|---|
| unknown field in `where`/`sortBy` | `E_UNKNOWN_OPTION` | graph | this node's fields; the producing op from `Produces` |
| removed field (`box`, `centroid`, `meanMask`) | `E_NO_FIELD` | graph | `RemovedFields` fix |
| point given as `{ x, y }` | `E_ARG_TYPE` | graph | "x, y is a box corner; use [o.x, o.y] or an object with cx, cy" |
| Regions passed to `sample`/`measure` | `E_WRONG_KIND` | graph | `regions.measure(img, { ring: true })` |
| measuring a field twice | `E_FIELD_EXISTS` | graph | add `prefix` |
| regions and image on different frames | `E_FRAME` | graph | both frames |
| profile too long | `E_ARG_RANGE` | graph | larger `step` |
| ring radii wrong (`r1 ≤ coreRadius` or `r2 ≤ r1`) | `E_ARG_RANGE` | graph | the default ring |
| too many regions | `E_TOO_MANY` | graph | `sortBy(…).top(n)` |
| save without out-root, or outside it | `E_ACCESS` | graph (moved) | `--out-root DIR`; a path inside |
| save of a Value/Regions node | `E_WRONG_KIND` | graph | `cv.json(x).save("x.json")` |
| `cv.json(imageNode)` | `E_WRONG_KIND` | graph | `node.save("x.tif")` |
| linear image to JPEG | `E_ARG_TYPE` | graph | `.png` |
| points outside the frame | `OUTSIDE` warning (evaluation time) | graph | count and the frame |
| integer format clamps signed values | `CLAMP` warning (evaluation time) | graph | `{ depth: 32 }` or `.npy` |
| prelude fails | session error | cvsession | `prelude FILE:LINE: …` |

Synonyms: ops `probe`, `ring`, `contrastAt` → `measure`; `valueAt`, `pixel`, `at` → `sample` (on Image); `lineProfile`,
`line`, `trace` → `profile`; `export`, `write`, `saveAs` → `save`; `toJSON` → `json`; `toCSV`, `table` → `csv`. Options:
`radius`, `r` → `coreRadius` (measure); `r` → `radius` (sample); `annulus`, `background`, `ring` → `ringRadii` (measure);
`thickness` → `width`; `fields` → `columns`; `bits` → `depth`.

## 11. Documentation and examples that change

- A new topic `pkg/cv/docs/topics/12-measuring.md` (id `measuring`) explains sample, profile (with a plot), core/ring
  with the worked example of §3.3, `regions.measure` with the hot-spot recipe, the naming rule, and which space numbers
  are in.
- A new topic `13-data-out.md` (id `data-out`) explains `cv.json`, `cv.csv`, `.save`, `--out-root`, `--prelude`, and the
  difference between YAML (for the agent) and data content (for programs).
- `05-masks-and-regions.md` changes the item description (line 29) and the example (lines 35-40) from `box`/`centroid` to
  the flat fields, and shows passing an item as a box.
- `09-recipes.md` changes "Measure inside a box or region" to `measure`/`sample` and adds "Rank candidates" (the hot list).
- `03-nodes-and-evaluation.md` says that awaited Regions have every item.
- The guidebook `pkg/doc/03-cli.md` documents `--prelude`, `--out-root` and the `files` list of `--json`;
  `pkg/doc/08-recipes.md` follows.
- The MCP instructions gain one line for `measure`/`sample` and one for data out; the Regions line lists the flat fields.
- For the colleague's hand-over: `mem.probe(x, y, r)` becomes `img.measure([[x + 0.5, y + 0.5]], { coreRadius: r + 0.5,
  ringRadii: [r + 3.5, r + 9.5] })` (§13.2); `mem.hot` becomes `components().measure(...)`; with a tile opened with
  GCV-001's `origin`, rows are already absolute raw-scan coordinates, so `mem.origin` and the `ax, ay` fields go; script
  83 uses the command of §7.9.

Tests and examples that change: every test reading `box`/`centroid` from items (`pkg/cv/ops/regions_test.go`,
`detect_test.go`, `pkg/gojamod/cv/module_test.go`, `m3_test.go`, cvsession tests, README); `truncated` assertions;
`meanMask` tests; the CLI tests of look file names and `--json` (`cmd/goja-cv/main_test.go`).

## 12. Implementation phases

Each phase ends with `go test ./... -count=1`, `make lint` and a commit. Write the listed tests first.

**Phase 1 — schema.**
1. Write tests: for every op that returns Regions or Value rows, the keys of an item or row equal the names of its
   node's `Fields` (a property test over the registry); `where` on a field from another op fails at build time with the
   producer hint; `RemovedFields` accessors throw with fixes; `mask.gradient` accepts a point or an edge name.
2. Implement `schema.go`, `OpDef.Fields` as a function, `Produces`, `Validate(args, inputs)` (update every validator),
   `Node.Fields` in `internNode`, flat `Region.Data`; delete `RegionFields`, `KnownRegionFields`, `speckFields`,
   `FieldNames` and `meanMask` (tests and docs that read `meanMask` switch to `stats({ mask })` until Phase 4).

After Phase 1, region items are flat, and region items can be passed as boxes and points everywhere.

**Phase 2 — no cap.**
1. Write tests: 5000 regions are all awaited; `E_TOO_MANY` above the limit; YAML truncation notes mention `cv.json`.
2. Remove `SummaryLimit` and `truncated`; add the safety limit.

**Phase 3 — probe kernel and measurement ops.**
1. Write tests: `probe` core and ring means equal `StatsWeighted` with `mask.ellipse` masks within 1e-9; `measure` matches
   `mem.probe` under the mapping of §13.2; `sample` with radius 0 equals the pixel; a profile across a synthetic edge of
   known σ recovers it (a fitted σ within 10 %); edge rows report `ringCoverage`; `OUTSIDE` warnings; spaces respected.
2. Implement `EllipseCoverage` (and switch `EllipseMask` to it), `kernels/probe.go`, `sample`, `profile`, `measure`; merge
   `StatsWeighted`.

**Phase 4 — `regions.measure`.**
1. Write tests: stats fields equal `stats({ mask: regions.only([id]).toMask() })` per region; ring fields equal
   `img.measure` at the centroid with the default core radius; prefixes; `E_FIELD_EXISTS`; `E_FRAME`; regions found at a
   different raster size use `Region.Coverage`.
2. Implement the op and `Region.Coverage`.

**Phase 5 — charts.**
1. Write tests: a profile look is a PNG line chart of the right size; the histogram chart is unchanged.
2. Implement `OpDef.Chart` and the line renderer.

**Phase 6 — writers and saving.**
1. Write tests: the TIFF writer's output is read back by GCV-001's parser with values, encoding and origin for uint8,
   uint16 and float32; PNG chunks are read back; NPY values round-trip and open as linear at origin (0, 0); JPEG refuses
   linear images; containment refuses a symlink pointing outside the out-root; `CLAMP` warnings.
2. Implement `tiffwrite.go`, PNG chunk insertion, the NPY writer, `save` on nodes, the shared containment helper and
   `Options.OutRoot`; switch `internal/testimage` to the new writers and delete its duplicate TIFF/NPY fixture writers
   (keep the DNG one); delete `buffer.WriteFile` and `EncodeTIFF16`; move `EAccess`.

**Phase 7 — data out.**
1. Write tests: `cv.json`/`cv.csv` placeholders and bytes; lazy nodes and eager data (an edited awaited object is
   written as edited); CSV columns; `data.save`; budgets (a fifth look and a fifth data item are replaced by notes; a 300
   KiB item over MCP gets the save note); `writeContent` names (`look-1-…`, `spots.csv`, `spots-2.csv`) and dropped-item
   reporting; `--json` `files` list.
2. Implement `pkg/cv/dataout`, the `json`/`csv`/`save` `FuncDef`s, the `data` argument type, the budgets in
   `collect`, `writeContent`, and the `--json` change.

**Phase 8 — preludes.**
1. Write tests: a prelude defining `mem.f` is usable in the eval; a failing prelude reports `FILE:LINE`; MCP sessions
   created later run preludes.
2. Implement `Options.Preludes` and the flags on `eval`, `run` and `mcp`.

**Phase 9 — documentation and agent check.**
1. Write the topics, guidebook, instructions and synonyms of §10 and §11; the docs tests pass.
2. Run the gated tests (§13.4) and the pi task (§13.5).

The key points:

- The schema change comes first because every later op declares its fields through it.
- Writers come before data out, so `data.save` and `node.save` share them.
- `meanMask` disappears in Phase 1 so the schema property test holds from the start.

## 13. Test strategy

### 13.1 Unit and property tests

- The probe tests check `EllipseCoverage` against `EllipseMask` bit for bit, and the probe means against `StatsWeighted`
  with ellipse masks within 1e-9 (sums run in a different order).
- The schema property test checks, for every op returning Regions or Value rows, that item or row keys equal the schema
  names and that every schema field is accepted by `where` and `sortBy`.
- The coercion tests check box and point objects, unions with enums, and the refusal of `{ x, y }` as a point.
- The CSV tests check quoting, nested values and column selection; the JSON tests round-trip through `encoding/json`.
- The writer tests check every row of the §7.10 format table, including what does not round-trip.

### 13.2 Equivalence with the colleague's helper

`measure` must return what the agents already trust. Copy `mem.probe` from `scripts/84-cv-helpers.js` into
`pkg/cvsession/testdata/probe.js`, run it and `img.measure` on the same points of the planted scan
(`internal/testimage/planted.go`) in one session, and compare. Two details of the helper are mapped, and `measure`'s
Doc string states the mapping so agents can translate the playbook's numbers:

- Coordinates. The helper's core is the ellipse inscribed in the box `[x − r, y − r, 2r + 1, 2r + 1]`, a disk of radius
  r + 0.5 centred on (x + 0.5, y + 0.5), the centre of pixel (x, y); its ring radii are r + 3.5 and r + 9.5. So
  `mem.probe(x, y, r)` corresponds to `img.measure([[x + 0.5, y + 0.5]], { coreRadius: r + 0.5, ringRadii: [r + 3.5,
  r + 9.5] })`.
- Luminance. The helper's `step` averages the three channel steps (equal weights) while its `spread` uses `stats()`
  luminance percentiles (Rec. 709 weights). `measure` uses one gray plane per call. With `gray: "average"`, `step`,
  `stepR/G/B` and `stepChroma` must equal the helper's within 1e-6; with the default `gray: "rec709"`, `spread` must.

Because both use 4 × 4 supersampled coverage (§3.1), any other difference is a bug.

### 13.3 Documentation tests

The `measuring` and `data-out` topics' ```js blocks run against the planted scan (Foundations §F11). Saving needs an
out-root: the docs test harness creates a temporary out-root and passes it in `Options`.

### 13.4 Real tiles (gated)

On `GOJACV_TILES` and `GOJACV_GOLD` (the gold points of GCV-002 §13.4): build the hot list with the unstretched DoG and
`components`, measure it with `ring: true`, and assert that labelled dust has median `snr` ≥ 1.5 and that the fraction of
gold points within 4 px of a top-30 spot is ≥ 70 % (the colleague's `mem.hot(2)` covered 77 %). The same test prints the
dust and non-dust ranges of `snr` and `stepChroma` with `regions.measure`'s defaults; these replace the playbook table in
the hand-over. A benchmark (`go test -bench HotList`) measures the hot-list measurement on a 1024² tile at scale 1 with a
warm cache and asserts it is at least 20 times faster than the helper's 56 `stats` calls on the same machine.

### 13.5 Agent harness

`questions-gcv-003.md`, next to `questions.md` in the pi harness (Foundations §F11, with GCV-001's overrides), on one tile:

1. Rank the 20 strongest candidate spots by how much they stand out, with snr and stepChroma.
2. Give the full candidate table as CSV.
3. Save the view you used as a 16-bit TIFF and say where it is.

Run the build before this ticket (expect the probe helper or many `stats` calls, and YAML paging) and after, twice each;
record calls, errors, and whether the CSV row count equals the region count.

## 14. Risks, alternatives and open questions

**Risks.**

- Breaking every region example: the flat schema touches docs, tests, README and the colleague's helpers. The throwing
  accessors and the docs tests catch what greps miss.
- Large data in the conversation: JSON delivered as MCP text enters the model's context. The 256 KiB limit and the save
  note keep large tables out; the docs say to return summaries to the model and data to files.
- Write access: an out-root is a new capability. Containment follows the read-root code and is tested with symlinks.

**Alternatives not taken.** A `Table` kind for tabular results (Regions and Value rows already are tables);
`components({ source: image })` from issue 10 (it would fold measurement into one producer, while `regions.measure`
works on any region set, including filtered and detected ones); writing the whole result as JSON from the CLI (`--json`
already prints the result text as JSON); storing preludes in a library (the CVJS-001 `lib` design, not yet built).

**Open questions.**

1. Should `regions.measure` support rings shaped like the region (dilated region minus region) instead of disks around
   the centroid? Better for elongated hairs; the disk version matches the playbook's numbers.
2. Should a data item over the MCP limit be saved automatically when the session has an out-root, instead of being
   replaced by a note?

## 15. Closing

With the three tickets done, an agent can open scientific data in its own encoding, build views in the right space, look
at any window at any zoom with coordinates, compare many candidates in one sheet, back each decision with numbers that
match the helpers it trusted, and hand complete tables and derived images to programs. The colleague's playbook loses its
Python zoom, its export trick, its pasted helpers and its YAML paging.

## 16. References

- Foundations: `ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md`.
- Review findings with disposition: `reference/01-review-read-findings.md` in this ticket.
- Handover issues 9, 10, 16, 17, 21, 23: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md`; feature requests items 4, 9, 10, 11: `…/reference/05-goja-cv-feature-requests.md`; generic list C1–C4b, D3, D4, E1, E4: `…/reference/06-goja-cv-generic-inspection-features.md`.
- Helpers and calibration: `…/DUST-002…/scripts/84-cv-helpers.js` (`mem.probe`, `mem.hot`), `…/scripts/83-cv-specks-vs-gold.py`; playbook section 4.
- go-go-goja result content and MCP mapping: `resultcontent/resultcontent.go`, `resultcontent/files.go`, `replmcp/tools.go` (module cache, version in `go.mod`).
- CVJS-001 intern guide Part 2 §5 (masks vs regions) and the integration-surfaces doc §3: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/`.
- GCV-001 (spaces, Level unit, frames, box/point types, decoders that read saved metadata) and GCV-002 (functions, text font, render path, evaluation warnings, sheet and annotate): `design-doc/01-design-and-implementation-guide.md` of each ticket.
- TIFF 6.0 specification; PNG specification (gAMA, sRGB, tEXt chunks); NumPy `.npy` format (NEP 1).

## Implementation notes (deviations)

Recorded while implementing on branch `task/gcv-003-measurement-data-out` (stacked on `task/gcv-002-inspection-views`;
details and reasons in `reference/03-diary-implementation.md`). The guide above is unchanged; these notes say where
the code differs.

1. **`OpDef.Produces` is a map** (§7.5): field name → the call that adds it (`"snr": "regions.measure(img, { ring:
   true })"`), so the producer hint carries a literal fix. The hint is added by the builder (`Registry.producerHint`)
   to errors made with `graph.UnknownFieldError`, because `Validate` does not see the registry.
2. **`FieldSpec.Optional`**: fields present only in some rows (`check.dust`'s `hint`, a sample/measure row's
   `outside`, channel fields whose presence depends on the channel count, which is not a node static). The schema
   property test requires every non-optional field and allows nothing outside the schema.
3. **Ordered data for JavaScript and JSON**: `graph.Record` (keys in schema order) for region items, and
   `graph.OrderBySchema` for awaited Value rows/objects, because the binding sorts map keys. Data-lambda items get
   the same prototype with the throwing accessors (`__itemProto`).
4. **`sortBy` of a string field** is `E_ARG_TYPE` at build time; unknown fields are `E_UNKNOWN_OPTION` for both
   `where` and `sortBy` (sortBy used `E_ARG_TYPE`).
5. **`only`/`without` take items**: an object with `id` (a region item) or a list of them, besides ids.
6. **Rows outside the frame** keep every required key with `null` values plus `outside: true`; `OUTSIDE` names the
   row indices.
7. **Points are `Regions|point[]`**: the Regions alternative exists only so the validator can refuse it with the
   `regions.measure` fix (a plain `point[]` would give the binder's generic "Value nodes only" hint).
8. **The probe measures through `StatsWeighted`** on the gathered pixels of each disk or ring (row-major, positive
   weights only), so it equals `stats({ mask: mask.ellipse(…) })` bit for bit in the tests, ties of the weighted
   quantile included. `kernels.EllipseCoverage(i, j, cx, cy, rx, ry)` takes the pixel's integer index.
9. **`regions.measure`**: the explicit input is `Image|Mask` (so `components().measure(mask, { stats: ["mean"] })`
   replaces `meanMask`); `stats` is a `string[]` validated by the op with synonyms (`maximum → max`, …) because the
   binder's enum check runs before its list check; stats per channel only with `channels: true`, while ring channel
   steps are on for colour images unless `channels: false`; `min`/`max` are the 0 and 1 weighted quantiles;
   `RegionsValue.Coverage` shares `Rasterize`'s per-pixel overlap loop.
10. **Charts**: the histogram bar chart is unchanged (no axis labels), so its goldens hold; only the profile line
    chart has labels (value range, t range in doc px).
11. **Writers**: TIFF strips are always Deflate (no uncompressed option); PNG `sRGB` chunk for sRGB values and `gAMA`
    100000 for linear ones; the frame origin is recorded only when it is not (0, 0). Views are derived images, so a
    saved view is `encoding=linear`. `EncodePNG` gained a `Meta` argument instead of a second function.
12. **Fixture writers kept for decoder variants**: GCV-001's TIFF/NPY fixture writers remain in `internal/testimage`
    for what goja-cv reads but never writes (LZW, PackBits, predictors, big-endian, `|u1`, `>u2`, `>f4`, `<f8`);
    deleting them would drop those decoder tests. Every plain write (planted scan, gradients, linear fixtures) uses
    the production writers through `testimage.WriteFile`.
13. **Containment**: `graph.Within` resolves the longest existing prefix of a path through symlinks, so a file
    under a symlinked directory pointing outside the root is refused before anything is created; `cv.open`'s root
    check uses it too. `OutRoot` lives in `graph.EngineOptions` (the cv module saves) and `cvsession.Options`.
14. **Data handles** are `goja.DynamicObject`s whose exported value embeds `*dataout.Data`, giving JavaScript
    `name`, `format` and `save` while `resultcontent.ProviderOf` still finds the provider. Eager data is exported on
    the owner goroutine with key order (`exportOrdered`); nodes inside plain data are written as their description.
    The binder type `data` keeps the argument as given (a node or plain data). `json`, `csv`, `save` are FuncDefs
    bound with `BindOptions`; the prelude hand-writes `cv.json`, `cv.csv` and `node.save` (marked Bespoke where
    module functions).
15. **CSV tables**: an object whose longest arrays share a length (a profile) is one row per element; awaited
    Regions (`{ kind: "Regions", items }`) are their items; lists of lists get columns `c1, c2, …`. The placeholder
    gives a column count only for row schemas (not for a profile's).
16. **Budgets**: looks and data items beyond `MaxLooks`/`MaxData` are left out in `collect` and their placeholders
    say `[not delivered: … — more than 4 … in one eval; …]`; the MCP over-limit note is a `text/plain` content item.
17. **Preludes** run through `App.Evaluate` (they appear in the session history); `FILE:LINE` is read from the
    rendered error's `where` line; a failing prelude deletes the half-made session; the CLI exits 1 for it and 2 for a
    missing file.
18. **Speck view default space** (GCV-002 open item): `view("speck")` and `view("speckColor")` default to linear
    space through a new `SpaceRule.For` (per-call default). `TestRealViewRanking`, 142 gold points on 15 tiles:
    encoded median rank 20, 70.4 % in the top 50; linear 18, 79.6 %. Other presets stay encoded. Golden `sheet-2x2`
    (a speck-view cell) moved.
19. **Gated hot-list test** (§13.4): the DoG runs in linear space (density on these tiles), as the speck view now
    does. Measured on 142 gold points of 15 tiles: 71.8 % within 4 px of a hot spot (about 37 per tile), 62.7 % within
    4 px of a top-30 spot by snr (the guide's 70 % for the top 30 is not met; the test asserts ≥ 70 % for any spot and
    ≥ 60 % for the top 30). With the encoded DoG: 63.4 % and 59.2 %. Labelled dust: snr median 2.17 (middle half
    1.61–3.03), stepChroma median 0.00 (0.00–0.26); unlabelled spots: snr 0.86 (0.56–1.23), stepChroma 0.00
    (0.00–0.51). These replace the playbook table for the hand-over.
20. **HotList benchmark**: kernel level (`kernels.ProbePoint` against the helper's three whole-image ellipse masks,
    a minus and two `StatsWeighted` per spot), 28 spots on a 1024² tile: about 2 ms against 470 ms (over 200×);
    `TestHotListIsFasterThanTheHelper` asserts ≥ 20×.
21. **Evaluation warnings live in the value cache** (GCV-002 open item): `Cache.SetWarnings/Warnings`, dropped with
    their entry; a value too large to cache reports its warnings directly.
22. **Extra legacy removed**: the old `EncodePNG`/`EncodeTIFF16`/`WriteFile` and the `x/image/tiff` writer path,
    `statsWeighted` wrapper, the `measure → stats` op synonym; method redirects added for `export`/`write`/`saveAs`
    (→ save), `toCSV`/`table` (→ cv.csv), `json`/`dump` (→ cv.json).
23. **Not done**: windowed pixel-op evaluation with `OpDef.Halo` (GCV-002 open item; design note in the diary,
    step 9); moving the `cvFuncs` help table to `pkg/cvsession` (optional; `cv.help` lives in the module).
