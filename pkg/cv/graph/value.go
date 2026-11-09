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

// ImageValue is an evaluated Image (or, later, Mask) node.
type ImageValue struct {
	Buf        *buffer.Buffer
	DocW, DocH int
	kind       Kind
}

var _ Value = &ImageValue{}

// NewImage wraps a buffer as an Image value.
func NewImage(b *buffer.Buffer, docW, docH int) *ImageValue {
	return &ImageValue{Buf: b, DocW: docW, DocH: docH, kind: KindImage}
}

func (v *ImageValue) Kind() Kind {
	if v.kind == "" {
		return KindImage
	}
	return v.kind
}

func (v *ImageValue) Bytes() int64 { return v.Buf.Bytes() + 64 }

// Summary is what JavaScript receives for an awaited Image: pixels never
// cross into the VM.
func (v *ImageValue) Summary() map[string]any {
	return map[string]any{
		"kind": string(v.Kind()), "w": v.Buf.W, "h": v.Buf.H, "channels": v.Buf.C,
		"scale": v.Buf.Scale, "docW": v.DocW, "docH": v.DocH,
	}
}

// DataValue is an evaluated Value or Param node: plain data made of float64,
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
