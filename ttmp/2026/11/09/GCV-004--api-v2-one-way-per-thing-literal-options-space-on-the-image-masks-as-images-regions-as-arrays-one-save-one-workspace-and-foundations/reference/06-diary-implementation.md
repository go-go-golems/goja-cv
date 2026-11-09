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
