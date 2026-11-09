package ops

import (
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// twoBoxes is a mask of two boxes (10×8 and 4×4 doc px) on the test image.
func twoBoxes(t *testing.T, e *graph.Engine, img *graph.Node) *graph.Node {
	a := call(t, e, "mask.box", img.ID, []any{4, 4, 10, 8})
	b := call(t, e, "mask.box", img.ID, []any{40, 30, 4, 4})
	return call(t, e, "or", a.ID, graph.NodeArg{ID: b.ID})
}

func regionsAt(t *testing.T, e *graph.Engine, n *graph.Node, scale float64) *graph.RegionsValue {
	t.Helper()
	return evalValue(t, e, n, scale).(*graph.RegionsValue)
}

// linked returns region set n's items at positions idx as JavaScript sends
// awaited items back: their fields and the link __cvref: [node, index].
func linked(t *testing.T, e *graph.Engine, n *graph.Node, idx ...int) []any {
	t.Helper()
	rv := regionsAt(t, e, n, 1)
	out := make([]any, len(idx))
	for k, i := range idx {
		m := map[string]any{}
		for key, v := range rv.Items[i].Data(n.Fields).Vals {
			m[key] = v
		}
		m["__cvref"] = []any{float64(n.ID), float64(i)}
		out[k] = m
	}
	return out
}

func coverageOf(t *testing.T, e *graph.Engine, m *graph.Node) float64 {
	t.Helper()
	return evalValue(t, e, call(t, e, "coverage", m.ID), 1).(*graph.DataValue).Data.(float64)
}

func TestComponentsAndMaskRegions(t *testing.T) {
	e, img := newEngine(t)
	comps := call(t, e, "components", twoBoxes(t, e, img).ID)
	rv := regionsAt(t, e, comps, 1)
	if len(rv.Items) != 2 {
		t.Fatalf("%d components", len(rv.Items))
	}
	big, small := rv.Items[0], rv.Items[1]
	if big.Box != [4]float64{4, 4, 10, 8} || big.Area != 80 || big.ID != 1 || small.Box != [4]float64{40, 30, 4, 4} || small.ID != 2 {
		t.Fatalf("components %+v %+v", big, small)
	}
	if math.Abs(big.Centroid[0]-9) > 1e-9 || math.Abs(big.Centroid[1]-8) > 1e-9 {
		t.Fatalf("centroid %v", big.Centroid)
	}
	// identical at any evaluation scale (analysed at scale 1)
	if rv2 := regionsAt(t, e, comps, 0.25); rv2.Items[0].Box != big.Box || len(rv2.Items) != 2 {
		t.Fatalf("components at 0.25 differ: %+v", rv2.Items[0])
	}
	// mask.regions coverage = region area / doc area at any scale
	tm := call(t, e, "coverage", call(t, e, "mask.regions", img.ID, nodeArg(comps)).ID)
	for _, sc := range []float64{1, 0.5, 0.3} {
		if got := evalValue(t, e, tm, sc).(*graph.DataValue).Data.(float64); math.Abs(got-96.0/(64*48)) > 1e-4 {
			t.Fatalf("mask.regions coverage at %g: %g", sc, got)
		}
	}
	if grown := coverageOf(t, e, call(t, e, "grow", call(t, e, "mask.regions", img.ID, nodeArg(comps)).ID, 2.0)); grown <= 96.0/(64*48) {
		t.Fatalf("mask.regions grow did not grow: %g", grown)
	}
	// a filtered array is the old only(ids): the one region's pixels
	if got := coverageOf(t, e, call(t, e, "mask.regions", img.ID, linked(t, e, comps, 1))); math.Abs(got-16.0/(64*48)) > 1e-6 {
		t.Fatalf("mask of item 2: coverage %g", got)
	}
	// the maxArea redirect is a JavaScript filter
	if ce := callErr(t, e, "components", twoBoxes(t, e, img).ID, map[string]any{"maxArea": 200.0}); ce.Code != graph.EUnknownOption || ce.Fix != "(await m.components()).filter(r => r.area <= 200)" {
		t.Fatalf("components maxArea: %+v", ce)
	}
}

// TestItemsResolveByPosition: linked items resolve to their own pixels by
// [node, index], whatever their id says; reordered, repeated, renumbered
// and labelled arrays keep their pixels; an edited box is refused when
// evaluated.
func TestItemsResolveByPosition(t *testing.T) {
	e, img := newEngine(t)
	m := twoBoxes(t, e, img)
	comps := call(t, e, "components", m.ID)
	items := linked(t, e, comps, 1, 0, 1)        // reordered and repeated
	items[0].(map[string]any)["id"] = 1.0        // renumbered: the small region now says id 1
	items[1].(map[string]any)["label"] = "big"   // a label
	items[2].(map[string]any)["note"] = "spread" // { ...r, note }
	meas := call(t, e, "measure", m.ID, items, map[string]any{"stats": []any{"mean"}})
	rv := regionsAt(t, e, meas, 1)
	if len(rv.Items) != 3 {
		t.Fatalf("%d items", len(rv.Items))
	}
	for k, want := range []struct {
		id    int
		label string
		area  float64
	}{{1, "", 16}, {1, "big", 80}, {2, "", 16}} {
		r := rv.Items[k]
		if r.ID != want.id || r.Label != want.label || r.Area != want.area || r.Pix == nil || r.Measures["meanL"] != 1 {
			t.Fatalf("item %d: %+v", k, r)
		}
	}
	if r := rv.Items[1].Data(meas.Fields).Vals; r["label"] != "big" {
		t.Fatalf("label field: %v", r)
	}
	// an edited box is refused, with the plain box as the fix
	moved := linked(t, e, comps, 0)
	moved[0].(map[string]any)["x"] = 5.0
	_, err := e.Evaluate(t.Context(), call(t, e, "mask.regions", img.ID, moved).ID, graph.EvalOptions{Scale: 1})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgType || !strings.Contains(ce.Message, "item 0 was moved") || !strings.HasPrefix(ce.Fix, "{ x: 5, y: 4, w: 10, h: 8 }") {
		t.Fatalf("moved item: %v", err)
	}
	// ids are numbers; text belongs in label
	bad := linked(t, e, comps, 0)
	bad[0].(map[string]any)["id"] = "A"
	if ce := callErr(t, e, "overlay", img.ID, bad); ce.Code != graph.EArgType || !strings.Contains(ce.Fix, `label: "A"`) {
		t.Fatalf("text id: %+v", ce)
	}
	// a linked item passed as a box or a point is its fields
	if c := call(t, e, "crop", img.ID, linked(t, e, comps, 0)[0]); c.Frame != (graph.Frame{X: 4, Y: 4, W: 10, H: 8}) {
		t.Fatalf("crop(item): %v", c.Frame)
	}
}

// TestItemsOverlayAndLiterals: an array overlays like its node; literal
// boxes and circles cover like mask.box and mask.ellipse; literal items mix
// with one set; two sets are refused; look-alikes that lost their link warn.
func TestItemsOverlayAndLiterals(t *testing.T) {
	e, img := newEngine(t)
	comps := call(t, e, "components", twoBoxes(t, e, img).ID)
	byNode := imgBuf(t, e, call(t, e, "overlay", img.ID, nodeArg(comps), map[string]any{"labels": "id"}))
	byArray := imgBuf(t, e, call(t, e, "overlay", img.ID, linked(t, e, comps, 0, 1), map[string]any{"labels": "id"}))
	if d := maxDiff(byNode, byArray); d != 0 {
		t.Fatalf("overlay(array) differs from overlay(node) by %g", d)
	}
	for _, tc := range []struct {
		items []any
		mask  *graph.Node
	}{
		{[]any{[]any{3.5, 7.25, 10.0, 6.5}}, call(t, e, "mask.box", img.ID, []any{3.5, 7.25, 10.0, 6.5})},
		{[]any{map[string]any{"x": 3.5, "y": 7.25, "w": 10.0, "h": 6.5}}, call(t, e, "mask.box", img.ID, []any{3.5, 7.25, 10.0, 6.5})},
		{[]any{map[string]any{"cx": 20.0, "cy": 15.5, "r": 6.0}}, call(t, e, "mask.ellipse", img.ID, []any{14.0, 9.5, 12.0, 12.0})},
	} {
		for _, sc := range []float64{1, 0.5} {
			a := evalValue(t, e, call(t, e, "mask.regions", img.ID, tc.items), sc).(*graph.ImageValue).Buf
			b := evalValue(t, e, tc.mask, sc).(*graph.ImageValue).Buf
			if d := maxDiff(a, b); d > 1e-6 {
				t.Fatalf("%v at %g: differs from %s by %g", tc.items, sc, tc.mask.Op.Name, d)
			}
		}
	}
	// gold points as circles next to a detector's items
	mixed := append(linked(t, e, comps, 0), map[string]any{"cx": 30.0, "cy": 20.0, "r": 3.0, "label": "gold"})
	cr, err := e.Call("overlay", img.ID, []any{mixed, map[string]any{"labels": "id"}})
	if err != nil || len(cr.Warnings) != 0 {
		t.Fatalf("mixed: %v %v", err, cr.Warnings)
	}
	imgBuf(t, e, cr.Node())
	// two sets
	other := call(t, e, "components", call(t, e, "mask.box", img.ID, []any{20, 20, 5, 5}).ID)
	two := append(linked(t, e, comps, 0), linked(t, e, other, 0)...)
	if ce := callErr(t, e, "overlay", img.ID, two); ce.Code != graph.EArgType || ce.Fix != "img.overlay(a).overlay(b)" || !strings.Contains(ce.Message, "two region sets") {
		t.Fatalf("two sets: %+v", ce)
	}
	// look-alikes: a map that built new objects lost the link
	lost := linked(t, e, comps, 0, 1)
	for _, it := range lost {
		delete(it.(map[string]any), "__cvref")
	}
	cr, err = e.Call("overlay", img.ID, []any{lost})
	if err != nil || len(cr.Warnings) != 1 || cr.Warnings[0].Code != graph.WItems || !strings.Contains(cr.Warnings[0].Message, "items 0–1 look like region items") {
		t.Fatalf("lost links: %v %v", err, cr.Warnings)
	}
	// points only where an op takes points
	if ce := callErr(t, e, "overlay", img.ID, []any{[]any{3.0, 4.0}}); ce.Code != graph.EArgType || !strings.Contains(ce.Message, "a list of 2 numbers; an item is a box") {
		t.Fatalf("point to overlay: %+v", ce)
	}
	if ce := callErr(t, e, "overlay", img.ID, map[string]any{"x": 1.0, "y": 1.0, "w": 2.0, "h": 2.0}); ce.Code != graph.EArgType || !strings.Contains(ce.Fix, "[item]") {
		t.Fatalf("one object: %+v", ce)
	}
	// the same array is the same node
	a1 := call(t, e, "mask.regions", img.ID, linked(t, e, comps, 0))
	a2 := call(t, e, "mask.regions", img.ID, linked(t, e, comps, 0))
	if a1 != a2 {
		t.Fatal("equal arrays are not hash-consed")
	}
}

// filtered is (await n).filter(keep): the linked items of region set n
// that keep accepts, as JavaScript sends them back.
func filtered(t *testing.T, e *graph.Engine, n *graph.Node, keep func(r *graph.Region) bool) []any {
	t.Helper()
	var idx []int
	for i, r := range regionsAt(t, e, n, 1).Items {
		if keep(r) {
			idx = append(idx, i)
		}
	}
	return linked(t, e, n, idx...)
}

// isDust is the agent's r.score >= 0.5.
func isDust(r *graph.Region) bool { return r.Measures["score"] >= DustScore }
