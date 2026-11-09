package cvmod_test

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/engine"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/ops"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

var (
	tiffPath, _ = filepath.Abs(filepath.Join("..", "..", "..", "testdata", "gradient-64x48-16bit.tif"))
	pngPath, _  = filepath.Abs(filepath.Join("..", "..", "..", "testdata", "gradient-64x48-8bit.png"))
)

func newRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	factory, err := engine.NewRuntimeFactoryBuilder().UseModuleMiddleware(engine.MiddlewareOnly("cv")).Build()
	if err != nil {
		t.Fatalf("build runtime factory: %v", err)
	}
	return startRuntime(t, factory)
}

func startRuntime(t *testing.T, factory *engine.RuntimeFactory) *engine.Runtime {
	t.Helper()
	rt, err := factory.NewRuntime(engine.WithStartupContext(context.Background()), engine.WithLifetimeContext(context.Background()))
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close(context.Background()) })
	return rt
}

// jsError is a rejected promise's cv error, exported.
type jsError struct {
	Code, Message, Hint, Fix string
	Path, Valid              []string
	Raw                      string
}

func strList(v goja.Value) []string {
	if v == nil || goja.IsUndefined(v) {
		return nil
	}
	var out []string
	for _, e := range v.Export().([]any) {
		out = append(out, fmt.Sprint(e))
	}
	return out
}

func str(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return ""
	}
	return v.String()
}

