# Tasks

## TODO

- [x] Design guide written (design-doc/01): evidence, concepts, API, architecture, decisions D1-D21, deletions, pseudocode, phases, tests, validation <!-- t:g5d1 -->
- [ ] Review read of the guide; revision 2 with the findings dispositioned <!-- t:g5rv -->
- [ ] Upload the guide to reMarkable <!-- t:g5rm -->
- [ ] Phase A2 - readable results: compact tables for arrays of flat records (shared budget, paging note), functions rendered as signature + doc line, context block functions (signatures; doc lines when new or replaced), UNUSED_NODE only for the eval's own code, uncapped mem walk; cut notes name cv.save (independent of GCV-004's remaining phases) <!-- t:pa2 -->
- [ ] Phase A1 - task libraries: cv.load(path, args) (workspace then --read, async function body with args, no cache, 32 active loads, SyntaxError with file:line), cv.scripts(), --prelude as a load inside a root, file:line locations and "called from" in errors and warnings, LOAD warning; help entries; topic 14-task-libraries (needs GCV-004 Phases 2 and 6) <!-- t:pa1 -->
- [ ] Phase A3 - labels and points: items op entries carry id and label, Tags["label"], sheet items|point[] and labels id|none, overlay labels; regenerate sheet-2x2 (needs GCV-004 Phase 5) <!-- t:pa3 -->
- [ ] Phase A4 - Part A documentation sweep: MCP Instructions (guide 12.1, under 4000 characters), EvalToolDescription, pkg/doc setup pages with --read/--prelude, agents page, recipes playbook pattern, troubleshooting, README <!-- t:pa4 -->
- [ ] Phase B1 - morphology: DiskMinMax (chords + van Herk, binary fast path via DistanceToSet), LineMinMax, ops erode/dilate/opening/closing (one channel, { line: angle }); delete grow, shrink, kernels.Grow/Shrink, mask.regions grow; help concept tophat (needs GCV-004 Phase 4) <!-- t:pb1 -->
- [ ] Phase B2 - peaks: diskSums step map, NMS with DiskMinMax, parabolic sub-pixel fit, ProbePoint measures, |snr| filter, non-overlap merge across radii; op peaks (Luma, AnalysisScale 1, Disk regions, geometry + r + core/ring fields) (needs GCV-004 Phases 3 and 5, and B1) <!-- t:pb2 -->
- [ ] Phase B3 - gold-gated relative tests (position, coverage, dark areas, saturated blobs, line openings), scripts/01-gold-metrics.sh, Part B docs (views note, recipes, measuring, instructions lines) <!-- t:pb3 -->
- [ ] Phase V - validation: annotator agent rerun with the rewritten playbook (script 91 metrics), pi runs with questions-gcv-005.md, gold metrics incl. re-derived snr thresholds; deviations section in the guide <!-- t:pv -->
