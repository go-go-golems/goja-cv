---
Title: Evidence - pi run API usage
Ticket: GCV-004
Status: active
Topics:
    - api
    - architecture
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-11-09T12:06:30.877939-05:00
WhatFor: ""
WhenToUse: ""
---

# Evidence - pi run API usage

## Goal

The usage numbers behind GCV-004: what 18 pi agent runs (751 eval calls, model `lunaroute/deepseek-4.1-flash`)
actually wrote against goja-cv's JavaScript API, and which errors they hit. GCV-004 removes or reshapes the
features that the agents never use, constantly override, or trip over.

## Context

- Runs 01–18 live in the dust-scanning repo: `ttmp/2026/11/08/CVJS-001--…/various/pi-runs/run-*/calls.txt`
  (one block per eval call: the code and the result). Runs 01–06 are the early API iterations (planted scan),
  07–10 GCV-001 before/after, 11–14 GCV-002 before/after, 15–18 GCV-003 before/after (density tiles).
- Reproduce with `scripts/01-pi-api-usage.py PI_RUNS_DIR`. Counts are regex occurrences in the agent's code,
  so a call that uses a feature twice counts twice; they measure use, not success.

## Quick Reference

| Feature | Uses in 751 calls | Reading |
|---|---|---|
| Params (`P.slider`, `P.choice`, …) | 0 | never used by agents |
| Value lifting (an unawaited node as an option value, e.g. `{ white: dog.stats().p999 }`) | 4 (one run) | `(await dog.stats()).p999` does the same |
| `{ space: … }` option | 198 | 80 of them `stats({ space: "encoded" })`: agents want numbers in file units |
| `await` / `cv.evaluate` | 1139 / 85 | two ways to evaluate |
| `.look(` / `show(` | 167 / 87 | two ways to show |
| JS `.filter(` / `.where(` `.sortBy(` `.top(` | 74 / 43, 20, 22 | agents filter awaited items in JS as often as with the query ops |
| `.overlay(` `.crop(` `.view(` | 69, 71, 64 | heavily used: keep |
| `cv.json`/`cv.csv` / `.save(` | 10 / 11 | three output functions, one job |
| `.channel(`, `mask.gradient` | 0, 6 | rarely used |

| Error code | Count | Typical message |
|---|---|---|
| E_UNKNOWN_METHOD | 35 | no op `mask.stats`, `Mask.coverage`, `Regions.filter`, `evaluate`, `Image` |
| E_ACCESS | 23 | `cv.open("speck-view.tif")`: reopening a file the agent saved, or a relative path |
| E_ARG_TYPE | 15 | `ids must be a number … got map[id:3]`, `show() takes a node or a look` |
| E_UNKNOWN_OPTION | 10 | guessed option names |
| E_WRONG_KIND | 6 | `overlay needs a Regions|Mask node, got a list`, `within needs a Mask, got an Image` |
| E_LAZY_COMPARE | 7 | comparing a lazy node in JS |

`{ space }` by op: stats/encoded 80, toGray/encoded 19, levels/encoded 13, exposure/encoded 11,
detect.specks/linear 10, gaussian/encoded 10, blend/encoded 7, mul/encoded 6, stats/linear 5, dog/linear 5.

## Usage Examples

```bash
python3 scripts/01-pi-api-usage.py \
  ~/code/wesen/2026-11-07--dust-scanning/ttmp/2026/11/08/CVJS-001--js-eval-cv-api-for-agents-compare-9-implementations/various/pi-runs
```

Full output at the time of writing (2026-11-09):

```text
runs 18 calls 751
  P.* params               0
  cv.evaluate              85
  await                    1139
  { space: }               198
  .look(                   167
  show(                    87
  mem.                     656
  cv.json/cv.csv           10
  .save(                   11
  as: (named image)        23
  encoding:                12
  .overlay(                69
  .crop(                   71
  .view(                   64
  cv.sheet(                12
  .where(                  43
  .sortBy(                 20
  .top(                    22
  .filter( (JS)            74
  .blend(                  13
  .channel(                0
  mask.gradient            6
{ space } by op: [(('stats', 'encoded'), 80), (('toGray', 'encoded'), 19), (('levels', 'encoded'), 13), (('exposure', 'encoded'), 11), (('detect.specks', 'linear'), 10), (('gaussian', 'encoded'), 10), (('blend', 'encoded'), 7), (('mul', 'encoded'), 6), (('stats', 'linear'), 5), (('dog', 'linear'), 5), (('compare', 'encoded'), 2), (('compare', 'linear'), 2), (('mask.luminance', 'linear'), 2), (('hp.levels', 'encoded'), 2), (('blend', 'linear'), 1)]
error codes: [('E_UNKNOWN_METHOD', 35), ('E_ACCESS', 23), ('E_ARG_TYPE', 15), ('E_UNKNOWN_OPTION', 10), ('E_ARG_RANGE', 10), ('E_LAZY_COMPARE', 7), ('E_WRONG_KIND', 6), ('E_FRAME', 4), ('E_NO_FIELD', 3), ('E_NO_DOCUMENT', 3), ('E_NAMESPACE', 2), ('E_NOT_ARRAY', 2), ('E_UNKNOWN_TOPIC', 2), ('E_NO_IMAGE', 1), ('E_TIMEOUT', 1)]
  E_UNKNOWN_METHOD e.g.: ['There is no op named "Image".', 'There is no op named "mask.stats".', 'There is no op named "evaluate".', 'There is no op named "Mask.coverage".', 'There is no op named "Regions.filter".']
  E_ACCESS e.g.: ['\'cv.open("7edbfe9c3e7ab7d6_512_1024.tif"): the path is outside the directories this session may read.\'', '\'cv.open("x.tif"): the path is outside the directories this session may read.\'', '\'cv.open("7edbfe9c3e7ab7d6_512_1024.tif"): the path is outside the directories this session may read.\'', '\'cv.open("speck-view.tif"): the path is outside the directories this session may read.\'']
  E_ARG_TYPE e.g.: ['mask.luminance needs above, below or both.', 'color = "lime" is not a colour name.', 'exposure is missing its ev argument.', 'show() takes a node or a look; got [object Object].', 'ids must be a number or a list of numbers, got map[id:3].']
  E_WRONG_KIND e.g.: ['overlay needs a Regions|Mask node as argument 1, got an object.', 'overlay needs a Regions|Mask node as argument 1, got a list.', 'within needs a Mask as argument 1, got an Image.', 'cv.compare.images[0] needs an Image node, got an object.']
  E_LAZY_COMPARE e.g.: []
E_ACCESS by run: [('run-12-gcv002-before', 4), ('run-16-gcv003-before', 4), ('run-17-gcv003-after', 2), ('run-11-gcv002-before', 1), ('run-15-gcv003-before', 1), ('run-18-gcv003-after', 1)]
value lifting (unawaited node as an option value):
   run-18-gcv003-after { black: 0, white: dog.stats().p999 }
   run-18-gcv003-after { black: 0, white: dog.stats().p999 }
   run-18-gcv003-after { black: 0, white: dog.stats().p999 }
   run-18-gcv003-after { black: 0, white: dog.stats().p99.mul(10) }
```

## Related

- `design-doc/01-design-and-implementation-guide.md` (this ticket): the changes these numbers motivate.
- GCV-001..003 PRs #2–#4 and their Codex review fixes (`4c79173`, `cd5f46c`, `319a6fc`): the bug classes the
  foundations part of GCV-004 makes impossible.
