package graph_test

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph/graphtest"
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
		Space:  graph.SpaceRule{Default: buffer.SpaceEncoded},
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
	return graph.NewEngine(reg, graph.EngineOptions{})
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

func TestSpaceDefaults(t *testing.T) {
	reg := ops.NewRegistry()
	for name, want := range map[string]buffer.Space{
		"exposure": buffer.SpaceLinear, "gaussian": buffer.SpaceLinear, "blend": buffer.SpaceLinear,
		"within": buffer.SpaceLinear, "heal": buffer.SpaceLinear, "inpaint": buffer.SpaceLinear,
		"toGray": buffer.SpaceLinear, "stats": buffer.SpaceLinear, "levels": buffer.SpaceLinear,
		"contrast": buffer.SpaceEncoded, "histogram": buffer.SpaceEncoded, "mask.luminance": buffer.SpaceEncoded,
		"mask.color": buffer.SpaceEncoded, "mask.flood": buffer.SpaceEncoded, "detect.specks": buffer.SpaceEncoded,
		"check.dust": buffer.SpaceEncoded, "overlay": buffer.SpaceEncoded, "compare": buffer.SpaceEncoded,
		"mask.box": buffer.SpaceNone, "mask.gradient": buffer.SpaceNone,
	} {
		op, ok := reg.Lookup(graph.KindImage, name)
		if !ok {
			t.Fatalf("no op %s", name)
		}
		if op.Space.Default != want {
			t.Errorf("%s defaults to %q, want %q", name, op.Space.Default, want)
		}
		hasOption := false
		for _, a := range op.Args {
			hasOption = hasOption || a.Name == graph.SpaceArg
		}
		if hasOption != (want != buffer.SpaceNone) {
			t.Errorf("%s: space option %v", name, hasOption)
		}
	}
	if op, ok := reg.Lookup(graph.KindMask, "refine"); !ok || op.Space.Default != buffer.SpaceEncoded {
		t.Error("refine defaults to encoded")
	}
	if len(reg.Find(graph.ConvertOp)) != 0 {
		t.Error("convert is internal: not listed")
	}
	e := spaceEngine(t)
	img := src(t, e, "srgb", 0.2, 0.3, 0.4)
	if _, err := e.Call(graph.ConvertOp, img.ID, []any{"encoded"}); err == nil {
		t.Error("convert must not be callable")
	}
}

