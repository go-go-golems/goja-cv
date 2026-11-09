package cvmod

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/runtimebridge"
	"github.com/go-go-golems/goja-cv/pkg/cv/dataout"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// cv.save(path, x, { depth }) (GCV-004 §6.7, §7.8, §10.4): the one output
// function. Every check that needs no pixels runs here, on the owner
// goroutine (the path, E_DUPLICATE, the format against what is saved);
// encoding and writing run on a goroutine and settle a promise. Every save
// an eval starts is pending until it settles, and the session's eval runner
// waits for all of them (Pending, Settled): a save the agent did
// not await still completes, and its failure fails the eval.

// pendingSave is one save of the current eval.
type pendingSave struct {
	path string
	site Site
	user *goja.Promise // the promise the agent got
	done goja.Value    // resolved when the save settles (never rejected)
	err  error         // set when it settles with an error
}

// UnhandledWarning is the warning of a promise rejected without a handler
// during an eval (an unawaited failure).
const UnhandledWarning = "UNHANDLED"

// __save(path, x, opts) → Promise of the saved file. x is a node (its id)
// or plain data (exported in key order, so item links never reach a file).
func (b *binding) save(c goja.FunctionCall) goja.Value {
	ws := b.eng.Workspace()
	pathV := c.Argument(0)
	if goja.IsUndefined(pathV) || goja.IsNull(pathV) || pathV.ExportType() == nil || pathV.ExportType().Kind().String() != "string" {
		b.throw(graph.Errorf(graph.EArgType, "cv.save needs a file path first: cv.save(path, x).").WithFix(`await cv.save("candidates.json", x)`))
	}
	path := pathV.String()
	x := c.Argument(1)
	if goja.IsUndefined(x) {
		b.throw(graph.Errorf(graph.EArgType, "cv.save(%q) needs what to save: cv.save(path, x).", path).WithFix(fmt.Sprintf("await cv.save(%q, img)", path)))
	}
	depth := 0
	if o := exportArg(c.Argument(2)); o != nil {
		m, ok := fromJS(o).(map[string]any)
		if !ok {
			b.throw(graph.Errorf(graph.EArgType, "cv.save options must be an object such as { depth: 16 }.").WithFix(fmt.Sprintf("await cv.save(%q, x, { depth: 16 })", path)))
		}
		for k, v := range m {
			if k != "depth" {
				e := graph.Errorf(graph.EUnknownOption, "cv.save has no option %q; it takes depth.", k).WithValid([]string{"depth"})
				e.Fix = fmt.Sprintf("await cv.save(%q, x, { depth: 16 })", path)
				b.throw(e)
			}
			f, ok := v.(float64)
			if !ok {
				b.throw(graph.Errorf(graph.EArgType, "cv.save.depth must be a number of bits (8, 16, 32).").WithFix(fmt.Sprintf("await cv.save(%q, x, { depth: 16 })", path)))
			}
			depth = int(f)
		}
	}
	if ws == nil {
		b.throw(graph.Errorf(graph.EAccess, "cv.save(%q): this session has no workspace to write in.", path))
	}
	rel, err := ws.SaveName(path)
	if err != nil {
		b.throw(err)
	}
	format, err := dataout.FormatOf(rel)
	if err != nil {
		b.throw(err)
	}
	var node *graph.Node
	var plain any
	if id, ok := NodeOf(x); ok {
		n, found := b.eng.Node(id)
		if !found {
			b.throw(graph.Errorf(graph.EInternal, "unknown node #%d", id))
		}
		node = n
	} else {
		plain = exportOrdered(b.vm, x, 0, map[*goja.Object]bool{})
	}
	isImage := node != nil && node.Kind == graph.KindImage
	if err := dataout.CheckKind(rel, format, isImage); err != nil {
		b.throw(err)
	}
	if depth != 0 && !isImage {
		b.throw(graph.Errorf(graph.EUnknownOption, "cv.save(%q): depth is for images; data has none.", rel).WithFix(fmt.Sprintf("await cv.save(%q, x)", rel)))
	}
	if isImage {
		if err := dataout.CheckImage(b.eng, node.ID, rel, depth); err != nil {
			b.throw(err)
		}
	}
	site := b.site()
	b.mu.Lock()
	if b.track.active {
		if line, dup := b.track.saved[rel]; dup {
			b.mu.Unlock()
			b.throw(graph.Errorf(graph.EDuplicate, "cv.save(%q): this eval already saves %s (line %d).", path, rel, line).
				WithHint("One save per path per eval: save another name, or save once.").
				WithFix(fmt.Sprintf("await cv.save(%q, x)", withSuffix(rel, "-2"))))
		}
		b.track.saved[rel] = site.Line
	}
	b.mu.Unlock()
	return b.startSave(rel, site, func(ctx context.Context, warn func(graph.Warning)) (any, error) {
		var data []byte
		var s dataout.Saved
		switch {
		case isImage:
			var err error
			if data, s, err = dataout.EncodeImage(ctx, b.eng, node.ID, rel, depth, warn); err != nil {
				return nil, withSavePath(b, node, err)
			}
		default:
			v := plain
			if node != nil {
				var err error
				if v, err = dataout.NodeData(ctx, b.eng, node.ID); err != nil {
					return nil, withSavePath(b, node, err)
				}
			}
			out, rows, err := dataout.EncodeData(rel, format, v)
			if err != nil {
				return nil, err
			}
			data, s = out, dataout.Saved{Format: format, Bytes: len(out), Rows: rows}
		}
		replaced, err := ws.Write(rel, data)
		if err != nil {
			return nil, err
		}
		s.Path, s.Replaced = rel, replaced
		return s.Data(), nil
	})
}

