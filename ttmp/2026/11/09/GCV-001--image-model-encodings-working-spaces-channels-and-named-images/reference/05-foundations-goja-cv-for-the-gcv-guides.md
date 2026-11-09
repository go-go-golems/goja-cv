---
Title: Foundations - goja-cv for the GCV guides
Ticket: GCV-001
Status: active
Topics:
    - image-model
    - io
    - color
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/doc/02-concepts.md
      Note: the user-facing concepts tour this chapter extends for implementers
    - Path: pkg/cv/graph/node.go
      Note: Builder.Intern, internNode, hash-consing, Substitute
    - Path: pkg/cv/graph/args.go
      Note: the binder (bind), units, lifting, option synonyms
    - Path: pkg/cv/graph/eval.go
      Note: evaluator, per-scale runs, AnalysisScale, cache signatures
    - Path: pkg/cv/graph/registry.go
      Note: OpDef, ArgSpec, Registry
    - Path: pkg/gojamod/cv/prelude.js
      Note: node proxies, awaiting, link() of awaited data
    - Path: pkg/cvsession/session.go
      Note: eval contract, limits, roots
    - Path: pkg/cvsession/processor.go
      Note: result processing, warnings, context block
    - Path: internal/testimage/planted.go
      Note: the planted scan used by tests and docs
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/02-intern-guide-part-1-what-we-are-building.md
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/scripts/06-pi-cv-agent/run.sh
Summary: "The shared foundations chapter of the GCV-001..003 guides: goja-cv's pieces, kinds and nodes, build time and evaluation, ops and the binder, one pixel and scale vocabulary (with the render-scale rule of GCV-002), masks and regions, the JavaScript surface, the session, looks and result content, diagnostics, and test infrastructure."
LastUpdated: 2026-11-09T20:00:00-05:00
WhatFor: "Reading before any of the three GCV guides; each guide cites sections of this document as Foundations §Fn."
WhenToUse: "When a GCV guide uses a goja-cv term you have not met."
---

# Foundations: goja-cv for the GCV guides

The three guides GCV-001 (image model), GCV-002 (inspection views and looks) and GCV-003 (measurement and
data out) build on the goja-cv code as it stands at commit `175b5d4`. This chapter defines the terms all three
use, once, in dependency order. Each guide cites it as "Foundations §Fn". Read it first; it takes about twenty
minutes. The user-facing tour of the same ideas is `pkg/doc/02-concepts.md` (also `goja-cv help concepts`), and
the agent-facing version is the docs topics in `pkg/cv/docs/topics/` (`01-getting-started.md`,
`03-nodes-and-evaluation.md`, `05-masks-and-regions.md`). The design that preceded the implementation is the
CVJS-001 intern guide, in another repository:
`/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/design-doc/`
(files `02` to `05` for Parts 1–4, `06` for integration surfaces). You do not need it to follow the GCV guides.

## F1. What goja-cv is, and its pieces

goja-cv is an image-analysis engine written in Go that LLM agents drive by writing JavaScript. An agent sends a
piece of JavaScript to one tool, `eval`; goja-cv runs it in a persistent session and sends back a YAML rendering
of the returned value, any images the script asked to see, and a short summary of the session's state. The
JavaScript runs in goja, a JavaScript interpreter written in Go, embedded through the go-go-goja runtime library.

| package | role |
|---|---|
| `pkg/cv/buffer` | planar float32 image buffers, colour transfer functions, file decoding and encoding, the preview pyramid |
| `pkg/cv/kernels` | pure functions on buffers (blur, levels, masks, stats, speck detection, drawing) |
| `pkg/cv/graph` | the op registry, nodes, the binder, the evaluator and its cache, errors, help |
| `pkg/cv/ops` | the registered ops: one `OpDef` per operation, each calling kernels |
| `pkg/cv/look` | looks: images an eval returns, rendered after the eval |
| `pkg/cv/docs` | the agent-facing docs topics, readable from inside eval |
| `pkg/gojamod/cv` | the JavaScript module `cv`: the Go **binding** and the JavaScript **prelude** that builds the fluent API |
| `pkg/cvsession` | the eval **session**: globals, result processing, the MCP server |
| `pkg/doc` | the guidebook pages shown by `goja-cv help` |
| `cmd/goja-cv` | the CLI: `eval`, `run`, `mcp`, `ops`, `help` |

