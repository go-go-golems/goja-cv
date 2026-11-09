---
Title: Diary - coordination (GCV-001..003)
Ticket: GCV-001
Status: active
Topics:
    - image-model
    - io
    - color
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Coordinator diary for GCV-001..003: ticket creation, design-guide writing and review read, implementation, verification."
LastUpdated: 2026-11-09T02:55:00.371223-05:00
WhatFor: ""
WhenToUse: ""
---

# Diary — coordination (GCV-001..003)

## Goal

Track the three tickets that follow the first real user's handover (DUST-002 `reference/07-goja-cv-handover-issues.md`):
GCV-001 image model, GCV-002 inspection views and looks, GCV-003 measurement and data out — from design guides
to merged code. Earlier history (design, prototypes, pi runs) is in CVJS-001 in the dust-scanning repo.

## Step 1: Tickets, guidelines, and the design writer

I merged the project-setup PR, created the three tickets in goja-cv's own `ttmp/`, read the writing guidelines the
user pointed to, and started one writer for all three guides so the architecture stays consistent; a separate
reviewer will do a cold "review read" before implementation.

### Prompt Context

**User prompt (verbatim):** "ok, write out tickets 001 002 003, we'll then pass it on. Make sure to read the updated guideines on writing design docs and properly introducing concepts by doing a review read.

Create the three ticket and write the design / implementation guide for an intern, in a textbook authoring manner. We are interested in keeping a strong core streamline but composable architecture, and opportunities to refactor / get rid of legacy or deprecated code should be taken. This is early stage, and no backwards wrappers needed.

Implement and commit at appropriate intervals, keep a daiary according to the format of the diary skill, subagents should write their own diaries as well. Go"

**Assistant interpretation:** Create GCV-001..003; write intern-level textbook design/implementation guides following the updated writing guidelines; review-read them for concept introduction; then implement all three with commits and diaries.

**Inferred user intent:** Hand well-explained tickets to implementers and get the colleague's blockers fixed with a cleaner core.

