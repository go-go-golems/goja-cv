package graph

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// testRegistry is a minimal op table: a constant-image source, a multiply,
// a doc-px op, a reducer, a two-input mix, field access and a param.
func testRegistry(t *testing.T) (*Registry, *atomic.Int64) {
	t.Helper()
	var slow atomic.Int64
	r := NewRegistry()
	r.MustRegister(
		OpDef{Name: "fill", On: KindNone, Returns: KindImage,
			Args: []ArgSpec{
				{Name: "v", Type: "number", Positional: true},
				{Name: "w", Type: "number", Default: 40.0},
				{Name: "h", Type: "number", Default: 20.0},
			},
			DocSize: func(a Args, _ []*Node) (int, int) { return a.Int("w"), a.Int("h") },
			Run: func(_ context.Context, ec *EvalCtx, _ []Value, a Args) (Value, error) {
				w, h := buffer.ScaledSize(a.Int("w"), a.Int("h"), ec.Scale)
				b := buffer.New(w, h, 1, ec.Scale)
				for i := range b.Pix {
					b.Pix[i] = float32(a.Number("v"))
				}
				return NewImage(b, a.Int("w"), a.Int("h")), nil
			}},
		OpDef{Name: "scale", On: KindImage, Returns: KindImage,
			Args:     []ArgSpec{{Name: "k", Type: "number", Positional: true, Min: F(-100), Max: F(100), Usual: [2]float64{0, 4}}},
			Identity: func(a Args) bool { return a.Number("k") == 1 },
			Run: func(_ context.Context, _ *EvalCtx, in []Value, a Args) (Value, error) {
				src := in[0].(*ImageValue)
				out := buffer.NewLike(src.Buf)
				for i, v := range src.Buf.Pix {
					out.Pix[i] = v * float32(a.Number("k"))
				}
				return NewImage(out, src.DocW, src.DocH), nil
			}},
		OpDef{Name: "radius", On: KindImage, Returns: KindValue,
			Args: []ArgSpec{{Name: "r", Type: "number", Unit: DocPx, Positional: true}, {Name: "bad", Type: "bool", Default: false}},
			Run: func(_ context.Context, ec *EvalCtx, in []Value, a Args) (Value, error) {
				if a.Bool("bad") {
					return &DataValue{Data: a.Number("r")}, nil // must panic: DocPx through Number
				}
				return &DataValue{Data: a.Length("r", in[0].(*ImageValue).Buf.Scale)}, nil
			}},
		OpDef{Name: "sum", On: KindImage, Returns: KindValue, Fields: []string{"sum", "mean"},
			Args: []ArgSpec{{Name: "delay", Type: "number", Default: 0.0}},
			Run: func(_ context.Context, _ *EvalCtx, in []Value, a Args) (Value, error) {
				if d := a.Number("delay"); d > 0 {
					slow.Add(1)
					time.Sleep(time.Duration(d) * time.Millisecond)
				}
				b := in[0].(*ImageValue).Buf
				var s float64
				for _, v := range b.Pix {
					s += float64(v)
				}
				return &DataValue{Data: map[string]any{"sum": s, "mean": s / float64(len(b.Pix))}}, nil
			}},
		OpDef{Name: "mix", On: KindImage, Inputs: []Kind{KindImage}, Returns: KindImage,
			Args: []ArgSpec{{Name: "opts", Type: "object", Default: map[string]any{}, Positional: true,
				Options: []ArgSpec{{Name: "w", Type: "number", Default: 0.5}}}},
			Run: func(_ context.Context, _ *EvalCtx, in []Value, a Args) (Value, error) {
				x, y := in[0].(*ImageValue), in[1].(*ImageValue)
				wt := float32(a.Object("opts")["w"].(float64))
				out := buffer.NewLike(x.Buf)
				for i := range out.Pix {
					out.Pix[i] = (1-wt)*x.Buf.Pix[i] + wt*y.Buf.Pix[i]
				}
				return NewImage(out, x.DocW, x.DocH), nil
			}},
		OpDef{Name: "fail", On: KindImage, Returns: KindImage,
			Run: func(context.Context, *EvalCtx, []Value, Args) (Value, error) {
				return nil, errFail
			}},
		OpDef{Name: "field", On: KindValue, Returns: KindValue,
			Args: []ArgSpec{{Name: "name", Type: "string", Positional: true}},
			Run: func(_ context.Context, _ *EvalCtx, in []Value, a Args) (Value, error) {
				m, _ := in[0].(*DataValue).Data.(map[string]any)
				v, ok := m[a.String("name")]
				if !ok {
					return nil, NoField(a.String("name"), sortedKeys(m))
				}
				return &DataValue{Data: v}, nil
			}},
		OpDef{Name: "param", On: KindNone, Returns: KindParam,
			Args: []ArgSpec{{Name: "name", Type: "string", Positional: true}},
			Run: func(_ context.Context, ec *EvalCtx, _ []Value, a Args) (Value, error) {
				return &DataValue{Data: ec.Params[a.String("name")]}, nil
			}},
		OpDef{Name: "mask.lum", On: KindImage, Returns: KindImage,
			Run: func(_ context.Context, _ *EvalCtx, in []Value, _ Args) (Value, error) { return in[0], nil }},
	)
	return r, &slow
}

