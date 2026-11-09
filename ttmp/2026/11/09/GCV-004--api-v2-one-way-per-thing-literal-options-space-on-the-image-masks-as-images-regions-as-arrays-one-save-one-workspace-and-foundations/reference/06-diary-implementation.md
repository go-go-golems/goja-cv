---
Title: Diary - implementation
Ticket: GCV-004
Status: active
Topics:
    - api
    - architecture
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/design-doc/01-design-and-implementation-guide.md
      Note: the guide this diary implements (revision 3)
    - Path: repo://ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/scripts/04-gate.sh
      Note: the gate run before every commit (real-data tests skipped; see Step 1)
    - Path: repo://ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/scripts/05-real-data.sh
      Note: the gated real-data tests, run separately and compared with the batch-1 baseline
ExternalSources: []
Summary: "Implementation diary of GCV-004 (batch 1: phases 0b, 0c, 1, 2, 3, 4 and the Phase 0a merge): what changed per phase, the gate, goldens, real-data numbers and deviations from the guide."
LastUpdated: 2026-11-09T14:30:00-05:00
WhatFor: "Reviewing the GCV-004 implementation phase by phase and continuing it (batch 2: phases 5-8)."
WhenToUse: "Before reviewing a GCV-004 commit or starting a later phase."
---

# Diary — implementation (GCV-004)

## Goal

This diary records the implementation of GCV-004 in goja-cv, one step per phase of the guide
(`design-doc/01-design-and-implementation-guide.md`, revision 3, §13): what changed, the gate, the goldens, the
real-data numbers, what failed on the way and where the code disagreed with the guide. Batch 1 covers phases 0a
(by merge, when the v1 patch lands on `main`), 0b, 0c, 1, 2, 3 and 4; batch 2 does phases 5–8.

The batch starts at `fa115dc` (guide revision 3 and the writer's diary step 4) on `task/gcv-004-api-v2`.

## Step 1: Phase 0b — foundations without API change

Phase 0b changes nothing an agent sees. It installs the panic boundary, replaces the hand-written JSON writer by
`encoding/json` with an ordered `Record.MarshalJSON`, gives `Evaluator.Render` one exit that decides the size of
every image it returns, turns `cv.open`'s ten hidden arguments (and the placeholder's four) into typed metadata,
and makes `make smoke` read the look file from the CLI's `--json` output. No golden moved.

I started with Phase 0b rather than 0a, as instructed: `git fetch origin` showed no v1 patch on `origin/main`
(`git log origin/main --oneline | grep -i -E 'stepChroma|LOOKS|badge|ruler'` matches only old GCV-001/002 commits).
Phase 0a is done later by merging `origin/main`, before Phase 3 if the patch has landed by then.

### Prompt Context

**User prompt (verbatim):** "You are the GCV-004 implementer, batch 1: phases 0a, 0b, 0c, 1, 2, 3 and 4 of the
GCV-004 guide in goja-cv. A second batch will do phases 5–8 later. Stop after Phase 4 and report.

## Where things are
- **Repository:** `/Users/manuel/code/wesen/go-go-golems/goja-cv`, branch `task/gcv-004-api-v2` (checked out). Pull first: `git pull`.
- **Guide:** `ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/design-doc/01-design-and-implementation-guide.md`. This is revision 3, with no open questions. §13 lists the phases, §9 the deletion checklist, §14 the test strategy, §8 the decision records D1–D21 and Appendix A the review disposition. Read the whole guide first.
- **Other docs in the same ticket:**
  - `reference/01` (evidence)
  - `reference/04` (review findings)
  - `reference/02` (coordination diary)
  - `reference/03` (writer diary)
  - `tasks.md` (docmgr tasks per phase)
- **Diary format:** `~/.codex/skills/diary/SKILL.md`. Create your diary with `docmgr doc add --ticket GCV-004 --doc-type reference --title "Diary - implementation"`. Write one step per phase. Every step has these sections: Prompt Context, What I did, Why, What worked, What didn't work, What I learned, What was tricky to build, What warrants a second pair of eyes, What should be done in the future, Code review instructions, Technical details.

## Phase 0a and the parallel v1 patch
Another agent is fixing the same two rendering bugs on `main` right now: the sheet badge and the colliding ruler labels, plus the stepChroma gate and a `LOOKS` warning. It works on branch `task/v1-field-fixes` in a separate worktree. Do not touch that branch or that worktree.

