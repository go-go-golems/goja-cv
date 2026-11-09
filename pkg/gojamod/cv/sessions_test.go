package cvmod_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// plantedPath writes the planted-fault scan for one test.
func plantedPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), testimage.PlantedTIFF)
	if _, err := testimage.WritePlanted(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return p
}

func nodeByOp(t *testing.T, e *graph.Engine, name string) []*graph.Node {
	t.Helper()
	var out []*graph.Node
	for id := graph.NodeID(1); ; id++ {
		n, ok := e.Node(id)
		if !ok {
			return out
		}
		if n.Op.Name == name {
			out = append(out, n)
		}
	}
}

// TestSessionB runs Part 1 Session B ("Find the dust and remove it"),
// adapted to M3: no show/doc.propose yet, so looks are awaited overlays and
// the heal is awaited. Each body is one eval; globalThis carries session
// variables between evals as the eval tool's persistent scope will.
func TestSessionB(t *testing.T) {
	rt := newRuntime(t)
	scan := plantedPath(t)
	// eval 1
	r1 := mustRun(t, rt, fmt.Sprintf(`
		const scan = globalThis.scan = cv.open(%q)
		const specks = globalThis.specks = scan.detect.specks({ maxArea: 120, minContrast: 0.04 })
		const r = await specks
		const look = await scan.overlay(specks, { labels: "id", color: "red" })
		const star = r.items.reduce((a, b) => (b.score < a.score ? b : a))
		return { count: r.count, largest: r.items.slice(0, 5), look, star }`, scan)).(map[string]any)
	if r1["count"] != int64(41) {
		t.Fatalf("eval 1 count %v", r1["count"])
	}
	largest := r1["largest"].([]any)
	first := largest[0].(map[string]any)
	for _, k := range []string{"id", "box", "area", "contrast", "score", "localNoise", "polarity"} {
		if _, ok := first[k]; !ok {
			t.Fatalf("speck item lacks %q: %v", k, first)
		}
	}
	star := r1["star"].(map[string]any)
	starID := star["id"]
	if c := star["centroid"].([]any); fmt.Sprint(c) != "[200.56 46.56]" && !(c[0].(float64) > 198 && c[0].(float64) < 203) {
		t.Fatalf("lowest-score region is not the star: %v", star)
	}
	if look := r1["look"].(map[string]any); look["kind"] != "Image" || look["channels"] != int64(3) {
		t.Fatalf("look %v", look)
	}
	t.Logf("eval 1: count %v, largest %v, star id %v score %v", r1["count"], first, starID, star["score"])

	// eval 2: exclude the star by id, keep a score threshold the person can move
	r2 := mustRun(t, rt, fmt.Sprintf(`
		const minScore = P.slider("minScore", { min: 0, max: 1, value: 0.5, label: "Speck confidence" })
		const keep = globalThis.keep = specks.where({ score: { gte: minScore } }).without([%v])
		const healed = globalThis.healed = scan.heal(keep, { radius: 3 })
		const n = await keep.count()
		const h = await healed
		const after = await healed.check.dust()
		return { n, h, after: after.verdict, afterCount: after.count }`, starID)).(map[string]any)
	if r2["n"] != int64(40) || r2["after"] != "clean" {
		t.Fatalf("eval 2: %v", r2)
	}
	eng := engineOf(t, rt)
	detect := nodeByOp(t, eng, "detect.specks")
	heal := nodeByOp(t, eng, "heal")
	where := nodeByOp(t, eng, "where")
	if len(detect) != 1 || len(heal) != 1 || len(where) != 1 {
		t.Fatalf("nodes: detect %d heal %d where %d", len(detect), len(heal), len(where))
	}
	runsDetect, runsHeal := eng.Evaluator().Runs(detect[0].ID), eng.Evaluator().Runs(heal[0].ID)

	// eval 3: the person drags the slider to 0 — the star stays excluded by id,
	// detection does not re-run, only where and below
	r3 := mustRun(t, rt, `
		P.set("minScore", 0)
		const n = await keep.count()
		await healed
		return { n, touched: P.list().find(p => p.name === "minScore").touched }`).(map[string]any)
	if r3["n"] != int64(40) || r3["touched"] != true {
		t.Fatalf("eval 3: %v", r3)
	}
	if eng.Evaluator().Runs(detect[0].ID) != runsDetect || eng.Evaluator().Runs(heal[0].ID) != runsHeal+1 {
		t.Fatalf("runs: detect %d→%d heal %d→%d", runsDetect, eng.Evaluator().Runs(detect[0].ID), runsHeal, eng.Evaluator().Runs(heal[0].ID))
	}
	// the same regions at preview scale and at full scale (doc px)
	same := mustRun(t, rt, `
		const a = await cv.evaluate(keep, { scale: 0.5 }), b = await cv.evaluate(keep, { scale: "full" })
		return JSON.stringify(a.items) === JSON.stringify(b.items) && a.count === b.count`)
	if same != true {
		t.Fatal("regions differ between scale 0.5 and full")
	}
	h := mustRun(t, rt, `return cv.help("heal").heal`).(map[string]any)
	if !strings.Contains(fmt.Sprint(h["args"]), "doc px") || h["returns"] != "Image" {
		t.Fatalf("help heal %v", h)
	}
}

