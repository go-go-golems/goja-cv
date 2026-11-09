---
Title: Diary - review read
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
    - Path: ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/reference/04-review-read-findings.md
      Note: the findings this diary records (37 findings, recommendations)
    - Path: ttmp/2026/11/09/GCV-004--api-v2-one-way-per-thing-literal-options-space-on-the-image-masks-as-images-regions-as-arrays-one-save-one-workspace-and-foundations/design-doc/01-design-and-implementation-guide.md
      Note: the guide under review (revision 1)
    - Path: pkg/cv/kernels/composite.go
      Note: matchChannels broadcast behind F1
    - Path: pkg/cv/graph/space.go
      Note: masks are SpaceNone today (F2); ENCODING fix text (F7)
ExternalSources: []
Summary: "Reviewer's diary for the GCV-004 guide review read: how the guide was read cold, what was checked against the code, the os.Root experiment, and how the 37 findings were rated."
LastUpdated: 2026-11-09T19:45:00-05:00
WhatFor: "Reproducing or auditing the review read of the GCV-004 guide."
WhenToUse: "Before revising the guide from reference/04, or when a finding's evidence is questioned."
---

# Diary — review read (GCV-004)

## Goal

This diary records the review read of the GCV-004 design and implementation guide. It covers what I read, how I
checked the guide against the code, the experiments behind the findings, and how I rated them. The findings are in
`reference/04-review-read-findings.md`.

## Step 1: Cold read of the guide and the ticket

I read the guide front to back as an intern new to goja-cv would, noting every term used before its definition and
every place where the tour left me unable to follow the trace. Then I read the ticket's other documents, the three
writing guidelines and the diary skill, so the findings use the same measures the writer was given.

### Prompt Context

**User prompt (verbatim):** the coordinator's brief. The opening is quoted exactly; the bracketed passages abbreviate
the lists that follow, and the findings document addresses each list item by item:

```text
You are the reviewer for the goja-cv GCV-004 design guide. Do a cold review read, as a capable intern new to the codebase would read it, and record findings that the writer can act on. Do not edit the guide.

## What to read
- **The guide:** `/Users/manuel/code/wesen/go-go-golems/goja-cv/ttmp/2026/11/09/GCV-004--…/design-doc/01-design-and-implementation-guide.md` (2,170 lines, on branch `task/gcv-004-api-v2`, already checked out).
- **The ticket's other docs:** [reference/01 evidence, reference/02 coordination diary (step 4 lists five bugs found using v1 on real data), reference/03 writer diary, scripts/]
- **The writing guidelines to check against:** [textbook-authoring SKILL.md, writing-style.md, ttmp/_guidelines/design-doc.md]
- **The real code:** `pkg/cv/graph`, `pkg/cv/ops`, `pkg/cv/dataout`, `pkg/cvsession`, `pkg/gojamod/cv`, `cmd/goja-cv`. Verify the guide's claims against it.

## Check, in this order of importance
1. **Concept introduction.** [every term introduced before use; can an intern follow the tour and the call trace; forward references with lines]
2. **Correctness against the code.** [spot-check at least 25 path:line citations; deletion checklist names exist and misses nothing important (params, lifting, space-option plumbing, Mask kind, query ops, dataout naming, Within)]
3. **Soundness of the design.** [contradictions; item→node links through JS array operations; arrays from mixed region sets; masks as one-channel images and ops that must refuse multi-channel input; space following for blend/sub with images in different spaces; os.Root semantics (symlinked parents inside the root, absolute paths, default writable cwd in MCP); queuing every look (memory, ordering, MaxLooks); await at full resolution on large DNGs]
4. **Implementability.** [each phase green; order; tests; missing work such as docs, MCP instructions, goldens]
5. **Simplicity.** [alpha internal software: rip out complexity, no compat code]
6. **The writer's open questions (§17) and D3 (exposure stays physical).** [a recommendation for each]
7. **The five v1 bugs in coordination diary step 4.** [bare look dropped; looks beyond MaxLooks dropped; stepChroma unstable; sheet badge covers item 0's label; ruler labels collide]

## Output
[findings via docmgr doc add "Review read findings", P1/P2/P3 with line reference, problem and fix, summary count and open-question recommendations; this diary via docmgr doc add "Diary - review read" in the diary skill format; commit findings then diary on task/gcv-004-api-v2, plain git push, Co-Authored-By line; modify nothing outside the ticket's reference/; report counts, P1s in one line each, recommendations]
```

