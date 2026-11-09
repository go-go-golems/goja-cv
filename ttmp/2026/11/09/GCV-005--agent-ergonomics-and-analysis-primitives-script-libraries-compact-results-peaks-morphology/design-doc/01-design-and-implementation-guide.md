---
Title: Design and implementation guide
Ticket: GCV-005
Status: active
Topics:
    - api
    - agents
    - analysis
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-005--agent-ergonomics-and-analysis-primitives-script-libraries-compact-results-peaks-morphology/reference/01-analysis-annotator-agent-feedback.md
      Note: coordinator analysis and per-issue disposition this guide designs against
    - Path: ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/design-doc/01-design-and-implementation-guide.md
      Note: the base API (revision 3); its architecture tour, workspace, items and look queue are assumed throughout
    - Path: pkg/cvsession/session.go
      Note: preludes (runPreludes), the eval wrapper and line mapping; --prelude becomes a cv.load eval
    - Path: pkg/cvsession/init.js
      Note: session globals and __cv_run
    - Path: pkg/cvsession/processor.go
      Note: warnings, UNUSED_NODE (unusedNodes), errorNode (where), contextBlock; site attribution, function docs, LOAD warning
    - Path: pkg/cvsession/yaml.go
      Note: the result renderer (array caps, capText); compact tables and function rendering
    - Path: pkg/cvsession/native.go
      Note: readFailure parses <eval>:line:col from the stack; gains file frames
    - Path: pkg/cvsession/reference.go
      Note: MCP Instructions (3940 of 4000 characters today) and the eval tool description
    - Path: pkg/gojamod/cv/evaltrack.go
      Note: Site (Source, Line), site(), noteCall; loads are tracked here
    - Path: pkg/gojamod/cv/binding.go
      Note: native functions (binding.object); gains __load and __scripts
    - Path: pkg/gojamod/cv/prelude.js
      Note: the cv object; gains cv.load and cv.scripts
    - Path: pkg/cv/kernels/morph.go
      Note: van Herk minMax1D and the square opening used by the speck background; the disk and line kernels build on it
    - Path: pkg/cv/kernels/mask.go
      Note: DistanceToSet, Grow, Shrink; Grow and Shrink are deleted, DistanceToSet becomes the binary fast path of dilate/erode
    - Path: pkg/cv/kernels/probe.go
      Note: ProbeCoreRing, the core/ring measure that peaks reports
    - Path: pkg/cv/kernels/specks.go
      Note: the speck detector's morphological background (square opening/closing, kept)
    - Path: pkg/cv/ops/mask.go
      Note: lengthMaskOp grow/shrink; replaced by the morphology family
    - Path: pkg/cv/ops/sheet.go
      Note: listItems and labels; label field, points, no index labels
    - Path: pkg/cv/ops/present.go
      Note: overlay labels; label field
    - Path: pkg/cv/ops/views.go
      Note: speckView and levelsAt (white = 5 x p99 over the receiver); doc note on the tile-global scale
    - Path: pkg/cv/ops/measurement.go
      Note: coreRingFields; peaks reports the same fields
    - Path: cmd/goja-cv/mcp.go
      Note: --prelude flag; prelude must lie in a root
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/11-goja-cv-agent-feedback.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-evals
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-diaries
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/84-cv-helpers.js
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/04-goja-cv-dust-playbook.md
    - /Users/manuel/code/wesen/go-go-golems/go-go-parc/Projects/2026/11/09/ARTICLE - Deep Dive - goja-cv in Agent Hands - Feedback from 39 Annotator Agents.md
Summary: "Intern guide for GCV-005 (revision 1): what 39 annotator agents did with goja-cv, and the design that follows from it on top of API v2 (GCV-004). Part A: task libraries as a first-class part of a session (cv.load, cv.scripts, --prelude as a load, file:line locations), mem function docs in the context block, compact tables for row lists, UNUSED_NODE attributed to the eval's own code, item labels. Part B: peaks (sub-pixel local maxima of the core/ring step, snr against the local ring spread, multi-radius, Regions of disks in the flat item schema) and one morphology family for one-channel images (erode, dilate, opening, closing; disk and line elements) that replaces mask grow/shrink. Phases A1-A4 and B1-B3 are independent; gold-gated tests are relative; validation reruns an annotator agent and measures detection on gold."
LastUpdated: 2026-11-09T20:00:00-05:00
WhatFor: "Understanding why agents need task libraries, readable results and coordinates from data, and implementing GCV-005 phase by phase on top of GCV-004."
WhenToUse: "Read before touching cv.load, preludes, the result renderer, the context block, UNUSED_NODE, sheet/overlay labels, peaks or the morphology ops; read the GCV-004 guide's section 2 first if you are new to goja-cv."
---

# GCV-005 — Agent ergonomics and analysis primitives

## Revision history

| date | revision | what changed |
|---|---|---|
| 2026-11-09 | 1 | First version, written on `task/gcv-005-agent-ergonomics-and-primitives` (stacked on `task/gcv-004-api-v2` at `10938ce`, GCV-004 Phases 0b, 0c and 1 implemented) from the evidence of DUST-002 `reference/11`, the 374 extracted agent evals, the 34 agent diaries and the coordinator's disposition in `reference/01-analysis-annotator-agent-feedback.md`. |

## How to read this guide

This guide is for an intern who will implement GCV-005 in goja-cv. It assumes the system that GCV-004 (API v2)
describes, not the one in the repository today: GCV-004 is being implemented while this guide is written, and
GCV-005 is built on its final form. If you have never opened goja-cv, read section 2 of the GCV-004 guide
(`ttmp/2026/11/09/GCV-004--…/design-doc/01-design-and-implementation-guide.md`, "An architecture tour of goja-cv")
first; it takes about an hour and traces one call through every layer. Section 2 of this guide only revisits the
parts GCV-005 changes.

The guide then presents the evidence (section 3) and builds the vocabulary of the new design (section 4) before it
shows where today's code does each thing (section 5), the JavaScript API with before-and-after code taken from the
agents' own evals (section 6), the architecture (section 7), the decisions and their alternatives (section 8), what
is deleted (section 9), pseudocode for every new algorithm (section 10), errors (section 11), documentation (section
12), and the work in phases with tests and validation (sections 13–15). Section 16 is a migration note for the
helpers the agents used; section 17 lists risks and open questions. Appendix A lists every GCV-004 detail this
guide relies on and whether it is implemented yet.

Citations have the form `path:line` or `path:first-last` and refer to this branch at `3ff1f59`. When a line number
has drifted, search for the named function; the names are stable. Paths of the evidence are in the dust-scanning
repository; this guide abbreviates `/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/`
as `DUST-002/`.

GCV-005, like GCV-004, is alpha work with no backward compatibility. "Delete" means delete: no alias, no wrapper,
no migration error.

## 1. Executive summary

Thirty-nine Claude Sonnet agents annotated dust on film-scan tiles through goja-cv's MCP server: 374 evals, no
errors. They did not use goja-cv the way its documentation teaches. They pasted a 3 KB helper file into their first
eval, called its functions (`mem.sheet` 161 times, `mem.zoom` 119, `mem.hot` 78, `mem.probe` 65), and filtered the
returned rows with plain JavaScript. Nobody opened `cv.docs` or `cv.help`. The helpers were the interface; goja-cv
was the substrate. Where the helpers stopped, the agents read coordinates off zoomed pictures with rulers, and those
coordinates were 5–10 px wrong: on one tile, 28 of 32 positions an agent read off a look measured a speck-view step
of about zero when probed.

GCV-005 makes goja-cv good at being that substrate. It has two independent halves.

**Part A: task libraries and readable results.**

1. `await cv.load("dust/helpers.js", args)` runs a JavaScript file in the session. The file is found like any file
   `cv.open` reads (the GCV-004 workspace, then the `--read` directories), runs as the body of an async function with
   `args` bound, and returns what the file returns. Errors and warnings point into the file (`helpers.js:57`). It
   works the same in the MCP server, the CLI and xgoja, because it lives in the `cv` module. `cv.scripts()` lists the
   `.js` files that can be loaded, each with its first comment line. `--prelude FILE` becomes "load this file when a
   session starts". There is no `eval_file` MCP tool: `cv.load` costs about 40 characters of agent code, where the
   paste cost 2,900–3,400.
2. The context block after every eval lists the `mem` functions with their parameters, and gives the one-line doc of
   each function that is new since the last eval. The doc is the first comment inside the function's source.
3. An array of flat records (detector items, peaks, measurement rows, the agents' own rows) renders as a compact
   table, a header line and one line per row, instead of one YAML map per item. About 2.5 times as many rows fit
   under the 9,000-character cap, and the cut note says which rows are shown and how to page.
4. `UNUSED_NODE` reports only nodes built by the eval's own lines. A node built inside a loaded library and dropped
   by its caller is the library's business, which removes all twelve false warnings of the study.
5. Sheets and overlays label each item with its `label` field when it has one, else its `id`; sheets accept plain
   points `{ cx, cy, id }`.
6. The MCP instructions, the context block and a new "task libraries" docs topic teach all this where agents read.

**Part B: analysis primitives.**

7. `x.peaks({ radius, snr, polarity })` returns the local maxima of the core/ring step, the quantity `measure`
   reports, as a Regions node of disks: flat items `{ id, x, y, w, h, cx, cy, r, coreMean, ringMean, step, spread,
   snr, ringCoverage }` with sub-pixel centres. The snr is measured against the spread of the spot's own ring, so a
   faint speck in a dark, quiet area stands out as much as a bright one in a busy area. A list of radii finds spots
   of several sizes and keeps, where two overlap, the one with the higher snr; a saturated blob is then one peak at
   its own size instead of a ring of small ones. Peaks feed `sheet`, `measure`, `overlay` and `crop` directly.
8. One morphology family for one-channel images: `erode`, `dilate`, `opening`, `closing`, each with a disk or (with
   `{ line: angle }`) a line structuring element. The mask ops `grow` and `shrink` are deleted; `dilate` and `erode`
   give the same pixels on a mask. A top-hat is `x.sub(x.opening(r))`: it keeps bright things smaller than the disk,
   filled, which is what a saturated speck needs. Openings with line elements remove scene lines before
   thresholding.
9. The speck view keeps its white point (a noise-scaled view was measured and was worse: 87 % against 91 % gold
   coverage); its documentation says that the scale is set over the whole image, and how to get a local one.

Not in scope, each with its reason in section 6.11: `crop(box, { fit: "shift" })`, `components({ maxAspect })`, a
labelled grid on looks, `cv.quiet`, `measure.line`, a median filter and a `white` option for the speck view.

The design follows one rule from the evidence: **never make a model read a number off a picture; every look comes
with the data it shows.** Peaks give coordinates, tables give rows, sheet labels give ids that map back to rows.

## 2. An architecture tour of the parts GCV-005 touches

GCV-005 changes five places in goja-cv. This section shows each as it is today and says, in one sentence, what
GCV-004 changes in it, so that you know the ground GCV-005 is built on. The full tour is section 2 of the GCV-004
guide.

### 2.1 Where the parts are

```
 front ends   cmd/goja-cv (eval, run, mcp: --prelude, --root, --read)      pkg/xgoja/providers/cv
                 │                                                             │
 session      pkg/cvsession ── session.go   preludes, the eval wrapper, line mapping
                 │            ├─ init.js     globals: cv, mem, img; __cv_run
                 │            ├─ processor.go warnings, UNUSED_NODE, errors (where:), context block   ◄── A1, A2
                 │            ├─ yaml.go      the result renderer, the 9000-character cap             ◄── A2
                 │            └─ reference.go MCP instructions, eval tool description                 ◄── A4
 JS module    pkg/gojamod/cv ── prelude.js   the cv object                                            ◄── A1 (cv.load, cv.scripts)
                 │             ├─ binding.go native functions                                          ◄── A1
                 │             └─ evaltrack.go per-eval tracking: nodes, sites, looks                  ◄── A1
 graph        pkg/cv/ops     ── sheet.go, present.go (labels)                                         ◄── A3
                 │            ├─ mask.go (grow, shrink)                                                ◄── B1 (deleted)
                 │            └─ peaks.go, morph.go (new)                                              ◄── B1, B2
 pixels       pkg/cv/kernels ── morph.go (van Herk), mask.go (distance transform), probe.go (core/ring) ◄── B1, B2
```

### 2.2 The eval session

A **session** is one goja runtime that lives across evals. `App.Evaluate` (`pkg/cvsession/session.go:311-319`)
wraps the agent's code as the body of an async function, `__cv_run(async function () {\n` + code + `\n})`
(`session.go:297-302`), so `await` and `return` work at top level and agent line N is line N + 1 of the evaluated
source. go-go-goja's replsession runs that source with `vm.RunString` (module `go-go-goja`,
`pkg/replsession/evaluate.go:676`), so every eval's code has the same source name, which goja's stack traces print
as `<eval>`. `__cv_run` (`pkg/cvsession/init.js:88-105`) starts the body from a promise job and records an uncaught
error with `native.fail`.

A **prelude** is a script file given with `--prelude FILE` (`cmd/goja-cv/mcp.go:83`, `cmd/goja-cv/eval.go:36`).
`CreateSession` runs every prelude in a new session before any agent code (`session.go:207-221`), and
`runPreludes` (`session.go:226-250`) evaluates each file's text exactly like agent code, through `Evaluate`, and
turns a failure into "prelude FILE:LINE: message". GCV-004 keeps the flag and says that prelude paths, like
`--image`, are operator input resolved against the process's working directory (GCV-004 §7.8), and that looks
created while a prelude runs are discarded (GCV-004 §7.7).

`mem` is a plain object defined once per session (`init.js:66`). It is the only thing that survives between evals:
`const` and `let` are local to the eval's function.

