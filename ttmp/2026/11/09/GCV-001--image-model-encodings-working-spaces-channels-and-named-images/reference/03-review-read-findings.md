---
Title: Review read findings
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
      Note: the guide under review
    - Path: pkg/cv/kernels/mask.go
      Note: PerceptualLuminance (41-52) is sRGB_encode of linear luminance, not luma; basis of finding P1-1
    - Path: pkg/cv/kernels/measure.go
      Note: Histogram channel L (200-216) has the same luminance-order issue
    - Path: pkg/cv/look/look_test.go
      Note: no golden images exist; basis of finding P1-2
    - Path: pkg/cv/graph/node.go
      Note: internNode and Substitute (115-230) bypass Intern; basis of finding P1-4
    - Path: pkg/cv/graph/engine.go
      Note: Placeholder (127-141) has no encoding or space today
    - Path: pkg/cv/buffer/color.go
      Note: ToPerceptual/ToLinear LUT error is up to 2e-5 (58-60); round-trip tolerance in Phase 1 is 2e-6
    - Path: pkg/cv/graph/explain.go
      Note: Describe omits only args equal to Default (79); space has no Default
    - Path: pkg/cv/graph/registry.go
      Note: Register rejects ops without Run (160); macros need an exception
    - Path: pkg/cv/graph/args.go
      Note: XY and AxisScales (118-139); AxisScales returns scales, not coordinates
    - Path: pkg/cv/graph/regions.go
      Note: NewRegions (37); the guide's NewRegion does not exist
    - Path: pkg/gojamod/cv/binding.go
      Note: open (259-271) lives in the module, which has no session access
    - Path: pkg/cvsession/session_test.go
      Note: the colorSpace assert (378) the guide attributes to module_test.go
ExternalSources:
    - /Users/manuel/.codex/skills/textbook-authoring/SKILL.md
    - /Users/manuel/.codex/skills/ticket-research-docmgr-remarkable/references/writing-style.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/82-export-cv-tiles.py
Summary: "Independent review read of the GCV-001 design and implementation guide: 4 P1, 22 P2 and 21 P3 findings on concept introduction, correctness against the code at 175b5d4, contradictions (also with GCV-002/003), guideline conformance, implementability and architecture."
LastUpdated: 2026-11-09T13:30:00-05:00
WhatFor: "Revising the GCV-001 guide before an implementer starts."
WhenToUse: "When editing design-doc/01 of GCV-001, or when GCV-002/003 depend on a GCV-001 decision flagged here."
---

# Review read findings

## Goal

This document records an independent review read of `design-doc/01-design-and-implementation-guide.md` of
GCV-001. The reader played an intern who knows Go and some JavaScript but has never seen goja-cv, read the guide
top to bottom, and then checked every `file:line` claim against the code. The findings are ranked so that the
writer can fix the P1 items before the guide goes to an implementer.

## Context

- **Code baseline.** The guide cites commit `175b5d4`. `git diff --stat 175b5d4 HEAD -- pkg internal cmd` is
  empty, so the citations were checked against the current tree.
- **Guidelines.** `textbook-authoring/SKILL.md` (Norvig style, no analogies, full-sentence bullets, key points,
  closing) and `writing-style.md` (eleven-section design-doc order, decision-record format, evidence rules).
- **Cross-reading.** The GCV-002 and GCV-003 guides were searched only for terms they take from GCV-001
  (`Frame`, `DocFrame`, `Expand`, `DerivedEnc`, `Level`, `space`, `encoded`, `Quantize`, saved-file metadata).
- **Rank meaning.** P1 means an implementer following the guide would build something wrong, or could not
  finish a phase with green tests. P2 means a gap, contradiction or unclear concept that costs a question or a
  rework. P3 means polish, small mismatches and style.

What the guide does well, so that revisions keep it: the §3 concept ladder with checkable numbers, the
one-pixel-through-three-spaces table, the honest `levels` difference table (the numbers check out: 0.48/0.26,
2.15/1.17, 4.45/2.47, 8.54/4.88 codes), the `convert`-as-node decision, the legacy-removal list, all required
design-doc sections in the prescribed order, complete decision records, and line references that are almost
all exact (see the verification table at the end).

## Quick Reference

### Counts

| rank | count |
|---|---|
| P1 | 4 |
| P2 | 22 |
| P3 | 21 |

### P1 — must fix before handing to an implementer

**P1-1. "Encoded = perceptual" does not hold for luminance-based ops on colour images.**
*Location:* §7.5 Defaults, "For sRGB images, every op that used to be "perceptual" now defaults to `encoded`,
which is numerically the same, so their behaviour does not change"; also §3.3 identity bullets, §12 Phase 3
step 3 ("`PerceptualLuminance` becomes `Luminance` (weighted channels of whatever space it is given)"), §13.3
("Any other change is a bug in the defaults table"), §14 Risks ("Only `levels` and mask display change").
*Problem:* The identity holds per channel. It does not hold for ops that first combine channels.
`PerceptualLuminance` (`pkg/cv/kernels/mask.go:41-52`) computes sRGB_encode(0.2126 R + 0.7152 G + 0.0722 B) on
**linear** channels. The proposed encoded-space `Luminance` computes 0.2126 R' + 0.7152 G' + 0.0722 B' on
**encoded** channels (luma). For pure sRGB red the old value is 0.498 and the new one 0.213; for (0.8, 0.4, 0.2)
it is 0.513 versus 0.471. The affected ops are `mask.luminance`, `refine` (`pkg/cv/ops/mask.go:364`),
`detect.specks` and `check.dust` (`specks.go:171`), and `histogram` with the default channel `"L"`
(`measure.go:200-216`). The planted scan has a tinted background (`planted.go:45`: ×1.04, ×1, ×0.93), so its
numbers move a little too, and the intern is told that any such change is a bug.
*Fix:* Choose one of two rules and record it as a decision. (a) Accept luma in encoded space: list
"luminance-based ops on colour sRGB images" next to `levels` in §7.5, §11 (breaking-change table), §13.3 and
§14, and give a measured table like the `levels` one. (b) Keep luminance defined as "luminance of linear light,
then expressed in the op's space": the op asks for its input in `linear`, computes Y, and converts the Y plane
to its space; this keeps sRGB behaviour exact but needs a stated exception to "kernels never convert" (for
example a luminance-plane helper owned by the graph, not by kernels). Rule (b) matches photometry; rule (a) is
simpler. Either way, correct the "numerically the same" sentence.

