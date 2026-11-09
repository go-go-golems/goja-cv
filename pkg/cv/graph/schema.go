package graph

import (
	"bytes"
	"encoding/json"
)

// A schema is the list of fields a result has: the items of a Regions
// value, the rows or the object of a Value. One schema per node (Node.Fields,
// computed at intern time from OpDef.Fields) serves the items JavaScript
// receives (in schema order), help and CSV columns (GCV-003 §7.5).
//
// The names follow one rule: x, y, w, h is a box (corner and size); cx, cy
// a point or a centre; meanL the mean of the gray plane; radii are options,
// never fields.

// FieldSpec describes one field of a result.
type FieldSpec struct {
	Name string
	Type string // "number" | "string" | "bool" | "number[]" | "point" ([x, y]) | "object" | "rows" (a list of rows: Rows)
	Unit Unit   // DocPx (x, y, w, h, cx, cy), DocPx2 (area), Level (meanL, step, …), Unitless
	Doc  string
	// Optional fields are present only in some items or rows (check.dust's
	// hint, a sample row's outside flag).
	Optional bool
	// Rows is the schema of each row of a "rows" field (a profile's
	// samples, a histogram's bins).
	Rows []FieldSpec
}

// GeometryFields are on every region, in this order.
var GeometryFields = []FieldSpec{
	{Name: "id", Type: "number", Doc: "a number: the detector numbers its regions; data you may edit, and ids may repeat (an item is identified by its set and position)"},
	{Name: "x", Type: "number", Unit: DocPx, Doc: "left of the bounding box"},
	{Name: "y", Type: "number", Unit: DocPx, Doc: "top of the bounding box"},
	{Name: "w", Type: "number", Unit: DocPx, Doc: "width of the bounding box"},
	{Name: "h", Type: "number", Unit: DocPx, Doc: "height of the bounding box"},
	{Name: "cx", Type: "number", Unit: DocPx, Doc: "centroid x"},
	{Name: "cy", Type: "number", Unit: DocPx, Doc: "centroid y"},
	{Name: "area", Type: "number", Unit: DocPx2, Doc: "area"},
	{Name: "aspect", Type: "number", Doc: "ratio of the principal axes (≥ 1)"},
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
	return op.Fields(DefaultArgs(op))
}

// nodeFields is the schema of a new node: a macro's is its expansion's,
// else op.Fields.
func nodeFields(op *OpDef, args Args) []FieldSpec {
	switch {
	case macroOutput(op, args) != nil:
		return macroOutput(op, args).Fields
	case op.Fields != nil:
		return op.Fields(args)
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

// MarshalJSON writes the record's keys in order.
func (r Record) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range r.Keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		vb, err := json.Marshal(r.Vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// OrderBySchema gives the object of a Value, or each row of a list of rows,
// the key order of the node's schema (keys outside it follow, sorted), so
// awaited measurement rows read cx, cy, … like the docs.
func OrderBySchema(v any, fields []FieldSpec) any {
	if len(fields) == 0 {
		return v
	}
	order := func(m map[string]any) any {
		for _, f := range fields { // nested rows in their own schema order, on a copy (m is a cached value)
			if l, ok := m[f.Name].([]any); ok && f.Rows != nil {
				c := make(map[string]any, len(m))
				for k, v := range m {
					c[k] = v
				}
				c[f.Name] = OrderBySchema(l, f.Rows)
				m = c
			}
		}
		rec := Record{Vals: m}
		seen := map[string]bool{}
		for _, f := range fields {
			if _, ok := m[f.Name]; ok {
				rec.Keys = append(rec.Keys, f.Name)
				seen[f.Name] = true
			}
		}
		for _, k := range sortedKeys(m) {
			if !seen[k] {
				rec.Keys = append(rec.Keys, k)
			}
		}
		return rec
	}
	switch x := v.(type) {
	case map[string]any:
		return order(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			if m, ok := e.(map[string]any); ok {
				out[i] = order(m)
			} else {
				out[i] = e
			}
		}
		return out
	}
	return v
}