### 2.3 After the eval: the result processor

replsession calls the **result processor** (`pkg/cvsession/processor.go`) after the code settles.
`Process` (`processor.go:64-92`) ends the eval's tracking, collects looks, renders the returned value as YAML,
appends warnings and the **context block**, and caps the text. Four of its parts matter here.

**Warnings with lines.** Every node a cv call returns is recorded with its **site**: the first stack frame outside
the cv prelude and native code (`Site{Source, Line, Column}`, `pkg/gojamod/cv/evaltrack.go:24-28`; `site()`,
`evaltrack.go:149-160`). `lineTag` (`processor.go:251-258`) turns a site into "line N" when its source is the
eval's (`Source == ""`) and its line falls inside the eval's code.

**UNUSED_NODE.** `unusedNodes` (`processor.go:260-316`) marks as used every node that the eval awaited, looked at,
returned, kept in `mem` or bound to an image name, together with all their inputs, and reports the last node of
every other chain the eval built: "builds a node and discards it. Nodes are lazy: nothing ran." To find nodes in
`mem` it renders `mem` with the display renderer (`processor.go:266-270`).

**The context block.** `contextBlock` (`processor.go:503-525`) lists the open images and the keys of `mem` with a
kind each (`memKind`, `processor.go:527-557`): `mem: T (string), best (function), box (array[4]), hot (function), …`.
A function is listed as "function", nothing more.

**Errors.** `readFailure` (`pkg/cvsession/native.go:144-187`) reads the thrown value and takes the first
`<eval>:line:col` of its stack (`native.go:136`, `182-185`) as the error's position; `errorNode`
(`processor.go:361-431`) renders `where: line N: <source line>`. A frame in any other source is ignored.

GCV-004 changes the processor in three ways that GCV-005 assumes: looks are queued when created and the over-budget
ones produce a `LOOKS` warning (GCV-004 §7.7); data handles and their budgets go (§7.8); the cut note names
`cv.save` instead of `cv.json`.

### 2.4 The result renderer

`renderer` (`pkg/cvsession/yaml.go:40-57`) turns a settled JavaScript value into a YAML node tree without running
getters or proxy traps. A cv node becomes a one-line summary, a look its caption, a function the string
`[Function]` (`yaml.go:111-113`). An array shows at most 48 items (`maxArrayItems`, `yaml.go:23`) and ends with a
comment "… N more (return cv.json(x) for all)" (`yaml.go:179-186`). A small collection of scalars is written in flow
style, `[1, 2, 3]` or `{x: 1, y: 2}`, and a map of more than eight entries is written one line per key (`flow`,
`yaml.go:243-265`). Finally `capText` (`yaml.go:286-303`) cuts the whole text at 9,000 characters on a line boundary
and appends a `CUT` note.

An awaited detector item has 10–20 fields, so a list of them renders one line per field: a list of 60 items is
about 1,200 lines. That is why the agents' helper turned its rows into strings
(`[o.id, o.x, o.y, o.sp, …].join(" ")`, `DUST-002/scripts/84-cv-helpers.js`, `mem.hot`).

### 2.5 Kernels and ops for spots and shapes

Three kernels in `pkg/cv/kernels` are the starting point of Part B.

- **The core/ring probe.** `ProbeCoreRing` (`pkg/cv/kernels/probe.go:178-195`) measures a point: the
  coverage-weighted mean of the gray plane in a **core** disk of radius r, the same in a **ring** (annulus) between
  r1 and r2, their difference (**step**), the 10–90 % spread of the ring's values (**spread**), and **snr** = step /
  max(spread, 1e-3). Pixel weights come from `EllipseCoverage` (`pkg/cv/kernels/mask.go:183`), the 4 × 4
  supersampled coverage that `mask.ellipse` uses. `measure` reports these fields (`coreRingFields`,
  `pkg/cv/ops/measurement.go:138-159`). GCV-004 sizes the core and ring from each item: core r from the item's `r` or
  its area, ring [1.5 r, 2.5 r] (GCV-004 D17); today's default ring is [r + 3, r + 9] (`measurement.go:117-124`).
- **Running minimum and maximum.** `minMax1D` (`pkg/cv/kernels/morph.go:14-46`) is the van Herk/Gil-Werman
  algorithm: the minimum or maximum over a sliding window of any length in three comparisons per sample.
  `SquareMinMax` (`morph.go:56-87`) applies it along rows and then columns, which gives the minimum or maximum over a
  square. `Opening` and `Closing` (`morph.go:91-107`) are built from it; their only user is the speck detector's
  background estimate (`SpeckBackground`, `pkg/cv/kernels/specks.go:106-154`). None of this is reachable from
  JavaScript.
- **The distance transform.** `DistanceToSet` (`mask.go:368-409`) computes, for every pixel, the exact Euclidean
  distance to the nearest selected pixel (Felzenszwalb and Huttenlocher's two-pass algorithm, `dt1d`,
  `mask.go:337-366`). `Grow` and `Shrink` (`mask.go:411-449`) threshold a mask at 0.5, take the distance to the
  selection (or to its complement), and draw the result with a one-pixel anti-aliased edge. They are the kernels of
  the mask ops `grow` and `shrink` (`lengthMaskOp`, `pkg/cv/ops/mask.go:301-319`, registered at `mask.go:340-341`)
  and of `regions.toMask({ grow })` (`pkg/cv/ops/regions.go:366`). `heal` also uses the distance transform
  (`pkg/cv/kernels/heal.go:35`).

The speck detector (`DetectSpecks`, `specks.go:166-…`) is the existing "find small things" op: luminance minus a
morphological background, thresholded at a noise level estimated once for the whole image (`MAD`, `specks.go:182`).

### 2.6 Items and regions, as GCV-004 defines them

GCV-005 builds on GCV-004's item model (GCV-004 §4.5, §7.6), which is not implemented yet on this branch. In short:

- An **item** is a flat object: `{ id, x, y, w, h, cx, cy, area, aspect, … }`, coordinates in doc px.
- A **Regions node** is a lazy region set produced by a detector (`detect.specks`, `components`) or by `measure`.
  Awaited, it is a plain JavaScript array of items; each item carries a hidden link `[node, index]` in a symbol-keyed
  property, which `filter`, `sort`, `slice` and object spread keep.
- An **item list** is what an op that takes regions accepts: a Regions node or an array. Linked items resolve to
  their regions (exact pixels); unlinked objects are **literal items**, a box `{ x, y, w, h }` or a circle
  `{ cx, cy, r }`. A centre without a radius is accepted only by ops that take points (`measure`, `sample`).
- A `Region` has a **shape**: `Pixels` (from a detector), `Box` or `Disk` (from literal items).
- Arrays are interned as an internal `items` op over at most one source node.

### 2.7 One session with a helper library, traced

This is what an annotator agent's session looked like (agent on tile `cfca85b41cd6f78f_2048_0`,
`DUST-002/various/agent-evals/cfca85b41cd6f78f_2048_0__pre-annotation.js`):

1. **Eval 1** is the helper file, 2,976 characters pasted as code. It defines ten functions on `mem` and returns
   "helpers ready (v1)". The context block lists `mem: T (string), best (function), hot (function), …`.
2. **Eval 2** calls `mem.tile(…)` and `mem.views()` and returns three looks.
3. **Eval 3** runs `await mem.hot(0.4)` and `await mem.hot(0.3)`. Inside, each helper builds a speck view, thresholds
   it, labels components, and calls `mem.probe` on each centroid; `mem.probe` measures 21 points around the centroid
   on two images and keeps the best. The result has 53 rows as strings; the array is cut at 48 with "… 5 more (return
   cv.json(x) for all)".
4. **Evals 4–7** page through the rows in contact sheets of 15 and look at four quarters of the speck view at 2×
   with rulers.
5. **Eval 8** probes 32 positions the agent read off those rulers. 28 of them measure a speck-view step below 0.06,
   which random places reach 90 % of the time: the agent had looked a few pixels away from the dot it saw.
6. **Evals 9–11** lower the threshold, filter rows by `sp`, `area` and region in JavaScript, sheet and zoom.

Every part of GCV-005 shortens one of these steps: eval 1 becomes a `cv.load` or a prelude; the strings in eval 3
become a table; the reading of rulers in evals 7–8 becomes `peaks`; the parallel arrays the agents kept to map sheet
cells to rows become item labels.

## 3. The problem and the evidence

### 3.1 The study

The DUST-002 project asked Claude Sonnet subagents to find dust, hair and scratches on 1024 × 1024 gold tiles (tiles
a person has labelled). Each agent followed a playbook (`DUST-002/reference/04-goja-cv-dust-playbook.md`), opened
one MCP session on goja-cv v1 (`main` at `b847520`), pasted the helper file `DUST-002/scripts/84-cv-helpers.js`, and
wrote a diary with a "goja-cv friction" section. Up to eight ran in parallel. `DUST-002/scripts/91-extract-agent-evals.py`
extracted every eval call from the transcripts into `DUST-002/various/agent-evals/` (`summary.md`, `evals.jsonl`,
one `.js` file per agent run); the diaries are in `DUST-002/various/agent-diaries/`. The coordinator's analysis is
this ticket's `reference/01-analysis-annotator-agent-feedback.md`; the colleague's hand-off is
`DUST-002/reference/11-goja-cv-agent-feedback.md`.

| measure | value |
|---|---|
| agent runs | 39 (31 pre-annotation, 4 trial 2, 4 trial 3) |
| evals | 374; 345 besides the helper paste |
| helper pastes | 29, of 2,859–3,418 characters each |
| median code length of the other evals | 183 characters |
| errors | 0 |
| warnings | 13 `UNUSED_NODE`, 7 `RANGE`, 1 `OUTSIDE`; 1 cut result |

### 3.2 What agents used

| | evals using it |
|---|---|
| helpers | `mem.sheet` 161, `mem.zoom` 119, `mem.hot` 78, `mem.probe` 65, `mem.tile` 37, `mem.views` 37, `mem.hotLook` 23, `mem.spots` 21, `mem.best` 20 |
| cv ops directly | `look` 288, `crop` 173, `sheet` 171, `view` 162, `levels` 52, `measure` 19, `luminance` 10, `components` 10, `overlay` 10, `exposure` 1 |
| never | `cv.docs`, `cv.help`, `explain`, `profile`, `sample`, `annotate`, `compare`, `highpass`/`dog`, `localContrast`, `chroma`/`channel`/`toLab`, `cv.csv`/`cv.json`, `save`, `P.slider` |

Everything the agents did beyond the helpers was JavaScript over returned rows. Counting the 345 non-paste evals
with a regular expression: `.map(` in 110, `.slice(` in 82, `.filter(` in 55, a filter or comparison on `sp` in 36,
on `aspect` in 31, on `snr` in 13; `mem.probe` in 52; `mem.hotByLevel` in 54. The diaries confirm the pattern: "the
useful pattern was filtering `mem.hotRows` in JS, then `mem.sheet(rows.map(o => [o.x, o.y]))`" (de14d942).

### 3.3 The ten issues

The hand-off lists ten issues with counts. The table gives each with its evidence, as the diaries and evals show it.

| # | issue | evidence |
|---|---|---|
| 1 | helper libraries must be pasted | about 15 diaries: "pasting the helpers needs the whole file text in the eval call (no include)"; agents trimmed comments and the final `return`; one lost a top-level `const r3` that its later code needed and added `mem.r3 = r3` (4b246f75) |
| 2 | the sheet badge hides cell 0's label; labels are list indices | 10 diaries on the badge ("(1342, 3250) shows as '2, 3250'", af8640d7 trial 2); 7 agents kept parallel arrays to map cell index to row (`mem.L`, `mem.LP`, 851dabe2) |
| 3 | coordinates read off ruler looks are wrong | 8 diaries; positions 5–10 px off; 28 of 32 guesses at sp ≈ 0 (cfca85b4); agents derived "x = crop_x + (px − 52)/2" for the ruler margin (a0a0f824) |
| 4 | the speck view's scale is set by scene edges | 6 diaries; real dust measured sp 0.03–0.1 on such tiles against a gold median of 0.31; agents boosted the view by hand (`.exposure(2.5)`, `.levels({ white: 0.25 })`) |
| 5 | saturated blobs become rings | b54a23af: the most obvious blob probed sp 0.21, snr 0.61; a capsule-shaped one was missing from the hot list |
| 6 | lines become long components | "the hot list is mostly line fragments" (21661036); "almost all streak rims" (8af4cabe, 55 of 62 rows); agents filtered `aspect < 3` |
| 7 | `RANGE` on edge crops gives non-square zooms | 7 warnings, all from zooms near the tile border |
| 8 | `UNUSED_NODE` for nodes a helper builds | all 13 warnings; 12 are `speckColor` of `mem.views()` when the caller used two of three views; 10 diaries call it harmless |
| 9 | long row lists hit the cap | 3 agents: "rows returned truncated at 9000 chars (`return rows.slice(30,90).join(" \| ")` was cut at ~60 rows)" (c71dd0f2 trial 2) |
| 10 | nothing measures the width of a thread | agents "eyeballed widths and path vertices from rulers" (af8640d7 trial 2); `profile` was never used |

### 3.4 What the evidence shows

Four findings drive the design.

**A task library replaced the documentation.** No agent opened `cv.docs` or `cv.help`. With a playbook and a helper
library, an agent needs goja-cv to run the library, show it what the library defined, and return rows it can read.
The documentation that matters is what an agent sees without asking: the MCP instructions, the context block, error
fixes, and the library's own comments.

