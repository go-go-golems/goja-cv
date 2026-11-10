---
Title: Review read findings
Ticket: GCV-005
Status: active
Topics:
    - api
    - agents
    - analysis
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-005--agent-ergonomics-and-analysis-primitives-script-libraries-compact-results-peaks-morphology/design-doc/01-design-and-implementation-guide.md
      Note: the guide under review (revision 1, 2,471 lines, at f676f60)
    - Path: pkg/cv/graph/order.go
      Note: CROP_FIRST lint that fires on the guide's crop-first patterns (F1)
    - Path: pkg/cv/kernels/mask.go
      Note: Grow/Shrink edge arithmetic that makes the dilate equivalence test fail (F2)
    - Path: pkg/cvsession/yaml.go
      Note: maxStringLen and maxArrayItems, the caps the agents actually hit (F4)
    - Path: pkg/cvsession/session.go
      Note: runPreludes evaluates through Process, so preludes consume the new-function docs (F3)
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-evals
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/various/agent-diaries
Summary: "Cold review read of the GCV-005 design guide (revision 1): 4 P1, 12 P2, 17 P3 findings with line references and fixes, and recommendations on the four open questions."
LastUpdated: 2026-11-09T21:30:00-05:00
WhatFor: "Revising the GCV-005 guide to revision 2."
WhenToUse: "Before revising the guide or starting any GCV-005 phase."
---

# Review read findings

## Goal

Actionable findings from a cold read of `design-doc/01-design-and-implementation-guide.md` (revision 1, at
`f676f60`), judged against the user's principles: simple, composable, consistent APIs; no compat code and no option
where JavaScript or an existing op does the job; never make a model read a number off a picture. Every finding has a
priority, the guide's line numbers, the problem and a concrete fix.

Priorities: **P1** blocks implementation or is wrong; **P2** should be fixed in revision 2; **P3** nice to have.

## Context

What was checked, and how:

- **Citations.** More than 60 `path:line` citations, all against this branch's code (unchanged since `3ff1f59`):
  `session.go` (297-302, 311-319, 207-250, 84-88, 136-144, 265-268, 223, 437-447), `init.js` (66, 88-105),
  `processor.go` (64-92, 251-316, 266-270, 361, 503-557), `evaltrack.go` (24-28, 149-160), `native.go` (136,
  144-187), `yaml.go` (22, 23, 66, 111-113, 155, 162-188, 210-223, 243-265, 286-303), `probe.go` (178-198),
  `mask.go` kernels (183, 337-449), `measurement.go` (117-124, 138-159), `morph.go` (14-107), `specks.go` (106, 166,
  182), `ops/mask.go` (301-341), `regions.go` (366), `heal.go` (35), `views.go` (201-216), `schema.go` (32-43),
  `detect.go` (38-40), `sheet.go` (113-137, 172-190, 272), `present.go` (18, 66-89), `reference.go` (16-46),
  `mcp_test.go` (177-178), `prelude.js` (290-336), `binding.go` (104-130), `README.md` (81, 98), `03-cli.md` (73-75,
  131), `regions_test.go` (74), `golden_test.go` (129-132), topics 02 line 7 and 05 line 17, go-go-goja
  `replsession/evaluate.go:676`. **All were accurate.** The writer's code reading is careful.
- **goja `Function.prototype.toString`.** Ran `go run ./cmd/goja-cv eval -e …` in this worktree: arrow, `async`
  arrow, `function` and single-parameter forms return their exact source with `//` and `/* */` comments. Bound and
  native functions return `function bound f() { [native code] }` / `function max() { [native code] }`. Method
  shorthand returns `m() { // doc m … }`, which §10.4's parser does not handle (F6). The claim of §4.2 holds.
- **Evidence.** Recounted from `various/agent-evals/evals.jsonl` and, because its `result_head` keeps only the first
  eight lines of each result, from the 39 raw transcripts that `scripts/91-extract-agent-evals.py` reads
  (`~/.claude/projects/…/f3ad297e…/subagents/agent-*.jsonl`). Several counts the guide takes from the hand-off are
  wrong (F4, F5).
