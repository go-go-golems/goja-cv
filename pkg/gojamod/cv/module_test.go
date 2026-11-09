package cvmod_test

import (
	"context"
	"fmt"
	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"math"
	"os"
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
	if got["kinds"] != "Image,Mask,Regions" || !strings.Contains(got["image"].(string), "gaussian") {
		t.Fatalf("help: %v", got)
	}
	if got["ev"] != "number in [-10, 10], usual [-3, 3], required, positional. stops: values are multiplied by 2^ev" || got["usage"] != "exposure(ev)" {
		t.Fatalf("help exposure: %#v", got)
	}
	// cv.help never throws (agents batch help calls): unknown names return
	// { found: false, didYouMean }.
	miss := mustRun(t, rt, `return cv.help("blur")`).(map[string]any)
	if miss["found"] != false || miss["didYouMean"].([]any)[0] != "gaussian" {
		t.Fatalf("help synonym: %+v", miss)
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
	if got["same"] != true || fmt.Sprint(got["kinds"]) != "[Image Image Value]" || got["str"] != "[Image #3 gaussian(sigma: 2 px)]" {
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
	if summary["kind"] != "Image" || summary["w"] != int64(64) || summary["h"] != int64(48) || summary["space"] != "linear" || summary["encoding"] != "srgb" {
		t.Fatalf("image summary: %#v", summary)
	}

	mustRun(t, rt, `return await img.exposure(0.3).stats()`)
	ex := mustRun(t, rt, `return cv.explain(img.exposure(0.3).gaussian(2))`).(map[string]any)
	lines := ex["graph"].([]any)
	if len(lines) != 3 ||
		!strings.Contains(lines[0].(string), " gaussian(sigma: 2 px)") ||
		!strings.HasSuffix(lines[0].(string), "Image 64×48           enc srgb · space linear") || // never evaluated
		!strings.HasPrefix(lines[1].(string), "  #2 exposure(ev: 0.3)") ||
		!strings.HasSuffix(lines[1].(string), "Image 64×48  cached   enc srgb · space linear") || // measured above
		!strings.HasPrefix(lines[2].(string), "    #1 source gradient-64x48-16bit.tif") {
		t.Fatalf("explain: %q", lines)
	}
	if ex["kind"] != "Image" || ex["op"] != "gaussian" || ex["call"] != "gaussian(sigma: 2 px)" || ex["space"] != "linear" || ex["encoding"] != "srgb" ||
		fmt.Sprint(ex["frame"]) != "[0 0 64 48]" || fmt.Sprint(ex["path"]) != "[open(gradient-64x48-16bit.tif) exposure gaussian]" {
		t.Fatalf("explain fields: %v", ex)
	}
}

// Part 2 §4 "Ops": argument rules.
func TestPart2Section4Ops(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const s = await img.toGray().stats()                                  // options are literals: await first
		const fixed = img.toGray().levels({ black: s.p01, white: s.p99 })
		const after = await fixed.stats()
		return { before: [s.p01, s.p99], after: [after.p01, after.p99] }`).(map[string]any)
	after := got["after"].([]any)
	near(t, "levels p01", after[0], 0, 2e-3)
	near(t, "levels p99", after[1], 1, 2e-3)

	e := mustFail(t, rt, "E_UNKNOWN_OPTION", `return img.levels({ blak: 0.02 })`)
	if strings.Join(e.Valid, ", ") != "black, white, gamma, auto, space" || e.Hint != `Did you mean "black"?` || e.Fix != "levels({ black: 0.02 })" {
		t.Fatalf("unknown option: %+v", e)
	}
	mustFail(t, rt, "E_ARG_TYPE", `return img.exposure("bright")`)
	mustFail(t, rt, "E_ARG_RANGE", `return img.exposure(12)`)
	if e := mustFail(t, rt, "E_ARG_TYPE", `return img.exposure(img)`); e.Fix != "(await img.stats()).p99" {
		t.Fatalf("a node as an option: %+v", e)
	}
	if e := mustFail(t, rt, "E_ARG_TYPE", `return img.levels({ white: img.stats() })`); e.Fix != "{ white: (await img.stats()).p99 }" {
		t.Fatalf("a measurement as an option: %+v", e)
	}
	mustFail(t, rt, "E_ARG_TYPE", `return img.gaussian(r => r * 2)`)

}

func TestDiagnostics(t *testing.T) {
	rt := newRuntime(t)
	for _, tc := range []struct{ body, code, hint, fix string }{
		{`return img.brightness(0.3)`, "E_UNKNOWN_METHOD", `Did you mean "exposure"?`, "img.exposure(…)"},
		{`return img.blur(2)`, "E_UNKNOWN_METHOD", `Did you mean "gaussian"?`, "img.gaussian(…)"},
		{`return img.exposre(1)`, "E_UNKNOWN_METHOD", `Did you mean "exposure"?`, "img.exposure(…)"},
		{`return img.meanL`, "E_UNKNOWN_METHOD", `"meanL" is a field of the Value returned by stats().`, ""},
		{`if (img.stats().meanL > 0.5) return 1`, "E_NOT_AWAITED", "", "(await img.stats()).meanL"},
		{`return img.stats() + 1`, "E_NOT_AWAITED", "Await the node first", "(await img.stats()).p99"},
		{`return img.stats().p99`, "E_NOT_AWAITED", "", "(await img.stats()).p99"},
		{`return img.exposure(1).stats().p99`, "E_NOT_AWAITED", "", "(await node.stats()).p99"},
		{`return img.detect.specks().items`, "E_NOT_AWAITED", "", "(await img.detect.specks()).items"},
		{`return img.detect.specks().heal()`, "E_NOT_AWAITED", "Pass the regions to the image method.", "img.heal(regions, …)"},
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
	// a method of a Regions node still resolves (Regions keep their methods until they become arrays)
	if got := mustRun(t, rt, `return typeof img.detect.specks().count`); got != "function" {
		t.Fatalf("Regions method: %v", got)
	}
	// errors are catchable JS errors with code and name
	got := mustRun(t, rt, `try { img.levels({ blak: 1 }) } catch (e) { return [e.name, e.code, e instanceof Error, String(e).startsWith("E_UNKNOWN_OPTION: levels has no option \"blak\"")] }`).([]any)
	if fmt.Sprint(got) != "[E_UNKNOWN_OPTION E_UNKNOWN_OPTION true true]" {
		t.Fatalf("catch: %v", got)
	}
	// generic machinery does not trigger evaluation or throw
	got = mustRun(t, rt, `const s = img.stats(); return [JSON.stringify({ s }), "then" in s, Object.keys(s).length]`).([]any)
	if got[0] != `{"s":{"node":"[Value #2 stats()]"}}` || got[1] != true || got[2] != int64(0) {
		t.Fatalf("probes: %v", got)
	}
	if engineOf(t, rt).Evaluator().TotalRuns() == 0 {
		// the awaited cases above did evaluate; this is only a sanity check that the engine is shared
		t.Fatal("expected some evaluations")
	}
}

// TestAwaitIsScaleOneAndImagesAreStatic (GCV-004 D8, D18): an awaited Value
// is evaluated at full resolution, also on an image larger than the old
// 1600 px preview; an awaited Image is its static summary and evaluates
// nothing.
func TestAwaitIsScaleOneAndImagesAreStatic(t *testing.T) {
	big := buffer.New(2000, 30, 1, 1)
	for i := range big.Pix {
		big.Pix[i] = float32(i%2000) / 2000
	}
	b, err := testimage.TIFFBytes(big, testimage.TIFFOptions{Bits: 16})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "wide.tif")
	if err := os.WriteFile(p, b, 0o644); err != nil { // #nosec G306 -- test data
		t.Fatal(err)
	}
	rt := newRuntime(t)
	mustRun(t, rt, `return 1`) // loads cv (opens img; evaluates nothing)
	eng := engineOf(t, rt)
	summary := mustRun(t, rt, fmt.Sprintf(`const w = cv.open(%q); return await w.gaussian(4).levels({ black: 0.1, white: 0.9 })`, p)).(map[string]any)
	if eng.Evaluator().TotalRuns() != 0 {
		t.Fatalf("awaiting an Image ran %d ops", eng.Evaluator().TotalRuns())
	}
	if summary["kind"] != "Image" || summary["w"] != int64(2000) || summary["h"] != int64(30) || summary["space"] != "linear" ||
		summary["encoding"] != "srgb" || fmt.Sprint(summary["frame"]) != "[0 0 2000 30]" {
		t.Fatalf("summary from statics: %v", summary)
	}
	got := mustRun(t, rt, fmt.Sprintf(`
		const w = cv.open(%q)
		const s = await w.stats()
		const all = await Promise.all([img.stats(), img.exposure(1).stats(), img.histogram({ bins: 16 }), img.stats({ percentiles: [0.5] })])
		const png = await cv.open(%q).stats()
		return { w: s.w, scale: s.scale, n: all.length, bins: all[2].counts.length, total: all[2].total, pick: Object.keys(all[3].percentiles).join(","), png: png.meanL, tif: all[0].meanL }`, p, pngPath)).(map[string]any)
	if got["w"] != int64(2000) || got["scale"] != int64(1) || got["bins"] != int64(16) || got["total"] != int64(64*48) || got["pick"] != "0.5" {
		t.Fatalf("await: %v", got)
	}
	near(t, "png vs tiff meanL", got["png"], got["tif"].(float64), 5e-3)
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

// TestSplitReturnsNodes: a macro that builds several nodes returns an array
// of node proxies in JavaScript.
func TestSplitReturnsNodes(t *testing.T) {
	rt := newRuntime(t)
	got := mustRun(t, rt, `
		const [r, g, b] = img.split()
		const s = await b.stats()
		return { kinds: [r, g, b].map(n => cv.explain(n).kind).join(","), same: g === img.channel("g"), enc: (await img.chroma()).encoding, p99: s.p99 }`).(map[string]any)
	if got["kinds"] != "Image,Image,Image" || got["same"] != true || got["enc"] != "linear" {
		t.Fatalf("split: %v", got)
	}
}

// TestBareModuleRejectsAs: names exist only in eval sessions.
func TestBareModuleRejectsAs(t *testing.T) {
	rt := newRuntime(t)
	e := mustFail(t, rt, "E_UNKNOWN_OPTION", fmt.Sprintf(`return cv.open(%q, { as: "ref" })`, tiffPath))
	if !strings.Contains(e.Message, "names exist only in eval sessions") || !strings.HasPrefix(e.Fix, "const ref = cv.open(") {
		t.Fatalf("as in a bare module: %+v", e)
	}
}
