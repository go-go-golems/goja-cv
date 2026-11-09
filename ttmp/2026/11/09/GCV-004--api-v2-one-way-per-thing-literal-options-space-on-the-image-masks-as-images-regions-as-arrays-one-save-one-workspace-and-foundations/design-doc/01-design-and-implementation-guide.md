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
      Note: evaluator; params leave the signature, await evaluates at scale 1, recover on every goroutine
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
process — the MCP server with it. A scratch program (`scripts/03-goja-panic-and-spread.go.txt`) confirms both halves: a native function that panics with
`vm.NewTypeError(…)` is caught by a JavaScript `try`, while one that indexes past a slice is not caught by `try` and
escapes `RunString` as a Go panic.

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
- There is no `{ space }` option. `img.gaussian(2, { space: "linear" })` is `E_UNKNOWN_OPTION` with the list of valid
  options, like any unknown key; the docs teach `img.as("linear").gaussian(2)`. No migration hint is kept for it
  (an agent only types `space` if a document told it to, and none will).

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
  eval (looks found in the returned value first, then the others in creation order), up to the budget of four, also
  when the eval throws later. Returning a look puts its placeholder in the result; not returning it still delivers it.
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
 graph.Evaluator   scale 1 for data; stamp (frame, enc); recover on every goroutine
 graph.Render      one exit: image result == ScaledSize(window, scale)
        │
        ▼
 dataout           cv.save: image writers | JSON (encoding/json, Record.MarshalJSON) | CSV (rows only); plain(): NaN → null
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

One case needs care. A derived image's `Space` is `linear` (`DerivedEnc`, `registry.go:134-137`), so a following op
whose receiver is derived converts a photographic second input to linear: `d.add(img)` adds `img`'s linear values,
while `img.add(d)` works in file units. The receiver decides, as everywhere else; the arithmetic docs say to put the
photograph first.

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
`look` (Value only), `toString`, `toJSON`, `Symbol.toStringTag`, the identity fields and the probe names (`prelude.js:43`); every other string-keyed property, and
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
| `min` | option of `mask.luminance` | fix `img.mask.luminance({ above: 0.99 })` |
| `scale`, `box`, `crop` | options of `look` | fixes `look({ zoom: 8 })`, `node.crop([x, y, w, h]).look()` |
| `maxArea` | option of `components` | fix `(await m.components()).filter(r => r.area <= 200)` |
| `ringRadii` | option of `measure` | fix `img.measure(items, { ring: [6, 12] })` |
| `Mask.coverage`, `Regions.filter` | `cv.help(name)` | `found: false`, with the concept entries `mask` (a one-channel image) and `regions` (awaited: an array) as `didYouMean`; these are documentation entries, not aliases |

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

## 8. Decision records

Each record states the context, the options considered, the decision, its rationale and its consequences.

### D1. Options are literals; data dependencies go through `await`

- **Context:** Params and value lifting let an option be a node. They cost a store, a kind, a lifting path in the
  binder, a substitution and re-check at evaluation, eleven Value ops, derived-field proxies, SPACE warnings and an
  opt-in `Static` flag; the review's `encoding` bug lived in that path. Agents used params 0 times and lifting 4 times.
- **Options considered:** (a) keep lifting and make every argument static unless marked liftable; (b) keep params
  only, as a typed source op; (c) literal options, with `await` for data dependencies.
- **Decision:** (c).
- **Rationale:** (a) keeps the machinery for four uses and still needs two accessor families. (b) keeps a feature no
  agent used. With (c) every option is known at build time, so every static is computable, every accessor can be
  strict, and no evaluation-time re-check exists. The cost, an `await` before a dependent call, is what agents already
  write in 1139 places.
- **Consequences:** `P`, `Param`, the store, lifting, `InputRef` for values, `Static`, the Value ops and SPACE go
  (§9). The speck view needs an internal `levelsAt` op (§7.4).
- **Status:** accepted.

### D2. Space is a property of the image; images open in file units

- **Context:** GCV-001 decided that "encoding is a property of the data, space a property of the op" and that
  `levels` and `stats` default to linear (GCV-001 guide §8). Agents then wrote 198 `{ space }` options, 80 of them to
  get `stats` in file units, and every explicit `gaussian` space was `encoded`. Agents think in the numbers a look
  shows them.
- **Options considered:** (a) keep the per-op option with all defaults switched to encoded; (b) a session-wide space;
  (c) the space of the image, chosen with `as`, file units at `cv.open`, physical ops converting internally.
- **Decision:** (c). This reverses the two GCV-001 decisions named above.
- **Rationale:** GCV-001's reasoning was that the right space depends on what an op does. That remains true for a
  few physical ops, which keep it internally (D3). For everything else, the evidence says the right space is the one
  the agent is looking at, and that it is chosen once per image, not per call: the 80 `stats` overrides were on the
  same images as the `levels` and `gaussian` overrides. (a) still makes every call carry an option and keeps a
  per-op table to remember. (b) breaks when two images in a session need different spaces (the GCV-001 v tile and
  its reference). (c) puts the choice where it is made: `img.as("linear")` once, and every op downstream follows.
- **Consequences:** Defaults change for `stats`, `levels`, `gaussian`, `blend`, `within` and the measurements on
  srgb files (encoded instead of linear); four golden looks move (§14.3); the DUST-002 helpers must add
  `as("linear")` to keep density units (§16). The `space` option and its synonyms go.
- **Status:** accepted.

### D3. Physical ops bracket themselves; gaussian and blend follow

- **Context:** With D2, each op must either follow its receiver's space or require one.
- **Options considered:** (a) every op follows, including exposure; (b) a list of physical ops that convert to linear
  and back; (c) physical ops that return linear results.
- **Decision:** (b) with `exposure`, `heal`, `inpaint`, `toGray("rec709")`, `toLab`, `encode` physical; `gaussian`,
  `blend` and `within` follow.
- **Rationale:** `exposure` means stops of light, Rec. 709 luminance and Lab are defined on linear light, and seam
  correction in `heal`/`inpaint` is additive, which is right for density and light, not for sRGB codes. For these a
  following version would compute something with a different meaning under the same name. `gaussian` is a smoothing
  step in analysis chains (DoG, thresholds) where agents asked for encoded values every time they chose; `blend` was
  overridden to encoded 7 of 8 times. (c) would make `img.exposure(1).stats()` silently report linear numbers,
  breaking the output-space rule. Agents asked for encoded `exposure` 11 of 17 times: they wanted a gain on file
  values, which is `mul(2 ** ev)`; the op's documentation says so.
- **Consequences:** The builder inserts a closing `as` after physical ops on encoded receivers (§7.4); captions hide it.
- **Status:** accepted; open question 1 in §17 asks the reviewer to confirm `gaussian`.

### D4. A mask is a one-channel image; the channel count is a static

