package cvmod_test

import (
	"fmt"
	"strings"
	"testing"
)

// TestMasksAndRegionsFromJS: mask summaries, region summaries, where with a
// param and the filter data lambda (runs on the owner goroutine through
// Owner.Call while the evaluation runs on its own goroutine).
func TestMasksAndRegionsFromJS(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const boxes = img.mask.box([4, 4, 10, 8]).or(img.mask.box([40, 30, 4, 4]))
		const m = await boxes
		const regs = boxes.components()
		const r = await regs
		const big = regs.filter(x => x.area > 50)
		const f = await big
		const same = regs.filter(x => x.area > 50) === big
		const n1 = await regs.where({ area: { gte: 50 } }).count()
		const n2 = await regs.where({ area: { gte: 5 } }).count()
		const coverage = await boxes.coverage()
		return { m, coverage, count: r.count, first: r.items[0], fcount: f.count, fid: f.items[0].id, same, n1, n2 }`).(map[string]any)
	m := got["m"].(map[string]any)
	if m["kind"] != "Mask" || fmt.Sprint(m["frame"]) != "[0 0 64 48]" {
		t.Fatalf("mask summary %v", m)
	}
	near(t, "coverage", got["coverage"], 96.0/(64*48), 1e-6)
	first := got["first"].(map[string]any)
	if fmt.Sprint(first["x"], first["y"], first["w"], first["h"]) != "4 4 10 8" || got["count"] != int64(2) {
		t.Fatalf("regions %v", got)
	}
	if got["fcount"] != int64(1) || got["fid"] != int64(1) || got["same"] != false {
		t.Fatalf("filter %v", got)
	}
	if got["n1"] != int64(1) || got["n2"] != int64(2) {
		t.Fatalf("where %v", got)
	}
	// the same function object gives the same node
	same := mustRun(t, rt, `const regs = img.mask.box([4, 4, 10, 8]).components(); const fn = x => x.area > 1; return regs.filter(fn) === regs.filter(fn)`)
	if same != true {
		t.Fatal("same function gave different nodes")
	}
	e := mustFail(t, rt, "E_EVAL", `return await img.mask.box([4, 4, 10, 8]).components().filter(x => { throw new Error("boom") })`)
	if e.Hint == "" {
		t.Fatalf("filter error %+v", e)
	}
	mustFail(t, rt, "E_ARG_TYPE", `return img.exposure(x => 1)`)
}

func TestWithinBuilderFromJS(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const sky = img.mask.luminance({ above: 0.3 }).and(img.mask.gradient({ from: "top", to: 0.6 })).feather(4)
		let seen = 0
		const out = img.within(sky, i => { seen++; return i.exposure(-0.4) })
		const again = img.within(sky, i => i.exposure(-0.4))
		const direct = img.within(sky, img.exposure(-0.4))
		const a = (await out.stats()).meanL
		const b = (await img.within(sky, i => i.exposure(-1.5)).stats()).meanL
		return { seen, same: out === again && out === direct, a, b, explain: cv.explain(out).graph }`).(map[string]any)
	if got["seen"] != int64(1) || got["same"] != true {
		t.Fatalf("builder: %v", got)
	}
	if !(got["b"].(float64) < got["a"].(float64)) {
		t.Fatalf("a darker edit inside within did not darken: %v", got)
	}
	lines := fmt.Sprint(got["explain"])
	if !strings.Contains(lines, "within(mask: #") || !strings.Contains(lines, "exposure(ev: -0.4)") || strings.Contains(lines, "placeholder") {
		t.Fatalf("explain: %s", lines)
	}
	mustFail(t, rt, "E_ARG_TYPE", `return img.within(img.mask.box([0, 0, 4, 4]), i => 3)`)
	e := mustFail(t, rt, "E_ARG_TYPE", `return img.within(img.mask.box([0, 0, 4, 4]), async i => i.exposure(1))`)
	if !strings.Contains(e.Message, "async") {
		t.Fatalf("async builder: %+v", e)
	}
	mustFail(t, rt, "E_PLACEHOLDER", `let leak; img.within(img.mask.box([0, 0, 4, 4]), i => { leak = i; return i }); return await leak.stats()`)
	mustFail(t, rt, "E_WRONG_KIND", `return img.within(img, i => i)`)
}
