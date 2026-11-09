---
Title: 'v1 field fixes: stepChroma gate, LOOKS warning, sheet badge, ruler labels'
Ticket: V1-FIXES
Status: active
Topics:
    - cv
    - measurement
    - looks
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "A small patch on main for a colleague validating dust labels, ahead of GCV-004."
LastUpdated: 2026-11-09T23:00:00-05:00
WhatFor: "Holds the diary of the v1 field fixes; belongs with GCV-004, which exists only on task/gcv-004-api-v2."
WhenToUse: "Read before reviewing task/v1-field-fixes or porting the fixes into GCV-004."
---

# v1 field fixes

## Overview

Four fixes to the v1 release found while validating film-dust gold points (DUST-002): stepChroma is null when
|snr| < 1, LOOKS/DATA warnings for items beyond the per-eval budget, the first sheet label clears the look
number, and ruler labels keep a gap. This is a stand-in ticket directory: `docmgr doc add --ticket GCV-004`
cannot find GCV-004 on `main`.

## Documents

- [Diary - v1 field fixes](reference/01-diary-v1-field-fixes.md)