**Models cannot read coordinates off pictures.** The most expensive failure was not an error message but a wrong
number: positions read off rulers. Rulers help a person; a vision model localizes poorly at this precision. What
worked was asking goja-cv for coordinates: component centroids, or `mem.probe`, which snaps to the best point within
2 px. This gives the design rule of GCV-005: **never make a model read a number off a picture; every look comes with
the data it shows.** It explains issues 2 and 3 together, and it is why a labelled grid on looks (the other ask of
issue 3) is not in scope: a better ruler is still a ruler.

**Agents filter with JavaScript.** Filters on `sp` (23 agents), on a region (7), on area or aspect (3), and probe
loops (25 agents) were all plain JavaScript over arrays. This confirms GCV-004's regions-as-arrays, and it is the
reason `components({ maxAspect })` is not added.

**The speck scale is tile-global, and the obvious fix was worse.** The speck view maps white to 5 × the 99th
percentile of its difference of Gaussians over the whole image (`levelsAt`, `pkg/cv/ops/views.go:215`). On tiles with
windows or cables that percentile is set by the edges. The colleague tested a view scaled by its own noise
(IQR / 1.349) on 197 gold points:

| hot-list scale | spots per tile (median) | gold covered |
|---|---|---|
| 5 × p99, level 0.3 (current) | 71 | 91 % |
| noise, 5.5 σ | 103 | 87 % |
| noise, 6 σ | 50 | 83 % |

Grain is not white noise, and grainy tiles flood a noise-scaled list. Local normalization belongs where candidates
are found, not in the view's default.

### 3.5 Caveats

- **"0 errors" is the helpers' result, not the raw API's.** The helpers absorbed the complexity; the earlier pi
  runs with DeepSeek and no helpers had 1–7 errors per run (GCV-004 §3.2). Both results stand: the raw API must stay
  learnable, and libraries must be easy to load.
- **"Never used" means "not shown by the playbook", not "unwanted".** `profile` was used 0 times and is the answer
  to issue 10. Nothing is removed on this evidence alone.
- **The population is narrow.** Sonnet with a playbook; the conclusion "docs do not matter" holds only when a task
  library exists.
- **Counts are regular-expression counts** over code; they measure use, not success.
- **Gold data changes.** `~/dustds/gold` held 142 points for GCV-003 and 198 today. Section 14.4 designs the gold
  tests to compare methods within one run, never against a fixed count.

### 3.6 Scope

In scope: `cv.load`, `cv.scripts`, preludes, error and warning locations in loaded files, the context block, function
rendering, compact tables, `UNUSED_NODE` attribution, item labels in sheets and overlays, sheets of points, the
`peaks` op, the morphology ops and the deletion of `grow`/`shrink`, the documentation an agent or a library author
reads, tests, and the validation tooling.

Out of scope and unchanged: the speck view's default; looks, rulers and crops; `measure` (GCV-004 already sizes it
from items); the speck detector; data out. Section 6.11 lists the asks that were considered and declined.

## 4. Concepts

This section defines the vocabulary of the design, in dependency order.

### 4.1 Task library, load, and the file's scope

A **task library** is a JavaScript file that defines functions for one task, usually on `mem`
(`mem.probe = async (x, y) => …`), so that an agent calls them instead of writing the same code again. A **pipeline
script** is a file that does one job and returns its result, for example a calibration that measures a tile and
returns numbers. goja-cv does not distinguish them; both are **script files**.

To **load** a script file is to run it in the current session with `cv.load(path, args)`. The file's text is the
body of an async function whose only parameter is `args`:

```js
// what cv.load("dust/helpers.js", { tile: "cfca…" }) runs, conceptually
(async function (args) { /* the file's text, unchanged, starting on this line */ })(theArgs)
```

So a script file has exactly an eval's semantics: top-level `await` and `return` work, `const` and `let` are local to
the file, the session globals (`cv`, `mem`, `img` and other image names) are visible, and anything meant to survive
goes on `mem`. The one addition is `args`, the value passed as the second argument (an empty object when none is
given). `cv.load` returns a promise of the file's return value.

### 4.2 A function's signature and doc line

A function on `mem` is visible to the agent through the context block. Its **signature** is its name and its
parameter list as written in the source (`probe(x, y)`, `hot(level = 0.4, max = 80)`). Its **doc line** is the first
comment inside the function's source text:

```js
mem.probe = async (x, y) => { // the best point within 2 px: { x, y, snr, sp, step, chroma, cover }
  …
}
mem.spots = (level = 0.4) => // blobs of the speck view above level, as regions
  mem.img.view("speck").mask.luminance({ above: level }).components({ minArea: 4 })
```

goja's `Function.prototype.toString` returns a function's exact source text, comments included; for the two
functions above it returns everything from `async (x, y) =>` and `(level = 0.4) =>` to the end of the body (checked
with `goja-cv eval` on this branch). The signature and the doc line can therefore be read from the function itself,
wherever it was defined.

### 4.3 A compact table

A **flat record** is a plain object whose property values are all scalars: numbers, strings, booleans, `null` or
`undefined`. Detector items, peaks, measurement rows and most rows agents build are flat records. A **table** is the
rendering of an array of at least two flat records as text: one header line with the union of their keys, then one
line per record with its values in the same columns. In the result's YAML it is a literal block scalar (a multi-line
string, `|-`), which the renderer already uses for multi-line strings.

### 4.4 The eval's own code; attribution

The **eval's own code** is the code the agent sent in this eval. A site (§2.3) is **in the eval's own code** when
its source is the eval's source name. Code from a loaded file has the file's name as its source. **Attribution** is
the rule that a warning about the agent's code (`UNUSED_NODE`) is raised only for sites in the eval's own code.

### 4.5 The step map and a peak

The **step** of a point is `measure`'s core/ring step: the mean of the gray plane in a disk of radius r around the
point minus the mean in the ring between 1.5 r and 2.5 r. The **step map** at radius r is the step computed at every
pixel. A **peak** is a local maximum of the step map: a pixel whose step is positive and at least as large as the step
of every other pixel within r of it. A **dark peak** is the same for −step. The peak's **sub-pixel centre** is the
maximum of a parabola fitted through the step map at the peak and its four neighbours.

### 4.6 Local noise and snr

The **local noise** of a point is the **spread** of its ring: the difference between the 90th and the 10th
percentile of the gray plane in the ring between 1.5 r and 2.5 r. The **snr** is step / max(spread, 1e-3), as in
`measure`. Because the spread comes from the spot's own surroundings, the snr compares a spot with its neighbourhood
and not with the whole image: a faint speck on a dark, quiet background has a small step and a smaller spread, and
therefore a high snr. This is the structural answer to issue 4, where a tile-global scale hid such specks.

### 4.7 Structuring elements and morphology

Grey-scale **morphology** computes, for each pixel, the minimum or maximum of the image over a small set of offsets
around it, the **structuring element** (SE). With an SE B:

- **dilation**: dilate(x)(p) = max over b ∈ B of x(p + b);
- **erosion**: erode(x)(p) = min over b ∈ B of x(p + b);
- **opening**: dilate(erode(x)): removes bright structures into which B does not fit, and leaves the rest unchanged;
- **closing**: erode(dilate(x)): removes dark structures into which B does not fit.

GCV-005 has two SEs. The **disk** of radius r is the set of integer offsets (dx, dy) with dx² + dy² ≤ r². The **line**
of radius r at angle θ is the digital segment of 2R + 1 pixels through the origin in direction θ, with R chosen so
that the segment is 2r + 1 doc px long (§10.10). Both are symmetric about the origin, so opening and closing need no
reflected element. Pixels outside the image are left out of the minimum and maximum.

On a mask (a one-channel image of 0 and 1), dilation by a disk of radius r selects exactly the pixels within distance
r of the selection: the same pixels as today's `grow(r)` before its anti-aliased edge.

### 4.8 Top-hat

The **white top-hat** of x with SE B is x − opening(x): what the opening removed, so the bright structures smaller
than B, with their full height. The **black top-hat** is closing(x) − x, the dark ones. A top-hat with a disk larger
than a saturated speck keeps the speck as a filled plateau, where a difference of Gaussians turns its flat top into a
ring (issue 5). A top-hat with a line element keeps everything that does not contain a segment of that length in that
direction.

### 4.9 Data with every look

The rule of §3.4 applied to results: every op that helps an agent choose where to look returns coordinates as data
(peaks, items), and every picture made of many places (a sheet) labels each place with an identifier that maps back
to a row of that data. Nothing the agent needs to type later should exist only in pixels.

## 5. Current-state analysis

This section shows, with line numbers, where today's code does each thing GCV-005 changes, and what the agents'
helpers do in its place.

### 5.1 Preludes exist; nobody knew

`--prelude FILE` is a repeatable flag of `eval`, `run` and `mcp` (`cmd/goja-cv/eval.go:36`,
`cmd/goja-cv/mcp.go:83`). `cvsession.Options.Preludes` (`session.go:84-88`) holds absolute paths resolved against
the base directory and "not subject to Roots"; `New` checks each is a readable file (`session.go:265-268`);
`runPreludes` reads each file and evaluates its text as an eval (`session.go:226-250`). The README mentions the flag
in one sentence (`README.md:81`) and the CLI page in another (`pkg/doc/03-cli.md:73-75`); the Claude Code setup page
does not. The DUST-002 server was registered from the setup page, without it, and every agent pasted the file.

The paste had a second cost besides tokens. A prelude or a paste is an eval, so its top-level `const` and `let` are
local to it. Functions defined in the paste keep them alive through closures, but the agent's later code cannot see
them: one agent's code used the helpers' `r3` rounding function and failed until it wrote `mem.r3 = r3` (diary
4b246f75).

### 5.2 Locations in errors and warnings

All eval code shares one source name. `readFailure` (`native.go:144-187`) takes the first `<eval>:L:C` of the stack
(`stackPos`, `native.go:136`), and `errorNode` maps it to the agent's line by subtracting the wrapper's line
(`agentLine`, `session.go:437-447`). A frame in another source, such as a loaded file, is ignored, so today an error
inside a prelude's function points at the eval line that called it, or at nothing.

A function pasted in eval 1 and called in eval 5 is worse: its frames are also `<eval>` frames, with line numbers of
eval 1's text. `lineTag` and `errorNode` map them against eval 5's lines. When eval 5 is shorter than the line number,
`agentLine` returns 0 and the location is dropped (this is why most of the 13 `UNUSED_NODE` warnings carry no line);
when it is longer, the location names an unrelated line. GCV-005 removes the case for libraries by giving loaded code
its own source name (§7.3); pasting stays possible and stays imprecise (§17, risk 5).

### 5.3 The context block shows names only

`contextBlock` (`processor.go:503-525`) prints `mem` as one line of `key (kind)` pairs, sorted; a function is
`(function)` (`memKind`, `processor.go:527-557`). An agent that did not write the library learns that `mem.faint`
exists but not what it takes or returns. The agents learned it from the playbook and the helper file's header
comment, which they read with the Read tool before pasting.

### 5.4 The renderer: 48 items, 9000 characters, cv.json

| limit | where | effect on row lists |
|---|---|---|
| `maxArrayItems = 48` | `yaml.go:23`, `array` at `yaml.go:167-188` | rows 49+ become "… N more (return cv.json(x) for all)" |
| `maxProperties = 2000` | `yaml.go:22`, `budget` at `yaml.go:162-165` | about 100 items of 20 fields |
| a map with more than 8 keys is block style | `flow`, `yaml.go:252-254` | every detector item is one line per field |
| `MaxResultChars = 9000` | `capText`, `yaml.go:286-303` | the CUT note: "Return less: pick fields, slice arrays, or summarize; return cv.json(x) or cv.csv(x) for complete data." |

Both notes name `cv.json`, which GCV-004 deletes (GCV-004 §6.7). The agents never followed them: they sliced by hand
(`mem.hotByLevel[0.3].slice(45)`, 2b3a8276) or turned rows into strings. With strings, a row of eight values costs
about 45 characters; as a YAML map it costs about 8 × 15 = 120.

### 5.5 UNUSED_NODE

`unusedNodes` (`processor.go:260-316`) treats the following as roots: nodes the eval used (`trace.Used`), nodes in
the returned value, nodes found by rendering `mem` (`processor.go:266-270`) and named images. It reports the last
node of every other chain, with the site's line when the site is eval code.

Two defects follow from the code. First, the warning cannot tell the agent's code from a library's: the 12
`speckColor` warnings of the study were raised for a node built inside `mem.views()` (pasted in eval 1) and dropped
by an agent who used two of the three returned views. The node was not reachable from the returned value or from
`mem`, so the existing reachability rule does not cover it; the node was built by code the agent did not write in
that eval. Second, `mem` is walked with the display renderer, which stops at 48 array items and 2,000 properties: a
node in the 49th element of an array on `mem` is invisible to the walk and reported as discarded.

The thirteenth warning was right: `const s = mem.img.view("speckColor")` on line 1 of an eval that never used `s`
(58f9e866, eval 8).

### 5.6 Sheet and overlay labels

`listItems` (`pkg/cv/ops/sheet.go:113-137`) labels a cell with the item's `id` only when it is a number
(`num(m["id"])`, line 118); otherwise with its list index, from 0. A string id such as `"h17"` is silently replaced by
the index. The `labels` option (`sheet.go:272`) is an enum `id | index | none` whose default depends on the input:
`id` for a Regions node, `index` for a list (`sheet.go:172-190`). An array of `[x, y]` pairs, which the helper built
for every sheet, therefore labels cells 0–14 in every sheet, and the agent must map positions back to rows by hand:

```js
// 851dabe2, evals 4–6: a list of rows, a list of their positions, and an index string per sheet
mem.L = mem.hotByLevel[0.3].filter(r => r.sp >= 0.15)
mem.LP = mem.L.map(o => [o.x, o.y])
return { idx: mem.L.slice(0, 15).map((o, i) => i + ":" + o.id).join(" "), a: mem.sheet(mem.LP.slice(0, 15), 40, "0-14") }
```

