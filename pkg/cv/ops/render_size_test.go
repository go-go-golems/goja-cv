package ops

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// TestRenderExactSize is the render size property (GCV-004 §7.10): for every
// op class (a source, a pixel op, a geometric op, a decoration, a macro and
// a picture composed at its AnalysisScale), at scales below, at and above 1,
// with windows at every edge and in the middle, Render returns a buffer of
// exactly ScaledSize(window, scale) with the window as its frame.
func TestRenderExactSize(t *testing.T) {
	ctx := context.Background()
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "detect.specks", img.ID)
	nodes := map[string]*graph.Node{
		"source":     img,
		"pixel":      call(t, e, "gaussian", img.ID, 1.5),
		"geometric":  call(t, e, "crop", img.ID, []any{37.0, 21.0, 151.0, 97.0}),
		"decoration": call(t, e, "annotate", img.ID, []any{map[string]any{"box": []any{40.0, 40.0, 30.0, 20.0}}}),
		"macro":      call(t, e, "view", img.ID, "speck"),
		"picture":    sheetCall(t, e, graph.NodeArg{ID: specks.ID}, map[string]any{"views": graph.NodeArg{ID: img.ID}, "size": 24.0, "max": 6.0}),
	}
	for name, n := range nodes {
		f := n.Frame
		windows := []graph.Frame{
			f,                              // the whole frame
			{X: f.X, Y: f.Y, W: 13, H: 11}, // top left
			{X: f.X + f.W - 13, Y: f.Y, W: 13, H: 11},            // top right
			{X: f.X, Y: f.Y + f.H - 11, W: 13, H: 11},            // bottom left
			{X: f.X + f.W - 13, Y: f.Y + f.H - 11, W: 13, H: 11}, // bottom right
			{X: f.X + 7, Y: f.Y + 5, W: 31, H: 17},               // inside, odd offsets
		}
		for _, s := range []float64{0.3, 1, 4} {
			for _, w := range windows {
				t.Run(fmt.Sprintf("%s/%g/%v", name, s, w), func(t *testing.T) {
					v, err := e.Render(ctx, n.ID, graph.RenderOptions{Scale: s, Window: &w})
					if err != nil {
						t.Fatal(err)
					}
					iv := v.(*graph.ImageValue)
					ww, wh := buffer.ScaledSize(w.W, w.H, graph.QuantizeScale(s))
					if g := iv.Frame; iv.Buf.W != ww || iv.Buf.H != wh || g.X != w.X || g.Y != w.Y || g.W != w.W || g.H != w.H {
						t.Fatalf("got %d×%d frame %s, want %d×%d frame %s", iv.Buf.W, iv.Buf.H, iv.Frame, ww, wh, w)
					}
				})
			}
		}
	}
}
