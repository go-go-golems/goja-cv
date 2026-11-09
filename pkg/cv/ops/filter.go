package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func filterOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "gaussian", On: graph.KindImage, Returns: graph.KindImage,
			Args: []graph.ArgSpec{{Name: "sigma", Type: "number", Unit: graph.DocPx, Positional: true, Min: graph.F(0), Max: graph.F(2000),
				Usual: [2]float64{0, 200}, Doc: "standard deviation σ of the Gaussian in doc px; the blur reaches about 3σ"}},
			Identity: func(a graph.Args) bool { return a.DocLength("sigma") == 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				out, err := kernels.Gaussian(ctx, src.Buf, a.Length("sigma", src.Buf.Scale))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Separable Gaussian blur in the image's space (img.as(\"linear\").gaussian(…) blurs light); sigma is the standard deviation in doc px: it keeps structures larger than about σ.",
			See: []string{"exposure"},
		},
	}
}
