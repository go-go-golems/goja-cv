---
Title: Diary - implementation
Ticket: GCV-003
Status: active
Topics:
    - measurement
    - regions
    - export
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/cv/graph/schema.go
      Note: FieldSpec schema, removed fields, producer hint (Phase 1)
    - Path: repo://pkg/cv/ops/schema.go
      Note: op result schemas (Phase 1)
    - Path: repo://pkg/gojamod/cv/prelude.js
      Note: shared item prototype with removed-field accessors (Phase 1)
ExternalSources: []
Summary: 'Implementation diary of GCV-003 (measurement and data out): one step per phase of the design guide, with commands, failures, golden-look outcomes, review notes and commit hashes.'
LastUpdated: 2026-11-09T07:23:01.422048-05:00
WhatFor: Reviewing and continuing the GCV-003 implementation on branch task/gcv-003-measurement-data-out (stacked on task/gcv-002-inspection-views).
WhenToUse: Before reviewing a GCV-003 commit, or when picking up the open items it hands over.
---


# Diary - implementation

## Goal

This diary records the implementation of GCV-003 (measurement and data out: sampling, profiles, region measures, one
flat region schema, export) in goja-cv, phase by phase as the design guide (`design-doc/01-design-and-implementation-guide.md`,
revision 3) prescribes, on top of GCV-001's and GCV-002's code. Each step lists what changed, what failed (verbatim),
which golden looks moved, what to review and the commit hashes. Deviations are also appended to the guide's
"Implementation notes (deviations)" section at the end.

## Step 1: Phase 1 - one field schema, flat region items

Phase 1 replaces the five field lists (`RegionFields`, `KnownRegionFields`, `speckFields`, `Region.FieldNames`,
`OpDef.Fields []string`) with one mechanism: `OpDef.Fields` is a function of the call's arguments and input nodes that
returns `[]graph.FieldSpec`, computed into `Node.Fields` at intern time like the other statics. Region items are flat
(`id, x, y, w, h, cx, cy, area, aspect, …`) in schema order, `where`/`sortBy` validate against the receiver's schema at
build time, and the error names the op that produces a missing field.

What it unlocks: a region item is now both a box and a point, so `img.crop(item)`, `img.stats({ box: item })`,
`img.mask.flood(item)`, `annotate([{ box: item }, { point: item }])`, `cv.sheet(items)` and `only(item)` work (open
item 2 of the hand-over). Old habits (`r.box`, `r.centroid`, `meanMask`) throw `E_NO_FIELD` with the literal fix.

### Prompt Context

