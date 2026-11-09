---
Title: Design and implementation guide
Ticket: GCV-001
Status: active
Topics:
    - image-model
    - io
    - color
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md
      Note: shared foundations chapter (read first)
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/03-review-read-findings.md
      Note: review findings and their disposition (revision 2)
    - Path: pkg/cv/buffer/io.go
      Note: decode always applies the sRGB curve; ToImage always encodes sRGB; replaced by a decoder registry and Quantize
    - Path: pkg/cv/buffer/color.go
      Note: transfer functions and LUTs; buffer.Convert uses the exact functions
    - Path: pkg/cv/graph/registry.go
      Note: OpDef.Space string becomes SpaceRule; Result, Gray, DocFrame, Expand, Internal
    - Path: pkg/cv/graph/node.go
      Note: internNode computes Enc, Space, Frame; Base is removed; Substitute and placeholders inherit statics
    - Path: pkg/cv/graph/args.go
      Note: space option, Level unit, box and point types, SPACE warning
    - Path: pkg/cv/graph/engine.go
      Note: Open takes encoding, box, origin; Placeholder copies statics; Call returns CallResult
    - Path: pkg/cv/graph/explain.go
      Note: SourceInfo reports encoding; Describe omits the default space
    - Path: pkg/cv/kernels/mask.go
      Note: PerceptualLuminance, ColorMask, FloodMask convert privately
    - Path: pkg/cv/kernels/tone.go
      Note: Levels and Contrast convert privately
    - Path: pkg/cv/kernels/measure.go
      Note: Histogram bins perceptual values privately
    - Path: pkg/cv/kernels/specks.go
      Note: DetectSpecks mixes perceptual detection and linear MeanL
    - Path: pkg/cv/look/look.go
      Note: Render renders encoded values after this ticket
    - Path: pkg/cvsession/init.js
      Note: cv.open wrapper gains as; img becomes one entry of a name table
    - Path: pkg/cvsession/processor.go
      Note: context block lists names, encodings and unnamed opens
    - Path: internal/testimage/planted.go
      Note: the planted scan; source of the new fixtures
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/82-export-cv-tiles.py
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/dustds/rawio.py
Summary: "Intern guide for GCV-001: what encodings and working spaces are, why goja-cv's hard-wired sRGB model fails on scientific data, and how to replace it with tagged encodings, one build-time space option with convert nodes, linear/float/npy/DNG decoders, frames, channel ops, macros and named images."
LastUpdated: 2026-11-09T21:00:00-05:00
WhatFor: "Implementing GCV-001 (the first of GCV-001..003)."
WhenToUse: "Read before touching buffer IO, op spaces, the open op, channels or session image bindings."
---

# GCV-001 — Image model: encodings, working spaces, channels and named images

## Revision history

| date | revision | what changed |
|---|---|---|
| 2026-11-09 | 1 | First version (commit 7b3bb16). |
| 2026-11-09 | 2 | Revised from the review read (`reference/03-review-read-findings.md`, disposition table at its end). Foundations moved to a shared chapter (`reference/05-foundations-goja-cv-for-the-gcv-guides.md`). `perceptual` dropped: two spaces. Luminance is computed from linear light (decision record). Node statics are computed in `internNode`; placeholders inherit them. Phases reordered so every phase ends green, with a golden-look harness in Phase 0. TIFF primer and one TIFF decoder with its own decompression. `box`/`point` argument types, macro nodes, saved-file metadata (`Info.Origin`, PNG `tEXt`), and five new decision records. |
| 2026-11-09 | 3 | Revised from the re-check (section "Re-check (revision 2)" of the findings, rows N1–N5 in the disposition). Golden-look tolerance and regeneration rule; a test-only source op so Phase 2 and 3 tests have linear and non-zero-origin inputs; the grey-input rule covers every luminance input (`refine`); `detect.specks`' `meanL` listed as a change; the evaluator stamps copies, never another node's value; macro nodes take their expansion's statics. |

## How to read this guide

This guide is for an intern who will implement GCV-001 in goja-cv (`/Users/manuel/code/wesen/go-go-golems/goja-cv`).
Read the shared foundations chapter first:
`ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md`.
It defines nodes, kinds, build time and evaluation, the binder and lifting, the pixel and scale vocabulary, the
session, looks and result content, diagnostics and the test infrastructure. This guide cites it as "Foundations
§Fn" and does not repeat it.

GCV-001 is the first of three tickets. GCV-002 (inspection views and looks) and GCV-003 (measurement and data
out) use what this ticket defines: encodings, the two working spaces and the `space` option, the `Level` unit,
frames, the `box`/`point` argument types, derived results, macros and named images. Implement GCV-001 first.

## 1. Executive summary

goja-cv decodes every file as if it held sRGB-encoded light and converts it to linear light at load time; each
kernel then converts back to a perceptual scale privately when it needs one. That model is correct for
photographs and wrong for linear scans, float data, density maps, depth maps and `.npy` arrays. The colleague who
uses goja-cv to prepare film-density tiles for vision-model subagents stores `sRGB_encode(density)` in 16-bit
TIFFs only so that goja-cv's decode undoes the curve.

This ticket replaces the hard-wired model with three ideas:

1. An **encoding** is a property of image data: the transfer function between the values a file stores (and a
   look shows) and values proportional to the measured quantity. goja-cv knows two, `srgb` and `linear`.
   Decoders report the encoding of what they read, and `cv.open(path, { encoding })` overrides it.
2. A **working space** is where an op does its arithmetic: `linear` or `encoded`. Every op that reads pixel
   values gets one `space` option, resolved when the node is built. The builder inserts explicit, cached
   `convert` nodes where an input is in the other space. Kernels never convert.
3. A **frame** places an image in document coordinates, so a window read from a large scan keeps the scan's
   coordinates; and a session holds several opened images under **names** (`cv.open(path, { as: "ref" })`), all
   listed in the context block.

Along the way the ticket adds decoders for linear and float TIFF, `.npy` and VueScan's linear DNG, channel ops
(`channel`, `split`, `toLab`, `chroma`, `colormap`, `encode`), `box` and `point` argument types, and a macro
mechanism (`OpDef.Expand`) that GCV-002 uses for its views. It removes the per-kernel conversions, the
documentation-only `Space` string on `OpDef`, the assumed-sRGB metadata, `DocW`/`DocH` pairs, and dead fields.

## 2. Problem statement and scope

### 2.1 The workflow this serves

The colleague's project (DUST-002, in the dust-scanning repository) annotates dust, hairs and scratches on film
scans with vision-language model (VLM) subagents. Each subagent gets one 1024 × 1024 px tile. Through goja-cv's
MCP tools it builds enhanced views in which faint specks stand out, lists candidate spots, and judges magnified
crops of them by eye. The agent playbook is `reference/04-goja-cv-dust-playbook.md` and the handover issues are
`reference/07-goja-cv-handover-issues.md`, both under
`/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/`.

The tiles are cut from VueScan DNG scans of colour negatives. The quantity that matters is film **density**,
D = −log10(I / I_base), where I is the light the scanner measured through the film and I_base the light
measured through unexposed film base. The export script normalizes D per tile to v ∈ [0, 1]. Dust blocks
light, so it raises density, and a speck of a given opacity adds the same density step on a thin and on a dense
part of the film. The export (`scripts/82-export-cv-tiles.py`, lines 55–59; the comment is added here) shows the
trick the colleague needed:

```python
v = np.clip((D - lo) / rng, 0, 1)
if not a.perceptual:
    v = np.where(v <= 0.0031308, 12.92 * v, 1.055 * np.power(v, 1 / 2.4) - 0.055)   # this stores sRGB_encode(v)
p = OUT / f"{t['tile_id']}.tif"
tifffile.imwrite(p, (v * 65535 + 0.5).astype(np.uint16), photometric="rgb")
```

They store sRGB_encode(v) so that goja-cv's decode, which always applies the inverse sRGB curve, recovers v as
the "linear" value on which blur, blend and stats run.

### 2.2 The issues

| issue | title | priority | in this ticket |
|---|---|---|---|
| 6 | Open linear and scientific inputs | P1 | encodings, decoders, `cv.open(path, { encoding })`, encoding in the context block |
| 7 | Choose the working space for filters and stats | P2 | the `space` option; `explain()` shows it |
| 13 | Channels, colour spaces and false colour | P2 | `channel`, `split`, `toLab`, `chroma`, `colormap` |
| 20 | Several named images per session | P3 | `cv.open(path, { as })`, names in the context block |
| 10 | Measure regions against another image | P2 | context only: its neutrality cue (chroma ≈ 0.2 for dust) is implemented in GCV-003 |
| 22 | Named view presets | P3 | context only: presets are GCV-002 macros built on §7.8 |

### 2.3 Scope

In scope: the image model (encodings, spaces, frames), decoders for the listed formats, the `space` option on
every pixel-reading op, the channel ops, `box`/`point` argument types, named images, the macro mechanism, and the
documentation an agent reads. Out of scope: ICC colour management (embedded profiles are ignored), camera-raw
demosaicing (only DNGs whose data is already `LinearRaw` are read), BigTIFF, writing files (GCV-003), crops and
zoomed looks (GCV-002), signed arithmetic (GCV-002).

## 3. Concepts

This section builds the vocabulary in dependency order, with numbers you can check with a calculator. It
assumes Foundations §F2–§F5.

### 3.1 Code values, quantities and "linear"

A 16-bit image file stores integers between 0 and 65535 per channel. These integers are **code values**. A code
value does not say what it measures; a file format, a tag, or a convention supplies that meaning. goja-cv
normalizes code values to [0, 1] by dividing by the largest code (255 or 65535); float files store their values
directly. In this guide an **encoded value** is that normalized code value.

The **quantity** is what the data measures: light for a photograph, film density for the colleague's tiles,
depth for a depth map. A stored number is **linear in a quantity Q** when it is proportional to Q: zero means
none, and doubling the number doubles Q. **Linear light** means linear in the amount of light (photon count). The
two notions differ for density. Density is logarithmic in light: a density of 0.3 transmits 10^−0.3 = 50 % of the
base light and a density of 0.6 transmits 25 %. So a tile that stores v linear in density is not linear in light,
and the right numbers for averaging specks are density numbers.

goja-cv cannot know the quantity. In goja-cv, **linear** always means "the values after the declared encoding is
undone". The user chooses the encoding under which those values are the quantity they care about: `linear` for
the density tiles (v is density), `srgb` for a photograph (undoing the curve gives light).

### 3.2 Encodings and transfer functions

An **encoding** is the function that maps an encoded value e to a linear value x. goja-cv needs two:

- `srgb`: the sRGB transfer function. x = e / 12.92 for e ≤ 0.04045, else x = ((e + 0.055) / 1.055)^2.4. Its
  inverse, used to encode, is e = 12.92 · x for x ≤ 0.0031308, else e = 1.055 · x^(1/2.4) − 0.055.
  `pkg/cv/buffer/color.go:29-50` implements both, extended to all reals (odd-symmetric below zero).
- `linear`: the identity, x = e.

Photographs (PNG, JPEG, ordinary 8/16-bit TIFF) are almost always sRGB-encoded, because the sRGB curve spends
code values where vision distinguishes small differences, in the shadows. Scanner raw data, float TIFFs and NumPy
arrays are almost always linear.

| encoded e | linear x (srgb) | linear x (linear) |
|---|---|---|
| 0.05 | 0.0039 | 0.05 |
| 0.5 | 0.214 | 0.5 |
| 0.8 | 0.604 | 0.8 |

The same code value 32768 (e = 0.5) means 21 % of white in an sRGB file and 50 % in a linear file. A reader
that guesses wrong is off by a factor of 2.3 in the midtones.

### 3.3 Two working spaces, and levels

A kernel computes on numbers. The **working space** of an op is the representation those numbers are in when the
kernel sees them. goja-cv uses two:

| space | definition | used by default for |
|---|---|---|
| `linear` | x: the encoding undone, proportional to the quantity | averaging, blurring, scaling, compositing; photometric measurement |
| `encoded` | e: the normalized code values, as the file stores them and as a look shows them | thresholds and contrast judged by eye; detection of what is visible |

