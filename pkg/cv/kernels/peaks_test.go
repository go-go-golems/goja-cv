package kernels

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// blob is a Gaussian spot of the synthetic fields (buffer px; amp < 0 is dark).
type blob struct{ x, y, sigma, amp float64 }

// blobField is a w×h one-channel buffer: base, Gaussian noise of σ noise,
// plus the blobs sampled at pixel centres.
func blobField(rng *rand.Rand, w, h int, base, noise float64, blobs []blob) *buffer.Buffer {
	b := buffer.New(w, h, 1, 1)
	p := b.Plane(0)
	for i := range p {
		p[i] = float32(base + noise*rng.NormFloat64())
	}
	for _, s := range blobs {
		R := 10 * s.sigma
		for y := max(0, int(s.y-R)); y < min(h, int(s.y+R)+1); y++ {
			for x := max(0, int(s.x-R)); x < min(w, int(s.x+R)+1); x++ {
				dx, dy := float64(x)+0.5-s.x, float64(y)+0.5-s.y
				if d2 := dx*dx + dy*dy; d2 < R*R {
					p[y*w+x] += float32(s.amp * math.Exp(-d2/(2*s.sigma*s.sigma)))
				}
			}
		}
	}
	return b
}

// gridBlobs are blobs on a grid of cells of side cell, jittered by up to a
// quarter cell, with σ in [1, 3] and amplitude in [0.05, 0.5] (sign: dark
// blobs have negative amplitude).
func gridBlobs(rng *rand.Rand, w, h, cell int, sign float64) []blob {
	var out []blob
	for cy := cell / 2; cy+cell/2 <= h; cy += cell {
		for cx := cell / 2; cx+cell/2 <= w; cx += cell {
			j := float64(cell) / 4
			out = append(out, blob{
				x: float64(cx) + (rng.Float64()*2-1)*j, y: float64(cy) + (rng.Float64()*2-1)*j,
				sigma: 1 + 2*rng.Float64(), amp: sign * (0.05 + 0.45*rng.Float64()),
			})
		}
	}
	return out
}

// bruteStep is the step map by definition: the mean over the in-image
// offsets with dx² + dy² ≤ r² minus the mean over those with
// (1.5 r)² < dx² + dy² ≤ (2.5 r)².
func bruteStep(p []float32, w, h int, r float64) []float64 {
	out := make([]float64, w*h)
	R := int(math.Ceil(2.5 * r))
	for y := range h {
		for x := range w {
			var sc, nc, sr, nr float64
			for dy := -R; dy <= R; dy++ {
				for dx := -R; dx <= R; dx++ {
					xx, yy := x+dx, y+dy
					if xx < 0 || yy < 0 || xx >= w || yy >= h {
						continue
					}
					d2, v := float64(dx*dx+dy*dy), float64(p[yy*w+xx])
					switch {
					case d2 <= r*r:
						sc, nc = sc+v, nc+1
					case d2 > 2.25*r*r && d2 <= 6.25*r*r:
						sr, nr = sr+v, nr+1
					}
				}
			}
			if nr > 0 {
				out[y*w+x] = sc/nc - sr/nr
			}
		}
	}
	return out
}

func TestStepMapMatchesBruteForce(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(3))
	w, h := 53, 37
	p := randomPlane(rng, w, h, false)
	for _, r := range []float64{1, 1.5, 2, 2.75, 3.3, 5, 8} {
		want := bruteStep(p, w, h, r)
		for _, rows := range [][2]int{{0, h}, {5, 20}, {30, h}} {
			got, err := stepRows(ctx, parallel.Rows, p, w, h, newPeakRadius(r), rows[0], rows[1])
			if err != nil {
				t.Fatal(err)
			}
			for j := range rows[1] - rows[0] {
				for x := range w {
					y := rows[0] + j
					if d := math.Abs(float64(got[j*w+x]) - want[y*w+x]); d > 1e-6 {
						t.Fatalf("r=%g rows %v: step at (%d, %d) = %g, want %g", r, rows, x, y, got[j*w+x], want[y*w+x])
					}
				}
			}
		}
	}
}

