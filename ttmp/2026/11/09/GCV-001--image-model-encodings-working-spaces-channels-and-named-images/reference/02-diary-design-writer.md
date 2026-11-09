---
Title: Diary - design writer
Ticket: GCV-001
Status: active
Topics:
    - image-model
    - io
    - color
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/design-doc/01-design-and-implementation-guide.md
      Note: GCV-001 guide (Step 1)
    - Path: ttmp/2026/11/09/GCV-002--inspection-views-and-looks-crops-zoom-rulers-sheets-arithmetic-high-pass/design-doc/01-design-and-implementation-guide.md
      Note: GCV-002 guide (Step 2)
    - Path: ttmp/2026/11/09/GCV-003--measurement-and-data-out-sampling-profiles-region-measures-schema-export/design-doc/01-design-and-implementation-guide.md
      Note: GCV-003 guide (Step 3)
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md
      Note: shared foundations chapter (Step 4)
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
Summary: "Diary of writing the three design-and-implementation guides GCV-001..003 (one step per guide and per revision round)."
LastUpdated: 2026-11-09T12:30:00-05:00
WhatFor: "Reviewing how the GCV guides were written and which decisions were made where."
WhenToUse: "Before revising a GCV guide or implementing from one."
---

# Diary - design writer

## Goal

This diary records the writing of the three intern guides for goja-cv's next phase: GCV-001 (image model),
GCV-002 (inspection views and looks) and GCV-003 (measurement and data out). One step per guide and one per
revision round, with the commands run, the evidence gathered, and the decisions that a reviewer should check.

## Step 1: GCV-001 guide — encodings, working spaces, frames, channels, named images

I read the writing guidelines, the handover issues and the colleague's playbook and scripts, the CVJS-001
intern guide, the pi-run evidence and the deep-dive article, and then the goja-cv code bottom-up (buffer,
kernels, graph, ops, look, gojamod, cvsession, docs, cmd). The guide replaces the hard-wired sRGB model with
an encoding per image, one build-time `space` option per op with explicit `convert` nodes, decoders for
linear/float/npy/DNG data, frames (replacing DocW/DocH), channel ops, a macro mechanism, and named images.

The central finding is that `OpDef.Space` is documentation only: the evaluator never reads it, and nine
kernels/helpers convert to perceptual privately. That makes the colleague's "choose the space" request a
refactor of where conversion happens, not a new option bolted onto kernels.

### Prompt Context

**User prompt (verbatim):** "ok, write out tickets 001 002 003, we'll then pass it on. Make sure to read the updated guideines on writing design docs and properly introducing concepts by doing a review read. Create the three ticket and write the design / implementation guide for an intern, in a textbook authoring manner. We are interested in keeping a strong core streamline but composable architecture, and opportunities to refactor / get rid of legacy or deprecated code should be taken. This is early stage, and no backwards wrappers needed."

**Assistant interpretation:** Write the three design-and-implementation guides (tickets already created) in
textbook style for an intern, with evidence-anchored current-state analysis, legacy-removal sections, decision
records, phases, tests, and docmgr tasks; commit each guide.

**Inferred user intent:** Hand implementable, teachable plans to interns that also clean up the core instead
of growing special cases.

**Commit (docs):** 7b3bb16 — "GCV-001: design and implementation guide (encodings, working spaces, decoders, frames, channels, named images) and tasks"

### What I did

- Read `~/.codex/skills/textbook-authoring/SKILL.md`, `ticket-research-docmgr-remarkable/references/writing-style.md`,
  `ttmp/_guidelines/design-doc.md`, `~/.codex/skills/diary/SKILL.md`.
- Read DUST-002 `reference/04`–`07`, `scripts/82-export-cv-tiles.py`, `scripts/84-cv-helpers.js`; CVJS-001
  design docs 02–06 (headings, values/nodes/ops/masks sections, buffers/registry/scale/looks, formats and
  result content); `various/pi-runs/README.md`; `scripts/06-pi-cv-agent/run.sh` and `questions.md`; the
  deep-dive article (error classes, rules, open questions).
