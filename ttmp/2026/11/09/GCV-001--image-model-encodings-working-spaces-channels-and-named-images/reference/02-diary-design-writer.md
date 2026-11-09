---
Title: Diary - design writer
Ticket: GCV-001
Status: active
Topics:
    - image-model
    - io
    - color
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/design-doc/01-design-and-implementation-guide.md
      Note: GCV-001 guide (Step 1)
ExternalSources:
    - /Users/manuel/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/07/DUST-002--film-dust-detector-model-exploration/reference/07-goja-cv-handover-issues.md
Summary: "Diary of writing the three design-and-implementation guides GCV-001..003 (one step per guide and per revision round)."
LastUpdated: 2026-11-09T12:30:00-05:00
WhatFor: "Reviewing how the GCV guides were written and which decisions were made where."
WhenToUse: "Before revising a GCV guide or implementing from one."
---

# Diary - design writer

## Goal

This diary records the writing of the three intern guides for goja-cv's next phase: GCV-001 (image model),
GCV-002 (inspection views and looks) and GCV-003 (measurement and data out). One step per guide and one per
revision round, with the commands run, the evidence gathered, and the decisions that a reviewer should check.

## Step 1: GCV-001 guide — encodings, working spaces, frames, channels, named images

I read the writing guidelines, the handover issues and the colleague's playbook and scripts, the CVJS-001
intern guide, the pi-run evidence and the deep-dive article, and then the goja-cv code bottom-up (buffer,
kernels, graph, ops, look, gojamod, cvsession, docs, cmd). The guide replaces the hard-wired sRGB model with
an encoding per image, one build-time `space` option per op with explicit `convert` nodes, decoders for
linear/float/npy/DNG data, frames (replacing DocW/DocH), channel ops, a macro mechanism, and named images.

The central finding is that `OpDef.Space` is documentation only: the evaluator never reads it, and nine
kernels/helpers convert to perceptual privately. That makes the colleague's "choose the space" request a
refactor of where conversion happens, not a new option bolted onto kernels.

### Prompt Context

**User prompt (verbatim):** "ok, write out tickets 001 002 003, we'll then pass it on. Make sure to read the updated guideines on writing design docs and properly introducing concepts by doing a review read. Create the three ticket and write the design / implementation guide for an intern, in a textbook authoring manner. We are interested in keeping a strong core streamline but composable architecture, and opportunities to refactor / get rid of legacy or deprecated code should be taken. This is early stage, and no backwards wrappers needed."

**Assistant interpretation:** Write the three design-and-implementation guides (tickets already created) in
textbook style for an intern, with evidence-anchored current-state analysis, legacy-removal sections, decision
records, phases, tests, and docmgr tasks; commit each guide.

**Inferred user intent:** Hand implementable, teachable plans to interns that also clean up the core instead
of growing special cases.

**Commit (docs):** 7b3bb16 — "GCV-001: design and implementation guide (encodings, working spaces, decoders, frames, channels, named images) and tasks"

### What I did

- Read `~/.codex/skills/textbook-authoring/SKILL.md`, `ticket-research-docmgr-remarkable/references/writing-style.md`,
  `ttmp/_guidelines/design-doc.md`, `~/.codex/skills/diary/SKILL.md`.
- Read DUST-002 `reference/04`–`07`, `scripts/82-export-cv-tiles.py`, `scripts/84-cv-helpers.js`; CVJS-001
  design docs 02–06 (headings, values/nodes/ops/masks sections, buffers/registry/scale/looks, formats and
  result content); `various/pi-runs/README.md`; `scripts/06-pi-cv-agent/run.sh` and `questions.md`; the
  deep-dive article (error classes, rules, open questions).
- Read with `nl -ba`: `pkg/cv/buffer/{buffer,io,color,pyramid}.go`, `pkg/cv/graph/{registry,value,eval,engine,
  regions,sources,explain,node,errors}.go`, parts of `args.go`, `suggest.go`, `help.go`; all of `pkg/cv/ops/*.go`;
  `pkg/cv/kernels/{tone,composite,measure,draw,specks,mask}.go` (relevant parts); `pkg/cv/look/{look,font,chart}.go`;
  `pkg/gojamod/cv/{prelude.js,binding.go,module.go,fields.go,evaltrack.go,help.go}`;
  `pkg/cvsession/{session,init.js,native,processor (context block),yaml,reference,mcp}.go`; docs topics 01, 02,
  05, 06, 09 and `docs_test.go`; `cmd/goja-cv/eval.go`.
- Checked go-go-goja `resultcontent` (Content/Provider/Limits, WriteFiles, ExtensionForMIME) and `replmcp/tools.go`
  (`application/json` and `text/*` become MCP TextContent; all items count against `maxImages`).