// nearest is the peak closest to (x, y) and its distance.
func nearest(ps []Peak, x, y float64) (Peak, float64) {
	best, bd := Peak{}, math.Inf(1)
	for _, p := range ps {
		if d := math.Hypot(p.CX-x, p.CY-y); d < bd {
			best, bd = p, d
		}
	}
	return best, bd
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s) == 0 {
		return math.NaN()
	}
	return s[len(s)/2]
}

// TestPeaksFindGaussianBlobs: every blob of a noisy flat field is found;
// at snr ≥ 5 the centre error is below 0.25 px (median) and 0.5 px (max).
func TestPeaksFindGaussianBlobs(t *testing.T) {
	ctx := context.Background()
	for seed := int64(1); seed <= 3; seed++ {
		rng := rand.New(rand.NewSource(seed))
		blobs := gridBlobs(rng, 320, 320, 40, 1)
		b := blobField(rng, 320, 320, 0.2, 0.01, blobs)
		ps, err := Peaks(ctx, b, PeakOptions{Radii: []float64{2, 3, 4}, MinSNR: 1})
		if err != nil {
			t.Fatal(err)
		}
		var errs []float64
		for _, s := range blobs {
			p, d := nearest(ps, s.x, s.y)
			if d > 1.5 {
				t.Errorf("seed %d: blob at (%.2f, %.2f) σ %.2f amp %.3f: nearest peak %.2f px away", seed, s.x, s.y, s.sigma, s.amp, d)
				continue
			}
			if p.CoreRing.SNR >= 5 {
				errs = append(errs, d)
			}
		}
		sort.Float64s(errs)
		t.Logf("seed %d: %d blobs, %d peaks, %d at snr ≥ 5: centre error median %.3f px, max %.3f px", seed, len(blobs), len(ps), len(errs), median(errs), errs[len(errs)-1])
		if len(errs) < len(blobs)/4 {
			t.Errorf("seed %d: only %d of %d blobs at snr ≥ 5", seed, len(errs), len(blobs))
		}
		if median(errs) >= 0.25 || errs[len(errs)-1] >= 0.5 {
			t.Errorf("seed %d: centre error median %.3f px, max %.3f px; want < 0.25 and < 0.5", seed, median(errs), errs[len(errs)-1])
		}
	}
}

// TestPeaksPrefilterKeepsEveryPeak: the digital-ring pre-filter never drops
// a peak that the exact measure keeps.
func TestPeaksPrefilterKeepsEveryPeak(t *testing.T) {
	ctx := context.Background()
	for seed := int64(1); seed <= 3; seed++ {
		rng := rand.New(rand.NewSource(seed))
		noise := []float64{0.005, 0.02, 0.05}[seed-1]
		b := blobField(rng, 256, 256, 0.2, noise, gridBlobs(rng, 256, 256, 32, 1))
		for _, snr := range []float64{0.5, 1, 1.4, 2} {
			o := PeakOptions{Radii: []float64{1.5, 2, 4, 8}, MinSNR: snr, Polarity: "both"}
			with, err := Peaks(ctx, b, o)
			if err != nil {
				t.Fatal(err)
			}
			o.noPrefilter = true
			without, err := Peaks(ctx, b, o)
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(with) != fmt.Sprint(without) {
				t.Errorf("seed %d snr %g: %d peaks with the pre-filter, %d without", seed, snr, len(with), len(without))
			}
		}
	}
}

