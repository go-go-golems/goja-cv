// Package kernels holds pure functions on buffers. Kernels know nothing about
// nodes, caching or JavaScript. Inputs are read-only; outputs are new buffers.
// Length arguments are already in buffer pixels.
package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// pointwise applies f to every sample of every channel in parallel bands.
func pointwise(ctx context.Context, in *buffer.Buffer, f func(v float32) float32) (*buffer.Buffer, error) {
	out := buffer.NewLike(in)
	err := parallel.Rows(ctx, in.H, func(y0, y1 int) {
		for c := 0; c < in.C; c++ {
			src, dst := in.Plane(c), out.Plane(c)
			for i := y0 * in.W; i < y1*in.W; i++ {
				dst[i] = f(src[i])
			}
		}
	})
	return out, err
}

// Exposure multiplies values by 2^ev.
func Exposure(ctx context.Context, in *buffer.Buffer, ev float64) (*buffer.Buffer, error) {
	k := float32(math.Exp2(ev))
	return pointwise(ctx, in, func(v float32) float32 { return v * k })
}

// Levels remaps tones: t = (v − black) / (white − black), then t^(1/gamma);
// values below black clip to 0, values above white stay above 1. black and
// white are levels of the space the values are in (the graph converts), so
// levels({black: s.p01, white: s.p99}) maps those percentiles to 0 and 1.
func Levels(ctx context.Context, in *buffer.Buffer, black, white, gamma float64) (*buffer.Buffer, error) {
	scale := 1 / (white - black)
	invG := 1 / gamma
	return pointwise(ctx, in, func(v float32) float32 {
		t := (float64(v) - black) * scale
		if t <= 0 {
			return 0
		}
		if invG != 1 {
			t = math.Pow(t, invG)
		}
		return float32(t)
	})
}

// Contrast scales values around mid-grey (0.5): v' = 0.5 + (v − 0.5)·(1 +
// amount). amount = −1 gives flat grey; negative results clip to 0.
func Contrast(ctx context.Context, in *buffer.Buffer, amount float64) (*buffer.Buffer, error) {
	k := float32(1 + amount)
	return pointwise(ctx, in, func(v float32) float32 {
		p := 0.5 + (v-0.5)*k
		if p <= 0 {
			return 0
		}
		return p
	})
}

// GrayWeights are the channel weights of ToGray: Rec. 709 luminance (defined
// on linear light) and the plain average.
var GrayWeights = map[string][3]float32{
	"rec709":  {buffer.LumR, buffer.LumG, buffer.LumB},
	"average": {1.0 / 3, 1.0 / 3, 1.0 / 3},
}

// ToGray returns a 1-channel buffer of weighted channels. A 1-channel input is
// returned as a copy.
func ToGray(ctx context.Context, in *buffer.Buffer, w [3]float32) (*buffer.Buffer, error) {
	if in.C == 1 {
		return in.Clone(), nil
	}
	out := buffer.NewLikeC(in, 1)
	r, g, b, dst := in.Plane(0), in.Plane(1), in.Plane(2), out.Plane(0)
	err := parallel.Rows(ctx, in.H, func(y0, y1 int) {
		for i := y0 * in.W; i < y1*in.W; i++ {
			dst[i] = w[0]*r[i] + w[1]*g[i] + w[2]*b[i]
		}
	})
	return out, err
}