`overlay`'s `labels` is `none | id` (`pkg/cv/ops/present.go:18`); with `id` it draws each region's id
(`present.go:66-89`). The sheet's look-number badge over the first label is fixed by the v1 patch (PR #5) and is not
GCV-005's concern.

### 5.7 Morphology exists only inside the speck detector

The van Herk running minimum and maximum (`morph.go:14-46`) and the square opening and closing (`morph.go:56-107`)
are internal to `SpeckBackground` (`specks.go:126-154`). No op exposes an erosion, a dilation or a top-hat. Masks have
`grow` and `shrink` (exact Euclidean, anti-aliased edge; `mask.go:411-449`), and `regions.toMask({ grow })` reuses
`Grow` (`regions.go:355-375`). On a grey image there is no way to remove structures by size or shape.

### 5.8 What the helpers build by hand

The helper library is the clearest specification of what agents needed. Its four central functions, with what each
costs today:

| helper | what it does | cost |
|---|---|---|
| `mem.best(node, x, y, opts)` | measures 21 points (a disk of radius 2 px around (x, y)) with one `measure` call and keeps the highest snr | one measure of 21 points |
| `mem.probe(x, y)` | `mem.best` on the tile (core 3, ring 6–12) and on the speck view; reports `x, y` of the best tile point, its `snr`, the speck view's `step` as `sp`, and `chroma` when snr ≥ 0.5 | 42 point measurements |
| `mem.hot(level, max)` | thresholds the speck view at `level`, labels components (min area 4), probes each centroid (first 300), clamps edge centroids, sorts by `sp`, returns strings | about 0.4 s for 160 probes |
| `mem.faint(opts)` | the same at level 0.06, keeps components of area 5–80 and aspect < 3 not within 4 px of a hot spot, probes them (first 600), keeps snr ≥ 1.4, sorts by snr | about 0.2 s |
| `mem.sheet(points, size, label)` | maps rows or pairs to `{ cx, cy, id }` and calls `cv.sheet` with the tile and the speck view | one sheet |

Read together, `mem.hot` and `mem.faint` are a peak finder: they find candidate places with a global threshold,
refine each to the best point within 2 px, and rank by a local contrast measure. `mem.faint` was mined from an
agent's own eval (2b3a8276, eval 13), which the article calls the best new helper of the project. Its global
threshold at a very low level (0.06) is what makes it find dust in dark areas; its snr filter is what keeps the list
short. `peaks` (§6.8) does the same in one op, with the threshold replaced by the local snr.

### 5.9 The speck view's white point

`speckView` (`pkg/cv/ops/views.go:201-216`) builds a difference of Gaussians (σ s/3 against 4 s/3) and stretches it
with the internal op `levelsAt` (`views.go:221-…`): white = max(5 × the 99th percentile of the receiver's luminance,
1e-4), computed when it runs over the whole receiver. `img.view("speck").crop(b)` therefore keeps the scale of the
whole image, while `img.crop(b).view("speck")` computes the scale over the crop alone. The second form is a local
stretch that already exists; no document says so. The agents' `mem.sweep` reimplemented it with a `stats` call and
`levels` (`84-cv-helpers.js`, `mem.sweep`).

### 5.10 The MCP instructions are full

`Instructions` (`pkg/cvsession/reference.go:16-43`) is 3,940 characters; a test requires fewer than 4,000
(`pkg/cvsession/mcp_test.go:177-178`). GCV-004 rewrites it (GCV-004 §12) without `P`, `show`, `{ space }`,
`cv.json` and the query ops, which frees about 600 characters; GCV-005 needs about 450 of them (§12.1).

## 6. The proposed API

Each subsection shows what an agent or a library author writes before GCV-005 (on GCV-004's API, which is what the
agents' code becomes after GCV-004's migration note) and after, using the agents' real code from
`DUST-002/various/agent-evals/`, then states the rules.

### 6.1 `cv.load`: run a script file in the session

```js
// before: eval 1 of every agent, the helper file pasted as code (2,859–3,418 characters)
mem.T = "/Users/manuel/dustdet/cv_tiles/"
mem.tile = (id, ox, oy) => {
  mem.img = cv.open(mem.T + id + ".tif", { encoding: "linear", origin: [ox, oy], name: "img" })
  …
}
… 90 more lines …
mem.tile("cfca85b41cd6f78f_2048_0", 2048, 0)
return "helpers ready"
```

```js
// after: the library is a file in a --read directory (or loaded as a --prelude)
await cv.load("dust/helpers.js")                      // defines mem.tile, mem.views, mem.sheet, …
mem.tile("cfca85b41cd6f78f_2048_0", 2048, 0)
```

```js
// a pipeline script with arguments: dust/candidates.js returns rows
const rows = await cv.load("dust/candidates.js", { snr: 1.4, radius: [2, 4, 8] })
return rows                                           // a table (§6.5)
```

A script file is an eval body. `dust/candidates.js` could read:

```js
// candidates.js: compact bright spots on the open tile that stand out from their own surroundings
const lin = img.as("linear")
const pk = await lin.peaks({ radius: args.radius ?? 3, snr: args.snr ?? 1 })
return await lin.view("speck").measure(pk, { prefix: "sp" })   // + spStep, spSnr, … per peak
```

Rules:

- `cv.load(path, args)` returns a promise of the file's return value. Write `await cv.load(…)`.
- `path` is resolved like a path given to `cv.open` (GCV-004 §4.6, §7.8): a relative path is looked up in the
  workspace, then in each `--read` directory in flag order, and the first that has the file wins; an absolute path
  must lie in one of them. The file must end in `.js`.
- The file runs as the body of `async function (args) { … }`, starting on the function's first line, so its line
  numbers are its own. Top-level `await` and `return` work; `const` and `let` stay local to the file; `cv`, `mem`, `img`
  and other image names are visible. `args` is the second argument as given (any JavaScript value, not copied), or
  `{}`.
- Every call reads and runs the file again. There is no cache: a second load of a library redefines its functions
  (picking up edits), and a pipeline runs again with its new arguments. Nodes the file builds are hash-consed and
  cached like any node, so a second run of a pipeline costs little.
- A loaded file may load other files, with the same path rule. At most 32 loads may be running at once; the 33rd is
  `E_TOO_MANY`, naming the chain (a file that loads itself, directly or not).
- Errors inside a loaded file name the file and line: `where: dust/helpers.js:57: const p = await mem.probe(cx, cy)`,
  plus `called from: line 1: await mem.hot(0.3)` when the eval called a function the file defined (§7.3).
- Looks created by a loaded file are delivered like the eval's own (the GCV-004 look queue); warnings from its calls
  carry `file:line`.
- If the eval ends while a load it started is still running or has failed without being awaited, the eval gets a
  `LOAD` warning with the fix `await cv.load("…")` and, for a failure, the file's error message and location.
- `cv.load` exists wherever the `cv` module does: the MCP server, `goja-cv eval` and `run`, and xgoja binaries, each
  with its workspace and read directories (in xgoja, the provider's `root` and `read` configuration, GCV-004 §7.8).

### 6.2 `cv.scripts()`: what can be loaded

```js
return cv.scripts()
```

```yaml
|-
  path                   doc
  dust/candidates.js     candidates.js: compact bright spots on the open tile that stand out from their own surroundings
  dust/helpers.js        Helpers for annotator agents on DUST-002 gold tiles: goja-cv v2.
  dust/thread-width.js   The width of a bright thread across a segment (half-height width).
```

Rules: `cv.scripts()` returns an array of `{ path, doc }`, sorted by path, for every `.js` file in the workspace and
the read directories, at most four directories deep, skipping directories whose name starts with `.` and
`node_modules`; at most 500 files are listed. `path` is what `cv.load` accepts; a file shadowed by one with the same
relative path in an earlier root is not listed. `doc` is the file's first comment line: the text of the first line
when it is a `//` comment, or the first non-empty line of a leading `/* … */` block without its `*`; otherwise `""`. It
never throws (like `cv.docs.list`).

### 6.3 `--prelude`: a load at session start

```bash
# before (DUST-002 reference/08): helpers not registered, so every agent pasted them
claude mcp add cv -- goja-cv mcp --root ~/dustdet/cv_out --read ~/dustdet/cv_tiles --read ~/dustds
# after
claude mcp add cv -- goja-cv mcp --root ~/dustdet/cv_out --read ~/dustdet/cv_tiles --read ~/dustds \
  --read ~/code/dust/lib --prelude ~/code/dust/lib/dust/helpers.js
```

Rules: `--prelude FILE` (repeatable, on `eval`, `run` and `mcp`) runs `await cv.load(FILE)` in every new session,
before any agent code, in flag order. The path is operator input, resolved against the process's working directory as
GCV-004 resolves `--image`, and must then lie in the workspace or a read directory; otherwise the command fails with
a usage error that names `--read`. Looks created by a prelude are discarded (GCV-004 §7.7). A failing prelude fails
session creation with `prelude dust/helpers.js:57: <message>`, as today.

### 6.4 `mem` functions in the context block and in results

```yaml
# before: eval 1's context block
context:
  mem: T (string), best (function), hot (function), hotLook (function), probe (function), sheet (function), spots (function), tile (function), views (function), zoom (function)
```

```yaml
# after: the first eval after the library was loaded
context:
  mem: T (string)
  functions: mem.best(node, x, y, opts), mem.faint(opts), mem.hot(level = 0.4, max = 80), mem.probe(x, y), mem.sheet(points, size = 40, label = "sheet"), mem.tile(id, ox, oy), mem.views(), mem.zoom(x, y, half = 24)
  new functions:
    mem.faint(opts): compact faint dots the hot list misses, ranked by snr: { candidates, kept, rows }
    mem.hot(level = 0.4, max = 80): bright blobs of the speck view, probed and sorted by sp
    mem.probe(x, y): the best point within 2 px: { x, y, snr, sp, step, chroma, cover }
    …
# every later eval
context:
  images: img = cfca85b41cd6f78f_2048_0.tif [2048,0,1024,1024] (16-bit TIFF, linear)
  mem: T (string), box (array[4]), hotRows (array[53]), img (#1 Image)
  functions: mem.best(node, x, y, opts), mem.faint(opts), mem.hot(level = 0.4, max = 80), …
```

Rules:

- `functions:` lists every function on `mem` with its signature (§4.2), sorted. Non-function keys stay on `mem:`.
- `new functions:` gives the doc line of each function that was added or replaced since the previous eval's context
  block (a new function object under the same key counts as replaced), so the docs appear once, right after the load.
- A function anywhere in a result renders as its signature and doc line instead of `[Function]`:
  `return mem.probe` gives `mem.probe(x, y): the best point within 2 px: …`, and `return mem` lists them all.
- A function without a comment shows its signature only. A native function shows `name()`.
- Library authors put the doc line as the first comment inside the function (§12.3).

### 6.5 Tables

```js
// before: c71dd0f2 trial 2, eval 10; rows turned into strings, then cut at about 60 of them
return { spots: h.spots, rows: h.rows.slice(30, 90).join(" | ") }
// b54a23af eval 2: the same pattern
return { rows: mem.hotByLevel[0.3].slice(0, 60).map(o => [o.id, o.x, o.y, o.sp, o.snr, o.area, o.aspect].join(" ")) }
```

```js
// after: return the rows themselves
return { rows: mem.hotByLevel[0.3] }
```

```yaml
rows: |-
  id  x     y     sp     snr     chroma  area  aspect  edge
  14  2278  333   1.762  11.954  0.214   61    2.783   false
  3   2573  62    0.614  0.377   -       5557  180.28  false
  13  2085  304   0.399  2.317   0.625   14    1.02    false
  …
  … rows 1–118 of 174 shown. Next: .slice(118, 236) of this array; all: await cv.save("rows.csv", <the array>)
```

Rules:

- An array of at least two flat records (§4.3) renders as a table, at any depth of the result. Columns are the union
  of the records' keys in first-seen order; a missing, `null` or `undefined` value is `-`.
- Numbers use the renderer's format (integers exactly, others with five significant digits); strings are written as
  they are unless they are empty or contain whitespace, `-` or `|`, which are written JSON-quoted; booleans as `true`
  and `false`. Columns are left-aligned and separated by two spaces.
- A table shows as many rows as fit in the result's table budget, two thirds of the result cap (6,000 characters of
  9,000), shared by all tables of one result in render order, and at least five. When rows are left out, the last
  line says which rows are shown, how to get the next page and how to get all of them.
- Arrays of anything else render as today; their cut note names `cv.save` instead of `cv.json`.
- There is no new API. `cv.table`, a `columns` option and a page option do not exist: picking columns is
  `rows.map(({ id, cx, cy, snr }) => ({ id, cx, cy, snr }))`, and paging is `slice`.

### 6.6 `UNUSED_NODE`

```js
// before: 12 of 13 warnings in the study (c71dd0f2 trial 2, eval 2, and 11 others)
mem.tile("c71dd0f2de533739_512_1536", 512, 1536)
const v = mem.views()
return { scene: v.scene.look("scene"), speck: v.speck.look("speck") }
// UNUSED_NODE: [Image #26 view(name: "speckColor")] builds a node and discards it. …
```

After GCV-005 the same eval, with `mem.views` defined by a loaded file, has no warning. The thirteenth case keeps its
warning, because the agent wrote the line:

```js
const s = mem.img.view("speckColor")              // UNUSED_NODE line 1: const s = mem.img.view("speckColor") builds a node and discards it. …
```

