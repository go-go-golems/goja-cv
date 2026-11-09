package cvmod

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/runtimebridge"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// binding is the per-runtime native side of the module. Its exported
// functions run on the runtime owner goroutine (they are called from
// JavaScript); evaluation runs on other goroutines and settles promises by
// posting back to the owner.
type binding struct {
	vm  *goja.Runtime
	eng *graph.Engine

	mu       sync.Mutex
	warnings []graph.Warning

	// funcs holds the JavaScript functions passed as data lambdas, by id.
	// Only the owner goroutine reads or writes it.
	funcs    map[graph.FuncID]goja.Callable
	nextFunc graph.FuncID
}

var _ graph.Callbacks = &binding{}

func newBinding(vm *goja.Runtime, eng *graph.Engine) *binding {
	b := &binding{vm: vm, eng: eng, funcs: map[graph.FuncID]goja.Callable{}}
	eng.Evaluator().SetCallbacks(b)
	return b
}

// __func(fn) → id: registers a data lambda. The prelude caches ids per
// function object, so passing the same function twice gives the same node.
func (b *binding) funcJS(c goja.FunctionCall) goja.Value {
	fn, ok := goja.AssertFunction(c.Argument(0))
	if !ok {
		b.throw(graph.Errorf(graph.EArgType, "expected a function"))
	}
	b.nextFunc++
	b.funcs[b.nextFunc] = fn
	return b.vm.ToValue(int64(b.nextFunc))
}

