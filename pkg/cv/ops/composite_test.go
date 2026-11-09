package ops

import (
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// TestWithinPlaceholder: the builder's graph on the placeholder is
// substituted onto the receiver (and is then the very node built directly),
// and another edit through the same mask re-runs only the edit and within.
func TestWithinPlaceholder(t *testing.T) {
	e, img := newEngine(t)
	sky := call(t, e, "feather", call(t, e, "and",
		call(t, e, "mask.luminance", img.ID, map[string]any{"above": 0.3}).ID,
		graph.NodeArg{ID: call(t, e, "mask.gradient", img.ID, map[string]any{"from": "top", "to": 0.6}).ID}).ID, 4)

	hole, err := e.Placeholder(img.ID)
	if err != nil {
		t.Fatal(err)
	}
	inner := call(t, e, "exposure", hole.ID, -0.4)
	w := call(t, e, "within", img.ID, graph.NodeArg{ID: sky.ID}, graph.BuiltArg{Root: inner.ID, Hole: hole.ID})
	direct := call(t, e, "exposure", img.ID, -0.4)
	if w.Inputs[2] != direct.ID {
		t.Fatalf("within's inner graph #%d is not the directly built exposure #%d", w.Inputs[2], direct.ID)
	}
	same := call(t, e, "within", img.ID, graph.NodeArg{ID: sky.ID}, graph.NodeArg{ID: direct.ID})
	if same != w {
		t.Fatal("within with a builder and with the equivalent node differ")
	}

	m1 := statOf(t, e, call(t, e, "stats", w.ID), "meanL")
	base := statOf(t, e, call(t, e, "stats", img.ID), "meanL")
	if !(m1 < base) {
		t.Fatalf("darkening inside the mask did not darken: %g vs %g", m1, base)
	}
	runsBefore := map[graph.NodeID]int{}
	for _, id := range []graph.NodeID{img.ID, sky.ID, sky.Inputs[0], direct.ID, w.ID} {
		runsBefore[id] = e.Evaluator().Runs(id)
	}
	darker := call(t, e, "exposure", img.ID, -1.5)
	w2 := call(t, e, "within", img.ID, graph.NodeArg{ID: sky.ID}, graph.NodeArg{ID: darker.ID})
	if m2 := statOf(t, e, call(t, e, "stats", w2.ID), "meanL"); !(m2 < m1) {
		t.Fatalf("a darker edit inside within did not darken: %g → %g", m1, m2)
	}
	if e.Evaluator().Runs(darker.ID) != 1 || e.Evaluator().Runs(w2.ID) != 1 {
		t.Fatal("the new edit and within did not run once")
	}
	for id, before := range runsBefore {
		if after := e.Evaluator().Runs(id); after != before {
			n, _ := e.Node(id)
			t.Fatalf("%s (#%d) ran %d more times, want 0", graph.Label(n), id, after-before)
		}
	}
	// a leaked placeholder cannot be evaluated
	_, err = e.Evaluate(t.Context(), inner.ID, graph.EvalOptions{Scale: 1})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EPlaceholder || ce.Fix == "" {
		t.Fatalf("leaked placeholder: %v", err)
	}
	// within with an empty mask is the identity; with a full mask it is the edit
	none := call(t, e, "mask.box", img.ID, []any{0, 0, 0, 0})
	id0 := evalValue(t, e, call(t, e, "within", img.ID, graph.NodeArg{ID: none.ID}, graph.NodeArg{ID: direct.ID}), 1).(*graph.ImageValue)
	src := evalValue(t, e, img, 1).(*graph.ImageValue)
	for i := range src.Buf.Pix {
		if math.Abs(float64(id0.Buf.Pix[i]-src.Buf.Pix[i])) > 1e-7 {
			t.Fatal("within an empty mask changed pixels")
		}
	}
}

func TestBlendModes(t *testing.T) {
	e, img := newEngine(t)
	dark := call(t, e, "exposure", img.ID, -1)
	src := evalValue(t, e, img, 1).(*graph.ImageValue).Buf
	for _, tc := range []struct {
		mode string
		f    func(a, b float32) float32
	}{
		{"normal", func(_, b float32) float32 { return b }},
		{"multiply", func(a, b float32) float32 { return a * b }},
		{"screen", func(a, b float32) float32 { return 1 - (1-a)*(1-b) }},
	} {
		out := evalValue(t, e, call(t, e, "blend", img.ID, graph.NodeArg{ID: dark.ID}, map[string]any{"mode": tc.mode, "opacity": 0.5}), 1).(*graph.ImageValue).Buf
		for _, i := range []int{0, 100, 1000, len(src.Pix) - 1} {
			a, b := src.Pix[i], src.Pix[i]/2
			want := a + (tc.f(a, b)-a)*0.5
			if math.Abs(float64(out.Pix[i]-want)) > 1e-5 {
				t.Fatalf("%s at %d: %v want %v", tc.mode, i, out.Pix[i], want)
			}
		}
	}
	// a mask restricts the blend
	m := call(t, e, "mask.box", img.ID, []any{0, 0, 32, 48})
	out := evalValue(t, e, call(t, e, "blend", img.ID, graph.NodeArg{ID: dark.ID}, map[string]any{"mask": graph.NodeArg{ID: m.ID}}), 1).(*graph.ImageValue).Buf
	if out.Pix[10] != src.Pix[10]/2 || out.Pix[50] != src.Pix[50] {
		t.Fatalf("masked blend: %v %v / %v %v", out.Pix[10], src.Pix[10], out.Pix[50], src.Pix[50])
	}
	if b := call(t, e, "blend", img.ID, graph.NodeArg{ID: dark.ID}, map[string]any{"opacity": 0}); evalValue(t, e, b, 1).(*graph.ImageValue).Buf != src {
		t.Fatal("blend at opacity 0 is not the identity")
	}
	// the removed modes redirect to arithmetic
	for mode, fix := range map[string]string{"difference": "a.sub(b).abs()", "add": "a.add(b)", "lighten": "a.max(b)", "darken": "a.min(b)"} {
		_, err := e.Call("blend", img.ID, []any{graph.NodeArg{ID: dark.ID}, map[string]any{"mode": mode, "opacity": 0.5}})
		ce, ok := graph.AsError(err)
		if !ok || ce.Code != graph.EArgType || ce.Fix != fix || !strings.Contains(ce.Hint, "{ opacity, mask }") {
			t.Fatalf("blend mode %s: %+v", mode, err)
		}
	}
}