- **Context:** The Mask kind duplicates ops (`crop`, `overlay`) and hides image ops from masks (`mask.stats`), which
  agents tripped over. Its one benefit is the build-time check that an argument is a mask.
- **Options considered:** (a) keep the kind and register every image op on it too; (b) merge the kind and check
  "one channel" when the op runs; (c) merge the kind and make the channel count a node static.
- **Decision:** (c).
- **Rationale:** (a) doubles the registry. (b) moves an error from build time, where it has the agent's line and a
  literal fix, to evaluation. (c) keeps the check and generalizes it: the same static moves `channel("g")` on a grey
  image and `colormap` on colour from evaluation to build time (GCV-001 deviation 15) and lets the grey-input rule
  skip `toGray` on grey images (GCV-001 deviation 2).
- **Consequences:** Every op that changes the channel count declares it (§7.5), checked by a property test.
- **Status:** accepted.

### D5. Regions nodes have no methods; region consumers are image methods

- **Context:** Awaited regions become arrays (the brief). If Regions nodes kept methods (`toMask`, `measure`), an
  agent who filtered an array would find that `array.toMask()` does not exist: a new class of "works on the node,
  fails on the array" errors.
- **Options considered:** (a) keep methods on nodes and attach the same methods to awaited arrays; (b) a JavaScript
  `Array` subclass with methods; (c) no methods on Regions nodes; every consumer is an image method or a `cv` function
  that takes items.
- **Decision:** (c), with `img.measure(items)` and `img.mask.regions(items)` replacing `regions.measure(img)` and
  `regions.toMask()`.
- **Rationale:** (a) fails as soon as `filter` returns a new array. (b) is lost the same way (`filter` on a subclass
  returns the subclass in modern engines, but `map`, spread and `JSON` round trips do not, and agents build arrays
  with all of them). (c) makes nodes and arrays interchangeable by construction: there is nothing a node can do that
  an array cannot. It also removes the last reverse aliases.
- **Consequences:** Three renamed calls (§6.6); the docs teach "regions are arguments".
- **Status:** accepted.

### D6. The item link is an enumerable symbol property; arrays become internal `items` nodes

- **Context:** An array passed back to an op must recover each item's exact pixels.
- **Options considered:** (a) a `WeakMap` from item objects to nodes; (b) a non-enumerable property; (c) an
  enumerable symbol-keyed property; (d) ids only, resolved against "the last awaited set".
- **Decision:** (c), with arrays interned as an internal `items` op over at most one source node.
- **Rationale:** (a) and (b) are lost when an agent spreads an item into a new object (`{ ...r, note }`), a common
  pattern. Object spread copies enumerable symbol properties, and JSON, `Object.keys` and the YAML renderer ignore
  them, so (c) survives spreads without showing up anywhere. (d) is ambiguous across evals. Restricting a list to one
  source node keeps one raster per `RegionsValue`; mixing sets is rare and has a literal fix (two calls).
- **Consequences:** goja (the version in `go.mod`) copies symbol-keyed properties in object spread and leaves them out of
  `Object.keys` and `JSON.stringify`: `scripts/03-goja-panic-and-spread.go.txt` printed `7:1:{"a":1}` for `c[s] + ":" +
  Object.keys(c).length + ":" + JSON.stringify(c)` with `c = { ...o }`, `o = { a: 1, [s]: 7 }`. A Phase 5 test keeps
  that true across goja upgrades.
- **Status:** accepted.

### D7. One `measure` for items and points

- **Context:** `img.measure({ points })` (rows) and `regions.measure(img, …)` (regions with fields) share a kernel
  (`kernels.ProbePoint`) and field names, but differ in receiver, option names (`ringRadii` versus `ring: [r1, r2]`)
  and result kind; agents typed `ringRadii` on the wrong one.
- **Options considered:** (a) keep both, moving `regions.measure` to an image receiver under a new name; (b) one op
  that takes items, with points as literal items.
- **Decision:** (b), returning Regions, `ring` defaulting to `true`.
- **Rationale:** Points are items with a centre. One op, one option set, one result shape; the awaited result is an
  array either way.
- **Consequences:** Point results gain `id` and become a Regions node; `sample` and `profile` stay Value ops (they
  are not about things in the image).
- **Status:** accepted.

### D8. `await` is the only evaluation, at scale 1

- **Context:** `await` evaluates at the preview scale; 77 of 79 `cv.evaluate` calls in agent code asked for
  `"full"`.
- **Options considered:** (a) keep the preview scale for `await` and an option to change it; (b) `await` at scale 1.
- **Decision:** (b).
- **Rationale:** Agents want exact numbers, and an option on `await` cannot exist. Previews still serve looks, which
  have their own scale. Region ops already evaluate at scale 1.
- **Consequences:** Awaits on very large scans cost more (§17, risk 4); `scaleArg`, `PreviewLong` and `PreviewScale` go.
- **Status:** accepted.

### D9. `look()` is the display call, and it queues

- **Context:** `x.look()` (167 uses) delivers only if returned; `show()` (87) queues and survives a later throw.
- **Options considered:** (a) keep `show` only; (b) keep `look` only, delivered only when returned; (c) keep `look`
  only, and every look created in the eval is queued.
- **Decision:** (c).
- **Rationale:** `look` is the more used and the discoverable one (a method on the node, listed by help). Queuing
  keeps the one property `show` had (a look survives a later error) and removes a silent failure of (b): a look built
  but not returned. Budgets already handle too many looks with a note.
- **Consequences:** Delivery order is: returned looks first, then the others in creation order. `show` and its
  plumbing go.
- **Status:** accepted.

### D10. `cv.save(path, x)` is the one output function

- **Context:** Three functions (`node.save`, `cv.json`, `cv.csv`), Data handles, content names, a naming rule and the
  CLI's suffix scheme produced a review bug and 10 + 11 uses.
- **Options considered:** (a) keep Data handles as content and add `cv.save` for files; (b) one function writing a
  file whose format comes from the extension, with duplicates refused.
- **Decision:** (b).
- **Rationale:** A file path is a name the agent already chose; refusing a second save of the same path in one eval
  makes collisions impossible instead of renaming around them. Data in the conversation is what the eval returns,
  which the YAML already carries.
- **Consequences:** No data content in normal sessions; in read-only sessions `cv.save` delivers inline (D11).
- **Status:** accepted.

### D11. One workspace: `os.Root`, the current directory by default, writable; `--read-only` delivers inline

- **Context:** Separate read roots and an out-root produced 23 `E_ACCESS` errors and a home-made containment check
  with a P1 escape.
- **Options considered:** (a) keep two roots but resolve both the same way; (b) one root, read-only by default,
  writable with a flag; (c) one root, writable by default, `--read-only` to deliver saves inline instead.
