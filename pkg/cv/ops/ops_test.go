package ops

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

var testTIFF = filepath.Join("..", "..", "..", "testdata", "gradient-64x48-16bit.tif")

func newEngine(t *testing.T) (*graph.Engine, *graph.Node) {
	t.Helper()
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	img, err := e.Open(testTIFF)
	if err != nil {
		t.Fatal(err)
	}
	return e, img
}

func call(t *testing.T, e *graph.Engine, name string, recv graph.NodeID, raw ...any) *graph.Node {
	t.Helper()
	n, _, err := e.Call(name, recv, raw)
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

func TestStatsLevelsLiftingOnTestImage(t *testing.T) {
	e, img := newEngine(t)
	if img.DocW != 64 || img.DocH != 48 || e.PreviewScale(img.ID) != 1 {
		t.Fatalf("source %dx%d preview %g", img.DocW, img.DocH, e.PreviewScale(img.ID))
	}
	s := call(t, e, "stats", img.ID)
	st := evalData(t, e, s, 1).(map[string]any)
	if st["w"] != 64.0 || st["clipLow"].(float64) <= 0 {
		t.Fatalf("stats: %v", st)
	}
	p01 := call(t, e, "field", s.ID, "p01")
	p99 := call(t, e, "field", s.ID, "p99")
	fixed := call(t, e, "levels", img.ID, map[string]any{"black": graph.NodeArg{ID: p01.ID}, "white": graph.NodeArg{ID: p99.ID}})
	fs := evalData(t, e, call(t, e, "stats", fixed.ID), 1).(map[string]any)
	// levels maps each channel; luminance is a weighted sum, so its
	// percentiles land near (not exactly on) 0 and 1 for a colour image...
	if math.Abs(fs["p01"].(float64)) > 0.01 || math.Abs(fs["p99"].(float64)-1) > 0.02 {
		t.Fatalf("levels from stats: p01 %v p99 %v", fs["p01"], fs["p99"])
	}
	// ...and exactly for a gray one (the map is monotone).
	gray := call(t, e, "toGray", img.ID)
	gs := call(t, e, "stats", gray.ID)
	gfixed := call(t, e, "levels", gray.ID, map[string]any{"black": graph.NodeArg{ID: call(t, e, "field", gs.ID, "p01").ID}, "white": graph.NodeArg{ID: call(t, e, "field", gs.ID, "p99").ID}})
	gfs := evalData(t, e, call(t, e, "stats", gfixed.ID), 1).(map[string]any)
	if math.Abs(gfs["p01"].(float64)) > 2e-3 || math.Abs(gfs["p99"].(float64)-1) > 2e-3 {
		t.Fatalf("gray levels from stats: p01 %v p99 %v", gfs["p01"], gfs["p99"])
	}
	// +1 EV doubles mean luminance (no clipping in the kernel)
	m0 := evalData(t, e, call(t, e, "field", s.ID, "meanL"), 1).(float64)
	m1 := evalData(t, e, call(t, e, "field", call(t, e, "stats", call(t, e, "exposure", img.ID, 1).ID).ID, "meanL"), 1).(float64)
	if math.Abs(m1-2*m0) > 1e-6 {
		t.Fatalf("meanL %g -> %g", m0, m1)
	}
	q := call(t, e, "percentile", call(t, e, "stats", img.ID, map[string]any{"percentiles": []any{0.25}}).ID, 0.25)
	if v := evalData(t, e, q, 1).(float64); v <= 0 || v >= m0*2 {
		t.Fatalf("percentile 0.25 = %v", v)
	}
	_, err := e.Evaluate(context.Background(), call(t, e, "percentile", s.ID, 0.3).ID, graph.EvalOptions{Scale: 1})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.ENoField || ce.Fix == "" {
		t.Fatalf("missing percentile: %v", err)
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
	lines := e.Explain(g.ID, 0.5)
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "#2 gaussian(r: 4 px → 2 at scale 0.5)  Image 32×24  cached") ||
		!strings.HasPrefix(lines[1], "  #1 source gradient-64x48-16bit.tif") {
		t.Fatalf("explain %q", lines)
	}
}

// walkNodes are the receivers and node inputs the registry walk uses.
type walkNodes struct {
	img, stats, num, mask, mask2, regions, edited *graph.Node
}

func nodeArg(n *graph.Node) graph.NodeArg { return graph.NodeArg{ID: n.ID} }

// walkSamples gives sample arguments for ops that need some; ops not listed
// are called without arguments.
var walkSamples = map[string]func(w walkNodes) []any{
	"exposure": func(walkNodes) []any { return []any{0.5} }, "contrast": func(walkNodes) []any { return []any{0.2} },
	"gaussian": func(walkNodes) []any { return []any{1.5} }, "field": func(walkNodes) []any { return []any{"meanL"} },
	"pick": func(walkNodes) []any { return []any{"meanL", "p99"} }, "percentile": func(walkNodes) []any { return []any{0.5} },
	"add": func(walkNodes) []any { return []any{1} }, "sub": func(walkNodes) []any { return []any{1} },
	"mul": func(walkNodes) []any { return []any{2} }, "div": func(walkNodes) []any { return []any{2} },
	"min": func(walkNodes) []any { return []any{0} }, "max": func(walkNodes) []any { return []any{0} },
	"distance":       func(walkNodes) []any { return []any{0.18} },
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
	"grow":           func(walkNodes) []any { return []any{2} },
	"shrink":         func(walkNodes) []any { return []any{2} },
	"refine":         func(w walkNodes) []any { return []any{nodeArg(w.img), map[string]any{"r": 4}} },
	"overlay":        func(w walkNodes) []any { return []any{nodeArg(w.img)} },
	"where":          func(walkNodes) []any { return []any{map[string]any{"area": map[string]any{"gte": 2}}} },
	"without":        func(walkNodes) []any { return []any{[]any{1}} },
	"only":           func(walkNodes) []any { return []any{1} },
	"sortBy":         func(walkNodes) []any { return []any{"area"} },
	"top":            func(walkNodes) []any { return []any{3} },
	"toMask":         func(walkNodes) []any { return []any{map[string]any{"grow": 1}} },
	"within":         func(w walkNodes) []any { return []any{nodeArg(w.mask), nodeArg(w.edited)} },
	"blend": func(w walkNodes) []any {
		return []any{nodeArg(w.edited), map[string]any{"mode": "screen", "mask": nodeArg(w.mask)}}
	},
}

// walkSkip are ops the Go-only walk cannot evaluate (data lambdas need a
// JavaScript runtime; they are covered by the runtime tests).
var walkSkip = map[string]bool{"filter": true}

// TestRegistryWalk builds every op with sample arguments on the test image,
// evaluates it at preview scale and at 1.0, checks the result kind and that
// help renders (Part 3 §18 "The registry").
func TestRegistryWalk(t *testing.T) {
	e, img := newEngine(t)
	stats := call(t, e, "stats", img.ID)
	w := walkNodes{img: img, stats: stats, num: call(t, e, "field", stats.ID, "meanL"),
		mask:  call(t, e, "mask.luminance", img.ID, map[string]any{"above": 0.3}),
		mask2: call(t, e, "mask.box", img.ID, []any{8, 8, 30, 20})}
	w.regions = call(t, e, "components", w.mask.ID)
	w.edited = call(t, e, "exposure", img.ID, -0.5)
	recv := map[graph.Kind]graph.NodeID{graph.KindImage: img.ID, graph.KindValue: stats.ID, graph.KindMask: w.mask.ID, graph.KindRegions: w.regions.ID}
	for _, op := range e.Registry().Ops() {
		if _, err := e.Registry().Help(op.Name); err != nil {
			t.Fatalf("help(%s): %v", op.Name, err)
		}
		r, ok := recv[op.On]
		if !ok || walkSkip[op.Name] {
			continue // sources are covered by Open/DeclareParam; Param has no receiver here
		}
		if op.On == graph.KindValue && op.Name != "field" && op.Name != "pick" && op.Name != "percentile" && op.Name != "explain" {
			r = w.num.ID
		}
		var raw []any
		if f, ok := walkSamples[op.Name]; ok {
			raw = f(w)
		}
		n, _, err := e.Call(op.Name, r, raw)
		if err != nil {
			t.Fatalf("%s:%s: %v", op.On, op.Name, err)
		}
		for _, scale := range []float64{e.PreviewScale(n.ID), 1, 0.5} {
			v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: scale})
			if err != nil {
				t.Fatalf("%s:%s at %g: %v", op.On, op.Name, scale, err)
			}
			want := op.Returns
			if want == graph.KindParam {
				want = graph.KindValue
			}
			if v.Kind() != want {
				t.Fatalf("%s returned %s, declared %s", op.Name, v.Kind(), op.Returns)
			}
			if iv, ok := v.(*graph.ImageValue); ok && iv.IsMask() {
				for _, p := range iv.Buf.Pix {
					if p < 0 || p > 1 || p != p {
						t.Fatalf("%s: mask value %v outside [0, 1]", op.Name, p)
					}
				}
			}
		}
	}
}
