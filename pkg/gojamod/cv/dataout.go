package cvmod

import (
	"context"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/runtimebridge"
	"github.com/go-go-golems/goja-cv/pkg/cv/dataout"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// Data out from JavaScript: node.save and the Data handles of cv.json and
// cv.csv (GCV-003 §7.9, §7.10). Saving runs off the owner goroutine and
// settles a promise, like await.

// bindFunc binds the raw arguments of registered function name (positional
// values, then an options object) through the op binder.
func (b *binding) bindFunc(name string, raw ...goja.Value) graph.Args {
	f, ok := b.eng.Registry().Func(name)
	if !ok {
		b.throw(graph.Errorf(graph.EInternal, "no %s function registered", name))
	}
	var vals []any
	for _, v := range raw {
		if x := exportArg(v); x != nil {
			vals = append(vals, fromJS(x))
		} else {
			vals = append(vals, nil)
		}
	}
	a, _, err := graph.BindOptions(f.CallName(), f.Args, vals)
	if err != nil {
		b.throw(err)
	}
	return a
}

// async runs fn on a goroutine under the eval's context and returns a
// promise settled on the owner goroutine with fn's plain-data result.
// Evaluation-time warnings are reported with the calling line.
func (b *binding) async(fn func(ctx context.Context, warn func(graph.Warning)) (any, error)) goja.Value {
	vm := b.vm
	svc, ok := runtimebridge.Lookup(vm)
	if !ok || svc.Owner == nil {
		b.throw(graph.Errorf(graph.EInternal, "cv: runtime services missing (create the runtime with go-go-goja's engine)"))
	}
	promise, resolve, reject := vm.NewPromise()
	callCtx := b.evalContext()
	site := b.site()
	go func() {
		v, err := fn(callCtx, func(w graph.Warning) { b.noteEvalWarning(w, site) })
		settleCtx := callCtx
		if callCtx.Err() != nil {
			settleCtx = svc.Lifetime()
		}
		_ = svc.PostWithCustomContext(settleCtx, "cv.async.settle", func(_ context.Context, vm *goja.Runtime) {
			if err != nil {
				_ = reject(errorObject(vm, err))
				return
			}
			_ = resolve(toJS(vm, v))
		})
	}()
	return vm.ToValue(promise)
}

// __save(id, path, opts) → Promise of { path, format, depth, encoding,
// frame, bytes, replaced }: node.save.
func (b *binding) saveNode(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	a := b.bindFunc("save", c.Argument(1), c.Argument(2))
	path, depth := a.String("path"), 0
	if a.Has("depth") {
		depth = a.Int("depth")
	}
	b.noteUsed(n.ID)
	root := b.eng.Options().OutRoot
	return b.async(func(ctx context.Context, warn func(graph.Warning)) (any, error) {
		s, err := dataout.SaveImage(ctx, b.eng, n.ID, root, path, depth, warn)
		if err != nil {
			if ce, ok := graph.AsError(err); ok && len(ce.Path) == 0 {
				ce.Path = append(b.eng.Path(n.ID), "✗ save")
			}
			return nil, err
		}
		return s.Data(), nil
	})
}
