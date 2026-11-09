package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func toneOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "exposure", On: graph.KindImage, Returns: graph.KindImage, Space: "linear",
			Args: []graph.ArgSpec{{Name: "ev", Type: "number", Positional: true, Min: graph.F(-10), Max: graph.F(10),
				Usual: [2]float64{-3, 3}, Doc: "stops: linear values are multiplied by 2^ev"}},
			Identity: func(a graph.Args) bool { return a.Number("ev") == 0 },
			Halo:     func(graph.Args) float64 { return 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				out, err := kernels.Exposure(ctx, src.Buf, a.Number("ev"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out, src.DocW, src.DocH), nil
			},
			Doc: "Change exposure by ev stops in linear light.",
			See: []string{"levels", "contrast", "stats"},
		},
		{
			Name: "levels", On: graph.KindImage, Returns: graph.KindImage, Space: "perceptual",
			Args: []graph.ArgSpec{
				{Name: "black", Type: "number", Default: 0.0, Min: graph.F(-1), Max: graph.F(16), Usual: [2]float64{0, 0.5},
					Doc: "linear input level mapped to 0 (same space as stats percentiles)"},
				{Name: "white", Type: "number", Default: 1.0, Min: graph.F(0), Max: graph.F(64), Usual: [2]float64{0.05, 4},
					Doc: "linear input level mapped to 1"},
				{Name: "gamma", Type: "number", Default: 1.0, Min: graph.F(0.01), Max: graph.F(10), Usual: [2]float64{0.2, 5},
					Doc: "midtone gamma applied in perceptual space (> 1 brightens)"},
			},
			Identity: func(a graph.Args) bool {
				return a.Number("black") == 0 && a.Number("white") == 1 && a.Number("gamma") == 1
			},
			Halo: func(graph.Args) float64 { return 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				b, w := a.Number("black"), a.Number("white")
				if !(w > b) {
					return nil, graph.Errorf(graph.EArgRange, "levels: white (%g) must be above black (%g).", w, b).WithFix("img.levels({ black: s.p01, white: s.p99 })   // with s = img.stats()")
				}
				src := image(in[0])
				out, err := kernels.Levels(ctx, src.Buf, b, w, a.Number("gamma"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out, src.DocW, src.DocH), nil
			},
			Doc: "Map the black and white input levels to 0 and 1 and bend the midtones with gamma.",
			See: []string{"stats", "curve", "contrast"},
		},
		{
			Name: "contrast", On: graph.KindImage, Returns: graph.KindImage, Space: "perceptual",
			Args: []graph.ArgSpec{{Name: "amount", Type: "number", Positional: true, Min: graph.F(-1), Max: graph.F(1),
				Usual: [2]float64{-0.5, 0.5}, Doc: "scale perceptual distance from mid-grey by 1 + amount"}},
			Identity: func(a graph.Args) bool { return a.Number("amount") == 0 },
			Halo:     func(graph.Args) float64 { return 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				out, err := kernels.Contrast(ctx, src.Buf, a.Number("amount"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out, src.DocW, src.DocH), nil
			},
			Doc: "Increase (amount > 0) or decrease contrast around perceptual mid-grey.",
			See: []string{"levels", "exposure"},
		},
		{
			Name: "toGray", On: graph.KindImage, Returns: graph.KindImage, Space: "linear",
			Args: []graph.ArgSpec{{Name: "weights", Type: "enum:rec709,rec601,average", Default: "rec709", Positional: true}},
			Halo: func(graph.Args) float64 { return 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				if src.Buf.C == 1 {
					return src, nil
				}
				out, err := kernels.ToGray(ctx, src.Buf, kernels.GrayWeights[a.String("weights")])
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out, src.DocW, src.DocH), nil
			},
			Doc: "Convert to a single-channel image of weighted linear channels.",
			See: []string{"stats"},
		},
	}
}
