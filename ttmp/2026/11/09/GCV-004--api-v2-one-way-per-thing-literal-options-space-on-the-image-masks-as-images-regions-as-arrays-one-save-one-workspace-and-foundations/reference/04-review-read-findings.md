---
Title: Review read findings
Ticket: GCV-004
Status: active
Topics:
    - api
    - architecture
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/design-doc/01-design-and-implementation-guide.md
      Note: the guide under review (revision 1, 2,170 lines); line numbers below refer to it
    - Path: pkg/cv/kernels/composite.go
      Note: matchChannels/binary broadcast a grey operand or receiver to RGB (F1)
    - Path: pkg/cv/graph/space.go
      Note: masks are EncNone/SpaceNone today and so never converted (F2); ENCODING fix names { space } (F7)
    - Path: pkg/gojamod/cv/prelude.js
      Note: link()/NODE and marshal's awaited-data branch, not in the deletion checklist (F17)
    - Path: pkg/xgoja/providers/cv/cv.go
      Note: a third front end with baseDir and previewLong config, absent from the guide (F8)
    - Path: pkg/cvsession/names.go
      Note: cv.open's { as } name binding collides with img.as(space) (F12)
    - Path: pkg/cvsession/processor.go
      Note: collect() drops looks past MaxLooks with only a placeholder note (F10)
    - Path: pkg/cv/ops/measure.go
      Note: stats meanL in encoded space is luma of codes, not encode(Y) (F15)
ExternalSources: []
Summary: "Cold review read of the GCV-004 design and implementation guide (revision 1): 37 findings (4 P1, 18 P2, 15 P3) on concept introduction, correctness against the code, design soundness, phasing, simplicity and the five v1 bugs, with recommendations on D3 and the six open questions."
LastUpdated: 2026-11-09T19:30:00-05:00
WhatFor: "Input for the writer's revision of the GCV-004 guide."
WhenToUse: "When revising design-doc/01 or before starting Phase 0."
---

# Review read findings (GCV-004 guide, revision 1)

## Goal

This document records a cold read of `design-doc/01-design-and-implementation-guide.md`, done as an intern new to
goja-cv would read it, then checked against the code on `task/gcv-004-api-v2` (`0bba23b`). Each finding has a
priority, a line reference into the guide, the problem, and a concrete fix the writer can apply.

- **P1**: blocks implementation, or the guide prescribes something wrong.
- **P2**: should be fixed in the revision.
- **P3**: nice to have.

## Context

The guide is strong. The architecture tour (§2) and the end-to-end trace (§2.11) let a newcomer follow the system,
the review bug classes map cleanly onto structural fixes (§3.1), and the decision records state their alternatives.
I checked more than 100 `path:line` citations and statements about current behaviour (grouped by file in the
diary, step 2). Four are wrong (F23), and every name in the deletion checklist exists in the code. The findings
below are mostly about cases the new model handles badly at its seams: masks that become images, items whose link
outlives an edit, and a workspace check that compares names instead of files.

One worry from the coordination diary is now settled. `os.Root.MkdirAll` through a symlinked parent that stays inside
the root works on macOS with Go 1.26.10, and a dangling link that points outside is refused (`path escapes from
parent`). The same test showed the symlink alias problem in F4.

## Quick Reference

