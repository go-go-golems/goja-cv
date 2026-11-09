// Package graph holds the cv op registry, the hash-consed node graph, the
// param store and the caching evaluator.
package graph

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
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
	Level         // a value on the scale of the op's working space (levels.black, mask.luminance.above)
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
	case Level:
		return "level"
	}
	return "unknown"
}

// SpaceRule is an op's working space: the space its kernel's numbers are in.
type SpaceRule struct {
	// Default is the working space when the call passes no { space };
	// SpaceNone means the op does not read levels (geometry, masks, data).
	Default buffer.Space
	// Fixed: the op only works in Default (toLab, encode); no space option.
	Fixed bool
}

// ResultEnc computes the encoding and space of an Image result. s is the
// op's resolved working space (SpaceNone for ops without one). The three
// named rules cover every op but sources, convert and encode.
type ResultEnc struct {
	Enc   func(args Args, inputs []*Node) buffer.Encoding
	Space func(args Args, s buffer.Space, inputs []*Node) buffer.Space
}

// firstImage is the first Image input (receiver, then Inputs, then
// kind-typed arguments), or nil.
func firstImage(inputs []*Node) *Node {
	for _, in := range inputs {
		if in.Kind == KindImage {
			return in
		}
	}
	return nil
}

var (
	// KeepEnc: the encoding of the first Image input; the space is the op's
	// working space, or the first Image input's for ops without one.
	KeepEnc = &ResultEnc{
		Enc: func(_ Args, inputs []*Node) buffer.Encoding {
			if in := firstImage(inputs); in != nil {
				return in.Enc
			}
			return buffer.EncNone
		},
		Space: func(_ Args, s buffer.Space, inputs []*Node) buffer.Space {
			if s != buffer.SpaceNone {
				return s
			}
			if in := firstImage(inputs); in != nil {
				return in.Space
			}
			return buffer.SpaceLinear
		},
	}
	// DerivedEnc: a new quantity (chroma, Lab; GCV-002's sub, highpass). The
	// values are reinterpreted as linear-encoded: their stored value is their
	// value, so what is measured is what is seen.
	DerivedEnc = &ResultEnc{
		Enc:   func(Args, []*Node) buffer.Encoding { return buffer.EncLinear },
		Space: func(Args, buffer.Space, []*Node) buffer.Space { return buffer.SpaceLinear },
	}
	// DisplayEnc: display colours (colormap), sRGB-encoded values.
	DisplayEnc = &ResultEnc{
		Enc:   func(Args, []*Node) buffer.Encoding { return buffer.EncSRGB },
		Space: func(Args, buffer.Space, []*Node) buffer.Space { return buffer.SpaceEncoded },
	}
)

// SpaceArg is the name of the working-space option Register adds.
const SpaceArg = "space"

// spaceOption is the option Register appends to every op with a working
// space that is not fixed.
var spaceOption = ArgSpec{Name: SpaceArg, Type: "enum:linear,encoded", Optional: true, Static: true,
	Doc:           "working space: linear (the encoding undone) or encoded (the file's values, what looks show)",
	ValueSynonyms: SpaceValueSynonyms}

// SpaceValueSynonyms and EncodingValueSynonyms are the value synonyms of the
// space option and of encoding arguments.
var (
	SpaceValueSynonyms = map[string]string{"file": "encoded", "stored": "encoded", "code": "encoded", "display": "encoded",
		"perceptual": "encoded", "srgb": "encoded", "sRGB": "encoded", "lin": "linear", "light": "linear", "physical": "linear"}
	EncodingValueSynonyms = map[string]string{"sRGB": "srgb", "SRGB": "srgb", "gamma": "srgb", "perceptual": "srgb",
		"lin": "linear", "raw": "linear", "identity": "linear", "none": "linear"}
)

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
	// ValueSynonyms map words agents use for an enum value to the value they
	// mean (scoped to this argument), for the did-you-mean hint and fix.
	ValueSynonyms map[string]string
	// ValueDocs describe the values of an enum (view presets): help lists
	// them and an unknown value's error names them.
	ValueDocs map[string]string
	// Redirects map an unknown option name (looked up over every spec of the
	// call) or an unknown enum value of this argument to the call that does
	// it now: look({ box }) → crop, blend({ mode: "difference" }) → sub.
	Redirects map[string]Redirect
	Doc       string
}

