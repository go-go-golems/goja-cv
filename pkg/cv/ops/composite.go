package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func compositeOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "within", On: graph.KindImage, Inputs: []graph.Kind{graph.KindMask, graph.KindImage}, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceLinear},
			Builders: []int{1},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				base, m, edited := image(in[0]), mask(in[1]), image(in[2])
				if err := sameFrame("within", base, m); err != nil {
					return nil, err
				}
				if err := sameFrame("within", base, edited); err != nil {
					return nil, err
				}
				out, err := kernels.MaskMix(ctx, base.Buf, edited.Buf, m.Buf, 1)
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Apply an edit inside a mask: within(mask, i => i.exposure(-0.4)). The function runs once while the graph is built, with a placeholder for this image; its result is blended through the mask in the working space (linear by default), so params inside it are tracked like any other.",
			See: []string{"blend", "mask.luminance", "feather"},
		},
		{
			Name: "blend", On: graph.KindImage, Inputs: []graph.Kind{graph.KindImage}, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceLinear},
			Args: []graph.ArgSpec{
				{Name: "mode", Type: "enum:" + joinModes(), Default: "normal", Redirects: blendRedirects,
					Doc: "normal (the other image), multiply or screen; arithmetic has its own ops: add, sub, min, max"},
				{Name: "opacity", Type: "number", Default: 1.0, Min: graph.F(0), Max: graph.F(1)},
				{Name: "mask", Type: "Mask", Optional: true, Doc: "where to blend (default everywhere)"},
			},
			Identity: func(a graph.Args) bool { return a.Number("opacity") == 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				base, top := image(in[0]), image(in[1])
				if err := sameFrame("blend", base, top); err != nil {
					return nil, err
				}
				var m *buffer.Buffer
				if v := a.Node("mask"); v != nil {
					mb := mask(v)
					if err := sameFrame("blend", base, mb); err != nil {
						return nil, err
					}
					m = mb.Buf
				}
				out, err := kernels.Blend(ctx, base.Buf, top.Buf, m, kernels.BlendModes[a.String("mode")], float32(a.Number("opacity")))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Blend another image over this one in the working space (linear by default): out = a + (mode(a, b) − a) · opacity · mask.",
			See: []string{"within"},
		},
	}
}

// targetMask returns the pixels a heal or inpaint must replace at the
// evaluation scale: rasterized regions (any coverage) or mask values ≥ 0.5.
func targetMask(ctx context.Context, op string, v graph.Value, base *graph.ImageValue) ([]bool, error) {
	var m *buffer.Buffer
	var thr float32 = 0.5
	switch x := v.(type) {
	case *graph.RegionsValue:
		if err := graph.SameFrame(op, x.Frame, base.Frame); err != nil {
			return nil, err
		}
		var err error
		if m, err = x.Rasterize(ctx, graph.RasterOf(base)); err != nil {
			return nil, err
		}
		thr = 1e-3
	case *graph.ImageValue:
		if err := sameFrame(op, base, x); err != nil {
			return nil, err
		}
		m = x.Buf
	default:
		return nil, graph.Errorf(graph.EWrongKind, "%s needs Regions or a Mask.", op)
	}
	pl := m.Plane(0)
	out := make([]bool, len(pl))
	for i, p := range pl {
		out[i] = p >= thr
	}
	return out, nil
}

func healOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "heal", On: graph.KindImage, Inputs: []graph.Kind{"Regions|Mask"}, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceLinear},
			Args: []graph.ArgSpec{
				{Name: "radius", Type: "number", Unit: graph.DocPx, Default: 3.0, Min: graph.F(0), Max: graph.F(200), Usual: [2]float64{0.5, 20},
					Doc: "margin in doc px: each target is grown by this much before it is replaced"},
				{Name: "method", Type: "enum:patch,diffuse", Default: "patch",
					Doc: "patch: copy the best-matching nearby patch and correct the seam (keeps grain); diffuse: smooth harmonic fill"},
			},
			Halo: func(a graph.Args) float64 { return 4*a.DocLength("radius") + 64 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				base := image(in[0])
				t, err := targetMask(ctx, "heal", in[1], base)
				if err != nil {
					return nil, err
				}
				out, err := kernels.Heal(ctx, base.Buf, t, kernels.HealOptions{Margin: a.Length("radius", base.Buf.Scale), Method: a.String("method")})
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Replace each target (regions, or a mask ≥ 0.5) with texture from its surroundings: patch search over clean nearby offsets, then seam correction; or a smooth harmonic fill.",
			See: []string{"inpaint", "detect.specks", "check.dust"},
		},
		{
			Name: "inpaint", On: graph.KindImage, Inputs: []graph.Kind{graph.KindMask}, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceLinear},
			Args: []graph.ArgSpec{{Name: "method", Type: "enum:diffuse,patch", Default: "diffuse"}},
			Halo: func(graph.Args) float64 { return 64 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				base := image(in[0])
				t, err := targetMask(ctx, "inpaint", in[1], base)
				if err != nil {
					return nil, err
				}
				out, err := kernels.Heal(ctx, base.Buf, t, kernels.HealOptions{Method: a.String("method")})
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Fill the mask (≥ 0.5) from its boundary: a smooth harmonic (membrane) fill, or a patch with seam correction.",
			See: []string{"heal"},
		},
	}
}

func joinModes() string { return strings.Join(kernels.BlendModeNames, ",") }

// blendRedirects are the removed blend modes: each is an arithmetic op.
var blendRedirects = func() map[string]graph.Redirect {
	opacity := " With opacity or a mask, blend the result: a.blend(a.%s(b, { space: \"linear\" }), { opacity, mask })."
	return map[string]graph.Redirect{
		"difference": {Hint: "The difference of two images is an image: subtract (sub keeps the sign; abs drops it)." + fmt.Sprintf(opacity, "sub"), Fix: "a.sub(b).abs()"},
		"add":        {Hint: "Adding is an op of its own." + fmt.Sprintf(opacity, "add"), Fix: "a.add(b)"},
		"lighten":    {Hint: "lighten is the per-sample maximum." + fmt.Sprintf(opacity, "max"), Fix: "a.max(b)"},
		"darken":     {Hint: "darken is the per-sample minimum." + fmt.Sprintf(opacity, "min"), Fix: "a.min(b)"},
	}
}()
