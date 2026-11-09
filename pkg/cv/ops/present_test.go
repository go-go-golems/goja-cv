package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// encodedOf evaluates the encoded values of node n (what a look shows) at scale 1.
func encodedOf(t *testing.T, e *graph.Engine, n *graph.Node) *buffer.Buffer {
	t.Helper()
	id, err := e.Display(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	shown, _ := e.Node(id)
	return evalValue(t, e, shown, 1).(*graph.ImageValue).Buf
}

func TestOverlayAndCompare(t *testing.T) {
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "where", call(t, e, "detect.specks", img.ID).ID, map[string]any{"score": map[string]any{"gte": DustScore}})
	ov := call(t, e, "overlay", img.ID, graph.NodeArg{ID: specks.ID}, map[string]any{"labels": "id", "color": "red"})
	same := call(t, e, "overlay", specks.ID, graph.NodeArg{ID: img.ID}, map[string]any{"labels": "id", "color": "red"})
	ob := evalValue(t, e, ov, 1).(*graph.ImageValue).Buf
	if sb := evalValue(t, e, same, 1).(*graph.ImageValue).Buf; len(sb.Pix) != len(ob.Pix) || sb.Pix[1234] != ob.Pix[1234] {
		t.Fatal("regions.overlay(img) differs from img.overlay(regions)")
	}
	src := encodedOf(t, e, img) // overlays and compare run in encoded space
	// box outlines are drawn in red: count strongly red pixels that were not
	red := buffer.ToLinear(1)
	var n int
	for i := 0; i < ob.N(); i++ {
		if ob.Pix[i] == red && src.Pix[i] != red {
			n++
		}
	}
	if n < 40*12 {
		t.Fatalf("only %d red outline pixels for 40 boxes", n)
	}
	healed := call(t, e, "heal", img.ID, graph.NodeArg{ID: specks.ID})
	pair := []any{graph.NodeArg{ID: img.ID}, graph.NodeArg{ID: healed.ID}}
	side := call(t, e, "compare", 0, pair)
	if side.Frame.W != 2*256+compareGap || side.Frame.H != 192 || !side.Frame.Picture {
		t.Fatalf("side doc size %d×%d picture %v", side.Frame.W, side.Frame.H, side.Frame.Picture)
	}
	sv := evalValue(t, e, side, 1).(*graph.ImageValue)
	hb := encodedOf(t, e, healed)
	if sv.Buf.W != 516 || sv.Buf.Pix[0] != src.Pix[0] || sv.Buf.Pix[260] != hb.Pix[0] {
		t.Fatalf("side: %v", sv.Buf)
	}
	if half := evalValue(t, e, side, 0.5).(*graph.ImageValue).Buf; half.W != 258 || half.H != 96 {
		t.Fatalf("side at 0.5: %v", half)
	}
	// The same image twice: the binder lifts it once, the layout still has two
	// panels (review of PR #3).
	twice := call(t, e, "compare", 0, []any{graph.NodeArg{ID: img.ID}, graph.NodeArg{ID: img.ID}})
	if twice.Frame.W != 2*256+compareGap {
		t.Fatalf("compare([img, img]) frame %d px wide", twice.Frame.W)
	}
	if tb := evalValue(t, e, twice, 1).(*graph.ImageValue).Buf; tb.W != 516 || tb.Pix[260] != src.Pix[0] {
		t.Fatalf("compare([img, img]): %d px wide, second panel %g want %g", tb.W, tb.Pix[260], src.Pix[0])
	}
	split := evalValue(t, e, call(t, e, "compare", 0, pair, map[string]any{"mode": "split"}), 1).(*graph.ImageValue).Buf
	if split.W != 256 || split.Pix[10] != src.Pix[10] || split.Pix[200] != hb.Pix[200] {
		t.Fatal("split")
	}
	_, err := e.Call("compare", 0, []any{pair, map[string]any{"mode": "diff"}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgType || ce.Fix != "a.sub(b).mul(4).add(0.5)" {
		t.Fatalf("compare diff: %+v", err)
	}
	if _, err := e.Call("compare", 0, []any{[]any{graph.NodeArg{ID: specks.ID}}}); err == nil {
		t.Fatal("regions accepted as an image to compare")
	}
	if _, err := e.Call("compare", img.ID, []any{[]any{graph.NodeArg{ID: healed.ID}}}); err == nil {
		t.Fatal("img.compare is still a method")
	}
	// a picture refuses crops and doc-px arguments
	for name, raw := range map[string][]any{"crop": {[]any{0.0, 0.0, 8.0, 8.0}}, "gaussian": {2.0}, "stats": {map[string]any{"box": []any{0.0, 0.0, 8.0, 8.0}}}} {
		_, err := e.Call(name, side.ID, raw)
		if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EWrongKind || !strings.Contains(ce.Message, "not doc px") {
			t.Fatalf("%s on a picture: %+v", name, err)
		}
	}
	diff := call(t, e, "sub", healed.ID, graph.NodeArg{ID: img.ID})
	// for looking at the results by hand: CV_LOOK_DIR=/tmp/x go test ./pkg/cv/ops -run OverlayAndCompare
	if dir := os.Getenv("CV_LOOK_DIR"); dir != "" {
		for name, n := range map[string]*graph.Node{"overlay": ov, "side": side, "diff": diff} {
			v := evalValue(t, e, n, 1).(*graph.ImageValue)
			if err := buffer.WriteFile(context.Background(), filepath.Join(dir, name+".png"), v.Buf, "png8"); err != nil {
				t.Fatal(err)
			}
		}
	}
}
