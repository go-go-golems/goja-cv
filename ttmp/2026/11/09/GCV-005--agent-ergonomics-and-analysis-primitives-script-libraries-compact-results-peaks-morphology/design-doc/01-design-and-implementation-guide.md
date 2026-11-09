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
    - Path: ttmp/2026/11/09/GCV-005--agent-ergonomics-and-analysis-primitives-script-libraries-compact-results-peaks-morphology/reference/04-review-read-findings.md
      Note: review read of revision 1 (33 findings); disposition in Appendix B
    - Path: ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/design-doc/01-design-and-implementation-guide.md
      Note: the base API (revision 3, deviations 1-28); its architecture tour, workspace, items and look queue are assumed throughout
    - Path: pkg/cvsession/session.go
      Note: preludes (runPreludes), the eval wrapper and line mapping; --prelude becomes a cv.load eval
    - Path: pkg/cvsession/init.js
      Note: __cv_run waits for the loads and saves an eval started
    - Path: pkg/cvsession/processor.go
      Note: warnings, unusedNodes (deleted), errorNode, contextBlock; the UNHANDLED warning, function docs
    - Path: pkg/cvsession/yaml.go
      Note: the result renderer (array and string caps, capText); compact tables exempt from both caps
    - Path: pkg/cvsession/native.go
      Note: readFailure parses <eval>:line:col; gains file frames
    - Path: pkg/cvsession/reference.go
      Note: MCP Instructions and the eval tool description
    - Path: pkg/gojamod/cv/evaltrack.go
      Note: Site and the eval trace; pending loads and saves, drop records
    - Path: pkg/gojamod/cv/binding.go
      Note: native functions; gains __compile, __pending, __drop, __scripts
    - Path: pkg/gojamod/cv/prelude.js
      Note: the cv object; gains cv.load (load stack in JavaScript) and cv.scripts
    - Path: pkg/cv/graph/order.go
      Note: the CROP_FIRST lint, deleted; DECORATION_FIRST stays
    - Path: pkg/cv/kernels/morph.go
      Note: van Herk minMax1D and the square opening of the speck background; the disk and line kernels build on it
    - Path: pkg/cv/kernels/mask.go
      Note: DistanceToSet, Grow, Shrink; Grow and Shrink are deleted, DistanceToSet becomes the binary fast path of dilate/erode
    - Path: pkg/cv/kernels/probe.go
      Note: ProbePoint and ProbeCoreRing, the core/ring measure that peaks reports
    - Path: pkg/cv/ops/mask.go
      Note: lengthMaskOp grow/shrink; replaced by erode and dilate
    - Path: pkg/cv/ops/sheet.go
      Note: labels enum and the caption's cell list
    - Path: pkg/cv/ops/views.go
      Note: speckView and levelsAt (white = 5 x p99 over the receiver); doc note on the image-global scale
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
Summary: "Intern guide for GCV-005 (revision 3): what 39 annotator agents did with goja-cv, and the design that follows from it on top of API v2 (GCV-004). Part A: task libraries (cv.load with a JavaScript load stack, cv.scripts, --prelude as a load, file:line locations, evals that wait for their loads and saves, one UNHANDLED warning from goja's rejection tracker), mem function docs shown when new or changed, compact tables exempt from the item and string caps; UNUSED_NODE is deleted. Part B: peaks (sub-pixel local maxima of the core/ring step, snr against the local ring, multi-radius, Regions of disks, row-band processing) and two morphology ops, erode and dilate (disk up to 64 doc px, van Herk lines), which replace mask grow/shrink; opening, closing and top-hat are compositions. CROP_FIRST and OpDef.Halo are deleted. Phases A1-A4 and B1-B3; relative gold tests; validation with an agent rerun and gold metrics; review disposition in Appendix B; no open questions."
LastUpdated: 2026-11-10T02:00:00-05:00
WhatFor: "Understanding why agents need task libraries, readable results and coordinates from data, and implementing GCV-005 phase by phase on top of GCV-004."
WhenToUse: "Read before touching cv.load, preludes, the result renderer, the context block, UNUSED_NODE (deleted), sheet labels, peaks, erode/dilate, OpDef.Halo or the crop lint; read the GCV-004 guide's section 2 first if you are new to goja-cv."
---

# GCV-005 — Agent ergonomics and analysis primitives

## Revision history

| date | revision | what changed |
|---|---|---|
| 2026-11-09 | 1 | First version, written on `task/gcv-005-agent-ergonomics-and-primitives` (stacked on `task/gcv-004-api-v2` at `10938ce`, GCV-004 Phases 0b, 0c and 1 implemented) from the evidence of DUST-002 `reference/11`, the 374 extracted agent evals, the 39 agent diaries and the coordinator's disposition in `reference/01-analysis-annotator-agent-feedback.md`. |
| 2026-11-10 | 2 | Revised from the review read (`reference/04-review-read-findings.md`, 33 findings) and the coordinator's decisions; GCV-004 batch 1 (Phases 0a–4) merged into the branch (`6bb71a8`). Evidence counts corrected (39 pastes, 335 other evals, 19 `UNUSED_NODE` of which 18 false, 24/28 of 32 probes, no result reached the 9,000 cap; the cuts were the string and item caps). `CROP_FIRST` is deleted and crop's help documents what an op after a crop sees. The disk is "distance ≤ r" without a promise of equality with `grow`. Morphology is two ops, `erode` and `dilate`; opening, closing and top-hat are compositions with help concept entries; disks are capped at 64 doc px, lines use van Herk. `UNUSED_NODE` is redesigned: it reports nodes and promises dropped by an expression statement, found with go-go-goja's `jsparse`, in evals and loaded files. Evals wait for the loads and saves they started; one `UNHANDLED` warning from goja's rejection tracker replaces `LOAD`; `E_TOO_MANY` counts nesting depth on a JavaScript load stack. Function docs: a comment that is the first token of the body, shown when the name is new or its source changed, never consumed by a prelude. Tables are exempt from the item and string caps. Peaks: row bands, a pre-filter before the exact measure, a time budget, and prototype numbers from four gold tiles; snr thresholds explained as random-place numbers. Labels, the string-`id` error and point lists move to GCV-004 Phase 5. Terms defined before use; Appendix A updated; Appendix B added. |
| 2026-11-10 | 3 | The coordinator's decisions on revision 2's open questions, all deletions or acceptances. `UNUSED_NODE` is deleted with no replacement lint: the jsparse rewrite, `__cv_drop`, `UNAWAITED` and the site-based reporting go (D10). `OpDef.Halo` is deleted with the crop lint (D25); `crop`'s help explains the edge effect in words. Sheet captions list cell ids for arrays and say "cells in region order" for a Regions node. `cv.save` promises join the pending list in GCV-004 Phase 6. GCV-004 Phase 5 does not add `mask.regions`' `grow`, so B1 deletes nothing there and needs only Phase 4. No open questions remain. |

## How to read this guide

This guide is for an intern who will implement GCV-005 in goja-cv. It assumes the system GCV-004 (API v2) describes.
GCV-004 Phases 0a–4 are implemented and merged into this branch; Phases 5–8 (regions as arrays, the workspace and
`cv.save`, names, documentation) are being implemented while this revision is written (Appendix A). If you have never
opened goja-cv, read section 2 of the GCV-004 guide (`ttmp/2026/11/09/GCV-004--…/design-doc/01-design-and-implementation-guide.md`,
"An architecture tour of goja-cv") first; it traces one call through every layer. Section 2 of this guide only
revisits the parts GCV-005 changes, and ends with the terms the rest of the guide uses.

The guide then presents the evidence (section 3) and the vocabulary of the design (section 4) before it shows where
today's code does each thing (section 5), the JavaScript API with before-and-after code taken from the agents' own
evals (section 6), the architecture (section 7), the decisions and their alternatives (section 8), what is deleted
(section 9), pseudocode (section 10), errors (section 11), documentation (section 12), and the work in phases with
tests and validation (sections 13–15). Section 16 is a migration note for the agents' helpers; section 17 lists risks
and open questions. Appendix A lists the GCV-004 details this guide relies on; Appendix B maps every review finding to
what this revision did.

Citations have the form `path:line` or `path:first-last` and refer to this branch at `6bb71a8` (GCV-004 batch 1
merged). When a line has drifted, search for the named function. Evidence paths abbreviate
`/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/`
as `DUST-002/`.

GCV-005, like GCV-004, is alpha work with no backward compatibility. "Delete" means delete: no alias, no wrapper, no
migration error.

## 1. Executive summary

Thirty-nine Claude Sonnet agents annotated dust on film-scan tiles through goja-cv's MCP server: 374 evals, no errors.
They did not use goja-cv the way its documentation teaches. Every one of them pasted a 3 KB helper file into its first
eval, called its functions, and filtered the returned rows with plain JavaScript. Nobody opened `cv.docs` or
`cv.help`. The helpers were the interface; goja-cv was the substrate. Where the helpers stopped, the agents read
coordinates off zoomed pictures with rulers, and those coordinates were 5–10 px wrong: on one tile, 24 of 32 positions
an agent read off a look measured a speck-view step below 0.06 when probed, which random places reach 90 % of the time.

GCV-005 makes goja-cv good at being that substrate. It has two independent halves.

**Part A: task libraries and readable results.**

1. `await cv.load("dust/helpers.js", args)` runs a JavaScript file in the session. The file is found like any file
   `cv.open` reads (the GCV-004 workspace, then the `--read` directories), runs as the body of an async function with
   `args` bound, and returns what the file returns. Errors and warnings point into the file (`helpers.js:57`). An eval
   does not end before the loads (and saves) it started have settled. `cv.scripts()` lists the `.js` files that can be
   loaded. `--prelude FILE` becomes "load this file when a session starts". There is no `eval_file` MCP tool.
2. The context block lists the `mem` functions with short signatures, and gives the doc line of each function whose
   name is new or whose source changed. The doc line is a comment that is the first token of the function's body.
   The first agent eval of a session shows the docs of everything a prelude defined.
3. An array of flat records renders as a compact table, a header line and one line per row. Tables are exempt from the
   48-item array cap and the 2,000-character string cap that cut the agents' row lists; only the result cap applies,
   and a paging note says which rows are shown and how to get the rest.
4. `UNUSED_NODE` is deleted. In the study 18 of its 19 warnings were false; in v2 a node nobody awaits, looks at or
   saves is a harmless no-op, and the mistakes that matter (a dropped `cv.load` or `cv.save`) are covered by evals that
   wait for them and by `UNHANDLED`. The docs state the rule: a node you don't await, look at or save does nothing.
5. One warning, `UNHANDLED`, reports a promise that was rejected and never handled, from goja's rejection tracker. It
   covers an unawaited `cv.load` or `cv.save` that failed.
6. Sheet captions list the ids of their cells in order, so a model never reads an id off a picture. (Item labels, the
   string-`id` error and sheets of plain points are implemented by GCV-004 Phase 5.)
7. The MCP instructions, the context block and a new "task libraries" docs topic teach all this where agents read.

**Part B: analysis primitives.**

8. `x.peaks({ radius, snr, polarity })` returns the local maxima of the core/ring step, the quantity `measure` reports,
   as a Regions node of disks with sub-pixel centres and `measure`'s fields (`step`, `spread`, `snr`, …). The snr is
   measured against the spot's own ring, so a faint speck on a quiet, dark background stands out. A list of radii finds
   spots of several sizes and keeps the best of overlapping peaks; a saturated blob is one large peak instead of a ring
   of small ones. A numpy prototype on four gold tiles kept 105–1,000 peaks per tile at `snr ≥ 1` and covered 101 of
   102 dust points (§3.6).
9. Two morphology ops for one-channel images, `erode` and `dilate`, with a disk (radius up to 64 doc px) or a line
   element (`{ line: angle }`, any length). Opening is `x.erode(r).dilate(r)`, closing `x.dilate(r).erode(r)`, a top-hat
   `x.sub(x.erode(r).dilate(r))`; `cv.help` has concept entries for the three words. The mask ops `grow` and `shrink`
   are deleted.
10. The `CROP_FIRST` lint is deleted, and with it `OpDef.Halo`, its only reader: cropping first is the intended way to
    get a local scale for peaks, morphology and views. `crop`'s help says what an op after a crop sees.
11. The speck view keeps its white point (a noise-scaled view measured worse: 87 % against 91 % gold coverage); its
    documentation says the scale is set over the image it is computed on.

Not in scope, each with its reason in section 6.11: `crop(box, { fit: "shift" })`, `components({ maxAspect })`, a
labelled grid on looks, `cv.quiet`, `measure.line`, a median filter, a `white` option for the speck view, and opening,
closing and top-hat ops.

The design follows one rule from the evidence: **never make a model read a number off a picture; every look comes with
the data it shows.** Peaks give coordinates, tables give rows, sheet captions give the ids of their cells.

## 2. An architecture tour of the parts GCV-005 touches

This section shows each part GCV-005 changes as it is on this branch (GCV-004 Phases 0a–4 included) and says what
GCV-004's remaining phases change in it. It ends with the terms the guide uses.

### 2.1 Where the parts are

```
 front ends   cmd/goja-cv (eval, run, mcp: --prelude, --root, --read)      pkg/xgoja/providers/cv
                 │                                                             │
 session      pkg/cvsession ── session.go   preludes, the eval wrapper, line mapping                 ◄── A1, A2
                 │            ├─ init.js     globals: cv, mem, img; __cv_run                          ◄── A1
                 │            ├─ processor.go warnings, UNUSED_NODE, errors (where:), context block   ◄── A1, A2
                 │            ├─ yaml.go      the result renderer and its caps                        ◄── A2
                 │            └─ reference.go MCP instructions, eval tool description                 ◄── A4
 JS module    pkg/gojamod/cv ── prelude.js   the cv object                                            ◄── A1 (cv.load, cv.scripts)
                 │             ├─ binding.go native functions                                          ◄── A1, A2
                 │             └─ evaltrack.go per-eval tracking: nodes, sites, looks                  ◄── A1, A2
 graph        pkg/cv/graph   ── order.go (CROP_FIRST)                                                  ◄── B1 (deleted)
              pkg/cv/ops     ── sheet.go (caption), mask.go (grow, shrink)                             ◄── A3, B1
                 │            └─ peaks.go, morph.go (new)                                              ◄── B1, B2
 pixels       pkg/cv/kernels ── morph.go (van Herk), mask.go (distance transform), probe.go (core/ring) ◄── B1, B2
```

### 2.2 The eval session

A **session** is one goja runtime that lives across evals. `App.Evaluate` (`pkg/cvsession/session.go:317-325`) wraps
the agent's code as the body of an async function, `__cv_run(async function () {\n` + code + `\n})`
(`session.go:301-308`), so `await` and `return` work at top level and agent line N is line N + 1 of the evaluated
source. go-go-goja's replsession runs that source with `vm.RunString` (module `go-go-goja`,
`pkg/replsession/evaluate.go:676`), so all eval code shares one source name, printed as `<eval>` in stack traces.
`__cv_run` (`pkg/cvsession/init.js:68-…`) starts the body from a promise job and records an uncaught error with
`native.fail`.

A **prelude** is a script file given with `--prelude FILE` (`cmd/goja-cv/mcp.go`, `cmd/goja-cv/eval.go:36`).
`CreateSession` runs every prelude in a new session before any agent code (`session.go:206-220`); `runPreludes`
(`session.go:226-257`) evaluates each file's text through `Evaluate`, like agent code, so the result processor and its
context block run for it, and its output is discarded. GCV-004 resolves prelude paths against the process's working
directory (GCV-004 §7.8) and discards looks created while a prelude runs (§7.7).

`mem` is a plain object defined once per session (`init.js:64`). It is the only thing that survives between evals:
`const` and `let` are local to the eval's function.

### 2.3 After the eval: the result processor

replsession calls the **result processor** (`pkg/cvsession/processor.go`) after the code settles. `Process`
(`processor.go:64-91`) ends the eval's tracking, collects looks (GCV-004's look queue, `collect`, `processor.go:122-…`),
renders the returned value as YAML, appends warnings and the **context block**, and caps the text.

- **Sites.** Every node a cv call returns is recorded with its **site**: the first stack frame outside the cv prelude
  and native code (`Site{Source, Line, Column}`, `pkg/gojamod/cv/evaltrack.go:24-28`; `site()`, `evaltrack.go:163-…`).
  `lineTag` (`processor.go:302-309`) turns a site into "line N" when its source is eval code and its line falls inside
  the eval's text.
- **UNUSED_NODE.** `unusedNodes` (`processor.go:311-370`) marks as used every node the eval awaited, looked at,
  returned, kept in `mem` or bound to an image name, with all their inputs, and reports the last node of every other
  chain the eval built. To find nodes in `mem` it renders `mem` with the display renderer.
- **The context block.** `contextBlock` (`processor.go:554-576`) lists the open images and the keys of `mem` with a
  kind each (`memKind`, `processor.go:578-608`). A function is listed as "function".
- **Errors.** `readFailure` (`pkg/cvsession/native.go:119-…`) takes the first `<eval>:line:col` of the thrown value's
  stack (`stackPos`, `native.go:111`) as the position; `errorNode` (`processor.go:412-486`) renders
  `where: line N: <source line>`. Frames in any other source are ignored.

### 2.4 The result renderer and its three caps

`renderer` (`pkg/cvsession/yaml.go:40-57`) turns a settled JavaScript value into a YAML node tree without running
getters or proxy traps. A cv node becomes a one-line summary, a look its caption, a function `[Function]`
(`yaml.go:111-113`). Three caps apply, and they cut at different points:

| cap | where | note it leaves |
|---|---|---|
| `maxArrayItems = 48` items per array | `yaml.go:23`, `array` at `yaml.go:167-188` | `# … N more (return cv.json(x) for all)` |
| `maxStringLen = 2000` characters per string | `yaml.go:24`, `primitive` at `yaml.go:153-156` | `… (N more characters; return cv.json(x) for all)` |
| `MaxResultChars = 9000` characters per result | `capText`, `yaml.go:286-303` | `# CUT: …` |

A map of more than eight entries is written one line per key (`flow`, `yaml.go:243-265`): an awaited detector item of
10–20 fields becomes 10–20 lines. That is why the agents' helper turned its rows into strings
(`[o.id, o.x, o.y, o.sp, …].join(" ")`, `DUST-002/scripts/84-cv-helpers.js`, `mem.hot`).

### 2.5 Kernels and ops for spots and shapes

- **The core/ring probe.** `ProbePoint` and `ProbeCoreRing` (`pkg/cv/kernels/probe.go:178-226`) measure a point on a
  `*buffer.Buffer`: the coverage-weighted mean of the gray plane in a **core** disk of radius r, the same in a **ring**
  between r1 and r2, their difference (**step**), the 10–90 % spread of the ring's values (**spread**), and **snr** =
  step / max(spread, 1e-3). Weights come from `EllipseCoverage` (`pkg/cv/kernels/mask.go:183`). `measure` reports these
  fields (`coreRingFields`, `pkg/cv/ops/measurement.go:138-159`); GCV-004 sizes core and ring from each item (core r,
  ring [1.5 r, 2.5 r], GCV-004 D17).
- **Running minimum and maximum.** `minMax1D` (`pkg/cv/kernels/morph.go:14-46`) is the van Herk/Gil-Werman algorithm:
  the minimum or maximum over a sliding window of any length in three comparisons per sample. `SquareMinMax`,
  `Opening` and `Closing` (`morph.go:56-107`) build square-element morphology on it for the speck detector's background
  (`SpeckBackground`, `pkg/cv/kernels/specks.go:106-154`). None of it is reachable from JavaScript.
- **The distance transform.** `DistanceToSet` (`mask.go:368-409`) computes the exact Euclidean distance from every
  pixel to the nearest selected pixel. `Grow` and `Shrink` (`mask.go:411-449`) build the mask ops `grow` and `shrink`
  on it (`lengthMaskOp`, `pkg/cv/ops/mask.go:299-317`, registered at `mask.go:338-339`); `heal` also uses it
  (`pkg/cv/kernels/heal.go:35`).

### 2.6 Items and regions, as GCV-004 defines them

GCV-004 Phase 5 (in progress) makes awaited regions plain arrays (GCV-004 §4.5, §7.6):

- An **item** is a flat object `{ id, x, y, w, h, cx, cy, area, aspect, … }` (`graph.GeometryFields`,
  `pkg/cv/graph/schema.go:32-43`, are on every region).
- A **Regions node** is a lazy region set produced by a detector or by `measure`. Awaited, it is an array of items,
  each carrying a hidden link `[node, index]` that `filter`, `sort`, `slice` and object spread keep.
- An op that takes regions accepts a Regions node or an array; unlinked objects are **literal items**: a box
  `{ x, y, w, h }`, a circle `{ cx, cy, r }`, and, for the ops that take points (`measure`, `sample`, `sheet`), a centre
  `{ cx, cy }` or `[x, y]`, which becomes a `Point` shape.
- A `Region` has a **shape**: `Pixels`, `Box`, `Disk` or `Point`. Arrays are interned as an internal `items` op that
  carries each element's `id` and `label`; a non-numeric `id` is `E_ARG_TYPE`; ids may repeat.

### 2.7 One session with a helper library, traced

The agent on tile `cfca85b41cd6f78f_2048_0` (`DUST-002/various/agent-evals/cfca85b41cd6f78f_2048_0__pre-annotation.js`):

1. **Eval 1** is the helper file, 2,976 characters pasted as code. It defines ten functions on `mem`; the context block
   lists `mem: T (string), best (function), hot (function), …`.
