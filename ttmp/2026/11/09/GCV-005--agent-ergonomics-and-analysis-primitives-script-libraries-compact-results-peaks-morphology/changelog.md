# Changelog

## 2026-11-09

- Initial workspace created
- Design guide revision 1 (design-doc/01): draft `67fa07b` (sections 1-9), finished in the next commit (pseudocode, errors, docs, phases A1-A4/B1-B3/V, tests, validation, migration note, risks, Appendix A); design writer diary steps 1-2 (reference/03); tasks.md filled with the phases, review read and reMarkable upload

## 2026-11-10

- Merged origin/task/gcv-004-api-v2 (GCV-004 batch 1, Phases 0a-4) into the branch (`6bb71a8`)
- Design guide revision 2 from the review read (reference/04, 33 findings) and the coordinator's decisions: evidence recounted; CROP_FIRST deleted; disk = distance <= r; erode/dilate only (disk <= 64, van Herk lines); UNUSED_NODE as a drop lint (jsparse); settled evals and one UNHANDLED warning; JS load stack; docs by first-token comment and source text, not consumed by preludes; tables exempt from the item and string caps; peaks in row bands with a pre-filter and time budget; labels and point lists moved to GCV-004 Phase 5; Appendix A updated, Appendix B (31 fixed, 2 partly, 0 declined). Numpy peaks prototype on four gold tiles: scripts/02-peaks-prototype.py
- Design guide revision 3 (coordinator decisions on revision 2's open questions): UNUSED_NODE deleted with no replacement lint (no jsparse rewrite, no __cv_drop, no UNAWAITED); OpDef.Halo deleted with CROP_FIRST; sheet captions accepted as proposed; cv.save promises on the pending list via GCV-004 Phase 6; mask.regions gets no grow option, so B1 needs only GCV-004 Phase 4; no open questions
