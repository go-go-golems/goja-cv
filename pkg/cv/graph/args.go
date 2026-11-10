package graph

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// NodeArg is a node passed as an argument value in a raw call (from JS).
type NodeArg struct{ ID NodeID }

// InputRef is a node argument inside canonical args: it refers to
// node.Inputs[Index].
type InputRef struct{ Index int }

// MarshalJSON encodes the reference for hashing and display.
func (r InputRef) MarshalJSON() ([]byte, error) {
	return []byte(`{"$in":` + strconv.Itoa(r.Index) + `}`), nil
}

// Args are an op's canonical arguments: defaults filled, numbers normalized
// to float64, node arguments recorded as InputRefs to the node's own inputs
// (one input per node argument, in list order, repetitions kept). Args also
// carries the node's inputs (build time and evaluation) and their values
// (evaluation of a non-lazy op), so op code reads node arguments through
// Receiver, Input, Inputs, Value and Values and never indexes inputs itself.
//
// Every accessor checks the spec's type and the value's Go type and panics
// when they disagree, and when an optional argument without a default is
// read without Has: there is no zero-value path. A panic is a programming
// error; the panic boundary reports it as E_INTERNAL and the registry
// property test calls every build-time function so it is found in CI.
type Args struct {
	op   *OpDef
	vals map[string]any
	meta any     // typed Go-side metadata of a source node (SourceMeta, PlaceholderMeta); nil otherwise
	ins  []*Node // the node's inputs: the receiver, then one per node argument
	vs   []Value // their values (evaluation of a non-lazy op); nil at build time
}

// Meta returns the node's typed metadata (SourceMeta of cv.open,
// PlaceholderMeta of a builder lambda's placeholder). It panics when the
// node has no metadata of type T: a programming error.
func Meta[T any](a Args) T {
	m, ok := a.meta.(T)
	if !ok {
		var want T
		panic(fmt.Sprintf("cv: %s has metadata %T, not %T", a.opName(), a.meta, want))
	}
	return m
}

func (a Args) opName() string {
	if a.op == nil {
		return "Args"
	}
	return a.op.Name
}

// Has reports whether the argument is present: the only way to ask whether
// an optional argument without a default was given.
func (a Args) Has(name string) bool { _, ok := a.vals[name]; return ok }

// Raw returns the canonical value (nil if absent), for union-typed
// arguments read with a type switch whose default branch panics.
func (a Args) Raw(name string) any { return a.vals[name] }

// Map returns a shallow copy of the canonical values.
func (a Args) Map() map[string]any {
	out := make(map[string]any, len(a.vals))
	for k, v := range a.vals {
		out[k] = v
	}
	return out
}

func (a Args) mustSpec(name string) *ArgSpec {
	if a.op == nil {
		panic(fmt.Sprintf("cv: Args without op reading %q", name))
	}
	s := a.op.spec(name)
	if s == nil {
		panic(fmt.Sprintf("cv: op %s has no arg %q", a.op.Name, name))
	}
	return s
}

// present returns the value of a declared argument; it panics when an
// optional argument without a default is absent (read it after Has).
func (a Args) present(name string) (*ArgSpec, any) {
	s := a.mustSpec(name)
	v, ok := a.vals[name]
	if !ok {
		panic(fmt.Sprintf("cv: %s.%s is optional and absent; check Args.Has first", a.op.Name, name))
	}
	return s, v
}

// wrongType panics with the accessor's complaint.
func (a Args) wrongType(name string, v any, want string) {
	panic(fmt.Sprintf("cv: %s.%s is %T, not %s", a.op.Name, name, v, want))
}

// allows reports whether spec type typ (or an alternative of a union)
// accepts values of kind want: "string", "bool", "number", "list", "object".
func allows(typ, want string) bool {
	for _, alt := range strings.Split(typ, "|") {
		switch {
		case alt == want, alt == "any":
			return true
		case want == "string" && strings.HasPrefix(alt, "enum:"):
			return true
		case want == "list" && (strings.HasSuffix(alt, "[]") || alt == "box" || alt == "point"):
			return true
		}
	}
	return false
}

func (a Args) number(name string) float64 {
	_, x := a.present(name)
	v, ok := x.(float64)
	if !ok {
		a.wrongType(name, x, "a resolved number")
	}
	return v
}

// Number reads a unitless or fraction number. Reading a DocPx (DocPx2)
// argument through Number panics: lengths must go through Length (Area).
func (a Args) Number(name string) float64 {
	if s := a.mustSpec(name); s.Unit == DocPx || s.Unit == DocPx2 {
		panic(fmt.Sprintf("cv: %s.%s is a doc-px %s; read it with Args.Length or Args.Area", a.op.Name, name, s.Unit))
	}
	return a.number(name)
}

// Area reads a DocPx2 argument converted to buffer px² at scale.
func (a Args) Area(name string, scale float64) float64 {
	if s := a.mustSpec(name); s.Unit != DocPx2 {
		panic(fmt.Sprintf("cv: %s.%s is not a doc-px² area", a.op.Name, name))
	}
	return a.number(name) * scale * scale
}

