package graph_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/ops"
)

// seen records what the test-only decorations received.
type seen struct {
	mu      sync.Mutex
	inW     int
	inScale float64
	ecScale float64
	window  graph.Frame
	level   float64
	regions float64
}

// renderEngine is the full registry on the planted scan, plus test-only ops:
// testframe (a decoration that draws a 1 image-px frame around its input
// and records what it got), testlevel (a decoration with a lifted Value
// argument and a Regions input) and testwarn (a pixel op that warns).
func renderEngine(t *testing.T, s *seen) (*graph.Engine, *graph.Node) {
	t.Helper()
	reg := ops.NewRegistry()
	reg.MustRegister(
		graph.OpDef{
			Name: "testframe", On: graph.KindImage, Returns: graph.KindImage, Class: graph.Decoration,
			Run: func(_ context.Context, ec *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				src := in[0].(*graph.ImageValue)
				s.mu.Lock()
				s.inW, s.inScale, s.ecScale, s.window = src.Buf.W, src.Buf.Scale, ec.Scale, ec.Window
				s.mu.Unlock()
				out := src.Buf.Clone()
				for c := 0; c < out.C; c++ {
					p := out.Plane(c)
					for x := 0; x < out.W; x++ {
						p[x], p[(out.H-1)*out.W+x] = 1, 1
					}
					for y := 0; y < out.H; y++ {
						p[y*out.W], p[y*out.W+out.W-1] = 1, 1
					}
				}
				return graph.NewImage(out), nil
			},
		},
		graph.OpDef{
			Name: "testlevel", On: graph.KindImage, Returns: graph.KindImage, Class: graph.Decoration,
			Args: []graph.ArgSpec{{Name: "regions", Type: "Regions", Positional: true}, {Name: "level", Type: "number", Positional: true}},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				s.mu.Lock()
				s.level = a.Number("level")
				s.regions = a.Value("regions").(*graph.RegionsValue).Scale
				s.mu.Unlock()
				return in[0], nil
			},
		},
		graph.OpDef{
			Name: "testwarn", On: graph.KindImage, Returns: graph.KindImage,
			Run: func(_ context.Context, ec *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				ec.Warn("TEST_WARN", "computed once", "img.testwarn()")
				return graph.NewImage(in[0].(*graph.ImageValue).Buf.Clone()), nil
			},
		},
	)
	e := graph.NewEngine(reg, graph.EngineOptions{})
	p := filepath.Join(t.TempDir(), testimage.PlantedTIFF)
	if _, err := testimage.WritePlanted(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	img, err := e.Open(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	return e, img
}

func render(t *testing.T, e *graph.Engine, n *graph.Node, o graph.RenderOptions) *graph.ImageValue {
	t.Helper()
	v, err := e.Render(context.Background(), n.ID, o)
	if err != nil {
		t.Fatalf("render %s: %v", graph.Label(n), err)
	}
	return v.(*graph.ImageValue)
}

func TestRenderPixelOpIsNearestUpsampledScaleOneWindow(t *testing.T) {
	ctx := context.Background()
	e, img := renderEngine(t, &seen{})
	g, _ := mustCall(t, e, "gaussian", img, 2.0)
	win := graph.Frame{X: 100, Y: 40, W: 24, H: 16}
	got := render(t, e, g, graph.RenderOptions{Scale: 8, Window: &win})
	if got.Frame != win || got.Buf.W != 192 || got.Buf.H != 128 || got.Buf.Scale != 8 {
		t.Fatalf("render: frame %s, %d×%d at %g", got.Frame, got.Buf.W, got.Buf.H, got.Buf.Scale)
	}
	full, err := e.Evaluate(ctx, g.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	fb := full.(*graph.ImageValue).Buf
	for c := 0; c < 3; c++ {
		gp, fp := got.Buf.Plane(c), fb.Plane(c)
		for j := 0; j < got.Buf.H; j++ {
			for i := 0; i < got.Buf.W; i++ {
				want := fp[(win.Y+j/8)*fb.W+win.X+i/8]
				if gp[j*got.Buf.W+i] != want {
					t.Fatalf("pixel (%d, %d, c%d) = %g, want %g", i, j, c, gp[j*got.Buf.W+i], want)
				}
			}
		}
	}
	// the pixel op was computed once, at scale 1 over its whole frame
	if n := e.Evaluator().Runs(g.ID); n != 1 {
		t.Fatalf("gaussian ran %d times", n)
	}
	// linear interpolation keeps the size and the scale-1 range
	lin := render(t, e, g, graph.RenderOptions{Scale: 8, Interp: buffer.InterpLinear, Window: &win})
	differ := 0
	for i := range lin.Buf.Pix {
		if lin.Buf.Pix[i] != got.Buf.Pix[i] {
			differ++
		}
	}
	if lin.Buf.W != 192 || differ == 0 {
		t.Fatalf("linear render: width %d, %d samples differ from nearest", lin.Buf.W, differ)
	}
}

func TestRenderDecorationRunsAtRenderScale(t *testing.T) {
	s := &seen{}
	e, img := renderEngine(t, s)
	fr, _ := mustCall(t, e, "testframe", img)
	win := graph.Frame{X: 10, Y: 20, W: 16, H: 12}
	got := render(t, e, fr, graph.RenderOptions{Scale: 8, Window: &win})
	if s.inW != 128 || s.inScale != 8 || s.ecScale != 8 || s.window != win {
		t.Fatalf("decoration got input width %d at %g, ec scale %g, window %s", s.inW, s.inScale, s.ecScale, s.window)
	}
	// the frame is one image px wide: pixel 0 is the line, pixel 1 is the image
	under := render(t, e, img, graph.RenderOptions{Scale: 8, Window: &win})
	p, u := got.Buf.Plane(0), under.Buf.Plane(0)
	w := got.Buf.W
	if p[5*w] != 1 || p[5*w+1] != u[5*w+1] || p[5*w+w-1] != 1 || p[5*w+w-2] != u[5*w+w-2] {
		t.Fatalf("frame line: %g %g (image %g)", p[5*w], p[5*w+1], u[5*w+1])
	}
	runs := e.Evaluator().Runs(fr.ID)
	// memoized by (node, scale, interp, window)
	render(t, e, fr, graph.RenderOptions{Scale: 8, Window: &win})
	if n := e.Evaluator().Runs(fr.ID); n != runs {
		t.Fatalf("same render ran the decoration again (%d → %d)", runs, n)
	}
	for i, o := range []graph.RenderOptions{
		{Scale: 4, Window: &win},
		{Scale: 8, Interp: buffer.InterpLinear, Window: &win},
		{Scale: 8, Window: &graph.Frame{X: 11, Y: 20, W: 16, H: 12}},
	} {
		render(t, e, fr, o)
		if n := e.Evaluator().Runs(fr.ID); n != runs+i+1 {
			t.Fatalf("render %d: decoration runs %d, want %d (a new key)", i, n, runs+i+1)
		}
	}
}

func TestRenderDataIsNotUpsampled(t *testing.T) {
	ctx := context.Background()
	s := &seen{}
	e, img := renderEngine(t, s)
	st, _ := mustCall(t, e, "stats", img)
	sp, _ := mustCall(t, e, "detect.specks", img)
	lv, _ := mustCall(t, e, "testlevel", img, graph.NodeArg{ID: sp.ID}, 0.25)
	render(t, e, lv, graph.RenderOptions{Scale: 8, Window: &graph.Frame{X: 0, Y: 0, W: 32, H: 32}})
	if s.level != 0.25 || s.regions != 1 {
		t.Fatalf("level %g, regions at scale %g, want 0.25 and 1", s.level, s.regions)
	}
	// rendering a Value node returns its data
	want, err := e.Evaluate(ctx, st.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	v, err := e.Render(ctx, st.ID, graph.RenderOptions{Scale: 8})
	if err != nil || v.(*graph.DataValue).Data.(map[string]any)["meanL"] != want.(*graph.DataValue).Data.(map[string]any)["meanL"] {
		t.Fatalf("render of a Value: %v %v", v, err)
	}
}

func TestRenderScalesAndMacros(t *testing.T) {
	ctx := context.Background()
	e, img := renderEngine(t, &seen{})
	if _, err := e.Evaluate(ctx, img.ID, graph.EvalOptions{Scale: 2}); err == nil || !strings.Contains(err.Error(), graph.EArgRange) {
		t.Fatalf("Evaluate at 2: %v", err)
	}
	if _, err := e.Render(ctx, img.ID, graph.RenderOptions{Scale: 17}); err == nil || !strings.Contains(err.Error(), graph.EArgRange) {
		t.Fatalf("Render at 17: %v", err)
	}
	if _, err := e.Render(ctx, img.ID, graph.RenderOptions{Scale: 2, Window: &graph.Frame{X: 1000, Y: 0, W: 8, H: 8}}); err == nil {
		t.Fatal("a window outside the frame rendered")
	}
	// a whole-image render at 2 is the whole frame doubled
	v := render(t, e, img, graph.RenderOptions{Scale: 2})
	if v.Buf.W != 512 || v.Buf.H != 384 || v.Frame != img.Frame {
		t.Fatalf("render at 2: %d×%d frame %s", v.Buf.W, v.Buf.H, v.Frame)
	}
	// below 1 a render is an evaluation cut to the window: exactly
	// ScaledSize(window, scale) buffer px, starting at the buffer pixel under
	// the window's corner (doc x 10 for x 11 at scale 0.5)
	half := render(t, e, img, graph.RenderOptions{Scale: 0.5, Window: &graph.Frame{X: 11, Y: 10, W: 20, H: 10}})
	if ox, oy := half.BufferOrigin(); half.Buf.W != 10 || half.Buf.H != 5 || ox != 10 || oy != 10 {
		t.Fatalf("render at 0.5 of a 20×10 window: %d×%d from (%g, %g)", half.Buf.W, half.Buf.H, ox, oy)
	}
	// a macro node renders its expansion
	parts, err := e.Call("split", img.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := render(t, e, parts.Nodes[0], graph.RenderOptions{Scale: 3})
	if r.Buf.C != 1 || r.Buf.W != 768 {
		t.Fatalf("channel render: %d channels, %d wide", r.Buf.C, r.Buf.W)
	}
}

func TestEvalTimeWarningsAreReplayedFromTheCache(t *testing.T) {
	ctx := context.Background()
	e, img := renderEngine(t, &seen{})
	w, _ := mustCall(t, e, "testwarn", img)
	g, _ := mustCall(t, e, "gaussian", w, 1.0)
	for i := 0; i < 2; i++ {
		var got []graph.Warning
		var mu sync.Mutex
		_, err := e.Evaluate(ctx, g.ID, graph.EvalOptions{Scale: 1, OnWarn: func(w graph.Warning) {
			mu.Lock()
			got = append(got, w)
			mu.Unlock()
		}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Code != "TEST_WARN" || got[0].Message != "computed once fix: img.testwarn()" {
			t.Fatalf("evaluation %d: warnings %+v", i, got)
		}
	}
	if n := e.Evaluator().Runs(w.ID); n != 1 {
		t.Fatalf("testwarn ran %d times", n)
	}
	// a render reports it too (the pixel op's scale-1 value)
	var n int
	if _, err := e.Render(ctx, g.ID, graph.RenderOptions{Scale: 4, OnWarn: func(graph.Warning) { n++ }}); err != nil || n != 1 {
		t.Fatalf("render: %d warnings, %v", n, err)
	}
}