**MCP** (Model Context Protocol) is the protocol through which agents such as Claude Code call tools; `goja-cv mcp`
serves the `eval` and `open` tools over it (`pkg/cvsession/mcp.go`).

## F2. Kinds and nodes

A JavaScript call such as `img.gaussian(2)` does not compute anything. It returns a **node**: an immutable
description of one operation applied to inputs with arguments. Every node has one **kind**, the type of value it
evaluates to:

| kind | evaluates to | examples |
|---|---|---|
| `Image` | a float32 buffer (1 or 3 channels) | `img`, `exposure`, `gaussian`, `overlay` |
| `Mask` | a 1-channel buffer of weights in [0, 1] | `mask.luminance`, `mask.box`, `regions.toMask()` |
| `Regions` | a list of connected areas, each with an id, geometry and measures | `mask.components()`, `detect.specks()` |
| `Value` | plain data: numbers, strings, arrays, objects | `stats()`, `histogram()`, `count()` |
| `Param` | a named value a person can move (a slider); behaves as a Value | `P.slider("ev", …)` |

Nodes form a graph. `img.exposure(0.3).gaussian(2).stats()` builds three nodes over the source node `img`:

```
open(scan.tif) ──▶ exposure(ev 0.3) ──▶ gaussian(r 2) ──▶ stats
```

**Build time** is when a node is created (the JavaScript call); **evaluation time** is when its value is
computed. The guides also say **intern time** for build time, after the function that creates nodes,
`Builder.Intern` (`pkg/cv/graph/node.go:62-111`). Nodes are **hash-consed**: a node's identity is a SHA-256 hash
of its op, its canonical arguments and its inputs' hashes (`hashNode`, `node.go:232-246`), and `internNode`
(`node.go:113-143`) returns the existing node when the hash is known. Writing `img.exposure(0.3)` twice gives the
same node (the same id, `===` in JavaScript). This is why a node must never be modified after it is created: it may
be shared by unrelated parts of a script.

## F3. Ops, the registry and the binder

An **op** is a registered operation, described by an `OpDef` (`pkg/cv/graph/registry.go:99-130`): its name, the
kind of its receiver (`On`), extra node inputs, its arguments (`[]ArgSpec`), the kind it returns, and the Go
function `Run` that computes it from evaluated inputs. The **registry** holds every `OpDef`, keyed by
`"<receiver kind>:<name>"`, so `Value:mul` and `Image:mul` can coexist. Ops whose receiver kind is `KindNone` are
**source ops** (`open`, `param`). The JavaScript method tables, `cv.help`, the generated reference and the
did-you-mean index are all derived from the registry.

An `ArgSpec` (`registry.go:65-87`) declares one argument: name, type (`number`, `string`, `enum:a,b`,
`number[]`, `object`, a node kind such as `Mask`), unit, default, limits, and whether it may be passed
positionally. **Units** tell the system what a number means: `DocPx` (a length or coordinate in doc px, F5),
`DocPx2` (an area), `Fraction`, `Unitless`. A `DocPx` argument can only be read with `Args.Length(name, scale)`,
which converts it to buffer px for the evaluation scale (`args.go:169-176`).

The **binder** (`bind`, `pkg/cv/graph/args.go`) turns the raw arguments of a JavaScript call into **canonical
arguments**: it fills defaults, normalizes numbers to float64, rejects wrong types and unknown option keys (with
synonyms and edit-distance suggestions, `args.go:556-614`), and records RANGE warnings. **Hidden arguments** are
set by Go only (for example the file size and time stamp of an opened file, which make the node change when the
file changes).

**Lifting.** Wherever an op wants a number, the call may pass a Value or Param node instead. The binder
records it as an extra input of the new node (an `InputRef`), and the evaluator substitutes the evaluated number
before `Run`. Property access on a Value node returns a derived Value node: `img.stats().p99` is the node
`field(stats, "p99")`. So `img.levels({ black: s.p01, white: s.p99 })` with `s = img.stats()` needs no `await`.

