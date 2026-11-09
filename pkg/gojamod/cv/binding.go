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
	track    tracker        // per-eval state (evaltrack.go)
	late     []QueuedLook   // looks created while no eval was running (evaltrack.go)
	opened   []graph.NodeID // sources opened with cv.open
	// itemProto is the prototype of region items (prelude: throwing
	// accessors for removed field names); owner goroutine only.
	itemProto *goja.Object
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
			arg := toJS(vm, a)
			if o, ok := arg.(*goja.Object); ok && b.itemProto != nil {
				if _, isRec := a.(graph.Record); isRec {
					_ = o.SetPrototype(b.itemProto)
				}
			}
			v, err := fn(goja.Undefined(), arg)
			if err != nil {
				if ce, ok := graph.AsError(jsError(err)); ok {
					return nil, ce
				}
				return nil, graph.Errorf(graph.EEval, "the function passed to filter threw on item %d: %v", i, err).
					WithHint(fmt.Sprintf("The function receives one region's data: { %s, … }.", strings.Join(graph.FieldNames(graph.GeometryFields), ", ")))
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
	set := func(name string, fn func(goja.FunctionCall) goja.Value) { _ = o.Set(name, b.export(name, fn)) }
	_ = o.Set("__version", Version)
	set("__registry", b.registry)
	set("__call", b.call)
	set("__open", b.open)
	set("__evaluate", b.evaluate)
	set("__explain", b.explain)
	set("__unknown", b.unknown)
	set("__notAwaited", b.notAwaited)
	set("__immutable", b.immutable)
	set("__help", b.help)
	set("__func", b.funcJS)
	set("__builders", b.builders)
	set("__placeholder", b.placeholder)
	set("__look", b.look)
	set("__save", b.saveNode)
	set("__data", b.dataJS)
	b.addDocs(o)
	set("__removedFields", b.removedFields)
	set("__noField", b.noField)
	set("__itemProto", b.itemProtoJS)
	set("__collision", b.collision)
	set("__notArray", b.notArray)
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

// jsNodeID reads a node id passed in from JS. Ids are positive; anything else
// is "no node" (0), which every lookup rejects.
func jsNodeID(v goja.Value) graph.NodeID {
	i := v.ToInteger()
	if i <= 0 {
		return 0
	}
	return graph.NodeID(i)
}

func (b *binding) nodeArg(v goja.Value) *graph.Node {
	id := jsNodeID(v)
	n, ok := b.eng.Node(id)
	if !ok {
		b.throw(graph.Errorf(graph.EInternal, "unknown node #%d", id))
	}
	return n
}

func handle(vm *goja.Runtime, n *graph.Node) goja.Value {
	o := vm.NewObject()
	_ = o.Set("id", int64(n.ID)) // #nosec G115 -- ids come from a sequential counter, far below MaxInt64
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

// __registry() → { Image: ["contrast", "exposure", …, "mask.luminance"], …,
// cv: ["compare", "sheet"] } (cv: the module ops the prelude exposes as
// cv.<name>)
func (b *binding) registry(goja.FunctionCall) goja.Value {
	out := map[string]any{}
	for _, k := range b.eng.Registry().Receivers() {
		out[string(k)] = toAny(b.eng.Registry().Methods(k))
	}
	out["cv"] = toAny(b.eng.Registry().ModuleOps())
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
	n, err := b.eng.Placeholder(jsNodeID(c.Argument(0)))
	if err != nil {
		b.throw(err)
	}
	return handle(b.vm, n)
}

// __call(op, receiverId, args[]) → {id, kind}, or an array of them for a
// macro that builds several nodes (split)
func (b *binding) call(c goja.FunctionCall) goja.Value {
	name := c.Argument(0).String()
	recv := jsNodeID(c.Argument(1))
	var raw []any
	if a := c.Argument(2); !goja.IsUndefined(a) && !goja.IsNull(a) {
		list, _ := fromJS(a.Export()).([]any)
		raw = list
	}
	r, err := b.eng.Call(name, recv, raw)
	if err != nil {
		b.throw(err)
	}
	b.addWarnings(r.Warnings)
	if len(r.Nodes) == 1 {
		b.noteCall(r.Nodes[0].ID, r.Warnings)
		return handle(b.vm, r.Nodes[0])
	}
	hs := make([]any, len(r.Nodes))
	for i, n := range r.Nodes {
		ws := r.Warnings
		if i > 0 {
			ws = nil
		}
		b.noteCall(n.ID, ws)
		hs[i] = handle(b.vm, n)
	}
	return b.vm.NewArray(hs...)
}

// __open(path, opts|null) → {id, kind}
func (b *binding) open(c goja.FunctionCall) goja.Value {
	p := c.Argument(0)
	if goja.IsUndefined(p) || p.ExportType() == nil || p.ExportType().Kind().String() != "string" {
		b.throw(graph.Errorf(graph.EArgType, "cv.open needs a file path string.").WithFix(`cv.open("scan.tif")`))
	}
	var opts map[string]any
	if o := c.Argument(1); !goja.IsUndefined(o) && !goja.IsNull(o) {
		m, ok := fromJS(o.Export()).(map[string]any)
		if !ok {
			b.throw(graph.Errorf(graph.EArgType, "cv.open options must be an object, got %s.", o.String()).
				WithFix(`cv.open("scan.tif", { encoding: "linear" })`))
		}
		opts = m
		if _, ok := m["as"]; ok {
			b.throw(graph.Errorf(graph.EUnknownOption, "cv.open has no option \"as\" here: names exist only in eval sessions.").
				WithHint("In a goja-cv eval session (CLI eval/run, the MCP server) cv.open(path, { as: \"ref\" }) binds a global; here keep the node in a variable.").
				WithFix(fmt.Sprintf("const ref = cv.open(%q)", p.String())))
		}
	}
	n, err := b.eng.Open(p.String(), opts)
	if err != nil {
		b.throw(err)
	}
	b.noteOpen(n.ID)
	return handle(b.vm, n)
}

// __evaluate(id) → Promise: await. A Value or Regions node is evaluated at
// scale 1 (data is exact); the promise is created here, on the owner
// goroutine, the engine runs on a goroutine and the result is converted and
// the promise settled in a callback posted back to the owner. An Image (or
// Mask) node resolves at once to its static summary: nothing is evaluated.
func (b *binding) evaluate(c goja.FunctionCall) goja.Value {
	vm := b.vm
	n := b.nodeArg(c.Argument(0))
	b.noteUsed(n.ID)
	promise, resolve, reject := vm.NewPromise()
	if n.Kind == graph.KindImage || n.Kind == graph.KindMask {
		_ = resolve(toJS(vm, graph.ImageSummary(n)))
		return vm.ToValue(promise)
	}
	svc, ok := runtimebridge.Lookup(vm)
	if !ok || svc.Owner == nil {
		b.throw(graph.Errorf(graph.EInternal, "cv: runtime services missing (create the runtime with go-go-goja's engine)"))
	}
	callCtx := b.evalContext()
	site := b.site()
	go func() {
		v, err := func() (v graph.Value, err error) { //nolint:nonamedreturns // recover needs named results
			defer graph.Recover(&err, "await")
			return b.eng.Evaluate(callCtx, n.ID, graph.EvalOptions{Scale: 1, OnWarn: func(w graph.Warning) { b.noteEvalWarning(w, site) }})
		}()
		settleCtx := callCtx
		if callCtx.Err() != nil {
			settleCtx = svc.Lifetime() // still reject, so awaiting code sees E_CANCELLED
		}
		_ = svc.PostWithCustomContext(settleCtx, "cv.evaluate.settle", func(_ context.Context, vm *goja.Runtime) {
			if err != nil {
				_ = reject(errorObject(vm, err))
				return
			}
			res, err := resultToJS(vm, v, n)
			if err != nil {
				ce, _ := graph.AsError(err)
				if ce != nil && len(ce.Path) == 0 {
					ce.Path = b.eng.Path(n.ID)
				}
				_ = reject(errorObject(vm, err))
				return
			}
			_ = resolve(res)
		})
	}()
	return vm.ToValue(promise)
}

func resultToJS(vm *goja.Runtime, v graph.Value, n *graph.Node) (goja.Value, error) {
	switch x := v.(type) {
	case *graph.RegionsValue:
		s, err := x.Summary()
		if err != nil {
			return nil, err
		}
		return toJS(vm, s), nil
	case *graph.DataValue:
		return toJS(vm, graph.OrderBySchema(x.Data, n.Fields)), nil
	}
	return nil, graph.Errorf(graph.EInternal, "%s evaluated to %T", graph.Label(n), v)
}

// __explain(id) → { id, kind, op, call, label, frame, space, path, graph }:
// cv.explain(node), synchronous; it evaluates nothing (graph lists each
// node's call, kind, size at scale 1 and whether its value is cached).
func (b *binding) explain(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	b.noteUsed(n.ID)
	d := map[string]any{
		"id": float64(n.ID), "kind": string(n.Kind), "op": graph.Shown(n).Op.Name, "label": graph.Label(n),
		"call":  graph.Describe(b.eng.Builder(), graph.Shown(n), 1),
		"frame": n.Frame.Data(),
		"path":  toAny(b.eng.Path(n.ID)),
		"graph": toAny(b.eng.Explain(n.ID)),
	}
	if n.Kind == graph.KindImage {
		d["encoding"], d["space"] = n.Enc.String(), n.Space.String()
	}
	if info, ok := graph.SourceInfo(n); ok {
		for k, v := range info {
			d[k] = v
		}
	}
	return toJS(b.vm, d)
}

// __unknown(kind, prop, receiverId) throws E_UNKNOWN_METHOD / E_NAMESPACE.
func (b *binding) unknown(c goja.FunctionCall) goja.Value {
	kind := graph.Kind(c.Argument(0).String())
	prop := c.Argument(1).String()
	var path []string
	if id := jsNodeID(c.Argument(2)); id != 0 {
		path = b.eng.Path(id)
	}
	b.throw(b.eng.Registry().UnknownMethod(kind, prop, path))
	return nil
}

// __notAwaited(id, prop, hint) throws E_NOT_AWAITED: a property read on a
// lazy Value or Regions node that is not one of its methods (prop), or a
// node used as a primitive (prop "", hint "number" or "primitive").
// Nothing has been computed; the fix awaits the node first.
func (b *binding) notAwaited(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	prop := c.Argument(1).String()
	if goja.IsUndefined(c.Argument(1)) || goja.IsNull(c.Argument(1)) {
		prop = ""
	}
	call := n.Op.Name + "()"
	expr := graph.AwaitExpr(n)
	if n.Kind == graph.KindImage || n.Kind == graph.KindMask {
		call = graph.Label(n)
	}
	var e *graph.Error
	switch {
	case prop == "":
		e = graph.Errorf(graph.ENotAwaited, "%s is a lazy node used as %s: nothing has been computed yet.", call, graph.A(c.Argument(2).String()))
		e.Hint = "Await the node first; awaited, a Value is its data and an Image its summary { kind, w, h, … }."
		if n.Kind == graph.KindImage || n.Kind == graph.KindMask {
			e.Fix = "(await img).w"
		} else {
			e.Fix = fmt.Sprintf("(await %s).%s", expr, sampleField(n))
		}
	case n.Kind == graph.KindRegions && b.takesRegions(prop):
		e = graph.Errorf(graph.ENotAwaited, "%s is a lazy Regions node and has no method %q: regions are arguments of image methods.", call, prop)
		e.Hint = "Pass the regions to the image method."
		e.Fix = fmt.Sprintf("img.%s(regions, …)", prop)
	case len(b.eng.Registry().Find(prop)) > 0 && !hasField(n, prop):
		// a method of another kind (stats().exposure, regions.feather): the cross-kind hint
		b.throw(b.eng.Registry().UnknownMethod(n.Kind, prop, b.eng.Path(n.ID)))
	default:
		e = graph.Errorf(graph.ENotAwaited, "%s is a lazy node: read .%s after awaiting it.", call, prop)
		e.Hint = "Nothing has been computed yet; awaiting the node evaluates it."
		e.Fix = fmt.Sprintf("(await %s).%s", expr, prop)
	}
	e.Path = b.eng.Path(n.ID)
	if prop != "" {
		e.Path = append(e.Path, "✗ "+prop)
	}
	b.throw(e)
	return nil
}

// sampleField is a field of node n's schema to show in a fix: p99 when it
// has one, else its first field.
func sampleField(n *graph.Node) string {
	for _, f := range n.Fields {
		if f.Name == "p99" {
			return "p99"
		}
	}
	if len(n.Fields) > 0 {
		return n.Fields[0].Name
	}
	if n.Kind == graph.KindRegions {
		return "items"
	}
	return "<field>"
}

func hasField(n *graph.Node, name string) bool {
	_, ok := graph.FindField(n.Fields, name)
	return ok
}

// takesRegions reports whether name is an Image method with a positional
// argument that takes regions (overlay, heal, measure): regions are passed
// to it, not called on.
func (b *binding) takesRegions(name string) bool {
	op, ok := b.eng.Registry().Lookup(graph.KindImage, name)
	if !ok {
		return false
	}
	for _, s := range op.Args {
		if s.Positional && strings.Contains(s.Type, string(graph.KindRegions)) {
			return true
		}
	}
	return false
}

// __immutable(id, prop) throws E_IMMUTABLE.
func (b *binding) immutable(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	prop := c.Argument(1).String()
	e := graph.Errorf(graph.EImmutable, "Nodes are immutable: cannot set %q on %s node.", prop, graph.A(string(n.Kind)))
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
	return toJS(b.vm, Help(b.eng.Registry(), name))
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