- **The UNUSED_NODE re-analysis.** Verified: every false warning is `speckColor` from `mem.views()`, bound to a local
  `const v`, of which the eval returned `v.scene.look(…)` and `v.speck.look(…)`. The node was neither returned nor
  kept in `mem`, so the writer's reading (attribution, not reachability) is right. The count is not (F5).
- **Behaviour.** Ran the current binary on a real tile: `img.crop(b).view("speck")` and `mask.crop(b).grow(2)` both
  raise `CROP_FIRST … put crop last` (F1); `measure` of 20,736 points costs 237 ms at core 3 and 39 µs per point at
  core 8 on 10 cores (F13).

## Quick Reference

### P1 — blocks implementation or is wrong

**F1. The guide's crop-first patterns trip the existing `CROP_FIRST` lint, whose fix says the opposite.**
Lines 638-639, 890, 1004-1005, 1264, 1328, 1540-1542, 1661, 2238, 2370, 2377, 2398.
`orderWarnings` (`pkg/cv/graph/order.go:18-38`, unchanged on `origin/task/gcv-004-api-v2`) warns for every
`PixelOp` (the default `Class`) on an Image or Mask receiver that has a `Halo` and a crop in its receiver chain:
"X after crop sees only the window (and its mirrored border); put crop last. fix: img.X(…).crop(b)". `Halo` is read
by nothing else; GCV-004 says windowed halo evaluation is future work. Consequences:
- `peaks` is given a `Halo` (line 1264), so `lin.crop(b).peaks()`, the guide's replacement for the `box` option (D13),
  its §6.8 example, its §16 `mem.best` and `mem.sweep` rows and its B2 test all raise `CROP_FIRST` with the fix
  `img.peaks(…).crop(b)`, which is meaningless for a Regions result.
- The morphology ops get a `Halo` (line 1328), and risk 3 recommends "crops before morphology on scans", which the
  lint then contradicts.
- §6.10's documented "local stretch" `img.crop(b).view("speck")` (also D21, line 1661) already raises
  `CROP_FIRST` today (checked with the binary). The doc note would teach a pattern the system tells agents to undo.

Fix: (a) give `peaks` no `Halo`, or make `orderWarnings` skip ops whose `Returns` is not Image/Mask, and say in §6.8
that peaks on a crop deliberately see only the window (`ringCoverage` flags edge rings); (b) for morphology, keep the
`Halo` (the lint is right there) and change risk 3's mitigation to "crop the result" or to the cost limits of F14;
(c) rewrite §6.10 and D21 to recommend a crop-last local stretch, `img.view("speck").crop(b).levels({ auto: true })`
or `levels({ black: 0, white: k × (await c.stats()).p99 })`, which is exactly what `mem.sweep` did; drop the
`img.crop(b).view("speck")` sentence and fix the §16 `mem.sweep` row.

**F2. `dilate(r)`/`erode(r)` do not select the pixels `grow(r)`/`shrink(r)` selected; the B1 property test fails.**
Lines 503-504, 976-978, 994-996, 1621-1623, 2219-2220, 2278.
`Grow` writes `max(m, clamp01(r + 1 − d))` (`pkg/cv/kernels/mask.go:424`), so its value is ≥ 0.5 exactly where
d ≤ r + 0.5; `Shrink` writes `min(m, clamp01(d − r))` (`mask.go:444`), ≥ 0.5 where d ≥ r + 0.5. Disk dilation selects d ≤ r and
erosion d > r. At r = 2 the pixel at offset (2, 1) (d = 2.236) is in `grow(2)` at ≥ 0.5 and not in `dilate(2)`. So the
test "`dilate(r)` selects the pixels `kernels.Grow(r)` selected at ≥ 0.5" fails for every r, and "the same pixels"
in §6.9, D18 and §14.3 is false by half a pixel.
Fix: compare `dilate(r)` with the pixels where `Grow(r)` equals 1 (d ≤ r) and `erode(r)` with `Shrink(r)` equal to 1
(d ≥ r + 1 versus d > r; state the difference), or state "`grow(r)` at 0.5 is `dilate(r + 0.5)`" and test that. Say in
the migration text that masks grow by half a pixel less than before.

