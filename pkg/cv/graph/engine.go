package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/pkg/errors"
)

// EngineOptions configures an Engine. Zero values select the defaults.
type EngineOptions struct {
	CacheBytes     int64  // byte cap of the value cache (default 2 GiB)
	PreviewLong    int    // long side of the preview scale (default 1600)
	LookLong       int    // long side of the look scale (default 1568)
	PyramidMinLong int    // pyramids stop at this long side (default 256)
	BaseDir        string // relative paths given to Open resolve against it (default: cwd)
}

func (o EngineOptions) withDefaults() EngineOptions {
	if o.CacheBytes <= 0 {
		o.CacheBytes = 2 << 30
	}
	if o.PreviewLong <= 0 {
		o.PreviewLong = 1600
	}
	if o.LookLong <= 0 {
		o.LookLong = 1568
	}
	if o.PyramidMinLong <= 0 {
		o.PyramidMinLong = 256
	}
	return o
}

// Engine is one session's cv engine: registry, graph, params, sources,
// cache and evaluator.
type Engine struct {
	opts EngineOptions
	reg  *Registry
	b    *Builder
	par  *ParamStore
	ev   *Evaluator
}

// NewEngine creates an engine over reg.
func NewEngine(reg *Registry, opts EngineOptions) *Engine {
	opts = opts.withDefaults()
	b := NewBuilder()
	par := NewParamStore()
	ev := NewEvaluator(b, par, NewCache(opts.CacheBytes), NewSourceStore(opts.PyramidMinLong))
	return &Engine{opts: opts, reg: reg, b: b, par: par, ev: ev}
}

func (e *Engine) Options() EngineOptions       { return e.opts }
func (e *Engine) Registry() *Registry          { return e.reg }
func (e *Engine) Builder() *Builder            { return e.b }
func (e *Engine) Params() *ParamStore          { return e.par }
func (e *Engine) Evaluator() *Evaluator        { return e.ev }
func (e *Engine) Cache() *Cache                { return e.ev.cache }
func (e *Engine) Node(id NodeID) (*Node, bool) { return e.b.Node(id) }

// Call interns op name on receiver (0 for source ops) with raw arguments.
// An unknown name yields E_UNKNOWN_METHOD / E_NAMESPACE with suggestions.
func (e *Engine) Call(name string, receiver NodeID, raw []any) (*Node, []Warning, error) {
	kind := KindNone
	if receiver != 0 {
		n, ok := e.b.Node(receiver)
		if !ok {
			return nil, nil, Errorf(EInternal, "unknown node #%d", receiver)
		}
		kind = n.Kind
	}
	op, ok := e.reg.Lookup(kind, name)
	if !ok {
		var path []string
		if receiver != 0 {
			path = e.ev.Path(receiver)
		}
		return nil, nil, e.reg.UnknownMethod(kind, name, path)
	}
	return e.b.Intern(op, receiver, raw, nil)
}

// Open interns a source node for an image file. The header is read now (for
// the doc size); pixels are decoded on first evaluation. The node identity
// includes the absolute path and the file's size and modification time.
func (e *Engine) Open(path string) (*Node, error) {
	op, ok := e.reg.Lookup(KindNone, "open")
	if !ok {
		return nil, Errorf(EInternal, "no source op registered")
	}
	abs := path
	if !filepath.IsAbs(abs) {
		base := e.opts.BaseDir
		if base == "" {
			wd, err := os.Getwd()
			if err != nil {
				return nil, errors.Wrap(err, "getwd")
			}
			base = wd
		}
		abs = filepath.Join(base, path)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, Errorf(EArgType, "cannot open %q: %v", path, err)
	}
	info, err := buffer.DecodeConfigFile(abs)
	if err != nil {
		return nil, Errorf(EArgType, "cannot read %q as an image: %v", path, errors.Cause(err)).
			WithHint("Supported formats: PNG, JPEG, TIFF (8/16-bit).")
	}
	stamp := fmt.Sprintf("%d-%d", st.Size(), st.ModTime().UnixNano())
	n, _, err := e.b.Intern(op, 0, []any{abs}, map[string]any{"stamp": stamp, "w": info.W, "h": info.H})
	return n, err
}

// DeclareParam declares (or re-declares) a param and returns its node.
func (e *Engine) DeclareParam(def ParamDef, reset bool) (*Node, ParamDef, error) {
	op, ok := e.reg.Lookup(KindNone, "param")
	if !ok {
		return nil, ParamDef{}, Errorf(EInternal, "no param op registered")
	}
	d, err := e.par.Declare(def, reset)
	if err != nil {
		return nil, ParamDef{}, err
	}
	n, _, err := e.b.Intern(op, 0, []any{def.Name}, nil)
	return n, d, err
}

// PreviewScale is min(1, PreviewLong / long side) for node id.
func (e *Engine) PreviewScale(id NodeID) float64 { return e.ScaleForLong(id, e.opts.PreviewLong) }

// ScaleForLong is min(1, long / the node's doc long side); 1 for data nodes.
func (e *Engine) ScaleForLong(id NodeID, long int) float64 {
	n, ok := e.b.Node(id)
	if !ok {
		return 1
	}
	l := max(n.DocW, n.DocH)
	if l <= 0 || l <= long {
		return 1
	}
	return QuantizeScale(float64(long) / float64(l))
}

// Evaluate computes node id; a zero opt.Scale means the preview scale.
func (e *Engine) Evaluate(ctx context.Context, id NodeID, opt EvalOptions) (Value, error) {
	if opt.Scale == 0 {
		opt.Scale = e.PreviewScale(id)
	}
	return e.ev.Evaluate(ctx, id, opt)
}

// Explain renders the graph under id at scale (0 = preview).
func (e *Engine) Explain(id NodeID, scale float64) []string {
	if scale == 0 {
		scale = e.PreviewScale(id)
	}
	return e.ev.Explain(id, scale)
}

// Path is the receiver chain of node id.
func (e *Engine) Path(id NodeID) []string { return e.ev.Path(id) }
