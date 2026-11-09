---
Title: 'Agent ergonomics and analysis primitives: script libraries, compact results, peaks, morphology'
Ticket: GCV-005
Status: active
Topics:
    - api
    - agents
    - analysis
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "goja-cv changes from the feedback of 39 annotator agents, on top of API v2 (GCV-004): task libraries (cv.load, cv.scripts, --prelude as a load), readable results (compact tables, mem function docs, UNUSED_NODE attribution, item labels), and analysis primitives (peaks; erode/dilate/opening/closing replacing mask grow/shrink)."
LastUpdated: 2026-11-09T20:30:00-05:00
WhatFor: "Entry point of the GCV-005 ticket."
WhenToUse: "Start here; the guide is design-doc/01."
---

# Agent ergonomics and analysis primitives: script libraries, compact results, peaks, morphology

## Overview

GCV-005 turns the feedback of 39 Claude Sonnet annotator agents (DUST-002 `reference/11`, 374 evals) into goja-cv
changes on top of GCV-004 (API v2). Part A makes task libraries a first-class part of a session and makes results
readable; Part B adds the analysis primitives agents improvised (`peaks`, morphology). The design and implementation
guide is `design-doc/01-design-and-implementation-guide.md` (revision 1); the coordinator's analysis is
`reference/01-…`; the diaries are `reference/02-…` (coordination) and `reference/03-…` (design writer).

Status: guide written, awaiting the review read; implementation follows GCV-004.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- api
- agents
- analysis

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
