package cvmod

import (
	"sort"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// __fieldNames() → the documented fields of every Value-returning op (its
// schema with default arguments and the fields it produces with others).
// The prelude uses it to tell a derived field (stats().meanL,
// check.dust().count) from a method of another kind called on a Value
// (stats().exposure(…)).
func (b *binding) fieldNames(goja.FunctionCall) goja.Value {
	seen := map[string]bool{}
	for _, op := range b.eng.Registry().Ops() {
		if op.Returns != graph.KindValue {
			continue
		}
		for _, f := range op.DocFields() {
			seen[f.Name] = true
		}
		for f := range op.Produces {
			seen[f] = true
		}
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return toJS(b.vm, toAny(out))
}

// __removedFields() → { box: fix, centroid: fix, meanMask: fix }: the field
// names awaited region items throw E_NO_FIELD for (graph.RemovedFields).
func (b *binding) removedFields(goja.FunctionCall) goja.Value {
	out := map[string]any{}
	for k, v := range graph.RemovedFields {
		out[k] = v
	}
	return toJS(b.vm, out)
}

// __noField(name) throws the E_NO_FIELD error of a removed field name.
func (b *binding) noField(c goja.FunctionCall) goja.Value {
	if e := graph.RemovedFieldError(c.Argument(0).String()); e != nil {
		b.throw(e)
	}
	return goja.Undefined()
}

// __itemProto(proto) registers the prototype the prelude gives awaited
// region items (throwing accessors for removed names), so the items data
// lambdas receive get it too.
func (b *binding) itemProtoJS(c goja.FunctionCall) goja.Value {
	if o, ok := c.Argument(0).(*goja.Object); ok {
		b.itemProto = o
	}
	return goja.Undefined()
}
