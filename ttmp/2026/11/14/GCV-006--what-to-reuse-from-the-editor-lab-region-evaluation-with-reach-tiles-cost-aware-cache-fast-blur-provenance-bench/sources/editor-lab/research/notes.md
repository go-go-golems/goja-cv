# Research notes for chapters 5–9 (2026-11-12)

Source code read at these revisions (GitHub HEAD on 2026-11-12):
darktable 157457d583dc · GIMP c483bf789fbd · GEGL b500a75d43dd · Krita 1cc1d5c7e335 · libvips 5e918bd43a5f · Chromium 0fcf5b8bdf04

## darktable (src/develop/pixelpipe_cache.c, pixelpipe_hb.c, develop.c, tiling.c, common/darktable.h)
- Cache key: dt_hash = djb2 variant `hash = ((hash << 5) + hash) ^ byte`, 64-bit, init 5381 (darktable.h:838-851).
- Key chains image id, pipe type, colour profiles, then each enabled, non-skipped module's piece->hash up to the position; ROI and detail mask hash appended when a ROI is given (pixelpipe_cache.c:99-173).
- Cache lines: full pipe 64 entries, preview pipe 12, preview2 5, export/thumbnail DT_PIPECACHE_MIN = 2 (pixelpipe_hb.c:251-270, pixelpipe_hb.h:32).
- Eviction: every lookup ages all lines (used[k]++); a hit sets used = -entries ("important"); the oldest line other than the last one is reused. Hash match with a size mismatch invalidates the line ("hash overlaps").
- Cancellation: atomic pipe->shutdown flag checked between modules; a new parameter change does not stop a running full pipe if it started less than half the average pipe runtime ago (develop.c:283-306).
- Tiling when memory is short: modules declare factor, maxbuf, overhead, overlap; tiles are cut with `overlap` pixels on each side, aligned (tiling.c ~590-790).

## GIMP (app/core/gimpprojection.c, gimpchunkiterator.c)
- Projection updates aligned to 32 × 32 chunks (GIMP_PROJECTION_UPDATE_CHUNK_WIDTH/HEIGHT).
- Chunk iterator renders as much as fits an interval of 1/15 s by default; target area = last_area × interval / elapsed, median of the last 3; minimum 4096 px per iteration; max ratio 2; priority rect (the viewport) first (gimpchunkiterator.c:38-54, 455-500).

## GEGL (gegl/gegl-config.c, buffer/gegl-tile-handler-cache.c)
- Default tile 128 × 128 (gegl-config.c:219-232).
- Default tile cache = total RAM, capped at available, at least 512 MB (gegl-config.c:236-348).
- LRU; idle "wash" writes the least recently used dirty tiles among the 20 % LRU to swap.

## Krita (libs/image/tiles3, kis_image_config.cpp, kis_update_scheduler.cpp; docs.krita.org/Instant_Preview)
- Tiles 64 × 64 (kis_tile_data_interface.h:24-25).
- Memory: hard limit 50 % of RAM by default, soft 2 %; swap up to 4096 MiB; swap slab 64 MiB, window 16 MiB.
- Instant Preview (LOD strokes): draws on a smaller version of the canvas while the real stroke is computed in the background; disabled by settings that make a stroke hard to predict ("popping" at the end); brush threshold 100 px.

## libvips (doc/how-it-works.md, iofuncs/cache.c, sinkscreen.c)
- Demand-driven partial images: start/generate/stop; intermediates small enough for L2; "four lock operations per output tile".
- Operation cache: last 100 calls, 100 MB, 100 files (cache.c:81-94); keyed by operation and arguments.
- sink_screen: display tiles with priority and LRU by ticks.
- SIMD via Highway: "typically speeds operations up by a factor of three or four".

## Chromium cc (docs/how_cc_works.md)
- Software raster tiles about 256 × 256; GPU raster tiles about viewport width × a quarter of viewport height.
- TileManager: task graph; running tasks cannot be cancelled; unstarted tasks are cancelled by submitting a new graph.

## Halide (PLDI 2013)
- Tension between parallelism, locality and redundant recomputation. Two-stage blur: breadth-first 1.0× work, total fusion 2.0×, tiled 1.0625×; tiled 10× faster than breadth-first.
- Local Laplacian: 99 stages, 52 lines, 1.7× faster than Adobe's expert C++ (113 vs 189 ms); GPU 7.5×.
- Camera pipeline 3.4× faster; bilateral grid 4.2× CPU, 2.3× vs CUDA.

## Photoshop on the web (web.dev/articles/ps-on-the-web; developer.chrome.com blog on files larger than memory)
- Emscripten, pthreads, SIMD: "3–4× speedup on average and in some cases a 80–160× speedup" with Halide.
- mmap not performant on the web; OPFS access handles; scratch files back VM pages; tiles are square regions of one layer occupying pages; a mipmap pyramid of tiles.
- Code caching reduced initialisation time by 75 %.

## Photoshop desktop (Adobe guide as quoted on community.adobe.com)
- Up to eight cache levels (default 4), four cache tile sizes; bigger tiles speed complex operations such as sharpening, smaller tiles keep brush strokes responsive; ~1 MP with 50+ layers: levels 1–2; 50 MP+: more than 4.

## Figma (figma.com/blog/figma-rendering-powered-by-webgpu)
- Explicit draw state; uniforms batched into one buffer per submit; GLSL → WGSL via naga; async readback; dynamic fallback to WebGL mid-session; blocklisting by fallback rate; compute-shader blur planned.

## Vello CPU (gaussian_blur.rs)
- σ ≤ 2 direct separable; larger: downsample → blur → upsample pyramid; [1,3,3,1]/8 decimation; σ = 100 → 6 decimations, kernel 11.

## Caching theory
- GreedyDual-Size (Cao & Irani, USITS 1997): locality + cost + size, no parameters.
- Build Systems à la Carte (Mokhov, Mitchell, Peyton Jones, ICFP 2018): dirty bits (Make, Excel), verifying traces (Shake), constructive traces (Bazel), deep constructive traces (Buck, Nix); deep constructive traces cannot support early cutoff.

## Timing and scheduling
- performance.now: 5 µs isolated, 100 µs non-isolated (MDN; Chrome 91 change).
- WebGPU timestamp queries quantised to 100 µs in Chrome.
- scheduler.yield: Chrome/Edge 129, Firefox 142, Safari not supported; continuation runs before other similar tasks. postTask priorities user-blocking, user-visible, background.
- INP: good ≤ 200 ms, poor > 500 ms.
- Coordinated omission (wrk2 README, Gil Tene): load generators that wait for responses ignore most high-latency artifacts; fix by constant-rate generation and measuring from intended start.

## WebGPU / WASM
- Default limits (MDN GPUSupportedLimits): maxTextureDimension2D 8192, maxBufferSize 256 MB, maxStorageBufferBindingSize 128 MB, maxComputeWorkgroupStorageSize 16384, invocations 256, workgroups per dimension 65535. web3dsurvey: 16384 storage 100 %, 32768 99 %.
- Wasm fixed-width SIMD: Chrome 91, Firefox 89, Safari 16.4.

## Pyramids
- Williams 1983 "Pyramidal parametrics": mip = multum in parvo; each level averaged down from its predecessor with box prefiltering; linear interpolation between two maps; "the most severe compromise ... is that it is symmetrical".
- Burt & Adelson 1983: Gaussian pyramid via REDUCE (5 × 5 weighted average), Laplacian pyramid = differences.
- Deep Zoom / libvips dzsave: tile 254 + overlap 1 = 256.
