---
Title: Diary - design writer
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
    - Path: repo://pkg/cv/graph/args.go
      Note: binder, lifting and silent accessors read for sections 5.1-5.3
    - Path: repo://pkg/cv/ops/views.go
      Note: speck view lifting found at 219-236 (levelsAt)
    - Path: repo://ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/design-doc/01-design-and-implementation-guide.md
      Note: the guide this diary records
    - Path: repo://ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/scripts/02-pi-names-typed.py
      Note: typed-name evidence behind guide 7.13
    - Path: repo://ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/scripts/03-goja-panic-and-spread.go.txt
      Note: goja panic and symbol-spread evidence (guide 5.12, D6)
ExternalSources: []
Summary: 'Design writer''s diary for GCV-004: reading the code and the pi evidence, the choices the brief left open, and writing the design and implementation guide.'
LastUpdated: 2026-11-09T14:00:00-05:00
WhatFor: Reviewing how the GCV-004 guide was written and why its open choices fell the way they did.
WhenToUse: Before reviewing or revising design-doc/01-design-and-implementation-guide.md.
---


# Diary — design writer (GCV-004)

## Goal

This diary records how the GCV-004 design and implementation guide was written: what was read, which numbers
were recomputed, the choices the coordinator's brief left to the writer, and what a reviewer should check.

## Step 1: Reading the code, the earlier guides and the evidence

I read the writing guidelines, the coordinator's diary (steps 1–3), the shared foundations chapter, the three
earlier guides with their deviation notes, and the code the brief names, before writing anything. I also
recounted the agent evidence in two places where the brief's numbers did not answer a design question (how
agents used `cv.evaluate`, which wrong names they typed), and added a second evidence script to the ticket.

### Prompt Context

**User prompt (verbatim):** the coordinator's brief, reproduced in full:

```text
You are the design writer for goja-cv ticket GCV-004. Write a detailed architecture, design and implementation guide for an intern who is new to the codebase, in a textbook authoring style. The guide is the main deliverable: an intern must be able to understand the system from it and implement GCV-004 phase by phase.

## Where things are
- **Repository:** `/Users/manuel/code/wesen/go-go-golems/goja-cv`, branch `task/gcv-004-api-v2`. It is already checked out and stacked on the GCV-001..003 branches, so it contains all of their code.
- **Ticket:** `ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/`.
  - Write the guide into `design-doc/01-design-and-implementation-guide.md`. Keep its docmgr frontmatter and fill in `Summary`, `RelatedFiles`, `WhatFor` and `WhenToUse`.
  - `reference/01-evidence-pi-run-api-usage.md` holds the usage numbers. `scripts/01-pi-api-usage.py` reproduces them.
  - `reference/02-diary-coordination.md` is my diary. Read steps 1–3: they hold the reasoning behind this ticket.
- **Writing guidelines.** Read all of these first and follow them:
  - `~/.codex/skills/textbook-authoring/SKILL.md`
  - `~/.codex/skills/ticket-research-docmgr-remarkable/references/writing-style.md`
  - `~/.codex/skills/ticket-research-docmgr-remarkable/references/deliverable-checklist.md`
  - `ttmp/_guidelines/design-doc.md`

  Introduce every concept before you use it. Define terms. Use real file paths, real type and function names, pseudocode, diagrams (ASCII or mermaid) and before/after JavaScript examples.
- **Shared vocabulary and earlier guides.**
  - The shared vocabulary is `ttmp/2026/11/09/GCV-001--…/reference/05-foundations-goja-cv-for-the-gcv-guides.md`.
  - The three previous guides are each ticket's `design-doc/01-design-and-implementation-guide.md`. Read their "Implementation notes (deviations)" sections: that is what was actually built.
  - The implementation diaries are GCV-001 `reference/06-…`, GCV-002 `reference/03-…` and GCV-003 `reference/03-…`.
  - Use the GCV-001..003 guides as a model for structure and depth.
- **Read the real code before describing it.** At least read: `pkg/cv/graph` (args.go, registry, builder, eval.go, render.go, schema.go, paths.go, the param store), `pkg/cv/ops`, `pkg/cv/dataout`, `pkg/cvsession` (including native.go), `pkg/gojamod/cv` (prelude.js, fields.go), `cmd/goja-cv` and `pkg/cv/docs/topics`. Cite `path:line` for the claims you make about current code.

## What GCV-004 is (decided: design how, not whether)
This is alpha internal testing software. There is no backward compatibility: no wrappers, aliases, deprecation shims or migration diagnostics. Rip out the complexity; prefer deletion to new mechanisms. The goal is a strong, streamlined core with a composable architecture, where whole bug classes become impossible to express.

### Part A — JavaScript API v2: one way per thing
The evidence is in the reference doc (751 pi calls).
1. **Options are literal values.**
   - Remove Params entirely (`P.*`, `DeclareParam`, the param store, the Param kind). There were 0 uses.
   - Remove value lifting, where a Value node is used as an option value. There were 4 uses, all in one run.
   - Data dependencies go through `await`, for example `levels({ white: (await dog.stats()).p999 })`.
   - Node inputs exist only where an op declares an image or regions input.
   - What this deletes: InputRef for values, the `Static` flag, lifted-input deduplication, and the build-time ambiguity behind the Codex findings on PR #2 (a param as `encoding` silently became srgb) and PR #3 (`cv.compare([img, img])` clipped).
2. **Space is a property of the image, not an option on every op.** There were 198 `{ space }` options, 80 of them `stats({ space: "encoded" })`.
   - Provide `img.as("linear" | "encoded")`, and remove the per-op `space` option.
   - Measurements report in the space of the image you measured, so file units by default.
   - The few physical ops convert internally to the space they require. Name them after reading the ops: exposure, heal, gaussian?, the speck view preset (which keeps its linear default, chosen by GCV-003's ranking test).
   - Design the output-space rule and the simplified SpaceRule.
   - This reverses GCV-001's "space belongs to the op". Explain why, using the evidence.
3. **A mask is a one-channel image.** Merge the Mask kind into Image. Every image op works on masks, and the mask ops (and/or/grow/coverage/…) accept any one-channel image. The errors this removes: E_WRONG_KIND, `mask.stats` and `Mask.coverage` E_UNKNOWN_METHOD.
4. **Regions become plain arrays when awaited.**
   - An awaited Regions value is a plain JS array of flat items (the GCV-003 schema).
   - Arrays of items are accepted wherever regions are, so agents filter with JS `filter`/`sort`/`slice` (74 JS `.filter(` calls).
   - Remove `where`, `sortBy`, `top`, `only` and `without` together with their machinery: `Produces`, build-time field checks, E_LAZY_COMPARE, `RemovedFields` and the throwing prelude accessors.
   - Detectors still return lazy nodes.
   - Decide and explain what a Regions node is now.
5. **One way per thing.**
   - Evaluate with `await` only; remove `cv.evaluate`.
   - One display call: pick between `x.look()` and `show()` and justify the choice.
   - One spelling per op: remove reverse aliases such as `regions.overlay(img)`.
   - One output function, `cv.save(path, x)`, with the format taken from the extension (csv, json, tif, png, npy). It replaces `cv.json`, `cv.csv`, `node.save`, data names and the separate naming rules. Saving the same path twice in one eval is an error, which makes the PR #4 uniqueName finding impossible. Design what happens when there is no out-root (inline delivery?).
6. **One workspace.** `cv.open` and `cv.save` resolve relative paths against the same root, implemented with Go's `os.Root` (Go 1.26 has `OpenFile`, `MkdirAll` and `WriteFile`) for both reads and writes. There were 23 E_ACCESS errors, mostly from reopening saved files. This replaces `Within`, `resolveExisting`, `resolveDepth` and the second check in `WriteOut`, and makes the PR #4 P1 symlink escape impossible.
   - Design the CLI and MCP flags (`--root`, `--out-root`, `--image`).
7. **Prune synonyms and redirects.** Keep only names that pi logs show agents actually typing. Use the evidence script, or grep the `calls.txt` files under the pi-runs dir named in the reference doc. Remove the rest.

### Part B — foundations (each one makes a review bug class impossible)
- **Build-time argument reading.** Build-time functions (encoding/space rules, DocFrame, Validate, Fields) read arguments through one typed path. Accessors must fail loudly on a wrong type, never return a zero value. Explain what remains of the old problem once options are literal.
- **Node lists.** One accessor resolves node-list arguments in list order. Do not hand raw deduplicated `inputs` to op code. This deletes `imageInputs`, sheet's `nodeOf` and `listInputs`.
- **`Evaluator.Render`.** Its postcondition (an image result is `ScaledSize(window, scale)`) is enforced at a single exit for all branches. Then `fitScale`'s special case and its tolerance go away.
- **Data out.** Use `encoding/json`: an ordered `MarshalJSON` on `graph.Record` and NaN → null in one place. This deletes the hand-written JSON writer.
  - Tables are records only. CSV is accepted only for a list of rows; profiles and histograms return rows plus a metadata object. This deletes `table()`'s shape guessing and `columnArrays`.
- **Panic boundary.** Recover at every Go function exposed to JS, converting the panic to E_INTERNAL. This covers the PR #2 TIFF panic class.
- **`cv.open` arguments.** Move its ~10 hidden args (stamp, w, h, format, bits, channels, sample, fileEncoding, recordedOrigin, notes) out of user Args into the source key or node metadata.
- **Smoke test.** It reads the files list from `--json` instead of hard-coding file names.
- **Remove today's edge-case code**, now made unnecessary: the symlink depth guard, `uniqueName`, the schema-less CSV fallback, and fitScale's `+1` tolerance. The review-fix commits are `4c79173`, `cd5f46c` and `319a6fc`.

### Out of scope (keep)
- The decoders, render op classes, looks, crop/zoom/rulers, sheets and views.
- Windowed `Halo` evaluation is still future work; mention it only if the new model changes its design.

## Structure the guide should have
[… sections 1–18 as listed in the brief: revision history and how to read; executive summary; architecture tour; problem statement; concepts; current state with path:line; the v2 API with before/after and a reference table; proposed architecture with diagrams; decision records; legacy to delete with a lines-removed estimate; pseudocode; errors; docs to change; implementation phases each green on build, vet, go test ./... -count=1 -race, make lint, make logcopter-check; test strategy with golden looks; validation plan with pi runs on questions-gcv-00{1,2,3}.md (run.sh with IMAGE, QUESTIONS, OUT_ROOT) and target metrics calls, errors, E_ACCESS, E_UNKNOWN_METHOD; migration note for the DUST-002 helpers (list what breaks, do not edit); risks, open questions, references. Also fill tasks.md with the phases.]

## Process
[… own diary in the diary skill format created with docmgr doc add --ticket GCV-004 --doc-type reference --title "Diary - design writer"; commit a draft then the finished guide, each followed by a diary commit, plain git push, Co-Authored-By line; do not touch code outside the ticket directory or other repositories; run docmgr doctor; report back the guide's length and sections, the decisions where the brief left a choice, the lines-removed estimate, and open questions.]
```

(The two bracketed passages abbreviate the brief's section-structure and process lists, which the guide and this diary
follow item by item.)

**Assistant interpretation:** Write `design-doc/01-design-and-implementation-guide.md` for GCV-004 (API v2 and
foundations) in the textbook style of the GCV-001..003 guides, deciding the open choices (display call, what a
Regions node is, saving without an out-root, the output-space rule) from the code and the pi evidence; fill
`tasks.md`; keep this diary; commit and push on `task/gcv-004-api-v2`.

**Inferred user intent:** An intern can implement a smaller, tighter goja-cv from the guide alone, and the
review-found bug classes become impossible to write.

### What I did
- Read `~/.codex/skills/textbook-authoring/SKILL.md`, `…/ticket-research-docmgr-remarkable/references/writing-style.md`,
  `ttmp/_guidelines/design-doc.md` and `~/.codex/skills/diary/SKILL.md`.
- Read the GCV-001 foundations chapter (`reference/05-…`), the deviation sections of the three guides, the "what was
  tricky" parts of the three implementation diaries, and coordination diary step 9 (the eight review fixes).
- Read the code with line numbers: `pkg/cv/graph/{registry,node,args,eval,render,schema,paths,params,space,engine,funcs,value,sources,errors,macro,suggest,regions}.go`,
  `pkg/cv/ops/{present,sheet,regions,regionmeasure,source,funcs,views}.go` (and the op table), `pkg/cv/dataout/{data,save}.go`,
  `pkg/gojamod/cv/{prelude.js,binding.go,fields.go,convert.go,dataout.go}`, `pkg/cvsession/{native.go,init.js,session.go,reference.go,processor.go}`,
  `cmd/goja-cv/{eval,mcp}.go`, the Makefile smoke target and the golden harness `pkg/cv/look/golden_test.go`.
- Built the binary into the scratchpad and listed every op with `goja-cv ops` (receiver, args, returns).
- Checked `go doc os.Root` on Go 1.26.10: `OpenRoot`, `Open`, `OpenFile`, `MkdirAll`, `WriteFile`, `Lstat`, `Stat` exist.
- Recounted from the pi logs: `cv.evaluate` scale arguments (77 of 79 calls pass `"full"`), op call counts in agent
  code, and every wrong name in error results. Wrote `scripts/02-pi-names-typed.py` for the latter; it also marks
  each key of the synonym and redirect tables as typed or never typed (14 typed, 232 never typed).
- Read the DUST-002 helpers (`scripts/84-cv-helpers.js`, `83-…py`, `85-…py`) and the pi harness `run.sh` for the
  migration note and the validation plan.

### Why
- The brief requires `path:line` citations and decisions grounded in code and evidence; the open choices (display call,
  Regions node, no out-root, output space) needed facts the brief did not state.

### What worked
- Every review fix maps to a concrete line: `Args.String` silently returning `""` (`args.go:222-226`), the lift
  deduplication (`args.go:371-376`) behind `listInputs` (`present.go:110-119`), the one-branch `fitScale` with its `+1`
  (`render.go:255-268`), the symlink depth guard (`paths.go:30-52`), `uniqueName` (`cmd/goja-cv/eval.go:258-265`).
- The `cv.evaluate` count settled the evaluation scale: agents want full-resolution numbers, so `await` evaluates at 1.

### What didn't work
- `cat ~/.codex/skills/ticket-research-docmgr-remarkable/references/deliverable-checklist.md`: `No such file or
  directory`. The brief names it; it does not exist. Followed the other three guidelines.
- `echo =====` in zsh: `(eval):1: ===== not found` (zsh `=word` expansion); used other separators.
- `grep --include=*.go` unquoted in zsh: `no matches found: --include=*.go`; quoted the glob.
- First version of `02-pi-names-typed.py` matched `Synonyms` inside `OptionSynonyms` and missed single-line maps;
  fixed with a look-behind and a brace matcher.

### What I learned
- The speck view uses value lifting internally (`views.go:219-236`: `levels({ white: max(5·p99, 1e-4) })` as a node
  chain). Removing lifting needs an internal op for that stretch.
- Image nodes already carry a static space (`Node.Space`, `space.go:215-239`) and sources are linear
  (`source.go:26`). "Space on the image" changes defaults and who chooses, not the existence of a static.
- Channel counts are not node statics (GCV-001 deviation 2), so merging Mask into Image needs one: otherwise the
  mask-kind check would move from build time to evaluation.

### What was tricky to build
- Making Regions nodes and awaited arrays interchangeable: a method on a Regions node (`toMask`) has no equivalent on a
  filtered array, which would be a new "works on the node, fails on the array" bug class. The resolution is that
  Regions nodes have no methods; every consumer is an image method that takes items.

### What warrants a second pair of eyes
- The evidence for `gaussian` and `blend` following the image's space (all 10 explicit `gaussian` spaces were
  `encoded`) versus `exposure` staying physical although 11 of 17 `exposure` calls asked for `encoded`.

