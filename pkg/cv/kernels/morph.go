package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// minMax1D is the van Herk/Gil-Werman running minimum (or maximum) over a
// centred window of k = 2r+1 samples, ignoring samples outside the array.
// It costs three comparisons per sample whatever k is. g and h are scratch
// of length len(src)+2r rounded up to a multiple of k (scratchLen).
func minMax1D(src, dst []float32, r int, isMax bool, g, h []float32) {
	n, k, m := len(src), 2*r+1, len(g)
	pad := float32(math.Inf(1))
	if isMax {
		pad = float32(math.Inf(-1))
	}
	// h holds the padded input; the backward pass overwrites it in place
	for j := range r {
		h[j] = pad
	}
	copy(h[r:r+n], src)
	for j := r + n; j < m; j++ {
		h[j] = pad
	}
	if isMax {
		for b0 := 0; b0 < m; b0 += k {
			e := min(b0+k, m)
			acc := h[b0]
			g[b0] = acc
			for j := b0 + 1; j < e; j++ {
				acc = max(acc, h[j])
				g[j] = acc
			}
			acc = h[e-1]
			for j := e - 2; j >= b0; j-- {
				acc = max(acc, h[j])
				h[j] = acc
			}
		}
		for i := 0; i < n; i++ { // window [i, i+k-1] in padded coordinates
			dst[i] = max(h[i], g[i+k-1])
		}
		return
	}
	for b0 := 0; b0 < m; b0 += k {
		e := min(b0+k, m)
		acc := h[b0]
		g[b0] = acc
		for j := b0 + 1; j < e; j++ {
			acc = min(acc, h[j])
			g[j] = acc
		}
		acc = h[e-1]
		for j := e - 2; j >= b0; j-- {
			acc = min(acc, h[j])
			h[j] = acc
		}
	}
	for i := 0; i < n; i++ {
		dst[i] = min(h[i], g[i+k-1])
	}
}

func scratchLen(n, r int) int {
	k := 2*r + 1
	l := n + 2*r
	return (l + k - 1) / k * k
}

// squareMinMax filters a w×h plane with a (2r+1)² square minimum (erosion)
// or maximum (dilation), separably. The speck background uses it.
func squareMinMax(ctx context.Context, p []float32, w, h, r int, isMax bool) ([]float32, error) {
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

// squareOpening is erosion then dilation with a (2r+1)² square: it removes
// bright structures smaller than the square.
func squareOpening(ctx context.Context, p []float32, w, h, r int) ([]float32, error) {
	e, err := squareMinMax(ctx, p, w, h, r, false)
	if err != nil {
		return nil, err
	}
	return squareMinMax(ctx, e, w, h, r, true)
}

// squareClosing is dilation then erosion: it removes dark structures
// smaller than the square.
func squareClosing(ctx context.Context, p []float32, w, h, r int) ([]float32, error) {
	d, err := squareMinMax(ctx, p, w, h, r, true)
	if err != nil {
		return nil, err
	}
	return squareMinMax(ctx, d, w, h, r, false)
}

// binaryDiskMin is the radius from which DiskMinMax takes the distance
// transform for a plane of only 0 and 1: below it the chords are cheaper.
const binaryDiskMin = 8

// DiskMinMax is the grey erosion (isMax false) or dilation (isMax true) of a
// w×h plane by the disk {(dx, dy) : dx² + dy² ≤ r²}: each output pixel is
// the minimum (maximum) of the input over the offsets of the disk that fall
// inside the plane (pixels outside take no part). The disk is cut into
// horizontal chords, each a van Herk running extremum; a plane of only 0
// and 1 with r ≥ 8 takes the exact distance transform instead (same result).
func DiskMinMax(ctx context.Context, p []float32, w, h int, r float64, isMax bool) ([]float32, error) {
	if r < 1 { // the disk is the origin alone
		return append([]float32(nil), p...), nil
	}
	if r >= binaryDiskMin && isBinary(p) {
		return binaryDisk(ctx, p, w, h, r, isMax)
	}
	return chordDisk(ctx, p, w, h, r, isMax, parallel.Rows)
}

// rowRunner runs fn over bands of [0, n): parallel.Rows, or serialRows
// inside work that is already parallel (a band of peaks).
type rowRunner func(ctx context.Context, n int, fn func(y0, y1 int)) error

// serialRows calls fn(0, n) on the caller's goroutine.
func serialRows(ctx context.Context, n int, fn func(y0, y1 int)) error {
	if n > 0 {
		fn(0, n)
	}
	return ctx.Err()
}

func isBinary(p []float32) bool {
	for _, v := range p {
		if v != 0 && v != 1 {
			return false
		}
	}
	return true
}

// diskChords returns the half-width of the disk's chord at each |dy| from
// 0 to ⌊r⌋: the largest a with a² + dy² ≤ r² (exact in float64).
func diskChords(r float64) []int {
	r2 := r * r
	half := make([]int, int(math.Floor(r))+1)
	for d := range half {
		a := int(math.Sqrt(max(0, r2-float64(d*d))))
		for a > 0 && float64(a*a+d*d) > r2 {
			a--
		}
		for float64((a+1)*(a+1)+d*d) <= r2 {
			a++
		}
		half[d] = a
	}
	return half
}

// chordDisk is DiskMinMax by chords. The chord half-width depends only on
// |dy| and does not increase with it, so the rows are filtered once per
// distinct half-width and every output row combines input rows y ± dy of
// that width: about (number of widths) running extrema and (2⌊r⌋ + 1)
// combinations per pixel.
func chordDisk(ctx context.Context, p []float32, w, h int, r float64, isMax bool, rows rowRunner) ([]float32, error) {
	half := diskChords(r)
	pad := float32(math.Inf(1))
	if isMax {
		pad = float32(math.Inf(-1))
	}
	out := make([]float32, w*h)
	for i := range out {
		out[i] = pad
	}
	run := make([]float32, w*h)
	for d0 := 0; d0 < len(half); {
		a := half[d0]
		d1 := d0
		for d1+1 < len(half) && half[d1+1] == a {
			d1++
		}
		err := rows(ctx, h, func(y0, y1 int) {
			sl := scratchLen(w, a)
			g, hh := make([]float32, sl), make([]float32, sl)
			for y := y0; y < y1; y++ {
				minMax1D(p[y*w:(y+1)*w], run[y*w:(y+1)*w], a, isMax, g, hh)
			}
		})
		if err != nil {
			return nil, err
		}
		err = rows(ctx, h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				o := out[y*w : (y+1)*w]
				for d := d0; d <= d1; d++ {
					for _, yy := range [2]int{y - d, y + d} {
						if yy < 0 || yy >= h || (d == 0 && yy != y) {
							continue
						}
						combineRow(o, run[yy*w:(yy+1)*w], isMax)
					}
				}
			}
		})
		if err != nil {
			return nil, err
		}
		d0 = d1 + 1
	}
	return out, nil
}

