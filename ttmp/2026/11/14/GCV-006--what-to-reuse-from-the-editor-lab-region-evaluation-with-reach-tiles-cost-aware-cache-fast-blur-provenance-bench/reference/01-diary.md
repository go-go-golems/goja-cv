---
Title: Diary
Ticket: GCV-006
Status: active
Topics:
    - cv
    - architecture
    - performance
    - research
    - agents
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/goja-cv/bench.go
      Note: the bench command (Phase 0)
    - Path: repo://internal/testimage/benchscan.go
      Note: the 24 MP bench scan
    - Path: repo://pkg/cv/graph/eval.go
      Note: per-node run time (countRun, LastRunTime)
    - Path: repo://pkg/cv/graph/explain.go
      Note: run time in explain lines
    - Path: repo://pkg/cv/kernels/measure.go
      Note: 'stats kernel: quickselect per plane, the 4.8 s finding'
    - Path: repo://pkg/cvsession/provenance_test.go
      Note: sidecar tests
    - Path: repo://pkg/gojamod/cv/provenance.go
      Note: provenance sidecars (Phase 6)
    - Path: repo://pkg/gojamod/cv/save.go
      Note: save calls the stamping path
    - Path: repo://ttmp/2026/11/14/GCV-006--what-to-reuse-from-the-editor-lab-region-evaluation-with-reach-tiles-cost-aware-cache-fast-blur-provenance-bench/scripts/01-boxblur-vs-gaussian.go
      Note: blur comparison
    - Path: repo://ttmp/2026/11/14/GCV-006--what-to-reuse-from-the-editor-lab-region-evaluation-with-reach-tiles-cost-aware-cache-fast-blur-provenance-bench/scripts/03-window-costs.js
      Note: window costs on a 24 MP image
ExternalSources: []
Summary: 'Diary for GCV-006: reading the Editor lab, mapping it onto goja-cv, measuring goja-cv on a 24 MP image, importing sources, writing the reuse guide.'
LastUpdated: 2026-11-14T17:10:00-05:00
WhatFor: ""
WhenToUse: ""
---










# Diary — GCV-006

## Goal

Compare the Editor lab (a browser photo editor with a tiled, content-cached engine, an agent `eval` tool and a
textbook, delivered as `~/Downloads/editor-lab-work.zip`) with goja-cv, decide what goja-cv should reuse, and write
it up as an intern-level analysis and design guide, with the relevant sources in the ticket and the guide on the
reMarkable.

## Step 1: Unpack, read the lab, map goja-cv

The zip was unpacked into its own empty scratch directory (untrusted data, nothing run from it). I read the writing
skills first (`textbook-authoring`, `ticket-research-docmgr-remarkable` with its writing-style and checklist
references, `docmgr`, `remarkable-upload`, `diary`), then the lab, then goja-cv's evaluator.

### Prompt Context

**User prompt (verbatim):** "Unzip @~/Downloads/editor-lab-work.zip and compare it to your CV implementation, what are things that would make sense to reuse from their work to augment our capabilities? Create a new docmgr ticket, write a detailed implementation / analysis / architecture/defsign guide for a new intern joining the project, making sure to introduce all the fundamentals they need in the right order, glossary for jargon (see skills in ~/.codex/skills) in a textbook authoring format, store in the ticket and upload to remarkable when done. Prose, pseudocode, code snippets, diagrams, bullet point lists, callouts, all the things."

Mid-turn: "Import the relevant source into sources/ of the ticket"

**Assistant interpretation:** Study the lab's engine and agent design against goja-cv's, find what is worth taking, ground it in measurements, and write a textbook-style guide with decision records; copy the relevant lab files and research sources into the ticket.

**Inferred user intent:** Know which of the lab's ideas improve goja-cv (especially for large scans) before investing in them, with a document a newcomer can learn the area from.