### What should be done in the future
- N/A (the guide follows in step 2).

### Code review instructions
- `python3 scripts/02-pi-names-typed.py PI_RUNS_DIR .` (from the goja-cv checkout) reproduces the typed-name list.

### Technical details
- `cv.evaluate` scale arguments in agent code: `{'"full"': 77, 'none': 1, '8': 1}`.
- Typed synonym/redirect keys: MethodRedirects export, store, write, zoom; OptionSynonyms min, scale, area, crop,
  resample; Redirects box, maxArea, ringRadii; lookRedirects box, crop. Wrong method names typed: save, write,
  export, store, persist, download, toFile, writeTiff, meta, resample, as, evaluate, field, crop (on old builds),
  mask.stats, Mask.coverage, Regions.filter, mask.components, heal (on Regions).

## Step 2: Writing the guide, deciding the open choices, verifying goja

I wrote the guide in four appends (sections 1–2, 3–5, 6, 7; then 8–9, 10–12, 13–18), committed the first seven
sections as a draft (`903846f`), and finished it (`411ed29`, 2,170 lines). Where the brief left a choice I decided it
from the code and the evidence and wrote a decision record for it. Two facts the design rests on (goja's handling of
Go panics, and whether object spread copies symbol-keyed properties) I checked with a scratch program rather than
assume.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Same task; this step is the writing.

