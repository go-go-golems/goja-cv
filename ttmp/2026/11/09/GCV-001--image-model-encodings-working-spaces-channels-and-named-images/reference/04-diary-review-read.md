---
Title: Diary - review read
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
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/reference/03-review-read-findings.md
      Note: the findings this diary step produced
    - Path: ttmp/2026/11/09/GCV-001--image-model-encodings-working-spaces-channels-and-named-images/design-doc/01-design-and-implementation-guide.md
      Note: the guide that was reviewed (not edited)
ExternalSources:
    - /Users/manuel/.codex/skills/textbook-authoring/SKILL.md
    - /Users/manuel/.codex/skills/ticket-research-docmgr-remarkable/references/writing-style.md
    - /Users/manuel/.codex/skills/diary/SKILL.md
Summary: "Diary of the independent review read of the GCV-001 design and implementation guide."
LastUpdated: 2026-11-09T13:30:00-05:00
WhatFor: "Understanding how the review findings were produced and what to double-check."
WhenToUse: "Before acting on 03-review-read-findings.md or re-reviewing the revised guide."
---

# Diary - review read

## Goal

This diary records the independent review read of the GCV-001 design and implementation guide: how the guide
was read, how its claims were checked against the code, and what a second reviewer should verify.

## Step 1: Review read of the GCV-001 guide

I read the two writing guidelines and the diary format first, then read the guide cold from top to bottom as an
intern who knows Go and some JavaScript but not goja-cv, noting every term used before its definition. In a
second pass I opened every cited file and line in the goja-cv tree, recomputed the guide's numbers, and searched
the GCV-002 and GCV-003 guides for the GCV-001 terms they depend on. The result is a ranked findings document
(4 P1, 22 P2, 21 P3); the guide itself was not edited and nothing was committed.

The most consequential finding was not a wrong line number but a wrong identity: "encoded = perceptual for sRGB
images" is true per channel and false for ops that combine channels before the transfer curve, which is how
`PerceptualLuminance` works today. Three of the four P1s are about the implementation path: the golden-look
tests the guide relies on do not exist, the phase order cannot keep tests green, and builder lambdas
(`within`) re-intern nodes through a path the space-resolution design does not cover.

### Prompt Context

**User prompt (verbatim):** "Do a review read of the GCV-001 design guide: check concept introduction, correctness against the code, gaps, guideline conformance, implementability and architecture; write ranked findings."

**Assistant interpretation:** Read the GCV-001 guide as a newcomer, verify every code claim against goja-cv at
the cited commit, check consistency with GCV-002/003, and write P1/P2/P3 findings with locations and concrete
fixes into a new docmgr reference doc, without editing the guide or committing.

**Inferred user intent:** Make sure the guide properly introduces its concepts and can be handed to an
implementer without questions, before implementation starts on a design that GCV-002 and GCV-003 build on.

**Commit (code):** N/A — no code or guide changes; the coordinator commits the two new docs.

### What I did

- Read `/Users/manuel/.codex/skills/textbook-authoring/SKILL.md`,
  `/Users/manuel/.codex/skills/ticket-research-docmgr-remarkable/references/writing-style.md` and
  `/Users/manuel/.codex/skills/diary/SKILL.md`.
- Read the guide (1281 lines) in three chunks, top to bottom, before opening any code.
- Confirmed the baseline: `git log --oneline` (HEAD `72911b1`, guide cites `175b5d4`) and
  `git diff --stat 175b5d4 HEAD -- pkg internal cmd` (empty), so current line numbers are valid.
- Checked citations with `nl -ba` on `pkg/cv/buffer/{color,io,buffer}.go`, `pkg/cv/kernels/{tone,mask,measure,
  draw,specks}.go`, `pkg/cv/ops/{source,mask,measure,tone}.go`, `pkg/cv/graph/{registry,node,engine,sources,
  explain,errors,args,value,regions,suggest,help}.go`, `pkg/cv/look/look.go`, `pkg/cvsession/{init.js,native.go,
  processor.go,mcp.go,reference.go,docs_test.go}`, `pkg/gojamod/cv/{binding.go,evaltrack.go,help.go,prelude.js}`,
  `pkg/cv/docs/topics/02-units-and-scales.md`.
- Enumerated all conversion call sites:
  `grep -rn -E "ToPerceptual|ToLinear\(|Linear8|Linear16|EncodeSRGB|SRGBToLinear|LinearToSRGB|ToImage\(" pkg cmd internal`.
