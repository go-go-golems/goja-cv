package kernels

import (
	"context"
	"math"
	"math/rand"
	"testing"
)

func TestSquareMinMaxMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	w, h := 29, 17
	p := make([]float32, w*h)
	for i := range p {
		p[i] = rng.Float32()
	}
	for _, r := range []int{1, 3, 8, 20} {
		for _, isMax := range []bool{false, true} {
			got, err := SquareMinMax(context.Background(), p, w, h, r, isMax)
			if err != nil {
				t.Fatal(err)
			}
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					want := float32(math.Inf(1))
					if isMax {
						want = float32(math.Inf(-1))
					}
					for j := max(0, y-r); j <= min(h-1, y+r); j++ {
						for i := max(0, x-r); i <= min(w-1, x+r); i++ {
							if isMax {
								want = max(want, p[j*w+i])
							} else {
								want = min(want, p[j*w+i])
							}
						}
					}
					if got[y*w+x] != want {
						t.Fatalf("r=%d max=%v (%d,%d): %v want %v", r, isMax, x, y, got[y*w+x], want)
					}
				}
			}
		}
	}
}

// TestSpeckBackgroundFollowsSlopesAndIgnoresSpecks: on a tilted plane with a
// strong dark and a strong bright speck, the background stays on the plane
// (also at the borders and next to the specks).
func TestSpeckBackgroundFollowsSlopesAndIgnoresSpecks(t *testing.T) {
	w, h := 120, 80
	L := make([]float32, w*h)
	plane := func(x, y int) float32 { return 0.2 + 0.003*float32(x) + 0.002*float32(y) }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			L[y*w+x] = plane(x, y)
			if math.Hypot(float64(x-40), float64(y-40)) <= 3 {
				L[y*w+x] -= 0.3
			}
			if math.Hypot(float64(x-80), float64(y-40)) <= 3 {
				L[y*w+x] += 0.3
			}
		}
	}
	bg, err := SpeckBackground(context.Background(), L, w, h, 8)
	if err != nil {
		t.Fatal(err)
	}
	var worst float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			worst = math.Max(worst, math.Abs(float64(bg[y*w+x]-plane(x, y))))
		}
	}
	wx, wy := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if math.Abs(float64(bg[y*w+x]-plane(x, y))) == worst {
				wx, wy = x, y
			}
		}
	}
	t.Logf("worst %g at (%d,%d)", worst, wx, wy)
	if worst > 0.006 {
		t.Fatalf("background error %g", worst)
	}
}