### What I did
- Read the lab's `README.md`, `REPORT.pdf` (text via PDFKit), `research/notes.md`, `src/engine.js` in full, `views/agent.js` (`labScope`, `LAB_REFERENCE`, `runLab`), `views/panels.js` (`Provenance`), the book's chapter headings, §§5.2–5.3, §6.4 and every "Toward a reusable API" section, and libvips' `how-it-works.md`.
- Read goja-cv: `graph/eval.go` (signatures, singleflight, compute), `registry.go` (`OpDef`: `Class`, `Global`, `AnalysisScale`, `Unit`), `render.go` (windows for crops and decorations only), `cache.go` and `engine.go` (LRU, 2 GiB), `sources.go` (`SourceKey`, pinned pyramids), `frame.go` and `value.go`, `kernels/filter.go` (Gaussian with 6σ+1 taps), `kernels/morph.go` (van Herk), `ops/views.go`, `ops/mask.go`, `ops/measure.go`, `cvsession/yaml.go` (`capText`), `cvsession/docs_test.go` (`TestGuideCodeBlocksRun`).

### Why
- The lab's value for goja-cv is not its editing features but its evaluation model; that needed the engine and the evaluator side by side.

### What worked
- The two systems share a vocabulary (lazy graph, content keys, one `eval` tool, YAML results), so the comparison reduced to a few real differences: reach and partial evaluation, per-tile caching, eviction policy, blur algorithm, provenance, a bench.
- goja-cv already has `Global` on `OpDef` and a `Window` in `EvalCtx`, so region evaluation extends existing structure.

### What didn't work
- `pdftotext` is not installed, and the Read tool's PDF rendering needs poppler: "pdftoppm is not installed". Extracted the report's text with a five-line Swift PDFKit script (`scripts/05-pdftext.swift`); tables lose their layout.

### What I learned
- The lab's research found that darktable keys its pixelpipe cache the same way goja-cv does (a chain of hashes, a deep constructive trace).
- goja-cv's `Render` evaluates pixel ops "at scale 1 over its whole frame … cut to the window": the window is known and discarded.
- `cv.open(path, { box })` is today's manual workaround for windows, and it changes every global statistic (the speck view's stretch).

### What was tricky to build
- Separating what goja-cv already has (runnable doc blocks, introspection, result cap, read-only sessions, DocPx scaling, masks) from what it lacks; several lab features look new but are not.

### What warrants a second pair of eyes
- The classification of ops as point, neighbourhood or global in the guide's §12.1 table comes from reading code, not from a test; Phase 1's registry test is what makes it reliable.

### What should be done in the future
- N/A (see step 3).

### Code review instructions
- Start with the guide's §10.4 table; check each row against the cited goja-cv file.

## Step 2: Measurements on a 24 MP image

The guide's argument rests on what goja-cv costs today for windows of a large image, and on whether the lab's fast
blur and JavaScript layers would pay in Go and goja.

### Prompt Context

**User prompt (verbatim):** (see step 1)

**Assistant interpretation:** Ground each proposal in numbers from goja-cv itself.

**Inferred user intent:** Recommendations backed by evidence, not by the lab's browser numbers.

### What I did
- `scripts/02-make-big-npy.py`: a synthetic 6000 × 4000 × 3 float32 image (288 MB).
- `scripts/03-window-costs.js` with the installed `goja-cv` (main 5a93095) under `/usr/bin/time -l`: 64 px windows of the source, of `gaussian(2/8/32)` and of the speck view; `peaks` over the image. Output in `scripts/03-window-costs.out`.
- `scripts/01-boxblur-vs-gaussian.go` (`//go:build ignore`, run with `go run` from the module root): `kernels.Gaussian` against a Go port of the lab's `boxSizes` three-box blur on one 6000 × 4000 plane, σ 1–32, time and error. Three runs.
- `scripts/04-goja-per-pixel.js`: a JS function per pixel over 1 MP in goja, against two Go point ops.

### Why
- The lab's timings are for JavaScript in headless Chrome on 2 cores; goja-cv's costs are different.

### What worked
- Window costs: crop of the source 2 ms; `gaussian(2/8/32)` window 161/662/3,143 ms (the whole image is blurred); speck-view window 365 ms, a second window 0 ms (cached); `peaks` 1,871 ms; peak RSS 3.6 GB.
- Blur: Gaussian cost linear in σ (about 0.04 to 5.6 s from σ 1 to 32 across runs); three boxes roughly flat (57–374 ms); error 31 % at σ 1 and 4–8 % of the peak deviation for σ ≥ 2.
- goja per pixel: 1,569 ms per MP against 175 ms for two Go point ops and a `stats()`.