**F3. A prelude consumes the "new functions" docs, so agents in the recommended setup never see them.**
Lines 744-748, 778-779, 1162-1170, 1184-1189, 2131.
`runPreludes` runs each prelude through `App.Evaluate` (`session.go:226-250`), so the result processor and its context
block run for the prelude eval, whose output is discarded. §7.4 updates `fnSeen` whenever the context block is built.
The prelude eval therefore marks every library function as seen, and the agent's first eval lists none under
`new functions:`. The prelude is the setup the guide recommends (§6.3, §12.5 step 1), and D8's rationale is that the
docs appear "right after the load".
Fix: prelude evals do not update `fnSeen` (or `fnSeen` is cleared after `runPreludes`), so the first agent eval of a
session shows the doc line of every `mem` function. Add the case to the A1 tests ("a prelude's functions appear with
doc lines in the first eval's context block").

**F4. The cut the agents hit was the 2,000-character string cap and the 48-item array cap, never the 9,000-character
result cap.**
Lines 126, 233-236, 326, 355, 563-566, 788, 1462-1463, 1471-1472, 2326.
In the 374 raw results the longest is 4,701 characters, and `# CUT` never appears. The "cut at about 60 rows" of
c71dd0f2 trial 2 eval 10 is `maxStringLen` (`yaml.go:24`, note at `yaml.go:153-155`) cutting the joined string:
`… (99 more characters; return cv.json(x) for all)`. The agent's diary says "truncated at 9000 chars"; the hand-off and
the guide repeat it. The array cap (`… N more (return cv.json(x) for all)`) fired in 40 evals. This changes what the
table design must guarantee and what Phase V measures.
Fix: correct §1 item 3, §3.1, issue 9, §5.4, §6.5's comment, D9 and the §15.1 metric ("results cut: 1 string cut and 40
array cuts in 374"). State in §6.5/§7.5 that a table is exempt from `maxArrayItems`, `maxStringLen` and `maxProperties`
(it is built as a `yaml.Node`, never through `str`), and add a test that a 200-row table is not cut at 48 rows or at
2,000 characters. The tables design itself is the right answer to both real caps.

### P2 — should fix

**F5. Several evidence counts are wrong.** Lines 108, 298-299, 322-326, 336-338, 349, 354, 548-549, 827, 884, 1017,
1350, 1450, 1477, 2323-2328.
- **Pastes.** All 39 agents pasted the helpers, not 29. The extractor flags a paste by the string
  `helpers ready (v1)` (`HELPER_MARK`, script 91); 10 agents dropped the final `return` and were counted as user code.
  Pastes are 2,859–3,454 characters; non-paste evals are 335 (median 181 characters), not 345 (183).
- **Regex counts** over the 335 true non-paste evals: `.map(` 100 (not 110), `.slice(` 72 (82), `.filter(` 55,
  `mem.probe(` 42 (52), `mem.hotByLevel` 44 (54), `aspect` 21 (31): each guide number includes the 10 unflagged pastes.
- **Warnings.** 19 `UNUSED_NODE`, not 13: 11 in eval 2 (no line), 7 in eval 1 (`line 10`, inside the paste, which was
  that eval's own code), 1 correct (58f9e866 eval 8). So 18 false, not 12. Attribution alone does not remove the 7
  eval-1 warnings; `cv.load` plus attribution removes all 18 (the guide's design claim survives with the right
  numbers). "Most carry no line" is 11 of 19. `RANGE` is 11 warnings in 9 evals (crops up to 200 × 340, not only
  48–72 px zooms); `OUTSIDE` does not occur.
- **28 of 32.** In cfca85b4 eval 8, 24 of 32 probes have sp below 0.06; 28 have sp below 0.1. Lines 108, 298-299 and
  884 say "below 0.06".
- Hand-typed coordinate lists probed in a loop: 22 agents (the "25" of §15.1 counts every probe loop).
Fix: correct the numbers; note in §3.5 that the extractor's paste flag and its eight-line `result_head` undercount, and
tell the coordinator (DUST-002 `reference/11` and `summary.md` carry the same errors).

**F6. The doc-line rule takes comments from the middle of a body and does not parse method shorthand.**
Lines 441-456, 1432-1445, 1781-1798.
"The first comment inside the function's source" picks any comment. In the real helper file
(`DUST-002/scripts/84-cv-helpers.js`), `mem.probe`'s first inner comment is `// best by its snr; we report its step`
(line 56, after code) and `mem.hot`'s is `// keep centroids inside the tile …` (line 75): both would be shown as the
functions' docs. §10.4 also falls through to "the identifier before `=>`" for method shorthand
(`probe(x, y) { … }`), the natural form for D2's "returns an object of functions", and for getters.
Fix: the doc line is a comment that comes before the first token of the body (after `{` or after `=>`, only
whitespace and comments in between); otherwise none. Parse method shorthand (`name(params) {`, `async name(…)`) and
skip `get`/`set`. Add both to the A2 tests, with the real helper file as a fixture.

**F7. No cache plus identity-based "new functions" repeats the docs after every reload.**
Lines 699-701, 1186-1188, 1381-1391, 1447-1458.
D3 runs a file on every `cv.load`, so a pipeline that loads its library (`await cv.load("dust/common.js")` at its top)
replaces every `mem` function on every call; §7.4 then lists all of them under `new functions:` in every eval, about
800 characters each time, which D8 set out to avoid.
Fix: compare the function's source text (`toString`), not the object: a key is "new" when its text is new or changed.
Store `fnSeen map[string]string`. This also keeps a re-run prelude quiet.

**F8. A load still running at the end of the eval leaks into later evals; the `LOAD` machinery is more than needed.**
Lines 708-709, 1172-1182, 1702, 2425-2426.
The guide warns "had not finished" and leaves the load running. Its later nodes, looks and errors then land in the
next eval's trace and results, attributed to nothing the agent wrote there.
Fix, simpler and complete: (1) `__cv_run` (`init.js:88-105`) awaits `Promise.allSettled` of the loads the eval started
before it settles, so the "pending" case disappears; (2) replace the load-specific rejection check with goja's
`Runtime.SetPromiseRejectionTracker` (present in the pinned goja, `builtin_promise.go:668`, unused today): a promise
rejected with no handler by the end of the eval gives one generic warning with its location. That covers
`cv.load`, `cv.save` (open question 4) and any helper's promise with one mechanism, and needs no `loadRec.Promise`
bookkeeping.

**F9. `E_TOO_MANY` counts concurrent loads, not nesting.** Lines 702-703, 1702-1704, 1978, 2186.
`Promise.all(tiles.map(t => cv.load("p.js", { t })))` over 33 tiles fails with "does a file load itself?" though
nothing recurses, and unsettled loads from earlier evals stay in `b.active`. The hazard that needs a limit is
synchronous recursion (a file that loads itself before its first `await`), which grows the Go stack through the
native call; asynchronous recursion is bounded by the eval timeout.
Fix: count synchronous nesting depth (increment before running the file function, decrement when it returns its
promise); at depth 32 throw `E_TOO_MANY` with the exact chain, which is then a stack. Drop `b.active`.

**F10. Attribution also silences the agent's own mistake of dropping a library's result.** Lines 841-844,
1207-1216, 1475-1490.
`mem.spots(0.3)` without `await` or `return` builds its node inside the library, so it is never reported, and that is
the forgotten-await mistake the lint exists for. Sites alone cannot tell it from the `speckColor` case.
Fix (or record the trade-off in D10): record on each site the outermost eval frame (the eval line that made the call).
Report library-built nodes grouped by that line only when *nothing* that call built was used; `mem.views()` with two of
three views used stays silent, `mem.spots(0.3)` alone warns "line 1: mem.spots(0.3) builds nodes and discards them".

**F11. Two phase dependencies are wrong.** Lines 2152-2164, 2177-2179, 2224-2225, 2465, 2468.
- A2 is said to need "nothing beyond Phase 1", but its paging and cut notes name `cv.save`, which arrives in GCV-004
  Phase 6 (Appendix A says so, line 2468). A2 landing "now" would print a function that does not exist.
- B1 is said to need Phase 4, but it deletes `mask.regions`' `grow`, which GCV-004 Phase 5 introduces. Landing B1
  before Phase 5 leaves Phase 5 to add the option back.
Fix: A2's notes use whatever data-out call exists when it lands and change with Phase 6 (or A2 waits for Phase 6). For
B1, ask GCV-004 now not to add `mask.regions(items, { grow })` in Phase 5 (the guide's own migration is
`mask.regions(items).dilate(r)`); then B1 needs only Phase 4. Update §13's table and Appendix A.

**F12. The snr thresholds come from random places, but peaks are local maxima.** Lines 925-926, 1532-1534,
1546-1547, 2353, 2391-2393, 2423-2424.
Peaks are the maxima of the step within r, refined to the sub-pixel point of highest step: their snr distribution is
biased upward against random places ("90 % below 0.45"). The false-positive rate at `snr: 1` is unknown, and D13's "a
few hundred peaks" is a guess.
Fix: make §15.2 measure the relevant null: peaks per tile at `snr` 0.5, 1, 1.4 and 2 that are not within max(r, 3) px
of a gold point, next to gold coverage at each level (an ROC over the threshold). Say in §6.8 that the random-place
numbers bound the probe, not peaks.

**F13. Peaks on a full scan at analysis scale 1 is unbudgeted.** Lines 941, 1292-1296, 1854-1875, 1952-1953.
Measured here: `measure` costs about 11 µs per point at core 3 and 39 µs at core 8 (10 cores, includes JS overhead).
With the guide's ~25,000 NMS candidates per 1024² tile at r = 2, a 53-megapixel scan has ~1.3 million candidates per
radius: 15–30 s for `radius: [2, 4, 8]`, against a 60 s eval timeout. `diskSums` as written materializes float64
`sum`/`cnt` planes: three disks × two planes × 424 MB on such a scan.
Fix: give the cost on a 53 MP scan in §10.7/§10.12; compute the step map row by row from one shared prefix-sum table
(count analytic away from the border) so no `sum`/`cnt` plane is stored; and either document "peaks on a crop or a
tile" or make the op refuse more than N candidates per radius with an `E_TOO_MANY`-style error that names the crop fix.

**F14. Morphology radius up to 500 is O(r) on grey images.** Lines 986, 1323, 1907-1908, 2397-2400.
The chord method costs 2R + 1 `minMax1D` passes per row: r = 500 on a 53 MP scan is about 2 × 10¹¹ operations.
`grow` allowed 2,000 because the distance transform does not depend on r; the fast path keeps that only for 0/1
planes.
Fix: cap the grey radius at what the chord method does in a few seconds on a scan (about 64 doc px; dust is ≤ 64 px by
the gold radii), keep a large cap for binary planes, and say so in the range error.

**F15. `opening` and `closing` are ops, but by D19's rule they should not be.** Lines 141-145, 983-996, 1599-1612,
1630-1640, 2037.
D19 refuses a top-hat op because "a top-hat is exactly one existing op applied to the result of another". An opening
is exactly `x.erode(r).dilate(r)` (same element, same angle), so the same rule removes it. Keeping four ops and
refusing the fifth applies the principle unevenly, and D17's naming debate (`open` vs `opening`) exists only because of
the extra ops.
Fix (recommended): two ops, `erode` and `dilate`; help concept entries `opening`, `closing`, `tophat` with the
compositions (`x.erode(r).dilate(r)`, `x.sub(x.erode(r).dilate(r))`). If opening stays an op, D19 must say why it
differs from top-hat, and then a `tophat` op is equally justified.

**F16. Central terms are used long before they are defined.** Lines 274, 292-299, 337, 354, 622, 635-637, 941, 1063.
For an intern: the **speck view** is used from §2.7 (line 292) and throughout §3, but defined only in §5.9 (line 635);
**sp** (its core/ring step) from line 298/337, defined in the §5.8 table (line 622); **doc px** (line 274) is never
defined (only inside the instructions text, line 2022); **NMS** (line 1063) is spelled out only at line 1293;
**analysis scale** (line 941) and **`Halo`** (line 1264) are never explained, and the latter matters (F1).
Fix: add to §2 a short "speck view and sp" paragraph and one-line definitions of doc px and analysis scale; spell out
non-maximum suppression in §4.5; define `Halo` where §7.8 uses it.

### P3 — nice to have

**F17. Appendix A and the base are stale.** Lines 74, 93, 2450-2461. `origin/task/gcv-004-api-v2` is at `7554bd1`:
Phase 0a (merged PR #5), Phase 2 (look queue, `LOOKS`) and Phase 3 (space, `as`, `Luma`, `LumaOf`/`LumaWhen`,
deviation 20) are implemented, and `processor.go`'s functions moved by about 50 lines (`unusedNodes` 311,
`contextBlock` 554, message already "look at it"). Rebase the branch and update the table before Phase A2.

**F18. Undefined or unexplained terms.** xgoja (lines 118, 164), hash-consed (700, 1388), MAD (268), builder
placeholders / `onPlaceholder` (1216), data handles (225), provider (1194), `RANGE`/`OUTSIDE` (326), trial 2/3 (321),
`mem.hotByLevel` (338, not in the §5.8 table). One line each, or a short glossary after §4.

**F19. `kernels.Peaks` cannot call `ProbePoint` as sketched.** Lines 1289, 1294, 1844. `ProbePoint` takes a
`*buffer.Buffer` and `CoreRingOptions{Gray}` (`probe.go:198`), not `L []float32, w, h`. Say that `Peaks` wraps `L` in a
one-channel buffer at scale 1, or take a buffer.

**F20. The step-map cost is about twice the stated one.** Line 1873. Three digital disks of radii r, 1.5 r, 2.5 r have
2⌊ρ⌋ + 1 chords each, two lookups per chord: about 20 r lookups per pixel (66 at r = 3, ~70 million per tile), not
10 r.

**F21. The line element's length is stated two ways.** Lines 499-500, 987-988, 1915. "2 r + 1 doc px long" (§4.7,
§6.9) versus "Euclidean length ≈ 2 r" (§10.10). Pick one: "2R + 1 pixels whose end centres are about 2 r apart".

