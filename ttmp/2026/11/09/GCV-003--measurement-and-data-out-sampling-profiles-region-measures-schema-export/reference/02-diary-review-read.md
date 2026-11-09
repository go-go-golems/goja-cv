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
