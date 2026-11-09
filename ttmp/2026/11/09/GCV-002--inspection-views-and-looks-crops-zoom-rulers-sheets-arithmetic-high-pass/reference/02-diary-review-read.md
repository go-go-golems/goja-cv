---
Title: Diary - review read
Ticket: GCV-002
Status: active
Topics:
    - looks
    - views
    - agents
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-002--inspection-views-and-looks-crops-zoom-rulers-sheets-arithmetic-high-pass/reference/01-review-read-findings.md
      Note: the findings this diary step produced
    - Path: ttmp/2026/11/09/GCV-002--inspection-views-and-looks-crops-zoom-rulers-sheets-arithmetic-high-pass/design-doc/01-design-and-implementation-guide.md
      Note: the guide under review (read, not edited)
ExternalSources:
    - /Users/manuel/.codex/skills/textbook-authoring/SKILL.md
    - /Users/manuel/.codex/skills/ticket-research-docmgr-remarkable/references/writing-style.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
Summary: "Diary of the independent review read of the GCV-002 design guide."
LastUpdated: 2026-11-09T18:00:00-05:00
WhatFor: "Recording how the GCV-002 review was done, so the findings can be checked and the review repeated."
WhenToUse: "When checking or acting on the GCV-002 review findings."
---

# Diary - review read

## Goal

This diary records the independent review read of the GCV-002 design and implementation guide: what I read,
which claims I checked against the code and how, and where the findings came from. The findings themselves are
in `01-review-read-findings.md`.

## Step 1: Review read of the GCV-002 design guide

I read the guide cold as an intern would, then went back through it section by section against the code, the
GCV-001 guide it depends on, the GCV-003 guide that follows it, and the colleague's handover issues. The
outcome is a findings document with 6 P1, 16 P2 and 15 P3 findings, an issue-coverage table and a table of the
line references I checked.

The review did not change the guide or any code, and nothing was committed: the coordinator commits, and others
are working in the repository.

### Prompt Context

**User prompt (verbatim):** "Do a review read of the GCV-002 design guide: check concept introduction,
correctness against the code, gaps, guideline conformance, implementability and architecture; write ranked
findings."

**Assistant interpretation:** Read the GCV-002 guide as a newcomer, verify its code claims and proposed APIs
against goja-cv at its current commit, check it against GCV-001/GCV-003 and the colleague's issues 1–5, 8, 14,
15, 22, 23, and write P1/P2/P3 findings with locations and fixes into a new docmgr reference doc.

**Inferred user intent:** Make sure the guide can be handed to an implementer: concepts introduced before use,
claims true, design workable, and the colleague's workflow actually served.

### What I did

- Read the writing guidelines (`textbook-authoring/SKILL.md`, `writing-style.md`) and the diary skill.
- Read the GCV-002 guide top to bottom (962 lines), noting every term at first use and whether it had been
  defined.
- Read the handover issues 1–5, 8, 14, 15, 22, 23 and the generic list's section A in DUST-002.
- Read GCV-001 §3, §4.7, §6, §7.1–§7.11, §9.2, §9.4, §12; grepped GCV-003 for its dependencies on GCV-002 and
  read §6 and §7.6–§7.9.
- Checked code claims with `sed -n`, `cat -n` and `grep -rn` in `pkg/`:
  `look/look.go`, `graph/eval.go`, `graph/registry.go`, `graph/node.go`, `graph/args.go`, `graph/suggest.go`,
  `graph/regions.go`, `graph/engine.go`, `graph/explain.go`, `ops/present.go`, `ops/tone.go`, `ops/filter.go`,
  `ops/value.go`, `ops/source.go`, `ops/mask.go`, `ops/composite.go`, `kernels/draw.go`,
  `kernels/composite.go`, `kernels/measure.go`, `look/font.go`, `look/look_test.go`, `gojamod/cv/prelude.js`,
  `gojamod/cv/binding.go`, `gojamod/cv/evaltrack.go`, `gojamod/cv/help.go`, `cvsession/session.go`,
  `cvsession/reference.go`, `cvsession/processor.go`, `cvsession/init.js`, `internal/testimage/planted.go`.
- Checked the colleague's scripts the guide cites (`80-vlm-render.py` line 76, `84-cv-helpers.js` line 22,
  `85-cv-views.py`'s view definitions and rank metric).
- Recomputed the sheet layout and the DoG example numbers with a one-line `python3 -c`.
- Created both documents with `docmgr doc add --ticket GCV-002 --doc-type reference` from the repo root and
  wrote the findings.

### Why

- The design's core mechanism (render scale, decorations) lives in the evaluator, so every claim about it had to
  be checked against `evalRun.eval`, `sig`, `scaleFor` and `compute`, not just against the guide's pseudocode.
- Several proposed APIs (sheet, compare, levels auto, arithmetic operands) touch build-time facts (doc size,
  identity short-circuit, ArgSpec types) that only show up when reading `node.go`, `registry.go` and the op
  definitions.

### What worked

- Line references were almost all correct, so the current-state section could be trusted and the review could
  focus on design gaps.
