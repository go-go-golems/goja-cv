package graph

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"runtime/debug"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

// EvalOptions selects how a node is evaluated.
type EvalOptions struct {
	// Scale is buffer px per doc px, in (0, 1]. It is quantized to 1e-4.
	Scale float64
}

// EvalCtx is what an op's Run sees besides its inputs and args.
type EvalCtx struct {
	Eval   *Evaluator
	Node   *Node
	Scale  float64        // quantized evaluation scale
	Params map[string]any // param values snapshotted at the start of the evaluation
}

// Callbacks runs JavaScript data lambdas for ops such as Regions.filter.
// The binding implements it by scheduling onto the runtime owner goroutine;
// evaluation never runs on that goroutine, so the call can be served.
type Callbacks interface {
	// CallEach calls function id once per element of args and returns the
	// results as plain data.
	CallEach(ctx context.Context, id FuncID, args []any) ([]any, error)
}

// Evaluator computes node values with signature-keyed caching, shared
// in-flight work, the identity short-circuit and parallel input evaluation.
type Evaluator struct {
	b       *Builder
	params  *ParamStore
	cache   *Cache
	sources *SourceStore
	flight  singleflight.Group
	cbs     Callbacks

	mu    sync.Mutex
	runs  map[NodeID]int
	total int
}

// NewEvaluator wires an evaluator to its graph, params, cache and sources.
func NewEvaluator(b *Builder, params *ParamStore, cache *Cache, sources *SourceStore) *Evaluator {
	return &Evaluator{b: b, params: params, cache: cache, sources: sources, runs: map[NodeID]int{}}
}

// SetCallbacks installs the runner for JavaScript data lambdas.
func (e *Evaluator) SetCallbacks(c Callbacks) {
	e.mu.Lock()
	e.cbs = c
	e.mu.Unlock()
}

// CallEach runs data lambda id on each of args through the installed
// Callbacks.
func (e *Evaluator) CallEach(ctx context.Context, id FuncID, args []any) ([]any, error) {
	e.mu.Lock()
	c := e.cbs
	e.mu.Unlock()
	if c == nil {
		return nil, Errorf(EInternal, "no JavaScript runtime is attached to run data lambdas")
	}
	return c.CallEach(ctx, id, args)
}

func (e *Evaluator) Builder() *Builder            { return e.b }
func (e *Evaluator) Params() *ParamStore          { return e.params }
func (e *Evaluator) Cache() *Cache                { return e.cache }
func (e *Evaluator) Sources() *SourceStore        { return e.sources }
func (e *Evaluator) node(id NodeID) *Node         { n, _ := e.b.Node(id); return n }
func (e *Evaluator) Node(id NodeID) (*Node, bool) { return e.b.Node(id) }

// Runs is how many times node id's op has actually run (cache misses that
// computed). Tests use it to assert what recomputed.
func (e *Evaluator) Runs(id NodeID) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runs[id]
}

// TotalRuns is the number of op runs since creation.
func (e *Evaluator) TotalRuns() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.total
}

func (e *Evaluator) countRun(id NodeID) {
	e.mu.Lock()
	e.runs[id]++
	e.total++
	e.mu.Unlock()
}

// QuantizeScale rounds a scale to 1e-4 (minimum 1e-4) so that nearly equal
// scales share cache entries and produce identical buffer sizes.
func QuantizeScale(s float64) float64 {
	q := math.Round(s*1e4) / 1e4
	if q < 1e-4 {
		q = 1e-4
	}
	return q
}

// evalRun is the state of one Evaluate call at one scale: memoized
// signatures and values. Nodes with an AnalysisScale are evaluated in a
// sibling run at that scale, which shares the param snapshot.
type evalRun struct {
	e      *Evaluator
	scale  float64
	params map[string]any
	group  *runGroup
	mu     sync.Mutex
	sigs   map[NodeID]Signature
	vals   map[Signature]Value
}

// runGroup holds the runs of one Evaluate call, one per scale.
type runGroup struct {
	mu   sync.Mutex
	runs map[float64]*evalRun
}