func withSuffix(rel, suf string) string {
	ext := ""
	for i := len(rel) - 1; i >= 0 && rel[i] != '/'; i-- {
		if rel[i] == '.' {
			ext = rel[i:]
			rel = rel[:i]
			break
		}
	}
	return rel + suf + ext
}

func withSavePath(b *binding, n *graph.Node, err error) error {
	if ce, ok := graph.AsError(err); ok && len(ce.Path) == 0 {
		ce.Path = append(b.eng.Path(n.ID), "✗ save")
	}
	return err
}

// startSave runs fn on a goroutine under the eval's context and returns
// the agent's promise, settled on the owner goroutine; the save is pending
// in the current eval until then.
func (b *binding) startSave(rel string, site Site, fn func(ctx context.Context, warn func(graph.Warning)) (any, error)) goja.Value {
	vm := b.vm
	svc, ok := runtimebridge.Lookup(vm)
	if !ok || svc.Owner == nil {
		b.throw(graph.Errorf(graph.EInternal, "cv: runtime services missing (create the runtime with go-go-goja's engine)"))
	}
	user, resolve, reject := vm.NewPromise()
	done, resolveDone, _ := vm.NewPromise()
	ps := &pendingSave{path: rel, site: site, user: user, done: vm.ToValue(done)}
	b.mu.Lock()
	if b.track.active {
		b.track.saves = append(b.track.saves, ps)
		b.track.pending = append(b.track.pending, ps.done)
	}
	b.mu.Unlock()
	callCtx := b.evalContext()
	go func() {
		v, err := func() (v any, err error) { //nolint:nonamedreturns // recover needs named results
			defer graph.Recover(&err, "save")
			return fn(callCtx, func(w graph.Warning) { b.noteEvalWarning(w, site) })
		}()
		settleCtx := callCtx
		if callCtx.Err() != nil {
			settleCtx = svc.Lifetime()
		}
		_ = svc.PostWithCustomContext(settleCtx, "cv.save.settle", func(_ context.Context, vm *goja.Runtime) {
			if err != nil {
				b.mu.Lock()
				ps.err = err
				b.mu.Unlock()
				_ = reject(errorObject(vm, err))
			} else {
				_ = resolve(toJS(vm, v))
			}
			_ = resolveDone(goja.Undefined())
		})
	}()
	return vm.ToValue(user)
}

