package ops

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// maxDiskRadius caps the disk of erode and dilate (doc px): a disk costs
// about one running extremum per chord row, and the largest dust in the
// gold labels has a radius of 64 px (GCV-005 D17). Lines have no such cap.
const maxDiskRadius = 64

// morphOp is erode (isMax false) or dilate (isMax true) on a one-channel
// image, with a disk or, with { line: angle }, a line element (GCV-005 §6.9).
func morphOp(name, doc string, isMax bool) graph.OpDef {
	return graph.OpDef{
		Name: name, On: graph.KindImage, Returns: graph.KindImage, OneChannel: true,
		Args: []graph.ArgSpec{
			{Name: "radius", Type: "number", Unit: graph.DocPx, Positional: true, Min: graph.F(0), Max: graph.F(2000),
				Doc: "radius of the disk (offsets at distance ≤ radius; at most 64), or of the line (its ends about 2·radius apart), in doc px"},
			{Name: "line", Type: "number", Optional: true,
				Doc: "angle in degrees, counter-clockwise from the x axis: a line element in that direction instead of a disk"},
		},
		Validate: func(a graph.Args) error {
			r := a.DocLength("radius")
			if a.Has("line") || r <= maxDiskRadius {
				return nil
			}
			return graph.Errorf(graph.EArgRange, "%s: a disk of radius %s doc px is above the cap of %d.", name, fmtLen(r), maxDiskRadius).
				WithHint(fmt.Sprintf("A disk costs about 2·radius passes per row. Crop first, or apply a smaller disk twice: x.%s(64).%s(64) reaches about 128.", name, name)).
				WithFix("img" + diskChain(name, r))
		},
		Identity: func(a graph.Args) bool { return a.DocLength("radius") == 0 },
		Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			src := image(in[0])
			b := src.Buf
			r := a.Length("radius", b.Scale)
			var (
				p   []float32
				err error
			)
			if a.Has("line") {
				p, err = kernels.LineMinMax(ctx, b.Plane(0), b.W, b.H, r, a.Number("line")*math.Pi/180, isMax)
			} else {
				p, err = kernels.DiskMinMax(ctx, b.Plane(0), b.W, b.H, r, isMax)
			}
			if err != nil {
				return nil, err
			}
			out := buffer.NewLike(b)
			copy(out.Plane(0), p)
			return graph.NewImage(out), nil
		},
		Doc: doc,
		See: []string{"erode", "dilate", "opening", "closing", "tophat", "feather"},
	}
}

// diskChain is a chain of disks of at most 64 that reaches about r:
// ".dilate(64).dilate(36)".
func diskChain(name string, r float64) string {
	var b strings.Builder
	for ; r > maxDiskRadius; r -= maxDiskRadius {
		fmt.Fprintf(&b, ".%s(%d)", name, maxDiskRadius)
	}
	fmt.Fprintf(&b, ".%s(%s)", name, fmtLen(r))
	return b.String()
}

func fmtLen(v float64) string { return fmtG(math.Round(v*100) / 100) }

func morphOps() []graph.OpDef {
	return []graph.OpDef{
		morphOp("erode", "Grey erosion of a one-channel image (a mask, a grey image, a view): each pixel becomes the minimum over a disk of radius r doc px (the offsets at distance ≤ r; r ≤ 64), or with { line: angle } over a line of radius r at that angle (degrees, counter-clockwise; any length). Pixels outside the image take no part. Bright structures the element does not fit in shrink or vanish; a mask's selection shrinks by r. Opening is x.erode(r).dilate(r), a white top-hat x.sub(x.erode(r).dilate(r)) (cv.help(\"opening\"), cv.help(\"tophat\")).", false),
		morphOp("dilate", "Grey dilation of a one-channel image (a mask, a grey image, a view): each pixel becomes the maximum over a disk of radius r doc px (the offsets at distance ≤ r; r ≤ 64), or with { line: angle } over a line of radius r at that angle (degrees, counter-clockwise; any length). Pixels outside the image take no part. A mask's selection grows by r (m.dilate(r).feather(1) for a soft edge). Closing is x.dilate(r).erode(r) (cv.help(\"closing\")).", true),
	}
}
