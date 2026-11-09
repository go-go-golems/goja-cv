package graph

import (
	"fmt"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// Value is an evaluated node result.
type Value interface {
	Kind() Kind
	Bytes() int64
}

// ImageValue is an evaluated Image node (a mask is a one-channel image).
// Frame is the node's static frame; the evaluator stamps it after Run,
// which only creates the value.
type ImageValue struct {
	Buf   *buffer.Buffer
	Frame Frame
	Enc   buffer.Encoding // the node's encoding
	// origin is the doc-px position of the buffer's top-left corner when it
	// is not the frame's origin (a crop evaluated below scale 1 is cut on
	// buffer pixels).
	origin *[2]float64
}

// BufferOrigin is the doc-px position of the top-left corner of buffer
// pixel (0, 0): the frame origin, or where a crop below scale 1 was cut.
func (v *ImageValue) BufferOrigin() (float64, float64) {
	if v.origin != nil {
		return v.origin[0], v.origin[1]
	}
	return float64(v.Frame.X), float64(v.Frame.Y)
}

// WithOrigin returns v with its buffer's top-left corner at doc px (x, y).
func (v *ImageValue) WithOrigin(x, y float64) *ImageValue {
	out := *v
	out.origin = &[2]float64{x, y}
	return &out
}

var _ Value = &ImageValue{}

// NewImage wraps a buffer as an Image value.
func NewImage(b *buffer.Buffer) *ImageValue {
	return &ImageValue{Buf: b}
}

// Coverage is the mean of a mask: the fraction of the image it selects.
func (v *ImageValue) Coverage() float64 {
	var s float64
	for _, p := range v.Buf.Plane(0) {
		s += float64(p)
	}
	return s / float64(max(1, v.Buf.N()))
}

func (v *ImageValue) Kind() Kind { return KindImage }

func (v *ImageValue) Bytes() int64 { return v.Buf.Bytes() + 64 }

// ImageSummary is what JavaScript receives for an awaited Image node: its
// statics only (the frame's size at scale 1, encoding, space, channels,
// frame, picture) plus the file metadata of an opened file. Nothing is
// evaluated, so awaiting a 53-megapixel scan is free (GCV-004 D18).
func ImageSummary(n *Node) map[string]any {
	m := map[string]any{"kind": string(n.Kind), "w": float64(n.Frame.W), "h": float64(n.Frame.H), "frame": n.Frame.Data()}
	if n.Kind == KindImage {
		m["encoding"], m["space"], m["channels"] = n.Enc.String(), n.Space.String(), float64(n.Channels)
	}
	if n.Frame.Picture {
		m["picture"] = true
	}
	if info, ok := SourceInfo(n); ok {
		for k, v := range info {
			m[k] = v
		}
	}
	return m
}

// DataValue is an evaluated Value node: plain data made of float64,
// string, bool, []any and map[string]any. Data held by the cache is shared
// and must not be modified.
type DataValue struct {
	Data any
}

var _ Value = &DataValue{}

func (v *DataValue) Kind() Kind   { return KindValue }
func (v *DataValue) Bytes() int64 { return 64 + dataSize(v.Data) }

func dataSize(d any) int64 {
	switch x := d.(type) {
	case string:
		return int64(len(x)) + 16
	case []any:
		n := int64(24)
		for _, e := range x {
			n += dataSize(e)
		}
		return n
	case []float64:
		return 24 + 8*int64(len(x))
	case []string:
		n := int64(24)
		for _, e := range x {
			n += int64(len(e)) + 16
		}
		return n
	case map[string]any:
		n := int64(48)
		for k, e := range x {
			n += int64(len(k)) + 16 + dataSize(e)
		}
		return n
	}
	return 16
}

// Number returns the data as a float64.
func (v *DataValue) Number() (float64, error) {
	f, ok := normalize(v.Data).(float64)
	if !ok {
		return 0, Errorf(EArgType, "expected a number, got %s", describeRaw(normalize(v.Data)))
	}
	return f, nil
}

func (v *DataValue) String() string { return fmt.Sprintf("Value(%v)", v.Data) }