- Reading the evaluator alongside §7.2 made the ordering problem (P1-2) and the Value/Regions gap (P2-4)
  visible quickly.
- Reading `levels`' `Identity` function surfaced the `levels({ auto })` no-op (P2-7), which no amount of reading
  the guide alone would have found.

### What didn't work

- `echo =====` as a separator in zsh fails with `(eval):1: ==== not found`, because zsh expands a leading `=`
  as a command path; the first two batched reads printed their first file and then stopped. I re-ran the reads
  separately.
- `grep -rn … --include=*.go` failed under zsh with `no matches found: --include=*.go` (glob expansion); quoting
  the pattern (`--include='*.go'`) fixed it.
- A first grep for `feather` among op names found nothing, because `grow`, `shrink` and `feather` are
  registered through `lengthMaskOp(name, …)`; reading `mask.go:310-335` showed they exist, so I did not report
  `feather` as missing.

### What I learned

- `OpDef.Global` and `OpDef.Halo` are both declared but unread; the guide's `CROP_FIRST` gives `Halo` its first
  reader, but "levels becomes a Global op" has no effect.
- Node sizes are fixed at intern time (`internNode` → `DocSize`), and `look.New` sizes the look at call time.
  Any op whose output size depends on evaluated data (a sheet over Regions) needs a static bound.
