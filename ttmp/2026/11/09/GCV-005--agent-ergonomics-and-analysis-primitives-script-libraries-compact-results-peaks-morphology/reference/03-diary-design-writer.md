---
Title: Diary - design writer
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
    - Path: ttmp/2026/11/09/GCV-005--agent-ergonomics-and-analysis-primitives-script-libraries-compact-results-peaks-morphology/design-doc/01-design-and-implementation-guide.md
      Note: the guide this diary records
    - Path: pkg/cvsession/processor.go
      Note: unusedNodes, contextBlock, errorNode read for the current-state analysis
    - Path: pkg/cvsession/yaml.go
      Note: renderer caps and capText read for the table design
    - Path: pkg/cv/kernels/morph.go
      Note: existing van Herk min/max, the base of the morphology kernels
    - Path: pkg/cv/kernels/probe.go
      Note: ProbeCoreRing, which peaks reports
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-evals
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-diaries
Summary: "Design writer diary for the GCV-005 guide: reading, evidence checks, decisions taken where the brief left a choice, drafts and verification."
LastUpdated: 2026-11-09T20:00:00-05:00
WhatFor: "Reviewing how the GCV-005 guide was written and why its decisions came out as they did."
WhenToUse: "Before a review read or a revision of the GCV-005 guide."
---

# Diary — design writer (GCV-005)

## Goal

This diary records how the GCV-005 design and implementation guide was written: what was read, which numbers were
checked against the evidence, which choices the brief left open and how they were decided, and what remains for
review.

## Step 1: Reading, evidence checks and the draft (sections 1–9)

I read the brief's sources in order (the coordinator's analysis and diary, the writing guidelines, the GCV-004 guide
in full including its implementation deviations, the ten-issue hand-off, the helper library, the playbook, the vault
article), then the code the guide cites, then mined the agent evals for the before/after examples. The draft covers
the evidence, concepts, current state, API, architecture, decision records and deletions; it was committed as a draft
before the pseudocode, phases, tests and validation were written.

The main surprise was issue 8: the brief framed it as "nodes reachable from the returned value or mem must not warn",
but the 12 false warnings came from a node that was neither returned nor kept (a helper's `speckColor` dropped by the
caller). Reachability already holds today; the fix that matches the evidence is attributing the lint to the eval's own
code. The guide keeps reachability (and makes the `mem` walk uncapped) and adds attribution.

### Prompt Context

**User prompt (verbatim):** "You are the design writer for goja-cv ticket GCV-005, "Agent ergonomics and analysis primitives: script libraries, compact results, peaks, morphology". Write a detailed architecture, design, analysis and implementation guide for an intern who is new to the codebase, in a textbook authoring style. An intern must be able to understand the problem and implement GCV-005 phase by phase from the guide alone. [… the full brief: worktree, ticket paths, evidence locations, GCV-004 base, PR #5, writing guidelines, design principles, Part A items 1–6, Part B items 7–9, out-of-scope list, guide structure, tasks.md, process (diary, commits, docmgr doctor), report contents …]"

**Assistant interpretation:** Write the GCV-005 guide against GCV-004's final model, deciding the open design choices
(cv.load scope and caching, eval_file, peaks options and algorithm, grow/shrink vs morphology, the white option) with
decision records, phases that keep Part A and Part B independent, relative gold tests, and a validation plan; keep my
own diary and commit draft and final.

**Inferred user intent:** A guide good enough that an intern can implement GCV-005 without re-deriving the design, and
that keeps goja-cv simple: one mechanism per need, data with every look.

### What I did

- Read `reference/01-analysis-annotator-agent-feedback.md`, `reference/02-diary-coordination.md`, the three writing
  guides, the diary skill, the GCV-004 guide (all 2,792 lines, including deviations 1–14) and the outline of its
  implementation diary; `git fetch origin` showed `origin/task/gcv-004-api-v2` at `10938ce`, the base of this branch,
  so no newer GCV-004 commits existed.
- Read the evidence: `DUST-002/reference/11-…`, `scripts/84-cv-helpers.js` (now 7,842 bytes with `mem.faint` and
  `mem.sweep`), the playbook, the article's issue sections and old issue 18 in `reference/07-…`.
- Mined `various/agent-evals/evals.jsonl` with a short Python script: 345 non-paste evals, median 183 characters,
  pastes 2,859–3,418 characters, regex counts (`.filter(` 55, `.slice(` 82, `.map(` 110, `aspect` 31, `mem.probe` 52,
  `mem.hotByLevel` 54); printed all 13 `UNUSED_NODE` warnings with the code that raised them.
- Collected every agent diary's "goja-cv friction" section (39 diaries) for quotes.
- Read the code: `pkg/cvsession/{session.go,init.js,processor.go,yaml.go,native.go,reference.go,mcp.go}`,
  `pkg/gojamod/cv/{evaltrack.go,binding.go,prelude.js}`, `pkg/cv/kernels/{morph.go,mask.go,specks.go,probe.go}`,
  `pkg/cv/ops/{mask.go,sheet.go,present.go,views.go,detect.go,measurement.go,regions.go}`, `cmd/goja-cv/mcp.go`,
  the xgoja provider; grepped which GCV-004 features exist on the branch (Phases 0b, 0c, 1: `export`/`isGojaPanic`,
  `MarshalJSON`, `SourceMeta`, `Args.Input`, `ENotAwaited`, `levelsAt`; not yet: `Workspace`, `OneChannel`, `Luma`,
  item links, `cv.save`).
