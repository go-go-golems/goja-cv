package buffer

import (
	"context"
	"math"
	"math/rand"
	"testing"
)

func TestIdentityTable(t *testing.T) {
	spaces := []Space{SpaceNone, SpaceLinear, SpaceEncoded}
	for _, e := range []Encoding{EncNone, EncSRGB, EncLinear} {
		for _, from := range spaces {
			for _, to := range spaces {
				want := from == to || e != EncSRGB || from == SpaceNone || to == SpaceNone
				if got := Identity(e, from, to); got != want {
					t.Errorf("Identity(%q, %q, %q) = %v, want %v", e, from, to, got, want)
				}
			}
		}
	}
	// the two non-trivial cells of the conversion table
	if Identity(EncSRGB, SpaceLinear, SpaceEncoded) || Identity(EncSRGB, SpaceEncoded, SpaceLinear) {
		t.Fatal("srgb linear<->encoded must convert")
	}
	if !Identity(EncLinear, SpaceLinear, SpaceEncoded) || !Identity(EncLinear, SpaceEncoded, SpaceLinear) {
		t.Fatal("linear-encoded data has equal spaces")
	}
}

func TestParseNames(t *testing.T) {
	for _, s := range Encodings {
		e, ok := ParseEncoding(s)
		if !ok || e.String() != s {
			t.Errorf("encoding %q: %v %v", s, e, ok)
		}
	}
	for _, s := range Spaces {
		sp, ok := ParseSpace(s)
		if !ok || sp.String() != s {
			t.Errorf("space %q: %v %v", s, sp, ok)
		}
	}
	if _, ok := ParseEncoding("perceptual"); ok {
		t.Error("perceptual is not an encoding")
	}
	if _, ok := ParseSpace("perceptual"); ok {
		t.Error("perceptual is not a space")
	}
}

func filled(vals ...float32) *Buffer {
	b := New(len(vals), 1, 1, 1)
	copy(b.Pix, vals)
	return b
}

func TestConvertKnownValues(t *testing.T) {
	ctx := context.Background()
	// guide §3.2: encoded 0.5 → linear 0.214, 0.05 → 0.0039, 0.8 → 0.604
	lin, err := Convert(ctx, filled(0.05, 0.5, 0.8), EncSRGB, SpaceEncoded, SpaceLinear)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{0.0039, 0.214, 0.604} {
		if got := float64(lin.Pix[i]); math.Abs(got-want) > 5e-4 {
			t.Errorf("decode #%d = %.5f, want %.4f", i, got, want)
		}
	}
	// the linear encoding is the identity, and Convert returns its input
	in := filled(0.05, 0.5, 0.8)
	same, err := Convert(ctx, in, EncLinear, SpaceEncoded, SpaceLinear)
	if err != nil || same != in {
		t.Fatalf("linear encoding must return the input: %v", err)
	}
	// density step example of §3.4 (decode direction)
	d, err := Convert(ctx, filled(0.10, 0.15, 0.60, 0.65), EncSRGB, SpaceEncoded, SpaceLinear)
	if err != nil {
		t.Fatal(err)
	}
	dark, light := float64(d.Pix[1]-d.Pix[0]), float64(d.Pix[3]-d.Pix[2])
	if math.Abs(dark-0.0096) > 2e-4 || math.Abs(light-0.0615) > 2e-4 {
		t.Errorf("decode steps %.4f / %.4f, want 0.0096 / 0.0615", dark, light)
	}
}

func TestConvertRoundTrip(t *testing.T) {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(1)) // #nosec G404 -- deterministic test data
	b := New(4096, 4, 3, 1)
	for i := range b.Pix {
		b.Pix[i] = float32(-0.5 + 2.5*rng.Float64())
	}
	b.Pix[0], b.Pix[1], b.Pix[2] = 0, 1, -1
	enc, err := Convert(ctx, b, EncSRGB, SpaceLinear, SpaceEncoded)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Convert(ctx, enc, EncSRGB, SpaceEncoded, SpaceLinear)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range b.Pix {
		got := back.Pix[i]
		if tol := 1e-6 * math.Max(math.Abs(float64(v)), 1e-6); math.Abs(float64(got-v)) > tol {
			t.Fatalf("round trip of %g gave %g (relative error %.3g)", v, got, math.Abs(float64(got-v))/math.Abs(float64(v)))
		}
	}
	// monotonic
	for _, from := range []Space{SpaceLinear, SpaceEncoded} {
		to := SpaceEncoded
		if from == SpaceEncoded {
			to = SpaceLinear
		}
		r := New(1001, 1, 1, 1)
		for i := range r.Pix {
			r.Pix[i] = float32(-0.5 + 2*float64(i)/1000)
		}
		c, err := Convert(ctx, r, EncSRGB, from, to)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(c.Pix); i++ {
			if c.Pix[i] < c.Pix[i-1] {
				t.Fatalf("%s→%s not monotonic at %g", from, to, r.Pix[i])
			}
		}
	}
}

func TestIntegerTablesAreExact(t *testing.T) {
	for u := 0; u < 256; u++ {
		if got, want := Linear8(uint8(u)), float32(SRGBToLinear(float64(u)/255)); got != want {
			t.Fatalf("Linear8(%d) = %g, exact %g", u, got, want)
		}
	}
	for u := 0; u < 65536; u++ {
		if got, want := Linear16(uint16(u)), float32(SRGBToLinear(float64(u)/65535)); got != want {
			t.Fatalf("Linear16(%d) = %g, exact %g", u, got, want)
		}
	}
	// the interpolated fast path stays within its documented error
	var worst float64
	for i := 0; i <= 100000; i++ {
		v := float32(i) / 100000
		worst = math.Max(worst, math.Abs(float64(ToPerceptual(v))-LinearToSRGB(float64(v))))
		worst = math.Max(worst, math.Abs(float64(ToLinear(v))-SRGBToLinear(float64(v))))
	}
	if worst > 2e-5 {
		t.Fatalf("LUT error %g above 2e-5", worst)
	}
}
