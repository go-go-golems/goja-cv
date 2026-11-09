---
Title: Design and implementation guide
Ticket: GCV-004
Status: active
Topics:
    - api
    - architecture
    - refactor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/05-foundations-goja-cv-for-the-gcv-guides.md
      Note: shared vocabulary (Foundations §Fn); this guide's architecture tour updates it for v2
    - Path: ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/reference/01-evidence-pi-run-api-usage.md
      Note: usage numbers from 751 pi calls that motivate Part A
    - Path: ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/scripts/02-pi-names-typed.py
      Note: wrong names agents typed; which synonym and redirect keys survive
    - Path: pkg/cv/graph/args.go
      Note: binder, lifting, InputRef, silent accessors, OptionSynonyms; reduced to literal options and loud accessors
    - Path: pkg/cv/graph/registry.go
      Note: Kind, SpaceRule, ArgSpec.Static, OpDef.Produces/MaxItems/Inputs; KindMask and KindParam removed
    - Path: pkg/cv/graph/node.go
      Note: Builder.call and internNode; node statics gain Channels; trusted map replaced by typed source metadata
    - Path: pkg/cv/graph/space.go
      Note: resolveSpace and spaceWarnings; becomes the three-rule space resolution with physical brackets
    - Path: pkg/cv/graph/params.go
      Note: the param store; deleted
    - Path: pkg/cv/graph/eval.go
      Note: evaluator; params leave the signature, stamp turns NaN into null, recover on every goroutine
    - Path: pkg/cv/graph/render.go
      Note: Render; one exit enforces ScaledSize(window, scale); fitScale deleted
    - Path: pkg/cv/graph/schema.go
      Note: RemovedFields, Produces hints, Record; Record gains MarshalJSON
    - Path: pkg/cv/graph/paths.go
      Note: Within, resolveDepth, OutPath, WriteOut; replaced by an os.Root workspace
    - Path: pkg/cv/ops/regions.go
      Note: where, sortBy, top, only, without, count, filter; deleted
    - Path: pkg/cv/ops/present.go
      Note: overlay (three receivers), compare, listInputs; one overlay, node lists through Args
    - Path: pkg/cv/ops/sheet.go
      Note: nodeOf, viewNodes, itemsBound; node lists through Args, items from arrays
    - Path: pkg/cv/ops/views.go
      Note: speck view uses value lifting and presetSpace; internal levelsAt op and an as("linear") step
    - Path: pkg/cv/ops/source.go
      Note: cv.open with ten hidden arguments and the param op
    - Path: pkg/cv/dataout/data.go
      Note: Data handles, hand-written JSON, CSV shape guessing; reduced to records-only encoders behind cv.save
    - Path: pkg/gojamod/cv/prelude.js
      Note: node proxies, link(), derived fields, P, cv.evaluate, cv.json/csv; v2 surface
    - Path: pkg/gojamod/cv/binding.go
      Note: native functions exposed to JS; panic boundary, items marshalling, params and data lambdas removed
    - Path: pkg/gojamod/cv/fields.go
      Note: field names for derived-field lifting and removed-field accessors; deleted
    - Path: pkg/cvsession/native.go
      Note: checkPath with graph.Within, show queue, doc stub; workspace and look queue
    - Path: pkg/cvsession/init.js
      Note: session globals (cv, P, mem, show, img, doc); P, show and doc removed
    - Path: cmd/goja-cv/eval.go
      Note: CLI flags, writeContent, uniqueName; --root and --read-only, uniqueName deleted
    - Path: cmd/goja-cv/mcp.go
      Note: MCP flags (--root repeatable, --out-root); one --root
    - Path: Makefile
      Note: smoke target hard-codes look-1-levels.png; reads the files list from --json
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/various/pi-runs
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/scripts/06-pi-cv-agent/run.sh
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/scripts/84-cv-helpers.js
Summary: "Intern guide for GCV-004: an architecture tour of goja-cv, then API v2 (literal options, space on the image, masks as one-channel images, regions as arrays, await only, look only, one cv.save, one os.Root workspace, pruned synonyms) and the foundations that make the review-found bug classes unwritable (loud argument accessors, node lists through Args, one Render exit, encoding/json, a panic boundary, typed source metadata), with phases, tests and a pi validation plan."
LastUpdated: 2026-11-09T18:00:00-05:00
WhatFor: "Understanding goja-cv end to end and implementing GCV-004 phase by phase."
WhenToUse: "Read before touching the binder, the space rules, masks, regions, data out, path handling or the JavaScript surface; read section 2 first if you are new to goja-cv."
---

# GCV-004 — API v2: one way per thing, and foundations

## Revision history

| date | revision | what changed |
|---|---|---|
| 2026-11-09 | 1 | First version, written from the code on `task/gcv-004-api-v2` (stacked on GCV-001..003, including the review fixes `4c79173`, `cd5f46c`, `319a6fc`) and the evidence in `reference/01-…` and `scripts/02-pi-names-typed.py`. |

## How to read this guide

This guide is for an intern who will implement GCV-004 in goja-cv (`/Users/manuel/code/wesen/go-go-golems/goja-cv`).
Unlike the three earlier guides, it does not assume that you have read the shared foundations chapter
(`ttmp/2026/11/09/GCV-001--…/reference/05-foundations-goja-cv-for-the-gcv-guides.md`): section 2 is a tour of the
whole system as it stands today, from the pixel buffer to the MCP server, and it ends with one JavaScript call
traced through every layer. If you have never opened the repository, read section 2 with the code beside you; it
takes about an hour. If you know goja-cv, skim it and start at section 3.

The guide then states the problem (section 3), builds the vocabulary of the new design (section 4), shows where
today's code does each thing that changes (section 5), presents the v2 JavaScript API with before-and-after code
(section 6), the architecture that implements it (section 7), the decisions and their alternatives (section 8),
what is deleted (section 9), the key flows in pseudocode (section 10), errors (section 11), documentation (section
12), and the work in phases (sections 13–15). Sections 16 and 17 cover the colleague's helpers and the risks.

Citations have the form `path:line` or `path:first-last` and refer to the branch as checked out for this guide. When
a line number has drifted, search for the named function; the names are stable.

GCV-004 is alpha internal software work with no backward compatibility. When this guide says "delete", it means
delete: no alias, wrapper, deprecation notice or migration error survives. The one place where old names are still
recognized is the short list of wrong names that agents demonstrably typed (section 7.13), and there they get a
literal fix, not a second spelling.

## 1. Executive summary

goja-cv is an image-analysis engine that language-model agents drive by writing JavaScript. Three tickets
(GCV-001..003) gave it an image model with encodings and working spaces, inspection views and zoomed looks, and
measurements with data export. A code review of those three pull requests found eight bugs. Each was fixed, and
each fix added a little code. Looking at them together shows that none came from a missing feature: every one came
from a mechanism that let two representations of the same thing drift apart, or from home-made infrastructure that a
standard library already does correctly. At the same time, 751 recorded agent calls show which parts of the
JavaScript API agents use, which they override every time, and which they trip over.

GCV-004 uses both findings to make goja-cv smaller. Part A reshapes the JavaScript API so that there is one way to
do each thing:

1. **Options are literal values.** Params (`P.slider` …) and value lifting (a lazy measurement used as an option)
   go. A measurement that feeds an option is awaited first: `levels({ white: (await d.stats()).p99 })`.
2. **Space is a property of the image.** `cv.open` returns the file's values (file units); `img.as("linear")`
   converts. The per-op `{ space }` option goes. Measurements report in the space of the image they measured. A few
   physical ops (exposure, heal, inpaint, Rec. 709 grey, Lab) convert internally and return in the caller's space.
3. **A mask is a one-channel image.** The Mask kind goes; every image op works on masks; mask ops accept any
   one-channel image. Nodes gain a static channel count so this is still checked at build time.
4. **Regions are arrays when awaited.** An awaited region set is a plain JavaScript array of flat items; agents
   filter, sort and slice it with JavaScript, and pass the array back wherever regions are accepted. The query ops
   (`where`, `sortBy`, `top`, `only`, `without`, `count`, `filter`) and their machinery go. A Regions node has no
   methods: regions are arguments of image methods.
5. **One way per thing.** `await` is the only way to evaluate (at full resolution); `look()` is the only way to show;
   each op has one spelling; `cv.save(path, x)` is the only way to write a file, with the format from the extension.
6. **One workspace.** `cv.open` and `cv.save` resolve paths in one directory, opened as a Go `os.Root`, so a saved
   file can always be reopened and no path can leave the workspace.
7. **Names agents typed, and no others.** Of 246 synonym and redirect keys, 14 were ever typed by an agent. The rest go.

Part B changes the foundations so that the review's bug classes cannot be written: argument accessors panic on a
wrong type instead of returning zero; node-list arguments are read only through `Args`, in list order, with no
deduplication; `Evaluator.Render` enforces its size postcondition at one exit; data is encoded with `encoding/json`
through an ordered `Record.MarshalJSON`; every Go function reachable from JavaScript and every goroutine goja-cv
starts recovers panics as `E_INTERNAL`; `cv.open`'s ten hidden arguments become one typed metadata value; the smoke
test reads the files the CLI reports.

The net effect is a deletion of roughly 2,900 lines of non-test code against about 800 added (section 9), and an
API whose JavaScript reference fits in one table (section 6.9).

## 2. An architecture tour of goja-cv

This section explains goja-cv as it is today, bottom to top, then traces one call through all of it. Every later
section refers back here. The design does not change in this section; where GCV-004 will change a piece, the
paragraph says so in one sentence and points to the section that explains it.

### 2.1 The layers

goja-cv is one Go module with a strict dependency order. Lower layers never import higher ones.

```
                ┌──────────────────────────────────────────────────────────────┐
 front ends     │ cmd/goja-cv (CLI: eval, run, mcp, ops, help)                 │
                ├──────────────────────────────────────────────────────────────┤
 session        │ pkg/cvsession: replapi app, init.js globals, result          │
                │ processor (YAML, warnings, context block), MCP server        │
                ├──────────────────────────────────────────────────────────────┤
 JS module      │ pkg/gojamod/cv: binding.go (native functions) + prelude.js   │
                │ (node proxies, the fluent API), help, docs access            │
                ├──────────────────────────────────────────────────────────────┤
 output         │ pkg/cv/look (looks: images returned from an eval)            │
                │ pkg/cv/dataout (files and data written for programs)         │
                ├──────────────────────────────────────────────────────────────┤
 graph          │ pkg/cv/ops (the registered ops)                              │
                │ pkg/cv/graph (registry, binder, builder, nodes, evaluator,   │
                │               cache, render, errors, help, suggestions)      │
                ├──────────────────────────────────────────────────────────────┤
 pixels         │ pkg/cv/kernels (pure functions on buffers)                   │
                │ pkg/cv/buffer (float32 buffers, colour, decoders, writers)   │
                └──────────────────────────────────────────────────────────────┘
```

The agent-facing documentation lives in `pkg/cv/docs/topics` (readable from inside an eval with `cv.docs`), the
person-facing guidebook in `pkg/doc` (shown by `goja-cv help`), and test fixtures in `internal/testimage`.

### 2.2 Buffers and kernels

Every image goja-cv computes is a `buffer.Buffer` (`pkg/cv/buffer/buffer.go:10-14`): a width, a height, a channel
count `C` (1 or 3), a planar `[]float32` (channel c occupies `Pix[c·W·H : (c+1)·W·H]`) and a `Scale`, the number of
buffer pixels per pixel of the original file. Values are nominally in [0, 1] but may lie outside it (a signed
difference is negative where the second image is brighter).

A file stores **code values**: integers or floats as written. An **encoding** (`buffer.Encoding`,
`pkg/cv/buffer/encoding.go:13`) is the transfer function between the stored values and values proportional to the
measured quantity. goja-cv knows two: `srgb` (photographs: stored = sRGB_encode(light)) and `linear` (stored =
quantity). A buffer's values are in one of two **spaces** (`buffer.Space`, `encoding.go:48`): `encoded` (the stored
values, what a look shows) or `linear` (the encoding undone). For a linear-encoded image the two coincide;
`buffer.Identity(e, from, to)` (`encoding.go:85`) reports when a conversion is the identity, and `buffer.Convert`
(`encoding.go:93`) converts. Decoders in `pkg/cv/buffer` read TIFF, PNG, JPEG, NPY and linear DNG and report the
encoding the file declares.

