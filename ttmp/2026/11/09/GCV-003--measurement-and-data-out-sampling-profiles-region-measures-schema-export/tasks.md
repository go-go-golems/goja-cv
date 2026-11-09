# Tasks

## TODO

- [ ] Phase 1: tests first - schema property (item keys == schema for every Regions op), build-time where/sortBy errors with producer hint, box/point coercion table, throwing box/centroid accessors <!-- t:byrt -->
- [ ] Phase 1: graph/schema.go FieldSpec + GeometryFields, Node.Fields, OpDef.RegionFields; flat Region.Data; delete RegionFields, KnownRegionFields, speckFields, FieldNames; box and point arg types; switch stats.box, mask.box/ellipse/flood/gradient, crop, annotate, sheet <!-- t:59jv -->
- [ ] Phase 2: remove SummaryLimit and truncated; MaxRegionItems safety limit (E_TOO_MANY); YAML truncation notes point to cv.json <!-- t:zcfq -->
- [ ] Phase 3: tests first - probe disk/annulus means equal StatsWeighted with ellipse masks; measure vs mem.probe mapping (coords +0.5, gray average/rec709); sample r=0; profile edge sigma recovery <!-- t:k557 -->
- [ ] Phase 3: kernels/probe.go (coverage-weighted sums, weighted quantiles); sample, profile, measure ops (space linear, analysisScale 1, gray option); merge StatsWeighted wrapper <!-- t:5eka -->
- [ ] Phase 4: regions.measure(image, {stats, channels, ring, core, prefix, space}) with tests against stats per region and measure at centroids; remove meanMask from components <!-- t:3sry -->
- [ ] Phase 5: OpDef.Chart (bars, lines); profile line chart with pkg/cv/text labels; look.New checks Chart instead of op name <!-- t:1mn4 -->
- [ ] Phase 6: graph.FuncDef module functions (move cvFuncs entries); pkg/cv/dataout Content provider; cv.json and cv.csv (lazy nodes, eager data, columns) <!-- t:zkad -->
- [ ] Phase 6: look/data budgets (MaxLooks enforced in collect; transport items = looks + data; 256 KiB MCP data limit with save hint); CLI uses resultcontent.WriteFiles (delete writeLooks) <!-- t:63tz -->
- [ ] Phase 6: node.save and content.save under --out-root (Options.OutRoot, containment with symlinks, encoding/frame metadata, CLAMP warning) with round-trip tests <!-- t:a9of -->
- [ ] Phase 7: --prelude on eval, run, mcp (Options.Preludes, evaluated at CreateSession, FILE:LINE errors) <!-- t:r6fu -->
- [ ] Phase 8: docs - topics 12-measuring and 13-data-out; flat fields in masks-and-regions, recipes, nodes topic; guidebook CLI flags; MCP instructions; synonyms (drop measure->stats) <!-- t:ufiy -->
- [ ] Phase 8: gated real-tile test (hot list ring measure: gold dust median snr >= 1.5, top-30 coverage >= 70%, < 200 ms); pi harness questions-gcv-003.md before/after <!-- t:c22y -->
