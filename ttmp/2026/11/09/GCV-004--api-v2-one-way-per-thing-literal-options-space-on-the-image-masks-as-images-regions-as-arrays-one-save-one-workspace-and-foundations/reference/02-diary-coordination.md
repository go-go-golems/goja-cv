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