**Kernels** (`pkg/cv/kernels`) are pure functions from buffers to buffers or numbers: blur, levels, masks, statistics,
speck detection, drawing. They never convert spaces and know nothing about nodes. This layer does not change in
GCV-004.

### 2.3 The registry: ops and functions

An **op** is a registered operation described by an `OpDef` (`pkg/cv/graph/registry.go:254-310`). The fields that
matter for this guide:

| field | meaning |
|---|---|
| `Name` | `"exposure"`, `"mask.luminance"` (a dot puts the op in a namespace) |
| `On` | the receiver **kind**: `Image`, `Mask`, `Regions`, `Value`, `Param`, or none for source ops (`registry.go:18-26`) |
| `Inputs` | extra node inputs passed positionally before the arguments (`overlay` takes `"Regions\|Mask"`) |
| `Args []ArgSpec` | the arguments (`registry.go:173-199`): name, type, unit, default, limits, positional or not |
| `Returns` | the kind of the result |
| `Space SpaceRule` | the working space the op computes in (`registry.go:70-79`) |
| `Result *ResultEnc` | how the encoding and space of an Image result are derived (`registry.go:95-143`) |
| `DocFrame`, `Fields`, `Validate` | build-time functions: the result's frame, its schema, argument checks |
| `Expand` | makes the op a **macro**: a small graph of other ops built when the call is made (`pkg/cv/graph/macro.go`) |
| `Run` | the Go function that computes the result from evaluated inputs |

The **registry** (`registry.go:325-329`) maps `"<receiver kind>:<name>"` to its `OpDef`, so `Image:mul` and `Value:mul`
coexist. `Register` (`registry.go:337-383`) validates a definition and, for every op with a non-fixed working space,
appends the `space` option (`registry.go:370-375`). Functions that do something immediately instead of building a
node (`look`, `save`, `cv.json`, `show`) are `FuncDef`s (`pkg/cv/graph/funcs.go:15-36`) in the same registry, so help,
the generated reference and did-you-mean see one list. The ops themselves are defined in `pkg/cv/ops`, one file per
family (`tone.go`, `mask.go`, `regions.go`, `measurement.go`, `views.go`, …), and registered by
`cvmod.DefaultRegistry` (`pkg/gojamod/cv/module.go:52`). `goja-cv ops` prints the whole table.

### 2.4 The builder: binding and interning

A JavaScript call such as `img.gaussian(2)` computes nothing. It returns a **node**: an immutable description of one
op applied to inputs with arguments. **Build time** is when the node is created; **evaluation** is when its value is
computed.

`Builder.call` (`pkg/cv/graph/node.go:102-169`) creates nodes. It checks that the receiver's kind fits the op
(`node.go:109-111`), substitutes builder-lambda results (`node.go:115`, section 2.9), and calls the **binder**,
`bind` (`pkg/cv/graph/args.go:938-1054`). The binder turns the raw JavaScript arguments into **canonical
arguments** (`Args`, `args.go:42-45`): explicit node inputs first, then positional values in declaration order,
then one trailing options object whose keys may name any argument. It fills defaults, converts numbers to
`float64`, checks types, enum values and hard limits, and refuses unknown option keys with a did-you-mean hint.
Wherever an argument is a node (an Image passed as the `b` of `img.sub(b)`, or a lazy measurement passed as a
number), the binder **lifts** it (`binder.lift`, `args.go:356-377`): the node becomes an extra input of the new node
and the canonical argument records an `InputRef{Index}` pointing at that input.

After binding, `Builder.call` runs the op's `Validate`, checks crops and frames, resolves the working space
(`Builder.resolveSpace`, `pkg/cv/graph/space.go:71-112`, which inserts `convert` and `toGray` nodes where inputs are
in another space), and finally **interns** the node (`Builder.internNode`, `node.go:173-191`): it hashes the op key,
the canonical arguments and the input hashes (`hashNode`, `node.go:377-391`) and returns the existing node when the
hash is known. Calling `img.gaussian(2)` twice returns the same node. This **hash-consing** is why nodes must never be
modified: unrelated parts of a script may share them.

### 2.5 The graph: nodes and their statics

A `Node` (`node.go:17-29`) holds its op, its input ids (`Inputs`: the receiver, explicit inputs, then lifted
arguments; the first `NExplicit` are passed to `Run`), its canonical `Args`, its kind and hash, and four **statics**:
values computed at build time from the op, the arguments and the inputs' statics alone.

| static | meaning | computed by |
|---|---|---|
| `Frame` | the rectangle the result covers, in **doc px** (pixels of the original file; coordinates are absolute) | `nodeFrame`, `node.go:275-288` |
| `Enc` | the encoding of an Image result | `nodeStatics`, `space.go:215-239` |
| `Space` | the space of an Image result's values | `nodeStatics` |
| `Fields` | the schema of a Regions or Value result: the fields of its items or rows | `nodeFields`, `pkg/cv/graph/schema.go:158-168` |

Because statics depend only on what is hashed, a hash hit returns a node with exactly the statics a new one would
get. Statics are what make build-time errors possible: "this box lies outside the image's frame", "these regions
have no field `snr`". GCV-004 adds one static, the channel count (section 7.5).

### 2.6 The evaluator and the cache

`Evaluator.Evaluate` (`pkg/cv/graph/eval.go:237-250`) computes a node at a **scale** in (0, 1]: the number of buffer
pixels per doc pixel. Scale 1 is full resolution; a **preview** is computed at min(1, 1600 / long side)
(`pkg/cv/graph/engine.go:201`), reading a pyramid of halved copies of the source. Lengths in arguments are in doc px
and converted to buffer px for the scale (`Args.Length`, `args.go:202-207`), so a script means the same thing at
every scale.

For each node the evaluator computes a **signature** (`evalRun.sig`, `eval.go:275-306`): a hash of the node's hash,
the scale and the signatures of its inputs (and, for a `param` node, the param's current value, `eval.go:293-296`).
The signature keys the byte-capped value cache (`pkg/cv/graph/cache.go`). On a miss, `compute` (`eval.go:356-409`)
evaluates the inputs in parallel, substitutes lifted values into the arguments (`resolveArgs`,
`args.go:1138-1156`), applies the op's `Identity` short-circuit, calls `Run` inside `safeRun` (`eval.go:449-456`,
which turns a panic in `Run` into `E_INTERNAL`), and **stamps** the result with the node's statics (`stamp`,
`eval.go:416-447`): `Run` only creates values; the evaluator gives them their frame and encoding. Ops with an
`AnalysisScale` (region detectors) are always evaluated at that scale, so region ids and coordinates do not depend on
the preview size (`eval.go:225-234`).

### 2.7 Rendering and looks

A **look** (`pkg/cv/look/look.go`) is a Go object that names a node and how to show it: label, zoom, rulers, grid,
format. `node.look(opts)` creates one (`look.New`, `look.go:137`); it renders after the eval, off the JavaScript
thread, through `Evaluator.Render` (`pkg/cv/graph/render.go:44-87`). Render handles **render scales** above 1 (zoom)
by sorting ops into three classes (`registry.go:209-225`): **pixel ops** are never computed above scale 1 (they are
computed at 1 and the visible window is upsampled); **geometric ops** (crop) pass the request through; **decorations**
(overlay, annotate) draw at the render scale so lines stay one image pixel wide. A **render window** bounds every
buffer by the look's size. The look then converts to display values (`Engine.Display`, `engine.go:171-184`),
quantizes to 8 bits, adds rulers and a caption, and encodes PNG or JPEG.

goja-cv returns a look as **result content**: go-go-goja's `resultcontent` package defines a `Provider` interface
(`ResultContent(ctx) ([]Content, error)`); the session replaces each provider in the returned value by a text
placeholder and renders it afterwards (`look.go:309-323`). The MCP server sends the bytes as image content; the CLI
writes them as files. This layer does not change in GCV-004 except for how looks are queued (section 7.7).

### 2.8 Data out

`pkg/cv/dataout` writes data for programs. `SaveImage` (`pkg/cv/dataout/save.go:84-152`) evaluates an Image node at
scale 1 and writes TIFF, PNG, NPY or JPEG with its encoding and frame origin recorded where the format allows. A
`Data` handle (`pkg/cv/dataout/data.go:40-59`), made by `cv.json(x)` or `cv.csv(x)`, encodes a Value or Regions node
or plain data as JSON or CSV, and is either delivered as result content or saved. Both write only under an
**out-root** (`graph.OutPath`, `graph.WriteOut`, `pkg/cv/graph/paths.go:59-102`). GCV-004 replaces all of this
surface with one function, `cv.save` (sections 6.7 and 7.8).

### 2.9 The JavaScript module: binding and prelude

goja-cv runs JavaScript in **goja**, a JavaScript interpreter written in Go, through the **go-go-goja** runtime
library. goja is not thread-safe: one goroutine, the **runtime owner**, may touch the VM. The module `cv` has two
halves:

- The **binding** (`pkg/gojamod/cv/binding.go`) is a Go object whose methods JavaScript calls: `__call(op,
  receiverId, args)` builds a node (`binding.go:273-300`), `__evaluate(id, scale)` returns a promise
  (`binding.go:357-393`), `__open`, `__look`, `__save`, `__data`, `__help`, … (registered in `binding.object`,
  `binding.go:104-137`). Native functions run on the owner goroutine. Evaluation runs on another goroutine, which
  posts the promise's settlement back to the owner.
- The **prelude** (`pkg/gojamod/cv/prelude.js`) builds the fluent API from the registry. Each node is a JavaScript
  `Proxy` over `{ __id, __kind }` (`nodeFor`, `prelude.js:295-302`) whose properties are the methods of its kind
  (`lookup`, `prelude.js:210-267`). `then` makes nodes awaitable (`prelude.js:221-227`). Unknown properties throw
  structured errors; on a Value node they build a derived-field node instead (`prelude.js:265`). Awaited data keeps a
  hidden link to its node (`link`, `prelude.js:72-100`), so passing awaited data to an op passes the node
  (`marshal`, `prelude.js:121-133`).

A **builder lambda** (`img.within(mask, i => i.exposure(-0.4))`) runs once at build time on a placeholder node; the
builder substitutes the receiver for the placeholder (`Builder.Substitute`, `node.go:334-375`). A **data lambda**
(`regions.filter(r => …)`) runs at evaluation time on the owner goroutine through `binding.CallEach`
(`binding.go:62-102`). GCV-004 keeps builder lambdas and deletes data lambdas (section 7.6).

### 2.10 The session, the MCP server and the CLI

A **session** (`pkg/cvsession`) is one goja runtime that lives across evals. `App.Evaluate`
(`pkg/cvsession/session.go:311-319`) wraps the agent's code as the body of an async function (`Wrap`,
`session.go:296-302`), so `await` and `return` work at top level and agent line N is line N + 1 of the source. The
session globals come from `init.js`: `cv`, `P` (params), `mem` (a plain object that survives between evals), `show`,
`img` (the opened image) and `doc` (a stub that throws) (`pkg/cvsession/init.js:10-110`). After an eval, the
**result processor** (`pkg/cvsession/processor.go:64-93`) renders the returned value as YAML (capped at 9000
characters), collects looks and data items under budgets (`collect`, `processor.go:124-169`), adds warnings with the
agent's line numbers, and appends a **context block** listing open images, params and the keys of `mem`
(`processor.go:503-545`).

`goja-cv mcp` (`cmd/goja-cv/mcp.go`) serves the session over MCP (Model Context Protocol, the protocol through which
agents such as Claude Code call tools): an `eval` tool, an `open` tool, the `cv://reference` resource and the server
instructions (`pkg/cvsession/reference.go:15-48`). `goja-cv eval -e CODE` and `goja-cv run FILE` run one eval in a
fresh session and print YAML or, with `--json`, a JSON object that lists the files written for looks and data
(`cmd/goja-cv/eval.go:97-157`). Reads are restricted by `--root` (MCP only) and writes by `--out-root`
(`pkg/cvsession/session.go:61-66`).