**Inferred user intent:** (see Step 1)

**Commit (docs):** `903846f` — "GCV-004: guide draft (sections 1-7 …); names-typed evidence script"; `411ed29` —
"GCV-004: design and implementation guide complete (…); tasks; goja panic/spread evidence"

### What I did
- Wrote `design-doc/01-design-and-implementation-guide.md`: revision history and reading guide; 1 executive summary;
  2 architecture tour with an end-to-end trace; 3 problem (review classes, evidence, scope); 4 concepts; 5 current
  state with `path:line`; 6 the v2 API with before/after and the reference table; 7 architecture (14 subsections);
  8 decision records D1–D14; 9 deletion checklist with the estimate; 10 pseudocode and two traces; 11 errors; 12 docs;
  13 phases 0–8; 14 test strategy; 15 pi validation plan; 16 DUST-002 migration; 17 risks and open questions; 18
  references.
- Decided: `look()` stays and queues (D9); Regions nodes have no methods and arrays become internal `items` nodes via
  an enumerable symbol link (D5, D6); one `measure(items)` (D7); `await` at scale 1 (D8); one writable workspace,
  default the working directory, `--read-only` delivers `cv.save` inline (D11); images open in encoded space and the
  output-space rule of §7.4; physical ops `exposure`, `heal`, `inpaint`, `toGray("rec709")`, `toLab`, `encode`;
  `gaussian`, `blend`, `within` follow (D3); channel count as a node static (D4); `E_NOT_AWAITED` (D13); internal
  `levelsAt` for the speck view's white point.