- **Decision:** (c), defaulting to the current directory for `eval`, `run` and `mcp`.
- **Rationale:** (a) keeps the reopen failure whenever the two differ. (b) makes the common case (save, then reopen)
  need a flag. With (c) the harness, Claude Code and the CLI all work without flags; a person who must protect a
  directory passes `--read-only`, and the session can never overwrite an image it opened. `os.Root` does the
  containment in the standard library, including symlinks.
- **Consequences:** The CLI can no longer read files outside the workspace (scripts pass `--root`, §16); `--out-root`
  goes.
- **Status:** accepted.

### D12. Keep only names agents typed

- **Context:** 246 synonym and redirect keys; 14 were ever typed.
- **Options considered:** (a) keep the tables; (b) keep typed names with literal fixes, plus edit distance.
- **Decision:** (b).
- **Rationale:** Untyped synonyms are untested guesses that each need maintenance when names change, and most point
  at options or ops this ticket deletes. Edit distance covers typos at no cost.
- **Consequences:** §7.13 lists what stays. New typed names come from future pi runs with the same script.
- **Status:** accepted.

### D13. One error for lazy data: `E_NOT_AWAITED`

- **Context:** With lifting gone, reading a field of a lazy Value node has no meaning; today it builds a node.
  `E_LAZY_COMPARE`, `E_NOT_ARRAY` and the collision hints each cover one way of using a lazy node as data.
- **Options considered:** (a) let JavaScript return `undefined`; (b) one error for every property read and primitive
  conversion on a Value or Regions node.
- **Decision:** (b).
- **Rationale:** (a) is the silent zero-value class again, in JavaScript. One rule, one code, one fix pattern.
- **Consequences:** `E_LAZY_COMPARE` and `E_NOT_ARRAY` disappear as codes; the docs' error table changes (§11).
- **Status:** accepted.

### D14. Foundations: loud accessors, ordered node lists, one Render exit, `encoding/json`, a panic boundary, typed metadata

- **Context:** The review classes of §3.1.
- **Options considered:** for each, fixing instances (done in the review commits) versus changing the mechanism.
- **Decision:** change each mechanism as in §7.2, 7.3, 7.9–7.12.
- **Rationale:** Each instance fix added code that the mechanism change deletes (§9), and each mechanism change makes
  the class unwritable rather than unlikely.
- **Consequences:** Build-time function signatures change in every op file (mechanical); `ProbeFile` and
  `DecodeFile` take files.
- **Status:** accepted.

## 9. Legacy and complexity to delete

The checklist is ordered by phase (§13). Each item is deleted outright; nothing is kept for compatibility. Line
counts are of non-test code on this branch, measured or estimated from the cited ranges.

| # | delete | where | lines |
|---|---|---|---|
| 1 | the param store, `param` op, `KindParam`, `P`, param binding functions, params in signatures and the context block | `pkg/cv/graph/params.go` (180), `source.go:140-151`, `registry.go:23,33-40,406-417,448-462`, `engine.go:186-198`, `eval.go:33,173,293-296`, `prelude.js:196-208,313-330`, `binding.go:503-603`, `processor.go:512-528`, `init.js` (`P`) | ~380 |
| 2 | lifting, `InputRef` for values, `liftIdx`, `Static`, `resolveArgs` re-check, `Expander.Arg`, Value ops, derived fields, SPACE warnings | `args.go:356-377,436-439,1138-1213`, `macro.go:38-64`, `pkg/cv/ops/value.go` (149), `prelude.js:34-39,265`, `pkg/gojamod/cv/fields.go` (62), `space.go:153-211`, `funcs.go:146-151` | ~460 |
| 3 | the space option and its rule fields | `registry.go:69-90,145-161,370-375`, `space.go:31-45`, `views.go:141-156`, `args.go:1131-1134`, `reference.go:110-113`, `SpaceArg` in every view expansion | ~120 |
| 4 | the Mask kind and its duplicate ops | `registry.go:20`, `value.go:52-61`, `geometry.go` (second registration), `mask.go:377-…`, `present.go:138-147`, `args.go:392-402`, look and help branches | ~80 |
| 5 | region queries and their machinery: `where`, `sortBy`, `top`, `only`, `without`, `count`, `filter`, `truthy`, `Produces`, `producerHint`, `UnknownFieldError`, `RemovedFields`, `MaxItems`, `itemsBound`, data lambdas, `ITEM_PROTO`, `E_NOT_ARRAY` methods, collisions | `pkg/cv/ops/regions.go:37-231,272-436`, `schema.go:43-133`, `regionmeasure.go:109-122`, `pkg/cv/ops/schema.go:35-…`, `registry.go:267-270,289-292`, `sheet.go:54-72`, `args.go:17-28`, `eval.go:63-109`, `binding.go:26-102`, `prelude.js:55-70,88-116`, `evaltrack.go:300-331`, `processor.go:449-489` | ~720 |
| 6 | `cv.evaluate`, preview scale, `show`, `cv.describe`, `explain` op, `cv.isNode`, `cv.warnings`, `cv.kinds`, `doc` stub | `prelude.js:355-378`, `binding.go:330-351,417-434,486-501`, `engine.go:200-214`, `init.js:71-90`, `native.go:100-121`, `processor.go:86-89,106-109`, `pkg/cv/ops/meta.go`, `funcs.go:56-83,114-119` | ~190 |
| 7 | Data handles, the hand-written JSON writer, `table`, `columnArrays`, data names, `uniqueName`, data captions | `pkg/cv/dataout/data.go` (518 → ~140), `pkg/gojamod/cv/dataout.go` (236 → ~110), `cmd/goja-cv/eval.go:195-287`, `suggest.go:96-104` | ~565 |
| 8 | home-made containment and the two roots | `pkg/cv/graph/paths.go` (102), `native.go:65-84`, `session.go:61-66,119-135`, `mcp.go:81-82`, `eval.go:35,302-311` | ~145 |
| 9 | synonym and redirect tables except §7.13 | `suggest.go:10-86,90-105`, `args.go:812-862`, `registry.go:154-161`, `views.go:262-266`, `regionmeasure.go:31-33`, `names.go:66`, `composite.go` (`blendRedirects`), `funcs.go:21-27` (two of three) | ~200 |
| 10 | `fitScale` and its tolerance | `render.go:249-268` | ~15 |
| 11 | hidden arguments, `trusted`, `Hidden` | `source.go:108-117,154-159`, `engine.go:125-137`, `args.go:1022-1024`, `graphtest/source.go` | ~40 |

**Estimate.** About 2,900 lines of non-test Go and JavaScript are deleted. About 800 are added: the workspace (~90),
the `items` op and `Region.Shape` (~150), channel counts (~100 with the per-op declarations), the space rule and
brackets (~50), `cv.save` with inline delivery (~150), `cv.explain` (~30), `E_NOT_AWAITED` (~25), `levelsAt` (~40),
`Record.MarshalJSON` and `plain` (~40), the panic boundary (~50), typed metadata (~40), loud accessors (~40). The net
change is about −2,100 of 23,154 non-test lines (−9 %). Tests move similarly: the param, query-op, Data-handle and
containment tests (roughly 1,200 lines) go; the new property and regression tests (§14) add about 900. The docs lose
`07-params.md` and most of `04-lambdas.md`. Treat these numbers as ±25 %; the implementation diary should record the
measured totals (`git diff --stat` per phase).

