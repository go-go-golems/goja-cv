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

## Step 3: Phase A1 — task libraries: cv.load, cv.scripts, preludes as loads

A1 lets an agent run a JavaScript file in its session instead of pasting it. `await cv.load("dust/helpers.js", args)`
finds the file like `cv.open` finds images (the workspace, then each `--read` directory), runs it as the body of an
async function of `args` on the file's own line numbers, and resolves to what it returns; `cv.scripts()` lists what can
be loaded; `--prelude FILE` is now that load at session start, with the file inside a root. Errors and warnings from a
file name it (`where: dust/helpers.js:6`, `called from: line 2`), and an eval never ends with a load or save still
running.

Most of the runtime half already existed: GCV-004 Phase 6 made `__cv_run` wait for saves and wired an `UNHANDLED`
warning to goja's rejection tracker. A1 generalized that pending list instead of adding a second one.

### Prompt Context

**User prompt (verbatim):** (see Step 2)

**Assistant interpretation:** implement guide §13 Phase A1 (§6.1–6.3, §7.2–7.4, §10.1–10.2, §10.6) on top of the merged
GCV-004, reusing its pending saves and `UNHANDLED`, with the listed tests, docs and help.

**Inferred user intent:** agents stop pasting helper files; a library is loaded by name, and its errors point into it.

**Commit (code):** c0ad935 — "GCV-005 A1: cv.load and cv.scripts; --prelude is a load inside a root"

### What I did

- `pkg/gojamod/cv/prelude.js`: `cv.load` (a JavaScript load stack, depth 32, `__compile` → `f.run(args)` →
  `__pending(inner)` → `inner.then(v => v)`) and `cv.scripts`.
- `pkg/gojamod/cv/load.go` (new): `__compile` (name and path checks, `Workspace.Open`, 1 MiB limit, `goja.Compile` of
  `"(async function (args) {" + text + "\n})"`, the file registry), `syntaxMessage`, `__pending`, `__tooDeep`,
  `__scripts`, `FileLine`, `StackFrames` (frames in `<eval>` or a loaded file, innermost first).
- `pkg/gojamod/cv/save.go`: `track.pending` holds loads and saves; `Pending` and `Settled` replace `SavesPending` and
  `SaveFailure`; unhandled rejections are kept in rejection order and become `UNHANDLED` warnings with a site from the
  reason's stack.
- `pkg/cv/graph/scripts.go` (new): `LoadableName`, `Workspace.ScriptName` (`read<N>:` for shadowed files),
  `Workspace.Scripts` (bounded walk), `firstComment`.
- `pkg/cvsession`: `init.js` waits until no new pending promise appears; `native.go` reads every stack frame
  (`Source`, `CalledFrom`); `processor.go` renders `where: file:line: text` and `called from: line N: text`, and tags
  warnings and LOOKS names with `file:line` (`siteTag`, `evalWhere`); `session.go` checks preludes against the roots
  and runs `await cv.load("<abs path>")` for each, mapping failures to `prelude name:line: message`.
- `cmd/goja-cv`: `checkRoots` checks `--prelude` like `--image` (usage error naming `--read`); flag help says "loaded".
- Help: `load`, `scripts` (`pkg/cv/ops/funcs.go`). Docs: new topic `14-task-libraries.md`, `01-getting-started`,
  `08-errors`, `pkg/doc/03-cli.md`. `make smoke` loads a two-line prelude.
