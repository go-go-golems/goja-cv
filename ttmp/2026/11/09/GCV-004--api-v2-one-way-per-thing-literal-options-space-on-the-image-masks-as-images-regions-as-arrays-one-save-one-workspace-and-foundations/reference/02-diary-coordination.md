---
Title: Diary - coordination
Ticket: GCV-004
Status: active
Topics:
    - api
    - architecture
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Coordinator diary for GCV-004 (API v2 and foundations): the review-driven architecture analysis, the pi usage evidence, the guide, review read and implementation."
LastUpdated: 2026-11-09T12:07:17.289955-05:00
WhatFor: ""
WhenToUse: ""
---

# Diary — coordination (GCV-004)

## Goal

GCV-004 makes the bug classes found in the Codex review of PRs #2–#4 impossible to express, and reshapes the
JavaScript API around what 751 pi agent calls actually used: one way per thing, literal options, space on the
image, masks as images, regions as arrays, one save, one workspace. It continues the coordination diary of
GCV-001..003 (`GCV-001/reference/01-diary-coordination-gcv-001-003.md`, steps 1–9). Alpha internal software: no
backward compatibility, no shims; complexity is removed, not wrapped.

## Step 1: Architectural reading of the review findings (backfilled)

After fixing the 8 Codex findings one by one (GCV-001 diary step 9), the user asked whether something
foundational would make them impossible to express, whether any came from backward compatibility, and whether
anything added complexity for negligible edge cases. I grouped the findings by cause and proposed one
structural fix per class.

### Prompt Context

**User prompt (verbatim):** "anything architectural / foundational that could address thes eissues holistically, ie make them impossible to express through the API, is anything cuased by backwards compatibility or deprecated functionality? did anything add complexity for the sake of handling negligible edge cases?"

**Assistant interpretation:** Root-cause the review findings into classes, propose structural fixes, audit compat-shaped code and edge-case complexity.

**Inferred user intent:** Stop fixing instances; change the foundation so the classes cannot recur, and shed weight.

### What I did
- Checked facts before answering: Go 1.26 (`go.mod`), `os.Root` has `OpenFile`, `MkdirAll`, `WriteFile`; `Args.String` returns `""` for a non-string (an InputRef) silently; 22 hand-placed `Static: true`; `RemovedFields` (box, centroid, meanMask) with throwing prelude accessors; 6 `Redirects:` and 35 synonym tables; `decode_tiff.go` 726 lines; `cvsession/native.go` uses `graph.Within` for reads.
- Classes: (1) opt-in `Static` + silent zero-value accessors → encoding bug; (2) two representations of a node list (deduplicated `inputs` vs bound args) → `compare([img, img])`; (3) postcondition checked on one `Render` branch → oversize sheet; (4) hand-rolled infrastructure (path containment, JSON writer, CSV shape guessing) → symlink escape, JSON escapes, short-profile CSV. Plus: panic boundary placement (TIFF), duplicate data names (uniqueName), brittle smoke file name.
- Answer: none of the 8 came from backward compatibility; compat-shaped code exists anyway (RemovedFields, redirects for renamed names); my own fixes added edge-case code (symlink depth guard, `+1` tolerance, `a-2-2` naming, schema-less CSV fallback) that structural fixes delete.

### Why
- Fixing instances leaves the classes open; the user wants the API to make them unwritable.

### What worked
- Every finding mapped onto one of four causes; each cause has a deletion-shaped fix.

### What didn't work
- N/A

### What I learned
- The review bugs were all in new code; the risk is in mechanisms (opt-in flags, duplicate representations, home-made infrastructure), not in legacy.

### What was tricky to build
- N/A (analysis only)

### What warrants a second pair of eyes
- `os.Root` semantics on macOS for `MkdirAll` through existing symlinked parents inside the root.

### What should be done in the future
- Fold into a ticket (became GCV-004).

### Code review instructions
- N/A (no code).