// Redirect is the hint and literal fix for a spelling the API does not have.
type Redirect struct{ Hint, Fix string }

func (s *ArgSpec) hasUsual() bool { return s.Usual != [2]float64{} }

// F returns a pointer to v, for Min and Max.
func F(v float64) *float64 { return &v }

// OpClass says how an op behaves when a look asks for a render scale above
// 1 (GCV-002 §3.4, Foundations §F5).
type OpClass uint8

const (
	// PixelOp computes values from values (exposure, blur, arithmetic): it
	// is never computed above scale 1; a render above 1 computes it at 1 and
	// upsamples the window.
	PixelOp OpClass = iota
	// GeometricOp only selects pixels (crop): its value in a window is its
	// receiver's value in that window, at any scale.
	GeometricOp
	// Decoration draws marks defined in doc px (overlay, annotate): it runs
	// at the render scale on inputs rendered at that scale, so lines stay
	// one image px wide.
	Decoration
)

func (c OpClass) String() string {
	switch c {
	case GeometricOp:
		return "geometric"
	case Decoration:
		return "decoration"
	case PixelOp:
	}
	return "pixel"
}

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
	Class    OpClass               // pixel (default), geometric or decoration (render above scale 1)
	Space    SpaceRule             // working space (SpaceNone: the op does not read levels)
	Result   *ResultEnc            // encoding and space of an Image result; nil = KeepEnc
	Gray     func(args Args) []int // positions of the Image inputs read as luminance (fed toGray rec709 in linear space)
	Internal bool                  // not callable from JavaScript, not listed in help (convert)
	Lossless bool                  // looks of its result are PNG by default (format "auto"): contact sheets
	// MaxItems is a static upper bound on the item count of a Regions
	// result (top(n)); ok false when unknown (a lifted n). A sheet sizes
	// itself by it.
	MaxItems func(args Args) (n int, ok bool)
	Bespoke  bool // a module op bound by hand (cv.open, P.slider, the builder placeholder): the prelude does not generate cv.<name>
	// Expand makes the op a macro: a graph of other ops built at intern
	// time (exactly one of Run and Expand is set). See ExpandFunc.
	Expand ExpandFunc
	// Warn adds build-time warnings for a call (toLab on camera RGB).
	Warn     func(b *Builder, args Args, inputs []*Node) []Warning
	Identity func(args Args) bool                  // neutral arguments: the receiver is returned unchanged
	Halo     func(args Args) float64               // doc px of border around each output pixel the op reads
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
	mu    sync.RWMutex
	ops   map[string]*OpDef
	funcs map[string]*FuncDef
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{ops: map[string]*OpDef{}, funcs: map[string]*FuncDef{}}
}

// Register validates def and adds it.
func (r *Registry) Register(def OpDef) error {
	if def.Name == "" {
		return errors.New("op without name")
	}
	if (def.Run == nil) == (def.Expand == nil) {
		return errors.Errorf("op %s must have exactly one of Run and Expand", def.Name)
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
	if d.Space.Default != buffer.SpaceNone && !d.Space.Fixed {
		if d.spec(SpaceArg) != nil {
			return errors.Errorf("op %s declares its own %s argument", def.Name, SpaceArg)
		}
		d.Args = append(append([]ArgSpec(nil), d.Args...), spaceOption)
	}
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

// Ops returns all ops callable from JavaScript (not Internal ones), sorted
// by receiver kind and name.
func (r *Registry) Ops() []*OpDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*OpDef, 0, len(r.ops))
	for _, op := range r.ops {
		if !op.Internal {
			out = append(out, op)
		}
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

// ModuleOps are the module ops the prelude exposes as cv.<name>: ops with
// no receiver that are neither internal nor bound by hand (cv.compare,
// cv.sheet).
func (r *Registry) ModuleOps() []string {
	var out []string
	for _, op := range r.Ops() {
		if op.On == KindNone && !op.Bespoke {
			out = append(out, op.Name)
		}
	}
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
