---
Title: Design and implementation guide
Ticket: GCV-002
Status: active
Topics:
    - looks
    - views
    - agents
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md
      Note: shared foundations chapter; §F5 defines the pixel and scale vocabulary and the render rule
    - Path: ttmp/2026/11/09/GCV-002--inspection-views-and-looks-crops-zoom-rulers-sheets-arithmetic-high-pass/reference/01-review-read-findings.md
      Note: review findings and their disposition (revision 2)
    - Path: pkg/cv/look/look.go
      Note: look options, scale fit, JPEG-unless-gray; gains zoom, interp, ruler, grid, format, captions
    - Path: pkg/cv/look/font.go
      Note: 5x7 digit font; merged with kernels/draw.go digits into pkg/cv/text
    - Path: pkg/cv/kernels/draw.go
      Note: 3x5 digit font, AbsDiff (removed)
    - Path: pkg/cv/kernels/composite.go
      Note: blend modes; add/difference/lighten/darken removed; one Binary kernel
    - Path: pkg/cv/ops/present.go
      Note: overlay drawn at evaluation scale, unsigned compare diff, compare side DocSize
    - Path: pkg/cv/ops/tone.go
      Note: levels gains auto; Identity must not short-circuit auto
    - Path: pkg/cv/graph/eval.go
      Note: scale limited to (0, 1]; render runs with windows and three op classes
    - Path: pkg/cv/graph/regions.go
      Note: Rasterize maps the whole doc rectangle; gains a target frame
    - Path: pkg/gojamod/cv/evaltrack.go
      Note: look options parsed by hand; bound through BindOptions
    - Path: pkg/gojamod/cv/prelude.js
      Note: cv.compare written by hand; module ops and functions come from the registry
    - Path: pkg/gojamod/cv/help.go
      Note: cvFuncs table, crop redirect, UnknownMember
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/85-cv-views.py
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/80-vlm-render.py
Summary: "Intern guide for GCV-002: the render scale with three op classes (pixel, geometric, decoration) and render windows, crops, zoomed looks with rulers, contact sheets, signed arithmetic, high-pass/DoG/local contrast, auto stretch, annotations, view presets and registry-driven module functions, on GCV-001's image model."
LastUpdated: 2026-11-09T22:30:00-05:00
WhatFor: "Implementing GCV-002 (after GCV-001)."
WhenToUse: "Read before touching looks, overlays, compare, blend, or adding inspection views."
---

# GCV-002 — Inspection views and looks: crops, zoom, rulers, sheets, arithmetic, high-pass

## Revision history

| date | revision | what changed |
|---|---|---|
| 2026-11-09 | 1 | First version (commit e1855d5). |
| 2026-11-09 | 2 | Revised from the review read (`reference/01-review-read-findings.md`, disposition table at its end). Pixel and scale vocabulary moved to Foundations §F5 and used consistently. Three op classes (pixel, geometric, decoration) with render windows, so decorations work before a crop. Captions use GCV-001's macro nodes instead of a mutable field. Sheets are layout ops at scale 1 with a size fixed at build time and defaults that fit 36 cells. `compare` diff removed in favour of `sub`. One registration rule for module functions (ops vs functions). Evaluation-time warnings. Acceptance checklist per issue. |

## How to read this guide

This guide is for an intern who will implement GCV-002 in goja-cv. Read first:

- the shared foundations chapter,
  `ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md`,
  especially §F5 (doc px, buffer px, image px, evaluation and render scale, the render rule and its worked
  example), §F3 (registry and binder), §F4 (evaluation, `AnalysisScale`) and §F9 (looks and result content);
- GCV-001 §3 (encodings, spaces, derived values), §7.2 (`Frame`), §7.4 (`box` and `point` argument types), §7.5
  (`SpaceRule`, statics in `internNode`), §7.6 (the `Level` unit), §7.7 (looks show encoded values), §7.8 (macros
  and macro nodes) and §7.9 (`colormap`, one channel only).

**Dependencies.** GCV-002 needs GCV-001 complete. GCV-003 uses this ticket's render path (profile plots), the
`pkg/cv/text` font, the registration rule for module functions (§7.10), evaluation-time warnings (§7.13), and the
`sheet` and `annotate` ops, which accept GCV-003's region items.

## 1. Executive summary

A vision model can only judge what it can see. Today every goja-cv look shows the whole image at no more than 1568
px on the long side; nothing can be magnified, coordinates cannot be read off the image, at most four looks arrive
per eval, colour looks are JPEG, and the only difference view is unsigned. The colleague's agents therefore leave
goja-cv for a Python script to get an 8× zoom with coordinate ticks, and build their best view (a difference of
Gaussians) out of four `blend` calls.

This ticket makes inspection native, using one principle: **anything that depends on pixel content or data is a
node; anything that depends only on the output raster is a look option.** Nodes: `crop`, signed arithmetic (`add`,
`sub`, `mul`, `abs`, `min`, `max`), `highpass`, `dog`, `localContrast`, `levels({ auto })`, `annotate`, `overlay`,
`cv.sheet`, `img.view`. Look options: `label`, `zoom`, `interp`, `maxSide`, `ruler`, `grid`, `format`.

Two engine changes carry it. A look may ask for a **render scale** above 1 (zoom); ops fall into three classes for
it: pixel ops compute at most at scale 1 and are upsampled, geometric ops (crop) pass the request on, and
decorations (overlay, annotate) draw at the render scale so lines and text stay thin. A **render window** limits
all of this to the visible rectangle. And captions show the **op chain**, including view and macro names.

The ticket removes the unsigned `AbsDiff` kernel and `compare`'s diff mode, the duplicate blend modes, the two digit
fonts, the hand-parsed look options, the hand-written `cv.compare` and most of the hand-maintained help table.

## 2. Problem statement and scope

### 2.1 The workflow

The colleague's annotator agent follows a four-step loop (DUST-002 `reference/06-goja-cv-generic-inspection-features.md`,
"The workflow these features serve"): transform the image so that what matters stands out; decide where to look;
crop and magnify those places; judge them by eye with a few numbers. Steps 1 and 3 are this ticket.

The colleague's view comparison (`scripts/85-cv-views.py`, 142 human-labelled specks on 19 tiles) rendered the look
a model would receive for each view, found all local maxima, and ranked each labelled speck among them:

| view | median rank of a labelled speck | specks among the 50 most visible bumps |
|---|---|---|
| levels-stretched tile (`scene`) | about 715 | 1 % |
| bright difference of Gaussians, σ 1 vs 4, white = 5 × p99 (`dog`) | 18 | 78 % |
| the same on colour (`cdog`) | — | 79 % |

In the DoG view a speck is the 18th thing the eye meets; in the stretched tile it is the 715th, about 40 times later.

### 2.2 The issues

| issue | title | priority |
|---|---|---|
| 1 | Crop and zoom looks | P1 |
| 2 | Coordinate rulers on looks | P1 |
| 3 | Contact sheets of many crops | P1 |
| 4 | Signed image arithmetic | P1 |
| 5 | High-pass / difference-of-Gaussians view | P1 |
| 8 | Local contrast and auto-stretch per box | P2 |
| 14 | Markers and annotations on looks | P2 |
| 15 | Lossless looks on request | P2 |
| 22 | Named view presets | P3 |
| 23a | Look provenance (op chain in the caption) | P3 |

Issue 23b (CLI `--prelude`) is GCV-003. Rank and morphological filters (issue 18) and fibre detection (issue 19)
are out of scope; they can be added later as primitive ops.

## 3. Concepts

### 3.1 Nodes and look options

goja-cv already separates *what an image is* (a node) from *how it is delivered* (a look, Foundations §F9). This
ticket keeps that separation as its organizing rule:

| a parameter depends on | it is | examples |
|---|---|---|
| pixel values or data | a node | `crop`, `sub`, `highpass`, `levels({ auto })`, `annotate`, `overlay`, `sheet`, `view` |
| only the output raster: scale, interpolation, margins, file format | a look option | `zoom`, `interp`, `maxSide`, `ruler`, `grid`, `format`, `label` |

Each misplacement has a concrete failure. If crop were a look option, a cropped window could not be measured
(`hp.crop(box).stats()`), put in a sheet, or shown in `explain()`, and a per-crop stretch would need a second
implementation of `levels`. If zoom were a node, its output pixels would not be doc px, so every coordinate passed
to a later op would be wrong.

### 3.2 What a look is today

`node.look("label")` creates a Go object implementing go-go-goja's `resultcontent.Provider`
(`pkg/cv/look/look.go:46-61`). Rendering evaluates the node at the look scale, chosen so the long side fits 1568 px
(`look.go:71-86`), burns the look number into the top-left corner, and encodes JPEG q85 for colour or PNG for
single-channel images (`look.go:211-225`). The caption carries the scale: `look 3: specks (1568×1045, 1 image px =
4.0 doc px)` (`look.go:151-162`). An eval delivers at most four looks of at most 8 MiB together
(`pkg/cvsession/session.go:40-43`).

### 3.3 Zoom and interpolation