**Builder lambdas.** `img.within(mask, i => i.exposure(-0.4))` runs the function once at build time with a
**placeholder** node standing for `img` (`Engine.Placeholder`, `pkg/cv/graph/engine.go:122-141`); the result graph
is then re-interned with `img` in place of the placeholder (`Builder.Substitute`, `node.go:185-230`).
**Data lambdas** (`regions.filter(r => …)`) run at evaluation time on the JavaScript thread, once per region.

## F4. Evaluation

Evaluation happens when a script awaits a node (`await node`), returns a look of it, or calls `show(node)`.
Nodes are *thenable*: the prelude gives each node a `then` method, so `await` starts an evaluation and resolves
with the value. An Image node awaits to a summary (`{ kind, w, h, scale, … }`); pixels never enter JavaScript. A
Value node awaits to its data, a Regions node to `{ count, items }`. Awaited data keeps a hidden link to its node
(`link()`, `pkg/gojamod/cv/prelude.js:46-79`), so passing awaited data to an op passes the node.

The **evaluator** (`pkg/cv/graph/eval.go`) computes a node at one scale (F5): it evaluates inputs in parallel,
applies the op's `Identity` short-circuit (neutral arguments return the receiver unchanged, `eval.go:291-293`),
calls `Run` (catching panics), and caches the result under a **signature**: a hash of the node, the scale and the
values of the params the node depends on (`eval.go:185-221`). Moving a slider changes the signatures downstream of
that param only. An op with `AnalysisScale` (`registry.go:117-121`) is always evaluated at that scale whatever the
caller asked; Regions ops use it (default 1) so that region ids and coordinates do not depend on the preview size.

The **runtime owner goroutine** is the one goroutine allowed to touch the goja interpreter, which is not
thread-safe. JavaScript and the binding run on it. Evaluations run on other goroutines and settle their
JavaScript promises by posting a callback back to the owner (`binding.evaluate`, `pkg/gojamod/cv/binding.go:296-326`).

## F5. Pixels, coordinates and scales

The GCV guides use exactly these names.

| term | definition |
|---|---|
| **doc px** | The coordinate unit of an image's frame, in pixels of the file it came from. Coordinates are continuous: pixel i covers [i, i + 1), so a centroid of 1312.5 is the centre of pixel 1312 (`pkg/cv/graph/regions.go:97`). Every coordinate, length, radius and box an agent passes or reads is in doc px. |
| **frame** | The rectangle an image covers in doc px: origin and size. Introduced by GCV-001; today every frame starts at (0, 0) and only its size is stored (`DocW`, `DocH`). |
| **buffer px** | A pixel of a computed buffer. |
| **scale** | Buffer px per doc px. A buffer of a 1024 px tile at scale 0.25 is 256 px wide. |
| **evaluation scale** | The scale at which a node's buffer is computed (`Buffer.Scale`). Kernels receive lengths already converted with it (F3). |
| **preview scale** | min(1, 1600 / long side): the scale of `await` (`engine.go:157-171`). |
| **look scale** | min(1, maxSide / long side) with maxSide ≤ 1568: today's scale of a look (`pkg/cv/look/look.go:71-86`). |
| **render scale** | Introduced by GCV-002: the scale of a look's image, which may exceed 1 (zoom). |
| **image px** | A pixel of a delivered look image: a buffer px of the root at the render scale, plus ruler margins when there are rulers. Captions state "1 image px = S doc px". |

Today every evaluation scale is in (0, 1] (`eval.go:176-178`): a preview reads a pyramid level of halved copies of
the source (`pkg/cv/buffer/pyramid.go`), and nothing is ever computed above the file's resolution.

**The render rule (GCV-002).** GCV-002 lets a look ask for a render scale s above 1 and sorts ops into three
classes for it:

- **Pixel ops** compute values from values (exposure, blur, arithmetic, levels). They are never computed above
  scale 1. Asked for s > 1, the evaluator computes them at scale 1 and upsamples the part that is needed.
- **Geometric ops** only select pixels (crop). They are exact at any scale and pass the request on.
- **Decorations** draw marks (overlay outlines and ids, annotations). They are computed at s, so a 1-image-px line
  stays 1 image px wide under zoom.

