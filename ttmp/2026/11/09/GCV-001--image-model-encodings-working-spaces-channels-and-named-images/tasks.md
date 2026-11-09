# Tasks

## TODO

- [x] Phase 0: golden-look harness pkg/cv/look/golden_test.go with testdata/golden and -update flag (source, exposure, gaussian, contrast, levels, mask ops, specks overlay, histogram, compare modes) <!-- t:1pu0 -->
- [x] Phase 0: TestKernelsAreSpaceAgnostic guard (go/parser over pkg/cv/kernels and pkg/cv/ops; skipped until Phase 3); remove Node.Base and unused codes EStale, EReadOnly, EFragmentCapture, ENoModel, WScale <!-- t:sjd1 -->
- [x] Phase 1: encoding_test.go (identity table, exact round trips within 1e-6 relative, 0.5->0.214, 0.05->0.0039, integer tables vs exact functions) <!-- t:bqeo -->
- [x] Phase 1: buffer.Encoding, Space (linear, encoded), ParseSpace, Identity, Convert(ctx, b, enc, from, to) with exact transfer functions; keep ToImage <!-- t:8y6b -->
- [ ] Phase 2: Frame replaces DocW/DocH on nodes, values and Regions; DocFrame replaces DocSize; evaluator stamps value statics; goldens unchanged <!-- t:vncu -->
- [ ] Phase 2: box and point argument types (arrays, {x,y,w,h} boxes, {cx,cy} points; {x,y} refused as point); switch stats.box, mask.box, mask.ellipse, mask.flood, mask.gradient points; Args.XY shifts by origin <!-- t:a2zx -->
- [ ] Phase 3: space_test.go (defaults table, hash, shared convert, linear identity, grey-input rule on pure red, DerivedEnc, within lambdas on srgb and linear, compare with mixed spaces, SPACE identity case, ENCODING) <!-- t:77c3 -->
- [ ] Phase 3: SpaceRule, Result func, Gray, Internal; Register appends static space option and checks Run xor Expand; Intern resolves space, inserts toGray/convert; internNode computes Enc/Space/Frame; placeholder copies statics; Level unit and warnings <!-- t:urz2 -->
- [ ] Phase 3: port kernels (Levels, Contrast, ColorMask, FloodMask, Histogram, DetectSpecks MeanL, parseColor); delete PerceptualLuminance; enable guard test <!-- t:50sk -->
- [ ] Phase 3: look.Render via convert(->encoded) + Quantize; WriteFile writes encoded values; delete ToImage; regenerate goldens (only levels and mask looks may differ) <!-- t:55lv -->
- [ ] Phase 4: internal/testimage writers (linear TIFF16, float32 TIFF, LZW+predictor TIFF, npy f4/u2, minimal LinearRaw DNG with SubIFD) <!-- t:mbgj -->
- [ ] Phase 4: decoder tests (fixture vs generating float within one code, Probe, Box, ImageDescription and PNG tEXt/gAMA -> Info.Encoding/Origin, FuzzTIFFProbe, FuzzNPYProbe) <!-- t:mb5z -->
- [ ] Phase 4: decoder registry; png (chunk scan), jpeg, tiff (own IFD parser, LZW/Deflate/PackBits, predictor, SubIFDs, windows), npy; delete Decode/DecodeFile/DecodeConfigFile; open args encoding, box, origin; SourceInfo encoding/sample/frame/notes <!-- t:fgoz -->
- [ ] Phase 5: macros (Expand, Expander, macro node for one-node results, CallResult, array return in binding/prelude) with tests <!-- t:pqq3 -->
- [ ] Phase 5: channel ops channel, split, toLab (+PRIMARIES warning), chroma, colormap (kernels/colormap.go), encode; remove toGray rec601 <!-- t:hadh -->
- [ ] Phase 6: named images (session name table, init.js wrapper with as and synonyms, E_NAME, unnamed opens in context block, MCP open encoding/as, bare module rejects as) <!-- t:lvq2 -->
- [ ] Phase 7: docs (topic 10-encodings-and-spaces, units/frames, getting-started, looks, recipes, guidebook, README), help concept entries, scoped value synonyms, MCP instructions <!-- t:6dl3 -->
- [ ] Phase 7: gated real-data tests (GOJACV_TILES re-encoded tile, GOJACV_DNG window) <!-- t:7kb7 -->
- [ ] Phase 7: pi harness IMAGE/QUESTIONS overrides and questions-gcv-001.md in the dust-scanning repo; two runs per build; record results <!-- t:t65h -->
- [ ] Phase 2: graphtest.RegisterSource test-only source op (hidden enc, frame, fill) for linear and non-zero-origin inputs in Phases 2-3 <!-- t:g4kg -->
- [ ] Phase 3: golden tolerance rule (levels/mask looks change; others <=1 code on <=1% samples; equal speck counts), then regenerate with -update; evaluator stamps copies for returned inputs; macro nodes take expansion statics; Gray positions incl. refine <!-- t:k7bx -->