### What didn't work
- `goja-cv eval --root $S --file t.js` → `goja-cv: unknown flag: --file`; used `-e "$(cat t.js)"`.
- The first box-blur port was single-threaded with a column-gather pass: about 500 ms at every σ, slower than the parallel Gaussian up to σ 16. An unfair comparison; rewrote it with parallel bands and a row-major running sum per column.
- The load average was 24 to 32 on 10 cores during all runs (`uptime`: "load averages: 32.56 26.68 24.27"); repeated runs differ by up to 2×. The guide reports ranges and argues from ratios.

### What I learned
- A window costs the whole-image computation: a σ 32 window of 64 px reads 0.27 % of what goja-cv computes for it.
- The three-box approximation's error is large enough to move calibrated thresholds; it cannot replace the Gaussian without the gold tests.

### What was tricky to build
- A fair box-blur port: the vertical pass must run row-major (one accumulator per column) to be cache-friendly in Go.
- The error metric: absolute error says little; dividing by the maximum deviation from the mean expresses it relative to the signal a detector sees.

### What warrants a second pair of eyes
- All timings were taken on a loaded machine; Phase 0's harness should redo them on an idle one.

### What should be done in the future
- Rerun `01` and `03` on an idle machine when Phase 0 lands.

### Code review instructions
- `go run ./ttmp/2026/11/14/GCV-006--*/scripts/01-boxblur-vs-gaussian.go` from the goja-cv root.
- `python3 scripts/02-make-big-npy.py DIR && goja-cv eval --root DIR -e "$(cat scripts/03-window-costs.js)"`.

## Step 3: Ticket, sources and the guide

### Prompt Context

**User prompt (verbatim):** (see step 1)

**Assistant interpretation:** Create GCV-006, copy the cited lab files and third-party sources into `sources/`, and write the guide.

**Inferred user intent:** One place that holds the evidence and the argument.

### What I did
- Branch `task/gcv-006-editor-lab-reuse`; topics `performance` and `research` added to the vocabulary; `docmgr ticket create-ticket --ticket GCV-006`.
- `sources/editor-lab/`: README, REPORT (PDF and text), `src/engine.js`, `workspace.js`, `app.js`, `views/agent.js`, `bench.js`, `code.js`, `panels.js`, book chapters 4–7 and 9, `research/notes.md`, and from `research/src/` the libvips, darktable cache and tiling, GIMP chunk iterator, GEGL tile cache and wrk2 files with `SOURCES.md`. `sources/README.md` says why each is there. Krita, Chromium, the UI views, tests, figures and the earlier docs stayed in the zip.
- `design-doc/01-…`: the guide. Part I fundamentals (reach, borders, content keys, pyramids, tiles and demand-driven evaluation, global barriers, eviction, measuring); Part II both systems with a side-by-side table; Part III the measurements; Part IV six proposals with decision records, plus "already have" and "leave behind"; Part V a phased plan, tests, risks; glossary; references.

### Why
- The user asked for the sources in the ticket; copying only what the guide cites keeps the ticket readable.

### What worked
- The proposals ordered themselves by evidence: region evaluation first (100–1,000× on windows), then tiles, the bench, eviction, the fast blur (gated), provenance.

### What didn't work
- First real upload: `Error: pandoc failed: xelatex not found. Please select a different --pdf-engine or install xelatex` (the dry run does not run pandoc). `/Library/TeX/texbin` was missing from this shell's PATH; with `export PATH="/Library/TeX/texbin:$PATH"` the upload printed `OK: uploaded GCV-006 Editor lab reuse guide.pdf -> /ai/2026/11/14/GCV-006`.

### What I learned
- goja-cv's speck view hides a global statistic (`levelsAt`, white = 5 × p99) inside a local chain; region evaluation needs such ops split into a statistic and a point op.

### What was tricky to build
- Keeping today's best case (a cached whole-image view makes every later window free) in the window design: the evaluator must cut from a cached whole value before it computes a window.

### What warrants a second pair of eyes
- The third-party files are GPL and LGPL reference copies with their headers; check that keeping them under `ttmp/` is acceptable for the repository.
- The decision records are `proposed`; none is agreed.

### What should be done in the future
- The user decides which proposals to take; Phase 0 (bench and timing) and Phase 6 (provenance) can start independently.