// Lengths reads a list of DocPx numbers (a box [x, y, w, h], a point [x,
// y]) converted to buffer px at scale. For a union-typed argument it returns
// nil when the value is not a list.
func (a Args) Lengths(name string, scale float64) []float64 {
	if s := a.mustSpec(name); s.Unit != DocPx {
		panic(fmt.Sprintf("cv: %s.%s is not a doc-px list", a.op.Name, name))
	}
	_, x := a.present(name)
	list, ok := x.([]any)
	if !ok {
		return nil
	}
	out := make([]float64, len(list))
	for i, v := range list {
		f, ok := v.(float64)
		if !ok {
			panic(fmt.Sprintf("cv: %s.%s[%d] is %T, not a resolved number", a.op.Name, name, i, v))
		}
		out[i] = f * scale
	}
	return out
}

// XY reads a "box" or "point" argument (or the point alternative of a union
// such as "point|number") as buffer px of image value v: the frame origin of
// v is subtracted from the coordinates (x, y of a point, the corner of a box;
// sizes are not shifted), then even elements are multiplied by v's x scale
// and odd ones by its y scale (AxisScales: rounded buffer sizes make them
// differ slightly from the nominal scale). It returns nil when the value is
// not a list.
func (a Args) XY(name string, v *ImageValue) []float64 {
	s := a.mustSpec(name)
	if s.Unit != DocPx {
		panic(fmt.Sprintf("cv: %s.%s is not a doc-px box or point", a.op.Name, name))
	}
	l := a.Lengths(name, 1)
	if l == nil {
		return nil
	}
	sx, sy := AxisScales(v)
	ox, oy := v.BufferOrigin()
	for i := range l {
		if i%2 == 0 {
			if i < 2 {
				l[i] -= ox
			}
			l[i] *= sx
		} else {
			if i < 2 {
				l[i] -= oy
			}
			l[i] *= sy
		}
	}
	return l
}

// Box reads a literal "box" argument in doc px, [x, y, w, h] (for frames
// and display; kernels use XY).
func (a Args) Box(name string) [4]float64 {
	if s := a.mustSpec(name); s.Unit != DocPx {
		panic(fmt.Sprintf("cv: %s.%s is not a doc-px box", a.op.Name, name))
	}
	l := a.Lengths(name, 1)
	if len(l) != 4 {
		panic(fmt.Sprintf("cv: %s.%s is not a box [x, y, w, h]", a.op.Name, name))
	}
	var out [4]float64
	copy(out[:], l)
	return out
}

// AxisScales are the effective buffer-px-per-doc-px scales of an image
// value along x and y.
func AxisScales(v *ImageValue) (float64, float64) {
	return float64(v.Buf.W) / float64(max(1, v.Frame.W)), float64(v.Buf.H) / float64(max(1, v.Frame.H))
}

// Luma is the evaluated luminance input of an op with OpDef.Luma: one
// channel, the luminance of the receiver (or of OpDef.LumaOf) in its space.
func (a Args) Luma() *ImageValue {
	if a.op == nil || !a.op.Luma {
		panic(fmt.Sprintf("cv: %s has no luminance input", a.opName()))
	}
	a.evaluated(lumaArg)
	x, ok := a.vals[lumaArg]
	if !ok {
		panic(fmt.Sprintf("cv: %s has no luminance input (no image to take it of)", a.opName()))
	}
	r, ok := x.(InputRef)
	if !ok || r.Index >= len(a.vs) {
		panic(fmt.Sprintf("cv: %s.%s is %v", a.opName(), lumaArg, x))
	}
	return a.vs[r.Index].(*ImageValue)
}

// HasInputs reports whether the args carry their node's inputs: false only
// when an op's schema is computed for help (DocFields), without a call.
func (a Args) HasInputs() bool { return a.ins != nil }

// Receiver is the node the op was called on. It panics for source and
// module ops, which have none.
func (a Args) Receiver() *Node {
	if a.op == nil || a.op.On == KindNone || len(a.ins) == 0 {
		panic(fmt.Sprintf("cv: %s has no receiver", a.opName()))
	}
	return a.ins[0]
}

// ref reads the InputRef a node argument holds.
func (a Args) ref(name string, v any) InputRef {
	r, ok := v.(InputRef)
	if !ok {
		a.wrongType(name, v, "a node argument")
	}
	if r.Index < 0 || r.Index >= len(a.ins) {
		panic(fmt.Sprintf("cv: %s.%s refers to input %d of %d", a.op.Name, name, r.Index, len(a.ins)))
	}
	return r
}

func (a Args) refs(name string) []InputRef {
	_, x := a.present(name)
	list, ok := x.([]any)
	if !ok {
		return []InputRef{a.ref(name, x)} // a single node for a "T|T[]" argument
	}
	out := make([]InputRef, len(list))
	for i, e := range list {
		out[i] = a.ref(name, e)
	}
	return out
}

// Input is the node passed for a node argument (an Image, a mask, Regions);
// nil only when the argument is optional and absent.
func (a Args) Input(name string) *Node {
	a.mustSpec(name)
	x, ok := a.vals[name]
	if !ok {
		return nil
	}
	return a.ins[a.ref(name, x).Index]
}

// Inputs are the nodes of a node-list argument ("Image[]"), in list order
// with repetitions kept; a single node for a "T|T[]" argument is a list of
// one.
func (a Args) Inputs(name string) []*Node {
	rs := a.refs(name)
	out := make([]*Node, len(rs))
	for i, r := range rs {
		out[i] = a.ins[r.Index]
	}
	return out
}

func (a Args) evaluated(name string) {
	if a.vs == nil {
		panic(fmt.Sprintf("cv: %s.%s read with Args.Value before evaluation (or in a lazy op)", a.opName(), name))
	}
}