// run executes body as an async function with cv, P and img bound (img is the
// 16-bit test TIFF), waits for the promise like replsession does, and returns
// the exported result or the cv error.
func run(t *testing.T, rt *engine.Runtime, body string) (any, *jsError) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	src := fmt.Sprintf("(async () => {\nconst cv = require(\"cv\"); const P = cv.P; const img = cv.open(%q);\n%s\n})()", tiffPath, body)
	ret, err := rt.Owner.Call(ctx, "cv.test.run", func(_ context.Context, vm *goja.Runtime) (any, error) {
		v, err := vm.RunString(src)
		if err != nil {
			return nil, err
		}
		return v.Export().(*goja.Promise), nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	promise := ret.(*goja.Promise)
	for {
		type snap struct {
			state goja.PromiseState
			val   any
			err   *jsError
		}
		r, err := rt.Owner.Call(ctx, "cv.test.poll", func(_ context.Context, vm *goja.Runtime) (any, error) {
			s := snap{state: promise.State()}
			switch s.state {
			case goja.PromiseStateFulfilled:
				s.val = promise.Result().Export()
			case goja.PromiseStateRejected:
				res := promise.Result()
				o := res.ToObject(vm)
				s.err = &jsError{Code: str(o.Get("code")), Message: str(o.Get("message")), Hint: str(o.Get("hint")),
					Fix: str(o.Get("fix")), Path: strList(o.Get("path")), Valid: strList(o.Get("valid")), Raw: res.String()}
			case goja.PromiseStatePending:
			}
			return s, nil
		})
		if err != nil {
			t.Fatalf("poll: %v", err)
		}
		s := r.(snap)
		switch s.state {
		case goja.PromiseStateFulfilled:
			return s.val, nil
		case goja.PromiseStateRejected:
			return nil, s.err
		case goja.PromiseStatePending:
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func mustRun(t *testing.T, rt *engine.Runtime, body string) any {
	t.Helper()
	v, e := run(t, rt, body)
	if e != nil {
		t.Fatalf("script failed: %s\n%+v", e.Raw, e)
	}
	return v
}

func mustFail(t *testing.T, rt *engine.Runtime, code, body string) *jsError {
	t.Helper()
	v, e := run(t, rt, body)
	if e == nil {
		t.Fatalf("script succeeded with %v, want %s", v, code)
	}
	if e.Code != code {
		t.Fatalf("got %s (%s), want %s", e.Code, e.Raw, code)
	}
	return e
}

func engineOf(t *testing.T, rt *engine.Runtime) *graph.Engine {
	t.Helper()
	e, ok := cvmod.EngineFor(rt.VM)
	if !ok {
		t.Fatal("no engine for runtime")
	}
	return e
}

func near(t *testing.T, name string, got any, want, tol float64) {
	t.Helper()
	f, ok := got.(float64)
	if !ok {
		if i, ok := got.(int64); ok {
			f = float64(i)
		} else {
			t.Fatalf("%s = %#v, not a number", name, got)
		}
	}
	if math.Abs(f-want) > tol {
		t.Fatalf("%s = %g, want %g ± %g", name, f, want, tol)
	}
}

func TestHelp(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const h = cv.help()
		const e = cv.help("exposure")
		return { kinds: h.kinds.join(","), image: h.ops.Image.join(","), ev: e.exposure.args.ev, usage: e.exposure.usage, version: cv.version }`).(map[string]any)
	if got["kinds"] != "Image,Mask,Regions,Value,Param" || !strings.Contains(got["image"].(string), "gaussian") {
		t.Fatalf("help: %v", got)
	}
	if got["ev"] != "number in [-10, 10], usual [-3, 3], required, positional. stops: linear values are multiplied by 2^ev" || got["usage"] != "exposure(ev)" {
		t.Fatalf("help exposure: %#v", got)
	}
	e := mustFail(t, rt, "E_UNKNOWN_METHOD", `return cv.help("blur")`)
	if e.Fix != `cv.help("gaussian")` {
		t.Fatalf("help synonym: %+v", e)
	}
}

// Part 2 §3 "Nodes and evaluation", verbatim where possible.
func TestPart2Section3NodesAndEvaluation(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const a = img.exposure(0.3)      // Image node: exposure(img, 0.3)
		const b = a.gaussian(2)          // Image node: gaussian(exposure(img, 0.3), 2)
		const s = b.stats()              // Value node: stats(gaussian(...))
		return { same: img.exposure(0.3) === a, kinds: [a.__kind, b.__kind, s.__kind], str: String(b) }`).(map[string]any)
	if got["same"] != true || fmt.Sprint(got["kinds"]) != "[Image Image Value]" || got["str"] != "[Image #3 gaussian(r: 2 px)]" {
		t.Fatalf("§3 graph: %v", got)
	}
	eng := engineOf(t, rt)
	if eng.Evaluator().TotalRuns() != 0 {
		t.Fatal("building nodes ran ops")
	}

	st := mustRun(t, rt, `
		const st = await img.stats()       // evaluated: { meanL: 0.21, p01: 0, p99: 0.71, ... }
		return st`).(map[string]any)
	for _, k := range []string{"meanL", "medianL", "p01", "p99", "clipLow", "clipHigh", "perChannel", "percentiles"} {
		if _, ok := st[k]; !ok {
			t.Fatalf("stats lacks %s: %v", k, st)
		}
	}
	runs := eng.Evaluator().TotalRuns()
	mustRun(t, rt, `const st2 = await img.stats(); return st2`) // same node, served from the cache
	if eng.Evaluator().TotalRuns() != runs {
		t.Fatal("second await recomputed")
	}

	summary := mustRun(t, rt, `return await img.exposure(0.3)`).(map[string]any)
	if summary["kind"] != "Image" || summary["w"] != int64(64) || summary["h"] != int64(48) || summary["scale"] != int64(1) {
		t.Fatalf("image summary: %#v", summary)
	}

	lines := mustRun(t, rt, `return img.exposure(0.3).gaussian(2).explain()`).([]any)
	if len(lines) != 3 ||
		!strings.HasPrefix(lines[0].(string), "#3 gaussian(r: 2 px)") ||
		!strings.HasSuffix(lines[0].(string), "Image 64×48") || // never evaluated
		!strings.HasPrefix(lines[1].(string), "  #2 exposure(ev: 0.3)") ||
		!strings.HasSuffix(lines[1].(string), "Image 64×48  cached") || // awaited above
		!strings.HasPrefix(lines[2].(string), "    #1 source gradient-64x48-16bit.tif") {
		t.Fatalf("explain: %q", lines)
	}
}

// Part 2 §4 "Ops": argument rules.
func TestPart2Section4Ops(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const s = img.toGray().stats()
		const fixed = img.toGray().levels({ black: s.p01, white: s.p99 })   // s.p01 is a Value node: stats().p01
		const before = await s
		const after = await fixed.stats()
		return { p01: s.p01.__kind, before: [before.p01, before.p99], after: [after.p01, after.p99] }`).(map[string]any)
	if got["p01"] != "Value" {
		t.Fatalf("derived field kind %v", got["p01"])
	}
	after := got["after"].([]any)
	near(t, "levels p01", after[0], 0, 2e-3)
	near(t, "levels p99", after[1], 1, 2e-3)

	e := mustFail(t, rt, "E_UNKNOWN_OPTION", `return img.levels({ blak: 0.02 })`)
	if strings.Join(e.Valid, ", ") != "black, white, gamma" || e.Hint != `Did you mean "black"?` || e.Fix != "levels({ black: 0.02 })" {
		t.Fatalf("unknown option: %+v", e)
	}
	mustFail(t, rt, "E_ARG_TYPE", `return img.exposure("bright")`)
	mustFail(t, rt, "E_ARG_RANGE", `return img.exposure(12)`)
	mustFail(t, rt, "E_WRONG_KIND", `return img.exposure(img)`)
	mustFail(t, rt, "E_ARG_TYPE", `return img.gaussian(r => r * 2)`)

	w := mustRun(t, rt, `cv.warnings({ clear: true }); img.exposure(4.2); return cv.warnings()`).([]any)
	if len(w) != 1 || w[0].(map[string]any)["code"] != "RANGE" ||
		w[0].(map[string]any)["message"] != "exposure.ev = 4.2 is outside the usual range [-3, 3]." {
		t.Fatalf("RANGE warning: %v", w)
	}
}

func TestDiagnostics(t *testing.T) {
	rt := newRuntime(t)
	for _, tc := range []struct{ body, code, hint, fix string }{
		{`return img.brightness(0.3)`, "E_UNKNOWN_METHOD", `Did you mean "exposure"?`, "img.exposure(…)"},
		{`return img.blur(2)`, "E_UNKNOWN_METHOD", `Did you mean "gaussian"?`, "img.gaussian(…)"},
		{`return img.exposre(1)`, "E_UNKNOWN_METHOD", `Did you mean "exposure"?`, "img.exposure(…)"},
		{`return img.meanL`, "E_UNKNOWN_METHOD", `"meanL" is a field of the Value returned by stats().`, ""},
		{`return img.percentile(0.5)`, "E_UNKNOWN_METHOD", `"percentile" is a Value method; measure the image first, e.g. img.stats()`, ""},
		{`if (img.stats().meanL > 0.5) return 1`, "E_LAZY_COMPARE", "Await the node to get its value.", ""},
		{`return img.stats().meanL + 1`, "E_LAZY_COMPARE", "", ""},
		{`img.ev = 0.3`, "E_IMMUTABLE", "", ""},
		{`img.exposure = 1`, "E_IMMUTABLE", "", "const next = node.exposure(…)"},
		{`delete img.foo`, "E_IMMUTABLE", "", ""},
	} {
		e := mustFail(t, rt, tc.code, tc.body)
		if (tc.hint != "" && !strings.HasPrefix(e.Hint, tc.hint)) || (tc.fix != "" && !strings.HasPrefix(e.Fix, tc.fix)) {
			t.Fatalf("%s: %+v", tc.body, e)
		}
	}
	e := mustFail(t, rt, "E_UNKNOWN_METHOD", `return img.exposure(0.3).frobnicate(3)`)
	if strings.Join(e.Path, " → ") != "open(gradient-64x48-16bit.tif) → exposure → ✗ frobnicate" {
		t.Fatalf("path: %v", e.Path)
	}
	// evaluation errors reject the awaited promise with the node path
	e = mustFail(t, rt, "E_ARG_RANGE", `return await img.levels({ black: 0.5, white: 0.2 }).stats()`)
	if strings.Join(e.Path, " → ") != "open(gradient-64x48-16bit.tif) → ✗ levels" {
		t.Fatalf("eval path: %v", e.Path)
	}
	e = mustFail(t, rt, "E_NO_FIELD", `return await img.stats().meanl`)
	if e.Hint != `Did you mean "meanL"?` {
		t.Fatalf("no field: %+v", e)
	}
	// errors are catchable JS errors with code and name
	got := mustRun(t, rt, `try { img.levels({ blak: 1 }) } catch (e) { return [e.name, e.code, e instanceof Error, String(e).startsWith("E_UNKNOWN_OPTION: levels has no option \"blak\"")] }`).([]any)
	if fmt.Sprint(got) != "[E_UNKNOWN_OPTION E_UNKNOWN_OPTION true true]" {
		t.Fatalf("catch: %v", got)
	}
	// generic machinery does not trigger evaluation or throw
	got = mustRun(t, rt, `const s = img.stats(); return [JSON.stringify({ s }), "then" in s, Object.keys(s).length, cv.isNode(s), cv.isNode({})]`).([]any)
	if got[0] != `{"s":{"node":"[Value #2 stats()]"}}` || got[1] != true || got[2] != int64(0) || got[3] != true || got[4] != false {
		t.Fatalf("probes: %v", got)
	}
	if engineOf(t, rt).Evaluator().TotalRuns() == 0 {
		// the awaited cases above did evaluate; this is only a sanity check that the engine is shared
		t.Fatal("expected some evaluations")
	}
}

func TestParamsRecomputeOnlyDownstream(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const ev = P.slider("ev", { min: -2, max: 2, step: 0.05, value: 0.3, label: "Exposure", unit: "EV" })
		const m = img.exposure(ev).stats().meanL
		const base = await img.stats().meanL
		const a = await m
		P.set("ev", 1)
		const b = await m
		return { base, a, b, value: ev.value, touched: ev.touched, again: P.slider("ev", { min: -2, max: 2, value: 0 }).value }`).(map[string]any)
	base := got["base"].(float64)
	near(t, "meanL at 0.3 EV", got["a"], base*math.Exp2(0.3), 1e-6)
	near(t, "meanL at 1 EV", got["b"], base*2, 1e-6)
	if got["value"] != int64(1) || got["touched"] != true || got["again"] != int64(1) {
		t.Fatalf("param state: %v", got)
	}
	eng := engineOf(t, rt)
	img, _ := eng.Open(tiffPath)
	if eng.Evaluator().Runs(img.ID) != 1 {
		t.Fatalf("source decoded/resampled %d times", eng.Evaluator().Runs(img.ID))
	}
	mustFail(t, rt, "E_ARG_RANGE", `P.slider("x", { min: -2, max: 2, value: 3 })`)
	mustFail(t, rt, "E_PARAM_REDECLARE", `P.slider("ev", { min: -5, max: 5 })`)
	mustFail(t, rt, "E_UNKNOWN_OPTION", `P.slider("y", { min: 0, max: 1, valeu: 0.5 })`)
	mustFail(t, rt, "E_ARG_RANGE", `P.set("ev", 9)`)
}

func TestEvaluateScalesAndConcurrency(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const g = img.gaussian(4)
		const half = await cv.evaluate(g, { scale: 0.5 })
		const full = await cv.evaluate(g, { scale: "full" })
		const all = await Promise.all([img.stats(), img.exposure(1).stats(), img.histogram({ bins: 16 }), img.stats().pick("meanL", "p99")])
		const png = await cv.open(`+fmt.Sprintf("%q", pngPath)+`).stats()
		return { half: [half.w, half.h, half.scale], full: full.w, n: all.length, bins: all[2].counts.length, total: all[2].total, pick: Object.keys(all[3]).join(","), png: png.meanL, tif: all[0].meanL }`).(map[string]any)
	if fmt.Sprint(got["half"]) != "[32 24 0.5]" || got["full"] != int64(64) || got["bins"] != int64(16) || got["total"] != int64(64*48) || got["pick"] != "meanL,p99" {
		t.Fatalf("evaluate: %v", got)
	}
	near(t, "png vs tiff meanL", got["png"], got["tif"].(float64), 5e-3)
	mustFail(t, rt, "E_ARG_RANGE", `return cv.evaluate(img, { scale: 2 })`)
}

// TestRegistrarAndNamespaces boots a runtime through the runtime-aware
// Registrar with a registry that has a namespaced op, as a session would.
func TestRegistrarAndNamespaces(t *testing.T) {
	reg := ops.NewRegistry()
	reg.MustRegister(graph.OpDef{Name: "info.size", On: graph.KindImage, Returns: graph.KindValue,
		Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
			b := in[0].(*graph.ImageValue).Buf
			return &graph.DataValue{Data: map[string]any{"w": float64(b.W), "h": float64(b.H)}}, nil
		}})
	factory, err := engine.NewRuntimeFactoryBuilder().
		WithModules(&cvmod.Registrar{NewEngine: func() *graph.Engine { return graph.NewEngine(reg, graph.EngineOptions{}) }}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	rt := startRuntime(t, factory)
	if v, ok := rt.Value(cvmod.EngineValueKey); !ok || v.(*graph.Engine).Registry() != reg {
		t.Fatal("session engine not in runtime values")
	}
	got := mustRun(t, rt, `return await img.info.size()`).(map[string]any)
	if got["w"] != int64(64) {
		t.Fatalf("namespaced op: %v", got)
	}
	e := mustFail(t, rt, "E_NAMESPACE", `return img.size()`)
	if e.Fix != "img.info.size(…)" {
		t.Fatalf("namespace fix: %+v", e)
	}
	mustFail(t, rt, "E_UNKNOWN_METHOD", `return img.info.sise()`)
	if eng, _ := cvmod.EngineFor(rt.VM); eng.Registry() != reg {
		t.Fatal("EngineFor returned another engine")
	}
}
