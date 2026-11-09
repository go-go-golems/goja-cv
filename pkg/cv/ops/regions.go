package ops

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func regions(v graph.Value) *graph.RegionsValue { return v.(*graph.RegionsValue) }

// analysisScaleArg is the static argument of Regions-producing ops that
// fixes the scale they analyse at (see OpDef.AnalysisScale).
var analysisScaleArg = graph.ArgSpec{Name: "analysisScale", Type: "number", Default: 1.0, Min: graph.F(0.05), Max: graph.F(1),
	Doc: "scale the input is analysed at, whatever the evaluation scale, so region ids and doc-px coordinates are the same in a look and a measurement"}

func analysisScale(a graph.Args) float64 {
	f, _ := a.Raw("analysisScale").(float64)
	return f
}

// sortByArea orders regions by area (largest first), then id.
func sortByArea(items []*graph.Region) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Area != items[j].Area {
			return items[i].Area > items[j].Area
		}
		return items[i].ID < items[j].ID
	})
}

var predOps = []string{"lt", "lte", "gt", "gte", "eq", "ne", "in"}

// validatePredicate checks a where() predicate against the receiver's
// fields: { field: { op: value } } or { field: value } (eq).
func validatePredicate(a graph.Args) error {
	pred, ok := a.Raw("pred").(map[string]any)
	if !ok {
		return graph.Errorf(graph.EArgType, "where needs a predicate object such as { area: { gte: 4 } }.").
			WithFix("regions.where({ area: { gte: 4, lte: 120 } })")
	}
	fields := receiverFields(a)
	for _, f := range sortedKeys(pred) {
		if _, ok := graph.FindField(fields, f); !ok {
			return graph.UnknownFieldError("where", f, fields, func(sug string) string { return fmt.Sprintf("where({ %s: … })", sug) })
		}
		cond, ok := pred[f].(map[string]any)
		if !ok {
			continue // shorthand { field: value } means eq
		}
		if len(cond) == 0 {
			return graph.Errorf(graph.EArgType, "where.%s is an empty condition; use one of %s.", f, strings.Join(predOps, ", ")).WithValid(predOps).WithFix(fmt.Sprintf("where({ %s: { gte: 0.5 } })", f))
		}
		for _, op := range sortedKeys(cond) {
			if !contains(predOps, op) {
				e := graph.Errorf(graph.EUnknownOption, "where.%s has no comparison %q. Valid: %s.", f, op, strings.Join(predOps, ", ")).WithValid(predOps)
				if sug := graph.Closest(op, predOps); sug != "" {
					e.Hint, e.Fix = fmt.Sprintf("Did you mean %q?", sug), fmt.Sprintf("where({ %s: { %s: … } })", f, sug)
				}
				return e
			}
			v := cond[op]
			switch op {
			case "in":
				if _, ok := v.([]any); !ok {
					return graph.Errorf(graph.EArgType, "where.%s.in must be a list.", f).WithFix(fmt.Sprintf("where({ %s: { in: [1, 2, 3] } })", f))
				}
			case "lt", "lte", "gt", "gte":
				if _, ok := v.(float64); !ok {
					return graph.Errorf(graph.EArgType, "where.%s.%s must be a number.", f, op).WithFix(fmt.Sprintf("where({ %s: { %s: 0.5 } })", f, op))
				}
			}
		}
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func compare(op string, got, want any) (bool, error) {
	switch op {
	case "eq":
		return got == want, nil
	case "ne":
		return got != want, nil
	case "in":
		list, ok := want.([]any)
		if !ok {
			return false, graph.Errorf(graph.EArgType, "in needs a list, got %v", want)
		}
		for _, e := range list {
			if e == got {
				return true, nil
			}
		}
		return false, nil
	}
	g, ok1 := got.(float64)
	w, ok2 := want.(float64)
	if !ok1 || !ok2 {
		return false, graph.Errorf(graph.EArgType, "%s compares numbers; got %v and %v", op, got, want)
	}
	switch op {
	case "lt":
		return g < w, nil
	case "lte":
		return g <= w, nil
	case "gt":
		return g > w, nil
	case "gte":
		return g >= w, nil
	}
	return false, graph.Errorf(graph.EArgType, "unknown comparison %q", op)
}

// receiverFields is the schema of a Regions op's receiver (the geometry
// fields in help, which has no call).
func receiverFields(a graph.Args) []graph.FieldSpec {
	if !a.HasInputs() {
		return graph.GeometryFields
	}
	return a.Receiver().Fields
}

func matches(r *graph.Region, pred map[string]any) (bool, error) {
	for _, f := range sortedKeys(pred) {
		got, ok := r.Field(f)
		if !ok {
			return false, graph.Errorf(graph.ENoField, "region %d has no field %q.", r.ID, f)
		}
		cond, ok := pred[f].(map[string]any)
		if !ok {
			cond = map[string]any{"eq": pred[f]}
		}
		for _, op := range sortedKeys(cond) {
			ok, err := compare(op, got, cond[op])
			if err != nil {
				return false, graph.Errorf(graph.EArgType, "where.%s: %v", f, err)
			}
			if !ok {
				return false, nil
			}
		}
	}
	return true, nil
}

// idSet reads ids given as a number, a region item (an object with id) or
// a list of them.
func idSet(v any) (map[int]bool, error) {
	out := map[int]bool{}
	var list []any
	switch x := v.(type) {
	case float64, map[string]any:
		list = []any{x}
	case []any:
		list = x
	default:
		return nil, graph.Errorf(graph.EArgType, "ids must be a number, a region item or a list of them, got %v.", v).WithFix("without([3, 17])   // region ids from (await regions).items")
	}
	for _, e := range list {
		if m, ok := e.(map[string]any); ok {
			id, ok := m["id"]
			if !ok {
				return nil, graph.Errorf(graph.EArgType, "an object passed as an id needs an id field (a region item).").WithFix("only(item)   // or only(item.id)")
			}
			e = id
		}
		f, ok := e.(float64)
		if !ok || f != math.Trunc(f) {
			return nil, graph.Errorf(graph.EArgType, "ids must be whole numbers, got %v.", e)
		}
		out[int(f)] = true
	}
	return out, nil
}

func validateIDs(a graph.Args) error {
	_, err := idSet(a.Raw("ids"))
	return err
}

func idFilterOp(name string, keep bool, doc string) graph.OpDef {
	return graph.OpDef{
		Name: name, On: graph.KindRegions, Returns: graph.KindRegions,
		Args:     []graph.ArgSpec{{Name: "ids", Type: "any", Positional: true, Doc: "a region id, a region item (anything with id) or a list of them"}},
		Validate: validateIDs,
		Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			ids, err := idSet(a.Raw("ids"))
			if err != nil {
				return nil, err
			}
			rv := regions(in[0])
			var out []*graph.Region
			for _, r := range rv.Items {
				if ids[r.ID] == keep {
					out = append(out, r)
				}
			}
			return rv.With(out), nil
		},
		Doc: doc,
		See: []string{"where", "only", "without"},
	}
}

func regionOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "components", On: graph.KindMask, Returns: graph.KindRegions, Global: true,
			Args: []graph.ArgSpec{
				{Name: "minArea", Type: "number", Unit: graph.DocPx2, Default: 1.0, Min: graph.F(0), Doc: "smallest region kept, in doc px²",
					Redirects: map[string]graph.Redirect{"maxArea": {Hint: "components keeps every region from minArea up; drop large ones with where.",
						Fix: "mask.components({ minArea: 4 }).where({ area: { lte: 200 } })"}}},
				{Name: "threshold", Type: "number", Default: 0.5, Min: graph.F(0), Max: graph.F(1), Doc: "mask values above this are selected"},
				analysisScaleArg,
			},
			AnalysisScale: analysisScale,
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				m := mask(in[0])
				pl := m.Buf.Plane(0)
				thr := float32(a.Number("threshold"))
				fg := make([]bool, len(pl))
				for i, v := range pl {
					fg[i] = v > thr
				}
				comps, err := kernels.Components(ctx, fg, m.Buf.W, m.Buf.H)
				if err != nil {
					return nil, err
				}
				rv := graph.NewRegions(m.Buf.W, m.Buf.H, ec.Scale, m.Frame)
				minPx := a.Area("minArea", ec.Scale)
				for _, pix := range comps {
					if float64(len(pix)) < minPx {
						continue
					}
					rv.Items = append(rv.Items, rv.NewRegion(len(rv.Items)+1, pix))
				}
				sortByArea(rv.Items)
				return rv, nil
			},
			Doc:    "The connected (8-neighbour) areas of the mask as Regions, largest first; ids follow raster order.",
			Fields: geometryFields,
			See:    []string{"toMask", "where", "measure"},
		},
		{
			Name: "where", On: graph.KindRegions, Returns: graph.KindRegions,
			Args: []graph.ArgSpec{{Name: "pred", Type: "any", Positional: true,
				Doc: "{ field: { lt, lte, gt, gte, eq, ne, in } } or { field: value }"}},
			Validate: validatePredicate,
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				rv := regions(in[0])
				pred := a.Raw("pred").(map[string]any)
				var out []*graph.Region
				for _, r := range rv.Items {
					ok, err := matches(r, pred)
					if err != nil {
						return nil, err
					}
					if ok {
						out = append(out, r)
					}
				}
				return rv.With(out), nil
			},
			Doc: "Keep the regions matching a declarative predicate (all conditions must hold).",
			See: []string{"filter", "without", "sortBy"},
		},
		idFilterOp("without", false, "Drop the regions with these ids."),
		idFilterOp("only", true, "Keep only the regions with these ids."),
		{
			Name: "sortBy", On: graph.KindRegions, Returns: graph.KindRegions,
			Args: []graph.ArgSpec{
				{Name: "key", Type: "string", Positional: true, Doc: "a region field (area, score, contrast, …)"},
				{Name: "desc", Type: "bool", Default: true, Doc: "largest first"},
			},
			Validate: func(a graph.Args) error {
				k := a.String("key")
				fields := receiverFields(a)
				f, ok := graph.FindField(fields, k)
				if !ok {
					return graph.UnknownFieldError("sortBy", k, fields, func(sug string) string { return fmt.Sprintf("sortBy(%q)", sug) })
				}
				if f.Type != "number" {
					return graph.Errorf(graph.EArgType, "sortBy(%q): %s is a %s field; sortBy orders by a number.", k, k, f.Type).
						WithFix(fmt.Sprintf("where({ %s: \"…\" })", k))
				}
				return nil
			},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				rv := regions(in[0])
				key, desc := a.String("key"), a.Bool("desc")
				items := append([]*graph.Region(nil), rv.Items...)
				var ferr error
				val := func(r *graph.Region) float64 {
					v, ok := r.Field(key)
					f, isNum := v.(float64)
					if !ok || !isNum {
						if ferr == nil {
							ferr = graph.Errorf(graph.ENoField, "region %d has no number %q.", r.ID, key)
						}
						return 0
					}
					return f
				}
				sort.SliceStable(items, func(i, j int) bool {
					vi, vj := val(items[i]), val(items[j])
					if desc {
						return vi > vj
					}
					return vi < vj
				})
				if ferr != nil {
					return nil, ferr
				}
				return rv.With(items), nil
			},
			Doc: "Order the regions by a numeric field (largest first unless desc: false).",
			See: []string{"top"},
		},
		{
			Name: "top", On: graph.KindRegions, Returns: graph.KindRegions,
			Args: []graph.ArgSpec{{Name: "n", Type: "number", Positional: true, Min: graph.F(0)}},
			MaxItems: func(a graph.Args) (int, bool) {
				n, ok := a.Raw("n").(float64)
				return int(n), ok
			},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				rv := regions(in[0])
				n := min(len(rv.Items), a.Int("n"))
				return rv.With(rv.Items[:n:n]), nil
			},
			Doc: "The first n regions in the current order.",
			See: []string{"sortBy"},
		},
		{
			Name: "count", On: graph.KindRegions, Returns: graph.KindValue,
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				return data(float64(len(regions(in[0]).Items))), nil
			},
			Doc: "The number of regions.",
		},
		{
			Name: "toMask", On: graph.KindRegions, Returns: graph.KindMask,
			Args: []graph.ArgSpec{{Name: "grow", Type: "number", Unit: graph.DocPx, Default: 0.0, Min: graph.F(0), Max: graph.F(500),
				Doc: "dilate each region by this many doc px"}},
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				rv := regions(in[0])
				m, err := rasterize(ctx, rv, ec.Scale)
				if err != nil {
					return nil, err
				}
				if g := a.Length("grow", ec.Scale); g > 0 {
					if m, err = kernels.Grow(ctx, m, g); err != nil {
						return nil, err
					}
				}
				return graph.NewMask(m), nil
			},
			Doc: "The regions as a Mask at the evaluation scale (exact area coverage), optionally grown.",
			See: []string{"components", "grow"},
		},
		{
			Name: "filter", On: graph.KindRegions, Returns: graph.KindRegions,
			Args: []graph.ArgSpec{{Name: "fn", Type: "func", Positional: true, Doc: "data lambda r => bool, called with each region's data"}},
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				rv := regions(in[0])
				args := make([]any, len(rv.Items))
				fields := rv.Schema()
				for i, r := range rv.Items {
					args[i] = r.Data(fields)
				}
				res, err := ec.Eval.CallEach(ctx, a.Func("fn"), args)
				if err != nil {
					return nil, err
				}
				var out []*graph.Region
				for i, r := range rv.Items {
					if truthy(res[i]) {
						out = append(out, r)
					}
				}
				return rv.With(out), nil
			},
			Doc: "Keep the regions for which a JavaScript function returns true (a data lambda: it runs on the JS thread at evaluation time). Prefer where() when the test can be written as data.",
			See: []string{"where"},
		},
	}
}

// rasterize draws rv at the buffer size of scale.
func rasterize(ctx context.Context, rv *graph.RegionsValue, scale float64) (*buffer.Buffer, error) {
	return rv.Rasterize(ctx, graph.FrameRaster(rv.Frame, scale))
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && !math.IsNaN(x)
	case int64:
		return x != 0
	case string:
		return x != ""
	}
	return true
}