func TestPeaksPolarity(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(7))
	bright := []blob{{40.3, 40.7, 1.5, 0.3}}
	dark := []blob{{100.6, 60.2, 1.5, -0.3}}
	b := blobField(rng, 160, 120, 0.5, 0.01, append(bright, dark...))
	find := func(pol string) []Peak {
		ps, err := Peaks(ctx, b, PeakOptions{Radii: []float64{2}, MinSNR: 3, Polarity: pol})
		if err != nil {
			t.Fatal(err)
		}
		return ps
	}
	one := func(ps []Peak, s blob, sign float64, what string) {
		p, d := nearest(ps, s.x, s.y)
		if d > 0.5 || sign*p.CoreRing.Step <= 0 || sign*p.CoreRing.SNR < 3 {
			t.Errorf("%s: nearest peak %.2f px away, step %g, snr %g", what, d, p.CoreRing.Step, p.CoreRing.SNR)
		}
	}
	if ps := find("bright"); len(ps) != 1 {
		t.Errorf("bright: %d peaks, want 1", len(ps))
	} else {
		one(ps, bright[0], 1, "bright")
	}
	if ps := find("dark"); len(ps) != 1 {
		t.Errorf("dark: %d peaks, want 1", len(ps))
	} else {
		one(ps, dark[0], -1, "dark")
	}
	ps := find("both")
	if len(ps) != 2 {
		t.Fatalf("both: %d peaks, want 2", len(ps))
	}
	one(ps, bright[0], 1, "both/bright")
	one(ps, dark[0], -1, "both/dark")
}

// flatDisk is a w×h field with a disk of radius rad and height v at (cx, cy)
// on a faint noise (a saturated blob).
func flatDisk(rng *rand.Rand, w, h int, cx, cy, rad, v float64) *buffer.Buffer {
	b := buffer.New(w, h, 1, 1)
	p := b.Plane(0)
	for y := range h {
		for x := range w {
			p[y*w+x] = float32(0.1 + 0.005*rng.NormFloat64())
			if math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) <= rad {
				p[y*w+x] = float32(v)
			}
		}
	}
	return b
}

// TestPeaksSaturatedBlob: a flat disk of radius 10 is one peak at its centre
// at radius 8 when radii [2, 4, 8] are searched (its top is flat, so the
// step map has a plateau of about 2 px around the centre), and a ring of rim
// peaks at radius 2, whose rings hold blob and background (low snr: they
// are kept with snr 0).
func TestPeaksSaturatedBlob(t *testing.T) {
	ctx := context.Background()
	b := flatDisk(rand.New(rand.NewSource(2)), 96, 96, 48, 48, 10, 1)
	ps, err := Peaks(ctx, b, PeakOptions{Radii: []float64{2, 4, 8}, MinSNR: 1})
	if err != nil {
		t.Fatal(err)
	}
	var near []Peak
	for _, p := range ps {
		if math.Hypot(p.CX-48, p.CY-48) < 16 {
			near = append(near, p)
		}
	}
	if len(near) != 1 || near[0].R != 8 || math.Hypot(near[0].CX-48, near[0].CY-48) > 1.5 {
		t.Errorf("radii [2, 4, 8]: peaks near the disk %+v; want one at (48, 48) with r = 8", near)
	}
	small, err := Peaks(ctx, b, PeakOptions{Radii: []float64{2}, MinSNR: 0})
	if err != nil {
		t.Fatal(err)
	}
	rim, rimSNR := 0, 0.0
	for _, p := range small {
		d := math.Hypot(p.CX-48, p.CY-48)
		if d < 5 {
			t.Errorf("radius 2: a peak %.1f px from the centre of a flat disk", d)
		}
		if d >= 5 && d <= 11 {
			rim++
			rimSNR = max(rimSNR, p.CoreRing.SNR)
		}
	}
	if rim < 6 {
		t.Errorf("radius 2: %d rim peaks, want a ring (≥ 6)", rim)
	}
	if len(near) == 1 && rimSNR >= near[0].CoreRing.SNR {
		t.Errorf("rim peaks reach snr %.2f, the blob's peak %.2f", rimSNR, near[0].CoreRing.SNR)
	}
	t.Logf("flat disk r=10: radii [2, 4, 8] → %d peak(s) near it; radius 2 → %d rim peaks (snr ≤ %.2f)", len(near), rim, rimSNR)
}

