---
Title: 'Editor lab and goja-cv: analysis and design guide for reuse'
Ticket: GCV-006
Status: active
Topics:
    - cv
    - architecture
    - performance
    - research
    - agents
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/cv/graph/cache.go
      Note: byte-capped LRU; cost-aware eviction would go here
    - Path: repo://pkg/cv/graph/eval.go
      Note: evaluator; region evaluation changes evalRun
    - Path: repo://pkg/cv/graph/registry.go
      Note: OpDef gains Reach; Global is the barrier flag
    - Path: repo://pkg/cv/graph/render.go
      Note: windows exist for crops and decorations; pixel ops still evaluate whole frames
    - Path: repo://pkg/cv/kernels/filter.go
      Note: Gaussian with 6 sigma + 1 taps; fast path for large sigma
    - Path: repo://ttmp/2026/11/14/GCV-006--what-to-reuse-from-the-editor-lab-region-evaluation-with-reach-tiles-cost-aware-cache-fast-blur-provenance-bench/sources/editor-lab/src/engine.js
      Note: the Editor lab engine (reach, tiles, content keys, TileCache, three-box blur)
ExternalSources: []
Summary: 'A textbook-style guide for a new intern: the fundamentals of tiled, cached, demand-driven image evaluation; how the Editor lab and goja-cv each implement them; measurements on a 24-megapixel image; and what goja-cv should take from the lab (region evaluation with declared reach first), with decision records, pseudocode, a phased plan and a glossary.'
LastUpdated: 2026-11-14T17:00:00-05:00
WhatFor: Deciding and building what goja-cv reuses from the Editor lab
WhenToUse: Before working on goja-cv evaluation performance, caching, large scans, provenance of saved results or a benchmark
---


# Editor lab and goja-cv: what to reuse

## How to read this guide

This guide compares two image engines and argues for a small set of changes to one of them. The **Editor lab** is
a photo editor that runs in one browser page, built between 12 and 14 November 2026; its source, tests, research
notes and a nine-chapter textbook arrived as `editor-lab-work.zip`. **goja-cv** is our Go engine that agents
script in JavaScript through an MCP `eval` tool, first used to inspect film scans for dust (DUST-002). The question
is which of the lab's mechanisms would make goja-cv better at its own job, and what each would cost.

The guide is ordered so that every term is defined before it is used:

| Part | What it gives you |
| --- | --- |
| I. Fundamentals (§§1–8) | The ideas both engines are built from: what an operation reads, borders, lazy graphs and content keys, scales and pyramids, tiles and regions, global operations, caches, measurement. |
| II. The two systems (§§9–10) | How the Editor lab and goja-cv each put those ideas together, with the code that does it. |
| III. Evidence (§11) | What goja-cv costs today on a 24-megapixel image, and what the lab's fast blur and JavaScript layers would cost in goja-cv. |
| IV. Proposals (§§12–14) | What to reuse, what goja-cv already has, and what to leave behind, with a decision record for each choice. |
| V. Building it (§§15–17) | A phased plan with files, tests and checks; risks and open questions. |
| Glossary and references (§§18–19) | Every term of art in this guide, and every file it cites. |

Callouts mark three kinds of aside:

> **Careful:** a mistake that is easy to make here.

> **Try it:** a command you can run to see the point for yourself.

> **Why:** the reason behind a design choice.

The Editor lab files this guide cites are copied into the ticket's `sources/` directory; `sources/README.md` says
what each one is. goja-cv paths are relative to the repository root, at `main` `5a93095`.

## Executive summary

The Editor lab and goja-cv agree on more than they differ. Both build a lazy graph of operations, both key cached
results by a hash of the operation, its arguments and the keys of its inputs, both give an agent one `eval` tool
that runs JavaScript against the engine's API and returns YAML, and both describe their operations to the agent
from a registry. The lab's distinctive strength is elsewhere: it never computes a pixel nobody asked for. Every
layer declares how far it reads beyond the pixels it writes (its **reach**), the engine computes the picture in
**tiles**, and a request for a small area pulls only the tiles under it, each widened by the reach of the layers
above. goja-cv computes every node over its whole frame, even when an agent then crops a 64-pixel window out of the
result.

On a 6,000 × 4,000 image that difference dominates. Cropping a 64 × 64 window out of a Gaussian blur cost goja-cv
161 ms at σ 2, 662 ms at σ 8 and 3.1 s at σ 32, because it blurs all 24 million pixels first; the window plus the
blur's reach is at most 0.3 % of them. One short script peaked at 3.6 GB of memory.

What to take, in order of value:

| # | Proposal | From the lab | Value for goja-cv | Cost |
| --- | --- | --- | --- | --- |
| 1 | **Region evaluation with declared reach** | `kind.reach`, `Engine.rect`, the apron | Crops, zoomed looks, `measure` and probes on large scans compute only what they read. Closes the open "windowed evaluation for large scans" item. | Large: a `Reach` on every op, window propagation in the evaluator, an invariance test over the registry. |
| 2 | **A tile grid with per-tile cache keys** | `Engine.key(i, L, tx, ty)`, `TileCache` | Overlapping windows share work; memory stays bounded. | Medium, after 1. |
| 3 | **A benchmark harness and per-node timing** | the Bench, `layerMs` | Every other proposal needs before-and-after numbers; agents see which step is slow. | Small to medium. |
| 4 | **Cost-aware eviction (GreedyDual-Size)** | `TileCache` policy `cost` | Keeps expensive results over cheap ones under the same byte cap. | Small; measure before adopting. |
| 5 | **A σ-independent Gaussian for large σ** | `boxSizes`, `gaussBlur` | Large blurs cost the same at σ 32 as at σ 4. | Small code, but changes values by up to 8 % of the signal: gold-gated. |
| 6 | **Provenance on saved results** | `Provenance`, `meta.source` | Every saved file says which eval, script line, inputs and graph made it; DUST-002 labels become traceable. | Small. |

**Decisions (14 November 2026, §12.7):** the bench and per-node timing (3) and provenance sidecars (6) are accepted and built first. Region evaluation (1) and the fast Gaussian (5) wait for evidence from real agent work; tiles (2) and cost-aware eviction (4) are deferred.

