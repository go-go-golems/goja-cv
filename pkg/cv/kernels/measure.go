package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/pkg/errors"
)

// Clip thresholds in linear light: a sample at or below ClipLow counts as
// crushed black, at or above ClipHigh as blown white.
const (
	ClipLow  = 1e-5
	ClipHigh = 0.9999
)

// ChannelStats summarizes one plane (linear light).
type ChannelStats struct {
	Mean, Median, P01, P99, ClipLow, ClipHigh float64
	Percentiles                               []float64 // values for the requested percentiles
}

// StatsResult summarizes a buffer. L is the luminance plane the caller
// passes (the graph's one luminance; the only plane for gray images). ClipLow/ClipHigh are fractions of pixels with
// any channel clipped.
type StatsResult struct {
	W, H               int
	L                  ChannelStats
	Channels           []ChannelStats
	ClipLow, ClipHigh  float64
	RequestedQuantiles []float64
}

// Stats measures b. percentiles are fractions in [0, 1] computed exactly
// (linear interpolation between order statistics) for luminance and every
// channel. lum is the luminance plane (len b.N()); nil for a one-channel b,
// whose plane is its luminance. Kernels never weight channels themselves.
func Stats(ctx context.Context, b *buffer.Buffer, lum []float32, percentiles []float64) (*StatsResult, error) {
	n := b.N()
	if n == 0 {
		return nil, errors.New("empty image")
	}
	for _, p := range percentiles {
		if p < 0 || p > 1 {
			return nil, errors.Errorf("percentile %g outside [0, 1]", p)
		}
	}
	res := &StatsResult{W: b.W, H: b.H, RequestedQuantiles: percentiles}
	lum, err := luminance(b, lum)
	if err != nil {
		return nil, err
	}
	scratch := make([]float32, n)
	if res.L, err = channelStats(ctx, lum, scratch, percentiles); err != nil {
		return nil, err
	}
	if b.C > 1 {
		for c := 0; c < b.C; c++ {
			copy(scratch, b.Plane(c))
			cs, err := channelStats(ctx, b.Plane(c), scratch, percentiles)
			if err != nil {
				return nil, err
			}
			res.Channels = append(res.Channels, cs)
		}
	}
	var lo, hi int
	for i := 0; i < n; i++ {
		anyLo, anyHi := false, false
		for c := 0; c < b.C; c++ {
			v := b.Pix[c*n+i]
			anyLo = anyLo || v <= ClipLow
			anyHi = anyHi || v >= ClipHigh
		}
		if anyLo {
			lo++
		}
		if anyHi {
			hi++
		}
	}
	res.ClipLow, res.ClipHigh = float64(lo)/float64(n), float64(hi)/float64(n)
	return res, nil
}

// channelStats measures plane; scratch (same length) is reordered by
// selection.
func channelStats(ctx context.Context, plane, scratch []float32, percentiles []float64) (ChannelStats, error) {
	if err := ctx.Err(); err != nil {
		return ChannelStats{}, err
	}
	copy(scratch, plane)
	var cs ChannelStats
	var sum float64
	var lo, hi int
	for _, v := range plane {
		sum += float64(v)
		if v <= ClipLow {
			lo++
		}
		if v >= ClipHigh {
			hi++
		}
	}
	n := float64(len(plane))
	cs.Mean, cs.ClipLow, cs.ClipHigh = sum/n, float64(lo)/n, float64(hi)/n
	cs.P01 = Percentile(scratch, 0.01)
	cs.Median = Percentile(scratch, 0.5)
	cs.P99 = Percentile(scratch, 0.99)
	for _, p := range percentiles {
		cs.Percentiles = append(cs.Percentiles, Percentile(scratch, p))
	}
	return cs, ctx.Err()
}

// Percentile returns the p-quantile of a (reordered in place) with linear
// interpolation between the two nearest order statistics (NumPy's default).
func Percentile(a []float32, p float64) float64 {
	n := len(a)
	pos := p * float64(n-1)
	lo := int(math.Floor(pos))
	f := pos - float64(lo)
	vlo := selectK(a, lo)
	if f == 0 || lo+1 >= n {
		return float64(vlo)
	}
	// after selection every element right of lo is >= vlo; the next order
	// statistic is their minimum.
	vhi := a[lo+1]
	for _, v := range a[lo+2:] {
		if v < vhi {
			vhi = v
		}
	}
	return float64(vlo) + f*float64(vhi-vlo)
}

// selectK partially orders a so that a[k] is the k-th smallest element and
// returns it (quickselect with a three-way partition, robust to the large
// runs of equal values that clipped images have).
func selectK(a []float32, k int) float32 {
	lo, hi := 0, len(a)-1
	for lo < hi {
		mid := lo + (hi-lo)/2
		p := median3(a[lo], a[mid], a[hi])
		lt, i, gt := lo, lo, hi
		for i <= gt {
			switch {
			case a[i] < p:
				a[lt], a[i] = a[i], a[lt]
				lt++
				i++
			case a[i] > p:
				a[i], a[gt] = a[gt], a[i]
				gt--
			default:
				i++
			}
		}
		switch {
		case k < lt:
			hi = lt - 1
		case k > gt:
			lo = gt + 1
		default:
			return p
		}
	}
	return a[k]
}

func median3(a, b, c float32) float32 {
	if a > b {
		a, b = b, a
	}
	if b > c {
		b = c
	}
	if a > b {
		b = a
	}
	return b
}

// HistogramResult counts samples in equal bins over [0, 1] of the space
// the values are in. Under/Over count samples below 0 and
// above 1.
type HistogramResult struct {
	Counts      []int
	Under, Over int
	Total       int
}

// Histogram bins channel ("L" = luminance: the only plane of a 1-channel
// buffer, else the Rec. 709 weighted sum; or "r", "g", "b") of b over
// [0, 1] of the space its values are in.
func Histogram(ctx context.Context, b *buffer.Buffer, bins int, channel string) (*HistogramResult, error) {
	if bins < 1 {
		return nil, errors.Errorf("bins = %d", bins)
	}
	var plane []float32
	switch channel {
	case "L":
		if b.C != 1 {
			return nil, errors.New("channel L reads the luminance plane: pass a one-channel buffer")
		}
		plane = b.Plane(0)
	case "r", "g", "b":
		c := map[string]int{"r": 0, "g": 1, "b": 2}[channel]
		if c >= b.C {
			return nil, errors.Errorf("channel %q: the image has %d channel(s)", channel, b.C)
		}
		plane = b.Plane(c)
	default:
		return nil, errors.Errorf("unknown channel %q", channel)
	}
	res := &HistogramResult{Counts: make([]int, bins), Total: len(plane)}
	for i, v := range plane {
		if i%65536 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p := v
		switch {
		case p < 0:
			res.Under++
		case p > 1:
			res.Over++
		default:
			k := int(p * float32(bins))
			if k >= bins {
				k = bins - 1
			}
			res.Counts[k]++
		}
	}
	return res, nil
}

// luminance is the luminance plane of b: lum (checked), or b's plane when b
// has one channel and lum is nil.
func luminance(b *buffer.Buffer, lum []float32) ([]float32, error) {
	switch {
	case lum != nil && len(lum) != b.N():
		return nil, errors.Errorf("luminance plane has %d values for %d pixels", len(lum), b.N())
	case lum != nil:
		return lum, nil
	case b.C == 1:
		return b.Plane(0), nil
	}
	return nil, errors.Errorf("a %d-channel buffer needs its luminance plane", b.C)
}