**P1-2. The golden-look safety net the guide relies on does not exist.**
*Location:* §11 table ("golden looks of levels", "look golden bytes of mask looks (`pkg/cv/look/look_test.go`)"),
§12 Phase 3 step 4 ("Update goldens deliberately, one commit"), §13.3 ("render the existing look tests before
and after Phase 3 and diff").
*Problem:* There are no golden images or hashes in the repository (`grep -ri golden pkg cmd internal testdata`
finds only `csscolors.go`). `look_test.go` checks captions, sizes, formats and the burned number, not pixel
values. The intern cannot "diff the goldens", and the main regression guard for "sRGB images look exactly as
before" (§7.7) is missing.
*Fix:* Add a Phase 0 step: write `pkg/cv/look/golden_test.go` that renders a fixed list of looks on the planted
scan (source, exposure, gaussian, contrast, levels from stats, each mask op, detect.specks overlay, histogram,
compare) and stores PNG bytes or per-look SHA-256 plus a max-code-difference report under `testdata/golden/`,
with an `-update` flag. Commit it before any model change. §13.3 then names that test and the expected
differences (levels, masks, and whatever P1-1 decides).

**P1-3. The phase order cannot keep `go test ./...` green at the end of each phase.**
*Location:* §12, "Each phase ends with `go test ./... -count=1`, `make lint`, and a commit"; Phase 1 step 2
"`Quantize` replacing `ToImage`; encoders write encoded values"; Phase 3 step 4 "`look.Render` renders encoded
values".
*Problem:* In Phase 1 buffers are still linear and nodes have no `Enc` or `Space` (those arrive in Phase 3).
If `ToImage` is replaced by `Quantize` in Phase 1, every look of an sRGB image shows linear values (dark) until
Phase 3, and `WriteFile` writes linear values into files that are read back as sRGB. Phase 2 adds
`open({ box })` before `Frame` exists (Phase 5), so window coordinates are window-local for three phases, and the
Phase 5 test "`open({ box })` regions … use file coordinates" changes semantics that Phase 2 already shipped.
*Fix:* Reorder and state each phase's intermediate semantics. Suggested order: Phase 0 (guard test skipped,
golden harness from P1-2, dead code); Phase 1 (`Encoding`, `Space`, `Identity`, `Convert`, with `ToImage` kept);
Phase 2 (`Frame` replaces `DocW/DocH`, mechanical, no behaviour change); Phase 3 (node `Enc`/`Space`,
`SpaceRule`, `convert` insertion, kernel port, `look.Render` = convert to encoded + `Quantize`, delete
`ToImage`, golden diff); Phase 4 (decoder registry, `encoding`, `box`, `origin`); then channels/macros and
names. Each phase gets one sentence of the form "after this phase, X behaves as Y".

**P1-4. Space resolution ignores builder lambdas (`within`) and `Substitute`.**
*Location:* §7.5 "Resolution at build time" and §9.2 pseudocode (`n := internNode(…)` then
`n.Enc, n.Space = resultEncoding(…)`).
*Problem:* `img.within(mask, i => i.contrast(0.2))` builds the lambda on a placeholder node
(`Engine.Placeholder`, `engine.go:127-141`), then `Builder.Substitute` (`node.go:189-230`) re-interns every node
on the path through `internNode` directly, not through `Intern`. The guide never says what `Enc`/`Space` the
placeholder has, so conversion decisions inside the lambda are made against an unknown encoding, and nodes
re-interned by `Substitute` would get no `Enc`/`Space`/`Frame` at all if those are set in `Intern` after
`internNode`. Setting fields after `internNode` also mutates a node that may already be shared (`internNode`
returns the existing node on a hash hit), although `Node` is documented as immutable (`node.go:12`).
*Fix:* (1) Compute `Enc`, `Space` and `Frame` inside `internNode` from `(op, args, inputs)` so that every
creation path, including `Substitute`, gets them; `Intern` only resolves the `space` argument and inserts
`convert` inputs before calling it. (2) The placeholder copies `Enc`, `Space` and `Frame` from its receiver (it
already copies `DocW/DocH`, `engine.go:139`), so lambda graphs resolve exactly as they will after substitution.
(3) Add a Phase 3 test: `within(mask, i => i.contrast(0.2))` on an sRGB image and on a linear-encoded image
produces the same inner graph as building `img.contrast(0.2)` directly.

### P2 — fix before or during the first phase that touches the area