**Edge-case code from the review fixes that goes.** The symlink depth guard and `resolveExisting` (`paths.go:24-52`,
from `319a6fc`), `uniqueName` (`eval.go:255-265`, `319a6fc`), the schema-less branch of `columnArrays`
(`data.go:350-363`, `319a6fc`), `fitScale`'s `+1` (`render.go:257`, `cd5f46c`), `listInputs` (`present.go:107-119`,
`cd5f46c`), and the two `Static: true` markings of `4c79173` (`source.go:102`, `channel.go`). The TIFF format-error
check of `4c79173` stays: it is a correct decoder fix, and the panic boundary is the second line of defence.

## 10. Pseudocode for the key flows

### 10.1 Building a node

```
Builder.call(op, receiver, raw, nested):
    recv := node(receiver)                                   // nil for module ops and sources
    check recv.Kind == op.On, and recv.Channels == 1 if op.OneChannel      // E_WRONG_KIND with fix .toGray()
    raw := resolveBuilt(raw, receiver)                       // builder lambdas → node arguments
    bd  := bind(op, raw)                                     // literals; node args appended to inputs in order;
                                                             // items lists interned as "items" nodes
    a   := Args{op, bd.vals, meta: nil, ins: [recv] + bd.inputs}
    op.Validate(a)                                           // panics are programming errors → E_INTERNAL
    checkPicture, checkInFrame                               // unchanged
    if op.Expand: return expand(op, a)                       // macros: unchanged apart from the space plumbing
    ins, ws := resolveSpace(op, a)                           // §7.4: Gray rule, conversions to the work space
    n := internNode(op, a with ins)                          // Frame, Enc, Space, Channels, Fields
    if op.Space == Physical and n is an Image and !Identity(n.Enc, linear, recv.Space):
        n = internNode(as, n, recv.Space)                    // the bracket's closing conversion
    return n, ws
```

### 10.2 Awaiting

```
JS:   await node                      → proxy.then → native.__evaluate(id)
Go:   binding.evaluate(id):           (owner goroutine, inside export())
          promise := newPromise()
          go func() {                  (evaluation goroutine)
              defer graph.Recover(&err, "await")
              v, err := eng.Evaluate(ctx, id, EvalOptions{Scale: 1, OnWarn: …})
              post to owner: settle(promise, resultToJS(v))
          }()
      resultToJS:  Image   → summary from statics + source metadata
                   Value   → data in schema order (Records)
                   Regions → []Record in list order
JS:   then(v => kind === "Regions" ? linkItems(v, node) : v)
```

JavaScript receives NaN as NaN (it is a JavaScript number); `plain()` applies only when data is encoded for a file
or for inline content.

### 10.3 Passing an array back

```
JS:   img.overlay(top)                → marshal(top) → [{x, y, …, __cvref: [17, 3]}, …]
Go:   fromJS: lists and maps as plain data (the __cvref key stays a key)
      bind, argument "target" of type "items|Mask":
          list → items(list):
              refs   := [(17, 3), (17, 9), …]    literal := []
              sources := {17}                      // > 1 → E_ARG_TYPE
              entries := [{id: 3}, {id: 9}, …]     // array order
              m := internNode(items, inputs [node 17], args {entries})
          → InputRef to m
Eval: items.Run: src := value(17); out := [src.byID[3], src.byID[9], …]; RegionsValue over src's raster
```

### 10.4 Saving

```
JS:   await cv.save("views/speck.tif", img.view("speck"))
Go:   binding.save(path, x, opts):                 (owner goroutine)
          rel := ws.Rel(path)                       // E_ACCESS on escape
          tracker.saved[rel] exists → E_DUPLICATE
          rel ∈ opened sources       → E_ACCESS
          kind := format(ext)                       // E_ARG_TYPE for an unknown extension
          data := x is a node ? x.ID : exportOrdered(x)
          go: defer Recover; bytes := encode(kind, x at scale 1 | data); ws.WriteFile(rel, bytes) | queue inline
          resolve { path: rel, … }
```

### 10.5 Resolving a workspace path

```
Workspace.Rel(p):
    if IsAbs(p):
        for base in [dir, resolved]:
            r := filepath.Rel(base, Clean(p)); if r does not start with ".." → return r
        → E_ACCESS "outside the workspace DIR"
    r := Clean(p)
    if r == ".." or r starts with "../" → E_ACCESS
    return r
Workspace.Open(p):  rel := Rel(p); f, err := root.Open(rel); err → E_ACCESS (escape) or E_ARG_TYPE (missing)
Workspace.WriteFile(p, data):
    rel := Rel(p); if readOnly → caller delivers inline
    _, statErr := root.Lstat(rel); replaced := statErr == nil
    root.MkdirAll(Dir(rel), 0o755); root.WriteFile(rel, data, 0o644)        // escapes are errors here
```

### 10.6 Two traces

**`img.exposure(1).stats()` on an srgb photograph.**

```
cv.open("photo.jpg")      open          enc srgb   space encoded  channels 3
as(linear)                as            enc srgb   space linear            (inserted: exposure is Physical)
exposure(ev 1)            exposure      enc srgb   space linear
as(encoded)               as            enc srgb   space encoded           (inserted: the bracket closes)
stats                     stats         numbers in encoded space (file units)
```

**`(await img.detect.specks()).slice(0, 5)` passed to `img.mask.regions(…, { grow: 2 })`.**

```
detect.specks             Regions  #12  (pixels at analysis scale 1)
awaited                   [ {id 1, …, [ITEM]: 12}, … ]     slice(0, 5) keeps the objects
items                     Regions  #20  inputs [#12]  entries [{id 1}, {id 4}, {id 2}, {id 7}, {id 3}]
mask.regions(grow 2)      Image    #21  channels 1, frame = img's, enc linear (derived)
```

## 11. Errors and diagnostics

