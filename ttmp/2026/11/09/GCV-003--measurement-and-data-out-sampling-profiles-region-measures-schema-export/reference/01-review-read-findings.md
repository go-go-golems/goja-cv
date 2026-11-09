---
Title: Review read findings
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
    - Path: ttmp/2026/11/09/GCV-003--measurement-and-data-out-sampling-profiles-region-measures-schema-export/design-doc/01-design-and-implementation-guide.md
      Note: the guide under review (commit b30efec, unchanged at 72911b1)
    - Path: pkg/cv/kernels/mask.go
      Note: EllipseMask is 4x4 supersampled, not exact coverage (P1-2)
    - Path: pkg/cv/graph/registry.go
      Note: OpDef.Fields []string and Validate(args) signature (P2-1, P2-2)
    - Path: pkg/cv/graph/regions.go
      Note: Rasterize draws a union mask (P2-6); Data/Summary/field lists verified
    - Path: pkg/cvsession/session.go
      Note: ContentLimits caps rendered items at MaxLooks (P2-4)
    - Path: cmd/goja-cv/eval.go
      Note: writeLooks names, --out, no --root on eval/run (P1-3, P2-12)
    - Path: pkg/cv/buffer/io.go
      Note: existing PNG/JPEG/TIFF16 writers the guide does not mention (P1-5)
ExternalSources:
    - /Users/manuel/go/pkg/mod/github.com/go-go-golems/go-go-goja@v0.10.8-0.20261109062017-5e1ba117462e/pkg/resultcontent/files.go
    - /Users/manuel/go/pkg/mod/github.com/go-go-golems/go-go-goja@v0.10.8-0.20261109062017-5e1ba117462e/pkg/resultcontent/resultcontent.go
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/84-cv-helpers.js
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
Summary: "Independent review read of the GCV-003 design guide: 5 P1, 13 P2, 17 P3 findings on concept introduction, correctness against the code, gaps, guideline conformance, implementability and architecture, each with a location and a concrete fix."
LastUpdated: 2026-11-09T17:30:00-05:00
WhatFor: "Fixing the GCV-003 guide before it is handed to an implementer."
WhenToUse: "Read when revising the GCV-003 design guide or when implementing GCV-003 phases 1, 3, 4 and 6."
---

# Review read findings: GCV-003 design and implementation guide

This document reports a cold read of `design-doc/01-design-and-implementation-guide.md` (GCV-003, as committed in
`b30efec` and unchanged at `72911b1`). I read it top to bottom as an intern who knows Go and some JavaScript and
has never seen goja-cv, then checked every `file:line` claim and code excerpt against the goja-cv working tree
and go-go-goja `v0.10.8-0.20261109062017-5e1ba117462e` (the version in `go.mod`), the GCV-001 and GCV-002 guides
where GCV-003 depends on them, the colleague's issue texts (issues 9, 10, 16, 17, 21, 23b) and their helper
`84-cv-helpers.js`.

The guide is well structured, has every section the writing guide asks for, uses concrete numbers in its
core/ring concept, and its main architectural moves (one probe kernel, flat region fields, no data cap, data
as result content, a separate write root) are sound. The problems are of three kinds. First, an intern cannot
start from this document alone: it assumes the goja-cv vocabulary from a ticket in another repository and uses
about twenty terms it never defines. Second, a few load-bearing claims are wrong against the code (ellipse
coverage, file naming by `WriteFiles`), and they propagate into acceptance tests that cannot pass as written.
Third, several mechanisms the design needs do not exist yet and are not planned (evaluation-time warnings,
receiver-aware validation, image writers with metadata), so phases 1, 3 and 6 would stall on questions.

## Summary

| rank | count | meaning |
|---|---|---|
| P1 | 5 | must fix before handing to an implementer |
| P2 | 13 | fix before or during the phase that needs it; an implementer would have to ask |
| P3 | 17 | polish, citations, small inconsistencies |

P1 findings:

1. **P1-1** The guide does not introduce goja-cv's foundations; it defers them to a CVJS-001 document in another repository and uses about twenty undefined terms.
2. **P1-2** `mask.ellipse` is 4 × 4 supersampled, not exact coverage, so the probe kernel's definition and three acceptance tests (1e-6 equality) contradict the code.
3. **P1-3** `resultcontent.WriteFiles` names files `content-N.ext`; the guide promises `spots.csv` / `name.json` through it with "no upstream change".
4. **P1-4** Measurement rows use `x, y` for the point while the new `point` type refuses `{ x, y }`, and region items use `x, y` for the box corner: the "one schema" ticket ships two meanings of `x, y`.
5. **P1-5** `save` needs writers that do not exist (float32 TIFF with tag 270, PNG chunks, NPY), and the Phase 6 round-trip test asks PNG and NPY to carry a frame origin they cannot store.

## How to read the findings

Each finding has an id, a location (section heading plus a short quote from the guide), the evidence, and a
concrete fix. Code locations are relative to the goja-cv repository root unless they start with `~/go/pkg/mod`
(go-go-goja) or name the DUST-002 ticket.

## P1 findings

### P1-1 Foundations are assumed, not introduced

**Location.** Opening paragraph: "The CVJS-001 vocabulary (node, kind, Regions, Value, evaluation, doc px,
look) and its distinction between masks (soft fields) and regions (discrete, identified things) are assumed."
§3 Concepts covers sampling, profiles, core/ring, schema and data out, but nothing underneath them.

