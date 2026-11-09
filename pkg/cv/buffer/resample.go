package buffer

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// Interp selects what fills the block of image px one source pixel becomes
// when a buffer is upsampled (GCV-002 §3.3).
type Interp uint8

const (
	// InterpNearest repeats each source pixel: pixel boundaries stay visible
	// and values are exact.
	InterpNearest Interp = iota
	// InterpLinear blends neighbouring pixels (bilinear, pixel centres).
	InterpLinear
)

func (i Interp) String() string {
	if i == InterpLinear {
		return "linear"
	}
	return "nearest"
}

// ParseInterp parses "nearest" or "linear".
func ParseInterp(s string) (Interp, bool) {
	switch s {
	case "nearest":
		return InterpNearest, true
	case "linear":
		return InterpLinear, true
	}
	return 0, false
}

// Cut returns the rectangle [x, x+w) × [y, y+h) of b (buffer px) as a new
// buffer; the rectangle must lie inside b.
func Cut(b *Buffer, x, y, w, h int) (*Buffer, error) {
	if x < 0 || y < 0 || w <= 0 || h <= 0 || x+w > b.W || y+h > b.H {
		return nil, errors.Errorf("cut [%d, %d, %d, %d] outside a %d×%d buffer", x, y, w, h, b.W, b.H)
	}
	out := New(w, h, b.C, b.Scale)
	for c := 0; c < b.C; c++ {
		sp, dp := b.Plane(c), out.Plane(c)
		for row := 0; row < h; row++ {
			copy(dp[row*w:(row+1)*w], sp[(y+row)*b.W+x:(y+row)*b.W+x+w])
		}
	}
	return out, nil
}

// Upsample returns b magnified by s (≥ 1) to ScaledSize(b.W, b.H, s). With
// InterpNearest, image pixel (i, j) is source pixel (⌊i / s⌋, ⌊j / s⌋), so an
// integer s gives exact s × s blocks; with InterpLinear the value is the
// bilinear interpolation at the image pixel's centre. The result's Scale is
// b.Scale · s.
func Upsample(ctx context.Context, b *Buffer, s float64, interp Interp) (*Buffer, error) {
	if !(s >= 1) {
		return nil, errors.Errorf("upsample factor %g is below 1", s)
	}
	w, h := ScaledSize(b.W, b.H, s)
	out := New(w, h, b.C, b.Scale*s)
	sx, sy := float64(w)/float64(b.W), float64(h)/float64(b.H) // effective factors after rounding
	xs := make([]int, w)
	for i := range xs {
		xs[i] = min(b.W-1, int(math.Floor(float64(i)/sx)))
	}
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		for c := 0; c < b.C; c++ {
			sp, dp := b.Plane(c), out.Plane(c)
			for j := y0; j < y1; j++ {
				row := dp[j*w : (j+1)*w]
				if interp == InterpNearest {
					src := sp[min(b.H-1, int(math.Floor(float64(j)/sy)))*b.W:]
					for i, x := range xs {
						row[i] = src[x]
					}
					continue
				}
				fy := (float64(j)+0.5)/sy - 0.5
				y0i := int(math.Floor(fy))
				ty := float32(fy - float64(y0i))
				ya, yb := clampInt(y0i, b.H-1), clampInt(y0i+1, b.H-1)
				for i := range row {
					fx := (float64(i)+0.5)/sx - 0.5
					x0i := int(math.Floor(fx))
					tx := float32(fx - float64(x0i))
					xa, xb := clampInt(x0i, b.W-1), clampInt(x0i+1, b.W-1)
					top := sp[ya*b.W+xa]*(1-tx) + sp[ya*b.W+xb]*tx
					bot := sp[yb*b.W+xa]*(1-tx) + sp[yb*b.W+xb]*tx
					row[i] = top*(1-ty) + bot*ty
				}
			}
		}
	})
	return out, err
}

func clampInt(v, hi int) int { return max(0, min(v, hi)) }
