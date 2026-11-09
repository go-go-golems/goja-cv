package testimage

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// TestCommittedTestdataMatchesGenerator decodes the committed files and checks
// them against the generator, so testdata cannot silently drift.
func TestCommittedTestdataMatchesGenerator(t *testing.T) {
	want := Gradient()
	for _, tc := range []struct {
		file string
		bits int
		tol  float64
	}{{TIFF16, 16, 1e-4}, {PNG8, 8, 1.0 / 60}} {
		got, info, err := buffer.DecodeFile(context.Background(), filepath.Join("..", "..", "testdata", tc.file))
		if err != nil {
			t.Fatalf("%s: %v (run `make testdata`)", tc.file, err)
		}
		if info.Bits != tc.bits || got.W != W || got.H != H || got.C != 3 {
			t.Fatalf("%s: %+v", tc.file, info)
		}
		for i := range want.Pix {
			if d := math.Abs(float64(got.Pix[i] - want.Pix[i])); d > tc.tol {
				t.Fatalf("%s: pixel %d off by %g", tc.file, i, d)
			}
		}
	}
}