Foundations §F5 defines the render scale: image px per doc px of a look, which may exceed 1. **Zoom** is a render
scale above 1: at zoom 8 each doc px becomes an 8 × 8 block of image px. Zoom adds no information; it makes
existing pixels large enough for a vision model to judge shape, edge softness and neighbourhood. A 3 px speck
becomes 24 image px wide.

**Interpolation** decides what fills the block. `nearest` repeats the pixel, so pixel boundaries stay visible and
values are exact; `linear` blends neighbours and invents intermediate values. For inspection `nearest` is the
default, as in the colleague's zoom script (`scripts/80-vlm-render.py`, line 76: "Upscale (nearest)").

### 3.4 Three classes of ops under zoom

Foundations §F5 states the render rule. Here is why it has three classes.

A **pixel op** computes values from values: exposure, blur, arithmetic, levels. Computing it above scale 1 would
mean computing on invented pixels: a Gaussian of an 8× nearest-upsampled image is not an 8× view of a Gaussian. So
pixel ops never run above scale 1; asked for scale 8, the evaluator computes them at 1 and upsamples.

A **geometric op** only selects or moves pixels: crop (later possibly flip or transpose). It is exact at any scale:
cutting a box out of an 8× buffer gives the 8× buffer of the cut. So it passes the request on, at the scale it was
asked for.

A **decoration** draws marks on an image: region outlines and ids (`overlay`), circles, points, polylines and text
(`annotate`). Its marks are defined in doc px but drawn in image px: an outline should be 1 image px wide at any
zoom, a label readable at any zoom. A decoration therefore runs at the render scale, on an input that the evaluator
upsampled.

Because crop is geometric, both orders work:

```
img.annotate(marks).crop(box).look({ zoom: 8 })    crop at 8 → annotate at 8 draws 1 px lines → img at 1, upsampled ×8
img.crop(box).annotate(marks).look({ zoom: 8 })    annotate at 8 → crop at 8 → img at 1, upsampled ×8
```

A pixel op after a decoration is the one order that cannot work: `img.overlay(r).levels({ auto: true })` computes
levels at scale 1 on an image whose outlines were drawn at scale 1, and its percentiles include the tint. The builder
warns (`DECORATION_FIRST`, §10).

The **render window** (Foundations §F5) keeps memory bounded: at scale s > 1 the request carries the visible doc-px
rectangle, a crop narrows it, and upsampling and drawing touch only that rectangle. No buffer is larger than the
look, at most 1568 × 1568 image px.

### 3.5 Crops and frames

A **crop** is a geometric op whose result has a smaller frame (GCV-001 §3.7): `img.crop([300, 100, 64, 64])` has
frame origin (300, 100) and size 64 × 64. Coordinates passed to ops after a crop, and coordinates reported about a
crop, stay absolute doc px. Rulers on that crop are labelled 300 … 364, not 0 … 64.

### 3.6 Signed images

The difference of two images has a sign: positive where the first is brighter. GCV-001 tags such results as derived
(linear encoding): their stored value is their value. A look clamps to [0, 1], so a signed image needs a mapping
to be seen: add 0.5 so that zero is mid-grey, or use GCV-001's `colormap("diverging")` on one channel, which shows
negative values blue, zero white and positive values red.

### 3.7 High-pass, difference of Gaussians and local contrast

goja-cv's `gaussian(r)` takes the Gaussian's standard deviation σ = r in doc px (`pkg/cv/ops/filter.go:14-15`); the
blur reaches about 3σ, which is its **halo**: the border in doc px an op needs around each output pixel
(`OpDef.Halo`, `registry.go:108`). A blur of σ keeps structures larger than about σ and suppresses smaller ones.

A **high-pass** subtracts the blur: a − G_σ(a) keeps what differs from its surroundings at scale σ. A **difference
of Gaussians** (DoG) blurs twice, with σ1 < σ2, and subtracts: G_σ1(a) − G_σ2(a). The first blur also suppresses
the finest grain, which is why the DoG ranks specks higher. A worked example: a speck shaped like a Gaussian blob of
σ 1 px and height 0.06 on a flat sky with white grain of standard deviation 0.01 per pixel. A 2-D Gaussian blob of
σ_b blurred by σ_g keeps the fraction σ_b² / (σ_b² + σ_g²) of its peak; white noise blurred by σ_g keeps
1 / (2 σ_g √π) of its spread.

| view | speck peak | grain spread | ratio |
|---|---|---|---|
| high-pass, σ = 4 | 0.06 − 0.06/17 ≈ 0.056 | ≈ 0.010 | ≈ 5.6 |
| DoG, σ1 = 1, σ2 = 4 | 0.06/2 − 0.06/17 ≈ 0.026 | 0.010 · 0.28 ≈ 0.003 | ≈ 9 |

The first Gaussian halves the speck but cuts the grain to 28 %.

**Local contrast** normalizes a high-pass by the local spread: (a − G_σ(a)) / (s_σ + 1e-3), where s_σ is the
standard deviation of pixel values in a Gaussian window of σ. Its values are local z-scores: 3 means "three local
standard deviations above the local mean", in a dark corner and in a bright one alike.

### 3.8 Stretch

A **stretch** maps a range of values to the display range [0, 1]: `levels({ black, white })`. An **auto stretch**
takes the range from the image itself, usually its 1st and 99th percentiles. A zoomed crop needs its own stretch: a
64 px window of sky occupies a sliver of the tile's range and looks flat grey under the global stretch.

### 3.9 Contact sheets

A **contact sheet** is one image made of many small crops in a grid, each labelled. Vision models compare similar
crops side by side well, and one sheet of 36 crops replaces 36 looks, of which an eval can deliver only four. A sheet
is a **layout**: its pixels are image px of the picture it composes, not doc px.

The key points:

- Content is a node; raster parameters are look options.
- Pixel ops cap at scale 1, geometric ops pass any scale, decorations draw at the render scale; render windows
  bound memory.
- Crops keep absolute doc-px coordinates through their frame.
- Signed and normalized views are derived values; a 0.5 offset or a diverging colormap makes them visible.
- In the DoG worked example the first blur halves the speck and cuts grain to 28 %, which is why DoG views rank
  specks well.

## 4. Current-state analysis

### 4.1 Looks cannot magnify or show a window

`look.New` (`pkg/cv/look/look.go:66-99`) accepts `MaxSide` between 16 and the engine's `LookLong` (1568) and
derives the scale with `ScaleForLong`, which is never above 1 (`pkg/cv/graph/engine.go:160-171`). The evaluator
rejects any scale outside (0, 1] (`pkg/cv/graph/eval.go:176-178`). There is no crop op; `cv.help("crop")` answers
with a redirect that says so (`pkg/gojamod/cv/help.go:51`).

### 4.2 Look options are parsed by hand

`__look` parses `label` and `maxSide` itself (`pkg/gojamod/cv/evaltrack.go:220-266`), with its own unknown-option
error and fix strings. It bypasses the binder, so look options get no synonyms and no did-you-mean from `checkKeys`
(`pkg/cv/graph/args.go:556-585`), and their help is a hand-written line in `cvFuncs` (`help.go:34`) and in the
generated reference (`pkg/cvsession/reference.go:70`). `show(node, opts)` (`pkg/cvsession/init.js:41-45`) passes
the same options through.

### 4.3 Encoding: JPEG unless gray

`look.Encode` writes PNG only for `*image.Gray` and charts (`look.go:211-225`). JPEG ringing and 8 × 8 block edges
are 2–4 px features, the size of the specks being judged; in a high-pass view they become fake specks (issue 15).

### 4.4 Overlays draw at evaluation scale

`drawRegions` (`pkg/cv/ops/present.go:21-73`) tints region pixels and draws box outlines and ids into the evaluated
buffer, with line width and digit size derived from the buffer's short side (`present.go:41-43`), and refuses
regions whose doc size differs from the image's (`present.go:25-27`). It rasterizes through
`RegionsValue.Rasterize` (`pkg/cv/graph/regions.go:194-240`), which maps the whole doc rectangle onto the target
buffer. Under a zoom these outlines would be upsampled with the image, and on a crop the rasterization would
squeeze the whole frame into the window.

### 4.5 Two fonts

`kernels/draw.go:9-13` has a 3 × 5 digit font for region ids; `look/font.go:12-23` has a 5 × 7 digit font for the
look number. Neither has letters, so no caption, coordinate label or annotation text can be drawn into an image.

### 4.6 Differences are unsigned

`compare({ mode: "diff" })` (`pkg/cv/ops/present.go:151-160`) calls `kernels.AbsDiff` (`kernels/draw.go:100-121`),
which returns |P(a) − P(b)| · gain in perceptual space, clamped. The sign is lost, and with gain the view draws
contour rings inside bright objects (issue 4). `blend` offers `difference`, `add`, `lighten` and `darken`
(`kernels/composite.go:10-27`) but no subtraction. The colleague builds "brighter than the surroundings" as

```js
const pos = (src, r) => src.blend(src.blend(src.gaussian(r), { mode: "darken" }), { mode: "difference" })   // = max(src − blur, 0)
```

