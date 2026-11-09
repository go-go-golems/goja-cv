package ops

import (
	"context"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func compositeOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "within", On: graph.KindImage, Inputs: []graph.Kind{graph.KindMask, graph.KindImage}, Returns: graph.KindImage, Space: "linear",
			Builders: []int{1},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				base, m, edited := image(in[0]), mask(in[1]), image(in[2])
				if err := sameSize("within", base, m); err != nil {
					return nil, err
				}
				if err := sameSize("within", base, edited); err != nil {
					return nil, err
				}
				out, err := kernels.MaskMix(ctx, base.Buf, edited.Buf, m.Buf, 1)
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out, base.DocW, base.DocH), nil
			},
			Doc: "Apply an edit inside a mask: within(mask, i => i.exposure(-0.4)). The function runs once while the graph is built, with a placeholder for this image; its result is blended in linear light through the mask, so params inside it are tracked like any other.",
			See: []string{"blend", "mask.luminance", "feather"},
		},
		{
			Name: "blend", On: graph.KindImage, Inputs: []graph.Kind{graph.KindImage}, Returns: graph.KindImage, Space: "linear",
			Args: []graph.ArgSpec{
				{Name: "mode", Type: "enum:" + joinModes(), Default: "normal"},
				{Name: "opacity", Type: "number", Default: 1.0, Min: graph.F(0), Max: graph.F(1)},
				{Name: "mask", Type: "Mask", Optional: true, Doc: "where to blend (default everywhere)"},
			},
			Identity: func(a graph.Args) bool { return a.Number("opacity") == 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				base, top := image(in[0]), image(in[1])
				if err := sameSize("blend", base, top); err != nil {
					return nil, err
				}
				var m *buffer.Buffer
				if v := a.Node("mask"); v != nil {
					mb := mask(v)
					if err := sameSize("blend", base, mb); err != nil {
						return nil, err
					}
					m = mb.Buf
				}
				out, err := kernels.Blend(ctx, base.Buf, top.Buf, m, kernels.BlendModes[a.String("mode")], float32(a.Number("opacity")))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out, base.DocW, base.DocH), nil
			},
			Doc: "Blend another image over this one in linear light: out = a + (mode(a, b) − a) · opacity · mask.",
			See: []string{"within"},
		},
	}
}

func joinModes() string { return strings.Join(kernels.BlendModeNames, ",") }
