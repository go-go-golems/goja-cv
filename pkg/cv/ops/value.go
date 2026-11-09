package ops

import (
	"context"
	"math"
	"sort"
	"strconv"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// fieldOf reads field name of d: a map key or an array index.
func fieldOf(d any, name string) (any, error) {
	switch x := d.(type) {
	case map[string]any:
		v, ok := x[name]
		if !ok {
			return nil, graph.NoField(name, keysOf(x))
		}
		return v, nil
	case []any:
		i, err := strconv.Atoi(name)
		if name == "length" {
			return float64(len(x)), nil
		}
		if err != nil || i < 0 || i >= len(x) {
			return nil, graph.Errorf(graph.ENoField, "The value is a list of %d; %q is not an index.", len(x), name)
		}
		return x[i], nil
	}
	return nil, graph.Errorf(graph.ENoField, "The value is %v, which has no field %q.", d, name)
}

func receiverNumber(op string, v graph.Value) (float64, error) {
	f, err := v.(*graph.DataValue).Number()
	if err != nil {
		return 0, graph.Errorf(graph.EArgType, "%s needs a number; the receiver evaluated to %v.", op, v.(*graph.DataValue).Data)
	}
	return f, nil
}

func arith(name, doc string, f func(a, b float64) (float64, error)) graph.OpDef {
	return graph.OpDef{
		Name: name, On: graph.KindValue, Returns: graph.KindValue,
		Args: []graph.ArgSpec{{Name: "x", Type: "number", Positional: true}},
		Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			v, err := receiverNumber(name, in[0])
			if err != nil {
				return nil, err
			}
			r, err := f(v, a.Number("x"))
			if err != nil {
				return nil, err
			}
			return data(r), nil
		},
		Doc: doc,
	}
}

func valueOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "field", On: graph.KindValue, Returns: graph.KindValue,
			Args: []graph.ArgSpec{{Name: "name", Type: "string", Positional: true}},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				v, err := fieldOf(in[0].(*graph.DataValue).Data, a.String("name"))
				if err != nil {
					return nil, err
				}
				return data(v), nil
			},
			Doc: "A derived Value: one field of the receiver. value.meanL is shorthand for value.field(\"meanL\").",
			See: []string{"pick"},
		},
		{
			Name: "pick", On: graph.KindValue, Returns: graph.KindValue,
			Args: []graph.ArgSpec{{Name: "keys", Type: "string[]", Positional: true, Variadic: true}},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				out := map[string]any{}
				for _, k := range a.Strings("keys") {
					v, err := fieldOf(in[0].(*graph.DataValue).Data, k)
					if err != nil {
						return nil, err
					}
					out[k] = v
				}
				return data(out), nil
			},
			Doc: "A derived Value with only the given fields.",
			See: []string{"field"},
		},
		{
			Name: "percentile", On: graph.KindValue, Returns: graph.KindValue,
			Args: []graph.ArgSpec{{Name: "p", Type: "number", Positional: true, Min: graph.F(0), Max: graph.F(1)}},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				p := a.Number("p")
				m, _ := in[0].(*graph.DataValue).Data.(map[string]any)
				if q, ok := m["percentiles"].(map[string]any); ok {
					if v, ok := q[PercentileKey(p)]; ok {
						return data(v), nil
					}
				}
				alias := map[float64]string{0.01: "p01", 0.5: "medianL", 0.99: "p99"}
				if k, ok := alias[p]; ok {
					if v, ok := m[k]; ok {
						return data(v), nil
					}
				}
				return nil, graph.Errorf(graph.ENoField, "Percentile %s was not computed.", PercentileKey(p)).
					WithFix("img.stats({ percentiles: [" + PercentileKey(p) + "] }).percentile(" + PercentileKey(p) + ")")
			},
			Doc: "A percentile of a stats() result (must be requested with stats({ percentiles }) unless it is 0.01, 0.5 or 0.99).",
			See: []string{"stats"},
		},
		arith("add", "receiver + x", func(a, b float64) (float64, error) { return a + b, nil }),
		arith("sub", "receiver − x", func(a, b float64) (float64, error) { return a - b, nil }),
		arith("mul", "receiver × x", func(a, b float64) (float64, error) { return a * b, nil }),
		arith("div", "receiver ÷ x", func(a, b float64) (float64, error) {
			if b == 0 {
				return 0, graph.Errorf(graph.EArgRange, "div: division by zero.")
			}
			return a / b, nil
		}),
		arith("min", "the smaller of receiver and x", func(a, b float64) (float64, error) { return math.Min(a, b), nil }),
		arith("max", "the larger of receiver and x", func(a, b float64) (float64, error) { return math.Max(a, b), nil }),
		arith("distance", "|receiver − x|", func(a, b float64) (float64, error) { return math.Abs(a - b), nil }),
		{
			Name: "abs", On: graph.KindValue, Returns: graph.KindValue,
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				v, err := receiverNumber("abs", in[0])
				if err != nil {
					return nil, err
				}
				return data(math.Abs(v)), nil
			},
			Doc: "|receiver|",
		},
	}
}