// TestPeaksLocalNoise: the same blob has a higher snr on a quiet dark
// background than on a noisy bright one (the snr is local).
func TestPeaksLocalNoise(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(5))
	w, h := 200, 100
	b := buffer.New(w, h, 1, 1)
	p := b.Plane(0)
	for y := range h {
		for x := range w {
			v := 0.05 + 0.003*rng.NormFloat64() // quiet and dark
			if x >= w/2 {
				v = 0.6 + 0.03*rng.NormFloat64() // noisy and bright
			}
			for _, cx := range []float64{50.4, 150.4} {
				dx, dy := float64(x)+0.5-cx, float64(y)+0.5-50.6
				v += 0.1 * math.Exp(-(dx*dx+dy*dy)/(2*1.5*1.5))
			}
			p[y*w+x] = float32(v)
		}
	}
	ps, err := Peaks(ctx, b, PeakOptions{Radii: []float64{2}, MinSNR: 0})
	if err != nil {
		t.Fatal(err)
	}
	dark, d1 := nearest(ps, 50.4, 50.6)
	bright, d2 := nearest(ps, 150.4, 50.6)
	if d1 > 0.5 || d2 > 1.5 {
		t.Fatalf("blobs not found: %.2f, %.2f px", d1, d2)
	}
	t.Logf("same blob: snr %.2f on a quiet dark background, %.2f on a noisy bright one", dark.CoreRing.SNR, bright.CoreRing.SNR)
	if dark.CoreRing.SNR <= 2*bright.CoreRing.SNR {
		t.Errorf("snr %.2f (quiet, dark) not well above %.2f (noisy, bright)", dark.CoreRing.SNR, bright.CoreRing.SNR)
	}
}

// TestPeaksBandInvariant: bands of 64 rows, of 7 rows and one band give the
// same peaks; peaks are sorted by |snr| and never overlap.
func TestPeaksBandInvariant(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(11))
	b := blobField(rng, 200, 230, 0.3, 0.02, gridBlobs(rng, 200, 230, 25, 1))
	var ref string
	for _, band := range []int{64, 7, 230} {
		ps, err := Peaks(ctx, b, PeakOptions{Radii: []float64{2, 4, 8}, MinSNR: 0.5, Polarity: "both", band: band})
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprint(ps)
		if ref == "" {
			ref = got
			for i, p := range ps {
				if i > 0 && math.Abs(p.CoreRing.SNR) > math.Abs(ps[i-1].CoreRing.SNR) {
					t.Fatalf("peak %d has |snr| %g above peak %d's %g", i, p.CoreRing.SNR, i-1, ps[i-1].CoreRing.SNR)
				}
				for _, q := range ps[:i] {
					if math.Hypot(p.CX-q.CX, p.CY-q.CY) < p.R+q.R {
						t.Fatalf("peaks %+v and %+v overlap", p, q)
					}
				}
			}
			t.Logf("%d peaks", len(ps))
			continue
		}
		if got != ref {
			t.Errorf("band %d gives different peaks than band 64", band)
		}
	}
}

func BenchmarkPeaks(b *testing.B) {
	ctx := context.Background()
	for _, n := range []int{1024, 4096} {
		rng := rand.New(rand.NewSource(1))
		buf := blobField(rng, n, n, 0.2, 0.02, gridBlobs(rng, n, n, 32, 1))
		for _, radii := range [][]float64{{3}, {2, 4, 8}} {
			b.Run(fmt.Sprintf("%d/r%v", n, radii), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if _, err := Peaks(ctx, buf, PeakOptions{Radii: radii, MinSNR: 1}); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
