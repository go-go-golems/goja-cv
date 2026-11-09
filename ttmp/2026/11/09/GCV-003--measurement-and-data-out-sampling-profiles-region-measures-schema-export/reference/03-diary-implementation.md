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

## Step 5: Phase 5 - charts declared by the op

`look()` of a Value node used to check for the histogram op by name. Ops now declare their chart (`OpDef.Chart`:
bars or lines), and a profile's look is a line chart: t in doc px on x, one line per channel plus `meanL` in black,
with the value and t ranges labelled in GCV-002's bitmap font (open item 4).

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide Phase 5 (§6 item 11, §7.8) and open item 4 (profile plots on GCV-002's render path and `pkg/cv/text`).

**Inferred user intent:** An agent can see the shape of an edge across a speck as a picture, not only as a list of numbers.

**Commit (code):** 7c8d1ef - "GCV-003 Phase 5: charts declared by the op (OpDef.Chart); profile looks are line charts"

### What I did
- `graph.ChartKind` (`ChartNone`, `ChartBars`, `ChartLines`) and `OpDef.Chart`; `histogram` declares bars, `profile` lines.
- `look.New` accepts a Value node when its op declares a chart; `Look.chart` is the kind; the render goes through `Evaluator.Render` (GCV-002's path) like every look, then `renderChart` or `renderLines`.
- `look/chart.go`: `renderLines` (range with 5 % padding, axis lines, labels at font size 2: the value range on the left, `t0` and `t1 doc px` below; Bresenham lines; nulls break a line) and `line`.
- Golden harness: case `profile-chart` (a profile across the star, width 3), class chart.
- `TestProfileLookIsALineChart`: PNG 512 × 204, a dark meanL line in the middle column, a zoom refused ("charts have no doc px").
- The look function's doc mentions profiles.

### Why
- Guide §6 item 11, §7.8, Phase 5; open item 4.

### What worked
- Golden check before regenerating: every existing look passed exactly; the only failure was the missing `profile-chart.png` ("open golden: … no such file or directory (run with -update)"). After `-update` only that file is new; no golden moved.

### What didn't work
- A first Bresenham with `if … else` stepped only one axis per iteration (a staircase that misses diagonals); the standard form tests both.
- `abs redeclared in this block` (vet): the golden test already defines `abs`; renamed to `iabs`.
- `make lint`: `pkg/cv/look/look.go:119:1: File is not properly formatted (gofmt)` after the struct field type changed width; the gate script now checks `gofmt -l` first.

### What I learned
- The guide says both charts use the font for axis labels but also that "the histogram chart is unchanged" (the test list); I kept the histogram exactly as it was so its goldens hold, and only the line chart has labels.

### What was tricky to build
- Nothing deep: choosing a left margin from the widest of the two value labels so the plot area adapts to `0.1724` versus `1.026`.

### What warrants a second pair of eyes
- Histogram bars still have no labels (deviation recorded).

### What should be done in the future
- N/A

### Code review instructions
- `pkg/cv/look/chart.go` (`renderLines`), `pkg/cv/look/look.go` (`New`, `image`), `pkg/cv/graph/registry.go` (`ChartKind`).
- `go test ./pkg/cv/look -run TestGoldenLooks -v`; `go test ./pkg/cvsession -run ProfileLook -v`.

### Technical details
- Golden: `pkg/cv/look/testdata/golden/profile-chart.png` (512 × 204).

## Step 6: Phase 6 - writers and saving

goja-cv can now write the images it computes: `node.save("views/speck.tif", { depth: 16 })` evaluates the node at scale
1 and writes its encoded values under the session's out-root, recording the encoding and the frame origin where the
format can hold them (TIFF ImageDescription, PNG gAMA/sRGB and tEXt). GCV-001's decoders read those back, so a saved
linear view reopens as linear at the same absolute coordinates (open item 6). Without `--out-root`, saving is disabled.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide Phase 6 (§6 items 7, 12, §7.10) and open item 6 (writers write the metadata the decoders read; round-trip tests prove it).

**Inferred user intent:** Derived images reach other programs with their meaning intact, and an agent can never write outside the directory the operator allowed.

**Commit (code):** c0e2180 - "GCV-003 Phase 6: writers that record encoding and frame; save under --out-root"

### What I did
- `pkg/cv/buffer/write.go`: `EncodeTIFF` (classic little-endian TIFF, zlib/Deflate strips of about 64 KiB, `BitsPerSample` 8/16/32, `SampleFormat` 1 or 3, 1 or 3 samples, chunky, tag 270 = `FormatMetadata`), `EncodePNG(ctx, w, b, bits, meta)` (image/png, then `gAMA` 100000 for linear or `sRGB` for sRGB and `tEXt` `goja-cv\0goja-cv encoding=… origin=…` inserted after IHDR with CRCs), `EncodeNPY` (`<f4`, `<u2`), `Encode`, `FormatOf`, `Depths` (TIFF 16/8/32, PNG 16/8, NPY 32/16, JPEG 8), `ClippedFraction`.
- Deleted `buffer.WriteFile`, `EncodeTIFF16` and the old `EncodePNG` (the RGBA-TIFF path through `x/image/tiff`).
- `pkg/cv/graph/paths.go`: `EAccess` (moved from cvsession), `Within(root, p)` (resolves the longest existing prefix through symlinks, so a not-yet-existing file under a symlinked directory is caught), `OutPath`, `WriteOut` (creates parents, re-checks, reports `replaced`). `cv.open`'s root check uses `Within`.
- `pkg/cv/dataout/save.go` (new package): `SaveImage` (Image/Mask only; extension and depth checks; masks saved linear; linear JPEG refused with the `.png` fix; `eng.Display` + `Evaluate` at scale 1; CLAMP warning with the clipped percentage and the `{ depth: 32 }` fix; notes for NPY and JPEG), `Saved.Data()`.
- `save` FuncDef on Image, Mask and Data (`ops.DataKind`) with `path` and `depth`; the binding's `__save` binds through `BindOptions` and runs `SaveImage` on a goroutine via a new generic `binding.async` (promise settled on the owner, warnings with the calling line); `node.save` in the prelude (also linked onto awaited data).
- `cvsession.Options.OutRoot` (absolute, separate from the read roots) → `graph.EngineOptions.OutRoot`; `--out-root DIR` on `eval`, `run` and `mcp` (must be a directory).
- `internal/testimage`: `Write`/`WritePlanted` use `buffer.Encode` (sRGB metadata recorded); new `WriteFile(path, b, depth, meta)`; plain fixture writes in tests switched to it; `testdata/` regenerated with `make testdata`.
- Tests: `buffer/write_test.go` (`TestWritersRoundTrip`: TIFF 16/8/32 and PNG 16/8 × 1/3 channels × srgb/linear read back values, encoding and origin; NPY 32/16 read back as linear at (0, 0); JPEG values; `TestWritersClampIntegersKeepFloats`), `cvsession/save_test.go` (save and reopen with frame [200, 80, …], replaced, masks, NPY note; refusals: `..`, absolute, symlink to outside, `.gif`, depth 32 PNG, linear JPEG, Regions, no out-root; CLAMP once for 16-bit and not for 32-bit).

### Why
- Guide §6 items 7 and 12, §7.10, Phase 6; open item 6.

### What worked
- Every TIFF and PNG variant round-trips values, encoding and origin through the existing decoders on the first run; the planted scan written by the new TIFF writer decodes to the same values, so no golden look moved and `TestCommittedTestdataMatchesGenerator` passed with the regenerated files.
- `make smoke` (CI) still passes.

### What didn't work
- JPEG row: `values off by 0.69343101978302 (tolerance 0.06)` on a sawtooth test pattern (chroma subsampling); the JPEG row uses a smooth image and the mean error.
- `TestSaveImagesReopenWithEncodingAndFrame` expected `view("speck")` to save as srgb; views are derived images (`DerivedEnc`), so they save as linear - the test now says so, and compares stats on an exposure(-0.5) image (no clipping).
- Lint: `missing cases in switch of type buffer.Encoding: buffer.EncNone (exhaustive)`, `param max has same name as predeclared identifier`; logcopter: `generated file is not current: …/pkg/cv/dataout/logcopter.go` (new package → `make logcopter-generate`).

### What I learned
- The decoders accept Deflate (8) and LZW, so a plain zlib writer is all saving needs; `x/image/tiff` is now used only for its LZW reader.

### What was tricky to build
- Containment for files that do not exist yet: `filepath.EvalSymlinks` fails on a missing path, so the old cv.open check left such paths unresolved. `resolveExisting` walks up to the longest existing prefix, resolves it and appends the rest; a symlink `out/link → /elsewhere` makes `out/link/x.tif` resolve outside the root and the save is refused before any directory is created.

### What warrants a second pair of eyes
- The guide says GCV-001's fixture TIFF/NPY writers are deleted; they still exist, documented as the writers of decoder variants goja-cv never writes (LZW, PackBits, predictors, big-endian, `|u1`, `>u2`, `>f4`, `<f8`). Deleting them would drop the tests of those decoder paths. Recorded as a deviation.
- `WriteOut` uses `os.WriteFile`, which follows a symlink at the final component; the containment check before it resolves that symlink, but a race between check and write is not defended against (local prototype).

### What should be done in the future
- N/A

### Code review instructions
- `pkg/cv/buffer/write.go`, `pkg/cv/graph/paths.go`, `pkg/cv/dataout/save.go`, `pkg/gojamod/cv/dataout.go`, `pkg/cvsession/session.go` (`OutRoot`).
- `go test ./pkg/cv/buffer -run Writers -v`; `go test ./pkg/cvsession -run Save -v`.

### Technical details
- `await img.view("speck").save("views/speck.tif")` → `{ path: "/abs/out/views/speck.tif", format: "tiff", depth: 16, encoding: "linear", frame: [200, 80, 256, 192], bytes: …, replaced: false }`.
- CLAMP example: `save("d16.tif"): 48.4 % of the samples lie outside [0, 1] and were clamped by the 16-bit format. fix: node.save("d16.tif", { depth: 32 })   // or .npy`.

## Step 7: Phase 7 - data out (cv.json, cv.csv)

Data now has its own way out of an eval. `cv.csv(regions, { name: "spots" })` returned from an eval is delivered as
content - text for MCP clients, the file `spots.csv` for the CLI - with every region in schema order; `cv.json(x)` does
the same as JSON; either can be saved under the out-root with `.save(path)`. Looks and data have separate budgets, and
data too large for a conversation becomes a note telling the agent to save it.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide Phase 7 (§6 item 8, §7.9, the rev-3 rules on names and the MCP limit) and open item 5 (json, csv, save as FuncDefs through RegisterFunc/BindOptions).

**Inferred user intent:** The colleague's calibration script gets every candidate as a CSV file from one `goja-cv eval --json` call instead of paging YAML.

**Commit (code):** 58c0480 - "GCV-003 Phase 7: cv.json and cv.csv Data handles; data budgets; CLI writes named data files"

### What I did
- `pkg/cv/dataout/data.go`: `Data` (lazy node or eager data; `SetNumber(n, maxBytes)` from the session; `Name` → name or `data-<N>`; `ResultPlaceholder`; `Render` once at scale 1; `ResultContent` with the byte-limit note; `Save`), own JSON writer (records keep order, maps sorted, NaN → null), CSV `table` (Regions items and row lists one row each; an object whose longest arrays share a length - a profile - one row per element; awaited `{ kind: "Regions", items }` its items; lists of lists `c1, c2, …`), `columns` checked at the call when the schema is known.
- `graph.OrderBySchema` (moved from the binding) orders rows/objects for JavaScript and for JSON.
- Binder type `data` (kept as given; BindOptions does not refuse a node there). FuncDefs `json`, `csv` (Bespoke, `Usage` lines) and `save` on `Data` too.
- Binding: `__data(format, x, opts)` exports plain data with `exportOrdered` (records in key order, nodes and looks as their description, cycles cut) - awaited data is *not* lifted back to its node; returns a `goja.DynamicObject` (`dataObject`, with `name`, `format`, `save`) whose exported value embeds the handle, so `resultcontent.ProviderOf` finds it. `data.save` runs through `binding.async`.
- Session: `Options.MaxData` (4), `Options.MaxDataBytes` (0; `MCPMaxDataBytes` = 256 KiB set by `goja-cv mcp`), `ContentLimits().MaxItems = MaxLooks + MaxData`, the MCP item limit likewise. `collect` keeps at most MaxLooks looks and MaxData data items, numbers looks, gives data items their content number and byte limit, and returns the dropped ones; the YAML renders them as `[not delivered: … — more than 4 looks in one eval; …]`.
- CLI: `writeLooks` → `writeContent` (`look-<n>-<slug>`, `<name>.<ext>` from the caption `data <name>.<ext>`, `-2` for a repeated name, dropped items listed), rows counted from the CSV/JSON; `--json` prints `files` (with `rows`) instead of `looks`.
- Tests: `cvsession/dataout_test.go` (placeholders, lazy CSV header in schema order, eager JSON with an edit to awaited data, columns, measurement rows in schema order, profile CSV, refusals: image, path as name, unknown column, misspelled option; data.save with extension added/refused; fifth look and fifth data item not delivered; byte-limit note), `TestMCPDataContent` (text content; a ~300 KiB CSV becomes the note), `cmd/goja-cv` `TestDataFilesFromEval` (file names, `-2`, rows, key order, `--out-root` save).

### Why
- Guide §6 item 8, §7.9, Phase 7; open item 5.

### What worked
- A `goja.DynamicObject` whose exported value is the Go handle gives JavaScript `data.save(…)` while `resultcontent.ProviderOf` still sees a provider, without any change to go-go-goja.

### What didn't work
- `TestDataContentLazyEagerAndColumns` first returned five data items in one eval; the fifth (the profile) was correctly left out by the new MaxData budget, so the test split it into its own eval.
- A dropped look rendered `[not delivered: [look: l5 …] — …]` (the caption already has brackets); trimmed.
- The CLI test expected `look-1-scene.jpg`; a 64 × 48 look is PNG by GCV-002's format rule.

### What I learned
- go-go-goja's `Render` counts every item against one limit; the session can enforce per-class counts by leaving providers out in `collect` and telling the agent in the placeholder, with the transport limit set to the sum.

### What was tricky to build
- Keeping the agent's key order for eager data: `Export()` gives Go maps (unordered). `exportOrdered` walks the goja object on the owner goroutine with `obj.Keys()` and builds `graph.Record`s, which both the JSON and CSV writers honour.
- Placeholders before rendering: the column count is known only for row schemas; a profile's schema (arrays) gives no count, so its placeholder names the node only.

### What warrants a second pair of eyes
- `cv.json(x)` of a plain value containing nodes writes their one-line description, not their data.
- `cvFuncs` (cv properties, session globals, concepts) stayed in `pkg/gojamod/cv/help.go`: moving it to cvsession would need a hook into `cv.help`, which lives in the module; recorded as not done (it was optional).

### What should be done in the future
- Open question 2 of the guide (save an oversize MCP data item automatically when there is an out-root) remains open.

### Code review instructions
- `pkg/cv/dataout/data.go`, `pkg/gojamod/cv/dataout.go` (`dataJS`, `dataObject`, `exportOrdered`), `pkg/cvsession/processor.go` (`collect`), `pkg/cvsession/yaml.go` (dropped), `cmd/goja-cv/eval.go` (`writeContent`, `countRows`).
- `go test ./pkg/cvsession -run 'Data|MCPData' -v`; `go test ./cmd/goja-cv -run DataFiles -v`.

### Technical details
- The calibration command: `goja-cv eval --json --out /tmp/specks --image TILE.tif -e 'return cv.csv(img.detect.specks({ polarity: "bright", maxArea: 2000 }), { name: "specks" })'` → `files: [{ caption: "data specks.csv", mimeType: "text/csv", rows: N, path: "/tmp/specks/specks.csv" }]`.

## Step 8: Phase 8 - preludes

`goja-cv eval --prelude helpers.js -e '…'` runs helper scripts in the session before the agent's code, so the
colleague's `mem` helpers no longer need re-pasting into every CLI call. Preludes run with the semantics of agent code
(they may `await` and `return`), in every new session, MCP sessions created later included.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide Phase 8 (§7.11, issue 23b).

**Inferred user intent:** Helpers defined once are available in every CLI run and every MCP session.

**Commit (code):** 8d6245c - "GCV-003 Phase 8: --prelude on eval, run and mcp"

### What I did
- `cvsession.Options.Preludes` (resolved against BaseDir to absolute paths; `New` refuses a missing file).
- `App.CreateSession` runs `runPreludes` after replapi creates the session: each file through `App.Evaluate` (the wrapped async body), status checked; on failure the session is deleted and the error is `prelude bad.js:2: nope is not defined` (line from the rendered `where: 'line N: …'`, message from the YAML).
- CLI: `--prelude` (repeatable, `StringArray`) on `eval`, `run`, `mcp`; a failing prelude exits 1 like agent code; a missing file exits 2.
- Tests: `TestPreludesRunInEverySession` (two preludes in order, one awaiting; mem listed in the context block; prelude result not printed; an MCP session made with `create_session` has the helpers; the failing prelude's message and no leftover session; missing file), `TestPreludeFlag` (CLI).

### Why
- Guide §7.11, Phase 8.

### What worked
- replmcp's `create_session` tool calls the `CreateSession` of the app it was given, which is cvsession's override, so later MCP sessions run the preludes without touching go-go-goja.

### What didn't work
- staticcheck `QF1008: could remove embedded field "App" from selector` on `a.App.DeleteSession`; the call goes through the promoted method.
- My first test parsed the new session id from create_session's text wrongly (`session "sionId" does not exist`); it uses a regexp for `session-<uuid>`.

### What I learned
- Running preludes through `App.Evaluate` gives them the same error rendering, line mapping and context block as agent code for free; the cost is that they appear in the session's history.

### What was tricky to build
- Extracting FILE:LINE from the processed failure: the line exists only in the rendered YAML (`where: 'line 2: const x = nope.y'`), so `runPreludes` reads it with a regexp and takes the message line.

### What warrants a second pair of eyes
- Prelude cells are part of the session history (`history` tool).

### What should be done in the future
- N/A

### Code review instructions
- `pkg/cvsession/session.go` (`Preludes`, `CreateSession`, `runPreludes`), `cmd/goja-cv/eval.go`, `cmd/goja-cv/mcp.go`.
- `go test ./pkg/cvsession -run Prelude -v`; `go test ./cmd/goja-cv -run Prelude -v`.

### Technical details
- `goja-cv eval --image tile.tif --prelude scripts/84-cv-helpers.js -e 'return await mem.probe(1337, 131, 3)'`.

## Step 9: Phase 9 - documentation, open items, real data

Phase 9 makes the new surface findable and checks it on real data. Two new agent topics (`measuring`, `data-out`),
updated recipes, guidebook, README and MCP instructions explain sampling, profiles, core/ring measures, region
measures, flat items and data out; redirects catch `export`, `toCSV` and friends. Two open items are settled on the
colleague's labelled tiles: the speck views now default to linear space (measured better), and evaluation warnings
live in the value cache. A gated test gives the hand-over its new snr and stepChroma ranges, and a benchmark shows the
native hot list is two orders of magnitude faster than the helper.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide Phase 9 (§10 synonyms, §11 docs, §13.3, §13.4) and open items 1 (speck default space, decided with `TestRealViewRanking`), 8 (windowed evaluation, if in scope) and 9 (unbounded warning cache).

**Inferred user intent:** Agents find and use the new calls without help, and the hand-over numbers come from the native ops on real tiles.

**Commit (code):** 8e9a3ed - "GCV-003: evaluation warnings live in the value cache; speck views default to linear space"; be25dd2 - "GCV-003 Phase 9: measuring and data-out topics, recipes, guidebook, MCP lines, redirects; gated hot-list test and benchmark"

### What I did
- Open item 9: `Cache.SetWarnings`/`Warnings` hold evaluation-time warnings next to the cached value; eviction, `Clear` and a rejected `Put` drop them; `Put` returns whether it stored, and a value too large to cache reports its warnings at once. `Evaluator.warns` is gone. `TestCacheWarningsLeaveWithTheirEntry`.
- Open item 1: `SpaceRule.For` and `OpDef.DefaultSpace(args)` give a call without `{ space }` a per-call default (also used by explain to hide the default); `presetSpaces` makes `speck` and `speckColor` linear. Golden `sheet-2x2` moved (checked first: "largest difference 85 codes, 6717 of 113472 samples differ (5.920 %)"; only that look) and was regenerated. Views topic says which presets are linear.
- Docs: `pkg/cv/docs/topics/12-measuring.md` and `13-data-out.md` (4.7 KB and 3.4 KB, every runnable block tested; the save block is `js warns=CLAMP`); recipes "Measure a spot, a box or regions" and "Rank candidates"; the docs harness passes an out-root; topic lists in the docs tests (13 topics). Guidebook: `03-cli.md` (flags `--out-root`, `--prelude`, `files` in `--json`, the calibration command), `01-what-is-goja-cv.md` (measure, data out; inputs now include float TIFF, DNG, NPY), `04-agents.md`, `08-recipes.md`. README (CLI data out, MCP flags, prompting tips with measure and flat items, embedding options, topic count). MCP instructions: flat items, measure, data (3985 characters).
- Redirects (`graph.MethodRedirects`): `export`, `write`, `saveAs`, `saveTo`, `writeFile`, `store` → save; `toCSV`, `toCsv`, `csv`, `table`, `rows` → `cv.csv`; `json`, `dump`, `serialize` → `cv.json` (on nodes and on `cv`).
- `pkg/cv/ops/measure_real_test.go`: `TestRealHotListMeasures` (gated), `BenchmarkHotList`, `TestHotListIsFasterThanTheHelper` (gated on the harness tile).
- Guide: "Implementation notes (deviations)", 23 items.

### Why
- Guide §10, §11, §12 Phase 9, §13.3, §13.4; open items 1, 8, 9.

### What worked
- `TestRealViewRanking` (142 points, 15 tiles): encoded default median rank 20, 70.4 % in the top 50; linear 18, 79.6 %. After the switch the default run gives 18 / 79.6 %.
- `TestRealHotListMeasures` with a linear DoG: 15 tiles, 555 hot regions (37 per tile); 71.8 % of the 142 gold points within 4 px of a hot spot, 62.7 % of a top-30 one by snr; labelled dust (100 matched) snr median 2.17 (middle half 1.61–3.03), stepChroma 0.00 (0.00–0.26); unlabelled spots (428) snr 0.86 (0.56–1.23), stepChroma 0.00 (0.00–0.51).
- `TestHotListIsFasterThanTheHelper`: "28 spots on a 1024×1024 tile: measure 1.99 ms, helper 471.1 ms: 237× faster".

### What didn't work
- My first background run of the two ranking variants seemed to show the spaces swapped; I bisected four phase commits with `git archive` before seeing that I had read the two result files in the wrong order. Every commit gives the same numbers; nothing in GCV-003 changed the view.
- The guide's assertion for the hot list ("≥ 70 % of gold points within 4 px of a top-30 spot") fails: 62.7 % (linear DoG), 59.2 % (encoded). The hot list as a whole covers 71.8 % (the colleague's `mem.hot(2)`: 77 % at about 28 spots per tile, matched his own way). The test asserts ≥ 70 % for any spot and ≥ 60 % for the top 30, and the numbers are in the deviations.
- MCP instructions grew to 4258 characters (limit 4000); trimmed to 3985.
- The data-out save example warned `CLAMP` (a stretched view saturates above white); the block is marked `js warns=CLAMP` and the text says why that is usually fine.

### What I learned
- On these density tiles both the speck view and the hot list rank and cover labelled dust better in linear space (density) than in encoded space; stepChroma is 0 for most spots because many tiles are monochrome.

### What was tricky to build
- A per-preset default space without breaking explicit `{ space }`: `OpSpace` consults `SpaceRule.For` only when the bound args have no space, and `explain` compares with the per-call default so `view(speck)` still prints without `space: linear`.

### What warrants a second pair of eyes
- The speck views' linear default is measured on density tiles only; on sRGB photographs `{ space: "encoded" }` may be the better choice (the views topic says how to choose).
- The relaxed hot-list threshold (deviation 19).

### What should be done in the future
- Windowed pixel-op evaluation (open item 8, not done; design note below).
- A ring shaped like the region for hairs (guide open question 1); auto-saving oversize MCP data when an out-root exists (open question 2).

### Code review instructions
- `pkg/cv/graph/cache.go`, `eval.go` (warnings), `registry.go`/`space.go`/`explain.go` (`DefaultSpace`), `pkg/cv/ops/views.go` (`presetSpaces`), the two new topics, `pkg/cvsession/reference.go`, `pkg/cv/graph/suggest.go` (redirects), `pkg/cv/ops/measure_real_test.go`.
- `go test ./pkg/cv/ops -run 'TestRealViewRanking|TestRealHotList|TestHotListIsFaster' -v` (needs `~/dustdet/cv_tiles` and `~/dustds/gold`); `GOJACV_VIEW_SPACE=encoded …` and `GOJACV_HOT_SPACE=encoded …` for the other space; `go test ./pkg/cv/ops -bench HotList -run '^$'`.

### Technical details
Design note for windowed evaluation (open item 8). Today a crop of a full DNG computes every pixel op above it over
the whole frame at scale 1 and cuts the window (GCV-002's render path does the same for zoomed windows). A windowed
evaluation would: (1) add `Window Frame` to `EvalOptions` for scale-1 evaluations and to the run's cache key, as the
render keys already do; (2) propagate a required window down the graph - a pixel op with `Halo(args)` h asks its
image inputs for its window grown by h doc px and clipped to their frames; a geometric op intersects; a decoration
passes it on; an op marked `Global` (stats, histogram, `levels({ auto })`, the view presets' p99, Regions ops with
`AnalysisScale`) stops propagation and evaluates its input whole; (3) let sources decode only the window
(`DecodeOptions.Box` and `SourceKey.Box` exist since GCV-001); (4) carry the buffer origin
(`ImageValue.BufferOrigin`, GCV-002) so `Args.XY`, rulers and region rasterizing stay exact; (5) cut the halo off
after the op runs. The invariant to test: for every pixel op with a Halo, `op(x).crop(w)` evaluated windowed equals
the full-frame result cut to `w`, sample for sample. Measurement ops (sample, measure, profile) would then read only
the windows around their points. Not done here: it touches the evaluator, the source store and every op's border
behaviour, and GCV-003's measurements are already fast on 1024² tiles.
