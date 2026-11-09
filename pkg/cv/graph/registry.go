// Package graph holds the cv op registry, the hash-consed node graph, the
// param store and the caching evaluator.
package graph

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

// Kind is the kind of value a node evaluates to.
type Kind string

const (
	KindImage   Kind = "Image"
	KindMask    Kind = "Mask"
	KindRegions Kind = "Regions"
	KindValue   Kind = "Value"
	KindParam   Kind = "Param"
	// KindNone is the receiver "kind" of source ops (cv.open, P.slider).
	KindNone Kind = ""
)

// Kinds lists every kind in display order.
var Kinds = []Kind{KindImage, KindMask, KindRegions, KindValue, KindParam}

// accepts reports whether a node of kind k satisfies a declared input kind
// such as "Image" or "Regions|Mask". Param satisfies Value.
func accepts(decl Kind, k Kind) bool {
	for _, d := range strings.Split(string(decl), "|") {
		if Kind(d) == k || (Kind(d) == KindValue && k == KindParam) {
			return true
		}
	}
	return false
}

// Unit tags numeric arguments.
type Unit int

const (
	Unitless Unit = iota
	DocPx         // length or coordinate in original-image pixels; scaled per evaluation
	Fraction      // 0..1 of width/height
	DocPx2        // area in original-image pixels²; scaled by scale² per evaluation
)

func (u Unit) String() string {
	switch u {
	case DocPx:
		return "doc px"
	case Fraction:
		return "fraction"
	case Unitless:
		return "unitless"
	case DocPx2:
		return "doc px²"
	}
	return "unknown"
}

// ArgSpec declares one argument of an op.
//
// Types: "number", "string", "bool", "enum:a,b,c", "number[]", "string[]",
// "box" ([x, y, w, h] or an object with x, y, w, h), "point" ([x, y] or an
// object with cx, cy), unions of these such as "point|number", "object"
// (with Options as the allowed keys), "any", and node kinds such as
// "Mask", "Regions|Mask" or "Image[]": a kind-typed argument holds a node of
// that kind (lifted into the node's inputs like a Value argument); Run reads
// its evaluated value with Args.Node / Args.Nodes.
type ArgSpec struct {
	Name       string
	Type       string
	Unit       Unit
	Default    any        // filled when the argument is absent; nil means required unless Optional
	Optional   bool       // may be absent without a default
	Positional bool       // may be passed positionally (in declaration order)
	Variadic   bool       // a positional "...[]" argument that takes all remaining positional values
	Hidden     bool       // set by Go only (e.g. a source file stamp); not settable or listed
	Static     bool       // must be a literal: a node (Value/Param) is rejected
	Len        int        // for list types: the exact number of elements
	InFrame    bool       // "box"/"point": a box must overlap, a point lie in, the receiver's frame (E_ARG_RANGE)
	Min, Max   *float64   // hard limits: violation is E_ARG_RANGE
	Usual      [2]float64 // soft range (zero value = none): violation is a RANGE warning
	Options    []ArgSpec  // for "object": the allowed keys (unknown keys are E_UNKNOWN_OPTION)
	Doc        string
}

func (s *ArgSpec) hasUsual() bool { return s.Usual != [2]float64{} }

// F returns a pointer to v, for Min and Max.
func F(v float64) *float64 { return &v }

// RunFunc computes an op. in holds the evaluated explicit inputs (the
// receiver first, then OpDef.Inputs); node-valued arguments have already been
// evaluated and substituted into args.
type RunFunc func(ctx context.Context, ec *EvalCtx, in []Value, args Args) (Value, error)

// OpDef describes one registered op.
type OpDef struct {
	Name     string // "exposure", "mask.luminance": a dot puts the op in a namespace
	On       Kind   // receiver kind; KindNone for source ops
	Inputs   []Kind // extra node inputs beyond the receiver, passed positionally first
	Args     []ArgSpec
	Returns  Kind
	Space    string                                // "linear" | "perceptual" | "geometry" | "analysis" | "source" | "data"
	Identity func(args Args) bool                  // neutral arguments: the receiver is returned unchanged
	Halo     func(args Args) float64               // doc px of border a tile needs (for tiled export; unused in M1)
	Global   bool                                  // needs the whole image (stats, histogram)
	NoCache  bool                                  // never cached (explain reads cache state)
	Lazy     bool                                  // inputs are not evaluated before Run (explain)
	DocFrame func(args Args, inputs []*Node) Frame // the result's frame; nil: the first input with a non-empty frame
	Fields   []string                              // documented fields of a Value result (help, hints)
	// Validate checks bound args at build time (after defaults and types);
	// lifted values are still InputRefs. Errors fail the call.
	Validate func(args Args) error
	// AnalysisScale, when set and > 0, is the scale at which the node's
	// inputs are evaluated whatever the evaluation scale: the node's value is
	// then the same at preview and export. Regions ops use it so that region
	// ids and doc-px coordinates are identities, not artefacts of the scale.
	AnalysisScale func(args Args) float64
	// Builders lists the positions (0-based, among the call's arguments) that
	// may be builder lambdas: the JavaScript surface calls the function once
	// with a placeholder for the receiver and passes the result as a
	// BuiltArg (within(mask, i => i.exposure(-0.4))).
	Builders []int
	Run      RunFunc
	Doc      string
	See      []string
}

