package cvmod

import (
	"sort"

	"github.com/dop251/goja"
)

// __fieldNames() → the documented fields of every Value-returning op. The
// prelude uses it to tell a derived field (stats().meanL, check.dust().count)
// from a method of another kind called on a Value (stats().exposure(…)).
func (b *binding) fieldNames(goja.FunctionCall) goja.Value {
	seen := map[string]bool{}
	for _, op := range b.eng.Registry().Ops() {
		for _, f := range op.Fields {
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
