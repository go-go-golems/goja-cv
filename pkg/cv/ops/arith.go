package ops

import (
	"context"
	"fmt"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// Signed image arithmetic (GCV-002 §7.4). The names are those of the Value
// arithmetic (stats().p99.mul(5)); ops are keyed by receiver kind, so both
// coexist. All work in the receiver's space (the operand image is converted
// to it). The operand is an image (same frame; a gray one is broadcast) or
// a number (a level, or a factor for mul).

func arithImageOp(name, doc string, unit graph.Unit, result *graph.ResultEnc, neutral float64, f func(x, y float32) float32) graph.OpDef {
	operandDoc := "an image of the same frame (converted to this image's space), or a number (a level in this image's space)"
	if unit == graph.Unitless {
		operandDoc = "an image of the same frame, or a number (a factor)"
	}
	return graph.OpDef{
		Name: name, On: graph.KindImage, Returns: graph.KindImage, Result: result,
		Args: []graph.ArgSpec{{Name: "b", Type: "Image|number", Unit: unit, Positional: true, Doc: operandDoc}},
		Identity: func(a graph.Args) bool {
			v, ok := a.Raw("b").(float64)
			return ok && v == neutral
		},
		Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			x := image(in[0])
			var out *buffer.Buffer
			var err error
			switch o := a.Raw("b").(type) {
			case graph.InputRef:
				y := image(a.Value("b"))
				if err := sameFrame(name, x, y); err != nil {
					return nil, err
				}
				out, err = kernels.Binary(ctx, x.Buf, y.Buf, f)
			case float64:
				k := float32(o)
				out, err = kernels.Unary(ctx, x.Buf, func(v float32) float32 { return f(v, k) })
			default:
				panic(fmt.Sprintf("cv: %s.b is %T", name, o))
			}
			if err != nil {
				return nil, err
			}
			return graph.NewImage(out), nil
		},
		Doc: doc,
		See: []string{"add", "sub", "mul", "abs", "min", "max", "colormap"},
	}
}

func arithImageOps() []graph.OpDef {
	return []graph.OpDef{
		arithImageOp("add", "a + b per sample, in a's space. d.add(0.5) shows a signed image with zero as mid-grey.",
			graph.Level, nil, 0, func(x, y float32) float32 { return x + y }),
		arithImageOp("sub", "a − b per sample, in a's space (put the photograph first: a derived receiver works in linear): a signed, derived image (its stored value is its value; positive where a is brighter). Show it with .mul(4).add(0.5) or .toGray(\"average\").colormap(\"diverging\").",
			graph.Level, graph.DerivedEnc, 0, func(x, y float32) float32 { return x - y }),
		arithImageOp("mul", "a × b per sample, in a's space; mul(-1) negates.",
			graph.Unitless, nil, 1, func(x, y float32) float32 { return x * y }),
		arithImageOp("min", "The smaller of a and b per sample, in a's space; d.min(0) keeps the negative part.",
			graph.Level, nil, math.Inf(1), func(x, y float32) float32 { return min(x, y) }),
		arithImageOp("max", "The larger of a and b per sample, in a's space; d.max(0) keeps the positive part.",
			graph.Level, nil, math.Inf(-1), func(x, y float32) float32 { return max(x, y) }),
		{
			Name: "abs", On: graph.KindImage, Returns: graph.KindImage,
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				out, err := kernels.Unary(ctx, image(in[0]).Buf, func(v float32) float32 { return float32(math.Abs(float64(v))) })
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "|a| per sample: a.sub(b).abs() is the unsigned difference.",
			See: []string{"sub"},
		},
	}
}
