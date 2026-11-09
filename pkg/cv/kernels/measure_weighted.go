package kernels

import (
	"context"
	"sort"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/pkg/errors"
)

// ErrEmptySelection is returned by StatsWeighted when no weight is positive.
var ErrEmptySelection = errors.New("the selection is empty")

// WeightedStats is a StatsResult with the selection's total weight.
type WeightedStats struct {
	*StatsResult
	Weight float64 // sum of the weights: selected pixels at this scale
}

// StatsWeighted measures b inside a selection: w (one weight in [0, 1] per
// pixel, len b.N()) weights every pixel. W and H of the result are b's size.
//
// When every weight is 0 or 1 the selected pixels are measured exactly as
// Stats measures an image. Otherwise means and clip fractions are weighted
// averages, and percentiles are weighted quantiles that reduce to NumPy's
// linear interpolation when all weights are equal.
func StatsWeighted(ctx context.Context, b *buffer.Buffer, w []float32, percentiles []float64) (*WeightedStats, error) {
	return statsWeighted(ctx, b, w, percentiles)
}

func statsWeighted(ctx context.Context, b *buffer.Buffer, w []float32, percentiles []float64) (*WeightedStats, error) {
	n := b.N()
	if len(w) != n {
		return nil, errors.Errorf("weights have %d values for %d pixels", len(w), n)
	}
	binary := true
	var total float64
	var idx []int
	for i, v := range w {
		switch {
		case v <= 0:
			continue
		case v < 1:
			binary = false
		}
		total += float64(min(v, 1))
		idx = append(idx, i)
	}
	if len(idx) == 0 {
		return nil, ErrEmptySelection
	}
	if binary {
		sub := buffer.New(len(idx), 1, b.C, b.Scale)
		for c := 0; c < b.C; c++ {
			src, dst := b.Plane(c), sub.Plane(c)
			for j, i := range idx {
				dst[j] = src[i]
			}
		}
		s, err := Stats(ctx, sub, percentiles)
		if err != nil {
			return nil, err
		}
		s.W, s.H = b.W, b.H
		return &WeightedStats{StatsResult: s, Weight: total}, nil
	}
	for _, p := range percentiles {
		if p < 0 || p > 1 {
			return nil, errors.Errorf("percentile %g outside [0, 1]", p)
		}
	}
	res := &StatsResult{W: b.W, H: b.H, RequestedQuantiles: percentiles}
	ws := make([]float32, len(idx))
	for j, i := range idx {
		ws[j] = min(w[i], 1)
	}
	gather := func(plane []float32) []float32 {
		out := make([]float32, len(idx))
		for j, i := range idx {
			out[j] = plane[i]
		}
		return out
	}
	var err error
	if res.L, err = weightedChannel(ctx, gather(b.Luminance()), ws, total, percentiles); err != nil {
		return nil, err
	}
	if b.C > 1 {
		for c := 0; c < b.C; c++ {
			cs, err := weightedChannel(ctx, gather(b.Plane(c)), ws, total, percentiles)
			if err != nil {
				return nil, err
			}
			res.Channels = append(res.Channels, cs)
		}
	}
	var lo, hi float64
	for j, i := range idx {
		anyLo, anyHi := false, false
		for c := 0; c < b.C; c++ {
			v := b.Pix[c*n+i]
			anyLo = anyLo || v <= ClipLow
			anyHi = anyHi || v >= ClipHigh
		}
		if anyLo {
			lo += float64(ws[j])
		}
		if anyHi {
			hi += float64(ws[j])
		}
	}
	res.ClipLow, res.ClipHigh = lo/total, hi/total
	return &WeightedStats{StatsResult: res, Weight: total}, nil
}

func weightedChannel(ctx context.Context, vals, ws []float32, total float64, percentiles []float64) (ChannelStats, error) {
	if err := ctx.Err(); err != nil {
		return ChannelStats{}, err
	}
	var cs ChannelStats
	var sum, lo, hi float64
	for j, v := range vals {
		wt := float64(ws[j])
		sum += wt * float64(v)
		if v <= ClipLow {
			lo += wt
		}
		if v >= ClipHigh {
			hi += wt
		}
	}
	cs.Mean, cs.ClipLow, cs.ClipHigh = sum/total, lo/total, hi/total
	order := make([]int, len(vals))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return vals[order[a]] < vals[order[b]] })
	q := func(p float64) float64 { return weightedQuantile(vals, ws, order, total, p) }
	cs.P01, cs.Median, cs.P99 = q(0.01), q(0.5), q(0.99)
	for _, p := range percentiles {
		cs.Percentiles = append(cs.Percentiles, q(p))
	}
	return cs, ctx.Err()
}

// weightedQuantile places sorted sample k at (S_k − w_k) / (total − w_last),
// where S_k is the cumulative weight, and interpolates linearly; with equal
// weights that is k / (n − 1), NumPy's default.
func weightedQuantile(vals, ws []float32, order []int, total, p float64) float64 {
	n := len(order)
	if n == 1 {
		return float64(vals[order[0]])
	}
	denom := total - float64(ws[order[n-1]])
	if denom <= 0 {
		return float64(vals[order[n-1]])
	}
	var cum, prevPos float64
	prevVal := float64(vals[order[0]])
	for k, i := range order {
		cum += float64(ws[i])
		pos := (cum - float64(ws[i])) / denom
		v := float64(vals[i])
		if pos >= p {
			if k == 0 || pos == prevPos {
				return v
			}
			f := (p - prevPos) / (pos - prevPos)
			return prevVal + f*(v-prevVal)
		}
		prevPos, prevVal = pos, v
	}
	return float64(vals[order[n-1]])
}