// Value is the evaluated value of a node argument; nil only when the
// argument is optional and absent. It panics at build time and in a lazy op.
func (a Args) Value(name string) Value {
	a.mustSpec(name)
	a.evaluated(name)
	x, ok := a.vals[name]
	if !ok {
		return nil
	}
	return a.vs[a.ref(name, x).Index]
}

// Values are the evaluated values of a node-list argument, in list order.
func (a Args) Values(name string) []Value {
	a.evaluated(name)
	rs := a.refs(name)
	out := make([]Value, len(rs))
	for i, r := range rs {
		out[i] = a.vs[r.Index]
	}
	return out
}

// Length reads a DocPx argument converted to buffer pixels at scale (buffer
// px per doc px). It is the only way to read a DocPx argument.
func (a Args) Length(name string, scale float64) float64 {
	if s := a.mustSpec(name); s.Unit != DocPx {
		panic(fmt.Sprintf("cv: %s.%s is not a doc-px length", a.op.Name, name))
	}
	return a.number(name) * scale
}

// DocLength reads a DocPx argument in doc px, for display only (explain,
// summaries); kernels must use Length.
func (a Args) DocLength(name string) float64 {
	if s := a.mustSpec(name); s.Unit != DocPx {
		panic(fmt.Sprintf("cv: %s.%s is not a doc-px length", a.op.Name, name))
	}
	return a.number(name)
}

// Int reads a unitless number as an int.
func (a Args) Int(name string) int { return int(math.Round(a.Number(name))) }

// String reads a string or enum argument.
func (a Args) String(name string) string {
	s, x := a.present(name)
	if !allows(s.Type, "string") {
		panic(fmt.Sprintf("cv: %s.%s is declared %s, not a string", a.op.Name, name, s.Type))
	}
	str, ok := x.(string)
	if !ok {
		a.wrongType(name, x, "a string")
	}
	return str
}

// Bool reads a bool argument.
func (a Args) Bool(name string) bool {
	s, x := a.present(name)
	if !allows(s.Type, "bool") {
		panic(fmt.Sprintf("cv: %s.%s is declared %s, not a bool", a.op.Name, name, s.Type))
	}
	b, ok := x.(bool)
	if !ok {
		a.wrongType(name, x, "a bool")
	}
	return b
}

// Numbers reads a unitless "number[]" argument.
func (a Args) Numbers(name string) []float64 {
	s, x := a.present(name)
	if s.Unit == DocPx || s.Unit == DocPx2 {
		panic(fmt.Sprintf("cv: %s.%s is a doc-px list; read it with Args.Lengths", a.op.Name, name))
	}
	list, ok := x.([]any)
	if !ok {
		a.wrongType(name, x, "a list of numbers")
	}
	out := make([]float64, len(list))
	for i, v := range list {
		f, ok := v.(float64)
		if !ok {
			a.wrongType(fmt.Sprintf("%s[%d]", name, i), v, "a number")
		}
		out[i] = f
	}
	return out
}

// Strings reads a "string[]" argument.
func (a Args) Strings(name string) []string {
	_, x := a.present(name)
	list, ok := x.([]any)
	if !ok {
		a.wrongType(name, x, "a list of strings")
	}
	out := make([]string, len(list))
	for i, v := range list {
		str, ok := v.(string)
		if !ok {
			a.wrongType(fmt.Sprintf("%s[%d]", name, i), v, "a string")
		}
		out[i] = str
	}
	return out
}

// Object reads an "object" or "any" argument holding an object.
func (a Args) Object(name string) map[string]any {
	_, x := a.present(name)
	m, ok := x.(map[string]any)
	if !ok {
		a.wrongType(name, x, "an object")
	}
	return m
}

// imageSource is the image an Image result keeps the encoding of: the
// receiver when it is an Image, else the first Image node argument in
// declaration order (an overlay called on regions or a mask), or nil.
func (a Args) imageSource() *Node {
	if a.op != nil && a.op.On != KindNone && len(a.ins) > 0 && a.ins[0].Kind == KindImage {
		return a.ins[0]
	}
	if a.op == nil {
		return nil
	}
	for i := range a.op.Args {
		s := &a.op.Args[i]
		if !isKindType(s.Type) || !accepts(Kind(s.Type), KindImage) || !a.Has(s.Name) {
			continue
		}
		for _, r := range a.refs(s.Name) {
			if a.ins[r.Index].Kind == KindImage {
				return a.ins[r.Index]
			}
		}
	}
	return nil
}

// canonicalJSON is the hashing form of the args.
func (a Args) canonicalJSON() []byte {
	b, err := json.Marshal(a.vals)
	if err != nil {
		panic(fmt.Sprintf("cv: args of %s not serializable: %v", a.op.Name, err))
	}
	return b
}

// normalize converts exported JS/Go values into the canonical value space:
// float64, string, bool, []any, map[string]any, NodeArg, nil.
func normalize(v any) any {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case float32:
		return normalize(float64(x))
	case float64:
		if x == 0 {
			return 0.0 // fold -0
		}
		return x
	case []float64:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalize(e)
		}
		return out
	case []string:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = e
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalize(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			if e == nil {
				continue // {key: undefined} means absent
			}
			out[k] = normalize(e)
		}
		return out
	}
	return v
}

