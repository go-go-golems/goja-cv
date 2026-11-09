---
Title: Diary - implementation
Ticket: GCV-005
Status: active
Topics:
    - api
    - agents
    - analysis
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/cv/graph/order.go
      Note: CROP_FIRST deleted; DECORATION_FIRST stays (B1)
    - Path: pkg/cv/kernels/mask.go
      Note: DistanceToSet over squaredDistanceToSet; Grow and Shrink deleted (B1)
    - Path: pkg/cv/kernels/morph.go
      Note: DiskMinMax (chords by half-width, binary fast path), LineMinMax (digital lines), minMax1D without closures (B1)
    - Path: pkg/cv/kernels/morph_test.go
      Note: brute-force disk and line references, fast-path equivalence, duality, opening/closing laws, top-hat on a ramp, benchmark (B1)
    - Path: pkg/cv/ops/morph.go
      Note: erode and dilate ops, disk cap 64 with the chain fix (B1)
    - Path: pkg/cv/ops/morph_test.go
      Note: op equality with the kernels, errors, saturated-blob top-hat vs DoG, crop-first without warnings (B1)
    - Path: pkg/gojamod/cv/help.go
      Note: concept entries opening, closing, tophat (B1)
    - Path: repo://pkg/cv/ops/regions.go
      Note: toMask grow deleted (B1, commit bcefb0a)
ExternalSources: []
Summary: 'Implementation diary of GCV-005: phase by phase, what changed, what failed, timings, deviations and review instructions.'
LastUpdated: 2026-11-09T17:30:00-05:00
WhatFor: Following and reviewing the GCV-005 implementation; continuing it from where the last step stopped.
WhenToUse: Before implementing the next GCV-005 phase, or when reviewing a GCV-005 commit.
---


# Diary - implementation

## Goal

This diary records the implementation of GCV-005 (agent ergonomics and analysis primitives) on
`task/gcv-005-agent-ergonomics-and-primitives`, in the worktree `goja-cv-gcv005`, one step per phase: what changed,
why, what failed, the numbers, and how to review it. The design is the guide `design-doc/01-design-and-implementation-guide.md`
(revision 3); deviations are appended to its "Implementation notes (deviations)" section.

## Step 1: Phase B1 — erode and dilate; grow, shrink, CROP_FIRST and OpDef.Halo deleted

Phase B1 adds the two morphology ops of the guide (§6.9, §7.9, D17–D19) and makes the deletions of §9 items 5–9. Grey
erosion and dilation now exist for every one-channel image, with a disk (distance ≤ r, at most 64 doc px) or a line
element at any angle and length; opening, closing and top-hat are compositions that `cv.help` finds as concept entries.
The mask ops `grow` and `shrink`, `toMask`'s `grow` option, `kernels.Grow`/`Shrink`, the `CROP_FIRST` lint and
`OpDef.Halo` are gone, with no alias or redirect.

It landed as two code commits so that the lint deletion can be looked at (or dropped) on its own: the morphology commit
and the crop-lint commit. No existing golden moved; two new goldens were added.

### Prompt Context

**User prompt (verbatim):** "You are the GCV-005 implementer. Implement Phase B1, morphology with `erode` and `dilate`, from the GCV-005 guide, then stop and report. Later phases wait for GCV-004 batch 2, which is still running. [...] Report back: Your commits. Benchmark timings for `erode`/`dilate` on a 1024² tile and a 4096² image, at r = 3, 16 and 64, plus a line element. What was deleted, as a `git diff --stat`. The goldens that moved. Your deviations. What the next GCV-005 phases need to know."

**Assistant interpretation:** implement guide §13 Phase B1 (kernels, ops, help, deletions, tests, goldens, docs) on the
GCV-005 branch only, gate every commit, record deviations, then hand back.

**Inferred user intent:** land the morphology half of Part B now, while GCV-004 batch 2 runs, in a shape that merges
cleanly with batch 2 later.

**Commit (code):** bcefb0a — "GCV-005 B1: erode and dilate (disk ≤ 64 doc px, line element); delete grow, shrink and toMask's grow"

**Commit (code):** 5c26122 — "GCV-005 B1: delete the CROP_FIRST lint and OpDef.Halo; crop's help says what an op after a crop sees"

### What I did

