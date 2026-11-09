package ops

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// sameFrame fails with E_FRAME when two values of one op cover different
// frames (masks made from another image or window).
func sameFrame(op string, a, b *graph.ImageValue) error {
	return graph.SameFrame(op, a.Frame, b.Frame)
}

var namedColors = map[string][3]float64{
	"red": {1, 0.1, 0.1}, "green": {0.1, 0.9, 0.2}, "blue": {0.15, 0.35, 1}, "yellow": {1, 0.9, 0.1},
	"cyan": {0.1, 0.9, 1}, "magenta": {1, 0.2, 0.9}, "orange": {1, 0.55, 0.1}, "white": {1, 1, 1}, "black": {0, 0, 0},
}

// colorNames lists the names suggested in errors: the overlay-tuned basics
// (any CSS colour name and #rgb / #rrggbb also work).
func colorNames() []string {
	return []string{"red", "green", "blue", "yellow", "cyan", "magenta", "orange", "white", "black", "lime", "#rrggbb"}
}

const colorFix = `color: "red"   // or a CSS name ("lime"), "#ff8800", or [1, 0, 0] (display r, g, b)`

// lookupColor resolves a colour name (the overlay-tuned basics first, then
// CSS names, case-insensitive) or #rgb / #rrggbb to display (sRGB-encoded) r, g, b.
func lookupColor(s string) ([3]float64, bool) {
	k := strings.ToLower(strings.TrimSpace(s))
	if c, ok := namedColors[k]; ok {
		return c, true
	}
	if v, ok := cssColors[k]; ok {
		return [3]float64{float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255}, true
	}
	if strings.HasPrefix(k, "#") {
		h := k[1:]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		if len(h) == 6 {
			if v, err := strconv.ParseUint(h, 16, 32); err == nil {
				return [3]float64{float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255}, true
			}
		}
	}
	return [3]float64{}, false
}

// parseColor reads a colour name, #rgb / #rrggbb, or a display [r, g, b]
// in [0, 1] and returns it as encoded values: overlays run in encoded space,
// where colours are display values.
func parseColor(v any, where string) ([3]float32, error) {
	var p [3]float64
	switch x := v.(type) {
	case string:
		c, ok := lookupColor(x)
		if !ok {
			e := graph.Errorf(graph.EArgType, "%s = %q is not a colour.", where, x).WithValid(colorNames()).WithFix(colorFix)
			all := make([]string, 0, len(cssColors))
			for n := range cssColors {
				all = append(all, n)
			}
			sort.Strings(all)
			if sug := graph.Closest(strings.ToLower(x), all); sug != "" {
				e.Hint = fmt.Sprintf("Did you mean %q? Any CSS colour name, #rgb or #rrggbb works.", sug)
			} else {
				e.Hint = "Use a CSS colour name, #rgb, #rrggbb or [r, g, b] in [0, 1]."
			}
			return [3]float32{}, e
		}
		p = c
	case []any:
		if len(x) != 3 {
			return [3]float32{}, graph.Errorf(graph.EArgType, "%s must be a colour name, #rrggbb or [r, g, b] in [0, 1].", where).WithFix(colorFix)
		}
		for i, e := range x {
			f, ok := e.(float64)
			if !ok || f < 0 || f > 1 {
				return [3]float32{}, graph.Errorf(graph.EArgType, "%s must be a colour name, #rrggbb or [r, g, b] in [0, 1].", where).WithFix(colorFix)
			}
			p[i] = f
		}
	default:
		return [3]float32{}, graph.Errorf(graph.EArgType, "%s must be a colour name, #rrggbb or [r, g, b] in [0, 1].", where).WithValid(colorNames()).WithFix(colorFix)
	}
	return [3]float32{float32(p[0]), float32(p[1]), float32(p[2])}, nil
}

func validateColor(arg string) func(graph.Args) error {
	return func(a graph.Args) error {
		v := a.Raw(arg)
		if v == nil {
			return nil
		}
		_, err := parseColor(v, arg)
		return err
	}
}

var edges = []string{"top", "bottom", "left", "right"}

