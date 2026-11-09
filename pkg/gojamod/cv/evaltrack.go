package cvmod

import (
	"context"
	"fmt"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/runtimebridge"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/look"
)

// This file holds the hooks an eval session (pkg/cvsession) uses around one
// evaluation: per-eval tracking of the nodes a script builds and uses (for
// the UNUSED_NODE lint), warnings with the script line that caused them, the
// eval context that cancels kernels, the list of opened sources, node
// identification for result rendering, and look() objects. Everything here
// runs on the runtime owner goroutine.

const preludeName = "cv/prelude.js"

// Site is the position in the calling script (the first stack frame outside
// the cv prelude and native code) where a node was built or a warning raised.
type Site struct {
	Source string // script name; "" for code run with vm.RunString (eval cells)
	Line   int    // 1-based; 0 when unknown
	Column int
}

// NodeUse records a node built by a call during the current eval.
type NodeUse struct {
	ID   graph.NodeID
	Site Site
}

// SiteWarning is a warning with the script position of the call that raised it.
type SiteWarning struct {
	graph.Warning
	Site Site
}

// EvalTrace is what one eval did with nodes: the nodes its calls returned (in
// call order, first call per node), the nodes it evaluated or looked at, and
// the warnings its calls raised.
type EvalTrace struct {
	Created  []NodeUse
	Used     []graph.NodeID
	Warnings []SiteWarning
	// Collisions are data fields of awaited values that shadow a method of
	// their node (awaited Regions have count, Regions nodes have count()).
	Collisions []Collision
}

// Collision is one data field that shadows a node method after await.
type Collision struct {
	Name, Kind, Value string
}

type tracker struct {
	active   bool
	ctx      context.Context
	created  []NodeUse
	seen     map[graph.NodeID]bool
	used     map[graph.NodeID]bool
	usedList []graph.NodeID
	warnings []SiteWarning
	collide  []Collision
}

func lookupBinding(vm *goja.Runtime) (*binding, bool) {
	b, ok := bindings.Load(vm)
	if !ok {
		return nil, false
	}
	return b.(*binding), true
}

// BeginEval starts tracking for one evaluation in vm and makes ctx the
// context of every evaluation the script starts until EndEval (so cancelling
// the eval cancels kernels). It reports false if vm has no cv module.
func BeginEval(vm *goja.Runtime, ctx context.Context) bool {
	b, ok := lookupBinding(vm)
	if !ok {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.track = tracker{active: true, ctx: ctx, seen: map[graph.NodeID]bool{}, used: map[graph.NodeID]bool{}}
	return true
}

// EndEval stops tracking and returns the trace of the evaluation.
func EndEval(vm *goja.Runtime) EvalTrace {
	b, ok := lookupBinding(vm)
	if !ok {
		return EvalTrace{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.track
	b.track = tracker{}
	return EvalTrace{Created: t.created, Used: t.usedList, Warnings: t.warnings, Collisions: t.collide}
}

// MarkUsed records ids as used by the current evaluation.
func MarkUsed(vm *goja.Runtime, ids ...graph.NodeID) {
	if b, ok := lookupBinding(vm); ok {
		b.noteUsed(ids...)
	}
}

// Opened lists the source nodes opened in vm (cv.open), in opening order.
func Opened(vm *goja.Runtime) []graph.NodeID {
	b, ok := lookupBinding(vm)
	if !ok {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]graph.NodeID(nil), b.opened...)
}

// NodeOf reports whether v is a cv node proxy and returns its id. It reads
// the proxy target directly and never runs a proxy trap.
func NodeOf(v goja.Value) (graph.NodeID, bool) {
	obj, ok := v.(*goja.Object)
	if !ok || obj == nil {
		return 0, false
	}
	px, ok := obj.Export().(goja.Proxy)
	if !ok {
		return 0, false
	}
	target := px.Target()
	if target == nil {
		return 0, false
	}
	id, kind := target.Get("__id"), target.Get("__kind")
	if id == nil || kind == nil || goja.IsUndefined(id) || goja.IsUndefined(kind) {
		return 0, false
	}
	n := id.ToInteger()
	if n <= 0 {
		return 0, false
	}
	return graph.NodeID(n), true
}

func (b *binding) site() Site {
	frames := b.vm.CaptureCallStack(32, nil)
	for _, f := range frames {
		src := f.SrcName()
		if src == preludeName || src == "<native>" {
			continue
		}
		p := f.Position()
		return Site{Source: src, Line: p.Line, Column: p.Column}
	}
	return Site{}
}

// noteCall records a node returned by a call and the warnings it raised.
func (b *binding) noteCall(id graph.NodeID, ws []graph.Warning) {
	b.mu.Lock()
	active := b.track.active
	b.mu.Unlock()
	if !active {
		return
	}
	s := b.site()
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.track.seen[id] {
		b.track.seen[id] = true
		b.track.created = append(b.track.created, NodeUse{ID: id, Site: s})
	}
	for _, w := range ws {
		b.track.warnings = append(b.track.warnings, SiteWarning{Warning: w, Site: s})
	}
}

// noteEvalWarning records an evaluation-time warning (EvalCtx.Warn) with the
// site of the await or cv.evaluate that started the evaluation. It is called
// from evaluation goroutines.
func (b *binding) noteEvalWarning(w graph.Warning, s Site) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.track.active {
		return
	}
	for _, x := range b.track.warnings {
		if x.Warning == w && x.Site == s {
			return
		}
	}
	b.track.warnings = append(b.track.warnings, SiteWarning{Warning: w, Site: s})
}

func (b *binding) noteUsed(ids ...graph.NodeID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.track.active {
		return
	}
	for _, id := range ids {
		if !b.track.used[id] {
			b.track.used[id] = true
			b.track.usedList = append(b.track.usedList, id)
		}
	}
}

func (b *binding) noteOpen(id graph.NodeID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, x := range b.opened {
		if x == id {
			return
		}
	}
	b.opened = append(b.opened, id)
}

// evalContext is the context evaluations started now run under: the eval's
// context while one is tracked, otherwise the current owner call's context.
func (b *binding) evalContext() context.Context {
	b.mu.Lock()
	ctx := b.track.ctx
	b.mu.Unlock()
	if ctx != nil {
		return ctx
	}
	return runtimebridge.CurrentOwnerContext(b.vm)
}

// __look(id, opts) → a Go-backed *look.Look. opts is a label string or an
// options object, bound against the look function's ArgSpecs.
func (b *binding) look(c goja.FunctionCall) goja.Value {
	n := b.nodeArg(c.Argument(0))
	f, ok := b.eng.Registry().Func("look")
	if !ok {
		b.throw(graph.Errorf(graph.EInternal, "no look function registered"))
	}
	var raw []any
	if x := exportArg(c.Argument(1)); x != nil {
		raw = []any{fromJS(x)}
	}
	a, _, err := graph.BindOptions(f.CallName(), f.Args, raw)
	if err != nil {
		b.throw(err)
	}
	opt, err := look.OptionsFrom(a)
	if err != nil {
		b.throw(err)
	}
	l, err := look.New(b.eng, n.ID, opt)
	if err != nil {
		if ce, ok := graph.AsError(err); ok && len(ce.Path) == 0 {
			ce.Path = append(b.eng.Path(n.ID), "✗ look")
		}
		b.throw(err)
	}
	b.noteUsed(n.ID)
	return b.vm.ToValue(l)
}

func exportArg(v goja.Value) any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	return v.Export()
}

