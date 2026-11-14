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
- N/A

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
