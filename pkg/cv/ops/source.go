package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func sourceOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "open", On: graph.KindNone, Returns: graph.KindImage, Space: "source",
			Args: []graph.ArgSpec{
				{Name: "path", Type: "string", Positional: true, Doc: "image file (PNG, JPEG, TIFF 8/16-bit)"},
				{Name: "stamp", Type: "string", Hidden: true},
				{Name: "w", Type: "number", Hidden: true},
				{Name: "h", Type: "number", Hidden: true},
			},
			DocSize: func(a graph.Args, _ []*graph.Node) (int, int) { return a.Int("w"), a.Int("h") },
			Run: func(ctx context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
				src, err := ec.Eval.Sources().Load(ctx, a.String("path"), a.String("stamp"))
				if err != nil {
					return nil, err
				}
				buf, err := src.Pyr.AtScale(ctx, ec.Scale)
				if err != nil {
					return nil, err
				}
				return graph.NewImage(buf, src.Pyr.DocW(), src.Pyr.DocH()), nil
			},
			Doc: "Open an image file as a source node (decoded on first evaluation, sRGB → linear float32).",
			See: []string{"stats", "explain"},
		},
		{
			Name: "param", On: graph.KindNone, Returns: graph.KindParam, Space: "source",
			Args: []graph.ArgSpec{{Name: "name", Type: "string", Positional: true}},
			Run: func(_ context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
				v, ok := ec.Params[a.String("name")]
				if !ok {
					return nil, graph.Errorf(graph.EArgType, "param %q is not declared.", a.String("name"))
				}
				return data(v), nil
			},
			Doc: "A named value a person can move (declare with P.slider / P.toggle / P.choice).",
		},
	}
}