| code | v2 meaning | example and fix |
|---|---|---|
| `E_ARG_TYPE` | a wrong value type; now also a node where a literal is expected, CSV of a non-list, an unknown save extension, items from two sets | `levels({ white: d.stats() })` → fix `levels({ white: (await d.stats()).p99 })` |
| `E_WRONG_KIND` | a node of the wrong kind; now also "needs a one-channel image" | `img.within(img, …)` → fix `img.toGray()` as the mask |
| `E_NOT_AWAITED` (new) | a property of, or a primitive from, a lazy Value or Regions node | `img.stats().p99` → fix `(await img.stats()).p99`; `regions.overlay(img)` → fix `img.overlay(regions)` |
| `E_DUPLICATE` (new) | the same path saved twice in one eval | fix: another path, or one save |
| `E_ACCESS` | a path outside the workspace; overwriting an opened image; a non-inline format in a read-only session | `cv.save("../x.tif", …)` → fix `cv.save("x.tif", …)` |
| `E_UNKNOWN_METHOD`, `E_NAMESPACE` | unchanged; the redirect table is §7.13 | `node.write("x")` → fix `await cv.save("x.tif", node)` |
| `E_UNKNOWN_OPTION` | unchanged; `space` is now an unknown key like any other | `mask.luminance({ min: 0.99 })` → fix `img.mask.luminance({ above: 0.99 })` (a typed name, §7.13) |
| `E_INTERNAL` | any Go panic, now at every boundary | the message names the function; the hint carries the stack |
| `E_FIELD_EXISTS`, `E_FRAME`, `E_FORMAT`, `E_TOO_MANY`, `E_ARG_RANGE`, `E_TIMEOUT`, `E_CANCELLED`, `E_EVAL`, `E_IMMUTABLE`, `E_PLACEHOLDER`, `E_NO_IMAGE` | unchanged | |
| removed | `E_LAZY_COMPARE`, `E_NOT_ARRAY`, `E_PARAM_REDECLARE`, `E_NO_DOCUMENT`, `E_NO_FIELD` (no derived fields and no removed-field accessors) | |

No error mentions a removed name (`space`, `cv.evaluate`, `where`, `P`): the brief excludes migration diagnostics, and
an agent only types a name that some document or error showed it. The typed names of §7.13 are kept because agents
reached for them without being shown them.

Warnings: `SPACE` goes (no lifting). `RANGE`, `UNUSED_NODE`, `CUT`, `ENCODING`, `CROP_FIRST`, `DECORATION_FIRST`,
`CLAMP`, `SHEET_TRUNCATED` and `OUTSIDE` stay.

## 12. Documentation to change

