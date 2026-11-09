package kernels

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

func randomBuffer(w, h, c int, seed int64) *buffer.Buffer {
	rng := rand.New(rand.NewSource(seed))
	b := buffer.New(w, h, c, 1)
	for i := range b.Pix {
		b.Pix[i] = float32(rng.Float64())
	}
	return b
}

// TestEllipseCoverageIsEllipseMask: the mask kernel and the probe weights
// are the same function, bit for bit.
func TestEllipseCoverageIsEllipseMask(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for k := 0; k < 50; k++ {
		x, y := rng.Float64()*40, rng.Float64()*30
		bw, bh := 1+rng.Float64()*20, 1+rng.Float64()*20
		m, err := EllipseMask(context.Background(), 48, 40, 1, x, y, bw, bh)
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 40; j++ {
			for i := 0; i < 48; i++ {
				if got := EllipseCoverage(i, j, x+bw/2, y+bh/2, bw/2, bh/2); got != m.Pix[j*48+i] {
					t.Fatalf("ellipse %d at (%d, %d): coverage %v, mask %v", k, i, j, got, m.Pix[j*48+i])
				}
			}
		}
	}
}

// ellipseWeights is mask.ellipse of the disk of radius r around (cx, cy)
// over the whole buffer, as the helper built it.
func ellipseWeights(t *testing.T, b *buffer.Buffer, cx, cy, r float64) []float32 {
	t.Helper()
	m, err := EllipseMask(context.Background(), b.W, b.H, 1, cx-r, cy-r, 2*r, 2*r)
	if err != nil {
		t.Fatal(err)
	}
	return m.Plane(0)
}

// TestProbeEqualsStatsWeighted: core and ring means, channel steps and the
// ring spread equal StatsWeighted over whole-image ellipse masks (the
// colleague's mem.probe), also next to the edge.
func TestProbeEqualsStatsWeighted(t *testing.T) {
	ctx := context.Background()
	b := randomBuffer(64, 48, 3, 1)
	pts := [][2]float64{{20.5, 20.5}, {31.25, 17.8}, {2.5, 3.5}, {63.2, 47.9}, {40, 24}}
	core, r1, r2 := 3.5, 6.5, 12.5
	res, err := ProbeCoreRing(ctx, b, pts, CoreRingOptions{Core: core, R1: r1, R2: r2})
	if err != nil {
		t.Fatal(err)
	}
	for k, p := range pts {
		cw := ellipseWeights(t, b, p[0], p[1], core)
		outer, inner := ellipseWeights(t, b, p[0], p[1], r2), ellipseWeights(t, b, p[0], p[1], r1)
		rw := make([]float32, len(outer))
		for i := range rw {
			rw[i] = min(outer[i], 1-inner[i]) // mask.minus
		}
		c, err := StatsWeighted(ctx, b, cw, nil)
		if err != nil {
			t.Fatal(err)
		}
		g, err := StatsWeighted(ctx, b, rw, []float64{0.1, 0.9})
		if err != nil {
			t.Fatal(err)
		}
		r := res[k]
		near := func(what string, got, want float64) {
			if math.Abs(got-want) > 1e-9 {
				t.Errorf("point %v: %s = %.12g, stats %.12g", p, what, got, want)
			}
		}
		near("coreMean", r.CoreMean, c.L.Mean)
		near("ringMean", r.RingMean, g.L.Mean)
		near("spread", r.Spread, g.L.Percentiles[1]-g.L.Percentiles[0])
		for ch := 0; ch < 3; ch++ {
			near("step", r.ChanSteps[ch], c.Channels[ch].Mean-g.Channels[ch].Mean)
		}
		if k >= 2 && k <= 3 && !(r.RingCoverage < 0.7) {
			t.Errorf("point %v next to the corner: ringCoverage %g", p, r.RingCoverage)
		}
		if k == 0 && r.RingCoverage != 1 {
			t.Errorf("point %v: ringCoverage %g, want 1", p, r.RingCoverage)
		}
	}
	if out, _ := ProbeCoreRing(ctx, b, [][2]float64{{-30, 10}}, CoreRingOptions{Core: core, R1: r1, R2: r2}); !out[0].Outside {
		t.Fatalf("a point 30 px outside is %+v", out[0])
	}
}

func TestSampleRadiusZeroIsThePixel(t *testing.T) {
	b := randomBuffer(16, 12, 3, 2)
	s, err := SamplePoints(context.Background(), b, [][2]float64{{5.5, 7.5}, {5.99, 7.01}, {16, 3}}, 0, GrayRec709)
	if err != nil {
		t.Fatal(err)
	}
	i := 7*16 + 5
	for k := 0; k < 2; k++ {
		for c := 0; c < 3; c++ {
			if s[k].Chans[c] != float64(b.Plane(c)[i]) {
				t.Fatalf("sample %d channel %d = %v, pixel %v", k, c, s[k].Chans[c], b.Plane(c)[i])
			}
		}
		if want := float64(b.Luminance()[i]); s[k].Gray != want || s[k].Weight != 1 {
			t.Fatalf("sample %d gray %v weight %v, want %v", k, s[k].Gray, s[k].Weight, want)
		}
	}
	if !s[2].Outside {
		t.Fatal("x = 16 is outside a 16 px wide buffer")
	}
	d, _ := SamplePoints(context.Background(), b, [][2]float64{{8, 6}}, 2, GrayRec709)
	if math.Abs(d[0].Weight-math.Pi*4) > 0.6 {
		t.Fatalf("disk of radius 2 weighs %g, want about %g", d[0].Weight, math.Pi*4)
	}
}

// TestBilinearProfileRecoversEdgeSigma: the 10–90 % rise of a profile
// across a blurred edge of known σ gives σ back within 10 %.
func TestBilinearProfileRecoversEdgeSigma(t *testing.T) {
	const w, h, sigma, edge = 80, 8, 1.6, 40.0
	b := buffer.New(w, h, 1, 1)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			b.Pix[y*w+x] = float32(0.2 + 0.6*0.5*(1+math.Erf((float64(x)+0.5-edge)/(sigma*math.Sqrt2))))
		}
	}
	var t10, t90 float64
	prev := 0.0
	for k := 0; k <= 400; k++ {
		x := 30 + float64(k)*0.05
		v, ok := Bilinear(b.Pix, w, h, x, 4)
		if !ok {
			t.Fatalf("x = %g outside", x)
		}
		f := (v - 0.2) / 0.6
		if prev < 0.1 && f >= 0.1 {
			t10 = x
		}
		if prev < 0.9 && f >= 0.9 {
			t90 = x
		}
		prev = f
	}
	got := (t90 - t10) / 2.5631 // 10–90 % rise of an erf edge is 2.5631 σ
	if math.Abs(got-sigma)/sigma > 0.1 {
		t.Fatalf("recovered σ %g, want %g ± 10 %%", got, sigma)
	}
}