- Read with `nl -ba`: `pkg/cv/buffer/{buffer,io,color,pyramid}.go`, `pkg/cv/graph/{registry,value,eval,engine,
  regions,sources,explain,node,errors}.go`, parts of `args.go`, `suggest.go`, `help.go`; all of `pkg/cv/ops/*.go`;
  `pkg/cv/kernels/{tone,composite,measure,draw,specks,mask}.go` (relevant parts); `pkg/cv/look/{look,font,chart}.go`;
  `pkg/gojamod/cv/{prelude.js,binding.go,module.go,fields.go,evaltrack.go,help.go}`;
  `pkg/cvsession/{session,init.js,native,processor (context block),yaml,reference,mcp}.go`; docs topics 01, 02,
  05, 06, 09 and `docs_test.go`; `cmd/goja-cv/eval.go`.
- Checked go-go-goja `resultcontent` (Content/Provider/Limits, WriteFiles, ExtensionForMIME) and `replmcp/tools.go`
  (`application/json` and `text/*` become MCP TextContent; all items count against `maxImages`).
- Checked x/image/tiff: rejects `SampleFormat` ≠ 1 (`reader.go:252-262`), has no LinearRaw case.
- Inspected a real tile's TIFF tags with a Python one-liner (uncompressed, 16-bit, 3 samples, one strip) and
  `dustds/rawio.py` plus `~/dustds/meta/dng_meta.jsonl` (VueScan LinearRaw in SubIFD, no black/white levels,
  7256 × 7326, data offset 8); confirmed `raw0027.dng` exists locally (319 MB).
- Built goja-cv into the scratchpad and measured the tile `1d955ef4b27d3d88_1536_1536.tif` (meanL 0.33567,
  p01 0.0085, p99 0.918; metadata reports `colorSpace: sRGB (assumed…)`).
- Computed the sRGB numbers used in the guide (0.5 → 0.214, 0.05 → 0.0039, density step 0.0744 vs 0.0289,
  code 38261 for v = 0.3) and the `levels` behaviour change table with `python3 -I`.
- Wrote the guide, added 19 tasks with `docmgr task add --ticket GCV-001`, ran
  `docmgr doctor --ticket GCV-001 --stale-after 30` (all checks passed), committed.

### Why

- The guide had to explain encodings and spaces before any API, because the API decisions (defaults, derived
  encoding, looks showing encoded values) only make sense once the identities "srgb: encoded = perceptual" and
  "linear: encoded = linear" are understood.
- Frames were pulled into GCV-001 (not GCV-002) because `open({ box })` on a DNG needs them and they are part of
  "where an image sits", i.e. the image model; GCV-002's crop becomes the third producer.
- The macro mechanism (`OpDef.Expand`) was put in GCV-001 because `split()` needs multi-node results, and
  GCV-002's views/high-pass need the same mechanism.

### What worked

- The registry already carries `Static`, units and option synonyms, so the `space` option and the `Level` unit
  slot into existing machinery rather than adding a parallel one.
- The identities made the default table nearly behaviour-preserving for sRGB images: every former "perceptual"
  op defaults to `encoded`, which is numerically identical there.

### What didn't work

- My first `grep` for error-code usage failed in zsh: `grep -rn "graph.$c\b\|[^a-zA-Z]$c[^a-zA-Z]" ...` gave
  `(eval):1: bad math expression: operand expected at '^a-zA-Z'`. Re-ran with `grep -rnw "$c"`.
- `grep --include=*.go` unquoted failed in zsh (`no matches found: --include=*.go`); quoting the glob fixed it.
- My first claim about the `levels` change ("less than two codes in the midtones") was too optimistic; a sweep
  showed up to 8.5 codes for white = 0.3 in the deep shadows, so the guide now has the measured table.

### What I learned

- `look.Render` → `buffer.ToImage` encodes masks through the sRGB curve, so a 0.5 mask shows as 188/255 today.
- `DetectSpecks` detects on perceptual luminance but reports `MeanL` from linear luminance: one op, two spaces.
- MCP delivers `application/json` content as TextContent but counts it against the image limit; relevant for
  GCV-003.

