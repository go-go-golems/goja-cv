package ops

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func near(t *testing.T, what string, got float32, want, tol float64) {
	t.Helper()
	if math.Abs(float64(got)-want) > tol {
		t.Fatalf("%s = %g, want %g ± %g", what, got, want, tol)
	}
}

func TestSplitIsThreeChannels(t *testing.T) {
	e := testEngine(t)
	img := testSource(t, e, "srgb", window, 0.1, 0.2, 0.3)
	r, err := e.Call("split", img.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Nodes) != 3 || r.Node() != nil {
		t.Fatalf("split built %d nodes", len(r.Nodes))
	}
	for i, c := range []any{"r", "g", "b"} {
		n := call(t, e, "channel", img.ID, c)
		if r.Nodes[i].ID != n.ID {
			t.Fatalf("split()[%d] is #%d, channel(%v) is #%d", i, r.Nodes[i].ID, c, n.ID)
		}
		v := evalValue(t, e, n, 1).(*graph.ImageValue)
		near(t, "channel value", v.Buf.Pix[0], []float64{0.1, 0.2, 0.3}[i], 1e-7)
		if v.Buf.C != 1 || n.Enc != buffer.EncSRGB || n.Space != buffer.SpaceLinear || n.Frame != window {
			t.Fatalf("channel statics %s %s", graph.Statics(n), n.Frame)
		}
	}
	grey := testSource(t, e, "srgb", window, 0.5)
	call(t, e, "channel", grey.ID, "r")
	g := call(t, e, "channel", grey.ID, "g")
	if _, err := e.Evaluate(t.Context(), g.ID, graph.EvalOptions{Scale: 1}); err == nil {
		t.Fatal("channel g of a grey image")
	} else if ce, _ := graph.AsError(err); ce.Code != graph.EArgRange || ce.Fix != "img.toGray()" {
		t.Fatalf("channel g of grey: %+v", ce)
	}
	if ce := callErr(t, e, "channel", img.ID, 1.5); ce.Code != graph.EArgType {
		t.Fatalf("channel 1.5: %+v", ce)
	}
	if ce := callErr(t, e, "channel", img.ID, "alpha"); ce.Code != graph.EArgType {
		t.Fatalf("channel alpha: %+v", ce)
	}
}