- `pkg/cv/kernels/morph.go`:
  - `DiskMinMax(ctx, p, w, h, r, isMax)`: r < 1 copies; a plane of only 0 and 1 with r ≥ 8 takes `binaryDisk`; otherwise
    `chordDisk`.
  - `diskChords(r)`: the half-width of the chord at each |dy|, the largest a with a² + dy² ≤ r², corrected by integer
    comparisons so it equals the brute-force definition exactly.
  - `chordDisk`: groups the |dy| by equal half-width (the widths do not increase with |dy|), filters every row once
    per distinct width with `minMax1D`, then combines rows y ± dy of that width into the output.
  - `binaryDisk`: `squaredDistanceToSet` to the 1s (dilation) or the 0s (erosion), compared with r² exactly.
  - `LineMinMax(ctx, p, w, h, r, theta, isMax)`: u = (cos θ, −sin θ); the family of digital lines
    other = c + round(along · slope), along x when |u.x| ≥ |u.y|, else along y; R = round(r · |u.along|); one
    `minMax1D` per line over its in-image run, parallel over lines.
  - `minMax1D` rewritten without closures (padded input in `h`, backward pass in place), same results.
  - `SquareMinMax`, `Opening`, `Closing` unexported (`squareMinMax`, `squareOpening`, `squareClosing`), used only by the
    speck background.
- `pkg/cv/kernels/mask.go`: `DistanceToSet` is now a square root over the new `squaredDistanceToSet` (row pass in place,
  no extra plane); `Grow` and `Shrink` deleted.
- `pkg/cv/ops/morph.go`: `morphOp(name, doc, isMax)` → `erode`, `dilate`: `OneChannel`, `radius` (doc px, positional,
  [0, 2000]), optional `line` (degrees); `Validate` refuses a disk above 64 with `E_ARG_RANGE`, the hint "Crop first, or
  apply a smaller disk twice" and a literal chain fix (`img.dilate(64).dilate(36)`); `Identity` at radius 0; Run on
  plane 0 at `a.Length("radius", scale)`.
- `pkg/cv/ops/mask.go`, `regions.go`: `grow`, `shrink` and `toMask`'s `grow` deleted; `See` lists point at `dilate`.
- `pkg/cv/graph/args.go`: an option given to an op without options now reads `toMask has no option "grow": it takes
  none.` (it read `Valid options: .`).
- `pkg/gojamod/cv/help.go`: concept entries `opening`, `closing`, `tophat` with the compositions; the `mask` entry no
  longer lists grow/shrink.
- Commit 2: `CROP_FIRST` branch of `orderWarnings`, `hasHalo`, `WCropFirst`, `OpDef.Halo` and the 13 `Halo:` functions
  deleted; `crop`'s Doc carries the edge-effect text of §6.10 (with erode/dilate in place of peaks, which B2 adds).
- Docs: `05-masks-and-regions` (new "Erosion and dilation" section, the Converting example), `02-units-and-scales`,
  `06-looks`, `08-errors` (rows and an `E_ARG_RANGE` block), `pkg/doc/02-concepts.md`, README, MCP `Instructions`
  (`m.grow(r)` → `m.dilate(r)`).
- Tests (all new unless noted): see Code review instructions.
- Goldens: added `dilate-mask.png` (the luminance mask dilated by 4) and `tophat-gray.png` (grey minus its opening with a
  disk of 6). The golden check ran before `-update` and passed: no existing golden moved.
- Gate: `GATE_LOG=<scratch>/gate bash ttmp/2026/11/09/GCV-004--*/scripts/04-gate.sh` printed `gate=0` (exit 0) before
  each commit.

### Why