Every ```js block in `pkg/cv/docs/topics` and `pkg/doc` runs in the docs tests (`pkg/cvsession/docs_test.go`), so each
phase updates the blocks it breaks in the same commit. The full rewrite is Phase 9.

| document | change |
|---|---|
| `pkg/cv/docs/topics/01-getting-started.md` | the eval contract with `await` at full resolution, `look()` queues, `cv.save`, the workspace |
| `02-units-and-scales.md` | no preview scale for data; looks keep theirs |
| `03-nodes-and-evaluation.md` | literal options, data dependencies through `await`, `E_NOT_AWAITED`, `cv.explain` |
| `04-lambdas.md` | builder lambdas only; data lambdas are JavaScript on awaited arrays |
| `05-masks-and-regions.md` | rewritten: masks are one-channel images; regions are arrays; items and links; consumers |
| `06-looks.md` | `look()` is the one display call and queues; `show` gone |
| `07-params.md` | deleted |
| `08-errors.md` | the table of §11 |
| `09-recipes.md` | every recipe in v2 (filtering, measuring, saving) |
| `10-encodings-and-spaces.md` | rewritten around the space of the image, `as`, the output-space rule, physical ops |
| `11-views.md` | the speck views compute in linear light; no `space` option |
| `12-measuring.md` | `measure(items)`, measurements in the receiver's space, `as("linear")` for physical units |
| `13-data-out.md` | rewritten: `cv.save`, formats, CSV rows, rows plus metadata, the workspace, read-only inline delivery |
| `pkg/doc/02-concepts.md` | kinds (Image, Regions, Value), the space of an image, items |
| `pkg/doc/03-cli.md` | `--root`, `--read-only`, `--out`, `--image` inside the workspace; no `--out-root` |
| `pkg/doc/04-agents.md` | the eval contract summary |
| `pkg/doc/05-setup-claude-code.md`, `06-setup-codex.md`, `07-setup-pi.md` | MCP command lines with one `--root` |
| `pkg/doc/08-recipes.md` | v2 recipes |
| `pkg/doc/10-troubleshooting.md` | `E_ACCESS` (workspace), `E_NOT_AWAITED`, `E_DUPLICATE` |
| `README.md` | the example and the flag list |
| `pkg/cvsession/reference.go` | `Instructions` (§2.10) rewritten under 4000 characters: no `P`, `show`, `{ space }`, `cv.json`/`cv.csv`, `where`/`sortBy`/`top`; `EvalToolDescription`; the `Globals` lines of `Reference` |
| `cmd/goja-cv/mcp.go` | the `Long` help text and flag help |
| help (`pkg/gojamod/cv/help.go`) | the `cvFuncs` table: `save`, `explain`; no `evaluate`, `describe`, `json`, `csv`, `isNode`, `warnings`, `P`, `show`, `doc` |

## 13. Implementation phases

Every phase ends green on `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, `make lint`, `make
logcopter-check` and, from Phase 0 on, `make smoke`, and is one commit (or a few) followed by an implementation
diary step. Write the listed tests first and watch them fail. Each phase updates the docs blocks it breaks, because the
docs tests run every ```js block. The order puts foundations first (they do not change the API), then removes
lifting before touching spaces (the SPACE warning depends on lifting), merges masks before regions (overlay's target
type is `items|Mask`), and does saving after regions (saving takes awaited arrays).

**Phase 0 — foundations without API change.**
1. Tests: a test-only op whose `Validate`, `DocFrame` and `Run` panic, and a test hook that makes a binding function
   panic, each surface as `E_INTERNAL` and the session survives the next eval; the GCV-001 empty-`BitsPerSample` TIFF
   through `cv.open`; JSON round trips of strings with `\x00`, ` ` and quotes; `Record` key order through
   `json.Marshal`; NaN and ±Inf become `null`; a render property test (every op class, scales 0.3, 1, 4, windows at
   each edge) requires exactly `ScaledSize(window, scale)`; `make smoke` reads the files list.
2. Implement: `graph.Recover`, `binding.export` (wrap the `set` helper and the docs functions), `native.object`
   wrapping, recovers in the evaluation goroutines and errgroup bodies; `Record.MarshalJSON`, `dataout.plain`,
   `encodeJSON` through `json.Marshal` (delete `writeJSON`, `writeString`, `writeObject`); `Render` with one exit and
   the exact cut (delete `fitScale`); typed `SourceMeta` and `PlaceholderMeta` (delete `Hidden`, `trusted`); the smoke
   target. No golden changes.

**Phase 1 — literal options, named node arguments, loud accessors.**
1. Tests: a node passed as an option is `E_ARG_TYPE` with the `(await …)` fix (replaces `TestEncodingArgsAreStatic`);
   `cv.compare([img, img])` lays out two panels (keeps `TestOverlayAndCompare`'s case) and a sheet keeps repeated
   views; each accessor panics on a wrong type and on an unguarded optional; the registry property test calls every
   build-time function of every op with default arguments without panicking; `img.stats().p99` and `img.stats() > 1`
   throw `E_NOT_AWAITED` with the fix; `view("speck")` is bit-identical (golden `sheet-2x2` exact).
2. Implement: delete params (§9 item 1), lifting, `Static`, Value ops, derived fields, SPACE warnings and
   `Expander.Arg` (item 2); turn `OpDef.Inputs` into named node `ArgSpec`s; `Args` with inputs, values and strict
   accessors; build-time signatures `(a Args)`; delete `listInputs`, `nodeOf`, `viewNodes`, `firstImage`; add
   `levelsAt`; `__notAwaited` in the prelude for Value and Regions nodes, deleting `__lazy` and `E_LAZY_COMPARE`. Docs:
   delete `07-params.md`; update `03`, `08`, `09`.

**Phase 2 — one way to evaluate, show and explain.**
1. Tests: `await` evaluates at scale 1 on a fixture larger than 1600 px (the summary's `scale` is 1); a look that is
   created but not returned is delivered; returned looks come first; a look created before a throw is delivered; the
   fifth look gets the budget note; `cv.explain` fields; `cv.evaluate` is an unknown `cv` member like any other.
2. Implement: delete `cv.evaluate`, `scaleArg`, `PreviewLong`, `PreviewScale`; the look queue in the eval tracker;
   delete `show`, `native.queue`, `state.shows`, the `shown:` block, `FuncDef.Global`; add `cv.explain`; delete
   `cv.describe`, the `explain` op, `cv.isNode`, `cv.warnings`, `cv.kinds`, the `doc` stub and `E_NO_DOCUMENT`. Docs:
   `01`, `03`, `06`, instructions.

**Phase 3 — space on the image.**
1. Tests: `await cv.open(f)` reports `space: "encoded"`; `as` is the identity on linear-encoded and derived images;
   the output-space property test (every op on an srgb fixture in each space: result space = receiver space, except
   display and derived results); a physical bracket: `img.exposure(1)` equals today's linear exposure followed by
   encoding within 1e-6; measurements in the receiver's space (`img.stats()` equals today's `stats({ space: "encoded"
   })`); `{ space }` is `E_UNKNOWN_OPTION`; `ENCODING` still fires; the speck view is unchanged
   (`sheet-2x2` exact; gated `TestRealViewRanking` unchanged); the gated hot-list test builds its DoG and measures on
   `img.as("linear")` and keeps its thresholds.
2. Implement: the `SpaceRule` enum and the resolution of §7.4 (delete item 3 of §9); the source's encoded `Run`;
   `as` (rename `convert`); brackets; views. Regenerate the four moving goldens (`gaussian`, `levels-gamma`,
   `levels-stats`, `profile-chart`) with `-update` in this commit and record each in the diary (§14.3). Docs: `10`,
   `11`, `12`, recipes.

**Phase 4 — masks are one-channel images.**
1. Tests: the channel property test (`Buf.C == Node.Channels` for every op on a grey and a colour fixture); mask ops
   on any one-channel image (`img.toGray().grow(2)`); `img.within(img, …)` is `E_WRONG_KIND` at build time with the
   agent's line and the `toGray()` fix; `m.stats()` works; `channel("g")` on a grey image and `colormap` on colour fail
   at build time; the grey-input rule inserts no `toGray` for a grey input (`cv.explain`).
2. Implement: `Node.Channels`, `OpDef.Channels`, `OpDef.OneChannel`, the `Mask` type word in `accepts`; delete the
   Mask kind (§9 item 4); one `crop`, one `overlay`. Mask goldens stay exact.

**Phase 5 — regions as arrays.**
1. Tests: an awaited `detect.specks` is an array; items stay linked after `filter`, `sort`, `slice` and object spread
   (the goja spread check of D6); `img.overlay(array)` equals `img.overlay(node)` pixel for pixel, and a filtered
   array equals the old `only(ids)` result; literal boxes overlay; point items are refused by `overlay` with the box
   fix; items from two sets are refused; `img.measure(items)` equals the old `regions.measure(img, { ring: true })`
   fields and `img.measure(points)` the old point rows (plus `id`); `img.mask.regions(items, { grow })` equals the old
   `toMask`; a sheet over an array is sized by its length; methods on a Regions node throw `E_NOT_AWAITED` with the
   image-method hint; `E_TOO_MANY` still applies.
2. Implement: the `items` type and op, `Region.Shape`, `linkItems` and the `__cvref` marshal; `measure` on Image;
   `mask.regions`; delete §9 item 5. Goldens `specks-overlay` and `sheet-2x2` stay exact. Docs: `04`, `05`, `09`, `12`.

**Phase 6 — one save, one workspace.**
1. Tests: `Workspace.Rel` and `Open`/`WriteFile` for relative and absolute paths, `..`, `/tmp` versus `/private/tmp`,
   a symlink inside pointing inside (allowed), inside pointing outside (refused), a dangling link pointing outside
   (refused, the P1 case), a symlinked directory outside (refused); `cv.save` for every extension; `E_DUPLICATE`;
   overwriting an opened image is `E_ACCESS`; `cv.open(saved.path)` reopens; CSV refuses objects with the right
   fixes; profile and histogram `rows`; read-only inline delivery over MCP (content items) and the CLI (`--out` files
   at their relative paths); `--image` outside `--root` is a usage error; `mcp --root` once.
2. Implement: `graph.Workspace`; decoders take files; `SourceKey` by relative path; `cv.save`; profile and histogram
   rows and their charts; CSV records only; the flags of §7.8; delete §9 items 7 and 8. Chart goldens stay exact (same
   pixels from `rows`). Docs: `13`, `pkg/doc/03`–`07`, README.

**Phase 7 — names.**
1. Tests: a table-driven test over §7.13 (each typed name gives its fix); a removed synonym (`brightness`) gives an
   edit-distance suggestion or none, never a table hit.
2. Implement: delete the tables (§9 item 9); keep §7.13. Run `scripts/02-pi-names-typed.py` on the tree: only the
   kept keys remain.

**Phase 8 — documentation sweep and validation.**
1. The documents of §12, the regenerated reference, `goja-cv help` pages; the docs tests pass.
2. The pi validation runs (§15); results in the implementation diary; an "Implementation notes (deviations)"
   section appended to this guide.

The key points:

- Foundations come first because they change no behaviour an agent sees and make the later phases safer.
- Lifting goes before spaces, masks before regions, regions before saving: each phase deletes what the next would
  otherwise have to carry.
- Only Phase 3 moves goldens.

## 14. Test strategy

### 14.1 Unit and property tests

- **Registry property tests.** For every registered op: build-time functions run with default arguments; the result's
  schema matches evaluated rows (the GCV-003 test, now over records); the evaluated channel count equals
  `Node.Channels`; the output space follows the rule of §7.4.
- **Binder.** Literal-only options; node arguments in order with repetitions; items lists with linked, literal, mixed
  and invalid elements.
- **Accessors.** Each accessor panics on a wrong type and on an unguarded optional.
- **Spaces.** Brackets, `as` identities, measurements per space, the ENCODING warning.
- **Items.** Links through JavaScript transformations, including object spread in goja; `items` op order and
  repetition; literal box coverage equals `mask.box`.
- **Render.** The exact-size property over op classes, scales and window positions.
- **Data.** JSON escapes and order, NaN to null, CSV rows and refusals, profile and histogram rows.
- **Workspace.** The path cases of Phase 6 on macOS and Linux (CI runs Linux; the `/tmp` case is macOS).
- **Panic boundary.** One deliberate panic per kind of site.

### 14.2 Documentation tests

`pkg/cvsession/docs_test.go` runs every ```js block of the topics and the guidebook with the planted scan as `img`;
blocks marked `js error=CODE` must fail with that code. The new codes get examples (`E_NOT_AWAITED`,
`E_DUPLICATE`); the docs test session uses a temporary workspace so `cv.save` blocks run.