(`scripts/84-cv-helpers.js`, line 21), which is correct and unreadable, and cannot express the dark or the signed
residual. `compare({ mode: "side" })` also produces an image whose width is the sum of its inputs plus gaps
(`present.go:128-142`): its pixels are not doc px, but nothing marks that.

### 4.7 Stretching takes bookkeeping

`levels({ black, white })` (`pkg/cv/ops/tone.go:29-57`) needs numbers, so an agent awaits `stats()` first or lifts
`s.p01`/`s.p99`. The colleague's DoG stretch "white = 5 × p99" awaits stats in a helper (`85-cv-views.py`, the
`stretch` function in `PRE`). `levels` has an `Identity` short-circuit for black 0, white 1, gamma 1
(`tone.go:39-41`), which matters for any new option that leaves those at their defaults.

### 4.8 Captions do not say what the image is

The caption is the label and the size (`look.go:151-162`); the default label is the last op's name
(`look.go:79-82`). A model looking at a DoG crop sees "look 2: levels (64×64, …)".

### 4.9 Module functions are written by hand

`cv.compare` is written in the prelude (`pkg/gojamod/cv/prelude.js:334-342`): it checks the list and calls the
`compare` op on `list[0]`. Its help, and that of `look`, `show`, `evaluate`, `describe`, `docs` and the other
module functions, is the hand-maintained `cvFuncs` table (`help.go:24-43`). `cv` is guarded in sessions
(`init.js:23-39`): an unknown member such as `cv.view` gets `UnknownMember` (`help.go:279-312`), which finds Image
ops of that name but does not consult the op synonym table.

## 5. Gap analysis

| need (issue) | today | gap |
|---|---|---|
| Magnify a window (1) | whole image ≤ 1568 px | crop, render scale above 1, windows, nearest upsampling |
| Read coordinates (2) | none | rulers and grid with absolute doc-px labels |
| Many crops at once (3) | ≤ 4 looks | contact sheet layout op |
| Signed residuals (4) | unsigned diff, blend tricks | `sub` and friends, derived results |
| High-pass views (5) | 4 blends + stats | `highpass`, `dog` macros |
| Per-crop stretch, local contrast (8) | two evals | `levels({ auto })`, `localContrast` |
| Draw marks (14) | regions and masks only, at evaluation scale | `annotate`, overlay as decorations |
| Lossless looks (15) | JPEG colour | `format`, PNG for zoomed, small and sheet looks |
| View menu (22) | none | `img.view(name)` presets, docs page |
| Provenance (23a) | label only | op-chain captions with macro names |

## 6. Legacy to remove or refactor

1. `kernels.AbsDiff` and `compare`'s `diff` mode are removed; the signed replacement is `a.sub(b)`, and the enum
   redirect gives the literal fix `a.sub(b).mul(4).add(0.5)`.
2. `blend`'s `add`, `difference`, `lighten` and `darken` modes are removed because they duplicate `add`,
   `sub(…).abs()`, `max` and `min`; `blend` keeps `normal`, `multiply` and `screen`, and both use one
   `kernels.Binary(a, b, f)` kernel.
3. The two fonts are merged into `pkg/cv/text`, one 5 × 7 bitmap font with digits, letters and caption punctuation.
4. The hand-parsed look options (`lookOptionKeys`, `evaltrack.go:220-266`) are replaced by `[]graph.ArgSpec` bound
   with `graph.BindOptions`, the binder entry point that op options use.
5. The hand-written `cv.compare`, and the `cvFuncs` entries of everything that becomes an op or a registered
   function, are removed; the prelude builds `cv.*` and node methods from the registry (§7.10).
6. `img.compare(…)` (an Image method today, `present.go:114`) is removed; `cv.compare(images, { mode })` is the only
   form.
7. `overlay` stops drawing at evaluation scale and becomes a decoration with image-px line widths.
8. `look.Encode`'s gray-or-JPEG rule is replaced by the `format` option with an `auto` rule.
9. The `crop` help redirect (`help.go:51`) is removed, because `crop` is an op.
10. The hand-written look line in the reference (`reference.go:70`) is generated from the look's ArgSpecs.
11. Two ad-hoc "this spelling is gone" tables are avoided by one mechanism: `ArgSpec.Redirects map[string]string`
    (option or enum value → literal fix), used for `look({ box })`, the removed blend modes and `compare`'s `diff`,
    and by GCV-003 for removed region fields.

Outside goja-cv, the colleague's `80-vlm-render.py --zoom` and the `pos`/`stretch` helpers in `84-cv-helpers.js` and
`85-cv-views.py` become unnecessary (§11).

## 7. Proposed architecture

### 7.1 The acceptance example

The colleague's one-eval acceptance test for issue 1 (a 64 px window of a plain, a high-pass and an overlay view at
8×) reads:

```js
const box = [300, 100, 64, 64]
const spots = img.view("speck").mask.luminance({ above: 0.5 }).components({ minArea: 4 })
return {
  plain: img.crop(box).levels({ auto: true }).look({ zoom: 8, ruler: 8 }),
  speck: img.view("speck").crop(box).look({ zoom: 8, ruler: 8 }),
  marked: img.overlay(spots, { labels: "id" }).crop(box).look({ zoom: 8 }),
}
```

### 7.2 Rendering: `Engine.Render`, classes and windows

`Evaluate` keeps accepting scales in (0, 1]; awaiting and `cv.evaluate` never render above 1. Looks call a new entry
point:

```go
// pkg/cv/graph/render.go
type Interp uint8

const (
    InterpNearest Interp = iota
    InterpLinear
)

type RenderOptions struct {
    Scale  float64 // image px per doc px, in (0, MaxRenderScale]
    Interp Interp  // upsampling above scale 1
    Window *Frame  // doc-px rectangle to render; nil = the node's frame
}

const MaxRenderScale = 16

// Render computes node id for a look or a sheet cell. At scale ≤ 1 it is
// Evaluate (cut to the window). Above 1 it dispatches on the op class.
func (e *Engine) Render(ctx context.Context, id NodeID, o RenderOptions) (Value, error)

// OpDef gains:
//   Class OpClass // PixelOp (default), GeometricOp, Decoration
```

Inside a render at scale s > 1 with window W (doc px), the evaluator dispatches on the node:

```
renderEval(node, s, W):
  switch:
  case node.Kind is Regions or Value:
      return eval(node at 1, or at its AnalysisScale)        // data does not upsample; no interp in its signature
  case node.Op.Class == Decoration:
      Image/Mask inputs via renderEval(in, s, W); other inputs as above
      return Run at s with ec.Window = W                     // draws only inside W
  case node.Op.Class == GeometricOp:                         // crop
      W' := W ∩ node's frame
      return renderEval(input, s, W')
  case node.Op.Expand != nil:                                // GCV-001 macro node: its last input
      return renderEval(lastInput, s, W)
  default:                                                   // pixel op or source
      v := eval(node at 1)                                   // full frame, cached as usual
      return memo(upsample(cut(v, W), s, interp))            // keyed by (node, s, interp, W)
```

`buffer.Upsample(b, s, interp)` handles integer and fractional factors; `nearest` maps image pixel (i, j) to source
pixel (⌊i / s⌋, ⌊j / s⌋) relative to the window origin, so a zoom-8 look of a 64 px crop is exactly 512 × 512 with
8 × 8 blocks.

A Phase 2 test registers a test-only decoration op (it draws a 1 px frame) so that the render path is tested before
`overlay` and `annotate` become decorations in Phase 6.

**DECORATION_FIRST.** When the builder interns a pixel op whose Image input chain contains a decoration, it warns:
"levels after overlay sees the drawn outlines; put overlay last. fix: img.levels(…).overlay(r)". Geometric ops after a
decoration are fine.

### 7.3 Crop

```go
{
    Name: "crop", On: graph.KindImage, Returns: graph.KindImage, Class: graph.GeometricOp,
    Args: []graph.ArgSpec{{Name: "box", Type: "box", Unit: graph.DocPx, Positional: true,
        Doc: "[x, y, w, h] in doc px (absolute); clipped to the image's frame"}},
    // Space: SpaceRule{} (plane copy, no conversion); Result: KeepEnc
    DocFrame: func(a graph.Args, inputs []*graph.Node) graph.Frame {
        return clipToFrame(a.Box("box"), inputs[0].Frame) // integer box: floor of x, y; ceil of x + w, y + h
    },
    Run: runCrop,
}
```

