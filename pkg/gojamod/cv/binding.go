package cvmod

import (
	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// binding is the per-runtime native side of the module. All of its methods
// run on the runtime owner goroutine (they are called from JavaScript).
type binding struct {
	vm  *goja.Runtime
	reg *graph.Registry
}

func newBinding(vm *goja.Runtime, reg *graph.Registry) *binding {
	return &binding{vm: vm, reg: reg}
}

func (b *binding) object() *goja.Object {
	o := b.vm.NewObject()
	_ = o.Set("__help", b.help)
	return o
}

func (b *binding) help(call goja.FunctionCall) goja.Value {
	ops := map[string]any{}
	for _, op := range b.reg.Ops() {
		ops[string(op.On)+"."+op.Name] = op.Doc
	}
	kinds := make([]string, 0, len(graph.Kinds))
	for _, k := range graph.Kinds {
		kinds = append(kinds, string(k))
	}
	return b.vm.ToValue(map[string]any{"kinds": kinds, "ops": ops})
}