**Assistant interpretation:** Review the GCV-004 guide cold and against the code, then write prioritized findings
with concrete fixes and recommendations on D3 and §17's six questions. Keep this diary, and commit both documents
without touching the guide or the code.

**Inferred user intent:** Before an intern implements from the guide, find what is wrong, unsafe, missing or
needlessly complex, while it is still cheap to fix in prose. This is the same writer → review → revision loop that
took GCV-001..003 to zero P1 findings.

### What I did
- Read the guide (2,170 lines) in five chunks, noting terms and forward references with line numbers.
- Read `reference/01` (evidence), `02` (coordination diary, especially step 4), `03` (writer diary) and
  `scripts/01`–`03`.
- Read `~/.codex/skills/textbook-authoring/SKILL.md`, `…/ticket-research-docmgr-remarkable/references/writing-style.md`,
  `ttmp/_guidelines/design-doc.md` and `~/.codex/skills/diary/SKILL.md`.

### Why
- Concept introduction was the first priority, and it can only be judged on a first read, before the code is
  familiar.

### What worked
- The guide follows the design-doc order of the writing-style guide almost exactly, and its decision records use the
  required format.
- The tour (§2) and the trace (§2.11) are good enough that I could predict most of the code before opening it.

### What didn't work
- `echo ======` in zsh failed with `(eval):1: ===== not found` (zsh `=word` expansion). The writer's diary records the
  same mistake. I used other separators.

### What I learned
- The guide never defines the kinds (Value in particular), although every later section uses them (F19).
- "pi", "replapi", "sheet" and "the colleague" appear before any explanation (F24).

### What was tricky to build
- Keeping the cold read honest: a cold read is lost once the code has been read. I noted the terms and forward
  references during the first pass and did not add any afterwards.

### What warrants a second pair of eyes
- Whether forward references in the executive summary (§1) count as defects. I did not count them: a summary may
  compress, and §2 follows immediately.

### What should be done in the future
- N/A

### Code review instructions
- Read `reference/04` Quick Reference, then the P1 sections.

### Technical details
- Guide sections and the lines they occupy: §2 146-387, §4 445-542, §6 689-929, §7 931-1485, §8 1487-1687,
  §9 1689-1721, §13 1886-1987, §17 2120-2155.

## Step 2: Checking the guide against the code

I verified the guide's citations and statements about current behaviour, by file, against `task/gcv-004-api-v2`
(`0bba23b`). I built the binary and listed every op with its space default, and I ran the evidence script, so the
claims about spaces, goldens and typed names rest on the code and the logs rather than on the guide.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Correctness against the code: spot-check at least 25 citations, and audit the deletion
checklist.

**Inferred user intent:** (see Step 1)

