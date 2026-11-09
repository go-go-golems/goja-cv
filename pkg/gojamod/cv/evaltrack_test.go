package cvmod_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/look"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

func TestLookReturnsGoBackedProvider(t *testing.T) {
	rt := newRuntime(t)
	v, jerr := run(t, rt, `return img.exposure(0.5).look({ label: "after", maxSide: 32 })`)
	if jerr != nil {
		t.Fatalf("look: %+v", jerr)
	}
	l, ok := v.(*look.Look)
	if !ok {
		t.Fatalf("look exported as %T", v)
	}
	if got, want := l.Caption(), "look: after — open(gradient-64x48-16bit.tif) → exposure(0.5) (32×24, 1 image px = 2.0 doc px)"; got != want {
		t.Fatalf("caption %q, want %q", got, want)
	}
	v, jerr = run(t, rt, `return img.look("before")`)
	if jerr != nil || v.(*look.Look).Label() != "before" {
		t.Fatalf("string label: %v %+v", v, jerr)
	}

	for body, code := range map[string]string{
		`return img.stats().look()`:          graph.EWrongKind,
		`return img.look({ lable: "x" })`:    graph.EUnknownOption,
		`return img.look({ maxSide: 4000 })`: graph.EArgRange,
		`return img.look(42)`:                graph.EArgType,
	} {
		if _, jerr := run(t, rt, body); jerr == nil || jerr.Code != code {
			t.Fatalf("%s: got %+v, want %s", body, jerr, code)
		}
	}
}

// Look options are bound like op options: synonyms, did-you-mean and
// redirects with literal fixes (GCV-002 §7.7).
func TestLookOptionsSynonymsAndRedirects(t *testing.T) {
	rt := newRuntime(t)
	for _, tc := range []struct{ body, code, hint, fix string }{
		{`return img.look({ lable: "x" })`, graph.EUnknownOption, `Did you mean "label"?`, `node.look({ label: … })`},
		{`return img.look({ size: 512 })`, graph.EUnknownOption, `Did you mean "maxSide"?`, `node.look({ maxSide: 512 })`},
		{`return img.look({ box: [0, 0, 8, 8] })`, graph.EUnknownOption, "A window is content: crop the node, then look at the crop.", "node.crop([x, y, w, h]).look()"},
		{`return img.look({ stretch: "auto" })`, graph.EUnknownOption, "A stretch is content: put levels({ auto: true }) in the chain where it should apply.", "node.levels({ auto: true }).look()"},
		{`return img.look({ maxSide: 100.5 })`, graph.EArgType, "", "node.look({ maxSide: 100 })"},
		{`return img.look({ magnify: 8 })`, graph.EUnknownOption, `Did you mean "zoom"?`, `node.look({ zoom: 8 })`},
		{`return img.look({ interpolation: "linear" })`, graph.EUnknownOption, `Did you mean "interp"?`, `node.look({ interp: … })`},
		{`return img.look({ zoom: 2, maxSide: 64 })`, graph.EArgType, "", "node.look({ zoom: 2 })"},
		{`return img.look({ zoom: 32 })`, graph.EArgRange, "", ""},
		{`return img.look({ interp: "cubic" })`, graph.EArgType, "", `node.look({ interp: "nearest" })`},
		{`return img.look({ label: img.stats().meanL })`, graph.ENotAwaited, "", "(await img.stats()).meanL"},
		{`return img.look({ zoom: img.stats() })`, graph.EArgType, "A measurement used as an option is awaited first", "{ zoom: (await x).<field> }"},
	} {
		_, jerr := run(t, rt, tc.body)
		if jerr == nil || jerr.Code != tc.code || (tc.hint != "" && !strings.HasPrefix(jerr.Hint, tc.hint)) || jerr.Fix != tc.fix {
			t.Errorf("%s: got %+v, want %s hint %q fix %q", tc.body, jerr, tc.code, tc.hint, tc.fix)
		}
	}
}

func TestEvalTrackingRecordsSitesUsesAndWarnings(t *testing.T) {
	rt := newRuntime(t)
	ctx := context.Background()
	if _, err := rt.Owner.Call(ctx, "begin", func(_ context.Context, vm *goja.Runtime) (any, error) {
		if _, err := vm.RunString(`require("cv")`); err != nil { // the binding exists once the module is loaded
			return nil, err
		}
		if !cvmod.BeginEval(vm, ctx) {
			t.Error("BeginEval: no cv module")
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	// run() puts the body at line 3 of the script.
	_, jerr := run(t, rt, "img.exposure(0.3)\nconst s = await img.stats()\nimg.exposure(5)\nreturn img.gaussian(2)")
	if jerr != nil {
		t.Fatalf("run: %+v", jerr)
	}
	var trace cvmod.EvalTrace
	var opened []graph.NodeID
	var returnedID graph.NodeID
	if _, err := rt.Owner.Call(ctx, "end", func(_ context.Context, vm *goja.Runtime) (any, error) {
		trace = cvmod.EndEval(vm)
		opened = cvmod.Opened(vm)
		v, err := vm.RunString(`require("cv").open(` + "`" + tiffPath + "`" + `).gaussian(2)`)
		if err != nil {
			return nil, err
		}
		returnedID, _ = cvmod.NodeOf(v)
		if _, ok := cvmod.NodeOf(vm.ToValue(map[string]any{"__id": 3})); ok {
			t.Error("NodeOf accepted a plain object")
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 {
		t.Fatalf("opened %v", opened)
	}
	if len(trace.Created) != 4 {
		t.Fatalf("created %+v", trace.Created)
	}
	lines := []int{}
	for _, c := range trace.Created {
		lines = append(lines, c.Site.Line)
		if c.Site.Source != "" {
			t.Fatalf("site source %q, want the RunString script", c.Site.Source)
		}
	}
	if want := []int{3, 4, 5, 6}; !equalInts(lines, want) {
		t.Fatalf("lines %v, want %v", lines, want)
	}
	// Returning a node from an async body resolves the promise with a
	// thenable, so the returned gaussian is evaluated (used) as well.
	if len(trace.Used) != 2 || trace.Used[0] != trace.Created[1].ID || trace.Used[1] != trace.Created[3].ID {
		t.Fatalf("used %v, want the stats and gaussian nodes", trace.Used)
	}
	if len(trace.Warnings) != 1 || trace.Warnings[0].Code != graph.WRange || trace.Warnings[0].Site.Line != 5 {
		t.Fatalf("warnings %+v", trace.Warnings)
	}
	if returnedID != trace.Created[3].ID {
		t.Fatalf("NodeOf = %d, want the hash-consed gaussian node %d", returnedID, trace.Created[3].ID)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
