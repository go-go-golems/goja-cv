// Package buffer holds planar float32 linear-light image buffers, sRGB
// conversion, image IO and the preview pyramid.
package buffer

import "fmt"

// Buffer is planar float32 image data in linear light. Buffers handed out by
// the evaluator are shared through the cache: kernels must treat their inputs
// as read-only and always allocate their outputs.
type Buffer struct {
	W, H, C int       // C = 1 (mask, gray), 3 (RGB)
	Pix     []float32 // len = W*H*C, planar: channel c starts at c*W*H
	Scale   float64   // buffer px per doc px: 1.0 at export, ~0.25 for a preview
}

// New allocates a zeroed buffer.
func New(w, h, c int, scale float64) *Buffer {
	return &Buffer{W: w, H: h, C: c, Pix: make([]float32, w*h*c), Scale: scale}
}

// NewLike allocates a zeroed buffer with the same geometry and scale as b.
func NewLike(b *Buffer) *Buffer { return New(b.W, b.H, b.C, b.Scale) }

// NewLikeC allocates a zeroed buffer with b's geometry and c channels.
func NewLikeC(b *Buffer, c int) *Buffer { return New(b.W, b.H, c, b.Scale) }

// N is the number of pixels per plane.
func (b *Buffer) N() int { return b.W * b.H }

// Plane returns channel c as a slice of W*H values.
func (b *Buffer) Plane(c int) []float32 {
	n := b.W * b.H
	return b.Pix[c*n : (c+1)*n]
}

// Bytes is the memory held by the pixel data.
func (b *Buffer) Bytes() int64 { return int64(len(b.Pix)) * 4 }

// Clone returns a deep copy.
func (b *Buffer) Clone() *Buffer {
	out := &Buffer{W: b.W, H: b.H, C: b.C, Scale: b.Scale, Pix: make([]float32, len(b.Pix))}
	copy(out.Pix, b.Pix)
	return out
}

func (b *Buffer) String() string {
	return fmt.Sprintf("Buffer(%dx%dx%d @%.4g)", b.W, b.H, b.C, b.Scale)
}

// Rec709 luminance weights in linear light.
const (
	LumR = 0.2126
	LumG = 0.7152
	LumB = 0.0722
)

// Luminance returns the linear luminance plane of b (the only plane for C=1).
// For C=1 the returned slice aliases b and must not be modified.
func (b *Buffer) Luminance() []float32 {
	if b.C == 1 {
		return b.Plane(0)
	}
	n := b.N()
	out := make([]float32, n)
	r, g, bl := b.Plane(0), b.Plane(1), b.Plane(2)
	for i := 0; i < n; i++ {
		out[i] = LumR*r[i] + LumG*g[i] + LumB*bl[i]
	}
	return out
}