**User prompt (verbatim):** "You are the GCV-003 implementer for goja-cv (`/Users/manuel/code/wesen/go-go-golems/goja-cv`, remote go-go-golems/goja-cv). Your goal is to implement the GCV-003 design guide (measurement and data out: sampling, profiles, region measures, one flat region schema, export) phase by phase, with commits, pushes, a PR and your own diary." (opening paragraph of the coordinator's brief; the brief continues with the reading list, nine open items handed over by GCV-001/GCV-002 - the speck view's default space, flat items as arguments, measurement ops on box/point types, profile plots on GCV-002's Render and font, json/csv/save as FuncDefs, writers with goja-cv metadata, stepChroma, windowed evaluation with OpDef.Halo, the unbounded warning cache - the branching and git rules, the gate, the no-compatibility-layer and remove-legacy rules, golden-look rules, data-out rules, docs to update, real-data limits, off-limits paths, optional pi runs and the final report.)

**Assistant interpretation:** On a branch `task/gcv-003-measurement-data-out` stacked on `task/gcv-002-inspection-views`, implement Phases 1-9 of the GCV-003 guide in order, following the real code where the guide assumes otherwise; gate every push (build, vet, race tests, lint, logcopter-check); commit code then diary per phase; remove the guide's §6 legacy; settle the nine open items; open a PR against the GCV-002 branch, watch CI and report.

**Inferred user intent:** Give the colleague's dust agents native measurement (sample, profile, core/ring measures, regions measured on another image) and a clean data path out of an eval (CSV/JSON, saved files with their encoding), so the pasted JavaScript helpers, YAML paging and Python export tricks go away, with a reviewable history and every deviation recorded.

**Commit (code):** f2509c0 - "GCV-003 Phase 1: one field schema (FieldSpec, OpDef.Fields/Produces, Validate with inputs), flat region items, removed-field diagnostics"

### What I did
- `pkg/cv/graph/schema.go` (new): `FieldSpec{Name, Type, Unit, Doc, Optional}`, `GeometryFields` (id, x, y, w, h, cx, cy, area, aspect), `RemovedFields` (box, centroid, meanMask → literal fix), `RemovedFieldError`, `UnknownFieldError` (records the field so the builder can add the producer), `Registry.producerHint`, `DefaultArgs`, `OpDef.DocFields`, `nodeFields`, and `Record` (an object whose keys keep schema order when converted to JavaScript).
- `OpDef`: `Fields func(args, inputs) []FieldSpec`, `Produces map[string]string` (field → the call that adds it), `Validate func(args, inputs) error`; all 16 validators changed signature. `Node.Fields` set in `internNode` (a macro takes its expansion's, Regions→Regions ops their receiver's). The evaluator stamps `RegionsValue.Fields`.
- `Region.Data(fields)` returns a `Record` in schema order (geometry to 0.01, measures to 4 decimals); `RegionsValue.Schema()` falls back to geometry + the first item's measures for values built outside the graph.
- Deleted `RegionFields`, `KnownRegionFields`, `Region.FieldNames`, `speckFields []string`, `components`' `meanMask`.
- `pkg/cv/ops/schema.go` (new): the schemas of `detect.specks`, `stats` (area and coverage only with mask/box), `histogram`, `check.dust` (`hint` optional), `components`.
- `where`/`sortBy` validate against `inputs[0].Fields`; `sortBy` of a string field is `E_ARG_TYPE`; `only`/`without` accept items (anything with `id`) and lists of them; `cv.sheet` list items with both centre and box are centred on `cx, cy` and labelled with their `id`.
- Prelude: one shared `ITEM_PROTO` with non-enumerable throwing accessors for the removed names, set on awaited items; registered with the binding (`__itemProto`) so data-lambda items (`filter(r => r.box…)`) get it too; `jsError` turns the thrown cv error back into a `graph.Error`. The data-lambda hint is generated from `GeometryFields`. `__fieldNames` reads `DocFields` and `Produces`. `cv.help("Regions")` lists the fields.
- Docs: masks-and-regions (item description and example, passing an item as a box), recipes, lambdas, nodes, units, guidebook concepts and recipes.
- Tests: `pkg/cv/ops/schema_test.go` (`TestSchemaProperty` over the registry: every Regions/Value-object op must be in a call table or the scalar list; keys equal the schema, in order, and every field is accepted by `where`/`sortBy`; `TestRegionFieldsFollowTheReceiver`; `TestGradientTakesPointsOrEdges`), `pkg/cvsession/measure_test.go` (`TestRegionItemsAreFlat`, `TestRegionItemsAsArguments`), updated `regions_test.go`, `m3_test.go`, `sessions_test.go`.

### Why
- Guide §6 items 1, 2, 3, 10; §7.5, §7.6; Phase 1 of §12; hand-over open item 2 (a pi run was refused `only({ id: 3 })`).

### What worked
- The removed-field accessors caught every doc example that still read `x.box` on the first docs-test run ("Region items have no field \"box\" … fix: '[r.x, r.y, r.w, r.h], or pass the item itself where a box is expected'"), which is exactly what the guide's risk section expected greps to miss.
- GCV-001 had already made `box`/`point` accept objects (`checkBox`, `checkPoint`) and `mask.gradient` a `point|enum` union, so flat items work as arguments without binder changes.
- Golden looks unchanged (the full test run includes `TestGoldenLooks` in strict mode).

### What didn't work
- First `jsError` version panicked inside the data lambda: `runtime error: invalid memory address or nil pointer dereference` - `goja.Object.Get` returns nil for a missing property and I called `.ToBoolean()` on it. Fixed with a nil-safe getter.
- `make lint` flagged two test files not gofmt'ed (`pkg/cv/graph/graph_test.go:55:1: File is not properly formatted (gofmt)`); fixed with `gofmt -w`.

### What I learned
- `toJS` sorts object keys, so schema order needs an ordered type (`graph.Record`) rather than a map.
- `check.dust` adds `hint` only when there is dust, so "keys equal the schema" needs optional fields; `FieldSpec.Optional` states it (also used later for sample rows outside the frame).

### What was tricky to build
- Telling the agent where a missing field comes from: `Validate` sees only args and inputs, not the registry. `UnknownFieldError` records the field in an unexported `Error.field`; `Builder.call` passes every validation error through `Registry.producerHint`, which looks the field up in all ops' `Produces` and adds "score comes from img.detect.specks()." plus that call as the fix.
- Keeping throwing accessors from becoming aliases or noise: they live on one prototype (non-enumerable), so `Object.keys`, `JSON.stringify`, spreads and the YAML renderer see only the flat fields; `"box" in item` is true but reading it throws.

### What warrants a second pair of eyes
- `OpDef.Produces` is a map (field → call) instead of the guide's `[]string`: the map gives the hint a literal fix. Recorded as a deviation.
- Region items now carry a non-Object prototype; code that checks `Object.getPrototypeOf(x) === Object.prototype` on items would see a difference.

### What should be done in the future
- N/A (Phase 2 removes the 100-item cap next).

### Code review instructions
- Start at `pkg/cv/graph/schema.go`, then `node.go` (`internNode`, the Validate call), `regions.go` (`Data`, `Schema`), `pkg/cv/ops/schema.go`, `ops/regions.go` (`validatePredicate`, `sortBy`, `idSet`), `pkg/gojamod/cv/prelude.js` (`ITEM_PROTO`), `fields.go`, `binding.go` (`CallEach`).
- `go test ./pkg/cv/ops -run 'Schema|RegionFields|Gradient|RegionPredicate' -v`; `go test ./pkg/cvsession -run RegionItems -v`.

### Technical details
- Example: `img.mask.luminance({ above: 0.75 }).components().where({ score: { gte: 0.5 } })` → `E_UNKNOWN_OPTION: where: these regions have no field "score"; they have id, x, y, w, h, cx, cy, area, aspect.` hint `score comes from img.detect.specks().`, fix `img.detect.specks()`.
- Item keys of `detect.specks`: `id,x,y,w,h,cx,cy,area,aspect,contrast,localNoise,sharpness,score,meanL,polarity`.

## Step 2: Phase 2 - no item cap

Awaited Regions now contain every item. The 100-item `SummaryLimit` and the `truncated` flag are gone; a safety limit
(`MaxRegionItems = 100000`) turns a pathological mask into `E_TOO_MANY` with a literal fix instead of a huge JavaScript
array. The caps that protect the model's context stay in the YAML renderer, and their notes now point to data out.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide Phase 2: remove the cap, add the safety limit, point truncation notes at `cv.json`.

**Inferred user intent:** JavaScript that iterates `items` (the colleague's `mem.hot`) must never silently see a prefix.

**Commit (code):** e5adb68 - "GCV-003 Phase 2: awaited Regions have every item; MaxRegionItems safety limit (E_TOO_MANY); truncation notes point to data out"

### What I did
- `graph.RegionsValue.Summary()` returns `(map, error)`: every item, or `E_TOO_MANY: 100001 regions; await at most 100000.` with fix `regions.sortBy("area").top(1000)   // or await regions.count()`. New code `graph.ETooMany`.
- `binding.evaluate` rejects the promise with that error (with the node path); `resultToJS` returns an error.
- `pkg/cvsession/yaml.go`: "… N more (return cv.json(x) for all)" on lists, "(… more characters; return cv.json(x) for all)" on strings; the CUT note adds "return cv.json(x) or cv.csv(x) for complete data".
- Docs: nodes topic says awaited Regions hold every region.
- Tests: `TestAwaitedRegionsHaveEveryItem` (a 150 × 150 PNG of isolated dots: 5625 regions, all awaited, the YAML list note), `TestAwaitingTooManyRegionsIsAnError` (Go level, 100001 items), session tests no longer expect `truncated`.

### Why
- Guide §6 item 4, §7.7, Phase 2.

### What worked
- Gate green on the first run (`build=0 vet=0 test=0 lint=0 logcopter=0`). I now run the gate through a script that prints each status and exits non-zero on any failure (`/tmp/gate.sh; echo gate=$?`), so no pipe can hide a red step.

### What didn't work
- Two session tests asserted the old key list: `text does not contain "keys: 'count,frame,items,kind,scale,truncated'"`; updated.

### What I learned
- The 100 000-item limit is checked on the evaluated value, so `await regions.count()` stays cheap and valid for any count.

### What was tricky to build
- Nothing beyond threading an error out of `resultToJS`, which runs inside the settle callback on the owner goroutine: the rejection must be built there (`errorObject(vm, err)`), not on the evaluation goroutine.

### What warrants a second pair of eyes
- The YAML notes mention `cv.json`/`cv.csv`, which only exist from Phase 7 on; within this stacked PR the final state is consistent.

### What should be done in the future
- N/A

### Code review instructions
- `pkg/cv/graph/regions.go` (`MaxRegionItems`, `Summary`), `pkg/gojamod/cv/binding.go` (`evaluate`, `resultToJS`), `pkg/cvsession/yaml.go`.
- `go test ./pkg/cvsession -run AwaitedRegions -v`; `go test ./pkg/cv/graph -run TooMany -v`.

### Technical details
- Memory: one awaited item is a JavaScript object of ~9-15 numbers; 100 000 items is tens of MB, the bound the limit sets.

## Step 3: Phase 3 - probe kernel and the measurement ops

Phase 3 adds the numbers the colleague's agents back their decisions with: `img.sample(points, { radius })`,
`img.measure(points, { coreRadius, ringRadii })` (core versus ring: step, spread, snr, channel steps, stepChroma,
ringCoverage) and `img.profile(a, b, { width })`. All run on one probe kernel that reads a small window per point and
weights pixels with the coverage rule of `mask.ellipse`, so `measure` returns what the helper `mem.probe` computed
with two masks and two whole-image `stats` calls.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide Phase 3 (§3.1-§3.3, §7.2-§7.4 for points, §9.1, §13.2) plus open items 3 (measurement ops on GCV-001's types) and 7 (stepChroma, which the guide defines in §3.3).

**Inferred user intent:** Replace `mem.probe` and its 56 `stats` calls per hot list by one native op with the same numbers.

**Commit (code):** 5ddd78a - "GCV-003 Phase 3: probe kernel (EllipseCoverage, core/ring, bilinear) and the sample, measure, profile ops"

### What I did
- `kernels.EllipseCoverage(i, j, cx, cy, rx, ry) float32`: the 4 × 4 rule extracted from `EllipseMask`, which now calls it (rx ≠ ry still works).
- `pkg/cv/kernels/probe.go`: `annulus` selects pixels with weight `cov(r2) − cov(r1)` in row-major order and records the weight inside the buffer and of the whole shape; `measureSel` measures a selection by gathering the gray plane into an n × 1 buffer and calling `StatsWeighted`, and per-channel means with the same sum order; `SamplePoints`, `ProbeCoreRing`, `Bilinear`.
- `StatsWeighted` is the exported function itself (the `statsWeighted` wrapper is gone, guide §6 item 6).
- `pkg/cv/ops/measurement.go`: ops `sample`, `measure`, `profile` (Value results; `Space: linear`; `analysisScale` 1; `gray: rec709|average`). Points are `Regions|point[]`: the Regions alternative exists only so `validatePoints` can refuse it with the fix `regions.measure(img, { ring: true })` (or `{ stats: ["mean"] }` for sample). Ring radii are checked (`coreRadius < r1 < r2`, fix with the default ring); a profile is at most 4096 samples (fix with a larger step).
- Schemas: rows `cx, cy, coreMean, ringMean, step, spread, snr, stepR, stepG, stepB, stepChroma, ringCoverage` (channel fields optional unless `channels` is given: channel counts are not node statics), `outside` optional; sample rows `cx, cy, meanL, meanR, meanG, meanB, area`; the profile object `from, to, step, width, t, cx, cy, meanL, meanR, meanG, meanB`.
- Points outside the frame: a row with `outside: true` and null values, and an `OUTSIDE` evaluation-time warning naming the rows.
- `binding.resultToJS` orders awaited Value objects and rows by the node's schema (`inSchemaOrder`), so rows read `cx, cy, …` instead of alphabetically.
- Synonyms: ops `probe`, `ring`, `contrastAt` → measure, `valueAt`, `pixel`, `at` → sample, `lineProfile`, `line`, `trace` → profile; removed `measure → stats`; options `r`/`radius` → coreRadius (after sigma, radius), `core`, `annulus`, `background`, `ring`, `ringRadius` → ringRadii, `thickness` → width, `fields` → columns, `bits` → depth.
- `measure`'s Doc states the helper mapping.
- Tests: `kernels/probe_test.go` (`TestEllipseCoverageIsEllipseMask` bit for bit over 50 random ellipses, `TestProbeEqualsStatsWeighted` within 1e-9 including corner points and ringCoverage, `TestSampleRadiusZeroIsThePixel`, `TestBilinearProfileRecoversEdgeSigma`), `cvsession/measure_test.go` (`TestMeasureMatchesTheHelper` on 15 points of the planted scan through `testdata/probe.js`, `TestMeasurementOps`), registry-walk and schema-property entries.

### Why
- Guide §6 item 6, §7.1-§7.4, §9.1, §10, Phase 3; open items 3 and 7.

### What worked
- Measuring a selection by handing the gathered pixels to `StatsWeighted` (instead of re-implementing weighted means and quantiles) made the probe equal to `stats({ mask: mask.ellipse })` on the first run: the selected pixels arrive in the same row-major order, so the sums and the sort of the weighted quantile see the same sequence.
- `TestMeasureMatchesTheHelper` passed first time: step, channel steps and stepChroma with `gray: "average"`, spread with rec709, all within 1e-6 of the helper on 15 points (12 planted specks, the star, two corner points).

### What didn't work
- `TestMeasurementOps` first asserted `e.meanR > s.meanR` (encoded > linear) at the star, but its red channel is clipped at 1 in both spaces; the test uses blue.
- `TestDiagnostics` (gojamod): `img.meanL` started to hint `"meanL" is a field of the Value returned by profile()` because profile sorts before stats; `UnknownMethod` now prefers Value ops that need no argument (`needsArgs`).
- Rows came back alphabetical (`coreMean,cx,cy,ringCoverage,…`) because `toJS` sorts map keys; fixed with `inSchemaOrder`.

### What I learned
- `mask.minus` is `min(a, 1 − b)`, which equals `a − b` for the helper's rings: the inner and outer edges are 6 px apart, so no pixel is partly inside both. The probe's `cov(r2) − cov(r1)` is therefore the helper's ring weight exactly.
- The helper's `step` uses equal channel weights while its `spread` uses Rec. 709 luminance; one call of `measure` uses one gray plane, so the equivalence holds per field as the guide's §13.2 says.

### What was tricky to build
- Exactness against `stats` without sharing a code path for the whole image: the weighted quantile depends on the order of tied values after `sort.Slice`, which is not stable. The fix was not a stable sort but the same input sequence: gather in row-major order, skip zero weights, and call the same function.
- Refusing Regions with a useful fix: a plain `point[]` argument would make the binder say "wants a list, got a Regions node" with a hint about Value nodes. Declaring `Regions|point[]` lets the union lift the node, and the validator then sees a Regions input and returns `E_WRONG_KIND` with the regions.measure call.

### What warrants a second pair of eyes
- Rows outside the frame keep every required key with null values plus `outside: true` (the guide says "no values"); the property test treats `outside` as an optional field.
- `profile`'s `width` averages only when every across-line position is inside the frame (otherwise the sample is null); a line along the image edge with width 3 is all null.

### What should be done in the future
- N/A

### Code review instructions
- `pkg/cv/kernels/probe.go` (`annulus`, `measureSel`, `probeOne`), `pkg/cv/kernels/mask.go` (`EllipseCoverage`), `pkg/cv/ops/measurement.go`, `pkg/gojamod/cv/binding.go` (`inSchemaOrder`).
- `go test ./pkg/cv/kernels -run 'Ellipse|Probe|Sample|Bilinear' -v`; `go test ./pkg/cvsession -run 'Measure' -v`.

### Technical details
- Mapping (in `measure`'s Doc): `mem.probe(x, y, r)` = `img.measure([[x + 0.5, y + 0.5]], { coreRadius: r + 0.5, ringRadii: [r + 3.5, r + 9.5], gray: "average" })`.
- `img.measure([[200.5, 46.5], [-20, 4]], { coreRadius: 2.5 })` → warning `OUTSIDE: measure: 1 of 2 points lie outside the frame [0, 0, 256, 192] (rows 1 have outside: true and null values).`

## Step 4: Phase 4 - regions.measure

`regions.measure(image, opts)` measures a region set on another image of the same frame and returns the same regions
(same ids, same order) with new fields: statistics over each region's pixels (`{ stats: ["max"] }` → `maxL`) and the
core/ring measure at each centroid (`{ ring: true }` → `step, spread, snr, stepChroma, …`). With it the colleague's
where-to-zoom list is one chain: DoG blobs, measured on the DoG for their peak and on the scan for how much they stand
out, ranked by `snr`.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide Phase 4 (§3.4, §7.4 regions part, §9.2) and the `meanMask` replacement of §6 item 3.

**Inferred user intent:** Replace `mem.hot`'s per-blob loop of `mem.probe` calls with one op that keeps the region ids.

**Commit (code):** 66318c0 - "GCV-003 Phase 4: regions.measure(image, { stats, channels, ring, coreRadius, prefix }) and Region coverage"

### What I did
- `pkg/cv/ops/regionmeasure.go`: op `measure` on Regions with explicit input `Image|Mask` (so `components().measure(mask, { stats: ["mean"] })` gives what `meanMask` gave), `Space: linear`, `analysisScale` 1. Options `stats` (string list validated with did-you-mean and synonyms `maximum → max`, `stddev → std`, …), `channels`, `ring` (`true` or `[r1, r2]`), `coreRadius`, `prefix` (identifier), `gray`.
- Fields: the receiver's plus `prefix + Stat + Channel` and the prefixed core/ring names; `E_FIELD_EXISTS` when a name is taken (fix with a prefix); `E_FRAME` at build time (node frames) and at evaluation; `Produces` lists every unprefixed field with the call that adds it.
- Per-region weights: the region's own pixels when the regions were found on the image's raster, else `RegionsValue.Coverage(r, raster)`, a new method that shares `Rasterize`'s overlap loop (`overlap`, callback per target pixel).
- `kernels.StatsOf` (weighted mean, std, median, p10, p90 and min/max as the 0 and 1 quantiles, through `StatsWeighted`), `kernels.GrayPlane`, `kernels.ProbePoint`.
- Tests (`pkg/cv/ops/regionmeasure_test.go`): stats equal `img.stats({ mask: regions.only([id]).toMask(), percentiles: [0, 0.1, 0.9, 1] })` for each of six regions found at `analysisScale` 1 and 0.5 (within 1e-9); ring fields equal `img.measure` at the centroid with the default core radius (1e-12); prefixed names in order; ids kept and the receiver's items untouched; `sortBy("linSnr")`; errors (`E_FIELD_EXISTS`, no stats or ring, `maximum`, `E_FRAME` against a crop, the snr producer hint on specks); a mask measured over its components.

### Why
- Guide §6 item 3, §7.4, §9.2, Phase 4.

### What worked
- The guide's hot-list recipe runs as written on the planted scan: 8 blobs, top one `{ id: 4, cx: 43.35, cy: 67.02, dogMaxL: 0.1605, step: 0.0937, spread: 0.014, snr: 6.7159, stepChroma: 0.0982, ringCoverage: 1 }`.
- Factoring `Rasterize`'s inner loop into `overlap` kept its arithmetic order, so overlay golden looks did not move.

### What didn't work
- `{ stats: ["maximum"] }` produced no hint (`Closest` finds nothing within its edit distance); added `statSynonyms`.
- I first declared `stats` as `enum:mean,…[]`: the binder's `enum:` case comes before the `[]` case, so the list type would have been read as an enum whose last value is `p90[]`. `stats` is `string[]` validated by the op.

### What I learned
- Default core radius uses π where the helper used 3.14; they differ only at area 95 (guide §7.4). The centre is the exact centroid, not the helper's `round(centroid) + 0.5`.

### What was tricky to build
- Ring channels versus stats channels: `channels` means "stats per channel too" for stats, but the ring's channel steps are wanted by default (stepChroma is the neutrality cue). The rule: stats per channel only with `channels: true`; ring channel steps on colour images unless `channels: false`.

### What warrants a second pair of eyes
- With an explicit `ring: [r1, r2]` and the per-region default core, a large region whose core exceeds r1 fails the whole evaluation with `E_ARG_RANGE` naming the ring rule.
- `regionWeights` decides "same raster" from sizes and the buffer origin; a crop's regions measured on the crop's image take the fast path.

### What should be done in the future
- Open question 1 of the guide (rings shaped like the region for hairs) remains open.

### Code review instructions
- `pkg/cv/ops/regionmeasure.go` (`validateRegionMeasure`, `runRegionMeasure`, `regionWeights`), `pkg/cv/graph/regions.go` (`overlap`, `Coverage`), `pkg/cv/kernels/probe.go` (`StatsOf`).
- `go test ./pkg/cv/ops -run 'RegionMeasure|Schema' -v`.

### Technical details
```js
const dog = img.toGray("average").dog(1, 4, { polarity: "bright" })
const hot = dog.levels({ black: 0, white: dog.stats().p99.mul(2) }).mask.luminance({ above: 0.99 }).components({ minArea: 4 })
const ranked = hot.measure(dog, { stats: ["max"], prefix: "dog" }).measure(img, { ring: true }).sortBy("snr").top(30)
```