### 14.3 Golden looks

The harness (`pkg/cv/look/golden_test.go`) compares 29 looks of the planted scan exactly. Only Phase 3 may change
them, and only these four, because their ops change default space from linear to the image's encoded space:

| golden | why it moves |
|---|---|
| `gaussian.png` | `gaussian` blurs file values instead of linear light |
| `levels-gamma.png`, `levels-stats.png` | `levels` (and the `stats` that feed `levels-stats`) work in file units |
| `profile-chart.png` | `profile` samples file values |

Every other golden must stay exact through all phases, including `exposure.png` and `togray.png` (physical: same values
through the bracket, within float rounding that the 8-bit quantization removes) and `sheet-2x2.png` (the speck view
keeps linear light through `as` and `levelsAt`). The rule: regenerate with `go test ./pkg/cv/look -run
TestGoldenLooks -update` in the Phase 3 commit only, list the four files in the diary with a one-line reason each, and
treat any other diff as a bug. The GCV-001 tolerance classes are no longer needed and can be deleted with the
`-golden.tolerant` flag.

### 14.4 Regression tests for the review classes

| review test | becomes |
|---|---|
| `TestEncodingArgsAreStatic` | "a node as an option is `E_ARG_TYPE`" (Phase 1) |
| `TestTIFFEmptyBitsPerSample` | kept; plus the panic-boundary test (Phase 0) |
| `TestOverlayAndCompare` (`[img, img]`) | kept, reading `Args.Inputs` (Phase 1) |
| `TestSheetCellsLabelsEdgesAndTruncation` (size below 1) | the render size property (Phase 0) |
| `TestOutPathSymlinks` | `TestWorkspaceSymlinks` (Phase 6) |
| `TestDataContentLazyEagerAndColumns` | `cv.save` CSV and JSON tests (Phase 6) |
| `TestUniqueName` | `E_DUPLICATE` (Phase 6) |

### 14.5 Real data (gated)

`TestRealViewRanking` (142 gold points, 15 tiles) must give the same numbers (median 18, 79.6 % in the top 50): the
speck view still computes in linear light. The hot-list test and benchmark (GCV-003 deviations 19–20) switch to
`img.as("linear")` explicitly and keep their thresholds. Both stay gated on the environment variables of GCV-002/003.

### 14.6 Smoke

`make smoke` (§7.14) runs in CI on every push (`.github/workflows/push.yml:40-51`).

## 15. Validation plan: pi runs before and after

The harness is in the dust-scanning repository:
`/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/scripts/06-pi-cv-agent/run.sh`
(`[IMAGE=…] [IMAGE_EXTRA=…] [QUESTIONS=…] [OUT_ROOT=1] run.sh GOJA_CV_DIR OUT_DIR [MODEL]`), with question files
`questions-gcv-001.md`, `questions-gcv-002.md`, `questions-gcv-003.md` beside it and results under `various/pi-runs`.
It always starts the server with `--root OUT_DIR/img`, which in v2 is the workspace; `OUT_ROOT=1` adds `--out-root`,
which v2 does not have, so after-runs must not set it (saved files land in `OUT_DIR/img`).

**Runs.** "Before" is the pre-GCV-004 head (`454a836`, GCV-001..003 with the review fixes); "after" is the GCV-004
head after Phase 8. Two runs per question file and build, twelve runs in all:

```bash
H=~/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations
R=$H/various/pi-runs; Q=$H/scripts/06-pi-cv-agent
git -C ~/code/wesen/go-go-golems/goja-cv worktree add /tmp/goja-cv-before 454a836
TILE=~/dustdet/cv_tiles/7edbfe9c3e7ab7d6_512_1024.tif
# GCV-001 questions: the v tile (density stored linearly) and the original tile as its reference, as for runs 09–10
mkdir -p /tmp/v && cp ~/dustdet/cv_tiles/1d955ef4b27d3d88_1536_1536.tif /tmp/v/tile_ref.tif
python3 ~/code/wesen/go-go-golems/goja-cv/ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/scripts/01-make-v-tile.py \
  /tmp/v/tile_ref.tif /tmp/v/tile_v.tif            # also prints the expected answers
VTILE=/tmp/v/tile_v.tif REFTILE=/tmp/v/tile_ref.tif
IMAGE=$VTILE IMAGE_EXTRA=$REFTILE QUESTIONS=$Q/questions-gcv-001.md $Q/run.sh /tmp/goja-cv-before $R/run-19-v2-before-gcv001
IMAGE=$TILE QUESTIONS=$Q/questions-gcv-002.md $Q/run.sh /tmp/goja-cv-before $R/run-21-v2-before-gcv002
IMAGE=$TILE QUESTIONS=$Q/questions-gcv-003.md OUT_ROOT=1 $Q/run.sh /tmp/goja-cv-before $R/run-23-v2-before-gcv003
# second before-run of each: run-20, run-22, run-24
# after: the same commands with GOJA_CV_DIR = the GCV-004 checkout, runs 25–30, and never OUT_ROOT
```

**Metrics.** From `07-pi-events-summary.py` and this ticket's `01-pi-api-usage.py` and `02-pi-names-typed.py` on the
new run directories:

| metric | before (expected from runs 09–18) | target after |
|---|---|---|
| calls per run (median per question file) | GCV-001 ≈ 25, GCV-002 ≈ 34, GCV-003 ≈ 50 | not more than before |
| results with `isError` | 1–7 per run | not more than before |
| `E_ACCESS` | 1–4 per run in runs 11–18 | 0 |
| `E_UNKNOWN_METHOD` | ≈ 2 per run | ≤ 1 per run |
| `E_WRONG_KIND` | occasional | 0 |
| `E_NOT_AWAITED` (new) versus `E_LAZY_COMPARE` | 7 in 18 runs | ≤ 1 per run |
| `{ space }` / `as(` | 198 / 0 | 0 / reported |
| answers | GCV-001 Q1 p01 0.00854, p99 0.91794 (file units); Q2 0.28713; Q3 neutral; GCV-002 ≥ 9 / 15 gold points; GCV-003 CSV and JSON rows equal the candidate count, the TIFF reopens with frame [0, 0, 1024, 1024] | unchanged |

