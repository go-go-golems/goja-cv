package ops

import (
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func sheetCall(t *testing.T, e *graph.Engine, items any, opts map[string]any) *graph.Node {
	t.Helper()
	cr, err := e.Call("sheet", 0, []any{items, opts})
	if err != nil {
		t.Fatalf("sheet: %v", err)
	}
	return cr.Node()
}

func TestSheetLayout(t *testing.T) {
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "detect.specks", img.ID)
	view := call(t, e, "view", img.ID, "speck")
	// 36 cells of 256 px at the defaults: 6 × 6 within 1568 px
	s := sheetCall(t, e, graph.NodeArg{ID: specks.ID}, map[string]any{"views": graph.NodeArg{ID: view.ID}})
	if f := s.Frame; f.W != 6*256+5*6 || f.H != 6*256+5*6 || !f.Picture {
		t.Fatalf("default sheet frame %s picture %v", f, f.Picture)
	}
	// two views: groups of 514 px, 3 per row, 6 rows: 18 items
	two := sheetCall(t, e, graph.NodeArg{ID: specks.ID}, map[string]any{"views": []any{graph.NodeArg{ID: img.ID}, graph.NodeArg{ID: view.ID}}})
	if f := two.Frame; f.W != 3*514+2*6 || f.H != 6*256+5*6 {
		t.Fatalf("two-view sheet frame %s", f)
	}
	// a repeated view is two views (node arguments are not deduplicated)
	same := sheetCall(t, e, graph.NodeArg{ID: specks.ID}, map[string]any{"views": []any{graph.NodeArg{ID: img.ID}, graph.NodeArg{ID: img.ID}}})
	if f := same.Frame; f.W != 3*514+2*6 || len(same.Args.Inputs("views")) != 2 {
		t.Fatalf("[img, img] sheet frame %s", f)
	}
	_, err := e.Call("sheet", 0, []any{graph.NodeArg{ID: specks.ID}, map[string]any{"views": graph.NodeArg{ID: img.ID}, "max": 40.0}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgRange || ce.Fix != "cv.sheet(items, { views, max: 36 })" {
		t.Fatalf("max 40: %+v", err)
	}
	// a literal list sizes the sheet: 3 points in one row
	pts := sheetCall(t, e, []any{[]any{30.0, 40.0}, map[string]any{"cx": 200.0, "cy": 46.0}, []any{100.0, 100.0, 20.0, 10.0}}, map[string]any{"views": graph.NodeArg{ID: img.ID}, "size": 32.0, "zoom": 2.0})
	if f := pts.Frame; f.W != 3*64+2*6 || f.H != 64 {
		t.Fatalf("3-point sheet frame %s", f)
	}
	if _, err := e.Call("sheet", 0, []any{[]any{[]any{1.0}}, map[string]any{"views": graph.NodeArg{ID: img.ID}}}); err == nil {
		t.Fatal("a bad item was accepted")
	}
	if _, err := e.Call("sheet", 0, []any{graph.NodeArg{ID: specks.ID}}); err == nil || !strings.Contains(err.Error(), "views") {
		t.Fatalf("sheet without views: %v", err)
	}
	// the picture refuses crops and doc-px arguments
	if _, err := e.Call("crop", s.ID, []any{[]any{0.0, 0.0, 10.0, 10.0}}); err == nil || !strings.Contains(err.Error(), "not doc px") {
		t.Fatalf("crop of a sheet: %v", err)
	}
}

func TestSheetCellsLabelsEdgesAndTruncation(t *testing.T) {
	ctx := context.Background()
	e, img, _ := plantedEngine(t)
	// the planted star at (200.5, 46.5) and a point near the corner (an edge window)
	items := []any{[]any{200.5, 46.5}, []any{3.0, 3.0}}
	s := sheetCall(t, e, items, map[string]any{"views": graph.NodeArg{ID: img.ID}, "size": 16.0, "zoom": 4.0})
	v, err := e.Evaluate(ctx, s.ID, graph.EvalOptions{Scale: 0.5}) // a sheet is always evaluated at scale 1
	if err != nil {
		t.Fatal(err)
	}
	b := v.(*graph.ImageValue).Buf
	if b.W != 2*64+6 || b.H != 64 || b.Scale != 1 {
		t.Fatalf("sheet buffer %d×%d at %g", b.W, b.H, b.Scale)
	}
	// Rendered for a look below scale 1, the sheet composed at scale 1 is
	// resized to the requested size (review of PR #3).
	r, err := e.Render(ctx, s.ID, graph.RenderOptions{Scale: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if rb := r.(*graph.ImageValue).Buf; rb.W != 67 || rb.H != 32 || rb.Scale != 0.5 {
		t.Fatalf("sheet rendered at 0.5: %d×%d at %g", rb.W, rb.H, rb.Scale)
	}
	// cell 0 is the star window [192, 38, 16, 16] at ×4: its centre is bright
	if c := b.Plane(1)[34*b.W+34]; c < 0.7 || c <= b.Plane(1)[34*b.W+60] {
		t.Fatalf("star cell centre %g", c)
	}
	// cell 1's window [-5, -5, 16, 16] reaches outside: the outside part is the edge grey
	if c := b.Plane(0)[60*b.W+70+2]; c != sheetEdgeGrey {
		t.Fatalf("edge cell outside pixel %g, want %g", c, sheetEdgeGrey)
	}
	// the gap between cells is the background
	if c := b.Plane(0)[30*b.W+66]; c != compareBackground {
		t.Fatalf("gap %g", c)
	}
	// labels: a dark box in the top-left corner of each cell with light text
	if c := b.Plane(0)[0]; c != sheetLabelBack {
		t.Fatalf("label box %g", c)
	}
	// more regions than cells: SHEET_TRUNCATED, reported by every evaluation (also from the cache)
	specks := call(t, e, "detect.specks", img.ID)
	small := sheetCall(t, e, graph.NodeArg{ID: specks.ID}, map[string]any{"views": graph.NodeArg{ID: img.ID}, "max": 4.0, "zoom": 2.0})
	for i := 0; i < 2; i++ {
		var ws []graph.Warning
		if _, err := e.Evaluate(ctx, small.ID, graph.EvalOptions{Scale: 1, OnWarn: func(w graph.Warning) { ws = append(ws, w) }}); err != nil {
			t.Fatal(err)
		}
		if len(ws) != 1 || ws[0].Code != WSheetTruncated || !strings.HasPrefix(ws[0].Message, "41 items, 4 shown") ||
			!strings.HasSuffix(ws[0].Message, `fix: regions.sortBy("contrast").top(4)`) {
			t.Fatalf("evaluation %d: %+v", i, ws)
		}
	}
	// a large box is shown whole at a lower scale; its label says so
	big := sheetCall(t, e, []any{[]any{20.0, 20.0, 128.0, 128.0}}, map[string]any{"views": graph.NodeArg{ID: img.ID}})
	if bb, err := e.Evaluate(ctx, big.ID, graph.EvalOptions{Scale: 1}); err != nil || bb.(*graph.ImageValue).Buf.W != 256 {
		t.Fatalf("box cell: %v", err)
	}
	if lab, px := cellLabel("0 (84, 84) ×2", "0", 256); lab != "0 (84, 84) ×2" || px != 2 {
		t.Fatalf("label %q at %d", lab, px)
	}
	if lab, px := cellLabel("7 (1337, 131)", "7", 96); lab != "7 (1337, 131)" || px != 1 {
		t.Fatalf("small cell label %q at %d", lab, px)
	}
	if lab, _ := cellLabel("7 (1337, 131) ×1.28 (edge)", "7", 32); lab != "7" {
		t.Fatalf("tiny cell label %q", lab)
	}
}

func TestOverlayAndAnnotateAtZoom(t *testing.T) {
	ctx := context.Background()
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "detect.specks", img.ID)
	// regions found on the whole image, drawn on a crop: the overlay accepts the larger frame
	ov := call(t, e, "overlay", call(t, e, "crop", img.ID, []any{100.0, 40.0, 64.0, 64.0}).ID, graph.NodeArg{ID: specks.ID}, map[string]any{"color": "lime"})
	shown, _ := e.Display(ov.ID)
	for _, z := range []float64{1, 8} {
		v, err := e.Render(ctx, shown, graph.RenderOptions{Scale: z})
		if err != nil {
			t.Fatal(err)
		}
		if w := strokeWidth(v.(*graph.ImageValue).Buf); w != 1 {
			t.Fatalf("overlay outline at zoom %g: %d px wide", z, w)
		}
	}
	an := call(t, e, "annotate", img.ID, []any{map[string]any{"circle": []any{132.0, 72.0, 6.0}, "color": "lime", "label": "7"}})
	for _, z := range []float64{1, 8} {
		v, err := e.Render(ctx, an.ID, graph.RenderOptions{Scale: z, Window: &graph.Frame{X: 112, Y: 52, W: 40, H: 40}})
		if err != nil {
			t.Fatal(err)
		}
		if w := strokeWidth(v.(*graph.ImageValue).Buf); w != 1 {
			t.Fatalf("annotate circle at zoom %g: %d px wide", z, w)
		}
	}
	for _, bad := range []any{
		[]any{map[string]any{"circ": []any{1.0, 2.0, 3.0}}},
		[]any{map[string]any{"circle": []any{1.0, 2.0, 3.0}, "box": []any{1.0, 2.0, 3.0, 4.0}}},
		[]any{map[string]any{"text": "x"}},
	} {
		_, err := e.Call("annotate", img.ID, []any{bad})
		if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgType {
			t.Fatalf("annotate %v: %+v", bad, err)
		}
	}
	_, err := e.Call("annotate", img.ID, []any{[]any{map[string]any{"line": []any{}}}})
	if ce, _ := graph.AsError(err); ce == nil || ce.Hint != `Did you mean "polyline"?` {
		t.Fatalf("line: %+v", err)
	}
}

// strokeWidth is the shortest horizontal run of pure lime samples
// (encoded (0, 1, 0)) in b: the line width of lime outlines (their vertical
// edges cross every row as short runs). 0 when nothing is lime.
func strokeWidth(b *buffer.Buffer) int {
	r, g, bl := b.Plane(0), b.Plane(1), b.Plane(2)
	best := 0
	for y := 0; y < b.H; y++ {
		run := 0
		for x := 0; x <= b.W; x++ {
			lime := x < b.W && r[y*b.W+x] < 0.02 && g[y*b.W+x] > 0.98 && bl[y*b.W+x] < 0.02
			if lime {
				run++
				continue
			}
			if run > 0 && (best == 0 || run < best) {
				best = run
			}
			run = 0
		}
	}
	return best
}

func TestSheetSizesItselfByTop(t *testing.T) {
	e, img, _ := plantedEngine(t)
	top := call(t, e, "top", call(t, e, "sortBy", call(t, e, "detect.specks", img.ID).ID, "area").ID, 8.0)
	s := sheetCall(t, e, graph.NodeArg{ID: top.ID}, map[string]any{"views": graph.NodeArg{ID: img.ID}})
	if f := s.Frame; f.W != 6*256+5*6 || f.H != 2*256+6 {
		t.Fatalf("sheet of top(8): %s, want two rows", f)
	}
}