- `box` is GCV-001's `box` argument type, so `crop(regionItem)` works once GCV-003 makes items flat. `Args.Box(name)
  [4]float64` is a new accessor returning the canonical box in doc px.
- The frame is the requested box rounded outward to integers and clipped to the input's frame; it is static and part
  of the hash. At an evaluation scale below 1 (a preview of a crop), the cut is aligned to buffer pixels and the
  value records its buffer origin, so rulers stay exact without making the frame scale-dependent.
- A partly outside box is clipped with a `RANGE` warning; an entirely outside box is `E_ARG_RANGE` with the nearest
  valid box of the same size as the fix.
- **CROP_FIRST.** Ops with a halo (`OpDef.Halo` > 0: gaussian, highpass, dog, localContrast, feather, heal) applied
  after a crop see only the window and its reflected border. The builder warns: "gaussian after crop sees only the
  64 px window; put crop last. fix: img.gaussian(3).crop([…])". This is the first reader of `Halo`.

### 7.4 Signed arithmetic

| call | operand | result encoding |
|---|---|---|
| `a.add(b)` | Image, or number (`Level`) | keeps `a`'s |
| `a.sub(b)` | Image, or number (`Level`) | derived (signed) |
| `a.mul(b)` | Image, or number (unitless factor) | keeps `a`'s; `mul(-1)` negates |
| `a.min(b)`, `a.max(b)` | Image, or number (`Level`) | keeps `a`'s; `d.max(0)` keeps the positive part |
| `a.abs()` | — | keeps `a`'s |

All are pixel ops with `Space: {Default: encoded}`: arithmetic works on the values a look shows unless the call
passes `{ space }`. The operand is declared with a new union type:

```go
{Name: "b", Type: "Image|number", Unit: graph.Level, Positional: true}
```

The binder lifts an Image node as an image input (converted to the op's space like any image input), and treats a
number, Value or Param as a level (with GCV-001's `SPACE` check for lifted Values). Image operands must have the same
frame (`E_FRAME`); a gray operand is broadcast to three channels, as `matchChannels` does today
(`kernels/composite.go:29-36`). The names match the existing `Value` arithmetic (`pkg/cv/ops/value.go:125-147`),
which stays: ops are keyed by receiver kind (`registry.go:132-133`), so `stats().p99.mul(5)` and `img.mul(5)` are
different ops with one vocabulary.

There is no `offset` option; adding 0.5 is `.add(0.5)`:

```js
const g = img.toGray("average")
const d = g.sub(g.gaussian(3, { space: "encoded" }))
return {
  grey: d.mul(4).add(0.5).look("signed, grey"),             // zero is mid-grey
  colour: d.colormap("diverging").look("signed, diverging"), // zero is white; colormap needs one channel
}
```

`blend` keeps compositing with opacity and mask. A removed mode with opacity stays expressible:
`blend({ mode: "lighten", opacity: 0.5, mask })` is `a.blend(a.max(b, { space: "linear" }), { opacity: 0.5, mask })`;
the `{ space: "linear" }` matches `blend`'s linear default.

### 7.5 High-pass, DoG and local contrast

`highpass` and `dog` are macros (GCV-001 §7.8) with `Space: {Default: encoded}`. GCV-001's builder resolves the space
before calling `Expand`, so `args.String("space")` is always set:

```go
// highpass(r, { polarity, space }): d = a − G_r(a) in space, then polarity
func expandHighpass(x *graph.Expander, a *graph.Node, args graph.Args) ([]*graph.Node, error) {
    sp := args.String("space") // resolved: "encoded" unless the call said otherwise
    g, err := x.Call("gaussian", a, args.Raw("r"), map[string]any{"space": sp})
    if err != nil {
        return nil, err
    }
    d, err := x.Call("sub", a, g, map[string]any{"space": sp})
    if err != nil {
        return nil, err
    }
    return polarity(x, d, args.String("polarity"))
}
```

| call | expansion | defaults |
|---|---|---|
| `img.highpass(r, { polarity, space })` | `a − G_r(a)`, then polarity | polarity `"both"`, space encoded |
| `img.dog(r1, r2, { polarity, space })` | `G_r1(a) − G_r2(a)`, then polarity; `r2 > r1` | polarity `"both"`, space encoded |
| `img.localContrast(r, { space })` | primitive kernel: (a − G_r a) / (s_r + 1e-3) | space encoded; derived |

`r`, `r1`, `r2` are σ in doc px, as for `gaussian`. Polarity `"bright"` keeps `d.max(0)`; `"dark"` keeps
`d.mul(-1).max(0)`. `dog` with `r2 ≤ r1` is `E_ARG_RANGE` with the swapped call as the fix. Because the expansion's
nodes are hash-consed, the macro node of `img.highpass(3)` has as its last input exactly the node of the hand-written
chain `img.sub(img.gaussian(3, { space: "encoded" }))`; tests assert this identity. `localContrast` is a kernel
because it needs a square and a square root, which are not worth public ops.

### 7.6 Auto stretch

```js
img.levels({ auto: true })              // black, white = p01, p99 of the input, in levels' space
img.levels({ auto: [0.005, 0.995] })    // explicit percentiles
```

Three details make this work with the current code:

- `black` and `white` become `Optional` (no `Default`); `Run` applies 0 and 1 when they are absent. So the binder
  records whether they were passed, and `Validate` can refuse `auto` together with `black` or `white` (`E_ARG_TYPE`
  with both fixes).
- `Identity` returns false when `auto` is set; otherwise `levels({ auto: true })` would be short-circuited as "black
  0, white 1, gamma 1" (`tone.go:39-41`, `eval.go:291-293`).
- The kernel computes the percentiles of its input at the evaluation scale (`kernels.Percentile`,
  `kernels/measure.go:115-135`), then remaps. After a crop it stretches by the crop's pixels: the per-box stretch of
  issue 8.

The colleague's "white = 5 × p99" stays a lifted expression: `d.levels({ black: 0, white: d.stats().p99.mul(5) })`.

### 7.7 Looks

```js
node.look({ label, zoom, interp, maxSide, ruler, grid, format })
node.look("label")    // shorthand for { label }; show(node, opts) takes the same options
```

| option | type | default | meaning |
|---|---|---|---|
| `label` | string | the chain's last call | caption text |
| `zoom` | number in [1, 16] | fit to `maxSide` | render scale; exclusive with `maxSide` |
| `interp` | `"nearest" \| "linear"` | `nearest` | upsampling above scale 1 |
| `maxSide` | integer in [16, 1568] | 1568 | long side when fitting (scale ≤ 1) |
| `ruler` | number (doc px) or `true` | none | tick spacing; `true` picks 1, 2, 4, 8, 16, 32 … so ticks are ≥ 40 image px apart |
| `grid` | number (doc px) | none | faint lines every `grid` doc px |
| `format` | `"auto" \| "png" \| "jpeg"` | `auto` | `auto`: PNG for gray, zoomed (zoom > 1), sheets and looks of at most 512 × 512 image px; JPEG q85 otherwise |

`look` and `show` are registered functions (§7.10) with these ArgSpecs, bound through `graph.BindOptions`, so
`{ magnify: 8 }` gets "Did you mean zoom?" through option synonyms (`magnify`, `scale`, `upscale` → `zoom`; `ticks`,
`axes`, `coords` → `ruler`; `lossless` → `format`), and `look({ box })`, `look({ crop })` and `look({ stretch })` get
`E_UNKNOWN_OPTION` with literal fixes from `Redirects`: `node.crop([x, y, w, h]).look({ zoom: 8 })` and
`node.levels({ auto: true }).look()`. On a histogram chart (a Value look), `zoom`, `ruler` and `grid` are
`E_ARG_TYPE` ("charts have no doc px"), and `format` is always PNG.

**Size.** The 1568 px limit applies to the image before ruler margins; margins add at most 56 px per axis. A zoomed
look whose image exceeds 1568 px is `E_ARG_RANGE` with two fixes: the same zoom with a smaller crop, or the same crop
with a lower zoom.

**Rulers and grid.** The renderer draws rulers into margins of 28 px on the top and left, and on the bottom and right
when the image is at least 256 px on that side. Tick labels are absolute doc px from the frame. A tick at doc x = 312
on a crop with origin x 300 at zoom 8 sits at image x = margin + (312 − 300) · 8, the left edge of doc pixel 312
(Foundations §F5: pixel i covers [i, i + 1)). Labels are thinned so that adjacent labels are at least their width + 4
px apart: with `ruler: 1` at zoom 8 (ticks 8 px apart, labels about 24 px wide), every fourth tick is labelled. The
grid draws 1 px lines at 35 % opacity, white over dark and black over light.

**Captions.** The caption states what the image is:

```
look 1: speck 8× — open(1d955…tif) → view(speck) → crop[300,100,64,64] (512×512 + ruler, 1 image px = 0.125 doc px, doc x 300–364, y 100–164)
```

The chain walks the receiver path (`Evaluator.Path`, `eval.go:348-363`, following `Inputs[0]`) with a compact
describer (op name and its non-default positional arguments). A macro node (GCV-001 §7.8) has the receiver as its
first input, so the walker prints `view(speck)` and continues at the receiver, never entering the expansion.
`explain()` shows the expansion indented under the macro node. Chains longer than 120 characters keep the source and
the last three steps with "…" between.

**Rendering pipeline.**

```
Render(look):
  s := zoom or ScaleForLong(node, maxSide)
  v := Engine.Render(convert(node → encoded) per GCV-001 §7.7, {Scale: s, Interp: interp})
  img := Quantize(v.Buf, 8)
  if ruler or grid: img = addMarginsAndTicks(img, v.Frame, v.BufferOrigin, s, ruler, grid)   // pkg/cv/look/ruler.go
  text.BurnNumber(img, n)
  encode by format rule
