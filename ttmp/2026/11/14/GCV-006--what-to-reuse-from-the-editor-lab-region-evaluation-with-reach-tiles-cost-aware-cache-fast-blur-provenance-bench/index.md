---
Title: 'What to reuse from the Editor lab: region evaluation with reach, tiles, cost-aware cache, fast blur, provenance, bench'
Ticket: GCV-006
Status: active
Topics:
    - cv
    - architecture
    - performance
    - research
    - agents
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Compare the Editor lab (browser photo editor with a tiled, content-cached engine) with goja-cv; measure goja-cv on a 24 MP image; propose what to reuse: region evaluation with declared reach first."
LastUpdated: 2026-11-14T16:23:38.241365-05:00
WhatFor: ""
WhenToUse: ""
---

# What to reuse from the Editor lab: region evaluation with reach, tiles, cost-aware cache, fast blur, provenance, bench

## Overview

The Editor lab (`~/Downloads/editor-lab-work.zip`, 12–14 November 2026) is a photo editor in one browser page: a
tiled, content-cached engine whose layers declare their reach, an agent with one `eval` tool, a bench and a
textbook. This ticket compares it with goja-cv and proposes what to reuse. The main finding: goja-cv computes every
pixel op over its whole frame even when an agent crops a 64 px window, so a window of `gaussian(32)` on a 24 MP
image costs 3.1 s; region evaluation with declared reach would compute 0.27 % of that.

## Documents

- `design-doc/01-editor-lab-and-goja-cv-analysis-and-design-guide-for-reuse.md`: the guide (fundamentals, both
  systems, measurements, six proposals with decision records, phased plan, glossary).
- `reference/01-diary.md`: what was done, measured and found, step by step.
- `sources/`: the Editor lab files and third-party research sources the guide cites (`sources/README.md`).
- `scripts/`: the measurements (`01` blur comparison, `02` synthetic scan, `03` window costs, `04` goja per pixel,
  `05` PDF text extraction).

## Status

Current status: **active**. Analysis and guide done; the proposals are not yet agreed or implemented.
