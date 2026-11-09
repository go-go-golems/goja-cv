package kernels

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

func TestDistanceToSetIsExact(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	w, h := 37, 23
	set := make([]bool, w*h)
	for i := range set {
		set[i] = rng.Float64() < 0.03
	}
	d, err := DistanceToSet(context.Background(), set, w, h)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			best := math.Inf(1)
			for j := 0; j < h; j++ {
				for i := 0; i < w; i++ {
					if set[j*w+i] {
						best = math.Min(best, math.Hypot(float64(x-i), float64(y-j)))
					}
				}
			}
			if math.Abs(float64(d[y*w+x])-best) > 1e-4 {
				t.Fatalf("(%d,%d): %v want %v", x, y, d[y*w+x], best)
			}
		}
	}
}

func disk(w, h int, cx, cy, r float64) *buffer.Buffer {
	b := buffer.New(w, h, 1, 1)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) <= r {
				b.Pix[y*w+x] = 1
			}
		}
	}
	return b
}

func TestFeatherKeepsRangeAndMass(t *testing.T) {
	m := disk(64, 64, 32, 32, 12)
	f, err := Feather(context.Background(), m, 6)
	if err != nil {
		t.Fatal(err)
	}
	var s0, s1 float64
	for i, v := range f.Pix {
		if v < 0 || v > 1 {
			t.Fatalf("feather value %v", v)
		}
		s0 += float64(m.Pix[i])
		s1 += float64(v)
	}
	if math.Abs(s1-s0) > 1e-3*s0 {
		t.Fatalf("feather changed mass %g → %g", s0, s1)
	}
	if f.Pix[32*64+32] < 0.99 || f.Pix[32*64+32+11] <= 0.2 || f.Pix[32*64+32+11] >= 0.9 {
		t.Fatalf("feather profile centre %v edge %v", f.Pix[32*64+32], f.Pix[32*64+32+11])
	}
}

func TestFloodAndGuidedFilter(t *testing.T) {
	ctx := context.Background()
	b := buffer.New(40, 20, 1, 1)
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			if x >= 20 {
				b.Pix[y*40+x] = 0.8
			} else {
				b.Pix[y*40+x] = 0.05
			}
		}
	}
	m, err := FloodMask(ctx, b, 3, 3, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, v := range m.Pix {
		if v == 1 {
			n++
		}
	}
	if n != 400 {
		t.Fatalf("flood selected %d px, want 400", n)
	}
	// a blurry mask ramp centred on x=20 snaps toward the guide edge at x=20
	p := make([]float32, 800)
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			p[y*40+x] = float32(math.Max(0, math.Min(1, (float64(x)-14)/12)))
		}
	}
	L := b.Luminance()
	q, err := GuidedFilter(ctx, L, p, 40, 20, 4, 1e-4)
	if err != nil {
		t.Fatal(err)
	}
	row := 10 * 40
	if !(q[row+17] < p[row+17] && q[row+23] > p[row+23]) {
		t.Fatalf("guided filter did not sharpen toward the edge: %v→%v, %v→%v", p[row+17], q[row+17], p[row+23], q[row+23])
	}
	for _, v := range q {
		if v < 0 || v > 1 {
			t.Fatalf("refined value %v", v)
		}
	}
}

func TestLuminanceAndBoxMask(t *testing.T) {
	ctx := context.Background()
	b := buffer.New(10, 1, 1, 1)
	for i := range b.Pix {
		b.Pix[i] = buffer.ToLinear(float32(i) / 9)
	}
	a := 0.5
	m, err := LuminanceMask(ctx, b, &a, nil, 0.2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 10; i++ {
		if m.Pix[i] < m.Pix[i-1] {
			t.Fatalf("not monotone: %v", m.Pix)
		}
	}
	if m.Pix[0] != 0 || m.Pix[9] != 1 {
		t.Fatalf("ends %v %v", m.Pix[0], m.Pix[9])
	}
	bm := BoxMask(10, 10, 1, 2.5, 3, 4, 2.25)
	var s float64
	for _, v := range bm.Pix {
		s += float64(v)
	}
	if math.Abs(s-9) > 1e-5 {
		t.Fatalf("box mask mass %g, want 9", s)
	}
}
