---
Title: Usage evidence for region evaluation
Ticket: GCV-007
Status: active
Topics:
    - cv
    - performance
    - research
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: 'How annotator agents used goja-cv (3,042 evals, 171 agents), replayed sessions timed on the current engine, a real 12 MP scan through the bench: the evidence and the recommendation for GCV-006 Phases 1-2 and 5.'
LastUpdated: 2026-11-14T22:48:10.275573-05:00
WhatFor: ""
WhenToUse: ""
---

# Usage evidence for region evaluation

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

# Usage evidence for region evaluation

GCV-006 left region evaluation (Phases 1–2) and the fast Gaussian (Phase 5) conditional on evidence of how agents
use goja-cv. This document collects it. Scripts: `scripts/01-usage-evidence.py` (static counts),
`scripts/replay/` (sessions replayed through `goja-cv bench`), `scripts/02-real-scan.js` (a real scan).

## 1. What agents did (3,042 evals, 171 agents)

Sources: the 39 Sonnet annotators of DUST-002 (`various/agent-evals/evals.jsonl`, 335 evals besides the helper
paste) and the pi annotators of `~/dustdet/autolabel/pi*` (2,707 evals). Counts are of the agents' own code.

| Pattern | Evals using it | Share |
| --- | --- | --- |
| open a gold tile (`mem.tile`, 1,024 × 1,024) | 161 | 5 % |
| open anything else (`cv.open` in agent code) | 0 | 0 % |
| `mem.zoom` (a zoomed window of a view) | 849 | 28 % |
| `cv.sheet` / `mem.sheet` (cells of views around items) | 763 | 25 % |
| `mem.probe` / `probeR` (measures at a point) | 312 | 10 % |
| look with zoom / box or ruler | 140 / 100 | 5 % / 3 % |
| crop of a computed image | 43 | 1 % |
| `gaussian` | 9 | 0.3 % |
| `dog`, `peaks`, `erode`/`dilate` | 3, 3, 4 | ~0 % |

Blur sigmas used: 0.8, 1.0 (2), 1.5 (4), 2.0, 2.5; DoG (1.5, 6) twice and (2, 8) once. **No agent opened a full
scan; every image was a 1,024² tile.** The work is windows of views computed over the whole tile (zoom, sheets,
probes), served from the cache after the first computation.

## 2. What a session costs on the current engine

Six pi annotator sessions (the three longest and three at random, 18–61 evals) replayed in order with the same
prelude, three cold repeats each, on the new build (fast stats), under load (load average 15–27):

| Session (tile) | Evals | Engine time, whole session | Median eval | Slowest eval |
| --- | --- | --- | --- | --- |
| 61ba690de9a8be1e_512_15360 | 61 | 2.9 s | 20 ms | 648 ms |
| 69345e1ba20c5694_2048_3072 | 60 | 3.0 s | 27 ms | 230 ms |
| d1a55e986363697c_2048_1024 | 38 | 1.5 s | 20 ms | 223 ms |
| 1ce57fe44ccc51bc_512_1024 | 25 | 2.1 s | 46 ms | 693 ms |
| 1496ee00c58eed64_1536_1024 | 23 | 1.8 s | 55 ms | 447 ms |
| 702691f5c5f280ca_512_1024 | 18 | 2.0 s | 84 ms | 741 ms |

A whole annotator session spends 1.5–3 s in the engine; each agent turn spends seconds to tens of seconds in the
model. (The Sonnet sessions use the v1 API and no longer run; the replay errors in the pi sessions are the agents'
own, such as sheets larger than a look.)

## 3. A real full-size scan

`raw0002.dng` (Epson V600, 3,528 × 3,516, 74 MB), old build (main before GCV-007) against new, back to back:

| Eval | Old | New |
| --- | --- | --- |
| whole-image `stats()` (includes the decode) | 1,785 ms | 294 ms |
| speck view, first 64 px window | 170 ms | 124 ms |
| 19 more windows of the speck view | 6 ms | 6 ms |
| `gaussian(2)`, 64 px window | 107 ms | 111 ms |
| `gaussian(8)`, 64 px window | 487 ms | 393 ms |

## 4. What the evidence says

- **Region evaluation (GCV-006 Phases 1–2): not now.** Agents work on tiles, where a whole computation costs tens of
  milliseconds. Even on a full scan, the first window of a computation costs 0.1–0.4 s at the sigmas agents use, and
  later windows of the same computation are already 0.3 ms each. Region evaluation would save at most a few hundred
  milliseconds per distinct computation, inside turns that take seconds of model time. The case changes only if
  agents are given full scans *and* compute many different large-reach images per scan.
- **Fast Gaussian (Phase 5): not needed.** Agents used σ ≤ 2.5, where the exact Gaussian is already fast and the
  three-box approximation is worst (31 % of the signal at σ 1, GCV-006 §11.2).
- **Already done:** the largest cost an agent paid on a full image was `stats()` (and the stats inside the speck view
  and `levels({ auto })`), now 6–26 times faster with identical values (step 1).
- **What would reopen the question:** a plan to run agents on full scans (DUST detection over whole frames, or a
  scan-level review tool), or a transcript that shows agents waiting on engine time.