At s > 1 every request also carries a **render window**: the doc-px rectangle that will be visible. A crop
narrows it to its box; upsampling and drawing touch only the window, so no buffer is ever larger than the look.

Worked example. A 1024 × 1024 tile, `img.view("speck").crop([300, 100, 64, 64]).look({ zoom: 8, ruler: 8 })`:

1. The render scale is 8 and the root is the crop. The window becomes [300, 100, 64, 64].
2. The crop asks its input (the speck view, a chain of pixel ops) for scale 8 in that window.
3. The view is a pixel op: the evaluator computes it at scale 1 over the whole tile (1024 × 1024, cached for any
   later crop), cuts the 64 × 64 window, and upsamples it with nearest-neighbour interpolation to 512 × 512.
4. The look quantizes to 8 bits and adds 28 px ruler margins on each side: the delivered PNG is 568 × 568 image
   px, and the caption says "512×512 + ruler, 1 image px = 0.125 doc px, doc x 300–364, y 100–164".

## F6. Masks, regions and Value data

A **mask** is a field: every pixel has a weight in [0, 1], suited to soft selections. A **region set** is a list
of discrete things, each with an id, a bounding box, an area, a centroid, an aspect ratio and op-specific
measures (`contrast`, `score`, …), suited to counting, ranking and excluding by id. `mask.components()` turns a
mask into regions; `regions.toMask()` turns regions back into a mask. Region geometry is in doc px; each region
also keeps the indices of its pixels in the raster it was found in (`Region.Pix`, `regions.go:11-22`), so it can
be redrawn at any scale (`RegionsValue.Rasterize`, `regions.go:194-240`). Region ids are stable for the same
image and options because detection runs at `AnalysisScale` 1.

Region sets are filtered declaratively: `where({ field: { gte: 0.5 } })`, `sortBy(field)` (largest first by
default), `top(n)`, `without(ids)`, `only(ids)`. These are ops; the predicate is data, so it is hashed and replayed.

## F7. The JavaScript surface

The **binding** (`pkg/gojamod/cv/binding.go`) is the Go object the prelude calls (`__call`, `__evaluate`,
`__look`, `__help`, …). The **prelude** (`prelude.js`) builds the fluent API from the registry: each node is a
JavaScript Proxy over `{ __id, __kind }` whose properties are the methods of its kind, with namespaces
(`img.mask.box`). Unknown properties throw structured errors with suggestions; comparing a node with `<` throws
`E_LAZY_COMPARE`; assigning to a node throws `E_IMMUTABLE`.

## F8. The session

