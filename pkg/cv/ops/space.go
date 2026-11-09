package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// convertEnc keeps the input's encoding; the result is in the space "to".
var convertEnc = &graph.ResultEnc{
	Enc: graph.KeepEnc.Enc,
	Space: func(a graph.Args, _ buffer.Space, _ []*graph.Node) buffer.Space {
		s, _ := buffer.ParseSpace(a.String("to"))
		return s
	},
}

func spaceOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: graph.ConvertOp, On: graph.KindImage, Returns: graph.KindImage, Internal: true, Result: convertEnc,
			Args: []graph.ArgSpec{{Name: "to", Type: "enum:linear,encoded", Positional: true}},
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				to, _ := buffer.ParseSpace(a.String("to"))
				out, err := buffer.Convert(ctx, src.Buf, src.Enc, ec.InputNode(0).Space, to)
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Express an image's values in another working space (inserted by the builder; not callable).",
		},
	}
}