| # | P | guide lines | finding (one line) |
|---|---|---|---|
| F1 | P1 | 509-510, 1128-1133 | channel count "the receiver's" is wrong for two-image ops: kernels broadcast grey to RGB |
| F2 | P1 | 486-500, 1068-1081, 1098 | mask inputs would be space-converted, which silently changes mask weights |
| F3 | P1 | 1170-1199, 1765-1778 | item links resolve by the mutable `id` field, and ids may repeat, so the wrong region can be used without any error |
| F4 | P1 | 877-878, 1293, 1651 | "never overwrite an opened image" compares names; an in-root symlink alias overwrites the scan (verified) |
| F5 | P2 | 1477, 1895-1905 | Phase 0 smoke snippet uses `--root`, which only exists from Phase 6 |
| F6 | P2 | 495, 1104, 913 | `colormap` as a Display op reads `range`/`center` in another space than the receiver's |
| F7 | P2 | 1080, 1855-1884 | ENCODING fix and ~32 op docs and hints still say `{ space }`, `.toMask()`, out-root; §12 misses in-code strings |
| F8 | P2 | 152-179, 957-964 | `pkg/xgoja/providers/cv` (third front end) missing: `baseDir`, `previewLong`, workspace |
| F9 | P2 | 1401-1414 | `isJSThrow` is undefined; interrupts must not become `E_INTERNAL` (E_TIMEOUT) |
| F10 | P2 | 832-834, 1623-1624, 1231-1235 | v1 bug: looks beyond `MaxLooks` dropped silently; the queue makes it worse |
| F11 | P2 | 1211-1215 | v1 bug: `stepChroma` unstable when step ≈ 0, not covered |
| F12 | P2 | 848, 893, 897 | `cv.open(p, { as })` binds a name; `img.as(s)` converts; `{ as: "linear" }` binds a global `linear` |
| F13 | P2 | 877-878 | refusing to overwrite any opened file blocks the save → reopen → save loop for the whole session |
| F14 | P2 | 1060-1063, 1605-1614, 2132-2134 | encoded sources plus brackets add full-size copies; at scale 1 this is ~0.6 GB each on a 53 Mpx scan |
| F15 | P2 | 743, 1527-1542 | 19 agent `toGray({space:"encoded"})` overrides unaddressed; two "luminances" in encoded space |
| F16 | P2 | 881-883, 1314-1321, 1642-1656 | `--read-only` inline delivery reintroduces a second output path; drop it |
| F17 | P2 | 1694-1706 | deletion checklist misses items the new model makes dead |
| F18 | P2 | 1907-1917 | Phase 1 is too large; the `Args` redesign is API-neutral and can be its own phase |
| F19 | P2 | 209 | the kinds (especially Value) are never defined |
| F20 | P2 | 520-528, 801 | edited linked items keep their old pixels; `map` to new objects silently degrades to boxes |
| F21 | P2 | 2013-2029 | v1 bugs: sheet badge and ruler labels; fixing them moves goldens against "only Phase 3" |
| F22 | P2 | 1642-1656, 2154-2155 | MCP default: writable current directory (also over HTTP); require `--root` for `mcp` |
| F23 | P3 | 1701, 1704, 1213, 1468 | four wrong citations |
| F24 | P3 | 160, 211, 361, 363, 402, 414, 482, 1080 | terms used before or without definition |
| F25 | P3 | 382-387 | trace step 10 omits the `Display` conversion |
| F26 | P3 | 992-1007, 1019 | `Args` sketch omits the accessors that stay |
| F27 | P3 | 1451-1469 | §7.13 drops typed keys `area`, `resample`; `lookRedirects` misdescribed |
| F28 | P3 | 1128-1141, 1941-1947 | ops that need three channels are not declared |
| F29 | P3 | 436-443 | "out of scope and unchanged" contradicts §7.6, §7.7, §7.9 |
| F30 | P3 | 1916, 1237-1239 | Phase 1 `__notAwaited` on Regions while Regions still has methods |
| F31 | P3 | 1170-1180, 1298 | `cv.save` must not marshal; strip `__cvref` from non-items arguments |
| F32 | P3 | 1276-1280, 1794-1810 | absolute paths through an outside alias; non-atomic writes |
| F33 | P3 | 1430, 1904 | Phase 0 `SourceMeta.Rel` before the workspace exists |
| F34 | P3 | 1888-1889 | `make logcopter-generate` not mentioned |
| F35 | P3 | 1294-1299 | `cv.save` type/extension mismatches unspecified |
| F36 | P3 | 1201-1205 | the `Point` region shape can go |
| F37 | P3 | 1231-1235 | look queue edge cases: preludes, late looks, duplicates |

## P1 findings

### F1 (P1). The channel count of two-image ops is not "the receiver's"

- **Where:** §4.4 lines 509-510 ("most ops keep their receiver's") and the §7.5 table, line 1133 ("everything else |
  the receiver's").
- **Problem:** `kernels.binary` calls `matchChannels` (`pkg/cv/kernels/composite.go:24-29`). When the channel counts
  differ, it converts both buffers to RGB. So `m.mul(img)`, `grey.add(colour)`, `grey.blend(colour)` and
  `grey.max(colour)` have **three** channels, while the static says one.
  - Such a node passes the build-time mask check (`within(m.mul(img), …)`).
  - At run time the kernel reads `Plane(0)` of an RGB buffer as weights.

  This is the silent wrong-value class that D4 is meant to remove. The property test (line 1135) catches it only if
  it combines a grey receiver with a colour operand, and the guide does not ask for that.
- **Fix:**
  - Add a row to the §7.5 table: "`add`, `sub`, `mul`, `min`, `max`, `blend` (and any op whose kernel uses
    `binary`): the maximum of the channel counts of its Image inputs".
  - Require the property test to iterate over every combination of grey and colour inputs for multi-input ops, not
    just for the receiver.

### F2 (P1). Mask inputs must never be space-converted

- **Where:**
  - §4.3 lines 488-490 ("Other image inputs are converted to that space first") and the physical ops (lines
    491-494);
  - the resolution pseudocode, lines 1075-1076;
  - output-space rule 5 (line 1098), which counts masks as derived, linear-encoded results.
- **Problem:** Today masks have `Enc = EncNone` and `Space = SpaceNone` (`space.go:236-238`). `buffer.Identity` treats
  `SpaceNone` as the identity (`encoding.go:84-87`), so a mask is never converted.

  In v2 any one-channel image is a mask. The guide itself suggests `img.toGray()` as the fix for `within` (lines 768
  and 1141), and that image keeps `enc srgb`. Under the following and physical rules, its weights are converted
  whenever the receiver is in another space:
  - `img.as("linear").within(img.toGray(), …)` converts the weights to linear;
  - `img.inpaint(img.toGray())` (physical, work space linear) converts them too, which moves the ≥ 0.5 threshold.

  Nothing reports either case. A mask is a weight, not a level.
- **Fix:**
  - Add one rule to §4.3, §7.4 and the pseudocode: "an input declared `Mask` (or the receiver of a mask op) is read
    as stored: the builder never inserts `as` for it."
  - Implement it in `resolveSpace` by skipping inputs bound to a `Mask`-typed spec and `OneChannel` receivers.
  - In rule 5, say what changes: mask makers become linear-encoded `Image`s; today they are `EncNone`/`SpaceNone`.
  - Add a Phase 4 test: `img.as("linear").within(img.toGray(), f)` uses the same weights as
    `img.within(img.toGray(), f)`.

### F3 (P1). Link items by position, not by the `id` field

- **Where:**
  - §4.5;
  - the marshal sketch, lines 1173-1177 (`__cvref: [x[ITEM], x.id]`);
  - the `items` op, lines 1196-1199 ("an id may repeat"; literal entries carry their "id if given");
  - §10.3.
