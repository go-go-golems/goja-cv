package ops

import (
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph/graphtest"
)

// testEngine is an engine whose registry also has the test source op.
func testEngine(t *testing.T) *graph.Engine {
	t.Helper()
	reg := NewRegistry()
	graphtest.RegisterSource(reg)
	return graph.NewEngine(reg, graph.EngineOptions{})
}

func testSource(t *testing.T, e *graph.Engine, enc string, f graph.Frame, fill ...float64) *graph.Node {
	t.Helper()
	n, err := graphtest.Source(e, enc, f, fill...)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func callErr(t *testing.T, e *graph.Engine, name string, recv graph.NodeID, raw ...any) *graph.Error {
	t.Helper()
	_, err := e.Call(name, recv, raw)
	ce, ok := graph.AsError(err)
	if !ok {
		t.Fatalf("%s: want a cv error, got %v", name, err)
	}
	return ce
}

// window is a 64×48 window of a larger scan at (100, 50).
var window = graph.Frame{X: 100, Y: 50, W: 64, H: 48}

func TestFramesPropagate(t *testing.T) {
	e := testEngine(t)
	src := testSource(t, e, "srgb", window, 0.2, 0.3, 0.4)
	for _, n := range []*graph.Node{
		call(t, e, "exposure", src.ID, 0.5),
		call(t, e, "gaussian", src.ID, 2.0),
		call(t, e, "mask.luminance", src.ID, map[string]any{"above": 0.5}),
	} {
		if n.Frame != window {
			t.Errorf("%s frame %s, want %s", n.Op.Name, n.Frame, window)
		}
	}
	v := evalValue(t, e, call(t, e, "exposure", src.ID, 0.5), 1).(*graph.ImageValue)
	if v.Frame != window || v.Buf.W != 64 {
		t.Fatalf("evaluated frame %s, buffer %v", v.Frame, v.Buf)
	}
	if got := graph.ImageSummary(call(t, e, "exposure", src.ID, 0.5))["frame"]; len(got.([]any)) != 4 || got.([]any)[0] != 100.0 {
		t.Fatalf("summary frame %v", got)
	}
}

func TestBoxAndPointTypes(t *testing.T) {
	e, img := newEngine(t) // 64×48 at the origin
	ref := call(t, e, "mask.box", img.ID, []any{4.0, 6.0, 20.0, 10.0})
	// the object form (extra keys ignored, so a region item works), positionally and as an option
	for _, raw := range [][]any{
		{map[string]any{"x": 4.0, "y": 6.0, "w": 20.0, "h": 10.0}},
		{map[string]any{"x": 4.0, "y": 6.0, "w": 20.0, "h": 10.0, "id": 3.0, "area": 200.0}},
		{map[string]any{"box": map[string]any{"x": 4.0, "y": 6.0, "w": 20.0, "h": 10.0}}},
	} {
		if n := call(t, e, "mask.box", img.ID, raw...); n.ID != ref.ID {
			t.Errorf("mask.box(%v) is #%d, want the node of the list form #%d", raw, n.ID, ref.ID)
		}
	}
	// points: [x, y] or { cx, cy }
	p := call(t, e, "mask.flood", img.ID, []any{10.0, 12.0})
	if n := call(t, e, "mask.flood", img.ID, map[string]any{"cx": 10.0, "cy": 12.0}); n.ID != p.ID {
		t.Error("{ cx, cy } is not the point [x, y]")
	}
	// { x, y } is a box corner, not a point
	ce := callErr(t, e, "mask.flood", img.ID, map[string]any{"x": 10.0, "y": 12.0})
	if ce.Code != graph.EArgType || !strings.Contains(ce.Message, "x, y is a box corner") || !strings.HasPrefix(ce.Fix, "[10, 12]") {
		t.Fatalf("{x, y} as a point: %+v", ce)
	}
	// wrong shapes
	if ce := callErr(t, e, "mask.box", img.ID, []any{1.0, 2.0, 3.0}); ce.Code != graph.EArgType {
		t.Fatalf("3-element box: %v", ce)
	}
	if ce := callErr(t, e, "stats", img.ID, map[string]any{"box": "everything"}); ce.Code != graph.EArgType {
		t.Fatalf("string box: %v", ce)
	}
	// the gradient's from/to: an edge and a fraction, or two points
	call(t, e, "mask.gradient", img.ID, map[string]any{"from": "left", "to": 0.3})
	call(t, e, "mask.gradient", img.ID, map[string]any{"from": []any{0.0, 0.0}, "to": map[string]any{"cx": 30.0, "cy": 20.0}})
	if ce := callErr(t, e, "mask.gradient", img.ID, map[string]any{"from": "middle"}); ce.Code != graph.EArgType {
		t.Fatalf("bad edge: %v", ce)
	}
}

func TestAbsoluteCoordinatesInAWindow(t *testing.T) {
	e := testEngine(t)
	src := testSource(t, e, "srgb", window, 0.5)

	// a box in the window's (absolute) coordinates
	box := call(t, e, "mask.box", src.ID, []any{110.0, 60.0, 10.0, 10.0})
	m := evalValue(t, e, box, 1).(*graph.ImageValue)
	if m.Frame != window {
		t.Fatalf("mask frame %s", m.Frame)
	}
	if got := m.Coverage(); math.Abs(got-100.0/(64*48)) > 1e-9 {
		t.Fatalf("coverage %g", got)
	}
	pl := m.Buf.Plane(0)
	if pl[10*64+10] != 1 || pl[19*64+19] != 1 || pl[9*64+10] != 0 || pl[10*64+20] != 0 {
		t.Fatal("the box is not at buffer (10, 10)-(20, 20)")
	}
	// at a preview scale too
	half := evalValue(t, e, box, 0.5).(*graph.ImageValue)
	if got := half.Coverage(); math.Abs(got-100.0/(64*48)) > 1e-9 {
		t.Fatalf("coverage at 0.5: %g", got)
	}

	// regions found in the window report absolute geometry
	items := evalValue(t, e, call(t, e, "components", box.ID), 1).(*graph.RegionsValue).Items
	if len(items) != 1 || items[0].Box != [4]float64{110, 60, 10, 10} || items[0].Centroid != [2]float64{115, 65} {
		t.Fatalf("region %+v", items)
	}

	// stats inside an absolute box
	s := evalData(t, e, call(t, e, "stats", src.ID, map[string]any{"box": []any{110.0, 60.0, 10.0, 10.0}}), 1).(map[string]any)
	if s["area"] != 100.0 {
		t.Fatalf("area %v", s["area"])
	}

	// a box or point outside the frame is refused with the nearest valid one
	ce := callErr(t, e, "stats", src.ID, map[string]any{"box": []any{0.0, 0.0, 10.0, 10.0}})
	if ce.Code != graph.EArgRange || !strings.Contains(ce.Message, window.String()) || ce.Fix != "img.stats({ box: [100, 50, 10, 10] })" {
		t.Fatalf("box outside: %+v", ce)
	}
	if ce := callErr(t, e, "mask.flood", src.ID, []any{10.0, 10.0}); ce.Code != graph.EArgRange || ce.Fix != "img.mask.flood([100, 50])" {
		t.Fatalf("point outside: %+v", ce)
	}
	call(t, e, "mask.flood", src.ID, []any{120.0, 70.0})
}

func TestTwoFramesAreRefused(t *testing.T) {
	e := testEngine(t)
	a := testSource(t, e, "srgb", window, 0.5)
	b := testSource(t, e, "srgb", graph.Frame{X: 0, Y: 0, W: 64, H: 48}, 0.5)
	n := call(t, e, "blend", a.ID, graph.NodeArg{ID: b.ID})
	_, err := e.Evaluate(t.Context(), n.ID, graph.EvalOptions{Scale: 1})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EFrame || !strings.Contains(ce.Message, window.String()) {
		t.Fatalf("blend of two frames: %v", err)
	}
}

func TestReturnedInputsAreStampedCopies(t *testing.T) {
	e := testEngine(t)
	// one channel, linear-encoded (no conversions around the physical ops): toGray returns its input
	src := testSource(t, e, "linear", window, 0.5)
	sv := evalValue(t, e, src, 1).(*graph.ImageValue)
	for _, n := range []*graph.Node{call(t, e, "toGray", src.ID), call(t, e, "exposure", src.ID, 0.0)} {
		v := evalValue(t, e, n, 1).(*graph.ImageValue)
		if v == sv {
			t.Errorf("%s returned the source's cached value object", n.Op.Name)
		}
		if v.Buf != sv.Buf || v.Frame != window {
			t.Errorf("%s: want a copy sharing the buffer with the node's frame", n.Op.Name)
		}
	}
}