- Removed two migration-style redirects I had first written (`{ space }` → `as`, `cv.evaluate` → `await`): the brief
  excludes migration diagnostics, and an agent only types a removed name if a document showed it.
- Wrote `scripts/03-goja-panic-and-spread.go.txt` and ran it against goja-cv's goja version.
- Filled `tasks.md` (Phases 0–8 plus review and upload), updated the changelog, related files, added the vocabulary
  topics `api`, `architecture`, `refactor` that `docmgr doctor` asked for (`ttmp/vocabulary.yaml`).

### Why
- The brief asks for decisions with alternatives, not options; each open choice has a record in §8.

### What worked
- goja check: a native panic with a goja value is catchable in JavaScript; a Go runtime panic escapes `RunString`
  (`recovered at top: true`); object spread copies symbol-keyed properties while `Object.keys` and `JSON` ignore them
  (`spread: 7:1:{"a":1}`). This turned D6 from "pending a test" into a verified decision.
- `docmgr doctor --ticket GCV-004`: all checks passed after the vocabulary additions.

### What didn't work
- First `go run` of the scratch program: `module lookup disabled by GOPROXY=off` for `github.com/google/pprof`;
  pinned goja's indirect dependencies to goja-cv's `go.mod` versions, which are in the module cache.
- Writing the guide in one pass left two inconsistencies that I fixed: NaN handling first said "in the evaluator's
  stamp" and later "in `dataout.plain`" (now only `plain`, at encoding time), and §6.5 and D9 disagreed on look order
  (now: returned looks first, then the others in creation order).

### What I learned
- The validation inputs for the GCV-001 questions are not kept in the run directories; the guide shows how to rebuild
  them with GCV-001's `scripts/01-make-v-tile.py` from tile `1d955ef4b27d3d88_1536_1536`.
- The pi harness always passes `--root OUT/img`, which becomes the v2 workspace, so v2 needs no harness change as long
  as `OUT_ROOT` is not set (v2 has no `--out-root`).

### What was tricky to build
- The output-space rule with derived receivers: a derived image's `Space` is linear, so `d.add(img)` converts `img` to
  linear while `img.add(d)` works in file units. I kept "the receiver decides" and documented it in §7.4 rather than add
  a special case.
- The deletion estimate: measured where a file or range goes whole, estimated elsewhere; stated as ±25 % with a request
  that the implementer record `git diff --stat` per phase.

### What warrants a second pair of eyes
- D3 (`gaussian` follows; `exposure` physical) and open questions 1–6 in §17.
- The writable-by-default workspace (D11) for the MCP server started by Claude Code in a project directory.
- Whether the guide's `Args` redesign (inputs and values carried by `Args`, build-time functions taking only `Args`) is
  too large for Phase 1; it touches every op file mechanically.

### What should be done in the future
- Review read of the guide, revision, reMarkable upload (tasks 2 and 12).

### Code review instructions
- Read §1, §6.9 and §8 first; then §7 against the code it cites; then §13 for the phase order.
- `docmgr doctor --ticket GCV-004`; `python3 scripts/02-pi-names-typed.py PI_RUNS_DIR .`.

### Technical details
- Guide: 2,170 lines, sections 1–18. Deletion estimate: ~2,900 lines removed, ~800 added, net ~−2,100 of 23,154
  non-test lines (−9 %), ±25 %.
- Goldens expected to move, Phase 3 only: `gaussian`, `levels-gamma`, `levels-stats`, `profile-chart`.

## Step 3: Revision 2 from the review read and the coordinator's decisions

The coordinator sent the review read (37 findings: 4 P1, 18 P2, 15 P3) together with decisions on revision 1's six
open questions. I revised the guide in place (`004ffd7`, 2,716 lines). I rewrote the sections the findings touched
as whole blocks rather than patching sentences, added seven decision records, and added Appendix A, which maps every
finding to its disposition: 36 fixed, 1 partly fixed (F36), none declined.

### Prompt Context

**User prompt (verbatim):**