### What I did
- Printed each cited range with `sed -n` and compared it with the guide's description. The citations checked, by
  file (✓ right, ✗ wrong):
  - `buffer.go:10-14` ✓; `encoding.go:13`, `48`, `85`, `93` ✓; `pyramid.go:61-63` ✓; `decode.go:111-142` ✓.
  - `registry.go:18-26`, `33-40`, `70-79`, `95-143`, `102-109`, `131-137`, `150-161`, `173-199`, `254-310`,
    `267-270`, `289-292`, `325-329`, `337-383`, `370-375`, `406-417` ✓.
  - `args.go:42-45`, `73-88`, `173-177`, `181-198`, `202-207`, `222-266`, `356-377`, `371-376`, `392-402`, `436-439`,
    `815-843`, `938-1054`, `1022-1024`, `1138-1156` ✓.
  - `node.go:17-29`, `102-169`, `109-111`, `173-191`, `275-288`, `334-375`, `377-391` ✓.
  - `space.go:33-45`, `71-112`, `156-178`, `215-239` ✓.
  - `source.go:14-27`, `26`, `32-47`, `102`, `108-117`, `140-151`, `154-159` ✓.
  - `engine.go:71-89`, `117`, `125-137`, `163-164`, `171-184`, `186-198`, `201` ✓.
  - `binding.go:104-137`, `273-300`, `330-351`, `357-393`, `395-415`, `417-434`, `486-501`, `503-603` ✓.
  - `prelude.js:43`, `55-69`, `72-100`, `121-133`, `140-143`, `196-208`, `210-267`, `221-227`, `265`, `295-302`,
    `313-330`, `355-361` ✓.
  - `native.go:24-37`, `65-84`, `100-114` ✓; `session.go:61-66`, `296-302`, `311-319` ✓.
  - `processor.go:64-93`, `86-89`, `106-109`, `124-169`, `449-489`, `503-545` ✓.
  - `present.go:28-34`, `107-119`, `124`, `139`, `145` ✓; `sheet.go:54-72`, `141-165` ✓; `mask.go:377-378` ✓.
  - `regionmeasure.go:31-33`, `109-122`, `160-168`, `311-315` ✓; `regionmeasure.go:76-104` for `coreRingFields` ✗
    (it is in `measurement.go:140`).
  - `views.go:141-156`, `219-236`, `257`, `262-266` ✓; `meta.go:13` ✓; `geometry.go:26` ✓; `value.go:52-61` ✓.
  - `ops/regions.go:37`, `163`, `272-413` ✓; `schema.go:43`, `107`, `158` ✓.
  - `render.go:40-43`, `44-87`, `255-268`, `257`, `270-284` ✓.
  - `paths.go:17-52`, `59-102` ✓.
  - `graph/funcs.go:15-36`, `146-158` ✓; `graph/funcs.go:56-83,114-119` ✗ (should be `ops/funcs.go`);
    `funcs.go:21-27` ✗ (`ops/funcs.go`).
  - `cmd/goja-cv/eval.go:97-157`, `255-265`, `303` ✓; `mcp.go:81-82` ✓; `Makefile:87-102` ✓;
    `push.yml:40-51` ✓; 29 golden looks ✓.
  - Statement ✗: "`lookRedirects` keeps box, crop, scale" (it holds box, crop, stretch).
- Built the binary into the scratchpad and ran `goja-cv ops`; also grepped every `SpaceRule{…}` in `pkg/cv/ops`.
  This confirmed that the four goldens listed in §14.3 are the ones that move, and that `channel` and `split` have no
  space rule.
- Checked every name in the deletion checklist with `grep` (`truthy`, `regionMeasureProduces`, `speckProduces`,
  `lookKindError`, `Callbacks`, `itemsBound`, `itemProtoJS`, `collision`, `blendRedirects`,
  `EncodingValueSynonyms`, `Within`, `IsMask`). All exist.
- Looked for what the new model makes dead but §9 omits (F17), and found `pkg/xgoja/providers/cv/cv.go` (F8).
- Ran `scripts/02-pi-names-typed.py PI_RUNS .` to compare §7.13 with the typed keys (F27).

### Why
- A wrong citation costs an intern time on every phase, and a wrong statement about current behaviour leads to a
  wrong design. Checking by file, not by sample, also exposed neighbours the guide does not cite. That is how
  `matchChannels` (F1) and the masks' `SpaceNone` (F2) came up.

### What worked
- Only 4 of more than 100 checks were wrong. The guide's citations are reliable.

### What didn't work
- `grep -rn "BaseDir" --include=*.go .` in zsh failed with `(eval):1: no matches found: --include=*.go`. I quoted the
  glob (`--include='*.go'`).
- A one-line `sed` pipeline to tabulate op names against space rules printed `head: illegal line count -- 0`.
  I used `goja-cv ops` and a simpler `grep -n 'SpaceRule{'` instead.

### What I learned
- Today masks are `EncNone`/`SpaceNone`, and `buffer.Identity` treats `SpaceNone` as the identity, which is why masks
  are never converted. v2 removes this protection without saying so (F2).
- `kernels.binary` broadcasts grey to RGB in either direction (F1).
- `stats`' `meanL` in encoded space is luma of codes, while `mask.luminance` and `histogram` use encode(Y) through
  the Gray rule (F15).
- The evidence shows 19 encoded `toGray` overrides, which D3 does not discuss (F15).
- `detect.specks` already accepts `polarity: "both"`, which covers the common mixed-set case (the Q5
  recommendation).
- The planted scan is 256 px wide, so every golden renders at scale ≥ 1. Phase 0's exact-size cut therefore cannot
  move a golden.