```

### 7.8 Decorations: overlay and annotate

`overlay` (`pkg/cv/ops/present.go:88-112`, `pkg/cv/ops/mask.go:387-400`) becomes a decoration. It receives the image
at the render scale (upsampled within the window) and the regions or mask, and draws only inside the window:

- Region pixels are rasterized at the render scale through a changed `RegionsValue.Rasterize(ctx, target Frame,
  scale)`, which maps a target frame (origin and size) rather than the whole doc rectangle. The tint uses the
  rasterized coverage.
- Box outlines are 1 image px (2 px when the look is at least 1024 px); ids are drawn with `pkg/cv/text` at font
  size 2.
- Decorations accept an overlay input whose frame *contains* the image's frame (regions found on the whole image,
  drawn on a crop); masks are cut to the image's frame. Two-image pixel ops still require equal frames (GCV-001
  §7.10).

`annotate` draws marks given as data:

```js
img.annotate([
  { circle: [1337, 131, 4], color: "lime", label: "7" },
  { point: [1683, 123] },
  { box: [1120, 455, 18, 18], color: "#ff8800" },
  { polyline: [[1475, 395], [1490, 410], [1508, 428]], color: "cyan" },
  { text: "hair?", at: [1500, 380] },
], { width: 1 })
```

- Coordinates are absolute doc px; marks outside the window are skipped.
- `width` is in image px (default 1), so a circle stays thin at zoom 8.
- `color` takes every colour form `overlay` accepts; colours are display (encoded) values.
- A mark with no known shape key, or two, is `E_ARG_TYPE` listing the shapes, with did-you-mean (`circ` → `circle`,
  `line` → `polyline`).
- `box` and `point` positions use GCV-001's types, so `{ box: item }` and `{ point: item }` work with GCV-003's flat
  region items; with today's items `{ box: r.box }` works.

### 7.9 Contact sheets

```js
const spots = img.view("speck").mask.luminance({ above: 0.5 }).components({ minArea: 4 })
const top = spots.sortBy("area").top(36)                                  // sortBy sorts largest first
return cv.sheet(top, { views: img.view("speck"), size: 64, zoom: 4 }).look("candidates")
```

`sheet` is a module op (`On: KindNone`, exposed as `cv.sheet`, §7.10). It is a **layout op**, not a decoration:

| option | type | default | meaning |
|---|---|---|---|
| items (positional) | Regions, or a list of `point`s or `box`es | — | one cell (or group of cells) per item |
| `views` | Image node or list of Image nodes | — (required) | what each cell shows; several views give one group per item |
| `size` | doc px | 64 | side of the square window centred on each item |
| `zoom` | 1–16 | 4 | render scale of each cell |
| `max` | integer, static | as many as fit | number of cells laid out |
| `labels` | `"id" \| "index" \| "none"` | `id` for Regions, `index` otherwise | label of each cell, with its centre in doc px |
| `stretch` | `"none" \| "cell"` | `none` | `cell` applies `levels({ auto: true })` to each cell's window |

**Layout arithmetic.** A cell is `size · zoom` image px square; at the defaults 64 · 4 = 256. Cells are separated by
a 6 px gap. Labels are drawn inside the cell, in a dark box at its top-left corner (the item's centre stays visible),
with font size 2: `7 (1337, 131)`. With one view, a row holds ⌊(1568 + 6) / (256 + 6)⌋ = 6 cells (6 · 256 + 5 · 6 =
1566 px), and 6 rows fit: **36 cells at the defaults**, issue 3's acceptance. With several views, a group is the
item's cells side by side with a 2 px inner gap (two views: 2 · 256 + 2 = 514 px); groups are laid out in a grid: 3
groups per row and 6 rows, so **18 items with two views** at the defaults.

**Size at build time.** A node's frame is fixed when it is interned (GCV-001 §7.5), but a Regions input's count is
known only after evaluation. So `max` is static: by default the number of cells that fit 1568 × 1568 at the given
`size`, `zoom` and number of views; an explicit `max` that does not fit is `E_ARG_RANGE` naming the number that fits.
For literal point or box lists, the count is the list length and must fit. The sheet's frame is the layout for `max`
cells. Unused cells are background; if the Regions input has more items than `max`, the sheet shows the first `max`
in list order and warns at evaluation (`SHEET_TRUNCATED: 52 items, 36 shown. fix: regions.sortBy("contrast").top(36)`,
§7.13).

**Evaluation.** The sheet has `AnalysisScale` 1 (Foundations §F4), so it is always evaluated at scale 1 and `await`
returns the scale-1 sheet. Each cell is produced by `Engine.Render` of its view at the cell's zoom with the cell's
window, the same path a look uses. So views that end in a decoration (an overlay) draw thin outlines in cells too.
Windows that extend past the image are filled with dark grey, and the label says `(edge)`. Region items are centred on
their centroid; points are centres; boxes are centred on the box centre with window side `max(w, h, size)`, so a long
hair box is not cut.

**Picture frames.** A sheet's pixels are image px, not doc px. Its frame is a **picture frame** (`Frame.Picture =
true`, extending GCV-001's `Frame`), as is the frame of `cv.compare` side and split layouts. The binder refuses any
`DocPx` argument, crop or ruler on a node with a picture frame (`E_WRONG_KIND`: "a sheet is a picture of crops; its
pixels are not doc px; use the items' coordinates"). Its look has no `zoom` and is 1:1.

### 7.10 Registering module functions and methods

goja-cv's JavaScript surface has two sorts of call:

- calls that **build a node** are **ops**. Method ops have a receiver kind; module ops have receiver `KindNone` and
  are exposed as `cv.<name>` (`cv.open`, `cv.sheet`, `cv.compare`);
- calls that **do something now and return something else** (a look object, a promise, a content object, help data)
  are **functions**, registered as `FuncDef`s.

```go
// pkg/cv/graph/funcs.go
type FuncDef struct {
    Name    string
    On      []string  // receiver types: node kinds ("Image", "Mask", "Value") or Go handle types ("Data");
                      // empty = a module function (cv.<name>)
    Args    []ArgSpec // options, bound with BindOptions (synonyms, did-you-mean, redirects)
    Doc     string
    See     []string
    Bespoke bool      // implemented by hand in the binding/prelude (evaluate, docs)
}

func (r *Registry) RegisterFunc(f FuncDef) error
```

This ticket registers `look` (on Image, Mask and Value) and `show`. GCV-003 registers `json` and `csv` (module) and
`save` (on Image and Mask, and on the `Data` handle that `json`/`csv` return). The remaining `cvFuncs` entries move as
follows: `evaluate`, `describe`, `docs`, `help`, `isNode`, `warnings` become module `FuncDef`s (bespoke where their
binding is special); `version` and `kinds` become generated properties; `mem`, `img`, `doc`, `P`, `slider`, `toggle`,
`choice` stay documented session globals in a small table in `pkg/cvsession`. The prelude builds `cv.*` and node
methods from the registry's op and function lists. Ops marked `Internal` are skipped; ops and functions that need
hand-written bindings (`open`, `param`, the builder placeholder, bespoke functions) are excluded by flag, not by a list
of names. `cv.compare` becomes the module op `{ Name: "compare", On: KindNone, Args: images (Image[], positional),
mode (side | split) }`.

### 7.11 View presets

```js
img.view("speck", { size: 3 })
```

`view` is a macro on Image with an enum name and `Space: {Default: encoded}`. Each preset is a Go function in
`pkg/cv/ops/views.go` plus a one-sentence description used by help and by the docs page. `size` (doc px, default 3) is
the typical size of the target; presets derive radii from it.

| name | expansion | shows | use it for |
|---|---|---|---|
| `scene` | `levels({ auto: true })` | the image, stretched 1–99 % | judging what belongs to the scene |
| `speck` | `toGray("average").dog(s/3, 4s/3, { polarity: "bright" })`, `levels({ black: 0, white: p99 × 5 })` | small bright things that differ from their surroundings | dust on density tiles, stars, sensor defects |
| `speckDark` | same with `polarity: "dark"` | small dark things | dust on positives and prints |
| `speckColor` | `dog(…)` on all channels, same stretch | the same in colour: neutral specks stay white | neutral defects versus colour grain |
| `signed` | `toGray("average").highpass(s)`, `colormap("diverging")` | brighter (red) and darker (blue) than surroundings | edges, scratches, both polarities |
| `detail` | `toGray("average").localContrast(4s)`, `colormap("diverging", { range: [-4, 4] })` | detail equally visible in shadows and highlights | sweeping a whole tile |
| `neutral` | `chroma({ relative: true })`, `colormap("viridis", { range: [0, 1] })` | where channels disagree | grey dust versus coloured detail |

`signed` and `detail` start with `toGray("average")` because `colormap` takes one channel (GCV-001 §7.9). The
colleague's view is `pos(img.toGray("average").gaussian(1), 4)` (`84-cv-helpers.js:24`), that is G₁(a) − G₄(G₁(a)),
whose outer σ is √17 ≈ 4.12; `speck` uses σ 4, a slight difference that the gated ranking test (§13.4) validates. An
unknown preset name gets did-you-mean over the names and the list with descriptions.

### 7.12 Text: `pkg/cv/text`

```go
package text