// LookOf reports whether v is a Go-backed look object.
func LookOf(v goja.Value) (*look.Look, bool) {
	obj, ok := v.(*goja.Object)
	if !ok || obj == nil {
		return nil, false
	}
	l, ok := obj.Export().(*look.Look)
	return l, ok && l != nil
}

// DescribeNode is the one-line summary of node id used in result rendering:
// "[Image #4 gaussian(sigma: 2 px)]".
func DescribeNode(eng *graph.Engine, id graph.NodeID) string {
	n, ok := eng.Node(id)
	if !ok {
		return fmt.Sprintf("[node #%d]", id)
	}
	scale := eng.PreviewScale(id)
	return fmt.Sprintf("[%s #%d %s]", n.Kind, n.ID, graph.Describe(eng.Builder(), n, scale))
}

// ENotArray is the error code of array methods called on awaited Regions.
const ENotArray = "E_NOT_ARRAY"

// __collision(name, kind, value) records a data field that shadows a node
// method of the same name (for the TypeError translation of the session).
func (b *binding) collision(c goja.FunctionCall) goja.Value {
	col := Collision{Name: c.Argument(0).String(), Kind: c.Argument(1).String(), Value: c.Argument(2).String()}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.track.active {
		return goja.Undefined()
	}
	for _, x := range b.track.collide {
		if x.Name == col.Name {
			return goja.Undefined()
		}
	}
	b.track.collide = append(b.track.collide, col)
	return goja.Undefined()
}

// __notArray(member) throws E_NOT_ARRAY: awaited Regions are an object with
// items, not an array. RECV in the fix is replaced by the session with the
// receiver expression from the failing line.
func (b *binding) notArray(c goja.FunctionCall) goja.Value {
	member := c.Argument(0).String()
	e := graph.Errorf(ENotArray, "Awaited Regions are { count, items: [...] }, not an array.").
		WithHint("The list of regions is .items; count is .count.")
	if member == "iterator" {
		e.Fix = "for (const r of regions.items) { … }"
	} else {
		e.Fix = "RECV.items." + member + "(…)"
	}
	obj := errorObject(b.vm, e)
	_ = obj.Set("member", member)
	panic(obj)
}
