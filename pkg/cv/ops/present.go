package ops

import (
	"context"
	"math"
	"strconv"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func overlayArgs() []graph.ArgSpec {
	return []graph.ArgSpec{
		{Name: "color", Type: "any", Default: "red", Doc: "colour name or perceptual [r, g, b]"},
		{Name: "alpha", Type: "number", Default: 0.5, Min: graph.F(0), Max: graph.F(1), Doc: "opacity of the fill (outlines are drawn opaque)"},
		{Name: "labels", Type: "enum:none,id", Default: "none", Doc: `"id" draws each region's id next to its box (digits only)`},
	}
}

// drawRegions draws regions onto img for looking: their pixels tinted at
// alpha·0.6, a box outline around each (at least 5 px, 2 px margin) and
// optionally the id above the box.
func drawRegions(ctx context.Context, img *graph.ImageValue, rv *graph.RegionsValue, a graph.Args) (graph.Value, error) {
	if rv.DocW != img.DocW || rv.DocH != img.DocH {
		return nil, graph.Errorf(graph.EWrongKind, "overlay: the regions belong to a %d×%d image, this one is %d×%d.", rv.DocW, rv.DocH, img.DocW, img.DocH)
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
		x0 := int(math.Floor(r.Box[0]*sx)) - 2
		y0 := int(math.Floor(r.Box[1]*sy)) - 2
		x1 := int(math.Ceil((r.Box[0]+r.Box[2])*sx)) + 1
		y1 := int(math.Ceil((r.Box[1]+r.Box[3])*sy)) + 1
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
			ty := y0 - lw - 5*k - 1
			if ty < 0 {
				ty = y1 + lw + 1
			}
			kernels.DrawDigits(out, x0, ty, strconv.Itoa(r.ID), k, col)
		}
	}
	return graph.NewImage(out, img.DocW, img.DocH), nil
}

// compareGap is the gap between images in compare({ mode: "side" }), doc px.
const compareGap = 4

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
			Name: "overlay", On: graph.KindImage, Inputs: []graph.Kind{"Regions|Mask"}, Returns: graph.KindImage, Space: "present",
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
			Name: "overlay", On: graph.KindRegions, Inputs: []graph.Kind{graph.KindImage}, Returns: graph.KindImage, Space: "present",
			Args:     overlayArgs(),
			Validate: validateColor("color"),
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				return drawRegions(ctx, image(in[1]), regions(in[0]), a)
			},
			Doc: "regions.overlay(img, opts) is img.overlay(regions, opts).",
		},
		{
			Name: "compare", On: graph.KindImage, Returns: graph.KindImage, Space: "present",
			Args: []graph.ArgSpec{
				{Name: "others", Type: "Image[]", Positional: true, Default: []any{}, Doc: "the images to compare with this one"},
				{Name: "mode", Type: "enum:side,split,diff", Default: "side", Doc: "side by side, vertical strips of each, or the amplified difference of two"},
				{Name: "gain", Type: "number", Default: 4.0, Min: graph.F(1), Max: graph.F(100), Doc: "diff: amplification of the perceptual difference"},
			},
			Validate: func(a graph.Args) error {
				n := len(a.Raw("others").([]any)) + 1
				if a.String("mode") == "diff" && n != 2 {
					return graph.Errorf(graph.EArgType, "compare({ mode: \"diff\" }) needs exactly two images, got %d.", n).
						WithFix(`cv.compare([before, after], { mode: "diff" })`)
				}
				return nil
			},
			DocSize: func(a graph.Args, inputs []*graph.Node) (int, int) {
				ims := imageInputs(inputs)
				if a.String("mode") != "side" {
					return ims[0].DocW, ims[0].DocH
				}
				w, h := 0, 0
				for i, n := range ims {
					if i > 0 {
						w += compareGap
					}
					w += n.DocW
					h = max(h, n.DocH)
				}
				return w, h
			},
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				ims := []*graph.ImageValue{image(in[0])}
				for _, v := range a.Nodes("others") {
					ims = append(ims, image(v))
				}
				for i := range ims {
					ims[i] = graph.NewImage(kernels.ToRGB(ims[i].Buf), ims[i].DocW, ims[i].DocH)
				}
				switch a.String("mode") {
				case "diff":
					if err := sameSize("compare", ims[0], ims[1]); err != nil {
						return nil, err
					}
					out, err := kernels.AbsDiff(ctx, ims[0].Buf, ims[1].Buf, float32(a.Number("gain")))
					if err != nil {
						return nil, err
					}
					return graph.NewImage(out, ims[0].DocW, ims[0].DocH), nil
				case "split":
					out := ims[0].Buf.Clone()
					n := len(ims)
					for k := 1; k < n; k++ {
						if err := sameSize("compare", ims[0], ims[k]); err != nil {
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
					return graph.NewImage(out, ims[0].DocW, ims[0].DocH), nil
				}
				docW, docH := 0, 0
				for i, im := range ims {
					if i > 0 {
						docW += compareGap
					}
					docW += im.DocW
					docH = max(docH, im.DocH)
				}
				w, h := buffer.ScaledSize(docW, docH, ec.Scale)
				out := buffer.New(w, h, 3, ec.Scale)
				for i := range out.Pix {
					out.Pix[i] = 0.02
				}
				x := 0
				for _, im := range ims {
					kernels.Paste(out, im.Buf, int(math.Round(float64(x)*ec.Scale)), 0)
					x += im.DocW + compareGap
				}
				return graph.NewImage(out, docW, docH), nil
			},
			Doc: `Compare images for looking: cv.compare([a, b, …], { mode }) — "side" by side (top-aligned, 4 doc px apart), "split" into vertical strips (same size), or "diff" (|a − b| perceptual × gain, two images).`,
			See: []string{"overlay"},
		},
	}
}