## Step 2: Usage evidence from 751 pi calls (backfilled)

The user asked whether API changes, given no backward compatibility, would make things simpler yet tighter. I
answered from data: counted what the agents wrote in all 18 pi runs and which errors they hit.

### Prompt Context

**User prompt (verbatim):** "since we don't need backwards compatibility, would API changes make things even simpler yet tighter ?"

**Assistant interpretation:** Propose JS API changes that remove machinery, justified by actual agent usage.

**Inferred user intent:** A smaller, more regular API that is easier for agents and has fewer internal mechanisms.

### What I did
- Counted features and error codes across `run-*/calls.txt` (now `scripts/01-pi-api-usage.py`, output in `reference/01-evidence-pi-run-api-usage.md`).
- Key numbers: Params 0 uses; value lifting 4 (one run); `{ space }` 198 (80 × `stats({ space: "encoded" })`); `await` 1139 vs `cv.evaluate` 85; `.look(` 167 vs `show(` 87; JS `.filter(` 74; E_UNKNOWN_METHOD 35 (`mask.stats`, `Regions.filter`), E_ACCESS 23 (reopening saved files), E_WRONG_KIND 6 (arrays passed to overlay).
- Proposed: literal options (no Params, no lifting); space on the image (`img.as(…)`), measurements in the image's space; Mask = one-channel Image; awaited regions are arrays, arrays accepted where regions are; one way per thing (await, one display call, one spelling per op, one `cv.save(path, x)`); one workspace root; prune synonyms to logged guesses.

### Why
- Each proposal deletes a mechanism behind a bug class or a recurring agent error.

### What worked
- The first regex pass in zsh broke (unquoted glob); a Python pass over the call blocks gave clean counts.

### What didn't work
- `for p in …; awk … $C` in zsh: `$C` not word-split, so awk saw one filename with newlines ("can't open file").

### What I learned
- Agents override the measurement space constantly: the GCV-001 decision "space belongs to the op" fights how agents think (file units).

### What was tricky to build
- Regex counts measure use, not success; noted in the evidence doc.

### What warrants a second pair of eyes
- Item "space on the image" reverses a GCV-001 decision; the speck view keeps its linear default internally.

### What should be done in the future
- Validate with pi before/after on the GCV-001..003 question files.

### Code review instructions
- `scripts/01-pi-api-usage.py`; re-run on the pi-runs directory to reproduce the numbers.

## Step 3: GCV-004 ticket, evidence, writer