**Evidence.** The CVJS-001 guide lives in the dust-scanning repository; the opening gives no path (only §15
does, and only to a directory). The repository itself has the material the reader needs
(`pkg/doc/02-concepts.md`: kinds, lazy nodes, doc px, masks and regions, looks; `pkg/cv/docs/topics/03-nodes-and-evaluation.md`:
awaited data remembers its node), and the guide does not point to it. First uses of terms never defined in
the guide:

| term | first use | defined in the guide? |
|---|---|---|
| Regions (as a kind), Value node, Image node | §1 "Regions cannot be measured on another image" | no; §3.5 says "Regions are a table" in passing |
| `mem` | §1 "`mem.probe` in `scripts/84-cv-helpers.js`" | no; §4.7 implies it persists in MCP sessions |
| DoG view | §2.1 "the connected blobs of the DoG view above k × its 99th percentile" | no (GCV-002 §3.6 defines it) |
| awaited / awaiting a node | §4.4 "Awaited Regions stop at 100 items" | no |
| analysis scale, `analysisScale` | §7.2 table "(static, like Regions ops)" | no |
| lifting a Value into an argument | §7.2 "lifting `sample(...)[0].l` into `levels`" | no |
| SPACE check, `Level` unit, frame | §7.2, opening | only by reference to GCV-001 |
| intern time | §7.5 "`Node.Fields []FieldSpec` is computed at intern time" | no |
| data lambda | §4.3 "The data-lambda hint" | no |
| the binder, ArgSpec, bind time | §7.6 "The binder (`pkg/cv/graph/args.go`)", §6 item 10 | no |
| node hash | §7.6 "so the hash and `explain()` see the numbers" | no |
| owner goroutine | §7.9 "lazy: evaluated at render time, off the owner goroutine" | no |
| result content, Provider, placeholder `[content N: …]` | §3.6, one clause | partially; the Extract/Render split and limits are never explained |
| CUT note | §7.7 "the CUT note says the same" | no |
| context block | §7.11 "the context block's `mem` listing" | no |
| read roots vs out-root | §3.6, one sentence | partially; that only `mcp` has `--root`, and that no roots means unrestricted reads, is missing |
| planted scan / planted specks | §12 Phase 3 | no (`internal/testimage/planted.go`) |
| gold points | §13.4 | no (GCV-002 §13.4 script) |
| pi harness | §13.5 | no (GCV-001 §13.5) |

The user asked that guides "properly introduce concepts"; the writing guide says "Define terms before using
them repeatedly" and "Assume the reader is new to the codebase". An intern reading §7.5 cannot tell what
"computed at intern time" means or why a Validate function might not see the receiver.

**Fix.** Add a §3.0 "goja-cv in one page" before §3.1, written in the guide's own prose, covering: the five kinds
(Image, Mask, Regions, Value, Param) with one line each; nodes as lazy, immutable, hash-consed graph entries built
by `Builder.Intern` (this is "intern time" = build time, before any pixel is read); ops and the registry
(`OpDef`, `ArgSpec`, the binder that turns JavaScript arguments into canonical args); `await` as evaluation that
returns plain data which keeps a hidden link to its node (`prelude.js:46-79`); derived Value fields and lifting
(`stats().p99` is a Value node usable as an argument); `analysisScale`; `mem`; looks. Point to
`pkg/doc/02-concepts.md` and topics 03 and 05 for detail, and give the CVJS-001 path in full. Expand §3.6 into
two subsections: (a) the result pipeline as a sequence (eval returns → `Extract` walks the value on the owner
goroutine and asks each `Describer` for a placeholder → `Render` calls each `Provider` off the owner goroutine
under `Limits` → MCP maps items to image/text content, the CLI writes files), with a five-line trace of one eval
that returns a look and a CSV; (b) a table of the directories a session touches: read roots (`--root`, `mcp` only;
no roots = unrestricted), the CLI content directory (`--out`), and the new out-root. Define DoG in one paragraph
with a pointer to GCV-002 §3.6, and give paths for the planted scan, the gold points and the pi harness.

### P1-2 The probe kernel's coverage rule does not match `mask.ellipse`

**Location.** §3.1: "weighting each pixel by the fraction of its area inside the disk, the same exact-coverage rule
`mask.ellipse` uses (`pkg/cv/kernels/mask.go:175`)". §9.1: "coverage of the pixel by the core disk (exact for
≤ 1 px from the edge, 0/1 otherwise)". Decision record "measurements are Value ops…": "`probe.go` must match
`StatsWeighted` exactly for binary weights". §13.1: "exact for binary coverage, within 1e-6 for soft edges".
§13.2: "`step`, `stepR/G/B` and `chroma` must equal the helper's within 1e-6".

**Evidence.** `EllipseMask` (`pkg/cv/kernels/mask.go:173-200`) counts 4 × 4 sub-samples per pixel; its weights are
multiples of 1/16, not area fractions. The colleague's `mem.probe` builds its core and ring with `mask.ellipse`
(`84-cv-helpers.js:36-37`). An exact-area probe differs from a 1/16-quantized one on every edge pixel, so the
soft-edge comparison against `StatsWeighted` with ellipse masks and the 1e-6 helper equivalence cannot both hold
with exact coverage. (`BoxMask` is exact; that may be where the claim came from.)

