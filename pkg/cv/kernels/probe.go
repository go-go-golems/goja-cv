package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// The probe kernel: point sampling, core/ring measures and line profiles in
// a small window around each point (GCV-003 §9.1). Pixels are weighted by
// EllipseCoverage, the 4×4 supersampled rule of mask.ellipse, and selected
// pixels are gathered in row-major order and measured by StatsWeighted, so a
// probe returns what img.stats({ mask: img.mask.ellipse(…) }) returns for the
// same disk or ring. Coordinates and radii here are buffer px.

// Gray selects the weights of the gray plane.
type Gray uint8

const (
	// GrayRec709 is Rec. 709 luminance (buffer.Luminance's weights).
	GrayRec709 Gray = iota
	// GrayAverage weighs the channels equally.
	GrayAverage
)

// grayPlane is the gray plane of the pixels idx of b (one channel: its plane).
func grayPlane(b *buffer.Buffer, idx []int, g Gray) []float32 {
	out := make([]float32, len(idx))
	if b.C == 1 {
		p := b.Plane(0)
		for j, i := range idx {
			out[j] = p[i]
		}
		return out
	}
	r, gr, bl := b.Plane(0), b.Plane(1), b.Plane(2)
	for j, i := range idx {
		if g == GrayAverage {
			out[j] = (r[i] + gr[i] + bl[i]) / 3
		} else {
			out[j] = buffer.LumR*r[i] + buffer.LumG*gr[i] + buffer.LumB*bl[i]
		}
	}
	return out
}

// selection is the pixels of a disk or annulus inside the buffer with
// positive weight, in row-major order.
type selection struct {
	idx []int
	w   []float32
	// inside and full are the total weight inside the buffer and of the
	// whole shape (as if the buffer extended past its edges).
	inside, full float64
}

// annulus selects the pixels between radii r1 and r2 around (cx, cy):
// weight EllipseCoverage(r2) − EllipseCoverage(r1); r1 = 0 is a disk.
func annulus(b *buffer.Buffer, cx, cy, r1, r2 float64) selection {
	var s selection
	if r2 <= 0 {
		return s
	}
	x0, x1 := int(math.Floor(cx-r2))-1, int(math.Ceil(cx+r2))+1
	y0, y1 := int(math.Floor(cy-r2))-1, int(math.Ceil(cy+r2))+1
	for j := y0; j <= y1; j++ {
		for i := x0; i <= x1; i++ {
			w := EllipseCoverage(i, j, cx, cy, r2, r2)
			if r1 > 0 && w > 0 {
				w -= EllipseCoverage(i, j, cx, cy, r1, r1)
			}
			if w <= 0 {
				continue
			}
			s.full += float64(w)
			if i < 0 || j < 0 || i >= b.W || j >= b.H {
				continue
			}
			s.inside += float64(w)
			s.idx = append(s.idx, j*b.W+i)
			s.w = append(s.w, w)
		}
	}
	return s
}

// measured is a weighted gray statistic and the weighted mean of each
// channel over one selection.
type measured struct {
	gray  *WeightedStats
	chans []float64 // per-channel means (nil for one channel)
}

func measureSel(ctx context.Context, b *buffer.Buffer, s selection, g Gray, percentiles []float64) (measured, error) {
	n := len(s.idx)
	if n == 0 {
		return measured{}, ErrEmptySelection
	}
	plane := buffer.New(n, 1, 1, b.Scale)
	copy(plane.Pix, grayPlane(b, s.idx, g))
	ws, err := StatsWeighted(ctx, plane, s.w, percentiles)
	if err != nil {
		return measured{}, err
	}
	m := measured{gray: ws}
	if b.C > 1 {
		for c := 0; c < b.C; c++ {
			p := b.Plane(c)
			var sum float64
			for j, i := range s.idx {
				sum += float64(s.w[j]) * float64(p[i])
			}
			m.chans = append(m.chans, sum/ws.Weight)
		}
	}
	return m, nil
}

// Sample is the value of a buffer near a point.
type Sample struct {
	Outside bool      // the point lies outside the buffer
	Gray    float64   // the gray plane
	Chans   []float64 // per channel (nil for one channel)
	Weight  float64   // covered weight in buffer px² (1 for radius 0)
}

// SamplePoints samples b at points (buffer px): radius 0 reads the pixel
// that contains the point, radius r > 0 the coverage-weighted mean of the
// disk of radius r (pixels outside the buffer do not count).
func SamplePoints(ctx context.Context, b *buffer.Buffer, pts [][2]float64, radius float64, g Gray) ([]Sample, error) {
	out := make([]Sample, len(pts))
	err := parallel.Rows(ctx, len(pts), func(k0, k1 int) {
		for k := k0; k < k1; k++ {
			x, y := pts[k][0], pts[k][1]
			if !(x >= 0 && y >= 0 && x < float64(b.W) && y < float64(b.H)) {
				out[k] = Sample{Outside: true}
				continue
			}
			var s selection
			if radius <= 0 {
				i := int(y)*b.W + int(x)
				s = selection{idx: []int{i}, w: []float32{1}, inside: 1, full: 1}
			} else {
				s = annulus(b, x, y, 0, radius)
			}
			m, err := measureSel(ctx, b, s, g, nil)
			if err != nil {
				out[k] = Sample{Outside: true}
				continue
			}
			out[k] = Sample{Gray: m.gray.L.Mean, Chans: m.chans, Weight: m.gray.Weight}
		}
	})
	return out, err
}