// The built-in 5×7 bitmap font: digits, A–Z, a–z, and [ ] ( ) , . : - + × → % ? / _.
func Width(s string, px int) int
func Draw(dst draw.Image, x, y int, s string, px int, fg, shadow color.Color)
func BurnNumber(dst draw.Image, n int)
```

`px` is the size of one font pixel in image px. The glyph table is taken from the X11 `misc-fixed` 5×7 font
(`5x7.bdf`), which is in the public domain; the implementer converts the needed glyphs into a Go table once, with a
test that renders each glyph inside its box. `Draw` works on quantized `image.Image`s; overlay and annotate, which
draw into float buffers, use a small adapter that writes encoded values.

### 7.13 Warnings at evaluation time

Today warnings are produced only at build time (Foundations §F10). Some facts are known only when an op runs:
`SHEET_TRUNCATED` here, and GCV-003's points outside a frame and clamped values when saving. The evaluator gains:

```go
func (ec *EvalCtx) Warn(code, message, fix string)
```

Warnings raised during `Run` are stored with the cached value and re-emitted when the value is served from the cache,
so every `await` or look that uses the value reports them. The session merges them into the `warnings:` block with
the line of the call that started the evaluation.

The key points:

- `Engine.Render` is the only path above scale 1; the three classes and the window decide what is computed where.
- Crop is geometric, so decorations work before or after it; pixel ops after decorations are warned about.
- Captions name macros through macro nodes; nothing on a shared node is mutated.
- A sheet's size is fixed at build time by a static `max`; 36 cells fit at the defaults.
- Ops build nodes, functions do things now; both come from the registry.

## 8. Decision records

### Decision: views are nodes, render parameters are look options

- **Context:** Issue 1 proposes both `look({ box, zoom })` and a lazy `crop(box)`; issues 2, 8 and 15 add more look
  options.
- **Options considered:** (a) everything as look options; (b) everything as nodes, including zoom and rulers; (c)
  nodes for content, look options for the raster.
- **Decision:** (c).
- **Rationale:** Under (a) a crop could not be measured, put in a sheet or explained. Under (b) zoom and rulers would
  produce pixels that are not doc px. (c) gives content the graph's caching, hashing and explanation.
- **Consequences:** `look({ box })` is an error with a literal fix.
- **Status:** accepted

### Decision: three op classes and render windows

- **Context:** Zoomed looks need thin lines and readable labels, must not compute filters on invented pixels, and must
  work when an annotation precedes a crop.
- **Options considered:** (a) evaluate everything at scale ≤ 1 and upsample the result; (b) evaluate everything at the
  render scale; (c) a decoration flag only; (d) pixel, geometric and decoration classes with render windows.
- **Decision:** (d).
- **Rationale:** (a) gives 8 px outlines; (b) changes filter values with the zoom; (c) breaks for decorations before a
  crop (they would be computed at 1 and upsampled). (d) handles both orders, and windows keep buffers at look size.
- **Consequences:** A render-only entry point (`Engine.Render`), a class field on `OpDef`, and a memo keyed by window.
- **Status:** accepted

### Decision: signed arithmetic as Image ops with the Value names; no offset option

- **Context:** Issue 4 proposes `sub`, `add`, `mul`, `abs`, `max`, `min` and an `offset` option.
- **Options considered:** (a) a single expression op; (b) named ops plus `offset`; (c) named ops without `offset`.
- **Decision:** (c).
- **Rationale:** Named ops are discoverable and typed. An `offset` option is exactly `.add(0.5)`, a second spelling;
  `colormap("diverging")` covers the display case.
- **Consequences:** The blend modes that duplicated arithmetic, and `compare`'s diff, are removed with redirect fixes.
- **Status:** accepted

### Decision: high-pass and DoG are macros; local contrast is a kernel

- **Context:** The views are compositions of existing ops; local contrast needs a square root.
- **Options considered:** (a) dedicated kernels for all three; (b) macros plus public `sqrt`/`square` ops; (c) macros
  for highpass/dog, a kernel for localContrast.
- **Decision:** (c).
- **Rationale:** Macros cost no kernel code and share caches with hand-written chains; public `sqrt` and `square`
  would grow the surface for one view.
- **Consequences:** Tests assert node identity between a macro's expansion and the hand-written chain.
- **Status:** accepted

### Decision: captions show macro names; `explain()` shows expansions

- **Context:** Issue 22 says "the look caption shows the expanded op chain"; GCV-001 revision 1 said captions show the
  primitive chain. Expanded chains of presets are 8–10 steps long.
- **Options considered:** (a) captions show the expansion; (b) captions show the macro call and `explain()` the
  expansion.
- **Decision:** (b), which deviates from issue 22's wording.
- **Rationale:** A caption is read by a vision model next to an image and must stay short; `view(speck)` names the
  view the docs describe. Provenance is not lost: `explain()` and the docs page show the expansion.
- **Consequences:** Macro nodes (GCV-001 §7.8) carry the label without mutating shared nodes.
- **Status:** accepted

### Decision: presets are `img.view(name)`, not `cv.view(name)`

- **Context:** Issue 22 proposes `cv.view("speck", { size })`, implicitly on `img`.
- **Options considered:** (a) `cv.view(name, opts)` on the bound `img`; (b) `cv.view(name, image, opts)`; (c)
  `image.view(name, opts)`.
- **Decision:** (c).
- **Rationale:** With GCV-001's named images there is no single implicit image; (c) names the image like every op and
  composes (`ref.view("speck")`). `cv.view` gets `UnknownMember`'s "is an Image method" fix.
- **Consequences:** The docs page lists presets under "Views".
- **Status:** accepted

### Decision: per-box stretch is crop then `levels({ auto })`; no stretch options

- **Context:** Issue 8 asked for `levels({ auto, box })` and `look({ stretch: "auto" })`; issue 5 asked for a stretch
  option on `highpass` and `dog`.
- **Options considered:** (a) the three options as asked; (b) one mechanism: `levels({ auto })` placed where the
  stretch should apply, and presets that include their stretch.
- **Decision:** (b).
- **Rationale:** A `box` on levels duplicates crop; a look-level stretch is content in a look option (§3.1); a stretch
  option on every view op repeats `levels`. One op placed in the chain covers all three.
- **Consequences:** Each asked-for spelling is an `E_UNKNOWN_OPTION` with a literal fix (§10).
- **Status:** accepted

### Decision: contact sheet is a layout op with a static size, exposed as `cv.sheet`

- **Context:** Issue 3 asks for one image of many crops; the generic list (DUST-002 `reference/06`, item A4) asks for
  the same crop from several views.
- **Options considered:** (a) a look-level function; (b) an Image method `view.sheet(items)`; (c) a module op
  `cv.sheet(items, { views, max })` with a picture frame.
- **Decision:** (c).
- **Rationale:** (c) supports several views per item, is a node that can be shown and cached, and a static `max` makes
  its size known at build time.
- **Consequences:** Picture frames, refused by coordinate ops; `SHEET_TRUNCATED` at evaluation.
- **Status:** accepted

### Decision: ops build nodes, functions do things now

- **Context:** Module functions were hand-written (`cv.compare`, `cvFuncs`); GCV-003 needs functions that return
  content objects and promises (`json`, `csv`, `save`).
- **Options considered:** (a) everything as `KindNone` ops; (b) everything as a function table; (c) ops for calls that
  build nodes, `FuncDef`s for calls that return anything else, both in the registry.
- **Decision:** (c).
- **Rationale:** An op must return a node of a kind (the registry requires `Returns`). Functions such as `look` and
  `save` return Go objects or promises. One rule by return type is easy to apply, and both tables give help, binding
  and did-you-mean from one place.
- **Consequences:** `cvFuncs` shrinks to session globals; `look`, `show`, `json`, `csv`, `save` are `FuncDef`s.
- **Status:** accepted

### Decision: lossless by default where it matters

- **Context:** Colour looks are JPEG; ringing is the size of specks (issue 15).
- **Options considered:** (a) always PNG; (b) a `format` option only; (c) `format` with an `auto` rule.
- **Decision:** (c).
- **Rationale:** Zoomed crops, sheets and small looks are where artefacts mislead, and they are small. Whole-tile colour
  looks stay JPEG to respect the 8 MiB budget of four looks.
- **Consequences:** Look tests that assert MIME types change for small colour looks.
- **Status:** accepted

## 9. Pseudocode and key flows

### 9.1 A zoomed crop with a ruler, end to end

```
agent: img.view("speck").crop([300,100,64,64]).look({ zoom: 8, ruler: 8, label: "speck 8×" })

