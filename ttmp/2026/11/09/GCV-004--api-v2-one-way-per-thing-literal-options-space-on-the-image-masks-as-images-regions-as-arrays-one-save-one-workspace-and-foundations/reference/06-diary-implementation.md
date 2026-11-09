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

## Step 2: Phase 0c — the Args redesign (no API change)

Phase 0c removes the second representation of node lists. Every node argument is now its own input of the node
(in list order, repetitions kept), `OpDef.Inputs` is gone (its inputs are ordinary named `ArgSpec`s), and `Args`
carries the node's inputs and, at evaluation, their values, so op code reads node arguments through `Receiver`,
`Input`, `Inputs`, `Value` and `Values` and never indexes `inputs`. Every other accessor became loud. Value lifting
still works (Phase 1 deletes it) through the same input slots.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Implement guide Phase 0c (§7.2, §7.3, §13): node arguments as own inputs, named node
`ArgSpec`s, the new `Args` and its accessors, `(a Args)` build-time signatures; delete §9 item 4.

**Inferred user intent:** Make the node-list and silent-accessor bug classes unwritable before the API phases.

**Commit (code):** `ceb1ec4` — "GCV-004 Phase 0c: the Args redesign (no API change)"

### What I did

- `pkg/cv/graph/args.go`: `Args{op, vals, meta, ins, vs}`; `Receiver`, `Input`, `Inputs`, `Value`, `Values`,
  `HasInputs`; `String`, `Bool`, `Numbers`, `Strings`, `Object`, `Func`, `Lengths`, `Box` check the spec type
  (`allows`, union-aware) and the value's Go type and panic (`cv: <op>.<arg> is <T>, not <want>`), and every
  accessor panics on an optional argument without a default read without `Has` (`present`). `Node`/`Nodes` are
  deleted. The binder appends one input per node argument (`binder.inputs`; `liftIdx` deleted); a required node
  argument given an object is not taken for the options object, so plain data keeps its `E_WRONG_KIND` with the
  "plain data, not a node" hint. `resolveArgs` substitutes only lifted Value data (and re-checks it); node
  arguments stay `InputRef`s that `Value` resolves against `vs`.
- `registry.go`: `OpDef.Inputs` deleted; `DocFrame`, `Fields`, `Validate` take `(a Args)`; `Warn` takes
  `(b *Builder, a Args)`; `ResultEnc{Enc(a), Space(a, s)}`; `firstImage` deleted (`KeepEnc` reads
  `a.imageSource()`: the receiver, or for an overlay called on regions or a mask its image argument).
- `node.go`: `Node.NExplicit` deleted (`Node.HasReceiver`); `internNode` stores the inputs in the node's `Args`;
  `macroOutput` replaces three `inputs[len(inputs)-1]`. `eval.go`/`render.go`: Run receives the receiver only
  (`receiverValue`); `EvalCtx.InputNode` deleted. `macro.go`: `Expander` reads inputs from `Args`.
