---
Title: Diary - coordination
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
Summary: "Coordinator diary for GCV-005 (agent ergonomics and analysis primitives): feedback analysis, scope, guide, review, implementation."
LastUpdated: 2026-11-09T14:23:00.444093-05:00
WhatFor: ""
WhenToUse: ""
---

# Diary — coordination (GCV-005)

## Goal

GCV-005 turns the feedback of 39 annotator agents (DUST-002 `reference/11`) into goja-cv changes: task script
libraries as a first-class part of a session, results agents can read without misreading, and the analysis
primitives they had to improvise (peaks, morphology). It builds on GCV-004 (API v2) and follows its rules: alpha
hard cutover, one way per thing, cut complexity.

## Step 1: Analysis of the annotator feedback (backfilled)

The user pasted the colleague's hand-off (ten issues, 374 evals, suggested order) and asked what I thought of it
while the GCV-004 agents were working.

### Prompt Context

**User prompt (verbatim):** "While the agents are working, analyze this and tell me what you think of it" (with the pasted hand-off pointing to DUST-002 `reference/11-goja-cv-agent-feedback.md`, the vault article, `various/agent-evals/`, `various/agent-diaries/`, `scripts/84-cv-helpers.js`, and a suggested order: eval_file, sheet fixes, peaks, view white option, smaller items).

**Assistant interpretation:** Assess the evidence and the asks against GCV-004's direction; say what to build, how, and what not to.

**Inferred user intent:** Decide what the next goja-cv work should be, grounded in real agent use.

### What I did
- Read `reference/11` and `various/agent-evals/summary.md`; checked the helper library size and the eval counts.
- Key reading: helpers replaced docs; models misread coordinates off rulers (28/32 guesses at sp ≈ 0); agents filter in plain JS; the noise-scaled view was tested worse (87 % vs 91 %).
- Caveats: "0 errors" reflects the helpers; "never used" means "not shown"; Sonnet-with-playbook vs DeepSeek-without.
- Disposition per issue (now `reference/01-analysis-annotator-agent-feedback.md`): `cv.load` instead of an MCP-only `eval_file`; `peaks` in the flat schema (`cx, cy`); one morphology family for one-channel images (covers rings, lines and old issue 18, and merges mask grow/shrink once masks are images); no `maxAspect` (JS filter); UNUSED_NODE escape rule; automatic compact tables; recipes in the task library.

### Why
- Several asks can be absorbed by GCV-004's model instead of new options; a few need real primitives.

### What worked
- The colleague's counts made each disposition checkable.

### What didn't work
- N/A

### What I learned
- The design rule "never make a model read a number off a picture" explains issues 2 and 3 together.

### What was tricky to build
- N/A (analysis)

### What warrants a second pair of eyes
- `cv.load` vs an `eval_file` tool: token cost of the call vs one mechanism across MCP, CLI and xgoja.

### What should be done in the future
- Ticket and guide (step 2).

### Code review instructions
- N/A

## Step 2: Ticket, scope, worktree

I proposed splitting into GCV-005 (ergonomics) and GCV-006 (analysis primitives); the user chose one ticket and
asked for the guide, the reMarkable upload, then implementation, with simplicity, composability and consistency.

### Prompt Context

**User prompt (verbatim):** "what about the morphologyand peaks and whatever stuff, is that a ticket after 005?" then "nah 005 is fine. Createa  new docmgr ticket and create a detailed intern guide about architecture and implementation and design and analysis, in a textbook autheoring sytle, and upload to remarkable. Then implement. Make sure to follow simplificity in API design and composability and consistence. Cut complexity, we are still in alpha hard cutover stage where we can tackle things as we want for maximum quality."

**Assistant interpretation:** One ticket GCV-005 with both halves; same writer → review → revision → upload loop; implement after GCV-004.

**Inferred user intent:** The next quality step for agent use, designed as carefully as GCV-004.

### What I did
- Worktree `~/code/wesen/go-go-golems/goja-cv-gcv005`, branch `task/gcv-005-agent-ergonomics-and-primitives` from `origin/task/gcv-004-api-v2` (`10938ce`, GCV-004 batch 1 in progress), so the GCV-004 implementer's checkout is untouched.
- `docmgr ticket create --ticket GCV-005`; docs: design-doc 01, reference 01 (analysis), reference 02 (this diary).

