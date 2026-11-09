package kernels

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// BlendModeNames lists the blend modes in display order.
var BlendModeNames = []string{"normal", "multiply", "screen", "add", "difference", "lighten", "darken"}

// BlendModes are per-sample blend functions in linear light (a base, b top).
var BlendModes = map[string]func(a, b float32) float32{
	"normal":   func(_, b float32) float32 { return b },
	"multiply": func(a, b float32) float32 { return a * b },
	"screen":   func(a, b float32) float32 { return 1 - (1-a)*(1-b) },
	"add":      func(a, b float32) float32 { return a + b },
	"difference": func(a, b float32) float32 {
		if a > b {
			return a - b
		}
		return b - a
	},
	"lighten": func(a, b float32) float32 { return max(a, b) },
	"darken":  func(a, b float32) float32 { return min(a, b) },
}

// matchChannels returns a and b with the same channel count (gray is
// expanded to RGB when the other is RGB).
func matchChannels(a, b *buffer.Buffer) (*buffer.Buffer, *buffer.Buffer) {
	if a.C == b.C {
		return a, b
	}
	return ToRGB(a), ToRGB(b)
}

// Blend computes a + (f(a, b) − a) · opacity · m per sample; m may be nil
// (everywhere). Linear-light op.
func Blend(ctx context.Context, a, b, m *buffer.Buffer, f func(a, b float32) float32, opacity float32) (*buffer.Buffer, error) {
	a, b = matchChannels(a, b)
	out := buffer.NewLike(a)
	var mp []float32
	if m != nil {
		mp = m.Plane(0)
	}
	err := parallel.Rows(ctx, a.H, func(y0, y1 int) {
		for c := 0; c < a.C; c++ {
			pa, pb, po := a.Plane(c), b.Plane(c), out.Plane(c)
			for i := y0 * a.W; i < y1*a.W; i++ {
				w := opacity
				if mp != nil {
					w *= mp[i]
				}
				po[i] = pa[i] + (f(pa[i], pb[i])-pa[i])*w
			}
		}
	})
	return out, err
}

// MaskMix is a·(1 − o·m) + b·o·m: b shown through mask m at opacity o.
func MaskMix(ctx context.Context, a, b, m *buffer.Buffer, o float32) (*buffer.Buffer, error) {
	return Blend(ctx, a, b, m, BlendModes["normal"], o)
}