- `Engine.Call` with receiver 0 dispatches to `KindNone` ops, so the guide's `__moduleOps` loop over `callOp({
  __id: 0 })` does work with the existing binding.
- `explainDataMisuse` already turns `cv.view(…)` into the fix `img.view(…)`, but with a hint written for awaited
  data.

### What was tricky to build

- Deciding which ordering problems are P1. The decoration rule is correct for the order the guide draws in §3.3;
  it fails only for the reverse order. I ranked it P1 because the guide's own §7.8 sentence and issue 1's
  overlay acceptance case use or need that order, and because the fix (a geometric op class) changes the
  evaluator branch the implementer would write first.
- The sheet size numbers depend on a label height the guide does not give. I took the guide's own font (5 × 7
  at `px = 2`, 14 px) plus 4 px spacing; the conclusion (does not fit) holds for any label of 2 px or more.
- The DoG worked example in P2-2 uses a Gaussian speck model so the arithmetic is exact (peak factor
  σ_s²/(σ_s² + σ_b²), white-noise reduction 1/(2√π σ)). It is an illustration for the guide's §3.6, not a claim
  about the colleague's data.

### What warrants a second pair of eyes

- P1-3's proposed side table for macro provenance: "first writer wins" is deterministic within a session but
  means a hand-written chain built first will caption as primitives even after a macro builds the same node.
  Someone should confirm that is acceptable, or prefer GCV-001's "captions show primitives" rule.
- P2-12's suggestion to drop `compare({ mode: "diff" })` entirely is a product decision for the maintainer; the
  finding stands without it (one OpDef cannot be a macro in one mode).
- P1-2's "crop is geometric and runs at any scale" changes the cache keys of crops above scale 1; check that the
  signature with `interp` still makes `crop` at 8 of an upsampled input equal the upsampled crop.

### What should be done in the future

- Revise the guide for the six P1 findings before handing it to an implementer, and fix GCV-001 §7.8 (captions),
  §7.5 (`DocFrame`) and §9.2 (space before `Expand`) in the same pass so the two guides agree.
- Re-run this review on the revised guide, focusing on §3 (new vocabulary table) and §7.9 (sheet layout).

### Code review instructions

- Start with `01-review-read-findings.md`, sections "P1 findings" and "Consistency with GCV-001 and GCV-003".
- To check P1-2 and P2-4: read `pkg/cv/graph/eval.go:161-307` next to the guide's §7.2 pseudocode.
- To check P1-4: read `pkg/cv/graph/node.go:113-143` (`internNode`) and `pkg/cv/look/look.go:66-99`.
- To check P2-7: read `pkg/cv/ops/tone.go:29-57` and `pkg/cv/graph/eval.go:291-293`.
- To check P2-10: read `pkg/cv/graph/regions.go:194-240` and `pkg/cv/ops/present.go:21-33`.
- Nothing to build or test: the review changed no code.

### Technical details

Sheet layout arithmetic used in P1-5:

```
defaults: size 64, zoom 4 → cell 256 px; gap 6 px; label band ≈ 18 px (5×7 font at px 2, plus spacing)
width:   6 · 256 + 5 · 6          = 1566 ≤ 1568   fits
height:  6 · (256 + 18) + 5 · 6   = 1674 > 1568   does not fit
px = 1:  6 · (256 + 10) + 5 · 6   = 1626 > 1568   still does not fit
two views, size 48, zoom 4, one row per item: row = 192 + 18 + 6 = 216 → floor(1574 / 216) = 7 items
```

DoG example used in P2-2 (speck: Gaussian blob, σ 1 px, height 0.06; grain: white, ±0.01):

```
G_4 at the speck centre:  0.06 · 1/(1 + 16)     ≈ 0.0035
high-pass r = 4:          0.06 − 0.0035         ≈ 0.056,  grain ≈ 0.010   ratio ≈ 5.6
G_1 at the speck centre:  0.06 · 1/(1 + 1)      = 0.030
DoG 1, 4:                 0.030 − 0.0035        ≈ 0.026,  grain ≈ 0.010 · 0.28 ≈ 0.003   ratio ≈ 9
```

Commands that produced the evidence (representative):

```bash
cd /Users/manuel/code/wesen/go-go-golems/goja-cv
cat -n pkg/cv/graph/eval.go | sed -n 1,380p
grep -rn '\.Global\b\|Global:' pkg --include='*.go' | grep -v _test
grep -rn 'Name: *"' pkg/cv/ops/*.go | grep -v _test
sed -n 586,640p pkg/cv/graph/args.go
sed -n 410,500p pkg/cvsession/processor.go
docmgr doc add --ticket GCV-002 --doc-type reference --title "Review read findings"
docmgr doc add --ticket GCV-002 --doc-type reference --title "Diary - review read"
```

## Step 2: Re-check of the P1 findings after revision 2

The writer revised the guide (revision 2, commit `a31654a`), moved the pixel and scale vocabulary into a shared
foundations chapter, and appended a disposition table to my findings. I re-checked each P1 against the new text,
recomputed the sheet layout, traced both decoration orders through the new `renderEval`, and skimmed the P2
dispositions against the sections they cite.

All six P1 findings are resolved. Revision 2 introduces no new P1; I recorded one new P2 (box items break the static
sheet layout) and three P3s in a "Re-check (revision 2)" section of the findings document.

### Prompt Context

**User prompt (verbatim):** "Re-check P1s after revision 2."

**Assistant interpretation:** Verify whether each of my six P1 findings is resolved, partly resolved or not, with
section evidence; verify the sheet arithmetic and the three op classes with render windows; check that P2s marked
fixed are fixed; flag any new P1-level problem; append a short section and a diary step; do not edit or commit.

**Inferred user intent:** Decide whether the revised GCV-002 guide can go to an implementer.

### What I did

- `git log --oneline -5` to find the revision commits (`48e3a50` GCV-001, `a31654a` GCV-002, `f6b03be` GCV-003).
- Read the disposition table at the end of `01-review-read-findings.md`.
- Read revision 2's revision history, "How to read", §1, §3 (all), §7.1–§7.11, §7.13, §8 (class and sheet records),
  §9, §13.4, §13.6, §14.
- Read Foundations §F4–§F5 and GCV-001 §7.8 (macro nodes); grepped GCV-001 for the `box` type and `DocFrame`.
- Read `UnknownMember` in `pkg/gojamod/cv/help.go:279-312` to check the writer's correction to P2-16.
- Recomputed the sheet layout: ⌊1574/262⌋ = 6, 6 · 256 + 30 = 1566; ⌊1574/520⌋ = 3, 3 · 514 + 12 = 1554.
- Appended "Re-check (revision 2)" to `01-review-read-findings.md` with `cat >>`.

### Why

- The P1s were design faults, so each needed a trace through the new pseudocode, not just a check that the
  disposition table cites a section.

### What worked

- Tracing `img.overlay(spots).crop(box).look({ zoom: 8 })` and the §9.3 example through `renderEval` confirmed the
  geometric class fixes both orders.
- The macro node from GCV-001 §7.8 resolves the caption problem cleanly: the walker never enters an expansion and no
  shared node is changed.

### What didn't work

- `wc -l ttmp/2026/11/09/GCV-002*/design-doc/01-*.md` inside a `G=…` assignment failed with `open: No such file or
  directory` (zsh does not expand the glob in an assignment); I used `G=$(ls …)` instead.

### What I learned

- P2-16 was partly based on a wrong premise: `cv` member misses already produce a structured `UnknownMember` error
  that finds Image ops; the writer's note is right.

### What was tricky to build

- Deciding whether the box-item window rule (N1) is P1 or P2. It breaks the static layout only for box items larger
  than `size`, and the fix is one sentence (render at a lower cell scale), so I ranked it P2.

### What warrants a second pair of eyes

- The render memo key includes the window W. Sheets render one window per cell, so 36 extra memo entries per sheet
  per view; they are small, but someone should confirm they are evicted with the normal cache.

### What should be done in the future

- Fix N1 in §7.9 and §9.2 before Phase 6; the P3s can be folded into the same edit.

### Code review instructions

- Read "Re-check (revision 2)" in `01-review-read-findings.md`.
- To check the P1 verdicts: revision 2 §3.4, §7.2, §7.7 (captions), §7.9, §9.1–§9.3; Foundations §F5; GCV-001 §7.8.
- Nothing to build or test.

### Technical details

```
36 cells, one view:  per row ⌊(1568 + 6) / (256 + 6)⌋ = 6;  6 · 256 + 5 · 6 = 1566 ≤ 1568 (both axes, labels inside)
two views:           group = 2 · 256 + 2 = 514;  per row ⌊1574 / 520⌋ = 3 → 3 · 514 + 2 · 6 = 1554;  6 rows → 18 items
N1 example:          box side 200, zoom 4 → 800 px cell in a 256 px layout
```