### Code review instructions
- Read the guide's executive summary, then §11 (evidence) and §12.1 (region evaluation).
- `docmgr doctor --ticket GCV-006 --stale-after 30`.

## Step 4: Decisions after review, folded into the guide

The proposals were reviewed with the user, using evidence the guide did not have when it was written: the
*Agent-Operated Engines* pattern language in the vault, whose chapter 4 is §12.1 as a pattern, and its validation
(agents built small region engines with and without the chapter; blind Opus reviewers judged the builds; probes found
concrete traps). The guide now records which proposals are accepted, which wait for evidence and which are deferred,
and carries the experiment's lessons into §12.1's precondition.

### Prompt Context

**User prompt (verbatim):** "ok, now oging back to editor-lab, what do you think would make sense to pull into goja-cv? did we already write a ticket and design doc about that?"

Then: "ok, do it. Keep a detailed diary (see skill in ~/.codex/skills) as you work, commit at appropriate intervals."

**Assistant interpretation:** Fold the updated recommendation into GCV-006 (decisions table, lessons), open a PR for the docs, then implement Phase 0 (bench and per-node timing) and Phase 6 (provenance sidecars), keeping this diary step by step and committing per step.

**Inferred user intent:** Turn the analysis into working, measured improvements, starting with the cheap ones that every later decision needs, without committing to the large region-evaluation work before the evidence says it pays.

### What I did
- `design-doc/01-…`: a decisions paragraph in the executive summary; new §12.7 "Decisions after review" with a status
  table (3 and 6 accepted; 1 and 5 conditional; 2 and 4 deferred) and six lessons for §12.1 from the construction
  experiments; the decision records of §12.3 and §12.6 marked `accepted`.
- `tasks.md`: regrouped into accepted, conditional (with a new task: check annotator transcripts for crops of computed
  full-size images), deferred and blockers.

### Why
- §11.1 measured a synthetic 24 MP case. Whether region evaluation pays depends on what agents really do; the DUST
  annotators mostly worked on tiles. The bench (Phase 0) and the transcripts answer that; until then the large phases
  stay conditional.

### What worked
- The experiment's findings map one to one onto §12.1's design points (exactness, footprint, cost rule, statistics,
  the cut-from-whole path), so the precondition is concrete rather than a general caution.

### What didn't work
- N/A

### What I learned
- The pattern experiments' strongest region-evaluation lesson is about tests, not code: tolerance-based invariance
  tests hid an origin-dependent kernel. The invariance test has to compare bit for bit.

### What was tricky to build
- N/A (documentation step).

### What warrants a second pair of eyes
- The conditional status of §12.1: it is the guide's biggest proposal; the user may prefer to commit to it now.

### What should be done in the future
- Mine the DUST-002 annotator transcripts for crops and windows of computed images (the conditional task).

### Code review instructions
- Read §12.7 and the executive summary's decisions paragraph; `tasks.md`.

### Technical details
- Source of the lessons: vault `Research/Software Architecture Garden/general/agent-operated-engines/_validation/LOG.md`
  step 9 and `reviews/04-o1.json`.

## Step 5: Phase 0, per-node run time and the bench

Phase 0 gives every later decision its numbers. The evaluator now records how long each node's op took the last time
it ran, and `cv.explain(node).graph` ends each evaluated node's line with that time, so an agent sees the slow step
of its chain. `goja-cv bench` runs scenario scripts (evals separated by `// --- name` lines), each repeat in a fresh
session, and reports per eval p50 and p95, op runs, cache hits and misses, cached megabytes and the process's peak
memory. A generator writes the 24 MP bench scan, and three scenarios ship in `examples/bench/`. The baseline is in
`reference/02-phase-0-bench-baseline.md`; its biggest surprise is that whole-image `stats()` costs 4.8 s.

### Prompt Context

**User prompt (verbatim):** (see Step 4)

**Assistant interpretation:** Implement Phase 0 of the plan (§12.3, §15): per-node timing in the evaluator and explain, and a scenario bench; record a baseline.

**Inferred user intent:** Measure before optimising, and give agents a way to see costs from inside an eval.

**Commit (code):** 283f9bc — "Per-node run time: the evaluator records each op's last run duration; explain shows it (GCV-006 Phase 0)"; 79a3a3a — "goja-cv bench: scenario runner (…), 24 MP bench scan generator, three scenarios (GCV-006 Phase 0)"

