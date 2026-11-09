package ops

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func mask(v graph.Value) *graph.ImageValue { return v.(*graph.ImageValue) }

func newMask(b *buffer.Buffer, like *graph.ImageValue) *graph.ImageValue {
	return graph.NewMask(b, like.DocW, like.DocH)
}

// sameSize fails when two buffers of one op do not line up (masks made from
// different images).
func sameSize(op string, a, b *graph.ImageValue) error {
	if a.Buf.W != b.Buf.W || a.Buf.H != b.Buf.H {
		return graph.Errorf(graph.EWrongKind, "%s: the inputs have different sizes (%d×%d doc px vs %d×%d doc px).",
			op, a.DocW, a.DocH, b.DocW, b.DocH).WithHint("Masks and images must come from the same image.")
	}
	return nil
}

var namedColors = map[string][3]float64{
	"red": {1, 0.1, 0.1}, "green": {0.1, 0.9, 0.2}, "blue": {0.15, 0.35, 1}, "yellow": {1, 0.9, 0.1},
	"cyan": {0.1, 0.9, 1}, "magenta": {1, 0.2, 0.9}, "orange": {1, 0.55, 0.1}, "white": {1, 1, 1}, "black": {0, 0, 0},
}

func colorNames() []string {
	return []string{"red", "green", "blue", "yellow", "cyan", "magenta", "orange", "white", "black"}
}

// parseColor reads a colour name or a perceptual [r, g, b] in [0, 1] and
// returns it in linear light.
func parseColor(v any, where string) ([3]float32, error) {
	var p [3]float64
	switch x := v.(type) {
	case string:
		c, ok := namedColors[x]
		if !ok {
			return [3]float32{}, graph.Errorf(graph.EArgType, "%s = %q is not a colour name.", where, x).WithValid(colorNames())
		}
		p = c
	case []any:
		if len(x) != 3 {
			return [3]float32{}, graph.Errorf(graph.EArgType, "%s must be a colour name or [r, g, b] in [0, 1].", where)
		}
		for i, e := range x {
			f, ok := e.(float64)
			if !ok || f < 0 || f > 1 {
				return [3]float32{}, graph.Errorf(graph.EArgType, "%s must be a colour name or [r, g, b] in [0, 1].", where)
			}
			p[i] = f
		}
	default:
		return [3]float32{}, graph.Errorf(graph.EArgType, "%s must be a colour name or [r, g, b] in [0, 1].", where).WithValid(colorNames())
	}
	return [3]float32{buffer.ToLinear(float32(p[0])), buffer.ToLinear(float32(p[1])), buffer.ToLinear(float32(p[2]))}, nil
}

func validateColor(arg string) func(graph.Args) error {
	return func(a graph.Args) error {
		v := a.Raw(arg)
		if _, ok := v.(graph.InputRef); ok || v == nil {
			return nil
		}
		_, err := parseColor(v, arg)
		return err
	}
}

var edges = []string{"top", "bottom", "left", "right"}

func isRef(v any) bool { _, ok := v.(graph.InputRef); return ok }

