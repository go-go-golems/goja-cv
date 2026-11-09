# Tasks

## TODO

- [ ] Phase 0: add TestKernelsAreSpaceAgnostic guard (go/parser over pkg/cv/kernels and pkg/cv/ops; skipped until Phase 3) <!-- t:qwuj -->
- [ ] Phase 0: remove Node.Base and unused error codes EStale, EReadOnly, EFragmentCapture, ENoModel, WScale <!-- t:539s -->
- [ ] Phase 1: tests first - buffer/encoding_test.go (identity table, round-trip property, 0.5->0.214 and 0.05->0.0039) <!-- t:ssjc -->
- [ ] Phase 1: implement buffer.Encoding, Space, Identity, Convert, Buffer.Space; Quantize replaces ToImage; encoders write encoded values <!-- t:kuqe -->
- [ ] Phase 2: internal/testimage writers for planted scan as linear TIFF16, float32 TIFF, LZW TIFF, npy (f4, u2), minimal LinearRaw DNG with SubIFD <!-- t:chw6 -->
- [ ] Phase 2: decoder tests - fixture equivalence within 2e-5, Probe fields, Box equals cut of full decode, FuzzTIFFProbe and FuzzNPYProbe <!-- t:nvjn -->
- [ ] Phase 2: decoder registry (png, jpeg, tiff with own IFD parser + x/image fallback for compressed, npy); delete Decode/DecodeFile/DecodeConfigFile <!-- t:65op -->
- [ ] Phase 2: Engine.Open(path, raw) and open op args encoding, box, origin, as; SourceInfo reports encoding, sample, frame, notes <!-- t:9lsj -->
- [ ] Phase 3: tests first - graph/space_test.go (defaults table, explicit space changes hash, shared convert node, no convert for linear encoding, DerivedEnc, SPACE and ENCODING warnings) <!-- t:30au -->
- [ ] Phase 3: replace OpDef.Space string with SpaceRule, add Result and Internal; Register appends the static space option; Intern resolves space and inserts convert nodes; Level unit <!-- t:8tbv -->
- [ ] Phase 3: port kernels (Levels, Contrast, PerceptualLuminance->Luminance, ColorMask, FloodMask, Histogram, DetectSpecks MeanL, parseColor) to space-agnostic; enable guard test <!-- t:zi95 -->
- [ ] Phase 3: look.Render renders encoded values via convert + Quantize; masks display linearly; update goldens in one commit <!-- t:j6q1 -->
- [ ] Phase 4: macro mechanism OpDef.Expand/Expander and array return in binding/prelude <!-- t:eqsm -->
- [ ] Phase 4: channel ops channel, split, toLab, chroma, colormap (kernels/colormap.go), encode, with tests (Lab reference colours, chroma of grey = 0) <!-- t:6fby -->
- [ ] Phase 5: Frame replaces DocW/DocH on Node, ImageValue, RegionsValue; XY/AxisScales/NewRegion/Rasterize honour origin; two-image ops compare frames <!-- t:i882 -->
- [ ] Phase 6: named images - session name table, cv.open({as}), E_NAME, context block names and encodings, MCP open tool encoding/as <!-- t:ge3c -->
- [ ] Phase 7: docs - topic 10-encodings-and-spaces, units/frames, getting-started names, looks, recipes; help concept entries; synonyms; MCP instructions; guidebook; README <!-- t:zztz -->
- [ ] Phase 7: gated real-data tests (GOJACV_TILES tile re-encoded as linear; GOJACV_DNG window open) <!-- t:r49f -->
- [ ] Phase 7: pi harness IMAGE/QUESTIONS overrides + questions-gcv-001.md; run before/after twice each; record table <!-- t:dgm6 -->
