package buffer_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

func testBuffer(w, h, c int, signed bool) *buffer.Buffer {
	b := buffer.New(w, h, c, 1)
	for i := range b.Pix {
		v := float32(i%97) / 96
		if signed && i%5 == 0 {
			v = -0.25 * v
		}
		b.Pix[i] = v
	}
	return b
}

// TestWritersRoundTrip is the format table of guide §7.10: what each format
// writes, at each depth, and what GCV-001's decoders read back (values,
// encoding, frame origin).
func TestWritersRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	origin := &[2]int{1536, 1024}
	for _, tc := range []struct {
		format     buffer.Format
		ext        string
		depth      int
		encodings  []buffer.Encoding
		tol        float64
		keepsMeta  bool // encoding and origin read back
		sample     string
		readsAsLin bool // NPY: values only, opens as linear at (0, 0)
	}{
		{buffer.FormatTIFF, ".tif", 16, []buffer.Encoding{buffer.EncSRGB, buffer.EncLinear}, 0.5/65535 + 1e-7, true, "uint", false},
		{buffer.FormatTIFF, ".tif", 8, []buffer.Encoding{buffer.EncSRGB, buffer.EncLinear}, 0.5/255 + 1e-7, true, "uint", false},
		{buffer.FormatTIFF, ".tif", 32, []buffer.Encoding{buffer.EncSRGB, buffer.EncLinear}, 0, true, "float", false},
		{buffer.FormatPNG, ".png", 16, []buffer.Encoding{buffer.EncSRGB, buffer.EncLinear}, 0.5/65535 + 1e-7, true, "uint", false},
		{buffer.FormatPNG, ".png", 8, []buffer.Encoding{buffer.EncSRGB, buffer.EncLinear}, 0.5/255 + 1e-7, true, "uint", false},
		{buffer.FormatNPY, ".npy", 32, []buffer.Encoding{buffer.EncLinear, buffer.EncSRGB}, 0, false, "float", true},
		{buffer.FormatNPY, ".npy", 16, []buffer.Encoding{buffer.EncLinear}, 0.5/65535 + 1e-7, false, "uint", true},
		{buffer.FormatJPEG, ".jpg", 8, []buffer.Encoding{buffer.EncSRGB}, 0.06, false, "uint", false},
	} {
		for _, ch := range []int{1, 3} {
			for _, enc := range tc.encodings {
				name := fmt.Sprintf("%s%d-%dch-%s", tc.format, tc.depth, ch, enc)
				src := testBuffer(37, 23, ch, false)
				if tc.format == buffer.FormatJPEG { // lossy and chroma-subsampled: a smooth image
					n := src.N()
					for i := range src.Pix {
						x, y, c := i%n%37, i%n/37, i/n
						src.Pix[i] = float32(x+y+5*c) / 70
					}
				}
				var out bytes.Buffer
				if err := buffer.Encode(ctx, &out, src, tc.format, tc.depth, buffer.Meta{Encoding: enc, Origin: origin}); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				p := filepath.Join(dir, name+tc.ext)
				if err := os.WriteFile(p, out.Bytes(), 0o644); err != nil { // #nosec G306 -- test data
					t.Fatal(err)
				}
				got, info, err := buffer.DecodeFile(ctx, p, buffer.DecodeOptions{})
				if err != nil {
					t.Fatalf("%s: decode: %v", name, err)
				}
				if got.W != 37 || got.H != 23 || got.C != ch || info.Bits != tc.depth || info.Sample != tc.sample {
					t.Fatalf("%s: read %dx%dx%d %+v", name, got.W, got.H, got.C, info)
				}
				var worst, sum float64
				for i := range src.Pix {
					d := math.Abs(float64(got.Pix[i] - src.Pix[i]))
					worst, sum = math.Max(worst, d), sum+d
				}
				if tc.format == buffer.FormatJPEG { // lossy (and chroma-subsampled): the mean error
					worst = sum / float64(len(src.Pix))
				}
				if worst > tc.tol {
					t.Errorf("%s: values off by %g (tolerance %g)", name, worst, tc.tol)
				}
				switch {
				case tc.keepsMeta:
					if info.Encoding != enc || info.Origin == nil || *info.Origin != *origin {
						t.Errorf("%s: metadata read back as %s %v", name, info.Encoding, info.Origin)
					}
				case tc.readsAsLin:
					if info.Encoding != buffer.EncLinear || info.Origin != nil {
						t.Errorf("%s: NPY reads as %s %v, want linear at (0, 0)", name, info.Encoding, info.Origin)
					}
				default: // JPEG: values only, sRGB
					if info.Encoding != buffer.EncSRGB || info.Origin != nil {
						t.Errorf("%s: JPEG reads as %s %v", name, info.Encoding, info.Origin)
					}
				}
			}
		}
	}
}

// TestWritersClampIntegersKeepFloats: integer depths clamp values outside
// [0, 1] (ClippedFraction says how many); float TIFF and NPY keep them.
func TestWritersClampIntegersKeepFloats(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	src := testBuffer(20, 10, 1, true)
	if f := buffer.ClippedFraction(src); !(f > 0.1 && f < 0.3) {
		t.Fatalf("clipped fraction %g", f)
	}
	for _, tc := range []struct {
		f     buffer.Format
		ext   string
		depth int
		keeps bool
	}{{buffer.FormatTIFF, ".tif", 32, true}, {buffer.FormatNPY, ".npy", 32, true}, {buffer.FormatTIFF, ".tif", 16, false}, {buffer.FormatPNG, ".png", 16, false}} {
		var out bytes.Buffer
		if err := buffer.Encode(ctx, &out, src, tc.f, tc.depth, buffer.Meta{Encoding: buffer.EncLinear}); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, fmt.Sprintf("s%d%s", tc.depth, tc.ext))
		if err := os.WriteFile(p, out.Bytes(), 0o644); err != nil { // #nosec G306 -- test data
			t.Fatal(err)
		}
		got, _, err := buffer.DecodeFile(ctx, p, buffer.DecodeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if neg := got.Pix[0+5] < 0; neg != tc.keeps {
			t.Errorf("%s %d: pixel 5 reads %g (negative kept: %v, want %v)", tc.f, tc.depth, got.Pix[5], neg, tc.keeps)
		}
	}
}