- Ops: `within(mask, edit)`, `blend(b, { …, mask })`, `heal(target, …)`, `inpaint(mask, …)`,
  `and/or/minus(other)`, `refine(guide, …)`, `Image:overlay(target, …)`, `Regions:overlay(image, …)`,
  `Mask:overlay(image, …)`, `regions.measure(image, …)` declare named node arguments; `cv.compare` and `cv.sheet`
  read `a.Inputs("images"/"views")`; `listInputs`, `nodeOf`, `viewNodes` and every `inputs[k]` in build-time code
  are gone (`crop`, `toLab`'s warning, `regions.measure`, `sortBy`, `where`, `sample`/`measure` points). Arithmetic
  reads its `Image|number` operand with `Raw` and a type switch whose default panics.
- Tests: `TestAccessorsAreLoud`, `TestNodeArgumentsAreOwnInputs` (`pkg/cv/graph/args_test.go`); a sheet over
  `[img, img]` views keeps two cells per item (`TestSheetLayout`); `TestOverlayAndCompare`'s `[img, img]` case
  passes unchanged. Test registries in `graph_test.go`, `m3_test.go` and `render_test.go` moved to named inputs.

### Why

- Two representations of a node list (deduplicated inputs, bound refs) caused the `cv.compare([img, img])` bug; a
  zero-value accessor caused the `encoding` bug. With one representation and loud accessors both are unwritable.

### What worked

- The regex rewrite of the build-time signatures (`func(a graph.Args, _ []*graph.Node)` → `func(a graph.Args)`)
  handled about 50 of 59 sites; the rest needed hand edits because they used the inputs.
- `TestRegistryWalk` (every registered op called and evaluated at three scales, plus help for every op, which runs
  `DocFields`) is the registry property test: it found the one accessor misuse left (arithmetic reading
  `*graph.ImageValue` from `Raw`), which showed up as `E_ARG_TYPE: sub needs an image or a number, got
  graph.InputRef` in 20 tests at once.
- Gate green; goldens and real-data numbers unchanged.

### What didn't work

- First run: `img.overlay({ items: [] })` gave `E_UNKNOWN_OPTION: overlay has no option "items"` instead of
  `E_WRONG_KIND`: with `Inputs` gone the positional loop took the object for the options object. Fixed by the
  required-node-argument rule above.
- `go vet` then failed on the three test registries that still used `Inputs:` and `NExplicit`.

### What I learned

- The registry walk is a better property test than calling build-time functions with default arguments in
  isolation: most ops need a real receiver (a mask, regions with a schema) before their `Validate` or `Fields` mean
  anything, and help covers the no-input case.

### What was tricky to build

- Help computes an op's schema without a call (`DocFields`), so `Fields` functions that read the receiver's schema
  would panic under the strict `Receiver`. `Args.HasInputs()` is false only there; `receiverFields` returns the
  geometry fields in that case.
- `resolveArgs` used to substitute every `InputRef`, so kind-typed arguments became `Value`s inside `vals` and the
  re-check in `check` accepted them. Now only lifted data is substituted (`substitute` reports whether it changed
  anything, and only changed values are re-checked), so a union such as `Image|number` keeps its `InputRef`.

### What warrants a second pair of eyes

- `a.imageSource()` still scans node arguments for an image when the receiver is not one (the Mask and Regions
  overlays and `regions.measure`); Phase 4 deletes those ops and with them the scan's purpose.
- The required-node-argument rule in `bind` (an object passed where a required node argument is positional is data,
  not options).

### What should be done in the future

- Phase 4 should reduce `imageSource` to `a.Receiver()` once the non-Image overlays are gone.

### Code review instructions

- Start with `pkg/cv/graph/args.go` (the accessor block, `binder.lift`, `bind`, `resolveArgs`/`substitute`), then
  `pkg/cv/graph/node.go` (`call`, `internNode`, `macroOutput`) and `pkg/cv/ops/sheet.go` (`runSheet`).
- Validate: `scripts/04-gate.sh`; `go test ./pkg/cv/graph -run 'TestAccessorsAreLoud|TestNodeArgumentsAreOwnInputs'`.

### Technical details

Agent-visible side effects (none of behaviour): `cv.help("heal").heal.usage` is now `heal(target, { radius,
method })` (was `heal(regions|mask, …)`, generated from `Inputs`), and explain lines name node arguments:
`within(mask: #7, edit: #11)` (was `within()`). The two tests that pinned the old strings were updated.

`git diff --stat 2b0955f ceb1ec4 -- pkg cmd internal`: 39 files changed, 656 insertions(+), 451 deletions(-).

## Step 3: Phase 1 — literal options

Phase 1 makes every option a literal value. Params, value lifting, the Value ops, derived fields, the SPACE warning
and `E_LAZY_COMPARE` are deleted; a node is accepted only for an argument of node type. A measurement that feeds an
option is awaited first, and reading a field of an unawaited Value or Regions node (or using it as a number) is the
new `E_NOT_AWAITED`, with the literal fix `(await img.stats()).p99`. The speck view keeps its white point through an
internal `levelsAt` op, bit-identical to the old lifted chain.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Implement guide Phase 1 (§6.1, §7.2, §7.4 "Views", §7.7 "Lazy data fails loudly",
§9 items 5–6) with its docs (`07-params.md` deleted; `03`, `08`, `09`; the lift error's fix).

**Inferred user intent:** Remove the machinery agents never used (params: 0 uses, lifting: 4) so every static is
computable and every accessor can be strict.

**Commit (code):** `01e5607` — "GCV-004 Phase 1: literal options"

### What I did

- Graph: `params.go` deleted; `KindParam`, the `accepts`/`Lookup`/`Methods` Param rules, `Engine.Params`,
  `DeclareParam`, `EvalCtx.Params`, the param part of signatures, `Describe`'s params argument, `ArgSpec.Static`,
  `Expander.Arg`, `spaceWarnings`/`WSpace`/`measuringOp`/`inputRefs`, `resolveArgs`' substitution and re-check
  (`resolveArgs` now only attaches the values), `NoField`, `ELazyCompare`, `EParamRedeclare`. `binder.lift` refuses a
  node for any argument not of node type with `literalError`: `E_ARG_TYPE "white must be a number, not a node:
  options are literal values."`, fix `{ white: (await img.stats()).p99 }` (option) or `(await img.stats()).p99`
  (positional). `graph.AwaitExpr` writes the awaited expression (`img.stats()` for an op on the opened image, seen
  through inserted conversions; `node.stats()` otherwise). `bind` takes `nodeOf` instead of `kindOf`.
- Ops: `value.go` (field, pick, percentile, add/sub/mul/div/min/max/abs/distance on Values) and the `param` source
  deleted; `levelsAt` (internal; `p` 0.99, `times` 5, `floor` 1e-4; the quantile with `kernels.Percentile` on a copy
  of the receiver's luminance, exactly what `stats` computes) replaces the lifted `stats → field → mul → max →
  levels` chain of `speckView`; `highpass`/`dog` pass their sigmas as literals; `isRef` and the "lifted" branches of
  `validateColor`, `validateGradient`, `validateIDs`, `where` deleted. `levels.white`'s usual range is now [0, 4].
- Binding/prelude: `__param*`, `__lazy`, `__fieldNames`, `P`, `paramProp`, `FIELDS`/`OTHER_METHODS` deleted;
  `__notAwaited(id, prop, hint)`: a field of the node's schema or any other non-method → `E_NOT_AWAITED`; an Image
  method that takes regions → `E_NOT_AWAITED` "regions are arguments of image methods", fix `img.heal(regions, …)`;
  a method of another kind (`stats().exposure`) → the old cross-kind `E_UNKNOWN_METHOD`. `Symbol.toPrimitive`/
  `valueOf` call it on every kind. `look()` marshals its options so a node there reaches the binder.
- Session: `init.js` loses `P` and its guard; the context block loses params; `Instructions` (3949 characters) teach
  awaiting a measurement; `formatAny` and `capitalize` (now unused) deleted.
- Docs: `07-params.md` deleted (topic list 12); `03` gains "Options are literal values" and `E_NOT_AWAITED` (with an
  `error=E_ARG_TYPE` and an `error=E_NOT_AWAITED` block); `08`'s table and its "Reading an error" example
  (`specks.heal()` is now `E_NOT_AWAITED`); `09`, `01`, `02`, `05`, `06`, `10`, `11`, `12` blocks that used `pick`,
  lazy fields or `d.stats().p99.mul(5)`; guidebook `02-concepts` (Params section gone), `08-recipes` ("Hand a person a
  slider" became "Explore a value"), `10-troubleshooting`, `04`, `07`, `01`; README.
- Tests: `TestNodeAsOptionIsArgType` in graph and ops (replaces `TestEncodingArgsAreStatic`); `TestDiagnostics`
  covers `img.stats().p99`, `img.stats() + 1`, `img.exposure(1).stats().p99`, `.items` on Regions,
  `specks.heal()`, and that `typeof img.detect.specks().count` is still `"function"`; `TestErrorYAMLHasCodeFixAndLine`
  covers the session YAML of both errors. Param tests were deleted or rewritten with literals
  (`TestParamMovesRecomputeOnlyDownstream`, `TestParamsRecomputeOnlyDownstream`, `TestParamsInContextAndPerson`
  deleted; `TestWithinPlaceholder`, Session B/C, `where` count tests now build a second node with another literal and
  check that only it runs).

### Why

- D1: with literal options every static is computable at build time and the review's `encoding` bug cannot be
  written; agents already await in 1139 places.

### What worked

- `sheet-2x2` (a `speck` cell) and every other golden stayed exact; `TestRealViewRanking` and the hot-list numbers are
  unchanged (the hot-list test now awaits `stats` and passes `2 × p99` as a literal).
- The registry walk and the docs tests found every remaining user of the deleted API.

### What didn't work

- The first full run failed in 11 places, e.g. `docs_test.go:91: getting-started.md line 30: … E_NOT_AWAITED … read
  .pick after awaiting it` (every docs block that used `pick`), `measuring.md line 52: unexpected warnings: RANGE line
  3: levels.white = 0.0448154 is outside the usual range [0.05, 4]` (a measured white used to be lifted and so never
  range-checked), `instructions are 4027 characters, want < 4000`.
- `img.detect.specks().feather(2)` first gave `E_NOT_AWAITED … read .feather after awaiting it`; the old cross-kind
  hint (`"feather" is a Mask method`) is better there, so methods of other kinds keep `E_UNKNOWN_METHOD`.
- `return await img.check.dust().count` used to work (a derived field that is also a Regions method name); it is now
  `E_NOT_AWAITED` with the fix `(await img.check.dust()).count`, which is the v2 rule.
- `img.look({ zoom: img.stats() })` gave `must be a number, got goja.Proxy`: look options were not marshalled.

### What I learned

- The fix expression needs the receiver as the agent wrote it: `detect.specks` is built on `convert(toGray(img))`
  (inserted by the builder), so `AwaitExpr` looks through `convert` and `toGray` to say `img.detect.specks()`.

### What was tricky to build

- Keeping the speck view bit-identical: the old chain evaluated `stats` (no mask: `kernels.Stats`, whose `L.P99` is
  `Percentile` on a copy of `Buf.Luminance()`), multiplied by 5 in float64 and took `max(·, 1e-4)`. `levelsAt` does
  exactly those steps on its receiver at the scale it runs; because the old lifted input was evaluated in the same
  run at the same scale as `levels`' receiver, the numbers match at every render scale.
- `E_NOT_AWAITED` versus `E_UNKNOWN_METHOD`: the guide says every non-method property of a lazy Value or Regions node
  is `E_NOT_AWAITED`; I kept the cross-kind error for names that are methods of another kind (and not fields of the
  node's schema), because its fix is better, and documented it as a deviation.

### What warrants a second pair of eyes

- `levels.white`'s usual range lower bound (now 0): measured whites of derived images (DoG p99 × 2 ≈ 0.045) are now
  literals and would otherwise warn in the recommended recipes.
- `binding.notAwaited`'s three cases and their order.

### What should be done in the future

- Phase 5 deletes `fields.go`'s remaining removed-field accessors (`__removedFields`, `__noField`, `__itemProto`) and
  `E_NO_FIELD` with the item prototype; Phase 1 removed only `__fieldNames`.

### Code review instructions

- Start with `pkg/cv/graph/args.go` (`binder.lift`, `literalError`, `AwaitExpr`), `pkg/gojamod/cv/binding.go`
  (`notAwaited`), `pkg/gojamod/cv/prelude.js` (`lookup`), `pkg/cv/ops/views.go` (`speckView`, `levelsAtOp`).
- Validate: `scripts/04-gate.sh`; `scripts/05-real-data.sh` (identical to the baseline); `go test ./pkg/cv/look -run
  TestGoldenLooks` (exact, no `-update`).

### Technical details

Goldens: none moved (checked without `-update`). Real data: identical to Step 1.

`git diff --stat ceb1ec4 01e5607 -- pkg cmd internal README.md`: 85 files changed, 658 insertions(+), 1550
deletions(-).

## Step 4: Phase 2 — one way to evaluate, show and explain

Phase 2 leaves one spelling for each: `await` evaluates (data at scale 1; an awaited Image is its static summary
and computes nothing), `look()` shows (every look created in an eval is delivered, up to four, and a `LOOKS` warning
names the others), and `cv.explain(node)` describes. `cv.evaluate`, the preview scale, `show`, `cv.describe`, the
`explain` op, `cv.isNode`, `cv.warnings`, `cv.kinds` and the `doc` stub are gone.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Implement guide Phase 2 (§6.5, §7.7, D8, D9, D18; §9 item 7) with docs `01`, `03`,
`06` and the instructions. The v1 patch is not on `origin/main` yet, so the `LOOKS` warning is this branch's own.

**Inferred user intent:** Remove the second ways of doing things that confused agents (85 `cv.evaluate` calls, 87
`show` calls, looks lost silently) and make data exact.

**Commit (code):** `e73cfd1` — "GCV-004 Phase 2: one way to evaluate, show and explain"

### What I did

- Graph: `EngineOptions.PreviewLong` and `Engine.PreviewScale` deleted; `Engine.Evaluate` with scale 0 means 1;
  `Engine.Explain(id)` renders at scale 1; `OpDef.NoCache`, `graph.Kinds` (replaced by `Registry.Receivers()` and a
  private display order) and `FuncDef.Global` deleted; `graph.ImageSummary(n)` builds the awaited summary from
  statics and source metadata (`ImageValue.Summary` deleted).
- Ops: `meta.go` (the per-kind `explain` op) deleted; `funcs.go` loses `evaluate`, `describe`, `isNode`, `warnings`,
  `show` and gains the bespoke `explain`; `look`'s doc says looks are queued.
- Binding: `__evaluate(id)` resolves Image/Mask promises on the owner goroutine from `ImageSummary`; Values and
  Regions are evaluated at scale 1 (`scaleArg` gone). `__explain` replaces `__describe` (adds `graph`, `encoding`,
  `space`; marks the node used). `__kinds` and `__warnings` deleted. Looks are queued in the eval tracker
  (`QueuedLook` with the call site; `EvalTrace.Looks`); a look created while no eval is tracked goes to
  `binding.late` and comes back as `EvalTrace.Late` at the next `EndEval`. `look.Look` gains `Key()` (node + options)
  and `AgentLabel()`.
- Session: `collect(fromValue, trace)` orders returned looks first, then creation order; folds equal keys (the
  duplicate gets the delivered look's number); keeps `MaxLooks`; returns the `LOOKS` warnings
  (`LOOKS line 4: 4 of 6 looks delivered; not delivered: line 4 look("4"), line 5 look("5"). Return one cv.sheet of
  the crops, or look in two evals.`) and the late-look warning. `state.prelude` (an `atomic.Bool` set by
  `runPreludes`) makes `collect` discard everything. `show`, `native.queue`, `state.shows`, the `shown:` block, the
  `doc` proxy, `native.noDoc`, `ENoDocument` deleted; `init.js` no longer uses `cv.isNode`; `show`, `doc` and `P`
  left the reserved-name list. UNUSED_NODE says "look at it" instead of "show() it".
- xgoja: config `previewLong` and its schema entry deleted.
- Docs: `06-looks.md` has a "Delivery" section (with a `looks=4 warns=LOOKS` block) instead of `show()`; `02`'s scale
  table (await = scale 1, an Image = its summary); `03`'s evaluation list and `cv.explain`; `01`'s globals; `08` (no
  `E_NO_DOCUMENT`); every `.explain()` became `cv.explain(x).graph` and every `show(...)` a `.look(...)`; guidebook
  `02`, `04`, `08`, `09`, `10`; README; `cmd/goja-cv/root.go`'s help; MCP `Instructions` and `EvalToolDescription`.
- Tests: `TestAwaitIsScaleOneAndImagesAreStatic` (a 2000 × 30 TIFF: `await` of an Image chain leaves
  `TotalRuns() == 0` and its summary is the statics; `stats().w == 2000` at scale 1), `TestPart2Section3…` checks
  every `cv.explain` field, `TestLookQueue` (unreturned look delivered, order, equal looks once, six looks → four
  images and the `LOOKS` line, prelude looks discarded, a late look named by the next eval), `cv.evaluate` is an
  unknown `cv` member (`TestUnknownNamesOnCv`), `TestLookBeforeThrowStillDeliversTheImage`.

### Why

- D8/D9/D18: agents asked for `"full"` in 77 of 79 `cv.evaluate` calls; bare `x.look()` statements were dropped
  silently in v1; an evaluating `await img` would compute a 53-megapixel scan to report its size.

### What worked

- Gate green; goldens unchanged; real-data numbers identical.

### What didn't work

- The first test run: `docs_test.go:91: looks.md line 66: want warning LOOKS` — the docs test wants warnings in the
  `CODE line N:` form, so the `LOOKS` warning is tagged with the line of the first look it drops.
- `TestDocsCodeBlocksRun/nodes-and-evaluation: unexpected warnings: UNUSED_NODE … const b = a.gaussian(2)`:
  `cv.explain(b)` did not count as using `b`; `__explain` now marks the node used.
- `looks is 6476 characters; keep topics short`: the Delivery section pushed the topic over 6000; I tightened the
  topic's prose (6000 − 1 characters of margin is not much; the Phase 8 sweep may want to split it).
- The late-look test: `setTimeout` is not a session global (`ReferenceError: setTimeout is not defined`), and
  `img.stats().then(f)` never ran `f`: the evaluation's context is cancelled when the eval ends, so the promise
  rejects. `.then(f, f)` runs after the eval returned and creates the late look.
- `TestSessionB`'s heal-run count: `await scan.heal(keep0)` no longer computes anything (an Image await), so the test
  measures the heal instead.

### What I learned

- Promise continuations from an eval run on the owner goroutine between evals (the runtime's event loop keeps
  running), so a late look is recorded while no eval is tracked; the tracker's `active` flag is enough to tell.

### What was tricky to build

- Duplicate looks: two equal looks are one delivered image, but the returned one must still show a numbered
  placeholder; `collect` gives the duplicate the delivered look's number (`AssignNumber`) instead of rendering it.
- Awaited summaries without a channel count: the guide's summary has `channels`, but the static arrives in Phase 4.
  Until then the summary has no `channels` (an opened file's `fileChannels` stays); tests that read `channels` from an
  awaited image were changed to `fileChannels` or other fields.

### What warrants a second pair of eyes

- `collect`'s order and budget with data items (still counted separately until Phase 6) and the alias numbering.
- The late-look warning's wording: it cannot name a line (the site belongs to the previous eval's source).

### What should be done in the future

- Phase 4: add `channels` to `ImageSummary` and `cv.explain` once `Node.Channels` exists.
- When the v1 patch lands on `main`, its `LOOKS` warning and this one must be reconciled in the merge (the guide says
  Phase 2 reuses the v1 code and message).

### Code review instructions

- Start with `pkg/cvsession/processor.go` (`collect`, `lookName`), `pkg/gojamod/cv/evaltrack.go` (`noteLook`,
  `EndEval`), `pkg/gojamod/cv/binding.go` (`evaluate`, `explain`), `pkg/cv/graph/value.go` (`ImageSummary`).
- Validate: `scripts/04-gate.sh`; `go test ./pkg/cvsession -run TestLookQueue -count=3`.

### Technical details

Goldens: none moved. Real data: identical.

`git diff --stat 01e5607 e73cfd1 -- pkg cmd internal README.md`: 58 files changed, 478 insertions(+), 509 deletions(-).

## Step 5: Phase 0a — merging the v1 field fixes from `main`

The v1 patch (PR #5, merge commit `b42502b`) landed on `main` while Phase 3 was being planned; the coordinator asked
for the merge at the next phase boundary. I merged `origin/main` into the branch with a merge commit (no rebase);
that merge is Phase 0a. It brings the sheet badge fix, the ruler label gap, the `stepChroma` gate (null when
|snr| < 1), the v1 `LOOKS`/`DATA` warnings and a GoSec-safe lookup in `regionmeasure.go`.

### Prompt Context

**User prompt (verbatim):** (see Step 1) and the coordinator's message: "The v1 patch has landed on main: PR #5,
merge commit b42502b. Please merge it into your branch now, as the guide's Phase 0a step 3 describes. … Phase 2
(`look()` queues and the `LOOKS` warning) should reuse the v1 warning text and code, adapted to the new queue: there
is no `show()` any more. … Move [the patch diary] into the GCV-004 ticket as `reference/` "Diary - v1 field fixes"
with `git mv`, update its frontmatter `Ticket: GCV-004`, and delete the V1-FIXES `index.md`. … If your gate runs [the
real-data tests] locally, set `GOJACV_GOLD` and `GOJACV_TILES` to an empty directory … and note this in your diary."

**Assistant interpretation:** Merge `origin/main` now (between Phase 2 and Phase 3), resolve conflicts keeping
Phase 2's queue with the v1 warning code and text, gate, record it, and move the v1 diary into this ticket.

**Inferred user intent:** One branch with both the v1 field fixes and API v2, so the goldens of Phase 0a and Phase 3
move in separate commits and batch 2 starts from the merged state.

**Commit (code):** `4b69185` — "Merge origin/main (v1 field fixes, PR #5) into task/gcv-004-api-v2: GCV-004 Phase 0a"

### What I did

- `git fetch origin && git merge --no-ff origin/main`. Two conflicts:
  - `pkg/cvsession/processor.go`: v1 kept `show()` and its `shown:` list and named dropped looks by label; Phase 2
    deleted `show` and queues every look. I kept the queue (returned looks first, creation order, equal looks once,
    late looks) and adopted the v1 code (`graph.WLooks`, `graph.WData` from v1's `errors.go`) and text, adapted to
    lines: `LOOKS line 4: 2 looks were not delivered (at most 4 per eval): line 4 look("4"), line 5 look("5"). Split
    the work over several calls, or return fewer looks (one cv.sheet of many crops).` The v1 `DATA` warning and its
    `notDelivered` placeholder helper are kept; the branch's own `WLooks` constant and v1's `lookList` are gone.
  - `pkg/cv/ops/regionmeasure.go`: v1's `stepChroma (null when |snr| < 1)` doc on the branch's named `image`
    argument, without `Static` (deleted in Phase 1).
- v1 tests that used `show()` (`TestEvalWarnsAboutLooksBeyondTheBudget`, `TestDataSaveAndBudgets`,
  `TestMCPLooksBeyondTheBudgetWarn`) now create unreturned looks and expect the line-tagged text; `TestLookQueue`
  expects the v1 text.
- The v1 diary moved: `git mv ttmp/2026/11/09/V1-FIXES/reference/01-diary-v1-field-fixes.md
  reference/07-diary-v1-field-fixes.md`, frontmatter `Ticket: GCV-004`; `V1-FIXES/index.md` deleted.

### Why

- Guide Phase 0a step 3: the merge does Phase 0a; Phase 2 reuses the v1 warning; Phase 5 keeps the v1 `stepChroma`
  rule.

### What worked

- Gate green after the resolution (`gofmt=0 build=0 vet=0 test=0 lint=0 logcopter=0 smoke=0`); the golden test
  passes with `main`'s regenerated `sheet-2x2.png`.

### What didn't work

- After the textual resolution three v1 tests failed (they call `show()`, which Phase 2 deleted), e.g.
  `main_test.go:55: exit 1` for `TestEvalWarnsAboutLooksBeyondTheBudget`; rewritten as above.

### What I learned

- The real-data tests: my gate already skips them by pointing `GOJACV_TILES` and `GOJACV_GOLD` at a directory that
  does not exist (Step 1), which is what the patch agent did with an empty directory; the coordinator confirms they
  fail on `main` too because `~/dustds/gold` changed after they were tuned. Re-tuning them is batch 2's.

### What was tricky to build

- The two `LOOKS` texts: v1 says how many looks were dropped and names them by label; Phase 2 named them by line. The
  merged text keeps v1's wording and budget, and names each as `line N look("label")`, tagged with the first line
  so the warning keeps the `CODE line N:` form the docs tests rely on.

### What warrants a second pair of eyes

- `collect` after the merge (`pkg/cvsession/processor.go`): one function now carries the queue, the dedup, the v1
  `LOOKS` and `DATA` warnings and the late-look warning.

### What should be done in the future

- Phase 5 must keep the v1 `stepChroma` rule (`kernels.StepChromaMinSNR`, NaN in region measures read back as null)
  when it rewrites `measure`.

### Code review instructions

- `git show 4b69185 --cc -- pkg/cvsession/processor.go pkg/cv/ops/regionmeasure.go`; validate with
  `scripts/04-gate.sh`.

### Technical details

Goldens moved by the merge (from `main`, not regenerated here): `sheet-2x2.png` (4752 → 4743 bytes: the badge no
longer covers cell 0's label). `ruler-crop-zoom8.png` and `grid-gray-crop-zoom4.png` did not move (the v1 ruler fix
changed no existing golden).

## Step 6: Phase 3 — space on the image

Phase 3 moves the space from the op to the image. `cv.open` returns file units (`space: "encoded"`), `img.as("linear")`
gives light, and an op works in its receiver's space and returns it. Physical ops (`exposure`, `heal`, `inpaint`,
`toLab`, `encode`, `toGray("rec709")`) compute in linear light inside a bracket the builder closes with `as`, and the
pictures (`overlay`, `annotate`, `compare`, `sheet`) draw encoded. Every luminance read (`stats`, `histogram` L,
`mask.luminance`, `refine`, `detect.specks`, `check.dust`, `levels({ auto })`, the measurements' gray plane) is one
`toGray()` input of the node. The `{ space }` option and its machinery are gone.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide §13 Phase 3: tests first (§14 Phase 3 list), then the `SpaceRule` enum and the
resolution of §7.4, the encoded source and `openLinear`, `as` with folding, brackets, `OpDef.Luma`, `colormap`
following, views; regenerate exactly the four named goldens; docs 10, 11, 12, recipes and the op docs.

**Inferred user intent:** Numbers measured on an image are thresholds for ops on that image; no op silently changes
the scale of its result; nothing extra is copied on a 0.6 GB scan.

**Commit (code):** `ae53037` — "GCV-004 Phase 3: space on the image"

### What I did

- `pkg/cv/graph`: `SpaceRule` (`Follows`, `Physical`, `Display`) with `OpDef.SpaceOf` for `toGray`; `workSpace`;
  `Builder.as` with three folds (identity; `as(as(x, s1), s0) = x`; `as(open, linear) = openLinear`;
  `as(openLinear, encoded) = open`); `resolveSpace` adds the luminance input and converts image inputs to the work
  space; `closeBracket` after `internNode`; `Substitute` folds `as` like the builder. `OpDef.Gray`, the space option,
  `SpaceArg`, `SpaceValueSynonyms`, `Default`/`Fixed`/`For`, `presetSpace` and the `workingSpace` option synonyms are
  deleted. `Args.Luma()` reads the luminance input. `Shown`, `Label`, `Chain`, `Path` hide inserted `as`;
  `cv.explain` lists them.
- `pkg/cv/ops`: `open`'s `Run` converts the pyramid level to encoded; internal `openLinear` returns the level; public
  `as` (positional `linear|encoded`, `Identity` when the conversion is one); `Space` rules mapped (Display for the
  four pictures, Physical for exposure/heal/inpaint/toLab/encode, the rest follow); the speck and speckColor presets
  start with `as("linear")`, the other presets and `highpass`/`dog` follow; `stats`, `histogram`, masks, `detect`,
  `sample`/`measure`/`profile`, `regions.measure` read `a.Luma()`; `exposure`'s doc and RANGE hint name
  `levels({ auto: true })` and `mul(2 ** ev)` (new `ArgSpec.UsualHint`).
- `pkg/cv/kernels`: `Stats`, `StatsWeighted`, `SamplePoints` and the probe take the luminance plane explicitly
  (`luminance()` accepts a one-channel buffer without one, and refuses a three-channel buffer without one); the
  `GrayPlane` enum is gone.
- Tests: `space_test.go` rewritten for the §14 Phase 3 list (`TestOpenIsEncodedAndAsConverts`, `TestPhysicalBracket`
  with run counter and cache bytes, `TestReceiverDecides`, `TestOutputSpaceFollowsTheReceiver`, `TestOneLuminance`,
  `TestColormapRangeOnItsReceiver`, `TestEncodingWarnings`, `{ space }` → `E_UNKNOWN_OPTION`); ops, cvsession,
  gojamod and cmd tests updated for file units (see deviation 23).
- Docs: `10-encodings-and-spaces` rewritten (the space of an image, `as`, the receiver decides, physical ops and
  pictures, one luminance, `ENCODING`'s fix); `11-views`, `12-measuring`, `02-units-and-scales`, `09-recipes`,
  `04-lambdas`, `05-masks-and-regions`, `03-nodes-and-evaluation`; `pkg/doc` 01, 02, 03, 08; help `space`; the MCP
  instructions line (shortened to stay under the 4000-character cap); every op and argument doc that said "working space
  (… by default)".
- Guide deviations 19–23.

### Why

- Guide §4.2, §6.2, §7.4, D2, D3, D15, D16, D20.

### What worked

- The gate: `gofmt=0 build=0 vet=0 test=0 lint=0 logcopter=0 smoke=0 gate=0`.
- The golden check before `-update` failed on exactly the four named looks and nothing else; `sheet-2x2` (the speck
  views) stayed exact.
- Real data (`05-real-data.sh`) identical to the post-merge numbers (below): the speck view and the hot list compute
  in linear light as before.

### What didn't work

- `go test ./pkg/cv/ops`: `pkg/cv/ops/measure_real_test.go:250:61: not enough arguments in call to
  kernels.StatsWeighted` (the kernels take the luminance plane now); the benchmark then panicked with `index out of
  range [393474] with length 0` in `ProbePoint` (no gray plane) until `hotPoints` computed one.
- `TestOneNodeMacro` panicked: `cv: op testview has no arg "space"` (a test macro that forwarded the option).
- `TestPart2Section3NodesAndEvaluation`: explain had four lines, not three: `#4 as(to: "encoded")` closes the exposure
  bracket and the source is `source gradient-64x48-16bit.tif (linear)`.
- The first `10-encodings-and-spaces` draft showed `img.mask.luminance(…).stats()` (masks get `stats` in Phase 4) and
  `img.blend(ref)` of an srgb and a linear file, which warns `ENCODING`: the block now blends `img.as("linear")`.
- I had first made `speckDark` linear too; the guide (§6.2, §7.4 "Views") names only `speck` and `speckColor`, so
  `speckDark` follows its receiver as before.

### What I learned

- The post-merge real-data numbers differ from Step 1's baseline in the `stepChroma` quartiles only (labelled dust
  0.00–0.62 → 0.00–0.45, unlabelled median 0.64 → 0.00): that is the v1 `stepChroma` rule from the merge (checked on
  a worktree at `37f451f`), not Phase 3.
- A cache entry carries 64 bytes of overhead beyond its pixels, so the "no extra buffer" test bounds the bytes
  between three and four buffers instead of comparing exactly.

### What was tricky to build

- Folding in two places. The builder interns `as` through `b.as`, but lambdas rebuild their graph with `Substitute`,
  which first produced `as(as(…))` chains a hand-written call never has (`TestLambdasResolveLikeDirectCalls`). Both
  paths now go through the same `b.as`.
- The fourth fold (`as(openLinear, encoded) = open`) is what makes `img.sub(img.as("linear"))` read the opened node
  twice instead of an encoded copy of the pyramid.
- `LumaWhen`: adding a luminance input to every `levels` and `histogram` would change their hash and hold a gray
  plane nobody reads; the input is added only for `auto` and for channel `L`.

### What warrants a second pair of eyes

- `pkg/cv/graph/space.go` (`resolveSpace`, `as`, `closeBracket`) and its call sites in `node.go` (`call`,
  `intern`, `Substitute`).
- The rule table in the ops (`Space:` fields): every op that is not listed follows.
- `kernels.luminance` refusing a three-channel buffer without a plane: any new kernel caller must pass `Args.Luma()`.

### What should be done in the future

- Batch 2 (Phase 5) re-tunes the hot-list and view-ranking thresholds on the current gold labels; the tests already
  build on `img.as("linear")`.

### Code review instructions

- Start with `pkg/cv/graph/space.go`, then `registry.go` (`SpaceRule`, `Luma*`), `node.go`, `explain.go`,
  `pkg/cv/ops/source.go` and `space.go`, `views.go`, then `pkg/cv/graph/space_test.go`.
- Validate: `scripts/04-gate.sh` (`gate=0`); `scripts/05-real-data.sh`; `go test ./pkg/cv/look -run Golden`.

### Technical details

Goldens moved (regenerated after the check failed on exactly these):

| golden | bytes | why |
|---|---|---|
| `gaussian.png` | 19875 → 20175 | `img.gaussian(2)` blurs file units (it blurred light) |
| `levels-gamma.png` | 48857 → 48250 | `levels({ black: 0.02, white: 0.6, gamma: 1.4 })` maps file units |
| `levels-stats.png` | 60806 → 58589 | `levels` from `stats()` p01/p99, both in file units |
| `profile-chart.png` | 2830 → 2982 | the profile's values are file units |

Real data (`05-real-data.sh`), identical to the state after the merge:

```
hot list: 15 tiles, 884 regions (58.9 per tile), 197 gold points: 43.7 % within 4 px of a top-30 spot (by snr), 48.2 % of any spot
labelled dust (94 matched): snr median 4.87 (middle half 2.21–7.84), stepChroma median 0.00 (0.00–0.45)
unlabelled spots (772): snr median 0.53 (middle half 0.29–1.06), stepChroma median 0.00 (0.00–0.26)
tile stats: meanL 0.13604 p01 0.00066 p99 0.84013   (now of img.as("linear"))
speck view: 197 points on 15 tiles, median rank 75, top50 39.6 %
77 specks, first at raw (1765.5, 2404.3)
```

`git diff --stat 37f451f ae53037 -- pkg cmd internal Makefile`: 75 files changed, 1005 insertions(+), 836
deletions(-).

## Step 7: Phase 4 — masks are one-channel images

Phase 4 removes the Mask kind. A mask is an Image with one channel: `await m` says `channels: 1`, every image op
works on it, the mask ops are Image methods that need a one-channel receiver, and an argument typed `Mask` accepts any
one-channel image. Every Image node now carries its channel count, so channel errors (a colour image as a mask, a grey
image for `toLab`) are `E_WRONG_KIND` when the call is built, on the agent's line, with a literal fix.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Guide §13 Phase 4 with §6.3, §7.5 and §9 item 9: `Node.Channels`, `OpDef.Channels`,
`OneChannel`, `ThreeChannel`, the `Mask` type word, the mask exemption in `resolveSpace`,
`PlaceholderMeta.Channels`; delete the Mask kind and its duplicates (one `crop`, one `overlay`); the Phase 4 tests;
mask goldens exact; docs.

**Inferred user intent:** One kind for pixels, so agents can measure and transform masks like images and get channel
mistakes at build time instead of at evaluation.

**Commit (code):** `cdc6326` — "GCV-004 Phase 4: masks are one-channel images"

### What I did

- `pkg/cv/graph`: `Node.Channels` (`nodeChannels`: a macro's output, `op.Channels`, else the receiver's or the image
  argument's); `graph.One`, `graph.Three`, `graph.MaxChannels` (the larger count of the image inputs, as
  `kernels.binary` broadcasts); `MaskType`, `acceptsNode`, `takesMask`; `wrongNode` (the argument error, fix
  `img.toGray()`) and `checkChannels` (the receiver error) replace `kindHint`; `maskInputs` marks inputs read as
  stored (Mask-typed arguments, `OneChannel` receivers, the luminance input) for `resolveSpace` and the `ENCODING`
  warning; a one-channel source is its own luminance (no `toGray` node); `PlaceholderMeta.Channels`;
  `ImageSummary` and `cv.explain` report `channels`.
- Deleted: `KindMask`, `ImageValue.kind`, `NewMask`, `IsMask`, `kindHint`, the Mask branches of `conversionHint`,
  `crossKind`, `receiverVar`, `argFix`, render, explain, order warnings, looks, save, data, help, the session's
  reference and `goja-cv ops`; the second `crop`; the Mask `overlay` (`mask.go`) and the Regions `overlay`
  (`present.go`); the `mask()` helper (`image()` does it).
- `pkg/cv/ops`: mask makers and `toMask` return `DerivedEnc` one-channel images; mask ops, `coverage` and
  `components` are Image methods with `OneChannel`; `colormap` is `OneChannel` (its run-time check is gone);
  `toLab`, `split`, `chroma`, `mask.color` are `ThreeChannel`; `channel`'s `Validate` refuses `"g"`/`"b"` on a grey
  receiver; `Channels` declared on `toGray`, `channel`, `chroma` (1), `colormap`, `toLab`, `overlay`, `annotate`,
  `compare`, `sheet` (3), `add`/`sub`/`mul`/`min`/`max`, `blend`, `within` (`MaxChannels`), the sources.
- Looks: a one-channel linear-encoded image is PNG (`isMaskLike`).
- Help: the `mask` concept; `cv.help("Mask")` resolves to it; `mask.stats` redirect rewritten.
- Tests: `TestRegistryWalk` became the channel property test (colour, grey and mask receivers; colour and grey
  argument passes; `Buf.C == Node.Channels`; mask values in [0, 1]); `TestMasksAreOneChannelImages`
  (`img.toGray().grow(2)`, `m.stats()`, `img.within(img, …)` and `img.within(m.mul(img), …)` refused with
  `img.toGray()`, same mask node under `img` and `img.as("linear")`, no `toGray` for a grey input); colour ops on grey
  and `colormap` on colour at build time (`channel_test.go`); the agent's line in the `within` error
  (`TestCrossKindErrorsHaveALiteralFixAndGrammar`); `m3_test.go` in graph with a colour test op.
- Docs: `05-masks-and-regions` (the mask sections; regions stay for Phase 5), `02-concepts` (kinds), `06-looks`,
  `08-errors`, `01-getting-started`, `02-units-and-scales`, `13-data-out`, `pkg/doc` 03, 04, 10, README, the MCP
  instructions (4000-character cap kept by shortening the masks line).
- Guide deviations 24–28.

### Why

- Guide §6.3, §7.5, D16, §9 item 9, review findings F1, F2, F28.

### What worked

- The gate: `gofmt=0 build=0 vet=0 test=0 lint=0 logcopter=0 smoke=0 gate=0`.
- Goldens: none moved (`mask-overlay` is now built as `img.overlay(mask)` and renders the same bytes).
- The property test passed on its first full run once the receivers were chosen by `OneChannel`/`ThreeChannel`:
  every op's evaluated channel count matched its static.
- Real data identical to Step 6.

### What didn't work

- `TestGoldenLooks/mask-refine`: `look differs from its golden: largest difference 49 codes, 6233 of 49152 samples
  differ (12.681 %)`. The mask receiver is linear (derived) and `refine` follows it, so the guide's encoded
  luminance input was converted to linear. The luminance input is now read as stored (deviation 24).
- `TestGoldenLooks/mask-overlay`: `overlay: E_WRONG_KIND: overlay.target needs a one-channel image (a mask) or
  regions; this image has 3 channels.` — the golden called the deleted `mask.overlay(img)`; rebuilt as
  `img.overlay(mask)`.
- The first walk failed on `Image:channel on 1-channel #4: E_ARG_RANGE: channel(g)`: the new build-time check on a
  grey receiver; the sample became `"r"`.
- The MCP instructions grew to 4061, then 4009 characters; the masks line was shortened until under 4000.

### What I learned

- `MaxChannels` over all image inputs is safe for masks and the luminance input: both have one channel and the
  receiver has at least one, so they never raise the maximum.
- With `Channels` static, `colormap`'s and `channel`'s run-time channel errors became dead code and could go.

### What was tricky to build

- Mask exemptions by position: `resolveSpace` works on the input list, so `maskInputs` maps the Mask-typed argument
  specs to their input indices through `Args.refs`; the luminance input is the one appended after `args.ins`.
- Keeping `imageSource` (the encoding a result keeps) blind to Mask-typed arguments: `accepts` stays a kind check
  (`"Mask"` is not a kind), and only `acceptsNode` knows that a mask is a one-channel Image.

### What warrants a second pair of eyes

- `pkg/cv/graph/space.go` (`maskInputs`, the luminance exemption, the grey-input rule) and `args.go`
  (`wrongNode`, `checkChannels`).
- The `isMaskLike` PNG rule in `look.go`: derived one-channel looks are now lossless too (deviation 25).

### What should be done in the future

- Phase 5 rewrites the regions half of `05-masks-and-regions` and adds the `regions` help concept.
- The GCV-001 golden tolerance classes and `-golden.tolerant` flag could go (§14.3 says they are no longer needed);
  not done in batch 1.

### Code review instructions

- Start with `pkg/cv/graph/node.go` (`nodeChannels`, `MaxChannels`, `checkChannels` call), `registry.go`
  (`MaskType`, `acceptsNode`, `OpDef.Channels`/`OneChannel`/`ThreeChannel`), `args.go`, `space.go`; then
  `pkg/cv/ops/mask.go`, `channel.go`, `present.go`, `regions.go`; then `ops_test.go` (`TestRegistryWalk`) and
  `mask_test.go`.
- Validate: `scripts/04-gate.sh`; `go test ./pkg/cv/ops -run 'TestRegistryWalk|TestMasksAreOneChannelImages' -v`.

### Technical details

Goldens: none moved. Real data (`05-real-data.sh`): identical to Step 6.

`git diff --stat 7554bd1 cdc6326 -- pkg cmd internal Makefile README.md`: 60 files changed, 514 insertions(+), 333
deletions(-).

Batch 1 total, `git diff --stat fa115dc cdc6326 -- pkg cmd internal Makefile`: 146 files changed, 4238
insertions(+), 3768 deletions(-).

## Step 8: Phase 5 — regions are arrays

Batch 2 starts here, at `7ffe54b` (batch 1 verified by the coordinator). Phase 5 turns region sets into plain
JavaScript arrays. An awaited Regions node is an array of flat items, each carrying a hidden `[node, index]` link;
every op that takes regions takes a Regions node or an array (linked items, literal boxes and circles, and for
`measure`, `sample` and `cv.sheet` points), so JavaScript does the querying and the query ops, data lambdas and
their machinery are gone. One `img.measure(items)` replaces both measures, with radii from each item;
`img.mask.regions(items)` replaces `toMask`. The three gated real-data tests are re-tuned, and the speck-ranking drop
turned out to be a change of the tile files, not of labels or code.

### Prompt Context

**User prompt (verbatim):** "You are the GCV-004 implementer, batch 2: phases 5, 6, 7 and 8 of the GCV-004 guide in
goja-cv, then the pi validation and the PR. […] Phase 5: Re-tune the three real-data tests (`TestRealHotListMeasures`,
`TestRealTileReEncodedAsLinear`, `TestRealViewRanking`) against the current gold labels in `~/dustds/gold` […] Make them
robust to label drift. Compare methods or check relative properties instead of asserting absolute counts […]
Investigate why the speck view's ranking is now median rank 75 with 39.6 % of gold points in the top 50 […] Verify
that by checking out `b847520` in a temporary worktree […] Keep the v1 stepChroma rule […] Delete `fields.go` […]
Rewrite the regions half of `05-masks-and-regions.md` and add a `regions` help concept. […] Additions from GCV-005
that are cheap here: Item `label` field […] `sheet` takes points […]" (the full prompt is in the batch-2 task; the
rules on gate, commits, goldens, docs, tasks and deviations are those of batch 1.)

Coordinator messages during the phase (verbatim): "1. **One binding for points.** Use one `items|point[]` binding
for `measure`, `sample` and `cv.sheet`, with a `Point` item shape for a centre without a radius (`[x, y]` or
`{ cx, cy }`). Delete the sheet's separate list path, so all three ops accept the same inputs. 2. **Ids may
repeat.** Items may carry a `label` string, and labels print `label`, else `id`. A non-numeric `id` is
`E_ARG_TYPE`, with a fix that moves the text to `label`. Ids may repeat: say so in the help text, and never use them
for identity, which stays the `[node, index]` link." and "1. **Phase 5: don't add `mask.regions`' `grow` option.**
GCV-005 replaces mask `grow`/`shrink` with `dilate`/`erode` on one-channel images, so a `grow` option would only be
deleted again. If you already added it, remove it and record the removal as a deviation. 2. **Phase 6: track
`cv.save` promises.** […]"

**Assistant interpretation:** Implement guide §13 Phase 5 (§6.4, §7.6, D5–D7, D17, D19, §9 item 10) with the
GCV-005 additions (label, Point, one points binding, repeatable ids, no `grow`), re-tune the real-data tests to
relative checks, explain the ranking drop with a controlled comparison, and update docs 04, 05, 09, 12 (and every
other block the change breaks).

**Inferred user intent:** Agents filter regions with the JavaScript they already know, the colleague's
detector-plus-gold workflow becomes one array, and the real-data tests stop breaking whenever the labels change.

**Commit (code):** `449aaed` — "GCV-004 Phase 5: regions as arrays"

### What I did

- **Items** (`pkg/cv/graph/items.go`, new): the type words `Items`, `Items|Mask`, `Items|point[]`; `binder.checkItems`
  takes a Regions node (an input), a one-channel image for `Items|Mask`, or an array, which `parseItems` turns into
  canonical entries — linked `{ index, x, y, w, h, id?, label? }` from `__cvref: [node, index]`, literal
  `{ shape: box|disk|point, geometry, id?, label? }` — and interns through a new `bind` callback
  (`Builder.itemsNode`) as the internal `items` op (`graph.ItemsOpDef`, registered by `ops`; `AnalysisScale` 1;
  frame of its source or empty). Two region sets → `E_ARG_TYPE` with the op's fix (`twoSetsFix`); unlinked objects
  with `id` and `area` → `ITEMS` warning; a non-number `id` → `E_ARG_TYPE` with the `label` fix. `runItems` takes
  linked regions by position and refuses a moved box (compared with the region's box rounded to 0.01 doc px); literal
  items are `Box`, `Disk` and `Point` regions. `stripRefs` drops `__cvref` from every non-items argument (an item
  passed as a box, a point, options or marks is its fields).
- **Regions** (`pkg/cv/graph/regions.go`): `Region.Shape` (`Pixels`, `Box`, `Disk`, `Point`), `R`, `Label`,
  `Text()` (label, else id); `overlap` draws a box with exact area coverage and a disk with
  `kernels.EllipseCoverage` (so a literal box equals `mask.box`, a circle `mask.ellipse`); `Records()` replaces
  `Summary()` (an array, `E_TOO_MANY` with a v2 fix); `Data` keeps measures exact (deviation 34) and rounds geometry
  and `r`.
- **Ops:** `measure` on Image (`pkg/cv/ops/regionmeasure.go`, rewritten): `items` (`Items|point[]`), `ring` (default:
  on unless `stats`), `stats`, `coreRadius`, `ringRadii`, `channels`, `prefix`, `gray`, `analysisScale`; core radius
  from `coreRadius`, the item's `r`, else √(area / π) ≥ 1; ring [1.5 r, 2.5 r]; a point without `coreRadius` is
  `E_ARG_TYPE` with `img.measure(points, { coreRadius: 3 })`; items outside the frame get null ring fields and an
  `OUTSIDE` warning; the v1 `stepChroma` gate is kept. The point `measure` (Value rows) is gone; `sample` takes
  `Items|point[]` and samples centres; `mask.regions(items)` replaces `toMask` (no `grow`); overlay, heal and
  `stats({ mask })` take `Items|Mask`; `cv.sheet` takes `Items|point[]` (its list path `listItems`/`sheetItem` is
  deleted; the bound comes from `graph.ItemCount`); overlay and sheet print `Region.Text()`.
- **Deleted** (§9 item 10): `where`, `sortBy`, `top`, `only`, `without`, `count`, `filter`, `toMask`,
  `validatePredicate`, `compare`, `matches`, `idSet`, `idFilterOp`, `truthy`, `rasterize`, `OpDef.Produces`,
  `producerHint`, `UnknownFieldError`, `RemovedFields`, `RemovedFieldError`, `Error.field`, `ENoField`,
  `speckProduces`, `regionMeasureProduces`, `statSynonyms`, `OpDef.MaxItems`, `FuncID`, `FuncArg`, `Args.Func`, the
  `func` type, `Callbacks`, `Evaluator.CallEach`/`SetCallbacks`, `binding.funcJS`/`CallEach`, `pkg/gojamod/cv/fields.go`
  (`__removedFields`, `__noField`, `__itemProto`), the prelude's `NODE`, `link`, `ITEM_PROTO`, `ARRAY_METHODS`,
  `funcIds`, `__cvfunc`, `__notArray`/`ENotArray`, `__collision` and `EvalTrace.Collisions`, `jsError`, the
  `Member`/`RECV` and collision branches of `explainDataMisuse`, the Regions branch of `kindHelp`.
- **JavaScript:** `linkItems` sets `it[ITEM] = [id, i]` (an enumerable symbol); `marshal` sends a linked item as
  `{ __cvref, …fields }`; `resultToJS` returns the records array. Regions have no method table, so every property of
  a lazy Regions node is `E_NOT_AWAITED` (an image method that takes items says "regions are arguments").
- **Help:** a `regions` concept; `Mask`/`Regions` are concepts (`helpKinds` is Image, Value); `Mask.x` and
  `Regions.x` answer `found: false` with the Image method and the concept (`conceptMethod`); the `id` field doc says
  ids may repeat.
- **Docs:** `05-masks-and-regions` regions half rewritten ("Regions are arrays", "Literal items", "Converting");
  `03` ("Awaited regions are arrays", kinds table), `04` (builder lambdas only; data is JavaScript), `12` (one
  `measure`, radii, gold `r / 2`, measuring regions, the re-measured hot-list numbers), `09`, `13`, `02`, `06`, `08`,
  `01`; guidebook `01`, `02`, `03`, `04`, `08`, `10`; README; MCP instructions (one Regions line, still under 4000).
- **Real data** (`measure_real_test.go`, `viewrank_test.go`, `realdata_test.go`): see Technical details.

### Why

- Guide §6.4, §7.6, D5–D7, D17, D19; the GCV-005 additions keep GCV-005 from changing the item model again.

### What worked

- The gate: `gofmt=0 build=0 vet=0 test=0 lint=0 logcopter=0 smoke=0 gate=0`.
- Goldens: the check ran without `-update` and passed. `specks-overlay` is exact; `sheet-2x2` is exact with
  `labels: "index"` in its call (deviation 35). No golden was regenerated.
- goja copies enumerable symbol keys in spread and hides them from `Object.keys` and `JSON.stringify`
  (`TestMasksAndRegionsFromJS` asserts `7:1:{"a":1}`, D6), and renumbered, labelled, reordered items keep their own
  pixels (`measuredIds: 1:r0:16,2:r1:80`).
- `overlay(array)` equals `overlay(node)` byte for byte; literal boxes and circles equal `mask.box`/`mask.ellipse`
  at scales 1 and 0.5.

### What didn't work

- The ranking investigation's first assumption (labels) was wrong; see Technical details.
- D17's prediction failed on the gold labels: `circles sized from r: median snr 0.86, not above the fixed 3 px core's
  1.17` (deviation 38).
- `TestMeasureMatchesTheHelper` failed after the switch to Regions: `ok: false d: [4.6269e-05, 4.976e-05, 3.1856e-05,
  4.8805e-05, 0.0076872]` — region measures were rounded to 4 decimals; they are exact now (deviation 34).
- `TestGoldenLooks/sheet-2x2: largest difference 238 codes, 99 of 113472 samples differ`: literal points are labelled
  by id (1, 2) instead of index (0, 1); the golden call passes `labels: "index"`.
- `TestSessionB`: `heal nodes: 1` — two filtered arrays with the same items are one hash-consed items node, so the
  "second threshold" heal was the same node; the test now heals with another radius.
- Topic size: `masks-and-regions is 6377 characters`, then `looks is 6103`, `recipes is 6054`; tightened prose.
- `make lint`: `func sortedNames is unused`, `rasterize`, `jsError`, `median`; deleted.

### What I learned

- The tile JSON now says `"view": "density, dust bright, code value = v: open with encoding 'linear'"`: the
  colleague's export changed under the tests in DUST-002 `d54088c` ("tiles re-exported as linear density").
- Gold `r` from dustview is the fit radius (2 × the half-level radius, per DUST-002 `cc70685`), not the speck radius.

### What was tricky to build

- The binder had to intern a node in the middle of binding. Passing `Builder.itemsNode` into `bind` keeps the
  binder free of the builder; `funcs.go`'s bespoke functions pass nil (arrays refused there).
- Frames: the evaluator stamps a Regions value with its node's frame, which is also the raster of the regions'
  pixels. `measure` therefore takes its items' frame (falling back to the receiver's for literal items), not the
  image's, or a linked region's pixels would be read in the wrong raster.
- Identity versus data: linked entries carry `id` and `label` (so a renumbering shows up in labels and results), but
  never pixels or box; `index` is the identity.

### What warrants a second pair of eyes

- `pkg/cv/graph/items.go` (`parseItems`, `runItems`'s moved check at 1e-6 after rounding to 0.01).
- `stripRefs` in `bind`: every non-Items argument is copied without `__cvref`.
- `itemsCover` (containment instead of same frame for heal, stats and measure, deviation 33).

### What should be done in the future

- D17 and the gold `r`: decide whether `{ cx, cy, r }` means the speck radius (as now) or whether the DUST-002
  helpers should pass `r / 2`; the measuring topic documents `r / 2` for dustview labels.
- Phase 6 replaces the `cv.json`/`cv.csv` tests touched here.

### Code review instructions

- Start with `pkg/cv/graph/items.go`, then `args.go` (`check` → `checkItems`, `bind`'s `stripRefs`),
  `regions.go` (`Shape`, `overlap`, `Records`), `pkg/cv/ops/regionmeasure.go`, `regions.go`, `sheet.go`,
  `pkg/gojamod/cv/prelude.js` (`linkItems`, `marshal`), `help.go` (`conceptMethod`).
- Validate: `scripts/04-gate.sh`; `go test ./pkg/cv/ops -run 'TestItems|TestMeasure' -v`;
  `go test ./pkg/gojamod/cv -run TestMasksAndRegionsFromJS`; `scripts/05-real-data.sh`.

### Technical details

**The speck-ranking drop.** On the same current tiles and labels, `b847520` (a temporary worktree in the scratch
area) prints exactly the batch-1 numbers: `speck view: 197 points on 15 tiles, median rank 75, top50 39.6 %` and the
same hot-list and tile-stats lines. So no code regression. The tiles changed: since DUST-002 `d54088c` they store
density v as the code value (open with `encoding: "linear"`), where they used to store sRGB_encode(v); the tests
opened them as sRGB, so `as("linear")` decoded density a second time. A split by encoding and by label change:

| open as | all 197 points | 111 points, 13 tiles with unchanged labels | 86 points, 2 relabelled tiles (2026-11-09) |
|---|---|---|---|
| srgb (as the old tests) | median 75, top50 39.6 % | 57, 43.2 % | 209, 34.9 % |
| linear (as the tile JSON says) | median 24, top50 78.2 % | 22, 74.8 % | 26, 82.6 % |

With the right encoding the numbers are back near the GCV-002 18 / 79.6 % (on 142 points), and the re-encoded tile's
stats are exactly the old "guide" constants (meanL 0.33567, p01 0.00854, p99 0.91794). The labels moved the
numbers only a little.

**Re-tuned tests** (relative assertions; numbers logged):

```
speck view: 197 points on 15 tiles, median rank 24, top50 78.2 %      (must beat levels({ auto: true }))
levels view: 197 points on 15 tiles, median rank 1467, top50 1.0 %
hot list: 15 tiles, 555 regions (37.0 per tile), 197 gold points: 60.4 % within 4 px of a top-30 spot by snr, 43.1 % by area, 73.6 % of any spot
labelled dust (142 matched): snr median 2.23 (middle half 1.75–2.81), stepChroma median 0.00 (0.00–0.57)
unlabelled spots (386): snr median 0.76 (middle half 0.51–1.06), stepChroma median 0.00 (0.00–0.36)
gold dust (190 points): snr median 1.56 (middle half 0.92–2.35) with r / 2, 0.86 (0.58–1.45) with the label's r, 1.17 (0.79–1.84) with a 3 px core
tile stats: meanL 0.33567 p01 0.00854 p99 0.91794   (TestRealTileReEncodedAsLinear: code-value tile = sRGB-encoded export through as("linear"))
78 specks, first at raw (1765.6, 2404.9)
```

Assertions: speck view median rank below and top-50 share above `levels({ auto: true })`'s; labelled dust median snr
≥ 2 × the unlabelled median and its lower quartile above it; ranking by snr puts more gold points in the top 30 than
ranking by area; gold circles at `r / 2` beat the 3 px core; the sRGB-encoded export through `as("linear")` matches
the code-value tile within 3/65535, masks within 2e-3, stats within 1e-4. Hot-list thresholds with the new radii:
37 spots per tile (was 58.9 with the clamped radius and the sRGB misread), 60.4 % by snr.

Ring-radius variants on the 190 gold dust points (an experiment, not committed): core r, ring [r+3, r+9]: 0.85; core
r/2, ring [1.5 r, 2.5 r]: 1.98; core r/2, ring [r, 2r]: 1.78; core r, ring [2r, 3r]: 0.88.

`git diff --stat 7ffe54b 449aaed -- pkg cmd internal README.md`: 74 files changed, 1847 insertions(+), 1983
deletions(-); non-test Go and JavaScript only: 33 files changed, 967 insertions(+), 1283 deletions(-).
