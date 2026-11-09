// Package dataout is how data leaves an eval for programs (GCV-003 §7.9,
// §7.10): images saved under the out-root with their encoding and frame,
// and Data handles (cv.json, cv.csv) delivered as result content or saved.
package dataout

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// WClamp is the warning of an integer format clamping values outside [0, 1].
const WClamp = "CLAMP"

// Saved is the result of a save: { path, format, depth, encoding, frame,
// bytes, rows, replaced, note }.
type Saved struct {
	Path     string
	Format   string
	Depth    int
	Encoding string
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
	if s.Frame != nil {
		m["frame"] = s.Frame.Data()
	}
	if s.Format == "csv" || s.Format == "json" {
		m["rows"] = float64(s.Rows)
	}
	if s.Note != "" {
		m["note"] = s.Note
	}
	return m
}

// imageFormat checks the extension and depth of an image save.
func imageFormat(path string, depth int) (buffer.Format, int, error) {
	f, ok := buffer.FormatOf(path)
	if !ok {
		base := strings.TrimSuffix(path, filepath.Ext(path))
		return "", 0, graph.Errorf(graph.EArgType, "save(%q): images are saved as .tif, .png, .npy or .jpg.", path).
			WithHint("TIFF and PNG record the encoding and frame; NPY keeps float values; JPEG is lossy.").
			WithFix(fmt.Sprintf("node.save(%q)", base+".tif"))
	}
	ds := buffer.Depths[f]
	if depth == 0 {
		return f, ds[0], nil
	}
	for _, d := range ds {
		if d == depth {
			return f, depth, nil
		}
	}
	valid := make([]string, len(ds))
	for i, d := range ds {
		valid[i] = fmt.Sprint(d)
	}
	return "", 0, graph.Errorf(graph.EArgRange, "save(%q): %s takes depth %s, not %d.", path, f, strings.Join(valid, ", "), depth).
		WithValid(valid).WithFix(fmt.Sprintf("node.save(%q, { depth: %d })", path, ds[0]))
}

// SaveImage evaluates Image or Mask node id at scale 1 and writes it under
// root as encoded values with its encoding (masks: linear) and frame origin
// recorded where the format holds them. warn receives CLAMP.
func SaveImage(ctx context.Context, eng *graph.Engine, id graph.NodeID, root, path string, depth int, warn func(graph.Warning)) (Saved, error) {
	n, ok := eng.Node(id)
	if !ok {
		return Saved{}, graph.Errorf(graph.EInternal, "unknown node #%d", id)
	}
	if n.Kind != graph.KindImage && n.Kind != graph.KindMask {
		return Saved{}, graph.Errorf(graph.EWrongKind, "save writes images and masks; %s is %s node.", graph.Label(n), graph.A(string(n.Kind))).
			WithHint("Data (Value and Regions nodes) is saved through a Data handle.").
			WithFix(fmt.Sprintf("cv.json(x).save(%q)", strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+".json"))
	}
	f, depth, err := imageFormat(path, depth)
	if err != nil {
		return Saved{}, err
	}
	enc := n.Enc
	if n.Kind == graph.KindMask || enc == buffer.EncNone {
		enc = buffer.EncLinear // a mask's value is its displayed value
	}
	if f == buffer.FormatJPEG && enc != buffer.EncSRGB {
		return Saved{}, graph.Errorf(graph.EArgType, "save(%q): JPEG holds sRGB pictures; this image's values are %s.", path, enc).
			WithHint("PNG and TIFF record the encoding so the file reopens with the same numbers.").
			WithFix(fmt.Sprintf("node.save(%q)", strings.TrimSuffix(path, filepath.Ext(path))+".png"))
	}
	abs, err := graph.OutPath(root, path)
	if err != nil {
		return Saved{}, err
	}
	rootID, err := eng.Display(id)
	if err != nil {
		return Saved{}, err
	}
	v, err := eng.Evaluate(ctx, rootID, graph.EvalOptions{Scale: 1, OnWarn: warn})
	if err != nil {
		return Saved{}, err
	}
	iv, ok := v.(*graph.ImageValue)
	if !ok {
		return Saved{}, graph.Errorf(graph.EInternal, "save: %s evaluated to %T", graph.Label(n), v)
	}
	frame := n.Frame
	meta := buffer.Meta{Encoding: enc}
	if frame.X != 0 || frame.Y != 0 {
		meta.Origin = &[2]int{frame.X, frame.Y}
	}
	if depth != 32 && warn != nil {
		if c := buffer.ClippedFraction(iv.Buf); c > 0 {
			fix := fmt.Sprintf("node.save(%q, { depth: 32 })", strings.TrimSuffix(path, filepath.Ext(path))+".tif")
			warn(graph.Warning{Code: WClamp, Message: fmt.Sprintf("save(%q): %.3g %% of the samples lie outside [0, 1] and were clamped by the %d-bit format. fix: %s   // or .npy",
				path, 100*c, depth, fix)})
		}
	}
	var out bytes.Buffer
	if err := buffer.Encode(ctx, &out, iv.Buf, f, depth, meta); err != nil {
		return Saved{}, graph.Errorf(graph.EEval, "save(%q): %v", path, err)
	}
	replaced, err := graph.WriteOut(root, abs, out.Bytes())
	if err != nil {
		return Saved{}, err
	}
	s := Saved{Path: abs, Format: string(f), Depth: depth, Encoding: enc.String(), Frame: &frame, Bytes: out.Len(), Replaced: replaced}
	switch f {
	case buffer.FormatNPY:
		s.Note = "NPY holds values only: the file reopens as linear with origin (0, 0); pass cv.open(path, { encoding, origin }) to restore them."
	case buffer.FormatJPEG:
		s.Note = "JPEG holds values only (lossy, 8-bit): the frame origin is not recorded."
	case buffer.FormatTIFF, buffer.FormatPNG:
	}
	return s, nil
}