- The chords grouped by width instead of per output row (§10.9's loop order): the same result with no recomputation of
  a row's running extremum for every output row it feeds, and parallel bands of any height.
- `squaredDistanceToSet`: squared distances are integers, so `d2 ≤ r²` selects exactly the offsets of the chords;
  the guide's float32 square root with a 1e-6 tolerance could disagree at large distances.
- `minMax1D` without closures: the first benchmark spent most of its time in closure calls (4096², r = 64: 1.22 s →
  0.33 s after the rewrite); the speck background gets the same speed-up with identical values (goldens exact).
- Two commits: the coordinator's brief said Halo and CROP_FIRST "go in their own phases" while the guide's §9 and
  §13 B1 (and task `pb1`) put them in B1. I followed the guide and kept them in a separate commit that can be reverted
  alone.

### What worked

- The brute-force references matched on the first run for disks (r 0.5–12 by 0.25), lines (16 angles, r up to 60),
  and the binary path (r 8–40 by 0.5).
- All existing goldens stayed exact through the `minMax1D` rewrite.

### What didn't work

- The first binary fast-path test was vacuous: random planes with 8 % ones dilate to all ones at r ≥ 8, so a mutated
  comparison (`v < r2`) still passed. Rewritten with sparse planes (single pixels and small boxes of one value on the
  other); the mutation now fails with `binary r=8 max=false at (41, 5): 1, want 0`.
- `erode(0)` is not the receiver node: `Identity` is applied at evaluation (`graph/eval.go:359`), so the test now checks
  the evaluated buffer is the receiver's.
- Gate failures on the way, each fixed: `masks-and-regions is 7134 characters; keep topics short` (topics are capped at
  6,000; the section was cut to one paragraph and one block), `looks is 6069 characters` (the crop sentence shortened),
  `instructions are 4008 characters, want < 4000` (only `m.dilate(r)` added), `func iou is unused` (lint, after the
  grow/shrink test went), `TestHighpassAndDoGAreTheHandWrittenChains: highpass after crop: []` (it expected
  CROP_FIRST; now expects no warning).
- `go test ./pkg/cv/...` without the gate's environment fails `TestRealHotListMeasures`, `TestRealTileReEncodedAsLinear`
  and `TestRealViewRanking` on the drifted gold data (GCV-004 deviation 1); not touched.

### What I learned

- The topic cap (6,000 characters, `pkg/cv/docs/docs_test.go`) and the instructions cap (4,000) leave little room;
  B3's docs (recipes for line removal and top-hats) belong in `09-recipes`, as the guide plans.
- The distance-transform path is not always faster: on 4096² binary planes it costs about 150 ms at any radius, while
  chords cost 86 ms at r = 16 and 284 ms at r = 64. The guide's threshold r ≥ 8 is kept (the loss is at most ~60 ms at
  4096², and nothing at 1024²).

### What was tricky to build

- **Digital lines with clipping.** Every pixel must lie on exactly one line and its element must be the run on its own
  line. With other = c + round(along · slope), round is monotone in `along`, so each line's in-image samples are one
  contiguous run of steps; `minMax1D` pads outside the run, which equals "samples outside the plane take no part". The
  brute-force reference walks the same definition independently (`y + round((x+k)·slope) − round(x·slope)`).
- **Chord half-widths.** `int(sqrt(r² − dy²))` can be off by one at exact squares; the loops after it fix it with integer
  comparisons in float64, which is what the brute force does.
- **The backward pass in place.** `minMax1D` keeps the padded input in `h` and overwrites it from the end of each block;
  `h[j]` is still the input when it is read because the pass goes downwards.

### What warrants a second pair of eyes

- `LineMinMax`'s convention: R = round(r · |cos|) steps along x for a mostly horizontal line, so the element's end centres
  are about 2r apart (exactly 2r only on the axes). This is §10.10's convention; the docs say "about 2r".
- `chordDisk` memory: two extra planes (`out`, `run`) of the input size.
- The `toMask` change touches `pkg/cv/ops/regions.go`, which GCV-004 batch 2 rewrites (`mask.regions`); at the merge,
  take batch 2's version and make sure it has no `grow` option and calls no `kernels.Grow`.

### What should be done in the future

- B2 (peaks): declare no `Halo` (the field no longer exists; task `pb2` still says "Halo"); add "peaks report cut rings
  (`ringCoverage`)" to `crop`'s Doc; use `DiskMinMax` for NMS (it accepts any r ≥ 1; the 64 cap is the op's).
- A4 / GCV-004 Phase 8: the MCP instructions still say "(crop last; rulers read absolute doc px)"; the §12.1 text
  replaces it.
- B3: the line-removal and top-hat recipes in `09-recipes`, and the gated `TestRealTopHatSaturated` and
  `TestRealLineOpening`.
- Optional: the distance transform's column pass reads with stride w; a blocked transpose would make the binary path
  faster than chords from r ≈ 8 at 4096² too.

### Code review instructions

- Start with `pkg/cv/kernels/morph.go` (`DiskMinMax`, `diskChords`, `chordDisk`, `binaryDisk`, `LineMinMax`,
  `digitalLines`, `minMax1D`), then `pkg/cv/ops/morph.go`, then the deletions in `pkg/cv/ops/mask.go`,
  `pkg/cv/ops/regions.go`, `pkg/cv/kernels/mask.go`, `pkg/cv/graph/order.go`.
- Tests: `pkg/cv/kernels/morph_test.go` (`TestDiskMinMaxMatchesBruteForce`, `TestBinaryDiskEqualsChords`,
  `TestLineMinMaxMatchesBruteForce`, `TestErodeDilateAreDual`, `TestOpeningAndClosingLaws`,
  `TestLineOpeningKeepsLinesDiskOpeningRemovesThem`, `TestTopHatOfDiskOnRamp`, `TestDilateThenErodeIsNearIdentityOnBlobs`),
  `pkg/cv/ops/morph_test.go` (`TestErodeDilateOps`, `TestTopHatKeepsSaturatedBlobFilled`, `TestCropFirstDoesNotWarn`),
  `pkg/gojamod/cv/help_test.go` (`TestHelpMorphologyConcepts`), the registry walk (`erode`, `dilate` samples in
  `pkg/cv/ops/ops_test.go`).
