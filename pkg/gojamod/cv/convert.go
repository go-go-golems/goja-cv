package cvmod

import (
	"sort"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// fromJS converts an exported, prelude-marshalled argument into the raw
// argument space of graph.Engine.Call: node markers {__cvnode: id} become
// graph.NodeArg, function markers become graph.FuncArg.
func fromJS(v any) any {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := x["__cvnode"]; ok && len(x) == 1 {
			f, _ := graph.Normalize(id).(float64)
			return graph.NodeArg{ID: graph.NodeID(f)}
		}
		if _, ok := x["__cvfunc"]; ok && len(x) == 1 {
			return graph.FuncArg{}
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
// never mutate cached Go data. Object keys are sorted.
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