**P2-1. The prerequisite vocabulary is outsourced and its path is missing.**
*Location:* intro, "It assumes you have read the CVJS-001 intern guide, Parts 1–3 (`design-doc/02` to `04` in the
CVJS-001 ticket), or at least its vocabulary table".
*Problem:* The CVJS-001 guide lives in another repository (`2026-11-07--dust-scanning`); the absolute path
appears only in §15. The guide then uses goja-cv terms it never defines: **context block** (§1, §2.1, §4.5,
§7.10; never explained), **look**, **hash-consed/interned**, **binder**, **lifted** Value node (§7.6),
**canonical args**, **hidden arguments** (§4.1), **binding** vs **prelude** vs **session** (§4.5, §7.10),
**owner goroutine** (§7.10), **MCP** (never expanded).
*Fix:* Give the absolute path in the intro, and add a §3.0 table of 12–15 rows (term, one-sentence definition,
where in the code): node, kind, op/OpDef, `Builder.Intern` and hash-consing, binder and lifting, canonical args,
evaluator and cache, scale and doc px, look, kernel, buffer, binding (`pkg/gojamod/cv`), session
(`pkg/cvsession`), context block, MCP tool.

**P2-2. "Linear light" and "linear" are used before they are defined, and "linear quantity" is muddled.**
*Location:* §1 "converts it to linear light at load time"; §3.1 "A quantity is **linear** when doubling it means
"twice as much": twice the photons, twice the density."
*Problem:* "Linear light" is the term the code and docs use (`buffer.go:1`, MCP instructions), but the guide
never defines it. §3.1 makes linearity a property of a quantity, while §3.2/§3.3 make it a property of an
encoding relative to a quantity. Density is logarithmic in light, so "twice the density" is linear in density
and not in light; an intern gets confused exactly where §3.4 says "for density tiles it is density". The guide
also never says that goja-cv cannot know the quantity: "linear" always means "the declared encoding undone".
*Fix:* In §3.1 define "linear in a quantity Q: the stored number is proportional to Q (zero means none, doubling
the number doubles Q)", define "linear light" as linear in photon count, give the density example with numbers
(D = 0.3 vs 0.6 transmits 50 % vs 25 % of the base light), and add one sentence: "goja-cv does not know the
quantity; `linear` means the values after the declared encoding is undone, and the user declares an encoding
under which that is the quantity they care about."

**P2-3. `perceptual` is a third space with no user and no decision record.**
*Location:* §3.3 table row `perceptual`; §7.5 defaults table (no op defaults to `perceptual`); §14 Risks "Three
spaces are one more concept to learn".
*Problem:* Issue 7 asks for `linear | encoded`. `perceptual` equals `encoded` for sRGB data, and for linear data
it equals `encode("srgb")` followed by `encoded`, which the guide already offers (§7.7). It doubles the
conversion table (§7.2), the enum, the value synonyms and the docs, and the name is never motivated (why the
sRGB curve is called perceptual: equal steps are roughly equal visible lightness steps).
*Fix:* Either drop `perceptual` (two spaces; `img.encode("srgb")` is the documented way to get perceptual values
of linear data) and shrink §7.2 to a 2 × 2 table, or keep it with a decision record that names a concrete op or
workflow that needs it and explains the name with one number.

**P2-4. Masks: linear encoding or no encoding?**
*Location:* §3.5 "Masks follow the same rule"; decision "derived quantities and masks carry the linear encoding";
versus §7.2 `Enc buffer.Encoding // Image nodes; 0 for other kinds` and §7.5 "Mask nodes have `Space =
SpaceNone`".
*Problem:* The two statements are incompatible. An implementer of `Identity`, `look.Render` and `stats` on
masks needs one.
*Fix:* State: "Mask nodes have `Enc = 0` and `Space = SpaceNone`; they are never converted and are displayed by
`Quantize` directly, which is what 'displays linearly' means." Retitle the decision "derived quantities carry
the linear encoding; masks carry none".

**P2-5. `KeepEnc` is undefined when the receiver is not an Image.**
*Location:* §7.5 "`KeepEnc`: `Enc = receiver.Enc`".
*Problem:* `mask.overlay(img)` (`pkg/cv/ops/mask.go:388`) and `regions.overlay(img)` (`present.go:105`) return an
Image whose receiver is a Mask or Regions node with no encoding. `compare` takes `Image[]` (`present.go:116`).
*Fix:* Define `KeepEnc` as "the encoding of the first Image input (receiver, then `Inputs`, then kind-typed
arguments)", and say what happens when Image inputs disagree (the `ENCODING` warning already covers encoded
space; in linear space the first wins).

**P2-6. The convert-insertion loop is not guarded for ops without a space, and lists are not covered.**
*Location:* §9.2 "`for each image input i of op: if i.Space != s and !Identity(…)`"; §7.5 step 2 "(the receiver
and `Inputs` of kind Image, plus kind-typed Image arguments)".
*Problem:* For `channel`, `crop` and other `SpaceNone` ops, `s` is `SpaceNone` and the loop would try to
convert every input to "none". Kind-typed arguments are lifted inputs (`bd.lifted`, `node.go:94`), not explicit
inputs, so "each explicit input … plus kind-typed arguments" mixes two categories; `Image[]` arguments
(`compare`) need element-wise handling.
*Fix:* Guard the loop with `if s != SpaceNone`, name the three input sources precisely (receiver, `bd.inputs`,
`bd.lifted` whose kind is Image, including list elements), and add `compare([a, b])` with mixed spaces to the
Phase 3 tests.

**P2-7. `Buffer.Space` duplicates the graph and is unreliable; `Convert` depends on it anyway.**
*Location:* §7.2 "`Buffer` gains one field, `Space Space`, set by every producer. It is a tag for assertions and
debugging; the graph is the authority" and "`Convert` … `b.Space` must be set. It returns `b` itself when
Identity holds".
*Problem:* When `Identity` holds, the same cached, read-only buffer serves two spaces, so its tag is wrong for
one of them and cannot be retagged. Every kernel output would need the tag set, which is boilerplate in every
`Run`. `Convert` reading `b.Space` contradicts "the graph is the authority".
*Fix:* Drop `Buffer.Space`. Make the signature `Convert(ctx, b, enc, from, to Space)`; the `convert` op passes
`from` from its input node. This removes a field instead of adding one.

**P2-8. `ImageValue.Enc` and `ImageValue.Frame` duplicate node statics.**
*Location:* §7.2 `ImageValue { Buf; Frame; Enc; kind }`; §7.5 convert op `graph.NewImage(out, src.Frame, src.Enc)`.
*Problem:* Every op's `Run` must now thread `Enc` and `Frame` through `NewImage`, although both are known
statically on the node. Today `DocW/DocH` are duplicated this way (`value.go:17-28`), and the guide carries the
duplication forward instead of removing it.
*Fix:* Let the evaluator stamp `Frame` (and `Enc`, if values need it at all) from the node after `Run`, and give
`Run` access to its node through `EvalCtx` for the few ops that need statics (convert, looks). `NewImage(buf)`
then takes only the buffer. This is the legacy-removal opportunity for `DocW/DocH` on values.

**P2-9. `DocFrame` is used but never declared; GCV-002 depends on it.**
*Location:* §9.2 "`n.Frame = op.DocFrame(args, inputs) or receiver.Frame`"; the §7.5 `OpDef` sketch lists only
`Space`, `Result`, `Expand`, `Internal`.
*Problem:* GCV-002's crop is written against `DocFrame: func(a graph.Args, in []*graph.Node) graph.Frame`
(GCV-002 guide line 389) and adds `Frame.Sheet`. GCV-001 must define the replacement of `OpDef.DocSize`
(`registry.go:112`).
*Fix:* Add `DocFrame func(args Args, inputs []*Node) Frame // replaces DocSize` to the `OpDef` sketch, state the
default (first input with a non-empty frame, as `internNode` does today at `node.go:131-140`), and use it for
`open` too (frame from `w`, `h`, `box`, `origin` args) instead of computing the frame in `Engine.Open`. Mention
that GCV-002 extends `Frame` with `Sheet`.