func validateGradient(a graph.Args) error {
	from, to := a.Raw("from"), a.Raw("to")
	switch f := from.(type) {
	case string:
		if !contains(edges, f) {
			return graph.Errorf(graph.EArgType, "mask.gradient.from = %q is not an edge or a point.", f).WithValid(edges).WithFix(`img.mask.gradient({ from: "top", to: 0.45 })`)
		}
		if t, ok := to.(float64); ok && !(t > 0 && t <= 1) {
			return graph.Errorf(graph.EArgRange, "mask.gradient.to = %g: from an edge, to is the fraction of the height/width where the mask reaches 0, in (0, 1].", t)
		} else if !ok {
			return graph.Errorf(graph.EArgType, "mask.gradient.to must be a fraction in (0, 1] when from is an edge.").
				WithFix(`img.mask.gradient({ from: "top", to: 0.45 })`)
		}
	case []any:
		tl, ok := to.([]any)
		if len(f) != 2 || !ok || len(tl) != 2 {
			return graph.Errorf(graph.EArgType, "mask.gradient from a point needs to: [x, y] too (doc px).").
				WithFix("img.mask.gradient({ from: [0, 0], to: [0, 800] })")
		}
	default:
		return graph.Errorf(graph.EArgType, "mask.gradient.from must be an edge or a point [x, y].").WithValid(edges).WithFix(`img.mask.gradient({ from: "top", to: 0.45 })   // or { from: [x0, y0], to: [x1, y1] }`)
	}
	return nil
}