### What I did
- `pkg/cv/graph/eval.go`: `Evaluator.times` map, `LastRunTime(id)`, `countRun(id, d)`; the op's `safeRun` is timed in
  `compute`. `pkg/cv/graph/render.go`: the same in the render path. `pkg/cv/graph/explain.go`: `fmtRunTime` ("0.4 ms",
  "41 ms", "3.1 s") appended after the cached marker.
- Tests: `TestExplain` (graph) and `TestPart2Section3NodesAndEvaluation` (module) asserted exact suffixes and column
  padding; rewritten to check the cached marker, the run time on evaluated nodes and its absence on the unevaluated
  root.
- `pkg/cvsession/session.go`: `EngineStats` (op runs, cache stats) and `App.EngineStats(ctx, sessionID)`.
- `cmd/goja-cv/bench.go`: `parseScenario`, `benchRepeat` (fresh `cvsession.App` per repeat), `summarize` (nearest-rank
  p50/p95, counts from the last repeat, a failure in any repeat reported), YAML or `--json`; `rss_unix.go`
  (`getrusage`, bytes on macOS, KiB elsewhere) and `rss_other.go`. `cmd/goja-cv/bench_test.go`: separators, percentile,
  and an end-to-end run (cold blur computes, the same request in the next eval is a hit with 0 runs, a failing eval is
  reported).
- `internal/testimage/benchscan.go` + `gen -bench-scan 6000x4000`: a deterministic 24 MP float32 NPY (noise around
  0.3, bright 3×3 specks), 1.5 s to write.
- `examples/bench/{big-blur,scan-zoom,scan-probe}.js`; `pkg/doc/03-cli.md` "bench" section; README development lines;
  `Makefile`: `bench.go` added to glazed-lint's allow-paths, as for the other Cobra commands.
- Ran the baseline (3 repeats) and an extra decode-vs-stats scenario; wrote `reference/02-phase-0-bench-baseline.md`.

### Why
- Every proposal after Phase 0 claims a speed-up; the bench is the yardstick. Per-node time in explain is the agent's
  view of the same thing (the lab shows `ms_last_update`).

### What worked
- The bench's counts are deterministic across cold repeats, so a change that alters op runs or cache behaviour shows
  up exactly, not just as noise in times.
- The baseline reproduces §11.1's shape (σ 32 window 4.0 s now vs 3.1 s then; this run was under heavier load).

### What didn't work
- `make lint`: `cmd/goja-cv/bench.go:63:2: define CLI flags with cmds.WithFlags(fields.New(...)) instead of raw Cobra/pflag/flag APIs` (seven lines). The other commands use raw Cobra flags through the Makefile's `-glazedclilint.allow-paths`; bench joined that list.
- The first `scan-probe` run: `promise rejected: measure would add coreMean, which these items already have.` (peaks
  items already carry the ring fields). Fixed with `measure(items, { prefix: "probe" })`. The bench reported it as an
  eval error without stopping, which is the intended behaviour.
- `go test ./...` still fails only `TestRealTopHatSaturated` (known, the gold set grew to 164 points; not touched here).

### What I learned
- Whole-image `stats()` costs 4.8 s on 24 MP, 16 times the decode (0.3 s): four planes, each copied and quickselected
  at least three times on one core. It is the largest single cost measured and is not on GCV-006's list.
- The speck view's later windows are already cheap (19 windows in 6 ms); region evaluation would only cut the first
  window of a view. That weakens the case for Phases 1–2 for zooming agents, and strengthens it only for many
  different computed images per scan.

### What was tricky to build
- Counting per eval: the cache and run counters are per session and cumulative, so the bench snapshots them before
  and after each eval and reports deltas; `cached_mb` is the level after the eval, not a delta.
- Peak RSS is process-wide and monotone; with several scenarios in one invocation, later scenarios report the peak of
  everything before them. Documented in the help; run one scenario per invocation for per-scenario peaks.

### What warrants a second pair of eyes
- `countRun` now takes the duration; both call sites (compute and render) time only `safeRun`, not input evaluation.
  Is "the op's own time" the right number for explain (it is what an agent can act on), or should inputs be included?
- The explain line grew a column; agents parse explain lines by eye only, but check nothing downstream splits them.

