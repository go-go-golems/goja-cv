package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// asEnc keeps the receiver's encoding; the result is in the space "to".
var asEnc = &graph.ResultEnc{
	Enc: graph.KeepEnc.Enc,
	Space: func(a graph.Args, _ buffer.Space) buffer.Space {
		s, _ := buffer.ParseSpace(a.String("to"))
		return s
	},
}

func spaceOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: graph.AsOp, On: graph.KindImage, Returns: graph.KindImage, Result: asEnc,
			Args: []graph.ArgSpec{{Name: "to", Type: "enum:linear,encoded", Positional: true,
				ValueDocs: map[string]string{
					"linear":  "the encoding undone: values proportional to light (density on linear-coded tiles)",
					"encoded": "the file's values (file units), what looks show",
				}}},
			Identity: func(a graph.Args) bool {
				r := a.Receiver()
				to, _ := buffer.ParseSpace(a.String("to"))
				return buffer.Identity(r.Enc, r.Space, to)
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				to, _ := buffer.ParseSpace(a.String("to"))
				out, err := buffer.Convert(ctx, src.Buf, src.Enc, a.Receiver().Space, to)
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "The image's values in another space: as(\"linear\") (the encoding undone: light, or density on linear-coded tiles) or as(\"encoded\") (file units, what looks show). " +
				"Every other op returns its result in its receiver's space, so img.as(\"linear\").stats() measures linear light. On a linear-encoded or derived image it is the identity.",
			See: []string{"stats", "exposure", "encode"},
		},
	}
}