**Fix.** Make disk coverage one function, `kernels.DiskCoverage(px, py, cx, cy, r float64) float64`, used by both
`EllipseMask` (for circles) and `probe.go`, and state its rule in §3.1. Two options, and the guide should choose:
keep 4 × 4 supersampling (helper equivalence at 1e-6 holds; numbers stay what the playbook was calibrated on),
or switch both to exact area (better definition; then loosen §13.2 to a stated tolerance such as 2e-3 absolute
on `step` and document that playbook thresholds move by less than that). Either way, update §3.1, §9.1, the
decision record, §13.1, §13.2 and Phase 3 to the same rule.

### P1-3 `WriteFiles` does not name files after the content

**Location.** §7.9: "the CLI writes them as `name.json` / `name.csv` in its `--out` directory through
`resultcontent.WriteFiles`"; "go-go-goja's `replmcp` limits are configured from goja-cv (`mcp.go:61`); no upstream
change is needed". §9.3: "CLI: WriteFiles(out, "spots.csv"); prints files: { "content 1 csv spots.csv": /abs/out/spots.csv }".
§6 item 7: "The CLI's `writeLooks`: replaced by `resultcontent.WriteFiles`".

**Evidence.** `WriteFiles(ctx, rendering, dir, prefix)` (`resultcontent/files.go:67-125`) writes
`<prefix>-<N><ext>` (or `<prefix>-<N>-<K><ext>`); it has no per-item name and skips undelivered refs. Today's
`writeLooks` (`cmd/goja-cv/eval.go:175-224`) names looks `look-<n>-<slug>` and reports dropped refs with a reason;
both behaviours are lost by the replacement. The colleague's calibration script runs `goja-cv eval --json`
(`83-cv-specks-vs-gold.py:44`), whose JSON output has a `looks` array (`eval.go:80-86`); the guide does not say
where data files appear in that JSON.

**Fix.** Choose one and say it: (a) an upstream change to go-go-goja adding an optional file name to
`resultcontent.Content` (for example `Name string`, used by `WriteFiles` when set, with the extension from
`ExtensionForMIME`), listed as a dependency with its own small PR; or (b) a goja-cv writer that keeps
`writeLooks`' naming and dropped-ref reporting, uses `ExtensionForMIME` for extensions, and adds data names. In
both cases specify the `--json` output (rename `looks` to `files`, each with `caption`, `mimeType`, `path` or
`dropped`) and update §9.3 and the §11 hand-over line for script 83.

### P1-4 Two meanings of `x, y` in a ticket about one schema

**Location.** §7.2 rows `{ x: 1337, y: 131, l: …}`; §7.4 rows `{ x: 1337, y: 131, core: …}`; §7.5 `GeometryFields`
`x` = "left of the bounding box"; §7.6: "An object with `x, y` but no `cx, cy` is refused as a point (`E_ARG_TYPE`,
'a region's x, y is its box corner; points use cx, cy')".

**Evidence.** Measurement rows name the measured point `x, y`. Region items name the box corner `x, y` and the
centroid `cx, cy`. The `point` type refuses `{ x, y }` precisely because of the region meaning, so
`img.sample(await img.measure(pts))`, `mask.flood(row)` or `cv.sheet(rows)` fail with an error that talks about
region box corners although no region is involved. The ticket's premise (§3.5: "must be the same list, or agents
learn one set and are refused by another") is broken by its own new ops.

**Fix.** Give a point one name everywhere. Either measurement rows use `cx, cy` (and the `point` type accepts any
object with numeric `cx, cy`), or the coercion rule becomes structural: an object with `w, h` is a box (its point
is `cx, cy`); an object with `x, y` and no `w, h` is a point. The second keeps rows readable and still prevents
reading a region's corner as its centre. Add the rule to §7.6 as a decision record and a row in the §10 table.

### P1-5 Saving needs writers that do not exist, and two round trips are impossible

**Location.** §7.10: "The TIFF writer records `goja-cv encoding=linear origin=1536,1536` in `ImageDescription`
(270)… PNG records `gAMA` = 1.0 for linear images and an `sRGB` chunk"; table: `depth` (8, 16, 32), formats `.png`,
`.tif`, `.npy`, `.jpg`. §12 Phase 6: "save → open round trips for PNG 8/16, TIFF 16/32, NPY, including encoding
and frame metadata". §4.6: "Images can be written only by the CLI as looks (8-bit JPEG/PNG)".

**Evidence.** goja-cv's writers (`pkg/cv/buffer/io.go:226-290`: `EncodePNG`, `EncodeJPEG`, `EncodeTIFF16`,
`WriteFile`) use the standard library and `golang.org/x/image/tiff`; the latter cannot write custom tags or
float32 samples and writes RGB as RGBA (`io.go:244-245`), and `image/png` cannot write `gAMA`/`sRGB`/`tEXt`
chunks. There is no NPY writer. §4.6 does not mention these existing writers. A PNG has no standard place for a
frame origin (the guide names none), and NumPy's `.npy` header accepts only `descr`, `fortran_order` and `shape`
(`numpy.load` rejects extra keys), so "frame metadata" cannot round-trip through PNG or NPY as specified. JPEG
cannot hold 16-bit or linear data, and a linear image saved as JPEG would reopen as sRGB (GCV-001 §7.3: jpeg
implies srgb).

