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
    - Path: pkg/cv/look/look.go
      Note: look options (label, maxSide), scale fit, JPEG-unless-gray encode; gains zoom, interp, ruler, grid, format and op-chain captions
    - Path: pkg/cv/look/font.go
      Note: 5x7 digit font for the look number; merged with kernels/draw.go digits into pkg/cv/text
    - Path: pkg/cv/kernels/draw.go
      Note: 3x5 digit font, DrawRect, Paste, AbsDiff (unsigned diff, removed)
    - Path: pkg/cv/kernels/composite.go
      Note: blend modes; add/difference/lighten/darken become arithmetic ops; one Binary kernel
    - Path: pkg/cv/ops/present.go
      Note: overlay draws at evaluation scale; compare diff is unsigned; both change
    - Path: pkg/cv/ops/composite.go
      Note: blend op
    - Path: pkg/cv/ops/tone.go
      Note: levels gains auto
    - Path: pkg/cv/graph/eval.go
      Note: scale limited to (0, 1]; render scale above 1 and decoration nodes
    - Path: pkg/gojamod/cv/evaltrack.go
      Note: look options parsed by hand (lookOptionKeys); bound through ArgSpecs instead
    - Path: pkg/gojamod/cv/prelude.js
      Note: cv.compare written by hand; KindNone ops exposed as cv.* automatically
    - Path: pkg/gojamod/cv/help.go
      Note: cvFuncs table and the crop redirect ("There is no crop")
    - Path: pkg/cv/docs/topics/06-looks.md
      Note: looks topic; new views topic
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/85-cv-views.py
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/80-vlm-render.py
Summary: "Intern guide for GCV-002: render scale and decorations, crops, zoomed looks with rulers, contact sheets, signed arithmetic, high-pass/DoG/local contrast, auto stretch, annotations and view presets, built as composable nodes on GCV-001's image model."
LastUpdated: 2026-11-09T14:00:00-05:00
WhatFor: "Implementing GCV-002 (after GCV-001)."
WhenToUse: "Read before touching looks, overlays, compare, blend, or adding inspection views."
---

# GCV-002 — Inspection views and looks: crops, zoom, rulers, sheets, arithmetic, high-pass

This guide is for an intern who will implement GCV-002 in goja-cv. It builds on GCV-001's image model:
encodings, working spaces, the `space` option, `convert` nodes, derived results, frames and macros
(`OpDef.Expand`). Read GCV-001 §3 (concepts) and §7.5 (space resolution) first. The vocabulary of the CVJS-001
intern guide (node, op, evaluation, scale, doc px, look) is assumed.

**Dependencies.** GCV-002 needs from GCV-001: `Frame` (crops and rulers place pixels in doc px), `DerivedEnc`
(signed results display and measure as their values), the `space` option and its defaults (views default to
`encoded`), `colormap` (signed maps in false colour), and `Expand` (high-pass, DoG, compare diff and view
presets are macros). GCV-003 uses this ticket's render pipeline for profile plots and its `pkg/cv/text` font.

## 1. Executive summary

A vision model can only judge what it can see. Today every goja-cv look shows the whole image at no more than
1568 px on the long side; nothing can be magnified, coordinates cannot be read off the image, at most four
looks arrive per eval, colour looks are JPEG, and the only difference view is unsigned. The colleague's agents
therefore leave goja-cv for a Python script to get an 8× zoom with coordinate ticks, and build their best view
(a difference of Gaussians) out of four `blend` calls.

This ticket makes inspection native, using one principle: **anything that depends on pixel content or data is
a node; anything that depends only on the output raster is a look option.** Concretely:

- Nodes: `crop(box)`, signed arithmetic (`add`, `sub`, `mul`, `abs`, `min`, `max`), `highpass`, `dog`,
  `localContrast`, `levels({ auto })`, `annotate(marks)`, `cv.sheet(items, { views })`, `img.view(name)`.
- Look options: `label`, `zoom`, `interp`, `maxSide`, `ruler`, `grid`, `format`.

Two engine changes make this work. Evaluation accepts a **render scale** above 1 (zoom): pixel ops still
compute at most at scale 1 and the evaluator upsamples their result, while **decoration** ops (overlay,
annotate, sheet labels) draw at the render resolution so that lines and text stay thin and legible. And look
captions show the **op chain**, so the model knows what it is looking at.

The ticket also removes the unsigned `AbsDiff` kernel, the duplicate blend modes, the two separate digit fonts,
the hand-parsed look options and the hand-written `cv.compare`.

## 2. Problem statement and scope

### 2.1 The workflow

The colleague's annotator agent follows a four-step loop (DUST-002 `reference/06-…`, "The workflow these
features serve"): transform the image so that what matters stands out; decide where to look; crop and magnify
those places; judge them by eye with a few numbers to back the judgement. Steps 1 and 3 are this ticket.

The numbers that justify the views come from the colleague's view comparison (`scripts/85-cv-views.py`, 142
human-labelled specks on 19 tiles). For each view they rendered the look a model would receive, found all local
maxima, and ranked each labelled speck among them:

| view | median rank of a labelled speck | specks among the 50 most visible bumps |
|---|---|---|
| levels-stretched tile (`scene`) | ~715 | 1 % |
| bright difference of Gaussians, σ 1 vs 4, white = 5 × p99 (`dog`) | 18 | 78 % |
| the same on colour (`cdog`) | — | 79 % |

A view is not a cosmetic choice: in the DoG view a speck is the 18th thing the eye meets; in the stretched tile
it is the 715th.

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

Issue 23b (CLI `--prelude`) is GCV-003. Rank and morphological filters (issue 18) and fibre detection (issue
19) are out of scope; the high-pass views cover most of their inspection value, and they can be added later as
primitive ops.

## 3. Concepts

### 3.1 What a look is today

A **look** is an image an eval returns to the agent. `node.look("label")` creates a Go object implementing
go-go-goja's `resultcontent.Provider` (`pkg/cv/look/look.go:46-61`); when the eval's returned value contains
it, the session renders it after the eval and attaches the bytes as image content. Rendering evaluates the node
at the **look scale** (buffer px per doc px), chosen so the long side fits 1568 px (`look.go:71-86`), burns the
look number into the top-left corner, and encodes JPEG q85 for colour or PNG for single-channel images
(`look.go:211-225`). The caption carries the scale: `look 3: specks (1568×1045, 1 image px = 4.0 doc px)`
(`look.go:151-162`). An eval delivers at most four looks of at most 8 MiB together (`pkg/cvsession/session.go:40-43`).

### 3.2 Render scale and zoom

The **scale** of an evaluation is buffer px per doc px. Previews and looks use scales at or below 1, because a
pyramid level or the full-resolution source provides the pixels. **Zoom** is a scale above 1: at zoom 8 each doc
px becomes an 8 × 8 block of screen pixels. Zoom adds no information; it makes existing pixels large enough for
a vision model to judge shape, edge softness and neighbourhood. A 3 px speck becomes 24 px wide.

