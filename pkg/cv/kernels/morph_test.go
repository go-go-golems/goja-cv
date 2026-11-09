package kernels

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func randomPlane(rng *rand.Rand, w, h int, binary bool) []float32 {
	p := make([]float32, w*h)
	for i := range p {
		if binary {
			if rng.Float32() < 0.08 {
				p[i] = 1
			}
			continue
		}
		p[i] = rng.Float32()
	}
	return p
}

func extreme(isMax bool) (float32, func(a, b float32) float32) {
	if isMax {
		return float32(math.Inf(-1)), func(a, b float32) float32 { return max(a, b) }
	}
	return float32(math.Inf(1)), func(a, b float32) float32 { return min(a, b) }
}

// bruteDisk is the definition: the min (max) over the offsets with
// dx² + dy² ≤ r² that fall inside the plane.
func bruteDisk(p []float32, w, h int, r float64, isMax bool) []float32 {
	out := make([]float32, w*h)
	R := int(math.Ceil(r))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v, better := extreme(isMax)
			for dy := -R; dy <= R; dy++ {
				for dx := -R; dx <= R; dx++ {
					if float64(dx*dx+dy*dy) > r*r || x+dx < 0 || x+dx >= w || y+dy < 0 || y+dy >= h {
						continue
					}
					v = better(v, p[(y+dy)*w+x+dx])
				}
			}
			out[y*w+x] = v
		}
	}
	return out
}

// bruteLine walks the digital line through each pixel: the steps k = −R…R
// along x (y when steep) and the other coordinate moved by
// round((i+k)·slope) − round(i·slope).
func bruteLine(p []float32, w, h int, r, theta float64, isMax bool) []float32 {
	ux, uy := math.Cos(theta), -math.Sin(theta)
	steep := math.Abs(uy) > math.Abs(ux)
	slope, R := uy/ux, int(math.Round(r*math.Abs(ux)))
	if steep {
		slope, R = ux/uy, int(math.Round(r*math.Abs(uy)))
	}
	rnd := func(i int) int { return int(math.Round(float64(i) * slope)) }
	out := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v, better := extreme(isMax)
			for k := -R; k <= R; k++ {
				xx, yy := x+k, y+rnd(x+k)-rnd(x)
				if steep {
					xx, yy = x+rnd(y+k)-rnd(y), y+k
				}
				if xx < 0 || xx >= w || yy < 0 || yy >= h {
					continue
				}
				v = better(v, p[yy*w+xx])
			}
			out[y*w+x] = v
		}
	}
	return out
}

func samePlane(t *testing.T, what string, got, want []float32, w int) {
	t.Helper()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s at (%d, %d): %v, want %v", what, i%w, i/w, got[i], want[i])
		}
	}
}

func TestDiskMinMaxMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	w, h := 31, 23
	p := randomPlane(rng, w, h, false)
	for r := 0.5; r <= 12; r += 0.25 {
		for _, isMax := range []bool{false, true} {
			got, err := DiskMinMax(context.Background(), p, w, h, r, isMax)
			if err != nil {
				t.Fatal(err)
			}
			samePlane(t, fmt.Sprintf("disk r=%g max=%v", r, isMax), got, bruteDisk(p, w, h, r, isMax), w)
		}
	}
}

// The binary fast path (distance transform) and the chords agree for every
// radius on planes of only 0 and 1, and both match the definition.
func TestBinaryDiskEqualsChords(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	ctx := context.Background()
	w, h := 151, 113
	// a few single pixels and boxes of one value on the other: the single
	// pixels make the disk's boundary visible at every radius
	sparse := func(on float32) []float32 {
		p := make([]float32, w*h)
		for i := range p {
			p[i] = 1 - on
		}
		for k := 0; k < 4; k++ {
			p[rng.Intn(h)*w+rng.Intn(w)] = on
			x0, y0 := rng.Intn(w-8), rng.Intn(h-8)
			for y := y0; y < y0+1+rng.Intn(7); y++ {
				for x := x0; x < x0+1+rng.Intn(7); x++ {
					p[y*w+x] = on
				}
			}
		}
		return p
	}
	ones, zeros := sparse(1), sparse(0)
	for r := 8.0; r <= 40; r += 0.5 {
		for _, isMax := range []bool{false, true} {
			p := zeros // erosion grows the 0s
			if isMax {
				p = ones
			}
			fast, err := binaryDisk(ctx, p, w, h, r, isMax)
			if err != nil {
				t.Fatal(err)
			}
			chords, err := chordDisk(ctx, p, w, h, r, isMax)
			if err != nil {
				t.Fatal(err)
			}
			what := fmt.Sprintf("binary r=%g max=%v", r, isMax)
			samePlane(t, what, fast, chords, w)
			if r == 8 || r == 23.5 {
				samePlane(t, what+" (brute)", fast, bruteDisk(p, w, h, r, isMax), w)
			}
		}
	}
	// all ones and all zeros (the empty set for the distance transform)
	for _, v := range []float32{0, 1} {
		q := make([]float32, w*h)
		for i := range q {
			q[i] = v
		}
		for _, isMax := range []bool{false, true} {
			got, err := DiskMinMax(ctx, q, w, h, 10, isMax)
			if err != nil {
				t.Fatal(err)
			}
			samePlane(t, fmt.Sprintf("constant %v max=%v", v, isMax), got, q, w)
		}
	}
}

func TestLineMinMaxMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	w, h := 41, 29
	p := randomPlane(rng, w, h, false)
	for a := 0.0; a < 360; a += 22.5 {
		for _, r := range []float64{0.4, 1, 3, 7.5, 20, 60} {
			for _, isMax := range []bool{false, true} {
				th := a * math.Pi / 180
				got, err := LineMinMax(context.Background(), p, w, h, r, th, isMax)
				if err != nil {
					t.Fatal(err)
				}
				samePlane(t, fmt.Sprintf("line %g° r=%g max=%v", a, r, isMax), got, bruteLine(p, w, h, r, th, isMax), w)
			}
		}
	}
}

// erode(x) = 1 − dilate(1 − x) on [0, 1] data, for both elements.
func TestErodeDilateAreDual(t *testing.T) {
	rng := rand.New(rand.NewSource(14))
	ctx := context.Background()
	w, h := 37, 31
	p := randomPlane(rng, w, h, false)
	inv := make([]float32, len(p))
	for i, v := range p {
		inv[i] = 1 - v
	}
	type morph func(q []float32, isMax bool) ([]float32, error)
	for name, f := range map[string]morph{
		"disk 5":     func(q []float32, isMax bool) ([]float32, error) { return DiskMinMax(ctx, q, w, h, 5, isMax) },
		"line 30° 6": func(q []float32, isMax bool) ([]float32, error) { return LineMinMax(ctx, q, w, h, 6, math.Pi/6, isMax) },
	} {
		e, err := f(p, false)
		if err != nil {
			t.Fatal(err)
		}
		d, err := f(inv, true)
		if err != nil {
			t.Fatal(err)
		}
		for i := range e {
			if math.Abs(float64(e[i]-(1-d[i]))) > 1e-6 {
				t.Fatalf("%s: erode %v, 1 − dilate(1 − x) %v", name, e[i], 1-d[i])
			}
		}
	}
}