**Fix.** Add a §7.10 subsection "Writers": a small TIFF writer paired with GCV-001's IFD parser (uncompressed or
Deflate, 1 or 3 samples, uint8/uint16/float32, tag 270), PNG chunk insertion after `IHDR` (`gAMA`, `sRGB`, and a
`tEXt` chunk with keyword `goja-cv` carrying the same description string if the origin should survive), and an NPY
writer (`<f4` or `<u2`). Add a format × depth table that lists what is allowed (`.jpg`: depth 8, srgb images
only, else `E_ARG_TYPE` with the `.png` fix) and what each format records. Change the Phase 6 test to "frame
round-trips for TIFF (and PNG if `tEXt` is chosen); NPY round-trips values and opens as linear with origin (0, 0)".
Correct §4.6 to mention `buffer.WriteFile` and say whether `save` replaces it.

## P2 findings

### P2-1 Two schema mechanisms: the existing `OpDef.Fields` is not mentioned

**Location.** §7.5 "`OpDef` gains, for ops returning Regions: `RegionFields func(args Args, inputs []*Node) []FieldSpec`";
§6 item 2 lists four hand-maintained lists.

**Evidence.** `OpDef.Fields []string` already exists ("documented fields of a Value result (help, hints)",
`pkg/cv/graph/registry.go:113`). The prelude uses it through `__fieldNames()` (`pkg/gojamod/cv/fields.go:9-25`,
`prelude.js:32`, `:238`) to decide whether `node.name` on a Value is a derived field or a method of another kind.
`components` sets `Fields: graph.RegionFields` (`pkg/cv/ops/regions.go:261`), a fifth use of the list. The new
Value ops (`sample`, `profile`, `measure`) return rows whose fields must be declared for lifting and help, and
their level-valued fields need the `Level` unit for GCV-001's SPACE check; the guide gives them no schema.

**Fix.** Replace `Fields []string` with one mechanism for every result kind:
`Fields func(args Args, inputs []*Node) []FieldSpec` (nil = the receiver's). Regions ops return item fields; Value
ops return their row or object fields with units. `__fieldNames` reads the same specs. Say this in §6 (legacy
item) and §7.5, and add `sample`/`measure`/`profile` field lists to §7.2-7.4.

### P2-2 Build-time field validation needs the receiver, which `Validate` does not get

**Location.** §7.5: "`where` and `sortBy` validate against the receiver node's schema at build time".

**Evidence.** `OpDef.Validate func(args Args) error` (`registry.go:114-116`) is called in `Builder.Intern` with
args only (`pkg/cv/graph/node.go:104-108`). Today both validators read the global `KnownRegionFields`
(`ops/regions.go:47-56`, `:295-304`) because they cannot see the receiver.

**Fix.** State the signature change (`Validate(args Args, inputs []*Node) error`, every existing validator
updated) or a schema check inside `Intern` after binding, and put it in Phase 1 step 2.

### P2-3 Evaluation-time warnings do not exist

**Location.** §7.2: "Points outside the frame produce a row with `outside: true`… a `RANGE` warning counts them".
§7.10: "the save then warns `CLAMP` with the clipped fraction".

**Evidence.** Warnings are produced only by the binder at build time (`pkg/cv/graph/args.go:438`); `EvalCtx`
(`pkg/cv/graph/eval.go:25-30`) has no warning channel. Whether a point is outside, or how many values clip, is
known only when the op runs (points can come from a Regions node). Cached values would have to replay their
warnings on the next `await`.

**Fix.** Add an evaluation-warning mechanism to §7 (for example `EvalCtx.Warn(code, msg, fix)`, warnings stored
with the cached value and re-emitted on cache hits, merged into the `warnings:` block by the session) with a
decision record, and a phase step that builds it before `sample` and `save` need it. Add `CLAMP` and any new
codes to `errors.go` (see P3-7).

### P2-4 Content budgets are set in the wrong place, and one sentence describes code that does not exist

**Location.** §7.9 Limits: "The session enforces `MaxLooks` (4) itself in `state.collect`
(`pkg/cvsession/processor.go:116-140`), dropping later looks with a note, and sets the transport's item limit to
`MaxLooks + MaxData` (4 + 4). Data items larger than `MaxDataBytes` (256 KiB) are dropped from MCP delivery…;
the CLI writes them regardless."

**Evidence.** `state.collect` deduplicates and numbers looks; it enforces nothing (`processor.go:116-140`). The
render limit is `Options.ContentLimits()` = `{MaxItems: MaxLooks, MaxTotalBytes: MaxLookBytes}`
(`pkg/cvsession/session.go:115-119`), passed to replapi at `session.go:199`; with it unchanged, any data item after
the fourth content item is dropped at render, before MCP sees it, in both MCP and CLI. `MaxTotalBytes` (8 MiB) is
shared by looks and data. The 256 KiB rule is MCP-only, but the session does not know its transport; the guide
does not say which layer applies it.

**Fix.** Write the sentence about `collect` as a proposal, and list the three settings: `Options.MaxData`,
`Options.MaxDataBytes` (0 = unlimited; set by the `mcp` command only, so the CLI writes everything) and
`ContentLimits()` = `MaxLooks + MaxData` items. Say that the session applies the look count and the data byte
limit in `collect` (replacing the item with a note), and that `mcp.go:61` mirrors the item total.

### P2-5 Lazy CSV placeholders cannot show row counts, and the example counts are wrong

**Location.** §7.9 example and output: `all: cv.csv(ranked, { name: "spots" })` →
`"[content 1: csv spots.csv (213 rows × 21 columns)]"`; §9.3 repeats it.

**Evidence.** `ResultPlaceholder` "is called on the runtime owner goroutine during Extract and must be fast"
(`resultcontent/resultcontent.go:33-38`); a lazy node has not been evaluated, so its row count is unknown. In
the example `ranked` is `….sortBy("snr").top(30)` (§7.4), so the CSV has at most 30 rows, not 213. Its columns are
the 9 geometry fields, `dogMax` and the 9 ring fields: 19, not 21.

**Fix.** Placeholders of lazy content give what is known statically ("csv spots.csv, 19 columns"), or `cv.csv`
of a node evaluates when the eval settles (say which). Correct the numbers in §7.9 and §9.3.

### P2-6 Per-region statistics cannot come from `Rasterize`

**Location.** §7.4: "the region pixels are rasterized at the image's analysis scale with the area-exact
`Rasterize` (`regions.go:194-240`)"; §9.2: "`m := Rasterize(regions at img size); per region: weighted stats over its pixels`".

**Evidence.** `RegionsValue.Rasterize` draws all items into one mask clamped to 1 (`pkg/cv/graph/regions.go:194-240`);
region identity is lost, so per-region weights cannot be read from it.

**Fix.** Specify per-region weights: when the region raster and the image have the same size (the usual case),
use `Region.Pix` directly; otherwise compute each region's area overlap in its own bounding window (a
`Region.Coverage(w, h, scale) (window, weights)` helper extracted from `Rasterize`). Update §9.2 and the Phase 4
test.