func (e *Evaluator) newRun(scale float64) *evalRun {
	r := &evalRun{e: e, scale: QuantizeScale(scale), params: e.params.Snapshot(),
		sigs: map[NodeID]Signature{}, vals: map[Signature]Value{}}
	r.group = &runGroup{runs: map[float64]*evalRun{r.scale: r}}
	return r
}

// at returns the run of the same evaluation at scale (quantized).
func (r *evalRun) at(scale float64) *evalRun {
	if scale == r.scale {
		return r
	}
	r.group.mu.Lock()
	defer r.group.mu.Unlock()
	if s, ok := r.group.runs[scale]; ok {
		return s
	}
	s := &evalRun{e: r.e, scale: scale, params: r.params, group: r.group,
		sigs: map[NodeID]Signature{}, vals: map[Signature]Value{}}
	r.group.runs[scale] = s
	return s
}

// scaleFor is the scale node n is evaluated at in run r: its AnalysisScale
// if it has one, else r's scale.
func (r *evalRun) scaleFor(n *Node) float64 {
	if n.Op.AnalysisScale == nil {
		return r.scale
	}
	s := n.Op.AnalysisScale(n.Args)
	if !(s > 0) {
		return r.scale
	}
	return QuantizeScale(math.Min(s, 1))
}

// Evaluate computes node id at opt.Scale.
func (e *Evaluator) Evaluate(ctx context.Context, id NodeID, opt EvalOptions) (Value, error) {
	if !(opt.Scale > 0 && opt.Scale <= 1) {
		return nil, Errorf(EArgRange, "evaluation scale %g is outside (0, 1].", opt.Scale)
	}
	if _, ok := e.b.Node(id); !ok {
		return nil, Errorf(EInternal, "unknown node #%d", id)
	}
	return e.newRun(opt.Scale).eval(ctx, id)
}

// Signature returns the cache key of node id at scale with the current params.
func (e *Evaluator) Signature(id NodeID, scale float64) Signature {
	return e.newRun(scale).sig(id)
}

func (r *evalRun) sig(id NodeID) Signature {
	r.mu.Lock()
	s, ok := r.sigs[id]
	r.mu.Unlock()
	if ok {
		return s
	}
	n := r.e.node(id)
	if fs := r.scaleFor(n); fs != r.scale {
		s = r.at(fs).sig(id)
		r.mu.Lock()
		r.sigs[id] = s
		r.mu.Unlock()
		return s
	}
	h := sha256.New()
	h.Write(n.Hash[:])
	_ = binary.Write(h, binary.LittleEndian, int64(math.Round(r.scale*1e4)))
	if n.Op.On == KindNone && n.Op.Name == "param" {
		js, _ := json.Marshal(r.params[n.Args.String("name")])
		h.Write(js)
	}
	for _, in := range n.Inputs {
		is := r.sig(in)
		h.Write(is[:])
	}
	copy(s[:], h.Sum(nil))
	r.mu.Lock()
	r.sigs[id] = s
	r.mu.Unlock()
	return s
}

func (r *evalRun) eval(ctx context.Context, id NodeID) (Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, ctxError(err)
	}
	n := r.e.node(id)
	if fs := r.scaleFor(n); fs != r.scale {
		return r.at(fs).eval(ctx, id)
	}
	if n.Op.NoCache {
		return r.compute(ctx, n, Signature{})
	}
	sig := r.sig(id)
	r.mu.Lock()
	v, ok := r.vals[sig]
	r.mu.Unlock()
	if ok {
		return v, nil
	}
	if v, ok := r.e.cache.Get(sig); ok {
		r.memo(sig, v)
		return v, nil
	}
	res, err, _ := r.e.flight.Do(string(sig[:]), func() (any, error) {
		if v, ok := r.e.cache.Get(sig); ok {
			return v, nil
		}
		return r.compute(ctx, n, sig)
	})
	if err != nil {
		return nil, err
	}
	v = res.(Value)
	r.memo(sig, v)
	return v, nil
}

func (r *evalRun) memo(sig Signature, v Value) {
	r.mu.Lock()
	r.vals[sig] = v
	r.mu.Unlock()
}