- **Problem:** The link stores only the node, and the region is then looked up by the item's `id` **field**, which
  the agent can change. Three ordinary flows pick the wrong region without any error:
  1. **Renumbering for labels:** `top.map((r, i) => ({ ...r, id: i + 1 }))`. The spread keeps the link, but the ids
     now name other regions of the source.
  2. **Mixing linked and literal items:** `img.measure([...linked, { id: 3, x, y, w, h }])`. The resulting Regions
     node has two items with id 3 and different geometry. When filtered items from it are passed back, `byID[3]`
     returns one of the two.
  3. **Repeated linked items, which the guide allows, followed by `measure`:** the result again has repeated ids.
- **Fix:** Make the link `[nodeId, index]`, where the index is the item's position in the awaited `RegionsValue`.
  - Set it in `linkItems`: `it[ITEM] = [node.__id, i]`.
  - Send `__cvref: [node, index]`.
  - Store `{ index }` entries in the `items` op.

  Positions are unique, and editing fields cannot change them, so ids become data only. Literal items keep any `id`
  they carry. Add a Phase 5 test: spread items with new ids still resolve to their own pixels.

### F4 (P1). The opened-image overwrite check compares names, not files

- **Where:**
  - §6.7 lines 877-878;
  - §7.8 line 1293 ("rel is the path of an image this session opened");
  - §10.4 line 1787;
  - D11 line 1651 ("the session can never overwrite an image it opened").
- **Problem:** I ran a scratch program against `os.Root` in Go 1.26.10, with `w/alias.tif → src.tif` (a symlink
  inside the root):
  - `root.WriteFile("alias.tif", …)` succeeds and replaces `src.tif`. `os.Root` permits it because the link stays
    inside the root.
  - The name check passes, because `alias.tif ≠ src.tif`.
  - Hard links have the same effect.

  So D11's promise is false as designed, and the input scan is the one file the workspace must protect.
- **Fix:**
  - At `cv.open`, record the `fs.FileInfo` from `root.Stat(rel)`. In `save`, call `root.Stat(rel)` (which follows
    links inside the root) and refuse the write if `os.SameFile` matches an opened file.
  - Better still, refuse to write through a symlink at all: if `root.Lstat(rel)` reports `ModeSymlink`, return
    `E_ACCESS` "saves do not follow links".
  - Add both cases to the Phase 6 workspace tests, which today list symlink cases for escaping only.

## P2 findings

### F5 (P2). The Phase 0 smoke target uses a flag that Phase 6 adds

- **Where:** §7.14 line 1477 (`eval --root $(SMOKE_DIR) …`) and Phase 0 (lines 1895-1905, "the smoke target").
- **Problem:** `eval` has no `--root` until Phase 6. The guide says so itself in §5.9 line 642: "The CLI's eval and
  run have no read root at all". So Phase 0 is not green as written.
- **Fix:**
  - In Phase 0, drop `--root` from the snippet and keep `--image`, `--out`, `--json` and the two `jq` checks.
  - Add `--root $(SMOKE_DIR)` in Phase 6, and mention it in that phase's implement list.

### F6 (P2). `colormap` should follow its receiver and output a picture

- **Where:** §4.3 line 495 and §7.4 line 1104 list `colormap` as a Display op; §6.9 line 913 says it "needs one
  channel".
- **Problem:** Display converts inputs to encoded values. `colormap`'s `range` and `center` are `Level` arguments
  (`channel.go:148-150`).
  - Suppose an agent measures `p99` on `img.as("linear").toGray()` and passes it as `range`. It gets colours from
    encoded values against a linear number, which breaks rule 6's promise (measure, then use the number).
  - On derived and mask inputs the conversion is the identity, which is why the goldens would not catch this.
- **Fix:**
  - Make `colormap` `Follows` with `Result: DisplayEnc`, so it computes in the receiver's space and returns an sRGB
    picture.
  - In the rule text, write: "Display ops: overlay, annotate, compare, sheet. colormap follows its receiver and
    returns a picture."
  - Check the `signed`, `detail` and `neutral` view presets. They call `colormap` on derived images, so nothing moves
    there.

### F7 (P2). The ENCODING fix and many in-code strings still teach removed APIs

- **Where:** §7.4 line 1080 ("ENCODING warning: unchanged") and §12 (lines 1855-1884).
- **Problem:** The ENCODING warning's fix is `op({ space: "linear" })` (`space.go:136`), which is the removed option.
  Agents read the strings in the code more than the topics, and many of those strings name removed APIs:
  - about 32 `OpDef.Doc`/`ArgSpec.Doc` strings in `pkg/cv/ops` and `pkg/cv/graph` say "in the working space (linear
    by default)" or `{ space }`;
  - `kindHint` and `conversionHint` say `.toMask()` (`args.go:392-402`, `suggest.go:270-282`);
  - `saveRedirect`, `csvRedirect` and `jsonRedirect` name the out-root and `cv.csv` (`suggest.go:98-104`);
  - the lift error's fix is `img.stats().p01` (`args.go:367-368`);
  - the MCP `open` tool's `As` description says `as` (`pkg/cvsession/mcp.go:50`).
- **Fix:**
  - Make the ENCODING fix `x.as("linear").<op>(…)`.
  - Add a row to §12 for "op and argument docs, error hints and fixes in Go", owned by the phase that changes the
    behaviour.
  - Add a CI grep test that fails on `{ space`, `toMask`, `cv.csv`, `cv.json`, `out-root`, `P.slider` and `show(`
    in non-test Go and JS strings.

