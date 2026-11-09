package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func filterOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "gaussian", On: graph.KindImage, Returns: graph.KindImage, Space: "linear",
			Args: []graph.ArgSpec{{Name: "r", Type: "number", Unit: graph.DocPx, Positional: true, Min: graph.F(0), Max: graph.F(2000),
				Usual: [2]float64{0, 200}, Doc: "standard deviation of the Gaussian"}},
			Identity: func(a graph.Args) bool { return a.DocLength("r") == 0 },
			Halo:     func(a graph.Args) float64 { return 3 * a.DocLength("r") },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				out, err := kernels.Gaussian(ctx, src.Buf, a.Length("r", src.Buf.Scale))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out, src.DocW, src.DocH), nil
			},
			Doc: "Separable Gaussian blur in linear light; r is the standard deviation in doc px.",
			See: []string{"exposure"},
		},
	}
}