func validateGradient(a graph.Args) error {
	from, to := a.Raw("from"), a.Raw("to")
	switch f := from.(type) {
	case string:
		if !contains(edges, f) {
			return graph.Errorf(graph.EArgType, "mask.gradient.from = %q is not an edge or a point.", f).WithValid(edges)
		}
		if t, ok := to.(float64); ok && !(t > 0 && t <= 1) {
			return graph.Errorf(graph.EArgRange, "mask.gradient.to = %g: from an edge, to is the fraction of the height/width where the mask reaches 0, in (0, 1].", t)
		} else if !ok && !isRef(to) {
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
		return graph.Errorf(graph.EArgType, "mask.gradient.from must be an edge or a point [x, y].").WithValid(edges)
	}
	return nil
}

func maskMakerOps() []graph.OpDef {
	boxArg := graph.ArgSpec{Name: "box", Type: "number[]", Unit: graph.DocPx, Len: 4, Positional: true, Doc: "[x, y, w, h] in doc px"}
	return []graph.OpDef{
		{
			Name: "mask.luminance", On: graph.KindImage, Returns: graph.KindMask, Space: "perceptual",
			Args: []graph.ArgSpec{
				{Name: "above", Type: "number", Optional: true, Min: graph.F(-1), Max: graph.F(2), Doc: "select perceptual luminance above this"},
				{Name: "below", Type: "number", Optional: true, Min: graph.F(-1), Max: graph.F(2), Doc: "select perceptual luminance below this"},
				{Name: "softness", Type: "number", Default: 0.05, Min: graph.F(0), Max: graph.F(1), Doc: "width of the smooth ramp around each threshold"},
			},
			Validate: func(a graph.Args) error {
				if !a.Has("above") && !a.Has("below") {
					return graph.Errorf(graph.EArgType, "mask.luminance needs above, below or both.").
						WithFix("img.mask.luminance({ above: 0.6 })")
				}
				return nil
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
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
				return newMask(m, src), nil
			},
			Doc: "Select by perceptual (sRGB-encoded) luminance, the scale of histogram(): above and/or below thresholds with a soft ramp.",
			See: []string{"mask.color", "feather", "histogram"},
		},
		{
			Name: "mask.color", On: graph.KindImage, Returns: graph.KindMask, Space: "perceptual",
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
				return newMask(m, src), nil
			},
			Doc: "Select by hue (degrees) within tol, for pixels at least minSat saturated.",
			See: []string{"mask.luminance"},
		},
		{
			Name: "mask.box", On: graph.KindImage, Returns: graph.KindMask, Space: "geometry",
			Args: []graph.ArgSpec{boxArg},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				sx, sy := graph.AxisScales(src)
				b := a.XY("box", sx, sy)
				return newMask(kernels.BoxMask(src.Buf.W, src.Buf.H, src.Buf.Scale, b[0], b[1], b[2], b[3]), src), nil
			},
			Doc: "A rectangle [x, y, w, h] in doc px; edge pixels are weighted by their exact overlap.",
			See: []string{"mask.ellipse"},
		},
		{
			Name: "mask.ellipse", On: graph.KindImage, Returns: graph.KindMask, Space: "geometry",
			Args: []graph.ArgSpec{boxArg},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				sx, sy := graph.AxisScales(src)
				b := a.XY("box", sx, sy)
				m, err := kernels.EllipseMask(ctx, src.Buf.W, src.Buf.H, src.Buf.Scale, b[0], b[1], b[2], b[3])
				if err != nil {
					return nil, err
				}
				return newMask(m, src), nil
			},
			Doc: "The ellipse inscribed in the box [x, y, w, h] (doc px), anti-aliased.",
			See: []string{"mask.box"},
		},
		{
			Name: "mask.gradient", On: graph.KindImage, Returns: graph.KindMask, Space: "geometry",
			Args: []graph.ArgSpec{
				{Name: "from", Type: "any", Unit: graph.DocPx, Default: "top", Doc: `"top" | "bottom" | "left" | "right" or a point [x, y] (doc px) where the mask is 1`},
				{Name: "to", Type: "any", Unit: graph.DocPx, Default: 0.5, Doc: "from an edge: the fraction of height/width where the mask reaches 0; from a point: the point [x, y] (doc px) where it reaches 0"},
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
						return nil, graph.Errorf(graph.EArgRange, "mask.gradient.to = %v must be in (0, 1] from an edge.", a.Raw("to"))
					}
					m, err = kernels.EdgeGradientMask(ctx, w, h, sc, from, to)
				} else {
					sx, sy := graph.AxisScales(src)
					p0, p1 := a.XY("from", sx, sy), a.XY("to", sx, sy)
					m, err = kernels.PointGradientMask(ctx, w, h, sc, p0[0], p0[1], p1[0], p1[1])
				}
				if err != nil {
					return nil, err
				}
				return newMask(m, src), nil
			},
			Doc: `A linear gradient: 1 at an edge falling to 0 at fraction "to" of the image ({ from: "top", to: 0.45 }), or 1 at point "from" falling to 0 at point "to".`,
			See: []string{"mask.luminance", "and"},
		},
		{
			Name: "mask.flood", On: graph.KindImage, Returns: graph.KindMask, Space: "perceptual",
			Args: []graph.ArgSpec{
				{Name: "point", Type: "number[]", Unit: graph.DocPx, Len: 2, Positional: true, Doc: "seed [x, y] in doc px"},
				{Name: "tol", Type: "number", Default: 0.05, Min: graph.F(0), Max: graph.F(1), Doc: "max perceptual difference per channel from the seed colour"},
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				sx, sy := graph.AxisScales(src)
				p := a.XY("point", sx, sy)
				x, y := int(math.Floor(p[0])), int(math.Floor(p[1]))
				if x < 0 || y < 0 || x >= src.Buf.W || y >= src.Buf.H {
					return nil, graph.Errorf(graph.EArgRange, "mask.flood: point [%g, %g] is outside the %d×%d image.",
						a.Raw("point").([]any)[0], a.Raw("point").([]any)[1], src.DocW, src.DocH)
				}
				m, err := kernels.FloodMask(ctx, src.Buf, x, y, a.Number("tol"))
				if err != nil {
					return nil, err
				}
				return newMask(m, src), nil
			},
			Doc: "The connected area around a point whose colour is within tol of the point's colour (magic wand).",
			See: []string{"mask.color", "grow"},
		},
	}
}

