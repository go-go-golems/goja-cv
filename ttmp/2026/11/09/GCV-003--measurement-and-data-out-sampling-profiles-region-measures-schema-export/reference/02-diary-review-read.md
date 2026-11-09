---
Title: Diary - review read
Ticket: GCV-003
Status: active
Topics:
    - measurement
    - regions
    - export
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-003--measurement-and-data-out-sampling-profiles-region-measures-schema-export/reference/01-review-read-findings.md
      Note: the findings this diary step produced
    - Path: ttmp/2026/11/09/GCV-003--measurement-and-data-out-sampling-profiles-region-measures-schema-export/design-doc/01-design-and-implementation-guide.md
      Note: the guide reviewed (not edited)
ExternalSources: []
Summary: "Diary of the independent review read of the GCV-003 design guide."
LastUpdated: 2026-11-09T17:30:00-05:00
WhatFor: "Recording how the GCV-003 review read was done and what it found."
WhenToUse: "Read before revising the GCV-003 guide or re-reviewing it."
---

# Diary

## Goal

This diary records the independent review read of the GCV-003 design and implementation guide: how it was done,
what was checked against which code, and what remains open. The findings themselves are in
`reference/01-review-read-findings.md`.

## Step 1: Review read of the GCV-003 design guide

I read the guide cold, as an intern who knows Go and some JavaScript and has never seen goja-cv, noting every
term used before it was defined. I then checked each `file:line` claim and code excerpt against the goja-cv tree
(HEAD `72911b1`; the guide is unchanged since `b30efec`), go-go-goja's `resultcontent` and `replmcp` at the
version in `go.mod`, the GCV-001 and GCV-002 guides where GCV-003 depends on them, and the colleague's issue
texts, playbook and helper scripts. The result is 5 P1, 13 P2 and 17 P3 findings, each with a location and a fix.

The review did not edit the guide and made no commits; the coordinator commits.

### Prompt Context

**User prompt (verbatim):** "Do a review read of the GCV-003 design guide: check concept introduction, correctness against the code, gaps, guideline conformance, implementability and architecture; write ranked findings."

**Assistant interpretation:** Read the GCV-003 guide as a newcomer, verify it against the current code and its
dependencies, check it solves issues 9, 10, 16, 17, 21 and 23b for the colleague's workflow, and write ranked
P1/P2/P3 findings with locations and concrete fixes into a new docmgr reference doc, plus this diary.

**Inferred user intent:** Make the guide safe to hand to an implementer: concepts introduced properly, claims
true, mechanisms general rather than special-cased, phases executable without questions.

### What I did

- Read the writing guidelines (`textbook-authoring/SKILL.md`, `ticket-research-docmgr-remarkable/references/writing-style.md`) and the diary skill.
- Read the GCV-003 guide (860 lines) top to bottom; read GCV-001 §1-3, §7.2-7.6 and GCV-002 §6-7.12 for the parts GCV-003 relies on.
- Read issues 9, 10, 16, 17, 21, 23 in `07-goja-cv-handover-issues.md`, `84-cv-helpers.js` in full, the playbook §4-5, and how `83-cv-specks-vs-gold.py` calls goja-cv.
- Checked code with `nl -ba`/`sed -n`/`grep` in: `pkg/cv/graph/{regions,registry,args,node,errors,suggest,eval}.go`, `pkg/cv/ops/{regions,measure,mask,detect,value}.go`, `pkg/cv/kernels/{mask,measure_weighted}.go`, `pkg/cv/buffer/{buffer,io}.go`, `pkg/cv/look/{look,chart}.go`, `pkg/gojamod/cv/{binding,help,fields,prelude.js}`, `pkg/cvsession/{session,processor,mcp,native,yaml}.go`, `cmd/goja-cv/{eval,mcp}.go`, and go-go-goja `resultcontent/{resultcontent,files}.go`, `replmcp/tools.go`.
- Created the two docs with `docmgr doc add --ticket GCV-003 --doc-type reference --title "Review read findings"` and `--title "Diary - review read"` from the goja-cv root, and wrote the findings.

### Why

- The user asked for guides that properly introduce concepts and follow the updated guidelines; a reader who did not write the guide is the only reliable check of that.
- Correctness claims in a guide become acceptance tests; a wrong claim (ellipse coverage) becomes a test that cannot pass.

### What worked

