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
