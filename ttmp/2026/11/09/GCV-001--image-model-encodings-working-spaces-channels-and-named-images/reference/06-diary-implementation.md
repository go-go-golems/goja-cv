---
Title: Diary - implementation
Ticket: GCV-001
Status: active
Topics:
    - image-model
    - io
    - color
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Implementation diary of GCV-001 (image model): one step per phase of the design guide, with commands, failures, review notes and commit hashes."
LastUpdated: 2026-11-09T04:19:18.942636-05:00
WhatFor: "Reviewing and continuing the GCV-001 implementation on branch task/gcv-001-image-model."
WhenToUse: "Before reviewing a GCV-001 commit, or when picking up GCV-002/GCV-003 on top of it."
---

# Diary - implementation

## Goal

This diary records the implementation of GCV-001 (encodings, working spaces, frames, channels, macros and named images)
in goja-cv, phase by phase as the design guide (`design-doc/01-design-and-implementation-guide.md`, revision 3)
prescribes. Each step lists what changed, what failed (verbatim), what to review and the commit hashes. Deviations from
the guide are also appended to the guide's "Implementation notes (deviations)" section.

## Step 1: Phase 0 - golden-look harness, space guard, dead code

Phase 0 builds the safety net before anything in the image model moves: a golden-look test that records what today's
looks of the planted scan are, and a parser-based guard that will forbid private colour conversions in kernels and ops
once Phase 3 has ported them. It also deletes the dead `Node.Base` field and the five error/warning codes nobody used.

Behaviour is unchanged after this step; the goldens record it byte for byte.

### Prompt Context

**User prompt (verbatim):** "Implement GCV-001 following the design and implementation guide (revision 3), phase by phase, with tests, commits and a diary."

**Assistant interpretation:** Work on branch `task/gcv-001-image-model`, implement Phases 0-7 of the GCV-001 guide in order with the tests it asks for first, gate every phase (build, vet, race tests, lint, logcopter-check), commit and push per phase, keep this diary and the ticket tasks in sync, then open a PR and watch CI.

**Inferred user intent:** Get the image-model rework (the foundation of GCV-002 and GCV-003) implemented faithfully and reviewably, with every deviation from the guide recorded.

**Commit (code):** b8a8bf5 - "GCV-001 Phase 0: golden-look harness, space guard test (skipped), remove Node.Base and unused codes"