```text
GCV-004 revision round: please revise the guide from the review findings plus the coordinator decisions below.

The review read is in `reference/04-review-read-findings.md` (37 findings: 4 P1, 18 P2, 15 P3), and the reviewer's diary is in `reference/05-diary-review-read.md`. Pull first: the branch has the reviewer's commits `780c2bf` and `243a9aa`, plus my diary step 4 `0bba23b`.

**What to do**
- Fix all four P1s:
  - F1: two-input ops take the larger channel count.
  - F2: inputs declared as masks are never space-converted.
  - F3: item links are `[node, index]`, never the `id` field.
  - F4: use `os.SameFile` and refuse to save through a symlink.
- Fix every P2, unless one conflicts with a decision below. Pay particular attention to these:
  - F5: Phase 0 must be green without `--root`.
  - F8: cover the xgoja provider front end.
  - F12: rename the `as` option to `name`.
  - F14: converting an opened file to linear must not copy its pyramid level.
  - F15: one luminance definition.
  - F16: drop `--read-only` and the inline delivery.
  - F18: split Phase 1, so the Args redesign becomes its own phase.
- Take P3s where they are cheap. Fold in F10 (a LOOKS warning that lists dropped looks), F11 (stepChroma) and F21 (Phase 0a, with the golden moves named for the sheet badge and the ruler-label fixes).

**Decisions on the open questions**
Write each one as a decision record, with pros and cons and the evidence.
- **Q1:** `gaussian` follows the image's space. Blur is a linear filter that is valid in any space, and the high-pass background must share the space of the subtraction it feeds. All 10 explicit agent choices were encoded.
- **Q2:** `blend` and `within` follow the receiver, provided F2 holds. State one rule for every multi-input op: the receiver decides, and the other inputs are converted to the receiver's space through cached convert nodes. Masks are never converted.
- **Q3:** `measure` includes the ring by default unless `stats` is given (the reviewer's condition). The radii come from the item when it has them:
  - core = the item's `r` if present (gold points carry one), otherwise the equivalent radius √(area/π) for region items;
  - ring = [1.5r, 2.5r];
  - an explicit `coreRadius` or `ringRadii` overrides.

  Evidence from a real gold tile (DUST-002 `reference/08`): a gold point with `r = 63` measured snr 0.12 with a fixed 3 px core and 0.56 with core and ring sized from `r`. Plain points without `r` and without `coreRadius` are `E_ARG` with a fix.
- **Q4:** awaiting an Image returns its static summary and evaluates nothing. Values and Regions still evaluate when awaited. This also removes most of the full-resolution cost that `await` would otherwise have on large DNGs.
- **Q5:** refuse arrays that mix two region sets (the reviewer's position). Spell out that literal objects mixed with items from one set are fine, for example gold points (x, y, r) together with one detector's items.
- **Q6:** a workspace plus read-only extra directories. Neither the reviewer nor the first draft covered the data-in-several-places case, so this is new:
  - `--root DIR` is the one read-write workspace. Relative paths for both `cv.open` and `cv.save` resolve against it.
  - A repeatable `--read DIR` adds read-only directories through their own `os.Root`. Real setups keep data in several places: the colleague reads tiles from `~/dustdet/cv_tiles` and data from `~/dustds`, and `os.Root` refuses symlinks that leave the root, so symlinking is not a workaround.
  - `mcp` requires `--root` and errors at startup without it.
  - `eval` and `run` default `--root` to the current directory.
  - There is no inline-save fallback (F16).
- **D3:** keep `exposure` physical, with the reviewer's two conditions (F14, F15). In addition, the `exposure` doc and hint should point agents who want "brighter for looking" at `levels({ auto })`, not only at `mul(2 ** ev)`. That was the real intent behind the 11 encoded exposure calls.

**Process**
- Add a revision-history entry summarising revision 2.
- Add a short "Review disposition" section or appendix that maps every finding ID to fixed, partly fixed or declined, with a reason for anything declined.
- Keep the guide self-contained for an intern, and introduce new concepts (receiver-decides, the item index link, `--read`) before you use them.
- Write diary step 3 in the same diary-skill format.
- Commit the guide, then the diary, and push with a plain `git push`. End commit messages with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Run `docmgr doctor` before finishing.

When you are done, report the disposition counts, anything you declined and why, and any new open questions.
```

**Assistant interpretation:** Revise the guide: apply every finding except where a coordinator decision overrides
one, write the six open-question decisions and the D3 refinement as decision records, add a disposition appendix
and a revision-history entry, then commit, write the diary and push.

**Inferred user intent:** A guide that can be implemented without the silent-failure seams the review found, and that
supports the colleague's real setup (data in several directories, gold points with radii).

**Commit (docs):** `004ffd7` — "GCV-004: guide revision 2 from the review read …"

### What I did
- Pulled `780c2bf`, `243a9aa`, `0bba23b`; read the findings in full and coordination diary step 4 (the five v1 bugs).
- Checked the code behind new claims:
  - `matchChannels` in `pkg/cv/kernels/composite.go:24-29` (F1);
  - goja's uncatchable exception types, `runtime.go:317-346` of the module: `InterruptedError` and
    `StackOverflowError` are exported, while their base is not, so `isGojaPanic` matches the pointer types (F9);
  - the `stepChroma` divisor (`measurement.go:155`) and `coreRingFields` (`:140`);
  - the measurements' `gray` option (`measurement.go:26-35`, Rec. 709 of working-space values, F15);
  - the xgoja provider config (`pkg/xgoja/providers/cv/cv.go`: `baseDir`, `cacheBytes`, `previewLong`, `lookLong`).
