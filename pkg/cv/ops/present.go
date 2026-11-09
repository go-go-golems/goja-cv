package ops

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
	"github.com/go-go-golems/goja-cv/pkg/cv/text"
)

func overlayArgs() []graph.ArgSpec {
	return []graph.ArgSpec{
		{Name: "color", Type: "any", Default: "red", Doc: "colour name or display [r, g, b] in [0, 1] (encoded values)"},
		{Name: "alpha", Type: "number", Default: 0.5, Min: graph.F(0), Max: graph.F(1), Doc: "opacity of the fill (outlines are drawn opaque)"},
		{Name: "labels", Type: "enum:none,id", Default: "none", Doc: `"id" draws each item's label (else its id) next to its box`},
	}
}

// Overlays are decorations (GCV-002 §7.8): in a render above scale 1 they
// receive the image rendered in the window and draw at the render scale, so
// outlines are 1 image px (2 px on looks of at least 1024 px) and ids are
// readable at any zoom. Regions and masks may cover a larger frame than the
// image (regions found on the whole image, drawn on a crop).

// containsFrame returns E_FRAME unless outer contains inner.
func containsFrame(op string, outer, inner graph.Frame) error {
	if inner.X >= outer.X && inner.Y >= outer.Y && inner.X+inner.W <= outer.X+outer.W && inner.Y+inner.H <= outer.Y+outer.H {
		return nil
	}
	return graph.SameFrame(op, outer, inner)
}

// decorationWidth is the line width of outlines on an image of w × h px.
func decorationWidth(w, h int) int {
	if max(w, h) >= 1024 {
		return 2
	}
	return 1
}

// drawRegions draws regions onto img for looking: their pixels tinted at
// alpha·0.6, a box outline around each (at least 5 px, 2 px margin) and
// optionally the id above the box (font size 2).
func drawRegions(ctx context.Context, img *graph.ImageValue, rv *graph.RegionsValue, a graph.Args) (graph.Value, error) {
	if err := itemsCover("overlay", rv, img.Frame); err != nil {
		return nil, err
	}
	col, err := parseColor(a.Raw("color"), "overlay.color")
	if err != nil {
		return nil, err
	}
	t := graph.RasterOf(img)
	m, err := rv.Rasterize(ctx, t)
	if err != nil {
		return nil, err
	}
	out, err := kernels.TintOverlay(ctx, img.Buf, m, col, float32(a.Number("alpha"))*0.6)
	if err != nil {
		return nil, err
	}
	lw := decorationWidth(out.W, out.H)
	const px = 2
	labels := a.String("labels") == "id"
	for i, r := range rv.Items {
		if i%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		x0 := int(math.Floor((r.Box[0]-t.OX)*t.SX)) - 2
		y0 := int(math.Floor((r.Box[1]-t.OY)*t.SY)) - 2
		x1 := int(math.Ceil((r.Box[0]+r.Box[2]-t.OX)*t.SX)) + 1
		y1 := int(math.Ceil((r.Box[1]+r.Box[3]-t.OY)*t.SY)) + 1
		if x1 < -lw || y1 < -lw || x0 >= out.W+lw || y0 >= out.H+lw-text.Height(px) {
			continue // outside the window
		}
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
			ty := y0 - lw - text.Height(px) - 1
			if ty < 0 {
				ty = y1 + lw + 1
			}
			kernels.DrawText(out, x0, ty, r.Text(), px, col)
		}
	}
	return graph.NewImage(out), nil
}

// compareGap is the gap between images in compare({ mode: "side" }), doc px.
const compareGap = 4

// compareBackground is the encoded grey between images in a side-by-side
// compare: sRGB_encode(0.02), a dark grey.
const compareBackground = 0.1517

func presentOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "overlay", On: graph.KindImage, Returns: graph.KindImage, Space: graph.Display, Class: graph.Decoration, Channels: graph.Three,
			Args: append([]graph.ArgSpec{itemsArg("target", graph.ItemsType+"|"+graph.MaskType,
				"what to draw: regions (a Regions node or an array of items: region items, boxes, circles; outlines, tint, labels) or a mask, any one-channel image (tint)")}, overlayArgs()...),
			Validate: validateColor("color"),
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				img := image(in[0])
				target := a.Value("target")
				if rv, ok := target.(*graph.RegionsValue); ok {
					return drawRegions(ctx, img, rv, a)
				}
				return tint(ctx, img, image(target), a)
			},
			DocFrame: func(a graph.Args) graph.Frame { return a.Receiver().Frame },
			Doc:      "The image with items (tinted area, box outlines, optional labels: label, else id) or a mask (tint) drawn over it, for looking. A decoration: in a zoomed look outlines stay 1 image px and labels readable, before or after a crop; regions found on the whole image can be drawn on a crop.",
			See:      []string{"compare", "detect.specks"},
		},
		{
			Name: "compare", On: graph.KindNone, Returns: graph.KindImage, Result: graph.DisplayEnc, Channels: graph.Three,
			Space: graph.Display, // a layout of what looks show
			Args: []graph.ArgSpec{
				{Name: "images", Type: "Image[]", Positional: true, Doc: "the images to compare: [before, after, …]"},
				{Name: "mode", Type: "enum:side,split", Default: "side", Doc: "side by side (top-aligned, 4 doc px apart) or split into vertical strips (same frame)",
					Redirects: map[string]graph.Redirect{"diff": {
						Hint: "compare lays images out; a difference is an image of its own: subtract (signed, zero = grey with .mul(4).add(0.5)).",
						Fix:  "a.sub(b).mul(4).add(0.5)"}}},
			},
			Validate: func(a graph.Args) error {
				if n := len(a.Raw("images").([]any)); n == 0 {
					return graph.Errorf(graph.EArgType, "cv.compare needs at least one image.").WithFix(`cv.compare([img, edited], { mode: "side" })`)
				}
				return nil
			},
			DocFrame: func(a graph.Args) graph.Frame {
				ims := a.Inputs("images")
				f := ims[0].Frame
				f.Picture = true
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
			Run: func(_ context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
				var ims []*graph.ImageValue
				for _, v := range a.Values("images") {
					im := image(v)
					ims = append(ims, &graph.ImageValue{Buf: kernels.ToRGB(im.Buf), Frame: im.Frame})
				}
				if a.String("mode") == "split" {
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
			Doc: `Compare images for looking: cv.compare([a, b, …], { mode }) — "side" by side (top-aligned, 4 doc px apart) or "split" into vertical strips (same frame). The result is a picture (its pixels are not doc px). For a difference, subtract: a.sub(b).`,
			See: []string{"sub", "overlay", "sheet"},
		},
	}
}