var errFail = Errorf(EEval, "kernel exploded")

func newTestEngine(t *testing.T) (*Engine, *atomic.Int64) {
	r, slow := testRegistry(t)
	return NewEngine(r, EngineOptions{}), slow
}

func mustCall(t *testing.T, e *Engine, name string, recv NodeID, raw ...any) *Node {
	t.Helper()
	n, _, err := e.Call(name, recv, raw)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return n
}

func codeOf(t *testing.T, err error) *Error {
	t.Helper()
	ce, ok := AsError(err)
	if !ok {
		t.Fatalf("not a cv error: %v", err)
	}
	return ce
}

func TestHashConsingAndCanonicalArgs(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 0.5)
	a := mustCall(t, e, "scale", src.ID, 2)
	b := mustCall(t, e, "scale", src.ID, map[string]any{"k": int64(2)})
	if a != b {
		t.Fatalf("scale(2) and scale({k: 2}) differ: #%d #%d", a.ID, b.ID)
	}
	src2 := mustCall(t, e, "fill", 0, 0.5, map[string]any{"w": 40.0, "h": 20})
	if src2 != src {
		t.Fatal("defaults are not filled before hashing")
	}
	n := e.Builder().Len()
	mustCall(t, e, "scale", src.ID, 2.0)
	if e.Builder().Len() != n {
		t.Fatal("re-interning created a node")
	}
	if c := mustCall(t, e, "scale", src.ID, 3); c == a {
		t.Fatal("different args must give a different node")
	}
}

func TestArgumentErrorsAndWarnings(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 1)

	_, _, err := e.Call("mix", src.ID, []any{NodeArg{src.ID}, map[string]any{"wt": 0.3}})
	ce := codeOf(t, err)
	if ce.Code != EUnknownOption || ce.Hint != `Did you mean "w"?` || strings.Join(ce.Valid, ",") != "w" {
		t.Fatalf("nested unknown option: %+v", ce)
	}
	_, _, err = e.Call("scale", src.ID, []any{map[string]any{"kk": 2}})
	if ce = codeOf(t, err); ce.Code != EUnknownOption || ce.Fix != "scale({ k: 2 })" {
		t.Fatalf("unknown option: %+v", ce)
	}
	_, _, err = e.Call("scale", src.ID, []any{500})
	if ce = codeOf(t, err); ce.Code != EArgRange {
		t.Fatalf("hard range: %+v", ce)
	}
	_, _, err = e.Call("scale", src.ID, []any{"two"})
	if ce = codeOf(t, err); ce.Code != EArgType {
		t.Fatalf("type: %+v", ce)
	}
	_, _, err = e.Call("scale", src.ID, nil)
	if ce = codeOf(t, err); ce.Code != EArgType || !strings.Contains(ce.Hint, "scale(k)") {
		t.Fatalf("missing: %+v", ce)
	}
	_, w, err := e.Call("scale", src.ID, []any{7})
	if err != nil || len(w) != 1 || w[0].Code != WRange {
		t.Fatalf("usual range: %v %v", w, err)
	}
	_, _, err = e.Call("scale", src.ID, []any{NodeArg{src.ID}})
	if ce = codeOf(t, err); ce.Code != EWrongKind {
		t.Fatalf("image as number: %+v", ce)
	}
	_, _, err = e.Call("mix", src.ID, []any{2})
	if ce = codeOf(t, err); ce.Code != EWrongKind {
		t.Fatalf("number as image input: %+v", ce)
	}
}