// combineRow sets o to the pointwise maximum (minimum) of o and row.
func combineRow(o, row []float32, isMax bool) {
	row = row[:len(o)]
	if isMax {
		for x, v := range row {
			if v > o[x] {
				o[x] = v
			}
		}
		return
	}
	for x, v := range row {
		if v < o[x] {
			o[x] = v
		}
	}
}

// binaryDisk is DiskMinMax for a plane of only 0 and 1, from the exact
// squared distance to the 1s (dilation: a 1 within r) or to the 0s
// (erosion: no 0 within r). Squared distances are integers, so the
// comparison with r² selects the same offsets as the chords.
func binaryDisk(ctx context.Context, p []float32, w, h int, r float64, isMax bool) ([]float32, error) {
	set := make([]bool, len(p))
	for i, v := range p {
		set[i] = (v == 1) == isMax
	}
	d2, err := squaredDistanceToSet(ctx, set, w, h)
	if err != nil {
		return nil, err
	}
	r2 := r * r
	out := make([]float32, len(p))
	for i, v := range d2 {
		if (v <= r2) == isMax {
			out[i] = 1
		}
	}
	return out, nil
}

// LineMinMax is the grey erosion (isMax false) or dilation (isMax true) of a
// w×h plane by the line element of radius r at angle theta (radians,
// counter-clockwise on screen from the x axis). The plane is cut into the
// digital lines of that direction, y = c + round(x·slope) when the line is
// mostly horizontal (x and y exchanged when it is steep); every pixel lies
// on one of them, and its element is the run of 2R + 1 samples of its own
// line centred on it, R = round(r·|cos|) (|sin| when steep), so the end
// centres are about 2r apart. Van Herk along each line: three comparisons
// per pixel whatever r is. Samples outside the plane take no part.
func LineMinMax(ctx context.Context, p []float32, w, h int, r, theta float64, isMax bool) ([]float32, error) {
	ux, uy := math.Cos(theta), -math.Sin(theta) // y grows downwards
	if math.Abs(ux) >= math.Abs(uy) {
		return digitalLines(ctx, p, w, h, uy/ux, int(math.Round(r*math.Abs(ux))), false, isMax)
	}
	return digitalLines(ctx, p, w, h, ux/uy, int(math.Round(r*math.Abs(uy))), true, isMax)
}

// lineOffsets is round(i·slope) for every step i along a line of n samples.
func lineOffsets(n int, slope float64) []int {
	off := make([]int, n)
	for i := range off {
		off[i] = int(math.Round(float64(i) * slope))
	}
	return off
}

// digitalLines runs minMax1D with half-width R along every line of the
// family other = c + round(along·slope); along is x (steep: y).
func digitalLines(ctx context.Context, p []float32, w, h int, slope float64, R int, steep, isMax bool) ([]float32, error) {
	if R <= 0 {
		return append([]float32(nil), p...), nil
	}
	n, m := w, h // samples along a line; extent of the other coordinate
	if steep {
		n, m = h, w
	}
	off := lineOffsets(n, slope)
	lo, hi := 0, 0
	for _, o := range off {
		lo, hi = min(lo, o), max(hi, o)
	}
	at := func(i, c int) int { // pixel index of step i on the line with other = c
		if steep {
			return i*w + c
		}
		return c*w + i
	}
	out := make([]float32, w*h)
	// lines c = -hi … m-1-lo meet the plane; off is monotone, so each
	// line's samples inside it are one run of steps
	err := parallel.Rows(ctx, m-lo+hi, func(l0, l1 int) {
		sl := scratchLen(n, R)
		g, hh := make([]float32, sl), make([]float32, sl)
		seq, res := make([]float32, n), make([]float32, n)
		for l := l0; l < l1; l++ {
			c0 := l - hi
			first, k := -1, 0
			for i := 0; i < n; i++ {
				c := c0 + off[i]
				if c < 0 || c >= m {
					if k > 0 {
						break
					}
					continue
				}
				if first < 0 {
					first = i
				}
				seq[k] = p[at(i, c)]
				k++
			}
			if k == 0 {
				continue
			}
			s := scratchLen(k, R)
			minMax1D(seq[:k], res[:k], R, isMax, g[:s], hh[:s])
			for j := 0; j < k; j++ {
				i := first + j
				out[at(i, c0+off[i])] = res[j]
			}
		}
	})
	return out, err
}