build (ids for a fresh session where #1 is the source):
  view(speck): expansion #2 toGray(average), #3 gaussian(1), #4 gaussian(4), #5 sub, #6 max(0),
               #7 stats, #8 .p99, #9 mul(5), #10 levels(black 0, white #9); macro node #11 view(speck) [#1, #10]
  crop:        #12 crop [300,100,64,64], frame {300, 100, 64, 64}, class geometric
  look:        Look{node #12, zoom 8, ruler 8, format auto → png}

render (after the eval, off the owner goroutine):
  s = 8, W = {300, 100, 64, 64}
  convert(#12 → encoded) is the identity (derived, linear encoding)
  #12 crop (geometric): renderEval(#11, 8, W)
     #11 macro node → its last input #10, a pixel op: eval #10 at 1 (whole tile, cached),
        cut W (64×64), upsample ×8 nearest (512×512)
  quantize; rulers: margins 28 px, ticks every 8 doc px labelled 300, 308, …, 364 / 100, …, 164
  burn number; PNG
caption: look 1: speck 8× — open(t.tif) → view(speck) → crop[300,100,64,64] (512×512 + ruler, 1 image px = 0.125 doc px, doc x 300–364, y 100–164)
```

### 9.2 Sheet composition

```
sheet at intern time:
  cells := opts.max or fitCount(size, zoom, len(views))      // E_ARG_RANGE when an explicit max does not fit
  frame := layout(cells, size·zoom, len(views))              // picture frame, static

sheet.Run(items, views, opts) at scale 1:
  centres := itemsToCentres(items)                           // Regions: centroid; point; box centre
  if len(centres) > cells: ec.Warn("SHEET_TRUNCATED", …); centres = centres[:cells]
  out := New(frame.W, frame.H, 3) filled with the sheet background
  for each item i, each view v:
      win := window(centre_i, side_i)                        // doc px
      cell := Engine.Render(v, {Scale: zoom, Window: win})   // same path as a look; outside the image → dark grey
      if stretch == cell: cell = levelsAuto(cell)
      paste(out, cell, x, y); text.Draw(out, label_i in the cell's top-left box)
  return NewImage(out)                                       // Enc srgb (display), Space encoded
```

### 9.3 A decoration before a crop

```
renderEval(crop#21, s = 8, W = frame of #21):
  geometric → W' = W ∩ box; renderEval(annotate#20, 8, W')
     decoration → its image input #19 (img): pixel → eval at 1, cut W', upsample ×8
     Run at 8 with ec.Window = W': map doc px → image px with W'.origin and s; draw 1 px outlines and text
```

## 10. Errors and diagnostics

| situation | code | message and fix |
|---|---|---|
| `look({ box })`, `look({ crop })` | `E_UNKNOWN_OPTION` | redirect: `node.crop([x, y, w, h]).look({ zoom: 8 })` |
| `look({ stretch })`, `levels({ box })`, `highpass(r, { stretch })` | `E_UNKNOWN_OPTION` | redirect: `node.crop(box).levels({ auto: true })`, or `img.view("speck")` for a stretched high-pass |
| zoomed look too large | `E_ARG_RANGE` | both fixes with numbers |
| `zoom` with `maxSide` | `E_ARG_TYPE` | "zoom sets the scale; maxSide fits it" |
| `zoom`/`ruler` on a chart | `E_ARG_TYPE` | "charts have no doc px" |
| crop entirely outside the image | `E_ARG_RANGE` | the nearest valid box of the same size |
| crop partly outside | `RANGE` warning | the clipped box |
| filter after crop | `CROP_FIRST` warning | the reordered chain |
| pixel op after a decoration | `DECORATION_FIRST` warning | the reordered chain |
| frames differ in arithmetic | `E_FRAME` | both frames; "crop both the same way, or crop after the arithmetic" |
| `dog(4, 1)` | `E_ARG_RANGE` | `dog(1, 4)` |
| `levels({ auto, white })` | `E_ARG_TYPE` | `levels({ auto: true })` or `levels({ black, white })` |
| unknown view | `E_ARG_TYPE` (enum) | did-you-mean and the preset list with descriptions |
| coordinates, crop or ruler on a sheet or side-by-side compare | `E_WRONG_KIND` | "its pixels are not doc px" |
| explicit sheet `max` too large | `E_ARG_RANGE` | the number that fits |
| more items than sheet cells | `SHEET_TRUNCATED` warning | `regions.sortBy(…).top(n)` |
| unknown annotate shape | `E_ARG_TYPE` | shape list, did-you-mean |
| `blend({ mode: "difference" })` and the other removed modes | `E_ARG_TYPE` (enum) | redirect: `a.sub(b).abs()`; `add` → `a.add(b)`; `lighten` → `a.max(b)`; `darken` → `a.min(b)`; with opacity: `a.blend(a.max(b, { space: "linear" }), { opacity, mask })` |
| `compare({ mode: "diff" })` | `E_ARG_TYPE` (enum) | redirect: `a.sub(b).mul(4).add(0.5)` |
| `cv.view(...)`, `cv.grid(...)` | `E_UNKNOWN_METHOD` | `img.view("speck")`; `cv.sheet(...)` |

`UnknownMember` (`help.go:279-312`) gains a lookup in the op synonym table, so `cv.grid` finds `sheet` and `img.zoom`
finds the `zoom` redirect. Op synonyms gain `subtract`, `difference`, `diff` → `sub` (on Image; `minus` stays the Mask
op); `multiply` → `mul`; `hp`, `highPass`, `residual` → `highpass`; `DoG`, `bandpass` → `dog`; `clahe` →
`localContrast`; `normalize`, `autoLevels`, `stretch` → `levels`; `window`, `roi` → `crop`; `marks`, `markers`, `draw`
→ `annotate`; `contactSheet`, `grid`, `mosaic` → `sheet`; `preset` → `view`. `zoom` is not a synonym of `crop`: an
agent writing `img.zoom(8)` wants a look option, so `zoom` gets a redirect with the fix `node.crop([x, y, w,
h]).look({ zoom: 8 })`. A new test, `TestOpSynonymsDoNotShadowOps`, checks that no synonym is also an op name on the
same receiver kind and that every synonym target exists.

## 11. Documentation and examples that change

- A new topic `pkg/cv/docs/topics/11-views.md` (id `views`) explains high-pass, DoG and local contrast in plain words
  ("shows only what differs from its surroundings at size σ"), the preset table with when to use each, signed maps
  (grey offset or diverging colormap), and the measured note that the speck view puts dust about 40 times earlier
  than a stretched tile. A test asserts that every preset name in `views.go` appears in this topic.
- `06-looks.md` is rewritten for render scale and zoom, rulers, formats, chain captions and sheets.
- `05-masks-and-regions.md` shows overlays at zoom; `09-recipes.md` gains "Inspect candidates" (speck view →
  components → sheet → zoomed crops), "Compare two versions" (`sub` with a diverging colormap) and "Mark points".
- The MCP instructions gain one line each for crop + zoom + ruler, `cv.sheet` and `img.view`; the `compare` line says
  side and split only, with `a.sub(b)` for differences.
- The colleague's playbook (DUST-002 `reference/04`) step 3 can replace the Python zoom with
  `img.view("speck").crop(box).look({ zoom: 8, ruler: 4 })` and a sheet of the hot list, and `84-cv-helpers.js`'s
  `pos()` with `highpass`/`view("speck")`. This is their document; the change is listed for the hand-over note.

Tests and examples that change: `blend` mode lists in docs and `pkg/cv/ops/composite_test.go`; `compare` tests
(`pkg/cv/ops/present_test.go`) lose diff and `img.compare`; overlay goldens (GCV-001 Phase 0 harness); look MIME
assertions in `pkg/cv/look/look_test.go:151-175` (small colour looks become PNG); `cvFuncs` help tests.

## 12. Implementation phases

Each phase ends with `go test ./... -count=1`, `make lint` and a commit. Write the listed tests first.

**Phase 1 — text, functions and look options.**
1. Write tests: every glyph of `pkg/cv/text` renders inside its box and `Width` matches; burned look numbers read back
   (the existing `GlyphPixel` test moves); `look({ magnify: 8 })` suggests `zoom`; `look({ box })` gives the redirect
   fix.
2. Merge the fonts; add `FuncDef`, `RegisterFunc`, `graph.BindOptions`, `ArgSpec.Redirects`; register `look` and
   `show` with their ArgSpecs; delete `lookOptionKeys`.

After Phase 1, looks behave as before and their options get synonyms and redirects.

**Phase 2 — rendering above scale 1.**
1. Write `pkg/cv/graph/render_test.go`: a pixel op rendered at 8 equals the nearest-upsampled scale-1 result in the
   window exactly; Regions and lifted Values inside a render are not upsampled; a test-only decoration receives
   upsampled inputs and draws 1 px lines; memo keys include scale, interp and window; `Evaluate` still refuses scales
   above 1; render scales above 16 are refused.
2. Add `Engine.Render`, `RenderOptions`, `OpDef.Class`, the render dispatch, `buffer.Upsample`, `EvalCtx.Warn` with
   cache replay; `look({ zoom, interp })` and captions below scale 1.

After Phase 2, whole-image looks can be zoomed within the size limit.

**Phase 3 — crop, rulers and captions.**
1. Write tests: crop frames, clipping and the outside-box fix; annotate-before-crop with the test decoration has 1 px
   lines; `CROP_FIRST` and `DECORATION_FIRST`; ruler tick positions and label thinning for a crop at zoom 8 (a golden
   PNG of a 32 × 32 synthetic crop); chain captions for plain, macro and long chains.
2. Implement `crop` (geometric), `Args.Box`, `look/ruler.go`, chain captions.

**Phase 4 — arithmetic, blend and compare.**
1. Write tests: sign preservation and derived encoding; `a.sub(b).add(b)` equals `a` within float error; broadcast
   gray; `E_FRAME`; Value and Image ops with the same names coexist; the `Image|number` operand with a lifted Value and
   its `SPACE` warning; blend and compare redirects.
2. Implement `kernels.Binary`, the six ops, the union argument type, the reduced `blend`, `compare` as a module op
   (side, split) with picture frames; delete `AbsDiff` and `img.compare`; expose module ops in the prelude.

**Phase 5 — views.**
1. Write tests: `img.highpass(3)`'s macro node has the hand-written chain as its last input; `highpass` without
   `space` runs the gaussian and the sub in encoded space; `dog` polarities on a planted speck; `localContrast` of a
   constant image is 0 and of white noise has unit spread; `levels({ auto: true })` maps p01/p99 to 0/1 and is not
   short-circuited; every preset builds and renders on the RGB planted scan.
2. Implement `highpass`, `dog`, `localContrast`, `levels.auto`, `views.go`.

**Phase 6 — decorations and sheets.**
1. Write tests: overlay outlines are 1 image px at zoom 1 and 8; overlay of whole-image regions on a crop at zoom 8
   (golden); annotate shapes land on the right pixels (golden); sheet layout with 36 cells at the defaults and 18 items
   with two views; labels; out-of-image cells; `SHEET_TRUNCATED`; explicit `max` errors; coordinate ops refuse picture
   frames.
2. Make `overlay` a decoration with `Rasterize(target Frame, scale)`; implement `annotate`, `sheet`, the `auto` format
   rule.

**Phase 7 — documentation, acceptance and agent check.**
1. Write the topics, recipes, instructions and synonyms of §10 and §11, the synonym collision test, and the
   `UnknownMember` synonym lookup; the docs tests pass.
2. Write the acceptance tests (§13.6) and the gated ranking test (§13.4); run the pi task (§13.5).

The key points:

- The render path is built and tested with a test-only decoration before any real decoration depends on it.
- Crops and rulers come before views, so every view can be inspected as it is built.
- Sheets come last because they reuse the render path, crops, labels and evaluation warnings.

## 13. Test strategy

### 13.1 Unit and property tests

- `render_test.go` checks upsampling exactness, the three classes, windows and cache keys.
- Macro tests check node identity between expansions and hand-written chains.
- Arithmetic property tests check `a.sub(b).add(b) ≈ a`, `a.max(b) ≥ a` and `≥ b`, and `abs ≥ 0` on random buffers.
- Frame tests check that a crop of a crop composes origins and that arithmetic rejects different frames.

### 13.2 Golden looks

Small PNG goldens (crops of at most 64 × 64 doc px at zoom 4–8) for rulers, grid, annotate shapes, overlay at zoom,
and a 2 × 2 sheet, generated from the planted scan through GCV-001's golden harness. PNG goldens are byte-stable; JPEG
goldens are not used.

### 13.3 Documentation tests

Every ```js block in the `views` and `looks` topics runs against the planted scan (Foundations §F11). A new block
attribute `js looks=N` asserts the number of looks a block returns, so docs cannot silently stop attaching images.

### 13.4 Real tiles: view ranking (gated)

Port the colleague's metric (`scripts/85-cv-views.py`) to a Go test gated on `GOJACV_TILES` (default
`~/dustdet/cv_tiles`) and `GOJACV_GOLD` (default `~/dustds/gold`). For each tile with labels, the test reads
`{tile_id}.final.json` or else `{tile_id}.p1.json` from the gold directory (`points: [{ x, y, r, type }]`, absolute
raw-scan coordinates, `type` ∈ dust, hair, scratch, as loaded by `dustdet/gold.py`'s `annotation()`), converts them to
tile coordinates with `origin_x`, `origin_y` from `{tile_id}.json`, renders the `speck` view as a look, finds local
maxima (5 × 5 plateaus) on the decoded look, and ranks each point's peak within r + 2 px. Assert median rank ≤ 30 and
top-50 ≥ 70 % over the labelled tiles (the colleague measured 18 and 78 %).

### 13.5 Agent harness

A question file `questions-gcv-002.md` for the pi harness (Foundations §F11, with GCV-001's `IMAGE` and `QUESTIONS`
overrides), on one gold tile:

1. Show the tile and a view in which small bright specks stand out. Which view did you use?
2. List the 20 most prominent candidate spots with coordinates, and show them in one picture.
3. Zoom on three of them at 8× with coordinates. For each: dust or not, and why?

Run the build before this ticket and after, twice each. Record calls, errors, looks per eval, and coordinate accuracy
against the gold points (distance to the nearest gold point).

### 13.6 Acceptance per issue

Each test is the agent's JavaScript, run by `pkg/cvsession` against the planted scan:

| issue | acceptance test |
|---|---|
| 1 | §7.1's eval returns three 8× looks (plain, speck view, overlay) of a 64 px window in one eval |
| 2 | ruler labels of a crop at (300, 100) read 300 … 364 and 100 … 164 (decoded from the PNG by matching `pkg/cv/text` glyphs) |
| 3 | `cv.sheet` of 36 regions at the defaults is one look ≤ 1568 × 1568 with 36 labelled cells whose ids match the regions |
| 4 | `a.sub(b)` keeps negative values (`stats().p01 < 0`) and `.add(0.5)` displays zero as grey 128 |
| 5 | `img.highpass(3)` and `img.dog(1, 4)` exist; the views topic explains them; the speck preset passes §13.4 when the data is present |
| 8 | `img.crop(box).levels({ auto: true })` maps the crop's p01/p99 to 0/1 |
| 14 | a circle annotation is 1 image px wide at zoom 1 and at zoom 8, and its label glyphs are present at both |
| 15 | `look({ format: "png" })` returns PNG for a colour look; a zoomed colour look is PNG by default |
| 22 | every preset has a paragraph in the views topic and renders |
| 23a | the caption of §9.1 contains `view(speck) → crop[300,100,64,64]` |

## 14. Risks, alternatives and open questions

**Risks.**

- Full-resolution evaluation for crops of large images: a crop of a whole DNG evaluates its pixel ops at scale 1 over
  the whole frame (638 MB per RGB buffer). Mitigation now: GCV-001's `open({ box })`. Later: windowed evaluation of
  pixel ops, where each op's `Halo` enlarges the window for its inputs; the render window is the first step.
- Agents' recipes using `blend({ mode: "difference" })` or `compare({ mode: "diff" })` break; the redirects give the
  replacement in the error.
- Look budget: PNG crops and sheets are larger than JPEG. A 1566 × 1566 RGB PNG sheet can exceed 2 MB; with four looks
  per eval and 8 MiB, a sheet should be the only large look in an eval. Dropped looks are reported by the transport, as
  today.

**Alternatives not taken.** An expression op (`cv.expr("a - gaussian(a, 3)")`); a separate "canvas" kind for decorated
images; CLAHE instead of local contrast (its tile artefacts are the size of what is inspected).

**Open questions.**

1. Should rulers label in tile-local or absolute coordinates for a file opened without `origin`? The guide labels the
   frame's coordinates; GCV-001's `origin` lets the agent choose.
2. Should `sheet` accept `stretch: "shared"` (one stretch across all cells of a view)? Add it if the colleague's agents
   compare brightness across cells.
3. Is a 16× maximum zoom enough? The colleague uses 8×; 16× allows 32 px windows at 512 image px.

## 15. Closing

GCV-003 can now assume looks that render any view at any zoom with absolute coordinates, a render path that a profile
plot can reuse, a text font for labels and axes, evaluation-time warnings, a registry that registers functions next to
ops, and sheet and annotate ops that accept boxes and points. GCV-003 makes region items flat so they plug into those
ops directly, and adds the measurements an agent uses to back what it sees.

## 16. References

- Foundations: `ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md`.
- Review findings with disposition: `reference/01-review-read-findings.md` in this ticket.
- Handover issues 1–5, 8, 14, 15, 22, 23: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md`; generic list sections A and B: `…/reference/06-goja-cv-generic-inspection-features.md`.
- View measurements `…/DUST-002…/scripts/85-cv-views.py`; zoom renderer `…/scripts/80-vlm-render.py`; helpers `…/scripts/84-cv-helpers.js`; playbook `…/reference/04-goja-cv-dust-playbook.md`; gold loader `/Users/manuel/code/wesen/2026-11-07--dust-scanning/dustdet/gold.py`.
- CVJS-001 intern guide Part 3 §17.3 (looks as returned values) and the integration-surfaces doc §3 (result content): `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/`.
- GCV-001 guide (frames, spaces, derived encoding, colormap, macros) and GCV-003 guide (flat region items, measurements, data out): `design-doc/01-design-and-implementation-guide.md` in each ticket.
- X11 `misc-fixed` fonts (public domain), `5x7.bdf`.
