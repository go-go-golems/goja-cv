// Package dataout is how files leave an eval (GCV-004 §6.7, §7.8, §7.9):
// cv.save(path, x) writes an image, JSON or CSV file in the workspace, the
// format chosen by the extension.
package dataout

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// WClamp is the warning of an integer format clamping values outside [0, 1].
const WClamp = "CLAMP"

// Formats of a save that are not images.
const (
	JSON = "json"
	CSV  = "csv"
)

// Saved is the result of a save: { path, format, bytes, rows, depth,
// encoding, space, frame, replaced, note }.
type Saved struct {
	Path     string
	Format   string
	Depth    int
	Encoding string
	Space    string
	Frame    *graph.Frame
	Bytes    int
	Rows     int // data only
	Replaced bool
	Note     string
}

// Data is the plain-data form JavaScript receives.
func (s Saved) Data() map[string]any {
	m := map[string]any{"path": s.Path, "format": s.Format, "bytes": float64(s.Bytes), "replaced": s.Replaced}
	if s.Depth != 0 {
		m["depth"] = float64(s.Depth)
	}
	if s.Encoding != "" {
		m["encoding"] = s.Encoding
	}
	if s.Space != "" {
		m["space"] = s.Space
	}
	if s.Frame != nil {
		m["frame"] = s.Frame.Data()
	}
	if s.Format == CSV || s.Format == JSON {
		m["rows"] = float64(s.Rows)
	}
	if s.Note != "" {
		m["note"] = s.Note
	}
	return m
}

// extensions lists what cv.save writes, for errors.
const extensions = ".tif, .png, .npy, .jpg (images), .json (any data), .csv (a list of rows)"

func withExt(path, ext string) string { return strings.TrimSuffix(path, filepath.Ext(path)) + ext }

// FormatOf is the format of a save path: an image format, or JSON or CSV.
// An unknown extension is E_ARG_TYPE with the list.
func FormatOf(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return JSON, nil
	case ".csv":
		return CSV, nil
	}
	if f, ok := buffer.FormatOf(path); ok {
		return string(f), nil
	}
	return "", graph.Errorf(graph.EArgType, "cv.save(%q): the extension picks the format: %s.", path, extensions).
		WithValid([]string{".tif", ".png", ".npy", ".jpg", ".json", ".csv"}).
		WithFix(fmt.Sprintf("cv.save(%q, x)", withExt(path, ".json")))
}

// CheckKind refuses a format that does not fit what is saved (§6.7): an
// Image node is written as an image file; a Value or Regions node and plain
// data as JSON or CSV.
func CheckKind(path, format string, image bool) error {
	data := format == JSON || format == CSV
	switch {
	case image && data:
		return graph.Errorf(graph.EArgType, "cv.save(%q): an Image is saved as an image file, not %s.", path, strings.ToUpper(format)).
			WithHint("Data (measurements, regions, plain values) is JSON or CSV; images are .tif, .png, .npy or .jpg.").
			WithFix(fmt.Sprintf("cv.save(%q, x)", withExt(path, ".tif")))
	case !image && !data:
		return graph.Errorf(graph.EArgType, "cv.save(%q): this is data, not an image: save it as JSON (or CSV for a list of rows).", path).
			WithFix(fmt.Sprintf("cv.save(%q, x)", withExt(path, ".json")))
	}
	return nil
}

// imageDepth checks the depth of an image save.
func imageDepth(path string, f buffer.Format, depth int) (int, error) {
	ds := buffer.Depths[f]
	if depth == 0 {
		return ds[0], nil
	}
	for _, d := range ds {
		if d == depth {
			return depth, nil
		}
	}
	valid := make([]string, len(ds))
	for i, d := range ds {
		valid[i] = fmt.Sprint(d)
	}
	return 0, graph.Errorf(graph.EArgRange, "cv.save(%q): %s takes depth %s, not %d.", path, f, strings.Join(valid, ", "), depth).
		WithValid(valid).WithFix(fmt.Sprintf("cv.save(%q, x, { depth: %d })", path, ds[0]))
}

// CheckImage checks what can be checked before evaluating an image save:
// the depth and JPEG's sRGB-only rule.
func CheckImage(eng *graph.Engine, id graph.NodeID, path string, depth int) error {
	n, ok := eng.Node(id)
	if !ok {
		return graph.Errorf(graph.EInternal, "unknown node #%d", id)
	}
	f, _ := buffer.FormatOf(path)
	if _, err := imageDepth(path, f, depth); err != nil {
		return err
	}
	if f == buffer.FormatJPEG && n.Enc != buffer.EncSRGB {
		return graph.Errorf(graph.EArgType, "cv.save(%q): JPEG holds sRGB pictures; this image's values are %s.", path, encOf(n)).
			WithHint("PNG and TIFF record the encoding so the file reopens with the same numbers.").
			WithFix(fmt.Sprintf("cv.save(%q, x)", withExt(path, ".png")))
	}
	return nil
}

func encOf(n *graph.Node) buffer.Encoding {
	if n.Enc == buffer.EncNone {
		return buffer.EncLinear
	}
	return n.Enc
}

