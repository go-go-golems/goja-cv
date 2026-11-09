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