### Why
- GCV-005 depends on GCV-004's workspace (`cv.load`), item schema (`peaks`) and masks-as-images (morphology); stacking keeps it reviewable on its own.

### What worked
- N/A yet

### What didn't work
- N/A yet

### What I learned
- N/A yet

### What was tricky to build
- The base moves while GCV-004 is implemented: the writer must design against the GCV-004 guide (revision 3) and its implementation notes, and the branch will merge GCV-004's later commits.

### What warrants a second pair of eyes
- Scope: two halves in one ticket; the phases must keep them independent so either half can land first.

### What should be done in the future
- Writer, review read, revision, upload; implementation after GCV-004 merges.

### Code review instructions
- `ttmp/2026/11/09/GCV-005--…/` on `task/gcv-005-agent-ergonomics-and-primitives`.

## Step 3: Guide, review read, revisions 2–3, reMarkable

The writer produced revision 1 (2,471 lines, D1–D21); the cold reviewer found 33 issues (4 P1, 12 P2, 17 P3);
I sent one decision list for revision 2 and a deletion-shaped revision 3; uploaded the result.

### Prompt Context

**User prompt (verbatim):** (see Step 2)

**Assistant interpretation:** Same writer → review → revision → upload loop as GCV-004, judged against "simplicity, composability, consistency; cut complexity".

**Inferred user intent:** A guide an intern can implement from, with nothing in it that isn't needed.

### What I did
- Revision 1 (`67fa07b` … `f676f60`): `cv.load` instead of an MCP-only `eval_file` (a paste is ~3 KB; `eval_file` would save ~20 characters over `await cv.load(...)`), loads not cached, `--prelude` = a load at start, function docs from the body's leading comment, `peaks({ radius, snr, polarity })` as a Regions node of disks with measure's core/ring fields, morphology, automatic tables, no `white` option.
- Review (`2e8cde3`, `8c7ab17`): P1s — CROP_FIRST lint contradicts crop-first use; dilate/erode ≠ grow/shrink (r + 0.5); preludes hide function docs; the 9,000-char cap never fired (the 2,000-char string cap and 48-item array cap did). Evidence counts inherited from the hand-off and script 91 were wrong (39 pastes, not 29; UNUSED_NODE 19 with 18 false).
- Decisions for revision 2: delete CROP_FIRST; standard disk (≤ r) with a brute-force test; preludes don't mark functions seen; tables exempt from item/string caps; docs from a comment that is the body's first token, shown when the name is new or its source changed; one `UNHANDLED` warning from goja's rejection tracker; `__cv_run` waits for loads; load depth, not count; only `erode`/`dilate` (opening/closing/top-hat as recipes with help concepts); peaks in row bands, snr default 1 pending a sweep; disk cap 64. Revision 2: `6bb71a8` (merge of GCV-004 batch 1), `47c4470`, `e4af848`; numpy prototype on 4 gold tiles: snr ≥ 1 covers 101/102 dust points, ≥ 1.4 covers 77/102.
- Revision 2 had grown a jsparse source rewrite to keep UNUSED_NODE alive; I cut it. Revision 3 (`d8be267`, `4b17382`): UNUSED_NODE deleted outright (18/19 false; the one true positive silent under the new rule; dropped lazy nodes are no-ops in v2), `OpDef.Halo` deleted, sheet captions settled; 2,398 lines, no open questions.
- Sent GCV-004 batch 2: one `items|point[]` binding with a Point shape, labels, ids may repeat; no `mask.regions` `grow`; `cv.save` promises on the eval's pending list.
- Uploaded guide + analysis + review findings: `/ai/2026/11/09/GCV-005/GCV-005 agent ergonomics and analysis primitives guide.pdf`.

### Why
- Every decision picked the option that deletes a mechanism.

### What worked
- The reviewer re-verified the evidence against the raw evals and caught counting errors in the source hand-off.

### What didn't work
- Revision 2 added complexity (source rewriting) to keep a warning with no true positives; caught at coordination.

### What I learned
- A warning should be judged by its true positives in the field: UNUSED_NODE had one, and the redesign would have lost it.