- Most `file:line` references are exact; the quoted helper code is verbatim; the §13.2 helper mapping (core r + 0.5 at the pixel centre, ring r + 3.5 / r + 9.5, Rec. 709 spread) checks out against `EllipseMask`, `buffer.Luminance` and the `minus` op.
- The registry key `On:Name` (`registry.go:133`) confirms `Image:measure` and `Regions:measure` can coexist; Value index lifting (`sample(...)[0].l`) already works through `fieldOf` (`ops/value.go:22-41`).

### What didn't work

- `echo =====` inside a zsh command line failed with `(eval):1: ===== not found` (zsh `=word` expansion); I dropped the separators.
- A `grep` with an unquoted glob path failed with `ugrep: warning: ttmp/2026/11/09/GCV-003*/design-doc/01*.md: No such file or directory` when the glob was inside a variable; resolving it with `F=$(ls …)` fixed it.
- The first `Write` of the findings failed with `File has not been read yet. Read it first before writing to it.` because docmgr had created the file; reading it first fixed it.

### What I learned

- `EllipseMask` is 4 × 4 supersampled (`kernels/mask.go:173-200`), while `BoxMask` is exact; "exact coverage" is true only for boxes.
- `resultcontent.WriteFiles` names files `<prefix>-<N><ext>`; it has no per-item name.
- Render limits come from `Options.ContentLimits()` (`session.go:115-119`), not only from the MCP transport limits.
- Warnings exist only at bind time; `EvalCtx` has no warning channel.
- `OpDef.Fields []string` already exists and drives Value field lifting in the prelude.
- Only `goja-cv mcp` has `--root`; `eval`/`run` read anything.

### What was tricky to build

- Separating "the guide is wrong" from "the guide describes a proposal in present tense". The §7.9 sentence "The session enforces `MaxLooks` (4) itself in `state.collect`" reads as current behaviour; the code does nothing of the kind, so I filed it as a proposal written as fact (P2-4) rather than as a false claim.
- Judging the throwing `box`/`centroid` accessors against the no-shim rule. They return no data, so they are not an alias; I ranked them P3 with a fix that generalizes them through the schema and gives them a removal condition.

### What warrants a second pair of eyes

