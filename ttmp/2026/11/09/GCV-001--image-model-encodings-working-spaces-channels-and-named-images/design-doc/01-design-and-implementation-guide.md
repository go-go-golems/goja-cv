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
    - Path: pkg/cv/buffer/io.go
      Note: decode always applies the sRGB curve (Linear8/Linear16); ToImage always encodes sRGB; replaced by a decoder registry and Quantize
    - Path: pkg/cv/buffer/color.go
      Note: transfer functions and LUTs; become the single conversion point (buffer.Convert)
    - Path: pkg/cv/buffer/buffer.go
      Note: Buffer gains a Space tag; DocW/DocH pairs elsewhere become Frame
    - Path: pkg/cv/graph/registry.go
      Note: OpDef.Space string (documentation only) becomes a typed SpaceRule plus Result encoding and Expand (macros)
    - Path: pkg/cv/graph/node.go
      Note: Node gains Enc, Space, Frame; Base (E_STALE) is removed
    - Path: pkg/cv/graph/args.go
      Note: binder resolves the space option, inserts convert nodes, new Level unit and SPACE warning
    - Path: pkg/cv/graph/engine.go
      Note: Open takes options (encoding, as, box, origin) and probes through the decoder registry
    - Path: pkg/cv/graph/explain.go
      Note: SourceInfo reports encoding instead of the assumed sRGB colorSpace
    - Path: pkg/cv/kernels/tone.go
      Note: Levels and Contrast convert to perceptual privately; become space-agnostic
    - Path: pkg/cv/kernels/mask.go
      Note: PerceptualLuminance, ColorMask, FloodMask convert privately; become space-agnostic
    - Path: pkg/cv/kernels/measure.go
      Note: Histogram bins perceptual values privately
    - Path: pkg/cv/kernels/draw.go
      Note: AbsDiff works in perceptual space privately (removed in GCV-002)
    - Path: pkg/cv/kernels/specks.go
      Note: DetectSpecks mixes perceptual detection and linear MeanL
    - Path: pkg/cv/ops/source.go
      Note: the open op; gains encoding, box, origin
    - Path: pkg/cv/ops/tone.go
      Note: toGray is the only channel op today
    - Path: pkg/cv/look/look.go
      Note: Render converts with ToImage (sRGB); renders encoded values after this ticket
    - Path: pkg/cvsession/init.js
      Note: img is a special-cased global; generalized to named images
    - Path: pkg/cvsession/native.go
      Note: getImg/setImg; becomes a named-image table
    - Path: pkg/cvsession/processor.go
      Note: context block lists opened images; adds names and encodings
    - Path: pkg/cvsession/mcp.go
      Note: the open MCP tool takes only a path today
    - Path: pkg/cv/docs/topics/02-units-and-scales.md
      Note: the Linear and perceptual section is rewritten as Encodings and spaces
    - Path: internal/testimage/planted.go
      Note: the planted scan; source of the new fixtures (linear TIFF, float TIFF, npy, DNG)
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/82-export-cv-tiles.py
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/dustds/rawio.py
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/04-intern-guide-part-3-implementing-cv-in-go-go-goja.md
Summary: "Intern guide for GCV-001: what encodings and working spaces are, why goja-cv's hard-wired sRGB model fails on scientific data, and how to replace it with tagged encodings, one evaluator-resolved space option, linear/float/npy/DNG decoders, frames, channel ops and named images."
LastUpdated: 2026-11-09T12:00:00-05:00
WhatFor: "Implementing GCV-001 (the first of GCV-001..003)."
WhenToUse: "Read before touching buffer IO, op spaces, the open op, channels or session image bindings."
---

# GCV-001 — Image model: encodings, working spaces, channels and named images

This guide is for an intern who will implement GCV-001 in goja-cv. It assumes you have read the CVJS-001
intern guide, Parts 1–3 (`design-doc/02` to `04` in the CVJS-001 ticket), or at least its vocabulary table:
node, kind, op, evaluation, scale, doc px, look, kernel, buffer. Where goja-cv's implementation diverged from
that guide, this document says so.

GCV-001 is the first of three tickets. GCV-002 (inspection views and looks) and GCV-003 (measurement and
data out) both rely on the image model defined here: every view and every measurement must know in which
space its numbers live. Implement GCV-001 first.

## 1. Executive summary

goja-cv decodes every file as if it held sRGB-encoded light and converts it to linear light at load time. Each
kernel then converts back to a perceptual scale privately when it needs one. That model is correct for
photographs and wrong for everything else: linear scans, float data, density maps, depth maps, `.npy` arrays.
The colleague who uses goja-cv to prepare film-density tiles for vision-model subagents currently stores
`sRGB_encode(density)` in 16-bit TIFFs only so that goja-cv's decode undoes the curve.

This ticket replaces the hard-wired model with three explicit ideas:

1. An **encoding** is a property of image data: the transfer function between the values a file stores (and a
   look shows) and values proportional to the measured quantity. goja-cv knows two: `srgb` and `linear`.
   Decoders report the encoding of what they read; `cv.open(path, { encoding })` overrides it.
2. A **working space** is where an op does its arithmetic: `linear`, `encoded` or `perceptual`. Every op that
   reads pixel values gets one `space` option, resolved when the node is built. The builder inserts explicit,
   cached `convert` nodes where an input is in another space. Kernels never convert.
3. **Frames** and **named images**: an image knows where it sits in document coordinates (so a window read
   from a large DNG keeps its raw coordinates), and a session can hold several opened images under names
   (`cv.open(path, { as: "ref" })`), all listed in the context block.

Along the way the ticket adds decoders for linear and float TIFF, `.npy` and VueScan's linear DNG, channel ops
(`channel`, `split`, `toLab`, `chroma`, `colormap`, `encode`), and a small macro mechanism (`OpDef.Expand`)
that GCV-002 uses for its views. It removes the per-kernel conversions, the documentation-only `Space` string
on `OpDef`, the assumed-sRGB metadata, and a few dead fields.

## 2. Problem statement and scope

### 2.1 The workflow this serves

The colleague's project (DUST-002) annotates dust, hairs and scratches on film scans with vision-language
model (VLM) subagents. Each subagent gets one 1024 × 1024 px tile and uses goja-cv through MCP as a darkroom:
it builds enhanced views in which faint specks stand out, gets a list of places worth a close look, and
judges magnified crops by eye. The playbook is `reference/04-goja-cv-dust-playbook.md` in DUST-002; the
handover issues are `reference/07-goja-cv-handover-issues.md`.

The tiles are cut from VueScan DNG scans of colour negatives. The quantity that matters is film **density**,
D = −log10(I / I_base), normalized per tile to v ∈ [0, 1]. Dust blocks light, so it raises density, and a
speck of a given opacity adds the same density step on a thin and on a dense part of the film. The export
script (`scripts/82-export-cv-tiles.py`, docstring lines 1–15) explains the trick the colleague needed:

```python
# 82-export-cv-tiles.py, lines 60-63
v = np.clip((D - lo) / rng, 0, 1)
if not a.perceptual:
    v = np.where(v <= 0.0031308, 12.92 * v, 1.055 * np.power(v, 1 / 2.4) - 0.055)   # sRGB_encode(v)
tifffile.imwrite(p, (v * 65535 + 0.5).astype(np.uint16), photometric="rgb")
```

They store sRGB_encode(v) so that goja-cv's decode, which always applies the inverse sRGB curve, recovers
v as the "linear" value on which blur, blend and stats run. Four handover issues follow from this:

| issue | title | priority | what it asks |
|---|---|---|---|
| 6 | Open linear and scientific inputs | P1 | `cv.open(path, { encoding })`; 16-bit linear TIFF, float TIFF, VueScan DNG, `.npy`; encoding in the context block |
| 7 | Choose the working space for filters and stats | P2 | a `{ space: "linear" \| "encoded" }` option; `explain()` and captions state the space |
| 13 | Channels, colour spaces and false colour | P2 | `channel`, `split`, `toLab`, `chroma`, `colormap` |
| 20 | Several named images per session | P3 | `cv.open(path, { as: "ref" })`, listed in the context block |

### 2.2 Scope

In scope: the image model (encodings, spaces, frames), decoders for the listed formats, the `space` option
on every pixel-reading op, the channel ops, named images, the macro mechanism, and the documentation an agent
reads. Out of scope: ICC colour management (files with embedded profiles are still treated by their transfer
curve only), camera-raw demosaicing (only DNGs whose data is already `LinearRaw` are read), writing files
(GCV-003, `node.save`), crops and zoomed looks (GCV-002), signed arithmetic (GCV-002).

## 3. Concepts

This section builds the vocabulary in dependency order. Each idea is defined before it is used, with numbers
you can check with a calculator.

### 3.1 Code values and quantities

A 16-bit image file stores integers between 0 and 65535 per channel. These integers are **code values**. A
code value by itself says nothing about the physical quantity it stands for; the file format, a tag, or a
convention supplies that meaning. goja-cv normalizes code values to [0, 1] by dividing by the largest code
(255 or 65535); float files store their values directly. In this guide "encoded value" always means this
normalized code value.

