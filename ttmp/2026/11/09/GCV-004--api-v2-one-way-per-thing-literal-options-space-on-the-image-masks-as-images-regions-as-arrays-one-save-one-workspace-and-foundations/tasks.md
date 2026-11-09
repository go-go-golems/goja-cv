# Tasks

## TODO

- [x] Design guide written (design-doc/01): architecture tour, v2 API, decisions, phases <!-- t:hz8q -->
- [x] Review read of the guide; revision 2 (all 37 findings dispositioned, Appendix A) <!-- t:h4ll -->
- [x] Phase 0a - v1 rendering bugs: sheet badge over the first label, colliding ruler labels; regenerate sheet-2x2, ruler-crop-zoom8, grid-gray-crop-zoom4 <!-- t:p0a1 -->
- [x] Phase 0b - foundations without API change: panic boundary (graph.Recover, binding.export with isGojaPanic; timeouts stay E_TIMEOUT), Record.MarshalJSON + dataout.plain, Render one exit with exact cuts (delete fitScale), typed SourceMeta (Path until Phase 6)/PlaceholderMeta (delete Hidden, trusted), smoke reads --json files (no --root yet) <!-- t:cxv1 -->
- [x] Phase 0c - Args redesign, no API change: node args as own inputs (no liftIdx), OpDef.Inputs become named node ArgSpecs, Args with Receiver/Input/Inputs/Value/Values and strict accessors, build-time functions take (a Args); registry property test <!-- t:p0c1 -->
- [x] Phase 1 - literal options: delete params, lifting, Static, Value ops, derived fields, SPACE warnings, E_LAZY_COMPARE; levelsAt; E_NOT_AWAITED after method lookup <!-- t:vb5d -->
- [x] Phase 2 - await: Values/Regions at scale 1, Images from statics (no evaluation); delete cv.evaluate, preview scale (also xgoja previewLong); look() queues with LOOKS warning (delete show); cv.explain; delete cv.describe, explain op, NoCache, cv.isNode, cv.warnings, cv.kinds, doc stub <!-- t:4ht1 -->
- [x] Phase 3 - space on the image: SpaceRule (Follows, Physical, Display), receiver decides, masks never converted, sources encoded, as() with openLinear and folding, brackets, one luminance (OpDef.Luma), colormap follows; delete space option; regenerate gaussian, levels-gamma, levels-stats, profile-chart <!-- t:45dv -->
- [x] Phase 4 - masks are one-channel images: Node.Channels (two-image ops take the larger count), OneChannel/ThreeChannel, Mask type word; delete KindMask; one crop, one overlay <!-- t:sqem -->
- [x] Phase 5 - regions as arrays: [node, index] item links, items op with moved-box check and ITEMS warning, Region.Shape Pixels/Box/Disk, measure(items) with radii from items and stepChroma gate, mask.regions; delete query ops, data lambdas, Produces, RemovedFields, MaxItems, item prototype, collisions; re-measure hot-list thresholds <!-- t:2oo1 -->
- [x] Phase 6 - one save, one workspace: graph.Workspace (os.Root --root + read-only --read roots, an engine option), mcp requires --root, decoders take files, cv.save (E_DUPLICATE, no symlinks, os.SameFile protection, temp+rename), CSV rows only, profile/histogram rows; xgoja root/read; delete Data handles, cv.json/csv, node.save, paths.go, out-root, uniqueName, data budgets <!-- t:dz7m -->
- [x] Phase 7 - names: keep only the typed names of guide 7.13; delete Synonyms, OptionSynonyms, ValueSynonyms and other redirect tables <!-- t:sxi3 -->
- [x] Phase 8 - documentation sweep (topics, guidebook, README, MCP instructions, reference) and pi validation runs 19-30 (guide section 15); deviations section <!-- t:umwo -->
- [x] Upload the guide to reMarkable <!-- t:6tle -->