### P2-7 Two ways to ring-measure regions, with different defaults

**Location.** §7.2 table: points "`point[]` or Regions… Regions use their centroids"; §7.4 `img.measure(points,
{ core: 3 })` and `regions.measure(image, { ring: true })` with "core radius = clamp(√(area / π), 2, 8)".

**Evidence.** `img.measure(regions)` and `regions.measure(img, { ring: true })` compute the same quantity with
different core radii (3 vs area-derived) and different outputs (rows without `id` vs region fields). An agent will
use both and get different `snr` for the same spot. `img.sample(regions)` rows also lose the region id.

**Fix.** Make the point ops take points only (a Regions argument is `E_WRONG_KIND` with the fix
`regions.measure(img, { ring: true })`), or have rows from a Regions input carry `id` and use the same core rule.
The first removes a special case; record the choice.

### P2-8 `regions.measure` does not reproduce `mem.hot`

**Location.** §7.4 core radius rule; §13.2 maps only `mem.probe`; §13.4 checks medians against playbook numbers.

**Evidence.** `mem.hot` probes at the rounded centroid with `r = clamp(round(√(area / 3.14)), 2, 8)`
(`84-cv-helpers.js:56-58`), which by §13.2's own mapping is a core of `r + 0.5` centred on a pixel centre, ring
`[r + 3.5, r + 9.5]`. `regions.measure` uses an unrounded radius about 0.5 px smaller, at the exact centroid. The
playbook's numbers (§2.1 table) were measured with `mem.hot`.

**Fix.** Either define the default core as `clamp(round(√(area / π)), 2, 8) + 0.5` and say why, or keep the
cleaner rule and add a step to §13.4 that re-measures the playbook table with `regions.measure` and hands the new
ranges to the colleague.

### P2-9 Measurement field names are not one vocabulary, and `chroma` means two things

**Location.** §7.2 rows `l, r, g, b, n` with option `r` (radius); §7.4 fields `core, ring` and options `core, ring`;
§7.4 `regions.measure` fields `mean, max, meanR`; Dependencies: "From GCV-001: … and `chroma`'s kernel".

**Evidence.** In `sample`, `r` is both the radius option and the red-channel field. `core` and `ring` are radii as
options and means as fields. Gray mean is `l` in `sample`, `meanL` in `stats` and `detect.specks`, `mean` in
`regions.measure`, and `core` in `measure`. GCV-001 adds an Image op `chroma` = (max − min) / mean per pixel and
claims (GCV-001 guide lines 817-818) that it "matches the colleague's chroma = (max − min) / mean on regions
(GCV-003 reuses the kernel)"; GCV-003's `chroma` is (max step − min step) / |step| (§3.3, matching
`84-cv-helpers.js:46`) and never uses the GCV-001 kernel.

**Fix.** Add a naming table to §7.5 that covers Value rows too: one name for the gray mean (`meanL` everywhere, or
`l` everywhere), `meanR/G/B` for channel means in `sample` and `regions.measure`, radius options named so they
cannot be read as fields (`radius`, `coreRadius`, `ringRadii`), or field names `coreMean`/`ringMean`. Drop "and
`chroma`'s kernel" from the Dependencies paragraph, consider naming the field `stepChroma` to separate it from
`img.chroma()`, and ask the GCV-001 author to correct lines 817-818.

