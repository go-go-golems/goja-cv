package ops

import (
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// twoBoxes is a mask of two boxes (10×8 and 4×4 doc px) on the test image.
func twoBoxes(t *testing.T, e *graph.Engine, img *graph.Node) *graph.Node {
	a := call(t, e, "mask.box", img.ID, []any{4, 4, 10, 8})
	b := call(t, e, "mask.box", img.ID, []any{40, 30, 4, 4})
	return call(t, e, "or", a.ID, graph.NodeArg{ID: b.ID})
}

func regionsAt(t *testing.T, e *graph.Engine, n *graph.Node, scale float64) *graph.RegionsValue {
	t.Helper()
	return evalValue(t, e, n, scale).(*graph.RegionsValue)
}

func TestComponentsAndRegionOps(t *testing.T) {
	e, img := newEngine(t)
	comps := call(t, e, "components", twoBoxes(t, e, img).ID)
	rv := regionsAt(t, e, comps, 1)
	if len(rv.Items) != 2 {
		t.Fatalf("%d components", len(rv.Items))
	}
	big, small := rv.Items[0], rv.Items[1]
	if big.Box != [4]float64{4, 4, 10, 8} || big.Area != 80 || big.ID != 1 || small.Box != [4]float64{40, 30, 4, 4} || small.ID != 2 {
		t.Fatalf("components %+v %+v", big, small)
	}
	if math.Abs(big.Centroid[0]-9) > 1e-9 || math.Abs(big.Centroid[1]-8) > 1e-9 {
		t.Fatalf("centroid %v", big.Centroid)
	}
	// identical at any evaluation scale (analysed at scale 1)
	if rv2 := regionsAt(t, e, comps, 0.25); rv2.Items[0].Box != big.Box || len(rv2.Items) != 2 {
		t.Fatalf("components at 0.25 differ: %+v", rv2.Items[0])
	}
	// where with a param: moving it changes the result, recomputing only where
	par, _, err := e.DeclareParam(graph.ParamDef{Name: "minA", Type: "slider", Min: 0, Max: 200, Value: 50.0}, false)
	if err != nil {
		t.Fatal(err)
	}
	w := call(t, e, "where", comps.ID, map[string]any{"area": map[string]any{"gte": graph.NodeArg{ID: par.ID}}})
	cnt := call(t, e, "count", w.ID)
	if got := evalValue(t, e, cnt, 1).(*graph.DataValue).Data; got != 1.0 {
		t.Fatalf("count(area ≥ 50) = %v", got)
	}
	if err := e.Params().Set("minA", 10.0); err != nil {
		t.Fatal(err)
	}
	if got := evalValue(t, e, cnt, 1).(*graph.DataValue).Data; got != 2.0 {
		t.Fatalf("count(area ≥ 10) = %v", got)
	}
	if e.Evaluator().Runs(comps.ID) != 1 || e.Evaluator().Runs(w.ID) != 2 {
		t.Fatalf("runs components=%d where=%d", e.Evaluator().Runs(comps.ID), e.Evaluator().Runs(w.ID))
	}
	// without / only / sortBy / top / shorthand eq
	if n := len(regionsAt(t, e, call(t, e, "without", comps.ID, []any{1}), 1).Items); n != 1 {
		t.Fatalf("without: %d", n)
	}
	if r := regionsAt(t, e, call(t, e, "only", comps.ID, 2), 1); len(r.Items) != 1 || r.Items[0].ID != 2 {
		t.Fatalf("only: %+v", r.Items)
	}
	asc := regionsAt(t, e, call(t, e, "top", call(t, e, "sortBy", comps.ID, "area", map[string]any{"desc": false}).ID, 1), 1)
	if len(asc.Items) != 1 || asc.Items[0].ID != 2 {
		t.Fatalf("sortBy asc + top: %+v", asc.Items)
	}
	if n := len(regionsAt(t, e, call(t, e, "where", comps.ID, map[string]any{"id": 2}), 1).Items); n != 1 {
		t.Fatalf("where id = 2: %d", n)
	}
	// toMask coverage = region area / doc area at any scale
	tm := call(t, e, "coverage", call(t, e, "toMask", comps.ID).ID)
	for _, sc := range []float64{1, 0.5, 0.3} {
		if got := evalValue(t, e, tm, sc).(*graph.DataValue).Data.(float64); math.Abs(got-96.0/(64*48)) > 1e-4 {
			t.Fatalf("toMask coverage at %g: %g", sc, got)
		}
	}
	grown := evalValue(t, e, call(t, e, "coverage", call(t, e, "toMask", comps.ID, map[string]any{"grow": 2}).ID), 1).(*graph.DataValue).Data.(float64)
	if grown <= 96.0/(64*48) {
		t.Fatalf("toMask grow did not grow: %g", grown)
	}
}

func TestRegionPredicateErrors(t *testing.T) {
	e, img := newEngine(t)
	comps := call(t, e, "components", twoBoxes(t, e, img).ID)
	for _, tc := range []struct {
		op   string
		raw  []any
		code string
		hint string
	}{
		{"where", []any{map[string]any{"aera": map[string]any{"gte": 1}}}, graph.EUnknownOption, `Did you mean "area"?`},
		{"where", []any{map[string]any{"score": map[string]any{"gte": 1}}}, graph.EUnknownOption, "score comes from img.detect.specks()."},
		{"where", []any{map[string]any{"centroid": 1}}, graph.ENoField, ""},
		{"where", []any{map[string]any{"area": map[string]any{"gtee": 1}}}, graph.EUnknownOption, `Did you mean "gte"?`},
		{"where", []any{map[string]any{"area": map[string]any{"lt": "big"}}}, graph.EArgType, ""},
		{"where", []any{5}, graph.EArgType, ""},
		{"sortBy", []any{"aera"}, graph.EUnknownOption, `Did you mean "area"?`},
		{"sortBy", []any{"box"}, graph.ENoField, ""},
		{"without", []any{"x"}, graph.EArgType, ""},
		{"filter", []any{3}, graph.EArgType, ""},
	} {
		_, err := e.Call(tc.op, comps.ID, tc.raw)
		ce, ok := graph.AsError(err)
		if !ok || ce.Code != tc.code || (tc.hint != "" && ce.Hint != tc.hint) {
			t.Fatalf("%s %v: %+v", tc.op, tc.raw, err)
		}
	}
	// a field of another op fails at build time, naming the op that adds it
	ce := callErr(t, e, "where", comps.ID, map[string]any{"score": map[string]any{"gte": 0.5}})
	if ce.Code != graph.EUnknownOption || ce.Fix != "img.detect.specks()" || !strings.Contains(ce.Message, "id, x, y, w, h, cx, cy, area, aspect") {
		t.Fatalf("score on components: %+v", ce)
	}
	// removed names carry the literal fix
	if ce := callErr(t, e, "where", comps.ID, map[string]any{"box": 1}); ce.Code != graph.ENoField || !strings.Contains(ce.Fix, "r.x, r.y, r.w, r.h") {
		t.Fatalf("box: %+v", ce)
	}
}
