package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func TestOverlayAndCompare(t *testing.T) {
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "where", call(t, e, "detect.specks", img.ID).ID, map[string]any{"score": map[string]any{"gte": DustScore}})
	ov := call(t, e, "overlay", img.ID, graph.NodeArg{ID: specks.ID}, map[string]any{"labels": "id", "color": "red"})
	same := call(t, e, "overlay", specks.ID, graph.NodeArg{ID: img.ID}, map[string]any{"labels": "id", "color": "red"})
	ob := evalValue(t, e, ov, 1).(*graph.ImageValue).Buf
	if sb := evalValue(t, e, same, 1).(*graph.ImageValue).Buf; len(sb.Pix) != len(ob.Pix) || sb.Pix[1234] != ob.Pix[1234] {
		t.Fatal("regions.overlay(img) differs from img.overlay(regions)")
	}
	src := evalValue(t, e, img, 1).(*graph.ImageValue).Buf
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
	side := call(t, e, "compare", img.ID, []any{graph.NodeArg{ID: healed.ID}})
	if side.DocW != 2*256+compareGap || side.DocH != 192 {
		t.Fatalf("side doc size %d×%d", side.DocW, side.DocH)
	}
	sv := evalValue(t, e, side, 1).(*graph.ImageValue)
	hb := evalValue(t, e, healed, 1).(*graph.ImageValue).Buf
	if sv.Buf.W != 516 || sv.Buf.Pix[0] != src.Pix[0] || sv.Buf.Pix[260] != hb.Pix[0] {
		t.Fatalf("side: %v", sv.Buf)
	}
	if half := evalValue(t, e, side, 0.5).(*graph.ImageValue).Buf; half.W != 258 || half.H != 96 {
		t.Fatalf("side at 0.5: %v", half)
	}
	split := evalValue(t, e, call(t, e, "compare", img.ID, []any{graph.NodeArg{ID: healed.ID}}, map[string]any{"mode": "split"}), 1).(*graph.ImageValue).Buf
	if split.W != 256 || split.Pix[10] != src.Pix[10] || split.Pix[200] != hb.Pix[200] {
		t.Fatal("split")
	}
	diff := call(t, e, "compare", img.ID, []any{graph.NodeArg{ID: healed.ID}}, map[string]any{"mode": "diff"})
	db := evalValue(t, e, diff, 1).(*graph.ImageValue).Buf
	var nz int
	for _, v := range db.Pix {
		if v > 0 {
			nz++
		}
	}
	if nz == 0 || nz > len(db.Pix)/5 {
		t.Fatalf("diff non-zero samples %d of %d", nz, len(db.Pix))
	}
	if _, _, err := e.Call("compare", img.ID, []any{[]any{graph.NodeArg{ID: healed.ID}, graph.NodeArg{ID: img.ID}}, map[string]any{"mode": "diff"}}); err == nil {
		t.Fatal("diff of three images accepted")
	}
	if _, _, err := e.Call("compare", img.ID, []any{[]any{graph.NodeArg{ID: specks.ID}}}); err == nil {
		t.Fatal("regions accepted as an image to compare")
	}
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