### P2-10 Module functions: a second mechanism without a decision record, and `save` has no home

**Location.** §7.9: "They are registered as **module functions**: `Registry.RegisterFunc(graph.FuncDef{…})`…
The remaining hand-written `cvFuncs` entries (`evaluate`, `describe`, `docs`, `help`, `isNode`, `warnings`) move
into the same table, which ends the hand-maintained help table"; §7.10 `node.save(path, opts)`; §10 synonyms
`export, write, saveAs → save`.

**Evidence.** GCV-002 §7.10 exposes `cv.<name>` from `KindNone` registry ops. GCV-003 adds a parallel table for
functions that return non-nodes; the reason is good, but there is no decision record. `cvFuncs`
(`pkg/gojamod/cv/help.go:25-43`) also holds `version`, `kinds`, `look`, `show`, `mem`, `img`, `doc`, `P`,
`slider`, `toggle`, `choice`; the guide does not say where they go, so the table does not end. `node.save` (like
`node.look`) is a node method that is neither an op nor a module function, so its help, option checking and
synonyms have no registration path. The guide's Phase 6 steps omit FuncDef (the tasks list has it).

**Fix.** Give `FuncDef` an `On []Kind` field (empty = `cv.` module function; kinds = node method returning a
non-node) so `save` and `look` register the same way, list each `cvFuncs` entry with its destination (FuncDef, op,
or a documented session global that stays hand-written), add a decision record "module functions and non-node
methods are FuncDefs", and add the step to Phase 6.

### P2-11 The §7.4 example uses an undefined `dog`

**Location.** §7.4: "`const hot = dog.levels({ black: 0, white: dog.stats().p99.mul(2) })…`".

**Evidence.** `dog` is not defined anywhere in the guide; the reader does not know it is a bright DoG of the
average-gray image (GCV-002 §7.5, and the `speck` preset of §7.11 without its stretch).

**Fix.** Start the example with
`const dog = img.toGray("average").dog(1, 4, { polarity: "bright" })` and one sentence saying why the unstretched
DoG is measured rather than `img.view("speck")` (the preset includes a stretch).

### P2-12 CLI directories: `--root`, `--out` and `--out-root` are not explained together

**Location.** §7.10: "`--out-root` is a flag of `eval`, `run` and `mcp`, distinct from `--root` (reads)";
§7.9: "the CLI writes them … in its `--out` directory".

**Evidence.** `--root` exists only on `mcp` (`cmd/goja-cv/mcp.go:76`); `eval`/`run` have `--image`, `--out`,
`--json`, `--timeout-ms` (`cmd/goja-cv/eval.go:27-32`) and read anything. The CLI would then have two output
directories with different rules. The prelude file's path policy (relative to what, subject to roots or not) is
not stated.

**Fix.** Add a table: flag, commands, purpose, default. State that `--out` receives content items and
`--out-root` bounds `save`, and decide whether the CLI defaults `--out-root` to `--out` (fewer flags for the
colleague) or keeps saving off. State that `--prelude` paths are operator input, resolved against the working
directory and not subject to `--root`.

### P2-13 Measurements near the frame edge are unspecified

**Location.** §7.2 "Points outside the frame produce a row with `outside: true`"; §7.4 has no edge rule.

**Evidence.** A point inside the frame whose core or ring extends past the edge gets partial weights. The playbook
warns that "specks within 3 px of the border are measured badly" (DUST-002 `reference/04`, §5). The guide does not
say whether weights renormalize over inside pixels, or how the agent learns that a number is unreliable.

**Fix.** Specify: weights cover inside pixels only; rows carry `ringCoverage` (fraction of the annulus inside) or
`edge: true`; `snr` is still computed. Add a test case.

## P3 findings

### P3-1 The throwing `box`/`centroid` accessors: not a shim, but a special case

**Location.** §7.5 "Agents that write `item.box`": "for Regions items it adds non-enumerable accessors `box` and
`centroid` that throw `E_NO_FIELD` with the literal fix… so they are a diagnostic, not an alias."

**Judgement.** It returns no data and cannot be relied on, so it is not a compatibility shim in the sense the
project rules forbid. It is a hard-coded special case in `prelude.js` for two names, though, and `where({ centroid })`
gets a different (generic) message for the same mistake. With no item cap, it adds two `defineProperty` calls per
item (200 000 for a 100 000-item await), and `link()` today touches only the top-level object (`prelude.js:54-79`).