func TestUnknownMethodSuggestions(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 1)
	for _, tc := range []struct{ name, code, hint string }{
		{"scael", EUnknownMethod, `Did you mean "scale"?`},
		{"lum", ENamespace, ""},
		{"field", EUnknownMethod, `"field" is a Value method`},
		{"mean", EUnknownMethod, `"mean" is a field of the Value returned by sum()`},
	} {
		_, _, err := e.Call(tc.name, src.ID, nil)
		ce := codeOf(t, err)
		if ce.Code != tc.code || !strings.HasPrefix(ce.Hint, tc.hint) {
			t.Fatalf("%s: %+v", tc.name, ce)
		}
		if ce.Path[len(ce.Path)-1] != "✗ "+tc.name {
			t.Fatalf("%s: path %v", tc.name, ce.Path)
		}
	}
	_, _, err := e.Call("lum", src.ID, nil)
	if ce := codeOf(t, err); ce.Fix != "img.mask.lum(…)" {
		t.Fatalf("namespace fix: %q", ce.Fix)
	}
}

func evalNum(t *testing.T, e *Engine, id NodeID) float64 {
	t.Helper()
	v, err := e.Evaluate(context.Background(), id, EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	f, err := v.(*DataValue).Number()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestLiftingIncludingOptionObjects(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 0.5)
	mean := mustCall(t, e, "field", mustCall(t, e, "sum", src.ID).ID, "mean")
	scaled := mustCall(t, e, "scale", src.ID, NodeArg{mean.ID}) // 0.5 * 0.5
	if len(scaled.Inputs) != 2 || scaled.Inputs[1] != mean.ID || scaled.NExplicit != 1 {
		t.Fatalf("lifted inputs: %+v", scaled.Inputs)
	}
	got := evalNum(t, e, mustCall(t, e, "field", mustCall(t, e, "sum", scaled.ID).ID, "mean").ID)
	if got != 0.25 {
		t.Fatalf("lifted scale: %v", got)
	}
	other := mustCall(t, e, "fill", 0, 1.0)
	mixed := mustCall(t, e, "mix", src.ID, NodeArg{other.ID}, map[string]any{"w": NodeArg{mean.ID}}) // 0.5*0.5+0.5*1
	if len(mixed.Inputs) != 3 || mixed.NExplicit != 2 {
		t.Fatalf("mix inputs %v", mixed.Inputs)
	}
	got = evalNum(t, e, mustCall(t, e, "field", mustCall(t, e, "sum", mixed.ID).ID, "mean").ID)
	if got != 0.75 {
		t.Fatalf("lifted option: %v", got)
	}
	// a lifted value of the wrong type fails at evaluation with a path
	strParam, _, err := e.DeclareParam(ParamDef{Name: "mode", Type: "choice", Options: []string{"a", "b"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	bad := mustCall(t, e, "scale", src.ID, NodeArg{strParam.ID})
	_, err = e.Evaluate(context.Background(), bad.ID, EvalOptions{Scale: 1})
	if ce := codeOf(t, err); ce.Code != EArgType || ce.Path[len(ce.Path)-1] != "✗ scale" {
		t.Fatalf("lifted type error: %+v", ce)
	}
}

func TestParamMovesRecomputeOnlyDownstream(t *testing.T) {
	e, _ := newTestEngine(t)
	p, _, err := e.DeclareParam(ParamDef{Name: "k", Type: "slider", Min: 0, Max: 10, Value: 2.0}, false)
	if err != nil {
		t.Fatal(err)
	}
	src := mustCall(t, e, "fill", 0, 1.0)
	a := mustCall(t, e, "scale", src.ID, 3)
	viaParam := mustCall(t, e, "field", mustCall(t, e, "sum", mustCall(t, e, "scale", a.ID, NodeArg{p.ID}).ID).ID, "mean")
	independent := mustCall(t, e, "field", mustCall(t, e, "sum", mustCall(t, e, "scale", a.ID, 4).ID).ID, "mean")
	if evalNum(t, e, viaParam.ID) != 6 || evalNum(t, e, independent.ID) != 12 {
		t.Fatal("bad values")
	}
	before := map[NodeID]int{src.ID: 1, a.ID: 1}
	for id, want := range before {
		if got := e.Evaluator().Runs(id); got != want {
			t.Fatalf("#%d ran %d times", id, got)
		}
	}
	total := e.Evaluator().TotalRuns()
	// unchanged params: everything is cached
	evalNum(t, e, viaParam.ID)
	evalNum(t, e, independent.ID)
	if e.Evaluator().TotalRuns() != total {
		t.Fatal("re-evaluation without changes ran ops")
	}
	if err := e.Params().Set("k", 5.0); err != nil {
		t.Fatal(err)
	}
	if got := evalNum(t, e, viaParam.ID); got != 15 {
		t.Fatalf("after move: %v", got)
	}
	evalNum(t, e, independent.ID)
	// param, scale(k), sum, field = 4 runs; src and a stay cached
	if d := e.Evaluator().TotalRuns() - total; d != 4 {
		t.Fatalf("moving the param ran %d ops, want 4", d)
	}
	if e.Evaluator().Runs(src.ID) != 1 || e.Evaluator().Runs(a.ID) != 1 {
		t.Fatal("upstream nodes recomputed")
	}
	if err := e.Params().Set("k", 50.0); err == nil {
		t.Fatal("out-of-range param value accepted")
	}
	if _, _, err := e.DeclareParam(ParamDef{Name: "k", Type: "slider", Min: 0, Max: 20, Value: 1.0}, false); codeOf(t, err).Code != EParamRedeclare {
		t.Fatal("redeclare with a different range accepted")
	}
	_, d, err := e.DeclareParam(ParamDef{Name: "k", Type: "slider", Min: 0, Max: 10, Value: 1.0}, false)
	if err != nil || d.Value != 5.0 || !d.Touched {
		t.Fatalf("person's value must win on redeclare: %+v %v", d, err)
	}
}

func TestIdentityShortCircuit(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 1.0)
	id := mustCall(t, e, "scale", src.ID, 1)
	a, _ := e.Evaluate(context.Background(), src.ID, EvalOptions{Scale: 1})
	b, err := e.Evaluate(context.Background(), id.ID, EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	if a.(*ImageValue).Buf != b.(*ImageValue).Buf {
		t.Fatal("identity op copied its input")
	}
	if e.Evaluator().Runs(id.ID) != 0 {
		t.Fatal("identity op ran")
	}
}

func TestSingleflightSharesConcurrentWork(t *testing.T) {
	e, slow := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 1.0)
	s := mustCall(t, e, "sum", src.ID, map[string]any{"delay": 50})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.Evaluate(context.Background(), s.ID, EvalOptions{Scale: 1}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if slow.Load() != 1 || e.Evaluator().Runs(s.ID) != 1 {
		t.Fatalf("slow op ran %d times", slow.Load())
	}
}

func TestByteCappedLRU(t *testing.T) {
	r, _ := testRegistry(t)
	// 40×20×1 float32 = 3200 bytes per image; cap fits two
	e := NewEngine(r, EngineOptions{CacheBytes: 7000})
	src := mustCall(t, e, "fill", 0, 1.0)
	a := mustCall(t, e, "scale", src.ID, 2)
	b := mustCall(t, e, "scale", src.ID, 3)
	c := mustCall(t, e, "scale", src.ID, 4)
	for _, n := range []*Node{a, b, c} {
		if _, err := e.Evaluate(context.Background(), n.ID, EvalOptions{Scale: 1}); err != nil {
			t.Fatal(err)
		}
	}
	st := e.Cache().Stats()
	if st.Bytes > 7000 || st.Evictions == 0 {
		t.Fatalf("cache stats %+v", st)
	}
	if _, err := e.Evaluate(context.Background(), a.ID, EvalOptions{Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if e.Evaluator().Runs(a.ID) != 2 {
		t.Fatalf("evicted node should recompute, ran %d", e.Evaluator().Runs(a.ID))
	}
}

func TestErrorsCarryThePath(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 1.0)
	f := mustCall(t, e, "fail", mustCall(t, e, "scale", src.ID, 2).ID)
	top := mustCall(t, e, "scale", f.ID, 3)
	_, err := e.Evaluate(context.Background(), top.ID, EvalOptions{Scale: 1})
	ce := codeOf(t, err)
	if strings.Join(ce.Path, " → ") != "fill → scale → ✗ fail" || ce.Code != EEval {
		t.Fatalf("path %+v", ce)
	}
	bad := mustCall(t, e, "radius", src.ID, 4, map[string]any{"bad": true})
	_, err = e.Evaluate(context.Background(), bad.ID, EvalOptions{Scale: 1})
	if ce := codeOf(t, err); ce.Code != EInternal || !strings.Contains(ce.Message, "Args.Length") {
		t.Fatalf("DocPx through Number must fail loudly: %+v", ce)
	}
}

func TestDocPxScalingAndScaleQuantization(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 1.0, map[string]any{"w": 4000, "h": 2000})
	r := mustCall(t, e, "radius", src.ID, 12)
	v, err := e.Evaluate(context.Background(), r.ID, EvalOptions{Scale: 0.25})
	if err != nil || v.(*DataValue).Data != 3.0 {
		t.Fatalf("radius at 0.25 = %v %v", v, err)
	}
	if s := e.PreviewScale(src.ID); s != 0.4 {
		t.Fatalf("preview scale %v", s)
	}
	if e.Evaluator().Signature(src.ID, 0.25) != e.Evaluator().Signature(src.ID, 0.250001) {
		t.Fatal("scale not quantized")
	}
	if e.Evaluator().Signature(src.ID, 0.25) == e.Evaluator().Signature(src.ID, 0.5) {
		t.Fatal("scale not in signature")
	}
}

func TestExplain(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 1.0, map[string]any{"w": 400, "h": 200})
	a := mustCall(t, e, "scale", src.ID, 2)
	r := mustCall(t, e, "radius", a.ID, 8)
	if _, err := e.Evaluate(context.Background(), a.ID, EvalOptions{Scale: 0.5}); err != nil {
		t.Fatal(err)
	}
	lines := e.Explain(r.ID, 0.5)
	want := []string{"#3 radius(r: 8 px → 4 at scale 0.5)", "  #2 scale(k: 2)", "    #1 fill(v: 1, w: 400, h: 200)"}
	if len(lines) != 3 {
		t.Fatalf("lines %q", lines)
	}
	for i, w := range want {
		if !strings.HasPrefix(lines[i], w) {
			t.Fatalf("line %d = %q, want prefix %q", i, lines[i], w)
		}
	}
	if !strings.HasSuffix(lines[1], "Image 200×100  cached") || strings.HasSuffix(lines[0], "cached") {
		t.Fatalf("cache markers: %q", lines)
	}
}

func TestHelp(t *testing.T) {
	r, _ := testRegistry(t)
	h, err := r.Help("scale")
	if err != nil {
		t.Fatal(err)
	}
	entry := h["scale"].(map[string]any)
	if entry["usage"] != "scale(k)" || !strings.Contains(entry["args"].(map[string]any)["k"].(string), "usual [0, 4]") {
		t.Fatalf("help: %#v", entry)
	}
	if _, err := r.Help("scael"); codeOf(t, err).Hint != `Did you mean "scale"?` {
		t.Fatal("help did-you-mean")
	}
}