- Counted op spaces: `grep -rhoE 'Space: *"[a-z]+"' pkg/cv/ops | sort | uniq -c` (matches 7/5/3/7/11/5/4/3) and
  listed every op with receiver, return kind and space to check the defaults table covers all pixel-reading ops.
- Checked unused error codes and `Base`/`Halo` readers with grep; checked x/image v0.47.0
  `tiff/reader.go:252-265`.
- Recomputed every number in §3, §7.5 and §7.9 with a short Python script in the scratchpad
  (`check.py`): transfer values, the tile code values, the speck-step ratio, the `levels` difference table, Lab
  of sRGB red, memory sizes, and luma versus perceptual luminance for coloured pixels.
- Read the handover issues 6, 7, 10, 13, 20-22, the export script, `dustds/rawio.py`, the first record of
  `~/dustds/meta/dng_meta.jsonl`, and the pi harness `run.sh`.
- Grepped GCV-002 and GCV-003 for `GCV-001|Frame|Expand|Derived|Level|perceptual|encoded|Quantize|Halo|E_FRAME`.
- Created the docs with `docmgr doc add --ticket GCV-001 --doc-type reference --title "Review read findings"`
  and `... --title "Diary - review read"` from the repo root, and wrote both.

### Why

- Reading cold before opening code was the only way to judge concept introduction honestly; once the code is
  familiar, undefined terms stop looking undefined.
- Checking every citation matters because the guide tells the intern "Read the cited lines; the guide only
  summarizes them" (§4); a wrong citation sends them to the wrong place.
- Recomputing numbers matters because §3 promises "numbers you can check with a calculator".

### What worked

- The citations are almost all exact; the verification table in the findings doc lists them.
- Listing all ops with their current `Space` strings made it quick to confirm the defaults table is complete and
  to find the ops whose receiver is not an Image (`mask.overlay(img)`, `regions.overlay(img)`, `refine`).
- Computing luma versus sRGB-encoded luminance for three colours turned a suspicion into a concrete P1 with
  numbers (pure red: 0.498 before, 0.213 after).

### What didn't work

- `nl -ba file | sed -n 1-80p`-style ranges with a leading digit range failed in this shell
  (`sed: 1: "1-80": invalid command code -` because BSD sed needs `a,bp`, and I had written `1-80p` with `-` instead of
  `,`); switched to `head`/`sed -n 'a,bp'`.
- `grep -rn ... --include=*.go` failed in zsh with `no matches found: --include=*.go` (unquoted glob); quoting the
  pattern or dropping `--include` fixed it.
- The first `Write` of the findings doc failed with "File has not been read yet" because docmgr had created the
  file; reading its first lines and writing again worked.

### What I learned

- `Builder.Substitute` re-interns nodes through `internNode`, bypassing `Intern`; any per-node static that is
  computed only in `Intern` is lost on the builder-lambda path.
- `Describe` hides an argument only when it equals its declared `Default`, so a per-op default stored in
  canonical args will always print.
- The repo has no golden look images; look tests assert format, caption and the burned number only.
- `golang.org/x/image/tiff/lzw` is an exported package, so an own TIFF parser can decompress LZW strips itself.

### What was tricky to build

- Separating "the identity is true" from "the identity is true for every op": the per-channel identity
  encoded = perceptual for sRGB is correct, so the error only shows when an op mixes channels before the
  curve. I found it by listing every kernel that calls `Luminance()` and comparing where the weighting happens
  relative to `ToPerceptual`.
- Ranking the lambda/`Substitute` issue: it could be read as an implementation detail, but following the
  guide's §9.2 pseudocode literally produces nodes without `Enc`/`Space` on that path, so I ranked it P1.
- Judging the `as` design required tracing three layers (`init.js` wrapper → `prelude.js` `cv.open` →
  `binding.open` → `Engine.Open`) to see that the module layer has no session reference.

### What warrants a second pair of eyes

- P1-1: whether to accept luma in encoded space (simpler, behaviour change) or keep photometric luminance
  (exact, needs an exception to "kernels never convert") is a design decision the writer and the user should
  make; the review only shows that the guide's current claim is false.
- P2-3 (drop `perceptual`): this is an architecture opinion. GCV-003's open question 1 mentions "all three spaces";
  dropping the space touches all three guides.
- P2-8 (stamp `Frame`/`Enc` from the node in the evaluator): plausible, but it changes every op's `Run`
  signature use of `NewImage`; someone who knows the evaluator's cache paths should confirm it fits.
