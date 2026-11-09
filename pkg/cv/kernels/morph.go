package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// minMax1D is the van Herk/Gil-Werman running minimum (or maximum) over a
// centred window of k = 2r+1 samples, ignoring samples outside the array.
// It costs three comparisons per sample whatever k is. g and h are scratch
// of length len(src)+2r rounded up to a multiple of k.
func minMax1D(src, dst []float32, r int, isMax bool, g, h []float32) {
	n := len(src)
	k := 2*r + 1
	pad := float32(math.Inf(1))
	better := func(a, b float32) float32 { return min(a, b) }
	if isMax {
		pad = float32(math.Inf(-1))
		better = func(a, b float32) float32 { return max(a, b) }
	}
	m := len(g)
	at := func(j int) float32 { // padded input
		i := j - r
		if i < 0 || i >= n {
			return pad
		}
		return src[i]
	}
	for b0 := 0; b0 < m; b0 += k {
		g[b0] = at(b0)
		for j := b0 + 1; j < b0+k && j < m; j++ {
			g[j] = better(g[j-1], at(j))
		}
		e := min(b0+k, m) - 1
		h[e] = at(e)
		for j := e - 1; j >= b0; j-- {
			h[j] = better(h[j+1], at(j))
		}
	}
	for i := 0; i < n; i++ {
		// window [i, i+k-1] in padded coordinates
		dst[i] = better(h[i], g[i+k-1])
	}
}

func scratchLen(n, r int) int {
	k := 2*r + 1
	l := n + 2*r
	return (l + k - 1) / k * k
}

// SquareMinMax filters a w×h plane with a (2r+1)² square minimum (erosion)
// or maximum (dilation), separably.
func SquareMinMax(ctx context.Context, p []float32, w, h, r int, isMax bool) ([]float32, error) {
	if r <= 0 {
		return append([]float32(nil), p...), nil
	}
	tmp := make([]float32, w*h)
	out := make([]float32, w*h)
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		sl := scratchLen(w, r)
		g, hh := make([]float32, sl), make([]float32, sl)
		for y := y0; y < y1; y++ {
			minMax1D(p[y*w:(y+1)*w], tmp[y*w:(y+1)*w], r, isMax, g, hh)
		}
	})
	if err != nil {
		return nil, err
	}
	err = parallel.Rows(ctx, w, func(x0, x1 int) {
		sl := scratchLen(h, r)
		g, hh := make([]float32, sl), make([]float32, sl)
		col, res := make([]float32, h), make([]float32, h)
		for x := x0; x < x1; x++ {
			for y := 0; y < h; y++ {
				col[y] = tmp[y*w+x]
			}
			minMax1D(col, res, r, isMax, g, hh)
			for y := 0; y < h; y++ {
				out[y*w+x] = res[y]
			}
		}
	})
	return out, err
}

// Opening is erosion then dilation with a (2r+1)² square: it removes bright
// structures smaller than the square.
func Opening(ctx context.Context, p []float32, w, h, r int) ([]float32, error) {
	e, err := SquareMinMax(ctx, p, w, h, r, false)
	if err != nil {
		return nil, err
	}
	return SquareMinMax(ctx, e, w, h, r, true)
}

// Closing is dilation then erosion: it removes dark structures smaller than
// the square.
func Closing(ctx context.Context, p []float32, w, h, r int) ([]float32, error) {
	d, err := SquareMinMax(ctx, p, w, h, r, true)
	if err != nil {
		return nil, err
	}
	return SquareMinMax(ctx, d, w, h, r, false)
}
