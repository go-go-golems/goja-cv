package graph

import (
	"fmt"
	"sort"
	"strings"
)

// A schema is the list of fields a result has: the items of a Regions
// value, the rows or the object of a Value. One schema per node (Node.Fields,
// computed at intern time from OpDef.Fields) serves the items JavaScript
// receives, where/sortBy validation, help, CSV columns and the prelude's
// derived-field rule (GCV-003 §7.5).
//
// The names follow one rule: x, y, w, h is a box (corner and size); cx, cy
// a point or a centre; meanL the mean of the gray plane; radii are options,
// never fields.

// FieldSpec describes one field of a result.
type FieldSpec struct {
	Name string
	Type string // "number" | "string" | "bool" | "number[]" | "object"
	Unit Unit   // DocPx (x, y, w, h, cx, cy), DocPx2 (area), Level (meanL, step, …), Unitless
	Doc  string
	// Optional fields are present only in some items or rows (check.dust's
	// hint, a sample row's outside flag).
	Optional bool
}

// GeometryFields are on every region, in this order.
var GeometryFields = []FieldSpec{
	{Name: "id", Type: "number", Doc: "stable id within this region set"},
	{Name: "x", Type: "number", Unit: DocPx, Doc: "left of the bounding box"},
	{Name: "y", Type: "number", Unit: DocPx, Doc: "top of the bounding box"},
	{Name: "w", Type: "number", Unit: DocPx, Doc: "width of the bounding box"},
	{Name: "h", Type: "number", Unit: DocPx, Doc: "height of the bounding box"},
	{Name: "cx", Type: "number", Unit: DocPx, Doc: "centroid x"},
	{Name: "cy", Type: "number", Unit: DocPx, Doc: "centroid y"},
	{Name: "area", Type: "number", Unit: DocPx2, Doc: "area"},
	{Name: "aspect", Type: "number", Doc: "ratio of the principal axes (≥ 1)"},
}

// RemovedFields maps field names agents may still use to the literal fix.
// Awaited region items throw E_NO_FIELD with the fix when one is read;
// where/sortBy refuse them the same way.
var RemovedFields = map[string]string{
	"box":      "[r.x, r.y, r.w, r.h], or pass the item itself where a box is expected",
	"centroid": "[r.cx, r.cy], or pass the item itself where a point is expected",
	"meanMask": `regions.measure(mask, { stats: ["mean"] }).meanL`,
}

// FieldNames lists the names of fs in order.
func FieldNames(fs []FieldSpec) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}

// FindField returns the field name of fs.
func FindField(fs []FieldSpec, name string) (FieldSpec, bool) {
	for _, f := range fs {
		if f.Name == name {
			return f, true
		}
	}
	return FieldSpec{}, false
}

// WithFields returns fs followed by more (a new slice).
func WithFields(fs []FieldSpec, more ...FieldSpec) []FieldSpec {
	return append(append(make([]FieldSpec, 0, len(fs)+len(more)), fs...), more...)
}

// RemovedFieldError is the E_NO_FIELD error for a removed field name.
func RemovedFieldError(name string) *Error {
	fix, ok := RemovedFields[name]
	if !ok {
		return nil
	}
	return Errorf(ENoField, "Region items have no field %q: geometry is flat (id, x, y, w, h, cx, cy, area, aspect).", name).
		WithHint("x, y, w, h is the bounding box (corner and size), cx, cy the centroid.").
		WithFix(fix)
}

// UnknownFieldError is the build-time error of a filter or sort on a field
// the receiver does not have (E_UNKNOWN_OPTION, or E_NO_FIELD for a removed
// name). call is how the agent wrote it ("where", "sortBy"); fix renders a
// literal fix for a suggested field. The builder adds which op produces the
// field (OpDef.Produces).
func UnknownFieldError(call, name string, fields []FieldSpec, fix func(sug string) string) *Error {
	if e := RemovedFieldError(name); e != nil {
		e.field = name
		return e
	}
	names := FieldNames(fields)
	e := Errorf(EUnknownOption, "%s: these regions have no field %q; they have %s.", call, name, strings.Join(names, ", ")).WithValid(names)
	if sug := closest(name, names); sug != "" {
		e.Hint = fmt.Sprintf("Did you mean %q?", sug)
		e.Fix = fix(sug)
	}
	e.field = name
	return e
}

// producerHint adds "score comes from detect.specks" (from the ops'
// Produces) to an UnknownFieldError.
func (r *Registry) producerHint(err error) error {
	ce, ok := AsError(err)
	if r == nil || !ok || ce.field == "" || ce.Code != EUnknownOption {
		return err
	}
	var calls []string
	for _, op := range r.Ops() {
		if c, ok := op.Produces[ce.field]; ok {
			calls = append(calls, c)
		}
	}
	if len(calls) == 0 {
		return err
	}
	sort.Strings(calls)
	h := fmt.Sprintf("%s comes from %s.", ce.field, strings.Join(calls, " or "))
	if ce.Hint != "" {
		h = ce.Hint + " " + h
	}
	ce.Hint = h
	if ce.Fix == "" {
		ce.Fix = calls[0]
	}
	return ce
}

// DefaultArgs are op's arguments with only the defaults filled (for help:
// the fields of a call that passes nothing).
func DefaultArgs(op *OpDef) Args {
	vals := map[string]any{}
	for _, s := range op.Args {
		if s.Default != nil {
			vals[s.Name] = normalize(s.Default)
		}
	}
	return Args{op: op, vals: vals}
}

// DocFields are the fields of op's result with default arguments and no
// known inputs (help, the prelude's field names).
func (op *OpDef) DocFields() []FieldSpec {
	if op.Fields == nil {
		return nil
	}
	return op.Fields(DefaultArgs(op), nil)
}

// nodeFields is the schema of a new node: a macro's is its expansion's,
// op.Fields when set, else a Regions-to-Regions op keeps its receiver's.
func nodeFields(op *OpDef, args Args, inputs []*Node) []FieldSpec {
	switch {
	case op.Expand != nil && len(inputs) > 0:
		return inputs[len(inputs)-1].Fields
	case op.Fields != nil:
		return op.Fields(args, inputs)
	case op.Returns == KindRegions && len(inputs) > 0 && inputs[0].Kind == KindRegions:
		return inputs[0].Fields
	}
	return nil
}

// Record is an object whose keys keep their order (schema order) when
// converted to JavaScript.
type Record struct {
	Keys []string
	Vals map[string]any
}

// Map returns the record's values as a plain map.
func (r Record) Map() map[string]any { return r.Vals }