- P3-18 (fixture tolerance): the estimate of the 16-bit sRGB step near white (3.5e-5 in linear) is analytic; it
  should be confirmed on the actual planted fixtures.

### What should be done in the future

- After the guide is revised, run a second short review read focused only on §7.5, §9.2 and §12, since most P1/P2
  findings land there.
- Apply the shared-vocabulary fixes (P2-9 `DocFrame`, P2-17 saved metadata, P2-19 space rule) to GCV-002 and
  GCV-003 in the same commit as GCV-001.

### Code review instructions

- Start with `03-review-read-findings.md`, section "P1 — must fix before handing to an implementer".
- Verify P1-1 with `pkg/cv/kernels/mask.go:41-52` and `pkg/cv/buffer/buffer.go:50-60`, then the Python snippet
  below.
- Verify P1-2 with `grep -rli golden pkg cmd internal testdata` and `grep -n "func Test" pkg/cv/look/look_test.go`.
- Verify P1-4 with `pkg/cv/graph/node.go:115-230` and `pkg/cv/graph/engine.go:127-141`.
- Verify the citation table by spot-checking two rows with `nl -ba`.

### Technical details

Luma versus perceptual luminance (P1-1):

```python
def dec(e): return e/12.92 if e<=0.04045 else ((e+0.055)/1.055)**2.4
def enc(x): return 12.92*x if x<=0.0031308 else 1.055*x**(1/2.4)-0.055
for rgb in [(1,0,0),(0.8,0.4,0.2),(0.3,0.5,0.7)]:
    lin=[dec(c) for c in rgb]; Y=0.2126*lin[0]+0.7152*lin[1]+0.0722*lin[2]
    print(rgb, round(enc(Y),4), round(0.2126*rgb[0]+0.7152*rgb[1]+0.0722*rgb[2],4))
# (1,0,0) 0.4984 0.2126 ; (0.8,0.4,0.2) 0.5125 0.4706 ; (0.3,0.5,0.7) 0.4856 0.4719
```

`levels` table check (§7.5), max difference in 8-bit codes over v ∈ (0, w] and for v/w in 20–80 %:
w = 0.92 → 0.48 / 0.26; 0.7 → 2.15 / 1.17; 0.5 → 4.45 / 2.47; 0.3 → 8.54 / 4.88 (guide: 0.5/0.3, 2.1/1.2,
4.5/2.5, 8.5/4.9).

Decode-direction speck example (P3-3): sRGB_decode(0.15) − sRGB_decode(0.10) = 0.0096;
sRGB_decode(0.65) − sRGB_decode(0.60) = 0.0615; ratio 6.4.

## Step 2: Re-check of the P1s after revision 2

The writer revised the guide (commit `48e3a50`), added a Disposition table to my findings doc and wrote a shared
Foundations chapter. I re-read the revised §3.3–3.7, §7.2, §7.5–7.8, §9, §12 and §13.3, checked each P1 against the
new text, spot-checked the P2 dispositions, and looked for problems the revision itself introduced, especially in
the phase plan and in the luminance and space decisions. I appended a "Re-check (revision 2)" section to the
findings doc. I did not edit the guide and did not commit anything.

Verdict: P1-4 is resolved. P1-2 and P1-3 are resolved structurally, but each leaves a gap (N1 golden expectation, N2
test inputs). P1-1 is partly resolved (N3 `refine`, N4 specks `meanL`). There is no new P1. There are five new P2s
(N1–N5). I also withdrew one sub-claim of my P2-10 (`NewRegion` exists as a method).

### Prompt Context

**User prompt (verbatim):** "Re-check P1s after revision 2."

**Assistant interpretation:** For each of the four P1s, say whether revision 2 resolves it fully, partly or not at
all, quoting the section. Check that no P2 marked "fixed" is actually still open. Report any new P1-level problem,
especially in the phase plan and the luminance/space decisions. Append the results to the findings doc and add this
diary step.

**Inferred user intent:** Decide whether GCV-001 can go to an implementer now, or whether it needs another
revision round first.

**Commit (code):** N/A. Docs only. The coordinator commits.

### What I did

- Ran `git log --oneline` and `git show --stat 48e3a50`. Listed the headings of the revised guide with
  `grep -n "^## \|^### "`.
- Read the Disposition table (`03-review-read-findings.md`, after the Related section).
- Read revised §3.3–3.7, §7.2, §7.5 (OpDef, resolution, statics, lambdas, defaults, convert), §7.6–7.8, §9.1–9.4,
  §12 and §13.1–13.3.
