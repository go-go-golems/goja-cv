package kernels

import (
	"context"
	"math"
	"math/rand"
	"testing"
)

// TestQuantilesEqualPercentile: Quantiles gives exactly Percentile's values
// (bit for bit, after the same interpolation) on inputs that stress the
// radix histogram: clipped runs, constants, values packed into one bin,
// negatives, signed zeros, and sizes on both sides of the small-input path.
func TestQuantilesEqualPercentile(t *testing.T) {
	ctx := context.Background()
	ps := []float64{0, 1e-9, 0.005, 0.01, 0.25, 0.5, 0.75, 0.99, 0.999, 1}
	rng := rand.New(rand.NewSource(3))
	gens := map[string]func(i int) float32{
		"uniform": func(int) float32 { return rng.Float32() },
		"normal":  func(int) float32 { return float32(0.3 + 0.05*rng.NormFloat64()) },
		"clipped": func(int) float32 {
			r := rng.Float32()
			if r < 0.3 {
				return 0
			}
			if r > 0.9 {
				return 1
			}
			return r
		},
		"constant": func(int) float32 { return 0.42 },
		"one bin":  func(int) float32 { return 0.3 + 1e-7*float32(rng.Intn(50)) }, // all share the high 16 bits
		"signed":   func(int) float32 { return float32(rng.NormFloat64()) },
		"zeros": func(i int) float32 {
			if i%2 == 0 {
				return float32(math.Copysign(0, -1))
			}
			return 0
		},
		"few": func(int) float32 { return float32(rng.Intn(4)) / 3 },
	}
	for name, gen := range gens {
		for _, n := range []int{1, 2, 7, quantileMinParallel - 1, quantileMinParallel, 200_003} {
			a := make([]float32, n)
			for i := range a {
				a[i] = gen(i)
			}
			orig := append([]float32(nil), a...)
			got, err := Quantiles(ctx, a, ps)
			if err != nil {
				t.Fatal(err)
			}
			for i, v := range a {
				if math.Float32bits(v) != math.Float32bits(orig[i]) {
					t.Fatalf("%s n=%d: Quantiles modified its input at %d", name, n, i)
				}
			}
			for i, p := range ps {
				want := Percentile(append([]float32(nil), a...), p)
				if got[i] != want {
					t.Fatalf("%s n=%d p=%g: Quantiles %v, Percentile %v", name, n, p, got[i], want)
				}
			}
		}
	}
}

func BenchmarkQuantiles24MP(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	a := make([]float32, 6000*4000)
	for i := range a {
		a[i] = float32(0.3 + 0.05*rng.NormFloat64())
	}
	ps := []float64{0.01, 0.5, 0.99}
	b.Run("quantiles", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := Quantiles(context.Background(), a, ps); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("copy+percentile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s := append([]float32(nil), a...)
			for _, p := range ps {
				Percentile(s, p)
			}
		}
	})
}
