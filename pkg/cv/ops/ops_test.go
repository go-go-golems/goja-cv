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

// TestRegistryWalk builds every op with sample arguments on the test image,
// evaluates it at preview scale and at 1.0, checks the result kind and that
// help renders (Part 3 §18 "The registry").
func TestRegistryWalk(t *testing.T) {
	e, img := newEngine(t)
	stats := call(t, e, "stats", img.ID)
	num := call(t, e, "field", stats.ID, "meanL")
	samples := map[string][]any{
		"exposure": {0.5}, "contrast": {0.2}, "gaussian": {1.5}, "field": {"meanL"}, "pick": {"meanL", "p99"},
		"percentile": {0.5}, "add": {1}, "sub": {1}, "mul": {2}, "div": {2}, "min": {0}, "max": {0}, "distance": {0.18},
	}
	recv := map[graph.Kind]graph.NodeID{graph.KindImage: img.ID, graph.KindValue: stats.ID}
	for _, op := range e.Registry().Ops() {
		if _, err := e.Registry().Help(op.Name); err != nil {
			t.Fatalf("help(%s): %v", op.Name, err)
		}
		r, ok := recv[op.On]
		if !ok {
			continue // sources are covered by Open/DeclareParam; Mask/Regions/Param have no M1 receivers here
		}
		if op.On == graph.KindValue && op.Name != "field" && op.Name != "pick" && op.Name != "percentile" && op.Name != "explain" {
			r = num.ID
		}
		n, _, err := e.Call(op.Name, r, samples[op.Name])
		if err != nil {
			t.Fatalf("%s:%s: %v", op.On, op.Name, err)
		}
		for _, scale := range []float64{e.PreviewScale(n.ID), 1} {
			v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: scale})
			if err != nil {
				t.Fatalf("%s at %g: %v", op.Name, scale, err)
			}
			want := op.Returns
			if want == graph.KindParam {
				want = graph.KindValue
			}
			if v.Kind() != want {
				t.Fatalf("%s returned %s, declared %s", op.Name, v.Kind(), op.Returns)
			}
		}
	}
}
