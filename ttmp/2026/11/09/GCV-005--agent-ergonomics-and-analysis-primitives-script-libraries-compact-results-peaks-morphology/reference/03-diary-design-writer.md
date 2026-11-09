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

## Step 2: Sections 10–18, Appendix A, tasks; consistency pass

I wrote the rest of the guide (pseudocode for every new algorithm, errors, documentation with the full proposed MCP
instructions, phases, tests, validation, the helper migration note, risks, references, and Appendix A listing which
GCV-004 details are implemented), then did a consistency pass that changed two design details: peaks items carry the
full region geometry (`area`, `aspect`), and string ids are an error with a `label` fix. `tasks.md`, `index.md` and the
changelog were filled; `docmgr doctor` passed. The finished guide is `8d76e63` (2,471 lines).

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Finish the guide to the structure the brief lists, fill the ticket's tasks, verify, commit
and report the decisions.

**Inferred user intent:** A complete revision 1 ready for a review read and the reMarkable upload.

### What I did

- Wrote §10 (pseudocode: `cv.load`, failure locations, the table renderer, `DescribeFunction`, attribution and
  `collectNodes`, peaks, the chord step map, sub-pixel fit, disk and line morphology, the binary fast path, two traces),
  §11 (errors), §12 (documentation, the instructions text, the task-library and thread-width recipes, the playbook
  pattern), §13 (phases A1–A4, B1–B3, V with a dependency table), §14 (tests, including five relative gold tests),
  §15 (validation loops), §16 (helper migration), §17 (risks, open questions), §18 and Appendix A.
- Checked identifiers used in the Go sketches: `numField`, `fixedFields`, `prefixed` exist in `pkg/cv/ops`; there is no
  `intField`, so the sketch now builds the schema with `graph.WithFields(graph.GeometryFields, …)`.
- Found that `graph.GeometryFields` (`pkg/cv/graph/schema.go:32-43`) is documented as present on every region; changed
  the peaks schema to geometry + `r` + core/ring fields, in §1, §6.8, §7.8 and §10.6.
- Checked which goldens move: `sheet-2x2` uses a list of two items (`pkg/cv/look/golden_test.go:129-132`), so its labels
  change from 0/1 to 1/2 in A3; no golden uses `grow`; `regions_test.go:74` uses `toMask({ grow })`.
- Checked every markdown table for unescaped pipes with a short Python script (no findings).
- Corrected: the diary count (39, not 34), the table quoting rule, a risk that referred to an undefined "chord cache",
  the getter-safety sentence (now via property descriptors, as `renderer.property` does), string ids (now `E_ARG_TYPE`
  with the `label` fix, in §7.7, §11 and the A3 tests), the `mem.faint` signature example.
- Filled `tasks.md` (guide, review read, reMarkable upload, phases A1–A4, B1–B3, V), `index.md`, `changelog.md`; ran
  `docmgr doctor --ticket GCV-005`: all checks passed.

### Why

- Relative gold tests: the gold set changes (142 → 198 points) and GCV-004 deviation 1 shows three real-data tests went
  stale on absolute numbers.
- The phase table makes Part A and Part B independent and names what each needs from GCV-004, so either half can land
  first and A2 can land now.

### What worked

- Writing the pseudocode surfaced the details the prose had glossed over: trailing spaces in YAML literal blocks (yaml.v3
  falls back to quoted style), the tie-breaking order of peaks, the border handling of the sub-pixel fit.

### What didn't work

- `Write` on `tasks.md` failed with "File has not been read yet"; I read it and wrote again.

### What I learned

- `graph.GeometryFields` is the schema contract for regions; any new Regions op should start from it.
- The sheet golden is the only golden GCV-005 moves (A3), because its list items have no ids.

### What was tricky to build

- **The table budget.** A per-table cap would let two tables exceed the 9,000-character cap and lose their paging notes
  to the CUT note; a budget shared across tables (two thirds of the cap, at least five rows each) keeps notes intact in
  the common case.
- **Peaks' candidate count.** Every grain maximum is a candidate; non-maximum suppression with a disk of radius r (the
  morphology kernel) bounds candidates to about one per π r² pixels before the exact measure runs.

### What warrants a second pair of eyes

- Open questions 1 and 2 (§17): how GCV-004's binder represents point lists for `sheet`, and whether the `items` op's
  `id`/`label` extension goes into GCV-004 Phase 5.
- The default `snr: 1` and the ring change (D14) before the playbook is rewritten.
- The MCP instructions text (§12.1) assumes GCV-004's final v2 contract; compare with GCV-004's Phase 8 text when it
  exists.

### What should be done in the future

- Review read, revision 2, reMarkable upload (tasks).

### Code review instructions

- Start at §6 (API) and §8 (decisions), then §13 (phases) and §14.4 (gold tests).
- Verify: `docmgr doctor --ticket GCV-005`; the tables check (Python one-liner in Technical details).

### Technical details

- Table check: for each markdown table, count unescaped `|` per row and compare with the header row.
- Guide length: 2,471 lines; sections 1–18 and Appendix A; decision records D1–D21.
