# Tasks

## TODO

- [x] Design guide written (design-doc/01): architecture tour, v2 API, decisions, phases <!-- t:hz8q -->
- [ ] Review read of the guide; revision <!-- t:h4ll -->
- [ ] Phase 0 - foundations without API change: panic boundary (graph.Recover, binding.export), Record.MarshalJSON + dataout.plain, Render one exit with exact cuts (delete fitScale), typed SourceMeta/PlaceholderMeta (delete Hidden, trusted), smoke reads --json files <!-- t:cxv1 -->
- [ ] Phase 1 - literal options: delete params, lifting, Static, Value ops, derived fields, SPACE warnings; OpDef.Inputs become named node ArgSpecs; strict Args accessors with Inputs/Values; build-time functions take (a Args); levelsAt; E_NOT_AWAITED replaces E_LAZY_COMPARE <!-- t:vb5d -->
- [ ] Phase 2 - await at scale 1 only (delete cv.evaluate, preview scale); look() queues (delete show); cv.explain replaces cv.describe and explain(); delete cv.isNode, cv.warnings, cv.kinds, doc stub <!-- t:4ht1 -->
- [ ] Phase 3 - space on the image: SpaceRule enum (Follows, Physical, Display), sources in encoded space, as() op, physical brackets, speck views via as(linear); delete space option; regenerate 4 goldens (gaussian, levels-gamma, levels-stats, profile-chart) <!-- t:45dv -->
- [ ] Phase 4 - masks are one-channel images: Node.Channels static, OneChannel receivers, Mask type word; delete KindMask; one crop, one overlay <!-- t:sqem -->
- [ ] Phase 5 - regions as arrays: items type and internal items op, Region.Shape, item links (prelude), measure(items) on Image, mask.regions; delete where/sortBy/top/only/without/count/filter, data lambdas, Produces, RemovedFields, MaxItems, item prototype, collisions <!-- t:2oo1 -->
- [ ] Phase 6 - one save, one workspace: graph.Workspace (os.Root), decoders take files, cv.save with E_DUPLICATE and inline read-only delivery, CSV rows only, profile/histogram rows; flags --root, --read-only; delete Data handles, cv.json/csv, node.save, paths.go, out-root, uniqueName <!-- t:dz7m -->
- [ ] Phase 7 - names: keep only the typed names of guide 7.13; delete Synonyms, OptionSynonyms, ValueSynonyms and other redirect tables <!-- t:sxi3 -->
- [ ] Phase 8 - documentation sweep (topics, guidebook, README, MCP instructions, reference) and pi validation runs 19-30 (guide section 15); deviations section <!-- t:umwo -->
- [ ] Upload the guide to reMarkable <!-- t:6tle -->
