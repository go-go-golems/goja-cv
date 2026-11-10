package ops

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func evalValue(t *testing.T, e *graph.Engine, n *graph.Node, scale float64) graph.Value {
	t.Helper()
	v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: scale})
	if err != nil {
		t.Fatalf("evaluate %s at %g: %v", n.Op.Name, scale, err)
	}
	return v
}

// TestBoxCoverageIsDocPxInvariant: a box mask covers the same fraction of
// the image at every evaluation scale, because the box is in doc px.
func TestBoxCoverageIsDocPxInvariant(t *testing.T) {
	e, img := newEngine(t) // 64×48
	box := call(t, e, "mask.box", img.ID, []any{8, 6, 30, 20})
	cov := call(t, e, "coverage", box.ID)
	want := 30.0 * 20 / (64 * 48)
	for _, sc := range []float64{1, 0.5, 0.37, 0.25, 0.13} {
		got := evalValue(t, e, cov, sc).(*graph.DataValue).Data.(float64)
		if math.Abs(got-want) > 2e-3 {
			t.Fatalf("scale %g: coverage %g, want %g", sc, got, want)
		}
	}
	// algebra: box ∪ ¬box = everything, box ∩ ¬box ≈ nothing for a pixel-aligned box
	notBox := call(t, e, "not", box.ID)
	all := evalValue(t, e, call(t, e, "coverage", call(t, e, "or", box.ID, graph.NodeArg{ID: notBox.ID}).ID), 1).(*graph.DataValue).Data.(float64)
	none := evalValue(t, e, call(t, e, "coverage", call(t, e, "and", box.ID, graph.NodeArg{ID: notBox.ID}).ID), 1).(*graph.DataValue).Data.(float64)
	if all != 1 || none != 0 {
		t.Fatalf("box ∪ ¬box = %g, box ∩ ¬box = %g", all, none)
	}
	minus := evalValue(t, e, call(t, e, "coverage", call(t, e, "minus", box.ID, graph.NodeArg{ID: box.ID}).ID), 1).(*graph.DataValue).Data.(float64)
	if minus != 0 {
		t.Fatalf("box − box = %g", minus)
	}
	// ellipse area ≈ π/4 of its box
	ell := evalValue(t, e, call(t, e, "coverage", call(t, e, "mask.ellipse", img.ID, []any{8, 6, 40, 30}).ID), 1).(*graph.DataValue).Data.(float64)
	if math.Abs(ell-math.Pi/4*40*30/(64*48)) > 3e-3 {
		t.Fatalf("ellipse coverage %g", ell)
	}
	// edge gradient from the top to 0.5: mean 0.25
	g := evalValue(t, e, call(t, e, "coverage", call(t, e, "mask.gradient", img.ID, map[string]any{"from": "top", "to": 0.5}).ID), 1).(*graph.DataValue).Data.(float64)
	if math.Abs(g-0.25) > 0.01 {
		t.Fatalf("gradient coverage %g", g)
	}
}

