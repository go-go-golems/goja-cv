package ops

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// schemaNodes are the receivers of the schema walk (on the planted scan).
type schemaNodes struct {
	img, mask, specks *graph.Node
}

// schemaCalls builds one node per op that returns Regions or a Value table
// or object (receiver, raw args). Every such op must be listed here or in
// schemaScalar, so a new op cannot skip the property.
var schemaCalls = map[string]func(s schemaNodes) (graph.NodeID, []any){
	"Image:detect.specks": func(s schemaNodes) (graph.NodeID, []any) { return s.img.ID, nil },
	"Image:check.dust":    func(s schemaNodes) (graph.NodeID, []any) { return s.img.ID, nil },
	"Image:stats":         func(s schemaNodes) (graph.NodeID, []any) { return s.img.ID, nil },
	"Image:histogram":     func(s schemaNodes) (graph.NodeID, []any) { return s.img.ID, []any{map[string]any{"bins": 16}} },
	"Mask:components":     func(s schemaNodes) (graph.NodeID, []any) { return s.mask.ID, nil },
	"Image:sample": func(s schemaNodes) (graph.NodeID, []any) {
		return s.img.ID, []any{[]any{[]any{10.5, 10.5}, []any{-5.0, 3.0}, map[string]any{"cx": 100.0, "cy": 50.0}}, map[string]any{"radius": 2}}
	},
	"Image:measure": func(s schemaNodes) (graph.NodeID, []any) {
		return s.img.ID, []any{[]any{[]any{200.5, 46.5}, []any{1.0, 1.0}, []any{-40.0, 10.0}}}
	},
	"Image:profile": func(s schemaNodes) (graph.NodeID, []any) {
		return s.img.ID, []any{[]any{190.0, 46.5}, []any{212.0, 46.5}, map[string]any{"width": 3}}
	},
	"Regions:where": func(s schemaNodes) (graph.NodeID, []any) {
		return s.specks.ID, []any{map[string]any{"score": map[string]any{"gte": 0.2}}}
	},
	"Regions:only":    func(s schemaNodes) (graph.NodeID, []any) { return s.specks.ID, []any{[]any{1, 2, 3}} },
	"Regions:without": func(s schemaNodes) (graph.NodeID, []any) { return s.specks.ID, []any{1} },
	"Regions:sortBy":  func(s schemaNodes) (graph.NodeID, []any) { return s.specks.ID, []any{"contrast"} },
	"Regions:top":     func(s schemaNodes) (graph.NodeID, []any) { return s.specks.ID, []any{5} },
	"Regions:measure": func(s schemaNodes) (graph.NodeID, []any) {
		return s.specks.ID, []any{graph.NodeArg{ID: s.img.ID}, map[string]any{"stats": []any{"max", "p90"}, "channels": true, "ring": true, "prefix": "img"}}
	},
}

// schemaScalar are Value ops whose result is a number or a derived value of
// their receiver, and ops the Go-only walk cannot run (data lambdas).
var schemaScalar = map[string]bool{
	"count": true, "coverage": true, "explain": true,
	"filter": true, // a data lambda: covered by the runtime tests (TestRegionItemsAreFlat)
}

