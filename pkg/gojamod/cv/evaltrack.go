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
// evaluation: warnings with the script line that caused them, the eval
// context that cancels kernels, the list of opened sources, node
// identification for result rendering, and look() objects. Everything here
// runs on the runtime owner goroutine.

const preludeName = "cv/prelude.js"

// Site is the position in the calling script (the first stack frame outside
// the cv prelude and native code) where a warning was raised or a look made.
type Site struct {
	Source string // script name; "" for code run with vm.RunString (eval cells)
	Line   int    // 1-based; 0 when unknown
	Column int
}

// SiteWarning is a warning with the script position of the call that raised it.
type SiteWarning struct {
	graph.Warning
	Site Site
}

// QueuedLook is a look created by node.look(), with the call site.
type QueuedLook struct {
	Look *look.Look
	Site Site
}

// EvalTrace is what one eval did: the warnings its calls raised, and the
// looks it created (in creation order; every look created in an eval is
// delivered, up to the session's budget). Late are looks created between
// evals (in a .then that ran after the previous eval settled): they were not
// delivered.
type EvalTrace struct {
	Warnings []SiteWarning
	Looks    []QueuedLook
	Late     []QueuedLook
}

type tracker struct {
	active   bool
	ctx      context.Context
	warnings []SiteWarning
	looks    []QueuedLook
	saved    map[string]int // workspace-relative path → line of its save (E_DUPLICATE)
	saves    []*pendingSave
	pending  []goja.Value // promises the eval waits for: cv.load's, and each save's settling
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
	b.track = tracker{active: true, ctx: ctx, saved: map[string]int{}}
	b.unhandled, b.unhandledOrder = nil, nil
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
	late := b.late
	b.late = nil
	return EvalTrace{Warnings: t.warnings, Looks: t.looks, Late: late}
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

// noteCall records the warnings a call raised, with the call's site.
func (b *binding) noteCall(ws []graph.Warning) {
	if len(ws) == 0 {
		return
	}
	b.mu.Lock()
	active := b.track.active
	b.mu.Unlock()
	if !active {
		return
	}
	s := b.site()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, w := range ws {
		b.track.warnings = append(b.track.warnings, SiteWarning{Warning: w, Site: s})
	}
}

// noteEvalWarning records an evaluation-time warning (EvalCtx.Warn) with the
// site of the await that started the evaluation. It is called
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

// __look(id, opts) → a Go-backed *look.Look, queued for delivery after the
// eval. opts is a label string or an options object, bound against the look
// function's ArgSpecs.
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
	b.noteLook(l)
	return b.vm.ToValue(l)
}

// noteLook queues a look for delivery: in the current eval's trace, or, when
// no eval is running (a .then after the eval settled), as a late look the
// next eval's warnings name.
func (b *binding) noteLook(l *look.Look) {
	q := QueuedLook{Look: l, Site: b.site()}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.track.active {
		b.track.looks = append(b.track.looks, q)
		return
	}
	b.late = append(b.late, q)
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
	return fmt.Sprintf("[%s #%d %s]", n.Kind, n.ID, graph.Describe(eng.Builder(), graph.Shown(n), 1))
}