goja-cv already has the lab's runnable documentation blocks, operation introspection, result capping, length units
that scale with evaluation (the lab's level invariance) and constant-time morphology. It should not take the lab's
per-pixel JavaScript layers (1.6 s per megapixel in goja), its blend-mode compositing and undo (an editor's
concerns), or its WebGPU and WebAssembly probes (a browser's concerns).

# Part I. Fundamentals

## 1. What an operation reads

A digital image is a grid of samples. goja-cv stores one as a `buffer.Buffer` (`pkg/cv/buffer/buffer.go`): a width
`W`, a height `H`, a channel count `C` and one `[]float32` holding the channels as separate **planes**, so channel
`c` of pixel `(x, y)` is `Pix[c*W*H + y*W + x]`. The Editor lab stores a tile as one `Float32Array` with the four
channels of a pixel side by side (**interleaved**), index `(y*w + x)*4 + c`, and its colour is **premultiplied**: red,
green and blue are already multiplied by alpha, which makes compositing one multiply-add per channel. goja-cv has
no alpha channel; its images are scans.

A sample's value also has a meaning the array does not show. goja-cv tracks it as an **encoding** (file units,
linear light, a display curve) and refuses to mix encodings silently; that was the subject of GCV-001 and GCV-004,
and nothing here changes it.

What matters here is a simpler property: **which input pixels an operation reads to produce one output pixel.**
Three classes cover every operation in both engines.

| Class | Reads for output (x, y) | goja-cv examples | Editor lab examples |
| --- | --- | --- | --- |
| Point | only input (x, y) | `exposure`, `levels`, `mul`, `max`, `toGray`, `as` | exposure, levels, curves, hue/saturation, invert |
| Neighbourhood | inputs within distance m of (x, y) | `gaussian`, `dog`, `erode`, `dilate`, `localContrast`, `refine` | blur, sharpen, clarity |
| Global | inputs anywhere in the image | `stats`, `histogram`, `components`, `mask.flood`, `levels({ auto })` | none in the render path |

The distance `m` of a neighbourhood operation is its **reach**: the number of pixels it reads beyond the pixel it
writes, in each direction. A 3 × 3 box filter has reach 1. goja-cv's Gaussian uses a kernel of half-width
`ceil(3σ)` (`kernels.GaussianKernel`, `pkg/cv/kernels/filter.go:13`), so its reach is `ceil(3σ)`. A point operation
has reach 0. A global operation has no finite reach.

Reach composes by addition. If `b = a.gaussian(2)` reads 6 pixels around each output and `c = b.erode(4)` reads 4
pixels of `b` around each output, then one pixel of `c` depends on `a` within 6 + 4 = 10 pixels:

```text
 a  ............##########............     the 10 px on each side that c's pixel depends on
 b  ................##.................     gaussian(2): 6 px of a per pixel of b
 c  ..................#...................   erode(4): 4 px of b per pixel of c
```

A chain's reach is the sum of its members' reaches, and any global member makes the chain global.

> **Why:** reach is the one number that lets an engine compute part of an image correctly. Without it, the only safe
> way to get any pixel of `c` is to compute all of `b`, and therefore all of `a`'s blur. With it, a 64 × 64 window
> of `c` needs an 84 × 84 window of `a` and nothing else.

Key points:

- An operation's reach is the distance, in pixels, of the furthest input it reads for one output pixel.
- Point operations have reach 0, neighbourhood operations a finite reach, global operations none.
- The reach of a chain is the sum of its members' reaches; one global member makes the chain global.

## 2. Borders, and why a computed window can be exact

A neighbourhood operation near the edge of an image needs pixels that do not exist. goja-cv mirrors the image at its
border without repeating the edge sample (`reflect101`, `pkg/cv/kernels/filter.go:34`): index -1 reads index 1, -2
reads 2. The lab extends the edge pixel (`Engine.rect` "extends edges" by copying the outermost row and column).
Either rule is fine; what matters is that the engine applies it only at the image's real border.

That is the property region evaluation depends on. To get output window `R` of an operation with reach `m`, compute
the operation over `R` grown by `m` on every side and clipped to the image, then keep only `R`:

```text
 image frame
 +-----------------------------------------------------+
 |                                                     |
 |     R + m (computed)                                |
 |   +-----------------------+                         |
 |   |  apron, m px wide     |                         |
 |   |   +---------------+   |                         |
 |   |   |  R (kept)     |   |                         |
 |   |   +---------------+   |                         |
 |   |                       |                         |
 |   +-----------------------+                         |
 +-----------------------------------------------------+
```

Every pixel of `R` reads only pixels inside `R + m`, so it is computed from the same inputs as in a whole-image
evaluation. Pixels in the **apron**, the band between `R` and `R + m`, may read beyond the computed area and come out
wrong, which is why they are discarded. Where `R + m` was clipped by the image frame, the border rule applies exactly
as in a whole-image evaluation, because that edge is the real border.

> **Careful:** the result is exact only if the reach is exact. If an operation reads one pixel further than it
> declares, the outermost ring of `R` is computed from mirrored apron pixels and differs from the whole-image result
> by an amount no visual check will catch. §15 makes this a test over the whole registry.

Key points:

- A window of a neighbourhood operation is exact if it is computed from the window grown by the reach and cut back.
- The border rule applies only at the image's frame, so aprons clipped by the frame stay exact.
- An under-declared reach produces silent errors at the window's edge; it has to be tested, not reviewed.

## 3. Lazy graphs, hash-consing and content keys

Both engines separate describing a computation from running it. In goja-cv, `img.gaussian(2)` does not blur
anything: it adds a **node** to a graph (`pkg/cv/graph/node.go`) and returns a handle. Pixels are computed only when
a value is needed, by `stats()`, `look()`, `cv.save` and the like. This is **lazy evaluation**.

goja-cv **hash-conses** its nodes: `Builder.intern` hashes the operation, its bound arguments and its input nodes
(`hashNode`, `node.go:406`) and returns the existing node when that hash is already in the graph. Writing
`img.gaussian(2)` twice yields one node.

When a node is evaluated, the evaluator computes its **signature**, the cache key (`evalRun.sig`,
`pkg/cv/graph/eval.go`):

```go
h := sha256.New()
h.Write(n.Hash[:])                                       // op + args + input identity
binary.Write(h, binary.LittleEndian, int64(math.Round(r.scale*1e4)))
for _, in := range n.Inputs {
    is := r.sig(in)                                      // the inputs' signatures, recursively
    h.Write(is[:])
}
```

A source is identified by its workspace location, a stamp of the file's size and modification time, an optional
window of the file and the encoding it is read with (`SourceKey`, `pkg/cv/graph/sources.go`). So a signature is a
function of the file versions a node reads, the operations on the way, their arguments and the scale; equal
signatures mean equal values, and the cache (`pkg/cv/graph/cache.go`) is keyed by signature.

The Editor lab does the same per tile (`Engine.key`, `engine.js:478`): a tile's key mixes the layer's settings hash,
the tile's level and position, the **revisions** of the source pixels it covers, and the keys of the tiles it reads
below it. Its source pixels change when someone paints, so instead of a file stamp it keeps a revision number per
64-pixel block of each raster (`Raster.bump`, `Raster.contentKey`) and hashes the revisions a tile covers.

Mokhov, Mitchell and Peyton Jones (*Build Systems à la Carte*, ICFP 2018, cited in `research/notes.md`) call this
scheme a **deep constructive trace**: each key is built from the keys of what it depends on, never from values. A
lookup needs no pixels. The price is that a rebuild cannot stop early: if an edit changes a layer but leaves its
output identical, every key above still changes. The lab measured this on its test picture: an edit to a masked
layer left 16 to 26 of 40 output tiles bit-identical, yet every key above changed, so 33 % to 52 % of the
recomputation was avoidable (`test/cutoff.js`, book §6.8). The alternative, a **verifying trace**, hashes outputs,
at 0.093 ms per 128-pixel float tile in the lab.

> **Why this matters less for goja-cv:** goja-cv's sources do not change during a session, and agents rarely re-run
> a chain with an edit that leaves the output unchanged. Early cutoff solves an editor's problem. goja-cv's deep
> constructive signatures are the right design for it and stay.

Key points:

- Lazy evaluation builds a graph first and computes values only when a result is needed.
- Hash-consing makes structurally equal calls one node; signatures make equal computations one cache entry.
- Both engines use deep constructive keys; the lab keys tiles, goja-cv keys whole nodes.

## 4. Scales, pyramids and level invariance

A look at a 24-megapixel scan does not need 24 million pixels; a 1,600-pixel-wide preview has 1.2 million. Both
engines therefore compute at reduced resolution when they can.

A **pyramid** is a sequence of images, each half the width and height of the previous one (Williams 1983; Burt and
Adelson 1983). goja-cv builds one per source (`buffer.NewPyramid`, `pkg/cv/buffer/pyramid.go`), area-averaging down
to a long side of 256 pixels, and resamples previews from the coarsest level that is still large enough. The lab
builds level `L` of each raster lazily, by 2 × 2 box averages weighted by alpha (`Raster.level`).

An operation evaluated at reduced scale must still mean the same thing. A blur of σ 8 *document pixels* is a blur of
σ 4 buffer pixels at half scale. The lab writes this into each layer kind: `reach: (p, L) => blurReach(p.radius /
2 ** L)` and `gaussBlur(buf, w, h, 4, p.radius / 2 ** L)`. goja-cv writes it into the argument type: a `DocPx`
argument (`Unit` in `pkg/cv/graph/registry.go`) is a length in original-image pixels that the evaluator scales by
the evaluation scale. Both achieve **level invariance**: a preview approximates the full-resolution result rather
than showing a different operation.

```text
 gaussian(8)       scale 1          scale 0.5        scale 0.25
 σ in buffer px    8                4                2
 reach ceil(3σ)    24               12               6
```

goja-cv goes one step further for analysis: region operations evaluate their inputs at a fixed `AnalysisScale`, so
item ids and coordinates do not depend on the preview scale.

Key points:

- A pyramid holds an image at halving resolutions; previews read the coarsest level that suffices.
- Lengths in operation arguments are in document pixels and scale with the evaluation; reach scales with them.
- goja-cv already has level invariance through its `DocPx` unit; nothing in this guide changes it.

## 5. Tiles, windows and demand-driven evaluation

There are two ways to get a window of a result. **Whole-frame evaluation** computes each node over its entire frame
and cuts the window out of the final value. **Demand-driven evaluation** starts from the window and asks each node for
exactly the area its consumer needs: the consumer's window, widened by the consumer's reach.

libvips is the classic demand-driven engine (`sources/editor-lab/research/src/libvips/how-it-works.md`). A libvips
image can be a *partial image*: instead of pixels it stores a function that can make any rectangle of pixels on
demand, split into start, generate and stop so that threads share nothing while generating. Operations are joined by
regions, and intermediate buffers stay small enough for a processor cache. goja-cv's operation cache already
resembles libvips' operation cache, keyed by operation and arguments (`cache.c`); what goja-cv lacks is the region
request.

The Editor lab is demand-driven at tile granularity. `Engine.tile(i, L, tx, ty)` returns the composite after layer `i`
for one tile; for a layer with reach `m` it asks for the rectangle below, widened by `m`:

```js
// engine.js, Engine.tile (abridged)
const m = this.reach(l, L), b0 = this.below(i);
below = m ? this.rect(b0, L, R.x - m, R.y - m, R.w + 2 * m, R.h + 2 * m)   // tile + apron from the layer below
          : this.tile(b0, L, tx, ty).buf;                                  // reach 0: the same tile below
const buf = applyLayer(this, l, L, R, below, m);                           // compute; keep R only
this.cache.put(k, { buf, w: R.w, h: R.h, cost: dt });
```

`Engine.rect` assembles any rectangle from the tiles under it, recursively, so a request for one output tile pulls
exactly the tiles each layer needs.

```text
 request: output tile (3, 2) of the top layer
   top layer, blur reach 16  -> rect of the layer below: tile (3, 2) + 16 px apron = tiles (2..4, 1..3) below
   middle layer, reach 0     -> each of those tiles: the same tile of the layer below
   bottom layer, the image   -> raster pixels, keyed by their block revisions
```

Tiling has a price: each tile of a layer with reach `m` reads `(T + 2m)²` pixels to write `T²`. The lab's Table 5.1
gives this **apron factor**:

| tile T | σ 1.2 (m 2) | σ 6 (m 16) | σ 24 (m 70) |
| --- | --- | --- | --- |
| 64 px | 1.13 × | 2.25 × | 10.16 × |
| 128 px | 1.06 × | 1.56 × | 4.38 × |
| 256 px | 1.03 × | 1.27 × | 2.39 × |

Halide's authors describe the underlying trade as one between parallelism, locality and redundant computation (book
§5.3): store every intermediate (no recomputation, poor locality), recompute everything per output (twice the work
in their two-stage blur), or tile with overlap (1.0625 times the work, about ten times faster than storing). The lab
found that no tile size wins every workload: a small brush stroke ran 1 : 2.4 : 5.8 at 64, 128 and 256-pixel tiles,
while a large blur ran 3.5 times slower at 64 pixels than at 256 (book Figure 5.1).