The **quantity** is what the data measures: light intensity for a photograph, film density for the
colleague's tiles, depth for a depth map. A quantity is **linear** when doubling it means "twice as much":
twice the photons, twice the density.

### 3.2 Encodings and transfer functions

An **encoding** is the function that maps an encoded value e to a linear value x. goja-cv needs two:

- `srgb`: the sRGB transfer function. x = e / 12.92 for e ≤ 0.04045, else x = ((e + 0.055) / 1.055)^2.4.
  Its inverse, used to encode, is e = 12.92 · x for x ≤ 0.0031308, else e = 1.055 · x^(1/2.4) − 0.055.
  `pkg/cv/buffer/color.go:29-50` implements both, extended to all reals (odd-symmetric below zero).
- `linear`: the identity. x = e.

Photographs (PNG, JPEG, ordinary 8/16-bit TIFF) are almost always sRGB-encoded, because the sRGB curve spends
code values where human vision distinguishes small differences, in the shadows. Scanner raw data, float
TIFFs and NumPy arrays are almost always linear.

A few values make the curve concrete:

| encoded e | linear x (srgb) | linear x (linear) |
|---|---|---|
| 0.05 | 0.0039 | 0.05 |
| 0.5 | 0.214 | 0.5 |
| 0.8 | 0.604 | 0.8 |

So the same code value 32768 (e = 0.5) means 21 % of white in an sRGB file and 50 % in a linear file. A reader
that guesses wrong is off by a factor of 2.3 in the midtones.

### 3.3 Three working spaces

A kernel computes on numbers. The **working space** of an op is the representation those numbers are in when
the kernel sees them. goja-cv uses three:

| space | definition | where it is the right scale |
|---|---|---|
| `linear` | x: values proportional to the quantity (the encoding undone) | averaging, blurring, scaling, compositing light; photometric measurement |
| `encoded` | e: the normalized code values, as the file stores them and as a look shows them | thresholds and contrast "as you see it"; data whose stored value is already the meaningful scale |
| `perceptual` | p = sRGB_encode(x): the sRGB curve applied to linear values, whatever the file's encoding | comparing lightness across images of different encodings |

Two identities follow from the definitions, and they matter for performance and for explaining the model:

- For an `srgb` image, encoded = perceptual. Every op that today runs "in perceptual space" runs in encoded
  space without any numerical change.
- For a `linear` image, encoded = linear. Opening the colleague's density tiles with `encoding: "linear"`
  makes every op, whatever its space, see v itself.

The table below follows one pixel through the three spaces for three kinds of file.

| file | stored code | encoded e | linear x | perceptual p |
|---|---|---|---|---|
| sRGB photo (16-bit TIFF) | 32768 | 0.5 | 0.214 | 0.5 |
| linear scan (16-bit TIFF, `encoding: "linear"`) | 32768 | 0.5 | 0.5 | 0.735 |
| today's density tile (stores sRGB_encode(v)), v = 0.3 | 38261 | 0.584 | 0.300 | 0.584 |
| density tile stored as v, opened `linear`, v = 0.3 | 19661 | 0.300 | 0.300 | 0.584 |

The last two rows are the same density. With this ticket the colleague can store v directly and open with
`encoding: "linear"`; linear and encoded then both equal v.

### 3.4 Why the space changes results

Spaces are not a formality. The colleague measured that their speck ranking in a high-pass view got two to
three times worse when the filter ran in the wrong space (issue 7). The reason is visible with two numbers.

Take a dust speck that adds a density step Δv = 0.05. On a thin part of the film (v = 0.10) and on a dense
part (v = 0.60) the step is the same in density units, because density is the linear quantity here. If the
same data were viewed through the sRGB curve, the step would be:

```
dark background:  sRGB_encode(0.15) − sRGB_encode(0.10) = 0.4236 − 0.3492 = 0.0744
light background: sRGB_encode(0.65) − sRGB_encode(0.60) = 0.8267 − 0.7977 = 0.0289
```

The same physical speck looks 2.6 times stronger on the dark background. A detector or a ranked list built in
that space prefers specks in thin areas and grain in thin areas alike. The correct space is the one in which
the quantity is linear; for photographs of light it is linear light, for density tiles it is density.

Blur makes the same point from another side. A Gaussian blur averages neighbours. Averaging two pixels of light
0.0 and 1.0 gives 0.5 linear, which displays as sRGB 0.735; averaging their encoded values gives 0.5 encoded,
which is 0.214 linear. Optics averages light, so blurring a photograph should happen in linear space; a
density map should be averaged in density.

### 3.5 Light and derived quantities

Some ops do not transform light; they compute a new quantity from it. The difference of two images, a
high-pass residual, a local contrast ratio, a chroma value, Lab coordinates: none of these is "light", and
pushing them through a transfer function is meaningless. sRGB_decode of a difference of 0.05 encoded units is
0.0039, a number that describes nothing.

This guide calls such outputs **derived**. A derived image is tagged with the `linear` encoding: its stored
value is its value, so linear, encoded and the displayed grey level all coincide. The rule an agent can rely
on is "for derived images, what you measure is what you see". Masks follow the same rule; a mask value of 0.5
displays as grey 128, not as sRGB 0.735 (which is what `pkg/cv/buffer/io.go:182-190` does today).

### 3.6 Frames

A **frame** is the rectangle an image covers in document coordinates: an origin (x, y) and a size (w, h), in
doc px. Today every image's frame has origin (0, 0) and the size of its source file; goja-cv only stores the
size (`DocW`, `DocH` on nodes and values). Two cases need a non-zero origin. Reading a 1024 px window out of
a 7256 × 7326 DNG should keep the raw coordinates, so that a speck found at (1337, 131) is at (1337, 131)
in the scan, not at (313, 131) in the window. Opening an exported tile whose top-left is raw (1024, 0) should
be able to say so. GCV-002's crop is the third producer of frames.

### 3.7 Channels and colormaps

A colour image has three channels. Several inspection cues live in one channel or in the relation between
channels: dust on colour film is neutral (equal density step in all three channels), colour grain is not.
The colleague measured chroma ≈ 0.2 for confirmed dust against ≈ 0.8 for rejects (issue 10). A **colormap**
maps a single-channel value to a display colour; a diverging map (blue–white–red) shows the sign of a signed
map at a glance, which grey cannot.

### 3.8 Named images

A session can hold several opened images. A **name** is a global the agent can use for one of them: `img` is
the first, `ref` could be a cleaned version or another scan of the same frame. Ops already accept nodes built
from different files as long as their sizes match; names only make the images reachable and visible in the
context block.

The key points:

- An encoding belongs to the data; a working space belongs to the op. Confusing the two is the bug this ticket
  removes.
- For sRGB images encoded and perceptual coincide; for linear images encoded and linear coincide.
- The space in which the measured quantity is linear is the right space for averaging and for thresholds that
  must mean the same everywhere.
- Derived values carry the linear encoding, so their numbers and their grey levels agree.
- A frame places an image in document coordinates; most images have origin (0, 0).

## 4. Current-state analysis

Every claim below is anchored to the code at the commit this ticket was written against (`175b5d4`). Read the
cited lines; the guide only summarizes them.

### 4.1 Decoding always applies the sRGB curve

`buffer.Decode` (`pkg/cv/buffer/io.go:67-133`) switches on the Go image type and converts every sample through
`Linear8` or `Linear16` (`io.go:89`, `:101`, `:124`, `:144`, `:160-162`). These are the sRGB decode tables from
`color.go:5-27`. There is no branch for linear data and no way to ask for one. `DecodeConfigFile`
(`io.go:30-51`) reads the header through `image.DecodeConfig`, which only knows PNG, JPEG and TIFF formats that
`golang.org/x/image/tiff` supports. That package rejects float samples (`SampleFormat` ≠ 1, x/image
`tiff/reader.go:252-262`) and unknown photometric interpretations such as DNG's `LinearRaw` (34892).

`Engine.Open` (`pkg/cv/graph/engine.go:87-120`) probes the header and stamps format, bit depth and channel count
into hidden arguments of the `open` node; its error hint says "Supported formats: PNG, JPEG, TIFF (8/16-bit)"
(`engine.go:113-114`). The `open` op's doc string reads "decoded on first evaluation, sRGB → linear float32"
(`pkg/cv/ops/source.go:34`). `SourceInfo` reports `"colorSpace": "sRGB (assumed; ICC profiles are not read)"`
for every file (`pkg/cv/graph/explain.go:21`).

### 4.2 The op `Space` field is documentation only

`OpDef.Space` is a string with six possible meanings: `"linear" | "perceptual" | "geometry" | "analysis" |
"source" | "data"` (`pkg/cv/graph/registry.go:106`), and the ops also use `"selection"` and `"present"`. The
only code that reads it is the help renderer, which copies it into the help entry (`pkg/cv/graph/help.go:77-79`).
The evaluator never looks at it (`pkg/cv/graph/eval.go` has no reference to `Space`). The field mixes two
unrelated things: the working space of pixel ops, and a category for everything else. The counts across
`pkg/cv/ops` are 7 linear, 5 perceptual, 3 geometry, 7 analysis, 11 data, 5 selection, 4 present, 3 source.

