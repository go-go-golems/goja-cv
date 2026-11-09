// Package graph holds the cv op registry, the hash-consed node graph and the
// caching evaluator.
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
	KindRegions Kind = "Regions"
	KindValue   Kind = "Value"
	// KindNone is the receiver "kind" of source and module ops (cv.open,
	// cv.compare).
	KindNone Kind = ""
)

// MaskType is the type word of an argument that takes a mask: an Image
// node with one channel (GCV-004 §7.5). It is not a kind.
const MaskType = "Mask"

// accepts reports whether a node of kind k satisfies a declared input kind
// such as "Image" or "Regions|Image" ("Mask" is not a kind: see acceptsNode).
func accepts(decl Kind, k Kind) bool {
	for _, d := range strings.Split(string(decl), "|") {
		if Kind(d) == k {
			return true
		}
	}
	return false
}

// acceptsNode reports whether node n satisfies a declared node type such as
// "Image", "Mask" (an Image with one channel) or "Regions|Mask".
func acceptsNode(decl string, n *Node) bool {
	for _, d := range strings.Split(decl, "|") {
		if Kind(d) == n.Kind || (d == MaskType && n.Kind == KindImage && n.Channels == 1) {
			return true
		}
	}
	return false
}

// takesMask reports whether a declared node type accepts a mask.
func takesMask(decl string) bool {
	for _, d := range strings.Split(decl, "|") {
		if d == MaskType {
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
	Level         // a value on the scale of the receiver's space (levels.black, mask.luminance.above)
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

// SpaceRule is how an op relates to the space of its receiver (GCV-004
// §4.3). The zero value is the common case.
type SpaceRule uint8

const (
	// Follows: the op works in its receiver's space; other image inputs are
	// converted to it (masks are read as stored).
	Follows SpaceRule = iota
	// Physical: the op works in linear light; the builder converts its inputs
	// to linear and its result back to the receiver's space.
	Physical
	// Display: the op draws a picture: image inputs are converted to encoded
	// values.
	Display
)

// ResultEnc computes the encoding and space of an Image result. s is the
// op's work space (SpaceNone for ops without an image). The three named
// rules cover every op but sources and as.
type ResultEnc struct {
	Enc   func(a Args) buffer.Encoding
	Space func(a Args, s buffer.Space) buffer.Space
}

var (
	// KeepEnc: the encoding of the receiver (or, for an op called on
	// regions or a mask, of its image argument); the space is the op's
	// work space, or that image's for ops without one.
	KeepEnc = &ResultEnc{
		Enc: func(a Args) buffer.Encoding {
			if in := a.imageSource(); in != nil {
				return in.Enc
			}
			return buffer.EncNone
		},
		Space: func(a Args, s buffer.Space) buffer.Space {
			if s != buffer.SpaceNone {
				return s
			}
			if in := a.imageSource(); in != nil {
				return in.Space
			}
			return buffer.SpaceLinear
		},
	}
	// DerivedEnc: a new quantity (chroma, Lab; GCV-002's sub, highpass). The
	// values are reinterpreted as linear-encoded: their stored value is their
	// value, so what is measured is what is seen.
	DerivedEnc = &ResultEnc{
		Enc:   func(Args) buffer.Encoding { return buffer.EncLinear },
		Space: func(Args, buffer.Space) buffer.Space { return buffer.SpaceLinear },
	}
	// DisplayEnc: display colours (colormap), sRGB-encoded values.
	DisplayEnc = &ResultEnc{
		Enc:   func(Args) buffer.Encoding { return buffer.EncSRGB },
		Space: func(Args, buffer.Space) buffer.Space { return buffer.SpaceEncoded },
	}
)

// EncodingValueSynonyms are the value synonyms of encoding arguments.
var EncodingValueSynonyms = map[string]string{"sRGB": "srgb", "SRGB": "srgb", "gamma": "srgb", "perceptual": "srgb",
	"lin": "linear", "raw": "linear", "identity": "linear", "none": "linear"}

// ArgSpec declares one argument of an op.
//
// Types: "number", "string", "bool", "enum:a,b,c", "number[]", "string[]",
// "box" ([x, y, w, h] or an object with x, y, w, h), "point" ([x, y] or an
// object with cx, cy), "data" (a node or plain data, kept as given: the
// argument of cv.json and cv.csv), unions such as "point|number", "object"
// (with Options as the allowed keys), "any", and node types such as
// "Mask" (an Image with one channel), "Regions|Mask" or "Image[]": a node
// argument is its own input of
// the node (one per list element, repetitions kept); build-time code reads
// it with Args.Input / Args.Inputs, Run with Args.Value / Args.Values.
type ArgSpec struct {
	Name       string
	Type       string
	Unit       Unit
	Default    any        // filled when the argument is absent; nil means required unless Optional
	Optional   bool       // may be absent without a default
	Positional bool       // may be passed positionally (in declaration order)
	Variadic   bool       // a positional "...[]" argument that takes all remaining positional values
	Len        int        // for list types: the exact number of elements
	InFrame    bool       // "box"/"point": a box must overlap, a point lie in, the receiver's frame (E_ARG_RANGE)
	Min, Max   *float64   // hard limits: violation is E_ARG_RANGE
	Usual      [2]float64 // soft range (zero value = none): violation is a RANGE warning
	UsualHint  string     // appended to that RANGE warning: what to do instead
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

// ChartKind is how look() draws a Value node: no chart, bars (histogram
// counts) or lines (a profile: t against meanL, meanR, meanG, meanB).
type ChartKind uint8

const (
	ChartNone ChartKind = iota
	ChartBars
	ChartLines
)

// RunFunc computes an op. in holds the evaluated receiver (empty for source
// and module ops); node arguments are read with Args.Value and Args.Values.
type RunFunc func(ctx context.Context, ec *EvalCtx, in []Value, args Args) (Value, error)

// OpDef describes one registered op.
type OpDef struct {
	Name    string // "exposure", "mask.luminance": a dot puts the op in a namespace
	On      Kind   // receiver kind; KindNone for source ops
	Args    []ArgSpec
	Returns Kind
	Class   OpClass                // pixel (default), geometric or decoration (render above scale 1)
	Space   SpaceRule              // follows its receiver (default), physical or display (§7.4)
	SpaceOf func(a Args) SpaceRule // overrides Space per call (toGray: rec709 is physical, average follows)
	Result  *ResultEnc             // encoding and space of an Image result; nil = KeepEnc
	// Channels is the channel count of an Image result; nil: the receiver's
	// (§7.5). MaxChannels is the rule of two-image ops.
	Channels func(a Args) int
	// OneChannel: the receiver must have one channel (the mask ops,
	// colormap); ThreeChannel: it must have colour (toLab, split, chroma,
	// mask.color). Both are E_WRONG_KIND at build time. A OneChannel
	// receiver is read as stored (never converted between spaces).
	OneChannel   bool
	ThreeChannel bool
	// Luma: the builder adds the luminance of the receiver (or of the argument
	// LumaOf names) as an extra input, toGray with the op's gray weights in
	// that image's space; Run reads it with Args.Luma (the one luminance).
	Luma     bool
	LumaOf   string
	LumaWhen func(a Args) bool // nil: always; else only for these args (levels: auto; histogram: channel L)
	Internal bool              // not callable from JavaScript, not listed in help (convert)
	Lossless bool              // looks of its result are PNG by default (format "auto"): contact sheets
	Chart    ChartKind         // look() of a Value result draws this chart (histogram: bars, profile: lines)
	// MaxItems is a static upper bound on the item count of a Regions
	// result (top(n)); ok false when unknown. A sheet sizes
	// itself by it.
	MaxItems func(args Args) (n int, ok bool)
	Bespoke  bool // a module op bound by hand (cv.open, the builder placeholder): the prelude does not generate cv.<name>
	// Expand makes the op a macro: a graph of other ops built at intern
	// time (exactly one of Run and Expand is set). See ExpandFunc.
	Expand ExpandFunc
	// Warn adds build-time warnings for a call (toLab on camera RGB).
	Warn     func(b *Builder, a Args) []Warning
	Identity func(a Args) bool  // neutral arguments: the receiver is returned unchanged
	Global   bool               // needs the whole image (stats, histogram)
	Lazy     bool               // inputs are not evaluated before Run (explain)
	DocFrame func(a Args) Frame // the result's frame; nil: the first input with a non-empty frame
	// Fields is the schema of the result: the items of a Regions result, the
	// rows or the object of a Value result (schema.go). It runs at intern
	// time with the bound args (node arguments are InputRefs; Args carries the
	// input nodes), and is called with Args without inputs for help. nil: a
	// Regions op on Regions keeps its receiver's fields; other results have
	// none.
	Fields func(a Args) []FieldSpec
	// Produces maps the field names the op can add with some arguments to
	// the call that adds them, for the hint of a filter on a field the
	// receiver lacks ("snr comes from regions.measure(img, { ring: true })").
	Produces map[string]string
	// Validate checks bound args at build time (after defaults and types);
	// the input nodes (their statics, such as Fields) are read through
	// Args. Errors fail the call.
	Validate func(a Args) error
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

// Lookup finds the op name on receiver kind on.
func (r *Registry) Lookup(on Kind, name string) (*OpDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	op, ok := r.ops[string(on)+":"+name]
	return op, ok
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

// receiverKinds are the receiver kinds in display order.
var receiverKinds = []Kind{KindImage, KindRegions, KindValue}

func kindOrder(k Kind) int {
	for i, kk := range receiverKinds {
		if kk == k {
			return i + 1
		}
	}
	return 0
}

// Receivers lists the receiver kinds that have methods, in display order.
func (r *Registry) Receivers() []Kind {
	var out []Kind
	for _, k := range receiverKinds {
		if len(r.Methods(k)) > 0 {
			out = append(out, k)
		}
	}
	return out
}

// Methods returns the op names callable on kind.
func (r *Registry) Methods(on Kind) []string {
	set := map[string]bool{}
	for _, op := range r.Ops() {
		if op.On == on {
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