> **Careful:** a single *window* request and a *tile grid* are different things. A window request computes exactly
> the window plus aprons, once. A tile grid rounds requests out to tile boundaries so that results can be shared
> between requests. goja-cv should build windows first (§12.1) and tiles second (§12.2).

Key points:

- Demand-driven evaluation asks each node for the area its consumer needs, widened by reach.
- Tiles make partial results reusable between requests, at the cost of aprons that grow with reach.
- Large reach favours large tiles; small local work favours small ones; no single size wins.

## 6. Global operations are barriers

A global operation cannot be computed from a window. `stats()` over a window gives the statistics of the window, not
of the image. When a demand-driven request reaches a global node, it stops: the global node's inputs are evaluated
over their whole frame.

The subtle case in goja-cv is an image operation whose *parameters* are global but whose *application* is pointwise.
`levels({ auto: true })` computes black and white points from the whole image, then maps each pixel; the speck view's
display stretch ("white = 5 × p99", `pkg/cv/ops/views.go`) works the same way through `levelsAt`, which is marked
`Global`. Written as two nodes, such an operation is a global statistic followed by a point operation, and a window
request passes through the point operation, needing only the statistic over the whole image.

```text
 view("speck") cropped to a 64 px window

 as(linear) -> gaussian(σ1) ---\
           \-> gaussian(σ2) ----> sub -> max(0) ----------> levels(black 0, white w) -> crop(64 px)
                                             \                         ^
                                              \-> p99 over the WHOLE --+   global statistic, w = 5 × p99
                                                  residual
```

The statistic can be made cheap in two ways: evaluate it over the full image once and cache it, which keeps today's
values exactly, or evaluate it on a coarse pyramid level, which changes them slightly. §12.1 decides between them.

There is a manual version of all this today: `cv.open(path, { box })` reads only a window of a file. An agent can
crop a scan at the source and work on the piece, but it must choose the apron itself, and every global statistic
(the speck view's stretch included) is then computed over the piece, not the scan, so the same speck looks different
in two pieces.

Key points:

- A global node stops a window request; its inputs are evaluated over their whole frame.
- An operation with global parameters and pointwise application should be two nodes, so windows pass through the
  pointwise part.
- Opening a window of the file is a manual workaround that changes every global statistic.

## 7. Caches, budgets and which entry to drop

A cache keeps computed values so that a second request costs nothing. Its **budget** bounds the memory it may hold;
when a new entry would exceed the budget, an **eviction policy** chooses what to drop. goja-cv's value cache is a
byte-capped **LRU** (least recently used) with a default cap of 2 GiB (`pkg/cv/graph/engine.go`, `cache.go`).
Decoded sources and their pyramids are kept separately, pinned for the session (`SourceStore`), outside that cap.

The lab offers LRU and **GreedyDual-Size** (Cao and Irani 1997). Each entry gets the priority `H = clock + cost /
size`, where cost is the time it took to compute; eviction removes the lowest `H` and raises the clock to it, so old
entries age out while expensive entries outlive cheap ones of the same age:

```js
// engine.js, TileCache (abridged)
put(k, e) { e.cost = Math.max(e.cost || 0, 0.01) * 1e6; e.H = this.clock + e.cost / e.buf.byteLength; /* ... */ }
evict(keep) {
  while (this.bytes > this.budget && this.map.size > 1) {
    // lru: drop the first entry in Map order; cost: drop the lowest H, then this.clock = victim.H
  }
}
```

Neither policy wins in general. On the lab's bench with a 32 MB budget, ten edits, ten undos and ten redos
recomputed 10,560 tiles under LRU and 2,662 under GreedyDual-Size, because LRU let shared, expensive tiles below the
edited layer age out; a brush stroke low in the stack went the other way, with GreedyDual-Size recomputing a third
more (book §6.4). The lab's conclusion is the one to keep: treat the policy as a setting and measure it on the
workloads that matter.

Key points:

- A cache needs a budget and a policy; the policy decides which results survive memory pressure.
- GreedyDual-Size weighs recency, compute cost and size without tuning parameters.
- The better policy depends on the access pattern and has to be measured.

## 8. Measuring an engine

Every proposal in this guide claims a speed-up or a memory saving, and every claim needs a number from a repeatable
measurement. The lab's book chapter 7 and its Bench give the vocabulary.

- **Service time** is how long a request takes once it starts; **response time** adds the time it waited. An
  agent's evals arrive one at a time, so for goja-cv the two are the same. The open-loop concerns of the lab's
  sources (coordinated omission, `research/src/wrk2/README.md`) apply only if goja-cv ever serves concurrent
  clients.
- **Percentiles** summarise a distribution: p50 is the median, p95 the value 95 % of runs stay under. A mean hides
  the slow runs an agent waits for.
- A **scenario** is a scripted workload that represents real use. The lab runs 9 scenarios on 4 modelled devices at 3
  tile sizes with 2 eviction policies and reports p50 and p95, tiles computed, hit rate and peak memory for each
  configuration.
- **Per-node cost** says where the time goes. The lab keeps milliseconds per layer (`Engine.layerMs`) and shows them
  to its agent (`ms_last_update` in `layer(id)`). goja-cv counts runs per node (`Evaluator.Runs`), not their time.

> **Careful:** a timing taken on a loaded machine is a sample, not a fact. The measurements in §11 were taken while
> another job kept the load average between 24 and 32 on 10 cores; their ratios are reliable, their absolute values
> are not. Repeat runs and report ranges, as the lab's book does.

Key points:

- For goja-cv, the service time of an eval is the latency that matters.
- Report percentiles over repeated runs of scripted scenarios, with memory and cache counters beside them.
- Time per node is the measurement that points at what to optimise.

# Part II. The two systems

## 9. The Editor lab in one page

The lab is one HTML page. Its architecture, after the report:

```text
 Stage tools   Agent (eval)   Code view   Bench   Book run blocks
      \             |             |          |          /
       +------------+----- lab API (labScope) ----------+
                                  |
                       Workspace: document, layers, masks, history, provenance
                                  |
                       Render: preview level first, then level-0 tiles in 8 ms slices,
                                  |   cancelled by generation
                       Engine: keys, tiles, aprons, levels  <->  TileCache (LRU | cost)
```

### 9.1 The document and the engine

A document is an ordered list of layers. A layer has a **kind** from the `KINDS` table (`engine.js:212`), settings
(`params`), opacity, a blend mode and an optional mask. A kind is a small record:

```js
blur: { group: 'Detail', label: 'Gaussian blur', main: 'radius', roi: true,
        params: { radius: P(0, 80, 6, 'px', 'radius (σ)', 0.1) },
        reach: (p, L) => Math.ceil(blurReach(p.radius / 2 ** L)),
        roiFn(buf, w, h, p, L) { gaussBlur(buf, w, h, 4, p.radius / 2 ** L); } },
```

Point kinds supply `fn(b, n, p, ctx)` over `n` pixels; neighbourhood kinds supply `roiFn` over a buffer that
includes the apron, plus `reach`; source kinds supply `paint`. The engine needs nothing else to tile, cache and
preview a kind. `Engine.key`, `Engine.tile` and `Engine.rect` (§§3, 5) implement the content-keyed tiled
evaluation. Groups add structure: a pass-through group at full opacity shares its top child's cache key, so wrapping
layers in a group costs nothing.

### 9.2 Blur as three boxes

The lab's Gaussian is three successive box blurs whose widths come from σ (`boxSizes`, `engine.js:74`, Kovesi's
construction). A box blur over a row is a **running sum**: add the sample entering the window and subtract the one
leaving it, so each output costs two additions whatever the width:

```text
 acc = sum(src[-r .. r])
 for x in 0 .. w-1:
     dst[x] = acc / (2r + 1)
     acc += src[x + r + 1] - src[x - r]        # border indices mirrored or clamped
```

Three successive boxes converge towards a Gaussian (repeated convolution with a box does), and their total cost does
not depend on σ. `blurReach(σ)`, the sum of the three box radii, is the reach the lab declares for it.

### 9.3 The agent's tool

The agent has one tool, `eval`, which "runs the body of an async JavaScript function with the lab API in scope and
returns the value as YAML" (`views/agent.js`). The scope is built per call by `labScope(readOnly, source)`: a
read-only scope replaces every mutating function with one that throws `read-only mode: … is off`, and `source`
stamps every layer the call creates with where it came from. Results longer than 9,000 characters are cut with the
hint "Return less: pick fields, slice arrays." The instructions carry a reference of the whole API
(`LAB_REFERENCE`), and every change is one undoable step in the history.

### 9.4 Provenance

Every layer carries `meta.source`, written when the layer is created (`Provenance` in `views/panels.js`):

```text
 preset    { id, name, index }           program { file, fileId, line, preset }
 script    { file, fileId, line }        agent   { thread, turn, step }
 codeLayer { file }                      any of them: via (the source that applied it), at (a timestamp)
```

The lab API reads it both ways: `source(id)` says what made a layer, `madeBy(q)` lists the layers a preset, file or
agent turn made, and `reveal(id)` opens the preset, or the program at the line.

### 9.5 The bench and the book

The Bench runs scenarios × device profiles × tile sizes × preview modes × eviction policies and reports update p50
and p95, settle time, tiles computed, hit rate, peak memory and a 16.7 ms gate. The book's numbers come from it; its
code quotes are pulled from the running page so that they cannot drift; its runnable blocks are tested.

## 10. goja-cv in one page

```text
 MCP eval / CLI eval
        |
 cvsession: one goja runtime per session, YAML results, looks, the context block
        |
 gojamod/cv: the JavaScript surface (cv.open, img.gaussian(...), await x.stats(), cv.save)
        |
 graph.Builder: hash-consed nodes, arguments bound and checked against the registry
        |
 graph.Evaluator: signatures, singleflight, parallel inputs  <->  graph.Cache (LRU, 2 GiB)
        |                \                                        SourceStore (decoded files and
 graph.Render: looks;     \                                        pyramids, pinned)
 windows for crops and     ops (an OpDef per operation) -> kernels (Gaussian, morphology,
 decorations only                                               peaks, measure, ...) -> buffer
```

### 10.1 The registry

Every operation is an `OpDef` (`pkg/cv/graph/registry.go`). The fields that matter here:

| Field | Meaning | For region evaluation |
| --- | --- | --- |
| `On`, `Returns` | receiver and result kind (Image, Regions, Value) | unchanged |
| `Class` | `PixelOp`, `GeometricOp` (crop) or `Decoration` (overlay, annotate) | unchanged |
| `Global` | "needs the whole image (stats, histogram)" | already the barrier flag; must become complete |
| `AnalysisScale` | evaluate inputs at a fixed scale | unchanged |
| `Args`, each with a `Unit` | `DocPx` lengths scale with the evaluation | reach is computed from them |
| `Run` | `func(ctx, ec *EvalCtx, in []Value, args Args) (Value, error)` | reads `ec.Window` |
| (missing) | how far the op reads | `Reach func(a Args, scale float64) int` |

`Global: true` is set today on `detect.specks`, `check.dust`, `coverage`, `stats`, `histogram`, `levelsAt` and
`components`. Other operations are global in fact but not marked: `mask.flood` (connectivity) and `levels({ auto })`;
`peaks` and `measure` return items over the whole image and need their own treatment (§12.1). Every op must be
classified before windows are safe.

### 10.2 The evaluator

`Evaluator.Evaluate(ctx, id, EvalOptions{Scale})` (`eval.go`) creates an `evalRun` for one scale, computes the node's
signature, returns a memoized or cached value if there is one, and otherwise runs `compute` inside a `singleflight`
group so that concurrent requests for one signature compute once. `compute` evaluates the inputs in parallel
(`errgroup`), applies the identity short-circuit (neutral arguments return the receiver), runs the op, stamps the
node's frame and encoding on the value, and caches it.

`EvalCtx` already has a `Window` field: "the doc-px rectangle a decoration draws in during a render above scale 1
(its inputs cover it); the node's frame otherwise." Only decorations use it today.

### 10.3 Rendering looks

`Evaluator.Render` (`render.go`) produces the image of a look. At scale ≤ 1 it evaluates the whole node and cuts the
window. Above scale 1 (a zoomed look) it dispatches on the op class: a crop forwards the window to its receiver; a
decoration draws at the render scale over its inputs rendered in the window; a pixel op is "evaluated at scale 1 over
its whole frame (cached as usual), cut to the window and upsampled". That last sentence is the gap: the window is
known, and the pixel op ignores it.

### 10.4 Side by side

| Concept | Editor lab | goja-cv today | Gap |
| --- | --- | --- | --- |
| Lazy graph | layer stack, evaluated per tile | hash-consed node graph | none |
| Content keys | per tile: settings, block revisions, keys below | per node: op, args, scale, input signatures | per window or tile (§12.1, §12.2) |
| Reach | `kind.reach(p, L)` on every neighbourhood kind | implicit in each kernel | `OpDef.Reach` (§12.1) |
| Partial evaluation | `Engine.rect` pulls tiles plus aprons | crops and decorations only; pixel ops whole-frame | windows for pixel ops (§12.1) |
| Pyramid, scale | lazy 2 × 2 levels, `radius / 2 ** L` | source pyramids, `DocPx` units | none |
| Cache | per tile, byte budget, LRU or GreedyDual-Size | per node, 2 GiB, LRU; sources pinned | policy option (§12.4) |
| Blur | three boxes, cost independent of σ | separable Gaussian, 6σ + 1 taps | fast path for large σ (§12.5) |
| Morphology | none | van Herk/Gil-Werman, constant per pixel | none |
| Agent tool | one `eval`, YAML, a scope per call | one `eval`, YAML, a session | none |
| Read-only | `labScope(readOnly)` | a session without a workspace cannot save | none |
| Result cap | 9,000 characters and a hint | `capText` and a note (`cvsession/yaml.go`) | none |
| Provenance | `meta.source` on every layer | none on saved files | sidecar (§12.6) |
| Per-node cost | `layerMs`, shown to the agent | run counts only | timing (§12.3) |
| Benchmark | Bench with scenarios and profiles | gold tests, no timing harness | harness (§12.3) |
| Runnable docs | run blocks tested | `TestGuideCodeBlocksRun` | none |

# Part III. Evidence

## 11. What goja-cv costs today

All measurements ran on a 10-core Apple machine whose load average stayed between 24 and 32 because another job was
running. Treat absolute times as samples; the argument uses the ratios. The scripts and outputs are in the ticket's
`scripts/` and the diary.

### 11.1 Windows of a large image

A synthetic 6,000 × 4,000 three-channel float image (288 MB as `.npy`; noise around 0.3 with 400 bright 3 × 3
specks), opened as linear light, then one 64 × 64 window of several results, each awaited once in one eval:

| Request | Time |
| --- | --- |
| open and `stats()` (decode, pyramid, statistics) | 4,303 ms |
| `crop([3000, 2000, 64, 64]).stats()` | 2 ms |
| `gaussian(2).crop(…64 px…).stats()` | 161 ms |
| `gaussian(8).crop(…).stats()` | 662 ms |
| `gaussian(32).crop(…).stats()` | 3,143 ms |
| `view("speck").crop(…).stats()` | 365 ms |
| the same view, another 64 px window (cached) | 0 ms |
| `peaks({ radius: [2, 4, 8], snr: 3 })` over the image | 1,871 ms |
| peak resident memory of the process | 3.6 GB |