func TestOneNodeMacro(t *testing.T) {
	e := testEngine(t)
	if err := e.Registry().Register(graph.OpDef{
		Name: "testview", On: graph.KindImage, Returns: graph.KindImage,
		Space: graph.SpaceRule{Default: buffer.SpaceEncoded},
		Args:  []graph.ArgSpec{{Name: "r", Type: "number", Unit: graph.DocPx, Positional: true, Default: 2.0}},
		Expand: func(x *graph.Expander, recv *graph.Node, a graph.Args) ([]*graph.Node, error) {
			blur, err := x.Call("gaussian", recv, a.DocLength("r"))
			if err != nil {
				return nil, err
			}
			c, err := x.Call("chroma", blur, map[string]any{"space": a.String("space")})
			if err != nil {
				return nil, err
			}
			return []*graph.Node{c}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	img := testSource(t, e, "srgb", window, 0.1, 0.2, 0.3)
	m := call(t, e, "testview", img.ID, 3.0)
	chain := call(t, e, "chroma", call(t, e, "gaussian", img.ID, 3.0).ID)
	if m.Inputs[len(m.Inputs)-1] != chain.ID || m.Inputs[0] != img.ID {
		t.Fatalf("macro inputs %v, hand-written chain #%d", m.Inputs, chain.ID)
	}
	if m.Enc != buffer.EncLinear || m.Space != buffer.SpaceLinear || m.Frame != window {
		t.Fatalf("a derived expansion makes a derived macro node: %s", graph.Statics(m))
	}
	if graph.Label(m) != "testview" || !strings.HasPrefix(graph.Describe(e.Builder(), m, 1), "testview(r: 3 px)") {
		t.Fatalf("macro label %q / %q", graph.Label(m), graph.Describe(e.Builder(), m, 1))
	}
	mv := evalValue(t, e, m, 1).(*graph.ImageValue)
	cv := evalValue(t, e, chain, 1).(*graph.ImageValue)
	if mv.Buf != cv.Buf || e.Evaluator().Runs(m.ID) != 0 || e.Evaluator().Runs(chain.ID) != 1 {
		t.Fatalf("macro evaluation: shared %v, runs %d / %d", mv.Buf == cv.Buf, e.Evaluator().Runs(m.ID), e.Evaluator().Runs(chain.ID))
	}
	if cv.Enc != buffer.EncLinear {
		t.Fatal("the expansion's cached value changed")
	}
	lines := e.Explain(m.ID, 1)
	if len(lines) != 5 || !strings.HasPrefix(lines[0], "#") || !strings.Contains(lines[0], "testview") ||
		!strings.HasPrefix(lines[1], "  #") || !strings.Contains(lines[1], "chroma") {
		t.Fatalf("explain: %q", lines)
	}
}

func TestToLabChromaColormapEncode(t *testing.T) {
	e := testEngine(t)
	// toLab: linear sRGB white → (1, 0, 0); red → (53.24, 80.09, 67.20)/100
	white := testSource(t, e, "srgb", window, 1, 1, 1)
	red := testSource(t, e, "srgb", window, 1, 0, 0)
	lab := evalValue(t, e, call(t, e, "toLab", white.ID), 1).(*graph.ImageValue)
	n := lab.Buf.N()
	near(t, "L* white", lab.Buf.Pix[0], 1, 1e-4)
	near(t, "a* white", lab.Buf.Pix[n], 0, 1e-3)
	near(t, "b* white", lab.Buf.Pix[2*n], 0, 1e-3)
	ln := call(t, e, "toLab", red.ID)
	lab = evalValue(t, e, ln, 1).(*graph.ImageValue)
	near(t, "L* red", lab.Buf.Pix[0], 0.5324, 1e-3)
	near(t, "a* red", lab.Buf.Pix[n], 0.8009, 1e-3)
	near(t, "b* red", lab.Buf.Pix[2*n], 0.6720, 1e-3)
	if ln.Enc != buffer.EncLinear || lab.Enc != buffer.EncLinear {
		t.Fatal("toLab is derived")
	}
	if _, err := e.Call("toLab", red.ID, []any{map[string]any{"space": "encoded"}}); err == nil {
		t.Fatal("toLab has a fixed space")
	}

	// chroma of grey is 0; of a colour, the channel spread in encoded values
	grey := testSource(t, e, "srgb", window, 0.3, 0.3, 0.3)
	near(t, "chroma of grey", evalValue(t, e, call(t, e, "chroma", grey.ID), 1).(*graph.ImageValue).Buf.Pix[0], 0, 1e-7)
	col := testSource(t, e, "srgb", window, 0.2, 0.4, 0.6)
	want := buffer.LinearToSRGB(0.6) - buffer.LinearToSRGB(0.2)
	cn := call(t, e, "chroma", col.ID)
	near(t, "chroma", evalValue(t, e, cn, 1).(*graph.ImageValue).Buf.Pix[0], want, 1e-6)
	rel := call(t, e, "chroma", col.ID, map[string]any{"relative": true})
	mean := (buffer.LinearToSRGB(0.2) + buffer.LinearToSRGB(0.4) + buffer.LinearToSRGB(0.6)) / 3
	near(t, "relative chroma", evalValue(t, e, rel, 1).(*graph.ImageValue).Buf.Pix[0], want/(mean+1e-3), 1e-5)
	if id, _ := e.Display(cn.ID); id != cn.ID {
		t.Fatal("chroma is shown as it is")
	}

	// colormap: ends of the range map to the ends of the table
	ramp := testSource(t, e, "linear", window, 0.25)
	for _, tc := range []struct {
		name  string
		rng   []any
		want  [3]float64
		label string
	}{
		{"gray", []any{0.0, 0.5}, [3]float64{0.5, 0.5, 0.5}, "middle of gray"},
		{"viridis", []any{0.25, 1.0}, [3]float64{0.267, 0.005, 0.329}, "viridis low end"},
		{"viridis", []any{-1.0, 0.25}, [3]float64{0.993, 0.906, 0.144}, "viridis high end"},
		{"magma", []any{-1.0, 0.25}, [3]float64{0.987, 0.991, 0.750}, "magma high end"},
		{"diverging", []any{-1.0, 1.5}, [3]float64{0.969, 0.969, 0.969}, "diverging centre"},
	} {
		cm := call(t, e, "colormap", ramp.ID, tc.name, map[string]any{"range": tc.rng})
		v := evalValue(t, e, cm, 1).(*graph.ImageValue)
		pn := v.Buf.N()
		for c := 0; c < 3; c++ {
			near(t, tc.label, v.Buf.Pix[c*pn], tc.want[c], 0.03)
		}
		if cm.Enc != buffer.EncSRGB || cm.Space != buffer.SpaceEncoded || v.Buf.C != 3 {
			t.Fatalf("colormap statics %s", graph.Statics(cm))
		}
	}
	cm := call(t, e, "colormap", col.ID, "viridis")
	if _, err := e.Evaluate(t.Context(), cm.ID, graph.EvalOptions{Scale: 1}); err == nil {
		t.Fatal("colormap of three channels")
	} else if ce, _ := graph.AsError(err); ce.Code != graph.EWrongKind || !strings.Contains(ce.Fix, `toGray("average")`) {
		t.Fatalf("colormap of three channels: %+v", ce)
	}
	// without range: the image's p01..p99 (a constant image maps to the low end)
	if v := evalValue(t, e, call(t, e, "colormap", ramp.ID, "gray"), 1).(*graph.ImageValue); v.Buf.Pix[0] != 0 {
		t.Fatalf("auto range: %g", v.Buf.Pix[0])
	}

	// encode keeps linear values and changes the encoding
	lin := testSource(t, e, "linear", window, 0.2)
	en := call(t, e, "encode", lin.ID, "srgb")
	ev := evalValue(t, e, en, 1).(*graph.ImageValue)
	if ev.Buf.Pix[0] != 0.2 || ev.Enc != buffer.EncSRGB || en.Space != buffer.SpaceLinear {
		t.Fatalf("encode: %g %v", ev.Buf.Pix[0], ev.Enc)
	}
	if lv := evalValue(t, e, lin, 1).(*graph.ImageValue); lv.Enc != buffer.EncLinear {
		t.Fatal("encode rewrote its input's value")
	}
	shown, _ := e.Display(en.ID)
	sv := evalValue(t, e, mustNode(t, e, shown), 1).(*graph.ImageValue)
	near(t, "encoded look", sv.Buf.Pix[0], buffer.LinearToSRGB(0.2), 1e-6)
	if ce := callErr(t, e, "toGray", lin.ID, "rec601"); ce.Code != graph.EArgType {
		t.Fatalf("rec601: %+v", ce)
	}
}

func mustNode(t *testing.T, e *graph.Engine, id graph.NodeID) *graph.Node {
	t.Helper()
	n, ok := e.Node(id)
	if !ok {
		t.Fatalf("no node #%d", id)
	}
	return n
}

func TestToLabWarnsOnCameraRGB(t *testing.T) {
	src, _ := testimage.PlantedScan()
	p := filepath.Join(t.TempDir(), "scan.dng")
	if err := testimage.WriteDNG(p, src, testimage.DNGOptions{}); err != nil {
		t.Fatal(err)
	}
	e := testEngine(t)
	img, err := e.Open(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.Call("toLab", call(t, e, "exposure", img.ID, 0.5).ID, nil)
	if err != nil || len(r.Warnings) != 1 || r.Warnings[0].Code != WPrimaries {
		t.Fatalf("toLab on a DNG: %v %v", err, r.Warnings)
	}
}

// TestNodeAsOptionIsArgType: options are literal values (GCV-004 D1); a
// node where one is expected is E_ARG_TYPE with the (await …) fix, so the
// encoding of a source or an encode node can never silently fall back to
// srgb (review of PR #2).
func TestNodeAsOptionIsArgType(t *testing.T) {
	e, img := newEngine(t)
	st := call(t, e, "stats", img.ID)
	if ce := callErr(t, e, "encode", img.ID, graph.NodeArg{ID: st.ID}); ce.Code != graph.EArgType || ce.Fix != "(await img.stats()).p99" {
		t.Fatalf("encode(node): want E_ARG_TYPE, got %+v", ce)
	}
	_, err := e.Open(testTIFF, map[string]any{"encoding": graph.NodeArg{ID: st.ID}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgType {
		t.Fatalf("cv.open({ encoding: node }): want E_ARG_TYPE, got %v", err)
	}
	if ce := callErr(t, e, "levels", img.ID, map[string]any{"white": graph.NodeArg{ID: st.ID}}); ce.Code != graph.EArgType || ce.Fix != "{ white: (await img.stats()).p99 }" {
		t.Fatalf("levels({ white: node }): %+v", ce)
	}
}
