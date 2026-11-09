package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// GaussianKernel returns normalized weights for sigma (buffer px) with a
// half-width of ceil(3·sigma).
func GaussianKernel(sigma float64) []float32 {
	rad := int(math.Ceil(3 * sigma))
	if rad < 1 {
		rad = 1
	}
	k := make([]float32, 2*rad+1)
	var sum float64
	w := make([]float64, len(k))
	for i := range w {
		x := float64(i - rad)
		w[i] = math.Exp(-x * x / (2 * sigma * sigma))
		sum += w[i]
	}
	for i := range k {
		k[i] = float32(w[i] / sum)
	}
	return k
}

// reflect101 maps an out-of-range index into [0, n) by mirroring without
// repeating the edge sample (…2 1 | 0 1 2 … n-1 | n-2 …).
func reflect101(i, n int) int {
	if n == 1 {
		return 0
	}
	period := 2*n - 2
	i %= period
	if i < 0 {
		i += period
	}
	if i >= n {
		i = period - i
	}
	return i
}

// Gaussian blurs each channel with a separable Gaussian of standard deviation
// sigma (buffer px), mirroring at the borders. sigma <= 0 returns a copy.
// Linear-light op.
func Gaussian(ctx context.Context, in *buffer.Buffer, sigma float64) (*buffer.Buffer, error) {
	if sigma <= 0 {
		return in.Clone(), nil
	}
	k := GaussianKernel(sigma)
	rad := len(k) / 2
	W, H := in.W, in.H
	tmp := buffer.NewLike(in)
	out := buffer.NewLike(in)
	// precomputed mirrored column indices for the horizontal pass
	xIdx := make([]int, W+2*rad)
	for i := range xIdx {
		xIdx[i] = reflect101(i-rad, W)
	}
	for c := 0; c < in.C; c++ {
		src, mid, dst := in.Plane(c), tmp.Plane(c), out.Plane(c)
		if err := parallel.Rows(ctx, H, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				row := src[y*W : (y+1)*W]
				orow := mid[y*W : (y+1)*W]
				for x := 0; x < W; x++ {
					var s float32
					for j, wt := range k {
						s += row[xIdx[x+j]] * wt
					}
					orow[x] = s
				}
			}
		}); err != nil {
			return nil, err
		}
		if err := parallel.Rows(ctx, H, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				orow := dst[y*W : (y+1)*W]
				for j, wt := range k {
					ry := reflect101(y+j-rad, H)
					srow := mid[ry*W : (ry+1)*W]
					for x := range orow {
						orow[x] += srow[x] * wt
					}
				}
			}
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// LocalContrast is (a − G_σ(a)) / (s_σ + 1e-3) per channel, where s_σ is
// the standard deviation of the values in a Gaussian window of σ (buffer
// px): local z-scores, alike in dark and bright areas.
func LocalContrast(ctx context.Context, in *buffer.Buffer, sigma float64) (*buffer.Buffer, error) {
	mean, err := Gaussian(ctx, in, sigma)
	if err != nil {
		return nil, err
	}
	sq := buffer.NewLike(in)
	for i, v := range in.Pix {
		sq.Pix[i] = v * v
	}
	msq, err := Gaussian(ctx, sq, sigma)
	if err != nil {
		return nil, err
	}
	out := buffer.NewLike(in)
	err = parallel.Rows(ctx, in.H, func(y0, y1 int) {
		for c := 0; c < in.C; c++ {
			a, m, q, o := in.Plane(c), mean.Plane(c), msq.Plane(c), out.Plane(c)
			for i := y0 * in.W; i < y1*in.W; i++ {
				v := float64(q[i]) - float64(m[i])*float64(m[i])
				o[i] = float32((float64(a[i]) - float64(m[i])) / (math.Sqrt(math.Max(v, 0)) + 1e-3))
			}
		}
	})
	return out, err
}