### What was tricky to build
- Telling "wrong" from "drifted" for line numbers. The guide says drifted lines should be found by name. I counted a
  citation as wrong only when the named thing is in another file or the description does not match.

### What warrants a second pair of eyes
- F15's claim that the two luminances differ in encoded space rests on the `stats` doc string and the Gray-rule code.
  An implementer should confirm it numerically: on an srgb image,
  `(await img.stats()).meanL` vs `(await img.toGray().stats()).meanL`.

### What should be done in the future
- N/A

### Code review instructions
- `sed -n A,Bp FILE` for any citation in the list above, from the goja-cv root at `0bba23b`.
- `python3 ttmp/2026/11/09/GCV-004--…/scripts/02-pi-names-typed.py <pi-runs dir> .`

### Technical details
- Typed keys reported by the script: `MethodRedirects:` `export`, `store`, `write`, `zoom`; `OptionSynonyms:` `area`,
  `crop`, `min`, `resample`, `scale`; `Redirects:` `box`, `maxArea`, `ringRadii`; `lookRedirects:` `box`, `crop`.
- `{ space }` overrides by op (from reference/01): stats/encoded 80, toGray/encoded 19, levels/encoded 13,
  exposure/encoded 11, detect.specks/linear 10, gaussian/encoded 10, blend/encoded 7.

## Step 3: Soundness probes: the os.Root experiment and design walk-throughs

For each focus area the brief named, I either ran the case or traced it through the code the guide would change.
`os.Root` behaviour was the one question I could settle by experiment, so I wrote a scratch program. The rest I
settled by reading the kernels, the binder, the prelude and the processor.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Soundness, in seven named areas.

**Inferred user intent:** (see Step 1)

### What I did
- Ran a Go 1.26.10 program against `os.Root` in the scratchpad (code in Technical details).
- Traced the item link through `marshal` and the planned `items` op, for `filter`, `sort`, `slice`, `map`, spread,
  spread with edits, renumbering and JSON round trips (F3, F20).
- Traced mixed region sets through the `items` op and checked `detect.specks`' `polarity` (Q5).
- Traced masks through `resolveSpace` with srgb one-channel images (F2), and two-image ops through
  `kernels.binary` (F1).
- Traced `blend` and `sub` with images in different spaces. "The receiver decides" is consistent. `colormap`,
  however, has `Level` arguments under the Display rule (F6).
- Traced the look budget through `collect` and `Process` (F10, F37).
- Estimated buffer sizes for a 7256 × 7326 scan through `SourceStore.Load`, the encoded `open.Run`, and the bracket
  (F14).

### Why
- These are the seams where an alpha refactor produces silent wrong values, which is the bug class the ticket exists
  to remove.

### What worked
- The experiment answered the coordination diary's open question: `MkdirAll` through a symlinked parent inside the
  root works.
- The same experiment exposed F4: a write through an in-root alias overwrites the source.

### What didn't work
- N/A

### What I learned
- `os.Root` refuses absolute paths outright (`openat /…: path escapes from parent`). The guide's `Rel` step is
  therefore required, not optional.
- `root.Lstat` on a dangling link succeeds, so `replaced` would be `true` for a write that then fails. This is
  harmless, because the write errors first.

### What was tricky to build
- Deciding P1 or P2 for F2 and F3. Both are silent wrong results in flows the guide itself recommends
  (`img.toGray()` as a mask; spreading items), and both have one-rule fixes. I rated them P1 because, as written,
  the guide prescribes the wrong behaviour.

### What warrants a second pair of eyes
- F9's claim that an interrupt unwinds through native frames as a non-`goja.Value` panic. It depends on the goja
  version in `go.mod`. A ten-line test (interrupt while a native function reads a getter) settles it.
- F14's memory arithmetic, which assumes the cache keeps the encoded copy at scale 1.

### What should be done in the future
- Copy the os.Root program into the Phase 6 tests as `TestWorkspaceSymlinks` cases, including the alias case.

### Code review instructions
- Re-run the program below with `go run . <dir>` in an empty module (no dependencies).

### Technical details