### F8 (P2). The xgoja provider is a third front end the guide does not mention

- **Where:** the §2.1 layer diagram (lines 152-179) and the §7.1 table (lines 957-964).
- **Problem:** `pkg/xgoja/providers/cv/cv.go` exposes the `cv` module to binaries that xgoja generates.
  - Its config has `baseDir`, `cacheBytes`, `previewLong` and `lookLong`, and it builds `graph.EngineOptions`
    itself. Phase 2 deletes `PreviewLong`, so this config stops compiling.
  - Phase 6 needs a workspace for `cv.open` and `cv.save` in a runtime without `cvsession`. The guide does not say
    where the `Workspace` lives: in `EngineOptions`, or in the module.
- **Fix:**
  - Add `pkg/xgoja` to the layer diagram.
  - State that the `Workspace` is an engine option, so the module works without a session.
  - In the provider's config, replace `baseDir` with `root` (plus `readOnly` if F16 is not adopted) and drop
    `previewLong`.
  - Add the provider to Phases 2 and 6.

### F9 (P2). Specify `isJSThrow`; interrupts must stay E_TIMEOUT

- **Where:** §7.11 lines 1401-1414 ("a goja.Value (b.throw), *goja.Exception, or another goja-internal panic").
- **Problem:** "Another goja-internal panic" is the case that matters.
  - The watchdog interrupts the VM (`session.go:376-405`), and goja unwinds an interrupt through native frames as a
    panic that JavaScript cannot catch. That panic has to propagate unchanged.
  - Native functions that read agent objects can run JavaScript, and that JavaScript can be interrupted. Examples
    are `exportOrdered` calling getters and the output of `marshal`.
  - If the wrapper turns the interrupt into `E_INTERNAL`, a timeout is reported as an internal error and the eval
    keeps running.
- **Fix:**
  - List the types that are re-panicked: `goja.Value`, `*goja.Exception`, `*goja.InterruptedError`, and goja's
    uncatchable wrapper. Check the goja version in `go.mod` for that type's name, and add a compile-time reference
    to it.
  - Add a Phase 0 panic test: "a timeout while `cv.save` exports a getter-heavy object is `E_TIMEOUT`".

### F10 (P2). v1 bug: looks past `MaxLooks` are dropped silently, and the queue makes it worse

- **Where:** §6.5 lines 832-834, D9 line 1624 ("Budgets already handle too many looks with a note"), §7.7 lines
  1231-1235, and the Phase 2 test "the fifth look gets the budget note".
- **Problem:** `collect` records the reason only for providers that appear in the returned value
  (`processor.go:124-169`, `r.dropped`). Coordination diary step 4 shows what the user sees: extra looks listed
  without a number, and no warning.

  In v2 most looks are queued without being returned. A dropped look then has no placeholder at all and disappears
  silently. D9's premise is false.
- **Fix:**
  - In §7.7, say that dropped looks produce one `LOOKS` warning in the warnings block: "4 of 6 looks delivered;
    dropped: line 7 `look("b")`, line 9 `look("c")`. Return one `cv.sheet`, or look in two evals."
  - Add `LOOKS` to the warnings list in §11 (line 1852).
  - Change the Phase 2 test so it asserts the warning for a look that was queued but not returned.
  - Render only the looks that are delivered.

### F11 (P2). v1 bug: `stepChroma` is unstable when step ≈ 0

- **Where:** §7.6 `measure` (lines 1211-1215). Not covered anywhere.
- **Problem:** `stepChroma = (max − min of the channel steps) / max(|step|, 1e-3)` (`measurement.go:155`). At
  snr 0.12 the colleague got 3.25, which reads as a strongly coloured speck. Phase 5 rewrites `measure`, so it is the
  natural place for the fix.
- **Fix:**
  - In Phase 5, either divide by `max(|step|, 2·spread)` (noise-relative), or report `null` when `snr < 2`. NaN
    becomes null through `plain` (§7.9).
  - Document the chosen rule in the field doc.
  - Add a test on a flat patch with per-channel noise: `stepChroma` is null or below 1.

### F12 (P2). `as` means two things

- **Where:** §6.6 line 848 (`cv.open(path, { as })`) and §6.9 lines 893 and 897, against `img.as(space)` (line 905).
- **Problem:** `cv.open("t.tif", { as: "linear" })` is a natural way to write "open in linear space".
  - It passes `checkName`, because `linear` is an identifier, and silently binds a global called `linear`.
  - This is a new silent failure, created by v2's choice of `as` for the space op.
  - It also breaks "one spelling, one meaning".
- **Fix:**
  - Rename the binding option to `name` (`cv.open(path, { name: "ref" })` in `init.js` and `names.go`, where
    `asSynonyms` is deleted), and rename the MCP `open` tool's `As` field.
  - Passing `as` then gives `E_UNKNOWN_OPTION` with the edit-distance or typed-name fix, like any other unknown key.
  - Alternatively, keep `as` and refuse the values `linear` and `encoded` with the fix `cv.open(p).as("linear")`. The
    rename is simpler.

### F13 (P2). "Never overwrite an opened file" blocks iteration

- **Where:** §6.7 lines 877-878.
- **Problem:** Agents iterate:
  1. save `views/speck.tif`;
  2. reopen it to check it;
  3. save it again with other parameters in the next eval.

  After the first reopen, every later save to that path is `E_ACCESS` for the rest of the session. Yet this reopen
  workflow is what D11 exists to enable (the 23 `E_ACCESS` errors).
