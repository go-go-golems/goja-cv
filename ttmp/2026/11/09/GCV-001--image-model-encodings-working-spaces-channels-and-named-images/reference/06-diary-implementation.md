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