- Checked x/image/tiff: rejects `SampleFormat` ≠ 1 (`reader.go:252-262`), has no LinearRaw case.
- Inspected a real tile's TIFF tags with a Python one-liner (uncompressed, 16-bit, 3 samples, one strip) and
  `dustds/rawio.py` plus `~/dustds/meta/dng_meta.jsonl` (VueScan LinearRaw in SubIFD, no black/white levels,
  7256 × 7326, data offset 8); confirmed `raw0027.dng` exists locally (319 MB).
- Built goja-cv into the scratchpad and measured the tile `1d955ef4b27d3d88_1536_1536.tif` (meanL 0.33567,
  p01 0.0085, p99 0.918; metadata reports `colorSpace: sRGB (assumed…)`).
- Computed the sRGB numbers used in the guide (0.5 → 0.214, 0.05 → 0.0039, density step 0.0744 vs 0.0289,
  code 38261 for v = 0.3) and the `levels` behaviour change table with `python3 -I`.
- Wrote the guide, added 19 tasks with `docmgr task add --ticket GCV-001`, ran
  `docmgr doctor --ticket GCV-001 --stale-after 30` (all checks passed), committed.

### Why

- The guide had to explain encodings and spaces before any API, because the API decisions (defaults, derived
  encoding, looks showing encoded values) only make sense once the identities "srgb: encoded = perceptual" and
  "linear: encoded = linear" are understood.
- Frames were pulled into GCV-001 (not GCV-002) because `open({ box })` on a DNG needs them and they are part of
  "where an image sits", i.e. the image model; GCV-002's crop becomes the third producer.
- The macro mechanism (`OpDef.Expand`) was put in GCV-001 because `split()` needs multi-node results, and
  GCV-002's views/high-pass need the same mechanism.

### What worked

- The registry already carries `Static`, units and option synonyms, so the `space` option and the `Level` unit
  slot into existing machinery rather than adding a parallel one.
- The identities made the default table nearly behaviour-preserving for sRGB images: every former "perceptual"
  op defaults to `encoded`, which is numerically identical there.

### What didn't work

- My first `grep` for error-code usage failed in zsh: `grep -rn "graph.$c\b\|[^a-zA-Z]$c[^a-zA-Z]" ...` gave
  `(eval):1: bad math expression: operand expected at '^a-zA-Z'`. Re-ran with `grep -rnw "$c"`.
- `grep --include=*.go` unquoted failed in zsh (`no matches found: --include=*.go`); quoting the glob fixed it.
- My first claim about the `levels` change ("less than two codes in the midtones") was too optimistic; a sweep
  showed up to 8.5 codes for white = 0.3 in the deep shadows, so the guide now has the measured table.

### What I learned

- `look.Render` → `buffer.ToImage` encodes masks through the sRGB curve, so a 0.5 mask shows as 188/255 today.
- `DetectSpecks` detects on perceptual luminance but reports `MeanL` from linear luminance: one op, two spaces.
- MCP delivers `application/json` content as TextContent but counts it against the image limit; relevant for
  GCV-003.

### What was tricky to build

- Choosing what looks display. Showing perceptual (today's rule) breaks signed maps on linear-encoded data (an
  offset of 0.5 would show as 0.735); showing encoded keeps every sRGB look identical and makes data display
  literal. The cost is that linear photographs need `encode("srgb")`, which the guide makes an explicit op.
- Deciding against a session/image default space. It would have been convenient for the colleague, but opening
  with `encoding: "linear"` already gives them v in every space, and a hidden default makes scripts ambiguous.
- The `levels` default: encoded would be the "display" choice, but it would silently break the
  `levels({ black: s.p01, white: s.p99 })` idiom against linear `stats()`. Kept linear and added a `SPACE`
  warning for lifted levels from another space.

### What warrants a second pair of eyes

- The default-space table (§7.5) and the claim that it changes nothing for sRGB images except `levels` and mask
  display.
- The derived-encoding rule (§3.5, decision record): whether every op the guides mark `DerivedEnc` is really a
  new quantity.
- The TIFF parser decision (own IFD parser + x/image fallback) versus a separate DNG decoder.

### What should be done in the future

- Implementation per the phases; the pi harness overrides (`IMAGE`, `QUESTIONS`) in CVJS-001's script directory.

### Code review instructions

- Start with §3 (concepts) and §7.5 (space resolution), then the decision records in §8.
- Validate evidence by opening the cited `file:line` ranges at commit 175b5d4.

### Technical details

- Commands: `docmgr doc add --ticket GCV-001 --doc-type reference --title "Diary - design writer"`,
  `docmgr task add --ticket GCV-001 --text "…"` (19 tasks), `docmgr doctor --ticket GCV-001 --stale-after 30`.
- Measurement: `goja-cv eval --image ~/dustdet/cv_tiles/1d955ef4b27d3d88_1536_1536.tif -e '…stats, histogram,
  components…'`.