### 4.3 Kernels convert privately

Because the evaluator does not convert, every kernel that wants a perceptual value converts on its own:

| kernel | file:lines | what it converts |
|---|---|---|
| `Levels` | `pkg/cv/kernels/tone.go:39-54` | black/white and every sample to perceptual, result back to linear |
| `Contrast` | `tone.go:59-68` | every sample to perceptual and back |
| `PerceptualLuminance` | `pkg/cv/kernels/mask.go:41-52` | linear luminance to perceptual (used by `mask.luminance`, `refine`, `detect.specks`) |
| `ColorMask` | `mask.go:124-145` (line 134) | each channel to perceptual before computing hue |
| `FloodMask` | `mask.go:251-270` (lines 262, 266) | seed and every visited pixel to perceptual |
| `Histogram` | `pkg/cv/kernels/measure.go:194-231` (line 216) | bins perceptual values |
| `AbsDiff` | `pkg/cv/kernels/draw.go:100-121` | |P(a) − P(b)| · gain, back to linear |
| `DetectSpecks` | `pkg/cv/kernels/specks.go:166-178` | detects on perceptual luminance, reports `MeanL` from linear luminance (line 175) |
| `parseColor` | `pkg/cv/ops/mask.go:69-106` (line 105) | overlay colours from perceptual to linear |

The histogram op even hard-codes `"space": "perceptual"` into its result (`pkg/cv/ops/measure.go:158`). No
caller can ask any of these for another space, and none of them can be reused on a linear-encoded image without
silently applying the sRGB curve.

### 4.4 Encoding for display and files assumes sRGB

`buffer.ToImage` (`io.go:169-224`) clamps linear values and encodes them with `EncodeSRGB8`/`EncodeSRGB16`
for every buffer, including masks and gray images. `look.Render` (`pkg/cv/look/look.go:180-209`) calls it for
every look. `WriteFile` (`io.go:254-294`) writes 16-bit sRGB TIFFs and PNGs the same way. A mask value of 0.5
therefore displays as code 188 of 255, not 128.

### 4.5 One bound image

The session defines `img` as a getter/setter pair over one slot (`pkg/cvsession/init.js:47-56`,
`pkg/cvsession/native.go:120-132`). `cv.open` returns a node and records it in the binding's `opened` list
(`pkg/gojamod/cv/binding.go:259-271`, `pkg/gojamod/cv/evaltrack.go:197-206`); `img = cv.open(p)` rebinds `img`.
The context block lists all opened files and marks the one bound to `img` (`pkg/cvsession/processor.go:482-498`).
A second image can be used through a `const` (lost after the eval) or a sloppy-mode global, which the context
block does not show. The MCP `open` tool takes only a path (`pkg/cvsession/mcp.go:47`).

### 4.6 Channels: only toGray

The only channel op is `toGray(weights)` (`pkg/cv/ops/tone.go:76-92`), with weights `rec709`, `rec601` and
`average` applied to linear values (`pkg/cv/kernels/tone.go:70-90`). Rec. 601 weights are defined for
gamma-encoded luma, so `toGray("rec601")` mixes conventions. There is no way to take one channel, compute a
colour difference, or display a single-channel value in false colour.

### 4.7 Dead and vestigial pieces in the core

`Node.Base` (`pkg/cv/graph/node.go:21`, set at `:127-129`) exists for an `E_STALE` check of a document layer
that does not exist; nothing reads it. The error codes `EStale`, `EReadOnly`, `EFragmentCapture`, `ENoModel`
and `WScale` (`pkg/cv/graph/errors.go:21-35`) are never used outside their declaration. `OpDef.Halo`
(`registry.go:108`) is declared and filled but never read; GCV-002 gives it its first reader, so it stays.

### 4.8 What the workaround costs

The colleague exported 300 MB of TIFFs for 50 tiles, cannot open the DNGs or their `.npy` caches, and has to
remember which of goja-cv's numbers are density and which are sRGB_encode(density). Issue 7 reports that their
probe helper had to re-apply the sRGB curve in JavaScript to get stats in file units. `stats`, `gaussian` and
`blend` happened to run on v only because of the export trick; `histogram`, `mask.luminance` and
`detect.specks` ran on sRGB_encode(v).

## 5. Gap analysis

| need | today | gap |
|---|---|---|
| Read linear, float, `.npy`, LinearRaw DNG data | PNG/JPEG/TIFF ints, always sRGB | decoders and an encoding per source |
| Say which encoding an image has | `colorSpace: sRGB (assumed)` | encoding in metadata, captions, context block |
| Run any pixel op in a chosen space | fixed per kernel, undocumented in the evaluator | one `space` option resolved at build time |
| Know where a number lives | not recorded | static `Space` on nodes; explain shows it |
| Neutral/colour cues, false colour | `toGray` only | `channel`, `split`, `toLab`, `chroma`, `colormap` |
| Read a window of a large scan in scan coordinates | not possible | frames, `open({ box })` |
| Two images in a session, visible to the agent | one `img` slot | named images |

## 6. Legacy to remove or refactor

The project is early-stage and local-only: no compatibility shims, no deprecated aliases. The items below are
removed or rewritten by this ticket.

1. **Per-kernel colour conversion.** Every `buffer.ToPerceptual` / `buffer.ToLinear` call in `pkg/cv/kernels`
   and `pkg/cv/ops` (table in §4.3) goes. Kernels compute on whatever numbers they are given. A test greps the
   kernel sources so the calls cannot come back.
2. **`OpDef.Space string`.** Replaced by a typed `SpaceRule` (working-space default and whether it is fixed).
   The category values (`geometry`, `analysis`, `data`, `selection`, `present`, `source`) are dropped; help
   already groups ops by receiver kind.
3. **sRGB decode in `buffer.Decode` and sRGB encode in `buffer.ToImage`.** Decoders return encoded values and
   an encoding; `ToImage` becomes `Quantize`, which only scales and clamps encoded values.
4. **`DecodeConfigFile` through `image.DecodeConfig`.** Replaced by `Probe` on a decoder registry.
5. **`colorSpace: "sRGB (assumed; …)"`** in `SourceInfo` and in the `metadata`/`meta`/`info` help redirects
   (`pkg/gojamod/cv/help.go:52-54`). Replaced by `encoding`.
6. **`DocW`, `DocH` pairs** on `Node`, `ImageValue` and `RegionsValue`. Replaced by one `Frame` value.
7. **The single `img` slot** (`getImg`/`setImg`, `noteOpen`/`Opened` as the image list). Replaced by a
   named-image table; `img` is the first name, not a special case.
8. **`"space": "perceptual"` literal** in the histogram result and the "perceptual" wording in docs, help
   strings and the MCP instructions (`pkg/cvsession/reference.go:28-29`). Replaced by the resolved space.
9. **`Node.Base` and the unused codes** `EStale`, `EReadOnly`, `EFragmentCapture`, `ENoModel`, `WScale`.
10. **`toGray("rec601")` on linear values.** Kept as a weight set, but the docs say to pass
    `{ space: "encoded" }` with it; the mixed convention is no longer the default behaviour of anything.
11. **`cv.kinds` hand list** in `pkg/gojamod/cv/prelude.js:307` duplicates `graph.Kinds`; the binding exports
    the list from Go.

## 7. Proposed architecture

### 7.1 Overview

```
 file ──▶ decoder registry ──▶ encoded float32 + Info{encoding, format, bits, sample}
                                   │ source loader: encoded → linear (exact 8/16-bit tables), pyramid in linear
                                   ▼
 open node  (Enc = srgb|linear, Space = linear, Frame = {x, y, w, h})
   │
   │  agent calls  img.mask.luminance({ above: 0.9 })
   ▼
 Builder.Intern ── resolve space (option or op default) ── input in another space?
   │                                                         │ yes: intern convert(input → space)
   ▼                                                         ▼
 mask.luminance node (Space = encoded)  ◀── input ── convert node (cached like any node)
   │
   ▼
 evaluator runs kernels on already-converted buffers; kernels never convert
   │
   ▼
 look: convert(root → encoded), Quantize to 8 bits, encode PNG/JPEG
```

Three rules carry the design:

- Encoding and space are **static**: known when the node is built, recorded on the node, part of its hash.
  Nothing about them is decided at evaluation time.
- Conversions are **nodes**. They are hash-consed and cached like every other node, so two ops that need the
  encoded version of the same input share one conversion, and `explain()` shows every conversion.
- Kernels are **space-agnostic**. A kernel computes on the numbers it receives; which numbers those are is the
  builder's decision.

### 7.2 Types

```go
// pkg/cv/buffer/encoding.go
package buffer

// Encoding is the transfer function between encoded values (what a file
// stores and a look shows) and linear values (proportional to the quantity).
type Encoding uint8

const (
    EncSRGB   Encoding = iota + 1 // sRGB curve
    EncLinear                     // identity
)

func (e Encoding) String() string            // "srgb" | "linear"
func ParseEncoding(s string) (Encoding, bool)

// Space is the representation a buffer's values are in.
type Space uint8

const (
    SpaceNone       Space = iota // not a level: masks, data
    SpaceLinear                  // x: proportional to the quantity
    SpaceEncoded                 // e: normalized code values, as stored and displayed
    SpacePerceptual              // p = sRGB_encode(x), whatever the encoding
)

// Identity reports whether converting from → to is the identity for e
// (encoded = perceptual for srgb, encoded = linear for linear).
func Identity(e Encoding, from, to Space) bool

// Convert returns b's values in space to. b.Space must be set. It returns b
// itself when Identity holds; otherwise a new buffer.
func Convert(ctx context.Context, b *Buffer, e Encoding, to Space) (*Buffer, error)
```