- P1-2: whether to keep 4 × 4 supersampling (exact helper equivalence) or move both `mask.ellipse` and the probe to exact area (cleaner definition, shifted playbook numbers). This is a product decision for the coordinator and the colleague.
- P1-4: the choice between `cx, cy` in measurement rows and the structural box/point rule.
- P2-9 touches GCV-001 (its lines 817-818 misdescribe the colleague's chroma); the GCV-001 author should confirm.

### What should be done in the future

- Revise the GCV-003 guide for the five P1 findings before handing it to an implementer.
- Run the same kind of review read on GCV-001 and GCV-002 if not already planned; P2-9 shows cross-ticket claims drift.

### Code review instructions

- Start with `reference/01-review-read-findings.md`, the Summary and P1 sections.
- Spot-check P1-2 with `nl -ba pkg/cv/kernels/mask.go | sed -n 173,200p`, P1-3 with `nl -ba ~/go/pkg/mod/github.com/go-go-golems/go-go-goja@v0.10.8-0.20261109062017-5e1ba117462e/pkg/resultcontent/files.go | sed -n 67,125p`, P2-4 with `nl -ba pkg/cvsession/session.go | sed -n 115,119p` and `sed -n 116,140p pkg/cvsession/processor.go`.

### Technical details

- Versions: goja-cv HEAD `72911b1`, guide at `b30efec`; go-go-goja `v0.10.8-0.20261109062017-5e1ba117462e`.
- Example evidence for P2-5: `ranked = hot.measure(dog, { stats: ["max"], prefix: "dog" }).measure(img, { ring: true }).sortBy("snr").top(30)` has at most 30 rows and 9 + 1 + 9 = 19 columns, while the guide's placeholder says "213 rows × 21 columns".

## Step 2: Re-check of the P1 findings after revision 2

The writer revised the guide (revision 2, commit `f6b03be`), appended a disposition table to the findings, and moved
shared vocabulary into a Foundations chapter in GCV-001. I re-checked each P1 against the new text and the code,
verified the two helper claims the coordinator singled out, skimmed every P2 disposition, and looked for new P1-level
problems in file naming, the writers and the naming rule across the three guides. All five P1s are resolved, and I
found no new P1. The new findings are two P2 (P2-14: unsanitized data file names; P2-15: the helper's centring bias)
and six P3 (P3-18 to P3-23). They are in the "Re-check (revision 2)" section of the findings doc.

### Prompt Context

**User prompt (verbatim):** "Re-check P1s after revision 2."

**Assistant interpretation:** Mark each of the five P1 findings resolved, partly resolved or not, with quotes. Verify
the claims that `DiskCoverage` equals `mask.ellipse`'s 4 × 4 supersampling and that the default core radius equals
`mem.hot`'s. Look for P2s marked fixed that are not. Report any new P1. Append a short section to the findings doc,
add a diary step, and do not edit the guide or commit.

**Inferred user intent:** Confirm that the guide can go to an implementer, and catch anything the revision broke.

### What I did

- Read revision 2 §1-§16 and the disposition table.
- Read Foundations §F4, §F8, §F9 and §F10, checked its section list against the P1-1 term table, and grepped it for
  each term.
- Read GCV-001 §7.3 (decoders, saved-file metadata), §7.4 (box/point types), the masks decision and Phase 4. Read
  GCV-002 §7.8 (annotate), §7.10 (`FuncDef`) and §7.13 (`EvalCtx.Warn`).
- Re-checked `EllipseMask` (`pkg/cv/kernels/mask.go:173-200`) against the helper's masks (`84-cv-helpers.js:36-37`) and
  the `minus` op.
- Checked `Engine.PreviewScale` (`engine.go:158-171`, `PreviewLong` 1600) and the planted scan size
  (`internal/testimage/planted.go:13-14`, 256 × 192). Both show the helper's `await stats()` runs at scale 1.
- Compared the radius formulas numerically: `python3 -I -c "…"` over areas 1-999 printed `[95]`, with √(95/π) =
  5.499 and √(95/3.14) = 5.500.
- Worked out the helper's centring from `mem.hot` (`84-cv-helpers.js:56`) and `NewRegion` (`regions.go:97`).
- Appended "Re-check (revision 2)" to `01-review-read-findings.md` with `cat >>`, since the writer had modified the file.

### Why

- The coordinator asked for verdicts per P1 and a check of the helper claims before the guide goes to an implementer.

### What worked

- The P1 fixes are concrete and consistent across tickets:
  - GCV-001's decoders read exactly what GCV-003's writers write (tag 270, `gAMA`, `sRGB`, `tEXt`).
  - GCV-001 §7.4 states the same `x, y` / `cx, cy` rule as GCV-003 §3.5.
  - GCV-002 §7.10 and §7.13 supply `FuncDef.On` and `EvalCtx.Warn`.

### What didn't work

- Nothing failed in this step. The findings doc had been modified by the writer, so I appended with a shell heredoc
  instead of rewriting the file.

### What I learned

- The helper's `Math.round(centroid)` followed by the probe's +0.5 inscribed-box convention shifts the probe centre
  by +0.5 px on average per axis (0 to 1 px), so the playbook numbers carry a small systematic bias.
- With half-integer centres and radii, the 4 × 4 sub-points never lie exactly on the circle, so floating-point
  differences between `x² + y² ≤ r²` and the normalized test cannot flip a sub-point.

### What was tricky to build

- Deciding whether "a sub-pixel difference" was acceptable wording. A full pixel at a 2.5 px core radius is not
  sub-pixel. Because §13.4 already re-measures the playbook ranges, I filed it as P2, not P1.

### What warrants a second pair of eyes

- P2-14 (data file name sanitization). It is a one-line rule, but it is a write path outside `--out-root`.
- Whether the colleague should be told about the helper's centring bias before the playbook ranges are re-measured.

### What should be done in the future

- Apply P2-14 and P2-15 to the guide. Fold the P3s into the next revision, or leave them to the implementer.

### Code review instructions

- Read the "Re-check (revision 2)" section at the end of `01-review-read-findings.md`.
- Check P2-15 by reading `84-cv-helpers.js:56` (`Math.round` of the centroid) next to `regions.go:97` (centroid =
  index + 0.5) and the §13.2 mapping (probe centre = x + 0.5).

### Technical details

- Helper centre for a region: round(cx) + 0.5. For cx ∈ [n − 0.5, n + 0.5) it is n + 0.5, so the offset from cx lies in
  (0, 1].
- Radius formulas differ only at area 95 in the clamp range [2, 8].
