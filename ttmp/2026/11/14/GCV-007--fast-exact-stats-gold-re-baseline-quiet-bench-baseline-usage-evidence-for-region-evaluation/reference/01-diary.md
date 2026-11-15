---
Title: Diary
Ticket: GCV-007
Status: active
Topics:
    - cv
    - performance
    - research
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/cv/kernels/measure.go
      Note: Stats uses Quantiles, planes in parallel
    - Path: repo://pkg/cv/kernels/quantile.go
      Note: exact quantiles (radix histogram)
    - Path: repo://pkg/cv/ops/tone.go
      Note: levels auto via Quantiles
    - Path: repo://pkg/cv/ops/views.go
      Note: speck view stretch via Quantiles
ExternalSources: []
Summary: ""
LastUpdated: 2026-11-14T22:34:05.441639-05:00
WhatFor: ""
WhenToUse: ""
---





# Diary

## Goal

<!-- What is the purpose of this reference document? -->

## Context

<!-- Provide background context needed to use this reference -->

## Quick Reference

<!-- Provide copy/paste-ready content, API contracts, or quick-look tables -->

## Usage Examples

<!-- Show how to use this reference in practice -->

## Related

<!-- Link to related documents or resources -->

# Diary — GCV-007

## Goal

Make the cheap improvements GCV-006's bench pointed at, and collect the evidence that decides whether region
evaluation (GCV-006 Phases 1–2) is worth building: fast exact `stats()`, the failing gold test, an idle-machine
baseline, and how agents really use goja-cv on large images.

## Step 1: Exact quantiles without copying, stats 26× faster

Whole-image `stats()` cost 4.8 s on the 24 MP bench scan, 16 times the decode. `kernels.Stats` copied each of four
planes (luminance and three channels) and ran at least three quickselects over 24 million values per plane on one
core. The new `kernels.Quantiles` finds the same order statistics without copying or reordering, with two parallel
passes of a radix histogram over an order-preserving 32-bit key; `Stats` runs its planes in parallel. `stats()` now
takes 186 ms and returns byte-identical results.

### Prompt Context

**User prompt (verbatim):** "ok, let's do 1-2-3, then 4"

**Assistant interpretation:** Do the stats() speed-up, fix or re-baseline TestRealTopHatSaturated, rerun the bench baseline on a quiet machine, then gather the usage evidence for region evaluation (transcripts and a real scan); one new ticket, diary, commits per step.

**Inferred user intent:** Take the measured cheap wins and settle the region-evaluation question with evidence before building anything large.

**Commit (code):** 04effef — "Exact quantiles without copies: two-pass parallel radix histogram (Quantiles); stats planes in parallel; …"

### What I did
- `pkg/cv/kernels/quantile.go`: `orderKey`/`fromOrderKey` (float32 ↔ uint32 preserving order); `Quantiles(ctx, a, ps)`:
  ranks lo and lo+1 per p, pass 1 counts the high 16 bits per chunk (`parallel.Each`, 2 × GOMAXPROCS chunks), the
  cumulative counts give each rank's bin and rank within it, pass 2 counts the low 16 bits inside only those bins,
  which gives the exact key; interpolation as `Percentile` (`float64(vlo) + f*float64(vhi-vlo)`, the difference in
  float32). Inputs under 65,536 values copy and use `Percentile`.