- **Fix:**
  - Protect only files the session opened and did not write itself, using a per-session `written` set.
  - Say what happens when a save replaces a file that a live node opened. The source store keys by stamp, so the old
    node would read the new file under its old key. Either re-probe and fail with `E_FORMAT` (or a stale-source
    error), or keep the earlier decoded pyramid pinned. Pick one and test it.

### F14 (P2). Encoded sources and physical brackets multiply full-size buffers

- **Where:**
  - §7.4 lines 1060-1063: the source's `Run` converts the linear pyramid level to encoded values, which is a copy;
  - D8, lines 1605-1614;
  - risk 4, lines 2132-2134, which covers time only.
- **Problem:** On a 7256 × 7326 RGB scan, one float32 buffer is about 0.64 GB.
  - Today a scale-1 evaluation of the source returns the pinned pyramid level, with no copy.
  - In v2, `open` at scale 1 adds an encoded copy, which is cached.
  - Every physical op on an srgb image then adds an `as(linear)` copy of that copy (numerically the pyramid level
    again), plus the closing `as(encoded)`.
  - `img.exposure(1).stats()` on such a scan therefore holds about four full-size buffers against a 2 GiB cache.
  - D8 moves every `await` to scale 1, where all of these copies are full size.
- **Fix:**
  1. Have the builder rewrite `as(open(…), linear)` into an internal `openLinear` source node whose `Run` returns the
     pyramid level, with no copy. This is one rule in the builder, next to the bracket.
  2. Fold `as(as(x, s1), s0)` back to `x` when `x.Space == s0`.
  3. Extend risk 4 to cover memory and cache pressure, not only time.
  4. Adopt open question 4 (awaiting an Image returns statics), which removes the most common full-size await.

### F15 (P2). Two "luminances" in encoded space; the toGray evidence is not addressed

- **Where:** §6.2 line 743 and D3 lines 1527-1542.
- **Problem:** The evidence table shows **19** `toGray({ space: "encoded" })` overrides (`reference/01`), more than
  the 11 for `exposure` that D3 cites. D3 never mentions them.

  Beyond that, v2's numbers disagree with each other in encoded space:
  - `img.stats().meanL` is the luma of the codes: "In encoded space meanL is the Rec. 709 weighted sum of encoded
    channels" (`stats` doc, `measure.go:136`).
  - `img.toGray()`, `mask.luminance`, `histogram` and `detect.specks` use encode(Y_linear), through the physical
    bracket.

  An agent that writes `mask.luminance({ above: s.p99 })` with `s = await img.stats()` mixes the two. This breaks
  rule 6's promise that the numbers you measure are numbers you can pass.
- **Fix:**
  - Choose one luminance for encoded space and use it everywhere. The smallest change consistent with D3 is to make
    `stats` compute its luminance fields through the same Gray rule (encode(Y_linear)). The `levels-stats` golden
    moves in Phase 3 anyway.
  - Add a sentence to D3 about the 19 `toGray` overrides: `toGray("average")` follows its receiver, and is the luma
    to use for an agent that wants code values.

### F16 (P2). Drop `--read-only` and inline delivery

- **Where:** §6.7 lines 881-883, the §7.8 flags (lines 1314-1321), D11, and the pseudocode at line 1300.
- **Problem:** This brings back a second output path:
  - content items named by path;
  - CLI files under `--out`;
  - `inline: true` results;
  - a format whitelist;
  - `MaxData`, `MaxDataBytes` and `MCPMaxDataBytes`, and the `numbered` interface.

  It exists for a person who wants to protect a directory. That person can point `--root` at a scratch directory,
  and once F4 is fixed the session cannot overwrite its inputs. D10 itself says that data in the conversation is what
  the eval returns.
- **Fix:**
  - Delete `--read-only` and inline delivery, so `cv.save` always writes or fails.
  - Delete the data-content budgets and `numbered`, and add them to §9.
  - Phase 6 loses its read-only tests.
  - If a real need appears, it can be added later with evidence.

### F17 (P2). Deletion checklist gaps

- **Where:** §9 (lines 1694-1706).
- **Problem:** The checklist misses several things that the new model makes dead:
  - `link()`'s `NODE` symbol, its method attachment, and the "awaited data → its node" branch of `marshal`
    (`prelude.js:48-53`, `72-87`, `124`; item 5 cites only `55-70,88-116`);
  - the mask branch of `ImageValue.Summary` (`value.go:91-96`);
  - `conversionHint` and the Mask/Regions branches of `crossKind` (`suggest.go:231-282`); `kindHint` is listed;
  - `OpDef.NoCache`, which only the `explain` op uses (`meta.go:13`);
  - the `Hidden` skip in `FuncDef.UsageLine` (`funcs.go:58-60`);
  - `checkOutRoot` (`cmd/goja-cv/eval.go:303-311`), `EngineOptions.OutRoot`, and `Options.Roots`, `OutRoot` and
    `BaseDir` (`session.go:61-68`), once the workspace replaces them;
  - the xgoja `previewLong` and `baseDir` config (F8);
  - `init.js`'s two uses of `cv.isNode` (lines 39 and 72), which break when Phase 2 deletes `cv.isNode`;
  - `binding.lazy` and `__lazy`, which Phase 1's text names but §9 does not;
  - `graph.Kinds` and `cv.kinds`;
  - `Evaluator.SetCallbacks` and the `params` parameter of `NewEvaluator`;
  - with F16: `MaxData`, `MaxDataBytes`, `MCPMaxDataBytes`, `numbered` and `SetNumber`.
