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

// FuncArg is a JavaScript function passed as an argument (no op accepts
// lambdas in M1).
type FuncArg struct{}

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

// Number reads a unitless or fraction number. Reading a DocPx argument
// through Number panics: lengths must go through Length.
func (a Args) Number(name string) float64 {
	if s := a.mustSpec(name); s.Unit == DocPx {
		panic(fmt.Sprintf("cv: %s.%s is a doc-px length; read it with Args.Length", a.op.Name, name))
	}
	return a.number(name)
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

// Numbers reads a "number[]" argument.
func (a Args) Numbers(name string) []float64 {
	a.mustSpec(name)
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
	if k != KindValue && k != KindParam {
		return nil, Errorf(EWrongKind, "%s wants a %s, got an %s node.", where, t2noun(want), k).
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
	}
	return t
}

// check validates and canonicalizes v against type typ of spec s. With b
// non-nil, NodeArgs are lifted; with b nil (evaluation time, after
// substitution) they are an error.
func (b *binder) check(s *ArgSpec, typ string, v any, where string) (any, error) {
	if na, ok := v.(NodeArg); ok {
		if b == nil {
			return nil, Errorf(EInternal, "%s: unresolved node argument", where)
		}
		return b.lift(na.ID, where, typ)
	}
	if _, ok := v.(FuncArg); ok {
		return nil, Errorf(EArgType, "%s does not accept a function.", where).
			WithHint("No M1 op takes a lambda; pass a value or a node.")
	}
	if _, ok := v.(InputRef); ok && b == nil {
		return nil, Errorf(EInternal, "%s: unresolved input reference", where)
	}
	switch {
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
		if sug := closest(str, allowed); sug != "" {
			e.Hint = fmt.Sprintf("Did you mean %q?", sug)
		}
		return nil, e
	case strings.HasSuffix(typ, "[]"):
		list, ok := v.([]any)
		if !ok {
			return nil, typeErr(where, typ, v)
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
		if !s.Hidden {
			valid = append(valid, s.Name)
			known[s.Name] = true
		}
	}
	for _, k := range sortedKeys(m) {
		if !known[k] {
			e := Errorf(EUnknownOption, "%s has no option %q. Valid options: %s.", what, k, strings.Join(valid, ", ")).WithValid(valid)
			if sug := closest(k, valid); sug != "" {
				e.Hint = fmt.Sprintf("Did you mean %q?", sug)
				e.Fix = fix(sug)
			}
			return e
		}
	}
	return nil
}

func typeErr(where, typ string, v any) *Error {
	return Errorf(EArgType, "%s must be a %s, got %s.", where, t2noun(typ), describeRaw(v))
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
// object whose keys may name any non-hidden argument. trusted values (set by
// Go, e.g. hidden source stamps) are bound without the unknown-key check.
func bind(op *OpDef, hasReceiver bool, raw []any, trusted map[string]any, kindOf func(NodeID) (Kind, bool)) (*bound, error) {
	raw = normalize(trimUndefined(raw)).([]any)
	call := callName(op)
	res := &bound{}
	i := 0
	for j, decl := range op.Inputs {
		if i >= len(raw) {
			return nil, Errorf(EArgType, "%s needs a %s as argument %d.", call, decl, j+1)
		}
		na, ok := raw[i].(NodeArg)
		if !ok {
			return nil, Errorf(EWrongKind, "%s needs a %s node as argument %d, got %s.", call, decl, j+1, describeRaw(raw[i]))
		}
		k, _ := kindOf(na.ID)
		if !accepts(decl, k) {
			return nil, Errorf(EWrongKind, "%s needs a %s as argument %d, got a %s.", call, decl, j+1, k)
		}
		res.inputs = append(res.inputs, na.ID)
		i++
	}

	given := map[string]any{}
	var positional []*ArgSpec
	for k := range op.Args {
		if op.Args[k].Positional && !op.Args[k].Hidden {
			positional = append(positional, &op.Args[k])
		}
	}
	for _, s := range positional {
		if i >= len(raw) {
			break
		}
		if s.Variadic {
			var rest []any
			for i < len(raw) && !(isOptionsObject(raw[i]) && i == len(raw)-1 && !typeAcceptsObject(strings.TrimSuffix(s.Type, "[]"))) {
				rest = append(rest, raw[i])
				i++
			}
			given[s.Name] = rest
			continue
		}
		if isOptionsObject(raw[i]) && !typeAcceptsObject(s.Type) {
			break
		}
		given[s.Name] = raw[i]
		i++
	}
	if i < len(raw) {
		opts, ok := raw[i].(map[string]any)
		if !ok || i != len(raw)-1 {
			return nil, Errorf(EArgType, "%s takes at most %d positional argument(s); got %d.", call, len(op.Inputs)+len(positional), len(raw)).
				WithHint(signatureHint(op))
		}
		fix := func(sug string) string { return fmt.Sprintf("%s({ %s: %s })", call, sug, fmtFixValue(opts, sug)) }
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
	for k, v := range trusted {
		given[k] = normalize(v)
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
				return nil, Errorf(EArgType, "%s is missing its %s argument.", call, s.Name).WithHint(signatureHint(op))
			}
			continue
		}
		c, err := b.check(s, s.Type, v, call+"."+s.Name)
		if err != nil {
			return nil, err
		}
		out[s.Name] = c
	}
	res.args, res.lifted, res.warnings = out, b.lifted, b.warnings
	return res, nil
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

func callName(op *OpDef) string { return op.Name }

// signatureHint renders "exposure(ev)" style usage.
func signatureHint(op *OpDef) string {
	var pos, opt []string
	for _, in := range op.Inputs {
		pos = append(pos, strings.ToLower(string(in)))
	}
	for _, a := range op.Args {
		if a.Hidden {
			continue
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
	return "Usage: " + op.Name + "(" + strings.Join(pos, ", ") + ")"
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
	return Args{op: n.Op, vals: out}, nil
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
		d, ok := ins[x.Index].(*DataValue)
		if !ok {
			return nil, Errorf(EWrongKind, "lifted input %d is %s, not data", x.Index, ins[x.Index].Kind())
		}
		return d.Data, nil
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