Follow the guide's Phase 0a step 3:
- Start with Phase 0b.
- Before you start Phase 3, the phase that moves goldens, run `git fetch origin`.
- If `origin/main` contains the v1 patch (`git log origin/main --oneline | grep -i -E 'stepChroma|LOOKS|badge|ruler'`, or `git merge-base --is-ancestor origin/task/v1-field-fixes origin/main`), merge `origin/main` into your branch. Use a merge commit, never a rebase. Resolve any conflicts, and that merge completes Phase 0a.
- If the patch is not on main yet, carry on and do the merge whenever it appears. The coordinator may also message you when it lands.
- Do not reimplement those two fixes yourself unless the coordinator tells you to.

## Rules
- **Alpha internal software:** no backward compatibility, no wrappers, aliases, shims or migration diagnostics. Delete what the guide says to delete, and anything else the new model makes dead. Keep the core streamlined but composable.
- **Gate before every commit and push:** gofmt clean, `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, `make lint` and `make logcopter-check`. Check `go test`'s own exit status (`…; echo gate=$?`). Never let `| tail` or `| grep` hide a failure; previous implementers slipped this way.
- **Commits:** after each phase, a focused code commit (or a few), then a diary commit. Push with a plain `git push`; never force-push, merge PRs or push tags. End commit messages with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Goldens:** run the golden check before every `-update`. Only the phases the guide names may move goldens, and they may move only the named looks. List what moved in the diary.
- **Docs:** update the docs that change in each phase (`cv.docs` topics, `pkg/doc` help pages, README, MCP instructions, examples) as the guide's §12 assigns them. Update existing pages rather than adding new ones.
- **Tasks:** check off each phase's docmgr task when it is done (`docmgr task check --ticket GCV-004 --id <id>`). The ids are in `tasks.md` as `<!-- t:xxxx -->`. Check only the tasks you actually completed.
- **Deviations:** where the code disagrees with the guide, follow the code's reality. Record the deviation in your diary, and append it to a section \"Implementation notes (deviations)\" at the end of the guide.
- **Real-data tests:** gate them on the files existing, so they skip in CI. They may read `~/dustdet/cv_tiles` and `~/dustds`, but must never modify them.
- **Off limits:** `~/.claude`, `~/.codex`, `~/.pi`, MCP configs, the dust-scanning repo, and `~/go/bin`.
- **Shell heredocs:** if a shell safety check refuses a heredoc, write the content with your file-writing tool instead.

## At the end of batch 1 (after Phase 4)
Push, then report back:
- the commits per phase;
- breaking changes in JS and Go;
- what was deleted, as a line count from `git diff --stat` against the batch-1 starting commit;
- the golden outcomes;
- the deviations;
- whether Phase 0a was done by merge;
- anything batch 2 needs to know.

Do not open a PR yet; the coordinator opens one after batch 2."

**Assistant interpretation:** Implement guide phases 0b, 0c, 1, 2, 3 and 4 on `task/gcv-004-api-v2`, one commit set
and one diary step per phase, behind the full gate; do Phase 0a only by merging `origin/main` once the v1 patch is
there; stop after Phase 4, push and report.

**Inferred user intent:** Get the first, foundational half of API v2 landed safely (each phase green and
reviewable on its own), without colliding with the parallel v1 fix, so batch 2 can build regions, saving and
names on top.

**Commit (code):** `2b0955f` — "GCV-004 Phase 0b: foundations without API change"

### What I did

- **Baseline first.** `go test ./... -count=1 -race` at `fa115dc` fails in `pkg/cv/ops`: the three gated real-data
  tests (`TestRealHotListMeasures`, `TestRealTileReEncodedAsLinear`, `TestRealViewRanking`) fail on this machine
  before any change. `~/dustdet/cv_tiles` was regenerated on 2026-11-09 12:19 and `~/dustds/gold` now holds 197
  points (the guide's numbers are from 142). The gate script (`scripts/04-gate.sh`) therefore runs the race tests
  with `GOJACV_TILES=/nonexistent GOJACV_GOLD=/nonexistent` (they skip, as in CI) and checks `go test`'s own exit
  status; `scripts/05-real-data.sh` runs the real-data tests separately, and each phase compares their numbers with
  the baseline (Technical details).
- **Panic boundary** (§7.11): `graph.Recover(&err, where)` (`pkg/cv/graph/errors.go`) in the goroutines of
  `binding.evaluate` and `binding.async`, the errgroup bodies of `compute` and `decorate`, `Look.ResultContent` and
  `Data.ResultContent`; `safeRun` uses it too. Goroutines below `graph` cannot import it, so
  `parallel.Recover(&err)` (`pkg/cv/parallel/parallel.go`) returns a `*parallel.PanicError` from kernel bands and
  TIFF strips (`decode_tiff.go`), and `safeRun` maps it to `E_INTERNAL` (`graph.internalError`).
  `cvmod.Export(vm, name, fn)` (`pkg/gojamod/cv/export.go`) wraps every native function: the binding's `set`
  helper, the docs functions, `data.save` and the session's `native.object` (`pkg/cvsession/native.go`).
  `isGojaPanic` passes `goja.Value`, `*goja.Exception`, `*goja.InterruptedError` and `*goja.StackOverflowError`
  through; a compile-time reference to the two uncatchable types guards against a goja rename.
- **JSON** (§7.9): `Record.MarshalJSON` (`pkg/cv/graph/schema.go`); `dataout.plain` replaces NaN and ±Inf by nil
  and is applied by `encodeJSON` (now `json.Marshal(plain(v))`) and by CSV `cell`; `writeJSON`, `writeString` and
  `writeObject` are deleted.
- **Render** (§7.10): `Render` = `renderRequest` → `renderBranch` → `exactSize`, the one place the size is
  decided. `cutValue(v, win, s)` cuts exactly `ScaledSize(win, s)` buffer px from a buffer at scale s, starting at
  the floor of the window's corner shifted back by any overflow, and records the buffer origin; a buffer at another
  scale (a sheet at its `AnalysisScale` 1) is cut to the pixels the window touches and resized by `exactSize`.
  `fitScale` and its `+1` are deleted.
- **Typed metadata** (§7.12): `graph.SourceMeta{Path, Stamp, Info, FileEncoding}` and
  `graph.PlaceholderMeta{For, Frame, Enc, Space}` (`pkg/cv/graph/meta.go`), carried in `Args.meta`, read with
  `graph.Meta[T](a)` (panics on the wrong type), hashed after the canonical arguments; `Builder.InternSource(op,
  raw, meta)`. Deleted: the hidden `ArgSpec`s of `open` and `placeholder`, `ArgSpec.Hidden` and every check of it
  (binder, `checkKeys`, `signatureHint`, `FuncDef.UsageLine`, help, `ShortCall`, `Describe`, `checkPicture`,
  `needsArgs`, the reference, `goja-cv ops`), the `trusted` parameter of `Intern`, `Call`, `call` and `bind`, and
  `ops.frameArg`. `graphtest`'s test source uses its own `graphtest.Meta`.
- **Smoke** (§7.14): `jq -e '.status == "ok" and (.files | length) == 1'` and `test -s` on `.files[0].path`.
- **Tests:** `TestRenderExactSize` (`pkg/cv/ops/render_size_test.go`: source, pixel, geometric, decoration, macro
  and sheet × scales 0.3, 1, 4 × six windows); `TestPanicBoundary`, `TestOpenEmptyBitsPerSample`,
  `TestInterruptPassesThroughNativeFunctions`, `TestStackOverflowPassesThroughNativeFunctions`
  (`pkg/gojamod/cv/panic_test.go`); `TestEncodeJSONRoundTripsOrderAndNaN`, `TestCSVCellsUsePlain`
  (`pkg/cv/dataout/json_test.go`); `TestTimeoutInsideANativeFunction` (`pkg/cvsession/session_test.go`).
  `TestRenderScalesAndMacros` now expects the exact cut (10×5 from doc x 10) instead of the covering cut (11×5).

### Why

- Each item removes a mechanism behind one of the review's bug classes (§3.1), before the API phases build on it:
  later phases delete code freely, and a panic anywhere must not end an MCP server.

### What worked

- The gate passed first time after the changes (`gofmt=0 build=0 vet=0 test=0 lint=0 logcopter=0 smoke=0`).
- The render property test failed on the old code in 25 of its 108 cases (pixel ops, sources and macros below
  scale 1: one pixel too wide or high) and passes on the new one.
- The panic tests fail on the old code: without `Export` the first panicking `__call` aborts the test binary.
- Goldens unchanged (the golden test ran inside the gate without `-update`); real-data numbers identical to the
  baseline.

### What didn't work

- The baseline itself is red: `go test ./... -count=1 -race` at `fa115dc` gives
  `--- FAIL: TestRealHotListMeasures`, `--- FAIL: TestRealTileReEncodedAsLinear`, `--- FAIL: TestRealViewRanking`
  (`FAIL github.com/go-go-golems/goja-cv/pkg/cv/ops`). The data changed under them, not the code.
- `TestTimeoutInsideANativeFunction` in the session does not discriminate: with `isGojaPanic` reduced to
  `goja.Value` it still reports `E_TIMEOUT`, because the session derives the code from its deadline. Neither does an
  interrupt test at the module level: goja keeps the interrupt flag set until `ClearInterrupt`, so the catch block
  that would see the `E_INTERNAL` is itself interrupted. The stack-overflow test is the discriminating one: with the
  passthrough reduced it fails with `got caught E_INTERNAL, <nil>; want a StackOverflowError`.
- A `python3` heredoc that edited the `Makefile` was refused by the shell safety check (the Makefile text contains
  `rm -rf $(SMOKE_DIR)`); I made that edit with the file-editing tool.

### What I learned

- goja's interrupt is sticky: once `vm.Interrupt` is called, every later instruction raises `InterruptedError`
  until `ClearInterrupt`. Converting it to a thrown value inside a native function would only matter for the
  error object, never for whether the eval stops; the stack overflow, which is not sticky, is the case the
  passthrough really protects.

### What was tricky to build

- The exact cut needs to know whether the value it cuts is at the requested scale. A sheet is evaluated at its
  `AnalysisScale` 1 but rendered below 1; cutting `ScaledSize(win, s)` pixels from its scale-1 buffer would take the
  wrong region. `cutValue` compares the buffer with `ScaledSize(frame, s)`: equal means "at scale s, cut exactly";
  otherwise it cuts the covering pixels and `exactSize` resizes, which is the only resize left.
- `withBuffer` copies the value, including a crop's buffer origin; the cut now sets the origin explicitly (nil
  when the buffer starts at the window's corner), so a cut of a cut cannot inherit a stale origin.
- `buffer.Info.Encoding` is `json:"-"`, so hashing `SourceMeta` as JSON would not include the file's encoding;
  `SourceMeta.FileEncoding` carries it (the stamp would change with the file anyway, but the node identity should
  not depend on that argument).

### What warrants a second pair of eyes

- `cutValue`'s scale test (`Buf == ScaledSize(frame, s)`) and the origin it records; `TestRenderExactSize` covers
  the op classes but not a crop below scale 1 cut again by a look window.
- `cvmod.panicHook` is a package variable read by every native call (nil outside tests).

### What should be done in the future

- The three real-data tests need their thresholds (and the "guide" constants of `TestRealTileReEncodedAsLinear`)
  re-measured against the regenerated tiles and the 197 gold points; that is data work outside this ticket's
  phases (batch 2 re-measures the hot-list thresholds in Phase 5 anyway).

### Code review instructions

- Start with `pkg/gojamod/cv/export.go`, `pkg/cv/graph/render.go` (`Render`, `renderRequest`, `renderBranch`,
  `exactSize`, `cutValue`), `pkg/cv/graph/meta.go` and its readers in `pkg/cv/ops/source.go`, then
  `pkg/cv/dataout/data.go` (`encodeJSON`, `plain`, `cell`).
- Validate: `scripts/04-gate.sh` (expects `gate=0`); `scripts/05-real-data.sh` (numbers as in the baseline below).

### Technical details

Real-data baseline (unchanged by this phase; `05-real-data.sh`):

```
hot list: 15 tiles, 884 regions (58.9 per tile), 197 gold points: 43.7 % within 4 px of a top-30 spot (by snr), 48.2 % of any spot
labelled dust (94 matched): snr median 4.87 (middle half 2.21–7.84), stepChroma median 0.00 (0.00–0.62)
unlabelled spots (772): snr median 0.53 (middle half 0.29–1.06), stepChroma median 0.64 (0.00–1.98)
tile stats: meanL 0.13604 p01 0.00066 p99 0.84013   (TestRealTileReEncodedAsLinear: "guide" 0.33567 / 0.0085 / 0.918)
speck view: 197 points on 15 tiles, median rank 75, top50 39.6 %
77 specks, first at raw (1765.5, 2404.3)   (TestRealTileAbsoluteCoordinates: passes)
```

`git diff --stat fa115dc 2b0955f -- pkg cmd internal Makefile`: 35 files changed, 708 insertions(+), 295
deletions(-) (of which tests: render_size_test 55, panic_test 160, json_test 52 lines).