### What should be done in the future
- A `stats()` speed-up (parallel planes or exact histogram selection), measured with this bench and gated by the gold
  tests.
- A `tile-session` scenario with the DUST prelude (`mem.views`, `mem.zoom`, `mem.sheet`), kept with the dust repo.
- Rerun the baseline on a quiet machine.

### Code review instructions
- Start at `cmd/goja-cv/bench.go` (`runBench`, `benchRepeat`, `summarize`), then `pkg/cv/graph/eval.go` (`countRun`,
  `LastRunTime`) and `explain.go` (`fmtRunTime`).
- `GOWORK=off go test ./cmd/goja-cv/ ./pkg/cv/graph/ ./pkg/gojamod/cv/ ./pkg/doc/`; `make lint`.
- `go run ./internal/testimage/gen -out /tmp/b -bench-scan 6000x4000 && go run ./cmd/goja-cv bench --root /tmp/b --repeat 2 examples/bench/big-blur.js`

### Technical details
- Scenario format: text before the first `// --- name` is a comment; each part is one eval body; `mem` carries values
  between evals.
- Report fields: `p50_ms`, `p95_ms`, `runs`, `cache_hits`, `cache_misses`, `cached_mb`, `status`, `error`; per
  scenario `total_p50_ms`, `peak_rss_mb`.

## Step 6: Phase 6, provenance sidecars on every save

