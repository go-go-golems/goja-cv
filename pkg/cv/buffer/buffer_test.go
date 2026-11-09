package buffer

import (
	"bytes"
	"context"
	"math"
	"math/rand"
	"testing"
)

func TestSRGBRoundTrip8And16(t *testing.T) {
	for i := 0; i < 256; i++ {
		if got := EncodeSRGB8(Linear8(uint8(i))); got != uint8(i) {
			t.Fatalf("8-bit %d -> %d", i, got)
		}
	}
	for i := 0; i < 65536; i++ {
		if got := EncodeSRGB16(Linear16(uint16(i))); got != uint16(i) {
			t.Fatalf("16-bit %d -> %d", i, got)
		}
	}
}

func TestPerceptualLUTAccuracy(t *testing.T) {
	for i := 0; i <= 100000; i++ {
		v := float32(i) / 100000
		exact := LinearToSRGB(float64(v))
		if d := math.Abs(float64(ToPerceptual(v)) - exact); d > 2e-5 {
			t.Fatalf("ToPerceptual(%g) off by %g", v, d)
		}
		back := ToLinear(ToPerceptual(v))
		if d := math.Abs(float64(back - v)); d > 3e-5 {
			t.Fatalf("round trip %g -> %g", v, back)
		}
	}
	// extended range is exact and monotone
	if ToPerceptual(2) <= ToPerceptual(1) || ToPerceptual(-0.5) >= 0 {
		t.Fatal("extended transfer not monotone")
	}
}

func randomBuffer(w, h, c int, seed int64) *Buffer {
	r := rand.New(rand.NewSource(seed))
	b := New(w, h, c, 1)
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

func TestEncodeDecodeRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := randomBuffer(13, 7, 3, 1)
	for _, tc := range []struct {
		name string
		enc  func(*bytes.Buffer) error
		bits int
		tol  float64
	}{
		{"png16", func(w *bytes.Buffer) error { return EncodePNG(ctx, w, src, 16) }, 16, 1e-4},
		{"png8", func(w *bytes.Buffer) error { return EncodePNG(ctx, w, src, 8) }, 8, 1.0 / 60},
		{"tiff16", func(w *bytes.Buffer) error { return EncodeTIFF16(ctx, w, src) }, 16, 1e-4},
	} {
		var bb bytes.Buffer
		if err := tc.enc(&bb); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got, info, err := Decode(ctx, &bb)
		if err != nil {
			t.Fatalf("%s decode: %v", tc.name, err)
		}
		if info.Bits != tc.bits || got.W != 13 || got.H != 7 || got.C != 3 {
			t.Fatalf("%s: info %+v buf %v", tc.name, info, got)
		}
		for i := range src.Pix {
			if d := math.Abs(float64(got.Pix[i] - src.Pix[i])); d > tc.tol {
				t.Fatalf("%s: pixel %d off by %g", tc.name, i, d)
			}
		}
	}
	gray := randomBuffer(5, 4, 1, 2)
	var bb bytes.Buffer
	if err := EncodePNG(ctx, &bb, gray, 16); err != nil {
		t.Fatal(err)
	}
	got, info, err := Decode(ctx, &bb)
	if err != nil || got.C != 1 || info.Bits != 16 {
		t.Fatalf("gray16: %v %+v %v", err, info, got)
	}
}

func TestResizeAreaPreservesMean(t *testing.T) {
	ctx := context.Background()
	src := randomBuffer(101, 67, 3, 3)
	for _, sz := range [][2]int{{50, 33}, {37, 29}, {1, 1}, {101, 67}} {
		out, err := Resize(ctx, src, sz[0], sz[1])
		if err != nil {
			t.Fatal(err)
		}
		if out.W != sz[0] || out.H != sz[1] {
			t.Fatalf("size %v", out)
		}
		for c := 0; c < 3; c++ {
			if d := math.Abs(mean(out.Plane(c)) - mean(src.Plane(c))); d > 1e-4 {
				t.Fatalf("resize %v channel %d mean off by %g", sz, c, d)
			}
		}
	}
}

func TestPyramidAtScale(t *testing.T) {
	ctx := context.Background()
	src := randomBuffer(640, 480, 3, 4)
	p, err := NewPyramid(ctx, src, 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Levels) != 5 || p.Levels[4].W != 40 {
		t.Fatalf("levels: %d, last %v", len(p.Levels), p.Levels[len(p.Levels)-1])
	}
	b, err := p.AtScale(ctx, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	if b.W != 192 || b.H != 144 || b.Scale != 0.3 {
		t.Fatalf("AtScale(0.3) = %v", b)
	}
	if d := math.Abs(mean(b.Plane(1)) - mean(src.Plane(1))); d > 1e-3 {
		t.Fatalf("mean drift %g", d)
	}
	full, _ := p.AtScale(ctx, 1)
	if full != src {
		t.Fatal("scale 1 must return the original buffer")
	}
	if _, err := p.AtScale(ctx, 1.5); err == nil {
		t.Fatal("expected error for scale > 1")
	}
}
