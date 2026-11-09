# Tasks

## TODO

- [x] Phase 1: tests first - schema property over the registry (item/row keys == node Fields), build-time where/sortBy errors with Produces hints, RemovedFields accessors, mask.gradient point|edge <!-- t:e7sc -->
- [x] Phase 1: graph/schema.go (FieldSpec, GeometryFields, RemovedFields); OpDef.Fields as func, Produces, Validate(args, inputs) for every validator; Node.Fields in internNode; flat Region.Data; delete RegionFields, KnownRegionFields, speckFields, FieldNames, meanMask; __fieldNames from specs <!-- t:wefh -->
- [x] Phase 2: remove SummaryLimit and truncated; MaxRegionItems safety limit (E_TOO_MANY); YAML truncation notes point to cv.json <!-- t:5pch -->
- [x] Phase 3: tests first - DiskCoverage vs EllipseMask; probe means vs StatsWeighted exactly; measure vs mem.probe mapping (+0.5 coords, gray average/rec709); sample radius 0; profile sigma recovery; ringCoverage; OUTSIDE warning <!-- t:n2qd -->
- [x] Phase 3: kernels.DiskCoverage (EllipseMask uses it), kernels/probe.go; sample, profile, measure ops (points only, space linear, analysisScale 1, gray option, naming table); merge StatsWeighted <!-- t:35jx -->
- [x] Phase 4: regions.measure(image, {stats, channels, ring, coreRadius, prefix, gray, space}) with Region.Coverage; tests vs stats per region and measure at centroids; E_FIELD_EXISTS, E_FRAME <!-- t:d4ya -->
- [x] Phase 5: OpDef.Chart (bars, lines); profile line chart with pkg/cv/text axis labels <!-- t:nq0p -->
- [x] Phase 6: writers - tiffwrite.go (uint8/16/float32, 1|3 samples, Deflate, tag 270), PNG chunk insertion (gAMA/sRGB/tEXt), NPY (<f4/<u2); round-trip table tests; delete buffer.WriteFile and EncodeTIFF16 <!-- t:ri8o -->
- [x] Phase 6: save FuncDef on Image/Mask and Data handle under --out-root (Options.OutRoot, shared containment helper with symlink test, EAccess moved to graph, CLAMP warning) <!-- t:gxf9 -->
- [x] Phase 7: pkg/cv/dataout Data handle; cv.json and cv.csv FuncDefs with the data argument type; budgets (MaxData, MaxDataBytes for mcp, ContentLimits sum) in collect <!-- t:w0kr -->
- [x] Phase 7: CLI writeContent (look-N-slug names, <name>.<ext> data files, dropped reporting) and --json files list; script 83 command in docs <!-- t:w62w -->
- [ ] Phase 8: --prelude on eval, run, mcp (Options.Preludes, evaluated at CreateSession, FILE:LINE errors) <!-- t:s5z6 -->
- [ ] Phase 9: docs - topics 12-measuring and 13-data-out; flat fields in masks-and-regions (lines 29, 35-40), recipes, nodes topic; guidebook CLI flags; MCP instructions; synonyms (drop measure->stats) <!-- t:5a6n -->
- [ ] Phase 9: gated real-tile test (snr/stepChroma ranges for the hand-over, top-30 coverage >= 70%), HotList benchmark >= 20x helper; pi harness questions-gcv-003.md before/after <!-- t:5bwg -->
- [x] Phase 6/7 (rev 3): data names as identifiers (default data-<N>); MaxDataBytes inside Data.ResultContent; masks saved with linear note; internal/testimage switched to new writers (keep DNG fixture writer); EllipseCoverage(rx, ry) <!-- t:aa69 -->
