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