func maskMakerOps() []graph.OpDef {
	boxArg := graph.ArgSpec{Name: "box", Type: "box", Unit: graph.DocPx, Positional: true, InFrame: true, Doc: "[x, y, w, h] in doc px (absolute: in the image's frame)"}
	return []graph.OpDef{
		{
			Name: "mask.luminance", On: graph.KindImage, Returns: graph.KindImage, Result: graph.DerivedEnc, Channels: graph.One,
			Args: []graph.ArgSpec{
				{Name: "above", Type: "number", Unit: graph.Level, Optional: true, Min: graph.F(-1), Max: graph.F(2), Doc: "select luminance above this level (in the image's space: the scale of stats() and histogram())"},
				{Name: "below", Type: "number", Unit: graph.Level, Optional: true, Min: graph.F(-1), Max: graph.F(2), Doc: "select luminance below this level"},
				{Name: "softness", Type: "number", Default: 0.05, Min: graph.F(0), Max: graph.F(1), Doc: "width of the smooth ramp around each threshold"},
			},
			Validate: func(a graph.Args) error {
				if !a.Has("above") && !a.Has("below") {
					return graph.Errorf(graph.EArgType, "mask.luminance needs above, below or both.").
						WithHint("Thresholds are luminance levels in [0, 1] of the image's space (the scale of stats() and histogram()): above selects highlights, below shadows.").
						WithFix("img.mask.luminance({ above: 0.7 })   // or { below: 0.2 }, or both for a band")
				}
				return nil
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
				src := a.Luma()
				var above, below *float64
				if a.Has("above") {
					v := a.Number("above")
					above = &v
				}
				if a.Has("below") {
					v := a.Number("below")
					below = &v
				}
				m, err := kernels.LuminanceMask(ctx, src.Buf, above, below, a.Number("softness"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(m), nil
			},
			Doc:  "Select by luminance (Rec. 709 of linear light, expressed in the image's space: file units for an opened file, the scale of stats() and histogram()): above and/or below thresholds with a soft ramp.",
			Luma: true,
			See:  []string{"mask.color", "feather", "histogram"},
		},
		{
			Name: "mask.color", On: graph.KindImage, Returns: graph.KindImage, Result: graph.DerivedEnc, Channels: graph.One, ThreeChannel: true,
			Args: []graph.ArgSpec{
				{Name: "hue", Type: "number", Min: graph.F(0), Max: graph.F(360), Doc: "hue in degrees (0 red, 120 green, 240 blue)"},
				{Name: "tol", Type: "number", Default: 20.0, Min: graph.F(0), Max: graph.F(180), Doc: "hue tolerance in degrees"},
				{Name: "minSat", Type: "number", Default: 0.1, Min: graph.F(0), Max: graph.F(1), Doc: "minimum HSV saturation"},
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				m, err := kernels.ColorMask(ctx, src.Buf, a.Number("hue"), a.Number("tol"), a.Number("minSat"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(m), nil
			},
			Doc: "Select by hue (degrees) within tol, for pixels at least minSat saturated.",
			See: []string{"mask.luminance"},
		},
		{
			Name: "mask.box", On: graph.KindImage, Returns: graph.KindImage, Result: graph.DerivedEnc, Channels: graph.One,
			Args: []graph.ArgSpec{boxArg},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				b := a.XY("box", src)
				return graph.NewImage(kernels.BoxMask(src.Buf.W, src.Buf.H, src.Buf.Scale, b[0], b[1], b[2], b[3])), nil
			},
			Doc: "A rectangle [x, y, w, h] in doc px; edge pixels are weighted by their exact overlap.",
			See: []string{"mask.ellipse"},
		},
		{
			Name: "mask.ellipse", On: graph.KindImage, Returns: graph.KindImage, Result: graph.DerivedEnc, Channels: graph.One,
			Args: []graph.ArgSpec{boxArg},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				b := a.XY("box", src)
				m, err := kernels.EllipseMask(ctx, src.Buf.W, src.Buf.H, src.Buf.Scale, b[0], b[1], b[2], b[3])
				if err != nil {
					return nil, err
				}
				return graph.NewImage(m), nil
			},
			Doc: "The ellipse inscribed in the box [x, y, w, h] (doc px), anti-aliased.",
			See: []string{"mask.box"},
		},
		{
			Name: "mask.gradient", On: graph.KindImage, Returns: graph.KindImage, Result: graph.DerivedEnc, Channels: graph.One,
			Args: []graph.ArgSpec{
				{Name: "from", Type: "point|enum:top,bottom,left,right", Unit: graph.DocPx, Default: "top", Doc: `"top" | "bottom" | "left" | "right" or a point [x, y] (doc px) where the mask is 1`},
				{Name: "to", Type: "point|number", Unit: graph.DocPx, Default: 0.5, Doc: "from an edge: the fraction of height/width where the mask reaches 0; from a point: the point [x, y] (doc px) where it reaches 0"},
			},
			Validate: validateGradient,
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				w, h, sc := src.Buf.W, src.Buf.H, src.Buf.Scale
				var m *buffer.Buffer
				var err error
				if from, ok := a.Raw("from").(string); ok {
					to, _ := a.Raw("to").(float64) // a fraction: scale-free
					if !(to > 0 && to <= 1) {
						return nil, graph.Errorf(graph.EArgRange, "mask.gradient.to = %v must be in (0, 1] from an edge.", a.Raw("to")).WithHint("From an edge, to is the fraction of the height/width where the mask reaches 0.").WithFix(`img.mask.gradient({ from: "top", to: 0.45 })`)
					}
					m, err = kernels.EdgeGradientMask(ctx, w, h, sc, from, to)
				} else {
					p0, p1 := a.XY("from", src), a.XY("to", src)
					m, err = kernels.PointGradientMask(ctx, w, h, sc, p0[0], p0[1], p1[0], p1[1])
				}
				if err != nil {
					return nil, err
				}
				return graph.NewImage(m), nil
			},
			Doc: `A linear gradient: 1 at an edge falling to 0 at fraction "to" of the image ({ from: "top", to: 0.45 }), or 1 at point "from" falling to 0 at point "to".`,
			See: []string{"mask.luminance", "and"},
		},
		{
			Name: "mask.flood", On: graph.KindImage, Returns: graph.KindImage, Result: graph.DerivedEnc, Channels: graph.One,
			Args: []graph.ArgSpec{
				{Name: "point", Type: "point", Unit: graph.DocPx, Positional: true, InFrame: true, Doc: "seed [x, y] in doc px"},
				{Name: "tol", Type: "number", Default: 0.05, Min: graph.F(0), Max: graph.F(1), Doc: "max difference per channel from the seed colour, in the image's space"},
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				p := a.XY("point", src)
				x, y := int(math.Floor(p[0])), int(math.Floor(p[1]))
				if x < 0 || y < 0 || x >= src.Buf.W || y >= src.Buf.H {
					f := src.Frame
					return nil, graph.Errorf(graph.EArgRange, "mask.flood: point [%g, %g] is outside the image's frame %s.",
						a.Raw("point").([]any)[0], a.Raw("point").([]any)[1], f).
						WithHint("Points are [x, y] in doc px, absolute in the image's frame; read them off a look and multiply by its doc-px factor.").
						WithFix(fmt.Sprintf("img.mask.flood([%d, %d], { tol: 0.05 })", f.X+f.W/2, f.Y+f.H/2))
				}
				m, err := kernels.FloodMask(ctx, src.Buf, x, y, a.Number("tol"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(m), nil
			},
			Doc: "The connected area around a point whose colour is within tol of the point's colour (magic wand).",
			See: []string{"mask.color", "dilate"},
		},
	}
}

func combineOp(name, doc string, f func(x, y float32) float32) graph.OpDef {
	return graph.OpDef{
		Name: name, On: graph.KindImage, Returns: graph.KindImage, OneChannel: true, Result: graph.DerivedEnc,
		Args: []graph.ArgSpec{{Name: "other", Type: "Mask", Positional: true, Doc: "the other mask (same frame)"}},
		Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, args graph.Args) (graph.Value, error) {
			a, b := image(in[0]), image(args.Value("other"))
			if err := sameFrame(name, a, b); err != nil {
				return nil, err
			}
			out, err := kernels.Combine(ctx, a.Buf, b.Buf, f)
			if err != nil {
				return nil, err
			}
			return graph.NewImage(out), nil
		},
		Doc: doc,
		See: []string{"and", "or", "minus", "not"},
	}
}

func lengthMaskOp(name, doc string, k func(ctx context.Context, m *buffer.Buffer, r float64) (*buffer.Buffer, error)) graph.OpDef {
	return graph.OpDef{
		Name: name, On: graph.KindImage, Returns: graph.KindImage, OneChannel: true, Result: graph.DerivedEnc,
		Args: []graph.ArgSpec{{Name: "radius", Type: "number", Unit: graph.DocPx, Positional: true, Min: graph.F(0), Max: graph.F(2000),
			Usual: [2]float64{0, 200}, Doc: "distance in doc px"}},
		Identity: func(a graph.Args) bool { return a.DocLength("radius") == 0 },
		Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			m := image(in[0])
			out, err := k(ctx, m.Buf, a.Length("radius", m.Buf.Scale))
			if err != nil {
				return nil, err
			}
			return graph.NewImage(out), nil
		},
		Doc: doc,
		See: []string{"dilate", "erode", "feather"},
	}
}

func maskOps() []graph.OpDef {
	return []graph.OpDef{
		combineOp("and", "Intersection of two masks (pointwise minimum).", func(x, y float32) float32 { return min(x, y) }),
		combineOp("or", "Union of two masks (pointwise maximum).", func(x, y float32) float32 { return max(x, y) }),
		combineOp("minus", "This mask without the other one: min(a, 1 − b).", func(x, y float32) float32 { return min(x, 1-y) }),
		{
			Name: "not", On: graph.KindImage, Returns: graph.KindImage, OneChannel: true, Result: graph.DerivedEnc,
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				m := image(in[0])
				out, err := kernels.MapMask(ctx, m.Buf, func(x float32) float32 { return 1 - x })
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "The inverse selection: 1 − m.",
			See: []string{"minus"},
		},
		lengthMaskOp("feather", "Soften the mask edge: a Gaussian blur of standard deviation radius/2 (doc px), so the ramp is about radius wide on each side.", kernels.Feather),
		{
			Name: "refine", On: graph.KindImage, Returns: graph.KindImage, OneChannel: true, Result: graph.DerivedEnc,
			Args: []graph.ArgSpec{
				{Name: "guide", Type: "Image", Positional: true, Doc: "the image whose edges the mask snaps to (its luminance)"},
				{Name: "radius", Type: "number", Unit: graph.DocPx, Default: 8.0, Min: graph.F(0.5), Max: graph.F(500), Doc: "window radius in doc px"},
				{Name: "eps", Type: "number", Default: 1e-3, Min: graph.F(1e-8), Max: graph.F(1), Doc: "regularization: larger smooths more, smaller follows edges more"},
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				m, img := image(in[0]), a.Luma() // the guide's luminance, in the guide's space
				if err := sameFrame("refine", m, img); err != nil {
					return nil, err
				}
				guide := img.Buf.Plane(0)
				r := max(1, int(math.Round(a.Length("radius", m.Buf.Scale))))
				q, err := kernels.GuidedFilter(ctx, guide, m.Buf.Plane(0), m.Buf.W, m.Buf.H, r, a.Number("eps"))
				if err != nil {
					return nil, err
				}
				out := buffer.NewLike(m.Buf)
				copy(out.Pix, q)
				return graph.NewImage(out), nil
			},
			Doc:    "Snap soft mask edges to the image's edges with a guided filter (guide: the image's luminance in its space).",
			Luma:   true,
			LumaOf: "guide",
			See:    []string{"feather"},
		},
		{
			Name: "coverage", On: graph.KindImage, Returns: graph.KindValue, Global: true, OneChannel: true,
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				return data(image(in[0]).Coverage()), nil
			},
			Doc: "The fraction of the image the mask selects (its mean), in [0, 1].",
		},
	}
}

func tint(ctx context.Context, img, m *graph.ImageValue, a graph.Args) (graph.Value, error) {
	if err := containsFrame("overlay", m.Frame, img.Frame); err != nil {
		return nil, err
	}
	if m.Frame != img.Frame { // a mask of the whole image on a crop: cut it like the image
		sx, _ := graph.AxisScales(img)
		cut, err := cropValue(m, img.Frame, sx)
		if err != nil {
			return nil, err
		}
		m = cut.(*graph.ImageValue)
	}
	col, err := parseColor(a.Raw("color"), "overlay.color")
	if err != nil {
		return nil, err
	}
	out, err := kernels.TintOverlay(ctx, img.Buf, m.Buf, col, float32(a.Number("alpha")))
	if err != nil {
		return nil, err
	}
	return graph.NewImage(out), nil
}

func contains(xs []string, x string) bool {
	for _, e := range xs {
		if e == x {
			return true
		}
	}
	return false
}
