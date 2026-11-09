# Tasks

## TODO

- [x] Phase 1: pkg/cv/text 5x7 font from X11 misc-fixed 5x7.bdf (digits, letters, caption punctuation) with glyph tests; delete kernels.DrawDigits and look/font.go glyphs <!-- t:wz2z -->
- [ ] Phase 1: graph.FuncDef + RegisterFunc, graph.BindOptions, ArgSpec.Redirects; register look and show with ArgSpecs (label, zoom, interp, maxSide, ruler, grid, format); delete lookOptionKeys; synonym and redirect tests <!-- t:vvf2 -->
- [x] Phase 2: render_test.go (pixel op at 8 == nearest-upsampled scale-1 window; Regions/Values not upsampled; test-only decoration; memo keys scale/interp/window; Evaluate refuses >1; render >16 refused) <!-- t:yxkk -->
- [x] Phase 2: Engine.Render with RenderOptions{Scale, Interp, Window}, OpDef.Class (pixel, geometric, decoration), macro-node dispatch, buffer.Upsample, EvalCtx.Warn with cache replay; look zoom/interp and captions below scale 1 <!-- t:h78y -->
- [x] Phase 3: crop (geometric, box type, Args.Box, integer frame, RANGE/E_ARG_RANGE nearest valid box), CROP_FIRST (first reader of Halo) and DECORATION_FIRST warnings <!-- t:nm2q -->
- [x] Phase 3: rulers and grid in look/ruler.go (margins, absolute labels, thinning) with golden PNG; chain captions through macro nodes <!-- t:410s -->
- [x] Phase 4: kernels.Binary; Image add, sub (derived), mul, abs, min, max; Image|number union operand; frame checks; gray broadcast; tests incl. lifted Value SPACE <!-- t:fblc -->
- [x] Phase 4: blend keeps normal/multiply/screen with enum redirects; delete AbsDiff and img.compare; cv.compare module op (side, split) with picture frames and diff redirect; prelude builds cv.* from module ops and functions <!-- t:v6od -->
- [ ] Phase 5: highpass and dog macros (Space default encoded, polarity) with node-identity tests; localContrast kernel; levels auto (Optional black/white, Identity off with auto) <!-- t:2g2r -->
- [ ] Phase 5: img.view presets (scene, speck, speckDark, speckColor, signed, detail, neutral) in ops/views.go; every preset renders on the RGB planted scan <!-- t:55ax -->
- [ ] Phase 6: overlay as decoration with Rasterize(target Frame, scale), containing-frame inputs; annotate marks with goldens <!-- t:lrvd -->
- [ ] Phase 6: cv.sheet layout op (AnalysisScale 1, static max, labels in cells, groups for several views, picture frame, SHEET_TRUNCATED); format auto rule <!-- t:hw6o -->
- [ ] Phase 7: docs (topic 11-views with preset drift test, rewrite 06-looks, recipes, MCP instructions), synonyms, TestOpSynonymsDoNotShadowOps, UnknownMember synonym lookup, js looks=N <!-- t:8ec3 -->
- [ ] Phase 7: acceptance tests per issue (13.6); gated view-ranking test reading ~/dustds/gold and ~/dustdet/cv_tiles; pi harness questions-gcv-002.md before/after <!-- t:vamq -->
- [ ] Phase 5: rename gaussian r -> sigma, feather/grow/shrink r -> radius, highpass/dog/localContrast sigma names; reverse option synonyms (r -> sigma|radius) <!-- t:zsur -->
- [ ] Phase 6: sheet cells via Evaluator.Render at scale zoom*size/side with integer window origins; label size rule (2, 1, id only) <!-- t:rmcw -->