// CallEach runs data lambda id on the runtime owner goroutine, once per
// argument (converted to fresh JS objects), and returns the exported
// results. It is called from evaluation goroutines, never from the owner
// (evaluation started from JavaScript always runs off the owner), so
// Owner.Call can schedule onto the idle event loop.
func (b *binding) CallEach(ctx context.Context, id graph.FuncID, args []any) ([]any, error) {
	svc, ok := runtimebridge.Lookup(b.vm)
	if !ok || svc.Owner == nil {
		return nil, graph.Errorf(graph.EInternal, "cv: runtime services missing; data lambdas need a go-go-goja runtime")
	}
	res, err := svc.Owner.Call(ctx, "cv.datalambda", func(_ context.Context, vm *goja.Runtime) (any, error) {
		fn, ok := b.funcs[id]
		if !ok {
			return nil, graph.Errorf(graph.EInternal, "data lambda #%d is not registered", id)
		}
		out := make([]any, len(args))
		for i, a := range args {
			v, err := fn(goja.Undefined(), toJS(vm, a))
			if err != nil {
				return nil, graph.Errorf(graph.EEval, "the function passed to filter threw on item %d: %v", i, err).
					WithHint("The function receives one region's data: { id, box, area, centroid, score, … }.")
			}
			out[i] = graph.Normalize(v.Export())
		}
		return out, nil
	})
	if err != nil {
		if ce, ok := graph.AsError(err); ok {
			return nil, ce
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, graph.Errorf(graph.EEval, "data lambda: %v", err)
	}
	return res.([]any), nil
}

func (b *binding) object() *goja.Object {
	o := b.vm.NewObject()
	set := func(name string, fn func(goja.FunctionCall) goja.Value) { _ = o.Set(name, fn) }
	_ = o.Set("__version", Version)
	set("__registry", b.registry)
	set("__call", b.call)
	set("__open", b.open)
	set("__evaluate", b.evaluate)
	set("__describe", b.describe)
	set("__unknown", b.unknown)
	set("__lazy", b.lazy)
	set("__immutable", b.immutable)
	set("__help", b.help)
	set("__warnings", b.warningsJS)
	set("__param", b.param)
	set("__paramInfo", b.paramInfo)
	set("__paramSet", b.paramSet)
	set("__params", b.params)
	set("__func", b.funcJS)
	set("__builders", b.builders)
	set("__placeholder", b.placeholder)
	return o
}

// throw raises err as a structured cv error in JavaScript.
func (b *binding) throw(err error) {
	panic(errorObject(b.vm, err))
}

// errorObject converts err into a JavaScript Error whose name is the cv code
// and whose own properties carry code, message, path, hint, fix and valid.
func errorObject(vm *goja.Runtime, err error) *goja.Object {
	ce, ok := graph.AsError(err)
	if !ok {
		ce = &graph.Error{Code: graph.EEval, Message: err.Error()}
	}
	obj, nerr := vm.New(vm.Get("Error"), vm.ToValue(ce.Message))
	if nerr != nil {
		return vm.NewGoError(err)
	}
	_ = obj.Set("name", ce.Code)
	_ = obj.Set("code", ce.Code)
	_ = obj.Set("cv", true)
	if len(ce.Path) > 0 {
		_ = obj.Set("path", toJS(vm, toAny(ce.Path)))
	}
	if ce.Hint != "" {
		_ = obj.Set("hint", ce.Hint)
	}
	if ce.Fix != "" {
		_ = obj.Set("fix", ce.Fix)
	}
	if len(ce.Valid) > 0 {
		_ = obj.Set("valid", toJS(vm, toAny(ce.Valid)))
	}
	return obj
}

func toAny(xs []string) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func (b *binding) nodeArg(v goja.Value) *graph.Node {
	id := graph.NodeID(v.ToInteger())
	n, ok := b.eng.Node(id)
	if !ok {
		b.throw(graph.Errorf(graph.EInternal, "unknown node #%d", id))
	}
	return n
}

func handle(vm *goja.Runtime, n *graph.Node) goja.Value {
	o := vm.NewObject()
	_ = o.Set("id", int64(n.ID))
	_ = o.Set("kind", string(n.Kind))
	return o
}

func (b *binding) addWarnings(ws []graph.Warning) {
	if len(ws) == 0 {
		return
	}
	b.mu.Lock()
	b.warnings = append(b.warnings, ws...)
	b.mu.Unlock()
}

func (b *binding) drainWarnings() []graph.Warning {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.warnings
	b.warnings = nil
	return out
}

// __registry() → { Image: ["contrast", "exposure", …, "mask.luminance"], … }
func (b *binding) registry(goja.FunctionCall) goja.Value {
	out := map[string]any{}
	for _, k := range graph.Kinds {
		out[string(k)] = toAny(b.eng.Registry().Methods(k))
	}
	return toJS(b.vm, out)
}

// __builders() → { "Image:within": [1], … }: argument positions that take
// builder lambdas.
func (b *binding) builders(goja.FunctionCall) goja.Value {
	out := map[string]any{}
	for _, op := range b.eng.Registry().Ops() {
		if len(op.Builders) == 0 {
			continue
		}
		pos := make([]any, len(op.Builders))
		for i, p := range op.Builders {
			pos[i] = float64(p)
		}
		out[string(op.On)+":"+op.Name] = pos
	}
	return toJS(b.vm, out)
}

// __placeholder(receiverId) → {id, kind}: the node a builder lambda receives.
func (b *binding) placeholder(c goja.FunctionCall) goja.Value {
	n, err := b.eng.Placeholder(graph.NodeID(c.Argument(0).ToInteger()))
	if err != nil {
		b.throw(err)
	}
	return handle(b.vm, n)
}

// __call(op, receiverId, args[]) → {id, kind}
func (b *binding) call(c goja.FunctionCall) goja.Value {
	name := c.Argument(0).String()
	recv := graph.NodeID(c.Argument(1).ToInteger())
	var raw []any
	if a := c.Argument(2); !goja.IsUndefined(a) && !goja.IsNull(a) {
		list, _ := fromJS(a.Export()).([]any)
		raw = list
	}
	n, ws, err := b.eng.Call(name, recv, raw)
	if err != nil {
		b.throw(err)
	}
	b.addWarnings(ws)
	return handle(b.vm, n)
}

// __open(path) → {id, kind}
func (b *binding) open(c goja.FunctionCall) goja.Value {
	p := c.Argument(0)
	if goja.IsUndefined(p) || p.ExportType() == nil || p.ExportType().Kind().String() != "string" {
		b.throw(graph.Errorf(graph.EArgType, "cv.open needs a file path string.").WithFix(`cv.open("scan.tif")`))
	}
	n, err := b.eng.Open(p.String())
	if err != nil {
		b.throw(err)
	}
	return handle(b.vm, n)
}

func (b *binding) scaleArg(n *graph.Node, v goja.Value) float64 {
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return b.eng.PreviewScale(n.ID)
	}
	switch x := graph.Normalize(v.Export()).(type) {
	case string:
		switch x {
		case "preview":
			return b.eng.PreviewScale(n.ID)
		case "look":
			return b.eng.ScaleForLong(n.ID, b.eng.Options().LookLong)
		case "full":
			return 1
		}
	case float64:
		if x > 0 && x <= 1 {
			return x
		}
	}
	b.throw(graph.Errorf(graph.EArgRange, "scale must be a number in (0, 1] or \"preview\", \"look\", \"full\"; got %s.", v.String()))
	return 0
}