Rules: a node is reported only when it was built at a site in the eval's own code (§4.4), was not used, returned,
kept in `mem` (at any depth, §7.6) or bound to an image name, and does not feed a node that was. Nodes built by a
loaded file are never reported; a library author lints a library by running it as an eval (`goja-cv run file.js`),
where its code is the eval's own. There is no `cv.quiet`.

### 6.7 Labels and points in sheets

```js
// before (on GCV-004): parallel arrays to map cells back to rows (851dabe2)
mem.L = mem.hotByLevel[0.3].filter(r => r.sp >= 0.15)
mem.LP = mem.L.map(o => [o.x, o.y])
return { idx: mem.L.slice(0, 15).map((o, i) => i + ":" + o.id).join(" "), a: mem.sheet(mem.LP.slice(0, 15), 40, "0-14") }
```

```js
// after: pass the rows; each cell is labelled with the row's id
const rows = mem.hotByLevel[0.3].filter(r => r.sp >= 0.15)
return { a: cv.sheet(rows.slice(0, 15), { views: [img, img.view("speck")], size: 40, stretch: "cell" }).look("0-14") }
// or with the caller's own text
const named = rows.map(r => ({ ...r, label: "h" + r.id }))      // spread keeps any item link (GCV-004 §4.5)
return cv.sheet(named.slice(0, 15), { views: [img, img.view("speck")] }).look("hot")
```

Rules:

- `cv.sheet(items, …)` accepts what `measure` accepts: a Regions node, linked items, literal boxes and circles, and
  plain points `{ cx, cy }` or `[x, y]` (its `items` argument is declared `items|point[]`, like `measure` and `sample`
  in GCV-004 §7.6). A point is a cell centre; `size` sets the cell.
- A cell's label is the item's `label` field when it has one (a string or a number), else its `id`. Literal items
  without an `id` get their 1-based position, as GCV-004's `items` op assigns. The sheet's `labels` option is
  `"id" | "none"`, default `"id"`; the value `index` is deleted.
- `overlay`'s `labels: "id"` draws the same label: `label`, else `id`.
- The items an array becomes keep each element's `id` and `label` as the array has them, for linked and literal
  elements alike: a renumbered or relabelled item is drawn with its new label (§7.7).

### 6.8 `peaks`

```js
// before: cfca85b4 eval 8; positions read off four 2× quarter looks with rulers, then probed one by one
const C = [[2092,293],[2075,262],[2100,270],[2104,222],[2306,203], …, [2745,725]]   // 32 guesses
const out = []
for (const [x, y] of C) { const p = await mem.probe(x, y); out.push([x, y, "->", p.x, p.y, "sp", p.sp, "snr", p.snr].join(" ")) }
return out
// 28 of 32 came back at sp below 0.06: the guesses were beside the dots
```

```js
// after: ask for the spots
const lin = img.as("linear")
const pk = await lin.crop([2048, 0, 512, 512]).peaks({ snr: 1 })   // the top-left quarter; sub-pixel cx, cy
return { n: pk.length, top: pk.slice(0, 40), sheet: cv.sheet(pk.slice(0, 15), { views: [img, img.view("speck")] }).look("TL") }
```

```js
// before: 2b3a8276 eval 13, the agent's own faint pass (the origin of mem.faint)
const items = (await mem.spots(0.06)).items; const out = []
for (const it of items) { if (it.area < 5 || it.area > 80 || it.aspect > 3) continue; if (it.cx > 2380 && it.cy < 2300) continue; out.push(it) }
const rows = []
for (const it of out.slice(0, 120)) { const p = await mem.probe(it.cx, it.cy); if (p.snr >= 1.4) rows.push([Math.round(p.x), Math.round(p.y), p.sp, p.snr, it.area, it.aspect].join(" ")) }
return { n: items.length, c: out.length, rows }
```

```js
// after
const pk = await img.as("linear").peaks({ snr: 1.4 })
return pk.filter(p => !(p.cx > 2380 && p.cy < 2300))                // the agent's region filter stays JavaScript
```

```js
// mixed sizes, and the speck view's step (the agents' "sp") for each peak
const lin = img.as("linear")
const pk = await lin.peaks({ radius: [2, 4, 8], snr: 1 })         // overlapping peaks: the higher snr wins
const rows = await lin.view("speck").measure(pk, { prefix: "sp" }) // the peaks with spStep, spSnr, … added
return rows.filter(r => r.spStep >= 0.15)
```

Rules:

- `x.peaks({ radius, snr, polarity })` is an image method that returns a Regions node. On a colour image it uses the
  luminance (GCV-004's one luminance, `OpDef.Luma`), in the receiver's space; on a one-channel image, the image
  itself. So `img.peaks()`, `img.as("linear").peaks()`, `img.toGray().peaks()` and `img.view("speck").peaks()` all
  work and mean what they say.
- `radius` (doc px, default 3) is the core radius r of the spots sought: a number, or a list of numbers to search
  several sizes. Each radius is in [1, 64].
- `snr` (default 1) is the smallest |snr| kept. Random places on the DUST-002 tiles are 90 % below 0.45 and 99 % below
  0.8 (`DUST-002/scripts/87-cv-measure-auc.py`, with the helpers' ring); labelled dust has a median of 1.7.
- `polarity` is `"bright"` (default: maxima of the step), `"dark"` (minima) or `"both"`.
- The result is a Regions node of disks (GCV-004's `Disk` shape): its awaited items have, in this order,
  `id, x, y, w, h, cx, cy, r, coreMean, ringMean, step, spread, snr, ringCoverage`. `cx, cy` is the sub-pixel centre;
  `r` the radius it was found at; `x, y, w, h` the disk's box; the other fields are `measure`'s core/ring fields for a
  core of radius r and a ring [1.5 r, 2.5 r] on the gray plane peaks searched. Items are sorted by |snr|, highest
  first, and `id` is the rank (1 is the strongest).
- Peaks never overlap: when the cores of two peaks overlap (their distance is less than the sum of their radii), only
  the one with the higher |snr| is kept. With one radius this makes peaks at least 2 r apart; with several, a large
  spot is one peak at a large radius rather than a ring of small ones.
- Peaks feed every op that takes items: `cv.sheet(pk)`, `img.overlay(pk, { labels: "id" })`, `x.measure(pk)` (which
  sizes its core and ring from each peak's `r`, GCV-004 D17), `img.crop(p)` (the disk's box), `img.mask.regions(pk)`.
  A filtered or sliced array of peaks keeps its links.
- Peaks are computed at analysis scale 1, like every Regions op, so ids and coordinates do not depend on a look's scale.
- On a view that clips at zero, such as `view("speck")`, most rings are flat zero, the spread falls to the 1e-3 floor
  and the snr is not meaningful; there `step` (the speck view's step, the agents' `sp`) is the quantity to rank by.
  Peaks look for spots on the image (`img.as("linear").peaks()`), and the speck view measures them (the example above).

### 6.9 Morphology

```js
// before: no way to keep a saturated blob filled; the speck view makes it a ring (b54a23af, eval 3)
return { pr1: await mem.probe(1262, 892) }                    // sp 0.209, snr 0.61 on the tile's most obvious blob
```

```js
// after: a top-hat keeps bright things smaller than the disk, at full height
const L = img.as("linear").toGray()
const blobs = L.sub(L.opening(12))                            // structures that a disk of radius 12 does not fit into
const pk = await blobs.peaks({ radius: [2, 4, 8], snr: 1 })   // or: blobs.mask.luminance({ above: t }).components()
```

```js
// before: line fragments dominate the hot list (21661036, 8af4cabe); agents filtered in JavaScript
mem.hotByLevel[0.3].filter(o => o.sp >= 0.15 && o.aspect < 3)
// after: remove lines longer than 21 px before thresholding; one opening per direction, combined with max
const s = img.view("speck")
const lines = [0, 22.5, 45, 67.5, 90, 112.5, 135, 157.5]
  .map(a => s.opening(10, { line: a }))
  .reduce((m, o) => m.max(o))                                 // what contains a 21 px segment in some direction
const compact = s.sub(lines)                                  // the speck view without lines
const spots = await compact.mask.luminance({ above: 0.3 }).components({ minArea: 4 })
```

```js
// before (GCV-004): mask ops grow and shrink, and mask.regions({ grow })
const m = img.mask.luminance({ above: 0.9 }).grow(3)
const around = img.mask.regions(spots, { grow: 2 })
// after: the morphology family; the same pixels on a mask
const m = img.mask.luminance({ above: 0.9 }).dilate(3)        // .dilate(3).feather(1) for a soft edge
const around = img.mask.regions(spots).dilate(2)
```

Rules:

- `x.erode(r, { line })`, `x.dilate(r, { line })`, `x.opening(r, { line })`, `x.closing(r, { line })`: image methods
  on a one-channel receiver (a mask, a grey image, a view, a top-hat). A colour receiver is `E_WRONG_KIND` with the fix
  `x.toGray().erode(r)`.
- `r` is a length in doc px (positional, 0 to 500; 0 returns the receiver). Without `line`, the structuring element is
  the disk of radius r (§4.7); with `line: angle` (degrees, counter-clockwise from the x axis, any number), it is the
  line of length 2 r + 1 doc px through the pixel at that angle.
- Results are in the receiver's space and encoding (morphology picks values, it does not compute new ones); the
  receiver is read as stored, like every one-channel op in GCV-004.
- Pixels outside the image do not take part: near the border the element is clipped.
- The top-hat is not an op: `x.sub(x.opening(r))` (white) and `x.closing(r).sub(x)` (black). `cv.help("tophat")`
  returns a concept entry with both.
- `grow`, `shrink` and `mask.regions`'s `grow` option are deleted. On a mask (values 0 and 1), `m.dilate(r)` and
  `m.erode(r)` select the same pixels as `grow(r)` and `shrink(r)` did, without their one-pixel anti-aliased edge;
  `.feather(1)` gives a soft edge.

### 6.10 The speck view's scale

No option is added. The view's documentation (`pkg/cv/docs/topics/11-views.md` and the op's `Doc`) gains:

> The speck view's white point is 5 × the 99th percentile of its high-pass over the image it is computed on. On a
> tile with strong scene edges (windows, cables, lattices) the edges set it, and dust can map to 3–10 % of white.
> `img.view("speck").crop(b)` keeps the whole image's scale; `img.crop(b).view("speck")` sets the scale from the crop
> alone (a local stretch). To find faint dust, do not lower thresholds on the view: use
> `img.as("linear").peaks({ snr })`, whose snr compares each spot with its own surroundings. A large saturated speck
> has a flat top, which the view turns into a ring: use a top-hat (`L.sub(L.opening(r))`) or peaks with several radii.

### 6.11 Not in scope

| ask | why not |
|---|---|
| `eval_file` MCP tool | `cv.load` is one mechanism for MCP, CLI and xgoja; an `eval_file` call would save about 20 characters per call over `eval({ code: 'await cv.load("x.js")' })`, cannot be combined with other code in one eval, and needs JSON args where `cv.load` takes any JavaScript value (D1). |
| a `--scripts DIR` flag | scripts live in the workspace and `--read` directories like every other file (D4). |
| `cv.require` | `cv.load` returns the file's value; a file that wants another file's functions loads it. |
| `look({ grid, gridLabels })` | a labelled grid is a better ruler, and a model still reads a cell name off a picture; `peaks` returns coordinates as data (§3.4). |
| `crop(box, { fit: "shift" })` | the 7 `RANGE` warnings are informative and the clip is correct; a library clamps its boxes in one line (`x = Math.min(Math.max(x, bx), bx + bw - w)`), and sheets already draw edge cells partly grey. Not free: a second meaning of a crop box. |
| `components({ maxAspect })`, an `elongation` field | `items.filter(r => r.aspect < 3)` is the one way to filter (GCV-004); line removal before thresholding is morphology (§6.9). |
| `cv.quiet(node)` | attribution (§6.6) removes the false warnings at their cause. |
| `cv.table(rows, { columns })` | tables are automatic (§6.5); columns and pages are JavaScript. |
| `measure.line(path, { width })` | a recipe with `profile` measures a thread's width (§12.4); the annotation tool now measures widths itself. |
| `view("speck", { white: "p99" \| "noise", k })` | the noise scale measured worse (87 % against 91 %); `dog().levels({ white })` and `img.crop(b).view("speck")` already give other scales; local normalization is `peaks`' snr (D21). |
| `median(r)` and other rank filters | no evidence in this study; the top-hat covers "background without the speck" for bright structures, and the speck detector keeps its own morphological background. A median would reuse the disk element of §10.9 with a sliding histogram; it waits for a use (D20). |
| `stretch: "sheet"` (one range for all cells) | raised in the article, not in the hand-off's issues; agents learned to read snr instead of trusting per-cell stretch. Left for later. |

### 6.12 The GCV-005 surface in one table

| call | returns | notes |
|---|---|---|
| `cv.load(path, args)` | Promise of the file's return value | workspace, then `--read` directories; `.js`; the file is an async function body with `args` |
| `cv.scripts()` | array of `{ path, doc }` | never throws |
| `x.peaks({ radius, snr, polarity })` | Regions (disks) | radius number or list (doc px, default 3); snr default 1; polarity bright/dark/both |
| `x.erode(r, { line })`, `x.dilate(r, { line })`, `x.opening(r, { line })`, `x.closing(r, { line })` | Image, 1 channel | receiver: one channel; disk, or a line at `line` degrees |
| `cv.sheet(items, { views, size, zoom, labels, max, stretch })` | Image (picture) | items: Regions, items, boxes, circles or points; `labels: "id" \| "none"`; label = `label` else `id` |
| `img.overlay(items, { labels })` | Image (display) | `labels: "id"` draws `label` else `id` |
| `--prelude FILE` | | a `cv.load` at session start; the file must lie in a root |

Removed: `m.grow(r)`, `m.shrink(r)`, `mask.regions(items, { grow })`, the sheet's `labels: "index"`.

## 7. Proposed architecture

### 7.1 Overview

```
 agent code ──eval──► session (cvsession)                          JS module (gojamod/cv)
                       __cv_run(async function () { … })            cv.load(path, args) ─► native.__load
                         │                                            │  Workspace.Open(path) (GCV-004)
                         │                                            │  goja.Compile(name, "(async function (args) {" + text + "\n})")
                         │                                            │  RunProgram → fn; fn(args) → promise
                         │                                            │  track.loads += {name, site, promise}
                         │                                            ▼
                         │                       nodes built in the file: Site{Source: name, Line}
                         ▼
                       processor.Process
                         ├─ render value ── renderer: tables (flat-record arrays), functions as signature + doc
                         ├─ warnings ─────── lineTag: "line N" (eval) | "name:N" (file)
                         │                   UNUSED_NODE: only sites in the eval's own code; mem walked uncapped
                         │                   LOAD: loads pending or failed and not awaited
                         ├─ errors ───────── where: innermost eval-or-file frame; called from: innermost eval frame
                         └─ context block ── mem, functions (signatures), new functions (doc lines)

 x.peaks(opts) ──► graph op "peaks" (Image → Regions, Luma, AnalysisScale 1)
                     └─ kernels.Peaks: step map (disk chord sums) → NMS (DiskMinMax) → sub-pixel fit
                                       → ProbeCoreRing at each candidate → snr filter → overlap rule
 x.erode/dilate/opening/closing(r, { line }) ──► graph ops (one channel → one channel)
                     └─ kernels.DiskMinMax (chords + van Herk; binary fast path: DistanceToSet) | kernels.LineMinMax
```

| layer | removed | added or changed |
|---|---|---|
| `pkg/cv/kernels` | `Grow`, `Shrink` | `DiskMinMax`, `LineMinMax`, `diskSums`, `Peaks`, `PeakOptions`, `Peak` |
| `pkg/cv/ops` | `grow`, `shrink` (`lengthMaskOp` for them), `mask.regions`' `grow`, the sheet's `index` labels | `peaks.go` (op `peaks`), `morph.go` (ops `erode`, `dilate`, `opening`, `closing`); sheet and overlay labels from `label` else `id`; sheet items `items\|point[]` |
| `pkg/cv/graph` | | the `items` op carries each entry's `id` and `label` (GCV-004 §7.6, extended) |
| `pkg/gojamod/cv` | | `cv.load`, `cv.scripts` (prelude.js), `binding.load`, `binding.scripts`, loads in the eval trace, source registry of loaded files, `DescribeFunction`; help concept entries `load`, `scripts`, `tophat` |
| `pkg/cvsession` | prelude file reading in `runPreludes`; `memKind`'s "function"; the display-renderer walk of `mem` | preludes as `cv.load` evals; file frames in `readFailure`, `errorNode`, `lineTag`; attribution in `unusedNodes`; `collectNodes`; `LOAD` warning; tables and functions in the renderer; context block functions; instructions |
| `cmd/goja-cv` | | `--prelude` must lie in a root (usage error) |

### 7.2 `cv.load` in the module

`cv.load` is a module function, so it is defined in `prelude.js` beside `cv.open` (`pkg/gojamod/cv/prelude.js:290-336`)
and implemented by a native function registered in `binding.object` (`binding.go:104-130`), which GCV-004 wraps in the
panic boundary (`export`, `pkg/gojamod/cv/export.go`):

```js
// prelude.js, in the cv object
// load(path, args) runs a script file (workspace, then --read directories) as the body of
// async function (args) { … } and returns a promise of its return value.
load(path, args) {
  return native.__load(path, args === undefined ? {} : args);
},
// scripts() lists the .js files cv.load can run: [{ path, doc }]
scripts() {
  return native.__scripts();
},
```

`args` is passed through untouched: no `marshal`, so nodes, linked items and functions arrive as the same JavaScript
values.

On the Go side, `binding.load` reads the file through the engine's workspace (GCV-004 `EngineOptions.Workspace`,
§7.8), compiles it under a name, and runs it:

```go
// loadRec is one cv.load call of the current eval.
type loadRec struct {
    Name    string        // the file's name in stacks and locations: its path relative to its root
    Site    Site          // the call site (an eval line, or file:line for a nested load)
    Promise *goja.Promise // the file function's promise
}

// sourceFile is the text of a loaded file, for "where:" lines.
type sourceFile struct{ lines []string }
```

The binding keeps `files map[string]sourceFile` (the last text loaded under each name) for the whole session, and the
eval tracker keeps the eval's `[]loadRec`. Section 10.1 has the full function. Three details matter:

- **The name.** A file's name is its path relative to the root it was found in (`dust/helpers.js`), which is also
  what the agent passes to `cv.load` and what `cv.scripts` lists. Two roots may hold the same relative name; since
  relative `cv.load` paths resolve to the first, the name identifies the file that ran. An absolute path is named by
  its root-relative part.
- **Line numbers.** The wrapper `(async function (args) {` is prepended to the file's first line, without a newline,
  so line N of the compiled source is line N of the file. Only columns on line 1 are shifted.
- **Running.** `goja.Compile(name, src, false)` returns a program; `vm.RunProgram` evaluates it to the function;
  calling the function with `args` starts the body and returns its promise. The body runs synchronously until its
  first `await`, so a library that only defines functions has defined them all when `cv.load` returns.

`cv.scripts` walks each root's file system (`os.Root.FS()` with `fs.WalkDir`, depth ≤ 4) and reads the first line or
comment block of each `.js` file (§6.2). It is implemented on the `Workspace`, so it shares the root list and the
shadowing rule with `cv.open`.

### 7.3 Locations in loaded files

Code from a loaded file has the file's name as its source, so every place that turns a stack frame into a location
learns one more case.

- **Sites.** `binding.site` (`evaltrack.go:149-160`) already returns the first frame outside the cv prelude and
  native code, with `Source` = `f.SrcName()`. For a file it is the file's name. Nothing changes here.
- **Warnings.** `lineTag` (`processor.go:251-258`) gives "CODE line N: " for an eval site and "CODE dust/helpers.js:57: "
  for a file site whose name is in the binding's file registry.
- **Errors.** `readFailure` (`native.go:144-187`) reads every frame of the stack in order, not only the first
  `<eval>` one. The **location** is the first frame whose source is `<eval>` or a loaded file; when that frame is in a
  file, **called from** is the first `<eval>` frame. `errorNode` writes both:

```yaml
error:
  code: TypeError
  message: 'Cannot read property "cx" of undefined'
  where: 'dust/helpers.js:57: const p = await mem.probe(it.cx, it.cy)'
  called from: 'line 1: const h = await mem.hot(0.3)'
  hint: …
```

- **Syntax errors in a file.** `goja.Compile` fails before anything runs. `binding.load` throws a cv error with code
  `SyntaxError` and the message `dust/helpers.js:12:5: <goja's message>`; the error's `where` is the eval line that
  called `cv.load`.
- **Preludes.** `whereLine` (`session.go:223`) accepts both `line N` and `name:N`, so a failing prelude still reads
  "prelude dust/helpers.js:57: message".

### 7.4 The session: preludes, loads, the context block

**Preludes.** `Options.Preludes` keeps the operator's paths; `withDefaults` resolves each against the working
directory and requires it to lie in the workspace or a read directory, returning the root-relative name (usage error
otherwise, §11). `runPreludes` (`session.go:226-250`) stops reading files: for each prelude it evaluates

```js
await cv.load("dust/helpers.js")
```

through `App.Evaluate`, as an eval whose looks are discarded (GCV-004 §7.7). The failure message is built as today.

**The `LOAD` warning.** `Process` and `ProcessFailure` read the eval trace's loads. For each load whose promise is
still pending, or rejected while the eval itself succeeded or failed with another error, the processor adds:

```
LOAD line 2: cv.load("dust/helpers.js") had not finished when the eval ended. fix: await cv.load("dust/helpers.js")
LOAD line 2: cv.load("dust/helpers.js") failed: dust/helpers.js:57: mem.img is undefined. fix: await cv.load("dust/helpers.js")
```

No promise plumbing is added: `goja.Promise.State()` and `Result()` give everything. An agent that awaited the load
and caught its error with `try` also gets the second warning; that is acceptable, since it names an error the agent
already saw.

**The context block.** `contextBlock` (`processor.go:503-525`) splits `mem`'s keys into functions and the rest. The
rest stays on `mem:` with `memKind`. Functions go on one `functions:` line as signatures (`mem.probe(x, y)`), sorted.
The session state keeps `fnSeen map[string]*goja.Object`, the function object last listed under each key; a key whose
object differs from the remembered one (new, or replaced by a reload) is listed again under `new functions:` with its
doc line, and the map is updated. Signatures and doc lines come from `cvmod.DescribeFunction` (§10.4), which the
renderer uses too.

### 7.5 The renderer: tables and functions

`renderer.array` (`yaml.go:167-188`) first asks `tableOf(obj)`: are there at least two elements, and is every element
a plain object (exported type `map[string]any`, not a node, look, error, provider or function) whose own enumerable
string-keyed values are all primitives? If so it calls `r.table(obj)`, which returns a literal-style scalar node
(`yaml.Node{Kind: ScalarNode, Style: LiteralStyle, Tag: "!!str"}`) with the table text (§10.3). Otherwise it renders as
today, with the note "… N more (await cv.save("x.json", <the array>) for all)".

The renderer gains one field, `tableLeft int`, the characters left for tables in this result, initialized from the
session's `MaxResultChars` × 2 / 3. Tables consume it in render order. The values are read with `obj.Get(key)` on plain
objects only; getters are not run, because a plain object exported as `map[string]any` has none that matter, and the
check rejects anything else.

Functions render through `DescribeFunction`: `str("mem.probe(x, y): the best point within 2 px: …")` when the
function's name is known from the key path, else its own `name` property.

### 7.6 `UNUSED_NODE`: attribution and an uncapped walk

`unusedNodes` (`processor.go:260-316`) changes in two places.

1. The roots from `mem` come from `collectNodes(vm, mem)`, a walk over own enumerable properties and array elements
   with a visited set and no item or property cap. It collects node ids (`cvmod.NodeOf`) and does not descend into
   nodes, looks, functions or proxies. Linked items are plain objects and are walked like any other.
2. A created node is reported only if `c.Site.Source` is the eval's source name (the value `f.SrcName()` returns for
   `vm.RunString` code, `""` today; a constant `cvmod.EvalSource` names it). Sites in loaded files are skipped, like
   nodes on builder placeholders (`onPlaceholder`).

The message changes with GCV-004 (no `show()`): "builds a node and discards it. Nodes are lazy: nothing ran. Return
it, await it, look at it, or keep it in mem."

### 7.7 Labels through the `items` op

GCV-004's `items` op (GCV-004 §7.6) turns an array into a Regions node. Its entries carry `{ index, x, y, w, h }` for
linked elements and the geometry (with `id` and `r` when given) for literal ones. GCV-005 adds two fields to every
entry:

- `id`: the element's `id` as the array has it (linked elements always have one; a literal element without one gets
  its 1-based position, as GCV-004 says). The resulting region's `ID` is this value, so a renumbered linked item is
  drawn and reported with its new id. Ids are data (GCV-004 D6); the link is the position.
- `label`: the element's `label` field when it is a string or a number (numbers are formatted as the renderer formats
  them). It is stored in the region's `Tags["label"]` (`Region.Tags` exists for detector tags such as `polarity`,
  `pkg/cv/ops/detect.go:38-40`), and the items node's schema gains an optional string field `label` when any entry has
  one.

`sheet` and `overlay` draw `Tags["label"]` when present, else the id. For the sheet's plain-list path (points and
boxes, `listItems`, `sheet.go:113-137`), the same rule reads `label` and `id` from each map, accepting string ids.
The `labels` enum of the sheet becomes `id | none` with default `id`; the default no longer depends on the input.

### 7.8 The `peaks` op and kernel

**The op** (`pkg/cv/ops/peaks.go`):

```go
var peakFields = []graph.FieldSpec{ // the flat item schema of a peak, in order
    intField("id", "rank by |snr|, 1 = strongest"),
    numField("x", graph.DocPx, "box of the disk: cx − r"), numField("y", graph.DocPx, "cy − r"),
    numField("w", graph.DocPx, "2 r"), numField("h", graph.DocPx, "2 r"),
    numField("cx", graph.DocPx, "sub-pixel centre x"), numField("cy", graph.DocPx, "sub-pixel centre y"),
    numField("r", graph.DocPx, "the radius the peak was found at (core radius)"),
    // then coreRingFields("off"): coreMean, ringMean, step, spread, snr, ringCoverage
}

graph.OpDef{
    Name: "peaks", On: graph.KindImage, Returns: graph.KindRegions, Luma: true, // GCV-004: the luminance input
    Args: []graph.ArgSpec{
        {Name: "radius", Type: "number|number[]", Unit: graph.DocPx, Default: 3.0, Min: graph.F(1), Max: graph.F(64),
            Usual: [2]float64{1.5, 16}, Doc: "core radius of the spots sought, or a list of radii to search several sizes"},
        {Name: "snr", Type: "number", Default: 1.0, Min: graph.F(0), Usual: [2]float64{0.5, 5},
            Doc: "smallest |snr| kept: step / ring spread (random places: 90 % below 0.45)"},
        {Name: "polarity", Type: "enum:bright,dark,both", Default: "bright", Doc: "maxima of the step, minima, or both"},
    },
    AnalysisScale: func(graph.Args) float64 { return 1 },
    Halo:          func(a graph.Args) float64 { return 2.5*maxRadius(a) + 2 },
    Fields:        fixedFields(append(peakFields, coreRingFields("off")...)),
    Run:           runPeaks, // §10.6
    Doc: "Local maxima of the core/ring step (measure's step) at sub-pixel precision, as disks: …",
    See: []string{"measure", "sheet", "detect.specks", "opening"},
}
```