The **interpolation** decides what fills the block. `nearest` repeats the pixel, so pixel boundaries stay
visible and values are exact; `linear` blends neighbours, which looks smoother and invents intermediate values.
For inspection `nearest` is the default; the colleague's zoom script uses it too
(`scripts/80-vlm-render.py`, line 76: "Upscale (nearest)").

### 3.3 Pixel ops and decorations

Two kinds of op produce images, and they behave differently under zoom.

A **pixel op** computes values from values: exposure, blur, arithmetic, levels, crop. Computing it above scale
1 would mean computing on invented pixels; a Gaussian of an 8× nearest-upsampled image is not an 8× view of a
Gaussian. So pixel ops never run above scale 1.

A **decoration** draws marks on top of an image: region outlines and ids (overlay), circles, points, polylines
and text (annotate), labels on a sheet. Its marks are defined in doc px but drawn in screen px: a circle outline
should be 1–2 screen px wide at any zoom, and a label should be readable at any zoom. A decoration therefore
runs at the render scale, on an input that the evaluator has upsampled.

```
look({ zoom: 8 }) of  img.highpass(3).crop(box).annotate(marks)

  scale 8   annotate  ◀── draws 1 px circles and 7 px text on the upsampled crop
                │
                ▼ evaluator: upsample ×8 (nearest)
  scale 1   crop ── highpass ── source        (pixel ops, at most scale 1)
```

### 3.4 Crops and frames

GCV-001 introduced the **frame**: the rectangle (origin and size, in doc px) an image covers. A **crop** is a
pixel op whose result has a smaller frame: `img.crop([1300, 100, 64, 64])` has frame origin (1300, 100) and
size 64 × 64. Coordinates passed to ops after a crop, and coordinates ops report about a crop (region
centroids), stay in the root doc px of the original image. Rulers on a crop are labelled 1300 … 1364, not 0 … 64.

### 3.5 Signed images

The difference of two images has a sign: positive where the first is brighter, negative where it is darker.
GCV-001 tags such results as **derived** (linear encoding): their stored value is their value. A look clamps
to [0, 1], so a signed image needs a mapping before it can be seen: add 0.5 so that zero is mid-grey, or use
GCV-001's `colormap("diverging")`, which shows negative values blue, zero white and positive values red.

### 3.6 High-pass and difference of Gaussians

A Gaussian blur of radius r keeps structures larger than about r and suppresses smaller ones. Subtracting it
leaves what differs from its surroundings at scale r: a **high-pass** image. A speck of 3 px on a smooth sky is
almost all high-pass; the sky is almost none. A **difference of Gaussians** (DoG) blurs twice, with r1 < r2, and
subtracts: it keeps structures between about r1 and r2, so it also suppresses the finest grain. The colleague's
best view is a DoG with σ 1 and 4, keeping only the bright part (dust on their density tiles is bright).

**Local contrast** normalizes a high-pass by the local spread: (a − G_r a) / σ_r, where σ_r is the standard
deviation in a window of radius r. Its values are local z-scores: 3 means "three times the local noise above the
local mean", in a dark corner and in a bright one alike.

### 3.7 Stretch

A **stretch** maps a range of values to the display range [0, 1]: `levels({ black, white })`. An **auto
stretch** takes the range from the image itself, usually the 1st and 99th percentiles. A zoomed crop needs its
own stretch: a 64 px window of sky occupies a sliver of the whole tile's range and looks flat grey under the
global stretch.

### 3.8 Contact sheets

A **contact sheet** is one image made of many small crops in a grid, each labelled. A vision model compares
similar crops side by side well, and one sheet of 36 crops replaces 36 looks, of which an eval can deliver only
four.

The key points:

- Zoom is a render scale above 1; pixel ops never compute above 1, decorations always draw at the render scale.
- Crops keep root doc-px coordinates through their frame.
- Signed and normalized views are derived values; they need an offset or a diverging colormap to be seen.
- A view is measured by how early the target appears among everything visible, and the measured DoG view puts
  dust 40 times earlier than a stretched tile.

## 4. Current-state analysis

### 4.1 Looks cannot magnify or show a window

`look.New` (`pkg/cv/look/look.go:66-99`) accepts `MaxSide` between 16 and the engine's `LookLong` (1568) and
derives the scale with `ScaleForLong`, which is never above 1 (`pkg/cv/graph/engine.go:160-171`). The
evaluator rejects any scale outside (0, 1] (`pkg/cv/graph/eval.go:176-178`). There is no box option and no crop
op; `cv.help("crop")` answers with a redirect that says so (`pkg/gojamod/cv/help.go:51`).

### 4.2 Look options are parsed by hand

`__look` parses `label` and `maxSide` itself (`pkg/gojamod/cv/evaltrack.go:220-266`), with its own unknown-option
error and its own fix strings. It bypasses the registry's argument binder, so look options get no synonyms, no
did-you-mean from the shared `checkKeys` (`pkg/cv/graph/args.go:556-585`), and no help entry beyond a
hand-written line in `cvFuncs` (`help.go:34`) and in the generated reference (`pkg/cvsession/reference.go:70`).

### 4.3 Encoding: JPEG unless gray

`look.Encode` writes PNG only for `*image.Gray` and charts (`look.go:211-225`). Colour looks are JPEG q85.
JPEG ringing and 8 × 8 block edges are 2–4 px features, the size of the specks being judged; in a high-pass view
they become fake specks (issue 15).

### 4.4 Overlays draw at evaluation scale

`drawRegions` (`pkg/cv/ops/present.go:21-73`) tints region pixels and draws box outlines and ids into the
evaluated buffer, with line width and digit size derived from the buffer's short side (`present.go:41-43`).
Under any future zoom these would be upsampled with the image: an outline 1 px at scale 1 becomes 8 px wide at
zoom 8 and covers the speck it marks.

### 4.5 Two fonts

`kernels/draw.go:9-13` has a 3 × 5 digit font for region ids; `look/font.go:10-28` has a 5 × 7 digit font for
the look number. Neither has letters, so no caption, coordinate label or annotation text can be drawn into an
image.

### 4.6 Differences are unsigned

`compare({ mode: "diff" })` (`pkg/cv/ops/present.go:151-160`) calls `kernels.AbsDiff` (`kernels/draw.go:100-121`),
which returns |P(a) − P(b)| · gain in perceptual space, clamped. The sign is lost, and with gain the view draws
contour rings inside bright objects (issue 4). `blend` offers `difference`, `add`, `lighten`, `darken`
(`kernels/composite.go:10-27`) with an opacity and mask, but no subtraction. The colleague builds "brighter than
the surroundings" as

```js
const pos = (src, r) => src.blend(src.blend(src.gaussian(r), { mode: "darken" }), { mode: "difference" })   // = max(src − blur, 0)
```

(`scripts/84-cv-helpers.js`, line 22), which is correct and unreadable, and cannot express the dark or the
signed residual.

### 4.7 Stretching takes two evals

