package graph

import (
	"fmt"
	"strings"
)

// Help renders the registry as plain data. With an empty name it returns an
// overview (kinds and method names per kind); otherwise the entry for that op
// in the shape of guide Part 2 §11. Unknown names are E_UNKNOWN_METHOD with a
// suggestion.
func (r *Registry) Help(name string) (map[string]any, error) {
	if name == "" {
		perKind := map[string]any{}
		var kinds []any
		for _, k := range r.Receivers() {
			kinds = append(kinds, string(k))
			ms := r.Methods(k)
			if len(ms) == 0 {
				continue
			}
			list := make([]any, len(ms))
			for i, m := range ms {
				list[i] = m
			}
			perKind[string(k)] = list
		}
		var sources []any
		for _, op := range r.Ops() {
			if op.On == KindNone {
				sources = append(sources, op.Name)
			}
		}
		return map[string]any{
			"kinds":   kinds,
			"ops":     perKind,
			"sources": sources,
			"usage":   "Calls build lazy nodes; await a node to compute it (data at full resolution), node.look() to see it. cv.help(\"name\") describes one op.",
		}, nil
	}
	ops := r.Find(name)
	if len(ops) == 0 {
		var all []string
		seen := map[string]bool{}
		for _, op := range r.Ops() {
			if !seen[op.Name] {
				all = append(all, op.Name)
				seen[op.Name] = true
			}
		}
		e := Errorf(EUnknownMethod, "There is no op named %q.", name)
		if t, ok := Synonyms[name]; ok && seen[t] {
			e.Hint, e.Fix = fmt.Sprintf("Did you mean %q?", t), fmt.Sprintf("cv.help(%q)", t)
		} else if sug := closest(name, all); sug != "" {
			e.Hint, e.Fix = fmt.Sprintf("Did you mean %q?", sug), fmt.Sprintf("cv.help(%q)", sug)
		}
		return nil, e
	}
	op := ops[0]
	var on any = string(op.On)
	if len(ops) > 1 {
		var kinds []any
		for _, o := range ops {
			kinds = append(kinds, string(o.On))
		}
		on = kinds
	}
	if op.On == KindNone {
		on = "cv"
	}
	entry := map[string]any{
		"on":      on,
		"returns": string(op.Returns),
		"usage":   strings.TrimPrefix(signatureHint(op, op.Name), "Usage: "),
		"doc":     op.Doc,
	}
	switch op.Space {
	case Physical:
		entry["space"] = "physical: computed in linear light, returned in the image's space"
	case Display:
		entry["space"] = "display: a picture of encoded values"
	case Follows:
		if op.On == KindImage && op.SpaceOf == nil {
			entry["space"] = "the image's: numbers and results are in the space of the image it is called on (img.as(\"linear\") for linear light)"
		}
	}
	if args := describeArgs(op.Args); len(args) > 0 {
		entry["args"] = args
	}
	if fs := op.DocFields(); len(fs) > 0 {
		entry["fields"] = toAnyList(FieldNames(fs))
	}
	if len(op.See) > 0 {
		entry["see"] = toAnyList(op.See)
	}
	return map[string]any{op.Name: entry}, nil
}

// FuncHelp renders function name as help data: { name: { on, usage, doc,
// args, see } }; ok is false when no such function is registered.
func (r *Registry) FuncHelp(name string) (map[string]any, bool) {
	f, ok := r.Func(name)
	if !ok {
		return nil, false
	}
	on := "cv"
	if len(f.On) > 0 {
		on = strings.Join(f.On, ", ")
	}
	entry := map[string]any{"on": on, "usage": f.UsageLine(), "doc": f.Doc}
	if args := describeArgs(f.Args); len(args) > 0 {
		entry["args"] = args
	}
	if len(f.See) > 0 {
		entry["see"] = toAnyList(f.See)
	}
	return map[string]any{f.Name: entry}, true
}

func toAnyList(xs []string) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func describeArgs(specs []ArgSpec) map[string]any {
	out := map[string]any{}
	for i := range specs {
		s := &specs[i]
		out[s.Name] = DescribeArgSpec(s)
	}
	return out
}

// DescribeArgSpec renders one argument for help: type, unit, limits, default.
func DescribeArgSpec(s *ArgSpec) string {
	var parts []string
	switch {
	case strings.HasPrefix(s.Type, "enum:"):
		parts = append(parts, strings.Join(quoteSplit(strings.TrimPrefix(s.Type, "enum:")), " | "))
	case s.Type == "number" && s.Unit == DocPx:
		parts = append(parts, "length (doc px)")
	case s.Type == "number" && s.Unit == Fraction:
		parts = append(parts, "fraction of width/height")
	case s.Type == "number" && s.Unit == DocPx2:
		parts = append(parts, "area (doc px²)")
	case s.Type == "box" || s.Type == "point" || isUnionType(s.Type):
		var alts []string
		for _, t := range strings.Split(s.Type, "|") {
			switch {
			case t == "box":
				alts = append(alts, "box [x, y, w, h] or { x, y, w, h }")
			case t == "point":
				alts = append(alts, "point [x, y] or { cx, cy }")
			case strings.HasPrefix(t, "enum:"):
				alts = append(alts, strings.Join(quoteSplit(strings.TrimPrefix(t, "enum:")), " | "))
			default:
				alts = append(alts, t)
			}
		}
		d := strings.Join(alts, " | ")
		if s.Unit == DocPx {
			d += " (doc px)"
		}
		parts = append(parts, d)
	case isKindType(s.Type):
		parts = append(parts, s.Type+" node")
	case s.Type == "object":
		var keys []string
		for _, o := range s.Options {
			keys = append(keys, o.Name)
		}
		parts = append(parts, "{ "+strings.Join(keys, ", ")+" }")
	default:
		parts = append(parts, s.Type)
	}
	if s.Min != nil || s.Max != nil {
		parts[0] += " in " + fmtLimits(s)
	}
	if s.hasUsual() {
		parts = append(parts, fmt.Sprintf("usual [%s, %s]", fmtNum(s.Usual[0]), fmtNum(s.Usual[1])))
	}
	switch {
	case s.Default != nil:
		parts = append(parts, "default "+fmtDefault(s.Default))
	case s.Optional || s.Variadic:
		parts = append(parts, "optional")
	default:
		parts = append(parts, "required")
	}
	if s.Positional {
		parts = append(parts, "positional")
	}
	out := strings.Join(parts, ", ")
	if s.Doc != "" {
		out += ". " + s.Doc
	}
	if len(s.ValueDocs) > 0 && strings.HasPrefix(s.Type, "enum:") {
		var docs []string
		for _, v := range strings.Split(strings.TrimPrefix(s.Type, "enum:"), ",") {
			docs = append(docs, v+": "+s.ValueDocs[v])
		}
		out += ". " + strings.Join(docs, "; ")
	}
	return out
}

func quoteSplit(s string) []string {
	xs := strings.Split(s, ",")
	for i, x := range xs {
		xs[i] = `"` + x + `"`
	}
	return xs
}

func fmtDefault(v any) string {
	switch x := normalize(v).(type) {
	case float64:
		return fmtNum(x)
	case string:
		return `"` + x + `"`
	case []any:
		if len(x) == 0 {
			return "[]"
		}
	}
	return fmt.Sprintf("%v", v)
}