**P2-10. Frame semantics for coordinates are underspecified.**
*Location:* §3.6; §7.2 "Region geometry stays in doc px of the root frame"; §8 decision "frames replace doc
sizes" ("`Args.XY` and `AxisScales` subtract the origin; `NewRegion` adds it"); §12 Phase 5.
*Problem:* "Root frame" is never defined. `AxisScales` (`args.go:137`) returns two scales, so "subtract the
origin" does not apply to it; `XY` reads `[x, y]` and `[x, y, w, h]`, where only `x, y` may be shifted.
`NewRegion` does not exist (`NewRegions`, `regions.go:37`). The user-visible consequence is not stated: after
`cv.open(p, { box: [1536, 1536, 1024, 1024] })`, `img.stats({ box: [0, 0, 100, 100] })` refers to a rectangle
outside the image, and the behaviour (error, empty, clip) is not specified. `Frame.X, Y` are `float64` while
`W, H` are `int`, without a reason.
*Fix:* Add to §3.6: "All doc-px arguments and results are absolute: they are in the coordinates of the file the
image came from (or the coordinates declared by `origin`)." Define out-of-frame behaviour (suggest `E_ARG_RANGE`
with the frame and a clipped fix, like the `box`-outside-the-file row of §10). Correct the function names
(shift in `XY` for point coordinates and box origins only; the origin is added where regions are created).
Explain or drop the float origin.

**P2-11. The `as` option lives in two layers, contradicting its own decision record.**
*Location:* §7.4 "`as` is declared too, but `Engine.Open` removes it before interning and returns it to the
binding"; §9.1 "`if as != "": session.names.bind(as, node)`" inside `binding.open`; §8 decision "named images are
a session feature, not an engine feature".
*Problem:* `binding.open` is in `pkg/gojamod/cv/binding.go:259-271` and has no access to the session; the session
wraps `cv.open` in `pkg/cvsession/init.js:16-21`. Declaring `as` on the engine op puts a session concept into the
engine. Outside a session (the bare module), `as` has no meaning and no behaviour is specified.
*Fix:* Pick one path and draw it: the `init.js` wrapper splits `{ as, ...rest }`, validates `as` through a
session-native `bindName`, and passes `rest` to the module; synonyms for `as` (`name`, `bind`, `alias`) are
checked by the wrapper with the `optionSynonym` helper exported from Go. The engine `open` op does not know
`as`. In the bare module, `as` is `E_UNKNOWN_OPTION` with a hint.

