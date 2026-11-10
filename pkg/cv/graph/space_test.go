package graph_test

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph/graphtest"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
	"github.com/go-go-golems/goja-cv/pkg/cv/ops"
)

var tile = graph.Frame{W: 32, H: 24}

func spaceEngine(t *testing.T) *graph.Engine {
	t.Helper()
	reg := ops.NewRegistry()
	graphtest.RegisterSource(reg)
	// a derived op with an identity, to test reinterpretation and stamping
	reg.MustRegister(graph.OpDef{
		Name: "testderived", On: graph.KindImage, Returns: graph.KindImage,
		Result: graph.DerivedEnc,
		Args:   []graph.ArgSpec{{Name: "k", Type: "number", Positional: true, Default: 1.0}},
		Identity: func(a graph.Args) bool {
			return a.Number("k") == 1
		},
		Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			src := in[0].(*graph.ImageValue)
			out := src.Buf.Clone()
			for i := range out.Pix {
				out.Pix[i] *= float32(a.Number("k"))
			}
			return graph.NewImage(out), nil
		},
	})
	return graph.NewEngine(reg, graphtest.Options(t))
}

func src(t *testing.T, e *graph.Engine, enc string, fill ...float64) *graph.Node {
	t.Helper()
	n, err := graphtest.Source(e, enc, tile, fill...)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func mustCall(t *testing.T, e *graph.Engine, name string, recv *graph.Node, raw ...any) (*graph.Node, []graph.Warning) {
	t.Helper()
	cr, err := e.Call(name, recv.ID, raw)
	n, ws := cr.Node(), cr.Warnings
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return n, ws
}

func node(t *testing.T, e *graph.Engine, id graph.NodeID) *graph.Node {
	t.Helper()
	n, ok := e.Node(id)
	if !ok {
		t.Fatalf("no node #%d", id)
	}
	return n
}

func image(t *testing.T, e *graph.Engine, n *graph.Node) *graph.ImageValue {
	t.Helper()
	v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatalf("evaluate %s: %v", graph.Label(n), err)
	}
	return v.(*graph.ImageValue)
}

func hasWarning(ws []graph.Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}

// gradientTIFF is an sRGB-encoded 16-bit test file (cv.open).
var gradientTIFF, _ = filepath.Abs("../../../testdata/gradient-64x48-16bit.tif")