**F22. `goja-cv run file.js` cannot lint a library that uses `args`.** Lines 843, 1487-1488, 2097. `run` evaluates the
file as an eval, where `args` is undefined. Bind `args = {}` in `run` (or a `--args JSON` flag) and say so in §12.3.

**F23. Loaded-file names can collide; the stack regex is fragile.** Lines 1115-1121, 1726. Two roots with the same
relative path share one `files[name]` entry when the second is loaded by absolute path. Name such files with their
root (`read2:dust/helpers.js`) or key the registry by `Location`. `([^\s()]+):(\d+):(\d+)` fails on names with
spaces or parentheses; refuse such names in `cv.load` or parse frames differently.

**F24. `cv.scripts` has no evidence behind it and adds a capability.** Lines 713-732, 1128-1130, 1418-1430. D6 itself
says no agent needed discovery. It walks every read directory (`--read ~/dustds` is large) and lets an agent list
`.js` names and first lines anywhere in the roots, which `cv.open` cannot. Recommendation: defer it; if kept, bound
the walk by entries visited, not only files listed, and note the enumeration in §7.2.

**F25. B1's test updates are incomplete.** Lines 2224-2227. Besides `regions_test.go:74`: `kernels/mask_test.go:66-95`
(`TestGrowThenShrinkIsNearIdentityOnBlobs`), `ops/ops_test.go:148-149, 161, 296` (registry-walk arguments and a
`grow` RANGE case), `ops/heal_test.go:76` (`toMask({ grow: 2.5 })`).

