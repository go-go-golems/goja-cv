package ops

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// erode and dilate (GCV-005 §6.9): the op runs the kernel on the
// receiver's plane, the disk is capped at 64 doc px, lines are not, colour
// is refused, radius 0 is the receiver.
func TestErodeDilateOps(t *testing.T) {
	e, img := newEngine(t)
	ctx := context.Background()
	grey := call(t, e, "toGray", img.ID)
	g := evalValue(t, e, grey, 1).(*graph.ImageValue)
	gb := g.Buf
	for _, tc := range []struct {
		op   string
		raw  []any
		want func() ([]float32, error)
	}{
		{"erode", []any{3.5}, func() ([]float32, error) { return kernels.DiskMinMax(ctx, gb.Plane(0), gb.W, gb.H, 3.5, false) }},
		{"dilate", []any{2.0}, func() ([]float32, error) { return kernels.DiskMinMax(ctx, gb.Plane(0), gb.W, gb.H, 2, true) }},
		{"dilate", []any{6.0, map[string]any{"line": 30.0}}, func() ([]float32, error) {
			return kernels.LineMinMax(ctx, gb.Plane(0), gb.W, gb.H, 6, math.Pi/6, true)
		}},
		{"erode", []any{500.0, map[string]any{"line": 90.0}}, func() ([]float32, error) { // a long line is accepted
			return kernels.LineMinMax(ctx, gb.Plane(0), gb.W, gb.H, 500, math.Pi/2, false)
		}},
	} {
		cr, err := e.Call(tc.op, grey.ID, tc.raw)
		if err != nil {
			t.Fatalf("%s %v: %v", tc.op, tc.raw, err)
		}
		if ws := cr.Warnings; len(ws) != 0 {
			t.Fatalf("%s %v warned: %+v", tc.op, tc.raw, ws)
		}
		n := cr.Node()
		v := evalValue(t, e, n, 1).(*graph.ImageValue)
		if n.Channels != 1 || n.Enc != grey.Enc || n.Space != grey.Space || v.Enc != g.Enc || v.Frame != g.Frame {
			t.Fatalf("%s: channels %d, %v/%v %s, receiver %v/%v %s", tc.op, n.Channels, n.Enc, n.Space, v.Frame, grey.Enc, grey.Space, g.Frame)
		}
		want, err := tc.want()
		if err != nil {
			t.Fatal(err)
		}
		for i, p := range v.Buf.Pix {
			if p != want[i] {
				t.Fatalf("%s %v at %d: %v, kernel %v", tc.op, tc.raw, i, p, want[i])
			}
		}
	}
	// radius 0 is the receiver (the evaluator's identity short cut)
	if v := evalValue(t, e, call(t, e, "erode", grey.ID, 0.0), 1).(*graph.ImageValue); v.Buf != gb {
		t.Fatal("erode(0) computed a new buffer")
	}
	// a disk above 64 doc px: crop first, or a chain of smaller disks
	_, err := e.Call("dilate", grey.ID, []any{100.0})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgRange || ce.Fix != "img.dilate(64).dilate(36)" ||
		!strings.Contains(ce.Hint, "Crop first, or apply a smaller disk twice") {
		t.Fatalf("dilate(100): %+v", err)
	}
	if _, err := e.Call("erode", grey.ID, []any{65.0}); err == nil {
		t.Fatal("erode(65) was accepted")
	}
	if _, err := e.Call("erode", grey.ID, []any{64.0}); err != nil {
		t.Fatalf("erode(64): %v", err)
	}
	// colour is refused with the toGray fix
	_, err = e.Call("erode", img.ID, []any{2.0})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EWrongKind || ce.Fix != "img.toGray().erode(…)" {
		t.Fatalf("erode on colour: %+v", err)
	}
	// grow and shrink are gone, with no redirect
	m := call(t, e, "mask.box", img.ID, []any{10.0, 10.0, 20.0, 12.0})
	for _, old := range []string{"grow", "shrink"} {
		if _, err := e.Call(old, m.ID, []any{2.0}); err == nil {
			t.Fatalf("%s still exists", old)
		} else if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EUnknownMethod {
			t.Fatalf("%s: %+v", old, err)
		}
	}
	// toMask has no grow option (and no options at all)
	_, err = e.Call("toMask", call(t, e, "components", m.ID).ID, []any{map[string]any{"grow": 2.0}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EUnknownOption || ce.Message != `toMask has no option "grow": it takes none.` {
		t.Fatalf("toMask({ grow }): %+v", err)
	}
	// a mask's selection grows by the disk: a box of 20 × 12 dilated by 2
	// covers 24 × 16 minus the corners outside the disk
	d := evalValue(t, e, call(t, e, "dilate", m.ID, 2.0), 1).(*graph.ImageValue).Buf
	var area float32
	for _, p := range d.Pix {
		area += p
	}
	if want := float32(24*16 - 4*3); area != want { // each corner loses the 3 offsets with dx² + dy² > 4
		t.Fatalf("box dilated by 2: area %v, want %v", area, want)
	}
}

// The composition recipes on a saturated blob (issue 5 of the agent study):
// the white top-hat keeps the blob a filled plateau where the difference
// of Gaussians makes it a ring.
func TestTopHatKeepsSaturatedBlobFilled(t *testing.T) {
	e, img := newEngine(t)
	x := call(t, e, "mask.ellipse", img.ID, []any{22.0, 14.0, 20.0, 20.0}) // a saturated disk of radius 10 at (32, 24)
	opening := call(t, e, "dilate", call(t, e, "erode", x.ID, 12.0).ID, 12.0)
	tophat := evalValue(t, e, call(t, e, "sub", x.ID, graph.NodeArg{ID: opening.ID}), 1).(*graph.ImageValue).Buf
	dog := evalValue(t, e, call(t, e, "dog", x.ID, 1.0, 4.0), 1).(*graph.ImageValue).Buf
	w := tophat.W
	at := func(p []float32, x, y int) float32 { return p[y*w+x] }
	var dogRim float32
	for y := 14; y < 34; y++ {
		for x := 22; x < 42; x++ {
			dx, dy := float64(x)+0.5-32, float64(y)+0.5-24
			if dx*dx+dy*dy <= 64 && at(tophat.Pix, x, y) < 0.999 {
				t.Fatalf("top-hat inside the blob at (%d, %d) = %v", x, y, at(tophat.Pix, x, y))
			}
			dogRim = max(dogRim, at(dog.Pix, x, y))
		}
	}
	if c := at(dog.Pix, 32, 24); c > 0.5*dogRim {
		t.Fatalf("dog: centre %v, rim %v: expected a ring", c, dogRim)
	}
	for i, p := range tophat.Pix {
		if x, y := i%w, i/w; (float64(x)-32)*(float64(x)-32)+(float64(y)-24)*(float64(y)-24) > 13*13 && p != 0 {
			t.Fatalf("top-hat outside the blob at (%d, %d) = %v", x, y, p)
		}
	}
	// the closing fills a dark hole the disk does not fit in
	hole := call(t, e, "minus", call(t, e, "mask.box", img.ID, []any{8.0, 8.0, 48.0, 32.0}).ID, graph.NodeArg{ID: call(t, e, "mask.box", img.ID, []any{30.0, 22.0, 3.0, 3.0}).ID})
	closed := evalValue(t, e, call(t, e, "erode", call(t, e, "dilate", hole.ID, 3.0).ID, 3.0), 1).(*graph.ImageValue).Buf
	if v := at(closed.Pix, 31, 23); v != 1 {
		t.Fatalf("closing left the hole at %v", v)
	}
}