- **Fix:** Add these rows to §9, and add each one to the phase that makes it dead.

### F18 (P2). Split Phase 1: the `Args` redesign does not change the API

- **Where:** Phase 1 (lines 1907-1917). The writer's own diary flags this.
- **Problem:** Phase 1 combines eight changes:
  1. deleting params;
  2. deleting lifting and the Value ops;
  3. `OpDef.Inputs` becoming named `ArgSpec`s;
  4. the new `Args` with inputs and values;
  5. strict accessors;
  6. a new signature for every build-time function in every op file;
  7. `levelsAt`;
  8. `E_NOT_AWAITED`.

  Changes 3 to 6 alter nothing an agent sees, and they are the riskiest mechanical part. A failure in them would be
  tangled with the deletions.
- **Fix:**
  - Move changes 3 to 6, together with "every node argument is its own input (no `liftIdx`)", to Phase 0 or to a
    new Phase 0b, with the registry property test as its gate.
  - Phase 1 then contains only the deletions, `levelsAt` and `E_NOT_AWAITED`.

### F19 (P2). Define the kinds before the registry table uses them

- **Where:** §2.3 line 209 (`On | the receiver kind: Image, Mask, Regions, Value, Param`).
- **Problem:** Value, Regions and Param are used throughout §2 and §5. A Value node is never defined; it is plain
  data, such as a stats record, a profile or a number. Regions are first explained in §4.5.
- **Fix:** Before §2.3, add a short table of the kinds and say which of them v2 keeps:
  - Image: pixels;
  - Mask: weights;
  - Regions: a set of found areas, with their pixels;
  - Value: plain data, such as numbers, records and rows;
  - Param: a value a person moves.

### F20 (P2). Edited and rebuilt items: the link is kept or lost without notice

- **Where:** §4.5 lines 520-528 and §6.4 line 801 ("`filter`, `sort`, `slice`, `map` and spreading keep the link").
- **Problem:** There are two opposite silent cases:
  - **The link is lost.** `map` keeps the link only if its callback returns the same object or a spread of it.
    `sp.map(r => ({ x: r.x, y: r.y, w: r.w, h: r.h, id: r.id }))` loses it, and the items become literal boxes. `heal`
    and `overlay` then work on boxes instead of the speck's pixels.
  - **The link is kept but the edit is ignored.** A spread with edited geometry, for example
    `{ ...r, x: r.x - 4, w: r.w + 8 }` to make the box "a bit larger", keeps the link, so the region's original
    pixels are used.
- **Fix:**
  - Correct line 801: `map` keeps the link only when it returns the item or a spread of it.
  - Send each linked entry's `x, y, w, h` with the entry. In `items.Run`, compare them with the region's box, and on
    a mismatch return `E_ARG_TYPE`: "item 3 was moved; pass a plain box { x, y, w, h } to use the new geometry, or
    the item unchanged to use its pixels".
  - Add an `ITEMS` warning when a list contains unlinked objects that carry region fields (`id` and `area`): "these
    look like region items but lost their link; they are drawn as boxes".

### F21 (P2). v1 bugs: sheet badge and ruler labels, and the golden rule

- **Where:** §3.3 (sheets and looks out of scope) and §14.3 lines 2013-2029 ("Only Phase 3 may change them").
- **Problem:** Coordination diary step 4 lists two rendering bugs, and the guide mentions neither:
  - the sheet badge covers item 0's label;
  - ruler labels collide at the start of an axis ("16191651").

  Fixing them moves `sheet-2x2`, and probably `ruler-crop-zoom8` and `grid-gray-crop-zoom4`. That contradicts "only
  Phase 3 moves goldens".
- **Fix:**
  - Add a "Phase 0a: v1 bug fixes" (or a separate commit before Phase 0) with:
    - a badge placed where it cannot cover the first cell's label, or the label moved below the badge;
    - ruler labels that skip a tick when its label box would overlap the previous label or the other axis's first
      label;
    - a test that rendered label boxes never intersect;
    - an explicit regeneration of the affected goldens, listed in the diary.
  - Amend §14.3: "Phase 0a and Phase 3 may move goldens; each lists the files and the reason."

### F22 (P2). MCP should require `--root`

- **Where:** D11 (lines 1642-1656), risk 5 (lines 2135-2136) and open question 6 (lines 2154-2155).
- **Problem:** Today `mcp` reads anywhere and writes nowhere by default; without `--out-root`, saving is disabled
  (`mcp.go:81-82`). v2 flips this to read and write the current directory.
  - For Claude Code, the current directory is the project root, so agents will write TIFFs into source trees.
  - `mcp` can also serve over HTTP (`serveHTTP`, `mcp.go:124`).
  - The CLI is different: a person typed the command in that directory.
- **Fix:**
  - Make `goja-cv mcp` require `--root`, with a usage error that names the flag.
  - Keep the current directory as the default for `eval` and `run`.
  - Update the setup pages `pkg/doc/05` to `07` (§12 already lists them).

## P3 findings

### F23 (P3). Wrong citations

- Line 1701 (§9 item 6): `pkg/cv/graph/funcs.go:56-83,114-119` should be `pkg/cv/ops/funcs.go:56-83,114-119`, the
  `FuncDef`s for `evaluate`, `describe`, `isNode`, `warnings`, `json`, `csv` and `show`. Lines 56-83 of the graph
  file are `UsageLine` and `RegisterFunc`.
