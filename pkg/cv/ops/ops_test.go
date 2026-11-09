package ops

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

var testTIFF = filepath.Join("..", "..", "..", "testdata", "gradient-64x48-16bit.tif")

func newEngine(t *testing.T) (*graph.Engine, *graph.Node) {
	t.Helper()
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	img, err := e.Open(testTIFF, nil)
	if err != nil {
		t.Fatal(err)
	}
	return e, img
}

func call(t *testing.T, e *graph.Engine, name string, recv graph.NodeID, raw ...any) *graph.Node {
	t.Helper()
	cr, err := e.Call(name, recv, raw)
	n := cr.Node()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return n
}

func evalData(t *testing.T, e *graph.Engine, n *graph.Node, scale float64) any {
	t.Helper()
	v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: scale})
	if err != nil {
		t.Fatalf("evaluate %s: %v", n.Op.Name, err)
	}
	return v.(*graph.DataValue).Data
}

// statOf evaluates a Value node at scale 1 and reads one of its fields: the
// literal an agent gets with (await node).name.
func statOf(t *testing.T, e *graph.Engine, n *graph.Node, name string) float64 {
	t.Helper()
	m := evalData(t, e, n, 1).(map[string]any)
	f, ok := m[name].(float64)
	if !ok {
		t.Fatalf("%s.%s is %v", n.Op.Name, name, m[name])
	}
	return f
}

func TestStatsLevelsOnTestImage(t *testing.T) {
	e, img := newEngine(t)
	if img.Frame != (graph.Frame{W: 64, H: 48}) {
		t.Fatalf("source %dx%d", img.Frame.W, img.Frame.H)
	}
	s := call(t, e, "stats", img.ID)
	st := evalData(t, e, s, 1).(map[string]any)
	if st["w"] != 64.0 || st["clipLow"].(float64) <= 0 {
		t.Fatalf("stats: %v", st)
	}
	fixed := call(t, e, "levels", img.ID, map[string]any{"black": statOf(t, e, s, "p01"), "white": statOf(t, e, s, "p99")})
	fs := evalData(t, e, call(t, e, "stats", fixed.ID), 1).(map[string]any)
	// levels maps each channel; luminance is encode(Y), Y a weighted sum of
	// linear light, so its percentiles land near (not exactly on) 0 and 1
	// for a colour image...
	if math.Abs(fs["p01"].(float64)) > 0.04 || math.Abs(fs["p99"].(float64)-1) > 0.02 {
		t.Fatalf("levels from stats: p01 %v p99 %v", fs["p01"], fs["p99"])
	}
	// ...and exactly for a gray one (the map is monotone).
	gray := call(t, e, "toGray", img.ID)
	gs := call(t, e, "stats", gray.ID)
	gfixed := call(t, e, "levels", gray.ID, map[string]any{"black": statOf(t, e, gs, "p01"), "white": statOf(t, e, gs, "p99")})
	gfs := evalData(t, e, call(t, e, "stats", gfixed.ID), 1).(map[string]any)
	if math.Abs(gfs["p01"].(float64)) > 2e-3 || math.Abs(gfs["p99"].(float64)-1) > 2e-3 {
		t.Fatalf("gray levels from stats: p01 %v p99 %v", gfs["p01"], gfs["p99"])
	}
	// +1 EV doubles mean luminance in linear light (no clipping in the kernel)
	lin := call(t, e, graph.AsOp, img.ID, "linear")
	l0 := statOf(t, e, call(t, e, "stats", lin.ID), "meanL")
	l1 := statOf(t, e, call(t, e, "stats", call(t, e, "exposure", lin.ID, 1).ID), "meanL")
	if math.Abs(l1-2*l0) > 1e-6 {
		t.Fatalf("linear meanL %g -> %g", l0, l1)
	}
	m0 := statOf(t, e, s, "meanL")
	q := evalData(t, e, call(t, e, "stats", img.ID, map[string]any{"percentiles": []any{0.25}}), 1).(map[string]any)["percentiles"]
	if v := q.(map[string]any)["0.25"].(float64); v <= 0 || v >= m0*2 {
		t.Fatalf("percentile 0.25 = %v", v)
	}
}