// trackRejections records promises rejected without a handler while an eval
// runs (goja's rejection tracker): an unawaited save that failed fails the
// eval; any other is an UNHANDLED warning.
func (b *binding) trackRejections() {
	b.vm.SetPromiseRejectionTracker(func(p *goja.Promise, op goja.PromiseRejectionOperation) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.unhandled == nil {
			b.unhandled = map[*goja.Promise]bool{}
		}
		switch op {
		case goja.PromiseRejectionReject:
			if b.track.active {
				b.unhandled[p] = true
				b.unhandledOrder = append(b.unhandledOrder, p)
			}
		case goja.PromiseRejectionHandle:
			delete(b.unhandled, p)
		}
	})
}

// Pending returns the promises the current eval waits for before it ends:
// cv.load's (the file's own promise) and one per save that resolves when
// the save settles. The eval runner waits for them until no new one
// appears, so a load or save started by a pending one is waited for too.
func Pending(vm *goja.Runtime) goja.Value {
	b, ok := lookupBinding(vm)
	if !ok {
		return vm.NewArray()
	}
	b.mu.Lock()
	vals := make([]any, len(b.track.pending))
	for i, p := range b.track.pending {
		vals[i] = p
	}
	b.mu.Unlock()
	return vm.NewArray(vals...)
}

// Settled is called by the eval runner once everything pending has
// settled. It returns the error of the first save that failed and that the
// agent did not handle (null when there is none), as a JavaScript error to
// throw; every other promise rejected without a handler during the eval
// becomes one UNHANDLED warning, located by its error's stack.
func Settled(vm *goja.Runtime) goja.Value {
	b, ok := lookupBinding(vm)
	if !ok {
		return goja.Null()
	}
	b.mu.Lock()
	var failed *pendingSave
	for _, s := range b.track.saves {
		if s.err != nil && b.unhandled[s.user] && failed == nil {
			failed = s
		}
		delete(b.unhandled, s.user)
	}
	rejected := make([]*goja.Promise, 0, len(b.unhandled))
	for _, p := range b.unhandledOrder {
		if b.unhandled[p] {
			rejected = append(rejected, p)
		}
	}
	b.unhandled, b.unhandledOrder = nil, nil
	b.mu.Unlock()
	var ws []SiteWarning
	for _, p := range rejected {
		msg, site := rejection(vm, p.Result())
		ws = append(ws, SiteWarning{Site: site, Warning: graph.Warning{Code: UnhandledWarning,
			Message: "a promise was rejected and never handled: " + msg + ". fix: await the call that returned it"}})
	}
	b.mu.Lock()
	b.track.warnings = append(b.track.warnings, ws...)
	b.mu.Unlock()
	if failed == nil {
		return goja.Null()
	}
	ce, _ := graph.AsError(failed.err)
	if ce == nil {
		ce = &graph.Error{Code: graph.EEval, Message: failed.err.Error()}
	}
	e := *ce
	e.Message = fmt.Sprintf("cv.save(%q) was not awaited and failed: %s", failed.path, ce.Message)
	obj := errorObject(vm, &e)
	if failed.site.Line > 0 { // the session reads the location from the stack
		src := failed.site.Source
		if src == "" {
			src = "<eval>"
		}
		_ = obj.Set("stack", fmt.Sprintf("Error: %s\n\tat %s:%d:%d(0)", e.Message, src, failed.site.Line, failed.site.Column))
	}
	return obj
}

// rejection is the message of a rejection reason and the eval or file
// position of its error (the innermost frame in eval code or a loaded file).
func rejection(vm *goja.Runtime, r goja.Value) (string, Site) {
	if r == nil || goja.IsUndefined(r) {
		return "undefined", Site{}
	}
	o, ok := r.(*goja.Object)
	if !ok || o.Get("message") == nil {
		return r.String(), Site{}
	}
	msg := strings.TrimSuffix(o.Get("message").String(), ".")
	if st := o.Get("stack"); st != nil {
		if fs := StackFrames(vm, st.String()); len(fs) > 0 {
			return msg, Site{Source: fs[0].Source, Line: fs[0].Line, Column: fs[0].Column}
		}
	}
	return msg, Site{}
}

// exportOrdered converts a JavaScript value into plain data for a file:
// objects become graph.Records in their key order (string keys only: item
// links stay out), arrays lists, nodes and looks their one-line
// description.
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
