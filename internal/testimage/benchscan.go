package testimage

import (
	"math"
	"math/rand"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// BenchScan builds the benchmark scan (GCV-006 §11.1, Phase 0): a w×h
// three-channel image in linear light, noise around 0.3 (σ 0.05) with
// specks bright 3×3 specks at fixed positions. It stands in for a full-size
// film scan when timing goja-cv; it is not a detection fixture. The result
// is deterministic.
func BenchScan(w, h, specks int) *buffer.Buffer {
	b := buffer.New(w, h, 3, 1)
	rng := rand.New(rand.NewSource(1)) // #nosec G404 -- a fixed seed: the bench image must be deterministic
	plane := w * h
	for i := 0; i < plane; i++ {
		for ch := 0; ch < 3; ch++ {
			b.Pix[ch*plane+i] = float32(math.Max(0, math.Min(1, 0.3+0.05*rng.NormFloat64())))
		}
	}
	for s := 0; s < specks && w > 40 && h > 40; s++ {
		cx, cy := 20+rng.Intn(w-40), 20+rng.Intn(h-40)
		for y := cy - 1; y <= cy+1; y++ {
			for x := cx - 1; x <= cx+1; x++ {
				for ch := 0; ch < 3; ch++ {
					p := &b.Pix[ch*plane+y*w+x]
					*p = float32(math.Min(1, float64(*p)+0.4))
				}
			}
		}
	}
	return b
}