- Line 1704 (§9 item 9): `funcs.go:21-27` is `pkg/cv/ops/funcs.go:21-27` (`lookRedirects`).
- Line 1213: `coreRingFields` is at `pkg/cv/ops/measurement.go:140`, not `regionmeasure.go:76-104`.
  `newRegionFields` is at `regionmeasure.go:79`.
- Line 1468: `lookRedirects` holds `box`, `crop` and `stretch`. `scale` is an `OptionSynonyms` key (`args.go:828`).

### F24 (P3). Terms used before or without a definition

- "replapi" (lines 160 and 361): add one sentence on go-go-goja's REPL session API.
- "the session's name table" (line 363).
- "a sheet" (line 402): define a contact sheet in §2.7.
- "pi runs" (line 414): the pi coding-agent CLI, which the harness in §15 drives.
- "the colleague's density tiles" (line 482): point to §16.
- "working space" (line 211): it is used before §2.2 and §4.2 define spaces.
- "ENCODING warning" (line 1080): first defined in §11.
- `isJSThrow` (line 1406): see F9.

### F25 (P3). Trace step 10 skips the display conversion

- **Where:** lines 382-384.
- **Problem:** The gaussian is in linear space, so the look does not render it directly. It renders
  `Engine.Display(id)`, which interns `convert(gaussian, encoded)`. This matters for the v2 change at step 4, where
  the conversion disappears. The trace also skips the eval tracker and warnings.
- **Fix:** In step 10, add "the look renders `convert(…, encoded)` (`engine.go:171-184`)", and add one line on
  warnings.

### F26 (P3). The `Args` sketch omits the accessors that stay

- **Where:** lines 992-1007 and 1019.
- **Fix:** List the accessors that stay: `Raw`, `Number`, `Int`, `Bool`, `Numbers`, `Strings`, `Object`, `Length`,
  `Lengths`, `Area`, `Box`, `XY` and `DocLength`. Say which of them already panic and which change.

### F27 (P3). §7.13 against the evidence script

- **Where:** lines 1451-1469.
- **Problem:**
  - The script's typed keys include `OptionSynonyms:area` and `OptionSynonyms:resample`. §7.13 neither keeps them
    nor explains why they go.
  - The table mixes typed table keys (14) with wrong method names that were never table keys (`persist`,
    `download`, `toFile`, `writeTiff`), so "14 of 246" and the table do not reconcile.
- **Fix:** Split the table into "typed keys kept" and "typed wrong names given a fix", and account for all 14 keys,
  including the ones dropped.

### F28 (P3). Ops that need three channels

- **Where:** §7.5 (lines 1128-1141) and the Phase 4 tests (lines 1941-1947).
- **Problem:** `toLab`, `mask.color`, `split`, `channel("g"|"b")` and `chroma` are meaningless or fail on a
  one-channel image. Only `channel("g")` and `colormap` are tested.
- **Fix:**
  - Add `OpDef.ThreeChannel`, or express channel requirements as data.
  - Make the failure a build-time `E_WRONG_KIND` with the fix "this image has one channel".
  - Test each op.

### F29 (P3). §3.3's scope statement contradicts later sections

- **Where:** lines 436-443, which say that looks, sheets and charts are unchanged.
- **Problem:** §7.7 queues looks, §7.6 removes `MaxItems` from sheets, and §7.9 makes charts read `rows`.
- **Fix:** Name these three changes in §3.3.

### F30 (P3). Phase 1's `E_NOT_AWAITED` on Regions

- **Where:** line 1916 and §7.7 lines 1237-1239.
- **Problem:** Phase 1 adds `__notAwaited` for Regions nodes, but Regions keep their methods until Phase 5.
- **Fix:** Say that `lookup` resolves methods first and calls `__notAwaited` only for non-method properties. Phase 5
  then removes the methods.

### F31 (P3). Marshalling boundaries

- **Where:** lines 1170-1180 and 1298.
- **Problem:**
  - `cv.save`'s plain-data path must read the goja value directly (`exportOrdered`), not the marshalled form.
    Otherwise `__cvref` ends up in the JSON and CSV files.
  - Arguments other than items (`annotate` marks, `object` options with strict keys) would refuse `__cvref` as an
    unknown key.
- **Fix:** State both rules. The binder drops `__cvref` from every argument that is not an items argument.

### F32 (P3). Workspace path edge cases

- **Where:** lines 1276-1280 and 1794-1810.
- **Problem:**
  - An absolute path through an alias outside the root that resolves into it is refused. Example:
    `/home/me/dustdet/x.tif`, with `/home/me/dustdet → /data/dustdet` and `--root /data/dustdet`. This is
    acceptable, but the guide should say so.
  - `root.WriteFile` truncates the file in place, so a failed TIFF encode or a crash leaves a partial file.
- **Fix:** Write to `rel + ".tmp-<n>"`, then move it into place with `root.Rename`.

### F33 (P3). `SourceMeta.Rel` in Phase 0

- **Where:** line 1430 and Phase 0 (line 1904).
- **Problem:** Phase 0 introduces `SourceMeta` with a workspace-relative path, but the workspace only arrives in
  Phase 6.
- **Fix:** Say that the field holds the absolute path until Phase 6, or name it `Path` and keep it.

### F34 (P3). Logcopter

- **Where:** lines 1888-1889.
- **Problem:** `make logcopter-check` fails when instrumented functions move or are deleted.
- **Fix:** Mention `make logcopter-generate` in the per-phase loop.

### F35 (P3). `cv.save` mismatches