func TestMaskArgumentErrors(t *testing.T) {
	e, img := newEngine(t)
	for _, tc := range []struct {
		op   string
		raw  []any
		code string
	}{
		{"mask.luminance", []any{map[string]any{"softness": 0.1}}, graph.EArgType},
		{"mask.box", []any{[]any{1, 2, 3}}, graph.EArgType},
		{"mask.gradient", []any{map[string]any{"from": "middle"}}, graph.EArgType},
		{"mask.gradient", []any{map[string]any{"from": "top", "to": 1.5}}, graph.EArgRange},
		{"mask.gradient", []any{map[string]any{"from": []any{0, 0}, "to": 0.5}}, graph.EArgType},
	} {
		_, err := e.Call(tc.op, img.ID, tc.raw)
		if ce, ok := graph.AsError(err); !ok || ce.Code != tc.code {
			t.Fatalf("%s %v: %v", tc.op, tc.raw, err)
		}
	}
	m := call(t, e, "mask.box", img.ID, []any{0, 0, 10, 10})
	_, err := e.Call("overlay", img.ID, []any{graph.NodeArg{ID: m.ID}, map[string]any{"color": "purpleish"}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgType || len(ce.Valid) == 0 {
		t.Fatalf("bad colour: %v", err)
	}
	_, err = e.Call("and", m.ID, []any{graph.NodeArg{ID: img.ID}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EWrongKind || ce.Fix != "img.toGray()" {
		t.Fatalf("colour image as mask input: %v", err)
	}
}

func TestParseColorAcceptsCSSNamesAndHex(t *testing.T) {
	for in, want := range map[any][3]float64{
		"lime":          {0, 1, 0},
		"Lime":          {0, 1, 0},
		"rebeccapurple": {0x66 / 255.0, 0x33 / 255.0, 0x99 / 255.0},
		"#ff8000":       {1, 0x80 / 255.0, 0},
		"#F80":          {1, 0x88 / 255.0, 0},
		"red":           {1, 0.1, 0.1}, // the overlay-tuned basics are kept
		"grey":          {0x80 / 255.0, 0x80 / 255.0, 0x80 / 255.0},
		"gray":          {0x80 / 255.0, 0x80 / 255.0, 0x80 / 255.0},
	} {
		got, err := parseColor(in, "color")
		if err != nil {
			t.Fatalf("%v: %v", in, err)
		}
		for i := range got {
			if math.Abs(float64(got[i])-want[i]) > 1e-6 { // display (encoded) values: overlays run in encoded space
				t.Fatalf("%v: %v, want %v", in, got, want)
			}
		}
	}
	got, err := parseColor([]any{0.0, 1.0, 0.0}, "color")
	if err != nil || got[1] != 1 {
		t.Fatalf("[0, 1, 0]: %v %v", got, err)
	}
	for _, bad := range []string{"#12", "#ggg", "limey", "chartreuse-ish"} {
		_, err := parseColor(bad, "color")
		ce, ok := graph.AsError(err)
		if !ok || ce.Code != graph.EArgType || ce.Fix == "" {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	_, err = parseColor("limey", "color")
	if ce, _ := graph.AsError(err); !strings.Contains(ce.Hint, `"lime"`) {
		t.Fatalf("limey hint: %+v", ce)
	}
}

// TestMasksAreOneChannelImages (GCV-004 §6.3, §7.5, D16): mask ops work on
// any one-channel image, a mask is measured like an image, a colour image
// is not a mask (E_WRONG_KIND with the toGray fix, at build time), masks
// are never converted between spaces, and a grey input is its own
// luminance (no toGray node).
func TestMasksAreOneChannelImages(t *testing.T) {
	e, img := newEngine(t)
	grey := call(t, e, "toGray", img.ID)
	grown := call(t, e, "grow", grey.ID, 2.0)
	if v := evalValue(t, e, grown, 1).(*graph.ImageValue); v.Buf.C != 1 || grown.Channels != 1 {
		t.Fatalf("img.toGray().grow(2): %d channels", v.Buf.C)
	}
	half := call(t, e, "mask.box", img.ID, []any{0.0, 0.0, 32.0, 48.0})
	if s := evalData(t, e, call(t, e, "stats", half.ID), 1).(map[string]any); math.Abs(s["meanL"].(float64)-0.5) > 1e-6 {
		t.Fatalf("m.stats().meanL = %v, want 0.5", s["meanL"])
	}
	edit := call(t, e, "exposure", img.ID, -0.4)
	for what, m := range map[string]*graph.Node{"img": img, "m.mul(img)": call(t, e, "mul", half.ID, graph.NodeArg{ID: img.ID})} {
		_, err := e.Call("within", img.ID, []any{graph.NodeArg{ID: m.ID}, graph.NodeArg{ID: edit.ID}})
		if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EWrongKind || ce.Fix != "img.toGray()" || !strings.Contains(ce.Message, "this image has 3 channels") {
			t.Fatalf("img.within(%s, …): %v", what, err)
		}
	}
	// the same weights in either space: the mask is the receiver's argument as built
	lin := call(t, e, graph.AsOp, img.ID, "linear")
	w1 := call(t, e, "within", img.ID, graph.NodeArg{ID: grey.ID}, graph.NodeArg{ID: edit.ID})
	w2 := call(t, e, "within", lin.ID, graph.NodeArg{ID: grey.ID}, graph.NodeArg{ID: call(t, e, "exposure", lin.ID, -0.4).ID})
	if w1.Args.Input("mask") != grey || w2.Args.Input("mask") != grey {
		t.Fatalf("mask converted: %s, %s", graph.Label(w1.Args.Input("mask")), graph.Label(w2.Args.Input("mask")))
	}
	// a grey input is its own luminance
	lum := call(t, e, "mask.luminance", grey.ID, map[string]any{"above": 0.5})
	if lines := e.Explain(lum.ID); strings.Count(strings.Join(lines, "\n"), "toGray") != 1 {
		t.Fatalf("a toGray inserted for a grey input: %q", lines)
	}
}
