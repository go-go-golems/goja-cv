---
Title: Diary - review read
Ticket: GCV-005
Status: active
Topics:
    - api
    - agents
    - analysis
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-005--agent-ergonomics-and-analysis-primitives-script-libraries-compact-results-peaks-morphology/reference/04-review-read-findings.md
      Note: the findings this diary records (commit 2e8cde3)
    - Path: ttmp/2026/11/09/GCV-005--agent-ergonomics-and-analysis-primitives-script-libraries-compact-results-peaks-morphology/design-doc/01-design-and-implementation-guide.md
      Note: the guide under review
    - Path: pkg/cv/graph/order.go
      Note: CROP_FIRST lint behind F1
    - Path: pkg/cv/kernels/mask.go
      Note: Grow/Shrink edge arithmetic behind F2
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-evals
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-diaries
Summary: "Diary of the cold review read of the GCV-005 guide: what was checked against code, evidence and the running binary, what was found, and how to verify it."
LastUpdated: 2026-11-09T21:40:00-05:00
WhatFor: "Reviewing how the review findings were reached and re-running its checks."
WhenToUse: "When revising the guide to revision 2 or disputing a finding."
---

# Diary — review read (GCV-005)

## Goal

This diary records the cold review read of the GCV-005 design and implementation guide (revision 1): what I read,
which claims I checked and how, what I found, and how a second reader can re-run every check.

## Step 1: Cold read, verification and findings

I read the guide end to end as an intern would, then checked it in the order the brief asked: concept introduction,
citations and evidence, design soundness, implementability, simplicity, and the open questions. The citations were all
accurate; the problems were in what the code does around the design (a lint, an edge formula, the prelude path), in
evidence counts inherited from the hand-off, and in a few decisions that apply the guide's own rules unevenly.

The result is `reference/04-review-read-findings.md` (commit `2e8cde3`): 4 P1, 12 P2 and 17 P3 findings, each with
line numbers and a fix, and a recommendation for each of the four open questions.

### Prompt Context

**User prompt (verbatim):** "You are the reviewer for the goja-cv GCV-005 design guide. Do a cold review read, as a capable intern new to the codebase would read it, and record actionable findings. Do not edit the guide.

## Where things are
- **Worktree:** `/Users/manuel/code/wesen/go-go-golems/goja-cv-gcv005`, branch `task/gcv-005-agent-ergonomics-and-primitives`. Work only there. The checkout at `/Users/manuel/code/wesen/go-go-golems/goja-cv` belongs to another agent; do not touch it. Run `git pull` first.
- **The guide:** `ttmp/2026/11/09/GCV-005--agent-ergonomics-and-analysis-primitives-script-libraries-compact-results-peaks-morphology/design-doc/01-design-and-implementation-guide.md` (2,471 lines).
- **Ticket docs:**
  - `reference/01-analysis-annotator-agent-feedback.md`: the coordinator's disposition that the guide designs against.
  - `reference/02-diary-coordination.md`
  - `reference/03-diary-design-writer.md`
- **The base API is GCV-004**, being implemented in parallel:
  - its guide: `ttmp/2026/11/09/GCV-004--…/design-doc/01-design-and-implementation-guide.md` (revision 3);
  - the newest GCV-004 code and notes: `git fetch origin` and `git show origin/task/gcv-004-api-v2:<path>`.

  The GCV-005 guide's Appendix A lists which GCV-004 parts it relies on that are not implemented yet.
- **The evidence:** `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/`
  - `reference/11-goja-cv-agent-feedback.md`
  - `various/agent-evals/`
  - `various/agent-diaries/`
  - `scripts/84-cv-helpers.js`
  - `reference/04-goja-cv-dust-playbook.md`

  Read it; do not edit it.
- **Guidelines:**
  - `~/.codex/skills/textbook-authoring/SKILL.md`
  - `~/.codex/skills/ticket-research-docmgr-remarkable/references/writing-style.md`
  - `ttmp/_guidelines/design-doc.md`
- **The real code:** `pkg/cvsession`, `pkg/cv/graph`, `pkg/cv/ops`, `pkg/cv/kernels`, `pkg/gojamod/cv`, `cmd/goja-cv`.