Every `cv.save` now writes `<file>.provenance.json` beside the file. The sidecar records what made the file (the
goja-cv module, version and VCS revision, a session id, the eval number, and the agent's line of the save), what the
file is (its bytes and SHA-256), and what it came from: the saved node's path and signature, and every source file
under the node with its size, mtime and SHA-256. Data saved from JavaScript values has no node, so its sidecar lists
the sources the session had opened and says `plain`. The save result names the sidecar. An agent cannot save a file
whose name ends in `.provenance.json`.

### Prompt Context

**User prompt (verbatim):** (see Step 4)

**Assistant interpretation:** Implement Phase 6 (§12.6): provenance sidecars on saves, with the decision "a sidecar for every save" now accepted.

**Inferred user intent:** Make every saved artefact, DUST labels included, traceable to the scan, the code and the script line that produced it.

**Commit (code):** a1f29bd — "Provenance sidecars: every cv.save writes <file>.provenance.json (…) (GCV-006 Phase 6)"

### What I did
- `pkg/gojamod/cv/provenance.go`: `Provenance` and its parts; `beginProvenance` (owner goroutine: mapped site, eval
  number, node path, scale-1 signature, a walk of `Inputs` collecting `open` nodes' `SourceMeta`, or the session's
  opened sources for plain data); `stampSave` (save goroutine: bytes hash, source hashes, JSON, `ws.Write` of the
  sidecar); `sourceHash` (once per session per location and stamp); `buildInfo` (`runtime/debug.ReadBuildInfo`);
  `SetSiteMapper`.
- `pkg/gojamod/cv/save.go`: refuses `*.provenance.json` names (`E_ARG_TYPE`, fix `…-data.json`); calls
  `beginProvenance` before `startSave` and `stampSave` after the file is written. `binding.go`: `prov` state;
  `evaltrack.go`: `BeginEval` counts evals.
- `pkg/cv/dataout/save.go`: `Saved.Provenance`, `provenance` in the JS result.
- `pkg/cvsession/session.go`: installs the site mapper (eval-cell lines past the one-line wrapper).
- `pkg/cvsession/provenance_test.go`: sidecar fields and hashes, agent line 3, eval 2, equal signatures for the same
  value and different ones for another; plain data; refused names. `pkg/cv/docs/topics/13-data-out.md`: the sidecar.

### Why
- The lineage-ledger lessons (vault chapter 11, construction experiments): one stamping path every creating path goes
  through, and a sidecar that names what the file was made from, not only the save's own inputs.

### What worked
- The node walk over `Inputs` finds the sources of views, macros and builder lambdas alike (the speck view's sidecar
  lists `scan.tif` once).
- A built binary embeds the VCS revision: `"vcs_revision": "b7046743…", "vcs_modified": true` (the work tree had the
  uncommitted Phase 6 change), version `v0.0.0-20261115005911-b7046743a769+dirty`.
- Mutation check of the line test: with the site mapper disabled the test fails with `Site:{Script: Line:4}`
  (want 3); restored, it passes.

### What didn't work
- First test draft used a `mustFail` helper that does not exist in the package (`undefined: mustFail`); the existing
  tests check failures with `text(eval(...))` and `code: …`, which I used instead.

### What I learned
- Line numbers of eval cells are the wrapped source's; only the session layer knows the wrapper, so the `cv` module
  needs a hook (`SetSiteMapper`) to record the agent's line. The E_DUPLICATE message in `save.go` reports
  `site.Line` raw; check whether the processor maps it (not changed here).

### What was tricky to build
- Ordering between goroutines: the graph, the binding's opened list and the runtime's call stack may only be read on
  the owner goroutine, so everything except hashing and writing happens before `startSave`; the save goroutine only
  hashes, marshals and writes.
- A sidecar that fails to write fails the save, after the file itself was written: the file then exists without
  provenance, and the eval fails with the sidecar's error. Writing the sidecar first would leave a sidecar for a file
  that may not exist; I chose the failing eval as the louder signal.

### What warrants a second pair of eyes
- Source paths in the sidecar are absolute (`/Users/…`): useful for audits, but they leak the machine's layout into
  saved artefacts that may be shared.
- Hashing a large source (288 MB) costs about 0.3 s on the first save of a session; later saves reuse the hash.
- Plain data lists every source the session opened, which over-approximates what the data came from.

### What should be done in the future
- DUST scripts that read saved labels can now check the sidecar; a small `cv`-side reader (`cv.provenance(path)`)
  could come later if agents need it.

### Code review instructions
- `pkg/gojamod/cv/provenance.go`, then the two calls in `save.go`; `pkg/cvsession/provenance_test.go`.
- `GOWORK=off go test ./pkg/cvsession/ -run 'Provenance|SidecarNames' -v`; `make lint`; `make smoke`.

### Technical details
- Sidecar schema `goja-cv/provenance/1`: `schema, file, bytes, sha256, at, goja_cv{module, version, vcs_revision,
  vcs_modified}, session, eval, site{script, line}, node{path, signature} | data, sources[{name, path, size, mtime,
  sha256}]`.

## Step 7: E_DUPLICATE reported the wrapped line

Recording the agent's line in provenance showed an older off-by-one: `E_DUPLICATE` said "this eval already saves
t.json (line 2)" when the first save was on the agent's line 1, because it stored the runtime's line, which counts
the session's one-line wrapper. The save now records `agentSite(site).Line`, the same mapping provenance uses.

### Prompt Context

**User prompt (verbatim):** (see Step 4)

**Assistant interpretation:** A defect found on the way; fix it in the Phase 6 PR since it uses the same hook.

**Inferred user intent:** Errors an agent acts on must name its own lines.

**Commit (code):** see `git log` on `task/gcv-006-phase6-provenance` — "E_DUPLICATE names the agent's line of the first save, not the wrapped source's (…)"

### What I did
- `pkg/gojamod/cv/provenance.go`: `agentSite`; `save.go`: `track.saved[rel] = b.agentSite(site).Line`.
- `pkg/cvsession/save_test.go`: a two-line case expecting "already saves twice2.json (line 1)".

### Why
- The error's `where` already showed the agent's line 2 for the second save; its message pointed at line 2 for the
  first one too, which is wrong and sends the agent to the wrong line.

### What worked
- Mutation check: with the old `site.Line` the new case fails with `already saves twice2.json (line 2)`; with the fix
  it passes.

### What didn't work
- My first mutation check stashed the whole change, test included, so it proved nothing; redone by reverting only
  `save.go`.

### What I learned
- Any message that formats a `Site.Line` inside the `cv` module needs the session's mapping; `agentSite` is now the
  one place to get it.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Other places in `pkg/gojamod/cv` that print `site.Line` into messages directly (grep `site.Line`); only the
  duplicate-save one was checked.

### What should be done in the future
- Audit the remaining `site.Line` uses.

### Code review instructions
- `GOWORK=off go test ./pkg/cvsession/ -run TestSaveRefusals -v`.

### Technical details
- Before: `message: 'cv.save("twice2.json"): this eval already saves twice2.json (line 2).'` for a first save on line 1.
