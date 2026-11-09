package ops

import (
	"context"
	"fmt"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// Geometric ops (GCV-002 §7.3) only select pixels: a render above scale 1
// passes its window through them, so decorations work before or after.

// cropFrame is the frame of crop(box) on an input with frame in: the box
// rounded outward to whole doc px and clipped to in.
func cropFrame(box [4]float64, in graph.Frame) graph.Frame {
	x0, y0 := int(math.Floor(box[0])), int(math.Floor(box[1]))
	x1, y1 := int(math.Ceil(box[0]+box[2])), int(math.Ceil(box[1]+box[3]))
	return graph.Intersect(graph.Frame{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, in)
}

func cropOp(on graph.Kind) graph.OpDef {
	recv := map[graph.Kind]string{graph.KindImage: "img", graph.KindMask: "mask"}[on]
	return graph.OpDef{
		Name: "crop", On: on, Returns: on, Class: graph.GeometricOp,
		Args: []graph.ArgSpec{{Name: "box", Type: "box", Unit: graph.DocPx, Positional: true, InFrame: true,
			Doc: "[x, y, w, h] in doc px (absolute, in the frame of the file); rounded outward to whole px and clipped to the image"}},
		Validate: func(a graph.Args) error {
			b := a.Box("box")
			if !(b[2] > 0 && b[3] > 0) {
				return graph.Errorf(graph.EArgRange, "crop box [%g, %g, %g, %g] has no area: w and h must be above 0.", b[0], b[1], b[2], b[3]).
					WithFix(fmt.Sprintf("%s.crop([%g, %g, 64, 64])", recv, b[0], b[1]))
			}
			return nil
		},
		DocFrame: func(a graph.Args) graph.Frame { return cropFrame(a.Box("box"), a.Receiver().Frame) },
		Warn: func(_ *graph.Builder, a graph.Args) []graph.Warning {
			b := a.Box("box")
			in := a.Receiver().Frame
			f := cropFrame(b, in)
			if float64(f.X) <= b[0] && float64(f.Y) <= b[1] && float64(f.X+f.W) >= b[0]+b[2] && float64(f.Y+f.H) >= b[1]+b[3] {
				return nil
			}
			return []graph.Warning{{Code: graph.WRange, Message: fmt.Sprintf(
				"crop([%g, %g, %g, %g]) reaches outside the image's frame %s; it is clipped to %s. fix: %s.crop(%s)",
				b[0], b[1], b[2], b[3], in, f, recv, f)}}
		},
		Run: func(_ context.Context, ec *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
			return cropValue(image(in[0]), ec.Node.Frame, ec.Scale)
		},
		Doc: "A window of the image in doc px: coordinates stay absolute (rulers on a crop at (300, 100) read 300 …). Crop last: filters after a crop see only the window. A zoomed look of a crop magnifies it: img.crop([x, y, 64, 64]).look({ zoom: 8 }).",
		See: []string{"look", "levels", "sheet"},
	}
}

// cropValue cuts frame f out of v at scale s. Below scale 1 the cut is
// ScaledSize(f, s) buffer px starting at the buffer pixel nearest to f's
// origin; the value records where its buffer starts in doc px.
func cropValue(v *graph.ImageValue, f graph.Frame, s float64) (graph.Value, error) {
	w, h := buffer.ScaledSize(f.W, f.H, s)
	sx, sy := graph.AxisScales(v)
	ox, oy := v.BufferOrigin()
	x0 := min(max(0, int(math.Round((float64(f.X)-ox)*sx))), v.Buf.W-w)
	y0 := min(max(0, int(math.Round((float64(f.Y)-oy)*sy))), v.Buf.H-h)
	if x0 < 0 || y0 < 0 {
		return nil, graph.Errorf(graph.EInternal, "crop %s: %d×%d buffer of %s is smaller than the cut", f, v.Buf.W, v.Buf.H, v.Frame)
	}
	b, err := buffer.Cut(v.Buf, x0, y0, w, h)
	if err != nil {
		return nil, err
	}
	out := graph.NewImage(b)
	if v.IsMask() {
		out = graph.NewMask(b)
	}
	if s == 1 && sx == 1 && sy == 1 {
		return out, nil
	}
	return out.WithOrigin(ox+float64(x0)/sx, oy+float64(y0)/sy), nil
}

func geometryOps() []graph.OpDef {
	return []graph.OpDef{cropOp(graph.KindImage), cropOp(graph.KindMask)}
}