- Validate: `bash ttmp/2026/11/09/GCV-004--*/scripts/04-gate.sh` (prints `gate=0`);
  `go test ./pkg/cv/kernels -run XXX -bench Morphology -benchtime 5x`;
  `go test ./pkg/cv/look -run TestGoldenLooks` (exact).

### Technical details

Benchmarks (`BenchmarkMorphology`, Apple M1 Max, 10 cores, `-benchtime 5x`; random grey planes, and planes of 8 % ones
for the binary path; dilation; the GCV-004 implementer was running on the same machine, so expect ±20 %):

| | r = 3 | r = 16 | r = 64 |
|---|---|---|---|
| disk, grey, 1024² | 3.7 ms | 7.8 ms | 24.8 ms |
| disk, binary (r ≥ 8: distance transform), 1024² | 3.1 ms | 8.4 ms | 8.8 ms |
| line at 30°, 1024² | 1.7 ms | 1.5 ms | 1.6 ms |
| disk, grey, 4096² | 42.5 ms | 86.2 ms | 283.7 ms |
| disk, binary, 4096² | 35.1 ms | 144.8 ms | 153.8 ms |
| line at 30°, 4096² | 27.4 ms | 25.7 ms | 28.3 ms |

Before the `minMax1D` rewrite: disk grey 4096² r = 64 took 1,216 ms, 1024² r = 64 74 ms.

Diff of the phase (`git diff --stat 148f9c0..5c26122`): 38 files, +1,047 / −223, of which about 600 lines are tests,
plus the two PNGs. Deleted: `kernels.Grow`, `kernels.Shrink`, the ops `grow`, `shrink`, `toMask`'s `grow`, `CROP_FIRST`
(`WCropFirst`, `hasHalo`, its branch), `OpDef.Halo` and 13 `Halo:` functions, the exported `SquareMinMax`/`Opening`/
`Closing` names, the `grow`∘`shrink` kernel test.

The disk-cap error as an agent sees it:

```yaml
error:
  code: E_ARG_RANGE
  message: 'dilate: a disk of radius 100 doc px is above the cap of 64.'
  where: 'line 1: return await img.toGray().dilate(100)'
  hint: 'A disk costs about 2·radius passes per row. Crop first, or apply a smaller disk twice: x.dilate(64).dilate(64) reaches about 128.'
  fix: img.dilate(64).dilate(36)
```

## Step 2: Merge the final GCV-004 (batch 2, Phases 5–8) into the GCV-005 branch

GCV-004 finished batch 2 (regions as arrays, the workspace and `cv.save`, names, the documentation sweep) and its pi
validation on `task/gcv-004-api-v2` (`a42c9d1`). This step merges it into the GCV-005 branch with a merge commit, so
that A1–A4 build on the final v2 model: the `items` op, `graph.Workspace`, pending saves waited for by `__cv_run`, the
`UNHANDLED` warning from goja's rejection tracker, and the Phase 8 instructions.

Ten files conflicted, all where B1 and batch 2 touched the same lines (`grow`/`toMask`, the help concept table, the
golden list, the docs). Each was resolved toward batch 2's code with B1's deletions kept: nothing calls `kernels.Grow`,
no `grow` option exists, no op has a `Halo:` field.

### Prompt Context

**User prompt (verbatim):** "You are the GCV-005 implementer, batch A. First, merge the final GCV-004 into the GCV-005 branch. Then implement phases A1, A2, A3 and A4 of the GCV-005 guide. Stop after A4 and report back. [...] Run `git fetch origin && git merge origin/task/gcv-004-api-v2`. Use a merge commit, never a rebase. [...] Run the full gate, commit the merge, push, and write diary step 2 (the merge, what conflicted and how you resolved it)."

**Assistant interpretation:** merge GCV-004's final branch with a merge commit, resolve conflicts toward batch 2 while
keeping B1's deletions, gate, push, and record the resolution.

**Inferred user intent:** put GCV-005 Part A on top of the finished v2 API without losing the B1 cutover.

**Commit (code):** 259df82 — "Merge GCV-004 final (a42c9d1) into GCV-005"

### What I did

