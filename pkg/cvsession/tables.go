package cvsession

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/resultcontent"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"gopkg.in/yaml.v3"
)

// Compact tables (GCV-005 §6.5, §10.3, D9). An array of at least two flat
// records (plain objects whose own properties are data properties holding
// undefined, null, a boolean, a number or a string) renders as a literal
// block: a header line with the union of the keys, then one line per
// record. Tables are exempt from the item, string and property caps; all
// tables of one result share a budget of two thirds of the result cap, each
// shows at least minTableRows rows, and a note says which rows are shown
// and how to get the others.

const (
	minTableRows  = 5
	maxCellRunes  = 60
	widthRows     = 200 // rows that set the column widths (and the columns)
	tableColumnSp = "  "
)

// table renders obj as a table, or returns nil when it is not an array of
// at least two flat records.
func (r *renderer) table(obj *goja.Object, path string) *yaml.Node {
	n := int(obj.Get("length").ToInteger())
	if n < 2 {
		return nil
	}
	rows := make([]*goja.Object, n)
	for i := range rows {
		o, ok := obj.Get(strconv.Itoa(i)).(*goja.Object)
		if !ok || o.ClassName() != "Object" || o.ExportType() != plainObjectType {
			return nil
		}
		if _, ok := resultcontent.ProviderOf(o); ok {
			return nil
		}
		rows[i] = o
	}
	// columns and widths from the first rows; every one of them must be flat
	var cols []string
	colIdx := map[string]int{}
	head := min(n, widthRows)
	cells := make([]map[string]string, head)
	for i := 0; i < head; i++ {
		c, keys, ok := r.cells(rows[i])
		if !ok {
			return nil
		}
		cells[i] = c
		for _, k := range keys {
			if _, seen := colIdx[k]; !seen {
				colIdx[k] = len(cols)
				cols = append(cols, k)
			}
		}
	}
	width := make([]int, len(cols))
	for j, k := range cols {
		width[j] = utf8.RuneCountInString(k)
		for _, c := range cells {
			width[j] = max(width[j], utf8.RuneCountInString(cellOf(c, k)))
		}
	}
	line := func(c map[string]string, header bool) string {
		var sb strings.Builder
		for j, k := range cols {
			v := k
			if !header {
				v = cellOf(c, k)
			}
			if j > 0 {
				sb.WriteString(tableColumnSp)
			}
			sb.WriteString(v)
			if pad := width[j] - utf8.RuneCountInString(v); pad > 0 {
				sb.WriteString(strings.Repeat(" ", pad))
			}
		}
		return strings.TrimRightFunc(sb.String(), unicode.IsSpace)
	}
	var sb strings.Builder
	h := line(nil, true)
	sb.WriteString(h)
	used := utf8.RuneCountInString(h) + 1
	shown := 0
	for i := 0; i < n; i++ {
		c := map[string]string(nil)
		if i < head {
			c = cells[i]
		} else {
			var ok bool
			if c, _, ok = r.cells(rows[i]); !ok {
				break // a row that is not flat ends the table; the note pages past it
			}
		}
		l := line(c, false)
		size := utf8.RuneCountInString(l) + 1
		if shown >= minTableRows && used+size > r.tableLeft {
			break
		}
		sb.WriteString("\n")
		sb.WriteString(l)
		used += size
		shown++
	}
	r.tableLeft -= used
	if shown < n {
		name := path
		if name == "" {
			name = "this array"
		}
		fmt.Fprintf(&sb, "\n… rows 1–%d of %d shown. Next: %s.slice(%d, %d); all: await cv.save(\"rows.csv\", %s)", shown, n, name, shown, 2*shown, name)
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.LiteralStyle, Tag: "!!str", Value: sb.String()}
}

func cellOf(c map[string]string, k string) string {
	if v, ok := c[k]; ok {
		return v
	}
	return "-"
}

// cells reads a record's own data properties as table cells (in key
// order); ok is false when it has an accessor or a value that is not a
// primitive.
func (r *renderer) cells(o *goja.Object) (map[string]string, []string, bool) {
	keys := o.Keys()
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		v, ok := r.dataProperty(o, k)
		if !ok {
			return nil, nil, false
		}
		s, ok := cellText(v)
		if !ok {
			return nil, nil, false
		}
		out[k] = s
	}
	return out, keys, true
}

// cellText formats one table cell: "-" for undefined and null, numbers as
// the renderer does, strings as they are unless empty, "-" or containing
// whitespace (then JSON-quoted), cut at maxCellRunes.
func cellText(v goja.Value) (string, bool) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return "-", true
	}
	if _, ok := v.(*goja.Object); ok {
		return "", false
	}
	switch x := v.Export().(type) {
	case bool:
		return strconv.FormatBool(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case float64:
		return FormatNumber(x), true
	case string:
		s := cvmod.CutRunes(x, maxCellRunes)
		if s == "" || s == "-" || strings.ContainsFunc(s, unicode.IsSpace) {
			s = strconv.Quote(s)
		}
		return s, true
	}
	return "", false
}

// function renders a function as its signature and doc line, named by its
// key path (its last key), by the mem key that holds it, or by its own name.
func (r *renderer) function(fn *goja.Object, path string) string {
	d, ok := cvmod.DescribeFunction(cvmod.FunctionSource(r.vm, fn))
	name := path
	if i := strings.LastIndexAny(name, ".]"); i >= 0 {
		name = strings.Trim(name[i:], `.[]"`)
	}
	if name == "" {
		if mem, isObj := r.vm.GlobalObject().Get("mem").(*goja.Object); isObj {
			for _, k := range mem.Keys() {
				if v, data := r.dataProperty(mem, k); data && v != nil && v.SameAs(fn) {
					name = "mem." + k
					break
				}
			}
		}
	}
	if name == "" {
		name = fn.Get("name").String()
	}
	if name == "" {
		name = "function"
	}
	if !ok {
		return "[Getter]"
	}
	if d.Native {
		return name + "()"
	}
	if d.Doc == "" {
		return d.Signature(name)
	}
	return d.Signature(name) + ": " + d.Doc
}