`Buffer` gains one field, `Space Space`, set by every producer. It is a tag for assertions and debugging; the
graph is the authority (§7.5). Conversion table:

| from → to | `srgb` | `linear` |
|---|---|---|
| linear → encoded | sRGB encode | identity |
| encoded → linear | sRGB decode | identity |
| linear → perceptual | sRGB encode | sRGB encode |
| encoded → perceptual | identity | sRGB encode |
| perceptual → encoded | identity | sRGB decode |
| perceptual → linear | sRGB decode | sRGB decode |

All non-identity conversions go through the existing extended transfer functions and LUTs in `color.go`, which
stay. `Linear8`, `Linear16`, `EncodeSRGB8`, `EncodeSRGB16` remain as table helpers used by `Convert` and by the
source loader's exact 8/16-bit decode.

```go
// pkg/cv/graph: frames replace DocW/DocH pairs
type Frame struct {
    X, Y float64 // origin in doc px
    W, H int     // size in doc px
}

type Node struct {
    ID        NodeID
    Op        *OpDef
    Inputs    []NodeID
    NExplicit int
    Args      Args
    Kind      Kind
    Hash      [32]byte
    Frame     Frame          // was DocW, DocH (and Base, removed)
    Enc       buffer.Encoding // Image nodes; 0 for other kinds
    Space     buffer.Space    // Image: space of the values; Value: space of level-valued numbers; else SpaceNone
}

type ImageValue struct {
    Buf   *buffer.Buffer
    Frame Frame
    Enc   buffer.Encoding
    kind  Kind
}
```

`RegionsValue` swaps `DocW, DocH` for `Frame` as well. Region geometry stays in doc px of the root frame, so a
region found on a window of a DNG has raw coordinates (§7.9).

### 7.3 Decoders

```go
// pkg/cv/buffer/decode.go
type Info struct {
    Format   string          // "png", "jpeg", "tiff", "dng", "npy"
    W, H     int
    Channels int             // 1 or 3 after decoding (alpha is dropped)
    Bits     int             // 8, 16 or 32 per sample in the file
    Sample   string          // "uint" | "float"
    Encoding Encoding        // what the file implies (overridable by cv.open)
    Notes    []string        // e.g. "camera RGB primaries (DNG)", "LZW"
}

type DecodeOptions struct {
    Box *image.Rectangle // read only this window (file px); nil = whole image
}

type Decoder interface {
    Name() string
    Sniff(head []byte) bool                                   // magic bytes
    Probe(r io.ReadSeeker) (Info, error)                      // header only
    Decode(ctx context.Context, r io.ReadSeeker, o DecodeOptions) (*Buffer, Info, error) // encoded values
}

func Register(d Decoder)
func ProbeFile(path string) (Info, error)
func DecodeFile(ctx context.Context, path string, o DecodeOptions) (*Buffer, Info, error)
```

The registry replaces `Decode`, `DecodeFile` and `DecodeConfigFile` in `io.go`. Decoders return **encoded**
values: integers divided by their maximum, floats as stored. The source loader (`graph.SourceStore.Load`,
`pkg/cv/graph/sources.go:34-61`) converts to linear once, with the exact 8/16-bit tables for integer files,
and builds the pyramid in linear values (area averaging of light, as today). The source node's `Space` is
therefore `linear`.

| decoder | formats | encoding implied | notes |
|---|---|---|---|
| `png` | 8/16-bit gray, RGB, palette | srgb | stdlib `image/png`; alpha dropped |
| `jpeg` | baseline/progressive | srgb | stdlib; EXIF orientation not applied (as today) |
| `tiff` | own IFD parser; uncompressed 8/16-bit uint and 32-bit float, chunky, 1 or 3 samples; Photometric 1, 2, 34892 (LinearRaw); SubIFDs | srgb for uint Gray/RGB, linear for float and LinearRaw | compressed strips (LZW, Deflate, PackBits) are handed to `golang.org/x/image/tiff`; `Box` reads only the strips it needs when uncompressed |
| `npy` | NumPy v1–v3, C order, `\|u1`, `<u2`, `>u2`, `<f4`, `>f4`, `<f8`; shape (H, W), (H, W, 1), (H, W, 3) | linear | f8 is narrowed to float32; Fortran order is `E_FORMAT` |

The DNG case is part of the TIFF decoder, because a DNG is a TIFF. The VueScan DNGs are well described by the
dataset code (`dustds/rawio.py:1-46`, and `~/dustds/meta/dng_meta.jsonl`): the full-resolution image is the
IFD with `NewSubfileType` (254) = 0 and `PhotometricInterpretation` (262) = 34892, either top-level or inside the
`SubIFDs` (330) of IFD0; 16 bits, 3 samples, uncompressed, contiguous strips; no `BlackLevel` (50714),
`WhiteLevel` (50717) or `LinearizationTable` (50712), so the decoded value equals the stored value / 65535. An
example from the metadata file: `raw0027.dng`, 7256 × 7326, data offset 8, VueScan 9.8.39, Perfection V600. The
decoder applies `BlackLevel`/`WhiteLevel` when present ((v − black) / (white − black)), refuses
`LinearizationTable` and colour-filter-array data (Photometric 32803) with `E_FORMAT`, and notes "camera RGB
primaries" in `Info.Notes` (the colour matrices are not applied).

