package graph

import (
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// Ops build nodes; functions do something now and return something else (a
// look object, a promise, help data). Both are registered, so help, the
// generated reference, option binding and did-you-mean come from one place
// (GCV-002 §7.10).

// FuncDef describes one registered function.
type FuncDef struct {
	Name string
	// On lists the receiver types: node kinds ("Image", "Mask", "Value") or
	// Go handle types; empty = a module function (cv.<name>) or, with Global,
	// a session global.
	On []string
	// Args are the function's options, bound with BindOptions (positional
	// arguments first, then an options object; synonyms, did-you-mean and
	// redirects as for ops).
	Args []ArgSpec
	// Usage overrides the generated usage line (for functions whose first
	// argument is a node, such as show).
	Usage string
	// Global: a session global (show), not a member of cv.
	Global bool
	// Bespoke: implemented by hand in the binding or prelude; the registry
	// documents it but the prelude does not generate it.
	Bespoke bool
	Doc     string
	See     []string
}

// CallName is how the function is written in JavaScript: node.look, cv.evaluate, show.
func (f *FuncDef) CallName() string {
	switch {
	case len(f.On) > 0:
		return "node." + f.Name
	case f.Global:
		return f.Name
	}
	return "cv." + f.Name
}

// UsageLine is the usage of the function: its Usage, or the positional
// arguments and the options generated from Args ("node.look(label) ·
// node.look({ label, maxSide })").
func (f *FuncDef) UsageLine() string {
	if f.Usage != "" {
		return f.Usage
	}
	var pos, all []string
	for _, a := range f.Args {
		if a.Positional {
			pos = append(pos, a.Name)
		}
		all = append(all, a.Name)
	}
	call := f.CallName()
	if len(all) == 0 {
		return call + "()"
	}
	if len(pos) == 0 {
		return call + "({ " + strings.Join(all, ", ") + " })"
	}
	return call + "(" + strings.Join(pos, ", ") + ") · " + call + "({ " + strings.Join(all, ", ") + " })"
}

// RegisterFunc validates f and adds it.
func (r *Registry) RegisterFunc(f FuncDef) error {
	if f.Name == "" {
		return errors.New("function without name")
	}
	for _, a := range f.Args {
		if a.Type == "" {
			return errors.Errorf("function %s: arg %s has no type", f.Name, a.Name)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.funcs == nil {
		r.funcs = map[string]*FuncDef{}
	}
	if _, ok := r.funcs[f.Name]; ok {
		return errors.Errorf("function %s already registered", f.Name)
	}
	d := f
	r.funcs[f.Name] = &d
	return nil
}

// MustRegisterFunc registers fs and panics on error (for init-time tables).
func (r *Registry) MustRegisterFunc(fs ...FuncDef) {
	for _, f := range fs {
		if err := r.RegisterFunc(f); err != nil {
			panic(err)
		}
	}
}

// Func finds the function name.
func (r *Registry) Func(name string) (*FuncDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.funcs[name]
	return f, ok
}

// Funcs returns every function sorted by name.
func (r *Registry) Funcs() []*FuncDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*FuncDef, 0, len(r.funcs))
	for _, f := range r.funcs {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// FuncsOn returns the functions callable on a node of kind k.
func (r *Registry) FuncsOn(k Kind) []*FuncDef {
	var out []*FuncDef
	for _, f := range r.Funcs() {
		for _, on := range f.On {
			if Kind(on) == k {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// BindOptions binds the raw arguments of a call written call (positional
// values, then an options object) against specs, as the binder binds op
// arguments: defaults, types, limits, unknown keys with synonyms,
// did-you-mean and redirects. Nodes are refused (options are literals).
func BindOptions(call string, specs []ArgSpec, raw []any) (Args, []Warning, error) {
	op := &OpDef{Name: call, Args: specs}
	bd, err := bind(op, call, false, raw, func(NodeID) (*Node, bool) { return nil, false })
	if err != nil {
		return Args{}, nil, err
	}
	return Args{op: op, vals: bd.args}, bd.warnings, nil
}
