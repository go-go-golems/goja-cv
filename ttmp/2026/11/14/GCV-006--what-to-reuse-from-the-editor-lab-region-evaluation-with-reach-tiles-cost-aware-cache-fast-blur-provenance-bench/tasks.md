# Tasks

## Done

- [x] Unpack the Editor lab, read its engine, agent, provenance, book and research notes
- [x] Map goja-cv's evaluator, registry, render path, cache and sources against it
- [x] Measure window costs, Gaussian vs three boxes, and goja per-pixel cost on a 24 MP image
- [x] Import the cited sources into `sources/`
- [x] Write the analysis and design guide; upload it to the reMarkable

## Accepted (14 November 2026, design doc §12.7)

- [ ] Phase 0: bench harness and per-node timing (§12.3)
- [ ] Phase 6: provenance sidecars on saves (§12.6)

## Conditional (decide from the bench and annotator transcripts)

- [ ] Check annotator transcripts: how often do agents crop computed images of full-size scans?
- [ ] Phase 1: `OpDef.Reach` for every op, complete `Global` marking, registry test (§12.1)
- [ ] Phase 2: window evaluation, exact region-invariance test, footprint guard in tests (§12.1, §12.7)
- [ ] Phase 5: fast Gaussian for large σ as an opt-in op, gold-gated (§12.5)

## Deferred

- [ ] Phase 3: tile grid with per-tile keys (§12.2)
- [ ] Phase 4: cost-aware eviction option (§12.4)

## Blockers

- [ ] Fix or re-baseline `TestRealTopHatSaturated` before Phase 2
