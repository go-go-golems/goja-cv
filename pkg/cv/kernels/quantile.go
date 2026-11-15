package kernels

import (
	"context"
	"math"
	"runtime"
	"sort"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// Exact quantiles without reordering or copying the input (GCV-007). The
// whole-image stats of a 24 MP scan used to copy each of four planes and run
// at least three quickselects over 24 million values per plane on one core
// (4.8 s). Quantiles finds the same order statistics with two parallel
// passes of a radix histogram over an order-preserving 32-bit key: the high
// 16 bits pick the bin that holds each wanted rank, the low 16 bits of the
// values in those bins give the exact key, which is the value. The results
// are identical to Percentile's (NumPy's linear interpolation between the
// two nearest order statistics), which the tests check value for value.

// quantileMinParallel is the size below which Quantiles copies and selects
// (Percentile): histograms cost more than they save on small inputs.
const quantileMinParallel = 1 << 16

// orderKey maps a float32 to a uint32 whose unsigned order is the float
// order (negative values reversed below positive ones). -0 sorts just below
// +0; both are 0, so the value returned is the same number.
func orderKey(v float32) uint32 {
	b := math.Float32bits(v)
	if b&0x80000000 != 0 {
		return ^b
	}
	return b | 0x80000000
}

func fromOrderKey(k uint32) float32 {
	if k&0x80000000 != 0 {
		return math.Float32frombits(k &^ 0x80000000)
	}
	return math.Float32frombits(^k)
}

// Quantiles returns the ps-quantiles of a (each p in [0, 1]) exactly as
// Percentile would, without modifying a. Values must not be NaN.
func Quantiles(ctx context.Context, a []float32, ps []float64) ([]float64, error) {
	n := len(a)
	out := make([]float64, len(ps))
	if n == 0 || len(ps) == 0 {
		return out, ctx.Err()
	}
	if n < quantileMinParallel {
		scratch := append([]float32(nil), a...)
		for i, p := range ps {
			out[i] = Percentile(scratch, p)
		}
		return out, ctx.Err()
	}
	// the order statistics we need: lo and lo+1 for each p
	type want struct {
		lo, hi int // ranks (0-based); hi = lo when no interpolation is needed
		f      float64
	}
	wants := make([]want, len(ps))
	rankSet := map[int]bool{}
	for i, p := range ps {
		pos := p * float64(n-1)
		lo := int(math.Floor(pos))
		w := want{lo: lo, hi: lo, f: pos - float64(lo)}
		if w.f != 0 && lo+1 < n {
			w.hi = lo + 1
		}
		wants[i] = w
		rankSet[w.lo], rankSet[w.hi] = true, true
	}
	ranks := make([]int, 0, len(rankSet))
	for r := range rankSet {
		ranks = append(ranks, r)
	}
	sort.Ints(ranks)

	chunks := runtime.GOMAXPROCS(0) * 2
	bounds := func(i int) (int, int) { return i * n / chunks, (i + 1) * n / chunks }

	// pass 1: counts of the high 16 bits
	hiHist := make([][]uint32, chunks)
	if err := parallel.Each(ctx, chunks, func(c int) error {
		h := make([]uint32, 1<<16)
		lo, hi := bounds(c)
		for _, v := range a[lo:hi] {
			h[orderKey(v)>>16]++
		}
		hiHist[c] = h
		return nil
	}); err != nil {
		return nil, err
	}
	total := make([]uint64, 1<<16)
	for _, h := range hiHist {
		for b, c := range h {
			total[b] += uint64(c)
		}
	}
	// each rank's bin and its rank within the bin
	type place struct {
		bin    int
		within uint64
	}
	places := map[int]place{}
	var cum uint64
	ri := 0
	for b := 0; b < 1<<16 && ri < len(ranks); b++ {
		next := cum + total[b]
		for ri < len(ranks) && uint64(ranks[ri]) < next {
			places[ranks[ri]] = place{bin: b, within: uint64(ranks[ri]) - cum}
			ri++
		}
		cum = next
	}
	bins := map[int]int{} // bin → index into the per-bin low histograms
	for _, pl := range places {
		if _, ok := bins[pl.bin]; !ok {
			bins[pl.bin] = len(bins)
		}
	}
	binOf := make([]int32, 1<<16) // bin → index+1, 0 when not needed
	for b, i := range bins {
		binOf[b] = int32(i + 1) // #nosec G115 -- at most 2 × len(ps) bins
	}

	// pass 2: counts of the low 16 bits inside the needed bins
	loHist := make([][][]uint32, chunks)
	if err := parallel.Each(ctx, chunks, func(c int) error {
		hs := make([][]uint32, len(bins))
		lo, hi := bounds(c)
		for _, v := range a[lo:hi] {
			k := orderKey(v)
			if j := binOf[k>>16]; j != 0 {
				if hs[j-1] == nil {
					hs[j-1] = make([]uint32, 1<<16)
				}
				hs[j-1][k&0xffff]++
			}
		}
		loHist[c] = hs
		return nil
	}); err != nil {
		return nil, err
	}
	value := map[int]float32{}
	for r, pl := range places {
		j := bins[pl.bin]
		var cum uint64
		for low := 0; low < 1<<16; low++ {
			var cnt uint64
			for _, hs := range loHist {
				if hs[j] != nil {
					cnt += uint64(hs[j][low])
				}
			}
			if pl.within < cum+cnt {
				value[r] = fromOrderKey(uint32(pl.bin)<<16 | uint32(low)) // #nosec G115 -- bin < 2^16, low < 2^16
				break
			}
			cum += cnt
		}
	}
	for i, w := range wants {
		vlo := value[w.lo]
		if w.hi == w.lo {
			out[i] = float64(vlo)
			continue
		}
		vhi := value[w.hi]
		out[i] = float64(vlo) + w.f*float64(vhi-vlo) // the float32 difference, as Percentile computes it
	}
	return out, ctx.Err()
}