- Tests: `pkg/cvsession/load_test.go` (new; the old `prelude_test.go` is folded into `TestPreludesAreLoads`),
  `pkg/cv/graph/scripts_test.go` (new), `cmd/goja-cv/main_test.go` `TestPreludeFlag` (eval, run, mcp),
  `pkg/xgoja/providers/cv/cv_test.go` (a load from the runtime's root), `TestLookQueue` (its prelude in the workspace).
- Deviations 11–20 in the guide.

### Why

- One pending list and one rejection tracker: the coordinator asked to reuse GCV-004's `UNHANDLED`, and loads and saves
  are the same problem (a promise the eval must not outlive).
- The waiting loop: a pending load may start another load or save after the body settled; waiting for one snapshot
  would let it leak into the next eval (the case §7.4 exists for).
- `UNHANDLED`'s location is computed where the warning is built (cvmod), so it needs no session state.

### What worked

- goja names compiled programs in stack frames exactly as `goja.Compile(name, …)` was given, so `([^\s()]+):(\d+):(\d+)`
  finds file frames, including `read2:twin.js`.
- Wrapping the text on the file's first line kept every line number equal to the file's.
- 40 parallel loads, nested loads and the self-loading file behaved as §6.1 says on the first run.

### What didn't work

- `goja.Compile`'s syntax error is a `*goja.CompilerSyntaxError` with `File == nil` and the message
  `a.js: Line 1:33 Unexpected token ) (and 2 more errors)`; my first version read `File.Position` and printed
  `dust/syntax.js: dust/syntax.js: Line 3:1 Unexpected token } (and 4 more errors)`. Now parsed from the message.
- `TestLoadErrors` expected `called from` for an error thrown after an `await` in the file; the stack of an async
  continuation has no eval frame (deviation 15).
- The docs tests copy `dust/helpers.js` into the planted scan's read directory, which then shadowed the load tests'
  own `dust/helpers.js` (`Object has no member 'f'`); the load tests use `tl/`.
- `TestLookQueue` failed (`prelude … is outside the workspace … add --read …`): its prelude was in a temporary
  directory outside the roots, now refused.
- Gate: `gofmt=1 pkg/cvsession/load_test.go` once.

### What I learned

- replsession's eval in xgoja does not await a returned promise (`[Promise]`); the xgoja test reads the load's value in
  a second eval.
- Rejection tracking only records while an eval is active; a load's own promise always has the derived promise's
  handler, so only the caller's promise can be "unhandled", which is what §7.2 intends.

### What was tricky to build

- **Two promises per load.** `__cv_run` must wait on the file's promise without marking the agent's promise handled,
  or an unawaited failing load would never be `UNHANDLED`. `cv.load` returns `inner.then(v => v)`; the runner waits on
  `inner` (already handled by that `.then`), and the tracker sees only the derived promise.
- **Lock order.** `Settled` collects the rejected promises under the binding's lock, releases it, then locates each
  through `StackFrames`, which takes the lock again to read the file registry.

### What warrants a second pair of eyes

- `cv.load` throws synchronously for path and syntax errors (deviation 13); an unawaited `cv.load("missing.js")` fails
  the eval at its line rather than producing `UNHANDLED`.
- `cvsession.Options.Preludes` must now lie in a root; embedders that passed arbitrary paths break (alpha, by design).
- `session.go`'s `whereFile` regexp reads the rendered YAML to build the prelude message, as before.

### What should be done in the future

- A2: function docs in the context block, and `inPrelude` (the state's `prelude` flag already exists) for `fnSeen`;
  the first agent eval after a prelude must list the prelude's functions' docs (an A1 test item that needs A2).
- B3: replace the model library in `testdata/scripts/dust/helpers.js` and the topic's example with §12.3's peaks version.

### Code review instructions

- Start with `pkg/gojamod/cv/prelude.js` (`load`), `pkg/gojamod/cv/load.go`, then `pkg/gojamod/cv/save.go`
  (`Pending`, `Settled`, `rejection`), `pkg/cvsession/init.js`, `pkg/cvsession/native.go` (`readFailure`),
  `pkg/cvsession/processor.go` (`errorNode`, `siteTag`), `pkg/cvsession/session.go` (`withDefaults`, `runPreludes`).
- Tests: `go test ./pkg/cvsession -run 'TestLoad|TestScripts|TestPreludes|TestUnawaited' -count=1`,
  `go test ./pkg/cv/graph -run TestScripts`, `go test ./cmd/goja-cv -run TestPreludeFlag`, `go test ./pkg/xgoja/...`.
- Gate: `GATE_LOG=<scratch> bash ttmp/2026/11/09/GCV-004--*/scripts/04-gate.sh` → `gate=0`.

### Technical details

`git diff --stat b21d9a3 c0ad935`: 28 files, +1,008 / −204. Deleted: `prelude_test.go` (folded into the load tests),
reading prelude files in `runPreludes`, the "not subject to roots" prelude path, `stackPos`, `whereLine`,
`SavesPending`/`SaveFailure`.

What an agent sees:

```yaml
error:
  code: ReferenceError
  message: nope is not defined
  where: 'dust/helpers.js:6: return nope.y'
  called from: 'line 2: return mem.bad()'
```

```yaml
warnings:
  - 'UNHANDLED dust/late.js:3: a promise was rejected and never handled: late boom. fix: await the call that returned it'
```

## Step 4: Phase A2 — compact tables and function docs

A2 makes results readable without workarounds. An array of flat records (awaited regions, measurement rows, any list
of `{ id, x, y, … }`) now renders as a table, one line per row, exempt from the 48-item and 2,000-character caps that
cut the agents' row lists; the tables of one result share two thirds of the result cap and say how to page. Functions
render as their signature and doc line (the comment that opens the body), and the context block lists `mem`'s
functions and gives the doc line of each new or changed one, once.

The `UNHANDLED` part of A2 was already in place after A1; A2 adds its tests for plain promises. `UNUSED_NODE`'s
deletion is left for A4, as the coordinator's batch plan says.

### Prompt Context

**User prompt (verbatim):** (see Step 2)

**Assistant interpretation:** implement guide §13 Phase A2 (§6.4, §6.5, §7.4–7.5, §10.3–10.4) except the
`UNUSED_NODE` deletion, which the brief moves to A4.

**Inferred user intent:** agents return rows and read them; library functions document themselves where agents look.

**Commit (code):** b10ff2d — "GCV-005 A2: compact tables; function signatures and doc lines in results and the context block"

### What I did

- `pkg/cvsession/tables.go` (new): `renderer.table` (detection, cells, rune-aligned lines, shared budget, paging note),
  `cells`, `cellText`, `renderer.function`.
- `pkg/cvsession/yaml.go`: `render`, `array`, `object`, `property` carry the key path; `dataProperty` (descriptor
  read, accessors never invoked); `keyPath`; `tableLeft` (set from `MaxResultChars` in `Process`).
- `pkg/gojamod/cv/describe.go` (new): `FunctionSource` (the original `Function.prototype.toString`),
  `DescribeFunction`, `FunctionDoc.Signature`, `CutRunes`; a small scanner for balanced parentheses that skips strings,
  template literals and comments.
- `pkg/cvsession/processor.go`: `contextBlock` reads `mem` through descriptors, splits functions from other keys,
  writes `functions:` (signatures cut at 40) and `new functions:` (by `fnSeen`, untouched while a prelude runs);
  `state.fnSeen`.
- Tests: `pkg/gojamod/cv/describe_test.go` with `testdata/helpers-fixture.js`; `pkg/cvsession/tables_test.go`
  (`TestTablesRenderFlatRecords`, `TestTablesAreExemptFromTheCaps`, `TestResultFormat` with
  `testdata/representative.result` and `-update-results`, `TestFunctionDocsInTheContextBlock`,
  `TestPreludeFunctionsAreNewInTheFirstEval`, `TestUnhandledRejections`); updated `TestDocsFromEval`,
  `TestAwaitedRegionsHaveEveryItem`, `TestLongResultsAreCut` (their arrays are tables now).
- Docs: `01-getting-started` (tables, the context block's functions), `13-data-out` (the table budget).
- Deviations 21–25 in the guide.

### Why

- Tables are built as one YAML literal node, never through `primitive` or `budget`, which is what exempts them from the
  caps (D9).
- `FunctionSource` calls the original `Function.prototype.toString`, so a library that overrides `toString` on a
  function cannot change what the context block shows.
- The prelude flag (`state.prelude`, set by `runPreludes`) already existed for looks; the function state reuses it.

### What worked

- goja's `toString` returned exact sources for every form in the fixture, comments included.
- yaml.v3 writes the tables as literal blocks as long as no line has trailing spaces (the lines are trimmed).

### What didn't work

- The first fixture had `mem.best`'s comment on the line after `{` with nothing before it; by D7 that is a doc line
  (first token of the body), so the "mid-body comment" case needed a statement before the comment.
- `goja` refused the fixture's `async *gen(a) {` ("Async generators are not supported yet").
- My first expectations for the 40-rune cut and for the literal block's trailing newline were wrong
  (`mem.loooooooooooooooong({ alpha = 1, be…`).
- Lint: `ineffectual assignment to body`, `func hasWord is unused`, `QF1001: could apply De Morgan's law`.

### What I learned

- A function assigned to a member (`mem.probe = async (x) => …`) has an empty `name` in JavaScript; naming a returned
  function needs the key path or a lookup in `mem`.
- `toJS` sorts the keys of Go maps, so tables of Go-made records (help, docs search) have sorted columns, while
  `graph.Record` and JavaScript objects keep their order.

### What was tricky to build

- **Detecting a table without running code.** A record must be read through property descriptors (no getters, no
  proxy traps) and every cell must be a primitive; checking all 100,000 rows of an awaited region set through
  descriptors would cost seconds, so only the header rows and the shown rows are read that way (deviation 21).
- **Accessors versus methods named get/set.** `get count() {` is an accessor; `get(k) {` is a method. The parser treats
  `get`/`set` as a keyword only when an identifier follows.

### What warrants a second pair of eyes

- `renderer.function` names a bare function by searching `mem` (one descriptor read per key); results with many
  functions on a large `mem` pay that per function.
- The "[Getter]" text is now also used for setter-only properties.

### What should be done in the future

- A4: the docs and recipes that return `rows.map(x => [...].join(" "))` should return the rows (tables); the
  instructions say "a list of flat rows comes back as a table".
- B2: add measured peaks to `testdata/representative.result` (§14.1).

### Code review instructions

- Start with `pkg/cvsession/tables.go`, then `pkg/gojamod/cv/describe.go`, then `contextBlock` in
  `pkg/cvsession/processor.go`.
- `go test ./pkg/cvsession -run 'TestTables|TestResultFormat|TestFunctionDocs|TestPreludeFunctions|TestUnhandled' -count=1`;
  `go test ./pkg/gojamod/cv -run TestDescribeFunction`.
- Gate: `gate=0`.

### Technical details

`git diff --stat 2d469fb b10ff2d`: 14 files, +887 / −34.

A table over the budget:

```yaml
rows: |-
  id   x      name         ok     n
  0    0      "two words"  true   3
  1    1.5    a            false  -
  …
  174  261    a            true   -
  … rows 1–175 of 300 shown. Next: rows.slice(175, 350); all: await cv.save("rows.csv", rows)
```

The context block after a prelude, in the first agent eval:

```yaml
context:
  mem: lin (#2 Image)
  functions: mem.probe(x, y, r = 3), mem.sheet(items, size = 40), mem.spots(level = 0.3)
  new functions: |-
    mem.probe(x, y, r = 3): core/ring measure at (x, y): { cx, cy, step, spread, snr }
    mem.sheet(items, size = 40): contact sheet of up to 15 items: image | speck view, captioned with their ids
    mem.spots(level = 0.3): blobs of the speck view above level, measured on the linear image
```

## Step 5: Phase A3 — sheet captions list their cells; labels id | none

A3 makes a contact sheet carry its mapping as text: the caption of a sheet of an array lists the label (else id) of
each cell in cell order, so a model maps cell 7 to a row by reading, never by squinting at a burned-in number. A sheet of
a detector's Regions node, whose ids exist only after evaluation, says "cells in region order". The `index` label mode
is deleted; `labels: "none"` keeps the caption list.

### Prompt Context

**User prompt (verbatim):** (see Step 2)

**Assistant interpretation:** implement guide §13 Phase A3 (§6.7, §7.7, D11) on top of GCV-004 Phase 5's items.

**Inferred user intent:** sheets come with the ids they show, as data.

**Commit (code):** c9987c0 — "GCV-005 A3: sheet captions list their cells; sheet labels are id | none"

### What I did

- `pkg/cv/graph/registry.go`: `OpDef.Short`; `pkg/cv/graph/explain.go`: `Short` uses it.
- `pkg/cv/graph/items.go`: `ItemTexts` (label, else id, else position).
- `pkg/cv/ops/sheet.go`: `sheetCaption` (40 cells, `… (N more)`), `labels` enum `id | none`, the index branch deleted,
  the op's Doc.
- Tests: `pkg/cvsession/sheet_test.go` (`TestSheetCaptionListsCells`: linked items, a labelled literal with a space, a
  point, a Regions node with `max`, `labels: "none"`, 45 items cut at 40, `labels: "index"` refused).
- Golden: ran `go test ./pkg/cv/look -run TestGoldenLooks` first (only `sheet-2x2` failed: 99 of 113,472 samples), then
  updated it alone; looked at the PNG: labels "1 (201, 47)" and "2 (55, 156) ×3.2".
- Docs: `06-looks` (the caption sentence; the topic was trimmed to stay at 6,000 characters).
- Deviations 26–28.

### Why

- The caption is built when the look is created, before evaluation, so only an items node (an array) can list ids;
  `ItemTexts` reads the entries GCV-004 Phase 5 already interns (each linked item's id and label are copied in).

### What worked

- The look caption already ran through `graph.Short` per chain step; one hook gave the sheet its own text.

### What didn't work

- `go test … -run 'TestGoldenLooks/sheet-2x2' -update` also rewrote `counts.json` to `{}` (the counts of the cases that
  did not run were dropped); restored it with `git checkout` and reran the full golden test.
- `looks is 6043 characters; keep topics short`, then `6003`: trimmed two sentences.

### What I learned

- `-update` on a subset of the golden cases is not safe for `counts.json`; update all cases or restore the file.

### What was tricky to build

- Nothing deep; the subtle part is the count versus the list when `max` is given (the count is the array's length,
  the list stops at `max`).

### What warrants a second pair of eyes

- `ItemTexts` formats a numeric id with `fmtNum` (whole numbers print as integers), the same as `Region.Text`.

### What should be done in the future

- A4: the recipes and docs that keep parallel arrays to map cells to rows should read the caption instead.

### Code review instructions

- `pkg/cv/ops/sheet.go` (`sheetCaption`), `pkg/cv/graph/items.go` (`ItemTexts`), `pkg/cv/graph/explain.go` (`Short`).
- `go test ./pkg/cvsession -run TestSheetCaptionListsCells`; `go test ./pkg/cv/look -run TestGoldenLooks`.

### Technical details

`git diff --stat b0ae2b3 c9987c0`: 8 files, +98 / −8. A caption:
`[look 1: a — sheet(6 items; cells: 9 11 14 40 "big one" 6) (…)]`.

## Step 6: Phase A4 — UNUSED_NODE deleted; the Part A documentation sweep

A4 deletes `UNUSED_NODE` with all the per-eval node tracking that only it read, and rewrites what agents and people
read for Part A: the MCP instructions (§12.1 without the peaks line), the eval tool description, the guidebook's
agents, setup, recipes and troubleshooting pages, the README, and the doc examples that still returned joined strings.

### Prompt Context

**User prompt (verbatim):** (see Step 2)

**Assistant interpretation:** delete `UNUSED_NODE` entirely (moved here from A2 by the brief) and do §12.2's A4 rows:
instructions, recipes (task library, thread width with `profile`, the playbook pattern), README, `help setup-*`.

**Inferred user intent:** the lint that was wrong 18 times in 19 goes, and agents learn task libraries, tables and the
"nothing happens unless awaited, looked at or saved" rule from what they read first.

**Commit (code):** eaafc1d — "GCV-005 A4: delete UNUSED_NODE"

**Commit (docs):** 7cec0bf — "GCV-005 A4: Part A documentation sweep"

### What I did

- Deleted `unusedNodes`, `onPlaceholder`, `WUnusedNode`, `NodeUse`, `EvalTrace.Created`/`Used`, `MarkUsed`,
  `noteUsed` (and its four callers), `renderer.nodes`, `namedNodes`; `state.warnings` lost its `returned` and `lint`
  parameters; `noteCall(ws)` records warnings only.
- Tests: `TestNoUnusedNodeWarning` (the study's `mem.views()` bundle, `img.gaussian(2)` alone, a `within` builder, a
  discarded chain: no warning at all); `TestM3KindsInTheSession` lost its `UNUSED_NODE` expectation;
  `TestEvalTrackingRecordsWarningSites` checks only warning sites.
- Docs for the rule: `03-nodes-and-evaluation` ("A node you don't use does nothing", with an unawaited save that is
  still written), `01`, `08`, `pkg/doc/02-concepts.md`.
- `pkg/cvsession/reference.go`: new `Instructions` (3,503 bytes), `EvalToolDescription` names `cv.load`/`cv.scripts`;
  `TestMCPInstructionsAreCompactAndDocsAreResources` also checks `cv.load(`, `cv.scripts()`, `x.dilate(r)`,
  "does nothing", "table".
- Guidebook: `04-agents` (task libraries and the study), `05`/`06`/`07` setup pages (`--read DIR --prelude
  DIR/lib.js`), `08-recipes` (write a task library, the width of a thread, the playbook pattern), `10-troubleshooting`;
  README (CLI and MCP sections).
- Topics: `14-task-libraries` gains the thread-width pipeline; `05`, `09`, `12` return rows.
- Deviations 29–32.

### Why

- Deleting the tracking with the lint: `Created`/`Used` had no other reader (grepped: looks and `SiteWarning` keep their
  own records), so keeping them would be dead state per eval.
- The instructions follow §12.1 but keep GCV-004 Phase 8's tested calls, so nothing the pi runs relied on disappears.

### What worked

- After the deletion the full suite passed on the first run except the one docs block that expected the warning.
- `cv.docs.search("width of a line")` (the example in the instructions) ranks the new thread-width section first.

### What didn't work

- `TestDocsCodeBlocksRun/nodes-and-evaluation`: `want warning UNUSED_NODE` (the topic's `warns=UNUSED_NODE` block);
  replaced by the new section.
- First instructions draft failed `instructions lack "img.stats()"` (the test's literal list).

### What I learned

- The instructions budget still has about 500 bytes after §12.1's text; B3's peaks line fits.

### What was tricky to build

- Keeping the masks, looks and recipes topics under 6,000 characters while converting examples to rows: `rows: keep`
  (the items' own table) is shorter than picking fields.

### What warrants a second pair of eyes

- `TestNoUnusedNodeWarning` asserts no `warnings:` at all for those evals; a future warning on them would fail it.
- `noteCall` now takes the site for every call with warnings, also for a macro's extra nodes (it used to attach them
  to the first node only; the result is the same list).

### What should be done in the future

- B3: the instructions' peaks line ("Find spots: await img.as(\"linear\").peaks({ radius: 3, snr: 1.4 }) …") and the
  recipes of §12.2's B3 rows; replace the model library (`pkg/cvsession/testdata/scripts/dust/helpers.js`, the
  guidebook recipe) with §12.3's peaks version.

### Code review instructions

- `git show eaafc1d` (deletions in `pkg/cvsession/processor.go`, `pkg/gojamod/cv/evaltrack.go`), then `git show 7cec0bf
  -- pkg/cvsession/reference.go pkg/doc/08-recipes.md`.
- `go test ./pkg/cvsession -run 'TestNoUnusedNode|TestMCPInstructions|TestDocs|TestGuide' -count=1`.
- Gate: `gate=0`.

### Technical details

`git diff --stat e5ec24d eaafc1d`: 14 files, +50 / −242. `git diff --stat eaafc1d 7cec0bf`: 13 files, +186 / −40.
Batch A without the ticket (`git diff --stat 259df82 7cec0bf -- . ':!ttmp'`): 62 files, +2,218 / −517.

## Step 7: Phase B2 — peaks

B2 adds `x.peaks({ radius, snr, polarity })`: the local maxima of `measure`'s core/ring step, as a Regions node of
disks with sub-pixel centres and `measure`'s fields, sorted by |snr|. It is the "coordinates as data" half of the design:
an agent asks for the spots instead of reading them off a ruler. A peak's numbers are exactly what `measure` reports at
the same centre and radius (tested to 1e-9).

### Prompt Context

**User prompt (verbatim):** "You are the GCV-005 implementer, batch B: implement phases B2 (peaks) and B3 (recipes and gold-based real-data tests) of the GCV-005 guide, run the validation (phase V), then open the PR. [...] B2: peaks — Follow the guide: `x.peaks({ radius = 3 | [radii], snr = 1, polarity })` on any image, through the luminance, at analysis scale 1. Step map: core minus ring over [1.5r, 2.5r], using disk chord sums over row prefix sums, processed in row bands. Non-maximum suppression with `kernels.DiskMinMax`. Parabolic sub-pixel fit. An exact `measure`-equivalent at each candidate, then a |snr| filter. Across radii, of overlapping cores the higher |snr| wins. Returns a Regions node of disks with the geometry, `r`, and the core/ring fields. The snr of a peak must equal what `measure` reports at that point and radius. Test that directly. Benchmarks: a 1024² tile, a 4096² image, and an extrapolation to 53 MP. [...]"

**Assistant interpretation:** implement §13 B2 (§6.8, §7.8, §10.7–10.8, D12–D16) with its tests, the golden, the result
fixture, crop's doc sentence, and benchmarks; honour batch A's notes (no Halo, result fixtures, topic caps).

**Inferred user intent:** spots as data, numerically identical to `measure`, fast enough for a tile in a fraction of a
second.

**Commit (code):** 0e04924 — "GCV-005 B2: peaks — sub-pixel maxima of the core/ring step as disks with measure's fields"

### What I did

- `pkg/cv/kernels/peaks.go`: `Peaks(ctx, L, PeakOptions{Radii, MinSNR, Polarity})`.
  - `newPeakRadius(r)`: chord half-widths of the digital disks r, 1.5 r, 2.5 r (`diskChords`, B1's exact rule) and the
    digital ring's offsets for the pre-filter.
  - `stepRows`: row prefix sums (float64) of the rows a band reads, then per pixel the disk sum and count from two
    lookups per chord, clipped at the image; step = core mean − ring mean over in-image pixels.
  - `bandPeaks`: NMS with `chordDisk` (DiskMinMax's chord path) on the band's step map plus ⌈r⌉ + 1 margin rows;
    candidates `T > 0 && T == D`; the digital-ring snr pre-filter (drop below 0.5 × MinSNR); `parabola` on both axes
    (δ = 0 on the image border, clamped to ±0.5); `ProbePoint` at the refined centre decides (`s·step > 0`,
    `s·snr ≥ MinSNR`) and gives the numbers.
  - `mergePeaks`: sort by |snr|, |step|, cy, cx, r; keep a peak unless a kept one lies closer than the sum of radii
    (grid buckets of side 2·max r).
  - Every (radius, band) is one task of the new `parallel.Each`; inside a task everything is serial (`serialRows`).
- `pkg/cv/kernels/morph.go`: `chordDisk` takes a row runner (`parallel.Rows` for `DiskMinMax`, `serialRows` inside a
  peaks band).
- `pkg/cv/parallel/parallel.go`: `Each(ctx, n, fn(i) error)`, a bounded task group without `Rows`' 16-row minimum.
- `pkg/cv/ops/peaks.go`: the op (`Luma`, `AnalysisScale` 1, `radius` `number|number[]` doc px [1, 64], `snr` ≥ 0,
  `polarity` enum), `peakFields` = geometry + `r` + `coreRingFields("off")`; Disk regions with `ID` = rank.
- `crop`'s Doc: "peaks report cut rings (ringCoverage)".
- Tests: `pkg/cv/kernels/peaks_test.go` (step map = brute force for r 1–8 and partial row ranges; Gaussian blobs σ 1–3,
  amplitude 0.05–0.5 on noise: every blob found, centre error at snr ≥ 5 median 0.045–0.053 px, max 0.12–0.17 px;
  the pre-filter changes nothing at snr 0.5/1/1.4/2 on three noise levels; polarity; flat disk r = 10: one peak at
  r = 8, a rim ring of 8 peaks at radius 2 with snr ≤ 0.42; local noise: snr 4.64 quiet/dark vs 0.99 noisy/bright;
  bands of 64, 7 and all rows give identical peaks); `pkg/cv/ops/peaks_test.go` (planted scan: 34 of 34 compact specks
  found within 1.5 px with their polarity; equality with `measure` to 1e-9; toGray and scale 0.25 identical; crop;
  argument errors; `E_FIELD_EXISTS` without a prefix; consumers `measure` with prefix, `mask.regions` (area = Σπr²
  within 5 %), `overlay`, `heal`, `sheet` ("cells in region order")); registry walk and schema property cover peaks.
- Goldens: `overlay-peaks.png` added. The B1 cases `dilate-mask` and `tophat-gray` had been dropped from
  `golden_test.go` by the GCV-004 merge (step 2) while their PNGs stayed; restored, and both matched exactly before the
  full `-update`, which changed only the new file (`counts.json` unchanged).
- `pkg/cvsession/testdata/representative.result` gains `peaks` (measured on the speck view with prefix `sp`);
  `TestPeaksRenderAsAPagedTable`: 596 bare peaks at snr 0 render as a table with "… rows 1–50 of 596 shown. Next:
  this array.slice(50, 100); all: await cv.save("rows.csv", this array)".
- Docs: a new topic `15-spots` (peaks, the speck step of peaks, a sheet, top-hats, line removal); `09` ("Inspect
  candidates" uses peaks; "Rank candidates" points to spots), `11` (the speck scale note of §6.10), `12` (the old
  `mem.probe` line replaced by the peaks pointer).

### Why

- Bands as parallel tasks: the first version ran bands one after another with `parallel.Rows` inside each phase; the
  profile was 60 % `pthread_cond_wait`/`usleep`/`madvise` (about 200 sync points per tile). Whole bands per goroutine
  halved the time (1024², [2, 4, 8]: 187 → 101 ms; 4096²: 2.03 → 1.39 s).
- Pre-filter at 0.5 × MinSNR: at 0.8 (the guide's 20 %) the filtered run lost 6–9 of ~235 peaks at snr 0.5. The ratio
  digital/exact snr at NMS candidates had a minimum of 0.63 (r = 1.5: digital disks of small radii are coarse), 0.90 at
  r = 2, ≈ 1 from r = 4.

### What worked

- The brute-force step map matched on the first run; the measure equality held exactly (same `ProbePoint`, same
  plane: the luminance input is what `measure` uses as its gray plane).

### What didn't work

- `TestPeaksPrefilterKeepsEveryPeak` failed at 0.8 (see Why).
- The first benchmark hung: the synthetic field generator looped over every blob for every pixel (16 k blobs × 16 M
  pixels at 4096²); rewritten to touch only each blob's window.
- `TestSchemaProperty`: "Image:peaks returns Regions and is in neither schemaCalls nor schemaScalar" → added.
- Crop equality: peaks of a crop far from its border equal the whole image's only to ~1e-5 in snr (the row prefix sums
  and the coverage weights start at the crop's corner); the test compares to 1e-4.
- Docs: the recipe's zoom box reached outside the frame (first peak at y 182: `RANGE`); it takes the strongest peak
  away from the bottom edge.

### What I learned

- On the flat disk, rim peaks at radius 2 have snr ≤ 0.42 (blob and background in the ring), the blob's own peak at
  r = 8 snr 70: D15's rule works by a wide margin.
- The planted scan's dark specks measured on the bright-only speck view have negative `spStep` and huge `spSnr`
  (806.98 in the fixture): the clipped-view warning in the docs is real.

### What was tricky to build

- Band margins: step rows [y0 − ⌈r⌉ − 1, y1 + ⌈r⌉ + 1) give exact NMS and the parabola's ±1 rows; prefix rows extend
  ⌊2.5 r⌋ further. Band invariance is exact because each row's prefix sums do not depend on the band.
- NMS on float32 step values compares exactly with the chord maximum of the same values (plateaus give several
  candidates; the merge keeps one).

### What warrants a second pair of eyes

- `peakPrefilter = 0.5` is an empirical bound on synthetic fields; on real grain the ratio could be lower at r = 1–1.5.
  Setting `noPrefilter` makes the kernel exact at a cost (real tile [2, 4, 8] at snr 1: see Technical details).
- Peaks on a plateau (a flat saturated top) are placed anywhere on it (the flat disk's peak is 1 px off centre); the
  test allows 1.5 px.

### What should be done in the future

- B3: gold tests and the snr sweep decide the default.

### Code review instructions

- `pkg/cv/kernels/peaks.go` top to bottom, then `pkg/cv/ops/peaks.go`; tests in `pkg/cv/kernels/peaks_test.go`,
  `pkg/cv/ops/peaks_test.go`, `pkg/cvsession/tables_test.go` (`TestResultFormat`, `TestPeaksRenderAsAPagedTable`).
- `go test ./pkg/cv/kernels -run 'Step|Peaks' -v`; `go test ./pkg/cv/kernels -run XXX -bench Peaks -benchtime 3x`;
  gated `go test ./pkg/cv/ops -run XXX -bench RealPeaks -benchtime 5x`.
- Gate: `gate=0`.

### Technical details

Benchmarks (Apple M1 Max, 10 cores; the machine had a load average of 60–70 from other agents, so expect noise: the
first real-tile run read 4× slower than the repeat):

| image | radius | snr | peaks | time |
|---|---|---|---|---|
| synthetic 1024² (noise σ 0.02, 1,024 blobs) | 3 | 1 | — | 25 ms |
| synthetic 1024² | [2, 4, 8] | 1 | — | 101 ms |
| synthetic 4096² | 3 | 1 | — | 315 ms |
| synthetic 4096² | [2, 4, 8] | 1 | — | 1.39 s |
| real tile `1d955ef4…` 1024² (density, linear) | 3 | 0.5 / 1 | 2,475 / 106 | 61 / 36 ms |
| real tile `1d955ef4…` 1024² | [2, 4, 8] | 0.5 / 1 | 6,027 / 414 | 166 / 95 ms |

Budget (§7.8): 0.5 s per 1024² tile with [2, 4, 8] — met (95 ms at snr 1, 166 ms at 0.5). Extrapolated linearly to a
53 MP scan: 4.4 s from the synthetic 4096² figure, about 5 s at snr 1 and 9 s at snr 0.5 from the real tile — well inside
the 60 s eval timeout; the op's doc says "about 0.1–0.2 s per megapixel with three radii".

`git diff --stat 6a2e4af 0e04924`: 22 files, +1,219 / −19.

## Step 8: Phase B3 — gold tests, the gold-metrics script, Part B documentation

B3 measures peaks, top-hats and line openings on the colleague's gold tiles, relative to the hot list the annotator
agents used, and finishes what agents read about Part B: the instructions' peaks line and a model task library built
on peaks.

### Prompt Context

**User prompt (verbatim):** (see Step 7) "B3: recipes and real-data tests — The recipes, the instructions line and the task-library model, as the guide specifies. The gated tests the guide names, including `TestRealTopHatSaturated` and `TestRealLineOpening`."

**Assistant interpretation:** §14.4's five gated tests with GCV-004's relative pattern, §15.2's script, §12.1's peaks
line, §12.3's library in testdata, the guidebook recipe and the task-libraries topic.

**Inferred user intent:** know, with numbers, whether peaks beat the hot list on gold before agents are told to use it.

**Commit (code):** 965358c — "GCV-005 B3: gold tests for peaks, top-hats and line openings; peaks in the instructions and the model library"

### What I did

- `pkg/cv/ops/peaks_gold_test.go`: a `goldSession` per labelled tile (its own engine; tile opened with
  `encoding: "linear"`, gold shifted by the sidecar origin), the DUST-002 hot list (speck view > 0.3, components of at
  least 4 px), the helpers' probe snap (best snr within 2 px, core 3, ring [6, 12]), and the five tests of §14.4. Gold
  `r` is the label's extent (covered within max(r, 3) px, the prototype's and the hot list's rule); a gold point is
  measured with a core of r / 2 (dark-area ring mean, saturated core maximum), per GCV-004's re-tune.
- `scripts/01-gold-metrics.sh`: runs those tests and the real-tile benchmark, prints the numbers.
- `Instructions`: "Find spots: await img.as("linear").peaks({ radius: [2, 4, 8] }) → disks …" and "take it from data
  (peaks, items, measure)"; 3,776 bytes.
- Model library: `pkg/cvsession/testdata/scripts/dust/helpers.js`, `pkg/doc/08-recipes.md` ("Write a task library")
  and the `14-task-libraries` example are §12.3's (`mem.tile`, `mem.spots` = peaks + speck step, `mem.sheet`), with
  `mem.img = img; mem.lin = img.as("linear")` at load so the library works before `mem.tile`; `03-cli.md`'s prelude
  example calls `mem.spots({ snr: 1.4 })`; the playbook pattern names `peaks`.
- `15-spots`'s line-removal example uses radius 12 (see What didn't work).

### Why

- The tests compare methods on the same points in one run and log their numbers, so label drift moves both sides.
- `mem.img`/`mem.lin` defaults: the docs tests run the library on the planted scan without a tile; on gold tiles
  `mem.tile` replaces both.

### What worked

- Position, coverage, dark areas and saturated blobs passed the guide's targets on the first run (numbers below).

### What didn't work

- `TestRealLineOpening` at the guide's radius 10: elongated components 40 → 3, but dust coverage 164 → 160 (97.6 %,
  target 98 %). The lost points: two saturated blobs (fit radius 18.5 and 28.2, which hold 20 px segments) and three
  smaller ones whose component changed shape. Radii tried: 12 → 40 → 8, 164 → 163; 15 → 40 → 15, 164 → 164; 20 →
  40 → 28, 164 → 164. The test and the doc example use 12; the targets are unchanged. Recorded as a deviation for the
  coordinator.
- `TestRealTopHatSaturated` panicked first: `stats` has no `max`; it reads `percentiles: [1]`.

### What I learned

- Peaks' positions are about three times tighter than the snapped hot list's (median 0.36 vs 0.99 px), and the p90s
  are equal (1.70 vs 1.68): the hot list's best cases are as good, its typical case is not.
- The snr sweep is steep: 95.8 % / 90.5 % / 70.5 % / 45.8 % coverage at 0.5 / 1 / 1.4 / 2, with 5,298 / 354 / 43 / 10
  peaks per tile (median).

### What was tricky to build

- The probe snap evaluates one `measure` call per tile over 13 grid points per hot spot, and picks the best per spot by
  position in the result (the items keep their order).

### What warrants a second pair of eyes

- "Not near gold" counts candidates farther than max(r, 3) from every gold point of any type (dust, hair, scratch).
- The saturated subset uses gold r ≥ 6 (the fit radius) as the guide says; that is a speck radius of about 3.

### What should be done in the future

- Phase V decides the default snr from these numbers; the coordinator decides on the line radius.

### Code review instructions

- `pkg/cv/ops/peaks_gold_test.go`; `bash ttmp/2026/11/09/GCV-005--*/scripts/01-gold-metrics.sh`.
- `git show 965358c -- pkg/cvsession/reference.go pkg/cvsession/testdata/scripts/dust/helpers.js`.
- Gate: `gate=0`; all real-data tests: `go test ./pkg/cv/ops -run TestReal` ok.

### Technical details

`scripts/01-gold-metrics.sh` on 15 labelled tiles, 190 dust points (gold of 2026-11-09):

| metric | peaks | hot list (speck view 0.3) |
|---|---|---|
| position, dust r ≤ 6 (172 points): matched within r, median / p90 distance | 162, 0.36 / 1.70 px ([2, 4], snr 0.5) | 147, 0.99 / 1.68 px (probe-snapped) |
| coverage within max(r, 3), snr ≥ 0.5 | 182 (95.8 %), 5,298 peaks/tile, 5,271 not near gold | |
| snr ≥ 1 | 172 (90.5 %), 354 (58–921), 346 not near gold | 164 (86.3 %), 71 (11–192), 65 not near gold |
| snr ≥ 1.4 | 134 (70.5 %), 43 (10–140), 35 not near gold | |
| snr ≥ 2 | 87 (45.8 %), 10 (1–43), 7 not near gold | |
| dark-area dust (ring < tile p25), 33 points | 31 | 31 |
| saturated or large dust, 63 points | top-hat (r 12) 60; peaks [2, 4, 8, 16] 60 | 59 |
| line openings on the speck view (r 12): elongated components | 40 → 8 | |
| … dust covered | 164 → 163 | |
| peaks time per tile ([2, 4, 8], snr 1, through the graph) | median 0.081 s, max 0.104 s | |

## Step 9: Merge the GCV-004 review fixes (e0823da) into the GCV-005 branch

PR #6's review produced two fixes on `task/gcv-004-api-v2` (`e0823da`): temporary files of `Workspace.Write` are
created exclusively under a random name, and `__cv_run` drains saves that another save's continuation started. The
coordinator asked for them in GCV-005 before its PR, as a merge commit.

### Prompt Context

**User prompt (verbatim):** "Before you open the GCV-005 PR, merge the GCV-004 review fixes now on `origin/task/gcv-004-api-v2` (commit `e0823da`). Run `git fetch origin && git merge origin/task/gcv-004-api-v2`: a merge commit, never a rebase. Then run the full gate. [...] `pkg/cvsession/init.js`. [...] Keep GCV-005's version, and make sure its loop has the same property: a save or load started from another one's continuation finishes before the eval returns. Port `e0823da`'s chained-save test [...] `pkg/cv/graph/workspace.go`. Take `e0823da`'s `writeTemp` [...] Keep `TestWorkspaceTempFiles`. Record the merge in your diary."

**Assistant interpretation:** merge, resolve `init.js` to GCV-005's pending loop, keep `e0823da`'s workspace code and
tests, prove the chained case on GCV-005's loop, gate, commit.

**Inferred user intent:** the PR stacked on #6 carries #6's final state, with no regression of either fix.

**Commit (merge):** 32610c2 — "Merge GCV-004 review fixes (e0823da) into GCV-005"

### What I did

- `git merge origin/task/gcv-004-api-v2`: one conflict, `pkg/cvsession/init.js`; `workspace.go`, `workspace_test.go`
  and `save_test.go` merged cleanly (`writeTemp` with O_CREATE|O_EXCL and a random suffix, no `tmp` counter;
  `TestWorkspaceTempFiles`; the c1.tif → c2.tif → c3.json chain in `TestUnawaitedSaves`).
- `init.js`: kept GCV-005's `saves(seen)`, which re-reads `native.pending()` after every `allSettled` and recurses
  while the list grew, then throws `native.settled()`'s failure.
- `TestUnawaitedSaves` gained a load chain: `cv.load("chain.js").then(() => cv.save("c5.json", …))`, where the file
  itself saves `c4.json`.
- Mutation check: with the loop waiting only once, the test fails (`chained save c2.tif: no such file`); restored.
- Gate: `gate=0`.

### Why

- GCV-005's list already generalizes `e0823da`'s drain (loads and saves, until no new promise joins); taking
  `e0823da`'s `drain` would have dropped loads.

### What worked

- The ported test passed on GCV-005's loop unchanged.

### What didn't work

- Nothing beyond the expected conflict.

### What I learned

- A user continuation attached in the eval body runs before `allSettled`'s, so the re-read of the pending list sees a
  save it started.

### What was tricky to build

- Nothing; the property was already there and is now tested for loads too.

### What warrants a second pair of eyes

- `git diff a42c9d1 e0823da -- pkg/cvsession/init.js` against `git show 32610c2:pkg/cvsession/init.js`.

### What should be done in the future

- Nothing.

### Code review instructions

- `git show 32610c2 --stat`; `go test ./pkg/cvsession -run TestUnawaitedSaves -v`; `go test ./pkg/cv/graph -run TestWorkspaceTempFiles`.

### Technical details

Conflict: `pkg/cvsession/init.js` only (ours kept). rerere recorded the resolution.