- `pkg/cv/kernels/measure.go`: `channelStats` uses `Quantiles` (no scratch plane); `Stats` measures the planes in
  parallel (each plane's sums stay sequential, so means are unchanged).
- `pkg/cv/ops/tone.go` (`levels({ auto })`) and `pkg/cv/ops/views.go` (the speck view's stretch): `Quantiles` on the
  luminance plane instead of copying 24 M values and quickselecting.
- `pkg/cv/kernels/quantile_test.go`: equality with `Percentile` for 10 quantiles on 8 generators (uniform, normal,
  clipped runs, constant, all in one high bin, signed, ±0, four values) at sizes 1, 2, 7, 65,535, 65,536 and 200,003,
  and that the input is not modified; `BenchmarkQuantiles24MP`.

### Why
- The GCV-006 baseline found it: every script that starts with `await img.stats()` paid 4.8 s on a full scan, more
  than any blur window.

### What worked
- Kernel benchmark, 24 M values, p01/median/p99: 15 ms (Quantiles) against 1,139 ms (copy + Percentile).
- End to end, old binary (main) against new, back to back on the same load (load average ~20): whole-image stats
  4,831 → 258 ms with Quantiles, 186 ms with parallel planes; speck view first window 636 → 501 ms; decode unchanged.
- Identity: printed stats of the bench scan equal to the last digit; the complete JSON of `stats()`, linear `stats()`,
  speck view `stats()` and `levels({ auto })` `stats()` has the same MD5 for both binaries on five DUST tiles.
- Gold-gated tests unchanged: `TestRealTopHatSaturated` reports the same 124/127/148 of 164 as before; all other
  `TestReal*` pass.

### What didn't work
- `make lint`: `quantile_test.go:19:1: File is not properly formatted (gofmt)` (one-line closures in a map); `gofmt -w`.
- My first tile comparison looped once over three paths: zsh does not split an unquoted `$T`; rerun with `$(ls …)`
  in the loop header.

### What I learned
- The remaining 186 ms is luminance and the sequential mean/clip loops. Parallelising inside a plane would change
  float64 summation order, so the means would differ in the last bits; parallel planes keep them identical.

### What was tricky to build
- Exactness: the radix key must order floats like `<` does (negative keys reversed), and both order statistics of an
  interpolation (lo, lo+1) can fall in different bins, so the low-bit pass collects every needed bin at once. ±0 have
  different keys but are equal numbers; the result is the same value.
- Clipped images put millions of equal values in one bin; the low-bit pass still resolves the exact key because all
  32 bits are covered by the two 16-bit levels.

### What warrants a second pair of eyes
- NaN: `Quantiles` assumes no NaN (as `selectK` effectively did; its comparisons were undefined with NaN). Images are
  clamped on decode, but a kernel that produced NaN would now give a different wrong answer than before.
- Memory: per chunk a 256 KB high histogram and one 256 KB low histogram per needed bin (20 chunks × up to 2 × len(ps)
  bins); a few tens of MB at most, against the 96 MB scratch plane per `stats()` it replaces.

### What should be done in the future
- N/A

### Code review instructions
- `pkg/cv/kernels/quantile.go`, then `measure.go` (`Stats`, `channelStats`).
- `GOWORK=off go test ./pkg/cv/kernels/ -run Quantiles -v`; `GOWORK=off go test ./pkg/cv/kernels/ -run X -bench Quantiles24MP -benchtime 3x`;
  `GOWORK=off go test ./pkg/cv/ops/ -run Real -v` (gold tests, local data).

### Technical details
- Bench scenario for the end-to-end numbers: open; `crop([3000, 2000, 64, 64]).stats()` (decodes); whole `stats()`.

## Step 2: TestRealTopHatSaturated re-baselined

The test encoded a GCV-005 hypothesis: on saturated or large dust, top-hat components cover at least as many gold
points as the hot list. Since the gold set grew to 164 such points, the top-hat covers 124 and the hot list 127, so
the test had failed on main ever since. The hypothesis was refuted by 3 points, not broken by code; I re-baselined
the test to what the data supports with a margin and put the measured coverage in the spots topic for agents.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Fix or re-baseline the failing gold test so main is green again.

**Inferred user intent:** A suite whose only failure is not "the known one", so a real regression shows.

**Commit (code):** d562ddc — "Re-baseline TestRealTopHatSaturated on the grown gold set: …"

### What I did
- Read the test and the GCV-005 guide: the claim "top-hat ≥ hot list" was an acceptance criterion
  (GCV-005 design doc §tests), not something the agent guides promise; they only say the top-hat keeps a saturated
  speck as a filled blob where the speck view makes a ring.
- `pkg/cv/ops/peaks_gold_test.go`: new doc comment with the numbers; asserts `peaks [2, 4, 8, 16] ≥ max(top-hat,
  hot list)` and `top-hat ≥ 0.9 × hot list` (a regression floor).
- `pkg/cv/docs/topics/15-spots.md`: the measured coverage (124 / 127 / 148 of 164), the advice to include a large
  radius in `peaks` for saturated dust and to use the top-hat for its shape, and that precision was not measured.

### Why
- A test that has failed for days hides the next real regression. Changing code to make the top-hat win would chase
  a number; the honest move is to record what the data says and keep a floor.

### What worked
- `go test ./...` passes in full, the first time since the gold set grew; numbers unchanged by Step 1 (124/127/148).

### What didn't work
- N/A

### What I learned
- On this gold set, `peaks` with a radius of 16 is the best of the three at finding saturated dust (90 %), which the
  guides did not say.

### What was tricky to build
- Choosing what to assert: the ordering that holds by a clear margin (peaks over both) plus a floor for the method
  the guides describe, rather than a ranking within noise (124 vs 127).

### What warrants a second pair of eyes
- Coverage without precision: `peaks` with radius 16 may also add more false candidates; the guide says so, but the
  advice leans on coverage only.

### What should be done in the future
- Measure precision of the three methods on the gold tiles (false candidates per tile) before recommending one.

### Code review instructions
- `GOWORK=off go test ./pkg/cv/ops/ -run TestRealTopHatSaturated -v` (needs the local gold data).

### Technical details
- Gold subset: dust with fit radius ≥ 6 px or core max ≥ 98 % of the tile's maximum.

## Step 3: Idle baseline — not possible yet

The idle-machine baseline needs an idle machine. After the pattern rebuilds finished, the load stayed at 15–27:
another session's Playwright Chromium processes (each near 100 % CPU) and a Node test run, none started here. I did
not stop them. Every comparison in this ticket was taken old build against new, back to back, so its ratios hold
under load; the absolute baseline in GCV-006 `reference/02` stays marked "under load" until a quiet run.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Rerun the bench baseline when the machine is quiet.

**Inferred user intent:** Absolute numbers that are not inflated by other work.

### What I did
- `uptime` and `ps -r` at 22:38–22:47: load 15–27; top processes `Google Chrome for Testing` (Playwright) and `node --test`.

### Why
- Other sessions' work is not mine to stop.

### What worked
- N/A

### What didn't work
- No quiet window yet.

### What I learned
- Back-to-back old/new runs are the robust comparison on a shared machine; absolute baselines need a scheduled quiet slot.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- Run `goja-cv bench --root DIR --repeat 5 examples/bench/*.js` when the load average is below 2, and replace the
  GCV-006 baseline tables.

### Code review instructions
- N/A

### Technical details
- N/A

## Step 4: Usage evidence for region evaluation

The evidence GCV-006 waited for, collected from what is already on disk: 3,042 annotator evals from 171 agents
(static counts of their own code), six pi annotator sessions replayed through the bench on the current engine, and a
real 12.4 MP Epson V600 scan through the zoom and blur patterns. Agents never opened a full scan; their work is
windows of views over 1,024² tiles, already served from the cache; a whole session spends 1.5–3 s in the engine.
Region evaluation and the fast Gaussian are not worth building now. The write-up is
`reference/02-usage-evidence-for-region-evaluation.md`.

### Prompt Context

**User prompt (verbatim):** (see Step 1); earlier: "what's the evidence we are waiting for? when will we get it?"

**Assistant interpretation:** Mine the annotator transcripts and bench a real scan, and say what they mean for GCV-006 Phases 1–2 and 5.

**Inferred user intent:** Decide the large proposals on evidence instead of a synthetic benchmark.

### What I did
- `scripts/01-usage-evidence.py`: counts patterns in the Sonnet evals (DUST-002 `various/agent-evals/evals.jsonl`) and
  the pi annotators' `tool_execution_start` code (`~/dustdet/autolabel/pi*/*/events.jsonl`), helper pastes excluded;
  output in `scripts/01-usage-evidence-output.md`.
- `scripts/replay/`: six pi sessions as bench scenarios (one eval per `// ---`), run with the DUST helpers as
  prelude, `--read` the tiles and scripts, three repeats: `replay-bench.json`.
- `scripts/02-real-scan.js`: `raw0002.dng` through open, stats, the speck view and 19 windows, σ 2 and σ 8 windows;
  old and new build: `02-real-scan-bench-{old,new}.yaml`.

### Why
- GCV-006 §11.1 measured a synthetic 24 MP scan; whether that workload exists decides Phases 1–2.

### What worked
- The pi sessions replay on the current API with the same prelude; their errors are the agents' own (oversized
  sheets, `cv.ops`, cancelled evals).
- The real scan reproduces the synthetic picture at half the size: first window of a computation 0.1–0.4 s, later
  windows 0.3 ms each, stats 1,785 → 294 ms with step 1.

### What didn't work
- The Sonnet sessions no longer run: they are v1 API (`cv.open has no option "as"`); I used the pi sessions instead.
- First real-scan run: windows placed for a 6,000 px image (`crop.box = [3711, 2229, 64, 64] is outside the image's
  frame [0, 0, 3528, 3516]`); positions taken modulo 3,400.
- pi's tool events carry no timestamps, so original eval durations cannot be read from the logs; the replay measures
  the engine instead (model time is not included, which is the point).

### What I learned
- Agents' heavy patterns (zoom, sheets, probes) are windows of one cached whole view, the case goja-cv already makes
  free. The costs region evaluation removes are first computations, which agents make a handful of per tile.

### What was tricky to build
- Counting only agent code: the pasted helper library contains every pattern; evals containing it are skipped.

### What warrants a second pair of eyes
- The regexes count calls in code, not executions (a loop calling `mem.zoom` 20 times counts once per occurrence).
- All agent evidence is from tile workloads; it cannot say what agents would do with full scans.

### What should be done in the future
- Revisit Phases 1–2 if agents get full scans.

### Code review instructions
- `python3 -I scripts/01-usage-evidence.py <evals.jsonl> ~/dustdet/autolabel/pi ~/dustdet/autolabel/pi_annot ~/dustdet/autolabel/pi_annot_run`
- `goja-cv bench --root WS --read ~/dustdet/cv_tiles --read <DUST-002 scripts> --prelude <84-cv-helpers.js> scripts/replay/*.js`

### Technical details
- See `reference/02-usage-evidence-for-region-evaluation.md` for the tables.
