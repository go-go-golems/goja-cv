---
Title: Review read findings
Ticket: GCV-002
Status: active
Topics:
    - looks
    - views
    - agents
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-002--inspection-views-and-looks-crops-zoom-rulers-sheets-arithmetic-high-pass/design-doc/01-design-and-implementation-guide.md
      Note: the guide under review (not edited)
    - Path: pkg/cv/graph/eval.go
      Note: evaluator; the render-scale branch is proposed next to evalRun.at/eval
    - Path: pkg/cv/graph/registry.go
      Note: OpDef fields (Halo, Global, DocSize, AnalysisScale) the guide builds on
    - Path: pkg/cv/graph/node.go
      Note: internNode fixes DocW/DocH at build time (sheet size problem) and hash-conses nodes (Node.Macro problem)
    - Path: pkg/cv/graph/regions.go
      Note: Rasterize maps the whole doc rectangle; no window, so overlay on a crop needs a change
    - Path: pkg/cv/ops/present.go
      Note: drawRegions size check, compare modes and DocSize
    - Path: pkg/cv/ops/tone.go
      Note: levels Identity short-circuit interacts with levels({ auto })
    - Path: pkg/cv/graph/suggest.go
      Note: op Synonyms is one global map; no collision test exists
    - Path: pkg/cvsession/processor.go
      Note: TypeError translation the guide relies on for cv.view
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
Summary: "Independent review read of the GCV-002 design guide: 6 P1, 16 P2, 15 P3 findings on concept introduction, correctness against the code, gaps, guideline conformance, implementability and architecture."
LastUpdated: 2026-11-09T18:00:00-05:00
WhatFor: "Fixing the GCV-002 guide before it goes to an implementer."
WhenToUse: "When revising the GCV-002 design guide, or when implementing it and something in the guide looks off."
---

# Review read findings — GCV-002 design and implementation guide

## Goal

This document is an independent review of `design-doc/01-design-and-implementation-guide.md` of GCV-002. I
read it cold, top to bottom, as an intern who knows Go and some JavaScript but has never seen goja-cv. I then
checked every `file:line` claim against the code at `115d7dc` (the last commit touching `pkg/`) and checked the
proposed APIs against the registry, evaluator, look and session code. I read the parts of GCV-001 the guide
depends on (§3, §7.2, §7.5–§7.9, §9.2, §9.4, §12) and checked GCV-003 for consistency. The guide was not
edited.

Findings are ranked:

- **P1** findings must be fixed before the guide goes to an implementer: as written, the design does not work,
  or a core concept is not introduced well enough to implement it.
- **P2** findings would cost an implementer a question, a wrong turn or a failing test.
- **P3** findings are accuracy nits, polish and small architectural improvements.

Each finding gives its location (section heading and a short quote), what is wrong, the evidence, and a
concrete fix.

## Overall assessment