A **level** is a number on the value scale of a space: a black point, a threshold, a percentile. Whether 0.5 is a
dark or a mid level depends on the space it is read in. (Levels in this sense are unrelated to the `levels` op,
except that the op's `black` and `white` arguments are levels.)

For a `linear`-encoded image the two spaces coincide: opening the colleague's density tiles with `encoding:
"linear"` makes every op, whatever its space, see v itself. For an `srgb` image the spaces differ by the sRGB
curve.

| file | stored code | encoded e | linear x |
|---|---|---|---|
| sRGB photo (16-bit TIFF) | 32768 | 0.5 | 0.214 |
| linear scan (16-bit TIFF, `encoding: "linear"`) | 32768 | 0.5 | 0.5 |
| today's density tile (stores sRGB_encode(v)), v = 0.3 | 38261 | 0.584 | 0.300 |
| density tile stored as v, opened `linear`, v = 0.3 | 19661 | 0.300 | 0.300 |

The last two rows are the same density. With this ticket the colleague stores v directly and opens with
`encoding: "linear"`.

goja-cv's code and docs also speak of a **perceptual** scale: the sRGB curve applied to linear light, a scale on
which equal steps look roughly equally different. For sRGB images it is the encoded space. For linear-encoded
light it is what `img.encode("srgb")` produces (§7.7). It is not a third space; §8 records why.

### 3.4 Which space an op should use

Spaces change results. The colleague measured that their speck ranking in a high-pass view got two to three
times worse when the filter ran in the wrong space (issue 7). Two numbers show why.

Take a dust speck that adds a density step Δv = 0.05, on a thin part of the film (v = 0.10) and on a dense part
(v = 0.60). In density units the step is the same. Read through the sRGB curve in either direction, it is not:

```
encode (v stored, viewed through sRGB_encode):
  dark:  sRGB_encode(0.15) − sRGB_encode(0.10) = 0.4236 − 0.3492 = 0.0744
  light: sRGB_encode(0.65) − sRGB_encode(0.60) = 0.8267 − 0.7977 = 0.0289      2.6× stronger on dark
decode (v stored as code, decoded to "linear light" by today's goja-cv):
  dark:  sRGB_decode(0.15) − sRGB_decode(0.10) = 0.0196 − 0.0100 = 0.0096
  light: sRGB_decode(0.65) − sRGB_decode(0.60) = 0.3800 − 0.3186 = 0.0615      6.4× stronger on light
```

The decode direction is the colleague's measured problem: v stored as a plain code value and decoded as sRGB made
grain louder in bright areas.

The rule has two parts, because ops answer two kinds of question:

- If equal **physical** steps must count equally (averaging light, scaling exposure, measuring how much
  material there is), use the space in which the quantity is linear: `linear`.
- If equal **visible** steps must count equally (a threshold chosen by looking at a picture, contrast around the
  displayed mid-grey, "is this speck visible"), use the space the look shows: `encoded`.

For linear-encoded data the two coincide, which is why the colleague's case is solved by the encoding alone. For
sRGB photographs they differ, and the defaults table in §7.5 labels each op with the question it answers.

### 3.5 Luminance is computed from light

Several ops reduce a colour image to one grey plane before they work: `mask.luminance`, `refine`,
`detect.specks`, `check.dust`, and `histogram` with channel `"L"`. Today they all compute **luminance** the
photometric way: Y = 0.2126 R + 0.7152 G + 0.0722 B on linear channels, then (for the perceptual ops) the sRGB
curve of Y (`PerceptualLuminance`, `pkg/cv/kernels/mask.go:41-52`).

Computing the same weighted sum on encoded channels gives a different number, called **luma**:

| colour (encoded R, G, B) | sRGB_encode(Y of linear channels) | luma of encoded channels |
|---|---|---|
| pure red (1, 0, 0) | 0.498 | 0.213 |
| orange (0.8, 0.4, 0.2) | 0.512 | 0.471 |

So "the same op in encoded space" is not automatically today's behaviour for colour images. This guide keeps
luminance photometric: a luminance op computes Y from linear channels and then expresses Y in its working space.
The graph does this, not the kernel (§7.5, "Grey input"), so kernels stay space-agnostic and today's numbers on
sRGB images are preserved exactly.

### 3.6 Light, derived values and masks

Some ops do not transform light; they compute a new quantity from it: the difference of two images, a high-pass
residual, a local contrast ratio, a chroma value, Lab coordinates. Pushing such a value through a transfer
function is meaningless: sRGB_decode of a difference of 0.05 encoded units is 0.0039, a number that describes
nothing.

This guide calls such outputs **derived**. A derived image is tagged with the `linear` encoding: its stored value
is its value, so linear, encoded and the displayed grey level coincide. For derived images, what you measure is
what you see.

**Masks** have no encoding at all (`Enc` is unset and their space is `none`). They are weights, never converted,
and a look shows a mask value directly: 0.5 is grey 128. Today a mask of 0.5 is shown through the sRGB curve as
188 (`pkg/cv/buffer/io.go:182-190`).

### 3.7 Frames and absolute coordinates

A **frame** (Foundations §F5) is the rectangle an image covers in doc px: an integer origin (x, y) and an integer
size (w, h). Today every frame has origin (0, 0); goja-cv stores only the size (`DocW`, `DocH`).

The rule this ticket introduces: **all doc-px arguments and results are absolute**. They are in the coordinates of
the file the image came from, or in the coordinates declared by `origin` when the file is itself a window.
Reading `cv.open("raw0027.dng", { box: [1536, 1536, 1024, 1024] })` gives an image whose frame is
(1536, 1536, 1024, 1024); a speck found at raw (1700, 1600) is reported at (1700, 1600), and
`img.stats({ box: [1600, 1600, 50, 50] })` measures inside it. A box or point outside the frame is
`E_ARG_RANGE` with the frame in the message and the nearest valid box as the fix; `img.stats({ box: [0, 0, 100,
100] })` on that image is such an error. Origins are integers because they come from file pixel positions.

### 3.8 Channels, chroma and colormaps

A colour image has three channels. Several inspection cues live in one channel or in the relation between
channels: dust on colour film is neutral (an equal density step in all three channels), colour grain and coloured
scene detail are not.

In goja-cv, **chroma** of a pixel means the spread of its channels: max(R, G, B) − min(R, G, B), optionally
divided by the channel mean. It is not CIE chroma C*, which is computed in Lab. The colleague's neutrality cue
is a different, related number: the spread of a spot's per-channel *contrast steps* relative to the mean step
(≈ 0.2 for confirmed dust, ≈ 0.8 for rejects, issue 10). GCV-003 implements that cue as the region field
`stepChroma`; this ticket's `chroma()` is the per-pixel map.

A **colormap** maps a single-channel value to a display colour. A diverging map (blue–white–red) shows the sign
of a signed map at a glance, which grey cannot.

### 3.9 Named images

A session can hold several opened images. A **name** is a global the agent uses for one of them: `img` is the
first, `ref` could be a cleaned version or another scan of the same frame. Ops already accept nodes built from
different files when their frames match; names make the images reachable across evals and visible in the
context block.

The key points:

- An encoding belongs to the data; a working space belongs to the op. Confusing them is the bug this ticket
  removes.
- `linear` means "the declared encoding undone"; the user picks the encoding under which that is the quantity.
- Physical questions use `linear`, visible questions use `encoded`; for linear-encoded data they coincide.
- Luminance is computed from linear light and then expressed in the op's space.
- Derived values carry the linear encoding; masks carry none; both display their values directly.
- Coordinates are absolute in the frame of the file they came from.

## 4. Current-state analysis

Every claim is anchored to the code at commit `175b5d4`; `pkg/`, `internal/` and `cmd/` are unchanged since.

### 4.1 Decoding always applies the sRGB curve

`buffer.Decode` (`pkg/cv/buffer/io.go:67-133`) switches on the Go image type and converts every sample through
`Linear8` or `Linear16` (`io.go:89`, `:101`, `:124`, `:144`, `:160-162`), the sRGB decode tables of
`color.go:5-27`. There is no branch for linear data. `DecodeConfigFile` (`io.go:30-51`) reads headers through
`image.DecodeConfig`, which knows PNG, JPEG, and the TIFF variants `golang.org/x/image/tiff` supports. That package
rejects float samples (`SampleFormat` ≠ 1, x/image v0.47.0 `tiff/reader.go:252-262`) and has no case for DNG's
`LinearRaw` photometric interpretation (34892).

`Engine.Open` (`pkg/cv/graph/engine.go:87-120`) probes the header and stamps format, bit depth and channel count
into hidden arguments of the `open` node; its error hint says "Supported formats: PNG, JPEG, TIFF (8/16-bit)"
(`engine.go:113-114`). The `open` op's doc string reads "decoded on first evaluation, sRGB → linear float32"
(`pkg/cv/ops/source.go:34`). `SourceInfo` reports `"colorSpace": "sRGB (assumed; ICC profiles are not read)"`
for every file (`pkg/cv/graph/explain.go:21`).

### 4.2 The op `Space` field is documentation only

`OpDef.Space` is a string with the documented values `"linear" | "perceptual" | "geometry" | "analysis" |
"source" | "data"` (`pkg/cv/graph/registry.go:106`); the ops also use `"selection"` and `"present"`. The only
reader is the help renderer (`pkg/cv/graph/help.go:77-79`); `pkg/cv/graph/eval.go` never mentions it. Across
`pkg/cv/ops` the values occur 7 (linear), 5 (perceptual), 3 (geometry), 7 (analysis), 11 (data), 5 (selection), 4
(present) and 3 (source) times.

### 4.3 Kernels convert privately

| kernel | file:lines | what it converts |
|---|---|---|
| `Levels` | `pkg/cv/kernels/tone.go:39-54` | black/white and every sample to perceptual, result back to linear |
| `Contrast` | `tone.go:59-68` | every sample to perceptual and back |
| `PerceptualLuminance` | `pkg/cv/kernels/mask.go:41-52` | sRGB_encode of linear Rec. 709 luminance (`mask.luminance`, `refine`, `detect.specks`) |
| `ColorMask` | `mask.go:124-145` (line 134) | each channel to perceptual before computing hue |
| `FloodMask` | `mask.go:251-270` (lines 262, 266) | seed and every visited pixel to perceptual |
| `Histogram` | `pkg/cv/kernels/measure.go:194-231` (line 216) | bins perceptual values; channel `"L"` is linear luminance first |
| `AbsDiff` | `pkg/cv/kernels/draw.go:100-121` | \|P(a) − P(b)\| · gain, back to linear |
| `DetectSpecks` | `pkg/cv/kernels/specks.go:166-178` | detects on perceptual luminance, reports `MeanL` from linear luminance (line 175) |
| `parseColor` | `pkg/cv/ops/mask.go:69-106` (line 105) | overlay colours from perceptual to linear |

The histogram op also hard-codes `"space": "perceptual"` into its result (`pkg/cv/ops/measure.go:158`).

### 4.4 Display and files assume sRGB

`buffer.ToImage` (`io.go:169-224`) clamps linear values and encodes them with `EncodeSRGB8`/`EncodeSRGB16` for every
buffer, masks included. `look.Render` (`pkg/cv/look/look.go:180-209`) calls it for every look, and `WriteFile`
(`io.go:254-294`) writes 16-bit sRGB PNGs and TIFFs the same way.

### 4.5 One bound image

The session defines `img` as a getter/setter over one slot (`pkg/cvsession/init.js:47-56`,
`pkg/cvsession/native.go:120-132`). `cv.open` returns a node and records it in the binding's `opened` list
(`pkg/gojamod/cv/binding.go:259-271`, `pkg/gojamod/cv/evaltrack.go:197-206`); `img = cv.open(p)` rebinds `img`.
The context block lists every opened file and marks the one bound to `img` (`pkg/cvsession/processor.go:482-498`).
A second image survives across evals today as `mem.ref = cv.open(p)` (listed under `mem`, not under images) or
as a sloppy-mode global (not listed at all). The MCP `open` tool takes only a path (`pkg/cvsession/mcp.go:47`).

### 4.6 Channels: only toGray

The only channel op is `toGray(weights)` (`pkg/cv/ops/tone.go:76-92`), with weights `rec709`, `rec601` and
`average` applied to linear values (`pkg/cv/kernels/tone.go:70-90`). Rec. 601 weights are defined for
gamma-encoded luma, so `toGray("rec601")` on linear values mixes conventions.

### 4.7 Dead and duplicated pieces in the core

- `Node.Base` (`pkg/cv/graph/node.go:21`, set at `:127-129`) exists for an `E_STALE` check of a document layer that
  does not exist; nothing reads it.
- The codes `EStale`, `EReadOnly`, `EFragmentCapture`, `ENoModel` and `WScale` (`pkg/cv/graph/errors.go:21-35`)
  are never used outside their declaration.
- `DocW`/`DocH` are duplicated on nodes and on evaluated values (`pkg/cv/graph/value.go:15-36`): every `Run`
  threads them through `NewImage(buf, docW, docH)` although the node knows them statically.
- `OpDef.Halo` (`registry.go:108`) is declared and never read; GCV-002 gives it a reader.

### 4.8 What the workaround costs

The colleague exported 300 MB of TIFFs for 50 tiles, cannot open the DNGs or their `.npy` caches, and has to
remember which goja-cv numbers are density and which are sRGB_encode(density). Issue 7 reports that their probe
helper once had to re-apply the sRGB curve in JavaScript to get stats in file units.

## 5. Gap analysis

| need | today | gap |
|---|---|---|
| Read linear, float, `.npy`, LinearRaw DNG data | PNG/JPEG/TIFF integers, always sRGB | decoders and an encoding per source |
| Say which encoding an image has | `colorSpace: sRGB (assumed)` | encoding in metadata, explain and the context block |
| Run any pixel op in a chosen space | fixed per kernel | one `space` option resolved at build time |
| Know which space a number is in | not recorded | static `Space` on nodes, shown by `explain()` |
| Neutral and colour cues, false colour | `toGray` only | `channel`, `split`, `toLab`, `chroma`, `colormap` |
| Read a window of a large scan in scan coordinates | not possible | frames, `open({ box })` |
| Two images in a session, visible to the agent | one `img` slot | named images |

## 6. Legacy to remove or refactor

The project is early-stage and local-only: no compatibility shims and no deprecated aliases. This ticket removes
or rewrites the following.

1. Every `buffer.ToPerceptual` and `buffer.ToLinear` call in `pkg/cv/kernels` and `pkg/cv/ops` (table in §4.3) is
   removed, and a test parses the sources so the calls cannot come back.
2. `OpDef.Space string` is replaced by a typed `SpaceRule`; the category values (`geometry`, `analysis`, `data`,
   `selection`, `present`, `source`) are dropped because help already groups ops by receiver kind.
3. The sRGB decode in `buffer.Decode` and the sRGB encode in `buffer.ToImage` are removed; decoders return encoded
   values, and `ToImage` becomes `Quantize`, which only scales and clamps encoded values.
4. `DecodeConfigFile` through `image.DecodeConfig` is replaced by `Probe` on a decoder registry.
5. The `colorSpace` metadata field and the `metadata`/`meta`/`info` help redirects that mention it
   (`pkg/gojamod/cv/help.go:52-54`) are replaced by `encoding`.
6. `DocW`/`DocH` on nodes and values, and `OpDef.DocSize`, are replaced by `Frame` and `OpDef.DocFrame`; values get
   their frame stamped by the evaluator instead of from every `Run`.
7. The single `img` slot (`getImg`/`setImg`) and the binding's `opened` list as the source of the context block are
   replaced by a session name table; `img` is its first entry.
8. The `"space": "perceptual"` literal in histogram results, and the word "perceptual" in docs, help strings and the
   MCP instructions (`pkg/cvsession/reference.go:28-29`), are replaced by the resolved space.
9. `Node.Base` and the unused codes `EStale`, `EReadOnly`, `EFragmentCapture`, `ENoModel` and `WScale` are deleted.
10. `toGray("rec601")` is deleted; `toGray` keeps `rec709` (linear light) and `average` (any space).
11. The hand-written `cv.kinds` list (`pkg/gojamod/cv/prelude.js:307`) is replaced by the list exported from
    `graph.Kinds`.
12. `ToPerceptual`/`ToLinear` interpolated tables stay only as a documented fast path; `Convert` uses the exact
    transfer functions.

## 7. Proposed architecture

### 7.1 Overview

```
 file ──▶ decoder registry ──▶ encoded float32 + Info{encoding, format, bits, sample, origin}
                                   │ source loader: encoded → linear (exact 8/16-bit tables), pyramid of linear values
                                   ▼
 open node  (Enc = srgb | linear, Space = linear, Frame = {x, y, w, h})
   │  agent calls img.mask.luminance({ above: 0.9 })
   ▼
 Builder.Intern: bind args → resolve space (option or op default) → grey input? → inputs in another space?
   │                                                     │ yes: intern toGray / convert nodes (cached, shared)
   ▼                                                     ▼
 internNode computes Enc, Space, Frame from (op, args, inputs)   ◀── the only place statics are set
   │
   ▼
 evaluator runs kernels on already-converted buffers; kernels never convert; values get statics stamped
   │
   ▼
 look: convert(root → encoded), Quantize to 8 bits, encode PNG/JPEG
```

Three rules carry the design:

- Encoding, space and frame are **static**: known when the node is built, computed from the node's own op, args
  and inputs, never modified afterwards.
- Conversions are **nodes**. They are hash-consed and cached like every other node, so two ops that need the
  encoded version of one input share one conversion, and `explain()` shows each conversion.
- Kernels are **space-agnostic**. A kernel computes on the numbers it receives.

### 7.2 Types

```go
// pkg/cv/buffer/encoding.go
package buffer

// Encoding is the transfer function between encoded values (what a file
// stores and a look shows) and linear values.
type Encoding uint8

const (
    EncNone   Encoding = iota // masks and non-image values
    EncSRGB                   // sRGB curve
    EncLinear                 // identity
)

func (e Encoding) String() string            // "", "srgb", "linear"
func ParseEncoding(s string) (Encoding, bool)

// Space is the representation a set of values is in.
type Space uint8

const (
    SpaceNone    Space = iota // not levels: masks, counts, geometry
    SpaceLinear               // x: the encoding undone
    SpaceEncoded              // e: normalized code values, as stored and displayed
)

func (s Space) String() string               // "", "linear", "encoded"
func ParseSpace(s string) (Space, bool)

// Identity reports whether converting from → to is the identity for e:
// always for from == to, and for every pair when e is linear.
func Identity(e Encoding, from, to Space) bool

// Convert returns b's values, which are in space from, in space to, for
// encoding e, using the exact transfer functions. It returns b itself when
// Identity holds.
func Convert(ctx context.Context, b *Buffer, e Encoding, from, to Space) (*Buffer, error)
```

The conversion table has two non-trivial cells:

| from → to | `srgb` | `linear` |
|---|---|---|
| linear → encoded | sRGB encode | identity |
| encoded → linear | sRGB decode | identity |

`Convert` uses the exact `SRGBToLinear`/`LinearToSRGB` functions (`color.go:29-50`), so a round trip is accurate
to float32 precision. The interpolated tables behind `ToPerceptual`/`ToLinear` (error up to 2e-5,
`color.go:58-60`) remain only as an optional fast path with that documented error. The exact 8/16-bit decode
tables (`Linear8`, `Linear16`) stay for the source loader.

`Buffer` does not gain a space field. The graph is the authority: when `Identity` holds, one cached buffer serves
both spaces, so a tag on the buffer would be wrong for one of them.

```go
// pkg/cv/graph: frames replace DocW/DocH pairs
type Frame struct {
    X, Y int // origin in doc px (file pixel positions)
    W, H int // size in doc px
}

type Node struct {
    ID        NodeID
    Op        *OpDef
    Inputs    []NodeID
    NExplicit int
    Args      Args
    Kind      Kind
    Hash      [32]byte
    Frame     Frame           // was DocW, DocH; Base is removed
    Enc       buffer.Encoding // Image: its encoding; Value: the encoding of the image it measured; else EncNone
    Space     buffer.Space    // Image: space of its values; Value: space of its level-valued numbers; else SpaceNone
}
```

Evaluated values carry the same statics so that `Run` functions can read their inputs' frames and encodings, but
`Run` never sets them: it returns `graph.NewImage(buf)` (or `NewMask(buf)`), and the evaluator stamps `Frame` and
`Enc` from the node after `Run` returns. `RegionsValue` swaps `DocW, DocH` for `Frame` the same way.

The evaluator stamps only values that `Run` created. Several paths return a value that belongs to another node: the
`Identity` short-circuit returns its receiver's value (`pkg/cv/graph/eval.go:291-293`), `toGray` returns its input for
a 1-channel image (`pkg/cv/ops/tone.go:81-83`), and a macro node returns its expansion's value. When the returned
value is one of the inputs' values, the evaluator stamps a shallow copy (a new `ImageValue` sharing the same `Buf`),
so no cached value of another node is ever rewritten.

### 7.3 Decoders

#### A TIFF primer

Most of the work in this ticket's IO is reading TIFF files, including DNG, which is a TIFF. A TIFF file is a
header followed by **IFDs** (image file directories), each a list of tagged entries describing one image. The
first 32 bytes of the colleague's tile `1d955ef4b27d3d88_1536_1536.tif`:

```
00000000: 4949 2a00 0800 0000 0f00 0001 0400 0100  II*.............
00000010: 0000 0004 0000 0101 0400 0100 0000 0004  ................
```

- `49 49` (`II`) means little-endian (`MM` would be big-endian); `2a 00` is 42, the TIFF magic number (43 would be
  BigTIFF, out of scope); `08 00 00 00` is the offset of the first IFD, 8.
- At offset 8, `0f 00` says the IFD has 15 entries. Each entry is 12 bytes: tag (2), type (2), count (4), value or
  offset (4). The first entry is `00 01 | 04 00 | 01 00 00 00 | 00 04 00 00`: tag 256 (ImageWidth), type 4 (LONG,
  4 bytes), count 1, value 1024. The next is tag 257 (ImageLength), 1024. A value that does not fit in 4 bytes is
  stored elsewhere and the entry holds its offset. Type sizes: BYTE 1, ASCII 1, SHORT 2, LONG 4, RATIONAL 8,
  FLOAT 4, DOUBLE 8.
- After the entries come 4 bytes with the offset of the next IFD (0 = none).

The tags the decoder needs:

| tag | name | used for |
|---|---|---|
| 254 | NewSubfileType | 0 = full-resolution image (DNG thumbnails have 1) |
| 256, 257 | ImageWidth, ImageLength | size |
| 258 | BitsPerSample | 8, 16 or 32 per sample |
| 259 | Compression | 1 none, 5 LZW, 8 / 32946 Deflate, 32773 PackBits |
| 262 | PhotometricInterpretation | 1 BlackIsZero, 2 RGB, 34892 LinearRaw (32803 CFA is refused) |
| 270 | ImageDescription | goja-cv's own metadata (below) |
| 273, 279 | StripOffsets, StripByteCounts | where each strip of rows lives |
| 277 | SamplesPerPixel | 1 or 3 (4 with ExtraSamples: alpha, dropped) |
| 278 | RowsPerStrip | rows per strip |
| 284 | PlanarConfiguration | 1 chunky (supported), 2 planar (refused) |
| 317 | Predictor | 2 horizontal differencing (supported with LZW/Deflate) |
| 330 | SubIFDs | offsets of child IFDs (DNG keeps the raw image there) |
| 339 | SampleFormat | 1 unsigned integer, 3 IEEE float |
| 50714, 50717, 50712 | BlackLevel, WhiteLevel, LinearizationTable | DNG value mapping |

Tiles (`TileOffsets`) are refused with `E_FORMAT` naming the feature; none of the target files use them.

#### The registry

```go
// pkg/cv/buffer/decode.go
type Info struct {
    Format   string          // "png", "jpeg", "tiff", "dng", "npy"
    W, H     int
    Channels int             // 1 or 3 after decoding (alpha dropped)
    Bits     int             // 8, 16 or 32 per sample in the file
    Sample   string          // "uint" | "float"
    Encoding Encoding        // what the file implies (overridable by cv.open)
    Origin   *[2]int         // frame origin recorded by goja-cv (TIFF description, PNG tEXt); nil otherwise
    Notes    []string        // e.g. "camera RGB primaries (DNG)", "LZW"
}

type DecodeOptions struct {
    Box *image.Rectangle // read only this window (file px); nil = whole image
}

type Decoder interface {
    Name() string
    Sniff(head []byte) bool                                                              // magic bytes
    Probe(r io.ReadSeeker) (Info, error)                                                 // header only
    Decode(ctx context.Context, r io.ReadSeeker, o DecodeOptions) (*Buffer, Info, error) // encoded values
}

func Register(d Decoder)
func ProbeFile(path string) (Info, error)
func DecodeFile(ctx context.Context, path string, o DecodeOptions) (*Buffer, Info, error)
```

Decoders return **encoded** values: integers divided by their maximum, floats as stored. The source loader
(`graph.SourceStore.Load`, `pkg/cv/graph/sources.go:34-61`) converts to linear once, with the exact 8/16-bit tables
for integer files, and builds the preview pyramid from linear values (area averaging of light, as today). The
source node's space is therefore `linear`.

| decoder | formats | encoding implied | notes |
|---|---|---|---|
| `png` | 8/16-bit gray, RGB, palette | `srgb`; `linear` when a `gAMA` chunk says 1.0 (100000) | pixels via stdlib `image/png`; a small chunk scan reads `gAMA`, `sRGB` and `tEXt` (stdlib ignores them) |
| `jpeg` | baseline, progressive | `srgb` | stdlib; EXIF orientation not applied (as today) |
| `tiff` | own parser for everything in the primer | `srgb` for uint Gray/RGB, `linear` for float and LinearRaw | decompression in the same parser: LZW via `golang.org/x/image/tiff/lzw` (an exported package), Deflate via `compress/zlib`, PackBits in about 20 lines; `Box` reads only the strips that intersect the window |
| `npy` | NumPy format v1–v3, C order; `\|u1`, `<u2`, `>u2`, `<f4`, `>f4`, `<f8`; shapes (H, W), (H, W, 1), (H, W, 3) | `linear` | f8 narrowed to float32; Fortran order is `E_FORMAT`; `Box` reads rows with seeks |

There is no fallback to `x/image/tiff`: one TIFF code path handles compressed and uncompressed, integer and float,
whole and windowed reads.

**DNG.** The VueScan DNGs are described by the dataset code (`dustds/rawio.py:1-46` and
`~/dustds/meta/dng_meta.jsonl`): the full-resolution image is the IFD with `NewSubfileType` = 0 and
`PhotometricInterpretation` = 34892, top-level or inside the `SubIFDs` of IFD0; 16 bits, 3 samples, uncompressed,
contiguous strips; no `BlackLevel`, `WhiteLevel` or `LinearizationTable`, so the decoded value is the stored value
/ 65535. Example from the metadata: `raw0027.dng`, 7256 × 7326, data offset 8, VueScan 9.8.39, Perfection V600.
The decoder applies `BlackLevel`/`WhiteLevel` when present ((v − black) / (white − black)), refuses
`LinearizationTable` and CFA data with `E_FORMAT`, and adds the note "camera RGB primaries (DNG)" (colour matrices
are not applied).

**Saved-file metadata.** GCV-003's `save` records the encoding and frame origin of what it writes: TIFFs carry the
ASCII string `goja-cv encoding=linear origin=1536,1536` in `ImageDescription` (270); PNGs carry a `gAMA` chunk
(1.0 for linear, absent for sRGB with an `sRGB` chunk instead) and a `tEXt` chunk with keyword `goja-cv` and the
same string. The decoders parse that string into `Info.Encoding` and `Info.Origin`. Other descriptions are ignored.
`.npy` files cannot carry metadata: they open as `linear` with origin (0, 0).

A full VueScan DNG decodes to 7256 × 7326 × 3 float32 = 638 MB plus about a third more for the pyramid. A 1024²
window read with `Box` costs 6 MB of reads.

### 7.4 Opening: `cv.open(path, options)`

```js
img = cv.open("/Users/manuel/dustdet/cv_tiles/1d955ef4b27d3d88_1536_1536.tif")          // encoding from the file: srgb
cv.open("/data/tile_v.tif", { encoding: "linear", origin: [1536, 1536], as: "v" })       // stored v, raw coordinates
cv.open("/data/raw0027.dng", { as: "scan", box: [1536, 1536, 1024, 1024] })               // linear, frame (1536, 1536, 1024, 1024)
const d = cv.open("/data/tile.npy")                                                       // linear (npy default), unnamed
```

| option | type | meaning |
|---|---|---|
| `encoding` | `"srgb" \| "linear"` | override what the file implies |
| `box` | `box` (file px) | read only this window; the frame origin becomes (x, y) |
| `origin` | `point` (doc px) | declare the frame origin of a file that is itself a window; not with `box`, and it overrides a recorded origin |
| `as` | identifier | session only (§7.10): bind the image to this global |

**Box and point types.** Coordinates are passed in many places, and today each op declares them as `number[]` with
`Len: 4` or `Len: 2` (`pkg/cv/ops/mask.go:149`, `:266`). This ticket adds two argument types to the binder, first
needed by `open` and then used by every op with a box or point:

| type | accepts | canonical form |
|---|---|---|
| `box` | `[x, y, w, h]`; an object with numeric `x, y, w, h` | `[]any{x, y, w, h}` |
| `point` | `[x, y]`; an object with numeric `cx, cy` | `[]any{x, y}` |

The two object forms follow one naming rule that GCV-003 applies to every result: `x, y` is always the corner of a
box, `cx, cy` is always a point or a centre. An object with only `x, y` is refused as a point with the fix
`[obj.x, obj.y]` and the explanation. Objects are converted at bind time, so hashes and `explain()` see numbers.
`open`'s `box` is declared `{Type: "box", Unit: graph.Unitless, Static: true}`: it is in file px and must not be
scaled as a doc-px length.

`open` stays a registry op (`KindNone`, `pkg/cv/ops/source.go:11-36`) with `encoding`, `box` and `origin` as
declared arguments, so unknown options, synonyms and did-you-mean work through the binder. Its frame comes from its
`DocFrame` (§7.5) applied to `w`, `h`, `box`, `origin` and the recorded origin. `Engine.Open(path string, raw
map[string]any) (*Node, error)` probes the file and interns the node; the engine knows nothing of `as`.

Awaiting an opened image, or `cv.describe(img)`, gives its metadata:

```yaml
kind: Image
file: raw0027.dng
format: dng
bitDepth: 16
sample: uint
fileChannels: 3
encoding: linear
frame: [1536, 1536, 1024, 1024]
notes: [camera RGB primaries (DNG)]
w: 1024
h: 1024
scale: 1
```

### 7.5 Working spaces in the graph

#### OpDef changes

```go
// pkg/cv/graph/registry.go (fields changed or added; Space string and DocSize are removed)
type SpaceRule struct {
    Default buffer.Space // working space when the call passes no { space }; SpaceNone: the op does not read levels
    Fixed   bool         // the op only works in Default (toLab, encode): no space option is offered
}

// ResultEnc computes the encoding of an Image result from the op's args and
// inputs. The three named rules cover every op but encode.
type ResultEnc func(args Args, inputs []*Node) buffer.Encoding

var (
    KeepEnc    ResultEnc // the encoding of the first Image input (receiver, then Inputs, then kind-typed args)
    DerivedEnc ResultEnc // EncLinear: a new quantity (chroma, toLab; GCV-002's sub, highpass)
    DisplayEnc ResultEnc // EncSRGB: display colours (colormap)
)

type OpDef struct {
    // … Name, On, Inputs, Args, Returns, Identity, Halo, Global, NoCache, Lazy, Fields, Validate, AnalysisScale, Builders, Run, Doc, See …
    Space    SpaceRule
    Result   ResultEnc                                   // nil = KeepEnc
    Gray     func(args Args) []int                       // positions of the Image inputs the op reads as luminance (§3.5)
    DocFrame func(args Args, inputs []*Node) Frame       // the result's frame; nil = the first input with a non-empty frame
    Expand   ExpandFunc                                  // macros (§7.8); exactly one of Run and Expand is set
    Internal bool                                        // not callable from JavaScript, not listed in help (convert)
}
```

`Registry.Register` (`registry.go:155-196`) checks that exactly one of `Run` and `Expand` is set, and appends the
`space` option to every op whose `Space.Default` is not `SpaceNone` and not `Fixed`:

```go
graph.ArgSpec{Name: "space", Type: "enum:linear,encoded", Optional: true, Static: true,
    Doc: "working space: linear (the encoding undone) or encoded (the file's values, what looks show)"}
```

`Static` means the value must be a literal; a param cannot move it, because it changes the graph's shape.

#### Resolution in `Intern`, statics in `internNode`

`Builder.Intern` (`pkg/cv/graph/node.go:67-111`) binds arguments as today. Then:

1. **Space.** `s := args.space` if given, else `op.Space.Default`. When `s` is not `SpaceNone`, `s` is stored in
   the canonical args, so the hash and the cache signature include it.
2. **Grey input.** For each position that `op.Gray(args)` returns, a 3-channel Image input at that position is
   replaced by `toGray(input, "rec709")` interned in `linear` space (§3.5); its result is linear luminance with the
   input's encoding. Positions count over the op's inputs in order (receiver, `Inputs`, kind-typed arguments):
   `mask.luminance` and `detect.specks` return `[0]` (the receiver), `refine` returns `[1]` (its guide image, an
   `Inputs` entry, `pkg/cv/ops/mask.go:353`), and `histogram` returns `[0]` only for channel `"L"`.
3. **Conversions.** If `s` is not `SpaceNone`, every Image input that is not in `s` and for which
   `!Identity(input.Enc, input.Space, s)` is replaced by `convert(input, s)` (interned, so shared). The Image
   inputs come from three places, all covered: the receiver, the explicit `Inputs` (`bd.inputs`), and lifted
   kind-typed arguments (`bd.lifted`) whose kind is Image, including the elements of `Image[]` arguments
   (`compare`). Mask inputs are never converted.
4. **Warnings.** `SPACE` for lifted `Level` arguments (§7.6); `ENCODING` when `s` is `encoded` and Image inputs
   have different encodings ("blend mixes img (srgb) and scan (linear) in encoded space; their encoded scales
   differ. fix: { space: \"linear\" }").
5. `internNode(op, args, inputs, nExplicit)` creates or finds the node.

`internNode` (`node.go:113-143`) computes the statics from `(op, args, inputs)` alone, before the node is
published: `Frame` from `op.DocFrame` (default: the first input with a non-empty frame, as the `DocW` loop does
today at `node.go:131-140`); `Enc` and `Space` from `op.Result` and `s`:

| op returns | Enc | Space |
|---|---|---|
| Image, `KeepEnc` | first Image input's | `s`, or the first Image input's space when `s` is `SpaceNone` |
| Image, `DerivedEnc` | linear | linear (the values are reinterpreted, not converted) |
| Image, `DisplayEnc` | srgb | encoded |
| Mask, Regions | none | none |
| Value | the measured image's (first Image input's) | `s` when the op reports levels, else none; `field` nodes inherit their receiver's |
| any, macro node (§7.8) | its last input's (the expansion's output) | its last input's |

Because statics depend only on the hash inputs, a hash hit returns a node with exactly the statics a new node would
get. Nothing sets fields on a node after it is published, so `Node` stays immutable (`node.go:12`).

**Builder lambdas.** The placeholder that a lambda receives (`Engine.Placeholder`, `pkg/cv/graph/engine.go:122-141`)
copies the receiver's `Enc`, `Space` and `Frame` into hidden arguments (it copies `w`, `h` today at
`engine.go:139`), and its `DocFrame`/`Result` read them back, so a lambda's graph resolves exactly as it would on
the receiver. `Builder.Substitute` (`node.go:189-230`) re-interns the path through `internNode`, which recomputes
statics; since the placeholder and the receiver have equal statics, the conversions decided inside the lambda stay
valid. A Phase 3 test checks that `img.within(mask, i => i.contrast(0.2))` builds the same inner graph as
`img.contrast(0.2)` on an sRGB and on a linear-encoded image.

#### Defaults

Each default answers one of the two questions of §3.4. For sRGB images, every op that was "perceptual" now runs in
`encoded`, which is numerically identical per channel; luminance ops stay identical through the grey-input rule.

| op | default | question | notes |
|---|---|---|---|
| `exposure`, `gaussian`, `blend`, `within`, `heal`, `inpaint` | linear | physical | scaling, averaging, compositing light |
| `toGray` | linear | physical | Rec. 709 weights are defined on linear light; `average` is valid in either space |
| `stats` | linear | physical | photometric measurement; matches `levels` |
| `levels` | linear | physical | `levels({ black: s.p01, white: s.p99 })` with `s = img.stats()` keeps working (§8) |
| `contrast` | encoded | visible | contrast around the displayed mid-grey |
| `histogram` | encoded | visible | bins over the scale a look shows; channel `"L"` uses the grey-input rule |
| `mask.luminance`, `refine` | encoded | visible | grey-input rule |
| `mask.color`, `mask.flood` | encoded | visible | hue and tolerance of displayed colour |
| `detect.specks`, `check.dust` | encoded | visible | grey-input rule; `meanL` is reported in the same space, which changes it on sRGB images (below) |
| `overlay` colours | encoded | visible | colours are display values |

`levels` is the one visible change on sRGB images: the remap now happens in linear space and the gamma is applied
to the linear ramp. With `black = 0` the displayed result is sRGB_encode(v / w) instead of today's
sRGB_encode(v) / sRGB_encode(w). Because the sRGB curve is close to a power law, the two agree when `white` is high
and drift apart when `white` is low and in the deep shadows, where the curve's linear toe breaks the power law.
Measured over v ∈ (0, w], in 8-bit codes:

| white w | largest difference | largest difference for v/w in 20–80 % |
|---|---|---|
| 0.92 | 0.5 | 0.3 |
| 0.7 | 2.1 | 1.2 |
| 0.5 | 4.5 | 2.5 |
| 0.3 | 8.5 | 4.9 |

With `black > 0` shadows come out slightly darker. Two other changes are expected. Mask looks change (§3.6). The
`meanL` field of `detect.specks` items, linear luminance today (`pkg/cv/kernels/specks.go:175`), becomes
sRGB_encode of that luminance on sRGB images, because detection runs in encoded space and reports its numbers there;
`specks.where({ meanL: … })` thresholds move accordingly. Golden-look tolerances are in §13.3.

**Cost.** The source is linear, so on an sRGB file every encoded-default op, and every look of a linear-space node,
needs one `convert(→ encoded)` of its input, cached per scale: 12 MB for a 1024² RGB tile at scale 1. This is what
`ToImage` paid on every render before, now cached and visible.

#### The convert op, explain and help

```go
{
    Name: "convert", On: graph.KindImage, Returns: graph.KindImage, Internal: true,
    // Space: SpaceRule{} — Register adds no space option; Result: KeepEnc
    Args: []graph.ArgSpec{{Name: "to", Type: "enum:linear,encoded", Positional: true}},
    Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
        src := image(in[0])
        from := ec.InputNode(0).Space
        to, _ := buffer.ParseSpace(a.String("to"))
        out, err := buffer.Convert(ctx, src.Buf, src.Enc, from, to)
        if err != nil {
            return nil, err
        }
        return graph.NewImage(out), nil // the evaluator stamps Frame and Enc
    },
}
```

`EvalCtx` gains `InputNode(i int) *Node` for the few `Run` functions that need an input's statics. `Internal` ops
are skipped by `Registry.Methods` (the JavaScript method tables), `cv.help` and the generated reference; macros and
the builder may still call them.

`Describe` (`explain.go:53-85`) omits an argument when it equals its default (`explain.go:79`); the `space` option
has no declared default, so `Describe` also omits `space` when it equals `op.Space.Default`. `explain()` shows each
Image node's statics in a labelled right-hand column: `enc srgb · space linear`.

### 7.6 The Level unit and the SPACE warning

`DocPx` taught the binder that some numbers are lengths. This ticket adds a unit for numbers that are levels
(§3.3):

```go
const (
    Unitless Unit = iota
    DocPx
    Fraction
    DocPx2
    Level // a value on the scale of the op's working space (levels.black, mask.luminance.above, colormap.range)
)
```

When a `Level` argument is a lifted Value node whose static `Space` differs from the op's resolved space `s`, and
`!Identity(valueNode.Enc, valueNode.Space, s)`, the binder emits a warning. It does not convert the number: a
percentile would survive conversion, a mean would not, and the binder cannot tell them apart. On the colleague's
linear-encoded tiles `Identity` holds, so `img.mask.luminance({ above: img.stats().p99 })` raises no warning.

```yaml
warnings:
  - "SPACE line 2: levels.black comes from stats() in linear space, but levels runs in encoded space here. fix: img.stats({ space: \"encoded\" })"
```

### 7.7 Looks and display

A look renders encoded values. `look.Render` evaluates `convert(node → encoded)` (no node is added when `Identity`
holds or the node is a Mask), then `buffer.Quantize(buf, 8)` clamps to [0, 1] and rounds to 0…255. Concretely:

- sRGB images look exactly as before, because encoded = sRGB_encode(linear) is what `ToImage` computed.
- Linear-encoded images show their stored values linearly. A density tile opened with `encoding: "linear"` shows
  v as grey level v.
- Masks and derived images show their values directly (mask 0.5 → grey 128).

A linear-encoded photograph (a demosaiced linear raw) looks dark when its values are shown linearly. The op
`img.encode("srgb")` re-encodes it: same linear light, encoding `srgb`, so looks and encoded-space ops see the
sRGB curve. `encode` has `Space: {Default: linear, Fixed: true}` and a `Result` function that returns its
argument. It never changes linear values; to reinterpret a file, open it with another `encoding`.

### 7.8 Macros: `OpDef.Expand`

Some ops are best defined as a small graph of other ops: `split()` returns three channel nodes, and GCV-002's
`highpass`, `dog` and `view` presets are chains of `gaussian`, `sub` and `levels`. A **macro** is an `OpDef` with
`Expand` instead of `Run`:

```go
// ExpandFunc builds the op's graph from other ops at intern time. args are
// bound and the space is resolved (args.String("space") is set for macros
// with a SpaceRule).
type ExpandFunc func(x *Expander, recv *Node, args Args) ([]*Node, error)

type Expander struct{ /* builder, registry, accumulated warnings */ }

// Call interns op name on recv with args through Intern (binding, space
// resolution, conversions). It may call Internal ops. Warnings are collected.
func (x *Expander) Call(name string, recv *Node, args ...any) (*Node, error)
```

The binder binds a macro's arguments and resolves its space first, then calls `Expand`. What happens next depends on
how many nodes `Expand` returns:

- **One node** (highpass, dog, view): the builder interns a **macro node** whose op is the macro's own `OpDef`,
  whose inputs are the receiver and then the expansion's output, and whose args are the macro's canonical args.
  Evaluating a macro node evaluates its last input and returns that value, with no compute and no separate cache
  entry. Its statics are its last input's, computed in `internNode` (§7.5), so a derived expansion (highpass) gives
  a derived macro node whatever the macro's own `Result` says. The macro node gives `img.highpass(3)` its own identity and label (GCV-002's captions show `highpass(3)`),
  while the expansion's nodes are exactly the nodes of the hand-written chain and are shared with it through
  hash-consing. `explain()` shows the macro node with its expansion indented below it.
- **Several nodes** (split): no macro node; the call returns the nodes.

`Engine.Call` returns a `CallResult{Nodes []*Node; Warnings []Warning}` instead of `(*Node, []Warning, error)`;
the binding returns one handle or an array of handles, and the prelude turns an array into an array of node
proxies. Macros get help, option checking, synonyms and did-you-mean from the registry like every op.

### 7.9 Channel ops

| call | returns | space | result |
|---|---|---|---|
| `img.channel("r" \| "g" \| "b" \| 0 \| 1 \| 2)` | 1-channel Image | none (plane selection) | keeps encoding and space |
| `img.split()` | `[r, g, b]` (macro over `channel`) | — | — |
| `img.toLab()` | 3-channel Image: L*/100, a*/100, b*/100 | fixed linear | derived |
| `img.chroma({ relative })` | 1-channel Image: max − min over channels (÷ (mean + 1e-3) when `relative`) | encoded | derived |
| `img.colormap(name, { range, center })` | 3-channel Image | encoded | display |
| `img.encode("srgb" \| "linear")` | Image | fixed linear | its argument |

- `channel` on a 1-channel image with "g" or "b" is `E_ARG_RANGE` with the fix `img.toGray()`.
- `toLab` assumes sRGB/Rec. 709 primaries and the D65 white point. On a DNG source (note "camera RGB primaries")
  it emits a warning with code `PRIMARIES`: the a*/b* values are approximate.
- `colormap` accepts only 1-channel images (`E_WRONG_KIND` with the fixes `.channel("g")` and
  `.toGray("average")`). Names: `gray`, `viridis`, `magma`, `diverging` (blue–white–red). `range: [lo, hi]` is in
  `Level` units. Without `range`, the range is the input's [p01, p99] computed when the colormap is evaluated, at
  the evaluation scale (so a preview and a full-scale look can differ slightly); for `diverging` it is ±max(|p01|,
  |p99|) around `center` (default 0). The tables are 256-entry sRGB lookup tables in `pkg/cv/kernels/colormap.go`.

### 7.10 Named images

The session (not the engine) owns names.

```go
// pkg/cvsession/names.go
type nameTable struct {
    order []string               // binding order, for the context block
    nodes map[string]goja.Value  // name → node proxy (owner goroutine only)
}

var reservedNames = []string{"cv", "P", "mem", "show", "doc", "globalThis", "Math", "JSON", "Promise", "console"}
```

- The `cv.open` wrapper in `init.js` (`pkg/cvsession/init.js:16-21`) splits `{ as, ...rest }`, checks `as` through a
  session-native `bindName`, and passes `rest` to the module's `open`. Synonyms for `as` (`name`, `bind`, `alias`)
  are resolved by the wrapper with the option-synonym helper the binding exports from Go. The module's `open` op
  does not know `as`; in a bare module (no session), `cv.open(p, { as })` is `E_UNKNOWN_OPTION` with the hint "names
  exist only in eval sessions".
- `bindName` validates the name (`^[A-Za-z_$][A-Za-z0-9_$]*$`, not reserved, not a global the table does not own)
  and defines a getter/setter on `globalThis` that reads the table. Errors use the code `E_NAME`:
  `cv.open(…, { as: "show" }): "show" is a session global. fix: { as: "show2" }`.
- `img` is the first entry, bound by `--image` or `cv.open(path, { as: "img" })`; `img = cv.open(p)` keeps working
  through the setter. Re-opening with an existing name rebinds it.
- The context block lists names in binding order, then unnamed opens, each with file, frame, format and encoding:

```yaml
context:
  images: img = 1d955ef4b27d3d88_1536_1536.tif (1024×1024, 16-bit TIFF, srgb), scan = raw0027.dng [1536,1536,1024,1024] (16-bit DNG, linear), (unnamed) tile.npy (1024×1024, float32 NPY, linear)
```

- The MCP `open` tool gains `encoding` and `as` (`pkg/cvsession/mcp.go:47`). Their schema descriptions say when to
  omit them, because the agent runs showed that an optional parameter without an explanation gets invented values.

Two-image ops (blend, within, compare, GCV-002's arithmetic) require equal frames (origin and size). A mismatch is
`E_FRAME` with both frames.

### 7.11 The agent-facing surface

```js
img = cv.open("tile_v.tif", { encoding: "linear", origin: [1536, 1536] })
cv.open("clean.tif", { as: "ref" })
const s = await img.stats({ space: "encoded" })                 // numbers in file units (equal to linear here)
const lum = img.mask.luminance({ above: 0.9 })                   // encoded by default
const neutral = img.chroma({ relative: true }).colormap("viridis")
return { enc: (await img).encoding, p99: s.p99, plan: lum.explain(), look: neutral.look("chroma") }
```

`cv.help("open")` answers from the registry. `space`, `encoding`, `frame` and `names` are concepts, not ops; they
become entries of the help table for non-op names (§10).

The key points:

- `internNode` is the only place statics are set, so every creation path (Intern, Substitute, macros) agrees.
- The builder decides conversions and grey inputs; kernels never convert.
- Looks show encoded values; masks and derived images show their values.
- Macros expand at intern time; single-output macros keep their own node for labels.
- Names live in the session; frames make coordinates absolute.

## 8. Decision records

### Decision: encoding is a property of the data, space a property of the op

- **Context:** The current code conflates the two: decode assumes an encoding, kernels assume a space. Issues 6 and
  7 each ask for one of them.
- **Options considered:** (a) one session-wide "the data is linear" switch; (b) a per-op space option only,
  keeping the sRGB decode; (c) an encoding per source plus a space per op.
- **Decision:** (c).
- **Rationale:** (a) cannot express two images of different encodings in one session (issue 20). (b) keeps the
  export trick alive: without an encoding, "encoded" for the colleague's tile is still sRGB_encode(v). With (c),
  sRGB data behaves as today and linear data has linear = encoded.
- **Consequences:** Nodes carry `Enc` and `Space`; decoders report encodings; the context block shows them. ICC
  profiles remain unsupported.
- **Status:** accepted

### Decision: two spaces, no `perceptual`

- **Context:** Revision 1 had three spaces; no op defaulted to `perceptual`, and for sRGB images it equals
  `encoded`.
- **Options considered:** (a) keep `perceptual` as a third option; (b) two spaces, with `img.encode("srgb")` as the
  way to get perceptual values of linear light.
- **Decision:** (b).
- **Rationale:** `perceptual` doubled the conversion table, the enum, the synonyms and the docs for a case that
  `encode("srgb")` covers explicitly. Issue 7 asks for `linear | encoded`.
- **Consequences:** Measuring lightness of a linear photograph is `img.encode("srgb").stats({ space: "encoded" })`.
- **Status:** accepted

### Decision: luminance is computed from linear light

- **Context:** Ops that reduce colour to one plane compute today sRGB_encode(Y of linear channels). Computing the
  weighted sum on encoded channels gives luma, which differs: pure red 0.498 vs 0.213, orange (0.8, 0.4, 0.2) 0.512
  vs 0.471.
- **Options considered:** (a) accept luma in encoded space and list the change; (b) compute Y from linear channels
  and express it in the op's space, through a graph rule.
- **Decision:** (b), with the `Gray` rule: the builder feeds each luminance input of such ops `toGray(input, "rec709")` in linear space,
  then converts to the op's space.
- **Rationale:** (b) is photometrically correct, keeps today's luminance numbers for sRGB images (up to the 1-code flips of §13.3), and keeps kernels
  space-agnostic because the graph does the work.
- **Consequences:** Luminance ops on colour images get one extra cached node; `toGray` is part of their graph and
  visible in `explain()`.
- **Status:** accepted

### Decision: conversions are graph nodes inserted by the builder

- **Context:** Some layer must convert between spaces.
- **Options considered:** (a) each kernel converts (status quo); (b) the evaluator converts inputs invisibly before
  `Run`; (c) the builder inserts `convert` nodes at intern time.
- **Decision:** (c).
- **Rationale:** (c) makes conversions cached and shared by hash-consing, visible in `explain()`, and testable as
  ordinary nodes. (b) would need a second cache keyed by (node, space). (a) is what made spaces untestable.
- **Consequences:** Every node needs static `Space`/`Enc`; the evaluator is unchanged.
- **Status:** accepted

### Decision: the space is resolved per call with op defaults; no session or image default

- **Context:** Issue 7 suggests a per-op option or one session-wide setting.
- **Options considered:** (a) per-call option with op defaults; (b) plus a session default; (c) plus an image default.
- **Decision:** (a).
- **Rationale:** A default outside the call makes the same source text build different graphs depending on hidden
  state. The colleague does not need one: with `encoding: "linear"` their spaces coincide.
- **Consequences:** Agents write `{ space }` where they mean it. An image-level default could be added later in
  the same resolution step.
- **Status:** accepted

### Decision: `levels` and `stats` default to linear

- **Context:** `levels` is a display stretch, which suggests `encoded`; agents and docs use
  `levels({ black: s.p01, white: s.p99 })` with `s = img.stats()`.
- **Options considered:** (a) both linear; (b) `levels` encoded, `stats` linear, with a `SPACE` warning on the idiom;
  (c) both encoded.
- **Decision:** (a).
- **Rationale:** (b) would warn on the most common idiom in every session. (c) makes `stats` report sRGB-encoded
  numbers for photographs, where photometric measurement is the expected meaning. (a) keeps the idiom correct and
  changes `levels` output by at most 0.5–8.5 codes (table in §7.5).
- **Consequences:** The levels change is documented as the main visible break on sRGB images.
- **Status:** accepted

### Decision: derived quantities carry the linear encoding; masks carry none

- **Context:** Differences, ratios, chroma and Lab are not light; masks are weights.
- **Options considered:** (a) treat every Image alike; (b) a derived result rule reinterpreting values as
  linear-encoded, and no encoding for masks.
- **Decision:** (b).
- **Rationale:** With (a), `stats()` of a high-pass built in encoded space returns sRGB_decode of differences, and
  a look shows sRGB_encode of differences. With (b), numbers and grey levels agree.
- **Consequences:** Op authors pick `KeepEnc`, `DerivedEnc` or `DisplayEnc`. Mask looks change (0.5 → 128).
- **Status:** accepted

### Decision: looks show encoded values

- **Context:** For linear-encoded data, showing sRGB_encode(linear) would show v = 0.3 as 0.584 and a signed map's
  0.5 offset as 0.735.
- **Options considered:** (a) show perceptual (today's rule); (b) show encoded; (c) a per-look display option.
- **Decision:** (b), with `img.encode("srgb")` for linear photographs.
- **Rationale:** (b) is today's rule for every sRGB file and the scientific-viewer rule for linear data. (c) adds a
  second place where spaces are chosen.
- **Consequences:** Linear photographs need `encode("srgb")` or a stretch to look normal.
- **Status:** accepted

### Decision: sources are stored as a linear pyramid

- **Context:** The source loader must store something; previews are area averages.
- **Options considered:** (a) store encoded values and average them; (b) store linear values and average light.
- **Decision:** (b), as today.
- **Rationale:** Area averaging is an average of light only in linear values; (a) would darken previews of
  high-contrast detail. For linear-encoded files the two are identical.
- **Consequences:** Every encoded-space op on an sRGB file needs one cached conversion (§7.5, "Cost").
- **Status:** accepted

### Decision: one TIFF decoder with its own IFD parser and decompression

- **Context:** `x/image/tiff` rejects float samples and LinearRaw, has no SubIFD or strip-level API, and cannot read
  windows.
- **Options considered:** (a) a separate DNG decoder next to x/image/tiff; (b) libtiff through cgo; (c) one own
  parser for all TIFFs, decompressing strips itself with `x/image/tiff/lzw`, `compress/zlib` and PackBits.
- **Decision:** (c).
- **Rationale:** One code path covers probing, SubIFDs, float, LinearRaw, compression and windows, and keeps the
  static pure-Go build. (a) would leave float + LZW unsupported and duplicate header parsing.
- **Consequences:** We own a TIFF parser and fuzz it (§13). BigTIFF and tiled TIFFs are refused.
- **Status:** accepted

### Decision: frames replace doc sizes; coordinates are absolute

- **Context:** A window read from a scan should keep scan coordinates; GCV-002's crop needs the same.
- **Options considered:** (a) keep `DocW/DocH` and add an origin where needed; (b) one `Frame` on nodes and values,
  with absolute coordinates everywhere.
- **Decision:** (b).
- **Rationale:** Every reader of `DocW/DocH` already reasons about "the rectangle this image covers"; one type makes
  the origin impossible to forget.
- **Consequences:** `Args.XY` shifts point coordinates and box origins by the frame origin (sizes are not shifted);
  region geometry adds the origin when regions are created (`RegionsValue.NewRegion`, `regions.go:67-100`); `Rasterize` and two-image
  ops compare frames.
- **Status:** accepted

### Decision: `box` and `point` argument types; `x, y` is a corner, `cx, cy` a centre

- **Context:** Boxes and points are bare arrays today; GCV-002 and GCV-003 need to pass region items and
  measurement rows as boxes and points.
- **Options considered:** (a) keep arrays; (b) structural coercion (any object with `x, y` is a point); (c) typed
  arguments with one naming rule.
- **Decision:** (c).
- **Rationale:** (b) would read a region's box corner as its centre, which is the mistake to prevent. (c) gives each
  name one meaning across all three tickets.
- **Consequences:** GCV-003's result rows name points `cx, cy`.
- **Status:** accepted

### Decision: macros expand at intern time; one-node macros keep a macro node

- **Context:** Views and `split` are compositions; captions should name them, and their parts should be shared.
- **Options considered:** (a) kernels for each composite op; (b) JavaScript helpers in the prelude; (c) Go macros
  expanded at intern time, wrapped in a pass-through macro node when they produce one node.
- **Decision:** (c).
- **Rationale:** (c) reuses the registry for help and options, shares caches with hand-written chains, and gives a
  label without mutating shared nodes. (b) would hide the composition from Go and from `explain()`.
- **Consequences:** `Register` accepts `Expand` instead of `Run`; `Engine.Call` returns several nodes.
- **Status:** accepted

### Decision: named images are a session feature

- **Context:** Issue 20 asks for several names.
- **Options considered:** (a) names in the engine; (b) names in the session layer as globals backed by a table.
- **Decision:** (b).
- **Rationale:** Names do not change what a node computes, so they must not reach the engine or the hash; the session
  already owns `img`.
- **Consequences:** The `init.js` wrapper handles `as`; the context block and the MCP tool learn names.
- **Status:** accepted

## 9. Pseudocode and key flows

### 9.1 Opening and decoding

```
cv.open(path, opts)                                  // init.js wrapper
  checkPath(path); { as, ...rest } := opts
  node := module.open(path, rest)                    // binding → Engine.Open
    info := buffer.ProbeFile(abs)                    // decoder registry, header only
    Intern(open, raw{path, encoding?, box?, origin?} + hidden{stamp, w, h, format, bits, sample, fileEncoding, recordedOrigin})
      internNode: Frame = DocFrame(args), Enc = encoding or fileEncoding, Space = linear
  if as: bindName(as, node)
  return node

first evaluation of open at scale s:
  src := sources.Load(abs, stamp, box)               // once per file version and window
    buf, info := decoder.Decode(r, {Box})            // encoded values
    lin := buffer.Convert(buf, enc, encoded, linear) // exact tables for 8/16-bit integers; identity for linear
    pyr := NewPyramid(lin)
  return NewImage(pyr.AtScale(s))                    // evaluator stamps Frame, Enc
```

### 9.2 Interning with spaces

```
Intern(op, receiver, raw):
  args, warnings := bind(op, raw)                       // types, units, box/point coercion, unknown options, synonyms
  s := args.space or op.Space.Default
  if s != none: args.space = s
  if op.Expand != nil:
      nodes := op.Expand(expander, receiver, args)      // inner Calls go through Intern
      if len(nodes) == 1: return internNode(op, args, [receiver, nodes[0]])   // macro node
      return nodes
  inputs := [receiver] + explicit inputs + lifted inputs
  for i in op.Gray(args) where inputs[i] has 3 channels:
      inputs[i] = Intern(toGray, inputs[i], ["rec709", {space: linear}])
  if s != none:
      for each Image input i (including Image[] elements):
          if i.Space != s and !Identity(i.Enc, i.Space, s): replace i by Intern(convert, i, [s])
  warn SPACE for lifted Level args v with v.Space != s and !Identity(v.Enc, v.Space, s)
  warn ENCODING when s == encoded and Image inputs differ in Enc
  return internNode(op, args, inputs)                    // computes Frame, Enc, Space from (op, args, inputs)
```

### 9.3 Two traces

The colleague's tile in today's form (stores sRGB_encode(v); `srgb` implied):

```js
return img.mask.luminance({ above: 0.9 }).explain()
```

```yaml
- "#4 mask.luminance(above: 0.9)        Mask 1024×1024"
- "  #3 convert(→ encoded)              Image 1024×1024   enc srgb · space encoded"
- "    #2 toGray(rec709)                Image 1024×1024   enc srgb · space linear"
- "      #1 source 1d955ef4….tif         Image 1024×1024   enc srgb · space linear"
```

The same tile re-exported with v stored and opened with `encoding: "linear"`:

```yaml
- "#3 mask.luminance(above: 0.9)        Mask 1024×1024"
- "  #2 toGray(rec709)                  Image 1024×1024   enc linear · space linear"
- "    #1 source 1d955ef4…_v.tif         Image 1024×1024   enc linear · space linear"
```

No conversion node: for a linear encoding the spaces coincide. The first mask thresholds sRGB_encode(Y) > 0.9; the
second thresholds Y(v) > 0.9. That difference is issue 7.

A builder lambda on the first tile, `img.within(mask, i => i.contrast(0.2))`: the placeholder has `enc srgb · space
linear`, so `contrast` (encoded default) gets `convert(placeholder → encoded)`; after `Substitute`, the same
`convert(img → encoded)` node that `img.contrast(0.2)` uses.

### 9.4 Rendering a look

```
look.Render(node, scale):
  root := node
  if node.Kind == Image and !Identity(node.Enc, node.Space, encoded): root = Intern(convert, node, [encoded])
  v := Evaluate(root, scale)
  img := buffer.Quantize(v.Buf, 8)                     // clamp to [0, 1], round to 0..255, no transfer curve
  burn the look number; encode PNG or JPEG (GCV-002 adds format)
```

## 10. Errors and diagnostics

New and changed diagnostics follow the existing patterns (Foundations §F10).

| situation | code | message and fix |
|---|---|---|
| unsupported file feature | `E_FORMAT` | names the feature: `cv.open("x.dng"): CFA (mosaiced) raw data is not supported; goja-cv reads LinearRaw DNGs.` Likewise for tiles, planar configuration, BigTIFF, Fortran order. |
| unknown encoding | `E_ARG_TYPE` | `cv.open.encoding = "gamma" is not one of srgb, linear.` fix: `cv.open(path, { encoding: "linear" })` |
| box or point outside the frame | `E_ARG_RANGE` | the frame and the nearest valid box of the same size as the fix |
| `box` and `origin` together | `E_ARG_TYPE` | "box already sets the origin" |
| `{ x, y }` given as a point | `E_ARG_TYPE` | "x, y is a box corner; a point is [x, y] or an object with cx, cy" with fix `[obj.x, obj.y]` |
| bad or taken name | `E_NAME` | names the conflicting global; fix with a free name |
| two-image op on different frames | `E_FRAME` | both frames |
| lifted level from another space | `SPACE` warning | §7.6 |
| mixed encodings in encoded space | `ENCODING` warning | §7.5 |
| `toLab` on camera RGB | `PRIMARIES` warning | "a*/b* are approximate: the DNG's colour matrices are not applied" |
| channel out of range | `E_ARG_RANGE` | fix `img.toGray()` or the valid channel names |
| colormap on 3 channels | `E_WRONG_KIND` | fix `img.toGray("average").colormap("viridis")` |

New codes go into `pkg/cv/graph/errors.go`, except `E_NAME`, which belongs to the session (`pkg/cvsession`).

Option synonyms (`OptionSynonyms`, `pkg/cv/graph/args.go:590-605`) gain `colorSpace`, `colorspace`, `transfer`,
`gamma` → `encoding`; `window`, `rect`, `region` → `box`; `workingSpace` → `space`. The session wrapper maps
`name`, `bind`, `alias` → `as`. Enum values get value synonyms scoped per enum (a map keyed by the enum's
`ArgSpec`): for `encoding`, `sRGB` and `gamma` → `srgb`, `lin`, `raw`, `identity` → `linear`; for `space`, `file`,
`stored`, `code`, `display` → `encoded`, `lin`, `light` → `linear`. Op synonyms (`pkg/cv/graph/suggest.go:10-41`)
gain `lab` → `toLab`, `saturation`, `channelSpread` → `chroma`, `falseColor`, `lut`, `heatmap` → `colormap`,
`reencode` → `encode`.

`cv.help` never throws (`pkg/gojamod/cv/help.go:12-17`). Its table of non-op names gains `encoding`, `space`,
`frame` and `names`, each with a usage line and the docs topic to read. The `metadata` redirect text says
`encoding` instead of `colorSpace`.

## 11. Documentation and examples that change

Docs are part of the tool: agents read them through `cv.docs`, and every ```js block runs in tests (Foundations
§F11).

- A new topic `pkg/cv/docs/topics/10-encodings-and-spaces.md` (id `encodings-and-spaces`) explains §3 in agent
  language: the two spaces and the two questions, the defaults table, opening linear data, `{ space }`, `encode`,
  and a `js warns=SPACE` block. The "Linear and perceptual" section of `02-units-and-scales.md:43-49` is removed
  and links to it.
- `02-units-and-scales.md` gains a "Frames" section; its awaited-summary example (line 25, which lists `docW`,
  `docH` and `colorSpace`) changes to `frame` and `encoding`.
- `01-getting-started.md` describes names in the globals table; `06-looks.md` says that masks and derived images
  show their values; `05-masks-and-regions.md` and `09-recipes.md` gain "Open scientific data" and "Neutral or
  coloured?" recipes.
- The guidebook `pkg/doc/02-concepts.md` replaces "float32 pixels in linear light" with encodings and spaces;
  `pkg/doc/03-cli.md` says `--image` binds `img`; `pkg/doc/08-recipes.md` replaces `docW` with `frame`.
- The MCP instructions (`pkg/cvsession/reference.go:15-37`) replace "(linear light)" and "black/white are linear
  levels like stats" with one line: "Numbers are in the op's space: linear (exposure, blur, blend, stats, levels)
  or encoded (what looks show: thresholds, contrast, histogram, specks); pass { space } to change it;
  cv.docs.read(\"encodings-and-spaces\")."
- README: the quick-start `stats` output and the image-metadata paragraph.

Tests and examples that change because of breaking behaviour:

| change | affected |
|---|---|
| `levels` remaps in linear | numbers after `levels` in `06-looks.md` (meanL 0.27479), README, `pkg/cv/ops/ops_test.go` levels cases, golden looks |
| masks display linearly | golden mask looks |
| `colorSpace` → `encoding` | `pkg/cvsession/session_test.go:378`, README |
| `docW/docH` → `frame` in awaited summaries | tests reading `docW`/`docH`; summaries keep `w`, `h` and add `frame` |
| histogram result `space` is the resolved space | histogram tests |
| `toGray("rec601")` removed | tone tests, docs mentioning it |

## 12. Implementation phases

Each phase ends with `go test ./... -count=1`, `make lint` and a commit, and states how the system behaves after it.
Write the listed tests first.

**Phase 0 — safety net and dead code.**
1. Write `pkg/cv/look/golden_test.go`. It renders a fixed list of looks on the planted scan (the source, exposure,
   gaussian, contrast, `levels` from stats, each mask op, a `detect.specks` overlay, the histogram chart, and the
   three `compare` modes) and stores per-look PNG bytes under `pkg/cv/look/testdata/golden/`. A `-update` flag
   rewrites them; a mismatch reports the largest per-channel code difference and the number of differing pixels.
2. Add `TestKernelsAreSpaceAgnostic` (`pkg/cv/kernels/space_guard_test.go`): parse the non-test files of
   `pkg/cv/kernels` and `pkg/cv/ops` with `go/parser` and fail on calls to `buffer.ToPerceptual`,
   `buffer.ToLinear`, `buffer.Linear8/16`, `buffer.EncodeSRGB8/16`; skip it until Phase 3 ends.
3. Remove `Node.Base` and the unused codes.

After Phase 0, behaviour is unchanged and the goldens record it.

**Phase 1 — encodings and conversion.**
1. Write `pkg/cv/buffer/encoding_test.go`: the identity table; `Convert` round trips within 1e-6 relative for random
   values in [−0.5, 2]; the numbers of §3.2 (0.5 → 0.214, 0.05 → 0.0039); 8/16-bit tables equal the exact functions
   at every code.
2. Implement `Encoding`, `Space`, `ParseSpace`, `Identity`, `Convert`. Keep `ToImage`.

After Phase 1, nothing uses the new types yet.

**Phase 2 — frames and coordinate types.**
1. Add a test-only source op, `graphtest.RegisterSource(reg)` in a new package `pkg/cv/graph/graphtest`, with hidden
   `enc`, `frame` and `fill` arguments that produce a fixed buffer with any encoding and frame origin. Phases 2 and 3
   use it for linear-encoded and non-zero-origin inputs, which `open` cannot produce before Phase 4 (`encoding`,
   `box`, `origin`) and Phase 5 (`encode`).
2. Write tests: `Frame` replaces sizes with no behaviour change (the goldens pass); `box` and `point` accept arrays
   and the object forms and refuse `{ x, y }` as a point; `Args.XY` shifts by a non-zero origin (test source).
3. Replace `DocW/DocH` with `Frame` on nodes, values and Regions (`grep -rn DocW pkg cmd` lists the sites);
   `DocSize` becomes `DocFrame`; values are stamped by the evaluator (copies for returned inputs); add `box`/`point` types and switch
   `stats.box`, `mask.box`, `mask.ellipse`, `mask.flood` and `mask.gradient`'s point form to them.

After Phase 2, every frame has origin (0, 0) and all goldens pass unchanged.

**Phase 3 — spaces in the graph.**
1. Write `pkg/cv/graph/space_test.go`: defaults per op (table-driven over §7.5); explicit `{ space }` changes the
   hash; a conversion node is inserted once and shared by two consumers (`Evaluator.Runs`); no conversion for
   linear-encoded inputs (test source); the grey-input rule gives today's numbers for pure red, for `mask.luminance`
   and for `refine` with a colour guide; `DerivedEnc` reinterprets; an identity short-circuit and a macro node do not
   change their input's cached value's statics;
   `within` lambdas build the same inner graph as direct calls on sRGB and linear images; `compare([a, b])` with
   inputs in different spaces converts each; `SPACE` fires across spaces on sRGB and not on linear-encoded data;
   `ENCODING` fires on mixed encodings.
2. Replace `OpDef.Space string` with `SpaceRule`; add `Result`, `Gray`, `Internal`; `Register` appends the `space`
   option; `Intern` resolves spaces and inserts `toGray`/`convert`; `internNode` computes `Enc`/`Space`; the
   placeholder copies statics; the `Level` unit and the warnings. Until Phase 4, `open` nodes are `enc srgb · space
   linear`, which is what every file means today.
3. Port the kernels in §4.3: remove the private conversions; `PerceptualLuminance` is deleted (the grey-input rule
   replaces it); `DetectSpecks` reports `MeanL` in its working space; enable the guard test.
4. `look.Render` renders encoded values through `convert` and `Quantize`; `WriteFile` writes `convert(→ encoded)`
   values for sRGB images; delete `ToImage`. Run the golden test with the tolerance rule of §13.3, inspect the
   reported differences, then regenerate the goldens with `-update` in one commit.

After Phase 3, sRGB images behave as before except `levels`, mask display and `detect.specks`' `meanL`, up to the
1-code flips of §13.3, and every op accepts `{ space }`.

**Phase 4 — decoders and opening.**
1. Extend `internal/testimage` with writers for the planted scan's float buffer as 16-bit linear TIFF, float32 TIFF,
   LZW TIFF with predictor, `.npy` (`<f4`, `<u2`), and a minimal DNG (IFD0 thumbnail plus a SubIFD with
   `NewSubfileType` 0, Photometric 34892, 16-bit, 3 samples, uncompressed). Each writer is a few dozen lines.
2. Write decoder tests: each fixture decodes to its generating float buffer within one code of its format (1/65535
   for 16-bit, exact for float); `Probe` fields; `Box` reads equal the full decode cut to the box; a hand-written
   `ImageDescription` and PNG `tEXt`/`gAMA` chunk produce `Info.Encoding` and `Info.Origin`; malformed headers return
   `E_FORMAT` and never panic (`FuzzTIFFProbe`, `FuzzNPYProbe`, seeded with the fixtures).
3. Implement the registry, `png`, `jpeg`, `tiff`, `npy`; delete `Decode`, `DecodeFile`, `DecodeConfigFile`; give
   `open` its `encoding`, `box` and `origin` arguments and `DocFrame`; `SourceInfo` reports `encoding`, `sample`,
   `frame` and `notes`.

After Phase 4, linear, float, npy and DNG files open, and windows keep file coordinates.

**Phase 5 — macros and channels.**
1. Write tests: `split()` returns three nodes equal to three `channel` calls; a one-node macro has its own node whose
   last input is the hand-written chain's node; `channel` on 1 and 3 channels; `toLab` of sRGB white → (1, 0, 0) and
   sRGB red → L* 53.24, a* 80.09, b* 67.20; `chroma` of a grey image is 0; `colormap` endpoints; `encode` keeps
   linear values.
2. Implement `Expand`, `Expander`, `CallResult`, the binding's array return, and the ops; remove `rec601`.

**Phase 6 — named images.**
1. Write `pkg/cvsession/names_test.go`: `as` binds and persists across evals; reserved and invalid names give `E_NAME`
   with a fix; rebinding; unnamed opens appear in the context block; the MCP `open` tool with `as`; the bare module
   rejects `as`.
2. Implement the name table, the `init.js` wrapper, the context block and the MCP schema.

**Phase 7 — documentation and agent check.**
1. Write the topics, help entries, synonyms, instructions, guidebook and README changes of §10 and §11; the docs tests
   must pass.
2. Add the gated real-data tests (§13.4).
3. In the dust-scanning repository, add the `IMAGE` and `QUESTIONS` overrides to
   `CVJS-001/scripts/06-pi-cv-agent/run.sh` and the question file (§13.5); run both builds twice and record the
   results in `CVJS-001/various/pi-runs/README.md`.

The key points:

- Phase 0 records today's looks before anything changes, so every later difference is deliberate.
- Frames land before spaces, and both before decoders, so each phase has defined, testable semantics.
- The kernel port and the display change happen in the same phase, which is the only phase where goldens move.

## 13. Test strategy

### 13.1 Unit and property tests

- `encoding_test.go` checks identities, round trips, monotonicity and the exact integer tables.
- The decoder tests compare each fixture with the float buffer that generated it, check probes and windows, and fuzz
  the TIFF and NPY header parsers.
- `space_test.go` covers resolution, sharing, grey inputs, derived reinterpretation, lambdas, lists and warnings.
- The existing kernel tests keep passing with explicitly converted inputs, and the guard test forbids private
  conversions.

### 13.2 Documentation tests

Every ```js block in `pkg/cv/docs/topics` and `pkg/doc` runs against the planted scan (`docs_test.go:77-152`). The new
topic needs a linear fixture: the docs test harness writes the planted scan as a 16-bit linear TIFF and an `.npy`
into the same temporary directory, and the docs refer to them by relative names (`cv.open("planted-linear.tif")`).

### 13.3 Golden looks

The Phase 0 golden test is the regression guard for "sRGB images look as before". It cannot stay byte-identical
across Phase 3, for two measured reasons: display used to round sRGB_encode(v) in float64, while the new path stores
encoded values as float32 and then quantizes, which flips 22 of 3 million 8-bit codes; and kernels used the
interpolated LUT (error up to 2e-5, `color.go:58-60`) where `Convert` uses the exact functions, which flips about
0.26 % of codes in `contrast`, `compare` and histogram bins and can move borderline specks.

The rule is therefore:

- `levels` looks and mask looks are expected to change; they are inspected by eye and regenerated.
- Every other look may differ by at most 1 code on at most 1 % of its samples; the test reports the largest
  difference and the fraction.
- `detect.specks` overlays must have the same number of specks; their `meanL` values are expected to change (§7.5).
- Anything beyond that is a bug in the defaults table or the grey-input rule.

After the check passes, the goldens are regenerated with `-update` in one commit, and from then on the test is
byte-exact again.

### 13.4 Real data (gated)

On `GOJACV_TILES` (default `~/dustdet/cv_tiles`, skipped when absent):

- Open `1d955ef4b27d3d88_1536_1536.tif` (stores sRGB_encode(v)); write its linear buffer as a 16-bit linear TIFF in
  `t.TempDir()`; open that with `encoding: "linear"`. Assert that the linear buffers agree within 1/65535, that
  `mask.luminance({ above: 0.9, space: "linear" })` on the first equals `mask.luminance({ above: 0.9 })` on the
  second, and that `stats()` agree (the tile has meanL 0.33567, p01 0.0085, p99 0.918 in linear today).
- With `origin` from the tile's JSON, region centroids are absolute raw-scan coordinates.

On `GOJACV_DNG` (for example `raw0027.dng`, listed in `~/dustds/meta/dng_meta.jsonl`): `Probe` gives 7256 × 7326,
16-bit, linear; `open({ box: [1536, 1536, 1024, 1024] })` returns in under a second and equals the same window cut
from a full decode (run the full decode only with `GOJACV_DNG_FULL=1`).

### 13.5 Agent harness

The pi harness (Foundations §F11) gets `IMAGE` and `QUESTIONS` environment overrides and a question file
`questions-gcv-001.md` next to `questions.md`, run against a density tile re-exported as v (16-bit linear) plus
the original tile as a reference:

1. What is the image's encoding, and what are p01 and p99 in file units?
2. Open the second file as `ref`. Is it the same frame? What is the largest absolute difference in file units?
3. Is the bright blob near (x, y) neutral or coloured? Give a number.

Run the build before this ticket and after, twice each, because the agent's path varies between runs of the same
build (two runs on one build in CVJS-001 differed by 21 calls). Record calls, errors and correctness in the
pi-runs table. Acceptance: on the new build, both runs answer all three questions correctly with no `E_` error
caused by a missing capability.

## 14. Risks, alternatives and open questions

**Risks.**

- The `levels` change is visible in agents' recipes. The golden diff and the docs examples show it; if it hurts,
  revisit the `levels` default before anything else.
- Conversion and grey nodes add cached buffers: 12 MB per conversion on a 1024² tile, 638 MB on a full DNG. Windows
  are the answer for DNGs, and the docs recipe uses them.
- Parser bugs in an own TIFF reader. Fuzzing and fixture equivalence are required before Phase 4 merges.
- Agents must learn spaces. The mitigations are two spaces instead of three, the two-question rule, the `SPACE`
  warning with a fix, and the docs topic.

**Alternatives not taken.** A single "data mode" flag; an ICC-based colour pipeline; reading DNGs through a Python
sidecar (not needed for LinearRaw).

**Open questions.**

1. Should `encode` accept a gamma (`encode({ gamma: 2.2 })`)? Only if a user brings such data.
2. Should the NPY decoder support memory-mapped windows of whole-scan caches (7256 × 7326 × 3 uint16)? It already
   reads windows with seeks; this is about performance only.

## 15. Closing

GCV-002 can now assume an image model in which every node knows its frame, its encoding and the space of its
values; in which a view's `space` is one option resolved at build time; in which derived values display and measure
as themselves; and in which composite views are macros with their own labels. GCV-002 adds the render scale and
the decoration rule on top of these frames, and its arithmetic and high-pass ops are the first `DerivedEnc` users.
GCV-003 adds measurements that report levels in a resolved space and writes files whose encoding and frame these
decoders read back.

## 16. References

- Foundations: `ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md`.
- Review findings with disposition: `reference/03-review-read-findings.md` in this ticket.
- Handover issues 6, 7, 10, 13, 20, 22: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md`; `05-goja-cv-feature-requests.md` items 4, 6, 7; `06-goja-cv-generic-inspection-features.md` B2b, B6, B7, C4, D1, D2.
- Playbook `…/DUST-002…/reference/04-goja-cv-dust-playbook.md`; export script `…/scripts/82-export-cv-tiles.py`; helpers `…/scripts/84-cv-helpers.js`.
- DNG layout: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/dustds/rawio.py`, `~/dustds/meta/dng_meta.jsonl`.
- CVJS-001 intern guide (design that preceded goja-cv): `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/` (Part 2 §2, Part 3 §15.1 and §15.3; `06-integration-surfaces…` for file format tiers).
- TIFF 6.0 specification (Adobe, 1992); DNG specification 1.6 (Adobe); NumPy `.npy` format (NEP 1); sRGB: IEC 61966-2-1; CIE L*a*b* with D65: CIE 15:2004.
- Next tickets: GCV-002 and GCV-003, `design-doc/01-design-and-implementation-guide.md` in each.

## Implementation notes (deviations)

Recorded while implementing on branch `task/gcv-001-image-model` (details and reasons in
`reference/06-diary-implementation.md`). The guide above is unchanged; these notes say where the code differs.

1. **`ResultEnc` is a struct, not a func type** (§7.5). `ResultEnc{Enc, Space}` holds two functions, and `OpDef.Result`
   is a `*ResultEnc`. A func value cannot be compared, so `internNode` could not tell `DerivedEnc` (space linear) from
   `KeepEnc` (space = the op's), and `convert` (space = its `to` argument), sources and the placeholder need their own
   space rule. `KeepEnc`, `DerivedEnc` and `DisplayEnc` keep their names and meaning.
2. **The grey-input rule always inserts `toGray`** (§7.5 step 2). Channel counts are not node statics, so the builder
   cannot know at intern time that an input has three channels. `toGray` on a one-channel image returns its input (a
   stamped copy), so the result is the same; `explain()` shows the extra `toGray` node for grey images.
3. **`Identity(e, from, to)` is also true when either space is `none` or the encoding is not `srgb`** (§7.2): masks
   and derived values are never converted.
4. **Golden tolerance classes** (§13.3): besides levels and mask looks, two classes may change in the Phase 3 commit:
   histogram charts (a bin count that flips by one moves a bar edge by a whole pixel: up to 215 codes on ≤ 0.006 % of
   samples) and overlays (`overlay` runs in encoded space per §7.5, so mask and region tints are composited on display
   values; tinted pixels change by up to 74 codes). Every other look stayed within 1 code on ≤ 0.006 % of its samples
   and the speck count stayed 41. The `compare` side-by-side background is the encoded constant 0.1517
   (sRGB_encode(0.02)), which keeps that look unchanged.
5. **`stats` has no grey-input rule.** It reports per-channel numbers next to `meanL`, so its input stays three
   channels; in encoded space `meanL`/`p01`/`p99` are the Rec. 709 weighted sum of encoded channels (luma). The default
   (linear) is photometric as before.
6. **Boxes and points outside the frame** (§3.7, §10): checked at intern time for arguments declared `InFrame`
   (`stats.box`, `mask.box`, `mask.ellipse`, `mask.flood`). A box is refused only when it does not overlap the frame
   (a zero-area box passes when its corner lies in the closed frame); gradient points may lie outside. The fix is a
   literal call with the nearest valid value, e.g. `img.stats({ box: [100, 50, 10, 10] })`.
7. **Usage lines omit the `space` option** (`signatureHint`, `goja-cv ops`, the reference's usage line); `cv.help(op)`
   lists it under `args` and as `space: "<default> (default; …)"`, and unknown-option errors list it as valid.
8. **Enum errors carry a literal fix** `op({ name: "value" })`: `mask.gradient.from` is now checked by the binder (type
   `point|enum:top,bottom,left,right`) before the op's own validation could add its fix.
9. **`box` on a file with a recorded origin** (§7.4): the frame origin is the recorded origin plus the box corner (a
   window of a window keeps the scan's coordinates); without a recorded origin it is the box corner, as written.
10. **Decoder extras**: the TIFF reader also accepts big-endian files, `SampleFormat` 3 with LZW/Deflate (no float
    predictor), one-channel LinearRaw, and two/four samples per pixel with the extra sample dropped; the NPY reader
    also accepts `>f8`. Before allocating, both check that the file can hold the declared data (a malformed header
    cannot ask for gigabytes). `Info` keeps an unexported `scaled` flag so that `Linearize` uses the exact integer
    tables only for plain codes (not after DNG black/white levels).
11. **Source store key**: `SourceStore.Load(ctx, SourceKey{Path, Stamp, Box, Enc})`, so a window and an encoding
    override are separate sources (§9.1 sketches `Load(abs, stamp, box)`).
12. **Enum fix for `cv.open`**: `cv.open(path, { encoding: "srgb" })` (the first valid value or the did-you-mean
    suggestion), following deviation 8.
13. **Macro node inputs** (§7.8): a macro node's inputs are all of the call's inputs (receiver, explicit inputs,
    lifted arguments) followed by the expansion's output, not only the receiver and the output, so that lifted
    arguments keep their `InputRef` positions in the canonical args (hash and `Describe`). `Expander.Arg(args, name)`
    turns a lifted argument back into a node argument for the inner calls. `Builder.Call` (and `Engine.Call`) return
    the `CallResult`; `Builder.Intern` stays for single-node calls; `CallResult.Node()` is the single node or nil.
14. **`OpDef.Warn`** is a build-time warning hook (used by `toLab` for `PRIMARIES`, found by walking the inputs to the
    opened files' notes).
15. **`channel` canonical form**: `channel(2)` and `channel("b")` are different nodes (the argument is hashed as given);
    `split()` uses the names. The channel count is not static, so "g"/"b" on a one-channel image is an `E_ARG_RANGE`
    at evaluation, and `colormap` on three channels an `E_WRONG_KIND` at evaluation, both with the guide's fixes.
16. **Colormap tables** are generated: viridis and magma from Matt Zucker's CC0 degree-6 polynomial fits (within
    about 0.01 of matplotlib), diverging from five ColorBrewer RdBu anchors (blue–white–red), gray linear.
