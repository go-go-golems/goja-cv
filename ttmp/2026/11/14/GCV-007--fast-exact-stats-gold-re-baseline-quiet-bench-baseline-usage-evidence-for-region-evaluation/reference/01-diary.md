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
