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

// Meta is the test source's metadata: Enc ("srgb" or "linear": the
// encoding of the image), Frame (doc px) and Fill (the linear value of each
// channel: 1 or 3 numbers).
type Meta struct {
	Enc   string      `json:"enc"`
	Frame graph.Frame `json:"frame"`
	Fill  []float64   `json:"fill"`
}

// RegisterSource adds the test source op to reg. Like open, the node holds
// encoded values (file units): Fill gives the linear value of each channel
// and Meta.Enc how it is encoded.
func RegisterSource(reg *graph.Registry) {
	reg.MustRegister(graph.OpDef{
		Name: SourceOp, On: graph.KindNone, Returns: graph.KindImage, Bespoke: true,
		DocFrame: func(a graph.Args) graph.Frame { return graph.Meta[Meta](a).Frame },
		Channels: func(a graph.Args) int { return len(graph.Meta[Meta](a).Fill) },
		Result: &graph.ResultEnc{
			Enc: func(a graph.Args) buffer.Encoding {
				e, _ := buffer.ParseEncoding(graph.Meta[Meta](a).Enc)
				return e
			},
			Space: func(graph.Args, buffer.Space) buffer.Space { return buffer.SpaceEncoded },
		},
		Run: func(_ context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
			m := graph.Meta[Meta](a)
			w, h := buffer.ScaledSize(m.Frame.W, m.Frame.H, ec.Scale)
			b := buffer.New(w, h, len(m.Fill), ec.Scale)
			enc, _ := buffer.ParseEncoding(m.Enc)
			for c, v := range m.Fill {
				if enc == buffer.EncSRGB {
					v = buffer.LinearToSRGB(v)
				}
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

// Source interns a test source node on e (whose registry must have the op):
// a constant image with encoding enc ("srgb" or "linear"), frame f and the
// linear channel values fill (1 or 3 numbers).
func Source(e *graph.Engine, enc string, f graph.Frame, fill ...float64) (*graph.Node, error) {
	op, ok := e.Registry().Lookup(graph.KindNone, SourceOp)
	if !ok {
		return nil, graph.Errorf(graph.EInternal, "graphtest.RegisterSource was not called on this registry")
	}
	n, _, err := e.Builder().InternSource(op, nil, Meta{Enc: enc, Frame: f, Fill: fill})
	return n, err
}