### 2.11 One call, end to end

Trace this eval through the layers:

```js
const s = await img.gaussian(2).stats()
return { p99: s.p99, look: img.gaussian(2).look({ zoom: 4 }) }
```

1. **Session.** `App.Evaluate` wraps the code and hands it to replapi, which runs it on the owner goroutine
   (`session.go:311-319`).
2. **Prelude.** `img` is a node proxy (the global is backed by the session's name table). `img.gaussian` resolves in
   the Image method table and returns a function (`prelude.js:260-262`); calling it calls `native.__call("gaussian",
   imgId, [2])` (`callOp`, `prelude.js:140-143`).
3. **Binding.** `binding.call` converts the arguments (`fromJS`, `pkg/gojamod/cv/convert.go:14-43`) and calls
   `Engine.Call` (`engine.go:71-89`), which looks up `Image:gaussian` and calls `Builder.Call`.
4. **Builder.** `bind` maps `2` onto `sigma` (positional) and fills `space: "linear"` (the op's default, added by
   `Register`). `resolveSpace` stores the space in the arguments; the source is already linear, so no conversion is
   inserted. `internNode` hashes and stores the node; its statics are the source's frame, encoding `srgb`, space
   `linear`. The handle `{ id, kind }` goes back to the prelude, which wraps it in a proxy.
5. **Second call.** `.stats()` does the same for `Image:stats` with the gaussian node as receiver.
6. **Await.** `await` calls the proxy's `then`, which calls `native.__evaluate(id, null)`; the binding picks the
   preview scale, starts a goroutine, and returns a promise (`binding.go:357-393`).
7. **Evaluator.** On the goroutine, `Evaluate` computes signatures, misses the cache, evaluates the source (decoding
   the file once into the source store, `pkg/cv/graph/sources.go:54-92`), runs the gaussian kernel, runs `stats`, and
   caches each result.
8. **Settle.** The goroutine posts back to the owner; `resultToJS` (`binding.go:395-415`) converts the stats record
   to a JavaScript object in schema order and the promise resolves. `link` attaches the node's methods to the object.
9. **Look.** `img.gaussian(2)` is the same node as before (hash-consing). `.look({ zoom: 4 })` binds the options
   through the same binder (`graph.BindOptions`, `funcs.go:146-158`) and returns a Go look object.
10. **Result.** The processor finds the look in the returned object, replaces it with its placeholder, renders the
    YAML, then renders the look: `Evaluator.Render` at scale 4 computes the gaussian at scale 1 (a pixel op) and
    upsamples it. The MCP server sends the YAML and a PNG.

Keep this trace in mind. GCV-004 changes steps 4 (no space option; space from the receiver), 6 (scale 1), 8 (no
methods attached to data) and 9 (the look is queued when created).

## 3. Problem statement

### 3.1 The review findings as bug classes

Codex reviewed PRs #2–#4 (GCV-001..003) and left nine comments; eight were real and were fixed in `4c79173`,
`cd5f46c` and `319a6fc` (coordination diary, `GCV-001…/reference/01-diary-coordination-gcv-001-003.md`, step 9).
Grouped by cause, they form six classes. The right-hand column is the structural change that makes the class
impossible to write, which Part B of this ticket implements.

| finding (PR) | what happened | class | structural fix |
|---|---|---|---|
| `cv.open({ encoding: P.choice(…) })` silently opened as srgb (#2) | the binder lifted the param into an `InputRef`; `sourceEnc` read the argument with `Args.String`, which returned `""` for a non-string (`args.go:222-226`), and fell back to srgb (`pkg/cv/ops/source.go:15-24`) | opt-in `Static` flag + accessors that return a zero value | options are literals; accessors panic on a wrong type (§7.2) |
| `cv.compare([img, img])` clipped the second panel (#3) | the binder lifts a repeated node once (`liftIdx`, `args.go:371-376`); `DocFrame` laid out from the deduplicated inputs | two representations of a node list | node lists read only through `Args`, no deduplication (§7.3) |
| a sheet looked at below scale 1 kept its scale-1 size (#3) | the size postcondition was enforced on one branch of `Render` only; the fix `fitScale` added a `+1` tolerance (`render.go:255-268`) | a postcondition checked on one path | one exit that enforces it (§7.10) |
| a dangling symlink in the out-root let a save escape it (P1, #4) | home-made containment (`Within`, `resolveExisting`, `resolveDepth`, `paths.go:17-52`) treated the link as a missing file | home-made path containment | `os.Root` (§7.8) |
| `cv.json` wrote Go escapes such as `\x00` (#4) | a hand-written JSON writer (`data.go:253-332`) used `strconv.Quote` | home-made JSON | `encoding/json` with an ordered `Record.MarshalJSON` (§7.9) |
| a 2-sample profile's CSV had the wrong shape (#4) | `table()` guesses table shapes; `columnArrays` chose columns by array length (`data.go:334-445`) | shape guessing | CSV only for a list of rows (§7.9) |
| two CLI data items named `a` overwrote each other (#4) | data names and file names were two namespaces; `uniqueName` (`cmd/goja-cv/eval.go:255-265`) reserved suffixes | two naming rules | one path per save, unique per eval (§7.8) |
| an empty TIFF `BitsPerSample` panicked outside the recovery boundary (#2) | `ProbeFile` runs on the owner goroutine at `cv.open`, outside `safeRun` (`eval.go:449-456`) | a recovery boundary at one place only | recover at every Go function reachable from JavaScript and on every goroutine (§7.11) |

None of the eight came from backward compatibility. Each fix added code for the instance (a depth bound, a
tolerance, a suffix scheme, a schema-less fallback); the structural fixes delete that code again (§9).

### 3.2 The usage evidence

The second input is what agents actually did. `reference/01-evidence-pi-run-api-usage.md` counts 751 eval calls in
18 pi runs (model `lunaroute/deepseek-4.1-flash`); `scripts/01-pi-api-usage.py` reproduces it, and this ticket's
`scripts/02-pi-names-typed.py` adds per-method call counts and the wrong names agents typed. The numbers that drive
each change:

| observation | count | consequence |
|---|---|---|
| Params (`P.slider`, …) in agent code | 0 | delete params |
| value lifting (an unawaited measurement as an option) | 4, all in one run | delete lifting |
| `{ space }` options | 198, of which `stats({ space: "encoded" })` 80; every explicit `gaussian` space (10 of 10) was `encoded` | space moves to the image; measurements in file units by default |
| `await` / `cv.evaluate` | 1139 / 85; 77 of 79 `cv.evaluate` calls in code pass `{ scale: "full" }` | `await` only, at full resolution |
| `.look(` / `show(` | 167 / 87 | one display call |
| JS `.filter(` / `.where(`, `.sortBy(`, `.top(` | 74 / 43, 20, 22 | regions become arrays |
| `cv.json`/`cv.csv` / `.save(` | 10 / 11 | one output function |
| `E_UNKNOWN_METHOD` | 35 (`mask.stats`, `Mask.coverage`, `Regions.filter`, guessed save names) | masks are images; one save |
| `E_ACCESS` | 23, mostly reopening a file the agent had just saved | one workspace |
| `E_WRONG_KIND` | 6 (arrays passed to `overlay`, an Image passed to `within`) | arrays accepted; masks are images |
| `E_LAZY_COMPARE` | 7 | a lazy node used as data must still fail loudly (§7.7) |
| synonym and redirect keys ever typed by an agent | 14 of 246 | prune the rest |

Counts are regex occurrences in the code agents wrote: they measure use, not success.

### 3.3 Scope

In scope: the JavaScript surface (`prelude.js`, `binding.go`, `init.js`, help and docs), the binder and builder, the
space rules, the Mask and Param kinds, the Regions ops, data out, path handling, the CLI and MCP flags, the panic
boundary, and the documentation an agent or a person reads. Out of scope and unchanged: the decoders and writers,
the kernels, the render op classes, looks, crop, zoom, rulers, sheets and view presets (except how the speck view
computes its white point and its space, §7.4). Windowed `Halo` evaluation remains future work; nothing here changes
its design, because the conversions GCV-004 inserts are pointwise.

## 4. Concepts

This section builds the vocabulary of the new design in dependency order. Each concept is defined before a later
one uses it.

### 4.1 Literal options and data dependencies

An **option** is any argument of an op that is not a node input: `sigma` of `gaussian`, `black` and `white` of
`levels`, `polarity` of `detect.specks`. In v2 every option is a **literal**: a number, string, boolean, list or
object written in the script or computed by JavaScript. A **node input** is an argument the op declares as an image
or as regions (`b` of `sub`, `mask` of `within`, the items of `overlay`). Nothing else may be a node.

A **data dependency** is an option whose value comes from a measurement. In v2 it is expressed with `await`:

```js
const d = img.dog(1, 4)
const white = (await d.stats()).p99 * 5        // evaluate now; white is a number
return d.levels({ black: 0, white }).look()
```

The cost is one evaluation before the node is built, which the cache makes cheap (the `stats` node and its inputs
are reused when the levels node is evaluated). The benefit is that every option of every node is a known value at
build time: frames, encodings, schemas and channel counts can always be computed, and no accessor ever meets a node
where it expects a value.

### 4.2 The space of an image

Section 2.2 defined encodings and spaces. The **space of an image** is the space its values are in. In v2 every
Image node has one (it already has the static, `Node.Space`), and the rule for choosing it changes:

- `cv.open` returns an image in **encoded** space: its values are the file's values divided by the code maximum,
  the "file units" agents asked for 80 times with `stats({ space: "encoded" })`.
- `img.as("linear")` and `img.as("encoded")` convert. `as` is the only op that changes the space.
- Every other op returns its result in its receiver's space (with two exceptions in §4.3).

Measurements (`stats`, `histogram`, `sample`, `measure`, `profile`) report numbers in the space of the image they
measured. So `await img.stats()` is in file units, and `await img.as("linear").stats()` in linear light (for the
colleague's density tiles, linear = density).

### 4.3 Following, physical and display ops

Every op that reads pixel values belongs to one of three **space rules**:

- A **following** op computes in its receiver's space. Other image inputs are converted to that space first. Its
  result is in the receiver's space. Most ops follow: `gaussian`, `levels`, `contrast`, arithmetic, `blend`, mask
  makers, `detect.specks`, measurements.
- A **physical** op has a meaning only in linear light: `exposure` multiplies light by 2^ev; `heal` and `inpaint`
  correct seams additively, which is right for density and light, not for sRGB codes; `toGray("rec709")` and
  `toLab()` are defined on linear light. The builder converts the inputs to linear, runs the op, and converts the
  result back to the receiver's space. The caller never sees linear values unless it asked for them.
- A **display** op produces a picture for looking: `overlay`, `annotate`, `colormap`, `cv.compare`, `cv.sheet`. Its
  inputs are converted to encoded values and its result is an sRGB-encoded picture in encoded space.

A **derived** result is a new quantity: a difference (`sub`), a high-pass (`highpass`, `dog`), local contrast,
chroma, Lab, a mask, a view. Derived results are linear-encoded (their stored value is their value,
`registry.go:131-137`), so their two spaces coincide and `as` on them is the identity.

### 4.4 Masks as one-channel images; the channel count

A **mask** is a one-channel image whose values are weights, usually in [0, 1]. In v2 it is not a separate kind: the
mask makers (`img.mask.luminance`, `img.mask.box`, …) return one-channel Image nodes, and the mask ops (`and`, `or`,
`minus`, `not`, `grow`, `shrink`, `feather`, `refine`, `coverage`, `components`) accept any one-channel image.

To keep "this needs a mask" a build-time check, every Image node gets a fourth static: its **channel count**
(`Node.Channels`, 1 or 3). Sources know it from the file header; most ops keep their receiver's; a few set it
(`toGray`, `channel`, mask makers: 1; `colormap`, `toLab`, displays: 3). An argument or receiver declared as `Mask` now
means "an Image with one channel".

### 4.5 Items, region sets and the item link

An **item** is a flat object describing one thing in the image: `{ id, x, y, w, h, cx, cy, area, aspect, … }`
(the GCV-003 schema: `x, y, w, h` the bounding box, `cx, cy` the centre, all in doc px). A **region set** is a list
of items whose regions also know their exact pixels (`Region.Pix`, `pkg/cv/graph/regions.go:15-23`), so they can be
drawn and measured exactly.

A **Regions node** is a lazy region set. Detectors (`detect.specks`, `components`) and `measure` produce them.
Awaited, a Regions node is a plain JavaScript array of items. Each item carries an **item link**: a hidden,
symbol-keyed property naming the node it came from. JSON, `Object.keys` and the YAML result do not see it; JavaScript
`filter`, `sort` and `slice` keep the same objects, and object spread (`{ ...r, note }`) copies symbol-keyed
properties, so the link survives the ways agents transform items.

An **item list** is what an op that takes regions accepts: a Regions node, or an array of items. Linked items are
resolved to their regions (exact pixels); unlinked objects with a box or a centre are **literal items** (geometry
only, drawn and measured by their box or centre).

### 4.6 The workspace

The **workspace** is the one directory a session reads and writes. `cv.open(path)` and `cv.save(path, x)` resolve a
relative path in it; an absolute path must lie in it. It is opened once as a Go `os.Root`, whose methods refuse any
path, `..` or symlink that leads outside the directory. A file the agent saved is therefore always reopenable by the
path `cv.save` returned.

### 4.7 One way per thing

The design rule of Part A: for each thing an agent wants to do there is one spelling. Evaluating is `await`. Showing
is `look()`. Writing a file is `cv.save`. Filtering regions is JavaScript. Choosing the space is `as`. An op has one
name and one receiver. Where the evidence shows agents typing a wrong name, the error carries the literal fix; there
is never a second accepted spelling.

## 5. Current-state analysis

This section shows, with line numbers, where today's code does each thing GCV-004 changes.

### 5.1 Params and value lifting

Params are a source op (`param`, `pkg/cv/ops/source.go:140-151`) over a store (`ParamStore`,
`pkg/cv/graph/params.go:29-180`) whose values enter every evaluation signature (`eval.go:173`, `eval.go:293-296`) and
every `EvalCtx` (`eval.go:33`). The kind `Param` (`registry.go:23`) satisfies `Value` in `accepts`
(`registry.go:33-40`) and falls back to Value ops in `Lookup` (`registry.go:406-417`). JavaScript reaches them through
`P.slider/toggle/choice/set/list` (`prelude.js:313-330`), `paramProp` (`prelude.js:196-208`) and six binding functions
(`binding.go:503-603`). The context block lists them (`processor.go:512-528`). Agents used none of it.

Lifting lets a Value node stand in for a number. `binder.lift` (`args.go:356-377`) records the node as an input and
the argument as an `InputRef`; `resolveArgs` (`args.go:1138-1156`) substitutes evaluated data and re-checks the type
at evaluation (`args.go:1146-1152`). Property access on an unawaited Value node builds a `field` node
(`prelude.js:265`), with `__fieldNames` (`pkg/gojamod/cv/fields.go:15-34`) deciding what is a field. Eleven Value ops
(`field`, `pick`, `percentile`, arithmetic; `pkg/cv/ops/value.go`) exist so lifted values can be combined. Because a
lifted value is unknown at build time, `Builder.resolveSpace` emits SPACE warnings when a lifted level comes from
another space (`spaceWarnings`, `space.go:156-178`), and the macro expander turns `InputRef`s back into node arguments
(`Expander.Arg`, `macro.go:40-64`). The speck view uses lifting internally for its white point
(`pkg/cv/ops/views.go:219-236`).

### 5.2 Silent accessors and the Static flag

Op code reads canonical arguments through accessors. Some panic on a wrong type (`Number`, `Length`,
`args.go:73-88`, `202-207`); most do not: `String` returns `""` (`args.go:222-226`), `Bool` false (`229-233`),
`Numbers` and `Strings` skip non-matching elements (`236-259`), `Object` nil (`262-266`), `Node` and `Nodes` nil or
fewer elements (`181-198`), `Func` zero (`173-177`). A build-time function that reads a lifted argument therefore
sees a zero value. To stop that, 23 `ArgSpec`s are marked `Static: true` (`registry.go:182`; the check is
`args.go:436-439`), and `BindOptions` marks all function options static (`funcs.go:146-151`). The flag is opt-in: the
review found `cv.open`'s `encoding` and `encode`'s `to` without it.

### 5.3 Node lists and deduplication

Kind-typed arguments (`"Image[]"` of `cv.compare`, `views` of `cv.sheet`, `mask` of `stats`) are lifted like values.
The binder deduplicates lifted nodes (`liftIdx`, `args.go:371-376`), so `[img, img]` becomes one input and two
`InputRef`s to it. Op code that needs the nodes in list order must walk the `InputRef`s: `listInputs`
(`pkg/cv/ops/present.go:107-119`, added by the review fix), `nodeOf` and `viewNodes` (`pkg/cv/ops/sheet.go:141-165`).
Other build-time code indexes inputs by position: `inputs[1].Frame` (`present.go:145`, `pkg/cv/ops/mask.go:378`,
`pkg/cv/ops/regionmeasure.go:160-168`) and `firstImage(inputs)` in the result rules (`registry.go:102-109`), which scans
lifted inputs too.

### 5.4 Space as an op option

`SpaceRule{Default, Fixed, For}` (`registry.go:70-79`) gives each op a default working space; `Register` appends a
`space` option to every op whose rule is not fixed (`registry.go:370-375`, the option at `150-152` with ten value
synonyms at `156-161`). `OpSpace` (`space.go:33-45`) resolves it, `resolveSpace` (`space.go:71-112`) stores it in the
arguments and converts inputs, and view presets pick a per-call default through `SpaceRule.For` and `presetSpace`
(`views.go:141-156`, `257`). Sources are linear (`source.go:26`). The defaults split the ops: exposure, levels,
gaussian, blend, within, heal, inpaint, stats and the measurements default to linear; arithmetic, masks, detection,
histogram, contrast, high-pass and views to encoded (`goja-cv ops`, `pkg/cv/ops/*.go`). An agent who wants file units
must remember which is which; the evidence shows it overriding `stats` alone 85 times.

### 5.5 The Mask kind

`KindMask` (`registry.go:20`) is a separate receiver kind with its own method table. Because a mask is an image to
the evaluator (`ImageValue` with `kind` Mask, `pkg/cv/graph/value.go:15-61`), several ops are registered twice or
three times: `crop` on Image and Mask (`pkg/cv/ops/geometry.go:26`), `overlay` on Image, Mask and Regions
(`present.go:124`, `139`, `mask.go:377`). `img.stats()` exists, `mask.stats()` does not; `mask.coverage()` exists,
`img.coverage()` does not. The agents' `E_UNKNOWN_METHOD` errors for `mask.stats` and `Mask.coverage` come from this
split.

### 5.6 Region queries and their machinery

Region sets are filtered by ops: `where` with a declarative predicate (`pkg/cv/ops/regions.go:272-294`, validated
against the receiver's schema at build time by `validatePredicate`, `37-85`), `only`/`without` (`163-231`),
`sortBy` (`297-346`), `top` (`347-361`), `count` (`362-368`) and `filter` with a data lambda (`389-413`). Their
support code spans the system: `OpDef.Produces` and `Registry.producerHint` (`registry.go:289-292`,
`schema.go:107-133`) say which op adds a missing field; `UnknownFieldError` and `RemovedFields` (`schema.go:43-105`)
refuse removed names at build time and through throwing accessors on a shared item prototype (`prelude.js:55-69`,
`fields.go:36-62`); `OpDef.MaxItems` (`registry.go:267-270`) lets a sheet size itself by `top(n)`
(`sheet.go:54-72`); data lambdas need `FuncArg`, the `Callbacks` interface and `binding.CallEach`
(`args.go:17-28`, `eval.go:63-70`, `binding.go:45-102`). An awaited Regions node is `{ kind, count, items, scale,
frame }` (`regions.go:209-224`) with array methods that throw `E_NOT_ARRAY` (`prelude.js:88-98`) and methods that
collide with data fields (`count`, explained after the fact by `explainDataMisuse`, `processor.go:449-489`).

### 5.7 Two ways to evaluate, two ways to show, two ways to describe

`await node` evaluates at the preview scale; `cv.evaluate(node, { scale })` at an explicit one (`prelude.js:355-361`,
`binding.go:330-351`). `node.look(opts)` creates a look that is delivered if it appears in the returned value;
`show(node, opts)` creates one and queues it (`init.js:71-75`, `native.go:100-114`, delivered by `collect`,
`processor.go:136-141`). `cv.describe(node)` (`binding.go:417-434`) and `node.explain()` (`pkg/cv/ops/meta.go:13`) both
describe a node.

### 5.8 Three output functions and two naming rules

`node.save(path, { depth })` writes an image (`pkg/gojamod/cv/dataout.go:69-90`); `cv.json(x, { name })` and
`cv.csv(x, { name, columns })` make Data handles (`dataout.go:132-174`) that are delivered as content named
`<name>.<ext>` or saved with `data.save(path)` (`dataout.go:176-192`). Data names follow `NamePattern`
(`pkg/cv/dataout/data.go:34-35`) and their own error with a computed fix (`data.go:69-82`); file names in the CLI's
`--out` follow a third rule (`writeContent` and `uniqueName`, `cmd/goja-cv/eval.go:200-265`).

### 5.9 Two roots

Reads are restricted by `Options.Roots` (`session.go:61-62`, MCP `--root`, repeatable, `cmd/goja-cv/mcp.go:81`;
checked in `native.checkPath`, `pkg/cvsession/native.go:65-84`, through `graph.Within`). Writes are restricted by
`Options.OutRoot` (`session.go:63-66`, `--out-root`, `mcp.go:82`, `eval.go:35`), checked by `OutPath` and again in
`WriteOut` (`paths.go:54-102`). The CLI's `eval` and `run` have no read root at all. A file saved under the out-root is
unreadable unless the out-root is also a read root, which is the 23 `E_ACCESS` errors.

### 5.10 Render's postcondition

`Evaluator.Render`'s comment promises that an Image result's buffer is `ScaledSize(window, scale)`
(`render.go:40-43`). The scale ≤ 1 branch evaluates, cuts (`cutValue`, `270-284`, which covers every buffer pixel the
window touches and can therefore be one pixel larger) and then calls `fitScale` (`255-268`), which resizes only when
the buffer exceeds the target by more than one pixel. The above-1 branch relies on each op class producing the right
size. The postcondition is checked in one branch, with a tolerance.

### 5.11 Hand-written JSON and CSV shape guessing

`encodeJSON`/`writeJSON`/`writeString`/`writeObject` (`data.go:251-332`) hand-write JSON to keep `graph.Record` key
order and to turn NaN into null. `table` (`data.go:366-445`) guesses the shape of data for CSV: a list of objects, an
object with column arrays (a profile, columns chosen by `columnArrays`, `334-364`, by schema or, without one, by the
longest arrays), awaited Regions (`{ kind, items }`), a list of lists (`c1, c2, …`), or a scalar (`value`).

### 5.12 Panics outside the boundary

`safeRun` (`eval.go:449-456`) recovers panics in an op's `Run`. Nothing recovers panics elsewhere: in binding
functions called from JavaScript (`binding.object`, `binding.go:104-137`), in `Engine.Open`'s header probe
(`engine.go:117`), in the binder and build-time functions, in the evaluator's goroutines outside `Run`
(`eval.go:358-370`, `render.go:178-195`), or in look and data rendering. goja turns a panic whose value is a JavaScript
value into an exception; any other panic unwinds out of the VM and, on a goroutine nobody recovers, ends the
process — the MCP server with it.

### 5.13 `cv.open`'s hidden arguments

`Engine.Open` probes the header and passes ten values as "trusted" arguments (`engine.go:125-137`): `stamp`, `w`, `h`,
`format`, `bits`, `channels`, `sample`, `fileEncoding`, `recordedOrigin`, `notes`, declared as hidden `ArgSpec`s
(`source.go:108-117`). They are hashed, so the node changes when the file changes, and the frame and encoding rules
read them (`openFrame`, `source.go:32-47`; `sourceEnc`, `source.go:14-27`). The binder has a `trusted` parameter only
for them and for the placeholder's four hidden arguments (`source.go:154-159`, `engine.go:163-164`). They are user
`Args` that no user may set.

### 5.14 Synonym and redirect tables

Did-you-mean has two mechanisms: edit distance over valid names (`closest`, `pkg/cv/graph/suggest.go:137-153`), and
tables that map other words to goja-cv names: op synonyms (`Synonyms`, `suggest.go:10-86`, 77 entries), method
redirects (`MethodRedirects`, `suggest.go:90-105`), option synonyms (`OptionSynonyms`, `args.go:815-843`, ~140 keys),
per-argument `ValueSynonyms` and `Redirects` (e.g. `views.go:262-266`, `regionmeasure.go:311-315`), stat synonyms
(`regionmeasure.go:31-33`) and `as` synonyms (`pkg/cvsession/names.go:66`). `scripts/02-pi-names-typed.py` finds that
agents typed 14 of the 246 keys.

## 6. The v2 JavaScript API

Each subsection shows what an agent writes today and what it writes after GCV-004, then states the rule. The
examples run against the planted scan (`internal/testimage/planted.go`) bound to `img`.

### 6.1 Options are literal values

```js
// before: a param, and a lazy measurement lifted into an option
const ev = P.slider("ev", { min: -2, max: 2, value: 0.5 })
const d = img.dog(1, 4)
return d.levels({ black: 0, white: d.stats().p99.mul(5) }).exposure(ev).look()
```

```js
// after: numbers are numbers; a measurement is awaited first
const d = img.dog(1, 4)
const white = 5 * (await d.stats()).p99
return d.levels({ black: 0, white }).exposure(0.5).look()
```

Rules:

- Every option is a literal. Passing a node where an option is expected is `E_ARG_TYPE` with the fix
  `(await node).<field>`.
- Node inputs exist only where an op declares an image, a mask or items (`sub(b)`, `within(mask, fn)`,
  `overlay(items)`).
- Reading a property of an unawaited Value or Regions node (`img.stats().p99`), or using one as a number
  (`img.stats() > 0.5`), throws `E_NOT_AWAITED` with the fix `(await img.stats()).p99` (§7.7).
- `P` and the `Param` kind do not exist. A person who wants to explore a value re-runs the eval with another literal.

### 6.2 Space belongs to the image

```js
// before: numbers in file units need an option on every call; defaults differ by op
const s = await img.stats({ space: "encoded" })
const soft = img.gaussian(2, { space: "encoded" })
const lin = await img.stats()                                  // linear by default
```

```js
// after: images are in file units; as() switches
const s = await img.stats()                                    // file units
const soft = img.gaussian(2)                                   // blurs file values
const lin = await img.as("linear").stats()                     // linear light (density on the DUST-002 tiles)
const brighter = img.exposure(1)                               // physical: computed in linear light, returned in file units
```

Rules:

- `cv.open` returns an image in encoded space (file units). `await img` reports `space: "encoded"`.
- `img.as("linear" | "encoded")` converts; on a linear-encoded image (or a derived one) it is the identity.
- Every op returns its result in its receiver's space, except display ops (sRGB pictures) and derived results
  (linear-encoded, so the space does not matter). Measurements report in the receiver's space.
- `exposure`, `heal`, `inpaint`, `toGray("rec709")` (also the luminance input of `mask.luminance`, `histogram`,
  `detect.specks`, `check.dust`, `refine`), `toLab` and `encode` are physical: computed in linear light, returned in the
  receiver's space. `view("speck")` and `view("speckColor")` compute in linear light (the GCV-003 ranking result) and
  return a derived picture.
- There is no `{ space }` option. `img.gaussian(2, { space: "linear" })` is `E_UNKNOWN_OPTION` whose fix is
  `img.as("linear").gaussian(2)`.

### 6.3 A mask is a one-channel image

```js
// before: two kinds, two method tables
const m = img.mask.luminance({ above: 0.6 })
const c = await m.coverage()          // ok
const s = await m.stats()             // E_UNKNOWN_METHOD: Mask has no method "stats"
const shown = m.overlay(img)          // a third spelling of overlay
```

```js
// after: one kind
const m = img.mask.luminance({ above: 0.6 })      // an Image with one channel
const c = await m.coverage()                      // the mean of a one-channel image
const s = await m.stats()                         // any image op works on a mask
const shown = img.overlay(m)                      // the one overlay
const edited = img.within(m.feather(4), i => i.exposure(-0.4))
const wrong = img.within(img, i => i.exposure(-0.4)) // E_WRONG_KIND: within needs a one-channel image (a mask); img has 3 channels. fix: img.toGray()
```

Rules: the mask ops (`and`, `or`, `minus`, `not`, `grow`, `shrink`, `feather`, `refine`, `coverage`, `components`)
are Image methods that require a one-channel receiver; arguments declared as a mask accept any one-channel image.
Both are checked at build time from the channel count.

### 6.4 Regions are arrays

```js
// before: query ops on lazy nodes, { count, items } after await
const sp = img.detect.specks({ polarity: "bright" })
const top = sp.where({ score: { gte: 0.5 } }).sortBy("contrast").top(20)
const n = await sp.count()
const items = (await top).items
const m = await sp.measure(img, { ring: true })                 // regions.measure
const mask = top.toMask({ grow: 2 })
return { n, items, look: img.overlay(top, { labels: "id" }).look() }
```

```js
// after: JavaScript does the querying; arrays go wherever regions go
const sp = await img.detect.specks({ polarity: "bright" })      // an array of items
const top = sp.filter(r => r.score >= 0.5).sort((a, b) => b.contrast - a.contrast).slice(0, 20)
const m = await img.as("linear").measure(top)                   // ring measure at each item: step, spread, snr, …
const mask = img.mask.regions(top, { grow: 2 })                 // a one-channel image
return { n: sp.length, top: m, look: img.overlay(top, { labels: "id" }).look() }
```

Rules:

- A Regions node is a lazy region set produced by `detect.specks`, `components` and `measure`. It has no methods.
- Awaiting it gives a plain array of flat items in the GCV-003 schema. Each item carries a hidden link to its region
  (§4.5). `filter`, `sort`, `slice`, `map` and spreading keep the link.
- Every op that takes regions (`overlay`, `heal`, `stats({ mask })`, `mask.regions`, `measure`, `cv.sheet`) accepts a
  Regions node or an array of items. Linked items keep their exact pixels; plain objects with `x, y, w, h` or
  `cx, cy` (or `[x, y, w, h]` and `[x, y]` lists) are literal items, drawn and measured by their box or centre.
- `measure(items, { ring, stats, coreRadius, channels, prefix, gray, analysisScale })` is the one measuring op for
  items and points: it returns a Regions node whose items are the input items with the new fields (`ring` defaults to
  `true`). It replaces `img.measure({ points })` and `regions.measure(img, …)`.
- `img.mask.regions(items, { grow })` replaces `regions.toMask({ grow })`; the mask has the image's frame.
- `where`, `sortBy`, `top`, `only`, `without`, `count` and `filter` are gone. Calling any method on a Regions node is
  `E_NOT_AWAITED` with the fix `(await regions).<method>(…)`, or, for an image method such as `overlay`, the fix
  `img.overlay(regions)`.

### 6.5 Evaluate with await; show with look

```js
// before
const s = await cv.evaluate(img.stats(), { scale: "full" })
show(img.view("speck"), "speck")
const d = cv.describe(img.gaussian(2))
```

```js
// after
const s = await img.stats()                         // await evaluates at full resolution
img.view("speck").look("speck")                     // every look created in the eval is delivered
const d = cv.explain(img.gaussian(2))               // id, kind, op, call, frame, space, channels, graph lines
```

Rules:

- `await node` is the only evaluation, always at scale 1. There is no preview scale for data.
- `node.look(opts)` is the only display call. It creates a look and queues it; queued looks are delivered after the
  eval, in creation order, up to the budget of four, also when the eval throws later. Returning a look puts its
  placeholder in the result; not returning it still delivers it.
- `cv.explain(node)` replaces both `cv.describe(node)` and `node.explain()`; it is synchronous and evaluates nothing.
- `show`, `cv.evaluate`, `cv.describe`, `cv.isNode`, `cv.warnings`, `cv.kinds` and the `doc` stub are gone.

### 6.6 One spelling per op

| before | after |
|---|---|
| `regions.overlay(img, opts)` | `img.overlay(regions, opts)` |
| `mask.overlay(img, opts)` | `img.overlay(mask, opts)` |
| `regions.measure(img, opts)`, `img.measure({ points })` | `img.measure(items, opts)` |
| `regions.toMask(opts)` | `img.mask.regions(items, opts)` |
| `mask.crop(box)` and `img.crop(box)` (two ops) | `x.crop(box)` (one op on Image) |
| `cv.describe(node)`, `node.explain()` | `cv.explain(node)` |
| `cv.open(path, { as \| name \| bind \| alias })` | `cv.open(path, { as })` |

### 6.7 One save, one workspace

```js
// before: three functions, two naming rules, two roots
const saved = await img.view("speck").save("views/speck.tif")      // under --out-root
const csv = cv.csv(regions, { name: "candidates" })                 // content named candidates.csv
const again = cv.open(saved.path)                                   // E_ACCESS unless --out-root is also a --root
return { csv, json: cv.json(await regions) }
```

```js
// after: one function, the format from the extension, one directory
const sp = await img.detect.specks()
const saved = await cv.save("views/speck.tif", img.view("speck"))  // { path: "views/speck.tif", format, depth, encoding, space, frame, bytes, replaced }
await cv.save("candidates.csv", sp)                                 // a list of rows
await cv.save("candidates.json", sp)
const again = cv.open(saved.path)                                   // the same workspace
return { rows: sp.length, again: await again }
```

Rules:

- `cv.save(path, x, { depth })` returns a promise. The extension picks the format: `.tif`, `.png`, `.npy`, `.jpg` for
  an Image node (evaluated at scale 1, encoding and frame origin recorded where the format can), `.json` for any data
  (a Value or Regions node evaluated at scale 1, or plain data as the agent holds it), `.csv` for a list of rows.
- CSV takes only a list of objects. An object (a `stats()` result, a profile) is `E_ARG_TYPE` with the fix
  `cv.save("x.json", …)` or, for profiles and histograms, `cv.save("x.csv", (await p).rows)`.
- Saving the same path twice in one eval is `E_DUPLICATE`. Overwriting a file this session opened as an image is
  `E_ACCESS`. Overwriting another existing file is allowed and reported as `replaced: true`.
- Paths are relative to the workspace (`--root`, default: the current directory). Absolute paths must lie in it.
  `..` and symlinks that leave it are `E_ACCESS`.
- In a read-only session (`--read-only`), `cv.save` writes nothing: `.csv`, `.json` and `.png` are delivered as
  result content named by the path (the CLI writes them under `--out`), and resolve with `{ path, inline: true, … }`;
  other formats are `E_ACCESS` with the fix `.png`.

### 6.8 Names agents typed

The did-you-mean machinery keeps edit distance over valid names (it costs no table) and a short table of wrong names
agents demonstrably typed, each with a literal fix. Everything else in today's synonym and redirect tables is deleted.
The kept list is in §7.13.

### 6.9 The v2 surface in one table

Globals in a session: `cv`, `img` (and any name bound with `cv.open(path, { as })`), `mem`.

| call | returns | notes |
|---|---|---|
| `cv.open(path, { encoding, box, origin, as })` | Image (encoded space) | path in the workspace; `encoding: "linear"` for linear data in integer files |
| `cv.save(path, x, { depth })` | Promise of `{ path, format, bytes, rows, depth, encoding, space, frame, replaced, inline }` | format from the extension; §6.7 |
| `cv.compare([a, b, …], { mode })` | Image (picture) | `mode: "side" \| "split"` |
| `cv.sheet(items, { views, size, zoom, labels, max, stretch })` | Image (picture) | items: Regions node or array |
| `cv.explain(node)` | object (synchronous) | id, kind, op, call, frame, space, channels, graph lines |
| `cv.help(name?)`, `cv.docs.list()`, `cv.docs.search(text, { limit })`, `cv.docs.read(id, { section })` | data | never throw |
| `cv.version` | string | |
| `x.look(label \| { label, zoom, interp, maxSide, ruler, grid, format })` | look (queued) | Image and chartable Value nodes |
| `img.as(space)` | Image | `"linear"` or `"encoded"` |
| `img.exposure(ev)` | Image | physical |
| `img.levels({ black, white, gamma, auto })`, `img.contrast(amount)` | Image | |
| `img.toGray(weights)` | Image, 1 channel | `"rec709"` physical, `"average"` follows |
| `img.gaussian(sigma)`, `img.highpass(sigma, { polarity })`, `img.dog(s1, s2, { polarity })`, `img.localContrast(sigma)` | Image | high-pass results are derived |
| `img.add(b)`, `sub(b)`, `mul(b)`, `max(b)`, `min(b)`, `abs()` | Image | `b`: an image or a number; `sub` is derived |
| `img.blend(b, { mode, opacity, mask })`, `img.within(mask, i => …)` | Image | |
| `img.heal(itemsOrMask, { method, radius })`, `img.inpaint(mask, { method })` | Image | physical |
| `img.channel(c)`, `img.split()`, `img.toLab()`, `img.chroma({ relative })`, `img.colormap(name, { range, center })`, `img.encode(to)` | Image | `colormap` needs one channel |
| `img.crop(box)`, `img.view(name, { size })` | Image | |
| `img.overlay(itemsOrMask, { color, alpha, labels })`, `img.annotate(marks, { width })` | Image (display) | decorations |
| `img.mask.luminance({ above, below, softness })`, `mask.color({ hue, minSat, tol })`, `mask.flood(point, { tol })`, `mask.box(box)`, `mask.ellipse(box)`, `mask.gradient({ from, to })`, `mask.regions(items, { grow })` | Image, 1 channel | mask makers |
| `m.and(m2)`, `or(m2)`, `minus(m2)`, `not()`, `grow(r)`, `shrink(r)`, `feather(r)`, `refine(guide, { radius, eps })` | Image, 1 channel | receiver: one channel |
| `m.coverage()` | Value (number) | receiver: one channel |
| `m.components({ minArea, threshold, analysisScale })` | Regions | receiver: one channel |
| `img.detect.specks({ polarity, minContrast, maxArea, analysisScale })` | Regions | |
| `img.check.dust(opts)` | Value | |
| `img.stats({ percentiles, mask, box })` | Value | `mask`: a one-channel image or items |
| `img.histogram({ bins, channel })` | Value `{ bins, channel, under, over, total, rows }` | chartable |
| `img.sample(points, { radius, gray, analysisScale })` | Value (rows) | points may be items |
| `img.profile(from, to, { step, width, gray, analysisScale })` | Value `{ from, to, step, width, rows }` | chartable |
| `img.measure(items, { ring, stats, coreRadius, channels, prefix, gray, analysisScale })` | Regions | `ring` defaults to true |

Awaited values: an Image gives `{ kind, w, h, channels, encoding, space, frame, … }` (plus file metadata for an opened
file), a Value its data, a Regions node an array of items.

## 7. Proposed architecture

### 7.1 Overview

The layers of §2.1 stay. What changes inside them:

```
 JS (prelude.js)   nodes: Image | Regions | Value         awaited: summary | array of linked items | data
                   marshal: nodes → {__cvnode}; linked items → {…fields, __cvref: [node, id]}
        │                                                          ▲
        ▼                                                          │
 binding.go        every native function wrapped by export(): panic → E_INTERNAL
        │          __call  __evaluate(scale 1)  __look(queues)  __save  __explain  __notAwaited
        ▼
 graph.Builder     bind: literals only; node args → inputs in order (no dedup); items → internal "items" node
                   space: Follows | Physical (bracketed by as) | Display; statics: Frame Enc Space Channels Fields
        │
        ▼
 graph.Evaluator   scale 1 for data; stamp (frame, enc, NaN → nil for data); recover on every goroutine
 graph.Render      one exit: image result == ScaledSize(window, scale)
        │
        ▼
 dataout           cv.save: image writers | JSON (encoding/json, Record.MarshalJSON) | CSV (rows only)
 graph.Workspace   one os.Root: Open (cv.open, decoders) and WriteFile (cv.save)
```

| layer | removed | added or changed |
|---|---|---|
| `pkg/cv/graph` | `KindParam`, `KindMask`, `ParamStore`, lifting (`InputRef` for values, `liftIdx`), `Static`, `Hidden`, `trusted`, `SpaceRule{Default,Fixed,For}`, space option, SPACE warnings, `Produces`, `MaxItems`, `RemovedFields`, `FuncArg`/`Callbacks`, `Within`/`OutPath`/`WriteOut`, `fitScale`, most synonym tables, preview scale | loud `Args` accessors that also resolve node arguments; `SpaceRule` enum; `Node.Channels`; typed source metadata; `Workspace`; `items` op; `Record.MarshalJSON`; `Recover` |
| `pkg/cv/ops` | `where`, `sortBy`, `top`, `only`, `without`, `count`, `filter`, Value ops, `param`, duplicate `crop`/`overlay`, `regions.measure`, `toMask`, `explain` op | `as` (the old internal `convert`), `mask.regions`, unified `measure`, internal `levelsAt`; mask ops on Image; profile and histogram rows |
| `pkg/cv/dataout` | Data handles, hand-written JSON, `table`, `columnArrays` | `Save(ctx, eng, ws, path, x, opts)`, `encodeJSON`, `encodeCSV(rows)`, inline content |
| `pkg/gojamod/cv` | `P`, `cv.evaluate`, `cv.json`, `cv.csv`, `node.save`, `cv.describe`, `cv.isNode`, `cv.warnings`, `cv.kinds`, derived fields, throwing item prototype, `E_NOT_ARRAY` methods, collisions, data lambdas, `fields.go` | `cv.save`, `cv.explain`, item links, `E_NOT_AWAITED`, look queue, `export()` panic wrapper |
| `pkg/cvsession` | `P`, `show`, `doc`, `checkPath`, roots, out-root, params in the context block, `explainDataMisuse` | `Options.Root`, `Options.ReadOnly`; looks from the eval tracker |
| `cmd/goja-cv` | `--out-root`, repeatable `--root`, `uniqueName`, data captions | `--root`, `--read-only`; inline saves under `--out` by path |

### 7.2 Arguments: literal options, node arguments and loud accessors

**Binding.** `bind` keeps its shape (positional values, then one options object, defaults, types, limits, unknown
keys). Two things change. First, a node is accepted only for an argument whose declared type is a node type:
`Image`, `Mask` (a one-channel Image), `items` (a Regions node or an item list), a list of them (`Image[]`), or a
union containing one (`Image|number` for the operand of `add`). Anywhere else a node is `E_ARG_TYPE` with the fix
`(await x).<field>`. Second, every node argument becomes its own input: the binder appends it to the node's inputs
and records `InputRef{Index}` (or a list of them) as the canonical value. There is no `liftIdx`; `[img, img]` is two
inputs. The `Static` flag, the re-check in `resolveArgs` and the `trusted` parameter disappear.

`OpDef.Inputs` (unnamed positional node inputs) becomes ordinary named `ArgSpec`s with node types, so every input
of every op has a name. For example `overlay`:

```go
// before (present.go:124)
{Name: "overlay", On: graph.KindImage, Inputs: []graph.Kind{"Regions|Mask"}, …}
// after
{Name: "overlay", On: graph.KindImage, Space: graph.Display, Class: graph.Decoration,
    Args: append([]graph.ArgSpec{{Name: "target", Type: "items|Mask", Positional: true,
        Doc: "regions (a Regions node or an array of items) or a mask (a one-channel image)"}}, overlayArgs()...), …}
```

**Accessors.** `Args` carries the node's inputs at build time and their values at evaluation time, so op code never
indexes inputs itself:

```go
type Args struct {
    op   *OpDef
    vals map[string]any // literals; node arguments as InputRef or []any of InputRef
    meta any            // typed Go-side metadata of source nodes (§7.12)
    ins  []*Node        // the node's inputs (build time and evaluation)
    vs   []Value        // their values (evaluation, non-lazy ops only)
}

func (a Args) Receiver() *Node           // ins[0]; panics for source ops
func (a Args) Input(name string) *Node   // a node argument; nil only if the argument is optional and absent
func (a Args) Inputs(name string) []*Node // a node-list argument, in list order, repetitions kept
func (a Args) Value(name string) Value   // evaluated; panics in a Lazy op or before evaluation
func (a Args) Values(name string) []Value
func (a Args) String(name string) string // panics unless the spec is a string or enum and the value a string
func (a Args) Has(name string) bool      // the only way to ask whether an optional argument was given
```

Every accessor checks the spec's type and the value's Go type and panics with `cv: <op>.<arg> is <T>, not <want>` when
they disagree, and when an optional argument without a default is read without `Has`. There is no zero-value path.
Build-time functions change signature from `(args Args, inputs []*Node)` to `(a Args)`: `DocFrame`, `Fields`,
`Validate`, `ResultEnc.Enc/Space`, `Warn`, `Channels`. A panic in one of them is a programming error; the binding's
panic boundary (§7.11) reports it as `E_INTERNAL`, and the registry property test (§14.1) calls every build-time
function of every op with default arguments so such panics are found in CI, not by agents.

**What remains of the old problem.** With literal options, an accessor can no longer meet a node where it expects a
value: the review's `encoding` bug cannot be written. What remains is an op author reading an argument with the wrong
accessor (an enum through `Number`, an optional argument without `Has`). Today that silently yields zero; after
GCV-004 it panics on the first test that builds the op. Union-typed arguments (`ring: bool|number[]`) are still read
with `Raw` and a type switch; the switch's `default` branch must panic.

### 7.3 Node lists

The node-list class of bug came from two representations (the deduplicated inputs and the bound list). After §7.2
there is one: the canonical argument is a list of `InputRef`s, each pointing at its own input, and `Args.Inputs(name)`
(build time) and `Args.Values(name)` (evaluation) resolve it in list order. `cv.compare`'s frame becomes:

```go
DocFrame: func(a graph.Args) graph.Frame {
    ims := a.Inputs("images")          // [img, img] → two nodes
    f := ims[0].Frame
    …
}
```

`listInputs` (`present.go:107-119`), `nodeOf` and `viewNodes` (`sheet.go:141-165`), `firstImage`
(`registry.go:102-109`) and every `inputs[k]` in build-time code are deleted; the sheet's lazy `Run` reads its cell
views with `a.Inputs("views")` and its items with `a.Input("items")`. Evaluating a repeated input twice costs nothing:
both slots have the same signature, so the run memo returns the first value (`eval.go:323-328`).

### 7.4 Spaces

**The rule type.** `SpaceRule` becomes a three-valued enum; its zero value is the common case:

```go
// SpaceRule is how an op relates to the space of its receiver (§4.3).
type SpaceRule uint8

const (
    Follows  SpaceRule = iota // works in the receiver's space; other image inputs are converted to it
    Physical                  // works in linear light; the builder converts inputs to linear and the result back
    Display                   // works on display values; inputs are converted to encoded; the result is a picture
)
```

`Default`, `Fixed`, `For`, `DefaultSpace`, `OpSpace`, `spaceOption`, `SpaceArg`, `SpaceValueSynonyms`, `presetSpace`
and the reference's special case go. The working space is not stored in the arguments: it is a function of the
receiver's `Space`, which is already part of the hash through the receiver's own hash.

**Sources.** The `open` op's result is in encoded space: its `Run` takes the pyramid level (linear values, so that
downsampling averages light) and converts it to encoded values with `buffer.Convert` (a copy; `AtScale(1)` returns
the pyramid's own level, which must not be modified, `pkg/cv/buffer/pyramid.go:61-63`). For a linear-encoded file the
conversion is the identity and `Run` returns the level as before. `sourceEnc.Space` (`source.go:26`) returns encoded.

**`as`.** The internal `convert` op becomes the public `as(to)`: positional enum `linear|encoded`, `Identity` when
`buffer.Identity(enc, from, to)`, result space `to`. `Engine.Display` (`engine.go:171-184`) interns `as("encoded")`.

**Resolution in the builder.** `resolveSpace` is replaced by:

```
work := receiver.Space            (Follows)
     |  linear                     (Physical)
     |  encoded                    (Display)
for each luminance input (op.Gray):  input := toGray(input, "rec709")     // toGray rec709 is Physical: bracketed
for each Image input in := … whose Space ≠ work and !Identity(in.Enc, in.Space, work):
    input := as(in, work)
node := internNode(op, args, inputs)                                   // node.Space = work for KeepEnc results
if op.Space == Physical && node is an Image && !Identity(node.Enc, work, receiver.Space):
    node = as(node, receiver.Space)                                    // the bracket's closing conversion
ENCODING warning: unchanged (encoded work space, inputs of different encodings, not Display)
```

So `img.exposure(1)` on an srgb file in encoded space builds `as(exposure(as(img, linear), 1), encoded)`. The caption
chain already hides inserted conversions (GCV-002 deviation 9); `cv.explain` shows them. The SPACE warning
disappears with lifting: no option can carry a number from another space any more.

**The output-space rule**, stated once for the docs and the tests:

1. `cv.open` returns an image in encoded space.
2. `as(s)` returns its receiver's values in space `s`. No other op changes the space.
3. A `Follows` or `Physical` op's Image result is in its receiver's space.
4. A `Display` op's result is an sRGB-encoded picture (encoded space).
5. A derived result (`DerivedEnc`: `sub`, `highpass`, `dog`, `localContrast`, `chroma`, `toLab`, masks, views) is
   linear-encoded: its two spaces coincide, so rule 3 holds trivially and `as` is the identity on it.
6. A measurement reports in its receiver's space; on a derived image, in the derived quantity's own units.

**Which ops are physical.** `exposure`, `heal`, `inpaint`, `toGray` with `rec709` (and therefore every luminance
input), `toLab` and `encode`. `gaussian`, `blend`, `within`, `levels`, `stats` and the measurements follow. The
rationale and the evidence are in §8 (D2, D3). Display: `overlay`, `annotate`, `colormap`, `compare`, `sheet`.

**Views.** `view` follows. The `speck` and `speckColor` presets start their expansion with `as("linear")`, keeping the
linear default GCV-003 chose with `TestRealViewRanking`. Their white point, today a lifted chain
`max(5 × p99, 1e-4)` (`views.go:219-236`), becomes one internal op:

```go
// levelsAt is levels with black 0 and white = max(times × the p-quantile of the receiver's luminance, floor),
// computed when it runs: the speck view's stretch. Internal: not callable from JavaScript.
{Name: "levelsAt", On: graph.KindImage, Internal: true, Global: true, Returns: graph.KindImage,
    Args: []graph.ArgSpec{{Name: "p", Type: "number", Default: 0.99}, {Name: "times", Type: "number", Default: 5.0},
        {Name: "floor", Type: "number", Default: 1e-4}},
    Run: /* the percentile with the function stats uses (same ties, same weights), then the levels kernel */}
```

Because it computes the percentile with the same function `stats` uses, the speck view's output does not change
(the `sheet-2x2` golden holds). The other presets lose their `SpaceArg` plumbing and otherwise stay.

### 7.5 Channels and masks

**The static.** `Node.Channels int` is computed in `internNode` like the other statics: for an Image it is
`op.Channels(a)` when the op declares it, else the receiver's count; 0 for Regions and Value. Sources read it from the
probe (`buffer.Info.Channels`, after dropping an alpha or extra sample, GCV-001 deviation 10). Ops that set it:

| ops | channels |
|---|---|
| `toGray`, `channel`, each output of `split`, `chroma`, mask makers (`mask.*`), mask ops (`and`, `or`, `minus`, `not`, `grow`, `shrink`, `feather`, `refine`) | 1 |
| `colormap`, `toLab`, `overlay`, `annotate`, `compare`, `sheet` | 3 |
| macros (`view`, `highpass`, `dog`) | their expansion's output |
| everything else | the receiver's |

A property test (§14.1) evaluates every op on a grey and a colour fixture and requires `Buf.C == Node.Channels`, so a
wrong declaration is found immediately.

**Declarations.** The type word `Mask` in an `ArgSpec` means "an Image node with one channel". A receiver
requirement is `OpDef.OneChannel bool`. `accepts` (`registry.go:33-40`) checks kind and, for `Mask`, channels.
Failures are `E_WRONG_KIND` at build time: "within needs a one-channel image (a mask) as mask; this image has 3
channels" with the fix `x.toGray()`.

**Deletions that follow.** `KindMask`, `ImageValue.kind`, `NewMask`, `IsMask` (`value.go:52-61`), the Mask method
table, the Mask branches in `kindHint` (`args.go:392-402`), in looks (`lookKindError`), in help and in the prelude; the
second `crop` (`geometry.go:26` registers it per kind) and the Mask and Regions `overlay`s (`mask.go:377`,
`present.go:138-147`). An awaited one-channel image reports `channels: 1`; `coverage()` gives the mean.

The grey-input rule (`OpDef.Gray`) now skips `toGray` for one-channel inputs, because the channel count is static;
GCV-001 deviation 2 had to insert it always.

### 7.6 Items and Regions

**What a Regions node is.** A lazy region set with no methods. Its value is a `RegionsValue` (`regions.go:28-36`)
as today. Three ops produce Regions: `detect.specks`, `components` and `measure`; one internal op, `items`, turns an
item list into a Regions node.

**Awaiting.** `resultToJS` (`binding.go:395-415`) returns a Regions value as an array of `Record`s in schema order
(the `{ kind, count, items, scale, frame }` wrapper goes). The prelude links each item:

```js
const ITEM = Symbol("cv.item"); // enumerable symbol key: copied by spread, invisible to JSON, Object.keys and YAML

function linkItems(arr, node) {
  for (const it of arr) it[ITEM] = node.__id; // plain assignment: an enumerable own symbol property
  return arr;
}
// then of a Regions node:  native.__evaluate(id).then(v => linkItems(v, node))
```

**Passing items back.** `marshal` (`prelude.js:121-133`) sends a linked item as its fields plus one key:

```js
if (typeof x === "object" && x !== null && x[ITEM] !== undefined) {
  const o = { __cvref: [x[ITEM], x.id] };
  for (const k of Object.keys(x)) o[k] = marshal(x[k]);
  return o; // still a box (x, y, w, h) and a point (cx, cy) for arguments of those types
}
```

The box and point checks already ignore other keys (`args.go:669-711`), so `img.crop(item)` and `img.sample([item])`
keep working. For an `items` argument the binder does this:

```
items(v):
  v is a Regions node            → an input, as any node argument
  v is a list:
    linked  := elements with __cvref        (node id, item id)
    literal := objects with x, y, w, h or cx, cy; [x, y, w, h] and [x, y] lists
    anything else                 → E_ARG_TYPE "items[i] is neither a region item nor a box or point"
    the linked elements name more than one node → E_ARG_TYPE, fix: pass each set in its own call
    entries := in array order: { id } for linked, { x, y, w, h } / { cx, cy } (and id if given) for literal
    node    := intern("items", inputs: [the linked node] or none, args: { entries })
    return an InputRef to node
```

The `items` op (internal, `AnalysisScale` 1) evaluates its source, looks regions up by id, keeps array order (an id
may repeat), and adds literal regions. Its schema is the source's for linked entries; for literal entries only the
geometry fields they all have. A list that is exactly the source's items in order still becomes an `items` node; that
costs one cheap op and keeps the rule simple.

**Literal regions.** `Region` gains a `Shape`: `Pixels` (from a detector, with `Pix`), `Box`, or `Point`.
`Rasterize` and `Coverage` (`regions.go:254-319`) draw a `Box` region with exact area coverage, as `mask.box` does.
Ops that need areas (`overlay`, `heal`, `mask.regions`, `stats({ mask })`) refuse an items node whose schema lacks
`w, h` at build time ("these items are points; give each a box: { x: cx - 3, y: cy - 3, w: 6, h: 6 }"). `measure`
with `ring` and `sample` need only centres; `measure` with `stats` needs areas.

**Frames.** An items node over a linked source has the source's frame; one of literal items has an empty frame, and
consumers skip the containment check for it (`containsFrame`, `present.go:28-34`). `mask.regions(items)` and
`measure` produce results in the receiver's frame.

**`measure`.** One op on Image: `measure(items, { ring = true, stats, coreRadius, channels, prefix, gray,
analysisScale })` returns a Regions node: the input items with the measurement fields appended (the GCV-003 field
names, `coreRingFields`, `newRegionFields`, `regionmeasure.go:76-104`). Point inputs become items with `id` 1…n and
`cx, cy`. The point form `img.measure({ points, ringRadii })` and `regions.measure(img, …)` go. `E_FIELD_EXISTS`
stays (use `prefix`).

**Deletions that follow.** The seven query ops (`regions.go:37-231`, `272-413`), `truthy`, `Produces` and
`producerHint`, `UnknownFieldError`, `RemovedFields` and `RemovedFieldError`, `regionMeasureProduces`,
`speckProduces`, `MaxItems` and `itemsBound` (a sheet over an array knows its length at build time; over a Regions
node it lays out what fits, as today), data lambdas (`FuncArg`, `FuncID`, the `func` type, `Callbacks`,
`Evaluator.CallEach`, `binding.funcJS`, `binding.CallEach`, `funcIds`), `ITEM_PROTO` and `__itemProto`, `__removedFields`,
`__noField`, `ARRAY_METHODS` and `__notArray`, `__collision` and `explainDataMisuse`'s collision branch, and
`pkg/gojamod/cv/fields.go`.

### 7.7 Evaluation, display and lazy data

**`await` at scale 1.** `binding.evaluate` always evaluates at scale 1; `scaleArg`, `EngineOptions.PreviewLong` and
`Engine.PreviewScale` go (`cv.explain` reports cache state at scale 1). Looks keep their own look scale. Awaiting an
Image still evaluates it, so errors surface at the `await`.

**Looks are queued.** `binding.look` creates the look and appends it to the eval tracker
(`pkg/gojamod/cv/evaltrack.go`), which the session reads at the end of the eval in place of `state.shows`. `collect`
(`processor.go:124-169`) takes the returned providers first, then the tracker's looks in creation order, deduplicated,
under the same budget. `show`, `native.queue`, `state.shows`, the `shown:` YAML block (`processor.go:86-89`,
`106-109`) and `FuncDef.Global` go.

**Lazy data fails loudly.** On a Value or Regions node the prelude's `lookup` (`prelude.js:210-267`) answers `then`,
`look` (Value only), `toString`, `toJSON`, `Symbol.toStringTag` and the identity fields; every other property, and
`Symbol.toPrimitive`/`valueOf`, calls `native.__notAwaited(id, prop)`, which throws:

```yaml
code: E_NOT_AWAITED
message: 'stats() is a lazy node: read .p99 after awaiting it.'
fix: (await img.stats()).p99
```

For a Regions node and an image method name (`overlay`, `heal`, `measure`, …) the hint says that regions are
arguments: `img.overlay(regions)`. This replaces `E_LAZY_COMPARE`, the derived `field` nodes, `E_NOT_ARRAY`, the
attached methods of `link` and the collision bookkeeping with one rule. Image nodes keep `E_UNKNOWN_METHOD` with
suggestions.

**`cv.explain(node)`.** A bespoke function: the describe map of `binding.describe` (`binding.go:417-434`) plus
`space`, `channels` and `graph`, the lines of `Evaluator.Explain(id, 1)`. The `explain` op (`meta.go`) and
`cv.describe` go.

### 7.8 The workspace and `cv.save`

**The type.**

```go
// Workspace is the one directory a session reads and writes (§4.6).
type Workspace struct {
    root     *os.Root
    dir      string // as given, cleaned and absolute
    resolved string // dir with symlinks resolved (macOS: /tmp → /private/tmp)
    readOnly bool
}

func OpenWorkspace(dir string, readOnly bool) (*Workspace, error)
func (w *Workspace) Rel(p string) (string, error)            // workspace-relative name, or E_ACCESS
func (w *Workspace) Open(p string) (*os.File, string, error) // for cv.open and the decoders
func (w *Workspace) Stat(p string) (fs.FileInfo, string, error)
func (w *Workspace) WriteFile(p string, data []byte) (rel string, replaced bool, err error)
```

`Rel` accepts a relative path, or an absolute path under `dir` or `resolved`; it cleans the path and refuses a
leading `..`. Everything else (symlinks that leave the directory, dangling links pointing outside, `..` inside a
link target) is refused by `os.Root` itself, whose errors `Open` and `WriteFile` map to `E_ACCESS`. `WriteFile` calls
`root.MkdirAll(filepath.Dir(rel), 0o755)` and `root.WriteFile(rel, data, 0o644)`; `replaced` comes from a
`root.Lstat` before the write. No containment code of goja-cv's own remains.

**Reading.** `Engine.Open` resolves the path with `Workspace.Open`, probes the header from the open file
(`buffer.ProbeFile` and `buffer.DecodeFile` take an `*os.File` instead of a path, `pkg/cv/buffer/decode.go:111-142`),
and records the workspace-relative path in the source metadata (§7.12). The source store loads by
`SourceKey{Rel, Stamp, Box, Enc}` and opens through the workspace when it decodes.

**`cv.save(path, x, { depth })`.** A bespoke function on the binding:

```
save(path, x, opts):
  rel := ws.Rel(path)                                       // E_ACCESS
  if rel in tracker.saved            → E_DUPLICATE ("saved at line N")
  if rel is the path of an image this session opened → E_ACCESS ("would overwrite an input")
  format := by extension: .tif .png .npy .jpg | .json | .csv   (else E_ARG_TYPE with the list)
  tracker.saved[rel] = line
  x is an Image node   → image format required; bytes := dataout.EncodeImage(eval(x, scale 1), format, depth)
  x is a Value/Regions node → data := eval(x, scale 1) as plain data (records in schema order)
  x is anything else   → data := exportOrdered(x)       // what the agent holds, edits included
  data formats: .json → encodeJSON(data); .csv → encodeCSV(data) (a list of rows, else E_ARG_TYPE)
  read-only: .csv .json .png → queue inline content named rel; others → E_ACCESS (fix: .png)
  else:      ws.WriteFile(rel, bytes)
  resolve { path: rel, format, bytes, rows?, depth?, encoding?, space?, frame?, replaced | inline: true }
```

The returned `path` is workspace-relative, so `cv.open(saved.path)` always works. Validation (path, duplicate,
format) happens synchronously at the call; evaluation and writing happen on a goroutine and settle the promise, as
`node.save` does today (`binding.async`, `pkg/gojamod/cv/dataout.go:43-67`).

**Flags.**

| flag | commands | meaning |
|---|---|---|
| `--root DIR` | `eval`, `run`, `mcp` | the workspace; default: the current directory |
| `--read-only` | `eval`, `run`, `mcp` | `cv.save` writes nothing and delivers `.csv`, `.json`, `.png` inline |
| `--image FILE` | `eval`, `run`, `mcp` | opened at session start and bound to `img`; must lie in the workspace |
| `--out DIR` | `eval`, `run` | where the CLI writes delivered looks and inline saves for the person (default: a temporary directory) |

`--out-root` and the repeatable `--root` of `mcp` go. `--image` and `--prelude` are operator input: they resolve
against the process's working directory; `--image` must then lie in the workspace (a usage error otherwise, naming
`--root`). Inline saves are written by the CLI under `--out` at their workspace-relative path; looks keep their
`look-<n>-<slug>` names. Both are unique within an eval by construction, so `uniqueName` goes.

### 7.9 Data encoding

**JSON.** `graph.Record` gets an ordered marshaller, and `encodeJSON` becomes `json.Marshal`:

```go
// MarshalJSON writes the record's keys in order.
func (r Record) MarshalJSON() ([]byte, error) {
    var b bytes.Buffer
    b.WriteByte('{')
    for i, k := range r.Keys {
        if i > 0 {
            b.WriteByte(',')
        }
        kb, _ := json.Marshal(k)
        vb, err := json.Marshal(r.Vals[k])
        if err != nil {
            return nil, err
        }
        b.Write(kb)
        b.WriteByte(':')
        b.Write(vb)
    }
    b.WriteByte('}')
    return b.Bytes(), nil
}
```

`encoding/json` escapes strings correctly, sorts plain maps and calls `MarshalJSON` on nested records. It refuses NaN
and infinities, so both encoders start with one function, `dataout.plain(v)`, which returns a copy of `v` with every
NaN and ±Inf replaced by `nil`: JSON `null`, an empty CSV cell. That is the one place the rule lives.

**CSV.** `encodeCSV(v)` accepts only a list whose elements are objects (`Record` or `map`). Columns are the keys in
first-seen order (records keep schema order; plain maps are sorted); a nested value is written as JSON in its cell.
Anything else is `E_ARG_TYPE`: "CSV is a list of rows; this is an object" with the fix `cv.save("x.json", …)`, or for
a value that has `rows`, `cv.save("x.csv", (await x).rows)`.

**Rows plus metadata.** `profile` returns `{ from, to, step, width, rows: [{ t, cx, cy, meanL, meanR, meanG, meanB }] }`
and `histogram` returns `{ bins, channel, under, over, total, rows: [{ lo, hi, count }] }`. `FieldSpec` gains
`Rows []FieldSpec` for the row schema of a `rows` field; `OrderBySchema` orders nested rows; the chart renderers
(`pkg/cv/look/chart.go:20-80`) read `rows`. `table`, `columnArrays`, `hasArrays`, `cell`'s JSON writer and the
hand-written JSON functions go.

### 7.10 Render: one exit

`Evaluator.Render` is split into a dispatcher that returns a value from whichever branch applies and one exit that
enforces the postcondition:

```go
func (e *Evaluator) Render(ctx context.Context, id NodeID, o RenderOptions) (Value, error) {
    n, win, s, err := e.renderRequest(id, o) // validation, window, quantized scale (render.go:45-63)
    if err != nil {
        return nil, err
    }
    v, err := e.renderBranch(ctx, n, win, s, o) // data, ≤ 1 (evaluate + cut), > 1 (render by op class)
    if err != nil || !n.IsImage() {
        return v, err
    }
    return exactSize(ctx, v.(*ImageValue), win, s) // the only place the size is decided
}
```

`exactSize` returns the value when its buffer is `ScaledSize(win, s)` and otherwise resizes it to exactly that size.
For it to resize only results computed at another scale (a sheet, evaluated at its `AnalysisScale` 1), the scale ≤ 1
cut must already produce the exact size: `cutValue` (`render.go:270-284`) takes `ScaledSize(win, s)` buffer pixels
starting at the floor of the window's corner, shifted left or up by the overflow at the buffer's edge, and records the
start with `WithOrigin`, the rule crops below scale 1 already follow (GCV-002 deviation 7). `fitScale` and its `+1`
go. A property test renders every op class at scales below, at and above 1 with windows at every edge and requires the
exact size.

### 7.11 The panic boundary

Two helpers, one for each kind of boundary:

```go
// graph.Recover converts a panic into E_INTERNAL with the stack in the hint. Use it in a deferred call on
// every goroutine goja-cv starts.
func Recover(err *error, where string)

// binding.export wraps a native function: a panic that is not a JavaScript throw becomes a thrown E_INTERNAL.
func (b *binding) export(name string, fn func(goja.FunctionCall) goja.Value) func(goja.FunctionCall) goja.Value {
    return func(c goja.FunctionCall) goja.Value {
        defer func() {
            if p := recover(); p != nil {
                if isJSThrow(p) { // a goja.Value (b.throw), *goja.Exception, or another goja-internal panic
                    panic(p)
                }
                b.throw(graph.Errorf(graph.EInternal, "%s panicked: %v", name, p).WithHint(string(debug.Stack())))
            }
        }()
        return fn(c)
    }
}
```

Sites: every function registered in `binding.object` (the `set` helper wraps them, `binding.go:106`), the docs
functions (`docs.go:14`), `native.object` in `pkg/cvsession/native.go:24-37`, the goroutines of `binding.evaluate` and
`binding.async`, the errgroup bodies in `compute` (`eval.go:361-365`) and `decorate` (`render.go:182-192`),
`Look.ResultContent` and the inline content provider. `safeRun` stays as the innermost boundary around `Run`. The
test is a deliberate panic at each kind of site (§14.1), including the GCV-001 TIFF fixture.

### 7.12 Source metadata

The ten hidden arguments of `open` and the four of the placeholder become typed metadata carried by `Args.meta`:

```go
// SourceMeta is what Go knows about an opened file that the agent did not write.
type SourceMeta struct {
    Rel, Stamp string      // workspace-relative path; size and modification time
    Info       buffer.Info // W, H, Format, Bits, Channels, Sample, Encoding, Origin, Notes
}

// PlaceholderMeta gives a builder lambda's placeholder the statics of the receiver it stands for.
type PlaceholderMeta struct {
    For      NodeID
    Frame    Frame
    Enc      buffer.Encoding
    Space    buffer.Space
    Channels int
}

func Meta[T any](a Args) T // panics when the node has no metadata of type T
func (b *Builder) InternSource(op *OpDef, raw []any, meta any) (*Node, []Warning, error)
```

`hashNode` hashes the metadata's JSON after the canonical arguments, so the node still changes when the file
changes. `openFrame`, `sourceEnc`, `SourceInfo` and the summary read `Meta[SourceMeta](a)`. The `trusted` parameter of
`Intern`, `Call` and `bind`, the `Hidden` flag of `ArgSpec` and its checks in `checkKeys`, `signatureHint` and help go.

### 7.13 Names agents typed

The tables shrink to one `MethodRedirects` map and a few per-argument `Redirects`, all from
`scripts/02-pi-names-typed.py`:

| typed | where | error and literal fix |
|---|---|---|
| `save`, `write`, `export`, `store`, `persist`, `download`, `toFile`, `writeTiff` | a method on a node | `E_UNKNOWN_METHOD`, fix `await cv.save("x.tif", node)` |
| `zoom` | a method on a node | fix `node.crop([x, y, w, h]).look({ zoom: 8 })` |
| `evaluate` | `cv.evaluate` | fix `await node` |
| `min` | option of `mask.luminance` | fix `img.mask.luminance({ above: 0.99 })` |
| `scale`, `box`, `crop` | options of `look` | fixes `look({ zoom: 8 })`, `node.crop([x, y, w, h]).look()` |
| `maxArea` | option of `components` | fix `(await m.components()).filter(r => r.area <= 200)` |
| `ringRadii` | option of `measure` | fix `img.measure(items, { ring: [6, 12] })` |
| `Mask`, `Mask.coverage`, `Regions.filter` | `cv.help(name)` | `found: false` with "a mask is a one-channel Image" or "await regions: an array" |

Edit distance (`closest`, `suggest.go:137-153`) and the namespace rule (`img.luminance` → `img.mask.luminance`,
`suggest.go:170-179`) stay: they cost no table. Deleted: `Synonyms`, `OptionSynonyms`, every other `ValueSynonyms`
and `Redirects` map (`lookRedirects` keeps `box`, `crop`, `scale`), `blendRedirects`, `statSynonyms`, `asSynonyms`,
`SpaceValueSynonyms`, `EncodingValueSynonyms`, and the remaining `MethodRedirects` entries.

### 7.14 The smoke test

The smoke target (`Makefile:87-102`) asserts a hard-coded file name (`look-1-levels.png`). It reads the files list the
CLI reports instead:

```make
	./dist/$(BINARY) eval --root $(SMOKE_DIR) --image $(SMOKE_DIR)/gradient-64x48-8bit.png --out $(SMOKE_DIR)/looks --json \
		-e 'const s = await img.stats(); return { meanL: s.meanL, fixed: img.levels({ black: s.p01, white: s.p99 }).look("levels") }' \
		> $(SMOKE_DIR)/eval.json
	jq -e '.status == "ok" and (.files | length) == 1' $(SMOKE_DIR)/eval.json >/dev/null
	test -s "$$(jq -r '.files[0].path' $(SMOKE_DIR)/eval.json)"
```

`jq` is present on the CI runners and on the development machines; if that changes, a ten-line
`internal/smokecheck` program does the same.