// Opening is idempotent and anti-extensive, closing idempotent and
// extensive, with the clipped elements at the border too.
func TestOpeningAndClosingLaws(t *testing.T) {
	rng := rand.New(rand.NewSource(15))
	ctx := context.Background()
	w, h := 43, 35
	p := randomPlane(rng, w, h, false)
	type morph func(q []float32, isMax bool) []float32
	must := func(v []float32, err error) []float32 {
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for name, f := range map[string]morph{
		"disk 3.5": func(q []float32, isMax bool) []float32 { return must(DiskMinMax(ctx, q, w, h, 3.5, isMax)) },
		"line 112.5°": func(q []float32, isMax bool) []float32 {
			return must(LineMinMax(ctx, q, w, h, 5, 112.5*math.Pi/180, isMax))
		},
	} {
		open := func(q []float32) []float32 { return f(f(q, false), true) }
		closeOp := func(q []float32) []float32 { return f(f(q, true), false) }
		o, c := open(p), closeOp(p)
		samePlane(t, name+" opening idempotent", open(o), o, w)
		samePlane(t, name+" closing idempotent", closeOp(c), c, w)
		for i := range p {
			if o[i] > p[i] || c[i] < p[i] {
				t.Fatalf("%s at %d: opening %v, x %v, closing %v", name, i, o[i], p[i], c[i])
			}
		}
	}
}

// A 1 px line at 30° vanishes under a disk opening of radius 3 and survives
// a line opening of radius 5 at its own angle.
func TestLineOpeningKeepsLinesDiskOpeningRemovesThem(t *testing.T) {
	ctx := context.Background()
	w, h := 64, 48
	th := 30 * math.Pi / 180
	p := make([]float32, w*h)
	off := lineOffsets(w, -math.Tan(th)) // the digital line of the element, through (0, 40)
	for x := 0; x < w; x++ {
		if y := 40 + off[x]; y >= 0 && y < h {
			p[y*w+x] = 1
		}
	}
	open := func(f func(q []float32, isMax bool) ([]float32, error)) []float32 {
		e, err := f(p, false)
		if err != nil {
			t.Fatal(err)
		}
		o, err := f(e, true)
		if err != nil {
			t.Fatal(err)
		}
		return o
	}
	disk := open(func(q []float32, isMax bool) ([]float32, error) { return DiskMinMax(ctx, q, w, h, 3, isMax) })
	line := open(func(q []float32, isMax bool) ([]float32, error) { return LineMinMax(ctx, q, w, h, 5, th, isMax) })
	for i := range p {
		if disk[i] != 0 {
			t.Fatalf("disk opening kept the line at %d", i)
		}
	}
	samePlane(t, "line opening", line, p, w)
}

// The white top-hat x − opening(x) of a flat disk of radius 4 on a ramp,
// with a disk of radius 6, is the disk at full height and zero elsewhere.
func TestTopHatOfDiskOnRamp(t *testing.T) {
	ctx := context.Background()
	w, h := 64, 48
	const slope, height = 1e-3, 0.5
	p := make([]float32, w*h)
	in := func(x, y int) bool { dx, dy := x-32, y-24; return dx*dx+dy*dy <= 16 }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p[y*w+x] = slope * float32(x)
			if in(x, y) {
				p[y*w+x] += height
			}
		}
	}
	e, err := DiskMinMax(ctx, p, w, h, 6, false)
	if err != nil {
		t.Fatal(err)
	}
	o, err := DiskMinMax(ctx, e, w, h, 6, true)
	if err != nil {
		t.Fatal(err)
	}
	for y := 12; y < h-12; y++ { // the opening of a ramp is the ramp away from the borders
		for x := 12; x < w-12; x++ {
			th := float64(p[y*w+x] - o[y*w+x])
			want := 0.0
			if in(x, y) {
				want = height
			}
			if math.Abs(th-want) > 12*slope+1e-6 {
				t.Fatalf("top-hat at (%d, %d) = %v, want %v", x, y, th, want)
			}
		}
	}
}

// The disk dilation then erosion of soft blobs (a closing) is close to the
// blobs: the replacement of the old grow∘shrink test.
func TestDilateThenErodeIsNearIdentityOnBlobs(t *testing.T) {
	ctx := context.Background()
	m := disk(80, 60, 30, 28, 9)
	ell := disk(80, 60, 60, 30, 6)
	for i := range m.Pix {
		m.Pix[i] = max(m.Pix[i], ell.Pix[i])
	}
	for _, r := range []float64{1, 2.5, 4} {
		d, err := DiskMinMax(ctx, m.Pix, m.W, m.H, r, true)
		if err != nil {
			t.Fatal(err)
		}
		c, err := DiskMinMax(ctx, d, m.W, m.H, r, false)
		if err != nil {
			t.Fatal(err)
		}
		var area0, area1, inter, union float64
		for i, v := range m.Pix {
			area0 += float64(v)
			area1 += float64(d[i])
			inter += float64(min(v, c[i]))
			union += float64(max(v, c[i]))
		}
		if inter/union < 0.97 {
			t.Fatalf("r=%g: IoU(closing, m) = %.3f", r, inter/union)
		}
		if area1 <= area0*1.1 {
			t.Fatalf("r=%g: dilation area %g, blobs %g", r, area1, area0)
		}
	}
}

func BenchmarkMorphology(b *testing.B) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(1))
	for _, n := range []int{1024, 4096} {
		grey := randomPlane(rng, n, n, false)
		bin := randomPlane(rng, n, n, true)
		for _, r := range []float64{3, 16, 64} {
			b.Run(fmt.Sprintf("disk/%d/r%g", n, r), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if _, err := DiskMinMax(ctx, grey, n, n, r, true); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run(fmt.Sprintf("binary/%d/r%g", n, r), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if _, err := DiskMinMax(ctx, bin, n, n, r, true); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run(fmt.Sprintf("line30/%d/r%g", n, r), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if _, err := LineMinMax(ctx, grey, n, n, r, math.Pi/6, true); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
