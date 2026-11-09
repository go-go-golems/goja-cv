package dataout

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/go-go-golems/go-go-goja/pkg/resultcontent"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// A Data handle is what cv.json(x) and cv.csv(x) return: a Go object that
// becomes result content (text for MCP clients, a named file for the CLI)
// or is saved under the out-root. x is a Value or Regions node (lazy: it is
// evaluated at scale 1 when the content is rendered, off the owner
// goroutine) or plain data (eager: converted on the owner goroutine at the
// call, so edits the agent made to awaited data are kept).

// Data formats.
const (
	JSON = "json"
	CSV  = "csv"
)

// NamePattern is what a data name may be: an identifier, never a path.
var NamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

var nonName = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// Data is a JSON or CSV data item.
type Data struct {
	format  string
	name    string
	columns []string

	eng    *graph.Engine
	node   graph.NodeID // lazy (0: eager)
	fields []graph.FieldSpec
	label  string
	data   any // eager

	mu       sync.Mutex
	number   int // content number (data-<N> when unnamed)
	maxBytes int // 0: unlimited

	once sync.Once
	out  []byte
	rows int
	err  error
}

// Options are the options of cv.json and cv.csv.
type Options struct {
	Name    string
	Columns []string // CSV: select and order columns
}

// New makes a Data handle of a node (node ≠ 0, evaluated lazily) or of
// plain data. Image and Mask nodes are E_WRONG_KIND.
func New(eng *graph.Engine, format string, node graph.NodeID, data any, o Options) (*Data, error) {
	call := "cv." + format
	if o.Name != "" && !NamePattern.MatchString(o.Name) {
		fixed := strings.Trim(nonName.ReplaceAllString(strings.TrimSuffix(filepath.Base(o.Name), filepath.Ext(o.Name)), "-"), "-")
		if fixed == "" {
			fixed = "data"
		}
		if len(fixed) > 64 {
			fixed = fixed[:64]
		}
		return nil, graph.Errorf(graph.EArgType, "%s: name %q is not a name: use letters, digits, - and _ (at most 64), no paths or extensions.", call, o.Name).
			WithHint("The name becomes the file name of the content (name." + format + "); to write a file elsewhere, use .save(path) under the out-root.").
			WithFix(fmt.Sprintf("%s(x, { name: %q })", call, fixed))
	}
	d := &Data{format: format, name: o.Name, columns: o.Columns, eng: eng, node: node, data: data}
	if node != 0 {
		n, ok := eng.Node(node)
		if !ok {
			return nil, graph.Errorf(graph.EInternal, "unknown node #%d", node)
		}
		if n.Kind == graph.KindImage || n.Kind == graph.KindMask {
			return nil, graph.Errorf(graph.EWrongKind, "%s writes data (Value and Regions nodes, plain data); %s is %s node.", call, graph.Label(n), graph.A(string(n.Kind))).
				WithHint("Images are saved as image files.").
				WithFix(`node.save("x.tif")`)
		}
		d.fields, d.label = n.Fields, graph.Label(n)
		if len(o.Columns) > 0 && len(n.Fields) > 0 {
			for _, c := range o.Columns {
				if _, ok := graph.FindField(n.Fields, c); !ok {
					names := graph.FieldNames(n.Fields)
					e := graph.Errorf(graph.EUnknownOption, "%s: column %q is not a field of %s; fields: %s.", call, c, d.label, strings.Join(names, ", ")).WithValid(names)
					if sug := graph.Closest(c, names); sug != "" {
						e.Hint = fmt.Sprintf("Did you mean %q?", sug)
					}
					return nil, e
				}
			}
		}
	}
	return d, nil
}

// Format is "json" or "csv".
func (d *Data) Format() string { return d.format }

// SetNumber gives an unnamed handle its content number (data-<N>) and the
// session's byte limit for content (0: unlimited). The session calls it on
// the owner goroutine before rendering.
func (d *Data) SetNumber(n, maxBytes int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.number, d.maxBytes = n, maxBytes
}

// Name is the item's name: its name option, or data-<N>.
func (d *Data) Name() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.name != "" {
		return d.name
	}
	if d.number > 0 {
		return fmt.Sprintf("data-%d", d.number)
	}
	return "data"
}

// FileName is name.ext.
func (d *Data) FileName() string { return d.Name() + resultcontent.ExtensionForMIME(d.mime()) }

func (d *Data) mime() string {
	if d.format == CSV {
		return "text/csv"
	}
	return "application/json"
}