2. **Eval 2** calls `mem.tile(…)` and `mem.views()` and returns three looks.
3. **Eval 3** runs `await mem.hot(0.4)` and `await mem.hot(0.3)`: each thresholds the speck view, labels components and
   calls `mem.probe` on each centroid. The 53 rows come back as strings, and the array is cut at 48 items.
4. **Evals 4–7** page through rows in contact sheets of 15 and look at four quarters of the speck view at 2× with rulers.
5. **Eval 8** probes 32 positions the agent read off those rulers: 24 measure a speck-view step below 0.06, 28 below 0.1.
6. **Evals 9–11** lower the threshold, filter rows in JavaScript, sheet and zoom.

Every part of GCV-005 shortens one of these steps: eval 1 becomes a `cv.load` or a prelude; the strings of eval 3
become a table; the rulers of evals 7–8 become `peaks`; the sheet cells map to rows through ids that are data.

### 2.8 Terms

| term | meaning |
|---|---|
| **doc px** | a pixel of the original file; every coordinate, length and box in goja-cv is in doc px, whatever the scale a node is computed at |
| **analysis scale** | the scale (buffer pixels per doc px) at which an op computes; region ops use 1, so their ids and coordinates do not depend on a look's size |
| **speck view** | `img.view("speck")`: a difference of Gaussians (fine detail minus its surroundings, σ s/3 against 4 s/3), bright part only, stretched so that white is 5 × its 99th percentile over the image (`speckView`, `pkg/cv/ops/views.go:184-201`); dust is a compact bright dot in it |
| **sp** | the helpers' name for the core/ring step measured on the speck view (core 3 px, ring 6–12 px); random places are 90 % below 0.06, labelled dust has a median of 0.31 |
| **hot list** | the helpers' `mem.hot(level)`: components of the speck view above `level`, each probed; `mem.hotByLevel[level]` keeps each level's rows |
| **NMS** | non-maximum suppression: keeping a pixel only if no pixel within a given distance has a larger value |
| **`Halo`** | `OpDef.Halo` (`pkg/cv/graph/registry.go:281`): how many doc px around each output pixel an op reads; read only by `CROP_FIRST`, and deleted with it (D25) |
| **hash-consing** | the builder returns the existing node for an identical call, so repeating work costs nothing (GCV-004 §2.4) |
| **MAD** | median absolute deviation, a robust noise estimate (`kernels.MAD`) |
| **builder placeholder** | the stand-in receiver a builder lambda (`img.within(m, i => …)`) is called with at build time (GCV-004 §2.9) |
| **provider** | go-go-goja's result-content interface; looks are providers rendered after the eval |
| **xgoja** | go-go-goja's binary builder; `pkg/xgoja/providers/cv` makes the `cv` module available in xgoja binaries |
| **`RANGE`** | the warning for an argument outside its usual range (for example a crop reaching outside the frame) |
| **pre-annotation, trial 2, trial 3** | the three DUST-002 agent runs (31, 4 and 4 agents) |

## 3. The problem and the evidence

### 3.1 The study

The DUST-002 project asked Claude Sonnet subagents to find dust, hair and scratches on 1024 × 1024 gold tiles (tiles a
person has labelled). Each agent followed a playbook (`DUST-002/reference/04-goja-cv-dust-playbook.md`), opened one MCP
session on goja-cv v1 (`main` at `b847520`), pasted the helper file `DUST-002/scripts/84-cv-helpers.js`, and wrote a
diary with a "goja-cv friction" section. `DUST-002/scripts/91-extract-agent-evals.py` extracted every eval call into
`DUST-002/various/agent-evals/`; the diaries are in `DUST-002/various/agent-diaries/`.

The numbers below are the review read's recount from the raw transcripts (`reference/04-review-read-findings.md`, F4
and F5). DUST-002 `reference/11`, `various/agent-evals/summary.md` and script 91 carry older numbers: the extractor
flags a paste by the string `helpers ready (v1)`, which ten agents dropped, and keeps only the first eight lines of
each result. The coordinator tells the user; this ticket does not edit that repository.

| measure | value |
|---|---|
| agent runs | 39 (31 pre-annotation, 4 trial 2, 4 trial 3) |
| evals | 374: 39 helper pastes (2,859–3,454 characters) and 335 others (median 181 characters) |
| errors | 0 |
| `UNUSED_NODE` | 19: 11 in eval 2 (no line), 7 in eval 1 (line 10, inside the paste), 1 correct; 18 false |
| `RANGE` | 11 warnings in 9 evals (crops near the border, up to 200 × 340 px); `OUTSIDE` never |
| cuts | the longest result was 4,701 characters, so the 9,000 cap never applied; the 2,000-character string cap cut one joined row string; the 48-item array cap fired in 40 evals |

### 3.2 What agents used

| | evals using it (`summary.md`; helper counts include up to ten unflagged pastes) |
|---|---|
| helpers | `mem.sheet` 161, `mem.zoom` 119, `mem.hot` 78, `mem.probe` 65, `mem.tile` 37, `mem.views` 37, `mem.hotLook` 23, `mem.spots` 21, `mem.best` 20 |
| cv ops directly | `look`, `crop`, `sheet`, `view`, `levels`, `measure`, `luminance`, `components`, `overlay`, `exposure` |
| never | `cv.docs`, `cv.help`, `explain`, `profile`, `sample`, `annotate`, `compare`, `highpass`/`dog`, `localContrast`, `chroma`/`channel`/`toLab`, `cv.csv`/`cv.json`, `save`, `P.slider` |

Beyond the helpers, the agents wrote JavaScript over returned rows. Over the 335 non-paste evals: `.map(` in 100,
`.slice(` in 72, `.filter(` in 55, `mem.probe(` in 42, `mem.hotByLevel` in 44, `aspect` in 21. By the hand-off's
count, 23 agents filtered on `sp`, 7 on a region, 3 on area or aspect; 22 agents probed hand-typed coordinate lists in
a loop. One diary (de14d942): "Filtering mem.hotRows in JS (`mem.hotRows.filter(o=>o.sp>=0.15 && o.y<1290)`) then
`mem.sheet(rows.map(o=>[o.x,o.y]))` was the useful pattern."

### 3.3 The ten issues

| # | issue | evidence |
|---|---|---|
| 1 | helper libraries must be pasted | every agent pasted; about 15 diaries complain ("Pasting the helpers needs the whole file text in the eval call (no include)"); agents trimmed comments and the final `return`; one wrote "`const r3` defined in the paste is not visible in later evals" and added `mem.r3 = r3` (4b246f75) |
| 2 | the sheet badge hides cell 0's label; labels are list indices | 10 diaries on the badge ("(1342, 3250)" shows as "2, 3250", af8640d7 trial 2); 7 agents kept parallel arrays to map cell index to row (`mem.L`, `mem.LP`, 851dabe2) |
| 3 | coordinates read off ruler looks are wrong | 8 diaries; positions 5–10 px off; in cfca85b4 eval 8, 24 of 32 guesses probed at sp < 0.06 and 28 at sp < 0.1; agents derived "x = crop_x + (px − 52)/2" for the ruler margin (a0a0f824) |
| 4 | the speck view's scale is set by scene edges | 6 diaries; dust measured sp 0.03–0.1 on such tiles against a gold median of 0.31; agents boosted the view (`.exposure(2.5)`, `.levels({ white: 0.25 })`) |
| 5 | saturated blobs become rings | b54a23af: the most obvious blob probed sp 0.21, snr 0.61; a capsule-shaped one was missing from the hot list |
| 6 | lines become long components | "the hot list is mostly line fragments" (21661036); "almost all streak rims" (8af4cabe); agents filtered `aspect < 3` |
| 7 | `RANGE` on edge crops | 11 warnings in 9 evals |
| 8 | `UNUSED_NODE` for nodes a helper builds | 18 of 19 warnings are `speckColor` of `mem.views()`, bound to `const v`, of which the eval used `v.scene` and `v.speck` |
| 9 | long row lists are cut | the item cap in 40 evals; one joined row string cut by the string cap (c71dd0f2 trial 2, eval 10, which the diary calls "truncated at 9000 chars") |
| 10 | nothing measures the width of a thread | agents "eyeballed widths and path vertices from rulers" (af8640d7 trial 2); `profile` was never used |

### 3.4 What the evidence shows

**A task library replaced the documentation.** No agent opened `cv.docs` or `cv.help`. With a playbook and a library,
an agent needs goja-cv to run the library, show what it defined, and return rows it can read. The documentation that
matters is what an agent sees without asking: the MCP instructions, the context block, error fixes and the library's
own comments.

**Models cannot read coordinates off pictures.** The most expensive failure was a wrong number, not an error. Rulers
help a person; a vision model localizes poorly at this precision. What worked was asking goja-cv for coordinates:
component centroids, or `mem.probe`, which snaps to the best point within 2 px. This gives the design rule: **never make
a model read a number off a picture; every look comes with the data it shows.** It is why a labelled grid on looks is
not in scope.

**Agents filter with JavaScript.** Filters on `sp`, regions, area and aspect, and probe loops, were plain JavaScript
over arrays, which confirms GCV-004's regions-as-arrays and is why `components({ maxAspect })` is not added.

**The speck scale is image-global, and the obvious fix was worse.** The colleague tested a view scaled by its own
noise (IQR / 1.349) on 197 gold points:

| hot-list scale | spots per tile (median) | gold covered |
|---|---|---|
| 5 × p99, level 0.3 (current) | 71 | 91 % |
| noise, 5.5 σ | 103 | 87 % |
| noise, 6 σ | 50 | 83 % |

Grain is not white noise. Local normalization belongs where candidates are found, not in the view's default.

### 3.5 Caveats

- **"0 errors" is the helpers' result.** The DeepSeek pi runs without helpers had 1–7 errors per run (GCV-004 §3.2).
  The raw API must stay learnable, and libraries must be easy to load.
- **"Never used" means "not shown by the playbook".** `profile` was used 0 times and is the answer to issue 10.
- **The population is narrow:** Sonnet with a playbook.
- **Counts are regular-expression counts** over code; they measure use, not success. The extractor undercounts pastes
  and truncates results (§3.1).
- **Gold data changes.** `~/dustds/gold` held 142 points for GCV-003 and 198 now. The gold tests compare methods within
  one run, never against a fixed count (§14.4).

### 3.6 A prototype of peaks on gold tiles

To ground Part B in numbers before any Go is written, a numpy prototype (`scripts/02-peaks-prototype.py` in this
ticket) computes peaks as §4.5 defines them, with digital disks instead of coverage-weighted ones and a 1024² tile's
luminance of linear values: step map, NMS within r, snr from the ring's 10–90 % spread, and the merge of §6.8 over
radii [2, 4, 8]. A gold point counts as covered when a peak lies within max(r, 3) px of it.

| tile (dust points) | NMS candidates r = 2 / 4 / 8 | peaks at snr ≥ 0.5 / 1 / 1.4 / 2 | covered at snr ≥ 0.5 / 1 / 1.4 / 2 | peaks not near gold at snr ≥ 1 |
|---|---|---|---|---|
| `7edbfe9c3e7ab7d6_512_1024` (52) | 25,449 / 5,371 / 1,034 | 3,178 / 105 / 48 / 23 | 51 / 51 / 37 / 21 | 54 |
| `af8640d7f21ec5ce_1024_3072` (17) | — | 8,515 / 1,000 / 156 / 45 | 17 / 17 / 17 / 16 | 981 |
| `c71dd0f2de533739_512_1536` (31) | — | 7,693 / 329 / 64 / 26 | 31 / 31 / 22 / 15 | 298 |
| `1d955ef4b27d3d88_1536_1536` (2) | — | 6,994 / 439 / 20 / 1 | 2 / 2 / 1 / 0 | 427 |

At `snr ≥ 1`, peaks covered 101 of 102 dust points with 105–1,000 peaks per tile; at 1.4, 77 of 102 with 20–156. The
hot list at level 0.3 gave a median of 71 spots and 91 % coverage on the colleague's 197 points. These are four tiles
and a prototype; they say that `snr: 1` is a recall-first default with a long, sorted list on grainy tiles, and that
the validation of §15.2 must measure both sides before the playbook is rewritten. They are not test thresholds.

## 4. Concepts

This section defines the vocabulary of the design, in dependency order.

### 4.1 Task library, load, and the file's scope

A **task library** is a JavaScript file that defines functions for one task, usually on `mem`
(`mem.probe = async (x, y) => …`). A **pipeline script** does one job and returns its result. goja-cv does not
distinguish them; both are **script files**.

To **load** a script file is to run it in the current session with `cv.load(path, args)`. The file's text is the body
of an async function whose only parameter is `args`:

```js
// what cv.load("dust/helpers.js", { tile: "cfca…" }) runs, conceptually
(async function (args) { /* the file's text, unchanged, starting on this line */ })(theArgs)
```

A script file therefore has an eval's semantics: top-level `await` and `return` work, `const` and `let` are local to the
file, the session globals (`cv`, `mem`, `img` and other image names) are visible, and anything meant to survive goes on
`mem`. The one addition is `args` (an empty object when none is given). `cv.load` returns a promise of the file's
return value.

The **load stack** is the list of files whose synchronous part is running: a file that calls `cv.load` before its
first `await` pushes a second entry. Its depth is bounded (§6.1).

### 4.2 A function's signature and doc line

A function on `mem` is visible to the agent through the context block. Its **signature** is its name and parameter
list as written (`probe(x, y)`, `hot(level = 0.4, max = 80)`). Its **doc line** is a comment that is the first token of
its body: right after the `{` of a block body, or right after the `=>` of an expression body, with only whitespace
before it.

```js
mem.probe = async (x, y) => { // the best point within 2 px: { x, y, snr, sp, step, chroma, cover }
  const o = { coreRadius: 3, ringRadii: [6, 12] }   // a comment here is not a doc line
  …
}
mem.spots = (level = 0.4) => // blobs of the speck view above level, as regions
  mem.img.view("speck").mask.luminance({ above: level }).components({ minArea: 4 })
const lib = {
  sheet(items, size = 40) { /* contact sheet of up to 15 items */ … },   // method shorthand
}
```

goja's `Function.prototype.toString` returns a function's exact source text, comments included, for arrow, `function`,
`async` and method-shorthand forms (checked with `goja-cv eval` on this branch by the writer and the reviewer); bound
and native functions return `function name() { [native code] }`. Signature and doc line can therefore be read from the
function itself, wherever it was defined.

### 4.3 A compact table

A **flat record** is a plain object whose own properties are data properties holding scalars: numbers, strings,
booleans, `null` or `undefined`. A **table** is the rendering of an array of at least two flat records as text: a header
line with the union of their keys, then one line per record. In the result's YAML it is a literal block scalar (a
multi-line string, `|-`). A table is built as one YAML node, not through the renderer's per-array and per-string caps.

### 4.4 A node that nothing uses

In v2 a node is a description of work: options are literal and only `await`, a look or `cv.save` evaluates it (GCV-004
§4.1, §6.5). A node that the code builds and then neither awaits, looks at nor saves **does nothing**: no pixels are
computed, nothing is written, and the cache is untouched. There is therefore no wrong result to warn about, and
GCV-005 has no lint for it (D10). The promises that matter, those of `cv.load` and `cv.save`, are waited for by the eval
(§4.9), and their failures are reported (§6.6).

### 4.5 The step map and a peak

The **step** of a point is `measure`'s core/ring step: the mean of the gray plane in a disk of radius r around the
point minus the mean in the ring between 1.5 r and 2.5 r. The **step map** at radius r is the step at every pixel. A
**peak** is a pixel whose step is positive and at least as large as the step of every pixel within r of it (non-maximum
suppression with a disk of radius r). A **dark peak** is the same for −step. The peak's **sub-pixel centre** is the
vertex of parabolas fitted through the step map at the peak and its four neighbours.

### 4.6 Local noise and snr

The **local noise** of a point is the **spread** of its ring: the 90th minus the 10th percentile of the gray plane in
the ring between 1.5 r and 2.5 r. The **snr** is step / max(spread, 1e-3), as in `measure`. Because the spread comes
from the spot's own surroundings, a faint speck on a dark, quiet background has a small step, a smaller spread, and a
high snr. This is the structural answer to issue 4.

The thresholds the playbook gives for snr (random places 90 % below 0.45, 99 % below 0.8; dust median 1.7) were
measured at **random places**. Peaks are not random places: they are local maxima of the step, refined to the point of
highest step, so their snr is biased upwards against those numbers. The random-place numbers bound what `measure`
reports at an arbitrary point, not what `peaks` reports; §3.6 and §15.2 measure peaks directly.

### 4.7 Structuring elements, erosion and dilation

Grey-scale **morphology** computes, for each pixel, the minimum or maximum of the image over a set of offsets around
it, the **structuring element** (SE). With an SE B:

- **dilation**: dilate(x)(p) = max over b ∈ B of x(p + b);
- **erosion**: erode(x)(p) = min over b ∈ B of x(p + b).

GCV-005 has two SEs. The **disk** of radius r is the set of integer offsets (dx, dy) with dx² + dy² ≤ r²: the pixels
whose centres are at distance at most r, the standard convention. The **line** of radius r at angle θ is the run of
2R + 1 pixels of the digital line through the pixel at angle θ, whose end centres are about 2r apart (§10.10). Both
are symmetric about the origin. Pixels outside the image take no part.

### 4.8 Opening, closing and top-hat are compositions

- **opening**: `x.erode(r).dilate(r)` removes bright structures into which the element does not fit and leaves the rest;
- **closing**: `x.dilate(r).erode(r)` removes dark ones;
- **white top-hat**: `x.sub(x.erode(r).dilate(r))` keeps the bright structures smaller than the element, at their full
  height; **black top-hat**: `x.dilate(r).erode(r).sub(x)`.

A top-hat with a disk larger than a saturated speck keeps the speck as a filled plateau, where a difference of
Gaussians turns its flat top into a ring (issue 5). An opening with a line element keeps what contains a segment of that
length in that direction, so a top-hat with it removes lines.

### 4.9 A settled eval, and an unhandled rejection

A **settled eval** is one that does not end before the promises of the `cv.load` and `cv.save` calls it started have
settled. An **unhandled rejection** is a promise that was rejected and to which no handler was attached by the end of
the eval; goja reports both events through `Runtime.SetPromiseRejectionTracker`
(`github.com/dop251/goja`, `builtin_promise.go:668`).

### 4.10 Data with every look

Every op that helps an agent choose where to look returns coordinates as data (peaks, items), and every picture made of
many places (a sheet) lists, as text in its caption, the ids of the places it shows. Nothing the agent needs to type
later should exist only in pixels.

## 5. Current-state analysis

### 5.1 Preludes exist; nobody knew

`--prelude FILE` is a repeatable flag of `eval`, `run` and `mcp`. `cvsession.Options.Preludes` (`session.go:87`) holds
absolute paths, not subject to the read roots; `runPreludes` reads each file and evaluates its text (`session.go:226-257`).
The README mentions the flag in one sentence (`README.md:81`), the CLI page in another (`pkg/doc/03-cli.md:73-75`); the
Claude Code setup page does not. The DUST-002 server was registered from the setup page, without it, and every agent
pasted the file.

### 5.2 Locations in errors and warnings

All eval code shares the source name `<eval>`. `readFailure` takes the first `<eval>:L:C` of the stack and `errorNode`
maps it to the agent's line by subtracting the wrapper's line (`agentLine`, `session.go:442-452`). A frame in another
source is ignored. A function pasted in eval 1 and called in eval 5 has `<eval>` frames with eval 1's line numbers,
mapped against eval 5's text: when eval 5 is shorter, `agentLine` returns 0 and the location is dropped (the 11
line-less `UNUSED_NODE` warnings), when longer, it names an unrelated line. Loaded files get their own source names
(§7.3); pasted code stays imprecise (§17, risk 5).

### 5.3 The context block shows names only

`contextBlock` prints `mem` as one line of `key (kind)` pairs; a function is `(function)`. An agent learns that
`mem.faint` exists, not what it takes or returns. The agents learned it from the playbook and the helper file's header
comment. And because `runPreludes` goes through the processor, anything the context block remembers is also
remembered for a prelude eval, whose output nobody sees (§7.4).

### 5.4 The renderer's caps

The three caps of §2.4 are what cut the agents' row lists: the 48-item cap fired in 40 evals, and the 2,000-character
string cap cut the one long joined string (c71dd0f2 trial 2, eval 10:
`return { spots: h.spots, rows: h.rows.slice(30,90).join(" | ") }`, cut at about 60 rows). No result reached 9,000
characters. The notes of all three caps name `cv.json`, which GCV-004 Phase 6 deletes. The agents did not follow them:
they sliced by hand (`mem.hotByLevel[0.3].slice(45)`, 2b3a8276) or turned rows into strings. A row of eight values
costs about 45 characters as a string and about 120 as a YAML map.

### 5.5 UNUSED_NODE

`unusedNodes` (`processor.go:311-370`) reports a node created during the eval when it is not reachable from the used
nodes, the returned value, `mem` or the named images, and does not feed a node that is. The 18 false warnings of the
study were all `speckColor`: `mem.views()` returned `{ scene, speck, speckColor }`, the eval bound it to `const v` and
returned looks of `v.scene` and `v.speck`. The node was neither returned nor kept in `mem`, so reachability does not
cover it. Seven of them were in eval 1, where the paste was the eval's own code, so attributing warnings to "code the
agent did not write" (revision 1's design) would not have removed those either. What the agent's code did was keep the
bundle in a variable; it dropped nothing. Conversely, the lint misses the mistake it exists for when a library builds
the node: `mem.spots(0.3)` alone on a line builds its nodes inside the library and discards them all.

`mem` is walked with the display renderer, which stops at 48 array items and 2,000 properties: a node in the 49th
element of an array on `mem` is invisible to the walk.

