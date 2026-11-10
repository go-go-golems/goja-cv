package kernels

import (
	"context"
	"math"
	"slices"
	"sort"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// Peaks (GCV-005 §4.5, §7.8, §10.7): the local maxima of the core/ring step,
// measure's step, as disks. For each radius r the step map (the mean of a
// digital disk of radius r minus the mean of the digital ring between 1.5 r
// and 2.5 r, over in-image pixels) is computed in bands of rows from row
// prefix sums; a pixel whose step is positive and equal to the maximum of the
// step map over the disk of radius r (DiskMinMax) is a candidate; its centre
// is refined by parabolas through its four neighbours; a cheap digital-ring
// snr drops candidates far below the threshold; ProbePoint, the exact
// coverage-weighted measure, decides and gives the reported numbers. Across
// radii and polarities, of two peaks closer than the sum of their radii the
// one with the higher |snr| is kept.

// PeakOptions configures Peaks. Radii are in buffer px.
type PeakOptions struct {
	Radii    []float64
	MinSNR   float64
	Polarity string // "bright" | "dark" | "both"

	band        int  // output rows per band (0: peakBand); tests vary it
	noPrefilter bool // tests: measure every candidate exactly
}

// Peak is one peak: its sub-pixel centre (buffer px, pixel i's centre at
// i + 0.5), the radius it was found at, and the core/ring measure there
// (core R, ring [1.5 R, 2.5 R]).
type Peak struct {
	CX, CY, R float64
	CoreRing  CoreRing
}

// peakBand is the number of output rows of a band.
const peakBand = 64

// peakPrefilter is the fraction of MinSNR below which a candidate's
// digital-ring snr drops it before the exact measure. The digital snr at the
// pixel was at least 0.63 × the exact snr at the refined centre on the
// synthetic fields (r = 1.5 is the worst: digital disks of small radii are
// coarse); 0.8, the guide's figure, dropped peaks the exact measure keeps.
const peakPrefilter = 0.5

// peakRadius holds the digital disks of one radius: chord half-widths of
// the core (r), the inner (1.5 r) and the outer (2.5 r) disk.
type peakRadius struct {
	r                  float64
	core, inner, outer []int
	reach              int // ⌊2.5 r⌋: the rows the step of a pixel reads on either side
	nmsReach           int // ⌈r⌉: the rows non-maximum suppression reads on either side
	ringDX, ringDY     []int
}

func newPeakRadius(r float64) peakRadius {
	p := peakRadius{r: r, core: diskChords(r), inner: diskChords(1.5 * r), outer: diskChords(2.5 * r)}
	p.reach = len(p.outer) - 1
	p.nmsReach = int(math.Ceil(r))
	for dy := -p.reach; dy <= p.reach; dy++ {
		d := abs(dy)
		a1 := -1
		if d < len(p.inner) {
			a1 = p.inner[d]
		}
		for dx := -p.outer[d]; dx <= p.outer[d]; dx++ {
			if abs(dx) > a1 {
				p.ringDX = append(p.ringDX, dx)
				p.ringDY = append(p.ringDY, dy)
			}
		}
	}
	return p
}

// Peaks searches the one-channel buffer L (scale 1). ProbePoint is called on
// it with its own plane as the gray plane.
func Peaks(ctx context.Context, L *buffer.Buffer, o PeakOptions) ([]Peak, error) {
	if L.C != 1 {
		return nil, errors.Errorf("peaks: %d channels, want 1", L.C)
	}
	var signs []float32
	switch o.Polarity {
	case "", "bright":
		signs = []float32{1}
	case "dark":
		signs = []float32{-1}
	case "both":
		signs = []float32{1, -1}
	default:
		return nil, errors.Errorf("peaks: polarity %q", o.Polarity)
	}
	band := o.band
	if band <= 0 {
		band = peakBand
	}
	w, h := L.W, L.H
	gray := L.Plane(0)
	type job struct {
		pr     *peakRadius
		y0, y1 int
	}
	var jobs []job
	for _, r := range o.Radii {
		if !(r >= 1) {
			return nil, errors.Errorf("peaks: radius %g, want ≥ 1", r)
		}
		pr := newPeakRadius(r)
		for y0 := 0; y0 < h; y0 += band {
			jobs = append(jobs, job{&pr, y0, min(y0+band, h)})
		}
	}
	// every band is one task, computed serially: the step map of its rows
	// and margins, NMS, the pre-filter and the exact measure
	found := make([][]Peak, len(jobs))
	err := parallel.Each(ctx, len(jobs), func(i int) error {
		j := jobs[i]
		sy0, sy1 := max(0, j.y0-j.pr.nmsReach-1), min(h, j.y1+j.pr.nmsReach+1)
		S, err := stepRows(ctx, serialRows, gray, w, h, *j.pr, sy0, sy1)
		if err != nil {
			return err
		}
		T := make([]float32, len(S))
		for _, s := range signs {
			for k, v := range S {
				T[k] = s * v
			}
			ps, err := bandPeaks(ctx, L, gray, T, w, h, sy0, sy1, j.y0, j.y1, *j.pr, s, o)
			if err != nil {
				return err
			}
			found[i] = append(found[i], ps...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var all []Peak
	for _, ps := range found {
		all = append(all, ps...)
	}
	return mergePeaks(all), nil
}

// stepRows is the step map of rows [sy0, sy1): digital disk mean (r) minus
// digital ring mean ([1.5 r, 2.5 r]), over in-image pixels, from row prefix
// sums of the rows they read.
func stepRows(ctx context.Context, rows rowRunner, gray []float32, w, h int, pr peakRadius, sy0, sy1 int) ([]float32, error) {
	p0, p1 := max(0, sy0-pr.reach), min(h, sy1+pr.reach)
	P := make([]float64, (p1-p0)*(w+1)) // row prefix sums: P[row][x] = Σ gray[row][0:x]
	err := rows(ctx, p1-p0, func(j0, j1 int) {
		for j := j0; j < j1; j++ {
			src, dst := gray[(p0+j)*w:(p0+j+1)*w], P[j*(w+1):(j+1)*(w+1)]
			var acc float64
			for x, v := range src {
				dst[x] = acc
				acc += float64(v)
			}
			dst[w] = acc
		}
	})
	if err != nil {
		return nil, err
	}
	S := make([]float32, (sy1-sy0)*w)
	err = rows(ctx, sy1-sy0, func(j0, j1 int) {
		sc, nc := make([]float64, w), make([]float64, w)
		si, ni := make([]float64, w), make([]float64, w)
		so, no := make([]float64, w), make([]float64, w)
		disk := func(y int, half []int, sum, cnt []float64) {
			clear(sum)
			clear(cnt)
			n := len(half) - 1
			for dy := -n; dy <= n; dy++ {
				yy := y + dy
				if yy < 0 || yy >= h {
					continue
				}
				a := half[abs(dy)]
				row := P[(yy-p0)*(w+1) : (yy-p0+1)*(w+1)]
				for x := range w {
					lo, hi := max(0, x-a), min(w, x+a+1)
					sum[x] += row[hi] - row[lo]
					cnt[x] += float64(hi - lo)
				}
			}
		}
		for j := j0; j < j1; j++ {
			y := sy0 + j
			disk(y, pr.core, sc, nc)
			disk(y, pr.inner, si, ni)
			disk(y, pr.outer, so, no)
			out := S[j*w : (j+1)*w]
			for x := range w {
				rn := no[x] - ni[x]
				if rn <= 0 {
					out[x] = 0
					continue
				}
				out[x] = float32(sc[x]/nc[x] - (so[x]-si[x])/rn)
			}
		}
	})
	return S, err
}

// bandPeaks finds the peaks of T (the signed step map of rows [sy0, sy1))
// in output rows [y0, y1).
func bandPeaks(ctx context.Context, L *buffer.Buffer, gray, T []float32, w, h, sy0, sy1, y0, y1 int, pr peakRadius, s float32, o PeakOptions) ([]Peak, error) {
	D, err := chordDisk(ctx, T, w, sy1-sy0, pr.r, true, serialRows) // DiskMinMax on the band, serially
	if err != nil {
		return nil, err
	}
	var cands []int // buffer pixel indices
	for y := y0; y < y1; y++ {
		row := (y - sy0) * w
		for x := range w {
			if t := T[row+x]; t > 0 && t == D[row+x] {
				cands = append(cands, y*w+x)
			}
		}
	}
	res := make([]Peak, len(cands))
	keep := make([]bool, len(cands))
	opts := CoreRingOptions{Core: pr.r, R1: 1.5 * pr.r, R2: 2.5 * pr.r, Gray: gray}
	err = serialRows(ctx, len(cands), func(k0, k1 int) {
		vals := make([]float32, 0, len(pr.ringDX))
		for k := k0; k < k1; k++ {
			x, y := cands[k]%w, cands[k]/w
			at := func(xx, yy int) float64 { return float64(T[(yy-sy0)*w+xx]) }
			t := at(x, y)
			if !o.noPrefilter && o.MinSNR > 0 {
				vals = vals[:0]
				for i, dx := range pr.ringDX {
					xx, yy := x+dx, y+pr.ringDY[i]
					if xx >= 0 && yy >= 0 && xx < w && yy < h {
						vals = append(vals, gray[yy*w+xx])
					}
				}
				if q := t / math.Max(spread1090(vals), 1e-3); q < peakPrefilter*o.MinSNR {
					continue
				}
			}
			var dx, dy float64
			if x > 0 && x < w-1 {
				dx = parabola(at(x-1, y), t, at(x+1, y))
			}
			if y > 0 && y < h-1 {
				dy = parabola(at(x, y-1), t, at(x, y+1))
			}
			cx, cy := float64(x)+0.5+dx, float64(y)+0.5+dy
			m := ProbePoint(ctx, L, cx, cy, opts)
			if m.Outside || float64(s)*m.Step <= 0 || float64(s)*m.SNR < o.MinSNR {
				continue
			}
			res[k], keep[k] = Peak{CX: cx, CY: cy, R: pr.r, CoreRing: m}, true
		}
	})
	if err != nil {
		return nil, err
	}
	out := res[:0]
	for k, p := range res {
		if keep[k] {
			out = append(out, p)
		}
	}
	return out, nil
}

// parabola is the offset in [−0.5, 0.5] of the vertex of the parabola
// through (−1, a), (0, b), (1, c); 0 when it does not open downwards.
func parabola(a, b, c float64) float64 {
	den := a - 2*b + c
	if !(den < 0) {
		return 0
	}
	return min(0.5, max(-0.5, (a-c)/(2*den)))
}

// spread1090 is p90 − p10 of vals (linear interpolation between order
// statistics); vals is reordered.
func spread1090(vals []float32) float64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	slices.Sort(vals)
	q := func(p float64) float64 {
		f := p * float64(n-1)
		i := int(f)
		if i >= n-1 {
			return float64(vals[n-1])
		}
		return float64(vals[i]) + (f-float64(i))*float64(vals[i+1]-vals[i])
	}
	return q(0.9) - q(0.1)
}

// mergePeaks sorts peaks by |snr|, |step|, cy, cx, r and keeps each one
// that lies at least the sum of the radii from every peak kept before it.
func mergePeaks(all []Peak) []Peak {
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if x, y := math.Abs(a.CoreRing.SNR), math.Abs(b.CoreRing.SNR); x != y {
			return x > y
		}
		if x, y := math.Abs(a.CoreRing.Step), math.Abs(b.CoreRing.Step); x != y {
			return x > y
		}
		if a.CY != b.CY {
			return a.CY < b.CY
		}
		if a.CX != b.CX {
			return a.CX < b.CX
		}
		return a.R < b.R
	})
	var maxR float64
	for _, p := range all {
		maxR = max(maxR, p.R)
	}
	side := 2 * maxR
	type cell struct{ x, y int }
	grid := map[cell][]int{}
	var kept []Peak
	for _, p := range all {
		c := cell{int(math.Floor(p.CX / side)), int(math.Floor(p.CY / side))}
		clash := false
		for gy := c.y - 1; gy <= c.y+1 && !clash; gy++ {
			for gx := c.x - 1; gx <= c.x+1 && !clash; gx++ {
				for _, k := range grid[cell{gx, gy}] {
					q := kept[k]
					if math.Hypot(p.CX-q.CX, p.CY-q.CY) < p.R+q.R {
						clash = true
						break
					}
				}
			}
		}
		if !clash {
			grid[c] = append(grid[c], len(kept))
			kept = append(kept, p)
		}
	}
	return kept
}