// ResultPlaceholder states what is known before rendering: the file name,
// and the columns (CSV of a node with a schema) or items (eager data).
func (d *Data) ResultPlaceholder() string {
	s := "data " + d.FileName()
	switch {
	case d.node != 0 && d.format == CSV && len(d.fields) > 0 && !hasArrays(d.fields):
		n := len(d.fields)
		if len(d.columns) > 0 {
			n = len(d.columns)
		}
		return fmt.Sprintf("%s (%d columns of %s)", s, n, d.label)
	case d.node != 0:
		return fmt.Sprintf("%s (%s)", s, d.label)
	}
	switch x := d.data.(type) {
	case []any:
		return fmt.Sprintf("%s (%d items)", s, len(x))
	case graph.Record:
		return fmt.Sprintf("%s (%d keys)", s, len(x.Keys))
	}
	return s
}

// Render evaluates a lazy node at scale 1 (once) and encodes the data; rows
// is the number of CSV records or JSON array items.
func (d *Data) Render(ctx context.Context) ([]byte, int, error) {
	d.once.Do(func() {
		v := d.data
		if d.node != 0 {
			val, err := d.eng.Evaluate(ctx, d.node, graph.EvalOptions{Scale: 1})
			if err != nil {
				d.err = err
				return
			}
			switch x := val.(type) {
			case *graph.RegionsValue:
				fields := x.Schema()
				items := make([]any, len(x.Items))
				for i, r := range x.Items {
					items[i] = r.Data(fields)
				}
				v = items
			case *graph.DataValue:
				v = graph.OrderBySchema(x.Data, d.fields)
			default:
				d.err = graph.Errorf(graph.EWrongKind, "cv.%s: %s is not data.", d.format, d.label)
				return
			}
		}
		if d.format == CSV {
			d.out, d.rows, d.err = encodeCSV(v, d.columns, d.fields)
		} else {
			d.out, d.rows, d.err = encodeJSON(v)
		}
	})
	return d.out, d.rows, d.err
}

// ResultContent renders the item. Over the session's byte limit (the MCP
// server's) it is replaced by a note telling the agent to save it.
func (d *Data) ResultContent(ctx context.Context) (_ []resultcontent.Content, err error) { //nolint:nonamedreturns // recover needs named results
	defer graph.Recover(&err, "data")
	out, rows, err := d.Render(ctx)
	if err != nil {
		return nil, err
	}
	caption := "data " + d.FileName()
	d.mu.Lock()
	limit := d.maxBytes
	d.mu.Unlock()
	if limit > 0 && len(out) > limit {
		note := fmt.Sprintf("data %s: too large for the conversation (%d KB, %d rows; limit %d KB): save it with .save(%q) under the out-root, or return a summary.",
			d.FileName(), (len(out)+1023)/1024, rows, limit/1024, d.FileName())
		return []resultcontent.Content{{MIMEType: "text/plain", Data: []byte(note), Caption: caption + " (not delivered: too large)"}}, nil
	}
	return []resultcontent.Content{{MIMEType: d.mime(), Data: out, Caption: caption}}, nil
}

// Save renders the item and writes it under root. The path's extension
// must be the item's format (added when missing).
func (d *Data) Save(ctx context.Context, root, path string) (Saved, error) {
	ext := strings.ToLower(filepath.Ext(path))
	want := "." + d.format
	switch ext {
	case want:
	case "":
		path += want
	default:
		return Saved{}, graph.Errorf(graph.EArgType, "save(%q): this is %s data; its file ends in %s.", path, strings.ToUpper(d.format), want).
			WithFix(fmt.Sprintf("cv.%s(x).save(%q)", d.format, strings.TrimSuffix(path, filepath.Ext(path))+want))
	}
	abs, err := graph.OutPath(root, path)
	if err != nil {
		return Saved{}, err
	}
	out, rows, err := d.Render(ctx)
	if err != nil {
		return Saved{}, err
	}
	replaced, err := graph.WriteOut(root, abs, out)
	if err != nil {
		return Saved{}, err
	}
	return Saved{Path: abs, Format: d.format, Bytes: len(out), Rows: rows, Replaced: replaced}, nil
}

// encodeJSON writes v as JSON with encoding/json: records keep their key
// order (Record.MarshalJSON), maps are sorted, NaN and infinities are null
// (plain).
func encodeJSON(v any) ([]byte, int, error) {
	b, err := json.Marshal(plain(v))
	if err != nil {
		return nil, 0, graph.Errorf(graph.EArgType, "this data cannot be written as JSON: %v", err)
	}
	rows := 1
	if l, ok := v.([]any); ok {
		rows = len(l)
	}
	return append(b, '\n'), rows, nil
}

// plain returns a copy of v in which every NaN and ±Inf is nil (JSON null,
// an empty CSV cell) and every other number is a float64: the one place
// that rule lives.
func plain(v any) any {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
		return x
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = plain(e)
		}
		return out
	case graph.Record:
		vals := make(map[string]any, len(x.Vals))
		for k, e := range x.Vals {
			vals[k] = plain(e)
		}
		return graph.Record{Keys: x.Keys, Vals: vals}
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = plain(e)
		}
		return out
	case nil, string, bool:
		return x
	}
	if f, ok := graph.Normalize(v).(float64); ok { // other numeric types
		return plain(f)
	}
	return v
}

