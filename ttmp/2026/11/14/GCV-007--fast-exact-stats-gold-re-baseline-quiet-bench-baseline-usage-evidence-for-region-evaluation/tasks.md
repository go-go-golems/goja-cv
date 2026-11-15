# Tasks

- [x] 1. Fast exact `stats()`: quantiles without copies or reordering, planes in parallel; identical values (GCV-006 baseline: 4.8 s on 24 MP)
- [ ] 2. Fix or re-baseline `TestRealTopHatSaturated` (fails on main since the gold set grew to 164 points)
- [ ] 3. Bench baseline on an idle machine (replaces the loaded GCV-006 Phase 0 numbers)
- [ ] 4. Usage evidence for region evaluation: annotator transcripts (crops/windows of computed images, image sizes, σ of blurs, eval times) and a real full-size scan through the bench