func openGradient(t *testing.T, e *graph.Engine) *graph.Node {
	t.Helper()
	img, err := e.Open(gradientTIFF, nil)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// TestOpenIsEncodedAndAsConverts (GCV-004 §4.2): cv.open returns file units;
// as() converts; as is the identity on linear-encoded and derived images; a
// conversion and its inverse cancel; the linear view of a file is its
// pyramid level (openLinear, no copy).
func TestOpenIsEncodedAndAsConverts(t *testing.T) {
	e := graph.NewEngine(ops.NewRegistry(), graphtest.Options(t))
	img := openGradient(t, e)
	if img.Enc != buffer.EncSRGB || img.Space != buffer.SpaceEncoded {
		t.Fatalf("open statics %s", graph.Statics(img))
	}
	lin, _ := mustCall(t, e, "as", img, "linear")
	if lin.Op.Name != graph.OpenLinearOp || lin.Space != buffer.SpaceLinear || lin.Frame != img.Frame || len(lin.Inputs) != 0 {
		t.Fatalf("as(open, linear) is %s %s", graph.Label(lin), graph.Statics(lin))
	}
	if back, _ := mustCall(t, e, "as", lin, "encoded"); back.ID != img.ID {
		t.Fatalf("as(openLinear, encoded) is %s, want the opened file", graph.Label(back))
	}
	g, _ := mustCall(t, e, "gaussian", lin, 2.0)
	ge, _ := mustCall(t, e, "as", g, "encoded")
	if gl, _ := mustCall(t, e, "as", ge, "linear"); gl.ID != g.ID {
		t.Fatalf("as(as(x, encoded), linear) is %s, want x", graph.Label(gl))
	}
	if same, _ := mustCall(t, e, "as", img, "encoded"); same.ID != img.ID {
		t.Fatal("as to the image's own space is the identity")
	}
	// linear-encoded and derived images: both spaces coincide
	te := spaceEngine(t)
	ld := src(t, te, "linear", 0.3)
	if n, _ := mustCall(t, te, "as", ld, "linear"); n.ID != ld.ID {
		t.Fatal("as on linear-encoded data is the identity")
	}
	d, _ := mustCall(t, te, "testderived", src(t, te, "srgb", 0.2), 2.0)
	if n, _ := mustCall(t, te, "as", d, "encoded"); n.ID != d.ID {
		t.Fatal("as on a derived image is the identity")
	}
	// values: encoded file units, linear light through as
	ev, lv := image(t, e, img), image(t, e, lin)
	for i := 0; i < ev.Buf.N(); i += 97 {
		if math.Abs(buffer.SRGBToLinear(float64(ev.Buf.Pix[i]))-float64(lv.Buf.Pix[i])) > 1e-5 {
			t.Fatalf("pixel %d: encoded %g, linear %g", i, ev.Buf.Pix[i], lv.Buf.Pix[i])
		}
	}
	if _, err := e.Call("gaussian", img.ID, []any{2.0, map[string]any{"space": "linear"}}); err == nil {
		t.Fatal("the space option is gone")
	} else if ce, _ := graph.AsError(err); ce.Code != graph.EUnknownOption {
		t.Fatalf("{ space }: %+v", ce)
	}
}

// TestPhysicalBracket (GCV-004 §7.4, D3): exposure works in linear light and
// returns file units: as(exposure(openLinear), encoded), the same values as
// linear exposure followed by encoding, with no extra conversion node.
func TestPhysicalBracket(t *testing.T) {
	e := graph.NewEngine(ops.NewRegistry(), graphtest.Options(t))
	img := openGradient(t, e)
	ex, _ := mustCall(t, e, "exposure", img, 1.0)
	if ex.Op.Name != graph.AsOp || ex.Space != buffer.SpaceEncoded || ex.Enc != buffer.EncSRGB {
		t.Fatalf("exposure on file units is %s %s", graph.Label(ex), graph.Statics(ex))
	}
	inner := node(t, e, ex.Inputs[0])
	src := node(t, e, inner.Inputs[0])
	if inner.Op.Name != "exposure" || inner.Space != buffer.SpaceLinear || src.Op.Name != graph.OpenLinearOp {
		t.Fatalf("bracket: %s ← %s", graph.Label(inner), graph.Label(src))
	}
	v := image(t, e, ex)
	lin := image(t, e, src)
	for i := 0; i < v.Buf.N(); i += 31 {
		want := buffer.LinearToSRGB(2 * float64(lin.Buf.Pix[i]))
		if math.Abs(float64(v.Buf.Pix[i])-want) > 1e-6 {
			t.Fatalf("pixel %d: %g, want %g", i, v.Buf.Pix[i], want)
		}
	}
	// two buffers beyond the pyramid: the exposure and the encoded result
	if runs := e.Evaluator().TotalRuns(); runs != 3 { // openLinear, exposure, as
		t.Fatalf("%d op runs, want 3 (openLinear, exposure, as)", runs)
	}
	if n := e.Evaluator().Runs(img.ID); n != 0 {
		t.Fatal("the encoded source was computed for a physical op")
	}
	// the cache holds the pyramid level (openLinear's value), the exposure
	// and the result: no conversion copy of the source
	if cs, one := e.Cache().Stats(), int64(4*len(v.Buf.Pix)); cs.Entries != 3 || cs.Bytes < 3*one || cs.Bytes >= 4*one {
		t.Fatalf("cache %d entries, %d bytes; want 3 buffers of %d", cs.Entries, cs.Bytes, one)
	}
}

// TestReceiverDecides (GCV-004 §4.3, D16): other image inputs are converted
// to the receiver's space, not the receiver.
func TestReceiverDecides(t *testing.T) {
	e := graph.NewEngine(ops.NewRegistry(), graphtest.Options(t))
	img := openGradient(t, e)
	lin, _ := mustCall(t, e, "as", img, "linear")
	d, _ := mustCall(t, e, "sub", lin, graph.NodeArg{ID: img.ID})
	if d.Inputs[0] != lin.ID || d.Inputs[1] != lin.ID {
		t.Fatalf("sub's inputs: %s, %s", graph.Label(node(t, e, d.Inputs[0])), graph.Label(node(t, e, d.Inputs[1])))
	}
	d2, _ := mustCall(t, e, "sub", img, graph.NodeArg{ID: lin.ID})
	if d2.Inputs[0] != img.ID || d2.Inputs[1] != img.ID { // as(openLinear, encoded) folds back to open
		t.Fatalf("img.sub(lin)'s inputs: %s, %s", graph.Label(node(t, e, d2.Inputs[0])), graph.Label(node(t, e, d2.Inputs[1])))
	}
}

// TestOutputSpaceFollowsTheReceiver is the output-space property (§7.4): on
// an srgb image in each space, every following op's Image result is in the
// receiver's space; physical ops return to it; display and derived results
// are in their own.
func TestOutputSpaceFollowsTheReceiver(t *testing.T) {
	e := graph.NewEngine(ops.NewRegistry(), graphtest.Options(t))
	img := openGradient(t, e)
	lin, _ := mustCall(t, e, "as", img, "linear")
	calls := map[string][]any{
		"exposure": {0.5}, "levels": {map[string]any{"black": 0.1, "white": 0.9}}, "contrast": {0.2}, "gaussian": {1.5},
		"toGray": {}, "add": {0.1}, "mul": {2.0}, "max": {0.2}, "min": {0.8}, "abs": {}, "crop": {[]any{4.0, 4.0, 20.0, 10.0}},
		"heal": {graph.NodeArg{}}, "blend": {graph.NodeArg{}},
	}
	for _, recv := range []*graph.Node{img, lin} {
		m, _ := mustCall(t, e, "mask.box", recv, []any{4.0, 4.0, 8.0, 8.0})
		for name, raw := range calls {
			args := append([]any(nil), raw...)
			switch name {
			case "heal":
				args = []any{graph.NodeArg{ID: m.ID}}
			case "blend":
				args = []any{graph.NodeArg{ID: img.ID}}
			}
			n, _ := mustCall(t, e, name, recv, args...)
			if n.Space != recv.Space {
				t.Errorf("%s on %s: result in %s", name, recv.Space, n.Space)
			}
			image(t, e, n)
		}
		for _, name := range []string{"stats", "histogram"} {
			n, _ := mustCall(t, e, name, recv)
			if n.Space != recv.Space {
				t.Errorf("%s on %s: numbers in %s", name, recv.Space, n.Space)
			}
		}
		sub, _ := mustCall(t, e, "sub", recv, 0.1)
		if sub.Enc != buffer.EncLinear {
			t.Errorf("sub is derived: %s", graph.Statics(sub))
		}
		cm, _ := mustCall(t, e, "colormap", node(t, e, mustToGray(t, e, recv).ID), "viridis")
		if cm.Enc != buffer.EncSRGB || cm.Space != buffer.SpaceEncoded {
			t.Errorf("colormap is a picture: %s", graph.Statics(cm))
		}
	}
}

func mustToGray(t *testing.T, e *graph.Engine, n *graph.Node) *graph.Node {
	t.Helper()
	g, _ := mustCall(t, e, "toGray", n, "average")
	return g
}

// TestOneLuminance (GCV-004 D20): stats' L fields, mask.luminance and
// histogram read toGray() of the receiver: stats().p99 is the 99th
// percentile of toGray(), and mask.luminance({ above: p99 }) selects about
// 1 % of the pixels.
func TestOneLuminance(t *testing.T) {
	e := graph.NewEngine(ops.NewRegistry(), graphtest.Options(t))
	img := openGradient(t, e)
	s, _ := mustCall(t, e, "stats", img)
	st := evalMap(t, e, s)
	g := image(t, e, func() *graph.Node { n, _ := mustCall(t, e, "toGray", img); return n }())
	lum := append([]float32(nil), g.Buf.Plane(0)...)
	if p99 := kernels.Percentile(lum, 0.99); math.Abs(p99-st["p99"].(float64)) > 1e-9 {
		t.Fatalf("stats p99 %g, toGray p99 %g", st["p99"], p99)
	}
	m, _ := mustCall(t, e, "mask.luminance", img, map[string]any{"above": st["p99"], "softness": 0.0})
	cov := evalNum(t, e, func() *graph.Node { n, _ := mustCall(t, e, "coverage", m); return n }())
	if cov < 0.005 || cov > 0.02 {
		t.Fatalf("mask.luminance({ above: p99 }) covers %g", cov)
	}
	// the luminance of pure red in file units: sRGB_encode(0.2126) = 0.498
	te := spaceEngine(t)
	red := src(t, te, "srgb", 1, 0, 0)
	for above, want := range map[float64]float32{0.49: 1, 0.51: 0} {
		mm, _ := mustCall(t, te, "mask.luminance", red, map[string]any{"above": above, "softness": 0.0})
		if got := image(t, te, mm).Buf.Pix[0]; got != want {
			t.Errorf("mask.luminance({ above: %g }) on red = %g", above, got)
		}
	}
}

func evalMap(t *testing.T, e *graph.Engine, n *graph.Node) map[string]any {
	t.Helper()
	v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	return v.(*graph.DataValue).Data.(map[string]any)
}

func evalNum(t *testing.T, e *graph.Engine, n *graph.Node) float64 {
	t.Helper()
	v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	f, _ := v.(*graph.DataValue).Number()
	return f
}

// TestColormapRangeOnItsReceiver: colormap follows its receiver, so a range
// measured on it maps p01 and p99 to the ends.
func TestColormapRangeOnItsReceiver(t *testing.T) {
	e := graph.NewEngine(ops.NewRegistry(), graphtest.Options(t))
	g, _ := mustCall(t, e, "toGray", openGradient(t, e))
	st := evalMap(t, e, func() *graph.Node { n, _ := mustCall(t, e, "stats", g); return n }())
	cm, _ := mustCall(t, e, "colormap", g, "gray", map[string]any{"range": []any{st["p01"], st["p99"]}})
	if cm.Inputs[0] != g.ID {
		t.Fatalf("colormap converted its receiver: %s", graph.Label(node(t, e, cm.Inputs[0])))
	}
	v := image(t, e, cm)
	var lo, hi float32 = 1, 0
	for _, p := range v.Buf.Plane(0) {
		lo, hi = min(lo, p), max(hi, p)
	}
	if lo > 0.01 || hi < 0.99 {
		t.Fatalf("gray colormap over [p01, p99] spans [%g, %g]", lo, hi)
	}
}

func TestDerivedReinterpretsAndIdentityStampsCopies(t *testing.T) {
	e := spaceEngine(t)
	img := src(t, e, "srgb", 0.2)
	d, _ := mustCall(t, e, "testderived", img, 2.0)
	if d.Enc != buffer.EncLinear || d.Space != buffer.SpaceLinear {
		t.Fatalf("derived statics %s", graph.Statics(d))
	}
	if id, _ := e.Display(d.ID); id != d.ID {
		t.Fatal("derived values are shown as they are")
	}
	v := image(t, e, d)
	if got := float64(v.Buf.Pix[0]); math.Abs(got-2*buffer.LinearToSRGB(0.2)) > 1e-6 || v.Enc != buffer.EncLinear {
		t.Fatalf("derived value %g enc %v", got, v.Enc)
	}
	// stats of a derived image are in its own units: no conversion
	s, _ := mustCall(t, e, "stats", d)
	if s.Inputs[0] != d.ID {
		t.Fatal("stats converted a derived image")
	}
	// the identity short-circuit returns the input's value stamped on a copy
	id, _ := mustCall(t, e, "testderived", img) // k = 1
	iv := image(t, e, id)
	sv := image(t, e, img)
	if iv == sv || iv.Buf != sv.Buf || iv.Enc != buffer.EncLinear || sv.Enc != buffer.EncSRGB {
		t.Fatalf("identity: copy %v, shared buffer %v, encodings %v / %v", iv != sv, iv.Buf == sv.Buf, iv.Enc, sv.Enc)
	}
	if again := image(t, e, img); again.Enc != buffer.EncSRGB {
		t.Fatal("the input's cached value was rewritten")
	}
}

func TestLambdasResolveLikeDirectCalls(t *testing.T) {
	for _, enc := range []string{"srgb", "linear"} {
		e := spaceEngine(t)
		img := src(t, e, enc, 0.2, 0.3, 0.4)
		m, _ := mustCall(t, e, "mask.box", img, []any{0.0, 0.0, 10.0, 10.0})
		hole, err := e.Placeholder(img.ID)
		if err != nil {
			t.Fatal(err)
		}
		if hole.Enc != img.Enc || hole.Space != img.Space || hole.Frame != img.Frame {
			t.Fatalf("%s: placeholder statics %s", enc, graph.Statics(hole))
		}
		inner, _ := mustCall(t, e, "exposure", hole, -0.2)
		w, _ := mustCall(t, e, "within", img, graph.NodeArg{ID: m.ID}, graph.BuiltArg{Root: inner.ID, Hole: hole.ID})
		direct, _ := mustCall(t, e, "exposure", img, -0.2)
		if w.Inputs[2] != direct.ID {
			t.Fatalf("%s: within's edit is %s, want the direct exposure", enc, graph.Label(node(t, e, w.Inputs[2])))
		}
	}
}

func TestCompareConvertsEachInput(t *testing.T) {
	e := graph.NewEngine(ops.NewRegistry(), graphtest.Options(t))
	img := openGradient(t, e)
	lin, _ := mustCall(t, e, "as", img, "linear")
	images := []any{graph.NodeArg{ID: img.ID}, graph.NodeArg{ID: lin.ID}}
	cr, err := e.Call("compare", 0, []any{images})
	if err != nil {
		t.Fatal(err)
	}
	cmp := cr.Node()
	if cmp.Inputs[0] != img.ID || cmp.Inputs[1] != img.ID { // display: both in file units
		t.Fatalf("compare inputs %s, %s", graph.Label(node(t, e, cmp.Inputs[0])), graph.Label(node(t, e, cmp.Inputs[1])))
	}
}

func TestEncodingWarnings(t *testing.T) {
	e := spaceEngine(t)
	a := src(t, e, "srgb", 0.2, 0.3, 0.4)
	b := src(t, e, "linear", 0.2, 0.3, 0.4)
	al, _ := mustCall(t, e, "as", a, "linear")
	if _, ws := mustCall(t, e, "blend", al, graph.NodeArg{ID: b.ID}); hasWarning(ws, graph.WEncoding) {
		t.Fatal("ENCODING in linear space")
	}
	_, ws := mustCall(t, e, "blend", a, graph.NodeArg{ID: b.ID})
	if !hasWarning(ws, graph.WEncoding) || !strings.Contains(ws[len(ws)-1].Message, `fix: img.as("linear").blend(…)`) {
		t.Fatalf("no ENCODING warning with the as fix: %v", ws)
	}
}

func TestExplainShowsStatics(t *testing.T) {
	e := spaceEngine(t)
	img := src(t, e, "srgb", 0.2, 0.3, 0.4)
	m, _ := mustCall(t, e, "mask.luminance", img, map[string]any{"above": 0.9})
	lines := e.Explain(m.ID)
	want := []string{"mask.luminance(above: 0.9)", "testsource", "as(to: \"encoded\")", "toGray(", "as(to: \"linear\")", "(see above)"}
	if len(lines) != len(want) {
		t.Fatalf("explain: %q", lines)
	}
	for i, w := range want {
		if !strings.Contains(lines[i], w) {
			t.Fatalf("explain line %d %q lacks %q", i, lines[i], w)
		}
	}
	// a mask is a derived (linear-encoded) one-channel image
	if !strings.HasSuffix(lines[2], "enc srgb · space encoded") || !strings.HasSuffix(lines[3], "enc srgb · space linear") || !strings.HasSuffix(lines[0], "enc linear · space linear") {
		t.Fatalf("statics column: %q", lines)
	}
}
