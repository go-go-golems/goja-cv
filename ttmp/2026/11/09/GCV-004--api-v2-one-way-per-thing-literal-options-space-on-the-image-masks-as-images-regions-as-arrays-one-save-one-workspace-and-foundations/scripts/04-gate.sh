#!/usr/bin/env bash
# GCV-004 implementation gate: gofmt, build, vet, race tests (real-data tests skipped:
# the DUST-002 tiles and labels drift; run them separately with 05-real-data.sh),
# lint, logcopter-check, smoke. Prints one line per step and exits non-zero on any failure.
set -u
cd "$(git rev-parse --show-toplevel)"
LOG=${GATE_LOG:-/tmp/gcv004-gate}
mkdir -p "$LOG"
fail=0
step() { local name=$1; shift; "$@" >"$LOG/$name.txt" 2>&1; local s=$?; echo "$name=$s"; [ $s -ne 0 ] && fail=1; return 0; }
fmt=$(gofmt -l cmd pkg internal); if [ -n "$fmt" ]; then echo "gofmt=1 $fmt"; fail=1; else echo "gofmt=0"; fi
step build go build ./...
step vet go vet ./...
step test env GOJACV_TILES=/nonexistent GOJACV_GOLD=/nonexistent go test ./... -count=1 -race
step lint make lint
step logcopter make logcopter-check
step smoke make smoke
if [ $fail -ne 0 ]; then grep -E -- '^(--- FAIL|FAIL|panic)' "$LOG/test.txt" | head -40; fi
echo gate=$fail
exit $fail