**F26. Table formatting details.** Lines 806, 1759-1760, 1769-1770. Widths use `len` (bytes): `…` and non-ASCII cells
misalign; use rune counts. "Next: .slice(118, 236) of this array" is ambiguous for a table nested in an object; the
renderer knows the key path (it uses it for functions), so write `rows.slice(118, 236)`.

**F27. Make sheet labels data too.** Lines 516-518, 864-874. A sheet's id labels are still numbers in a picture. The
look's caption (or the result next to it) can list the ids in cell order (`cells: 14 3 13 …`), so the model never has
to read a label to know which row a cell shows. One line in `sheet`, in the spirit of the guide's design rule.

**F28. Two quotations are not what the sources say.** Lines 338-339, 535-537. The de14d942 sentence is a paraphrase in
quotation marks (the diary: "Filtering mem.hotRows in JS (…) then mem.sheet(rows.map(o=>[o.x,o.y])) was the useful
pattern"). The 4b246f75 diary says the agent added `mem.r3 = r3` before using it; nothing failed (the study had no
errors). Quote exactly; drop "failed until".

**F29. Compare squared distances in the binary fast path.** Lines 1932-1936. `DistanceToSet` returns float32 square
roots; compare the squared distance with r² (or test `d ≤ r + 1e-4`) so float rounding cannot flip a boundary pixel
against the chord path in the property test.

