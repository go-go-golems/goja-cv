package graph_test

import (
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func box(x, y, w, h float64) []any { return []any{x, y, w, h} }

func TestCropFramesAndClipping(t *testing.T) {
	e, img := renderEngine(t, &seen{})
	c, ws := mustCall(t, e, "crop", img, box(100.5, 40, 32, 31.2))
	if want := (graph.Frame{X: 100, Y: 40, W: 33, H: 32}); c.Frame != want || len(ws) != 0 {
		t.Fatalf("crop frame %s (warnings %v), want %s", c.Frame, ws, want)
	}
	// a crop of a crop composes: absolute coordinates, clipped to the inner frame
	cc, ws := mustCall(t, e, "crop", c, box(120, 50, 40, 40))
	if want := (graph.Frame{X: 120, Y: 50, W: 13, H: 22}); cc.Frame != want || len(ws) != 1 || ws[0].Code != graph.WRange ||
		!strings.Contains(ws[0].Message, "clipped to [120, 50, 13, 22]") || !strings.Contains(ws[0].Message, "fix: img.crop([120, 50, 13, 22])") {
		t.Fatalf("crop of a crop: %s %+v", cc.Frame, ws)
	}
	// entirely outside: E_ARG_RANGE with the nearest valid box of the same size
	_, err := e.Call("crop", img.ID, []any{box(300, 10, 64, 64)})
	ce, ok := graph.AsError(err)
	if !ok || ce.Code != graph.EArgRange || ce.Fix != "img.crop([192, 10, 64, 64])" {
		t.Fatalf("outside crop: %+v", err)
	}
	if _, err := e.Call("crop", img.ID, []any{box(10, 10, 0, 5)}); err == nil || !strings.Contains(err.Error(), "no area") {
		t.Fatalf("empty crop: %v", err)
	}
	if _, err := e.Call("crop", img.ID, []any{[]any{graph.NodeArg{ID: img.ID}, 0.0, 8.0, 8.0}}); err == nil {
		t.Fatal("a crop box with a node in it was accepted")
	}

	ctx := context.Background()
	full, _ := e.Evaluate(ctx, img.ID, graph.EvalOptions{Scale: 1})
	v, err := e.Evaluate(ctx, c.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	fb, cb := full.(*graph.ImageValue).Buf, v.(*graph.ImageValue).Buf
	if cb.W != 33 || cb.H != 32 || cb.Plane(1)[5*33+7] != fb.Plane(1)[(40+5)*fb.W+100+7] {
		t.Fatalf("crop at 1: %d×%d, pixel mismatch", cb.W, cb.H)
	}
	// below scale 1 the cut keeps the frame's buffer size and records where it starts
	half, err := e.Evaluate(ctx, c.ID, graph.EvalOptions{Scale: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	hv := half.(*graph.ImageValue)
	if ox, oy := hv.BufferOrigin(); hv.Buf.W != 17 || hv.Buf.H != 16 || ox != 100 || oy != 40 || hv.Frame != c.Frame {
		t.Fatalf("crop at 0.5: %d×%d origin (%g, %g) frame %s", hv.Buf.W, hv.Buf.H, ox, oy, hv.Frame)
	}
	odd, _ := mustCall(t, e, "crop", img, box(101, 41, 20, 20))
	ov, _ := e.Evaluate(ctx, odd.ID, graph.EvalOptions{Scale: 0.5})
	if ox, oy := ov.(*graph.ImageValue).BufferOrigin(); ox != 102 || oy != 42 {
		t.Fatalf("odd crop at 0.5 starts at (%g, %g), want the buffer pixel at (102, 42)", ox, oy)
	}
}

func TestCropRendersAndDecorationsThroughIt(t *testing.T) {
	e, img := renderEngine(t, &seen{})
	b := box(100, 40, 16, 16)
	c, _ := mustCall(t, e, "crop", img, b)
	// a crop rendered at 8 is the source's window, upsampled
	got := render(t, e, c, graph.RenderOptions{Scale: 8})
	win := c.Frame
	want := render(t, e, img, graph.RenderOptions{Scale: 8, Window: &win})
	if got.Frame != c.Frame || got.Buf.W != 128 || got.Buf.Pix[1000] != want.Buf.Pix[1000] {
		t.Fatalf("crop render: frame %s width %d", got.Frame, got.Buf.W)
	}
	// a decoration before the crop draws 1 image-px lines at zoom 8, around the window
	fr, _ := mustCall(t, e, "testframe", img)
	fc, _ := mustCall(t, e, "crop", fr, b)
	v := render(t, e, fc, graph.RenderOptions{Scale: 8})
	p, u, w := v.Buf.Plane(0), want.Buf.Plane(0), v.Buf.W
	if p[7*w] != 1 || p[7*w+1] != u[7*w+1] || p[w*(v.Buf.H-1)+9] != 1 || p[w*(v.Buf.H-2)+9] != u[w*(v.Buf.H-2)+9] {
		t.Fatalf("decoration before crop: line %g, next %g (image %g)", p[7*w], p[7*w+1], u[7*w+1])
	}
	// and after the crop
	cf, _ := mustCall(t, e, "testframe", c)
	v2 := render(t, e, cf, graph.RenderOptions{Scale: 8})
	for i := range v2.Buf.Pix {
		if v2.Buf.Pix[i] != v.Buf.Pix[i] {
			t.Fatalf("annotate→crop and crop→annotate differ at sample %d", i)
		}
	}
}

func TestOrderWarnings(t *testing.T) {
	e, img := renderEngine(t, &seen{})
	c, _ := mustCall(t, e, "crop", img, box(100, 40, 64, 64))
	// cropping first is how an agent works locally: nothing warns (GCV-005 D22)
	for _, tc := range []struct {
		name string
		recv *graph.Node
		raw  []any
	}{{"exposure", c, []any{0.5}}, {"gaussian", c, []any{0.0}}, {"gaussian", c, []any{3.0}}} {
		if _, ws := mustCall(t, e, tc.name, tc.recv, tc.raw...); len(ws) != 0 {
			t.Fatalf("%s after crop warned: %+v", tc.name, ws)
		}
	}
	g, _ := mustCall(t, e, "gaussian", img, 3.0)
	if _, ws := mustCall(t, e, "crop", g, box(100, 40, 64, 64)); len(ws) != 0 {
		t.Fatalf("crop after gaussian warned: %+v", ws)
	}
	fr, _ := mustCall(t, e, "testframe", img)
	_, ws := mustCall(t, e, "levels", fr, map[string]any{"black": 0.1})
	if len(ws) != 1 || ws[0].Code != graph.WDecorationFirst || ws[0].Message != "levels after testframe sees the drawn marks; put testframe last. fix: img.levels(…).testframe(…)" {
		t.Fatalf("levels after a decoration: %+v", ws)
	}
	fc, _ := mustCall(t, e, "crop", fr, box(100, 40, 64, 64))
	if _, ws := mustCall(t, e, "stats", fc); len(ws) != 1 || ws[0].Code != graph.WDecorationFirst {
		t.Fatalf("stats after decoration and crop: %+v", ws)
	}
	// conversions and toGray inserted by the builder (nested calls) do not warn
	if _, ws := mustCall(t, e, "mask.luminance", c, map[string]any{"above": 0.5}); len(ws) != 0 {
		t.Fatalf("mask.luminance after crop warned: %+v", ws)
	}
}
