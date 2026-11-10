package cvmod

import (
	"sort"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// fromJS converts an exported, prelude-marshalled argument into the raw
// argument space of graph.Engine.Call: node markers {__cvnode: id} become
// graph.NodeArg and builder results {__cvbuilt: [root, placeholder]} become
// graph.BuiltArg. An awaited item keeps its link as the key __cvref: [node,
// index], which the binder reads for Items arguments and drops elsewhere.
func fromJS(v any) any {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := x["__cvnode"]; ok && len(x) == 1 {
			f, _ := graph.Normalize(id).(float64)
			return graph.NodeArg{ID: graph.NodeID(f)}
		}
		if ids, ok := x["__cvbuilt"].([]any); ok && len(x) == 1 && len(ids) == 2 {
			root, _ := graph.Normalize(ids[0]).(float64)
			hole, _ := graph.Normalize(ids[1]).(float64)
			return graph.BuiltArg{Root: graph.NodeID(root), Hole: graph.NodeID(hole)}
		}
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = fromJS(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = fromJS(e)
		}
		return out
	}
	return graph.Normalize(v)
}

// toJS deep-copies plain Go data into fresh JavaScript values, so scripts can
// never mutate cached Go data. Object keys are sorted, except a
// graph.Record's, which keep their (schema) order.
func toJS(vm *goja.Runtime, v any) goja.Value {
	switch x := v.(type) {
	case nil:
		return goja.Null()
	case map[string]any:
		o := vm.NewObject()
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			_ = o.Set(k, toJS(vm, x[k]))
		}
		return o
	case graph.Record:
		o := vm.NewObject()
		for _, k := range x.Keys {
			_ = o.Set(k, toJS(vm, x.Vals[k]))
		}
		return o
	case []any:
		items := make([]any, len(x))
		for i, e := range x {
			items[i] = toJS(vm, e)
		}
		return vm.NewArray(items...)
	case []string:
		items := make([]any, len(x))
		for i, e := range x {
			items[i] = e
		}
		return vm.NewArray(items...)
	case []float64:
		items := make([]any, len(x))
		for i, e := range x {
			items[i] = e
		}
		return vm.NewArray(items...)
	}
	return vm.ToValue(v)
}