// TestSessionC runs Part 1 Session C ("Darken the sky a little") adapted to
// M3 on the planted scan (its bright top-right stands in for the sky; there
// is no saturation op yet, so the inner edit is exposure + contrast).
func TestSessionC(t *testing.T) {
	rt := newRuntime(t)
	scan := plantedPath(t)
	got := mustRun(t, rt, fmt.Sprintf(`
		const sc = cv.open(%q)
		const sky = sc.mask.luminance({ above: 0.55 })
		  .and(sc.mask.gradient({ from: "top", to: 0.45 }))   // fades from the top edge to 45%% height
		  .feather(20)
		const look = await sky.overlay(sc)
		const skyEv = P.slider("skyEv", { min: -2, max: 1, value: -0.4 })
		const out = globalThis.out = sc.within(sky, i => i.exposure(skyEv).contrast(0.1))
		const coverage = await sky.coverage()
		const coverageHalf = await cv.evaluate(sky.coverage(), { scale: 0.5 })
		const before = await sc.stats().meanL, after = await out.stats().meanL
		P.set("skyEv", -1.2)
		const after2 = await out.stats().meanL
		return { coverage, coverageHalf, look, before, after, after2, explain: await out.explain() }`, scan)).(map[string]any)
	cov, covHalf := got["coverage"].(float64), got["coverageHalf"].(float64)
	if cov < 0.02 || cov > 0.5 || covHalf < cov*0.9 || covHalf > cov*1.1 {
		t.Fatalf("sky coverage %v at 1.0, %v at 0.5", cov, covHalf)
	}
	before, after, after2 := got["before"].(float64), got["after"].(float64), got["after2"].(float64)
	if !(after2 < after && after < before) {
		t.Fatalf("meanL %v → %v → %v", before, after, after2)
	}
	ex := fmt.Sprint(got["explain"])
	if !strings.Contains(ex, "within()") || !strings.Contains(ex, "feather(radius: 20 px") || !strings.Contains(ex, "exposure(ev: #") {
		t.Fatalf("explain %s", ex)
	}
	t.Logf("sky coverage %.4f (0.5: %.4f); meanL %.4f → %.4f → %.4f", cov, covHalf, before, after, after2)
	eng := engineOf(t, rt)
	for _, op := range []string{"feather", "mask.luminance", "and"} {
		for _, n := range nodeByOp(t, eng, op) {
			if r := eng.Evaluator().Runs(n.ID); r > 2 { // once per scale (1.0 and 0.5)
				t.Fatalf("%s ran %d times; moving skyEv must not re-run the mask", op, r)
			}
		}
	}
}