## The user's design principles to judge against
Simplicity in API design, composability and consistency. Cut complexity: this is an alpha hard cutover, so there is no compat code and no new options where JS or an existing op does the job. Aim for maximum quality. One rule from the evidence: never make a model read a number off a picture.

## Check, in this order
1. **Concept introduction.** Every term is introduced before it is used, and an intern can follow the guide. List forward references and undefined terms with line numbers.
2. **Correctness against the code and the evidence.**
   - Spot-check at least 25 `path:line` citations and claims about current behaviour.
   - Check the evidence claims against `various/agent-evals` and the diaries: counts, quoted agent code, and the UNUSED_NODE re-analysis (the writer says the 12 false warnings were for nodes that were neither returned nor kept in `mem`; verify this).
   - Check the claim that goja's `Function.prototype.toString` returns comments by running a one-liner with `go run ./cmd/goja-cv eval -e '…'` in this worktree.
3. **Design soundness.**
   - **`cv.load`:** no caching, so every call reruns; up to 32 nested loads; the path rule; `--prelude` becomes a load; the `LOAD` warning taken from the promise state; security (`os.Root`, symlinks).
   - **Peaks:**
     - the algorithm: chord sums, NMS with the disk kernel, parabolic sub-pixel fit, the probe measure, merging across radii;
     - its noise estimate and its consistency with `measure`;
     - its cost on large DNG images at analysis scale 1;
     - the dropped options (`minDistance`, `box`, `max`, `above`), and whether `crop(...).peaks()` keeps coordinates absolute;
     - its output as a Regions node of disks.
   - **Morphology:**
     - the naming (`opening`/`closing`);
     - no `tophat` op;
     - merging grow/shrink into dilate/erode, which loses the anti-aliased edge;
     - kernel correctness and speed, including van Herk with a disk via chords, and the distance-transform fast path.
   - **Tables:** the automatic rendering rule, the budget of two thirds of the cap, and the paging note.
   - **Labels:** `label` else `id`; a non-numeric `id` is an error.
   - **The UNUSED_NODE rule:** warn only for nodes built in the eval's own code.
4. **Implementability.** Each phase must be green on its own, and Part A and Part B must be independent. Check that the GCV-004 dependencies per phase are right and that every phase has tests. Look for missing work: docs, MCP instructions under 4000 bytes, goldens, xgoja.
5. **Simplicity.** Flag anything added that is not needed, and anything kept that could go.
6. **The writer's open questions (§17).** Give a recommendation for each.

## Output
- **Findings.** Write them with `docmgr doc add --ticket GCV-005 --doc-type reference --title \"Review read findings\"`. Rate each P1 (blocks implementation, or wrong), P2 (should fix) or P3 (nice to have). Give each a line reference, the problem and a concrete fix. End with counts and your recommendations on the open questions.
- **Your diary.** Write it with `docmgr doc add --ticket GCV-005 --doc-type reference --title \"Diary - review read\"`, in the diary skill format (`~/.codex/skills/diary/SKILL.md`). Every step has these sections: Prompt Context, What I did, Why, What worked, What didn't work, What I learned, What was tricky to build, What warrants a second pair of eyes, What should be done in the future, Code review instructions, Technical details.
- **Commits.** Commit the findings, then the diary, and push with a plain `git push`. End commit messages with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Scope.** Touch only the ticket's `reference/` directory. If a shell safety check refuses a heredoc, use your file-writing tool.
- **Report back:** the counts by priority, the P1s in one line each, and your open-question recommendations."

**Assistant interpretation:** Read the GCV-005 guide cold, verify its citations, evidence and behavioural claims
against the code, the agent transcripts and the running binary, judge the design against the user's principles, and
write prioritized findings with fixes plus a diary, committing each and pushing.

**Inferred user intent:** A revision 2 of the guide that an intern can implement without discovering contradictions
mid-phase, and a design that stays as small as the evidence allows.

**Commit (docs):** `2e8cde3` — "GCV-005: review read findings (4 P1, 12 P2, 17 P3; open-question recommendations)"

### What I did