### What was tricky to build

- Choosing what looks display. Showing perceptual (today's rule) breaks signed maps on linear-encoded data (an
  offset of 0.5 would show as 0.735); showing encoded keeps every sRGB look identical and makes data display
  literal. The cost is that linear photographs need `encode("srgb")`, which the guide makes an explicit op.
- Deciding against a session/image default space. It would have been convenient for the colleague, but opening
  with `encoding: "linear"` already gives them v in every space, and a hidden default makes scripts ambiguous.
- The `levels` default: encoded would be the "display" choice, but it would silently break the
  `levels({ black: s.p01, white: s.p99 })` idiom against linear `stats()`. Kept linear and added a `SPACE`
  warning for lifted levels from another space.

### What warrants a second pair of eyes

- The default-space table (§7.5) and the claim that it changes nothing for sRGB images except `levels` and mask
  display.
- The derived-encoding rule (§3.5, decision record): whether every op the guides mark `DerivedEnc` is really a
  new quantity.
- The TIFF parser decision (own IFD parser + x/image fallback) versus a separate DNG decoder.

### What should be done in the future

- Implementation per the phases; the pi harness overrides (`IMAGE`, `QUESTIONS`) in CVJS-001's script directory.

### Code review instructions

- Start with §3 (concepts) and §7.5 (space resolution), then the decision records in §8.
- Validate evidence by opening the cited `file:line` ranges at commit 175b5d4.

### Technical details

- Commands: `docmgr doc add --ticket GCV-001 --doc-type reference --title "Diary - design writer"`,
  `docmgr task add --ticket GCV-001 --text "…"` (19 tasks), `docmgr doctor --ticket GCV-001 --stale-after 30`.
- Measurement: `goja-cv eval --image ~/dustdet/cv_tiles/1d955ef4b27d3d88_1536_1536.tif -e '…stats, histogram,
  components…'`.

## Step 2: GCV-002 guide — render scale, decorations, crops, sheets, signed arithmetic, views

The GCV-002 guide turns the colleague's look requests into one rule: anything that depends on pixel content or
data is a node (crop, arithmetic, highpass/dog/localContrast, levels auto, annotate, overlay, sheet, view
presets); anything that depends only on the output raster is a look option (zoom, interp, maxSide, ruler, grid,
format, label). Two engine changes carry it: evaluation at a render scale above 1, where pixel ops still run at
scale ≤ 1 and are upsampled, and a `Decoration` flag for ops that draw at the render scale (thin lines, readable
text under zoom).

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Second guide of the three, consistent with GCV-001's model.

**Inferred user intent:** Make inspection native to goja-cv without growing look-time special cases.

**Commit (docs):** e1855d5 — "GCV-002: design and implementation guide (render scale and decorations, crop, zoom/ruler looks, sheets, signed arithmetic, high-pass views, presets) and tasks"

### What I did

- Re-read look.go (options, scale fit, Encode), look/font.go and kernels/draw.go (two digit fonts),
  ops/present.go (overlay at evaluation scale, unsigned compare diff), kernels/composite.go (blend modes),
  evaltrack.go (hand-parsed look options), prelude.js (`cv.compare`), help.go (`cvFuncs`, crop redirect),
  eval.go (scale check), the colleague's `85-cv-views.py` (rank metric) and `80-vlm-render.py` (nearest zoom,
  ticks).
- Wrote the guide; added 15 tasks with `docmgr task add --ticket GCV-002`; doctor passed; committed.
- Fixed two statements after a review read: ruler tick placement now follows goja-cv's continuous coordinate
  convention (pixel i covers [i, i + 1), centroid 1312.5 is the centre of pixel 1312; `regions.go:97`), and the
  annotate note no longer used flat item fields that only exist after GCV-003.

### Why

- Under "everything as look options", a crop could not be measured or reused in a sheet, and a per-crop stretch
  would duplicate `levels`. Under "everything as nodes", zoom and rulers would produce pixels that are not doc px.
- No `offset` option on arithmetic: it is exactly `.add(0.5)`; one mechanism, per the project's no-alias rule.
- `img.view(name)` instead of `cv.view(name)`: GCV-001's named images make an implicit `img` ambiguous.

