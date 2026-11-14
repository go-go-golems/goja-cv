# Sources for GCV-006

Copied on 14 November 2026 from `~/Downloads/editor-lab-work.zip` (the Editor lab, built 12 to 14 November 2026;
README and REPORT inside). Only the files the design guide cites are here; the zip holds the rest (the UI views,
styles, tests, figures, the earlier docs). The files are copies for reading, not code goja-cv builds.

## Editor lab (`editor-lab/`)

| File | Why it is here |
| --- | --- |
| `README.md`, `REPORT.pdf`, `REPORT-text-extract.txt` | What the lab is, its phases, its measurements, what it did not build. The text extract was made with macOS PDFKit (no poppler on this machine); tables lose their layout in it. |
| `src/engine.js` | The engine: `Raster` (64 px block revisions, lazy pyramid), the layer kinds with `reach`, three-box Gaussian (`boxSizes`, `boxH4`, `gaussBlur`), `TileCache` (LRU and GreedyDual-Size), `Engine.key`/`tile`/`rect` (content keys per tile, aprons), masks, `applyLayer`/`applyGroup`. Guide §§4–7, 9. |
| `src/workspace.js` | The document, history as commands, `Provenance` (where each layer came from). Guide §12.5. |
| `src/app.js` | `Render`: coarse preview first, then level-0 tiles in time slices, cancelled by generation. Guide §8. |
| `src/views/agent.js` | The agent's one tool, `eval`, with the lab API in scope (`labScope`), read-only mode, YAML results, result cap with a hint, provenance stamping. Guide §9.3. |
| `src/views/bench.js` | Scenarios × device profiles × tile sizes, p50/p95, the 16.7 ms gate. Guide §12.6. |
| `src/views/code.js` | Code layers and programs (JavaScript operations compiled per source). Guide §13 (rejected). |
| `book/04.md`–`07.md`, `book/09.md` | The chapters on neighbourhood operations, pyramids and tiles, caching by content, scheduling and measuring, and designing a fast engine. Their numbers come from the lab's own bench. |
| `research/notes.md` | One page of facts from the third-party source below, with file and line references. |

## Third-party source (`editor-lab/research/src/`)

Downloaded by the Editor lab's author from each project's public repository on 12 November 2026, at the revisions in
`SOURCES.md`. Each file keeps its own licence (GPL for darktable, GIMP and GEGL; LGPL for libvips), stated in its
header. Only the files that bear on goja-cv are copied here.

| File | Why it is here |
| --- | --- |
| `libvips/how-it-works.md`, `cache.c`, `tilecache.c`, `threadpool.c`, `sinkscreen.c` | Demand-driven partial images (start / generate / stop over regions) and an operation cache keyed by operation and arguments: the closest existing design to what goja-cv's region evaluation would become. Guide §5, §12.1. |
| `darktable/pixelpipe_cache.c`, `.h`, `tiling.c` | A cache key chained through the pipeline (the same deep constructive trace as goja-cv's signatures); tiling with declared overlap when memory is short. Guide §6, §12.1. |
| `gimp/gimpchunkiterator.c` | Adaptive chunk sizes for a time budget. Guide §8 (not proposed). |
| `gegl/gegl-tile-handler-cache.c` | An LRU tile cache with a byte budget. Guide §7. |
| `wrk2/README.md` | Coordinated omission: why a benchmark must measure from the intended start time. Guide §8. |