- `git pull` (up to date at `f676f60`), `git fetch origin`: `origin/task/gcv-004-api-v2` is now at `7554bd1` (Phases
  0a, 2 and 3 done since the guide's base `10938ce`).
- Read the whole guide (2,471 lines), the coordinator's disposition (`reference/01`), the writer's diary
  (`reference/03`), the relevant parts of the GCV-004 guide (§4.6, §6.4, §7.6, §7.8, D6, D7, D17–D20, phase list) and
  its implementation deviations 15–23 on the newest branch.
- Checked more than 60 citations with `sed -n` over the cited ranges (list in the findings' Context). All accurate.
- Ran the `toString` check: `go run ./cmd/goja-cv eval -e '…'` with arrow, async arrow, `function`, single-parameter,
  bound, native and method-shorthand functions.
- Recounted the evidence with three short Python scripts (run with `python3 -I`, in the scratchpad): first over
  `evals.jsonl`, then, because `result_head` keeps only eight lines per result, over the 39 raw transcripts the
  extractor reads. Counted pastes, non-paste evals and their median length, the regex counts, every warning code,
  cut notes (`# CUT`, `more characters`, `… N more`), the 28-of-32 probe results, hand-typed coordinate loops.
- Read the 39 diaries' friction sections for the quoted passages (de14d942, c71dd0f2 trial 2, af8640d7 trial 2,
  4b246f75) and the badge/paste/harmless counts.
- Built the binary into the scratchpad and ran it on `~/dustdet/cv_tiles/cfca85b41cd6f78f_2048_0.tif`: checked that
  `img.crop(b).view("speck")` and `m.crop(b).grow(2)` raise `CROP_FIRST`, that the speck view is one channel, that
  `sub`/`max` exist on images, and timed `measure` on 20,736 points (core 3) and 1,000 points (core 8).
- Read `order.go` (the only reader of `OpDef.Halo`), `Grow`/`Shrink`, `ProbePoint`, the sheet's list path, the
  `grow`/`shrink` users in tests and docs, goja's `SetPromiseRejectionTracker`, and replsession's `waitPromise`.
- Wrote `reference/04-review-read-findings.md` (F1–F33) and committed it.

### Why

- The brief asked for evidence counts to be checked; the extracted `result_head` is truncated, so only the raw
  transcripts can confirm warning counts and cuts.
- Behavioural claims about lints and kernels are cheap to test with the binary and expensive to discover mid-phase.

### What worked

- The raw transcripts settled every count question in one pass (39 pastes, 19 `UNUSED_NODE`, 0 `# CUT`, 40 array
  cuts, 1 string cut, max result 4,701 characters).
- Running the binary turned a suspicion about `Halo` into a fact: the guide's own "local stretch" pattern already
  draws a `CROP_FIRST` warning that tells the agent to do the opposite.
- Reading `Grow`'s one-line formula was enough to see that the B1 equivalence test cannot pass.

### What didn't work

- `grep -rn … --include=*.go` failed under zsh: `(eval):1: no matches found: --include=*.go`. Quoting the glob
  (`--include='*.go'`) or listing directories fixed it.
- `ls cmd/goja-cv/run.go` failed (`No such file or directory`): the `run` subcommand is defined in `eval.go:44`.
- The first eval timing script returned `Error:` with an empty message for `await m.stats()` on a cropped grown mask;
  I did not chase it, since only the warnings mattered for F1.
- `wc -c -m` on macOS printed a single number (3387) for the proposed instructions; good enough to confirm they are
  under 4,000 bytes.

### What I learned

- `OpDef.Halo` today feeds only the `CROP_FIRST` lint; there is no windowed evaluation, so a new op's `Halo` is a
  promise to warn after every crop.
- The extractor's paste flag is the literal `helpers ready (v1)`: ten agents trimmed the final `return`, so every
  "non-paste" regex count in the hand-off and the guide includes ten full helper files.
- The cut the agents complained about ("truncated at 9000 chars") was `maxStringLen` (2,000) on a joined string; no
  result in the study came near 9,000 characters.
- `Grow` at 0.5 is a dilation by r + 0.5, not r.
- goja exposes a promise rejection tracker that nothing in goja-cv or go-go-goja uses yet.

### What was tricky to build

- **Separating the guide's errors from the hand-off's.** The wrong counts (29 pastes, 13 warnings, the 9,000 cap) come
  from DUST-002 `reference/11` and `summary.md`; the guide repeated them faithfully. I rated them by their effect on
  the design (F4 is P1 because it changes what tables must be exempt from; F5 is P2) and asked for the coordinator to
  be told.
- **Judging attribution.** The writer's re-analysis is right for the study's warnings, but the rule also hides a real
  agent mistake (dropping a library's return value). Sites alone cannot distinguish the two; grouping by the eval's
  call line can (F10).
- **The prelude/docs interaction.** It only shows up when combining §7.4 (`fnSeen` updated by every context block) with
  `runPreludes` going through `Evaluate`; neither section is wrong alone.

### What warrants a second pair of eyes

- F1's proposed fixes: whether `orderWarnings` should skip non-Image results, or `peaks` should have no `Halo`.
- F15 (two morphology ops instead of four) is a judgment on consistency; the coordinator may prefer to keep
  `opening`/`closing` and add a `tophat` concept only.
- F13's timing is `measure` through JavaScript, not the bare kernel; a Go benchmark of `ProbePoint` would tighten it.
- F8's rejection-tracker proposal: check how go-go-goja's replsession owns the runtime before installing a tracker.

### What should be done in the future

- Revision 2 of the guide addressing F1–F16 at least; tell the coordinator about the DUST-002 count errors (F4, F5).
- Rebase the branch on the newest GCV-004 before Phase A2 (F17).
- Ask the GCV-004 implementer now about Phase 5: no `mask.regions({ grow })`, `id`/`label` in `items` entries, a
  `Point` shape (F11, open questions 1–2).

### Code review instructions

- Start with the findings' P1 section; each item names the code that proves it.
- Re-run F1: build `./cmd/goja-cv` and run
  `goja-cv eval --image ~/dustdet/cv_tiles/cfca85b41cd6f78f_2048_0.tif -e 'const a = img.crop([0,0,256,256]).view("speck"); return await a.stats()'`;
  the result carries `CROP_FIRST … put crop last`.
- Re-run F2 by reading `pkg/cv/kernels/mask.go:424` and `:444`.
- Re-run F3 by reading `pkg/cvsession/session.go:226-250` against guide §7.4.
- Re-run F4/F5 with the scripts in Technical details.

### Technical details

- Transcript recount (Python, `-I`): for each `agent-*.jsonl` under
  `~/.claude/projects/-Users-manuel-code-wesen-2026-11-07--dust-scanning/f3ad297e-8f51-4ac7-9531-e28596a20802/subagents`,
  pair `mcp__cv__eval` tool uses with their tool results (as `91-extract-agent-evals.py` does), keep agents whose first
  prompt has `Tile:`. A paste is an eval whose code contains both `mem.views =` and `mem.best =`.
- Results: 374 evals, 39 agents, 39 pastes (2,859–3,454 characters), 335 other evals (median 181 characters);
  `UNUSED_NODE` 19 (11 eval 2 without line, 7 eval 1 `line 10`, 1 eval 8 `line 1`); `RANGE` 11 in 9 evals; `OUTSIDE`
  0; `# CUT` 0; `… N more (return cv.json(x) for all)` in 40 evals; `more characters` in 1 (c71dd0f2 trial 2 eval 10);
  longest result 4,701 characters; regex counts over the 335: `.map(` 100, `.slice(` 72, `.filter(` 55,
  `mem.probe(` 42, `mem.hotByLevel` 44, `aspect` 21.
- cfca85b4 eval 8 sp values: 24 of 32 below 0.06, 28 below 0.1.
- `toString` output (this worktree): `async (x, y = 2) => { // best value near x …}`, `function h(a, /* b */ c) { /*
  block doc …`, `x => x`, `function max() { [native code] }`, `function bound f() { [native code] }`,
  `m() { // doc m … }`.
- Timing: `img.measure(20,736 points, { coreRadius: 3, ringRadii: [4.5, 7.5], channels: false })` 237 ms;
  1,000 points at core 8, ring [12, 20]: 39 ms; 10 cores.
- Proposed instructions (§12.1 text block): 3,387 bytes; current `Instructions`: 3,949 bytes, 3,940 characters.