### What worked

- Macros (from GCV-001) make `img.highpass(3)` the same node as the hand-written chain; tests can assert node
  identity instead of pixel equality.
- `OpDef.Halo`, declared since M1 and never read, gets a first reader (`CROP_FIRST` warning).

### What didn't work

- N/A (no tool failures in this step).

### What I learned

- Region rasterization is already area-exact at any target size, so overlay as a decoration at scale 8 needs no
  new rasterizer.
- The colleague's ranking numbers (median rank 18 vs ~715, 78 % vs 1 % in the top 50) are a ready-made
  regression metric for the `speck` preset.

### What was tricky to build

- Zoom versus filters: evaluating everything at the render scale would change filter values with the zoom;
  evaluating everything at ≤ 1 would make overlay outlines 8 px wide at 8×. The decoration flag separates the
  two with one branch in `evalRun.eval`.
- Sheets: their pixels are not doc px, so the guide gives them a sheet frame that coordinate ops and rulers
  refuse.

### What warrants a second pair of eyes

- The rule "content = node, raster = look option", especially rulers as look options (they draw pixels, but
  only from frame and scale).
- Removing blend modes `add`, `difference`, `lighten`, `darken` in favour of arithmetic ops.
- The `format: auto` rule and the look byte budget for PNG sheets.

### What should be done in the future

- Windowed evaluation (Halo-driven) for crops of full-resolution DNGs, listed as a risk.

### Code review instructions

- Read §7.1 (the rule), §7.2 (evaluator change), §7.7 (looks), §8 decision records.

### Technical details

- `docmgr task add --ticket GCV-002 --text "…"` (15 tasks); `docmgr doctor --ticket GCV-002 --stale-after 30`.

## Step 3: GCV-003 guide — measurements, one region schema, data out, saving, preludes

The GCV-003 guide adds `sample`, `profile`, `measure` and `regions.measure` on one shared probe kernel, replaces
the four hand-maintained region field lists with a per-node schema of flat scalar fields, removes the 100-item
cap, and adds data out (`cv.json`, `cv.csv` as result content), saving under a separate `--out-root`, and CLI
preludes. It also moves module functions into the registry (`graph.FuncDef`), ending the hand-written `cvFuncs`
help table.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Third guide, completing the set, with cross-references to GCV-001 and GCV-002.

**Inferred user intent:** Give agents numbers and data channels that match what they already trust, without
aliases or caps disguised as API.

**Commit (docs):** b30efec — "GCV-003: design and implementation guide (sample, profile, ring measures, regions.measure, one region schema, no item cap, JSON/CSV content, save under out-root, preludes) and tasks; GCV-001: decoders read saved metadata"

### What I did

- Re-read graph/regions.go (Field vs Data, RegionFields, KnownRegionFields, SummaryLimit), ops/regions.go
  (where/sortBy validation, meanMask), detect.go (speckFields), measure.go and measure_weighted.go, args.go box
  and point conventions, yaml.go caps, processor.go `collect`, cmd/goja-cv/eval.go `writeLooks`,
  go-go-goja `resultcontent/files.go` and `replmcp/tools.go` (text content mapping and the shared item limit).