The crop of the source costs 2 ms because the source is decoded already; a crop of any computed image costs the
whole computation. With region evaluation the σ 32 window needs `(64 + 2 × 96)² = 65,536` input pixels, 0.27 % of 24
million. The second speck-view window costs nothing today, and a design must keep that: agents look at many windows
of the same view, and the whole-image value, once computed, is the cheapest source for all of them.

The memory follows from the design: the pinned source (288 MB) and its pyramid, plus one 288 MB float image per
cached whole-frame result (three blurs, the view's intermediates), up to the 2 GiB cap.

> **Try it:** `python3 scripts/02-make-big-npy.py <dir>` writes the image; then
> `goja-cv eval --root <dir> -e "$(cat scripts/03-window-costs.js)"` prints the table's times.

### 11.2 Gaussian against three boxes

`scripts/01-boxblur-vs-gaussian.go` blurs one 6,000 × 4,000 plane with `kernels.Gaussian` and with a Go port of the
lab's three-box blur (running sums, rows then columns, both parallel over 10 cores). Three runs:

| σ | Gaussian, ms (3 runs) | three boxes, ms (3 runs) | max abs diff | max diff ÷ max deviation from mean |
| --- | --- | --- | --- | --- |
| 1 | 106 · 42 · 59 | 57 · 69 · 64 | 1.1e-1 | 31 % |
| 2 | 81 · 151 · 72 | 199 · 259 · 85 | 7.9e-3 | 5.6 % |
| 4 | 207 · 164 · 169 | 121 · 110 · 341 | 1.7e-3 | 3.8 % |
| 8 | 417 · 459 · 1,163 | 100 · 129 · 277 | 9.9e-4 | 6.3 % |
| 16 | 917 · 1,233 · 2,560 | 70 · 115 · 374 | 4.3e-4 | 6.7 % |
| 32 | 3,084 · 3,666 · 5,600 | 184 · 129 · 97 | 1.8e-4 | 7.9 % |

The Gaussian's cost grows linearly with σ, as its 6σ + 1 taps predict; the boxes' cost does not. The boxes win from
about σ 4 and by more than an order of magnitude at σ 32. The approximation is poor at σ 1 (box widths 1, 1 and 3)
and differs by 4 % to 8 % of the signal's peak deviation above it. For an engine whose thresholds were calibrated on
the exact Gaussian (`snr`, `peaks`, the speck view), that difference is not cosmetic; §12.5 gates it.

### 11.3 JavaScript per pixel

The lab lets a user write a layer's operation in JavaScript, called per pixel. In goja, a small function called once
per pixel over one megapixel took 1,569 ms; two Go point operations and a `stats()` over the same megapixel took
175 ms. At that rate a per-pixel JavaScript operation on a 24-megapixel scan would take about 38 s.

# Part IV. Proposals

Each proposal says what it is, how it would work in goja-cv, and records the decision. Status `proposed` means this
guide recommends it and nobody has agreed yet.

## 12. What to reuse

### 12.1 Region evaluation with declared reach

**What.** Every image operation declares its reach. An evaluation may carry a **window**, a doc-px rectangle. The
evaluator asks each node for its consumer's window widened by the consumer's reach and clipped to the node's frame;
point operations pass a window through unchanged; global operations stop it. A node evaluated for a window returns a
value that covers exactly the window.

**Where windows come from.** Three callers know a window today and throw it away:

1. `crop(box)`: its value is its receiver over `box`.
2. `Render` of a look: `renderRun.render` passes `win` to crops and decorations already; pixel ops should take it too.
3. `measure(items, …)`, the DUST-002 `mem.probe` helper and `cv.sheet`: each item reads a small neighbourhood (its
   core and ring radii, or the sheet cell). The union of the item windows is the request. For probes on a large
   scan this is the whole saving.

**The registry change.**

```go
// pkg/cv/graph/registry.go
type OpDef struct {
    // ...
    // Reach is how far, in buffer px at scale, the op reads beyond an output
    // pixel, in its receiver and in every Image argument. nil: 0 (a point op).
    // Global ops leave it nil: they read their whole input.
    Reach func(a Args, scale float64) int
}
```

| Op | Reach at scale s (σ and r in doc px) |
| --- | --- |
| `exposure`, `levels` (fixed points), `mul`, `add`, `max`, `as`, `toGray`, `mask.luminance` | 0 |
| `gaussian(σ)` | `max(1, ceil(3·σ·s))`, the kernel half-width |
| `erode(r)`, `dilate(r)` | `ceil(r·s)` |
| `localContrast(σ)` | `max(1, ceil(3·σ·s))` (its Gaussians share σ) |
| `refine(guide, radius)` | twice the box radius (a guided filter applies box means twice) |
| macros (`dog`, `highpass`, `view`) | none of their own: their expansion's nodes carry the reaches |
| `stats`, `histogram`, `components`, `mask.flood`, `levelsAt`, `levels({ auto })` | `Global: true` |

**The evaluator change.** `EvalOptions` gains `Window *Frame`. The request propagates down the graph and values are
cut on the way up:

```text
evalWindow(node n, window W):
    if n is a macro:                  return evalWindow(output node of n, W)
    if W is nil or n.Op.Global:       return cut(evalWhole(n), W)          # today's path
    if cache has n's whole value:     return cut(that value, W)             # never recompute what we have
    if area(W) > 0.5 × area(n.Frame): return cut(evalWhole(n), W)          # too large to be worth it
    m  := n.Op.Reach(n.Args, scale), in doc px: ceil(m / scale)
    Wi := clip(grow(W, m), n.Frame)
    ins := [evalWindow(input, Wi) for each input]                            # in parallel, as today
    v  := n.Op.Run(ctx, EvalCtx{Window: Wi}, ins, args)                      # buffers cover Wi
    cache v under sha256(signature(n), Wi)
    return cut(v, W)                                                         # drop the apron
```

Without tiles, two overlapping windows compute their overlap twice; §12.2 fixes that.

**What op code must respect.** A kernel that only reads buffers (`Gaussian`, `DiskMinMax`, arithmetic) works on a
windowed buffer unchanged. A kernel that turns buffer indices into document coordinates (`mask.box`, `mask.ellipse`,
`mask.gradient`, `peaks`, `measure`, `annotate`) must read the buffer's origin, which `ImageValue.BufferOrigin()`
already provides for cut buffers. A kernel whose result depends on statistics of its input (an "auto" option) is
global and must say so.

**Items.** `peaks` and `measure` return items over a frame. `measure` reads only item neighbourhoods and fits the
window scheme directly: request the union of the item windows from its image argument. `peaks` over a whole scan is a
whole-frame computation by nature; its kernel already works in bands with overlap (`bandPeaks` in
`pkg/cv/kernels/peaks.go`), so it can stay whole-frame in the first version.

**The test that makes it safe.** For every registered image op that is not global, build a small chain on a test
image, evaluate it whole and cut a window, evaluate it for the window, and compare:

```go
func TestRegionInvariance(t *testing.T) {
    for _, op := range imageOps(registry) {                 // every non-global op, typical and largest args
        full := evaluate(chain(op), nil)                      // whole frame
        for _, w := range windows(frame) {                    // interior, each edge, each corner, 1 × 1, whole
            got := evaluate(chain(op), &w)
            assertEqualBits(t, cut(full, w), got)             // bit for bit (§12.7): a kernel whose result depends
                                                                  // on the buffer origin must fail here
        }
    }
}
```

A missing or short `Reach` fails this test at the window's edge, which is exactly where a review would miss it.

#### Decision: windows before tiles

- **Context:** the lab tiles; goja-cv has no partial evaluation of pixel ops.
- **Options considered:** (a) a tile grid from the start; (b) windows first, tiles on top; (c) no partial
  evaluation, only a faster blur.
- **Decision:** (b).
- **Rationale:** a window request is the smallest change that delivers the 100 to 1,000-fold saving on crops and
  probes measured in §11.1. It needs `Reach`, window propagation and the invariance test, which tiles need too. Tiles
  add a grid, per-tile keys and assembly, and their benefit, shared overlaps, only shows once windows exist.
- **Consequences:** overlapping windows compute their overlap twice until §12.2 lands. The cut-from-whole path keeps
  today's behaviour whenever a whole value is cached.
- **Status:** proposed.

#### Decision: global statistics over the whole image, cached

- **Context:** the speck view's stretch and `levels({ auto })` need image-wide percentiles (§6).
- **Options considered:** (a) evaluate the statistic over the whole input at scale 1, once, and cache it; (b)
  evaluate it on a coarse pyramid level; (c) evaluate it over the window only.
- **Decision:** (a), after splitting each such op into a global statistic node and a point node.
- **Rationale:** (c) changes the view per window, so one speck would look different in two zooms. (b) changes the
  stretch slightly and needs its own validation. (a) keeps every value identical to today's; it costs one
  whole-image pass of the statistic's input per session, which agents pay today anyway.
- **Consequences:** the first speck-view window on a scan still costs a whole-image DoG; later windows cost their own
  area. Revisit (b) if the bench shows the first pass dominating.
- **Status:** proposed.

#### Decision: when a window is not worth it

- **Context:** a window covering most of a frame saves little and does not share cache entries with whole-frame
  requests.
- **Options considered:** always window; never window above a fixed area; window only below a fraction of the frame.
- **Decision:** window only when the window is under half the frame's area, and always cut from a cached whole value
  when one exists.
- **Rationale:** keeps today's behaviour for whole-image work (`peaks` over a scan, `stats`, saves) and for repeated
  windows of a cached view.
- **Consequences:** the threshold is a constant to tune with the bench.
- **Status:** proposed.

### 12.2 A tile grid with per-tile keys

**What.** Round each window request out to a fixed grid of tiles (`T` buffer pixels at the evaluation scale) and
cache each tile under `sha256(signature, scale, tx, ty)`. A window is assembled from its tiles, as `Engine.rect`
does. Overlapping windows, sheets of nearby items and repeated zooms then share tiles.

**Tile size.** goja-cv's reaches are larger than the lab's: the speck view for a 16-pixel size uses σ2 ≈ 21, a reach
of 64. By the apron formula, 256-pixel tiles then read 2.25 times what they write and 512-pixel tiles 1.56 times.
Start at 512, keep it a constant, and let the bench decide; the lab's Figure 5.1 shows the best size depends on the
workload.

> **Careful:** a tile at the right or bottom edge of a frame is smaller than `T`; the lab clips tiles to the picture
> (`tileRect`). If goja-cv aligned its grid to each node's frame, two crops of one source starting at different
> offsets would never share a tile. Align the grid to the source's document pixels instead.

#### Decision: tiles after windows, aligned to the source

- **Context:** tiles exist to share overlapping work; the grid decides whether two requests meet.
- **Options considered:** a grid per node frame; a grid in the source's document pixels; no grid.
- **Decision:** a grid in the source's document pixels, scaled per evaluation.
- **Rationale:** two crops of one source then share the source's tiles, and nodes downstream of one source line up.
- **Consequences:** a crop that starts off the grid computes partial tiles at its edges.
- **Status:** proposed.

### 12.3 A benchmark harness and per-node timing

**What.** A `bench` subcommand (or a Go benchmark suite) that runs **scenarios**, each an agent-like sequence of evals
on a fixed image, and reports per eval p50 and p95 over repeats, op runs, cache hits and misses, cached bytes and peak
resident memory. Scenarios to start with, from what DUST-002 agents do:

| Scenario | Evals |
| --- | --- |
| `tile-session` | open a 1,024² gold tile; `mem.views()`; `mem.hot()`; three `mem.zoom`; one `mem.sheet` of 12 points |
| `scan-zoom` | open a 24 MP scan; the speck view; 20 windows of 64 px at random positions |
| `scan-probe` | open a 24 MP scan; `mem.probe` at 50 points |
| `scan-sweep` | four speck-view quarters at 2× |
| `big-blur` | `gaussian(σ)` for σ 2 to 32, whole and as a 64 px window |

Alongside it, the evaluator records the duration of each `Run` per node beside `countRun`, and `explain()` shows it,
as the lab shows `ms_last_update` to its agent. That tells an agent which step of its chain is expensive and tells us
where to optimise.

#### Decision: a harness before any optimisation lands

- **Context:** §11's timings come from ad-hoc scripts on a loaded machine.
- **Options considered:** ad-hoc timing per PR; Go `testing.B` benchmarks only; a scenario harness.
- **Decision:** a scenario harness, plus Go benchmarks for kernels.
- **Rationale:** scenarios measure what agents feel (eval latency, memory); kernel benchmarks isolate one change. Both
  are needed to accept proposals 1, 2, 4 and 5 on evidence.
- **Consequences:** the first PR changes no behaviour and records a baseline.
- **Status:** accepted (14 November 2026, §12.7).

### 12.4 Cost-aware eviction

**What.** A policy option on `graph.Cache`: `lru` (today) or `cost`, GreedyDual-Size with the run time recorded in
§12.3 as the cost. goja-cv's entries differ in cost far more than the lab's tiles do: a whole-image speck view of a
scan costs seconds, a window of it milliseconds. Under LRU, a burst of cheap windows can push out the whole-image view
that every later window would cut from.

#### Decision: measure, then choose the default

- **Context:** the lab found each policy better on different workloads.
- **Options considered:** keep LRU; switch to GreedyDual-Size; make it an option.
- **Decision:** add the policy as an engine option and choose the default from the `scan-*` scenarios.
- **Rationale:** cheap to add (the lab's version is about 25 lines); the right default is an empirical question.
- **Consequences:** eviction scans all entries unless a heap is added; goja-cv's caches hold hundreds of entries, so a
  scan is acceptable at first.
- **Status:** proposed.

### 12.5 A σ-independent Gaussian for large σ

**What.** For σ at or above a threshold (σ ≥ 8 buffer pixels, from §11.2), compute the Gaussian as three box blurs with
running sums; below it, keep the exact separable kernel. For very large σ, an alternative is to blur a decimated
level and upsample, as Vello's CPU renderer does (`research/notes.md`: σ 100 becomes six decimations and an 11-tap
kernel, about 1 % of the direct blur's reads).

#### Decision: gate the fast path on the gold tests

- **Context:** the fast path is 10 to 30 times faster at σ 32 and differs by up to 8 % of the peak signal.
- **Options considered:** replace the Gaussian; add the fast path for large σ only; leave the Gaussian and rely on
  windows.
- **Decision:** large σ only, adopted only if the gold-gated tests (`peaks_gold_test.go` and the measure tests) keep
  their numbers within agreed tolerances and the region-invariance test passes for both paths.
- **Rationale:** windows (§12.1) remove most of the cost for crops and probes; the fast path matters for the
  whole-image passes that remain (the first speck view, `peaks` over a scan). Those passes feed calibrated thresholds,
  so their values must not move without evidence.
- **Consequences:** two code paths for one op. The box path's reach (the sum of its box radii, the lab's `blurReach`)
  differs from `ceil(3σ)` and must be what `Reach` reports for that σ.
- **Status:** proposed; lower priority than §§12.1 to 12.3.

### 12.6 Provenance on saved results

**What.** Every `cv.save` writes a sidecar `<name>.provenance.json` beside the file:

```json
{
  "goja_cv": { "version": "v0.0.0-...", "vcs_revision": "5a93095" },
  "session": "c1f4...", "eval": 17, "at": "2026-11-14T16:40:02Z",
  "site": { "file": "84-cv-helpers.js", "line": 36 },
  "node": { "path": ["open(b2a1....tif)", "as(linear)", "peaks({ radius: [2, 4, 8], snr: 1 })"],
            "signature": "9f3c..." },
  "sources": [ { "path": "/Users/.../cv_tiles/b2a1....tif", "size": 6291728,
                 "mtime": "2026-11-09T21:59:00Z", "sha256": "..." } ]
}
```

The lab stamps provenance on every layer when it is created, because its layers persist in a document. goja-cv's only
persistent outputs are saved files, so the save is the place to stamp. `pkg/gojamod/cv/save.go` already resolves the
calling site (file and line) for its error messages, and `Evaluator.Path` already produces the chain of labels.

#### Decision: a sidecar, not a field inside the file

- **Context:** saved JSON is read by DUST-002 scripts as arrays of rows; images are saved as TIFF, PNG or NPY.
- **Options considered:** a `provenance` key inside JSON objects; TIFF tags; a sidecar for every save.
- **Decision:** a sidecar for every save.
- **Rationale:** one rule for every format and no change to what readers parse. The project needs no backward
  compatibility; the sidecar is chosen because it is uniform, not to protect old readers.
- **Consequences:** two files per save; the workspace's rule against replacing files the session opened applies to the
  sidecar too. Hashing a 288 MB source costs about 0.3 s, so hash once per source load and keep the hash in the
  `SourceStore`.
- **Status:** accepted (14 November 2026, §12.7).

### 12.7 Decisions after review (14 November 2026)

The proposals above were reviewed with the user, and with evidence this guide did not have when it was written: a
pattern language distilled from goja-cv, the lab and the dust pipelines (the vault's *Agent-Operated Engines*,
chapter 4 "Reach-Declared Region Evaluation" is §12.1 as a pattern), validated by having agents build small region
engines from the chapter and without it, and by blind design reviews of those builds.

| # | Proposal | Status | Why |
| --- | --- | --- | --- |
| 3 | Bench and per-node timing | **accepted, Phase 0** | Every other decision needs before-and-after numbers; per-node time in `explain` helps agents find the slow step. |
| 6 | Provenance sidecars | **accepted, Phase 6** | Small, independent; makes saved labels traceable. |
| 1 | Region evaluation | **conditional** | Large; worth it only if agents really crop computed images of full scans. §11.1 measured a synthetic case; the DUST annotators mostly worked on tiles. Decide from the bench and from annotator transcripts. |
| 5 | Fast Gaussian | **conditional, opt-in** | 4–8 % value change against calibrated thresholds; add as a separate operation only if agents use large σ. |
| 2 | Tiles | **deferred** | The whole-result cache already serves repeated windows for free. |
| 4 | Cost-aware eviction | **deferred** | Wait for the bench to show pressure on the 2 GiB cap. |

**Lessons for §12.1, if it goes ahead.** Agents that built region engines from the pattern chapter got exactness and
economy right more often than those without it, but blind reviewers judged the leaner builds better adapted, and
probes found these traps:

- **Exact equality, not tolerance.** A prefix-sum box blur computed over a buffer makes a window differ from the
  whole-image result in the last bits (872 of 941 cells in one build), because the arithmetic depends on where the
  buffer starts. Tolerance-based invariance tests hid it. `TestRegionInvariance` compares bit for bit.
- **A footprint guard in tests.** Cut every input, sources included, to the declared reach and fail on a read
  outside it. Only one of five builds did, and it was the only one whose wrong reach could not pass silently.
- **Windows one at a time.** The "evaluate whole when a window costs about the whole" rule is per node and per
  window. One build applied it to the bounding box of two distant windows and computed the whole grid.
- **Global statistics as ordinary inputs.** `divide(img, stat(img))`: the statistic is an input node evaluated whole,
  the point half is windowed. This is the split §12.1 already proposes for the speck view.
- **Keep the best case.** A cached whole value serves later windows; one build lost this and recomputed every window.
- **No tiles, no cost constant copied from the lab.** The 0.6 threshold is a placeholder until the bench measures the
  crossover.

**Status of §12.1's decision records:** unchanged (`proposed`), with the precondition above.

## 13. What goja-cv already has

These lab features have working goja-cv counterparts; reusing them would duplicate code.

| Lab feature | goja-cv counterpart |
| --- | --- |
| Runnable book blocks, tested | `TestGuideCodeBlocksRun` (`pkg/cvsession/docs_test.go`) runs every `js` block of the guide |
| `kinds()` with ranges, units and defaults | `help()`, function docs and the context block, all from the registry |
| Result cap with a hint | `capText` (`pkg/cvsession/yaml.go`) |
| Read-only scope | a session without a workspace: `cv.save` fails with `E_ACCESS` (`gojamod/cv/save.go`) |
| Level invariance (`radius / 2 ** L`) | `DocPx` arguments scaled per evaluation |
| Mask components (linear, radial, luminance, colour, brush) | `mask.gradient`, `mask.ellipse`, `mask.box`, `mask.luminance` with softness, `mask.color`, `mask.flood`, `refine`, `and`, `or`, `not` |
| Undo per agent step | not needed: values are immutable and nothing is edited in place |
| Presets as programs | scripts and recipes loaded with `cv.load` and listed by `cv.scripts` (GCV-005) |
| Constant-time large kernels | morphology uses van Herk/Gil-Werman (`kernels/morph.go`) |

## 14. What to leave behind

#### Decision: no per-pixel JavaScript operations

- **Context:** the lab's code layers run a user's JavaScript function per pixel.
- **Options considered:** an `img.map(fn)` op; an expression language compiled to Go; neither.
- **Decision:** neither, for now.
- **Rationale:** goja ran 1.6 s per megapixel (§11.3), about 38 s per call on a scan. Agents compose the registry's
  operations instead, which run in Go and cache.
- **Consequences:** a genuinely new pixel operation needs a Go op. Revisit if agents ask for one that composition
  cannot express.
- **Status:** proposed.

The rest is outside goja-cv's job:

- **Blend modes, groups, layer documents and history.** An editor composes and edits; goja-cv measures. Its `blend`
  op covers what analysis needs.
- **Coarse-first previews and time-sliced rendering.** These keep an interactive canvas responsive. goja-cv answers
  one eval at a time; it should cancel cleanly between kernel bands, which it does through the context passed to
  `parallel.Rows`, and does not need slices.
- **WebGPU and WebAssembly.** Browser executors. goja-cv's levers are Go parallelism and, later, SIMD kernels.
- **`lab:` links, the dock and the stage tools.** User interface.

# Part V. Building it

## 15. Phased plan

Each phase is one PR with the usual gate (`make lint`, `make vet fmt-check`, `go test ./...` and the gold-gated tests
on the local data). Every phase after Phase 0 reports the bench before and after.

**Phase 0. Bench and timing (§12.3).**

1. `pkg/cv/graph/eval.go`: record the duration of each `safeRun` per node beside `countRun`; add
   `Evaluator.RunTime(id)`.
2. `pkg/cv/graph/explain.go`: show the time per node.
3. `cmd/goja-cv/bench.go` (or `pkg/cv/bench`): scenarios as JavaScript files, a generator for the synthetic scan,
   repeats, p50 and p95, cache statistics, peak resident memory.
4. Record the baseline in the ticket.

**Phase 1. Reach declarations (§12.1, no behaviour change).**

1. `pkg/cv/graph/registry.go`: `OpDef.Reach`.
2. `pkg/cv/ops/*.go`: `Reach` for every neighbourhood op; `Global: true` for `mask.flood`, `levels({ auto })` and
   anything the audit finds; split ops with global parameters into a statistic node and a point node.
3. A registry test: every image op is on an explicit list of point ops, declares `Reach`, or is `Global`.

**Phase 2. Window evaluation (§12.1).**

1. `pkg/cv/graph/eval.go`: `EvalOptions.Window`, propagation, `(signature, window)` keys, the cut-from-whole path and
   the area threshold.
2. `pkg/cv/ops/geometry.go`: `crop` evaluates its receiver with its box as the window.
3. `pkg/cv/graph/render.go`: pixel ops render through the window path instead of whole-frame evaluation.
4. `pkg/cv/ops/measure.go`, `measurement.go` and `sheet.go`: request the union of item windows.
5. Audit every kernel that maps buffer indices to document coordinates to read the buffer origin.
6. Tests: `TestRegionInvariance` over the registry, the existing suite, the gold-gated tests unchanged.

**Phase 3. Tiles (§12.2).** A grid in source doc px, per-tile keys, assembly, the tile size as a constant, bench at
256 and 512.

**Phase 4. Eviction policy (§12.4).** The `graph.Cache` policy option with the run time as cost; bench both.

**Phase 5. Fast large-σ Gaussian (§12.5).** The kernel, `Reach` per path, the invariance test for both paths, the gold
tests, the bench.

**Phase 6. Provenance sidecars (§12.6).** `gojamod/cv/save.go` and `dataout/save.go` write the sidecar; the
`SourceStore` keeps source hashes; a test that every save writes a sidecar with the right site and path.

Phases 0 and 6 are independent of the rest and can go first.

## 16. Testing and validation

| Check | What it guards | Where |
| --- | --- | --- |
| Region invariance over the registry | exact windows, correct reach, correct coordinates | new, `pkg/cv/graph` or `pkg/cv/ops` |
| Registry completeness | no neighbourhood op without `Reach` or `Global` | new, `pkg/cv/ops` |
| Gold-gated tests | calibrated numbers (`peaks`, `measure`, the speck view) do not move | `peaks_gold_test.go`, `measure_real_test.go` |
| Unit and guide-block tests | behaviour of every op and every guide example | `go test ./...` |
| Bench scenarios | the speed and memory claims | the Phase 0 harness |
| Provenance test | every save writes a sidecar with site, path, signature and sources | new, `pkg/gojamod/cv` |

> **Careful:** `TestRealTopHatSaturated` already fails on `main` because the gold set grew (GCV-005 coordination
> diary, step 6). Fix or re-baseline it before Phase 2, or its failure will hide a real regression.

## 17. Risks and open questions

**Risks.**

- *Reach mistakes are silent.* The invariance test is the only defence; it must cover every op with realistic
  arguments, including the largest σ and radius agents use.
- *Windowed values break assumptions.* Code that assumes a value's buffer covers its node's frame (`stamp`,
  `cutValue`, the savers, looks) must be audited; `ImageValue.origin` already exists for one such case.
- *Cache fragmentation.* Window keys without tiles can fill the cache with many small, non-overlapping entries; the
  area threshold and Phase 3 limit it.
- *Two Gaussian paths* double what must stay calibrated.

**Open questions.**

1. Should the first whole-image pass under a speck-view window use a coarse pyramid level for its percentile (option
   b in §12.1's second decision)? Only the bench can say whether that pass dominates.
2. Does `peaks` over a whole scan gain from tiles, given that its bands already run in parallel, or only from the
   fast Gaussian?
3. Should provenance also appear in the eval's YAML, as a `saved:` entry with the sidecar's path, so an agent can
   cite it?
4. Is σ 8 the right threshold for the fast path, given a cost crossover near σ 4 and its error profile?

## 18. Glossary

**Apron.** The band of pixels around a requested area that a neighbourhood operation reads but whose own results are
discarded; its width is the reach. Also called overlap or halo.

**Blend mode.** A rule for combining a layer's colour with the colour below it (multiply, screen, overlay and so on),
standardised in W3C Compositing and Blending. The lab implements 18.

**Box blur.** The mean over a square window. Computed with a running sum, it costs the same per pixel at any width;
three successive box blurs approximate a Gaussian.

**Buffer pixel, document pixel.** A document pixel (doc px) is a pixel of the original image; a buffer pixel is a
pixel of the array an evaluation computes. At scale `s`, one buffer pixel spans `1/s` document pixels.

**Cache budget.** The memory a cache may hold. goja-cv's value cache defaults to 2 GiB.

**Content key, signature.** A hash that identifies a computed value by what produced it. goja-cv's signature hashes a
node's op, arguments and scale with its inputs' signatures.

**Coordinated omission.** A benchmarking error in which a load generator waits for slow responses and so never
measures the requests that would have queued behind them (wrk2's README).

**Deep constructive trace.** A build-system scheme in which each key is built from the keys of its dependencies, not
their values (Mokhov et al. 2018). Lookups need no data; rebuilds cannot stop early when an output is unchanged.

**Demand-driven evaluation.** Computing only the area a consumer requests, by propagating the request down the graph
(libvips' partial images, the lab's `Engine.rect`).

**DoG (difference of Gaussians).** `G_σ1(a) − G_σ2(a)` with σ1 < σ2, a band-pass filter that keeps structures between
two sizes. goja-cv's speck view is a DoG.

**Early cutoff.** Stopping a rebuild when a recomputed value equals its previous value, so nothing that depends on it
is rebuilt. It needs output hashes, a verifying trace.

**Eviction policy.** The rule that chooses what a full cache drops: LRU, GreedyDual-Size, darktable's ageing.

**Frame.** In goja-cv, the doc-px rectangle a node's value covers (`graph.Frame`): a source's extent or a crop's box.

**Global operation.** An operation whose output may depend on any input pixel (statistics, connectivity). It cannot
be computed from a window. goja-cv marks these `Global: true`.

**GreedyDual-Size.** An eviction policy that gives each entry the priority `clock + cost / size`, evicts the lowest
and raises the clock to it (Cao and Irani 1997).

**Hash-consing.** Returning the existing node when a structurally identical node is requested, so equal computations
share one node.

**Level invariance.** The property that an operation evaluated at a reduced pyramid level approximates the same
operation at full resolution, achieved by scaling its lengths with the level.

**LRU (least recently used).** An eviction policy that drops the entry unused for longest.

**Macro node.** In goja-cv, a node whose op expands into a graph of other ops at build time (`dog`, `highpass`,
`view`); it computes nothing itself.

**Neighbourhood operation.** An operation whose output pixel depends on input pixels within a finite distance, its
reach: blurs, morphology, local contrast.

**Partial image.** In libvips, an image that stores a function able to compute any rectangle of pixels on demand
instead of the pixels themselves.

**Percentile (p50, p95).** The value below which that percentage of measurements falls; p50 is the median.

**Point operation.** An operation whose output pixel depends only on the input pixel at the same position: exposure,
levels, arithmetic, colour conversion. Its reach is 0.

**Premultiplied alpha.** Colour stored already multiplied by its alpha (coverage), which makes compositing one
multiply-add. The lab uses it between operations; goja-cv has no alpha.

**Provenance.** A record of what produced a value: the program and line, the inputs, the graph, the time.

**Pyramid.** A sequence of images each half the size of the previous one, used for previews and large-scale
operations.

**Reach.** How far, in pixels, an operation reads beyond the output pixel it computes: 0 for point operations,
`ceil(3σ)` for goja-cv's Gaussian, unbounded for global operations.

**reflect101.** goja-cv's border rule: mirror the image at its edge without repeating the edge sample.

**Region invariance.** The property this guide asks of every non-global op: evaluating it for a window equals
evaluating it whole and cutting out the window.

**Running sum.** Computing a sliding-window sum by adding the entering sample and subtracting the leaving one, at
constant cost per output.

**Scale.** In goja-cv, buffer pixels per document pixel for an evaluation: in (0, 1] when evaluating, up to 16 when
rendering a zoomed look.

**Scenario.** A scripted workload a benchmark uses to represent real use.

**Separable filter.** A two-dimensional filter computed as a horizontal pass then a vertical one, costing `2k` instead
of `k²` per pixel for a `k`-wide kernel. goja-cv's Gaussian is separable.

**Service time.** The time a request takes once it starts, excluding any wait.

**Singleflight.** A concurrency pattern (`golang.org/x/sync/singleflight`) in which concurrent requests for one key
share one computation.

**Tile.** A fixed-size rectangle of a grid over an image; the unit of partial computation and caching in the lab.

**Top-hat.** An image minus its morphological opening; it keeps bright structures smaller than the structuring
element. In goja-cv, `img.sub(img.erode(r).dilate(r))`.

**van Herk/Gil-Werman.** An algorithm for the running minimum or maximum over a window at three comparisons per sample,
whatever the window size; goja-cv's morphology uses it.

**Verifying trace.** A build-system scheme that records hashes of a computation's inputs and outputs, so a rebuild can
check whether anything changed and stop early.

**Window.** In this guide, a doc-px rectangle requested from an evaluation. In goja-cv's render path, the rectangle a
look shows (`RenderOptions.Window`, `EvalCtx.Window`).

## 19. References

Editor lab, copies in `sources/editor-lab/`:

- `src/engine.js`: `boxSizes`, `blurReach` and `gaussBlur` (lines 74–136); `KINDS` with `reach` (212–286);
  `TileCache` (400–426); `Engine.struct`, `key`, `tile` and `rect` (429–552).
- `src/views/agent.js`: `labScope` (9–137), `LAB_REFERENCE`, `runLab` (225–240).
- `src/views/panels.js`: `Provenance` (25–75).
- `book/05.md` §§5.2–5.3 (tile size, aprons); `book/06.md` §§6.1–6.8 (keys, eviction, early cutoff); `book/07.md`
  (measuring); `book/09.md` §§9.5–9.7 (render graph, scheduler, cache tiers).
- `REPORT.pdf`: phases, measurements, what was not built.
- `research/notes.md` and `research/src/` (libvips, darktable, GIMP, GEGL, wrk2).

goja-cv, `main` at `5a93095`:

- `pkg/cv/graph/eval.go`: `Evaluator`, `evalRun.sig`, `eval`, `compute`.
- `pkg/cv/graph/registry.go`: `OpDef`, `OpClass`, `Unit`.
- `pkg/cv/graph/render.go`: `Render`, `renderRun.render`, `cutValue`.
- `pkg/cv/graph/cache.go`, `engine.go`: the LRU cache and its 2 GiB default.
- `pkg/cv/graph/sources.go`: `SourceKey`, `SourceStore`.
- `pkg/cv/graph/frame.go`, `value.go`: `Frame`, `ImageValue`, `BufferOrigin`.
- `pkg/cv/kernels/filter.go`: `GaussianKernel`, `Gaussian`, `LocalContrast`.
- `pkg/cv/kernels/morph.go`: van Herk/Gil-Werman morphology.
- `pkg/cv/kernels/peaks.go`: `Peaks`, `bandPeaks`.
- `pkg/cv/ops/views.go`: the speck view and `levelsAt`.
- `pkg/cv/buffer/pyramid.go`: source pyramids.
- `pkg/gojamod/cv/save.go`, `pkg/cv/dataout/save.go`: saving.

This ticket: `scripts/01-boxblur-vs-gaussian.go` and its output, and the diary (`reference/01-diary.md`).

Papers and documents cited through the lab's notes: Ragan-Kelley et al., *Halide* (PLDI 2013); Mokhov, Mitchell and
Peyton Jones, *Build Systems à la Carte* (ICFP 2018); Cao and Irani, *Cost-Aware WWW Proxy Caching Algorithms* (USITS
1997); Williams, *Pyramidal Parametrics* (SIGGRAPH 1983); Burt and Adelson, *The Laplacian Pyramid as a Compact Image
Code* (1983); the libvips documentation *How it works*.