func TestPreviewScaleEvaluatesSmallerBuffers(t *testing.T) {
	e, img := newEngine(t)
	g := call(t, e, "gaussian", img.ID, 4)
	v, err := e.Evaluate(context.Background(), g.ID, graph.EvalOptions{Scale: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	b := v.(*graph.ImageValue).Buf
	if b.W != 32 || b.H != 24 || b.Scale != 0.5 {
		t.Fatalf("buffer %v", b)
	}
	lines := e.Evaluator().Explain(g.ID, 0.5)
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "#2 gaussian(sigma: 4 px → 2 at scale 0.5)  Image 32×24  cached") ||
		!strings.HasPrefix(lines[1], "  #1 source gradient-64x48-16bit.tif") {
		t.Fatalf("explain %q", lines)
	}
}

// walkNodes are the node inputs the registry walk passes: img and edited
// are the image arguments (colour or grey, per pass), mask and mask2
// masks, regions regions of mask.
type walkNodes struct {
	img, stats, mask, mask2, regions, edited *graph.Node
}

func nodeArg(n *graph.Node) graph.NodeArg { return graph.NodeArg{ID: n.ID} }

// walkSamples gives sample arguments for ops that need some; ops not listed
// are called without arguments.
var walkSamples = map[string]func(w walkNodes) []any{
	"channel": func(walkNodes) []any { return []any{"r"} }, "encode": func(walkNodes) []any { return []any{"srgb"} },
	"exposure": func(walkNodes) []any { return []any{0.5} }, "contrast": func(walkNodes) []any { return []any{0.2} },
	"gaussian": func(walkNodes) []any { return []any{1.5} }, "as": func(walkNodes) []any { return []any{"linear"} },
	"add": func(w walkNodes) []any { return []any{nodeArg(w.edited)} }, "sub": func(w walkNodes) []any { return []any{nodeArg(w.edited)} },
	"mul": func(w walkNodes) []any { return []any{nodeArg(w.edited)} }, "div": func(walkNodes) []any { return []any{2} },
	"min": func(w walkNodes) []any { return []any{nodeArg(w.edited)} }, "max": func(w walkNodes) []any { return []any{nodeArg(w.edited)} },
	"distance": func(walkNodes) []any { return []any{0.18} },
	"sample": func(walkNodes) []any {
		return []any{[]any{[]any{10.5, 10.5}, []any{30.0, 20.0}}, map[string]any{"radius": 2}}
	},
	"measure": func(walkNodes) []any { return []any{[]any{[]any{20.5, 20.5}}} },
	"profile": func(walkNodes) []any { return []any{[]any{2.0, 24.0}, []any{60.0, 24.0}} },
	"Regions:measure": func(w walkNodes) []any {
		return []any{nodeArg(w.img), map[string]any{"stats": []any{"mean", "max"}, "ring": true}}
	},
	"mask.luminance": func(walkNodes) []any { return []any{map[string]any{"above": 0.4}} },
	"mask.color":     func(walkNodes) []any { return []any{map[string]any{"hue": 30}} },
	"mask.box":       func(walkNodes) []any { return []any{[]any{4, 4, 20, 10}} },
	"mask.ellipse":   func(walkNodes) []any { return []any{[]any{4, 4, 20, 10}} },
	"mask.gradient":  func(walkNodes) []any { return []any{map[string]any{"from": "left", "to": 0.6}} },
	"mask.flood":     func(walkNodes) []any { return []any{[]any{10, 10}} },
	"and":            func(w walkNodes) []any { return []any{nodeArg(w.mask2)} },
	"or":             func(w walkNodes) []any { return []any{nodeArg(w.mask2)} },
	"minus":          func(w walkNodes) []any { return []any{nodeArg(w.mask2)} },
	"feather":        func(walkNodes) []any { return []any{3} },
	"erode":          func(walkNodes) []any { return []any{2} },
	"dilate":         func(walkNodes) []any { return []any{6, map[string]any{"line": 30}} },
	"refine":         func(w walkNodes) []any { return []any{nodeArg(w.img), map[string]any{"radius": 4}} },
	"overlay": func(w walkNodes) []any {
		return []any{nodeArg(w.regions), map[string]any{"labels": "id", "color": []any{0.2, 0.9, 0.3}}}
	},
	"compare":       func(w walkNodes) []any { return []any{[]any{nodeArg(w.edited)}} },
	"where":         func(walkNodes) []any { return []any{map[string]any{"area": map[string]any{"gte": 2}}} },
	"without":       func(walkNodes) []any { return []any{[]any{1}} },
	"only":          func(walkNodes) []any { return []any{1} },
	"sortBy":        func(walkNodes) []any { return []any{"area"} },
	"top":           func(walkNodes) []any { return []any{3} },
	"within":        func(w walkNodes) []any { return []any{nodeArg(w.mask), nodeArg(w.edited)} },
	"heal":          func(w walkNodes) []any { return []any{nodeArg(w.regions), map[string]any{"radius": 1}} },
	"crop":          func(walkNodes) []any { return []any{[]any{5.0, 7.0, 21.0, 13.0}} },
	"highpass":      func(walkNodes) []any { return []any{2.0} },
	"dog":           func(walkNodes) []any { return []any{1.0, 4.0, map[string]any{"polarity": "bright"}} },
	"localContrast": func(walkNodes) []any { return []any{3.0} },
	"view":          func(walkNodes) []any { return []any{"speck"} },
	"annotate": func(walkNodes) []any {
		return []any{[]any{map[string]any{"circle": []any{20.0, 20.0, 5.0}, "label": "a"}, map[string]any{"box": []any{3.0, 4.0, 10.0, 8.0}}}}
	},
	"inpaint": func(w walkNodes) []any { return []any{nodeArg(w.mask2)} },
	"blend": func(w walkNodes) []any {
		return []any{nodeArg(w.edited), map[string]any{"mode": "screen", "mask": nodeArg(w.mask)}}
	},
}

// walkSkip are ops the Go-only walk cannot evaluate (data lambdas need a
// JavaScript runtime; they are covered by the runtime tests).
var walkSkip = map[string]bool{"filter": true, "split": true} // split builds 3 nodes: channel_test.go

// TestRegistryWalk is the registry and channel property test (GCV-004
// §7.5, §14.1): every op is built with sample arguments on colour, grey and
// mask receivers (as its channel requirement allows) and, in two passes,
// with colour and grey image arguments (every grey/colour combination of a
// two-image op: m.mul(img) is colour), evaluated at preview scale and at
// 1.0; the result has the declared kind, Buf.C == Node.Channels, and a
// mask's values lie in [0, 1]. Help renders for every op.
func TestRegistryWalk(t *testing.T) {
	e, colour := newEngine(t)
	grey := call(t, e, "toGray", colour.ID)
	mask := call(t, e, "mask.luminance", colour.ID, map[string]any{"above": 0.3})
	stats := call(t, e, "stats", colour.ID)
	regs := call(t, e, "components", mask.ID)
	for _, op := range e.Registry().Ops() {
		if _, err := e.Registry().Help(op.Name); err != nil {
			t.Fatalf("help(%s): %v", op.Name, err)
		}
	}
	for _, arg := range []*graph.Node{colour, grey} {
		w := walkNodes{img: arg, stats: stats, mask: mask, regions: regs,
			mask2:  call(t, e, "mask.box", colour.ID, []any{8, 8, 30, 20}),
			edited: call(t, e, "exposure", arg.ID, -0.5)}
		for _, op := range e.Registry().Ops() {
			if walkSkip[op.Name] {
				continue
			}
			var recvs []*graph.Node
			switch {
			case op.On == graph.KindValue:
				recvs = []*graph.Node{stats}
			case op.On == graph.KindRegions:
				recvs = []*graph.Node{regs}
			case op.On != graph.KindImage: // sources are covered by Open
			case op.OneChannel:
				recvs = []*graph.Node{grey, mask}
			case op.ThreeChannel:
				recvs = []*graph.Node{colour}
			default:
				recvs = []*graph.Node{colour, grey, mask}
			}
			var raw []any
			if f, ok := walkSamples[string(op.On)+":"+op.Name]; ok {
				raw = f(w)
			} else if f, ok := walkSamples[op.Name]; ok {
				raw = f(w)
			}
			for _, r := range recvs {
				walkOne(t, e, op, r, raw)
			}
		}
	}
}

func walkOne(t *testing.T, e *graph.Engine, op *graph.OpDef, r *graph.Node, raw []any) {
	t.Helper()
	what := fmt.Sprintf("%s:%s on %d-channel #%d", op.On, op.Name, r.Channels, r.ID)
	cr, err := e.Call(op.Name, r.ID, raw)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	n := cr.Node()
	for _, scale := range []float64{0.25, 1, 0.5} {
		v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: scale})
		if err != nil {
			t.Fatalf("%s at %g: %v", what, scale, err)
		}
		if want := op.Returns; v.Kind() != want {
			t.Fatalf("%s returned %s, declared %s", what, v.Kind(), op.Returns)
		}
		iv, ok := v.(*graph.ImageValue)
		if !ok {
			continue
		}
		if iv.Buf.C != n.Channels {
			t.Fatalf("%s: %d channels, Node.Channels %d", what, iv.Buf.C, n.Channels)
		}
		if strings.HasPrefix(op.Name, "mask.") || (op.OneChannel && n.Channels == 1) {
			for _, p := range iv.Buf.Pix {
				if p < 0 || p > 1 || p != p {
					t.Fatalf("%s: mask value %v outside [0, 1]", what, p)
				}
			}
		}
	}
}