### 5.6 Sheet labels

`listItems` (`pkg/cv/ops/sheet.go:114-…`) labels a cell with the item's numeric `id`, or else its list index from 0; a
string id is silently replaced by the index. The `labels` option is `id | index | none` (`sheet.go:297`), with a default
that depends on the input. GCV-004 Phase 5 replaces the list path with the shared `items|point[]` binding, the `label`
field and the string-`id` error (§2.6). The sheet's caption, which the agent reads as text, does not say which item each
cell shows.

### 5.7 Morphology exists only inside the speck detector; CROP_FIRST forbids local work

The van Herk running extremum and the square opening and closing are internal to `SpeckBackground`. Masks have `grow`
and `shrink`: `Grow` writes `max(m, clamp01(r + 1 − d))` (`mask.go:424`), which is at least 0.5 up to distance r + 0.5;
`Shrink` writes `min(m, clamp01(d − r))` (`mask.go:444`). On a grey image there is no way to remove structures by size
or shape.

`orderWarnings` (`pkg/cv/graph/order.go:18-39`) raises `CROP_FIRST` for every pixel op with a `Halo` whose receiver chain
contains a crop: "X after crop sees only the window (and its mirrored border); put crop last. fix: img.X(…).crop(b)".
It is the only reader of `OpDef.Halo`. Today `img.crop(b).view("speck")` and `mask.crop(b).grow(2)` raise it. Cropping
first is exactly how an agent gets a local scale for a view or a local search for peaks, and for a Regions result the
fix (`img.peaks(…).crop(b)`) has no meaning.

### 5.8 What the helpers build by hand

| helper | what it does | cost |
|---|---|---|
| `mem.best(node, x, y, opts)` | measures 21 points (a disk of radius 2 px) with one `measure` call and keeps the highest snr | one measure of 21 points |
| `mem.probe(x, y)` | `mem.best` on the tile (core 3, ring 6–12) and on the speck view; reports the tile point's `x, y, snr`, the speck view's step as `sp`, and `chroma` when snr ≥ 0.5 | 42 point measurements |
| `mem.hot(level, max)` | thresholds the speck view at `level`, labels components, probes each centroid (first 300), sorts by `sp`, returns strings; keeps every level's rows in `mem.hotByLevel` | about 0.4 s for 160 probes |
| `mem.faint(opts)` | the same at level 0.06, keeps components of area 5–80 and aspect < 3 not within 4 px of a hot spot, probes them (first 600), keeps snr ≥ 1.4, sorts by snr | about 0.2 s |
| `mem.sheet(points, size, label)` | maps rows or pairs to `{ cx, cy, id }` and calls `cv.sheet` with the tile and the speck view | one sheet |

`mem.hot` and `mem.faint` are a peak finder: candidates from a global threshold, each refined to the best point within
2 px, ranked by a local contrast. `mem.faint` was mined from an agent's own eval (2b3a8276, eval 13). `peaks` does the
same in one op, with the global threshold replaced by the local snr.

### 5.9 The speck view's white point