- Checked the code paths that the new design touches:
  - `pkg/cv/graph/eval.go:280-300`: the `Identity` shortcut returns `ins[0]`.
  - `pkg/cv/ops/tone.go:82`: `toGray` returns `src`.
  - `pkg/cv/ops/mask.go:353-364`: `refine`'s Image is an input, not the receiver.
  - `pkg/cv/ops/detect.go:10,37`: specks `meanL`.
  - `pkg/cv/graph/regions.go:64-67`: the `NewRegion` method.
- Grepped the guide for `Origin`, `tEXt`, `lzw`, `PackBits`, `` `as` `` and `refine`. Grepped Foundations for its
  headings, "context block", lifting and the CVJS-001 path.
- Measured 8-bit rounding flips with numpy on 3 M random samples:
  - float64 vs float32 storage of encoded values: 22 flips;
  - a 2e-5 (LUT-sized) perturbation: 7660 flips (0.26 %).
- Appended "Re-check (revision 2)" to the findings doc with a verdict table, N1–N5 and the P2-10 correction.

### Why

- The coordinator needs a per-P1 verdict to decide whether to hand GCV-001 to an implementer.
- Revisions that add mechanisms (the grey-input rule, evaluator stamping, macro nodes) can introduce problems the
  first review could not see. That is why I re-read the code paths those mechanisms touch.

### What worked

- The Disposition table made the P2 spot-check quick. Each row names a section, and every claim I checked held.
- The rounding experiment turned "the goldens will probably differ slightly" into a number: N1 is real, not
  speculative.

### What didn't work

- My original P2-10 claimed `NewRegion` does not exist. It does: `func (v *RegionsValue) NewRegion(id int, pix
  []int32) *Region` at `regions.go:67`. My first-pass grep `grep -rn "func NewRegion"` cannot match a method with a
  receiver. I recorded the correction in the findings doc.
- `grep -n … $D/design-doc/01*.md` printed `ugrep: warning: …01*.md: No such file or directory`. The glob was
  quoted by the variable expansion, so I used the full file name instead.

### What I learned

- The evaluator already returns input value objects in two places: the `Identity` shortcut and `toGray` on 1-channel
  images. Any design that writes into returned values has to copy first.
- Switching from the 2e-5 LUT to the exact transfer functions is a visible change at 8-bit resolution, even though it
  is numerically tiny.

### What was tricky to build

- Ranking N1. Golden failures on nearly every look would block Phase 3, and the guide tells the intern to read them
  as a defaults bug. I ranked it P2 rather than P1 because the harness already reports the maximum code difference,
  which makes the failure easy to diagnose, and the fix is one tolerance rule. I put it at the top of the new
  findings.
- Seeing that the grey-input rule misses `refine` needed the op's shape (`On: KindMask`, `Inputs: [Image]`). The
  guide's rule text only mentions "the receiver".

### What warrants a second pair of eyes

- N1: whether to accept a tolerance in the golden test, or keep the LUT in Phase 3 so the goldens stay byte-identical,
  is a judgement call for the writer.
- N5: whether macro-node statics should be copied from the expansion output, or declared by each macro, affects
  GCV-002's `highpass`, `dog` and `view`.

### What should be done in the future

- After N1–N5 are addressed, a five-minute check of §7.5 step 2, §7.8 and §13.3 is enough. A full re-read is not
  needed.

### Code review instructions

- Start with "Re-check (revision 2)" at the end of `03-review-read-findings.md`.
- Verify N3 with `sed -n 353,365p pkg/cv/ops/mask.go`, N5 with `sed -n 288,293p pkg/cv/graph/eval.go` and
  `sed -n 79,83p pkg/cv/ops/tone.go`, and N4 with `grep -n meanL pkg/cv/ops/detect.go`.
- Re-run the rounding experiment from the Technical details below.

### Technical details

```python
import numpy as np
rng=np.random.default_rng(1); v=rng.random(3_000_000).astype(np.float32); x=v.astype(np.float64)
enc=np.where(x<=0.0031308,12.92*x,1.055*np.power(x,1/2.4)-0.055)
old=np.round(enc*255); new=np.round(enc.astype(np.float32)*np.float32(255))
print((old!=new).sum())                                                         # 22
print((np.round((enc+rng.uniform(-2e-5,2e-5,enc.size))*255)!=old).sum())       # 7660
```
