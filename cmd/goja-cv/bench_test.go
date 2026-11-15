package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseScenarioSplitsAtSeparators(t *testing.T) {
	evals, err := parseScenario("// a comment, not run\n// --- open\nmem.a = 1\n//---\nreturn mem.a\n// ---   last one  \nreturn 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(evals) != 3 || evals[0].name != "open" || evals[1].name != "eval 2" || evals[2].name != "last one" ||
		strings.TrimSpace(evals[0].code) != "mem.a = 1" || strings.Contains(evals[0].code, "comment") {
		t.Fatalf("evals: %+v", evals)
	}
	if _, err := parseScenario("return 1\n"); err == nil || !strings.Contains(err.Error(), "separator") {
		t.Fatalf("no separator: %v", err)
	}
	if _, err := parseScenario("// --- empty\n\n// --- x\nreturn 1\n"); err == nil || !strings.Contains(err.Error(), `"empty"`) {
		t.Fatalf("empty eval: %v", err)
	}
}

func TestPercentileIsNearestRank(t *testing.T) {
	xs := []float64{5, 1, 4, 2, 3}
	if percentile(xs, 50) != 3 || percentile(xs, 95) != 5 || percentile(xs, 1) != 1 || percentile(nil, 50) != 0 {
		t.Fatalf("p50 %v p95 %v p1 %v", percentile(xs, 50), percentile(xs, 95), percentile(xs, 1))
	}
}

// TestBenchReportsEvalsRunsAndCache: a cold repeat computes, the same request
// in a later eval of the same session is a cache hit with no op runs, and a
// failing eval is reported, not fatal.
func TestBenchReportsEvalsRunsAndCache(t *testing.T) {
	dir := t.TempDir()
	scenario := filepath.Join(dir, "s.js")
	src := "// test scenario\n// --- blur\nreturn await img.gaussian(2).stats()\n// --- again\nreturn await img.gaussian(2).stats()\n// --- broken\nreturn img.nope()\n"
	if err := os.WriteFile(scenario, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "bench", "--root", dir, "--read", filepath.Dir(tiffPath), "--image", tiffPath, "--repeat", "2", "--json", scenario)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	var doc struct {
		Scenarios []ScenarioReport `json:"scenarios"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(doc.Scenarios) != 1 || doc.Scenarios[0].Name != "s" || doc.Scenarios[0].Repeats != 2 || len(doc.Scenarios[0].Evals) != 3 {
		t.Fatalf("report: %+v", doc)
	}
	blur, again, broken := doc.Scenarios[0].Evals[0], doc.Scenarios[0].Evals[1], doc.Scenarios[0].Evals[2]
	if blur.Status != "ok" || blur.Runs < 1 || blur.CachedMB < 0 || blur.P50Ms <= 0 || blur.P95Ms < blur.P50Ms {
		t.Fatalf("blur: %+v", blur)
	}
	if again.Status != "ok" || again.Runs != 0 || again.CacheHits < 1 {
		t.Fatalf("again (cached): %+v", again)
	}
	if broken.Status == "ok" || !strings.Contains(broken.Error, "nope") {
		t.Fatalf("broken: %+v", broken)
	}
}

// TestTotalIsTheMedianOfRepeatTotals: total_p50_ms is the median of the
// per-repeat sums, not the sum of per-eval medians.
func TestTotalIsTheMedianOfRepeatTotals(t *testing.T) {
	evals := []benchEval{{name: "a"}, {name: "b"}}
	samples := [][]sample{
		{{ms: 100, status: "ok"}, {ms: 1, status: "ok"}, {ms: 1, status: "ok"}},
		{{ms: 1, status: "ok"}, {ms: 100, status: "ok"}, {ms: 1, status: "ok"}},
	}
	if got := summarize("s.js", evals, samples, 3).TotalP50; got != 101 {
		t.Fatalf("total p50 = %v, want 101 (repeat totals 101, 101, 2)", got)
	}
}