Files that goja-cv itself writes (GCV-003's `save`) record their encoding and frame: TIFFs carry
`goja-cv encoding=linear origin=1536,1536` in `ImageDescription` (270), PNGs a `gAMA` chunk of 1.0 for linear
data or an `sRGB` chunk. The TIFF and PNG decoders read these back into `Info.Encoding` and the frame origin, so a
saved derived view reopens as it was. Other files' descriptions are ignored.

A full VueScan DNG decodes to 7256 × 7326 × 3 float32 = 638 MB, plus about a third more for the pyramid.
`DecodeOptions.Box` makes the tile workflow cheap: an uncompressed TIFF window is read row by row with seeks,
so a 1024² window costs 6 MB of reads.

### 7.4 Opening: `cv.open(path, options)`

```js
img = cv.open("/Users/manuel/dustdet/cv_tiles/1d955ef4b27d3d88_1536_1536.tif")          // encoding from the file: srgb
const d = cv.open("tile.npy")                                                            // linear (npy default)
cv.open("raw0027.dng", { as: "scan", box: [1536, 1536, 1024, 1024] })                    // linear, frame origin (1536, 1536)
cv.open("tile_v.tif", { encoding: "linear", origin: [1536, 1536], as: "img" })           // stored v, raw coordinates
```

| option | type | meaning |
|---|---|---|
| `encoding` | `"srgb" \| "linear"` | override what the file implies |
| `as` | identifier | bind the image to this global (§7.10); session-only, not part of the node hash |
| `box` | `[x, y, w, h]` file px | read only this window; the frame origin becomes (x, y) |
| `origin` | `[x, y]` doc px | declare the frame origin of a file that is itself a window (not with `box`) |

`open` stays a registry op (`KindNone`, `pkg/cv/ops/source.go:11-36`) with `encoding`, `box` and `origin` as
declared arguments, so unknown options, synonyms and did-you-mean work through the binder like for every other
op. `as` is declared too, but `Engine.Open` removes it before interning and returns it to the binding:

```go
// pkg/cv/graph/engine.go
type OpenOptions struct {
    Encoding buffer.Encoding // 0 = from the file
    Box      *[4]int
    Origin   *[2]float64
}

func (e *Engine) Open(path string, raw map[string]any) (*Node, string /*as*/, error)
```

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

### 7.5 Working spaces: one option, resolved by the builder

**SpaceRule.** Each op declares how it relates to pixel values:

```go
// pkg/cv/graph/registry.go
type SpaceRule struct {
    Default buffer.Space // working space when the call passes no { space }; SpaceNone: the op does not read levels
    Fixed   bool         // the op only makes sense in Default (toLab: linear); no option is offered
}

// ResultEnc says what encoding an Image result has.
type ResultEnc uint8

const (
    KeepEnc    ResultEnc = iota // light in, light out: the receiver's encoding (exposure, gaussian, blend, …)
    DerivedEnc                  // a new quantity: linear encoding, values as computed (chroma, toLab, GCV-002 sub/highpass)
    DisplayEnc                  // display colours: srgb, values are encoded (colormap)
)

type OpDef struct {
    // … existing fields, minus Space string …
    Space  SpaceRule
    Result ResultEnc
    Expand ExpandFunc // macros, §7.8
    Internal bool     // not callable from JavaScript, not listed in help (convert)
}
```

**The option.** `Registry.Register` appends one argument to every op whose `Space.Default` is not
`SpaceNone` and not `Fixed`:

```go
graph.ArgSpec{Name: "space", Type: "enum:linear,encoded,perceptual", Optional: true, Static: true,
    Doc: "working space of this op: linear (proportional to the quantity), encoded (the file's values, what looks show) or perceptual (sRGB curve of linear)"}
```

`Static` means the value must be a literal; a param cannot move it, because it changes the graph's shape.

**Resolution at build time.** `Builder.Intern` (`pkg/cv/graph/node.go:67`) binds arguments as today, then:

1. `s := args.space` if given, else `op.Space.Default`. It stores `s` in the canonical args, so the hash, the
   cache signature and `explain()` include it.
2. For each explicit input of kind `Image` (the receiver and `Inputs` of kind Image, plus kind-typed Image
   arguments), if `input.Space != s` and `!buffer.Identity(input.Enc, input.Space, s)`, the input is replaced by
   `convert(input, s)` (interned, so shared).
3. The new node's `Space` and `Enc` follow from `op.Result`:
   - `KeepEnc`: `Enc = receiver.Enc`, `Space = s` (or the receiver's space when `Default` is `SpaceNone`).
   - `DerivedEnc`: `Enc = linear`, `Space = linear`; the values are reinterpreted, not converted.
   - `DisplayEnc`: `Enc = srgb`, `Space = encoded`.
   - Value results: `Space = s` when the op reports levels (stats, histogram edges, GCV-003's sample); fields
     derived from a Value inherit its `Space`.

Masks are weights, not light: Mask nodes have `Space = SpaceNone` and are never converted.

**Defaults.** The table fixes the default of every existing op. For sRGB images, every op that used to be
"perceptual" now defaults to `encoded`, which is numerically the same, so their behaviour does not change.

| op | default | reason |
|---|---|---|
| `exposure`, `gaussian`, `blend`, `within`, `heal`, `inpaint` | linear | they model light: scaling, averaging, compositing |
| `toGray` | linear | Rec. 709 weights are defined on linear light |
| `stats` | linear | photometric measurement; matches `levels` |
| `levels` | linear | `levels({ black: s.p01, white: s.p99 })` with `s = img.stats()` keeps working |
| `contrast` | encoded | contrast around the displayed mid-grey |
| `histogram` | encoded | bins over the scale the look shows |
| `mask.luminance`, `mask.color`, `mask.flood`, `refine` | encoded | thresholds and tolerances "as you see it" |
| `detect.specks`, `check.dust` | encoded | contrast as seen; `MeanL` is reported in the same space |
| `overlay` colours, `compare` | encoded | colours are display values |

`levels` is the one visible behaviour change on sRGB images: the remap now happens in linear space and the
gamma is applied to the linear ramp. With `black = 0` the displayed result is sRGB_encode(v / w) instead of
today's sRGB_encode(v) / sRGB_encode(w). Because the sRGB curve is close to a power law the two agree well
when `white` is high, and drift apart when `white` is low and in the deep shadows, where the curve's linear toe
breaks the power law. Measured over v ∈ (0, w], in 8-bit codes:

| white w | largest difference | largest difference for v/w in 20–80 % |
|---|---|---|
| 0.92 | 0.5 | 0.3 |
| 0.7 | 2.1 | 1.2 |
| 0.5 | 4.5 | 2.5 |
| 0.3 | 8.5 | 4.9 |

With `black > 0` shadows come out slightly darker. The docs examples that print post-levels numbers change
(§11).

**The convert op.** It is an ordinary registered op, marked `Internal`:

```go
{
    Name: "convert", On: graph.KindImage, Returns: graph.KindImage, Internal: true,
    Args: []graph.ArgSpec{{Name: "to", Type: "enum:linear,encoded,perceptual", Positional: true}},
    Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
        src := image(in[0])
        out, err := buffer.Convert(ctx, src.Buf, src.Enc, buffer.MustSpace(a.String("to")))
        if err != nil {
            return nil, err
        }
        return graph.NewImage(out, src.Frame, src.Enc), nil
    },
}
```

### 7.6 Levels and the SPACE warning

`DocPx` taught the binder that some numbers are lengths. This ticket adds a unit for numbers that are
**levels**: values on the intensity scale of the op's working space.

```go
const (
    Unitless Unit = iota
    DocPx
    Fraction
    DocPx2
    Level // a value in the op's working space (levels.black, mask.luminance.above, colormap.range, …)
)
```

When a `Level` argument is a lifted Value node whose static `Space` differs from the op's resolved space, the
binder emits a warning. It does not convert the number: a percentile would survive conversion, a mean would not,
and the binder cannot know which one it has.

```yaml
warnings:
  - "SPACE line 2: levels.black comes from stats() in linear space, but levels runs in encoded space here. fix: img.stats({ space: \"encoded\" })"
```

### 7.7 Looks and display

A look renders encoded values. `look.Render` evaluates `convert(node → encoded)` (a no-op for derived images
and for most display chains), then `buffer.Quantize(buf, 8)` clamps to [0, 1] and scales to 255. Concretely:

- sRGB images look exactly as before, because encoded = sRGB_encode(linear), which is what `ToImage` computed.
- Linear-encoded images show their stored values linearly. A density tile opened with `encoding: "linear"`
  shows v as grey level v.
- Masks and derived images show their values linearly (mask 0.5 → grey 128).

A linear-encoded photograph (a demosaiced linear raw) looks dark when its values are shown linearly. The op
`img.encode("srgb")` re-encodes it: same linear light, `Enc = srgb`, so looks and encoded-space ops see the
sRGB curve. `encode` is `Space: {Default: linear, Fixed: true}` with a `KeepEnc` result whose `Enc` comes from
its argument. It never changes linear values; to reinterpret a file, open it with another `encoding`.

### 7.8 Macros: `OpDef.Expand`

Some ops are best defined as a small graph of other ops. `split()` returns three channel nodes; GCV-002's
`highpass`, `dog`, `compare({ mode: "diff" })` and `view` presets are sub/gaussian/levels chains. A macro is an
`OpDef` whose `Expand` builds that graph at intern time:

```go
// ExpandFunc builds the op's graph from primitive ops. It returns one node
// or a list of nodes (split). The macro node itself is never interned.
type ExpandFunc func(x *Expander, recv *Node, args Args) ([]*Node, error)

type Expander struct{ /* builder and registry */ }

func (x *Expander) Call(name string, recv *Node, args ...any) (*Node, error) // interns name on recv with args
```

Macros are registered like ops, so they get help, option checking, synonyms and did-you-mean for free. Because
they expand at build time, `explain()` and look captions show the primitive chain that actually ran, which is
what issue 22 asks for. The binding returns an array of handles when `Expand` returns several nodes; the
prelude turns it into an array of node proxies.

### 7.9 Channel ops

| call | returns | space | result |
|---|---|---|---|
| `img.channel("r" \| "g" \| "b" \| 0 \| 1 \| 2)` | 1-channel Image | none (plane selection) | keeps encoding and space |
| `img.split()` | `[r, g, b]` (macro over `channel`) | — | — |
| `img.toLab()` | 3-channel Image: L*/100, a*/100, b*/100 | fixed linear | derived |
| `img.chroma({ relative })` | 1-channel Image: max − min over channels (÷ mean when `relative`) | encoded | derived |
| `img.colormap(name, { range, center })` | 3-channel Image | encoded | display |
| `img.encode("srgb" \| "linear")` | Image | fixed linear | re-encoded |

Details an implementer needs:

- `channel` on a 1-channel image with "g" or "b" is `E_ARG_RANGE` with the fix `img.toGray()`.
- `toLab` assumes sRGB/Rec. 709 primaries and the D65 white point. On a DNG source (`notes` contains "camera
  RGB primaries") it emits a `RANGE`-class warning, `PRIMARIES`, saying the a*/b* values are approximate.
- `chroma` is the per-pixel channel spread. With `relative: true` it divides by the channel mean plus 1e-3,
  which matches the colleague's chroma = (max − min) / mean on regions (GCV-003 reuses the kernel).
- `colormap` accepts only 1-channel images (`E_WRONG_KIND` with the fixes `.channel("g")` and `.toGray()`).
  Names: `gray`, `viridis`, `magma`, `diverging` (blue–white–red). `range: [lo, hi]` is in `Level` units; the
  default is the image's [p01, p99] computed at evaluation, and for `diverging` the symmetric range ±max(|p01|,
  |p99|) around `center` (default 0). The colormap tables are 256-entry sRGB lookup tables in a new file
  `pkg/cv/kernels/colormap.go`.

### 7.10 Named images

A **name table** in the session replaces the `img` slot:

```go
// pkg/cvsession/names.go
type nameTable struct {
    order []string                 // binding order, for the context block
    nodes map[string]goja.Value    // name → node proxy (owner goroutine only)
}

var reservedNames = []string{"cv", "P", "mem", "show", "doc", "globalThis", "Math", "JSON", "Promise", "console"}
```

- `cv.open(path, { as: "ref" })` validates the name (`^[A-Za-z_$][A-Za-z0-9_$]*$`, not reserved, not an
  existing global that the table does not own) and defines a getter/setter on `globalThis` that reads the table.
  Errors use a new code `E_NAME`: `cv.open(…, { as: "show" }): "show" is a session global. fix: { as: "show2" }`.
- `img` is the first entry of the table, bound by `--image` or `cv.open(path, { as: "img" })`. `img =
  cv.open(p)` keeps working: the setter rebinds the name, as today.
- Re-opening with an existing name rebinds it.
- The context block lists every name with file, size, format and encoding, in binding order:

```yaml
context:
  images: img = 1d955ef4b27d3d88_1536_1536.tif (1024×1024, 16-bit TIFF, srgb), scan = raw0027.dng [1536,1536,1024,1024] (16-bit DNG, linear)
```

- The MCP `open` tool gains `encoding` and `as` fields (`pkg/cvsession/mcp.go:47`), with schema descriptions
  that say when to omit them (the deep-dive article's lesson about optional parameters).

Ops that take two images (blend, within, compare, GCV-002's sub) check frames instead of buffer sizes: equal
`W` and `H` and equal origin. Mixing encodings in `encoded` space emits an `ENCODING` warning ("blend mixes img
(srgb) and scan (linear) in encoded space; their encoded scales differ. fix: { space: \"linear\" }").

### 7.11 Agent-facing surface

The JavaScript surface after this ticket, as an agent sees it:

```js
img = cv.open("tile_v.tif", { encoding: "linear", origin: [1536, 1536] })
cv.open("clean.tif", { as: "ref" })
const s = await img.stats({ space: "encoded" })                 // numbers in file units
const lum = img.mask.luminance({ above: 0.9 })                   // encoded by default
const neutral = img.chroma({ relative: true }).colormap("viridis")
return {
  enc: (await img).encoding,
  p99: s.p99,
  plan: lum.explain(),
  look: neutral.look("chroma"),
}
```

`cv.help("open")`, `cv.help("space")` and `cv.help("encoding")` must answer. `space` and `encoding` are not ops,
so they become entries in the help table of concepts (§10).

## 8. Decision records

### Decision: encoding is a property of the data, space a property of the op

- **Context:** The current code conflates the two: decode assumes an encoding, kernels assume a space. Issues 6
  and 7 each ask for one of them.
- **Options considered:** (a) a single session-wide "the data is linear" switch; (b) a per-op space option
  only, keeping the sRGB decode; (c) an encoding per source plus a space per op.
- **Decision:** (c).
- **Rationale:** (a) cannot express two images of different encodings in one session (issue 20) and gives
  stats and thresholds no way to differ. (b) keeps the colleague's export trick alive: without an encoding,
  "encoded" for their tile would still be sRGB_encode(v). With (c), the identities in §3.3 make the common cases
  free: sRGB data behaves as today, linear data has linear = encoded.
- **Consequences:** Nodes carry `Enc` and `Space`; decoders report encodings; captions and the context block
  show them. ICC profiles remain unsupported and are the obvious next encoding source.
- **Status:** proposed

### Decision: conversions are graph nodes inserted by the builder

- **Context:** Some layer must convert between spaces. Today each kernel does it privately.
- **Options considered:** (a) each kernel converts (status quo); (b) the evaluator converts inputs before
  `Run`, invisibly; (c) the builder inserts `convert` nodes at intern time.
- **Decision:** (c).
- **Rationale:** (c) makes conversions cached and shared by hash-consing (two encoded-space ops on the same
  input convert once), visible in `explain()`, and testable as ordinary nodes. (b) would need a second cache
  keyed by (node, space) and would hide work from `explain`. (a) is what made the space untestable and
  unchangeable.
- **Consequences:** `Intern` needs static `Space`/`Enc` for every node; the evaluator stays unchanged. Graphs
  grow by one node per distinct conversion.
- **Status:** proposed

### Decision: the space is resolved at build time, per call, with op defaults; no session or image default

- **Context:** Issue 7 suggests either a per-op option or one session-wide setting.
- **Options considered:** (a) per-call option with op defaults; (b) plus a session default (`cv.space(...)`);
  (c) plus an image default (`cv.open(path, { space })`).
- **Decision:** (a).
- **Rationale:** A default that lives outside the call makes the same source text build different graphs
  depending on hidden state, which defeats reading a script. The colleague's case does not need it: opened
  with `encoding: "linear"`, their linear and encoded spaces coincide, so every default is already v. A
  photographer who wants a different space for one op says so in that op.
- **Consequences:** Agents write `{ space }` where they mean it; `explain()` shows non-default spaces. If a
  second user group needs a default, an image-level default (c) can be added without changing nodes, because
  resolution already happens in one place.
- **Status:** proposed

### Decision: derived quantities and masks carry the linear encoding

- **Context:** Differences, ratios, chroma and Lab are not light; converting them through the sRGB curve is
  meaningless, and stats on them must read the values themselves.
- **Options considered:** (a) treat every Image alike and let users pick spaces; (b) a `Derived` result class
  whose output is reinterpreted as linear-encoded.
- **Decision:** (b).
- **Rationale:** With (a), `stats()` (linear by default) of a high-pass built in encoded space returns
  sRGB_decode of differences (0.05 becomes 0.0039), and a look of it shows sRGB_encode of differences. With
  (b), numbers and grey levels agree, and every later op sees the computed values in every space.
- **Consequences:** Op authors choose `KeepEnc`, `DerivedEnc` or `DisplayEnc`. Masks display linearly, which
  changes mask looks (0.5 shows as 128 instead of 188).
- **Status:** proposed

### Decision: looks show encoded values

- **Context:** Today a look shows sRGB_encode(linear). For linear-encoded data that would show v = 0.3 as
  0.584 and a signed map's 0.5 offset as 0.735.
- **Options considered:** (a) show perceptual (today's rule); (b) show encoded; (c) a per-look display option.
- **Decision:** (b), with `img.encode("srgb")` for linear photographs.
- **Rationale:** (b) is today's rule for every sRGB file and the scientific-viewer rule (show the stored
  value) for linear data. It makes a display offset of 0.5 grey 128 in every case. (c) adds a second place where
  spaces are chosen.
- **Consequences:** Linear photographs need `.encode("srgb")` (or a stretch from GCV-002) to look normal.
- **Status:** proposed

### Decision: one TIFF decoder with its own IFD parser

- **Context:** `x/image/tiff` rejects float samples and LinearRaw; DNG needs SubIFD traversal; windowed reads
  need strip offsets.
- **Options considered:** (a) a separate DNG decoder next to x/image/tiff; (b) libtiff through cgo; (c) an own
  IFD parser for all TIFFs, delegating compressed pixel data to x/image/tiff.
- **Decision:** (c).
- **Rationale:** The formats in question are uncompressed and simple; an IFD parser is about 300 lines. One
  parser gives probing, SubIFDs, float, LinearRaw and windows in one place, keeps the static pure-Go build
  (CVJS-001 integration-surfaces decision on tier-1 formats), and leaves LZW/Deflate to the existing package.
- **Consequences:** We own a TIFF parser and must fuzz it (§13). Compressed files cannot be read by window
  (the whole image is decoded, then cut).
- **Status:** proposed

### Decision: frames replace doc sizes

- **Context:** A window read from a scan should keep scan coordinates; GCV-002's crop needs the same.
- **Options considered:** (a) keep `DocW/DocH` and add a separate origin where needed; (b) a `Frame` value on
  nodes and values.
- **Decision:** (b).
- **Rationale:** Every place that reads `DocW/DocH` already reasons about "the rectangle this image covers"; one
  type makes the origin impossible to forget. The refactor is mechanical.
- **Consequences:** `Args.XY` and `AxisScales` subtract the origin; `NewRegion` adds it; two-image ops compare
  frames.
- **Status:** proposed

### Decision: named images are a session feature, not an engine feature

- **Context:** Issue 20 asks for several names.
- **Options considered:** (a) names in the engine (nodes carry names); (b) names in the session layer as
  globals backed by a table.
- **Decision:** (b).
- **Rationale:** Names do not change what a node computes, so they must not enter hashes. The session already
  owns `img`; generalizing it removes a special case.
- **Consequences:** The context block and the MCP tool learn names; the CLI's `--image` binds `img`.
- **Status:** proposed

## 9. Pseudocode and key flows

### 9.1 Opening and decoding

```
cv.open(path, opts)                                    // init.js wrapper: checkPath(path)
  binding.open(path, opts)
    node, as := engine.Open(path, opts)
      abs := resolve(path)
      info := buffer.ProbeFile(abs)                    // decoder registry, header only
      enc := opts.encoding or info.Encoding
      frame := Frame{X: box.x or origin.x or 0, Y: …, W: box.w or info.W, H: box.h or info.H}
      n := Intern(open, args{path, stamp, enc, box, hidden info}) with n.Enc = enc, n.Space = linear, n.Frame = frame
    if as != "": session.names.bind(as, node)
    return node

first evaluation of open at scale s:
  src := sources.Load(abs, stamp, box)                 // once per file version and window
    buf, info := decoder.Decode(r, {Box})              // encoded values
    lin := buffer.Convert(buf, enc, linear)            // exact 8/16-bit tables; identity for linear
    pyr := NewPyramid(lin)                             // area averages of linear values
  return pyr.AtScale(s) with Space = linear
```

### 9.2 Interning with spaces

```
Intern(op, receiver, raw):
  args := bind(op, raw)                                // types, units, unknown options, synonyms (unchanged)
  if op.Expand != nil: return op.Expand(expander, receiver, args)
  s := args.space or op.Space.Default
  if op.Space.Default != none: args.space = s
  for each image input i of op:
      if i.Space != s and !Identity(i.Enc, i.Space, s):
          i = Intern(convert, i, [s])                  // hash-consed: shared with other consumers
  warn SPACE for lifted Level args whose node.Space != s
  warn ENCODING when s == encoded and image inputs have different Enc
  n := internNode(op, args, inputs)
  n.Enc, n.Space = resultEncoding(op.Result, receiver, s)
  n.Frame = op.DocFrame(args, inputs) or receiver.Frame
  return n
```

### 9.3 A trace

Take the colleague's tile in today's form (sRGB_encode(v) stored, `encoding: "srgb"` implied) and ask for a
luminance mask:

```js
return img.mask.luminance({ above: 0.9 }).explain()
```

```yaml
- "#3 mask.luminance(above: 0.9)       Mask 1024×1024"
- "  #2 convert(→ encoded)             Image 1024×1024 srgb"
- "    #1 source 1d955ef4….tif          Image 1024×1024 srgb, linear"
```

The same tile re-exported with v stored and opened with `encoding: "linear"`:

```yaml
- "#2 mask.luminance(above: 0.9)       Mask 1024×1024"
- "  #1 source 1d955ef4…_v.tif          Image 1024×1024 linear, linear"
```

No conversion node: for a linear encoding, linear and encoded are the same values, so the builder did not
insert one. The first mask thresholds sRGB_encode(v) > 0.9; the second thresholds v > 0.9. That difference
is issue 7.

### 9.4 Rendering a look

```
look.Render(node, scale):
  root := Intern(convert, node, [encoded]) unless Identity(node.Enc, node.Space, encoded) or node.Kind == Mask
  v := Evaluate(root, scale)
  img := buffer.Quantize(v.Buf, 8)                     // clamp to [0, 1], round to 0..255, no transfer curve
  burn the look number; encode PNG or JPEG (GCV-002 adds format)
```

## 10. Errors and diagnostics

New and changed diagnostics follow the existing patterns: a code, a message in the API's terms, a hint, a
literal fix, a `valid` list, and did-you-mean (`pkg/cv/graph/args.go:556-585`).

| situation | code | message and fix |
|---|---|---|
| unsupported file feature | `E_FORMAT` | `cv.open("x.dng"): CFA (mosaiced) raw data is not supported; goja-cv reads LinearRaw DNGs.` hint: export a linear DNG or TIFF. Each decoder names the feature (compression, sample format, Fortran order, planar config). |
| unknown encoding | `E_ARG_TYPE` (enum) | `cv.open.encoding = "gamma" is not one of srgb, linear.` fix: `cv.open(path, { encoding: "linear" })` |
| `box` outside the file | `E_ARG_RANGE` | with the file size and a clipped box as the fix |
| `box` and `origin` together | `E_ARG_TYPE` | "box already sets the origin" |
| bad or taken name | `E_NAME` | names the conflicting global; fix with a free name |
| lifted level from another space | `SPACE` warning | §7.6 |
| mixed encodings in encoded space | `ENCODING` warning | §7.10 |
| channel out of range | `E_ARG_RANGE` | fix `img.toGray()` or the valid channel names |
| colormap on 3 channels | `E_WRONG_KIND` | fix `img.channel("g").colormap("viridis")` |

Synonyms (`OptionSynonyms`, `pkg/cv/graph/args.go:590-605`) gain entries that map words agents use to the new
options: `colorSpace`, `colorspace`, `transfer`, `gamma` → `encoding`; `name`, `bind`, `alias` → `as`;
`window`, `rect`, `region` → `box` (already partly present); `workingSpace`, `in` → `space`. Enum values gain
a small value-synonym table checked before edit distance: `sRGB`, `gamma` → `srgb`; `raw`, `lin`, `none`,
`identity` → `linear`; `file`, `stored`, `code` → `encoded`. Op synonyms (`pkg/cv/graph/suggest.go:10-41`) gain
`grayscaleChannel` → `channel`, `lab` → `toLab`, `saturation` → `chroma`, `falseColor`, `lut`, `heatmap` →
`colormap`, `reencode` → `encode`.

`cv.help` never throws (`pkg/gojamod/cv/help.go:12-17`). Add concept entries to its function table for
`encoding`, `space`, `frame` and `names`, each with a usage line and a pointer to the docs topic. Replace the
`metadata` redirect text ("colorSpace") with "encoding".

## 11. Documentation and examples that change

Docs are part of the tool: agents read them through `cv.docs` and every ```js block runs in tests
(`pkg/cvsession/docs_test.go:77-152`).

- New topic `pkg/cv/docs/topics/10-encodings-and-spaces.md` (id `encodings-and-spaces`): the §3 concepts in
  agent language, the default table, the three identities, how to open linear data, `{ space }` examples,
  `encode`, and a `js warns=SPACE` block. Remove the "Linear and perceptual" section from
  `02-units-and-scales.md:43-49` and link to the new topic.
- `02-units-and-scales.md`: a "Frames" section (window opens keep file coordinates).
- `01-getting-started.md`: the globals table describes names (`img` and `cv.open(path, { as })`).
- `06-looks.md`: "masks and derived images show their values; colour images show encoded values".
- `09-recipes.md`: a recipe "Open scientific data" (npy, float TIFF, DNG window) and "Neutral or coloured?"
  (chroma + colormap).
- Guidebook (`pkg/doc/02-concepts.md`): encodings and spaces paragraph; `03-cli.md`: `--image` binds `img`.
- MCP instructions (`pkg/cvsession/reference.go:15-37`): replace "(linear light)" and "black/white are linear
  levels like stats" with one line: "Numbers are in the op's space: linear (default for exposure, blur, blend,
  stats, levels) or encoded (what looks show: thresholds, contrast, histogram, specks); pass { space } to
  change it; cv.docs.read(\"encodings-and-spaces\")." Keep the instructions under 4000 characters (comment at
  `reference.go:14`).
- README: the quick-start `stats` output and the "image metadata" paragraph.

Examples and tests that change because of breaking behaviour:

| change | affected |
|---|---|
| `levels` remaps in linear | numbers after `levels` in `06-looks.md` (meanL 0.27479), README, `pkg/cv/ops/ops_test.go` levels cases, golden looks of levels |
| masks display linearly | look golden bytes of mask looks (`pkg/cv/look/look_test.go`) |
| `colorSpace` → `encoding` | `pkg/gojamod/cv/module_test.go` metadata asserts, README, `cvsession` context-block tests |
| `DocW/DocH` → `frame` in awaited summaries | every test that reads `docW`/`docH` from an awaited value; the summary keeps `w`, `h` and adds `frame` |
| histogram result `space` is the resolved space | histogram tests |

## 12. Implementation phases

Each phase ends with `go test ./... -count=1`, `make lint`, and a commit. Write the listed tests first.

**Phase 0 — groundwork.**
1. Add `TestKernelsAreSpaceAgnostic` (`pkg/cv/kernels/space_guard_test.go`): parse the non-test files of
   `pkg/cv/kernels` and `pkg/cv/ops` with `go/parser` and fail on calls to `buffer.ToPerceptual`,
   `buffer.ToLinear`, `buffer.Linear8/16`, `buffer.EncodeSRGB8/16`. Mark it `t.Skip` until Phase 3 ends.
2. Remove `Node.Base` and the unused error codes; `go build ./...` and tests pass unchanged.

**Phase 1 — encodings, spaces, conversion.**
1. Tests first: `pkg/cv/buffer/encoding_test.go` with the identity table, round-trip properties
   (`Convert(Convert(b, e, from→to), e, to→from) ≈ b` within 2e-6 for random values in [−0.5, 2]), and the
   numbers of §3.2 (0.5 → 0.214, 0.05 → 0.0039).
2. Implement `Encoding`, `Space`, `Identity`, `Convert`, `Buffer.Space`; `Quantize` replacing `ToImage`;
   encoders write encoded values.

**Phase 2 — decoders.**
1. Fixtures: extend `internal/testimage` with writers for the planted scan as 16-bit linear TIFF, float32
   TIFF, LZW TIFF, `.npy` (`<f4` and `<u2`), and a minimal DNG (IFD0 thumbnail plus a SubIFD with
   `NewSubfileType` 0, Photometric 34892, 16-bit, 3 samples, uncompressed). The writers are a few dozen lines
   each; a TIFF writer for these simple layouts is easier to trust than a third-party one.
2. Tests: each fixture decodes to the same linear buffer as the sRGB TIFF within 2e-5; `Probe` reports format,
   bits, sample, encoding; `Box` reads equal the full decode cut to the box; malformed headers return
   `E_FORMAT`, never panic (`FuzzTIFFProbe`, `FuzzNPYProbe` with the fixtures as seeds).
3. Implement the registry, `png`, `jpeg`, `tiff` (IFD parser, SubIFDs, uncompressed reader, x/image fallback),
   `npy`. Delete `Decode`, `DecodeFile`, `DecodeConfigFile`.
4. `Engine.Open(path, raw)`; the `open` op gains `encoding`, `box`, `origin`, `as`; `SourceInfo` reports
   `encoding`, `sample`, `frame`, `notes`.

**Phase 3 — spaces in the graph.**
1. Tests first (`pkg/cv/graph/space_test.go`): default resolution per op (table-driven over the §7.5
   defaults); explicit `{ space }` changes the hash; a conversion node is inserted once and shared by two
   consumers (`Evaluator.Runs`); no conversion for linear-encoded inputs; `DerivedEnc` reinterprets;
   `SPACE` and `ENCODING` warnings with their fixes.
2. Replace `OpDef.Space string` with `SpaceRule`; add `Result`, `Internal`; `Register` appends the `space`
   option; `Intern` resolves and inserts `convert`.
3. Port the kernels in §4.3: remove the private conversions; `PerceptualLuminance` becomes `Luminance`
   (weighted channels of whatever space it is given); `DetectSpecks` reports `MeanL` in its working space.
   Turn on the guard test.
4. `look.Render` renders encoded values; masks display linearly. Update goldens deliberately, one commit.

**Phase 4 — channels and macros.**
1. Tests: `channel` on 1 and 3 channels; `split` returns three nodes equal to three `channel` calls (same
   ids); `toLab` of known sRGB colours against reference values (white → (1, 0, 0); sRGB red → L* 53.24,
   a* 80.09, b* 67.20); `chroma` of a grey image is 0; `colormap` endpoints; `encode` round trip.
2. Implement `Expand`/`Expander`, the binding's array return, and the ops.

**Phase 5 — frames.**
1. Tests: `open({ box })` regions and `stats({ box })` use file coordinates; two-image ops reject different
   origins; `origin` shifts coordinates without reading differently.
2. Replace `DocW/DocH` with `Frame` everywhere (`grep -rn DocW pkg cmd` lists the sites); update `XY`,
   `AxisScales`, `NewRegion`, `Rasterize`, summaries.

**Phase 6 — named images.**
1. Tests (`pkg/cvsession/names_test.go`): `as` binds and persists across evals; reserved and invalid names give
   `E_NAME` with a fix; rebinding; the context block lists names and encodings; the MCP `open` tool with `as`.
2. Implement the name table, `init.js` changes, context block, MCP schema.

**Phase 7 — documentation and agent check.**
1. New and changed topics, help concept entries, synonyms, instructions, guidebook, README (§11). The docs
   tests must pass with the new blocks.
2. Run the pi harness task (§13.5) on the old and new builds; record the table in the ticket.

## 13. Test strategy

### 13.1 Unit and property tests

- Transfer functions: identities, round trips, monotonicity of `Convert` for every (encoding, from, to).
- Decoders: fixture equivalence, windows, probes, malformed input fuzzing.
- Graph: space resolution, conversion sharing, derived reinterpretation, frames, warnings.
- Kernels: the existing kernel tests keep passing with explicitly converted inputs; the guard test forbids
  private conversions.

### 13.2 Documentation tests

Every ```js block in `pkg/cv/docs/topics` and `pkg/doc` runs against the planted scan
(`pkg/cvsession/docs_test.go:77-152`). The new topic needs fixtures beyond the planted TIFF; add a second test
image set written by `internal/testimage` (the linear TIFF and `.npy` of the planted scan) into the same
temporary directory, and refer to them by relative names in the docs (`cv.open("planted-linear.tif")`).

### 13.3 Golden looks

Keep golden looks for sRGB images byte-identical except for `levels` and masks: render the existing look
tests before and after Phase 3 and diff. Any other change is a bug in the defaults table.

### 13.4 Real data (gated)

`~/dustdet/cv_tiles` holds 50 tiles (`{tile_id}.tif` 16-bit, 1024², uncompressed RGB, one strip; plus
`{tile_id}.json` with `origin_x`, `origin_y`). Gate on `GOJACV_TILES` (default `~/dustdet/cv_tiles`, skip when
absent):

- Open `1d955ef4b27d3d88_1536_1536.tif` (srgb, stores sRGB_encode(v)). Write its linear buffer as a 16-bit
  linear TIFF in `t.TempDir()`, open that with `encoding: "linear"`. Assert: linear buffers equal within
  2e-5; `mask.luminance({ above: 0.9, space: "linear" })` on the first equals `mask.luminance({ above: 0.9 })`
  on the second; `stats()` agree (the current tile has meanL 0.33567, p01 0.0085, p99 0.918 in linear).
- With `origin` from the tile's JSON, region centroids are absolute raw coordinates.

DNG, gated on `GOJACV_DNG` (for example the 319 MB `raw0027.dng` listed in `dng_meta.jsonl`): `Probe` gives
7256 × 7326, 16-bit, linear; `open({ box: [1536, 1536, 1024, 1024] })` takes under a second and equals the same
window cut from a full decode (run the full decode only with `GOJACV_DNG_FULL=1`).

### 13.5 Agent harness

The pi harness (`CVJS-001/scripts/06-pi-cv-agent/run.sh GOJA_CV_DIR OUT_DIR [MODEL]`) builds goja-cv, writes the
planted scan, and asks `questions.md`. Extend it with two environment overrides, `IMAGE` and `QUESTIONS`, and a
question file for this ticket, `questions-gcv-001.md`, against a density tile re-exported as v (16-bit linear)
plus the original tile as a reference:

1. What is the image's encoding, and what are p01 and p99 in file units?
2. Open the second file as `ref`. Is it the same frame? What is the largest absolute difference in file
   units?
3. Is the bright blob near (x, y) neutral or coloured? Give a number.

Run it against the build before this ticket (where the model has to work around the missing encoding) and
after. Record calls, errors and correctness in the CVJS-001 pi-runs table format. The measure is friction, not
capability; the deep-dive article's caveat about run-to-run variance applies (run each build twice).

## 14. Risks, alternatives and open questions

**Risks.**

- *Default changes on sRGB images.* Only `levels` and mask display change; the golden diff (§13.3) is the
  guard. If agents' existing recipes break visibly, revisit the `levels` default before changing anything else.
- *Graph growth.* Conversion nodes add cached buffers. On a 1024² RGB tile a conversion is 12 MB at scale 1;
  the cache cap (2 GiB, `engine.go:24`) absorbs dozens. On full DNGs it is 638 MB per conversion; windows are
  the answer, and the docs recipe uses them.
- *TIFF parser bugs.* Fuzzing and the fixture equivalence tests are mandatory before merging Phase 2.
- *Agent confusion about spaces.* Three spaces are one more concept to learn. The mitigations are the
  identities (most images have only two distinct spaces), the `SPACE` warning with a fix, and the docs topic.

**Alternatives not taken.** A single "data mode" flag; an ICC-based colour pipeline (too large for the need);
reading DNGs through a Python sidecar (CVJS-001 Part 4 §20.2 suggested an external decoder for camera raw; the
LinearRaw case does not need one).

**Open questions.**

1. Should `stats` default to `encoded` for linear-encoded images? It makes no numerical difference there, so the
   question only matters for sRGB images, where linear is the photometric default. Proposed answer: no.
2. Does the colleague want `.npy` caches of whole DNGs (7256 × 7326 × 3 uint16, 319 MB) opened with `box`? The
   NPY decoder can read windows of C-order arrays with seeks; it is cheap to add in Phase 2 if needed.
3. Should `encode` accept a gamma value (`encode({ gamma: 2.2 })`)? Only if a user brings such data.

## 15. References

- Handover issues: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md` (issues 6, 7, 13, 20), `05-goja-cv-feature-requests.md` (items 4, 6, 7), `06-goja-cv-generic-inspection-features.md` (B2b, B6, B7, C4, D1, D2).
- Playbook: `…/DUST-002…/reference/04-goja-cv-dust-playbook.md`; export script `…/scripts/82-export-cv-tiles.py`; helpers `…/scripts/84-cv-helpers.js`.
- DNG layout: `/Users/manuel/code/wesen/2026-11-07--dust-scanning/dustds/rawio.py`, `~/dustds/meta/dng_meta.jsonl`.
- CVJS-001 intern guide Parts 1–3 (vocabulary; Part 2 §2 on linear light; Part 3 §15.1 buffers, §15.3 registry) and `06-integration-surfaces…` (file format tiers, pure-Go decision): `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/`.
- Agent lessons: go-go-parc `Projects/2026/11/09/ARTICLE - Deep Dive - Teaching a Cheap Model an Image Eval API - goja-cv, Docs and pi Runs.md` (docs inside the tool, never-throwing help, literal fixes, synonyms, describing optional parameters).
- Next tickets: GCV-002 (`design-doc/01-design-and-implementation-guide.md`: crops and zoom use `Frame`, views use macros and derived encodings), GCV-003 (measurements use `Level` units and resolved spaces; `node.save` writes encoded values with the image's encoding).
- sRGB transfer function: IEC 61966-2-1; CIE L*a*b* with D65: CIE 15:2004.