// __evaluate(id, scale|null) → Promise. The promise is created here, on the
// owner goroutine; the engine runs on a goroutine; the result is converted
// and the promise settled in a callback posted back to the owner. The
// goroutine never touches the VM.
func (b *binding) evaluate(c goja.FunctionCall) goja.Value {
	vm := b.vm
	n := b.nodeArg(c.Argument(0))
	scale := b.scaleArg(n, c.Argument(1))
	svc, ok := runtimebridge.Lookup(vm)
	if !ok || svc.Owner == nil {
		b.throw(graph.Errorf(graph.EInternal, "cv: runtime services missing (create the runtime with go-go-goja's engine)"))
	}
	promise, resolve, reject := vm.NewPromise()
	callCtx := runtimebridge.CurrentOwnerContext(vm)
	go func() {
		v, err := b.eng.Evaluate(callCtx, n.ID, graph.EvalOptions{Scale: scale})
		settleCtx := callCtx
		if callCtx.Err() != nil {
			settleCtx = svc.Lifetime() // still reject, so awaiting code sees E_CANCELLED
		}
		_ = svc.PostWithCustomContext(settleCtx, "cv.evaluate.settle", func(_ context.Context, vm *goja.Runtime) {
			if err != nil {
				_ = reject(errorObject(vm, err))
				return
			}
			_ = resolve(resultToJS(vm, v))
		})
	}()
	return vm.ToValue(promise)
}

func resultToJS(vm *goja.Runtime, v graph.Value) goja.Value {
	switch x := v.(type) {
	case *graph.ImageValue:
		return toJS(vm, x.Summary())
	case *graph.RegionsValue:
		return toJS(vm, x.Summary(graph.SummaryLimit))
	case *graph.DataValue:
		return toJS(vm, x.Data)
	}
	return goja.Undefined()
}

// __describe(id) → {id, kind, op, call, label, docW, docH, path}
func (b *binding) describe(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	scale := b.eng.PreviewScale(n.ID)
	return toJS(b.vm, map[string]any{
		"id": float64(n.ID), "kind": string(n.Kind), "op": n.Op.Name, "label": graph.Label(n),
		"call": graph.Describe(b.eng.Builder(), n, scale, b.eng.Params().Snapshot()),
		"docW": float64(n.DocW), "docH": float64(n.DocH),
		"path":  toAny(b.eng.Path(n.ID)),
		"scale": scale,
	})
}

// __unknown(kind, prop, receiverId) throws E_UNKNOWN_METHOD / E_NAMESPACE.
func (b *binding) unknown(c goja.FunctionCall) goja.Value {
	kind := graph.Kind(c.Argument(0).String())
	prop := c.Argument(1).String()
	var path []string
	if id := graph.NodeID(c.Argument(2).ToInteger()); id != 0 {
		path = b.eng.Path(id)
	}
	b.throw(b.eng.Registry().UnknownMethod(kind, prop, path))
	return nil
}

// __lazy(id, hint) throws E_LAZY_COMPARE.
func (b *binding) lazy(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	e := graph.Errorf(graph.ELazyCompare, "A %s node was used as a %s, but nodes are lazy: nothing has been computed yet.", n.Kind, c.Argument(1).String())
	e.Path = b.eng.Path(n.ID)
	e.Hint = "Await the node to get its value."
	if n.Kind == graph.KindValue || n.Kind == graph.KindParam {
		e.Fix = "(await node) > 0.5   // e.g. (await img.stats()).meanL > 0.5"
	} else {
		e.Fix = "const v = await node   // an Image awaits to a summary { kind, w, h, scale }"
	}
	b.throw(e)
	return nil
}

// __immutable(id, prop) throws E_IMMUTABLE.
func (b *binding) immutable(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	prop := c.Argument(1).String()
	e := graph.Errorf(graph.EImmutable, "Nodes are immutable: cannot set %q on a %s node.", prop, n.Kind)
	e.Path = append(b.eng.Path(n.ID), "✗ "+prop+" =")
	e.Hint = "Every call returns a new node; keep the result in a variable."
	if _, ok := b.eng.Registry().Lookup(n.Kind, prop); ok {
		e.Fix = fmt.Sprintf("const next = node.%s(…)", prop)
	}
	b.throw(e)
	return nil
}

// __help(name) → help data
func (b *binding) help(c goja.FunctionCall) goja.Value {
	name := ""
	if a := c.Argument(0); !goja.IsUndefined(a) && !goja.IsNull(a) {
		name = a.String()
	}
	h, err := b.eng.Registry().Help(name)
	if err != nil {
		b.throw(err)
	}
	if name == "" {
		h["params"] = "P.slider(name, { min, max, value, step, label, unit }), P.toggle(name, { value }), P.choice(name, { options, value })"
		h["sources"] = toAny([]string{"cv.open(path)"})
	}
	return toJS(b.vm, h)
}

