package ops

import (
	"context"
	"math"
	"strconv"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
	"github.com/go-go-golems/goja-cv/pkg/cv/text"
)

func overlayArgs() []graph.ArgSpec {
	return []graph.ArgSpec{
		{Name: "color", Type: "any", Default: "red", Doc: "colour name or display [r, g, b] in [0, 1] (encoded values)"},
		{Name: "alpha", Type: "number", Default: 0.5, Min: graph.F(0), Max: graph.F(1), Doc: "opacity of the fill (outlines are drawn opaque)"},
		{Name: "labels", Type: "enum:none,id", Default: "none", Doc: `"id" draws each region's id next to its box`},
	}
}

// drawRegions draws regions onto img for looking: their pixels tinted at
// alpha·0.6, a box outline around each (at least 5 px, 2 px margin) and
// optionally the id above the box.
func drawRegions(ctx context.Context, img *graph.ImageValue, rv *graph.RegionsValue, a graph.Args) (graph.Value, error) {
	if err := graph.SameFrame("overlay", rv.Frame, img.Frame); err != nil {
		return nil, err
	}
	col, err := parseColor(a.Raw("color"), "overlay.color")
	if err != nil {
		return nil, err
	}
	m, err := rv.Rasterize(ctx, img.Buf.W, img.Buf.H, img.Buf.Scale)
	if err != nil {
		return nil, err
	}
	out, err := kernels.TintOverlay(ctx, img.Buf, m, col, float32(a.Number("alpha"))*0.6)
	if err != nil {
		return nil, err
	}
	sx, sy := graph.AxisScales(img)
	short := min(out.W, out.H)
	lw := max(1, int(math.Round(float64(short)/800)))
	k := max(1, int(math.Round(float64(short)/400)))
	labels := a.String("labels") == "id"
	for i, r := range rv.Items {
		if i%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		bx, by := r.Box[0]-float64(img.Frame.X), r.Box[1]-float64(img.Frame.Y) // absolute → buffer origin
		x0 := int(math.Floor(bx*sx)) - 2
		y0 := int(math.Floor(by*sy)) - 2
		x1 := int(math.Ceil((bx+r.Box[2])*sx)) + 1
		y1 := int(math.Ceil((by+r.Box[3])*sy)) + 1
		if x1-x0 < 4 {
			c := (x0 + x1) / 2
			x0, x1 = c-2, c+2
		}
		if y1-y0 < 4 {
			c := (y0 + y1) / 2
			y0, y1 = c-2, c+2
		}
		kernels.DrawRect(out, x0, y0, x1, y1, lw, col, 1)
		if labels {
			ty := y0 - lw - text.Height(k) - 1
			if ty < 0 {
				ty = y1 + lw + 1
			}
			kernels.DrawText(out, x0, ty, strconv.Itoa(r.ID), k, col)
		}
	}
	return graph.NewImage(out), nil
}

// compareGap is the gap between images in compare({ mode: "side" }), doc px.
const compareGap = 4

// compareBackground is the encoded grey between images in a side-by-side
// compare: sRGB_encode(0.02), a dark grey.
const compareBackground = 0.1517

func imageInputs(inputs []*graph.Node) []*graph.Node {
	var out []*graph.Node
	for _, n := range inputs {
		if n.Kind == graph.KindImage {
			out = append(out, n)
		}
	}
	return out
}

func presentOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "overlay", On: graph.KindImage, Inputs: []graph.Kind{"Regions|Mask"}, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceEncoded},
			Args:     overlayArgs(),
			Validate: validateColor("color"),
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				img := image(in[0])
				if rv, ok := in[1].(*graph.RegionsValue); ok {
					return drawRegions(ctx, img, rv, a)
				}
				return tint(ctx, img, mask(in[1]), a)
			},
			Doc: "The image with regions (tinted pixels, box outlines, optional ids) or a mask (tint) drawn over it, for looking.",
			See: []string{"compare", "detect.specks"},
		},
		{
			Name: "overlay", On: graph.KindRegions, Inputs: []graph.Kind{graph.KindImage}, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceEncoded},
			Args:     overlayArgs(),
			Validate: validateColor("color"),
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				return drawRegions(ctx, image(in[1]), regions(in[0]), a)
			},
			Doc: "regions.overlay(img, opts) is img.overlay(regions, opts).",
		},
		{
			Name: "compare", On: graph.KindImage, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceEncoded},
			Args: []graph.ArgSpec{
				{Name: "others", Type: "Image[]", Positional: true, Default: []any{}, Doc: "the images to compare with this one"},
				{Name: "mode", Type: "enum:side,split,diff", Default: "side", Doc: "side by side, vertical strips of each, or the amplified difference of two"},
				{Name: "gain", Type: "number", Default: 4.0, Min: graph.F(1), Max: graph.F(100), Doc: "diff: amplification of the difference in the working space (encoded by default)"},
			},
			Validate: func(a graph.Args) error {
				n := len(a.Raw("others").([]any)) + 1
				if a.String("mode") == "diff" && n != 2 {
					return graph.Errorf(graph.EArgType, "compare({ mode: \"diff\" }) needs exactly two images, got %d.", n).
						WithFix(`cv.compare([before, after], { mode: "diff" })`)
				}
				return nil
			},
			DocFrame: func(a graph.Args, inputs []*graph.Node) graph.Frame {
				ims := imageInputs(inputs)
				f := ims[0].Frame
				if a.String("mode") != "side" {
					return f
				}
				f.W, f.H = 0, 0
				for i, n := range ims {
					if i > 0 {
						f.W += compareGap
					}
					f.W += n.Frame.W
					f.H = max(f.H, n.Frame.H)
				}
				return f
			},
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				ims := []*graph.ImageValue{image(in[0])}
				for _, v := range a.Nodes("others") {
					ims = append(ims, image(v))
				}
				for i := range ims {
					ims[i] = &graph.ImageValue{Buf: kernels.ToRGB(ims[i].Buf), Frame: ims[i].Frame}
				}
				switch a.String("mode") {
				case "diff":
					if err := sameFrame("compare", ims[0], ims[1]); err != nil {
						return nil, err
					}
					out, err := kernels.AbsDiff(ctx, ims[0].Buf, ims[1].Buf, float32(a.Number("gain")))
					if err != nil {
						return nil, err
					}
					return graph.NewImage(out), nil
				case "split":
					out := ims[0].Buf.Clone()
					n := len(ims)
					for k := 1; k < n; k++ {
						if err := sameFrame("compare", ims[0], ims[k]); err != nil {
							return nil, err
						}
						x0, x1 := k*out.W/n, (k+1)*out.W/n
						for c := 0; c < 3; c++ {
							dp, sp := out.Plane(c), ims[k].Buf.Plane(c)
							for y := 0; y < out.H; y++ {
								copy(dp[y*out.W+x0:y*out.W+x1], sp[y*out.W+x0:y*out.W+x1])
								dp[y*out.W+x0] = 1 // separator
							}
						}
					}
					return graph.NewImage(out), nil
				}
				f := ec.Node.Frame
				w, h := buffer.ScaledSize(f.W, f.H, ec.Scale)
				out := buffer.New(w, h, 3, ec.Scale)
				for i := range out.Pix {
					out.Pix[i] = compareBackground
				}
				x := 0
				for _, im := range ims {
					kernels.Paste(out, im.Buf, int(math.Round(float64(x)*ec.Scale)), 0)
					x += im.Frame.W + compareGap
				}
				return graph.NewImage(out), nil
			},
			Doc: `Compare images for looking: cv.compare([a, b, …], { mode }) — "side" by side (top-aligned, 4 doc px apart), "split" into vertical strips (same size), or "diff" (|a − b| × gain in the working space, encoded by default, two images).`,
			See: []string{"overlay"},
		},
	}
}