func TestSpaceOptionAndStatics(t *testing.T) {
	e := spaceEngine(t)
	img := src(t, e, "srgb", 0.2, 0.3, 0.4)
	if img.Enc != buffer.EncSRGB || img.Space != buffer.SpaceLinear {
		t.Fatalf("source statics %s", graph.Statics(img))
	}
	a, _ := mustCall(t, e, "exposure", img, 0.3)
	b, _ := mustCall(t, e, "exposure", img, 0.3, map[string]any{"space": "linear"})
	c, _ := mustCall(t, e, "exposure", img, 0.3, map[string]any{"space": "encoded"})
	if a.ID != b.ID || a.ID == c.ID || a.Hash == c.Hash {
		t.Fatal("the resolved space must be part of the hash, and the default must equal the omitted option")
	}
	if a.Space != buffer.SpaceLinear || c.Space != buffer.SpaceEncoded || c.Enc != buffer.EncSRGB {
		t.Fatalf("statics %s / %s", graph.Statics(a), graph.Statics(c))
	}
	if got := graph.Describe(e.Builder(), a, 1, nil); got != "exposure(ev: 0.3)" {
		t.Errorf("default space shown: %q", got)
	}
	if got := graph.Describe(e.Builder(), c, 1, nil); got != `exposure(ev: 0.3, space: "encoded")` {
		t.Errorf("explicit space hidden: %q", got)
	}
	// a param cannot move the space: it changes the graph's shape
	p, _, err := e.DeclareParam(graph.ParamDef{Name: "sp", Type: "choice", Options: []string{"linear", "encoded"}, Value: "linear"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Call("exposure", img.ID, []any{0.3, map[string]any{"space": graph.NodeArg{ID: p.ID}}}); err == nil {
		t.Fatal("a node for space was accepted")
	}
	// Value statics: stats measured img (srgb) in linear space; fields inherit
	s, _ := mustCall(t, e, "stats", img)
	p99, _ := mustCall(t, e, "field", s, "p99")
	if s.Enc != buffer.EncSRGB || s.Space != buffer.SpaceLinear || p99.Enc != buffer.EncSRGB || p99.Space != buffer.SpaceLinear {
		t.Fatalf("value statics: stats %v/%v, p99 %v/%v", s.Enc, s.Space, p99.Enc, p99.Space)
	}
	// masks have no encoding and no space
	m, _ := mustCall(t, e, "mask.luminance", img, map[string]any{"above": 0.5})
	if m.Enc != buffer.EncNone || m.Space != buffer.SpaceNone {
		t.Fatalf("mask statics %v/%v", m.Enc, m.Space)
	}
}

func TestConvertInsertedOnceAndShared(t *testing.T) {
	e := spaceEngine(t)
	img := src(t, e, "srgb", 0.2, 0.3, 0.4)
	c1, _ := mustCall(t, e, "contrast", img, 0.2)
	c2, _ := mustCall(t, e, "mask.color", img, map[string]any{"hue": 30.0})
	conv := node(t, e, c1.Inputs[0])
	if conv.Op.Name != graph.ConvertOp || conv.Args.String("to") != "encoded" || conv.Inputs[0] != img.ID {
		t.Fatalf("contrast input is %s", graph.Label(conv))
	}
	if c2.Inputs[0] != conv.ID {
		t.Fatal("the two consumers do not share the conversion")
	}
	if conv.Enc != buffer.EncSRGB || conv.Space != buffer.SpaceEncoded {
		t.Fatalf("convert statics %s", graph.Statics(conv))
	}
	v := image(t, e, c1)
	image(t, e, c2)
	if n := e.Evaluator().Runs(conv.ID); n != 1 {
		t.Fatalf("convert ran %d times", n)
	}
	// contrast 0.2 around encoded mid-grey of sRGB_encode(0.2)
	want := 0.5 + (buffer.LinearToSRGB(0.2)-0.5)*1.2
	if got := float64(v.Buf.Plane(0)[0]); math.Abs(got-want) > 1e-6 || v.Enc != buffer.EncSRGB {
		t.Fatalf("contrast value %g, want %g", got, want)
	}
	// the look of a linear-space node converts; the look of an encoded node does not
	if id, _ := e.Display(img.ID); id != conv.ID {
		t.Fatal("a look of img must render convert(img → encoded)")
	}
	if id, _ := e.Display(c1.ID); id != c1.ID {
		t.Fatal("a look of an encoded-space node needs no conversion")
	}
	if id, _ := e.Display(c2.ID); id != c2.ID {
		t.Fatal("masks are shown as they are")
	}
}

func TestNoConversionForLinearData(t *testing.T) {
	e := spaceEngine(t)
	img := src(t, e, "linear", 0.3)
	c, _ := mustCall(t, e, "contrast", img, 0.2)
	if c.Inputs[0] != img.ID || c.Enc != buffer.EncLinear {
		t.Fatalf("contrast on linear data reads %s", graph.Label(node(t, e, c.Inputs[0])))
	}
	if id, _ := e.Display(img.ID); id != img.ID {
		t.Fatal("linear data is shown as stored")
	}
	if got := image(t, e, c).Buf.Pix[0]; math.Abs(float64(got)-(0.5+(0.3-0.5)*1.2)) > 1e-6 {
		t.Fatalf("contrast on linear data: %g", got)
	}
}

func TestGreyInputIsLuminanceOfLight(t *testing.T) {
	e := spaceEngine(t)
	red := src(t, e, "srgb", 1, 0, 0) // pure red: linear (1, 0, 0)
	m, _ := mustCall(t, e, "mask.luminance", red, map[string]any{"above": 0.5})
	conv := node(t, e, m.Inputs[0])
	gray := node(t, e, conv.Inputs[0])
	if conv.Op.Name != graph.ConvertOp || gray.Op.Name != "toGray" || gray.Args.String("weights") != "rec709" || gray.Space != buffer.SpaceLinear {
		t.Fatalf("mask.luminance reads %s ← %s", graph.Label(conv), graph.Label(gray))
	}
	// sRGB_encode(0.2126) = 0.498 (today's number), not the luma 0.213
	if got := float64(image(t, e, conv).Buf.Pix[0]); math.Abs(got-buffer.LinearToSRGB(0.2126)) > 1e-6 || math.Abs(got-0.498) > 1e-3 {
		t.Fatalf("encoded luminance of red %g", got)
	}
	for above, want := range map[float64]float32{0.49: 1, 0.51: 0} {
		mm, _ := mustCall(t, e, "mask.luminance", red, map[string]any{"above": above, "softness": 0.0})
		if got := image(t, e, mm).Buf.Pix[0]; got != want {
			t.Errorf("mask.luminance({ above: %g }) on red = %g", above, got)
		}
	}
	// refine's guide image (an Inputs entry) gets the same rule
	box, _ := mustCall(t, e, "mask.box", red, []any{4.0, 4.0, 10.0, 10.0})
	r, _ := mustCall(t, e, "refine", box, graph.NodeArg{ID: red.ID})
	if r.Inputs[1] != conv.ID {
		t.Fatalf("refine's guide is %s", graph.Label(node(t, e, r.Inputs[1])))
	}
	// histogram: the rule applies to channel "L" only
	hl, _ := mustCall(t, e, "histogram", red)
	hr, _ := mustCall(t, e, "histogram", red, map[string]any{"channel": "r"})
	if hl.Inputs[0] != conv.ID || node(t, e, hr.Inputs[0]).Inputs[0] != red.ID {
		t.Fatal("histogram grey input")
	}
	// detect.specks reports meanL in its (encoded) space
	sp, _ := mustCall(t, e, "detect.specks", red)
	if sp.Inputs[0] != conv.ID {
		t.Fatal("detect.specks grey input")
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
	conv := node(t, e, id.Inputs[0])
	cv := image(t, e, conv)
	iv := image(t, e, id)
	if iv == cv || iv.Buf != cv.Buf || iv.Enc != buffer.EncLinear || cv.Enc != buffer.EncSRGB {
		t.Fatalf("identity: copy %v, shared buffer %v, encodings %v / %v", iv != cv, iv.Buf == cv.Buf, iv.Enc, cv.Enc)
	}
	if again := image(t, e, conv); again.Enc != buffer.EncSRGB {
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
		inner, _ := mustCall(t, e, "contrast", hole, 0.2)
		w, _ := mustCall(t, e, "within", img, graph.NodeArg{ID: m.ID}, graph.BuiltArg{Root: inner.ID, Hole: hole.ID})
		direct, _ := mustCall(t, e, "contrast", img, 0.2)
		edited := node(t, e, w.Inputs[2])
		switch enc {
		case "srgb": // within (linear) converts the encoded contrast result back
			if edited.Op.Name != graph.ConvertOp || edited.Inputs[0] != direct.ID {
				t.Fatalf("srgb: within's edit is %s", graph.Label(edited))
			}
		case "linear":
			if edited.ID != direct.ID {
				t.Fatalf("linear: within's edit is %s, want the direct contrast", graph.Label(edited))
			}
		}
	}
}

func TestCompareConvertsEachInput(t *testing.T) {
	e := spaceEngine(t)
	img := src(t, e, "srgb", 0.2, 0.3, 0.4)
	c, _ := mustCall(t, e, "contrast", img, 0.2) // encoded
	cmp, _ := mustCall(t, e, "compare", img, []any{graph.NodeArg{ID: c.ID}})
	if a := node(t, e, cmp.Inputs[0]); a.Op.Name != graph.ConvertOp || a.Inputs[0] != img.ID {
		t.Fatalf("compare receiver %s", graph.Label(a))
	}
	if cmp.Inputs[1] != c.ID {
		t.Fatalf("compare others[0] %s", graph.Label(node(t, e, cmp.Inputs[1])))
	}
	lin, _ := mustCall(t, e, "compare", img, []any{graph.NodeArg{ID: c.ID}}, map[string]any{"space": "linear"})
	if lin.Inputs[0] != img.ID {
		t.Fatal("linear compare converted img")
	}
	if b := node(t, e, lin.Inputs[1]); b.Op.Name != graph.ConvertOp || b.Inputs[0] != c.ID || b.Args.String("to") != "linear" {
		t.Fatalf("linear compare others[0] %s", graph.Label(b))
	}
}

func TestSpaceAndEncodingWarnings(t *testing.T) {
	for _, tc := range []struct {
		enc  string
		warn bool
	}{{"srgb", true}, {"linear", false}} {
		e := spaceEngine(t)
		img := src(t, e, tc.enc, 0.2, 0.3, 0.4)
		s, _ := mustCall(t, e, "stats", img) // linear
		p99, _ := mustCall(t, e, "field", s, "p99")
		_, ws := mustCall(t, e, "mask.luminance", img, map[string]any{"above": graph.NodeArg{ID: p99.ID}}) // encoded
		if hasWarning(ws, graph.WSpace) != tc.warn {
			t.Fatalf("%s: SPACE warning %v, want %v: %v", tc.enc, hasWarning(ws, graph.WSpace), tc.warn, ws)
		}
		if tc.warn && !strings.Contains(ws[len(ws)-1].Message, `mask.luminance.above comes from stats() in linear space, but mask.luminance runs in encoded space here. fix: img.stats({ space: "encoded" })`) {
			t.Fatalf("SPACE message: %v", ws)
		}
		// the idiom levels({ black: s.p01, white: s.p99 }) stays quiet: both linear
		p01, _ := mustCall(t, e, "field", s, "p01")
		if _, ws := mustCall(t, e, "levels", img, map[string]any{"black": graph.NodeArg{ID: p01.ID}, "white": graph.NodeArg{ID: p99.ID}}); len(ws) != 0 {
			t.Fatalf("%s: levels idiom warns: %v", tc.enc, ws)
		}
	}
	e := spaceEngine(t)
	a := src(t, e, "srgb", 0.2, 0.3, 0.4)
	b := src(t, e, "linear", 0.2, 0.3, 0.4)
	if _, ws := mustCall(t, e, "blend", a, graph.NodeArg{ID: b.ID}); hasWarning(ws, graph.WEncoding) {
		t.Fatal("ENCODING in linear space")
	}
	_, ws := mustCall(t, e, "blend", a, graph.NodeArg{ID: b.ID}, map[string]any{"space": "encoded"})
	if !hasWarning(ws, graph.WEncoding) {
		t.Fatalf("no ENCODING warning: %v", ws)
	}
}

func TestExplainShowsStatics(t *testing.T) {
	e := spaceEngine(t)
	img := src(t, e, "srgb", 0.2, 0.3, 0.4)
	m, _ := mustCall(t, e, "mask.luminance", img, map[string]any{"above": 0.9})
	lines := e.Explain(m.ID, 1)
	want := []string{"mask.luminance(above: 0.9)", "convert(→ encoded)", "toGray(", "testsource"}
	if len(lines) != 4 {
		t.Fatalf("explain: %q", lines)
	}
	for i, w := range want {
		if !strings.Contains(lines[i], w) {
			t.Fatalf("explain line %d %q lacks %q", i, lines[i], w)
		}
	}
	if !strings.HasSuffix(lines[1], "enc srgb · space encoded") || !strings.HasSuffix(lines[2], "enc srgb · space linear") || strings.Contains(lines[0], "enc ") {
		t.Fatalf("statics column: %q", lines)
	}
}