// __warnings(clear) → [{code, message}]
func (b *binding) warningsJS(c goja.FunctionCall) goja.Value {
	var ws []graph.Warning
	if c.Argument(0).ToBoolean() {
		ws = b.drainWarnings()
	} else {
		b.mu.Lock()
		ws = append(ws, b.warnings...)
		b.mu.Unlock()
	}
	out := make([]any, len(ws))
	for i, w := range ws {
		out[i] = map[string]any{"code": w.Code, "message": w.Message}
	}
	return toJS(b.vm, out)
}

var paramKeys = map[string][]string{
	"slider": {"min", "max", "value", "step", "label", "unit", "reset"},
	"toggle": {"value", "label", "reset"},
	"choice": {"options", "value", "label", "reset"},
}

// __param(type, name, opts) → {id, kind}
func (b *binding) param(c goja.FunctionCall) goja.Value {
	typ := c.Argument(0).String()
	name := c.Argument(1)
	if goja.IsUndefined(name) || name.String() == "" {
		b.throw(graph.Errorf(graph.EArgType, "P.%s needs a name.", typ).WithFix(fmt.Sprintf(`P.%s("ev", { … })`, typ)))
	}
	opts, _ := fromJS(c.Argument(2).Export()).(map[string]any)
	valid := paramKeys[typ]
	for _, k := range sortedKeys(opts) {
		if !contains(valid, k) {
			b.throw(graph.Errorf(graph.EUnknownOption, "P.%s has no option %q. Valid options: %s.", typ, k, strings.Join(valid, ", ")).WithValid(valid))
		}
	}
	def := graph.ParamDef{Name: name.String(), Type: typ, Value: opts["value"]}
	def.Label, _ = opts["label"].(string)
	def.Unit, _ = opts["unit"].(string)
	num := func(k string, required bool) float64 {
		v, ok := opts[k].(float64)
		if !ok && (required || opts[k] != nil) {
			b.throw(graph.Errorf(graph.EArgType, "P.%s(%q): %s must be a number.", typ, def.Name, k))
		}
		return v
	}
	switch typ {
	case "slider":
		def.Min, def.Max, def.Step = num("min", true), num("max", true), num("step", false)
	case "choice":
		list, ok := opts["options"].([]any)
		if !ok {
			b.throw(graph.Errorf(graph.EArgType, "P.choice(%q) needs options: [\"a\", \"b\"].", def.Name))
		}
		for _, o := range list {
			s, ok := o.(string)
			if !ok {
				b.throw(graph.Errorf(graph.EArgType, "P.choice(%q): options must be strings.", def.Name))
			}
			def.Options = append(def.Options, s)
		}
	}
	reset, _ := opts["reset"].(bool)
	n, _, err := b.eng.DeclareParam(def, reset)
	if err != nil {
		b.throw(err)
	}
	return handle(b.vm, n)
}

func paramData(d graph.ParamDef) map[string]any {
	m := map[string]any{"name": d.Name, "type": d.Type, "value": d.Value, "default": d.Default, "touched": d.Touched}
	if d.Type == "slider" {
		m["min"], m["max"] = d.Min, d.Max
		if d.Step != 0 {
			m["step"] = d.Step
		}
	}
	if len(d.Options) > 0 {
		m["options"] = toAny(d.Options)
	}
	if d.Label != "" {
		m["label"] = d.Label
	}
	if d.Unit != "" {
		m["unit"] = d.Unit
	}
	return m
}

// __paramInfo(id) → the param definition with its current value
func (b *binding) paramInfo(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	d, ok := b.eng.Params().Get(n.Args.String("name"))
	if !ok {
		b.throw(graph.Errorf(graph.EInternal, "param node #%d has no definition", n.ID))
	}
	return toJS(b.vm, paramData(d))
}

// __paramSet(name, value) moves a param, as the person would.
func (b *binding) paramSet(c goja.FunctionCall) goja.Value {
	if err := b.eng.Params().Set(c.Argument(0).String(), graph.Normalize(c.Argument(1).Export())); err != nil {
		b.throw(err)
	}
	return goja.Undefined()
}

// __params() → [definition…]
func (b *binding) params(goja.FunctionCall) goja.Value {
	list := b.eng.Params().List()
	out := make([]any, len(list))
	for i, d := range list {
		out[i] = paramData(d)
	}
	return toJS(b.vm, out)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(xs []string, x string) bool {
	for _, e := range xs {
		if e == x {
			return true
		}
	}
	return false
}