**F30. The repeated `functions:` line is long.** Lines 762, 772, 777. With ten functions and default-valued parameters
it is 300–500 characters in every result. Cut each signature to about 40 characters on the repeated line and keep the
full one under `new functions:`.

**F31. `collectNodes` ignores the item link.** Lines 1211-1213, 1815. Linked items reach their node only through the
symbol key, which the walk skips. Harmless today (an awaited node is in `trace.Used`), but say so, or follow the
`ITEM` symbol.

**F32. Let JavaScript call the loaded function.** Lines 1088-1090, 1124-1126, 1711. `fn(undefined, args)` from Go
re-enters the VM inside a native call. `native.__compile(path)` returning the function and `prelude.js` doing
`return fn(args)` keeps the call JS → JS, makes the sync-depth count of F9 a JS counter, and keeps the binding to
"read, compile, record".

**F33. Goldens and shape consumers for Part B.** Lines 1520, 2273-2280. Add one golden for `overlay(peaks)` (exact
disks) and one for a morphology look. Peaks is the first op (not the `items` op) to produce `Disk` regions; list in B2
the consumers that must switch on `Region.Shape` (overlay, sheet, `mask.regions`, `measure`, `heal`) and test each.

## Counts

| priority | count | findings |
|---|---|---|
| P1 | 4 | F1–F4 |
| P2 | 12 | F5–F16 |
| P3 | 17 | F17–F33 |