`speckView` (`pkg/cv/ops/views.go:184-201`) builds the difference of Gaussians and stretches it with the internal op
`levelsAt` (`views.go:206-…`): white = max(5 × the 99th percentile of the receiver's luminance, 1e-4), computed over the
whole receiver. `img.view("speck").crop(b)` keeps the whole image's scale; `img.crop(b).view("speck")` computes the
scale over the crop alone, a local stretch, which `CROP_FIRST` warns against today and which `mem.sweep`
reimplemented with `stats` and `levels`.

### 5.10 The MCP instructions

`Instructions` (`pkg/cvsession/reference.go:15-42`) must stay under 4,000 characters (`pkg/cvsession/mcp_test.go:177`).
GCV-004 rewrites it in Phase 8; GCV-005's version (§12.1, 3,458 bytes) is GCV-004's v2 contract plus its additions.

## 6. The proposed API

Each subsection shows what an agent or a library author writes before GCV-005 (on GCV-004's API, which is what the
agents' code becomes after GCV-004's migration note) and after, using the agents' real code, then states the rules.

### 6.1 `cv.load`: run a script file in the session

```js
// before: eval 1 of every agent, the helper file pasted as code (2,859–3,454 characters)
mem.T = "/Users/manuel/dustdet/cv_tiles/"
mem.tile = (id, ox, oy) => {
  mem.img = cv.open(mem.T + id + ".tif", { encoding: "linear", origin: [ox, oy], name: "img" })
  …
}
… 90 more lines …
```

```js
// after: the library is a file in a --read directory (or a --prelude)
await cv.load("dust/helpers.js")                      // defines mem.tile, mem.spots, mem.sheet, …
mem.tile("cfca85b41cd6f78f_2048_0", 2048, 0)
```

```js
// a pipeline script with arguments
const rows = await cv.load("dust/candidates.js", { snr: 1.4, radius: [2, 4, 8] })
return rows                                           // a table (§6.5)
```

```js
// dust/candidates.js: compact bright spots on the open tile that stand out from their own surroundings
const lin = img.as("linear")
const pk = await lin.peaks({ radius: args.radius ?? 3, snr: args.snr ?? 1 })
return await lin.view("speck").measure(pk, { prefix: "sp" })   // + spStep, spSnr, … per peak
```

Rules:

- `cv.load(path, args)` returns a promise of the file's return value. Write `await cv.load(…)`.
- `path` is resolved like a path given to `cv.open` (GCV-004 §4.6, §7.8): a relative path in the workspace, then in
  each `--read` directory in flag order; an absolute path must lie in one of them. It must end in `.js` and contain no
  whitespace or parentheses (they would make stack frames ambiguous, §7.3).
- The file runs as the body of `async function (args) { … }` starting on the function's first line, so its line numbers
  are its own. `args` is the second argument as given (any JavaScript value, not copied), or `{}`.
- Every call reads and runs the file again; there is no cache. A second load of a library redefines its functions;
  a pipeline runs again with new arguments. Nodes are hash-consed and cached, so a rerun costs little.
- A loaded file may load other files with the same path rule. The load stack (§4.1) is at most 32 deep; a deeper load
  is `E_TOO_MANY`, naming the stack (`a.js → b.js → a.js → …`). Parallel loads (`Promise.all(tiles.map(t =>
  cv.load("p.js", { t })))`) do not nest and are not limited.
- An eval does not end before the loads it started have settled (§4.9), so nothing a file does leaks into a later eval.
  A load that failed and was never awaited is reported by `UNHANDLED` (§6.6).
- Errors in a loaded file name the file and line: `where: dust/helpers.js:57: …`, plus `called from: line 1: …` when
  the eval called a function the file defined. Warnings raised by calls in the file carry `dust/helpers.js:57`.
- Looks created by a loaded file are delivered like the eval's own (GCV-004's look queue).
- `cv.load` exists wherever the `cv` module does: the MCP server, `goja-cv eval` and `run`, and xgoja binaries (with the
  provider's `root` and `read`, GCV-004 §7.8).

### 6.2 `cv.scripts()`: what can be loaded

```js
return cv.scripts()
```

```yaml
|-
  path                  doc
  dust/candidates.js    candidates.js: compact bright spots on the open tile that stand out from their own surroundings
  dust/helpers.js       dust/helpers.js: helpers for annotator agents on DUST-002 gold tiles (goja-cv v2).
  dust/thread-width.js  dust/thread-width.js: the width of a bright thread across a segment (half-height width)
```

Rules: `cv.scripts()` returns `{ path, doc }` rows, sorted by path, for the `.js` files in the workspace and the read
directories. The walk visits at most four directory levels and 5,000 entries per root, skipping directories whose name
starts with `.` and `node_modules`, and lists at most 500 files. `path` is what `cv.load` accepts; a file shadowed by one
with the same relative path in an earlier root is not listed. `doc` is the file's first comment line. It never throws.
Unlike `cv.open`, which reads one named file, this enumerates names; it lists only `.js` files under directories the
operator granted (§17, risk 8).

### 6.3 `--prelude`: a load at session start

```bash
claude mcp add cv -- goja-cv mcp --root ~/dustdet/cv_out --read ~/dustdet/cv_tiles --read ~/dustds \
  --read ~/code/dust/lib --prelude ~/code/dust/lib/dust/helpers.js
```

Rules: `--prelude FILE` (repeatable, on `eval`, `run` and `mcp`) runs `await cv.load(FILE)` in every new session before
any agent code, in flag order. The path is resolved against the working directory, as GCV-004 resolves `--image`, and
must lie in the workspace or a read directory; otherwise the command fails with a usage error naming `--read`. Looks
created by a prelude are discarded. A failing prelude fails session creation with
`prelude dust/helpers.js:57: <message>`. A prelude does not mark functions as seen: the agent's first eval lists the
docs of every function the preludes defined (§6.4).

### 6.4 `mem` functions in the context block and in results

```yaml
# before: eval 1's context block
context:
  mem: T (string), best (function), hot (function), hotLook (function), probe (function), sheet (function), spots (function), tile (function), views (function), zoom (function)
```

```yaml
# after: the agent's first eval in a session with the library as a prelude
context:
  images: img = cfca85b41cd6f78f_2048_0.tif [2048,0,1024,1024] (16-bit TIFF, linear)
  mem: box (array[4]), img (#1 Image), lin (#2 Image)
  functions: mem.sheet(items, size = 40), mem.spots({ snr = 1, radius = [2, 4, 8] } =…), mem.tile(id, ox, oy), mem.zoom(x, y, half = 24)
  new functions:
    mem.sheet(items, size = 40): contact sheet of up to 15 items: tile | speck view, captioned with their ids
    mem.spots({ snr = 1, radius = [2, 4, 8] } = {}): peaks with the speck view's step: sp = spStep
    mem.tile(id, ox, oy): open a tile at its origin: every coordinate is an absolute scan coordinate
    mem.zoom(x, y, half = 24): two looks (scene, speck view) of a 2·half square with rulers
# every later eval, until a function's source changes
context:
  …
  functions: mem.sheet(items, size = 40), mem.spots({ snr = 1, radius = [2, 4, 8] } =…), mem.tile(id, ox, oy), …
```

Rules:

- `functions:` lists every function on `mem`, sorted, each signature cut at 40 characters on this line.
- `new functions:` gives the full signature and the doc line of each function whose name is new or whose source text
  changed since the last agent eval that showed the context block. Reloading an unchanged library lists nothing new.
- Prelude evals never update this state: the first agent eval of a session shows the docs of everything the preludes
  defined.
- A function anywhere in a result renders as its signature and doc line instead of `[Function]`: `return mem.probe`
  gives `mem.probe(x, y): the best point within 2 px: …`, and `return mem` lists them all.
- A function without a doc line shows its signature only; native and bound functions show `name()`; getters and setters
  on `mem` are not listed as functions.

### 6.5 Tables

```js
// before: c71dd0f2 trial 2, eval 10; rows joined into one string, cut by the 2,000-character string cap
return { spots: h.spots, rows: h.rows.slice(30, 90).join(" | ") }
// b54a23af eval 2: the same pattern, to dodge the 48-item cap and the one-line-per-field maps
return { rows: mem.hotByLevel[0.3].slice(0, 60).map(o => [o.id, o.x, o.y, o.sp, o.snr, o.area, o.aspect].join(" ")) }
```

```js
// after: return the rows themselves
return { rows: mem.hotByLevel[0.3] }
```

```yaml
rows: |-
  id  x     y    sp     snr     chroma  area  aspect  edge
  14  2278  333  1.762  11.954  0.214   61    2.783   false
  3   2573  62   0.614  0.377   -       5557  180.28  false
  13  2085  304  0.399  2.317   0.625   14    1.02    false
  …
  … rows 1–118 of 174 shown. Next: rows.slice(118, 236); all: await cv.save("rows.csv", rows)
```

Rules:

- An array of at least two flat records renders as a table, at any depth of the result. Columns are the union of the
  records' keys in first-seen order; a missing, `null` or `undefined` value is `-`.
- Numbers use the renderer's format (integers exactly, others with five significant digits); a string is written as
  it is unless it is empty, is exactly `-` or contains whitespace, which are JSON-quoted; strings longer than 60
  characters are cut with `…`; booleans are `true` and `false`. Columns are left-aligned by character count (runes, not
  bytes) and separated by two spaces.
- A table is exempt from the 48-item array cap, the 2,000-character string cap and the property budget. Only the result
  cap applies to it, as a table budget of two thirds of the result cap (6,000 of 9,000 characters) shared by all tables
  of one result in render order; each table shows at least five rows.
- When rows are left out, the last line says which rows are shown, the next page, and how to get all of them. It names
  the array by its key path in the returned value (`rows`, `hot.rows`), or "this array" for a top-level array.
- Arrays of anything else render as today; every cut note names `cv.save` instead of `cv.json`.
- There is no new API: picking columns is `rows.map(({ id, cx, cy, snr }) => ({ id, cx, cy, snr }))`, paging is `slice`.

### 6.6 `UNUSED_NODE` is deleted; `UNHANDLED`

```js
// before: 18 false warnings in the study (c71dd0f2 trial 2, eval 2, and 17 others)
mem.tile("c71dd0f2de533739_512_1536", 512, 1536)
const v = mem.views()
return { scene: v.scene.look("scene"), speck: v.speck.look("speck") }
// UNUSED_NODE: [Image #26 view(name: "speckColor")] builds a node and discards it. …
```

After GCV-005 no eval gets `UNUSED_NODE`: the warning is deleted (D10). A node you don't await, look at or save does
nothing, which the eval contract states in the instructions and the getting-started topic. What can go wrong is a
promise that fails with nobody listening:

```js
cv.load("dust/helpers.js")            // not awaited: the eval still waits for it before it ends
cv.save("x.tif", img.view("speck"))   // not awaited: the eval waits for it too (GCV-004 Phase 6 puts it on the pending list)
```

```
UNHANDLED dust/helpers.js:57: a promise was rejected and never handled: mem.img is undefined. fix: await the call that returned it
```

Rules:

- `UNHANDLED` reports, once, each promise rejected during the eval that had no handler when the eval ended, with the
  rejection's message and the location of its error. It covers an unawaited `cv.load` or `cv.save` that failed and an
  async helper whose error nobody caught.
- There is no warning for unused nodes, no `UNAWAITED`, and no `cv.quiet`.

### 6.7 Sheets: labels and the cell list

GCV-004 Phase 5 implements the item model sheets need: one `items|point[]` binding shared by `measure`, `sample` and
`sheet`, in which a centre without a radius is a `Point` shape; the `label` field (a cell or overlay label is the
item's `label` when it has one, else its `id`); and `E_ARG_TYPE` for a non-numeric `id`, whose fix moves the text to
`label`. With it:

```js
// before (on GCV-004 Phase 4): parallel arrays to map cells back to rows (851dabe2)
mem.L = mem.hotByLevel[0.3].filter(r => r.sp >= 0.15)
mem.LP = mem.L.map(o => [o.x, o.y])
return { idx: mem.L.slice(0, 15).map((o, i) => i + ":" + o.id).join(" "), a: mem.sheet(mem.LP.slice(0, 15), 40, "0-14") }
```

```js
// after: pass the rows; cells are labelled with ids, and the caption lists them
const rows = mem.hotByLevel[0.3].filter(r => r.sp >= 0.15)
return { a: cv.sheet(rows.slice(0, 15), { views: [img, img.view("speck")], size: 40, stretch: "cell" }).look("0-14") }
// a: '[look 1: 0-14 — sheet(15 items; cells: 14 13 26 22 33 28 31 30 36 29 2 27 1 39 32) (1306×658, …)]'
```

Rules added by GCV-005: the sheet's caption lists the label (else id) of each cell in cell order, `cells: …`, so the
agent maps cells to rows from text; the sheet's `labels` option is `"id" | "none"` with default `"id"` (the value
`index` and the input-dependent default go).

### 6.8 `peaks`

```js
// before: cfca85b4 eval 8; positions read off four 2× quarter looks with rulers, then probed one by one
const C = [[2092,293],[2075,262],[2100,270],[2104,222],[2306,203], …, [2745,725]]   // 32 guesses
const out = []
for (const [x, y] of C) { const p = await mem.probe(x, y); out.push([x, y, "->", p.x, p.y, "sp", p.sp, "snr", p.snr].join(" ")) }
return out
// 24 of 32 came back at sp below 0.06: the guesses were beside the dots
```

```js
// after: ask for the spots of the quarter
const lin = img.as("linear")
const pk = await lin.crop([2048, 0, 512, 512]).peaks({ snr: 1 })   // sub-pixel cx, cy in doc px; the search sees only the crop
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
return pk.filter(p => !(p.cx > 2380 && p.cy < 2300))                // the region filter stays JavaScript
```

```js
// mixed sizes, and the speck view's step (the agents' sp) for each peak
const lin = img.as("linear")
const pk = await lin.peaks({ radius: [2, 4, 8], snr: 1 })
const rows = await lin.view("speck").measure(pk, { prefix: "sp" })   // + spStep, spSnr, …
return rows.filter(r => r.spStep >= 0.15)
```

Rules:

- `x.peaks({ radius, snr, polarity })` is an image method that returns a Regions node. On a colour image it searches the
  luminance (GCV-004's `Luma`), in the receiver's space; on a one-channel image, the image itself.
- `radius` (doc px, default 3) is the core radius of the spots sought: a number, or a list to search several sizes,
  each in [1, 64].
- `snr` (default 1) is the smallest |snr| kept. The default favours recall: on the four prototype tiles it kept 105–1,000
  peaks and 101 of 102 dust points (§3.6); the validation of §15.2 decides whether it stays.
- `polarity` is `"bright"` (default), `"dark"` or `"both"`.
- Items have `id, x, y, w, h, cx, cy, area, aspect` (the geometry every region has: `x, y, w, h` the disk's box, `area`
  π r², `aspect` 1), `r` (the radius it was found at), and `measure`'s core/ring fields `coreMean, ringMean, step,
  spread, snr, ringCoverage`, measured with core r and ring [1.5 r, 2.5 r]. They are sorted by |snr|, highest first;
  `id` is the rank.
- Peaks never overlap: of two peaks whose distance is less than the sum of their radii, the one with the higher |snr| is
  kept. With several radii, a large spot is one peak at a large radius rather than a ring of small ones.
- Peaks feed every op that takes items: `cv.sheet`, `overlay`, `measure` (core and ring from each peak's `r`; a `prefix`
  is needed, since the peaks already have `step` and `snr`), `crop`, `mask.regions`.
- Peaks are computed at analysis scale 1. On a crop they search only the crop; near its border rings are cut, which
  `ringCoverage` reports. Crop for speed on a large scan (§10.7 gives the budget).
- On a view that clips at zero, such as the speck view, most rings are flat zero, the spread falls to the 1e-3 floor and
  the snr is not meaningful; there `step` is the quantity to rank by. Search the image and measure the view, as above.

### 6.9 Erosion and dilation

```js
// before: no way to keep a saturated blob filled; the speck view makes it a ring (b54a23af, eval 3)
return { pr1: await mem.probe(1262, 892) }                    // sp 0.209, snr 0.61 on the tile's most obvious blob
```

```js
// after: a top-hat keeps bright things smaller than the disk, at full height
const L = img.as("linear").toGray()
const blobs = L.sub(L.erode(12).dilate(12))                   // white top-hat with a disk of radius 12
const pk = await blobs.peaks({ radius: [2, 4, 8], snr: 1 })   // or: blobs.mask.luminance({ above: t }).components()
```

```js
// before: line fragments dominate the hot list (21661036, 8af4cabe); agents filtered in JavaScript
mem.hotByLevel[0.3].filter(o => o.sp >= 0.15 && o.aspect < 3)
// after: remove lines longer than 21 px before thresholding; one opening per direction, combined with max
const s = img.view("speck")
const lines = [0, 22.5, 45, 67.5, 90, 112.5, 135, 157.5]
  .map(a => s.erode(10, { line: a }).dilate(10, { line: a }))
  .reduce((m, o) => m.max(o))                                 // what contains a 21 px segment in some direction
const spots = await s.sub(lines).mask.luminance({ above: 0.3 }).components({ minArea: 4 })
```

```js
// before (GCV-004 revision 3's design): mask ops grow and shrink, and mask.regions({ grow })
const m = img.mask.luminance({ above: 0.9 }).grow(3)
const around = img.mask.regions(spots, { grow: 2 })
// after
const m = img.mask.luminance({ above: 0.9 }).dilate(3)        // .dilate(3).feather(1) for a soft edge
const around = img.mask.regions(spots).dilate(2)
```

Rules:

- `x.erode(r, { line })` and `x.dilate(r, { line })` are image methods on a one-channel receiver (a mask, a grey image, a
  view, a difference). A colour receiver is `E_WRONG_KIND` with the fix `x.toGray().erode(r)` (GCV-004's `OneChannel`).
- `r` is a length in doc px (positional; 0 returns the receiver). Without `line`, the element is the disk of radius r
  (dx² + dy² ≤ r²), and r is at most 64; a larger disk is `E_ARG_RANGE` with the fix "crop first, or apply a smaller
  disk twice: `x.dilate(64).dilate(64)` reaches about 128". With `line: angle` (degrees, counter-clockwise from the x
  axis), the element is the line of radius r, and r is at most 2,000.
- Results keep the receiver's space and encoding (morphology picks values); the receiver is read as stored.
- Pixels outside the image take no part: near the border the element is clipped.
- Opening, closing and top-hat are compositions (§4.8); `cv.help("opening")`, `cv.help("closing")` and
  `cv.help("tophat")` return concept entries with them.
- `grow` and `shrink` are deleted, and `mask.regions` has no `grow` option (GCV-004 Phase 5 does not add it). `grow(r)` selected (at 0.5) the pixels up to distance r + 0.5;
  `dilate(r)` selects those up to r, so masks grow by half a pixel less than before; `dilate(r + 0.5)` gives the old set.

### 6.10 The speck view's scale, and crops

No option is added to the view. Its documentation (`pkg/cv/docs/topics/11-views.md` and the op's `Doc`) gains:

> The speck view's white point is 5 × the 99th percentile of its high-pass over the image it is computed on. On a tile
> with strong scene edges (windows, cables, lattices) the edges set it, and dust can map to 3–10 % of white.
> `img.view("speck").crop(b)` keeps the whole image's scale; `img.crop(b).view("speck")` sets the scale from the crop
> alone (a local stretch). To find faint dust, do not lower thresholds on the view: use
> `img.as("linear").peaks({ snr })`, whose snr compares each spot with its own surroundings. A large saturated speck
> has a flat top, which the view turns into a ring: use a top-hat (`L.sub(L.erode(r).dilate(r))`) or peaks with
> several radii.

`CROP_FIRST` is deleted, and `crop`'s help says what cropping first means:

> Ops after a crop see only the crop. An op that reads a neighbourhood around each pixel has edge effects near the
> crop's border: a blur sees mirrored pixels, peaks report cut rings (`ringCoverage`), a
> view's scale comes from the crop. Crop first to work locally or to save time; crop last to look at part of a
> whole-image result.

### 6.11 Not in scope

| ask | why not |
|---|---|
| `eval_file` MCP tool | `cv.load` is one mechanism for MCP, CLI and xgoja; `eval_file` would save about 20 characters per call over `eval({ code: 'await cv.load("x.js")' })`, could not be combined with other code in one eval, and would need JSON args (D1) |
| a `--scripts DIR` flag, `cv.require` | scripts live in the workspace and `--read` directories like every other file (D4); `cv.load` returns the file's value |
| `look({ grid, gridLabels })` | a labelled grid is a better ruler, and a model still reads a cell name off a picture; `peaks` returns coordinates as data |
| `crop(box, { fit: "shift" })` | the `RANGE` warnings are informative and the clip is correct; a library clamps its boxes in one line (`x = Math.min(Math.max(x, bx), bx + bw - w)`) |
| `components({ maxAspect })`, an `elongation` field | `items.filter(r => r.aspect < 3)` is the one way to filter; line removal is morphology (§6.9) |
| `cv.quiet(node)` | `UNUSED_NODE` is deleted (D10), so there is nothing to silence |
| `cv.table(rows, { columns })` | tables are automatic (§6.5) |
| `measure.line(path, { width })` | a recipe with `profile` measures a thread's width (§12.4) |
| `view("speck", { white })` | the noise scale measured worse; `dog().levels({ white })` and `img.crop(b).view("speck")` give other scales; local normalization is `peaks`' snr (D21) |
| `opening`, `closing`, `tophat` ops | each is one existing op applied to the result of another (D19) |
| `median(r)` and other rank filters | no evidence in this study; a top-hat covers "background without the speck" for bright structures (D20) |
| `stretch: "sheet"` | raised in the article, not in the hand-off's issues |

### 6.12 The GCV-005 surface in one table

| call | returns | notes |
|---|---|---|
| `cv.load(path, args)` | Promise of the file's return value | workspace, then `--read`; `.js`; an async function body with `args`; load stack ≤ 32 |
| `cv.scripts()` | array of `{ path, doc }` | never throws |
| `x.peaks({ radius, snr, polarity })` | Regions (disks) | radius number or list (doc px, default 3, each ≤ 64); snr default 1; polarity bright/dark/both |
| `x.erode(r, { line })`, `x.dilate(r, { line })` | Image, 1 channel | one-channel receiver; disk r ≤ 64, or a line at `line` degrees |
| `cv.sheet(items, …)` | Image (picture) | caption lists the cells' labels or ids; `labels: "id" \| "none"` |
| `--prelude FILE` | | a `cv.load` at session start; the file must lie in a root |

Removed: `m.grow(r)`, `m.shrink(r)`, the sheet's `labels: "index"`, the `CROP_FIRST` and `UNUSED_NODE` warnings,
`OpDef.Halo`. (`mask.regions(items, { grow })` is never added: GCV-004 Phase 5 leaves `grow` out.)

## 7. Proposed architecture

### 7.1 Overview

```
 agent code ──eval──► session (cvsession)
                       __cv_run(async function () { … })
                         body settles → await allSettled(native.pending())   // the eval's loads and saves
                         │
                         │  cv.load(path, args)  (prelude.js)
                         │    f := native.__compile(path)       Workspace.Open → goja.Compile(name)
                         │    load stack depth ≤ 32 (JS array)  → f.run(args) → inner promise
                         │    native.__pending(inner); return inner.then(v => v)
                         ▼
                       rejection tracker (vm.SetPromiseRejectionTracker): rejected-without-handler set
                       processor.Process
                         ├─ render value ── tables (exempt from item/string caps), functions as signature + doc
                         ├─ warnings ─────── UNHANDLED from the tracker (no UNUSED_NODE)
                         │                   lineTag: "line N" (eval) | "name:N" (file)
                         ├─ errors ───────── where: innermost eval-or-file frame; called from: innermost eval frame
                         └─ context block ── mem, functions (short signatures), new functions (docs; agent evals only)

 x.peaks(opts) ──► op "peaks" (Image → Regions, Luma, AnalysisScale 1)
                     └─ kernels.Peaks, in row bands: step map (chord sums) → NMS (DiskMinMax) → sub-pixel fit
                        → digital-ring snr pre-filter → ProbePoint at survivors → |snr| filter → non-overlap merge
 x.erode/dilate(r, { line }) ──► ops (one channel → one channel)
                     └─ kernels.DiskMinMax (chords + van Herk; binary fast path: DistanceToSet) | kernels.LineMinMax (van Herk along lines)
```

| layer | removed | added or changed |
|---|---|---|
| `pkg/cv/kernels` | `Grow`, `Shrink` | `DiskMinMax`, `LineMinMax`, `Peaks` and its helpers |
| `pkg/cv/graph` | `CROP_FIRST` (`WCropFirst`, its branch of `orderWarnings`, `hasHalo`); `OpDef.Halo` | — |
| `pkg/cv/ops` | `grow`, `shrink`, the `index` labels of `sheet` | `peaks.go`, `morph.go` (`erode`, `dilate`); the sheet caption's cell list; `crop`'s doc |
| `pkg/gojamod/cv` | | `cv.load` and `cv.scripts` (prelude.js), `binding.compile`, `binding.pending`, `binding.scripts`, the file registry, `DescribeFunction`; help: `load`, `scripts`, concept entries `opening`, `closing`, `tophat` |
| `pkg/cvsession` | prelude file reading; `UNUSED_NODE` (`unusedNodes`, its walk of `mem`, `feeds`, `onPlaceholder`) | preludes as loads, `inPrelude`; `__cv_run` waits for pending promises; the rejection tracker and `UNHANDLED`; file frames in `readFailure`, `errorNode`, `lineTag`; tables and functions in the renderer; context block functions; instructions |
| `cmd/goja-cv` | | `--prelude` must lie in a root |

### 7.2 `cv.load` in the module

`cv.load` is defined in `prelude.js` beside `cv.open` (`pkg/gojamod/cv/prelude.js:295-…`). The native side reads and
compiles; JavaScript calls the file's function, so the call stays JS → JS and the load stack is a JavaScript array:

```js
// prelude.js, inside the module's closure
const loadStack = [];                       // names of the files whose synchronous part is running
// in the cv object:
// load(path, args) runs a script file (workspace, then --read directories) as the body of
// async function (args) { … } and returns a promise of its return value.
load(path, args) {
  const f = native.__compile(path);         // { name, run }: read and compiled
  if (loadStack.length >= 32) native.__tooDeep(loadStack.concat([f.name]));   // throws E_TOO_MANY with the stack
  loadStack.push(f.name);
  let inner;
  try {
    inner = f.run(args === undefined ? {} : args);   // runs the body up to its first await
  } finally {
    loadStack.pop();
  }
  native.__pending(inner);                  // the eval waits for it (§7.4)
  return inner.then((v) => v);              // the caller's promise: if it rejects unhandled, UNHANDLED reports it
},
// scripts() lists the .js files cv.load can run: [{ path, doc }]
scripts() {
  return native.__scripts();
},
```

The returned promise is derived from the inner one so that the two have different handlers: `__cv_run` attaches its
wait to the inner promise, and the rejection tracker sees whether the caller handled the derived one. A rejection of an
awaited load fails the eval as any rejected `await` does.

`binding.compile` (`pkg/gojamod/cv/binding.go`, registered in `binding.object`, `binding.go:105-…`, inside GCV-004's
`export` panic boundary) resolves the path through the engine's `Workspace` (GCV-004 §7.8), reads the file and
compiles it:

```go
// compiled is what __compile returns to JavaScript.
type compiled struct {
    Name string        // the file's name in stacks and locations
    Run  goja.Callable // the file's async function
}
```

- **The name** is the path relative to the root the file was found in (`dust/helpers.js`). When an absolute path
  reaches a file in a later root whose relative path is shadowed by an earlier root, the name is `read<N>:dust/helpers.js`
  (N is the read directory's position in flag order), so two different files never share a name. The binding keeps a
  registry `files map[string][]string` (name → lines of the text last loaded under it) for `where:` lines.
- **Line numbers.** The wrapper `(async function (args) {` is prepended to the file's first line without a newline, so
  line N of the compiled source is line N of the file.
- **Pending promises.** `binding.pending` appends the inner promise to the eval trace's `Pending` list. GCV-004's
  `cv.save` appends its promise to the same list (GCV-004 Phase 6).

`cv.scripts` walks each root with `os.Root.FS()` and `fs.WalkDir` within the bounds of §6.2. It is a method of the
`Workspace`, so it shares the root list and the shadowing rule with `cv.open`.

### 7.3 Locations in loaded files

- **Sites.** `binding.site` returns the first frame outside the cv prelude and native code, with `Source` =
  `f.SrcName()`; for a file it is the file's name. Unchanged.
- **Warnings.** `lineTag` (`processor.go:302-309`) gives `CODE line N: ` for an eval site and `CODE dust/helpers.js:57: `
  for a site whose source is in the binding's file registry.
- **Errors.** `readFailure` reads every frame of the stack in order. The **location** is the first frame whose source is
  `<eval>` or a registered file; when it is a file, **called from** is the first `<eval>` frame. File names contain no
  whitespace or parentheses (§6.1), so `([^\s()]+):(\d+):(\d+)` parses every frame. `errorNode` writes:

```yaml
error:
  code: TypeError
  message: 'Cannot read property "cx" of undefined'
  where: 'dust/helpers.js:57: const p = await mem.probe(it.cx, it.cy)'
  called from: 'line 1: const h = await mem.hot(0.3)'
```

- **Syntax errors in a file.** `goja.Compile` fails before anything runs; `binding.compile` throws a cv error with code
  `SyntaxError` and the message `dust/helpers.js:12:5: <goja's message>`; `where` is the eval line that called `cv.load`.
- **Preludes.** `whereLine` (`session.go:222`) accepts `line N` and `name:N`, so a failing prelude reads
  `prelude dust/helpers.js:57: message`.

### 7.4 The session: preludes, settled evals, rejections, the context block

**Preludes.** `withDefaults` resolves each prelude against the working directory and requires it to lie in the
workspace or a read directory (usage error otherwise). `runPreludes` evaluates `await cv.load("dust/helpers.js")` through
`App.Evaluate` with the session state's `inPrelude` flag set; its looks are discarded, and the context block does not
update the function state (below).

**Settled evals.** `__cv_run` (`init.js:68-…`) waits for the eval's pending promises before it settles, on success and on
failure:

```js
value: function (body) {
  const settle = () => Promise.allSettled(native.pending());     // loads and saves this eval started
  return Promise.resolve()
    .then(function () { return body(); })
    .then(function (v) { return settle().then(() => v); },
          function (e) { return settle().then(() => { throw e; }); })
    .then(undefined, function (e) { native.fail(e); throw e; });
},
```

The watchdog still bounds the whole eval, so a load that never settles ends as `E_TIMEOUT`.

**Unhandled rejections.** `InitRuntime` installs `vm.SetPromiseRejectionTracker`. On `PromiseRejectionReject` the
session records the promise and its reason; on `PromiseRejectionHandle` it removes it. `Process` and `ProcessFailure`
turn every recorded promise into one `UNHANDLED` warning, located by `readFailure(reason)`, skip the reason that is the
eval's own failure, and clear the set. The tracker sees every promise in the VM, so the same warning covers a failed
unawaited `cv.load`, a failed unawaited `cv.save`, and an async helper whose error nobody caught.

**The context block.** `contextBlock` (`processor.go:554-576`) splits `mem`'s keys (read through property descriptors,
so getters are not run) into functions and the rest. The rest stays on `mem:` with `memKind`. Functions go on one
`functions:` line as signatures cut at 40 characters. The session state keeps `fnSeen map[string]string`, the source
text last shown under each key; when an agent eval's context block finds a key whose source differs from the
remembered one (or is new), it lists the full signature and doc line under `new functions:` and updates the map. When
`inPrelude` is set, the map is not read or written.

### 7.5 The renderer: tables and functions

`renderer.array` (`yaml.go:167-188`) first asks `tableOf(obj)`: at least two elements, each a plain object (exported as
`map[string]any`, not a node, look, error, provider or function) whose own string-keyed properties are data
properties holding primitives. Properties are read through descriptors, as `renderer.property` does (`yaml.go:210-223`),
so an element with an accessor is not a flat record and no getter runs. If so, `r.table(obj, path)` returns a literal
block scalar node built directly (`yaml.Node{Kind: ScalarNode, Style: LiteralStyle, Tag: "!!str"}`), which never passes
through `primitive` or `budget`: the table is exempt from `maxArrayItems`, `maxStringLen` and `maxProperties`. The
renderer gains `tableLeft int` (two thirds of `MaxResultChars`) and passes the key path down (`rows`, `hot.rows`) for
the paging note. Cut notes of other arrays and strings name `cv.save`.

Functions render through `DescribeFunction` (§10.4): `mem.probe(x, y): the best point within 2 px: …`, named by the key
path, else by the function's own `name`.

### 7.6 Deleting `UNUSED_NODE`

`unusedNodes` (`processor.go:311-370`), its call in `warnings` (`processor.go:264-…`), its walk of `mem` through the
display renderer, `feeds`, `onPlaceholder` (`processor.go:372-396`), the `WUnusedNode` code and, if no other reader
remains, `EvalTrace.Created` and `noteCall`'s recording of created nodes (`evaltrack.go:177-…`) are deleted. Before
deleting `Created`, grep for readers: warnings with sites (`SiteWarning`) and looks use their own records. Tests and docs
blocks that expect `UNUSED_NODE` are changed to expect no warning. Nothing replaces it (D10).

### 7.7 Sheets

GCV-004 Phase 5 delivers the shared `items|point[]` binding with the `Point` shape, the `label` field and the string-`id`
error, and deletes the sheet's separate list path (`listItems`). GCV-005 then changes two things in `sheet`
(`pkg/cv/ops/sheet.go`): the `labels` enum becomes `id | none` with default `id` (`sheet.go:297`), and the look's caption
names the cells: `sheet(15 items; cells: 14 13 26 …)`. The caption is written into the result before looks render, so
it is built from what is known when the look is created: for an array, the `items` op's entries carry each element's
label and id, and the caption lists them in cell order (with more than 40 cells it ends with `… (N more)`); for a Regions
node passed directly, whose ids are known only after evaluation, it says `cells in region order`.

### 7.8 The `peaks` op and kernel

**The op** (`pkg/cv/ops/peaks.go`):

```go
// peakFields: the geometry every region has (id, x, y, w, h, cx, cy, area, aspect; schema.go:32-43),
// the radius, then measure's core/ring fields without channel steps.
var peakFields = graph.WithFields(graph.GeometryFields,
    append([]graph.FieldSpec{numField("r", graph.DocPx, "the radius the peak was found at (core radius)")},
        coreRingFields("off")...)...)

graph.OpDef{
    Name: "peaks", On: graph.KindImage, Returns: graph.KindRegions, Luma: true, // GCV-004 §7.4, deviation 20
    Args: []graph.ArgSpec{
        {Name: "radius", Type: "number|number[]", Unit: graph.DocPx, Default: 3.0, Min: graph.F(1), Max: graph.F(64),
            Usual: [2]float64{1.5, 16}, Doc: "core radius of the spots sought, or a list of radii to search several sizes"},
        {Name: "snr", Type: "number", Default: 1.0, Min: graph.F(0), Usual: [2]float64{0.5, 5},
            Doc: "smallest |snr| kept: step / ring spread"},
        {Name: "polarity", Type: "enum:bright,dark,both", Default: "bright", Doc: "maxima of the step, minima, or both"},
    },
    AnalysisScale: func(graph.Args) float64 { return 1 },
    Fields:        fixedFields(peakFields),
    Run:           runPeaks, // §10.6
    Doc: "Local maxima of the core/ring step (measure's step) at sub-pixel precision, as disks: …",
    See: []string{"measure", "sheet", "detect.specks", "erode"},
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

// Peaks searches the one-channel buffer L (scale 1); ProbePoint is called on it with CoreRingOptions{Gray: GrayRec709}
// (for one channel the gray plane is the plane itself).
func Peaks(ctx context.Context, L *buffer.Buffer, o PeakOptions) ([]Peak, error)
```

The kernel works in **row bands** (§10.7): for each band of output rows it computes the step map of the band plus a
margin, finds candidates, and measures them, keeping memory at O(width × (band + margin)) beyond the input plane. Each
candidate first gets a cheap **digital-ring snr** (the 10–90 % spread of the digital annulus's pixels, by selection);
only candidates within 20 % of `MinSNR` go on to the exact, coverage-weighted `ProbePoint`, whose numbers are the ones
reported. On the prototype tile `7edbfe9c…`, of 31,854 NMS candidates over the three radii, 3,270 have a digital-ring
snr of at least 0.5 and 122 of at least 1, so the exact measure runs on a few hundred to a few thousand points.

`runPeaks` converts doc px to buffer px (the same numbers at scale 1, offset by the frame origin), calls `Peaks`, and
builds a `RegionsValue` of `Disk` regions (GCV-004's `Region.Shape`) with `ID` = rank and the measures in
`Region.Measures`.

**Time budget.** On 8 cores, a 1024² tile with `radius: [2, 4, 8]` should take at most 0.5 s: the step maps cost about
20 r chord lookups per pixel per radius (280 per pixel for the three radii, about 300 million lookups), NMS about the
same, the pre-filter a few million pixel reads over the ~32,000 candidates the prototype found, and the exact measure a
few hundred to a few thousand points at the reviewer's measured 11–39 µs each (`measure` through JavaScript, 10 cores).
Extrapolated linearly, a 53-megapixel scan takes up to 30 s, within the 60 s eval timeout but slow; the op's doc says to
crop for speed. Phase B2 measures the tile figure with a gated benchmark and records it; there is no hard size limit.

### 7.9 Erosion, dilation, help, and the crop lint

**Kernels** (`pkg/cv/kernels/morph.go`, beside `minMax1D`):

```go
// DiskMinMax is the grey erosion (isMax false) or dilation (isMax true) of a w×h plane by the disk
// {dx² + dy² ≤ r²}; pixels outside the plane take no part.
func DiskMinMax(ctx context.Context, p []float32, w, h int, r float64, isMax bool) ([]float32, error)

// LineMinMax is the same with the line element of radius r at angle theta (radians): van Herk along the
// digital lines of that direction, three comparisons per pixel whatever r is.
func LineMinMax(ctx context.Context, p []float32, w, h int, r, theta float64, isMax bool) ([]float32, error)
```

`DiskMinMax` decomposes the disk into horizontal chords and runs `minMax1D` once per chord and output row (§10.9).
For a plane of only 0 and 1 with r ≥ 8 it computes the same set from `DistanceToSet`, comparing squared distances
(§10.11). `SquareMinMax`, `Opening` and `Closing` stay for the speck background, unexported.

**Ops** (`pkg/cv/ops/morph.go`):

```go
func morphOp(name, doc string, isMax bool) graph.OpDef {
    return graph.OpDef{
        Name: name, On: graph.KindImage, Returns: graph.KindImage, OneChannel: true,
        Args: []graph.ArgSpec{
            {Name: "radius", Type: "number", Unit: graph.DocPx, Positional: true, Min: graph.F(0), Max: graph.F(2000),
                Usual: [2]float64{1, 32}, Doc: "radius of the disk (at most 64), or of the line, in doc px"},
            {Name: "line", Type: "number", Optional: true, Doc: "angle in degrees (counter-clockwise from x): a line element instead of a disk"},
        },
        Validate: func(a graph.Args) error { /* disk (no line) with radius > 64 → E_ARG_RANGE with the crop / twice fix */ },
        Identity: func(a graph.Args) bool { return a.DocLength("radius") == 0 },
        Run:      /* DiskMinMax or LineMinMax on plane 0, at a.Length("radius", scale) */,
        Doc:      doc,
        See:      []string{"erode", "dilate", "feather", "opening", "tophat"},
    }
}
// erode := morphOp("erode", …, false); dilate := morphOp("dilate", …, true)
```

The ops follow their receiver's space and keep its encoding; a one-channel receiver is read as stored (GCV-004 §4.4,
deviation 24).

**Help.** `crop`'s help carries the text of §6.10. The concept entries `opening`, `closing` and `tophat` in `cvFuncs`
(`pkg/gojamod/cv/help.go`) give the compositions of §4.8.

**The crop lint and `Halo`.** The `CROP_FIRST` branch of `orderWarnings` (`pkg/cv/graph/order.go:27-33`), `hasHalo`
(`order.go:56-60`) and `WCropFirst` (`pkg/cv/graph/errors.go:35`) are deleted; `DECORATION_FIRST` stays. With them goes
their only reader's input, `OpDef.Halo` (`pkg/cv/graph/registry.go:281`) and the `Halo` function of every op that
declares one (`grep -n "Halo:" pkg/cv/ops`). Windowed evaluation, the future work GCV-004 §3.3 mentions, can reintroduce a
halo when it exists (D25).

### 7.10 The instructions

`Instructions` is replaced by the text of §12.1 (3,458 bytes). `EvalToolDescription` gains one clause: "Task libraries:
await cv.load(path, args); cv.scripts() lists them." The 4,000-character test stays.

## 8. Decision records

### D1. `cv.load`, not an `eval_file` MCP tool

- **Context:** Every agent pasted the helpers (2,859–3,454 characters). The hand-off asked for an `eval_file` MCP tool.
- **Options considered:** (a) `eval_file`; (b) `cv.load(path, args)` in the `cv` module; (c) both.
- **Decision:** (b).
- **Rationale:** The saving is in not pasting, which both give. Between them the difference is about 20 characters per
  call (`await cv.load("dust/helpers.js")` in an eval). (a) exists only in MCP, cannot be combined with other code in
  one eval, and needs JSON args where `cv.load` passes any JavaScript value; (c) adds a tool for 20 characters.
- **Consequences:** One mechanism in every front end; the instructions teach it.
- **Status:** accepted.

### D2. A script file is an async function body with `args`

- **Context:** Libraries define functions on `mem`; pipelines compute and return.
- **Options considered:** (a) a module with `exports`; (b) a script at global scope; (c) the body of
  `async function (args) { … }`.
- **Decision:** (c).
- **Rationale:** (c) is an eval, which agents know; a library sets `mem.*` or returns an object of functions; a
  pipeline awaits and returns. (a) forces one shape and breaks the existing helpers; (b) leaks `const` and `let` into
  the session's global scope and forbids top-level `await`.
- **Consequences:** Line numbers equal file lines.
- **Status:** accepted.

### D3. No cache: every `cv.load` runs the file

- **Context:** A second load of a library could be skipped.
- **Options considered:** (a) cache by name; (b) cache by name and file stamp; (c) always run.
- **Decision:** (c).
- **Rationale:** A pipeline must rerun with new arguments; re-running a library costs microseconds and picks up edits;
  nodes are hash-consed. The docs churn the review found (a reload replaces every function object) is removed by D8,
  which compares source text, not objects.
- **Consequences:** `cv.load` in a loop runs the file each time.
- **Status:** accepted.

### D4. One path rule for every file goja-cv reads

- **Context:** The hand-off proposed `--scripts DIR` and paths relative to the loading file.
- **Options considered:** (a) a scripts directory; (b) Node's relative rule; (c) GCV-004's `cv.open` rule.
- **Decision:** (c), for `cv.load`, nested loads, `cv.scripts` and `--prelude`.
- **Rationale:** One rule, already implemented with `os.Root`. (a) adds a third kind of directory; (b) gives one string
  two meanings.
- **Consequences:** A library loads a sibling by its root-relative path; libraries are served with `--read`.
- **Status:** accepted.

### D5. `--prelude` is a load at session start that does not consume the docs

- **Context:** `--prelude` exists and reads files outside the roots; it runs through the result processor.
- **Options considered:** (a) keep it as raw eval text outside the roots; (b) `await cv.load(FILE)` at session start,
  the file inside a root, with the function state untouched.
- **Decision:** (b).
- **Rationale:** One mechanism: file:line locations, `UNHANDLED`, `cv.scripts`, nested loads from a prelude. The
  prelude's output is never seen, so it must not mark anything as shown: the agent's first eval shows the docs of
  everything the prelude defined (review F3).
- **Consequences:** The operator names the library's directory with `--read`; `runPreludes` sets `inPrelude`.
- **Status:** accepted.

### D6. Scripts are listed by a bounded function

- **Context:** Agents never call `cv.docs`; the brief asked for a listing; the review (F24) noted it lacks evidence and
  enumerates names under the roots.
- **Options considered:** (a) list scripts in every context block; (b) defer; (c) `cv.scripts()`, named in the
  instructions, with a bounded walk.
- **Decision:** (c).
- **Rationale:** An agent without a playbook needs one way to discover libraries; the instructions point at it. (a)
  repeats a file list in every result. The walk is bounded by entries visited (5,000 per root) as well as files listed
  (500), and lists only `.js` names under directories the operator granted.
- **Consequences:** §17 risk 8 records the enumeration.
- **Status:** accepted.

### D7. A function's doc line is a comment that is the first token of its body

- **Context:** The context block should say what each `mem` function does.
- **Options considered:** (a) the comment above the definition; (b) a property (`mem.probe.doc = …`); (c) the first
  comment anywhere inside the source (revision 1); (d) a comment that is the first token of the body.
- **Decision:** (d), with method shorthand parsed and accessors skipped.
- **Rationale:** goja returns each function's exact source (§4.2). (a) needs a position goja does not expose; (b) is a
  second statement to keep in sync; (c) picks mid-body comments, which in the real helper file would have shown
  `// best by its snr; we report its step` as `mem.probe`'s doc (review F6).
- **Consequences:** Libraries put one comment at the start of each body; the migration note says so.
- **Status:** accepted.

### D8. Docs appear when a name is new or its source changed

- **Context:** Ten docs cost about 800 characters; the context block follows every eval.
- **Options considered:** (a) docs every time; (b) names only; (c) docs when the function object is new (revision 1);
  (d) docs when the name is new or its source text changed, in agent evals only.
- **Decision:** (d), with short signatures (40 characters) on the repeated `functions:` line and `return mem` showing
  all docs on request.
- **Rationale:** An agent needs a doc when the function appears and when it changes. (c) repeats every doc after each
  reload (D3) and lets a prelude consume them (F3, F7). (a) pays the full cost every time.
- **Consequences:** The session keeps `fnSeen map[string]string`.
- **Status:** accepted.

### D9. Tables are automatic literal blocks, exempt from the item and string caps

- **Context:** The cuts the agents hit were the 48-item array cap (40 evals) and the 2,000-character string cap (their
  joined rows); no result reached 9,000 characters.
- **Options considered:** (a) `cv.table`; (b) YAML flow rows with a header row; (c) a literal block chosen
  automatically, exempt from the item and string caps, under a share of the result cap.
- **Decision:** (c).
- **Rationale:** (a) is an API for the most common result. (b) costs brackets and commas and makes the header look like
  data. (c) reads like a printed table, and only an exemption from the two caps that actually fired makes it help.
- **Consequences:** A table is text in the YAML; programs use `cv.save`.
- **Status:** accepted.

### D10. `UNUSED_NODE` is deleted, with no replacement lint

- **Context:** In the study, 18 of 19 `UNUSED_NODE` warnings were false: `speckColor` from a helper's bundle that the
  eval kept in `const v` and used in part. Revision 2 replaced the lint with one that rewrites the agent's source
  (go-go-goja's `jsparse`, `__cv_drop(k, expr)` around expression statements) to report dropped nodes and promises
  (`UNAWAITED`). Under that rule the study's one true positive, `const s = mem.img.view("speckColor")` never used, is
  silent too.
- **Options considered:** (a) `cv.quiet(node)`; (b) attribute warnings to sites in the eval's own code (revision 1);
  (c) a drop lint over rewritten source (revision 2); (d) no lint.
- **Decision:** (d).
- **Rationale:** The warning has no true positives left in the evidence. In v2 a lazy node that nobody uses is a harmless
  no-op: options are literal and only `await`, a look or `cv.save` evaluates anything (GCV-004 §4.1, §6.5), so a dropped
  node cannot produce a wrong result. The mistakes that matter are dropped `cv.load` and `cv.save` promises; the eval
  waits for both through the pending list (D23), and `UNHANDLED` reports their failures. (c) rewrites agent source to
  support a warning, the kind of complexity GCV-005 exists to cut, and it made replsession store marked source text.
  (a) and (b) keep a warning that was wrong 18 times in 19.
- **Consequences:** `unusedNodes` and its machinery are deleted (§7.6, §9); the docs state "a node you don't await, look
  at or save does nothing". A forgotten `await` on a value the agent wanted shows up as a missing value in the result,
  which the agent sees.
- **Status:** accepted (revision 3).

### D11. Labels are data; sheet captions list their cells

- **Context:** Sheet cells were labelled with list indices; agents kept parallel arrays.
- **Options considered:** (a) a `labels` option naming any field; (b) string ids; (c) a `label` field drawn when present,
  else the `id`, plus the cells' labels as text in the caption.
- **Decision:** (c). The `label` field, the string-`id` error and repeatable ids are implemented by GCV-004 Phase 5
  with the `items` op; GCV-005 adds the caption's cell list and drops the `index` label mode.
- **Rationale:** One rule for the two ops that draw items; the label lives in the data. A label drawn in a cell is still
  a number in a picture; the caption makes the mapping text (review F27).
- **Consequences:** Phase A3 is small and depends on GCV-004 Phase 5.
- **Status:** accepted.

### D12. `peaks` returns Regions of disks with `measure`'s fields

- **Context:** Peaks must feed `sheet`, `measure`, `overlay` and `crop`, with numbers that mean what the same names mean
  elsewhere.
- **Options considered:** (a) rows `{ x, y, value, snr }`; (b) rows in the item schema used as literal circles; (c) a
  Regions node of `Disk` regions with the geometry fields and `measure`'s core/ring fields.
- **Decision:** (c).
- **Rationale:** (a) uses `x, y`, which in the item schema are a box corner. (b) loses the fields when measured, since
  `measure` keeps only the geometry of literal items. (c) is what a detector returns: linked items, exact disks, and a
  peak's snr is the number `measure` gives there.
- **Consequences:** Measuring peaks on another image needs a `prefix`. Peaks are the first producer of `Disk` regions
  besides the `items` op; Phase B2 tests every consumer.
- **Status:** accepted.

### D13. Three options: `radius`, `snr`, `polarity`

- **Context:** Candidates were `minDistance`, `snr`, `box`, `max`, `above`.
- **Decision and rationale, one by one:**
  - `radius` kept: the one scale parameter; NMS distance and the overlap rule derive from it.
  - `snr` kept, default 1: the op's own selection; without it the result is every grain maximum (25,449 NMS candidates
    at r = 2 on one prototype tile). The prototype (§3.6) found the default recall-first; §15.2 sweeps 0.5, 1, 1.4 and 2
    on all gold tiles, measuring coverage and peaks not near gold, before the playbook is rewritten.
  - `polarity` kept, as in `detect.specks`; the alternative `x.toGray().mul(-1).peaks()` negates the reported levels.
  - `minDistance` not kept: the overlap rule spaces peaks.
  - `box` not kept: `img.crop(b).peaks()`, now without a crop warning (D22).
  - `max` not kept: items are sorted; `slice` caps.
  - `above` not kept: a global threshold is what issue 4 shows fails.
- **Status:** accepted.

### D14. The local noise is `measure`'s ring spread

- **Options considered:** (a) a global noise (MAD of the step map); (b) a local σ in a window; (c) the 10–90 % spread of
  the ring, as `measure` computes it.
- **Decision:** (c).
- **Rationale:** (a) is the image-global scale again; (b) is a second definition of noise. (c) gives a peak exactly the
  snr `measure` would report. The playbook's thresholds were measured at random places with the helpers' ring (6–12 px
  for core 3), and do not transfer to peaks (§4.6).
- **Consequences:** Next to a strong edge the ring holds both sides and the snr drops; ranking by the speck view's step
  covers that case.
- **Status:** accepted.

### D15. Several radii; overlapping peaks keep the higher snr

- **Context:** Gold radii range from 2.4 to 64 px (median 3.8, 90 % ≤ 6.3, 198 points). A saturated blob searched at a
  small radius gives a ring of rim maxima.
- **Options considered:** (a) one radius per call and a JavaScript merge; (b) a scale-space detector; (c) a list of
  radii, merged by one rule: of two overlapping peaks the higher |snr| wins.
- **Decision:** (c).
- **Rationale:** snr is unitless, so radii compare without normalization; at a radius near the blob's own the ring is
  outside the blob and the snr high, while rim peaks have blob in their rings.
- **Status:** accepted.

### D16. Peaks of any image; polarity bright by default

- **Decision:** peaks of any image through the one luminance (`Luma`); default `bright`.
- **Rationale:** A one-channel receiver is searched as it is, a colour one through its luminance. A peak is a maximum by
  name, and DUST-002 dust is bright.
- **Consequences:** The docs warn that clipped views make the snr meaningless.
- **Status:** accepted.

### D17. Two morphology ops, a disk up to 64, lines of any length

- **Context:** The brief listed erode, dilate, open, close and top-hat with disk and line elements; the review showed
  that grey disks are O(r) per pixel and that opening and closing fall under D19's rule.
- **Options considered:** (a) five ops; (b) four (revision 1: `erode`, `dilate`, `opening`, `closing`); (c) two,
  `erode` and `dilate`, with `{ line: angle }`. Caps: (d) 500 for every element; (e) a 64 doc px disk, lines unbounded
  with van Herk.
- **Decision:** (c) and (e).
- **Rationale:** Opening, closing and top-hat are compositions of the two (D19). `{ line: angle }` cannot express an
  invalid combination. A chord disk of 64 costs about 129 `minMax1D` passes per row, a few seconds on a 53 MP scan; the
  largest gold dust radius is 64 px. Van Herk along lines costs three comparisons per pixel at any length.
- **Consequences:** `E_ARG_RANGE` for larger disks, with "crop first, or apply a smaller disk twice".
- **Status:** accepted.

### D18. `grow` and `shrink` are deleted; the disk means distance ≤ r

- **Context:** With masks as one-channel images, `grow(r)` and `dilate(r)` would be two dilations. `grow(r)` selects (at
  0.5) up to distance r + 0.5, `shrink` the mirror (review F2).
- **Options considered:** (a) keep `grow`/`shrink`; (b) delete them, with the standard disk (distance ≤ r) and no claim of
  equality.
- **Decision:** (b).
- **Rationale:** One family; the standard convention; a test against a brute-force reference of that convention, not
  against the deleted ops. The distance transform stays as an exact fast path for binary planes.
- **Consequences:** Masks grow by half a pixel less than with `grow` (`dilate(r + 0.5)` gives the old set); the soft
  edge is `feather(1)`.
- **Status:** accepted.

### D19. No opening, closing or top-hat ops

- **Context:** Each is one existing op applied to the result of another.
- **Options considered:** (a) ops; (b) compositions with `cv.help` concept entries.
- **Decision:** (b).
- **Rationale:** GCV-004's "one way per thing"; the composition reads as its definition; concept entries make
  `cv.help("opening")` find it.
- **Status:** accepted.

### D20. No median or rank filter now

- **Decision:** none now. Min and max filters are `erode` and `dilate`; the top-hat gives a background without the speck
  for bright structures. A median would reuse the disk's chords with a sliding histogram when a use appears.
- **Status:** accepted.

### D21. No `white` option for the speck view

- **Decision:** a doc note (§6.10) and the local snr of `peaks`.
- **Rationale:** The measured alternative was worse (87 % against 91 %); other scales are `img.dog(1, 4).levels({ black:
  0, white })` or `img.crop(b).view("speck")`.
- **Status:** accepted.

### D22. `CROP_FIRST` is deleted

- **Context:** The lint warns for every op with a `Halo` after a crop and tells the agent to crop last; `peaks`,
  morphology and views need cropping first for a local search or scale, and for Regions results the fix is meaningless
  (review F1).
- **Options considered:** (a) exceptions (no `Halo` for peaks, skip non-image results); (b) delete the lint and document
  the behaviour in `crop`'s help.
- **Decision:** (b).
- **Rationale:** Cropping first is intended, not a mistake. Exceptions would leave a lint whose advice is right for blurs
  and wrong for everything GCV-005 adds; the help text states the actual behaviour once.
- **Consequences:** `OpDef.Halo` loses its only reader and is deleted (D25).
- **Status:** accepted.

### D23. Evals wait for their loads and saves; one `UNHANDLED` warning

- **Context:** A load still running when the eval ends leaks its nodes, looks and errors into a later eval (review F8);
  a failed unawaited load or save loses its error.
- **Options considered:** (a) a `LOAD` warning for pending and failed loads (revision 1); (b) `__cv_run` waits for the
  eval's pending promises, and goja's rejection tracker reports any promise rejected without a handler.
- **Decision:** (b).
- **Rationale:** Waiting removes the pending case; the tracker is one mechanism for loads, saves (GCV-005 open question
  4) and helpers' own promises.
- **Consequences:** `cv.load` returns a promise derived from the file's, so the tracker sees the caller's handling.
- **Status:** accepted.

### D24. The load limit is nesting depth on a JavaScript stack

- **Context:** Revision 1 counted running loads, which refused 33 parallel loads (review F9).
- **Decision:** a load stack in `prelude.js`, pushed around the synchronous part of each file; depth 32.
- **Rationale:** The hazard is synchronous recursion through the native call; asynchronous recursion is bounded by the
  eval timeout. Calling the file's function from JavaScript keeps the count in JavaScript (review F32).
- **Status:** accepted.

### D25. `OpDef.Halo` is deleted

- **Context:** After D22 the only possible reader of `OpDef.Halo` is help output; GCV-004 says windowed evaluation, the
  feature a halo exists for, is future work.
- **Options considered:** (a) keep `Halo` and print it in help (revision 2); (b) delete it.
- **Decision:** (b).
- **Rationale:** A field read only to be printed is metadata every op must keep correct for no behaviour. When windowed
  evaluation exists it will define the halo it needs and test it. `crop`'s help explains the edge effect in words: an
  op after a crop sees only the crop.
- **Consequences:** Every op's `Halo` function goes; `peaks`, `erode` and `dilate` declare none.
- **Status:** accepted (revision 3).

## 9. Legacy and complexity to delete

| # | phase | delete | where | lines (est.) |
|---|---|---|---|---|
| 1 | A1 | prelude file reading and the "not subject to roots" path | `session.go:87`, `withDefaults`, `runPreludes` (`226-257`), `New` | ~20 |
| 2 | A2 | `UNUSED_NODE` entirely: `unusedNodes`, its call, its walk of `mem`, `feeds`, `onPlaceholder`, `WUnusedNode`, `EvalTrace.Created` if unread; tests and docs that expect it | `processor.go:264-396`, `evaltrack.go`, `graph/errors.go` | ~110 |
| 3 | A2 | "return cv.json(x) for all" in the item and string cap notes, and in `capText` (if GCV-004 Phase 6 has not replaced them) | `yaml.go:155`, `180`, `290` | ~3 |
| 4 | A3 | the sheet's `index` labels and input-dependent default | `sheet.go:297` and its default logic (the list path itself goes in GCV-004 Phase 5) | ~10 |
| 5 | B1 | `CROP_FIRST`: its branch of `orderWarnings`, `hasHalo`, `WCropFirst`; its tests in `pkg/cv/graph/crop_test.go`; its mention in `pkg/cv/docs/topics/06-looks.md` | `order.go:27-33`, `56-60`; `errors.go:35` | ~25 |
| 6 | B1 | `grow`, `shrink` | `pkg/cv/ops/mask.go:338-339` | ~2 (+ docs, help) |
| 7 | B1 | `kernels.Grow`, `kernels.Shrink` | `pkg/cv/kernels/mask.go:411-449` | ~40 |
| 8 | B1 | `OpDef.Halo` and every op's `Halo` function | `registry.go:281`, `pkg/cv/ops/*.go` | ~30 |
| 9 | B1 | exported `SquareMinMax`, `Opening`, `Closing` (unexported, kept for the speck background) | `morph.go:54-107` | renames |

Tests that use deleted names and must change in B1: `pkg/cv/ops/regions_test.go:74` (`toMask({ grow })`, or its
GCV-004 successor), `pkg/cv/kernels/mask_test.go:66-95` (`TestGrowThenShrinkIsNearIdentityOnBlobs`, rewritten for
`DiskMinMax`), `pkg/cv/ops/ops_test.go:148-149, 161, 296` (registry-walk arguments and a `grow` RANGE case),
`pkg/cv/ops/heal_test.go:76` (`toMask({ grow: 2.5 })`), `pkg/cv/graph/crop_test.go` (`CROP_FIRST` cases).

Nothing keeps an old name: `grow`, `shrink`, `labels: "index"` get edit-distance suggestions only (GCV-004 D12).

**Estimate.** Deleted: about 250 lines of non-test code. Added: `cv.load`, `cv.scripts`, compile and pending (~180),
locations (~60), settled evals and `UNHANDLED` (~60), context block functions and
`DescribeFunction` (~100), tables (~130), the sheet caption (~20), morphology (~200), peaks (~300). Net about +850; tests
about +1,150. Treat as ±30 %; the implementation diary records `git diff --stat` per phase.

## 10. Pseudocode for the key flows

### 10.1 `cv.load`

The JavaScript half is in §7.2. The native half:

```
binding.compile(path):                                      (owner goroutine, inside export())
    path is not a string                      → E_ARG_TYPE  fix: await cv.load("dir/file.js")
    path does not end in ".js", or contains whitespace or "(" or ")"
                                              → E_ARG_TYPE  "cv.load runs .js files whose names have no spaces or parentheses"
    f, loc, err := eng.Workspace().Open(path)               // GCV-004: workspace, then --read in order
        outside every root                    → E_ACCESS (GCV-004's message)
        missing                               → E_ARG_TYPE "no such file", listing the roots searched
    text := read f (at most 1 MiB, else E_ARG_TYPE "too large for a script")
    name := loc.Rel ("/"-separated); "read<N>:" + loc.Rel when an earlier root has the same relative path
    prg, err := goja.Compile(name, "(async function (args) {" + text + "\n})", false)
    err is a syntax error                     → throw { code: "SyntaxError", message: name + ":" + L + ":" + C + ": " + msg }
    files[name] := split(original text, "\n")
    return { name, run: AssertFunction(vm.RunProgram(prg)) }

binding.pending(p):  trace.Pending += p                     // also appended by GCV-004's cv.save
binding.tooDeep(stack): throw E_TOO_MANY "cv.load is nested 32 deep: a.js → b.js → a.js → …; does a file load itself?"
```

### 10.2 Locations of an error

```
readFailure(v):                                             (pkg/cvsession/native.go)
    … name, code, message, hint, fix, path, valid as today …
    frames := every match of `([^\s()]+):(\d+):(\d+)` in v.stack, innermost first
    known(src) := src == "<eval>" || files has src
    loc := first frame with known(src)
    if loc.src != "<eval>": calledFrom := first frame with src == "<eval>"

errorNode(f):
    where := f.Source == "<eval>" ? "line " + agentLine(f.Line) + ": " + sourceLine
                                  : f.Source + ":" + f.Line + ": " + trim(files[f.Source][f.Line − 1])
    if calledFrom: "called from" := "line " + agentLine(calledFrom.line) + ": " + sourceLine
```

### 10.3 The table renderer

```
renderer.array(arr, depth, path):
    if tableOf(arr): return r.table(arr, path)
    … as today; notes name cv.save

tableOf(arr):
    arr.length ≥ 2; every element: an object exported as map[string]any (not a node, look, provider, error,
    function or array) whose own string keys all have data descriptors holding undefined, null, boolean, number or string

table(arr, path):
    cols  := union of the elements' keys, first-seen order
    cell(v) := undefined | null | missing → "-";  number → FormatNumber(v);  boolean → "true" | "false"
               string → s == "" || s == "-" || contains whitespace → strconv.Quote(s) else s;
                        more than 60 runes → first 59 runes + "…"
    width[j] := max rune count of cols[j] and of cells[i][j], i < min(n, 200)
    line(cs) := join(cs[j] right-padded to width[j] runes, "  "), trailing spaces trimmed   // yaml.v3 writes a literal
                                                                                           // block only without them
    text := line(cols); used := runes(text) + 1; shown := 0
    for each element:
        l := line(cells);  if shown ≥ 5 and used + runes(l) + 1 > r.tableLeft: break
        text += "\n" + l;  used += runes(l) + 1;  shown++
    r.tableLeft −= used
    if shown < n:
        name := path != "" ? path : "this array"
        text += "\n… rows 1–" + shown + " of " + n + " shown. Next: " + name + ".slice(" + shown + ", " + 2·shown +
                "); all: await cv.save(\"rows.csv\", " + name + ")"
    return yaml.Node{ Kind: Scalar, Style: Literal, Tag: "!!str", Value: text }    // not through primitive(): no string cap
```

### 10.4 A function's signature and doc line

```
DescribeFunction(fn, name) → (sig, doc):
    src := Function.prototype.toString.call(fn)
    src contains "[native code]"      → (name + "()", "")
    head := src without a leading "async "
    params, bodyStart :=
        head starts with "function"           → text inside the first balanced "( )"; body after the next "{"
        head starts with "("                   → the balanced "( )"; body after "=>" (and its "{" if any)
        head starts with an identifier + "=>"  → that identifier; body after "=>"
        head starts with an identifier + "("   → method shorthand: the balanced "( )"; body after "{"
        head starts with "get " or "set "      → not listed
    sig := name + "(" + collapse whitespace(params) + ")"
    doc := skip whitespace from bodyStart; if the next token is "//": the rest of that line, trimmed;
           if it is "/*": the first non-empty line inside, without leading "*" and spaces; otherwise ""
           cut at 120 runes with "…"
    the repeated functions: line cuts sig at 40 runes with "…"; new functions: and results show it whole
```

### 10.5 (removed in revision 3)

Revision 2's drop lint lived here. Revision 3 deletes `UNUSED_NODE` with no replacement (D10); the section number is
kept so that references to §10.6–§10.12 stay valid.

### 10.6 Settled evals and unhandled rejections

```
InitRuntime:  vm.SetPromiseRejectionTracker(func(p, op) {
                  op == Reject: st.unhandled[p] = p.Result()
                  op == Handle: delete(st.unhandled, p) })

__cv_run(body):  (§7.4) body → settle(allSettled(native.pending())) → value or rethrow

Process / ProcessFailure:
    for p, reason in st.unhandled (in rejection order):
        reason is the eval's failure value → skip
        f := readFailure(reason)
        warning: "UNHANDLED " + location(f) + ": a promise was rejected and never handled: " + f.Message +
                 ". fix: await the call that returned it"
    clear st.unhandled
```

### 10.7 Peaks in row bands

The digital disk of radius ρ is the union of 2⌊ρ⌋ + 1 horizontal chords; the chord at row offset dy has half-width
a(dy) = ⌊√(ρ² − dy²)⌋. With row prefix sums, each chord costs two lookups.

```
kernels.Peaks(L, o):                                         (L: one-channel buffer at scale 1)
    all := []
    for r in o.Radii:
        M := ⌈2.5 r⌉ + ⌈r⌉ + 1                               // margin: ring of the step map + NMS + parabola
        for each band of B output rows [y0, y1) (B = 64; bands run in parallel):
            P := row prefix sums (float64) of rows [y0 − M, y1 + M) ∩ image
            for y in [y0 − ⌈r⌉ − 1, y1 + ⌈r⌉ + 1) ∩ image, every x:
                S[y][x] := diskMean(P, r) − ringMean(P, 1.5 r, 2.5 r)     // means over in-image pixels; counts from
                                                                          // the chord widths, clipped at the border
            for s in signs(o.Polarity):  T := s · S
                D := DiskMinMax(T, r, max) on the band (its margin rows included)
                for (x, y) in the band with T > 0 and T == D:
                    (dx, dy) := parabola(T, x, y)                          // §10.8
                    q := digital-ring snr at (x, y): s · T / max(p90 − p10 of the digital ring's pixels, 1e-3)
                    if q < 0.8 · o.MinSNR: continue                        // pre-filter; the exact measure decides
                    m := ProbePoint(L, x + 0.5 + dx, y + 0.5 + dy, { Core: r, R1: 1.5 r, R2: 2.5 r, Gray: GrayRec709 })
                    if m.Outside or s · m.Step ≤ 0 or s · m.SNR < o.MinSNR: continue
                    all += Peak{ x + 0.5 + dx, y + 0.5 + dy, r, m }
    sort all by |snr| desc, |step| desc, cy, cx
    kept := [];  grid := buckets of side 2 · max(radii)
    for p in all: if no k in the 3 × 3 buckets around p with dist(p, k) < p.R + k.R: kept += p
    return kept
```

**Cost.** Three digital disks of radii r, 1.5 r and 2.5 r have 2⌊ρ⌋ + 1 chords each, two lookups per chord: about 20 r
lookups per pixel per radius (66 at r = 3, about 70 million per 1024² tile), and 280 per pixel for `radius: [2, 4, 8]`.
NMS by `DiskMinMax` costs about (2r + 1) · 4 operations per pixel. Memory beyond the input plane is O(width × (B + 2M))
per band in flight. §7.8 gives the time budget.

### 10.8 Sub-pixel refinement

```
parabola(T, x, y):
    for each axis, with a, b, c the values at −1, 0, +1 along it (on the border: δ = 0):
        den := a − 2b + c
        δ := den < 0 ? (a − c) / (2 · den) : 0;  δ := clamp(δ, −0.5, 0.5)
    return (δx, δy)
```

### 10.9 Disk erosion and dilation by chords

```
DiskMinMax(p, w, h, r, isMax):
    if r < 0.5: return copy(p)
    if r ≥ 8 and every value of p is 0 or 1: return binaryDisk(p, w, h, r, isMax)       // §10.11
    R := ⌊r⌋;  better := isMax ? max : min;  pad := isMax ? −∞ : +∞
    for each output row y (parallel):
        out[y][*] := pad
        for dy in −R..R with 0 ≤ y + dy < h:
            a := ⌊√(r² − dy²)⌋
            run := minMax1D(row y + dy, half-width a, isMax)     // morph.go:14, three comparisons per sample
            out[y][x] := better(out[y][x], run[x]) for every x
```

Cost: (2R + 1) passes per output row. For r = 12 on a 1024² tile, about 100 million operations; for the cap r = 64 on a
53 MP scan, about 129 × 4 × 53 million ≈ 27 billion, a few seconds on 8 cores (§17, risk 3).

### 10.10 Line erosion and dilation (van Herk along digital lines)

```
LineMinMax(p, w, h, r, θ, isMax):
    u := (cos θ, −sin θ)                                     // y grows downwards; θ counter-clockwise on screen
    if |u.x| ≥ |u.y|:                                        // mostly horizontal: one sample per column
        slope := u.y / u.x;  R := round(r · |u.x|)           // R steps of length 1/|u.x| span about r
        for every offset y0 such that the line y = y0 + round(x · slope) meets the image:
            seq := the pixels (x, y0 + round(x · slope)) inside the image, in order of x
            out[seq] := minMax1D(p[seq], half-width R, isMax)
    else: the same with x and y exchanged
```

Each pixel lies on exactly one line of the family, so every pixel is written once. The element of a pixel is the run of
2R + 1 samples of its own digital line, whose end centres are about 2r apart. The brute-force reference of the tests
walks the same lines.

### 10.11 The binary fast path

```
binaryDisk(p, w, h, r, isMax):
    r2 := r · r
    if isMax: d := DistanceToSet(p == 1);  out := d·d ≤ r2 + 1e-6 ? 1 : 0     // dilation: within r of a 1
    else:     d := DistanceToSet(p == 0);  out := d·d > r2 + 1e-6 ? 1 : 0     // erosion: no 0 within r
```

`DistanceToSet` (`mask.go:368`) returns float32 square roots of integer squared distances; comparing squares with a
small tolerance keeps boundary pixels identical to the chord path.

### 10.12 Two traces

**`await img.as("linear").peaks({ radius: [2, 4], snr: 1 })` on an RGB density tile.**

```
cv.open(tile.tif)          open         linear-encoded, channels 3
as("linear")               identity (linear-encoded)
toGray(rec709)             the luminance input (GCV-004 Luma), channels 1
peaks                      Regions, AnalysisScale 1, 16 bands of 64 rows
  r = 2: step map → NMS → 25,449 candidates (prototype, tile 7edbfe9c…) → pre-filter → exact → 100 at snr ≥ 1
  r = 4: step map → NMS → 5,371 candidates                          → pre-filter → exact → 18
  merge: cores must not overlap → about 105 peaks, ids by |snr|
awaited                    [{ id 1, x, y, w, h, cx 2278.43, cy 333.12, area, aspect 1, r 2, coreMean, …, snr 11.9, …, [ITEM]: [n, 0] }, …]
returned                   a table of about 105 rows, under the 6,000-character budget
```

**`L.sub(L.erode(12).dilate(12))` on a saturated blob of radius 10.**

```
L.erode(12)      the blob vanishes (the disk does not fit); the background stays, slightly darker at its own small detail
.dilate(12)      the background is restored, without the blob: the opening
L.sub(…)         the blob as a filled plateau on zero; anything wider than 25 px removed
peaks r = 8      one peak at the plateau's centre, its ring outside the blob: a high snr
```

## 11. Errors and diagnostics

| code | when | example and fix |
|---|---|---|
| `E_ARG_TYPE` | `cv.load` with a non-string path, a path not ending in `.js` or containing whitespace or parentheses, a missing file (listing the roots searched), a file over 1 MiB | `cv.load("helpers")` → fix `await cv.load("helpers.js")` |
| `E_ACCESS` | `cv.load` of a path outside the workspace and the read directories (GCV-004's message) | fix: add the directory with `--read DIR` |
| `SyntaxError` | a loaded file does not compile; the message starts with `name:line:col`, `where` is the eval line of the `cv.load` | `dust/helpers.js:12:5: Unexpected token )` |
| `E_TOO_MANY` | the load stack would exceed 32 | "cv.load is nested 32 deep: a.js → b.js → a.js → …; does a file load itself?" |
| `E_WRONG_KIND` | `erode`, `dilate` on a colour image (GCV-004's `OneChannel`) | `img.erode(2)` → fix `img.toGray().erode(2)` |
| `E_ARG_RANGE` | a disk radius above 64; a `peaks` radius outside [1, 64] or `snr` below 0 | `L.dilate(100)` → fix "crop first, or `L.dilate(64).dilate(36)`" |
| `E_FIELD_EXISTS` | `measure` on peaks without a prefix | fix `{ prefix: "sp" }` |
| `E_UNKNOWN_METHOD` | `m.grow(2)`, `m.shrink(2)`, `x.opening(3)` | edit-distance suggestion; `cv.help("opening")` finds the concept entry |
| `E_UNKNOWN_OPTION` | `cv.sheet(…, { labels: "index" })` (an enum error) | valid values `id`, `none` |

Errors raised inside a loaded file carry `where: name:line: text` and `called from: line N: text`; errors in a prelude
read `prelude name:line: message`. The non-numeric-`id` error belongs to GCV-004 Phase 5.

Warnings:

| code | when | change |
|---|---|---|
| `UNUSED_NODE` | — | deleted (D10) |
| `UNHANDLED` | a promise was rejected and never handled during the eval | new; covers `cv.load` and `cv.save` |
| `CROP_FIRST` | — | deleted |

Every warning raised by a call in a loaded file carries `name:line`.

## 12. Documentation to change

Agents read the MCP instructions, the eval tool description, the context block, error fixes and the comments of the
libraries they load. People read the README and the guidebook (`goja-cv help`). Every ```js block in
`pkg/cv/docs/topics` and `pkg/doc` runs in the docs tests (`pkg/cvsession/docs_test.go`), so each phase updates the
blocks it breaks.

### 12.1 The MCP instructions

The text below replaces `Instructions` (`reference.go:15-42`): GCV-004's v2 contract plus GCV-005's additions, 3,458
bytes. If GCV-004 Phase 8's text differs, keep its contract lines and add these; the 4,000-character test stays. If Part
A lands before Part B, the peaks and morphology lines come with Phase B3.

```text
cv is a lazy image-analysis API. You drive it with the eval tool.

Eval contract
- The code is the body of an async function: use await and end with return. The value comes back as YAML (max 9000 chars; a list of flat rows comes back as a table, paged), then warnings (with your line) and a context block (images, mem, and the docs of new mem functions).
- const/let do not survive between evals: keep values on mem.
- Calls build lazy, immutable nodes; a node you don't await, look at or save does nothing. await gives data: a Value or Regions node is computed at full resolution (regions become an array of flat items { id, x, y, w, h, cx, cy, r, … }); await img gives the image's summary. Options are literal values: levels({ white: (await d.stats()).p99 }).
- node.look(label | { zoom, ruler }) shows an image; every look is delivered, at most 4 per eval. Never read a coordinate off a picture: get it from data (peaks, items, measure).
- Coordinates, lengths and boxes are doc px (original-image pixels).
- Errors are YAML with code, message, where (your line, or file:line in a loaded file), hint and a literal fix: apply the fix.

Task libraries
- await cv.load("dir/file.js", args) runs a script file in the session (a task library that defines mem functions, or a saved pipeline); cv.scripts() lists the .js files you can load with their first comment line. Use a library's functions when a playbook names one; never paste a file into an eval.

Images, spaces, files
- img is the open image; cv.open(path, { name }) opens another (workspace first, then --read directories). Images open in file units; img.as("linear") converts; every op works in its receiver's space.
- await cv.save("out/x.tif" | ".json" | ".csv", x) writes into the workspace.

Most used calls
- await img.stats({ box, mask }) → { meanL, p01, p99, … }; img.histogram().look()
- img.levels({ black, white, auto }), img.exposure(ev), img.gaussian(sigma), img.toGray(), x.colormap("viridis")
- Magnify: img.crop([x, y, w, h]).look({ zoom: 8, ruler: true })
- Views: img.view("speck" | "speckDark" | "signed" | "detail" | "scene"); img.dog(1, 4); the speck view's scale is set over the whole image.
- Find spots: await img.as("linear").peaks({ radius: 3, snr: 1.4 }) → items with sub-pixel cx, cy, r, step, snr (step against the local surroundings); radius: [2, 4, 8] for mixed sizes; on img.crop(b) it searches only the crop. img.detect.specks() → regions.
- Many candidates in one picture: cv.sheet(items.slice(0, 15), { views: [img, img.view("speck")] }).look(); cells are labelled with each item's label or id.
- Measure: await img.as("linear").measure(items) (step, spread, snr around each item, sized from r); img.profile(a, b, { width }) across a line; img.sample(points).
- Filter, sort and page items with JavaScript: items.filter(r => r.snr >= 1.4 && r.aspect < 3).
- Masks are one-channel images: img.mask.luminance({ above }), mask.box, mask.regions(items); m.and/or/minus/not, m.feather(r), m.components(), m.coverage().
- Morphology on one-channel images: x.erode(r), x.dilate(r) (a disk; { line: angle } for a line). Opening: x.erode(r).dilate(r). Top-hat: x.sub(x.erode(r).dilate(r)) keeps bright things smaller than r, filled.
- img.overlay(items, { labels: "id" }).look() draws items; img.within(mask, i => i.exposure(-0.4)) edits inside a mask.

More: cv.help(name) (one op or function), cv.docs.search("width of a line"), cv.docs.read("recipes"); resources cv://docs/<id>, cv://reference, cv://guide/<slug>.
```

### 12.2 Topics, pages and help

| document | change | phase |
|---|---|---|
| `pkg/cv/docs/topics/14-task-libraries.md` (new) | §12.3: script files, `args`, return, `mem`, doc lines, `cv.scripts`, `--prelude`, `file:line` errors, settled evals, `UNHANDLED`, no cache | A1 |
| `01-getting-started.md` | the eval contract: tables, `cv.load`, the context block's functions | A1, A2 |
| `03-nodes-and-evaluation.md` | the rule "a node you don't await, look at or save does nothing"; `UNUSED_NODE` gone | A2 |
| `05-masks-and-regions.md` | `dilate`/`erode` replace `grow`/`shrink` (line 17), `mask.regions(items).dilate(2)` | B1 |
| `06-looks.md` | the `CROP_FIRST` mention goes; sheet captions list their cells | A3, B1 |
| `08-errors.md` | §11's rows | A1, A2, B1 |
| `09-recipes.md` | dust candidates with `peaks`; the speck step of peaks; the thread width (§12.4); removing lines; a top-hat for saturated specks | B3 |
| `11-views.md` | the speck scale note and rings (§6.10) | B3 |
| `12-measuring.md` | `peaks`; `profile` for widths | B3 |
| `02-units-and-scales.md` | "a grow radius" → "a morphology radius" (line 7) | B1 |
| `pkg/doc/03-cli.md` | `--prelude` is a load; the file lies in `--root` or a `--read` directory (lines 73-75, 131) | A1 |
| `pkg/doc/04-agents.md` | task libraries, with the study's numbers | A4 |
| `pkg/doc/05-setup-claude-code.md`, `06-setup-codex.md`, `07-setup-pi.md` | registration with `--read DIR --prelude DIR/lib.js`, and when to use it ("give agents a task-specific helper library") | A4 |
| `pkg/doc/08-recipes.md` | the playbook pattern (§12.5) | A4 |
| `pkg/doc/10-troubleshooting.md` | `UNHANDLED`, a prelude outside the roots, `E_TOO_MANY` from loads | A4 |
| `README.md` | MCP section: `--prelude` and `cv.load` (lines 81, 98) | A4 |
| `pkg/cvsession/reference.go` | `Instructions` (§12.1), `EvalToolDescription` | A4, B3 |
| help (`pkg/gojamod/cv/help.go`) | entries `load`, `scripts`; concept entries `opening` (`x.erode(r).dilate(r)`), `closing` (`x.dilate(r).erode(r)`), `tophat` (`x.sub(x.erode(r).dilate(r))`, black `x.dilate(r).erode(r).sub(x)`); `crop`'s text of §6.10 | A1, B1 |
| op docs | `peaks`, `erode`, `dilate`, the speck view's `Doc`, `sheet`'s `labels` | B1, B2, A3 |

### 12.3 Recipe: writing a task library

```js
// dust/helpers.js: helpers for annotator agents on DUST-002 gold tiles (goja-cv v2)
// Load: await cv.load("dust/helpers.js")  or  goja-cv mcp … --read DIR --prelude DIR/dust/helpers.js
// The comment that opens each function's body is its doc line in the context block.

mem.tile = (id, ox, oy) => { // open a tile at its origin: every coordinate is an absolute scan coordinate
  mem.img = cv.open("cv_tiles/" + id + ".tif", { encoding: "linear", origin: [ox, oy], name: "img" })
  mem.lin = mem.img.as("linear")
  return mem.img
}

mem.spots = async ({ snr = 1, radius = [2, 4, 8] } = {}) => { // peaks with the speck view's step: sp = spStep
  const pk = await mem.lin.peaks({ radius, snr })
  return await mem.lin.view("speck").measure(pk, { prefix: "sp" })
}

mem.sheet = (items, size = 40) => // contact sheet of up to 15 items: tile | speck view, captioned with their ids
  cv.sheet(items.slice(0, 15), { views: [mem.img, mem.img.view("speck")], size, zoom: 4, stretch: "cell" })

return "dust helpers ready"
```

The topic states the rules: a script file is an eval body with `args`; keep state on `mem`; open each function's body
with its doc comment; return a short string or an object; load siblings by root-relative path; `cv.load` runs the file
every time; warnings and errors from the file carry `file:line`.

### 12.4 Recipe: the width of a thread

Issue 10's answer. `profile` (GCV-004: `{ from, to, step, width, rows }`, rows `{ t, cx, cy, meanL, … }`) samples across
the thread; the width is the full width at half height above the baseline of the two ends.

```js
// dust/thread-width.js: the width of a bright thread across the segment a → b (doc px), full width at half height
// args: { a: [x, y], b: [x, y], along: 3 }   (a and b on either side of the thread; along: px averaged along it)
const p = await img.as("linear").profile(args.a, args.b, { step: 0.25, width: args.along ?? 3 })
const v = p.rows.map(r => r.meanL), t = p.rows.map(r => r.t), n = v.length
const base = (v[0] + v[n - 1]) / 2
let k = 0
for (let i = 1; i < n; i++) if (v[i] > v[k]) k = i
const half = base + (v[k] - base) / 2
const cross = d => {                                // where the profile falls below half, walking from the peak
  let i = k
  while (i + d >= 0 && i + d < n && v[i + d] > half) i += d
  const j = i + d
  if (j < 0 || j >= n) return null                  // it never does: the segment is too short
  return t[i] + (t[j] - t[i]) * (v[i] - half) / (v[i] - v[j])
}
const lo = cross(-1), hi = cross(1)
return { width: lo === null || hi === null ? null : hi - lo, height: v[k] - base, at: [p.rows[k].cx, p.rows[k].cy] }
```

### 12.5 Playbook pattern

`pkg/doc/08-recipes.md` gains a short pattern for people who write agent playbooks:

1. Register the server with the task library as a prelude, so agents never paste it.
2. Name the library's functions and what they return in the playbook; agents use what the playbook names.
3. Give agents coordinates as data: candidates from `peaks` or a library function, sheets of those items (whose
   captions list the ids), and the instruction "never read a coordinate off a ruler; take it from the rows".
4. For threads: pick two points either side of the thread from a zoom's data, and run
   `await cv.load("dust/thread-width.js", { a, b })` at two or three places along it.
5. Return rows, not strings: the result renders them as a table.

## 13. Implementation phases

Every phase ends green on `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, `make lint`,
`make logcopter-check` (after `make logcopter-generate` when instrumented functions changed) and `make smoke`, is one
commit (or a few) followed by an implementation diary step with `git diff --stat`, and updates the docs blocks and
in-code strings it breaks. GCV-004's gate script (`ttmp/2026/11/09/GCV-004--…/scripts/04-gate.sh`) runs the race tests
with the real-data directories pointed at nothing; run the gated real-data tests separately
(`ttmp/2026/11/09/GCV-004--…/scripts/05-real-data.sh`). Write the listed tests first and watch them fail.

**Independence and dependencies.** Part A (the session, the module's JS surface, the sheet caption) and Part B (kernels,
new ops, the crop lint) touch disjoint code; either can land first. Every phase needs some of GCV-004 batch 2:

| phase | content | needs from GCV-004 | needs from GCV-005 |
|---|---|---|---|
| A2 | tables, function docs, context block, `UNHANDLED`; delete `UNUSED_NODE` | Phase 6 (`cv.save`, named by the paging and cut notes) | — |
| A1 | `cv.load`, `cv.scripts`, compile and pending, settled evals, preludes as loads, file locations | Phase 6 (`Workspace`, `cv.save` appends to the pending list) | — |
| A3 | sheet caption's cell list, `labels: id \| none` | Phase 5 (`items\|point[]`, `Point`, `label`, string-`id` error) | — |
| A4 | Part A documentation sweep | Phase 8 (its instructions), or the current text | A1–A3 |
| B1 | `erode`, `dilate`, help concept entries, `crop`'s help; delete `CROP_FIRST`, `OpDef.Halo`, `grow`, `shrink` | Phase 4 (`OneChannel`, implemented) | — |
| B2 | `peaks` | Phase 3 (`Luma`, implemented), Phase 5 (`Disk` regions, links, `measure` radii) | B1 (`DiskMinMax`) |
| B3 | gold-gated tests, `scripts/01-gold-metrics.sh`, Part B docs | — | B1, B2 |
| V | validation (§15) | GCV-004 complete | A1–A4, B1–B3 |

Recommended order: B1 now (its only GCV-004 dependency, Phase 4, is implemented, and GCV-004 Phase 5 does not add
`mask.regions`' `grow`); A2 and A1 after GCV-004 Phase 6; A3 and B2 after Phase 5; then B3, A4 and V.

**Phase A2 — readable results.**
1. Tests: an array of two or more flat records renders as a table with the union of keys, `-` for missing and null,
   quoted strings with spaces, columns aligned by rune count (a cell with `…` and one with `é`), no trailing spaces; a
   200-row table is not cut at 48 rows and not at 2,000 characters; a table over the budget shows at least five rows
   and the paging note with the key path (`rows.slice(118, 236)`) and "this array" at top level; two tables share the
   budget; non-flat arrays and arrays with an accessor property render as before; `DescribeFunction` on arrow,
   `function`, `async`, single-parameter, destructured-parameter and method-shorthand forms, with a doc comment opening
   the body and with a mid-body comment only (no doc), on a bound and a native function, and on a fixture file copied
   from the DUST-002 helpers into `testdata`; the context block lists short signatures, shows docs once, again after the
   source of a function changes, not after an unchanged reload; a getter on `mem` is not run; no eval produces
   `UNUSED_NODE` (`img.gaussian(2)` alone on a line, the `mem.views()` case); `UNHANDLED` for `Promise.reject(new
   Error("x"))` left unhandled, none for a caught rejection, none for the eval's own failure.
2. Implement: `renderer.table`, `tableOf`, `tableLeft`, key paths; `DescribeFunction`; `fnSeen`, `inPrelude`; the
   rejection tracker; delete §9 items 2 and 3. Docs: `01`, `03`, `08`.

**Phase A1 — task libraries.**
1. Tests: `cv.load` of a file in the workspace, in a read directory, shadowed by the workspace (the workspace's runs), by
   absolute path from a shadowed read directory (named `read1:…`), outside every root (`E_ACCESS`), missing (`E_ARG_TYPE`
   listing roots), without `.js`, with a space in its name; `args` by reference and `{}` by default; the return value;
   top-level `await`; `const` stays local; a second load re-reads an edited file; a nested load; a self-loading file
   stops with `E_TOO_MANY` and the stack; 40 parallel loads of one file all succeed; a syntax error names
   `file:line:col`; a runtime error in a file gives `where: file:line` and `called from: line N`, also from a function
   the file defined and the eval called later; warnings in a file carry `file:line`; looks created in a file are
   delivered; an unawaited load that awaits an evaluation inside finishes before the eval's result (its look and its
   `mem` writes are in this eval); an unawaited load that fails gives `UNHANDLED` with the file location; `cv.scripts()`
   lists doc lines, skips `.git` and `node_modules`, omits shadowed files, stops at the entry and file bounds;
   `--prelude` inside a read directory works in `eval`, `run` and `mcp` (default and created sessions), outside every
   root is a usage error naming `--read`, a failing prelude reads `prelude name:line: message`, and the first agent eval
   shows the docs of every function the prelude defined; xgoja loads a file from its `root`.
2. Implement: `cv.load` and `cv.scripts` in `prelude.js`; `binding.compile`, `pending`, `tooDeep`, `scripts` (on
   `Workspace`), the file registry; `__cv_run`'s wait; `readFailure` frames and `called from`; `lineTag` for files;
   preludes as loads with `inPrelude`; `whereLine`; help entries. Delete §9 item 1. Docs: `14-task-libraries.md`, `01`,
   `08`, `pkg/doc/03-cli.md`.

**Phase A3 — sheet captions and labels.**
1. Tests: a sheet of an array lists its cells' labels (else ids) in its caption in cell order, cut after 40 with the
   count; a sheet of a Regions node says `cells in region order`; `labels: "none"` draws none and still lists the cells; `labels: "index"` is an enum error.
2. Implement: the caption list; `labels: id | none`. Delete §9 item 4. If `sheet-2x2.png` moves (its labels), regenerate
   it and record why in the diary.

**Phase A4 — Part A documentation sweep.** The documents of §12.2 marked A4; `Instructions` (§12.1, without the peaks
and morphology lines if Part B has not landed) and its length test; `EvalToolDescription`.

**Phase B1 — erosion, dilation, and the crop lint.**
1. Tests: `DiskMinMax` equals a brute-force min/max over {dx² + dy² ≤ r²} for r from 0.5 to 12 in steps of 0.25 on
   random planes, borders included; the binary fast path equals the chord path for r from 8 to 40 on random binary
   planes; `LineMinMax` equals a brute-force min/max along the same digital lines for angles 0, 22.5, …, 337.5 and r up
   to 60; erosion and dilation are dual (`erode(x) = 1 − dilate(1 − x)` on [0, 1] data); `x.erode(r).dilate(r)` is
   idempotent and anti-extensive, `x.dilate(r).erode(r)` idempotent and extensive; a disk radius of 65 is `E_ARG_RANGE`
   with the fix, a line radius of 500 is accepted; a colour receiver is `E_WRONG_KIND` with the fix; radius 0 returns
   the receiver; a 1 px line vanishes under the disk opening of radius 3 and survives the line opening of radius 5 at
   its angle; the top-hat of a flat disk of radius 4 on a ramp is the disk at full height; `img.crop(b).view("speck")`,
   `img.crop(b).gaussian(2)` and `m.crop(b).dilate(2)` raise no warning; `cv.help("opening")`, `("closing")` and
   `("tophat")` return the compositions; no op declares a `Halo` (the field does not exist).
2. Implement: `DiskMinMax`, `LineMinMax`, `binaryDisk`; `morphOp`, `erode`, `dilate`; help concept entries;
   `crop`'s help; delete §9 items 5–9 and update the tests listed in §9. Golden: add `dilate-mask.png` (a mask dilated by
   a disk) and `tophat-gray.png` (a top-hat of the planted scan's grey image). Docs: `02`, `05`, `06`, `08`.

**Phase B2 — peaks.**
1. Tests: synthetic Gaussian blobs (σ 1–3 px, amplitude 0.05–0.5) at random sub-pixel positions on a noisy flat field:
   every blob found, centre error below 0.25 px (median) and 0.5 px (max) at snr ≥ 5; a peak's `snr`, `step` and `spread`
   equal `img.measure([{ cx, cy, r }])` at the same centre and radius to 1e-9; the pre-filter never drops a peak that the
   exact measure would keep (on the synthetic fields, compared with a run without it); dark blobs with `polarity: "dark"`
   (negative step and snr), both with `both`; a flat disk of radius 10 with `radius: [2, 4, 8]` gives one peak at its
   centre with r = 8, with `radius: 2` a ring of rim peaks; the same blob has a higher snr on a quiet dark background
   than on a noisy bright one (the local-noise property); results are identical with bands of 64 rows and with one band;
   items are sorted by |snr| with ids 1…n; peaks on a crop lie inside it, in doc px; peaks on a colour image equal peaks
   on its `toGray()`; the result is the same at every look scale. Every consumer of `Disk` regions: `overlay` (golden
   `overlay-peaks.png`), `sheet`, `mask.regions`, `measure` (with a prefix), `heal`.
2. Implement: the band step map, `parabola`, the pre-filter, `kernels.Peaks`, the `peaks` op; help; the op's doc; a gated
   benchmark on a real tile whose time the diary records against the 0.5 s budget. Docs: `12`.

**Phase B3 — real data, validation tooling and Part B documentation.** The gated tests of §14.4; `scripts/01-gold-metrics.sh`
(§15.2), building on `scripts/02-peaks-prototype.py`; docs `09`, `11`, `12`; the instructions' peaks and morphology lines
if A4 landed first.

**Phase V — validation.** §15; results in the implementation diary; an "Implementation notes (deviations)" section
appended to this guide.

## 14. Test strategy

### 14.1 Unit and property tests

- **Renderer.** Table detection, formatting, cap exemption, budget and paging; function rendering. A golden text file of
  a representative result (detector items, measured peaks, a nested object with two tables) pins the format.
- **Docs and state.** `DescribeFunction` forms; `fnSeen` by source text; preludes do not consume docs.
- **No `UNUSED_NODE`.** No eval produces it; tests that expected it are changed.
- **Rejections.** `UNHANDLED` for unhandled rejections; none when handled; settled evals.
- **Loading.** Path resolution through the workspace (GCV-004's fixtures), naming, args, returns, re-reading, nesting
  depth, parallel loads, locations.
- **Kernels.** Brute-force equivalence for `DiskMinMax`, `LineMinMax` and the band step map; the binary fast path; the
  morphology laws; peaks' sub-pixel accuracy, equality with `measure`, the pre-filter, the overlap rule, polarity, local
  noise, band invariance.
- **Registry.** GCV-004's `TestRegistryWalk` (deviation 28) evaluates `erode`, `dilate` and `peaks` on colour, grey and
  mask receivers as their declarations allow, at three scales.

### 14.2 Documentation tests

The new topic's blocks run in `pkg/cvsession/docs_test.go` with a temporary workspace holding the model library of §12.3
and the thread-width script of §12.4. The instructions' length test stays at 4,000.

### 14.3 Golden looks

| phase | golden | why |
|---|---|---|
| A3 | `sheet-2x2.png` if its labels move (GCV-004 Phase 5 decides the list items' ids) | labels |
| B1 | new `dilate-mask.png`, `tophat-gray.png` | the new ops |
| B2 | new `overlay-peaks.png` | exact disks of peaks |

Every other golden stays exact.

### 14.4 Gold-based tests (gated)

The gated tests live beside `TestRealViewRanking` (`pkg/cv/ops/viewrank_test.go`) and use its gates: `GOJACV_TILES`
(default `~/dustdet/cv_tiles`) and `GOJACV_GOLD` (default `~/dustds/gold`; labels `{tile}.final.json` or `{tile}.p1.json`,
points `{ x, y, r, type }` in absolute scan coordinates). They skip when either is missing and never assume a number of
gold points: the gold set grew from 142 to 198 points, and three earlier real-data tests went stale on absolute numbers
(GCV-004 deviation 1). Every assertion compares two methods on the same points in one run, or bounds a quantity that
does not depend on the count; every test logs its numbers.

| test | method | assertion |
|---|---|---|
| `TestRealPeaksPosition` | dust points with r ≤ 6: the nearest peak (`img.as("linear").peaks({ radius: [2, 4], snr: 0.5 })`) within r | median distance ≤ 1.5 px, and below the median distance of the probe-snapped hot-list centroid (speck view at 0.3, components, best snr within 2 px) computed in the same test |
| `TestRealPeaksCoverage` | dust points with a peak within max(r, 3) px, `radius: [2, 4, 8]`, `snr: 1` | ≥ the hot list's coverage at level 0.3 minus 2 points, on the same points; logs peaks per tile and peaks not near gold at snr 0.5, 1, 1.4, 2 |
| `TestRealPeaksDarkAreas` | dust points whose ring mean is below the tile's 25th luminance percentile | peaks' coverage ≥ the hot list's on that subset; log only with fewer than 5 points |
| `TestRealTopHatSaturated` | dust points with r ≥ 6 or a core maximum within 2 % of the tile's maximum | coverage by components of `L.sub(L.erode(12).dilate(12))` above its own p99 ≥ the hot list's; log only with fewer than 5 points |
| `TestRealLineOpening` | speck-view components at 0.3 before and after subtracting the max of eight line openings of radius 10 | fewer components with aspect > 5 after; gold coverage after ≥ 98 % of before |

The targets (1.5 px, 2 points, 98 %) come from the evidence (guesses 5–10 px off; the probe snapped within 2 px). If a
target fails on real data, the implementer records the numbers and the coordinator decides; a test is not loosened
silently.

### 14.5 Smoke

`make smoke` gains `goja-cv eval --root $(SMOKE_DIR) --prelude $(SMOKE_DIR)/lib.js -e 'return mem.f(41)'` with a
two-line library.

## 15. Validation plan

### 15.1 Loop 1: an annotator agent and the pi harness

**The agent rerun.** Rerun one annotator agent on tiles from the study with a rewritten playbook (DUST-002
`reference/04`, rewritten by the coordinator: helpers as a prelude, candidates from `peaks`, sheets of items, no ruler
reading) and compare its evals with the original runs, using script 91 on the new transcripts and the review's recount
method on the raw transcripts. Tiles: `cfca85b41cd6f78f_2048_0` (the 24-of-32 tile), `b54a23af8f1608f0_512_512` (the
ring tile), `216610367f9a7bfe_1536_3072` (lines).

| metric | before (all 39 agents) | target after |
|---|---|---|
| helper pastes | 1 per agent (2,859–3,454 characters) | 0 |
| median code characters per eval (non-paste) | 181 | not more |
| evals per tile | 6–14 | not more |
| cut results | the item cap in 40 of 374 evals; the string cap once | no table cut by the item or string cap |
| row lists returned as joined strings | in most `mem.hot` results | none |
| `UNUSED_NODE` | 19 (18 false) | none (the warning is deleted) |
| hand-typed coordinate lists probed in loops | 22 agents | none, or each from data |
| suggestions accepted by the person | per the DUST-002 review | not fewer |

**The pi harness.** The CVJS-001 harness (`CVJS-001…/scripts/06-pi-cv-agent/run.sh`, GCV-004 §15) runs an agent without
a playbook. A new `questions-gcv-005.md` beside the others asks three questions: the five strongest compact bright spots
with coordinates (expected: `peaks`, within 1 px of the reference), the width of the thread through (x, y) (expected:
`profile`, within 0.5 px), and what `lib/tile.js` defines (expected: `cv.load` and the context block). Two runs on the
GCV-005 build; the GCV-004 question files once more to show nothing regressed.

### 15.2 Loop 2: detection quality on gold

`scripts/01-gold-metrics.sh` in this ticket (a `goja-cv run` driver over every gold tile, writing CSV with `cv.save`, plus
a Python summary; `scripts/02-peaks-prototype.py` is its numpy forerunner) computes on the current gold set:

| metric | how | compared with |
|---|---|---|
| position error | distance from each dust point (r ≤ 6) to the nearest peak | the hot list's probe-snapped centroids; the agents' 5–10 px |
| the snr sweep | at `snr` 0.5, 1, 1.4 and 2 (radii [2, 4, 8]): dust coverage within max(r, 3) px, and peaks per tile not within max(r, 3) px of a gold point | the hot list at 0.3 (91 % coverage, median 71 spots per tile on 197 points); the prototype of §3.6 |
| saturated blobs | coverage of the r ≥ 6 or near-maximum subset by peaks with radii and by top-hat components | the hot list |
| dark-area dust | coverage of the dark subset | the hot list |
| scene-line false positives | candidates on components with aspect > 5, before and after line openings | — |
| peaks time per tile | the B2 benchmark on every gold tile | the 0.5 s budget |

The sweep decides `peaks`' default `snr` (kept at 1 until then) and the numbers the rewritten playbook gives; the
random-place numbers of the old playbook are not reused for peaks (§4.6).

## 16. Migration note: the DUST-002 helpers

`DUST-002/scripts/84-cv-helpers.js` belongs to the user's other repository; this ticket does not edit it. After GCV-004
(whose migration note, GCV-004 §16, covers `as("linear")`, arrays and `cv.open`'s `name`) and GCV-005:

| helper | after GCV-005 |
|---|---|
| the header and `return "helpers ready (v1)"` | keep a first comment line (the `cv.scripts` doc); open each function's body with its doc comment (today `mem.probe`'s first inner comment is a mid-body note, which D7 ignores); load with `--prelude` or `await cv.load(…)` |
| `const r3 = …` used by closures | unchanged; private to the file, as intended |
| `mem.tile` | unchanged (`name: "img"`); add `mem.lin = mem.img.as("linear")` |
| `mem.views` | unchanged; `UNUSED_NODE` no longer exists |
| `mem.best` | delete; for a hand-given point, `(await mem.lin.crop([x - 16, y - 16, 32, 32]).peaks({ snr: 0 }))[0]` is the strongest spot near it |
| `mem.probe(x, y)` | `mem.lin.measure([{ cx: x, cy: y, r: 3 }])`; the speck step with `mem.lin.view("speck").measure(…, { prefix: "sp" })` |
| `mem.probeR(x, y, r)` | `mem.lin.measure([{ cx: x, cy: y, r }])` |
| `mem.spots(level)` | `mem.lin.peaks({ radius: [2, 4, 8], snr })` |
| `mem.hot(level, max)` | `const pk = await mem.lin.peaks({ radius: [2, 4, 8], snr: 1 }); await mem.lin.view("speck").measure(pk, { prefix: "sp" })`, sorted by `spStep`; return the rows (a table); no 300-item cap, no edge clamp (`ringCoverage` flags edges) |
| `mem.hotByLevel` | unnecessary: one list, filtered in JavaScript |
| `mem.faint(opts)` | `mem.lin.peaks({ snr })`, with the threshold from §15.2 |
| `mem.sweep(q)` | `mem.img.crop(quarter).view("speck").look({ zoom: 2, ruler: 32 })` sets the quarter's own scale, with no warning now; better, `mem.lin.crop(quarter).peaks()` gives its spots as data |
| `mem.hotLook(level)` | `mem.img.view("scene").overlay(pk.slice(0, 40), { labels: "id", color: "lime" }).look()` |
| `mem.sheet(points, size, label)` | `cv.sheet(rows.slice(0, 15), { views: [mem.img, mem.img.view("speck")], size, zoom: 4, stretch: "cell" })`; the caption lists the ids |
| `mem.zoom(x, y, half)` | unchanged |
| mask `grow(r)` anywhere | `dilate(r)`; `grow(r)` selected up to r + 0.5, so `dilate(r + 0.5)` reproduces it |
| widths of threads | the thread-width script of §12.4 |

The playbook changes the same way: step 1 loses the paste, step 2 becomes "get candidates as data", the warning about
rulers becomes "take coordinates from the rows", and the probe thresholds are replaced by §15.2's.

## 17. Risks and open questions

**Risks.**

1. *Peaks' default snr.* The prototype kept 1,000 peaks on one grainy tile at `snr: 1`. Mitigation: results are sorted
   and the table shows the strongest first; §15.2's sweep decides the default before the playbook changes.
2. *Snr near scene edges.* A speck beside a bright edge has the edge in its ring. Mitigation: rank by the speck view's
   step (`spStep`); the coverage tests compare with the hot list.
3. *Morphology and peaks cost on full scans.* A disk of 64 on a 53 MP scan takes seconds; peaks up to 30 s. Mitigation:
   the disk cap, the binary fast path, bands and the pre-filter; the docs say to crop; the B2 benchmark measures.
4. *Tables as text.* A program that parses the YAML result gets text for a table. Mitigation: programs use `cv.save` or
   `--json` files; the renderer test pins the format.
5. *Pasted code.* All eval code shares the source name `<eval>`, so error lines of a function pasted in an earlier eval
   are mapped against the wrong text (§5.2). Mitigation: libraries are loaded; the instructions say "never paste a file
   into an eval". A complete fix needs a source name per eval in go-go-goja's replsession.
6. *goja behaviour.* `Function.prototype.toString`, `goja.Compile` names in stack frames and the rejection tracker are
   pinned by A1 and A2 tests.
7. *No unused-node lint.* A forgotten `await` on a value the agent wanted is not warned about (D10); the agent sees the
   value missing from its result. Dropped loads and saves are waited for, and their failures reported.
8. *`cv.scripts` enumerates.* It lists `.js` names under granted directories, which `cv.open` cannot; bounded by entries
   and files (D6).
9. *GCV-004 drift.* GCV-005 assumes GCV-004's final model (Appendix A); a phase that meets a GCV-004 deviation follows it
   and records it.

**Decided in revision 3** (the coordinator's decisions on revision 2's open questions):

1. The eval source is not rewritten: `UNUSED_NODE` is deleted (D10), so replsession never stores marked text.
2. Sheet captions list cell ids for arrays; for a Regions node passed directly they say "cells in region order" (§7.7).
3. `cv.save` promises join the pending list in GCV-004 Phase 6 (Appendix A).
4. `OpDef.Halo` is deleted (D25).

There are no open questions left in this revision.

## 18. References

- Ticket: `reference/01-analysis-annotator-agent-feedback.md`, `reference/02-diary-coordination.md`,
  `reference/03-diary-design-writer.md`, `reference/04-review-read-findings.md`, `reference/05-diary-review-read.md`;
  `scripts/02-peaks-prototype.py`.
- The base: the GCV-004 guide (revision 3, deviations 1–28), its implementation diary and gate scripts.
- Evidence: `DUST-002/reference/11-goja-cv-agent-feedback.md`; `DUST-002/various/agent-evals/`;
  `DUST-002/various/agent-diaries/`; `DUST-002/scripts/84-cv-helpers.js`, `91-extract-agent-evals.py`,
  `87-cv-measure-auc.py`, `88-cv-hot-recall.py`; `DUST-002/reference/04-goja-cv-dust-playbook.md`; old issue 18 in
  `DUST-002/reference/07-goja-cv-handover-issues.md`.
- Background: the vault article "Deep Dive — goja-cv in Agent Hands — Feedback from 39 Annotator Agents".
- go-go-goja: `pkg/replsession/evaluate.go`; goja:
  `Runtime.SetPromiseRejectionTracker` (`builtin_promise.go:668`), `Compile`, `Function.prototype.toString`.
- Algorithms: M. van Herk (1992) and J. Gil and M. Werman (1993), running min/max (`minMax1D`); P. Soille, E. Breen and
  R. Jones (1996), erosions along discrete lines at arbitrary angles; P. Felzenszwalb and D. Huttenlocher (2012),
  distance transforms (`dt1d`).

## Appendix A. GCV-004 details this guide relies on

GCV-004 batch 1 (Phases 0a–4) is implemented and merged into this branch at `6bb71a8`; batch 2 (Phases 5–8) is being
implemented.

| detail | GCV-004 | status | used by |
|---|---|---|---|
| literal options; `E_NOT_AWAITED` | §4.1, Phase 1 | implemented | all examples |
| panic boundary (`export`, `isGojaPanic`); ordered `Record.MarshalJSON` | Phase 0b | implemented | `binding.compile`; `cv.save` of peaks |
| `Args.Input/Inputs/Value`, loud accessors | Phase 0c | implemented | the new ops |
| the v1 patch (badge, `stepChroma` gate, `LOOKS`) | Phase 0a (deviation 18) | implemented | sheets |
| `levelsAt` | Phase 1 | implemented | §6.10 |
| `await` only; Values and Regions at scale 1; the look queue and `LOOKS` | Phase 2 | implemented | loads' looks; peaks awaited at full resolution |
| space on the image; `as`; `Follows`; `OpDef.Luma` (`LumaOf`, `LumaWhen`, deviation 20) | Phase 3 | implemented | peaks on colour; morphology follows the receiver |
| channel count; `OneChannel`; masks as one-channel images; luminance inputs never converted (deviation 24) | Phase 4 | implemented | `erode`/`dilate` receivers; mask dilation |
| awaited regions as arrays with `[node, index]` links | Phase 5 | in progress | peaks items |
| the `items` op; literal boxes, circles and points; `Region.Shape` `Disk` and `Point`; one `items\|point[]` binding for `measure`, `sample`, `sheet` | Phase 5 (open question 1 settled there) | in progress | peaks regions; sheets |
| the `label` field, the non-numeric-`id` error, repeatable ids | Phase 5 (open question 2 settled there) | in progress | sheet and overlay labels |
| `measure(items)` with radii from `r`; `prefix`; `E_FIELD_EXISTS` | Phase 5 | in progress | `measure` of peaks |
| `mask.regions(items)` without a `grow` option (the coordinator asked batch 2 not to add it; `dilate` replaces it) | Phase 5 | in progress | B1 deletes nothing there |
| `Workspace`, `--root`, `--read`, `Workspace.Open`, `Location`; `--image`/`--prelude` inside a root | Phase 6 | in progress | `cv.load`, `cv.scripts`, preludes |
| `cv.save`; its promise on the pending list (revision 2's open question 3, satisfied by Phase 6); profile and histogram `rows` | Phase 6 | in progress | notes, settled evals, the thread-width recipe |
| xgoja `root` and `read` | Phase 6 | in progress | `cv.load` in xgoja |
| pruned names; edit distance for removed names | Phase 7 | in progress | `grow`, `shrink`, `labels: "index"` |
| the v2 instructions | Phase 8 | in progress | §12.1 |

## Appendix B. Review disposition (revision 2)

Every finding of `reference/04-review-read-findings.md`, with what this revision did. "Fixed" means the guide now does
what the reviewer proposed or what the coordinator decided in its place; "partly" means the intent is met differently.

| finding | P | disposition | where |
|---|---|---|---|
| F1 crop-first patterns trip `CROP_FIRST` | P1 | fixed (coordinator): the lint is deleted; `crop`'s help documents the behaviour | §5.7, §6.10, §7.9, D22, §9 item 5, B1 |
| F2 `dilate` ≠ `grow` | P1 | fixed (coordinator): the disk is distance ≤ r; tests against a brute-force reference; the migration note says `grow(r)` reached r + 0.5 | §4.7, §6.9, D18, B1, §16 |
| F3 a prelude consumes the docs | P1 | fixed: preludes never update the function state; first agent eval shows all docs; an A1 test | §6.3, §6.4, §7.4, D5, D8 |
| F4 the caps the agents hit | P1 | fixed: tables exempt from the item and string caps; evidence corrected (4,701 characters, string cap, 40 array cuts); a 200-row test | §1, §2.4, §3.1, §3.3, §5.4, §6.5, §7.5, D9, §15.1 |
| F5 evidence counts | P2 | fixed: 39 pastes, 335 evals, regex counts, 19/18 `UNUSED_NODE`, 24/28 of 32, `RANGE` 11, 22 agents; the DUST-002 sources noted | §1, §3, §15.1 |
| F6 doc line from mid-body comments; method shorthand | P2 | fixed: the first token of the body; shorthand parsed, accessors skipped; the helper file as a fixture | §4.2, §10.4, D7, A2 |
| F7 docs repeated after reloads | P2 | fixed: by source text | §6.4, §7.4, D3, D8 |
| F8 loads leak into later evals | P2 | fixed (coordinator): `__cv_run` waits; one `UNHANDLED` warning from the rejection tracker | §4.9, §6.6, §7.4, §10.6, D23 |
| F9 `E_TOO_MANY` counts parallel loads | P2 | fixed: nesting depth on a JavaScript load stack | §6.1, §7.2, D24 |
| F10 attribution silences dropped library results | P2 | fixed (coordinator): revision 2 replaced attribution with a drop lint; revision 3 deletes `UNUSED_NODE` altogether (no true positives left; dropped nodes are no-ops; loads and saves are waited for) | §4.4, §5.5, §6.6, §7.6, D10 |
| F11 phase dependencies | P2 | fixed: A2 needs Phase 6; B1 needs Phase 5 | §13 |
| F12 snr thresholds from random places | P2 | fixed: explained; the validation sweeps 0.5/1/1.4/2 for coverage and peaks not near gold; prototype numbers | §3.6, §4.6, D13, D14, §15.2 |
| F13 peaks cost on a full scan | P2 | fixed: row bands, a pre-filter, a time budget (0.5 s per tile, ≤ 30 s at 53 MP), a benchmark; no hard limit, crop documented | §7.8, §10.7, B2 |
| F14 morphology radius 500 | P2 | fixed (coordinator): disks ≤ 64 with `E_ARG_RANGE` and a fix; lines with van Herk at any length | §6.9, §7.9, §10.10, D17 |
| F15 opening/closing are ops but top-hat is not | P2 | fixed (coordinator): only `erode` and `dilate`; concept entries for opening, closing, top-hat | §4.8, §6.9, D17, D19 |
| F16 terms used before definition | P2 | fixed: §2.8 defines speck view, sp, doc px, NMS, analysis scale, `Halo` and others | §2.8, §4.5 |
| F17 Appendix A and the base stale | P3 | fixed: GCV-004 merged at `6bb71a8`; citations refreshed; Appendix A rewritten | Appendix A |
| F18 unexplained terms | P3 | fixed: in §2.8 | §2.8 |
| F19 `ProbePoint` signature | P3 | fixed: `Peaks` takes a one-channel buffer | §7.8 |
| F20 step-map cost | P3 | fixed: about 20 r lookups per pixel | §7.8, §10.7 |
| F21 line length stated two ways | P3 | fixed: 2R + 1 samples whose end centres are about 2r apart | §4.7, §10.10 |
| F22 `goja-cv run` cannot lint a library with `args` | P3 | partly: superseded, there is no unused-node lint to run (D10) | D10 |
| F23 name collisions, fragile frame regex | P3 | fixed: `read<N>:` names for shadowed files; names without whitespace or parentheses | §6.1, §7.2, §7.3 |
| F24 `cv.scripts` lacks evidence, enumerates | P3 | partly: kept (the brief asks for a listing) with a walk bounded by entries visited; the enumeration recorded as a risk | §6.2, D6, §17 risk 8 |
| F25 B1 test updates incomplete | P3 | fixed: the full list | §9 |
| F26 byte widths; ambiguous paging note | P3 | fixed: rune widths; the note names the key path | §6.5, §10.3 |
| F27 sheet labels as data | P3 | fixed: the caption lists the cells | §6.7, §7.7, D11, A3 |
| F28 quotations | P3 | fixed: quoted exactly; "failed until" dropped | §3.2, §3.3 |
| F29 squared distances in the fast path | P3 | fixed | §10.11 |
| F30 the repeated `functions:` line is long | P3 | fixed: signatures cut at 40 characters there | §6.4, §10.4 |
| F31 `collectNodes` ignores the item link | P3 | fixed in revision 2; superseded in revision 3, where `collectNodes` goes with `UNUSED_NODE` | D10 |
| F32 let JavaScript call the loaded function | P3 | fixed: `__compile` returns the function; `prelude.js` calls it | §7.2, D24 |
| F33 goldens and `Disk` consumers | P3 | fixed: `overlay-peaks`, `dilate-mask`, `tophat-gray`; B2 tests every `Disk` consumer | §13, §14.3 |

Totals: 31 fixed, 2 partly fixed (F22, F24), 0 declined. Revision 3 changed the fixes of F10 and F22 (deletion of
`UNUSED_NODE`) and of F1 (`OpDef.Halo` deleted with the lint), without changing their disposition.

## Implementation notes (deviations)

Recorded by the implementer, phase by phase; the implementation diary (`reference/06-diary-implementation.md`) has the
details and numbers.

### Phase B1 (commits `bcefb0a`, `5c26122`)

1. **`toMask({ grow })` is deleted in B1.** GCV-004 Phase 5 (which replaces `toMask` with `mask.regions`) has not landed,
   and `toMask`'s `grow` was the other caller of `kernels.Grow`. It goes with no redirect: `regions.toMask().dilate(r)`.
   At the merge with GCV-004 batch 2, take batch 2's `mask.regions` (it has no `grow`).
2. **`DiskMinMax`'s loop order differs from §10.9.** The chord half-width depends only on |dy| and does not increase with
   it, so `chordDisk` filters every row once per distinct half-width and combines rows y ± dy of that width, instead of
   one `minMax1D` per chord and output row. Same result (brute-force tests), less work, any band height.
3. **The binary fast path compares integer squared distances** (`squaredDistanceToSet`, which `DistanceToSet` now wraps)
   with r², instead of float32 square roots with a 1e-6 tolerance (§10.11). Exact at every distance. The threshold
   r ≥ 8 is kept; on 4096² planes the distance transform (about 150 ms) is slower than chords below r ≈ 24 (86 ms at
   r = 16), a loss of at most ~60 ms.
4. **`minMax1D` is rewritten without closures** (padded input, in-place backward pass): about 3× faster (disk 4096²,
   r = 64: 1.22 s → 0.28–0.33 s); the speck background uses it too, and every golden stayed exact.
5. **A disk of r < 1 returns a copy** (§10.9 says r < 0.5): a disk of radius below 1 is the origin alone, so the result
   is the same.
6. **No `Usual` range on `radius`** (§7.9's sketch had [1, 32]): it would raise `RANGE` for radius 0 (the identity) and for
   the long lines the tests accept (500); the disk's hard cap is `Validate`'s.
7. **`crop`'s Doc names erode and dilate instead of peaks** in the edge-effect sentence; B2 adds "peaks report cut rings
   (`ringCoverage`)".
8. **An option given to an op without options** now reads `toMask has no option "grow": it takes none.` (it read
   `Valid options: .`), so the deleted option's error is readable.
9. **The `05-masks-and-regions` section is one paragraph and one block** (the topic cap is 6,000 characters); the
   line-removal example of §6.9 waits for B3's recipes. The MCP instructions only swap `m.grow(r)` for `m.dilate(r)`
   (the 4,000-character cap); "(crop last; …)" in them is left for A4 / GCV-004 Phase 8.
10. **CROP_FIRST and `OpDef.Halo` are a separate commit** (`5c26122`) inside B1, as §9 and §13 place them; the
    coordinator's brief had said they go in their own phases, so the commit can be reverted alone.

### Phase A1 (commit `c0ad935`)

11. **The pending list is GCV-004's, generalized.** GCV-004 Phase 6 already had `__cv_run` wait for saves and an
    `UNHANDLED` warning from goja's rejection tracker (its deviation 43). A1 does not add a second mechanism: the eval
    trace keeps one `pending` list (each load's own promise, each save's settling promise); `cvmod.Pending` and
    `cvmod.Settled` replace `SavesPending` and `SaveFailure`. `__cv_run` waits until no new promise has joined the list
    (a load started by a pending load is waited for too), where §7.4's sketch waits once.
12. **`UNHANDLED` is located in cvmod**, from the rejection reason's stack (`cvmod.StackFrames`: the innermost frame in
    eval code or a loaded file), and reads `UNHANDLED dust/x.js:3: a promise was rejected and never handled: <message>.
    fix: await the call that returned it` (§6.6's text). An unawaited `cv.save` that fails still fails the eval
    (GCV-004 deviation 43).
13. **Path and compile errors are thrown synchronously by `cv.load`** (§7.2's sketch calls `__compile` before any
    promise exists): a missing file, a name without `.js`, `E_ACCESS` or a `SyntaxError` fail the eval at the line of
    the `cv.load` call, awaited or not. Only what the file does at run time goes through the promise.
14. **Syntax errors**: goja reports `name: Line L:C message (and N more errors)` as a `CompilerSyntaxError` without a
    file position; the message is parsed, the "(and N more errors)" dropped, line 1's column corrected for the wrapper,
    and an error on the wrapper's closing line (the file ended inside a bracket, string or comment) reads
    `name:N: … at the end of the file (an unclosed bracket, string or comment?)`.
15. **`called from` needs the eval's frame on the stack**: an error thrown in a file after an `await` (an async
    continuation) has `where: file:line` only.
16. **The JavaScript hints** ("const/let do not survive between evals", "a value is undefined") are not given for errors
    inside a loaded file; they are about the agent's own code.
17. **Prelude checks**: `cvsession.New` refuses a prelude outside the roots ("… add --read DIR"), and the CLI's
    `checkRoots` (eval, run, mcp) turns the same into a usage error. A prelude's name is its script name relative to
    the root it lies in (`lib/dust/helpers.js` when the workspace holds `lib/`). A prelude that fails without a file
    location (a syntax error) reads `prelude syntax.js:1:11: …`, the location taken from the message.
18. **`cv.scripts`** lists regular files with loadable names only (no symlinks, no names with spaces); "four directory
    levels" is the root and three below it (`a/b/c/f.js`).
19. **Help for `load` and `scripts`** are function definitions in `pkg/cv/ops/funcs.go` (like `save`), not `cvFuncs`
    entries, so `cv.help("load")` lists their arguments.
20. **The task-libraries topic** loads `dust/helpers.js` and `dust/thread-width.js` from `pkg/cvsession/testdata/scripts`,
    which the docs tests copy into the planted scan's read directory. The model library uses `measure` and the speck
    view, not `peaks`, until B2/B3 replace it with §12.3's.

### Phase A2 (commit `b10ff2d`)

21. **A table's columns and widths come from its first 200 rows** (§10.3 takes the widths from 200 rows and the
    columns from all of them). Every row is checked to be a plain object, but only the rows read for the header and
    the rows shown are read through their descriptors: a later row with a key the header lacks loses that cell, and a
    shown row past the first 200 that is not flat ends the table with the paging note. A 100,000-row array renders in
    about 0.3 s.
22. **Functions in a result are named** by the last key of their path (`return { f: mem.f }` → `f(a, b = 3): …`), by
    the `mem` key that holds them when returned bare (`return mem.probe` → `mem.probe(x, y): …`), else by their own
    name, else `function`.
23. **`new functions:` is a literal block**, one `signature: doc` line per function, not a mapping (keys such as
    `mem.spots({ snr = 1 } = {})` would be quoted). A getter on `mem` is listed as `name (getter)` under `mem:` and
    never run. The remembered state is replaced by the current functions at every agent eval, so a function deleted
    and defined again is new again.
24. **A2 deletes nothing from §9 items 2–3**: `UNUSED_NODE` goes in A4 (the coordinator's batch plan), and the cut
    notes already named `cv.save` (GCV-004 Phase 6).
25. **The DUST-002 helper fixture** (`pkg/gojamod/cv/testdata/helpers-fixture.js`) reproduces the helper file's
    shapes (a doc comment on `mem.tile`, mid-body comments in `mem.best` and `mem.probe`, a `/*` block, an expression
    body, method shorthand, accessors, a bound function) instead of copying the file: the dust-scanning repository was
    off limits for this batch. goja does not parse async generator methods, so the fixture's generator is plain.

### Phase A3 (commit `c9987c0`)

26. **The caption list is an op hook**, `OpDef.Short` (the compact call `graph.Short` uses for chains and captions),
    fed by `graph.ItemTexts` (the items node's entries: label, else id, else position from 1, as the cells are
    labelled). The count is the array's length; with `max` the list stops at `max` cells. A label with whitespace is
    JSON-quoted, as in tables.
27. **`labels: "index"` is `E_ARG_TYPE`** (the binder's enum error: `cv.sheet.labels = "index" is not one of "id",
    "none"`, with `valid: [id, none]`), not `E_UNKNOWN_OPTION` as §11 says.
28. **Golden `sheet-2x2` moved**: it passed `labels: "index"` (GCV-004 deviation 35) to keep its labels 0 and 1; with
    the mode gone its literal items show their ids 1 and 2. Only the two label texts changed (99 of 113,472 samples).
    No other golden moved.