### What I did
- `pkg/cv/look/look.go`: split `Render` into `image(ctx)` (evaluate + 8-bit image) and the number burn + encode, so the golden test can compare the image before JPEG encoding.
- `pkg/cv/look/golden_test.go`: 24 looks of the planted scan (source, exposure, gaussian, contrast, two levels looks, toGray, ten mask looks incl. feather/refine/overlay, specks overlay, two histograms, three compare modes). Goldens are PNGs in `pkg/cv/look/testdata/golden/`, plus `counts.json` with the speck count of the overlay. Flags: `-update` rewrites; `-golden.tolerant` applies the one-time §13.3 rule (levels/mask/specks classes may change, others at most 1 code on at most 1 % of samples). The mismatch report gives the largest code difference and the number/fraction of differing samples.
- `pkg/cv/kernels/space_guard_test.go`: `TestKernelsAreSpaceAgnostic` parses the non-test files of `pkg/cv/kernels` and `pkg/cv/ops` with `go/parser` and fails on `buffer.ToPerceptual/ToLinear/Linear8/Linear16/EncodeSRGB8/EncodeSRGB16`; `t.Skip` until Phase 3.
- `pkg/cv/graph/node.go`, `errors.go`: removed `Node.Base` and `EStale`, `EReadOnly`, `EFragmentCapture`, `ENoModel`, `WScale`.
- Gate script (scratchpad `gate.sh`): `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, `make lint`, `make logcopter-check`, each with its own exit status.

### Why
- The guide makes the golden harness the first thing written, so that the only phase where looks move (Phase 3) can be checked against a recorded baseline.

### What worked
- `go test ./pkg/cv/look -run TestGoldenLooks -update` then a plain run: exact match. Full gate green.

### What didn't work
- My first gate command contained a stray `cat > /tmp/x.go` (a leftover from an edit), which blocked on stdin; the background job hung until I killed it (`exit code 144`). No code issue; I moved the gate into a script.

### What I learned
- The planted scan's specks overlay finds 41 regions (40 planted specks plus one extra candidate); the count is recorded in `counts.json` for the Phase 3 check.

### What was tricky to build
- Colour looks are JPEG; comparing JPEG bytes would hide or invent differences. The harness therefore compares the 8-bit image before the look number and the encoder, stored losslessly as PNG.

### What warrants a second pair of eyes
- The case list in `goldenCases()`: it should cover every op whose output Phase 3 can move.

### What should be done in the future
- N/A

### Code review instructions
- Start at `pkg/cv/look/golden_test.go` (`goldenCases`, `comparePixels`, the class switch).
- `go test ./pkg/cv/look -run TestGoldenLooks -count=1`; `go test ./pkg/cv/kernels -run TestKernelsAreSpaceAgnostic -v` (skipped).

### Technical details
- Regenerate: `go test ./pkg/cv/look -run TestGoldenLooks -update`. Tolerance check: `go test ./pkg/cv/look -run TestGoldenLooks -golden.tolerant -v`.

## Step 2: Phase 1 - encodings, spaces and exact conversion

Phase 1 adds the vocabulary of the new image model to `pkg/cv/buffer` without using it anywhere: `Encoding` (`srgb`,
`linear`, plus `EncNone` for masks), `Space` (`linear`, `encoded`, plus `SpaceNone`), `Identity` and `Convert`, which
uses the exact transfer functions instead of the interpolated tables.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Implement guide Phase 1 with its tests first.

**Inferred user intent:** A tested conversion primitive the graph can rely on in Phase 3.

**Commit (code):** 41a5773 - "GCV-001 Phase 1: buffer.Encoding, Space, Identity and exact Convert"

### What I did
- `pkg/cv/buffer/encoding_test.go` (written first): the identity table over all encodings and spaces, parse/print round trips (and "perceptual" refused), the §3.2 numbers (0.05 → 0.0039, 0.5 → 0.214, 0.8 → 0.604) and the §3.4 decode steps (0.0096 / 0.0615), round trips within 1e-6 relative on random values in [-0.5, 2], monotonicity, the 8/16-bit tables against the exact functions at every code, and the LUT fast path within 2e-5.
- `pkg/cv/buffer/encoding.go`: the types, `ParseEncoding`, `ParseSpace`, `Identity`, `Convert` (parallel by rows, returns its input when `Identity` holds).
- `color.go`: `ToPerceptual`/`ToLinear` documented as the fast path only.

### Why
- Guide §7.2: conversions must be accurate to float32 precision because they become cached graph nodes shared by many consumers.

### What worked
- All tests passed on the first run; gate green.

### What didn't work
- N/A

### What I learned
- `Identity` also has to hold when either side is `SpaceNone` (masks, counts): the graph never converts those, and treating them as identity keeps the builder loop simple.

### What was tricky to build
- Nothing; the one decision was `Identity(e, from, to)` returning true for every non-sRGB encoding, which makes `EncNone` (masks) and `EncLinear` (data, derived values) behave the same.

### What warrants a second pair of eyes
- `Identity` for `SpaceNone` endpoints (a deliberate widening of the guide's two-row table).

### What should be done in the future
- N/A

### Code review instructions
- `pkg/cv/buffer/encoding.go`, then `encoding_test.go`. `go test ./pkg/cv/buffer -run 'Identity|Convert|Integer' -v`.

### Technical details
- `Convert` maps `linear→encoded` to `LinearToSRGB` and `encoded→linear` to `SRGBToLinear`, computed in float64 per sample and stored as float32.

## Step 3: Phase 2 - frames, box and point types, test source

Phase 2 replaces every `DocW`/`DocH` pair with a `graph.Frame` (integer origin and size in doc px) and makes
coordinates absolute: a box or point an agent passes is in the frame of the file, `Args.XY` subtracts the frame origin
before scaling to buffer px, and regions add it back when they are created. The evaluator, not `Run`, now gives values
their frame. Every frame still has origin (0, 0) for opened files, and all goldens pass unchanged.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Implement guide Phase 2 (frames, box/point argument types, the test-only source op), keep goldens byte-identical.

**Inferred user intent:** Make windows of large scans possible later (Phase 4 `open({ box })`) without any coordinate translation by the agent.

**Commit (code):** 5709062 - "GCV-001 Phase 2: frames, box and point argument types, test source"

### What I did
- `pkg/cv/graph/frame.go`: `Frame` with `Empty`, `String` ("[x, y, w, h]"), `Data`, `ContainsPoint`, `OverlapsBox`, `NearestBox/NearestPoint`, and `SameFrame` (new code `E_FRAME`).
- `Node.Frame` computed in `internNode` by `nodeFrame` (op `DocFrame`, else the first input with a non-empty frame). `OpDef.DocSize` → `DocFrame`. `ImageValue{Buf, Frame}`, `NewImage(buf)`, `NewMask(buf)`; `RegionsValue.Frame`, `NewRegions(w, h, scale, frame)`; summaries carry `frame` instead of `docW/docH`.
- `eval.go`: `stamp(n, v, ins)` after `Run` and on the identity short-circuit; a value that is one of the inputs is shallow-copied first.
- Binder (`args.go`): types `box` and `point` (lists, or objects `{x, y, w, h}` / `{cx, cy}`; `{x, y}` as a point is `E_ARG_TYPE` with the fix `[x, y]`), unions such as `point|number` and `point|enum:top,bottom,left,right` (the alternative is picked by the value's shape), positional coordinate objects are not mistaken for options objects; `Args.XY(name, imageValue)`.
- `ArgSpec.InFrame` (stats.box, mask.box, mask.ellipse, mask.flood): checked in `Intern` against the receiver's frame; the fix is a literal call with the nearest valid box or point (`img.stats({ box: [100, 50, 10, 10] })`).
- Enum errors now carry a literal fix `op({ name: "value" })` (`withEnumFix`) - needed because `mask.gradient.from` is now checked by the binder before the op's `Validate`.
- `pkg/cv/graph/graphtest`: `RegisterSource(reg)` and `Source(e, enc, frame, fill...)`.
- Ops ported (`sameSize` → `sameFrame`, compare's side layout frame, overlays shift region boxes by the frame origin); tests and the four docs lines that read `docW`/`docH`.
- `pkg/cv/ops/frame_test.go`: frames propagate; box/point forms and refusals; absolute coordinates in a window at (100, 50) (mask pixels, coverage at scale 1 and 0.5, component geometry, stats area, out-of-frame errors); `E_FRAME` for blend across frames; returned inputs are stamped copies.

### Why
- Guide §3.7, §7.2, §7.4 and the frames/box-point decision records.

### What worked
- The new tests passed on the first run; goldens byte-identical.

### What didn't work
- `TestGoldenLooks/compare-side` failed at first with "largest difference 216 codes, 294848 of 297216 samples differ": my regex rewrite of `NewImage(x, a.DocW, a.DocH)` also hit compare's `ims[i] = graph.NewImage(kernels.ToRGB(...))`, which dropped the frames the side layout uses for paste offsets. Fixed by building `&graph.ImageValue{Buf, Frame}` there.
- `TestWithinPlaceholderAndParams` failed: `mask.box([0, 0, 0, 0])` is a zero-area box and did not "overlap" the frame. Zero-area boxes now pass when their corner lies in the closed frame.
- Session tests expected `docW/docH` keys and the old flood fix; updated (`fix: img.mask.flood([63, 10])`, `fix: 'mask.gradient({ from: "top" })'`).
- `make logcopter-check`: "generated file is not current: .../pkg/cv/graph/graphtest/logcopter.go" - new packages need `make logcopter-generate`.

### What I learned
- The binder positional loop treats any object as the trailing options object; coordinate objects must be recognised by shape (`isCoordinateObject`) or `mask.box({x, y, w, h})` turns into "unknown option x".

### What was tricky to build
- Where to check "outside the frame": the binder does not know frames, ops' `Validate` gets no nodes, and `Run` is too late for a build-time fix. `Intern` knows the receiver's frame after binding, so the check lives there behind an explicit `InFrame` flag (gradient points may legitimately lie outside the image).

### What warrants a second pair of eyes
- `checkUnion`'s shape-based choice of alternative and the `InFrame` semantics (a box must overlap, not be contained; zero-area boxes allowed at the edge).
- `stamp` relies on pointer identity with the inputs; a `Run` that returns a value it got from elsewhere (not an input) would be stamped in place.

### What should be done in the future
- GCV-003 switches the remaining measurement ops to `box`/`point` (guide note).

### Code review instructions
- Start at `pkg/cv/graph/frame.go`, `node.go` (`checkInFrame`, `nodeFrame`), `eval.go` (`stamp`), `args.go` (`checkBox`, `checkPoint`, `checkUnion`, `XY`).
- `go test ./pkg/cv/ops -run 'Frame|BoxAndPoint|Absolute|TwoFrames|Stamped' -v`; `go test ./pkg/cv/look -run TestGoldenLooks`.

### Technical details
- `Args.XY` shifts elements 0 and 1 (point, or box corner) by the origin; elements 2 and 3 (box size) are only scaled.