func TestStatsInsideBoxAndMask(t *testing.T) {
	e, img := newEngine(t)
	ctx := context.Background()
	v, err := e.Evaluate(ctx, img.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	full := v.(*graph.ImageValue).Buf
	// the one luminance: toGray() of the receiver
	lum := evalValue(t, e, call(t, e, "toGray", img.ID), 1).(*graph.ImageValue).Buf.Plane(0)
	// reference: the left half [0, 0, 32, 48] cropped by hand
	crop := buffer.New(32, 48, 3, 1)
	lumCrop := make([]float32, 0, 32*48)
	for y := 0; y < 48; y++ {
		for c := 0; c < 3; c++ {
			copy(crop.Plane(c)[y*32:(y+1)*32], full.Plane(c)[y*64:y*64+32])
		}
		lumCrop = append(lumCrop, lum[y*64:y*64+32]...)
	}
	ref, err := kernels.Stats(ctx, crop, lumCrop, nil)
	if err != nil {
		t.Fatal(err)
	}

	box := call(t, e, "stats", img.ID, map[string]any{"box": []any{0.0, 0.0, 32.0, 48.0}})
	got := evalData(t, e, box, 1).(map[string]any)
	if math.Abs(got["meanL"].(float64)-ref.L.Mean) > 1e-6 || math.Abs(got["medianL"].(float64)-ref.L.Median) > 1e-6 {
		t.Fatalf("box stats %v, want meanL %v medianL %v", got, ref.L.Mean, ref.L.Median)
	}
	if got["area"].(float64) != 32*48 || math.Abs(got["coverage"].(float64)-0.5) > 1e-9 {
		t.Fatalf("area %v coverage %v", got["area"], got["coverage"])
	}
	m := call(t, e, "mask.box", img.ID, []any{0.0, 0.0, 32.0, 48.0})
	byMask := evalData(t, e, call(t, e, "stats", img.ID, map[string]any{"mask": graph.NodeArg{ID: m.ID}}), 1).(map[string]any)
	if byMask["meanL"] != got["meanL"] {
		t.Fatalf("mask %v vs box %v", byMask["meanL"], got["meanL"])
	}
	whole := evalData(t, e, call(t, e, "stats", img.ID), 1).(map[string]any)
	if _, ok := whole["area"]; ok {
		t.Fatal("whole-image stats should not report area")
	}
	// a box outside the frame is refused when the node is built, with the nearest valid box as the fix
	_, err = e.Call("stats", img.ID, []any{map[string]any{"box": []any{500.0, 500.0, 4.0, 4.0}}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgRange || ce.Fix != "img.stats({ box: [60, 44, 4, 4] })" {
		t.Fatalf("box outside the frame: %v", err)
	}
	// an empty mask inside the frame selects no pixels
	empty := call(t, e, "mask.box", img.ID, []any{10.0, 10.0, 0.0, 0.0})
	if _, err := e.Evaluate(ctx, call(t, e, "stats", img.ID, map[string]any{"mask": graph.NodeArg{ID: empty.ID}}).ID, graph.EvalOptions{Scale: 1}); err == nil ||
		!strings.Contains(err.Error(), "selects no pixels") {
		t.Fatalf("empty selection: %v", err)
	}
}

// TestOptionSynonyms: pi runs 3 and 4 both wrote mask.luminance({ min: 0.99 }).
func TestOptionSynonyms(t *testing.T) {
	e, img := newEngine(t)
	m := call(t, e, "mask.box", img.ID, []any{0.0, 0.0, 10.0, 10.0})
	for _, tc := range []struct {
		op   string
		recv graph.NodeID
		raw  []any
		sug  string
		fix  string
	}{
		{"mask.luminance", img.ID, []any{map[string]any{"min": 0.99}}, "above", "mask.luminance({ above: 0.99 })"},
		{"mask.luminance", img.ID, []any{map[string]any{"max": 0.1}}, "below", "mask.luminance({ below: 0.1 })"},
		{"mask.luminance", img.ID, []any{map[string]any{"threshold": 0.8}}, "above", ""},
		{"gaussian", img.ID, []any{map[string]any{"radius": 2.0}}, "sigma", "gaussian({ sigma: 2 })"},
		{"gaussian", img.ID, []any{map[string]any{"r": 2.0}}, "sigma", ""},
		{"feather", m.ID, []any{map[string]any{"r": 2.0}}, "radius", "feather({ radius: 2 })"},
		{"dilate", m.ID, []any{map[string]any{"sigma": 2.0}}, "radius", ""},
		{"detect.specks", img.ID, []any{map[string]any{"threshold": 0.05}}, "minContrast", ""},
		{"detect.specks", img.ID, []any{map[string]any{"maxSize": 50.0}}, "maxArea", ""},
		{"overlay", img.ID, []any{graph.NodeArg{ID: m.ID}, map[string]any{"opacity": 0.3}}, "alpha", ""},
		{"overlay", img.ID, []any{graph.NodeArg{ID: m.ID}, map[string]any{"colour": "red"}}, "color", ""},
		{"levels", img.ID, []any{map[string]any{"low": 0.01}}, "black", ""},
		{"mask.flood", img.ID, []any{[]any{1.0, 1.0}, map[string]any{"tolerance": 0.1}}, "tol", ""},
		{"stats", img.ID, []any{map[string]any{"region": graph.NodeArg{ID: m.ID}}}, "mask", ""},
	} {
		_, err := e.Call(tc.op, tc.recv, tc.raw)
		ce, ok := graph.AsError(err)
		if !ok || ce.Code != graph.EUnknownOption || !strings.Contains(ce.Hint, `Did you mean "`+tc.sug+`"?`) {
			t.Fatalf("%s %v: %+v, want did-you-mean %q", tc.op, tc.raw, err, tc.sug)
		}
		if tc.fix != "" && ce.Fix != tc.fix {
			t.Fatalf("%s %v: fix %q, want %q", tc.op, tc.raw, ce.Fix, tc.fix)
		}
	}
}

// TestOpSynonymsDoNotShadowOps: a synonym must name an existing op, and
// must not itself be an op on a kind where its target lives (it would
// never be consulted, or worse, mean two things); method redirects must not
// be methods.
func TestOpSynonymsDoNotShadowOps(t *testing.T) {
	reg := NewRegistry()
	for s, target := range graph.Synonyms {
		targets := reg.Find(target)
		if len(targets) == 0 {
			t.Errorf("synonym %q → %q: no such op", s, target)
		}
		for _, op := range targets {
			if _, ok := reg.Lookup(op.On, s); ok {
				t.Errorf("synonym %q is also an op on %s", s, op.On)
			}
		}
	}
	for name := range graph.MethodRedirects {
		for _, k := range reg.Receivers() {
			if _, ok := reg.Lookup(k, name); ok {
				t.Errorf("method redirect %q is an op on %s", name, k)
			}
		}
	}
}