**Fix.** Put removed names in the schema module (`RemovedFields = map[string]string{"box": "[r.x, r.y, r.w, r.h] or pass the item", "centroid": "[r.cx, r.cy]"}`),
and use it in both the item accessors and the `where`/`sortBy` errors. Define the accessors once on a shared
prototype for items instead of per item. Add a decision record (compatibility trade-off) with a removal condition
(for example, after the playbook and the colleague's helpers are migrated).

### P3-2 Citation: the "zoom and look" range is not in the playbook

**Location.** §3.3: "snr = 1.5, which the playbook's table puts in the 'zoom and look' range". **Evidence.** The
playbook §4 table has only the dust/non-dust ranges shown in §2.1; the 1.5-2.5 "zoom" band is in
`84-cv-helpers.js:17`. **Fix.** Cite the helper's comment, or say "inside the overlap of the dust and non-dust ranges".

### P3-3 Example and coordinate details in `sample`

**Location.** §7.2 output `n: 13.1` for `r: 2`; points `[1337, 131]`. **Evidence.** With area weighting,
`n` = π r² ≈ 12.57; under 4 × 4 supersampling it is close to that. Integer coordinates are pixel corners (§3.1),
so `[1337, 131]` centres the disk on a corner. **Fix.** Correct `n`, and add one sentence: to centre on pixel
(i, j), pass `[i + 0.5, j + 0.5]` or a region item.

### P3-4 Two line references are off

`05-masks-and-regions.md` "the example at lines 31-41" is lines 35-40 (line 29 holds the item description that also
changes). "`pkg/cvsession/yaml.go:20-27`" holds the array and string caps; the 9000-character cap is
`DefaultMaxResultChars` (`pkg/cvsession/session.go:38-39`). All other references I checked are correct (see the
table below).

### P3-5 The producer index misses parameterized fields

**Location.** §7.5: "the 'which op produces this field' index… evaluating each op's `RegionFields` with default
arguments". **Evidence.** `regions.measure` with default arguments adds no fields, so `where({ snr })` on plain
components cannot hint "snr comes from regions.measure({ ring: true })". **Fix.** Let an op declare the fields it
can produce (a static `MayProduce []FieldSpec`) for hints.

### P3-6 Phase ordering: `meanMask` breaks the Phase 1 property test

**Location.** Phase 1 test "every Regions-producing op's items have exactly its schema's names"; `meanMask` removal
is Phase 4. **Evidence.** `components` still adds `meanMask` until Phase 4 (`ops/regions.go:254`), while §7.5
gives it the geometry schema. **Fix.** Remove `meanMask` in Phase 1 (tests and docs that read it switch to
`stats({ mask })` until Phase 4), or declare it in the components schema until Phase 4.

### P3-7 Error and warning codes: where they live and which ones

**Location.** §10 table. **Evidence.** `E_TOO_MANY` and `CLAMP` are new and not in `pkg/cv/graph/errors.go`;
`E_ACCESS` is defined in `pkg/cvsession/session.go:45`, but `save` runs in the cv module. Regions and image on
different frames should use GCV-001's `E_FRAME` (added in `bfbc3a0`), which is missing from the table. A duplicate
field name reported as `E_ARG_TYPE` reads oddly. **Fix.** List new codes and their file, move `EAccess` to `graph`
if the cv module raises it, add the `E_FRAME` row, and consider `E_ARG_RANGE` or a dedicated code for the
duplicate field.

### P3-8 Fragment bullets

§6 items 2-11, §11, §12 step lists and §13.1 are fragments ("Box/point coercion table tests.", "Save: containment,
formats, metadata round trip."). The writing guide asks for complete sentences in bullets. Rewrite them as
sentences; numbered implementation steps can stay terse but should each be a complete instruction. No AI-slop
phrasing or analogies were found anywhere in the guide.

### P3-9 Hand-over command for script 83

**Location.** §11: "`goja-cv eval --out DIR -e 'return cv.csv(img.detect.specks({ maxArea: 2000 }))'`".
**Evidence.** Script 83 passes `--image` and `--json` and uses `polarity: "bright"` (`83-cv-specks-vs-gold.py:4`,
`:44`). **Fix.** Give the full command and the JSON key Python reads the CSV path from (see P1-3).

### P3-10 Test infrastructure paths

"The planted scan" is `internal/testimage/planted.go`; the gold points file is produced by GCV-002's
`scripts/01-export-gold-points.py` (GCV-002 §13.4); the pi harness and its `IMAGE`/`QUESTIONS` overrides are in
GCV-001 §13.5. Name them where they are first used, and say where `questions-gcv-003.md` goes.

### P3-11 Timing acceptance is unconditioned

**Location.** §13.4: "it must take under 200 ms". **Fix.** State conditions (scale 1, warm cache, which machine or
`go test -bench` on CI) or make it relative ("at least 20 × faster than the 56-stats helper on the same machine").

### P3-12 Union argument types

`mask.gradient`'s `from`/`to` are type `any` (an edge name or a point, `ops/mask.go:233-262`); GCV-002's `sheet` takes
a list of points or boxes. The guide says both "switch" to the new types without saying how a `point` composes
with an enum or with `box` in one list. Add the binder rule (for example `point|enum:top,bottom,left,right` and
`(point|box)[]`).

### P3-13 `cv.json` of modified awaited data

**Location.** §7.9: "awaited data (its link to the node is used, so it renders lazily)". **Evidence.** `marshal`
sends any object carrying the hidden node link as the node (`prelude.js:103`); if the agent mutates the awaited
object before `cv.json(d)`, the edits are silently ignored. **Fix.** Document it in the `data-out` topic, or
serialize awaited data as data.

### P3-14 Missing decision records

Records are complete and in format for the six choices they cover. Choices that deserve one and have none: the
`box`/`point` coercion rule (P1-4), the throwing accessors (P3-1), module functions (P2-10), evaluation-time
warnings (P2-3), and look/data budgets (P2-4). The issue 10 alternative `components({ source: image })` is not
mentioned in "Alternatives not taken".

### P3-15 `dataout.Content` shadows `resultcontent.Content`

The proposed type has the same name as the go-go-goja type it produces, which makes signatures such as
`ResultContent(ctx) ([]resultcontent.Content, error)` on a `Content` hard to read. Name it `dataout.Item` or
`dataout.Data`.

### P3-16 `sortBy` direction in the example

`sortBy("snr").top(30)` relies on `desc` defaulting to `true` (`ops/regions.go:293`). One clause in §7.4 ("sortBy
sorts largest first") saves the reader a lookup.

### P3-17 Absolute coordinates for the colleague

`mem.probe` returns `ax, ay` from `mem.origin` (`84-cv-helpers.js:43-44`). With GCV-001 frames, rows are in root doc
px, so a window opened with `origin` gives absolute coordinates directly. Say so in the §11 hand-over line; it
removes `mem.origin` from the colleague's workflow.

## Issue coverage

| issue | asked for | guide's answer | verdict |
|---|---|---|---|
| 9 | `sample`, `profile` (plot), `measure` with core, ring, step, spread, snr, per-channel step | §7.2-7.4, §7.8 | covered; P1-2, P2-9, P2-13 affect it |
| 10 | `regions.measure(image, { stats, channels })` or `components({ source })`; neutrality | §7.4; chroma only with `ring` (justified) | covered; P2-6, P2-7, P2-8; the `source` alternative is not discussed (P3-14) |
| 16 | all items; large results to a file | no cap + safety limit (decision record), `cv.json`/`cv.csv`, `.save` | covered; deviation from `items({ all })` justified; P1-3, P2-4, P2-5 |
| 17 | aliases both ways | flat scalars only (decision record), throwing accessors | deviation justified; P1-4, P3-1 |
| 21 | `node.save(path, { depth: 16 })` under `--out-root`, returns the path | §7.10 | covered; P1-5, P2-12 |
| 23b | `eval`/`run --prelude` | §7.11, also on `mcp` | covered; P2-12 for path policy |

## Claims checked against the code

| claim in the guide | result |
|---|---|
| `regions.go:97` centroid at pixel centre; `:105`, `:108-109`, `:111-138`, `:141-151`, `:158-175`, `:177-192`, `:194-240` | correct |
| `ops/regions.go:223-263` (`meanMask` at 254), `:47-56`, `:295-304` | correct |
| `ops/detect.go:10` `speckFields`; specks fields contrast, localNoise, sharpness, score, meanL, polarity | correct |
| `ops/measure.go:126-140`, `:53-98`, `:134` | correct |
| `kernels/measure_weighted.go:27-29` wrapper, `:27-114` | correct |
| `kernels/mask.go:175` "exact-coverage rule" | wrong: 4 × 4 supersampling (P1-2) |
| `ops/mask.go:149` box arg, `:266` flood point | correct |
| `binding.go:74` hint, `:339` `SummaryLimit` | correct |
| `cvsession/yaml.go:20-27`, `:279-283`, `:109-110` | correct except the 9000 cap location (P3-4) |
| `cmd/goja-cv/eval.go:88-137`, `:175-224` | correct |
| `resultcontent/files.go:48-137`, `WriteFiles` "do the same job with proper extensions" | extensions yes, names no (P1-3) |
| `replmcp/tools.go` `toMCPContent` text mapping; `attached >= maxImages` | correct (`tools.go:222`, `:236-244`) |
| `cvsession/mcp.go:61` `MaxImages: MaxLooks` | correct |
| `cvsession/session.go:56` roots; `native.go:62-88`, `:72-84` | correct |
| `processor.go:116-140` "the session enforces MaxLooks" | describes proposed behaviour as current (P2-4) |
| `prelude.js:54-79` `link()` | correct |
| `look.go:87-92`, `chart.go:11-40` | correct |
| `help.go:24-43` `cvFuncs` | correct; inventory incomplete (P2-10) |
| `suggest.go:29` `measure → stats` | correct |
| `84-cv-helpers.js` lines 35-47, 52-61, quoted code | correct, verbatim |
| §13.2 helper mapping (core r + 0.5 at pixel centre, ring r + 3.5 / r + 9.5; spread uses Rec. 709 luminance) | correct (`EllipseMask` centre `x + bw/2`; `buffer.Luminance` is Rec. 709; `minus` = min(a, 1 − b) equals a − b for these radii) |
| `resultcontent.Describer.ResultPlaceholder`, `Provider.ResultContent` signatures | correct |
| registry key `On:Name` lets `Image:measure` and `Regions:measure` coexist | correct (`registry.go:133`) |
| Value index lifting `sample(...)[0].l` | works today (`ops/value.go:22-41`) |

## Architecture verdict

The core is strong where the guide commits to one mechanism: one probe kernel for four ops, one schema per
node, presentation caps kept in the presenter, data as result content, and an explicit write root. Legacy removal
is taken seriously (eleven items, no aliases). The places where it falls short of "streamlined and composable" are
the ones where a second mechanism appears next to an existing one: `RegionFields` next to `OpDef.Fields` (P2-1),
`FuncDef` next to `KindNone` ops with `save`/`look` left outside both (P2-10), two entry points for ring measures on
regions (P2-7), and two meanings of `x, y` (P1-4). Each has a general fix that removes a special case rather than
adding one. The throwing accessors are a diagnostic rather than a shim; they should be generalized and
time-limited (P3-1).