- Checked two assumptions empirically: goja's `Function.prototype.toString` returns source with comments (ran
  `goja-cv eval` with three function forms); go-go-goja's replsession runs evals with `vm.RunString`
  (`pkg/replsession/evaluate.go:676` in the module cache), so all eval code shares one source name.
- Measured the gold radii (198 points: min 2.4, median 3.8, p90 6.3, max 64.3 px) and the current instructions length
  (3,940 characters); drafted the v2+GCV-005 instructions in the scratchpad: 3,397 bytes.
- Wrote sections 1–9 of the guide and committed them as the draft (`67fa07b`).

### Why

- The brief asks to design against GCV-004's final model; reading its deviations first avoided citing APIs that the
  implementer already changed (for example deviation 11 on cross-kind method names, deviation 6 on `Warn`).
- Before/after examples had to be the agents' real code, so I extracted them from the per-agent `.js` files rather
  than paraphrasing the hand-off.

### What worked

- The per-agent `.js` files made each issue checkable: the 28-of-32 probe loop (cfca85b4 eval 8), the faint pass that
  became `mem.faint` (2b3a8276 eval 13), the truncated row strings (c71dd0f2 trial 2), the parallel arrays
  (851dabe2), the ring probe (b54a23af).
- The existing kernels gave Part B a natural base: `minMax1D` for running min/max, `ProbeCoreRing` for the snr,
  `DistanceToSet` for the binary fast path.

### What didn't work

- A first shell command left a stray `cat > /tmp/claude-501/x.go` waiting on stdin and was moved to the background by
  the 120 s timeout; I stopped it (TaskStop) and reran the measurement without it.
- `grep --include=*.go` failed under zsh globbing (`no matches found: --include=*.go`); quoting or dropping the flag
  fixed it.
- The first `Write` of the guide failed with "File has not been read yet"; I read the stub and wrote again.

### What I learned

- The helpers' `mem.hot` and `mem.faint` are a peak finder in disguise: global threshold → candidates → snap within
  2 px by snr → rank by a local measure. Designing `peaks` as "local maxima of `measure`'s step, snr from `measure`'s
  ring" keeps one definition of every number.
- The 13 `UNUSED_NODE` warnings mostly have no line because `agentLine` drops lines beyond the current eval's length:
  helper code pasted in eval 1 has eval-1 line numbers. When the current eval is longer, the location would be wrong.
- `img.crop(b).view("speck")` already gives a local speck scale (the white point is computed over the receiver);
  nobody documented it, and `mem.sweep` reimplemented it.

### What was tricky to build

- **Sheets of points.** GCV-004 says centres without a radius are accepted only by `measure` and `sample`, but the
  agents' main pattern is a sheet of `{ cx, cy, id }` points. The guide declares sheet items `items|point[]` and
  leaves the representation of point lists to GCV-004's binder, flagged as an open question.
- **Labels through the `items` op.** GCV-004's linked entries carry only `{ index, box }`, so an edited `id` or an
  added `label` would be lost on the way to a sheet. The guide extends the entries with `id` and `label`.
- **Unawaited loads.** Making the eval wait for pending loads needs to know whether the agent handled a rejection;
  goja's promise state is enough for a warning, so the design reports pending or failed loads as a `LOAD` warning
  instead of adding promise plumbing.
- **Peaks on views.** Clipped views (speck) have flat-zero rings, so the snr hits the 1e-3 floor. The guide says so and
  recommends peaks on the image and `measure` on the view.

### What warrants a second pair of eyes

- D10 (attribution) changes the lint's semantics; check that no legitimate warning inside a library matters to agents.
- D18 (grow/shrink → dilate/erode): the anti-aliased edge is lost; check goldens and `heal`/`within` users of `grow`.
- D13/D14: default `snr` 1 with GCV-004's ring [1.5 r, 2.5 r] instead of the helpers' [2 r, 4 r]; the thresholds need
  re-deriving (§15.2).
- The table format (D9) is text inside YAML; check that nothing parses result YAML as data (the CLI's `--json` output,
  tests).

### What should be done in the future

- Finish sections 10–18 and Appendix A (step 2), fill `tasks.md`, run `docmgr doctor`.

### Code review instructions

- Read `design-doc/01-design-and-implementation-guide.md` sections 3 (evidence), 6 (API) and 8 (decisions) first.
- Spot-check citations: `pkg/cvsession/processor.go:260-316` (`unusedNodes`), `pkg/cvsession/yaml.go:167-188`,
  `pkg/cv/kernels/morph.go:14-46`, `pkg/cv/kernels/probe.go:178-195`, `pkg/cv/ops/sheet.go:113-137`.

### Technical details

- Eval mining (run with `python3 -I`): loads `evals.jsonl`, separates `helpers_pasted`, counts regexes over `code`,
  finds `UNUSED_NODE` in `result_head`.
- Instructions draft: scratchpad `instructions.txt`, `wc -c` → 3,397.
- `goja-cv eval -e 'const f = async (x, y = 2) => { // best value … }; … return { f: f.toString(), … }'` printed the
  sources with comments.
