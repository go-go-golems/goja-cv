# Changelog

## 2026-11-09

- Initial workspace created
- Design guide revision 1 (design-doc/01): draft `67fa07b` (sections 1-9), finished in the next commit (pseudocode, errors, docs, phases A1-A4/B1-B3/V, tests, validation, migration note, risks, Appendix A); design writer diary steps 1-2 (reference/03); tasks.md filled with the phases, review read and reMarkable upload

## 2026-11-10

- Merged origin/task/gcv-004-api-v2 (GCV-004 batch 1, Phases 0a-4) into the branch (`6bb71a8`)
- Design guide revision 2 from the review read (reference/04, 33 findings) and the coordinator's decisions: evidence recounted; CROP_FIRST deleted; disk = distance <= r; erode/dilate only (disk <= 64, van Herk lines); UNUSED_NODE as a drop lint (jsparse); settled evals and one UNHANDLED warning; JS load stack; docs by first-token comment and source text, not consumed by preludes; tables exempt from the item and string caps; peaks in row bands with a pre-filter and time budget; labels and point lists moved to GCV-004 Phase 5; Appendix A updated, Appendix B (31 fixed, 2 partly, 0 declined). Numpy peaks prototype on four gold tiles: scripts/02-peaks-prototype.py
- Design guide revision 3 (coordinator decisions on revision 2's open questions): UNUSED_NODE deleted with no replacement lint (no jsparse rewrite, no __cv_drop, no UNAWAITED); OpDef.Halo deleted with CROP_FIRST; sheet captions accepted as proposed; cv.save promises on the pending list via GCV-004 Phase 6; mask.regions gets no grow option, so B1 needs only GCV-004 Phase 4; no open questions

## 2026-11-09

Phase B1 implemented: erode/dilate (disk <= 64 via chords grouped by half-width + exact squared-distance fast path; van Herk lines), opening/closing/tophat help concepts; deleted grow, shrink, toMask grow, kernels.Grow/Shrink, CROP_FIRST, OpDef.Halo; new goldens dilate-mask, tophat-gray (commits bcefb0a, 5c26122); diary reference/06; deviations 1-10 in the guide


## 2026-11-09

Merged GCV-004 final (a42c9d1, batch 2 Phases 5-8) into the branch: 10 conflicts resolved toward batch 2 with B1's deletions kept (commit 259df82); diary step 2


## 2026-11-09

Phase A1: cv.load (JS load stack, file:line locations, no cache), cv.scripts (bounded walk), --prelude as a load inside a root, pending loads and saves on one list, UNHANDLED located by stack (commit c0ad935); deviations 11-20; diary step 3


## 2026-11-09

Phase A2: compact tables exempt from the item/string caps (shared 2/3 budget, paging note), DescribeFunction, context block functions and new-function docs (not consumed by preludes) (commit b10ff2d); UNUSED_NODE deletion moved to A4; deviations 21-25; diary step 4


## 2026-11-09

Phase A3: sheet captions list cells (label else id) for arrays, 'cells in region order' for Regions nodes; labels id|none; golden sheet-2x2 regenerated (commit c9987c0); deviations 26-28; diary step 5


## 2026-11-09

Phase A4: UNUSED_NODE deleted with its node tracking (commit eaafc1d); Part A docs sweep: instructions (12.1 without peaks), eval tool description, guidebook agents/setup/recipes/troubleshooting, README, rows instead of joined strings (commit 7cec0bf); deviations 29-32; diary step 6