The user asked for a new docmgr ticket with a detailed intern guide in textbook style, uploaded to reMarkable,
and added that this is alpha internal software: rip out the complexity. I created the ticket on branch
`task/gcv-004-api-v2` (stacked on `task/gcv-003-measurement-data-out`, since v2 rewrites parts of #2–#4),
committed the evidence script and reference (`286947d`), and started the design writer.

### Prompt Context

**User prompt (verbatim):** "ok, create a new docmgr ticket and create a detailed guide / architecture guide / implementation guide for an intern new to the codebase in a textbook authoring style a dn upload to remarkable." Then: "also, write your diary / backfill if necessary , using the diary format from the diary skill" and "also, this is alpha internal testing software, no backwards compatibility needed, rip out the complexity"

**Assistant interpretation:** GCV-004 ticket + guide (architecture and implementation, intern level, textbook style), same process as GCV-001..003 (writer, review read, revision), upload to reMarkable; keep this diary with backfill; the guide removes complexity outright.

**Inferred user intent:** A guide an intern can implement from, that leaves goja-cv smaller and tighter.

### What I did
- `docmgr ticket create --ticket GCV-004 …`; docs: design-doc 01, reference 01 (evidence), reference 02 (this diary).
- Spawned the design writer with the guidelines (`~/.codex/skills/textbook-authoring/SKILL.md`, `~/.codex/skills/ticket-research-docmgr-remarkable/references/writing-style.md`, `ttmp/_guidelines/design-doc.md` if present), the foundations doc of GCV-001, the three GCV guides with their deviation notes, the evidence, and the decisions above.

### Why
- Same writer → review read → revision loop that brought GCV-001..003 from 15 P1 findings to 0.

### What worked
- N/A yet

### What didn't work
- N/A yet

### What I learned
- N/A yet

### What was tricky to build
- N/A yet

### What warrants a second pair of eyes
- Whether to merge #2–#4 before GCV-004 lands: v2 rewrites parts of them; stacking avoids churn.

### What should be done in the future
- Review read, revision, reMarkable upload, then implementation.

### Code review instructions
- `ttmp/2026/11/09/GCV-004--…/` on branch `task/gcv-004-api-v2`.

## Step 4: #2–#4 merged; binary installed; colleague onboarding; guide drafted

The user merged PRs #2–#4 (main `b847520`) and asked for a getting-started message for the colleague
(validating gold dust points with the fundamental ops, not `detect.specks`) and an install of the new binary.
Meanwhile the design writer finished the GCV-004 guide (`903846f` … `9c672d6`, 2,170 lines, D1–D14).

### Prompt Context

**User prompt (verbatim):** "merged all 3. you can pull and rebase. I also want my colleague to start working with what we have, they will be able to surface more bugs. What should I tell them to get started, they should not use the speck detection but more the fundamental options to help them along with validating a dataset of dust specs on negatives. Point them towards the tools and mcp and updates since ⏺ /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md and maybe install the binary with the new features as well."

**Assistant interpretation:** Install goja-cv from main; write a tested onboarding note for gold-point validation mapping the 23 handover issues to what shipped; give a paste-ready message; keep GCV-004 moving.

**Inferred user intent:** Put v1 in a real user's hands now so they find bugs while v2 is designed.

### What I did
- Built from a separate worktree of `origin/main` (the writer was using the main checkout) and `go install ./cmd/goja-cv` → `~/go/bin/goja-cv` (the user-scope MCP `cv` points there). No rebase of GCV-004: its base `454a836` is an ancestor of main (merge commits), so the branch already sits on main; no force push needed.
- Read `~/dustds/gold/*.json` (absolute raw-scan x, y, r; r covers the object) and the tile sidecars (`origin_x/y`); ran every snippet of the note against tile `1d955ef4b27d3d88_1536_1536`.
- Wrote DUST-002 `reference/08-goja-cv-v1-getting-started-for-gold-point-validation.md` (dust-scanning `52b44f1`): issue status table (17/23 done; 11, 12, 18, 19 not), setup (binary, MCP re-registration command for the user to run, CLI, docs), the loop (open at origin, speck view + gold circles, sheet, ruled zoom, measure sized from r, profile, CSV), reading the numbers, retiring `84-cv-helpers.js`, known issues, coming GCV-004 changes, how to report bugs.

### Why
- The colleague should validate with the primitives they asked for, and their bug reports feed GCV-004.

### What worked
- Sizing the measure from the gold radius: snr 0.12 (coreRadius 3) → 0.56 (coreRadius 63, ringRadii [95, 160]); the ruled zoom shows the labelled object as a faint vertical streak.

### What didn't work
- First run of the note's loop delivered no looks: bare `x.look()` statements are dropped silently. Second run delivered 4 of 5: `MaxLooks` = 4 per eval, extras listed as `[look: …]` without a number and no warning. Fixed the note (`show(...)`, ≤ 4 looks).
- `84-cv-helpers.js` (DUST-002) no longer runs: removed blend modes, `centroid`.

### What I learned
- Bugs found by using v1 on real data: silent look drops (bare look, > MaxLooks), `stepChroma` unstable when step ≈ 0 (3.25 at snr 0.12), sheet badge covers item 0's label, ruler labels collide at the axis start ("16191651").

### What was tricky to build
- Keeping the writer's working tree untouched: worktree for main, diary commit deferred until the writer finished.

### What warrants a second pair of eyes
- The MCP re-registration (adds `--out-root ~/dustdet/cv_out`) is left to the user; GCV-004 D-records change those flags again (one `--root` workspace).

### What should be done in the future
- Feed the five v1 bugs into the GCV-004 revision (the writer's "every look is queued" decision already covers the bare-look drop; MaxLooks needs a warning; stepChroma needs gating).
- Review read of the guide, revision, reMarkable upload.

### Code review instructions
- dust-scanning `52b44f1`; re-run the note's two JS blocks with `goja-cv eval --out DIR -e "…"`.

## Step 5: Review read, open-question decisions, revision 2, reMarkable

The reviewer's cold read found 37 issues (4 P1, 18 P2, 15 P3; 4 wrong citations among 100+ checked). I
merged its open-question recommendations with mine, sent the writer one revision list, and uploaded revision 2
(36 fixed, 1 partly, 0 declined) to reMarkable.

### Prompt Context

**User prompt (verbatim):** "what are pros/cons for the questions, and your suggestion (motivated)" and "what's the current status, and should we make a new ticket for these or are they integratable into our current work ?"

**Assistant interpretation:** Decide the guide's open questions with reasons; keep them inside GCV-004; finish the review loop and the upload the user asked for in step 3.

**Inferred user intent:** Settle the design and get the guide onto the reMarkable for reading.

### What I did
- Gave pros/cons and recommendations for Q1–Q6 and D3; said they belong to GCV-004 (its own §17), not a new ticket; proposed a small v1 patch on main for the colleague's bugs (user has not answered).
- Reviewer (`780c2bf`, `243a9aa`): P1s F1 channel counts of two-input ops, F2 masks must never be space-converted, F3 item links must not use the editable `id`, F4 "never overwrite an opened image" compared names, not files (an in-root symlink replaced the source).
- Merged decisions: gaussian/blend/within follow the image (receiver decides, masks never converted); exposure physical with a `levels({ auto })` hint; `measure` ring by default unless `stats`, radii from item `r` or √(area/π); `await img` returns statics; refuse arrays mixing two region sets (reviewer's position over mine); `--root` workspace + repeatable read-only `--read` (mine; reviewer's "mcp requires --root" and "no inline fallback").
- Writer revision 2 (`004ffd7`, diary `fb6673c`): 2,716 lines, Appendix A disposition, D15–D21, phases 0a/0b/0c then 1–8, est. net −2,050 lines.
- Uploaded guide + evidence + review findings as one bundle: `/ai/2026/11/09/GCV-004/GCV-004 API v2 design and implementation guide.pdf`; checked tasks 3 (review read) and 4 (upload).

### Why
- One revision round with the decisions settled avoids a third pass.

### What worked
- The reviewer verified claims by running code (the v1 channel-count result, the `os.Root` symlink overwrite), which turned two design assumptions into P1s.

### What didn't work
- N/A

### What I learned
- `os.Root` stops escapes but not in-root aliasing: a link inside the root to another file in the root is followed. "Don't overwrite X" needs file identity (`os.SameFile`), not paths.

### What was tricky to build
- Reconciling Q5/Q6 where the reviewer and I differed: took the simpler side (refuse mixed sets; no inline fallback) and kept `--read`, which neither the draft nor the review had, because the real setup needs it.

### What warrants a second pair of eyes
- New §17 Q3: the `stepChroma` gate at |snr| < 2 nulls it for about half the labelled dust (median snr 2.17). I recommend a lower gate or a noise-relative form; user decision pending.

### What should be done in the future
- User answers on §17 (4 questions) and the v1 patch; then implementation phase by phase with implementer diary and pi validation.

### Code review instructions
- Guide Appendix A (disposition) and §8 D15–D21; review `reference/04-review-read-findings.md`.