The guide is strong. Its central rule ("anything that depends on pixel content or data is a node; anything
that depends only on the output raster is a look option") is the right one, it is applied consistently, and it
lets one mechanism (render scale with a decoration flag) carry zoom, overlays, annotations and legible text.
The current-state section is accurate: almost every line reference checks out (table at the end). The
legacy-removal list is real removal with no compatibility shims. The decision records are in the required
format, the required sections are all present, and the prose has no analogies and very little filler.

The weaknesses cluster in three places. First, the vocabulary of pixels and scales (doc px, buffer px, screen
px, image px; look, render and evaluation scale) is used before it is defined and drifts between names, and
it is exactly the vocabulary the new mechanism rests on. Second, the decoration rule is specified only for
the order "pixel ops, then decorations"; the guide's own examples use the other order and would draw 8 px
outlines. Third, the contact sheet's size cannot be known when the node is built, and the documented defaults
do not fit the colleague's acceptance case.

| rank | count |
|---|---|
| P1 | 6 |
| P2 | 16 |
| P3 | 15 |

## P1 findings

### P1-1. The pixel and scale vocabulary is never set up, and it drifts

**Location.** Intro ("The vocabulary of the CVJS-001 intern guide (node, op, evaluation, scale, doc px, look)
is assumed"), §1 ("a **render scale** above 1"), §3.1 ("the **look scale** (buffer px per doc px)"), §3.2
("The **scale** of an evaluation is buffer px per doc px … an 8 × 8 block of screen pixels"), §7.7 table
("render scale (screen px per doc px)"), captions ("1 image px = 0.125 doc px").

**Problem.** The whole ticket rests on keeping four kinds of pixel and three kinds of scale apart, and the
guide never puts them side by side:

- Pixel units: *doc px* is never defined in this guide. *Buffer px*, *screen px* and *image px* are used for
  what is, in the zoomed case, the same output pixel, and for different things once ruler margins exist.
- Scales: *look scale* (§3.1), *render scale* (§1, §3.3, §7.7) and *evaluation scale* / "scale of an
  evaluation" (§3.2, §3.3) appear without a statement of how they relate. The key new fact, "for pixel ops
  the evaluation scale is min(render scale, 1); for decorations it equals the render scale", is only implied
  by the pseudocode in §7.2.

The user asked specifically that the guides introduce "render scale vs evaluation scale" and "doc px". An
intern who has not read CVJS-001 cannot follow §3.2–§3.3 without them, and an intern who has will still
wonder whether "screen px" and "buffer px" differ.

**Fix.** Add a §3.0 "Pixels and scales" before §3.1 with a table and one worked example, and use only these
names afterwards:

| term | definition |
|---|---|
| doc px | a pixel of the original image; every coordinate, length and box an agent passes is in doc px |
| buffer px | a pixel of a computed buffer |
| scale | buffer px per doc px |
| evaluation scale | the scale a node's buffer is computed at; at most 1 for pixel ops |
| render scale | the scale of the look's raster; may be above 1 (zoom); decorations are evaluated at it |
| image px | a pixel of the delivered look (the caption's unit): a buffer px of the root at the render scale, plus ruler margins |

Worked example: a 1024 × 1024 tile, `crop([1300, 100, 64, 64])`, `look({ zoom: 8 })`. The crop is
evaluated at scale 1 into a 64 × 64 buffer; the evaluator upsamples it to 512 × 512 at render scale 8; a
decoration draws a 1 image px outline on the 512 × 512 buffer; the ruler adds 28 px margins on all four sides,
so the delivered PNG is 568 × 568. Then replace "screen px" by "image px" everywhere.

### P1-2. Decorations placed before a pixel op are drawn at scale 1 and upsampled; the guide's own examples do this

**Location.** §7.2 pseudocode (`if s > 1 and not n.Op.Decoration: v := at(1).eval(id); return
memo(upsample(v, s, interp))`), §7.8 ("marks outside the frame are skipped (a crop of an annotated image
shows only its marks)"), §7.9 (`views: [...]`), and handover issue 1's acceptance ("look at a 64 px window of
any view (plain, high-pass, overlay) at 8× in one eval").

**Problem.** The rule only works when decorations come last. In `img.annotate(marks).crop(box).look({ zoom:
8 })` the root is `crop`, a pixel op, so the evaluator evaluates it at scale 1, which evaluates `annotate` at
scale 1 (1 px lines in doc px), and then upsamples the result ×8: 8 px outlines covering the speck, the exact
failure §4.4 sets out to fix. The same happens to `img.overlay(r).crop(box)`, to `levels({ auto })` after an
overlay (whose percentiles then also include the tint), and to any decorated view passed to `cv.sheet`. §7.8's
parenthetical describes the annotate-then-crop order as supported. The overlay case of issue 1's acceptance
test is therefore either broken or only works in the order `img.crop(box).overlay(r)`, which the guide does
not state and which runs into P2-10.

**Fix.** Make the ordering rule explicit and general:

1. Classify ops in three groups, not two. *Pixel ops* compute at scale ≤ 1. *Geometric ops* (`crop`, and later
   a window or a flip) only select or move pixels; they are exact at any scale, so they run at the scale they
   are asked for. *Decorations* run at the render scale. With `crop` geometric, `annotate → crop` works: the
   annotation is drawn at 8 and the crop cuts the 8× buffer (the cut box is the doc box × s).
2. For pixel ops applied to a decoration's output (`overlay(…).levels(…)`, `overlay(…).gaussian(…)`), emit a
   build-time warning `DECORATION_FIRST` ("levels after overlay sees the drawn outlines; put overlay last.
   fix: img.levels(…).overlay(r)"), in the style of `CROP_FIRST`.
3. Say what `sheet` does with a decorated view: either refuse it with a fix, or let sheet evaluate decorated
   views at the cell zoom.
4. Add the order cases to the Phase 2 and Phase 6 tests: `annotate → crop` at zoom 8 has 1 px outlines;
   `overlay → levels` warns.

### P1-3. Macro captions via a mutable `Node.Macro` field break hash-consing and contradict GCV-001

**Location.** §7.7 Captions ("macros appear under their own name (`view(speck)`, `highpass(3)`) by recording
the macro call on the first node of its expansion (`Node.Macro string`)"); §8 decision "high-pass and DoG are
macros" ("`img.highpass(3)` and the hand-written chain are the same node, which tests assert").

**Problem.** Three things are wrong.

1. Nodes are hash-consed (`Builder.internNode`, `pkg/cv/graph/node.go`): the expansion of `img.highpass(3)`
   *is* the node of the hand-written chain, as the decision record requires. A field set on that node when the
   macro expands also relabels the user's hand-written chain, and its value depends on which call happened
   first in the session.
2. "The first node of its expansion" is the inner `gaussian`. The caption chain comes from `Evaluator.Path`
   (`eval.go:348-363`), which follows `Inputs[0]`; for `sub(a, gaussian(a))` it goes `sub → a`, so the
   gaussian is never on the path and its `Macro` field is never seen. The name has to sit on the expansion's
   output node, and the walker has to know where the expansion started so it can skip the inner nodes.
3. GCV-001 §7.8 says the opposite: "Because they expand at build time, `explain()` and look captions show the
   primitive chain that actually ran, which is what issue 22 asks for." Issue 22 itself says "The look caption
   shows the expanded op chain". The guide changes this without a decision record.

**Fix.** Keep provenance outside the node. The builder keeps a side table `macroCalls map[NodeID]MacroCall{Label
string; Input NodeID}`, filled for the expansion's output node, first writer wins (deterministic within a
session, and documented as such). The caption walker, on reaching a node with an entry, prints the label and
jumps to `Input`. Add a decision record "captions show macro names, `explain()` shows expansions", state the
deviation from issue 22 and from GCV-001 §7.8, and update GCV-001 §7.8's sentence so the two guides agree.

### P1-4. A contact sheet's size is not known when the node is built

**Location.** §7.9 ("A sheet too large for 1568 px is `E_ARG_RANGE` at build time with fixes for `cols`,
`size` and `zoom`"), and the item type "Regions, or a list of …".

**Problem.** A node's size is fixed when it is interned (`OpDef.DocSize(args, inputs)`, `node.go:131-140`),
and `look.New` computes the look's size from it at call time (`look.go:83-86`). A sheet over a Regions node
(`spots.top(36)`, the headline example) has an item count that is only known after evaluation: `top(36)`
yields at most 36, `components()` any number. So the sheet has no doc size at build time, the build-time size
check cannot run, and the caption's size is unknown. The guide does not address this.

**Fix.** Pick one rule and state it. The simplest one that fits the graph: `sheet` takes a static upper bound
`max` (default 36, `Static: true`), lays out for `max` cells at build time (which fixes the doc size and allows
the build-time check), fills unused cells with the background, and warns at evaluation when there are more
items than cells (`SHEET_TRUNCATED: 52 items, 36 shown. fix: regions.sortBy("contrast").top(36)`). For
literal point and box lists the bound is the list length. Add this to §7.9, §9.2 and the Phase 6 tests.

### P1-5. The documented sheet defaults do not fit issue 3's acceptance case, and the §7.9 example exceeds the cap fivefold

**Location.** §7.9 ("At the defaults, 36 items of one view fit as 6 × 6 cells of 256 px"), the example
`cv.sheet(spots.top(36), { views: [img.levels({ auto: true }), img.view("speck")], size: 48, zoom: 4, … })`,
and the layout rule "several views → one row per item".

**Problem.** At the defaults (size 64, zoom 4, 6 px gap) six cells take 6 · 256 + 5 · 6 = 1566 px, which fits
horizontally. But each cell has a label line under it ("7 (1337, 131)" drawn with `px = 2`: 14 px of glyph plus
spacing, about 18 px). Vertically, 6 · (256 + 18) + 5 · 6 = 1674 px > 1568, so issue 3's acceptance ("36
candidates at 64 px × 4 fit in one look") fails at the guide's own defaults. The §7.9 example with two views
puts one item per row: 36 rows of 192 + 18 + 6 px is about 7 800 px tall. The headline example is an
`E_ARG_RANGE`; at size 48 and zoom 4, two views fit 7 items per sheet.

**Fix.** Put the layout arithmetic in the guide (cell = size · zoom; row height = cell + label band + gap; the
constants). Draw labels inside the cell, on a dark band along its bottom edge, which makes 6 × 6 cells of 256
px fit (1566 px both ways); shrinking the font alone does not (with `px = 1`, 6 · 266 + 30 = 1626 px). For
several views, lay out groups (one item's views side by side) in a grid instead of one row per item, and give
the build-time error the number of items that fit. Replace the example with one that fits (36 items of one
view, or `top(6)` with two views), and add a test asserting that 36 items fit at the defaults.

### P1-6. Sheet and the decoration concept contradict each other

**Location.** §1 ("**decoration** ops (overlay, annotate, sheet labels) draw at the render resolution"), §3.3
("labels on a sheet" listed as a decoration), §7.9 ("A sheet is evaluated only at scale 1 … a look of it
cannot use `zoom`").

**Problem.** If `sheet` has `Decoration: true`, its inputs are "evaluated (and upsampled) at that scale"
(§7.2), which contradicts "the views are evaluated at scale 1" and would upsample whole views before cutting
cells. If it does not, §1 and §3.3 are wrong. An implementer has to guess, and the two guesses produce
different sheets. Nothing in the evaluator today pins a node to scale 1 either: `await sheet` evaluates at the
preview scale (`Engine.Evaluate` with `PreviewLong` 1600), so "evaluated only at scale 1" needs a mechanism.

**Fix.** State that `sheet` is not a decoration. It is a layout op whose output pixels are image px; it draws
its own labels at its own internal zoom. Give it `AnalysisScale: func(Args) float64 { return 1 }` so its
inputs are always evaluated at 1 (the existing mechanism, `registry.go:117-121`), and say what a preview of a
sheet returns (the scale-1 sheet, which is at most 1568 px). Remove "sheet labels" from §1 and §3.3.

## P2 findings

### P2-1. The prerequisites list is incomplete, and several terms are used without definition

**Location.** Intro ("Read GCV-001 §3 (concepts) and §7.5 (space resolution) first").

**Problem.** The guide also depends on GCV-001 §7.2 (`Frame`), §7.6 (the `Level` unit, used in §7.4's table as
"Image or number (Level)"), §7.7 (the display rule used in this guide's rendering pipeline), §7.8 (`Expander`,
`x.Call`) and §7.9 (`colormap` and its one-channel restriction). Terms used with no definition or pointer:
*hash-consing* (§7.1, §7.5), *argument lifting* / "lifted expression" (§4.7, §7.6), `AnalysisScale` (§7.2),
`Global` (§7.6), `Halo` (§7.3, explained only in passing), *RANGE warning*, *planted scan* (§13.2, §13.3), *pi
harness* (§13.5), *the generic inspection list* and "A4" (§7.11, §8; the file is only named in §15).

**Fix.** Extend the prerequisites line to the five GCV-001 sections, and add a short "Terms this guide assumes"
table after the intro (one line each, with where it is defined): node, op, kind, hash-consing, lifting,
`AnalysisScale`, `Halo`, `Global`, `Level`, macro, frame, planted scan, pi harness.

### P2-2. High-pass and DoG are explained without numbers, and `r`, σ and σ_r collide

**Location.** §3.6 ("A Gaussian blur of radius r keeps structures larger than about r … The colleague's best
view is a DoG with σ 1 and 4 … (a − G_r a) / σ_r, where σ_r is the standard deviation in a window of radius r").

**Problem.** goja-cv's `gaussian(r)` takes the *standard deviation* (`pkg/cv/ops/filter.go:14-15`, "standard
deviation of the Gaussian"), not a radius. The guide calls it "radius r", then "σ 1 and 4", then uses σ_r for a
local standard deviation of pixel values. The section also has no worked numbers, which the writing guide asks
for and which GCV-001 §3 does well. As written it does not explain *why* the DoG beats the plain high-pass in
§2.1's measurement.

**Fix.** Say once: "`r` is the Gaussian's standard deviation σ in doc px; the blur reaches about 3σ (its
`Halo`)". Rename the local spread to s_r. Add a small worked example with numbers. Model the speck as a
Gaussian blob of σ 1 px and height 0.06 on a flat sky, with white grain of ±0.01 per pixel:

| view | speck peak | grain spread | ratio |
|---|---|---|---|
| high-pass, r = 4 | 0.06 − 0.06/17 ≈ 0.056 | ≈ 0.010 | ≈ 5.6 |
| DoG, r1 = 1, r2 = 4 | 0.06/2 − 0.06/17 ≈ 0.026 | 0.010 · 0.28 ≈ 0.003 | ≈ 9 |

The first Gaussian halves the speck but cuts the grain to 28 %, which is why the DoG ranks specks higher.

### P2-3. The node-versus-look-option rule is not in the Concepts section

**Location.** §1 (principle in bold), §7.1 (table). §3 has no subsection for it.

**Problem.** It is the guide's organizing idea and the answer to "why is crop not a look option", but a reader
of §3 meets looks, zoom, decorations and crops without it. The user asked specifically for "nodes vs look
options" to be introduced.

**Fix.** Move §7.1's table and its two "because" sentences into a §3.x "Nodes and look options", with one
example of each failure mode (a look-option crop cannot be measured or put in a sheet; a zoom node would have
pixels that are not doc px). §7.1 can then refer back.

### P2-4. The evaluator pseudocode upsamples every kind; Values and Regions above scale 1 are unspecified

**Location.** §7.2 (`if s > 1 and not n.Op.Decoration: … return memo(upsample(v, s, interp))`; "Regions and
Values are evaluated at their own scales as today").

**Problem.** "As today" means at the run's scale unless the op has an `AnalysisScale` (`eval.go:163-172`).
Inside a decoration run at s = 8, a Regions input without `AnalysisScale` (`where`, `top`, `sortBy`) or a
lifted Value (`stats().p99.mul(5)`) takes the `s > 1` branch and is passed to `upsample`.

**Fix.** Write the branch by kind: Image and Mask are evaluated at 1 and upsampled; Regions and Value are
evaluated at 1 (or at their `AnalysisScale`) and returned unchanged, and their signature does not include
`interp`. Add a Phase 2 test with a lifted Value inside a zoomed look.

### P2-5. "Only looks may ask for scales above 1" has no mechanism

**Location.** §6 item 10, §7.2 (`Scale … above 1 only for looks`), Phase 2 tests ("scale > 1 outside looks
are errors").

**Problem.** `EvalOptions` carries only `Scale` and `Interp`; the evaluator cannot tell who called it.
`cv.evaluate(node, { scale })` goes through the same `Engine.Evaluate`.

**Fix.** Either add `EvalOptions.Render bool`, set only by `look.Render`, with `Evaluate` rejecting s > 1
without it, or keep `Evaluate` at (0, 1] and add an `Engine.Render(ctx, id, RenderOptions)` used only by looks.
State which in §7.2.

### P2-6. `expandHighpass` passes an empty `space` when the caller gives none

**Location.** §7.5 code (`sp := args.String("space")` … `map[string]any{"space": sp}`).

**Problem.** GCV-001 §9.2 calls `op.Expand` *before* resolving the space (`if op.Expand != nil: return
op.Expand(...)` precedes `s := args.space or op.Space.Default`), and the `space` option is `Optional` with no
default. `args.String("space")` is therefore `""` for `img.highpass(3)`: the inner `gaussian` call either fails
enum validation or, if empty means absent, runs at gaussian's own default (linear) while `sub` runs encoded,
which silently changes the stated default "space encoded". Also, `Register` only appends `space` to ops whose
`Space.Default` is not `SpaceNone`, so the macro needs a declared `SpaceRule`.

**Fix.** Either resolve the space before `Expand` in GCV-001 §9.2 and pass the resolved value in `args`, or
write the excerpt as `sp := cmp.Or(args.String("space"), "encoded")`. Give `highpass`, `dog` and `view` a
`Space: {Default: SpaceEncoded}`. Add "highpass without `space` runs both the gaussian and the sub in encoded
space" to the Phase 5 tests.

### P2-7. `levels({ auto })` is short-circuited by levels' identity check, and exclusivity cannot be detected

**Location.** §7.6 ("`auto` and `black`/`white` are mutually exclusive … With `auto`, levels becomes a `Global`
op").

**Problem.** Three code facts the guide misses:

1. `levels.Identity` returns true when black = 0, white = 1 and gamma = 1 (`pkg/cv/ops/tone.go:39-41`), and the
   evaluator returns the receiver unchanged when Identity holds (`eval.go:291-293`). `levels({ auto: true })`
   leaves black and white at their defaults, so as written it is a no-op.
2. `black` and `white` have `Default` values, so after binding `Args.Has("white")` is always true; a
   `Validate` function cannot tell `levels({ auto: true, white: 0.8 })` from `levels({ auto: true })`.
3. `Global` is a static `bool` on `OpDef` (`registry.go:109`) and nothing reads it (only op declarations
   mention it), so "levels becomes a Global op when `auto` is set" can neither be expressed per call nor have
   an effect.

**Fix.** Make Identity return false when `auto` is set; make `black` and `white` `Optional`, with the defaults
applied in `Run`, or record which keys were passed explicitly; drop the `Global` sentence, or change `Global` to
`func(Args) bool` and name its reader (the future windowed evaluation, §14).

### P2-8. Arithmetic operands "Image or number" have no ArgSpec type

**Location.** §7.4 table ("Image or number (Level)", "Image or number (unitless factor)").

**Problem.** `ArgSpec.Type` supports a node kind ("Image", "Regions|Mask") or a scalar ("number"), not a union
of the two (`registry.go:65-71`). Numbers may also be lifted Values (`img.sub(img.stats().meanL)`), which is
where GCV-001's `Level` unit and `SPACE` warning matter. The guide does not say how `a.add(b)` is declared.

**Fix.** Specify the declaration, for example `{Name: "b", Type: "Image|number", Unit: graph.Level,
Positional: true}` with a binder extension (an Image node is lifted as an image input; a number or Value node
is a Level), or two registry entries per op. Add the binder change to Phase 4 and a test for a lifted Value
operand with a `SPACE` warning.

### P2-9. The crop ArgSpec uses names that do not exist yet

**Location.** §7.3 (`Type: "box"`, `DocFrame: func(…) { return clip(a.Box("box"), in[0].Frame) }`).

**Problem.** The `box` argument type is introduced by GCV-003 §7.6, which comes after this ticket; today boxes
are `Type: "number[]", Len: 4` (`pkg/cv/ops/mask.go:149`). `Args.Box` does not exist (`args.go` has `Numbers`,
`XY`, `Lengths`). `DocFrame` appears in GCV-001's pseudocode (§9.2) but not in GCV-001's list of `OpDef`
changes (§7.5), and the existing `DocSize` takes `inputs []*Node`, not `in`.

**Fix.** Use `Type: "number[]", Len: 4, Unit: graph.DocPx` here and let GCV-003 switch it to `box`; read it with
`a.Numbers("box")`; name `DocFrame` explicitly as a GCV-001 change and add it to GCV-001 §7.5's `OpDef` block.

### P2-10. Overlay as a decoration on a cropped image needs a framed rasterizer; the guide says it already works

**Location.** §7.8 ("it rasterizes region pixels at s (the area-exact `RegionsValue.Rasterize`,
`pkg/cv/graph/regions.go:194-240`, already works at any target size)").

**Problem.** `Rasterize(w, h, scale)` maps the *whole* doc rectangle onto a w × h buffer (`rx :=
float64(w)/float64(max(1, v.DocW))/sx`). For a crop with origin (1300, 100) rendered at 8, it would squeeze the
full 1024 px tile into 512 px. `drawRegions` also refuses regions whose size differs from the image's
(`present.go:25-27`), so `img.crop(box).overlay(regions)` fails as the code stands, and the guide does not say
how a full-frame Mask input to `overlay` (or to `within`, or `blend`'s `mask`) meets a cropped image.

**Fix.** Change `Rasterize` to take a target frame (origin and size) and a scale; let overlay and annotate accept
inputs whose frame contains the image's frame (cut masks to the image frame, offset regions by the origin);
state the rule for two-image ops (equal frames, as GCV-001 §7.10 says) versus decorations (containing frame).
Add a golden test of `img.crop(box).overlay(regions).look({ zoom: 8 })`.

### P2-11. Diverging colormaps are applied to three-channel images

**Location.** §7.4 example (`d.colormap("diverging")` with `d = img.sub(img.gaussian(3, …))`), §7.11 presets
`signed` (`highpass(s), colormap("diverging")`) and `detail` (`localContrast(4s), colormap(…)`).

**Problem.** GCV-001 §7.9: "`colormap` accepts only 1-channel images (`E_WRONG_KIND` …)". On a colour image `d`
has three channels. The planted scan used by the docs tests is RGB (`internal/testimage/planted.go:43`, a
`[3]float64` per pixel), so the §7.4 block and every `signed`/`detail` example would fail
`TestDocsCodeBlocksRun`.

**Fix.** Start `signed` and `detail` with `toGray("average")`, as `speck` does, and do the same in the §7.4
example. Name the RGB input explicitly in the Phase 5 test "every preset builds and renders".

### P2-12. `compare` as one op with a macro mode and kernel modes has no mechanism; side-by-side output has the sheet's frame problem

**Location.** §6 item 5, §7.10 ("`compare` becomes `{ Name: "compare", On: KindNone, … }` and its `diff` mode
is a macro").

**Problem.** `Expand` is per op, not per argument value: one `OpDef` cannot be a macro for `mode: "diff"` and a
kernel for `side`/`split`. Today `compare` is `On: KindImage` (`present.go:114`); moving it to `KindNone`
removes `img.compare(b)`, which the legacy list does not mention. And `compare({ mode: "side" })` produces an
image whose width is the sum of its inputs plus gaps (`present.go:128-142`): its pixels are not doc px, exactly
like a sheet's, yet only sheets get a "sheet frame". A `crop` or `ruler` on a side-by-side compare would give
wrong coordinates.

**Fix.** Generalize the sheet frame into a *picture frame* (a frame kind, set by every layout op: `sheet`,
`compare` side and split), with one check in the binder: a `DocPx` argument on a node with a picture frame is
`E_WRONG_KIND`. Split `compare`: keep `cv.compare(images, { mode: "side" | "split" })` as a layout op, and drop
`mode: "diff"` in favour of `a.sub(b)`, following the guide's own "one way to do each thing", with an
enum-value redirect fix `a.sub(b).mul(4).add(0.5)`. If `diff` stays, say how `Expand` dispatches on the mode.
List the removal of `img.compare` in §6.

### P2-13. Three API choices lack decision records, two of them deviations from the issues

**Location.** §8.

**Problem.** These choices are made in prose only, and a future reader will re-litigate them:

- Module functions come from the registry (`KindNone` ops exposed as `cv.<name>`) rather than from a function
  table; GCV-003 then adds a second mechanism (`graph.FuncDef`, GCV-003 §7.9).
- Captions show macro names (P1-3), which deviates from issue 22 and GCV-001 §7.8.
- Issue 8 asked for `levels({ auto, box })` and `look({ stretch: "auto" })`; issue 5 asked for a stretch option
  on `highpass` and `dog`. The guide replaces all three with `crop(…).levels({ auto })` and the `speck` preset.
  The reasoning is sound (one sentence in §7.6) but not recorded.

**Fix.** Add three records in the standard format. In the error table, add `look({ stretch })`, `levels({ box
})` and `highpass(r, { stretch })` as `E_UNKNOWN_OPTION` with literal fixes.

### P2-14. The gated view-ranking test depends on a script that does not exist

**Location.** §13.4 ("A short script in this ticket's `scripts/` (`01-export-gold-points.py`, run in the dust
repository with `dustdet.gold`) writes `{tile_id}.gold.json`").

**Problem.** `GCV-002/scripts/` is empty, the `dustdet.gold` API is not described, and the JSON schema of the
points file is not given. The intern cannot write the Go test without the file format, nor the script without
the dust repository's API. GCV-003 §13 reuses this file.

**Fix.** Give the schema (for example `{ "tile": "…", "points": [{ "x": …, "y": …, "class": "dust" | "hair" |
"scratch" }] }`, tile-local doc px), and either write the script now or point at the loader the colleague uses
in `85-cv-views.py`, by function name.

### P2-15. There is no issue-level acceptance checklist

**Location.** §12, §13.

**Problem.** Each phase lists unit tests, but nothing ties the ticket's completion to the colleague's acceptance
statements: issue 1 (one eval, a 64 px window of plain, high-pass and overlay views at 8×), issue 2 (ticks
follow crop coordinates), issue 3 (36 candidates at 64 px × 4 in one look, referable by id), issue 14 (labels
legible at any zoom), issue 15 (PNG on request), issue 22 (a docs page per preset), issue 23a (the chain in the
caption).

**Fix.** Add a §13.6 "Acceptance" with one test per issue, written as the agent's JavaScript and run by
`pkg/cvsession` against the planted scan.

### P2-16. Synonym claims rely on a test and a translation that do not exist

**Location.** §10 ("Each synonym is checked for collisions with existing op names by the existing synonym
test"; "`contactSheet`, `grid`, `mosaic` → `sheet` (on `cv`)"; "`cv.view(...)` | unknown member of `cv` | fix
`img.view("speck")`"), §8 ("`cv.view` is answered by the existing 'op called on the wrong object'
translation").

**Problem.** The only synonym test is `TestOptionSynonyms` (`pkg/cv/ops/ops_test.go:255-288`), which covers
option synonyms; nothing checks `graph.Synonyms` against op names. `cv` is a plain JavaScript object, so
`cv.grid(…)` is a raw `TypeError` that never reaches `graph.Synonyms`. The existing translation
(`pkg/cvsession/processor.go:432-447`) does produce the fix `img.view(…)` for `cv.view`, but its hint reads "cv
is plain data here, not a node", which is wrong for `cv`.

**Fix.** Add the collision test to Phase 7 as new work. Extend `explainDataMisuse` with a `cv` receiver case that
consults `graph.Synonyms` and the module-op list and says "view is an Image method". Drop the word "existing".

## P3 findings

1. **§7.6 "`Global` op (`registry.go:110`)".** `Global` is at line 109; line 110 is `NoCache`. Fix the
   reference.
2. **§9.1 trace node ids and label.** The `speck` expansion has nine nodes (toGray, two gaussians, sub, max,
   stats, `.p99`, mul, levels), so with #5 as its first node crop is #14 and levels #15, not #13 and #14. The
   caption says "speck view ×8" although the call passes no label and the default label is the last op (§7.7
   table). Pass a label in the call, or show the default.
3. **§7.11 `speck` vs the measured view.** The colleague's view is `pos(img.toGray('average').gaussian(1), 4)`
   (`84-cv-helpers.js:22`, `85-cv-views.py:46`), that is G₁(a) − G₄(G₁(a)), whose outer σ is √17 ≈ 4.12, not
   G₁(a) − G₄(a). Say that the preset differs slightly and that the gated test (§13.4) is what validates it.
4. **§7.3 crop alignment.** "The frame records the aligned box" makes a frame depend on the evaluation scale,
   while GCV-001 makes frames static and part of the hash. Keep the frame as the requested (clipped, integer)
   box and let the evaluated value carry its buffer origin for rulers at s < 1.
5. **§7.3 entirely-outside box.** "A clipped box as the fix" is empty for such a box; the fix should be the
   nearest valid box of the same size.
6. **§7.7 rulers.** Say whether the 1568 px cap includes the 28 px margins, and how labels are thinned when the
   tick spacing is smaller than the label width (`ruler: 1` at zoom 8 gives 8 px spacing and 24 px labels).
7. **§7.9 example `spots.top(36)`.** `top` takes "the first n regions in the current order"
   (`pkg/cv/ops/regions.go:345`); after `components` that is raster order, not prominence. Use
   `.sortBy("contrast").top(36)` or similar.
8. **`show(node, opts)` and chart looks.** `show` (`pkg/cvsession/init.js:41`) takes the same options and must
   bind them through `BindOptions` too; say what `zoom`, `ruler` and `format` do on histogram charts.
9. **§7.10 `__moduleOps` excludes `open`, `param` and the placeholder by name.** Use GCV-001's `Internal` flag
   for the placeholder and a registry flag for ops with hand-written bindings, so the exclusion is data, not a
   list of names. Say how this relates to GCV-003's `FuncDef`, the second mechanism for module functions.
10. **Two ad-hoc redirect tables.** The `look({ box })` literal fix and the `blend` enum-value redirect table
    both say "this spelling is gone, write that instead". One `ArgSpec.Redirects` (for options and enum values)
    would serve both, and GCV-003's removed fields too.
11. **Phase ordering.** Phase 2 tests "decorations receive upsampled inputs" before any decoration exists
    (overlay becomes one in Phase 6); register a test-only decoration op. Each phase should end with `go test
    ./... -count=1`, `make lint` and a commit, as GCV-001 §12 states.
12. **Style.** §13.1 bullets and the phase steps are fragments ("Upsampling exactness (`nearest`), decoration
    scale, cache keys."); the writing guide asks for complete sentences. Only §3 closes with key points; §7.2
    and §7.9 would benefit from them.
13. **`pkg/cv/text` glyphs.** The intern must draw about 90 glyphs of 5 × 7. Point to a public-domain 5 × 7
    font table and state its licence, or include the table in the ticket.
14. **Blend modes with opacity or mask.** `blend({ mode: "lighten", opacity: 0.5, mask })` stays expressible as
    `a.blend(a.max(b), { opacity: 0.5, mask })`; say so in the redirect fix. Note also that `blend` runs in
    linear space and arithmetic in encoded space, so `add` replaces `blend`'s `add` exactly only with `{ space:
    "linear" }`.
15. **Op synonyms that point the wrong way.** `zoom` → `crop` and `normalize` → `localContrast` will mislead: an
    agent writing `img.zoom(8)` wants `look({ zoom: 8 })`, and "normalize" usually means a stretch. Give `zoom`
    a redirect with the literal fix `node.crop([x, y, w, h]).look({ zoom: 8 })`, and map `normalize` to
    `levels`.

## Issue coverage

| issue | covered by | verdict |
|---|---|---|
| 1 crop and zoom | `crop`, `look({ zoom, interp })`, caption | solved for plain and high-pass views; the overlay case needs P1-2 and P2-10 |
| 2 rulers | `look({ ruler, grid })`, root doc-px labels | solved; P3-6 for label density |
| 3 contact sheets | `cv.sheet(items, { views })` | design right; size model and acceptance numbers broken (P1-4, P1-5) |
| 4 signed arithmetic | `add/sub/mul/abs/min/max`, no `offset` (decision record) | solved; operand type unspecified (P2-8) |
| 5 high-pass / DoG | `highpass`, `dog` macros, `speck` preset, views topic | solved; stretch-option deviation unrecorded (P2-13) |
| 8 local contrast, auto stretch | `localContrast`, `crop().levels({ auto })` | solved in substance; levels identity bug (P2-7); deviation unrecorded (P2-13) |
| 14 annotations | `annotate` decoration | solved when last in the chain; ordering (P1-2) |
| 15 lossless | `format` with `auto` (decision record) | solved |
| 22 view presets | `img.view(name)` (decision record for `img.` vs `cv.`) | solved; caption deviation (P1-3); colormap channel bug (P2-11) |
| 23a provenance | chain captions | solved in intent; mechanism broken (P1-3) |

## Consistency with GCV-001 and GCV-003

- **GCV-001 §7.8 vs GCV-002 §7.7** on what captions show: contradictory (P1-3).
- **GCV-001 §9.2** runs `Expand` before space resolution; GCV-002's macro excerpt assumes the opposite (P2-6).
- **GCV-001 §7.9** restricts `colormap` to one channel; GCV-002's presets and example ignore it (P2-11).
- **GCV-001 §7.5** does not list `DocFrame` among the `OpDef` changes, though its own pseudocode and GCV-002 use
  it (P2-9).
- **GCV-003 §7.6** introduces the `box` type that GCV-002 already uses (P2-9). GCV-003 §6 item 10 ("GCV-002
  already moved `compare` and `sheet` to registry ops") and its use of `graph.BindOptions` and `pkg/cv/text`
  match GCV-002. GCV-003's `FuncDef` is a second module-function mechanism (P3-9). Topic numbering (10, 11, 12)
  is consistent across the three guides.

## Line references checked

| claim | result |
|---|---|
| `look.go:46-61`, `:66-99`, `:71-86`, `:79-82`, `:151-162`, `:211-225` | correct |
| `engine.go:160-171` (`ScaleForLong`) | correct |
| `eval.go:145-159`, `:176-178`, `:348-363` | correct |
| `registry.go:108` (`Halo`), `:132-133` (`Key`) | correct |
| `registry.go:110` (`Global`) | off by one: line 109 (P3-1) |
| `args.go:556-585` (`checkKeys`) | correct |
| `help.go:24-43`, `:34`, `:51` | correct |
| `evaltrack.go:220-266` | correct |
| `reference.go:15-37`, `:70` | correct |
| `session.go:40-43` | correct |
| `present.go:21-73`, `:41-43`, `:88-112`, `:151-160` | correct |
| `mask.go:387-400` | correct |
| `kernels/draw.go:9-13`, `:100-121` | correct |
| `kernels/composite.go:10-27`, `:29-36` | correct |
| `look/font.go:10-28` | correct (glyph table at 12-23) |
| `kernels/measure.go:115-135` | correct |
| `ops/tone.go:29-57` | correct |
| `ops/value.go:125-147` | correct (Value also has `div` and `distance`) |
| `graph/regions.go:97` (centroid + 0.5) | correct |
| `graph/regions.go:194-240` (`Rasterize`) | lines correct; "already works at any target size" is not true for a crop (P2-10) |
| `prelude.js:305-351`, `:334-342` | correct |
| `look_test.go:151-175` | correct |
| "the existing synonym test" | does not exist (P2-16) |
| `scripts/80-vlm-render.py` line 76 | correct ("Upscale (nearest)") |
| `scripts/84-cv-helpers.js` line 22 | correct (`pos`) |

## Related

- Guide under review: `../design-doc/01-design-and-implementation-guide.md`
- Diary of this review: `02-diary-review-read.md`
- GCV-001 guide: `ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/design-doc/01-design-and-implementation-guide.md`
- GCV-003 guide: `ttmp/2026/11/09/GCV-003--measurement-and-data-out-sampling-profiles-region-measures-schema-export/design-doc/01-design-and-implementation-guide.md`

## Disposition (revision 2 of the guide, 2026-11-09)

Section numbers refer to revision 2 of `design-doc/01-design-and-implementation-guide.md`. "Foundations" is
`GCV-001…/reference/05-foundations-goja-cv-for-the-gcv-guides.md`.

| finding | disposition | where |
|---|---|---|
| P1-1 | fixed: one vocabulary (doc px, buffer px, image px; evaluation, preview, look, render scale) defined in Foundations §F5 with the 64 px × 8 → 512 + rulers → 568 worked example; "screen px" removed | Foundations §F5, §3.3, §3.4, §7.7 |
| P1-2 | fixed: three classes (pixel, geometric, decoration) and render windows; crop is geometric, so annotate → crop draws 1 px lines; `DECORATION_FIRST` warning; sheets render cells through the same path; order tests | §3.4, §7.2, §7.3, §7.9, §9.3, §12 Phases 2, 3, 6 |
| P1-3 | fixed: no mutable node field; GCV-001 macro nodes carry the label; the caption walker follows `Inputs[0]`; decision record states the deviation from issue 22; GCV-001 §7.8 updated to match | §7.7, §8, GCV-001 §7.8 |
| P1-4 | fixed: static `max` (default = what fits); frame laid out at build time; `SHEET_TRUNCATED` at evaluation | §7.9, §9.2, §7.13 |
| P1-5 | fixed: layout arithmetic; labels inside cells; 36 cells at the defaults (1566 px both ways); two views → 18 items in a grid of groups; example replaced | §7.9 |
| P1-6 | fixed: sheet is a layout op (not a decoration), `AnalysisScale` 1, `await` gives the scale-1 sheet; "sheet labels" removed from the decoration list | §1, §3.4, §7.9 |
| P2-1 | fixed: prerequisites list GCV-001 §3, §7.2, §7.4–§7.9 and Foundations; terms are defined in Foundations (hash-consing, lifting, AnalysisScale, planted scan, pi harness) and here (halo §3.7); `Global` no longer used | How to read, §3.7 |
| P2-2 | fixed: `r` is σ; s_σ for local spread; DoG worked example with numbers | §3.7 |
| P2-3 | fixed: §3.1 "Nodes and look options" with both failure modes | §3.1 |
| P2-4 | fixed: dispatch by kind; Regions/Value not upsampled; lifted Value test | §7.2, §12 Phase 2 |
| P2-5 | fixed: `Engine.Render` used only by looks and sheet cells; `Evaluate` stays (0, 1] | §7.2 |
| P2-6 | fixed: GCV-001 resolves the space before `Expand`; macros declare `Space: {Default: encoded}`; test | §7.5, §12 Phase 5 |
| P2-7 | fixed: `Identity` false with `auto`; `black`/`white` Optional with defaults in `Run`; `Global` sentence dropped | §7.6 |
| P2-8 | fixed: `Image\|number` union type with binder rule; Phase 4 test with lifted Value and `SPACE` | §7.4, §12 Phase 4 |
| P2-9 | fixed: `box` type now introduced in GCV-001 §7.4; `Args.Box` accessor named; `DocFrame` defined in GCV-001 §7.5 | §7.3 |
| P2-10 | fixed: `Rasterize(target Frame, scale)`; decorations accept containing frames, pixel ops equal frames; golden test | §7.8, §12 Phase 6 |
| P2-11 | fixed: `signed`, `detail` and the §7.4 example start with `toGray("average")` | §7.4, §7.11 |
| P2-12 | fixed: picture frames for sheet and compare layouts; `compare` diff dropped (redirect to `sub`); `img.compare` removal listed | §6 items 1, 6; §7.9; §7.10 |
| P2-13 | fixed: records for module functions, captions, and stretch options; error rows for `look({ stretch })`, `levels({ box })`, `highpass(r, { stretch })` | §8, §10 |
| P2-14 | fixed: no export script needed; the test reads `~/dustds/gold/{tile_id}.final.json`/`.p1.json` directly (schema and loader named) | §13.4 |
| P2-15 | fixed: acceptance table per issue | §13.6 |
| P2-16 | fixed: "existing" dropped; new `TestOpSynonymsDoNotShadowOps`; `UnknownMember` consults the op synonym table. Note: in sessions `cv.view` is answered by `UnknownMember` (`help.go:279-312`, which already finds Image ops), not by `explainDataMisuse` | §4.9, §10, §12 Phase 7 |
| P3-1 | fixed differently: the `Global` reference is gone (it has no reader) | §7.6 |
| P3-2 | fixed: ids recounted (#2–#10 expansion, #11 macro node, #12 crop); label passed in the call | §9.1 |
| P3-3 | fixed: √17 ≈ 4.12 noted; gated test validates | §7.11 |
| P3-4 | fixed: frame is the requested integer box; values carry their buffer origin below scale 1 | §7.3 |
| P3-5 | fixed: nearest valid box of the same size | §7.3, §10 |
| P3-6 | fixed: cap applies before margins (≤ 56 px added); label thinning rule with the `ruler: 1` example | §7.7 |
| P3-7 | fixed: `sortBy("area").top(36)` with "largest first" | §7.9 |
| P3-8 | fixed: `show` is a registered function with the look ArgSpecs; chart behaviour of `zoom`/`ruler`/`format` stated | §7.7, §7.10 |
| P3-9 | fixed: exclusion by flags (`Internal`, bespoke), relation to `FuncDef` decided in one record | §7.10, §8 |
| P3-10 | fixed: one `ArgSpec.Redirects` mechanism | §6 item 11, §7.7, §10 |
| P3-11 | fixed: test-only decoration in Phase 2; every phase ends with tests, lint and a commit | §7.2, §12 |
| P3-12 | fixed: full-sentence bullets; key points after §7 and §12; closing §15 | §7, §12, §13.1, §15 |
| P3-13 | fixed: glyphs from X11 misc-fixed `5x7.bdf` (public domain) | §7.12 |
| P3-14 | fixed: opacity redirect and the `{ space: "linear" }` note | §7.4, §10 |
| P3-15 | fixed: `zoom` is a redirect to `crop(…).look({ zoom })`; `normalize` → `levels` | §10 |

## Re-check (revision 2)

I re-read revision 2 of the guide (commit `a31654a`), Foundations §F4–§F5, and GCV-001 §7.4, §7.5 and §7.8 as
revised (`48e3a50`). The guide was not edited.

### The six P1 findings

| finding | verdict | evidence |
|---|---|---|
| P1-1 vocabulary | resolved | Foundations §F5 defines doc px, frame, buffer px, scale, evaluation, preview, look and render scale, image px, with the 64 px × 8 → 512 + 28 px margins → 568 example; "screen px" is gone (§3.3 "8 × 8 block of image px"); "How to read this guide" points to §F5. |
| P1-2 decoration order | resolved | §3.4 "Three classes of ops under zoom"; §7.2 `renderEval` dispatches Regions/Value (not upsampled), Decoration (inputs rendered at s, `Run` with `ec.Window`), GeometricOp (`W' := W ∩ frame`, passes s on), macro node (last input), pixel op (eval at 1, cut W, upsample). Traced both orders: `annotate → crop` (§9.3) and `crop → annotate` give 1 image px lines; §7.1's `img.overlay(spots).crop(box).look({ zoom: 8 })` works. `DECORATION_FIRST` covers pixel ops after a decoration. |
| P1-3 macro captions | resolved | No mutable field. GCV-001 §7.8 now interns a macro node `[receiver, expansion output]`; the caption walker follows `Inputs[0]` to the receiver and prints `view(speck)`; expansion nodes stay shared with hand-written chains; GCV-001 §7.8 and GCV-002 agree; decision record "captions show macro names" states the issue 22 deviation. |
| P1-4 sheet size at build | resolved | §7.9 "Size at build time": static `max`, default = what fits; frame laid out at intern time (§9.2); `SHEET_TRUNCATED` at evaluation through the new `EvalCtx.Warn` (§7.13). |
| P1-5 sheet arithmetic | resolved | Labels inside the cell. One view: ⌊(1568 + 6) / 262⌋ = 6 per row, 6 · 256 + 5 · 6 = 1566 ≤ 1568 both ways, so 36 cells. Two views: group 2 · 256 + 2 = 514, ⌊1574 / 520⌋ = 3 groups per row (3 · 514 + 2 · 6 = 1554), 6 rows, so 18 items. I recomputed both; the example is replaced by one view of 36 items; §13.6 issue 3 asserts it. |
| P1-6 sheet vs decoration | resolved | §1 and §3.4 list only overlay and annotate as decorations; §7.9 "It is a layout op, not a decoration", `AnalysisScale` 1, `await` returns the scale-1 sheet; cells come from `Engine.Render`, so decorated views draw thin lines in cells too. |

### P2 dispositions

I checked P2-1 to P2-16 against the cited sections; all are fixed as the table says. My P2-16 premise was partly
wrong: `cv` member misses already go through `UnknownMember` (`pkg/gojamod/cv/help.go:279-312`), which finds Image
ops, as the disposition notes. P2-9's `box` type and `DocFrame` are now in GCV-001 (§7.4 table, §7.5 `OpDef`).

### New findings introduced by revision 2

No new P1.

- **N1 (P2). Box items break the static sheet layout.** §7.9 keeps "boxes are centred on the box centre with window
  side `max(w, h, size)`", and §9.2 renders each cell with `Engine.Render(v, {Scale: zoom, Window: win})`. A 200 px
  hair box at zoom 4 gives an 800 px cell inside a layout of 256 px cells. Fix: render such windows at scale `zoom ·
  size / side` so every cell is `size · zoom` px, and say so in §7.9 and §9.2.
- **N2 (P3). The memory claim is overstated.** §3.4 says "No buffer is larger than the look, at most 1568 × 1568
  image px", and the decision record says "windows keep buffers at look size". Pixel ops are still evaluated at scale 1
  over the whole frame (§7.2 default case, §F5 step 3, §14 first risk: 638 MB per DNG buffer). Only upsampled and
  decorated buffers are bounded. Reword both to say that.
- **N3 (P3). Cell labels overflow small cells.** A label `7 (1337, 131)` at font size 2 is about 156 × 14 px; at
  `zoom: 1` or `size: 32, zoom: 2` the cell is 64 px. State a rule (font size 1, then id only, when the label is wider
  than the cell).
- **N4 (P3). `sheet.Run` calls `Engine.Render`, but `Run` sees an `EvalCtx` whose handle is the `Evaluator`.** Put
  `Render` on the `Evaluator` (with `Engine.Render` delegating), and round each cell window's origin to integer doc px
  (centroids are fractional, `regions.go:97`), so the nearest-neighbour cut is exact.

### Disposition of the re-check (revision 3 of the guide)

| finding | disposition | where |
|---|---|---|
| N1 | fixed: cell window side `max(w, h, size)` with an integer origin, rendered at scale `zoom · size / side`, so every cell is `size · zoom` px; the label shows the reduced scale | §7.9, §9.2 |
| N2 | fixed: only upsampled and decorated buffers are bounded by the look; pixel ops stay whole at scale 1 (12 MB per tile, 638 MB per DNG) | §3.4, §8 (three classes record) |
| N3 | fixed: font size 2, then 1, then id only when the label is wider than the cell minus 4 px | §7.9 |
| N4 | fixed: `Evaluator.Render` (with `Engine.Render` delegating), called by `sheet.Run` through `EvalCtx`; integer window origins | §7.2, §7.9, §9.2 |
| GCV-003 re-check P3-20 | fixed here: `highpass(sigma)`, `dog(sigma1, sigma2)`, `localContrast(sigma)`; `gaussian`'s `r` renamed `sigma`, `feather`/`grow`/`shrink`'s `r` renamed `radius`; option synonyms map `r` to whichever the op has | §3.7, §7.5 |
