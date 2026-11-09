package kernels

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// BlendModeNames lists the blend modes in display order. Arithmetic
// (add, sub, min, max) is not a blend mode: it has its own ops.
var BlendModeNames = []string{"normal", "multiply", "screen"}

// BlendModes are per-sample blend functions (a base, b top).
var BlendModes = map[string]func(a, b float32) float32{
	"normal":   func(_, b float32) float32 { return b },
	"multiply": func(a, b float32) float32 { return a * b },
	"screen":   func(a, b float32) float32 { return 1 - (1-a)*(1-b) },
}

// matchChannels returns a and b with the same channel count (gray is
// expanded to RGB when the other is RGB).
func matchChannels(a, b *buffer.Buffer) (*buffer.Buffer, *buffer.Buffer) {
	if a.C == b.C {
		return a, b
	}
	return ToRGB(a), ToRGB(b)
}

// binary is the one per-sample loop of two-image kernels: out[c][i] =
// f(i, a[c][i], b[c][i]) with b broadcast from gray to RGB when a is RGB
// (and the other way round).
func binary(ctx context.Context, a, b *buffer.Buffer, f func(i int, x, y float32) float32) (*buffer.Buffer, error) {
	a, b = matchChannels(a, b)
	if a.W != b.W || a.H != b.H {
		return nil, errors.Errorf("images of %d×%d and %d×%d px", a.W, a.H, b.W, b.H)
	}
	out := buffer.NewLike(a)
	err := parallel.Rows(ctx, a.H, func(y0, y1 int) {
		for c := 0; c < a.C; c++ {
			pa, pb, po := a.Plane(c), b.Plane(c), out.Plane(c)
			for i := y0 * a.W; i < y1*a.W; i++ {
				po[i] = f(i, pa[i], pb[i])
			}
		}
	})
	return out, err
}

// Binary computes f(a, b) per sample (a gray operand is broadcast).
func Binary(ctx context.Context, a, b *buffer.Buffer, f func(x, y float32) float32) (*buffer.Buffer, error) {
	return binary(ctx, a, b, func(_ int, x, y float32) float32 { return f(x, y) })
}

// Unary computes f(a) per sample.
func Unary(ctx context.Context, a *buffer.Buffer, f func(x float32) float32) (*buffer.Buffer, error) {
	out := buffer.NewLike(a)
	err := parallel.Rows(ctx, a.H, func(y0, y1 int) {
		for c := 0; c < a.C; c++ {
			pa, po := a.Plane(c), out.Plane(c)
			for i := y0 * a.W; i < y1*a.W; i++ {
				po[i] = f(pa[i])
			}
		}
	})
	return out, err
}

// Blend computes a + (f(a, b) − a) · opacity · m per sample; m may be nil
// (everywhere).
func Blend(ctx context.Context, a, b, m *buffer.Buffer, f func(a, b float32) float32, opacity float32) (*buffer.Buffer, error) {
	var mp []float32
	if m != nil {
		mp = m.Plane(0)
	}
	return binary(ctx, a, b, func(i int, x, y float32) float32 {
		w := opacity
		if mp != nil {
			w *= mp[i]
		}
		return x + (f(x, y)-x)*w
	})
}

// MaskMix is a·(1 − o·m) + b·o·m: b shown through mask m at opacity o.
func MaskMix(ctx context.Context, a, b, m *buffer.Buffer, o float32) (*buffer.Buffer, error) {
	return Blend(ctx, a, b, m, BlendModes["normal"], o)
}