// EncodeImage evaluates Image node id at scale 1 and encodes it as path's
// format: encoded values with the encoding (linear-encoded images:
// linear) and frame origin recorded where the format holds them. warn
// receives CLAMP.
func EncodeImage(ctx context.Context, eng *graph.Engine, id graph.NodeID, path string, depth int, warn func(graph.Warning)) ([]byte, Saved, error) {
	n, _ := eng.Node(id)
	f, _ := buffer.FormatOf(path)
	depth, err := imageDepth(path, f, depth)
	if err != nil {
		return nil, Saved{}, err
	}
	enc := encOf(n)
	rootID, err := eng.Display(id)
	if err != nil {
		return nil, Saved{}, err
	}
	v, err := eng.Evaluate(ctx, rootID, graph.EvalOptions{Scale: 1, OnWarn: warn})
	if err != nil {
		return nil, Saved{}, err
	}
	iv, ok := v.(*graph.ImageValue)
	if !ok {
		return nil, Saved{}, graph.Errorf(graph.EInternal, "save: %s evaluated to %T", graph.Label(n), v)
	}
	frame := n.Frame
	meta := buffer.Meta{Encoding: enc}
	if frame.X != 0 || frame.Y != 0 {
		meta.Origin = &[2]int{frame.X, frame.Y}
	}
	if depth != 32 && warn != nil {
		if c := buffer.ClippedFraction(iv.Buf); c > 0 {
			fix := fmt.Sprintf("cv.save(%q, x, { depth: 32 })", withExt(path, ".tif"))
			warn(graph.Warning{Code: WClamp, Message: fmt.Sprintf("cv.save(%q): %.3g %% of the samples lie outside [0, 1] and were clamped by the %d-bit format. fix: %s   // or .npy",
				path, 100*c, depth, fix)})
		}
	}
	var out bytes.Buffer
	if err := buffer.Encode(ctx, &out, iv.Buf, f, depth, meta); err != nil {
		return nil, Saved{}, graph.Errorf(graph.EEval, "cv.save(%q): %v", path, err)
	}
	space := buffer.SpaceEncoded // a file holds encoded values
	if enc == buffer.EncLinear {
		space = buffer.SpaceLinear
	}
	s := Saved{Format: string(f), Depth: depth, Encoding: enc.String(), Space: space.String(), Frame: &frame, Bytes: out.Len()}
	switch f {
	case buffer.FormatNPY:
		s.Note = "NPY holds values only: the file reopens as linear with origin (0, 0); pass cv.open(path, { encoding, origin }) to restore them."
	case buffer.FormatJPEG:
		s.Note = "JPEG holds values only (lossy, 8-bit): the frame origin is not recorded."
	case buffer.FormatTIFF, buffer.FormatPNG:
	}
	return out.Bytes(), s, nil
}

// NodeData evaluates a Value or Regions node at scale 1 as plain data:
// region items (records in schema order) or the value's data in schema
// order.
func NodeData(ctx context.Context, eng *graph.Engine, id graph.NodeID) (any, error) {
	n, ok := eng.Node(id)
	if !ok {
		return nil, graph.Errorf(graph.EInternal, "unknown node #%d", id)
	}
	val, err := eng.Evaluate(ctx, id, graph.EvalOptions{Scale: 1})
	if err != nil {
		return nil, err
	}
	switch x := val.(type) {
	case *graph.RegionsValue:
		fields := x.Schema()
		items := make([]any, len(x.Items))
		for i, r := range x.Items {
			items[i] = r.Data(fields)
		}
		return items, nil
	case *graph.DataValue:
		return graph.OrderBySchema(x.Data, n.Fields), nil
	}
	return nil, graph.Errorf(graph.EWrongKind, "cv.save: %s is not data.", graph.Label(n))
}

// EncodeData writes data as JSON or CSV; rows is the number of CSV records
// or JSON array items (1 for anything else).
func EncodeData(path, format string, v any) ([]byte, int, error) {
	if format == CSV {
		return encodeCSV(path, v)
	}
	return encodeJSON(v)
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

// rowOf is a CSV row: a record (its key order) or a map (sorted keys).
func rowOf(o any) ([]string, map[string]any, bool) {
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

// notRows is the error of CSV of something that is not a list of rows, with
// the fix: JSON, or the rows of a value that has them (profile, histogram).
func notRows(path string, v any) error {
	what := "a " + fmt.Sprintf("%T", v)
	_, m, isObj := rowOf(v)
	switch {
	case isObj:
		what = "an object"
	case v == nil:
		what = "null"
	}
	e := graph.Errorf(graph.EArgType, "cv.save(%q): CSV is a list of rows (objects); this is %s.", path, what).
		WithFix(fmt.Sprintf("cv.save(%q, x)", withExt(path, ".json")))
	if isObj {
		if _, ok := m["rows"].([]any); ok {
			e.Hint = "This value keeps its table in rows."
			e.Fix = fmt.Sprintf("cv.save(%q, (await x).rows)", path)
		}
	}
	return e
}

// encodeCSV writes a list of rows as CSV with a header: columns are the
// rows' keys in first-seen order (records keep schema order); a nested
// value is JSON in its cell. Anything else is E_ARG_TYPE.
func encodeCSV(path string, v any) ([]byte, int, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, 0, notRows(path, v)
	}
	var cols []string
	seen := map[string]bool{}
	rows := make([]map[string]any, 0, len(list))
	for i, e := range list {
		ks, m, ok := rowOf(e)
		if !ok {
			return nil, 0, graph.Errorf(graph.EArgType, "cv.save(%q): CSV is a list of rows (objects); row %d is %s.", path, i, describe(e)).
				WithFix(fmt.Sprintf("cv.save(%q, x)", withExt(path, ".json")))
		}
		for _, k := range ks {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
		rows = append(rows, m)
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

func describe(v any) string {
	switch v.(type) {
	case []any:
		return "a list"
	case string:
		return "a string"
	case float64:
		return "a number"
	case nil:
		return "null"
	}
	return fmt.Sprintf("%T", v)
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