GCV-001 Q1 is the direct test of D2: with file units as the default, `await img.stats()` answers it in one call.
Add the twelve rows to the pi-runs README in the dust-scanning repository (the coordinator's step; this guide does
not edit that repository).

## 16. Migration note: the DUST-002 helpers

The colleague's helpers live in another repository and belong to the user:
`/Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/`.
This ticket does not edit them. What breaks, and the v2 replacement:

| file and use | breaks because | v2 |
|---|---|---|
| `scripts/84-cv-helpers.js` throughout: "goja-cv's linear value is density … so blurs and stats are in density units" | images open in file units (sRGB_encode(density) for these tiles) | start with `const lin = img.as("linear")` and build `views`, `probe`, `_hotRegions` on `lin` |
| `84`: `reg.items.slice(0, max)`, `reg.count` | awaited regions are arrays | `reg.slice(0, max)`, `reg.length` |
| `84`: `s.centroid.map(Math.round)` | already removed in GCV-003 | `[Math.round(s.cx), Math.round(s.cy)]` |
| `84`: `img.detect.specks(…).sortBy("contrast").top(n)` | query ops removed | `(await img.detect.specks(…)).sort((a, b) => b.contrast - a.contrast).slice(0, n)` |
| `84`: `mem.probe` with two `stats({ mask })` calls | still works (masks are images), but in file units | `lin.measure([[x, y]], { coreRadius: r, ring: [r + 3, r + 9] })` gives step, spread and snr in one call |
| `84`: `v.scene.overlay(await mem._hotRegions(k), …)` | unchanged: arrays are accepted | none |
| `scripts/83-cv-specks-vs-gold.py`: `all.where({ id: { gte, lt } })`, `all.count()`, `r.items`, `x.centroid`, paging by 40 | query ops removed; arrays; GCV-003 names | `await cv.save("specks.csv", img.detect.specks(…))` and read the CSV: no paging |
| `83`, `85`: `goja-cv eval --image /abs/path.tif` and `cv.open(TIF)` with absolute tile paths | the workspace defaults to the working directory | pass `--root ~/dustdet` (or run from there) |
| `85`: `cv.open(TIF, { encoding: "linear" })` and the `gdog_manual` recipe | unchanged numbers: on a linear-encoded image file units are linear | none |
| `85`: reading `look-*` files from `--out` | unchanged naming | none; the `--json` `files` list is the robust way |
| `reference/04-goja-cv-dust-playbook.md`, `reference/08-…getting-started…` | code snippets use v1 calls (`where`, `sortBy`, `top`, `show`, `{ space }`, `cv.csv`) | rewrite the snippets against §6.9 |

## 17. Risks, alternatives and open questions

**Risks.**

1. *Default space change.* Every srgb workflow's numbers change for `stats`, `levels`, `gaussian`, `blend` and the
   measurements. Mitigation: the output-space rule is one paragraph in the docs; `as("linear")` restores physical
   units; the DUST-002 note lists each place.
2. *goja and symbol spread.* The item link relies on object spread copying symbol-keyed properties, which the current
   goja does (D6). A goja upgrade that broke it would turn spread items into literal boxes; the Phase 5 test catches it.
3. *Synonym pruning on one model.* The 14 typed names come from one model's 751 calls. Another model may type other
   names. Mitigation: edit distance stays, and the script makes adding a typed name a one-line change backed by
   evidence.
4. *Full-resolution awaits.* On 7000 px scans an `await` that used to run at about scale 0.23 now runs at 1 (about 19
   times the pixels). Mitigation: crop first; looks keep previews. If this hurts in practice, an image summary could
   come from statics without evaluation (open question 4).
5. *Workspace default.* Writing into the current directory by default lets an agent create files where the server
   was started. Mitigation: `--read-only`; sources are never overwritten.
6. *Channel declarations.* A missed `Channels` declaration gives a wrong build-time mask check. Mitigation: the
   property test evaluates every op.

**Alternatives not taken** are in the decision records (§8): keeping lifting with static-by-default arguments,
per-op spaces with encoded defaults, a Mask kind with copied methods, methods on Regions nodes, an array subclass,
two measure ops, a preview scale for `await`, `show` as the display call, Data handles beside `cv.save`, two roots.

**Open questions for review.**

1. Should `gaussian` be physical (optical blur) rather than following? The evidence (10 of 10 explicit spaces
   encoded) says follow; image-processing convention for resampling says linear.
2. Should `blend` and `within` be physical? Compositing light is linear; agents chose encoded 7 of 8 times.
3. Is `ring: true` the right default for `measure`, given that `measure(items, { stats: ["mean"] })` then also adds
   ring fields?
4. Should awaiting an Image skip evaluation and return its statics? It would make `await img` free but move chain
   errors from the `await` to the first look or save.
5. Should a list of items from two region sets be accepted by giving each region its own raster, instead of refused?
6. Is the current directory the right default workspace for `goja-cv mcp` started by Claude Code (the project
   directory), or should MCP require `--root`?

## 18. References

- Coordinator diary: `reference/02-diary-coordination.md` (steps 1–3); GCV-001..003 coordination diary step 9
  (`GCV-001…/reference/01-diary-coordination-gcv-001-003.md`).
- Evidence: `reference/01-evidence-pi-run-api-usage.md`, `scripts/01-pi-api-usage.py`, `scripts/02-pi-names-typed.py`,
  `scripts/03-goja-panic-and-spread.go.txt`.
- Shared vocabulary: `GCV-001…/reference/05-foundations-goja-cv-for-the-gcv-guides.md`.
- Earlier guides and their deviations: `GCV-001…/design-doc/01-…` (§8 decisions on spaces, deviations 1–19),
  `GCV-002…/design-doc/01-…` (render classes, deviations 1–23), `GCV-003…/design-doc/01-…` (schema, data out,
  deviations 1–23). Implementation diaries: GCV-001 `reference/06-…`, GCV-002 `reference/03-…`, GCV-003 `reference/03-…`.
- Review fix commits: `4c79173` (#2), `cd5f46c` (#3), `319a6fc` (#4).
- Go `os.Root`: `go doc os.Root` (Go 1.26: `OpenRoot`, `Open`, `OpenFile`, `Lstat`, `MkdirAll`, `WriteFile`).
- The pi harness and runs: `CVJS-001…/scripts/06-pi-cv-agent/run.sh`, `CVJS-001…/various/pi-runs/README.md`.
- DUST-002 helpers: `DUST-002…/scripts/84-cv-helpers.js`, `83-cv-specks-vs-gold.py`, `85-cv-views.py`.