`levels({ black, white })` (`pkg/cv/ops/tone.go:29-57`) needs numbers, so an agent first awaits `stats()` and
then calls `levels`, or lifts `s.p01`/`s.p99` (which works, `pkg/cv/graph/args.go` argument lifting). The
colleague's DoG stretch "white = 5 × p99" needs `await n.stats()` in a helper (`85-cv-views.py`, the `stretch`
function in `PRE`). For a crop, the stretch must come from the crop's own pixels, which no op computes.

### 4.8 Captions do not say what the image is

The caption is the label and the size (`look.go:151-162`). The default label is the last op's name
(`look.go:79-82`). A model looking at a DoG crop sees "look 2: levels (64×64, …)" and must remember how it was
built.

### 4.9 Hand-written module functions

`cv.compare` is written in the prelude (`pkg/gojamod/cv/prelude.js:334-342`): it checks the list and calls the
`compare` op on `list[0]`. Its help is in the hand-maintained `cvFuncs` table (`help.go:24-43`). Every new
module-level function (`sheet`) would repeat both.

## 5. Gap analysis

| need (issue) | today | gap |
|---|---|---|
| Magnify a window (1) | whole image ≤ 1568 px | crop node, render scale > 1, nearest upsampling |
| Read coordinates (2) | none | rulers and grid with root doc-px labels |
| Many crops at once (3) | ≤ 4 looks | contact sheet node |
| Signed residuals (4) | unsigned diff, blend tricks | `sub` and friends, derived results |
| High-pass views (5) | 4 blends + stats | `highpass`, `dog` macros |
| Per-crop stretch, local contrast (8) | two evals | `levels({ auto })`, `localContrast` |
| Draw marks (14) | regions and masks only | `annotate` decoration |
| Lossless looks (15) | JPEG colour | `format`, PNG for zoomed and small looks |
| View menu (22) | none | `img.view(name)` presets, docs page |
| Provenance (23a) | label only | op-chain caption |

## 6. Legacy to remove or refactor

1. **`kernels.AbsDiff` and the unsigned `compare` diff.** `compare({ mode: "diff" })` becomes a macro over
   `sub`, `mul` and `add` (signed, zero at mid-grey). `AbsDiff` is deleted.
2. **Duplicate blend modes.** `blend`'s `add`, `difference`, `lighten`, `darken` duplicate `add`,
   `sub(…).abs()`, `max`, `min`. They are removed; `blend` keeps the compositing modes `normal`, `multiply`,
   `screen`. Both `Blend` and the arithmetic ops use one `kernels.Binary(a, b, f)` kernel.
3. **Two fonts.** `kernels/draw.go` digits and `look/font.go` glyphs merge into `pkg/cv/text`: one 5 × 7 bitmap
   font with digits, letters and the punctuation captions need (`[ ] ( ) , . : - + × → %`), drawn at an integer
   pixel size.
4. **Hand-parsed look options.** `lookOptionKeys` and the parsing in `evaltrack.go:220-266` go; look options are
   an `[]graph.ArgSpec` bound with a new exported `graph.BindOptions(specs, raw, where)`, which is the same code
   path op options use.
5. **Hand-written `cv.compare`** and its `cvFuncs` entry. Every registry op with receiver `KindNone` (except
   `open`, `param` and the builder placeholder) is exposed as `cv.<name>` by the prelude, and its help comes from
   the registry. `compare` becomes such an op with an `images` list; `sheet` is the second.
6. **Overlay drawn at evaluation scale.** `overlay` becomes a decoration op; outline width and label size are
   in screen px.
7. **`look.Encode`'s gray-or-JPEG rule.** Replaced by the `format` option with an `auto` rule (§7.7).
8. **The `crop` help redirect** (`help.go:51`). `crop` is an op.
9. **Hand-written look line in the reference** (`reference.go:70`). Generated from the look ArgSpecs.
10. **Scale validation** `(0, 1]` in `Evaluator.Evaluate` (`eval.go:176`). Becomes `(0, MaxRenderScale]` with
    `MaxRenderScale = 16`, and only looks may ask for scales above 1.

Outside goja-cv, the colleague's `80-vlm-render.py --zoom` and the `pos`/`stretch` helpers in
`84-cv-helpers.js` and `85-cv-views.py` become unnecessary; the playbook update is listed in §11.

## 7. Proposed architecture

### 7.1 The rule: nodes for content, options for the raster

| it depends on | it is | examples |
|---|---|---|
| pixel values or data | a node | `crop`, `sub`, `highpass`, `levels({ auto })`, `annotate`, `overlay`, `sheet`, `view` |
| only the output raster (scale, interpolation, margins, file format) | a look option | `zoom`, `interp`, `maxSide`, `ruler`, `grid`, `format`, `label` |

Because views are nodes, they are cached, hash-consed, measurable (`hp.crop(box).stats()`), reusable in a
sheet, and visible in `explain()` and in the caption. Because render parameters are look options, the same node
can be looked at at 1× and 8× without building a second graph.

The colleague's one-eval acceptance test for issue 1 reads:

```js
const dog = img.view("speck")
return dog.crop([1300, 100, 64, 64]).levels({ auto: true }).look({ zoom: 8, ruler: 8 })
```

### 7.2 Render scale and decorations in the evaluator

```go
// pkg/cv/graph/eval.go
type EvalOptions struct {
    Scale  float64 // buffer px per doc px, in (0, MaxRenderScale]; above 1 only for looks
    Interp Interp  // upsampling for scales above 1: InterpNearest (default) or InterpLinear
}

const MaxRenderScale = 16

// OpDef gains:
//   Decoration bool // draws at the render scale; inputs are evaluated (and upsampled) at that scale
```

The evaluator change is small and sits next to the existing per-scale runs (`evalRun.at`,
`pkg/cv/graph/eval.go:145-159`):

```
evalRun.eval(id) at scale s:
  n := node(id)
  if s > 1 and not n.Op.Decoration:
      v := at(1).eval(id)                      // pixel ops never compute above scale 1
      return memo(upsample(v, s, interp))      // cached under (node, s, interp)
  … unchanged: AnalysisScale, cache lookup, compute (inputs evaluated in this run, i.e. at s)
```

The cache signature includes the interpolation when s > 1. Value nodes, Regions nodes and analysis ops are
never decorations; Regions and Values are evaluated at their own scales as today (a Regions node with an
`AnalysisScale` is unaffected by zoom).

`buffer.Upsample(b, k, interp)` handles integer and fractional factors; `nearest` maps buffer pixel (i, j) at
scale s to source pixel (⌊i / s⌋, ⌊j / s⌋), so a zoom-8 look of a 64 px crop is exactly 512 × 512 with 8 × 8
blocks.

### 7.3 Crop

```go
{
    Name: "crop", On: graph.KindImage, Returns: graph.KindImage,
    Args: []graph.ArgSpec{{Name: "box", Type: "box", Unit: graph.DocPx, Positional: true,
        Doc: "[x, y, w, h] in doc px of the original image; clipped to the image"}},
    // Space: zero rule (plane copy, no conversion); Result: KeepEnc
    DocFrame: func(a graph.Args, in []*graph.Node) graph.Frame { return clip(a.Box("box"), in[0].Frame) },
    Run: runCrop,
}
```

