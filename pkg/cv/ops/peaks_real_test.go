package ops

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// BenchmarkRealPeaks: peaks of the harness tile's luminance (1024², density
// opened as linear), as the op runs them (GCV-005 §7.8: budget 0.5 s for
// radii [2, 4, 8]). Gated on GOJACV_TILES.
func BenchmarkRealPeaks(b *testing.B) {
	lin, gray, _, _ := hotPoints(b)
	L := buffer.New(lin.W, lin.H, 1, 1)
	copy(L.Plane(0), gray)
	for _, radii := range [][]float64{{3}, {2, 4, 8}} {
		for _, snr := range []float64{0.5, 1} {
			b.Run(fmt.Sprintf("r%v/snr%g", radii, snr), func(b *testing.B) {
				n := 0
				for range b.N {
					ps, err := kernels.Peaks(context.Background(), L, kernels.PeakOptions{Radii: radii, MinSNR: snr, Polarity: "bright"})
					if err != nil {
						b.Fatal(err)
					}
					n = len(ps)
				}
				b.ReportMetric(float64(n), "peaks")
			})
		}
	}
}
