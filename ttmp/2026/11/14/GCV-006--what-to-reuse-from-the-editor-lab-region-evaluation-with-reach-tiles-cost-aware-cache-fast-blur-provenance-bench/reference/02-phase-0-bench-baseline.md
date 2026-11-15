---
Title: Phase 0 bench baseline
Ticket: GCV-006
Status: active
Topics:
    - cv
    - architecture
    - performance
    - research
    - agents
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: 'Baseline of goja-cv bench (Phase 0) on the 24 MP bench scan before any optimisation: blurs, speck-view windows, peaks, and the finding that whole-image stats() costs 4.8 s.'
LastUpdated: 2026-11-14T19:57:46.704567-05:00
WhatFor: 'The before numbers every later GCV-006 phase is compared against.'
WhenToUse: 'Before and after a change that claims to make goja-cv faster or leaner.'
---

# Phase 0 bench baseline

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

# Phase 0 bench baseline

Measured on 14 November 2026 with `goja-cv bench` at commit `79a3a3a` (branch `task/gcv-006-phase0-bench`), on the
10-core Apple machine of §11, **under load** (load average 8 to 13: eighteen agent builds were running). Treat
absolute times as samples and compare ratios; rerun the baseline on a quiet machine before accepting a change.

```bash
go run ./internal/testimage/gen -out DIR -bench-scan 6000x4000      # bench-scan-6000x4000.npy, 288 MB, 1.5 s
goja-cv bench --root DIR --repeat 3 examples/bench/big-blur.js examples/bench/scan-zoom.js examples/bench/scan-probe.js
```

Every repeat starts in a fresh session (cold cache, source not decoded).

## big-blur

| Eval | p50 ms | p95 ms | op runs | cached MB after |
| --- | --- | --- | --- | --- |
| open and stats (decode, pyramid, whole-image stats) | 5,332 | 5,356 | 3 | 366 |
| crop of the source, 64 px | 6 | 6 | 3 | 366 |
| gaussian 2, 64 px window | 222 | 238 | 4 | 641 |
| gaussian 8, 64 px window | 815 | 951 | 4 | 916 |
| gaussian 32, 64 px window | 4,029 | 4,463 | 4 | 1,190 |
| gaussian 32 again, another window (cached whole value) | 6 | 6 | 3 | 1,191 |

Peak resident memory after the scenario: 3,248 MB.

## scan-zoom

| Eval | p50 ms | p95 ms | op runs | cache hits | cached MB after |
| --- | --- | --- | --- | --- | --- |
| open (frame only, nothing decoded) | 0.3 | 0.3 | 0 | 0 | 0 |
| speck view, first 64 px window | 695 | 709 | 9 | 0 | 824 |
| 19 more windows of the speck view | 6 | 11 | 38 | 20 | 824 |

## scan-probe

| Eval | p50 ms | p95 ms | op runs | cached MB after |
| --- | --- | --- | --- | --- |
| open | 0.4 | 0.7 | 0 | 0 |
| peaks, radius 2, 4, 8 | 2,893 | 2,974 | 3 | 366 |
| measure the strongest 50 (`prefix: "probe"`) | 7 | 11 | 2 | 366 |

## Decode against stats (an extra scenario, not shipped)

| Eval | p50 ms | op runs |
| --- | --- | --- |
| open | 0.4 | 0 |
| crop stats (decodes the source) | 300 | 4 |
| whole-image stats | 4,840 | 2 |

## What the baseline says

- **Windows of blurs cost the whole blur**, as §11.1 found: 222 ms, 815 ms and 4,029 ms for σ 2, 8 and 32 against 6 ms
  for a window of the source. A second window of a cached whole blur costs 6 ms.
- **The speck view's later windows are already cheap:** 19 windows in 6 ms once the first (695 ms) has computed the
  view. An agent that zooms around one view of a scan pays once. Region evaluation would cut the first window, not
  the later ones.
- **Whole-image `stats()` is the largest single cost measured: 4.8 s**, against 0.3 s to decode the source. It is
  not on the GCV-006 list; it is a new candidate. `kernels.Stats` copies each of four planes (luminance and three
  channels) and runs at least three quickselects (p01, median, p99, plus requested percentiles) over 24 million
  values per plane, on one core. Running the planes in parallel, or an exact two-pass histogram (bin counts, then a
  selection inside the bin that holds the rank), would keep the values identical. Its cost is paid by every script
  that starts with `await img.stats()`, which the guide's own quick start does.
- **Memory:** 3.2–3.5 GB peak: the source and its pyramid plus one 288 MB float image per cached whole-frame result.