// compute evaluates inputs in parallel, applies the identity short-circuit,
// runs the op and caches the result (unless NoCache).
func (r *evalRun) compute(ctx context.Context, n *Node, sig Signature) (Value, error) {
	ins := make([]Value, len(n.Inputs))
	if !n.Op.Lazy && len(n.Inputs) > 0 {
		g, gctx := errgroup.WithContext(ctx)
		for i, in := range n.Inputs {
			g.Go(func() error {
				v, err := r.eval(gctx, in)
				ins[i] = v
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}
	var args Args
	if n.Op.Lazy {
		args = n.Args
	} else {
		var err error
		if args, err = resolveArgs(n, ins); err != nil {
			return nil, r.e.wrapWithPath(err, n)
		}
	}
	if n.Op.Identity != nil && n.NExplicit > 0 && n.Op.Identity(args) {
		return stamp(n, ins[0], ins), nil // neutral args: the receiver's value (a stamped copy); not cached twice
	}
	ec := &EvalCtx{Eval: r.e, Node: n, Scale: r.scale, Params: r.params}
	out, err := safeRun(ctx, n.Op, ec, ins[:n.NExplicit], args)
	if err != nil {
		return nil, r.e.wrapWithPath(err, n)
	}
	if out == nil {
		return nil, r.e.wrapWithPath(Errorf(EInternal, "%s returned no value", n.Op.Name), n)
	}
	out = stamp(n, out, ins)
	r.e.countRun(n.ID)
	if !n.Op.NoCache {
		r.e.cache.Put(sig, out)
	}
	return out, nil
}

// stamp sets node n's statics (frame) on v, the value its Run returned. Run
// only creates values; the evaluator is the one place that gives them their
// node's statics. A value that belongs to one of the inputs (the identity
// short-circuit, an op that returns its input unchanged) is copied first, so
// the cached value of another node is never rewritten.
func stamp(n *Node, v Value, ins []Value) Value {
	isInput := func(v Value) bool {
		for _, in := range ins {
			if in == v {
				return true
			}
		}
		return false
	}
	switch x := v.(type) {
	case *ImageValue:
		if isInput(v) {
			c := *x
			x = &c
		}
		x.Frame = n.Frame
		return x
	case *RegionsValue:
		if isInput(v) {
			c := *x
			x = &c
		}
		x.Frame = n.Frame
		return x
	}
	return v
}

func safeRun(ctx context.Context, op *OpDef, ec *EvalCtx, in []Value, args Args) (v Value, err error) { //nolint:nonamedreturns // recover needs named results
	defer func() {
		if p := recover(); p != nil {
			err = Errorf(EInternal, "op %s panicked: %v", op.Name, p).WithHint(string(debug.Stack()))
		}
	}()
	return op.Run(ctx, ec, in, args)
}

func ctxError(err error) *Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: ETimeout, Message: "The evaluation exceeded its time limit.", cause: err}
	}
	return &Error{Code: ECancelled, Message: "The evaluation was cancelled.", cause: err}
}

// wrapWithPath attaches the graph path of n to err unless an inner node
// already did.
func (e *Evaluator) wrapWithPath(err error, n *Node) error {
	ce, ok := AsError(err)
	if ok && len(ce.Path) > 0 {
		return err
	}
	if !ok {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			ce = ctxError(err)
		} else {
			ce = &Error{Code: EEval, Message: fmt.Sprintf("%s failed: %v", n.Op.Name, err), cause: err}
		}
	} else {
		cp := *ce
		ce = &cp
	}
	path := e.Path(n.ID)
	path[len(path)-1] = pathFailureMarker + path[len(path)-1]
	ce.Path = path
	return ce
}

// Path is the chain of receivers from the source to node id, as labels.
func (e *Evaluator) Path(id NodeID) []string {
	var rev []string
	for n := e.node(id); n != nil; {
		rev = append(rev, Label(n))
		if n.NExplicit == 0 || len(n.Inputs) == 0 {
			break
		}
		n = e.node(n.Inputs[0])
	}
	out := make([]string, len(rev))
	for i, l := range rev {
		out[len(rev)-1-i] = l
	}
	return out
}