## Recommendations on the open questions (§17)

1. **Sheets of points.** Settle it in GCV-004 Phase 5, not later: one binder type `items|point[]` shared by
   `measure`, `sample` and `sheet`, where the `items` op represents a centre without a radius as a `Point` shape
   (centre, no area). `sheet` sizes such cells from `size`; ops that need an area (`mask.regions`, `heal`) refuse a
   `Point` with a fix that adds `r`. One representation, no sheet-only list path; delete `listItems`.
2. **Labels through the `items` op.** Agree with the writer: in GCV-004 Phase 5, together with the string-`id` error,
   since that phase writes the `items` op now and doing it twice costs more. A3 then touches only sheet and overlay
   drawing. Also decide there that ids may repeat and nothing keys by id (D6 makes ids data).
3. **Default `snr`.** Keep 1 (recall first; results are sorted and the table shows the strongest first), but do not
   rest it on the random-place numbers: decide from the F12 measurement (peaks per tile not near gold, and coverage,
   at 0.5/1/1.4/2) before the playbook is rewritten. Do not switch to 1.4 without that data.
4. **`cv.save` without `await`.** Do not add a second load-style warning. Use goja's promise rejection tracker for
   one generic "rejected and never handled" warning (F8), implemented in A1 in place of the `LOAD` failure branch;
   `cv.save` then gets it for free and GCV-004 Phase 6 needs nothing. Make `__cv_run` wait for loads the eval
   started, so the "had not finished" branch is not needed.

## Usage Examples

Revision 2 can work through the findings in order: F1–F4 change the design text and tests; F5 is a sweep of numbers;
F6–F15 change decisions D3, D7, D8, D10, D13, D17–D19 and the phase table; F16–F33 are local edits.

## Related

- `design-doc/01-design-and-implementation-guide.md` (the guide), `reference/05-diary-review-read.md` (this review's
  diary), `reference/01-analysis-annotator-agent-feedback.md` (the disposition).
