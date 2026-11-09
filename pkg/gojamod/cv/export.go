package cvmod

import (
	"runtime/debug"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// The panic boundary (GCV-004 §7.11). A Go panic in a native function
// called from JavaScript would unwind out of the VM and, on a goroutine
// nobody recovers, end the process. Every native function is wrapped by
// Export: a Go panic becomes a thrown E_INTERNAL; the values goja itself
// panics with while unwinding native frames (a JavaScript throw, an
// interrupt from the eval's watchdog, a stack overflow) pass through, so a
// timeout stays E_TIMEOUT.

// The uncatchable exceptions goja raises; a goja upgrade that renames them
// breaks the build here instead of silently turning timeouts into
// E_INTERNAL.
var (
	_ = (*goja.InterruptedError)(nil)
	_ = (*goja.StackOverflowError)(nil)
)

// panicHook, when set (tests only, export_test.go), is called at the start
// of every wrapped native function.
var panicHook func(name string)

// Export wraps a native function: a Go panic becomes a thrown E_INTERNAL
// (with the stack in the hint); goja's own panics pass through.
func Export(vm *goja.Runtime, name string, fn func(goja.FunctionCall) goja.Value) func(goja.FunctionCall) goja.Value {
	return func(c goja.FunctionCall) goja.Value {
		defer func() {
			if p := recover(); p != nil {
				if isGojaPanic(p) {
					panic(p) // a JavaScript throw, an interrupt (timeout), a stack overflow: goja's to handle
				}
				panic(errorObject(vm, graph.Errorf(graph.EInternal, "%s panicked: %v", name, p).WithHint(string(debug.Stack()))))
			}
		}()
		if panicHook != nil {
			panicHook(name)
		}
		return fn(c)
	}
}

// isGojaPanic reports whether p is a value goja itself panics with while
// unwinding native frames.
func isGojaPanic(p any) bool {
	switch p.(type) {
	case goja.Value, *goja.Exception, *goja.InterruptedError, *goja.StackOverflowError:
		return true
	}
	return false
}

func (b *binding) export(name string, fn func(goja.FunctionCall) goja.Value) func(goja.FunctionCall) goja.Value {
	return Export(b.vm, name, fn)
}
