# Tasks

## TODO

- [ ] Phase 1: pkg/cv/text 5x7 font (digits, letters, caption punctuation) with tests; delete kernels.DrawDigits and look/font.go glyphs <!-- t:tnuk -->
- [ ] Phase 1: graph.BindOptions; look options as ArgSpecs (label, zoom, interp, maxSide, ruler, grid, format); delete lookOptionKeys; synonyms; look({box}) literal fix to crop <!-- t:zug5 -->
- [ ] Phase 2: tests first - render_test.go (scale-8 pixel op equals nearest-upsampled scale-1; decorations get upsampled inputs; cache per scale+interp; scale limits) <!-- t:gg9r -->
- [ ] Phase 2: EvalOptions.Interp, OpDef.Decoration, upsample branch in evalRun.eval, buffer.Upsample, MaxRenderScale=16; look zoom and captions below scale 1 <!-- t:w7br -->
- [ ] Phase 3: crop op (frame, clipping, RANGE warning) and CROP_FIRST warning for Halo ops after crop (first reader of OpDef.Halo) <!-- t:tkq3 -->
- [ ] Phase 3: rulers and grid in look renderer (margins, tick positions, labels from frame) with golden PNG; op-chain captions with Node.Macro <!-- t:snac -->
- [ ] Phase 4: kernels.Binary; Image ops add, sub (derived), mul, abs, min, max with frame checks and gray broadcast; tests for sign and algebra <!-- t:gnzh -->
- [ ] Phase 4: blend keeps normal/multiply/screen (enum redirect fixes for removed modes); delete AbsDiff; compare as KindNone op with signed diff macro; prelude exposes KindNone ops as cv.* (__moduleOps); drop cvFuncs compare entry <!-- t:tze4 -->
- [ ] Phase 5: highpass and dog macros (polarity, space) with node-identity tests; localContrast kernel; levels({auto}) Global <!-- t:3sg2 -->
- [ ] Phase 5: img.view presets (scene, speck, speckDark, speckColor, signed, detail, neutral) in ops/views.go; preset docs-drift test <!-- t:9dpa -->
- [ ] Phase 6: overlay as decoration (screen-px outlines, text ids); annotate marks (circle, point, box, polyline, text) with goldens <!-- t:wife -->
- [ ] Phase 6: cv.sheet (Regions/points/boxes, views list, size, zoom, cols, labels, stretch cell; sheet frame; size errors); format auto rule <!-- t:ltem -->
- [ ] Phase 7: docs - topic 11-views, rewrite 06-looks, recipes (inspect candidates, signed compare, marks), MCP instructions, synonyms; js looks=N docs-test assertion <!-- t:lq2v -->
- [ ] Phase 7: gated view-ranking test on GOJACV_TILES with exported gold points (median rank <= 30, top50 >= 70%); scripts/01-export-gold-points.py <!-- t:sdul -->
- [ ] Phase 7: pi harness questions-gcv-002.md; run before/after twice; record calls, errors, looks, coordinate accuracy <!-- t:xxu1 -->