### What I did
- `gh pr merge 1 --merge --delete-branch` (goja-cv PR #1, `14de61c`).
- Looked for the "updated guidelines": newest relevant files are `~/.codex/skills/ticket-research-docmgr-remarkable/references/writing-style.md` and `~/.codex/skills/textbook-authoring/SKILL.md` (both 2026-08-25); plus `ttmp/_guidelines/design-doc.md`. Interpreted "review read" as a separate cold read of each guide checking that concepts are introduced before use.
- `docmgr ticket create` GCV-001..003, design-doc + this diary, topic vocabulary; commit `175b5d4`.
- Started the design writer (all three guides, diary `reference/02-diary-design-writer.md`).

### Why
- One writer keeps vocabulary and architecture consistent across the three tickets; an independent reader catches undefined concepts the writer cannot see.

### What worked
- docmgr in goja-cv (`.ttmp.yaml` root `ttmp`).

### What didn't work
- No newer guideline file under `~/.pi/agent/skills` (path absent on this Mac).

### What I learned
- N/A

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- My reading of "updated guidelines" and "review read" (see What I did).

### What should be done in the future
- Review read, revisions, then implementation in ticket order 001 → 002 → 003.

### Code review instructions
- `ttmp/2026/11/09/GCV-00*/design-doc/01-design-and-implementation-guide.md` once written.

## Step 2: Guides written; review read started

The writer delivered the three guides (GCV-001 ~1280 lines/20 tasks, GCV-002 ~960/15, GCV-003 ~860/13), same
15-section outline, `docmgr doctor` clean, commits `7b3bb16`, `e1855d5`, `b30efec`, `bfbc3a0` (+ diary `193e98f`).
I started three independent reviewers, one per guide, each writing ranked findings and its own diary, without
committing (the writer and reviewers share the repo).

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Run the review read before anything goes to implementers.

**Inferred user intent:** Guides an intern can execute without asking.

### What I did
- Read the writer's report: key decisions (encoding belongs to data, space to ops, explicit `convert` nodes; `Frame` replaces DocW/DocH; macros via `OpDef.Expand`; pixel-dependent things are nodes, output-only things are look options; signed arithmetic without `offset`; `img.view` presets; one flat region schema; result content with separate budgets; `--out-root`) and the legacy-removal lists.
- Spawned reviewers for GCV-001/002/003 (checks: concept introduction, code correctness, gaps, guideline conformance, implementability, architecture), findings in `reference/*review-read-findings*`, diaries `reference/*diary-review-read*`.

### Why
- Three parallel cold reads are faster and keep each reviewer focused on one guide.

### What worked
- N/A yet.

### What didn't work
- N/A

### What I learned
- The writer departed from the colleague's proposals in three places (`img.view` vs `cv.view`, no `offset`, no `items({ all })`), each with a decision record — reviewers should judge those.

### What was tricky to build
- Concurrency: several agents in one repo; only the coordinator and the writer commit.

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- Send findings to the writer for revision; then implementation.

### Code review instructions
- The three design docs; reviewers' findings once written.

## Step 3: Review read results; revision round

All three reviewers finished (GCV-001: 4 P1/22 P2/21 P3; GCV-002: 6/16/15; GCV-003: 5/13/17). I committed their
findings and diaries (`07e569e`) and sent everything to the writer for one revision round, with the cross-cutting
fixes named explicitly so they are solved once.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Apply the review read before handing the guides to implementers.

**Inferred user intent:** Guides that introduce concepts properly and are executable.

### What I did
- Read the three reports. The reviewers converged on: missing foundations (CVJS-001 vocabulary from another repo; pixel/scale terms undefined and inconsistent), phase orders that would break the build between phases, special mechanisms beside existing ones (`RegionFields` vs `OpDef.Fields`, `FuncDef` vs `KindNone` ops, two ring-measure paths), field-name collisions (`x,y`, `r`, gray mean, `chroma`), space semantics (luminance in encoded space changes pure red 0.498 → 0.213; resolution must happen inside `internNode`/`Substitute`), decorations/sheets that cannot work as specified (scale order, build-time size, 1568 px limit), mutable caption fields on hash-consed nodes, and data-out claims that do not match go-go-goja `resultcontent` (file names) or existing writers.
- Sent the writer: fix all P1/P2, fix-or-decline P3, revision history per guide, a disposition table per findings doc.
- `docmgr doctor` on the three tickets: no errors.

### Why
- Several findings span tickets; naming the shared fixes avoids three inconsistent local patches.

### What worked
- The reviewers verified nearly every `file:line` citation; problems were conceptual and architectural, not factual.

### What didn't work
- N/A

### What I learned
- A guide that builds on vocabulary from another repo reads as unfinished to a cold reader, even when every claim is right.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Whether one revision round is enough; a short P1 re-check by the reviewers after revision is cheap.

### What should be done in the future
- Re-check P1 dispositions; then implement GCV-001.

### Code review instructions
- `reference/*review-read-findings*` in each ticket.

## Step 4: Revision 2, re-checks, revision 3

The writer's revision 2 (`48e3a50`, `a31654a`, `f6b03be`, diary `2915fb2`) added a shared foundations chapter
(`reference/05-foundations-goja-cv-for-the-gcv-guides.md`) and resolved the P1s. I asked each reviewer to re-check
its own P1s; their re-checks are committed (`3b5c033`). GCV-002 and GCV-003: all P1s resolved, no new P1s.
GCV-001: P1-1 partly resolved and five new P2-level items (N1–N5). I sent one short revision-3 list.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Converge the guides before implementation.

**Inferred user intent:** Executable guides with concepts introduced properly.

### What I did
- Resumed the three reviewers for a P1 re-check of revision 2.
- Collected new items: GCV-001 N1 (goldens cannot be byte-identical: exact transfer functions flip ~0.26% of 8-bit codes), N2 (test inputs before Phase 4/5), N3 (luminance rule must cover `refine`'s image input), N4 (speck `meanL` space change undocumented), N5 (evaluator must not write statics into values it did not create); GCV-002 N1 (oversized box items vs fixed cells) and P3s; GCV-003 P2-14 (file-name sanitizing), P2-15 (`mem.hot` 0–1 px offset) and P3s.
- Sent revision 3 to the writer.

### Why
- Cheap re-checks caught real problems the revision introduced (N5 came from the P2-8 fix).

### What worked
- The re-check pattern: the reviewer who raised a P1 judges its resolution.

### What didn't work
- N/A

### What I learned
- Fixes can introduce new defects at the same level of detail; a second cold read pays for itself.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- After revision 3: implement GCV-001 (branch, PR, CI), then 002, then 003.

### Code review instructions
- "Re-check (revision 2)" sections at the end of each findings doc.

## Step 5: Revision 3 done; GCV-001 implementation started

The writer's revision 3 (`82f93a6`, `92d026e`, `ff3bf42`, diary `3cbff7a`) fixed every re-check item without
declining any. The design phase is closed. I started the GCV-001 implementer on branch
`task/gcv-001-image-model`, phase by phase, with its own diary (`reference/…diary-implementation…`).

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Move from guides to code: GCV-001 first.

**Inferred user intent:** Working code following the reviewed guides.

### What I did
- Noted revision-3 changes: golden tolerance rule (≤ 1 code on ≤ 1 % of samples; levels/mask looks expected to change; regenerate with `-update` after the check), `graphtest.RegisterSource` for early phases, `Gray` covering luminance inputs, shallow-copy stamping of statics, sheet cell windows rendered at `zoom·size/side`, Gaussian args renamed to `sigma`/`radius` (breaking), data names restricted to `[A-Za-z0-9_-]{1,64}`, `EllipseCoverage(rx, ry)`.
- Spawned the GCV-001 implementer: branch, per-phase green checks (build, vet, race tests, lint, logcopter), push per phase, PR at the end, CI watch, no merge; deviations recorded in the diary and appended to the guide.

### Why
- GCV-002 and GCV-003 build on GCV-001's encodings, spaces, frames, macros and box/point types.

### What worked
- Three review/revision rounds converged: P1 counts 15 → 1 partly → 0.

### What didn't work
- N/A

### What I learned
- N/A

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- The breaking renames (`gaussian(sigma)`, `radius`) touch docs and examples across the repo.

### What should be done in the future
- Verify GCV-001's PR; then GCV-002, then GCV-003; pi runs on a real tile after GCV-002.

### Code review instructions
- Branch `task/gcv-001-image-model`; its PR when opened.

## Step 6: GCV-001 verified; GCV-002 stacked on it

The GCV-001 implementer finished: 8 phases, PR #2 (https://github.com/go-go-golems/goja-cv/pull/2), diary
`reference/06-diary-implementation.md` (steps 1–9, every skill section present). I re-ran the checks on the
branch myself and committed the pi harness changes and runs 07–10 in the dust-scanning repo (`a5ffac4`).
I did not merge PR #2: merges are the user's call (they merged #1 by explicit request). GCV-002 branches
from `task/gcv-001-image-model` and its PR targets that branch, so the stack merges in order.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Verify GCV-001, then keep going with GCV-002 without waiting.

**Inferred user intent:** All three tickets implemented, each reviewable on its own.

### What I did
- Local re-check on `task/gcv-001-image-model` (355f1c4): `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race` (all packages ok), `make lint` clean.
- Read the hand-back: all 12 legacy items of guide §6 removed; 19 deviations appended to the guide; golden check before regenerating showed only the expected looks (levels, mask, overlay) moving.
- Committed the harness `IMAGE`/`IMAGE_EXTRA`/`QUESTIONS` options and runs 07–10 in dust-scanning (`a5ffac4`).
- Spawned the GCV-002 implementer on `task/gcv-002-inspection-views`, based on the GCV-001 branch, with the hand-back's open items for GCV-002.

### Why
- GCV-002 needs GCV-001's spaces, frames, macros and `Engine.Display`; stacking avoids waiting on a merge.

### What worked
- pi runs before/after GCV-001: 48/41 calls → 26/23 calls for the same questions; no error from a missing capability after.

### What didn't work
- CI "Dependency Review" fails on PR #2 (and #1) because the repository's Dependency graph setting is off — not caused by the code.

### What I learned
- The phase-by-phase brief with "check `go test`'s own exit status" held: no red pushes this time.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Breaking display change: masks show their own values (0.5 → grey 128); `levels` remaps in linear space; `detect.specks` `meanL` in encoded space on sRGB input — downstream thresholds in DUST-002 helpers move.

### What should be done in the future
- User: enable Dependency graph (or drop the job); merge #2 then #3 in order.
- GCV-003 after GCV-002; pi run on a real tile after GCV-002.

### Code review instructions
- PR #2 diff; start with `pkg/cv/buffer/encoding.go`, `pkg/cv/graph` (space rules, `internNode` conversions), the decoder registry, and the guide's "Implementation notes (deviations)".