### What was tricky to build
- Keeping GCV-005 consistent with GCV-004 while batch 2 is still changing items, points and the workspace.

### What warrants a second pair of eyes
- The colleague's DUST-002 `reference/11`, `summary.md` and script 91 still carry the wrong counts; asked the user whether to fix them.

### What should be done in the future
- Implementation: B1 (morphology) depends only on GCV-004 Phase 4 and can start now; the rest after GCV-004 batch 2.

### Code review instructions
- Guide revision 3, Appendix B (dispositions), §8 decision records, §9 deletion checklist.

## Step 4: Implementation (B1, A1–A4, B2, B3, V), PR #7, require hole closed

Three implementers built GCV-005 in its own worktree: B1 (morphology) while GCV-004 batch 2 ran, batch A (merge of
final GCV-004, A1–A4) and batch B (B2 peaks, B3, validation, PR #7). Validation found a sandbox hole that predates
GCV-005; I fixed it on GCV-004 (PR #6) and it was merged up.

### Prompt Context

**User prompt (verbatim):** (see Step 2: "… Then implement. Make sure to follow simplificity in API design and composability and consistence. Cut complexity …")

**Assistant interpretation:** Implement phase by phase, verify each batch, keep the stack mergeable.

**Inferred user intent:** The agent-facing improvements, measured.

### What I did
- B1 (`bcefb0a`, `5c26122`, `17dbcb1`): `erode`/`dilate` (disk ≤ 64, line any length), grow/shrink/Halo/CROP_FIRST deleted; 1024² r = 64 disk 25 ms. Verified gate.
- Batch A (`259df82` merge of GCV-004 a42c9d1, 10 conflicts resolved toward GCV-004 + B1 deletions; A1 `c0ad935` cv.load/cv.scripts/prelude-as-load; A2 `b10ff2d` tables + function docs; A3 `c9987c0` sheet captions; A4 `eaafc1d`/`7cec0bf` UNUSED_NODE deleted, docs). Verified gate and smoke-tested `cv.load` + docs + table on a real tile.
- Batch B (B2 `0e04924` peaks, B3 `965358c`, merge of GCV-004 review fixes `32610c2`, V `7c6b858`, CI fix `0e96cce`, PR #7). Gold (15 tiles, 190 points): peaks snr 1 cover 90.5 % vs hot list 86.3 %, position error median 0.36 vs 0.99 px; snr default 1 kept (1.4 → 70.5 %); top-hat 60/63 saturated; line opening r 12 cuts elongated components 40 → 8 for 1/164 dust. Pi: after-runs locate 14/13 of 53 gold points vs 4/5 before, errors 11/2 vs 18/8.
- Validation found `require("/abs/path.js")` loading any file (the before-agents used it to load their library). Fixed on GCV-004: cv sessions install a require loader that refuses files (`8ab11af`, test fails without fix); merged into GCV-005 (`c70557f`), PR #7 green on `06ec6a6`.
- Archived pi runs 31–36 in CVJS-001 (dust-scanning `054ef2c`).

### Why
- Verify every batch myself; fix sandbox holes at the layer that owns them (GCV-004's workspace).

### What worked
- Running B1 in parallel with GCV-004 batch 2: the later merge had only mechanical conflicts because both sides kept to the guide.
- Peaks are both more complete and more precise than the helper hot list, and the API stayed at three options.

### What didn't work
- My message to batch B named the require commit `f5fd593`; the real hash is `8ab11af` (the agent found it by message).

### What I learned
- Agents probe the sandbox: an escape that nobody designed for got used within two runs.

### What was tricky to build
- Stacked PRs (#6 → #7) with review fixes landing on the lower branch and merged upward.

### What warrants a second pair of eyes
- The xgoja provider keeps the host's require policy (by design); document it if an xgoja host is exposed to agents.
- The annotator comparison is one session per build with a different model than the study: indicative only.

### What should be done in the future
- User merges #6, then #7. Then: reinstall the binary, update the colleague's note for v2 (MCP flags `--root`/`--read`, `--prelude` for helpers, `peaks`), port `84-cv-helpers.js` to v2 (the v2 ports in `gcv005-scripts/lib-after` are a start).

### Code review instructions
- PR #7; guide deviations 1–45; implementation diary steps 1–12.