- Rewrote these blocks (§ numbers are the guide's):
  - §4.3–4.6: receiver decides, one luminance, the larger channel count, masks read as stored, `[node, index]`
    links, literal boxes and circles, workspace and read directories;
  - §6.4–6.9: radii from items, the `ITEMS` warning, Image awaits, `LOOKS`, `name`, save mismatches and overwrite
    rules, the surface table;
  - §7.1, §7.4–7.8 and §7.11–7.14: `openLinear` and `as` folding, `OpDef.Luma`, `colormap` follows, the channel
    table, `ThreeChannel`, moved-item check, `Region.Shape` `Pixels`/`Box`/`Disk`, the look queue rules, `Workspace`
    with read roots and `Location`, `Lstat` and `SameFile`, temporary-name writes, the stale-source rule, `isGojaPanic`,
    `SourceMeta.Path`→`Loc`, the 14 typed keys accounted for, the Phase 0b smoke without `--root`;
  - §9 with a phase column and every F17 item;
  - §10–14 and §16–17.
- Smaller fixes in §1–3 (F19 kinds table, F24 definitions, F25 trace step 10, F29 scope).
- Decision records:
  - rewrote D3, D6, D7, D8, D9, D10, and D11 (now the workspace plus `--read` decision);
  - added D15 (`gaussian` follows), D16 (receiver decides), D17 (measure radii), D18 (Image awaits), D19 (one set
    per array), D20 (one luminance), D21 (`name`).
- Updated `tasks.md` (Phases 0a, 0b, 0c; the review task checked) and the changelog; `docmgr doctor` passes.

### Why
- The P1s are silent wrong-value or data-loss cases at the seams of the new model. The coordinator's decisions settle
  the open questions; recording them as decision records keeps the rationale with the evidence.

### What worked
- Writing whole sections to files and splicing them by heading kept the revision coherent; a table checker found one
  cell broken by `|snr|`, which I escaped.

### What didn't work
- One Bash call that wrote a section through a heredoc was refused by the shell safety check ("could not be checked
  for dangerous rm operations"); I wrote the section with the Write tool and spliced it with a small Python helper
  (`splice.py` in the scratchpad).
- A `python3 /dev/stdin` invocation silently did nothing (the heredoc and the argument competed for stdin); I
  re-ran it through the helper script.

### What I learned
- On a derived, linear-encoded DoG, the old luma of codes and the new luminance coincide, so the speck views and the
  `sheet-2x2` golden are unaffected by the one-luminance change.
- Gold JSON uses `x, y` for a centre, while goja-cv's `x, y` is a box corner. Literal circles are therefore
  `{ cx, cy, r }`, and the examples convert gold points explicitly.

### What was tricky to build
- **Phase numbering.** The coordinator named "Phase 0a" for the v1 bugs and wanted the Args redesign split out. To
  keep the labels in execution order I used 0a (bugs), 0b (foundations) and 0c (Args).
- **F36 against Q5.** The reviewer asked to drop the point shape; the coordinator wants gold points (centre and
  radius) mixed with items. I dropped points but added a `Disk` shape for literal circles, and centres without a
  radius are accepted only where an op takes points.

### What warrants a second pair of eyes
- The `stepChroma` gate at |snr| < 2: labelled dust has a median snr of 2.17, so about half of the labelled dust
  would report `null` (open question 3 of §17).
- "Saves do not follow links" refuses even links that stay inside the workspace (open question 1).
- The `ringRadii` option name in `measure` replaces revision 1's `ring: [r1, r2]` (the coordinator's wording); the
  typed redirect for `ringRadii` is therefore dropped as unnecessary.

### What should be done in the future
- reMarkable upload of revision 2.
- Implementation from Phase 0a.

### Code review instructions
- Start with Appendix A, then check each "where" column against the guide.
- `docmgr doctor --ticket GCV-004`.

### Technical details
- Disposition: 36 fixed, 1 partly (F36: no point shape, but a `Disk` for literal circles), 0 declined.
- Estimate now: ~2,950 removed, ~900 added, net ~−2,050 (−9 %).
- New open questions (§17):
  1. symlinks inside the workspace when saving;
  2. pinned versus failing stale sources;
  3. the `stepChroma` gate threshold;
  4. relative paths into `--read` directories.

## Step 4: Revision 3, the user's answers to the four open questions

The user accepted the coordinator's recommendations for the four open questions of revision 2. I folded them into
the guide as a small, targeted edit (`4c9a1ac`) so that an implementer can start on the branch, and moved §17's
questions to a "Decided in revision 3" list.

### Prompt Context

**User prompt (verbatim):** (from the coordinator, relaying the user's "yeah, it's fine. do it")

```text
GCV-004 revision 3: please fold the user's answers to the four new §17 questions into the guide. The user accepted all four of my recommendations ("yeah, it's fine. do it"). This is a small edit, needed quickly: an implementer starts on this branch as soon as you push.

1. **stepChroma gate.** `stepChroma` is `null` when |snr| < 1, not 2. Update F11, D17 and §7, the measure fields table, the tests and Phase 5. Reason: labelled dust has a median snr of 2.17, so a gate at 2 would null the colour reading for about half the dust, and that reading is what validation needs. Below 1 the step is smaller than the local spread, so its colour cannot be attributed. Note that a v1 patch on main applies the same rule now, so the v2 rewrite keeps the behaviour.
2. **Symlinks on save.** Keep refusing every symlink in the final path component, including links inside the workspace. It is simpler, and nobody has needed the exception.
3. **A file that changes after `cv.open`.** Accept the pinned behaviour as designed: "file changed" before the first decode, and the old pixels after it. State that it is deliberate.
4. **Relative `cv.open` paths.** Resolve them against the workspace (`--root`) first, then each `--read` directory in flag order; the first that exists wins. `cv.save` stays workspace-only. Reason: reopening files was the main source of the 23 E_ACCESS errors.

**Also:** a separate agent is fixing the sheet badge over the first label and the colliding ruler labels on main right now, as a v1 patch, along with the stepChroma gate and a LOOKS warning when looks beyond `MaxLooks` are dropped. Add a note to Phase 0a: if main already has that patch when implementation starts, Phase 0a is done by merging origin/main into the branch (a merge commit, no rebase), and the goldens move there. Phase 2's LOOKS warning then reuses the v1 warning.

**Process:**
- Move §17 Q1–Q4 to "decided", with one line each.
- Add a revision-history entry: revision 3.
- Write diary step 4.
- Commit the guide, then the diary, and push with a plain `git push`. End commit messages with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Run `docmgr doctor`.
- Report the commit hashes.
```

**Assistant interpretation:** Apply four decided answers and the Phase 0a merge note to the guide, quickly and
precisely; record the revision; push.

**Inferred user intent:** Unblock the implementer with a guide whose last open points are settled.

**Commit (docs):** `4c9a1ac` — "GCV-004: guide revision 3 (…)"

### What I did
- `stepChroma` gate changed from 2 to 1 in the following places:
  - §6.4;
  - §7.6, with the reason and the exact field doc string for `coreRingFields`;
  - D17's consequences;
  - the Phase 5 test (null on a noise patch, a number at snr 1.5);
  - Appendix A's F11 row.
- Relative `cv.open` paths are now searched in the workspace first, then in each `--read` directory in flag order.
  Updated §4.6, §6.7, the §7.8 resolution text and the flag table, the §10.5 `locate` pseudocode, the Phase 6 tests
  (found in the workspace, in the second read directory, in both, in neither) and D11's cons. `cv.save` stays
  workspace-only.
- Made both deliberate in §7.8: the pinned behaviour for files that change after `cv.open`, and the refusal of every
  final-component symlink.
- Phase 0a gained step 3: when `origin/main` has the v1 patch, Phase 0a is a merge commit (no rebase). Phase 2 reuses
  the v1 `LOOKS` warning, and Phase 5 keeps the v1 `stepChroma` rule.
- §17's open questions became "Decided in revision 3", one line each, with no open questions left.
- Added a revision-3 row to the revision history and updated the frontmatter and the changelog; `docmgr doctor`
  passes.

### Why
- An implementer starts as soon as this is pushed. These four answers change behaviour the tests pin down.

### What worked
- Each edit was an exact-match replacement with a count check, so nothing was replaced twice or silently missed.

### What didn't work
- N/A

### What I learned
- The guide has no separate measure-fields table; the field list is `coreRingFields`. I put the exact field doc
  string in §7.6 so that there is one place to copy it from.

### What was tricky to build
- Keeping the relative-path rule symmetric with saving. `cv.open` searches every root, while `cv.save` only ever
  writes the workspace. That keeps `cv.open(saved.path)` correct, because the workspace is searched first.

### What warrants a second pair of eyes
- A relative name present in both the workspace and a read directory opens the workspace's copy. This is intended
  (saved results shadow inputs), but it can surprise someone who expects the read directory's file.

### What should be done in the future
- reMarkable upload; implementation from Phase 0a (or the merge of the v1 patch).

### Code review instructions
- `git show 4c9a1ac -- ttmp/…/design-doc/01-design-and-implementation-guide.md`.

### Technical details
- Decided list: §17 "Decided in revision 3", items 1–4.
