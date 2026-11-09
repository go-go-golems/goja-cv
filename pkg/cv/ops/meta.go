package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func metaOps() []graph.OpDef {
	var out []graph.OpDef
	for _, k := range graph.Kinds {
		out = append(out, graph.OpDef{
			Name: "explain", On: k, Returns: graph.KindValue, NoCache: true, Lazy: true,
			Run: func(_ context.Context, ec *graph.EvalCtx, _ []graph.Value, _ graph.Args) (graph.Value, error) {
				lines := ec.Eval.Explain(ec.Node.Inputs[0], ec.Scale)
				list := make([]any, len(lines))
				for i, l := range lines {
					list[i] = l
				}
				return data(list), nil
			},
			Doc: "The graph under this node as lines: id, call, kind, size at the evaluation scale, cached. Computes no pixels.",
		})
	}
	return out
}