- **Where:** lines 1294-1299.
- **Fix:** Specify the error for each mismatch. Each is `E_ARG_TYPE` with the matching fix
  (`cv.save("x.json", …)` or `cv.save("x.tif", node)`):
  - plain data with an image extension;
  - a Value or Regions node with `.tif`;
  - an Image node with `.json`.

### F36 (P3). Drop the `Point` region shape

- **Where:** lines 1201-1205.
- **Fix:**
  - Only `measure` and `sample` need centres. Let them take `items|point[]`, as `sample` already does (type
    `Regions|point[]`).
  - Limit `Region.Shape` to `Pixels` and `Box`.
  - This removes the "these items are points" error class from `overlay`, `heal`, `mask.regions` and `stats`.

### F37 (P3). Look queue edge cases

- **Where:** lines 1231-1235.
- **Fix:** State three rules:
  - looks created while a prelude runs are discarded;
  - looks created after the eval settles (in an unawaited `.then`) are dropped with a note;
  - two looks of the same node with equal options are delivered once.

## Usage Examples

### The five v1 bugs (coordination diary step 4)

| v1 bug | covered? | where it fits |
|---|---|---|
| bare `.look()` dropped silently | yes | D9, the §7.7 queue, Phase 2 test "a look created but not returned is delivered" |
| looks beyond `MaxLooks` dropped without a warning | no: D9 assumes the note suffices | F10: `LOOKS` warning in §7.7 and §11, Phase 2 |
| `stepChroma` unstable when step ≈ 0 | no | F11: Phase 5 (`measure` unification), field doc, flat-patch test |
| sheet badge covers item 0's label | no | F21: Phase 0a bug fixes, with an explicit `sheet-2x2` golden move |
| ruler labels collide at the axis start | no | F21: Phase 0a, label-box intersection test, ruler goldens |

### Recommendations on D3 and the open questions (§17)

- **D3 (`exposure`, `heal`, `inpaint`, Rec. 709 luminance, Lab and `encode` are physical): keep.**
  - "+1 stop" means double the light. A following `exposure` would compute something else under the same name.
  - The bracket returns file units, so the agents who asked for encoded `exposure` (11 times) still see a brighter
    image, in the numbers they expect.
  - Two conditions: make the bracket cheap (F14: `as(open, linear)` reuses the pyramid level), and address the 19
    `toGray` overrides by unifying luminance (F15).
- **Q1 (should `gaussian` be physical?): no, keep it following.**
  - All 10 explicit choices were encoded.
  - In analysis chains (DoG, thresholds), `gaussian` smooths the values that are then thresholded.
  - The speck view already brackets itself with `as("linear")`, and an optical blur is one documented
    `as("linear")` away.
- **Q2 (should `blend` and `within` be physical?): no, keep both following, provided F2 holds** (a mask is never
  converted).
  - Agents chose encoded for `blend` 7 times out of 8.
  - `within`'s edit runs inside the lambda, so a physical edit such as `exposure` stays physical.
- **Q3 (should `ring` default to `true`?): only when `stats` is not given.**
  - With an unconditional `true`, `img.measure(await img.measure(items), { stats: ["mean"] })` fails with
    `E_FIELD_EXISTS` on the ring fields, and plain `stats` calls pay for ring measurements.
  - The conditional rule fits in one sentence of the docs: "measure measures the ring unless you ask for stats".
  - Requiring one of the two options explicitly would also work, but it loses `img.measure(points)`.
- **Q4 (should awaiting an Image return its statics without evaluating?): yes.**
  - Awaiting an Image never returns pixels, and after F1 everything in its summary is static: `w` and `h` at scale 1
    (the frame), `channels`, `encoding`, `space`, `frame` and the file metadata.
  - It makes the most common full-size await free: `await img`, and the MCP `open` tool's `await opened`. This
    removes most of risk 4.
  - Chain errors still surface at the first look, save or measurement, with their node path.
- **Q5 (should items from two region sets be accepted?): keep refusing them for now.**
  - `detect.specks({ polarity: "both" })` covers the common bright-plus-dark case.
  - `overlay`, `heal` and `measure` compose in two calls.
  - With F3 (links by `[node, index]`), accepting several sources later changes only the `items` op: one source
    raster per entry instead of one per value.
  - Make the refusal's fix specific to the op: `img.overlay(a).overlay(b)` for `overlay`, and
    `detect.specks({ polarity: "both" })` for `cv.sheet`.
- **Q6 (the MCP workspace default): require `--root` for `mcp`, and keep the current directory for `eval` and `run`**
  (F22).

## Related

- Guide under review: `design-doc/01-design-and-implementation-guide.md` (revision 1, commit `411ed29`).
- The five v1 bugs: coordination diary step 4, `reference/02-diary-coordination.md`.
- Evidence: `reference/01-evidence-pi-run-api-usage.md` and `scripts/02-pi-names-typed.py`.
- Review diary, with the list of checked citations and the `os.Root` test: `reference/05-diary-review-read.md`.

## Summary

37 findings: **4 P1**, **18 P2**, **15 P3**.

**P1 findings:**

- F1: channel statics for two-image ops;
- F2: mask inputs must not be space-converted;
- F3: link items by position, not by `id`;
- F4: the overwrite check must compare files, not names.

**Recommendations:**

- D3: keep, with F14 and F15.
- Q1 and Q2: keep `gaussian`, `blend` and `within` following.
- Q3: `ring` defaults to `true` only when `stats` is not given.
- Q4: yes, return statics.
- Q5: keep refusing mixed sets.
- Q6: require `--root` for `mcp`.