- Crop is a pixel op: at scale s ≤ 1 it cuts the input buffer at s; the cut is aligned to buffer pixels
  (floor of the origin, ceil of the far edge) and the frame records the aligned box so labels stay exact.
- A box entirely outside the image is `E_ARG_RANGE` with a clipped box as the fix; a partly outside box is
  clipped and a `RANGE` warning says so.
- **CROP_FIRST warning.** Ops with a neighbourhood (`OpDef.Halo` > 0: gaussian, highpass, dog, localContrast,
  feather, heal) applied *after* a crop see only the window and its reflected border. The builder warns when it
  interns such an op on a node whose chain contains a crop: "gaussian after crop sees only the 64 px window;
  put crop last. fix: img.gaussian(3).crop([…])". This is the first reader of `Halo` (declared since M1,
  `pkg/cv/graph/registry.go:108`).

Crops make previews of large images cheap only once the evaluator can evaluate a window of the input (§14,
windowed evaluation). For 1024 px tiles and windows opened with GCV-001's `open({ box })`, evaluating the full
input at scale 1 costs milliseconds and is shared by every crop of a sheet through the cache.

### 7.4 Signed arithmetic

| call | operand | result encoding | notes |
|---|---|---|---|
| `a.add(b)` | Image or number (Level) | keeps `a`'s | |
| `a.sub(b)` | Image or number (Level) | derived | signed |
| `a.mul(b)` | Image or number (unitless factor) | keeps `a`'s | `mul(-1)` negates |
| `a.min(b)`, `a.max(b)` | Image or number (Level) | keeps `a`'s | `d.max(0)` keeps the positive part |
| `a.abs()` | — | keeps `a`'s | |

