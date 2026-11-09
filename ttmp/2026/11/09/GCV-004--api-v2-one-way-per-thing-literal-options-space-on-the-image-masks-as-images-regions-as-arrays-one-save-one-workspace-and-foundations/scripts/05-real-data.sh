#!/usr/bin/env bash
# GCV-004: the gated real-data tests (DUST-002 tiles in ~/dustdet/cv_tiles, labels in ~/dustds/gold),
# read-only. Prints their measured numbers so each phase can be compared with the batch-1 baseline.
set -u
cd "$(git rev-parse --show-toplevel)"
go test ./pkg/cv/ops -count=1 -run 'TestReal' -v 2>&1 | grep -E -- '^(=== RUN|--- |\s+\S+_test.go:)' 
echo real=${PIPESTATUS[0]}