```go
// scratch: os.Root behaviour (Go 1.26.10, darwin/arm64)
r, _ := os.OpenRoot(w)                    // w/linkdir → real, w/dangle → ../out/x.tif, w/outdir → ../out,
                                          // w/alias.tif → src.tif
r.MkdirAll("linkdir/a/b", 0o755)          // nil
r.WriteFile("linkdir/a/b/f", …)           // nil
r.WriteFile("dangle", …)                  // openat dangle: path escapes from parent (outside file not created)
r.MkdirAll("outdir/sub", 0o755)           // mkdirat outdir/sub: path escapes from parent
r.Open("/abs/w/src.tif")                  // openat /abs/…: path escapes from parent
r.Open("../out")                          // path escapes from parent
r.Lstat("alias.tif")                      // mode Lrwxr-xr-x
r.WriteFile("alias.tif", "OVERWRITTEN")   // nil; src.tif now "OVERWRITTEN"  ← F4
r.Lstat("dangle")                         // nil error
```

## Step 4: Rating, recommendations, writing and committing

I rated each finding against the brief's definitions, wrote the recommendations on D3 and the six open questions,
and mapped the five v1 bugs onto phases. Then I wrote the findings document and this diary and committed them in
that order.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Produce the findings document and this diary, then commit and push.

**Inferred user intent:** (see Step 1)

**Commit (docs):** `780c2bf` — "GCV-004: review read findings on the design guide (4 P1, 18 P2, 15 P3; D3 and
open-question recommendations)"

### What I did
- `docmgr doc add --ticket GCV-004 --doc-type reference --title "Review read findings"` (→ `reference/04-…`) and
  `… --title "Diary - review read"` (→ `reference/05-…`).
- Wrote 37 findings: 4 P1, 18 P2, 15 P3. Each has guide lines, the problem and a fix.
- Mapped the v1 bugs:
  - the bare look is covered (D9);
  - the `MaxLooks` drop is not (F10, Phase 2);
  - `stepChroma` is not (F11, Phase 5);
  - the badge and the ruler labels are not (F21, a new Phase 0a with explicit golden moves).
- Recommendations:
  - D3: keep, with F14 and F15;
  - Q1 and Q2: keep `gaussian`, `blend` and `within` following, provided F2 holds;
  - Q3: `ring` defaults to true only without `stats`;
  - Q4: yes;
  - Q5: keep refusing;
  - Q6: `mcp` requires `--root`.
- `docmgr doctor --ticket GCV-004`: all checks passed.

### Why
- The brief asks for actionable findings, so each one has a fix the writer can paste or adapt. They are ordered by
  priority so that the revision starts with the P1s.

### What worked
- Most fixes are deletions or a single rule, which fits the ticket's direction.
- F16 (drop `--read-only`) and F36 (drop the `Point` shape) remove code.

### What didn't work
- N/A

### What I learned
- The guide's weakest areas are not its explanations but the seams it creates: masks entering the space rules, links
  keyed by mutable data, and a name check that guards a file.

### What was tricky to build
- The golden rule. Two v1 bugs are render bugs whose fixes move goldens, while the guide promises that only Phase 3
  moves goldens. Rather than call the bugs out of scope, I proposed a Phase 0a that may move named goldens with a
  stated reason.

### What warrants a second pair of eyes
- The Q3 recommendation (a conditional default) against the simplicity goal. The alternative is requiring
  `ring` or `stats` explicitly.
- F16 (drop `--read-only`) is a product call. The user may want a protected-directory mode for the colleague.

### What should be done in the future
- The writer revises the guide from `reference/04`, starting with F1–F4, then F5, F8 and F10, because they change
  phase contents.
- A second, shorter review read of revision 2 should check only the P1 and P2 fixes.

### Code review instructions
- Start with the Quick Reference table in `reference/04`, then the P1 sections.
- Validate F1 on v1 (run in Step 4 on the planted scan):
  `goja-cv eval --image planted-256x192-16bit.tif -e 'const g = img.toGray(); return { gray: (await g).channels, product: (await g.mul(img)).channels }'`
  printed `{gray: 1, product: 3}`. A one-channel receiver times a colour operand has three channels.
- Validate F4 with the program in Step 3.

### Technical details
- Commits on `task/gcv-004-api-v2`: findings `780c2bf`, then this diary.
- Counts: P1 4 (F1–F4), P2 18 (F5–F22), P3 15 (F23–F37).