// Key is the registry key "On:Name".
func (op *OpDef) Key() string { return string(op.On) + ":" + op.Name }

func (op *OpDef) spec(name string) *ArgSpec {
	for i := range op.Args {
		if op.Args[i].Name == name {
			return &op.Args[i]
		}
	}
	return nil
}

// Registry is the single source of truth for every op the agent can call.
type Registry struct {
	mu  sync.RWMutex
	ops map[string]*OpDef
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{ops: map[string]*OpDef{}}
}

// Register validates def and adds it.
func (r *Registry) Register(def OpDef) error {
	if def.Name == "" {
		return errors.New("op without name")
	}
	if def.Run == nil {
		return errors.Errorf("op %s has no Run", def.Name)
	}
	if def.Returns == KindNone {
		return errors.Errorf("op %s has no Returns kind", def.Name)
	}
	seen := map[string]bool{}
	sawOptional := false
	for i, a := range def.Args {
		if seen[a.Name] {
			return errors.Errorf("op %s: duplicate arg %s", def.Name, a.Name)
		}
		seen[a.Name] = true
		if a.Variadic && (!a.Positional || i != lastPositional(def.Args)) {
			return errors.Errorf("op %s: variadic arg %s must be the last positional arg", def.Name, a.Name)
		}
		if a.Positional {
			if sawOptional && a.Default == nil && !a.Optional && !a.Variadic {
				return errors.Errorf("op %s: required positional %s after optional", def.Name, a.Name)
			}
			if a.Default != nil || a.Optional {
				sawOptional = true
			}
		}
		if a.Type == "" {
			return errors.Errorf("op %s: arg %s has no type", def.Name, a.Name)
		}
	}
	d := def
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.ops[d.Key()]; ok {
		return errors.Errorf("op %s already registered", d.Key())
	}
	r.ops[d.Key()] = &d
	return nil
}

func lastPositional(args []ArgSpec) int {
	last := -1
	for i, a := range args {
		if a.Positional {
			last = i
		}
	}
	return last
}

// MustRegister registers defs and panics on error (for init-time tables).
func (r *Registry) MustRegister(defs ...OpDef) {
	for _, d := range defs {
		if err := r.Register(d); err != nil {
			panic(err)
		}
	}
}

// Lookup finds the op name on receiver kind on. Param receivers fall back to
// Value ops, since a param evaluates to a value.
func (r *Registry) Lookup(on Kind, name string) (*OpDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if op, ok := r.ops[string(on)+":"+name]; ok {
		return op, true
	}
	if on == KindParam {
		op, ok := r.ops[string(KindValue)+":"+name]
		return op, ok
	}
	return nil, false
}

// Ops returns all ops sorted by receiver kind and name.
func (r *Registry) Ops() []*OpDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*OpDef, 0, len(r.ops))
	for _, op := range r.ops {
		out = append(out, op)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].On != out[j].On {
			return kindOrder(out[i].On) < kindOrder(out[j].On)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func kindOrder(k Kind) int {
	for i, kk := range Kinds {
		if kk == k {
			return i + 1
		}
	}
	return 0
}

// Methods returns the op names callable on kind (including Value ops for Param).
func (r *Registry) Methods(on Kind) []string {
	set := map[string]bool{}
	for _, op := range r.Ops() {
		if op.On == on || (on == KindParam && op.On == KindValue) {
			set[op.Name] = true
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Find returns every op with the given name, on any kind.
func (r *Registry) Find(name string) []*OpDef {
	var out []*OpDef
	for _, op := range r.Ops() {
		if op.Name == name {
			out = append(out, op)
		}
	}
	return out
}