// CoreRing is the core/ring measure of one point (GCV-003 §3.3).
type CoreRing struct {
	Outside      bool // the core has no pixel inside the buffer
	CoreMean     float64
	RingMean     float64
	Step, Spread float64
	SNR          float64
	ChanSteps    []float64 // per channel (nil for one channel)
	StepChroma   float64   // (max − min of ChanSteps) / max(|Step|, 1e-3)
	RingCoverage float64   // fraction of the ring's weight inside the buffer
}

// CoreRingOptions are the radii (buffer px) and the gray plane of a probe.
type CoreRingOptions struct {
	Core, R1, R2 float64
	Gray         Gray
}

// ProbeCoreRing measures each point (buffer px): the coverage-weighted
// mean of the gray plane in the core disk and in the ring between R1 and R2,
// their difference (step), the 10–90 % spread of the gray plane in the ring
// (weighted quantiles), snr = step / max(spread, 1e-3), the step of each
// channel and how unequal they are (StepChroma). Pixels outside the buffer
// are left out; RingCoverage says how much of the ring was inside.
func ProbeCoreRing(ctx context.Context, b *buffer.Buffer, pts [][2]float64, o CoreRingOptions) ([]CoreRing, error) {
	if !(o.Core > 0 && o.R1 >= o.Core && o.R2 > o.R1) {
		return nil, errors.Errorf("probe radii core %g, ring [%g, %g]: need 0 < core ≤ r1 < r2", o.Core, o.R1, o.R2)
	}
	out := make([]CoreRing, len(pts))
	err := parallel.Rows(ctx, len(pts), func(k0, k1 int) {
		for k := k0; k < k1; k++ {
			out[k] = ProbePoint(ctx, b, pts[k][0], pts[k][1], o)
		}
	})
	return out, err
}

// ProbePoint is ProbeCoreRing for one point (radii not checked).
func ProbePoint(ctx context.Context, b *buffer.Buffer, x, y float64, o CoreRingOptions) CoreRing {
	core, err := measureSel(ctx, b, annulus(b, x, y, 0, o.Core), o.Gray, nil)
	if err != nil {
		return CoreRing{Outside: true}
	}
	rs := annulus(b, x, y, o.R1, o.R2)
	ring, err := measureSel(ctx, b, rs, o.Gray, []float64{0.1, 0.9})
	if err != nil {
		return CoreRing{Outside: true}
	}
	r := CoreRing{CoreMean: core.gray.L.Mean, RingMean: ring.gray.L.Mean}
	r.Step = r.CoreMean - r.RingMean
	r.Spread = ring.gray.L.Percentiles[1] - ring.gray.L.Percentiles[0]
	r.SNR = r.Step / math.Max(r.Spread, 1e-3)
	if rs.full > 0 {
		r.RingCoverage = rs.inside / rs.full
	}
	if core.chans != nil {
		lo, hi := math.Inf(1), math.Inf(-1)
		for c := range core.chans {
			d := core.chans[c] - ring.chans[c]
			r.ChanSteps = append(r.ChanSteps, d)
			lo, hi = math.Min(lo, d), math.Max(hi, d)
		}
		r.StepChroma = (hi - lo) / math.Max(math.Abs(r.Step), 1e-3)
	}
	return r
}

// Bilinear is the bilinearly interpolated value of plane p (w × h) at
// continuous buffer coordinates (x, y) (pixel i's centre is i + 0.5),
// clamped at the edges; ok is false outside the buffer.
func Bilinear(p []float32, w, h int, x, y float64) (float64, bool) {
	if !(x >= 0 && y >= 0 && x < float64(w) && y < float64(h)) {
		return 0, false
	}
	u, v := x-0.5, y-0.5
	i0, j0 := int(math.Floor(u)), int(math.Floor(v))
	fu, fv := u-float64(i0), v-float64(j0)
	cl := func(a, n int) int { return min(max(a, 0), n-1) }
	i1, j1 := cl(i0+1, w), cl(j0+1, h)
	i0, j0 = cl(i0, w), cl(j0, h)
	a := float64(p[j0*w+i0])*(1-fu) + float64(p[j0*w+i1])*fu
	c := float64(p[j1*w+i0])*(1-fu) + float64(p[j1*w+i1])*fu
	return a*(1-fv) + c*fv, true
}

// SelectionStats are weighted statistics of the gray plane (or of one
// channel) over a selection: what regions.measure's stats option reports.
type SelectionStats struct {
	Mean, Min, Max, Std, Median, P10, P90 float64
}

// StatsOf measures plane over pixels idx with weights w (the region's own
// pixels with weight 1, or its coverage on another raster): mean, median and
// percentiles as StatsWeighted computes them (min and max are its 0 and 1
// quantiles), and the weighted standard deviation.
func StatsOf(ctx context.Context, plane []float32, idx []int, w []float32, scale float64) (SelectionStats, error) {
	b := buffer.New(len(idx), 1, 1, scale)
	for j, i := range idx {
		b.Pix[j] = plane[i]
	}
	ws, err := StatsWeighted(ctx, b, w, []float64{0, 0.1, 0.9, 1})
	if err != nil {
		return SelectionStats{}, err
	}
	q := ws.L.Percentiles
	s := SelectionStats{Mean: ws.L.Mean, Median: ws.L.Median, Min: q[0], P10: q[1], P90: q[2], Max: q[3]}
	var ss float64
	for j, v := range b.Pix {
		d := float64(v) - s.Mean
		ss += float64(min(w[j], 1)) * d * d
	}
	s.Std = math.Sqrt(ss / ws.Weight)
	return s, nil
}

// GrayPlane is the gray plane of a whole buffer (its plane for one channel).
func GrayPlane(b *buffer.Buffer, g Gray) []float32 {
	if b.C == 1 {
		return b.Plane(0)
	}
	idx := make([]int, b.N())
	for i := range idx {
		idx[i] = i
	}
	return grayPlane(b, idx, g)
}
