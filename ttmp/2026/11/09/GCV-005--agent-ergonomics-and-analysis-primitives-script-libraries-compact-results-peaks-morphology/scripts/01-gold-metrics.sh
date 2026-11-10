#!/usr/bin/env bash
# GCV-005 §15.2: detection quality of peaks, top-hats and line openings on the DUST-002 gold tiles.
# Runs the gated Go tests (pkg/cv/ops/peaks_gold_test.go: TestRealPeaksPosition, TestRealPeaksCoverage with the
# snr sweep 0.5/1/1.4/2 and the time per tile, TestRealPeaksDarkAreas, TestRealTopHatSaturated, TestRealLineOpening)
# and the gated real-tile benchmark, and prints their numbers. Read-only: tiles in $GOJACV_TILES
# (default ~/dustdet/cv_tiles), labels in $GOJACV_GOLD (default ~/dustds/gold). Exit status: the tests'.
set -u
cd "$(git rev-parse --show-toplevel)"
out=$(go test ./pkg/cv/ops -count=1 -v -run 'TestRealPeaks|TestRealTopHatSaturated|TestRealLineOpening' 2>&1)
status=$?
printf '%s\n' "$out" | grep -E -v '^(=== RUN|PASS$|ok |FAIL$)' | sed -E 's/^ +peaks_gold_test\.go:[0-9]+: /  /'
go test ./pkg/cv/ops -run XXX -bench RealPeaks -benchtime 5x -count=1 2>&1 | grep -E '^Benchmark'
exit $status
