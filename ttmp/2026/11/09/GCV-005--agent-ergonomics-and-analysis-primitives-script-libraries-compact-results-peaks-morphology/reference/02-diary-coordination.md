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
