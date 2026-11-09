package graph

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// NodeArg is a node passed as an argument value in a raw call (from JS).
type NodeArg struct{ ID NodeID }

// FuncID identifies a JavaScript function registered by the binding.
type FuncID int64

// FuncArg is a JavaScript function passed as an argument. Only arguments of
// type "func" (data lambdas such as Regions.filter) accept it; builder
// lambdas (within) are run by the prelude and arrive as BuiltArg.
type FuncArg struct{ ID FuncID }

// MarshalJSON encodes the function reference for hashing and display.
func (f FuncArg) MarshalJSON() ([]byte, error) {
	return []byte(`{"$fn":` + strconv.FormatInt(int64(f.ID), 10) + `}`), nil
}

// InputRef is a lifted node argument inside canonical args: it refers to
// node.Inputs[Index].
type InputRef struct{ Index int }

// MarshalJSON encodes the reference for hashing and display.
func (r InputRef) MarshalJSON() ([]byte, error) {
	return []byte(`{"$in":` + strconv.Itoa(r.Index) + `}`), nil
}

// Args are an op's canonical arguments: defaults filled, numbers normalized
// to float64, node-valued arguments recorded as InputRefs (before
// evaluation) or substituted with their evaluated data (inside Run).
type Args struct {
	op   *OpDef
	vals map[string]any
	meta any // typed Go-side metadata of a source node (SourceMeta, PlaceholderMeta); nil otherwise
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

// Has reports whether the argument is present.
func (a Args) Has(name string) bool { _, ok := a.vals[name]; return ok }

// Raw returns the canonical value (nil if absent).
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

func (a Args) number(name string) float64 {
	v, ok := a.vals[name].(float64)
	if !ok {
		panic(fmt.Sprintf("cv: %s.%s is %T, not a resolved number", a.op.Name, name, a.vals[name]))
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
// y]) converted to buffer px at scale. For an "any" argument it returns nil
// when the value is not a list.
func (a Args) Lengths(name string, scale float64) []float64 {
	if s := a.mustSpec(name); s.Unit != DocPx {
		panic(fmt.Sprintf("cv: %s.%s is not a doc-px list", a.op.Name, name))
	}
	list, ok := a.vals[name].([]any)
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
	var out [4]float64
	copy(out[:], l)
	return out
}

// AxisScales are the effective buffer-px-per-doc-px scales of an image
// value along x and y.
func AxisScales(v *ImageValue) (float64, float64) {
	return float64(v.Buf.W) / float64(max(1, v.Frame.W)), float64(v.Buf.H) / float64(max(1, v.Frame.H))
}

// Func reads a "func" argument.
func (a Args) Func(name string) FuncID {
	a.mustSpec(name)
	f, _ := a.vals[name].(FuncArg)
	return f.ID
}

// Node reads a kind-typed argument: the evaluated value of the node passed
// for it (nil if the argument is absent).
func (a Args) Node(name string) Value {
	a.mustSpec(name)
	v, _ := a.vals[name].(Value)
	return v
}

// Nodes reads a list of kind-typed values ("Image[]").
func (a Args) Nodes(name string) []Value {
	a.mustSpec(name)
	list, _ := a.vals[name].([]any)
	out := make([]Value, 0, len(list))
	for _, e := range list {
		if v, ok := e.(Value); ok {
			out = append(out, v)
		}
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
	a.mustSpec(name)
	s, _ := a.vals[name].(string)
	return s
}

// Bool reads a bool argument.
func (a Args) Bool(name string) bool {
	a.mustSpec(name)
	b, _ := a.vals[name].(bool)
	return b
}

// Numbers reads a unitless "number[]" argument.
func (a Args) Numbers(name string) []float64 {
	if s := a.mustSpec(name); s.Unit == DocPx || s.Unit == DocPx2 {
		panic(fmt.Sprintf("cv: %s.%s is a doc-px list; read it with Args.Lengths", a.op.Name, name))
	}
	list, _ := a.vals[name].([]any)
	out := make([]float64, 0, len(list))
	for _, v := range list {
		f, _ := v.(float64)
		out = append(out, f)
	}
	return out
}

// Strings reads a "string[]" argument.
func (a Args) Strings(name string) []string {
	a.mustSpec(name)
	list, _ := a.vals[name].([]any)
	out := make([]string, 0, len(list))
	for _, v := range list {
		s, _ := v.(string)
		out = append(out, s)
	}
	return out
}

// Object reads an "object" or "any" argument.
func (a Args) Object(name string) map[string]any {
	a.mustSpec(name)
	m, _ := a.vals[name].(map[string]any)
	return m
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
// float64, string, bool, []any, map[string]any, NodeArg, FuncArg, nil.
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
	kindOf   func(NodeID) (Kind, bool)
	base     int // index of the first lifted input in node.Inputs
	lifted   []NodeID
	liftIdx  map[NodeID]int
	warnings []Warning
}

func (b *binder) lift(id NodeID, where string, want string) (any, error) {
	k, ok := b.kindOf(id)
	if !ok {
		return nil, Errorf(EArgType, "%s: unknown node #%d", where, id)
	}
	if isKindType(want) {
		if !accepts(Kind(want), k) {
			return nil, Errorf(EWrongKind, "%s wants %s node, got %s node.", where, A(want), A(string(k))).
				WithHint(kindHint(Kind(want), k))
		}
	} else if k != KindValue && k != KindParam {
		return nil, Errorf(EWrongKind, "%s wants %s, got %s node.", where, A(t2noun(want)), A(string(k))).
			WithHint("Only Value and Param nodes can stand in for plain values; measure the image first.").
			WithFix(fmt.Sprintf("e.g. %s: img.stats().p01", lastSegment(where)))
	}
	if i, ok := b.liftIdx[id]; ok {
		return InputRef{Index: b.base + i}, nil
	}
	b.liftIdx[id] = len(b.lifted)
	b.lifted = append(b.lifted, id)
	return InputRef{Index: b.base + len(b.lifted) - 1}, nil
}

// isKindType reports whether typ names node kinds ("Mask", "Regions|Mask").
func isKindType(typ string) bool {
	if typ == "" {
		return false
	}
	for _, part := range strings.Split(typ, "|") {
		if k := Kind(part); k != KindImage && k != KindMask && k != KindRegions {
			return false
		}
	}
	return true
}

func kindHint(want, got Kind) string {
	switch {
	case got == KindRegions && accepts(want, KindMask):
		return "Convert regions with .toMask()."
	case got == KindMask && accepts(want, KindRegions):
		return "Convert a mask with .components()."
	case got == KindImage && accepts(want, KindMask):
		return "Make a mask from the image, e.g. img.mask.luminance({ above: 0.5 })."
	}
	return ""
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

// check validates and canonicalizes v against type typ of spec s. With b
// non-nil, NodeArgs are lifted; with b nil (evaluation time, after
// substitution) they are an error.
func (b *binder) check(s *ArgSpec, typ string, v any, where string) (any, error) {
	if typ == "data" { // a node or plain data, kept as given (cv.json, cv.csv)
		return v, nil
	}
	if na, ok := v.(NodeArg); ok {
		if b == nil {
			return nil, Errorf(EInternal, "%s: unresolved node argument", where)
		}
		if s.Static {
			return nil, Errorf(EArgType, "%s must be a literal value, not a node.", where).
				WithHint("This argument decides how the node is evaluated; it cannot depend on a measurement or a param.")
		}
		if isUnionType(typ) {
			return b.checkUnion(s, typ, v, where)
		}
		return b.lift(na.ID, where, typ)
	}
	if isKindType(typ) {
		if val, ok := v.(Value); ok && b == nil && accepts(Kind(typ), val.Kind()) {
			return val, nil
		}
		return nil, Errorf(EWrongKind, "%s needs %s node, got %s.", where, A(typ), describeRaw(v))
	}
	if fa, ok := v.(FuncArg); ok {
		if typ == "func" {
			return fa, nil
		}
		return nil, Errorf(EArgType, "%s does not accept a function.", where).
			WithHint("Only within(mask, i => …) (a builder) and Regions.filter(r => …) (a data lambda) take functions; pass a value or a node.")
	}
	if typ == "func" {
		return nil, Errorf(EArgType, "%s must be a function, got %s.", where, describeRaw(v)).
			WithFix("regions.filter(r => r.contrast > 3 * r.localNoise)")
	}
	if _, ok := v.(InputRef); ok && b == nil {
		return nil, Errorf(EInternal, "%s: unresolved input reference", where)
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
		if b != nil && s.hasUsual() && (f < s.Usual[0] || f > s.Usual[1]) {
			b.warnings = append(b.warnings, Warning{Code: WRange,
				Message: fmt.Sprintf("%s = %s is outside the usual range [%s, %s].", where, fmtNum(f), fmtNum(s.Usual[0]), fmtNum(s.Usual[1]))})
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
		if r, ok := s.Redirects[str]; ok {
			e.Hint, e.Fix = r.Hint, r.Fix
		} else if sug, ok := s.ValueSynonyms[str]; ok {
			e.Hint = fmt.Sprintf("Did you mean %q?", sug)
		} else if sug := closest(str, allowed); sug != "" {
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
	if na, isNode := v.(NodeArg); isNode {
		if b != nil { // a node of a kind alternative ("Image|number") is an input of that kind
			if k, ok := b.kindOf(na.ID); ok {
				for _, alt := range alts {
					if isKindType(alt) && accepts(Kind(alt), k) {
						return b.check(s, alt, v, where)
					}
				}
			}
		}
		return b.check(s, "number", v, where) // a lifted Value: checked again after substitution
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
		case float64, NodeArg, InputRef:
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

// coords checks the elements of a box or point list (numbers or lifted
// Values) and returns the canonical list.
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
	case NodeArg, FuncArg:
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
			e := Errorf(EUnknownOption, "%s has no option %q. Valid options: %s.", what, k, strings.Join(valid, ", ")).WithValid(valid)
			if r, ok := optionRedirect(specs, k); ok {
				e.Hint, e.Fix = r.Hint, r.Fix
				return e
			}
			sug, semantic := optionSynonym(k, known), true
			if sug == "" {
				sug, semantic = closest(k, valid), false
			}
			if sug != "" {
				e.Hint = fmt.Sprintf("Did you mean %q?", sug)
				for _, s := range specs {
					if semantic && s.Name == sug && s.Doc != "" { // explain why a different word maps here
						e.Hint += " (" + s.Doc + ")"
					}
				}
				e.Fix = fix(sug)
			}
			return e
		}
	}
	return nil
}

// OptionSynonyms maps option names agents reach for to the cv option names
// they usually mean, in order of preference; the first one the op has wins
// ({ min: 0.99 } on mask.luminance → above; { radius } on gaussian → r).
var OptionSynonyms = map[string][]string{
	"min": {"above", "minArea", "minContrast"}, "minimum": {"above", "minArea"}, "lower": {"above"}, "from": {"above"},
	"max": {"below", "maxArea"}, "maximum": {"below", "maxArea"}, "upper": {"below"}, "to": {"below"},
	"threshold": {"above", "minContrast", "threshold"}, "thresh": {"above", "minContrast", "threshold"}, "cutoff": {"above", "minContrast"},
	"r": {"sigma", "radius", "coreRadius"}, "radius": {"sigma", "coreRadius"}, "sigma": {"radius"}, "core": {"coreRadius"}, "coreR": {"coreRadius"},
	"annulus": {"ringRadii"}, "background": {"ringRadii"}, "ring": {"ringRadii"}, "rings": {"ringRadii"}, "ringRadius": {"ringRadii"},
	"thickness": {"width"}, "fields": {"columns"}, "bits": {"depth"}, "bitDepth": {"depth"}, "std": {"sigma"}, "stddev": {"sigma"}, "sd": {"sigma"},
	"size": {"sigma", "radius", "maxArea", "maxSide"}, "blur": {"sigma"}, "kernel": {"sigma", "radius"}, "width": {"sigma", "radius"},
	"distance": {"radius"}, "dist": {"radius"},
	"maxSize": {"maxArea"}, "minSize": {"minArea"}, "area": {"maxArea", "minArea"}, "contrast": {"minContrast"},
	"opacity": {"alpha", "opacity"}, "alpha": {"opacity", "alpha"}, "strength": {"amount", "opacity", "alpha"}, "intensity": {"amount", "opacity"},
	"colour": {"color"}, "label": {"labels", "label"}, "showIds": {"labels"}, "ids": {"labels"},
	"tolerance": {"tol"}, "soft": {"softness"}, "feather": {"softness"}, "smooth": {"softness"},
	"scale": {"analysisScale", "zoom"}, "type": {"polarity", "mode", "method"}, "kind": {"polarity", "mode"},
	"low": {"black", "below"}, "lo": {"black", "below"}, "blackPoint": {"black"}, "shadows": {"black", "below"},
	"high": {"white", "above"}, "hi": {"white", "above"}, "whitePoint": {"white"}, "highlights": {"white", "above"},
	"midtones": {"gamma"}, "mid": {"gamma"}, "quantiles": {"percentiles"}, "percentile": {"percentiles"},
	"region": {"mask", "box"}, "regions": {"mask"}, "selection": {"mask"}, "rect": {"box"}, "rectangle": {"box"}, "roi": {"box", "mask"},
	"window": {"box"}, "crop": {"box"}, "colorSpace": {"encoding"}, "colorspace": {"encoding"}, "transfer": {"encoding"},
	"gamma": {"encoding"}, "workingSpace": {"space"}, "working_space": {"space"}, "offset": {"origin"},
	"amount": {"amount", "ev", "opacity"}, "stops": {"ev"}, "exposure": {"ev"},
	"long": {"maxSide"}, "longSide": {"maxSide"}, "maxsize": {"maxSide"}, "max_side": {"maxSide"}, "width_px": {"maxSide"},
	"caption": {"label"}, "title": {"label"}, "text": {"label"},
	"magnify": {"zoom"}, "magnification": {"zoom"}, "upscale": {"zoom"}, "factor": {"zoom"}, "zoomFactor": {"zoom"},
	"ticks": {"ruler"}, "axes": {"ruler"}, "axis": {"ruler"}, "coords": {"ruler"}, "coordinates": {"ruler"}, "rulers": {"ruler"}, "scaleBar": {"ruler"},
	"lossless": {"format"}, "png": {"format"}, "jpeg": {"format"}, "quality": {"format"}, "compression": {"format"}, "mime": {"format"},
	"gridlines": {"grid"}, "gridLines": {"grid"}, "lines": {"grid"},
	"interpolation": {"interp"}, "resample": {"interp"}, "resampling": {"interp"}, "upsample": {"interp"},
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

func optionSynonym(k string, known map[string]bool) string {
	for _, c := range OptionSynonyms[k] {
		if known[c] {
			return c
		}
	}
	return ""
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
	inputs   []NodeID // explicit node inputs (OpDef.Inputs), not the receiver
	lifted   []NodeID
	warnings []Warning
}

// bind maps raw call arguments onto op: explicit node inputs first, then
// positional args in declaration order, then an optional trailing options
// object whose keys may name any argument.
func bind(op *OpDef, call string, hasReceiver bool, raw []any, kindOf func(NodeID) (Kind, bool)) (*bound, error) {
	raw = normalize(trimUndefined(raw)).([]any)
	res := &bound{}
	i := 0
	for j, decl := range op.Inputs {
		if i >= len(raw) {
			return nil, Errorf(EArgType, "%s needs %s as argument %d.", call, A(string(decl)), j+1)
		}
		na, ok := raw[i].(NodeArg)
		if !ok {
			e := Errorf(EWrongKind, "%s needs %s node as argument %d, got %s.", call, A(string(decl)), j+1, describeRaw(raw[i]))
			if _, isData := raw[i].(map[string]any); isData {
				e.Hint = "That is plain data, not a node. Keep the node in a variable and await it only to read data."
				switch {
				case accepts(decl, KindRegions):
					e.Fix = fmt.Sprintf("const regions = img.detect.specks()   // a node; then %s(regions, …)", op.Name)
				case accepts(decl, KindMask):
					e.Fix = fmt.Sprintf("const m = img.mask.luminance({ above: 0.6 })   // a node; then %s(m, …)", op.Name)
				default:
					e.Fix = fmt.Sprintf("const other = img.exposure(0.3)   // a node; then %s(other, …)", op.Name)
				}
			}
			return nil, e
		}
		k, _ := kindOf(na.ID)
		if !accepts(decl, k) {
			return nil, Errorf(EWrongKind, "%s needs %s as argument %d, got %s.", call, A(string(decl)), j+1, A(string(k)))
		}
		res.inputs = append(res.inputs, na.ID)
		i++
	}

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
		if isOptionsObject(raw[i]) && !typeAcceptsObject(s.Type) && !isCoordinateObject(s.Type, raw[i]) {
			break
		}
		given[s.Name] = raw[i]
		i++
	}
	if i < len(raw) {
		opts, ok := raw[i].(map[string]any)
		if !ok || i != len(raw)-1 {
			return nil, Errorf(EArgType, "%s takes at most %d positional argument(s); got %d.", call, len(op.Inputs)+len(positional), len(raw)).
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

	b := &binder{op: op, kindOf: kindOf, liftIdx: map[NodeID]int{}}
	b.base = len(op.Inputs)
	if hasReceiver {
		b.base++
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
		c, err := b.check(s, s.Type, v, call+"."+s.Name)
		if err != nil {
			return nil, withEnumFix(err, call, s)
		}
		out[s.Name] = c
	}
	res.args, res.lifted, res.warnings = out, b.lifted, b.warnings
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
	for _, in := range op.Inputs {
		pos = append(pos, strings.ToLower(string(in)))
	}
	for _, a := range op.Args {
		if isSpaceOption(op, &a) {
			continue // the working-space option is on every pixel op; help and errors explain it
		}
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

// isSpaceOption reports whether a is the space option Register added.
func isSpaceOption(op *OpDef, a *ArgSpec) bool {
	return a.Name == SpaceArg && op.Space.Default != buffer.SpaceNone && !op.Space.Fixed
}

// resolveArgs substitutes lifted inputs with their evaluated data and
// re-checks types and hard limits.
func resolveArgs(n *Node, ins []Value) (Args, error) {
	sub := func(v any) (any, error) { return substitute(v, ins) }
	out := make(map[string]any, len(n.Args.vals))
	for k, v := range n.Args.vals {
		r, err := sub(v)
		if err != nil {
			return Args{}, err
		}
		if s := n.Op.spec(k); s != nil && containsRef(v) {
			c, err := (*binder)(nil).check(s, s.Type, normalize(r), n.Op.Name+"."+k)
			if err != nil {
				return Args{}, err
			}
			r = c
		}
		out[k] = r
	}
	return Args{op: n.Op, vals: out, meta: n.Args.meta}, nil
}

func containsRef(v any) bool {
	switch x := v.(type) {
	case InputRef:
		return true
	case []any:
		for _, e := range x {
			if containsRef(e) {
				return true
			}
		}
	case map[string]any:
		for _, e := range x {
			if containsRef(e) {
				return true
			}
		}
	}
	return false
}

func substitute(v any, ins []Value) (any, error) {
	switch x := v.(type) {
	case InputRef:
		if d, ok := ins[x.Index].(*DataValue); ok {
			return d.Data, nil
		}
		return ins[x.Index], nil // kind-typed argument: the evaluated node value
	case []any:
		if !containsRef(x) {
			return x, nil
		}
		out := make([]any, len(x))
		for i, e := range x {
			r, err := substitute(e, ins)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	case map[string]any:
		if !containsRef(x) {
			return x, nil
		}
		out := make(map[string]any, len(x))
		for k, e := range x {
			r, err := substitute(e, ins)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	}
	return v, nil
}

// Normalize converts exported JavaScript values (int64, float32, typed
// slices, nested maps) into the canonical value space used by args and data.
func Normalize(v any) any { return normalize(v) }
