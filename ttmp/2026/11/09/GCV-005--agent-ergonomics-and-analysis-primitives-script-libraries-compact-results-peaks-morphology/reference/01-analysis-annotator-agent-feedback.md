---
Title: Analysis - annotator agent feedback
Ticket: GCV-005
Status: active
Topics:
    - api
    - agents
    - analysis
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Coordinator analysis of the 39-agent goja-cv feedback (DUST-002 reference/11): what the evidence shows, its caveats, and an issue-by-issue disposition for GCV-005."
LastUpdated: 2026-11-09T14:23:00.363191-05:00
WhatFor: ""
WhenToUse: ""
---

# Analysis - annotator agent feedback

## Goal

The starting evidence for GCV-005. It assesses the feedback that 39 Claude Sonnet annotator agents produced while
using goja-cv v1 (`b847520`) over MCP, and records the disposition the guide should design against.

## Context

- **Source:** `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/11-goja-cv-agent-feedback.md` — ten issues with counts and code.
- **Raw evidence:** `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-evals/` (`summary.md`, `evals.jsonl`, one `.js` file per agent, extracted
  by `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/91-extract-agent-evals.py`) and `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-diaries/` (each agent's "goja-cv friction"
  section). The agents' helper library is `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/84-cv-helpers.js`; their playbook is `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/04-goja-cv-dust-playbook.md`.
- **Background:** vault article `~/code/wesen/go-go-golems/go-go-parc/Projects/2026/11/09/ARTICLE - Deep Dive - goja-cv in Agent Hands - Feedback from 39 Annotator Agents.md`.
- **Numbers:** 374 evals (345 besides the helper paste), 0 errors, 13 UNUSED_NODE, 7 RANGE, 1 cut result; 29
  helper pastes. Used: look 288, crop 173, sheet 171, view 162, max 67, levels 52, min 41, measure 19. Never used:
  cv.docs, cv.help, profile, sample, annotate, compare, highpass/dog, localContrast, chroma/channel/toLab, csv/json,
  save, params.
- GCV-004 (API v2) is the base: literal options, space on the image, masks are one-channel images, awaited regions
  are plain arrays with `[node, index]` links, `look()` only, `cv.save`, one workspace (`--root` + `--read`).

## Quick Reference

### What the evidence shows

1. **A task helper library replaced the docs.** No agent opened `cv.docs` or `cv.help`; they worked through the
   helpers and a handful of ops, then plain JS over returned rows. For agents, the interface is playbook + helpers;
   goja-cv is the substrate. Task libraries must be first-class; the instructions/context block matters more than
   docs pages.
2. **Models cannot read coordinates off pictures.** Positions read off 2× ruler crops were 5–10 px off; on one tile
   28 of 32 guessed positions probed at sp ≈ 0. Design rule for GCV-005: **never make a model read a number off an
   image; every look comes with the data it shows** (coordinates, ids that map back to rows).
3. **Plain JS over arrays is how agents filter** (sp 23 agents, region 7, area/aspect 3, probe loops 25): confirms
   GCV-004's regions-as-arrays.
4. **The speck scale is tile-global** and scene edges set it; a noise-scaled view was tried and was worse (87 % vs
   91 % gold coverage on 197 points). Local normalisation belongs where candidates are found, not in the view's
   default.

### Caveats

- "0 errors" mostly reflects the helpers absorbing complexity, not the raw API (the pi/DeepSeek runs without
  helpers had errors). Keep both results.
- "Never used" means "not shown by the playbook", not "unwanted": `profile` (0 uses) is the answer to issue 10.
  Nothing should be removed on this evidence.
- Population: Sonnet with a playbook vs DeepSeek without; "docs don't matter" holds only when a task library exists.

### Disposition (coordinator recommendation; the guide designs the how)

| # | Ask | Disposition |
|---|---|---|
| 1 | `eval_file` MCP tool, `--prelude` docs, mem listing | **Yes, as one in-language primitive**: `cv.load(path, args)` resolved through the GCV-004 workspace/`--read` roots (works in MCP, CLI, xgoja alike), plus a script listing and `mem.*` one-line docs in the context block; document `--prelude`. An `eval_file` MCP tool only as a thin wrapper if it saves real tokens. No separate `--scripts` flag. |
| 2 | sheet badge, `{ cx, cy, id }` items, `label` field | Badge fixed by the v1 patch (PR #5). Document items; accept a `label` field (flat schema). |
| 3 | `img.peaks()` | **Yes, the most valuable new op.** Returns items in the flat schema (`cx, cy`, not `x, y`, which are a box corner) with `value` and `snr` against local noise, so they feed `sheet`, `measure`, `crop` directly. No `gridLabels` look option: data beats a better ruler. |
| 4 | `view("speck", { white })` | Keep the default (evidence). Low priority; `dog().levels()` already gives control; local normalisation in `peaks` is the structural fix; doc note on the global scale. |
| 5 | saturated blobs become rings | **Image morphology**: white top-hat keeps flat tops filled. |
| 6 | `components({ maxAspect })` | **No option**: `items.filter(r => r.aspect < 3)` is the one way; line suppression is morphology (opening with a line element). |
| 5+6+old 18 | morphology | **One family for one-channel images** (erode, dilate, open, close, top-hat; disk and line elements). With masks as images (GCV-004), mask grow/shrink merge into it. |
| 7 | `crop(box, { fit: "shift" })` | Out of the core unless nearly free; helpers clamp, sheets handle edges. |
| 8 | UNUSED_NODE for helper bundles | **Correctness fix**: nodes reachable from the returned value or `mem` were not discarded; no `cv.quiet`. |
| 9 | 9000-char cap on row lists | **No new API**: render uniform arrays of flat records as a compact table (header + rows) automatically. |
| 10 | thread width | A `profile` recipe — in the task library/playbook too, since agents don't read docs. No `measure.line` op. |

### How to measure success

- Agent loop: rerun one annotator with playbook `reference/04` (helpers loaded via `cv.load`/`--prelude`), compare
  with script 91: pastes, code tokens, truncated results, UNUSED_NODE, evals per tile.
- Detection quality on gold (`~/dustds/gold`): position error of candidates vs gold (the 5–10 px problem), coverage
  of saturated blobs, dust in dark areas, scene-line false positives — `peaks` and top-hat must move these.

## Related

- GCV-004 guide (`ttmp/2026/11/09/GCV-004--…/design-doc/01-design-and-implementation-guide.md`): the base API.
- v1 patch PR #5 (sheet badge, stepChroma gate, LOOKS warning).
