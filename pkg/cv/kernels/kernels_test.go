package kernels

import (
	"context"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

var ctx = context.Background()

func randBuf(w, h, c int, seed int64) *buffer.Buffer {
	r := rand.New(rand.NewSource(seed))
	b := buffer.New(w, h, c, 1)
	for i := range b.Pix {
		b.Pix[i] = r.Float32()
	}
	return b
}

func mean(p []float32) float64 {
	var s float64
	for _, v := range p {
		s += float64(v)
	}
	return s / float64(len(p))
}

func TestExposurePlusOneEVDoubles(t *testing.T) {
	in := randBuf(17, 9, 3, 1)
	out, err := Exposure(ctx, in, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range in.Pix {
		if out.Pix[i] != 2*in.Pix[i] {
			t.Fatalf("pixel %d: %g -> %g", i, in.Pix[i], out.Pix[i])
		}
	}
	half, _ := Exposure(ctx, in, -0.5)
	if d := math.Abs(float64(half.Pix[3]) - float64(in.Pix[3])/math.Sqrt2); d > 1e-6 {
		t.Fatalf("-0.5 EV off by %g", d)
	}
}

func TestGaussianPreservesMean(t *testing.T) {
	flat := buffer.New(31, 23, 1, 1)
	for i := range flat.Pix {
		flat.Pix[i] = 0.4
	}
	out, err := Gaussian(ctx, flat, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range out.Pix {
		if math.Abs(float64(v)-0.4) > 1e-6 {
			t.Fatalf("flat image changed at %d: %g", i, v)
		}
	}
	in := randBuf(64, 48, 3, 2)
	out, err = Gaussian(ctx, in, 2)
	if err != nil {
		t.Fatal(err)
	}
	for c := 0; c < 3; c++ {
		if d := math.Abs(mean(out.Plane(c)) - mean(in.Plane(c))); d > 2e-3 {
			t.Fatalf("channel %d mean drift %g", c, d)
		}
	}
	// impulse: energy is conserved and spread symmetrically
	imp := buffer.New(41, 41, 1, 1)
	imp.Pix[20*41+20] = 1
	out, _ = Gaussian(ctx, imp, 2.5)
	if d := math.Abs(mean(out.Pix)*41*41 - 1); d > 1e-5 {
		t.Fatalf("impulse energy off by %g", d)
	}
	if out.Pix[20*41+17] != out.Pix[20*41+23] || out.Pix[17*41+20] != out.Pix[23*41+20] {
		t.Fatal("impulse response not symmetric")
	}
	if k := GaussianKernel(2); len(k) != 13 {
		t.Fatalf("kernel half-width: %d taps", len(k))
	}
}

func TestLevelsMapping(t *testing.T) {
	black, white := 0.02, 0.6
	in := buffer.New(5, 1, 1, 1)
	copy(in.Pix, []float32{0, float32(black), float32(white), 0.8, buffer.ToLinear(0.5)})
	out, err := Levels(ctx, in, black, white, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{0, 0, 1}
	for i, w := range want {
		if math.Abs(float64(out.Pix[i])-w) > 1e-4 {
			t.Fatalf("levels[%d] = %g, want %g", i, out.Pix[i], w)
		}
	}
	if out.Pix[3] <= 1 {
		t.Fatalf("values above white must stay above 1, got %g", out.Pix[3])
	}
	// identity args leave values (almost) unchanged
	id, _ := Levels(ctx, in, 0, 1, 1)
	for i := range in.Pix {
		if math.Abs(float64(id.Pix[i]-in.Pix[i])) > 5e-5 {
			t.Fatalf("identity levels changed %d: %g -> %g", i, in.Pix[i], id.Pix[i])
		}
	}
	// gamma 2 lifts perceptual mid-grey to sqrt(0.5)
	g, _ := Levels(ctx, in, 0, 1, 2)
	if d := math.Abs(float64(buffer.ToPerceptual(g.Pix[4])) - math.Sqrt(0.5)); d > 1e-4 {
		t.Fatalf("gamma 2: perceptual %g", buffer.ToPerceptual(g.Pix[4]))
	}
}

func TestContrastAndGray(t *testing.T) {
	in := buffer.New(3, 1, 3, 1)
	for c := 0; c < 3; c++ {
		copy(in.Plane(c), []float32{buffer.ToLinear(0.25), buffer.ToLinear(0.5), buffer.ToLinear(0.75)})
	}
	out, _ := Contrast(ctx, in, 0.5)
	got := []float32{buffer.ToPerceptual(out.Pix[0]), buffer.ToPerceptual(out.Pix[1]), buffer.ToPerceptual(out.Pix[2])}
	for i, w := range []float32{0.125, 0.5, 0.875} {
		if math.Abs(float64(got[i]-w)) > 1e-4 {
			t.Fatalf("contrast %d: %g want %g", i, got[i], w)
		}
	}
	g, _ := ToGray(ctx, in, GrayWeights["rec709"])
	if g.C != 1 || math.Abs(float64(g.Pix[1]-in.Pix[1])) > 1e-6 {
		t.Fatalf("gray of neutral pixel: %v %g", g, g.Pix[1])
	}
}

func TestStatsOnKnownData(t *testing.T) {
	// 101 gray samples 0, 0.01, …, 1 (shuffled)
	b := buffer.New(101, 1, 1, 1)
	for i := range b.Pix {
		b.Pix[i] = float32(i) / 100
	}
	rand.New(rand.NewSource(3)).Shuffle(len(b.Pix), func(i, j int) { b.Pix[i], b.Pix[j] = b.Pix[j], b.Pix[i] })
	s, err := Stats(ctx, b, []float64{0.25, 0.005})
	if err != nil {
		t.Fatal(err)
	}
	near := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-6 {
			t.Fatalf("%s = %g, want %g", name, got, want)
		}
	}
	near("mean", s.L.Mean, 0.5)
	near("median", s.L.Median, 0.5)
	near("p01", s.L.P01, 0.01)
	near("p99", s.L.P99, 0.99)
	near("q25", s.L.Percentiles[0], 0.25)
	near("q0.5%", s.L.Percentiles[1], 0.005) // interpolated between 0 and 0.01
	near("clipLow", s.ClipLow, 1.0/101)
	near("clipHigh", s.ClipHigh, 1.0/101)
	if len(s.Channels) != 0 {
		t.Fatal("gray image has no per-channel stats")
	}
	// selection agrees with sorting on random data with many ties
	r := rand.New(rand.NewSource(4))
	a := make([]float32, 5000)
	for i := range a {
		a[i] = float32(r.Intn(40)) / 40
	}
	sorted := append([]float32(nil), a...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	for _, k := range []int{0, 1, 999, 2500, 4999} {
		cp := append([]float32(nil), a...)
		if got := selectK(cp, k); got != sorted[k] {
			t.Fatalf("selectK(%d) = %g, want %g", k, got, sorted[k])
		}
	}
	rgb := randBuf(20, 10, 3, 5)
	s, _ = Stats(ctx, rgb, nil)
	if len(s.Channels) != 3 || math.Abs(s.Channels[1].Mean-mean(rgb.Plane(1))) > 1e-9 {
		t.Fatalf("per-channel stats: %+v", s.Channels)
	}
}

func TestHistogram(t *testing.T) {
	b := buffer.New(4, 1, 1, 1)
	copy(b.Pix, []float32{0, buffer.ToLinear(0.3), 1, 2})
	h, err := Histogram(ctx, b, 10, "L")
	if err != nil {
		t.Fatal(err)
	}
	if h.Counts[0] != 1 || h.Counts[2]+h.Counts[3] != 1 || h.Counts[9] != 1 || h.Over != 1 || h.Total != 4 {
		t.Fatalf("histogram %+v", h)
	}
	if _, err := Histogram(ctx, b, 10, "g"); err == nil {
		t.Fatal("channel g on a gray image must fail")
	}
}

func TestStatsWeighted(t *testing.T) {
	ctx := context.Background()
	b := buffer.New(10, 10, 3, 1)
	for c := 0; c < 3; c++ {
		p := b.Plane(c)
		for i := range p {
			p[i] = float32(i) / 99
		}
	}
	// binary weights: exactly Stats of the selected pixels (the first row)
	w := make([]float32, 100)
	for i := 0; i < 10; i++ {
		w[i] = 1
	}
	got, err := StatsWeighted(ctx, b, w, []float64{0.25})
	if err != nil {
		t.Fatal(err)
	}
	sub := buffer.New(10, 1, 3, 1)
	for c := 0; c < 3; c++ {
		copy(sub.Plane(c), b.Plane(c)[:10])
	}
	want, _ := Stats(ctx, sub, []float64{0.25})
	if got.Weight != 10 || got.W != 10 || got.H != 10 || got.L.Mean != want.L.Mean || got.L.Median != want.L.Median || got.L.Percentiles[0] != want.L.Percentiles[0] {
		t.Fatalf("binary: %+v vs %+v", got.L, want.L)
	}
	// equal soft weights give the same numbers as binary ones
	for i := 0; i < 10; i++ {
		w[i] = 0.5
	}
	soft, err := StatsWeighted(ctx, b, w, []float64{0.25})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]float64{{soft.L.Mean, want.L.Mean}, {soft.L.Median, want.L.Median}, {soft.L.P01, want.L.P01}, {soft.L.P99, want.L.P99}, {soft.L.Percentiles[0], want.L.Percentiles[0]}} {
		if math.Abs(pair[0]-pair[1]) > 1e-6 {
			t.Fatalf("equal soft weights: %v vs %v", soft.L, want.L)
		}
	}
	if soft.Weight != 5 {
		t.Fatalf("weight %v", soft.Weight)
	}
	// a weight pulls the mean towards its pixel
	w2 := make([]float32, 100)
	w2[0], w2[99] = 1, 0.25 // values 0 and 1
	m, _ := StatsWeighted(ctx, b, w2, nil)
	if math.Abs(m.L.Mean-0.2) > 1e-6 {
		t.Fatalf("weighted mean %v, want 0.2", m.L.Mean)
	}
	if _, err := StatsWeighted(ctx, b, make([]float32, 100), nil); err != ErrEmptySelection {
		t.Fatalf("empty: %v", err)
	}
}