// checkKeys reports keys that are not in the schema and required schema
// fields that are missing.
func checkKeys(t *testing.T, what string, keys []string, fields []graph.FieldSpec) {
	t.Helper()
	have := map[string]bool{}
	for _, k := range keys {
		have[k] = true
		if _, ok := graph.FindField(fields, k); !ok {
			t.Errorf("%s: key %q is not in the schema %v", what, k, graph.FieldNames(fields))
		}
	}
	for _, f := range fields {
		if !f.Optional && !have[f.Name] {
			t.Errorf("%s: schema field %q is missing (keys %v)", what, f.Name, keys)
		}
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestSchemaProperty: for every op returning Regions or Value rows/objects,
// the keys of its items, rows or object are the names of its node's Fields
// (optional ones may be absent), and every Regions field is accepted by
// where and sortBy.
func TestSchemaProperty(t *testing.T) {
	e, img, _ := plantedEngine(t)
	s := schemaNodes{img: img, mask: call(t, e, "mask.luminance", img.ID, map[string]any{"above": 0.75})}
	s.specks = call(t, e, "detect.specks", img.ID)
	ctx := context.Background()
	for _, op := range e.Registry().Ops() {
		if op.Returns != graph.KindValue && op.Returns != graph.KindRegions {
			continue
		}
		mk, ok := schemaCalls[op.Key()]
		if !ok {
			if !schemaScalar[op.Name] {
				t.Errorf("%s returns %s and is in neither schemaCalls nor schemaScalar", op.Key(), op.Returns)
			}
			continue
		}
		recv, raw := mk(s)
		n := call(t, e, op.Name, recv, raw...)
		if len(n.Fields) == 0 {
			t.Errorf("%s: node has no fields", op.Key())
			continue
		}
		v, err := e.Evaluate(ctx, n.ID, graph.EvalOptions{Scale: 1})
		if err != nil {
			t.Fatalf("%s: %v", op.Key(), err)
		}
		switch x := v.(type) {
		case *graph.RegionsValue:
			if len(x.Items) == 0 {
				t.Errorf("%s: no items to check", op.Key())
			}
			for _, r := range x.Items {
				d := r.Data(x.Fields)
				checkKeys(t, op.Key()+" item", d.Keys, n.Fields)
				if strings.Join(d.Keys, ",") != strings.Join(graph.FieldNames(n.Fields), ",") {
					t.Errorf("%s: item keys %v are not in schema order %v", op.Key(), d.Keys, graph.FieldNames(n.Fields))
				}
			}
			for _, f := range n.Fields {
				pred := map[string]any{f.Name: map[string]any{"gte": -1e9}}
				if f.Type == "string" {
					pred = map[string]any{f.Name: "x"}
				} else if _, err := e.Call("sortBy", n.ID, []any{f.Name}); err != nil {
					t.Errorf("%s: sortBy(%q): %v", op.Key(), f.Name, err)
				}
				w, err := e.Call("where", n.ID, []any{pred})
				if err != nil {
					t.Errorf("%s: where(%s): %v", op.Key(), f.Name, err)
					continue
				}
				if _, err := e.Evaluate(ctx, w.Node().ID, graph.EvalOptions{Scale: 1}); err != nil {
					t.Errorf("%s: where(%s) at evaluation: %v", op.Key(), f.Name, err)
				}
			}
		case *graph.DataValue:
			switch d := x.Data.(type) {
			case map[string]any:
				checkKeys(t, op.Key(), mapKeys(d), n.Fields)
			case []any:
				for i, row := range d {
					m, ok := row.(map[string]any)
					if !ok {
						t.Fatalf("%s: row %d is %T", op.Key(), i, row)
					}
					checkKeys(t, op.Key()+" row", mapKeys(m), n.Fields)
				}
			default:
				t.Errorf("%s: has fields but evaluates to %T", op.Key(), d)
			}
		}
	}
	// a mask with area and coverage: the schema follows the call
	st := call(t, e, "stats", img.ID, map[string]any{"box": []any{10.0, 10.0, 20.0, 20.0}})
	d := evalData(t, e, st, 1).(map[string]any)
	checkKeys(t, "stats({ box })", mapKeys(d), st.Fields)
}

// TestRegionFieldsFollowTheReceiver: filters keep their receiver's schema,
// and a field of another producer is refused at build time with its source.
func TestRegionFieldsFollowTheReceiver(t *testing.T) {
	e, img, _ := plantedEngine(t)
	comps := call(t, e, "components", call(t, e, "mask.luminance", img.ID, map[string]any{"above": 0.75}).ID)
	top := call(t, e, "top", call(t, e, "sortBy", comps.ID, "area").ID, 3)
	if got := strings.Join(graph.FieldNames(top.Fields), ","); got != "id,x,y,w,h,cx,cy,area,aspect" {
		t.Fatalf("components().sortBy().top() fields %s", got)
	}
	ce := callErr(t, e, "sortBy", top.ID, "snr")
	if ce.Code != graph.EUnknownOption || !strings.Contains(ce.Message, "id, x, y, w, h, cx, cy, area, aspect") {
		t.Fatalf("sortBy(snr): %+v", ce)
	}
	if ce := callErr(t, e, "sortBy", call(t, e, "detect.specks", img.ID).ID, "polarity"); ce.Code != graph.EArgType {
		t.Fatalf("sortBy of a string field: %+v", ce)
	}
	h, err := e.Registry().Help("detect.specks")
	if err != nil {
		t.Fatal(err)
	}
	entry := h["detect.specks"].(map[string]any)
	if f := entry["fields"].([]any); len(f) != 15 || f[0] != "id" || f[5] != "cx" || f[14] != "polarity" {
		t.Fatalf("help fields %v", f)
	}
}

// TestGradientTakesPointsOrEdges: mask.gradient's from/to are a point
// (list, or an object with cx, cy such as a region item) or an edge name.
func TestGradientTakesPointsOrEdges(t *testing.T) {
	e, img := newEngine(t)
	list := call(t, e, "mask.gradient", img.ID, map[string]any{"from": []any{2.0, 3.0}, "to": []any{40.0, 30.0}})
	obj := call(t, e, "mask.gradient", img.ID, map[string]any{
		"from": map[string]any{"id": 1.0, "cx": 2.0, "cy": 3.0, "x": 0.0, "y": 0.0, "w": 4.0, "h": 6.0},
		"to":   map[string]any{"cx": 40.0, "cy": 30.0},
	})
	if list.ID != obj.ID {
		t.Fatalf("an item and a list give different nodes: #%d #%d", list.ID, obj.ID)
	}
	if n := call(t, e, "mask.gradient", img.ID, map[string]any{"from": "top", "to": 0.4}); n.ID == list.ID {
		t.Fatal("edge and point gradients share a node")
	}
	if ce := callErr(t, e, "mask.gradient", img.ID, map[string]any{"from": map[string]any{"x": 2.0, "y": 3.0}, "to": []any{4.0, 4.0}}); ce.Code != graph.EArgType || !strings.Contains(ce.Message, "box corner") {
		t.Fatalf("{ x, y } as a point: %+v", ce)
	}
}