// columnArrays finds the per-row arrays of a column table. With a schema they
// are its number[] fields, whatever their length (a 2-sample profile is 2
// rows; its from/to points are not columns). Without one they are the
// longest arrays, when longer than a point.
func columnArrays(ks []string, m map[string]any, fields []graph.FieldSpec) ([]string, int) {
	var cols []string
	longest := 0
	if len(fields) > 0 {
		for _, f := range fields {
			if l, ok := m[f.Name].([]any); ok && f.Type == "number[]" {
				cols = append(cols, f.Name)
				longest = max(longest, len(l))
			}
		}
		return cols, longest
	}
	for _, k := range ks {
		if l, ok := m[k].([]any); ok {
			longest = max(longest, len(l))
		}
	}
	if longest <= 2 {
		return nil, 0
	}
	for _, k := range ks {
		if l, ok := m[k].([]any); ok && len(l) == longest {
			cols = append(cols, k)
		}
	}
	return cols, longest
}

// table turns data into CSV rows: a list of objects (Regions items, rows)
// is one row each; an object whose longest arrays have the same length (a
// profile) is one row per array element; awaited Regions ({ kind, items })
// are their items; anything else is one row.
func table(v any, fields []graph.FieldSpec) ([]string, []map[string]any) {
	keysOf := func(o any) ([]string, map[string]any, bool) {
		switch x := o.(type) {
		case graph.Record:
			return x.Keys, x.Vals, true
		case map[string]any:
			ks := make([]string, 0, len(x))
			for k := range x {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			return ks, x, true
		}
		return nil, nil, false
	}
	var cols []string
	seen := map[string]bool{}
	add := func(ks []string) {
		for _, k := range ks {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	if len(fields) > 0 {
		add(graph.FieldNames(fields))
	}
	if ks, m, ok := keysOf(v); ok {
		if items, ok := m["items"].([]any); ok && m["kind"] == string(graph.KindRegions) {
			return table(items, fields)
		}
		arrayCols, longest := columnArrays(ks, m, fields)
		if len(arrayCols) > 0 { // a column table such as a profile
			rows := make([]map[string]any, longest)
			for i := range rows {
				rows[i] = map[string]any{}
				for _, c := range arrayCols {
					if l := m[c].([]any); i < len(l) {
						rows[i][c] = l[i]
					}
				}
			}
			return arrayCols, rows
		}
		if len(fields) == 0 {
			add(ks)
		}
		return cols, []map[string]any{m}
	}
	list, ok := v.([]any)
	if !ok {
		return []string{"value"}, []map[string]any{{"value": v}}
	}
	rows := make([]map[string]any, 0, len(list))
	for _, e := range list {
		if ks, m, ok := keysOf(e); ok {
			add(ks)
			rows = append(rows, m)
			continue
		}
		if l, ok := e.([]any); ok { // a list of lists: columns c1, c2, …
			m := map[string]any{}
			for j, x := range l {
				k := fmt.Sprintf("c%d", j+1)
				add([]string{k})
				m[k] = x
			}
			rows = append(rows, m)
			continue
		}
		add([]string{"value"})
		rows = append(rows, map[string]any{"value": e})
	}
	return cols, rows
}

func cell(v any) (string, error) {
	v = plain(v)
	switch x := v.(type) {
	case nil:
		return "", nil
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// encodeCSV writes v as CSV with a header; columns selects and orders.
func encodeCSV(v any, columns []string, fields []graph.FieldSpec) ([]byte, int, error) {
	cols, rows := table(v, fields)
	if len(columns) > 0 {
		have := map[string]bool{}
		for _, c := range cols {
			have[c] = true
		}
		for _, c := range columns {
			if !have[c] {
				e := graph.Errorf(graph.EUnknownOption, "cv.csv: column %q is not in the data; columns: %s.", c, strings.Join(cols, ", ")).WithValid(cols)
				if sug := graph.Closest(c, cols); sug != "" {
					e.Hint = fmt.Sprintf("Did you mean %q?", sug)
				}
				return nil, 0, e
			}
		}
		cols = columns
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write(cols); err != nil {
		return nil, 0, err
	}
	rec := make([]string, len(cols))
	for _, r := range rows {
		for i, c := range cols {
			s, err := cell(r[c])
			if err != nil {
				return nil, 0, err
			}
			rec[i] = s
		}
		if err := w.Write(rec); err != nil {
			return nil, 0, err
		}
	}
	w.Flush()
	return b.Bytes(), len(rows), w.Error()
}

// hasArrays reports whether a schema describes an object of arrays (a
// profile), whose CSV columns are its sample arrays.
func hasArrays(fs []graph.FieldSpec) bool {
	for _, f := range fs {
		if strings.HasSuffix(f.Type, "[]") {
			return true
		}
	}
	return false
}
