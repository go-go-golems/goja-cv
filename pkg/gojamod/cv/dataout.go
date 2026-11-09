package cvmod

import (
	"context"
	"fmt"
	"strconv"

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
		v, err := func() (v any, err error) { //nolint:nonamedreturns // recover needs named results
			defer graph.Recover(&err, "save")
			return fn(callCtx, func(w graph.Warning) { b.noteEvalWarning(w, site) })
		}()
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

// dataObject is the JavaScript face of a Data handle: a dynamic object
// whose exported value is the handle (a result-content provider), with the
// properties name, format and save.
type dataObject struct {
	*dataout.Data
	b *binding
}

var _ goja.DynamicObject = &dataObject{}

func (o *dataObject) Get(key string) goja.Value {
	switch key {
	case "save":
		return o.b.vm.ToValue(o.b.export("data.save", func(c goja.FunctionCall) goja.Value { return o.b.saveData(o.Data, c.Argument(0), c.Argument(1)) }))
	case "name":
		return o.b.vm.ToValue(o.Name())
	case "format":
		return o.b.vm.ToValue(o.Format())
	}
	return nil
}

func (o *dataObject) Set(string, goja.Value) bool { return false }
func (o *dataObject) Has(key string) bool         { return key == "save" || key == "name" || key == "format" }
func (o *dataObject) Delete(string) bool          { return false }
func (o *dataObject) Keys() []string              { return nil }

// DataOf reports whether v is a Data handle.
func DataOf(v goja.Value) (*dataout.Data, bool) {
	obj, ok := v.(*goja.Object)
	if !ok || obj == nil {
		return nil, false
	}
	d, ok := obj.Export().(*dataObject)
	if !ok || d == nil {
		return nil, false
	}
	return d.Data, true
}

// __data(format, x, opts) → a Data handle: cv.json and cv.csv. A node is
// kept (evaluated when delivered); anything else is plain data converted
// now, keeping object key order (awaited data is not lifted back to its
// node: what the agent sees, edits included, is written).
func (b *binding) dataJS(c goja.FunctionCall) goja.Value {
	format := c.Argument(0).String()
	f, ok := b.eng.Registry().Func(format)
	if !ok {
		b.throw(graph.Errorf(graph.EInternal, "no %s function registered", format))
	}
	x := c.Argument(1)
	var raw any
	node := graph.NodeID(0)
	if id, ok := NodeOf(x); ok {
		node = id
		raw = graph.NodeArg{ID: id}
	} else {
		raw = exportOrdered(b.vm, x, 0, map[*goja.Object]bool{})
	}
	vals := []any{raw}
	if o := exportArg(c.Argument(2)); o != nil {
		vals = append(vals, fromJS(o))
	}
	a, _, err := graph.BindOptions(f.CallName(), f.Args, vals)
	if err != nil {
		b.throw(err)
	}
	o := dataout.Options{}
	if a.Has("name") {
		o.Name = a.String("name")
	}
	if a.Has("columns") {
		o.Columns = a.Strings("columns")
	}
	d, err := dataout.New(b.eng, format, node, raw, o)
	if err != nil {
		b.throw(err)
	}
	if node != 0 {
		b.noteUsed(node)
	}
	return b.vm.NewDynamicObject(&dataObject{Data: d, b: b})
}

// saveData is data.save(path): a promise of { path, format, bytes, rows,
// replaced }.
func (b *binding) saveData(d *dataout.Data, path, opts goja.Value) goja.Value {
	a := b.bindFunc("save", path, opts)
	p := a.String("path")
	if a.Has("depth") {
		b.throw(graph.Errorf(graph.EUnknownOption, "data.save takes no depth: depth is for images.").WithFix(fmt.Sprintf("data.save(%q)", p)))
	}
	root := b.eng.Options().OutRoot
	return b.async(func(ctx context.Context, _ func(graph.Warning)) (any, error) {
		s, err := d.Save(ctx, root, p)
		if err != nil {
			return nil, err
		}
		return s.Data(), nil
	})
}

// exportOrdered converts a JavaScript value into plain data for a Data
// handle: objects become graph.Records in their key order, arrays lists,
// nodes and looks their one-line description.
func exportOrdered(vm *goja.Runtime, v goja.Value, depth int, seen map[*goja.Object]bool) any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	obj, ok := v.(*goja.Object)
	if !ok {
		return graph.Normalize(v.Export())
	}
	if _, ok := NodeOf(obj); ok {
		return obj.String()
	}
	if l, ok := LookOf(obj); ok {
		return l.String()
	}
	if _, ok := goja.AssertFunction(obj); ok {
		return "[Function]"
	}
	if depth > 32 || seen[obj] {
		return "[Circular]"
	}
	seen[obj] = true
	defer delete(seen, obj)
	if obj.ClassName() == "Array" {
		n := int(obj.Get("length").ToInteger())
		out := make([]any, n)
		for i := 0; i < n; i++ {
			out[i] = exportOrdered(vm, obj.Get(strconv.Itoa(i)), depth+1, seen)
		}
		return out
	}
	if obj.ClassName() != "Object" {
		return graph.Normalize(obj.Export())
	}
	keys := obj.Keys()
	rec := graph.Record{Keys: keys, Vals: make(map[string]any, len(keys))}
	for _, k := range keys {
		rec.Vals[k] = exportOrdered(vm, obj.Get(k), depth+1, seen)
	}
	return rec
}