- `git fetch origin && git merge origin/task/gcv-004-api-v2` (11 commits, `7ffe54b..a42c9d1`).
- Conflicts and their resolution:
  - `pkg/cv/ops/regions.go`: batch 2's version (`mask.regions`, no `toMask`, no `where`/`sortBy`/`top`); its Doc said
    "Grow it with .grow(r)" and `See` listed `grow`: now `.dilate(r)` and `dilate`.
  - `pkg/cv/ops/regions_test.go`, `heal_test.go`: batch 2's tests with `call(…, "grow", mask.regions…)` replaced by
    `dilate` (the heal test uses 3.0, B1's `grow(2.5)` → `dilate(r + 0.5)` equivalent).
  - `pkg/cv/ops/ops_test.go`: batch 2's walk samples (the Regions ops are gone) and its typed-names test; B1's `erode`
    and `dilate` samples were outside the conflict and stay.
  - `pkg/cv/look/golden_test.go`: batch 2 deleted the tolerance classes (its deviation 49); B1's `dilate-mask` and
    `tophat-gray` cases re-added without a class.
  - `pkg/gojamod/cv/help.go`: batch 2's `mask`, `regions` and `names` concepts (its `mask` text listed `grow, shrink`:
    now "… and erode/dilate need a one-channel receiver") plus B1's `opening`, `closing`, `tophat`.
  - `pkg/gojamod/cv/help_test.go`: both sides (B1's `TestHelpMorphologyConcepts`, batch 2's `stringsOf`).
  - `pkg/cv/docs/topics/05-masks-and-regions.md`: batch 2's "Regions are arrays" and "Literal items" sections; B1's
    separate "Erosion and dilation" section folded into one clause of "Combining and refining" to stay under the
    6,000-character topic cap; the Converting example uses `img.mask.regions(dust).dilate(2)`.
  - `pkg/cv/docs/topics/08-errors.md`, `pkg/doc/02-concepts.md`: batch 2's rows and paragraph with B1's morphology text.
- Not conflicting but wrong after the merge:
  - `pkg/cv/ops/morph_test.go` called `toMask` (gone): it now checks that `mask.regions` refuses a `grow` option.
  - The Phase 8 MCP instructions said "(crop last; rulers read absolute doc px)": now "(ops after a crop see only the
    crop; …)", and "(a plain object that persists)" became "(it persists)" to stay under 4,000 characters (A4 replaces
    the text).
- `grep -rnE 'Halo:|kernels\.(Grow|Shrink)|CROP_FIRST|crop last|\bgrow\b|\bshrink\b' pkg cmd README.md` finds only
  B1's deletion tests and comments.
- Gate: `gate=0`.

### Why

- Batch 2's model is the base A1–A4 need (Appendix A); B1's deletions are the GCV-005 decisions D17, D18, D22, D25.
- Merge, not rebase: the branch is pushed and other workers read it.

### What worked

- `git rerere` recorded every resolution; the gate passed on the first full run after the two cap fixes.
- Batch 2 had already put `m.dilate(r)` in the instructions (the coordinator told it B1's names).

### What didn't work

- First test run after resolving: `masks-and-regions is 6017 characters; keep topics short` and
  `instructions are 4019 characters, want < 4000`. Fixed by shortening the morphology clause and the `mem` note.

### What I learned

- Batch 2 already implements most of §7.4's runtime half: `__cv_run` waits for pending saves (GCV-004 deviation 43) and
  `UNHANDLED` comes from a per-runtime rejection tracker. A1 adds loads to the same pending list rather than building it.

### What was tricky to build

- The masks topic: two independent additions (B1's morphology section, batch 2's array sections) each fitted the cap
  alone and not together. The morphology text became one clause with a pointer to `cv.help("tophat")`.

### What warrants a second pair of eyes

- `pkg/cv/ops/regions.go`'s `mask.regions` Doc now says "Grow it with .dilate(r)"; `dilate(r)` grows by half a pixel
  less than the old `grow(r)` (D18).

### What should be done in the future

- A4: the instructions still mention `UNUSED_NODE` and the old crop wording's successor; §12.1's text replaces them.

### Code review instructions

- `git show 259df82 --cc` shows only the conflict hunks; start with `pkg/cv/ops/regions.go`, `pkg/gojamod/cv/help.go`,
  `pkg/cv/look/golden_test.go`.
- Validate: `GATE_LOG=<scratch> bash ttmp/2026/11/09/GCV-004--*/scripts/04-gate.sh` → `gate=0`.

### Technical details

- `git diff --stat 17dbcb1 259df82`: 138 files, +4,844 / −4,192 (batch 2 itself). Against batch 2
  (`git diff --stat origin/task/gcv-004-api-v2 259df82 -- pkg cmd README.md`): 37 files, +1,027 / −213, i.e. B1 on top
  of v2.