- Re-read `84-cv-helpers.js` with line numbers to specify `measure` against `mem.probe`.
- Wrote the guide; added 13 GCV-003 tasks and one GCV-001 task (decoders read goja-cv's saved metadata);
  doctor passed for both; committed. After commit, added `E_FRAME` to GCV-001's error table for consistency
  with GCV-002.

### Why

- Flat scalar fields win over `box`/`centroid` because filters need scalars; region items become acceptable
  wherever a box or point is expected, which removes the reason arrays were convenient.
- The 100-item cap truncated data inside JavaScript; caps belong to the YAML presentation layer.
- `cv.json`/`cv.csv` return Go objects, so they cannot be ops; a module-function table in the registry gives
  them help and option checking without the hand-written table.

### What worked

- go-go-goja already maps `application/json` and `text/*` to MCP text and has `WriteFiles` with proper
  extensions, so data out needs no transport changes, only goja-cv-side budgets.

### What didn't work

- My first draft said `measure` would equal `mem.probe` within 1e-6 directly. Reading the helper with line
  numbers showed two mismatches: its disks are centred on pixel centres with radius r + 0.5 (boxes
  `[x − r, y − r, 2r + 1, 2r + 1]`), and its `step` averages channels while its `spread` uses Rec. 709 luminance
  percentiles. The guide now gives the exact mapping and a `gray` option, and the test compares under it.
- My first draft registered `cv.json`/`cv.csv` as `KindNone` ops, which cannot work because ops return nodes;
  replaced with `graph.FuncDef`.

### What I learned

- replmcp counts every content item (images and text) against `MaxImages`, which goja-cv sets to 4; data
  content needs its own budget in the session.

### What was tricky to build

- Keeping old agent habits (`item.box`) from failing silently without adding aliases: the guide uses
  non-enumerable throwing accessors that return no data and carry a literal fix.
- Points versus boxes from region objects: a region's `x, y` is its box corner, so point coercion accepts only
  `cx, cy` objects and refuses `{ x, y }` with an explanation.

### What warrants a second pair of eyes

- The schema decision (no `box`/`centroid` arrays at all) and the throwing accessors.
- Data budgets: 4 looks + 4 data items, 256 KiB MCP data limit.
- Write access via `--out-root` and its containment checks.

### What should be done in the future

- Revision round after the reviewer's findings.

### Code review instructions

- Read §3.3 (core/ring with numbers), §7.5 (schema), §7.9–7.10 (data out, saving), §13.2 (probe equivalence).

### Technical details

- `docmgr task add --ticket GCV-003 --text "…"` (13 tasks), `docmgr task add --ticket GCV-001 --text "Phase 2:
  TIFF and PNG decoders read goja-cv ImageDescription …"`, `docmgr doctor --ticket GCV-00N --stale-after 30`.

## Step 4: Revision round — shared foundations and GCV-001 revision 2

The review read (commit 07e569e) found 4 P1, 22 P2 and 21 P3 issues in GCV-001, and the three reviewers converged on
missing foundations. I wrote one shared foundations chapter (`reference/05-foundations-goja-cv-for-the-gcv-guides.md`,
§F1–§F12) that all three guides cite, with one pixel and scale vocabulary and the render rule worked through on a
64 px × 8 crop, and then rewrote GCV-001 as revision 2 with a revision history and a disposition table appended to the
findings.

The two decisions that changed the design most: luminance-based ops now get their grey plane from linear light through
a graph-level rule (`OpDef.Gray`), because "encoded = perceptual" does not hold for luminance (pure red 0.498 vs 0.213);
and `perceptual` is dropped, leaving two spaces.

### Prompt Context

**User prompt (verbatim):** Revise the guides from the review-read findings (all P1/P2, P3 fix or decline).

**Assistant interpretation:** Fix every P1 and P2 of the three findings documents, fix or decline each P3 with a
reason, apply the cross-cutting fixes once and consistently, re-verify any code claim touched, add revision histories
and disposition tables, update tasks, run doctor, commit per guide.

**Inferred user intent:** Make the guides implementable by an intern without outside context and without
contradictions between the three tickets.

**Commit (docs):** 48e3a50 — "GCV-001: revision 2 from the review read (…)"

### What I did

- Read the three findings documents in full and re-verified the code they cite: `node.go:80-246` (Intern,
  internNode, Substitute, hashNode), `kernels/mask.go:148-200` (BoxMask exact, EllipseMask 4 × 4), `explain.go:53-85`,
  `regions.go:37` and `:67` (both `NewRegions` and `NewRegion` exist), `help.go:279-312` (UnknownMember),
  `processor.go:420-470`, the tile's first 32 bytes with `xxd`, and the gold annotation files under `~/dustds/gold`.
- Recomputed numbers with `python3 -I`: decode-direction density steps (0.0096 vs 0.0615), luminance vs luma (0.498 vs
  0.213; 0.512 vs 0.471), density transmission (50 %, 25 %).
- Created the foundations doc with `docmgr doc add --ticket GCV-001 --doc-type reference --title "Foundations - goja-cv
  for the GCV guides"`.
- Rewrote GCV-001: two spaces; §3.1 "linear in Q"; §3.4 two-question rule; §3.5 luminance; masks with no encoding;
  absolute coordinates with integer origins; `Convert(from, to)` with exact functions and no `Buffer.Space`; values
  stamped by the evaluator; `DocFrame`, `Result` as a function, `Gray`, `Internal`; statics in `internNode`;
  placeholders copy statics; `box`/`point` types; TIFF primer with own decompression; `Info.Origin` and PNG `tEXt`;
  macro nodes and `CallResult`; `as` only in the session wrapper; unnamed opens listed; phases reordered with a golden
  harness in Phase 0; five new decision records; key points and a closing.
- Appended the disposition table to `03-review-read-findings.md`; removed and re-added the 19 tasks for the new phases.

### Why

- A shared chapter removes three partial glossaries and the dependency on another repository's guide.
- Computing statics inside `internNode` is the only way every creation path (Intern, Substitute, macros) agrees
  without mutating shared nodes.

### What worked

- The rule "luminance from linear light, expressed in the op's space" keeps sRGB behaviour exact while keeping
  kernels space-agnostic, so the golden diff after Phase 3 can be restricted to `levels` and mask looks.

### What didn't work

- `Write` refused to overwrite the GCV-002 guide because it had not been read in this context
  (`File has not been read yet. Read it first before writing to it.`); I read it and re-sent the content.

### What I learned

- One review claim was wrong against the code: `RegionsValue.NewRegion` exists (`regions.go:67`); the reviewer had
  found only `NewRegions`. The disposition table says so.

### What was tricky to build

- Phase order: frames must precede spaces (spaces need frames on placeholders) and decoders (windows need origins),
  while the display change must land in the same phase as the kernel port, so the goldens move exactly once.

### What warrants a second pair of eyes

- The `Gray` rule's interaction with `histogram` (only channel `"L"` uses it) and with gray inputs.
- Whether dropping `perceptual` loses a use case the colleague has.

### What should be done in the future

- N/A

### Code review instructions

- Read Foundations §F5 and GCV-001 §3.5, §7.5 and §12; then the disposition table.

### Technical details

- `docmgr task remove --ticket GCV-001 --id …` for each old task, then `docmgr task add` for 19 new ones;
  `docmgr doctor --ticket GCV-001 --stale-after 30` passed.

## Step 5: GCV-002 revision 2

GCV-002 had 6 P1, 16 P2 and 15 P3 findings. The central fix is a third op class: crop is **geometric** (exact at any
scale), so a decoration before a crop draws at the render scale, and render windows keep every buffer at look size.
Sheets became layout ops at scale 1 with a static `max`, labels inside cells, and defaults that fit 36 cells
(6 · 256 + 5 · 6 = 1566 px); captions name macros through GCV-001's macro nodes instead of a mutable `Node.Macro`.

### Prompt Context

**User prompt (verbatim):** (see Step 4)

**Assistant interpretation:** Same as Step 4, for GCV-002.

**Inferred user intent:** Same as Step 4.

**Commit (docs):** a31654a — "GCV-002: revision 2 from the review read (…)"

### What I did

- Rewrote the guide around Foundations §F5; added §3.1 "Nodes and look options"; σ semantics and the DoG worked example
  (speck/grain ratio 5.6 vs 9); `Engine.Render` with `RenderOptions{Scale, Interp, Window}` and a dispatch by class and
  kind; `DECORATION_FIRST`; crop as geometric with an integer frame; `Image|number` operands; `levels.auto` with
  Optional black/white and no Identity short-circuit; `Rasterize(target Frame, scale)`; picture frames for sheets and
  compare layouts; `compare` diff removed; `FuncDef` and the "ops build nodes, functions do things now" record;
  `EvalCtx.Warn`; `ArgSpec.Redirects`; acceptance table per issue; gold points read directly from `~/dustds/gold`.
- Appended the disposition table; replaced the 15 tasks with 14 for the new phases; doctor passed.

### Why

- Without a geometric class, `img.annotate(marks).crop(box)` at zoom 8 would compute the annotation at scale 1 and
  upsample 8 px outlines, the exact failure the ticket set out to fix.

### What worked

- The gold annotation files (`{tile_id}.final.json` or `.p1.json`) already have a simple schema, so no export script
  is needed for the ranking test.

### What didn't work

- N/A

### What I learned

- In sessions `cv` is guarded by a Proxy whose unknown-member path (`UnknownMember`) already suggests `img.view(…)`
  for Image ops; it does not consult the op synonym table, which is the actual gap.

### What was tricky to build

- Render windows: a geometric op at scale 8 would otherwise request an 8× upsample of the whole input (8192² for a 1024
  tile). Carrying the window lets pixel ops be computed whole at scale 1 (cached) and upsampled only inside the window.

### What warrants a second pair of eyes

- The render dispatch pseudocode (§7.2), especially macro nodes and memo keys.
- The decision to drop `compare({ mode: "diff" })` entirely.

### What should be done in the future

- Windowed evaluation of pixel ops for full-size DNGs (risk in §14).

### Code review instructions

- Read §3.4, §7.2, §7.9 and §13.6.

### Technical details

- Sheet arithmetic: one view → ⌊(1568 + 6)/(256 + 6)⌋ = 6 per row, 6 rows; two views → groups of 514 px, 3 per row,
  6 rows = 18 items.

## Step 6: GCV-003 revision 2

GCV-003 had 5 P1, 13 P2 and 17 P3 findings. The fixes: one naming rule shared with GCV-001 (`x, y` corner, `cx, cy`
point, `meanL`, radius options named so they cannot be read as fields, `stepChroma`); coverage is `mask.ellipse`'s 4 × 4
supersampling, extracted into `DiskCoverage`, so measurements stay identical to the colleague's helper; one schema
mechanism (`OpDef.Fields` as a function returning `FieldSpec`s) for Regions and Value results, with `Validate(args,
inputs)`; goja-cv writes named data files itself; own writers with an honest round-trip table.

### Prompt Context

**User prompt (verbatim):** (see Step 4)

**Assistant interpretation:** Same as Step 4, for GCV-003.

**Inferred user intent:** Same as Step 4.

**Commit (docs):** f6b03be — "GCV-003: revision 2 from the review read (…)"

### What I did

- Rewrote the guide: §3.1 coverage, §3.3 core/ring with the gray plane and `ringCoverage`, §3.5 naming table, §3.6
  result pipeline trace and directories table; points-only `sample`/`measure`; `regions.measure` with the `mem.hot`
  core rule and `Region.Coverage`; schema with `RemovedFields` and `Produces`; `data` argument type; `writeContent` and
  the `--json` `files` list with the script-83 command; budgets table; writers and the format table; new decision
  records; corrected counts (30 rows, 20 columns).
- Appended the disposition table; replaced the 13 tasks with 14; doctor passed.

### Why

- Two meanings of `x, y` would have made `img.sample(rows)` fail with an error about region box corners in a ticket
  whose purpose is one schema.

### What worked

- The `mask.ellipse` coverage rule, kept rather than replaced, makes the helper-equivalence test exact.

### What didn't work

- N/A

### What I learned

- `resultcontent.WriteFiles` names files `<prefix>-<N><ext>` and skips dropped items, so it cannot replace the CLI's
  writer without an upstream change; keeping the writer in goja-cv avoids that dependency.

### What was tricky to build

- Content budgets: go-go-goja counts every item against one limit; the session has to enforce look and data counts
  itself and set the transport limit to their sum.

### What warrants a second pair of eyes

- The `FuncDef.On` handle types (`Data`) as receivers for `save`.
- The 256 KiB MCP data limit.

### What should be done in the future

- N/A

### Code review instructions

- Read §3.5, §7.5, §7.9, §7.10 and §13.2.

### Technical details

- Column count of the §7.9 example: 9 geometry fields + `dogMaxL` + 10 ring fields = 20.