func isOptionsObject(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func typeAcceptsObject(t string) bool { return t == "object" || t == "any" }

// binder carries the state of binding one call.
type binder struct {
	op       *OpDef
	nodeOf   func(NodeID) (*Node, bool)
	items    func(entries []any, src NodeID) (NodeID, error) // interns an item array (nil: arrays refused)
	base     int                                             // index of the first node argument in node.Inputs (1 after a receiver)
	inputs   []NodeID                                        // one per node argument, in binding order, repetitions kept
	warnings []Warning
}

// lift makes node id an input of the node being built, for an argument of
// node type want. Options are literal values: a node anywhere else is
// E_ARG_TYPE with the (await …) fix.
func (b *binder) lift(s *ArgSpec, id NodeID, where string, want string) (any, error) {
	n, ok := b.nodeOf(id)
	if !isKindType(want) {
		if !ok {
			n = nil
		}
		return nil, literalError(n, where, want, !s.Positional)
	}
	if !ok {
		return nil, Errorf(EArgType, "%s: unknown node #%d", where, id)
	}
	if !acceptsNode(want, n) {
		return nil, wrongNode(where, want, n)
	}
	b.inputs = append(b.inputs, id) // every node argument is its own input: [img, img] is two
	return InputRef{Index: b.base + len(b.inputs) - 1}, nil
}

// AwaitExpr is the expression a fix awaits for node n: img.stats() for an
// op called on the opened image, node.stats() on another node, img for an
// image.
func AwaitExpr(n *Node) string {
	if n.Kind == KindImage {
		return "img"
	}
	if !n.HasReceiver() || len(n.Args.ins) == 0 {
		return callName(n.Op) + "(…)"
	}
	recv := "node"
	r := n.Args.ins[0]
	for r.HasReceiver() && (r.Op.Name == AsOp || r.Op.Name == ToGrayOp) && len(r.Args.ins) > 0 {
		r = r.Args.ins[0] // conversions the builder inserts (and a grey plane) do not change what to write
	}
	if r.Op.On == KindNone {
		recv = "img"
	}
	return recv + "." + n.Op.Name + "()"
}

// literalError is the E_ARG_TYPE of a node passed where an option (a literal
// value) is expected; its fix awaits the node and reads a field (n is nil
// when the node is not known, as for function options).
func literalError(n *Node, where, want string, option bool) *Error {
	field := "<field>"
	expr := "x"
	if n != nil {
		for _, f := range n.Fields {
			if f.Name == "p99" {
				field = "p99"
				break
			}
		}
		if field == "<field>" && len(n.Fields) > 0 {
			field = n.Fields[0].Name
		}
		switch n.Kind {
		case KindValue:
			expr = AwaitExpr(n)
		case KindImage:
			expr, field = "img.stats()", "p99"
		case KindRegions:
			expr, field = AwaitExpr(n), "items"
		case KindNone:
		}
	}
	fix := fmt.Sprintf("(await %s).%s", expr, field)
	if seg := lastSegment(where); option && identifier(seg) {
		fix = fmt.Sprintf("{ %s: %s }", seg, fix)
	}
	return Errorf(EArgType, "%s must be %s, not a node: options are literal values.", where, A(t2noun(want))).
		WithHint("A measurement used as an option is awaited first; the number is then known when the node is built.").
		WithFix(fix)
}

// isKindType reports whether typ names node types ("Image", "Mask",
// "Regions|Mask").
func isKindType(typ string) bool {
	if typ == "" {
		return false
	}
	for _, part := range strings.Split(typ, "|") {
		if k := Kind(part); k != KindImage && k != KindRegions && part != MaskType && part != ItemsType {
			return false
		}
	}
	return true
}

// wrongNode is the E_WRONG_KIND of a node argument of the wrong kind or,
// for a mask, the wrong channel count (§7.5).
func wrongNode(where, want string, n *Node) *Error {
	if n.Kind == KindImage && takesMask(want) {
		what := "a one-channel image (a mask)"
		if takesItems(want) || accepts(Kind(want), KindRegions) {
			what += " or regions"
		}
		return Errorf(EWrongKind, "%s needs %s; this image has %d channels.", where, what, n.Channels).
			WithHint("A mask is any one-channel image (weights in [0, 1]): toGray(), a mask op such as img.mask.luminance({ above: 0.5 }), or img.mask.regions(regions).").
			WithFix(AwaitExpr(n) + ".toGray()")
	}
	e := Errorf(EWrongKind, "%s wants %s node, got %s node.", where, A(want), A(string(n.Kind)))
	switch {
	case n.Kind == KindRegions && takesMask(want):
		e.Hint = "A mask of regions is img.mask.regions(regions)."
		e.Fix = "img.mask.regions(regions)"
	case n.Kind == KindImage && n.Channels == 1 && (takesItems(want) || accepts(Kind(want), KindRegions)):
		e.Hint = "Find the regions of a mask with .components()."
	}
	return e
}

// checkChannels refuses a receiver of the wrong channel count at build time
// (OpDef.OneChannel, OpDef.ThreeChannel).
func checkChannels(op *OpDef, recv *Node) error {
	switch {
	case recv.Kind != KindImage || recv.Channels == 0:
	case op.OneChannel && recv.Channels != 1:
		return Errorf(EWrongKind, "%s needs a one-channel image (a mask or a grey image); this image has %d channels.", callName(op), recv.Channels).
			WithFix(AwaitExpr(recv) + ".toGray()." + op.Name + "(…)")
	case op.ThreeChannel && recv.Channels != 3:
		return Errorf(EWrongKind, "%s needs a colour image; this image has one channel.", callName(op)).
			WithHint("A grey image or a mask has no colour to read; call it on the colour image.")
	}
	return nil
}

// identifier reports whether s is a plain JavaScript identifier (an option
// name, not a list index).
func identifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		letter := r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !letter && (i == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func lastSegment(where string) string {
	if i := strings.LastIndexAny(where, ".("); i >= 0 {
		return strings.TrimSuffix(where[i+1:], ")")
	}
	return where
}

func t2noun(t string) string {
	switch {
	case strings.HasPrefix(t, "enum:"):
		return "string"
	case t == "number[]" || t == "string[]":
		return "list"
	case t == "box":
		return "box [x, y, w, h]"
	case t == "point":
		return "point [x, y]"
	}
	return t
}

// check validates and canonicalizes v against type typ of spec s. A node is
// accepted only for an argument of node type (it becomes an input).
func (b *binder) check(s *ArgSpec, typ string, v any, where string) (any, error) {
	if typ == "data" { // a node or plain data, kept as given (cv.save's x)
		return v, nil
	}
	if takesItems(typ) {
		return b.checkItems(s, typ, v, where)
	}
	if na, ok := v.(NodeArg); ok {
		if isUnionType(typ) {
			return b.checkUnion(s, typ, v, where)
		}
		return b.lift(s, na.ID, where, typ)
	}
	if isKindType(typ) {
		e := Errorf(EWrongKind, "%s needs %s node, got %s.", where, A(typ), describeRaw(v))
		if _, isData := v.(map[string]any); isData {
			e.Hint = "That is plain data, not a node. Keep the node in a variable and await it only to read data."
			switch {
			case accepts(Kind(typ), KindRegions):
				e.Fix = fmt.Sprintf("const regions = img.detect.specks()   // a node; then %s(regions, …)", b.callName())
			case takesMask(typ):
				e.Fix = fmt.Sprintf("const m = img.mask.luminance({ above: 0.6 })   // a node; then %s(m, …)", b.callName())
			default:
				e.Fix = fmt.Sprintf("const other = img.exposure(0.3)   // a node; then %s(other, …)", b.callName())
			}
		}
		return nil, e
	}
	switch {
	case isUnionType(typ):
		return b.checkUnion(s, typ, v, where)
	case typ == "number":
		f, ok := v.(float64)
		if !ok {
			return nil, typeErr(where, typ, v)
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, Errorf(EArgRange, "%s is %v; it must be a finite number.", where, f)
		}
		if (s.Min != nil && f < *s.Min) || (s.Max != nil && f > *s.Max) {
			return nil, Errorf(EArgRange, "%s = %s is outside the allowed range %s.", where, fmtNum(f), fmtLimits(s)).
				WithValid([]string{fmtLimits(s)})
		}
		if s.hasUsual() && (f < s.Usual[0] || f > s.Usual[1]) {
			b.warnings = append(b.warnings, Warning{Code: WRange,
				Message: strings.TrimSpace(fmt.Sprintf("%s = %s is outside the usual range [%s, %s]. %s", where, fmtNum(f), fmtNum(s.Usual[0]), fmtNum(s.Usual[1]), s.UsualHint))})
		}
		return f, nil
	case typ == "string":
		if _, ok := v.(string); !ok {
			return nil, typeErr(where, typ, v)
		}
		return v, nil
	case typ == "bool":
		if _, ok := v.(bool); !ok {
			return nil, typeErr(where, typ, v)
		}
		return v, nil
	case strings.HasPrefix(typ, "enum:"):
		str, ok := v.(string)
		allowed := strings.Split(strings.TrimPrefix(typ, "enum:"), ",")
		if !ok {
			return nil, typeErr(where, typ, v).WithValid(allowed)
		}
		for _, a := range allowed {
			if a == str {
				return str, nil
			}
		}
		e := Errorf(EArgType, "%s = %q is not one of %s.", where, str, quoteList(allowed)).WithValid(allowed)
		if sug := closest(str, allowed); sug != "" {
			e.Hint = fmt.Sprintf("Did you mean %q?", sug)
		}
		if len(s.ValueDocs) > 0 {
			var docs []string
			for _, a := range allowed {
				docs = append(docs, a+": "+s.ValueDocs[a])
			}
			e.Hint = strings.TrimSpace(e.Hint + " " + strings.Join(docs, "; ") + ".")
		}
		return nil, e
	case strings.HasSuffix(typ, "[]"):
		list, ok := v.([]any)
		if !ok {
			return nil, typeErr(where, typ, v)
		}
		if s.Len > 0 && len(list) != s.Len {
			return nil, Errorf(EArgType, "%s must be a list of %d values, got %d.", where, s.Len, len(list)).
				WithHint(lenHint(s.Len))
		}
		elem := strings.TrimSuffix(typ, "[]")
		out := make([]any, len(list))
		for i, e := range list {
			c, err := b.check(s, elem, e, fmt.Sprintf("%s[%d]", where, i))
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	case typ == "box":
		return b.checkBox(s, v, where)
	case typ == "point":
		return b.checkPoint(s, v, where)
	case typ == "object":
		m, ok := v.(map[string]any)
		if !ok {
			return nil, typeErr(where, typ, v)
		}
		return b.bindObject(s.Options, m, where, s.Name)
	case typ == "any":
		return b.checkAny(v, where)
	}
	return nil, Errorf(EInternal, "%s: unknown arg type %q", where, typ)
}

// isUnionType reports whether typ is a union of value types such as
// "point|number" (kind unions such as "Regions|Mask" are kind types).
func isUnionType(typ string) bool {
	return strings.Contains(typ, "|") && !isKindType(typ)
}

// checkUnion checks v against the alternative of typ that fits its shape:
// strings against string and enum alternatives, numbers against "number",
// lists and objects against the others.
func (b *binder) checkUnion(s *ArgSpec, typ string, v any, where string) (any, error) {
	alts := strings.Split(typ, "|")
	if na, isNode := v.(NodeArg); isNode { // a node of a kind alternative ("Image|number") is an input of that kind
		if n, ok := b.nodeOf(na.ID); ok {
			for _, alt := range alts {
				if isKindType(alt) && accepts(Kind(alt), n.Kind) {
					return b.check(s, alt, v, where)
				}
			}
		}
		return b.lift(s, na.ID, where, typ)
	}
	pick := func(match func(alt string) bool) (string, bool) {
		for _, alt := range alts {
			if match(alt) {
				return alt, true
			}
		}
		return "", false
	}
	var alt string
	var ok bool
	switch v.(type) {
	case string:
		alt, ok = pick(func(a string) bool { return a == "string" || strings.HasPrefix(a, "enum:") })
	case float64:
		alt, ok = pick(func(a string) bool { return a == "number" })
	case bool:
		alt, ok = pick(func(a string) bool { return a == "bool" })
	case []any:
		alt, ok = pick(func(a string) bool { return strings.HasSuffix(a, "[]") || a == "box" || a == "point" })
	default:
		alt, ok = pick(func(a string) bool {
			return a != "number" && a != "string" && a != "bool" && !strings.HasSuffix(a, "[]") && !strings.HasPrefix(a, "enum:")
		})
	}
	if !ok {
		names := make([]string, len(alts))
		for i, a := range alts {
			names[i] = t2noun(a)
		}
		return nil, Errorf(EArgType, "%s must be %s, got %s.", where, strings.Join(names, " or "), describeRaw(v))
	}
	return b.check(s, alt, v, where)
}

// boxKeys and pointKeys are the object forms of the box and point types:
// x, y is always a corner, cx, cy always a point or a centre.
var (
	boxKeys   = []string{"x", "y", "w", "h"}
	pointKeys = []string{"cx", "cy"}
)

func hasNumericKeys(m map[string]any, keys []string) bool {
	for _, k := range keys {
		switch m[k].(type) {
		case float64:
		default:
			return false
		}
	}
	return true
}

// isCoordinateObject reports whether v is the object form of a box or point
// (so a positional box or point can be an object without being taken for an
// options object).
func isCoordinateObject(typ string, v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for _, alt := range strings.Split(typ, "|") {
		switch alt {
		case "box":
			if hasNumericKeys(m, boxKeys) {
				return true
			}
		case "point":
			if hasNumericKeys(m, pointKeys) || hasNumericKeys(m, boxKeys[:2]) {
				return true
			}
		}
	}
	return false
}

// coords checks the elements of a box or point list (numbers) and returns
// the canonical list.
func (b *binder) coords(s *ArgSpec, list []any, where string) (any, error) {
	out := make([]any, len(list))
	for i, e := range list {
		c, err := b.check(s, "number", e, fmt.Sprintf("%s[%d]", where, i))
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// checkBox accepts [x, y, w, h] or an object with numeric x, y, w, h (other
// keys are ignored, so a region item works) and returns [x, y, w, h].
func (b *binder) checkBox(s *ArgSpec, v any, where string) (any, error) {
	switch x := v.(type) {
	case []any:
		if len(x) != 4 {
			return nil, Errorf(EArgType, "%s must be a box [x, y, w, h], got a list of %d values.", where, len(x)).
				WithHint(lenHint(4))
		}
		return b.coords(s, x, where)
	case map[string]any:
		if !hasNumericKeys(x, boxKeys) {
			return nil, Errorf(EArgType, "%s must be a box: [x, y, w, h] or an object with numeric x, y, w, h.", where).
				WithHint("x, y is the box corner, w, h its size, in doc px.")
		}
		return b.coords(s, []any{x["x"], x["y"], x["w"], x["h"]}, where)
	}
	return nil, Errorf(EArgType, "%s must be a box [x, y, w, h] (doc px), got %s.", where, describeRaw(v)).WithHint(lenHint(4))
}

// checkPoint accepts [x, y] or an object with numeric cx, cy and returns
// [x, y]. An object with only x, y is refused: x, y names a box corner.
func (b *binder) checkPoint(s *ArgSpec, v any, where string) (any, error) {
	switch x := v.(type) {
	case []any:
		if len(x) != 2 {
			return nil, Errorf(EArgType, "%s must be a point [x, y], got a list of %d values.", where, len(x)).
				WithHint(lenHint(2))
		}
		return b.coords(s, x, where)
	case map[string]any:
		if hasNumericKeys(x, pointKeys) {
			return b.coords(s, []any{x["cx"], x["cy"]}, where)
		}
		if hasNumericKeys(x, boxKeys[:2]) {
			return nil, Errorf(EArgType, "%s: x, y is a box corner; a point is [x, y] or an object with cx, cy.", where).
				WithHint("Region items and measurement rows name their centre cx, cy and their box corner x, y; pass the centre, or the corner as a list if you mean it.").
				WithFix(fmt.Sprintf("[%s, %s]   // [obj.x, obj.y]; or { cx: obj.cx, cy: obj.cy } for the centre", fmtCoord(x["x"]), fmtCoord(x["y"])))
		}
		return nil, Errorf(EArgType, "%s must be a point: [x, y] or an object with numeric cx, cy.", where)
	}
	return nil, Errorf(EArgType, "%s must be a point [x, y] (doc px), got %s.", where, describeRaw(v)).WithHint(lenHint(2))
}

func fmtCoord(v any) string {
	if f, ok := v.(float64); ok {
		return fmtNum(f)
	}
	return "…"
}

func (b *binder) checkAny(v any, where string) (any, error) {
	switch x := v.(type) {
	case NodeArg:
		return b.check(&ArgSpec{}, "number", x, where)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			c, err := b.checkAny(e, fmt.Sprintf("%s[%d]", where, i))
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for _, k := range sortedKeys(x) {
			c, err := b.checkAny(x[k], where+"."+k)
			if err != nil {
				return nil, err
			}
			out[k] = c
		}
		return out, nil
	}
	return v, nil
}

// bindObject checks an options object against specs: unknown keys are
// errors, defaults are filled, values are checked in spec order.
func (b *binder) bindObject(specs []ArgSpec, m map[string]any, where, what string) (map[string]any, error) {
	if err := checkKeys(specs, m, what, func(sug string) string { return where + "." + sug }); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for i := range specs {
		s := &specs[i]
		v, ok := m[s.Name]
		if !ok || v == nil {
			if s.Default != nil {
				out[s.Name] = normalize(s.Default)
			} else if !s.Optional {
				return nil, Errorf(EArgType, "%s.%s is required.", where, s.Name)
			}
			continue
		}
		c, err := b.check(s, s.Type, v, where+"."+s.Name)
		if err != nil {
			return nil, err
		}
		out[s.Name] = c
	}
	return out, nil
}

// checkKeys rejects keys of m that are not in specs. fix renders the literal
// fix for a suggested key.
func checkKeys(specs []ArgSpec, m map[string]any, what string, fix func(sug string) string) error {
	var valid []string
	known := map[string]bool{}
	for _, s := range specs {
		valid = append(valid, s.Name)
		known[s.Name] = true
	}
	for _, k := range sortedKeys(m) {
		if !known[k] {
			list := ": it takes none"
			if len(valid) > 0 {
				list = ". Valid options: " + strings.Join(valid, ", ")
			}
			e := Errorf(EUnknownOption, "%s has no option %q%s.", what, k, list).WithValid(valid)
			if r, ok := optionRedirect(specs, k); ok {
				e.Hint, e.Fix = r.Hint, r.Fix
				return e
			}
			if sug := closest(k, valid); sug != "" {
				e.Hint = fmt.Sprintf("Did you mean %q?", sug)
				e.Fix = fix(sug)
			}
			return e
		}
	}
	return nil
}

// optionRedirect finds the redirect of an unknown option name among specs.
func optionRedirect(specs []ArgSpec, k string) (Redirect, bool) {
	for i := range specs {
		if r, ok := specs[i].Redirects[k]; ok {
			return r, true
		}
	}
	return Redirect{}, false
}

func lenHint(n int) string {
	switch n {
	case 2:
		return "A point is [x, y] in doc px."
	case 4:
		return "A box is [x, y, w, h] in doc px."
	}
	return ""
}

func typeErr(where, typ string, v any) *Error {
	return Errorf(EArgType, "%s must be %s, got %s.", where, A(t2noun(typ)), describeRaw(v))
}

func describeRaw(v any) string {
	switch x := v.(type) {
	case nil:
		return "nothing"
	case float64:
		return "the number " + fmtNum(x)
	case string:
		return strconv.Quote(x)
	case bool:
		return strconv.FormatBool(x)
	case []any:
		return "a list"
	case map[string]any:
		return "an object"
	}
	return fmt.Sprintf("%T", v)
}

func fmtNum(f float64) string { return strconv.FormatFloat(f, 'g', 6, 64) }

func fmtLimits(s *ArgSpec) string {
	lo, hi := "-∞", "∞"
	if s.Min != nil {
		lo = fmtNum(*s.Min)
	}
	if s.Max != nil {
		hi = fmtNum(*s.Max)
	}
	return "[" + lo + ", " + hi + "]"
}

func quoteList(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = strconv.Quote(x)
	}
	return strings.Join(q, ", ")
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// bound is the result of binding a call's raw arguments.
type bound struct {
	args     map[string]any
	inputs   []NodeID // one per node argument, in binding order (not the receiver)
	warnings []Warning
}

func (b *binder) callName() string {
	if b.op == nil {
		return "op"
	}
	return callName(b.op)
}

// bind maps raw call arguments onto op: explicit node inputs first, then
// positional args in declaration order, then an optional trailing options
// object whose keys may name any argument.
func bind(op *OpDef, call string, hasReceiver bool, raw []any, nodeOf func(NodeID) (*Node, bool), items func([]any, NodeID) (NodeID, error)) (*bound, error) {
	raw = normalize(trimUndefined(raw)).([]any)
	res := &bound{}
	i := 0
	given := map[string]any{}
	var positional []*ArgSpec
	for k := range op.Args {
		if op.Args[k].Positional {
			positional = append(positional, &op.Args[k])
		}
	}
	for _, s := range positional {
		if i >= len(raw) {
			break
		}
		if s.Variadic {
			var rest []any
			// a trailing options object ends the variadic run unless the element type takes objects
			trailingOptions := func(i int) bool {
				return isOptionsObject(raw[i]) && i == len(raw)-1 && !typeAcceptsObject(strings.TrimSuffix(s.Type, "[]"))
			}
			for i < len(raw) && !trailingOptions(i) {
				rest = append(rest, raw[i])
				i++
			}
			given[s.Name] = rest
			continue
		}
		requiredNode := (isKindType(s.Type) || takesItems(s.Type)) && s.Default == nil && !s.Optional // an object there is data passed for a node
		if isOptionsObject(raw[i]) && !typeAcceptsObject(s.Type) && !isCoordinateObject(s.Type, raw[i]) && !requiredNode {
			break
		}
		given[s.Name] = raw[i]
		i++
	}
	if i < len(raw) {
		opts, ok := raw[i].(map[string]any)
		if !ok || i != len(raw)-1 {
			return nil, Errorf(EArgType, "%s takes at most %d positional argument(s); got %d.", call, len(positional), len(raw)).
				WithHint(signatureHint(op, call))
		}
		fix := func(sug string) string {
			if call == "cv.open" {
				return fmt.Sprintf("cv.open(path, { %s: %s })", sug, fmtFixValue(opts, sug))
			}
			return fmt.Sprintf("%s({ %s: %s })", call, sug, fmtFixValue(opts, sug))
		}
		if err := checkKeys(op.Args, opts, call, fix); err != nil {
			return nil, err
		}
		for _, k := range sortedKeys(opts) {
			if _, dup := given[k]; dup {
				return nil, Errorf(EArgType, "%s: %s given both positionally and as an option.", call, k)
			}
			given[k] = opts[k]
		}
	}

	b := &binder{op: op, nodeOf: nodeOf, items: items}
	if hasReceiver {
		b.base = 1
	}
	out := map[string]any{}
	for k := range op.Args {
		s := &op.Args[k]
		v, ok := given[s.Name]
		if !ok || v == nil {
			switch {
			case s.Default != nil:
				out[s.Name] = normalize(s.Default)
			case s.Variadic:
				out[s.Name] = []any{}
			case !s.Optional:
				return nil, Errorf(EArgType, "%s is missing its %s argument.", call, s.Name).WithHint(signatureHint(op, call))
			}
			continue
		}
		if !takesItems(s.Type) {
			v = stripRefs(v) // an item passed as a box, a point or data is its fields
		}
		c, err := b.check(s, s.Type, v, call+"."+s.Name)
		if err != nil {
			return nil, withEnumFix(err, call, s)
		}
		out[s.Name] = c
	}
	res.args, res.inputs, res.warnings = out, b.inputs, b.warnings
	return res, nil
}

// withEnumFix gives an enum error without a fix the literal fix
// call({ name: "value" }), with the suggested value or the first valid one.
func withEnumFix(err error, call string, s *ArgSpec) error {
	ce, ok := AsError(err)
	if !ok || ce.Code != EArgType || ce.Fix != "" || len(ce.Valid) == 0 || !strings.Contains(s.Type, "enum:") {
		return err
	}
	v := ce.Valid[0]
	if rest, ok := strings.CutPrefix(ce.Hint, `Did you mean "`); ok {
		if i := strings.Index(rest, `"`); i > 0 {
			v = rest[:i]
		}
	}
	switch {
	case call == "cv.open":
		ce.Fix = fmt.Sprintf("cv.open(path, { %s: %q })", s.Name, v)
	case s.Positional && !strings.Contains(s.Type, "|"):
		ce.Fix = fmt.Sprintf("%s(%q)", call, v)
	default:
		ce.Fix = fmt.Sprintf("%s({ %s: %q })", call, s.Name, v)
	}
	return ce
}

// fmtFixValue renders the value the agent passed under the misspelled key
// (the only key of opts that is unknown), for the literal fix.
func fmtFixValue(opts map[string]any, _ string) string {
	for _, v := range opts {
		if f, ok := v.(float64); ok && len(opts) == 1 {
			return fmtNum(f)
		}
	}
	return "…"
}

func trimUndefined(raw []any) []any {
	for len(raw) > 0 && raw[len(raw)-1] == nil {
		raw = raw[:len(raw)-1]
	}
	return raw
}

func callName(op *OpDef) string {
	if op.On == KindNone && (op.Name == "open" || !op.Bespoke) {
		return "cv." + op.Name
	}
	return op.Name
}

// signatureHint renders "exposure(ev)" style usage of op called as call.
func signatureHint(op *OpDef, call string) string {
	var pos, opt []string
	for _, a := range op.Args {

		if a.Positional {
			n := a.Name
			if a.Variadic {
				n = "..." + n
			}
			pos = append(pos, n)
		} else {
			opt = append(opt, a.Name)
		}
	}
	if len(opt) > 0 {
		pos = append(pos, "{ "+strings.Join(opt, ", ")+" }")
	}
	return "Usage: " + call + "(" + strings.Join(pos, ", ") + ")"
}

// resolveArgs gives a node's canonical args their evaluated inputs: node
// arguments stay InputRefs that Value and Values resolve.
func resolveArgs(n *Node, ins []Value) Args {
	a := n.Args
	a.vs = ins
	return a
}

// Normalize converts exported JavaScript values (int64, float32, typed
// slices, nested maps) into the canonical value space used by args and data.
func Normalize(v any) any { return normalize(v) }