func combineOp(name, doc string, f func(x, y float32) float32) graph.OpDef {
	return graph.OpDef{
		Name: name, On: graph.KindMask, Inputs: []graph.Kind{graph.KindMask}, Returns: graph.KindMask, Space: "selection",
		Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
			a, b := mask(in[0]), mask(in[1])
			if err := sameSize(name, a, b); err != nil {
				return nil, err
			}
			out, err := kernels.Combine(ctx, a.Buf, b.Buf, f)
			if err != nil {
				return nil, err
			}
			return newMask(out, a), nil
		},
		Doc: doc,
		See: []string{"and", "or", "minus", "not"},
	}
}

func lengthMaskOp(name, doc string, k func(ctx context.Context, m *buffer.Buffer, r float64) (*buffer.Buffer, error)) graph.OpDef {
	return graph.OpDef{
		Name: name, On: graph.KindMask, Returns: graph.KindMask, Space: "selection",
		Args: []graph.ArgSpec{{Name: "r", Type: "number", Unit: graph.DocPx, Positional: true, Min: graph.F(0), Max: graph.F(2000),
			Usual: [2]float64{0, 200}, Doc: "distance in doc px"}},
		Identity: func(a graph.Args) bool { return a.DocLength("r") == 0 },
		Halo:     func(a graph.Args) float64 { return 2 * a.DocLength("r") },
		Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			m := mask(in[0])
			out, err := k(ctx, m.Buf, a.Length("r", m.Buf.Scale))
			if err != nil {
				return nil, err
			}
			return newMask(out, m), nil
		},
		Doc: doc,
		See: []string{"grow", "shrink", "feather"},
	}
}

