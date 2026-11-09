// Package graphtest holds graph helpers for tests: a test-only source op that
// produces a constant image with any encoding and frame, which cv.open cannot
// make (a linear encoding, a non-zero frame origin).
package graphtest

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// SourceOp is the name of the test source op.
const SourceOp = "testsource"

// RegisterSource adds the test source op to reg. Its hidden arguments are
// enc ("srgb" or "linear": the encoding of the image), frame ([x, y, w, h]
// in doc px) and fill (the linear value of each channel: 1 or 3 numbers).
// Like open, the node holds linear values: enc says what they mean.
func RegisterSource(reg *graph.Registry) {
	reg.MustRegister(graph.OpDef{
		Name: SourceOp, On: graph.KindNone, Returns: graph.KindImage, Bespoke: true,
		Args: []graph.ArgSpec{
			{Name: "enc", Type: "enum:srgb,linear", Hidden: true, Default: "srgb"},
			{Name: "frame", Type: "number[]", Hidden: true},
			{Name: "fill", Type: "number[]", Hidden: true},
		},
		DocFrame: func(a graph.Args, _ []*graph.Node) graph.Frame { return frameOf(a) },
		Result: &graph.ResultEnc{
			Enc: func(a graph.Args, _ []*graph.Node) buffer.Encoding {
				e, _ := buffer.ParseEncoding(a.String("enc"))
				return e
			},
			Space: func(graph.Args, buffer.Space, []*graph.Node) buffer.Space { return buffer.SpaceLinear },
		},
		Run: func(_ context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
			f := frameOf(a)
			fill := a.Numbers("fill")
			w, h := buffer.ScaledSize(f.W, f.H, ec.Scale)
			b := buffer.New(w, h, len(fill), ec.Scale)
			for c, v := range fill {
				p := b.Plane(c)
				for i := range p {
					p[i] = float32(v)
				}
			}
			return graph.NewImage(b), nil
		},
		Doc: "Test-only source: a constant image with any encoding and frame.",
	})
}

func frameOf(a graph.Args) graph.Frame {
	l := a.Numbers("frame")
	if len(l) != 4 {
		return graph.Frame{}
	}
	return graph.Frame{X: int(l[0]), Y: int(l[1]), W: int(l[2]), H: int(l[3])}
}

// Source interns a test source node on e (whose registry must have the op):
// a constant image with encoding enc ("srgb" or "linear"), frame f and the
// linear channel values fill (1 or 3 numbers).
func Source(e *graph.Engine, enc string, f graph.Frame, fill ...float64) (*graph.Node, error) {
	op, ok := e.Registry().Lookup(graph.KindNone, SourceOp)
	if !ok {
		return nil, graph.Errorf(graph.EInternal, "graphtest.RegisterSource was not called on this registry")
	}
	vals := make([]any, len(fill))
	for i, v := range fill {
		vals[i] = v
	}
	n, _, err := e.Builder().Intern(op, 0, nil, map[string]any{"enc": enc, "frame": f.Data(), "fill": vals})
	return n, err
}