A **session** (`pkg/cvsession`) is one goja runtime that lives across evals. Its **eval contract**: the agent's
code is the body of an async function (`await` works, `return` gives the result); the returned value comes back
as YAML capped at 9000 characters (`DefaultMaxResultChars`, `session.go:38-39`); then a `warnings:` block (RANGE,
UNUSED_NODE, each with the agent's line number) and a **context block**: a short YAML summary of the session
rebuilt after every eval, listing the open images, the params and the keys of `mem` (`processor.go:474-533`).

Session globals (`init.js`): `cv` (the module), `P` (params), `img` (the open image), `show(node, opts)` (queue a
look), `doc` (a stub that throws `E_NO_DOCUMENT`), and `mem`, a plain object that persists between evals. `const`
and `let` do not survive an eval, so agents keep helpers and values on `mem`.

**Roots.** `goja-cv mcp --root DIR` (repeatable) restricts the files `cv.open` may read (`checkPath`,
`native.go:62-88`); with no roots, any path is readable. `eval` and `run` have no `--root`.

**CLI.** `goja-cv eval -e CODE --image FILE [--out DIR] [--json]` and `goja-cv run SCRIPT.js` create a fresh
session per process, print the YAML (or a JSON object with `--json`), and write looks as files into `--out` (a
temporary directory by default, `cmd/goja-cv/eval.go:88-137`, `:175-224`).

## F9. Looks and result content

A **look** (`pkg/cv/look`) is a Go object that names a node and how to render it; `node.look(opts)` creates one.
go-go-goja's `resultcontent` package defines the interfaces: a value returned from an eval that implements
`Provider` (`ResultContent(ctx) ([]Content, error)`) becomes rich content; `Describer` gives it a cheap text
placeholder. The pipeline after an eval:

1. The session walks the returned value on the owner goroutine (**Extract**) and replaces each provider by its
   placeholder, `"[content N: look 2: after (1024×683, 1 image px = 1.0 doc px)]"`.
2. Off the owner goroutine it calls each provider (**Render**) under limits: at most `MaxLooks` = 4 items and
   8 MiB together (`session.go:40-43`, `:115-119`).
3. The MCP server turns `image/*` items into image content and `text/*` and `application/json` into text
   content (go-go-goja `replmcp/tools.go`, `toMCPContent`); the CLI writes them as files.

## F10. Diagnostics

Every failure is a structured error (`graph.Error`, `errors.go:40-51`): a code (`E_ARG_TYPE`, `E_UNKNOWN_OPTION`,
…), a message in the API's terms, the node path, a hint, a literal **fix** (a corrected call the agent can copy),
and a list of valid alternatives. **Synonyms** map words agents use to canonical names, for ops
(`pkg/cv/graph/suggest.go:10-41`) and options (`args.go:590-605`). `cv.help(name)` never throws; unknown names
return `{ found: false, didYouMean }`. Warnings are produced at build time today: RANGE (an argument outside its
usual range), UNUSED_NODE (a node built and never used), CUT (the result text was truncated).

These rules come from measured agent runs (the CVJS-001 pi runs and the deep-dive article "Teaching a Cheap Model
an Image Eval API" in go-go-parc): agents read documentation they can reach from inside eval; a help function that
throws kills the whole batch; an error with a literal fix is applied on the next call; reasonable words such as
`min` or `threshold` should map to the API's names; and an unexplained optional parameter gets invented values.

## F11. Test infrastructure

- **The planted scan** (`internal/testimage/planted.go`): a 256 × 192 px, 16-bit sRGB TIFF with a smooth warm
  gradient background, 40 planted specks (dark and bright, some elongated) with known contrast, and one soft
  bright star at (200.5, 46.5). Tests and every docs code block run against it.
- **Docs tests** (`pkg/cvsession/docs_test.go:77-152`): every ```js block in `pkg/cv/docs/topics` and `pkg/doc`
  runs in a session with the planted scan as `img`; blocks marked `js warns=CODE` or `js error=CODE` must warn or
  fail with that code.
- **Real tiles**: `~/dustdet/cv_tiles/{tile_id}.tif` (50 tiles, 1024 × 1024, 16-bit RGB, uncompressed, storing
  sRGB_encode(density)) with `{tile_id}.json` (`origin_x`, `origin_y`, `film_type`). Human labels:
  `~/dustds/gold/{tile_id}.final.json` or `.p1.json`, `points: [{ x, y, r, type }]` in absolute raw-scan
  coordinates, `type` ∈ dust, hair, scratch (the loader is `dustdet/gold.py`, `annotation()` and
  `POSITIVE_TYPES`). Tests that use them are gated on environment variables and skip when the files are absent.
- **The pi harness**: `CVJS-001/scripts/06-pi-cv-agent/run.sh GOJA_CV_DIR OUT_DIR [MODEL]` (absolute directory in
  the header of this file) builds goja-cv, writes the planted scan, starts the `pi` coding agent with goja-cv as an
  MCP server, and asks the questions in `questions.md`; `07-pi-events-summary.py` counts calls and errors. Results
  of six runs are in `various/pi-runs/README.md`. The GCV guides add an `IMAGE` and a `QUESTIONS` override and one
  question file per ticket; these edits are in the dust-scanning repository.

## F12. Where each GCV guide builds on this

| guide | builds on |
|---|---|
| GCV-001 | F2–F5 (nodes, binder, evaluation, frames), F8 (names in the session), F11 |
| GCV-002 | F5 (render rule and windows), F9 (looks), F3 (registry-driven functions), F6 (overlays) |
| GCV-003 | F6 (regions and items), F3 (binder, lifting), F9 (result content), F8 (CLI, roots) |