**The kernel** (`pkg/cv/kernels/peaks.go`):

```go
// PeakOptions configures Peaks. Radii are in buffer px.
type PeakOptions struct {
    Radii    []float64
    MinSNR   float64
    Polarity string // "bright" | "dark" | "both"
}

// Peak is one peak: its sub-pixel centre (buffer px, pixel i's centre at i + 0.5), the radius it was found at,
// and the core/ring measure there.
type Peak struct {
    CX, CY, R float64
    CoreRing  CoreRing
}

func Peaks(ctx context.Context, L []float32, w, h int, o PeakOptions) ([]Peak, error)
```

`Peaks` uses three helpers: `diskSums` (sum and pixel count of a plane over the digital disk of radius ρ around every
pixel, by chords over row prefix sums, §10.7), `DiskMinMax` (the morphology kernel, used here for non-maximum
suppression) and `ProbePoint` (`probe.go:198`, the exact core/ring measure). `runPeaks` converts doc px to buffer px
at scale 1 (the same numbers, offset by the frame's origin), calls `Peaks`, and builds a `RegionsValue` of `Disk`
regions (GCV-004's `Region.Shape`) with `ID` = rank and the measures in `Region.Measures`.

### 7.9 The morphology ops and kernels

**Kernels** (`pkg/cv/kernels/morph.go`, beside `minMax1D`):

```go
// DiskMinMax is the grey erosion (isMax false) or dilation (isMax true) of a w×h plane by the digital disk
// {dx² + dy² ≤ r²}; pixels outside the plane do not take part.
func DiskMinMax(ctx context.Context, p []float32, w, h int, r float64, isMax bool) ([]float32, error)

// LineMinMax is the same with the digital line of half-length r at angle theta (radians).
func LineMinMax(ctx context.Context, p []float32, w, h int, r, theta float64, isMax bool) ([]float32, error)
```

`DiskMinMax` decomposes the disk into horizontal chords and runs `minMax1D` once per chord and output row (§10.9). When
every value of the plane is 0 or 1 and r ≥ 8, it computes the same result from `DistanceToSet` (§10.11), whose cost
does not grow with r. The existing `SquareMinMax`, `Opening` and `Closing` stay, renamed `squareMinMax`,
`squareOpening`, `squareClosing` (unexported; the speck background is their only user).

**Ops** (`pkg/cv/ops/morph.go`), one constructor for the four:

```go
func morphOp(name, doc string, run func(ctx context.Context, p []float32, w, h int, r float64, line *float64) ([]float32, error), halo float64) graph.OpDef {
    return graph.OpDef{
        Name: name, On: graph.KindImage, Returns: graph.KindImage, OneChannel: true, // GCV-004 §7.5
        Args: []graph.ArgSpec{
            {Name: "radius", Type: "number", Unit: graph.DocPx, Positional: true, Min: graph.F(0), Max: graph.F(500),
                Usual: [2]float64{1, 50}, Doc: "radius of the disk, or half-length of the line, in doc px"},
            {Name: "line", Type: "number", Optional: true, Doc: "angle in degrees (counter-clockwise from x): a line element instead of a disk"},
        },
        Identity: func(a graph.Args) bool { return a.DocLength("radius") == 0 },
        Halo:     func(a graph.Args) float64 { return halo * a.DocLength("radius") },
        Run:      /* buffer px radius = a.Length("radius", scale); line angle from a.Has("line") */,
        Doc:      doc,
        See:      []string{"erode", "dilate", "opening", "closing", "feather"},
    }
}
// erode: run = min (halo 1); dilate: max (halo 1); opening: max(min(·)) (halo 2); closing: min(max(·)) (halo 2)
```

The ops follow their receiver's space (GCV-004's `Follows`, the zero value) and keep its encoding (the default
`KeepEnc` result). One-channel receivers are read as stored (GCV-004 §4.4), so a mask is never converted.

### 7.10 The instructions

`Instructions` (`reference.go:16-43`) is replaced by the text of §12.1 (3,397 bytes), which covers GCV-004's v2 rules
and GCV-005's additions. `EvalToolDescription` (`reference.go:46`) gains one clause: "Task libraries: await
cv.load(path, args); cv.scripts() lists them." The MCP test keeps the 4,000-character bound.

## 8. Decision records

### D1. `cv.load`, not an `eval_file` MCP tool

- **Context:** The paste cost 2,859–3,418 characters of agent output per session (29 pastes) and broke code when
  trimmed. The hand-off asked for `eval_file({ path, sessionId, args })`, an MCP tool that runs a file.
- **Options considered:** (a) an `eval_file` MCP tool; (b) `cv.load(path, args)` in the `cv` module; (c) both, with
  `eval_file` a thin wrapper around `cv.load`.
- **Decision:** (b).
- **Rationale:** The token saving is in not pasting, which both do. Between them the difference is the wrapper: an
  eval with `await cv.load("dust/helpers.js")` is about 40 characters of code, an `eval_file` call about 20 fewer.
  Against that, (a) exists only in the MCP server, not in `goja-cv eval` or xgoja; it cannot be combined with other
  code in one eval (load, then call, then return a look); its `args` must be JSON, while `cv.load` passes any
  JavaScript value, including nodes and linked items; and it is a second way to run code. (c) adds a tool for 20
  characters.
- **Consequences:** One mechanism in every front end. The MCP instructions must teach `cv.load` (§12.1), since agents
  do not discover functions by themselves.
- **Status:** proposed.

### D2. A script file is an async function body with `args`

- **Context:** Libraries define functions on `mem`; pipelines compute and return. A file's scope must serve both
  without a second mechanism.
- **Options considered:** (a) a module with `exports` (`const h = await cv.load("x.js"); h.probe(…)`); (b) a script
  run at global scope; (c) the body of `async function (args) { … }`, exactly an eval's semantics plus `args`.
- **Decision:** (c).
- **Rationale:** (c) is what agents already know: an eval. A library sets `mem.*` (as the DUST-002 helpers do,
  unchanged) or returns an object of functions (a module, if the author prefers); a pipeline awaits and returns. (a)
  forces every library into one shape and breaks the existing helpers. (b) leaks `const` and `let` into the global
  scope of the session, where they collide between loads and are invisible in the next eval anyway, and forbids top-level
  `await`.
- **Consequences:** Line numbers equal file lines (the wrapper shares line 1). A library that wants once-only
  behaviour writes `if (mem.ready) return`.
- **Status:** proposed.

### D3. No cache: every `cv.load` runs the file

- **Context:** Node's `require` caches modules. A second load of a library could be skipped.
- **Options considered:** (a) cache by name, run once per session; (b) cache by name and file stamp; (c) always run.
- **Decision:** (c).
- **Rationale:** A pipeline with arguments must run again with new arguments, so (a) and (b) need an exception for
  pipelines, which is a second mode. Re-running a library costs microseconds (it defines functions), picks up edits
  while a library is being developed, and builds no new work: nodes are hash-consed and cached. (b) also needs a stat
  per load.
- **Consequences:** `cv.load` in a loop runs the file each time; the docs say so.
- **Status:** proposed.

### D4. One path rule for every file goja-cv reads

- **Context:** The hand-off proposed a `--scripts DIR` flag and resolving nested loads relative to the loading file.
- **Options considered:** (a) a separate scripts directory; (b) paths relative to the loading file (Node's rule);
  (c) GCV-004's rule for `cv.open`: workspace first, then each `--read` directory in order.
- **Decision:** (c), for `cv.load`, nested loads, `cv.scripts` and `--prelude` alike.
- **Rationale:** One rule, already implemented with `os.Root` (GCV-004 D11), with its containment and error messages.
  (a) adds a flag and a third kind of directory. (b) gives the same string two meanings depending on where it is
  written, and makes a file's location matter for its behaviour.
- **Consequences:** A library that loads a sibling names it with its root-relative path (`cv.load("dust/common.js")`).
  Task libraries are served by adding their directory with `--read`.
- **Status:** proposed.

### D5. `--prelude` is a load at session start

- **Context:** `--prelude` already exists and reads files outside the roots.
- **Options considered:** (a) keep the prelude as raw eval text, outside the roots; (b) make it `await cv.load(FILE)`
  at session start, the file inside a root.
- **Decision:** (b).
- **Rationale:** One mechanism: a prelude gets file:line locations, the `LOAD` and attribution rules, and appears in
  `cv.scripts`; nested loads from a prelude find their siblings. The cost is that the operator must name the
  library's directory with `--read`, which the usage error says.
- **Consequences:** `runPreludes` stops reading files; `Options.Preludes` is validated against the workspace (§7.4).
- **Status:** proposed.

### D6. Scripts are listed by a function, not in the context block

- **Context:** Agents never call `cv.docs`; a listing they have to ask for may never be seen.
- **Options considered:** (a) list the scripts in every context block; (b) list them in the first eval's context
  block; (c) `cv.scripts()`, named in the MCP instructions and the eval tool description.
- **Decision:** (c).
- **Rationale:** The agents of the study never needed to discover a library: the playbook named it. Discovery matters
  for an agent without a playbook, which reads the instructions; one sentence there points at `cv.scripts()`. (a)
  repeats a list of files in every result (and read directories can be large); (b) is a special case of the context
  block.
- **Consequences:** The instructions name `cv.scripts()`; the functions a loaded library defines are shown by the
  context block (D8), which is where an agent with a playbook looks next.
- **Status:** proposed.

### D7. A function's doc line is the first comment inside its source

- **Context:** The context block should say what each `mem` function does. The doc has to come from somewhere.
- **Options considered:** (a) the comment above the definition (JSDoc style); (b) a property convention
  (`mem.probe.doc = "…"`); (c) the first comment inside the function's source text.
- **Decision:** (c).
- **Rationale:** goja's `Function.prototype.toString` returns the function's exact source, comments included (§4.2),
  for functions defined anywhere: in a loaded file, a prelude or an eval. (a) needs the position of the definition in
  its file, which goja does not expose; finding the function's text in the file is fragile. (b) is a second statement
  the author must keep in sync and agents forget. The convention of (c) costs one comment at the start of the body,
  which the DUST-002 helpers already almost have (their header block documents each function).
- **Consequences:** The task-library topic (§12.3) states the convention; functions without a comment show their
  signature only.
- **Status:** proposed.

### D8. Doc lines appear once, when a function is new

- **Context:** Ten functions with docs cost about 800 characters. The context block follows every eval: over the
  study's 345 evals that is about 280 KB of repeated text.
- **Options considered:** (a) docs in every context block; (b) names only, docs on request; (c) signatures always,
  doc lines when a function is new or replaced.
- **Decision:** (c), with `return mem` (or `return mem.f`) showing the doc lines on request.
- **Rationale:** An agent needs a function's doc when the function appears, right after the load; afterwards the
  signature reminds it of the arguments. (a) pays the full cost every time; (b) hides the docs exactly when they are
  needed.
- **Consequences:** The session remembers, per key, the function object it last listed.
- **Status:** proposed.

### D9. Tables are automatic, as a literal block

- **Context:** Row lists are the agents' most common result and were cut at 9,000 characters; the hand-off proposed
  `cv.table(rows, { columns })` or a better cut note.
- **Options considered:** (a) `cv.table`; (b) a YAML flow sequence per row with a header row (`- [id, cx, cy]`,
  `- [1, 2278.4, 333.1]`); (c) a literal block with aligned columns, chosen automatically for arrays of flat records.
- **Decision:** (c).
- **Rationale:** (a) is an API agents must remember for the case they meet most; with (c) the default is right. (b)
  stays valid YAML data but costs brackets and commas on every row, and the header row looks like a data row. (c) is a
  string in the YAML, which the renderer already produces for multi-line strings, reads like a printed table, and puts
  the paging note right under the rows.
- **Consequences:** A result's table is text, not YAML data; programs that need data use `cv.save` (GCV-004). Column
  alignment costs a few spaces per value; the shared budget keeps the CUT note from cutting a table in the middle.
- **Status:** proposed.

### D10. `UNUSED_NODE` is attributed to the eval's own code

- **Context:** 12 of 13 warnings were for a node built inside a helper and dropped by the caller; the hand-off asked
  for no warning on nodes that "escape through a returned or mem-stored object", or for `cv.quiet`.
- **Options considered:** (a) `cv.quiet(node)`; (b) widen reachability (the node was neither returned nor kept, so
  this does not cover the case); (c) report only nodes built at sites in the eval's own code.
- **Decision:** (c), plus an uncapped walk of `mem`.
- **Rationale:** The lint exists to catch a forgotten `return` or `await` in the code the agent just wrote. A library
  returning three views to a caller who uses two is not that mistake, and the warning cannot be acted on: the line it
  names is not the agent's. (a) adds an API to silence a false positive. Reachability from the returned value and from
  `mem` stays, as the brief requires; it already holds today, except for the 48-item cap of the walk, which the
  uncapped walk removes.
- **Consequences:** Nodes discarded inside a library are not reported; authors lint libraries by running them with
  `goja-cv run`. Code pasted into an earlier eval keeps the old behaviour, since all eval code shares one source name
  (§17, risk 5).
- **Status:** proposed.

### D11. Labels are an item field, drawn by sheets and overlays

- **Context:** Sheet cells were labelled with list indices; agents kept parallel arrays to map cells to rows. The
  hand-off asked for a `label` field.
- **Options considered:** (a) a `labels` option that names any field (`labels: "snr"`); (b) string ids; (c) a `label`
  field, drawn when present, else the `id`.
- **Decision:** (c), with the sheet's `index` mode deleted and string `label` values allowed.
- **Rationale:** (c) keeps one rule for the two ops that draw items and keeps the label in the data, where a filter or
  a map sets it. (a) is an option for what one `map` does. (b) changes the type of `id`, which every schema and
  detector gives as a number.
- **Consequences:** The `items` op carries `id` and `label` per entry (§7.7, an extension of GCV-004 §7.6).
- **Status:** proposed.

### D12. `peaks` returns Regions of disks with `measure`'s fields

- **Context:** Peaks must feed `sheet`, `measure`, `overlay` and `crop` (the brief), and their numbers must mean what
  the same names mean elsewhere.
- **Options considered:** (a) a Value of rows `{ x, y, value, snr }` (the hand-off's sketch); (b) a Value of rows in
  the flat item schema `{ id, cx, cy, r, … }`, used as literal circles; (c) a Regions node of `Disk` regions whose items
  carry `measure`'s core/ring fields.
- **Decision:** (c).
- **Rationale:** (a) uses `x, y`, which in the item schema are a box corner (GCV-004 §4.5), and a `value` whose meaning
  is new. (b) works as literal circles, but `measure` keeps only the geometry of literal items (GCV-004 §7.6), so
  measuring peaks on another image would drop their snr. (c) is what a detector returns: awaited items are linked, so
  `measure` adds fields to them, a filtered array keeps its links, and `overlay` draws exact disks. Reporting
  `measure`'s own fields (`step`, `snr`, `spread`, …) means a peak's snr is the number `measure` gives at that point
  with that radius: there is one definition of each quantity. "Value" in the hand-off is `step` (height above the
  ring) and `coreMean` (level).
- **Consequences:** A `Disk` region needs no raster; the `RegionsValue` of peaks has the receiver's frame. Measuring
  peaks on another image needs a `prefix` (`E_FIELD_EXISTS` otherwise), which is what the agents' `sp` is.
- **Status:** proposed.

### D13. Three options: `radius`, `snr`, `polarity`

- **Context:** The candidates were `minDistance`, `snr`, `box`, `max` (the hand-off: `above`, `minDistance`, `box`,
  `snrMin`). The helpers show which knobs agents turned: `mem.faint`'s level, `snrMin`, area and aspect bounds and
  `max`; `mem.probe`'s core 3 and its 2 px search.
- **Options considered and decided, one by one:**
  - `radius`: kept. It is the one scale parameter: core radius of the step, non-maximum suppression distance and
    overlap rule all derive from it. The helpers' core 3 is its default; a list replaces `mem.faint`'s area bounds.
  - `snr`: kept, default 1. It is the op's own selection criterion: without it the result is every grain maximum
    (about one per 30 px² at radius 3, tens of thousands on a tile), which costs time to measure and to return. Its
    default is the playbook's "snr ≥ 1 stands out"; `mem.faint` used 1.4.
  - `polarity`: kept, for consistency with `detect.specks` (the same name and values), and because the alternative
    for dark spots, `x.toGray().mul(-1).peaks()`, negates `coreMean` and does not work on colour receivers through the
    luminance (the luminance of a negated colour image is not the negated luminance).
  - `minDistance`: not kept. The overlap rule (cores do not overlap) gives a spacing from `radius`; a sparser list is a
    JavaScript filter.
  - `box`: not kept. `img.crop(b).peaks()` restricts the search, and items keep doc px coordinates. Near the crop's
    edge the ring is partly outside (its `ringCoverage` says so); a crop with a margin and a JavaScript filter on
    `cx, cy` is exact.
  - `max`: not kept. Items are sorted by |snr|; `slice(0, n)` is the cap.
  - `above` (a level threshold): not kept; it is the global threshold that issue 4 shows fails.
- **Decision:** `peaks({ radius = 3, snr = 1, polarity = "bright" })`.
- **Consequences:** A grainy tile at snr 1 may return a few hundred peaks; they are sorted, and the table renders the
  strongest first.
- **Status:** proposed.

### D14. The local noise is `measure`'s ring spread

- **Context:** The snr must compare a spot with its own surroundings (issue 4), and the agents' probe snr came from
  the same core/ring measure.
- **Options considered:** (a) a global noise estimate (the MAD of the whole step map, as `detect.specks` does);
  (b) a robust local σ of the step map in a window around the peak; (c) the 10–90 % spread of the gray plane in the
  peak's ring, exactly as `measure` computes it.
- **Decision:** (c).
- **Rationale:** (a) is the tile-global scale again, which hides dust in dark areas when edges dominate. (b) is a
  second definition of noise beside `measure`'s, with its own window size to choose. (c) reuses the measured,
  documented quantity the agents' thresholds (random places 90 % below 0.45) were derived from, and gives a peak
  exactly the snr `measure` would report there.
- **Consequences:** Next to a strong edge the ring holds both sides of it and the snr drops; the playbook notes the
  same of the probe ("sp stays high for a speck next to a bright edge, where snr drops"). Ranking by the speck view's
  step (`spStep`) covers that case. The thresholds were derived with the helpers' ring (6–12 px for core 3); GCV-004's
  ring is [4.5, 7.5]; the validation re-derives them (§15.2).
- **Status:** proposed.

### D15. Several radii; overlapping peaks keep the higher snr

- **Context:** Gold radii range from 2.4 to 64 px (median 3.8, 90 % below 6.3, 198 points today). A saturated blob of
  radius 10 searched at radius 3 has a flat centre (step 0) and a ring of rim maxima (issue 5).
- **Options considered:** (a) one radius per call, merging in JavaScript; (b) a scale-space detector with a continuous
  scale and normalized responses; (c) a list of radii, each searched independently, merged by one rule: of two peaks
  whose cores overlap, the higher |snr| wins.
- **Decision:** (c).
- **Rationale:** snr is unitless, so peaks at different radii are comparable without normalization. On a saturated blob
  the peak at a radius near its own has a quiet ring outside the blob and a high snr; rim peaks at small radii have
  part of the blob in their ring and a lower snr, and their cores overlap the large one: they are dropped. (a) is a
  greedy merge every agent would write differently. (b) needs a scale normalization and a choice of scale steps that
  the snr already makes unnecessary.
- **Consequences:** One rule for one radius and several (peaks never overlap). A spot is reported once, with the
  radius that describes it best, which `measure` then reuses.
- **Status:** proposed.

### D16. Peaks of any image; polarity bright by default

- **Context:** The brief asks how peaks relates to views: `img.view("speck").peaks()` against `img.peaks()`.
- **Options considered:** (a) peaks only on one-channel images; (b) peaks of any image through the luminance
  (GCV-004's `Luma`, like `detect.specks` and `stats`); (c) a peaks mode inside the views.
- **Decision:** (b), default polarity `bright`.
- **Rationale:** (b) composes: a one-channel receiver (a view, a top-hat, a mask) is searched as it is, a colour one
  through the one luminance, in the receiver's space. (a) makes `img.peaks()` an error on every colour tile for no
  gain. (c) ties a data op to pictures. The default is `bright` because a peak is a maximum by name, and dust on the
  DUST-002 density tiles is bright; `detect.specks` keeps its own default.
- **Consequences:** The docs say that views which clip at zero make the snr meaningless (§6.8) and recommend peaks on
  the image and `measure` on the view.
- **Status:** proposed.

### D17. Four morphology ops; disk or line

- **Context:** The brief lists erode, dilate, open, close and white/black top-hat with disk and line elements.
- **Options considered:** names (a) `erode, dilate, open, close` (OpenCV's verbs); (b) `erosion, dilation, opening,
  closing` (scikit-image's nouns); (c) `erode, dilate, opening, closing`. Element: (d) `{ shape: "disk" | "line",
  angle }`; (e) `{ line: angle }`, absent = disk; (f) more shapes (square, ellipse).
- **Decision:** (c) and (e).
- **Rationale:** `open` as an image method next to `cv.open` gives one word two meanings, the problem GCV-004 D21
  removed for `as`; `opening`/`closing` name the operation without the clash, and `erode`/`dilate` are the words
  everyone types. (e) cannot express an invalid combination (an angle with a disk); (d) can. Squares and ellipses have
  no use in the evidence; the speck background's square stays internal.
- **Consequences:** `cv.help("open")` keeps its meaning (`cv.open`); the morphology entries are found by their names
  and by the concept entry `tophat`.
- **Status:** proposed.

### D18. Mask `grow` and `shrink` become `dilate` and `erode`

- **Context:** With masks as one-channel images (GCV-004 D4), `grow(r)` and `dilate(r)` would be two dilations of the
  same image.
- **Options considered:** (a) keep both: `grow`/`shrink` exact (distance transform, anti-aliased edge) for masks,
  morphology for grey images; (b) one family, with the distance transform as an internal fast path for binary inputs.
- **Decision:** (b).
- **Rationale:** On a 0/1 mask, dilation by the digital disk {dx² + dy² ≤ r²} selects exactly the pixels whose
  distance between centres to the selection is at most r, which is `grow`'s set; a property test checks it. What
  `grow` adds is a one-pixel ramp, which `feather(1)` provides as a separate, explicit step. The distance transform's
  advantage (cost independent of r) is kept as a fast path for binary planes with r ≥ 8, so large radii on large masks
  stay fast.
- **Consequences:** `grow`, `shrink`, `kernels.Grow`, `kernels.Shrink` and `mask.regions`' `grow` option are deleted
  (§9). Golden looks that use `grow` change only at the mask edge (§14.3).
- **Status:** proposed.

### D19. No top-hat op

- **Context:** The brief lists white and black top-hat.
- **Options considered:** (a) `tophat(r, { polarity })`; (b) the composition `x.sub(x.opening(r))` and
  `x.closing(r).sub(x)`, with a help concept entry.
- **Decision:** (b).
- **Rationale:** A top-hat is exactly one existing op applied to the result of another; an op would be a second way to
  write it, which GCV-004's rule ("one way per thing") excludes. The composition reads as its definition, and the
  `tophat` concept entry makes it findable.
- **Consequences:** The docs show the top-hat in the views topic, the measuring topic and the recipes.
- **Status:** proposed.

### D20. No median or rank filter now

- **Context:** Old issue 18 asked for a median and min/max filters.
- **Options considered:** (a) `median(r)` with the disk element; (b) nothing now.
- **Decision:** (b).
- **Rationale:** No agent of this study needed one; min and max filters are `erode` and `dilate`; the robust
  background a median would give is what a top-hat or the speck detector's background already provides for bright
  structures. A median with a disk element is a sliding-histogram kernel (Huang's algorithm over the chords of §10.9),
  a separate piece of work that waits for a use.
- **Consequences:** None; the element definitions of §4.7 are ready for it.
- **Status:** proposed.

### D21. No `white` option for the speck view

- **Context:** Issue 4 asked for `view("speck", { white: "p99" | "noise", k })`.
- **Options considered:** (a) the option; (b) a doc note and the local measure in `peaks`.
- **Decision:** (b).
- **Rationale:** The measured alternative was worse (87 % against 91 % coverage, 103 against 71 spots per tile).
  Other scales are already one call away: `img.dog(1, 4).levels({ black: 0, white })` with any white, or
  `img.crop(b).view("speck")` for a local scale. The structural fix for faint dust is a local snr where candidates are
  found, which `peaks` provides.
- **Consequences:** The note of §6.10 in the views topic and the op's doc.
- **Status:** proposed.

## 9. Legacy and complexity to delete

| # | phase | delete | where | lines (est.) |
|---|---|---|---|---|
| 1 | A1 | prelude file reading and the "not subject to Roots" path of preludes | `session.go:84-88`, `136-144`, `226-232`, `265-268` | ~20 |
| 2 | A2 | `memKind`'s bare "function" for `mem` functions; the display-renderer walk of `mem` in `unusedNodes` | `processor.go:266-270`, `527-557` | ~10 |
| 3 | A2 | "return cv.json(x) for all" and "return cv.json(x) or cv.csv(x)" (if GCV-004 has not already replaced them) | `yaml.go:155`, `180`, `290` | ~3 |
| 4 | A3 | the sheet's `index` labels and the input-dependent default; numeric-only ids | `sheet.go:113-121`, `172-190`, `272` | ~15 |
| 5 | B1 | `grow`, `shrink` (their `lengthMaskOp` registrations) | `pkg/cv/ops/mask.go:340-341` | ~2 (+ docs, help, tests) |
| 6 | B1 | `kernels.Grow`, `kernels.Shrink` | `pkg/cv/kernels/mask.go:411-449` | ~40 |
| 7 | B1 | `mask.regions`' `grow` option (GCV-004's successor of `toMask({ grow })`) and its `Grow` call | `regions.go:357-358`, `366-370` (GCV-004 moves them) | ~10 |
| 8 | B1 | exported `SquareMinMax`, `Opening`, `Closing` (unexported, kept for the speck background) | `morph.go:54-107` | renames |

`DistanceToSet` stays (heal and the binary fast path). Nothing in GCV-005 keeps an old name: `grow`, `shrink` and
`labels: "index"` become unknown names with edit-distance suggestions only, as GCV-004 D12 prescribes for removed
names (no table entry, no migration error).

**Estimate.** Deleted: about 100 lines of non-test code. Added: `cv.load`, `cv.scripts` and their tracking (~180),
file locations (~60), `LOAD` (~30), context block functions and `DescribeFunction` (~90), tables (~120), attribution and
`collectNodes` (~40), labels (~40), morphology kernels and ops (~220), peaks kernel and op (~260). Net: about +1,000
lines, half of it in the two kernels. Tests add about 1,200. Treat these as ±30 %; the implementation diary records
`git diff --stat` per phase.

<!-- PART4 -->
