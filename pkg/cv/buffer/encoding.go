package buffer

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// Encoding is the transfer function between encoded values (what a file
// stores and a look shows) and linear values (the encoding undone,
// proportional to the quantity the data measures).
type Encoding uint8

const (
	EncNone   Encoding = iota // masks and non-image values
	EncSRGB                   // the sRGB curve
	EncLinear                 // the identity
)

// Encodings lists the image encodings by name, in display order.
var Encodings = []string{"srgb", "linear"}

func (e Encoding) String() string {
	switch e {
	case EncSRGB:
		return "srgb"
	case EncLinear:
		return "linear"
	case EncNone:
		return ""
	}
	return "unknown"
}

// ParseEncoding reads "srgb" or "linear".
func ParseEncoding(s string) (Encoding, bool) {
	switch s {
	case "srgb":
		return EncSRGB, true
	case "linear":
		return EncLinear, true
	}
	return EncNone, false
}

// Space is the representation a set of values is in.
type Space uint8

const (
	SpaceNone    Space = iota // not levels: masks, counts, geometry
	SpaceLinear               // x: the encoding undone
	SpaceEncoded              // e: normalized code values, as stored and displayed
)

// Spaces lists the working spaces by name.
var Spaces = []string{"linear", "encoded"}

func (s Space) String() string {
	switch s {
	case SpaceLinear:
		return "linear"
	case SpaceEncoded:
		return "encoded"
	case SpaceNone:
		return ""
	}
	return "unknown"
}

// ParseSpace reads "linear" or "encoded".
func ParseSpace(s string) (Space, bool) {
	switch s {
	case "linear":
		return SpaceLinear, true
	case "encoded":
		return SpaceEncoded, true
	}
	return SpaceNone, false
}

// Identity reports whether converting values of encoding e from space from
// to space to is the identity: always for from == to, for every pair when e
// is linear (or none: masks and derived values are never converted).
func Identity(e Encoding, from, to Space) bool {
	return from == to || e != EncSRGB || from == SpaceNone || to == SpaceNone
}

// Convert returns b's values, which are in space from, in space to, for
// encoding e, using the exact transfer functions (a round trip is accurate to
// float32 precision). It returns b itself when Identity holds. b is not
// modified.
func Convert(ctx context.Context, b *Buffer, e Encoding, from, to Space) (*Buffer, error) {
	if Identity(e, from, to) {
		return b, nil
	}
	var f func(float64) float64
	switch {
	case from == SpaceLinear && to == SpaceEncoded:
		f = LinearToSRGB
	case from == SpaceEncoded && to == SpaceLinear:
		f = SRGBToLinear
	default:
		return nil, errors.Errorf("convert %s from %s to %s: unsupported", e, from, to)
	}
	out := NewLike(b)
	n := b.N()
	err := parallel.Rows(ctx, b.H, func(y0, y1 int) {
		for c := 0; c < b.C; c++ {
			src, dst := b.Pix[c*n:(c+1)*n], out.Pix[c*n:(c+1)*n]
			for i := y0 * b.W; i < y1*b.W; i++ {
				dst[i] = float32(f(float64(src[i])))
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