func maskOps() []graph.OpDef {
	return []graph.OpDef{
		combineOp("and", "Intersection of two masks (pointwise minimum).", func(x, y float32) float32 { return min(x, y) }),
		combineOp("or", "Union of two masks (pointwise maximum).", func(x, y float32) float32 { return max(x, y) }),
		combineOp("minus", "This mask without the other one: min(a, 1 − b).", func(x, y float32) float32 { return min(x, 1-y) }),
		{
			Name: "not", On: graph.KindMask, Returns: graph.KindMask, Space: "selection",
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				m := mask(in[0])
				out, err := kernels.MapMask(ctx, m.Buf, func(x float32) float32 { return 1 - x })
				if err != nil {
					return nil, err
				}
				return newMask(out, m), nil
			},
			Doc: "The inverse selection: 1 − m.",
			See: []string{"minus"},
		},
		lengthMaskOp("feather", "Soften the mask edge: a Gaussian blur of standard deviation r/2 (doc px), so the ramp is about r wide on each side.", kernels.Feather),
		lengthMaskOp("grow", "Dilate the selection (m ≥ 0.5) by r doc px using an exact Euclidean distance transform.", kernels.Grow),
		lengthMaskOp("shrink", "Erode the selection (m ≥ 0.5) by r doc px using an exact Euclidean distance transform.", kernels.Shrink),
		{
			Name: "refine", On: graph.KindMask, Inputs: []graph.Kind{graph.KindImage}, Returns: graph.KindMask, Space: "selection",
			Args: []graph.ArgSpec{
				{Name: "r", Type: "number", Unit: graph.DocPx, Default: 8.0, Min: graph.F(0.5), Max: graph.F(500), Doc: "window radius in doc px"},
				{Name: "eps", Type: "number", Default: 1e-3, Min: graph.F(1e-8), Max: graph.F(1), Doc: "regularization: larger smooths more, smaller follows edges more"},
			},
			Halo: func(a graph.Args) float64 { return 2 * a.DocLength("r") },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				m, img := mask(in[0]), image(in[1])
				if err := sameSize("refine", m, img); err != nil {
					return nil, err
				}
				guide, err := kernels.PerceptualLuminance(ctx, img.Buf)
				if err != nil {
					return nil, err
				}
				r := max(1, int(math.Round(a.Length("r", m.Buf.Scale))))
				q, err := kernels.GuidedFilter(ctx, guide, m.Buf.Plane(0), m.Buf.W, m.Buf.H, r, a.Number("eps"))
				if err != nil {
					return nil, err
				}
				out := buffer.NewLike(m.Buf)
				copy(out.Pix, q)
				return newMask(out, m), nil
			},
			Doc: "Snap soft mask edges to the image's edges with a guided filter (guide: perceptual luminance).",
			See: []string{"feather"},
		},
		{
			Name: "coverage", On: graph.KindMask, Returns: graph.KindValue, Space: "analysis", Global: true,
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				return data(mask(in[0]).Coverage()), nil
			},
			Doc: "The fraction of the image the mask selects (its mean), in [0, 1].",
		},
		{
			Name: "overlay", On: graph.KindMask, Inputs: []graph.Kind{graph.KindImage}, Returns: graph.KindImage, Space: "present",
			Args: []graph.ArgSpec{
				{Name: "color", Type: "any", Default: "red", Doc: "colour name or perceptual [r, g, b]"},
				{Name: "alpha", Type: "number", Default: 0.5, Min: graph.F(0), Max: graph.F(1)},
			},
			Validate: validateColor("color"),
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				m, img := mask(in[0]), image(in[1])
				return tint(ctx, img, m, a)
			},
			Doc: "The image with the mask tinted over it, for looking at a selection.",
			See: []string{"coverage"},
		},
	}
}

func tint(ctx context.Context, img, m *graph.ImageValue, a graph.Args) (graph.Value, error) {
	if err := sameSize("overlay", m, img); err != nil {
		return nil, err
	}
	col, err := parseColor(a.Raw("color"), "overlay.color")
	if err != nil {
		return nil, err
	}
	out, err := kernels.TintOverlay(ctx, img.Buf, m.Buf, col, float32(a.Number("alpha")))
	if err != nil {
		return nil, err
	}
	return graph.NewImage(out, img.DocW, img.DocH), nil
}

func contains(xs []string, x string) bool {
	for _, e := range xs {
		if e == x {
			return true
		}
	}
	return false
}