All are pixel ops with `Space: {Default: encoded}`: arithmetic works on the values a look shows unless the call
passes `{ space }`. Image operands must have the same frame (`E_FRAME`, "the inputs cover different
rectangles", with both frames). A gray operand is broadcast to three channels, as `matchChannels` does today
(`kernels/composite.go:29-36`).

The names match the existing `Value` arithmetic (`pkg/cv/ops/value.go:125-147`), which stays: the registry keys
ops by receiver kind (`registry.go:132-133`), so `stats().p99.mul(5)` and `img.mul(5)` are different ops with one
vocabulary.

There is no `offset` option. Adding 0.5 is `.add(0.5)`, which is what an offset would do; one mechanism is
enough, and the docs show both idioms for signed maps:

```js
const d = img.sub(img.gaussian(3, { space: "encoded" }))
return {
  grey: d.mul(4).add(0.5).look("signed, grey"),                       // zero is mid-grey
  colour: d.colormap("diverging").look("signed, diverging"),          // zero is white (GCV-001)
}
```

### 7.5 High-pass, DoG and local contrast

`highpass` and `dog` are macros (GCV-001 §7.8). Their expansion is the graph an expert would write by hand, so
hash-consing makes `img.highpass(3)` and the hand-written chain the same node:

```go
// highpass(r, { polarity, space }) = d, d.max(0) or d.mul(-1).max(0), with d = a − G_r(a), all in space
func expandHighpass(x *graph.Expander, a *graph.Node, args graph.Args) ([]*graph.Node, error) {
    sp := args.String("space")
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

| call | expansion | default |
|---|---|---|
| `img.highpass(r, { polarity, space })` | `a − G_r(a)`, then polarity | polarity `"both"`, space encoded |
| `img.dog(r1, r2, { polarity, space })` | `G_r1(a) − G_r2(a)`, then polarity; `r2 > r1` | polarity `"both"`, space encoded |
| `img.localContrast(r, { space })` | primitive kernel: (a − G_r a) / (σ_r + 1e-3) | space encoded; derived |

Polarity `"bright"` keeps `d.max(0)`; `"dark"` keeps `d.mul(-1).max(0)` (darker-than-surroundings shown
bright). Both expansions use the op's space for the inner Gaussian, so on the colleague's linear-encoded density
tiles everything runs on v, and on an sRGB photograph the default runs on encoded values, where the eye judges
differences. `dog` with `r2 ≤ r1` is `E_ARG_RANGE` with the swapped call as the fix.

`localContrast` is a primitive op rather than a macro because it needs a square and a square root
(σ_r = sqrt(G_r(d²))), which are not worth adding as public ops. It reuses the Gaussian kernel twice.

### 7.6 Auto stretch

`levels` gains one option, `auto`:

```js
img.levels({ auto: true })              // black, white = p01, p99 of the input, in levels' space
img.levels({ auto: [0.005, 0.995] })    // explicit percentiles
```

`auto` and `black`/`white` are mutually exclusive (`E_ARG_TYPE` with both fixes). With `auto`, levels becomes a
`Global` op (`registry.go:110`): its kernel computes the percentiles of its input at the evaluation scale (the
existing `kernels.Percentile`, `kernels/measure.go:115-135`), then remaps. Placed after a crop it stretches by
the crop's pixels, which is the per-box stretch of issue 8 without a `box` option. The colleague's "white = 5 ×
p99" stays a lifted expression: `d.levels({ black: 0, white: d.stats().p99.mul(5) })`.

### 7.7 Looks

```js
node.look({ label, zoom, interp, maxSide, ruler, grid, format })
node.look("label")    // shorthand for { label }
```

| option | type | default | meaning |
|---|---|---|---|
| `label` | string | the last op of the chain | caption text |
| `zoom` | number in [1, 16] | fit to `maxSide` | render scale (screen px per doc px); exclusive with `maxSide` |
| `interp` | `"nearest" \| "linear"` | `nearest` | upsampling above scale 1 |
| `maxSide` | integer [16, 1568] | 1568 | long side when fitting (scale ≤ 1) |
| `ruler` | number (doc px) or `true` | none | tick spacing; `true` picks 1, 2, 4, 8, 16, 32 … so ticks are ≥ 40 screen px apart |
| `grid` | number (doc px) | none | faint lines every `grid` doc px across the image |
| `format` | `"auto" \| "png" \| "jpeg"` | `auto` | `auto`: PNG for gray, zoomed (zoom > 1), sheets and looks of ≤ 512 × 512 px; JPEG q85 otherwise |

The options are declared as `[]graph.ArgSpec` in `pkg/cv/look/options.go` and bound with `graph.BindOptions`,
so `{ magnify: 8 }` gets "Did you mean zoom?" through option synonyms (`magnify`, `scale`, `upscale` → `zoom`;
`ticks`, `axes`, `coords` → `ruler`; `lossless`, `png: true` → `format`), and `{ box: […] }` on a look gets
`E_UNKNOWN_OPTION` with the literal fix `node.crop([x, y, w, h]).look({ zoom: 8 })`.

A zoomed look larger than 1568 px on its long side is `E_ARG_RANGE` with a fix that keeps the zoom and shrinks
the crop, or keeps the crop and lowers the zoom.

**Rulers and grid.** Rulers are drawn by the renderer into margins added around the rendered image: 28 px on top
and left, and on bottom and right when the image is at least 256 px on that side. Tick labels are root doc px
from the node's frame: a tick at doc x = 1312 on a crop with origin x 1300 rendered at zoom 8 sits at screen x
= margin + (1312 − 1300) · 8, the left edge of doc pixel 1312. goja-cv coordinates are continuous: pixel i
covers [i, i + 1), so a region centroid of 1312.5 is the centre of that pixel (`pkg/cv/graph/regions.go:97`). The grid draws 1 px lines at 35 % opacity in a
colour that contrasts with the local background (white over dark, black over light). Rulers do not change the
node; the caption mentions them.

**Captions.** The caption states what the image is:

```
look 2: speck view ×8 — open(1d955…tif) → view(speck) → crop[1300,100,64,64] → levels(auto) (512×512 + ruler, 1 image px = 0.125 doc px, doc x 1300–1364, y 100–164)
```

The chain comes from the receiver path (`Evaluator.Path`, `eval.go:348-363`) with a compact describer (op name
and its non-default positional args); macros appear under their own name (`view(speck)`, `highpass(3)`) by
recording the macro call on the first node of its expansion (`Node.Macro string`), so captions stay short while
`explain()` shows the full expansion. Chains longer than 120 characters keep the source and the last three ops
with "…" between.

**Rendering pipeline.**

```
Render(look):
  s := zoom or ScaleForLong(node, maxSide)
  v := Evaluate(convert(node → encoded), {Scale: s, Interp: interp})       // GCV-001 display rule
  img := Quantize(v.Buf, 8)
  if ruler or grid: img = addMarginsAndTicks(img, v.Frame, s, ruler, grid)  // pkg/cv/look/ruler.go
  BurnNumber(img, n)                                                        // pkg/cv/text
  encode by format rule
```

### 7.8 Decorations: overlay and annotate

`overlay` (`pkg/cv/ops/present.go:88-112`, `pkg/cv/ops/mask.go:387-400`) becomes a decoration. At render scale s
it receives the image upsampled to s and the regions or mask; it rasterizes region pixels at s (the area-exact
`RegionsValue.Rasterize`, `pkg/cv/graph/regions.go:194-240`, already works at any target size) for the tint, and
draws box outlines (1 screen px, 2 px when the look is ≥ 1024 px) and ids with `pkg/cv/text` at 2× font size.

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

- Coordinates are root doc px; marks outside the frame are skipped (a crop of an annotated image shows only its
  marks).
- `width` is in screen px (default 1), so a circle stays thin at zoom 8.
- `color` takes every colour form `overlay` accepts (CSS names, `#rgb`, `#rrggbb`, `[r, g, b]`); colours are
  display (encoded) values.
- A mark object with no known key, or with two shape keys, is `E_ARG_TYPE` listing the shapes, with did-you-mean
  over shape names (`circ` → `circle`, `line` → `polyline`, `label` alone → `text`).
- With today's region items, `{ box: r.box }` marks a region; after GCV-003 items carry flat `x, y, w, h` fields
  and box arguments accept such objects, so `{ box: item }` works directly.

### 7.9 Contact sheets

```js
const spots = img.view("speck").mask.luminance({ above: 0.5 }).components({ minArea: 4 })
return cv.sheet(spots.top(36), { views: [img.levels({ auto: true }), img.view("speck")], size: 48, zoom: 4, labels: "id" }).look("candidates")
```

`sheet` is a registry op with receiver `KindNone`, exposed as `cv.sheet` (§7.10):

| option | type | default | meaning |
|---|---|---|---|
| items (positional) | Regions, or a list of `[x, y]` points or `[x, y, w, h]` boxes | — | one cell (or one row of cells) per item |
| `views` | Image node or list of Image nodes | — (required) | the images cut; several views give one column per view |
| `size` | doc px | 64 | side of the square window centred on each item |
| `zoom` | 1–16 | 4 | magnification of each cell (nearest) |
| `cols` | integer | as many as fit 1568 px | cells per row (one view); rows are items when several views |
| `labels` | `"id" \| "index" \| "none"` | `id` for Regions, `index` otherwise | the label under each cell, plus its centre in doc px |
| `stretch` | `"none" \| "cell"` | `none` | `cell` applies `levels({ auto: true })` to each cell's window |

Evaluation: the views are evaluated at scale 1 (pixel ops), each cell is cut (a window may extend past the image
edge; the outside is filled with a neutral dark grey and the label marks it), stretched if asked, upsampled by
`zoom`, and pasted with a 6 px gap; labels are drawn with `pkg/cv/text` under each cell (`7 (1337, 131)`). A
sheet is evaluated only at scale 1 and its frame is a **sheet frame** (`Frame.Sheet = true`): its pixels are
not doc px, so ops that take doc-px coordinates refuse it (`E_WRONG_KIND`: "a sheet is a picture of crops; its
pixels are not doc px") and a look of it cannot use `zoom` or `ruler`. A sheet too large for 1568 px is
`E_ARG_RANGE` at build time with fixes for `cols`, `size` and `zoom`. At the defaults, 36 items of one view fit
as 6 × 6 cells of 256 px.

Region items are centred on their centroid. Points are centres. Boxes are centred on the box centre and use
`max(w, h, size)` as the window side, so a long hair box is not cut.

### 7.10 Module functions from the registry

The prelude builds `cv` from a hand-written object (`prelude.js:305-351`). After this ticket it adds one method
per registered `KindNone` op that is not `open`, `param` or the placeholder:

```js
for (const name of native.__moduleOps()) {        // ["compare", "sheet"]
  cv[name] = function () { return callOp({ __id: 0, __kind: "" }, name, Array.prototype.slice.call(arguments)); };
}
```

`compare` becomes `{ Name: "compare", On: KindNone, Args: images (Image[] positional), mode, gain }` and its
`diff` mode is a macro: `a.sub(b).mul(gain).add(0.5)` in encoded space. `cv.help("sheet")` and
`cv.help("compare")` come from the registry; their `cvFuncs` entries go.

### 7.11 View presets

```js
img.view("speck", { size: 3 })      // the colleague's dog: bright DoG σ size/3 … 4·size/3, white = 5 × p99, gray
```

`view` is a macro op on Image with an enum name and a small option set. Each preset is a Go function in
`pkg/cv/ops/views.go` that expands into primitive ops, plus a one-sentence description used by help and by the
docs page. The first set, chosen from the colleague's measurements and the generic inspection list:

| name | expansion | shows | use it for |
|---|---|---|---|
| `scene` | `levels({ auto: true })` | the image, stretched 1–99 % | judging what belongs to the scene |
| `speck` | `toGray("average").dog(s/3, 4s/3, { polarity: "bright" })`, `levels({ black: 0, white: p99 × 5 })` | small bright things that differ from their surroundings | finding dust on density tiles, stars, sensor defects |
| `speckDark` | same with `polarity: "dark"` | small dark things | dust on positives and prints |
| `speckColor` | `dog(…)` on all channels, same stretch, colour | the same, in colour: neutral specks stay white | separating neutral defects from colour grain |
| `signed` | `highpass(s)`, `colormap("diverging")` | brighter (red) and darker (blue) than surroundings | edges, scratches, both polarities |
| `detail` | `localContrast(4s)`, `colormap("diverging", { range: [-4, 4] })` | detail equally visible in shadows and highlights | sweeping a whole tile |
| `neutral` | `chroma({ relative: true })`, `colormap("viridis", { range: [0, 1] })` | where channels disagree | telling grey dust from coloured detail |

`size` (doc px, default 3) is the typical size of the target; presets derive radii from it. The look caption
shows `view(speck)` and `explain()` the expansion. Presets are not options of other ops and have no synonyms
among themselves; an unknown preset name gets did-you-mean over the preset names and the list with descriptions.

### 7.12 Text: `pkg/cv/text`

```go
package text

// Face is the built-in 5×7 bitmap font: digits, A–Z, a–z, and [ ] ( ) , . : - + × → % ? / _.
func Width(s string, px int) int
func Draw(dst draw.Image, x, y int, s string, px int, fg, shadow color.Color)
```

`px` is the size of one font pixel in screen px (1 for ruler labels at 7 px height, 2 for sheet labels). It
draws on `image.Image` (after quantization), so text is always in display colours and independent of spaces.
Overlay and annotate draw onto float buffers; they rasterize text through a small adapter that writes encoded
values into the buffer at render scale.

## 8. Decision records

### Decision: views are nodes, render parameters are look options

- **Context:** Issue 1 proposes both `look({ box, zoom })` and a lazy `crop(box)`; issues 2, 8 and 15 add more
  look options. The project wants one way to do each thing.
- **Options considered:** (a) everything as look options (box, stretch, marks, ruler, zoom); (b) everything as
  nodes (including zoom and ruler as ops); (c) nodes for content, look options for the raster.
- **Decision:** (c).
- **Rationale:** Under (a) a crop could not be measured, put in a sheet, or explained, and a per-crop stretch
  would be a second implementation of `levels`. Under (b) zoom and rulers would be nodes whose "pixels" are not
  doc px, breaking the meaning of scale. (c) gives every content transform the graph's caching, hashing and
  explanation, and keeps the raster concerns where the raster is made.
- **Consequences:** `look({ box })` is an error with a literal fix to `crop`. One eval still suffices for the
  acceptance test (§7.1).
- **Status:** proposed

### Decision: pixel ops cap at scale 1; decorations render at the render scale

- **Context:** Zoomed looks need thin lines and readable labels, and must not compute filters on invented
  pixels.
- **Options considered:** (a) evaluate everything at scale ≤ 1 and upsample the final image (lines and text
  grow with zoom); (b) evaluate everything at the render scale (filters on upsampled pixels); (c) a decoration
  flag: pixel ops at ≤ 1 with automatic upsampling, decorations at the render scale.
- **Decision:** (c).
- **Rationale:** (a) makes an 8× look of an overlay cover the specks with 8 px outlines. (b) changes the values
  of every filter with the zoom. (c) adds one branch to `evalRun.eval` and one flag to `OpDef`.
- **Consequences:** The cache holds upsampled buffers keyed by scale and interpolation; zoomed looks are small
  (≤ 1568²), so the cost is bounded.
- **Status:** proposed

### Decision: signed arithmetic as Image ops with the Value names; no offset option

- **Context:** Issue 4 proposes `sub`, `add`, `mul`, `abs`, `max`, `min` and an `offset` option.
- **Options considered:** (a) a single `math(expr)` op; (b) named ops plus `offset`; (c) named ops without
  `offset`.
- **Decision:** (c).
- **Rationale:** Named ops are discoverable, typed and composable; an expression language would need its own
  parser and error messages. An `offset` option is exactly `.add(0.5)` and would be a second spelling;
  `colormap("diverging")` covers the case where the offset is a display device.
- **Consequences:** The blend modes that duplicated arithmetic are removed; recipes and the colleague's helpers
  are rewritten (§11).
- **Status:** proposed

### Decision: high-pass and DoG are macros; local contrast is a kernel

- **Context:** The views are compositions of existing ops; local contrast needs a square root.
- **Options considered:** (a) dedicated kernels for all three; (b) macros for all three with new `square`/`sqrt`
  ops; (c) macros for highpass/dog, a kernel for localContrast.
- **Decision:** (c).
- **Rationale:** Macros cost no kernel code and inherit caching of their parts (a sheet of speck and signed views
  shares the Gaussians). Adding public `sqrt` and `square` only to define one view would grow the surface for
  little use.
- **Consequences:** `img.highpass(3)` and the hand-written chain are the same node, which tests assert.
- **Status:** proposed

### Decision: presets are `img.view(name)`, not `cv.view(name)`

- **Context:** Issue 22 proposes `cv.view("speck", { size })`, implicitly on `img`.
- **Options considered:** (a) `cv.view(name, opts)` on the bound `img`; (b) `cv.view(name, image, opts)`;
  (c) `image.view(name, opts)`.
- **Decision:** (c).
- **Rationale:** With GCV-001's named images there is no single implicit image; (c) names the image the way every
  other op does, and it composes (`ref.view("speck")`). `cv.view` is answered by the existing "op called on the
  wrong object" translation with the fix `img.view("speck")`.
- **Consequences:** The docs page lists presets under "Views".
- **Status:** proposed

### Decision: contact sheet is a node with a sheet frame, exposed as `cv.sheet`

- **Context:** Issue 3 asks for one image of many crops; the generic list (A4) asks for the same crop from several
  views side by side.
- **Options considered:** (a) a look-level function returning a look; (b) an Image method `view.sheet(items)`;
  (c) a module op `cv.sheet(items, { views })`.
- **Decision:** (c).
- **Rationale:** (c) supports several views per item, which (b) cannot express without a second call, and it is
  a node, so it can be shown, sized and cached like any image, which (a) cannot.
- **Consequences:** Sheet frames are not doc px; coordinate ops and rulers refuse them.
- **Status:** proposed

### Decision: lossless by default where it matters

- **Context:** Colour looks are JPEG; ringing is the size of specks (issue 15).
- **Options considered:** (a) always PNG; (b) a `format` option only; (c) `format` with an `auto` rule.
- **Decision:** (c).
- **Rationale:** Zoomed crops, sheets and small looks are where artefacts mislead, and they are small, so PNG costs
  little. Whole-tile colour looks stay JPEG to respect the 8 MiB budget for four looks.
- **Consequences:** Look tests that assert MIME types change for small colour looks.
- **Status:** proposed

## 9. Pseudocode and key flows

### 9.1 A zoomed crop with a ruler, end to end

```
agent: img.view("speck").crop([1300,100,64,64]).levels({ auto: true }).look({ zoom: 8, ruler: 8 })

build:
  view(speck)  → expands to toGray → gaussian(1, enc) , gaussian(4, enc) → sub → max(0) → stats.p99.mul(5) → levels   (#5…#12)
  crop         → #13 frame {1300, 100, 64, 64}
  levels(auto) → #14 Global
  look         → Look{node #14, zoom 8, ruler 8, format auto→png}

render (after the eval, off the owner goroutine):
  s = 8
  convert(#14 → encoded) is identity (derived, linear encoding)
  eval #14 at 8: not a decoration → eval at 1, upsample ×8 nearest
     eval #14 at 1: levels(auto) computes p01/p99 of #13's 64×64 pixels, remaps
        eval #13 at 1: cut [1300,100,64,64] from #12 at 1 (full tile, cached)
  quantize → 512×512 gray
  rulers: margins 28 px, ticks every 8 doc px labelled 1300, 1308, …, 1364 / 100, …, 164
  burn number; PNG
caption: look 1: speck view ×8 — open(t.tif) → view(speck) → crop[1300,100,64,64] → levels(auto) (512×512 + ruler, 1 image px = 0.125 doc px, doc x 1300–1364, y 100–164)
```

### 9.2 Sheet composition

```
sheet.Run(items, views, opts) at scale 1:
  centres := itemsToCentres(items)               // Regions: centroid; point; box centre
  for each view v: buf[v] := eval(v at 1)        // shared through the cache
  cell := size · zoom
  layout: one view → cols = opts.cols or floor((1568 + gap) / (cell + gap)); several views → one row per item
  out := New(W, H, 3) filled with the sheet background
  for each (item i, view v):
      win := window(buf[v], centre_i, side_i)    // outside the image → dark grey
      if stretch == cell: win = levelsAuto(win)
      paste(out, upsampleNearest(win, zoom), x, y)
      text.Draw(out, label_i under the cell)
  return Image{Frame{0, 0, W, H, Sheet: true}, Enc: srgb (display)}
```

### 9.3 Decoration evaluation

```
evalRun.eval(annotate#20) at s = 8:
  annotate is a decoration → compute here at s
  inputs: #19 (crop) at s → not a decoration → eval at 1, upsample ×8 (memoized)
  Run: copy input, for each mark: map doc px → buffer px with frame origin and s; draw 1 px outlines and text
```

## 10. Errors and diagnostics

| situation | code | message and fix |
|---|---|---|
| `look({ box })`, `look({ crop })` | `E_UNKNOWN_OPTION` | "crop is an op" — fix `node.crop([x, y, w, h]).look({ zoom: 8 })` |
| zoomed look too large | `E_ARG_RANGE` | both fixes (smaller box, smaller zoom) with numbers |
| `zoom` with `maxSide` | `E_ARG_TYPE` | "zoom sets the scale; maxSide fits it" |
| crop outside the image | `E_ARG_RANGE` | clipped box |
| filter after crop | `CROP_FIRST` warning | reordered chain |
| frames differ in arithmetic | `E_FRAME` | both frames; "crop both the same way, or crop after the arithmetic" |
| `dog(4, 1)` | `E_ARG_RANGE` | `dog(1, 4)` |
| `levels({ auto, white })` | `E_ARG_TYPE` | `levels({ auto: true })` or `levels({ black, white })` |
| unknown view | `E_ARG_TYPE` (enum) | did-you-mean and the preset list with descriptions |
| coordinates on a sheet | `E_WRONG_KIND` | "a sheet's pixels are not doc px; use the items' coordinates" |
| unknown annotate shape | `E_ARG_TYPE` | shape list, did-you-mean |
| `cv.view(...)` | unknown member of `cv` | fix `img.view("speck")` |
| `blend({ mode: "difference" })` | `E_ARG_TYPE` (enum) | fix `a.sub(b).abs()` (an enum-value redirect table: `difference` → `sub().abs()`, `add` → `add`, `lighten` → `max`, `darken` → `min`) |

Op synonyms gain `subtract`, `minus` (on Image; `minus` stays the Mask op) → `sub`; `multiply` → `mul`;
`difference`, `diff` → `sub`; `hp`, `highPass`, `residual` → `highpass`; `DoG`, `bandpass` → `dog`; `clahe`,
`normalize` → `localContrast`; `zoom`, `window`, `roi` → `crop`; `marks`, `markers`, `draw` → `annotate`;
`contactSheet`, `grid`, `mosaic` → `sheet` (on `cv`); `preset` → `view`. Each synonym is checked for collisions
with existing op names by the existing synonym test.

## 11. Documentation and examples that change

- New topic `pkg/cv/docs/topics/11-views.md` (id `views`): what high-pass, DoG and local contrast show in plain
  words ("shows only what differs from its surroundings at size r", as issue 5 asks), the preset table with
  when to use each, signed maps (grey offset vs diverging), and a measured note: the speck view puts dust about
  40 times earlier than a stretched tile on the colleague's tiles. A test asserts that every preset name in
  `views.go` appears in this topic (the article's drift concern about hand-written surfaces).
- `06-looks.md` rewritten: render scale and zoom, rulers, formats, captions with chains, sheets; the "What can
  be looked at" list gains sheets and annotations.
- `05-masks-and-regions.md`: "Looking at selections" uses `overlay` at zoom.
- `09-recipes.md`: "Inspect candidates" (speck view → components → sheet → zoomed crops), "Compare two versions"
  (signed diff with diverging colormap), "Mark points" (annotate).
- MCP instructions (`reference.go:15-37`): one line each for crop + zoom + ruler, `cv.sheet`, `img.view`; the
  `compare` line says "diff is signed (grey = no change)".
- The colleague's playbook (`DUST-002/reference/04`): step 3 replaces the Python zoom with
  `dog.crop(box).look({ zoom: 8, ruler: 4 })` and a sheet of the hot list; `84-cv-helpers.js` `pos()` becomes
  `highpass`/`view("speck")`. This is their document; the guide lists the change for the hand-over note.

Tests and examples that change: `blend` mode lists in docs and `pkg/cv/ops/composite_test.go`; `compare` diff
expectations (`pkg/cv/ops/present_test.go`) become signed around 0.5; overlay goldens; look MIME assertions in
`pkg/cv/look/look_test.go:151-175` (small colour looks become PNG); `cvFuncs` help tests for `compare`.

## 12. Implementation phases

**Phase 1 — text and look options.**
1. Tests: `pkg/cv/text` renders every glyph inside its box; `Width` is consistent; burned look numbers read back
   (the existing `GlyphPixel` test moves).
2. Merge the fonts; delete `kernels.DrawDigits` and `look/font.go` glyphs.
3. `graph.BindOptions`; look options as ArgSpecs; delete `lookOptionKeys`; synonyms; tests for did-you-mean and
   the `box` literal fix.

**Phase 2 — render scale and decorations.**
1. Tests first (`pkg/cv/graph/render_test.go`): a pixel op evaluated at scale 8 equals nearest-upsampled scale-1
   output exactly; decorations receive upsampled inputs; cache entries per (scale, interp); scale > 16 and
   scale > 1 outside looks are errors.
2. `EvalOptions.Interp`, `OpDef.Decoration`, the branch in `evalRun.eval`, `buffer.Upsample`.
3. `look({ zoom, interp })`, size checks, captions with the scale below 1 ("1 image px = 0.125 doc px").

**Phase 3 — crop, rulers, grid, captions.**
1. Tests: crop frames and clipping; `CROP_FIRST` for Halo ops after crop; ruler tick positions and labels for a
   crop at zoom 8 (golden PNG of a 32 × 32 synthetic crop); chain captions for plain, macro and long chains.
2. Implement `crop`, `look/ruler.go`, chain captions (`Node.Macro`).

**Phase 4 — arithmetic and blend cleanup.**
1. Tests: sign preservation; derived encoding; broadcast gray; frame mismatch; Value and Image ops with the same
   names coexist; `blend` modes removed with redirect fixes.
2. `kernels.Binary`, the six ops, `blend` reduction, delete `AbsDiff`, `compare` as a `KindNone` op with a diff
   macro, `__moduleOps` in the prelude.

**Phase 5 — views.**
1. Tests: `img.highpass(3)` and the hand-written chain have the same node id; `dog` polarity cases on a planted
   speck (bright speck positive in `bright`, zero in `dark`); `localContrast` of a constant image is 0 and of
   noise has unit spread; `levels({ auto })` maps p01/p99 to 0/1; every preset builds and renders.
2. `highpass`, `dog`, `localContrast`, `levels.auto`, `views.go`.

**Phase 6 — decorations and sheets.**
1. Tests: overlay outlines are 1 screen px at zoom 1 and 8; annotate shapes land on the right pixels (golden);
   sheet layout, labels, out-of-image cells, several views, size errors.
2. `overlay` as a decoration, `annotate`, `sheet`, format `auto`.

**Phase 7 — documentation and agent check.**
1. Topics, recipes, instructions, synonyms (§10, §11); docs tests pass.
2. Gated view-ranking test and the pi task (§13).

## 13. Test strategy

### 13.1 Unit and property tests

- Upsampling exactness (`nearest`), decoration scale, cache keys.
- Macro equivalence by node identity (highpass, dog, compare diff).
- Arithmetic algebra: `a.sub(b).add(b)` equals `a` within float error (property over random buffers);
  `a.max(b)` ≥ both; `abs` is non-negative.
- Frames: crop of crop composes origins; arithmetic rejects different frames.

### 13.2 Golden looks

Small PNG goldens (≤ 64 × 64 crops, zoom 4–8) for ruler, grid, annotate shapes, overlay at zoom, and a 2 × 2
sheet, generated from the planted scan. PNG goldens are byte-stable; JPEG goldens are not used.

### 13.3 Documentation tests

Every ```js block in the new `views` topic and the rewritten `looks` topic runs against the planted scan
(`pkg/cvsession/docs_test.go`). Add a `js looks=N` block info to assert the number of looks a block returns, so
docs cannot silently stop attaching images.

### 13.4 Real tiles: view ranking (gated)

Port the colleague's metric (`scripts/85-cv-views.py`) to a Go test gated on `GOJACV_TILES` and a points file per
tile. A short script in this ticket's `scripts/` (`01-export-gold-points.py`, run in the dust repository with
`dustdet.gold`) writes `{tile_id}.gold.json` next to each tile (human-labelled dust/hair/scratch points, tile-local
coordinates). The test renders the `speck` view as a look, finds local maxima (5 × 5 plateaus) on the decoded
look, and computes each point's rank. Assert the median rank ≤ 30 and top-50 ≥ 70 % over the labelled tiles
(the colleague measured 18 and 78 %). This guards the preset's definition against well-meant edits.

### 13.5 Agent harness

A question file `questions-gcv-002.md` for the pi harness (`CVJS-001/scripts/06-pi-cv-agent/run.sh` with the
`IMAGE` and `QUESTIONS` overrides from GCV-001), on one gold tile:

1. Show the tile and a view in which small bright specks stand out. Which view did you use?
2. List the 20 most prominent candidate spots with coordinates, and show them in one picture.
3. Zoom on three of them at 8× with coordinates. For each: dust or not, and why?

Run on the build before this ticket (the model has no zoom; record how it copes) and after. Record calls,
errors, looks per eval, and coordinate accuracy against the gold points (distance to the nearest gold point).

## 14. Risks, alternatives and open questions

**Risks.**

- *Full-resolution evaluation for crops of large images.* A crop of a full DNG evaluates the whole chain at scale
  1 (638 MB per RGB buffer). Mitigation now: GCV-001's `open({ box })`. Later: **windowed evaluation**, where
  `EvalOptions` carries a doc-px window and each op's `Halo` enlarges it for its inputs; `Global` ops and
  `AnalysisScale` ops evaluate whole. The `Halo` declarations exist for this; this ticket makes them read by
  `CROP_FIRST` only.
- *Behaviour changes in `compare` and `blend`.* Agents' existing recipes using `blend({ mode: "difference" })`
  break; the enum redirect fix gives the replacement in the error.
- *Look budget.* PNG crops and sheets are larger than JPEG. A 1536² RGB PNG of a sheet can exceed 2 MB; with four
  looks per eval and 8 MiB, sheets should be the only large look in an eval. The caption reports bytes when a
  look is dropped (existing behaviour of the transport).

**Alternatives not taken.** An expression op (`cv.expr("a - gaussian(a, 3)")`); a separate "canvas" kind for
decorated images; CLAHE instead of local contrast (CLAHE's tile artefacts are the size of what is inspected).

**Open questions.**

1. Should rulers label in tile-local or absolute coordinates when a file was opened without `origin`? With
   GCV-001's `origin` option the agent can choose; the guide labels the frame's coordinates.
2. Should `sheet` accept `stretch: "shared"` (one stretch across all cells of a view)? Add if the colleague's
   agents compare brightness across cells.
3. Is a 16× maximum zoom enough? The colleague uses 8×; 16× allows 32 px windows at 512 px.

## 15. References

- Handover issues 1–5, 8, 14, 15, 22, 23: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md`; generic list sections A and B: `…/reference/06-goja-cv-generic-inspection-features.md`.
- View measurements: `…/DUST-002…/scripts/85-cv-views.py`; zoom renderer: `…/scripts/80-vlm-render.py`; helpers: `…/scripts/84-cv-helpers.js`; playbook: `…/reference/04-goja-cv-dust-playbook.md`.
- CVJS-001 intern guide Part 3 §17.3 (looks as returned values) and the integration-surfaces doc §3 (result content): `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/`.
- GCV-001 guide (frames, spaces, derived encoding, colormap, macros): `ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/design-doc/01-design-and-implementation-guide.md`.
- GCV-003 guide (region schema used by sheet labels and annotate; profile plots use this ticket's renderer): `ttmp/2026/11/09/GCV-003--measurement-and-data-out-sampling-profiles-region-measures-schema-export/design-doc/01-design-and-implementation-guide.md`.
