package cvmod_test

import (
	"fmt"
	"strings"
	"testing"
)

// TestMasksAndRegionsFromJS: a mask is an image; awaited regions are a
// plain array whose items stay linked to their region through filter,
// sort, slice and object spread (goja copies enumerable symbol keys), which
// JSON and Object.keys do not see; ids are data; regions have no methods.
func TestMasksAndRegionsFromJS(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const boxes = img.mask.box([4, 4, 10, 8]).or(img.mask.box([40, 30, 4, 4]))
		const m = await boxes
		const regs = boxes.components()
		const r = await regs
		const big = r.filter(x => x.area > 50)
		const coverage = await boxes.coverage()
		// goja: spread copies the link; JSON and Object.keys ignore it (D6)
		const s = Symbol("s"), o = { a: 1, [s]: 7 }, c = { ...o }
		const spread = c[s] + ":" + Object.keys(c).length + ":" + JSON.stringify(c)
		// renumbered and sorted items keep their own pixels
		const renum = r.slice().reverse().map((x, i) => ({ ...x, id: i + 1, label: "r" + i }))
		const cov = async (items) => await img.mask.regions(items).coverage()
		const covBig = await cov(big), covAll = await cov(r), covRenum = await cov(renum), covNode = await cov(regs)
		const measured = await img.measure(renum, { stats: ["mean"] })
		return { m, coverage, isArray: Array.isArray(r), n: r.length, first: r[0], keys: Object.keys(r[0]).join(","),
			json: JSON.stringify(r[0]), spread, covBig, covAll, covRenum, covNode,
			measuredIds: measured.map(x => x.id + ":" + x.label + ":" + x.area).join(",") }`).(map[string]any)
	mm := got["m"].(map[string]any)
	if mm["kind"] != "Image" || mm["channels"] != int64(1) || fmt.Sprint(mm["frame"]) != "[0 0 64 48]" {
		t.Fatalf("mask summary %v", mm)
	}
	near(t, "coverage", got["coverage"], 96.0/(64*48), 1e-6)
	first := got["first"].(map[string]any)
	if got["isArray"] != true || got["n"] != int64(2) || fmt.Sprint(first["x"], first["y"], first["w"], first["h"]) != "4 4 10 8" {
		t.Fatalf("regions %v", got)
	}
	if got["keys"] != "id,x,y,w,h,cx,cy,area,aspect" || strings.Contains(got["json"].(string), "cv") || got["spread"] != `7:1:{"a":1}` {
		t.Fatalf("the link shows: keys %v json %v spread %v", got["keys"], got["json"], got["spread"])
	}
	near(t, "filtered coverage", got["covBig"], 80.0/(64*48), 1e-6)
	for _, k := range []string{"covAll", "covRenum", "covNode"} {
		near(t, k, got[k], 96.0/(64*48), 1e-6)
	}
	if got["measuredIds"] != "1:r0:16,2:r1:80" {
		t.Fatalf("renumbered items %v", got["measuredIds"])
	}
	// regions have no methods: lazy data and image methods say so
	e := mustFail(t, rt, "E_NOT_AWAITED", `return img.mask.box([4, 4, 10, 8]).components().filter(x => x.area > 1)`)
	if e.Fix != "(await node.components()).filter" {
		t.Fatalf("filter on a node: %+v", e)
	}
	mustFail(t, rt, "E_ARG_TYPE", `return img.exposure(x => 1)`)
	// a moved item is refused; a plain box is not
	mv := mustFail(t, rt, "E_ARG_TYPE", `const r = await img.mask.box([4, 4, 10, 8]).components(); return await img.mask.regions([{ ...r[0], x: 6 }]).coverage()`)
	if !strings.Contains(mv.Message, "was moved") {
		t.Fatalf("moved: %+v", mv)
	}
	plain := mustRun(t, rt, `const r = await img.mask.box([4, 4, 10, 8]).components(); const { x, y, w, h } = r[0]; return await img.mask.regions([{ x: x + 2, y, w, h }]).coverage()`)
	near(t, "plain box", plain, 80.0/(64*48), 1e-6)
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