**P2-12. Unnamed opened images vanish from the context block.**
*Location:* §6 item 7 "`noteOpen`/`Opened` as the image list … Replaced by a named-image table"; §7.10 "The
context block lists every name".
*Problem:* Today every opened file is listed (`processor.go:482-498`). With only names listed,
`const d = cv.open("tile.npy")` (the guide's own §7.4 example) is invisible, which loses information the agent
has today.
*Fix:* Either list unnamed opens as `(unnamed) tile.npy (…)` or state that the change is intended and why.

**P2-13. `SPACE` warnings fire where the numbers are identical.**
*Location:* §7.6 "When a `Level` argument is a lifted Value node whose static `Space` differs from the op's
resolved space, the binder emits a warning"; §9.2 "warn SPACE for lifted Level args whose node.Space != s".
*Problem:* On the colleague's linear-encoded tiles, `img.mask.luminance({ above: img.stats().p99 })`
mixes `linear` (stats) with `encoded` (mask), but the numbers are the same. The warning would fire on the main
workflow the ticket serves. Value nodes have no `Enc` (§7.2: "0 for other kinds"), so `Identity` cannot be
evaluated for them.
*Fix:* Give level-valued Value nodes the `Enc` of the image they measured, and warn only when
`!Identity(enc, valueSpace, s)`. Add the identity case to the Phase 3 warning tests.

**P2-14. `explain()` would print `space` on every node.**
*Location:* §7.5 step 1 "It stores `s` in the canonical args, so the hash, the cache signature and `explain()`
include it"; §8 "`explain()` shows non-default spaces"; §9.3 trace shows `mask.luminance(above: 0.9)`.
*Problem:* `Describe` omits only arguments equal to their declared `Default` (`explain.go:79`); the `space`
ArgSpec has no `Default` (it is `Optional`, and the default is per op), so every pixel op would print
`space: "encoded"`. The guide states both behaviours.
*Fix:* Say that `Describe` omits `space` when it equals `op.Space.Default`, and that `explain()` shows the node's
encoding and space in the right-hand column, labelled (`enc srgb · space linear`, not `srgb, linear` as in §9.3).

**P2-15. Macros: `Register`, `Intern` and `Engine.Call` signatures are not adapted.**
*Location:* §7.8 `ExpandFunc … ([]*Node, error)`; §9.2 "`if op.Expand != nil: return op.Expand(…)`".
*Problem:* `Registry.Register` rejects an `OpDef` without `Run` (`registry.go:160-162`). `Builder.Intern` and
`Engine.Call` return `(*Node, []Warning, error)`, one node. Warnings raised by the expansion's inner calls must
be collected. None of this is stated.
*Fix:* Specify `Register` ("exactly one of `Run` and `Expand`"), the new `Engine.Call` return (`[]*Node` or a
`Result{Nodes, Warnings}`), how `Expander.Call` accumulates warnings, and that `Expander.Call` may reach
`Internal` ops.

**P2-16. The TIFF/DNG parser is assigned with no format primer, and the compressed path is unworkable.**
*Location:* §7.3 tiff row "compressed strips (LZW, Deflate, PackBits) are handed to `golang.org/x/image/tiff`";
decision "one TIFF decoder with its own IFD parser" ("an IFD parser is about 300 lines").
*Problem:* An intern who has not written a TIFF reader needs the structure: byte-order header (`II`/`MM`, 42,
first IFD offset), IFD entries (tag, type, count, value-or-offset; type sizes), `StripOffsets`,
`StripByteCounts`, `RowsPerStrip`, `PlanarConfiguration`, `SampleFormat`, `SubIFDs`, and BigTIFF (43) as out of
scope. `x/image/tiff` has no strip-level API, so "handing strips" to it means decoding the whole file through it,
which rejects float and LinearRaw (the reason for the own parser) and returns an `image.Image` that needs a
second conversion path.
*Fix:* Add a half-page TIFF primer with a hex dump of the first 16 bytes of a fixture. Decompress strips inside
the own parser with `golang.org/x/image/tiff/lzw` (an exported package), `compress/zlib` and a 20-line PackBits
reader: one code path, windows work for compressed files at strip granularity, and float + LZW works. Remove
the x/image fallback (one decoder, no special case).

**P2-17. Saved-file metadata: no origin field, no PNG origin, no test.**
*Location:* §7.3 "TIFFs carry `goja-cv encoding=linear origin=1536,1536` … PNGs a `gAMA` chunk … The TIFF and PNG
decoders read these back into `Info.Encoding` and the frame origin".
*Problem:* `Info` (§7.3) has no origin field. No PNG chunk is named for the origin. Go's `image/png` ignores
`gAMA` and `sRGB`, so the PNG decoder needs its own chunk scan. Phase 2 has no test for this, while GCV-003
("Decision: saved files carry their encoding and frame") depends on it.
*Fix:* Add `Origin *[2]float64` to `Info`, store the PNG origin in a `tEXt` chunk with the same `goja-cv …`
string, describe the chunk scan, and add a Phase 2 read-back test with a hand-written description/chunk (the
writer itself arrives in GCV-003).

**P2-18. The Phase 1 round-trip tolerance is below the LUT error.**
*Location:* §12 Phase 1 "`Convert(Convert(b, e, from→to), e, to→from) ≈ b` within 2e-6".
*Problem:* `ToPerceptual`/`ToLinear` use interpolated tables with error up to 2e-5 (`color.go:58-60`), and the
guide says all conversions go through "the existing … LUTs". Near zero the encode curve is steep, so the round
trip through two tables exceeds 2e-6.
*Fix:* Either require the exact transfer functions in `Convert` (and call the LUTs a speed path with a
documented error), or set the tolerance to 5e-5 and test exactness separately for the 8/16-bit integer tables.

**P2-19. The space rule in §3.4 contradicts the defaults table and GCV-002.**
*Location:* §3.4 "The correct space is the one in which the quantity is linear"; §7.5 defaults (`detect.specks`,
`mask.luminance`, `contrast`, `histogram` default to `encoded`); GCV-002 `highpass`/`dog` default to `encoded`.
*Problem:* By §3.4's own example, a detector in sRGB-encoded space "prefers specks in thin areas", yet
`detect.specks` defaults to `encoded`, which is the sRGB curve for photographs. An intern cannot derive the
defaults table from the stated rule.
*Fix:* State the rule as a pair: "if equal physical steps must count equally, use the space where the quantity
is linear; if equal visible steps must count equally, use encoded." Then label each default row with which of
the two questions the op answers, and note that for linear-encoded data both coincide, which is why the
colleague is unaffected.

**P2-20. Concept order: terms used before their section.**
*Location and fix, one per item:*
"chroma ≈ 0.2" in §3.7 is defined only in §7.9; move the definition (max − min over channels, optionally ÷
mean) into §3.7 and note that it is not CIE chroma C*.
"`SpaceNone // not a level`" in §7.2 precedes the `Level` unit in §7.6; add one sentence in §3.3 introducing
"level: a number on the intensity scale of some space". Note also that `Level` (unit) and `levels` (op) are
homonyms; one sentence prevents the confusion in "`levels.black` … `Level`".
"root frame" (§7.2) is undefined (see P2-10).
"lifted Value node" (§7.6) is undefined (see P2-1).
"issue 10" (§3.7) and "issue 22" (§7.8) are outside the §2.1 table; add both rows.

**P2-21. Decision records are missing for the visible breaks and two structural choices.**
*Location:* §7.5 "`levels` is the one visible behaviour change on sRGB images"; §7.8 macros; §7.3 linear pyramid.
*Problem:* The `levels`/`stats` default is the choice most likely to be re-litigated (agents' recipes change),
and with P1-1 it is not the only break. The alternatives (`levels` default `encoded` with a `SPACE` warning when
black/white come from linear stats; `stats` default `encoded`) are not recorded. "Macros expand at intern time"
and "the source pyramid is linear" are non-trivial and both constrain GCV-002.
*Fix:* Add "Decision: `levels` and `stats` default to linear", "Decision: macros expand at intern time" and
"Decision: sources are stored as a linear pyramid" in the required format, with the `levels` table as evidence.

**P2-22. The cost of `encoded` defaults on sRGB sources is not stated.**
*Location:* §7.1 "two ops that need the encoded version of the same input share one conversion"; §14 "Graph
growth".
*Problem:* The source is linear (§7.3), so on every sRGB file each encoded-default op (masks, specks, contrast,
histogram, overlay, compare) and every look of a linear-space node needs a `convert(→ encoded)` of its input,
cached per scale. The risk section counts 12 MB per conversion but not how often they occur. This is acceptable,
but an implementer reading `explain()` output for the first time should expect it.
*Fix:* Add one sentence to §7.7 or §14: "Every look of a linear-space node of an sRGB image adds one convert
node; this is the price of the linear pyramid and is what `ToImage` paid on each render before."

### P3 — polish

1. **Analogy.** §2.1 "uses goja-cv through MCP as a darkroom". Replace it with what the agent does: "builds
   enhanced views, lists candidate spots and judges magnified crops".
2. **Excerpt mismatch.** §2.1 "`# 82-export-cv-tiles.py, lines 60-63`": the code is at lines 55-59, and the
   trailing comment `# sRGB_encode(v)` is not in the source. Cite 55-59 and mark the added comment.
3. **§3.4 example direction.** Issue 7's measured problem was the decode direction (v stored as code value,
   decoded to linear, grain louder in bright areas). With sRGB_decode the same Δv = 0.05 gives 0.0096 at
   v = 0.10 and 0.0615 at v = 0.60, 6.4× stronger in bright areas. Add this pair next to the encode pair.
4. **I_base.** §2.1 uses I_base without saying it is the intensity transmitted by unexposed film base.
5. **Garbled sentence.** §4.1 "`image.DecodeConfig`, which only knows PNG, JPEG and TIFF formats that
   `golang.org/x/image/tiff` supports" should read "PNG, JPEG, and the TIFF variants `x/image/tiff` supports".
6. **Workaround list.** §4.5 omits `mem.ref = cv.open(p)`, which persists across evals today.
7. **Test location.** The §11 table says `colorSpace` asserts are in `pkg/gojamod/cv/module_test.go`; the only one
   is `pkg/cvsession/session_test.go:378`.
8. **Docs list.** §11 misses `02-units-and-scales.md:25` (the awaited summary lists `docW, docH` and
   `colorSpace`), `05-masks-and-regions.md` and `pkg/doc/08-recipes.md` (`docW`).
9. **`OpenOptions` unused.** §7.4 declares `OpenOptions`, but `Engine.Open(path string, raw map[string]any)` takes
   a map. Keep one.
10. **`box` unit.** §7.4 `box` is in "file px"; give its `ArgSpec` (`number[]`, `Len: 4`, `Unit: Unitless`,
    `Static: true`) so that the binder does not scale it as `DocPx`.
11. **Undeclared helpers.** `buffer.MustSpace` (§7.5) and a `ParseSpace` are not in §7.2.
12. **`encode` special case.** §7.7 "a `KeepEnc` result whose `Enc` comes from its argument" is a fourth result
    rule. Make `Result` a small function `func(args, inputs) Encoding` with the three named rules as values, or
    add `ArgEnc`, instead of a special case.
13. **`convert` op's own space.** Say that `convert` has `Space: SpaceRule{}`, so `Register` does not append a
    `space` option to it, and that `Lookup` from JavaScript and `cv.help` skip `Internal` ops.
14. **`colormap` default range.** "[p01, p99] computed at evaluation" makes `colormap` a `Global` op whose look
    depends on the scale; say so and set `Global: true`.
15. **"RANGE-class warning, PRIMARIES".** §7.9 should say "a warning with code `PRIMARIES`", and §10 should list it.
16. **Value synonyms.** §10 maps `none` and `raw` to `linear`; for the `space` enum this is misleading (raw data is
    closest to `encoded`). Scope value synonyms per enum.
17. **`toGray("rec601")`.** §6 item 10 keeps a mixed-convention option with docs advice. Per the no-compat rule,
    either drop `rec601` or give `toGray` a weights-dependent default space (rec601 → encoded).
18. **Fixture tolerance.** §12 Phase 2 "decodes to the same linear buffer as the sRGB TIFF within 2e-5": a 16-bit
    sRGB code near white spans 3.5e-5 in linear, so half a step plus the linear TIFF's half step exceeds 2e-5.
    Compare each fixture with the float buffer that generated it, with a per-format tolerance of one code.
19. **Full-sentence bullets.** The §13.1 bullets are fragments ("Decoders: fixture equivalence, windows, probes,
    malformed input fuzzing."). Rewrite them as sentences that name the test functions.
20. **Key points and closing.** Only §3 ends with key points; add three to five to §7 and §12, and a closing
    paragraph that hands over to GCV-002 (what it can now assume).
21. **Vague references and a cross-repo step.** "the deep-dive article's lesson about optional parameters" (§7.10)
    and "the deep-dive article's caveat" (§13.5) should state the lesson in one sentence. Phase 7's harness step
    edits files in another repository (CVJS-001 `scripts/06-pi-cv-agent`); say so and give an acceptance
    criterion (for example: the new build answers all three questions with no `E_` errors in both runs).

## Usage Examples

How to work through this list:

1. Fix P1-1 first, because its decision changes §7.5, §11, §13.3 and §14 together.
2. Fix P1-3 (phase order) next; P1-2 (golden harness) and P1-4 (lambda test) become steps inside the new order.
3. P2-1, P2-2, P2-3 and P2-20 are one editing pass over §3.
4. P2-4 to P2-15 are one pass over §7 and §9. After the edit, run the pseudocode by hand on `within`,
   `mask.overlay(img)`, `channel` and `compare([a, b])`.
5. P2-9, P2-17 and P2-19 touch GCV-002/GCV-003; change the shared wording in all three guides in one commit.

### Verification of the guide's code citations

| claim | result |
|---|---|
| `color.go:29-50` transfer functions; `:5-27` decode tables | exact |
| `io.go:67-133` Decode; `:89, :101, :124, :144, :160-162`; `:30-51`; `:169-224`; `:182-190`; `:254-294` | exact |
| x/image `tiff/reader.go:252-262` rejects SampleFormat ≠ 1 (v0.47.0) | exact |
| `engine.go:87-120`, `:113-114`, `:24` (2 GiB) | exact |
| `ops/source.go:34`, `:11-36`; `explain.go:21` | exact |
| `registry.go:106`, `:108`; `help.go:77-79`; no `Space` in `eval.go` | exact |
| op space counts 7/5/3/7/11/5/4/3 | exact |
| §4.3 kernel table (tone 39-54, 59-68; mask 41-52, 134, 262/266; measure 216; draw 100-121; specks 175; ops/mask 105; ops/measure 158) | exact |
| `node.go:21`, `:127-129`, `:67`; unused codes in `errors.go:21-35` | exact (none referenced elsewhere) |
| `init.js:47-56`, `native.go:120-132`, `binding.go:259-271`, `evaltrack.go:197-206`, `processor.go:482-498`, `mcp.go:47` | exact |
| `reference.go:14`, `:15-37`, `:28-29`; `gojamod/cv/help.go:12-17`, `:52-54`; `prelude.js:307` | exact |
| `args.go:556-585`, `:590-605`; `suggest.go:10-41`; `docs_test.go:77-152`; `02-units-and-scales.md:43-49` | exact |
| `06-looks.md` meanL 0.27479 | exact |
| `NewRegion` | does not exist (`NewRegions`, `regions.go:37`) |
| `AxisScales` "subtract the origin" | returns scales; not applicable |
| `module_test.go` colorSpace asserts | actually `session_test.go:378` |
| "golden looks" | none exist |
| `82-export-cv-tiles.py` lines 60-63 | actually 55-59 |
| §3 numbers (0.0039/0.214/0.604; 0.735; 38261; 0.0744/0.0289, 2.57×; mask 188; Lab red 53.24/80.09/67.20) | correct |
| DNG facts (raw0027.dng 7256 × 7326, offset 8, VueScan 9.8.39, PerfectionV600, no black/white/linearization) | match `~/dustds/meta/dng_meta.jsonl` and `dustds/rawio.py` |

## Related

- Guide under review: `../design-doc/01-design-and-implementation-guide.md`.
- Review diary: `04-diary-review-read.md` in this directory.
- GCV-002 guide (uses `DocFrame`, `Frame.Sheet`, `DerivedEnc`, `Expand`, encoded defaults for arithmetic and
  high-pass) and GCV-003 guide (uses `Level`, linear measurement defaults, saved-file metadata read back by
  GCV-001 decoders).

## Disposition (revision 2 of the guide, 2026-11-09)

Section numbers refer to revision 2 of `design-doc/01-design-and-implementation-guide.md`. "Foundations" is
`reference/05-foundations-goja-cv-for-the-gcv-guides.md`.

| finding | disposition | where |
|---|---|---|
| P1-1 | fixed: rule (b), luminance from linear light through a graph-level grey-input rule (`OpDef.Gray`); decision record with the 0.498/0.213 and 0.512/0.471 numbers | §3.5, §7.5 "Grey input", §8, §9.3 |
| P1-2 | fixed: golden-look harness is Phase 0 step 1; expected differences named | §12 Phase 0, §13.3 |
| P1-3 | fixed: phases reordered (golden/dead code, encodings, frames + box/point, spaces + display, decoders, macros/channels, names, docs), each with its after-state | §12 |
| P1-4 | fixed: statics computed in `internNode` from (op, args, inputs); placeholder copies receiver statics; `Substitute` inherits them; lambda test | §7.5, §9.3, §12 Phase 3 |
| P2-1 | fixed: shared Foundations chapter (§F1–§F12) with the CVJS-001 absolute path; guide points to it | How to read, Foundations |
| P2-2 | fixed: "linear in Q", linear light, density numbers (50 %/25 %), "goja-cv does not know the quantity" | §3.1 |
| P2-3 | fixed: `perceptual` dropped; two spaces; decision record | §3.3, §7.2, §8 |
| P2-4 | fixed: masks have `EncNone`/`SpaceNone`, displayed directly; decision retitled | §3.6, §7.2, §8 |
| P2-5 | fixed: `KeepEnc` = first Image input (receiver, Inputs, kind-typed args) | §7.5 |
| P2-6 | fixed: loop guarded by `s != SpaceNone`; three input sources named; `Image[]` elements; compare test | §7.5, §9.2, §12 Phase 3 |
| P2-7 | fixed: `Buffer.Space` dropped; `Convert(ctx, b, enc, from, to)` | §7.2 |
| P2-8 | fixed: values get statics stamped by the evaluator; `NewImage(buf)`; `EvalCtx.InputNode` | §7.2, §7.5, §6 item 6 |
| P2-9 | fixed: `DocFrame` in the `OpDef` block with its default; used by `open` | §7.4, §7.5 |
| P2-10 | fixed: absolute-coordinate rule, out-of-frame `E_ARG_RANGE`, integer origin; `XY` shifts corners/points only. Note: `RegionsValue.NewRegion` does exist (`regions.go:67`); the finding confused it with `NewRegions` | §3.7, §8 frames record, §10 |
| P2-11 | fixed: `as` handled only by the `init.js` wrapper; engine `open` does not know it; bare module rejects it | §7.4, §7.10, §9.1 |
| P2-12 | fixed: unnamed opens listed as `(unnamed) …` | §7.10 |
| P2-13 | fixed: Value nodes carry the measured image's `Enc`; warning only when `!Identity` | §7.2, §7.6 |
| P2-14 | fixed: `Describe` omits `space` equal to the op default; labelled statics column | §7.5, §9.3 |
| P2-15 | fixed: `Register` checks exactly one of `Run`/`Expand`; `CallResult`; Expander collects warnings, may call Internal ops | §7.8 |
| P2-16 | fixed: TIFF primer with the tile's hex dump; own decompression (x/image/tiff/lzw, zlib, PackBits); no x/image fallback | §7.3, §8 |
| P2-17 | fixed: `Info.Origin`, PNG `tEXt` with keyword `goja-cv`, chunk scan, Phase 4 read-back test | §7.3, §12 Phase 4 |
| P2-18 | fixed: `Convert` uses exact functions; LUTs only as a documented fast path; 1e-6 relative | §7.2, §12 Phase 1 |
| P2-19 | fixed: two-question rule; defaults table labels each row | §3.4, §7.5 |
| P2-20 | fixed: chroma defined in §3.8 (not CIE C*); level defined with the homonym note; "root frame" removed; lifting in Foundations §F3; issues 10 and 22 in §2.2 | §2.2, §3.3, §3.8 |
| P2-21 | fixed: records for levels/stats defaults, macros, linear pyramid | §8 |
| P2-22 | fixed: cost paragraph | §7.5 "Cost" |
| P3-1 | fixed: analogy removed | §2.1 |
| P3-2 | fixed: lines 55–59, comment marked as added | §2.1 |
| P3-3 | fixed: decode-direction pair 0.0096/0.0615 (6.4×) | §3.4 |
| P3-4 | fixed: I_base defined | §2.1 |
| P3-5 | fixed | §4.1 |
| P3-6 | fixed: `mem.ref` workaround mentioned | §4.5 |
| P3-7 | fixed: `session_test.go:378` | §11 |
| P3-8 | fixed: `02-units-and-scales.md:25`, `05-masks-and-regions.md`, `pkg/doc/08-recipes.md` added | §11 |
| P3-9 | fixed: `OpenOptions` removed; `Engine.Open(path, raw)` only | §7.4 |
| P3-10 | fixed: `open.box` is `{Type: "box", Unit: Unitless, Static: true}` | §7.4 |
| P3-11 | fixed: `ParseSpace` declared; `MustSpace` dropped | §7.2, §7.5 |
| P3-12 | fixed: `Result` is a function with three named values; `encode` returns its argument | §7.5, §7.7 |
| P3-13 | fixed: `convert` has `SpaceRule{}`; Internal ops skipped by method tables and help | §7.5 |
| P3-14 | fixed differently: the default range is computed at the evaluation scale and the doc says so; `Global: true` declined because `Global` has no reader (GCV-002 P2-7) | §7.9 |
| P3-15 | fixed: code `PRIMARIES`, listed in §10 | §7.9, §10 |
| P3-16 | fixed: value synonyms scoped per enum | §10 |
| P3-17 | fixed: `rec601` removed | §4.6, §6 item 10, §12 Phase 5 |
| P3-18 | fixed: compare each fixture with its generating float buffer within one code | §12 Phase 4 |
| P3-19 | fixed: full-sentence bullets | §13.1 |
| P3-20 | fixed: key points after §7 and §12; closing §15 | §7, §12, §15 |
| P3-21 | fixed: the optional-parameter lesson and the run-variance caveat are stated; Phase 7 names the other repository and gives an acceptance criterion | §7.10, §12 Phase 7, §13.5 |
